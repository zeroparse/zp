package tsgen_test

import (
	"regexp"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// A rule that only value-free contexts reference has no original and, entered only through its plain
// function without a value, no constant; a rule that nothing references has none either, yet every
// rule name stays known to parseRule's errors.
func TestGeneratedTSLeavesOutUnreachableRules(t *testing.T) {
	t.Parallel()
	g := mustParse("@start\nrule main = !bad \"a\" kept\nrule bad = \"b\" kept\nrule kept = \"k\"\nrule dead = \"d\"")
	code, err := codegentest.GenerateTSBundle(g, codegen.GenOptions{Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	src := string(code)
	for name, want := range map[string]int{"main": 1, "kept": 1, "bad": 0, "dead": 0} {
		if got := len(regexp.MustCompile(`mkRule\(\d+, "`+name+`", `).FindAllString(src, -1)); got != want {
			t.Errorf("rule %s has %d constants, want %d", name, got, want)
		}
	}
	if !strings.Contains(src, `const ruleNames = ["main", "bad", "kept", "dead"]`) {
		t.Errorf("rule names are not all listed")
	}
}

// A Document parser whose only start rule is main must keep the bodies of the plain rules that main
// calls, with and without Recognize: the reach of the printed calls and the generated rules must
// agree, which a test that marks every rule as a start rule cannot show.
func TestGeneratedTSDocumentKeepsPlainRules(t *testing.T) {
	t.Parallel()
	g := mustParse("@start\nrule main = item* $$\nrule item = word / num\nrule word = [a-z]+\nrule num = [0-9]+")
	for _, opts := range []codegen.GenOptions{
		{Start: "main", Document: true},
		{Start: "main", Document: true, Recognize: true},
		{Start: "main", Document: true, Types: true},
	} {
		code, err := codegentest.GenerateTSBundle(g, opts)
		if err != nil {
			t.Fatalf("%+v: %v", opts, err)
		}
		src := string(code)
		if got := len(regexp.MustCompile(`R\d+\.body = e\d+;`).FindAllString(src, -1)); got != 4 {
			t.Errorf("%+v: %d rules have a body, want 4", opts, got)
		}
	}
}
