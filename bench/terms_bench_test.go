package bench

import (
	"testing"

	"zeroparse.dev/zp"
	gtermsheavy "zeroparse.dev/zp/internal/bench/gen/termsheavy"
	gtermspred "zeroparse.dev/zp/internal/bench/gen/termspred"
	"zeroparse.dev/zp/internal/bench/inputs"
)

// termWorkloads measure the cost of evaluating terms. pred runs four predicate terms per line and builds
// one int; heavy builds a struct of five fields from maps, folds, filters and comparisons of the words of
// the line. They are separate from workloads because they have no counterpart in another implementation.
var termWorkloads = []struct {
	name, grammar string
	gen           func(input string) error
	ast           func(input string) error
	rec           func(input string) error
}{
	{"TermsPredicates", "grammars/terms/pred.zp",
		func(s string) error { return genErr(gtermspred.Parse(s)) },
		func(s string) error { return genErr(gtermspred.ParseAST(s)) },
		func(s string) error { return gtermspred.Recognize(s) }},
	{"TermsHeavy", "grammars/terms/heavy.zp",
		func(s string) error { return genErr(gtermsheavy.Parse(s)) },
		func(s string) error { return genErr(gtermsheavy.ParseAST(s)) },
		func(s string) error { return gtermsheavy.Recognize(s) }},
}

// TestTermWorkloads checks that every term workload parses on every backend and in generated Go.
func TestTermWorkloads(t *testing.T) {
	for _, w := range termWorkloads {
		t.Run(w.name, func(t *testing.T) {
			p := load(t, w.grammar)
			input := inputs.TermsInput(50)
			for _, be := range backends {
				if _, err := p.Parse(input, zp.WithBackend(be.b)); err != nil {
					t.Fatalf("%s: %v", be.name, err)
				}
			}
			for name, f := range map[string]func(string) error{"generated": w.gen, "generated_ast": w.ast, "recognize": w.rec} {
				if err := f(input); err != nil {
					t.Errorf("%s: %v", name, err)
				}
			}
		})
	}
}

// BenchmarkTerms parses the term workloads with every backend, the generated parsers and ParseAST.
func BenchmarkTerms(b *testing.B) {
	for _, w := range termWorkloads {
		b.Run(w.name, func(b *testing.B) {
			p := load(b, w.grammar)
			input := inputs.TermsInput(2000)
			for _, be := range backends {
				b.Run(be.name, func(b *testing.B) {
					b.SetBytes(int64(len(input)))
					b.ReportAllocs()
					for b.Loop() {
						if _, err := p.Parse(input, zp.WithBackend(be.b)); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
			for name, f := range map[string]func(string) error{"generated": w.gen, "generated_ast": w.ast, "generated_recognize": w.rec} {
				b.Run(name, func(b *testing.B) {
					b.SetBytes(int64(len(input)))
					b.ReportAllocs()
					for b.Loop() {
						if err := f(input); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
