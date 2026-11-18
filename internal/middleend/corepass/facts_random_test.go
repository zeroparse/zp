package corepass_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/testing/compiletest"
	"zeroparse.dev/zp/internal/testing/factstest"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// TestFactsRandom checks the rule facts on random grammars: left recursion through options,
// lookaheads, recoveries, cuts and Operators Rules, variables and positions read by actions and
// predicates, insertions, cycles through them that exist only with insertion enabled, and hidden
// rules. For every grammar that compiles it requires
//   - the facts of the untyped module of the rules, which the compiler takes its rule flags from,
//     to equal those of the typed module of the program, value-free twins included, with the
//     rules' policies cleared so that no fact is read back from compilation;
//   - the rule flags of the program to be those facts;
//   - the facts to hold their invariants (checkFactInvariants).
func TestFactsRandom(t *testing.T) {
	t.Parallel()
	compiled := 0
	for seed := range uint64(factsSeeds) {
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
			continue // a binding before a left call, a type error
		}
		compiled++
		origin := map[core.Node]ast.Expr{}
		m, err := elaborate.CoreFrom(prog, origin)
		if err != nil {
			t.Fatalf("seed %d: %v\n%s", seed, err, src)
		}
		for _, r := range m.Rules {
			r.Policy = core.Policy{}
		}
		typed := corepass.Compute(m, nil)
		canon, err := compile.Canonicalize(g, compile.Options{}, desugar.InlineNames{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defs, _ := desugar.UniqueRules(canon.Grammar, nil)
		sorigin := map[core.Node]ast.Expr{}
		sm, err := elaborate.Structural(compile.StructuralInput(canon.Grammar, defs, canon.Starts), sorigin)
		if err != nil {
			t.Fatalf("seed %d: %v\n%s", seed, err, src)
		}
		structural := corepass.Compute(sm, nil)
		want := factstest.Render(factstest.CoreRules(m, typed, origin))
		if got := factstest.Render(factstest.CoreRules(sm, structural, sorigin)); got != want {
			t.Fatalf("seed %d: facts differ\n%s\ntyped:\n%s\nuntyped:\n%s", seed, src, want, got)
		}
		if err := checkProgramFlags(prog, structural); err != nil {
			t.Fatalf("seed %d: %v\n%s", seed, err, src)
		}
		if err := checkFactInvariants(sm, structural); err != nil {
			t.Fatalf("seed %d: %v\n%s", seed, err, src)
		}
	}
	// The generator must write mostly valid grammars, or the checks prove little.
	if compiled < factsSeeds*3/4 {
		t.Errorf("only %d of %d random grammars compile", compiled, factsSeeds)
	}
}

// checkProgramFlags reports a rule flag of prog that is not the fact f has of the rule.
func checkProgramFlags(prog *compile.Program, f *corepass.Facts) error {
	for i, r := range prog.RuleList {
		x := &f.Rules[i]
		if r.Leader != x.Leader || r.Member != x.Member || r.Cycle != x.Cycle || r.Positional != x.Positional ||
			r.Transient != x.Transient || r.Rec != x.Rec || !slices.Equal(r.Vars, x.Vars) || r.Memo != x.Memo {
			return fmt.Errorf("rule %s: flags %+v, facts %+v", r.Name, *r, *x)
		}
		ins := compile.RuleFlags{Leader: r.Leader, Member: r.Member, Cycle: r.Cycle}
		if prog.InsertionFlags != nil {
			ins = prog.InsertionFlags[i]
		}
		if ins.Leader != x.LeaderIns || ins.Member != x.MemberIns || ins.Cycle != x.CycleIns {
			return fmt.Errorf("rule %s: insertion flags %+v, facts %+v", r.Name, ins, *x)
		}
	}
	return nil
}

// checkFactInvariants reports a fact of m that breaks an invariant of the facts: the leaders and
// members split the rules on a cycle, every cycle has a leader and is a cycle of calls, a rule
// proven to succeed is on no cycle, the variables are sorted, a rule nullable without insertion is
// nullable with it, and a rule on a cycle calls a rule of its cycle before it consumes input.
func checkFactInvariants(m *core.Module, f *corepass.Facts) error {
	leaders := map[int]bool{}
	for _, r := range m.Rules {
		x := &f.Rules[r.ID]
		switch {
		case x.Leader && x.Member, x.Cyclic() != (x.Leader || x.Member):
			return fmt.Errorf("rule %s: cycle %d, leader %v, member %v", r.Name, x.Cycle, x.Leader, x.Member)
		case x.Cyclic() && !x.Rec:
			return fmt.Errorf("rule %s: on a left-recursion cycle but not on a cycle of calls", r.Name)
		case x.Succeeds && x.Cyclic():
			return fmt.Errorf("rule %s: proven to succeed on a cycle", r.Name)
		case !slices.IsSorted(x.Vars):
			return fmt.Errorf("rule %s: variables %v not sorted", r.Name, x.Vars)
		case x.Nullable && !x.NullableIns:
			return fmt.Errorf("rule %s: nullable only without insertion", r.Name)
		case x.Memo == x.Leader:
			return fmt.Errorf("rule %s: memo %v for leader %v", r.Name, x.Memo, x.Leader)
		}
		if x.Leader {
			leaders[x.Cycle] = true
		}
		if x.Cyclic() && !slices.ContainsFunc(x.LeftCalls, func(k int) bool { return f.Rules[k].Cycle == x.Cycle }) {
			return fmt.Errorf("rule %s: on cycle %d but no left call into it", r.Name, x.Cycle)
		}
	}
	for _, r := range m.Rules {
		if c := f.Rules[r.ID].Cycle; c != 0 && !leaders[c] {
			return fmt.Errorf("cycle %d has no leader", c)
		}
	}
	return nil
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
