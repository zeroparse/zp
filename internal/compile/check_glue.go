package compile

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// checkProgram is the program as the checker reads it (check.Program).
type checkProgram struct{ p *Program }

func (c checkProgram) Grammar() *ast.Grammar              { return c.p.Grammar }
func (c checkProgram) TypeDecls() map[string]ast.TypeDecl { return c.p.Types }
func (c checkProgram) Vars() *check.VarTable              { return c.p.Vars }

func (c checkProgram) Rule(name string) check.RuleInfo { return ruleInfo(c.p.ByName[name]) }

func (c checkProgram) EachRule(f func(name string, r check.RuleInfo)) {
	for name, r := range c.p.ByName {
		f(name, ruleInfo(r))
	}
}

func (c checkProgram) ElementScoped(e ast.Expr, control *cancel.Control) bool {
	return elementScopedControlled(e, control)
}

func (c checkProgram) MarkUnsupported(t ast.TypeExpr) bool {
	if c.p.unsupportedSeen[t] {
		return false
	}
	if c.p.unsupportedSeen == nil {
		c.p.unsupportedSeen = map[ast.TypeExpr]bool{}
	}
	c.p.unsupportedSeen[t] = true
	return true
}

func ruleInfo(r *Rule) check.RuleInfo {
	if r == nil {
		return check.RuleInfo{}
	}
	return check.RuleInfo{Def: r.Def, Splice: r.Splice, Where: r.Where}
}

// Checker returns the checker of the program, made on first use.
func (p *Program) Checker() *check.Checker {
	p.checkerOnce.Do(func() { p.chk = check.NewChecker(checkProgram{p}) })
	return p.chk
}

// CheckTerm is check.TermChecker.CheckTerm over the compiler's errors.
func (c *Compiler) CheckTerm(t ast.ActionExpr, s *check.Scope, locals []string) {
	(&check.TermChecker{Control: c.Control, Prog: c.Prog.Checker(), Errs: &c.errs}).CheckTerm(t, s, locals)
}

// checkerOf returns the checker of p, nil for a nil program.
func checkerOf(p *Program) *check.Checker {
	if p == nil {
		return nil
	}
	return p.Checker()
}
