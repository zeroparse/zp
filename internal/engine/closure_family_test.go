//go:build !zp_budget_controls_reference

package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"weak"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/closure"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/gentest"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/runtime/parse"
)

// leastCompute returns the smallest compute limit under which run succeeds.
func leastCompute(t *testing.T, run func(parse.Limits) error) uint64 {
	t.Helper()
	fits := func(b uint64) bool { return run(parse.Limits{Compute: b}) == nil }
	hi := uint64(1)
	for !fits(hi) {
		if hi > 1<<30 {
			t.Fatalf("the parse fails under every limit: %v", run(parse.Limits{}))
		}
		hi *= 2
	}
	lo := hi / 2
	for lo+1 < hi {
		mid := (lo + hi) / 2
		if fits(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// familyCharges returns the least compute limit of every kind of parse of input by prog on the
// backend b, untraced and traced: each kind runs the family that its options select.
func familyCharges(t *testing.T, prog *Program, b Backend, input string, stream bool) map[string]uint64 {
	t.Helper()
	out := map[string]uint64{}
	for _, traced := range []bool{false, true} {
		o := ParseOptions{Expectations: true, Backend: b}
		suffix := ""
		if traced {
			o.Trace, suffix = func(TraceEvent) {}, "+trace"
		}
		with := func(l parse.Limits) ParseOptions { x := o; x.Limits = l; return x }
		if stream {
			out["stream"+suffix] = leastCompute(t, func(l parse.Limits) error {
				return prog.ParseStreamWith("main", strings.NewReader(input), func(*parse.Node) error { return nil }, with(l))
			})
			continue
		}
		out["parse"+suffix] = leastCompute(t, func(l parse.Limits) error { _, err := prog.ParseWith("main", input, with(l)); return err })
		out["recognize"+suffix] = leastCompute(t, func(l parse.Limits) error {
			x := with(l)
			x.Recognize = true
			_, err := prog.ParseWith("main", input, x)
			return err
		})
		out["prefix"+suffix] = leastCompute(t, func(l parse.Limits) error { _, err := prog.ParsePrefixWith("main", input, with(l)); return err })
		out["document"+suffix] = leastCompute(t, func(l parse.Limits) error {
			d, err := prog.NewDocumentWith("main", input, ParseOptions{Expectations: true, Backend: b})
			if err != nil {
				return err
			}
			_, err = d.ParseWith(with(l))
			return err
		})
	}
	return out
}

// TestClosureFamilyCharges pins the budget charge (the least compute limit) of every kind of
// closure parse, traced and untraced. The numbers are those of the closure backend when each call
// site tested Parser.NoPlain at run time: each parse runs the family that keeps them. A call of a
// plain rule through Parser.Call charges more than the plain path, so a parse that ran the wrong
// family changes a number. A stream parse of the default build also charges for walking and
// clearing the capture frames it keeps for reuse at each commit, which the reference build of
// stream frames drops instead, so the stream numbers are those of the build.
func TestClosureFamilyCharges(t *testing.T) {
	t.Parallel()
	stream := map[string]uint64{"stream": 141, "stream+trace": 151}
	if !core.StreamFrameScratch {
		stream = map[string]uint64{"stream": 137, "stream+trace": 147}
	}
	cases := []struct {
		name, src string
		opts      compile.Options
		input     string
		stream    bool
		want      map[string]uint64
	}{
		{"plain calls", "rule main = item* $$\nrule item = \"a\"", compile.Options{}, "aaaa", false, map[string]uint64{"document": 65, "document+trace": 73, "parse": 35, "parse+trace": 48, "prefix": 34, "prefix+trace": 47, "recognize": 27, "recognize+trace": 40}},
		{"plain calls, no memo", "rule main = item* $$\nrule item = \"a\"", compile.Options{DisableMemo: true}, "aaaa", false, map[string]uint64{"document": 38, "document+trace": 46, "parse": 35, "parse+trace": 48, "prefix": 34, "prefix+trace": 47, "recognize": 27, "recognize+trace": 40}},
		{"recursion", "rule main = e $$\nrule e = \"(\" e \")\" / x\nrule x = \"x\"", compile.Options{DisableMemo: true}, "((x))", false, map[string]uint64{"document": 20, "document+trace": 29, "parse": 26, "parse+trace": 39, "prefix": 25, "prefix+trace": 38, "recognize": 26, "recognize+trace": 39}},
		{"captures", "rule main = item* $$\nrule item = x:\"a\" {len(text($x)) == 1}", compile.Options{}, "aaa", false, map[string]uint64{"document": 62, "document+trace": 69, "parse": 41, "parse+trace": 48, "prefix": 40, "prefix+trace": 47, "recognize": 29, "recognize+trace": 36}},
		{"stream", gentest.Records, compile.Options{}, "#records\na=1\nbc=22\n", true, stream},
	}
	for _, c := range cases {
		prog := mustCompile(t, c.src, c.opts)
		got := familyCharges(t, prog, Closure, c.input, c.stream)
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: charges %v, want %v", c.name, got, c.want)
		}
	}
}

// parseInFamily parses input with the start rule main of prog on the backend b in the family f,
// with no tracer, and returns the result as ResultJSON and the parse statistics.
func parseInFamily(prog *Program, b Backend, input string, unit parse.PositionUnit, f core.Family) (string, parse.ParseStats) {
	p := core.AcquirePooledParser(&prog.sharedProgram)
	defer p.ReleaseBatchScratch()
	p.Input = core.NewControlledTextInput(input, unit, true, p.Scratch, nil)
	p.MaxDepth = DefaultMaxDepth
	p.Expectations = true
	p.DeferMemo = true
	p.NoPlain = f == core.CallPreserving
	n, err := prog.Run(p, b, "main")
	return gentest.ResultJSON(n, err), p.ReportedStats()
}

// TestClosureFamiliesMatchOnCorpus checks that the two closure families give the same results,
// errors, expectations, recovered errors, positions and statistics on every grammar and input of
// the corpus, when no tracer observes the calls.
func TestClosureFamiliesMatchOnCorpus(t *testing.T) {
	t.Parallel()
	for _, c := range gentest.GenCorpus(t) {
		t.Run(c.Name, func(t *testing.T) {
			for _, o := range []compile.Options{{}, {DisableMemo: true}} {
				prog := mustCompile(t, c.Src, o)
				for _, in := range c.Inputs {
					for _, u := range []parse.PositionUnit{parse.CodePoints, parse.Bytes} {
						a, as := parseInFamily(prog, Closure, in, u, core.Inlined)
						b, bs := parseInFamily(prog, Closure, in, u, core.CallPreserving)
						if a != b || as != bs {
							t.Errorf("memo off=%t input %q (%v): families differ\n inlined         %s %+v\n call-preserving %s %+v", o.DisableMemo, in, u, a, as, b, bs)
						}
					}
				}
			}
		})
	}
}

// TestClosureFamiliesBuiltOnFirstUse checks that a program builds each closure family only when a
// parse first runs it: ordinary parses the inlined one, traced and Document parses the
// call-preserving one, and that the families run distinct rules.
func TestClosureFamiliesBuiltOnFirstUse(t *testing.T) {
	t.Parallel()
	const src = "rule main = item* $$\nrule item = \"a\""
	// A parse under budget limits (the default ones too) runs the metered program that the program
	// builds for them (prepareControlledProgram), so a family counts as built in either.
	built := func(prog *Program) [2]bool {
		var got [2]bool
		for _, q := range []*Program{prog, prog.budgetProgram} {
			if q != nil {
				got[0] = got[0] || q.families[core.Inlined].rules != nil
				got[1] = got[1] || q.families[core.CallPreserving].rules != nil
			}
		}
		return got
	}
	prog := mustCompile(t, src)
	if got := built(prog); got != [2]bool{} {
		t.Fatalf("after Compile: built %v, want none", got)
	}
	if _, err := prog.Parse("main", "aa"); err != nil {
		t.Fatal(err)
	}
	if got := built(prog); got != [2]bool{true, false} {
		t.Fatalf("after a parse: built %v, want the inlined family", got)
	}
	if _, err := prog.ParseWith("main", "aa", ParseOptions{Trace: func(TraceEvent) {}}); err != nil {
		t.Fatal(err)
	}
	if got := built(prog); got != [2]bool{true, true} {
		t.Fatalf("after a traced parse: built %v, want both families", got)
	}
	a, err := prog.rule(Closure, "item", core.Inlined, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := prog.rule(Closure, "item", core.CallPreserving, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || a.ID != b.ID || !a.Plain || !b.Plain {
		t.Fatalf("item: inlined %p (ID %d, plain %t), call-preserving %p (ID %d, plain %t)", a, a.ID, a.Plain, b, b.ID, b.Plain)
	}

	doc := mustCompile(t, src)
	d, err := doc.NewDocument("main", "aa")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Parse(); err != nil {
		t.Fatal(err)
	}
	if got := built(doc); got != [2]bool{false, true} {
		t.Fatalf("after a Document parse: built %v, want the call-preserving family", got)
	}

	stream := mustCompile(t, gentest.Records)
	if err := stream.ParseStreamWith("main", strings.NewReader("#records\na=1\n"), func(*parse.Node) error { return nil }, ParseOptions{Trace: func(TraceEvent) {}}); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if got := built(stream); got != [2]bool{false, true} {
		t.Fatalf("after a traced stream parse: built %v, want the call-preserving family", got)
	}
}

// TestClosureFamiliesLeaveModuleUnchanged checks that building the closures of both families only
// reads the Core IR module that they share: the module prints the same, with its attributes, before
// and after the builds of every grammar of the corpus.
func TestClosureFamiliesLeaveModuleUnchanged(t *testing.T) {
	t.Parallel()
	for _, c := range gentest.GenCorpus(t) {
		t.Run(c.Name, func(t *testing.T) {
			sp := mustCompile(t, c.Src).Static()
			m, err := closure.Core(sp, nil)
			if err != nil {
				t.Fatal(err)
			}
			before := ir.Print(m, true)
			for _, f := range []core.Family{core.Inlined, core.CallPreserving} {
				for _, plainDispatch := range []bool{true, false} {
					if _, err := closure.Build(sp, m, plainDispatch, f); err != nil {
						t.Fatal(err)
					}
					if after := ir.Print(m, true); after != before {
						t.Fatalf("family %d, plainDispatch %t: the build changed the module:\n%s\nbefore:\n%s", f, plainDispatch, after, before)
					}
				}
			}
		})
	}
}

// TestClosureFamiliesBuildConcurrently builds both families of each corpus grammar from one module
// at once, as two parses of different families may. Build only reads the module; under the race
// detector a write to it, also to a part that Print does not show, fails this test.
func TestClosureFamiliesBuildConcurrently(t *testing.T) {
	t.Parallel()
	for _, c := range gentest.GenCorpus(t) {
		t.Run(c.Name, func(t *testing.T) {
			sp := mustCompile(t, c.Src).Static()
			m, err := closure.Core(sp, nil)
			if err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			errs := make([]error, 4)
			for i := range errs {
				wg.Go(func() {
					_, errs[i] = closure.Build(sp, m, i%2 == 0, core.Family(i/2))
				})
			}
			wg.Wait()
			for i, err := range errs {
				if err != nil {
					t.Errorf("build %d: %v", i, err)
				}
			}
		})
	}
}

// TestClosureFamilyBuildPanicIsAnError checks that a family whose build panicked reports an error
// on later use instead of running no rules.
func TestClosureFamilyBuildPanicIsAnError(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = "a" $$`)
	prog.buildFamily = func(*ir.Module, core.Family) ([]*core.Rule, error) { panic("build failed") }
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("the first use did not panic")
			}
		}()
		_, _ = prog.familyRules(core.Inlined, nil)
	}()
	if _, err := prog.familyRules(core.Inlined, nil); err == nil || !strings.Contains(err.Error(), "was not built") {
		t.Fatalf("err = %v, want the family not built", err)
	}
	if r, err := prog.rule(Closure, "main", core.Inlined, nil); err == nil {
		t.Fatalf("the start rule %v of a family that was not built", r)
	}
}

// TestClosureFamiliesReleaseTheCoreIR checks that a program builds its Core IR with the first
// closure family, keeps it until both families are built, and that nothing retains the module
// after that.
func TestClosureFamiliesReleaseTheCoreIR(t *testing.T) {
	prog := mustCompile(t, `rule main = item+ $$
rule item = "a" / "b"`)
	if prog.coreIR.m != nil {
		t.Fatal("Compile built the Core IR before a family needed it")
	}
	if _, err := prog.familyRules(core.Inlined, nil); err != nil {
		t.Fatal(err)
	}
	if prog.coreIR.m == nil {
		t.Fatal("the first family did not build the Core IR")
	}
	module := weak.Make(prog.coreIR.m)
	if prog.coreIR.m == nil {
		t.Fatal("the Core IR was dropped while the call-preserving family was not built")
	}
	if _, err := prog.familyRules(core.CallPreserving, nil); err != nil {
		t.Fatal(err)
	}
	if prog.coreIR.m != nil {
		t.Fatal("the Core IR is kept after both families were built")
	}
	runtime.GC()
	if module.Value() != nil {
		t.Fatal("the Core IR is still reachable after both families were built")
	}
	for _, f := range []core.Family{core.Inlined, core.CallPreserving} {
		if _, err := prog.rule(Closure, "main", f, nil); err != nil {
			t.Errorf("family %d after the release: %v", f, err)
		}
	}
	runtime.KeepAlive(prog)
}

// TestMeteredProgramSharesTheCoreIR checks that the metered program of a program runs the same
// static program and builds its closure families from the same Core IR, which stays until the
// families of both programs are built, and that a metered program made after the release builds
// the Core IR again.
func TestMeteredProgramSharesTheCoreIR(t *testing.T) {
	if !core.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	const src = "rule main = item+ $$\nrule item = \"a\" / \"b\""
	prog := mustCompile(t, src)
	metered := prog.meteredProgram()
	if metered.Static() != prog.Static() || metered.coreIR != prog.coreIR {
		t.Fatal("the metered program does not share the static program and the Core IR")
	}
	var m *ir.Module
	for _, q := range []*Program{prog, metered} {
		for _, f := range []core.Family{core.Inlined, core.CallPreserving} {
			if m == nil {
				if _, err := q.familyRules(f, nil); err != nil {
					t.Fatal(err)
				}
				m = prog.coreIR.m
				continue
			}
			if q == metered && f == core.CallPreserving {
				if prog.coreIR.m != m {
					t.Fatal("the Core IR was dropped or built again while a family was not built")
				}
			}
			if _, err := q.familyRules(f, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	if prog.coreIR.m != nil {
		t.Fatal("the Core IR is kept after the families of both programs were built")
	}
	if a, b := prog.families[core.Inlined].rules, metered.families[core.Inlined].rules; len(a) == 0 || &a[0] == &b[0] {
		t.Fatal("the metered program runs the rules of the plain one")
	}

	late := mustCompile(t, src)
	for _, f := range []core.Family{core.Inlined, core.CallPreserving} {
		if _, err := late.familyRules(f, nil); err != nil {
			t.Fatal(err)
		}
	}
	lateMetered := late.meteredProgram()
	if _, err := lateMetered.familyRules(core.Inlined, nil); err != nil {
		t.Fatal(err)
	}
	if late.coreIR.m == nil {
		t.Fatal("the metered program made after the release did not build the Core IR again")
	}
	if _, err := lateMetered.ParseWith("main", "ab", ParseOptions{Backend: Closure}); err != nil {
		t.Fatal(err)
	}
}

// inCoreBuild reports whether the calling goroutine is inside the build of the Core IR of the
// closures (closure.Core).
func inCoreBuild() bool {
	buf := make([]byte, 64<<10)
	return strings.Contains(string(buf[:runtime.Stack(buf, false)]), "/internal/engine/closure.Core(")
}

// buildHookContext is a live context whose Err calls hook when it is called from inside the build
// of the Core IR (inCoreBuild) and returns hook's error; elsewhere it returns nil. calls counts the
// calls of hook. It is for one goroutine at a time: a parse that uses it must not run concurrently
// with another parse that uses it.
type buildHookContext struct {
	context.Context
	hook  func() error
	calls int
}

func (c *buildHookContext) Err() error {
	if !inCoreBuild() {
		return nil
	}
	c.calls++
	return c.hook()
}

// closureCoreGrammar is a grammar whose Core IR build polls its context more than once.
func closureCoreGrammar() string {
	var src strings.Builder
	src.WriteString("rule main = (")
	for i := range 40 {
		if i > 0 {
			src.WriteString(" / ")
		}
		fmt.Fprintf(&src, "r%d", i)
	}
	src.WriteString(")+ $$\n")
	for i := range 40 {
		fmt.Fprintf(&src, "rule r%d = \"k%d\" [a-z]* (\",\" / \";\")?\n", i, i)
	}
	return src.String()
}

// TestClosureCoreBuildObservesTheParseContext checks that the parse that first needs a closure
// family interrupts the build of the Core IR when its context ends, reports the context's error,
// and leaves the family and the Core IR unbuilt for the next parse, which builds them.
func TestClosureCoreBuildObservesTheParseContext(t *testing.T) {
	prog := mustCompile(t, closureCoreGrammar())
	base, stop := context.WithCancel(context.Background())
	defer stop()
	ctx := &buildHookContext{Context: base, hook: func() error { return context.Canceled }}
	if _, err := prog.ParseWith("main", "k1ab,k7", ParseOptions{Context: ctx}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the context's error", err)
	}
	if ctx.calls != 1 {
		t.Fatalf("%d checks inside the Core IR build saw the context ended, want 1", ctx.calls)
	}
	metered := prog.budgetProgram
	if metered == nil || metered.families[core.Inlined].built.Load() || prog.coreIR.m != nil {
		t.Fatal("a cancelled build left a family or the Core IR built")
	}
	if want := 2 * len(prog.families); prog.coreIR.pending != want {
		t.Fatalf("pending = %d after a cancelled build, want %d", prog.coreIR.pending, want)
	}
	if n, err := prog.ParseWith("main", "k1ab,k7", ParseOptions{Context: base}); n == nil || err != nil {
		t.Fatalf("the parse after the cancelled one: node=%v err=%v", n, err)
	}
	for _, q := range []*Program{prog, metered} {
		for f := range q.families {
			if _, err := q.familyRules(core.Family(f), nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	if prog.coreIR.m != nil || prog.coreIR.pending != 0 {
		t.Fatalf("after every family: Core IR held %v, pending %d", prog.coreIR.m != nil, prog.coreIR.pending)
	}
}

// TestClosureCoreBuildAllowsReentrantContexts checks that a context method called during the build
// of the Core IR may parse on the same program, in the metered and in the plain families, without
// deadlocking the build, and that both parses succeed.
func TestClosureCoreBuildAllowsReentrantContexts(t *testing.T) {
	prog := mustCompile(t, closureCoreGrammar())
	base, stop := context.WithCancel(context.Background())
	defer stop()
	var nested []error
	ctx := &buildHookContext{Context: base}
	ctx.hook = func() error {
		if ctx.calls == 1 {
			for _, o := range []ParseOptions{{Context: base}, {Limits: parse.Limits{Compute: parse.Unlimited}}} {
				_, err := prog.ParseWith("main", "k3x;k9", o)
				nested = append(nested, err)
			}
		}
		return nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := prog.ParseWith("main", "k1ab,k7", ParseOptions{Context: ctx})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the parse whose context reentered: %v", err)
		}
	case <-time.After(time.Minute):
		t.Fatal("a parse from a context method during the Core IR build deadlocked")
	}
	if len(nested) != 2 || nested[0] != nil || nested[1] != nil {
		t.Fatalf("the nested parses: %v", nested)
	}
}

// TestClosureCoreBuildContextPanic checks that a panic of a context method during the build of the
// Core IR reaches the caller as its own value and leaves the family unbuilt, so that a later parse
// with a well-behaved context builds it.
func TestClosureCoreBuildContextPanic(t *testing.T) {
	prog := mustCompile(t, closureCoreGrammar())
	base, stop := context.WithCancel(context.Background())
	defer stop()
	ctx := &buildHookContext{Context: base, hook: func() error { panic("context failed") }}
	func() {
		defer func() {
			if x := recover(); x != "context failed" {
				t.Fatalf("recovered %#v, want the context's own panic value", x)
			}
		}()
		_, _ = prog.ParseWith("main", "k1ab,k7", ParseOptions{Context: ctx})
	}()
	metered := prog.budgetProgram
	if metered == nil || metered.families[core.Inlined].built.Load() || prog.coreIR.m != nil {
		t.Fatal("a build interrupted by a context panic left a family or the Core IR built")
	}
	if want := 2 * len(prog.families); prog.coreIR.pending != want {
		t.Fatalf("pending = %d after a context panic, want %d", prog.coreIR.pending, want)
	}
	if n, err := prog.ParseWith("main", "k1ab,k7", ParseOptions{Context: base}); n == nil || err != nil {
		t.Fatalf("the parse after the context panic: node=%v err=%v", n, err)
	}
}

// TestClosureCoreBuildPanicIsAnError checks that a panic of the Core IR build that is not the
// context's reaches the first parse and makes the family report an internal error on later use, as a
// panic of the closures' build does, and that the family counts as returned.
func TestClosureCoreBuildPanicIsAnError(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = "a" $$`)
	// A rule list with a nil rule makes the Core IR builder panic (compile.Program.CoreInput).
	prog.coreIR.sp.RuleList = append(prog.coreIR.sp.RuleList, nil)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("the first use did not panic")
			}
		}()
		_, _ = prog.familyRules(core.Inlined, nil)
	}()
	if _, err := prog.familyRules(core.Inlined, nil); err == nil || !strings.Contains(err.Error(), "was not built") {
		t.Fatalf("err = %v, want the family not built", err)
	}
	if want := len(prog.families) - 1; prog.coreIR.pending != want {
		t.Fatalf("pending = %d, want %d", prog.coreIR.pending, want)
	}
}
