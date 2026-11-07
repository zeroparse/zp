package opt

import (
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/core"
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
	if _, err := Profile("memory"); err == nil || !strings.Contains(err.Error(), "valid: speed") {
		t.Errorf("memory: %v", err)
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
	if got := PrintNames(); got[0] != "build" || got[len(got)-1] != "all" || !slices.Contains(got, "class-runs") {
		t.Errorf("names %v", got)
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
