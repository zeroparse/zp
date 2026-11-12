package closure

import (
	"fmt"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/core"
	ir "zeroparse.dev/zp/internal/ir/core"
)

// termSite is a term of a matcher: a predicate or the value of a binding.
type termSite struct {
	term *core.TermProgram
}

// pendingTerm is a term of the program whose TermProgram the builder makes once the compiler is
// done (Builder.Finish): its Core IR is built from the whole program.
type pendingTerm struct {
	term  grammar.Term
	scope *compile.Scope
	// proj are the projected captures of the rule whose action the term is.
	proj map[string]string
	set  func(*core.TermProgram)
}

// termLater makes the program of the term t, which reads the frame of the scope s, in Finish and
// passes it to set.
func (b *Builder) termLater(t grammar.Term, s *compile.Scope, proj map[string]string, set func(*core.TermProgram)) {
	b.terms = append(b.terms, pendingTerm{term: t, scope: s, proj: proj, set: set})
}

// evaluator returns the evaluator of the term program that set receives.
func evaluator(set *func(*core.TermProgram)) core.Evaluator {
	var t *core.TermProgram
	*set = func(p *core.TermProgram) { t = p }
	return func(c *core.EvalCtx) (any, error) { return t.Eval(c) }
}

// compileTerms makes the programs of the pending terms from the Core IR of the program.
func (b *Builder) compileTerms() error {
	if len(b.terms) == 0 {
		return nil
	}
	terms, err := core.CoreTerms(b.prog)
	if err != nil {
		return err
	}
	for _, p := range b.terms {
		t, ok := terms[p.term]
		if !ok {
			return fmt.Errorf("internal error: no Core IR of the term %s", grammar.FormatTerm(p.term))
		}
		prog, err := compileTerm(t, b.prog, p.scope, p.proj)
		if err != nil {
			return fmt.Errorf("internal error: %w", err)
		}
		p.set(prog)
	}
	b.terms = nil
	return nil
}

func compileTerm(t ir.Term, prog *compile.Program, s *compile.Scope, proj map[string]string) (*core.TermProgram, error) {
	return core.CompileTerm(t, core.TermEnv{Prog: prog, Scope: s, Proj: proj})
}

// isFunction is the operation term.isFunction. A lowered term has no function values: the lambdas
// of map, filter and the folds are evaluated in place.
func isFunction(v any) bool {
	_, ok := v.(core.Function)
	return ok
}
