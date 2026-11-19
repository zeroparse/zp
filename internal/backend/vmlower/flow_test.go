package vmlower

import (
	"os"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/ir/code"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/testing/compiletest"
	"zeroparse.dev/zp/pkg/bytecode"
)

func jsonProgram(t *testing.T) *compile.Program {
	t.Helper()
	src, err := os.ReadFile("../../../testdata/grammars/json/json.zp")
	if err != nil {
		t.Fatal(err)
	}
	return compiletest.Program(t, string(src), compile.Options{})
}

func mustCore(t *testing.T, prog *compile.Program) *ir.Module {
	t.Helper()
	m, err := CoreOf(prog)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func ruleIndex(t *testing.T, m *ir.Module, name string) int {
	t.Helper()
	for i, r := range m.Rules {
		if r.Name == name {
			return i
		}
	}
	t.Fatalf("no rule %s", name)
	return -1
}

// selectRule selects the Flow body of the rule of index rule into a fresh module of prog.
func selectRule(t *testing.T, prog *compile.Program, rule int, name string) (*bytecode.Module, error) {
	t.Helper()
	m := mustCore(t, prog)
	bc, err := CompileCore(prog, mustCore(t, prog))
	if err != nil {
		t.Fatal(err)
	}
	fm, err := FlowModule(m, rule, name)
	if err != nil {
		return nil, err
	}
	return bc, SelectFlows(bc, prog, m, map[int]*code.Module{rule: fm})
}

// TestSelectJSONValueFlow selects the body of the JSON value rule, the first spike of the Flow form.
func TestSelectJSONValueFlow(t *testing.T) {
	prog := jsonProgram(t)
	rule := ruleIndex(t, mustCore(t, prog), "value")
	bc, err := selectRule(t, prog, rule, "value")
	if err != nil {
		t.Fatal(err)
	}
	f := bc.Flows[rule]
	if f == nil {
		t.Fatal("value has no Flow body")
	}
	t.Logf("value: %d registers, %d instructions", f.Regs, len(f.Code))
}

// TestSelectJSONRulesCount reports how many rules of the JSON grammar have a bytecode Flow form with
// the operations selected so far; a rule that has none is an error that names its operation.
func TestSelectJSONRulesCount(t *testing.T) {
	prog := jsonProgram(t)
	m := mustCore(t, prog)
	ok := 0
	for i, r := range m.Rules {
		if _, err := selectRule(t, prog, i, r.Name); err != nil {
			t.Logf("%s: %v", r.Name, err)
			continue
		}
		ok++
	}
	t.Logf("selected %d of %d rules", ok, len(m.Rules))
}
