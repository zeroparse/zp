package syntax

import "zeroparse.dev/zp/grammar"

// ExprPrefix is the rule header that ParseExpr puts before an expression: the
// columns of the positions of the expression are those of src plus its length.
const ExprPrefix = "rule e = "

// ParseExpr parses a single parser expression with Parse, as the body of a rule.
func ParseExpr(src string) (grammar.Expr, error) {
	g, err := Parse(ExprPrefix + src)
	if err != nil {
		return nil, err
	}
	return g.File.Decls[0].(*grammar.RuleDef).Expr, nil
}
