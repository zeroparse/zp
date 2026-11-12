package compile_test

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/core/facts"
)

// TestFactsDifferential compares the grammar facts that compilation computes with those that
// internal/ir/core/facts computes on the Core IR module, with the rules' policies cleared, on
// random grammars: left recursion through options, lookaheads, recoveries, cuts and Operators
// Rules, variables and positions read by actions and predicates, insertions, cycles through them
// that exist only with insertion enabled, and hidden rules.
func TestFactsDifferential(t *testing.T) {
	t.Parallel()
	var compiled atomic.Int64
	eachSeed(t, factsSeeds, func(t *testing.T, seed uint64) {
		if factsDifferentialSeed(t, seed) {
			compiled.Add(1)
		}
	})
	// The generator must write mostly valid grammars, or the comparison proves little.
	if n := compiled.Load(); n < factsSeeds*3/4 {
		t.Errorf("only %d of %d random grammars compile", n, factsSeeds)
	}
}

// factsDifferentialSeed compares the facts of the grammar of one seed and reports whether the
// grammar compiles.
func factsDifferentialSeed(t *testing.T, seed uint64) bool {
	t.Helper()
	src := (&factsGrammar{r: rand.New(rand.NewPCG(seed, 53))}).source()
	g, err := compiletest.Parse(src)
	if err != nil {
		t.Fatalf("seed %d: %v\n%s", seed, err, src)
	}
	prog, err := compiletest.Compile(g, compile.Options{})
	if err != nil {
		if testing.Verbose() {
			t.Logf("seed %d: %v", seed, err)
		}
		return false // a binding before a left call, a type error
	}
	want := factstest.Render(compile.ProgramFacts(prog))
	origin := map[core.Node]grammar.Expr{}
	m, err := build.CoreFrom(prog, origin)
	if err != nil {
		t.Fatalf("seed %d: %v\n%s", seed, err, src)
	}
	for _, r := range m.Rules {
		r.Policy = core.Policy{}
	}
	if got := factstest.Render(factstest.CoreRules(m, facts.Compute(m, nil), origin)); got != want {
		t.Fatalf("seed %d: facts differ\n%s\ncompile:\n%s\ncore:\n%s", seed, src, want, got)
	}
	return true
}

// factsGrammar writes one random grammar.
type factsGrammar struct {
	r     *rand.Rand
	rules []string
}

func (g *factsGrammar) pick(xs ...string) string { return xs[g.r.IntN(len(xs))] }

func (g *factsGrammar) term() string {
	switch g.r.IntN(14) {
	case 0, 1, 2:
		return g.pick(`"a"`, `"b"`, `"ab"`, `""`, `"a"i`)
	case 3, 4:
		return g.pick(`[ab]`, `[^a]`, `.`)
	case 5:
		return g.pick(`"x" #missing`, `$$`, `{v == 1}`, `{w := 2}`, `#cut`)
	}
	return g.rules[g.r.IntN(len(g.rules))]
}

func (g *factsGrammar) expr(depth int) string {
	if depth <= 0 {
		return g.term()
	}
	sub := func() string { return g.expr(depth - 1) }
	switch n := g.r.IntN(30); {
	case n < 5:
		return g.term()
	case n < 11:
		return "(" + sub() + " " + sub() + ")"
	case n < 16:
		return "(" + sub() + " / " + sub() + ")"
	case n < 18:
		return "(" + sub() + ")" + g.pick("*", "+", "?", " #repeat(0)", " #repeat(,1)", " #repeat(2, 3)")
	case n < 20:
		return g.pick("!", "&") + "(" + sub() + ")"
	case n < 22:
		return "(" + sub() + " #recover(skip=" + g.pick(`[ab]+`, `[ab]*`, `"b"`, g.term()) + "))"
	case n < 24:
		return "(" + sub() + " #cut " + sub() + " / " + sub() + ")"
	case n < 25:
		return "-(" + sub() + ")"
	case n < 26:
		return "(" + sub() + " #atomic)"
	case n < 28:
		return "(" + sub() + " #cut " + sub() + ")" + g.pick("?", "*", "+")
	}
	return "(" + sub() + " / " + sub() + " / " + sub() + ")"
}

// body writes a rule body: often one that calls a rule before it consumes input.
func (g *factsGrammar) body(name string) string {
	d := 1 + g.r.IntN(2)
	call := g.rules[g.r.IntN(len(g.rules))]
	if g.r.IntN(2) == 0 {
		call = name
	}
	switch g.r.IntN(9) {
	case 0:
		return call + " " + g.expr(d) + " / " + g.expr(d)
	case 5:
		// Left-recursive only through an insertion.
		return `"y" #missing ` + call + " " + g.expr(d) + " / " + g.expr(d)
	case 1:
		return "(" + g.expr(d) + ")? " + call + " " + g.expr(d)
	case 2:
		return "!(" + g.expr(d) + ") " + call + " " + g.expr(d) + " / " + g.expr(d)
	case 3:
		return "(" + call + " " + g.expr(d) + " #recover(skip=[ab]+)) / " + g.expr(d)
	case 4:
		other := g.rules[g.r.IntN(len(g.rules))]
		if other == name {
			other = `"q"`
		}
		return "#operators {\n  skip " + g.expr(1) + "\n  operand \"o\" " + g.expr(d) + "\n  operand " + g.pick(`"p"`, `[ab]?`, g.expr(1)) +
			"\n  operand " + other + "\n  level add { infix left " + g.pick(`"+"`, other) + " }\n  level neg { prefix " + g.pick(`"-"`, call) + " }\n}"
	}
	return g.expr(2 + g.r.IntN(2))
}

func (g *factsGrammar) source() string {
	count := 2 + g.r.IntN(5)
	g.rules = nil
	for i := range count {
		name := fmt.Sprintf("r%d", i)
		if g.r.IntN(5) == 0 {
			name = "_" + name
		}
		g.rules = append(g.rules, name)
	}
	var b strings.Builder
	b.WriteString("var v int\nvar w int\n")
	fmt.Fprintf(&b, "rule main = %s $$\n", g.expr(2))
	for _, name := range g.rules {
		body := g.body(name)
		switch {
		case strings.HasPrefix(body, "#operators"):
			fmt.Fprintf(&b, "rule %s = %s\n", name, body)
		case g.r.IntN(6) == 0:
			fmt.Fprintf(&b, "rule %s = p:(%s) => $p.%s\n", name, body, g.pick("Start", "End"))
		case g.r.IntN(8) == 0:
			fmt.Fprintf(&b, "rule %s = (%s) {%s}\n", name, body, g.pick("v == 1", "w := v"))
		case g.r.IntN(2) == 0:
			fmt.Fprintf(&b, "rule %s = %s => 1\n", name, body)
		default:
			fmt.Fprintf(&b, "rule %s = %s\n", name, body)
		}
	}
	return b.String()
}
