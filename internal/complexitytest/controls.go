package complexitytest

import (
	"strings"

	"zeroparse.dev/zp/internal/engine"
)

// The classes that the complexity classifier assigns to a start rule.
const (
	ClassLinear    = engine.ClassLinear
	ClassLinearEnv = engine.ClassLinearEnv
	ClassPoly      = engine.ClassPoly
	ClassExp       = engine.ClassExp
	ClassUnknown   = engine.ClassUnknown
)

// Control is a grammar family with a known growth order.
type Control struct {
	Name   string
	Family string // E0 to E10, or a name for a case outside the table
	Source string // ZeroParse source with the start rule main
	// Input builds the input of size n.
	Input func(n int) string
	// Sizes are n of the series n, 2n, 4n; ShortSizes are used with testing.Short.
	Sizes, ShortSizes []int
	// Order is the measured growth order of the budget spend.
	Order Order
	// Capped lets the 4n point reach Cap steps. Only Exp entries may be capped.
	Capped bool
	// Class is the class that the complexity classifier assigns to main (see TestControlClasses).
	Class engine.ComplexityClass
	// Positive marks a control that is linear and guards against a classifier or runtime that
	// reports growth where there is none.
	Positive bool
}

// Cap is the budget that stops an exponential control. It keeps the exponential cases short.
const Cap = 20_000_000

// parens is the input of E2 and E3: n open parentheses, "n", then n closing ones, each followed
// by "z".
func parens(n int) string { return rep("(", n) + "n" + rep(")z", n) }

func double(n int) []int { return []int{n, 2 * n, 4 * n} }

// regionGrammar is a grammar of nested terms whose blanks hold comments. The lookahead
// !(ws ".") fails after ws has scanned a comment, and what follows evaluates ws again where it
// started.
const regionGrammar = "rule main = (p \";\" ws)* $$\n" +
	"rule p = k !(ws \".\") / k ws \".\" ws id / \"(\" ws p ws \")\" / id\nrule k = \"true\"\nrule id = [a-z]+\n" +
	"rule ws = [ ]* (c [ ]*)*\nrule c = \"/*\" [^*]* \"*/\"\n"

// Controls returns the controls table. The negative controls record growth that the engine has
// today: when a planned change makes one linear, its entry moves to the linear set by hand. The
// negative E8 controls (repeated, nested and alternating walks) are linear in measurement through
// the records of skip runs, with classes that claim no linear cost, and so are E1, E7, E9 and the
// controls of word exclusion and nested scans whose quadratic cost is a repetition that a failed
// alternative walks again, through the records of run sites; E6 and E10 stay quadratic. The rebinding variants of E6 are positive
// controls: a binding after the left call adds no growth, and the restart in E6 is the negative
// control. Each source is a grammar from the measured families of the
// grammar complexity design record; "main" is the start rule.
func Controls() []Control {
	cs := []Control{
		{
			Name: "E0 repetition after backtracking, safe", Family: "E0", Positive: true,
			Source: "rule main = (a / \"a\")* $$\nrule a = \"a\" \"c\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(1000), ShortSizes: double(250), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "E1 repetition after backtracking", Family: "E1",
			Source: "rule main = (a / \"a\")* $$\nrule a = \"a\"* \"c\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		{
			// Sizes start above the length of input that a run site may walk again while it is
			// cold (core.RunHotSpan, 64): below it the walks are plain and the cost is cubic.
			Name: "E7 nested scans, degree 3", Family: "E7",
			Source: "rule main = (a / \"a\")* $$\nrule a = (b / \"a\")* \"c\"\nrule b = \"a\"* \"d\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		{
			Name: "E9 lookahead over scans", Family: "E9",
			Source: "rule main = (\"a\" !rest)* \"a\"* $$\nrule rest = \"a\"* \"c\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassUnknown,
		},
		// The lookahead is linear, since w is memoized, but each iteration copies the rest of the
		// input, which the capture in the lookahead holds.
		{
			Name: "Copy of a capture in a lookahead", Family: "lookahead copy",
			Source: "rule main = a* $$\nrule a string = \"a\" &(x:w) => text($x) + \"!\"\nrule w = \"a\" w / \"\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(500), ShortSizes: double(250), Order: Poly(2), Class: ClassUnknown,
		},
		{
			Name: "Copy of a lookahead capture that a rule returns", Family: "lookahead copy",
			Source: "rule main = a* $$\nrule a string = \"a\" y:look => text($y) + \"!\"\nrule look = &(x:w) => $x\nrule w = \"a\" w / \"\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(500), ShortSizes: double(250), Order: Poly(2), Class: ClassUnknown,
		},
		{
			Name: "Copy of the text of a lookahead capture that a rule returns", Family: "lookahead copy",
			Source: "rule main = a* $$\nrule a string = \"a\" y:look => \"!\" + $y\nrule look string = &(x:w) => text($x)\nrule w = \"a\" w / \"\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(500), ShortSizes: double(250), Order: Poly(2), Class: ClassUnknown,
		},
		{
			Name: "Copies of lookahead captures in the elements of a map", Family: "lookahead copy",
			Source: "rule main = b $$\nrule b []string = ps:p* => map($ps, (e) => text(e.$y) + \"!\")\nrule p = \"a\" y:look\nrule look = &(x:w) => $x\nrule w = \"a\" w / \"\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(500), ShortSizes: double(250), Order: Poly(2), Class: ClassUnknown,
		},
		// Each step of the fold copies a capture from outside its lambda, as large as the input.
		{
			Name: "Fold copying a capture from outside its lambda", Family: "outer copy",
			Source: "rule main = b $$\nrule b string = x:as \"|\" ps:c* => foldl(\"\", $ps, (s, e) => text($x) + \"!\")\nrule as = \"a\"*\nrule c = \"b\"\n",
			Input:  func(n int) string { return rep("a", n) + "|" + rep("b", n) },
			Sizes:  double(500), ShortSizes: double(250), Order: Poly(2), Class: ClassUnknown,
		},
		// A copy reads values that are not own, or parts of the values of calls on its recursion
		// that hold none: it costs at most the input that it reads, which each match gives to a
		// constant number of copies (Lemmas 6 and 11 of the linear cost proof).
		{
			Name: "Copy of an element of a list that map builds", Family: "read input", Positive: true,
			Source: "rule main = b $$\nrule b string = m:mm => $m[0] + \"!\"\nrule mm []string = xs:c* => map($xs, (q) => text($xs))\nrule c = \"a\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(500), ShortSizes: double(250), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Copy of a member of the value of a recursive call", Family: "read input", Positive: true,
			Source: "rule main = e $$\nrule e string = \"(\" z:f \")\" => text($z.$y) + \"!\"\nrule f = y:k* w:e?\nrule k = \"b\"\n",
			Input:  func(n int) string { return rep("(bb", n) + rep(")", n) },
			Sizes:  double(500), ShortSizes: double(250), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Copies of members of the elements of a list of recursive calls", Family: "read input", Positive: true,
			Source: "rule main = e $$\nrule e []string = \"(\" zs:f* \")\" => map($zs, (q) => text(q.$y) + \"!\")\nrule f = \"<\" y:k* w:e? \">\"\nrule k = \"b\"\n",
			Input:  func(n int) string { return "(" + rep("<bb>", n) + ")" },
			Sizes:  double(500), ShortSizes: double(250), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Copies of members of the elements of a list of recursive calls for each element", Family: "read input",
			Source: "rule main = e $$\nrule e [][]string = \"(\" zs:f* \")\" => map($zs, (a) => map($zs, (q) => text(q.$y) + \"!\"))\nrule f = \"<\" y:k* w:e? \">\"\nrule k = \"b\"\n",
			Input:  func(n int) string { return "(" + rep("<bb>", n) + ")" },
			Sizes:  double(250), ShortSizes: double(125), Order: Poly(2), Class: ClassUnknown,
		},
		{
			Name: "Copies of the elements of a list that map builds from a capture", Family: "read input",
			Source: "rule main = b $$\nrule b []string = l:mk => map($l, (e) => e + \"!\")\nrule mk []string = ys:k* => map($ys, (q) => text($ys))\nrule k = \"b\"\n",
			Input:  func(n int) string { return rep("b", n) },
			Sizes:  double(500), ShortSizes: double(250), Order: Poly(2), Class: ClassPoly,
		},
		{
			Name: "Text of a capture in a lookahead, safe", Family: "lookahead copy", Positive: true,
			Source: "rule main = a* $$\nrule a string = \"a\" &(x:w) => text($x)\nrule w = \"a\" w / \"\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(500), ShortSizes: double(250), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "E8 recovery skips", Family: "E8", Positive: true,
			Source: "rule main = (x / \"a\")* $$\nrule x = (y #recover(skip=[a]+)) \"c\"\nrule y = \"b\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			// An iteration of the skip consumes one or two characters; the walks still share their
			// runs, whose iterations each examine a constant amount of the input.
			Name: "E8 recovery skips of two characters", Family: "E8", Positive: true,
			Source: "rule main = (x / \"a\")* $$\nrule x = (y #recover(skip=(\"aa\" / \"a\")+)) \"c\"\nrule y = \"b\"\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			// Each walk starts one position before the one before it and runs to the end.
			Name: "E8 nested recovery skips", Family: "E8", Positive: true,
			Source: "rule main = (s \"!\" / [a(]*) $$\nrule s = (\"(\" s \")\") #recover(skip=[a(]+)\n",
			Input:  func(n int) string { return rep("(", n) + "a" },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			// The walks alternate between the two runs of "a": one from each position of the first
			// and one from the start of the second.
			Name: "E8 recovery skips in alternating regions", Family: "E8", Positive: true,
			Source: "rule main = (p / \"a\" / \"b\")* $$\nrule p = x \"b\" x \"z\"\nrule x = \"c\" #recover(skip=[a]+)\n",
			Input:  func(n int) string { return rep("a", n) + "b" + rep("a", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "E6 right recursion, safe", Family: "E6", Positive: true,
			Source: "rule main = (e \"q\" / \"x\")* $$\nrule e = \"x\" e / \"x\"\n",
			Input:  func(n int) string { return rep("x", n) },
			Sizes:  double(500), ShortSizes: double(125), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "E6 left recursion restarted", Family: "E6",
			Source: "rule main = (e \"q\" / \"x\")* $$\nrule e = e \"x\" / \"x\"\n",
			Input:  func(n int) string { return rep("x", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		{
			Name: "LR rebinding after the left call, growing value", Family: "E6 variant", Positive: true,
			Source: "var s string = \"a\"\nrule main = e $$\nrule e = e \"x\" {s := s + s} / \"x\" {len(s) > 0}\n",
			Input:  func(n int) string { return rep("x", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinearEnv,
		},
		{
			Name: "LR rebinding after the left call, counter", Family: "E6 variant", Positive: true,
			Source: "var k int\nrule main = e $$\nrule e = e \"x\" {k := k + 1} / \"x\" {k >= 0}\n",
			Input:  func(n int) string { return rep("x", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinearEnv,
		},
		{
			Name: "E10 Operators Rule restarted", Family: "E10",
			Source: "rule main = (e \"q\" / \"x\")* $$\nrule e = #operators {\n    operand \"x\"\n    level add { infix left \"\" }\n}\n",
			Input:  func(n int) string { return rep("x", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		{
			Name: "Operators Rule skip shared with the follow, safe", Family: "shared skip", Positive: true,
			Source: "rule main = s* $$\nrule s = \"r\" e? ws \";\"\nrule e = #operators {\n    skip ws\n    operand \"x\"\n    level add { infix left \"+\" }\n}\nrule ws = \" \"*\n",
			Input:  func(n int) string { return rep("r x + x ;", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Operators Rule skip shared, restarted", Family: "shared skip",
			Source: "rule main = (e \"q\" / ws \"x\")* $$\nrule e = #operators {\n    skip ws\n    operand \"x\"\n    level add { infix left \"\" }\n}\nrule ws = \" \"*\n",
			Input:  func(n int) string { return rep("x", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		{
			Name: "Operators Rule skip that must consume input, optional", Family: "shared skip",
			Source: "rule main = (e \"(\")* $$\nrule e = #operators {\n    skip \" \"+\n    operand \"y\"\n    level { postfix \"(\" [^;]* \";\" }\n}\n",
			Input:  func(n int) string { return rep("y(", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		// A call that the memo does not answer costs its scan each time. The continuation of a
		// failed unit evaluates the shared call again at the same position: a bounded number of
		// times when it reaches the call within the bodies around the unit, and once for each
		// level when it reaches it through the end of a recursion, which can nest as deep as the
		// input.
		{
			Name: "Scan shared by the alternatives of nested rules", Family: "rescanned calls", Positive: true,
			Source: "rule main = b* $$\nrule b = a \"c\" / a \"d\"\nrule a = ws \"a\" / ws \"b\"\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("  bd", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Memoized scan shared through the end of a recursion", Family: "rescanned calls", Positive: true,
			Source: "rule main = e ws $$\nrule e = \"(\" e? (ws \")\")?\nrule ws = sp*\nrule sp = \" \"\n",
			Input:  func(n int) string { return rep("(", n) + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Scan shared through the end of a recursion", Family: "rescanned calls",
			Source: "rule main = e ws $$\nrule e = \"(\" e? (ws \")\")?\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Scan shared through the ends of rules around a recursion", Family: "rescanned calls",
			Source: "rule main = e ws $$\nrule e = \"(\" e? t\nrule t = o\nrule o = (ws \")\")?\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Scan shared after a literal through the end of a recursion", Family: "rescanned calls",
			Source: "rule main = e \"x\" ws $$\nrule e = \"(\" e? (\"x\" ws \")\")?\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + "x" + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Scan shared after a memoized call through the end of a recursion", Family: "rescanned calls",
			Source: "rule main = e m ws $$\nrule e = \"(\" m e? (m ws \")\")?\nrule m = mx\nrule mx = \"x\"\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("(x", n) + "x" + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Memoized scan shared after a literal through the end of a recursion", Family: "rescanned calls", Positive: true,
			Source: "rule main = e \"x\" ws $$\nrule e = \"(\" e? (\"x\" ws ws \")\")?\nrule ws = sp*\nrule sp = \" \"\n",
			Input:  func(n int) string { return rep("(", n) + "x" + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		// A nesting restarted at one end: a unit fails after its call ended at a position, and a
		// continuation that evaluates the same call alike, through the memo answer, ends the rule
		// there, so every level ends where the innermost one ends and scans the spaces after it
		// again. R is not pure, but its repetition is a scan site, whose records answer the levels
		// from the second one on.
		{
			Name: "Scan restarted at the end of nested alternatives", Family: "nesting restarts",
			Source: "rule main = e R $$\nrule e = \"(\" (e R \"x\" / e) / \".\"\nrule R = &\"\" [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + "." + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		// The same restart of a scan in a rule that calls a rule, which is no scan site, so that no
		// record answers the levels.
		{
			Name: "Scan of a rule that calls a rule restarted at the end of nested alternatives", Family: "nesting restarts",
			Source: "rule main = e S $$\nrule e = \"(\" (e R \"x\" / e) / \".\"\nrule R = [ ]* Q\nrule Q = \"\"\nrule S = [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + "." + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		{
			Name: "Scan restarted through a looked-through call", Family: "nesting restarts",
			Source: "rule main = m R \"z\" $$\nrule z = \"(\" y R \"x\"\nrule y = m\nrule m = \".\" / z \"!\" / \"(\" m\nrule R = &\"\" [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + "." + rep(" ", n) + "z" },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		// The same restart through the operands of an Operators Rule: the first operand fails after
		// its call ended at a position, and the later operand ends the rule there through the memo
		// answer of that call.
		{
			Name: "Scan restarted by a later operand", Family: "nesting restarts",
			Source: "rule main = e R $$\nrule e = #operators {\n    operand \"(\" e R \"x\"\n    operand \"(\" e\n    operand \".\"\n    level { postfix \"!\" }\n}\nrule R = &\"\" [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + "." + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassUnknown,
		},
		// An operator part fails after its scan ran over the operator text that follows, and the
		// other part with the same literal succeeds, after which the rule applies the operator
		// parts again inside that text: R scans the rest of the input once for each "[".
		{
			Name: "Scan restarted by the next operator application", Family: "shared literals",
			Source: "rule main = e $$\nrule e = #operators {\n    operand \".\"\n    level { postfix \"[\" R \"x\"\n        postfix \"[\" }\n}\nrule R = [\\[]*\n",
			Input:  func(n int) string { return "." + rep("[", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		// Each nested evaluation of a left-recursive rule that ends at a position grows there, and
		// each step of its growth scans what follows its seed again.
		{
			Name: "Scan after the seed of nested growths", Family: "nesting restarts",
			Source: "rule main = e R $$\nrule e = e R \"x\" / \"(\" e / \".\"\nrule R = &\"\" [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + "." + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Operators Rule skip shared by nested prefix operators", Family: "rescanned calls",
			Source: "rule main = x ws $$\nrule x = #operators {\n    skip ws\n    operand \"a\"\n    level { prefix \"(\"\n        postfix \")\" }\n}\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + "a" + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Operators Rule part shared by nested right operands", Family: "rescanned calls",
			Source: "rule main = x ws $$\nrule x = #operators {\n    operand \"a\"\n    level { infix right ws \"^\" }\n}\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("a^", n) + "a" + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassUnknown,
		},
		{
			Name: "Operators Rule part shared by nested operands", Family: "rescanned calls",
			Source: "rule main = x ws $$\nrule x = #operators {\n    operand \"a\"\n    operand \"(\" x?\n    level { postfix ws \"!\" }\n}\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Operators Rule part shared by operands nested through a rule", Family: "rescanned calls",
			Source: "rule main = x ws $$\nrule x = #operators {\n    operand \"a\"\n    operand \"(\" y?\n    level { postfix ws \"!\" }\n}\nrule y = x\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Operators Rule parts shared by operands of two nested Operators Rules", Family: "rescanned calls",
			Source: "rule main = x ws $$\nrule x = #operators {\n    operand \"a\"\n    operand \"(\" z?\n    level { postfix ws \"!\" }\n}\nrule z = #operators {\n    operand \"b\"\n    operand \"(\" x?\n    level { postfix ws \"?\" }\n}\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("(", n) + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Operators Rule part shared by operator parts that nest", Family: "rescanned calls",
			Source: "rule main = x ws $$\nrule x = #operators {\n    operand \"a\"\n    level { postfix \"(\" x? }\n    level { postfix ws \"!\" }\n}\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("a(", n) + "a" + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassUnknown,
		},
		{
			Name: "Operators Rule memoized part shared by nested operands", Family: "rescanned calls", Positive: true,
			Source: "rule main = x ws $$\nrule x = #operators {\n    operand \"a\"\n    operand \"(\" x?\n    level { postfix ws \"!\" }\n}\nrule ws = sp*\nrule sp = \" \"\n",
			Input:  func(n int) string { return rep("(", n) + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Operators Rule memoized skip shared by nested prefix operators", Family: "rescanned calls", Positive: true,
			Source: "rule main = x ws $$\nrule x = #operators {\n    skip ws\n    operand \"a\"\n    level { prefix \"(\"\n        postfix \")\" }\n}\nrule ws = sp*\nrule sp = \" \"\n",
			Input:  func(n int) string { return rep("(", n) + "a" + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			// A level of a prefix operator alone applies no operator after an operand, so the skip
			// is not evaluated again at each level of the chain: "a"^n "c" " "^n costs n^2 steps
			// if it is.
			Name: "Operators Rule skip after nested prefix operators that no operator follows", Family: "rescanned calls", Positive: true,
			Source: "rule main = x ws $$\nrule x = #operators {\n    skip ws\n    operand \"c\"\n    level { prefix \"a\" }\n}\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("a", n) + "c" + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Operators Rule part shared by left operands", Family: "rescanned calls", Positive: true,
			Source: "rule main = x ws $$\nrule x = #operators {\n    operand \"a\"\n    level { infix left ws \"^\" }\n}\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("a^", n) + "a" + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Keyword excluded by a negative lookahead, safe", Family: "word exclusion", Positive: true,
			Source: "rule main = (s / \" \")* $$\nrule s = kw (\" \" / \"let\")* \"q\" / id\nrule kw = \"let\" !c\nrule c = [a-z]\nrule id = !key c+\nrule key = (\"if\" / \"let\") !c\n",
			Input:  func(n int) string { return rep("ab let q ", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Keyword lookahead reading a variable", Family: "word exclusion",
			Source: "var v string\nrule main = (s / \" \")* $$\nrule s = kw (\" \" / \"let\")* \"q\" / id\nrule kw = \"let\" !c\nrule c = [a-z]\nrule id = !key c+\nrule key = (\"if\" / \"let\") !c {v != \"\"}\n",
			Input:  func(n int) string { return rep("let ", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		{
			Name: "Keyword lookahead with a nested lookahead", Family: "word exclusion",
			Source: "rule main = (s / \" \")* $$\nrule s = kw (\" \" / \"let\")* \"q\" / id\nrule kw = \"let\" !c\nrule c = [a-z]\nrule id = !key c+\nrule key = (\"if\" / \"let\" !\" \") !c\n",
			Input:  func(n int) string { return rep("let ", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		{
			Name: "Keyword lookahead under a level restriction", Family: "word exclusion",
			Source: "rule main = (s / \" \")* $$\nrule s = kw (\" \" / \"let\")* \"q\" / id\nrule kw = \"let\" !c\nrule c = [a-z]\nrule id = !key c+\nrule key = level(ops, tight) !c\nrule ops = #operators {\n    operand \"le\"\n    level loose { postfix \"t\" }\n    level tight { postfix \"!\" }\n}\n",
			Input:  func(n int) string { return rep("let ", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		{
			Name: "Keyword lookahead with a shorter earlier keyword", Family: "word exclusion",
			Source: "rule main = (s / \" \")* $$\nrule s = kw (\" \" / \"let\")* \"q\" / id\nrule kw = \"let\" !c\nrule c = [a-z]\nrule id = !key c+\nrule key = (\"le\" / \"let\") !c\n",
			Input:  func(n int) string { return rep("let ", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		{
			Name: "Keyword lookahead with a wider character set", Family: "word exclusion",
			Source: "rule main = (s / \" \")* $$\nrule s = kw [a-z ]* \"q\" / id\nrule kw = \"let\" ![a-m]\nrule c = [a-z]\nrule id = !key c+\nrule key = (\"if\" / \"let\") !c\n",
			Input:  func(n int) string { return rep("letz ", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		{
			Name: "Operator part excluded by the text it consumes, safe", Family: "word exclusion", Positive: true,
			Source: "rule main = (e (\"=\" !\"=\" e)? \";\")* $$\nrule e = #operators {\n    operand \"x\"\n    operand \"(\" e \")\"\n    level { infix none \"==\" }\n}\n",
			Input:  func(n int) string { return rep("x==(", n) + "x" + rep(")", n) + "=x;" },
			Sizes:  double(64), ShortSizes: double(32), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Literal excluded by the text it shares, safe", Family: "word exclusion", Positive: true,
			Source: "rule main = (s / \" \" / \"x\")* $$\nrule s = \"==\" (\" \" / \"=\")* \"q\" / \"=\" !\"=\" \"x\"\n",
			Input:  func(n int) string { return rep("== q =x ", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Literal lookahead that the text does not match", Family: "word exclusion",
			Source: "rule main = (s / \" \")* $$\nrule s = \"==\" (\" \" / \"=\")* \"q\" / \"=\" !\"y\" \"=\"\n",
			Input:  func(n int) string { return rep("== ", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		{
			Name: "Literal lookahead longer than the text", Family: "word exclusion",
			Source: "rule main = (s / \" \")* $$\nrule s = \"==\" (\" \" / \"=\")* \"q\" / \"=\" !\"==\" \"=\"\n",
			Input:  func(n int) string { return rep("== ", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		{
			Name: "Literal longer than the text", Family: "word exclusion",
			Source: "rule main = (s / \" \")* $$\nrule s = \"let\" [a-z ]* \"q\" / \"lett\" [a-z]*\n",
			Input:  func(n int) string { return rep("lett ", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		{
			Name: "E2 cycle members behind a leader, safe", Family: "E2", Positive: true,
			Source: "rule main = c $$\nrule a = \"n\"\nrule c = \"(\" c \")\" \"y\" / \"(\" c \")\" \"z\" / a\n",
			Input:  parens, Sizes: double(64), ShortSizes: double(32), Order: Linear, Class: ClassLinear,
		},
		// The cycle members are memoized outside the growths at their positions, and the last
		// alternative of c enters the growth where the first two began, which they examine only
		// inside the memoized call of c.
		{
			Name: "E2 non-leader cycle members", Family: "E2", Positive: true,
			Source: "rule main = c $$\nrule a = c \"+\" / \"n\"\nrule c = \"(\" c \")\" \"y\" / \"(\" c \")\" \"z\" / a\n",
			Input:  parens, Sizes: double(64), ShortSizes: double(32), Order: Linear, Class: ClassLinear,
		},
		// c is a member of the cycle led by a, called by s where the cycle does not grow, so its
		// result is memoized there and the second alternative of s reuses it after the shared
		// literal; the growth that the call starts continues after the call.
		{
			Name: "E2 cycle member called outside its growth", Family: "E2", Positive: true,
			Source: "rule main = s $$\nrule s = \"(\" c \")\" \"y\" / \"(\" c \")\" \"z\"\nrule a = c \"+\" / \"n\" / \"[\" s \"]\"\nrule c = a\n",
			Input:  func(n int) string { return rep("([", n) + "(n)z" + rep("])z", n) },
			Sizes:  double(64), ShortSizes: double(32), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "E3 environments, safe", Family: "E3", Positive: true,
			Source: "var v string\nrule main = r $$\nrule r = \"(\" {v := \"0\"} r \")\" \"y\" / \"(\" {v := \"1\"} r \")\" \"z\" / \"n\" {len(v) >= 0}\n",
			// len examines all of v, whose size the classifier bounds only by its value.
			Input: parens, Sizes: double(64), ShortSizes: double(32), Order: Linear, Class: ClassLinearEnv,
		},
		{
			Name: "E3 diverging environments", Family: "E3",
			Source: "var v string\nrule main = r $$\nrule r = \"(\" {v := v + \"0\"} r \")\" \"y\" / \"(\" {v := v + \"1\"} r \")\" \"z\" / \"n\" {len(v) >= 0}\n",
			Input:  parens, Sizes: double(3), ShortSizes: double(2), Order: Exp, Capped: true, Class: ClassExp,
		},
		{
			Name: "E4 self-growing variable", Family: "E4",
			Source: "var s string = \"a\"\nrule main = r $$\nrule r = \"x\" {s := s + s} r / {len(s) > 0}\n",
			Input:  func(n int) string { return rep("x", n) },
			Sizes:  double(4), ShortSizes: double(3), Order: Exp, Capped: true, Class: ClassExp,
		},
		{
			Name: "E5 self-growing fold", Family: "E5",
			Source: "rule main = xs:\"x\"* => len(foldl(\"a\", $xs, (acc, x) => acc + acc))\n",
			Input:  func(n int) string { return rep("x", n) },
			Sizes:  double(4), ShortSizes: double(3), Order: Exp, Capped: true, Class: ClassExp,
		},
		// The memo evaluates a rule at most twice at a position, but it does not share the scan of
		// a memoized rule between positions. In both controls every rule that scans is memoized, and
		// a continuation that consumes one character evaluates the scan again one position later:
		// through a repetition, and through a memoized recursion in which the unit that is not
		// committing is nested in itself.
		{
			Name: "Memoized scan restarted by a repetition", Family: "memo bound",
			Source: "rule main = (a \"y\" / \"q\" a / .)* $$\nrule a = x+\nrule x = \"x\"\n",
			Input:  func(n int) string { return rep("x", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		{
			Name: "Memoized scan restarted through a memoized recursion", Family: "memo bound",
			Source: "rule main = t a? $$\nrule t = a t \"!\" / . t / \"\"\nrule a = x+\nrule x = \"x\"\n",
			Input:  func(n int) string { return rep("x", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		// A later growing alternative that succeeds ends a step with a longer result, and the next
		// step scans again from its end the input that the failed growing unit examined: what
		// follows it at the end of the rule body includes the next step, so the shared call sp
		// does not make the unit committing.
		{
			Name: "Growing unit restarted by the next step", Family: "growth steps",
			Source: "rule main = a $$\nrule a = a sp [^!]* \"!\" / a sp / \"n\"\nrule sp = \"+\"\n",
			Input:  func(n int) string { return "n" + rep("+", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		{
			Name: "Growing unit restarted by the next step through a member", Family: "growth steps",
			Source: "rule main = a $$\nrule a = c sp [^!]* \"!\" / c sp / \"n\"\nrule c = a\nrule sp = \"+\"\n",
			Input:  func(n int) string { return "n" + rep("+", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		// The same restart when the two alternatives share a literal instead of a call.
		{
			Name: "Growing unit restarted by the next step after a shared literal", Family: "growth steps",
			Source: "rule main = a $$\nrule a = c \"+\" [^!]* \"!\" / c \"+\" / \"n\"\nrule c = a\n",
			Input:  func(n int) string { return "n" + rep("+", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		// The call of c after "(" is not a left call, so the left call of the leader in c starts
		// a growth there, and the parser continues after it with "+" when the growth ends: the
		// growing unit is restarted by what follows the growth, though c = a reads the seed in
		// the steps of other growths.
		{
			Name: "Growing unit restarted after a growth that a member starts", Family: "growth steps",
			Source: "rule main = c \"+\" [^!]* \"!\" / \"(\" c \"+\" / \"n\"\nrule c = main\n",
			Input:  func(n int) string { return rep("(", n) + "n" + rep("+", n) },
			Sizes:  double(100), ShortSizes: double(50), Order: Poly(2), Class: ClassUnknown,
		},
		{
			Name: "Growing unit restarted after a growth that a member starts, to the end", Family: "growth steps",
			Source: "rule main = a $$\nrule a = c \"+\" [^!]* \"!\" / \"(\" c \"+\" / \"n\"\nrule c = a\n",
			Input:  func(n int) string { return rep("(", n) + "n" + rep("+", n) },
			Sizes:  double(100), ShortSizes: double(50), Order: Poly(2), Class: ClassUnknown,
		},
		// The last alternative of c enters the growth where the first began, and each step of
		// the growth evaluates the first again: its repetition after "(" scans the same input in
		// every step.
		{
			Name: "Growth restarting a scan at a fixed position", Family: "growth steps",
			Source: "rule main = c $$\nrule c = \"(\" [a-z]* \"!\" / a / \"(\" [a-z]* \"?\"\nrule a = c \"+\" / \"n\" / \"(\" [a-z]* \"?\"\n",
			Input:  func(n int) string { return "(" + rep("z", n) + "?" + rep("+", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		// The start rule is called only from its own body, but a parse can begin at it, so it is
		// memoized (Evaluations of Rules): the second iteration of the repetition, which
		// calls it again at the position where the first iteration of its child called it, reads
		// the memo. Were the start rule not memoized, the evaluations would grow as the Fibonacci
		// numbers. The rule t, called from two places, matches the rest of the input once per
		// position.
		{
			Name: "Start rule called from its own body inside a bounded repetition", Family: "single callers", Positive: true,
			Source: "@start\nrule main = (\"(\" main \")!\" / \"(\")#repeat(1,2) t\nrule t = \"(\" t / \"\"\n",
			Input:  func(n int) string { return rep("(", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Start rule on a cycle of rules called from one place", Family: "single callers", Positive: true,
			Source: "@start\nrule main = (\"(\" b \")!\" / \"(\")#repeat(1,2) t\nrule b = main\nrule t = \"(\" t / \"\"\n",
			Input:  func(n int) string { return rep("(", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		// The alternatives of main share the "(" with which they begin, and the iterations of b
		// begin with "(" too, so after the first alternative fails at the end, the second consumes
		// one "(" and the next iteration calls b again one position later: b, called from one place
		// and not memoized, scans the rest of the input again from each position.
		{
			Name: "Scan restarted inside a rule called from one place", Family: "nested scans",
			Source: "@start\nrule main = (\"(\" b \")!\" / \"(\")+ \"(\"*\nrule b = (\"(\" c \")!\" / \"(\")* $$\nrule c = \"(\" c / \"x\"\n",
			Input:  func(n int) string { return rep("(", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassUnknown,
		},
		// Two callers of one repetition: both alternatives call b after the "(" they share, so b
		// walks from the same position twice, and its iterations begin with "(" as the callers do,
		// so the next iteration of main walks b again from the next position. The records of b's
		// repetition serve the second caller and the later walks.
		{
			Name: "Scan restarted from two callers", Family: "nested scans",
			Source: "@start\nrule main = (\"(\" b \")!\" / \"(\" b \")?\" / \"(\")+ \"(\"*\nrule b = (\"(\" c \")!\" / \"(\")* $$\nrule c = \"(\" c / \"x\"\n",
			Input:  func(n int) string { return rep("(", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassUnknown,
		},
		// A literal that both alternatives begin with puts the memoized call after it at the same
		// position in both, so the second alternative reuses its result.
		{
			Name: "Memoized call shared after a literal", Family: "shared literals", Positive: true,
			Source: "rule main = x $$\nrule x = \"(\" y \")\" \"a\" / \"(\" y \")\" \"b\" / \"n\"\nrule y = \"z\"+ / x\n",
			Input:  func(n int) string { return rep("(", n) + "z" + rep(")b", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		// The rule that starts the right recursion comes before the rule whose action copies the
		// tail, so the classifier needs the captures of a rule that it walks later.
		{
			Name: "Tail copied along a right recursion", Family: "memo bound",
			Source: "rule main = s $$\nrule s []string = \"\\\"\" t:r => concat(list(\"q\"), $t)\nrule r []string = m / l\nrule m []string = e:i \",\" t:r => concat(list($e), $t)\nrule l []string = e:i \"\\\"\" => list($e)\nrule i string = x:(\"(\" s \")\" / [a-z]+) => text($x)\n",
			Input: func(n int) string {
				items := make([]string, n)
				for i := range items {
					items[i] = "a"
				}
				return "\"" + strings.Join(items, ",") + "\""
			},
			Sizes: double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		// A lambda over a list with at most a constant number of elements runs at most that many
		// times, so its copies from outside it, and the copies of a fold accumulator, are linear;
		// inside a lambda over a list that is not bounded, the same fold runs once for each of its
		// elements.
		{
			Name: "Fold over a bounded list", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* b:\"y\" #repeat(,2) => foldl(text($a), $b, (acc, y) => acc + text($a))\n",
			Input:  func(n int) string { return rep("x", n) + "yy" },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Bounded map copying a value from outside", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* b:\"y\" #repeat(,2) => map($b, (y) => text($a) + text(y))\n",
			Input:  func(n int) string { return rep("x", n) + "yy" },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Bounded elements inside a map that is not bounded", Family: "bounded lambda",
			Source: "rule main = zs:\"z\"* b:\"y\" #repeat(,2) => map($zs, (z) => foldl(\"\", $b, (acc, y) => acc + text(y)))\n",
			Input:  func(n int) string { return rep("z", n) + "yy" },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Bounded fold inside a map that is not bounded", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* zs:(b:\"y\" #repeat(,1) \"z\")* => map($zs, (z) => foldl(\"\", z.$b, (acc, y) => acc + text($a)))\n",
			Input:  func(n int) string { return rep("x", n) + rep("yz", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		// A fold whose accumulator doubles inside an element of the list it builds grows by a
		// factor exponential in the number of steps, which the bound limits but the input reaches.
		// Duplicating the elements of the accumulator, or padding each and joining them, doubles
		// it on each step; joining each element once keeps its size, but the inner fold copies its
		// own accumulator once for each element; the classifier knows that the accumulator keeps
		// the one element of the initial value, of up to n characters, so it bounds the steps by
		// n^2, though here that element holds one character and the fold measures linear.
		{
			Name: "Fold duplicating and joining elements", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => foldl(list(), map(acc, (e) => list(e, e)), (s, l) => concat(s, l)))\n",
			Input:  func(n int) string { return "x" + rep("y", n) },
			Sizes:  double(3), ShortSizes: double(3), Order: Exp, Capped: true, Class: ClassExp,
		},
		{
			Name: "Fold padding and joining elements", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => foldl(list(), map(acc, (e) => list(e, \"x\")), (s, l) => concat(s, l)))\n",
			Input:  func(n int) string { return "x" + rep("y", n) },
			Sizes:  double(3), ShortSizes: double(3), Order: Exp, Capped: true, Class: ClassExp,
		},
		{
			Name: "Fold joining elements once", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => foldl(list(), map(acc, (e) => list(e)), (s, l) => concat(s, l)))\n",
			Input:  func(n int) string { return "x" + rep("y", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		// A value read from outside the fold adds up to n on each of n steps, so the accumulator
		// grows quadratically and copying it costs n^3.
		{
			Name: "Fold adding an outside value", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* b:\"y\"* c:\"z\"* => foldl(text($a), $b, (acc, y) => acc + text($c))\n",
			Input:  func(n int) string { return "x" + rep("y", n) + rep("z", n) },
			Sizes:  double(40), ShortSizes: double(25), Order: Poly(3), Class: ClassUnknown,
		},
		// Measuring the first element, of n characters, for each of n elements examines n^2
		// characters on each of n steps.
		{
			Name: "Fold measuring its first element for every element", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* b:\"y\"* => foldl(concat(list(text($a)), map($a, (z) => text(z))), $b, (acc, y) => filter(acc, (e) => len(acc[0]) > 0))\n",
			Input:  func(n int) string { return rep("x", n) + rep("y", n) },
			Sizes:  double(40), ShortSizes: double(25), Order: Poly(3), Class: ClassPoly,
		},
		// A fold that copies its accumulator inside a map over a list as large as its own runs once
		// for each element of the map.
		{
			Name: "Fold inside a map over the same list", Family: "bounded lambda",
			Source: "rule main = b:\"y\"* => map($b, (y) => foldl(\"\", $b, (s, l) => s + text(l)))\n",
			Input:  func(n int) string { return rep("y", n) },
			Sizes:  double(40), ShortSizes: double(40), Order: Poly(3), Class: ClassPoly,
		},
		// Elements that capture the rest of the input in a lookahead overlap, so a fold over the
		// capture of each element runs over up to n elements for each of n elements.
		{
			Name: "Fold over a capture in a lookahead of each element", Family: "bounded lambda",
			Source: "rule main = items:item* => map($items, (it) => foldl(\"\", it.$c, (s, l) => s + text(l)))\nrule item = &(c:\"x\"*) \"x\"\n",
			Input:  func(n int) string { return rep("x", n) },
			Sizes:  double(90), ShortSizes: double(80), Order: Poly(3), Class: ClassUnknown,
		},
		// A map that extends every element of an accumulator of n elements on each of n steps
		// costs n per step for each element.
		{
			Name: "Fold extending every element", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* b:\"y\"* => foldl(map($a, (z) => text(z)), $b, (acc, y) => map(acc, (e) => e + \"x\"))\n",
			Input:  func(n int) string { return rep("x", n) + rep("y", n) },
			Sizes:  double(40), ShortSizes: double(25), Order: Poly(3), Class: ClassPoly,
		},
		// The value of sq joins its whole text once for each of its n characters, n^2 in all, so
		// the fold of main that adds it on each of n steps builds an accumulator of n^3 characters
		// and copies it on every step.
		{
			Name: "Fold adding a quadratic value of a rule", Family: "value sizes",
			Source: "rule main = c:sq b:\"y\"* => foldl(\"\", $b, (acc, y) => acc + $c)\nrule sq = a:\"x\"* => foldl(\"\", $a, (s, z) => s + text($a))\n",
			Input:  func(n int) string { return rep("x", n) + rep("y", n) },
			Sizes:  double(12), ShortSizes: double(10), Order: Poly(4), Class: ClassUnknown,
		},
		// The inner map holds the text of the whole list once for each element, so the elements
		// add up to n^2 characters, which the outer map copies.
		{
			Name: "Map copying elements that add up to more than the input", Family: "value sizes",
			Source: "rule main = b:\"y\"* => map(map($b, (q) => text($b)), (y) => y + \"x\")\n",
			Input:  func(n int) string { return rep("y", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		// The initial value holds the text of $a once for each of its elements, n^2 characters,
		// which each of n steps measures.
		{
			Name: "Fold from a quadratic initial value", Family: "value sizes",
			Source: "rule main = a:\"x\"* b:\"y\"* => foldl(map($a, (z) => text($a)), $b, (acc, y) => filter(acc, (e) => len(e) > 0))\n",
			Input:  func(n int) string { return rep("x", n) + rep("y", n) },
			Sizes:  double(40), ShortSizes: double(25), Order: Poly(3), Class: ClassPoly,
		},
		// The value of e holds the value of the a inside it twice, and that value is the value of
		// the e inside it, so the value doubles on each level of the nesting.
		{
			Name: "Fold holding a recursive value twice", Family: "value sizes",
			Source: "rule main = e $$\nrule e string = l:a rest:(o:\"+\" r:a)* => foldl($l, $rest, (acc, i) => acc + text(i.$o) + i.$r + i.$r)\nrule a string = (\"x\" => \"x\") / (\"(\" v:e \")\" => $v)\n",
			Input:  func(n int) string { return rep("x+(", n) + "x" + rep(")", n) },
			Sizes:  double(3), ShortSizes: double(3), Order: Exp, Class: ClassExp,
		},
		// The same with a constant value of a: the copies hold nothing of the nesting.
		{
			Name: "Fold holding twice a value that does not hold the recursion", Family: "value sizes",
			Source: "rule main = e $$\nrule e string = l:a rest:(o:\"+\" r:a)* => foldl($l, $rest, (acc, i) => acc + text(i.$o) + i.$r + i.$r)\nrule a string = (\"x\" => \"x\") / (\"(\" e \")\" => \"v\")\n",
			Input:  func(n int) string { return rep("x+(", n) + "x" + rep(")", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		// The predicate joins the value of the l inside it to itself on each level, which costs
		// that value twice but leaves the value of l one character longer per level: n^2.
		{
			Name: "Predicate copying a recursive value twice", Family: "value sizes",
			Source: "rule main = l $$\nrule l string = (\"(\" x:l \")\" {len($x + $x) >= 0} => $x + \"y\") / (\"a\" => \"a\")\n",
			Input:  func(n int) string { return rep("(", n) + "a" + rep(")", n) },
			Sizes:  double(200), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		// Elements of their own text add up to the input, so copying each of them is linear.
		{
			Name: "Map copying elements of their own text", Family: "value sizes", Positive: true,
			Source: "rule main = b:\"y\"* => map(map($b, (q) => text(q)), (y) => y + \"x\")\n",
			Input:  func(n int) string { return rep("y", n) },
			Sizes:  double(1000), ShortSizes: double(250), Order: Linear, Class: ClassLinear,
		},
		{
			Name: "Bounded fold doubling an element it keeps", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* b:\"y\" #repeat(,14) => foldl(list(text($a)), $b, (acc, y) => concat(list(acc[0] + acc[0]), acc))\n",
			Input:  func(n int) string { return rep("x", 64) + rep("y", n) },
			Sizes:  double(3), ShortSizes: double(3), Order: Exp, Class: ClassExp,
		},
		{
			Name: "Bounded fold building a doubled element", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* b:\"y\" #repeat(,14) => foldl(list(text($a)), $b, (acc, y) => list(acc[0] + acc[0]))\n",
			Input:  func(n int) string { return rep("x", 64) + rep("y", n) },
			Sizes:  double(3), ShortSizes: double(3), Order: Exp, Class: ClassExp,
		},
		{
			Name: "Bounded fold doubling each element", Family: "bounded lambda",
			Source: "rule main = a:\"x\"* b:\"y\" #repeat(,14) => foldl(list(text($a)), $b, (acc, y) => map(acc, (e) => e + e))\n",
			Input:  func(n int) string { return rep("x", 64) + rep("y", n) },
			Sizes:  double(3), ShortSizes: double(3), Order: Exp, Class: ClassExp,
		},
		// A binding that reads its own earlier value before a recursive call that follows consumed
		// input still reaches each position with a different value from every start, so the
		// number of combinations of values at a position grows with the input. The memo finds the
		// entry of a combination through a hash index, so the cost is the number of (position,
		// value) pairs, quadratic; walking the entries of a position would make it cubic. The
		// sizes start at 100 because a chain is walked until it passes 32 entries.
		{
			Name: "Counter bound before a call after consumed input", Family: "bounded binding",
			Source: "var v int\nrule main = (r / .)* $$\nrule r = \"#\" {v := v + 1} r / \"y\" {v >= 0}\n",
			Input:  func(n int) string { return rep("#", n) },
			Sizes:  double(100), ShortSizes: double(100), Order: Poly(2), Class: ClassExp,
		},
		// A guard that reads only the variable keeps its values in a set bounded by a constant.
		{
			Name: "Counter guarded before a call after consumed input", Family: "bounded binding",
			Source: "var v int\nrule main = (r / .)* $$\nrule r = \"#\" {v < 3} {v := v + 1} r / \"y\" {v >= 0}\n",
			Input:  func(n int) string { return rep("#", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		// The values that reach a binding are those after the earlier items of its sequence. Here
		// they are the constants of the choice before it and the flags that the recursion clears
		// and sets; a closure over every value found, regardless of order, would add 2 without end.
		{
			Name: "Context flags set after a choice of values", Family: "bounded binding",
			Source: "var ctx int\nrule main = (\"0\" {ctx := 64} / {ctx := 0}) (\"1\" {ctx := ctx + 2})? (r / .)* $$\nrule r = g:\"*\"? \"#\" {ctx := ctx - ctx % 4 + len($g) * 2} r / \"(\" {ctx := ctx - ctx / 4 % 2 * 4 + 4} r / \"y\" {ctx / 4 % 2 == 1}\n",
			Input:  func(n int) string { return "01" + rep("*#(", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		// A guard after the call does not filter the values that the binding before the call reads,
		// which grow with the depth of the recursion.
		{
			Name: "Counter guarded after a call after consumed input", Family: "bounded binding",
			Source: "var v int\nrule main = (r / .)* $$\nrule r = \"#\" {v := v + 1} r {v < 3} / \"y\" {v >= 0}\n",
			Input:  func(n int) string { return rep("#", n) },
			Sizes:  double(100), ShortSizes: double(100), Order: Poly(2), Class: ClassExp,
		},
		// A plain group of a sequence splices its elements into the tuple around it: len($x) is 3
		// here, so the binding adds 1 on every call, while dividing by 4 adds nothing.
		{
			Name: "Counter adding the length of a spliced tuple", Family: "bounded binding",
			Source: "var v int\nrule main = (r / .)* $$\nrule r = x:(\"#\" (\"a\" \"b\")) {v := v + len($x) / 3} r / \"y\" {v >= 0}\n",
			Input:  func(n int) string { return rep("#ab", n) },
			Sizes:  double(100), ShortSizes: double(100), Order: Poly(2), Class: ClassExp,
		},
		{
			Name: "Counter adding a spliced tuple length below its step", Family: "bounded binding",
			Source: "var v int\nrule main = (r / .)* $$\nrule r = x:(\"#\" (\"a\" \"b\")) {v := v + len($x) / 4} r / \"y\" {v >= 0}\n",
			Input:  func(n int) string { return rep("#ab", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		// The product of the numbers of values of the variables with finite values is the bound on
		// K that the class reports: seven flag bits, seven bound Booleans beside one never bound, a
		// counter with 130 values, and two with 12 each, are exempt.
		{
			Name: "Seven flag bits", Family: "bounded binding",
			Source: "var ctx int\nrule main = (r / .)* $$\nrule r = \"a\" {ctx := ctx - ctx / 1 % 2 * 1 + 1} r / \"b\" {ctx := ctx - ctx / 2 % 2 * 2 + 2} r / \"c\" {ctx := ctx - ctx / 4 % 2 * 4 + 4} r / \"d\" {ctx := ctx - ctx / 8 % 2 * 8 + 8} r / \"e\" {ctx := ctx - ctx / 16 % 2 * 16 + 16} r / \"f\" {ctx := ctx - ctx / 32 % 2 * 32 + 32} r / \"g\" {ctx := ctx - ctx / 64 % 2 * 64 + 64} r / \"y\" {ctx >= 0}\n",
			Input:  func(n int) string { return rep("abcdefg", n) },
			Sizes:  double(100), ShortSizes: double(50), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Seven bound Booleans and an unbound one", Family: "bounded binding",
			Source: "var b1 bool\nvar b2 bool\nvar b3 bool\nvar b4 bool\nvar b5 bool\nvar b6 bool\nvar b7 bool\nvar b8 bool\nrule main = (r / .)* $$\nrule r = \"a\" {b1 := true} r / \"b\" {b2 := true} r / \"c\" {b3 := true} r / \"d\" {b4 := true} r / \"e\" {b5 := true} r / \"f\" {b6 := true} r / \"g\" {b7 := true} r / \"y\" {b1 || b2 || b3 || b4 || b5 || b6 || b7 || b8}\n",
			Input:  func(n int) string { return rep("abcdefg", n) },
			Sizes:  double(100), ShortSizes: double(50), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Counter with 130 values", Family: "bounded binding",
			Source: "var v int\nrule main = (r / .)* $$\nrule r = \"#\" {v < 129} {v := v + 1} r / \"y\" {v >= 0}\n",
			Input:  func(n int) string { return rep("##y", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Two counters with 144 combinations", Family: "bounded binding",
			Source: "var u int\nvar v int\nrule main = (r / .)* $$\nrule r = \"#\" {u < 11} {u := u + 1} r / \"(\" {v < 11} {v := v + 1} r / \"y\" {u + v >= 0}\n",
			Input:  func(n int) string { return rep("#(", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		// A test right after a binding bounds the values that the binding leaves bound.
		{
			Name: "Counter guarded after its binding", Family: "bounded binding",
			Source: "var v int\nrule main = (r / .)* $$\nrule r = \"#\" {v := v + 1} {v <= 129} r / \"y\" {v >= 0}\n",
			Input:  func(n int) string { return rep("##y", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		// A lookahead of a later alternative is evaluated only by that alternative: a scan in it,
		// even of a rule that calls no rule, runs once at each level of the recursion.
		{
			Name: "Scanning lookahead of a later alternative", Family: "rescanned calls",
			Source: "rule main = (e / \"a\")*\nrule e = \"a\" e \"x\" / !s \"b\" / \"c\"\nrule s = \"a\" [a-z]* \"!\"\n",
			Input:  func(n int) string { return rep("a", n) + "c" },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		{
			Name: "Scanning first alternative of a later lookahead", Family: "rescanned calls",
			Source: "rule main = (e / \"a\")*\nrule e = \"a\" e \"x\" / !(s / \"a\") \"b\" / \"c\"\nrule s = \"a\" [a-z]* \"!\"\n",
			Input:  func(n int) string { return rep("a", n) + "c" },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		// A repetition whose alphabet cannot extend what stands before it starts at one place of
		// each run of its alphabet, and a constant number of evaluations start there: alternatives
		// that scan the same input cost a constant number of passes, however they fail.
		{
			Name: "Number tokens over one digit scan", Family: "bounded scans",
			Source: "rule main = (n ws)* $$\nrule n = double / int\nrule double = [0-9]+ \".\" [0-9]+\nrule int = [0-9]+\nrule ws = [ ]*\n",
			Input:  func(n int) string { return rep("1", n) },
			Sizes:  double(1000), ShortSizes: double(250), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "String tokens over one quoted scan", Family: "bounded scans",
			Source: "rule main = (s ws)* $$\nrule s = raw / str\nrule raw = \"\\\"\" [^\"\\n]* \"\\\"\" \"r\"\nrule str = \"\\\"\" ([^\"\\\\]+ / \"\\\\\" .)* \"\\\"\"\nrule ws = [ ]*\n",
			Input:  func(n int) string { return "\"" + rep("a", n) + "\" " + rep("\"a\" ", n) },
			Sizes:  double(1000), ShortSizes: double(250), Order: Linear, Class: ClassLinear, Positive: true,
		},
		{
			Name: "Whitespace before two tokens", Family: "bounded scans",
			Source: "rule main = (a / b)* $$\nrule a = [ ]* \"x\"\nrule b = [ ]* \"y\"\n",
			Input:  func(n int) string { return rep(rep(" ", 40)+"y", n) },
			Sizes:  double(100), ShortSizes: double(25), Order: Linear, Class: ClassLinear, Positive: true,
		},
		// The bound needs both halves. Here each start is bounded inside a run of spaces, but a
		// call that is not memoized runs once for each level of the recursion that ends there.
		{
			Name: "Scan at the end of a nest", Family: "bounded scans",
			Source: "rule main = e sp $$\nrule e = \"a\" e (sp \"!\")? / \"b\"\nrule sp = [ ]*\n",
			Input:  func(n int) string { return rep("a", n) + "b" + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear, Positive: true,
		},
		// The opening delimiter is in the alphabet of the body, so a run of openers holds a start at
		// each of its positions.
		{
			Name: "Scan whose alphabet holds its opener", Family: "bounded scans",
			Source: "rule main = t* $$\nrule t = \"<\" [^>]* \">\" / \"<\"\n",
			Input:  func(n int) string { return rep("<", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		// A memo bounds the evaluations at one position, not the sum over the positions of a run:
		// the alternative "." starts the scan at every digit, and each start scans to the end.
		{
			Name: "Memoized scan that each position starts", Family: "bounded scans",
			Source: "rule main = (t \"y\" / t \"z\" / .)* $$\nrule t = d \"x\"\nrule d = [0-9]+\n",
			Input:  func(n int) string { return rep("1", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		// A choice that fails after a cut tries no later alternative, so a repetition of it can stop
		// where the next character is one that the later alternative takes: the run is not cleared,
		// and s starts at every position of it.
		{
			Name: "Scan after a repetition that a cut stops", Family: "bounded scans",
			Source: "rule main = (t (s \"z\")? \"a\")* \"aa\"? $$\nrule t = (\"aa\" #cut $$ / \"a\")*\nrule s = [a]+\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		{
			Name: "Scan after a lookahead that a cut passes", Family: "bounded scans",
			Source: "rule main = (!(\"aa\" #cut $$ / \"a\") (s \"z\")? \"a\")* \"aa\"? $$\nrule s = [a]+\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		{
			Name: "Scan after a repetition of a rule with a cut", Family: "bounded scans",
			Source: "rule main = (t (s \"z\")? \"a\")* \"aa\"? $$\nrule t = c*\nrule c = \"aa\" #cut $$ / \"a\"\nrule s = [a]+\n",
			Input:  func(n int) string { return rep("a", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		// The call of t inside the growth of r reads the failing seed, so t* stops at once, whatever
		// its rule would consume, and u scans from every position of the run.
		{
			Name: "Scan after a repetition of a left call", Family: "bounded scans",
			Source: "rule main = (r / \"b\")* $$\nrule r = t* u / \"b\"\nrule t = r\nrule u = s \"x\"\nrule s = [b]*\n",
			Input:  func(n int) string { return rep("b", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		// A step of the growth of S after the seed "bc" matches "a", and the last alternative of T
		// scans the rest of the input and fails at its end. The step is dropped, the parser goes on
		// at the end of the seed, and the next growth, four characters on, scans the rest again.
		{
			Name: "Dropped growth step that scans", Family: "growth steps",
			Source: "rule main = (S / [ac-z])* $$\nrule S = (S \"a\" / \"b\") T\nrule T = \"c\" / \"d\" [a-z]* \"!\"\n",
			Input:  func(n int) string { return rep("bcad", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassUnknown,
		},
		// Without the scan in T no step can fail after a scanning source.
		{
			Name: "Dropped growth step without a scan", Family: "growth steps", Positive: true,
			Source: "rule main = (S / [ac-z])* $$\nrule S = (S \"a\" / \"b\") T\nrule T = \"c\" / \"d\" \"!\"\n",
			Input:  func(n int) string { return rep("bcad", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		// A recovery ends after its skip, which reaches sp from every position of a run of letters.
		{
			Name: "Scan behind a recovery with many starts", Family: "bounded scans",
			Source: "rule main = ((\"q\" #recover(skip=[ab]+)) sp \"!\" / [ab])* sp $$\nrule sp = [ ]*\n",
			Input:  func(n int) string { return rep("a", n) + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		// An Operators Rule applies any number of prefix operators, so it reaches sp from every
		// position of a run of them. The records of scan sites lower its steps 2.9 times at n=1000 (812,032 to
		// 282,880) but it stays quadratic, which shows from 1000 on, so it is measured at the larger sizes.
		// The same grammar without sp costs as many steps (1,019,030 against 1,166,914 at n=1000 in
		// a traced parse with a hot span of 64), so the growth is the prefix loop of e, which runs from each of the n
		// starts to the operand: an Operators Rule is no run site.
		{
			Name: "Scan behind an Operators Rule with many starts", Family: "bounded scans",
			Source: "rule main = (e sp \"!\" / [-x])* sp $$\nrule e = #operators {\n    operand \"x\"\n    level neg { prefix \"-\" }\n}\nrule sp = [ ]*\n",
			Input:  func(n int) string { return rep("-", n) + "x" + rep(" ", n) },
			Sizes:  double(1000), ShortSizes: double(500), Order: Poly(2), Class: ClassPoly,
		},
		// The run before sp is empty, but [ab]+ reaches it from every position of a run of letters.
		{
			Name: "Scan behind a prefix with many starts", Family: "bounded scans",
			Source: "rule main = (w sp \"!\" / [ab])* sp $$\nrule w = [ab]+\nrule sp = [ ]*\n",
			Input:  func(n int) string { return rep("a", n) + rep(" ", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
		// The blanks hold comments, so the run before them has no bound, but nothing outside them
		// consumes a blank or a slash: no evaluation outside them starts inside their match, and the
		// lookahead that fails after them at each level of the nest starts them again only where
		// they started (the region bound).
		{
			Name: "Comments in blanks before a failed lookahead", Family: "region bounds", Positive: true,
			Source: regionGrammar,
			Input: func(n int) string {
				return rep("( /*"+rep("x", 8)+"*/ ", n/8) + "true /*" + rep("x", n) + "*/ . y" + rep(" )", n/8) + ";"
			},
			Sizes: double(250), ShortSizes: double(100), Order: Linear, Class: ClassLinear,
		},
		// A token where the blanks ended consumes the opener of a comment that is not closed, which
		// is no match of the blanks, and the scan of the next comment runs to the end again. The
		// blanks keep the region bound, and the scan inside the comment stays a scanning source. The
		// engine measures linear through the records of run sites, which generated parsers do not
		// keep: there it is quadratic.
		{
			Name: "Comments that are not closed after tokens", Family: "region bounds",
			Source: "rule main = (ws t)* ws $$\nrule t = \"/\" \"*\" [a-z]+\nrule ws = (\" \" / c)*\nrule c = \"/*\" (!\"*/\" .)* \"*/\"\n",
			Input:  func(n int) string { return rep(" /*b", n) },
			Sizes:  double(250), ShortSizes: double(100), Order: Linear, Class: ClassPoly,
		},
		// The opener of a comment is a token, so an evaluation of the blanks starts inside the match
		// of another at each opener.
		{
			Name: "Comments whose opener is a token", Family: "region bounds",
			Source: "rule main = e $$\nrule e = \"<\" (ws \"!\" / \"\") e / \">\" ws\nrule ws = [ ]* (c [ ]*)*\nrule c = \"<\" [^>]* \">\"\n",
			Input:  func(n int) string { return rep("<", n) + ">" },
			Sizes:  double(250), ShortSizes: double(100), Order: Poly(2), Class: ClassPoly,
		},
	}
	return append(cs, rewriteControls()...)
}
