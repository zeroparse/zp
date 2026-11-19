package engine

import (
	"os"
	"testing"

	"zeroparse.dev/zp/internal/backend/vmlower"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/runtime/state"
)

// parseCharged parses input with the bytecode VM of prog as Parse does, and returns the tree, the
// error and the budget spend of the parse.
func parseCharged(t *testing.T, prog *Program, input string) (string, string, uint64) {
	t.Helper()
	o := ParseOptions{Backend: Bytecode}
	start := prog.ResolveStart("")
	active, _, control, err := prog.admitParse(start, o, optionsFamily(o), false, input)
	if err != nil {
		t.Fatal(err)
	}
	control.Check()
	p := state.AcquirePooledParser(&active.sharedProgram)
	defer p.ReleaseBatchScratch()
	p.Input = state.NewControlledTextInput(input, o.Unit, true, p.Scratch, control)
	control.AttachMemory(p)
	p.MaxDepth, _ = state.DepthLimit(o, o.Backend)
	p.DeferMemo = true
	n, err := active.Run(p, o.Backend, start)
	if !p.Aborted {
		control.Check()
	}
	tree := "<nil>"
	if n != nil {
		tree = n.String()
	}
	msg := "<nil>"
	if err != nil {
		msg = err.Error()
	}
	return tree, msg, control.Used
}

// jsonFlowPrograms returns the JSON grammar compiled as usual, and the same grammar whose VM runs the
// value rule from its Flow form. mutate, if not nil, changes the selected body before it is installed.
func jsonFlowPrograms(t *testing.T, mutate func(*bytecode.Flow)) (*Program, *Program) {
	t.Helper()
	src, err := os.ReadFile("../../testdata/grammars/json/json.zp")
	if err != nil {
		t.Fatal(err)
	}
	old := mustCompile(t, string(src))
	cp := old.src.(interface{ Checked() *compile.Program }).Checked()
	coreFlow, err := vmlower.CoreOf(cp)
	if err != nil {
		t.Fatal(err)
	}
	value := -1
	for i, r := range coreFlow.Rules {
		if r.Name == "value" {
			value = i
		}
	}
	fm, err := vmlower.FlowModule(coreFlow, value, "value")
	if err != nil {
		t.Fatal(err)
	}
	coreBC, err := vmlower.CoreOf(cp)
	if err != nil {
		t.Fatal(err)
	}
	bc, err := vmlower.CompileCore(cp, coreBC)
	if err != nil {
		t.Fatal(err)
	}
	if err := vmlower.SelectFlows(bc, cp, coreFlow, map[int]*code.Module{value: fm}); err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(bc.Flows[value])
	}
	return old, New(flowSource{Source: old.src, m: bc})
}

// flowSource is the source of a grammar whose module is m.
type flowSource struct {
	Source
	m *bytecode.Module
}

func (s flowSource) Module() (*bytecode.Module, error) { return s.m, nil }

var jsonDifferentialInputs = []string{
	`{}`, `[]`, `1`, `"a\"b"`, `true`, `null`, `-0.5e+10`,
	`{"a":[1,2,{"b":null}],"c":"xA"}`, `[[[[]]]]`, `{"k":{"k":{"k":[]}}}`,
	`  [ -1.5e+3 , false ]  `, `[1,]`, `{"a" 1}`, `"unterminated`, `tru`, `-`, `01`, `[1,2`, ``,
}

// TestFlowBodyJSONValueDifferential runs the JSON corpus through the bytecode VM with the value rule
// as its match code and as its Flow body: the trees, the errors and the budget spends must agree.
func TestFlowBodyJSONValueDifferential(t *testing.T) {
	old, spiked := jsonFlowPrograms(t, nil)
	for _, in := range jsonDifferentialInputs {
		ot, oe, ou := parseCharged(t, old, in)
		st, se, su := parseCharged(t, spiked, in)
		if ot != st || oe != se || ou != su {
			t.Errorf("input %q: old tree %s err %s spend %d; flow tree %s err %s spend %d", in, ot, oe, ou, st, se, su)
		}
	}
}

// TestFlowBodyDifferentialDetectsMutations checks that the differential harness sees a changed Flow body:
// a call of the value rule's body to a different rule changes the trees of the corpus, and an extra read
// of the input at the start of the body, which changes no result, changes the spends.
func TestFlowBodyDifferentialDetectsMutations(t *testing.T) {
	old, called := jsonFlowPrograms(t, func(f *bytecode.Flow) {
		for i := range f.Code {
			if in := &f.Code[i]; in.Op == bytecode.FlowCall {
				x := in.Imm.(bytecode.FlowCallOf)
				x.Rule = 0 // main: a different rule than the one the body calls
				in.Imm = x
				return
			}
		}
	})
	_, peeked := jsonFlowPrograms(t, func(f *bytecode.Flow) {
		for i := range f.Code {
			if in := &f.Code[i]; in.Op == bytecode.FlowJump || in.Op == bytecode.FlowJumpFalse {
				in.Target++
			}
		}
		peek := bytecode.FlowInstr{Op: bytecode.FlowPeek, Dst: []int{-1, -1, -1}}
		f.Code = append([]bytecode.FlowInstr{peek}, f.Code...)
	})
	trees, spends := false, false
	for _, in := range jsonDifferentialInputs {
		ot, oe, ou := parseCharged(t, old, in)
		ct, _, _ := parseCharged(t, called, in)
		pt, pe, pu := parseCharged(t, peeked, in)
		trees = trees || ot != ct
		if pt != ot || pe != oe {
			t.Errorf("input %q: an extra read changed the result: %s %s, want %s %s", in, pt, pe, ot, oe)
		}
		spends = spends || pu != ou
	}
	if !trees {
		t.Error("a changed call in the Flow body changed no tree of the corpus: the Flow body does not run")
	}
	if !spends {
		t.Error("an extra read in the Flow body changed no spend of the corpus: the harness does not compare charges")
	}
}
