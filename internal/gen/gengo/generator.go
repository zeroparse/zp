package gengo

import (
	"fmt"
	"go/format"
	"go/token"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gen/opt"
	"zeroparse.dev/zp/internal/ir/core"
)

// defaultPackageName returns the package name of the generated code for a module path: its last
// element, without a trailing major version element (example.com/m/v2 gives m), as Go tooling
// names a package.
func defaultPackageName(module string) string {
	base := path.Base(module)
	if dir := path.Dir(module); dir != "." && dir != "/" && len(base) >= 2 && base[0] == 'v' && base[1] != '0' && strings.Trim(base[1:], "0123456789") == "" {
		if base[1] != '1' || len(base) > 2 {
			return path.Base(dir)
		}
	}
	return base
}

// Generate generates the files of a Go parser for g, compiling it with c: parser.go with the code
// generated from the grammar, api.go, which re-exports the names the runtime defines for the users
// of the parser, with opts.Types types/types.go, the package of the Go types of the typed values,
// and, with opts.Runtime RuntimeEmbed, a copy of the runtime in internal/runtime. Paths are
// relative to the directory of the generated package. gen.Go describes the options.
//
// Eligible rule bodies inline their expressions; other expressions become Go functions. Actions
// and predicates become Go expressions, and the runtime (package runtime of this module) behaves
// exactly like the engine. Without opts.Stream, #stream is a plain repetition.
func Generate(g *grammar.Grammar, c gen.Compiler, opts gen.GenOptions) ([]gen.GeneratedFile, error) {
	if opts.Runtime == gen.RuntimeOnly {
		return runtimeFiles()
	}
	return GenerateParser(g, c, opts, nil)
}

// Hooks let a test observe or change the decisions of one generation. The zero value, like nil,
// changes nothing.
type Hooks struct {
	// Generator receives the generator of the pass that writes the output, before it prints, so
	// that the test can build the Core IR of the same options (Generator.TypedCore).
	Generator func(*Generator)
	// Reach receives the result of the first pass of GenerateParser, before the second pass; a test
	// removes a root from it to show that the output then fails.
	Reach func(*liveRules)
	// LeanRoute is called with the rule table and the rule for each rule that takes the lean route
	// in rules, in a Node or recognizer table, so that a test checks the route that is chosen, not
	// the text of the generated code.
	LeanRoute func(table string, r *compile.Rule)
	// FramedRoute is called like LeanRoute for each rule whose framed body is printed from Flow IR
	// (flowNodeFrameBody), in a Node or recognizer table.
	FramedRoute func(table string, r *compile.Rule)
	// OperatorsRoute is called like LeanRoute for each line of a Node or recognizer table that is
	// printed from Flow IR for an Operators Rule.
	OperatorsRoute func(table string, r *compile.Rule)
	// StreamRoute is called like LeanRoute for each ParseStream start rule whose stream body is
	// printed from Flow IR (flowStreamBody).
	StreamRoute func(table string, r *compile.Rule)
	// RefuseFlowRecRule makes flowRecRule refuse the rules it selects, so that the framed body that
	// takes their place is exercised. No manifest grammar reaches that fallback.
	RefuseFlowRecRule func(*compile.Rule) bool
	// FlowVarsMutant makes every variable of a printed function unread: the generated code must
	// then fail to compile, which shows that the tests of generated code catch a wrong decision.
	FlowVarsMutant bool
	// FlowVarsMutantUses ignores the reads of the values of assignments that stay, which leaves
	// variables that only they read undeclared: the generated code must then fail to compile. In the
	// functions printed from Exec IR (execRoute) it leaves undeclared the locals that no statement
	// other than an assignment reads.
	FlowVarsMutantUses bool
	// CoreRun runs the Core passes in place of core.Run (gen.Generator.CoreRun), so that a test
	// generates with a pipeline of its own, for example one with a pass that breaks the module added.
	CoreRun func(m *core.Module, env *core.Env, o core.RunOptions) ([]string, error)
}

// GenerateParser generates the files of Generate (other than for RuntimeOnly). It generates twice: the
// first pass records which rules of the Node, recognizer and typed tables a generated call or an
// entry point can reach (liveRules), and the second leaves out the others. A typed-only parser
// without Node rules has no Node or recognizer rule to leave out (keepTableRules); one that keeps
// Node rules for its converting entry points prints those that the entry points reach, like any
// other output.
func GenerateParser(g *grammar.Grammar, c gen.Compiler, opts gen.GenOptions, h *Hooks) ([]gen.GeneratedFile, error) {
	if h == nil {
		h = &Hooks{}
	}
	if opts.KeepUnreachable {
		return generateOnce(g, c, opts, h, nil, nil)
	}
	var lv liveRules
	if _, err := generateOnce(g, c, opts, h, nil, &lv); err != nil {
		return nil, err
	}
	if h.Reach != nil {
		h.Reach(&lv)
	}
	return generateOnce(g, c, opts, h, &lv, nil)
}

// generateOnce is one pass of GenerateParser. A pass with collect records the reachable rules into it;
// a pass with live omits the rules that are not reachable in live.
func generateOnce(g *grammar.Grammar, c gen.Compiler, opts gen.GenOptions, h *Hooks, live, collect *liveRules) (files []gen.GeneratedFile, err error) {
	if collect != nil {
		*collect = liveRules{}
	}
	if opts.Module == "" && (opts.Runtime == gen.RuntimeEmbed || opts.Package == "") {
		return nil, fmt.Errorf("module path is required")
	}
	derived := opts.Package == ""
	if derived {
		opts.Package = defaultPackageName(opts.Module)
	}
	if !token.IsIdentifier(opts.Package) || opts.Package == "_" {
		if derived {
			return nil, fmt.Errorf("the module path %q gives the package name %q, which is not a valid Go identifier; choose the package name with -pkg (PackageName)", opts.Module, opts.Package)
		}
		return nil, fmt.Errorf("invalid package name %q", opts.Package)
	}
	if opts.RecognizeOnly {
		switch {
		case opts.Types || opts.TypedOnly || opts.Document || opts.Stream:
			return nil, fmt.Errorf("recognize-only output generates only Recognize: it cannot have typed output, typed-only output, a Document or a stream")
		}
		opts.Recognize = true
	}
	if opts.TypedOnly {
		switch {
		case !opts.Types:
			return nil, fmt.Errorf("typed-only output needs typed output (-types)")
		case opts.Document || opts.Stream:
			return nil, fmt.Errorf("typed-only output cannot have a Document or a stream, which use the untyped Node (Recognize is allowed)")
		}
	}
	if opts.Types && opts.Module == "" {
		return nil, fmt.Errorf("module path is required for typed output: the parser imports the package module/types")
	}
	if err := opts.ValidateMaxDepth(false); err != nil {
		return nil, err
	}
	if err := opts.ValidateDefaultValue(); err != nil {
		return nil, err
	}
	settings, err := opts.Optimizations()
	if err != nil {
		return nil, err
	}
	// Only the pass that writes the output prints the Core IR.
	var printAfter []string
	if collect == nil {
		printAfter, _ = opt.ParsePrintAfter(opts.PrintAfter)
	}
	info, err := loadRuntime()
	if err != nil {
		return nil, err
	}
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
	if opts.Stream {
		if !start.Stream {
			return nil, fmt.Errorf("stream generation needs a #stream repetition at the top level of the start rule %s", start.Name)
		}
	}
	genGo := &Generator{typedOnly: (opts.TypedOnly || opts.RecognizeOnly) && !opts.KeepNodes, Observability: opts.Observability, Document: opts.Document, NoBudget: !opts.LimitControls, Ordinary: !opts.LimitControls && !opts.Document, Prog: prog, Table: "rules", Opt: settings, CoreRun: h.CoreRun, PrintAfter: printAfter, PrintTo: opts.PrintTo, VerifyEach: opts.VerifyEachPass, hooks: *h, live: live, collect: collect, keepTableRules: (opts.TypedOnly || opts.RecognizeOnly) && !opts.KeepNodes}
	if live != nil {
		genGo.SeedDescs()
		for _, d := range live.descs[len(genGo.Descs):] {
			genGo.DescIDs[d] = compile.ExpID(len(genGo.Descs))
			genGo.Descs = append(genGo.Descs, d)
		}
	}
	if opts.Stream {
		genGo.streamStart = start
	}
	genGo.Desc(compile.FixedDescs[0]) // the fixed expectations come first
	if h.Generator != nil && collect == nil {
		h.Generator(genGo)
	}
	if err := genGo.StartRecFlow(prog); err != nil {
		return nil, err
	}
	genGo.rules()
	genGo.Flow = nil
	if genGo.streamErr != nil {
		return nil, genGo.streamErr
	}
	if genGo.tableErr != nil {
		return nil, genGo.tableErr
	}
	if genGo.FlowErr != nil {
		return nil, genGo.FlowErr
	}
	var rec *compile.Program
	if opts.Recognize {
		// The recognizer: the program in which no rule builds a value, in a rule table of its own.
		if rec, err = recognizer(); err != nil {
			return nil, err
		}
		genGo.Prog, genGo.Table = rec, "recRules"
		if err := genGo.StartRecFlow(rec); err != nil {
			return nil, err
		}
		genGo.rules()
		genGo.Flow = nil
		genGo.Prog, genGo.Table = prog, "rules"
		if genGo.tableErr != nil {
			return nil, genGo.tableErr
		}
		if genGo.FlowErr != nil {
			return nil, genGo.FlowErr
		}
	}
	if collect != nil {
		// The expectations of the Node and recognizer tables are those the second pass keeps; the typed
		// code adds its own after them, and only for the rules it reaches.
		collect.descs = append([]string(nil), genGo.Descs...)
		if !opts.Types {
			return nil, genGo.finishReach(prog, rec)
		}
		typed, err := genGo.typedGo(start, opts.ConvertTypes)
		if err != nil {
			return nil, err
		}
		genGo.convertsNodes = typed.needNodes
		if opts.TypedOnly && typed.needNodes && !opts.KeepNodes && !opts.DisableKeepNodes {
			opts.KeepNodes = true
			return generateOnce(g, c, opts, h, live, collect)
		}
		return nil, genGo.finishReach(prog, rec)
	}
	fmt.Fprintf(&genGo.Init, "\tzpGrammar.Starts = %s\n", goStrings(prog.StartRules))
	fmt.Fprintf(&genGo.Init, "\tzpGrammar.DefaultStart = %q\n", prog.DefaultStartRule)
	if !opts.TypedOnly {
		genGo.writeDescs(opts)
	}
	if opts.Observability {
		genGo.Init.WriteString("\tzpGrammar.Observable = true\n")
	}
	if !opts.LimitControls {
		genGo.Init.WriteString("\tzpGrammar.NoBudgetControls = true\n")
	}
	if prog.RecordsSourceInfo {
		genGo.Init.WriteString("\tzpGrammar.SourceInfo = true\n")
	}
	if t := prog.Vars; t != nil {
		var names, vals []string
		for _, d := range t.List {
			names = append(names, d.Name)
			vals = append(vals, fmt.Sprintf("%#v", d.Init))
		}
		fmt.Fprintf(&genGo.Init, "\tzpGrammar.SetVars(%s, []any{%s})\n", goStrings(names), strings.Join(vals, ", "))
	}

	if rec != nil && !opts.TypedOnly {
		genGo.Init.WriteString("\tzpRecGrammar.Share(zpGrammar)\n")
	}

	var out strings.Builder
	if opts.Observability && opts.Document {
		out.WriteString("// ErrDocumentBusy is returned by a Parse, ParseAST or Edit of a Document that\n// is already parsing, such as one made by a trace callback of that parse.\n// Schedule the work after the parse returns, or use another Document.\nvar ErrDocumentBusy = DocumentBusy\n\n")
	}
	out.WriteString("// --- Generated code ---\n\n")
	if !opts.TypedOnly && !opts.RecognizeOnly {
		fmt.Fprintf(&out, "// Parse parses the whole input with the rule %s. Options select position units, depth, cancellation and compute limits.\n", start.Name)
		out.WriteString("// If the parse recovered from errors with #recover, it returns the node together with SyntaxErrors.\n")
		fmt.Fprintf(&out, "func Parse(input string, opts ...ParseOption) (*Node, error) { return RunParse(rules[%d], input, opts) }\n\n", start.ID)
	}
	if opts.Stream {
		fmt.Fprintf(&out, "// ParseStream parses the input read from r with the rule %s, passing each committed element of\n", start.Name)
		out.WriteString("// its #stream repetition to emit (nil if the element has no value) without retaining it. Committed\n// elements are final even if the parse fails later. A non-EOF error from r or an error returned by\n// emit ends the parse and is returned unchanged. A nil emit collects the repetition instead.\n")
		out.WriteString("func ParseStream(r io.Reader, emit func(*Node) error, opts ...ParseOption) error {\n\treturn RunParseStream(streamRule, r, emit, opts)\n}\n\n")
		out.WriteString(`// Iterate returns an iterator over the elements that ParseStream would pass to emit, parsing only as
// far as each call to Next needs. Options are validated before it returns; r is first read by Next.
// The parser's lookahead and buffering may read ahead of the returned elements.
func Iterate(r io.Reader, opts ...ParseOption) (*StreamIterator, error) {
	pull, err := IterateStream(zpGrammar, opts, func(emit func(*Node) error) error { return RunParseStream(streamRule, r, emit, opts) })
	if err != nil {
		return nil, err
	}
	return NewStreamIterator(pull), nil
}

`)
	}
	if opts.Document {
		fmt.Fprintf(&out, "// NewDocument creates an incremental Node document for the start rule %s.\n", start.Name)
		if opts.Types {
			fmt.Fprintf(&out, "func NewDocument(text string, opts ...ParseOption) (*Document, error) {\n\td, err := MakeDocument(rules[%d], text, opts)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn &Document{d}, nil\n}\n\n", start.ID)
		} else {
			fmt.Fprintf(&out, "func NewDocument(text string, opts ...ParseOption) (*Document, error) { return MakeDocument(rules[%d], text, opts) }\n\n", start.ID)
		}
	}
	if rec != nil {
		fmt.Fprintf(&out, "// Recognize checks that the whole input matches the rule %s without building a tree, and returns the\n", start.Name)
		out.WriteString("// syntax errors Parse would return (SyntaxErrors for errors recovered with #recover). Actions needed by\n// predicates are evaluated and can report runtime errors; other actions are skipped.\n")
		fmt.Fprintf(&out, "func Recognize(input string, opts ...ParseOption) error {\n\t_, err := RunParse(recRules[%d], input, opts)\n\treturn err\n}\n\n", rec.ByName[start.Name].ID)
		fmt.Fprintf(&out, "// recRules is the rule table of Recognize, and zpRecGrammar its grammar.\nvar recRules []*Rule\nvar zpRecGrammar = NewGrammar(%d)\n\n", gen.GeneratedMaxDepth(opts))
	}
	fmt.Fprintf(&out, "// rules is the rule table, and zpGrammar its grammar.\nvar rules []*Rule\nvar zpGrammar = NewGrammar(%d)\n\n", gen.GeneratedMaxDepth(opts))
	if !opts.TypedOnly && !opts.RecognizeOnly {
		out.WriteString(`// ParseRule parses the whole input with the start rule name, a rule marked @start; an empty
// name selects the grammar's default start rule.
func ParseRule(name, input string, opts ...ParseOption) (*Node, error) {
	o, err := ResolveOptions(zpGrammar, opts)
	if err != nil {
		return nil, err
	}
	r, err := zpGrammar.StartRule(name)
	if err != nil {
		return nil, err
	}
	return RunParseResolved(r, input, o)
}

`)
	}
	var typesBody string
	if opts.Types {
		typed, err := genGo.typedGo(start, opts.ConvertTypes)
		if err != nil {
			return nil, err
		}
		genGo.convertsNodes = typed.needNodes
		if opts.TypedOnly && typed.needNodes && !opts.KeepNodes && !opts.DisableKeepNodes {
			// A typed entry point converts the Node result of its rule, so the Node rule functions
			// are needed; generate again with them.
			opts.KeepNodes = true
			return generateOnce(g, c, opts, h, live, collect)
		}
		if opts.TypedOnly {
			// The expectations of the typed bodies are known only now.
			genGo.writeDescs(opts)
		}
		out.WriteString("\n// --- Typed values ---\n\n")
		if opts.Document {
			out.WriteString("// Document is an incremental document: it parses its text again after edits, reusing the\n// results of the parse before that it can.\ntype Document struct{ *" + runtimeAlias + ".Document }\n\n")
		}
		out.WriteString(typed.parser)
		out.WriteString("\n")
		typesBody = typesPreamble + typed.types
	}
	if rec != nil && opts.TypedOnly {
		// The recognizer grammar shares zpGrammar by copying, so Share follows the expectation table,
		// which a typed-only parser writes only after its typed code.
		genGo.Init.WriteString("\tzpRecGrammar.Share(zpGrammar)\n")
	}
	if err := genGo.finishReach(prog, rec); err != nil {
		return nil, err
	}
	out.WriteString(genGo.Vars.String())
	out.WriteString("\nfunc init() {\n")
	out.WriteString(genGo.Init.String())
	out.WriteString("}\n\n")
	out.WriteString(genGo.Methods.String())
	whole, err := format.Source([]byte("package " + opts.Package + "\n" + out.String()))
	if err != nil {
		return nil, fmt.Errorf("generated code does not compile: %w\n%s", err, out.String())
	}
	body := shakeVars(strings.TrimPrefix(string(whole), "package "+opts.Package+"\n"))
	var tp *typesPackage
	var typesFile []byte
	if opts.Types {
		if typesFile, tp, err = typesPackageFile(opts, typesBody); err != nil {
			return nil, err
		}
		// The one reference to the schema of the types package (see schemaName).
		body += "\nconst _ = " + tp.schema + "\n"
	}
	if opts.Runtime == gen.RuntimeImport {
		// The parser builds only against a runtime that implements the protocol it was generated
		// for: another runtime does not declare the constant.
		body += "\nconst _ = " + info.protocol + "\n"
	}
	if genGo.chunksInRuntime {
		body = chunkNames.Replace(body)
	}
	body, err = qualifyRuntime(runtimeImport(opts), body, info, tp)
	if err != nil {
		return nil, err
	}
	var file strings.Builder
	file.WriteString(gen.Header() + "\n\n")
	if !opts.OmitPackageDoc {
		fmt.Fprintf(&file, "// Package %s is a parser generated from a ZeroParse grammar.\n", opts.Package)
	}
	fmt.Fprintf(&file, "package %s\n\n%s", opts.Package, body)
	src, err := format.Source([]byte(file.String()))
	if err != nil {
		return nil, fmt.Errorf("generated code does not compile: %w", err)
	}
	if !opts.KeepComments {
		if src, err = trimComments(src); err != nil {
			return nil, fmt.Errorf("generated code does not compile: %w", err)
		}
	}
	api, err := apiSource(opts, info)
	if err != nil {
		return nil, err
	}
	files = []gen.GeneratedFile{{Path: "parser.go", Content: src}, {Path: "api.go", Content: []byte(api)}}
	if opts.Types {
		files = append(files, gen.GeneratedFile{Path: "types/types.go", Content: typesFile})
	}
	if opts.Runtime == gen.RuntimeEmbed {
		rt, err := runtimeFiles()
		if err != nil {
			return nil, err
		}
		files = append(files, rt...)
	}
	return files, nil
}

// goFieldType writes a field type as a generated fieldType value.
func goFieldType(t *compile.FieldType) string {
	var b strings.Builder
	fmt.Fprintf(&b, "&FieldType{Kind: %q, Text: %q", rune(t.Kind), t.Text)
	if t.Name != "" {
		fmt.Fprintf(&b, ", Name: %q", t.Name)
	}
	if t.Elem != nil {
		b.WriteString(", Elem: " + goFieldType(t.Elem))
	}
	if len(t.Alts) > 0 {
		alts := make([]string, len(t.Alts))
		for i, a := range t.Alts {
			alts[i] = goFieldType(a)
		}
		b.WriteString(", Alts: []*FieldType{" + strings.Join(alts, ", ") + "}")
	}
	return b.String() + "}"
}

// Generator generates Go. It shares the expectation table, names and analyses with the
// TypeScript generator through gen.Generator.
type Generator struct {
	gen.Generator
	directTypedDocument bool
	budgetClassNames    map[string]string
	budgetClassKeys     []string
	shaperNames         map[string]string
	shaperKeys          []string
	recoveryProg        map[*compile.Program]bool // whether a program can recover from errors
	tableErr            error                     // the rule table does not fit the form of runtime.RuleSpec
	hooks               Hooks                     // the zero value changes nothing
	// chunksInRuntime is set when the typed values take their storage from the runtime (Chunk).
	chunksInRuntime bool
	ruleTypesDone   bool // the types of the rule table entries are written
	FlowRules       int
	FlowFrames      int
	ExecFuncs       int            // the functions printed from Exec IR (execRoute)
	ExecTables      map[string]int // ExecFuncs by table
	LoweredTerms    int            // the terms printed from Exec IR (lowered)
	// NodeScopes records the frame layouts shared by typed rule metadata.
	NodeScopes map[*compile.Rule][]string
	// typedOnly leaves out the untyped Node family: the rule table keeps only the metadata the
	// typed table shares.
	typedOnly bool
	// caps maps capture names to the Go variables that hold them in a flat rule of the typed
	// runtime (see flowrule.go), instead of frame slots.
	caps map[string]string
	// streamStart is the start rule of ParseStream, whose separate body
	// (generated while streamBody is set) emits its #stream elements.
	streamStart *compile.Rule
	streamBody  bool
	streamErr   error
	// plainCalled holds the plain rules that a generated call refers to by their wrapper
	// (ruleCall). wrappers are the methods of the plain rules, which dropUnusedWrappers keeps only
	// for the rules in plainCalled.
	plainCalled map[*compile.Rule]bool
	wrappers    []wrapperSpan
	// live and collect are the passes of GenerateParser (reach.go): live leaves out the unreachable
	// rules of the Node and recognizer tables, collect records the rules that the first pass reaches.
	live, collect *liveRules
	// keepTableRules marks a typed-only parser without Node rules: its Node and recognizer tables
	// have no rule bodies to check; the typed table leaves its unreached rules out as usual.
	keepTableRules bool
	// convertsNodes reports that a typed entry point converts the Node result of its rule, which
	// needs the Node rule bodies.
	convertsNodes bool
	// typedRoots are the typed rules whose Entry a typed entry point runs, and typedEntry their IDs.
	typedRoots []ruleKey
	typedEntry map[int]bool
	bodySet    map[ruleKey]bool
	reachErr   error
}

func (g *Generator) literal(value string) string {
	name, shared := g.LiteralName(value)
	if !shared {
		fmt.Fprintf(&g.Vars, "var %s = []rune(%q)\n", name, value)
	}
	return name
}

func goStrings(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = strconv.Quote(x)
	}
	return "[]string{" + strings.Join(q, ", ") + "}"
}

// ruleTableLimits reports whether a rule table with nstrs lists of strings and nseen deferred
// memoization indexes fits the fields of runtime.RuleSpec (Scope and Vars are 16 bits, Seen 32).
func ruleTableLimits(nstrs, nseen int) error {
	if nstrs > math.MaxUint16+1 {
		return fmt.Errorf("the rule table needs %d distinct capture or variable lists, more than %d", nstrs, math.MaxUint16+1)
	}
	if nseen > math.MaxInt32 {
		return fmt.Errorf("the rule table needs %d deferred memoization indexes, more than %d", nseen, math.MaxInt32)
	}
	return nil
}

// saveRec returns the variables that a backtracking point saves, start and, if the program can
// recover, rec, followed by more; saveRecValues their values. restoreRec returns the statement that
// goes back to them.
func (g *Generator) saveRec(more string) string {
	if g.recovers() {
		return "start, rec" + more
	}
	return "start" + more
}

func (g *Generator) saveRecValues(more string) string {
	if g.recovers() {
		return "p.Pos, len(p.Recovered)" + more
	}
	return "p.Pos" + more
}

func (g *Generator) restoreRec(pos, rec string) string {
	if g.recovers() {
		return fmt.Sprintf("p.Restore(%s, %s)", pos, rec)
	}
	return fmt.Sprintf("p.Pos = %s", pos)
}

// recovers reports whether a parse of the program being generated can append to p.Recovered: only
// recovery (#recover) does. Without it p.Recovered stays empty, so
// the positions that backtracking restores need no saved length of it.
func (g *Generator) recovers() bool {
	if v, ok := g.recoveryProg[g.Prog]; ok {
		return v
	}
	found := false
	check := func(e grammar.Expr) {
		switch e := e.(type) {
		case *grammar.Recovery:
			found = true
		case *grammar.Attributed:
			for _, a := range e.Attrs {
				if a.Name == "recover" {
					found = true
				}
			}
		}
	}
	for _, rs := range [][]*compile.Rule{g.Prog.RuleList, g.Prog.Twins} {
		for _, r := range rs {
			compile.WalkExpr(r.Def.Expr, check)
			compile.WalkExpr(r.BodyExpr(), check)
		}
	}
	if g.recoveryProg == nil {
		g.recoveryProg = map[*compile.Program]bool{}
	}
	g.recoveryProg[g.Prog] = found
	return found
}

func (g *Generator) rules() {
	init := &g.Init
	methodsBefore := g.Methods.String()
	bodies := map[*compile.Rule]string{}
	scopes := map[*compile.Rule]*compile.Scope{}
	flats := map[*compile.Rule]*flatRule{}
	all := append(append([]*compile.Rule(nil), g.Prog.RuleList...), g.Prog.Twins...)
	for _, r := range all {
		s := compile.NewScope()
		scopes[r] = s
		if g.pruned(r) {
			// Not printed; its table row keeps the capture layout of the first pass.
			s.Names = append(s.Names, g.live.scopes[ruleKey{g.Table, r.ID, false}]...)
			continue
		}
		if g.Table == "trules" {
			// Typed flat rules need no separate body; Flow IR prints every typed rule.
			if d := g.flowTypedRule(r); d != nil {
				flats[r] = d
				continue
			}
		} else if gen.RecFlowEligible(r) {
			// flowRecRule returns nil for a rule whose printed function would read its start or context, and
			// for one whose types it cannot name; such a rule takes its framed body below and is called through
			// the rule table (plainCall).
			if d := g.flowRecRule(r); d != nil {
				if g.hooks.LeanRoute != nil && g.collect == nil {
					g.hooks.LeanRoute(g.Table, r)
				}
				flats[r] = d
				if g.slotReached(r) {
					bodies[r] = g.leanRuleBody(r, d)
				}
				continue
			}
		}
		if _, ok := r.Def.Expr.(*grammar.OperatorsRule); !ok {
			if g.Table != "trules" {
				if body := g.flowNodeFrameBody(r, s); body != "" {
					if g.hooks.FramedRoute != nil && g.collect == nil {
						g.hooks.FramedRoute(g.Table, r)
					}
					bodies[r] = body
					continue
				}
			} else if body := g.flowFrameBody(r, s); body != "" {
				bodies[r] = body
				continue
			}
			g.FailFlow("the body of rule " + r.Name)
		}
	}
	if g.collect != nil && g.Table != "trules" {
		if g.collect.scopes == nil {
			g.collect.scopes = map[ruleKey][]string{}
		}
		for _, r := range all {
			g.collect.scopes[ruleKey{g.Table, r.ID, false}] = append([]string(nil), scopes[r].Names...)
		}
	}
	if g.Table == "rules" {
		g.NodeScopes = make(map[*compile.Rule][]string, len(all))
		for _, r := range all {
			g.NodeScopes[r] = append([]string(nil), scopes[r].Names...)
		}
	}
	if g.Table == "trules" {
		// The typed rules share the analysis results of the rules.
		init.WriteString("\ttrules = NewTRules(rules)\n\tdefer tinit()\n")
		g.Methods.WriteString("// trules is the typed rule table.\nvar trules []*TRule\n\n")
	} else {
		// The table is static data of a compact form (RuleSpec); the init function builds the rules.
		var tab strings.Builder
		strs := []string{"nil"}
		strIdx := map[string]int{"nil": 0}
		strsOf := func(xs []string) int {
			if len(xs) == 0 {
				return 0
			}
			lit := goStrings(xs)
			if k, ok := strIdx[lit]; ok {
				return k
			}
			strIdx[lit] = len(strs)
			strs = append(strs, strings.TrimPrefix(lit, "[]string"))
			return strIdx[lit]
		}
		var extras []string
		for i, r := range all {
			if r.ID != i {
				panic("rule ids are not positions in the rule table")
			}
			value := "nil"
			if _, ok := r.Def.Expr.(*grammar.OperatorsRule); !ok { // the lines build the value of an Operators Rule
				value = "&" + g.shaper(r.BodyExpr())
			}
			var flags []string
			if r.Memo && !r.Transient {
				flags = append(flags, "RuleMemo")
			}
			if r.Leader {
				flags = append(flags, "RuleLeader")
			}
			if r.Novalue {
				flags = append(flags, "RuleNovalue")
			}
			if g.Document && (r.Leader || r.Memo) {
				flags = append(flags, "RuleDocMemo")
			}
			if g.Document && r.Positional {
				flags = append(flags, "RulePositional")
			}
			if r.Splice {
				flags = append(flags, "RuleSplice")
			}
			if r.Rec {
				flags = append(flags, "RuleRec")
			}
			flagsLit := "0"
			if len(flags) > 0 {
				flagsLit = strings.Join(flags, " | ")
			}
			seen := 0
			if r.Seen >= 0 {
				seen = r.Seen + 1
			}
			fmt.Fprintf(&tab, "\t{%q, %s, %s, %d, %d, %d},\n", r.Name, value, flagsLit, seen, strsOf(scopes[r].Names), strsOf(r.Vars))
			w := r.WhereName()
			if w == r.Name {
				w = ""
			}
			if w != "" || r.NodeType != "" || r.OpNodeType != "" || r.Cycle != 0 {
				extras = append(extras, fmt.Sprintf("\t{%d, %q, %q, %q, %d},\n", r.ID, w, r.NodeType, r.OpNodeType, r.Cycle))
			}
		}
		if err := ruleTableLimits(len(strs), g.Prog.NSeen); err != nil {
			g.tableErr = err
		}
		spec, strTable, extraTable := g.Table+"Spec", g.Table+"Strs", g.Table+"Extras"
		// The entries are positional, which vet accepts only for a type of the package; NewRules takes
		// any type of the layout of RuleSpec and RuleExtra.
		if !g.ruleTypesDone {
			g.ruleTypesDone = true
			g.Vars.WriteString("// ruleSpec and ruleExtra have the layout of RuleSpec and RuleExtra, with the fields in this order:\n// name, value, flags, seen, scope, vars; and id, where, node type, operators rule node type, cycle.\ntype ruleSpec struct {\n\tName  string\n\tValue *Shaper\n\tFlags uint8\n\tSeen  int32\n\tScope uint16\n\tVars  uint16\n}\n\ntype ruleExtra struct {\n\tId         int\n\tWhere      string\n\tNodeType   string\n\tOpNodeType string\n\tCycle      int32\n}\n\n")
		}
		fmt.Fprintf(&g.Vars, "// %s is the compact form of the rule table, built by NewRules.\nvar %s = [...]ruleSpec{\n%s}\n\n", spec, spec, tab.String())
		fmt.Fprintf(&g.Vars, "var %s = [...][]string{\n", strTable)
		for _, x := range strs {
			if x == "nil" {
				g.Vars.WriteString("\tnil,\n")
			} else {
				fmt.Fprintf(&g.Vars, "\t%s,\n", x)
			}
		}
		g.Vars.WriteString("}\n\n")
		fmt.Fprintf(&g.Vars, "var %s = [...]ruleExtra{\n%s}\n\n", extraTable, strings.Join(extras, ""))
		fmt.Fprintf(init, "\t%s = NewRules(%s[:], %s[:], %s[:])\n", g.Table, spec, strTable, extraTable)
		fmt.Fprintf(init, "\t%s.SetRules(%s, %d)\n", g.grammarVar(), g.Table, g.Prog.NSeen)
	}
	nodeCode := g.Table == "rules" && g.typedOnly
	if nodeCode {
		// The Node bodies were generated for the frame layouts they fix; their methods are not
		// kept, and the typed rules need none of the Node entries.
		g.Methods.Reset()
		g.Methods.WriteString(methodsBefore)
	}
	for _, r := range all {
		if nodeCode {
			break
		}
		if g.pruned(r) {
			continue
		}
		if pr, ok := r.Def.Expr.(*grammar.OperatorsRule); ok {
			g.Cur = r
			g.operatorsRule(r, pr)
			if r.Action != nil && !r.Novalue {
				// The whole-rule action, which the runtime runs on the folded value after each call.
				if g.Table == "trules" {
				}
				locals := map[string]string{compile.OperatorsRuleValue: "c.Value"}
				fmt.Fprintf(init, "\t%s[%d].Action = func(c *%s) (any, any) { %s }\n", g.Table, r.ID, g.ctxType(), g.actionBody(g.CoreAction(r), compile.NewScope(), locals))
			}
			continue
		}
		if d := flats[r]; d != nil && g.Table == "trules" {
			g.typedCall(r, "", scopes[r], d)
			continue
		}
		if g.slotReached(r) {
			fmt.Fprintf(init, "\t%s[%d].Body = %s\n", g.Table, r.ID, bodies[r])
			g.printedBody(r)
		}
		if r.Action != nil && g.Table == "trules" {
			// A function of its own, which typedCall calls directly.
			g.Proj = g.Projections(r)
			fmt.Fprintf(&g.Methods, "// The action of %s.\nfunc ta%d(c *TActCtx) (any, any) { %s }\n\n", r.Name, r.ID, g.actionBody(g.CoreAction(r), scopes[r], nil))
			g.Proj = nil
			fmt.Fprintf(init, "\ttrules[%d].Action = ta%d\n", r.ID, r.ID)
		} else if r.Action != nil {
			g.Proj = g.Projections(r)
			fmt.Fprintf(init, "\t%s[%d].Action = func(c *%s) (any, any) { %s }\n", g.Table, r.ID, g.ctxType(), g.actionBody(g.CoreAction(r), scopes[r], nil))
			g.Proj = nil
		}
		if g.Table == "trules" {
			g.typedCall(r, bodies[r], scopes[r], nil)
		} else if r.Plain {
			start := g.Methods.Len()
			if d := flats[r]; d != nil {
				g.flatMethod(r, d, fmt.Sprintf("%s%d", g.plainPrefix(), r.ID), "called as by invokePlain", true)
			} else {
				g.plainCall(r, bodies[r])
			}
			g.wrappers = append(g.wrappers, wrapperSpan{r: r, start: start, end: g.Methods.Len()})
		}
	}
	if r := g.streamStart; r != nil && g.Table == "rules" {
		// ParseStream calls a copy of the start rule whose body emits the
		// #stream elements, generated last so that the batch code is the same
		// as without streams. Its captures have a layout of their own.
		s := compile.NewScope()
		body := g.flowStreamBody(r, s)
		if body != "" {
			if g.hooks.StreamRoute != nil && g.collect == nil {
				g.hooks.StreamRoute(g.Table, r)
			}
		} else {
			g.FailFlow("the stream body of rule " + r.Name)
		}
		if strings.Join(s.Names, "\x00") != strings.Join(scopes[r].Names, "\x00") {
			// The copied action addresses captures by the batch body's slots.
			g.streamErr = fmt.Errorf("internal error: the stream body of %s has a different capture layout", r.Name)
		}
		fmt.Fprintf(init, "\tstreamRule = new(Rule)\n\t*streamRule = *rules[%d]\n\tstreamRule.Scope = %s\n\tstreamRule.Body = %s\n", r.ID, goStrings(s.Names), body)
		g.Methods.WriteString("// streamRule is the start rule of ParseStream.\nvar streamRule *Rule\n\n")
	}
	g.dropUnusedWrappers()
	if g.Table != "rules" {
		return // the recognizer shares the struct fields
	}
	// Struct fields, used to detect references to undeclared fields at run time and to read the
	// fields that a constructor left unset.
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
			fields = append(fields, fmt.Sprintf("%q, %q", f, sz.Zeros[f]))
		}
		fmt.Fprintf(init, "\tzpGrammar.DeclareStruct(%q%s)\n", name, joinPrefixed(fields))
		fmt.Fprintf(init, "\tzpGrammar.TypeKinds[%q] = KindOf(%q, \"\")\n", name, name)
	}
	// The kinds of the named node types, under their names and those of their aliases.
	names = names[:0]
	for name := range g.Prog.NodeKinds {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(init, "\tzpGrammar.NodeTypes[%q] = NewNodeTypeKinds(%q)\n", name, g.Prog.NodeKinds[name][0].Type)
	}
}

// nodeTypeNames returns the names of the grammar's named node types, sorted, in the order of their
// generated indexes from 1 (nodeTypeIndex).
func (g *Generator) nodeTypeNames() []string {
	var names []string
	for name, spec := range g.Prog.Types {
		if td, ok := spec.(*grammar.TypeDef); ok && td.Struct() == nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// nodeTypeIndex returns the generated index of the named node type name, from 1.
func (g *Generator) nodeTypeIndex(name string) int {
	n, ok := g.Prog.NodeTypeOf(name)
	if !ok {
		panic("internal error: " + name + " is not a named node type")
	}
	return sort.SearchStrings(g.nodeTypeNames(), n.Name) + 1
}

// nodeTypeID returns the Go expression of the registered index of the named node type name in the
// typed runtime (nt in tmatch and tnode).
func (g *Generator) nodeTypeID(name string) string {
	return fmt.Sprintf("%s[%d]", nodeTypesName, g.nodeTypeIndex(name))
}

// typedResultConversion returns the statement that converts the value v of rule r to the rule's
// named node type in the typed runtime (what finish does), or "" if r has none. body, if not "",
// holds the fresh node of the rule's body (tfreshNode), which converts in place.
func (g *Generator) typedResultConversion(r *compile.Rule, v, body string) string {
	if r.NodeType == "" {
		return ""
	}
	if body != "" {
		return fmt.Sprintf("%s = p.TConvertResultOwned(trules[%d], %s, %s)", v, r.ID, v, body)
	}
	return fmt.Sprintf("%s = p.TConvertResult(trules[%d], %s)", v, r.ID, v)
}

// ownsBody reports whether finishing the value of rule r in the typed runtime converts the fresh
// node of its body in place: the rule has a named node type and returns its body's value.
func (g *Generator) ownsBody(r *compile.Rule) bool {
	return r.NodeType != "" && !r.Novalue && r.Action == nil
}

// typedCall writes the methods that call the rule r in the typed runtime: s<id> for a rule never
// memoized and without captures (what invokePlain does), and otherwise u<id> (what call does, the
// memo kept by callMemo) and i<id> (what invoke does). Each calls the body's method and the action
// directly and finishes the value as the rule's kind requires (what finish does), so a call goes
// through no rule table entry or function value.
//
// For a flat rule (d, see flowrule.go), the method that runs the body (s<id>, v<id> or i<id>)
// has it inlined, with the action.
func (g *Generator) typedCall(r *compile.Rule, body string, s *compile.Scope, d *flatRule) {
	if _, operatorsRule := r.Def.Expr.(*grammar.OperatorsRule); operatorsRule || r.Leader {
		return // the general call
	}
	m := &g.Methods
	entry := fmt.Sprintf("u%d", r.ID)
	if r.Plain {
		entry = fmt.Sprintf("s%d", r.ID)
	}
	if g.typedEntry[r.ID] {
		// Only a typed entry point runs an Entry; every other call names the function.
		fmt.Fprintf(&g.Init, "\ttrules[%d].Entry = %s\n", r.ID, entry)
	}
	ncaps := len(s.Names)
	if d != nil {
		ncaps = len(r.Scope.Names)
	}
	finish := func(frame string) {
		body := ""
		if g.ownsBody(r) {
			body = "body"
			m.WriteString("\t\tbody := TFreshNode(v)\n")
		}
		switch {
		case r.Novalue:
			m.WriteString("\t\tv = nil\n")
		case r.Action != nil:
			fmt.Fprintf(m, "\t\tc := p.UseCtx(TActCtx{P: p, F: %s, Start: start, End: p.Pos, Cbase: len(p.Created)})\n", frame)
			fmt.Fprintf(m, "\t\tv = c.Result(ta%d, %q)\n", r.ID, r.WhereName())
		default:
			fmt.Fprintf(m, "\t\tv = p.ScopeValue(trules[%d].Value, v, start)\n", r.ID)
			if len(s.Names) > 0 {
				fmt.Fprintf(m, "\t\tv = p.AttachCaptures(v, trules[%d].Scope, %s.Vals, start, p.Pos)\n", r.ID, frame)
			}
			m.WriteString("\t\tif n := AsTval(v); n != nil {\n\t\t\tn.TSetFresh(false)\n\t\t}\n")
		}
		if conv := g.typedResultConversion(r, "v", body); conv != "" {
			fmt.Fprintf(m, "\t\t%s\n", conv)
		}
	}
	plain := func(name string) {
		if d != nil {
			g.flatMethod(r, d, name, "called as by invokePlain", true)
			return
		}
		if g.Observability && r.Plain && !g.directTypedDocument {
			name = g.tracePlainWrapper(r, name)
		}
		fmt.Fprintf(m, "// %s, called as by invokePlain\nfunc %s(p *TParser) (any, bool) {\n", r.Name, name)
		m.WriteString("\t" + g.saveRec(", trail") + " := " + g.saveRecValues(", len(p.Trail)") + "\n\tprevEnv, prevCut := p.Env, p.Cut\n\tp.Cut = false\n" + enterRec(r))
		if g.Observability && (!r.Plain || g.directTypedDocument) {
			m.WriteString("p.Observation.Evaluated()\n")
		}
		fmt.Fprintf(m, "\tv, ok := %s(p, 0)\n%s\tp.Cut = prevCut\n\tp.DropTrail(trail)\n\tif ok {\n", body, leaveRec(r))
		finish("EmptyTFrame")
		m.WriteString("\t} else {\n\t\t" + g.restoreRec("start", "rec") + "\n\t}\n\tp.Env = prevEnv\n\treturn v, ok\n}\n\n")
	}
	if r.Plain {
		if g.directTypedDocument {
			name := fmt.Sprintf("s%d", r.ID)
			if g.Observability {
				name = g.traceCallWrapper(r, name, false)
			}
			fmt.Fprintf(m, "func %s(p *TParser) (any, bool) {\n if p.TypedDocument != nil { return p.DocumentCall(trules[%d], 0, ds%d) }; return ds%d(p)\n}\n", name, r.ID, r.ID, r.ID)
			plain(fmt.Sprintf("ds%d", r.ID))
			return
		}
		plain(fmt.Sprintf("s%d", r.ID))
		return
	}
	// u<id>: the call. An unmemoized call of a rule without captures is plain.
	name := fmt.Sprintf("u%d", r.ID)
	if g.Observability {
		name = g.traceCallWrapper(r, name, false)
	}
	fmt.Fprintf(m, "// rule %s\nfunc %s(p *TParser) (any, bool) {\n", r.Name, name)
	// Without Documents the invocation of a flat rule without captures is its plain method: the
	// two bodies differ only in the rollback to the start on failure, which the callers of the
	// invocation repeat harmlessly, so one body serves both. A Document reads the position the
	// invocation leaves at its failure to find the frontier it examined, so Documents keep both.
	merged := d != nil && ncaps == 0 && !g.Document
	iname := fmt.Sprintf("i%d", r.ID)
	if merged {
		iname = fmt.Sprintf("v%d", r.ID)
	}
	if g.directTypedDocument {
		fmt.Fprintf(m, " if p.TypedDocument != nil { return p.DocumentCall(trules[%d], 0, %s) }\n", r.ID, iname)
	}
	memo := r.Memo && !r.Transient
	if memo {
		fmt.Fprintf(m, "\tif !p.FirstCall(rules[%d]) {\n\t\treturn p.CallMemo(trules[%d], %s)\n\t}\n", r.ID, r.ID, iname)
	}
	if ncaps == 0 {
		fmt.Fprintf(m, "\treturn v%d(p)\n}\n\n", r.ID)
		plain(fmt.Sprintf("v%d", r.ID))
	} else {
		fmt.Fprintf(m, "\t%s := %s\n\tv, ok := i%d(p)\n\tif !ok {\n\t\t%s\n\t}\n\treturn v, ok\n}\n\n", g.saveRec(""), g.saveRecValues(""), r.ID, g.restoreRec("start", "rec"))
	}
	// i<id>: the invocation (also used by the memo).
	if merged {
		return // v<id>
	}
	if d != nil {
		g.flatMethod(r, d, fmt.Sprintf("i%d", r.ID), "invoked as by invoke", false)
		return
	}
	fmt.Fprintf(m, "// %s, invoked as by invoke\nfunc i%d(p *TParser) (any, bool) {\n", r.Name, r.ID)
	fmt.Fprintf(m, "\tf := p.NewFrame(%d)\n\tprevFrame, prevEnv, prevCut, trail := p.Frame, p.Env, p.Cut, len(p.Trail)\n\tp.Frame, p.Cut = f, false\n%s\tstart := p.Pos\n\t_ = start\n", len(s.Names), enterRec(r))
	if g.Observability {
		m.WriteString("p.Observation.Evaluated()\n")
	}
	fmt.Fprintf(m, "\tv, ok := %s(p, 0)\n%s\tp.Frame, p.Cut = prevFrame, prevCut\n\tp.DropTrail(trail)\n\tif ok {\n", body, leaveRec(r))
	finish("f")
	m.WriteString("\t}\n\tp.Env = prevEnv\n\tp.FreeFrame(f)\n\treturn v, ok\n}\n\n")
}

// wrapperSpan is the span [start, end) of the methods of a plain rule's wrapper in generator.methods.
type wrapperSpan struct {
	r          *compile.Rule
	start, end int
}

// dropUnusedWrappers removes the wrappers of the plain rules that no generated call refers to. It
// runs after the bodies and the stream body, which are the last generated calls, so that a wrapper
// is kept exactly when a call of it was generated. Spans are removed from the last one to the first,
// so the earlier spans keep their offsets.
func (g *Generator) dropUnusedWrappers() {
	text := g.Methods.String()
	for i := len(g.wrappers) - 1; i >= 0; i-- {
		w := g.wrappers[i]
		if !strings.HasPrefix(text[w.start:w.end], "// ") || !strings.HasSuffix(text[w.start:w.end], "}\n\n") {
			panic(fmt.Sprintf("internal error: the wrapper of %s is not at its recorded span", w.r.Name))
		}
		if !g.plainCalled[w.r] {
			text = text[:w.start] + text[w.end:]
		}
	}
	g.wrappers = nil
	g.Methods.Reset()
	g.Methods.WriteString(text)
}

// plainPrefix is the prefix of the methods written by plainCall, and seenVar the variable that
// holds the number of rules with rule.seen set, for the rule table being generated.
func (g *Generator) plainPrefix() string {
	switch g.Table {
	case "rules":
		return "r"
	case "trules":
		return "s"
	}
	return "q"
}

// pick returns untyped for the Node runtime and typed for the typed runtime (trules).
func (g *Generator) pick(untyped, typed string) string {
	if g.Table == "trules" {
		return typed
	}
	return untyped
}

func (g *Generator) recv() string    { return g.pick("Parser", "TParser") }
func (g *Generator) valType() string { return g.pick("*Node", "any") }
func (g *Generator) ctxType() string { return g.pick("ActCtx", "TActCtx") }

// step returns the statement that charges one step to the budget control of recv, if there is one.
func (g *Generator) step(recv string) string {
	if g.NoBudget {
		return ""
	}
	return recv + ".Spend()\n"
}

func (g *Generator) frameVar() string { return g.pick("EmptyFrame", "EmptyTFrame") }
func (g *Generator) rt() string       { return g.pick("rt", "trt") }

// grammarVar returns the generated grammar of the rule table being generated.
func (g *Generator) grammarVar() string {
	if g.Table == "rules" {
		return "zpGrammar"
	}
	return "zpRecGrammar"
}

// Method emits a method with the given body and returns its name.
func (g *Generator) Method(comment, body string) string {
	name, shared := g.MethodName(g.recv()+"/"+g.valType(), body)
	if shared {
		return name
	}
	if comment != "" {
		fmt.Fprintf(&g.Methods, "// %s\n", comment)
	}
	fmt.Fprintf(&g.Methods, "func %s(p *%s) (%s, bool) {\n%s%s}\n\n", name, g.recv(), g.valType(), g.step("p"), body)
	return name
}

// bodyMethod emits the function of a rule body and returns its name. It has the signature of the
// Body field of the rule table, so the table names it without a closure; the Operators Rule level
// it ignores is 0 where the generated code calls it.
func (g *Generator) bodyMethod(comment, body string) string {
	name, shared := g.MethodName(g.recv()+"/"+g.valType()+"/body", body)
	if shared {
		return name
	}
	if comment != "" {
		fmt.Fprintf(&g.Methods, "// %s\n", comment)
	}
	fmt.Fprintf(&g.Methods, "func %s(p *%s, _ int) (%s, bool) {\n%s%s}\n\n", name, g.recv(), g.valType(), g.step("p"), body)
	return name
}

// shaper returns the name of the package variable that holds the runtime shaper of the expression
// e, its static shape (internal/compile/value_shape.go). Few distinct shapes occur, so the variables are shared.
func (g *Generator) shaper(e grammar.Expr) string {
	return g.shaperOf(shaperLit(g.Prog, e))
}

// shaperOf returns the name of the package variable that holds the shaper of the literal lit,
// declaring it on first use.
func (g *Generator) shaperOf(lit string) string {
	if g.shaperNames == nil {
		g.shaperNames = map[string]string{}
	}
	if name := g.shaperNames[lit]; name != "" {
		return name
	}
	name := g.Name("sh")
	g.shaperNames[lit] = name
	g.shaperKeys = append(g.shaperKeys, lit)
	fmt.Fprintf(&g.Vars, "var %s = %s\n", name, lit)
	return name
}

// shaperLit returns the literal of the runtime shaper of the expression e.
func shaperLit(prog *compile.Program, e grammar.Expr) string {
	return fmt.Sprintf("Shaper{Elems: %v, K: %d, Mode: %d, Tuple: %v, Shared: %v}", compile.IsElems(prog, e), compile.ValuedCount(prog, e), compile.SingleModeOf(prog, e), compile.ScopeTuple(prog, e), compile.IsElems(prog, e) && compile.SharesTuple(prog, e))
}

// plainCall writes the method r<id>, which does what invokePlain does for the rule r (a rule never
// memoized and without captures) with a direct call of its body.
func (g *Generator) plainCall(r *compile.Rule, body string) {
	name := fmt.Sprintf("%s%d", g.plainPrefix(), r.ID)
	if g.Observability {
		name = g.tracePlainWrapper(r, name)
	}
	fmt.Fprintf(&g.Methods, "// %s, called as by invokePlain\nfunc %s(p *%s) (%s, bool) {\n%s", r.Name, name, g.recv(), g.valType(), g.step("p"))
	g.Methods.WriteString("\t" + g.saveRec(", trail") + " := " + g.saveRecValues(", len(p.Trail)") + "\n")
	g.Methods.WriteString(`	prevEnv, prevCut := p.Env, p.Cut
	p.Cut = false
` + enterRec(r))
	fmt.Fprintf(&g.Methods, "\tv, ok := %s(p, 0)\n", body)
	g.Methods.WriteString(leaveRec(r) + "\tp.Cut = prevCut\n" + g.pick("\tp.Trail = p.Trail[:trail]\n", "\tp.DropTrail(trail)\n") + "\tif ok {\n")
	if r.Novalue {
		g.Methods.WriteString("\t\tv = nil\n")
	} else {
		fmt.Fprintf(&g.Methods, "\t\tv = p.Finish(%s[%d], %s, v, start)\n", g.Table, r.ID, g.frameVar())
	}
	g.Methods.WriteString("\t} else {\n\t\t" + g.restoreRec("start", "rec") + "\n\t}\n")
	g.Methods.WriteString(`	p.Env = prevEnv
	return v, ok
}

`)
}

// first returns the condition on the next character ch under which the expression e can match,
// when e must begin with a given terminal (a literal or a character class, possibly behind
// sequences, captures, @, - and calls of rules that are not left-recursive or OperatorsRule), with the
// expectation e records when the condition does not hold and the number of rule calls on the way.
// Then e fails at once without the terminal, recording only that expectation (a call memoizes the
// same failure, and memoization only affects speed), so a choice can skip it; unless the calls on
// the way would exceed the nesting limit.
func (g *Generator) first(e grammar.Expr, depth int, seen map[string]bool) (string, compile.ExpID, int, bool) {
	switch e := e.(type) {
	case *grammar.Literal:
		if e.Value == "" {
			break
		}
		r, _ := utf8.DecodeRuneInString(e.Value)
		if e.IgnoreCase {
			// The next character matches the first code point in either case of an ASCII letter.
			f := compile.FoldASCIIRune(r)
			cond := fmt.Sprintf("ch == %d", f)
			if 'a' <= f && f <= 'z' {
				cond += fmt.Sprintf(" || ch == %d", f-('a'-'A'))
			}
			return cond, g.Desc(compile.LiteralDesc(e)), depth, true
		}
		return fmt.Sprintf("ch == %d", r), g.Desc(strconv.Quote(e.Value)), depth, true
	case *grammar.CharClass:
		return NegateCondition(g.ControlledClassReject(e)), g.Desc(compile.CharClassString(e)), depth, true
	case *grammar.Seq:
		if len(e.Items) > 0 {
			return g.first(e.Items[0], depth, seen)
		}
	case *grammar.Capture:
		return g.first(e.Expr, depth, seen)
	case *grammar.Atomic:
		return g.first(e.Expr, depth, seen)
	case *grammar.Discard:
		return g.first(e.Expr, depth, seen)
	case *grammar.Ref:
		r := g.Prog.ByName[e.Name]
		if r == nil || r.Leader || seen[e.Name] {
			break
		}
		if _, operatorsRule := r.Def.Expr.(*grammar.OperatorsRule); operatorsRule {
			break
		}
		seen[e.Name] = true
		return g.first(r.Def.Expr, depth+r.EntryDepth(), seen)
	}
	return "", 0, 0, false
}

// enterRec returns the statement that counts the entry of the rule r against the nesting limit: only
// a rule on a cycle of calls counts.
func enterRec(r *compile.Rule) string {
	if r.Rec {
		return "\tp.Enter()\n"
	}
	return ""
}

// leaveRec returns the statement that ends what enterRec counted.
func leaveRec(r *compile.Rule) string {
	if r.Rec {
		return "\tp.Depth--\n"
	}
	return ""
}

// depthLimit returns the nesting guard needed before calling depth rules.
func depthLimit(depth int) string {
	if depth == 0 {
		return "p.Depth <= p.MaxDepth"
	}
	return fmt.Sprintf("p.Depth+%d <= p.MaxDepth", depth)
}

// NegateCondition inverts a generated condition without retaining a leading
// logical negation as a double negation.
func NegateCondition(condition string) string {
	if strings.HasPrefix(condition, "!") {
		return strings.TrimPrefix(condition, "!")
	}
	return "!(" + condition + ")"
}

// classReject returns a Go condition on ch that is true when ch is not accepted by the class.
func classReject(e *grammar.CharClass) string {
	var in []string
	for _, r := range e.Ranges {
		if r.Lo == r.Hi {
			in = append(in, fmt.Sprintf("ch == %d", r.Lo))
		} else {
			in = append(in, fmt.Sprintf("ch >= %d && ch <= %d", r.Lo, r.Hi))
		}
	}
	if len(in) == 0 {
		in = []string{"ch < 0"} // never true; mentions ch so that it is used
	}
	cond := strings.Join(in, " || ")
	if e.Negated {
		return "(" + cond + ")"
	}
	return "!(" + cond + ")"
}

func (g *Generator) operatorsRule(r *compile.Rule, e *grammar.OperatorsRule) {
	var b strings.Builder
	fmt.Fprintf(&b, "\t%s[%d].OperatorsRule = &%s{\n", g.Table, r.ID, g.pick("OperatorsRule", "TOperatorsRule"))
	// The lines of the rule's Core IR, in the order of e, if they are printed from Flow IR.
	co := g.flowOperatorsRule(r)
	if e.Skip != nil {
		s := compile.NewScope()
		var skip *core.OpLine
		if co != nil {
			skip = co.Skip
		}
		m := g.flowOperatorsRuleBody(skip, e.Skip, s, false)
		if m == "" {
			g.FailFlow("the skip of an Operators Rule of rule " + g.Cur.Name)
		}
		fmt.Fprintf(&b, "\t\tSkip: %s,\n", m)
	}
	fmt.Fprintf(&b, "\t\tOperands: []*%s{\n", g.pick("OperatorsRuleLine", "TOperatorsRuleLine"))
	for i, o := range e.Operands {
		var l *core.OpLine
		if co != nil {
			l = co.Operands[i]
		}
		fmt.Fprintf(&b, "\t\t\t%s,\n", g.operatorsRuleLine(o.Expr, o.Action, false, l))
	}
	b.WriteString("\t\t},\n")
	var prefix, led []string
	id := 0
	for i, l := range e.Levels {
		for _, op := range l.Operators {
			var l *core.OpLine
			if co != nil {
				l = co.Ops[id].Line
			}
			s := fmt.Sprintf("{Id: %d, Kind: %q, Assoc: %q, Level: %d, Line: %s}", id, op.Kind, op.Assoc, i+1, g.operatorsRuleLine(op.Expr, op.Action, true, l))
			id++
			if op.Kind == grammar.Prefix {
				prefix = append(prefix, s)
			} else {
				led = append(led, s)
			}
		}
	}
	op := g.pick("OperatorsRuleOp", "TOperatorsRuleOp")
	fmt.Fprintf(&b, "\t\tPrefix: []*%s{%s},\n", op, strings.Join(prefix, ",\n"))
	fmt.Fprintf(&b, "\t\tLed: []*%s{%s},\n", op, strings.Join(led, ",\n"))
	b.WriteString("\t}\n")
	fmt.Fprintf(&b, "\t%s[%d].Body = func(p *%s, min int) (%s, bool) { return p.OperatorsRuleParse(%s[%d], min) }\n", g.Table, r.ID, g.recv(), g.valType(), g.Table, r.ID)
	g.printedBody(r)
	g.Init.WriteString(b.String())
}

// operatorsRuleLine returns the line of an Operators Rule with the expression e and the action;
// l is its Core IR if it is printed from Flow IR, or nil.
func (g *Generator) operatorsRuleLine(e grammar.Expr, action grammar.Term, operator bool, l *core.OpLine) string {
	s := compile.NewScope()
	var locals []string
	if operator {
		locals = []string{"lhs", "rhs", "op"}
	}
	// Lines whose value is unused are matched without values, as in the engine; so are all lines
	// of a rule without a value (in the recognizer).
	build := !g.Cur.Novalue && !compile.LeanLine(action, locals)
	m := g.flowOperatorsRuleBody(l, e, s, build)
	if m == "" {
		g.FailFlow("a line of an Operators Rule of rule " + g.Cur.Name)
	}
	act := "nil"
	if action != nil && !g.Cur.Novalue {
		var locals map[string]string
		if operator {
			locals = map[string]string{"lhs": "c.Lhs", "rhs": "c.Rhs", "op": "c.Op"}
		}
		var ct core.Term
		if l != nil {
			ct = l.Action
		}
		act = fmt.Sprintf("func(c *%s) (any, any) { %s }", g.ctxType(), g.actionBody(ct, s, locals))
	}
	return fmt.Sprintf("&%s{Scope: %s, M: %s, Action: %s, Value: %s}", g.pick("OperatorsRuleLine", "TOperatorsRuleLine"), goStrings(s.Names), m, act, g.shaper(e))
}

// ruleCall returns the call of the rule r from Operators Rule level min.
func (g *Generator) ruleCall(r *compile.Rule, min int) string {
	_, operatorsRule := r.Def.Expr.(*grammar.OperatorsRule)
	wrapper := g.callWrapper(g.Table, r)
	g.checkCall(r, wrapper)
	switch {
	case g.Document && g.Table == "rules":
		// Document parses route every call through call, which memoizes per rule.docMemo,
		// including otherwise plain rules.
		return fmt.Sprintf("p.Call(%s[%d], %d)", g.Table, r.ID, min)
	case g.directTypedDocument && g.Table == "trules" && operatorsRule:
		return fmt.Sprintf("p.Call(%s[%d], %d)", g.Table, r.ID, min)
	case r.Plain && !operatorsRule:
		// Never memoized and without captures: call does nothing but invokePlain, which a
		// method of its own does with a direct call of the body (see plainCalls).
		if g.plainCalled == nil {
			g.plainCalled = map[*compile.Rule]bool{}
		}
		g.plainCalled[r] = true
		return fmt.Sprintf("%s%d(p)", g.plainPrefix(), r.ID)
	case g.Table == "trules" && !operatorsRule && !r.Leader:
		// The typed runtime calls a method of the rule's own (typedCall).
		return fmt.Sprintf("u%d(p)", r.ID)
	case r.Plain:
		return fmt.Sprintf("p.InvokePlain(%s[%d], %d)", g.Table, r.ID, min)
	}
	return fmt.Sprintf("p.Call(%s[%d], %d)", g.Table, r.ID, min)
}

// ControlledClassReject preserves the compact inactive condition while bounded
// membership uses immutable generated range tables. TS keeps its own runtime.
func (g *Generator) ControlledClassReject(e *grammar.CharClass) string {
	ordinary := classReject(e)
	if g.NoBudget || len(e.Ranges) <= 4 {
		// Without budget controls the class is tested without a control, so it has no range table.
		return ordinary
	}
	key := fmt.Sprintf("%v%#v", e.Negated, e.Ranges)
	if g.budgetClassNames == nil {
		g.budgetClassNames = map[string]string{}
	}
	name := g.budgetClassNames[key]
	if name == "" {
		name = g.Name("budgetClass")
		g.budgetClassNames[key] = name
		g.budgetClassKeys = append(g.budgetClassKeys, key)
		fmt.Fprintf(&g.Vars, "var %s = NewBudgetClass([]BudgetRange{", name)
		for _, r := range e.Ranges {
			fmt.Fprintf(&g.Vars, "{Lo: %d, Hi: %d},", r.Lo, r.Hi)
		}
		fmt.Fprintf(&g.Vars, "}, %v)\n", e.Negated)
	}
	return fmt.Sprintf("((p.Control != nil && p.RejectBudgetClass(ch, %s)) || (p.Control == nil && (%s)))", name, ordinary)
}

// atEnd and atLineStart name the runtime methods that test the anchors; a parser without
// ParseStream uses the forms that need not consider a stream, which inline.
func (g *Generator) atEnd() string {
	if g.streamStart == nil {
		return g.helper("AtEndBatch")
	}
	return g.helper("AtEnd")
}

func (g *Generator) atLineStart() string {
	if g.streamStart == nil {
		return g.helper("AtLineStartBatch")
	}
	return g.helper("AtLineStart")
}

// helper names the runtime method that a generated execution function calls on its parser. A
// parser generated without budget controls and without Documents has no controlled form of its
// bodies, so they call the Ordinary form of the method.
func (g *Generator) helper(name string) string {
	if g.Ordinary && ordinaryHelperName(name) {
		return name + "Ordinary"
	}
	return name
}

// leanTyped reports whether typed rule bodies are emitted in their ordinary form only.
func (g *Generator) leanTyped() bool {
	return g.Ordinary && g.Table == "trules"
}

// classTable returns the name of the package variable that holds the membership table of e,
// declaring it on first use.
func (g *Generator) classTable(e *grammar.CharClass) string {
	key := fmt.Sprintf("cls%v%#v", e.Negated, e.Ranges)
	if g.budgetClassNames == nil {
		g.budgetClassNames = map[string]string{}
	}
	if name := g.budgetClassNames[key]; name != "" {
		return name
	}
	name := g.Name("cls")
	g.budgetClassNames[key] = name
	g.budgetClassKeys = append(g.budgetClassKeys, key)
	ascii, wideRunes := classBits(e)
	var wide []string
	for _, r := range wideRunes {
		wide = append(wide, strconv.Itoa(int(r)))
	}
	fmt.Fprintf(&g.Vars, "var %s = Class{ASCII: [2]uint64{%#x, %#x}", name, ascii[0], ascii[1])
	if len(wide) > 0 {
		fmt.Fprintf(&g.Vars, ", Wide: []rune{%s}", strings.Join(wide, ", "))
	}
	if e.Negated {
		g.Vars.WriteString(", Neg: true")
	}
	g.Vars.WriteString("}\n")
	return name
}

// classSteps returns the name of the package variable that lists the class steps of items, which
// are character classes, declaring it on first use.
func (g *Generator) classSteps(items []grammar.Expr) string {
	var steps []string
	for _, it := range items {
		c := it.(*grammar.CharClass)
		steps = append(steps, fmt.Sprintf("{C: &%s, ID: %d}", g.classTable(c), g.Desc(compile.CharClassString(c))))
	}
	body := strings.Join(steps, ", ")
	key := "steps" + body
	if name := g.budgetClassNames[key]; name != "" {
		return name
	}
	name := g.Name("steps")
	g.budgetClassNames[key] = name
	g.budgetClassKeys = append(g.budgetClassKeys, key)
	fmt.Fprintf(&g.Vars, "var %s = []ClassStep{%s}\n", name, body)
	return name
}

// classBits returns the membership tables of a character class as the runtime's Class holds
// them: the bitset of the matching characters below 128, with the negation applied, and the
// inclusive pairs of the ranges at or above 128 (a range that starts below is cut at 128).
func classBits(e *grammar.CharClass) (ascii [2]uint64, wide []rune) {
	for c := rune(0); c < 128; c++ {
		in := false
		for _, r := range e.Ranges {
			in = in || (c >= r.Lo && c <= r.Hi)
		}
		if in != e.Negated {
			ascii[c>>6] |= 1 << (uint(c) & 63)
		}
	}
	for _, r := range e.Ranges {
		if r.Hi >= 128 {
			wide = append(wide, max(r.Lo, 128), r.Hi)
		}
	}
	return ascii, wide
}

// writeDescs gives the generated parser the table of expectation descriptions, which a parse
// records only when it passes WithExpectations, when the options ask for it.
func (g *Generator) writeDescs(opts gen.GenOptions) {
	if !opts.Expectations {
		return
	}
	fmt.Fprintf(&g.Init, "\tzpGrammar.Descs = %s\n", goStrings(g.Descs))
	g.Init.WriteString("\tzpGrammar.Expectations = true\n")
}

// joinPrefixed returns the items separated by commas, with a comma before the first.
func joinPrefixed(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return ", " + strings.Join(items, ", ")
}
