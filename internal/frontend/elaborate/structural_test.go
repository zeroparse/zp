package elaborate_test

import (
	"fmt"
	"reflect"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/testing/compiletest"
	"zeroparse.dev/zp/internal/testing/factstest"
)

// structuralInput returns the input of an untyped module of the rules of the compiled program:
// no twins, and none of the flags the compiler computes.
func structuralInput(prog *compile.Program) *elaborate.Input {
	typed := prog.CoreInput()
	in := &elaborate.Input{ByName: map[string]*elaborate.Rule{}, Vars: typed.Vars}
	for _, r := range typed.Rules[:len(prog.RuleList)] {
		br := &elaborate.Rule{ID: r.ID, Name: r.Name, Where: r.Where, Def: r.Def, Body: r.Body, Action: r.Action, Splice: r.Splice}
		in.Rules = append(in.Rules, br)
		in.ByName[br.Name] = br
	}
	for _, r := range typed.Starts {
		in.Starts = append(in.Starts, in.ByName[r.Name])
	}
	return in
}

// shape returns the sequence of node types of the module without captures, which the typed and the untyped build
// must agree on.
func shape(m *core.Module, n int) []string {
	var out []string
	var walk func(core.Node)
	walk = func(n core.Node) {
		// A lean rule of the typed build leaves out the captures its action does not read.
		if _, ok := n.(*core.Capture); !ok {
			out = append(out, fmt.Sprintf("%T", n))
		}
		for _, c := range core.Children(n) {
			walk(c)
		}
	}
	for _, r := range m.Rules[:n] {
		out = append(out, "rule "+r.Name)
		walk(r.Body)
	}
	return out
}

// TestStructuralFacts requires the untyped module of every corpus grammar and variant to have the
// structure of the typed module and the same facts, rule by rule and on every rule body, and the
// same fact errors: the facts read structure only.
func TestStructuralFacts(t *testing.T) {
	t.Parallel()
	for _, path := range factstest.Corpus {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			g, err := compiletest.Parse(factstest.Read(t, path))
			if err != nil {
				t.Fatal(err)
			}
			for _, opts := range []compile.Options{{}, {Recognition: compile.RecognizeKeepStreamValues}, {NoProjections: true}, {DefaultValue: true}} {
				prog, err := compiletest.Compile(g, opts)
				if err != nil {
					continue
				}
				typed, err := elaborate.CoreFrom(prog, nil)
				if err != nil {
					t.Fatal(err)
				}
				n := len(prog.RuleList) // the typed module has the twins after them
				untyped, err := elaborate.Structural(structuralInput(prog), nil)
				if err != nil {
					t.Fatal(err)
				}
				if untyped.Typed || len(untyped.Types) != 0 {
					t.Fatalf("untyped module is typed")
				}
				if got, want := shape(untyped, n), shape(typed, n); !reflect.DeepEqual(got, want) {
					for i := range got {
						if i >= len(want) || got[i] != want[i] {
							t.Fatalf("node structure differs at %d: %v, want %v", i, got[max(0, i-6):i+3], want[max(0, i-6):min(len(want), i+3)])
						}
					}
					t.Fatalf("node structure differs: %d nodes, want %d", len(got), len(want))
				}
				tf, uf := corepass.Compute(typed, nil), corepass.Compute(untyped, nil)
				if !reflect.DeepEqual(uf.Rules[:n], tf.Rules[:n]) {
					t.Fatalf("rule facts differ:\n%+v\nwant\n%+v", uf.Rules[:n], tf.Rules[:n])
				}
				for i := 0; i < n; i++ {
					tn, un := typed.Rules[i].Body, untyped.Rules[i].Body
					if uf.Succeeds(un) != tf.Succeeds(tn) || uf.Fails(un) != tf.Fails(tn) ||
						!reflect.DeepEqual(uf.LeftCalls(un, false), tf.LeftCalls(tn, false)) ||
						!reflect.DeepEqual(uf.LeftCalls(un, true), tf.LeftCalls(tn, true)) {
						t.Errorf("rule %s: body facts differ", typed.Rules[i].Name)
					}
				}
				if got, want := fmt.Sprint(uf.Errors(untyped, true)), fmt.Sprint(tf.Errors(typed, true)); got != want {
					t.Errorf("fact errors differ: %s, want %s", got, want)
				}
			}
		})
	}
}

// TestStructuralFactsNegative checks that the comparison can fail: a rule whose written call
// is removed from one module changes the facts.
func TestStructuralFactsNegative(t *testing.T) {
	t.Parallel()
	g, err := compiletest.Parse("@start\nrule a = b \"x\"\nrule b = a / \"y\"")
	if err != nil {
		t.Fatal(err)
	}
	prog, err := compiletest.Compile(g, compile.Options{LeftRecursionBindings: true})
	if err != nil {
		t.Fatal(err)
	}
	in := structuralInput(prog)
	m, err := elaborate.Structural(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := corepass.Compute(m, nil)
	if !f.Rules[0].Cyclic() {
		t.Fatal("rule a is not left-recursive")
	}
	in.Rules[0].Body = &grammar.Literal{Value: "x"}
	m2, err := elaborate.Structural(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(corepass.Compute(m2, nil).Rules, f.Rules) {
		t.Fatal("facts do not see the rule body")
	}
}
