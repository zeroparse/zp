package complexitytest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/complexity"
	ecore "zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/starttest"
)

// The slope gate holds the classifier to what it says against measured cost. It writes random
// grammars from fixed seeds, classifies each and measures it on every backend on inputs made of
// repeated fragments. A grammar of the class Linear whose cost grows faster than the input fails the
// gate, and so does a polynomial one that grows clearly faster than its degree, or that has reasons
// for the class unknown, which leave its cost open. The default tier runs a few seeds and those of slopeCaught; the tag zp_long runs many more. The gate
// holds the classifier only where its grammars reach: some checks (the cut of a choice in what an
// expression surely consumes, counted rules off every cycle, the repetition depth of nesting) are
// held by the class tests and controls instead.

const (
	slopeSize   = 150 // the first of the doubling sizes of an input
	slopeCap    = 40_000_000
	slopeMargin = 0.45 // what a slope can pass the degree by at these sizes
)

var (
	slopeLiterals = []string{`"a"`, `"a"`, `"b"`, `"c"`, `" "`, `"ab"`, `"aa"`, `"ba"`, `"a"i`}
	slopeClasses  = []string{`[a]`, `[ab]`, `[^a]`, `[a-c]`, `[ ]`, `[^ ]`, `[b]`, `[ac]`}
	slopeInputs   = []string{"a", "b", "c", " ", "ab", "aa", "aab", "a ", "ab ", "abc", "ba", "ca", "a b", "cab"}
)

// slopeGrammar writes one random grammar.
type slopeGrammar struct {
	r     *rand.Rand
	rules []string
}

func (g *slopeGrammar) term() string {
	switch g.r.IntN(15) {
	case 0, 1, 2, 3, 4:
		return slopeLiterals[g.r.IntN(len(slopeLiterals))]
	case 5, 6, 7, 8:
		return slopeClasses[g.r.IntN(len(slopeClasses))]
	case 9:
		return "."
	}
	return g.rules[g.r.IntN(len(g.rules))]
}

func (g *slopeGrammar) expr(depth int) string {
	if depth <= 0 {
		return g.term()
	}
	sub := func() string { return g.expr(depth - 1) }
	list := func(sep string) string {
		parts := []string{sub(), sub()}
		if g.r.IntN(2) == 0 {
			parts = append(parts, sub())
		}
		return "(" + strings.Join(parts, sep) + ")"
	}
	switch n := g.r.IntN(30); {
	case n < 4:
		return g.term()
	case n < 10:
		return list(" ")
	case n < 15:
		return list(" / ")
	case n < 18:
		return "(" + sub() + ")*"
	case n < 20:
		return "(" + sub() + ")+"
	case n < 22:
		return "(" + sub() + ")?"
	case n < 24:
		return "!(" + sub() + ")"
	case n < 25:
		return "&(" + sub() + ")"
	case n < 26:
		return "(" + sub() + " #recover(skip=" + g.term() + "+))"
	case n < 27:
		return "$$"
	case n < 28:
		return "((" + sub() + ") #repeat(," + strconv.Itoa(1+g.r.IntN(3)) + "))"
	}
	return "(" + sub() + " #cut " + sub() + " / " + sub() + ")"
}

// leftRecursive writes a body that calls rule before it consumes input.
func (g *slopeGrammar) leftRecursive(rule string, depth int) string {
	sub := func() string { return g.expr(depth) }
	switch g.r.IntN(4) {
	case 0:
		return rule + " " + sub() + " / " + sub()
	case 1:
		return "(" + sub() + ")? (" + rule + " / " + sub() + ") " + sub()
	case 2:
		return "!(" + sub() + ") " + rule + " " + sub() + " / " + sub()
	}
	return "(" + rule + " " + sub() + " / " + sub() + ")*"
}

// oneOf picks one of the choices.
func (g *slopeGrammar) oneOf(choices ...string) string { return choices[g.r.IntN(len(choices))] }

// template writes a left-recursive rule around a scan that can fail late, the shape in which a call
// is a left call on one path and an ordinary call on another. Random grammars reach it too rarely
// to find a false class in a few seeds.
func (g *slopeGrammar) template() string {
	g.rules = []string{"r", "w"}
	lead := g.oneOf(`[ab]?`, `"a"?`, `[a]*`, `"ab"?`, `("a" #cut "b")?`, `!"q"`, `&[ab]`, `("q" #recover(skip=[ab]+))?`, `(!"a" "b")?`)
	other := g.oneOf(`[a-c]`, `[ab]`, `"a"`, `.`, `"b"`, `w`)
	scan := g.oneOf(`w`, `w [ ]`, `w [ ] [b]`, `w "q"`, `&w`, `!w`, `(w #recover(skip=[ab]+))`, `(w "q" / "a")`, `(w #cut "q" / "b")`)
	end := g.oneOf(`[^a]*`, `[ab]*`, `"a"*`, `[^ ]*`, `[ab]+`, `(. .)*`)
	stop := g.oneOf(`$$`, `"q"`, `[ ]`, `"c"`, `!.`, ``)
	var body, calls string
	switch g.r.IntN(7) {
	case 0:
		body = fmt.Sprintf("%s (r / %s) (%s)", lead, other, scan)
	case 1:
		body = fmt.Sprintf("(%s (r / %s) (%s))*", lead, other, scan)
	case 2:
		body = fmt.Sprintf("%s r %s / %s", lead, scan, other)
	case 3:
		body = fmt.Sprintf("(r %s %s / %s)*", g.oneOf(`[ab]`, `"a"`, `.`), scan, other)
	case 4:
		body = fmt.Sprintf("%s (r %s / %s) / %s", lead, scan, other, g.expr(1))
	default:
		// The alternative that begins with the call of r is behind one or two calls, and what can
		// match nothing before it is in a rule of its own or not.
		if g.r.IntN(2) == 0 {
			calls = fmt.Sprintf("rule p = %s\n", lead)
			lead = "p"
		}
		if g.r.IntN(2) == 0 {
			calls += fmt.Sprintf("rule f = r / %s\n", other)
		} else {
			calls += fmt.Sprintf("rule f = g / %s\nrule g = r\n", other)
		}
		body = fmt.Sprintf("%s f (%s)", lead, scan)
		if g.r.IntN(3) == 0 {
			body = fmt.Sprintf("(%s)*", body)
		}
	}
	rules := fmt.Sprintf("rule r = %s\n%s", body, calls)
	if calls != "" && g.r.IntN(3) == 0 {
		// The leader of the cycle is the rule declared first on it.
		rules = calls + fmt.Sprintf("rule r = %s\n", body)
	}
	return fmt.Sprintf("rule main = %s\n%srule w = %s %s\n", g.oneOf(`r $$`, `r* $$`, `(r / "a")* $$`), rules, end, stop)
}

func (g *slopeGrammar) source() string {
	if g.r.IntN(3) == 0 {
		return g.template()
	}
	g.rules = nil
	count := 2 + g.r.IntN(4)
	for i := range count {
		g.rules = append(g.rules, "r"+strconv.Itoa(i))
	}
	var b strings.Builder
	if g.r.IntN(2) == 0 {
		fmt.Fprintf(&b, "rule main = (%s)* $$\n", g.expr(3))
	} else {
		fmt.Fprintf(&b, "rule main = %s $$\n", g.expr(3))
	}
	for i, r := range g.rules {
		switch {
		case i == count-1 && g.r.IntN(5) == 0:
			fmt.Fprintf(&b, "rule %s = #operators {\n  skip %s\n  operand %s\n  level add { infix left %s }\n  level neg { prefix %s }\n}\n",
				r, g.expr(1), g.expr(1), slopeLiterals[g.r.IntN(len(slopeLiterals))], slopeLiterals[g.r.IntN(len(slopeLiterals))])
		case g.r.IntN(4) == 0:
			fmt.Fprintf(&b, "rule %s = %s\n", r, g.leftRecursive(r, 1+g.r.IntN(2)))
		default:
			fmt.Fprintf(&b, "rule %s = %s\n", r, g.expr(2+g.r.IntN(2)))
		}
	}
	return b.String()
}

// spent returns the cost of a parse, its Compute. It reports a parse that used up slopeCap as
// capped.
func spent(p *engine.Parser, input string, b engine.Backend) (cost uint64, capped bool) {
	_, err := p.Parse(input, engine.WithBackend(b), engine.WithLimits(core.Limits{Compute: slopeCap}),
		engine.WithTrace(func(e engine.TraceEvent) {
			if c := ecore.TraceParser(e).Control; c != nil {
				cost = c.Used
			}
		}))
	var le *core.LimitError
	return cost, errors.As(err, &le)
}

// slopeHit measures one input on one backend at four doubling sizes and describes a slope above
// limit that holds at each of them, or returns "".
func slopeHit(p *engine.Parser, build func(int) string, b engine.Backend, limit float64) string {
	var costs [4]uint64
	for i := range costs {
		c, capped := spent(p, build(slopeSize<<i), b)
		if capped {
			// The cap stops the parse, so all that is known is that the cost is at least c.
			if i == 0 || math.Log2(float64(c)/float64(costs[i-1])) <= limit {
				return ""
			}
			return fmt.Sprintf("%s reaches the cap at size %d, costs %v", BackendName(b), slopeSize<<i, costs[:i])
		}
		if i == 0 && c < 3000 {
			return ""
		}
		costs[i] = c
		if i > 0 {
			if s := math.Log2(float64(c) / float64(costs[i-1])); i >= 2 && s <= limit || i == 1 && s <= limit-0.3 {
				return ""
			}
		}
	}
	return fmt.Sprintf("%s costs %v", BackendName(b), costs)
}

// slopeLimit returns the slope above which the class of the grammar is false, or 0 when the class
// makes no statement to hold it to.
func slopeLimit(src string) (limit float64, class *complexity.Complexity) {
	g, err := engine.ParseGrammar(src)
	if err != nil {
		return 0, class
	}
	class, err = complexity.ClassifyComplexity(context.Background(), starttest.MarkAll(g), "main")
	if err != nil {
		return 0, class
	}
	switch class.Class {
	case ClassLinear, ClassLinearEnv:
		return 1 + slopeMargin, class
	case ClassPoly:
		return float64(class.Degree) + slopeMargin, class
	}
	return 0, class
}

// slopeCheck checks the grammar of one seed on random inputs and describes what it finds, or
// returns "". A grammar that does not compile, as an operand with a left call does not, is skipped.
func slopeCheck(seed uint64, inputs int) string {
	r := rand.New(rand.NewPCG(seed, 7))
	src := (&slopeGrammar{r: r}).source()
	p, err := Compile(src)
	if err != nil {
		return ""
	}
	limit, class := slopeLimit(src)
	if class != nil && class.Class == ClassPoly && len(class.Reasons) > 0 {
		return fmt.Sprintf("seed %d: polynomial of degree %d with %d reasons for the class unknown\n%s", seed, class.Degree, len(class.Reasons), src)
	}
	if limit == 0 {
		return ""
	}
	for range inputs {
		frag := func(p int) string {
			if r.IntN(p) == 0 {
				return slopeInputs[r.IntN(len(slopeInputs))]
			}
			return ""
		}
		before, x, y, after := frag(2), slopeInputs[r.IntN(len(slopeInputs))], frag(2), frag(2)
		build := func(n int) string {
			return before + strings.Repeat(x, n) + strings.Repeat(y, n) + after
		}
		for _, b := range Backends {
			if hit := slopeHit(p, build, b, limit); hit != "" {
				return fmt.Sprintf("seed %d: %v of degree %d, but %s on %q (%q)^n (%q)^n %q\n%s", seed, class.Class, class.Degree, hit, before, x, y, after, src)
			}
		}
	}
	return ""
}

// TestSlopeGateGrowth holds the gate to grammars that grow, whose inputs the grammars reject, as
// the controls of Controls do not: each slope stays above grows on every backend, and the class of
// the grammar must allow for it. Seeds of the gate need not reach such a grammar, so a classifier
// that claims too little of one fails here first.
func TestSlopeGateGrowth(t *testing.T) {
	cases := []struct {
		name, source string
		input        func(int) string
		grows        float64
	}{
		// The alternative r begins with a left call only when [ab]? matches nothing. After a letter
		// it starts at a new position, and the scan in w fails after it at each of them.
		{"left call after an option that consumed input",
			"rule main = r $$\nrule r = [ab]? (r / [a-c]) (w [ ] [b])\nrule w = [^a]* $$\n",
			func(n int) string { return rep("ab", n) + rep("b", n) + " " }, 1 + slopeMargin},
		// The same alternative behind a call, and behind two calls after an option in a rule of its
		// own: the context of a call after input reaches the rules that it calls.
		{"left call after an option, behind a call",
			"rule main = r $$\nrule r = [ab]? f (w [ ] [b])\nrule f = r / [a-c]\nrule w = [^a]* $$\n",
			func(n int) string { return rep("ab", n) + rep("b", n) + " " }, 1 + slopeMargin},
		{"left call after an option rule, behind two calls",
			"rule main = r $$\nrule r = p f (w [ ] [b])\nrule p = [ab]?\nrule f = g / [a-c]\nrule g = r\nrule w = [^a]* $$\n",
			func(n int) string { return rep("ab", n) + rep("b", n) + " " }, 1 + slopeMargin},
		// Each step of the growth of r runs the repetition, which restarts the scan in s. The rules
		// that scan call q, so that their repetitions are no scan sites, whose records would answer
		// the restarts.
		{"restarted scan in a repetition of each growth step",
			"rule main = r* $$\nrule r = (r [ab] s . . .)*\nrule s = ([a]* q \"ab\"+ \" \"*)?\nrule q = \"\"\n",
			func(n int) string { return rep("a", n/2) }, 2 + slopeMargin},
		// The same, with the scan in a rule that the repetition reaches through another rule.
		{"restarted scan in a rule called indirectly",
			"rule main = r* $$\nrule r = (r [ab] m)*\nrule m = s . . .\nrule s = ([a]* q \"ab\"+ \" \"*)?\nrule q = \"\"\n",
			func(n int) string { return rep("a", n/2) }, 2 + slopeMargin},
		// The same growth from the next size on reaches the cap, whose cost is a lower bound.
		{"restarted scan up to the cap",
			"rule main = r* $$\nrule r = (r [ab] s . . .)*\nrule s = ([a]* q \"ab\"+ \" \"*)?\nrule q = \"\"\n",
			func(n int) string { return rep("a", n) }, 2},
		// Units that are not committing nest without bound around the scans that each step of the
		// growth of r evaluates, so the cost passes the degree 2 of the causes: a reason must make
		// the class unknown.
		{"nesting without bound around a growth",
			"rule main = r* $$\nrule r = ([ab]? (r / [a-c]) (w [ ]))*\nrule w = [ab]* q\nrule q = \"\"\n",
			func(n int) string { return " " + rep("a ", n/8) + rep("a", n/8) + "c" }, 2 + slopeMargin + 0.05},
		// The same, where the cycle can also grow from a call of a rule that is not a leader.
		{"nesting without bound around a growth from a rule that is not a leader",
			"rule main = r $$\nrule p = (!\"a\" \"b\")?\nrule f = r / [a-c]\nrule r = (p f (w [ ] [b]))*\nrule w = [^ ]* q\nrule q = \"\"\n",
			func(n int) string { return "aab" + rep("a b", n/8) + rep("b", n/8) + "a" }, 2 + slopeMargin + 0.05},
		// The inner map copies $y for each element of each element, which the degree 2 of the
		// restarted scan in B does not cover.
		{"copies of an outer value",
			"rule main = v:V (B / \"a\")* $$ => $v\nrule V = xs:\"x\"* y:\"y\"* => map($xs, (p) => map($xs, (q) => text($y) + \"!\"))\nrule B = \"a\"* \"c\"\n",
			func(n int) string { return rep("x", n/6) + rep("y", n/6) + "a" }, 2 + slopeMargin + 0.05},
		// The value of e holds the value of the e nested inside it twice, so it doubles on each
		// level: the slope passes any that a copy outside the value, of degree 2, allows.
		{"fold holding a recursive value twice",
			"rule main = e $$\nrule e string = l:a rest:(o:\"+\" r:a)* => foldl($l, $rest, (acc, i) => acc + text(i.$o) + i.$r + i.$r)\nrule a string = (\"x\" => \"x\") / (\"(\" v:e \")\" => $v)\n",
			func(n int) string { return rep("x+(", n/12) + "x" + rep(")", n/12) }, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := Compile(c.source)
			if err != nil {
				t.Fatal(err)
			}
			limit, class := slopeLimit(c.source)
			if limit != 0 && limit <= c.grows {
				t.Errorf("%v of degree %d, below its measured growth", class.Class, class.Degree)
			}
			for _, b := range Backends {
				if slopeHit(p, c.input, b, c.grows) == "" {
					t.Errorf("%s stays below a slope of %.2f", BackendName(b), c.grows)
				}
				if limit != 0 {
					if hit := slopeHit(p, c.input, b, limit); hit != "" {
						t.Errorf("%v of degree %d, but %s", class.Class, class.Degree, hit)
					}
				}
			}
		})
	}
}

// TestSlopeGateCountsRecoveredErrors holds Linear grammars whose results hold recovered errors to a
// linear slope on every backend, with the work on those errors counted: each item of the first
// two holds the errors of all the items after it, and a call reuses a list of errors at each
// position of the third. While the parser copied the lists, each of them cost the square of n.
func TestSlopeGateCountsRecoveredErrors(t *testing.T) {
	for _, c := range []struct {
		name, src string
		input     func(int) string
	}{
		{"nested items", `rule main = s $$
rule s = item s "!" / item s / item
rule item = ("a" ";") #recover(skip=[^;]* ";")
`, func(n int) string { return rep("b;", n) }},
		{"left recursion", `rule main = e $$
rule e = e "," item / item
rule item = ("a" ";") #recover(skip=[^;]* ";")
`, func(n int) string { return strings.TrimSuffix(rep("b;,", n), ",") }},
		{"reused lists", `rule main = (("a"+ &r3)*)* $$
rule r0 = ([b] r3)?
rule r1 = (([^a] [ac]) #recover(skip="a"i+)) / "a"i / (("aa" / r0) r4 &r2)
rule r2 = !(!r0)
rule r3 = (([ac] "a") / (r2 / r4)) ((r1 r0 .) / "ba"?)
rule r4 = &[^ ]
`, func(n int) string { return rep("ab", n) + rep("aab", n) + "b" }},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, err := Compile(c.src)
			if err != nil {
				t.Fatal(err)
			}
			limit, class := slopeLimit(c.src)
			if class.Class != ClassLinear {
				t.Fatalf("class %v, want Linear", class.Class)
			}
			for _, b := range Backends {
				if hit := slopeHit(p, c.input, b, limit); hit != "" {
					t.Error(hit)
				}
			}
		})
	}
}

// TestSlopeLimits holds the limits of the gate to the classes that they hold grammars to.
func TestSlopeLimits(t *testing.T) {
	for _, c := range []struct {
		source string
		want   float64
	}{
		{"rule main = (a / \"a\")* $$\nrule a = \"a\" \"c\"\n", 1 + slopeMargin},
		{"rule main = (a / \"a\")* $$\nrule a = \"a\"* \"c\"\n", 2 + slopeMargin},
		{"rule main = (a / \"a\")* $$\nrule a = (b / \"a\")* \"c\"\nrule b = \"a\"* \"d\"\n", 3 + slopeMargin},
	} {
		if got, class := slopeLimit(c.source); got != c.want {
			t.Errorf("%v of degree %d: limit %.2f, want %.2f", class.Class, class.Degree, got, c.want)
		}
	}
}

// slopeCaught lists seeds beyond the seeds of the default tier that found a false class under a
// negative control: 508 when the check of a unit after input in its own rule was taken out, and 609
// when a call after input no longer made its callee start growths (startsGrowth).
var slopeCaught = []uint64{508, 609}

func TestSlopeGate(t *testing.T) {
	const inputs = 8
	grammars := uint64(5000)
	if reduced() {
		grammars = 150
	}
	var mu sync.Mutex
	next := make(chan uint64)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range next {
				if hit := slopeCheck(seed, inputs); hit != "" {
					mu.Lock()
					t.Error(hit)
					mu.Unlock()
				}
			}
		}()
	}
	for seed := range grammars {
		next <- seed
	}
	for _, seed := range slopeCaught {
		if seed >= grammars {
			next <- seed
		}
	}
	close(next)
	wg.Wait()
}
