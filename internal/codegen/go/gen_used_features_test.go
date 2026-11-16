package gogen_test

import (
	"math/rand"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// usedFeatureCases are grammars that use each feature of the library (memoization, left recursion,
// recovery, Operators Rules, variables, skip runs and scan sites), with inputs, besides those of
// FlagParityCases, which use few.
func usedFeatureCases() []codegentest.Case {
	cases := append(codegentest.FlagParityCases(), codegentest.ActionVariableCases...)
	cases = append(cases,
		codegentest.Case{Name: "memo", Src: "rule main = a \"x\" / a \"y\" / a \"z\" $$\nrule a = \"(\" a \")\" / \"b\"", Inputs: []string{"((b))z", "(b)y", "bx", "(b", ""}},
		codegentest.Case{Name: "left recursion", Src: "rule main = e $$\nrule e = e \"+\" n / e \"-\" n / n\nrule n = [0-9]+", Inputs: []string{"1+2-3", "1", "1+", "+", ""}},
		codegentest.Case{Name: "operators", Src: codegentest.Arith, Inputs: []string{"1+2*3", "(1+2)*3", "1+", ""}},
	)
	rng := rand.New(rand.NewSource(80))
	random := func(alphabet string) []string {
		letters := strings.Split(alphabet, "")
		ins := []string{""}
		for range 12 {
			var b strings.Builder
			for range rng.Intn(10) {
				b.WriteString(letters[rng.Intn(len(letters))])
			}
			ins = append(ins, b.String())
		}
		return ins
	}
	for _, g := range codegentest.SkipRunGrammars {
		cases = append(cases, codegentest.Case{Name: g.Name, Src: g.Src, Inputs: random(g.Alphabet)})
	}
	for _, g := range codegentest.ScanSiteGrammars {
		cases = append(cases, codegentest.Case{Name: g.Name, Src: g.Src, Inputs: random(g.Alphabet)})
	}
	return cases
}

// TestUsedFeaturesEmbedParity compares parsers that carry their own copy of the library, narrowed to
// their features, with the engine on usedFeatureCases, in every mode of nodeRouteModes and in the
// Document route, and does the same with the optimization used-features off, whose copy is the whole
// library. Each copy must build, so this also checks that pruning leaves no dangling name. The
// default tier runs the narrowed copies of the plain and Document routes, the long tier
// every combination.
func TestUsedFeaturesEmbedParity(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	codegentest.PinOrdinary(t)
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	modes := append(nodeRouteModes[:len(nodeRouteModes):len(nodeRouteModes)], struct {
		name string
		opts codegen.GenOptions
	}{"document", codegen.GenOptions{Document: true}})
	for _, mode := range modes {
		for _, off := range []bool{false, true} {
			name := mode.name
			if off {
				name += "/no-used-features"
			}
			if !codegentest.Long() && (off || mode.name != "plain" && mode.name != "document") {
				continue
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				o := mode.opts
				o.Lib = codegen.LibEmbed
				if off {
					o.NoOpt = []string{codegen.UsedFeatures}
				}
				codegentest.GeneratedParsersCorpus(t, goBin, usedFeatureCases(), o)
			})
		}
	}
}

