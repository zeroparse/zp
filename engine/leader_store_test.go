package engine_test

import (
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestLeaderGrowthStoresResult checks, on both engine backends, that a growth of a leader stores
// its result even where the memo leaves the first evaluation of other rules unstored, so that a
// later call of the leader at the position in the same lookahead state or a greater one is answered
// and does not grow it again (Evaluations of Rules in spec/runtime/parsing.md: a first evaluation
// may leave no result in the memo, except a growth). Each case calls the leader e several times at position 0 from main; the
// test records, for each of those calls, whether it was inside a lookahead and whether the memo
// answered it, and compares the sequence with the one that a memo storing every growth gives.
func TestLeaderGrowthStoresResult(t *testing.T) {
	cases := []struct {
		src, input, want string
	}{
		// Three calls outside lookaheads: only the first grows.
		{"@start\nrule main = e \";\" / e \"!\" / e \"?\"\nrule e = e \"+\" \"x\" / \"x\"\n", "x+x?", "0:grow 0:memo 0:memo"},
		// Inside a lookahead, then outside twice: the entry stored inside answers only calls
		// inside, so the first call outside grows again and the last is answered.
		{"@start\nrule main = &(e \";\") e \";\" / e \"!\" / e \"?\"\nrule e = e \"+\" \"x\" / \"x\"\n", "x+x?", "1:grow 0:grow 0:memo"},
		// Outside, then inside, then outside twice: the entry stored outside answers every later call.
		{"@start\nrule main = e \";\" / &(e \"?\") e \"!\" / e \"?\"\nrule e = e \"+\" \"x\" / \"x\"\n", "x+x?", "0:grow 1:memo 0:memo 0:memo"},
		// Twice inside a lookahead, then outside: the growth inside a lookahead stores its result
		// too, so the second call inside is answered and only the call outside grows again.
		{"@start\nrule main = &(e \"?\") &(e \"?\") e \"?\"\nrule e = e \"+\" \"x\" / \"x\"\n", "x+x?", "1:grow 1:memo 0:grow"},
	}
	type frame struct {
		rule     string
		pos      int
		fromMain bool
	}
	for i, c := range cases {
		p, err := engine.CompileSource(c.src, "")
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode} {
			var stack []frame
			var got []string
			_, _ = p.Parse(c.input, engine.WithBackend(backend), engine.WithTrace(func(e engine.TraceEvent) {
				if e.Kind == engine.TraceEnter {
					fromMain := len(stack) > 0 && stack[len(stack)-1].rule == "main"
					stack = append(stack, frame{e.Rule, e.Pos, fromMain})
					return
				}
				f := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if f.rule != "e" || f.pos != 0 || !f.fromMain {
					return
				}
				state, how := 0, "grow"
				if e.Lookahead {
					state = 1
				}
				if e.Memo {
					how = "memo"
				}
				got = append(got, fmt.Sprintf("%d:%s", state, how))
			}))
			if s := strings.Join(got, " "); s != c.want {
				t.Errorf("case %d %v: calls of e at 0 from main were %q, want %q", i, backend, s, c.want)
			}
		}
	}
}

// TestLeaderGrowthEvals checks, on both engine backends, that the leader e of
// codegentest.LeaderStoreCases evaluates its body as often as the growths that the memo allows take
// steps; the generated parsers are held to the same counts by TestGeneratedLeaderStore.
func TestLeaderGrowthEvals(t *testing.T) {
	for _, c := range codegentest.LeaderStoreCases() {
		p, err := engine.CompileSource(c.Src, "")
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode} {
			var prof engine.Profile
			_, _ = p.Parse(c.Input, engine.WithBackend(backend), engine.WithProfile(&prof))
			got := -1
			for _, r := range prof.Rules {
				if r.Rule == "e" {
					got = r.Evals
				}
			}
			if got != c.Evals {
				t.Errorf("%s %v: e evaluated %d times, want %d", c.Name, backend, got, c.Evals)
			}
		}
	}
}
