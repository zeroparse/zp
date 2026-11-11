package core

import (
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/limits"
	"zeroparse.dev/zp/internal/value"
)

// iterFunc is a function of the iteration table: it returns the element it is passed (the last argument of a
// fold), or, if one, whether the element is the int 1.
type iterFunc struct {
	arity int
	one   bool
}

func (f iterFunc) Arity() int { return f.arity }
func (f iterFunc) Apply(x, y any) (any, error) {
	if f.arity == 2 {
		return y, nil
	}
	if f.one {
		return x == any(1), nil
	}
	return x, nil
}

// TestEngineValueIterTable runs the iteration table (conform.IterCases) on the engine's compositions itemAt and
// filterKeep (termItemAt and termFilterKeep, printed from Exec IR), through Builtin, which the backends call.
func TestEngineValueIterTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		prog := &SharedProgram{}
		InitSharedProgram(prog, compiletest.Program(t, "@start\nrule main = \"a\"\n", compile.Options{}))
		p := &Parser{Prog: prog, Input: NewInput(conform.BuiltinInput, Bytes)}
		if controlled {
			p.Control = NewBudgetControl(nil, limits.Limits{Compute: 1 << 62})
		}
		c := &EvalCtx{P: p}
		one, _ := c.Element("list", 1)
		two, _ := c.Element("list", 2)
		vals := map[string]any{
			conform.MemberNil:     nil,
			conform.MemberInt:     1,
			conform.BuiltinString: "ab",
			conform.BuiltinBool:   true,
			conform.MemberList:    c.ListNode(nil),
			conform.BuiltinPair:   c.ListNode([]*value.Node{one, two}),
		}
		err := conform.RunIters("engine", controlled, func(call, v string) (string, int) {
			var args []any
			fn := call
			switch call {
			case conform.CallFoldl, conform.CallFoldr:
				args = []any{nil, vals[v], iterFunc{arity: 2}}
			case conform.CallFilterSelf:
				fn = "filter"
				args = []any{vals[v], iterFunc{arity: 1}}
			default:
				args = []any{vals[v], iterFunc{arity: 1, one: call == conform.CallFilter}}
			}
			var before uint64
			if controlled {
				before = p.Control.Used
			}
			got, err := c.Builtin(fn, args)
			steps := 0
			if controlled {
				steps = int(p.Control.Used - before)
			}
			if err != nil {
				return "error: " + err.Error(), steps
			}
			return engineMemberCanon(got), steps
		})
		if err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}
