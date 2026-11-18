package print

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/middleend/execlower"
)

// Rules printed from the Exec stage of Code IR.
//
// The functions of grammar code are written at the Flow stage and converted to the Exec stage
// (common.LowerFlow), and printed by print.Body, which prints their structure and decides which
// assignments stay (codepass.DeadStores). The primitives are printed in place by tsFlowPrinter.
// Before printing (tsRuns), charges in a row become one spend, the end of an expectation record
// followed by the merge of its expectations one unisolateMerge, and the test of the call an If's Init
// makes the If's condition, so that the locals they no longer read are not declared.

// tsSpendRun is the immediate operand of a call of tsSpendOp: the number of charges in a row.
type tsSpendRun struct{ n int }

// tsFailed is the immediate operand of a call of tsFailedOp: the variable of the bool that the Init
// of the If sets.
type tsFailed struct{ flag string }

// The primitive operations of the TypeScript target that tsRuns makes.
const (
	tsSpendOp  = "ts.spend"
	tsMergeOp  = "ts.unisolateMerge"
	tsFailedOp = "ts.failed"
)

// printTSFlowExec is printTSFlow from Exec IR.
func (g *Generator) printTSFlowExec(f *code.Module) (string, []string, []tsDecl) {
	fn := flow.FuncOf(f)
	a := flow.AttrsOf(fn)
	l, err := common.LowerFlow(f)
	if err != nil {
		panic(fmt.Errorf("rule %s of table %s: %w", fn.Name, g.Table, err))
	}
	g.ExecPasses(l)
	g.ExecFuncs++
	tsRuns(l)
	vars := map[string]bool{}
	for _, v := range append(append([]string(nil), a.Vars...), a.Start, a.Result, a.Ctx) {
		if v != "" {
			vars[v] = true
		}
	}
	st := codepass.DeadStores(l.Func, l.Vars, tsPure)
	reads := map[string]bool{}
	for name := range vars {
		if st.Read[name] {
			reads[name] = true
		}
	}
	// The parts of the marks that are read: the reads of a pass do not depend on them, but which
	// parts a mark saves does.
	parts := map[string]*[4]string{}
	partReads := map[string]bool{}
	newPrinter := func(dry bool) *tsFlowPrinter {
		r := map[string]bool{}
		for v := range reads {
			r[v] = true
		}
		for v := range partReads {
			r[v] = true
		}
		return &tsFlowPrinter{g: g, parts: parts, dry: dry, reads: r, next: map[string]bool{}, labels: map[string]string{}}
	}
	partsRead := func(p *tsFlowPrinter) map[string]bool {
		out := map[string]bool{}
		for _, ps := range parts {
			for _, part := range ps {
				if p.next[part] {
					out[part] = true
				}
			}
		}
		return out
	}
	for i := 0; ; i++ {
		p := newPrinter(true)
		Body(l.Module, l.Func, p.bodyTarget(), st)
		next := partsRead(p)
		same := len(next) == len(partReads)
		for v := range next {
			same = same && partReads[v]
		}
		if same || i > 8 {
			break
		}
		partReads = next
	}
	p := newPrinter(false)
	res := Body(l.Module, l.Func, p.bodyTarget(), st)
	for _, v := range []string{a.Start, a.Ctx} {
		if v != "" && (st.Read[v] || p.next[v]) {
			panic(tsFlowUnsupported("the start or the context of a value-free rule"))
		}
	}
	body := res.Text
	if !a.Frame {
		inner := ""
		if res.End {
			inner = "return null;\n"
		}
		if res.Exited {
			body = fmt.Sprintf("%s: {\n%s%s}\nreturn undefined;\n", p.label(flow.AttrsOf(l.Func).Fail), indent(body), indent(inner))
		} else {
			body += inner
		}
	}
	var decl []string
	for _, v := range flow.VarsOf(fn) {
		t := flow.TypeOf(v.Type)
		if t == flow.TypeMark {
			// A mark of the whole parser state is four variables, declared as far as they are read.
			if ps := parts[v.Name]; ps != nil {
				for i, part := range ps {
					if p.next[part] {
						decl = append(decl, fmt.Sprintf("  let %s%s = %s;\n", v.Name, tsMarkParts[i], tsMarkZero[i]))
					}
				}
			}
			continue
		}
		if st.Read[v.Name] {
			if typ := tsFlowType(t); typ == "number" || typ == "boolean" {
				// The initial value gives the type.
				decl = append(decl, fmt.Sprintf("  let %s = %s;\n", v.Name, tsFlowZero(t)))
			} else {
				decl = append(decl, fmt.Sprintf("  let %s: %s = %s;\n", v.Name, typ, tsFlowZero(t)))
			}
		}
	}
	return body, decl, p.decls
}

// tsRuns replaces, in the lowered function l, each run of charges with one call of tsSpendOp, each
// end of an expectation record that a merge of its expectations follows with one call of tsMergeOp,
// and the condition of an If that tests the call of its Init with a call of tsFailedOp.
func tsRuns(l *execlower.Lowered) {
	m := l.Module
	op := func(o *code.Op) {
		o.Component, o.Dialect, o.Kind = execlower.Component, execlower.Dialect, code.OpPrimitive
		m.AddOp(o)
	}
	op(&code.Op{Name: tsFailedOp, Results: []code.Type{code.TBool}, Reads: []string{execlower.Component},
		Imm: "the bool the Init sets", Contract: "Reports that the call of the Init of the If failed: that the bool it sets is false."})
	op(&code.Op{Name: tsSpendOp, Writes: []string{execlower.Component}, NoReturn: true,
		Imm: "the number of charges", Contract: "Charges a step as many times as Imm says."})
	op(&code.Op{Name: tsMergeOp, Params: []code.Param{code.P("m", code.THandle(flow.TExpMark))},
		Reads: []string{execlower.Component}, Writes: []string{execlower.Component}, NoReturn: true,
		Contract: "Ends the expectation record m and merges its expectations into the enclosing record."})
	var block func(ss []code.Stmt) []code.Stmt
	block = func(ss []code.Stmt) []code.Stmt {
		var out []code.Stmt
		for i := 0; i < len(ss); i++ {
			if tsStep(ss[i]) {
				n := 1
				for i+n < len(ss) && tsStep(ss[i+n]) {
					n++
				}
				out = append(out, &code.Do{Call: &code.Call{Callee: tsSpendOp, Imm: tsSpendRun{n}}})
				i += n - 1
				continue
			}
			if i+1 < len(ss) {
				if end, ok := tsMergedRecord(ss[i], ss[i+1]); ok {
					out = append(out, &code.Do{Call: &code.Call{Callee: tsMergeOp, Args: end.Args}})
					i++
					continue
				}
			}
			switch s := ss[i].(type) {
			case *code.Block:
				s.Stmts = block(s.Stmts)
			case *code.Loop:
				s.Body.Stmts = block(s.Body.Stmts)
			case *code.If:
				if flag, ok := tsTestedFlag(s); ok {
					// The condition tests the call of the Init, so that nothing else reads the bool.
					s.Cond = &code.Call{Callee: tsFailedOp, Imm: tsFailed{flag}}
				}
				s.Then.Stmts = block(s.Then.Stmts)
				if s.Else != nil {
					s.Else.Stmts = block(s.Else.Stmts)
				}
			}
			out = append(out, ss[i])
		}
		return out
	}
	l.Func.Body.Stmts = block(l.Func.Body.Stmts)
}

// tsTestedFlag reports whether the If s tests that the call of its Init fails, which prints as one
// condition (tsFlowPrinter.testedCond), and returns the variable of the bool that the Init sets.
func tsTestedFlag(s *code.If) (string, bool) {
	if s.Init == nil || len(s.Init.Dst) != 2 || len(s.Init.Values) != 1 {
		return "", false
	}
	c, ok := s.Init.Values[0].(*code.Call)
	if !ok || !tsTwoResults(execlower.FlowOp(c.Callee)) {
		return "", false
	}
	flag := s.Init.Dst[1]
	not, ok := s.Cond.(*code.Unary)
	if !ok || not.Op != "!" || flag.Reg || flag.Name == "" {
		return "", false
	}
	if l, ok := not.X.(*code.Local); !ok || l.Name != flag.Name {
		return "", false
	}
	return flag.Name, true
}

// tsStep reports whether s charges a step.
func tsStep(s code.Stmt) bool {
	d, ok := s.(*code.Do)
	return ok && execlower.FlowOp(d.Call.Callee) == flow.OpStep
}

// tsMergedRecord reports whether a and b end an expectation record and merge its expectations into
// the enclosing record, as unisolateMerge prints them, and returns the call that ends the record.
func tsMergedRecord(a, b code.Stmt) (*code.Call, bool) {
	set, ok := a.(*code.Set)
	if !ok || len(set.Dst) != 2 || len(set.Values) != 1 {
		return nil, false
	}
	c, ok := set.Values[0].(*code.Call)
	if !ok || execlower.FlowOp(c.Callee) != flow.OpUnisolate {
		return nil, false
	}
	do, ok := b.(*code.Do)
	if !ok || execlower.FlowOp(do.Call.Callee) != flow.OpMergeExpected {
		return nil, false
	}
	far, ids := set.Dst[0], set.Dst[1]
	f, fok := do.Call.Args[0].(*code.Local)
	e, eok := do.Call.Args[1].(*code.Local)
	if !fok || !eok || far.Reg || ids.Reg || far.Name == "" || ids.Name == "" || f.Name != far.Name || e.Name != ids.Name {
		return nil, false
	}
	return c, true
}

// bodyTarget returns the printing of the primitives of a lowered function by the printer p.
func (p *tsFlowPrinter) bodyTarget() *BodyTarget {
	// at runs print at the point that dead tells and returns what it writes.
	at := func(dead bool, print func()) string {
		prev, prevDead := p.b, p.dead
		p.b, p.dead = strings.Builder{}, dead
		print()
		text := p.b.String()
		p.b, p.dead = prev, prevDead
		return text
	}
	// primitive returns the call of a primitive that s assigns, if it assigns one.
	primitive := func(s *code.Set) (*code.Call, bool) {
		if len(s.Values) != 1 {
			return nil, false
		}
		c, ok := s.Values[0].(*code.Call)
		return c, ok && execlower.FlowOp(c.Callee) != ""
	}
	return &BodyTarget{
		Reg:   p.reg,
		Label: p.label,
		Op: func(c *code.Call, args func() []string, dead bool) string {
			var s string
			at(dead, func() { s = p.expr(c, args) })
			return s
		},
		Do: func(c *code.Call, args func() []string, dead bool) string {
			switch c.Callee {
			case tsSpendOp:
				return at(dead, func() { p.write(tsSpend(c.Imm.(tsSpendRun).n)) })
			case tsMergeOp:
				return at(dead, func() {
					p.read(c.Args...)
					p.write(fmt.Sprintf("p.unisolateMerge(%s);\n", args()[0]))
				})
			}
			switch execlower.FlowOp(c.Callee) {
			case flow.OpSetEnv, flow.OpSetFrame:
				// The assignment of a register that holds a handle.
				if dead {
					return ""
				}
				reg := "p.env"
				if execlower.FlowOp(c.Callee) == flow.OpSetFrame {
					reg = "p.frame"
				}
				return reg + " = " + args()[0] + ";\n"
			}
			return at(dead, func() { p.stmtOp(c, args) })
		},
		Set: func(s *code.Set, expr func(code.Expr) string, init, dead bool) (string, bool) {
			c, ok := primitive(s)
			if !ok {
				return "", false
			}
			switch execlower.FlowOp(c.Callee) {
			case flow.OpPeek:
				return at(dead, func() { p.peek(s) }), true
			case flow.OpAppend:
				return at(dead, func() {
					p.stmtOp(c, func() []string { return []string{expr(c.Args[0]), expr(c.Args[1])} })
				}), true
			}
			if init {
				var text string
				at(dead, func() { text = p.setText(s, expr, true) })
				return text, true
			}
			return at(dead, func() { p.set(s, expr) }), true
		},
		Test: func(s *code.If, expr func(code.Expr) string, dead bool) (string, bool) {
			c, ok := primitive(s.Init)
			if !ok {
				return "", false
			}
			cond, ok := s.Cond.(*code.Call)
			if !ok || cond.Callee != tsFailedOp {
				return "", false
			}
			v := ""
			if d := s.Init.Dst[0]; !d.Reg {
				v = d.Name
			}
			var text string
			at(dead, func() { text = p.testedCond(c, v, expr) })
			return text, true
		},
		Pure: tsFlowPure,
	}
}
