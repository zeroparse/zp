package flowlower

import (
	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// The expressions that Flow IR lowers: in place, and as lines of an Operators Rule. Which compiled
// rules are lowered as flat rules and as framed bodies is decided by the generators
// (common.RecEligible, common.NodeFrameOK) from these, so every backend lowers the same rules.

// BodyOK reports whether Flow IR can lower the expression e in place. A cut is allowed if cuts is set
// and a recovery attribute if recovery is set; an Operators Rule never is.
func BodyOK(expr ast.Expr, cuts, recovery bool) bool {
	ok := true
	astfacts.WalkExpr(expr, func(e ast.Expr) {
		switch e := e.(type) {
		case *ast.Cut:
			if !cuts {
				ok = false
			}
		case *ast.OperatorsRule:
			ok = false
		case *ast.Attributed:
			for _, a := range e.Attrs {
				if a.Name == "recover" && !recovery {
					ok = false
				}
			}
		}
	})
	return ok
}

// LineOK reports whether the Operators Rule line e is a framed body that Flow IR can lower inside the
// runtime-owned frame of its rule.
func LineOK(e ast.Expr) bool {
	return BodyOK(e, true, true)
}
