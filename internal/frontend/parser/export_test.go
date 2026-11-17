package parser

import "zeroparse.dev/zp/pkg/grammar/ast"

// ExprPrefix is the rule header that ParseExpr puts before an expression: the
// columns of the positions of the expression are those of src plus its length.
const ExprPrefix = "rule e = "

// ParseExpr parses a single parser expression with Parse, as the body of a rule.
func ParseExpr(src string) (ast.Expr, error) {
	g, err := Parse(ExprPrefix + src)
	if err != nil {
		return nil, err
	}
	return g.File.Decls[0].(*ast.RuleDef).Expr, nil
}
