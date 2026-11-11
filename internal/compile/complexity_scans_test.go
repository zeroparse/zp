package compile

import (
	"fmt"
	"strings"
	"testing"
	"zeroparse.dev/zp/internal/cancel"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/internal/syntax"
)

// TestRunFlow checks the algebra of the transfer functions of the run length: a bound stays a bound
// only below runOpen, whatever comes before a part with no bound.
func TestRunFlow(t *testing.T) {
	one := runFlow{runNone, 1}
	reset := runFlow{0, runNone}
	clearing := runFlow{runClear, runNone}
	for _, c := range []struct {
		name string
		f    runFlow
		in   int
		want int
	}{
		{"identity", runID, 3, 3},
		{"no arrival", runBottom, 3, runNone},
		{"one character", one, 3, 4},
		{"one character after a clear", one, runClear, 0},
		{"a reset forgets", reset, 7, 0},
		{"a reset then a character", reset.then(one), 7, 1},
		{"a character then a reset", one.then(reset), 7, 0},
		{"a join keeps the longer", one.join(reset), 7, 8},
		{"a clear keeps the longer run", clearing.join(runID), 5, 5},
		{"a clear alone", clearing, 5, runClear},
		{"a star grows without bound", one.star(), 0, runOpen},
		{"a star grows from a clear", one.star(), runClear, runOpen},
		{"a star of a reset and a character", reset.then(one).star(), 4, 4},
		{"a star after a reset grows", reset.then(one.star()), 4, runOpen},
		{"the longest run with a bound", runFlow{runNone, runMax}, 0, runMax},
		{"a run over the longest with a bound", runFlow{runNone, runMax}, 1, runOpen},
		{"no arrival goes through", one, runNone, runNone},
	} {
		if got := c.f.apply(c.in); got != c.want {
			t.Errorf("%s: apply(%d) = %d, want %d", c.name, c.in, got, c.want)
		}
	}
}

// boundedScans classifies src and returns the text of the repetitions outside main that are bounded
// scans; main is evaluated once, at the start of the input, so its own repetitions always are.
func boundedScans(t *testing.T, src string) map[string]bool {
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
	var scans []*grammar.Repeat
	for _, r := range k.rules {
		if r.Name != "main" {
			WalkExpr(r.Expr, func(e grammar.Expr) {
				if x, ok := e.(*grammar.Repeat); ok && k.srcExprs[x] && k.again[x] != nil {
					scans = append(scans, x)
				}
			})
		}
	}
	k.computeBoundedScans()
	out := map[string]bool{}
	for _, r := range scans {
		if !k.srcExprs[r] {
			out[grammar.FormatExpr(r)] = true
			if k.srcExprs[k.again[r]] {
				t.Errorf("the later iterations of %s stay a scanning source", grammar.FormatExpr(r))
			}
		}
	}
	return out
}

// TestBoundedScans checks each condition of a bounded scan on the repetitions of small grammars:
// want lists the repetitions that are bounded scans, every other one is not.
func TestBoundedScans(t *testing.T) {
	const space = "rule ws = [ ]*\n"
	for _, c := range []struct {
		name, src string
		want      []string
	}{
		// Alternatives that scan the same characters.
		{"digits of two number tokens", "rule main = (n ws)* $$\nrule n = double / int\nrule double = [0-9]+ \".\" [0-9]+\nrule int = [0-9]+\n" + space,
			[]string{"[0-9]+", "[ ]*"}},
		// A repetition after a character outside its alphabet starts a run.
		{"quoted body", "rule main = (s ws)* $$\nrule s = \"\\\"\" [^\"\\n]* \"\\\"\"\n" + space,
			[]string{"[^\"\\n]*", "[ ]*"}},
		// What a greedy repetition leaves cannot start the alphabet.
		{"greedy repetition before its twin", "rule main = (y x)* $$\nrule y = [a]*\nrule x = [a]+ \"b\"\n",
			[]string{"[a]*", "[a]+"}},
		{"a negative lookahead clears the run", "rule main = (\"a\" !\"a\" x)* $$\nrule x = [a]* \"b\"\n",
			[]string{"[a]*"}},
		{"a positive lookahead clears the run", "rule main = (\"a\" &\"b\" x)* $$\nrule x = [a]* \"b\"\n",
			[]string{"[a]*"}},
		{"a bounded run before the scan", "rule main = (\"!\" \"a\" \"a\" x)* $$\nrule x = [a]* \"b\"\n",
			[]string{"[a]*"}},

		// A character of the alphabet before the scan, repeated.
		{"opener in the alphabet", "rule main = t* $$\nrule t = \"<\" [^>]* \">\" / \"<\"\n", nil},
		{"a repetition that extends the run", "rule main = (a / \"x\")* $$\nrule a = \"x\"* \"y\"\n", nil},
		{"a dot that extends the run", "rule main = (t / .)* $$\nrule t = d \"x\"\nrule d = [0-9]+\n", nil},
		{"a run over the longest bound", "rule main = (\"!\" \"" + strings.Repeat("a", runMax+1) + "\" x)* $$\nrule x = [a]* \"b\"\n", nil},
		{"a skip of a recovery that extends the run", "rule main = (x / \"!\")* $$\nrule x = (\"b\" #recover(skip=\"a\")) y\nrule y = [a]* \"c\"\n", nil},
		{"an operator that extends the run", "rule main = e $$\nrule e = #operators {\n    operand y\n    level add { infix left \"a\" }\n}\nrule y = [a]* \"c\"\n", nil},

		// A bound on the run with no bound on the evaluations.
		{"a call at the end of a nest", "rule main = e sp $$\nrule e = \"a\" e (sp \"!\")? / \"b\"\nrule sp = [ ]*\n", nil},
		{"a call behind a prefix with many starts", "rule main = (w sp \"!\" / [ab])* sp $$\nrule w = [ab]+\nrule sp = [ ]*\n", nil},
		{"a memoized call is counted at the end of a nest", "rule main = e sp sp $$\nrule e = \"a\" e (sp \"!\")? / \"b\"\nrule sp = [ ] x\nrule x = [ ]*\n", []string{"[ ]*"}},
		{"an operand of an operators rule", "rule main = e $$\nrule e = #operators {\n    operand y\n    level add { infix left \"+\" }\n}\nrule y = [a]+\n", nil},
		{"an operator part of an operators rule", "rule main = e $$\nrule e = #operators {\n    operand \"c\"\n    level add { infix left \"+\" sp }\n}\nrule sp = [ ]*\n", nil},
		{"a memoized rule on a recursion, in a run with no bound", "rule main = (a / .)* $$\nrule a = \"x\" b\nrule b = \"y\" a / [ ]*\n", nil},
		{"a repetition in a left-recursive rule", "rule main = e $$\nrule e = e \"+\" [a]+ / \"b\"\n", nil},
		{"the skip of an operators rule", "rule main = e $$\nrule e = #operators {\n    skip sp\n    operand \"c\"\n    level add { infix left \"+\" }\n}\nrule sp = [ ]*\n", nil},
		{"a call behind an operators rule", "rule main = (e sp \"!\" / [-x])* sp $$\nrule e = #operators {\n    operand \"x\"\n    level neg { prefix \"-\" }\n}\nrule sp = [ ]*\n", nil},
		{"a call behind a recovery", "rule main = ((\"q\" #recover(skip=[ab]+)) sp \"!\" / [ab])* sp $$\nrule sp = [ ]*\n", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := boundedScans(t, c.src)
			want := map[string]bool{}
			for _, w := range c.want {
				want[w] = true
			}
			for r := range want {
				if !got[r] {
					t.Errorf("%s is no bounded scan", r)
				}
			}
			for r := range got {
				if !want[r] {
					t.Errorf("%s is a bounded scan", r)
				}
			}
		})
	}
}

// TestScanRuns checks the first bound alone: the longest run before the repetition [a]* or, when
// named, another repetition, whether or not its evaluations are counted.
func TestScanRuns(t *testing.T) {
	const x = "rule x = [a]* \"z\"\n"
	for _, c := range []struct {
		name, src string
		rep       string
		want      int
	}{
		{"the start of the input", "rule main = x $$\n" + x, "", 0},
		{"a literal from the start", "rule main = \"aa\" x $$\n" + x, "", 2},
		{"a run from the start past the bound", "rule main = \"" + strings.Repeat("a", runMax+1) + "\" x $$\n" + x, "", runOpen},
		{"a bounded repetition", "rule main = (\"aa\" #repeat(,10)) x $$\n" + x, "", 20},
		{"a bounded repetition past the unrolled count", "rule main = (\"a\" #repeat(,40)) x $$\n" + x, "", runOpen},
		{"an option can match nothing", "rule main = \"a\" [b]? x $$\n" + x, "", 1},
		{"a lookahead that can match nothing clears no run", "rule main = \"a\" &[b]? x $$\n" + x, "", 1},
		{"a skip extends the run", "rule main = (\"b\" #recover(skip=\"a\")) x $$\n" + x, "", 1},
		{"#missing can match nothing", "rule main = \"a\" (\"b\" #missing) x $$\n" + x, "", 1},
		{"an operator part after any number of steps", "rule main = e x $$\nrule e = #operators {\n    operand \"c\"\n    level add { postfix \"a\" }\n}\n" + x, "", runOpen},
		{"a choice that an operator part extends", "rule main = e x $$\nrule e = #operators {\n    operand \"c\"\n    level add { postfix \"aa\" / \"a\" }\n}\n" + x, "", runOpen},
		{"an expression that can skip a character", "rule main = (\"b\"? [ab])* y $$\nrule y = [ab]* \"z\"\n", "[ab]*", runOpen},
		{"an alternative that matches nothing stops a repetition", "rule main = (![b] / \"a\")* x $$\n" + x, "", runOpen},
		{"a literal of two characters leaves one", "rule main = \"aa\"* x $$\n" + x, "", runOpen},
		{"a repetition of two surely consumes nothing", "rule main = ([a] #repeat(2,))* x $$\n" + x, "", 0},
		{"a sequence that can fail after a character", "rule main = ([a] \"b\")* x $$\n" + x, "", 0},
		{"a greedy repetition clears the run", "rule main = [a]+ x $$\n" + x, "", runClear},
		{"a negative lookahead clears the run", "rule main = \"a\" ![a] x $$\n" + x, "", runClear},
		{"a failing alternative leaves the next one", "rule main = t x $$\nrule t = (\"aa\" $$ / \"a\")*\n" + x, "", runClear},
		{"a cut can fail the whole choice", "rule main = t x $$\nrule t = (\"aa\" #cut $$ / \"a\")*\n" + x, "", runOpen},
		{"a cut in a called rule", "rule main = t x $$\nrule t = c*\nrule c = \"aa\" #cut $$ / \"a\"\n" + x, "", runOpen},
		{"a cut under a negative lookahead", "rule main = \"a\" !(\"aa\" #cut $$ / \"a\") x $$\n" + x, "", 1},
		{"a cut in a skip commits the choice around the recovery", "rule main = t x $$\nrule t = ((\"b\" #recover(skip=\"q\" #cut \"r\")) / \"a\")*\n" + x, "", runOpen},
		{"an alternative after the one with a cut is not sure, the one with it is", "rule main = t x $$\nrule t = (\"a\" #cut \"b\"? / \"c\")*\n" + x, "", runClear},
		{"a cut in an option before the character", "rule main = t x $$\nrule t = ((#cut \"q\")? \"a\")*\n" + x, "", runOpen},
		{"a cut in a repetition before the character", "rule main = t x $$\nrule t = ((#cut \"q\")* \"a\")*\n" + x, "", runOpen},
		{"a cut in an option after the character", "rule main = t x $$\nrule t = (\"a\" (\"q\" #cut \"r\")?)*\n" + x, "", runOpen},
		{"a left call can read a failing seed", "rule main = (r / \"b\")* $$\nrule r = t* u / \"b\"\nrule t = r\nrule u = s \"x\"\nrule s = [b]*\n", "[b]*", runOpen},
		{"the alphabet holds the skip", "rule main = \"aa\" y $$\nrule y = (\"b\" #recover(skip=\"a\"))* \"z\"\n", "recover", 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, err := syntax.Parse(c.src)
			if err != nil {
				t.Fatal(err)
			}
			k, _, err := prepareClassifier(starttest.MarkAll(g), "", nil)
			if err != nil {
				t.Fatal(err)
			}
			k.collect()
			k.computeBoundedScans()
			name := c.rep
			if name == "" {
				name = "[a]*"
			}
			found := false
			for r, d := range k.scanRun {
				if text := grammar.FormatExpr(r); text == name || c.rep != "" && strings.Contains(text, name) {
					found = true
					if d != c.want {
						t.Errorf("run %d, want %d", d, c.want)
					}
				}
			}
			if !found {
				t.Errorf("no run for %s", name)
			}
		})
	}
}

// TestLooseCutInSkip checks that a cut in the skip of a recovery commits the choice around it.
func TestLooseCutInSkip(t *testing.T) {
	g, err := syntax.Parse("rule main = \"x\"\n")
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(starttest.MarkAll(g), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !k.looseCut(&grammar.Recovery{Expr: &grammar.Literal{Value: "a"}, Skip: &grammar.Cut{}}) {
		t.Error("a cut in the skip does not commit the choice around the recovery")
	}
}

// TestScanWork bounds the steps of a classification on calls that branch at every level, which
// visit the rules of a grammar exponentially many times unless each rule is followed once.
func TestScanWork(t *testing.T) {
	rigidChain := func(n int) string {
		var b strings.Builder
		b.WriteString("rule main = (r0 \"!\")* $$\n")
		for i := range n {
			fmt.Fprintf(&b, "rule r%d = r%d \"a\" r%d\n", i, i+1, i+1)
		}
		fmt.Fprintf(&b, "rule r%d = \"b\"\n", n)
		return b.String()
	}
	sureChain := func(n int) string {
		var b strings.Builder
		b.WriteString("rule main = (r0)* x $$\nrule x = [ab]* \"!\"\n")
		for i := range n {
			fmt.Fprintf(&b, "rule r%d = r%d \"\" / s%d\nrule s%d = r%d / [c]\n", i, i+1, i, i, i+1)
		}
		fmt.Fprintf(&b, "rule r%d = \"a\"\n", n)
		return b.String()
	}
	steps := func(t *testing.T, src string) uint32 {
		g, err := syntax.Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		c := &cancel.Control{Ctx: compileTestContext(t, 1<<26/cancel.PollStride)}
		if _, err := ClassifyComplexity(starttest.MarkAll(g), "", c); err != nil {
			t.Fatalf("after %d steps: %v", c.Steps, err)
		}
		return c.Steps
	}
	for name, chain := range map[string]func(int) string{"rigid": rigidChain, "sure": sureChain} {
		t.Run(name, func(t *testing.T) {
			small, large := steps(t, chain(32)), steps(t, chain(64))
			t.Logf("%d steps at depth 32, %d at depth 64", small, large)
			if large > 6*small {
				t.Errorf("%d steps at depth 32, %d at depth 64", small, large)
			}
		})
	}
}

// TestBoundedScanClasses checks the classes that bounded scans change, and that the families that
// keep a super-linear cost keep a class that says so.
func TestBoundedScanClasses(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      ComplexityClass
	}{
		{"digits", "rule main = (n ws)* $$\nrule n = double / int\nrule double = [0-9]+ \".\" [0-9]+\nrule int = [0-9]+\nrule ws = [ ]*\n", ClassLinear},
		{"strings", "rule main = (s ws)* $$\nrule s = raw / str\nrule raw = \"\\\"\" [^\"\\n]* \"\\\"\" \"r\"\nrule str = \"\\\"\" ([^\"\\\\]+ / \"\\\\\" .)* \"\\\"\"\nrule ws = [ ]*\n", ClassLinear},
		{"whitespace in two tokens", "rule main = (a / b)* $$\nrule a = [ ]* \"x\"\nrule b = [ ]* \"y\"\n", ClassLinear},
		{"memoized scan that each position starts", "rule main = (t \"y\" / t \"z\" / .)* $$\nrule t = d \"x\"\nrule d = [0-9]+\n", ClassPoly},
		{"scan restarted by a repetition", "rule main = (a \"y\" / \"q\" a / .)* $$\nrule a = x+\nrule x = \"x\"\n", ClassPoly},
		{"scan restarted through a memoized recursion", "rule main = t a? $$\nrule t = a t \"!\" / . t / \"\"\nrule a = x+\nrule x = \"x\"\n", ClassUnknown},
		// The records of the scan site of sp answer the levels that end at the run of blanks.
		{"scan at the end of a nest", "rule main = e sp $$\nrule e = \"a\" e (sp \"!\")? / \"b\"\nrule sp = [ ]*\n", ClassLinear},
		{"cut in a repetition before a scan", "rule main = (t (s \"z\")? \"a\")* \"aa\"? $$\nrule t = (\"aa\" #cut $$ / \"a\")*\nrule s = [a]+\n", ClassUnknown},
		{"cut under a lookahead before a scan", "rule main = (!(\"aa\" #cut $$ / \"a\") (s \"z\")? \"a\")* \"aa\"? $$\nrule s = [a]+\n", ClassPoly},
		{"cut in a called rule before a scan", "rule main = (t (s \"z\")? \"a\")* \"aa\"? $$\nrule t = c*\nrule c = \"aa\" #cut $$ / \"a\"\nrule s = [a]+\n", ClassUnknown},
		{"scan behind a recovery", "rule main = ((\"q\" #recover(skip=[ab]+)) sp \"!\" / [ab])* sp $$\nrule sp = [ ]*\n", ClassPoly},
		{"nested comments that are not closed", "rule main = ws \"x\" $$\nrule ws = (\" \" / c)*\nrule c = \"/*\" (c / !\"*/\" .)* \"*/\"\n", ClassUnknown},
	} {
		if got := classifySource(t, c.src).Class; got != c.want {
			t.Errorf("%s: class %v, want %v", c.name, got, c.want)
		}
	}
}
