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
	for _, name := range []string{core.FlagTextDispatch, core.FlagFieldCompare, core.FlagGuardedClasses} {
		if _, ok := lookup(name); !ok {
			t.Errorf("the variant flag %s is not registered", name)
		}
	}
}

func TestFlagNamesAreDistinctAndDocumented(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Flags {
		if seen[f.Name] || f.Doc == "" {
			t.Errorf("flag %q is declared twice or has no description", f.Name)
		}
		seen[f.Name] = true
		if !f.Int && f.Default != 1 {
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
