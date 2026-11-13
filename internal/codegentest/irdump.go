package codegentest

import (
	"regexp"
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/flow"
	"zeroparse.dev/zp/internal/ir/pipeline"
)

// irDumpHeader matches the header of a dump of one Flow IR or Exec IR function before the first
// pass of its pipeline (GenOptions.PrintAfter "flow" and "exec").
var irDumpHeader = regexp.MustCompile(`(?m)^-- (Flow|Exec) IR of (\S+) in the (\S+) table after (?:flow|exec) --$`)

// CheckIRPipelines checks the dumps that a generation with PrintAfter "flow,exec" wrote: every function
// printed from Exec IR was lowered from a Flow IR function, so each function of each table must have
// as many Flow IR dumps as Exec IR dumps, and each table in tables must have some. A function whose
// Flow passes or Exec passes the generator skipped has no dump of that IR. It returns the number of
// Flow IR dumps of each table.
func CheckIRPipelines(t *testing.T, dump string, tables ...string) map[string]int {
	t.Helper()
	type key struct{ table, fn string }
	flows, execs := map[key]int{}, map[key]int{}
	perTable := map[string]int{}
	for _, m := range irDumpHeader.FindAllStringSubmatch(dump, -1) {
		k := key{m[3], m[2]}
		if m[1] == "Flow" {
			flows[k]++
			perTable[k.table]++
		} else {
			execs[k]++
		}
	}
	for k, n := range flows {
		if execs[k] != n {
			t.Errorf("function %s of the %s table: %d Flow IR dumps, %d Exec IR dumps", k.fn, k.table, n, execs[k])
		}
	}
	for k, n := range execs {
		if _, ok := flows[k]; !ok {
			t.Errorf("function %s of the %s table: no Flow IR dump, %d Exec IR dumps", k.fn, k.table, n)
		}
	}
	for _, table := range tables {
		if perTable[table] == 0 {
			t.Errorf("no function of the %s table went through the Flow and Exec pipelines", table)
		}
	}
	return perTable
}

// BrokenFlowPass is a Flow pass that lists a variable that is also an entry variable, which flow.Verify
// rejects.
var BrokenFlowPass = pipeline.Pass[*flow.Func, *flow.Env]{
	Name:       "break-flow",
	Obligation: "none: it breaks the function on purpose",
	Run: func(f *flow.Func, _ *flow.Env) error {
		f.Vars = append(f.Vars, &flow.Var{Name: "broken", Type: flow.TypeInt, Entry: true})
		return nil
	},
}

// BrokenExecPass is an Exec pass that declares the first function of the module twice, which
// exec.Verify rejects.
var BrokenExecPass = pipeline.Pass[*exec.Module, *exec.Env]{
	Name:       "break-exec",
	Obligation: "none: it breaks the module on purpose",
	Run: func(m *exec.Module, _ *exec.Env) error {
		m.Funcs = append(m.Funcs, m.Funcs[0])
		return nil
	},
}

// BrokenFlowRun returns a FlowRun hook of a generator that runs the Flow pipeline with
// BrokenFlowPass added at its end on the functions of the table named table, and the Flow pipeline
// as it is on the others. It counts the functions of the table in seen.
func BrokenFlowRun(table string, seen *int) func(string, *flow.Func, *flow.Env, flow.RunOptions) ([]string, error) {
	p := *flow.Pipeline
	p.Passes = append(slices.Clone(flow.Pipeline.Passes), BrokenFlowPass)
	return func(tb string, f *flow.Func, env *flow.Env, o flow.RunOptions) ([]string, error) {
		if tb != table {
			return flow.Run(f, env, o)
		}
		*seen++
		po := o.Options(env)
		// The pass is not a registered optimization, which the settings of the generator would refuse.
		po.Enabled = func(name string) bool { return name == BrokenFlowPass.Name || env.On(name) }
		return p.Run(f, env, po)
	}
}

// BrokenExecRun is BrokenFlowRun for the Exec pipeline, with BrokenExecPass.
func BrokenExecRun(table string, seen *int) func(string, *exec.Module, *exec.Env, exec.RunOptions) ([]string, error) {
	p := *exec.Pipeline
	p.Passes = append(slices.Clone(exec.Pipeline.Passes), BrokenExecPass)
	return func(tb string, m *exec.Module, env *exec.Env, o exec.RunOptions) ([]string, error) {
		if tb != table {
			return exec.RunPasses(m, env, o)
		}
		*seen++
		po := o.Options(env)
		// The pass is not a registered optimization, which the settings of the generator would refuse.
		po.Enabled = func(name string) bool { return name == BrokenExecPass.Name || env.On(name) }
		return p.Run(m, env, po)
	}
}
