// Package starttest lets tests whose subject is not start rules enter any rule
// of a grammar.
package starttest

import (
	"strings"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

// MarkAll marks every rule of g that is not hidden @start, and the first of
// them @start(default) when there are several, unless a rule of g already
// has declaration attributes, and returns g. A hidden rule, whose name begins
// with an underscore, cannot be a start rule. Only the syntax tree changes:
// source positions stay where they were, so the positions tests expect in
// diagnostics do not move.
func MarkAll(g *ast.Grammar) *ast.Grammar {
	if g == nil {
		return nil
	}
	var marked []*ast.RuleDef
	for _, rd := range g.Rules() {
		if len(rd.Attrs) > 0 {
			return g
		}
		if !strings.HasPrefix(rd.Name, "_") {
			marked = append(marked, rd)
		}
	}
	for i, rd := range marked {
		a := &ast.Attribute{Name: "start"}
		if i == 0 && len(marked) > 1 {
			a.Args = []any{&ast.Ref{Name: "default"}}
		}
		rd.Attrs = []*ast.Attribute{a}
	}
	return g
}
