package gentest

import (
	"math/rand"
	"strings"
)

// FlagParityCases are grammars on which the optimizations that every route shares act (guards that
// skip alternatives, scans, runs of classes, text predicates, literals around the length compared in
// place and recovery), with inputs at their edges and seeded random ones. The tests that turn an
// optimization off on the Node, recognizer and TypeScript routes compare the parsers with the engine
// on them.
func FlagParityCases() []Case {
	alphabet := append(strings.Split("abcdefghijklmnopqrstuvwxyz0123456789 _é日,!", ""), "\xff")
	cases := []Case{
		{Name: "guards", Src: "rule main = (word / num / ident / -\" \")* $$\nrule word = \"if\" / \"else\" / \"while\" / \"fn\"\nrule num = [0-9]+\nrule ident = [a-z] [a-z0-9_]*\n", Inputs: []string{"if else while fn", "if1 x9", "123 abc", "", "else9", "é"}},
		{Name: "scans", Src: "rule main = word (-[ ]* word)* -.* $$\nrule word = [a-z]+\n", Inputs: []string{"a b c", "abc   def  ", "a  b ~~~ tail", "", "a\xff"}},
		{Name: "class runs", Src: "rule main = (-([a-c] [d-f] [g-h] [0-9]) / -([x-z] [é] [日] \"!\") / -\" \")* $$\n", Inputs: []string{"adg1 xé日!", "adg", "", "adg1adg1"}},
		{Name: "text comparison", Src: "rule main = (x:[a-z]+ #atomic \",\" y:[a-z]+ #atomic {text($x) == text($y)} -\" \"?)* $$\n", Inputs: []string{"a,a", "ab,ab a,a", "a,b", ",", "é,é"}},
		{Name: "text dispatch", Src: "rule main = (x:[a-z]+ #atomic {text($x) == \"if\" || text($x) == \"fn\" || text($x) == \"do\"} -\" \"?)* $$\n", Inputs: []string{"if fn do", "if x", "", "do do"}},
		{Name: "literals", Src: "rule main = (l:lit / -\" \")* $$\nrule lit = \"a\" / \"ab\" / \"abc\" / \"abcd\" / \"abcde\" / \"日本\" / \"日本語日本語\"\n", Inputs: []string{"a ab abc abcd abcde", "abcdefg", "日本 日本語日本語", ""}},
		{Name: "recovery", Src: "rule main = (stmt #recover(skip=(![;] .)* \";\"))* $$\nrule stmt = [a-z]+ \";\" / \"(\" [0-9]+ \")\" \";\"\n", Inputs: []string{"ab;cd;", "a!;b;", "(1);x;", "(;", ""}},
	}
	rng := rand.New(rand.NewSource(5))
	for i := range cases {
		for range 60 {
			var b strings.Builder
			for range rng.Intn(12) {
				b.WriteString(alphabet[rng.Intn(len(alphabet))])
			}
			cases[i].Inputs = append(cases[i].Inputs, b.String())
		}
	}
	return cases
}
