package engine

import (
	"context"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/engine/vm"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/runtime/parse"
)

// Parse parses the whole input with the rule start.
// It also returns a *SyntaxError if only part of the input matched.
// If errors were recovered with #recover, it returns SyntaxErrors along with the resulting node.
// If the whole input still did not match after recovery, it returns only the last error.
func (prog *Program) Parse(start, input string) (*parse.Node, error) {
	return prog.ParseWith(start, input, ParseOptions{})
}

// ParseWith parses with options such as the position unit.
func (prog *Program) ParseWith(start, input string, o ParseOptions) (n *parse.Node, err error) {
	if err := core.ValidateStreamDiagnostics(o, false); err != nil {
		return nil, err
	}
	// Initialization can interrupt before run installs its execution recovery.
	// Preserve caller panics: run already distinguishes bytecode faults from
	// caller panics, so an outer boundary must not classify them again.
	defer func() {
		if x := recover(); x != nil {
			if f, ok := x.(core.Fatal); ok {
				n, err = nil, f.Err
			} else if cp, ok := x.(core.ContextPanic); ok {
				panic(cp.Value)
			} else {
				panic(x)
			}
		}
	}()
	start = prog.ResolveStart(start) // an empty name selects the default start rule
	if err := prog.checkMode(o, false); err != nil {
		return nil, err
	}
	active, _, control, err := prog.admitParse(start, o, optionsFamily(o), false, input)
	if err != nil {
		return nil, err
	}
	control.Check()
	p := core.AcquirePooledParser(&active.sharedProgram)
	defer p.ReleaseBatchScratch()
	p.Input = core.NewControlledTextInput(input, o.Unit, !o.Recognize, p.Scratch, control)
	control.AttachMemory(p)
	p.MaxDepth, _ = core.DepthLimit(o, o.Backend) // admitParse rejected an invalid limit
	p.Expectations = o.Expectations
	p.DeferMemo = true
	p.ContextMemoOff = o.ContextMemoOff
	p.ContextMemoUnexamined = o.ContextMemoUnexamined
	if o.MissingTokens > 0 && active.HasMissing {
		p.Missing, p.MissingLen = &core.MissingTokenState{Limit: o.MissingTokens}, 0
	}
	p.SetTrace(o.Trace)
	n, err = active.Run(p, o.Backend, start)
	if !p.Aborted {
		control.Check()
	}
	if o.Recognize {
		n = nil
	}
	return n, err
}

// recognizable reports an error when prog cannot recognize: without the grammar, only the module
// of a compiled grammar in recognize mode recognizes. It builds nothing, so parses check it before
// any other work.
func (prog *Program) recognizable() error {
	if prog.Grammar != nil || prog.loaded && prog.mode == CompiledRecognize {
		return nil
	}
	return modeError(prog.mode, CompiledRecognize, "recognition needs")
}

// Recognizer returns a program that only checks whether the input conforms to the grammar,
// without building a tree (recognition); it is built on first use. The memo for value-free rules
// belongs to that separate program, so it does not mix with that of ordinary parses.
func (prog *Program) Recognizer() (*Program, error) {
	prog.recOnce.Do(func() {
		if prog.Grammar == nil {
			if prog.recErr = prog.recognizable(); prog.recErr == nil {
				prog.rec = prog // the module of a compiled grammar in recognize mode is the recognizer's
			}
			return
		}
		flags := prog.Static().Flags()
		prog.rec, prog.recErr = build(prog.SourceGrammar(), prog.lazyOptions(func(o *compile.Options) {
			o.Recognize, o.Recognizer = true, true
		}), flags)
	})
	return prog.rec, prog.recErr
}

// insertionProgram returns the program that the parses with missing-token insertion enabled run
// when prog's grammar has left recursion through an insertion: the grammar built with the rule
// flags of insertion (compile.Program.InsertionFlags), whose leaders grow the cycles that exist
// only with insertion. It is built on first use. The parses without insertion keep running prog,
// so they neither grow those rules nor lose the memoization of their members.
func (prog *Program) insertionProgram() (*Program, error) {
	prog.insOnce.Do(func() {
		prog.ins, prog.insErr = build(prog.SourceGrammar(), prog.lazyOptions(func(*compile.Options) {}), prog.InsertionFlags)
	})
	return prog.ins, prog.insErr
}

// backend resolves backend b, choosing one based on the grammar when b is the default.
func (prog *Program) backend(b Backend) Backend {
	if b == Default {
		if prog.Grammar == nil {
			return Bytecode
		}
		return Closure
	}
	return b
}

// optionsFamily is core.FamilyOf for a parse with the options o that does not read a Document (a Document's
// parses run the call-preserving family): such a parse sets NoPlain exactly when it is traced
// (core.Parser.SetTrace).
func optionsFamily(o ParseOptions) core.Family {
	if o.Trace != nil {
		return core.CallPreserving
	}
	return core.Inlined
}

// familyRules returns the rules that the closure backend runs in the family f, by ID (rule.id),
// building them on first use. The build of the Core IR polls the context of c, the parse's budget
// control (nil for none); if the context ends, familyRules returns its error and leaves the family
// unbuilt. The build that returns last among those that read the Core IR drops it (closureCore); a
// build that panicked, other than by a panic of the parse's context, counts as returned, since its
// family only reports an error after.
func (prog *Program) familyRules(f core.Family, c *core.BudgetControl) ([]*core.Rule, error) {
	fam := &prog.families[f]
	if !fam.built.Load() {
		if err := prog.buildFamilyRules(fam, f, c); err != nil {
			return nil, err
		}
	}
	return fam.rules, fam.err
}

// buildFamilyRules builds the family f into fam unless it is built (familyRules). It returns an
// error only for a build that the parse's context cancelled. The Core IR is built before fam.mu is
// taken (closureCore.module), since its build calls the parse's context; fam.mu is held only while
// the closures, which call no context method, are built.
func (prog *Program) buildFamilyRules(fam *closureFamily, f core.Family, c *core.BudgetControl) error {
	if prog.buildFamily == nil {
		fam.mu.Lock()
		defer fam.mu.Unlock()
		fam.err = errClosureNeedsSource
		fam.built.Store(true)
		return nil
	}
	var ctx context.Context
	if c != nil {
		ctx = c.Ctx
	}
	m, err := prog.coreModule(fam, f, ctx)
	if cancelled, ok := err.(buildCancelled); ok {
		return cancelled.err
	}
	fam.mu.Lock()
	defer fam.mu.Unlock()
	if fam.built.Load() {
		return nil
	}
	returned := false
	defer func() {
		if !returned {
			fam.err = fmt.Errorf("internal error: the closure family %d was not built", f)
			fam.built.Store(true)
			prog.coreIR.done()
		}
	}()
	if err == nil {
		fam.rules, err = prog.buildFamily(m, f)
	}
	returned = true
	fam.err = err
	fam.built.Store(true)
	prog.coreIR.done()
	return nil
}

// coreModule returns the Core IR for the build of the family f (closureCore.module). A panic of the
// build that is not the parse context's (core.ContextPanic) is recorded on fam as an internal error,
// as a panic of the closures' build is, and re-raised.
func (prog *Program) coreModule(fam *closureFamily, f core.Family, ctx context.Context) (*ir.Module, error) {
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(core.ContextPanic); !ok {
				fam.mu.Lock()
				if !fam.built.Load() {
					fam.err = fmt.Errorf("internal error: the closure family %d was not built", f)
					fam.built.Store(true)
					prog.coreIR.done()
				}
				fam.mu.Unlock()
			}
			panic(x)
		}
	}()
	return prog.coreIR.module(ctx)
}

// closureRule returns the rule the closure backend runs for the rule name in the family f, or nil
// if there is none.
func (prog *Program) closureRule(name string, f core.Family, c *core.BudgetControl) (*core.Rule, error) {
	sr := prog.ByName[name]
	if sr == nil {
		return nil, nil
	}
	rules, err := prog.familyRules(f, c)
	if err != nil {
		return nil, err
	}
	return rules[sr.ID], nil
}

// rule returns the rule name for backend b; the closure backend returns it from the family f, which
// it builds on first use under the parse's budget control c (familyRules).
func (prog *Program) rule(b Backend, name string, f core.Family, c *core.BudgetControl) (*core.Rule, error) {
	if err := desugar.EntryError(name); err != nil {
		return nil, err
	}
	if prog.HasMissing && prog.backend(b) != Closure {
		return nil, compile.MissingUnsupported("bytecode backends")
	}
	var r *core.Rule
	switch prog.backend(b) {
	case Bytecode, BytecodeIterative:
		v, err := prog.prepareVM(prog.backend(b), f)
		if err != nil {
			return nil, err
		}
		r = v.Rule(name)
	case Closure:
		if prog.Grammar == nil {
			return nil, errClosureNeedsSource
		}
		var err error
		if r, err = prog.closureRule(name, f, c); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown backend %d", b)
	}
	if r == nil {
		return nil, fmt.Errorf("rule %s is not defined", name)
	}
	return r, nil
}

// familyVM is the VM program of one VM and family, prepared on first use.
type familyVM struct {
	once sync.Once
	vm   *vm.Program
}

// vmIndex returns the index of the VM backend b (Bytecode or BytecodeIterative) in Program.vms.
func vmIndex(b Backend) int {
	if b == BytecodeIterative {
		return 1
	}
	return 0
}

// prepareVM returns the VM program of the VM backend b (Bytecode or BytecodeIterative) in the family
// f, preparing it on first use.
func (prog *Program) prepareVM(b Backend, f core.Family) (*vm.Program, error) {
	m, terms, err := prog.moduleTerms()
	if err != nil {
		return nil, err
	}
	v := &prog.vms[vmIndex(b)][f]
	v.once.Do(func() {
		if b == BytecodeIterative {
			v.vm = vm.NewIterative(m, terms, f)
		} else {
			v.vm = vm.New(m, terms, f)
		}
	})
	return v.vm, nil
}

// vmFor returns the VM program of the VM backend b in the family f, which rule has prepared.
func (prog *Program) vmFor(b Backend, f core.Family) *vm.Program { return prog.vms[vmIndex(b)][f].vm }

// descs returns the expectation table of backend b in the family f (call it after rule has
// prepared it).
func (prog *Program) descs(b Backend, f core.Family) []string {
	if be := prog.backend(b); be == Bytecode || be == BytecodeIterative {
		return prog.vmFor(be, f).Descs()
	}
	return prog.DescTable
}

// recoverParse turns an aborted parse (fatal) into an error. Runtime faults in bytecode
// execution are reported as errors caused by an invalid module.
func recoverParse(x any, b Backend, err *error) {
	if x == nil {
		return
	}
	if f, ok := x.(core.Fatal); ok {
		*err = f.Err
		return
	}
	if tp, ok := x.(core.TracePanic); ok {
		panic(tp.Value) // a panic of the trace function, not of the parse
	}
	if cp, ok := x.(core.ContextPanic); ok {
		panic(cp.Value)
	}
	if re, ok := x.(runtime.Error); ok && (b == Bytecode || b == BytecodeIterative) {
		*err = fmt.Errorf("invalid bytecode: %v", re)
		return
	}
	panic(x)
}

// Run parses the whole input of p with the start rule start.
func (prog *Program) Run(p *core.Parser, b Backend, start string) (n *parse.Node, err error) {
	p.Aborted = false
	r, err := prog.rule(b, start, core.FamilyOf(p), p.Control)
	if err != nil {
		return nil, err
	}
	p.Descs = prog.descs(b, core.FamilyOf(p))
	p.Memo.Stride = prog.NSeen
	if be := prog.backend(b); be == Bytecode || be == BytecodeIterative {
		p.Memo.Stride = prog.vmFor(be, core.FamilyOf(p)).NumSeen()
	}
	p.Memo.Sparse = core.SparseMemo && p.Memo.Stride > 64 && p.Loaded() > 1024
	p.SeedVars()
	defer func() {
		if x := recover(); x != nil {
			p.Aborted = true
			n = nil
			recoverParse(x, prog.backend(b), &err)
		}
	}()
	v, ok := p.Call(r, 0)
	if ok && p.AtEOF() {
		if len(p.Recovered) > 0 {
			return v, parse.SyntaxErrors(p.RecoveredErrors())
		}
		return v, nil
	}
	if ok {
		p.Expect(p.Pos, compile.IDEndInput)
	}
	return nil, p.SyntaxError()
}

// Rules returns the names of the rules that can be entry points, in declaration order. A hidden
// rule is not one.
func (prog *Program) Rules() []string {
	if prog.Grammar == nil {
		m := prog.Module()
		var names []string
		seen := map[string]bool{}
		for _, r := range m.Rules {
			if name := m.Strings[r.Name]; !seen[name] && !desugar.HiddenRule(name) { // value-free twins follow the original rules
				seen[name] = true
				names = append(names, name)
			}
		}
		return names
	}
	names := make([]string, 0, len(prog.RuleList))
	for _, r := range prog.RuleList {
		if !desugar.HiddenRule(r.Name) {
			names = append(names, r.Name)
		}
	}
	return names
}

// ParseStream parses with the rule start while reading from r.
// Each element of the #stream repetition at the top level of start's body is passed to emit as
// soon as it matches. Passed elements are not retained and the input before them is discarded,
// so the whole input never has to be held in memory.
// If emit returns an error, the parse is aborted and that error is returned.
func (prog *Program) ParseStream(start string, r io.Reader, emit func(*parse.Node) error) error {
	return prog.ParseStreamWith(start, r, emit, ParseOptions{})
}

// ParseStreamWith parses a stream with options such as the position unit.
func (prog *Program) ParseStreamWith(start string, r io.Reader, emit func(*parse.Node) error, o ParseOptions) (err error) {
	// Admission invokes caller-owned Context methods before execution recovery.
	// Preserve their panic values rather than classifying them as VM faults.
	defer func() {
		if x := recover(); x != nil {
			if f, ok := x.(core.Fatal); ok {
				err = f.Err
			} else if cp, ok := x.(core.ContextPanic); ok {
				panic(cp.Value)
			} else {
				panic(x)
			}
		}
	}()
	if err := core.ValidateStreamDiagnostics(o, true); err != nil {
		return err
	}
	if o.Recognize {
		return fmt.Errorf("recognition is not supported for stream parsing")
	}
	start = prog.ResolveStart(start) // an empty name selects the default start rule
	if err := prog.checkMode(o, false); err != nil {
		return err
	}
	_, rule, control, err := prog.admitParse(start, o, optionsFamily(o), true, "")
	if err != nil {
		return err
	}
	control.Check()
	p := core.NewControlledStreamParser(&prog.sharedProgram, r, o.Unit, control)
	control.AttachMemory(p)
	// A limit beyond math.MaxInt bounds no stream.
	if lim := o.Limits.Input; lim != 0 && lim != parse.Unlimited && lim <= math.MaxInt {
		p.Input.MaxInput = int(lim)
	}
	defer p.ReleaseTraceReferences() // after error recovery, including rethrown panics
	control.Check()
	p.MaxDepth, _ = core.DepthLimit(o, o.Backend) // admitParse rejected an invalid limit
	p.Expectations = o.Expectations
	p.Emit, p.StreamID = emit, rule.ID
	p.Diagnostic = o.DiagnosticCallback
	p.DiagnosticTail.Limit = o.DiagnosticLimit
	if (p.Diagnostic != nil || o.DiagnosticLimit > 0) && p.Emit == nil {
		// A diagnostic-only stream still needs element commit boundaries.
		p.Emit = func(*parse.Node) error { return nil }
	}
	if core.StreamPrefixIsolation && p.Emit == nil {
		p.NodeChunks = 0 // No callback: collect the ordinary repetition instead.
	}
	p.Descs = prog.descs(o.Backend, optionsFamily(o))
	p.SetTrace(o.Trace)
	defer func() {
		x := recover()
		if x != nil && (p.ReaderCall || p.EmitCall) {
			// A caller panic is not a VM fault. Trace callbacks may have
			// already attributed a nested reader panic; unwrap it as usual.
			if tp, ok := x.(core.TracePanic); ok {
				panic(tp.Value)
			}
			panic(x)
		}
		recoverParse(x, prog.backend(o.Backend), &err)
	}()
	control.Check()
	p.SeedVars()
	_, ok := p.Call(rule, 0)
	if ok && p.AtEOF() {
		control.Check()
		if p.Diagnostic != nil || p.DiagnosticTail.Limit > 0 {
			if err := p.ReportRecovered(); err != nil {
				return err
			}
		}
		if p.DiagnosticTail.Limit > 0 {
			return p.DiagnosticTail.Result(p)
		}
		if len(p.Recovered) > 0 {
			return parse.SyntaxErrors(p.RecoveredErrors())
		}
		return nil
	}
	if ok {
		p.Expect(p.Pos, compile.IDEndInput)
	}
	err = p.SyntaxError()
	control.Check()
	return err
}

func (prog *Program) admitMissing(o ParseOptions, stream bool) error {
	if o.MissingTokens < 0 {
		return fmt.Errorf("missing token limit must be nonnegative")
	}
	if stream && (prog.HasMissing || o.MissingTokens > 0) {
		return compile.MissingUnsupported("stream parsing")
	}
	if prog.backend(o.Backend) != Closure && (prog.HasMissing || o.MissingTokens > 0) {
		return compile.MissingUnsupported("bytecode backends")
	}
	return nil
}

// lazyOptions returns the options for a program that is built later from the grammar of prog (the
// recognizer, the program of missing-token insertion): the options prog was compiled with, changed by derive
// for what the build is for. This is the one place that makes them, so no option of prog is
// dropped from a lazily built program (TestLazyProgramsCarryEveryOption).
func (prog *Program) lazyOptions(derive func(*compile.Options)) compile.Options {
	o := prog.Opts
	derive(&o)
	return o
}
