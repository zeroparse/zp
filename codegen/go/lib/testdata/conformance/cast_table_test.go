package libgo

import (
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/noderep"
	"zeroparse.dev/zp/internal/ir/exec/conform"
)

// The cast table (conform.CastCases) on the compositions conforms, valueConforms, elemsConform and
// tupleConform of the node and the typed library (TermConforms and the functions it calls, printed from
// Exec IR), through the check that Cast makes.

// castFieldType returns the type of a cast of the table as the library declares it.
func castFieldType(t *conform.CastType) *FieldType {
	f := &FieldType{Kind: t.Kind, Name: t.Name}
	if t.Elem != nil {
		f.Elem = castFieldType(t.Elem)
	}
	for _, a := range t.Alts {
		f.Alts = append(f.Alts, castFieldType(a))
	}
	return f
}

// nodeCastValue builds a value of the table in the node library.
func nodeCastValue(c *ActCtx, v conform.CastValue) any {
	text := func() *core.Node {
		return nodeOfData(&Data{Kind: MatchKind, Terminal: true, Text: "xyz", Start: 2, End: 4})
	}
	elems := func() []any {
		var out []any
		for _, e := range v.Elems {
			if e.Kind == "nilnode" {
				out = append(out, nil)
				continue
			}
			out = append(out, nodeCastValue(c, e))
		}
		return out
	}
	switch v.Kind {
	case "nil":
		return nil
	case "nilnode":
		return (*core.Node)(nil)
	case "int":
		return 1
	case "string":
		return "s"
	case "bool":
		return true
	case "text":
		return text()
	case "tuple":
		return nodeOfData(&Data{Kind: SeqKind, Start: 2, End: 5, Children: []*core.Node{text(), text()}})
	case "rep":
		return nodeOfData(&Data{Kind: ListKind, Start: 2, End: 5, Children: []*core.Node{text(), text()}})
	case "err":
		return nodeOfData(&Data{Kind: noderep.ErrorKind, Terminal: true, Start: 2, End: 4, Text: "x"})
	case "struct":
		return c.fxStruct(v.Name)
	case "box":
		return nodeOfData(&Data{Kind: noderep.BoxKind, Box: 1})
	case "list":
		return c.fxList(elems()...)
	case "tupleValue":
		return c.fxTuple(elems()...)
	}
	panic("no value " + v.Kind)
}

// typedCastValue builds a value of the table in the typed library.
func typedCastValue(v conform.CastValue) any {
	text := func() *TMatch { return &TMatch{TNode: TNode[string]{Span: Span{2, 4}, Value: "xyz"}} }
	elems := func() []any {
		var out []any
		for _, e := range v.Elems {
			out = append(out, typedCastValue(e))
		}
		return out
	}
	switch v.Kind {
	case "nil", "nilnode":
		return nil
	case "int", "box":
		return 1
	case "string":
		return "s"
	case "bool":
		return true
	case "text":
		return text()
	case "tuple":
		return &TSeq{Typ: "tuple", Span: Span{2, 5}, Kids: []any{text(), text()}}
	case "rep":
		return &TSeq{Typ: "list", Span: Span{2, 5}, Kids: []any{text(), text()}}
	case "err":
		return NewTError(2, 4, "x")
	case "struct":
		return &memberStruct{name: v.Name}
	case "list":
		return &TSeq{Typ: TypeListValue, Kids: elems()}
	case "tupleValue":
		return &TSeq{Typ: TypeTupleValue, Kids: elems()}
	}
	panic("no value " + v.Kind)
}

func TestValueCastTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		g := NewGrammar(100)
		for _, s := range conform.CastStructs {
			g.DeclareStruct(s)
		}
		p := &Parser{g: g}
		tp := &Parser{}
		if controlled {
			p.Control = newBudgetControl(t.Context(), core.Limits{Compute: 1 << 62})
			tp.Control = newBudgetControl(t.Context(), core.Limits{Compute: 1 << 62})
		}
		c := &ActCtx{P: p}
		if err := conform.RunCasts("node", controlled, func(typ *conform.CastType, v conform.CastValue) (bool, int) {
			val := nodeCastValue(c, v)
			var before uint64
			if controlled {
				before = p.Control.budgetUsed()
			}
			ok := p.TermConforms(castFieldType(typ), val)
			if controlled {
				return ok, int(p.Control.budgetUsed() - before)
			}
			return ok, 0
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
		tparser := &TParser{Parser: tp}
		if err := conform.RunCasts("typed", controlled, func(typ *conform.CastType, v conform.CastValue) (bool, int) {
			val := typedCastValue(v)
			var before uint64
			if controlled {
				before = tp.Control.budgetUsed()
			}
			ok := tparser.TermConforms(castFieldType(typ), val)
			if controlled {
				return ok, int(tp.Control.budgetUsed() - before)
			}
			return ok, 0
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}
