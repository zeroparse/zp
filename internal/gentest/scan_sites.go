package gentest

import (
	"math/rand"
	"strings"
)

// ScanSiteGrammars are grammars with scan sites (see "Scan Sites" in spec/runtime/parsing.md):
// repetitions of one character in rules that call no rule, which a failed alternative restarts at
// the same position, in rules called from one place or more, and under labels, captures and
// actions. Their parse trees hold the values of the scan sites, which later walks share.
var ScanSiteGrammars = []struct{ Name, Src, Alphabet string }{
	{"a scan restarted by alternatives", `rule main = (r "!" / r "?" / r "w" / [abz])* $$
rule r = [ab]*`, "abz!?w"},
	{"a repetition of groups", `rule main = (r "!" / r "?" / [ab/z\n])* $$
rule r = [ ab]* ("//" [^\n]* [ ab]*)*`, " ab/z!?\n"},
	{"calls at the same position", `rule main = e " " ws $$
rule e = "(" e? (ws ")")?
rule ws = [ ]*`, "( )"},
	{"lookahead and silent calls", `rule main = (!(ws "!") "x" / ws "!" / ws "?" / [,y ])* $$
rule ws = [ ]*`, "x!? y,"},
	{"expectations", `rule main = (ws "a" ws "b" / ws "a" ws "c" / ws "d")* $$
rule ws = [ \t]*`, "abcd \t"},
	{"recovery in the caller", `rule main = s* $$
rule s = (ws "a" ws ";" / ws "a" ws "!") #recover(skip=[^;]* ";")
rule ws = [ ]*`, "a ;!"},
	{"memoized callers of a scan", `rule main = (a ";" / a ":" / b ";" / b ":" / c ";" / c ":" / d ";" / d ":" / [;: ])* $$
rule a = ws "x"
rule b = ws "y"
rule c = ws "z"
rule d = ws "w"
rule ws = [ ]*`, "xyzw;:  "},
	{"a called scan with a minimum", `rule main = (r "!" / r "?" / [ab ])* $$
rule r = [ab]+`, "ab!? "},
	{"a minimum above the hot span in text", `rule main = (w / [ab ])* $$
rule w = (r "!" / r "?") #atomic
rule r = [ab] #repeat(5,)`, "ab!? "},
	{"a minimum above the hot span", `rule main = (r "!" / r "?" / [ab ])* $$
rule r = [ab] #repeat(5,)`, "ab!? "},
	{"several sites with labels", `rule main = (r "!" / r "?" / [abz ])* $$
rule r = x:[ab]* [ ]* y:[z]*`, "abz !?"},
	{"captures and actions", `rule main = (r "!" / r "?" / s "w" / s "z" / [ab ])* $$
rule r = xs:[ab]* => text($xs) + "."
rule s = c:[ab]+ d:[ ]* => text($c) + "," + text($d)`, "ab !?wz"},
	{"a rule called once in a recursion", `rule main = e " " ws $$
rule e = "(" e? (ws ")")? / ws "!"
rule ws = xs:[ ]* => text($xs)`, "( )!"},
	{"lookaheads around an action", `rule main = (&(r "!") r "!" / !(r "?") r "w" / r "?" / [ab ])* $$
rule r = c:[ab]* => text($c)`, "ab !?w"},
}

// ScanSiteInputs returns the inputs a grammar of ScanSiteGrammars is parsed with: n random strings
// over alphabet (SkipRunInputs) and strings with long runs of each character that the walks restart
// at.
func ScanSiteInputs(alphabet string, n int, rng *rand.Rand) []string {
	in := SkipRunInputs(alphabet, n, rng)
	for _, c := range alphabet {
		for _, n := range []int{3, 40, 150} {
			run := strings.Repeat(string(c), n)
			in = append(in, run+"x;", run+"z:")
			in = append(in, run, run+"!", run+"?", run+"x", "("+run, "("+run+"("+run+")", run+" "+run+"!")
		}
	}
	return in
}

// ScanSiteCases returns the cases of ScanSiteGrammars, each with n random inputs and the inputs
// with long runs.
func ScanSiteCases(n int) []Case {
	rng := rand.New(rand.NewSource(134))
	var cases []Case
	for _, g := range ScanSiteGrammars {
		cases = append(cases, Case{Name: "scan sites/" + g.Name, Src: g.Src, Inputs: ScanSiteInputs(g.Alphabet, n, rng)})
	}
	return cases
}
