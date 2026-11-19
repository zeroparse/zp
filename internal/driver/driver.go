package driver

import (
	"fmt"
	"io"
	"slices"

	gopass "zeroparse.dev/zp/internal/backend/codegen/golang/pass"
	tspass "zeroparse.dev/zp/internal/backend/codegen/typescript/pass"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/middleend/execlower"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/internal/middleend/flowpass"
	"zeroparse.dev/zp/internal/pass"
)

// A Driver runs the pipeline of a compilation under one Settings value: it turns the passes on and
// off by their flags, verifies the unit after each pass when asked, and writes the dumps that
// PrintAfter selects. Every backend that lowers from the IR runs its pipeline through a Driver, so a
// profile or a flag means the same passes whatever the backend. The zero value runs the speed
// profile, verifies once at the end of each stage and dumps nothing.
//
// One Driver value is one compilation: its stages run under one manager (pass.Run), created when the
// first stage runs, which numbers the runs of optional passes across the tables and functions of the
// compilation and tells the observers (the dumps, CoreHook) about every step. A copy of a Driver
// that has not run yet is a compilation of its own.
type Driver struct {
	// Opt holds the optimizations that are on.
	Opt pass.Settings
	// PrintAfter lists the names (PrintNames) after which a unit is written to PrintTo; "all"
	// selects every step and pass, "core" the Core IR before the first Core pass, "flow" each
	// Flow function as flowlower made it and "exec" each Exec function as execlower made it.
	PrintAfter []string
	PrintTo    io.Writer
	// VerifyEach verifies the unit after every pass instead of once at the end, so that the error
	// names the first pass that broke an invariant.
	VerifyEach bool
	// CoreHook, if set, is called with the Core module as it reaches the Core passes and after each
	// Core pass that ran, before it is verified.
	CoreHook func(pass string, m *core.Module)
	// CorePipeline, FlowPipeline and ExecPipeline, if set, return the pipeline of the Core, Flow
	// or Exec passes that runs on the units of the named table in place of corepass.Pipeline,
	// flowpass.Pipeline or codepass.Pipeline, for tests that run passes of their own.
	CorePipeline func(table string) *pass.Pipeline[*core.Module, *corepass.Env]
	FlowPipeline func(table string) *pass.Pipeline[*code.Module, *flowpass.Env]
	ExecPipeline func(table string) *pass.Pipeline[*code.Module, *codepass.Env]
	// TestPasses names passes of those pipelines that are no registered optimization: no flag turns
	// them off, so they run wherever they apply.
	TestPasses []string

	// run is the manager of the compilation, nil until the first stage runs.
	run *pass.Run
}

// Settings returns the optimizations that are on (Opt), as common.Runner requires.
func (d *Driver) Settings() pass.Settings { return d.Opt }

// prints reports whether the unit is dumped after the step whose dump name is name.
func (d *Driver) prints(name string) bool {
	return d.PrintTo != nil && name != "" && (slices.Contains(d.PrintAfter, name) || slices.Contains(d.PrintAfter, PrintAll))
}

// enabled reports whether the optional pass of the given name may run: a test pass always may,
// a registered optimization when it is on.
func (d *Driver) enabled(name string) bool {
	return slices.Contains(d.TestPasses, name) || d.Opt.On(name)
}

// manager returns the manager of the compilation, created on first use.
func (d *Driver) manager() *pass.Run {
	if d.run == nil {
		d.run = &pass.Run{Enabled: d.enabled, VerifyEach: d.VerifyEach, Observers: []pass.Observer{observer{d}}}
	}
	return d.run
}

// observer is the observer of the steps of a compilation that writes the dumps of PrintAfter and
// calls CoreHook. It reads the fields of the driver when it is told of a step, so that a hook set
// between two stages sees the later one.
type observer struct{ d *Driver }

func (observer) Before(pass.Event) bool { return true }

func (o observer) After(e pass.Event, print func(io.Writer), unit any) {
	if o.d.CoreHook != nil && e.Step.Out == corepass.Stage {
		o.d.CoreHook(e.Step.Dump, unit.(*core.Module))
	}
	if o.d.prints(e.Step.Dump) {
		print(o.d.PrintTo)
	}
}

// RunCore runs the Core passes on m, the Core IR of the table named table, with the optimizations
// of the driver; prepareScans is called around the scans (corepass.Env). The module is dumped as
// "core" as it reaches the passes. A failure is an internal error that names the pass.
func (d *Driver) RunCore(table string, m *core.Module, prepareScans func(*core.Module) func()) error {
	p := corepass.Pipeline
	if d.CorePipeline != nil {
		p = d.CorePipeline(table)
	}
	s := pass.Transforms(pass.Transform[*core.Module, *corepass.Env]{
		IR: corepass.IR, Pipeline: p, Entry: corepass.Stage,
		Env: func(*pass.Run, pass.Label, *core.Module) *corepass.Env {
			return &corepass.Env{Flag: d.Opt.On, PrepareScans: prepareScans}
		},
	})
	if _, err := s.Run(d.manager(), pass.Label{Table: table}, m); err != nil {
		return fmt.Errorf("internal error: %w", err)
	}
	return nil
}

// FlowFunc runs the Flow part of the pipeline on one function of the table named table: the
// lowering flowlower of u, which its postcondition verifies, the Flow passes with the optimizations
// of the driver, and flow-finish, which also runs after a failed pass. observe says that the parser
// reports its rules and skipped alternatives, and target what the backend prints (flowpass.Env).
// The function is dumped as "flow" as flowlower made it, labeled with its table.
//
// A failure of flowlower is returned as the *pass.StepError that names it, with no module. A failure
// of a pass is an internal error that names the pass, returned with the module, which flow-finish
// has ended.
func (d *Driver) FlowFunc(table string, u flowlower.Unit, observe bool, target flowpass.Target) (*code.Module, error) {
	p := flowpass.Pipeline
	if d.FlowPipeline != nil {
		p = d.FlowPipeline(table)
	}
	s := pass.Then(flowlower.Lowering, pass.Then(pass.Transforms(pass.Transform[*code.Module, *flowpass.Env]{
		IR: flowlower.IR, Pipeline: p, BuiltVerified: true,
		Env: func(_ *pass.Run, _ pass.Label, m *code.Module) *flowpass.Env {
			return &flowpass.Env{Func: flow.AttrsOf(flow.FuncOf(m)), Observe: observe, Flag: d.Opt.On, Value: d.Opt.Value, Target: target}
		},
	}), flowpass.Finishing))
	m, err := s.Run(d.manager(), pass.Label{Table: table, Func: u.Func.Name}, u)
	return m, stepError(err)
}

// ExecFunc runs the Exec part of the pipeline on the module m of one function of the table named
// table, which FlowFunc returned: the conversion execlower, which its postcondition verifies, the
// Exec passes that every target runs and then target, the pipeline of the backend's own Exec passes,
// with the optimizations of the driver; observe is as for FlowFunc. Each function of the module is
// dumped as "exec" as execlower made it and after each Exec pass, the lowered function after each
// pass of target, labeled with its table.
//
// A failure of execlower is returned as the *pass.StepError that names it, with no result. A failure
// of a pass is an internal error that names the pass, returned with the result.
func (d *Driver) ExecFunc(table string, m *code.Module, target *TargetPipeline, observe bool) (*execlower.Lowered, error) {
	p := codepass.Pipeline
	if d.ExecPipeline != nil {
		p = d.ExecPipeline(table)
	}
	env := func(*pass.Run, pass.Label, any) *codepass.Env { return &codepass.Env{Observe: observe, Flag: d.Opt.On} }
	s := pass.Then(execlower.Lowering, pass.Then(
		pass.Project(func(l *execlower.Lowered) *code.Module { return l.Module }, pass.Transforms(pass.Transform[*code.Module, *codepass.Env]{
			IR: execModuleIR, Pipeline: p, BuiltVerified: true,
			Env: func(r *pass.Run, l pass.Label, m *code.Module) *codepass.Env { return env(r, l, m) },
		})),
		pass.Transforms(pass.Transform[*execlower.Lowered, *codepass.Env]{
			IR: loweredIR, Pipeline: target, BuiltVerified: true,
			Env: func(r *pass.Run, lb pass.Label, l *execlower.Lowered) *codepass.Env { return env(r, lb, l) },
		})))
	var name string
	if len(m.Funcs) > 0 {
		name = flow.FuncOf(m).Name
	}
	l, err := s.Run(d.manager(), pass.Label{Table: table, Func: name}, m)
	return l, stepError(err)
}

// stepError returns the failure of a required step as it is, and the failure of a pass as an
// internal error.
func stepError(err error) error {
	if _, ok := err.(*pass.StepError); ok || err == nil {
		return err
	}
	return fmt.Errorf("internal error: %w", err)
}

// execModuleIR describes the Exec module of a lowered function to the Exec passes: each function of
// it is dumped.
var execModuleIR = pass.IR[*code.Module]{Stage: codepass.Stage, Verify: code.Verify, Print: execlower.PrintModule}

// loweredIR describes a lowered function to the passes of a target: only the function is dumped.
var loweredIR = pass.IR[*execlower.Lowered]{
	Stage:  codepass.Stage,
	Verify: execlower.IR.Verify,
	Print: func(w io.Writer, lb pass.Label, after string, l *execlower.Lowered) {
		fmt.Fprintf(w, "-- Exec IR of %s in the %s table after %s --\n%s\n", l.Func.Name, lb.Table, after, code.FuncStringIn(l.Module, l.Func))
	},
}

// TargetPipeline is the pipeline of the Exec passes of one target (golang/pass, typescript/pass), which
// run on a lowered function after the Exec passes that every target runs (ExecFunc).
type TargetPipeline = pass.Pipeline[*execlower.Lowered, *codepass.Env]

// targets holds the pipelines of the targets' Exec passes, whose passes -print-after names
// (PrintNames).
var targets = []*TargetPipeline{gopass.Pipeline, tspass.Pipeline}
