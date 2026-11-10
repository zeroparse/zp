package zp_test

import (
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/internal/gentest"
)

// TestLookaheadHasNoFirstTerminal checks, on both engine backends, in a parse and a recognition,
// that an alternative that begins with a positive lookahead is not skipped by the next character
// (Evaluations of Rules in spec/runtime/parsing.md): on "bc" the alternative x of
// gentest.LookaheadFirstSrc is called once, at 0, although the "q" after its lookahead cannot match
// there. The linear cost proof takes the activations inside a skipped alternative to be none, which
// fails for x: its lookahead consumes input.
func TestLookaheadHasNoFirstTerminal(t *testing.T) {
	p, err := zp.CompileSource(gentest.LookaheadFirstSrc, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []zp.Backend{zp.Closure, zp.Bytecode} {
		for _, recognize := range []bool{false, true} {
			opts := []zp.ParseOption{zp.WithBackend(backend)}
			if recognize {
				opts = append(opts, zp.RecognizeOnly())
			}
			calls := 0
			if _, err := p.Parse("bc", append(opts, zp.WithTrace(func(e zp.TraceEvent) {
				if e.Kind == zp.TraceEnter && e.Rule == "x" {
					calls++
				}
			}))...); err != nil {
				t.Fatalf("backend %v, recognize %v: %v", backend, recognize, err)
			}
			if calls != 1 {
				t.Errorf("backend %v, recognize %v: x called %d times, want 1", backend, recognize, calls)
			}
		}
	}
}
