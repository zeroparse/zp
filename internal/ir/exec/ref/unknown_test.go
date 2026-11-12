package ref

import (
	"testing"

	"zeroparse.dev/zp/grammar"
)

// Terms built by hand: the package cannot depend on a grammar parser.
func capt(n string) grammar.Term  { return &grammar.CaptureRef{Name: n} }
func param(n string) grammar.Term { return &grammar.CaptureRef{Name: n, Param: true} }
func vr(n string) grammar.Term    { return &grammar.VarRef{Name: n} }
func ilit(i int) grammar.Term     { return &grammar.IntLit{Value: i} }
func blit(b bool) grammar.Term    { return &grammar.BoolLit{Value: b} }
func bin(op string, l, r grammar.Term) grammar.Term {
	return &grammar.Binary{Op: op, L: l, R: r}
}
func un(op string, x grammar.Term) grammar.Term { return &grammar.Unary{Op: op, X: x} }
func fn(name string, args ...grammar.Term) grammar.Term {
	return &grammar.Call{Func: name, Args: args}
}
func lam(body grammar.Term, params ...string) grammar.Term {
	return &grammar.Lambda{Params: params, Body: body}
}
func mem(x grammar.Term, name string) grammar.Term { return &grammar.Member{X: x, Name: name} }

// unknownTerms read the capture x, which the tests make unknown, the known capture xs (a list node
// holding x and an int) and the variables v (unknown) and n (known).
var unknownTerms = map[string]grammar.Term{
	"x.Start":           mem(capt("x"), "Start"),
	"x.$c":              mem(capt("x"), "$c"),
	"len(x)":            fn("len", capt("x")),
	"text(x)":           fn("text", capt("x")),
	"-x":                un("-", capt("x")),
	"!x":                un("!", capt("x")),
	"x + 1":             bin("+", capt("x"), ilit(1)),
	"x == nil":          bin("==", capt("x"), &grammar.NilLit{}),
	"x[0]":              bin("[]", capt("x"), ilit(0)),
	"x.(int)":           &grammar.Cast{X: capt("x"), Type: &grammar.TypeRef{Name: "int"}},
	"list(x)":           fn("list", capt("x")),
	"len(list(x, 1))":   fn("len", fn("list", capt("x"), ilit(1))),
	"foldl count xs":    fn("foldl", ilit(0), capt("xs"), lam(bin("+", param("a"), ilit(1)), "a", "s")),
	"foldl over x":      fn("foldl", ilit(0), capt("x"), lam(param("a"), "a", "s")),
	"foldl sum xs":      fn("foldl", ilit(0), capt("xs"), lam(bin("+", param("a"), param("s")), "a", "s")),
	"list(x, 2)[1]":     bin("[]", fn("list", capt("x"), ilit(2)), ilit(1)),
	"list(x, 2)[0]":     bin("[]", fn("list", capt("x"), ilit(2)), ilit(0)),
	"filter xs":         fn("filter", capt("xs"), lam(blit(true), "s")),
	"filter xs by s":    fn("filter", capt("xs"), lam(param("s"), "s")),
	"false && x":        bin("&&", blit(false), capt("x")),
	"true || x":         bin("||", blit(true), capt("x")),
	"true && x":         bin("&&", blit(true), capt("x")),
	"x && false":        bin("&&", capt("x"), blit(false)),
	"x && true":         bin("&&", capt("x"), blit(true)),
	"x || true":         bin("||", capt("x"), blit(true)),
	"x || false":        bin("||", capt("x"), blit(false)),
	"!(x && false)":     un("!", bin("&&", capt("x"), blit(false))),
	"1/0 == 0 && x":     bin("&&", bin("==", bin("/", ilit(1), ilit(0)), ilit(0)), capt("x")),
	"x && 1/0 == 0":     bin("&&", capt("x"), bin("==", bin("/", ilit(1), ilit(0)), ilit(0))),
	"(x && false) || n": bin("||", bin("&&", capt("x"), blit(false)), bin("==", vr("n"), ilit(2))),
	"v % 2 == 0":        bin("==", bin("%", vr("v"), ilit(2)), ilit(0)),
	"n + 1":             bin("+", vr("n"), ilit(1)),
	"w":                 vr("w"),
	"len(xs) > 1":       bin(">", fn("len", capt("xs")), ilit(1)),
}

// unknownCandidates are values that x and v may turn out to have: Unknown stands for any of them.
var unknownCandidates = []V{
	Nil, Int(0), Int(1), Int(-1), Str(""), Str("ab"), Bool(true), Bool(false), List(Int(1)),
	Node(StringNode, "ab", 0, 2, nil, map[string]V{"c": Str("q")}), Node(ListNode, "", 0, 0, nil, nil),
}

func unknownEnv(x V) *TermEnv {
	return &TermEnv{
		Captures: map[string]V{"x": x, "xs": Node(ListNode, "", 0, 3, []V{x, Int(2)}, nil)},
		Vars:     map[string]V{"v": x, "n": Int(2)},
		End:      3,
	}
}

func hasUnknown(v V) bool {
	if v.Kind == KUnknown {
		return true
	}
	for _, x := range v.Items {
		if hasUnknown(x) {
			return true
		}
	}
	for _, m := range []map[string]V{v.Fields, v.Caps} {
		for _, x := range m {
			if hasUnknown(x) {
				return true
			}
		}
	}
	return false
}

// TestUnknownNeverLeaks holds Eval and Decide to soundness: a value, a failure or a verdict that they
// report as known with x unknown is the one that every value of x gives. Without Unknown, Decide is
// the predicate of the value that Eval returns.
func TestUnknownNeverLeaks(t *testing.T) {
	for name, term := range unknownTerms {
		u := unknownEnv(Unknown)
		v, msg := u.Eval(term)
		holds, known := u.Decide(term)
		for _, c := range unknownCandidates {
			ce := unknownEnv(c)
			cv, cmsg := ce.Eval(term)
			if ch, ck := ce.Decide(term); !ck || ch != Holds(cv, cmsg) {
				t.Errorf("%s with x = %v: Decide = %v, %v; the value %v, %q holds: %v", name, c, ch, ck, cv, cmsg, Holds(cv, cmsg))
			}
			if !hasUnknown(v) && (v.String() != cv.String() || msg != cmsg) {
				t.Errorf("%s: with x unknown it is %v, %q; with x = %v it is %v, %q", name, v, msg, c, cv, cmsg)
			}
			if known && holds != Holds(cv, cmsg) {
				t.Errorf("%s: with x unknown it holds: %v; with x = %v: %v", name, holds, c, Holds(cv, cmsg))
			}
		}
	}
}

// TestUnknownResults pins what the evaluation knows with x unknown, so that the soundness above does
// not hold only because everything is unknown.
func TestUnknownResults(t *testing.T) {
	u := unknownEnv(Unknown)
	for _, c := range []struct {
		name  string
		value string // "unknown", "partial" (known, with an unknown part), or the String of the value
		msg   string
		holds string // "unknown", "true" or "false"
	}{
		{"x.Start", "unknown", "", "unknown"},
		{"x.$c", "unknown", "", "unknown"},
		{"len(x)", "unknown", "", "unknown"},
		{"text(x)", "unknown", "", "unknown"},
		{"-x", "unknown", "", "unknown"},
		{"!x", "unknown", "", "unknown"},
		{"x + 1", "unknown", "", "unknown"},
		{"x == nil", "unknown", "", "unknown"},
		{"x[0]", "unknown", "", "unknown"},
		{"x.(int)", "unknown", "", "unknown"},
		{"list(x)", "partial", "", "true"},
		{"len(list(x, 1))", "2", "", "true"},
		{"foldl count xs", "2", "", "true"},
		{"foldl over x", "unknown", "", "unknown"},
		{"foldl sum xs", "unknown", "", "unknown"},
		{"list(x, 2)[1]", "2", "", "true"},
		{"list(x, 2)[0]", "unknown", "", "unknown"},
		{"filter xs", "partial", "", "true"},
		{"filter xs by s", "unknown", "", "unknown"},
		{"false && x", "false", "", "false"},
		{"true || x", "true", "", "true"},
		{"true && x", "unknown", "", "unknown"},
		{"x && false", "unknown", "", "false"},
		{"x && true", "unknown", "", "unknown"},
		{"x || true", "unknown", "", "unknown"},
		{"x || false", "unknown", "", "unknown"},
		{"!(x && false)", "unknown", "", "unknown"},
		{"1/0 == 0 && x", "nil", "division by zero", "false"},
		{"x && 1/0 == 0", "unknown", "", "false"},
		{"(x && false) || n", "unknown", "", "unknown"},
		{"v % 2 == 0", "unknown", "", "unknown"},
		{"n + 1", "3", "", "true"},
		{"w", "nil", "undefined variable w", "false"},
		{"len(xs) > 1", "true", "", "true"},
	} {
		term, ok := unknownTerms[c.name]
		if !ok {
			t.Fatalf("no term %s", c.name)
		}
		v, msg := u.Eval(term)
		got := v.String()
		switch {
		case v.Kind == KUnknown:
			got = "unknown"
		case hasUnknown(v):
			got = "partial"
		}
		if got != c.value || msg != c.msg {
			t.Errorf("%s = %s, %q; want %s, %q", c.name, got, msg, c.value, c.msg)
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
