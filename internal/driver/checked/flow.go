package checked

import (
	"fmt"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/driver"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/internal/middleend/flowpass"
	"zeroparse.dev/zp/internal/middleend/flowplan"
)

// FlowTable is the Node table of a program as the bytecode VM runs it from Flow IR: the table's Core
// IR and lowering options, its plan, and the Flow function of each rule body that the plan holds.
type FlowTable struct {
	IR   *flowplan.TableIR
	Plan *flowplan.Table
	// Bodies maps the ID of each rule whose body the plan holds to the function the table runs for
	// it, after the Flow passes: the rule's flat function, or its framed body where the plan has no
	// flat function or refuses it (flowplan.Refused).
	Bodies map[int]*code.Module
}

// FlowTable builds the Node table of the program the way the generated Go parser with budget
// controls builds its Node table, so that the VM runs the same functions: the same Core IR and
// lowering options (flowplan.BuildTable), the same plan (flowplan.Plan) and the Core, Flow and Exec
// steps of the driver at the default optimizations, the Flow passes for the VM's target
// (flowpass.Bytecode).
func (s *Source) FlowTable() (*FlowTable, error) {
	p := s.prog
	d := &driver.Driver{}
	t, err := flowplan.BuildTable(p, flowplan.TableConfig{Values: true, Budget: true}, func(build func() (*core.Module, error)) (*core.Module, error) {
		return d.Core("rules", build)
	})
	if err != nil {
		return nil, fmt.Errorf("internal error: Core IR: %w", err)
	}
	rules := append(append([]*compile.Rule(nil), p.RuleList...), p.Twins...)
	plan := flowplan.Plan(t.M, planRules(rules), nil)
	ft := &FlowTable{IR: t, Plan: plan, Bodies: map[int]*code.Module{}}
	lower := func(spec *flowplan.FuncSpec) (*code.Module, error) {
		f, err := d.FlowFunc("rules", t.Unit(spec.Func), false, flowpass.Bytecode)
		if err != nil {
			return nil, fmt.Errorf("function %s of table rules: %w", spec.Name, err)
		}
		return f, nil
	}
	for _, r := range rules {
		spec, ok := plan.Func(flowplan.Key{Use: flowplan.RuleBody, Rule: r.ID})
		if !ok {
			continue
		}
		if spec.Form == flowlower.Flat {
			refused, err := refused(d, lower, spec)
			if err != nil {
				return nil, err
			}
			if refused {
				if spec = spec.Fallback; spec == nil {
					continue // the table cannot run the rule from Flow IR
				}
			}
		}
		f, err := lower(spec)
		if err != nil {
			return nil, err
		}
		ft.Bodies[r.ID] = f
	}
	return ft, nil
}

// refused reports whether the plan refuses the flat function spec (flowplan.Refused), which it lowers
// with lower and converts to the Exec stage, after the Exec passes of the driver d, as a printer would.
func refused(d *driver.Driver, lower func(*flowplan.FuncSpec) (*code.Module, error), spec *flowplan.FuncSpec) (bool, error) {
	f, err := lower(spec)
	if err != nil {
		return false, err
	}
	l, err := d.ExecFunc("rules", f, noTarget, false)
	if err != nil {
		return false, fmt.Errorf("function %s of table rules: %w", spec.Name, err)
	}
	return flowplan.Refused(l), nil
}

// noTarget is the pipeline of the Exec passes of a target that has none of its own: the refusal is
// decided after the Exec passes that every target runs.
var noTarget = &driver.TargetPipeline{}

// planRules returns what the plan of a table reads of the compiled rules (flowplan.Rule).
func planRules(rules []*compile.Rule) []*flowplan.Rule {
	out := make([]*flowplan.Rule, len(rules))
	for i, r := range rules {
		out[i] = &flowplan.Rule{ID: r.ID, Name: r.Name, Body: r.Def.Expr, Action: r.Action, Plain: r.Plain, Lean: r.Lean,
			Novalue: r.Novalue, Leader: r.Leader}
	}
	return out
}
