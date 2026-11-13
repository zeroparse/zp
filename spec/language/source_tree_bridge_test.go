package language_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	rt "zeroparse.dev/zp/codegen/go/lib"
	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax"
	"zeroparse.dev/zp/internal/syntax/grammarparser/types"
)

// The adapter reads the typed values of the generated parser of zp.zp. A
// changed copy of zp.zp has no generated parser, so the tests that run such
// copies parse with the engine and move the tree of the engine into the
// typed values by the names of the types and fields, which zp.zp declares for
// both. This file holds that move; it is part of the tests, not a frontend.

// treeTypes are the struct and node types of the typed values, by name.
var treeTypes = func() map[string]reflect.Type {
	m := map[string]reflect.Type{}
	for _, v := range []any{
		&types.ActionBind{},
		&types.ActionBlock{},
		&types.ActionGroup{},
		&types.ActionTuple{},
		&types.Atomic{},
		&types.Attr{},
		&types.AttrArgs{},
		&types.AttrNamed{},
		&types.AttrSlot{},
		&types.Attributed{},
		&types.BinaryOp{},
		&types.Call{},
		&types.Capture{},
		&types.CaptureAccess{},
		&types.Cast{},
		&types.CharClass{},
		&types.CharRng{},
		&types.Choice{},
		&types.CommentGap{},
		&types.CommentGroup{},
		&types.Constructor{},
		&types.Discard{},
		&types.Field{},
		&types.FieldAccess{},
		&types.FieldBody{},
		&types.File{},
		&types.GapFlag{},
		&types.IgnoreCase{},
		&types.Infix{},
		&types.InitField{},
		&types.InitFieldBody{},
		&types.InlineAction{},
		&types.InlineComment{},
		&types.JoinType{},
		&types.Lambda{},
		&types.LevelCall{},
		&types.ListConstructor{},
		&types.ListType{},
		&types.Lookahead{},
		&types.MeetType{},
		&types.MissingPragma{},
		&types.OneOrMore{},
		&types.OperatorsLevel{},
		&types.OperatorsLevelBody{},
		&types.OperatorsOperand{},
		&types.OperatorsOperandBody{},
		&types.OperatorsOperatorBody{},
		&types.OperatorsRule{},
		&types.OperatorsSkip{},
		&types.OperatorsSkipBody{},
		&types.Optional{},
		&types.OptionalType{},
		&types.Pkg{},
		&types.PkgBody{},
		&types.Postfix{},
		&types.Predicate{},
		&types.Prefix{},
		&types.Recover{},
		&types.Repeat{},
		&types.RepeatBounds{},
		&types.RuleDef{},
		&types.RuleDefBody{},
		&types.RuleGroup{},
		&types.RuleRef{},
		&types.Sequence{},
		&types.SourceBoundary{},
		&types.SourceComment{},
		&types.SourceItem{},
		&types.SourceLead{},
		&types.SourceMetadata{},
		&types.Stream{},
		&types.StructType{},
		&types.TupleType{},
		&types.TypeAlias{},
		&types.TypeAliasBody{},
		&types.TypeApp{},
		&types.TypeDef{},
		&types.TypeDefBody{},
		&types.TypeGroup{},
		&types.TypeRef{},
		&types.UnaryOp{},
		&types.VarDef{},
		&types.VarDefBody{},
		&types.ZeroOrMore{},
		&types.Any{},
		&types.BeginInput{},
		&types.BeginLine{},
		&types.Bottom{},
		&types.CaptureRef{},
		&types.Char{},
		&types.Cut{},
		&types.DefaultAction{},
		&types.EndInput{},
		&types.EndLine{},
		&types.False{},
		&types.Ident{},
		&types.Int{},
		&types.Nil{},
		&types.String{},
		&types.Top{},
		&types.True{},
		&types.VarRef{},
	} {
		t := reflect.TypeOf(v)
		m[t.Elem().Name()] = t
	}
	return m
}()

// TestTreeTypesComplete requires treeTypes to name every exported struct type
// of the typed values, so that a type that zp.zp adds is not missed.
func TestTreeTypesComplete(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "../../internal/syntax/grammarparser/types/types.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.TYPE {
			continue
		}
		for _, s := range g.Specs {
			ts := s.(*ast.TypeSpec)
			if _, ok := ts.Type.(*ast.StructType); !ok || !ts.Name.IsExported() {
				continue
			}
			if _, ok := treeTypes[ts.Name.Name]; !ok {
				missing = append(missing, ts.Name.Name)
			}
		}
	}
	if len(missing) > 0 {
		t.Errorf("treeTypes misses %v", missing)
	}
}

var (
	spanType = reflect.TypeOf(rt.Span{})
)

// typedTree moves the File node root of an engine parse into the typed values.
func typedTree(t testing.TB, root *core.Node) *types.File {
	t.Helper()
	f, _ := typedValue(t, root, reflect.TypeOf((*types.File)(nil))).Interface().(*types.File)
	return f
}

// typedValue returns the value v of the engine tree as a value of type typ.
func typedValue(t testing.TB, v any, typ reflect.Type) reflect.Value {
	t.Helper()
	out := reflect.New(typ).Elem()
	n, _ := v.(*core.Node)
	if v == nil || n == nil && typ.Kind() != reflect.String && typ.Kind() != reflect.Bool {
		return out
	}
	switch typ.Kind() {
	case reflect.String:
		if n != nil {
			out.SetString(n.Text())
		} else {
			out.SetString(v.(string))
		}
	case reflect.Bool:
		b, _ := v.(bool)
		out.SetBool(b)
	case reflect.Slice:
		for _, e := range n.Elems() {
			if typ.Elem().Kind() == reflect.String {
				s, _ := e.Value().(string)
				out = reflect.Append(out, reflect.ValueOf(s))
				continue
			}
			out = reflect.Append(out, typedValue(t, e, typ.Elem()))
		}
	case reflect.Interface:
		var p reflect.Value
		if n.Kind() == core.KindError {
			p = reflect.ValueOf(&types.Error{Span: span(n), Message: n.Text()})
		} else if pt, ok := treeTypes[n.Type()]; ok {
			p = typedValue(t, n, pt)
		} else {
			t.Fatalf("the engine tree has a value of type %q, which has no typed value", n.Type())
		}
		if !p.Type().Implements(typ) {
			t.Fatalf("the typed value %s is not a %s", p.Type(), typ)
		}
		out.Set(p)
	case reflect.Pointer:
		if n.Kind() == core.KindError {
			return out // an error in a place of a struct type is nil
		}
		p := reflect.New(typ.Elem())
		fill(t, n, p.Elem())
		out.Set(p)
	default:
		t.Fatalf("no typed value of Go type %s", typ)
	}
	return out
}

func span(n *core.Node) rt.Span { return rt.Span{Start: n.Start(), End: n.End()} }

// fill sets the struct s, a struct or node type of the typed values, from n.
func fill(t testing.TB, n *core.Node, s reflect.Value) {
	t.Helper()
	st := s.Type()
	if st.NumField() == 1 && st.Field(0).Anonymous && st.Field(0).Type != spanType {
		// A node type: struct{ TNode[string] }.
		fill(t, n, s.Field(0))
		return
	}
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		switch {
		case !f.IsExported():
		case f.Type == spanType:
			s.Field(i).Set(reflect.ValueOf(span(n)))
		case f.Name == "Value" && f.Type.Kind() == reflect.String:
			s.Field(i).SetString(n.Text())
		default:
			s.Field(i).Set(typedValue(t, n.Field(strings.TrimSuffix(f.Name, "_")), f.Type))
		}
	}
}

// bridgedFrontend returns the frontend of a changed copy of zp.zp compiled by
// the engine: the parse, the move into the typed values and the adapter.
func bridgedFrontend(t testing.TB, p *engine.Parser) func(string) (*grammar.Grammar, error) {
	return func(src string) (*grammar.Grammar, error) {
		text, invalid := syntax.WithoutInvalidCharacters(src)
		n, err := p.Parse(text)
		if err != nil {
			return nil, canonicalErrors(src, err)
		}
		if invalid {
			return nil, syntax.SourceErrors(src, nil)
		}
		return syntax.FromSourceTree(src, typedTree(t, n))
	}
}
