package core

import (
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

// engineFieldType returns the type of a cast of the table as the engine declares it.
func engineFieldType(t *conform.CastType) *compile.FieldType {
	f := &compile.FieldType{Kind: t.Kind, Name: t.Name}
	if t.Elem != nil {
		f.Elem = engineFieldType(t.Elem)
	}
	for _, a := range t.Alts {
		f.Alts = append(f.Alts, engineFieldType(a))
	}
	return f
}

// TestEngineValueCastTable runs the cast table (conform.CastCases) on the engine's compositions conforms,
// valueConforms, elemsConform and tupleConform (termConforms and the functions it calls, printed from Exec
// IR), through FieldConforms, which the backends call to check a cast.
func TestEngineValueCastTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		src := "type S = { F int }\ntype T = { F int }\n@start\nrule main = \"a\"\n"
		prog := &SharedProgram{}
		InitSharedProgram(prog, compiletest.Program(t, src, compile.Options{}))
		p := &Parser{Prog: prog, Input: NewInput(conform.BuiltinInput, parse.Bytes)}
		if controlled {
			p.Control = NewBudgetControl(nil, parse.Limits{Compute: 1 << 62})
		}
		c := &EvalCtx{P: p}
		var build func(v conform.CastValue) any
		build = func(v conform.CastValue) any {
			text := func() *parse.Node {
				return p.NewNode(value.Data{Kind: noderep.MatchKind, Terminal: true, Text: "xyz", Start: 2, End: 4})
			}
			elems := func() []*parse.Node {
				var out []*parse.Node
				for _, e := range v.Elems {
					if e.Kind == "nilnode" {
						out = append(out, nil)
						continue
					}
					n, err := element(c, "list", build(e))
					if err != nil {
						t.Fatal(err)
					}
					out = append(out, n)
				}
				return out
			}
			switch v.Kind {
			case "nil":
				return nil
			case "nilnode":
				return (*parse.Node)(nil)
			case "int":
				return 1
			case "string":
				return "s"
			case "bool":
				return true
			case "text":
				return text()
			case "tuple":
				return p.NewNode(value.Data{Kind: noderep.SeqKind, Start: 2, End: 5, Children: []*parse.Node{text(), text()}})
			case "rep":
				return p.NewNode(value.Data{Kind: noderep.ListKind, Start: 2, End: 5, Children: []*parse.Node{text(), text()}})
			case "err":
				return p.NewNode(value.Data{Kind: noderep.ErrorKind, Terminal: true, Text: "x", Start: 2, End: 4})
			case "missing":
				return p.NewNode(value.Data{Kind: noderep.MissingKind, Start: 2, End: 2})
			case "struct":
				n, err := c.NewStruct(v.Name, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				return n
			case "box":
				n, err := element(c, "list", 1)
				if err != nil {
					t.Fatal(err)
				}
				return n
			case "list":
				return c.ListNode(elems())
			case "tupleValue":
				return c.TupleNode(elems())
			}
			panic("no value " + v.Kind)
		}
		err := conform.RunCasts("engine", controlled, func(typ *conform.CastType, v conform.CastValue) (bool, int) {
			val := build(v)
			var before uint64
			if controlled {
				before = p.Control.Used
			}
			ok := p.FieldConforms(engineFieldType(typ), val)
			if controlled {
				return ok, int(p.Control.Used - before)
			}
			return ok, 0
		})
		if err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}
