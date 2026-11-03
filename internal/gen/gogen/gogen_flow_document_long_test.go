//go:build zp_long

package gogen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// TestFlowNodeDocumentMutants checks that the checks notice a Flow build that reads input in place in a
// Document parse: the mutants remove the guard of one in-place read (a literal, then a peek), or of all of them,
// so that the read does not record how far the parse examined; a Document then reuses results that an edit
// has invalidated, which differs from a parse from scratch or changes the counts the golden holds. Verify
// rejects the same read in the IR (TestVerifyRejects in package flow).
func TestFlowNodeDocumentMutants(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	gentest.PinOrdinary(t)
	var cases []documentCase
	for _, c := range documentShapes {
		cases = append(cases, documentCase{c, 300})
	}
	golden := gentest.ReadGolden(t, documentGolden)
	digestsDiffer := func(lines []string) bool {
		for k, v := range documentDigests(t, "corpus", cases, lines) {
			if w, ok := golden[k]; !ok || w != v {
				return true
			}
		}
		return false
	}
	base := runDocumentProbes(t, cases, "main", nil)
	if digestsDiffer(base) {
		t.Fatalf("the digests of the unmutated build differ from %s", documentGolden)
	}
	for _, m := range []struct {
		name   string
		mutate func(string) string
	}{
		{"multi-character literal reads", func(s string) string { return strings.ReplaceAll(s, `p.Document == nil && (p.Pos+`, `(p.Pos+`) }},
		{"single-character literal reads", func(s string) string {
			return strings.ReplaceAll(s, `p.Document == nil && (p.Pos < len(p.In) && p.In[p.Pos] ==`, `(p.Pos < len(p.In) && p.In[p.Pos] ==`)
		}},
		{"peeks", func(s string) string {
			return strings.ReplaceAll(s, `if p.Document == nil && p.Pos < len(p.In) {`, `if p.Pos < len(p.In) {`)
		}},
		{"every read", func(s string) string { return guardedRead.ReplaceAllString(s, "") }},
	} {
		t.Run(m.name, func(t *testing.T) {
			changed := 0
			mf := runDocumentProbes(t, cases, "main", func(code []byte) []byte {
				out := m.mutate(string(code))
				if out != string(code) {
					changed++
				}
				return []byte(out)
			})
			if changed == 0 {
				t.Fatalf("the mutant %q changes no build", m.name)
			}
			if mf == nil {
				return // the consumer of the mutant failed
			}
			stale := false
			for _, l := range mf {
				if mm := documentLine.FindStringSubmatch(l); mm[4] != "0" {
					stale = true
				}
			}
			if !stale && !digestsDiffer(mf) {
				t.Errorf("the mutant %q changes no report: the reads are not checked", m.name)
			}
		})
	}
}
