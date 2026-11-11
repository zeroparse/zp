package compile

import (
	"os"
	"strings"
	"testing"
)

// nestingRestarts are grammars whose cost is quadratic because a unit restarts a nesting: the
// unit fails after a nested call ended at a position, and its continuation ends the rule at that
// position through calls that it evaluates alike with the unit, which the memo answers. Each level
// of the nesting then evaluates the scan R at that position again. R is pure or not; either way
// its repetition is a scan site, whose records answer the levels, so the tests that check the
// restarts classify them without scan sites (withoutScanSites), as for a scan that is no site.
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
	// The unit looks through the nesting call z, shares the literal at its start with the
	// continuation and then the call of m inside it: the calls looked through carry over from one
	// shared call to the next.
	{"through the call, after a literal", "rule main = m\nrule z = \"(\" m\nrule m = \".\" / z R \"!\" / \"(\" m\nrule R = [ ]*\n"},
	{"through the call, after a literal, impure scan", "rule main = m\nrule z = \"(\" m\nrule m = \".\" / z R \"!\" / \"(\" m\nrule R = &\"\" [ ]*\n"},
	// The shared call is a call of a rule that contains the nesting call and ends where it ends:
	// E1 adds w, and E2 the rule of the restart, whose continuation calls w at its end.
	{"through a rule that nests", "rule main = e\nrule e = \"(\" (w R \"x\" / w) / \".\"\nrule w = e\nrule R = [ ]*\n"},
	{"through a rule that nests, impure scan", "rule main = e\nrule e = \"(\" (w R \"x\" / w) / \".\"\nrule w = e\nrule R = &\"\" [ ]*\n"},
	{"through a memoized rule that nests", "rule main = e / \"!\" w\nrule e = \"(\" (w R \"x\" / w) / \".\"\nrule w = e\nrule R = &\"\" [ ]*\n"},
	{"through a memoized rule that nests, pure scan", "rule main = e / \"!\" w\nrule e = \"(\" (w R \"x\" / w) / \".\"\nrule w = e\nrule R = [ ]*\n"},
	// A lookahead is restarted by what follows it.
	{"lookahead", "rule main = e\nrule e = \"(\" &(e R) e \"x\"? / \".\"\nrule R = [ ]*\n"},
	// The option ends the body of f, and its continuation leaves f and then e through the call of
	// e that nests at one end, and then main: the scan in main follows that nesting, although the
	// call of e in main does not nest.
	{"after the ends of two bodies", "rule main = e R\nrule e = \"(\" f e / \".\"\nrule f = (e R \"x\")?\nrule R = &\"\" [ ]*\n"},
	{"after the ends of two bodies, nested option", "rule main = m R\nrule m = \"[\" e m / \".\"\nrule e = \"(\" (m R \"x\")? / \".\"\nrule R = &\"\" [ ]*\n"},
	// An operand fails after its call of e ended at a position, and a later operand ends e at
	// that position through the memo answer of the same call, so every level ends there and the
	// first operand of the level around it, or what follows e, scans R there again.
	{"later operand", "rule main = e\nrule e = #operators {\n operand \"(\" e R \"x\"\n operand \"(\" e\n operand \".\"\n}\nrule R = &\"\" [ ]*\n"},
	{"later operand, pure scan", "rule main = e R\nrule e = #operators {\n operand \"(\" e R \"x\"\n operand \"(\" e\n operand \".\"\n}\nrule R = [ ]*\n"},
	{"later operand, postfix part", "rule main = e\nrule e = #operators {\n operand \"(\" e R \"x\"\n operand \"(\" e\n operand \".\"\n level a { postfix \"!\" }\n}\nrule R = &\"\" [ ]*\n"},
	{"later operand, scan after the rule", "rule main = e R\nrule e = #operators {\n operand \"(\" e R \"x\"\n operand \"(\" e\n operand \".\"\n level a { postfix \"!\" }\n}\nrule R = &\"\" [ ]*\n"},
	{"later operand through a rule", "rule main = e R\nrule e = #operators {\n operand \"(\" w R \"x\"\n operand \"(\" w\n operand \".\"\n level a { postfix \"!\" }\n}\nrule w = e\nrule R = &\"\" [ ]*\n"},
	{"later operand through another rule", "rule main = x R\nrule x = \"{\" x / e\nrule e = #operators {\n operand \"(\" x R \"x\"\n operand \"(\" x\n operand \".\"\n level a { postfix \"!\" }\n}\nrule R = &\"\" [ ]*\n"},
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
// keeps the linear grammars of the same shapes linear. It classifies them without scan sites.
func TestNestingRestarts(t *testing.T) {
	withoutScanSites(t)
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

// TestNestingRestartsDecided checks the restarts that only the decided comparison finds: the unit
// begins with a character class and the continuation with a literal, so they share no call at
// their start and nestsAt reads no restart from them, while the comparison passes the class and
// the literal alike and then the call of e. The call of e in the unit then nests at one end, which
// checkRestarts reports. The grammars are quadratic: R scans the same spaces once for each level.
func TestNestingRestartsDecided(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"class against a literal", "rule main = e\nrule e = \"(\" ([.] e R \"x\" / \".\" e) / \".\"\nrule R = [ ]*\n"},
		{"class against a literal, impure scan", "rule main = e\nrule e = \"(\" ([.] e R \"x\" / \".\" e) / \".\"\nrule R = &\"\" [ ]*\n"},
	} {
		c := classifySource(t, "@start\n"+tc.src)
		found := false
		for _, r := range c.Reasons {
			found = found || strings.Contains(r.Message, "call of e at 3:19 in rule e can nest without a bound") &&
				strings.Contains(r.Message, "found only after it relied on the call")
		}
		if !found {
			t.Errorf("%s: no reason for the restart that the decided comparison found: %v", tc.name, c.Reasons)
		}
	}
}

// TestNestingRestartsDecidedRejectLinearExit pins a grammar that the recorded restarts reject
// although it is linear: the decided comparison passes R and the call of e in f alike, and the
// continuation then leaves f and ends e through the call of e after f, so the call of e in f is
// reported to nest at one end, while the steps of every backend grow linearly. The rejection is
// a loss of precision, not of soundness; a change that makes the grammar linear must show why
// the nesting cannot restart.
func TestNestingRestartsDecidedRejectLinearExit(t *testing.T) {
	src := "@start\nrule main = e R\nrule e = \"<\" f e / \".\"\nrule f = (R e)*\nrule R = sp*\nrule sp = [ ]\n"
	c := classifySource(t, src)
	if c.Class != ClassUnknown {
		t.Fatalf("class %v, want unknown: %v", c.Class, c.Reasons)
	}
	found := false
	for _, r := range c.Reasons {
		found = found || strings.Contains(r.Message, "call of e at 4:13 in rule f can nest without a bound") &&
			strings.Contains(r.Message, "found only after it relied on the call")
	}
	if !found {
		t.Errorf("no reason for the restart that the decided comparison found: %v", c.Reasons)
	}
}

// nestingGrowths are grammars whose cost is quadratic because the evaluations of a left-recursive
// rule nest and end at one position: each of them grows there, and the next step of its growth
// scans R after its seed at that position again.
var nestingGrowths = []struct{ name, src string }{
	{"growth after a nesting", "rule main = e\nrule e = e R \"x\" / \"(\" e / \".\"\nrule R = &\"\" [ ]*\n"},
	{"growth after a nesting, pure scan", "rule main = e\nrule e = e R \"x\" / \"(\" e / \".\"\nrule R = [ ]*\n"},
	{"two growing alternatives", "rule main = e\nrule e = e R \"x\" / e \"+\" / \"(\" e / \".\"\nrule R = &\"\" [ ]*\n"},
	{"nesting through another rule", "rule main = e\nrule e = e R \"x\" / \"(\" f / \".\"\nrule f = e\nrule R = &\"\" [ ]*\n"},
	{"growing alternative in a member", "rule main = e\nrule e = a / \"(\" e / \".\"\nrule a = e R \"x\"\nrule R = &\"\" [ ]*\n"},
}

// TestNestingGrowths checks that the classifier counts the steps of the growths of nested
// evaluations that end at one position, and keeps the grammars whose nesting consumes input after
// the nested call, or that scan nothing after a seed, linear. It classifies them without scan
// sites.
func TestNestingGrowths(t *testing.T) {
	withoutScanSites(t)
	for _, h := range nestingHooks {
		t.Run(h.name, func(t *testing.T) {
			defer h.hooks.Set()()
			cutPrefixesOn = h.cut
			for _, tc := range nestingGrowths {
				if h.hooks.Memo && !strings.Contains(tc.src, "&\"\"") {
					continue
				}
				if c := classifySource(t, "@start\n"+tc.src); c.Class == ClassLinear {
					t.Errorf("%s: class linear, want the growths after the nesting found", tc.name)
				}
			}
			for _, tc := range []struct{ name, src string }{
				{"nesting consumes after the call", "rule main = e\nrule e = e R \"x\" / e \"+\" / \"(\" e \")\" / \".\"\nrule R = &\"\" [ ]*\n"},
				{"no scan after the seed", "rule main = e\nrule e = e \"+\" / \"(\" e / \".\"\n"},
				// The calls of n nest and end at one position, but those of x, another cycle, do
				// not: the unit "y" R "!" of t is left at the end of x, and the next step of the
				// growth of x scans R again only once.
				{"nesting in another cycle", "rule main = n \";\" x\nrule n = n \"-\" / \"(\" n / \".\"\nrule x = x R \"+\" t / t\nrule t = \"y\" R \"!\" / \"y\"\nrule R = &\"\" [ ]*\n"},
			} {
				if c := classifySource(t, "@start\n"+tc.src); c.Class != ClassLinear {
					t.Errorf("%s: class %v, want linear: %v %v", tc.name, c.Class, c.Reasons, c.Causes)
				}
			}
		})
	}
}

// withoutScanSites makes the test compile and classify without scan sites, as if the repetitions
// of its grammars were scanning sources that no record answers.
func withoutScanSites(t *testing.T) {
	NoScanSites = true
	t.Cleanup(func() { NoScanSites = false })
}

// scanOnlyNestingRejected are the grammars of nestingRestarts and nestingGrowths that stay rejected
// with scan sites, for a reason other than the evaluations of R again at one position: a
// lookahead that what follows restarts, the operator parts of an Operators Rule that nests, and
// the growths of two alternatives.
var scanOnlyNestingRejected = map[string]bool{
	"lookahead":                          true,
	"later operand, postfix part":        true,
	"later operand, scan after the rule": true,
	"later operand through a rule":       true,
	"later operand through another rule": true,
	"two growing alternatives":           true,
}

// TestScanOnlyNestings checks that the scan-only rule R, whose only scanning source is a scan site,
// is shared where a nesting restarts at one end: the records of its site answer each level after
// the first walks from a position, so the classifier accepts the grammars of nestingRestarts and
// nestingGrowths but those that it rejects for another reason. Without scan sites it rejects them
// all (TestNestingRestarts, TestNestingGrowths). The controls of the families "nesting restarts"
// and "rescanned calls" measure the same shapes linear.
func TestScanOnlyNestings(t *testing.T) {
	defer restoreExclusionDefaults()
	for _, tc := range append(nestingRestarts[:len(nestingRestarts):len(nestingRestarts)], nestingGrowths...) {
		c := classifySource(t, "@start\n"+tc.src)
		if linear := c.Class == ClassLinear; linear == scanOnlyNestingRejected[tc.name] {
			t.Errorf("%s: class %v with scan sites: %v", tc.name, c.Class, c.Reasons)
		}
	}
}
