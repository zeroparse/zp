package core

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/execgo"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/pass"
)

//go:generate go run print_runtime.go

// GoRuntimeFile is the file of the printed compositions in the generated Go runtime, relative to
// the directory of this package. go generate ./gen copies it into the runtime that generated
// parsers embed.
const GoRuntimeFile = "../../../../runtime/call_exec.go"

// GoStreamFile is the file of the printed stream compositions in the generated Go runtime, relative
// to the directory of this package.
const GoStreamFile = "../../../../runtime/stream_exec.go"

// GoOperatorsFile is the file of the printed Operators Rule compositions in the generated Go runtime,
// relative to the directory of this package.
const GoOperatorsFile = "../../../../runtime/operators_exec.go"

// goSpec is a printed function of the Go runtime: a specialization of a composition for the node
// Parser or the typed TParser.
type goSpec struct {
	fn    string
	typed bool
	on    []string
	name  string // the Go name
	body  string // the method that call.body calls
	plain bool   // printed as a function of its package, without the parser as receiver
}

// goSpecs are the functions of the Go runtime that the compositions replace.
var goSpecs = []goSpec{
	// Parser.Call takes the tracked path for observation and Documents, so it neither observes nor
	// reads a Document itself.
	{fn: "call", on: []string{ops.Document, ops.Budget}, name: "Call"},
	{fn: "grow", name: "grow"},
	// The expectation record: the Document form, the form of a parse without a Document, and the
	// form of an ordinary parse with its slow path.
	{fn: "expect", on: []string{ops.Document, ops.Budget}, name: "Expect"},
	{fn: "expect", on: []string{ops.Budget}, name: "ExpectBudget"},
	{fn: "expectOrdinary", name: "ExpectOrdinary"},
	{fn: "expectSlow", name: "expectOrdinarySlow"},
	{fn: "mergeExpected", on: []string{ops.Document, ops.Budget}, name: "MergeExpected"},
	{fn: "keep", on: []string{ops.Budget}, name: "Keep"},
	{fn: "call", typed: true, on: []string{ops.Document, ops.Observe, ops.Typed, ops.Budget}, name: "callBody"},
	{fn: "callMemo", typed: true, on: []string{ops.Document, ops.Observe, ops.Typed}, name: "CallMemo"},
	{fn: "grow", typed: true, on: []string{ops.Typed}, name: "grow"},
	// The observed calls and plain invocations of both runtimes, which their callers take in an
	// observed parse.
	{fn: "observed", on: []string{ops.Document, ops.Observe}, name: "callObserved", body: "callBody"},
	{fn: "observed", on: []string{ops.Document, ops.Observe}, name: "invokePlainObserved", body: "invokePlainBody"},
	{fn: "observed", typed: true, on: []string{ops.Document, ops.Observe}, name: "callObserved", body: "callBody"},
	{fn: "observed", typed: true, on: []string{ops.Document, ops.Observe}, name: "invokePlainObserved", body: "invokePlainBody"},
	// The tracked path of the node runtime: the rule call of a Document parse, the unmemoized and
	// memoized calls, the call without observation, and the entry of the path.
	{fn: "document", on: []string{ops.Document, ops.Budget}, name: "callDocument"},
	{fn: "unmemoized", name: "callUnmemoized"},
	{fn: "memoized", on: []string{ops.Document, ops.Observe, ops.Budget}, name: "callMemo"},
	{fn: "body", on: []string{ops.Document, ops.Budget}, name: "callBody"},
	{fn: "tracked", on: []string{ops.Document, ops.Observe}, name: "callTracked"},
	// The rule call of a Document parse in the typed runtime.
	{fn: "typedDocument", typed: true, on: []string{ops.Document, ops.Observe, ops.Typed, ops.Budget}, name: "DocumentCall"},
	// The predicates and bindings of both runtimes.
	{fn: "predicate", name: "Predicate"},
	{fn: "bind", name: "Bind"},
	{fn: "predicate", typed: true, name: "Predicate"},
	{fn: "bind", typed: true, name: "Bind"},
}

// goStreamSpecs are the functions of the stream mechanism that the compositions replace, in the
// node runtime (the typed runtime has no streams).
var goStreamSpecs = []goSpec{
	{fn: "streamEmit", name: "EmitStream"},
	{fn: "streamCommit", name: "streamCommit"},
	{fn: "streamsIn", name: "StreamsIn"},
}

// goOperatorSpecs are the functions of the Operators Rule loop that the compositions replace, in the
// node and the typed runtime.
var goOperatorSpecs = []goSpec{
	{fn: "operatorsParse", on: []string{ops.Budget}, name: "OperatorsRuleParse"},
	{fn: "operatorsExpr", on: []string{ops.Budget}, name: "operatorsRuleExpr"},
	{fn: "operatorsNud", on: []string{ops.Budget}, name: "operatorsRuleNud"},
	{fn: "longest", on: []string{ops.Budget}, name: "longest"},
	{fn: "skipTrivia", name: "skipTrivia"},
	{fn: "reset", name: "Reset"},
	{fn: "operatorsParseTyped", typed: true, on: []string{ops.Budget, ops.Typed}, name: "OperatorsRuleParse"},
	{fn: "operatorsExprTyped", typed: true, on: []string{ops.Budget, ops.Typed}, name: "operatorsRuleExpr"},
	{fn: "operatorsNudTyped", typed: true, on: []string{ops.Budget, ops.Typed}, name: "operatorsRuleNud"},
	{fn: "longest", typed: true, on: []string{ops.Budget}, name: "longest"},
	{fn: "skipTrivia", typed: true, name: "skipTrivia"},
	{fn: "reset", typed: true, name: "Reset"},
}

// GoOperators returns the Go file of the printed Operators Rule compositions for package runtime.
func GoOperators() (string, error) { return goFile(goOperatorSpecs) }

// GoStream returns the Go file of the printed stream compositions for package runtime.
func GoStream() (string, error) {
	m := Build()
	var b strings.Builder
	b.WriteString("// Code generated by internal/ir/exec/core; DO NOT EDIT.\n\npackage runtime\n")
	for _, s := range goStreamSpecs {
		f, err := pass.Specialize(m, m.Func(s.fn), s.fn, []string{ops.Stream})
		if err != nil {
			return "", err
		}
		src, err := execgo.Func(m, f, goTarget(s))
		if err != nil {
			return "", err
		}
		b.WriteString("\n" + src)
	}
	return b.String(), nil
}

// GoRuntime returns the Go file of the printed compositions for package runtime.
func GoRuntime() (string, error) {
	src, err := goFile(goSpecs)
	if err != nil {
		return "", err
	}
	// The limit on pending edits is defined once, in the operations, and printed for the runtime.
	const header = "package runtime\n"
	limit := fmt.Sprintf(`
// pendingEditLimit is the number of edits after which a Document drops its memo until the next parse
// (ops.PendingEditLimit): the engine treats a memo entry that lags behind by more edits as absent, and
// a generated parser applies each edit to the memo when it is made, which a memo that many edits old
// no longer pays for.
const pendingEditLimit = %d
`, ops.PendingEditLimit)
	return strings.Replace(src, header, header+limit, 1), nil
}

// goFile returns the Go file of the printed functions specs for package runtime.
func goFile(specs []goSpec) (string, error) {
	m := Build()
	var b strings.Builder
	b.WriteString("// Code generated by internal/ir/exec/core; DO NOT EDIT.\n\npackage runtime\n")
	for _, s := range specs {
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
	return b.String(), nil
}

// goTarget returns the Go implementation of the operations in the node or the typed runtime, for
// the printed function s.
func goTarget(s goSpec) *execgo.Target {
	typed := s.typed
	t := &execgo.Target{Recv: "p *Parser", Source: "internal/ir/exec/core"}
	handles := map[string]string{
		ops.Rule: "*Rule", ops.Value: "*Node", ops.Entry: "*memoEntry", ops.Values: "[]any", ops.Errs: "recEntry",
		ops.ExpID: "ExpID", ops.Mark: "ExpMark", ops.Set: "[]ExpID", ops.Kept: "[]ExpID",
		ops.Ctx: "ctxCallState", ops.Res: "*ctxResult", ops.Grow: "growState",
		ops.Frame: "*CaptureFrame", ops.Backtrack: "Mark", ops.Scope: "*Env", ops.Operators: "[]*OperatorsRuleOp",
		ops.Operator: "*OperatorsRuleOp", ops.Line: "*OperatorsRuleLine", ops.Attempt: "operatorsRuleAttempt",
		ops.Term: "func(*ActCtx) any", ops.Datum: "any", ops.Seq: "[]*Node",
		ops.Obj: "*Node", ops.Name: "string", ops.Act: "*ActCtx",
	}
	names := map[string]string{"streamEmit": "EmitStream", "streamCommit": "streamCommit", "streamsIn": "StreamsIn", "call": "Call", "grow": "grow", "unmemoized": "callUnmemoized", "memoized": "callMemo",
		"document": "callDocument", "body": "callBody", "observed": "callObserved", "expectSlow": "expectOrdinarySlow", "expect": "Expect", "mergeExpected": "MergeExpected", "keep": "Keep",
		// A kept set is a slice like the set of an isolation, so MergeExpected merges both.
		"mergeKept": "MergeExpected"}
	for _, s := range goOperatorSpecs {
		names[s.fn] = s.name
	}
	goTermNames(names)
	entryType, valueField, alloc := "memoEntry", "node", "p.memo.alloc()"
	// spender is the parser that the generic copies of values charge.
	attemptType, spender := "operatorsRuleAttempt", "p"
	// created is the field of the nodes that terms and actions create.
	created := "created"
	rule := func(r string) string { return r }
	// step passes the value of a growth step as the node or the typed value of growStep.
	step := func(v string) string { return v + ", nil" }
	if typed {
		t.Recv = "p *TParser"
		handles[ops.Rule], handles[ops.Value], handles[ops.Entry] = "*TRule", "any", "*tmemoEntry"
		handles[ops.Body] = "func(*TParser) (any, bool)"
		handles[ops.Frame], handles[ops.Operators], handles[ops.Operator] = "*TCaptureFrame", "[]*TOperatorsRuleOp", "*TOperatorsRuleOp"
		handles[ops.Line], handles[ops.Attempt] = "*TOperatorsRuleLine", "toperatorsRuleAttempt"
		handles[ops.Term] = "func(*TActCtx) any"
		handles[ops.Seq], handles[ops.Obj], handles[ops.Act] = "[]any", "TVal", "*TActCtx"
		names = map[string]string{"call": "callBody", "callMemo": "CallMemo", "grow": "grow", "keep": "Keep",
			"mergeExpected": "MergeExpected", "mergeKept": "MergeExpected"}
		for _, s := range goOperatorSpecs {
			names[s.fn] = s.name
		}
		goTermNames(names)
		entryType, valueField, alloc = "tmemoEntry", "val", "p.memoSlab.one()"
		attemptType, spender, created = "toperatorsRuleAttempt", "p.Parser", "Created"
		rule = func(r string) string { return r + ".Rule" }
		step = func(v string) string { return "nil, " + v }
	}
	names[s.fn] = s.name
	t.Handle = func(name string) string { return handles[name] }
	t.FuncName = func(name string) string { return names[name] }
	t.Reg = func(name string) string {
		return map[string]string{ops.Pos: "p.Pos", ops.Silent: "p.Silent", ops.Cut: "p.Cut"}[name]
	}
	t.Op = func(c *exec.Call, a []string) string {
		if s, ok := goValueOp(c.Callee, a, typed); ok {
			return s
		}
		switch c.Callee {
		case "depth.enter":
			return "p.Depth++\nif p.Depth > p.MaxDepth {\np.TooDeep()\n}"
		case "depth.exit":
			return "p.Depth--"
		case "frame.mark":
			return "p.Mark()"
		case "frame.markPos":
			return a[0] + ".Pos"
		case "frame.markRec":
			return a[0] + ".recovered"
		case "frame.markEnv":
			return a[0] + ".env"
		case "frame.markTrail":
			return a[0] + ".Trail"
		case "frame.undo":
			if typed {
				return fmt.Sprintf("p.undoCaptures(%s)\np.DropTrail(%[1]s)", a[0])
			}
			return fmt.Sprintf("p.undoCaptures(%s)\np.Trail = p.Trail[:%[1]s]", a[0])
		case "frame.current":
			return "p.Frame"
		case "frame.use":
			return "p.Frame = " + a[0]
		case "frame.open":
			return "p.NewFrame(" + a[0] + ")"
		case "frame.slots":
			return "len(" + a[0] + ".Vals)"
		case "frame.set":
			return fmt.Sprintf("p.SetCapture(%s, %s)", a[0], a[1])
		case "frame.close":
			if typed {
				return "p.FreeFrame(" + a[0] + ")"
			}
			return ""
		case "frame.save":
			return fmt.Sprintf("p.saved = appendSpend(%s, p.saved[:0], %s.Vals)", spender, a[0])
		case "frame.restore":
			return fmt.Sprintf("copySpend(%s, %s.Vals, p.saved)", spender, a[0])
		case "env.restore":
			return "p.Env = " + a[0]
		case "env.bind":
			return fmt.Sprintf("p.bind(%s, %s)", a[0], a[1])
		case "term.eval":
			return "p.evalTerm(" + a[0] + ")"
		case "term.created":
			return "len(p." + created + ")"
		case "term.dropCreated":
			return fmt.Sprintf("clearSpend(%s, p.%s[%s:])\np.%[2]s = p.%[2]s[:%[3]s]", spender, created, a[0])
		case "term.kids":
			return "len(p.KidStack)"
		case "term.dropKids":
			return fmt.Sprintf("clearSpend(%s, p.KidStack[%s:])\np.KidStack = p.KidStack[:%[2]s]", spender, a[0])
		case "term.isFalse":
			return "isFalse(" + a[0] + ")"
		case "term.isFunction":
			return "isFunction(" + a[0] + ")"
		case "opr.led":
			return a[0] + ".OperatorsRule.Led"
		case "opr.prefix":
			return a[0] + ".OperatorsRule.Prefix"
		case "opr.count":
			return "len(" + a[0] + ")"
		case "opr.at":
			return a[0] + "[" + a[1] + "]"
		case "opr.operands":
			return "len(" + a[0] + ".OperatorsRule.Operands)"
		case "opr.operand":
			return a[0] + ".OperatorsRule.Operands[" + a[1] + "]"
		case "opr.line":
			return a[0] + ".Line"
		case "opr.level":
			return a[0] + ".Level"
		case "opr.infix":
			return a[0] + `.Kind == "infix"`
		case "opr.postfix":
			return a[0] + `.Kind == "postfix"`
		case "opr.right":
			return a[0] + `.Assoc == "right"`
		case "opr.nonAssoc":
			return a[0] + `.Assoc == "none"`
		case "opr.scope":
			return "len(" + a[0] + ".Scope)"
		case "opr.hasSkip":
			return a[0] + ".OperatorsRule.Skip != nil"
		case "opr.match":
			return a[0] + ".M(p)"
		case "opr.lineValue":
			return fmt.Sprintf("p.lineResult(%s, %s, %s, %s, %s)", a[0], a[1], a[2], a[3], a[4])
		case "opr.build":
			return fmt.Sprintf("p.operatorsRuleBuild(%s, &%s, %s, %s)", a[0], a[1], a[2], a[3])
		case "opr.buildTyped":
			return fmt.Sprintf("p.operatorsRuleBuild(%s, &%s, %s, %s, %s)", a[0], a[1], a[2], a[3], a[4])
		case "opr.attempt":
			return fmt.Sprintf("%s{op: %s, frame: %s, v: %s, start: %s, end: p.Pos, env: p.Env, cut: %s, errs: %s}",
				attemptType, a[0], a[1], a[2], a[3], a[4], a[5])
		case "opr.found":
			return a[0] + ".op != nil"
		case "opr.op", "opr.end", "opr.cut", "opr.env", "opr.errs":
			return a[0] + "." + strings.TrimPrefix(c.Callee, "opr.")
		case "opr.setCutFailed":
			return "p.cutFailed = " + a[0]
		case "opr.cutFailed":
			return "p.cutFailed"
		case "opr.setNudBound":
			return "p.nudBound = " + a[0]
		case "opr.nudBound":
			return "p.nudBound"
		case "call.tracked":
			if typed {
				return "p.TypedDocument != nil"
			}
			return "p.Observation != nil || p.Document != nil"
		case "call.track":
			if typed {
				return fmt.Sprintf("p.DocumentCall(%s, %s, nil)", a[0], a[1])
			}
			return fmt.Sprintf("p.callTracked(%s, %s)", a[0], a[1])
		case "call.trackBody":
			return fmt.Sprintf("p.DocumentCall(%s, 0, %s)", a[0], a[1])
		case "budget.charge":
			// Spend is the charge of one unit, small enough to inline.
			if k, ok := c.Args[0].(*exec.Const); ok && k.Value == uint64(1) {
				return "p.Spend()"
			}
			return "p.Control.Charge(" + a[0] + ")"
		case "budget.freeBegin":
			return "p.Control.FreeBegin(" + a[0] + ")"
		case "budget.freeEnd":
			return "p.Control.FreeEnd(" + a[0] + ")"
		case "call.first":
			return "p.FirstCall(" + rule(a[0]) + ")"
		case "rule.memo":
			return a[0] + ".Memo"
		case "rule.leader":
			return a[0] + ".Leader"
		case "rule.plain":
			return "len(" + a[0] + ".Scope) == 0"
		case "rule.docMemo":
			return a[0] + ".DocMemo"
		case "rule.docOnlyMemo":
			return "(" + a[0] + ".DocMemo && !" + a[0] + ".Memo && !" + a[0] + ".Leader)"
		case "rule.positional":
			return a[0] + ".Positional"
		case "rule.id":
			return a[0] + ".Id"
		case "rule.invoke":
			return fmt.Sprintf("p.invoke(%s, %s)", a[0], a[1])
		case "rule.invokePlain":
			return fmt.Sprintf("p.invokePlainBody(%s, %s)", a[0], a[1])
		case "rule.run":
			return a[0] + "(p)"
		case "env.key":
			return "p.memoEnv(" + rule(a[0]) + ")"
		case "rec.mark":
			return "len(p.Recovered)"
		case "rec.truncate":
			return "p.Recovered = p.Recovered[:" + a[0] + "]"
		case "rec.since":
			return "p.recSince(" + a[0] + ")"
		case "rec.add":
			return "p.recAdd(" + a[0] + ")"
		case "rec.none":
			return a[0] + ".none()"
		case "exp.farthest":
			return "p.farthest"
		case "exp.recording":
			return "p.Expectations"
		case "exp.reset":
			return fmt.Sprintf("p.farthest = %s\np.exp = p.exp[:p.expBase]\np.expBits = 0", a[0])
		case "exp.fresh":
			return "len(p.exp) == p.expBase"
		case "exp.placeholder":
			return "p.exp = append(p.exp, ExpFar)"
		case "exp.maybe":
			return fmt.Sprintf("p.expBits&(uint64(1)<<(uint32(%s)*0x9e3779b1>>26)) != 0", a[0])
		case "exp.count":
			return "len(p.exp) - p.expBase"
		case "exp.at":
			return "p.exp[p.expBase+" + a[0] + "]"
		case "exp.add":
			return fmt.Sprintf("p.expBits |= uint64(1) << (uint32(%s) * 0x9e3779b1 >> 26)\np.exp = append(p.exp, %[1]s)", a[0])
		case "exp.setCount":
			return "len(" + a[0] + ")"
		case "exp.setAt":
			return a[0] + "[" + a[1] + "]"
		case "exp.isolate":
			return "p.Isolate(" + a[0] + ")"
		case "exp.unisolate":
			return "p.Unisolate(" + a[0] + ")"
		case "exp.empty":
			return "nil"
		case "exp.last":
			return "p.lastKept"
		case "exp.keptCount":
			return "len(" + a[0] + ")"
		case "exp.keptAt":
			return a[0] + "[" + a[1] + "]"
		case "memo.far", "memo.expected", "memo.ok", "memo.end", "memo.silent", "memo.errs":
			return a[0] + "." + strings.TrimPrefix(c.Callee, "memo.")
		case "memo.value":
			return a[0] + "." + valueField
		case "memo.setExpected":
			return fmt.Sprintf("%s.far, %s.expected = %s, %s", a[0], a[0], a[1], a[2])
		case "obs.memoHit":
			return "p.Observation.memoHit()"
		case "obs.enter":
			return fmt.Sprintf("p.Observation.Enter(%s.Name, %s, %s)", a[0], a[1], a[2])
		case "obs.exit":
			return fmt.Sprintf("p.Observation.Exit(%s, %s, %s, %s)", a[0], a[1], a[2], a[3])
		case "doc.examinedBegin":
			return "p.ExaminedBegin()"
		case "doc.examinedEnd":
			return "p.ExaminedEnd(" + a[0] + ")"
		case "doc.leave":
			// The binding level of a call is named min: the low end is lowered without the builtin.
			return fmt.Sprintf("p.docHigh = max(%s, p.docHigh)\nif %s < p.docLow {\np.docLow = %s\n}", a[0], a[1], a[1])
		case "doc.touchFrom":
			return "p.documentTouchFrom(" + a[0] + ")"
		case "doc.low":
			return "p.docLow"
		case "doc.move":
			if typed {
				return "p.TypedDocument.move(" + a[0] + ")"
			}
			return "p.documentMove(" + a[0] + ")"
		case "memo.before":
			if typed {
				return "p.TypedDocument.meta[" + a[0] + "].before"
			}
			return a[0] + ".from == fromBefore"
		case "memo.pin":
			if typed {
				return "p.TypedDocument.pin(" + a[0] + ")"
			}
			return a[0] + ".pinned = true"
		case "obs.active":
			return "p.Observation != nil"
		case "doc.active":
			return "p.Document != nil"
		case "doc.touch":
			return "p.documentTouch(" + a[0] + ")"
		case "doc.high":
			return "p.docHigh"
		case "memo.examined":
			if typed {
				return "p.TypedDocument.meta[" + a[0] + "].examined"
			}
			return a[0] + ".examined"
		case "doc.evaluated":
			return "p.Document.stats.Evaluated++"
		case "memo.forgetDoc":
			return "delete(p.TypedDocument.meta, " + a[0] + ")"
		case "doc.reused":
			return "p.Document.stats.Reused++"
		case "memo.examine":
			if typed {
				return "p.TypedDocument.meta[" + a[0] + "] = typedDocumentMeta{examined: " + a[2] + ", recorded: true, before: " + a[1] + "}"
			}
			return fmt.Sprintf("%s.examined, %s.from, %s.pinned = %s, examinedFromOf(%s), false", a[0], a[0], a[0], a[2], a[1])
		case "call.body":
			return fmt.Sprintf("p.%s(%s, %s)", s.body, a[0], a[1])
		case "ctx.found":
			return a[0] + " != nil"
		case "ctx.ok":
			return a[0] + ".ok"
		case "ctx.value":
			return a[0] + "." + valueField
		case "ctx.replay":
			return fmt.Sprintf("p.replayCtx(%s, %s)", a[0], a[1])
		case "ctx.allows":
			return fmt.Sprintf("%s.memoAllows(%s.present)", a[0], a[1])
		case "ctx.active":
			return a[0] + ".ctx"
		case "ctx.lookup":
			return fmt.Sprintf("p.ctxLookup(&%s, %s.Id, %s, %s)", a[0], a[1], a[2], a[3])
		case "ctx.push":
			return "p.ctxPush(&" + a[0] + ")"
		case "ctx.pop":
			return "p.ctxPop(&" + a[0] + ")"
		case "ctx.storeMemo":
			return "storeMemo"
		case "ctx.storeCtx":
			return "storeCtx"
		case "memo.setPresent":
			return fmt.Sprintf("%s.present = %s.present", a[0], a[1])
		case "ctx.keep":
			return fmt.Sprintf("p.ctxKeep(&%s, %s.Id, %s, %s, %s, &ctxResult{%s: %s, ok: %s, end: %s, examined: p.examinedNow(), silent: %s, errs: %s, far: %s, expected: %s})",
				a[0], a[1], a[2], a[3], a[4], valueField, a[8], a[5], a[6], a[7], a[9], a[10], a[11])
		case "ctx.done":
			return "p.ctxDone(&" + a[0] + ")"
		case "depth.current":
			return "p.Depth"
		case "stream.emits":
			return "p.Stream != nil && p.Stream.Emit != nil"
		case "stream.start":
			return "p.Stream.start"
		case "stream.check":
			return "p.Control.Check()"
		case "stream.report":
			return "p.streamReport()"
		case "stream.deliver":
			return "p.streamDeliver(" + a[0] + ")"
		case "stream.discard":
			return "p.streamDiscard(" + a[0] + ")"
		case "stream.release":
			return "p.streamRelease()"
		case "memo.floor":
			return "p.Stream.memoBase"
		case "memo.prune":
			return "p.pruneStreamMemo(" + a[0] + ")"
		case "stream.dropRecords":
			return "p.streamDropRecords(" + a[0] + ")"
		case "stream.tail":
			return "p.Stream.tail.limit > 0"
		case "stream.settleErrors":
			return "p.streamSettleErrors()"
		case "grow.begin":
			return fmt.Sprintf("p.growBegin(%s, %s)", rule(a[0]), a[1])
		case "grow.next":
			return "p.growNext(&" + a[0] + ")"
		case "grow.step":
			return fmt.Sprintf("p.growStep(&%s, %s, %s)", a[0], step(a[1]), a[2])
		case "grow.end":
			return "p.growEnd(&" + a[0] + ")"
		}
		panic("core: no Go implementation of " + c.Callee)
	}
	t.Negate = func(c *exec.Call, a []string) (string, bool) {
		switch c.Callee {
		case "opr.found":
			return a[0] + ".op == nil", true
		case "opr.hasSkip":
			return a[0] + ".OperatorsRule.Skip == nil", true
		case "opr.infix":
			return a[0] + `.Kind != "infix"`, true
		case "rec.none":
			return "!" + a[0] + ".none()", true
		}
		return "", false
	}
	t.Stmt = func(c *exec.Call, a []string, dst []string, define bool) (string, bool) {
		op := " = "
		if define {
			op = " := "
		}
		key := func() string { return strings.Join(a[:4], ", ") }
		switch c.Callee {
		case "opr.skip":
			return fmt.Sprintf("_, %s%s%s.OperatorsRule.Skip(p)", dst[0], op, a[0]), true
		case "memo.lookup":
			s := dst[0] + op + "p.memoGet(" + key() + ", " + a[4] + ")"
			if dst[1] != "_" {
				s += "\n" + dst[1] + op + dst[0] + " != nil"
			}
			return s, true
		case "ctx.enter":
			// The context of a call of a rule of no cycle, or outside every growth, is empty.
			c, seed := dst[0], dst[1]
			s := ""
			if define {
				s = fmt.Sprintf("var %s ctxCallState\nvar %s *ctxResult\n", c, seed)
			}
			return s + fmt.Sprintf("if %s.Cycle != 0 && len(p.gamma) != 0 {\n%s = p.ctxEnter(&%s, %s, %s)\n}", a[0], seed, c, rule(a[0]), a[1]), true
		case "exp.copy":
			// The kept sets fill append-only chunks of the arena.
			return fmt.Sprintf(`if cap(p.arena)-len(p.arena) < len(%[1]s) {
// A new chunk; the previous one lives while sets in it remain.
p.arena = make([]ExpID, 0, max(1024, len(%[1]s)))
p.memUsed += uint64(cap(p.arena)) * expIDBytes
}
off := len(p.arena)
p.arena = appendSpend(p, p.arena, %[1]s)
p.lastKept = p.arena[off:len(p.arena):len(p.arena)]
%[2]s%[3]sp.lastKept`, a[0], dst[0], op), true
		case "doc.enter":
			return fmt.Sprintf("%s, %s%sp.docHigh, p.docLow\np.docHigh, p.docLow = %s+1, %s", dst[0], dst[1], op, a[0], a[0]), true
		case "memo.store":
			e := dst[0]
			fields := fmt.Sprintf("%s: %s, ok: %s, end: %s, silent: %s, errs: %s", valueField, a[7], a[4], a[5], a[6], a[8])
			return fmt.Sprintf("%s%s%s\n*%s = %s{%s}\np.memoPut(%s, %s, %s)", e, op, alloc, e, entryType, fields, key(), a[9], e), true
		}
		return "", false
	}
	return t
}
