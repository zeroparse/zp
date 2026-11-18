package check

import (
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Model is the semantic model of a checked grammar: the grammar the checker read, its type
// declarations, its variables and its start rules. The compiler's program embeds it and adds the
// rules with their static facts; a program loaded from a compiled grammar has a model without a
// grammar.
type Model struct {
	Grammar *ast.Grammar
	// Source is the grammar as written when Grammar is its lowering (inline actions replaced by
	// hidden rules); nil when Grammar is the grammar itself.
	Source *ast.Grammar
	// Types holds the type declarations by name.
	Types map[string]ast.TypeDecl
	// Vars holds the declared variables (nil if there are none).
	Vars *VarTable
	// StartRules holds the start rules in declaration order and DefaultStartRule the default one
	// (desugar.ResolveStarts).
	StartRules       []string
	DefaultStartRule string
}

// NewModel returns a model of the grammar g, whose lowering of source it is (nil if g is the
// grammar as written), with no type declarations yet.
func NewModel(g, source *ast.Grammar) *Model {
	return &Model{Grammar: g, Source: source, Types: map[string]ast.TypeDecl{}}
}

// ResolveStart returns the rule name selects: name, or the default start rule when name is
// empty.
func (m *Model) ResolveStart(name string) string {
	if name == "" {
		return m.DefaultStartRule
	}
	return name
}

// CheckStartName reports why name cannot begin a parse of a grammar with the start rules starts,
// the default start rule def and the rules that defined reports. It is the check that every entry
// to a parse makes, and that the classification of a grammar makes of its start rule.
func CheckStartName(starts []string, def string, defined func(string) bool, name string) error {
	return program.CheckStartName(starts, def, defined, name)
}
