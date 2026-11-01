// Package limits defines the limits of one parse invocation, the error
// that reports an exhausted limit, and the overflow-safe arithmetic used to
// spend budget against them. The parser, the document and the structural
// query share these definitions; the public API exposes them as aliases.
package limits

import (
	"errors"
	"fmt"
	"math"
)

// Default limits on call nesting depth. Backends that run recursively use the host stack, so
// their limit is smaller.
const (
	DefaultMaxDepth          = 100_000
	DefaultMaxDepthIterative = 10_000_000
)

// Unlimited is the value of a limit that disables it. Every field of Limits is a uint64 and
// takes it, so one constant serves them all.
const Unlimited = math.MaxUint64

// The default budget of a parse is DefaultBudgetPerUnit for each unit of input, and at least
// DefaultBudgetFloor.
const (
	DefaultBudgetFloor   = 1 << 20
	DefaultBudgetPerUnit = 1024
)

// DefaultBudget returns the budget of a parse whose Compute is zero, for an input of n units of the
// position unit of the parse.
func DefaultBudget(n uint64) uint64 {
	if n > math.MaxUint64/DefaultBudgetPerUnit {
		return math.MaxUint64 - 1
	}
	return max(DefaultBudgetFloor, n*DefaultBudgetPerUnit)
}

// Limits bounds one parse invocation. A zero limit selects the default of that limit, and
// Unlimited disables it.
//
// Compute bounds the cumulative cost of the parse: every input unit
// inspected, every rule call and loop iteration, and every unit of value
// construction adds to one counter, so a bound on it bounds scanning,
// recursion and value building alike.
//
// Input bounds the length of the input in the position unit of the parse. The default is
// Unlimited: the default budget already grows with the input. A parse of an input that is longer
// fails before any work, with a LimitError of kind LimitInput; a stream fails when the parser
// reaches a unit beyond the limit.
//
// Depth bounds the nesting of recursive rule calls: the entries of rules on a cycle of calls and
// the operands of Operators Rules that nest like them. Zero selects the backend's default
// (DefaultMaxDepth, or DefaultMaxDepthIterative on the iterative bytecode VM, or the default a
// generated parser was generated with). Unlimited disables it only where the call stack is on the
// heap, the iterative bytecode VM; elsewhere it is an option error, because the host stack would
// overflow first. A finite Depth above the largest int counts as the largest int. A deeper parse
// fails with a LimitError of kind LimitDepth.
type Limits struct {
	Compute uint64
	Input   uint64
	Depth   uint64
}

// Merge returns l with every nonzero field of o in place of the field of l: the limits of a later
// WithLimits over those of an earlier one. A zero field of o keeps the field of l.
func Merge(l, o Limits) Limits {
	return Limits{Compute: nonzero(o.Compute, l.Compute), Input: nonzero(o.Input, l.Input), Depth: nonzero(o.Depth, l.Depth)}
}

// nonzero returns v, or def if v is zero.
func nonzero(v, def uint64) uint64 {
	if v != 0 {
		return v
	}
	return def
}

// ErrLimit is matched by errors.Is for any LimitError.
var ErrLimit = errors.New("parse limit exceeded")

// LimitKind identifies the limit that was exhausted.
type LimitKind uint8

const (
	// LimitCompute is the kind of an exhausted Compute limit.
	LimitCompute LimitKind = iota
	// LimitInput is the kind of an input longer than the Input limit.
	LimitInput
	// LimitDepth is the kind of a parse that nests recursive rule calls deeper than the Depth
	// limit.
	LimitDepth
)

// LimitError reports a reservation that would exceed a limit. Used is the
// counter value before the rejected reservation; Requested is the complete
// atomic reservation that could not be made. For LimitInput, Limit is the Input limit, Used is zero and
// Requested is the length of the input in the position unit; for a stream it is the number of
// units the parser had asked for, or the length of the stream if that is shorter. For LimitDepth,
// Limit is the resolved Depth limit, Used is the depth reached (the limit) and Requested is one,
// the nested call that would exceed it.
type LimitError struct {
	Kind      LimitKind
	Limit     uint64
	Used      uint64
	Requested uint64
}

func (e *LimitError) Error() string {
	if e == nil {
		return ErrLimit.Error()
	}
	switch e.Kind {
	case LimitInput:
		return fmt.Sprintf("parse input limit exceeded: limit %d, length %d", e.Limit, e.Requested)
	case LimitDepth:
		return fmt.Sprintf("nesting too deep: more than %d nested recursive rule calls", e.Limit)
	}
	return fmt.Sprintf("parse compute limit exceeded: limit %d, used %d, requested %d", e.Limit, e.Used, e.Requested)
}

// Is reports whether target is the limit sentinel.
func (e *LimitError) Is(target error) bool { return e != nil && target == ErrLimit }

// ExceedsLimit checks a reservation without computing a potentially
// overflowing used+requested sum. The limit is already resolved: zero, which
// Unlimited resolves to, sets no bound.
func ExceedsLimit(used, requested, limit uint64) bool {
	return limit != 0 && (used > limit || requested > limit-used)
}

// ReachesInterval checks whether adding requested would meet or cross interval
// without overflowing. Progress counters are kept below their interval.
func ReachesInterval(used, requested, interval uint64) bool {
	return used >= interval || requested >= interval-used
}

// SaturatingAdd returns a+b, or the largest uint64 when the sum overflows.
func SaturatingAdd(a, b uint64) uint64 {
	if math.MaxUint64-a < b {
		return math.MaxUint64
	}
	return a + b
}
