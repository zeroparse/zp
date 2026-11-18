package compile

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/diagnostic"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/frontend/types"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// The functions and methods of this file run the checker of internal/frontend/check over a
// program: each passes the program's checker (Program.Checker) to the function of the same name.

// CheckTypes is check.CheckTypes over p, whose Typed it sets.
func CheckTypes(prog *Program, errs *diagnostic.ErrorList) { CheckTypesControlled(prog, errs, nil) }

// CheckTypesControlled is CheckTypes polling control; a nil control never cancels.
func CheckTypesControlled(prog *Program, errs *diagnostic.ErrorList, control *cancel.Control) {
	c := prog.Checker()
	check.CheckTypesControlled(c, errs, control)
	prog.Typed = c.Typed
}

func shapeElems(p *Program, e ast.Expr) []check.ValueElem {
	return check.ShapeElems(checkerOf(p), e)
}

func ValuedCount(p *Program, e ast.Expr) int { return check.ValuedCount(checkerOf(p), e) }
func IsElems(p *Program, e ast.Expr) bool    { return check.IsElems(checkerOf(p), e) }
func SingleModeOf(p *Program, e ast.Expr) check.SingleMode {
	return check.SingleModeOf(checkerOf(p), e)
}
func ScopeTuple(p *Program, e ast.Expr) bool       { return check.ScopeTuple(checkerOf(p), e) }
func ShaperOf(p *Program, e ast.Expr) check.Shaper { return check.ShaperOf(checkerOf(p), e) }
func SharesTuple(p *Program, e ast.Expr) bool      { return check.SharesTuple(checkerOf(p), e) }

func (p *Program) ValueRecord() *check.ValueRecord      { return p.Checker().ValueRecord() }
func (p *Program) RuleType(name string) (string, bool)  { return p.Checker().RuleType(name) }
func (p *Program) ValueTypes() map[string]types.Ty      { return p.Checker().ValueTypes() }
func (p *Program) ValueType(name string) (string, bool) { return p.Checker().ValueType(name) }
func (p *Program) StructType(name string) (string, *ast.TypeDef) {
	return p.Checker().StructType(name)
}
func (p *Program) SpliceBody(name string) (ast.Expr, bool)     { return checkerOf(p).SpliceBody(name) }
func (p *Program) NodeTypeOf(name string) (types.NodeTy, bool) { return p.Checker().NodeTypeOf(name) }
func (p *Program) concreteType(t ast.TypeExpr) (string, ast.TypeDecl) {
	return p.Checker().ConcreteType(t)
}
func (p *Program) ruleNodeType(rd *ast.RuleDef) (types.NodeTy, bool) {
	return p.Checker().RuleNodeType(rd)
}
func (p *Program) operatorsNodeType(rd *ast.RuleDef) (types.NodeTy, bool) {
	return p.Checker().OperatorsNodeType(rd)
}

// ValueTypes is check.ValueTypes over p.
func ValueTypes(p *Program, control *cancel.Control) (map[string]types.Ty, diagnostic.ErrorList) {
	return check.ValueTypes(checkerOf(p), control)
}
