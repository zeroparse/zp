//go:build zp_long

package gogen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedTermChargeMutants checks that TestGeneratedTermChargesPerValue notices a runtime that
// forgets the values of a term without charging for them: with each charged clear of Predicate and
// Bind of the node Parser made a plain clear, some pair spends other than its delta.
func TestGeneratedTermChargeMutants(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	const (
		kids    = "clearSpend(p, p.KidStack[kids:])"
		created = "clearSpend(p, p.created[created:])"
	)
	// The node Parser's Predicate clears the gathered values and the created nodes of a failed term, then
	// the created nodes of a term that succeeds; Bind does the same after it.
	for _, m := range []struct {
		name string
		old  string
		nth  int
	}{
		{"a failed predicate keeps its charge for gathered values", kids, 0},
		{"a failed predicate keeps its charge for created nodes", created, 0},
		{"a predicate keeps its charge for created nodes", created, 1},
		{"a failed binding keeps its charge for gathered values", kids, 1},
		{"a failed binding keeps its charge for created nodes", created, 2},
		{"a binding keeps its charge for created nodes", created, 3},
	} {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			patched := false
			rt := gentest.RuntimeCopyWith(t, func(name, src string) string {
				if name != "call_exec.go" {
					return src
				}
				i := -1
				for range m.nth + 1 {
					j := strings.Index(src[i+1:], m.old)
					if j < 0 {
						return src
					}
					i += 1 + j
				}
				patched = true
				arg := strings.TrimPrefix(strings.TrimSuffix(m.old, ")"), "clearSpend(p, ")
				return src[:i] + "clear(" + arg + ")" + src[i+len(m.old):]
			}, nil)
			if !patched {
				t.Fatalf("call_exec.go has no occurrence %d of %s", m.nth, m.old)
			}
			got := termChargeDeltas(t, rt)
			differ := 0
			for k, v := range termChargeWant() {
				if got[k] != v {
					differ++
					t.Logf("%s: %d, unmutated %d", k, got[k], v)
				}
			}
			if differ == 0 {
				t.Errorf("the mutant %q changes no delta", m.name)
			}
		})
	}
}
