package opt

import (
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/flow"
)

// Every pass of the Core pipeline is an optimization that can be turned off, under its own name.
func TestEveryCorePassIsAFlag(t *testing.T) {
	for _, name := range core.Pipeline.Names() {
		if f, ok := lookup(name); !ok || f.Kind != Pass {
			t.Errorf("pass %s is not a registered optimization of kind pass", name)
		}
	}
	for _, name := range []string{"text-dispatch", "field-compare", core.FlagGuardedClasses} {
		if _, ok := lookup(name); !ok {
			t.Errorf("the flag %s is not registered", name)
		}
	}
}

// Every pass of the Flow pipeline is an optimization that can be turned off, under its own name, and
// no Flow pass has the name of a Core pass.
func TestEveryFlowPassIsAFlag(t *testing.T) {
	for _, name := range flow.Pipeline.Names() {
		if f, ok := lookup(name); !ok || f.Kind != FlowPass {
			t.Errorf("pass %s is not a registered optimization of kind flow pass", name)
		}
		if slices.Contains(core.Pipeline.Names(), name) {
			t.Errorf("pass %s is both a Core and a Flow pass", name)
		}
	}
	if err := flow.Pipeline.Validate(); err != nil {
		t.Error(err)
	}
}

// Every pass of the Exec pipeline is an optimization that can be turned off, under its own name, and
// no Exec pass has the name of a Core or Flow pass.
func TestEveryExecPassIsAFlag(t *testing.T) {
	for _, name := range exec.Pipeline.Names() {
		if f, ok := lookup(name); !ok || f.Kind != ExecPass {
			t.Errorf("pass %s is not a registered optimization of kind exec pass", name)
		}
		if slices.Contains(core.Pipeline.Names(), name) || slices.Contains(flow.Pipeline.Names(), name) {
			t.Errorf("pass %s is also a Core or Flow pass", name)
		}
	}
	if err := exec.Pipeline.Validate(); err != nil {
		t.Error(err)
	}
}

func TestRegistryIsValid(t *testing.T) {
	if err := check(Flags); err != nil {
		t.Error(err)
	}
	for _, c := range []struct {
		name  string
		flags []Flag
		want  string
	}{
		{"repeated", []Flag{{Name: "a", Doc: "x"}, {Name: "a", Doc: "y"}}, "twice"},
		{"undocumented", []Flag{{Name: "a"}}, "no description"},
		{"on and off flag with a count", []Flag{{Name: "a", Doc: "x", Default: 2}}, "speed value is 2"},
		{"output change that is on", []Flag{{Name: "a", Doc: "x", Default: 1, ChangesOutput: true}}, "must be an opt-in"},
		{"integer output change that is on", []Flag{{Name: "a", Doc: "x", Int: true, Default: 4, ChangesOutput: true}}, "must be an opt-in"},
		{"name of a step", []Flag{{Name: PrintFlow, Kind: FlowPass, Doc: "x"}}, "name of a step"},
		{"name of the Exec step", []Flag{{Name: PrintExec, Kind: ExecPass, Doc: "x"}}, "name of a step"},
	} {
		if err := check(c.flags); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error with %q", c.name, err, c.want)
		}
	}
	if err := check([]Flag{{Name: "a", Doc: "x", ChangesOutput: true}, {Name: "b", Doc: "x", Int: true, Default: 4}}); err != nil {
		t.Errorf("a valid list: %v", err)
	}
}

func TestFlagNamesAreDistinctAndDocumented(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Flags {
		if seen[f.Name] || f.Doc == "" {
			t.Errorf("flag %q is declared twice or has no description", f.Name)
		}
		seen[f.Name] = true
		if !f.Int && f.Default != 0 && f.Default != 1 {
			t.Errorf("flag %s is on or off but its speed value is %d", f.Name, f.Default)
		}
	}
}

func TestSettings(t *testing.T) {
	speed, err := Profile("")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range Flags {
		if speed.Value(f.Name) != f.Default {
			t.Errorf("%s is %d in speed, want %d", f.Name, speed.Value(f.Name), f.Default)
		}
	}
	if s, err := Profile("speed"); err != nil || s.Value("inline-literals") != 4 {
		t.Errorf("speed: %v, %v", s, err)
	}
	if _, err := Profile("nosuch"); err == nil || !strings.Contains(err.Error(), "valid: speed, memory, size") {
		t.Errorf("unknown profile: %v", err)
	}
	off, err := speed.Without("guards", "inline-literals")
	if err != nil {
		t.Fatal(err)
	}
	if sp, err := speed.Without(" guards", "scans "); err != nil || sp.On("guards") || sp.On("scans") {
		t.Errorf("Without does not ignore spaces around a name: %v", err)
	}
	if off.On("guards") || off.Value("inline-literals") != 0 || !off.On("scans") || !speed.On("guards") {
		t.Error("Without changes the wrong flags, or its receiver")
	}
	if _, err := speed.Without("guards", "nonsense"); err == nil || !strings.Contains(err.Error(), `"nonsense"`) || !strings.Contains(err.Error(), "valid: terminals") {
		t.Errorf("unknown name: %v", err)
	}
}

func TestParsePrintAfter(t *testing.T) {
	if got, err := ParsePrintAfter(""); err != nil || got != nil {
		t.Errorf("empty: %v, %v", got, err)
	}
	if got, err := ParsePrintAfter("build, guards,all"); err != nil || !slices.Equal(got, []string{"build", "guards", "all"}) {
		t.Errorf("list: %v, %v", got, err)
	}
	if _, err := ParsePrintAfter("guards,nonsense"); err == nil || !strings.Contains(err.Error(), "unknown pass") {
		t.Errorf("unknown: %v", err)
	}
	// The names follow the order the steps run in: the Core build and passes, then the Flow lowering
	// and passes, then the Exec lowering and passes.
	want := append([]string{"build"}, core.Pipeline.Names()...)
	want = append(append(want, "flow"), flow.Pipeline.Names()...)
	want = append(append(want, "exec"), exec.Pipeline.Names()...)
	want = append(want, "all")
	if got := PrintNames(); !slices.Equal(got, want) {
		t.Errorf("names %v, want %v", got, want)
	}
	if got, err := ParsePrintAfter("flow"); err != nil || !slices.Equal(got, []string{"flow"}) {
		t.Errorf("flow: %v, %v", got, err)
	}
}

func TestWith(t *testing.T) {
	speed, _ := Profile("")
	on, err := speed.With("ts-literal-sharing", 1)
	if err != nil || !on.On("ts-literal-sharing") || speed.On("ts-literal-sharing") {
		t.Fatalf("opt-in: %v, speed %v", err, speed.On("ts-literal-sharing"))
	}
	if s, err := speed.With("class-tables", 1); err != nil || s.Value("class-tables") != 1 {
		t.Errorf("count: %v, %v", s, err)
	}
	for _, bad := range []struct {
		name string
		v    int
	}{{"nonsense", 1}, {"guards", 2}, {"guards", -1}, {"class-tables", -1}} {
		if _, err := speed.With(bad.name, bad.v); err == nil {
			t.Errorf("With(%q, %d) is accepted", bad.name, bad.v)
		}
	}
	off, err := on.Without("ts-literal-sharing")
	if err != nil || off.On("ts-literal-sharing") {
		t.Errorf("Without after With: %v", err)
	}
}

// A profile is a preset of flags that keep behavior: it never sets one that changes the output.
func TestProfilesSetNoOutputFlag(t *testing.T) {
	for _, name := range Profiles() {
		s, err := Profile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range Flags {
			if f.ChangesOutput && s.Value(f.Name) != f.Default {
				t.Errorf("profile %s sets %s, which changes the output", name, f.Name)
			}
		}
	}
}

func TestParseValues(t *testing.T) {
	got, err := ParseValues(" ts-literal-sharing , class-tables=3,inline-literals=0 ")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"ts-literal-sharing": 1, "class-tables": 3, "inline-literals": 0}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %d, want %d", k, got[k], v)
		}
	}
	if got, err := ParseValues("  "); got != nil || err != nil {
		t.Errorf("an empty list is %v, %v", got, err)
	}
	for list, want := range map[string]string{
		"a,,b":                        "empty optimization name",
		"=3":                          "empty optimization name",
		"class-tables=x":              `"x" is not a count`,
		"class-tables=,scans":         `"" is not a count`,
		"class-tables=3,class-tables": "twice",
	} {
		if _, err := ParseValues(list); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ParseValues(%q) = %v, want an error with %q", list, err, want)
		}
	}
}

func TestProfileDefinitionsAreValid(t *testing.T) {
	if err := checkProfiles(profileDefs, Flags); err != nil {
		t.Error(err)
	}
	for _, c := range []struct {
		name string
		defs []profileDef
		want string
	}{
		{"undocumented", []profileDef{{Name: "a"}}, "no description"},
		{"repeated", []profileDef{{Name: "a", Doc: "x"}, {Name: "a", Doc: "y"}}, "twice"},
		{"unknown flag", []profileDef{{Name: "a", Doc: "x", Values: map[string]int{"nosuch": 1}}}, "not a flag"},
		{"bad value", []profileDef{{Name: "a", Doc: "x", Values: map[string]int{"guards": 2}}}, "value 2"},
	} {
		if err := checkProfiles(c.defs, Flags); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error with %q", c.name, err, c.want)
		}
	}
	if err := checkProfiles([]profileDef{{Name: "a", Doc: "x", Values: map[string]int{"o": 1}}}, []Flag{{Name: "o", Doc: "x", ChangesOutput: true}}); err == nil || !strings.Contains(err.Error(), "changes the output") {
		t.Errorf("a profile that sets an output flag: %v", err)
	}
}

func TestProfileSettings(t *testing.T) {
	if got := Profiles(); !slices.Equal(got, []string{"speed", "memory", "size"}) {
		t.Fatalf("profiles %v", got)
	}
	mem, err := Profile("memory")
	if err != nil || mem.On("typed-object-slabs") || mem.On("typed-list-slabs") || !mem.On("typed-output-bootstrap") || !mem.On("guards") {
		t.Errorf("memory: %v", err)
	}
	size, err := Profile("size")
	if err != nil || size.Value("inline-literals") != 0 || size.Value("class-tables") != 5 || !size.On("guards") {
		t.Errorf("size: %v", err)
	}
	for _, name := range Profiles() {
		if ProfileDoc(name) == "" {
			t.Errorf("profile %s has no description", name)
		}
	}
	if ProfileDoc("nosuch") != "" || ProfileValues("nosuch") != nil {
		t.Errorf("an unknown profile has a description or values")
	}
	if v := ProfileValues("size"); v["class-tables"] != 5 || len(v) != 2 {
		t.Errorf("size values %v", v)
	}
}

// A registry with an optimization that changes the output, which no flag of the real one does yet,
// shows the checks on their positive path: it is valid as an opt-in, and neither a nonzero default nor
// a profile that sets it is.
func TestOutputChangingFlagRegistry(t *testing.T) {
	fake := Flag{Name: "fake-output", Kind: Printer, Doc: "changes the generated API", ChangesOutput: true}
	flags := append(slices.Clone(Flags), fake)
	if err := check(flags); err != nil {
		t.Errorf("an opt-in that changes the output: %v", err)
	}
	on := fake
	on.Default = 1
	if err := check(append(slices.Clone(Flags), on)); err == nil || !strings.Contains(err.Error(), "must be an opt-in") {
		t.Errorf("a nonzero default for a flag that changes the output: %v, want the opt-in error", err)
	}
	intOn := Flag{Name: "fake-count", Kind: Lowering, Int: true, Doc: "changes the generated API", Default: 3, ChangesOutput: true}
	if err := check([]Flag{intOn}); err == nil || !strings.Contains(err.Error(), "must be an opt-in") {
		t.Errorf("a nonzero count default for a flag that changes the output: %v, want the opt-in error", err)
	}
	for _, v := range []int{0, 1} {
		defs := []profileDef{{Name: "p", Doc: "x", Values: map[string]int{"fake-output": v}}}
		if err := checkProfiles(defs, flags); err == nil || !strings.Contains(err.Error(), "changes the output") {
			t.Errorf("a profile that gives the flag %d: %v, want an error", v, err)
		}
	}
	if err := checkProfiles([]profileDef{{Name: "p", Doc: "x", Values: map[string]int{"guards": 0}}}, flags); err != nil {
		t.Errorf("a profile of flags that keep behavior, in a registry that also has the flag: %v", err)
	}
	if got := fake.Summary(); got != "fake-output (printer, off; changes output)" {
		t.Errorf("Summary = %q", got)
	}
}

func TestFlagSummaries(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"class-tables", "class-tables (lowering, off; count)"},
		{"inline-literals", "inline-literals (lowering, 4; count)"},
		{"guards", "guards (pass, on)"},
		{"ts-literal-sharing", "ts-literal-sharing (printer, off)"},
		{"sparse-memo", "sparse-memo (runtime, on)"},
	} {
		f, ok := lookup(c.name)
		if !ok || f.Summary() != c.want {
			t.Errorf("%s: Summary = %q, want %q", c.name, f.Summary(), c.want)
		}
	}
	for _, f := range Flags {
		if !strings.Contains(Summaries(), f.Summary()) {
			t.Errorf("Summaries lacks %s", f.Name)
		}
	}
}
