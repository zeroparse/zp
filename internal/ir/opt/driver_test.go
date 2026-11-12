package opt

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/flow"
	"zeroparse.dev/zp/internal/ir/pipeline"
)

// The driver runs the Core passes that the settings leave on, dumps the module after the selected
// names under the label of its table, calls the hook after every pass, and asks for the
// verification after each pass when VerifyEach is set.
func TestDriverRunCore(t *testing.T) {
	off, err := Settings{}.Without(core.PassTerminals)
	if err != nil {
		t.Fatal(err)
	}
	var dump strings.Builder
	var hooked []string
	var verifyEach bool
	d := &Driver{Opt: off, PrintAfter: []string{"build", core.PassGuards}, PrintTo: &dump, VerifyEach: true,
		CoreHook: func(pass string, _ *core.Module) { hooked = append(hooked, pass) }}
	var ran []string
	d.CoreRun = func(m *core.Module, env *core.Env, o core.RunOptions) ([]string, error) {
		verifyEach = o.VerifyEach
		var err error
		ran, err = core.Run(m, env, o)
		return ran, err
	}
	lean := core.Mode{Lean: true, Ordinary: true}
	if err := d.RunCore("typed", &core.Module{}, lean, func(*core.Rule) bool { return true }, nil); err != nil {
		t.Fatal(err)
	}
	want := []string{core.PassGuards, core.PassScans, core.PassClassRuns}
	if !slices.Equal(ran, want) {
		t.Errorf("ran %v, want %v", ran, want)
	}
	if !slices.Equal(hooked, append([]string{"build"}, want...)) {
		t.Errorf("hook saw %v", hooked)
	}
	if !verifyEach {
		t.Error("VerifyEach did not reach the pipeline")
	}
	got := dump.String()
	for _, label := range []string{"-- Core IR of the typed table after build --", "-- Core IR of the typed table after guards --"} {
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
	if err := d.RunCore("rules", &core.Module{}, core.Mode{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	d.PrintTo = &dump
	if err := d.RunCore("rules", &core.Module{}, core.Mode{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(dump.String(), "-- Core IR of the rules table after "); n != 3 {
		t.Errorf("all printed %d sections, want build, guards and scans:\n%s", n, dump.String())
	}
}

// A failing pass is reported as an internal error that keeps the cause.
func TestDriverRunCoreError(t *testing.T) {
	cause := errors.New("broken")
	d := &Driver{CoreRun: func(*core.Module, *core.Env, core.RunOptions) ([]string, error) { return nil, cause }}
	err := d.RunCore("typed", &core.Module{}, core.Mode{}, nil, nil)
	if !errors.Is(err, cause) || !strings.HasPrefix(err.Error(), "internal error: ") {
		t.Fatalf("error %v", err)
	}
}

// testFlowPass is a Flow pass of the tests: it adds a comment to the function, which keeps it valid.
const testFlowPass = "test-flow-comment"

// withTestFlowPass registers testFlowPass, on in speed and off in a profile of its own, for the
// duration of the test, and returns a FlowRun that runs a pipeline with it.
func withTestFlowPass(t *testing.T) func(string, *flow.Func, *flow.Env, flow.RunOptions) ([]string, error) {
	flags, defs := Flags, profileDefs
	Flags = append(slices.Clone(Flags), Flag{Name: testFlowPass, Kind: FlowPass, Default: 1, Doc: "adds a comment"})
	profileDefs = append(slices.Clone(profileDefs), profileDef{Name: "no-comment", Doc: "x", Values: map[string]int{testFlowPass: 0}})
	t.Cleanup(func() { Flags, profileDefs = flags, defs })
	p := *flow.Pipeline
	p.Passes = append(slices.Clone(p.Passes), pipeline.Pass[*flow.Func, *flow.Env]{
		Name: testFlowPass, Obligation: "a comment changes nothing",
		Run: func(f *flow.Func, _ *flow.Env) error {
			f.Body = append(f.Body, &flow.Comment{Text: "tested"})
			return nil
		}})
	return func(_ string, f *flow.Func, env *flow.Env, o flow.RunOptions) ([]string, error) {
		return p.Run(f, env, o.Options(env))
	}
}

func framed() *flow.Func {
	return &flow.Func{Name: "r", Frame: true, Fail: &flow.Label{Name: "fail"}}
}

// A Flow pass runs when its flag is on, and a profile or a flag turns it off; the dumps show the
// function as lowered under "flow" and after each selected pass, labeled with its table.
func TestDriverRunFlow(t *testing.T) {
	run := withTestFlowPass(t)
	var dump strings.Builder
	d := &Driver{FlowRun: run, PrintAfter: []string{PrintFlow, testFlowPass}, PrintTo: &dump, VerifyEach: true}
	f := framed()
	if err := d.RunFlow("rules", f, false); err != nil {
		t.Fatal(err)
	}
	if len(f.Body) != 1 {
		t.Errorf("the pass did not run: %d statements", len(f.Body))
	}
	got := dump.String()
	for _, label := range []string{"-- Flow IR of r in the rules table after flow --", "-- Flow IR of r in the rules table after " + testFlowPass + " --"} {
		if !strings.Contains(got, label) {
			t.Errorf("dump lacks %q:\n%s", label, got)
		}
	}
	byProfile, err := Profile("no-comment")
	if err != nil {
		t.Fatal(err)
	}
	byFlag, err := Settings{}.Without(testFlowPass)
	if err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]Settings{"profile": byProfile, "flag": byFlag} {
		f := framed()
		if err := (&Driver{Opt: s, FlowRun: run}).RunFlow("rules", f, false); err != nil {
			t.Fatal(err)
		}
		if len(f.Body) != 0 {
			t.Errorf("the pass ran although the %s turned it off", name)
		}
	}
	// The real pipeline leaves a function alone and prints only the lowered function.
	dump.Reset()
	f = framed()
	if err := (&Driver{PrintAfter: []string{PrintAll}, PrintTo: &dump}).RunFlow("typed", f, true); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(dump.String(), "-- Flow IR of r in the typed table after "); n != 1+len(flow.Pipeline.Passes) {
		t.Errorf("all printed %d sections:\n%s", n, dump.String())
	}
}
