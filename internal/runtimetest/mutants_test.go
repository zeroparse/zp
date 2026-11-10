package runtimetest

import (
	"os/exec"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// A mutant changes the printed code of the operators of terms in one way: in the compositions of
// runtime/term_exec.go, or in the primitives of runtime/term_value.go. Each is in the node runtime
// and in the typed one, whose compositions are printed apart (the receiver *TParser).
type goMutant struct {
	name, file, fn, old, new string
}

var goMutants = []goMutant{
	{"swapped operands of -", "term_exec.go", "termBinary", "any(l.(int) - r.(int))", "any(r.(int) - l.(int))"},
	{"a node is equal to a node", "term_exec.go", "termEqual", "return false, false\n}", "return true, true\n}"},
	{"an int equals a bool", "term_exec.go", "termEqual", "if ka != kb {\n\t\treturn false, false\n\t}", "if ka != kb {\n\t\treturn false, true\n\t}"},
	{"an index may be the length", "term_value.go", "intIndex", "k >= n", "k > n"},
	{"the least index is 1", "term_value.go", "intIndex", "k < 0", "k < 1"},
	{"a division by zero succeeds", "term_exec.go", "termBinary", "if r.(int) == 0 {", "if false {"},
	{"< is <=", "term_exec.go", "termBinary", "intCmp(l, r) < 0", "intCmp(l, r) <= 0"},
	{"strings compare as ints", "term_exec.go", "termBinary", "p.strCmp(l, r) < 0", "p.strCmp(l, r) > 0"},
	{"the controlled concatenation drops the right string", "term_value.go", "strAdd", "for _, s := range [...]string{ls, rs} {", "for _, s := range [...]string{ls} {"},
	{"the controlled equality ignores the content", "term_value.go", "strEq", "return p.TextEqual(ls, rs)", "return len(ls) == len(rs)"},
	{"the controlled comparison orders a prefix after its extension", "budget.go", "compareTextControlled", "if len(l) < len(r) {\n\t\treturn -1", "if len(l) < len(r) {\n\t\treturn 1"},
	{"the typed string fast path orders < as <=", "budget.go", "compareStrings", "return p.compareText(l, r) < 0, true", "return p.compareText(l, r) <= 0, true"},
	{"the typed string fast path decides == without the budget control", "budget.go", "compareStrings", "case OpEq:\n\t\treturn p.TextEqual(l, r), true", "case OpEq:\n\t\treturn l == r, true"},
	{"the typed string fast path negates ==", "budget.go", "compareStrings", "return p.TextEqual(l, r), true", "return !p.TextEqual(l, r), true"},
	{"a remainder floors", "term_exec.go", "termBinary", `any(l.(int) % r.(int))`, `any(func() int { m := l.(int) % r.(int); if m != 0 && (m < 0) != (r.(int) < 0) { m += r.(int) }; return m }())`},
	{"a division floors", "term_exec.go", "termBinary", `any(l.(int) / r.(int))`, `any(func() int { q := l.(int) / r.(int); if (l.(int)%r.(int) != 0) && ((l.(int) < 0) != (r.(int) < 0)) { q-- }; return q }())`},
	{"a box is not opened", "term_exec.go", "termEqual", "a := UnboxValue(l)", "a := l"},
	{"! of a bool does not negate", "term_exec.go", "termUnary", "return any(!x.(bool)), nil", "return any(x.(bool)), nil"},
	{"a logical operand need not be a bool", "term_exec.go", "termLogic", "if valKind(x) != 3 {", "if false {"},
	{"Start reads the value", "term_exec.go", "termMember", "if kind == 1 {", "if kind == 0 {"},
	{"an unset field of a struct is nil", "term_exec.go", "termMember", "if !st {\n\t\treturn nil, nil\n\t}", "if true {\n\t\treturn nil, nil\n\t}"},
	{"a struct has captures", "term_exec.go", "termMember", "if st {\n\t\t\treturn nil, p.termError(9", "if false {\n\t\t\treturn nil, p.termError(9"},
	{"a set field of a struct reads as its zero", "term_exec.go", "termMember", "if set {\n\t\treturn f, nil\n\t}", "if set && !st {\n\t\treturn f, nil\n\t}"},
	{"a capture keeps its box", "term_exec.go", "termMember", "return UnboxValue(v), nil", "return v, nil"},
	{"End is a field", "term_exec.go", "memberKind", "return memberEnd, name", "return memberField, name"},
	{"nil has length 1", "term_exec.go", "termLength", "return any(0), nil", "return any(1), nil"},
	{"a string has no length", "term_exec.go", "termLength", "if k == 2 {", "if false {"},
	{"a bool is not an element", "term_exec.go", "termElement", "if k == 1 || k == 2 || k == 3 {", "if k == 1 || k == 2 {"},
	{"a function is an element", "term_exec.go", "termElement", "return nil, p.termError(11, v, any(fn), 0, 0)", "return nil, nil"},
	{"concat takes any value", "term_exec.go", "termItems", "return items, p.termError(12, v, any(fn), 0, 0)", "return items, nil"},
	{"the text of a string is empty", "term_exec.go", "termText", "return x.(string), nil", "return \"\", nil"},
	{"a list value converts", "term_exec.go", "termConvert", "if ok {\n\t\tif ", "if ok {\n\t\tif true || "},
	{"a value that does not convert is nil", "term_exec.go", "termConvert", "return nil, p.termError(15, ", "return nil, nil\n\treturn nil, p.termError(15, "},
	{"a node field value does not cover", "term_exec.go", "termCover", "if !ok {\n\t\treturn first", "if !ok && false {\n\t\treturn first"},
	{"the start of the range is the latest", "term_exec.go", "termCover", "cs < ns", "cs > ns"},
	{"the end of the range is the earliest", "term_exec.go", "termCover", "ce > ne", "ce < ne"},
	{"a later node replaces the range", "term_exec.go", "termCover", "if first || cs < ns {", "if true {"},
	{"a struct without a node keeps no range", "term_exec.go", "termMade", "if first {", "if false {"},
	{"a struct is not recorded", "term_exec.go", "termMade", "c.made(n)", ""},
}

// TestPrintedOperatorMutants is the proof that the value conformance holds the printed code of the
// operators of terms and of the member reads in the generated Go runtime: with the source of each
// mutant, TestValueConformance, TestValueMemberTable, TestValueBuiltinTable or TestValueStructTable fails. A mutant that no test notices would be
// one that the model and the code do not both bound.
func TestPrintedOperatorMutants(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	for _, m := range goMutants {
		// The compositions are printed for each runtime; the primitives are plain functions.
		heads := []string{"func (p *Parser) ", "func (p *TParser) "}
		if m.file == "term_value.go" || m.fn == "memberKind" {
			heads = []string{"func "}
		}
		if m.file == "budget.go" || m.fn == "strAdd" || m.fn == "strEq" {
			heads = []string{"func (p *Parser) "}
		}
		if strings.Contains(m.old, "UnboxValue") {
			heads = heads[:1] // a typed value is never boxed
		}
		for _, head := range heads {
			t.Run(m.name+" "+strings.TrimSuffix(strings.TrimPrefix(head, "func "), " "), func(t *testing.T) {
				t.Parallel()
				done := false
				dir := runtimeCopy(t, func(name, src string) string {
					if name != m.file {
						return src
					}
					i := strings.Index(src, head+m.fn+"(")
					if i < 0 {
						return src
					}
					end := len(src)
					if j := strings.Index(src[i+1:], "\nfunc "); j >= 0 {
						end = i + 1 + j
					}
					out, ok := gentest.MutateFunc(src[i:end], m.fn, m.old, m.new)
					if !ok {
						return src
					}
					done = true
					return src[:i] + out + src[end:]
				})
				if !done {
					t.Fatalf("the runtime has no %s%s with %q", head, m.fn, m.old)
				}
				cmd := exec.Command("go", "test", "-count=1", "-run", "^TestValue(Conformance|BoxedOperands|CompareStrings|MemberTable|BuiltinTable|StructTable)$", ".")
				cmd.Dir = dir
				cmd.Env = append(cmd.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=-mod=mod -trimpath")
				out, err := cmd.CombinedOutput()
				if err == nil {
					t.Fatalf("the value tests pass with the mutant %q:\n%s", m.name, out)
				}
				if !strings.Contains(string(out), "--- FAIL: TestValue") {
					t.Fatalf("the mutant %q fails no value test:\n%s", m.name, out)
				}
			})
		}
	}
}
