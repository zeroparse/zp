package driver

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/middleend/execlower"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/internal/middleend/flowpass"
	"zeroparse.dev/zp/internal/pass"
)

// The driver runs the Core passes that the settings leave on, dumps the module after the selected
// names under the label of its table, calls the hook after every pass, and asks for the
// verification after each pass when VerifyEach is set.
func TestDriverCore(t *testing.T) {
	off, err := pass.Settings{}.Without(corepass.PassScans)
	if err != nil {
		t.Fatal(err)
	}
	var dump strings.Builder
	var hooked []string
	d := &Driver{Opt: off, PrintAfter: []string{Elaborate, corepass.PassGuards}, PrintTo: &dump, VerifyEach: true,
		Hooks: Hooks{CoreHook: func(pass string, _ *core.Module) { hooked = append(hooked, pass) }}}
	var ran []string
	p := *corepass.Pipeline
	p.Passes = slices.Clone(p.Passes)
	for i := range p.Passes {
		run := p.Passes[i].Run
		name := p.Passes[i].Name
		p.Passes[i].Run = func(m *core.Module, e *corepass.Env) (*core.Module, pass.PreservedSet, error) {
			ran = append(ran, name)
			return run(m, e)
		}
	}
	d.CorePipeline = func(string) *pass.Pipeline[*core.Module, *corepass.Env] { return &p }
	if _, err := d.Core("typed", empty); err != nil {
		t.Fatal(err)
	}
	want := []string{corepass.PassGuards}
	if !slices.Equal(ran, want) {
		t.Errorf("ran %v, want %v", ran, want)
	}
	if !slices.Equal(hooked, append([]string{Elaborate}, want...)) {
		t.Errorf("hook saw %v", hooked)
	}
	if !d.manager().VerifyEach {
		t.Error("VerifyEach did not reach the manager")
	}
	got := dump.String()
	for _, label := range []string{"-- Core IR of the typed table after elaborate --", "-- Core IR of the typed table after guards --"} {
		if !strings.Contains(got, label) {
			t.Errorf("dump lacks %q:\n%s", label, got)
		}
	}
	if strings.Count(got, "-- Core IR") != 2 {
		t.Errorf("dump has other sections than the two selected:\n%s", got)
	}

	// Without a writer nothing is printed; "all" selects every pass that ran.
	dump.Reset()
	d = &Driver{PrintAfter: []string{"all"}}
	if _, err := d.Core("rules", empty); err != nil {
		t.Fatal(err)
	}
	d.PrintTo = &dump
	if _, err := d.Core("rules", empty); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(dump.String(), "-- Core IR of the rules table after "); n != 3 {
		t.Errorf("all printed %d sections, want elaborate, guards and scans:\n%s", n, dump.String())
	}
}

// empty builds an empty Core module, as the step elaborate of a program without rules does.
func empty() (*core.Module, error) { return &core.Module{}, nil }

// A failing pass is reported as an internal error that keeps the cause; a failing build is the
// failure of the step elaborate, with the message of the build.
func TestDriverCoreError(t *testing.T) {
	cause := errors.New("broken")
	p := &pass.Pipeline[*core.Module, *corepass.Env]{Passes: []pass.Pass[*core.Module, *corepass.Env]{{
		Name: "test-core-fail", Obligation: "none: it fails",
		Run: func(m *core.Module, _ *corepass.Env) (*core.Module, pass.PreservedSet, error) {
			return m, pass.PreserveAll(), cause
		}}}}
	d := &Driver{CorePipeline: func(string) *pass.Pipeline[*core.Module, *corepass.Env] { return p }, TestPasses: []string{"test-core-fail"}}
	_, err := d.Core("typed", empty)
	if !errors.Is(err, cause) || !strings.HasPrefix(err.Error(), "internal error: ") {
		t.Fatalf("error %v", err)
	}
	bad := errors.New("no such rule")
	m, err := (&Driver{}).Core("typed", func() (*core.Module, error) { return nil, bad })
	if m != nil || !isStep(err, Elaborate) || !errors.Is(err, bad) || err.Error() != bad.Error() {
		t.Fatalf("a failing build: %v, %v", m, err)
	}
}

// testFlowPass is a Flow pass of the tests: it adds a comment to the function, which keeps it valid.
const testFlowPass = "test-flow-comment"

// withTestFlowPass adds a profile that turns testFlowPass (registered on in speed) off for the
// duration of the test, and returns a FlowPipeline hook that runs a pipeline with the pass.
func withTestFlowPass(t *testing.T) func(string) *pass.Pipeline[*code.Module, *flowpass.Env] {
	defs := profileDefs
	profileDefs = append(slices.Clone(profileDefs), profileDef{Name: "no-comment", Doc: "x", Values: map[string]int{testFlowPass: 0}})
	t.Cleanup(func() { profileDefs = defs })
	p := *flowpass.Pipeline
	p.Passes = append(slices.Clone(p.Passes), pass.Pass[*code.Module, *flowpass.Env]{
		Name: testFlowPass, Obligation: "a comment changes nothing",
		Run: func(m *code.Module, _ *flowpass.Env) (*code.Module, pass.PreservedSet, error) {
			b := flow.FuncOf(m).Body
			b.Stmts = append(b.Stmts, &code.Comment{Text: "tested"})
			return m, pass.PreserveNone(), nil
		}})
	return func(string) *pass.Pipeline[*code.Module, *flowpass.Env] { return &p }
}

// framed returns the unit of a framed function r that matches the literal "a" and builds no value.
func framed() flowlower.Unit {
	lit := &core.Lit{Text: "a", Exp: 5}
	m := &core.Module{Exps: []string{"any character", "beginning of input", "end of input", "beginning of line", "end of line", `"a"`}}
	r := &core.Rule{Name: "r", Body: lit, Policy: core.Policy{Seen: -1}}
	m.Rules = []*core.Rule{r}
	core.Analyze(m)
	return flowlower.Unit{Module: m, Func: flowlower.Func{Form: flowlower.Frame, Rule: r, Name: "r", Body: lit}}
}

// comments counts the comments that testFlowPass added to the function of m.
func comments(m *code.Module) int {
	n := 0
	for _, st := range flow.FuncOf(m).Body.Stmts {
		if c, ok := st.(*code.Comment); ok && c.Text == "tested" {
			n++
		}
	}
	return n
}

// The plan's Exec step lowers a function and ends it with flow-finish without running a Flow pass,
// even one whose flag is on, then converts it and runs the Exec passes, all under the manager: the
// dumps show every step labeled with the table, and the manager numbers the run of the Exec pass.
func TestDriverPlanExec(t *testing.T) {
	const record = "test-plan-exec"
	p := *codepass.Pipeline
	p.Passes = append(slices.Clone(p.Passes), pass.Pass[*code.Module, *codepass.Env]{Name: record, Obligation: "it changes nothing",
		Run: func(m *code.Module, _ *codepass.Env) (*code.Module, pass.PreservedSet, error) {
			return m, pass.PreserveAll(), nil
		}})
	var dump strings.Builder
	d := &Driver{FlowPipeline: withTestFlowPass(t), ExecPipeline: func(string) *pass.Pipeline[*code.Module, *codepass.Env] { return &p },
		TestPasses: []string{record}, PrintAfter: []string{PrintAll}, PrintTo: &dump}
	l, err := d.PlanExec("rules", framed(), false)
	if err != nil {
		t.Fatal(err)
	}
	if l.Func.Name != "r" {
		t.Fatalf("lowered %s, want r", l.Func.Name)
	}
	want := []string{"-- Flow IR of r in the rules table after " + flowlower.Name + " --",
		"-- Flow IR of r in the rules table after " + flowpass.FinishName + " --",
		"-- Exec IR of r in the rules table after " + execlower.Name + " --",
		"-- Exec IR of r in the rules table after " + record + " --"}
	for _, h := range want {
		if !strings.Contains(dump.String(), h) {
			t.Errorf("the dump lacks %q", h)
		}
	}
	if strings.Contains(dump.String(), testFlowPass) {
		t.Errorf("a Flow pass ran in the plan's Exec step:\n%s", dump.String())
	}
	if n := d.run.Runs(); n != 1 {
		t.Errorf("the manager numbered %d runs of optional passes, want the one of %s", n, record)
	}
}

// The driver lowers a function, runs a Flow pass when its flag is on, and a profile or a flag turns
// it off; the dumps show the function as lowered under "flowlower" and after each selected pass, labeled
// with its table.
func TestDriverFlowFunc(t *testing.T) {
	run := withTestFlowPass(t)
	var dump strings.Builder
	d := &Driver{FlowPipeline: run, PrintAfter: []string{flowlower.Name, testFlowPass}, PrintTo: &dump, VerifyEach: true}
	f, err := d.FlowFunc("rules", framed(), false, flowpass.Target{})
	if err != nil {
		t.Fatal(err)
	}
	if comments(f) != 1 {
		t.Errorf("the pass did not run:\n%s", code.FuncStringIn(f, flow.FuncOf(f)))
	}
	got := dump.String()
	for _, label := range []string{"-- Flow IR of r in the rules table after flowlower --", "-- Flow IR of r in the rules table after " + testFlowPass + " --"} {
		if !strings.Contains(got, label) {
			t.Errorf("dump lacks %q:\n%s", label, got)
		}
	}
	byProfile, err := Profile("no-comment")
	if err != nil {
		t.Fatal(err)
	}
	byFlag, err := pass.Settings{}.Without(testFlowPass)
	if err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]pass.Settings{"profile": byProfile, "flag": byFlag} {
		f, err := (&Driver{Opt: s, FlowPipeline: run}).FlowFunc("rules", framed(), false, flowpass.Target{})
		if err != nil {
			t.Fatal(err)
		}
		if comments(f) != 0 {
			t.Errorf("the pass ran although the %s turned it off", name)
		}
	}
	// The real pipeline prints the function after each pass that applies: for a framed function
	// of a target that has every form and no observability, every pass but terminals (local
	// functions only) and class-tables (a number, off at its default 0), and flow-finish.
	all := flowpass.Target{Terminals: true, ClassRuns: true, ClassTables: true, GuardedClasses: true, InlineLiterals: true}
	dump.Reset()
	if _, err := (&Driver{PrintAfter: []string{PrintAll}, PrintTo: &dump}).FlowFunc("typed", framed(), false, all); err != nil {
		t.Fatal(err)
	}
	var printed []string
	for _, line := range strings.Split(dump.String(), "\n") {
		if name, ok := strings.CutPrefix(line, "-- Flow IR of r in the typed table after "); ok {
			printed = append(printed, strings.TrimSuffix(name, " --"))
		}
	}
	if want := []string{flowlower.Name, flowpass.PassGuardedClasses, flowpass.PassClassRuns, flowpass.PassInlineLiterals, flowpass.FinishName}; !slices.Equal(printed, want) {
		t.Errorf("all printed %v, want %v:\n%s", printed, want, dump.String())
	}
}

// The Flow passes of a function change one copy of the module that flowlower made, which flow-finish
// and the printer go on without if a pass fails, and flow-finish finishes it in place; under
// VerifyEach every pass that runs takes a copy of its own.
func TestDriverFlowFuncCopies(t *testing.T) {
	all := flowpass.Target{Terminals: true, ClassRuns: true, ClassTables: true, GuardedClasses: true, InlineLiterals: true}
	for _, verifyEach := range []bool{false, true} {
		var copies, ran int
		p := *flowpass.Pipeline
		p.Copy = func(m *code.Module) *code.Module {
			copies++
			return m.Clone()
		}
		p.Passes = slices.Clone(p.Passes)
		for i := range p.Passes {
			run := p.Passes[i].Run
			p.Passes[i].Run = func(m *code.Module, e *flowpass.Env) (*code.Module, pass.PreservedSet, error) {
				ran++
				return run(m, e)
			}
		}
		d := &Driver{FlowPipeline: func(string) *pass.Pipeline[*code.Module, *flowpass.Env] { return &p }, VerifyEach: verifyEach}
		if _, err := d.FlowFunc("typed", framed(), false, all); err != nil {
			t.Fatal(err)
		}
		want := 1
		if verifyEach {
			want = ran
		}
		if ran < 2 || copies != want {
			t.Errorf("VerifyEach %v: %d passes ran with %d copies, want %d", verifyEach, ran, copies, want)
		}
	}
}

// The table's Core module, which the driver lowers each function of the table from, is the caller's:
// neither the lowering nor the Flow passes nor flow-finish change it, whether the passes succeed or
// one fails, so the next function of the table lowers from the same module.
func TestDriverFlowFuncKeepsTheCoreModule(t *testing.T) {
	all := flowpass.Target{Terminals: true, ClassRuns: true, ClassTables: true, GuardedClasses: true, InlineLiterals: true}
	cause := errors.New("broken")
	failing := *flowpass.Pipeline
	failing.Passes = append(slices.Clone(failing.Passes), pass.Pass[*code.Module, *flowpass.Env]{
		Name: "test-flow-fail", Obligation: "none: it fails", Run: func(m *code.Module, _ *flowpass.Env) (*code.Module, pass.PreservedSet, error) {
			flow.FuncOf(m).Body.Stmts = nil
			return m, pass.PreserveAll(), cause
		}})
	for _, c := range []struct {
		what string
		d    *Driver
		err  error
	}{
		{"passes that succeed", &Driver{}, nil},
		{"a failing pass", &Driver{FlowPipeline: func(string) *pass.Pipeline[*code.Module, *flowpass.Env] { return &failing }, TestPasses: []string{"test-flow-fail"}}, cause},
	} {
		u := framed()
		before := core.Print(u.Module, true)
		for range 2 {
			if _, err := c.d.FlowFunc("typed", u, false, all); !errors.Is(err, c.err) {
				t.Fatalf("%s: %v", c.what, err)
			}
			if after := core.Print(u.Module, true); after != before {
				t.Errorf("%s: the Core module changed:\n%s\nwas\n%s", c.what, after, before)
			}
		}
	}
}

// A failure of flowlower is returned as the failure of the step, with no module; a failure of a Flow
// pass is an internal error that names the pass, returned with the module that flow-finish ended.
func TestDriverFlowFuncErrors(t *testing.T) {
	u := framed()
	u.Func.Form = flowlower.Form(9)
	if f, err := (&Driver{}).FlowFunc("rules", u, false, flowpass.Target{}); f != nil || !isStep(err, flowlower.Name) {
		t.Errorf("a refused unit: %v, %v", f, err)
	}
	cause := errors.New("broken")
	p := *flowpass.Pipeline
	p.Passes = append(slices.Clone(p.Passes), pass.Pass[*code.Module, *flowpass.Env]{
		Name: "test-flow-fail", Obligation: "none: it fails", Run: func(m *code.Module, _ *flowpass.Env) (*code.Module, pass.PreservedSet, error) {
			return m, pass.PreserveAll(), cause
		}})
	d := &Driver{FlowPipeline: func(string) *pass.Pipeline[*code.Module, *flowpass.Env] { return &p }, TestPasses: []string{"test-flow-fail"}}
	f, err := d.FlowFunc("rules", framed(), false, flowpass.Target{})
	if f == nil || !errors.Is(err, cause) || !strings.HasPrefix(err.Error(), "internal error: pass test-flow-fail: ") {
		t.Errorf("a failing pass: %v, %v", f, err)
	}
}

// isStep reports whether err is the failure of the required step name.
func isStep(err error, name string) bool {
	var se *pass.StepError
	return errors.As(err, &se) && se.Step.Name == name
}

// testExecPass is an Exec pass of the tests: it sets the documentation of every function, which keeps
// the module valid.
const testExecPass = "test-exec-doc"

// testExecBreak is an Exec pass of the tests, off in speed, that breaks the module.
const testExecBreak = "test-exec-break"

// execModule returns the module of a function at the Flow stage, ready for execlower.
func execModule(t *testing.T) *code.Module {
	m, err := (&Driver{}).FlowFunc("rules", framed(), false, flowpass.Target{})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// noTarget is the pipeline of a target without Exec passes of its own.
var noTarget = &TargetPipeline{}

// An Exec pass runs when its flag is on and a flag turns it off; the dumps show each function of the
// module as lowered under "execlower" and after each selected pass, and a pass that breaks the module is
// named with VerifyEach.
func TestDriverExecFunc(t *testing.T) {
	p := *codepass.Pipeline
	p.Passes = append(slices.Clone(p.Passes),
		pass.Pass[*code.Module, *codepass.Env]{Name: testExecPass, Obligation: "a comment changes nothing",
			Run: func(m *code.Module, _ *codepass.Env) (*code.Module, pass.PreservedSet, error) {
				for _, f := range m.Funcs {
					f.Doc = "tested"
				}
				return m, pass.PreserveNone(), nil
			}},
		pass.Pass[*code.Module, *codepass.Env]{Name: testExecBreak, Obligation: "none: it breaks the module",
			Run: func(m *code.Module, _ *codepass.Env) (*code.Module, pass.PreservedSet, error) {
				m.Funcs = append(m.Funcs, m.Funcs[0])
				return m, pass.PreserveNone(), nil
			}})
	run := func(string) *pass.Pipeline[*code.Module, *codepass.Env] { return &p }
	var dump strings.Builder
	l, err := (&Driver{ExecPipeline: run, PrintAfter: []string{execlower.Name, testExecPass}, PrintTo: &dump}).ExecFunc("rules", execModule(t), noTarget, false)
	if err != nil {
		t.Fatal(err)
	}
	if l.Func.Doc != "tested" {
		t.Error("the pass did not run")
	}
	for _, label := range []string{"-- Exec IR of r in the rules table after execlower --", "-- Exec IR of r in the rules table after " + testExecPass + " --"} {
		if !strings.Contains(dump.String(), label) {
			t.Errorf("dump lacks %q:\n%s", label, dump.String())
		}
	}
	off, err := pass.Settings{}.Without(testExecPass)
	if err != nil {
		t.Fatal(err)
	}
	if l, err := (&Driver{Opt: off, ExecPipeline: run}).ExecFunc("rules", execModule(t), noTarget, false); err != nil || l.Func.Doc != "" {
		t.Errorf("the pass ran although the flag turned it off (%v)", err)
	}
	on, err := pass.Settings{}.With(testExecBreak, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (&Driver{Opt: on, ExecPipeline: run, VerifyEach: true}).ExecFunc("rules", execModule(t), noTarget, false)
	if err == nil || !strings.Contains(err.Error(), "after pass test-exec-break:") {
		t.Errorf("a broken module: %v", err)
	}
	// The real pipeline leaves the module alone and prints only the lowered functions.
	dump.Reset()
	if _, err := (&Driver{PrintAfter: []string{PrintAll}, PrintTo: &dump}).ExecFunc("typed", execModule(t), noTarget, true); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(dump.String(), "-- Exec IR of r in the typed table after "); n != 1+len(codepass.Pipeline.Passes) {
		t.Errorf("all printed %d sections:\n%s", n, dump.String())
	}
}
