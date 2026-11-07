package core

import (
	"context"
	"errors"
	"unicode/utf8"

	"zeroparse.dev/zp/internal/limits"
)

// BudgetCheckInterval is the spend between two observations of cancellation.
const BudgetCheckInterval = uint64(256)

// BudgetControl is private state for a single parser invocation. Callers must
// split long operations into bounded reservations so cancellation can be
// observed between chunks.
type BudgetControl struct {
	Ctx    context.Context
	Limits limits.Limits

	// Used is the spend reserved so far; SinceCheck is the part of it since the
	// last observation of cancellation.
	Used       uint64
	SinceCheck uint64

	// Adaptive is set for a stream whose budget is the default: the budget follows the furthest
	// position the parser has demanded from the stream (Demand).
	Adaptive bool

	// DefaultCompute is set when the Compute limit is the default of the entry point. Under it the
	// limit counts only the work that a parse from scratch counts: the units reserved for work that
	// only a Document does (ChargeDoc, FreeBegin) are also added to Doc and to the limit, so they
	// leave room for the counted work. An explicit limit counts all units, and Doc stays zero.
	DefaultCompute bool
	// Doc is the part of Used that only a Document parse reserves, under a default limit.
	Doc uint64
	// freeing is set between FreeBegin and FreeEnd; freed holds the limit that FreeBegin lifted.
	freeing  bool
	freed    uint64
	freeMark uint64 // Used when the open region began

	// Mem is the memory, in bytes, charged to the control itself: the bytes of the history a
	// Document retains outside its memo table when its parse starts. The memory check adds values,
	// errs and the counters of the parser attached by AttachMemory (MemoryUsed).
	Mem uint64
	// values counts the storage of the values the parse builds on the controlled path: text
	// copied out of the input, concatenated strings, rebound environments, memo keys and the error
	// lists of memo entries. It belongs to the nodes and memo entries built since, so a stream
	// returns it with their chunks (Parser.splitChunks).
	values uint64
	// errs counts the syntax errors the parse builds. A stream returns it at a commit that
	// releases the recovered errors to the diagnostic tail (Parser.Commit), which counts those it
	// keeps; otherwise the errors stay held to the end of the parse.
	errs uint64
	memP *Parser
}

// ResolveLimits returns the limits a parse runs under for an input of n units of its position
// unit: a zero Compute selects the default (limits.DefaultBudget), and Unlimited disables the
// budget, which the result leaves zero. A stream does not know n: its default starts at the floor
// and grows with the furthest position the parser demands (BudgetControl.Demand), which adaptive reports. The reference
// build has no controls to charge, so no default applies there.
func ResolveLimits(l limits.Limits, n uint64, stream bool) (resolved limits.Limits, adaptive bool) {
	if l.Input == limits.Unlimited {
		l.Input = 0 // the default of Input is Unlimited
	}
	if l.Memory == limits.Unlimited {
		l.Memory = 0 // the default of Memory is Unlimited
	}
	switch l.Compute {
	case limits.Unlimited:
		l.Compute = 0
	case 0:
		switch {
		case !BudgetControlsEnabled:
		case stream:
			l.Compute, adaptive = limits.DefaultBudgetFloor, true
		default:
			l.Compute = limits.DefaultBudget(n)
		}
	}
	return l, adaptive
}

// Demand raises the default budget of a stream to that of n units, where n is the number of
// units up to the furthest position the parser has asked the stream for. The number depends on the
// input and the grammar only, not on how the reader chunks the input. It does nothing for any
// other budget, and on a nil receiver; the budget never shrinks.
func (c *BudgetControl) Demand(n int) {
	if c == nil || !c.Adaptive || n <= 0 {
		return
	}
	if b := limits.DefaultBudget(uint64(n)); b > c.Limits.Compute {
		c.Limits.Compute = b
	}
}

// NewBudgetControl returns nil when neither cancellation nor a finite compute budget or memory
// limit is active. A nil context is equivalent to context.Background. A context that can never be
// canceled (Done is nil) is never polled.
func NewBudgetControl(ctx context.Context, limits limits.Limits) *BudgetControl {
	if ctx != nil && ctx.Done() == nil {
		ctx = nil
	}
	if ctx == nil && limits.Compute == 0 && limits.Memory == 0 {
		return nil
	}
	return &BudgetControl{Ctx: ctx, Limits: limits}
}

// ContextPanic attributes a panic to caller-owned context code rather than VM
// execution. Parse boundaries re-raise its original value after cleanup.
type ContextPanic struct{ Value any }

func contextErr(ctx context.Context) error {
	defer func() {
		if x := recover(); x != nil {
			switch x.(type) {
			case Fatal, ContextPanic:
				panic(x) // preserve engine interruption and an already attributed panic
			default:
				panic(ContextPanic{Value: x})
			}
		}
	}()
	return ctx.Err()
}

// Check observes cancellation, then the memory limit, at an explicit engine checkpoint. It is safe
// on a nil receiver.
func (c *BudgetControl) Check() {
	if c == nil {
		return
	}
	if c.Ctx != nil {
		if err := contextErr(c.Ctx); err != nil {
			panic(Fatal{Err: err})
		}
		c.SinceCheck = 0
	}
	if c.Limits.Memory != 0 {
		c.checkMemory()
	}
}

// Charge atomically reserves n compute units. Cancellation wins when it is
// observed at a poll or when the reservation would fail. A failed reservation
// leaves the counter unchanged. It is safe on a nil receiver.
func (c *BudgetControl) Charge(n uint64) {
	if c == nil {
		return
	}

	exceeded := limits.ExceedsLimit(c.Used, n, c.Limits.Compute)
	poll := limits.ReachesInterval(c.SinceCheck, n, BudgetCheckInterval)
	if poll || exceeded {
		c.chargeSlow(n, poll, exceeded)
		return
	}
	c.Used = limits.SaturatingAdd(c.Used, n)
	c.SinceCheck = limits.SaturatingAdd(c.SinceCheck, n)
}

// noFree is the mark that FreeBegin returns when it opens no region.
const noFree = ^uint64(0)

// ChargeDoc reserves n units of work that only a Document parse does, as Charge does, and under a
// default limit leaves them out of the count against it: the limit rises by n first, so the units
// leave the room that counted work has. Under an explicit limit it is Charge. It is safe on a nil
// receiver.
func (c *BudgetControl) ChargeDoc(n uint64) {
	if c == nil {
		return
	}
	if c.DefaultCompute && !c.freeing {
		c.Doc += n
		c.Limits.Compute = limits.SaturatingAdd(c.Limits.Compute, n)
	}
	c.Charge(n)
}

// FreeBegin opens a region in which every reservation is work that only a Document parse does, as
// ChargeDoc reserves it, and returns the mark for FreeEnd. The region ends the work it covers before
// any counted work starts. Under a default limit it lifts the limit until FreeEnd; otherwise, or in
// a region, it opens none and returns noFree. It is safe on a nil receiver.
func (c *BudgetControl) FreeBegin() uint64 {
	if c == nil || !c.DefaultCompute || c.freeing {
		return noFree
	}
	c.freeing, c.freed, c.Limits.Compute = true, c.Limits.Compute, 0
	c.freeMark = c.Used
	return c.Used
}

// FreeEnd ends the region that FreeBegin opened (mark is what it returned, noFree if it opened none):
// the units reserved since the region began, or resumed after FreeHold, are Document units, and the
// limit is what it was plus them.
func (c *BudgetControl) FreeEnd(mark uint64) {
	if mark == noFree || !c.freeing {
		return
	}
	n := c.Used - c.freeMark
	c.Doc += n
	c.Limits.Compute, c.freeing = limits.SaturatingAdd(c.freed, n), false
}

// Freeing reports whether a region is open. It is safe on a nil receiver.
func (c *BudgetControl) Freeing() bool { return c != nil && c.freeing }

// FreeHold ends the open region for work that counts, and reports whether there was one; FreeResume
// opens it again. The FreeEnd of the region that was held takes the work from the resume on.
func (c *BudgetControl) FreeHold() bool {
	if c == nil || !c.freeing {
		return false
	}
	c.FreeEnd(c.freeMark)
	return true
}

// FreeResume opens again the region that FreeHold ended, if it ended one.
func (c *BudgetControl) FreeResume(held bool) {
	if held {
		c.FreeBegin()
	}
}

// chargeSlow is Charge on a poll boundary or a failed reservation, kept out of line so that the
// reservation every step makes stays small. It checks cancellation first, so that an already
// canceled invocation wins over a simultaneously exhausted budget, then, at a poll, the memory
// limit.
func (c *BudgetControl) chargeSlow(n uint64, poll, exceeded bool) {
	if c.Ctx != nil {
		if err := contextErr(c.Ctx); err != nil {
			panic(Fatal{Err: err})
		}
	}
	if poll && c.Limits.Memory != 0 {
		c.checkMemory()
	}
	if exceeded {
		panic(Fatal{Err: &limits.LimitError{
			Kind: limits.LimitCompute, Limit: c.Limits.Compute - c.Doc,
			Used: c.Used - c.Doc, Requested: n,
		}})
	}
	// Not exceeded, so the reservation reached a poll boundary.
	c.Used = limits.SaturatingAdd(c.Used, n)
	c.SinceCheck = 0
}

// InputUnits returns the length of s in units of the position unit. Below the size at which the
// default budget leaves its floor the byte length stands in for the count of code points, which it
// bounds from above.
func InputUnits(s string, unit PositionUnit) uint64 {
	if unit == Bytes || len(s) <= limits.DefaultBudgetFloor/limits.DefaultBudgetPerUnit {
		return uint64(len(s))
	}
	return uint64(utf8.RuneCountInString(s))
}

// ErrNoControls is returned in the reference build when a context or a finite limit is requested.
var ErrNoControls = errors.New("budget controls are unavailable in the reference build")

// CheckInput rejects an input of s that is longer than the resolved Input limit of l, in the position
// unit. The check is O(1) unless the byte length passes the limit in CodePoints, where the exact
// count decides: the byte length bounds the count from above.
func CheckInput(l limits.Limits, s string, unit PositionUnit) error {
	if l.Input == 0 {
		return nil
	}
	if !BudgetControlsEnabled {
		// The reference build exists only for measurement, so it refuses any Input limit too.
		return ErrNoControls
	}
	n := uint64(len(s))
	if n > l.Input && unit == CodePoints {
		n = uint64(utf8.RuneCountInString(s))
	}
	return CheckInputUnits(l, n)
}

// CheckInputUnits rejects an input of n units that is longer than the resolved Input limit of l.
func CheckInputUnits(l limits.Limits, n uint64) error {
	if l.Input == 0 {
		return nil
	}
	if !BudgetControlsEnabled {
		// The reference build exists only for measurement, so it refuses any Input limit too.
		return ErrNoControls
	}
	if n > l.Input {
		return &limits.LimitError{Kind: limits.LimitInput, Limit: l.Input, Requested: n}
	}
	return nil
}
