package complexity

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/internal/syntax"
)

// regionCredit classifies src and reports whether the body of the trivia rule ws has the region
// bound.
func regionCredit(t *testing.T, src string) bool {
	t.Helper()
	g, err := syntax.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(starttest.MarkAll(g), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	k.collect()
	k.computeBoundedScans()
	ws := k.byName["ws"]
	if ws == nil {
		t.Fatal("no rule ws")
	}
	rep, ok := triviaUnwrap(ws.Expr).(*grammar.Repeat)
	return ok && k.regionScan[rep]
}

// regionSource is a grammar of sums whose blanks hold comments: the lookahead !(ws ".") fails
// after ws has scanned a comment, and what follows starts again where ws started, at each level of
// a nest of parentheses. alts are added to the alternatives of p, and rules to the rules.
func regionSource(alts, rules string) string {
	return "rule main = (e \";\" ws)* $$\n" +
		"rule e = p (ws \"+\" ws p)*\n" +
		"rule p = k !(ws \".\") / k ws \".\" ws id / \"(\" ws e ws \")\" / id" + alts + "\n" +
		"rule k = \"true\"\n" +
		"rule id = [a-z]+\n" +
		"rule ws = [ ]* (c [ ]*)*\n" +
		"rule c = \"/*\" [^*]* \"*/\"\n" + rules
}

// TestRegionScans checks each condition of the region bound with a grammar that breaks it and a
// twin that differs only where the condition looks, which keeps the bound.
func TestRegionScans(t *testing.T) {
	for _, c := range []struct {
		name, credited, broken string
	}{
		// (G2): the first character of a comment as a token, with and without a check after it that
		// fails where a comment goes on.
		{"a token before a check",
			regionSource(" / \"/\" !\"*\" id", ""),
			regionSource(" / \"/\" id", "")},
		// (G2): the token where a call of ws has ended on every path, and on one path only.
		{"a token where ws ended",
			regionSource(" / \"[\" (\"x\" ws / ws) \"/\" id \"]\"", ""),
			regionSource(" / \"[\" (\"x\" / ws) \"/\" id \"]\"", "")},
		// (G2): a check that does not fail where a comment goes on.
		{"a check that a comment passes",
			regionSource(" / \"/\" !\"*\" id", ""),
			regionSource(" / \"/\" !\"+\" id", "")},
		// (G2): a check after a repetition is not evaluated where an iteration ends, since the next
		// iteration starts there.
		{"a check after a repetition",
			regionSource(" / \"[\" (id \"/\" !\"*\")* id \"]\"", ""),
			regionSource(" / \"[\" (id \"/\")* !\"*\" id \"]\"", "")},
		// (G2): where ws ended on the first iteration of a repetition only.
		{"a token in a repetition after ws",
			regionSource(" / \"[\" (ws \"/\" id)* \"]\"", ""),
			regionSource(" / \"[\" ws (\"/\" id)* \"]\"", "")},
		// (G2): an option that can leave out the call of ws.
		{"a token after an option",
			regionSource(" / \"[\" (\"x\" ws) \"/\" id \"]\"", ""),
			regionSource(" / \"[\" (\"x\" ws)? \"/\" id \"]\"", "")},
		// (G2): a rule starts with what all its calls hold.
		{"a token at the start of a rule",
			regionSource(" / \"[\" ws slash \"]\" / \"{\" ws slash \"}\"", "rule slash = \"/\" id\n"),
			regionSource(" / \"[\" ws slash \"]\" / \"{\" slash \"}\"", "rule slash = \"/\" id\n")},
		// (G2): a call ends with what the body of its rule ends with.
		{"a token after a rule that ends with ws",
			regionSource(" / \"[\" w2 \"/\" id \"]\"", "rule w2 = \"x\" ws\n"),
			regionSource(" / \"[\" w2 \"/\" id \"]\"", "rule w2 = ws \"x\"\n")},
		// (G2): a token that no comment begins with, after its first character.
		{"a token that no comment goes on with",
			regionSource(" / \"/a\" id", ""),
			regionSource(" / \"/\" [a-z] id", "")},
		// (G2): a blank as a token, which a blank of ws can follow.
		{"a blank as a token",
			regionSource(" / \"[\" \" \" !(\" \" / \"/\") id \"]\"", ""),
			regionSource(" / \"[\" \" \" id \"]\"", "")},
		// (G): a string, whose body holds blanks and slashes, is a region of its own, which a token
		// that consumes its quote breaks.
		{"a string as a region",
			regionSource(" / str", "rule str = \"\\\"\" [^\"]* \"\\\"\"\n"),
			regionSource(" / str / \"<\" \"\\\"\" id", "rule str = \"\\\"\" [^\"]* \"\\\"\"\n")},
		// (G1): the first character of a comment inside a token.
		{"an opener inside a literal",
			regionSource(" / \"a+\" id", ""),
			regionSource(" / \"a/\" id", "")},
		// (G): a skip of a recovery that can consume a comment.
		{"a recovery that skips into a comment",
			regionSource(" / (\"q\" #recover(skip=[^;/ ]*)) id", ""),
			regionSource(" / (\"q\" #recover(skip=[^;]*)) id", "")},
		// (G): a crossing in a lookahead counts as one outside it.
		{"a lookahead that holds a crossing",
			regionSource(" / \"!\" &(\"/\" !\"*\" id) id", ""),
			regionSource(" / \"!\" &(\"/\" id) id", "")},
		// (G): the skip and the operators of an Operators Rule.
		{"the skip of an Operators Rule",
			regionSource(" / \"!\" o", "rule o = #operators {\n    skip ws\n    operand \"x\"\n    level add { infix left \"-\" }\n}\n"),
			regionSource(" / \"!\" o", "rule o = #operators {\n    skip ([ ] / \"/\")*\n    operand \"x\"\n    level add { infix left \"-\" }\n}\n")},
		{"an operator of an Operators Rule",
			regionSource(" / \"!\" o", "rule o = #operators {\n    skip ws\n    operand id\n    level mul { infix left \"%\" }\n}\n"),
			regionSource(" / \"!\" o", "rule o = #operators {\n    skip ws\n    operand id\n    level mul { infix left \"/\" }\n}\n")},
		// (N): a rule that the body of a region reaches, called outside it.
		{"a rule called inside and outside a region",
			strings.Replace(regionSource(" / \"*\" id", "rule star = \"*\"\n"), "[^*]* \"*/\"", "[^*]* star \"/\"", 1),
			strings.Replace(regionSource(" / star id", "rule star = \"*\"\n"), "[^*]* \"*/\"", "[^*]* star \"/\"", 1)},
		// (O): two trivia rules whose first characters overlap, and two whose do not.
		{"two trivia rules",
			regionSource(" / \"[\" sp id \"]\"", "rule sp = [\\t]* (d [\\t]*)*\nrule d = \"#\" [^\\n]* \"\\n\"\n"),
			regionSource(" / \"[\" sp id \"]\"", "rule sp = [\\t]* (d [\\t]*)*\nrule d = \"//\" [^\\n]* \"\\n\"\n")},
		// (O): an auxiliary region that overlaps a trivia region gives way to it. The rule m is made
		// an auxiliary region for the crossing of sp, which it holds, and overlaps ws by its first
		// character. It is dropped, which leaves ws with the bound, if its slash is guarded; if it
		// is not, the slash is a crossing of ws as well, and ws fails.
		{"an auxiliary region and a trivia region",
			regionSource(" / \"[\" sp id \"]\" / m", "rule sp = [\\t]* (d [\\t]*)*\nrule d = \"#\" [^\\n /]* \"\\n\"\nrule m = \"/\" !\"*\" \"#\" \"x\"\n"),
			regionSource(" / \"[\" sp id \"]\" / m", "rule sp = [\\t]* (d [\\t]*)*\nrule d = \"#\" [^\\n /]* \"\\n\"\nrule m = \"/\" \"#\" \"x\"\n")},
		// (D): a region with an inline action that reads a variable.
		{"a region that reads a variable",
			"var v int = 0\n" + regionSource("", ""),
			"var v int = 0\n" + strings.Replace(regionSource("", ""), "rule c = \"/*\" [^*]* \"*/\"", "rule c = (\"/*\" [^*]* \"*/\" => v)", 1)},
		// (D): a region with a whole-rule action.
		{"a region with an action",
			regionSource("", ""),
			strings.Replace(regionSource("", ""), "rule c = \"/*\" [^*]* \"*/\"", "rule c = \"/*\" [^*]* \"*/\" => 1", 1)},
		// (D): a region in a left-recursion cycle.
		{"a region in a cycle",
			regionSource("", ""),
			strings.Replace(regionSource("", ""), "rule c = \"/*\" [^*]* \"*/\"", "rule c = c \"*\" / \"/*\" [^*]* \"*/\"", 1)},
		// (D): a region that makes a level call.
		{"a region with a level call",
			regionSource("", "rule o = #operators {\n    operand \"x\"\n    level add { infix left \"-\" }\n}\n"),
			strings.Replace(regionSource("", "rule o = #operators {\n    operand \"x\"\n    level add { infix left \"-\" }\n}\n"), "rule c = \"/*\" [^*]* \"*/\"", "rule c = \"/*\" [^*]* \"*/\" level(o, add)?", 1)},
		// (D): a region with a recovery, which the classifier reads as an attributed expression.
		{"a region with a recovery",
			regionSource("", ""),
			strings.Replace(regionSource("", ""), "rule c = \"/*\" [^*]* \"*/\"", "rule c = \"/*\" [^*]* (\"*/\" #recover(skip=[^;]*))", 1)},
		// (D): a region with a stream, likewise.
		{"a region with a stream",
			regionSource("", ""),
			strings.Replace(regionSource("", ""), "rule c = \"/*\" [^*]* \"*/\"", "rule c = \"/*\" ([^*]* #stream) \"*/\"", 1)},
		// (N): a region whose body reaches a region that only it calls. The letters before cb leave
		// its run open, which makes it a candidate.
		{"a region inside a region",
			strings.Replace(regionSource("", ""), "rule c = \"/*\" [^*]* \"*/\"", "rule c = \"/*\" [a-z]* ([a-z] / \"*\" [a-z])* \"*/\"", 1),
			strings.Replace(regionSource("", "rule cb = ([a-z] / \"*\" [a-z])*\n"), "rule c = \"/*\" [^*]* \"*/\"", "rule c = \"/*\" [a-z]* cb \"*/\"", 1)},
		// (D): a region with a predicate.
		{"a region with a predicate",
			"var w int = 0\n" + regionSource("", ""),
			"var w int = 0\n" + strings.Replace(regionSource("", ""), "rule c = \"/*\"", "rule c = {w := 1} \"/*\"", 1)},
		// (N): a region whose body reaches another region.
		{"a region that reaches a region",
			regionSource(" / \"[\" sp id \"]\"", "rule sp = [\\t]* (d [\\t]*)*\nrule d = \"#\" [^\\n]* \"\\n\"\n"),
			regionSource(" / \"[\" sp id \"]\"", "rule sp = [\\t]* (d [\\t]*)*\nrule d = \"#\" [^\\n]* \"\\n\" ws\n")},
	} {
		t.Run(c.name, func(t *testing.T) {
			if !regionCredit(t, c.credited) {
				t.Errorf("the twin has no region bound:\n%s", c.credited)
			}
			if regionCredit(t, c.broken) {
				t.Errorf("the broken grammar has the region bound:\n%s", c.broken)
			}
		})
	}
}

const unclosedComments = "rule main = (ws t)* ws $$\nrule t = \"/\" \"*\" [a-z]+\nrule ws = (\" \" / c)*\nrule c = \"/*\" (!\"*/\" .)* \"*/\"\n"

// TestRegionScanClasses checks the classes that the region bound changes and those that it leaves.
func TestRegionScanClasses(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      ComplexityClass
	}{
		{"comments in blanks", regionSource("", ""), ClassLinear},
		// The blanks are not memoized, so the second bound fails.
		{"blanks that are not memoized", strings.Replace(regionSource("", ""), "rule ws = [ ]* (c [ ]*)*", "rule ws = [ ]*", 1), ClassUnknown},
		{"a token that a comment begins with", regionSource(" / \"/\" id", ""), ClassUnknown},
		// The repetition of an unclosed comment scans to the end of the input from each opener.
		{"nested comments that are not closed", "rule main = ws \"x\" $$\nrule ws = (\" \" / c)*\nrule c = \"/*\" (c / !\"*/\" .)* \"*/\"\n", ClassUnknown},
		// The blanks have the region bound, but a comment that is not closed is no match of them: a
		// token where they ended consumes its opener, and the scan of the next one runs to the end of
		// the input again.
		{"comments that are not closed", unclosedComments, ClassPoly},
	} {
		if c.src == unclosedComments && !regionCredit(t, c.src) {
			t.Errorf("%s: no region bound", c.name)
		}
		got := classifySource(t, c.src)
		if got.Class != c.want {
			t.Errorf("%s: class %v, want %v\ncauses %v", c.name, got.Class, c.want, got.Causes)
		}
	}
}
