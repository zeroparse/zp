//go:build zp_long

package gengo_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// termChargeMutants are the edits of call_exec.go that make a charged clear of Predicate or Bind a plain clear:
// the nth occurrence of old (in the order of the file) becomes clear(arg). Each must change some delta.
type termChargeMutant struct {
	name string
	old  string
	nth  int
}

// patchTermCharge returns a runtime copy in which m's clear is not charged, failing if call_exec.go has no
// such occurrence.
func patchTermCharge(t *testing.T, m termChargeMutant) string {
	t.Helper()
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
		arg := strings.TrimPrefix(strings.TrimSuffix(m.old, ")"), "clearSpend(")
		_, arg, _ = strings.Cut(arg, ", ")
		return src[:i] + "clear(" + arg + ")" + src[i+len(m.old):]
	}, nil)
	if !patched {
		t.Fatalf("call_exec.go has no occurrence %d of %s", m.nth, m.old)
	}
	return rt
}

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
	for _, m := range []termChargeMutant{
		{"a failed predicate keeps its charge for gathered values", kids, 0},
		{"a failed predicate keeps its charge for created nodes", created, 0},
		{"a predicate keeps its charge for created nodes", created, 1},
		{"a failed binding keeps its charge for gathered values", kids, 1},
		{"a failed binding keeps its charge for created nodes", created, 2},
		{"a binding keeps its charge for created nodes", created, 3},
	} {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			got := termChargeDeltas(t, patchTermCharge(t, m))
			requireChargesChange(t, m.name, got, termChargeWant())
		})
	}
}

// TestGeneratedTypedTermChargeMutants is TestGeneratedTermChargeMutants for the typed Parser (TParser),
// which clears its own Created list and charges for it through the node Parser.
func TestGeneratedTypedTermChargeMutants(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	const (
		kids    = "clearSpend(p.Parser, p.KidStack[kids:])"
		created = "clearSpend(p.Parser, p.Created[created:])"
	)
	for _, m := range []termChargeMutant{
		{"a failed typed predicate keeps its charge for gathered values", kids, 0},
		{"a failed typed predicate keeps its charge for created values", created, 0},
		{"a typed predicate keeps its charge for created values", created, 1},
		{"a failed typed binding keeps its charge for gathered values", kids, 1},
		{"a failed typed binding keeps its charge for created values", created, 2},
		{"a typed binding keeps its charge for created values", created, 3},
	} {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			got := termChargeDeltasWith(t, patchTermCharge(t, m), gen.GenOptions{Types: true}, "ParseAST")
			requireChargesChange(t, m.name, got, termChargeWantTyped())
		})
	}
}

// requireChargesChange fails unless some delta of got differs from want.
func requireChargesChange(t *testing.T, name string, got, want map[string]int64) {
	t.Helper()
	differ := 0
	for k, v := range want {
		if got[k] != v {
			differ++
			t.Logf("%s: %d, unmutated %d", k, got[k], v)
		}
	}
	if differ == 0 {
		t.Errorf("the mutant %q changes no delta", name)
	}
}
