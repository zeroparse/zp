package closure

import (
	"fmt"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/core"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
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
	terms, err := coreTerms(b.prog)
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

// coreTerms builds the Core IR of the program and returns the Core IR term of each term of its
// rules: the actions of the rules and of the lines of Operators Rules, the predicates and the
// values of bindings.
func coreTerms(prog *compile.Program) (map[grammar.Term]ir.Term, error) {
	origin := map[ir.Node]grammar.Expr{}
	m, err := build.CoreFrom(prog, origin)
	if err != nil {
		return nil, fmt.Errorf("internal error: Core IR: %w", err)
	}
	terms := map[grammar.Term]ir.Term{}
	add := func(g grammar.Term, t ir.Term) {
		if g != nil && t != nil {
			if _, ok := terms[g]; !ok {
				terms[g] = t
			}
		}
	}
	all := append(append([]*compile.Rule(nil), prog.RuleList...), prog.Twins...)
	for _, cr := range m.Rules {
		if cr == nil {
			continue
		}
		if cr.ID < len(all) && all[cr.ID] != nil {
			sr := all[cr.ID]
			add(sr.Action, cr.Finish.Action)
			if o, ok := cr.Body.(*ir.OperatorsRule); ok {
				if g, ok := sr.Def.Expr.(*grammar.OperatorsRule); ok {
					for i, l := range o.Operands {
						add(g.Operands[i].Action, l.Action)
					}
					i := 0
					for _, level := range g.Levels {
						for _, x := range level.Operators {
							add(x.Action, o.Ops[i].Line.Action)
							i++
						}
					}
				}
			}
		}
		ir.Walk(cr.Body, func(n ir.Node) {
			e, ok := origin[n].(*grammar.Predicate)
			if !ok {
				return
			}
			switch n := n.(type) {
			case *ir.Pred:
				add(e.Term, n.Term)
			case *ir.BindNode:
				if a, ok := e.Term.(*grammar.Bind); ok {
					add(a.Value, n.Term)
				}
			}
		})
	}
	return terms, nil
}

// isFunction is the operation term.isFunction. A lowered term has no function values: the lambdas
// of map, filter and the folds are evaluated in place.
func isFunction(v any) bool {
	_, ok := v.(core.Function)
	return ok
}
