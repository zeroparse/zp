package tsgen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// shortCircuitCases are actions whose right operand of && or || fails (a division by zero) when it
// is evaluated. The engine does not evaluate it after a left operand that decides the result, and
// reports the failure when it must.
func shortCircuitCases() []codegentest.Case {
	const rules = "rule c = [a-z]\n"
	return []codegentest.Case{
		{Name: "and skips a failing right operand", Src: "rule main = x:c $$ => text($x) == \"b\" && 1 / 0 == 1\n" + rules, Inputs: []string{"a"}},
		{Name: "and evaluates a failing right operand", Src: "rule main = x:c $$ => text($x) == \"a\" && 1 / 0 == 1\n" + rules, Inputs: []string{"a"}},
		{Name: "or skips a failing right operand", Src: "rule main = x:c $$ => text($x) == \"a\" || 1 / 0 == 1\n" + rules, Inputs: []string{"a"}},
		{Name: "or evaluates a failing right operand", Src: "rule main = x:c $$ => text($x) == \"b\" || 1 / 0 == 1\n" + rules, Inputs: []string{"a"}},
		{Name: "predicate skips a failing right operand", Src: "rule main = x:c ({text($x) == \"b\" && 1 / 0 == 1} \"!\" / \"?\") $$\n" + rules, Inputs: []string{"a?", "a!"}},
	}
}

// TestShortCircuitGeneratedTypeScript checks that a generated TypeScript parser evaluates the right
// operand of && and || only when the left operand does not decide the result, as the engine does.
func TestShortCircuitGeneratedTypeScript(t *testing.T) {
	t.Parallel()
	node := codegentest.RequireNode(t)
	codegentest.TSParsersCorpus(t, node, shortCircuitCases(), codegen.GenOptions{})
}

// TestShortCircuitGeneratedTypeScriptDetectsEagerOperands is the negative control of the test above:
// a parser that evaluates the right operand whatever the left one is differs from the engine.
func TestShortCircuitGeneratedTypeScriptDetectsEagerOperands(t *testing.T) {
	t.Parallel()
	node := codegentest.RequireNode(t)
	for _, m := range []struct{ name, from, to string }{
		{"and", "if (tb", "if (true || tb"},
		{"or", "if (!tb", "if (true || !tb"},
	} {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			var replaced bool
			mismatches := codegentest.TSParsersMismatches(t, node, shortCircuitCases(), codegen.GenOptions{}, func(src string) string {
				if strings.Contains(src, m.from) {
					replaced = true
				}
				return strings.ReplaceAll(src, m.from, m.to)
			})
			if !replaced {
				t.Fatalf("the generated parser has no %q", m.from)
			}
			if len(mismatches) == 0 {
				t.Errorf("an eager %s operand differs from the engine on none of the cases", m.name)
			}
		})
	}
}
