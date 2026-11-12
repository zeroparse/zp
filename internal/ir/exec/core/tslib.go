package core

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/exects"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/pass"
)

// TSMethod is a printed method of the TypeScript runtime's Parser class.
type TSMethod struct {
	// Name is the method's name, Text its source, indented for the class body.
	Name, Text string
}

// tsSpecs are the methods of the TypeScript runtime that the compositions replace. The runtime's
// call observes before callBody, and it has neither Documents nor streams.
var tsSpecs = []struct {
	fn   string
	on   []string
	name string
}{
	{fn: "call", on: []string{ops.Observe, ops.Budget}, name: "callBody"},
	{fn: "grow", name: "grow"},
	{fn: "expect", on: []string{ops.Budget}, name: "expect"},
	{fn: "mergeKept", on: []string{ops.Budget}, name: "mergeKept"},
	{fn: "mergeExpected", on: []string{ops.Budget}, name: "mergeExpected"},
	{fn: "keep", on: []string{ops.Budget}, name: "keep"},
	// The Operators Rule loop and backtracking.
	{fn: "operatorsParse", on: []string{ops.Budget}, name: "operatorsRuleParse"},
	{fn: "operatorsExpr", on: []string{ops.Budget}, name: "operatorsRuleExpr"},
	{fn: "operatorsNud", on: []string{ops.Budget}, name: "operatorsRuleNud"},
	{fn: "longest", on: []string{ops.Budget}, name: "longest"},
	{fn: "skipTrivia", name: "skipTrivia"},
	{fn: "reset", name: "reset"},
	// Predicates and bindings.
	{fn: "predicate", name: "predicate"},
	{fn: "bind", name: "bind"},
	// The operators of terms.
	{fn: "equal", name: "termEqual"},
	{fn: "index", name: "termIndex"},
	{fn: "binary", name: "termBinary"},
	{fn: "unary", name: "termUnary"},
	{fn: "logic", name: "termLogic"},
	{fn: "member", name: "termMember"},
	{fn: "push", name: "termPush"},
	{fn: "pushItems", name: "termPushItems"},
	{fn: "items", name: "termItems"},
	{fn: "itemAt", name: "termItemAt"},
	{fn: "filterKeep", name: "termFilterKeep"},
	{fn: "length", name: "termLength"},
	{fn: "text", name: "termText"},
	{fn: "convert", name: "termConvert"},
	{fn: "cover", name: "termCover"},
	{fn: "made", name: "termMade"},
	{fn: "hull", name: "termHull"},
	{fn: "conforms", name: "fieldConforms"},
	{fn: "valueConforms", name: "valueConforms"},
	{fn: "elemsConform", name: "elemsConform"},
	{fn: "tupleConform", name: "tupleConform"},
}

// TSMethods returns the printed methods of the TypeScript runtime.
func TSMethods() ([]TSMethod, error) {
	m := Build()
	var out []TSMethod
	for _, s := range tsSpecs {
		f, err := pass.Specialize(m, m.Func(s.fn), s.fn, s.on)
		if err != nil {
			return nil, err
		}
		src, err := exects.Method(m, f, tsTarget)
		if err != nil {
			return nil, err
		}
		out = append(out, TSMethod{Name: s.name, Text: src})
	}
	kind, name := tsMemberKind()
	out = append(out, TSMethod{Name: "termError", Text: tsTermError()}, TSMethod{Name: "memberKind", Text: kind},
		TSMethod{Name: "memberName", Text: name})
	return out, nil
}

// tsOpTable returns the TypeScript array of the names of the operators that the failure code indexes.
func tsOpTable(code int) string {
	list := ops.BinaryOps
	switch code {
	case ops.ErrUnary:
		list = ops.UnaryOps
	case ops.ErrLogical:
		list = ops.LogicalOps
	}
	q := make([]string, len(list))
	for i, o := range list {
		q[i] = fmt.Sprintf("%q", o)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// tsMemberKind returns the methods that classify a member name by ops.MemberKinds: memberKind returns
// its kind, and memberName the name of the field or capture that a member of that kind reads.
func tsMemberKind() (kind, name string) {
	var b strings.Builder
	b.WriteString("  // memberKind returns the kind of the member read name (ops.MemberKinds).\n  memberKind(name: string): number {\n    switch (name) {\n")
	for code, k := range ops.MemberKinds {
		if code != ops.MemberCapture && k.Spelling != "" {
			fmt.Fprintf(&b, "      case %q:\n        return %d; // %s\n", k.Spelling, code, k.Name)
		}
	}
	b.WriteString("    }\n")
	k := ops.MemberKinds[ops.MemberCapture]
	fmt.Fprintf(&b, "    if (name.startsWith(%q)) {\n      return %d; // %s\n    }\n", k.Spelling, ops.MemberCapture, k.Name)
	fmt.Fprintf(&b, "    return %d; // %s\n  }", ops.MemberField, ops.MemberKinds[ops.MemberField].Name)
	return b.String(), fmt.Sprintf("  // memberName returns the name of the field or capture that the member read name of the kind reads.\n  memberName(kind: number, name: string): string {\n    return kind === %d ? name.slice(%d) : name;\n  }",
		ops.MemberCapture, len(k.Spelling))
}

// tsTermError returns the method that makes the text of a failure of a term from ops.TermErrors.
func tsTermError() string {
	var b strings.Builder
	b.WriteString("  termError(code: number, a: Value, b: Value, op: number, n: number): string {\n    switch (code) {\n")
	for code, e := range ops.TermErrors {
		var t strings.Builder
		for _, seg := range splitMessage(e.Text) {
			switch seg.hole {
			case "":
				t.WriteString(strings.NewReplacer("\\", "\\\\", "`", "\\`", "$", "\\$").Replace(seg.text))
			case "a", "b":
				t.WriteString("${typeName(" + seg.hole + ")}")
			case "op":
				t.WriteString("${" + tsOpTable(code) + "[op]}")
			case "name":
				t.WriteString("${b as string}")
			case "i":
				t.WriteString("${a}")
			case "n":
				t.WriteString("${n}")
			}
		}
		t.WriteString(e.Hint)
		fmt.Fprintf(&b, "      case %d: // %s\n        return `%s`;\n", code, e.Name, t.String())
	}
	b.WriteString("    }\n    throw new Error(\"no term failure\");\n  }")
	return b.String()
}

// tsTarget is the TypeScript implementation of the operations in the runtime's Parser class.
var tsTarget = &exects.Target{
	Source: "internal/ir/exec/core",
	Packed: ops.Value,
	Parts: map[string][]exects.Part{
		// The set of an isolation is the range of the stack of the record that it ended.
		ops.Set: {{Suffix: "From", Type: "number"}, {Suffix: "To", Type: "number"}},
		// A backtracking point is the position, the environment, the length of the trail and the
		// number of recovered errors.
		ops.Backtrack: {{Suffix: "Pos", Type: "number"}, {Suffix: "Env", Type: "Env | null"}, {Suffix: "Trail", Type: "number"},
			{Suffix: "Rec", Type: "number"}},
	},
	Handle: func(name string) string {
		return map[string]string{
			ops.Rule: "Rule", ops.Value: "R", ops.Entry: "MemoEntry | null", ops.Values: "Value[] | null",
			ops.Errs: "Errs", ops.Kept: "number[]", ops.ExpID: "number",
			ops.Ctx: "CtxCall", ops.Res: "CtxResult | null", ops.Grow: "GrowState",
			ops.Frame: "Frame", ops.Scope: "Env | null", ops.Operators: "OperatorsRuleOp[]", ops.Operator: "OperatorsRuleOp",
			ops.Line: "OperatorsRuleLine", ops.Attempt: "OperatorsRuleAttempt | null",
			ops.Term: "Action", ops.Datum: "Value", ops.Seq: "readonly (Node | null)[]",
			ops.Obj: "Node", ops.Name: "string", ops.Str: "string", ops.Act: "Actx", ops.Fail: "Value", ops.Elem: "Node | null",
			ops.NodeType: "string", ops.FType: "FieldType",
		}[name]
	},
	FuncName: func(name string) string {
		for _, s := range tsSpecs {
			if s.fn == name {
				return s.name
			}
		}
		return ""
	},
	Reg: func(name string) string {
		return map[string]string{ops.Pos: "this.pos", ops.Silent: "this.silent", ops.Cut: "this.cut"}[name]
	},
	Negate: func(c *exec.Call, a []string) (string, bool) {
		switch c.Callee {
		case "opr.found":
			return a[0] + " === null", true
		case "opr.hasSkip":
			return a[0] + ".operatorsRule!.skip === null", true
		case "opr.infix":
			return a[0] + `.kind !== "infix"`, true
		case "rec.none":
			return a[0] + " !== null", true
		}
		return "", false
	},
	Op: func(c *exec.Call, a []string) string {
		switch c.Callee {
		case "val.kind":
			return "valKind(" + a[0] + ")"
		case "val.unbox":
			return "unboxValue(" + a[0] + ")"
		case "val.fromBool":
			return a[0]
		case "val.toBool":
			return "(" + a[0] + " as boolean)"
		case "val.fail":
			return fmt.Sprintf("this.termError(%s, %s, %s, %s, %s)", a[0], a[1], a[2], a[3], a[4])
		case "int.add", "int.sub", "int.mul", "int.quo", "int.rem", "int.cmp", "int.index":
			return "int" + strings.ToUpper(c.Callee[4:5]) + c.Callee[5:] + "(" + strings.Join(a, ", ") + ")"
		case "int.neg":
			return "intNeg(" + a[0] + ")"
		case "int.zero":
			return "(" + a[0] + " === 0)"
		case "str.add", "str.cmp", "str.eq":
			return "this.str" + strings.ToUpper(c.Callee[4:5]) + c.Callee[5:] + "(" + strings.Join(a, ", ") + ")"
		case "seq.items":
			return "seqItems(" + a[0] + ")"
		case "seq.len":
			return a[0] + ".length"
		case "seq.at":
			return "unboxValue(" + a[0] + "[" + a[1] + "]!)"
		case "val.name", "val.fromInt":
			return a[0]
		case "node.elem":
			return a[0]
		case "val.box":
			return a[0] + ".box(" + a[1] + ")"
		case "node.hasText":
			return a[0] + ".terminal"
		case "node.width":
			return "(" + a[0] + ".end - " + a[0] + ".start)"
		case "node.count":
			return a[0] + ".children.length"
		case "node.rangeText":
			return "this.text(" + a[0] + ".start, " + a[0] + ".end)"
		case "str.len":
			return "this.unitLen(" + a[0] + " as string)"
		case "str.of":
			return "(" + a[0] + " as string)"
		case "str.empty":
			return `""`
		case "node.isNode":
			return "isValueNode(" + a[0] + ")"
		case "node.start", "node.end":
			return a[0] + "." + c.Callee[5:]
		case "node.value":
			return "this.nodeValue(" + a[0] + ")"
		case "node.isStruct":
			return "(this.g.structFields.get(" + a[0] + ".type) !== undefined)"
		case "node.setSpan":
			return fmt.Sprintf("setNodeSpan(%s, %s, %s)", a[0], a[1], a[2])
		case "act.made":
			return "this.created.push(" + a[1] + ")"
		case "act.push":
			return "this.kidStack.push(" + a[1] + ")"
		case "act.pushAll":
			return a[0] + ".pushAll(" + a[1] + ")"
		case "ftype.kind":
			return a[0] + ".kind.charCodeAt(0)"
		case "ftype.elem":
			return a[0] + ".elem!"
		case "ftype.count":
			return a[0] + ".alts.length"
		case "ftype.alt":
			return a[0] + ".alts[" + a[1] + "]!"
		case "node.shape":
			return "nodeShape(" + a[0] + ")"
		case "node.isType":
			return "(" + a[0] + ".type === " + a[1] + ".name)"
		case "node.isMissing":
			return "false"
		case "node.children":
			return a[0] + ".children"
		case "seq.step":
			return "this.spend()"
		case "val.seq":
			return fmt.Sprintf("this.counted(new Node(%s ? typeTupleValue : typeListValue, \"\", 0, 0, \"\", %s, noFields, false, false), nodeBytes)", a[1], a[2])
		case "node.typeName":
			return a[0]
		case "node.hasBase":
			return "(nodeShape(" + a[0] + ") <= shapeList)"
		case "node.convert":
			return fmt.Sprintf("this.retyped(%s, %s)", a[0], a[1])
		case "depth.exit":
			return "this.depth--"
		case "frame.markPos", "frame.markEnv", "frame.markTrail", "frame.markRec":
			return a[0] + strings.TrimPrefix(c.Callee, "frame.mark")
		case "frame.current":
			return "this.frame"
		case "frame.use":
			return "this.frame = " + a[0]
		case "frame.set":
			return fmt.Sprintf("this.setCapture(%s, %s)", a[0], a[1])
		case "frame.close":
			return ""
		case "frame.open":
			return "this.newFrame(" + a[0] + ")"
		case "frame.slots":
			return a[0] + ".length"
		case "env.restore":
			return "this.env = " + a[0]
		case "env.bind":
			return fmt.Sprintf("this.bindValue(%s, %s)", a[0], a[1])
		case "term.created":
			return "this.created.length"
		case "term.dropCreated":
			return "this.dropSpend(this.created, " + a[0] + ")"
		case "term.kids":
			return "this.kidStack.length"
		case "term.dropKids":
			return "this.dropSpend(this.kidStack, " + a[0] + ")"
		case "term.isFalse":
			return a[0] + " === false"
		case "term.isFunction":
			return `typeof ` + a[0] + ` === "function"`
		case "opr.led", "opr.prefix":
			return a[0] + ".operatorsRule!." + strings.TrimPrefix(c.Callee, "opr.")
		case "opr.count":
			return a[0] + ".length"
		case "opr.at":
			return a[0] + "[" + a[1] + "]!"
		case "opr.operands":
			return a[0] + ".operatorsRule!.operands.length"
		case "opr.operand":
			return a[0] + ".operatorsRule!.operands[" + a[1] + "]!"
		case "opr.line", "opr.level":
			return a[0] + "." + strings.TrimPrefix(c.Callee, "opr.")
		case "opr.infix", "opr.postfix":
			return fmt.Sprintf("%s.kind === %q", a[0], strings.TrimPrefix(c.Callee, "opr."))
		case "opr.right":
			return a[0] + `.assoc === "right"`
		case "opr.nonAssoc":
			return a[0] + `.assoc === "none"`
		case "opr.scope":
			return a[0] + ".scope.length"
		case "opr.hasSkip":
			return a[0] + ".operatorsRule!.skip !== null"
		case "opr.skip":
			return a[0] + ".operatorsRule!.skip!(this) !== undefined"
		case "opr.match":
			return a[0] + ".m(this)"
		case "opr.lineValue":
			return fmt.Sprintf("this.lineResult(%s, %s, %s, %s as Node | null, %s)", a[0], a[1], a[2], a[3], a[4])
		case "opr.build":
			return fmt.Sprintf("this.operatorsRuleBuild(%s, %s!, %s as Node | null, %s as Node | null)", a[0], a[1], a[2], a[3])
		case "opr.attempt":
			return fmt.Sprintf("{ op: %s, frame: %s, v: %s as Node | null, start: %s, end: this.pos, env: this.env, cut: %s, errs: %s }",
				a[0], a[1], a[2], a[3], a[4], a[5])
		case "opr.found":
			return a[0] + " !== null"
		case "opr.op", "opr.end", "opr.cut", "opr.env", "opr.errs":
			return a[0] + "!." + strings.TrimPrefix(c.Callee, "opr.")
		case "opr.setCutFailed":
			return "this.cutFailed = " + a[0]
		case "opr.cutFailed":
			return "this.cutFailed"
		case "opr.setNudBound":
			return "this.nudBound = " + a[0]
		case "opr.nudBound":
			return "this.nudBound"
		case "opr.setAfterEmpty":
			return "this.afterEmpty = " + a[0]
		case "opr.afterEmpty":
			return "this.afterEmpty"
		case "budget.charge":
			if k, ok := c.Args[0].(*exec.Const); ok && k.Value == uint64(1) {
				return "this.spend()"
			}
			return "this.spendUnits(Number(" + a[0] + "))"
		case "call.first":
			return "this.firstCall(" + a[0] + ")"
		case "rule.memo", "rule.leader", "rule.id":
			return a[0] + "." + strings.TrimPrefix(c.Callee, "rule.")
		case "rule.docOnlyMemo":
			return "false" // TypeScript has no Documents: no rule is memoized by a Document alone
		case "rule.plain":
			return a[0] + ".scope.length === 0"
		case "rule.invoke":
			return fmt.Sprintf("this.invoke(%s, %s)", a[0], a[1])
		case "rule.invokePlain":
			return fmt.Sprintf("this.invokePlainBody(%s, %s)", a[0], a[1])
		case "env.key":
			return "this.envValues(" + a[0] + ".vars)"
		case "rec.mark":
			return "this.recovered.length"
		case "rec.truncate":
			return "this.recovered.length = " + a[0]
		case "rec.since":
			return "this.recSince(" + a[0] + ")"
		case "rec.none":
			return a[0] + " === null"
		case "exp.farthest":
			return "this.farthest"
		case "exp.recording":
			return "this.expectations"
		case "exp.reset":
			return fmt.Sprintf("this.farthest = %s;\nthis.expTop = this.expBase", a[0])
		case "exp.fresh":
			return "this.expTop === this.expBase"
		case "exp.placeholder":
			return "this.exp[this.expTop++] = expFar"
		case "exp.maybe":
			return "true" // the record has no filter of its entries
		case "exp.count":
			return "this.expTop - this.expBase"
		case "exp.at":
			return "this.exp[this.expBase + " + a[0] + "]!"
		case "exp.add":
			return "this.exp[this.expTop++] = " + a[0]
		case "exp.keptCount":
			return a[0] + ".length"
		case "exp.keptAt":
			return a[0] + "[" + a[1] + "]!"
		case "exp.setCount":
			return fmt.Sprintf("%sTo - %sFrom", a[0], a[0])
		case "exp.setAt":
			return fmt.Sprintf("this.exp[%sFrom + %s]!", a[0], a[1])
		case "exp.empty":
			return "noExp"
		case "exp.last":
			return "this.lastKept"
		case "memo.far", "memo.expected", "memo.ok", "memo.end", "memo.silent", "memo.errs":
			return a[0] + "." + strings.TrimPrefix(c.Callee, "memo.")
		case "memo.value":
			return a[0] + ".node"
		case "memo.setExpected":
			return fmt.Sprintf("%s.far = %s;\n%s.expected = %s", a[0], a[1], a[0], a[2])
		case "obs.memoHit":
			return "this.observation?.memoHit()"
		case "ctx.found":
			return a[0] + " !== null"
		case "ctx.ok":
			return a[0] + "!.ok"
		case "ctx.value":
			return a[0] + "!.node"
		case "ctx.replay":
			return fmt.Sprintf("this.replayCtx(%s!, %s)", a[0], a[1])
		case "ctx.allows":
			return fmt.Sprintf("memoAllows(%s, %s.present)", a[0], a[1])
		case "ctx.active":
			return a[0] + ".ctx"
		case "ctx.lookup":
			return fmt.Sprintf("this.ctxLookup(%s, %s.id, %s, %s)", a[0], a[1], a[2], a[3])
		case "ctx.push", "ctx.pop", "ctx.done":
			return fmt.Sprintf("this.%s(%s)", map[string]string{"ctx.push": "ctxPush", "ctx.pop": "ctxPop", "ctx.done": "ctxDone"}[c.Callee], a[0])
		case "ctx.storeMemo":
			return "storeMemo"
		case "ctx.storeCtx":
			return "storeCtx"
		case "memo.setPresent":
			return fmt.Sprintf("%s.present = %s.present", a[0], a[1])
		case "ctx.keep":
			return fmt.Sprintf("this.ctxKeep(%s, %s.id, %s, %s, %s, newCtxResult(%s, %s, %s, %s, %s, %s))",
				a[0], a[1], a[2], a[3], a[4], a[8], a[6], a[7], a[9], a[10], a[11])
		case "grow.begin":
			return fmt.Sprintf("this.growBegin(%s, %s)", a[0], a[1])
		case "grow.next":
			return "this.growNext(" + a[0] + ")"
		case "grow.step":
			return fmt.Sprintf("this.growStep(%s, %s)", a[0], a[1])
		case "grow.end":
			return "this.growEnd(" + a[0] + ")"
		}
		panic("core: no TypeScript implementation of " + c.Callee)
	},
	Stmt: func(c *exec.Call, a []string, dst []string, decl string) (string, bool) {
		lhs := func(name string) string {
			if decl != "" {
				return decl + " " + name
			}
			return name
		}
		key := func() string { return strings.Join(a[:4], ", ") }
		entry := func(e, node, ok, end, silent, errs string) string {
			return fmt.Sprintf("%s = newEntry(%s, %s, %s, %s, %s);\nthis.memoPut(%s, %s);", lhs(e), node, ok, end, silent, errs, key(), e)
		}
		// absent prints an operation whose result is undefined where it has none as its two results.
		absent := func(call string) string {
			if dst[1] == "_" {
				return fmt.Sprintf("%s = %s ?? null;", lhs(dst[0]), call)
			}
			h := dst[0] + "Held"
			return fmt.Sprintf("const %[1]s = %[2]s;\n%[3]s = %[1]s !== undefined;\n%[4]s = %[1]s ?? null;", h, call, lhs(dst[1]), lhs(dst[0]))
		}
		switch c.Callee {
		case "node.of":
			var lines []string
			if dst[0] != "_" {
				lines = append(lines, fmt.Sprintf("%s = %s as Node;", lhs(dst[0]), a[0]))
			}
			if dst[1] != "_" {
				lines = append(lines, fmt.Sprintf("%s = %s instanceof Node;", lhs(dst[1]), a[0]))
			}
			return strings.Join(lines, "\n"), true
		case "node.heldText":
			var lines []string
			if dst[0] != "_" {
				lines = append(lines, fmt.Sprintf("%s = %s.text;", lhs(dst[0]), a[0]))
			}
			if dst[1] != "_" {
				lines = append(lines, fmt.Sprintf("%s = %s.terminal;", lhs(dst[1]), a[0]))
			}
			return strings.Join(lines, "\n"), true
		case "seq.span":
			var lines []string
			if dst[0] != "_" {
				lines = append(lines, fmt.Sprintf("%s = %s[%s]?.start ?? 0;", lhs(dst[0]), a[1], a[2]))
			}
			if dst[1] != "_" {
				lines = append(lines, fmt.Sprintf("%s = %s[%s]?.end ?? 0;", lhs(dst[1]), a[1], a[2]))
			}
			if dst[2] != "_" {
				lines = append(lines, fmt.Sprintf("%s = %s[%s] !== null;", lhs(dst[2]), a[1], a[2]))
			}
			return strings.Join(lines, "\n"), true
		case "node.span", "act.span":
			var lines []string
			if dst[0] != "_" {
				lines = append(lines, fmt.Sprintf("%s = %s.start;", lhs(dst[0]), a[0]))
			}
			if dst[1] != "_" {
				lines = append(lines, fmt.Sprintf("%s = %s.end;", lhs(dst[1]), a[0]))
			}
			return strings.Join(lines, "\n"), true
		case "node.field":
			return absent(fmt.Sprintf("findFieldSpend(%s, %s, this.control)", a[0], a[1])), true
		case "node.zero":
			return absent(fmt.Sprintf("%s.memberZero(%s, %s)", a[0], a[1], a[2])), true
		case "depth.enter":
			return "if (++this.depth > this.maxDepth) {\n  this.tooDeep();\n}", true
		case "frame.mark":
			m := dst[0]
			return fmt.Sprintf("%s %sPos = this.pos;\n%[1]s %[2]sEnv = this.env;\n%[1]s %[2]sTrail = this.trail.length;\n%[1]s %[2]sRec = this.recovered.length;", decl, m), true
		case "frame.undo":
			return fmt.Sprintf("for (let t = this.trail.length - 1; t >= %s; t--) {\n  this.spend();\n  const u = this.trail[t]!;\n  u.f[u.slot] = u.old;\n}\nthis.trail.length = %[1]s;", a[0]), true
		case "frame.save":
			return fmt.Sprintf("this.saved.length = 0;\nfor (const x of %s) {\n  this.spend();\n  this.saved.push(x);\n}", a[0]), true
		case "frame.restore":
			return fmt.Sprintf("for (let t = 0; t < %s.length; t++) {\n  this.spend();\n  %[1]s[t] = this.saved[t]!;\n}", a[0]), true
		case "memo.lookup":
			s := fmt.Sprintf("%s = this.memoGet(%s);", lhs(dst[0]), key())
			if dst[1] != "_" {
				s += fmt.Sprintf("\n%s = %s !== null;", lhs(dst[1]), dst[0])
			}
			return s, true
		case "memo.store":
			return entry(dst[0], a[7], a[4], a[5], a[6], a[8]), true
		case "ctx.enter":
			// The context of a call of a rule of no cycle, or outside every growth, is empty.
			c, seed := dst[0], dst[1]
			return fmt.Sprintf("let %s = noCtx;\nlet %s: CtxResult | null = null;\nif (%s.cycle !== 0 && this.gamma.length !== 0) {\n  %s = newCtxCall();\n  %s = this.ctxEnter(%s, %s, %s);\n}",
				c, seed, a[0], c, seed, c, a[0], a[1]), true
		case "term.eval":
			return fmt.Sprintf("const %[1]sR = %[3]s(this.useCtx(this.frame, this.pos, this.pos, null, null, null));\nlet %[1]s: Value = %[1]sR[0];\nlet %[2]s = %[1]sR[1] === null;", dst[0], dst[1], a[0]), true
		case "rec.add":
			return "this.recAdd(" + a[0] + ");", true
		case "exp.copy":
			return fmt.Sprintf("this.lastKept = this.control === null ? this.exp.slice(%[1]sFrom, %[1]sTo) : copyIDsSpend(this.exp, %[1]sFrom, %[1]sTo, this.control);\n%[2]s = this.lastKept;", a[0], lhs(dst[0])), true
		case "exp.isolate":
			m := dst[0]
			return fmt.Sprintf("const %sFar = this.farthest;\nconst %sBase = this.expBase;\nthis.expBase = this.expTop;\nthis.farthest = %s;", m, m, a[0]), true
		case "exp.unisolate":
			far, set, m := dst[0], dst[1], a[0]
			return fmt.Sprintf("const %s = this.farthest;\nconst %sFrom = this.expBase;\nconst %sTo = this.expTop;\n"+
				"this.expTop = this.expBase;\nthis.expBase = %sBase;\nthis.farthest = %sFar;", far, set, set, m, m), true
		}
		return "", false
	},
}

// TSTermMethod returns the name of the method of the TypeScript runtime's Parser class that the
// composition fn is printed as, for printing calls of the compositions outside the runtime (lowered
// terms).
func TSTermMethod(fn string) string { return tsTarget.FuncName(fn) }

// TSHandle returns the TypeScript type of the handle type name in the TypeScript runtime.
func TSHandle(name string) string { return tsTarget.Handle(name) }
