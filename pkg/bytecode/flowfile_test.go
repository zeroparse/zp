package bytecode_test

import (
	"bytes"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/pkg/bytecode"
)

// flowModule returns a module of two rules, the second with a Flow body that uses every operation
// and every kind of immediate, and a class for its guard and class test. The instructions after
// the first RETURN are unreachable; they are there for the encoding.
func flowModule() *bytecode.Module {
	f := &bytecode.Flow{Regs: 8, Code: []bytecode.FlowInstr{
		{Op: bytecode.FlowConst, Dst: []int{0}},
		{Op: bytecode.FlowConst, Dst: []int{1}, Imm: -1 << 40},
		{Op: bytecode.FlowConst, Dst: []int{2}, Imm: true},
		{Op: bytecode.FlowConst, Dst: []int{3}, Imm: false},
		{Op: bytecode.FlowPos, Dst: []int{1}},
		{Op: bytecode.FlowMark, Dst: []int{4}},
		{Op: bytecode.FlowPeek, Dst: []int{5, -1, 6}},
		{Op: bytecode.FlowGuard, Dst: []int{7}, Args: []int{5, 6}, Imm: bytecode.FlowGuardOf{Lit: true, Rune: 'é', Fold: true, Class: -1, Depth: 2}},
		{Op: bytecode.FlowGuard, Dst: []int{7}, Args: []int{5, 6}, Imm: bytecode.FlowGuardOf{Class: 0}},
		{Op: bytecode.FlowJumpFalse, Args: []int{7}, Target: 14},
		{Op: bytecode.FlowExpect, Args: []int{1}, Imm: bytecode.FlowExpectOf{Exp: bytecode.FixedExps + 1, Message: true}},
		{Op: bytecode.FlowReset, Args: []int{4}},
		{Op: bytecode.FlowSetPos, Args: []int{1}},
		{Op: bytecode.FlowJump, Target: 17},
		{Op: bytecode.FlowCall, Dst: []int{0, 7}, Imm: bytecode.FlowCallOf{Rule: 0, Level: 3}},
		{Op: bytecode.FlowUnfresh, Args: []int{0}},
		{Op: bytecode.FlowNot, Dst: []int{2}, Args: []int{7}},
		{Op: bytecode.FlowMove, Dst: []int{3}, Args: []int{2}},
		{Op: bytecode.FlowReturn, Args: []int{0, 3}},
		{Op: bytecode.FlowAdd, Dst: []int{1}, Args: []int{1, 1}},
		{Op: bytecode.FlowSub, Dst: []int{1}, Args: []int{1, 1}},
		{Op: bytecode.FlowEq, Dst: []int{2}, Args: []int{1, 1}},
		{Op: bytecode.FlowNe, Dst: []int{2}, Args: []int{1, 1}},
		{Op: bytecode.FlowLt, Dst: []int{2}, Args: []int{5, 1}},
		{Op: bytecode.FlowLe, Dst: []int{2}, Args: []int{1, 1}},
		{Op: bytecode.FlowStep},
		{Op: bytecode.FlowSilent, Imm: bytecode.FlowSilenceOf{Delta: 1, Site: 3}},
		{Op: bytecode.FlowSilent, Imm: bytecode.FlowSilenceOf{Delta: -1, Site: -1, Ok: true}},
		{Op: bytecode.FlowAdvance, Args: []int{1}},
		{Op: bytecode.FlowInClass, Dst: []int{2}, Args: []int{5}, Imm: 0},
		{Op: bytecode.FlowAtAnchor, Dst: []int{2}, Imm: 3},
		{Op: bytecode.FlowMatchLit, Dst: []int{0, 2}, Imm: bytecode.FlowLitOf{Str: 0, Exp: bytecode.FixedExps, Fold: true}},
		{Op: bytecode.FlowIsolate, Dst: []int{4}},
		{Op: bytecode.FlowUnisolate, Dst: []int{1, 6}, Args: []int{4}},
		{Op: bytecode.FlowMergeExpected, Args: []int{1, 6}},
		{Op: bytecode.FlowNewVals, Dst: []int{4}, Imm: 2},
		{Op: bytecode.FlowText, Dst: []int{0}, Args: []int{1}},
		{Op: bytecode.FlowNewTerminal, Dst: []int{0}, Args: []int{1, 0}},
		{Op: bytecode.FlowAppend, Dst: []int{4}, Args: []int{4, 0}},
		{Op: bytecode.FlowSpread, Dst: []int{4}, Args: []int{4, 0}},
		{Op: bytecode.FlowNewSeq, Dst: []int{0}, Args: []int{1, 4}},
		{Op: bytecode.FlowNewList, Dst: []int{0}, Args: []int{1, 4}},
		{Op: bytecode.FlowLiteralText, Dst: []int{0}, Args: []int{1}, Imm: 0},
		{Op: bytecode.FlowStr, Dst: []int{0}, Imm: 1},
		{Op: bytecode.FlowKidsLen, Dst: []int{1}},
		{Op: bytecode.FlowPushKid, Args: []int{0}},
		{Op: bytecode.FlowKids, Dst: []int{4}, Args: []int{1}},
		{Op: bytecode.FlowDropKids, Args: []int{1}},
		{Op: bytecode.FlowShapeSingle, Dst: []int{0, 2}, Args: []int{0, 1}, Imm: bytecode.FlowShapeOf{Elems: true, Shared: true, K: 2, Mode: 2}},
		{Op: bytecode.FlowScopeValue, Dst: []int{0}, Args: []int{0, 1}, Imm: bytecode.FlowShapeOf{Tuple: true, K: 1}},
		{Op: bytecode.FlowSetCapture, Args: []int{0}, Imm: 0},
		{Op: bytecode.FlowMarkPos, Dst: []int{1}, Args: []int{4}},
		{Op: bytecode.FlowFrame, Dst: []int{5}},
		{Op: bytecode.FlowNewFrame, Dst: []int{6}, Imm: 0},
		{Op: bytecode.FlowSetFrame, Args: []int{6}},
		{Op: bytecode.FlowClearVals, Args: []int{6}},
		{Op: bytecode.FlowFrameVal, Dst: []int{0}, Args: []int{6}, Imm: 1},
		{Op: bytecode.FlowAttachFrame, Dst: []int{0}, Args: []int{0, 1, 6}, Imm: 0},
		{Op: bytecode.FlowShapeCapture, Dst: []int{0}, Args: []int{0, 1}, Imm: bytecode.FlowShapeOf{K: 1, Mode: 1}},
		{Op: bytecode.FlowRunHot, Dst: []int{2}, Imm: 1},
		{Op: bytecode.FlowRunBegin, Imm: bytecode.FlowRunWalkOf{Site: 1, Build: true, KeyEnv: true}},
		{Op: bytecode.FlowSkipVisit},
		{Op: bytecode.FlowSkipHit, Dst: []int{2}},
		{Op: bytecode.FlowSkipValue, Args: []int{0}},
		{Op: bytecode.FlowSkipKids, Dst: []int{4}},
		{Op: bytecode.FlowSkipEnd},
		{Op: bytecode.FlowRunWalked, Args: []int{1}, Imm: 1},
		{Op: bytecode.FlowScanReach, Dst: []int{1}, Args: []int{1, 1}, Imm: 5},
		{Op: bytecode.FlowScanEnd, Args: []int{1, 1}, Imm: 4},
		{Op: bytecode.FlowScanKids, Dst: []int{4}, Args: []int{1, 1, 1}, Imm: 5},
		{Op: bytecode.FlowPred, Dst: []int{2}, Imm: 0},
		{Op: bytecode.FlowBind, Dst: []int{2}, Imm: bytecode.FlowBindOf{Var: 0, Term: 0}},
		{Op: bytecode.FlowCut, Dst: []int{2}},
		{Op: bytecode.FlowSetCut, Args: []int{2}},
		{Op: bytecode.FlowReturn, Args: []int{0, 3}},
	}}
	return &bytecode.Module{
		Strings: []string{"a", "b", "[a-z]"},
		Classes: []bytecode.Class{{Ranges: []rune{'a', 'z'}, Desc: 2}},
		Rules: []bytecode.RuleInfo{
			{Name: 0, Action: -1, NodeType: -1, OperatorsRule: -1, Where: -1},
			{Name: 1, Action: -1, NodeType: -1, OperatorsRule: -1, Where: -1, Scope: []int{0}},
		},
		Scopes:  [][]int{{0, 1}},
		RunKeys: []bytecode.RunKeyInfo{{Site: 1}},
		Code:    []bytecode.Instr{{Op: bytecode.OpReturn}},
		Starts:  []int{0},
		Terms:   []bytecode.Term{{}},
		Vars:    []bytecode.VarInfo{{Str: -1}},
		Flows:   map[int]*bytecode.Flow{1: f},
	}
}

func writeModule(m *bytecode.Module) []byte {
	w := &bytecode.Encoder{Strs: map[string]int{}}
	w.WriteModule(m)
	return w.Body.Bytes()
}

func noFieldTypes(string) error { return nil }

// TestFlowBodyEncoding checks that a Flow body survives writing and reading with every operation
// and kind of immediate, that the loaded module validates, and that reading rejects an unknown
// operation, a constant of an unknown kind or a value its kind does not have, and a rule with two
// bodies.
func TestFlowBodyEncoding(t *testing.T) {
	m := flowModule()
	if err := bytecode.ValidateModule(m, noFieldTypes); err != nil {
		t.Fatal(err)
	}
	data := writeModule(m)
	r := &bytecode.Decoder{Data: data, Strs: m.Strings}
	got := r.ReadModule()
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if r.Pos != len(data) {
		t.Fatalf("read %d of %d bytes", r.Pos, len(data))
	}
	if !reflect.DeepEqual(got.Flows, m.Flows) {
		t.Fatalf("flows read back as %+v, want %+v", got.Flows[1], m.Flows[1])
	}
	if err := bytecode.ValidateModule(got, noFieldTypes); err != nil {
		t.Fatal(err)
	}
	// The body is written last: one body, rule 1 (zigzag 2), 8 registers, then its instructions;
	// the last is RETURN with the arguments 0 and 3 (zigzag 0 and 6).
	ret := []byte{byte(bytecode.FlowReturn), 0, 6}
	if !bytes.HasSuffix(data, ret) {
		t.Fatalf("the module does not end with the RETURN of the body: % x", data[len(data)-3:])
	}
	head := bytes.LastIndex(data, []byte{1, 2, 8, byte(len(m.Flows[1].Code))})
	if head < 0 {
		t.Fatal("no header of the body")
	}
	// A body with one instruction, a CONST of each tail, so that the bytes after the header are
	// the instruction alone.
	one := func(in ...byte) []byte {
		return append(append(bytes.Clone(data[:head]), 1, 2, 8, 1), in...)
	}
	two := append(bytes.Clone(data[:head]), 2)
	two = append(two, data[head+1:]...)
	two = append(two, data[head+1:]...)
	for _, c := range []struct {
		name string
		data []byte
		want string
	}{
		{"unknown operation", one(0x7f), "invalid flow operation"},
		{"constant of an unknown kind", one(byte(bytecode.FlowConst), 0, 9, 0), "invalid flow constant"},
		{"boolean constant 2", one(byte(bytecode.FlowConst), 0, bytecode.ConstBool, 4), "invalid flow constant"},
		{"nil constant 1", one(byte(bytecode.FlowConst), 0, bytecode.ConstNil, 2), "invalid flow constant"},
		{"two bodies of a rule", two, "has two Flow bodies"},
	} {
		r := &bytecode.Decoder{Data: c.data, Strs: m.Strings}
		r.ReadModule()
		if r.Err == nil || !strings.Contains(r.Err.Error(), c.want) {
			t.Errorf("%s: error %v, want %q", c.name, r.Err, c.want)
		}
	}
}

// TestFlowBodyValidation checks that loading rejects a Flow body whose rule, registers, targets,
// shape or immediates are out of range, or that can run past its end.
func TestFlowBodyValidation(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(m *bytecode.Module, f *bytecode.Flow)
		want string
	}{
		{"rule", func(m *bytecode.Module, f *bytecode.Flow) { m.Flows[2] = f }, "flow body of rule 2 out of range"},
		{"destination", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[0].Dst[0] = 8 }, "register 8 out of range"},
		{"discarded argument", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[15].Args[0] = -1 }, "register -1 out of range"},
		{"destination below -1", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[6].Dst[1] = -2 }, "register -2 out of range"},
		{"target", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[13].Target = len(f.Code) }, "invalid jump"},
		{"target of no jump", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[15].Target = 1 }, "invalid jump"},
		{"arguments", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[18].Args = f.Code[18].Args[:1] }, "RETURN at 18 with 0 destinations and 1 arguments"},
		{"operation", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[15].Op = 0x7f }, "invalid flow operation"},
		{"end", func(m *bytecode.Module, f *bytecode.Flow) { f.Code = f.Code[:17] }, "does not end with JUMP or RETURN"},
		{"empty", func(m *bytecode.Module, f *bytecode.Flow) { f.Code = nil }, "does not end with JUMP or RETURN"},
		{"constant", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[0].Imm = "s" }, "invalid immediate"},
		{"immediate of no kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[15].Imm = 1 }, "invalid immediate"},
		{"guard kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[7].Imm = bytecode.FlowCallOf{} }, "invalid immediate"},
		{"class", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[8].Imm = bytecode.FlowGuardOf{Class: 1} }, "invalid guard class 1"},
		{"class of a literal", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[7].Imm = bytecode.FlowGuardOf{Lit: true} }, "invalid guard class 0"},
		{"folded class", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[8].Imm = bytecode.FlowGuardOf{Fold: true} }, "invalid guard class 0"},
		{"depth", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[8].Imm = bytecode.FlowGuardOf{Depth: -1} }, "invalid guard depth"},
		{"expectation", func(m *bytecode.Module, f *bytecode.Flow) {
			f.Code[10].Imm = bytecode.FlowExpectOf{Exp: bytecode.FixedExps + len(m.Strings)}
		}, "expectation 8 out of range"},
		{"called rule", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[14].Imm = bytecode.FlowCallOf{Rule: 2} }, "rule 2 out of range"},
		{"level", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[14].Imm = bytecode.FlowCallOf{Level: -1} }, "invalid binding level"},
		{"tested class", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[29].Imm = 1 }, "class 1 out of range"},
		{"class kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[29].Imm = true }, "invalid immediate"},
		{"anchor", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[30].Imm = 4 }, "invalid anchor 4"},
		{"negative anchor", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[30].Imm = -1 }, "invalid anchor -1"},
		{"literal", func(m *bytecode.Module, f *bytecode.Flow) {
			f.Code[31].Imm = bytecode.FlowLitOf{Str: len(m.Strings), Exp: bytecode.FixedExps}
		}, "literal 3 out of range"},
		{"literal expectation", func(m *bytecode.Module, f *bytecode.Flow) {
			f.Code[31].Imm = bytecode.FlowLitOf{Exp: bytecode.FixedExps + len(m.Strings)}
		}, "expectation 8 out of range"},
		{"literal kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[31].Imm = bytecode.FlowExpectOf{} }, "invalid immediate"},
		{"lookahead delta", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[26].Imm = bytecode.FlowSilenceOf{Delta: 2} }, "invalid lookahead"},
		{"lookahead site", func(m *bytecode.Module, f *bytecode.Flow) {
			f.Code[26].Imm = bytecode.FlowSilenceOf{Delta: 1, Site: -2}
		}, "invalid lookahead"},
		{"lookahead entered matched", func(m *bytecode.Module, f *bytecode.Flow) {
			f.Code[26].Imm = bytecode.FlowSilenceOf{Delta: 1, Ok: true}
		}, "invalid lookahead"},
		{"lookahead kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[27].Imm = 1 }, "invalid immediate"},
		{"immediate of a step", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[25].Imm = 1 }, "invalid immediate"},
		{"count", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[35].Imm = -1 }, "invalid count -1"},
		{"count kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[35].Imm = true }, "invalid immediate"},
		{"string", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[43].Imm = len(m.Strings) }, "string 3 out of range"},
		{"literal text", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[42].Imm = -1 }, "string -1 out of range"},
		{"string kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[43].Imm = "a" }, "invalid immediate"},
		{"shape mode", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[48].Imm = bytecode.FlowShapeOf{Mode: 3} }, "invalid shape"},
		{"shape count", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[49].Imm = bytecode.FlowShapeOf{K: -1} }, "invalid shape"},
		{"shape kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[49].Imm = 1 }, "invalid immediate"},
		{"immediate of a push", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[45].Imm = 1 }, "invalid immediate"},
		{"slot", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[56].Imm = 2 }, "capture slot 2 out of range"},
		{"negative slot", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[50].Imm = -1 }, "capture slot -1 out of range"},
		{"slot past the rule's frame", func(m *bytecode.Module, f *bytecode.Flow) {
			f.Code[53] = bytecode.FlowInstr{Op: bytecode.FlowFrame, Dst: []int{6}}
			f.Code[56].Imm = 1
		}, "capture slot 1 out of range"},
		{"slot kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[50].Imm = bytecode.FlowShapeOf{} }, "invalid immediate"},
		{"scope", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[57].Imm = 1 }, "scope 1 out of range"},
		{"scope of a frame", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[53].Imm = -1 }, "scope -1 out of range"},
		{"scope kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[57].Imm = nil }, "invalid immediate"},
		{"run site", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[59].Imm = 2 }, "invalid run site 2"},
		{"run site zero", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[66].Imm = 0 }, "invalid run site 0"},
		{"run walk site", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[60].Imm = bytecode.FlowRunWalkOf{Site: 2} }, "invalid run site 2"},
		{"run walk key", func(m *bytecode.Module, f *bytecode.Flow) { m.RunKeys = nil }, "run site 1 is keyed by the variables but has no run key"},
		{"run walk kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[60].Imm = 1 }, "invalid immediate"},
		{"scan site", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[67].Imm = 6 }, "invalid scan site 6"},
		{"scan site zero", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[69].Imm = 0 }, "invalid scan site 0"},
		{"term", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[70].Imm = 1 }, "term 1 out of range"},
		{"term kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[70].Imm = nil }, "invalid immediate"},
		{"bind variable", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[71].Imm = bytecode.FlowBindOf{Var: 1} }, "variable 1 out of range"},
		{"bind term", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[71].Imm = bytecode.FlowBindOf{Term: -1} }, "term -1 out of range"},
		{"bind kind", func(m *bytecode.Module, f *bytecode.Flow) { f.Code[71].Imm = 0 }, "invalid immediate"},
	} {
		m := flowModule()
		c.edit(m, m.Flows[1])
		if err := bytecode.ValidateModule(m, noFieldTypes); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want %q", c.name, err, c.want)
		}
	}
}

// TestFlowShapes checks that every operation has a shape with a name, so that a module file can
// write it and a disassembly can show it.
func TestFlowShapes(t *testing.T) {
	for op, s := range bytecode.FlowShapes {
		if s.Name == "" {
			t.Errorf("operation %d has no shape", op)
		}
	}
	if _, ok := bytecode.FlowOp(len(bytecode.FlowShapes)).Shape(); ok {
		t.Error("an operation past the table has a shape")
	}
}

// TestFlowBodyDisassembly checks that the disassembly shows each Flow body under its rule, with the
// registers, targets and immediates of its instructions.
func TestFlowBodyDisassembly(t *testing.T) {
	got := flowModule().Disassemble()
	for _, want := range []string{
		"flow b regs=8:\n",
		"     0  CONST      r0 = nil\n",
		"     1  CONST      r1 = -1099511627776\n",
		"     6  PEEK       r5,_,r6 =\n",
		"     7  GUARD      r7 = r5 r6 'é' fold depth=2\n",
		"     8  GUARD      r7 = r5 r6 \"[a-z]\" depth=0\n",
		"     9  JUMPFALSE  r7 -> 14\n",
		"    10  EXPECT     r1 \"b\" message\n",
		"    14  CALL       r0,r7 = a level=3\n",
		"    18  RETURN     r0 r3\n",
		"    19  ADD        r1 = r1 r1\n",
		"    25  STEP       \n",
		"    26  SILENT     delta=1 site=3 ok=false\n",
		"    29  INCLASS    r2 = r5 \"[a-z]\"\n",
		"    30  ATANCHOR   r2 = 3\n",
		"    31  MATCHLIT   r0,r2 = \"a\" fold \"a\"\n",
		"    33  UNISOLATE  r1,r6 = r4\n",
		"    34  MERGEEXPECTED r1 r6\n",
		"    35  NEWVALS    r4 = 2\n",
		"    42  LITERALTEXT r0 = r1 \"a\"\n",
		"    43  STR        r0 = \"b\"\n",
		"    48  SHAPESINGLE r0,r2 = r0 r1 elems=true shared=true tuple=false k=2 mode=2\n",
		"    49  SCOPEVALUE r0 = r0 r1 elems=false shared=false tuple=true k=1 mode=0\n",
		"    50  SETCAPTURE r0 slot=0\n",
		"    51  MARKPOS    r1 = r4\n",
		"    53  NEWFRAME   r6 = scope=[a b]\n",
		"    56  FRAMEVAL   r0 = r6 slot=1\n",
		"    57  ATTACHFRAME r0 = r0 r1 r6 scope=[a b]\n",
		"    58  SHAPECAPTURE r0 = r0 r1 elems=false shared=false tuple=false k=1 mode=1\n",
		"    59  RUNHOT     r2 = site=1\n",
		"    60  RUNBEGIN   site=1 build=true keyenv=true\n",
		"    66  RUNWALKED  r1 site=1\n",
		"    69  SCANKIDS   r4 = r1 r1 r1 site=5\n",
		"    70  PRED       r2 = term=0\n",
		"    71  BIND       r2 = var=0 term=0\n",
		"    72  CUT        r2 =\n",
		"    73  SETCUT     r2\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("disassembly has no %q:\n%s", want, got)
		}
	}
}

// TestSpecFlowOperations checks that the instruction table of Flow Bodies in spec/bytecode/README.md
// gives each operation the code, name and numbers of registers of its shape: a module stores the
// codes and reads as many registers as the shape says, so a change to either changes the file
// format.
func TestSpecFlowOperations(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../spec/bytecode/README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, sec, ok := strings.Cut(string(data), "\n## Flow Bodies\n")
	if !ok {
		t.Fatal("spec/bytecode/README.md has no Flow Bodies section")
	}
	sec, _, _ = strings.Cut(sec, "\n## ")
	rows := regexp.MustCompile("(?m)^\\| (\\d+) \\| `([A-Z]+)` \\| (\\d+) \\| (\\d+) \\|").FindAllStringSubmatch(sec, -1)
	if len(rows) != len(bytecode.FlowShapes) {
		t.Fatalf("the specification lists %d operations; the runtime has %d", len(rows), len(bytecode.FlowShapes))
	}
	for _, r := range rows {
		code, _ := strconv.Atoi(r[1])
		d, _ := strconv.Atoi(r[3])
		a, _ := strconv.Atoi(r[4])
		s, ok := bytecode.FlowOp(code).Shape()
		if !ok || s.Name != r[2] || s.Dst != d || s.Args != a {
			t.Errorf("the specification has %s as code %d with %d destinations and %d arguments; the runtime has %+v", r[2], code, d, a, s)
		}
	}
}
