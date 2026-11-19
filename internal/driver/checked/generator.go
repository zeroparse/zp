package checked

import (
	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Generator returns the view of the checked program that the generators read (common.Program): the
// semantic model, the checker and the run-time program of the checked program, and a view of each
// rule over its run-time rule, with the facts that only the checked program holds copied (Lean, the
// twin and the projections).
func (s *Source) Generator() *common.Program {
	p := s.prog
	run := p.Runtime()
	v := &common.Program{Model: p.Model, Check: p.Checker(), Typed: p.Typed, Run: run, Checked: p,
		RuleList: make([]*common.Rule, len(p.RuleList)), Twins: make([]*common.Rule, len(p.Twins)),
		ByName: make(map[string]*common.Rule, len(p.RuleList))}
	view := func(r *compile.Rule) *common.Rule {
		rt := run.RuleList
		id := r.ID
		if id >= len(rt) {
			rt, id = run.Twins, id-len(rt)
		}
		return &common.Rule{Rule: rt[id], Def: r.Def, Action: r.Action, Captures: r.Scope, Lean: r.Lean,
			Projections: compile.RuleProjections(p, r)}
	}
	for i, r := range p.RuleList {
		v.RuleList[i] = view(r)
		v.ByName[r.Name] = v.RuleList[i]
	}
	for i, r := range p.Twins {
		v.Twins[i] = view(r)
	}
	for i, r := range p.RuleList {
		if r.Twin != nil {
			v.RuleList[i].Twin = v.Twins[r.Twin.ID-len(p.RuleList)]
		}
	}
	return v
}

// CompileOptions returns the options of the compiler that the choices o of a generator stand for.
func CompileOptions(o common.CompileOptions) compile.Options {
	return compile.Options{SourceInfo: o.SourceInfo, DefaultValue: o.DefaultValue, LeftRecursionBindings: o.LeftRecursionBindings}
}

// GeneratorCompiler returns the compiler of the generators (common.Compiler) that checks a grammar
// with c, which returns the checked program and a function that returns that of its recognizer, and
// gives the generators their views (Source.Generator).
func GeneratorCompiler(c func(g *ast.Grammar, opts compile.Options) (*compile.Program, func() (*compile.Program, error), error)) common.Compiler {
	return func(g *ast.Grammar, opts common.CompileOptions) (*common.Program, func() (*common.Program, error), error) {
		prog, rec, err := c(g, CompileOptions(opts))
		if err != nil {
			return nil, nil, err
		}
		recognizer := func() (*common.Program, error) {
			r, err := rec()
			if err != nil {
				return nil, err
			}
			return Of(r).Generator(), nil
		}
		return Of(prog).Generator(), recognizer, nil
	}
}
