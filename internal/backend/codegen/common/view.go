package common

import (
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Program is a checked grammar as the generators read it: a view over the owners of its facts, not a
// record of its own. internal/driver/checked builds it (checked.Source.Generator), as it builds the
// source of the output schema export (checked.Source.Schema).
//
// Each field names its owner:
//   - Model, the grammar, its types, variables and start rules: the semantic model (check.Model).
//   - Checker, the types of the grammar's expressions and terms: the checker (check.Checker).
//   - Typed, the types the checker found for the grammar: the checker's result (check.TypeInfo).
//   - Run, the tables a parser runs (rules, expectations, sites, kinds): the run-time program
//     (program.Program).
//   - RuleList, Twins and ByName: the views of the rules of Run, in the same order (Rule).
//   - Elab, the input of the Core IR builder: the elaboration's contract (elaborate.Program).
type Program struct {
	*check.Model
	// Check is the checker of the grammar's types, which Checker returns.
	Check *check.Checker
	// Typed holds the types that the checker found, or nil if it did not run.
	Typed *check.TypeInfo
	// Run is the run-time program of the grammar.
	Run *program.Program
	// RuleList holds the views of Run.RuleList and Twins those of Run.Twins, index for index;
	// ByName holds RuleList by name.
	RuleList []*Rule
	Twins    []*Rule
	ByName   map[string]*Rule
	// Elab is the input of the elaboration to Core IR (elaborate.Program): the rules, sites and
	// casts of the grammar as the Core IR builder reads them, for the generators that build Core IR.
	Elab elaborate.Program
}

// Checker returns the checker of the grammar's types.
func (p *Program) Checker() *check.Checker { return p.Check }

// StartRule returns the view of the start rule that name selects (program.Program.StartRule).
func (p *Program) StartRule(name string) (*Rule, error) {
	r, err := p.Run.StartRule(name)
	if err != nil {
		return nil, err
	}
	return p.RuleList[r.ID], nil
}

// Rule is a rule as the generators read it. It embeds the rule of the run-time program (ID, name,
// memoization, left recursion, variables and the other facts a parser runs it with) and points to
// the rule's definition in the grammar; the facts that no other representation holds yet are
// copied from the checked program by the builder of the view.
type Rule struct {
	*program.Rule
	// Def is the rule's definition in the grammar of the semantic model.
	Def *ast.RuleDef
	// Action is the rule's action, nil if it has none: that of Def, or of the hidden rule of an
	// inline action.
	Action ast.ActionExpr
	// Captures is the scope of the rule's captures, in which the checker resolved its terms.
	Captures *check.Scope
	// Lean reports that the body's value is unused (an action, which reads captures only), so the
	// body runs value-free; captures still build values. Copied: only the checked program holds it.
	Lean bool
	// Twin is the view of the value-free twin of the rule, if it has one. Copied: only the checked
	// program links a rule to its twin.
	Twin *Rule
	// Projections maps the captures of repetitions that the action reads only as map($x, (e) => e.$f)
	// to the capture f, nil if there are none (compile.RuleProjections). Copied: only the checked
	// program computes them.
	Projections map[string]string
}

// BodyExpr returns the expression the rule's body matches.
func (r *Rule) BodyExpr() ast.Expr { return r.Def.Expr }
