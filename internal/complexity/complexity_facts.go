package complexity

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
)

// ruleFacts are the rule facts that the classifier reads, by rule name. They are the facts of the
// untyped Core IR module of the rules (internal/ir/core/facts), as a parse without token insertion
// has them, except nullable, which counts an insertion.
type ruleFacts struct {
	// an holds the nullable and always-succeeding rules, the left-recursion cycles and their
	// leaders, the rules that read positions and the rules whose values can hold a recovered error.
	an *compile.Analysis
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
	source *Source
}

// classifierFacts returns the rule facts of rules, the unique rules of g.
func classifierFacts(g *grammar.Grammar, rules []*grammar.RuleDef, control *cancel.Control) (*ruleFacts, error) {
	src, err := costSource(g, rules, control)
	if err != nil {
		return nil, err
	}
	f := src.Facts
	an := &compile.Analysis{Control: control, Nullable: map[string]bool{}, Succeeding: map[string]bool{},
		Leaders: map[string]bool{}, Cyclic: map[string]bool{}, Cycle: map[string]int{},
		Positional: map[string]bool{}, Recovering: map[string]string{}}
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
		set(an.Nullable, name, r.Nullable)
		set(an.Succeeding, name, r.Succeeds)
		set(an.Leaders, name, r.Leader)
		set(an.Cyclic, name, r.Cyclic())
		set(an.Positional, name, r.Positional)
		set(rf.nullable, name, r.NullableIns)
		if r.Cyclic() {
			an.Cycle[name] = r.Cycle
		}
		if r.Recovering >= 0 {
			an.Recovering[name] = rules[r.Recovering].Name
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
