package frontend

import (
	"slices"

	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/diagnostic"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/facts"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// structuralErrors returns the errors that depend on the facts of g: they read the structure of the
// grammar only, so they are found on an untyped module, before any type is checked. It returns nil
// if g cannot be built into such a module (the compiler then reports why); the errors are in the
// order of Facts.Errors, with the names of the hidden rules that inline actions generate shown as
// written.
func structuralErrors(g *ast.Grammar, opts compile.Options, names desugar.InlineNames, control *cancel.Control) diagnostic.ErrorList {
	canon, err := compile.Canonicalize(g, opts, names, control)
	if err != nil {
		return nil
	}
	defs, _ := desugar.UniqueRules(canon.Grammar, control)
	in := compile.StructuralInput(canon.Grammar, defs, canon.Starts)
	origin := map[core.Node]ast.Expr{}
	m, err := elaborate.Structural(in, origin)
	if err != nil {
		return nil
	}
	f := facts.Compute(m, control)
	var out diagnostic.ErrorList
	for _, e := range f.Errors(m, !opts.LeftRecursionBindings) {
		var pos ast.Pos
		switch x := origin[e.Node].(type) {
		case *ast.Predicate:
			pos = x.Term.(*ast.Bind).Diagnostic()
		case *ast.Attributed:
			pos = x.Attrs[0].Diagnostic()
		default:
			pos = defs[e.Rule.ID].Diagnostic()
		}
		out = append(out, &diagnostic.Error{Pos: pos, Msg: e.Msg})
	}
	if len(out) == 0 {
		return nil
	}
	return names.Rewrite(out).(diagnostic.ErrorList)
}

// mergeErrors returns the compiler's error with the fact errors it does not report. The compiler's
// errors keep their order; each added error goes before the first error already in the list that
// comes after it in the source, or last. If the compiler reports all of them, it returns the
// compiler's error unchanged.
func mergeErrors(err error, factErrs diagnostic.ErrorList) error {
	list, ok := err.(diagnostic.ErrorList)
	if !ok || len(factErrs) == 0 {
		return err
	}
	merged := slices.Clone(list)
	for _, fe := range factErrs {
		if slices.ContainsFunc(list, func(e *diagnostic.Error) bool { return *e == *fe }) {
			continue
		}
		i := slices.IndexFunc(merged, func(e *diagnostic.Error) bool { return before(fe.Pos, e.Pos) })
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
func before(a, b ast.Pos) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col
}
