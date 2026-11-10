package core

import (
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/limits"
	"zeroparse.dev/zp/internal/value"
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
		p := &Parser{Prog: prog, Input: NewInput(conform.BuiltinInput, Bytes)}
		if controlled {
			p.Control = NewBudgetControl(nil, limits.Limits{Compute: 1 << 62})
		}
		c := &EvalCtx{P: p}
		var build func(v conform.CastValue) any
		build = func(v conform.CastValue) any {
			text := func() *value.Node {
				return p.NewNode(value.Data{Kind: value.MatchKind, Terminal: true, Text: "xyz", Start: 2, End: 4})
			}
			elems := func() []*value.Node {
				var out []*value.Node
				for _, e := range v.Elems {
					if e.Kind == "nilnode" {
						out = append(out, nil)
						continue
					}
					n, err := c.Element("list", build(e))
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
				return (*value.Node)(nil)
			case "int":
				return 1
			case "string":
				return "s"
			case "bool":
				return true
			case "text":
				return text()
			case "tuple":
				return p.NewNode(value.Data{Kind: value.SeqKind, Start: 2, End: 5, Children: []*value.Node{text(), text()}})
			case "rep":
				return p.NewNode(value.Data{Kind: value.ListKind, Start: 2, End: 5, Children: []*value.Node{text(), text()}})
			case "err":
				return p.NewNode(value.Data{Kind: value.ErrorKind, Terminal: true, Text: "x", Start: 2, End: 4})
			case "missing":
				return p.NewNode(value.Data{Kind: value.MissingKind, Start: 2, End: 2})
			case "struct":
				n, err := c.NewStruct(v.Name, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				return n
			case "box":
				n, err := c.Element("list", 1)
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
