package core

import (
	"fmt"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/noderep"
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
		text := p.NewNode(value.Data{Kind: noderep.MatchKind, Terminal: true, Text: "xyz", Start: 2, End: 4})
		vals := map[string]any{
			conform.MemberNil:     nil,
			conform.MemberInt:     1,
			conform.BuiltinString: "ab",
			conform.BuiltinText:   text,
			conform.BuiltinNode:   p.NewNode(value.Data{Kind: noderep.SeqKind, Start: 2, End: 5, Children: []*parse.Node{text, text}}),
			conform.StructLate:    p.NewNode(value.Data{Kind: noderep.MatchKind, Terminal: true, Text: "q", Start: 7, End: 9}),
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

// TestEngineValueStructTableTermCode runs the struct table on term code: the code the term compiler
// emits for a struct literal, with FIELD for each field, makes the struct the table wants, and FIELD
// charges what STEP, SETFIELD and COVER, which it fuses, charge.
func TestEngineValueStructTableTermCode(t *testing.T) {
	cp := compiletest.Program(t, "type S = { F0 int\n F1 int }\n@start\nrule main = \"a\"\n", compile.Options{})
	for _, controlled := range []bool{false, true} {
		prog := &SharedProgram{}
		InitSharedProgram(prog, cp)
		// literal returns the term code of a struct literal of the fields, fused or not, whose values
		// are the locals v<i>.
		literal := func(fields []string, fused bool) *TermProgram {
			tp := &TermProgram{na: int32(len(fields)) + 1, nn: 1, structs: []termStruct{{kind: cp.TypeKind("S"), fields: 2}}}
			tp.code = append(tp.code, termInstr{op: tStruct}, termInstr{op: tIntK, a: 1})
			for i := range fields {
				tp.names = append(tp.names, fmt.Sprintf("v%d", i), fmt.Sprintf("F%d", i))
				v, name := int32(i)+1, int32(2*i+1)
				tp.code = append(tp.code, termInstr{op: tLocal, dst: v, a: name - 1})
				if fused {
					tp.code = append(tp.code, termInstr{op: tField, a: 0, b: v, c: name})
				} else {
					tp.code = append(tp.code, termInstr{op: tStep}, termInstr{op: tSetField, a: 0, b: v, c: name}, termInstr{op: tCover, a: 0, b: v})
				}
			}
			tp.code = append(tp.code, termInstr{op: tMade, b: 0}, termInstr{op: tRet})
			return tp
		}
		text := value.Ptr(value.Data{Kind: noderep.MatchKind, Terminal: true, Text: "xyz", Start: 2, End: 4})
		vals := map[string]any{
			conform.MemberNil:     nil,
			conform.MemberInt:     1,
			conform.BuiltinString: "ab",
			conform.BuiltinText:   text,
			conform.BuiltinNode:   value.Ptr(value.Data{Kind: noderep.SeqKind, Start: 2, End: 5, Children: []*parse.Node{text, text}}),
			conform.StructLate:    value.Ptr(value.Data{Kind: noderep.MatchKind, Terminal: true, Text: "q", Start: 7, End: 9}),
		}
		err := conform.RunStructs("engine term code", func(fields []string) string {
			var got [2]string
			var used [2]uint64
			var fieldsOf [2]string // the same values in both, so the same pointers
			for k, fused := range []bool{true, false} {
				p := &Parser{Prog: prog, Input: NewInput(conform.BuiltinInput, parse.Bytes)}
				if controlled {
					p.Control = NewBudgetControl(nil, parse.Limits{Compute: 1 << 62})
				}
				var locals *Local
				for i, f := range fields {
					locals = &Local{Name: fmt.Sprintf("v%d", i), Val: vals[f], Next: locals}
				}
				c := &EvalCtx{P: p, Start: conform.StructRuleStart, End: conform.StructRuleEnd, Locals: locals}
				v, err := literal(fields, fused).Eval(c)
				if err != nil {
					return "error: " + err.Error()
				}
				n := v.(*parse.Node)
				got[k] = fmt.Sprintf("%d-%d", value.D(n).Start, value.D(n).End)
				if len(p.Created) == 1 && p.Created[0] == n {
					got[k] += conform.StructRecorded
				}
				fieldsOf[k] = fmt.Sprint(value.D(n).Fields)
				if controlled {
					used[k] = p.Control.Used
				}
			}
			if got[0] != got[1] || used[0] != used[1] || fieldsOf[0] != fieldsOf[1] {
				return fmt.Sprintf("fused %s %s (%d units), unfused %s %s (%d units)", got[0], fieldsOf[0], used[0], got[1], fieldsOf[1], used[1])
			}
			return got[0]
		})
		if err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}
