package core

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/execgo"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/pass"
)

// EngineFile is the file of the printed compositions of the engine, relative to the directory of
// this package.
const EngineFile = "../../../engine/core/call_exec.go"

// EngineOperatorsFile is the file of the printed Operators Rule compositions of the engine.
const EngineOperatorsFile = "../../../engine/core/operators_exec.go"

// EngineStreamFile is the file of the printed stream compositions of the engine.
const EngineStreamFile = "../../../engine/core/stream_exec.go"

// EngineDocumentFile is the file of the printed Document compositions of the engine.
const EngineDocumentFile = "../../../engine/core/document_exec.go"

// EngineClosureTermFile is the file of the printed term compositions of the closure backend.
const EngineClosureTermFile = "../../../engine/closure/term_exec.go"

// EngineVMTermFile is the file of the printed term compositions of the bytecode VM.
const EngineVMTermFile = "../../../engine/vm/term_exec.go"

// engineOn are the features of the engine's specializations: it keeps the bookkeeping of the
// examined input in every parse, has the missing-token mode and charges a budget. Its observation
// (tracing) is a tracked path of its own.
var engineOn = []string{ops.Document, ops.Missing, ops.Budget}

// engineSpecs are the functions of the engine that the compositions replace.
var engineSpecs = []goSpec{
	{fn: "engineCall", on: engineOn, name: "Call"},
	{fn: "engineCallDoc", on: engineOn, name: "callDoc"},
	{fn: "callBegin", on: engineOn, name: "CallBegin"},
	{fn: "callEnd", on: engineOn, name: "CallEnd"},
	{fn: "grow", name: "grow"},
	// The engine's backends record the examined input where they read it, so its expectations are
	// specialized without the Document's touch of the unit at the expectation.
	{fn: "expect", on: []string{ops.Budget}, name: "Expect"},
	{fn: "mergeExpected", on: []string{ops.Budget}, name: "MergeExpected"},
	{fn: "keep", on: []string{ops.Budget}, name: "Keep"},
}

// engineOperatorSpecs are the functions of the Operators Rule loop of the engine: the recursive forms
// of the closure backend and the bytecode VM, and the split forms of the iterative VM.
var engineOperatorSpecs = func() []goSpec {
	on := []string{ops.Budget, ops.Missing}
	specs := []goSpec{
		{fn: "operatorsParse", on: on, name: "OperatorsRuleParse"},
		{fn: "operatorsExpr", on: on, name: "operatorsRuleExpr"},
		{fn: "operatorsNud", on: on, name: "operatorsRuleNud"},
		{fn: "longest", on: on, name: "Longest"},
		{fn: "skipTrivia", on: []string{ops.Missing}, name: "skipTrivia"},
		{fn: "reset", on: []string{ops.Missing}, name: "Reset"},
	}
	for _, s := range engineOperatorSplit {
		specs = append(specs, goSpec{fn: s, on: on, name: strings.ToUpper(s[:1]) + s[1:]})
	}
	return specs
}()

// engineOperatorSplit are the split forms of the Operators Rule loop.
var engineOperatorSplit = []string{"triviaBegin", "triviaEnd", "longestBegin", "longestNext", "longestStep", "nudBegin", "nudPrefix",
	"nudRhs", "nudNext", "nudOperand", "exprBegin", "exprStart", "exprLoop", "exprLed", "exprRhs"}

// engineStreamSpecs are the functions of the stream mechanism of the engine. Every backend, the
// iterative VM among them, delivers and commits an element in straight-line code that runs no rule
// body, so the compositions need no split form.
var engineStreamSpecs = []goSpec{
	{fn: "streamEmit", on: []string{ops.Stream}, name: "EmitStream"},
	{fn: "streamCommit", on: []string{ops.Stream}, name: "Commit"},
	{fn: "streamsIn", on: []string{ops.Stream}, name: "StreamsIn"},
}

// engineDocumentSpecs are the functions of the edits of a Document in the engine: the form a parse
// runs, which charges the edits it applies, and the form an edit runs, which has no parse to charge.
var engineDocumentSpecs = []goSpec{
	{fn: "memoAdvance", on: []string{ops.Document, ops.Budget}, name: "AdvanceEntry"},
	{fn: "memoAdvance", on: []string{ops.Document}, name: "AdvanceEntry", plain: true},
}

// Engine returns the Go file of the printed compositions for package core of the engine.
func Engine() (string, error) {
	return engineFile(engineSpecs, "import \"zeroparse.dev/zp/internal/value\"\n")
}

// EngineOperators returns the Go file of the printed Operators Rule compositions for package core of
// the engine.
func EngineOperators() (string, error) {
	return engineFile(engineOperatorSpecs, "import (\n\t\"zeroparse.dev/zp/grammar\"\n\t\"zeroparse.dev/zp/internal/value\"\n)\n")
}

// EngineStream returns the Go file of the printed stream compositions for package core of the engine.
func EngineStream() (string, error) {
	return engineFile(engineStreamSpecs, "import \"zeroparse.dev/zp/internal/value\"\n")
}

// EngineDocument returns the Go file of the printed Document compositions for package core of the
// engine.
func EngineDocument() (string, error) { return engineFile(engineDocumentSpecs, "") }

// EngineClosureTerms returns the Go file of the term compositions for the closure backend, which
// holds a term as a site of its matcher.
func EngineClosureTerms() (string, error) {
	return engineTerms("closure", &execgo.Target{Params: "p *core.Parser"}, "*termSite", "Scope: t.scope, ", "evalTerm(ctx, t.term)")
}

// EngineVMTerms returns the Go file of the term compositions for the bytecode VM, which holds a term
// as the entry of its expression code.
func EngineVMTerms() (string, error) {
	return engineTerms("vm", &execgo.Target{Recv: "vm *Program", Params: "p *core.Parser"}, "int", "", "vm.eval(ctx, t, nil)")
}

// engineBackendFuncs are the compositions that each backend of the engine prints into its own
// package, without features, by engineTerms.
var engineBackendFuncs = []string{"predicate", "bind"}

// engineTerms prints the term compositions into a backend's own package, over what the backend
// calls a term: a backend evaluates terms in its own form, so a printed function reaches the
// evaluation with a direct call, and the composition costs no indirection on the path of a matcher.
// scope is the field of the evaluation context that the term t selects, and eval the evaluation of t
// in the context ctx.
func engineTerms(pkg string, t *execgo.Target, term, scope, eval string) (string, error) {
	m := Build()
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by internal/ir/exec/core; DO NOT EDIT.\n\npackage %s\n\nimport \"zeroparse.dev/zp/internal/engine/core\"\n", pkg)
	t.Source = "internal/ir/exec/core"
	t.Handle = func(name string) string { return map[string]string{ops.Term: term, ops.Datum: "any"}[name] }
	t.Stmt = func(c *exec.Call, a []string, dst []string, _ bool) (string, bool) {
		if c.Callee != "term.eval" {
			return "", false
		}
		return fmt.Sprintf("ctx := p.UseCtx(core.EvalCtx{P: p, %sFrame: p.Frame, Start: p.Pos, End: p.Pos, Cbase: len(p.Created)})\n"+
			"%s, err := %s\n%s := err == nil", scope, dst[0], eval, dst[1]), true
	}
	t.Op = func(c *exec.Call, a []string) string {
		switch c.Callee {
		case "term.created":
			return "len(p.Created)"
		case "term.dropCreated":
			// Most terms create nothing, and the call is on the path of every matcher of a term.
			return fmt.Sprintf("if len(p.Created) > %s {\np.DropCreated(%[1]s)\n}", a[0])
		case "term.kids":
			return "len(p.KidStack)"
		case "term.dropKids":
			return "p.DropKids(" + a[0] + ")"
		case "term.isFalse":
			return "core.IsFalse(" + a[0] + ")"
		case "term.isFunction":
			return "isFunction(" + a[0] + ")"
		case "env.bind":
			return fmt.Sprintf("p.Bind(%s, %s)", a[0], a[1])
		}
		panic("core: no engine term implementation of " + c.Callee)
	}
	for _, fn := range engineBackendFuncs {
		f, err := pass.Specialize(m, m.Func(fn), fn, nil)
		if err != nil {
			return "", err
		}
		src, err := execgo.Func(m, f, t)
		if err != nil {
			return "", err
		}
		b.WriteString("\n" + src)
	}
	return b.String(), nil
}

// engineFile returns the Go file of the printed functions specs with the imports imports.
func engineFile(specs []goSpec, imports string) (string, error) {
	m := Build()
	var b strings.Builder
	b.WriteString("// Code generated by internal/ir/exec/core; DO NOT EDIT.\n\npackage core\n")
	if imports != "" {
		b.WriteString("\n" + imports)
	}
	for _, s := range specs {
		f, err := pass.Specialize(m, m.Func(s.fn), s.fn, s.on)
		if err != nil {
			return "", err
		}
		t := engineTarget()
		if s.plain {
			t.Recv = ""
		}
		src, err := execgo.Func(m, f, t)
		if err != nil {
			return "", err
		}
		b.WriteString("\n" + src)
	}
	return b.String(), nil
}

// engineTarget returns the Go implementation of the operations in the engine. The printed call
// protocol names its rule r and its context state c, which the implementations of memo.store,
// ctx.keep and grow.end read: a leader's result is the entry that its growth made (Parser.grown),
// and an entry made in a context comes from the heap (Parser.entry).
func engineTarget() *execgo.Target {
	t := &execgo.Target{Recv: "p *Parser", Source: "internal/ir/exec/core"}
	handles := map[string]string{
		ops.Rule: "*Rule", ops.Value: "*value.Node", ops.Entry: "*MemoEntry", ops.Values: "KeyVals", ops.Errs: "RecEntry",
		ops.Mark: "ExpMark", ops.Set: "[]ExpID", ops.Kept: "[]ExpID", ops.ExpID: "ExpID",
		ops.Ctx: "CtxState", ops.Res: "ctxHit", ops.Grow: "GrowState", ops.CallState: "*CallState",
		ops.Frame: "*Frame", ops.Backtrack: "Mark", ops.Scope: "*Env", ops.Operators: "[]*OperatorsRuleOp",
		ops.Operator: "*OperatorsRuleOp", ops.Line: "*OperatorsRuleLine", ops.Attempt: "OperatorsRuleAttempt",
		ops.Seq: "[]*value.Node", ops.Datum: "any", ops.LongestState: "*LongestState", ops.NudState: "*NudState", ops.ExprState: "*ExprState", ops.EditLog: "[]DocEdit",
		ops.Obj: "*value.Node", ops.Name: "string", ops.Str: "string", ops.Act: "*EvalCtx", ops.Fail: "error", ops.Elem: "*value.Node",
		ops.NodeType: "*value.NodeTypeKinds", ops.FType: "*compile.FieldType",
	}
	names := map[string]string{"engineCall": "Call", "engineCallDoc": "callDoc", "callBegin": "CallBegin", "callEnd": "CallEnd", "grow": "grow",
		"expect": "Expect", "keep": "Keep",
		// A kept set is a slice like the set of an isolation, so MergeExpected merges both.
		"mergeKept": "MergeExpected", "mergeExpected": "MergeExpected"}
	for _, specs := range [][]goSpec{engineOperatorSpecs, engineStreamSpecs, engineDocumentSpecs} {
		for _, s := range specs {
			names[s.fn] = s.name
		}
	}
	goTermNames(names)
	t.Handle = func(name string) string { return handles[name] }
	t.FuncName = func(name string) string { return names[name] }
	t.Reg = func(name string) string {
		return map[string]string{ops.Pos: "p.Pos", ops.Silent: "p.Silent", ops.Cut: "p.Cut"}[name]
	}
	// key is the memo key of the first four arguments of a memo operation, and whether its rule only a Document memoizes.
	key := func(a []string, docOnly string) string {
		return fmt.Sprintf("MemoKey{Rule: %s, Pos: %s, Min: %s, Env: %s.Env, MissingSilent: %[4]s.Missing, DocOnly: %s}", a[0], a[1], a[2], a[3], docOnly)
	}
	t.Op = func(c *exec.Call, a []string) string {
		if s, ok := engineValueOp(c.Callee, a); ok {
			return s
		}
		// The records of the split forms of the Operators Rule loop have a Go field for each field.
		if rec, field, ok := strings.Cut(c.Callee, "."); ok && (rec == "oprLongest" || rec == "oprNud" || rec == "oprExpr") {
			if name, ok := strings.CutPrefix(field, "set"); ok && len(a) == 2 {
				return a[0] + "." + name + " = " + a[1]
			}
			return a[0] + "." + strings.ToUpper(field[:1]) + field[1:]
		}
		switch c.Callee {
		case "stream.emits":
			return "p.Emit != nil"
		case "stream.start":
			return "p.StreamID"
		case "stream.check":
			return "if BudgetControlsEnabled {\np.Control.Check()\n}"
		// The report and the delivery run for every element, so they stay in the printed function: a
		// call for each costs rows with small elements up to 3% (record 065).
		case "stream.report":
			return "if p.Diagnostic != nil || p.DiagnosticTail.Limit > 0 {\np.streamReport()\n}"
		case "stream.deliver":
			// EmitCall marks only the callback's execution, so that a later fault of the VM keeps its
			// attribution.
			return fmt.Sprintf("if %[1]s != nil {\nvalue.D(%[1]s).Fresh = false\n}\np.EmitCall = true\nerr := p.Emit(%[1]s)\np.EmitCall = false\nif err != nil {\npanic(Fatal{Err: err})\n}", a[0])
		case "stream.discard":
			return "p.Discard(" + a[0] + ")"
		case "stream.release":
			return "p.streamRelease()"
		case "stream.dropRecords":
			return "p.streamDropRecords(" + a[0] + ")"
		case "stream.tail":
			return "p.DiagnosticTail.Limit > 0"
		case "stream.settleErrors":
			return "p.streamSettleErrors()"
		case "memo.floor":
			return "p.Memo.Base"
		case "memo.prune":
			return "p.PruneStreamMemo(" + a[0] + ")"
		case "depth.current":
			return "p.Depth"
		case "depth.enter":
			return "p.Enter()"
		case "depth.exit":
			return "p.Depth--"
		case "frame.mark":
			return "p.Mark()"
		case "frame.markPos":
			return a[0] + ".Pos"
		case "frame.markRec":
			return a[0] + ".Recovered"
		case "frame.markEnv":
			return a[0] + ".Env"
		case "frame.markTrail":
			return a[0] + ".Trail"
		case "frame.markMissing":
			return a[0] + ".missing"
		case "frame.undo":
			return fmt.Sprintf("p.UndoCaptures(%s)\np.Trail = p.Trail[:%[1]s]", a[0])
		case "frame.current":
			return "p.Frame"
		case "frame.use":
			return "p.Frame = " + a[0]
		case "frame.set":
			return fmt.Sprintf("p.SetCapture(%s, %s)", a[0], a[1])
		case "frame.close":
			return ""
		case "frame.open":
			return "p.NewFrame(" + a[0] + ")"
		case "frame.slots":
			return "len(" + a[0] + ".Vals)"
		case "frame.save":
			return fmt.Sprintf("p.Saved = AppendSpend(p, p.Saved[:0], %s.Vals)", a[0])
		case "frame.restore":
			return fmt.Sprintf("CopySpend(p, %s.Vals, p.Saved)", a[0])
		case "env.restore":
			return "p.Env = " + a[0]
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
			return a[0] + ".Kind == grammar.Infix"
		case "opr.postfix":
			return a[0] + ".Kind == grammar.Postfix"
		case "opr.right":
			return a[0] + ".Assoc == grammar.AssocRight"
		case "opr.nonAssoc":
			return a[0] + ".Assoc == grammar.AssocNone"
		case "opr.scope":
			return "len(" + a[0] + ".Scope.Names)"
		case "opr.hasSkip":
			return a[0] + ".OperatorsRule.Skip != nil"
		case "opr.match":
			return a[0] + ".M(p)"
		case "opr.lineValue":
			return fmt.Sprintf("p.LineResult(%s, %s, %s, %s, %s, nil)", a[0], a[1], a[2], a[3], a[4])
		case "opr.build":
			return fmt.Sprintf("p.OperatorsRuleBuild(%s, &%s, %s, %s)", a[0], a[1], a[2], a[3])
		case "opr.attempt":
			return fmt.Sprintf("OperatorsRuleAttempt{Op: %s, Frame: %s, V: %s, Start: %s, End: p.Pos, Env: p.Env, Cut: %s, Errs: %s}",
				a[0], a[1], a[2], a[3], a[4], a[5])
		case "opr.attemptMissing":
			return fmt.Sprintf("OperatorsRuleAttempt{Op: %s, Frame: %s, V: %s, Start: %s, End: p.Pos, Env: p.Env, Cut: %s, Errs: %s, missing: p.MissingEvents(%s)}",
				a[0], a[1], a[2], a[3], a[4], a[5], a[6])
		case "opr.replay":
			return fmt.Sprintf("if p.Missing != nil || !%s.Errs.None() {\np.ApplyRecovered(&%[1]s)\n}", a[0])
		case "opr.found":
			return a[0] + ".Op != nil"
		case "opr.op", "opr.end", "opr.cut", "opr.env", "opr.errs":
			name := strings.TrimPrefix(c.Callee, "opr.")
			return a[0] + "." + strings.ToUpper(name[:1]) + name[1:]
		case "opr.setCutFailed":
			return "p.cutFailed = " + a[0]
		case "opr.cutFailed":
			return "p.cutFailed"
		case "opr.setNudBound":
			return "p.nudBound = " + a[0]
		case "opr.nudBound":
			return "p.nudBound"
		case "opr.setAfterEmpty":
			return "p.afterEmpty = " + a[0]
		case "opr.afterEmpty":
			return "p.afterEmpty"
		case "rec.none":
			return a[0] + ".None()"
		case "budget.charge":
			if k, ok := c.Args[0].(*exec.Const); ok && k.Value == uint64(1) {
				return "p.Spend()"
			}
			return "p.SpendN(" + a[0] + ")"
		case "doc.active":
			return "p.MemoAll"
		case "budget.freeBegin":
			return "p.FreeBegin(" + a[0] + ")"
		case "budget.freeEnd":
			return "p.FreeEnd(" + a[0] + ")"
		case "rule.docOnlyMemo":
			return "p.docOnlyMemo(" + a[0] + ")"
		case "rule.docPlain":
			return a[0] + ".Plain"
		case "call.tracked":
			return "p.Tr != nil && p.tracing()"
		case "call.track":
			return fmt.Sprintf("p.tracedCall(%s, %s)", a[0], a[1])
		case "call.first":
			return "p.FirstCall(" + a[0] + ")"
		case "call.state":
			return "&CallState{}"
		case "call.save":
			return fmt.Sprintf("%s.start, %[1]s.min, %[1]s.key, %[1]s.ctx, %[1]s.rec, %[1]s.exp = %s, %s, %s, %s, %s, %s", a[0], a[1], a[2], a[3], a[4], a[5], a[6])
		case "call.saveExamined":
			return fmt.Sprintf("%s.high, %[1]s.low = %s, %s", a[0], a[1], a[2])
		case "call.saveMissing":
			return a[0] + ".missing = " + a[1]
		case "call.start", "call.min", "call.key", "call.ctx", "call.rec", "call.high", "call.low", "call.missing":
			return a[0] + "." + strings.TrimPrefix(c.Callee, "call.")
		case "call.mark":
			return a[0] + ".exp"
		case "rule.memo":
			return "p.memoized(" + a[0] + ")"
		case "rule.leader":
			return a[0] + ".Leader"
		case "rule.plain":
			return "len(" + a[0] + ".Scope.Names) == 0"
		case "rule.positional":
			return a[0] + ".Positional"
		case "rule.id":
			return a[0] + ".ID"
		case "rule.invoke":
			return fmt.Sprintf("p.invoke(%s, %s)", a[0], a[1])
		case "rule.invokePlain":
			return fmt.Sprintf("p.InvokePlain(%s, %s)", a[0], a[1])
		case "rec.mark":
			return "len(p.Recovered)"
		case "rec.truncate":
			return "p.Recovered = p.Recovered[:" + a[0] + "]"
		case "rec.since":
			return "p.recSince(" + a[0] + ")"
		case "rec.add":
			return "p.recAdd(" + a[0] + ")"
		case "missing.active":
			return "p.Missing != nil"
		case "missing.recompute":
			return "p.Missing != nil && " + a[0] + ".Missing"
		case "missing.mark":
			return "p.missingMark()"
		case "missing.reset":
			return fmt.Sprintf("if p.Missing != nil {\np.ResetMissing(%s)\n}", a[0])
		case "missing.replay":
			return fmt.Sprintf("p.ReplayMissing(p.MissingSeedEvents(%s), %[1]s.Errs)", a[0])
		case "exp.isolate":
			return "p.Isolate(" + a[0] + ")"
		case "exp.unisolate":
			return "p.Unisolate(" + a[0] + ")"
		case "exp.farthest":
			return "p.Farthest"
		case "exp.recording":
			return "p.Expectations"
		case "exp.reset":
			return fmt.Sprintf("p.Farthest = %s\np.Exp = p.Exp[:p.ExpBase]\np.ExpBits = 0", a[0])
		case "exp.fresh":
			return "len(p.Exp) == p.ExpBase"
		case "exp.placeholder":
			return "p.Exp = append(p.Exp, ExpFar)"
		case "exp.maybe":
			return fmt.Sprintf("p.ExpBits&expBit(%s) != 0", a[0])
		case "exp.count":
			return "len(p.Exp) - p.ExpBase"
		case "exp.at":
			return "p.Exp[p.ExpBase+" + a[0] + "]"
		case "exp.add":
			return fmt.Sprintf("p.ExpBits |= expBit(%s)\np.Exp = append(p.Exp, %[1]s)", a[0])
		case "exp.setCount", "exp.keptCount":
			return "len(" + a[0] + ")"
		case "exp.setAt", "exp.keptAt":
			return a[0] + "[" + a[1] + "]"
		case "exp.empty":
			return "nil"
		case "exp.last":
			return "p.LastKept"
		case "memo.far":
			return a[0] + ".Far"
		case "memo.expected":
			return a[0] + ".Expected"
		case "memo.ok":
			return a[0] + ".Ok"
		case "memo.end":
			return a[0] + ".End"
		case "memo.silent":
			return a[0] + ".silent"
		case "memo.errs":
			return a[0] + ".Errs"
		case "memo.value":
			return a[0] + ".Node"
		case "memo.examined":
			return a[0] + ".Examined"
		case "memo.before":
			return a[0] + ".From < " + a[0] + ".Pos"
		case "memo.setPresent":
			return fmt.Sprintf("%s.Present = %s.present", a[0], a[1])
		case "memo.setExpected":
			return fmt.Sprintf("%s.Far, %[1]s.Expected = %s, %s", a[0], a[1], a[2])
		case "memo.pin":
			return a[0] + ".Positional = true"
		case "memo.seed":
			return a[0] + ".Growing"
		case "memo.from":
			return a[0] + ".From"
		case "memo.pinned":
			return a[0] + ".Positional"
		case "memo.shifted":
			return a[0] + ".shifted"
		case "memo.applied":
			return "int(" + a[0] + ".Vgen)"
		case "memo.advance":
			return fmt.Sprintf("%[1]s.Pos += %[4]s\n%[1]s.End += %[4]s\n%[1]s.From, %[1]s.Examined = %[2]s, %[3]s\n%[1]s.Far += %[4]s\n%[1]s.shift += int32(%[4]s)\n%[1]s.shifted = %[5]s\n%[1]s.Vgen = uint32(%[6]s)",
				a[0], a[1], a[2], a[3], a[4], a[5])
		case "edit.count":
			return "len(" + a[0] + ")"
		case "edit.start":
			return a[0] + "[" + a[1] + "].Start"
		case "edit.end":
			return a[0] + "[" + a[1] + "].End"
		case "edit.delta":
			return a[0] + "[" + a[1] + "].Delta"
		case "doc.touch":
			return "p.touch(" + a[0] + ")"
		case "doc.touchFrom":
			return "p.lw = Min2(p.lw, " + a[0] + ")"
		case "doc.low":
			return "p.lw"
		case "doc.high":
			return "p.Hw"
		case "doc.reused":
			return "p.Stats.Reused++"
		case "doc.evaluated":
			return "p.Stats.Evaluated++"
		case "doc.leave":
			// The binding level of a call is named min: the low end is lowered without the builtin.
			return fmt.Sprintf("p.Hw = max(%s, p.Hw)\np.lw = Min2(%s, p.lw)", a[0], a[1])
		case "ctx.found":
			return a[0] + ".e != nil"
		case "ctx.ok":
			return a[0] + ".e.Ok"
		case "ctx.value":
			return a[0] + ".e.Node"
		case "ctx.replay":
			return fmt.Sprintf("p.ctxReplay(%s, %s)", a[0], a[1])
		case "ctx.allows":
			return fmt.Sprintf("(!%s.ctx || p.ctxAllows(&%[1]s, %s))", a[0], a[1])
		case "ctx.active":
			return a[0] + ".ctx"
		case "ctx.lookup":
			return fmt.Sprintf("p.ctxLookup(&%s, %s, %s, %s)", a[0], a[1], a[2], a[3])
		case "ctx.push":
			return fmt.Sprintf("if %s.framed {\np.pushFrame(p.Pos, %[1]s.gp)\n}", a[0])
		case "ctx.done":
			return fmt.Sprintf("if %s.framed {\np.mergeDeps(%[1]s.ds)\n}", a[0])
		case "ctx.storeMemo":
			return "storeMemo"
		case "ctx.storeCtx":
			return "storeCtx"
		case "grow.begin":
			return fmt.Sprintf("p.GrowBegin(p.GrowKey(%s, %s), %[1]s.LeaderBit, %[1]s.Cycle)", a[0], a[1])
		case "grow.next":
			return "p.GrowNext(&" + a[0] + ")"
		case "grow.step":
			return fmt.Sprintf("p.GrowStep(&%s, %s, %s)", a[0], a[1], a[2])
		case "grow.end":
			return "p.growEnd(r, &" + a[0] + ")"
		}
		panic("core: no engine implementation of " + c.Callee)
	}
	t.Negate = func(c *exec.Call, a []string) (string, bool) {
		switch c.Callee {
		case "opr.found":
			return a[0] + ".Op == nil", true
		case "opr.hasSkip":
			return a[0] + ".OperatorsRule.Skip == nil", true
		case "opr.infix":
			return a[0] + ".Kind != grammar.Infix", true
		case "rec.none":
			return "!" + a[0] + ".None()", true
		case "memo.pinned":
			return "!" + a[0] + ".Positional", true
		}
		return "", false
	}
	t.Stmt = func(c *exec.Call, a []string, dst []string, define bool) (string, bool) {
		op := " = "
		if define {
			op = " := "
		}
		if c.Callee == "opr.skip" {
			return fmt.Sprintf("_, %s%s%s.OperatorsRule.Skip(p)", dst[0], op, a[0]), true
		}
		switch c.Callee {
		case "memo.lookup":
			e, found := dst[0], dst[1]
			if found == "_" {
				panic("core: the engine's memo.lookup needs its found result")
			}
			// A Document applies its edits to an entry when the entry is found (AdvanceEntry).
			return fmt.Sprintf("%s, %s%sp.MemoGet(%s)\nif %[2]s && %[1]s.Vgen != p.Gen && !p.AdvanceEntry(%[1]s, p.Edits[:p.Gen]) {\n%[2]s = false\n}",
				e, found, op, key(a, a[4])), true
		case "ctx.enter":
			// The context of a call of a rule of no cycle, or outside every growth, is empty.
			cs, seed := dst[0], dst[1]
			s := ""
			if define {
				s = fmt.Sprintf("var %s CtxState\nvar %s ctxHit\n", cs, seed)
			}
			return s + fmt.Sprintf("if %[1]s.Cycle != 0 && len(p.Gamma) != 0 {\n%[2]s = p.ctxEnter(&%[3]s, %[1]s, %[4]s)\n}", a[0], seed, cs, a[1]), true
		case "env.key":
			// The values of the variables are read only for a rule that reads some (Parser.keyVals).
			return fmt.Sprintf("%s%sKeyVals{Missing: p.Missing != nil && %s.Missing && p.Silent > 0}\nif len(%[3]s.Vars) > 0 {\n%[1]s.Env = p.memoEnv(%[3]s)\n}", dst[0], op, a[0]), true
		case "doc.enter":
			return fmt.Sprintf("%s, %s%sp.Hw, p.lw\np.Hw, p.lw = %s, %[4]s", dst[0], dst[1], op, a[0]), true
		case "doc.move":
			if len(dst) != 0 && dst[0] != "_" {
				panic("core: the engine moves every value; doc.move has no result to read")
			}
			return fmt.Sprintf("if %s.shifted {\np.moveResult(%[1]s)\n}", a[0]), true
		case "ctx.pop":
			return fmt.Sprintf("%s%sstoreMemo\nif %s.ctx {\n%[1]s = p.ctxPop(&%[3]s)\n}", dst[0], op, a[0]), true
		case "memo.store":
			e := dst[0]
			decl := ""
			if define {
				decl = "var " + e + " *MemoEntry\n"
			}
			return fmt.Sprintf(`%sif r.Leader {
%s, p.grown = p.grown, nil
} else {
%[2]s = p.entry(c.ctx)
*%[2]s = MemoEntry{Node: %s, Ok: %s, End: %s, silent: %s, Gen: p.Gen, Vgen: p.Gen, Errs: %s}
}
p.MemoPut(%s, %[2]s)`, decl, e, a[7], a[4], a[5], a[6], a[8], key(a, a[9])), true
		case "exp.copy":
			// The kept sets fill append-only chunks of the arena.
			return fmt.Sprintf(`if cap(p.Arena)-len(p.Arena) < len(%[1]s) {
// A new chunk; the previous one lives while sets in it remain.
p.Arena = make([]ExpID, 0, max(1024, len(%[1]s)))
p.MemUsed += uint64(cap(p.Arena)) * expIDBytes
}
off := len(p.Arena)
p.Arena = AppendSpend(p, p.Arena, %[1]s)
p.LastKept = p.Arena[off:len(p.Arena):len(p.Arena)]
%[2]s%[3]sp.LastKept`, a[0], dst[0], op), true
		case "memo.examine":
			// The engine records the start of the examined input, the low end that before compares
			// with the position: a call examines at most the unit before its position, so the start
			// is the position or the unit before it (TestDocumentEntriesExamineFromTheirPosition).
			return fmt.Sprintf("%s.Examined, %[1]s.From, %[1]s.Positional = %s, p.lw, false", a[0], a[2]), true
		case "ctx.keep":
			// c, r, pos, min, key, ok, end, silent, value, errs, far, kept
			return fmt.Sprintf(`e := p.grown
if r.Leader {
p.grown = nil
} else {
e = new(MemoEntry)
*e = MemoEntry{Node: %s, Ok: %s, End: %s, Examined: p.Hw, From: p.lw, silent: %s, Gen: p.Gen, Vgen: p.Gen, Positional: r.Positional, Errs: %s}
}
e.Far, e.Expected = %s, %s
p.ctxKeep(&%s, %s, %s, %s, %s, e)`, a[8], a[5], a[6], a[7], a[9], a[10], a[11], a[0], a[1], a[2], a[3], a[4]), true
		}
		return "", false
	}
	return t
}
