package complexity_test

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"zeroparse.dev/zp/internal/complexity"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/complexitytest"
	"zeroparse.dev/zp/internal/starttest"
)

// sureGrammar writes random rules over the forms that can fail further than where they stand.
type sureGrammar struct {
	r     *rand.Rand
	rules []string
}

func (g *sureGrammar) term() string {
	terms := []string{`"a"`, `"b"`, `"c"`, `"ab"`, `"aa"`, `"a"i`, `[ab]`, `[^a]`, `.`, `""`, `$$`}
	if g.r.IntN(3) == 0 {
		return g.rules[g.r.IntN(len(g.rules))]
	}
	return terms[g.r.IntN(len(terms))]
}

func (g *sureGrammar) expr(d int) string {
	if d <= 0 {
		return g.term()
	}
	sub := func() string { return g.expr(d - 1) }
	switch g.r.IntN(15) {
	case 0, 1:
		return "(" + sub() + " " + sub() + ")"
	case 2, 3:
		return "(" + sub() + " / " + sub() + ")"
	case 4:
		return "(" + sub() + ")*"
	case 5:
		return "(" + sub() + ")+"
	case 6:
		return "(" + sub() + ")?"
	case 7:
		return "!(" + sub() + ")"
	case 8:
		return "&(" + sub() + ")"
	case 9:
		return "(" + sub() + " #cut " + sub() + ")"
	case 10:
		return "(#cut " + sub() + ")"
	case 11:
		return "(#cut " + sub() + ")?"
	case 12:
		return "(" + sub() + " #recover(skip=" + sub() + "))"
	case 13:
		return "((" + sub() + ") #repeat(2,))"
	}
	return g.term()
}

func (g *sureGrammar) source() string {
	g.rules = []string{"r0", "r1", "r2"}
	var b strings.Builder
	for _, r := range g.rules {
		fmt.Fprintf(&b, "rule %s = %s\n", r, g.expr(3))
	}
	return b.String()
}

// words returns every word over the characters up to the given length.
func words(chars string, length int) []string {
	out, last := []string{""}, []string{""}
	for range length {
		var next []string
		for _, w := range last {
			for _, c := range chars {
				next = append(next, w+string(c))
			}
		}
		out, last = append(out, next...), next
	}
	return out
}

// TestSurelyConsumes runs the claim that the run analysis takes from sure against the parser: a
// greedy repetition of a rule stops only where the next character is outside what the rule surely
// consumes. The check rule fails exactly where the repetition ends before such a character.
func TestSurelyConsumes(t *testing.T) {
	const chars = "abc"
	iterations := 6000
	if testing.Short() {
		iterations = 300
	}
	inputs := words(chars, 5)
	claims := 0
	for seed := range iterations {
		g := &sureGrammar{r: rand.New(rand.NewPCG(uint64(seed), 11))}
		rules := g.source()
		parsed, err := engine.ParseGrammar(rules)
		if err != nil {
			continue
		}
		sure, err := complexity.SurelyConsumed(starttest.MarkAll(parsed), chars)
		if err != nil {
			continue
		}
		for _, r := range g.rules {
			if sure[r] == "" {
				continue
			}
			claims++
			check := fmt.Sprintf("rule main = (%s* ![%s] / !%s*) .* $$\n", r, sure[r], r)
			p, err := complexitytest.Compile(check + rules)
			if err != nil {
				t.Fatalf("seed %d: %v\n%s%s", seed, err, check, rules)
			}
			for _, in := range inputs {
				if n, _ := p.Parse(in); n == nil {
					t.Fatalf("seed %d: %s stops before a character of [%s] on %q\n%s", seed, r, sure[r], in, rules)
				}
			}
		}
	}
	if claims < iterations/10 {
		t.Errorf("only %d claims in %d grammars", claims, iterations)
	}
	t.Logf("%d claims", claims)
}
