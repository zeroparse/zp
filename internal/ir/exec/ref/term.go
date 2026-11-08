package ref

import (
	"fmt"

	"zeroparse.dev/zp/grammar"
)

// TermEnv is what a term reads: the captures of the rule, the unit of positions and the place of the
// evaluation. The reference evaluator covers the terms that a generated term uses: literals, captures
// and lambda parameters, members of nodes and structs, struct and tuple constructors, the built-in
// functions, the operators and casts to int and string.
type TermEnv struct {
	// Captures are the captures by name; a string node is Node(text, start, end).
	Captures map[string]V
	// Bytes sets the unit of positions and lengths to bytes instead of code points.
	Bytes bool
	// Start and End are the range of the match that a struct without nodes takes as its own.
	Start, End int
	// Pos, Line and Col are the input position where the evaluation started.
	Pos, Line, Col int
}

// Node returns the value of a string node of the input.
func Node(text string, start, end int) V { return V{Kind: KNode, S: text, Start: start, End: end} }

// failure is the panic value of an evaluation error.
type failure string

// Eval evaluates the term t and returns its value and the text of the error that stopped it, "" if
// none did.
func (e *TermEnv) Eval(t grammar.Term) (v V, err string) {
	defer func() {
		if x := recover(); x != nil {
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
	case *grammar.Member:
		return e.member(e.eval(t.X, p), t.Name)
	case *grammar.New:
		if len(t.Args) > 0 {
			fail("ref: node constructors are not modeled")
		}
		s := V{Kind: KStruct, Type: t.Type, Fields: map[string]V{}}
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
		v, msg := Unary(t.Op, e.eval(t.X, p))
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
	switch x.Kind {
	case KNode:
		switch name {
		case "Start":
			return Int(int64(x.Start))
		case "End":
			return Int(int64(x.End))
		}
	case KStruct:
		return x.Fields[name] // a field never set is nil
	}
	fail("cannot select %s from %s", name, x.TypeName())
	return Nil
}

func (e *TermEnv) binary(t *grammar.Binary, p params) V {
	if t.Op == "&&" || t.Op == "||" {
		l, msg := Truth(t.Op, e.eval(t.L, p))
		if msg != "" {
			fail("%s", msg)
		}
		if l == (t.Op == "||") {
			return Bool(l) // the right operand decides nothing
		}
		r, msg := Truth(t.Op, e.eval(t.R, p))
		if msg != "" {
			fail("%s", msg)
		}
		return Bool(r)
	}
	l := e.eval(t.L, p)
	r := e.eval(t.R, p)
	v, msg := Binary(t.Op, l, r) // "[]" indexes a list or a tuple
	if msg != "" {
		fail("%s", msg)
	}
	return v
}

// items returns the elements of a list; nil is the empty list.
func (e *TermEnv) items(fn string, v V) []V {
	switch v.Kind {
	case KNil:
		return nil
	case KList:
		return v.Items
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
		switch x.Kind {
		case KNil:
			return Int(0)
		case KStr:
			return Int(int64(unitLen(x.S, e.Bytes)))
		case KNode:
			if x.Items != nil {
				return Int(int64(len(x.Items))) // a list or a tuple node
			}
			return Int(int64(x.End - x.Start)) // a string node
		case KList, KTuple:
			return Int(int64(len(x.Items)))
		}
		fail("len: invalid argument %s", x.TypeName())
	case "text":
		x := e.eval(c.Args[0], p)
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
	ref, ok := c.Type.(*grammar.TypeRef)
	if !ok {
		panic(fmt.Sprintf("ref: unsupported cast type %T", c.Type))
	}
	fits := false
	switch ref.Name {
	case "int":
		fits = x.Kind == KInt
	case "string":
		fits = x.Kind == KStr
	case "bool":
		fits = x.Kind == KBool
	default:
		// A struct type, or another type that the generated terms do not cast to: no value other
		// than a struct of that type is one.
		fits = x.Kind == KStruct && x.Type == ref.Name
	}
	if !fits {
		fail("cannot cast %s to %s at input position %d (line %d, column %d)", x.TypeName(), ref.Name, e.Pos, e.Line, e.Col)
	}
	return x
}

// Plain returns v as the plain Go value that a parse result shows in JSON: a struct is an object with
// the range of the match.
func (e *TermEnv) Plain(v V) any {
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
			out[i] = e.Plain(x)
		}
		return out
	case KStruct:
		fields := map[string]any{}
		for k, x := range v.Fields {
			fields[k] = e.Plain(x)
		}
		return map[string]any{"type": v.Type, "start": e.Start, "end": e.End, "fields": fields}
	}
	panic("ref: a node is not a result")
}

// Holds reports whether a predicate whose term gave v, or failed with the error msg, succeeds: it
// fails if the result is false or the evaluation is an error, and succeeds with any other value, nil
// included (predicates.md, Predicate Actions).
func Holds(v V, msg string) bool { return msg == "" && !(v.Kind == KBool && !v.B) }
