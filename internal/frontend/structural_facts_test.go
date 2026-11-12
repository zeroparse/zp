package frontend

import (
	"fmt"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/core/facts"
)

// TestStructuralInputFacts requires the untyped module that Run checks before compiling, built
// from structuralInput, to have the rule facts of the typed module of every corpus grammar and
// variant the compiler accepts, and no fact error: an error there would add itself to the
// compiler's errors only when compilation fails, so a false one would go unnoticed on success.
// Under Recognize the compiler drops the actions of the rules whose values are not needed, so the
// typed module's Vars and Positional describe fewer actions than the grammar has; the untyped
// module keeps the grammar's, and those two facts are not compared.
func TestStructuralInputFacts(t *testing.T) {
	t.Parallel()
	var cyclic atomic.Int64
	t.Run("corpus", func(t *testing.T) {
		for _, path := range factstest.Corpus {
			t.Run(path, func(t *testing.T) {
				t.Parallel()
				cyclic.Add(int64(structuralInputFacts(t, path)))
			})
		}
	})
	if cyclic.Load() == 0 {
		t.Fatal("no corpus rule is on a left-recursion cycle; the comparison would not see one")
	}
}

// structuralInputFacts checks the grammar at path for TestStructuralInputFacts and returns the
// number of rules on a left-recursion cycle it saw.
func structuralInputFacts(t *testing.T, path string) int {
	cyclic := 0
	g, err := compiletest.Parse(factstest.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	for _, opts := range []compile.Options{{}, {Recognize: true}, {NoProjections: true}, {DefaultValue: true}} {
		name := fmt.Sprintf("%+v", opts)
		prog, err := compiletest.Compile(g, opts)
		if err != nil {
			continue
		}
		typed, err := build.CoreFrom(prog, nil)
		if err != nil {
			t.Fatal(err)
		}
		canon, err := compile.Canonicalize(g, opts, desugar.InlineNames{}, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		in, _ := structuralInput(canon, nil)
		untyped, err := build.Structural(in, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		n := len(prog.RuleList) // the typed module has the twins after them
		if len(untyped.Rules) != n {
			t.Fatalf("%s: %d untyped rules, want %d", name, len(untyped.Rules), n)
		}
		tf, uf := facts.Compute(typed, nil), facts.Compute(untyped, nil)
		want, got := slices.Clone(tf.Rules[:n]), slices.Clone(uf.Rules[:n])
		for i := range got {
			if got[i].Cyclic() {
				cyclic++
			}
			if opts.Recognize {
				got[i].Vars, got[i].Positional = nil, false
				want[i].Vars, want[i].Positional = nil, false
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: rule facts differ:\n%+v\nwant\n%+v", name, got, want)
		}
		if errs := uf.Errors(untyped, !opts.LeftRecursionBindings); len(errs) != 0 {
			t.Errorf("%s: fact errors on an accepted grammar: %v", name, errs)
		}
		if errs := structuralErrors(g, opts, desugar.InlineNames{}, nil); errs != nil {
			t.Errorf("%s: structural errors on an accepted grammar: %v", name, errs)
		}
	}
	return cyclic
}
