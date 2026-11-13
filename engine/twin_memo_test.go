package engine_test

import (
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/gentest"
)

// TestTwinMemo checks, on both engine backends, the two facts about value-free twins that the
// charging of Lemma 4 in spec/proofs/linear-cost.md rests on (Evaluations of Rules in
// spec/runtime/parsing.md): a twin has memo entries of its own, so an entry of the twin answers
// no call of the rule, and a twin is memoized exactly when its rule is, even where it is called
// from one place only. In each case the rule r is called from two places or more, so it is
// memoized. The first case is the one where the iterations of `[a-c]*` at 0 are direct in an
// evaluation of the twin and in a later one of the rule, which are both the first evaluations of
// their keys: the proof charges the later to the earlier through their class. In the last case the
// twin of r is called from one place only, `-r` in w, whose value is never discarded, so that w has
// no twin and the place is the only one of the twin of r in the compiled program too; w at 0 and
// at 1 call it at 1, and the memo answers the third call. The test records, for each call
// of r at the position from the caller, whether the memo answered it, in a parse and in a
// recognition.
func TestTwinMemo(t *testing.T) {
	cases := []struct {
		src, input, caller string
		pos                int
		// want is the sequence in a parse, recognize the one in a recognition, where a rule that
		// no call inside a capture that a predicate can read reaches builds no value and has no twin,
		// so the calls of r and -r evaluate one rule with one memo entry.
		want, recognize string
	}{
		{"@start\nrule main = (-r \"x\" / r \"y\")* $$\nrule r = [a-c]* t?\nrule t = \"d\"\n", "abcy", "main", 0, "eval eval", "eval eval"},
		// The twin is evaluated twice, its first evaluation leaving no result, and then answered;
		// the rule is then evaluated twice as if the twin had not been, and then answered.
		{gentest.TwinOwnEntriesSrc, "abcy", "main", 0, "eval eval memo eval eval memo memo", "eval eval memo memo memo memo memo"},
		// The twin of r is called from one place only; the first two calls evaluate it, and the
		// second stores its result, which answers the call of w at 1.
		{gentest.TwinOnePlaceSrc, "abcy", "w", 1, "eval eval memo", "eval eval memo"},
		// The predicate reads only the x of its own alternative, which calls c; the captures named x
		// that call r are in other alternatives, where no predicate can read them, so a
		// recognition gives r no twin: two evaluations, then the memo answers the calls of -r.
		{gentest.TwinScopedCaptureSrc, "aa5", "main", 0, "eval eval eval eval", "eval eval memo memo"},
		// The x that the predicate names is a lambda parameter, which reads no capture, so a
		// recognition gives r no twin although a capture named x calls r before the predicate.
		{gentest.TwinLambdaParamSrc, "aa5", "main", 0, "eval eval eval eval memo", "eval eval memo memo memo"},
		// r is reached through d, which a capture that a predicate reads calls, so a recognition
		// builds its value and keeps its twin although no predicate reads the value of r: d calls
		// the rule twice and the twin twice at 1, each evaluated twice and then answered.
		{"@start\nrule main = s:d { len($s) > 0 }\nrule d = x:\"c\" (y:r \"1\" / y:r \"2\" / -r \"3\" / -r \"4\" / y:r) => $x\nrule r = t*\nrule t = \"a\"\n", "caa", "d", 1, "eval eval eval eval memo", "eval eval eval eval memo"},
	}
	type frame struct {
		rule       string
		pos        int
		fromCaller bool
	}
	for i, c := range cases {
		p, err := engine.CompileSource(c.src, "")
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		for _, run := range []struct {
			backend   engine.Backend
			recognize bool
		}{{engine.Closure, false}, {engine.Bytecode, false}, {engine.Closure, true}, {engine.Bytecode, true}} {
			backend, want := run.backend, c.want
			opts := []engine.ParseOption{engine.WithBackend(backend)}
			if run.recognize {
				want = c.recognize
				opts = append(opts, engine.RecognizeOnly())
			}
			var stack []frame
			var got []string
			_, _ = p.Parse(c.input, append(opts, engine.WithTrace(func(e engine.TraceEvent) {
				if e.Kind == engine.TraceEnter {
					fromCaller := len(stack) > 0 && stack[len(stack)-1].rule == c.caller
					stack = append(stack, frame{e.Rule, e.Pos, fromCaller})
					return
				}
				f := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if f.rule != "r" || f.pos != c.pos || !f.fromCaller {
					return
				}
				how := "eval"
				if e.Memo {
					how = "memo"
				}
				got = append(got, how)
			}))...)
			if s := strings.Join(got, " "); s != want {
				t.Errorf("case %d %v recognize=%v: calls of r at %d from %s were %q, want %q", i, backend, run.recognize, c.pos, c.caller, s, want)
			}
		}
	}
}

// TestRecognizeLambdaParameter checks, on both engine backends, that a recognition in which a
// capture named x records nothing, since the only x that a predicate names is a lambda parameter,
// has the outcome of a parse, the lambda reading its parameter and not the capture.
func TestRecognizeLambdaParameter(t *testing.T) {
	p, err := engine.CompileSource(gentest.TwinLambdaParamSrc, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"aaz6", "aazz6", "aa6", "aa5"} {
		for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode} {
			_, perr := p.Parse(input, engine.WithBackend(backend))
			_, rerr := p.Parse(input, engine.WithBackend(backend), engine.RecognizeOnly())
			if fmt.Sprint(perr) != fmt.Sprint(rerr) {
				t.Errorf("%q %v: parse error %v, recognition error %v", input, backend, perr, rerr)
			}
			if wantOK := strings.HasSuffix(input, "z6") || strings.HasSuffix(input, "zz6"); (perr == nil) != wantOK {
				t.Errorf("%q %v: parse error %v, want success %v", input, backend, perr, wantOK)
			}
		}
	}
}
