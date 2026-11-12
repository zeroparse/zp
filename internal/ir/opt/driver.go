package opt

import (
	"fmt"
	"io"
	"slices"

	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/flow"
)

// A Driver runs the pass pipelines of the IR under one Settings value: it turns the passes on and
// off by their flags, verifies the unit after each pass when asked, and writes the dumps that
// PrintAfter selects. Every backend that lowers from the IR runs its pipelines through a Driver, so
// a profile or a flag means the same passes whatever the backend. The zero value runs the speed
// profile, verifies once at the end of each pipeline and dumps nothing.
type Driver struct {
	// Opt holds the optimizations that are on.
	Opt Settings
	// PrintAfter lists the names (PrintNames) after which a unit is written to PrintTo; "all"
	// selects every step and pass, "build" the Core IR before the first Core pass, "flow" each
	// Flow function before the first Flow pass and "exec" each Exec function before the first Exec
	// pass.
	PrintAfter []string
	PrintTo    io.Writer
	// VerifyEach verifies the unit after every pass instead of once at the end, so that the error
	// names the first pass that broke an invariant.
	VerifyEach bool
	// CoreHook, if set, is called with the Core module after it is built and after each pass that
	// runs on it (core.RunOptions.Dump).
	CoreHook func(pass string, m *core.Module)
	// CoreRun, if set, runs the Core passes in place of core.Run, for tests that run a pipeline of
	// their own.
	CoreRun func(m *core.Module, env *core.Env, o core.RunOptions) ([]string, error)
	// FlowRun, if set, runs the Flow passes in place of flow.Run, for tests that run a pipeline of
	// their own.
	FlowRun func(f *flow.Func, env *flow.Env, o flow.RunOptions) ([]string, error)
	// ExecRun, if set, runs the Exec passes in place of exec.RunPasses, for tests that run a pipeline
	// of their own.
	ExecRun func(m *exec.Module, env *exec.Env, o exec.RunOptions) ([]string, error)
}

// prints reports whether the unit is dumped after the named pass.
func (d *Driver) prints(pass string) bool {
	return d.PrintTo != nil && (slices.Contains(d.PrintAfter, pass) || slices.Contains(d.PrintAfter, PrintAll))
}

// RunCore runs the pipeline of Core passes on m in the mode, with the optimizations of the driver.
// local selects the rules that terminal regions are made in, and prepareScans is called around the
// scans (core.Env). The module is the one of the table named table, which the printed Core IR is
// labeled with. A failure is an internal error that names the pass.
func (d *Driver) RunCore(table string, m *core.Module, mode core.Mode, local func(*core.Rule) bool, prepareScans func(*core.Module) func()) error {
	env := &core.Env{Mode: mode, Flag: d.Opt.On, Local: local, PrepareScans: prepareScans}
	run := core.Run
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
	if _, err := run(m, env, core.RunOptions{Dump: dump, VerifyEach: d.VerifyEach}); err != nil {
		return fmt.Errorf("internal error: %w", err)
	}
	return nil
}

// RunFlow runs the pipeline of Flow passes on the function f, which the lowering has verified, with
// the optimizations of the driver; observe says that the parser reports its rules and skipped
// alternatives (flow.Env). The function belongs to the table named table, which the printed Flow IR
// is labeled with, as the dump of "flow" before the first pass. A failure is an internal error that
// names the pass.
func (d *Driver) RunFlow(table string, f *flow.Func, observe bool) error {
	env := &flow.Env{Observe: observe, Flag: d.Opt.On}
	run := flow.Run
	if d.FlowRun != nil {
		run = d.FlowRun
	}
	var dump func(pass string, f *flow.Func)
	if len(d.PrintAfter) > 0 && d.PrintTo != nil {
		dump = func(pass string, f *flow.Func) {
			if pass == PrintBuild {
				pass = PrintFlow
			}
			if d.prints(pass) {
				fmt.Fprintf(d.PrintTo, "-- Flow IR of %s in the %s table after %s --\n%s\n", f.Name, table, pass, flow.String(f))
			}
		}
	}
	if _, err := run(f, env, flow.RunOptions{Dump: dump, VerifyEach: d.VerifyEach}); err != nil {
		return fmt.Errorf("internal error: %w", err)
	}
	return nil
}

// RunExec runs the pipeline of Exec passes on the module m, which the lowering of one function from
// Flow IR made and verified, with the optimizations of the driver; observe is as for RunFlow. The
// module belongs to the table named table, which the printed Exec IR of its functions is labeled
// with, as the dump of "exec" before the first pass. A failure is an internal error that names the
// pass.
func (d *Driver) RunExec(table string, m *exec.Module, observe bool) error {
	env := &exec.Env{Observe: observe, Flag: d.Opt.On}
	run := exec.RunPasses
	if d.ExecRun != nil {
		run = d.ExecRun
	}
	var dump func(pass string, m *exec.Module)
	if len(d.PrintAfter) > 0 && d.PrintTo != nil {
		dump = func(pass string, m *exec.Module) {
			if pass == PrintBuild {
				pass = PrintExec
			}
			if !d.prints(pass) {
				return
			}
			for _, f := range m.Funcs {
				fmt.Fprintf(d.PrintTo, "-- Exec IR of %s in the %s table after %s --\n%s\n", f.Name, table, pass, exec.FuncString(f))
			}
		}
	}
	if _, err := run(m, env, exec.RunOptions{Dump: dump, VerifyEach: d.VerifyEach}); err != nil {
		return fmt.Errorf("internal error: %w", err)
	}
	return nil
}
