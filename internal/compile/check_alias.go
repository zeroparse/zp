package compile

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/check"
	"zeroparse.dev/zp/internal/diag"
)

// The checker lives in internal/check. These aliases and forwarders keep the packages that name it
// through compile building; they go with the rename freeze, when those packages import
// internal/check directly.

type (
	TypeInfo       = check.TypeInfo
	ValueRecord    = check.ValueRecord
	TypeResolver   = check.TypeResolver
	ValueInference = check.ValueInference
	SingleMode     = check.SingleMode
	Shaper         = check.Shaper
)

const (
	singleNone  = check.SingleNone
	SinglePass  = check.SinglePass
	SingleTuple = check.SingleTuple
)

var (
	nodeMembers      = check.NodeMemberNames
	retiredTypeNames = check.RetiredTypeNames
	ungroupExpr      = check.UngroupExpr
	ungroupTerm      = check.UngroupTerm
	recovers         = check.Recovers
	termPos          = check.TermPos
	isSeqOf          = check.IsSeqOf
	ruleCycles       = check.RuleCycles
	DefFields        = check.DefFields
	FieldOf          = check.FieldOf
	JoinMembers      = check.JoinMembers
	CapType          = check.CapType
)

// CheckTypes is check.CheckTypes over p, whose Typed it sets.
func CheckTypes(prog *Program, errs *diag.ErrorList) { CheckTypesControlled(prog, errs, nil) }

// CheckTypesControlled is CheckTypes polling control; a nil control never cancels.
func CheckTypesControlled(prog *Program, errs *diag.ErrorList, control *cancel.Control) {
	c := prog.Checker()
	check.CheckTypesControlled(c, errs, control)
	prog.Typed = c.Typed
}

func shapeElems(p *Program, e grammar.Expr) []check.ValueElem {
	return check.ShapeElems(checkerOf(p), e)
}

func ValuedCount(p *Program, e grammar.Expr) int         { return check.ValuedCount(checkerOf(p), e) }
func IsElems(p *Program, e grammar.Expr) bool            { return check.IsElems(checkerOf(p), e) }
func SingleModeOf(p *Program, e grammar.Expr) SingleMode { return check.SingleModeOf(checkerOf(p), e) }
func ScopeTuple(p *Program, e grammar.Expr) bool         { return check.ScopeTuple(checkerOf(p), e) }
func ShaperOf(p *Program, e grammar.Expr) Shaper         { return check.ShaperOf(checkerOf(p), e) }
func SharesTuple(p *Program, e grammar.Expr) bool        { return check.SharesTuple(checkerOf(p), e) }

func (p *Program) ValueRecord() *ValueRecord            { return p.Checker().ValueRecord() }
func (p *Program) RuleType(name string) (string, bool)  { return p.Checker().RuleType(name) }
func (p *Program) ValueTypes() map[string]Ty            { return p.Checker().ValueTypes() }
func (p *Program) ValueType(name string) (string, bool) { return p.Checker().ValueType(name) }
func (p *Program) StructType(name string) (string, *grammar.TypeDef) {
	return p.Checker().StructType(name)
}
func (p *Program) SpliceBody(name string) (grammar.Expr, bool) { return checkerOf(p).SpliceBody(name) }
func (p *Program) NodeTypeOf(name string) (NodeTy, bool)       { return p.Checker().NodeTypeOf(name) }
func (p *Program) concreteType(t grammar.TypeExpr) (string, grammar.TypeDecl) {
	return p.Checker().ConcreteType(t)
}
func (p *Program) ruleNodeType(rd *grammar.RuleDef) (NodeTy, bool) {
	return p.Checker().RuleNodeType(rd)
}
func (p *Program) operatorsNodeType(rd *grammar.RuleDef) (NodeTy, bool) {
	return p.Checker().OperatorsNodeType(rd)
}

// ValueTypes is check.ValueTypes over p.
func ValueTypes(p *Program, control *cancel.Control) (map[string]Ty, diag.ErrorList) {
	return check.ValueTypes(checkerOf(p), control)
}
