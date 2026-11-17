package lint

import (
	"context"
	"slices"

	"zeroparse.dev/zp/internal/frontend"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// A Rewrite is a change to the source of a grammar that fixes a finding without changing what the
// grammar matches or the values it builds. Only three findings have one, each when the proof for
// its kind holds, and a tool that applies a Rewrite still checks the result:
//
//   - char-class, for ranges that overlap or repeat: the class is replaced by one with merged
//     ranges. The proof is that both classes contain the same code points, checked at the ends of
//     every range, and that negation is unchanged. A class that also matches punctuation between
//     letters, such as [A-z], has no Rewrite, because the writer must say which characters were meant.
//   - useless-lookahead, for a lookahead that always succeeds: it is removed, but only where it is
//     itself an item of a sequence of at least three items, so that the sequence keeps at least two
//     and stays a tuple. Under a discard "-", a label or any operator it is not removed, because the
//     "-" would move onto the next item and change the values. The proof is that the
//     lookahead matches only a combination of literals, character classes, any-character, groups,
//     sequences, choices, options and repetitions, so it holds no capture, cut, call, predicate,
//     recovery or attribute, and a lookahead consumes nothing and has no value.
//   - unreachable-rule: the rule is removed. The proof is that no start rule reaches it, directly
//     or through an inline action, so nothing calls it and no parse starts at it.
//
// Start and End delimit the source to replace by NewText, with End exclusive. For a rule the span is
// the whole definition including its attributes.
type Rewrite struct {
	Start, End ast.Pos
	NewText    string
}

// IsValid reports whether the Rewrite holds a change.
func (r Rewrite) IsValid() bool { return r.Start.IsValid() && r.End.IsValid() }

// Rewrites returns, for each finding of the lint run of g from start, its Rewrite or the zero
// Rewrite. The findings must come from Config.Run on the same g and start. Rewrites returns an
// error, and no rewrites, if g does not compile or start is not a start rule of g. Cancellation
// discards the result.
func Rewrites(ctx context.Context, g *ast.Grammar, start string, findings []Finding) ([]Rewrite, error) {
	res, err := checkGrammar(ctx, g)
	if err != nil {
		return nil, err
	}
	if _, err := resolveStart(res, start); err != nil {
		return nil, err
	}
	return rewritesOf(ctx, res, g, findings)
}

// rewritesOf is Rewrites for a grammar the front end checked to the result res.
func rewritesOf(ctx context.Context, res *frontend.Result, g *ast.Grammar, findings []Finding) (out []Rewrite, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var control *lintControl
	if ctx.Done() != nil {
		control = &lintControl{ctx: ctx}
		defer func() {
			if failure := recover(); failure != nil {
				cancelled, ok := failure.(lintCancellation)
				if !ok {
					panic(failure)
				}
				out, err = nil, cancelled.err
			}
		}()
	}
	lowered, _, lowerErr := desugar.LowerInlineActions(g)
	if lowerErr != nil {
		lowered = g
	}
	l := &linter{control: control, an: newAnalysisControlled(lowered, control, tablesOf(res))}
	out = make([]Rewrite, len(findings))
	for i, f := range findings {
		control.poll()
		if !f.Pos.IsValid() {
			continue
		}
		switch f.Check {
		case CheckCharClass:
			out[i] = l.classRewrite(f)
		case CheckUselessLookahead:
			out[i] = l.lookaheadRewrite(f)
		case CheckUnreachableRule:
			out[i] = l.ruleRewrite(f)
		}
	}
	return out, nil
}

// find returns the expression of the rules at pos for which match holds.
func (l *linter) find(pos ast.Pos, match func(ast.Expr) bool) ast.Expr {
	control := l.control
	var found ast.Expr
	for _, r := range l.an.order {
		control.poll()
		walkExprControlled(r.Expr, func(e ast.Expr) {
			control.poll()
			if n, ok := e.(ast.SourceNode); ok && found == nil && n.Source().Diagnostic() == pos && match(e) {
				found = e
			}
		}, control)
		if found != nil {
			break
		}
	}
	return found
}

func (l *linter) classRewrite(f Finding) Rewrite {
	control := l.control
	c, _ := l.find(f.Pos, func(e ast.Expr) bool { _, ok := e.(*ast.CharClass); return ok }).(*ast.CharClass)
	if c == nil || !c.End.IsValid() {
		return Rewrite{}
	}
	overlaps := false
	for j, rj := range c.Ranges {
		control.poll()
		for _, ri := range c.Ranges[:j] {
			control.poll()
			overlaps = overlaps || ri.Lo <= rj.Hi && rj.Lo <= ri.Hi
		}
	}
	if !overlaps {
		return Rewrite{}
	}
	merged := &ast.CharClass{Negated: c.Negated, Ranges: normalizeControlled(c.Ranges, control)}
	if !sameCodePoints(c.Ranges, merged.Ranges, control) {
		return Rewrite{}
	}
	return Rewrite{Start: c.Pos, End: c.End, NewText: control.formatExpr(merged)}
}

// sameCodePoints reports whether the ranges x and y contain the same code points. Two unions of
// intervals are equal when they agree on every end of every range and on the code point on each
// side of it, since they can differ only next to such a point.
func sameCodePoints(x, y []ast.CharRange, control *lintControl) bool {
	in := func(rs []ast.CharRange, r rune) bool {
		for _, rg := range rs {
			control.poll()
			if rg.Lo <= r && r <= rg.Hi {
				return true
			}
		}
		return false
	}
	for _, rs := range [][]ast.CharRange{x, y} {
		for _, rg := range rs {
			for _, p := range []rune{rg.Lo - 1, rg.Lo, rg.Hi, rg.Hi + 1} {
				control.poll()
				if in(x, p) != in(y, p) {
					return false
				}
			}
		}
	}
	return true
}

func (l *linter) lookaheadRewrite(f Finding) Rewrite {
	control := l.control
	a := l.an
	// The lookahead itself, not a node that wraps it and starts at the same place.
	e := l.find(f.Pos, func(e ast.Expr) bool {
		switch e.(type) {
		case *ast.And, *ast.Not:
			return true
		}
		return false
	})
	if e == nil || !l.seqItem(e) {
		return Rewrite{}
	}
	var body ast.Expr
	switch x := e.(type) {
	case *ast.And:
		if !hasCaptures(x.Expr, control) && a.succeeds(x.Expr) {
			body = x.Expr
		}
	case *ast.Not:
		if !a.succeeds(x.Expr) && !a.mayMatch(x.Expr) {
			body = x.Expr
		}
	}
	node, ok := e.(ast.SourceNode)
	if !ok || body == nil || !pureMatcher(body, control) {
		return Rewrite{}
	}
	src := node.Source()
	if !src.End.IsValid() {
		return Rewrite{}
	}
	return Rewrite{Start: src.Pos, End: src.End}
}

// seqItem reports whether e is itself an item of a sequence of at least three items, so that at
// least two remain and the sequence stays a tuple. A lookahead that
// another construct holds is not: under a discard "-", deleting it would move the "-" onto the next
// item, and a label, an operator or a repetition would be left without its operand.
func (l *linter) seqItem(e ast.Expr) bool {
	control := l.control
	found := false
	for _, r := range l.an.order {
		control.poll()
		walkExprControlled(r.Expr, func(x ast.Expr) {
			control.poll()
			if seq, ok := x.(*ast.Seq); ok && len(seq.Items) >= 3 {
				found = found || slices.ContainsFunc(seq.Items, func(it ast.Expr) bool { control.poll(); return it == e })
			}
		}, control)
	}
	return found
}

// pureMatcher reports whether e is made only of literals, character classes, any-character, groups,
// sequences, choices, options and repetitions.
func pureMatcher(e ast.Expr, control *lintControl) bool {
	pure := true
	walkExprControlled(e, func(x ast.Expr) {
		control.poll()
		switch x.(type) {
		case *ast.Literal, *ast.CharClass, *ast.Any, *ast.Group, *ast.Seq,
			*ast.Choice, *ast.Optional, *ast.Repeat:
		default:
			pure = false
		}
	}, control)
	return pure
}

func (l *linter) ruleRewrite(f Finding) Rewrite {
	control := l.control
	i := slices.IndexFunc(l.an.order, func(r *ast.RuleDef) bool { control.poll(); return r.Name == f.Rule })
	if i < 0 || len(l.an.order) < 2 {
		return Rewrite{}
	}
	r := l.an.order[i]
	if !r.End.IsValid() || r.Pos != f.Pos && r.Diagnostic() != f.Pos {
		return Rewrite{}
	}
	return Rewrite{Start: r.Pos, End: r.End}
}
