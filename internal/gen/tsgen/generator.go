package tsgen

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/flow"
	"zeroparse.dev/zp/internal/ir/opt"
)

//go:embed tsrt/runtime.ts
var tsRuntimeSource string

//go:embed tsrt/budget.ts
var tsBudgetSource string

//go:embed tsrt/trace.ts
var tsTraceSource string

//go:embed tsrt/profile.ts
var tsProfileSource string

// tsRuntimeMarker starts the part of tsrt/runtime.ts that is copied into generated code.
const tsRuntimeMarker = "// --- Runtime ---\n"

// Generate generates the files of a TypeScript parser for g, compiling it with c, that behaves
// exactly like the engine (and the generated Go parsers): runtime.ts, with the runtime
// (RuntimeEmbed), parser.ts, with the code generated from the grammar, and with opts.Types
// types.ts, with the declarations of the grammar's types. RuntimeImport imports the runtime from
// opts.RuntimeModule instead of writing it, and RuntimeOnly writes only runtime.ts, for which g may
// be nil. gen.TypeScript describes the options.
//
// Each parsing expression becomes a function and actions and predicates become TypeScript
// expressions, as in the Go generator; the embedded runtime (tsrt) mirrors runtime. opts.Package
// is not used. opts.Types adds declarations and Node-to-value conversion entry points.
// maxLeaderBits is the number of leaders of each left-recursion cycle that have a bit in the present
// sets of the runtime's dependency memo (Rule.leaderBit), as in the Go runtime. The sign bit stays
// clear: a negative present set is the identity of a larger one (Parser.presentOf).
const maxLeaderBits = 31

func Generate(g *grammar.Grammar, c gen.Compiler, opts gen.GenOptions) ([]gen.GeneratedFile, error) {
	return GenerateParser(g, c, opts, nil)
}

// Hooks change how GenerateParser generates, for tests.
type Hooks struct {
	// Generator, if not nil, is called with the generator once it has generated.
	Generator func(*Generator)
	// FlowRun runs the Flow passes in place of flow.Run (opt.Driver.FlowRun), so that a test
	// generates with a pipeline of its own, for example one with a pass that breaks a function added.
	FlowRun func(table string, f *flow.Func, env *flow.Env, o flow.RunOptions) ([]string, error)
	// ExecRun runs the Exec passes in place of exec.RunPasses (opt.Driver.ExecRun), as FlowRun does
	// for the Flow passes.
	ExecRun func(table string, m *exec.Module, env *exec.Env, o exec.RunOptions) ([]string, error)
}

// GenerateParser generates the files of Generate with the hooks h, which may be nil.
func GenerateParser(g *grammar.Grammar, c gen.Compiler, opts gen.GenOptions, h *Hooks) ([]gen.GeneratedFile, error) {
	if h == nil {
		h = &Hooks{}
	}
	if opts.Runtime == gen.RuntimeOnly {
		return TsRuntimeFiles(), nil
	}
	if opts.Runtime == gen.RuntimeImport && opts.RuntimeModule == "" {
		return nil, fmt.Errorf("the TypeScript runtime module (the specifier the parser imports the runtime from) is required with the import runtime mode")
	}
	if opts.Runtime != gen.RuntimeImport && opts.RuntimeModule != "" {
		return nil, fmt.Errorf("the TypeScript runtime module can only be set with the import runtime mode")
	}
	if opts.Stream {
		return nil, fmt.Errorf("stream generation is only supported for Go")
	}
	if opts.TypedOnly {
		return nil, fmt.Errorf("typed-only generation is only supported for Go")
	}
	if err := opts.ValidateMaxDepth(true); err != nil {
		return nil, err
	}
	if err := opts.ValidateDefaultValue(); err != nil {
		return nil, err
	}
	settings, err := opts.Optimizations()
	if err != nil {
		return nil, err
	}
	printAfter, _ := opt.ParsePrintAfter(opts.PrintAfter)
	prog, recognizer, err := c(g, compile.Options{SourceInfo: opts.SourceInfo, DefaultValue: opts.DefaultValue, LeftRecursionBindings: opts.LeftRecursionBindings})
	if err != nil {
		return nil, err
	}
	if prog.HasMissing {
		return nil, compile.MissingUnsupported("generated parsers")
	}
	start, err := prog.StartRule(opts.Start)
	if err != nil {
		return nil, err
	}
	var rec *compile.Program
	if opts.Recognize {
		if rec, err = recognizer(); err != nil {
			return nil, err
		}
	}
	live, err := reachableRules(prog, rec, start)
	if err != nil {
		return nil, err
	}
	// The code is generated up to twice; only the last generation's Core IR is printed.
	var dumped *bytes.Buffer
	newGen := func(live *tsLive) (*Generator, error) {
		dumped = &bytes.Buffer{}
		tsg := &Generator{Generator: gen.Generator{Prog: prog, Table: "rules", Observability: opts.Observability, TypeScript: true, Opt: settings, VerifyEach: opts.VerifyEachPass, FlowRun: h.FlowRun, ExecRun: h.ExecRun}, plainUsed: map[*compile.Rule]bool{}, live: live, printed: map[string]map[int][]int{}}
		tsg.NoBudget = !opts.LimitControls
		if opts.PrintTo != nil {
			tsg.PrintAfter, tsg.PrintTo = printAfter, dumped
		}
		tsg.optionsType = "ParseOptions"
		if !opts.LimitControls {
			tsg.optionsType = "PlainParseOptions"
		}
		tsg.Desc(compile.FixedDescs[0]) // the fixed expectations come first
		tsg.rules()
		if tsg.FlowErr != nil {
			return nil, tsg.FlowErr
		}
		if tsg.reachErr != nil {
			return nil, tsg.reachErr
		}
		if rec != nil {
			tsg.Prog, tsg.Table = rec, "recRules"
			tsg.rules()
			tsg.Prog, tsg.Table = prog, "rules"
			if tsg.FlowErr != nil {
				return nil, tsg.FlowErr
			}
			if tsg.reachErr != nil {
				return nil, tsg.reachErr
			}
		}
		return tsg, nil
	}
	tsg, err := newGen(live)
	if err != nil {
		return nil, err
	}
	// The printed code can call fewer rules than the Core IR does (a call after an expression that
	// cannot match is not printed): the rules that only such calls reach are generated again without.
	if printed := tsg.printedReach(prog, rec, start); !printed.equal(live) {
		if tsg, err = newGen(printed); err != nil {
			return nil, err
		}
	}
	if h.Generator != nil {
		defer h.Generator(tsg)
	}
	if opts.PrintTo != nil {
		if _, err := opts.PrintTo.Write(dumped.Bytes()); err != nil {
			return nil, err
		}
	}

	var out strings.Builder
	fmt.Fprintf(&out, "const G = mkGrammar(%d, %v, %v, %v);\n\n", gen.GeneratedMaxDepth(opts), gen.UseSparseMemo(settings, prog, rec), opts.Observability, opts.LimitControls)
	fmt.Fprintf(&out, "/**\n * Parses the whole input with the rule %s. unit selects the position unit (CodePoints by default).\n", start.Name)
	out.WriteString(" * A string is parsed as its UTF-8 encoding would be (lone surrogates as U+FFFD), and bytes are\n")
	out.WriteString(" * decoded as UTF-8 (invalid sequences as U+FFFD, one per byte). If the parse recovered from errors\n")
	out.WriteString(" * with #recover, the result has both the node and SyntaxErrors.\n */\n")
	// With limit controls every parse is controlled: the entry points without options use the default
	// compute limit of the input, as parseWithOptions does with no limits.
	parseBody, ruleBody := fmt.Sprintf("run(G, R%d, nseen, input, unit)", start.ID), "run(G, r, nseen, input, unit)"
	if opts.LimitControls {
		parseBody, ruleBody = "parseWithOptions(input, { unit: unit ?? CodePoints })", "parseRuleWithOptions(name, input, { unit: unit ?? CodePoints })"
	}
	fmt.Fprintf(&out, "export function parse(input: string | Uint8Array, unit?: Unit): ParseResult {\n  return %s;\n}\n\n", parseBody)
	if opts.LimitControls {
		fmt.Fprintf(&out, `/** Parses the whole input with the start rule name, a rule marked @start; "" selects the default start rule. */
export function parseRule(name: string, input: string | Uint8Array, unit?: Unit): ParseResult {
  return %s;
}

`, ruleBody)
	} else {
		fmt.Fprintf(&out, `/** Parses the whole input with the start rule name, a rule marked @start; "" selects the default start rule. */
export function parseRule(name: string, input: string | Uint8Array, unit?: Unit): ParseResult {
  const r = startIn(rules, ruleNames, starts, defaultStart, name);
  if (r instanceof Error) return { node: null, error: r };
  return %s;
}

`, ruleBody)
	}
	if rec != nil {
		fmt.Fprintf(&out, "/**\n * Checks that the whole input matches the rule %s without building a tree, and returns the syntax\n", start.Name)
		out.WriteString(" * error parse would return (SyntaxErrors for errors recovered with #recover), or null. Actions needed by\n")
		out.WriteString(" * predicates are evaluated and can report runtime errors; other actions are skipped.\n */\n")
		recBody := fmt.Sprintf("run(G, Q%d, recNseen, input, unit).error", rec.ByName[start.Name].ID)
		if opts.LimitControls {
			recBody = "recognizeWithOptions(input, { unit: unit ?? CodePoints })"
		}
		fmt.Fprintf(&out, "export function recognize(input: string | Uint8Array, unit?: Unit): Error | null {\n  return %s;\n}\n\n", recBody)
	}
	fmt.Fprintf(&out, `/** Parses with invocation-local position and depth options. */
export function parseWithOptions(input: string | Uint8Array, options?: %[1]s): ParseResult {
  const o = resolveOptions(G, options, input);
  if (o instanceof Error) return { node: null, error: o };
  return withObservedOptions(o, control => run(G, R%[2]d, nseen, input, o.unit, o.maxDepth, control, o), error => ({ node: null, error }));
}

/** Parses a named start rule ("" for the default) with invocation-local options, validated before rule lookup. */
export function parseRuleWithOptions(name: string, input: string | Uint8Array, options?: %[1]s): ParseResult {
  const o = resolveOptions(G, options, input);
  if (o instanceof Error) return { node: null, error: o };
  const r = startIn(rules, ruleNames, starts, defaultStart, name);
  if (r instanceof Error) return { node: null, error: r };
  return withObservedOptions(o, control => run(G, r, nseen, input, o.unit, o.maxDepth, control, o), error => ({ node: null, error }));
}

`, tsg.optionsType, start.ID)
	if rec != nil {
		fmt.Fprintf(&out, `/** Recognizes with invocation-local options; predicate-dependent actions may run. */
export function recognizeWithOptions(input: string | Uint8Array, options?: %[1]s): Error | null {
  const o = resolveOptions(G, options, input);
  if (o instanceof Error) return o;
  return withObservedOptions<Error | null>(o, control => run(G, Q%[2]d, recNseen, input, o.unit, o.maxDepth, control, o).error, error => error);
}

`, tsg.optionsType, rec.ByName[start.Name].ID)
	}
	var types string
	if opts.Types {
		var typed string
		if types, typed, err = tsg.typedTS(start); err != nil {
			return nil, err
		}
		out.WriteString(typed)
	}
	if opts.Document {
		tsg.document(&out, start, opts.Types)
	}
	if len(tsg.FieldTypes) > 0 {
		out.WriteString(tsg.fieldCheckSource())
	}
	out.WriteString(tsg.Vars.String())
	if opts.Expectations {
		fmt.Fprintf(&out, "\nG.descs = %s;\nG.expectations = true;\n", TsStrings(tsg.Descs))
	}
	if prog.RecordsSourceInfo {
		out.WriteString("G.sourceInfo = true;\n")
	}
	for _, v := range prog.Vars.All() {
		switch x := v.Init.(type) {
		case int:
			fmt.Fprintf(&out, "G.initialVars.push([%s, %s]);\n", TsString(v.Name), tsInt(x))
		case string:
			fmt.Fprintf(&out, "G.initialVars.push([%s, %s]);\n", TsString(v.Name), TsString(x))
		default:
			fmt.Fprintf(&out, "G.initialVars.push([%s, %v]);\n", TsString(v.Name), x)
		}
	}
	out.WriteString("\n")
	out.WriteString(tsg.Init.String())
	out.WriteString("\n")
	out.WriteString(tsg.Methods.String())
	return tsFiles(opts, out.String(), types), nil
}

// TsPrintRuns returns the code points that strconv.IsPrint accepts, for the runtime's quoteGo: the
// lengths of the alternating runs of code points that it rejects and accepts, from U+0000, in base
// 36 separated by commas (about 3 KB). Writing the table of the Go that generates the parser, rather
// than relying on the JavaScript runtime's Unicode properties, makes toString quote as the engine's
// Node.String does even where their Unicode versions differ.
var TsPrintRuns = sync.OnceValue(func() string {
	var b strings.Builder
	prev, start := false, 0
	for r := 0; r <= unicode.MaxRune; r++ {
		if p := strconv.IsPrint(rune(r)); p != prev {
			if b.Len() > 0 {
				b.WriteByte(',')
			}
			b.WriteString(strconv.FormatInt(int64(r-start), 36))
			prev, start = p, r
		}
	}
	return b.String()
})

// fieldCheckSource returns the run-time type check the parser uses: castFailure if called, which
// the runtime's fieldConforms serves.
func (g *Generator) fieldCheckSource() string {
	const f = "castFailure"
	if strings.Contains(g.Init.String(), f+"(") || strings.Contains(g.Vars.String(), f+"(") || strings.Contains(g.Methods.String(), f+"(") {
		return "\n" + tsCastFailure + "\n"
	}
	return ""
}

// tsFieldTypeDecl declares the check type variable v.
func tsFieldTypeDecl(v string, t *compile.FieldType) string {
	return fmt.Sprintf("const %s: FieldType = %s;\n", v, tsFieldType(t))
}

// tsFieldType writes a field type as a generated FieldType value.
func tsFieldType(t *compile.FieldType) string {
	elem := "null"
	if t.Elem != nil {
		elem = tsFieldType(t.Elem)
	}
	alts := make([]string, len(t.Alts))
	for i, a := range t.Alts {
		alts[i] = tsFieldType(a)
	}
	return fmt.Sprintf("{ kind: %s, name: %s, elem: %s, alts: [%s], text: %s }", TsString(string(rune(t.Kind))), TsString(t.Name), elem, strings.Join(alts, ", "), TsString(t.Text))
}

// Generator generates TypeScript. It shares the expectation table, names and analyses of the Go
// generator through gen.Generator.
type Generator struct {
	gen.Generator
	// plainUsed holds the plain rules whose function (plainCall) is called.
	plainUsed map[*compile.Rule]bool
	// optionsType is the name of the runtime's options type that the entry points take: ParseOptions
	// with limit controls, PlainParseOptions without.
	optionsType string
	// live holds the rules that a parse can enter (reach.go).
	live     *tsLive
	reachErr error
	// printed holds, for each table, the rules that the printed code of each rule calls (by ID).
	printed map[string]map[int][]int
	// startType and startRepr are, with types, the TypeScript type of the start rule's value and the
	// recipe that converts it (typedTS).
	startType string
	startRepr int
	// ExecFuncs counts the functions printed from Exec IR (execRoute).
	ExecFuncs int
	// LoweredTerms counts the terms printed from Exec IR (termStmts).
	LoweredTerms int
}

// TsString returns s as a TypeScript string literal (JSON is valid TypeScript).
func TsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// TsStrings returns xs as a TypeScript array literal of strings.
func TsStrings(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = TsString(x)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// TsInts returns xs as a TypeScript array literal of numbers.
func TsInts[T ~int32 | ~byte](xs []T) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = strconv.Itoa(int(x))
	}
	return "[" + strings.Join(q, ", ") + "]"
}

func tsJoinCondition(parts []string, op string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	mid := len(parts) / 2
	return "(" + tsJoinCondition(parts[:mid], op) + op + tsJoinCondition(parts[mid:], op) + ")"
}

func (g *Generator) seenVar() string {
	if g.Table == "rules" {
		return "nseen"
	}
	return "recNseen"
}

// ruleVar returns the constant that holds the rule r of the table being generated.
func (g *Generator) ruleVar(r *compile.Rule) string {
	if g.Table == "rules" {
		return fmt.Sprintf("R%d", r.ID)
	}
	return fmt.Sprintf("Q%d", r.ID)
}

func (g *Generator) plainPrefix() string {
	if g.Table == "rules" {
		return "r"
	}
	return "q"
}

// Method emits a function of an expression with the given body and returns its name.
//
// fn turns a call expression into a function.
func (g *Generator) Method(comment, body string) string {
	if !g.NoBudget {
		body = "  p.spend();\n" + body
	}
	name, shared := g.MethodName("Parser/R", body)
	if shared {
		return name
	}
	if comment != "" {
		fmt.Fprintf(&g.Methods, "// %s\n", comment)
	}
	param := "p"
	if !strings.Contains(body, "p.") && !strings.Contains(body, "(p)") {
		param = "_p" // not reported as unused (the body is in generated code only, without user text)
	}
	fmt.Fprintf(&g.Methods, "function %s(%s: Parser): R {\n%s}\n\n", name, param, body)
	return name
}

// fn returns a function (a Matcher) that does what the call expression x does.
func (g *Generator) fn(x string) string {
	if name, ok := strings.CutSuffix(x, "(p)"); ok && !strings.ContainsAny(name, "(). ") {
		return name
	}
	return g.Method("", "  return "+x+";\n")
}

// tsActxUse matches a use of the evaluation context c in the text of a term.
var tsActxUse = regexp.MustCompile(`\bc\b`)

// tsActxParam returns the name of the evaluation context parameter of a function that returns the term
// body: c, or _c if the term reads no context (a leading _ keeps it from being reported as unused).
func tsActxParam(body string) string {
	if tsActxUse.MatchString(body) {
		return "c"
	}
	return "_c"
}

// action emits a function that evaluates the term ct of Core IR (an action or a predicate) and
// returns its name.
func (g *Generator) action(ct core.Term, s *compile.Scope, locals map[string]string) string {
	name := g.Name("a")
	g.Methods.WriteString(g.actionFunc(name, ct, s, locals))
	return name
}

func (g *Generator) rules() {
	g.startTSFlow()
	init := &g.Init
	bodies := map[*compile.Rule]string{}
	scopes := map[*compile.Rule]*compile.Scope{}
	var all []*compile.Rule
	for _, r := range append(append([]*compile.Rule(nil), g.Prog.RuleList...), g.Prog.Twins...) {
		if !g.pruned(r) {
			all = append(all, r)
		}
	}
	for _, r := range all {
		s := compile.NewScope()
		scopes[r] = s
		if _, ok := r.Def.Expr.(*grammar.OperatorsRule); !ok {
			g.Cur, g.Proj = r, g.Projections(r)
			if call, ok := g.flowRule(r, s); ok {
				bodies[r] = call
			} else {
				g.FailFlow("rule " + r.Name + " of table " + g.Table)
				bodies[r] = "null" // the generation fails with the error g.failFlow records
			}
			g.Proj = nil
		}
	}
	// Each rule is a constant, which the generated code refers to.
	var vars []string
	leaders := map[int]int{}
	for _, r := range all {
		if !g.tableRow(r) {
			continue
		}
		value := tsShaper(g.Prog, r.BodyExpr())
		if _, ok := r.Def.Expr.(*grammar.OperatorsRule); ok {
			value = 0 // the lines build the rule's value
		}
		fmt.Fprintf(init, "const %s = mkRule(%d, %s, %s, %d, %v, %v, %d, %v, %s);\n", g.ruleVar(r),
			r.ID, TsString(r.Name), TsStrings(scopes[r].Names), value, r.Memo && !r.Transient, r.Leader, r.Seen, r.Novalue, tsIntList(g.Prog.Vars.Slots(r.Vars)))
		vars = append(vars, g.ruleVar(r))
		if r.Splice {
			fmt.Fprintf(init, "%s.splice = true;\n", g.ruleVar(r))
		}
		if r.Rec {
			fmt.Fprintf(init, "%s.rec = true;\n", g.ruleVar(r))
		}
		if r.Cycle != 0 {
			fmt.Fprintf(init, "%s.cycle = %d;\n", g.ruleVar(r), r.Cycle)
			if r.Leader {
				// The leaders of each cycle are numbered, and the first maxLeaderBits have a bit in
				// present sets.
				k := leaders[r.Cycle]
				if k > 0 {
					fmt.Fprintf(init, "%s.leaderIndex = %d;\n", g.ruleVar(r), k)
				}
				if k < maxLeaderBits {
					fmt.Fprintf(init, "%s.leaderBit = %d;\n", g.ruleVar(r), 1<<k)
				}
				leaders[r.Cycle]++
			}
		}
		if w := r.WhereName(); w != r.Name {
			fmt.Fprintf(init, "%s.where = %s;\n", g.ruleVar(r), TsString(w))
		}
		if r.NodeType != "" {
			fmt.Fprintf(init, "%s.nodeType = %s;\n", g.ruleVar(r), TsString(r.NodeType))
		}
		if r.OpNodeType != "" {
			fmt.Fprintf(init, "%s.opNodeType = %s;\n", g.ruleVar(r), TsString(r.OpNodeType))
		}
	}
	if g.Table == "rules" {
		// The table and the start rules, for parseRule (the recognizer has none).
		fmt.Fprintf(init, "const rules: Rule[] = [%s];\n", strings.Join(vars, ", "))
		var names []string
		for _, r := range g.Prog.RuleList {
			names = append(names, r.Name)
		}
		fmt.Fprintf(init, "const ruleNames = %s;\n", TsStrings(names))
		fmt.Fprintf(init, "const starts = %s;\n", TsStrings(g.Prog.StartRules))
		fmt.Fprintf(init, "const defaultStart = %s;\n", TsString(g.Prog.DefaultStartRule))
	}
	fmt.Fprintf(init, "const %s = %d;\n", g.seenVar(), g.Prog.NSeen)
	for _, r := range all {
		if pr, ok := r.Def.Expr.(*grammar.OperatorsRule); ok {
			g.Cur = r
			g.operatorsRule(r, pr)
			if r.Action != nil && !r.Novalue {
				// The whole-rule action, which the runtime runs on the folded value after each call.
				locals := map[string]string{compile.OperatorsRuleValue: "c.value"}
				fmt.Fprintf(init, "%s.action = %s;\n", g.ruleVar(r), g.action(g.CoreAction(r), compile.NewScope(), locals))
			}
			continue
		}
		if g.slotReached(r) {
			fmt.Fprintf(init, "%s.body = %s;\n", g.ruleVar(r), g.fn(bodies[r]))
		}
		if r.Action != nil && g.tableRow(r) {
			g.Proj = g.Projections(r)
			fmt.Fprintf(init, "%s.action = %s;\n", g.ruleVar(r), g.action(g.CoreAction(r), scopes[r], nil))
			g.Proj = nil
		}
	}
	// The functions of plain rules that are called (all calls are generated by now).
	for _, r := range all {
		if g.plainUsed[r] {
			g.plainCall(r, bodies[r])
		}
	}
	if g.Table != "rules" {
		return // the recognizer shares the struct fields
	}
	// Struct fields, used to detect references to undeclared fields at run time.
	var names []string
	for name, spec := range g.Prog.Types {
		if td, ok := spec.(*grammar.TypeDef); ok && td.Struct() != nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		sz := g.Prog.StructZeroOf(name)
		var fields []string
		for _, f := range sz.Names {
			fields = append(fields, "["+TsString(f)+", "+TsString(sz.Zeros[f])+"]")
		}
		fmt.Fprintf(init, "G.structFields.set(%s, structFieldsOf([%s]));\n", TsString(name), strings.Join(fields, ", "))
	}
}

// plainCall writes the function r<id>, which does what invokePlain does for the rule r (a rule never
// memoized and without captures) with a direct call of its body.
func (g *Generator) plainCall(r *compile.Rule, body string) {
	m := &g.Methods
	name := g.tracePlainCall(r, fmt.Sprintf("%s%d", g.plainPrefix(), r.ID))
	fmt.Fprintf(m, "// %s, called as by invokePlain\nfunction %s(p: Parser): R {\n", r.Name, name)
	if !g.NoBudget {
		m.WriteString("  p.spend();\n")
	}
	m.WriteString(`  const start = p.pos;
  const rec = p.recovered.length;
  const trail = p.trail.length;
  const prevEnv = p.env;
  const prevCut = p.cut;
  p.cut = false;
`)
	if r.Rec {
		m.WriteString("  if (++p.depth > p.maxDepth) {\n    p.tooDeep();\n  }\n")
	}
	fmt.Fprintf(m, "  let v = %s;\n", body)
	if r.Rec {
		m.WriteString("  p.depth--;\n")
	}
	m.WriteString(`  p.cut = prevCut;
  p.trail.length = trail;
  if (v !== undefined) {
`)
	if r.Novalue {
		m.WriteString("    v = null;\n")
	} else {
		fmt.Fprintf(m, "    v = p.finish(%s, emptyFrame, v, start);\n", g.ruleVar(r))
	}
	m.WriteString(`  } else {
    p.pos = start;
    p.recovered.length = rec;
  }
  p.env = prevEnv;
  return v;
}

`)
}

// tsShaper returns the runtime shaper of the expression e, its static shape (internal/compile/value_shape.go), packed
// as the runtime's Shaper.
func tsShaper(prog *compile.Program, e grammar.Expr) int {
	s := int(compile.SingleModeOf(prog, e))<<2 | compile.ValuedCount(prog, e)<<5
	if compile.IsElems(prog, e) {
		s |= 1
	}
	if compile.ScopeTuple(prog, e) {
		s |= 2
	}
	if compile.IsElems(prog, e) && compile.SharesTuple(prog, e) {
		s |= 16
	}
	return s
}

func (g *Generator) operatorsRule(r *compile.Rule, e *grammar.OperatorsRule) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s.operatorsRule = {\n", g.ruleVar(r))
	// The lines of the rule's Core IR, in the order of e, if the printer prints them from Flow IR.
	var co *core.OperatorsRule
	if g.Flow != nil {
		co, _ = g.Flow.M.Rules[r.ID].Body.(*core.OperatorsRule)
	}
	if e.Skip != nil {
		var skip *core.OpLine
		if co != nil {
			skip = co.Skip
		}
		m, ok := g.flowLine(skip, e.Skip, compile.NewScope(), false)
		if !ok {
			g.FailFlow("the skip of an Operators Rule of rule " + r.Name)
			m = "null"
		}
		fmt.Fprintf(&b, "  skip: %s,\n", g.fn(m))
	} else {
		b.WriteString("  skip: null,\n")
	}
	b.WriteString("  operands: [\n")
	for i, o := range e.Operands {
		var l *core.OpLine
		if co != nil {
			l = co.Operands[i]
		}
		fmt.Fprintf(&b, "    %s,\n", g.operatorsRuleLine(o.Expr, o.Action, false, l))
	}
	b.WriteString("  ],\n")
	var prefix, led []string
	id := 0
	for i, l := range e.Levels {
		for _, op := range l.Operators {
			var line *core.OpLine
			if co != nil {
				line = co.Ops[id].Line
			}
			s := fmt.Sprintf("    { id: %d, kind: %s, assoc: %s, level: %d, line: %s },\n", id, TsString(op.Kind), TsString(op.Assoc), i+1, g.operatorsRuleLine(op.Expr, op.Action, true, line))
			id++
			if op.Kind == grammar.Prefix {
				prefix = append(prefix, s)
			} else {
				led = append(led, s)
			}
		}
	}
	fmt.Fprintf(&b, "  prefix: [\n%s  ],\n", strings.Join(prefix, ""))
	fmt.Fprintf(&b, "  led: [\n%s  ],\n", strings.Join(led, ""))
	b.WriteString("};\n")
	fmt.Fprintf(&b, "%s.body = (p: Parser, min: number): R => p.operatorsRuleParse(%s, min);\n", g.ruleVar(r), g.ruleVar(r))
	g.Init.WriteString(b.String())
}

// operatorsRuleLine returns the line of an Operators Rule with the expression e and the action; l is
// its Core IR if the printer prints it from Flow IR, or nil.
func (g *Generator) operatorsRuleLine(e grammar.Expr, action grammar.Term, operator bool, l *core.OpLine) string {
	s := compile.NewScope()
	var locals []string
	if operator {
		locals = []string{"lhs", "rhs", "op"}
	}
	// Lines whose value is unused are matched without values, as in the engine; so are all lines
	// of a rule without a value (in the recognizer).
	build := !g.Cur.Novalue && !compile.LeanLine(action, locals)
	m, ok := g.flowLine(l, e, s, build)
	if !ok {
		g.FailFlow("a line of an Operators Rule of rule " + g.Cur.Name)
		m = "null"
	}
	m = g.fn(m)
	act := "null"
	if action != nil && !g.Cur.Novalue {
		var locals map[string]string
		if operator {
			locals = map[string]string{"lhs": "c.lhs", "rhs": "c.rhs", "op": "c.op"}
		}
		var ct core.Term
		if l != nil {
			ct = l.Action
		}
		act = g.action(ct, s, locals)
	}
	return fmt.Sprintf("{ scope: %s, m: %s, action: %s, value: %d }", TsStrings(s.Names), m, act, tsShaper(g.Prog, e))
}

// tsInt returns the TypeScript value of the int v: a number when it is a safe integer, otherwise a
// bigint (see the runtime's ints).
func tsInt(v int) string {
	const maxSafe = 1<<53 - 1
	if int64(v) >= -maxSafe && int64(v) <= maxSafe {
		return strconv.Itoa(v)
	}
	return "BigInt(\"" + strconv.Itoa(v) + "\")"
}

// tsIntList returns xs as a TypeScript array literal of numbers.
func tsIntList(xs []int) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = strconv.Itoa(x)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// document writes the Document of the start rule: createDocument and, with types, the Document's
// parseAST, which converts the Node result as parseAST does.
func (g *Generator) document(out *strings.Builder, start *compile.Rule, types bool) {
	fmt.Fprintf(out, `/**
 * Document is an incremental Document of the start rule %[1]s: a text that edit changes, and parse,
 * which returns what parse of the whole current text returns. It is mutable and parses its whole
 * text on each parse.
 */
export class Document extends DocumentBase {
`, start.Name)
	if types {
		fmt.Fprintf(out, `  /** parseAST parses the current text and converts its completed Node result, as parseAST does. */
  parseAST(options?: %[1]s): Types.TypedParseResult<%[2]s> {
    return this.parseWith<Types.TypedParseResult<%[2]s>>(options, (r, control) => astResult<%[2]s>(r, %[3]d, control), error => ({ value: null, error }));
  }
`, g.optionsType, g.startType, g.startRepr)
	}
	fmt.Fprintf(out, `}

/**
 * createDocument makes a Document of input, which it copies, with options as the defaults of its
 * parses; document is null when the options are invalid, with the error.
 */
export function createDocument(input: string | Uint8Array, options?: %[1]s): { document: Document | null; error: Error | null } {
  return createDocumentBase(G, input, options, (bytes, defaults, o) => new Document(G, R%[2]d, nseen, bytes, defaults, o));
}

`, g.optionsType, start.ID)
}
