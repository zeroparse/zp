package engine

import (
	"os"
	"strings"
	"testing"
	"unicode"
)

// TestLinearRewriteSections checks that each section that an @linear error links to is a heading of
// the recipe of linear rewrites, so that renaming a heading cannot leave the errors with a dead link.
func TestLinearRewriteSections(t *testing.T) {
	src, err := os.ReadFile("../../docs/cookbook/linear-rewrites.md")
	if err != nil {
		t.Fatal(err)
	}
	anchors := map[string]bool{}
	for _, line := range strings.Split(string(src), "\n") {
		if heading, ok := strings.CutPrefix(line, "## "); ok {
			anchors[strings.Join(strings.FieldsFunc(strings.ToLower(heading), func(r rune) bool {
				return !unicode.IsLetter(r) && !unicode.IsDigit(r)
			}), "-")] = true
		}
	}
	for kind, r := range rewrites {
		if !anchors[r.section] {
			t.Errorf("the section %q of the rewrite for %v is not a heading of the recipe", r.section, kind)
		}
	}
}
