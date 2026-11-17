package lint

import (
	"context"
	"fmt"

	"zeroparse.dev/zp/internal/analysis/complexity"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// complexity runs the checks that come from the classification of the cost of the grammar g as
// written from start: growing-variable, and super-linear when the classification is requested.
// The classification lowers inline actions itself and describes its findings as written.
func (l *linter) complexity(g *ast.Grammar, start string) {
	growing := !l.opts.Disable[CheckGrowingVariable]
	superLinear := l.opts.Classify && !l.opts.Disable[CheckSuperLinear]
	if !growing && !superLinear {
		return
	}
	ctx := context.Background()
	if l.control != nil {
		ctx = l.control.ctx
	} else if l.opts.values != nil {
		ctx = l.opts.values
	}
	var c *complexity.Complexity
	var err error
	if superLinear {
		c, err = complexity.ClassifyComplexity(ctx, g, start)
	} else {
		c = &complexity.Complexity{}
		c.Growing, err = complexity.GrowingBindings(ctx, g, start)
	}
	if err != nil {
		l.control.check()
		return // a grammar that does not compile is not classified
	}
	defs := map[string]*ast.RuleDef{}
	for _, r := range l.control.rules(g) {
		defs[r.Name] = r
	}
	add := func(check string, sev Severity, rule string, pos ast.Pos, msg string) {
		l.control.poll()
		if r := defs[rule]; r != nil && !pos.IsValid() {
			pos = r.Diagnostic()
		}
		l.findings = append(l.findings, Finding{Pos: pos, Severity: sev, Check: check, Rule: rule, Message: msg})
	}
	if growing {
		for _, b := range c.Growing {
			add(CheckGrowingVariable, Warning, b.Rule, b.Pos, fmt.Sprintf(
				"%s can at least double the size of %s on each recursion or iteration, so that it can grow exponentially with the input",
				b.Text, b.Var))
		}
	}
	if superLinear {
		for _, cause := range c.Causes {
			add(CheckSuperLinear, Hint, cause.Rule, cause.Pos, cause.Message)
		}
	}
}

// Classify classifies the cost of the grammar g from the start rule start ("" selects the default
// start rule) after it checks g with the front end, so that a grammar that does not compile, or a
// start that is not a start rule of g, is an error and not a precondition. Cancellation of ctx
// discards the result.
func Classify(ctx context.Context, g *ast.Grammar, start string) (*complexity.Complexity, error) {
	res, err := checkGrammar(ctx, g)
	if err != nil {
		return nil, err
	}
	if start, err = resolveStart(res, start); err != nil {
		return nil, err
	}
	return complexity.ClassifyComplexity(ctx, g, start)
}
