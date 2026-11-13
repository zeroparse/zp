package core

import (
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/limits"
	"zeroparse.dev/zp/internal/value"
)

// TestEngineValuePushTable runs the push table (conform.PushCases) on the engine's compositions push and
// pushItems (termPush and termPushItems, printed from Exec IR): through Push, which the backends call, and
// pushItems itself, without the step that PushElems charges for each argument of concat.
func TestEngineValuePushTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		prog := &SharedProgram{}
		InitSharedProgram(prog, compiletest.Program(t, "@start\nrule main = \"a\"\n", compile.Options{}))
		p := &Parser{Prog: prog, Input: NewInput(conform.BuiltinInput, Bytes)}
		if controlled {
			p.Control = NewBudgetControl(nil, limits.Limits{Compute: 1 << 62})
		}
		c := &EvalCtx{P: p}
		one, _ := element(c, "list", 1)
		two, _ := element(c, "list", 2)
		vals := map[string]any{
			conform.MemberNil:     nil,
			conform.MemberInt:     1,
			conform.BuiltinString: "ab",
			conform.BuiltinBool:   true,
			conform.MemberList:    c.ListNode(nil),
			conform.BuiltinPair:   c.ListNode([]*value.Node{one, two}),
			conform.BuiltinFunc:   builtinFunc{},
		}
		// The budget-controls reference build charges nothing, so its units are not checked.
		err := conform.RunPushes("engine", controlled && BudgetControlsEnabled, func(call, v string) (string, int, int) {
			base := len(p.KidStack)
			var before uint64
			if controlled {
				before = p.Control.Used
			}
			var err error
			if call == conform.CallPush {
				err = c.Push(conform.PushFn(call), vals[v])
			} else {
				err = p.termPushItems(c, conform.PushFn(call), vals[v])
			}
			units := 0
			if controlled {
				units = int(p.Control.Used - before)
			}
			pushed := slices.Clone(p.KidStack[base:])
			p.DropKids(base)
			if err != nil {
				return "error: " + err.Error(), len(pushed), units
			}
			if call == conform.CallPush {
				if len(pushed) != 1 || pushed[0] == nil {
					return "nil", len(pushed), units
				}
				return engineMemberCanon(pushed[0]), len(pushed), units
			}
			return engineMemberCanon(c.ListNode(pushed)), len(pushed), units
		})
		if err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}
