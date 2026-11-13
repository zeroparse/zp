//go:build zp_budget_controls_reference

package engine

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/runtime/parse"
)

// The reference build has no limits: a finite Input limit is unavailable, and zero and Unlimited run.
func TestMaxInputUnavailableInReferenceBuild(t *testing.T) {
	prog := mustCompile(t, `rule main = [a-z]* $$`)
	for _, max := range []uint64{0, parse.Unlimited} {
		if _, err := prog.ParseWith("main", "abc", ParseOptions{Limits: parse.Limits{Input: max}}); err != nil {
			t.Errorf("MaxInput %d: %v", max, err)
		}
	}
	_, err := prog.ParseWith("main", "abc", ParseOptions{Limits: parse.Limits{Input: 100}})
	if err == nil || !strings.Contains(err.Error(), "unavailable in the reference build") {
		t.Errorf("finite MaxInput: error %v", err)
	}
}
