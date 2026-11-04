package gentest

import (
	"fmt"
	"strings"
)

// MemoLookaheadCase is a grammar whose rule a is called at one position alternately outside and
// inside lookaheads, through its value-free twin (`-a`, `&(a ...)`), and then once through the rule
// itself (`a "z"`). With #recover the result of a has recovered errors.
type MemoLookaheadCase struct {
	Name, Src, Input string
	// Evals and Recognized are the body evaluations of a at the position in a parse and in a
	// recognition, where every call of a is a call of its twin. Per variant (the rule and its twin)
	// they are at most the deferred first call, one memoized evaluation inside a lookahead and one
	// outside, whose entry answers the calls in both states.
	Evals, Recognized int
}

// MemoLookaheadCases returns the MemoLookaheadCase grammars.
func MemoLookaheadCases() []MemoLookaheadCase {
	const recovered, clean = `("x" b) #recover(skip=[ab]+)`, `"a" b`
	cases := []struct {
		name, form, body  string
		evals, recognized int
	}{
		// The twin: deferred (outside), inside, outside, which answers the later calls in both
		// states; the rule: deferred.
		{"recovered alternating", "OIOIOIOI", recovered, 4, 3},
		{"clean alternating", "OIOIOIOI", clean, 4, 3},
		// The twin: deferred (inside), outside, which answers the calls inside.
		{"recovered inside first", "IOIOIOIO", recovered, 3, 2},
		{"clean inside first", "IOIOIOIO", clean, 3, 2},
		{"recovered outside only", "OOOO", recovered, 3, 2},
		// In a recognition, the last call is the first outside a lookahead.
		{"recovered inside only", "IIII", recovered, 3, 3},
	}
	out := make([]MemoLookaheadCase, len(cases))
	for i, c := range cases {
		out[i] = MemoLookaheadCase{Name: c.name, Src: memoLookaheadGrammar(c.form, c.body), Input: "abz", Evals: c.evals, Recognized: c.recognized}
	}
	return out
}

// memoLookaheadGrammar returns a grammar whose start rule s tries the alternatives of form, a twin
// call outside a lookahead (O) or inside one (I) followed by a literal that does not match, and then
// `a "z"`; a has the body body.
func memoLookaheadGrammar(form, body string) string {
	var alts []string
	for i, c := range form {
		lit := fmt.Sprintf("%q", string(rune('k'+i)))
		if c == 'I' {
			alts = append(alts, "&(a "+lit+")")
		} else {
			alts = append(alts, "-a "+lit)
		}
	}
	alts = append(alts, `a "z"`)
	return "@start\nrule s = " + strings.Join(alts, " / ") + "\nrule a = " + body + "\nrule b = \"b\"\n"
}
