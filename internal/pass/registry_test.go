package pass

import (
	"slices"
	"strings"
	"testing"
)

// The registry of this test binary holds only these flags: the package links no part that registers
// its own. They are registered out of order, across two calls, as parts initialized one after
// another register theirs.
func init() {
	Register(
		Flag{Name: "t-sharing", Kind: KindPrinter, Default: 1, Doc: "shares"},
		Flag{Name: "t-guards", Kind: KindCorePass, Default: 1, Doc: "guards"},
		Flag{Name: "t-opt-in", Kind: KindPrinter, Doc: "an opt-in"},
		Flag{Name: "t-slabs", Kind: KindRuntime, Default: 1, Doc: "slabs"},
	)
	Register(
		Flag{Name: "t-exec", Kind: KindExecPass, Default: 1, Doc: "an exec pass"},
		Flag{Name: "t-count", Kind: KindLowering, Int: true, Default: 4, Doc: "a count"},
		Flag{Name: "t-flow", Kind: KindFlowPass, Default: 1, Doc: "a flow pass"},
		Flag{Name: "t-ablation", Kind: KindCorePass, Default: 1, Doc: "a pass named before guards"},
	)
}

// The registry lists the flags by kind, in the order pass, lowering, flow pass, exec pass, printer,
// runtime, and by name within a kind, whatever order the parts registered them in: the usage text
// and the errors that list the names are the same in every build.
func TestRegistryOrder(t *testing.T) {
	want := []string{"t-ablation", "t-guards", "t-count", "t-flow", "t-exec", "t-opt-in", "t-sharing", "t-slabs"}
	if got := Names(); !slices.Equal(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
	var kinds []Kind
	for _, f := range Flags() {
		kinds = append(kinds, f.Kind)
	}
	if !slices.IsSorted(kinds) {
		t.Errorf("kinds out of order: %v", kinds)
	}
	if got := []string{KindCorePass.String(), KindLowering.String(), KindFlowPass.String(), KindExecPass.String(), KindPrinter.String(), KindRuntime.String()}; !slices.Equal(got, []string{"pass", "lowering", "flow pass", "exec pass", "printer", "runtime"}) {
		t.Errorf("kind names %v", got)
	}
	// Sorting is a function of the set alone.
	a := Flags()
	b := slices.Clone(a)
	slices.Reverse(b)
	SortFlags(b)
	if !slices.Equal(a, b) {
		t.Errorf("sorting a reversed list gives %v", b)
	}
	if got := Summaries(); !strings.HasPrefix(got, "t-ablation (pass, on), t-guards (pass, on), t-count (lowering, 4; count), ") {
		t.Errorf("Summaries() = %q", got)
	}
}

func TestCheckFlags(t *testing.T) {
	if err := CheckFlags(Flags()); err != nil {
		t.Error(err)
	}
	for _, c := range []struct {
		name  string
		flags []Flag
		want  string
	}{
		{"repeated", []Flag{{Name: "a", Doc: "x"}, {Name: "a", Doc: "y"}}, "twice"},
		{"undocumented", []Flag{{Name: "a"}}, "no description"},
		{"unnamed", []Flag{{Doc: "x"}}, "no name"},
		{"unknown kind", []Flag{{Name: "a", Doc: "x", Kind: KindRuntime + 1}}, "unknown kind(6)"},
		{"on and off flag with a count", []Flag{{Name: "a", Doc: "x", Default: 2}}, "speed value is 2"},
		{"output change that is on", []Flag{{Name: "a", Doc: "x", Default: 1, ChangesOutput: true}}, "must be an opt-in"},
		{"integer output change that is on", []Flag{{Name: "a", Doc: "x", Int: true, Default: 4, ChangesOutput: true}}, "must be an opt-in"},
	} {
		if err := CheckFlags(c.flags); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error with %q", c.name, err, c.want)
		}
	}
	if err := CheckFlags([]Flag{{Name: "a", Doc: "x", ChangesOutput: true}, {Name: "b", Doc: "x", Int: true, Default: 4}}); err != nil {
		t.Errorf("a valid list: %v", err)
	}
}

// Register refuses a flag that breaks a rule of the registry, among them one that a part registered
// already, and leaves the registry as it was.
func TestRegisterPanics(t *testing.T) {
	before := Names()
	for _, f := range []Flag{{Name: "t-guards", Doc: "again"}, {Name: "t-new"}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Register(%v) does not panic", f)
				}
			}()
			Register(f)
		}()
	}
	if got := Names(); !slices.Equal(got, before) {
		t.Errorf("a refused registration changed the registry: %v", got)
	}
}

func TestSettings(t *testing.T) {
	var speed Settings
	for _, f := range Flags() {
		if speed.Value(f.Name) != f.Default {
			t.Errorf("%s is %d in speed, want %d", f.Name, speed.Value(f.Name), f.Default)
		}
	}
	off, err := speed.Without("t-guards", "t-count")
	if err != nil {
		t.Fatal(err)
	}
	if sp, err := speed.Without(" t-guards", "t-flow "); err != nil || sp.On("t-guards") || sp.On("t-flow") {
		t.Errorf("Without does not ignore spaces around a name: %v", err)
	}
	if off.On("t-guards") || off.Value("t-count") != 0 || !off.On("t-flow") || !speed.On("t-guards") {
		t.Error("Without changes the wrong flags, or its receiver")
	}
	if _, err := speed.Without("t-guards", "nonsense"); err == nil || !strings.Contains(err.Error(), `"nonsense"`) || !strings.Contains(err.Error(), "valid: t-ablation, t-guards, t-count") {
		t.Errorf("unknown name: %v", err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("On of an unregistered name does not panic")
			}
		}()
		speed.On("nonsense")
	}()
}

func TestWith(t *testing.T) {
	var speed Settings
	on, err := speed.With("t-opt-in", 1)
	if err != nil || !on.On("t-opt-in") || speed.On("t-opt-in") {
		t.Fatalf("opt-in: %v, speed %v", err, speed.On("t-opt-in"))
	}
	if s, err := speed.With("t-count", 1); err != nil || s.Value("t-count") != 1 {
		t.Errorf("count: %v, %v", s, err)
	}
	for _, bad := range []struct {
		name string
		v    int
	}{{"nonsense", 1}, {"t-guards", 2}, {"t-guards", -1}, {"t-count", -1}} {
		if _, err := speed.With(bad.name, bad.v); err == nil {
			t.Errorf("With(%q, %d) is accepted", bad.name, bad.v)
		}
	}
	off, err := on.Without("t-opt-in")
	if err != nil || off.On("t-opt-in") || !on.On("t-opt-in") {
		t.Errorf("Without after With: %v", err)
	}
}

func TestFlagSummaries(t *testing.T) {
	for _, c := range []struct {
		f    Flag
		want string
	}{
		{Flag{Name: "class-tables", Kind: KindLowering, Int: true}, "class-tables (lowering, off; count)"},
		{Flag{Name: "inline-literals", Kind: KindLowering, Int: true, Default: 4}, "inline-literals (lowering, 4; count)"},
		{Flag{Name: "guards", Kind: KindCorePass, Default: 1}, "guards (pass, on)"},
		{Flag{Name: "ts-literal-sharing", Kind: KindPrinter}, "ts-literal-sharing (printer, off)"},
		{Flag{Name: "sparse-memo", Kind: KindRuntime, Default: 1}, "sparse-memo (runtime, on)"},
		{Flag{Name: "f", Kind: KindFlowPass, Default: 1}, "f (flow pass, on)"},
		{Flag{Name: "fake-output", Kind: KindPrinter, ChangesOutput: true}, "fake-output (printer, off; changes output)"},
	} {
		if got := c.f.Summary(); got != c.want {
			t.Errorf("Summary = %q, want %q", got, c.want)
		}
	}
	for _, f := range Flags() {
		if !strings.Contains(Summaries(), f.Summary()) {
			t.Errorf("Summaries lacks %s", f.Name)
		}
	}
}
