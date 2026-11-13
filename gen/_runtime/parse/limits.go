package parse

import (
	"errors"
	"fmt"
	"math"
)

// Unlimited is the value of a limit that disables it. Every field of Limits is a uint64 and
// takes it, so one constant serves them all.
const Unlimited = math.MaxUint64

// Limits bounds one parse invocation. A zero limit selects the default of that limit, and
// Unlimited disables it.
//
// Compute bounds the cumulative cost of the parse: every input unit inspected, every rule call
// and loop iteration, and every unit of value construction adds to one counter, so a bound on it
// bounds scanning, recursion and value building alike. The default grows with the length of the
// input.
//
// Memory bounds the bytes of storage the parser holds for the parse, counted when the parser
// takes it into use and returned when it lets it go at a point it controls (a stream's commit, a
// Document's history reset). Storage dropped by backtracking is not returned. The default is
// Unlimited. The count is checked with the compute polls, so a parse can pass the limit by what it
// allocates between two polls before it fails with a LimitError of kind LimitMemory. A
// Document's count starts at the bytes it retains from earlier parses. What is counted depends on
// the parser and its version.
//
// Input bounds the length of the input in the position unit of the parse. The default is
// Unlimited: the default compute budget already grows with the input. A parse of an input that
// is longer fails before any work, with a LimitError of kind LimitInput; a stream fails when the
// parser reaches a unit beyond the limit.
//
// Depth bounds the nesting of recursive rule calls: the entries of rules on a cycle of calls and
// the operands of Operators Rules that nest like them. Zero selects the default of the parser
// (of the backend that runs the parse, or the default a generated parser was generated with).
// Unlimited disables it only where the call stack is on the heap, the engine's iterative bytecode
// VM; elsewhere it is an option error, because the goroutine stack would overflow first. A finite
// Depth above the largest int counts as the largest int. A deeper parse fails with a LimitError
// of kind LimitDepth.
type Limits struct {
	Compute uint64
	Memory  uint64
	Input   uint64
	Depth   uint64
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
	// LimitMemory is the kind of a parse whose storage passed the Memory limit.
	LimitMemory
)

// LimitError reports a reservation that would exceed a limit. Used is the counter value before
// the rejected reservation; Requested is the complete atomic reservation that could not be made.
// For LimitInput, Limit is the Input limit, Used is zero and Requested is the length of the input
// in the position unit; for a stream it is the number of units the parser had asked for, or the
// length of the stream if that is shorter. For LimitDepth, Limit is the resolved Depth limit,
// Used is the depth reached (the limit) and Requested is one, the nested call that would exceed
// it. For LimitMemory, Used is the count of bytes at the check that failed and Requested is zero,
// since memory is checked at polls rather than reserved.
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
	case LimitMemory:
		return fmt.Sprintf("parse memory limit exceeded: limit %d, used %d", e.Limit, e.Used)
	}
	return fmt.Sprintf("parse compute limit exceeded: limit %d, used %d, requested %d", e.Limit, e.Used, e.Requested)
}

// Is reports whether target is ErrLimit.
func (e *LimitError) Is(target error) bool { return e != nil && target == ErrLimit }
