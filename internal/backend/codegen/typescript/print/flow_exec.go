package print

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	tspass "zeroparse.dev/zp/internal/backend/codegen/typescript/pass"
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
// Before printing, the TypeScript Exec passes (typescript/pass) make charges in a row one spend, the
// end of an expectation record followed by the merge of its expectations one unisolateMerge, and the
// test of the call an If's Init the If's condition, so that the locals they no longer read are not
// declared. Each pass has a flag; the printer prints the operations they replace as well.

// printTSFlowExec is printTSFlow from Exec IR.
func (g *Generator) printTSFlowExec(f *code.Module) (string, []string, []tsDecl) {
	fn := flow.FuncOf(f)
	a := flow.AttrsOf(fn)
	l, err := common.LowerFlow(f)
	if err != nil {
		panic(fmt.Errorf("rule %s of table %s: %w", fn.Name, g.Table, err))
	}
	g.ExecPasses(l, tspass.Pipeline)
	g.ExecFuncs++
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
			case tspass.SpendOp:
				return at(dead, func() { p.write(tsSpend(c.Imm.(tspass.SpendRun).N)) })
			case tspass.MergeOp:
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
			if !ok || cond.Callee != tspass.FailedOp {
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
