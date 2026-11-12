package check

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
)

// The functions below let the tests of internal/compile reach checker internals that its tests
// reached while the checker lived there.

// PollPredicates checks the predicates of e with an empty environment, polling control.
func PollPredicates(control *cancel.Control, e grammar.Expr) {
	(&ValueInference{control: control}).predicates(e, &valueEnv{})
}

// NewShownInfo returns the checked types of a program whose rules show the types shown.
func NewShownInfo(shown map[string]Ty) *TypeInfo { return &TypeInfo{shown: shown} }
