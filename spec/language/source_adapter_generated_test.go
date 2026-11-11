package language_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/value"
)

// TestGeneratedSourceParserMatchesEngine runs the canonical grammar as a generated Go parser on the
// sources of the differential tests (the grammars of the repository, the documentation examples and
// mutated windows of them) and requires the trees, with their positions, and the errors of the engine
// that the other tests of this package parse with. The adapter reads the tree of the engine, so a
// generated parser that agrees with it is covered by the comparison with the hand-written parser.
func TestGeneratedSourceParserMatchesEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skip("go command not found")
	}
	// The generator and the comparison compile with the engine, as the tests of the generator do.
	gentest.Compile = func(g *grammar.Grammar, opts compile.Options) (*compile.Program, func() (*compile.Program, error), error) {
		prog, err := engine.Compile(g, opts)
		if err != nil {
			return nil, nil, err
		}
		return prog.Static(), func() (*compile.Program, error) {
			rec, err := prog.Recognizer()
			if err != nil {
				return nil, err
			}
			return rec.Static(), nil
		}, nil
	}
	gentest.Reference = func(t *testing.T, src string, opts compile.Options) gentest.ReferenceParse {
		g, err := compiletest.Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		prog, err := engine.Compile(g, opts)
		if err != nil {
			t.Fatal(err)
		}
		return func(input string, bytes, recognize bool) (*value.Node, error) {
			unit := engine.CodePoints
			if bytes {
				unit = engine.Bytes
			}
			return prog.ParseWith("main", input, engine.ParseOptions{Unit: unit, Recognize: recognize, Expectations: true})
		}
	}
	_, self := sourceGrammar(t)
	corpus := mutationCorpus(t)
	inputs := append([]string(nil), corpus...)
	inputs = append(inputs, frontendRejections...)
	inputs = append(inputs, frontendAgreements...)
	rng := rand.New(rand.NewSource(31))
	for i := 0; i < generatedMutations; i++ {
		inputs = append(inputs, mutatedSource(rng, corpus))
	}
	cases := []gentest.Case{{Name: "zp.zp", Src: self, Inputs: inputs}}
	t.Run("values", func(t *testing.T) { gentest.GeneratedParsersCorpus(t, goBin, cases, gen.GenOptions{}) })
	t.Run("source info", func(t *testing.T) { gentest.GeneratedParsersCorpus(t, goBin, cases, gen.GenOptions{SourceInfo: true}) })
}
