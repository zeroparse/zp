package compile

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/internal/syntax"
)

// complexityMemoSources are grammars with the shapes that decide memoization: left-recursion
// cycles with leaders and other members, transient rules, rules called once, #missing in a rule
// and in a callee, and inline actions, which are lowered to hidden rules.
var complexityMemoSources = []string{
	"rule main = e $$\nrule e = e \"+\" t / t\nrule t = t \"*\" f / f\nrule f = \"x\" / \"(\" e \")\"\n",
	"rule main = c $$\nrule a = c \"+\" / \"n\"\nrule c = \"(\" c \")\" \"y\" / \"(\" c \")\" \"z\" / a\n",
	"rule main = x y y z\nrule x = \"x\"\nrule y = \"y\" x\nrule z = (\"z\" #missing) / w\nrule w = z \"w\" / \"w\"\n",
	"rule main = p p r r\nrule p = q \"p\"\nrule q = \"q\" #missing\nrule r = (\";\" #missing) p\n",
	"rule main = a:(\"a\" => 1) b:(\"b\" => 2) => $a + $b\n",
	"rule main = (e \"q\" / \"x\")* $$\nrule e = #operators {\n    operand \"x\"\n    level add { infix left \"+\" }\n}\n",
}

// TestComplexityMemoPolicy checks that the rules the classifier assumes the memo answers are the
// rules that every parse memoizes: a leader, or a rule that is memoized, not transient and not
// subject to missing tokens (Parser.Memoizes under every option). A member of a left-recursion
// cycle is memoized only outside the growths it reads, so the classifier counts it as not
// memoized.
func TestComplexityMemoPolicy(t *testing.T) {
	srcs := map[string]string{}
	for i, src := range complexityMemoSources {
		srcs[string(rune('a'+i))] = src
	}
	for _, pattern := range []string{"../../testdata/grammars/*/*.zp", "../../examples/*/*.zp"} {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			srcs[p] = string(b)
		}
	}
	for name, src := range srcs {
		t.Run(name, func(t *testing.T) {
			g, err := syntax.Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			g = starttest.MarkAll(g)
			prog, err := compileStatic(g, Options{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			c, err := ClassifyComplexity(g, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range prog.RuleList {
				always := r.Leader || r.Memo && !r.Transient && !r.Missing && !r.Member
				if c.Memoized[r.Name] != always {
					t.Errorf("rule %s: classifier assumes memoized=%v, compiled rule memoizes in every parse=%v (Leader %v Member %v Memo %v Transient %v Missing %v)",
						r.Name, c.Memoized[r.Name], always, r.Leader, r.Member, r.Memo, r.Transient, r.Missing)
				}
			}
			for n := range c.Memoized {
				if prog.ByName[n] == nil && !strings.HasPrefix(n, "_") {
					t.Errorf("classifier rule %s is not a compiled rule", n)
				}
			}
		})
	}
}

func classifySource(t *testing.T, src string) *Complexity {
	t.Helper()
	g, err := syntax.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	c, err := ClassifyComplexity(starttest.MarkAll(g), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestClassifyComplexity checks the class of grammars that each exercise one part of the
// conditions for linear cost.
func TestClassifyComplexity(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		class     ComplexityClass
		degree    int
	}{
		{"committing alternative", "rule main = (a / \"a\")* $$\nrule a = \"a\" \"c\"\n", ClassLinear, 0},
		{"restarted scan", "rule main = (a / \"a\")* $$\nrule a = \"a\"* \"c\"\n", ClassPoly, 2},
		{"nested restarted scans", "rule main = (a / \"a\")* $$\nrule a = (b / \"a\")* \"c\"\nrule b = \"a\"* \"d\"\n", ClassPoly, 3},
		{"left call after an option that consumed input", "rule main = r $$\nrule r = [ab]? (r / [a-c]) (w [ ] [b])\nrule w = [^a]* $$\n", ClassUnknown, 0},
		{"left call after an option, behind a call", "rule main = r $$\nrule r = [ab]? f (w [ ] [b])\nrule f = r / [a-c]\nrule w = [^a]* $$\n", ClassUnknown, 0},
		{"left call after an option rule, behind two calls", "rule main = r $$\nrule r = p f (w [ ] [b])\nrule p = [ab]?\nrule f = g / [a-c]\nrule g = r\nrule w = [^a]* $$\n", ClassUnknown, 0},
		{"restarted scan in a repetition of each growth step", "rule main = r* $$\nrule r = (r [ab] s . . .)*\nrule s = ([a]* \"ab\"+ \" \"*)?\n", ClassPoly, 3},
		{"restarted scan in a rule that the repetition calls indirectly", "rule main = r* $$\nrule r = (r [ab] m)*\nrule m = s . . .\nrule s = ([a]* \"ab\"+ \" \"*)?\n", ClassPoly, 3},
		// The iteration fails only at "\"\"", before its scan, so it need not be committing.
		{"unit failing before its source", "rule main = (s \";\")* $$\nrule s = \"\\\"\" [^\"]* (\"\\\"\\\"\" [^\"]*)* \"\\\"\"\n", ClassLinear, 0},
		{"unit failing after its source", "rule main = (s \";\")* $$\nrule s = \"\\\"\" [^\"]* (\"\\\"\" [^\"]* \"!\")* \"\\\"\"\n", ClassPoly, 2},
		// The operands overlap, but their only source is the shared skip ws.
		// A later growing alternative that succeeds is followed by the next step, which scans
		// again from the end of its result what the failed growing unit examined, so the shared
		// call sp does not make the unit committing.
		{"growing unit restarted by the next step", "rule main = a $$\nrule a = a sp [^!]* \"!\" / a sp / \"n\"\nrule sp = \"+\"\n", ClassUnknown, 0},
		{"growing unit restarted by the next step through a member", "rule main = a $$\nrule a = c sp [^!]* \"!\" / c sp / \"n\"\nrule c = a\nrule sp = \"+\"\n", ClassUnknown, 0},
		// The base and the skip share the call b, and what follows the skip, the next iteration,
		// examines again the input that [ab]* examined, so the base is not committing.
		{"recovery whose skip shares a call with its base", "rule main = ((b [ab]* \"!\") #recover(skip=b))* $$\nrule b = \"b\"\n", ClassPoly, 2},
		{"recovery of the last element only", "rule main = (b [ab]* \"!\" #recover(skip=b))* $$\nrule b = \"b\"\n", ClassLinear, 0},
		// A capture in a lookahead holds the rest of the input at every position, so a copy of it
		// repeats over that input; its text alone costs a constant.
		{"copy of a capture in a lookahead", "rule main = a* $$\nrule a string = \"a\" &(x:w) => text($x) + \"!\"\nrule w = \"a\" w / \"\"\n", ClassUnknown, 0},
		{"text of a capture in a lookahead", "rule main = a* $$\nrule a string = \"a\" &(x:w) => text($x)\nrule w = \"a\" w / \"\"\n", ClassLinear, 0},
		// A rule that consumes nothing can return such a capture, as a node or as its text, and
		// a list can hold it in each element.
		{"copy of a lookahead capture that a rule returns", "rule main = a* $$\nrule a string = \"a\" y:look => text($y) + \"!\"\nrule look = &(x:w) => $x\nrule w = \"a\" w / \"\"\n", ClassUnknown, 0},
		{"copy of the text of a lookahead capture that a rule returns", "rule main = a* $$\nrule a string = \"a\" y:look => \"!\" + $y\nrule look string = &(x:w) => text($x)\nrule w = \"a\" w / \"\"\n", ClassUnknown, 0},
		{"copies of lookahead captures in the elements of a map", "rule main = b $$\nrule b []string = ps:p* => map($ps, (e) => text(e.$y) + \"!\")\nrule p = \"a\" y:look\nrule look = &(x:w) => $x\nrule w = \"a\" w / \"\"\n", ClassUnknown, 0},
		{"copies of lookahead captures in the steps of a fold", "rule main = b $$\nrule b string = ps:p* => foldl(\"\", $ps, (s, e) => text(e.$y) + \"!\")\nrule p = \"a\" y:look\nrule look = &(x:w) => $x\nrule w = \"a\" w / \"\"\n", ClassUnknown, 0},
		{"texts of lookahead captures in the elements of a map", "rule main = b $$\nrule b []string = ps:p* => map($ps, (e) => text(e.$y))\nrule p = \"a\" y:look\nrule look = &(x:w) => $x\nrule w = \"a\" w / \"\"\n", ClassLinear, 0},
		// A member of a lookahead capture, or a field of a struct built from one, holds the
		// same input.
		{"copy of a member of a lookahead capture that a rule returns", "rule main = a* $$\nrule a string = \"a\" y:look => text($y) + \"!\"\nrule look = &(x:p) => $x.$q\nrule p = q:w\nrule w = \"a\" w / \"\"\n", ClassUnknown, 0},
		{"copy of a member of a member of a lookahead capture", "rule main = a* $$\nrule a string = \"a\" y:look => text($y.$x.$q) + \"!\"\nrule look = &(x:p)\nrule p = q:w\nrule w = \"a\" w / \"\"\n", ClassUnknown, 0},
		{"copy of a struct field that holds a lookahead capture", "type S = { F string }\nrule main = a* $$\nrule a string = \"a\" y:look => $y.F + \"!\"\nrule look S = &(x:w) => S{F: text($x)}\nrule w = \"a\" w / \"\"\n", ClassUnknown, 0},
		{"copy of a bounded lookahead capture", "rule main = a* \"b\" $$\nrule a string = \"a\" y:look => text($y) + \"!\"\nrule look = &(x:\"a\" / \"b\") => $x\n", ClassLinear, 0},
		{"source inside a shared call", "rule main = e $$\nrule e = #operators {\n    skip ws\n    operand \"true\"\n    operand [a-z]+\n    level { infix left \"+\" }\n}\nrule ws = [ \\t]*\n", ClassLinear, 0},
		// A list built from the elements of a repetition costs its number of elements.
		{"list of a repetition", "rule main = l $$\nrule l = \"[\" ms:m? \"]\" => concat($ms)\nrule m = f:v rs:(\",\" x:v)* => concat(list($f), map($rs, (r) => r.$x))\nrule v = \"x\" / l\n", ClassLinear, 0},
		{"list built by right recursion", "rule main = l $$\nrule l = x:\"a\" t:l? => concat(list($x), $t)\n", ClassPoly, 2},
		// The rule that starts the recursion of r is walked before m, whose captures decide that
		// the value of r includes the value of its own recursion.
		{"list built by right recursion after its user", "rule main = s $$\nrule s []string = \"\\\"\" t:r => concat(list(\"q\"), $t)\nrule r []string = m / l\nrule m []string = e:i \",\" t:r => concat(list($e), $t)\nrule l []string = e:i \"\\\"\" => list($e)\nrule i string = x:(\"(\" s \")\" / [a-z]+) => text($x)\n", ClassPoly, 2},
		{"tail passed on by right recursion after its user", "rule main = s $$\nrule s []string = \"\\\"\" t:r => $t\nrule r []string = m / l\nrule m []string = i \",\" t:r => $t\nrule l []string = e:i \"\\\"\" => list($e)\nrule i string = x:(\"(\" s \")\" / [a-z]+) => text($x)\n", ClassLinear, 0},
		// A lambda over a list with at most a constant number of elements runs at most that many
		// times: its copies from outside it and of a fold accumulator are a constant number of
		// copies. Over a list that is not bounded, or inside a lambda over one, they are not, and
		// neither are those of a fold whose accumulator doubles, by a factor exponential in the
		// bound.
		{"fold over a bounded list", "rule main = a:\"x\"* b:\"y\" #repeat(,2) => foldl(text($a), $b, (acc, y) => acc + text($a))\n", ClassLinear, 0},
		{"doubling fold over a bounded list", "rule main = a:\"x\"* b:\"y\" #repeat(,2) => foldl(text($a), $b, (acc, y) => acc + acc + text($a))\n", ClassExp, 0},
		{"fold over a bounded list that doubles an element", "rule main = a:\"x\"* b:\"y\" #repeat(,2) => foldl(list(text($a)), $b, (acc, y) => concat(list(acc[0] + acc[0]), acc))\n", ClassExp, 0},
		{"fold over a bounded list that builds a doubled element", "rule main = a:\"x\"* b:\"y\" #repeat(,2) => foldl(list(text($a)), $b, (acc, y) => list(acc[0] + acc[0]))\n", ClassExp, 0},
		{"fold over a bounded list that doubles each element", "rule main = a:\"x\"* b:\"y\" #repeat(,2) => foldl(list(text($a)), $b, (acc, y) => map(acc, (e) => e + e))\n", ClassExp, 0},
		{"fold over a bounded list that extends each element", "rule main = a:\"x\"* b:\"y\" #repeat(,2) => foldl(list(text($a)), $b, (acc, y) => map(acc, (e) => e + text(y)))\n", ClassLinear, 0},
		// Duplicating the elements of the accumulator and joining them doubles it on each step,
		// though no copy reads it twice; joining each element once does not.
		{"fold that duplicates and joins elements", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => foldl(list(), map(acc, (e) => list(e, e)), (s, l) => concat(s, l)))\n", ClassExp, 0},
		{"bounded fold that duplicates and joins elements", "rule main = a:\"x\"* b:\"y\" #repeat(,2) => foldl(list(text($a)), $b, (acc, y) => foldl(list(), map(acc, (e) => list(e, e)), (s, l) => concat(s, l)))\n", ClassExp, 0},
		{"fold that joins each element twice", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => foldl(list(), acc, (s, l) => concat(s, list(l, l))))\n", ClassExp, 0},
		// Joining each element once copies the accumulator inside the inner fold on each of its
		// steps, as many as the accumulator has elements: one here, as in the initial value, so
		// each of n steps copies n.
		{"fold that joins elements once", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => foldl(list(), map(acc, (e) => list(e)), (s, l) => concat(s, l)))\n", ClassPoly, 2},
		// From n elements, the inner fold copies n on each of n steps, n^3 in all; the bound on the
		// accumulator adds its number of elements on each step, so it overstates its size as n^2.
		{"fold that joins elements once from a list of the input", "rule main = a:\"x\"* b:\"y\"* => foldl(map($a, (z) => text(z)), $b, (acc, y) => foldl(list(), map(acc, (e) => list(e)), (s, l) => concat(s, l)))\n", ClassPoly, 4},
		// The size, the largest element and the number of elements of an accumulator bound those
		// of the value of the lambda: a fold whose lambda keeps one element twice does not grow;
		// one that reads the accumulator in a lambda over another list, or that pads every element
		// and joins them, can double it.
		{"fold that keeps one element twice", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => foldl(list(), acc, (s, l) => list(l, l)))\n", ClassLinear, 0},
		{"fold that keeps one element in two lists", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => foldl(list(), acc, (s, l) => concat(list(l), list(l))))\n", ClassLinear, 0},
		{"inner fold that adds the accumulator on each step", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => foldl(list(), $b, (s, l) => concat(s, acc)))\n", ClassExp, 0},
		{"map that copies the accumulator for each element", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => foldl(list(), map($b, (z) => acc), (s, l) => concat(s, l)))\n", ClassExp, 0},
		{"fold that duplicates the parts of its first element", "rule main = a:\"x\"* b:\"y\"* => foldl(list(list(text($a))), $b, (acc, y) => list(foldl(list(), map(acc[0], (e) => list(e, e)), (s, l) => concat(s, l))))\n", ClassExp, 0},
		{"fold that pads and joins elements", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => foldl(list(), map(acc, (e) => list(e, \"x\")), (s, l) => concat(s, l)))\n", ClassExp, 0},
		// A map that adds to every element adds to the size once for each element, which the
		// initial value can make n, so the size grows quadratically; so does appending a copy of
		// the first element on each step.
		{"fold that extends each element", "rule main = a:\"x\"* b:\"y\"* => foldl(map($a, (z) => text(z)), $b, (acc, y) => map(acc, (e) => e + \"x\"))\n", ClassPoly, 3},
		// A value read from outside the fold can add up to the input on every step, and a part of it
		// for each element multiplies the number of elements.
		{"fold that adds an outside value on each step", "rule main = a:\"x\"* b:\"y\"* c:\"z\"* => foldl(text($a), $b, (acc, y) => acc + text($c))\n", ClassPoly, 3},
		{"fold that appends an outside list on each step", "rule main = a:\"x\"* b:\"y\"* c:\"z\"* => foldl(list(text($a)), $b, (acc, y) => concat(acc, map($c, (z) => text(z))))\n", ClassPoly, 3},
		{"fold that adds an outside value to every element", "rule main = a:\"x\"* b:\"y\"* c:\"z\"* => foldl(map($a, (z) => text(z)), $b, (acc, y) => map(acc, (e) => e + text($c)))\n", ClassPoly, 4},
		{"fold that extends every element and appends an outside list", "rule main = a:\"x\"* b:\"y\"* c:\"z\"* => foldl(map($a, (z) => text(z)), $b, (acc, y) => concat(map(acc, (e) => e + \"x\"), map($c, (z) => text(z))))\n", ClassPoly, 4},
		// An index is an int and adds nothing to the element it takes; a map that keeps every element
		// rebuilds the list without growing it.
		{"fold that appends its last element", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => concat(acc, list(acc[len(acc) - 1])))\n", ClassPoly, 2},
		{"fold that maps its accumulator unchanged", "rule main = a:\"x\"* b:\"y\"* => foldl(map($a, (z) => text(z)), $b, (acc, y) => map(acc, (e) => e))\n", ClassPoly, 2},
		// A fold inside the step copies its own accumulator once for each element of its list.
		{"fold that rebuilds its accumulator by an inner fold", "rule main = a:\"x\"* b:\"y\"* => foldl(map($a, (z) => text(z)), $b, (acc, y) => foldl(list(), acc, (s, l) => concat(s, list(l))))\n", ClassPoly, 3},
		{"fold that joins its accumulator in a predicate", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => concat(acc, filter(list(\"x\"), (z) => len(foldl(\"\", acc, (s, l) => s + l)) > 0)))\n", ClassPoly, 3},
		// A comparison of values that are not ints, and len of a string, examine their operands;
		// len of a list does not. The accumulator holds n elements, the first of n characters.
		// Comparing every element with the first is classed by the sizes of both operands, which
		// over-approximates: == stops at the shorter one, so it measures about n^2.
		{"fold that compares every element with its first", "rule main = a:\"x\"* b:\"y\"* => foldl(concat(list(text($a)), map($a, (z) => text(z))), $b, (acc, y) => filter(acc, (e) => e == acc[0]))\n", ClassPoly, 3},
		{"fold that measures its first element for every element", "rule main = a:\"x\"* b:\"y\"* => foldl(concat(list(text($a)), map($a, (z) => text(z))), $b, (acc, y) => filter(acc, (e) => len(acc[0]) > 0))\n", ClassPoly, 3},
		{"fold that measures every element", "rule main = a:\"x\"* b:\"y\"* => foldl(concat(list(text($a)), map($a, (z) => text(z))), $b, (acc, y) => filter(acc, (e) => len(e) > 0))\n", ClassPoly, 2},
		{"fold that measures its length for every element", "rule main = a:\"x\"* b:\"y\"* => foldl(concat(list(text($a)), map($a, (z) => text(z))), $b, (acc, y) => filter(acc, (e) => len(acc) > 0))\n", ClassPoly, 2},
		// A fold that copies its accumulator runs once for each evaluation of the lambdas around it
		// over lists that are not bounded, unless it runs over an element of the innermost one,
		// whose sizes add up to the input.
		{"fold inside a map over the same list", "rule main = b:\"y\"* => map($b, (y) => foldl(\"\", $b, (s, l) => s + text(l)))\n", ClassPoly, 3},
		{"fold inside a filter over the same list", "rule main = b:\"y\"* => filter($b, (y) => len(foldl(\"\", $b, (s, l) => s + text(l))) > 0)\n", ClassPoly, 3},
		{"fold inside two maps", "rule main = b:\"y\"* => map($b, (y) => map($b, (z) => foldl(\"\", $b, (s, l) => s + text(l))))\n", ClassPoly, 4},
		// Elements whose values hold more than the input they consume, through a capture in a
		// lookahead or an action, need not add up to the input.
		{"fold over a capture in a lookahead of the elements of a map", "rule main = items:item* => map($items, (it) => foldl(\"\", it.$c, (s, l) => s + text(l)))\nrule item = &(c:\"x\"*) \"x\"\n", ClassPoly, 3},
		{"fold over elements built by an action", "rule main = items:item* => map($items, (it) => foldl(\"\", it, (s, l) => s + text(l)))\nrule item = &(c:\"x\"*) \"x\" => $c\n", ClassPoly, 3},
		{"fold over a capture in a lookahead inside a fold step", "rule main = items:item* => foldl(0, $items, (acc, it) => len(foldl(\"\", it.$c, (s, l) => s + text(l))))\nrule item = &(c:\"x\"*) \"x\"\n", ClassPoly, 3},
		{"fold over the captures of elements of a rule", "rule main = items:item* => map($items, (it) => foldl(\"\", it.$c, (s, l) => s + text(l)))\nrule item = c:\"x\"+\n", ClassPoly, 2},
		{"fold over the elements of a map over copies of a list", "rule main = b:\"y\"* => map(map($b, (q) => $b), (y) => foldl(\"\", y, (s, l) => s + text(l)))\n", ClassPoly, 3},
		{"fold adding an outside value inside a map", "rule main = a:\"x\"* b:\"y\"* => map($b, (z) => foldl(text($a), $b, (acc, y) => acc + text($a)))\n", ClassPoly, 4},
		{"fold that measures its first element for every element inside a map", "rule main = a:\"x\"* b:\"y\"* => map($b, (z) => foldl(concat(list(text($a)), map($a, (q) => text(q))), $b, (acc, y) => filter(acc, (e) => len(acc[0]) > 0)))\n", ClassPoly, 4},
		{"fold that appends its first element inside a map", "rule main = a:\"x\"* b:\"y\"* => map($b, (z) => foldl(list(text($a)), $b, (acc, y) => concat(list(acc[0]), acc)))\n", ClassPoly, 3},
		{"fold over the elements of a map", "rule main = b:(c:\"y\"*)* => map($b, (y) => foldl(\"\", y.$c, (s, l) => s + text(l)))\n", ClassPoly, 2},
		{"fold that appends its first element", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => concat(list(acc[0]), acc))\n", ClassPoly, 2},
		{"bounded fold that appends its first element", "rule main = a:\"x\"* b:\"y\" #repeat(,2) => foldl(list(text($a)), $b, (acc, y) => concat(list(acc[0]), acc))\n", ClassLinear, 0},
		{"fold that filters its accumulator into a copy", "rule main = a:\"x\"* b:\"y\"* => foldl(list(text($a)), $b, (acc, y) => concat(acc, filter(acc, (e) => e != \"\")))\n", ClassExp, 0},

		{"bounded fold that joins elements once", "rule main = a:\"x\"* b:\"y\" #repeat(,2) => foldl(list(text($a)), $b, (acc, y) => foldl(list(), map(acc, (e) => list(e)), (s, l) => concat(s, l)))\n", ClassPoly, 2},
		{"bounded elements inside a map", "rule main = zs:\"z\"* b:\"y\" #repeat(,2) => map($zs, (z) => foldl(\"\", $b, (acc, y) => acc + text(y)))\n", ClassLinear, 0},
		{"doubling fold over a list", "rule main = a:\"x\"* b:\"y\"* => foldl(text($a), $b, (acc, y) => acc + acc)\n", ClassExp, 0},
		{"bounded map copying a value from outside", "rule main = a:\"x\"* b:\"y\" #repeat(,2) => map($b, (y) => text($a) + text(y))\n", ClassLinear, 0},
		{"map copying a value from outside", "rule main = a:\"x\"* b:\"y\"* => map($b, (y) => text($a) + text(y))\n", ClassUnknown, 0},
		{"bounded fold inside a map", "rule main = a:\"x\"* zs:(b:\"y\" #repeat(,1) \"z\")* => map($zs, (z) => foldl(\"\", z.$b, (acc, y) => acc + text($a)))\n", ClassUnknown, 0},
		{"bounded fold over the elements of a map", "rule main = a:\"x\"* zs:(b:\"y\" #repeat(,1) \"z\")* => map($zs, (z) => foldl(\"\", z.$b, (acc, y) => acc + text(y)))\n", ClassLinear, 0},
		{"bounded fold that adds a recursive list on each step", "rule main = l $$\nrule l []string = x:\"a\" t:l? b:\"y\" #repeat(,2) => foldl(list(), $b, (acc, y) => concat(acc, $t))\n", ClassExp, 0},
		{"bounded fold of a list built by right recursion", "rule main = l $$\nrule l []string = x:\"a\" t:l? b:\"y\" #repeat(,1) => foldl($t, $b, (acc, y) => concat(acc, list(text(y))))\n", ClassPoly, 2},
		// A capture, a rule call or an initial value has the size degree that the classifier knows
		// for it, so a value larger than the input raises the degree of what reads it. A value
		// that is own, at most a constant times the input its match consumes, has degree 1, and
		// own elements add up to at most the input.
		// The control of the same name measures n^4 on every backend.
		{"fold adding a quadratic value of a rule", "rule main = c:sq b:\"y\"* => foldl(\"\", $b, (acc, y) => acc + $c)\nrule sq = a:\"x\"* => foldl(\"\", $a, (s, z) => s + text($a))\n", ClassPoly, 4},
		{"fold adding an own value of a rule", "rule main = c:sq b:\"y\"* => foldl(\"\", $b, (acc, y) => acc + $c)\nrule sq = a:\"x\"* => text($a)\n", ClassPoly, 3},
		{"fold adding a quadratic list of a rule", "rule main = c:sq b:\"y\"* => foldl(list(), $b, (acc, y) => concat(acc, $c))\nrule sq = a:\"x\"* => map($a, (z) => text($a))\n", ClassPoly, 3},
		// The control of the same name measures n^2, and the one of their own text linear.
		{"map copying elements that add up to more than the input", "rule main = b:\"y\"* => map(map($b, (q) => text($b)), (y) => y + \"x\")\n", ClassPoly, 2},
		{"map copying elements of their own text", "rule main = b:\"y\"* => map(map($b, (q) => text(q)), (y) => y + \"x\")\n", ClassLinear, 0},
		{"map copying elements of a quadratic rule value", "rule main = c:sq => map($c, (y) => y + \"x\")\nrule sq = a:\"x\"* => map($a, (z) => text($a))\n", ClassPoly, 2},
		{"map copying elements built by own actions", "rule main = items:item* => map($items, (it) => it + \"x\")\nrule item string = c:\"x\"+ \",\" => text($c)\n", ClassLinear, 0},
		{"map copying captures in a lookahead of its elements", "rule main = items:item* => map($items, (it) => map(it.$c, (y) => text(y) + \"x\"))\nrule item = &(c:\"x\"*) \"x\"\n", ClassPoly, 2},
		// A fold over a repetition that holds each element, and so each value of the recursion in
		// it, at most once builds an own value, as a left-associative expression does.
		{"fold adding an own value of a left-associative fold", "rule main = c:e b:\"y\"* => foldl(\"\", $b, (acc, y) => acc + $c)\nrule e string = l:a rest:(\"+\" r:a)* => foldl($l, $rest, (acc, i) => acc + \"+\" + i.$r)\nrule a string = (\"x\" => \"x\") / (\"(\" v:e \")\" => $v)\n", ClassPoly, 3},
		{"fold adding an own value of a left-associative fold over two members", "rule main = c:e b:\"y\"* => foldl(\"\", $b, (acc, y) => acc + $c)\nrule e string = l:a rest:(o:\"+\" r:a)* => foldl($l, $rest, (acc, i) => acc + text(i.$o) + i.$r)\nrule a string = (\"x\" => \"x\") / (\"(\" v:e \")\" => $v)\n", ClassPoly, 3},
		// The control of the same name measures n^3.
		{"fold from a quadratic initial value", "rule main = a:\"x\"* b:\"y\"* => foldl(map($a, (z) => text($a)), $b, (acc, y) => filter(acc, (e) => len(e) > 0))\n", ClassPoly, 3},
		{"fold from an own initial value", "rule main = a:\"x\"* b:\"y\"* => foldl(map($a, (z) => text(z)), $b, (acc, y) => filter(acc, (e) => len(e) > 0))\n", ClassPoly, 2},
		// A rule whose value includes its own recursion twice has no size degree that the
		// classifier knows, and one that includes it once is own.
		{"map over a value that doubles over a recursion", "rule main = c:l => map($c, (e) => e + \"x\")\nrule l = x:\"a\" t:l? => list($t, $t)\n", ClassUnknown, 0},
		{"fold adding the text of a capture of a value that doubles", "rule main = c:l b:\"y\"* => foldl(\"\", $b, (acc, y) => acc + text($c))\nrule l = x:\"a\" t:l? => list($t, $t)\n", ClassPoly, 3},
		{"fold adding a value of a right recursion", "rule main = c:l b:\"y\"* => foldl(list(), $b, (acc, y) => concat(acc, map($c, (e) => e)))\nrule l = x:\"a\" t:l? => list(text($x), $t)\n", ClassPoly, 3},
		// Comparing two strings examines the shorter one: against a literal it is bounded, between
		// two values of the recursion it is not.
		{"comparison with a literal", "rule main = l $$\nrule l string = (\"(\" x:l \")\" {$x != \"\"} => $x) / a\nrule a string = \"a\" => \"a\"\n", ClassLinear, 0},
		{"comparison of recursive values", "rule main = l $$\nrule l string = (\"(\" x:l y:l \")\" {$x == $y} => $x) / a\nrule a string = \"a\" => \"a\"\n", ClassPoly, 2},
		// The text of a capture is a range of the input and costs a constant, but joining it
		// copies it.
		{"text of a recursive capture", "rule main = a $$\nrule a = s:(\"(\" a \")\" / \"x\") => text($s)\n", ClassLinear, 0},
		{"join of the text of a recursive capture", "rule main = a $$\nrule a = s:(\"(\" a \")\" / \"x\") => text($s) + \"!\"\n", ClassPoly, 2},
		// An element of a list holds anything the list can, so indexing keeps the size.
		{"element of a recursive list", "rule main = r $$\nrule r []string = (\"a\" x:r => list($x[0] + \"a\")) / (\"\" => list(\"\"))\n", ClassPoly, 2},
		{"list of literal elements", "rule main = r $$\nrule r []string = (\"a\" x:r => list(\"a\")) / (\"\" => list(\"\"))\n", ClassLinear, 0},
		{"comparison of recursive elements", "rule main = l $$\nrule l []string = (\"(\" x:l y:l \")\" {$x[0] == $y[0]} => $x) / (t:[a-z]+ => list(text($t)))\n", ClassPoly, 2},
		// A comparison examines no more than one of its operands.
		{"comparison of a recursive element with itself", "rule main = l $$\nrule l []string = (\"(\" x:l \")\" {$x[0] == $x[0]} => $x) / (t:[a-z]+ => list(text($t)))\n", ClassPoly, 2},
		// len examines all of a string or a list.
		{"len of a failing unit", "rule main = (v \"q\" / \"x\")* $$\nrule v = x:e #atomic {len($x.Value) > 0}\nrule e = \"x\" e / \"x\"\n", ClassPoly, 2},
		{"len of a recursive capture", "rule main = r $$\nrule r = x:(\"a\" r / \"\") #atomic {len($x.Value) >= 0}\n", ClassPoly, 2},
		// The length of a node is its range, which len reads in constant time.
		{"len of the node of a failing unit", "rule main = (x:e #atomic {len($x) > 0} \"q\" / \"x\")* $$\nrule e = \"x\" e / \"x\"\n", ClassLinear, 0},
		{"len of the node of a recursive capture", "rule main = r $$\nrule r = x:(\"a\" r / \"\") #atomic {len($x) >= 0}\n", ClassLinear, 0},
		{"len of a list node of a recursive capture", "rule main = r $$\nrule r = x:(\"a\" r)? {len($x) >= 0}\n", ClassLinear, 0},
		{"nil test of a recursive capture", "rule main = r $$\nrule r = x:(\"a\" r / \"\") #atomic {$x != nil}\n", ClassLinear, 0},
		// A repetition ends at its first empty iteration, so it can fail after scanning even
		// when its expression always succeeds.
		{"repetition below its minimum", "rule main = (p / \"a\")* $$\nrule p = (\"a\"*) #repeat(3)\n", ClassPoly, 2},
		// With a minimum of two, "a"* fails only when its first iteration is empty, after a
		// constant examination; the classifier does not tell this from a body that can be empty
		// after scanning, as &"a"* can, and stays pessimistic.
		{"repetition of two below its minimum", "rule main = (p / \"a\")* $$\nrule p = (\"a\"*) #repeat(2)\n", ClassPoly, 2},
		{"repetition below its minimum without a retry", "rule main = (\"a\"*) #repeat(3) $$\n", ClassLinear, 0},
		{"self-growing variable", "var s string = \"a\"\nrule main = r $$\nrule r = \"x\" {s := s + s} r / {len(s) > 0}\n", ClassExp, 0},
		// Only a polynomial class has a degree.
		{"exponential with a polynomial cause", "var s string = \"a\"\nrule main = r (a / \"a\")* $$\nrule r = \"x\" {s := s + s} r / {len(s) > 0}\nrule a = \"a\"* \"c\"\n", ClassExp, 0},
		// A variable whose values lie in a set bounded by a constant adds a constant number of
		// combinations of values; the set of an int comes from its initial value and bindings that
		// read only it, literals and bounded lengths, applied to the values after the earlier
		// items of their sequences.
		{"counter before a call", "var v int\nrule main = (r / .)* $$\nrule r = \"#\" {v := v + 1} r / \"y\" {v >= 0}\n", ClassExp, 0},
		{"guarded counter before a call", "var v int\nrule main = (r / .)* $$\nrule r = \"#\" {v < 3} {v := v + 1} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"counter modulo a constant", "var v int = 1\nrule main = r $$\nrule r = \"#\" {v := (v * 3 + 1) % 7} r / \"y\" {v != 0}\n", ClassLinear, 0},
		{"counter guarded outside a repetition", "var v int\nrule main = r $$\nrule r = \"(\" {v < 3} (\"#\" {v := v + 1})* r \")\" / \"y\" {v >= 0}\n", ClassExp, 0},
		{"guard before another binding", "var v int\nrule main = r $$\nrule r = \"#\" {v < 3} {v := v + 1} {v := v + 1} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"counter that reads a capture", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {v < 3} {v := v + len($x)} r / \"y\" {v >= 0}\n", ClassExp, 0},
		{"counter that reads a bounded capture", "var v int\nrule main = r $$\nrule r = x:\"#\" {v < 3} {v := v + len($x)} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"counter that reads the bounded value of a recursive rule", "var v int\nrule main = r $$\nrule r = x:k \"#\" {v := v - v % 4 + len($x)} r / \"y\" {v >= 0}\nrule k = a:\"*\"? (\"[\" k \"]\")? => $a\n", ClassLinear, 0},
		// The values that reach a binding are those after the earlier items of its sequence: a
		// binding after a choice of constants reads only them, and a guard after the call does
		// not filter what the binding reads.
		{"flags set after a choice of values", "var ctx int\nrule main = (\"0\" {ctx := 64} / {ctx := 0}) (\"1\" {ctx := ctx + 2})? r $$\nrule r = g:\"*\"? \"#\" {ctx := ctx - ctx % 4 + len($g) * 2} r / \"(\" {ctx := ctx - ctx / 4 % 2 * 4 + 4} r \")\" / \"y\" {ctx / 4 % 2 == 1}\n", ClassLinear, 0},
		{"guard after the call", "var v int\nrule main = r $$\nrule r = \"#\" {v := v + 1} r {v < 3} / \"y\" {v >= 0}\n", ClassExp, 0},
		// A plain group of a sequence splices its elements into the tuple of the sequence around it.
		{"counter that reads the length of a spliced tuple", "var v int\nrule main = (r / .)* $$\nrule r = x:(\"#\" (\"a\" \"b\")) {v := v + len($x) / 3} r / \"y\" {v >= 0}\n", ClassExp, 0},
		{"counter that reads the length of a spliced tuple below its step", "var v int\nrule main = (r / .)* $$\nrule r = x:(\"#\" (\"a\" \"b\")) {v := v + len($x) / 4} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		// The values of all the variables with finite values combine, and the product is the bound
		// on K that the class reports (see TestComplexityCombinations). One variable has at most
		// 65536 values.
		{"counter with 128 values", "var v int\nrule main = r $$\nrule r = \"#\" {v < 127} {v := v + 1} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"counter with 129 values", "var v int\nrule main = r $$\nrule r = \"#\" {v < 128} {v := v + 1} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"two counters with 144 combinations", "var u int\nvar v int\nrule main = r $$\nrule r = \"#\" {u < 11} {u := u + 1} r / \"(\" {v < 11} {v := v + 1} r / \"y\" {u + v >= 0}\n", ClassLinear, 0},
		{"counter with 65536 values", "var v int\nrule main = r $$\nrule r = \"#\" {v < 65535} {v := v + 1} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"counter with 65537 values", "var v int\nrule main = r $$\nrule r = \"#\" {v < 65536} {v := v + 1} r / \"y\" {v >= 0}\n", ClassExp, 0},
		// A test right after a binding, with only predicates between them, rejects a value before
		// any call or later binding sees it, since the failure undoes the binding; so does a test
		// right before a binding of the expression that it tests.
		{"counter guarded after its binding", "var v int\nrule main = r $$\nrule r = \"#\" {v := v + 1} {v <= 3} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"counter guarded in the block of its binding", "var v int\nrule main = r $$\nrule r = \"#\" {v := v + 1; v <= 3} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"counter guarded after its binding with a label", "var v int\nrule main = r $$\nrule r = \"#\" {v := v + 1} {v <= 3} @error(\"too deep\") r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"counter guarded after a test of another variable", "var v int\nvar b bool\nrule main = r $$\nrule r = \"#\" {v := v + 1} {!b} {v <= 3} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"counter guarded by a test that is not a range", "var v int\nrule main = r $$\nrule r = \"#\" {v := v + 1} {v / 4 == 0} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"length guarded after its binding", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {v := v + len($x)} {0 <= v && v <= 9} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"length bounded above after its binding", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {v := v + len($x)} {v < 10} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"length bounded above with a constant expression", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {v := v + len($x)} {v <= 2 * 5 - 1} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"length guarded before its binding", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {0 <= v + len($x) && v + len($x) <= 9} {v := v + len($x)} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"length bounded above before its binding", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {len($x) < 10} {v := len($x)} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		{"length bounded below only", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {v := v + len($x)} {v >= 0} r / \"y\" {v >= 0}\n", ClassExp, 0},
		{"length bounded only below zero", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {v := v - len($x)} {v <= 0} r / \"y\" {v >= 0}\n", ClassExp, 0},
		// What is not one of the forms bounds nothing.
		{"length guarded after a call", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {v := v + len($x)} r {v <= 9} / \"y\" {v >= 0}\n", ClassExp, 0},
		{"length guarded after input", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {v := v + len($x)} \"z\" {v <= 9} r / \"y\" {v >= 0}\n", ClassExp, 0},
		{"length guarded after a choice of bindings", "var v int\nrule main = r $$\nrule r = x:\"#\"+ ({v := v + len($x)} / {v := 0}) {v <= 9} r / \"y\" {v >= 0}\n", ClassExp, 0},
		{"length guarded by another variable", "var v int\nvar w int = 9\nrule main = r $$\nrule r = x:\"#\"+ {v := v + len($x)} {v <= w} r / \"y\" {v >= 0}\n", ClassExp, 0},
		{"length guarded after another binding of the variable", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {v := v + len($x)} {v := v + 1} {v <= 9} r / \"y\" {v >= 0}\n", ClassExp, 0},
		{"length guarded by a recovered test", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {v := v + len($x)} ({v <= 9}) #recover(skip=\"z\") r / \"y\" {v >= 0}\n", ClassExp, 0},
		{"length guarded before a binding of another expression", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {len($x) <= 9} {v := v + len($x)} r / \"y\" {v >= 0}\n", ClassExp, 0},
		{"length guarded before another binding", "var v int\nvar u int\nrule main = r $$\nrule r = x:\"#\"+ {len($x) <= 9} {u := 1} {v := len($x)} r / \"y\" {v + u >= 0}\n", ClassLinearEnv, 0},
		// A variable bound only to constants holds one of them.
		{"string bound to constants", "var s string\nrule main = r $$\nrule r = \"#\" {s := \"a\"} r / \"(\" {s := \"b\" + \"c\"} r / \"y\" {s == \"a\"}\n", ClassLinear, 0},
		// A bool counts by the values its bindings can give it: one never bound, or bound only to
		// its initial value, counts one.
		{"counter with 64 values and two unbound Booleans", "var v int\nvar b bool\nvar c bool = true\nrule main = r $$\nrule r = \"#\" {v < 63} {v := v + 1} {c := true} r / \"y\" {v >= 0 && !b && c}\n", ClassLinear, 0},
		{"counter with 64 values and a bound Boolean", "var v int\nvar b bool\nrule main = r $$\nrule r = \"#\" {v < 63} {v := v + 1} {b := true} r / \"y\" {v >= 0 && b}\n", ClassLinear, 0},
		{"seven bound Booleans and an unbound one", "var b1 bool\nvar b2 bool\nvar b3 bool\nvar b4 bool\nvar b5 bool\nvar b6 bool\nvar b7 bool\nvar b8 bool\nrule main = (r / .)* $$\nrule r = \"a\" {b1 := true} r / \"b\" {b2 := true} r / \"c\" {b3 := true} r / \"d\" {b4 := true} r / \"e\" {b5 := true} r / \"f\" {b6 := true} r / \"g\" {b7 := true} r / \"y\" {b1 || b2 || b3 || b4 || b5 || b6 || b7 || b8}\n", ClassLinear, 0},
		{"counter with 128 values and a bound Boolean", "var v int\nvar b bool\nrule main = r $$\nrule r = \"#\" {v < 127} {v := v + 1} {b := true} r / \"y\" {v >= 0 && b}\n", ClassLinear, 0},
		{"Boolean toggled before a call", "var b bool\nrule main = r $$\nrule r = x:\"#\" {b := !b && len($x) > 0} r / \"y\" {b}\n", ClassLinear, 0},
		{"counter divided by zero", "var v int\nrule main = r $$\nrule r = \"#\" {v := v + 1 / v} r / \"y\" {v >= 0}\n", ClassLinear, 0},
		// A start rule that its own body calls, directly or through rules called from one place, is
		// memoized, so a call of it that a body evaluates more than once is answered by the memo.
		// Only main is marked @start, so the other rules called from one place are not memoized.
		{"start rule called from one place in a bounded repetition", "@start\nrule main = (\"(\" main \")!\" / \"(\")#repeat(1,2)\n", ClassLinear, 0},
		{"start rule on a cycle of rules called from one place", "@start\nrule main = (\"(\" b \")!\" / \"(\")#repeat(1,2)\nrule b = main\n", ClassLinear, 0},
		{"start rule called from one place in nested bounded repetitions", "@start\nrule main = ((\"(\" main \")!\" / \"(\")#repeat(1,2))#repeat(1,2)\n", ClassLinear, 0},
		{"start rule called from one place at most once per evaluation", "@start\nrule main = (\"(\" main \")!\" / \"(\") (\"(\" \")!\" / \"(\")?\n", ClassLinear, 0},
		{"start rule called from one place in a bounded option", "@start\nrule main = (\"(\" main \")!\" / \"(\")#repeat(0,1)\n", ClassLinear, 0},
		// The literal "(" with which both alternatives of main begin is shared, and so is the "("
		// with which the iterations of b begin, so the comparison looks inside b, past the start
		// of its repetition: what remains of the alternative still contains that repetition, and
		// the later alternative "(" starts again inside the input that it examined.
		{"scan restarted inside a rule called from one place", "@start\nrule main = (\"(\" b \")!\" / \"(\")+ \"(\"*\nrule b = (\"(\" c \")!\" / \"(\")* $$\nrule c = \"(\" c / \"x\"\n", ClassPoly, 2},
		{"scan restarted inside a rule called from one place, without a call after the literal", "@start\nrule main = (\"(\" b \")!\" / \"(\")+ \"(\"*\nrule b = (\"(\" c / \"(\")* $$\nrule c = \"(\" c / \"x\"\n", ClassPoly, 2},
		{"scan restarted inside a rule called from one place, through a bounded call", "@start\nrule main = (\"(\" b \")!\" / \"(\")+ \"(\"*\nrule b = (\"(\" (c)#repeat(0,2) \")!\" / \"(\")* $$\nrule c = \"(\" c / \"x\"\n", ClassPoly, 2},
		// The outer repetition runs at most twice, so the restarted scan does not compound, but
		// condition 2 does not look at the bound of the outer repetition: the class is loose here.
		{"scan restarted inside a rule called from one place, under a bounded outer repetition", "@start\nrule main = (\"(\" b \")!\" / \"(\")#repeat(2,2) \"(\"*\nrule b = (\"(\" c \")!\" / \"(\")* $$\nrule c = \"(\" c / \"x\"\n", ClassPoly, 2},
		{"bounded repetition inside a rule called from one place", "@start\nrule main = (\"(\" b \")!\" / \"(\")+ \"(\"*\nrule b = (\"(\" c \")!\" / \"(\")#repeat(0,3) $$\nrule c = \"(\" c / \"x\"\n", ClassLinear, 0},
		{"rule called from one place that begins apart", "@start\nrule main = (\"(\" b \")!\" / \"(\")+ \"(\"*\nrule b = \"x\" (\"(\" c \")!\" / \"(\")* $$\nrule c = \"(\" c / \"x\"\n", ClassLinear, 0},
		{"rule called from one place by a memoized caller", "@start\nrule top = main $$\nrule main = (\"(\" main \")!\" / \"(\")#repeat(1,2)\n", ClassLinear, 0},
		{"start rule called from one place in an unbounded repetition", "@start\nrule main = \"(\" main* \")\"\n", ClassLinear, 0},
		{"start rule called from one place in a repeated group", "@start\nrule main = (\"(\" main \")\")*\n", ClassLinear, 0},
		{"start rule on a cycle through a repetition", "@start\nrule main = \"(\" b \")\"\nrule b = main*\n", ClassLinear, 0},
		{"start rule called from one place in a bounded list", "@start\nrule main = \"x\" (\",\" main)#repeat(0,3)\n", ClassLinear, 0},
		{"rebinding after the left call", "var k int\nrule main = e $$\nrule e = e \"x\" {k := k + 1} / \"x\" {k >= 0}\n", ClassLinearEnv, 0},
		{"non-leader cycle members", "rule main = c $$\nrule a = c \"+\" / \"n\"\nrule c = \"(\" c \")\" \"y\" / \"(\" c \")\" \"z\" / a\n", ClassLinear, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := classifySource(t, tc.src)
			if c.Class != tc.class || c.Degree != tc.degree {
				t.Errorf("class %v degree %d, want %v degree %d\ncauses %v\nreasons %v", c.Class, c.Degree, tc.class, tc.degree, c.Causes, c.Reasons)
			}
			if tc.class == ClassPoly || tc.class == ClassExp {
				if len(c.Causes) == 0 {
					t.Error("no witness")
				}
			}
			if tc.class == ClassUnknown && len(c.Reasons) == 0 {
				t.Error("no reason")
			}
		})
	}
}

// TestClassifyCarriedGrowth checks condition 5: a leader that grows where another leader of its
// cycle grows, from a result that includes the other's growing result, is quadratic when the step
// can drop its result and its growth goes on over input that later steps examine again, and the
// witness names both rules. The linear grammars have leaders that grow inside other growths
// without both.
func TestClassifyCarriedGrowth(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, src string
		class     ComplexityClass
	}{
		// The lookahead drops the growth of R from each seed of S, which goes on over the "q" of
		// later steps.
		{"tree carried in a lookahead", "rule main = S $$\nrule S = &R S \"q\" / \"y\"\nrule R = R \"q\" / S\n", ClassPoly},
		// The growth of R fails at "!" and S grows by its next alternative.
		{"tree carried before a growing alternative", "rule main = S $$\nrule S = R \"!\" / S \"q\" / \"y\"\nrule R = R \"q\" / S\n", ClassPoly},
		// When R "s" fails, S cannot grow, so R grows past the seed of S once.
		{"nested reads", "rule main = S $$\nrule S = S \"q\" / R \"s\" / \"y\"\nrule R = R \"r\" / S \"t\"\n", ClassLinear},
		// R grows from the seed of S only after a "t", where S cannot grow.
		{"lookahead regrowth", "rule main = S $$\nrule S = &R S \"q\" / \"y\"\nrule R = R (\"q\" / \"r\") / S \"t\" / \"y\"\n", ClassLinear},
		{"three leaders", "rule main = S $$\nrule S = &R S \"q\" / \"y\"\nrule R = &T R \"q\" / \"y\"\nrule T = T \"q\" / S \"z\" / \"y\"\n", ClassLinear},
		// R is after S's growing alternative, so a step that drops it does not grow.
		{"tree carried after the growing alternative", "rule main = S $$\nrule S = S \"q\" / R \"!\" / \"y\"\nrule R = R \"q\" / S\n", ClassLinear},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := classifySource(t, tc.src)
			if c.Class != tc.class {
				t.Fatalf("class %v, want %v\ncauses %v\nreasons %v", c.Class, tc.class, c.Causes, c.Reasons)
			}
			if tc.class != ClassPoly {
				return
			}
			if c.Degree != 2 || len(c.Causes) != 1 {
				t.Fatalf("degree %d causes %v, want one cause of degree 2", c.Degree, c.Causes)
			}
			if m := c.Causes[0].Message; !strings.Contains(m, "grows R where S grows") || !strings.Contains(m, "growing result of S") {
				t.Errorf("witness %q does not name R and S", m)
			}
		})
	}
}

// TestClassifyMemoizedMembers checks the members of left-recursion cycles: a member called by a
// call that is not a left call is memoized at the call and can start a growth, and the leading
// literals that two alternatives share put the calls after them at the same position. A member
// that reads a variable whose values diverge is not shared, and E2 stays without a claim, since
// its last alternative enters the growth that evaluates the earlier ones again.
func TestClassifyMemoizedMembers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, src string
		class     ComplexityClass
		reason    string // a part of a reason of the class unknown
	}{
		{"member called outside its growth", "rule main = s $$\nrule s = \"(\" c \")\" \"y\" / \"(\" c \")\" \"z\"\nrule a = c \"+\" / \"n\" / \"[\" s \"]\"\nrule c = a\n", ClassLinear, ""},
		{"memoized call after a shared literal", "rule main = x $$\nrule x = \"(\" y \")\" \"a\" / \"(\" y \")\" \"b\" / \"n\"\nrule y = \"z\"+ / x\n", ClassLinear, ""},
		{"growing unit restarted by the next step after a shared literal", "rule main = a $$\nrule a = c \"+\" [^!]* \"!\" / c \"+\" / \"n\"\nrule c = a\n", ClassUnknown, "later growing alternative"},
		// The left call of a in c starts a growth when the call of c after "(" evaluates it, and
		// what follows that growth, "+", restarts the growing unit; so through b.
		{"growth started by a member, restarted after it", "rule main = a $$\nrule a = c \"+\" [^!]* \"!\" / \"(\" c \"+\" / \"n\"\nrule c = a\n", ClassUnknown, ""},
		{"growth started by a member through a member", "rule main = a $$\nrule a = c sp [^!]* \"!\" / \"(\" c sp / \"n\"\nrule b = a\nrule c = b\nrule sp = \"+\"\n", ClassUnknown, ""},
		{"growth started by a member from the start rule", "rule a = c \"+\" [^!]* \"!\" / \"(\" c \"+\" / \"n\"\nrule c = a\n", ClassUnknown, ""},
		// The last alternative of c enters the growth where the first two began, and they
		// evaluate their scanning sources only inside calls that the memo answers.
		{"E2", "rule main = c $$\nrule a = c \"+\" / \"n\"\nrule c = \"(\" c \")\" \"y\" / \"(\" c \")\" \"z\" / a\n", ClassLinear, ""},
		{"E2 through a memoized rule off the cycle", "rule main = c $$\nrule a = c \"+\" / \"n\"\nrule c = \"(\" d \")\" \"y\" / \"(\" d \")\" \"z\" / a\nrule d = c\n", ClassLinear, ""},
		// A body of the cycle scans at a position that is the same in each step, so the paths
		// that enter the growth stay in the comparison.
		{"E2 with a scan at a fixed position of the cycle", "rule main = c $$\nrule a = c \"+\" / \"n\" / \"<\" [a-z]* \">\"\nrule c = \"(\" c \")\" \"y\" / \"(\" c \")\" \"z\" / a\n", ClassUnknown, "later alternative a"},
		{"E2 through a memoized call that reads a variable", "var v string\nrule main = c $$\nrule a = c \"+\" / \"n\"\nrule c = \"(\" d \")\" \"y\" / \"(\" d \")\" \"z\" / a\nrule d = c {len(v) >= 0}\n", ClassUnknown, "later alternative a"},
		{"E2 with a scanning source after the growth", "rule main = c [a-z]* $$\nrule a = c \"+\" / \"n\"\nrule c = \"(\" c \")\" \"y\" / \"(\" c \")\" \"z\" / a\n", ClassUnknown, "later alternative a"},
		// Each step evaluates [a-z]* after "(" again.
		{"growth restarting a scan at a fixed position", "rule main = c $$\nrule c = \"(\" [a-z]* \"!\" / a / \"(\" [a-z]* \"?\"\nrule a = c \"+\" / \"n\" / \"(\" [a-z]* \"?\"\n", ClassPoly, ""},
		{"E2 with diverging variables", "var v string\nrule main = c $$\nrule a = c \"+\" / \"n\"\nrule c = \"(\" {v := v + \"0\"} c \")\" \"y\" / \"(\" {v := v + \"1\"} c \")\" \"z\" / a {len(v) >= 0}\n", ClassExp, ""},
		{"member alternatives before its left call", "rule main = c $$\nrule a = c \"+\" / \"n\"\nrule c = \"[\" c \"]\" / a\n", ClassLinear, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := classifySource(t, tc.src)
			if c.Class != tc.class {
				t.Fatalf("class %v, want %v\ncauses %v\nreasons %v", c.Class, tc.class, c.Causes, c.Reasons)
			}
			for _, r := range c.Reasons {
				if strings.Contains(r.Message, "not a leader") {
					t.Errorf("reason %q refuses a growth from a member", r.Message)
				}
			}
			if tc.reason == "" {
				return
			}
			found := false
			for _, r := range c.Reasons {
				found = found || strings.Contains(r.Message, tc.reason)
			}
			if !found {
				t.Errorf("no reason with %q in %v", tc.reason, c.Reasons)
			}
		})
	}
}

// TestClassifyComplexityDeterministic checks that classifying a grammar again gives the same
// result, on a grammar whose units have several callees with sources.
func TestClassifyComplexityDeterministic(t *testing.T) {
	src := "rule main = (a b c d \"q\" / \"x\" / \"y\" / \"z\" / \"w\")* $$\nrule a = \"x\"*\nrule b = \"y\"*\nrule c = \"z\"*\nrule d = \"w\"*\n"
	want := fmt.Sprintf("%+v", *classifySource(t, src))
	for range 50 {
		if got := fmt.Sprintf("%+v", *classifySource(t, src)); got != want {
			t.Fatalf("classification\n%s\nthen\n%s", want, got)
		}
	}
}

// TestComplexityGrowingBindings checks the bindings that the growing-variable check reports.
func TestComplexityGrowingBindings(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      int
	}{
		{"doubling before the call", "var s string = \"a\"\nrule main = r $$\nrule r = \"x\" {s := s + s} r / \"\"\n", 1},
		{"bounded extension", "var indent string\nrule main = r $$\nrule r = \"x\" {indent := indent + \"  \"} r / \"\"\n", 0},
		{"after the call", "var s string = \"a\"\nrule main = r $$\nrule r = \"x\" r {s := s + s} / \"\"\n", 0},
		{"different alternative", "var s string = \"a\"\nrule main = r $$\nrule r = {s := s + s} \"q\" / \"x\" r / \"\"\n", 0},
		{"empty variable", "var s string\nrule main = r $$\nrule r = \"x\" {s := s + s} r / \"\"\n", 0},
		{"in a repetition", "var s string = \"a\"\nrule main = (\"x\" {s := s + s})* $$\n", 1},
		{"int variable", "var n int = 1\nrule main = r $$\nrule r = \"x\" {n := n + n} r / \"\"\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := classifySource(t, tc.src)
			if len(c.Growing) != tc.want {
				t.Errorf("growing %v, want %d", c.Growing, tc.want)
			}
		})
	}
}

// TestGrowingBindingsAlone checks that GrowingBindings returns the Growing member of the full
// classification, on the grammars of the repository and on bindings in inline actions, whose rule
// is the rule that holds them.
func TestGrowingBindingsAlone(t *testing.T) {
	srcs := map[string]string{
		"recursion":     "var s string = \"a\"\nrule main = r $$\nrule r = \"x\" {s := s + s} r / \"\"\n",
		"inline action": "var s string = \"a\"\nrule main = r $$\nrule r = ((\"x\" {s := s + s})* => 1) \"y\"\n",
		"variables":     "var s string = \"a\"\nvar t string\nrule main = (\"x\" {t := t + t} {s := s + s + \"b\"})* $$\n",
	}
	for _, pattern := range []string{"../../testdata/grammars/*/*.zp", "../../examples/*/*.zp"} {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			srcs[p] = string(b)
		}
	}
	for name, src := range srcs {
		t.Run(name, func(t *testing.T) {
			c := classifySource(t, src)
			g, err := syntax.Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			alone, err := GrowingBindings(starttest.MarkAll(g), "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(alone, c.Growing) {
				t.Errorf("GrowingBindings %v, classification %v", alone, c.Growing)
			}
			if strings.HasPrefix(name, "inline") && (len(alone) != 1 || alone[0].Rule != "r") {
				t.Errorf("inline action: %v, want one binding in r", alone)
			}
			if name == "variables" && len(alone) != 1 {
				t.Errorf("variables: %v, want the binding of s", alone)
			}
		})
	}
}

// TestAbbreviate checks that a shortened source text stays valid UTF-8.
func TestAbbreviate(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"a  b\n c", "a b c"},
		{strings.Repeat("x", 40), strings.Repeat("x", 40)},
		{strings.Repeat("x", 41), strings.Repeat("x", 37) + "..."},
		{strings.Repeat("x", 36) + "ééé", strings.Repeat("x", 36) + "..."},
		{strings.Repeat("x", 35) + "一" + strings.Repeat("y", 10), strings.Repeat("x", 35) + "..."},
		{strings.Repeat("x", 34) + "一" + strings.Repeat("y", 10), strings.Repeat("x", 34) + "一..."},
	} {
		got := abbreviate(tc.in)
		if got != tc.want || !utf8.ValidString(got) {
			t.Errorf("abbreviate(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestComplexityCombinations checks the product of the numbers of values of the variables with
// finite values, which bounds K for the class Linear, and that a variable without finite values
// leaves the others as they are.
func TestComplexityCombinations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, src string
		want      int
		// causes are the variables whose bindings the causes and the environment causes name.
		causes []string
	}{
		{"no variable", "rule main = \"a\" $$\n", 1, nil},
		{"guarded counter", "var v int\nrule main = r $$\nrule r = \"#\" {v := v + 1} {v <= 3} r / \"y\" {v >= 0}\n", 4, nil},
		{"length bounded above", "var v int\nrule main = r $$\nrule r = x:\"#\"+ {v := len($x)} {v < 10} r / \"y\" {v >= 0}\n", 10, nil},
		{"constant strings and a Boolean", "var s string\nvar b bool\nrule main = r $$\nrule r = \"#\" {s := \"a\"} {b := true} r / \"(\" {s := \"b\"} r / \"y\" {s == \"a\" || b}\n", 6, nil},
		{"unbounded counter beside a Boolean", "var v int\nvar b bool\nrule main = r $$\nrule r = \"#\" {v := v + 1} {b := true} r / \"y\" {v >= 0 && b}\n", 2, []string{"v"}},
		// Taken from the smallest set up, w would bring the product to 2^33.
		{"product past the cap", "var b bool\nvar u int\nvar w int\nrule main = r $$\nrule r = \"#\" {b := true} r / \"(\" {u < 65535} {u := u + 1} r / \"[\" {w < 65535} {w := w + 1} r / \"y\" {b && u + w >= 0}\n", 2 << 16, []string{"w"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := classifySource(t, tc.src)
			if c.Combinations != tc.want {
				t.Errorf("combinations %d, want %d", c.Combinations, tc.want)
			}
			named := map[string]bool{}
			for _, x := range slices.Concat(c.Causes, c.Env) {
				for _, v := range []string{"b", "s", "u", "v", "w"} {
					if strings.Contains(x.Message, "binding "+v+" :=") {
						named[v] = true
					}
				}
			}
			if want := sliceSet(tc.causes); !maps.Equal(named, want) {
				t.Errorf("causes name the bindings of %v, want %v\ncauses %v\nenv %v", named, want, c.Causes, c.Env)
			}
		})
	}
}

func sliceSet(xs []string) map[string]bool {
	out := map[string]bool{}
	for _, x := range xs {
		out[x] = true
	}
	return out
}
