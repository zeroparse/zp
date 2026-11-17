package ref

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func typeOf(s string) ast.TypeExpr {
	switch s {
	case "*error":
		return &ast.OptionalType{Elem: tref("error")}
	case "error | int":
		return &ast.JoinType{Types: []ast.TypeExpr{tref("error"), tref("int")}}
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
	skipped, missing := ErrorValue("b;", 1, 3, "1:2: syntax error"), MissingToken(3, ";", "1:4: syntax error")
	env := &TermEnv{Captures: map[string]V{"s": skipped, "m": missing, "n": Node(StringNode, "ab", 0, 2, nil, nil)},
		Aliases: map[string]ast.TypeExpr{"E": tref("error"), "M": &ast.TypeGroup{Type: tref("missing")}}}
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

// TestErrorValueTerms pins the other terms that the model gives an error value, a skipped range and a missing
// token: it is not nil, it is a node with a range and no value, whose text is the input of its range and whose
// length is that of nil, and the list built-ins take it as an empty list. A missing token has the string
// fields expected and message; every other field reads nil.
func TestErrorValueTerms(t *testing.T) {
	env := &TermEnv{Captures: map[string]V{"e": ErrorValue("b;", 1, 3, "1:2: syntax error"), "m": MissingToken(3, ";", `1:4: syntax error: expected ";"`)}}
	for _, c := range []struct {
		name string
		term ast.ActionExpr
		want string
	}{
		{"e != nil", bin("!=", capt("e"), &ast.NilLit{}), "true"},
		{"e == nil", bin("==", capt("e"), &ast.NilLit{}), "false"},
		{"map(e)", fn("map", capt("e"), lam(param("i"), "i")), "[]"},
		{"foldl(0, e)", fn("foldl", ilit(7), capt("e"), lam(param("a"), "a", "i")), "7"},
		{"filter(e)", fn("filter", capt("e"), lam(&ast.BoolLit{Value: true}, "i")), "[]"},
		{"concat(e)", fn("concat", capt("e")), "[]"},
		{"e.Start", mem(capt("e"), "Start"), "1"},
		{"e.End", mem(capt("e"), "End"), "3"},
		{"e.Value", mem(capt("e"), "Value"), "nil"},
		{"e.F", mem(capt("e"), "F"), "nil"},
		{"e.$y", mem(capt("e"), "$y"), "nil"},
		{"len(e)", fn("len", capt("e")), "0"},
		{"text(e)", fn("text", capt("e")), `"b;"`},
		{"m.Start", mem(capt("m"), "Start"), "3"},
		{"m.End", mem(capt("m"), "End"), "3"},
		{"m.Value", mem(capt("m"), "Value"), "nil"},
		{"len(m)", fn("len", capt("m")), "0"},
		{"text(m)", fn("text", capt("m")), `""`},
		{"m.expected", mem(capt("m"), "expected"), `";"`},
		{"m.message", mem(capt("m"), "message"), `"1:4: syntax error: expected \";\""`},
		{"m.F", mem(capt("m"), "F"), "nil"},
		{"e.expected", mem(capt("e"), "expected"), "nil"},
		{"e.message", mem(capt("e"), "message"), "nil"},
	} {
		if v, msg := env.Eval(c.term); msg != "" || v.String() != c.want {
			t.Errorf("%s = %v, %q; want %s", c.name, v, msg, c.want)
		}
	}
	// An invalid comparison is an evaluation error, which Eval reports.
	if v, msg := env.Eval(bin("==", capt("e"), ilit(1))); msg == "" {
		t.Errorf("e == 1 gives %v", v)
	}
	// A conversion of a recovered value does not type-check, so the evaluator does not take one.
	for _, name := range []string{"e", "m"} {
		func() {
			defer func() {
				if s, _ := recover().(string); !strings.Contains(s, "which does not type-check") {
					t.Errorf("N{%s}: panic %q, want a term that does not type-check", name, s)
				}
			}()
			env.Eval(&ast.New{Type: "N", Args: []ast.ActionExpr{capt(name)}})
		}()
	}
}
