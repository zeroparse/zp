package complexitytest

import (
	"os"
	"testing"

	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/engine/state"
)

// skipWithoutBudget skips the rest of t in the reference build, which has no budget controls: the
// tests measure the cost of a parse as the budget it spends.
func skipWithoutBudget(t *testing.T) {
	t.Helper()
	if !state.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
}

// TestMain gives the go commands that the tests run, which build generated parsers, a build cache
// of their own, removed at the end.
func TestMain(m *testing.M) {
	remove := codegentest.SetGoCache()
	code := m.Run()
	remove()
	os.Exit(code)
}
