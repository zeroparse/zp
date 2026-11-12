package ref

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/grammar"
)

// TermEnv is what a term reads: the captures of the rule, the unit of positions and the place of the
// evaluation. The reference evaluator covers every term of a value or predicate action: literals,
// captures and lambda parameters, the members of nodes (.Start, .End, .Value and captures) and of
// structs, struct, tuple and named node constructors, the built-in functions, the operators and casts
// to every type (spec/language/actions.md). It does not model the values of error recovery.
type TermEnv struct {
	// Captures are the captures by name, made with Node; a capture that did not match is absent.
	Captures map[string]V
	// Bytes sets the unit of positions and lengths to bytes instead of code points.
	Bytes bool
	// Start and End are the range of the match that a struct without nodes takes as its own.
	Start, End int
	// Pos, Line and Col are the input position where the evaluation started.
	Pos, Line, Col int
	// Zeros are the zero values of the fields of the struct types by type and field name: what a field
	// that a constructor left unset reads as (spec/language/types.md#zero-values).
	Zeros map[string]map[string]V
	// Vars are the values of the grammar's variables by name; reading a variable that is absent fails.
	Vars map[string]V
	// Aliases are the type aliases of the grammar by name: a cast or a constructor that names one
	// means the type that it stands for (spec/language/types.md#type-aliases).
	Aliases map[string]grammar.TypeExpr
}

// failure is the panic value of an evaluation error.
type failure string

// Eval evaluates the term t and returns its value and the text of the error that stopped it, "" if
// none did.
func (e *TermEnv) Eval(t grammar.Term) (v V, err string) {
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(unknownSignal); ok {
				v, err = Unknown, ""
				return
			}
			f, ok := x.(failure)
			if !ok {
				panic(x)
			}
			v, err = Nil, string(f)
		}
	}()
	return e.eval(t, nil), ""
}

type params map[string]V

func (p params) with(names []string, vals ...V) params {
	out := params{}
	for k, v := range p {
		out[k] = v
	}
	for i, n := range names {
		out[n] = vals[i]
	}
	return out
}

func fail(format string, args ...any) { panic(failure(fmt.Sprintf(format, args...))) }

func (e *TermEnv) eval(t grammar.Term, p params) V {
	switch t := t.(type) {
	case *grammar.TermGroup:
		return e.eval(t.Term, p)
	case *grammar.IntLit:
		return Int(int64(t.Value))
	case *grammar.StringLit:
		return Str(t.Value)
	case *grammar.BoolLit:
		return Bool(t.Value)
	case *grammar.NilLit:
		return Nil
	case *grammar.CaptureRef:
		if t.Param {
			return p[t.Name]
		}
		return e.Captures[t.Name]
	case *grammar.VarRef:
		v, ok := e.Vars[t.Name]
		if !ok {
			fail("undefined variable %s", t.Name)
		}
		return v
	case *grammar.Member:
		return e.member(e.eval(t.X, p), t.Name)
	case *grammar.New:
		if len(t.Args) > 0 {
			// N{x} converts the node x to the named node type N: it keeps the range and the content
			// of x and carries the name N (types.md, Named Node Types). The checker admits only nodes.
			x := e.eval(t.Args[0], p)
			need(x)
			if x.Kind != KNode {
				panic("ref: conversion of a value that is not a node: " + x.TypeName())
			}
			x.Type = e.typeName(t.Type)
			return x
		}
		s := V{Kind: KStruct, Type: e.typeName(t.Type), Fields: map[string]V{}}
		for _, f := range t.Fields {
			s.Fields[f.Name] = e.eval(f.Value, p)
		}
		return s
	case *grammar.TupleLit:
		return Tuple(e.terms(t.Elems, p)...)
	case *grammar.Call:
		return e.call(t, p)
	case *grammar.Binary:
		return e.binary(t, p)
	case *grammar.Unary:
		x := e.eval(t.X, p)
		need(x)
		v, msg := Unary(t.Op, x)
		if msg != "" {
			fail("%s", msg)
		}
		return v
	case *grammar.Cast:
		return e.cast(t, e.eval(t.X, p))
	}
	panic(fmt.Sprintf("ref: unsupported term %T", t))
}

func (e *TermEnv) terms(ts []grammar.Term, p params) []V {
	out := make([]V, len(ts))
	for i, t := range ts {
		out[i] = e.eval(t, p)
	}
	return out
}

func (e *TermEnv) member(x V, name string) V {
	need(x)
	switch x.Kind {
	case KNode:
		switch name {
		case "Start":
			return Int(int64(x.Start))
		case "End":
			return Int(int64(x.End))
		case "Value":
			return x.nodeValue()
		}
		if cap, ok := strings.CutPrefix(name, "$"); ok {
			if x.Shape == UnknownShape && x.Caps == nil {
				need(Unknown)
			}
			return x.Caps[cap] // a capture that did not match reads as nil
		}
	case KStruct:
		if v, ok := x.Fields[name]; ok {
			return v
		}
		// A field never set reads as the zero value of its declared type.
		z, ok := e.Zeros[x.Type][name]
		if !ok {
			fail("ref: the zero value of field %s of %s is not declared", name, x.Type)
		}
		return z
	}
	fail("cannot select %s from %s", name, x.TypeName())
	return Nil
}

func (e *TermEnv) binary(t *grammar.Binary, p params) V {
	if t.Op == "&&" || t.Op == "||" {
		return e.logic(t, p)
	}
	l := e.eval(t.L, p)
	r := e.eval(t.R, p)
	need(l, r)
	v, msg := Binary(t.Op, l, r) // "[]" indexes a list or a tuple
	if msg != "" {
		fail("%s", msg)
	}
	return v
}

// strictLogic applies && or || as the language does: the left operand first, and the right one only
// if the left one does not decide.
func (e *TermEnv) strictLogic(t *grammar.Binary, p params) V {
	lv := e.eval(t.L, p)
	need(lv)
	l, msg := Truth(t.Op, lv)
	if msg != "" {
		fail("%s", msg)
	}
	if l == (t.Op == "||") {
		return Bool(l) // the right operand decides nothing
	}
	rv := e.eval(t.R, p)
	need(rv)
	r, msg := Truth(t.Op, rv)
	if msg != "" {
		fail("%s", msg)
	}
	return Bool(r)
}

// items returns the elements of a list value or a list node; nil is the empty list.
func (e *TermEnv) items(fn string, v V) []V {
	need(v)
	switch v.Kind {
	case KNil:
		return nil
	case KList:
		return v.Items
	case KNode:
		switch v.Shape {
		case ListNode:
			return v.Items
		case UnknownShape:
			need(Unknown)
		}
	}
	fail("%s: expected a list, got %s", fn, v.TypeName())
	return nil
}

func (e *TermEnv) call(c *grammar.Call, p params) V {
	lambda := func(i int) *grammar.Lambda { return c.Args[i].(*grammar.Lambda) }
	apply := func(f *grammar.Lambda, args ...V) V { return e.eval(f.Body, p.with(f.Params, args...)) }
	switch c.Func {
	case "len":
		x := e.eval(c.Args[0], p)
		need(x)
		switch x.Kind {
		case KNil:
			return Int(0)
		case KStr:
			return Int(int64(unitLen(x.S, e.Bytes)))
		case KNode:
			if x.Shape == UnknownShape {
				need(Unknown)
			}
			if x.Shape != StringNode {
				return Int(int64(len(x.Items))) // a list or a tuple node
			}
			return Int(int64(x.End - x.Start)) // a string node
		case KList, KTuple:
			return Int(int64(len(x.Items)))
		}
		fail("len: invalid argument %s", x.TypeName())
	case "text":
		x := e.eval(c.Args[0], p)
		need(x)
		switch x.Kind {
		case KNil:
			return Str("")
		case KStr, KNode:
			return Str(x.S)
		}
		fail("text: invalid argument %s", x.TypeName())
	case "foldl", "foldr":
		acc := e.eval(c.Args[0], p)
		items := e.items(c.Func, e.eval(c.Args[1], p))
		for i := range items {
			it := items[i]
			if c.Func == "foldr" {
				it = items[len(items)-1-i]
			}
			acc = apply(lambda(2), acc, it)
		}
		return acc
	case "map":
		var out []V
		for _, it := range e.items("map", e.eval(c.Args[0], p)) {
			out = append(out, apply(lambda(1), it))
		}
		return List(out...)
	case "filter":
		var out []V
		for _, it := range e.items("filter", e.eval(c.Args[0], p)) {
			keep := apply(lambda(1), it)
			need(keep)
			if keep.Kind != KBool {
				fail("filter: predicate must produce a bool, got %s", keep.TypeName())
			}
			if keep.B {
				out = append(out, it)
			}
		}
		return List(out...)
	case "list":
		return List(e.terms(c.Args, p)...)
	case "concat":
		var out []V
		for _, a := range e.terms(c.Args, p) {
			out = append(out, e.items("concat", a)...)
		}
		return List(out...)
	}
	panic("ref: unsupported built-in " + c.Func)
}

func (e *TermEnv) cast(c *grammar.Cast, x V) V {
	if !e.admits(c.Type, x) {
		fail("cannot cast %s to %s at input position %d (line %d, column %d)", x.TypeName(), castName(c.Type), e.Pos, e.Line, e.Col)
	}
	return x
}

// castName is the type of a cast as an error message shows it: as written, without the parentheses
// that group the whole type.
func castName(t grammar.TypeExpr) string {
	for {
		g, ok := t.(*grammar.TypeGroup)
		if !ok {
			return grammar.FormatType(t)
		}
		t = g.Type
	}
}

// typeName is the name of the type that the name n, written in a constructor, means: the name of the
// struct type or named node type that n stands for through aliases, and n itself if it is not an alias.
func (e *TermEnv) typeName(n string) string {
	for {
		t, ok := e.Aliases[n]
		if !ok {
			return n
		}
		for {
			g, ok := t.(*grammar.TypeGroup)
			if !ok {
				break
			}
			t = g.Type
		}
		r, ok := t.(*grammar.TypeRef)
		if !ok {
			return n
		}
		n = r.Name
	}
}

// admits reports whether the type t admits the value x (actions.md, Values and Constructors): a name
// other than a built-in one or an alias is a declared struct type or named node type, which admits
// only its own values. The model has no values of error recovery, so error admits none.
func (e *TermEnv) admits(t grammar.TypeExpr, x V) bool {
	need(x)
	switch t := t.(type) {
	case *grammar.TypeGroup:
		return e.admits(t.Type, x)
	case *grammar.TypeRef:
		if a, ok := e.Aliases[t.Name]; ok {
			return e.admits(a, x)
		}
		switch t.Name {
		case "_":
			return true
		case "int":
			return x.Kind == KInt
		case "string":
			return x.Kind == KStr
		case "bool":
			return x.Kind == KBool
		case "error":
			return false
		}
		if x.Kind == KNode && x.Shape == UnknownShape {
			need(Unknown) // the type of the node is unknown
		}
		return (x.Kind == KStruct || x.Kind == KNode) && x.Type == t.Name
	case *grammar.OptionalType:
		return x.Kind == KNil || e.admits(t.Elem, x)
	case *grammar.JoinType:
		for _, m := range t.Types {
			if e.admits(m, x) {
				return true
			}
		}
		return false
	case *grammar.MeetType:
		for _, m := range t.Types {
			if !e.admits(m, x) {
				return false
			}
		}
		return true
	case *grammar.ListType:
		return x.Kind == KList && e.all(x.Items, t.Elem)
	case *grammar.TupleType:
		return x.Kind == KTuple && len(x.Items) == len(t.Types) && e.each(x.Items, t.Types)
	case *grammar.TypeApp:
		// node[U] admits a node, named or not, whose value U admits.
		if t.Name == "node" && len(t.Args) == 1 {
			return x.Kind == KNode && e.admits(t.Args[0], x.nodeValue())
		}
	}
	panic(fmt.Sprintf("ref: unsupported cast type %s", grammar.FormatType(t)))
}

func (e *TermEnv) all(xs []V, t grammar.TypeExpr) bool {
	for _, x := range xs {
		if !e.admits(t, x) {
			return false
		}
	}
	return true
}

func (e *TermEnv) each(xs []V, ts []grammar.TypeExpr) bool {
	for i, x := range xs {
		if !e.admits(ts[i], x) {
			return false
		}
	}
	return true
}

// Plain returns v as the plain Go value that a parse result shows in JSON: a struct that is the result
// is an object with the range of the match, and any other struct has the range of its fields
// (structRange).
func (e *TermEnv) Plain(v V) any { return e.plain(v, true) }

func (e *TermEnv) plain(v V, result bool) any {
	switch v.Kind {
	case KNil:
		return nil
	case KInt:
		return v.I
	case KStr:
		return v.S
	case KBool:
		return v.B
	case KList, KTuple:
		out := make([]any, len(v.Items))
		for i, x := range v.Items {
			out[i] = e.plain(x, false)
		}
		return out
	case KNode:
		out := map[string]any{"start": v.Start, "end": v.End, "value": v.S}
		if v.Shape != StringNode {
			out["value"] = e.plain(v.nodeValue(), false)
		}
		if v.Type != "" {
			out["type"] = v.Type
		}
		return out
	case KStruct:
		fields := map[string]any{}
		for k, x := range v.Fields {
			fields[k] = e.plain(x, false)
		}
		start, end := e.Start, e.End
		if !result {
			start, end = e.structRange(v)
		}
		return map[string]any{"type": v.Type, "start": start, "end": end, "fields": fields}
	}
	panic(fmt.Sprintf("ref: no plain form of %s", v.TypeName()))
}

// structRange is the range of a struct: the range that covers the ranges of the fields whose values
// are nodes, or the range of the rule if there are none (spec/runtime/nodes.md, Node Model).
func (e *TermEnv) structRange(v V) (start, end int) {
	first := true
	for _, x := range v.Fields {
		if x.Kind != KList && x.Kind != KTuple && x.Kind != KStruct && x.Kind != KNode {
			continue // a scalar or nil field is not a node
		}
		s, t := e.nodeRange(x)
		if first || s < start {
			start = s
		}
		if first || t > end {
			end = t
		}
		first = false
	}
	if first {
		return e.Start, e.End
	}
	return start, end
}

// nodeRange is the range of a value as a node: the range of a capture, the range that covers the
// elements of a list or a tuple that are not nil, which is empty at the start of the rule when there
// are none, and the range of a struct. An int, a string or a bool, which an element holds as a node of
// its own, has the range of the rule.
func (e *TermEnv) nodeRange(v V) (start, end int) {
	switch v.Kind {
	case KNode:
		return v.Start, v.End
	case KStruct:
		return e.structRange(v)
	case KList, KTuple:
		first := true
		for _, x := range v.Items {
			if x.Kind == KNil {
				continue
			}
			s, t := e.nodeRange(x)
			if first || s < start {
				start = s
			}
			if first || t > end {
				end = t
			}
			first = false
		}
		if first {
			return e.Start, e.Start
		}
		return start, end
	}
	return e.Start, e.End
}

// Holds reports whether a predicate whose term gave v, or failed with the error msg, succeeds: it
// fails if the result is false or the evaluation is an error, and succeeds with any other value, nil
// included (predicates.md, Predicate Actions).
func Holds(v V, msg string) bool { return msg == "" && !(v.Kind == KBool && !v.B) }
