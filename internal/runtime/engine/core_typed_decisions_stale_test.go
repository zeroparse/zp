package engine

import (
	"slices"
	"testing"
)

// TestDecisionsStaleEntries is the negative control of the stale entry check of the frozen
// decisions: an entry of a grammar that ran in no tier is reported, an entry of a grammar that
// ran is not, and an entry of a grammar that the tier skipped is reported only if the grammar was
// not skipped.
func TestDecisionsStaleEntries(t *testing.T) {
	gd := &decisionsGolden{lines: map[decisionsKey][]string{
		{"m", "ran"}:       {"x"},
		{"m", "skipped"}:   {"x"},
		{"m", "gone"}:      {"x"},
		{"m", "gone2"}:     {"x"},
		{"other", "gone3"}: {"x"},
	}}
	ran := map[string][]string{"ran": nil}
	if got, want := gd.staleEntries("m", ran, map[string]bool{"skipped": true}), []string{"gone", "gone2"}; !slices.Equal(got, want) {
		t.Errorf("with a skipped grammar: stale %v, want %v", got, want)
	}
	if got, want := gd.staleEntries("m", ran, nil), []string{"gone", "gone2", "skipped"}; !slices.Equal(got, want) {
		t.Errorf("without skipped grammars: stale %v, want %v", got, want)
	}
}
