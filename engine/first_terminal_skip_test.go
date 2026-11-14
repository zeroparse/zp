package engine_test

import (
	"testing"
	"zeroparse.dev/zp/core"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestLookaheadHasNoFirstTerminal checks, on both engine backends, in a parse and a recognition,
// that an alternative that begins with a positive lookahead is not skipped by the next character
// (Evaluations of Rules in spec/runtime/parsing.md): on "bc" the alternative x of
// codegentest.LookaheadFirstSrc is called once, at 0, although the "q" after its lookahead cannot match
// there. The linear cost proof takes the activations inside a skipped alternative to be none, which
// fails for x: its lookahead consumes input.
func TestLookaheadHasNoFirstTerminal(t *testing.T) {
	p, err := engine.CompileSource(codegentest.LookaheadFirstSrc, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode} {
		for _, recognize := range []bool{false, true} {
			opts := []engine.ParseOption{engine.WithBackend(backend)}
			if recognize {
				opts = append(opts, engine.RecognizeOnly())
			}
			calls := 0
			if _, err := p.Parse("bc", append(opts, engine.WithTrace(func(e core.TraceEvent) {
				if e.Kind == core.TraceEnter && e.Rule == "x" {
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
