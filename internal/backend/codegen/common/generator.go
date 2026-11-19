package common

import (
	"fmt"
	"slices"
	"strings"

	"zeroparse.dev/zp/internal/frontend/check"
	irbuild "zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/frontend/types"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/execlower"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/internal/middleend/flowpass"
	"zeroparse.dev/zp/internal/pass"
	"zeroparse.dev/zp/internal/runtime/limits"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/grammar/ast"
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
	// helpers with an Ordinary form, no charges and inline class tests. The Flow passes choose the lean
	// forms of such functions (terminal programs, class runs and tables, guarded classes) that the
	// backend prints (FlowTarget).
	Ordinary bool
	// FlowTarget says which forms of the Flow passes the backend prints (flowpass.Target).
	FlowTarget flowpass.Target
	Document   bool
	// Traced reports the traced pass of a Go parser generated with observability: it prints the copies
	// of the rule tables that an observed parse runs (named with TracedSuffix), whose bodies report the
	// rule calls and record, in the primitives that read the input, how far each call examined it.
	Traced  bool
	FlowErr error // the first rule, line or stream body that Flow IR does not print, or the first internal error of the unread-variable check
	// TypeScript reports that the generator prints TypeScript, whose literal tables are shared only
	// with the optimization ts-literal-sharing.
	TypeScript bool
	// Only exact emitted bodies with the same signature share a helper. Rule
	// wrappers, memo IDs, frame layouts and expectation IDs remain in the code.
	methodNames  map[generatedMethod]string
	literalNames map[string]string
	methodKeys   []generatedMethod
	literalKeys  []string
	// Runner runs the pipelines of the IR with the optimizations that are on, the dumps of
	// GenOptions.PrintAfter and the verification of GenOptions.VerifyEachPass (Passes.Runner). It is
	// required: it is also the only holder of the optimizations (Opt), so that the printers and the
	// passes cannot disagree on them.
	Runner Runner
	// Flow holds the Core IR of the typed rules while they are printed from Flow IR, or nil;
	// flowRules counts the rules printed from Flow IR as flat rules and flowFrames the bodies
	// and lines printed as matchers of runtime-owned frames.
	Flow  *FlowState
	Prog  *Program
	Table string // the rule table being generated: rules, or recRules for Recognize
	Cur   *Rule  // the rule being generated
	// Proj holds the projected repetition captures of the rule being generated (Projections).
	Proj    map[string]string
	Vars    strings.Builder
	Init    strings.Builder
	Methods strings.Builder
	n       int
	// Descs is the expectation table of the generated parser (descs in runtime); descIDs maps a
	// description to its ID.
	Descs   []string
	DescIDs map[string]program.ExpID
	// FieldTypes names the field type variable of each checked constructed field, result and cast (by
	// *ast.FieldInit or checkInfo.results key), numbered separately so that checks do not
	// rename other generated names (pkg/runtime/golib/fieldcheck.go).
	FieldTypes map[any]string
	// fieldTypeKeys are the keys of fieldTypes in the order of their variables, for rollback.
	fieldTypeKeys []any
}

// runner returns the Runner of the generator, and panics if there is none: a Generator is built
// with the Runner of its Passes, so a missing one is a programming error.
func (g *Generator) runner() Runner {
	if g.Runner == nil {
		panic("common.Generator has no Runner: set Generator.Runner to the Runner of its Passes before generating")
	}
	return g.Runner
}

// Opt returns the optimizations that are on: the settings of the Runner, the one holder of them.
func (g *Generator) Opt() pass.Settings { return g.runner().Settings() }

// RunCore runs the Core passes through the Runner.
func (g *Generator) RunCore(table string, m *core.Module, prepareScans func(*core.Module) func()) error {
	return g.runner().RunCore(table, m, prepareScans)
}

// FieldTypeVar returns the variable holding check type ft of key, declaring it with decl the
// first time.
func (g *Generator) FieldTypeVar(key any, ft types.Ty, decl func(v string, t *types.FieldType) string) string {
	v, ok := g.FieldTypes[key]
	if !ok {
		if g.FieldTypes == nil {
			g.FieldTypes = map[any]string{}
		}
		v = fmt.Sprintf("fieldType%d", len(g.FieldTypes)+1)
		g.FieldTypes[key] = v
		g.fieldTypeKeys = append(g.fieldTypeKeys, key)
		g.Vars.WriteString(decl(v, types.NewFieldType(ft)))
	}
	return v
}

// UseSparseMemo removes the representation selector entirely when neither
// emitted rule table can cross the sparse-storage threshold.
func UseSparseMemo(o pass.Settings, prog, rec *Program) bool {
	return o.On("sparse-memo") && (prog.Run.NSeen > 64 || rec != nil && rec.Run.NSeen > 64)
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
	if !g.Opt().On("method-sharing") {
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
	if !g.Opt().On("literal-sharing") || g.TypeScript && !g.Opt().On("ts-literal-sharing") {
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
	g.DescIDs = map[string]program.ExpID{}
	for i, d := range core.FixedDescs {
		g.Descs = append(g.Descs, d)
		g.DescIDs[d] = program.ExpID(i)
	}
}

// Desc returns the expectation ID of description s, adding it to the table.
func (g *Generator) Desc(s string) program.ExpID {
	g.SeedDescs()
	if id, ok := g.DescIDs[s]; ok {
		return id
	}
	id := program.ExpID(len(g.Descs))
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
func (g *Generator) Projections(r *Rule) map[string]string {
	if g.Table == "recRules" {
		return nil
	}
	return r.Projections
}

// FlowState holds the Core IR of the typed rules while they are printed from Flow IR.
type FlowState struct {
	M      *core.Module
	Origin map[core.Node]ast.Expr
	Opts   flowlower.Options
	// Rules holds the rules of the program by ID, value-free twins included.
	Rules []*Rule
}

// CoreAction returns the action of Core IR of the rule r, the term that its value is made of, or nil
// if it has none or the table has no Core IR.
func (g *Generator) CoreAction(r *Rule) core.ActionExpr {
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
// a framed body (NodeFrameOK), with its Core rule. The typed table has no framed bodies.
func (g *Generator) FlowNodeFrameOK(r *Rule) (*core.Rule, bool) {
	if g.Flow == nil || g.Table == "trules" {
		return nil, false
	}
	cr := g.Flow.M.Rules[r.ID]
	if !NodeFrameOK(r, cr) {
		return nil, false
	}
	return cr, true
}

// FlowPasses runs the Flow part of the pipeline (Runner.FlowFunc) on the function u of the table
// being generated, labeled "typed" for the typed rules as in the dumps of Core IR, and returns its
// module at the Flow stage. Every function that a generator prints goes through it. An error of the
// lowering is returned; a failure of the passes is kept as the first internal error of the
// generation (FlowErr), which fails it, and the module is returned for the printing to go on.
func (g *Generator) FlowPasses(u flowlower.Unit) (*code.Module, error) {
	m, err := g.runner().FlowFunc(g.IRTable(), u, g.Observability, g.FlowTarget)
	if m == nil {
		return nil, err
	}
	if err != nil && g.FlowErr == nil {
		g.FlowErr = fmt.Errorf("function %s of table %s: %w", flow.FuncOf(m).Name, g.IRTable(), err)
	}
	return m, nil
}

// ExecPasses runs the Exec part of the pipeline (Runner.ExecFunc) on the module m of one function
// that FlowPasses returned, with target, the pipeline of the backend's own Exec passes, labeled with
// the table being generated as FlowPasses labels it, and returns the lowered function. An error of
// the conversion to Exec IR is returned; a failure of the passes is kept as the first internal error
// of the generation (FlowErr), which fails it. The printers print l.Func with the locals l.Vars, so
// a pass keeps that function in the module and those locals in it (the pipeline contract of Exec
// IR); a run that breaks this is an internal error too.
func (g *Generator) ExecPasses(m *code.Module, target *TargetPipeline) (*execlower.Lowered, error) {
	l, err := g.runner().ExecFunc(g.IRTable(), m, target, g.Observability)
	if l == nil {
		return nil, err
	}
	if err == nil {
		err = keepsLowered(l)
	}
	if err != nil && g.FlowErr == nil {
		g.FlowErr = fmt.Errorf("module %s of table %s: %w", l.Module.Name, g.IRTable(), err)
	}
	return l, nil
}

// keepsLowered reports an error if the module of l no longer holds l.Func, or l.Func no longer
// declares a local of l.Vars.
func keepsLowered(l *execlower.Lowered) error {
	if !slices.Contains(l.Module.Funcs, l.Func) {
		return fmt.Errorf("internal error: an Exec pass replaced or removed the function %s", l.Func.Name)
	}
	for _, v := range l.Vars {
		if !slices.ContainsFunc(l.Func.Locals, func(p code.Param) bool { return p.Name == v }) {
			return fmt.Errorf("internal error: an Exec pass removed the local %s of the function %s", v, l.Func.Name)
		}
	}
	return nil
}

// IRTable is the name of the table being generated in the dumps of the IR: "typed" for the typed
// rules, as Core IR names it, and the name of the table otherwise; with TracedSuffix in the traced pass.
func (g *Generator) IRTable() string {
	if g.Table == "trules" {
		return "typed" + g.TracedSuffix()
	}
	return g.Table + g.TracedSuffix()
}

// TracedSuffix is the suffix of the names that the traced pass allocates (Traced), and "" otherwise.
func (g *Generator) TracedSuffix() string {
	if g.Traced {
		return "Traced"
	}
	return ""
}

// RecordsExamined reports whether the bodies being lowered record how far each call examines the
// input in the primitives that read it: those of a build with Documents, and the traced bodies.
func (g *Generator) RecordsExamined() bool { return g.Document || g.Traced }

// FrameScope returns the scope of the frame whose slots are names.
func FrameScope(names []string) *check.Scope {
	s := check.NewScope()
	for _, name := range names {
		s.Slot(name)
	}
	return s
}

// StartRecFlow builds the Core IR of the program rec (the recognizer, or the program of the Node
// table), with the passes of the untyped tables (no terminal programs and no class runs), for
// printing its eligible rules from Flow IR. It runs the pipeline of Core passes: the guards and the
// scans, with the repeats that irbuild.FollowRepeats keeps in step with them. Terminals and class runs
// apply only to lean typed rules (the Flow passes, flowpass.Target): the Node recognizer writes its
// terminals and classes as inline tests.
func (g *Generator) StartRecFlow(rec *Program) error {
	origin := map[core.Node]ast.Expr{}
	m, err := irbuild.CoreProjecting(rec.Elab, origin, g.Table != "recRules")
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
	if err := g.RunCore(g.IRTable(), m, prepare); err != nil {
		return err
	}
	// Classes are inline tests, as the Node library's generated code writes them.
	opts := flowlower.Options{Budget: !g.NoBudget, Recovers: flowlower.Recovers(m), Observe: g.Observability, Node: true,
		Document: g.RecordsExamined(), Traced: g.Traced}
	g.Flow = &FlowState{M: m, Origin: origin, Opts: opts, Rules: append(append([]*Rule(nil), rec.RuleList...), rec.Twins...)}
	return nil
}

// CstJoin reports whether the join has members that stay *Node. The members are read as
// joinMembers writes them, so a join that is spelled with node[_] or error has the same answer as
// the alias that names it: node[_] stays a node, and error converts to an Error.
func CstJoin(u types.JoinTy) bool {
	if m, ok := check.JoinMembers(u).(types.JoinTy); ok {
		u = m
	}
	for _, a := range u.Alts {
		switch a := a.(type) {
		case types.NamedTy:
			if a.Kind == 'c' && a != types.TyMatch && a != types.TyError {
				return true
			}
		case types.BasicTy, types.ListTy, types.RecordTy:
			return true
		}
	}
	return false
}
