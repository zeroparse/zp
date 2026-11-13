package closure

import (
	"fmt"

	"zeroparse.dev/zp/internal/engine/state"
	ir "zeroparse.dev/zp/internal/ir/core"
)

// termSite is a term of a matcher: a predicate or the value of a binding.
type termSite struct {
	term *state.TermProgram
}

// term makes the program of the Core IR term t, which reads the frame of the current scope; proj
// are the projected captures of the rule whose action t is, nil for every other term.
func (b *Builder) term(t ir.Term, proj map[string]string) (*state.TermProgram, error) {
	prog, err := state.CompileTerm(t, state.TermEnv{Prog: b.prog, Scope: b.scope, Proj: proj})
	if err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}
	return prog, nil
}

// isFunction is the operation term.isFunction. A lowered term has no function values: the lambdas
// of map, filter and the folds are evaluated in place.
func isFunction(v any) bool {
	_, ok := v.(state.Function)
	return ok
}
