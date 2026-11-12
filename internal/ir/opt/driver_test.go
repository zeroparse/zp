package opt

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/core"
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
