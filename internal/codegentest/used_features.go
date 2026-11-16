package codegentest

import (
	"math/rand"
	"strings"
)

// UsedFeatureCases are grammars that use each feature of the library (memoization, left recursion,
// recovery, Operators Rules, variables, skip runs and scan sites), with inputs, besides those of
// FlagParityCases, which use few.
func UsedFeatureCases() []Case {
	cases := append(FlagParityCases(), ActionVariableCases...)
	cases = append(cases,
		Case{Name: "memo", Src: "rule main = a \"x\" / a \"y\" / a \"z\" $$\nrule a = \"(\" a \")\" / \"b\"", Inputs: []string{"((b))z", "(b)y", "bx", "(b", ""}},
		Case{Name: "left recursion", Src: "rule main = e $$\nrule e = e \"+\" n / e \"-\" n / n\nrule n = [0-9]+", Inputs: []string{"1+2-3", "1", "1+", "+", ""}},
		Case{Name: "operators", Src: Arith, Inputs: []string{"1+2*3", "(1+2)*3", "1+", ""}},
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
	for _, g := range SkipRunGrammars {
		cases = append(cases, Case{Name: g.Name, Src: g.Src, Inputs: random(g.Alphabet)})
	}
	for _, g := range ScanSiteGrammars {
		cases = append(cases, Case{Name: g.Name, Src: g.Src, Inputs: random(g.Alphabet)})
	}
	return cases
}
