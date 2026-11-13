package tsgen_test

import (
	"testing"

	"zeroparse.dev/zp/internal/codegentest"
)

// TestGeneratedTSLookaheadHasNoFirstTerminal checks that a generated TypeScript parser does not
// skip, by the next character, an alternative that begins with a positive lookahead (Evaluations
// of Rules in spec/runtime/parsing.md), as TestLookaheadHasNoFirstTerminal does for the engine: on
// "bc", a parse and a recognition, without and with limit controls, evaluate the body of x of
// codegentest.LookaheadFirstSrc once.
func TestGeneratedTSLookaheadHasNoFirstTerminal(t *testing.T) {
	t.Parallel()
	const evals = `(m, run) => {
  const profile = new m.Profile();
  run({ profile });
  return String(profile.snapshot().rules.filter((r) => r.rule === "x").reduce((n, r) => n + r.evals, 0));
}`
	got := runTS(t, []tsRunCase{{"lookahead first", codegentest.LookaheadFirstSrc, "bc"}}, evals)
	for j, form := range tsRunForms {
		if got[0][j] != "1" {
			t.Errorf("%s: x evaluated %s times, want 1", form, got[0][j])
		}
	}
}
