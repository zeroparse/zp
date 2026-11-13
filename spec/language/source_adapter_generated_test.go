package language_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/runtime/parse"
)

// TestGeneratedSourceParserMatchesEngine runs the canonical grammar as a generated Go parser on the
// sources of the differential tests (the grammars of the repository, the documentation examples and
// mutated windows of them) and requires the trees, with their positions, and the errors of the engine.
// The comparisons with the hand-written parser read the typed values of the checked-in parser; this
// test holds the generated parser to the engine, which the tests of mutated grammars run.
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
		return func(input string, bytes, recognize bool) (*parse.Node, error) {
			unit := parse.CodePoints
			if bytes {
				unit = parse.Bytes
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
	// Control: the generated parser builds the name of a rule from the expression of its body. The
	// comparison must notice.
	t.Run("control", func(t *testing.T) {
		hits := 0
		got := gentest.GeneratedParsersMismatches(t, goBin, cases, gen.GenOptions{}, func(code []byte) []byte {
			changed, n := ruleNameFromExpr(string(code))
			hits += n
			return []byte(changed)
		})
		if hits == 0 {
			t.Fatal("the generated parser has no construction of a rule to change")
		}
		if len(got) == 0 {
			t.Errorf("the comparison does not notice a changed rule name (%d changes)", hits)
		}
	})
	t.Run("values", func(t *testing.T) { gentest.GeneratedParsersCorpus(t, goBin, cases, gen.GenOptions{}) })
	t.Run("source info", func(t *testing.T) { gentest.GeneratedParsersCorpus(t, goBin, cases, gen.GenOptions{SourceInfo: true}) })
}

// ruleNameFromExpr changes the action of the generated parser that makes a RuleDef so that it reads
// the Name of the rule's body from its Expr member, and reports how many reads it changed.
func ruleNameFromExpr(code string) (string, int) {
	const from, to = `c.Cap(1), "Name", "Name")`, `c.Cap(1), "Expr", "Expr")`
	made := strings.Index(code, `c.TermStruct("RuleDef", `)
	if made < 0 {
		return code, 0
	}
	action := strings.LastIndex(code[:made], ".Action = func(")
	if action < 0 {
		return code, 0
	}
	body := code[action:made]
	n := strings.Count(body, from)
	return code[:action] + strings.ReplaceAll(body, from, to) + code[made:], n
}
