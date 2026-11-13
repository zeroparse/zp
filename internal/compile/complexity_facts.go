package compile

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/complexity"
)

// ruleFacts are the rule facts that the classifier reads, by rule name. They are the facts of the
// untyped Core IR module of the rules (internal/ir/core/facts), as a parse without token insertion
// has them, except nullable, which counts an insertion.
type ruleFacts struct {
	// an holds the nullable and always-succeeding rules, the left-recursion cycles and their
	// leaders, the rules that read positions and the rules whose values can hold a recovered error.
	an *Analysis
	// memo are the rules whose results every parse memoizes (memoized), by name of every rule;
	// members, the rules of left-recursion cycles that are memoized outside the growths of their
	// cycle.
	memo, members map[string]bool
	// vars are the variables each rule reads, directly or through its callees, sorted; a rule
	// that reads none has no entry.
	vars map[string][]string
	// nullable are the rules that can succeed without consuming input when token insertion is
	// enabled.
	nullable map[string]bool
	// source is the untyped Core IR module of the rules that the facts are of, which the cost view
	// is built from.
	source *complexity.Source
}

// classifierFacts returns the rule facts of rules, the unique rules of g.
func classifierFacts(g *grammar.Grammar, rules []*grammar.RuleDef, control *cancel.Control) (*ruleFacts, error) {
	src, err := costSource(g, rules, control)
	if err != nil {
		return nil, err
	}
	f := src.Facts
	an := &Analysis{Control: control, nullable: map[string]bool{}, succeeding: map[string]bool{},
		leaders: map[string]bool{}, cyclic: map[string]bool{}, cycle: map[string]int{},
		positional: map[string]bool{}, recovering: map[string]string{}}
	rf := &ruleFacts{source: src, an: an, memo: map[string]bool{}, members: map[string]bool{}, vars: map[string][]string{},
		nullable: map[string]bool{}}
	set := func(m map[string]bool, name string, v bool) {
		if v {
			m[name] = true
		}
	}
	for i, rd := range rules {
		control.Poll()
		r, name := &f.Rules[i], rd.Name
		set(an.nullable, name, r.Nullable)
		set(an.succeeding, name, r.Succeeds)
		set(an.leaders, name, r.Leader)
		set(an.cyclic, name, r.Cyclic())
		set(an.positional, name, r.Positional)
		set(rf.nullable, name, r.NullableIns)
		if r.Cyclic() {
			an.cycle[name] = r.Cycle
		}
		if r.Recovering >= 0 {
			an.recovering[name] = rules[r.Recovering].Name
		}
		if len(r.Vars) > 0 {
			rf.vars[name] = r.Vars
		}
		// A rule that may meet an insertion is not memoized when insertion is enabled, nor is
		// a transient rule; a member of a cycle is memoized only outside its growths.
		rf.memo[name] = r.Leader || !r.Cyclic() && !r.Transient && !r.Missing
		rf.members[name] = r.Member && !r.Transient && !r.Missing
	}
	return rf, nil
}
