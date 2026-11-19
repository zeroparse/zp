package print

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/internal/middleend/flowplan"
	rprogram "zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Value-free rules of the TypeScript library printed from Flow IR.
//
// The rules are lowered by flowlower.Lower (with the options of the untyped tables, except for the
// Go-only forms: in-place literal reads and class tables) and printed by tsFlowPrinter as one
// function each. A block becomes a labeled block and a loop a labeled
// for statement, so that an exit leaves the construct it names with break or continue. A rule
// that Flow IR or the printer does not cover fails the generation.

// tsFlowUnsupported is the panic value of the printer for a construct it does not print; the
// generation then fails.
type tsFlowUnsupported string

// startTSFlow builds the Core IR of the table being generated for printing its rules from Flow IR. It
// reports whether the Core IR was built; if not, the generation fails with the error it records.
func (g *Generator) startTSFlow() bool {
	g.Flow = nil
	if err := g.StartRecFlow(g.Prog); err != nil {
		if g.FlowErr == nil {
			g.FlowErr = err
		}
		return false
	}
	// The generated TypeScript reads input through the library's peek and matches literals through
	// matchLiteral: it has no in-place reads and no class tables, so it prints none of the forms of
	// the Flow passes that need them (FlowTarget is empty).
	if chosen := outlineNodes(g.Flow.M, g.Table == "rules"); len(chosen) > 0 {
		g.Flow.Opts.Outline = func(n core.Node) bool { return chosen[n] }
	}
	return true
}

// flowRule prints the rule r from Flow IR as a function of its own and returns the call
// expression that runs it, or false if r is not printed from Flow IR. A lean value-free rule that
// flowlower.LocalRule accepts prints as a flat function; a rule whose body is a framed body prints under the
// frame the library owns, which also runs its action and finish. On success it sets scope to the
// capture scope of the printed body.
func (g *Generator) flowRule(r *common.Rule, scope *check.Scope) (call string, ok bool) {
	if g.Flow == nil {
		return "", false
	}
	var f *code.Module
	var slots []core.Slot
	cr := g.Flow.M.Rules[r.ID]
	if flowplan.RecEligible(r.PlanRule()) && flowlower.LocalRule(cr) {
		var err error
		if f, err = g.FlowPasses(flowlower.Lower(g.Flow.M, cr, r.Name, g.Flow.Opts)); err != nil {
			panic(fmt.Errorf("rule %s of table %s: %w", r.Name, g.Table, err)) // LocalRule holds
		}
		if flow.AttrsOf(flow.FuncOf(f)).Binds {
			f = nil // the library owns the frame the binds write
		}
	}
	if f == nil {
		if _, eligible := g.FlowNodeFrameOK(r); !eligible {
			return "", false
		}
		var err error
		if f, err = g.FlowPasses(flowlower.LowerFrame(g.Flow.M, cr, cr.Body, cr.Scope, !r.Lean, r.Name, g.Flow.Opts)); err != nil {
			panic(fmt.Errorf("rule %s of table %s: %w", r.Name, g.Table, err)) // FramedBody holds
		}
		slots = cr.Scope
	}
	return g.flowFunc(f, slots, scope, common.Describe(r.BodyExpr()))
}

// flowFunc prints the function f, whose capture scope has the slots, as a function of its own and
// returns the call expression that runs it, or false if the printer does not print f. On success it
// sets scope to the capture scope of a framed f. The comment describes the function.
func (g *Generator) flowFunc(f *code.Module, slots []core.Slot, scope *check.Scope, comment string) (call string, ok bool) {
	defer func() {
		if e := recover(); e != nil {
			if _, unsupported := e.(tsFlowUnsupported); !unsupported {
				panic(e)
			}
			call, ok = "", false
		}
	}()
	frame := flow.AttrsOf(flow.FuncOf(f)).Frame // the conversion to the Exec stage moves the function out of f
	var body string
	var vars []string
	var decls []tsDecl
	body, vars, decls = g.printTSFlowExec(f)
	var b strings.Builder
	for _, v := range vars {
		b.WriteString(v)
	}
	b.WriteString(indent(body))
	if frame {
		*scope = *check.NewScope()
		for _, sl := range slots {
			scope.Slot(sl.Name)
		}
	}
	name, shared := g.MethodName("Parser/R", tsSharedKey(b.String(), decls))
	if !shared {
		for _, d := range decls {
			if tsNamesIdentifier(b.String(), d.name) {
				g.Vars.WriteString(d.text)
			}
		}
		param := "p"
		if !strings.Contains(b.String(), "p.") && !strings.Contains(b.String(), "(p)") {
			param = "_p"
		}
		fmt.Fprintf(&g.Methods, "// %s\nfunction %s(%s: Parser): R {\n%s}\n\n", comment, name, param, b.String())
	}
	return name + "(p)", true
}

var (
	tsMarkParts = [4]string{"Pos", "Env", "Trail", "Rec"}
	tsMarkZero  = [4]string{"0", "null as Env | null", "0", "0"}
)

func tsFlowType(t flow.Type) string {
	switch t {
	case flow.TypeInt, flow.TypeRune:
		return "number"
	case flow.TypeBool:
		return "boolean"
	case flow.TypeValue:
		return "Node | null"
	case flow.TypeVals:
		return "(Node | null)[]"
	case flow.TypeFrame:
		return "Frame"
	case flow.TypeExpMark:
		return "number"
	case flow.TypeExpIDs:
		return "number[]"
	case flow.TypeError:
		return "SyntaxError | null"
	}
	panic(tsFlowUnsupported("variable of type " + t.String()))
}

func tsFlowZero(t flow.Type) string {
	switch t {
	case flow.TypeBool:
		return "false"
	case flow.TypeValue:
		return "null"
	case flow.TypeVals:
		return "[]"
	case flow.TypeFrame:
		return "emptyFrame"
	case flow.TypeExpIDs:
		return "noExp"
	case flow.TypeError:
		return "null"
	}
	return "0"
}

// tsSharedKey returns the text by which printed functions share a body: the text with each constant
// it names replaced by the declaration of the constant, so that equal functions share whatever the
// names of their constants (the literal tables of the TypeScript generation are not shared) are.
func tsSharedKey(code string, decls []tsDecl) string {
	for _, d := range decls {
		if !tsNamesIdentifier(code, d.name) {
			continue
		}
		id := regexp.MustCompile(`\b` + regexp.QuoteMeta(d.name) + `\b`)
		code = id.ReplaceAllLiteralString(code, "{"+id.ReplaceAllLiteralString(d.text, "$")+"}")
	}
	return code
}

// tsDecl is a package-level constant of a printed function.
type tsDecl struct{ name, text string }

func (p *tsFlowPrinter) write(s string) {
	if !p.dead && !p.dry {
		p.b.WriteString(s)
	}
}

// counted returns the expression node, which constructs a node, counted for the memory limit as
// bytes, an expression of the library's size model, when the parser has limit controls.
func (p *tsFlowPrinter) counted(node, bytes string) string {
	if p.g.NoBudget {
		return node
	}
	return "p.counted(" + node + ", " + bytes + ")"
}

// tsSpend returns the statement that charges n steps, as n one-unit charges.
func tsSpend(n int) string {
	if n == 1 {
		return "p.spend();\n"
	}
	return fmt.Sprintf("p.spendUnits(%d);\n", n)
}

func indent(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.SplitAfter(s, "\n")
	var b strings.Builder
	for _, l := range lines {
		if l != "" {
			b.WriteString("  ")
			b.WriteString(l)
		}
	}
	return b.String()
}

func stmtSep(stmt bool) string {
	if stmt {
		return ";\n"
	}
	return ", "
}

var tsFixedExpNames = []string{"idAny", "idBeginInput", "idEndInput", "idBeginLine", "idEndLine"}

// exp returns the TypeScript expression of the expectation id: the name of a fixed one, otherwise its
// ID in the parser's table.
func (p *tsFlowPrinter) exp(id core.ExpID) string {
	if int(id) < len(tsFixedExpNames) {
		return tsFixedExpNames[id]
	}
	return strconv.Itoa(int(p.desc(id)))
}

func (p *tsFlowPrinter) desc(id core.ExpID) rprogram.ExpID { return p.g.Desc(p.g.Flow.M.Exps[id]) }

// classIn returns the condition that the code point in ch is in the class c (false at the end of the
// input is the caller's check).
func tsClassIn(c *core.Class, ch string) string {
	var in []string
	for _, r := range c.Ranges {
		if r.Lo == r.Hi {
			in = append(in, fmt.Sprintf("%s === %d", ch, r.Lo))
		} else {
			in = append(in, fmt.Sprintf("(%s >= %d && %s <= %d)", ch, r.Lo, ch, r.Hi))
		}
	}
	if len(in) == 0 {
		in = []string{ch + " < 0"} // never true; mentions ch so that it is used
	}
	cond := tsJoinCondition(in, " || ")
	if c.Negated {
		return "!" + wrap(cond)
	}
	return cond
}

func wrap(cond string) string {
	if strings.HasPrefix(cond, "(") && strings.HasSuffix(cond, ")") {
		return cond
	}
	return "(" + cond + ")"
}

// decl declares a package-level constant of the printed function, once per pass that prints.
func (p *tsFlowPrinter) decl(name, s string) {
	p.decls = append(p.decls, tsDecl{name, s})
}

// call returns the call of the rule r from the Operators Rule level min.
func (p *tsFlowPrinter) call(r *common.Rule, min int) string {
	g := p.g
	g.checkCall(r)
	switch {
	case callsPlain(r, g.Document):
		g.plainUsed[r] = true
		return fmt.Sprintf("%s%d(p)", g.plainPrefix(), r.ID)
	case r.Plain && !g.Document:
		return fmt.Sprintf("p.invokePlain(%s, %d)", g.ruleVar(r), min)
	}
	return fmt.Sprintf("p.call(%s, %d)", g.ruleVar(r), min)
}

// tsNamesIdentifier reports whether the code uses the identifier name.
func tsNamesIdentifier(code, name string) bool {
	for i := 0; ; {
		j := strings.Index(code[i:], name)
		if j < 0 {
			return false
		}
		j += i
		end := j + len(name)
		if end >= len(code) || !(code[end] >= '0' && code[end] <= '9' || code[end] >= 'a' && code[end] <= 'z' || code[end] >= 'A' && code[end] <= 'Z' || code[end] == '_') {
			return true
		}
		i = j + 1
	}
}

// tsShape packs a shape as the library's Shaper (tsShaper).
func tsShape(sh core.Shape) int {
	s := int(sh.Mode)<<2 | sh.K<<5
	if sh.Elems {
		s |= 1
	}
	if sh.Tuple {
		s |= 2
	}
	if sh.Shared {
		s |= 16
	}
	return s
}

// scopeNames returns the constant that holds the capture names n, declaring it on first use.
func (p *tsFlowPrinter) scopeNames(n *flow.Names) string {
	if p.dry {
		return "scope"
	}
	if name, ok := p.names[n]; ok {
		return name
	}
	name := p.g.Name("scope")
	if p.names == nil {
		p.names = map[*flow.Names]string{}
	}
	p.names[n] = name
	p.decl(name, fmt.Sprintf("const %s: string[] = %s;\n", name, TsStrings(n.List)))
	return name
}

// grammarTerm returns the term of the grammar predicate that the node n is built from, for the name
// of the variable that a binding sets.
func (p *tsFlowPrinter) grammarTerm(n core.Node) ast.ActionExpr {
	e, ok := p.g.Flow.Origin[n].(*ast.Predicate)
	if !ok {
		panic(tsFlowUnsupported(fmt.Sprintf("no grammar predicate for %T", n)))
	}
	return e.Action
}

// flowLine prints the line l of the Operators Rule g.cur, whose expression is e, from Flow IR as a
// function and returns the call expression that runs it, or false if the printer does not print
// the line. It sets scope to the capture scope of the line. With budget controls the library loop that
// runs the line charges its entry and the function charges each iteration of a repetition.
func (g *Generator) flowLine(l *core.OpLine, e ast.Expr, scope *check.Scope, build bool) (string, bool) {
	if g.Flow == nil || l == nil || !flowlower.LineOK(e) || !flowlower.FramedBody(l.Body) {
		return "", false
	}
	f, err := g.FlowPasses(flowlower.LowerFrame(g.Flow.M, g.Flow.M.Rules[g.Cur.ID], l.Body, l.Scope, build, g.Cur.Name, g.Flow.Opts))
	if err != nil {
		panic(fmt.Errorf("line of Operators Rule %s in table %s: %w", g.Cur.Name, g.Table, err)) // FramedBody holds
	}
	prev := g.Proj
	g.Proj = nil // the actions of the lines use no projections of the rule
	defer func() { g.Proj = prev }()
	return g.flowFunc(f, l.Scope, scope, g.Cur.Name+" (OperatorsRule line)")
}
