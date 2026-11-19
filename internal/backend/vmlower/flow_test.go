package vmlower

import (
	"os"
	"path/filepath"
	"reflect"
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

// TestSelectedFlowsSurviveTheFileForm selects the Flow body of every rule of the testdata grammars
// that has one with the operations selected so far, and checks that the module validates and that
// its bodies read back from the module file as they were written.
func TestSelectedFlowsSurviveTheFileForm(t *testing.T) {
	paths, err := filepath.Glob("../../../testdata/grammars/*/*.zp")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no grammars: %v", err)
	}
	total := 0
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		prog := compiletest.Program(t, string(src), compile.Options{})
		m := mustCore(t, prog)
		bc, err := CompileCore(prog, mustCore(t, prog))
		if err != nil {
			t.Fatal(err)
		}
		for i, r := range m.Rules {
			fm, err := FlowModule(m, i, r.Name)
			if err != nil {
				continue // a rule whose Flow form the VM's options do not have
			}
			_ = SelectFlows(bc, prog, m, map[int]*code.Module{i: fm}) // a rule it fails for keeps no body
		}
		total += len(bc.Flows)
		if err := compile.ValidateModule(bc); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		w := &bytecode.Encoder{Strs: map[string]int{}}
		w.WriteModule(bc)
		r := &bytecode.Decoder{Data: w.Body.Bytes(), Strs: bc.Strings}
		got := r.ReadModule()
		if r.Err != nil {
			t.Fatalf("%s: %v", path, r.Err)
		}
		if !reflect.DeepEqual(got.Flows, bc.Flows) {
			t.Errorf("%s: the Flow bodies read back differ from those written", path)
		}
	}
	if total == 0 {
		t.Fatal("no rule of the testdata grammars has a Flow body")
	}
	t.Logf("%d Flow bodies", total)
}
