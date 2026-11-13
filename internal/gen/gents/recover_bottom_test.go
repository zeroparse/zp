package gents_test

import (
	"encoding/json"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedTSRecoverBottom checks that a generated TypeScript parser gives the recovery of an
// expression of type _|_ an error value, and an expression without a value none: each parse
// succeeds or fails as gentest.RecoverBottomCases states, and agrees with the engine.
func TestGeneratedTSRecoverBottom(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated code")
	}
	node := gentest.RequireNode(t)
	cases := gentest.RecoverBottomGenCases()
	lines := strings.Split(strings.TrimSuffix(gentest.TSParsersOutputs(t, node, cases, gen.GenOptions{}, nil), "\n"), "\n")
	k := 0
	for _, c := range gentest.RecoverBottomCases {
		for i, in := range c.Inputs {
			// Each ASCII input runs in code points and bytes, as bytes and as a string.
			for range 4 {
				if k >= len(lines) {
					t.Fatalf("%d result lines, want more", len(lines))
				}
				var r struct{ Node string } // the JSON text of the node, "null" for a failed parse
				if err := json.Unmarshal([]byte(lines[k]), &r); err != nil {
					t.Fatalf("%s: %v", lines[k], err)
				}
				if accepted := r.Node != "null"; accepted != c.Accept[i] {
					t.Errorf("%s, %q: accepted %v, want %v\n%s", c.Name, in, accepted, c.Accept[i], lines[k])
				}
				k++
			}
		}
	}
	if k != len(lines) {
		t.Errorf("%d result lines, want %d", len(lines), k)
	}
	gentest.TSParsersCorpus(t, node, cases, gen.GenOptions{})
}
