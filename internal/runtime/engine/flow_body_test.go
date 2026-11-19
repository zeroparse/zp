package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"zeroparse.dev/zp/internal/backend/vmlower"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/internal/runtime/state"
	"zeroparse.dev/zp/pkg/bytecode"
)

// parseCharged parses input with the recursive bytecode VM of prog as Parse does, and returns the
// tree, the error and the budget spend of the parse.
func parseCharged(t *testing.T, prog *Program, input string) (string, string, uint64) {
	t.Helper()
	return parseChargedWith(t, prog, input, Bytecode)
}

// parseChargedWith is parseCharged on the backend b.
func parseChargedWith(t *testing.T, prog *Program, input string, b Backend) (string, string, uint64) {
	t.Helper()
	o := ParseOptions{Backend: b}
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
	var used uint64 // nothing is charged where budget controls are compiled out
	if control != nil {
		used = control.Used
	}
	return tree, msg, used
}

// jsonFlowPrograms returns the JSON grammar compiled as usual, and the same grammar whose VM runs the
// value rule from its Flow form. mutate, if not nil, changes the selected body before it is installed.
func jsonFlowPrograms(t *testing.T, mutate func(*bytecode.Flow)) (*Program, *Program) {
	t.Helper()
	old, flows, n := flowPrograms(t, "../../../testdata/grammars/json/json.zp", func(name string) bool { return name == "value" }, func(_ string, f *bytecode.Flow) {
		if mutate != nil {
			mutate(f)
		}
	})
	if n != 1 {
		t.Fatalf("the value rule has no Flow body (%d bodies)", n)
	}
	return old, flows
}

// flowPrograms returns the grammar of the file path compiled as usual, and the same grammar whose VM
// runs from its Flow form each rule that pick accepts and that has a Flow body with the operations
// selected so far; n is the number of such rules. A rule without one keeps its match code. mutate, if
// not nil, changes the body of each selected rule before it is installed.
func flowPrograms(t *testing.T, path string, pick func(rule string) bool, mutate func(rule string, f *bytecode.Flow)) (old, flows *Program, n int) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old = mustCompile(t, string(src))
	cp := old.src.(interface{ Checked() *compile.Program }).Checked()
	coreFlow, err := vmlower.CoreOf(cp)
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
	for i, r := range coreFlow.Rules {
		if !pick(r.Name) {
			continue
		}
		fm, err := vmlower.FlowModule(coreFlow, i, r.Name)
		if err != nil {
			continue // a rule whose Flow form the VM's options do not have
		}
		if err := vmlower.SelectFlows(bc, cp, coreFlow, map[int]*code.Module{i: fm}); err != nil {
			continue // an operation without a bytecode Flow form yet
		}
		if mutate != nil {
			mutate(r.Name, bc.Flows[i])
		}
	}
	return old, New(flowSource{Source: old.src, m: bc}), len(bc.Flows)
}

// mutateCall changes the first call of a Flow body to call the null rule (jsonNull), a different rule
// than the one the body calls.
func mutateCall(f *bytecode.Flow) {
	for i := range f.Code {
		if in := &f.Code[i]; in.Op == bytecode.FlowCall {
			in.Imm = bytecode.FlowCallOf{Rule: jsonNull, Level: in.Imm.(bytecode.FlowCallOf).Level}
			return
		}
	}
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

// saveAndLoad saves prog as a compiled grammar and loads it.
func saveAndLoad(t *testing.T, prog *Program) *Program {
	t.Helper()
	data, err := prog.MarshalBinary("")
	if err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadProgram(data)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

// jsonNull is the index of the null rule of the JSON grammar, which the changed call of mutateCall
// calls: the null rule calls no rule, while a call of main would close a cycle of calls through
// main, which is not marked Rec, and the loader rejects such a module.
const jsonNull = 16

// TestFlowBodyJSONValueFileForm saves the grammar whose value rule runs its Flow body as a compiled
// grammar and loads it: the loaded module has the same Flow bodies, and runs the JSON corpus with the
// trees, errors and spends of the match code. A loaded body whose call is changed changes a tree, so
// the loaded program runs the body it read.
func TestFlowBodyJSONValueFileForm(t *testing.T) {
	old, spiked := jsonFlowPrograms(t, nil)
	if m := spiked.Module(); m.Strings[m.Rules[jsonNull].Name] != "null" {
		t.Fatalf("rule %d is %s, not null", jsonNull, m.Strings[m.Rules[jsonNull].Name])
	}
	_, called := jsonFlowPrograms(t, mutateCall)
	loaded, loadedCalled := saveAndLoad(t, spiked), saveAndLoad(t, called)
	changed := false
	for _, in := range jsonDifferentialInputs {
		ot, _, _ := parseCharged(t, old, in)
		ct, _, _ := parseCharged(t, loadedCalled, in)
		changed = changed || ot != ct
	}
	if !changed {
		t.Error("a changed call in the loaded Flow body changed no tree: the loaded program does not run its body")
	}
	want := spiked.Module().Flows
	if got := loaded.Module().Flows; len(want) == 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded Flow bodies %v, want %v", got, want)
	}
	for _, in := range jsonDifferentialInputs {
		ot, oe, ou := parseCharged(t, old, in)
		lt, le, lu := parseCharged(t, loaded, in)
		if ot != lt || oe != le || ou != lu {
			t.Errorf("input %q: match code tree %s err %s spend %d; loaded flow tree %s err %s spend %d", in, ot, oe, ou, lt, le, lu)
		}
	}
}

// TestFlowBodyDifferentialDetectsMutations checks that the differential harness sees a changed Flow body:
// a call of the value rule's body to a different rule changes the trees of the corpus, and an extra read
// of the input at the start of the body, which changes no result, changes the spends.
func TestFlowBodyDifferentialDetectsMutations(t *testing.T) {
	old, called := jsonFlowPrograms(t, mutateCall)
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
	if !spends && state.BudgetControlsEnabled { // a build without budget controls charges nothing
		t.Error("an extra read in the Flow body changed no spend of the corpus: the harness does not compare charges")
	}
}

// TestFlowBodyJSONValueIterative runs the JSON corpus through the iterative model with the value
// rule as its match code and as its Flow body: the trees, the errors and the budget spends must
// agree, and a changed call in the body must change a tree, so that the iterative model runs it.
func TestFlowBodyJSONValueIterative(t *testing.T) {
	old, spiked := jsonFlowPrograms(t, nil)
	_, called := jsonFlowPrograms(t, mutateCall)
	changed := false
	for _, in := range jsonDifferentialInputs {
		ot, oe, ou := parseChargedWith(t, old, in, BytecodeIterative)
		st, se, su := parseChargedWith(t, spiked, in, BytecodeIterative)
		if ot != st || oe != se || ou != su {
			t.Errorf("input %q: match code tree %s err %s spend %d; flow tree %s err %s spend %d", in, ot, oe, ou, st, se, su)
		}
		ct, _, _ := parseChargedWith(t, called, in, BytecodeIterative)
		changed = changed || ct != ot
	}
	if !changed {
		t.Error("a changed call in the Flow body changed no tree: the iterative model does not run it")
	}
}

// TestFlowBodyIterativeDeepNesting checks that the iterative model runs a Flow body without host
// recursion: JSON arrays nested deeper than a small host stack allows the recursive model, whose
// value rule runs its Flow body at every level, parse with the tree, error and spend of the match
// code.
func TestFlowBodyIterativeDeepNesting(t *testing.T) {
	old, spiked := jsonFlowPrograms(t, nil)
	const depth = 20000
	in := strings.Repeat("[", depth) + strings.Repeat("]", depth)
	ot, oe, ou := parseChargedWith(t, old, in, BytecodeIterative)
	if oe != "<nil>" {
		t.Fatalf("the match code fails: %s", oe)
	}
	prev := debug.SetMaxStack(1 << 20) // too small for the recursive model
	st, se, su := parseChargedWith(t, spiked, in, BytecodeIterative)
	debug.SetMaxStack(prev)
	if ot != st || oe != se || ou != su {
		t.Errorf("match code err %s spend %d; flow err %s spend %d; trees equal: %v", oe, ou, se, su, ot == st)
	}
}

// grammarCorpus returns the inputs of the grammar of the file path: the sample files next to it, and
// prefixes of each, which end the input inside its constructs and so take the failing paths.
func grammarCorpus(t *testing.T, path string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		in := string(data)
		out = append(out, in)
		for _, k := range []int{1, 2, 3, 5, 8} {
			if cut := len(in) * k / 9; cut > 0 && utf8.ValidString(in[:cut]) {
				out = append(out, in[:cut])
			}
		}
	}
	return out
}

// TestFlowBodiesTestdataGrammars runs the samples of the testdata grammars, and prefixes of them,
// through both VM models with every rule that has a Flow body running it: the trees and the errors
// must be those of the match code, and the module must load. The spends are not compared: a Flow
// body charges what its Flow function charges, which is not what the match code of its rule charges
// (spec/bytecode/README.md, Flow Bodies).
//
// The rules of flatNamingGap are left out: their whole-rule Flow function ends with the rule's
// finish and takes its value off the fresh nodes (flow.unfresh), so the rule frame that runs around
// it does not name the value after the rule, as it does the match code's value when a parse records
// source information. Generated Go runs such a function only for value-free rules; a rule that
// builds a value runs its framed body inside the rule frame.
func TestFlowBodiesTestdataGrammars(t *testing.T) {
	paths, err := filepath.Glob("../../../testdata/grammars/*/*.zp")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no grammars: %v", err)
	}
	total := 0
	for _, path := range paths {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			gap := flatNamingGap[filepath.Base(filepath.Dir(path))]
			old, flows, n := flowPrograms(t, path, func(r string) bool { return !slices.Contains(gap, r) }, nil)
			total += n
			// The loader accepts the module with its Flow bodies: their calls close no cycle of
			// calls through a rule that is not marked Rec.
			if err := program.ValidateModule(flows.Module()); err != nil {
				t.Fatalf("module with %d Flow bodies: %v", n, err)
			}
			for _, in := range grammarCorpus(t, path) {
				for _, b := range []Backend{Bytecode, BytecodeIterative} {
					ot, oe, _ := parseChargedWith(t, old, in, b)
					ft, fe, _ := parseChargedWith(t, flows, in, b)
					if ot != ft || oe != fe {
						t.Errorf("%v, input of %d bytes: match code err %s; flow err %s; trees equal: %v", b, len(in), oe, fe, ot == ft)
					}
				}
			}
			t.Logf("%d rules run their Flow bodies", n)
		})
	}
	if total == 0 {
		t.Fatal("no rule of the testdata grammars has a Flow body")
	}
}

// flatNamingGap lists, by grammar, the rules whose whole-rule Flow function returns a value that the
// rule frame does not name after the rule (TestFlowBodiesTestdataGrammars).
var flatNamingGap = map[string][]string{
	"duckdb": {"cmp_op", "op_arrow", "op_idiv", "op_plus", "op_star"},
}
