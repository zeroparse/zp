package libgo_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// A mutant changes the printed code of the operators of terms in one way: in the compositions of
// runtime/term_exec.go, or in the primitives of runtime/term_value.go. Each is in the node library
// and in the typed one, whose compositions are printed apart (the receiver *TParser).
type goMutant struct {
	name, file, fn, old, new string
}

var goMutants = []goMutant{
	{"swapped operands of -", "term_exec.go", "TermBinary", "any(l.(int) - r.(int))", "any(r.(int) - l.(int))"},
	{"a node is equal to a node", "term_exec.go", "termEqual", "return false, false\n}", "return true, true\n}"},
	{"an int equals a bool", "term_exec.go", "termEqual", "if ka != kb {\n\t\treturn false, false\n\t}", "if ka != kb {\n\t\treturn false, true\n\t}"},
	{"an index may be the length", "term_value.go", "intIndex", "k >= n", "k > n"},
	{"the least index is 1", "term_value.go", "intIndex", "k < 0", "k < 1"},
	{"a division by zero succeeds", "term_exec.go", "TermBinary", "if r.(int) == 0 {", "if false {"},
	{"< is <=", "term_exec.go", "TermBinary", "intCmp(l, r) < 0", "intCmp(l, r) <= 0"},
	{"strings compare as ints", "term_exec.go", "TermBinary", "p.strCmp(l, r) < 0", "p.strCmp(l, r) > 0"},
	{"the controlled concatenation drops the right string", "term_value.go", "strAdd", "for _, s := range [...]string{ls, rs} {", "for _, s := range [...]string{ls} {"},
	{"the controlled equality ignores the content", "term_value.go", "strEq", "return p.TextEqual(ls, rs)", "return len(ls) == len(rs)"},
	{"the controlled comparison orders a prefix after its extension", "budget.go", "compareTextControlled", "if len(l) < len(r) {\n\t\treturn -1", "if len(l) < len(r) {\n\t\treturn 1"},
	{"the typed string fast path orders < as <=", "budget.go", "compareStrings", "return p.compareText(l, r) < 0, true", "return p.compareText(l, r) <= 0, true"},
	{"the typed string fast path decides == without the budget control", "budget.go", "compareStrings", "case OpEq:\n\t\treturn p.TextEqual(l, r), true", "case OpEq:\n\t\treturn l == r, true"},
	{"the typed string fast path negates ==", "budget.go", "compareStrings", "return p.TextEqual(l, r), true", "return !p.TextEqual(l, r), true"},
	{"a remainder floors", "term_exec.go", "TermBinary", `any(l.(int) % r.(int))`, `any(func() int { m := l.(int) % r.(int); if m != 0 && (m < 0) != (r.(int) < 0) { m += r.(int) }; return m }())`},
	{"a division floors", "term_exec.go", "TermBinary", `any(l.(int) / r.(int))`, `any(func() int { q := l.(int) / r.(int); if (l.(int)%r.(int) != 0) && ((l.(int) < 0) != (r.(int) < 0)) { q-- }; return q }())`},
	{"a box is not opened", "term_exec.go", "termEqual", "a := UnboxValue(l)", "a := l"},
	{"a boxed string compares without the budget control", "term_exec.go", "termEqual", "a := UnboxValue(l)\n\tb := UnboxValue(r)\n", "a := UnboxValue(l)\n\tb := UnboxValue(r)\n\tif _, boxed := l.(*core.Node); boxed {\n\t\tif sa, ok := a.(string); ok {\n\t\t\tif sb, ok := b.(string); ok {\n\t\t\t\treturn sa == sb, true\n\t\t\t}\n\t\t}\n\t}\n"},
	{"! of a bool does not negate", "term_exec.go", "TermUnary", "return any(!x.(bool)), nil", "return any(x.(bool)), nil"},
	{"a logical operand need not be a bool", "term_exec.go", "TermLogic", "if valKind(x) != 3 {", "if false {"},
	{"Start reads the value", "term_exec.go", "TermMember", "if kind == 1 {", "if kind == 0 {"},
	{"an unset field of a struct is nil", "term_exec.go", "TermMember", "if !st {\n\t\treturn nil, nil\n\t}", "if true {\n\t\treturn nil, nil\n\t}"},
	{"a struct has captures", "term_exec.go", "TermMember", "if st {\n\t\t\treturn nil, p.termError(9", "if false {\n\t\t\treturn nil, p.termError(9"},
	{"a set field of a struct reads as its zero", "term_exec.go", "TermMember", "if set {\n\t\treturn f, nil\n\t}", "if set && !st {\n\t\treturn f, nil\n\t}"},
	{"a capture keeps its box", "term_exec.go", "TermMember", "return UnboxValue(v), nil", "return v, nil"},
	{"End is a field", "term_exec.go", "memberKind", "return MemberEnd, name", "return MemberField, name"},
	{"nil has length 1", "term_exec.go", "TermLength", "return any(0), nil", "return any(1), nil"},
	{"a string has no length", "term_exec.go", "TermLength", "if k == 2 {", "if false {"},
	{"a bool is not an element", "term_exec.go", "TermPush", "if k == 1 || k == 2 || k == 3 {", "if k == 1 || k == 2 {"},
	{"a function is an element", "term_exec.go", "TermPush", "return p.termError(11, v, any(fn), 0, 0)", "return nil"},
	{"nil pushes no element", "term_exec.go", "TermPush", "p.KidStack = append(p.KidStack, nil)\n", ""},
	{"a node pushes no element", "term_exec.go", "TermPush", "p.KidStack = append(p.KidStack, n)\n", "_ = n\n"},
	{"concat pushes no elements", "term_exec.go", "TermPushItems", "\t\tp.KidStack = appendSpend(", "\t\t_ = appendSpend("},
	{"concat takes a value that is not a list", "term_exec.go", "TermPushItems", "if ok {", "if ok || true {"},
	{"concat charges nothing for the elements it pushes", "term_exec.go", "TermPushItems", "appendSpend(p, p.KidStack, items)", "append(p.KidStack, items...)"},
	{"typed concat charges nothing for the elements it pushes", "term_exec.go", "TermPushItems", "appendSpend(p.Parser, p.KidStack, items)", "append(p.KidStack, items...)"},
	{"concat takes any value", "term_exec.go", "TermItems", "return items, p.termError(12, v, any(fn), 0, 0)", "return items, nil"},
	{"the text of a string is empty", "term_exec.go", "TermText", "return x.(string), nil", "return \"\", nil"},
	{"a list value converts", "term_exec.go", "TermConvert", "if ok {\n\t\tif ", "if ok {\n\t\tif true || "},
	{"a value that does not convert is nil", "term_exec.go", "TermConvert", "return nil, p.termError(15, ", "return nil, nil\n\treturn nil, p.termError(15, "},
	{"a node field value does not cover", "term_exec.go", "TermCover", "if !ok {\n\t\treturn first", "if !ok && false {\n\t\treturn first"},
	{"the start of the range is the latest", "term_exec.go", "TermCover", "cs < ns", "cs > ns"},
	{"the end of the range is the earliest", "term_exec.go", "TermCover", "ce > ne", "ce < ne"},
	{"a later node replaces the range", "term_exec.go", "TermCover", "if first || cs < ns {", "if true {"},
	{"a struct without a node keeps no range", "term_exec.go", "TermMade", "if first {", "if false {"},
	{"a struct is not recorded", "term_exec.go", "TermMade", "c.made(n)", ""},
	{"an element of a hull charges no step", "term_exec.go", "TermHull", "p.Spend()", ""},
	{"an element that adds no range widens the range", "term_exec.go", "TermHull", "if has {", "if has || true {"},
	{"the start of a hull is the latest", "term_exec.go", "TermHull", "es < lo", "es > lo"},
	{"the end of a hull is the earliest", "term_exec.go", "TermHull", "ee > hi", "ee < hi"},
	{"a later element replaces the range of a hull", "term_exec.go", "TermHull", "if first || es < lo {", "if true {"},
	{"an empty hull is not at the start of the rule", "term_exec.go", "TermHull", "lo := rs", "lo := 0"},
	{"foldr passes the elements from the first", "term_exec.go", "TermItemAt", "if right {", "if false {"},
	{"the folds pass the elements in each other's order", "term_exec.go", "TermItemAt", "if right {", "if !right {"},
	{"an iteration passes a box", "term_exec.go", "TermItemAt", "return UnboxNode(items[i])", "return items[i]"},
	{"filter takes a result that is not a bool", "term_exec.go", "TermFilterKeep", "if valKind(v) != 3 {", "if valKind(v) == 0 {"},
	{"filter drops what it keeps", "term_exec.go", "TermFilterKeep", "return v.(bool), nil", "return !v.(bool), nil"},
	{"an element of a list charges no step", "term_exec.go", "termElemsConform", "p.Spend()", ""},
	{"an element of a tuple charges no step", "term_exec.go", "termTupleConform", "p.Spend()", ""},
	{"a tuple of another length conforms", "term_exec.go", "termTupleConform", "if count != len(t.Alts) {", "if count > len(t.Alts) {"},
	{"a list type takes any value", "term_exec.go", "TermConforms", "if !(shape == 6) {", "if false {"},
	{"a tuple type takes a list value", "term_exec.go", "TermConforms", "if !(shape == 5) {", "if !(shape == 5 || shape == 6) {"},
	{"an optional type does not take nil", "term_exec.go", "TermConforms", "return kx == 0 || p.TermConforms(t.Elem, x)", "return p.TermConforms(t.Elem, x)"},
	{"a join takes the value of no alternative", "term_exec.go", "TermConforms", "if p.TermConforms(t.Alts[i], x) {", "if !p.TermConforms(t.Alts[i], x) {"},
	{"a named type takes the values of other names", "term_exec.go", "TermConforms", " == t.Name", " != t.Name"},
	{"a node whose value is a list has any elements", "term_exec.go", "termValueConforms", "if !(shape == 2) {", "if false {"},
	{"a box is not opened by a cast", "term_exec.go", "TermConforms", "x := UnboxValue(v)", "x := v"},
}

// TestPrintedOperatorMutants is the proof that the value conformance holds the printed code of the
// operators of terms and of the member reads in the generated Go library: with the source of each
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
		if strings.Contains(m.old, "UnboxValue") || strings.Contains(m.old, "UnboxNode") {
			heads = heads[:1] // a typed value is never boxed
		}
		if strings.Contains(m.old, "(p.Parser,") {
			heads = heads[1:] // the typed library passes its embedded parser
		} else if strings.Contains(m.old, "Spend(p,") {
			heads = heads[:1]
		}
		for _, head := range heads {
			t.Run(m.name+" "+strings.TrimSuffix(strings.TrimPrefix(head, "func "), " "), func(t *testing.T) {
				t.Parallel()
				done := false
				overlay := libOverlay(t, func(name, src string) string {
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
					t.Fatalf("the library has no %s%s with %q", head, m.fn, m.old)
				}
				out, err := goTestLib(overlay, "-count=1", "-run", "^TestValue(Conformance|BoxedOperands|BoxedStringCharges|CompareStrings|MemberTable|BuiltinTable|StructTable|HullTable|IterTable|PushTable|CastTable)$").CombinedOutput()
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
