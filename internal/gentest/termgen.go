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
	// Lit is the text of the literal that the rule main matches, and $x captures: "a" in the terms of
	// TermCases, and "a", "é" or "😀" in those of TermCasesWide, whose positions differ in the two units.
	Lit string
}

// termTypes are the Types of the values a generated term may have.
var termTypes = []string{"int", "string", "bool", "[]int", "[]string", "R", "(int, string)"}

// termPrelude declares the struct type of the generated grammars.
const termPrelude = "type R = { A int; B string; C bool }\n@start\n"

// widePrelude declares the struct types of the grammars of TermCasesWide: R has a field of each kind
// whose zero value is not a scalar, which a constructor may leave unset: a list, a tuple, an optional
// string and a struct, which holds a struct in turn, so that a term reads fields of structs nested
// more than one level deep.
const widePrelude = "type Leaf = { V int; S string }\n" +
	"type Inner = { N int; Q Leaf }\n" +
	"type R = { A int; B string; C bool; L []int; T (int, string); O *string; In Inner }\n@start\n"

// TermInputs are the inputs of a generated case: "a" is what the rule main matches; "ab" shows
// whether a predicate holds.
var TermInputs = []string{"a", "ab"}

// TermCases returns n well-typed terms generated from seed, as grammars. The terms read the capture
// $x, a string node of the input "a", and use integer and string literals small enough that
// division by zero, an index out of range, a cast that fails and a nil field often occur. The grammars
// are well typed by construction; a caller drops any that the compiler rejects and counts them.
//
// With unset, a struct constructor may leave fields out. A field that is left out reads as the zero
// value of its Type on every target.
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

// TermCasesWide returns n well-typed terms generated from seed like TermCases, with more forms: strings
// with characters from U+E000 to U+FFFF and outside the Basic Multilingual Plane, a captured literal
// that is not ASCII, tuple indexing, nil lists and nil elements, lists of bools, of structs, of lists
// and of tuples, casts to bool and to a struct, and the errors that only run time can find (the
// comparison of structured values, the operand of && that is not a bool, the filter that gives no
// bool), and a struct R with a list, a tuple, an optional string and a struct field, which a term
// reads whether or not the constructor set them. TermCases is not changed by the forms, so that what the goldens pin stays the same.
func TermCasesWide(seed uint64, n int, unset bool) []TermCase {
	g := &termGen{rng: rand.New(rand.NewPCG(seed, 0x91de5)), unset: unset, wide: true}
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

// wideTypes are the Types of the values a term of TermCasesWide may have, besides termTypes.
var wideTypes = []string{"[]bool", "[]R", "[][]int", "[](int, string)", "Inner", "Leaf", "*string"}

// wideLits are the literals the rule main may match in a term of TermCasesWide.
var wideLits = []string{"a", "é", "😀"}

func (g *termGen) actionCase(i int) TermCase {
	typ, lit := "", "a"
	if g.wide {
		lit = wideLits[g.rng.IntN(len(wideLits))]
		all := append(append([]string{}, termTypes...), wideTypes...)
		typ = all[g.rng.IntN(len(all))]
	} else {
		typ = termTypes[g.rng.IntN(len(termTypes))]
	}
	term := g.term(typ, 3, nil)
	return TermCase{
		Case: Case{Name: fmt.Sprintf("term %d: %s", i, term),
			Src: fmt.Sprintf("%srule main %s = x:%q \"b\"? $$ => %s\n", g.prelude(), typ, lit, term), Inputs: []string{lit}},
		Term: term, Type: typ, Kind: TermAction, Lit: lit,
	}
}

func (g *termGen) predicateCase(i int) TermCase {
	lit := "a"
	if g.wide {
		lit = wideLits[g.rng.IntN(len(wideLits))]
	}
	term := g.term("bool", 3, nil)
	return TermCase{
		Case: Case{Name: fmt.Sprintf("predicate %d: %s", i, term),
			Src: fmt.Sprintf("%srule main int = x:%q ({%s} \"b\" / \"\") $$ => 1\n", g.prelude(), lit, term), Inputs: []string{lit, lit + "b"}},
		Term: term, Kind: TermPredicate, Lit: lit,
	}
}

func (g *termGen) prelude() string {
	if g.wide {
		return widePrelude
	}
	return termPrelude
}

type termGen struct {
	rng   *rand.Rand
	next  int
	unset bool
	wide  bool // the forms of TermCasesWide
	// noNil is positive while a list that is indexed is generated.
	noNil int
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
	case "[]int", "[]string", "[]bool", "[]R", "[][]int", "[](int, string)":
		return g.listTerm(typ, depth, s)
	case "R":
		return g.structTerm(depth, s)
	case "Inner":
		return g.innerTerm(depth, s)
	case "Leaf":
		return g.leafTerm(depth, s)
	case "*string":
		return g.optionalTerm(depth, s)
	case "(int, string)":
		return "(" + g.term("int", depth-1, s) + ", " + g.term("string", depth-1, s) + ")"
	}
	panic("termgen: " + typ)
}

var termStrings = []string{`""`, `"a"`, `"bc"`, `"é"`, `"😀x"`}

// wideStrings are the strings besides termStrings in TermCasesWide: characters from U+E000 to U+FFFF,
// which sort before the characters outside the Basic Multilingual Plane by code point but after them by
// UTF-16 code unit, and such a character.
var wideStrings = []string{"\"\ue000\"", "\"\uffff\"", "\"\U0001F600\"", "\"a\uffff\"", "\"\U00010000\""}

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
	if g.wide {
		switch g.rng.IntN(20) {
		case 6:
			return "len((" + g.term("R", d, s) + ").L)"
		case 7:
			return "(" + g.term("R", d, s) + ").T[0]"
		case 8:
			return "(" + g.term("R", d, s) + g.pick(").In.N", ").In.Q.V")
		case 9:
			return "len((" + g.term("R", d, s) + ").O)"
		case 0:
			return "(" + g.term("(int, string)", d, s) + ")[0]"
		case 1:
			return "len(" + g.source(g.pick("[]bool", "[]R", "[][]int", "[](int, string)"), d, s) + ")"
		case 5:
			// A nil element makes the Type of the list optional, which only len and the rest of the
			// term that does not read an element accept.
			return "len(list(" + g.pick("nil", "nil, 1", "1, nil, nil", "nil, \"a\"", "list(nil), nil", "nil, R{A: 1}") + "))"
		case 2:
			return "len(nil)"
		case 3:
			return "(" + g.indexed("[][]int", d, s) + ")[" + g.term("int", d, s) + "][" + g.term("int", d, s) + "]"
		case 4:
			return "(" + g.indexed("[]R", d, s) + ")[" + g.term("int", d, s) + "].A"
		}
	}
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
		return "len(" + g.source(g.pick("[]int", "[]string"), d, s) + ")"
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
	return "(" + g.indexed("[]"+typ, d, s) + ")[" + g.term("int", d, s) + "]"
}

// emptyList returns an empty list that a function reads: list(), or in TermCasesWide nil too, which
// is the empty list as well. Where a list is the value of the term, nil has no Type that fits.
func (g *termGen) emptyList() string {
	if g.wide && g.rng.IntN(2) == 0 {
		return "nil"
	}
	return "list()"
}

// source returns a list term of Type typ for len, which is nil sometimes.
func (g *termGen) source(typ string, d int, s scope) string {
	if g.wide && g.rng.IntN(8) == 0 {
		return "nil"
	}
	return g.term(typ, d, s)
}

// indexed returns a list term of Type typ that is not nil.
func (g *termGen) indexed(typ string, d int, s scope) string {
	g.noNil++
	defer func() { g.noNil-- }()
	return g.term(typ, d, s)
}

// fold returns foldl/foldr of the element Type elem into an accumulator of Type acc.
func (g *termGen) fold(acc, elem string, d int, s scope) string {
	a, e := g.fresh("a"), g.fresh("e")
	if g.rng.IntN(6) == 0 {
		// An empty list has no element Type, so the body may not use the element.
		body := g.term(acc, d, append(s[:len(s):len(s)], scoped{a, acc}))
		return fmt.Sprintf("%s(%s, %s, (%s, %s) => %s)", g.pick("foldl", "foldr"), g.term(acc, d, s), g.emptyList(), a, e, body)
	}
	body := g.term(acc, d, append(s[:len(s):len(s)], scoped{a, acc}, scoped{e, elem}))
	return fmt.Sprintf("%s(%s, %s, (%s, %s) => %s)", g.pick("foldl", "foldr"), g.term(acc, d, s), g.term("[]"+elem, d, s), a, e, body)
}

// cast returns an element of a list that mixes Types, cast to typ: the list holds a term of typ and a
// value of the other Type, so that the cast is well typed and may fail.
func (g *termGen) cast(typ string, d int, s scope) string {
	other := `"a"`
	if typ == "string" || typ == "bool" {
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
		if g.wide && g.rng.IntN(3) == 0 {
			return wideStrings[g.rng.IntN(len(wideStrings))]
		}
		return termStrings[g.rng.IntN(len(termStrings))]
	}
	d := depth - 1
	if g.wide && g.rng.IntN(10) == 0 {
		return "(" + g.term("(int, string)", d, s) + ")[1]"
	}
	if g.wide && g.rng.IntN(10) == 0 {
		return "(" + g.term("R", d, s) + ").T[1]"
	}
	if g.wide && g.rng.IntN(10) == 0 {
		return "(" + g.term("R", d, s) + ").In.Q.S"
	}
	if g.wide && g.rng.IntN(12) == 0 {
		return "(" + g.term("Leaf", d, s) + ").S"
	}
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
	if g.wide {
		switch g.rng.IntN(12) {
		case 0:
			return g.cast("bool", d, s)
		case 1:
			// Structured values and values of different kinds have no equality: the error says so, and
			// how to compare them. The checker accepts the operands only if their Types are not known,
			// like an element of a list that mixes Types.
			return g.pick("(list(R{A: 1}, 1)[0] == list(R{A: 1}, 1)[0])", "(list(list(1), 1)[0] != list(list(1), 1)[0])",
				"(list($x, 1)[0] == list($x, 1)[0])", "(list(($x, 1), 1)[0] == list(($x, 1), 1)[0])",
				"(list(1, \"a\")[0] == list(1, \"a\")[1])", "(list(1, true)[1] != list(1, true)[0])",
				"(list(1, \"a\")[1] == list(1, \"a\")[1])", "(list() != nil)", "(list(nil)[0] == list(nil)[0])")
		case 6:
			return "(" + g.indexed("[]bool", d, s) + ")[" + g.term("int", d, s) + "]"
		case 7:
			return "((" + g.term("R", d, s) + ").O " + g.pick("==", "!=") + " nil)"
		}
	}
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
	other := map[string]string{"int": "string", "string": "int", "bool": "int", "R": "string", "[]int": "int", "(int, string)": "int"}[elem]
	if depth <= 0 {
		var items []string
		for range 1 + g.rng.IntN(3) {
			items = append(items, g.term(elem, 0, s))
		}
		return "list(" + strings.Join(items, ", ") + ")"
	}
	d := depth - 1
	if g.wide && typ == "[]int" && g.rng.IntN(8) == 0 {
		return "(" + g.term("R", d, s) + ").L"
	}
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
		if g.wide && g.rng.IntN(6) == 0 {
			// The elements of an empty list have no Type, so the function does not read one.
			return fmt.Sprintf("map(%s, (%s) => %s)", g.emptyList(), e, g.term(elem, d, s))
		}
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
	if g.wide && depth > 0 && g.rng.IntN(8) == 0 {
		return g.cast("R", d, s)
	}
	var fields []string
	if !g.unset || g.rng.IntN(5) != 0 { // a field left unset reads as its zero value
		fields = append(fields, "A: "+g.term("int", d, s))
	}
	if !g.unset || g.rng.IntN(5) != 0 {
		fields = append(fields, "B: "+g.term("string", d, s))
	}
	if !g.unset || g.rng.IntN(5) != 0 {
		fields = append(fields, "C: "+g.term("bool", d, s))
	}
	if g.wide {
		if !g.unset || g.rng.IntN(5) != 0 {
			fields = append(fields, "L: "+g.term("[]int", d, s))
		}
		if !g.unset || g.rng.IntN(5) != 0 {
			fields = append(fields, "T: "+g.term("(int, string)", d, s))
		}
		if !g.unset || g.rng.IntN(5) != 0 {
			fields = append(fields, "O: "+g.term("string", d, s))
		}
		if !g.unset || g.rng.IntN(5) != 0 {
			fields = append(fields, "In: "+g.term("Inner", d, s))
		}
	}
	return "R{" + strings.Join(fields, ", ") + "}"
}

// innerTerm returns a term of Type Inner: a constructor, which may leave fields unset, or the field In
// of a struct R.
func (g *termGen) innerTerm(depth int, s scope) string {
	d := depth - 1
	if depth <= 0 {
		return "Inner{N: " + g.term("int", 0, s) + ", Q: " + g.term("Leaf", 0, s) + "}"
	}
	if g.rng.IntN(4) == 0 {
		return "(" + g.term("R", d, s) + ").In"
	}
	var fields []string
	if !g.unset || g.rng.IntN(5) != 0 {
		fields = append(fields, "N: "+g.term("int", d, s))
	}
	if !g.unset || g.rng.IntN(5) != 0 {
		fields = append(fields, "Q: "+g.term("Leaf", d, s))
	}
	return "Inner{" + strings.Join(fields, ", ") + "}"
}

// leafTerm returns a term of Type Leaf: a constructor, which may leave fields unset, or the field Q of
// a struct Inner, which is nested two levels below a struct R.
func (g *termGen) leafTerm(depth int, s scope) string {
	d := depth - 1
	if depth <= 0 {
		return "Leaf{V: " + g.term("int", 0, s) + ", S: " + g.term("string", 0, s) + "}"
	}
	switch g.rng.IntN(4) {
	case 0:
		return "(" + g.term("Inner", d, s) + ").Q"
	case 1:
		return "(" + g.term("R", d, s) + ").In.Q"
	}
	var fields []string
	if !g.unset || g.rng.IntN(5) != 0 {
		fields = append(fields, "V: "+g.term("int", d, s))
	}
	if !g.unset || g.rng.IntN(5) != 0 {
		fields = append(fields, "S: "+g.term("string", d, s))
	}
	return "Leaf{" + strings.Join(fields, ", ") + "}"
}

// optionalTerm returns a term of Type *string, an optional string used as a value: the field O of a
// struct R, which a constructor may leave unset (nil), nil, or a string.
func (g *termGen) optionalTerm(depth int, s scope) string {
	if depth <= 0 {
		return g.pick("nil", g.stringTerm(0, s))
	}
	d := depth - 1
	switch g.rng.IntN(4) {
	case 0:
		return "(" + g.term("R", d, s) + ").O"
	case 1:
		return "nil"
	}
	return g.term("string", d, s)
}

// MemberTermCases returns fixed terms that read every kind of member on the struct types of
// TermCasesWide: the range of a node, a capture of a node, and each field of R and Inner, set by the
// constructor or left unset so that it reads as its zero value, alone and as the operand of an
// operator, an index or len. Unlike the generated cases, each read is sure to occur.
func MemberTermCases() []TermCase {
	actions := []struct{ typ, term string }{
		{"int", "$x.Start"},
		{"int", "$x.End"},
		{"int", "$x.End - $x.Start"},
		{"int", "(R{}).A"},
		{"string", "(R{}).B"},
		{"bool", "(R{}).C"},
		{"[]int", "(R{}).L"},
		{"int", "len((R{}).L)"},
		{"(int, string)", "(R{}).T"},
		{"int", "(R{}).T[0]"},
		{"string", "(R{}).T[1]"},
		{"bool", "(R{}).O == nil"},
		{"Inner", "(R{}).In"},
		{"int", "(R{}).In.N"},
		{"int", "(R{}).In.Q.V"},
		{"string", "(R{}).In.Q.S"},
		{"Leaf", "(R{}).In.Q"},
		{"Leaf", "(Inner{}).Q"},
		{"Leaf", "(R{In: Inner{Q: Leaf{V: 3}}}).In.Q"},
		{"int", "(R{In: Inner{Q: Leaf{V: $x.End}}}).In.Q.V"},
		{"bool", "(R{}).L == nil"},
		{"bool", "(R{L: list()}).L != nil"},
		{"*string", "(R{}).O"},
		{"*string", "(R{O: text($x)}).O"},
		{"int", "(R{A: 7}).A"},
		{"string", "(R{B: text($x)}).B"},
		{"[]int", "(R{L: list(1, 2, 3)}).L"},
		{"int", "(R{L: list($x.Start, 2)}).L[1]"},
		{"(int, string)", "(R{T: ($x.End, \"t\")}).T"},
		{"string", "(R{O: \"o\"}).O.(string)"},
		{"int", "(R{In: Inner{N: $x.End}}).In.N"},
		{"int", "(Inner{}).N"},
		{"int", "list(R{}, R{A: 2})[1].A + len(list(R{}, R{A: 2})[0].L)"},
		{"[]int", "map(list(R{}, R{L: list(4)}), (r) => len(r.L))"},
		{"int", "foldl(0, list(R{In: Inner{N: 1}}, R{}), (a, r) => a + r.In.N + r.T[0])"},
	}
	var out []TermCase
	for _, a := range actions {
		for _, lit := range []string{"a", "😀"} {
			out = append(out, TermCase{
				Case: Case{Name: "member: " + a.term,
					Src: fmt.Sprintf("%srule main %s = x:%q \"b\"? $$ => %s\n", widePrelude, a.typ, lit, a.term), Inputs: []string{lit}},
				Term: a.term, Type: a.typ, Kind: TermAction, Lit: lit,
			})
		}
	}
	// A capture of a node, matched and unmatched, read directly and by a predicate.
	capture := widePrelude + "rule main string = x:sub $$ => text($x.$y) + text($x.$z)\nrule sub = y:\"a\" z:\"b\"?\n"
	out = append(out, TermCase{Case: Case{Name: "member: capture", Src: capture, Inputs: []string{"a", "ab"}},
		Term: "text($x.$y) + text($x.$z)", Type: "string", Kind: TermAction, Lit: "a"})
	return out
}

// convertPrelude declares the named node types of ConvertTermCases: two of string nodes, one of the tuple
// node of a sequence of two literals and one of the list node of a repetition of a literal.
const convertPrelude = "type N = node[string]\ntype M = node[string]\ntype P = node[(node[string], node[string])]\n" +
	"type Ls = node[[]node[string]]\n@start\n"

// ConvertTermCases returns fixed terms that convert a value to a named node type: a string node, a node
// of another named node type, a node that already has the type, a tuple node and a list node, alone and
// as the operand of a member read, len or text, and the value of a rule of the named node type. Each
// conversion is sure to occur.
func ConvertTermCases() []TermCase {
	actions := []struct{ typ, body, term string }{
		{"string", "x:%[1]q", "text(N{$x})"},
		{"int", "x:%[1]q", "N{$x}.End - N{$x}.Start"},
		{"string", "x:%[1]q", "N{$x}.Value"},
		{"string", "x:%[1]q", "text(M{N{$x}})"},
		{"int", "x:%[1]q", "len(N{N{$x}})"},
		{"N", "x:%[1]q", "N{$x}"},
		{"int", "x:(%[1]q %[1]q)", "len(P{$x})"},
		{"string", "x:(%[1]q %[1]q)", "text(P{$x}.Value[1])"},
		{"int", "x:%[1]q+", "len(Ls{$x})"},
		{"int", "x:%[1]q+", "len(Ls{Ls{$x}}.Value)"},
	}
	var out []TermCase
	for _, a := range actions {
		for _, lit := range []string{"a", "😀"} {
			input := lit
			if strings.Contains(a.body, "%[1]q %[1]q") || strings.HasSuffix(a.body, "+") {
				input = lit + lit
			}
			out = append(out, TermCase{
				Case: Case{Name: "convert: " + a.term,
					Src: fmt.Sprintf("%srule main %s = %s $$ => %s\n", convertPrelude, a.typ, fmt.Sprintf(a.body, lit), a.term), Inputs: []string{input}},
				Term: a.term, Type: a.typ, Kind: TermAction, Lit: lit,
			})
		}
	}
	return out
}

// structPrelude declares the struct types of StructTermCases: S has two fields that hold nodes, an int, a
// struct and a list.
const structPrelude = "type Inner = { N int }\n" +
	"type S = { A *node[string]; B *node[string]; K int; I Inner; L []node[string] }\n@start\n"

// StructTermCases returns fixed terms that make a struct value whose fields hold nodes: none, one, two,
// a struct, a list and a node that did not match (nil), as the value of a rule and under a read of a field.
// The range of a struct covers the ranges of the field values that are nodes and is the range of the rule if
// there is none, so each of the cases shows the range logic through the units it charges. The rule captures
// two nodes, of one literal each, and a third that does not match, each on a literal of one byte and one
// beyond the Basic Multilingual Plane.
func StructTermCases() []TermCase {
	cases := []struct{ typ, term string }{
		{"S", "S{K: 1}"},
		{"S", "S{A: $y, K: 1}"},
		{"S", "S{A: $x, B: $y}"},
		{"S", "S{A: $y, B: $x}"},
		{"S", "S{A: $z, K: 2}"},
		{"S", "S{A: $z, B: $y}"},
		{"S", "S{I: Inner{N: 1}}"},
		{"S", "S{L: list($y, $x)}"},
		{"S", "S{A: $x, B: $y, K: 3, I: Inner{N: 4}, L: list($x)}"},
		{"int", "S{K: 1}.K"},
		{"int", "S{A: $y, K: 1}.K"},
		{"int", "S{I: Inner{N: 1}}.I.N"},
		{"int", "len(S{L: list($y, $x)}.L)"},
		{"bool", "S{A: $z, K: 2}.A == nil"},
	}
	var out []TermCase
	for _, a := range cases {
		for _, lit := range []string{"a", "😀"} {
			out = append(out, TermCase{
				Case: Case{Name: "struct: " + a.term,
					Src:    fmt.Sprintf("%srule main %s = x:%[3]q y:%[3]q z:\"c\"? $$ => %[4]s\n", structPrelude, a.typ, lit, a.term),
					Inputs: []string{lit + lit}},
				Term: a.term, Type: a.typ, Kind: TermAction, Lit: lit,
			})
		}
	}
	return out
}

// castPrelude declares the types of CastTermCases: the named node type Num of string nodes, the named
// types Pair and Many of tuple and list nodes, and the struct Inner.
const castPrelude = "type Inner = { N int }\ntype Num = node[string]\ntype Pair = node[(node[string], node[string])]\n" +
	"type Many = node[[]node[string]]\n"

// CastTermCases returns fixed terms that cast a value to a type, each with the type it checks against and
// the value it checks. The rule captures a string node $x, a tuple node $y of two string nodes, a list node
// $z of one or more string nodes and an optional node $w that does not match, and each case runs with one and
// two elements of $z. A cast
// charges one step for each element of a list or a tuple it examines, up to the first that fails, so the
// terms cast lists and tuples of the values of the rule, and of literals, that are of the type, of another
// type at the first, the middle and the last element, of another length and of another shape, besides
// every other type of a cast: the scalars, an optional, a join, the top type, the nodes of each shape, the
// named node types and a struct.
//
// Each term puts the value in a list with a field of a struct of the type, which a cast needs to accept a
// value whose static type it does not know to be of the type; the field is never cast.
func CastTermCases() []TermCase {
	casts := []struct{ typ, value string }{
		{"int", "1"}, {"int", `"s"`}, {"int", "true"}, {"string", `"s"`}, {"string", "1"}, {"bool", "true"}, {"bool", "1"},
		{"_", "1"}, {"_", `"s"`},
		{"*int", "1"}, {"*int", `"s"`}, {"*Num", "$w"}, {"Num", "$w"}, {"*Num", "$x"},
		{"int | string", "1"}, {"int | string", `"s"`}, {"int | string", "true"}, {"Num | int", "$x"},
		{"node[string]", "$x"}, {"node[string]", "$y"}, {"node[string]", "1"},
		{"node[_]", "$x"}, {"node[_]", "$y"}, {"node[_]", "$z"}, {"node[_]", "1"}, {"node[_]", "$z.Value"},
		{"error", "$x"},
		{"Num", "$x"}, {"Num", "Num{$x}"}, {"Num", "$y"}, {"Num", "1"},
		{"Pair", "$y"}, {"Pair", "Pair{$y}"}, {"Many", "Many{$z}"}, {"Many", "$z"},
		{"Inner", "Inner{N: 1}"}, {"Inner", "Num{$x}"}, {"Inner", "1"},
		{"[]Inner", "list(Inner{N: 1}, Inner{N: 2})"}, {"[]Inner", "list(Inner{N: 1}, 2)"},
		// Nodes whose value has a type.
		{"node[(node[string], node[string])]", "$y"}, {"node[(node[string], int)]", "$y"}, {"node[(int, node[string])]", "$y"},
		{"node[(node[string], node[string], node[string])]", "$y"}, {"node[(node[string], node[string])]", "$x"},
		{"node[[]node[string]]", "$z"}, {"node[[]int]", "$z"}, {"node[[]_]", "$z"}, {"node[[]node[string]]", "$y"},
		{"node[*string]", "$x"}, {"node[*string]", "$y"}, {"node[string | []node[string]]", "$z"},
		{"node[string | []node[string]]", "$y"}, {"node[int]", "$x"},
		// Lists and tuples of values.
		{"[]node[string]", "$z.Value"}, {"[]int", "$z.Value"}, {"[]_", "$z.Value"}, {"[][]int", "$z.Value"},
		{"[]int", "list(1, 2, 3)"}, {"[]int", `list("s", 2, 3)`}, {"[]int", `list(1, "s", 3)`}, {"[]int", `list(1, 2, "s")`},
		{"[]string", "list(1, 2)"}, {"[]_", "list(1, true)"}, {"[]int", "list(1, true)"},
		{"[](int | string)", `list(1, "s", 2)`}, {"[](int | string)", "list(1, true)"}, {"[]*int", "list(1, 2)"},
		{"[][]int", "list(list(1, 2), list(3))"}, {"[][]int", `list(list(1, 2), list(3, "s"))`}, {"[][]int", "list(list(1, 2), 3)"},
		{"[]int", "list(list(1))"}, {"int", "list(1)"},
		{"(Num, Num)", "$y.Value"}, {"(Num, int)", "$y.Value"}, {"(int, Num)", "$y.Value"}, {"(Num, Num, Num)", "$y.Value"},
		{"(Num)", "$y.Value"}, {"(_, _)", "$y.Value"}, {"(Num, Num)", "$z.Value"},
		{"(int, string)", `(1, "a")`}, {"(int, string)", `(1, 2)`}, {"(int, string)", `("a", "b")`},
		{"(int, string)", `(1, "a", true)`}, {"(int, string)", `(1,)`},
		{"(int, []int)", "(1, list(2, 3))"}, {"(int, []int)", `(1, list(2, "s"))`}, {"([]int, string)", `(list(1, 2), "a")`},
		{"(int, (int, string))", `(1, (2, "a"))`}, {"(int, (int, string))", `(1, (2, 3))`},
		{"[](int, string)", `list((1, "a"), (2, "b"))`}, {"[](int, string)", `list((1, "a"), (2, 3))`},
		{"(int, string)", "list(1, 2)"}, {"[]int", "(1, 2)"},
	}
	// The struct F has a field of each type that a case casts to, or an optional one where the type has no
	// zero value.
	var fieldOf = map[string]int{}
	var fields []string
	for _, a := range casts {
		if _, ok := fieldOf[a.typ]; !ok {
			typ := a.typ
			if strings.ContainsAny(typ, "|") || strings.Contains(typ, "node") || strings.Contains(typ, "Num") ||
				strings.Contains(typ, "Pair") || strings.Contains(typ, "Many") || typ == "error" {
				if !strings.HasPrefix(typ, "*") {
					typ = "*(" + typ + ")"
				}
			}
			fieldOf[a.typ] = len(fields)
			fields = append(fields, fmt.Sprintf("F%d %s", len(fields), typ))
		}
	}
	prelude := castPrelude + "type F = { " + strings.Join(fields, "; ") + " }\n@start\n"
	var out []TermCase
	for _, a := range casts {
		for _, lit := range []string{"a"} {
			term := fmt.Sprintf("list(%s, (F{}).F%d)[0].(%s)", a.value, fieldOf[a.typ], a.typ)
			out = append(out, TermCase{
				Case: Case{Name: "cast: " + term,
					Src:    fmt.Sprintf("%srule main %s = x:%[3]q y:(%[3]q %[3]q) z:%[3]q+ w:\"c\"? $$ => %[4]s\n", prelude, a.typ, lit, term),
					Inputs: []string{strings.Repeat(lit, 4), strings.Repeat(lit, 5)}},
				Term: term, Type: a.typ, Kind: TermAction, Lit: lit,
			})
		}
	}
	return out
}

// hullPrelude declares the struct type of HullTermCases: S has a list and a tuple field.
const hullPrelude = "type S = { L []node[string]; T (int, string) }\n@start\n"

// HullTermCases returns fixed terms that make a list value or a tuple value: of no element, of nodes in
// either order, of nodes with one that did not match (nil), of ints, strings and bools (which the node
// runtimes box in nodes), and of lists and tuples of them, alone, as a field of a struct and under len,
// an index, map and concat. The range of such a value covers the elements that are nodes, which the value
// model does not show, so the cases pin what making the value charges for each element, whatever its kind,
// each on a literal of one byte and one beyond the Basic Multilingual Plane. The rule captures two nodes,
// of one literal each, and a third that does not match.
func HullTermCases() []TermCase {
	cases := []struct{ typ, term string }{
		{"int", "len(list())"},
		{"int", "len(list($x))"},
		{"int", "len(list($y, $x))"},
		{"int", "len(list($z))"},
		{"int", "len(list($z, $y))"},
		{"int", "len(list($y, $z, $x))"},
		{"int", "len(list(1))"},
		{"int", "len(list($y, 1))"},
		{"int", `len(list("s", $x))`},
		{"int", "len(list(true, $y, 1))"},
		{"int", "len(list(1, 2, 3, 4))"},
		{"int", "len(list(list($y), $x))"},
		{"int", "len(list(($y, 1), $x))"},
		{"int", "len(list(list(), list(), $z))"},
		{"int", "len(list($x, 1, $y))"},
		{"int", "len(($x, $y, 2))"},
		{"int", "len(($x,))"},
		{"int", "list($x, $y)[1].End"},
		{"int", "($y, $x)[0].Start"},
		{"int", "($z, $y)[1].End"},
		{"int", "(1, $y)[1].End"},
		{"int", `("s", 2, true)[1]`},
		{"int", "(list($y), ($x, 1))[1][0].Start"},
		{"int", "len(S{L: list($y)}.L)"},
		{"int", "len(S{L: list($y, $x), T: (1, \"s\")}.L)"},
		{"int", "S{L: list($y, $x), T: (1, \"s\")}.T[0]"},
		{"int", "len(S{}.L)"},
		{"int", "len(map(list($x, $y), (n) => n.End))"},
		{"int", "len(concat(list($x), list($y, 1)))"},
		{"int", "len(concat(list($x), list($y, $z)))"},
	}
	var out []TermCase
	for _, a := range cases {
		for _, lit := range []string{"a", "😀"} {
			out = append(out, TermCase{
				Case: Case{Name: "hull: " + a.term,
					Src:    fmt.Sprintf("%srule main %s = x:%[3]q y:%[3]q z:\"c\"? $$ => %[4]s\n", hullPrelude, a.typ, lit, a.term),
					Inputs: []string{lit + lit}},
				Term: a.term, Type: a.typ, Kind: TermAction, Lit: lit,
			})
		}
	}
	return out
}
