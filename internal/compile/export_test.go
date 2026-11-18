package compile

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// CheckGrammar checks a grammar as internal/frontend.Check does, which computes the rule facts and
// the sites on Core IR. The front end imports this package, so an external test of the package
// sets it (check_hook_test.go) for the internal tests.
var CheckGrammar func(g *ast.Grammar, opts Options, flags []RuleFlags, names desugar.InlineNames, control *cancel.Control) (*Program, error)

// Check is CheckGrammar.
func Check(g *ast.Grammar, opts Options, flags []RuleFlags, names desugar.InlineNames, control *cancel.Control) (*Program, error) {
	return CheckGrammar(g, opts, flags, names, control)
}
