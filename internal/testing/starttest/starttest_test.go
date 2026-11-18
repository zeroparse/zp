package starttest_test

import (
	"testing"

	"zeroparse.dev/zp/internal/driver/entry"
	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/testing/starttest"
)

func TestMarkAllSkipsHiddenRules(t *testing.T) {
	g, err := parser.Parse("rule _a = \"x\"\nrule b = _a\nrule c = b\n")
	if err != nil {
		t.Fatal(err)
	}
	prog, err := entry.Compile(starttest.MarkAll(g), entry.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := prog.Starts(); len(got) != 2 || got[0] != "b" || got[1] != "c" || prog.DefaultStart() != "b" {
		t.Errorf("starts %v default %q", got, prog.DefaultStart())
	}
}

// A grammar that already has declaration attributes keeps them.
func TestMarkAllKeepsAttributes(t *testing.T) {
	g, err := parser.Parse("rule a = \"x\"\n@start\nrule b = a\n")
	if err != nil {
		t.Fatal(err)
	}
	prog, err := entry.Compile(starttest.MarkAll(g), entry.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := prog.Starts(); len(got) != 1 || got[0] != "b" {
		t.Errorf("starts %v", got)
	}
}
