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
	"zeroparse.dev/zp/internal/middleend/flowpass"
	"zeroparse.dev/zp/internal/pass"
)

// A Driver runs the pass pipelines of the IR under one Settings value: it turns the passes on and
// off by their flags, verifies the unit after each pass when asked, and writes the dumps that
// PrintAfter selects. Every backend that lowers from the IR runs its pipelines through a Driver, so
// a profile or a flag means the same passes whatever the backend. The zero value runs the speed
// profile, verifies once at the end of each pipeline and dumps nothing.
type Driver struct {
	// Opt holds the optimizations that are on.
	Opt pass.Settings
	// PrintAfter lists the names (PrintNames) after which a unit is written to PrintTo; "all"
	// selects every step and pass, "core" the Core IR before the first Core pass, "flow" each
	// Flow function before the first Flow pass and "exec" each Exec function before the first Exec
	// pass.
	PrintAfter []string
	PrintTo    io.Writer
	// VerifyEach verifies the unit after every pass instead of once at the end, so that the error
	// names the first pass that broke an invariant.
	VerifyEach bool
	// CoreHook, if set, is called with the Core module after it is built and after each pass that
	// runs on it (corepass.RunOptions.Dump).
	CoreHook func(pass string, m *core.Module)
	// CoreRun, if set, runs the Core passes in place of corepass.Run, for tests that run a pipeline of
	// their own.
	CoreRun func(m *core.Module, env *corepass.Env, o corepass.RunOptions) ([]string, error)
	// FlowRun, if set, runs the Flow passes in place of flowpass.Run, for tests that run a pipeline of
	// their own; table names the table of the function, as in the dumps.
	FlowRun func(table string, m *code.Module, env *flowpass.Env, o flowpass.RunOptions) ([]string, error)
	// ExecRun, if set, runs the Exec passes in place of codepass.RunPasses, for tests that run a pipeline
	// of their own; table names the table of the module, as in the dumps.
	ExecRun func(table string, m *code.Module, env *codepass.Env, o codepass.RunOptions) ([]string, error)
}

// prints reports whether the unit is dumped after the named pass.
func (d *Driver) prints(pass string) bool {
	return d.PrintTo != nil && (slices.Contains(d.PrintAfter, pass) || slices.Contains(d.PrintAfter, PrintAll))
}

// RunCore runs the pipeline of Core passes on m, with the optimizations of the driver; prepareScans
// is called around the scans (corepass.Env). The module is the one of the table named table, which the
// printed Core IR is labeled with. A failure is an internal error that names the pass.
func (d *Driver) RunCore(table string, m *core.Module, prepareScans func(*core.Module) func()) error {
	env := &corepass.Env{Flag: d.Opt.On, PrepareScans: prepareScans}
	run := corepass.Run
	if d.CoreRun != nil {
		run = d.CoreRun
	}
	dump := d.CoreHook
	if len(d.PrintAfter) > 0 && d.PrintTo != nil {
		hook := dump
		dump = func(pass string, m *core.Module) {
			if hook != nil {
				hook(pass, m)
			}
			if d.prints(pass) {
				fmt.Fprintf(d.PrintTo, "-- Core IR of the %s table after %s --\n%s\n", table, pass, core.Print(m, true))
			}
		}
	}
	if _, err := run(m, env, corepass.RunOptions{Dump: dump, VerifyEach: d.VerifyEach}); err != nil {
		return fmt.Errorf("internal error: %w", err)
	}
	return nil
}

// RunFlow runs the pipeline of Flow passes on the module m of one lowered function (flow.FuncOf),
// which the lowering has verified, with the optimizations of the driver; observe says that the parser
// reports its rules and skipped alternatives, and target what the backend prints (flowpass.Env). The function belongs to the table named
// table, which the printed Flow IR is labeled with, as the dump of "flow" before the first pass. A
// failure is an internal error that names the pass.
func (d *Driver) RunFlow(table string, m *code.Module, observe bool, target flowpass.Target) error {
	env := &flowpass.Env{Func: flow.AttrsOf(flow.FuncOf(m)), Observe: observe, Flag: d.Opt.On, Value: d.Opt.Value, Target: target}
	var dump func(pass string, m *code.Module)
	if len(d.PrintAfter) > 0 && d.PrintTo != nil {
		dump = func(pass string, m *code.Module) {
			if d.prints(pass) {
				f := flow.FuncOf(m)
				fmt.Fprintf(d.PrintTo, "-- Flow IR of %s in the %s table after %s --\n%s\n", f.Name, table, pass, code.FuncStringIn(m, f))
			}
		}
	}
	o := flowpass.RunOptions{Dump: dump, VerifyEach: d.VerifyEach}
	var err error
	if d.FlowRun != nil {
		_, err = d.FlowRun(table, m, env, o)
	} else {
		_, err = flowpass.Run(m, env, o)
	}
	flowpass.Finish(m)
	if err != nil {
		return fmt.Errorf("internal error: %w", err)
	}
	return nil
}

// RunExec runs the pipeline of Exec passes on the module m, which the lowering of one function from
// Flow IR made and verified, with the optimizations of the driver; observe is as for RunFlow. The
// module belongs to the table named table, which the printed Exec IR of its functions is labeled
// with, as the dump of "exec" before the first pass. A failure is an internal error that names the
// pass.
func (d *Driver) RunExec(table string, m *code.Module, observe bool) error {
	env := &codepass.Env{Observe: observe, Flag: d.Opt.On}
	var dump func(pass string, m *code.Module)
	if len(d.PrintAfter) > 0 && d.PrintTo != nil {
		dump = func(pass string, m *code.Module) {
			if !d.prints(pass) {
				return
			}
			for _, f := range m.Funcs {
				fmt.Fprintf(d.PrintTo, "-- Exec IR of %s in the %s table after %s --\n%s\n", f.Name, table, pass, code.FuncStringIn(m, f))
			}
		}
	}
	o := codepass.RunOptions{Dump: dump, VerifyEach: d.VerifyEach}
	var err error
	if d.ExecRun != nil {
		_, err = d.ExecRun(table, m, env, o)
	} else {
		_, err = codepass.RunPasses(m, env, o)
	}
	if err != nil {
		return fmt.Errorf("internal error: %w", err)
	}
	return nil
}

// TargetPipeline is the pipeline of the Exec passes of one target (golang/pass, typescript/pass), which
// run on a lowered function after the Exec passes that every target runs (RunExec).
type TargetPipeline = pass.Pipeline[*execlower.Lowered, *codepass.Env]

// targets holds the pipelines of the targets' Exec passes, whose passes -print-after names
// (PrintNames).
var targets = []*TargetPipeline{gopass.Pipeline, tspass.Pipeline}

// RunTarget runs the pipeline p of the Exec passes of a target on the lowered function l, with the
// optimizations of the driver, as RunExec runs the Exec passes: observe and table are as for RunExec,
// and the dumps after each pass are labeled with the table. A failure is an internal error that names
// the pass.
func (d *Driver) RunTarget(table string, p *TargetPipeline, l *execlower.Lowered, observe bool) error {
	env := &codepass.Env{Observe: observe, Flag: d.Opt.On}
	var dump func(pass string, l *execlower.Lowered)
	if len(d.PrintAfter) > 0 && d.PrintTo != nil {
		dump = func(name string, l *execlower.Lowered) {
			if name != codepass.Stage && d.prints(name) {
				fmt.Fprintf(d.PrintTo, "-- Exec IR of %s in the %s table after %s --\n%s\n", l.Func.Name, table, name, code.FuncStringIn(l.Module, l.Func))
			}
		}
	}
	o := pass.Options[*execlower.Lowered]{Enabled: env.On, VerifyEach: d.VerifyEach, Dump: dump, Stage: codepass.Stage, BuiltVerified: true}
	if _, err := p.Run(l, env, o); err != nil {
		return fmt.Errorf("internal error: %w", err)
	}
	return nil
}
