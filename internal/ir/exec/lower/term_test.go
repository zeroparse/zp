package lower_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/core"
	irbuild "zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/lower"
)

var update = flag.Bool("update", false, "update golden files")

// TestTermDumpGolden lists the terms of every grammar of testdata/grammars lowered to Exec IR, against
// the goldens in testdata/terms: a review aid that shows what a change of the lowering does to the
// printed programs. A term that Term does not lower is listed by the kinds that it holds and Term
// does not lower.
func TestTermDumpGolden(t *testing.T) {
	paths, err := filepath.Glob("../../../../testdata/grammars/*/*.zp")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no grammars: %v", err)
	}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".zp")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			g, err := compiletest.Parse(string(src))
			if err != nil {
				t.Fatal(err)
			}
			prog, err := compiletest.Compile(g, compile.Options{})
			if err != nil {
				t.Fatal(err)
			}
			m, err := irbuild.CoreFrom(prog, nil)
			if err != nil {
				t.Fatal(err)
			}
			got := dumpTerms(t, m)
			golden := filepath.Join("testdata", "terms", name+".golden")
			if *update {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("the lowered terms differ from %s; run go test -run TestTermDumpGolden -update", golden)
			}
		})
	}
}

// dumpTerms lists the terms of the rules of m in the order of the rules: their actions, the actions of
// the lines of Operators Rules, and the terms of predicates and bindings.
func dumpTerms(t *testing.T, m *core.Module) string {
	var b strings.Builder
	dump := func(rule, what string, x core.Term) {
		if x == nil {
			return
		}
		fmt.Fprintf(&b, "## %s %s\n", rule, what)
		l, err := lower.Term(x, lower.TermOptions{})
		if err != nil {
			t.Fatalf("%s %s: %v", rule, what, err)
		}
		b.WriteString(exec.FuncString(l.Func))
		b.WriteString("\n")
	}
	for _, r := range m.Rules {
		if r == nil {
			continue
		}
		if r.Finish.Kind == core.FinishAction {
			dump(r.Name, "action", r.Finish.Action)
		}
		core.Walk(r.Body, func(n core.Node) {
			switch n := n.(type) {
			case *core.Pred:
				dump(r.Name, "predicate", n.Term)
			case *core.BindNode:
				dump(r.Name, "binding "+n.Var, n.Term)
			case *core.OperatorsRule:
				for i, l := range n.Lines() {
					dump(r.Name, fmt.Sprintf("line %d", i), l.Action)
				}
			}
		})
	}
	return b.String()
}

// TestTermRejectsMalformedTerms checks that Term reports an error, and no lowered function, for terms
// that the front end does not build, so that a generator fails loudly instead of printing them.
func TestTermRejectsMalformedTerms(t *testing.T) {
	one := &core.Const{Value: 1}
	f := &core.Lambda{Params: []string{"e"}}
	e := &core.Local{Kind: core.LocalParam, Name: "e", Lambda: f}
	f.Body = e
	mapped := &core.Builtin{Func: "map", Args: []core.Term{&core.Builtin{Func: "list", Args: []core.Term{one}}, f}}
	for _, x := range []core.Term{
		e, // a parameter outside its lambda
		&core.Binary{Op: "+", L: mapped, R: e},
		&core.Builtin{Func: "nope", Args: []core.Term{one}},
		&core.Builtin{Func: "len", Args: []core.Term{one, one}},
		&core.Builtin{Func: "map", Args: []core.Term{one, one}},
		&core.Lambda{Body: one},
		&core.Binary{Op: "+", L: one, R: &core.Lambda{Params: []string{"e"}, Body: one}},
	} {
		if l, err := lower.Term(x, lower.TermOptions{}); err == nil || l != nil {
			t.Errorf("Term(%s) = %v, %v; want an error", core.TermString(x), l, err)
		}
	}
	if _, err := lower.Term(&core.Builtin{Func: "len", Args: []core.Term{one}}, lower.TermOptions{}); err != nil {
		t.Errorf("Term(len(1)): %v", err)
	}
	if _, err := lower.Term(mapped, lower.TermOptions{}); err != nil {
		t.Errorf("Term(%s): %v", core.TermString(mapped), err)
	}
}

// TestTermLowersNoFunctionValue pins the closed set of built-ins and where a lambda may stand, which is
// why a lowered term never yields a function and the targets keep no check for one: a lambda is accepted
// only as the last argument of map, filter, foldl and foldr, with one parameter for the first two and two
// for the folds, and no other built-in takes one.
func TestTermLowersNoFunctionValue(t *testing.T) {
	one := &core.Const{Value: 1}
	list := &core.Builtin{Func: "list", Args: []core.Term{one}}
	lambda := func(params ...string) *core.Lambda { return &core.Lambda{Params: params, Body: one} }
	b := func(fn string, args ...core.Term) core.Term { return &core.Builtin{Func: fn, Args: args} }
	for _, c := range []struct {
		name string
		term core.Term
		ok   bool
	}{
		{"len", b("len", one), true},
		{"text", b("text", one), true},
		{"list", b("list", one, one), true},
		{"concat", b("concat", list, list), true},
		{"map", b("map", list, lambda("e")), true},
		{"filter", b("filter", list, lambda("e")), true},
		{"foldl", b("foldl", one, list, lambda("a", "e")), true},
		{"foldr", b("foldr", one, list, lambda("a", "e")), true},
		{"len of a lambda", b("len", lambda("e")), false},
		{"text of a lambda", b("text", lambda("e")), false},
		{"list of a lambda", b("list", lambda("e")), false},
		{"list with a lambda among its elements", b("list", one, lambda("e")), false},
		{"concat of a lambda", b("concat", lambda("e")), false},
		{"map of a lambda", b("map", lambda("e"), lambda("e")), false},
		{"map with a function of two parameters", b("map", list, lambda("a", "e")), false},
		{"filter with a function of two parameters", b("filter", list, lambda("a", "e")), false},
		{"foldl with a function of one parameter", b("foldl", one, list, lambda("e")), false},
		{"foldr with a lambda as the seed", b("foldr", lambda("a", "e"), list, lambda("a", "e")), false},
		{"lambda returning a lambda", b("map", list, &core.Lambda{Params: []string{"e"}, Body: lambda("x")}), false},
		{"a built-in that is not one", b("apply", list, lambda("e")), false},
		{"a lambda alone", lambda("e"), false},
	} {
		l, err := lower.Term(c.term, lower.TermOptions{})
		if (err == nil) != c.ok || (err == nil) != (l != nil) {
			t.Errorf("%s: Term(%s) = %v, %v; want ok %v", c.name, core.TermString(c.term), l, err, c.ok)
		}
	}
}
