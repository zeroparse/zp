package common

import (
	"io"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/middleend/execlower"
	"zeroparse.dev/zp/internal/middleend/flowpass"
	"zeroparse.dev/zp/internal/pass"
)

// Passes are the pass settings of one generation, which the driver resolves from the GenOptions of
// the generation (internal/driver/generate): the optimizations of GenOptions.Optimize, Opt and NoOpt,
// the dumps of GenOptions.PrintAfter and the verification of GenOptions.VerifyEachPass. The
// generators read the settings and run the pipelines through a Runner, so that a profile or a flag
// means the same passes whatever the backend, and the back end does not import the driver.
type Passes interface {
	// Settings returns the optimizations that are on, or the error of resolving the settings: a
	// profile or an optimization that does not exist, a value that it does not take, a name that Opt
	// gives twice, or a PrintAfter list that names an unknown step or has no PrintTo writer. A
	// generator asks for them where it validates its options, so that the error of an option that it
	// checks first comes first.
	Settings() (pass.Settings, error)
	// Runner returns the runner of the pipelines with the settings, which writes the dumps that
	// PrintAfter selects to w, or none if w is nil. Settings must have returned no error.
	Runner(w io.Writer) Runner
}

// A Runner runs the pass pipelines of the IR for a generator, under the settings of its Passes. A
// failure is an internal error that names the pass. The driver implements it (driver.Driver).
type Runner interface {
	// RunCore runs the pipeline of Core passes on m, the Core IR of the table named table;
	// prepareScans is called around the scans (corepass.Env).
	RunCore(table string, m *core.Module, prepareScans func(*core.Module) func()) error
	// RunFlow runs the pipeline of Flow passes on the module m of one lowered function of the table
	// named table; observe says that the parser reports its rules and skipped alternatives, and
	// target which forms the backend prints (flowpass.Env).
	RunFlow(table string, m *code.Module, observe bool, target flowpass.Target) error
	// RunExec runs the pipeline of Exec passes that every target runs on the module m, which the
	// lowering of one function of the table named table from Flow IR made; observe is as for RunFlow.
	RunExec(table string, m *code.Module, observe bool) error
	// RunTarget runs p, the pipeline of a target's own Exec passes, on the lowered function l, after
	// RunExec; table and observe are as for RunExec.
	RunTarget(table string, p *TargetPipeline, l *execlower.Lowered, observe bool) error
}

// TargetPipeline is the pipeline of the Exec passes of one target (golang/pass, typescript/pass), which
// run on a lowered function after the Exec passes that every target runs (Runner.RunExec).
type TargetPipeline = pass.Pipeline[*execlower.Lowered, *codepass.Env]
