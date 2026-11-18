package lint

import (
	"testing"

	"zeroparse.dev/zp/internal/driver/entry"
	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/testing/starttest"
)

func TestMissingTokenDoesNotProveAChoiceShadowed(t *testing.T) {
	// Insertion permits the first alternative to match a different source
	// prefix. A strict literal-prefix proof must not discard that alternative.
	g, err := parser.Parse(`rule main = "a" / "ab" #missing`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Compile(starttest.MarkAll(g), entry.Options{}); err != nil {
		t.Fatal(err)
	}
	for _, finding := range runUnchecked(g, "main", options{}) {
		if finding.Check == "shadowed-alternative" {
			t.Fatal(finding)
		}
	}
}
