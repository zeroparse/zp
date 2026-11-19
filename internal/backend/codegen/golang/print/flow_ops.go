package print

import (
	"fmt"
	"strconv"
	"strings"

	coreimpl "zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/execlower"
	rprogram "zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// The Go forms of the primitive operations of grammar code at the Exec stage. Each primitive stands
// for a Flow operation (execlower.FlowOp), whose name selects its form; the immediate holds its
// static data and args its printed arguments. A primitive without results prints as a statement,
// without its final newline.

// op returns the Go text of the call c of a primitive operation with the printed arguments args.
func (p *flowPrinter) op(c *code.Call, args []string) string {
	g := p.g
	switch name := execlower.FlowOp(c.Callee); name {
	// Statements.
	case flow.OpStep:
		return "p.Spend()"
	case flow.OpSilent:
		s := c.Imm.(flow.Silence)
		if s.Delta > 0 {
			if s.Site != 0 {
				return fmt.Sprintf("p.Silent++\np.LookBegin(%d)", s.Site)
			}
			return "p.Silent++"
		}
		if s.Site != 0 {
			return fmt.Sprintf("p.LookEnd(%t)\np.Silent--", s.Ok)
		}
		return "p.Silent--"
	case flow.OpAdvance:
		if n, ok := c.Args[0].(*code.Const); ok {
			return advance(int(n.Value.(int64)))
		}
		return "p.Pos += " + args[0]
	case flow.OpExpect:
		x := c.Imm.(flow.Expectation)
		if x.Message {
			return fmt.Sprintf("p.%s(%s, MsgBit|%d)", p.expectHelper(), args[0], p.desc(x.Exp))
		}
		return fmt.Sprintf("p.%s(%s, %s)", p.expectHelper(), args[0], p.exp(x.Exp))
	case flow.OpRestore:
		return fmt.Sprintf("p.Restore(%s, %s)", args[0], args[1])
	case flow.OpMergeExpected:
		return fmt.Sprintf("p.MergeExpected(%s, %s)", args[0], args[1])
	case flow.OpMergeRenamed:
		return fmt.Sprintf("p.MergeExpected(%s, RenamedExpected(%s, %s, %d))", args[0], p.matcher(), args[1], p.desc(c.Imm.(core.ExpID)))
	case flow.OpPushKid:
		if p.frame {
			return fmt.Sprintf("p.KidStack = append(p.KidStack, %s)", args[0])
		}
		return fmt.Sprintf("p.PushKid(%s)", args[0])
	case flow.OpStreamStart:
		return "p.StreamStart()"
	case flow.OpSkipBegin:
		return fmt.Sprintf("p.SkipBegin(%d)", c.Imm.(flow.Site))
	case flow.OpRunBegin:
		w := c.Imm.(flow.RunWalk)
		return fmt.Sprintf("p.RunBegin(%d, %t, %t)", w.Site, w.Build, w.KeyEnv)
	case flow.OpSkipVisit:
		return "p.SkipVisit()"
	case flow.OpSkipValue:
		return fmt.Sprintf("p.SkipValue(%s)", args[0])
	case flow.OpSkipEnd:
		return "p.SkipEnd()"
	case flow.OpRunWalked:
		// A walk shorter than RunHotSpan changes nothing, so the call is left out.
		return fmt.Sprintf("if p.Pos-%s >= RunHotSpan {\np.RunWalked(%d, %s)\n}", args[0], c.Imm.(flow.Site), args[0])
	case flow.OpScanEnd:
		return fmt.Sprintf("p.ScanEnd(%d, %s, %s)", c.Imm.(flow.Site), args[0], args[1])
	case flow.OpEmitStream:
		return fmt.Sprintf("p.EmitStream(%s, p.Pos)", args[0])
	case flow.OpDropKids:
		return fmt.Sprintf("p.DropKids(%s)", args[0])
	case flow.OpSetWhere:
		return fmt.Sprintf("p.Where = %q", string(c.Imm.(flow.Chars)))
	case flow.OpDropCreated:
		return fmt.Sprintf("p.DropCreated(%s.Cbase)", args[0])
	case flow.OpUnfresh:
		return fmt.Sprintf("if n := AsTval(%s); n != nil {\nn.TSetFresh(false)\n}", args[0])
	case flow.OpReset:
		return fmt.Sprintf("p.Reset(%s)", args[0])
	case flow.OpSetCapture:
		return fmt.Sprintf("p.%s(%d, %s)", g.helper("SetCapture"), c.Imm.(flow.Slot), args[0])
	case flow.OpClearVals:
		return fmt.Sprintf("clear(%s.Vals)", args[0])
	case flow.OpDropTrail:
		return fmt.Sprintf("p.DropTrail(%s.Trail)", args[0])
	case flow.OpFreeFrame:
		return fmt.Sprintf("p.FreeFrame(%s)", args[0])
	case flow.OpAppendError:
		return fmt.Sprintf("p.PushRecovered(%s)", args[0])
	case flow.OpDeclareNames:
		// Declared whatever the reachability.
		n := c.Imm.(*flow.Names)
		p.scopeNames(n)
		p.declare(n)
		return ""
	case flow.OpObserveSkip:
		// The alternative's expression begins with the call it skips, if any.
		return strings.TrimSuffix(strings.TrimPrefix(g.traceSkip(g.Flow.Origin[c.Imm.(core.Node)]), "\n"), "\n")
	case flow.OpSetEnv:
		return "p.Env = " + args[0]
	case flow.OpSetFrame:
		return "p.Frame = " + args[0]
	case flow.OpAppend:
		return p.appendText(c, args)

	// Expressions.
	case flow.OpEnv:
		return "p.Env"
	case flow.OpFrame:
		return "p.Frame"
	case flow.OpRecLen:
		return "len(p.Recovered)"
	case flow.OpKidsLen:
		return "len(p.KidStack)"
	case flow.OpStreamOn:
		return fmt.Sprintf("p.StreamsIn(%d, %v)", g.Cur.ID, g.Cur.Rec)
	case flow.OpSkipHit:
		return "p.SkipHit()"
	case flow.OpRunHot:
		return fmt.Sprintf("p.RunHot(%d)", c.Imm.(flow.Site))
	case flow.OpSkipKids:
		return "p.SkipKids()"
	case flow.OpScanReach:
		return fmt.Sprintf("p.ScanReach(%d, %s, %s)", c.Imm.(flow.Site), args[0], args[1])
	case flow.OpScanKids:
		return fmt.Sprintf("p.ScanKids(%d, %s, %s, %s)", c.Imm.(flow.Site), args[0], args[1], args[2])
	case flow.OpHasLit:
		return p.hasLit(c.Imm.(*core.Lit))
	case flow.OpMatchLit:
		lit := c.Imm.(*core.Lit)
		if lit.Fold {
			return fmt.Sprintf("%s.%s(%s, %d, false)", p.matcher(), p.helper("MatchLiteralFold"), g.literal(string(common.FoldedRunes([]rune(lit.Text)))), p.desc(lit.Exp))
		}
		name := g.literal(lit.Text)
		var desc rprogram.ExpID
		if lit.Text == "" {
			desc = g.Desc(strconv.Quote(""))
		} else {
			desc = p.desc(lit.Exp)
		}
		return fmt.Sprintf("%s.%s(%s, %q, %d, false)", p.matcher(), p.helper("MatchLiteral"), name, lit.Text, desc)
	case flow.OpGuardFails:
		guard := c.Imm.(*core.First)
		var cond string
		if guard.Lit {
			cond = fmt.Sprintf("ch == %d", guard.Rune)
			if guard.Fold {
				cond = fmt.Sprintf("ch == %d || ch == %d", guard.Rune, guard.Rune-('a'-'A'))
			}
		} else {
			cond = NegateCondition(g.ControlledClassReject(grammarClass(guard.Class)))
		}
		return fmt.Sprintf("!(%s && (%s)) && %s", args[1], chIdent.ReplaceAllString(cond, args[0]), depthLimit(guard.Depth))
	case flow.OpCallRule:
		x := c.Imm.(flow.RuleCall)
		return g.ruleCall(g.Flow.Rules[x.Rule], x.Level)
	case flow.OpIsolate:
		return "p.Isolate(p.Pos)"
	case flow.OpUnisolate:
		return fmt.Sprintf("p.Unisolate(%s)", args[0])
	case flow.OpText:
		return fmt.Sprintf("p.Text(%s, p.Pos)", args[0])
	case flow.OpLiteralText:
		return fmt.Sprintf("p.LiteralText(%s, %q)", args[0], string(c.Imm.(flow.Chars)))
	case flow.OpStr:
		return strconv.Quote(string(c.Imm.(flow.Chars)))
	case flow.OpNewMatch:
		return fmt.Sprintf("p.%s(%s, p.Pos, %s, true)", g.helper("NewMatch"), args[0], args[1])
	case flow.OpNewTerminalNode:
		return fmt.Sprintf("p.%s(%s, %s)", g.helper("NewTerminal"), args[0], args[1])
	case flow.OpNewBranchNode:
		kind := "SeqKind"
		if c.Imm.(flow.List) {
			kind = "ListKind"
		}
		return fmt.Sprintf("p.%s(%s, %s, %s)", g.helper("NewBranch"), kind, args[0], args[1])
	case flow.OpNewNode:
		typ := "tuple"
		if c.Imm.(flow.List) {
			typ = coreimpl.TypeList
		}
		return fmt.Sprintf("p.%s(%q, %s, p.Pos, %s)", g.helper("NewNode"), typ, args[0], args[1])
	case flow.OpNewVals:
		if p.untyped() {
			return fmt.Sprintf("p.Nodes(%d)[:0]", c.Imm.(flow.Room))
		}
		return fmt.Sprintf("p.NewVals(%d)[:0]", c.Imm.(flow.Room))
	case flow.OpKids:
		return fmt.Sprintf("p.Kids(%s)", args[0])
	case flow.OpShapeSingle:
		return fmt.Sprintf("p.ShapeSingle(%s, %s, %s)", p.shaper(c.Imm.(core.Shape)), args[0], args[1])
	case flow.OpShapeCapture:
		return fmt.Sprintf("p.ShapeCapture(%s, %s, %s)", p.shaper(c.Imm.(core.Shape)), args[0], args[1])
	case flow.OpScopeValue:
		return fmt.Sprintf("p.ScopeValue(%s, %s, %s)", p.shaper(c.Imm.(core.Shape)), args[0], args[1])
	case flow.OpAttachFrame:
		names := p.scopeNames(c.Imm.(flow.Attach).Names)
		if p.untyped() {
			// The Node library's AttachCaptures takes the frame.
			return fmt.Sprintf("p.AttachCaptures(%s, %s, %s, %s, p.Pos)", args[0], names, args[2], args[1])
		}
		return fmt.Sprintf("p.AttachCaptures(%s, %s, %s.Vals, %s, p.Pos)", args[0], names, args[2], args[1])
	case flow.OpAttachCaptures:
		names := p.scopeNames(c.Imm.(flow.Attach).Names)
		return fmt.Sprintf("p.AttachCaptures(%s, %s, []any{%s}, %s, p.Pos)", args[0], names, strings.Join(args[2:], ", "), args[1])
	case flow.OpMarkState:
		return "p.Mark()"
	case flow.OpMarkPos:
		return args[0] + ".Pos"
	case flow.OpNewFrame:
		return fmt.Sprintf("p.NewFrame(len(%s))", p.scopeNames(c.Imm.(*flow.Names)))
	case flow.OpFrameVal:
		return fmt.Sprintf("%s.Vals[%d]", args[0], c.Imm.(flow.Slot))
	case flow.OpKeep:
		return fmt.Sprintf("p.Keep(%s)", args[0])
	case flow.OpMakeError:
		return fmt.Sprintf("p.MakeError(%s, %s)", args[0], args[1])
	case flow.OpErrorValue:
		return fmt.Sprintf("NewTError(%s, p.Pos, p.RecoveryErrorText(%s))", args[0], args[1])
	case flow.OpNewErrorNode:
		return fmt.Sprintf("p.NewErrorNode(%s, %s)", args[0], args[1])
	case flow.OpFreshNode:
		return fmt.Sprintf("TFreshNode(%s)", args[0])
	case flow.OpConvertResult:
		x := c.Imm.(flow.Conversion)
		if x.Owned {
			return fmt.Sprintf("p.TConvertResultOwned(trules[%d], %s, %s)", x.Rule, args[0], args[1])
		}
		return fmt.Sprintf("p.TConvertResult(trules[%d], %s)", x.Rule, args[0])
	case flow.OpUseCtx:
		return fmt.Sprintf("p.UseCtx(TActCtx{P: p, Start: %s, End: p.Pos, Cbase: len(p.Created)})", args[0])
	case flow.OpEval:
		x := c.Imm.(flow.Action)
		defer p.caps(x.Names, args[1:])()
		stmts, res := g.loweredStmts(x.Term, check.NewScope(), nil)
		p.pre += stmts
		return res
	case flow.OpCtxFinish:
		return fmt.Sprintf("%s.Finish(%s)", args[0], args[1])
	default:
		panic(fmt.Sprintf("flow printer: operation %s", c.Callee))
	}
}

// appendText returns the statement of flow.append, which assigns the list it appends to.
func (p *flowPrinter) appendText(c *code.Call, args []string) string {
	vals, x := args[0], args[1]
	spread := bool(c.Imm.(flow.Spread))
	if spread && p.untyped() {
		x = "D(" + x + ").Children..."
	} else if spread {
		x = fmt.Sprintf("AsTval(%s).TKids()...", x)
	}
	return fmt.Sprintf("%s = append(%s, %s)", vals, vals, x)
}

// hasLit returns the condition that the input at the position begins with the literal, read in place.
func (p *flowPrinter) hasLit(lit *core.Lit) string {
	g := p.g
	if g.Traced {
		panic("internal error: an in-place read of a literal in a traced body")
	}
	rs := []rune(lit.Text)
	cond := fmt.Sprintf("p.Pos+%d <= len(p.In)", len(rs))
	if len(rs) == 1 {
		cond = "p.Pos < len(p.In)"
	}
	for i, r := range rs {
		if i == 0 {
			cond += fmt.Sprintf(" && p.In[p.Pos] == %d", r)
		} else {
			cond += fmt.Sprintf(" && p.In[p.Pos+%d] == %d", i, r)
		}
	}
	if p.document {
		if g.NoBudget {
			return "p.Document == nil && (" + cond + ")"
		}
		return "p.Document == nil && (" + cond + ") && p.Control.Take(" + strconv.Itoa(len(rs)) + ")"
	}
	if p.untyped() && !g.Flow.Opts.Budget {
		// The Node library's generated code of a parser without budget controls has no such check.
		return cond
	}
	if g.Ordinary {
		return cond
	}
	if g.Flow.Opts.Budget {
		// The literal is charged one unit per code point, as MatchLiteral does when every one matches.
		return "(" + cond + ") && p.Control.Take(" + strconv.Itoa(len(rs)) + ")"
	}
	return "p.Control == nil && (" + cond + ")"
}

// negate returns the negation of the call c of a primitive operation with the printed arguments args.
func (p *flowPrinter) negate(c *code.Call, args []string) string {
	g := p.g
	switch execlower.FlowOp(c.Callee) {
	case flow.OpAtAnchor:
		switch c.Imm.(core.AnchorKind) {
		case core.BeginInput:
			return "p.Pos != 0"
		case core.EndInput:
			return "!p." + g.atEnd() + "()"
		case core.BeginLine:
			return "!p." + g.atLineStart() + "()"
		default:
			return "!p." + g.helper("AtLineEnd") + "()"
		}
	case flow.OpMatchClass:
		class := c.Imm.(*core.Class)
		table := g.classTable(grammarClass(class))
		return fmt.Sprintf("!p.ClassFast(&%s) && !p.ClassSkip(&%s, %d)", table, table, p.desc(class.Exp))
	case flow.OpInClass:
		return p.reject(c.Imm.(*core.Class), args[0])
	case flow.OpSkipTerminal:
		table, ok := g.terminalTable(g.Flow.Origin[c.Imm.(*core.Terminal).Body])
		if !ok {
			panic("flow printer: terminal region without a terminal program")
		}
		return fmt.Sprintf("!p.SkipTerminal(%s)", table)
	case flow.OpSkipClasses:
		run := c.Imm.(*core.ClassRun)
		items := make([]ast.Expr, len(run.Classes))
		for i, cl := range run.Classes {
			items[i] = grammarClass(cl)
		}
		return fmt.Sprintf("!p.ClassesSkip(%s)", g.classSteps(items))
	case flow.OpPred:
		x := c.Imm.(flow.Predicate)
		if x.Frame {
			// The term reads the current frame.
			return fmt.Sprintf("!p.Predicate(func(c *%s) (any, any) { %s })", g.ctxType(), g.predicate(x.Node.Term, common.FrameScope(x.Names)))
		}
		defer p.caps(x.Names, args)()
		return fmt.Sprintf("!p.Predicate(func(c *TActCtx) (any, any) { %s })", g.predicate(x.Node.Term, check.NewScope()))
	case flow.OpBind:
		x := c.Imm.(flow.Binding)
		b := p.term(x.Node).(*ast.Bind)
		if x.Frame {
			return fmt.Sprintf("!p.Bind(%d, func(c *%s) (any, any) { %s })", g.Prog.Vars.Index(b.Name), g.ctxType(), g.actionBody(x.Node.Term, common.FrameScope(x.Names), nil))
		}
		defer p.caps(x.Names, args)()
		return fmt.Sprintf("!p.Bind(%d, func(c *TActCtx) (any, any) { %s })", g.Prog.Vars.Index(b.Name), g.actionBody(x.Node.Term, check.NewScope(), nil))
	}
	return "!" + p.op(c, args)
}

// caps sets the capture variables of the generator's terms to the captures of the slots names, whose
// printed arguments are args (nil for a slot the term does not read), and returns the function that
// restores them.
func (p *flowPrinter) caps(names, args []string) func() {
	caps := map[string]string{}
	for i, a := range args {
		if a != "nil" {
			caps[names[i]] = a
		}
	}
	prev := p.g.caps
	p.g.caps = caps
	return func() { p.g.caps = prev }
}
