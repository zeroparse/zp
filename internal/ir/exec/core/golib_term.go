package core

import (
	"fmt"
	"go/format"
	"strings"

	"zeroparse.dev/zp/internal/ir/exec/execgo"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/pass"
)

// GoTermsFile is the file of the printed term compositions in the generated Go runtime, relative to
// the directory of this package.
const GoTermsFile = "../../../../runtime/term_exec.go"

// termFuncs are the compositions of the operators of terms, in the order they are printed.
var termFuncs = []struct{ fn, name string }{
	{"equal", "termEqual"}, {"index", "termIndex"}, {"binary", "termBinary"}, {"unary", "termUnary"}, {"logic", "termLogic"},
	{"member", "termMember"},
}

// goTermSpecs are the compositions of the operators of terms in the node and the typed runtime.
var goTermSpecs = func() []goSpec {
	var specs []goSpec
	for _, typed := range []bool{false, true} {
		for _, f := range termFuncs {
			specs = append(specs, goSpec{fn: f.fn, typed: typed, name: f.name})
		}
	}
	return specs
}()

// goTermNames is the name of each composition of the operators of terms in the Go runtime.
func goTermNames(names map[string]string) {
	for _, f := range termFuncs {
		names[f.fn] = f.name
	}
}

// errOps returns the Go table of the names of the operators that the failure code indexes.
func errOps(code int) string {
	switch code {
	case ops.ErrUnary:
		return "termUnaryOps"
	case ops.ErrLogical:
		return "termLogicalOps"
	}
	return "termBinaryOps"
}

// goTermConsts returns the constants and the tables of the operators of terms for the Go runtime.
func goTermConsts() string {
	var b strings.Builder
	b.WriteString("\n// The codes of the binary operators, as the compositions of terms take them.\nconst (\n")
	for i := range ops.BinaryNames {
		fmt.Fprintf(&b, "\t%s = %d // %s\n", ops.Ident(ops.BinaryNames[i]), i, ops.BinaryOps[i])
	}
	b.WriteString(")\n\n// The codes of the unary operators.\nconst (\n")
	for i := range ops.UnaryNames {
		fmt.Fprintf(&b, "\t%s = %d // %s\n", ops.Ident(ops.UnaryNames[i]), i, ops.UnaryOps[i])
	}
	b.WriteString(")\n\n// The codes of the logical operators.\nconst (\n")
	for i := range ops.LogicalNames {
		fmt.Fprintf(&b, "\t%s = %d // %s\n", ops.Ident(ops.LogicalNames[i]), i, ops.LogicalOps[i])
	}
	b.WriteString(")\n\n")
	for _, t := range []struct {
		name string
		list []string
	}{{"termBinaryOps", ops.BinaryOps}, {"termUnaryOps", ops.UnaryOps}, {"termLogicalOps", ops.LogicalOps}} {
		fmt.Fprintf(&b, "var %s = [...]string{", t.name)
		for i, s := range t.list {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q", s)
		}
		b.WriteString("}\n")
	}
	b.WriteString("\n// The codes of the kinds of values.\nconst (\n")
	for _, k := range []struct {
		name string
		code int
	}{{"Nil", ops.KindNil}, {"Int", ops.KindInt}, {"String", ops.KindString}, {"Bool", ops.KindBool}, {"Other", ops.KindOther}} {
		fmt.Fprintf(&b, "\tvalKind%s = %d\n", k.name, k.code)
	}
	b.WriteString(")\n\n// The codes of the kinds of member reads.\nconst (\n")
	for code, k := range ops.MemberKinds {
		fmt.Fprintf(&b, "\tmember%s = %d\n", k.Name, code)
	}
	b.WriteString(")\n\n// memberKind returns the kind of the member read name and the name of the field or capture that it\n// reads.\nfunc memberKind(name string) (int, string) {\n\tswitch name {\n")
	var prefix []int
	for code, k := range ops.MemberKinds {
		switch {
		case code == ops.MemberCapture:
			prefix = append(prefix, code)
		case k.Spelling != "":
			fmt.Fprintf(&b, "\tcase %q:\n\t\treturn member%s, name\n", k.Spelling, k.Name)
		}
	}
	b.WriteString("\t}\n")
	for _, code := range prefix {
		k := ops.MemberKinds[code]
		fmt.Fprintf(&b, "\tif field, ok := strings.CutPrefix(name, %q); ok {\n\t\treturn member%s, field\n\t}\n", k.Spelling, k.Name)
	}
	fmt.Fprintf(&b, "\treturn member%s, name\n}\n", ops.MemberKinds[ops.MemberField].Name)
	return b.String()
}

// goTermError returns the function that makes the failure of a term for a runtime whose parser is
// recv, with the types named by typeName. The text is that of ops.TermErrors. A failure of a term
// that bounds its work is put together piece by piece by evalErrorValue, as it was before: the
// message of == or != applied to operands that it does not accept is one piece, and every other
// message, including that of the other operators, is cut at its placeholders.
func goTermError(recv, typeName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n// termError returns the failure of a term with the message ops.TermErrors[code]; it is a value that the\n// caller raises.\nfunc (p %s) termError(code int, a, b any, op, n int) any {\n\tswitch code {\n", recv)
	for code, e := range ops.TermErrors {
		fmt.Fprintf(&b, "\tcase %d: // %s\n", code, e.Name)
		var format strings.Builder
		var args []string
		for _, seg := range splitMessage(e.Text) {
			switch seg.hole {
			case "":
				format.WriteString(strings.ReplaceAll(seg.text, "%", "%%"))
			case "a", "b":
				args = append(args, typeName+"("+seg.hole+")")
				format.WriteString("%s")
			case "op":
				args = append(args, errOps(code)+"[op]")
				format.WriteString("%s")
			case "name":
				args = append(args, "b.(string)")
				format.WriteString("%s")
			case "i":
				args = append(args, "a.(int)")
				format.WriteString("%d")
			case "n":
				args = append(args, "n")
				format.WriteString("%d")
			}
		}
		if code == ops.ErrOperands || code == ops.ErrCompare {
			hint := ""
			if e.Hint != "" {
				hint = fmt.Sprintf("+%q", e.Hint)
			}
			if code == ops.ErrOperands {
				// Only == and != put the message together first; the other operators charge it piece by
				// piece.
				fmt.Fprintf(&b, "\t\tif op != %d && op != %d {\n\t\t\treturn p.evalErrorValue(%q, %s)\n\t\t}\n",
					ops.OpEq, ops.OpNe, format.String(), strings.Join(args, ", "))
			}
			fmt.Fprintf(&b, "\t\treturn p.evalErrorValue(\"%%s\", fmt.Sprintf(%q, %s)%s)\n", format.String(), strings.Join(args, ", "), hint)
			continue
		}
		if len(args) == 0 {
			fmt.Fprintf(&b, "\t\treturn p.evalErrorValue(%q)\n", format.String())
			continue
		}
		fmt.Fprintf(&b, "\t\treturn p.evalErrorValue(%q, %s)\n", format.String(), strings.Join(args, ", "))
	}
	b.WriteString("\t}\n\tpanic(\"runtime: no term failure\")\n}\n")
	return b.String()
}

// segment is a piece of a message: fixed text, or a hole.
type segment struct{ text, hole string }

// splitMessage cuts a message of ops.TermErrors at its placeholders.
func splitMessage(text string) []segment {
	var out []segment
	for text != "" {
		i := strings.IndexByte(text, '{')
		if i < 0 {
			out = append(out, segment{text: text})
			break
		}
		if i > 0 {
			out = append(out, segment{text: text[:i]})
		}
		j := strings.IndexByte(text, '}')
		out = append(out, segment{hole: text[i+1 : j]})
		text = text[j+1:]
	}
	return out
}

// GoTerms returns the Go file of the printed term compositions for package runtime.
func GoTerms() (string, error) {
	m := Build()
	var b strings.Builder
	b.WriteString("// Code generated by internal/ir/exec/core; DO NOT EDIT.\n\npackage runtime\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n")
	b.WriteString(goTermConsts())
	b.WriteString(goTermError("*Parser", "typeName"))
	b.WriteString(goTermError("*TParser", "ttypeName"))
	for _, s := range goTermSpecs {
		f, err := pass.Specialize(m, m.Func(s.fn), s.fn, s.on)
		if err != nil {
			return "", err
		}
		src, err := execgo.Func(m, f, goTarget(s))
		if err != nil {
			return "", err
		}
		b.WriteString("\n" + src)
	}
	out, err := format.Source([]byte(b.String()))
	return string(out), err
}

// goValueOp returns the Go form of the value primitive named callee in the node or the typed
// runtime, given its printed arguments, or reports false for another operation.
func goValueOp(callee string, a []string, typed bool) (string, bool) {
	c := callee
	t := ""
	if typed {
		t = "t"
	}
	switch callee {
	case "val.kind":
		return "valKind(" + a[0] + ")", true
	case "val.unbox":
		if typed {
			return a[0], true
		}
		return "UnboxValue(" + a[0] + ")", true
	case "val.fromBool":
		return "any(" + a[0] + ")", true
	case "val.toBool":
		return a[0] + ".(bool)", true
	case "val.fail":
		return fmt.Sprintf("p.termError(%s, %s, %s, %s, %s)", a[0], a[1], a[2], a[3], a[4]), true
	case "int.add":
		return fmt.Sprintf("any(%s.(int) + %s.(int))", a[0], a[1]), true
	case "int.sub":
		return fmt.Sprintf("any(%s.(int) - %s.(int))", a[0], a[1]), true
	case "int.mul":
		return fmt.Sprintf("any(%s.(int) * %s.(int))", a[0], a[1]), true
	case "int.quo":
		return fmt.Sprintf("any(%s.(int) / %s.(int))", a[0], a[1]), true
	case "int.rem":
		return fmt.Sprintf("any(%s.(int) %% %s.(int))", a[0], a[1]), true
	case "int.neg":
		return fmt.Sprintf("any(-%s.(int))", a[0]), true
	case "int.zero":
		return a[0] + ".(int) == 0", true
	case "int.cmp":
		return fmt.Sprintf("intCmp(%s, %s)", a[0], a[1]), true
	case "int.index":
		return fmt.Sprintf("intIndex(%s, %s)", a[0], a[1]), true
	case "str.add":
		return fmt.Sprintf("p.strAdd(%s, %s)", a[0], a[1]), true
	case "str.cmp":
		return fmt.Sprintf("p.strCmp(%s, %s)", a[0], a[1]), true
	case "str.eq":
		return fmt.Sprintf("p.strEq(%s, %s)", a[0], a[1]), true
	case "seq.items":
		return t + "valItems(" + a[0] + ")", true
	case "seq.len":
		return "len(" + a[0] + ")", true
	case "val.name":
		return "any(" + a[0] + ")", true
	case "node.of":
		return t + "nodeOf(" + a[0] + ")", true
	case "node.isNode":
		if typed {
			return "tisNode(" + a[0] + ")", true
		}
		return a[0] + ".isNode()", true
	case "node.start", "node.end":
		if typed {
			return "tnode" + strings.ToUpper(c[5:6]) + c[6:] + "(" + a[0] + ")", true
		}
		return "any(int(" + a[0] + ".Raw" + strings.ToUpper(c[5:6]) + c[6:] + "))", true
	case "node.value":
		return "p.nodeValue(" + a[0] + ")", true
	case "node.isStruct":
		if typed {
			return a[0] + ".TStruct()", true
		}
		return "p.g.StructFields[" + a[0] + ".Type()] != nil", true
	case "node.field":
		if typed {
			return fmt.Sprintf("p.valueField(%s, %s)", a[0], a[1]), true
		}
		return fmt.Sprintf("p.valueField(%s.RawFields, %s)", a[0], a[1]), true
	case "node.zero":
		return fmt.Sprintf("p.memberZero(%s, %s, %s)", a[0], a[1], a[2]), true
	case "seq.at":
		if typed {
			return fmt.Sprintf("%s[%s]", a[0], a[1]), true
		}
		return fmt.Sprintf("UnboxNode(%s[%s])", a[0], a[1]), true
	}
	return "", false
}

// EngineTermsFile is the file of the printed term operator compositions of the engine.
const EngineTermsFile = "../../../engine/core/term_exec.go"

// EngineTerms returns the Go file of the printed term operator compositions for package core of the
// engine.
func EngineTerms() (string, error) {
	m := Build()
	var b strings.Builder
	b.WriteString("// Code generated by internal/ir/exec/core; DO NOT EDIT.\n\npackage core\n\nimport (\n\t\"errors\"\n\t\"strconv\"\n\t\"strings\"\n\n\t\"zeroparse.dev/zp/internal/value\"\n)\n")
	b.WriteString(goTermConsts())
	b.WriteString(engineTermError())
	for _, f := range termFuncs {
		fn, err := pass.Specialize(m, m.Func(f.fn), f.fn, nil)
		if err != nil {
			return "", err
		}
		src, err := execgo.Func(m, fn, engineTarget())
		if err != nil {
			return "", err
		}
		b.WriteString("\n" + src)
	}
	out, err := format.Source([]byte(b.String()))
	return string(out), err
}

// engineTermError returns the function that makes the failure of a term in the engine: an error whose
// text is that of ops.TermErrors, put together from the same fragments as the other runtimes charge.
func engineTermError() string {
	var b strings.Builder
	b.WriteString("\n// termError returns the failure of a term with the message ops.TermErrors[code]; it is an error that the\n// caller returns.\nfunc (p *Parser) termError(code int, a, b any, op, n int) error {\n\tswitch code {\n")
	for code, e := range ops.TermErrors {
		fmt.Fprintf(&b, "\tcase %d: // %s\n", code, e.Name)
		var parts []string
		for _, seg := range splitMessage(e.Text) {
			switch seg.hole {
			case "":
				parts = append(parts, fmt.Sprintf("%q", seg.text))
			case "a", "b":
				parts = append(parts, "TypeName("+seg.hole+")")
			case "op":
				parts = append(parts, errOps(code)+"[op]")
			case "name":
				parts = append(parts, "b.(string)")
			case "i":
				parts = append(parts, "strconv.Itoa(a.(int))")
			case "n":
				parts = append(parts, "strconv.Itoa(n)")
			}
		}
		if code == ops.ErrOperands || code == ops.ErrCompare {
			// The hint is a fragment of its own even where it is empty: the controlled engine
			// charges one unit for each fragment of an error.
			parts = append(parts, fmt.Sprintf("%q", e.Hint))
		}
		fmt.Fprintf(&b, "\t\treturn p.ActionError(%s)\n", strings.Join(parts, ", "))
	}
	b.WriteString("\t}\n\treturn errors.New(\"core: no term failure\")\n}\n")
	return b.String()
}

// engineValueOp returns the Go form of the value primitive named callee in the engine, given its
// printed arguments, or reports false for another operation.
func engineValueOp(callee string, a []string) (string, bool) {
	switch callee {
	case "val.kind":
		return "valKind(" + a[0] + ")", true
	case "val.unbox":
		return "unboxed(" + a[0] + ")", true
	case "val.fromBool":
		return "any(" + a[0] + ")", true
	case "val.toBool":
		return a[0] + ".(bool)", true
	case "val.fail":
		return fmt.Sprintf("p.termError(%s, %s, %s, %s, %s)", a[0], a[1], a[2], a[3], a[4]), true
	case "seq.items":
		return "valItems(" + a[0] + ")", true
	case "seq.at":
		return fmt.Sprintf("value.UnboxNode(%s[%s])", a[0], a[1]), true
	case "node.isNode":
		return "value.IsNode(" + a[0] + ")", true
	case "node.start":
		return "any(int(value.D(" + a[0] + ").Start))", true
	case "node.end":
		return "any(int(value.D(" + a[0] + ").End))", true
	case "node.isStruct":
		return "p.isStruct(" + a[0] + ")", true
	case "node.field":
		return fmt.Sprintf("p.ValueField(value.D(%s).Fields, %s)", a[0], a[1]), true
	}
	// The remaining primitives read the same in every Go runtime.
	for _, typed := range []bool{false} {
		if s, ok := goValueOp(callee, a, typed); ok {
			return s, true
		}
	}
	return "", false
}
