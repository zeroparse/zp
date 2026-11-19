package checked

import (
	"fmt"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/driver"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/flowpass"
	"zeroparse.dev/zp/internal/middleend/flowplan"
)

// FlowTable is the Node table of a program as the bytecode VM runs it from Flow IR: the table's Core
// IR and lowering options, its plan, and the Flow function of each rule body that the plan holds.
type FlowTable struct {
	IR   *flowplan.TableIR
	Plan *flowplan.Table
	// Bodies maps the ID of each rule whose body the plan holds to the function the table runs for
	// it, after the Flow passes: the rule's flat function or its framed body, as the plan decides.
	Bodies map[int]*code.Module
}

// FlowTable builds the Node table of the program the way the generated Go parser with budget
// controls builds its Node table, so that the VM runs the same functions: the same Core IR and
// lowering options (flowplan.BuildTable), the same plan, with its decisions (flowplan.Plan), and
// the Core and Flow passes of the driver at the default optimizations, for the VM's target
// (flowpass.Bytecode).
func (s *Source) FlowTable() (*FlowTable, error) {
	p := s.prog
	d := &driver.Driver{}
	t, err := flowplan.BuildTable(p, flowplan.TableConfig{Values: true, Budget: true}, func(m *core.Module, prepare func(*core.Module) func()) error {
		return d.RunCore("rules", m, prepare)
	})
	if err != nil {
		return nil, fmt.Errorf("internal error: Core IR: %w", err)
	}
	rules := append(append([]*compile.Rule(nil), p.RuleList...), p.Twins...)
	plan, err := flowplan.Plan(t, planRules(rules), nil, d.Opt)
	if err != nil {
		return nil, err
	}
	ft := &FlowTable{IR: t, Plan: plan, Bodies: map[int]*code.Module{}}
	for _, r := range rules {
		spec, ok := plan.Func(flowplan.Key{Use: flowplan.RuleBody, Rule: r.ID})
		if !ok {
			continue // the table cannot run the rule from Flow IR
		}
		f, err := spec.Lower(t.M, t.Opts)
		if err != nil {
			return nil, fmt.Errorf("internal error: flow of %s: %w", spec.Name, err)
		}
		if err := d.RunFlow("rules", f, false, flowpass.Bytecode); err != nil {
			return nil, fmt.Errorf("function %s of table rules: %w", spec.Name, err)
		}
		ft.Bodies[r.ID] = f
	}
	return ft, nil
}

// planRules returns what the plan of a table reads of the compiled rules (flowplan.Rule).
func planRules(rules []*compile.Rule) []*flowplan.Rule {
	out := make([]*flowplan.Rule, len(rules))
	for i, r := range rules {
		out[i] = &flowplan.Rule{ID: r.ID, Name: r.Name, Body: r.Def.Expr, Action: r.Action, Plain: r.Plain, Lean: r.Lean,
			Novalue: r.Novalue, Leader: r.Leader}
	}
	return out
}
