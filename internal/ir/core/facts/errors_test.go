package facts_test

import (
	"fmt"
	"slices"
	"sync/atomic"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/diag"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/core/facts"
)

// bindingCases are grammars whose only error is a binding before a left-recursive call of a rule
// that reads the variable. Compiled with the compiler's own check, they fail with the list the
// facts must produce; compiled with that check off, they give the module the facts run on.
var bindingCases = []struct{ name, src string }{
	{"direct", "var k int\n@start\nrule main = R $$\nrule R = {k := k + 1} R \"x\" / \"x\""},
	{"after an optional", "var k int\n@start\nrule main = R $$\nrule R = (\"a\" / \"\") {k := k + 1} R \"x\" / \"x\""},
	{"in a repetition", "var k int\n@start\nrule main = R $$\nrule R = ({k := k + 1} \"b\"?)* R \"x\" / \"x\""},
	{"through a lookahead is accepted", "var k int\n@start\nrule main = R $$\nrule R = &({k := k + 1}) \"a\" R \"x\" / \"x\""},
	{"after input is accepted", "var k int\n@start\nrule main = R $$\nrule R = \"a\" {k := k + 1} R \"x\" / \"x\""},
	{"two rules", "var k int\n@start\nrule main = A $$\nrule A = {k := k + 1} B \"x\" / \"x\"\nrule B = A \"y\""},
}

func TestBindingErrorsMatchCompile(t *testing.T) {
	var reporting atomic.Int32
	t.Cleanup(func() {
		if reporting.Load() < 4 {
			t.Errorf("only %d cases make the compiler report an error: the cases do not exercise the check", reporting.Load())
		}
	})
	for _, tc := range bindingCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g, err := compiletest.Parse(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, cerr := compiletest.Compile(g, compile.Options{})
			want := errorLines(cerr)
			prog, err := compiletest.Compile(g, compile.Options{LeftRecursionBindings: true})
			if err != nil {
				t.Fatal(err)
			}
			origin := map[core.Node]grammar.Expr{}
			m, err := build.CoreFrom(prog, origin)
			if err != nil {
				t.Fatal(err)
			}
			f := facts.Compute(m, nil)
			var got []string
			for _, e := range f.Errors(m, true) {
				p := origin[e.Node].(*grammar.Predicate).Term.(*grammar.Bind).Diagnostic()
				got = append(got, fmt.Sprintf("%d:%d: %s", p.Line, p.Col, e.Msg))
			}
			if len(want) > 0 {
				reporting.Add(1)
			}
			if !slices.Equal(got, want) {
				t.Errorf("facts errors\n%q\ncompile errors\n%q", got, want)
			}
			if len(f.Errors(m, false)) != 0 {
				t.Error("errors with bindings accepted")
			}
		})
	}
}

func errorLines(err error) []string {
	var out []string
	if l, ok := err.(diag.ErrorList); ok {
		for _, e := range l {
			out = append(out, e.Error())
		}
	} else if err != nil {
		out = append(out, err.Error())
	}
	return out
}

// TestFactsErrorsClean requires no error on the grammars of the corpus, which compile.
func TestFactsErrorsClean(t *testing.T) {
	t.Parallel()
	for _, path := range factsCorpus() {
		m, f := moduleFacts(t, path)
		if errs := f.Errors(m, true); len(errs) != 0 {
			t.Errorf("%s: %v", path, errs)
		}
	}
}

func factsCorpus() []string { return factstest.Corpus }

// handModule builds a module of the given rule bodies, rule i having ID i; splice marks the
// splice rules. The grammars that these errors need do not compile, so the modules are built by
// hand.
func handModule(bodies []core.Node, splice ...int) *core.Module {
	m := &core.Module{Calls: make([][]int, len(bodies))}
	for i, b := range bodies {
		r := &core.Rule{ID: i, Name: string(rune('a' + i)), Body: b}
		r.Policy.Splice = slices.Contains(splice, i)
		m.Rules = append(m.Rules, r)
		core.Walk(b, func(n core.Node) {
			if c, ok := n.(*core.Call); ok {
				m.Calls[i] = append(m.Calls[i], c.Rule)
			}
		})
	}
	return m
}

func TestStreamOnCycleError(t *testing.T) {
	t.Parallel()
	lit := func(s string) *core.Lit { return &core.Lit{Text: s} }
	stream := &core.Stream{Repeat: &core.Repeat{Max: -1, Body: lit("x")}}
	// a = a "x" / #stream: a is on a left-recursion cycle.
	cyclic := handModule([]core.Node{&core.Choice{Alts: []*core.Alt{
		{Body: &core.Seq{Items: []core.Node{&core.Call{Rule: 0}, lit("x")}}}, {Body: stream}}}})
	core.Analyze(cyclic)
	f := facts.Compute(cyclic, nil)
	errs := f.Errors(cyclic, true)
	if len(errs) != 1 || errs[0].Node != stream || errs[0].Msg != "#stream cannot be used in rule a, which is on a left-recursion cycle; growing the recursion would re-evaluate elements already delivered" {
		t.Fatalf("cyclic: %+v", errs)
	}
	// a = "y" #stream: no cycle.
	plain := handModule([]core.Node{&core.Seq{Items: []core.Node{lit("y"), &core.Stream{Repeat: &core.Repeat{Max: -1, Body: lit("x")}}}}})
	core.Analyze(plain)
	if errs := facts.Compute(plain, nil).Errors(plain, true); len(errs) != 0 {
		t.Fatalf("plain: %+v", errs)
	}
}

func TestSpliceCycleError(t *testing.T) {
	t.Parallel()
	lit := &core.Lit{Text: "x"}
	// a = "x" a, with a splice: its elements are infinite in number.
	self := handModule([]core.Node{&core.Seq{Items: []core.Node{lit, &core.Call{Rule: 0}}}}, 0)
	core.Analyze(self)
	errs := facts.Compute(self, nil).Errors(self, true)
	if len(errs) != 1 || errs[0].Msg != "rule a contains itself in its sequence of elements, so it has no fixed number of them" {
		t.Fatalf("self: %+v", errs)
	}
	// a = ("x" a)? : the call is an element of its own, so it may recur.
	opt := handModule([]core.Node{&core.Opt{Body: &core.Seq{Items: []core.Node{&core.Lit{Text: "x"}, &core.Call{Rule: 0}}}}}, 0)
	core.Analyze(opt)
	if errs := facts.Compute(opt, nil).Errors(opt, true); len(errs) != 0 {
		t.Fatalf("option: %+v", errs)
	}
}
