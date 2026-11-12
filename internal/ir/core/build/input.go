package build

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/check"
	"zeroparse.dev/zp/internal/types"
)

// Program is the compiled program as the builder reads it. The builder depends on this interface
// and on the checker, never on the compiler that implements it.
type Program interface {
	// CoreInput returns the rules and declarations of the program (see Input). Each call returns
	// a new Input.
	CoreInput() *Input
	// Checker returns the checker of the program's types.
	Checker() *check.Checker
	// ResolveCheck returns the type a cast checks, or nil if every value fits it.
	ResolveCheck(t grammar.TypeExpr) types.Ty
	// CastPos returns the line and column of a cast, for the message of a failed check.
	CastPos(t *grammar.Cast) (int, int)
	// SkipSite returns the number of the #recover skip site of the repetition (core.Repeat.Site).
	SkipSite(e *grammar.Repeat) int
	// ScanSite returns the number of the scan site of the repetition (core.Repeat.ScanSite).
	ScanSite(e *grammar.Repeat) int
}

// Input holds the rules and declarations that the builder reads from a program.
type Input struct {
	// Rules holds the rules and then their value-free twins; each is at the position of its ID.
	Rules []*Rule
	// Starts holds the start rules, in order.
	Starts []*Rule
	// ByName maps a name to the rule of that name (a twin has the name of its rule and is not in it).
	ByName map[string]*Rule
	// Types holds the type declarations of the grammar.
	Types map[string]grammar.TypeDecl
	// Vars holds the variable declarations, nil if there are none.
	Vars *check.VarTable
}

// PredCaps tells which captures of a value-free rule record their values.
type PredCaps interface {
	// Records reports whether the capture records its value.
	Records(c *grammar.Capture) bool
	// Slotted reports whether a capture that records nothing still has the slot of its name.
	Slotted(c *grammar.Capture) bool
}

// Rule is a compiled rule as the builder reads it.
type Rule struct {
	ID   int
	Name string
	// Where, if not "", names the rule in the message of a failed action instead of Name.
	Where string
	Def   *grammar.RuleDef
	// Body is the expression the rule matches.
	Body     grammar.Expr
	Action   grammar.Term
	NodeType string
	// Memo to Vars are the policy of the rule (core.Policy); Lean and Novalue decide whether the
	// body builds its value.
	Memo, Transient, Plain, Rec, Leader, Splice, Novalue, Lean bool
	Seen                                                       int
	Vars                                                       []string
	// Twin is the value-free twin of the rule, if it has one.
	Twin *Rule
	// PredCaps is nil for an ordinary rule.
	PredCaps PredCaps
	// Projections returns the projected repetition captures of the rule (capture name to the field
	// it fills); it is not called if the module builds no projection.
	Projections func() map[string]string
}

// WhereName returns the text that names the rule in the message of a failed action.
func (r *Rule) WhereName() string {
	if r.Where != "" {
		return r.Where
	}
	return r.Name
}
