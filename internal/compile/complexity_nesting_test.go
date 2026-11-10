package compile

import (
	"os"
	"strings"
	"testing"
)

// nestingRestarts are grammars whose cost is quadratic because a unit restarts a nesting: the
// unit fails after a nested call ended at a position, and its continuation ends the rule at that
// position through calls that it evaluates alike with the unit, which the memo answers. Each level
// of the nesting then evaluates the scan R at that position again. R is pure (a scan site that
// the engine's records answer, but a generated Go parser does not) or not.
var nestingRestarts = []struct{ name, src string }{
	// The shared call is the nesting call itself (E3 through t).
	{"call itself", "rule main = e\nrule e = \"(\" (e R \"x\" / e) / \".\"\nrule R = [ ]*\n"},
	{"call itself, impure scan", "rule main = e\nrule e = \"(\" (e R \"x\" / e) / \".\"\nrule R = &\"\" [ ]*\n"},
	{"two scans per level", "rule main = e\nrule e = \"(\" (e R \"x\" / e R \"y\" / e) / \".\"\nrule R = [ ]*\n"},
	{"two scans per level, impure scan", "rule main = e\nrule e = \"(\" (e R \"x\" / e R \"y\" / e) / \".\"\nrule R = &\"\" [ ]*\n"},
	{"continuation with another path", "rule main = e\nrule e = \"(\" (e R \"x\" / (e / \"y\")) / \".\"\nrule R = [ ]*\n"},
	// The shared call is reached by looking through the nesting call (E3 through the body of t).
	{"through the call", "rule main = z\nrule z = \"(\" (y R \"x\" / z) / \".\"\nrule y = z\nrule R = [ ]*\n"},
	{"through two calls", "rule main = m R \"z\"\nrule z = \"(\" y R \"x\"\nrule y = m\nrule m = \".\" / z \"!\" / \"(\" m\nrule R = &\"\" [ ]*\n"},
	{"through two calls, pure scan", "rule main = m R \"z\"\nrule z = \"(\" y R \"x\"\nrule y = m\nrule m = \".\" / z \"!\" / \"(\" m\nrule R = [ ]*\n"},
	{"after a shared literal", "rule main = v R \"z\"\nrule e = \"(\" (v R \"x\" / v R \"y\")\nrule v = \".\" / e \";\" / \"(\" v\nrule R = &\"\" [ ]*\n"},
	{"after a shared literal, pure scan", "rule main = v R \"z\"\nrule e = \"(\" (v R \"x\" / v R \"y\")\nrule v = \".\" / e \";\" / \"(\" v\nrule R = [ ]*\n"},
	// The shared call is a call of a rule that contains the nesting call and ends where it ends:
	// E1 adds w, and E2 the rule of the restart, whose continuation calls w at its end.
	{"through a rule that nests", "rule main = e\nrule e = \"(\" (w R \"x\" / w) / \".\"\nrule w = e\nrule R = [ ]*\n"},
	{"through a rule that nests, impure scan", "rule main = e\nrule e = \"(\" (w R \"x\" / w) / \".\"\nrule w = e\nrule R = &\"\" [ ]*\n"},
	{"through a memoized rule that nests", "rule main = e / \"!\" w\nrule e = \"(\" (w R \"x\" / w) / \".\"\nrule w = e\nrule R = &\"\" [ ]*\n"},
	{"through a memoized rule that nests, pure scan", "rule main = e / \"!\" w\nrule e = \"(\" (w R \"x\" / w) / \".\"\nrule w = e\nrule R = [ ]*\n"},
	// A lookahead is restarted by what follows it.
	{"lookahead", "rule main = e\nrule e = \"(\" &(e R) e \"x\"? / \".\"\nrule R = [ ]*\n"},
}

// nestingLinear are grammars of the same shapes whose cost is linear.
var nestingLinear = []struct{ name, src string }{
	{"restart without a scan", "rule main = e\nrule e = \"(\" (e \"x\" / e) / \".\"\n"},
	// The continuation consumes input after the shared call, so the levels end apart.
	{"continuation consumes", "rule main = e\nrule e = \"(\" (e R \"x\" / e \")\") / \".\"\nrule R = [ ]*\n"},
	// The restart is not complete: the continuation consumes ")" after the shared call e, so the
	// call of e does not nest at one end, and the option (R "!")? shares R with what follows e.
	{"continuation consumes before the end", "rule main = e\nrule e = \"(\" (e R \"x\" / f Q \")\") (R \"!\")? / \".\"\nrule f = e\nrule Q = R\nrule R = [ ]*\n"},
	{"continuation consumes before the end, impure scan", "rule main = e\nrule e = \"(\" (e R \"x\" / f Q \")\") (R \"!\")? / \".\"\nrule f = e\nrule Q = R\nrule R = &\"\" [ ]*\n"},
	{"option before the end", "rule main = e\nrule e = \"(\" e ws \")\" / \"a\" (ws \"!\")?\nrule ws = [ ]*\n"},
	{"calculator, spaces first", "rule main = expr $$\nrule expr = term (ws \"+\" term)*\nrule term = factor (ws \"*\" factor)*\nrule factor = \"(\" expr ws \")\" / ws [0-9]+\nrule ws = [ ]*\n"},
	{"repeats", "rule main = s \"x\" / s \"w\" / s (R \"b\")? \"y\"\nrule s = \".\" (R \"a\")?\nrule R = [ ]*\n"},
}

// nestingHooks are the settings of the analyses that tell committing units apart, each of which
// must find the restarts.
var nestingHooks = []struct {
	name  string
	hooks ExclusionHooks
	cut   bool
}{
	{"default", ExclusionHooks{Decided: true, Trivia: true}, false},
	{"without decided units", ExclusionHooks{Trivia: true}, false},
	{"without trivia", ExclusionHooks{Decided: true}, false},
	{"with the memo of trivia", ExclusionHooks{Decided: true, Trivia: true, Memo: true}, false},
	{"with cut prefixes", ExclusionHooks{Decided: true, Trivia: true}, true},
}

// TestNestingRestarts checks that the restart clause of nesting at one end makes the classifier
// reject the grammars whose units restart a nesting, under each setting of the analyses, and
// keeps the linear grammars of the same shapes linear.
func TestNestingRestarts(t *testing.T) {
	calc, err := os.ReadFile("../../examples/calculator/calc_lr.zp")
	if err != nil {
		t.Fatal(err)
	}
	linear := append(nestingLinear[:len(nestingLinear):len(nestingLinear)],
		struct{ name, src string }{"calc_lr", string(calc)},
		struct{ name, src string }{"calc_lr, impure spaces", strings.Replace(string(calc), "rule ws = [", "rule ws = &\"\" [", 1)})
	for _, h := range nestingHooks {
		t.Run(h.name, func(t *testing.T) {
			defer h.hooks.Set()()
			cutPrefixesOn = h.cut
			for _, tc := range nestingRestarts {
				// With the memo of trivia rules, the memo answers a pure scan R again.
				if h.hooks.Memo && !strings.Contains(tc.src, "&\"\"") {
					continue
				}
				if c := classifySource(t, "@start\n"+tc.src); c.Class == ClassLinear {
					t.Errorf("%s: class linear, want a restart of the nesting found", tc.name)
				}
			}
			for _, tc := range linear {
				src := tc.src
				if !strings.Contains(src, "@start") {
					src = "@start\n" + src
				}
				if c := classifySource(t, src); c.Class != ClassLinear {
					t.Errorf("%s: class %v, want linear: %v %v", tc.name, c.Class, c.Reasons, c.Causes)
				}
			}
		})
	}
}

// TestNestingRestartsUsed checks the check of the restarts that the checks of the units relied
// on: without the restarts that nestsAt reads, the classifier finds them only in the checks, and
// turns the calls that they make nest into reasons.
func TestNestingRestartsUsed(t *testing.T) {
	restartsAhead = false
	defer func() { restartsAhead = true }()
	for _, tc := range nestingRestarts[:2] {
		c := classifySource(t, "@start\n"+tc.src)
		found := false
		for _, r := range c.Reasons {
			found = found || strings.Contains(r.Message, "found only after it relied on the call")
		}
		if !found {
			t.Errorf("%s: no reason for a restart found in the checks: %v", tc.name, c.Reasons)
		}
	}
}
