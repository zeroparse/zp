package gen

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/ir/core"
	irbuild "zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/flow"
	flowlower "zeroparse.dev/zp/internal/ir/flow/lower"
	"zeroparse.dev/zp/internal/ir/opt"
	"zeroparse.dev/zp/internal/limits"
)

// Generator is the state that the Go and TypeScript generators share: the program and rule being
// printed, the generation switches both read, the expectation table, the shared names of
// literals, methods and field type variables, and the Flow IR of the rules printed from it.
type Generator struct {
	Observability bool
	// NoBudget leaves the step charges out of Go output generated without budget controls, where
	// no control exists to charge: the statements, and the closures that only held one.
	NoBudget bool
	// Ordinary prints the bodies in their ordinary form (no budget controls and no Documents): the
	// helpers with an Ordinary form, no charges, inline class tests, and for typed rule bodies the
	// lean lowering (terminal programs, class runs and tables, guarded classes).
	Ordinary bool
	Document bool
	FlowErr  error // the first rule, line or stream body that Flow IR does not print, or the first internal error of the unread-variable check
	// TypeScript reports that the generator prints TypeScript, whose literal tables are shared only
	// with the optimization ts-literal-sharing.
	TypeScript bool
	// Only exact emitted bodies with the same signature share a helper. Rule
	// wrappers, memo IDs, frame layouts and expectation IDs remain in the code.
	methodNames  map[generatedMethod]string
	literalNames map[string]string
	methodKeys   []generatedMethod
	literalKeys  []string
	// Driver runs the pipelines of the IR with the optimizations that are on (GenOptions.Optimize, Opt
	// and NoOpt), the dumps of GenOptions.PrintAfter and the verification of GenOptions.VerifyEachPass.
	opt.Driver
	// Flow holds the Core IR of the typed rules while they are printed from Flow IR, or nil;
	// flowRules counts the rules printed from Flow IR as flat rules and flowFrames the bodies
	// and lines printed as matchers of runtime-owned frames.
	Flow  *FlowState
	Prog  *compile.Program
	Table string        // the rule table being generated: rules, or recRules for Recognize
	Cur   *compile.Rule // the rule being generated
	// Proj holds the projected repetition captures of the rule being generated (Projections).
	Proj    map[string]string
	Vars    strings.Builder
	Init    strings.Builder
	Methods strings.Builder
	n       int
	// Descs is the expectation table of the generated parser (descs in runtime); descIDs maps a
	// description to its ID.
	Descs   []string
	DescIDs map[string]compile.ExpID
	// FieldTypes names the field type variable of each checked constructed field, result and cast (by
	// *grammar.FieldInit or checkInfo.results key), numbered separately so that checks do not
	// rename other generated names (runtime/fieldcheck.go).
	FieldTypes map[any]string
	// fieldTypeKeys are the keys of fieldTypes in the order of their variables, for rollback.
	fieldTypeKeys []any
}

// FieldTypeVar returns the variable holding check type ft of key, declaring it with decl the
// first time.
func (g *Generator) FieldTypeVar(key any, ft compile.Ty, decl func(v string, t *compile.FieldType) string) string {
	v, ok := g.FieldTypes[key]
	if !ok {
		if g.FieldTypes == nil {
			g.FieldTypes = map[any]string{}
		}
		v = fmt.Sprintf("fieldType%d", len(g.FieldTypes)+1)
		g.FieldTypes[key] = v
		g.fieldTypeKeys = append(g.fieldTypeKeys, key)
		g.Vars.WriteString(decl(v, compile.NewFieldType(ft)))
	}
	return v
}

// UseSparseMemo removes the representation selector entirely when neither
// emitted rule table can cross the sparse-storage threshold.
func UseSparseMemo(o opt.Settings, prog, rec *compile.Program) bool {
	return o.On("sparse-memo") && (prog.NSeen > 64 || rec != nil && rec.NSeen > 64)
}

// GeneratedMaxDepth returns the maximum depth that a generated parser enforces: o.MaxDepth, or the
// default depth when it is zero.
func GeneratedMaxDepth(o GenOptions) int {
	if o.MaxDepth == 0 {
		return limits.DefaultMaxDepth
	}
	return o.MaxDepth
}

type generatedMethod struct {
	signature string
	body      string
}

// MethodName returns an existing exact helper or reserves the next name.
// The signature keeps Node, typed Go and TypeScript functions distinct.
func (g *Generator) MethodName(signature, body string) (string, bool) {
	if !g.Opt.On("method-sharing") {
		return g.Name("e"), false
	}
	key := generatedMethod{signature, body}
	if name, ok := g.methodNames[key]; ok {
		return name, true
	}
	if g.methodNames == nil {
		g.methodNames = make(map[generatedMethod]string)
	}
	name := g.Name("e")
	g.methodNames[key] = name
	g.methodKeys = append(g.methodKeys, key)
	return name, false
}

// LiteralName shares immutable literal tables by decoded text. Descriptions
// belong to each matcher, so spellings with different expectations stay distinct.
func (g *Generator) LiteralName(value string) (string, bool) {
	if !g.Opt.On("literal-sharing") || g.TypeScript && !g.Opt.On("ts-literal-sharing") {
		return g.Name("lit"), false
	}
	if name, ok := g.literalNames[value]; ok {
		return name, true
	}
	if g.literalNames == nil {
		g.literalNames = make(map[string]string)
	}
	name := g.Name("lit")
	g.literalNames[value] = name
	g.literalKeys = append(g.literalKeys, value)
	return name, false
}

// SeedDescs starts the table of expectation descriptions with the fixed ones.
func (g *Generator) SeedDescs() {
	if g.DescIDs != nil {
		return
	}
	g.DescIDs = map[string]compile.ExpID{}
	for i, d := range compile.FixedDescs {
		g.Descs = append(g.Descs, d)
		g.DescIDs[d] = compile.ExpID(i)
	}
}

// Desc returns the expectation ID of description s, adding it to the table.
func (g *Generator) Desc(s string) compile.ExpID {
	g.SeedDescs()
	if id, ok := g.DescIDs[s]; ok {
		return id
	}
	id := compile.ExpID(len(g.Descs))
	g.Descs = append(g.Descs, s)
	g.DescIDs[s] = id
	return id
}

// Name returns a fresh name: prefix followed by a number unique within the generator.
func (g *Generator) Name(prefix string) string {
	g.n++
	return fmt.Sprintf("%s%d", prefix, g.n)
}

// Projections returns the projected repetition captures of the rule r (see ruleProjections), none
// in the recognizer.
func (g *Generator) Projections(r *compile.Rule) map[string]string {
	if g.Table == "recRules" {
		return nil
	}
	return compile.RuleProjections(g.Prog, r)
}

// FlowState holds the Core IR of the typed rules while they are printed from Flow IR.
type FlowState struct {
	M      *core.Module
	Origin map[core.Node]grammar.Expr
	Opts   flowlower.Options
	// Rules holds the rules of the program by ID, value-free twins included.
	Rules []*compile.Rule
}

// CoreAction returns the action of Core IR of the rule r, the term that its value is made of, or nil
// if it has none or the table has no Core IR.
func (g *Generator) CoreAction(r *compile.Rule) core.Term {
	if g.Flow == nil || r.ID >= len(g.Flow.M.Rules) {
		return nil
	}
	cr := g.Flow.M.Rules[r.ID]
	if cr == nil || cr.Finish.Kind != core.FinishAction {
		return nil
	}
	return cr.Finish.Action
}

// FailFlow records that Flow IR does not print a part of a table, which fails the generation: every
// table is printed from Flow IR and has no other route.
func (g *Generator) FailFlow(what string) {
	if g.FlowErr == nil {
		g.FlowErr = fmt.Errorf("Flow IR does not print %s", what)
	}
}

// FlowNodeFrameOK reports whether the rule r of the Node or recognizer table is printed from Flow IR as
// a framed body (flowlower.NodeFrameOK), with its Core rule. The typed table has no framed bodies.
func (g *Generator) FlowNodeFrameOK(r *compile.Rule) (*core.Rule, bool) {
	if g.Flow == nil || g.Table == "trules" {
		return nil, false
	}
	cr := g.Flow.M.Rules[r.ID]
	if !flowlower.NodeFrameOK(r, cr) {
		return nil, false
	}
	return cr, true
}

// FlowPasses runs the pipeline of Flow passes on the function f that the lowering returned with err,
// and returns it. Every function that a generator prints from Flow IR goes through it, labeled with
// the table being generated ("typed" for the typed rules, as in the dumps of Core IR). An error of the
// lowering is returned; a failure of the passes is kept as the first internal error of the
// generation (FlowErr), which fails it, and the function is returned for the printing to go on.
func (g *Generator) FlowPasses(f *flow.Func, err error) (*flow.Func, error) {
	if err != nil {
		return nil, err
	}
	if err := g.RunFlow(g.irTable(), f, g.Observability); err != nil && g.FlowErr == nil {
		g.FlowErr = fmt.Errorf("function %s of table %s: %w", f.Name, g.irTable(), err)
	}
	return f, nil
}

// irTable is the name of the table being generated in the dumps of the IR: "typed" for the typed
// rules, as Core IR names it, and the name of the table otherwise.
func (g *Generator) irTable() string {
	if g.Table == "trules" {
		return "typed"
	}
	return g.Table
}

// FrameScope returns the scope of the frame whose slots the captures c name.
func FrameScope(c flow.Captures) *compile.Scope {
	s := compile.NewScope()
	for _, name := range c.Names {
		s.Slot(name)
	}
	return s
}

// StartRecFlow builds the Core IR of the program rec (the recognizer, or the program of the Node
// table), with the passes of the untyped tables (no terminal programs and no class runs), for
// printing its eligible rules from Flow IR. It runs the pipeline of Core passes in a mode that is not
// Lean: the rewrites, the guards and the scans, with the repeats that irbuild.FollowRepeats keeps in
// step with them. Terminals and class runs apply only to lean typed rules (gengo.Generator.TypedCore):
// the Node recognizer writes its terminals and classes as inline tests.
func (g *Generator) StartRecFlow(rec *compile.Program) error {
	origin := map[core.Node]grammar.Expr{}
	m, err := irbuild.CoreProjecting(rec, origin, g.Table != "recRules")
	if err != nil {
		return err
	}
	// The scans of the Flow route keep the origin of each repetition they replace (FollowRepeats).
	prepare := func(m *core.Module) func() {
		repeats := irbuild.FollowRepeats(m, origin)
		return func() {
			for _, f := range repeats {
				f()
			}
		}
	}
	if err := g.RunCore(g.Table, m, core.Mode{Ordinary: g.Ordinary, Observe: g.Observability}, nil, prepare); err != nil {
		return err
	}
	// Classes are inline tests, as the Node runtime's generated code writes them.
	opts := flowlower.Options{Budget: !g.NoBudget, Recovers: flowlower.Recovers(m), GuardedClasses: g.GuardedClasses(),
		InlineLiterals: g.Opt.Value("inline-literals"), Observe: g.Observability, Node: true, Document: g.Document}
	g.Flow = &FlowState{M: m, Origin: origin, Opts: opts, Rules: append(append([]*compile.Rule(nil), rec.RuleList...), rec.Twins...)}
	return nil
}

// GuardedClasses reports whether the first class of a guarded alternative consumes the character that
// the guard read: the parser prints bodies in their ordinary form, and the guards and their guarded
// classes are on.
func (g *Generator) GuardedClasses() bool {
	return g.Ordinary && g.Opt.On(core.PassGuards) && g.Opt.On(core.FlagGuardedClasses)
}

// CstJoin reports whether the join has members that stay *Node. The members are read as
// joinMembers writes them, so a join that is spelled with node[_] or error has the same answer as
// the alias that names it: node[_] stays a node, and error converts to an Error.
func CstJoin(u compile.JoinTy) bool {
	if m, ok := compile.JoinMembers(u).(compile.JoinTy); ok {
		u = m
	}
	for _, a := range u.Alts {
		switch a := a.(type) {
		case compile.NamedTy:
			if a.Kind == 'c' && a != compile.TyMatch && a != compile.TyError {
				return true
			}
		case compile.BasicTy, compile.ListTy, compile.RecordTy:
			return true
		}
	}
	return false
}
