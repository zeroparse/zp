package frontend

import (
	"slices"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/diag"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/core/facts"
)

// structuralErrors returns the errors that depend on the facts of g: they read the structure of the
// grammar only, so they are found on an untyped module, before any type is checked. It returns nil
// if g cannot be built into such a module (the compiler then reports why); the errors are in the
// order of Facts.Errors, with the names of the hidden rules that inline actions generate shown as
// written.
func structuralErrors(g *grammar.Grammar, opts compile.Options, names desugar.InlineNames, control *cancel.Control) diag.ErrorList {
	canon, err := compile.Canonicalize(g, opts, names, control)
	if err != nil {
		return nil
	}
	defs, _ := desugar.UniqueRules(canon.Grammar, control)
	in := compile.StructuralInput(canon.Grammar, defs, canon.Starts)
	origin := map[core.Node]grammar.Expr{}
	m, err := build.Structural(in, origin)
	if err != nil {
		return nil
	}
	f := facts.Compute(m, control)
	var out diag.ErrorList
	for _, e := range f.Errors(m, !opts.LeftRecursionBindings) {
		var pos grammar.Pos
		switch x := origin[e.Node].(type) {
		case *grammar.Predicate:
			pos = x.Term.(*grammar.Bind).Diagnostic()
		case *grammar.Attributed:
			pos = x.Attrs[0].Diagnostic()
		default:
			pos = defs[e.Rule.ID].Diagnostic()
		}
		out = append(out, &diag.Error{Pos: pos, Msg: e.Msg})
	}
	if len(out) == 0 {
		return nil
	}
	return names.Rewrite(out).(diag.ErrorList)
}

// mergeErrors returns the compiler's error with the fact errors it does not report. The compiler's
// errors keep their order; each added error goes before the first error already in the list that
// comes after it in the source, or last. If the compiler reports all of them, it returns the
// compiler's error unchanged.
func mergeErrors(err error, factErrs diag.ErrorList) error {
	list, ok := err.(diag.ErrorList)
	if !ok || len(factErrs) == 0 {
		return err
	}
	merged := slices.Clone(list)
	for _, fe := range factErrs {
		if slices.ContainsFunc(list, func(e *diag.Error) bool { return *e == *fe }) {
			continue
		}
		i := slices.IndexFunc(merged, func(e *diag.Error) bool { return before(fe.Pos, e.Pos) })
		if i < 0 {
			i = len(merged)
		}
		merged = slices.Insert(merged, i, fe)
	}
	if len(merged) == len(list) {
		return err
	}
	return merged
}

// before reports whether a comes before b in the source.
func before(a, b grammar.Pos) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col
}
