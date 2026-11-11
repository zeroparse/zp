package engine_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// engineMutant changes the printed code of the operators of terms in the engine in one way: in the
// compositions of internal/engine/core/term_exec.go, or in the primitives of term_value.go.
type engineMutant struct {
	name, file, fn, old, new string
}

var engineMutants = []engineMutant{
	{"swapped operands of -", "term_exec.go", "termBinary", "any(l.(int) - r.(int))", "any(r.(int) - l.(int))"},
	{"a node is equal to a node", "term_exec.go", "termEqual", "return false, false\n}", "return true, true\n}"},
	{"an int equals a bool", "term_exec.go", "termEqual", "if ka != kb {\n\t\treturn false, false\n\t}", "if ka != kb {\n\t\treturn false, true\n\t}"},
	{"an index may be the length", "term_value.go", "intIndex", "k >= n", "k > n"},
	{"the least index is 1", "term_value.go", "intIndex", "k < 0", "k < 1"},
	{"a division by zero succeeds", "term_exec.go", "termBinary", "if r.(int) == 0 {", "if false {"},
	{"< is <=", "term_exec.go", "termBinary", "intCmp(l, r) < 0", "intCmp(l, r) <= 0"},
	{"strings compare as ints", "term_exec.go", "termBinary", "p.strCmp(l, r) < 0", "p.strCmp(l, r) > 0"},
	{"! of a bool does not negate", "term_exec.go", "termUnary", "return any(!x.(bool)), nil", "return any(x.(bool)), nil"},
	{"a logical operand need not be a bool", "term_exec.go", "termLogic", "if valKind(x) != 3 {", "if false {"},
	{"the controlled concatenation drops the right string", "term_value.go", "strAdd", "for _, s := range [...]string{ls, rs} {", "for _, s := range [...]string{ls} {"},
	{"the controlled equality ignores the content", "budget_value.go", "equalText", "return len(ls) == len(rs) && p.CompareText(ls, rs) == 0", "return len(ls) == len(rs)"},
	{"the controlled comparison orders a prefix after its extension", "budget_value.go", "CompareText", "if len(l) < len(r) {\n\t\treturn -1", "if len(l) < len(r) {\n\t\treturn 1"},
	{"the fast path of the value equality ignores the control", "budget_value.go", "ValueEqual", "return p.equalText(ls, rs)", "return ls == rs"},
	{"the fast path of the value equality negates", "budget_value.go", "ValueEqual", "return p.equalText(ls, rs)", "return !p.equalText(ls, rs)"},
	{"a remainder floors", "term_exec.go", "termBinary", `any(l.(int) % r.(int))`, `any(func() int { m := l.(int) % r.(int); if m != 0 && (m < 0) != (r.(int) < 0) { m += r.(int) }; return m }())`},
	{"a division floors", "term_exec.go", "termBinary", `any(l.(int) / r.(int))`, `any(func() int { q := l.(int) / r.(int); if (l.(int)%r.(int) != 0) && ((l.(int) < 0) != (r.(int) < 0)) { q-- }; return q }())`},
	{"a box is not opened", "term_value.go", "unboxed", "return unboxed(value.D(n).Box)", "return v"},
	{"Start reads the value", "term_exec.go", "termMember", "if kind == 1 {", "if kind == 0 {"},
	{"an unset field of a struct is nil", "term_exec.go", "termMember", "if !st {\n\t\treturn nil, nil\n\t}", "if true {\n\t\treturn nil, nil\n\t}"},
	{"a struct has captures", "term_exec.go", "termMember", "if st {\n\t\t\treturn nil, p.termError(9", "if false {\n\t\t\treturn nil, p.termError(9"},
	{"a set field of a struct reads as its zero", "term_exec.go", "termMember", "if set {\n\t\treturn f, nil\n\t}", "if set && !st {\n\t\treturn f, nil\n\t}"},
	{"a capture keeps its box", "term_exec.go", "termMember", "return unboxed(v), nil", "return v, nil"},
	{"End is a field", "term_exec.go", "memberKind", "return memberEnd, name", "return memberField, name"},
	{"a struct declares every field", "eval.go", "memberZero", "if !p.HasValueField(", "if false && !p.HasValueField("},
	{"nil has length 1", "term_exec.go", "termLength", "return any(0), nil", "return any(1), nil"},
	{"the length of a string node is that of its text", "term_exec.go", "termLength", "if value.D(n).Terminal {", "if false {"},
	{"a bool is not an element", "term_exec.go", "termElement", "if k == 1 || k == 2 || k == 3 {", "if k == 1 || k == 2 {"},
	{"a function is an element", "term_exec.go", "termElement", "return nil, p.termError(11, v, any(fn), 0, 0)", "return nil, nil"},
	{"concat takes any value", "term_exec.go", "termItems", "return items, p.termError(12, v, any(fn), 0, 0)", "return items, nil"},
	{"the text of a string node is the input", "term_exec.go", "termText", "if held {", "if held && false {"},
	{"a list value converts", "term_exec.go", "termConvert", "if value.HasBase(n) {", "if true {"},
	{"a value that does not convert is nil", "term_exec.go", "termConvert", "return nil, p.termError(15, x, any(t[0].Type), 0, 0)", "return nil, nil"},
	{"a node field value does not cover", "term_exec.go", "termCover", "if !ok {\n\t\treturn first", "if !ok && false {\n\t\treturn first"},
	{"the start of the range is the latest", "term_exec.go", "termCover", "cs < ns", "cs > ns"},
	{"the end of the range is the earliest", "term_exec.go", "termCover", "ce > ne", "ce < ne"},
	{"a later node replaces the range", "term_exec.go", "termCover", "if first || cs < ns {", "if true {"},
	{"a struct without a node keeps no range", "term_exec.go", "termMade", "if first {", "if false {"},
	{"a struct is not recorded", "term_exec.go", "termMade", "c.made(n)", ""},
	{"an element of a hull charges no step", "term_exec.go", "termHull", "p.Spend()", ""},
	{"an element that adds no range widens the range", "term_exec.go", "termHull", "if has {", "if has || true {"},
	{"the start of a hull is the latest", "term_exec.go", "termHull", "es < lo", "es > lo"},
	{"the end of a hull is the earliest", "term_exec.go", "termHull", "ee > hi", "ee < hi"},
	{"a later element replaces the range of a hull", "term_exec.go", "termHull", "if first || es < lo {", "if true {"},
	{"an empty hull is not at the start of the rule", "term_exec.go", "termHull", "lo := rs", "lo := 0"},
	{"a fold step charges nothing", "eval.go", "Builtin", "c.P.Spend()\n\t\t\tif acc, err = f.Apply(acc", "if acc, err = f.Apply(acc"},
	{"foldr passes the elements from the first", "term_exec.go", "termItemAt", "if right {", "if false {"},
	{"the folds pass the elements in each other's order", "term_exec.go", "termItemAt", "if right {", "if !right {"},
	{"an iteration passes a box", "term_exec.go", "termItemAt", "return value.UnboxNode(items[i])", "return items[i]"},
	{"filter takes a result that is not a bool", "term_exec.go", "termFilterKeep", "if valKind(v) != 3 {", "if valKind(v) == 0 {"},
	{"filter drops what it keeps", "term_exec.go", "termFilterKeep", "return v.(bool), nil", "return !v.(bool), nil"},
	{"an element of a list charges no step", "term_exec.go", "termElemsConform", "p.Spend()", ""},
	{"an element of a tuple charges no step", "term_exec.go", "termTupleConform", "p.Spend()", ""},
	{"a tuple of another length conforms", "term_exec.go", "termTupleConform", "if count != len(t.Alts) {", "if count > len(t.Alts) {"},
	{"a list type takes any value", "term_exec.go", "termConforms", "if !(shape == 6) {", "if false {"},
	{"a tuple type takes a list value", "term_exec.go", "termConforms", "if !(shape == 5) {", "if !(shape == 5 || shape == 6) {"},
	{"an optional type does not take nil", "term_exec.go", "termConforms", "return kx == 0 || p.termConforms(t.Elem, x)", "return p.termConforms(t.Elem, x)"},
	{"a join takes the value of no alternative", "term_exec.go", "termConforms", "if p.termConforms(t.Alts[i], x) {", "if !p.termConforms(t.Alts[i], x) {"},
	{"a named type takes any value", "term_exec.go", "termConforms", "return n.Type() == t.Name", "return true"},
	{"a node whose value is a list has any elements", "term_exec.go", "termValueConforms", "if !(shape == 2) {", "if false {"},
	{"a box is not opened by a cast", "term_exec.go", "termConforms", "x := unboxed(v)", "x := v"},
}

// TestPrintedOperatorMutants is the proof that TestEngineValueConformance, TestEngineValueBoxedOperands,
// TestEngineValueMemberTable, TestEngineValueBuiltinTable and TestEngineValueStructTable hold the printed code of the operators,
// the member reads, the built-in functions, the conversions and the ranges of structs of terms in the engine: with a source overlay of each mutant, one
// of them fails.
func TestPrintedOperatorMutants(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	root := gentest.ModuleRoot()
	for _, m := range engineMutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, "internal", "engine", "core", m.file)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			mutated, ok := gentest.MutateFunc(string(src), m.fn, m.old, m.new)
			if !ok {
				t.Fatalf("%s has no %s with %q", m.file, m.fn, m.old)
			}
			dir := t.TempDir()
			file := filepath.Join(dir, m.file)
			if err := os.WriteFile(file, []byte(mutated), 0o644); err != nil {
				t.Fatal(err)
			}
			overlay, _ := json.Marshal(map[string]map[string]string{"Replace": {path: file}})
			overlayFile := filepath.Join(dir, "overlay.json")
			if err := os.WriteFile(overlayFile, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "test", "-count=1", "-overlay", overlayFile, "-run", "^TestEngineValue(Conformance|BoxedOperands|EqualCharges|MemberTable|BuiltinTable|StructTable|HullTable|IterTable|CastTable)$", "./internal/engine/core")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GOFLAGS=")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the value tests of the engine pass with the mutant %q:\n%s", m.name, out)
			}
			if !strings.Contains(string(out), "--- FAIL: TestEngineValue") {
				t.Fatalf("the mutant %q fails no value test of the engine:\n%s", m.name, out)
			}
		})
	}
}
