package gentest

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// TermKind says how a generated term is evaluated.
type TermKind int

const (
	// TermAction puts the term in the value action of the rule main.
	TermAction TermKind = iota
	// TermPredicate puts a bool term in a predicate that the rule main tries before an optional "b".
	TermPredicate
)

// TermCase is a generated grammar that evaluates one term.
type TermCase struct {
	Case
	Term string
	// Type is the declared Type of the rule main of a TermAction case, "" for a TermPredicate case.
	Type string
	Kind TermKind
}

// termTypes are the Types of the values a generated term may have.
var termTypes = []string{"int", "string", "bool", "[]int", "[]string", "R", "(int, string)"}

// termPrelude declares the struct type of the generated grammars.
const termPrelude = "type R = { A int; B string; C bool }\n@start\n"

// TermInputs are the inputs of a generated case: "a" is what the rule main matches; "ab" shows
// whether a predicate holds.
var TermInputs = []string{"a", "ab"}

// TermCases returns n well-typed terms generated from seed, as grammars. The terms read the capture
// $x, a string node of the input "a", and use integer and string literals small enough that
// division by zero, an index out of range, a cast that fails and a nil field often occur. The grammars
// are well typed by construction; a caller drops any that the compiler rejects and counts them.
//
// With unset, a struct constructor may leave fields out. A field that is left out reads as nil in the
// engine, in generated node parsers and in TypeScript, and as the zero value of its Type in generated
// typed Go parsers.
func TermCases(seed uint64, n int, unset bool) []TermCase {
	g := &termGen{rng: rand.New(rand.NewPCG(seed, 0x7e12a)), unset: unset}
	out := make([]TermCase, 0, n)
	for i := 0; i < n; i++ {
		if i%4 == 3 {
			out = append(out, g.predicateCase(i))
		} else {
			out = append(out, g.actionCase(i))
		}
	}
	return out
}

func (g *termGen) actionCase(i int) TermCase {
	typ := termTypes[g.rng.IntN(len(termTypes))]
	term := g.term(typ, 3, nil)
	return TermCase{
		Case: Case{Name: fmt.Sprintf("term %d: %s", i, term),
			Src: fmt.Sprintf("%srule main %s = x:\"a\" \"b\"? $$ => %s\n", termPrelude, typ, term), Inputs: TermInputs[:1]},
		Term: term, Type: typ, Kind: TermAction,
	}
}

func (g *termGen) predicateCase(i int) TermCase {
	term := g.term("bool", 3, nil)
	return TermCase{
		Case: Case{Name: fmt.Sprintf("predicate %d: %s", i, term),
			Src: fmt.Sprintf("%srule main int = x:\"a\" ({%s} \"b\" / \"\") $$ => 1\n", termPrelude, term), Inputs: TermInputs},
		Term: term, Kind: TermPredicate,
	}
}

type termGen struct {
	rng   *rand.Rand
	next  int
	unset bool
}

// scope maps a lambda parameter to its Type.
type scope []scoped

type scoped struct{ name, typ string }

func (g *termGen) pick(xs ...string) string { return xs[g.rng.IntN(len(xs))] }

func (g *termGen) fresh(prefix string) string {
	g.next++
	return fmt.Sprintf("%s%d", prefix, g.next)
}

// vars returns the parameters in scope of the Type.
func (s scope) vars(typ string) []string {
	var out []string
	for _, v := range s {
		if v.typ == typ {
			out = append(out, v.name)
		}
	}
	return out
}

// term returns a term of Type typ with at most depth levels of nesting.
func (g *termGen) term(typ string, depth int, s scope) string {
	if vs := s.vars(typ); len(vs) > 0 && g.rng.IntN(3) == 0 {
		return vs[g.rng.IntN(len(vs))]
	}
	switch typ {
	case "int":
		return g.intTerm(depth, s)
	case "string":
		return g.stringTerm(depth, s)
	case "bool":
		return g.boolTerm(depth, s)
	case "[]int", "[]string":
		return g.listTerm(typ, depth, s)
	case "R":
		return g.structTerm(depth, s)
	case "(int, string)":
		return "(" + g.term("int", depth-1, s) + ", " + g.term("string", depth-1, s) + ")"
	}
	panic("termgen: " + typ)
}

var termStrings = []string{`""`, `"a"`, `"bc"`, `"é"`, `"😀x"`}

func (g *termGen) intTerm(depth int, s scope) string {
	if depth <= 0 {
		switch g.rng.IntN(6) {
		case 0:
			return "len(text($x))"
		case 1:
			return "$x.Start"
		case 2:
			return "$x.End"
		case 3:
			// Beyond 2^53, where TypeScript numbers lose integers, and at the 64-bit limits.
			return g.pick("9223372036854775807", "4611686018427387904", "9007199254740993", "3037000500")
		}
		return fmt.Sprint(g.rng.IntN(7) - 2)
	}
	d := depth - 1
	switch g.rng.IntN(16) {
	case 0, 1, 2:
		return "(" + g.term("int", d, s) + " " + g.pick("+", "-", "*") + " " + g.term("int", d, s) + ")"
	case 3, 4:
		return "(" + g.term("int", d, s) + " " + g.pick("/", "%") + " " + g.term("int", d, s) + ")"
	case 5:
		return "-(" + g.term("int", d, s) + ")"
	case 6:
		return "len(" + g.term("string", d, s) + ")"
	case 7:
		return "len(" + g.term(g.pick("[]int", "[]string"), d, s) + ")"
	case 8:
		return g.index("int", d, s)
	case 9, 10:
		return g.fold("int", "int", d, s)
	case 11:
		return g.fold("int", "string", d, s)
	case 12:
		return "(" + g.term("R", d, s) + ").A"
	case 13:
		return "len(" + g.pick("$x", "text($x)", "list()", "concat()") + ")"
	case 14:
		return g.cast("int", d, s)
	}
	return g.intTerm(0, s)
}

// index returns an element of a list of Type typ: list[i].
func (g *termGen) index(typ string, d int, s scope) string {
	return "(" + g.term("[]"+typ, d, s) + ")[" + g.term("int", d, s) + "]"
}

// fold returns foldl/foldr of the element Type elem into an accumulator of Type acc.
func (g *termGen) fold(acc, elem string, d int, s scope) string {
	a, e := g.fresh("a"), g.fresh("e")
	if g.rng.IntN(6) == 0 {
		// An empty list has no element Type, so the body may not use the element.
		body := g.term(acc, d, append(s[:len(s):len(s)], scoped{a, acc}))
		return fmt.Sprintf("%s(%s, list(), (%s, %s) => %s)", g.pick("foldl", "foldr"), g.term(acc, d, s), a, e, body)
	}
	body := g.term(acc, d, append(s[:len(s):len(s)], scoped{a, acc}, scoped{e, elem}))
	return fmt.Sprintf("%s(%s, %s, (%s, %s) => %s)", g.pick("foldl", "foldr"), g.term(acc, d, s), g.term("[]"+elem, d, s), a, e, body)
}

// cast returns an element of a list that mixes Types, cast to typ: the list holds a term of typ and a
// value of the other Type, so that the cast is well typed and may fail.
func (g *termGen) cast(typ string, d int, s scope) string {
	other := `"a"`
	if typ == "string" {
		other = "1"
	}
	items := []string{g.term(typ, d, s), other}
	for range g.rng.IntN(2) {
		if g.rng.IntN(3) == 0 {
			items = append(items, other)
		} else {
			items = append(items, g.term(typ, d, s))
		}
	}
	g.rng.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	return fmt.Sprintf("list(%s)[%d].(%s)", strings.Join(items, ", "), g.rng.IntN(len(items)+1), typ)
}

func (g *termGen) stringTerm(depth int, s scope) string {
	if depth <= 0 {
		if g.rng.IntN(4) == 0 {
			return "text($x)"
		}
		return termStrings[g.rng.IntN(len(termStrings))]
	}
	d := depth - 1
	switch g.rng.IntN(9) {
	case 0, 1, 2:
		return "(" + g.term("string", d, s) + " + " + g.term("string", d, s) + ")"
	case 3:
		return g.index("string", d, s)
	case 4, 5:
		return g.fold("string", "string", d, s)
	case 6:
		return "(" + g.term("R", d, s) + ").B"
	case 7:
		return g.cast("string", d, s)
	}
	return g.stringTerm(0, s)
}

func (g *termGen) boolTerm(depth int, s scope) string {
	if depth <= 0 {
		return g.pick("true", "false", "($x != nil)", "($x == nil)")
	}
	d := depth - 1
	switch g.rng.IntN(12) {
	case 0, 1:
		return "(" + g.term("int", d, s) + " " + g.pick("==", "!=", "<", "<=", ">", ">=") + " " + g.term("int", d, s) + ")"
	case 2, 3:
		return "(" + g.term("string", d, s) + " " + g.pick("==", "!=", "<", "<=", ">", ">=") + " " + g.term("string", d, s) + ")"
	case 4:
		return "(" + g.term("bool", d, s) + " " + g.pick("==", "!=") + " " + g.term("bool", d, s) + ")"
	case 5:
		return "!" + g.term("bool", d, s)
	case 6, 7:
		return "(" + g.term("bool", d, s) + " " + g.pick("&&", "||") + " " + g.term("bool", d, s) + ")"
	case 8:
		return g.fold("bool", "int", d, s)
	case 9:
		return "(" + g.term("R", d, s) + ").C"
	case 10:
		return "(" + g.term(g.pick("int", "string", "[]int"), d, s) + " " + g.pick("==", "!=") + " nil)"
	}
	return g.boolTerm(0, s)
}

func (g *termGen) listTerm(typ string, depth int, s scope) string {
	elem := strings.TrimPrefix(typ, "[]")
	other := map[string]string{"int": "string", "string": "int"}[elem]
	if depth <= 0 {
		var items []string
		for range 1 + g.rng.IntN(3) {
			items = append(items, g.term(elem, 0, s))
		}
		return "list(" + strings.Join(items, ", ") + ")"
	}
	d := depth - 1
	switch g.rng.IntN(8) {
	case 0, 1:
		var items []string
		for range 1 + g.rng.IntN(3) {
			items = append(items, g.term(elem, d, s))
		}
		return "list(" + strings.Join(items, ", ") + ")"
	case 2, 3:
		e := g.fresh("e")
		src := g.pick(elem, other)
		return fmt.Sprintf("map(%s, (%s) => %s)", g.term("[]"+src, d, s), e, g.term(elem, d, append(s[:len(s):len(s)], scoped{e, src})))
	case 4:
		e := g.fresh("e")
		return fmt.Sprintf("filter(%s, (%s) => %s)", g.term(typ, d, s), e, g.term("bool", d, append(s[:len(s):len(s)], scoped{e, elem})))
	case 5:
		var args []string
		for range 1 + g.rng.IntN(3) {
			args = append(args, g.term(typ, d, s))
		}
		return "concat(" + strings.Join(args, ", ") + ")"
	case 6:
		return "concat(" + g.term(typ, d, s) + ", map(" + g.term("[]"+other, d, s) + ", (" + g.fresh("e") + ") => " + g.term(elem, 0, s) + "))"
	}
	return g.listTerm(typ, 0, s)
}

func (g *termGen) structTerm(depth int, s scope) string {
	d := depth - 1
	if depth <= 0 {
		d = 0
	}
	var fields []string
	if !g.unset || g.rng.IntN(5) != 0 { // a field left unset reads as nil
		fields = append(fields, "A: "+g.term("int", d, s))
	}
	if !g.unset || g.rng.IntN(5) != 0 {
		fields = append(fields, "B: "+g.term("string", d, s))
	}
	if !g.unset || g.rng.IntN(5) != 0 {
		fields = append(fields, "C: "+g.term("bool", d, s))
	}
	return "R{" + strings.Join(fields, ", ") + "}"
}
