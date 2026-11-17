package ref

import (
	"testing"

	"zeroparse.dev/zp/grammar"
)

func cast(x grammar.Term, t grammar.TypeExpr) grammar.Term { return &grammar.Cast{X: x, Type: t} }
func tref(n string) grammar.TypeExpr                       { return &grammar.TypeRef{Name: n} }
func nodeOf(t grammar.TypeExpr) grammar.TypeExpr {
	return &grammar.TypeApp{Name: "node", Args: []grammar.TypeExpr{t}}
}

// partialTerms read the node p, whose value, type and captures the tests leave unknown (UnknownShape,
// nil Caps), and the node q, whose captures are known.
var partialTerms = map[string]grammar.Term{
	"p.Start":                mem(capt("p"), "Start"),
	"p.End":                  mem(capt("p"), "End"),
	"text(p)":                fn("text", capt("p")),
	"len(text(p))":           fn("len", fn("text", capt("p"))),
	"len(p)":                 fn("len", capt("p")),
	"p.Value":                mem(capt("p"), "Value"),
	"p.$c":                   mem(capt("p"), "$c"),
	"q.$c":                   mem(capt("q"), "$c"),
	"q.$d":                   mem(capt("q"), "$d"),
	"len(q)":                 fn("len", capt("q")),
	"p == nil":               bin("==", capt("p"), &grammar.NilLit{}),
	"p.(node[string])":       cast(capt("p"), nodeOf(tref("string"))),
	"p.(N)":                  cast(capt("p"), tref("N")),
	"p.(A)":                  cast(capt("p"), tref("A")),
	"p.(_)":                  cast(capt("p"), tref("_")),
	"p.(int)":                cast(capt("p"), tref("int")),
	"foldl over p":           fn("foldl", ilit(0), capt("p"), lam(param("a"), "a", "s")),
	"map over p":             fn("map", capt("p"), lam(param("s"), "s")),
	"N{p}.(N)":               cast(&grammar.New{Type: "N", Args: []grammar.Term{capt("p")}}, tref("N")),
	"A{s}.(N)":               cast(&grammar.New{Type: "A", Args: []grammar.Term{capt("s")}}, tref("N")),
	"A{s}.(A)":               cast(&grammar.New{Type: "A", Args: []grammar.Term{capt("s")}}, tref("A")),
	"s.(B)":                  cast(capt("s"), tref("B")),
	"s.(A)":                  cast(capt("s"), tref("A")),
	"s.(node[string])":       cast(capt("s"), nodeOf(tref("string"))),
	"p.Start == 1 && len(p)": bin("&&", bin("==", mem(capt("p"), "Start"), ilit(1)), bin(">", fn("len", capt("p")), ilit(0))),
	"p.Start == 0 && len(p)": bin("&&", bin("==", mem(capt("p"), "Start"), ilit(0)), bin(">", fn("len", capt("p")), ilit(0))),
}

// partial is the node p of the tests: what the evaluation knows of every candidate below.
var partial = V{Kind: KNode, Shape: UnknownShape, S: "ab", Start: 1, End: 3}

// partialCandidates are nodes that p may turn out to be: each has the range and the text of partial.
var partialCandidates = []V{
	Node(StringNode, "ab", 1, 3, nil, nil),
	Node(StringNode, "ab", 1, 3, nil, map[string]V{"c": Str("q")}),
	Node(TupleNode, "ab", 1, 3, []V{Int(1), Str("b")}, map[string]V{"c": Int(1)}),
	Node(TupleNode, "ab", 1, 3, nil, nil),
	Node(ListNode, "ab", 1, 3, []V{Str("a"), Str("b"), Str("c")}, nil),
	{Kind: KNode, Shape: StringNode, S: "ab", Start: 1, End: 3, Type: "N"},
	{Kind: KNode, Shape: ListNode, S: "ab", Start: 1, End: 3, Type: "N"},
}

func partialEnv(p V) *TermEnv {
	q := p
	q.Caps = map[string]V{"c": Int(7)}
	return &TermEnv{
		Captures: map[string]V{"p": p, "q": q, "s": Node(StringNode, "x", 0, 1, nil, nil)},
		Aliases:  map[string]grammar.TypeExpr{"A": &grammar.TypeGroup{Type: tref("N")}, "B": nodeOf(tref("string"))},
		End:      3,
	}
}

// TestPartialNodeNeverLeaks holds Eval and Decide to soundness for a node of UnknownShape: a value, a
// failure or a verdict that they report as known is the one that every candidate gives. The text of an
// error is not compared, since it names the type of the node.
func TestPartialNodeNeverLeaks(t *testing.T) {
	for name, term := range partialTerms {
		u := partialEnv(partial)
		v, msg := u.Eval(term)
		holds, known := u.Decide(term)
		for _, c := range partialCandidates {
			ce := partialEnv(c)
			cv, cmsg := ce.Eval(term)
			if !hasUnknown(v) && (v.String() != cv.String() || (msg == "") != (cmsg == "")) {
				t.Errorf("%s: with p partial it is %v, %q; with p = %v it is %v, %q", name, v, msg, c, cv, cmsg)
			}
			if known && holds != Holds(cv, cmsg) {
				t.Errorf("%s: with p partial it holds: %v; with p = %v: %v", name, holds, c, Holds(cv, cmsg))
			}
		}
	}
}

// TestPartialNodeResults pins what the evaluation knows of a node of UnknownShape and that aliases
// stand for the types they name.
func TestPartialNodeResults(t *testing.T) {
	u := partialEnv(partial)
	for _, c := range []struct {
		name, value string // "unknown", or the String of the value
		failed      bool
		holds       string
	}{
		{"p.Start", "1", false, "true"},
		{"p.End", "3", false, "true"},
		{"text(p)", `"ab"`, false, "true"},
		{"len(text(p))", "2", false, "true"},
		{"len(p)", "unknown", false, "unknown"},
		{"p.Value", "unknown", false, "unknown"},
		{"p.$c", "unknown", false, "unknown"},
		{"q.$c", "7", false, "true"},
		{"q.$d", "nil", false, "true"},
		{"len(q)", "unknown", false, "unknown"},
		{"p == nil", "false", false, "false"},
		{"p.(node[string])", "unknown", false, "unknown"},
		{"p.(N)", "unknown", false, "unknown"},
		{"p.(A)", "unknown", false, "unknown"},
		{"p.(_)", "unknown", false, "true"},
		{"p.(int)", "nil", true, "false"},
		{"foldl over p", "unknown", false, "unknown"},
		{"map over p", "unknown", false, "unknown"},
		{"N{p}.(N)", "unknown", false, "unknown"},
		{"A{s}.(N)", "N", false, "true"},
		{"A{s}.(A)", "N", false, "true"},
		{"s.(B)", "node[string]", false, "true"},
		{"s.(A)", "nil", true, "false"},
		{"s.(node[string])", "node[string]", false, "true"},
		{"p.Start == 1 && len(p)", "unknown", false, "unknown"},
		{"p.Start == 0 && len(p)", "false", false, "false"},
	} {
		term, ok := partialTerms[c.name]
		if !ok {
			t.Fatalf("no term %s", c.name)
		}
		v, msg := u.Eval(term)
		got := v.String()
		if hasUnknown(v) {
			got = "unknown"
		}
		if got != c.value || (msg != "") != c.failed {
			t.Errorf("%s = %s, %q; want %s, failed %v", c.name, got, msg, c.value, c.failed)
		}
		h, known := u.Decide(term)
		gotHolds := "unknown"
		if known {
			gotHolds = map[bool]string{true: "true", false: "false"}[h]
		}
		if gotHolds != c.holds {
			t.Errorf("%s holds: %s, want %s", c.name, gotHolds, c.holds)
		}
	}
}
