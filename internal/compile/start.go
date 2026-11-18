package compile

import (
	"slices"

	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/pkg/bytecode"
)

// CheckStart reports why name cannot begin a parse of prog: it is empty and there is no default
// start rule, it is hidden, it is not defined, or it is not a start rule (check.CheckStartName).
func (prog *Program) CheckStart(name string) error {
	return check.CheckStartName(prog.StartRules, prog.DefaultStartRule, prog.definesRule, name)
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
	// A program without a grammar was loaded from its module.
	m := prog.Bytecode
	return slices.ContainsFunc(m.Rules, func(r bytecode.RuleInfo) bool { return m.Strings[r.Name] == name })
}
