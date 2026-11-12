package sample

import (
	"unicode/utf8"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// Kinds of generated values (see val).
const (
	vNone    = iota // no value: lookahead, discard, anchors, predicates
	vNil            // an optional expression or a capture that did not match
	vText           // a string node
	vList           // a list node: a repetition
	vNode           // another node, whose elements the generator does not track (Seq, ...)
	vUnknown        // the result of an action, or of a OperatorsRule expression
)

// val describes the value that the parser will build for a generated expression, as far as the
// generator can tell. Predicates read it through captures, in the form of the reference evaluator of
// terms (see refValue).
type val struct {
	kind       int
	start, end int      // the range of the input it covers, in bytes
	items      []val    // the values of the iterations of a list
	caps       *capture // the captures of a node of a rule or of an iteration
	// capsKnown is set on a node whose captures are caps: the node of a rule or of an iteration.
	capsKnown bool
}

// capture is a capture made in the current scope (a persistent list).
type capture struct {
	name string
	v    val
	next *capture
	// ref caches the value in the form of the reference evaluator: the text and the range of a value
	// do not change once it is generated.
	ref *ref.V
}

// variable is a variable definition (a persistent list; the nearest definition comes first).
type variable struct {
	name string
	v    ref.V
	next *variable
}

// zeroValue is the zero value of a variable or a field of the type t, Unknown where the generator
// does not tell it.
func zeroValue(t grammar.TypeExpr) ref.V {
	if r, ok := t.(*grammar.TypeRef); ok {
		switch r.Name {
		case "int":
			return ref.Int(0)
		case "string":
			return ref.Str("")
		case "bool":
			return ref.Bool(false)
		}
	}
	return ref.Unknown
}

// terms holds what the reference evaluator needs of the grammar besides the input: the zero values of
// the fields of its struct types, its aliases, and the initial values of its variables.
type terms struct {
	zeros    map[string]map[string]ref.V
	aliases  map[string]grammar.TypeExpr
	initVars *variable
}

func newTerms(g *grammar.Grammar) *terms {
	t := &terms{zeros: map[string]map[string]ref.V{}, aliases: map[string]grammar.TypeExpr{}}
	for _, d := range g.Types() {
		switch d := d.(type) {
		case *grammar.TypeDef:
			if s, ok := d.Type.(*grammar.StructType); ok {
				z := map[string]ref.V{}
				for _, f := range s.Fields {
					z[f.Name] = zeroValue(f.Type)
				}
				t.zeros[d.Name] = z
			}
		case *grammar.TypeAlias:
			t.aliases[d.Name] = d.Type
		}
	}
	// The variables in order: an initializer may read those declared before it.
	for _, v := range g.Vars() {
		init := zeroValue(v.Type)
		if v.Value != nil {
			init = ref.Unknown
			env := &ref.TermEnv{Zeros: t.zeros, Aliases: t.aliases, Vars: t.vars(t.initVars)}
			if x, failed, known := evaluate(env, v.Value); known && !failed {
				init = x
			}
		}
		t.initVars = &variable{name: v.Name, v: init, next: t.initVars}
	}
	return t
}

// vars returns the values of the variables of the list vs.
func (*terms) vars(vs *variable) map[string]ref.V {
	m := map[string]ref.V{}
	for v := vs; v != nil; v = v.next {
		if _, ok := m[v.name]; !ok {
			m[v.name] = v.v
		}
	}
	return m
}

// evaluate evaluates the term t with the reference evaluator. known is false when the value or the
// failure depends on what the generator does not know, or when the evaluator does not model t.
func evaluate(env *ref.TermEnv, t grammar.Term) (v ref.V, failed, known bool) {
	defer func() {
		if recover() != nil {
			v, failed, known = ref.Unknown, false, false
		}
	}()
	v, msg := env.Eval(t)
	return v, msg != "", v.Kind != ref.KUnknown
}

// decide decides the predicate term t with the reference evaluator (see evaluate for known).
func decide(env *ref.TermEnv, t grammar.Term) (holds, known bool) {
	defer func() {
		if recover() != nil {
			holds, known = false, false
		}
	}()
	return env.Decide(t)
}

// env returns the environment of a term at the current point of the generation: the captures of the
// scope, the variables and the grammar's types. Positions are in code points, the default unit of the
// parser that checks the candidates.
func (g *gen) env() *ref.TermEnv {
	caps := map[string]ref.V{}
	for c := g.caps; c != nil; c = c.next {
		if _, ok := caps[c.name]; ok {
			continue // the nearest capture of the name comes first
		}
		if c.ref == nil {
			v := g.refValue(c.v, true)
			c.ref = &v
		}
		caps[c.name] = *c.ref
	}
	t := g.in.terms
	return &ref.TermEnv{Captures: caps, Vars: t.vars(g.vars), Zeros: t.zeros, Aliases: t.aliases}
}

// refValue returns v in the form of the reference evaluator, as the value of a capture or else as an
// element of a list. Where the generator cannot tell the value that the parser builds, it is Unknown,
// or a node of ref.UnknownShape when only its range, text and perhaps captures are known.
func (g *gen) refValue(v val, capture bool) ref.V {
	switch v.kind {
	case vNil:
		if capture {
			return ref.Nil // a capture that did not match
		}
	case vText, vList, vNode:
		text := g.slice(v.start, v.end)
		start := g.units(v.start)
		end := start + utf8.RuneCountInString(text)
		switch v.kind {
		case vText:
			return ref.Node(ref.StringNode, text, start, end, nil, nil)
		case vList:
			items := make([]ref.V, len(v.items))
			for i, it := range v.items {
				items[i] = g.refValue(it, false)
			}
			return ref.Node(ref.ListNode, text, start, end, items, nil)
		}
		var caps map[string]ref.V
		if v.capsKnown {
			caps = map[string]ref.V{}
			for c := v.caps; c != nil; c = c.next {
				if _, ok := caps[c.name]; !ok {
					caps[c.name] = g.refValue(c.v, true)
				}
			}
		}
		return ref.V{Kind: ref.KNode, Shape: ref.UnknownShape, S: text, Start: start, End: end, Caps: caps}
	}
	// An element that did not match, a value-less expression, whose value the generator does not
	// model, and the value of an action.
	return ref.Unknown
}

// units returns the byte offset off of the input known so far in code points.
func (g *gen) units(off int) int {
	if off <= len(g.out) {
		return utf8.RuneCount(g.out[:off])
	}
	return utf8.RuneCount(g.out) + utf8.RuneCountInString(g.forced[:min(off-len(g.out), len(g.forced))])
}

// predicate evaluates the predicate e. It returns false only when the parser's predicate would
// certainly fail; when the generator cannot tell, it returns true and leaves the decision to the
// parser. A variable definition is returned as def.
func (g *gen) predicate(e *grammar.Predicate) (ok bool, def *variable) {
	if a, isBind := e.Term.(*grammar.Bind); isBind {
		v, failed, known := evaluate(g.env(), a.Value)
		if known && failed {
			return false, nil
		}
		return true, &variable{name: a.Name, v: v}
	}
	holds, known := decide(g.env(), e.Term)
	return holds || !known, nil
}
