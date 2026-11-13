package core

import (
	"fmt"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/parse"
)

// TestEngineValueStructTable runs the struct table (conform.StructCases) on the engine's compositions
// cover and made (termCover and termMade, printed from Exec IR), through the method that the backends
// call to make a struct.
func TestEngineValueStructTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		prog := &SharedProgram{}
		InitSharedProgram(prog, compiletest.Program(t, "type S = { F0 int }\n@start\nrule main = \"a\"\n", compile.Options{}))
		p := &Parser{Prog: prog, Input: NewInput(conform.BuiltinInput, parse.Bytes)}
		if controlled {
			p.Control = NewBudgetControl(nil, parse.Limits{Compute: 1 << 62})
		}
		c := &EvalCtx{P: p, Start: conform.StructRuleStart, End: conform.StructRuleEnd}
		text := p.NewNode(value.Data{Kind: value.MatchKind, Terminal: true, Text: "xyz", Start: 2, End: 4})
		vals := map[string]any{
			conform.MemberNil:     nil,
			conform.MemberInt:     1,
			conform.BuiltinString: "ab",
			conform.BuiltinText:   text,
			conform.BuiltinNode:   p.NewNode(value.Data{Kind: value.SeqKind, Start: 2, End: 5, Children: []*value.Node{text, text}}),
			conform.StructLate:    p.NewNode(value.Data{Kind: value.MatchKind, Terminal: true, Text: "q", Start: 7, End: 9}),
		}
		err := conform.RunStructs("engine", func(fields []string) string {
			var names []string
			var vs []any
			for i, f := range fields {
				names = append(names, fmt.Sprintf("F%d", i))
				vs = append(vs, vals[f])
			}
			before := len(p.Created)
			n, err := c.NewStruct("S", names, vs)
			if err != nil {
				return "error: " + err.Error()
			}
			recorded := len(p.Created) == before+1 && p.Created[before] == n
			s := fmt.Sprintf("%d-%d", value.D(n).Start, value.D(n).End)
			if recorded {
				s += conform.StructRecorded
			}
			return s
		})
		if err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}
