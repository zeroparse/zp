// Package complexity builds the cost view of a grammar: a Core IR module derived from the
// untyped module of the grammar's rules by the normalization rules of
// spec/proofs/grammar-laws.md, which the complexity classifier reads in place of the rules as
// written (spec/core-ir/cost-view.md).
package complexity

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/facts"
)

// Law names a normalization rule of the cost view.
type Law string

const (
	N1  Law = "N1"  // splice groups and sequences, flatten choices (S1, C2)
	N2  Law = "N2"  // drop the alternatives after one that always succeeds (C3)
	N3  Law = "N3"  // drop duplicate and subsumed alternatives (C1, C5)
	N4  Law = "N4"  // drop bottom alternatives, cut sequences after a leading bottom (C4, S2)
	N5  Law = "N5"  // canonicalize stacked lookaheads (K1, K2)
	N6  Law = "N6"  // drop harmless cuts and recoveries that never fire (T2, V1)
	N7  Law = "N7"  // drop labels: @error and @diagnostic
	N8  Law = "N8"  // merge runs of single-character alternatives (C9)
	N9  Law = "N9"  // merge repetitions in a row (R3, R4, R5)
	N10 Law = "N10" // flatten nested repetitions (R6)
	N11 Law = "N11" // unroll X* (Y X*)* and absorb X* (X / Y)* (R7)
	N12 Law = "N12" // replace !X X by bottom, left-factor alternatives (K4, C6)
	N13 Law = "N13" // inline calls (P1)
)

// Options selects the rules of a view. The zero value is the view the complexity classifier
// reads: every rule but N7 and N13.
type Options struct {
	// Labels applies N7. It changes the expectations and the run sites of the module.
	Labels bool
	// Inline applies N13. It changes which evaluations the memo answers.
	Inline bool
	// Laws, if not nil, holds the only rules that apply, for comparing views that apply some rules
	// only. N7 and N13 apply only if Labels and Inline select them, too.
	Laws []Law
}

// Source is what a view is built from: an untyped Core IR module (build.Structural), the
// grammar expression each of its nodes is built from, and the facts of the module.
type Source struct {
	Module *core.Module
	Origin map[core.Node]grammar.Expr
	Facts  *facts.Facts
}

// View is the cost view of a module. Its rules are those of the module, in the same order and
// with the same IDs, with their bodies rewritten. A node that no rule changes is shared with the
// module; a node that a rule builds is new.
type View struct {
	Module *core.Module
	// Origin is the origin map of the source: the expression of the grammar each shared node is
	// built from. A node the view builds has none.
	Origin map[core.Node]grammar.Expr
	// Text gives, for a node that the view builds, the source text of the expressions that it
	// stands for, so that messages quote the grammar as it is written.
	Text map[core.Node]string
	// Applied counts the rewrites by rule.
	Applied map[Law]int
	// replaces gives, for a node that the view builds by giving a node new parts, the node of the
	// module that it replaces (Grammar). A node that the view builds has no attributes.
	replaces map[core.Node]core.Node
	// chains gives, for each node of the module that one attribute of an expression of several
	// attributes is built into, its place in that expression (attrChains).
	chains map[core.Node]chainLink
	// at gives, for a node that the view builds, the node of the module whose position it takes:
	// that of the expression it rewrites, or of the first of those it merges.
	at map[core.Node]core.Node
}

// Build returns the view of src with opts.
func Build(src *Source, opts Options, control *cancel.Control) *View {
	m := *src.Module
	m.Rules = make([]*core.Rule, len(src.Module.Rules))
	v := &View{Module: &m, Origin: src.Origin, Text: map[core.Node]string{}, Applied: map[Law]int{},
		replaces: map[core.Node]core.Node{}, chains: attrChains(src), at: map[core.Node]core.Node{}}
	b := &builder{src: src, opts: opts, control: control, view: v, orig: map[core.Node]bool{},
		values: valueReaders(src.Module)}
	for _, r := range src.Module.Rules {
		core.Walk(r.Body, func(n core.Node) { b.orig[n] = true })
	}
	b.cheapRules()
	for i, r := range src.Module.Rules {
		control.Poll()
		b.cur, b.depth = i, 0
		if b.values.valued[i] {
			b.depth = 1
		}
		cp := *r
		cp.Body = b.fix(r.Body)
		m.Rules[i] = &cp
	}
	return v
}

// textOf returns the source text that the node n of the view stands for.
func (v *View) textOf(n core.Node) string {
	if t, ok := v.Text[n]; ok {
		return t
	}
	if e, ok := v.Origin[n]; ok {
		return grammar.FormatExpr(e)
	}
	if e, err := v.expr(n); err == nil {
		return grammar.FormatExpr(e)
	}
	return ""
}
