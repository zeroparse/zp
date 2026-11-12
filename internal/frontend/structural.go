package frontend

import (
	"slices"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/check"
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
	in, defs := structuralInput(canon, control)
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

// structuralInput returns the input of the untyped module of the canonical grammar and its rule
// definitions by ID: the rules once each, as the compiler takes them.
func structuralInput(canon *compile.Canonical, control *cancel.Control) (*build.Input, []*grammar.RuleDef) {
	rules, _ := desugar.UniqueRules(canon.Grammar, control)
	in := &build.Input{ByName: map[string]*build.Rule{}}
	for i, rd := range rules {
		_, operators := rd.Expr.(*grammar.OperatorsRule)
		r := &build.Rule{ID: i, Name: rd.Name, Def: rd, Body: rd.Expr, Action: rd.Action,
			Splice: desugar.HiddenRule(rd.Name) && rd.Action == nil && !rd.DefaultAction && !operators}
		in.Rules = append(in.Rules, r)
		in.ByName[rd.Name] = r
	}
	for _, name := range canon.Starts {
		if r := in.ByName[name]; r != nil {
			in.Starts = append(in.Starts, r)
		}
	}
	for _, d := range canon.Grammar.File.Decls {
		if vd, ok := d.(*grammar.VarDef); ok {
			if in.Vars == nil {
				in.Vars = check.NewVarTable()
			}
			if in.Vars.Lookup(vd.Name) == nil {
				in.Vars.Add(&check.VarDecl{Name: vd.Name})
			}
		}
	}
	return in, rules
}

// mergeErrors returns the compiler's error with the fact errors it does not report, in the order of
// their positions. If the compiler reports all of them, it returns the compiler's error unchanged.
func mergeErrors(err error, factErrs diag.ErrorList) error {
	list, ok := err.(diag.ErrorList)
	if !ok || len(factErrs) == 0 {
		return err
	}
	merged := slices.Clone(list)
	for _, fe := range factErrs {
		if !slices.ContainsFunc(list, func(e *diag.Error) bool { return *e == *fe }) {
			merged = append(merged, fe)
		}
	}
	if len(merged) == len(list) {
		return err
	}
	slices.SortStableFunc(merged, func(a, b *diag.Error) int {
		if a.Pos.Line != b.Pos.Line {
			return a.Pos.Line - b.Pos.Line
		}
		return a.Pos.Col - b.Pos.Col
	})
	return merged
}
