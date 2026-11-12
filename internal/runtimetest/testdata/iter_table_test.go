package runtime

import (
	"testing"

	"zeroparse.dev/zp/internal/ir/exec/conform"
)

// The iteration table (conform.IterCases) on the compositions itemAt and filterKeep of the node and the typed
// runtime (TermItemAt and TermFilterKeep, printed from Exec IR), through the helpers of compose_test.go
// for foldl, foldr, map and filter.

// iterCall calls the built-in function of the iteration table's call on v through the helpers of compose_test.go:
// fold, mapList and filterList are those of the node or the typed runtime.
func iterCall(call string, v any, fold func(fn string, right bool, acc, list any, f func(acc, item any) any) any,
	mapList, filterList func(list any, f func(item any) any) any) any {
	switch call {
	case conform.CallFoldl, conform.CallFoldr:
		return fold(call, call == conform.CallFoldr, nil, v, func(_, item any) any { return item })
	case conform.CallMap:
		return mapList(v, func(item any) any { return item })
	case conform.CallFilter:
		return filterList(v, func(item any) any { return item == any(1) })
	}
	return filterList(v, func(item any) any { return item })
}

func TestValueIterTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		c, vals := nodeBuiltinFixture(controlled)
		if err := conform.RunIters("node", controlled, func(call, value string) (string, int) {
			var before uint64
			if controlled {
				before = c.P.Control.budgetUsed()
			}
			got := memberRead(func() any { return iterCall(call, vals[value], c.fxFold, c.fxMap, c.fxFilter) }, memberCanon)
			if controlled {
				return got, int(c.P.Control.budgetUsed() - before)
			}
			return got, 0
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
		tc, tvals := typedBuiltinFixture(controlled)
		if err := conform.RunIters("typed", controlled, func(call, value string) (string, int) {
			var before uint64
			if controlled {
				before = tc.P.Control.budgetUsed()
			}
			got := memberRead(func() any { return iterCall(call, tvals[value], tc.fxFold, tc.fxMap, tc.fxFilter) }, tmemberCanon)
			if controlled {
				return got, int(tc.P.Control.budgetUsed() - before)
			}
			return got, 0
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}
