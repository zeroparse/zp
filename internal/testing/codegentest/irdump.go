package codegentest

import (
	"regexp"
	"slices"
	"testing"
	"zeroparse.dev/zp/internal/middleend/flowpass"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/pass"
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

// BrokenFlowPass is a Flow pass that lists a variable twice, which flow.VerifyModule rejects.
var BrokenFlowPass = pass.Pass[*code.Module, *flowpass.Env]{
	Name:       "break-flow",
	Obligation: "none: it breaks the function on purpose",
	Run: func(m *code.Module, _ *flowpass.Env) error {
		a := flow.AttrsOf(flow.FuncOf(m))
		a.Vars = append(a.Vars, "broken", "broken")
		return nil
	},
}

// BrokenExecPass is an Exec pass that declares the first function of the module twice, which
// code.Verify rejects.
var BrokenExecPass = pass.Pass[*code.Module, *codepass.Env]{
	Name:       "break-exec",
	Obligation: "none: it breaks the module on purpose",
	Run: func(m *code.Module, _ *codepass.Env) error {
		m.Funcs = append(m.Funcs, m.Funcs[0])
		return nil
	},
}

// BrokenFlowPipeline returns a FlowPipeline hook of a generator (driver.Driver) that runs the Flow
// pipeline with BrokenFlowPass added at its end on the functions of the table named table, and the
// Flow pipeline as it is on the others. It counts the functions of the table in seen. The pass is no
// registered optimization, so the hooks name it in TestPasses.
func BrokenFlowPipeline(table string, seen *int) func(string) *pass.Pipeline[*code.Module, *flowpass.Env] {
	p := *flowpass.Pipeline
	p.Passes = append(slices.Clone(flowpass.Pipeline.Passes), BrokenFlowPass)
	return func(tb string) *pass.Pipeline[*code.Module, *flowpass.Env] {
		if tb != table {
			return flowpass.Pipeline
		}
		*seen++
		return &p
	}
}

// BrokenExecPipeline is BrokenFlowPipeline for the Exec pipeline, with BrokenExecPass.
func BrokenExecPipeline(table string, seen *int) func(string) *pass.Pipeline[*code.Module, *codepass.Env] {
	p := *codepass.Pipeline
	p.Passes = append(slices.Clone(codepass.Pipeline.Passes), BrokenExecPass)
	return func(tb string) *pass.Pipeline[*code.Module, *codepass.Env] {
		if tb != table {
			return codepass.Pipeline
		}
		*seen++
		return &p
	}
}
