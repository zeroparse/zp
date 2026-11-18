package gogen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/flowlower"
)

// Typed Go rules printed from Flow IR.
//
// The typed rules are built as Core IR (TypedCore) and printed from Flow IR here. The rules that
// flowlower.LocalRule accepts are lowered to local functions (flowlower.Lower) and printed into the flat
// rule that typedCall and flatMethod wrap. The bodies of left-recursion leaders and of rules
// that recover, and the lines of Operators Rules, are lowered to framed functions
// (flowlower.LowerFrame) and printed into the matcher methods that the library calls under the frames
// it owns (finishFrameBody). A construct that neither form accepts is reported as an error
// (failFlow): no table has another route.
//
// The printer follows reachability: a statement at an unreachable point is left out,
// but its text is still formed, so that the package variables it names (literals, expectations,
// class tables, shapers, scopes) are declared in the same order. A variable that no printed statement
// reads is not declared and its assignments are left out (flowVars); one that must stay is assigned to the
// blank identifier on the first line.

// startFlow builds the Core IR of the typed rules for printing them from Flow IR.
func (g *Generator) startFlow() error {
	origin := map[core.Node]grammar.Expr{}
	m, err := g.TypedCore(origin)
	if err != nil {
		return err
	}
	g.Flow = &codegen.FlowState{M: m, Origin: origin, Rules: append(append([]*compile.Rule(nil), g.Prog.RuleList...), g.Prog.Twins...),
		Opts: g.FlowOptions(m)}
	return nil
}

// FlowOptions returns the options of lowering the typed rules of the Core IR m, while the typed
// table is generated.
func (g *Generator) FlowOptions(m *core.Module) flowlower.Options {
	// Node is false: the options of the Node and recognizer tables (codegen.Generator.StartRecFlow) are
	// built apart, and a function lowered with one set of options is never printed for the other
	// runtime.
	tableRanges := 0
	if g.leanTyped() {
		tableRanges = g.Opt.Value("class-tables")
	}
	return flowlower.Options{Budget: !g.NoBudget, Recovers: flowlower.Recovers(m), ClassTableRanges: tableRanges,
		GuardedClasses: g.GuardedClasses(), InlineLiterals: g.InlineLiterals(), Observe: g.Observability, Document: g.RecordsExamined(), Traced: g.Traced}
}

// flowTypedRule prints the rule r from Flow IR as a flat rule, or returns nil if r is not
// printed from Flow IR.
func (g *Generator) flowTypedRule(r *compile.Rule) *flatRule {
	if g.Flow == nil || g.Table != "trules" {
		return nil
	}
	cr := g.Flow.M.Rules[r.ID]
	if !flowlower.LocalRule(cr) {
		return nil
	}
	f, err := g.FlowPasses(flowlower.Lower(g.Flow.M, cr, r.Name, g.Flow.Opts))
	if err != nil {
		panic(fmt.Errorf("rule %s of table %s: %w", r.Name, g.Table, err)) // LocalRule holds
	}
	g.Cur, g.Proj = r, g.Projections(r)
	defer func() { g.Proj = nil }()
	p := newFlowPrinter(g, false)
	p.document = f.Document
	return g.execFlatRule(p, f)
}

// flowFrameBody prints the body of the typed rule r from Flow IR as the matcher of its frame
// body, or returns "" if Flow IR does not print r that way.
func (g *Generator) flowFrameBody(r *compile.Rule, scope *check.Scope) string {
	if g.Flow == nil || g.Table != "trules" {
		return ""
	}
	comment, ok := g.typedFrameComment(r)
	cr := g.Flow.M.Rules[r.ID]
	// The runtime-owned frame has the slots of the Node rule's scope.
	if !ok || !flowlower.FramedBody(cr.Body) || !slices.Equal(core.SlotNames(cr.Scope), g.NodeScopes[r]) {
		return ""
	}
	if g.Flow.Opts.Node {
		panic("internal error: a typed rule is lowered with the options of the Node tables")
	}
	f, err := g.FlowPasses(flowlower.LowerFrame(g.Flow.M, cr, cr.Body, cr.Scope, cr.Want, r.Name, g.Flow.Opts))
	if err != nil {
		panic(fmt.Errorf("rule %s of table %s: %w", r.Name, g.Table, err)) // FramedBody holds
	}
	g.Cur, g.Proj = r, g.Projections(r)
	defer func() { g.Proj = nil }()
	return g.flowFramed(f, cr.Scope, scope, r.Name+" ("+comment+")", true)
}

// flowNodeFrameBody prints the body of the rule r of the Node table from Flow IR as the matcher of
// its frame body, or returns "" if r takes another route. The rule is eligible
// (codegen.Generator.FlowNodeFrameOK) when its Core body is a framed body and its action reads only
// captures of the frame; the library owns depth, memo, frame, action and finish. With budget controls
// (Options.Budget) the method that holds the body charges one step on entry (Func.Entered) and the
// body charges one on each iteration of a repetition, plus the input spend of each Peek; the limits
// of a parse hold at these charges.
func (g *Generator) flowNodeFrameBody(r *compile.Rule, scope *check.Scope) string {
	cr, ok := g.FlowNodeFrameOK(r)
	if !ok {
		return ""
	}
	if !g.Flow.Opts.Node {
		panic("internal error: a Node rule is lowered with the options of the typed table")
	}
	f, err := g.FlowPasses(flowlower.LowerFrame(g.Flow.M, cr, cr.Body, cr.Scope, !r.Lean, r.Name, g.Flow.Opts))
	if err != nil {
		panic(fmt.Errorf("rule %s of table %s: %w", r.Name, g.Table, err)) // FramedBody holds
	}
	g.Cur, g.Proj = r, g.Projections(r)
	defer func() { g.Proj = nil }()
	return g.flowFramed(f, cr.Scope, scope, r.Name+" (Node body inlined)", true)
}

// flowStreamBody prints the body of the start rule r of ParseStream from Flow IR as the matcher of its
// frame body, with the repetition of its #stream delivering the elements (flowlower.Options.Stream), or
// returns "" if the rule is not printed that way: a rule is eligible as for flowNodeFrameBody. The
// capture layout of the printed body must be the batch body's, which the action addresses by slot.
func (g *Generator) flowStreamBody(r *compile.Rule, scope *check.Scope) string {
	cr, ok := g.FlowNodeFrameOK(r)
	if !ok || !g.Flow.Opts.Node {
		return ""
	}
	opts := g.Flow.Opts
	opts.Stream = true
	f, err := g.FlowPasses(flowlower.LowerFrame(g.Flow.M, cr, cr.Body, cr.Scope, !r.Lean, r.Name, opts))
	if err != nil {
		panic(fmt.Errorf("stream body of rule %s: %w", r.Name, err)) // FramedBody holds
	}
	g.Cur, g.Proj = r, g.Projections(r)
	defer func() { g.Proj = nil }()
	return g.flowFramed(f, cr.Scope, scope, r.Name+" (stream body)", true)
}

// flowOperatorsRuleBody prints the line l of the Operators Rule g.cur, whose expression is e, from Flow
// IR, or returns "" if Flow IR does not print the line that way.
//
// In the typed table every line is printed from Flow IR, as in the Node and recognizer tables (a line with
// recovery builds its error node with NewErrorNode there). Nothing needs a check. The lines of an Operators
// Rule have no projected repetitions (ruleProjections gives none for one), so Core IR and the tables agree
// on the shape of every capture, and the checker lets an action name only captures of its line. With budget
// controls (Options.Budget) the method that holds the function charges its entry and the function charges
// each iteration of a repetition, as Flow IR makes them explicit.
func (g *Generator) flowOperatorsRuleBody(l *core.OpLine, e grammar.Expr, scope *check.Scope, build bool) string {
	if g.Flow == nil || l == nil || !flowlower.LineOK(e) || !flowlower.FramedBody(l.Body) {
		return ""
	}
	if g.Table != "trules" && !flowlower.BodyOK(e, true, true) {
		return ""
	}
	f, err := g.FlowPasses(flowlower.LowerFrame(g.Flow.M, g.Flow.M.Rules[g.Cur.ID], l.Body, l.Scope, build, g.Cur.Name, g.Flow.Opts))
	if err != nil {
		panic(fmt.Errorf("line of Operators Rule %s in table %s: %w", g.Cur.Name, g.Table, err)) // FramedBody holds
	}
	prev := g.Proj
	g.Proj = nil // the actions of the lines use no projections of the rule
	defer func() { g.Proj = prev }()
	if g.Table != "trules" {
		if g.hooks.OperatorsRoute != nil && g.collect == nil && !g.Traced {
			g.hooks.OperatorsRoute(g.Table, g.Cur)
		}
		return g.flowFramed(f, l.Scope, scope, g.Cur.Name+" (OperatorsRule body inlined)", false)
	}
	return g.flowFramed(f, l.Scope, scope, g.Cur.Name+" (typed OperatorsRule body inlined)", false)
}

// flowOperatorsRule returns the Core IR of the Operators Rule r, or nil if its lines are not
// printed from Flow IR: the typed table prints them all from it, the Node and recognizer tables
// when their Flow route is on (flowOperatorsRuleBody decides each line).
func (g *Generator) flowOperatorsRule(r *compile.Rule) *core.OperatorsRule {
	if g.Flow == nil || g.Table != "trules" && !g.Flow.Opts.Node {
		return nil
	}
	o, _ := g.Flow.M.Rules[r.ID].Body.(*core.OperatorsRule)
	return o
}

// flowFramed prints the framed function f, whose capture scope has the slots, as a matcher method
// with the comment, as finishFrameBody does, and sets scope to its capture scope. A rule body (ruleBody)
// has the signature of the Body field of the rule table (see bodyMethod).
func (g *Generator) flowFramed(f *flow.Func, slots []core.Slot, scope *check.Scope, comment string, ruleBody bool) string {
	p := newFlowPrinter(g, true)
	p.document = f.Document
	return g.execFramed(p, f, slots, scope, comment, ruleBody)
}

func goType(t flow.Type) string {
	return [...]string{flow.TypeInt: "int", flow.TypeBool: "bool", flow.TypeRune: "rune", flow.TypeValue: "any",
		flow.TypeEnv: "*Env", flow.TypeExpMark: "ExpMark", flow.TypeExpIDs: "[]ExpID", flow.TypeVals: "[]any",
		flow.TypeCtx: "*TActCtx", flow.TypeMark: "Mark", flow.TypeFrame: "*TCaptureFrame", flow.TypeError: "*SyntaxError"}[t]
}

// goType returns the Go type of a variable of the function printed: the types of the Node library
// where the printer writes its code, otherwise those of the typed library.
func (p *flowPrinter) goType(t flow.Type) string {
	if p.untyped() {
		switch t {
		case flow.TypeValue:
			return "*Node"
		case flow.TypeVals:
			return "[]*Node"
		case flow.TypeFrame:
			return "*CaptureFrame"
		}
	}
	return goType(t)
}

// flowPrinter prints the body of a Flow IR function as the statements of a flat rule or, if
// frame, of a matcher method.
type flowPrinter struct {
	g     *Generator
	frame bool
	// document reports that the function is lowered for Documents (flow.Func.Document): its in-place reads
	// are guarded by p.Document == nil, so that a Document parse reads through the recording primitives.
	document bool
	b        strings.Builder
	// names holds the package variables of the capture names of scopes, declared on first use
	// unless a DeclareNames statement declares them.
	names map[*flow.Names]string
	// pre holds the statements the expression printed last needs before it: those of the term of an Eval.
	pre string
}

func newFlowPrinter(g *Generator, frame bool) *flowPrinter {
	return &flowPrinter{g: g, frame: frame, names: map[*flow.Names]string{}}
}

// scopeNames returns the package variable of the capture names n, naming it on first use.
func (p *flowPrinter) scopeNames(n *flow.Names) string {
	if name, ok := p.names[n]; ok {
		return name
	}
	name := p.g.Name("scope")
	p.names[n] = name
	if !n.Declared {
		p.declare(n)
	}
	return name
}

func (p *flowPrinter) declare(n *flow.Names) {
	fmt.Fprintf(&p.g.Vars, "var %s = %s\n", p.names[n], goStrings(n.List))
}

// untyped reports whether the printer writes the Node library's code (the Node and recognizer tables).
func (p *flowPrinter) untyped() bool { return p.g.Table == "recRules" || p.g.Table == "rules" }

// matcher returns the parser whose methods match literals: the embedded Node parser of a typed
// parser, or the Node parser itself.
func (p *flowPrinter) matcher() string {
	if p.untyped() {
		return "p"
	}
	return "p.Parser"
}

// helper names the library method that the printed code calls on the parser the body runs on: the
// typed parser's embedded parser is not renamed.
func (p *flowPrinter) helper(name string) string {
	if !p.untyped() {
		return name
	}
	return p.g.helper(name)
}

// write writes s.
func (p *flowPrinter) write(s string) { p.b.WriteString(s) }

// expectHelper names the library method that records an expectation.
func (p *flowPrinter) expectHelper() string {
	return p.g.helper("Expect")
}

// peekText returns the read of the next character into the places ch, size and ok.
func (p *flowPrinter) peekText(ch, size, ok string) string {
	// The inlined read and its fallback are one call of the library, which the compiler inlines.
	if p.g.Ordinary {
		return fmt.Sprintf("%s, %s, %s = p.NextOrdinary()\n", ch, size, ok)
	}
	if p.g.Traced {
		// A traced body runs only in an observed parse, which records every read.
		return fmt.Sprintf("%s, %s, %s = p.%s()\n", ch, size, ok, p.g.helper("Peek"))
	}
	guard := "p.Pos < len(p.In)"
	if p.document {
		guard = "p.Document == nil && " + guard
	}
	// The inlined read charges its input spend and leaves a Document's examined range to Peek.
	spend := "p.Control.Take(1)\n"
	if p.document {
		spend = "p.SpendRead(1)\n"
	}
	if !p.g.Flow.Opts.Budget {
		spend = ""
	}
	return fmt.Sprintf("if %s {\n%s%s, %s, %s = p.In[p.Pos], 1, true\n} else {\n%[3]s, %[4]s, %[5]s = p.Peek()\n}\n", guard, spend, ch, size, ok)
}

// fixedExpNames names the expectations of the library that every parser has, by ID.
var fixedExpNames = []string{"IdAny", "IdBeginInput", "IdEndInput", "IdBeginLine", "IdEndLine"}

// exp returns the Go expression of the expectation id: the name of a fixed one, otherwise its ID
// in the parser's table.
func (p *flowPrinter) exp(id core.ExpID) string {
	if int(id) < len(fixedExpNames) {
		return fixedExpNames[id]
	}
	return strconv.Itoa(int(p.desc(id)))
}

// desc returns the ID in the parser's table of the expectation id of the module.
func (p *flowPrinter) desc(id core.ExpID) compile.ExpID { return p.g.Desc(p.g.Flow.M.Exps[id]) }

// grammarClass returns the character class expression of the class c.
func grammarClass(c *core.Class) *grammar.CharClass {
	e := &grammar.CharClass{Negated: c.Negated}
	for _, r := range c.Ranges {
		e.Ranges = append(e.Ranges, grammar.CharRange{Lo: r.Lo, Hi: r.Hi})
	}
	return e
}

// reject returns the condition under which the code point in the variable ch is not in the class.
func (p *flowPrinter) reject(c *core.Class, ch string) string {
	cond := p.g.ControlledClassReject(grammarClass(c))
	if ch != "ch" {
		cond = chIdent.ReplaceAllString(cond, ch)
	}
	return cond
}

// shaper returns the package variable that holds the shaper of the shape.
func (p *flowPrinter) shaper(sh core.Shape) string {
	return p.g.shaperOf(fmt.Sprintf("Shaper{Elems: %v, K: %d, Mode: %d, Tuple: %v, Shared: %v}", sh.Elems, sh.K, sh.Mode, sh.Tuple, sh.Shared))
}

// term returns the term of the grammar predicate that the node n is built from.
func (p *flowPrinter) term(n core.Node) grammar.Term {
	e, ok := p.g.Flow.Origin[n].(*grammar.Predicate)
	if !ok {
		panic(fmt.Sprintf("flow printer: no grammar predicate for %T", n))
	}
	return e.Term
}
