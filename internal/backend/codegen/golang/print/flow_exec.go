package print

import (
	"fmt"
	goparser "go/parser"
	"go/token"
	"strings"

	goast "go/ast"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	irc "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/execlower"
)

// Go rules printed from the Exec stage of Code IR.
//
// The rules of every table (the typed table, the Node table and the recognizer, and the stream
// body), in the ordinary form, with budget controls and with a Document, are written at the Flow
// stage and converted to the Exec stage (common.LowerFlow), and printed by the Go backend of the Exec
// stage (print.Body), which prints the compositions of the library too. The primitives of grammar
// code are printed in place by the flow printer (flowPrinter.op).

// execBody prints the body of the Flow IR function f from Exec IR with the flow printer p, for a
// function that is printed whatever its body: it checks the variables (execVars) and counts the
// function (execCount).
func (p *flowPrinter) execBody(f *code.Module) BodyResult {
	l, res := p.execPrint(f)
	p.execVars(l, &res)
	p.execCount()
	return res
}

// execPrint lowers the Flow IR function f to Exec IR and prints its body with the flow printer p.
func (p *flowPrinter) execPrint(f *code.Module) (*execlower.Lowered, BodyResult) {
	fn := flow.FuncOf(f) // the conversion to the Exec stage moves it out of f
	l, err := common.LowerFlow(f)
	if err != nil {
		panic(fmt.Errorf("rule %s of table %s: %w", fn.Name, p.g.Table, err))
	}
	p.g.ExecPasses(l)
	return l, Body(l.Module, l.Func, p.execTarget(), l.Vars)
}

// execVars checks the variables of the printed body res of the lowered function l: a variable that
// the body names but that is not declared is an internal error (Generator.FlowErr). Hooks.FlowVarsMutant and Hooks.FlowVarsMutantUses instead leave variables
// undeclared, without the check.
func (p *flowPrinter) execVars(l *execlower.Lowered, res *BodyResult) {
	if p.g.hooks.FlowVarsMutant {
		// The locals that the body names stay undeclared.
		for _, v := range l.Vars {
			res.Stores.Dead[v] = true
		}
		return
	}
	if p.g.hooks.FlowVarsMutantUses {
		// The locals that only the values of assignments read stay undeclared.
		hard := otherReads(l.Func.Body)
		for _, v := range l.Vars {
			if !hard[v] {
				res.Stores.Dead[v] = true
			}
		}
		return
	}
	if p.g.FlowErr == nil {
		for _, id := range freeIdents(res.Text) {
			if res.Stores.Dead[id] {
				p.g.FlowErr = fmt.Errorf("internal error: the printed body names the variable %s, which is not declared", id)
				break
			}
		}
	}
}

// execCount counts a function printed from Exec IR that the generated code includes.
func (p *flowPrinter) execCount() {
	p.g.ExecFuncs++
	if p.g.ExecTables == nil {
		p.g.ExecTables = map[string]int{}
	}
	p.g.ExecTables[p.g.Table]++
}

// otherReads returns the locals that the statements of b other than assignments read, whatever
// their reachability: the reads that Hooks.FlowVarsMutantUses keeps.
func otherReads(b *code.Block) map[string]bool {
	out := map[string]bool{}
	read := func(x code.Expr) {
		if x == nil {
			return
		}
		code.Walk(&code.Block{Stmts: []code.Stmt{&code.Assert{Cond: x}}}, nil, func(e code.Expr) {
			if l, ok := e.(*code.Local); ok {
				out[l.Name] = true
			}
		})
	}
	header := func(s *code.Set) {
		if s != nil {
			for _, x := range s.Values {
				read(x)
			}
		}
	}
	var block func(*code.Block)
	block = func(b *code.Block) {
		if b == nil {
			return
		}
		for _, st := range b.Stmts {
			switch st := st.(type) {
			case *code.Block:
				block(st)
			case *code.Loop:
				header(st.Init)
				read(st.Cond)
				header(st.Post)
				block(st.Body)
			case *code.If:
				header(st.Init)
				read(st.Cond)
				block(st.Then)
				block(st.Else)
			case *code.Let:
				read(st.Value)
			case *code.Do:
				read(st.Call)
			case *code.Assert:
				read(st.Cond)
			case *code.Return:
				for _, x := range st.Values {
					read(x)
				}
			}
		}
	}
	block(b)
	return out
}

// execTarget returns the Go implementation of the primitive operations of grammar code at the Exec
// stage, which the flow printer prints in place (flowPrinter.op).
func (p *flowPrinter) execTarget() *Target {
	t := p.target()
	if h := p.g.hooks.FlowTarget; h != nil {
		return h(t, &p.pre)
	}
	return t
}

// target returns the Go implementation of execTarget that Hooks.FlowTarget does not replace.
func (p *flowPrinter) target() *Target {
	return &Target{
		Reg: func(name string) string {
			return map[string]string{execlower.Pos: "p.Pos", execlower.Cut: "p.Cut"}[name]
		},
		Handle: func(name string) string { return name },
		Op:     p.op,
		Stmt: func(c *code.Call, args []string, dst []string, _ bool) (string, bool) {
			switch execlower.FlowOp(c.Callee) {
			case flow.OpPeek:
				return strings.TrimSuffix(p.peekText(dst[0], dst[1], dst[2]), "\n"), true
			case flow.OpAppend:
				return p.appendText(c, args), true
			case flow.OpEval:
				v := p.op(c, args)
				pre := p.pre
				p.pre = ""
				assign := strings.Join(dst, ", ") + " = " + v
				if pre != "" {
					return "{ " + pre + assign + "\n}", true
				}
				return assign, true
			}
			return "", false
		},
		Negate: func(c *code.Call, args []string) (string, bool) { return p.negate(c, args), true },
		Prec:   goPrec,
		Pure: func(c *code.Call) bool {
			switch execlower.FlowOp(c.Callee) {
			case flow.OpStr, flow.OpMarkPos, flow.OpEnv, flow.OpFrame:
				return true
			}
			return false
		},
	}
}

// goPrec returns the precedence of the loosest binary operator of the Go expression text outside
// parentheses, or 6 for an operand.
func goPrec(text string) int {
	e, err := goparser.ParseExpr(text)
	if err != nil {
		return 0
	}
	if b, ok := e.(*goast.BinaryExpr); ok {
		return b.Op.Precedence()
	}
	return token.UnaryPrec
}

// execFlatRule prints the local function f from Exec IR as a flat rule (flowTypedRule).
func (g *Generator) execFlatRule(p *flowPrinter, f *code.Module) *flatRule {
	fn := flow.FuncOf(f) // the conversion to the Exec stage moves it out of f
	res := p.execBody(f)
	st := res.Stores
	a := flow.AttrsOf(fn)
	read := func(v string) bool { return v != "" && st.Read[v] }
	out := &flatRule{fail: res.Exited, succeed: res.End, binds: a.Binds, start: read(a.Start), ctx: read(a.Ctx)}
	var unread []string
	for _, v := range flow.VarsOf(fn) {
		out.vars = append(out.vars, dvar{v.Name, goType(flow.TypeOf(v.Type))})
		switch {
		case st.Dead[v.Name]:
			out.markVar(v.Name, true)
		case !st.Named[v.Name]:
			unread = append(unread, v.Name)
		case !st.Read[v.Name]:
			out.markVar(v.Name, false)
		}
	}
	out.body = res.Text
	if len(unread) > 0 {
		out.body = "\t" + strings.Repeat("_, ", len(unread)-1) + "_ = " + strings.Join(unread, ", ") + "\n" + out.body
	}
	g.FlowRules++
	return out
}

// execRecRule prints the local function f of a rule of the Node library from Exec IR as a flat rule,
// or returns nil where flowRecRule does: if the printed function reads its start or its context, or
// a variable has a type that untypedGoType cannot name. It checks the variables at the point where
// flowRecRule does, between the two refusals, and counts only a rule it prints.
func (g *Generator) execRecRule(p *flowPrinter, f *code.Module) *flatRule {
	fn := flow.FuncOf(f) // the conversion to the Exec stage moves it out of f
	l, res := p.execPrint(f)
	st := res.Stores
	a, vars := flow.AttrsOf(fn), flow.VarsOf(fn)
	read := func(v string) bool { return v != "" && st.Named[v] }
	if read(a.Start) || read(a.Ctx) {
		return nil
	}
	p.execVars(l, &res)
	types := make([]string, len(vars))
	for i, v := range vars {
		t, ok := untypedGoType(flow.TypeOf(v.Type))
		if !ok {
			return nil
		}
		types[i] = t
	}
	p.execCount()
	out := &flatRule{fail: res.Exited, succeed: res.End}
	var unread []string
	for i, v := range vars {
		out.vars = append(out.vars, dvar{v.Name, types[i]})
		switch {
		case st.Dead[v.Name]:
			out.markVar(v.Name, true)
		case !st.Named[v.Name]:
			unread = append(unread, v.Name)
		case !st.Read[v.Name]:
			out.markVar(v.Name, false)
		}
	}
	out.body = res.Text
	if len(unread) > 0 {
		out.body = "\t" + strings.Repeat("_, ", len(unread)-1) + "_ = " + strings.Join(unread, ", ") + "\n" + out.body
	}
	g.FlowRules++
	return out
}

// execFramed prints the framed function f from Exec IR as flowFramed does.
func (g *Generator) execFramed(p *flowPrinter, f *code.Module, slots []irc.Slot, scope *check.Scope, comment string, ruleBody bool) string {
	fn := flow.FuncOf(f) // the conversion to the Exec stage moves it out of f
	res := p.execBody(f)
	st := res.Stores
	var vars []dvar
	dead, quiet := map[string]bool{}, map[string]bool{}
	var unread []string
	for _, v := range flow.VarsOf(fn) {
		vars = append(vars, dvar{v.Name, p.goType(flow.TypeOf(v.Type))})
		switch {
		case st.Dead[v.Name]:
			dead[v.Name] = true
		case !st.Named[v.Name]:
			unread = append(unread, v.Name)
		case !st.Read[v.Name]:
			quiet[v.Name] = true
		}
	}
	var b strings.Builder
	b.WriteString(varLinesExcept(vars, dead, quiet))
	if len(unread) > 0 {
		fmt.Fprintf(&b, "%s_ = %s\n", strings.Repeat("_, ", len(unread)-1), strings.Join(unread, ", "))
	}
	b.WriteString(res.Text)
	*scope = *check.NewScope()
	for _, sl := range slots {
		scope.Slot(sl.Name)
	}
	g.FlowFrames++
	if ruleBody {
		return g.bodyMethod(comment, b.String())
	}
	return g.Method(comment, b.String())
}
