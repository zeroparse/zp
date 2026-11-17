package engine

import (
	"testing"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func jsonActionBlockGrammar(t *testing.T, g *ast.Grammar) *ast.Grammar {
	t.Helper()
	data, err := ast.MarshalJSON(g)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ast.UnmarshalJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestContextualOperatorsBeforeLiteralBrace(t *testing.T) {
	t.Parallel()
	check(t, "rule operators = \"x\"\nrule main = operators \"{\"", ok("x{", `("x"@operators "{")@main`))
}
