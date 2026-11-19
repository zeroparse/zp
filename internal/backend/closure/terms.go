package closure

import (
	"fmt"

	"zeroparse.dev/zp/internal/backend/actionlower"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/runtime/state"
)

// termSite is a term of a matcher: a predicate or the value of a binding.
type termSite struct {
	term *state.TermProgram
}

// term makes the program of the Core IR term t, which reads the frame of the current scope; proj
// are the projected captures of the rule whose action t is, nil for every other term.
func (b *Builder) term(t ir.ActionExpr, proj map[string]string) (*state.TermProgram, error) {
	prog, err := actionlower.Compile(t, actionlower.Env{Checker: b.checker, Vars: b.checker.Vars(), Scope: b.scope, Proj: proj})
	if err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}
	return prog, nil
}
