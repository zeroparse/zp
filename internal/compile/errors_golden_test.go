package compile

import (
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/syntax"
)

// TestErrorsGolden pins the error lists of grammars with errors of several kinds: type errors
// together with the errors that depend on the grammar facts (a binding before a left-recursive
// call, #stream on a left-recursion cycle, a splice rule that contains itself) and a redeclared
// rule, so that the order in which the checks run keeps the lists.
func TestErrorsGolden(t *testing.T) {
	cases := []struct{ name, src string }{
		{"type error", "type T = { A string }\n@start\nrule main = t $$\nrule t = \"a\" => T{A: 1}"},
		{"type error and left-recursion binding",
			"type T = { A string }\nvar k int\n@start\nrule main = R $$\nrule R = {k := k + 1} R \"x\" / \"x\"\nrule t = \"a\" => T{A: 1}"},
		{"type error and stream on a cycle",
			"type T = { A string }\n@start\nrule main = item* #stream\nrule item = \"a\" \"\\n\" / main\nrule t = \"a\" => T{A: 1}"},
		{"splice cycle and redeclared rule",
			"@start\nrule main = _a $$\nrule _a = \"x\" _a\nrule main = \"y\""},
		{"every kind",
			"type T = { A string }\nvar k int\n@start\nrule main = R item* #stream\nrule R = {k := k + 1} R \"x\" / \"x\"\nrule item = \"a\" / main\n" +
				"rule _a = \"x\" _a\nrule t = \"a\" => T{A: 1}\nrule t = \"b\""},
	}
	var b strings.Builder
	for _, tc := range cases {
		fmt.Fprintf(&b, "# %s\n", tc.name)
		g, err := syntax.Parse(tc.src)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		names := desugar.InlineNames{}
		_, err = Check(g, Options{}, nil, names, nil)
		if err == nil {
			t.Fatalf("%s: compiled", tc.name)
		}
		fmt.Fprintf(&b, "%v\n", names.Rewrite(err))
	}
	factstest.GoldenFile(t, "errors.txt", b.String())
}
