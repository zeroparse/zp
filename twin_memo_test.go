package zp_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp"
)

// TestTwinMemo checks, on both engine backends, the two facts about value-free twins that the
// charging of Lemma 4 in spec/proofs/linear-cost.md rests on (Evaluations of Rules in
// spec/runtime/parsing.md): a twin has memo entries of its own, so an entry of the twin answers
// no call of the rule, and a twin is memoized exactly when its rule is, even where it is called
// from one place only. In each case the rule r is called from two places, so it is memoized, and
// its twin only from `-r`. The first case is the one where the iterations of `[a-c]*` at 0 are
// direct in an evaluation of the twin and in a later one of the rule, which are both the first
// evaluations of their keys: the proof charges the later to the earlier through their class.
// The test records, for each call of r at 0 from main, whether the memo answered it.
func TestTwinMemo(t *testing.T) {
	cases := []struct {
		src, input, want string
	}{
		{"@start\nrule main = (-r \"x\" / r \"y\")* $$\nrule r = [a-c]* t?\nrule t = \"d\"\n", "abcy", "eval eval"},
		// The twin is evaluated twice, its first evaluation leaving no result, and then answered;
		// the rule is then evaluated twice as if the twin had not been, and then answered.
		{"@start\nrule main = -r \"1\" / -r \"2\" / -r \"3\" / r \"4\" / r \"5\" / r \"6\" / r \"y\"\nrule r = [a-c]* t?\nrule t = \"d\"\n", "abcy", "eval eval memo eval eval memo memo"},
	}
	type frame struct {
		rule     string
		pos      int
		fromMain bool
	}
	for i, c := range cases {
		p, err := zp.CompileSource(c.src, "")
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		for _, backend := range []zp.Backend{zp.Closure, zp.Bytecode} {
			var stack []frame
			var got []string
			_, _ = p.Parse(c.input, zp.WithBackend(backend), zp.WithTrace(func(e zp.TraceEvent) {
				if e.Kind == zp.TraceEnter {
					fromMain := len(stack) > 0 && stack[len(stack)-1].rule == "main"
					stack = append(stack, frame{e.Rule, e.Pos, fromMain})
					return
				}
				f := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if f.rule != "r" || f.pos != 0 || !f.fromMain {
					return
				}
				how := "eval"
				if e.Memo {
					how = "memo"
				}
				got = append(got, how)
			}))
			if s := strings.Join(got, " "); s != c.want {
				t.Errorf("case %d %v: calls of r at 0 from main were %q, want %q", i, backend, s, c.want)
			}
		}
	}
}
