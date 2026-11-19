package print

import (
	"fmt"
	"strconv"
	"strings"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	tspass "zeroparse.dev/zp/internal/backend/codegen/typescript/pass"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/execlower"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/internal/runtime/nodes"
	rprogram "zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// The TypeScript forms of the primitive operations of grammar code at the Exec stage. Each primitive
// stands for a Flow operation (execlower.FlowOp), whose name selects its form; the immediate holds
// its static data and the arguments its operands.

// tsFlowPrinter prints the primitives of a function of grammar code at the Exec stage.
type tsFlowPrinter struct {
	g *Generator
	b strings.Builder
	// dead reports that the current point is unreachable.
	dead bool
	// reads holds the locals and the parts of marks read at reachable points, by the pass before (nil
	// before the first pass: all are read).
	reads map[string]bool
	// next collects the locals and the parts of marks that the pass being printed reads.
	next map[string]bool
	// dry reports a pass that only collects reads: it writes no declarations.
	dry    bool
	labels map[string]string
	// decls holds the constants the printed text names, which the caller declares if it keeps the text.
	decls []tsDecl
	names map[*flow.Names]string
	// marks holds the marks of the whole parser state by local, with the parts they save.
	marks map[string]flow.StateMark
	// outlined holds the calls of the functions printed for the outlined nodes.
	outlined map[*code.Call]string
	// parts holds, for each mark local, the names that stand for its position, environment, trail
	// length and number of recovered errors, so that a mark is declared as far as it is read.
	parts map[string]*[4]string
}

// part returns the name that stands for the component i of the mark m.
func (p *tsFlowPrinter) part(m string, i int) string {
	ps := p.parts[m]
	if ps == nil {
		ps = &[4]string{}
		for j := range ps {
			ps[j] = m + tsMarkParts[j]
		}
		p.parts[m] = ps
	}
	return ps[i]
}

// label returns the TypeScript label of the label name: L_ and its name, or for a label that the
// lowering left unnamed, L and its number.
func (p *tsFlowPrinter) label(name string) string {
	if n, ok := p.labels[name]; ok {
		return n
	}
	n := "L" + strconv.Itoa(len(p.labels)+1)
	if !flow.Unnamed(name) {
		n = "L_" + name
	}
	for _, other := range p.labels {
		if other == n {
			n += "_" + strconv.Itoa(len(p.labels)+1)
		}
	}
	p.labels[name] = n
	return n
}

// read records the locals that the expressions read at a reachable point.
func (p *tsFlowPrinter) read(xs ...code.Expr) {
	if p.dead {
		return
	}
	for _, x := range xs {
		code.Walk(&code.Block{Stmts: []code.Stmt{&code.Assert{Cond: x}}}, nil, func(e code.Expr) {
			if l, ok := e.(*code.Local); ok {
				p.next[l.Name] = true
			}
		})
	}
}

// readName records that the local or the part of a mark name is read at a reachable point.
func (p *tsFlowPrinter) readName(name string) {
	if !p.dead {
		p.next[name] = true
	}
}

// isRead reports whether the local name is read, by the previous pass.
func (p *tsFlowPrinter) isRead(name string) bool {
	return name != "" && (p.reads == nil || p.reads[name])
}

// tsFlowPure reports whether evaluating the call c of a primitive has no effect, so that an
// assignment of it to nothing read is left out.
func tsFlowPure(c *code.Call) bool {
	switch execlower.FlowOp(c.Callee) {
	case flow.OpEnv, flow.OpFrame, flow.OpRecLen, flow.OpKidsLen, flow.OpNewVals, flow.OpStr, flow.OpMarkPos,
		flow.OpFrameVal, flow.OpInClass, flow.OpGuardFails:
		return true
	}
	return false
}

// tsPure reports whether evaluating x has no effect.
func tsPure(x code.Expr) bool { return Pure(x, tsFlowPure) }

// stmtOp writes the call c of a primitive without results, whose printed arguments args returns.
func (p *tsFlowPrinter) stmtOp(c *code.Call, args func() []string) {
	switch execlower.FlowOp(c.Callee) {
	case flow.OpStep:
		p.write(tsSpend(1))
	case flow.OpSilent:
		s := c.Imm.(flow.Silence)
		if s.Delta > 0 {
			p.write("p.silent++;\n")
			if s.Site != 0 {
				p.write(fmt.Sprintf("p.lookBegin(%d);\n", s.Site))
			}
		} else {
			if s.Site != 0 {
				p.write(fmt.Sprintf("p.lookEnd(%t);\n", s.Ok))
			}
			p.write("p.silent--;\n")
		}
	case flow.OpAdvance:
		p.read(c.Args...)
		p.write("p.pos += " + args()[0] + ";\n")
	case flow.OpExpect:
		p.read(c.Args...)
		x := c.Imm.(flow.Expectation)
		if x.Message {
			p.write(fmt.Sprintf("p.expect(%s, msgBit | %d);\n", args()[0], p.desc(x.Exp)))
		} else {
			p.write(fmt.Sprintf("p.expect(%s, %s);\n", args()[0], p.exp(x.Exp)))
		}
	case flow.OpRestore:
		p.read(c.Args...)
		a := args()
		p.write(fmt.Sprintf("p.pos = %s;\np.recovered.length = %s;\n", a[0], a[1]))
	case flow.OpMergeExpected:
		p.read(c.Args...)
		a := args()
		p.write(fmt.Sprintf("p.mergeKept(%s, %s);\n", a[0], a[1]))
	case flow.OpMergeRenamed:
		p.read(c.Args...)
		a := args()
		p.write(fmt.Sprintf("p.mergeKept(%s, p.renameExpected(%s, %d));\n", a[0], a[1], p.desc(c.Imm.(core.ExpID))))
	case flow.OpAppendError:
		p.read(c.Args...)
		p.write(fmt.Sprintf("p.recovered.push(%s!);\n", args()[0]))
	case flow.OpPushKid:
		p.read(c.Args...)
		p.write(fmt.Sprintf("p.kidStack.push(%s);\n", args()[0]))
	case flow.OpDropKids:
		p.read(c.Args...)
		p.write(fmt.Sprintf("p.dropKids(%s);\n", args()[0]))
	case flow.OpAppend:
		p.read(c.Args...)
		a := args()
		if c.Imm.(flow.Spread) {
			p.write(fmt.Sprintf("for (const k of (%s as Node).children) {\n  %s.push(k);\n}\n", a[1], a[0]))
		} else {
			p.write(fmt.Sprintf("%s.push(%s);\n", a[0], a[1]))
		}
	case flow.OpSetCapture:
		p.read(c.Args...)
		p.write(fmt.Sprintf("p.setCapture(%d, %s);\n", c.Imm.(flow.Slot), args()[0]))
	case flow.OpClearVals:
		p.read(c.Args...)
		p.write(args()[0] + ".fill(null);\n")
	case flow.OpFreeFrame:
		// Frames are not pooled.
	case flow.OpDeclareNames:
		// The names are declared where they are first used.
	case flow.OpReset:
		p.write(p.reset(p.markLocal(c)))
	case flow.OpDropTrail:
		m := p.markLocal(c)
		if !p.marks[m].Trail {
			panic(tsFlowUnsupported("a mark without a trail dropped"))
		}
		p.readName(p.part(m, 2))
		p.write(fmt.Sprintf("p.trail.length = %sTrail;\n", m))
	case flow.OpSkipBegin:
		p.write(fmt.Sprintf("p.skipBegin(%d);\n", c.Imm.(flow.Site)))
	case flow.OpRunBegin:
		w := c.Imm.(flow.RunWalk)
		key := "null"
		if w.KeyEnv {
			key = p.g.runKeyName(w.Site)
		}
		p.write(fmt.Sprintf("p.runBegin(%d, %t, %s);\n", w.Site, w.Build, key))
	case flow.OpSkipVisit:
		p.write("p.skipVisit();\n")
	case flow.OpSkipValue:
		p.write(fmt.Sprintf("p.skipValue(%s);\n", args()[0]))
	case flow.OpSkipEnd:
		p.write("p.skipEnd();\n")
	case flow.OpRunWalked:
		p.write(fmt.Sprintf("p.runWalked(%d, %s);\n", c.Imm.(flow.Site), args()[0]))
	case flow.OpScanEnd:
		a := args()
		p.write(fmt.Sprintf("p.scanEnd(%d, %s, %s);\n", c.Imm.(flow.Site), a[0], a[1]))
	case flow.OpObserveSkip:
		if p.dead || p.dry {
			return
		}
		trace := p.g.traceSkipped(p.g.Flow.Origin[c.Imm.(core.Node)])
		p.b.WriteString(strings.TrimLeft(trace, " "))
	default:
		panic(tsFlowUnsupported("operation " + c.Callee))
	}
}

// expr returns the TypeScript expression of the call c of a primitive with results, whose printed
// arguments args returns.
func (p *tsFlowPrinter) expr(c *code.Call, args func() []string) string {
	g := p.g
	switch execlower.FlowOp(c.Callee) {
	case flow.OpEnv:
		return "p.env"
	case flow.OpFrame:
		return "p.frame"
	case flow.OpRecLen:
		return "p.recovered.length"
	case flow.OpKidsLen:
		return "p.kidStack.length"
	case flow.OpSkipHit:
		return "p.skipHit()"
	case flow.OpRunHot:
		return fmt.Sprintf("p.runHot(%d)", c.Imm.(flow.Site))
	case flow.OpSkipKids:
		return "p.skipKids()"
	case flow.OpScanReach:
		a := args()
		return fmt.Sprintf("p.scanReach(%d, %s, %s)", c.Imm.(flow.Site), a[0], a[1])
	case flow.OpScanKids:
		a := args()
		return fmt.Sprintf("p.scanKids(%d, %s, %s, %s)", c.Imm.(flow.Site), a[0], a[1], a[2])
	case flow.OpMatchLit:
		lit := c.Imm.(*core.Lit)
		if lit.Fold {
			folded := string(common.FoldedRunes([]rune(lit.Text)))
			if p.dry {
				return ""
			}
			v, shared := g.LiteralName(folded)
			if !shared {
				p.decl(v, fmt.Sprintf("const %s: Lit = { text: %s, cps: %s, bytes: %s };\n", v, TsString(folded), TsInts([]rune(folded)), TsInts([]byte(folded))))
			}
			return fmt.Sprintf("p.matchLiteralFold(%s, %d, false)", v, p.desc(lit.Exp))
		}
		if p.dry {
			return ""
		}
		val := nodes.LiteralValue(lit.Text)
		v, shared := g.LiteralName(val)
		if !shared {
			p.decl(v, fmt.Sprintf("const %s: Lit = { text: %s, cps: %s, bytes: %s };\n", v, TsString(val), TsInts([]rune(val)), TsInts([]byte(val))))
		}
		var desc rprogram.ExpID
		if lit.Text == "" {
			desc = g.Desc(strconv.Quote(""))
		} else {
			desc = p.desc(lit.Exp)
		}
		return fmt.Sprintf("p.matchLiteral(%s, %d, false)", v, desc)
	case flow.OpGuardFails:
		guard := c.Imm.(*core.First)
		a := args()
		ch, more := a[0], a[1]
		var cond string
		if guard.Lit {
			cond = fmt.Sprintf("%s === %d", ch, guard.Rune)
			if guard.Fold {
				cond = fmt.Sprintf("%s === %d || %s === %d", ch, guard.Rune, ch, guard.Rune-('a'-'A'))
			}
		} else {
			cond = tsClassIn(guard.Class, ch)
		}
		depth := "p.depth <= p.maxDepth"
		if guard.Depth != 0 {
			depth = fmt.Sprintf("p.depth + %d <= p.maxDepth", guard.Depth)
		}
		return fmt.Sprintf("!(%s && (%s)) && %s", more, cond, depth)
	case flow.OpInClass:
		return tsClassIn(c.Imm.(*core.Class), args()[0])
	case flow.OpAtAnchor:
		switch c.Imm.(core.AnchorKind) {
		case core.BeginInput:
			return "p.pos === 0"
		case core.EndInput:
			return "p.atEnd()"
		case core.BeginLine:
			return "p.atLineStart()"
		default:
			return "p.atLineEnd()"
		}
	case flow.OpCallRule:
		x := c.Imm.(flow.RuleCall)
		return p.call(g.Flow.Rules[x.Rule], x.Level)
	case flow.OpIsolate:
		return "p.isolate()"
	case flow.OpKeep:
		// The expectations are a copy of the record already.
		return args()[0]
	case flow.OpMakeError:
		a := args()
		return fmt.Sprintf("p.recoveredError(%s, %s)", a[0], a[1])
	case flow.OpNewErrorNode:
		a := args()
		return p.counted(fmt.Sprintf("new Node(\"error\", \"\", %s, p.pos, \"\", noNodes, [{ name: \"message\", value: %s!.message }], true, true)", a[0], a[1]), "nodeBytes + arrayBytes(1) + objectBytes(2)")
	case flow.OpText, flow.OpLiteralText:
		// A literal's text is the input it matched: where the literal holds the replacement character,
		// that stands for invalid input.
		p.read(c.Args...)
		return fmt.Sprintf("p.text(%s, p.pos)", args()[0])
	case flow.OpStr:
		return TsString(string(c.Imm.(flow.Chars)))
	case flow.OpNewTerminalNode:
		a := args()
		return p.counted(fmt.Sprintf("new Node(\"string\", \"\", %s, p.pos, %s, noNodes, noFields, true, true)", a[0], a[1]), "nodeBytes")
	case flow.OpNewBranchNode:
		kind := "tuple"
		if c.Imm.(flow.List) {
			kind = "list"
		}
		a := args()
		return p.counted(fmt.Sprintf("new Node(\"%s\", \"\", %s, p.pos, \"\", %s, noFields, true, false)", kind, a[0], a[1]), "nodeBytes")
	case flow.OpNewVals:
		return "[]"
	case flow.OpKids:
		return fmt.Sprintf("p.kids(%s)", args()[0])
	case flow.OpShapeSingle:
		a := args()
		return fmt.Sprintf("p.shapeSingle(%d, %s, %s)", tsShape(c.Imm.(core.Shape)), a[0], a[1])
	case flow.OpShapeCapture:
		a := args()
		return fmt.Sprintf("p.shapeCapture(%d, %s, %s)", tsShape(c.Imm.(core.Shape)), a[0], a[1])
	case flow.OpScopeValue:
		a := args()
		return fmt.Sprintf("p.scopeValue(%d, %s, %s)", tsShape(c.Imm.(core.Shape)), a[0], a[1])
	case flow.OpAttachCaptures:
		panic(tsFlowUnsupported("captures attached from variables"))
	case flow.OpAttachFrame:
		a := args()
		return fmt.Sprintf("p.attachCaptures(%s, %s, %s, %s, p.pos)", a[0], p.scopeNames(c.Imm.(flow.Attach).Names), a[2], a[1])
	case flow.OpPred:
		x := c.Imm.(flow.Predicate)
		if !x.Frame {
			panic(tsFlowUnsupported("a predicate that reads variables"))
		}
		// The term reads the current frame.
		if p.dry {
			return "p.predicate(a)"
		}
		return fmt.Sprintf("p.predicate(%s)", p.g.predicateFunc(x.Node.Term, common.FrameScope(x.Names)))
	case flow.OpBind:
		x := c.Imm.(flow.Binding)
		if !x.Frame {
			panic(tsFlowUnsupported("a bind that reads variables"))
		}
		b, ok := p.grammarTerm(x.Node).(*ast.Bind)
		if !ok {
			panic(tsFlowUnsupported("a bind without a grammar bind"))
		}
		if p.dry {
			return "p.bind(a)"
		}
		// The function is declared only if the rule keeps the printed text.
		name := p.g.Name("a")
		p.decl(name, p.g.actionFunc(name, x.Node.Term, common.FrameScope(x.Names), nil))
		return fmt.Sprintf("p.bind(%d, %s)", p.g.Prog.Vars.Index(b.Name), name)
	case flow.OpCallOutline:
		return p.outline(c)
	case flow.OpMarkPos:
		m := p.markLocal(c)
		p.readName(p.part(m, 0))
		return m + "Pos"
	case flow.OpNewFrame:
		return fmt.Sprintf("p.newFrame(%d)", len(c.Imm.(*flow.Names).List))
	case flow.OpFrameVal:
		return fmt.Sprintf("%s[%d]!", args()[0], c.Imm.(flow.Slot))
	}
	panic(tsFlowUnsupported("operation " + c.Callee))
}

// markLocal returns the local of the mark that the call c takes as its argument.
func (p *tsFlowPrinter) markLocal(c *code.Call) string {
	l, ok := c.Args[0].(*code.Local)
	if !ok {
		panic(tsFlowUnsupported("a mark that is not a variable"))
	}
	return l.Name
}

// set writes the assignment s of one call of a primitive.
func (p *tsFlowPrinter) set(s *code.Set, expr func(code.Expr) string) {
	if p.dead {
		return
	}
	if len(s.Dst) == 1 {
		// The expectations are a copy already, so keeping them leaves the variable as it is.
		c := s.Values[0].(*code.Call)
		if execlower.FlowOp(c.Callee) == flow.OpKeep && !s.Dst[0].Reg {
			if l, ok := c.Args[0].(*code.Local); ok && l.Name == s.Dst[0].Name {
				return
			}
		}
	}
	if text := p.setText(s, expr, true); text != "" {
		p.write(text)
	}
}

// setText returns the assignment s of one call of a primitive as TypeScript, or "" if it is left out.
// As a statement it ends with a semicolon and a newline.
func (p *tsFlowPrinter) setText(s *code.Set, expr func(code.Expr) string, stmt bool) string {
	end := ""
	if stmt {
		end = ";\n"
	}
	c := s.Values[0].(*code.Call)
	name := execlower.FlowOp(c.Callee)
	used := false
	for _, d := range s.Dst {
		if d.Reg || d.Name == "" {
			used = true
		} else {
			used = used || p.isRead(d.Name)
		}
	}
	local := func(d code.Place) string {
		if d.Reg {
			return ""
		}
		return d.Name
	}
	if len(s.Dst) == 1 && name == flow.OpMarkState {
		m := local(s.Dst[0])
		if p.marks == nil {
			p.marks = map[string]flow.StateMark{}
		}
		p.marks[m] = c.Imm.(flow.StateMark)
		srcs := [4]string{"p.pos", "p.env", "p.trail.length", "p.recovered.length"}
		var parts []string
		for i, src := range srcs {
			if p.isRead(p.part(m, i)) {
				parts = append(parts, m+tsMarkParts[i]+" = "+src)
			}
		}
		if len(parts) == 0 {
			return ""
		}
		return strings.Join(parts, stmtSep(stmt)) + end
	}
	if len(s.Dst) == 2 && name == flow.OpShapeSingle {
		// The second result, whether the value is a single element, is not used.
		return p.setText(&code.Set{Dst: s.Dst[:1], Values: s.Values}, expr, stmt)
	}
	if len(s.Dst) == 2 && name == flow.OpUnisolate {
		// The farthest failure of the record, and a copy of its expectations in p.isoIds.
		p.read(c.Args...)
		far, ids := local(s.Dst[0]), local(s.Dst[1])
		farRead, idsRead := p.isRead(far), p.isRead(ids)
		call := fmt.Sprintf("p.unisolate(%s)", expr(c.Args[0]))
		switch {
		case farRead && idsRead:
			return fmt.Sprintf("%s = %s%s%s = p.isoIds%s", far, call, stmtSep(stmt), ids, end)
		case farRead:
			return fmt.Sprintf("%s = %s%s", far, call, end)
		}
		// Only the expectations are read, or none: the record still ends.
		if idsRead {
			return fmt.Sprintf("%s%s%s = p.isoIds%s", call, stmtSep(stmt), ids, end)
		}
		return call + end
	}
	if len(s.Dst) == 2 && tspass.TwoResults(name) {
		// A call with two results: the value and whether it matched.
		p.read(c)
		x := expr(c)
		vv, ov := local(s.Dst[0]), local(s.Dst[1])
		valRead, okRead := p.isRead(vv), p.isRead(ov)
		switch {
		case valRead && okRead:
			return fmt.Sprintf("%s = %s as Node | null%s%s = %s !== undefined%s", vv, x, stmtSep(stmt), ov, vv, end)
		case valRead:
			return fmt.Sprintf("%s = %s as Node | null%s", vv, x, end)
		case okRead:
			return fmt.Sprintf("%s = %s !== undefined%s", ov, x, end)
		}
		return x + end
	}
	if !used && tsPure(c) {
		return ""
	}
	if len(s.Dst) != 1 {
		panic(tsFlowUnsupported("an assignment of a call with several results"))
	}
	var parts []string
	switch d := s.Dst[0]; {
	case d.Reg:
		p.read(c)
		parts = append(parts, p.reg(d.Name)+" = "+expr(c))
	case d.Name == "":
		if !tsPure(c) {
			p.read(c)
			parts = append(parts, expr(c))
		}
	default:
		if !p.isRead(d.Name) && tsPure(c) {
			break
		}
		p.read(c)
		x := expr(c)
		if p.isRead(d.Name) {
			parts = append(parts, d.Name+" = "+x)
		} else {
			parts = append(parts, x)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, stmtSep(stmt)) + end
}

// reg returns the TypeScript expression of the register name.
func (p *tsFlowPrinter) reg(name string) string {
	if name == execlower.Cut {
		return "p.cut"
	}
	return "p.pos"
}

// testedCond returns the condition that the call, which has two results, fails: the call and its
// test in one condition, since nothing else reads the flag it sets. The condition assigns the value
// of the call to the local v if something reads v.
func (p *tsFlowPrinter) testedCond(call *code.Call, v string, expr func(code.Expr) string) string {
	p.read(call)
	cond := expr(call) + " === undefined"
	if p.isRead(v) {
		cond = fmt.Sprintf("(%s = %s as Node | null) === undefined", v, expr(call))
	}
	return cond
}

// peek writes the read of the next character into the places of s.
func (p *tsFlowPrinter) peek(s *code.Set) {
	if p.dead {
		return
	}
	name := func(i int) string {
		if s.Dst[i].Reg {
			return ""
		}
		return s.Dst[i].Name
	}
	ch, size, ok := name(0), name(1), name(2)
	wantCh, wantSize, wantOK := p.isRead(ch), p.isRead(size), p.isRead(ok)
	var b strings.Builder
	switch {
	case wantCh:
		fmt.Fprintf(&b, "%s = p.peek();\n", ch)
		if wantOK {
			fmt.Fprintf(&b, "%s = %s >= 0;\n", ok, ch)
		}
	case wantOK:
		fmt.Fprintf(&b, "%s = p.peek() >= 0;\n", ok)
	default:
		b.WriteString("p.peek();\n")
	}
	if wantSize {
		fmt.Fprintf(&b, "%s = p.psize;\n", size)
	}
	p.write(b.String())
}

// reset returns the statement that goes back to the mark m: the position, and the parts of the parser
// state that the code since the mark may change.
func (p *tsFlowPrinter) reset(m string) string {
	st, ok := p.marks[m]
	if !ok {
		panic(tsFlowUnsupported("a reset of a mark that is not set"))
	}
	p.readName(p.part(m, 0))
	if !st.Env && !st.Trail && !st.Rec {
		return "p.pos = " + m + "Pos;\n"
	}
	args := []string{m + "Pos", "p.env", "p.trail.length", "p.recovered.length"}
	for i, needed := range [4]bool{true, st.Env, st.Trail, st.Rec} {
		if needed {
			p.readName(p.part(m, i))
			args[i] = m + tsMarkParts[i]
		}
	}
	return "p.reset(" + strings.Join(args, ", ") + ");\n"
}

// outline prints the function of the outlined node of the call c and returns the call of it. The
// passes that only collect reads print no function.
func (p *tsFlowPrinter) outline(c *code.Call) string {
	if p.dry {
		return "e(p)"
	}
	if call, ok := p.outlined[c]; ok {
		return call
	}
	x := c.Imm.(flow.Outline)
	g := p.g
	f, err := g.FlowPasses(flowlower.Unit{Module: g.Flow.M, Opts: g.Flow.Opts, Func: flowlower.Func{Form: flowlower.Outline, Rule: g.Flow.M.Rules[g.Cur.ID], Name: g.Cur.Name, Body: x.Node, Slots: x.Slots, Build: x.Build}})
	if err != nil {
		panic(fmt.Errorf("outlined node of rule %s in table %s: %w", g.Cur.Name, g.Table, err)) // Outlinable holds
	}
	call, ok := g.flowFunc(f, x.Slots, check.NewScope(), g.Cur.Name+" (outlined)")
	if !ok {
		panic(tsFlowUnsupported("an outlined node"))
	}
	if p.outlined == nil {
		p.outlined = map[*code.Call]string{}
	}
	p.outlined[c] = call
	return call
}
