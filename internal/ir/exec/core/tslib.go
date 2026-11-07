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
	return out, nil
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
			ops.Errs: "SyntaxError[]", ops.Kept: "number[]", ops.ExpID: "number",
			ops.Ctx: "CtxCall", ops.Res: "CtxResult | null", ops.Grow: "GrowState",
			ops.Frame: "Frame", ops.Scope: "Env | null", ops.Operators: "OperatorsRuleOp[]", ops.Operator: "OperatorsRuleOp",
			ops.Line: "OperatorsRuleLine", ops.Attempt: "OperatorsRuleAttempt | null",
			ops.Term: "Action", ops.Datum: "Value",
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
			return a[0] + ".length !== 0", true
		}
		return "", false
	},
	Op: func(c *exec.Call, a []string) string {
		switch c.Callee {
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
		case "budget.charge":
			if k, ok := c.Args[0].(*exec.Const); ok && k.Value == uint64(1) {
				return "this.spend()"
			}
			return "this.spend(Number(" + a[0] + "))"
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
			return fmt.Sprintf("this.recovered.length > %s ? (this.control === null ? this.recovered.slice(%s) : "+
				"copyErrorsSpend(this.recovered, %s, this.recovered.length, this.control)) : noErrs", a[0], a[0], a[0])
		case "rec.none":
			return a[0] + ".length === 0"
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
		switch c.Callee {
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
			return fmt.Sprintf("let %s: Value = null;\nlet %s = true;\ntry {\n  %[1]s = %[3]s(this.useCtx(this.frame, this.pos, this.pos, null, null, null));\n"+
				"} catch (x) {\n  if (!(x instanceof EvalError)) {\n    throw x;\n  }\n  %[2]s = false;\n}", dst[0], dst[1], a[0]), true
		case "rec.add":
			return fmt.Sprintf("for (const x of %s) {\n  this.spend();\n  this.recovered.push(x);\n}", a[0]), true
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
