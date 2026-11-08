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
	{"the typed string fast path negates ==", "budget.go", "compareStrings", "return p.TextEqual(l, r), true", "return !p.TextEqual(l, r), true"},
	{"a remainder floors", "term_exec.go", "termBinary", `any(l.(int) % r.(int))`, `any(func() int { m := l.(int) % r.(int); if m != 0 && (m < 0) != (r.(int) < 0) { m += r.(int) }; return m }())`},
	{"a division floors", "term_exec.go", "termBinary", `any(l.(int) / r.(int))`, `any(func() int { q := l.(int) / r.(int); if (l.(int)%r.(int) != 0) && ((l.(int) < 0) != (r.(int) < 0)) { q-- }; return q }())`},
	{"a box is not opened", "term_exec.go", "termEqual", "a := UnboxValue(l)", "a := l"},
	{"! of a bool does not negate", "term_exec.go", "termUnary", "return any(!x.(bool)), nil", "return any(x.(bool)), nil"},
	{"a logical operand need not be a bool", "term_exec.go", "termLogic", "if valKind(x) != 3 {", "if false {"},
}

// TestPrintedOperatorMutants is the proof that the value conformance holds the printed code of the
// operators of terms in the generated Go runtime: with the source of each mutant, TestValueConformance
// fails. A mutant that no test notices would be one that the model and the code do not both bound.
func TestPrintedOperatorMutants(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	for _, m := range goMutants {
		// The compositions are printed for each runtime; the primitives are plain functions.
		heads := []string{"func (p *Parser) ", "func (p *TParser) "}
		if m.file == "term_value.go" {
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
				cmd := exec.Command("go", "test", "-count=1", "-run", "^TestValue(Conformance|BoxedOperands|CompareStrings)$", ".")
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
