package compile

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/desugar"
)

// CheckStart reports why name cannot begin a parse of prog: it is empty and there is no default
// start rule, it is hidden, it is not defined, or it is not a start rule.
func (prog *Program) CheckStart(name string) error {
	return checkStartName(prog.StartRules, prog.DefaultStartRule, prog.definesRule, name)
}

// checkStartName reports why name cannot begin a parse of a grammar with the start rules starts,
// the default start rule def and the rules that defined reports. It is the check that every entry
// to a parse makes, and that the classification of a grammar makes of its start rule.
func checkStartName(starts []string, def string, defined func(string) bool, name string) error {
	resolved := name
	if name == "" {
		resolved = def
	}
	if slices.Contains(starts, resolved) {
		return nil
	}
	if name == "" {
		return fmt.Errorf("no start rule is given")
	}
	if err := desugar.EntryError(name); err != nil {
		return err
	}
	if defined(name) {
		return fmt.Errorf("rule %s is not a start rule; parsing can begin only at a rule marked @start", name)
	}
	return fmt.Errorf("rule %s is not defined", name)
}

// ResolveStart returns the rule name selects: name, or the default start rule
// when name is empty.
func (prog *Program) ResolveStart(name string) string {
	if name == "" {
		return prog.DefaultStartRule
	}
	return name
}

// StartRule returns the start rule name selects.
func (prog *Program) StartRule(name string) (*Rule, error) {
	if err := prog.CheckStart(name); err != nil {
		return nil, err
	}
	return prog.ByName[prog.ResolveStart(name)], nil
}

// definesRule reports whether the grammar defines the rule name, without preparing a backend.
func (prog *Program) definesRule(name string) bool {
	if prog.Grammar != nil {
		return prog.ByName[name] != nil
	}
	m := prog.BytecodeModule()
	return slices.ContainsFunc(m.Rules, func(r bytecode.RuleInfo) bool { return m.Strings[r.Name] == name })
}
