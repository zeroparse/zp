package desugar

import (
	"fmt"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/diag"
)

// UniqueRules returns the first declaration of each rule name of g in order, and an error for each
// later declaration of a name. The analyses key rules by name and loop on a rule list that holds a
// name twice, so every entry point that analyzes a grammar takes its rules from here.
func UniqueRules(g *grammar.Grammar, control *cancel.Control) ([]*grammar.RuleDef, diag.ErrorList) {
	var rules []*grammar.RuleDef
	var errs diag.ErrorList
	seen := map[string]bool{}
	for _, rd := range control.Rules(g) {
		control.Poll()
		if seen[rd.Name] {
			errs = append(errs, &diag.Error{Pos: rd.Diagnostic(), Msg: fmt.Sprintf("rule %s is already defined", rd.Name)})
			continue
		}
		seen[rd.Name] = true
		rules = append(rules, rd)
	}
	return rules, errs
}
