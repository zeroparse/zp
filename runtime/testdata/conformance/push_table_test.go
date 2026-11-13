package runtime

import (
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec/conform"
)

// The push table (conform.PushCases) on the compositions push and pushItems of the node and the typed runtime
// (TermPush and TermPushItems, printed from Exec IR), as a generated parser calls them for the
// elements of list, map, a tuple literal and concat.

func TestValuePushTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		c, vals := nodeBuiltinFixture(controlled)
		if err := conform.RunPushes("node", controlled, func(call, value string) (string, int, int) {
			base := len(c.P.KidStack)
			var before uint64
			if controlled {
				before = c.P.Control.budgetUsed()
			}
			got := memberRead(func() any {
				if call == conform.CallPush {
					c.fxPush(conform.PushFn(call), vals[value])
				} else {
					c.fxPushItems(vals[value])
				}
				return nil
			}, memberCanon)
			units := 0
			if controlled {
				units = int(c.P.Control.budgetUsed() - before)
			}
			pushed := slices.Clone(c.P.KidStack[base:])
			c.P.DropKids(base)
			if got == "nil" {
				got = memberRead(func() any {
					if call == conform.CallPush {
						if len(pushed) != 1 || pushed[0] == nil {
							return nil
						}
						return pushed[0]
					}
					return c.listNode(pushed)
				}, memberCanon)
			}
			return got, len(pushed), units
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
		tc, tvals := typedBuiltinFixture(controlled)
		if err := conform.RunPushes("typed", controlled, func(call, value string) (string, int, int) {
			base := len(tc.P.KidStack)
			var before uint64
			if controlled {
				before = tc.P.Control.budgetUsed()
			}
			got := memberRead(func() any {
				if call == conform.CallPush {
					tc.fxPush(conform.PushFn(call), tvals[value])
				} else {
					tc.fxPushItems(tvals[value])
				}
				return nil
			}, tmemberCanon)
			units := 0
			if controlled {
				units = int(tc.P.Control.budgetUsed() - before)
			}
			pushed := slices.Clone(tc.P.KidStack[base:])
			tc.P.DropKids(base)
			if got == "nil" {
				if call == conform.CallPush {
					got = "nil"
					if len(pushed) == 1 && pushed[0] != nil {
						e := pushed[0]
						if k := valKind(e); k == valKindInt || k == valKindString || k == valKindBool {
							// The typed runtime keeps a scalar element as it is, where the node runtime boxes it.
							got = "box " + tmemberCanon(e)
						} else {
							got = tmemberCanon(e)
						}
					}
				} else {
					got = tmemberCanon(&TSeq{Typ: TypeListValue, Kids: pushed})
				}
			}
			return got, len(pushed), units
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}
