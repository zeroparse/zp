// Package limits holds the defaults of the limits of one parse invocation
// (core.Limits) and the overflow-safe arithmetic used to spend budget against
// them. The parser, the document and the structural query share it.
package limits

import (
	"math"

	"zeroparse.dev/zp/core"
)

// Default limits on call nesting depth. Backends that run recursively use the host stack, so
// their limit is smaller.
const (
	DefaultMaxDepth          = 100_000
	DefaultMaxDepthIterative = 10_000_000
)

// The default budget of a parse is DefaultBudgetPerUnit for each unit of input, and at least
// DefaultBudgetFloor.
const (
	DefaultBudgetFloor   = 1 << 20
	DefaultBudgetPerUnit = 1024
)

// DefaultBudget returns the budget of a parse whose Compute is zero, for an input of n units of the
// position unit of the parse. It counts the work that a parse of the input from scratch does. A
// Document parse also does work that only a Document does, and the default budget leaves that out,
// so a Document parse under it succeeds where a parse from scratch of its text does.
func DefaultBudget(n uint64) uint64 {
	if n > math.MaxUint64/DefaultBudgetPerUnit {
		return math.MaxUint64 - 1
	}
	return max(DefaultBudgetFloor, n*DefaultBudgetPerUnit)
}

// Merge returns l with every nonzero field of o in place of the field of l: the limits of a later
// WithLimits over those of an earlier one. A zero field of o keeps the field of l.
func Merge(l, o core.Limits) core.Limits {
	return core.Limits{Compute: nonzero(o.Compute, l.Compute), Memory: nonzero(o.Memory, l.Memory), Input: nonzero(o.Input, l.Input), Depth: nonzero(o.Depth, l.Depth)}
}

// nonzero returns v, or def if v is zero.
func nonzero(v, def uint64) uint64 {
	if v != 0 {
		return v
	}
	return def
}

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
