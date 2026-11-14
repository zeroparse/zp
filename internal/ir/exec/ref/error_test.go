package ref

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
)

func typeOf(s string) grammar.TypeExpr {
	switch s {
	case "*error":
		return &grammar.OptionalType{Elem: tref("error")}
	case "error | int":
		return &grammar.JoinType{Types: []grammar.TypeExpr{tref("error"), tref("int")}}
	case "node[_]":
		return nodeOf(tref("_"))
	case "node[string]":
		return nodeOf(tref("string"))
	}
	return tref(s)
}

// TestErrorValueCast pins which types admit the error values of recovery (spec/language/actions.md,
// Values and Constructors): error admits a skipped range and a missing token, missing only the token,
// and _ every value; no other type does, node[_] among them.
func TestErrorValueCast(t *testing.T) {
	skipped, missing := ErrorValue(1, 3, false), ErrorValue(3, 3, true)
	env := &TermEnv{Captures: map[string]V{"s": skipped, "m": missing, "n": Node(StringNode, "ab", 0, 2, nil, nil)},
		Aliases: map[string]grammar.TypeExpr{"E": tref("error"), "M": &grammar.TypeGroup{Type: tref("missing")}}}
	for _, c := range []struct {
		typ        string
		skip, miss bool // whether the skipped range and the missing token are admitted
		node       bool // whether a string node is
	}{
		{"error", true, true, false}, {"missing", false, true, false}, {"_", true, true, true},
		{"*error", true, true, false}, {"error | int", true, true, false}, {"E", true, true, false}, {"M", false, true, false},
		{"node[_]", false, false, true}, {"node[string]", false, false, true}, {"int", false, false, false},
		{"string", false, false, false}, {"bool", false, false, false}, {"N", false, false, false},
	} {
		for _, v := range []struct {
			name string
			ok   bool
		}{{"s", c.skip}, {"m", c.miss}, {"n", c.node}} {
			val, msg := env.Eval(cast(capt(v.name), typeOf(c.typ)))
			if v.ok != (msg == "") {
				t.Errorf("%s.(%s): %v, %q; want admitted: %v", v.name, c.typ, val, msg, v.ok)
			}
			if !v.ok && !strings.HasPrefix(msg, "cannot cast "+map[string]string{"s": "error", "m": "missing", "n": "node[string]"}[v.name]+" to ") {
				t.Errorf("%s.(%s) fails with %q", v.name, c.typ, msg)
			}
		}
	}
}

// TestErrorValueTerms pins the other terms that the model gives an error value: it is not nil, and the list
// built-ins take it as an empty list.
func TestErrorValueTerms(t *testing.T) {
	env := &TermEnv{Captures: map[string]V{"e": ErrorValue(1, 3, false)}}
	for _, c := range []struct {
		name string
		term grammar.Term
		want string
	}{
		{"e != nil", bin("!=", capt("e"), &grammar.NilLit{}), "true"},
		{"e == nil", bin("==", capt("e"), &grammar.NilLit{}), "false"},
		{"map(e)", fn("map", capt("e"), lam(param("i"), "i")), "[]"},
		{"foldl(0, e)", fn("foldl", ilit(7), capt("e"), lam(param("a"), "a", "i")), "7"},
		{"filter(e)", fn("filter", capt("e"), lam(&grammar.BoolLit{Value: true}, "i")), "[]"},
		{"concat(e)", fn("concat", capt("e")), "[]"},
	} {
		if v, msg := env.Eval(c.term); msg != "" || v.String() != c.want {
			t.Errorf("%s = %v, %q; want %s", c.name, v, msg, c.want)
		}
	}
	// A term that the specification leaves open for an error value is not modelled and does not give a value.
	for name, term := range map[string]grammar.Term{
		"e.Start": mem(capt("e"), "Start"), "len(e)": fn("len", capt("e")), "text(e)": fn("text", capt("e")),
		"e == 1": bin("==", capt("e"), ilit(1)),
	} {
		func() {
			defer func() {
				r := recover()
				if name == "e == 1" {
					return // an invalid comparison is an evaluation error, which Eval reports
				}
				if s, _ := r.(string); !strings.Contains(s, "is not modelled") {
					t.Errorf("%s: panic %v, want an unmodelled operation", name, r)
				}
			}()
			if _, msg := env.Eval(term); name == "e == 1" && msg == "" {
				t.Errorf("%s gives a value", name)
			}
		}()
	}
}
