package engine

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/diag"
)

// linearRewrites is the recipe that shows how to rewrite the constructs whose cost the classifier
// cannot prove linear.
const linearRewrites = "https://zeroparse.dev/docs/cookbook/linear-rewrites/"

// checkDeclaredLinear checks the declarations of linear cost of g, which compiles: @linear on the
// package clause declares every start rule linear, and @linear on a rule declares that rule linear,
// classified as if parsing began at it. Each declaration covers every rule its start reaches. A
// declared start whose class is not linear is an error at each cause and reason of its
// classification, and, for the class linear given an environment bound, at each reason for the
// bound. A grammar without @linear costs only a scan of its declaration attributes.
func checkDeclaredLinear(g *grammar.Grammar, control *cancel.Control) error {
	whole, rules, errs := desugar.LinearDeclarations(g, control)
	if len(errs) > 0 {
		return errs // compilation reports them first; kept for a grammar that skipped it
	}
	if !whole && len(rules) == 0 {
		return nil
	}
	defs := map[string]*grammar.RuleDef{}
	for _, rd := range control.Rules(g) {
		if _, ok := defs[rd.Name]; !ok {
			defs[rd.Name] = rd
		}
	}
	type key struct {
		pos grammar.Pos
		msg string
	}
	seen := map[key]bool{}
	var out diag.ErrorList
	check := func(subject string, at grammar.Pos, c *compile.Complexity) {
		report := func(cause compile.ComplexityCause, format string) {
			control.Poll()
			pos := cause.Pos
			if !pos.IsValid() {
				pos = at
				if rd := defs[cause.Rule]; rd != nil {
					pos = rd.Diagnostic()
				}
			}
			k := key{pos, cause.Message}
			if seen[k] {
				return // another declaration reported the same cause
			}
			seen[k] = true
			out = append(out, &diag.Error{Pos: pos, Msg: fmt.Sprintf(format, subject, cause.Message, linearRewrites)})
		}
		switch c.Class {
		case compile.ClassLinear:
			return
		case compile.ClassLinearEnv:
			if len(c.Env) == 0 {
				break
			}
			for _, env := range c.Env {
				report(env, "%s is declared @linear, but its cost is linear only given a bound on the values of the variables: %s; keep the values of the variables bounded (see %s)")
			}
			return
		default:
			if len(c.Causes)+len(c.Reasons) == 0 {
				break
			}
			for _, cause := range c.Causes {
				report(cause, "%s is declared @linear, but %s; rewrite it so that the cost stays linear (see %s)")
			}
			for _, reason := range c.Reasons {
				report(reason, "%s is declared @linear, but the classifier cannot prove its cost linear: %s; rewrite it so that the cost stays linear (see %s)")
			}
			return
		}
		// A class other than linear has a cause or a reason; this is a guard, not a case.
		out = append(out, &diag.Error{Pos: at, Msg: fmt.Sprintf("%s is declared @linear, but its cost is %s (see %s)", subject, c.Class, linearRewrites)})
	}
	classify := func(g *grammar.Grammar, start string) (*compile.Complexity, error) {
		return compile.ClassifyComplexity(g, start, control)
	}
	starts, _, _ := desugar.ResolveStarts(control.Rules(g), control)
	if whole {
		for _, start := range starts {
			c, err := classify(g, start)
			if err != nil {
				return err
			}
			check("the grammar from start rule "+start, g.File.Pkg.Diagnostic(), c)
		}
	}
	for _, name := range rules {
		control.Poll()
		rd := defs[name]
		var c *compile.Complexity
		var err error
		if slices.Contains(starts, name) {
			c, err = classify(g, name)
		} else {
			c, err = classify(startingAt(g, name, defs), "")
		}
		if err != nil {
			return err
		}
		check("rule "+name, rd.Diagnostic(), c)
	}
	if len(out) == 0 {
		return nil
	}
	slices.SortStableFunc(out, func(a, b *diag.Error) int {
		return cmp.Or(cmp.Compare(a.Pos.Line, b.Pos.Line), cmp.Compare(a.Pos.Col, b.Pos.Col))
	})
	return out
}

// startingAt returns a copy of g whose default start rule calls rule, so that a classification of
// the copy from its default start rule classifies parsing that begins at rule. The rules of g are
// shared, except that @start(default) is copied as @start: the start rules stay marked, so that
// the analyses that set a marked rule apart see the rules as compiled.
func startingAt(g *grammar.Grammar, rule string, defs map[string]*grammar.RuleDef) *grammar.Grammar {
	name := "linear_start"
	for i := 2; defs[name] != nil; i++ {
		name = "linear_start" + strconv.Itoa(i)
	}
	starting := *g
	starting.File.Decls = make([]grammar.Decl, 0, len(g.File.Decls)+1)
	for _, d := range g.File.Decls {
		if rd, ok := d.(*grammar.RuleDef); ok && desugar.StartMarked(rd) {
			unmarked := *rd
			unmarked.Attrs = make([]*grammar.Attribute, len(rd.Attrs))
			for i, a := range rd.Attrs {
				if !a.Pragma && a.Name == "start" {
					a = &grammar.Attribute{SourceInfo: a.SourceInfo, Name: "start"}
				}
				unmarked.Attrs[i] = a
			}
			d = &unmarked
		}
		starting.File.Decls = append(starting.File.Decls, d)
	}
	starting.File.Decls = append(starting.File.Decls, &grammar.RuleDef{
		Attrs: []*grammar.Attribute{{Name: "start", Args: []any{&grammar.Ref{Name: "default"}}}},
		Name:  name,
		Expr:  &grammar.Ref{Name: rule},
	})
	return &starting
}
