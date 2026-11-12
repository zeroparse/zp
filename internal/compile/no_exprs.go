package compile

import "zeroparse.dev/zp/grammar"

// noExprs is the expression builder of Check, which builds nothing: the compiler still walks every
// rule body and decides its static facts. Its methods implement ExprBuilder, which documents them,
// and return the zero form.
type noExprs struct{}

var _ ExprBuilder[struct{}] = noExprs{}

// NewCheckWalker returns the walker of Check for the compiler c: it checks rule bodies and builds
// nothing.
func NewCheckWalker(c *Compiler) *Walker[struct{}] {
	return &Walker[struct{}]{Compiler: c, Builder: noExprs{}}
}

func (noExprs) Literal(e *grammar.Literal, desc ExpID, build bool) struct{} {
	return struct{}{}
}
func (noExprs) Class(e *grammar.CharClass, desc ExpID, build bool) struct{} {
	return struct{}{}
}
func (noExprs) Any(build bool) struct{}        { return struct{}{} }
func (noExprs) Call(r *Rule, min int) struct{} { return struct{}{} }
func (noExprs) Seq(items []struct{}, flat, valued []bool, n int, build bool) struct{} {
	return struct{}{}
}
func (noExprs) Choice(e *grammar.Choice, alts []struct{}, firsts []First, build bool) struct{} {
	return struct{}{}
}
func (noExprs) Optional(e *grammar.Optional, m struct{}, build bool) struct{} { return struct{}{} }
func (noExprs) And(m struct{}, keepsState bool) struct{}                      { return struct{}{} }
func (noExprs) Not(m struct{}) struct{}                                       { return struct{}{} }
func (noExprs) Atomic(m struct{}, build bool) struct{}                        { return struct{}{} }
func (noExprs) Discard(m struct{}) struct{}                                   { return struct{}{} }
func (noExprs) Capture(e *grammar.Capture, slot int, m struct{}, projected, build bool) struct{} {
	return struct{}{}
}
func (noExprs) Cut() struct{}                                                  { return struct{}{} }
func (noExprs) Empty() struct{}                                                { return struct{}{} }
func (noExprs) Fail() struct{}                                                 { return struct{}{} }
func (noExprs) Anchor(e grammar.Expr, desc ExpID) struct{}                     { return struct{}{} }
func (noExprs) Predicate(e *grammar.Predicate, s *Scope) struct{}              { return struct{}{} }
func (noExprs) ScanRepeat(e *grammar.Repeat, desc ExpID, scan int) struct{}    { return struct{}{} }
func (noExprs) SkipRepeat(e *grammar.Repeat, elem struct{}, skip int) struct{} { return struct{}{} }
func (noExprs) Repeat(e *grammar.Repeat, elem struct{}, elemScope *Scope, ownScope, build, stream bool, site int, shiftable bool, in *Rule, run, scan int) struct{} {
	return struct{}{}
}
func (noExprs) ProjectRepeat(e *grammar.Repeat, elem struct{}, slot, n int) struct{} {
	return struct{}{}
}
func (noExprs) Missing(m struct{}, site int, expected string, build bool) struct{} { return struct{}{} }
func (noExprs) ErrorMessage(m struct{}, msg ExpID, missingScope bool) struct{} {
	return struct{}{}
}
func (noExprs) Diagnostic(m struct{}, name ExpID) struct{}                    { return struct{}{} }
func (noExprs) Recover(e grammar.Expr, m, skip struct{}, build bool) struct{} { return struct{}{} }
func (noExprs) RuleBody(r *Rule, body struct{})                               {}
func (noExprs) OperatorsRule(r *Rule, skip struct{}, hasSkip bool, operands, operators []OpLine[struct{}]) {
}
