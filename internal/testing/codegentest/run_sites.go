package codegentest

import (
	"math/rand"
	"strings"
)

// RunSiteGrammars are grammars with run sites (see "Run Sites" in spec/runtime/parsing.md): repetitions outside skip
// expressions whose parse builds no value (the start rule has an action, so the elements' values
// are not needed), each with the characters of its random inputs. They exercise the parts of what a
// record holds, as the skip-run grammars do, and nested walks.
var RunSiteGrammars = []struct{ Name, Src, Alphabet string }{
	{"repetition restarted by an alternative", `rule main = (a "y" / "q" a / [x])* $$ => 1
rule a = x+ => 1
rule x = "x"`, "xyqz"},
	{"recursion over a scan", `rule main = t a? $$ => 1
rule t = a t "!" / a "?" t / [x!?] t / ""
rule a = x+ => 1
rule x = "x"`, "x!z"},
	{"nested walks", `rule main = (g "y" / g "z" / [abc,])* $$ => 1
rule g = (w ","?)+ => 1
rule w = [a-b]+ "c"? => 1`, "abc,yzq"},
	{"minimum and empty elements", `rule main = (a "!" / [xy])* $$ => 1
rule a = (x? y?)#repeat(2,) => 1
rule x = "x"
rule y = "y"`, "xy!z"},
	{"lookahead in the element", `rule main = (a "!" / [xy])* $$ => 1
rule a = (!"xx" x)+ => 1
rule x = "x" / "y"`, "xy!z"},
	{"expectations", `rule main = (a "!" / "q" a "?" / [xyz])* $$ => 1
rule a = (x / "y" z)+ => 1
rule x = "x"
rule z = "z"`, "xyz!?qw"},
	{"silent walks", `rule main = (!(a "!") "x" / a "!" / [,y])* $$ => 1
rule a = (x ","?)+ => 1
rule x = "x" "y"?`, "xy,!z"},
	{"line starts", `rule main = (l "!" / [ab\n])* $$ => 1
rule l = (^ "a" / x)+ => 1
rule x = "b" / "\n"`, "ab\n!z"},
	{"operators in a called rule", `rule main = (a "!" / [xy+*!])* $$ => 1
rule a = (e ";"?)+ => 1
rule e = #operators {
    operand "x" / "y"
    level { infix left "+" }
    level { infix right "*" }
}`, "xy+*;!z"},
}

// RunValueGrammars are grammars whose run sites build values, capture and read variables in their
// elements, and recover errors in them or in the rules they call; they have no action at the top,
// so the values of the start rule are the parse trees.
var RunValueGrammars = []struct{ Name, Src, Alphabet string }{
	{"values of restarted lists", `rule main = (a "y" / "q" a / [x])* $$
rule a = x+
rule x = "x"`, "xyqz"},
	{"nested lists", `rule main = (g "y" / g "z" / [abc,])* $$
rule g = (w ","?)+
rule w = [a-b]+ "c"?`, "abc,yzq"},
	{"minimum and empty elements", `rule main = (a "!" / [xy])* $$
rule a = (x? y?)#repeat(2,)
rule x = "x"
rule y = "y"`, "xy!z"},
	{"captures in the element", `rule main = (a "!" / "q" a "?" / [xyz])* $$
rule a = (c:x d:y? => $c)+
rule x = "x"
rule y = "y"`, "xyz!?qw"},
	{"actions in the element", `rule main = (a "!" / [xy])* $$
rule a = (v:x => $v)+
rule x = "x" "y"?`, "xy!z"},
	{"variables in predicates", `var n int
rule main = (a "!" / "q" a / [xz])* $$
rule a = ({ n < 3 } x { n := n + 1 })+
rule x = "x"`, "xz!q"},
	{"two environments at one position", `var n int
rule main = ({ n := 1 } r "!" / { n := 5 } r "?" / [xyzwq])* $$
rule r = a "y" / a "w" / a "z" / a "q"
rule a = "x" ("x" { n < 3 })*`, "xyzwq!?"},
	{"walks that merge with others in the middle", `rule main = (r "!" / r "?" / r "w" / [abz])* $$
rule r = ("ab" / "a" / "b")*`, "abz!?w"},
	{"bindings that a record restores", `var n int
rule main = (r "!" / r "?" / r "w" / [xyzwq])* $$
rule r = ("x" { n := n + 1 })* { n == 3 }`, "xyzwq!?"},
	{"errors that a record replays", `rule main = (a "?" / "x" a "?" / "x" "x" a "!" / [xz!?])* $$
rule a = e+
rule e = ("x" "z") #recover(skip=[x])`, "xz!?"},
	{"recovery in the element", `rule main = (a "!" / "q" a "?" / [xyz])* $$
rule a = (x #recover(skip=[y]+))+
rule x = "x" "z"`, "xzy!?q"},
	{"recovery in a called rule", `rule main = (l "!" / [abz])* $$
rule l = (e)+
rule e = ("a" "b" #recover(skip=[z]+))`, "abz!"},
	{"lookahead and silent walks", `rule main = (!(a "!") "x" / a "!" / [,y])* $$
rule a = (x ","?)+
rule x = "x" "y"?`, "xy,!z"},
	{"a site free of the environment under rebindings", `var n int
rule main = ({ n := 1 } r "!" / { n := 2 } r "?" / r "#" / [xyzw!?#])* $$
rule r = ("x" / "yz" / b)*
rule b = "w" { n := 3 }`, "xyzw!?#"},
	{"a called rule that reads a variable", `var n int
rule main = ({ n := 1 } r "!" / { n := 5 } r "?" / [xyz!?])* $$
rule r = e*
rule e = "x" { n < 3 } / "y"`, "xyz!?"},
	{"a binding that the element does not read", `var k int
rule main = ({ k := 1 } r "!" / { k := 2 } r "?" / [xyzwv!?])* $$
rule r = ("x" { k := 7 } / "y")* ({ k == 7 } "z" / { k == 2 } "w" / { k == 1 } "v")`, "xyzwv!?"},
	{"a binding and a called rule that reads", `var n int
var m int
rule main = ({ n := 1 } r "!" / { n := 5 } r "?" / [xyz!?])* $$
rule r = (e { m := 1 })* { m == 1 }
rule e = "x" { n < 3 } / "y"`, "xyz!?"},
	{"bindings in a called rule", `var x int = 0
rule main = (r "!" / [ab!])* $$
rule r = (c { x == 0 })*
rule c = { x := 1 } "a" { x == 1 } / { x := 2 } "b" { x == 2 }`, "ab!"},
	{"walks of a value-free twin", `rule main = (y / "a")* $$ / w $$
rule y = a "?" => 1
rule w = "a" a "!"
rule a = ("a" / "b")*`, "ab!?"},
	{"lookahead walks before lists", `rule main = (!(a "?") !(a ",") &(x:a "!" { len($x) > 3 }) w / a "," / [ab!?,])* $$
rule w = a "!"
rule a = ("a" / "b")*`, "ab!?,"},
	{"operators in a called rule", `rule main = (a "!" / [xy+*!])* $$
rule a = (e ";"?)+
rule e = #operators {
    operand "x" / "y"
    level { infix left "+" => $lhs }
    level { infix right "*" => $rhs }
}`, "xy+*;!z"},
}

// RunValueInputs are inputs, besides random ones, that reach what a grammar of RunValueGrammars is
// about: here a walk that a record of another environment would answer wrongly.
var RunValueInputs = map[string][]string{
	"two environments at one position":         {"xxxq!", "xxxq?", "xxxq?xxxq!", "xxxxxq?!"},
	"bindings that a record restores":          {"xxxw", "xxw", "xxxxw", "xxxw!xxw"},
	"errors that a record replays":             {"xxxxxx!", "xxzxxx!", "xxxxzxx!xxxxx!", "xxxxxxx?"},
	"a called rule that reads a variable":      {"xxxx?", "xxxx!", "xyxx?", "xxxx?xxx!"},
	"a binding that the element does not read": {"yyxyv!", "yyyxyw?", "yyyxyyv!yyxyw?", "yyyv!", "yyxyz?"},
	"a binding and a called rule that reads":   {"xxxx?", "xxxx!", "yxxx?", "xxxx?xxx!", "yyyy!"},
	"bindings in a called rule":                {"ab!", "abab!", "aab!ba!b", "ababab"},
	"walks of a value-free twin":               {"aaaaaa!", "ababab!", "aaa?aaaa!"},
	"lookahead walks before lists":             {"aaaaaa!", "ababab,", "aba?abab!", "ab,ab!"},
}

// RunSiteInputs returns the inputs a grammar of run sites is parsed with when its sites must turn
// hot at the threshold of the parsers: those of SkipRunInputs, each repeated until it is longer
// than four times the threshold, so that walks span it.
func RunSiteInputs(alphabet string, n int, rng *rand.Rand) []string {
	var inputs []string
	for _, in := range SkipRunInputs(alphabet, n, rng) {
		if in == "" {
			continue
		}
		inputs = append(inputs, strings.Repeat(in, 1+256/len(in)))
	}
	return inputs
}

// RunSiteCases returns the cases of RunSiteGrammars and RunValueGrammars, each with the inputs of
// RunValueInputs and n random inputs, which are long (RunSiteInputs) when long is set.
func RunSiteCases(n int, long bool) []Case {
	rng := rand.New(rand.NewSource(103))
	var cases []Case
	for _, set := range []struct {
		prefix   string
		grammars []struct{ Name, Src, Alphabet string }
	}{{"run sites/", RunSiteGrammars}, {"run values/", RunValueGrammars}} {
		for _, g := range set.grammars {
			inputs := SkipRunInputs(g.Alphabet, n, rng)
			if long {
				inputs = RunSiteInputs(g.Alphabet, n, rng)
			}
			cases = append(cases, Case{Name: set.prefix + g.Name, Src: g.Src, Inputs: append(inputs, RunValueInputs[g.Name]...)})
		}
	}
	return cases
}
