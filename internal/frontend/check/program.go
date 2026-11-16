package check

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
)

// Program is what the checker reads of the program it checks: the lowered grammar, its type
// declarations, its variables and its rules.
type Program interface {
	// Grammar returns the grammar whose rules and types are checked.
	Grammar() *grammar.Grammar
	// TypeDecls returns the type declarations by name.
	TypeDecls() map[string]grammar.TypeDecl
	// Vars returns the declared variables, nil if there are none.
	Vars() *VarTable
	// Rule returns the rule name; its Def is nil if the program has no such rule.
	Rule(name string) RuleInfo
	// EachRule calls f with each rule of the program, in no particular order.
	EachRule(f func(name string, r RuleInfo))
	// ElementScoped reports whether the repetition element e gets a capture scope of its own per
	// iteration (the capture analysis every backend decides the same way), polling control.
	ElementScoped(e grammar.Expr, control *cancel.Control) bool
	// MarkUnsupported records that the unsupported type syntax t was reported and returns true
	// the first time, so that a type reached from several places is reported once.
	MarkUnsupported(t grammar.TypeExpr) (first bool)
}

// RuleInfo is what the checker reads of a rule.
type RuleInfo struct {
	Def *grammar.RuleDef
	// Splice reports that the rule makes no node of its own, so that a call of it contributes what
	// its body does.
	Splice bool
	// Where names the rule in the message of a failed action, if not by its name.
	Where string
}
