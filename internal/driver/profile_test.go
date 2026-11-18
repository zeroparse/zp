package driver

import (
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/flow"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/pass"
)

// The flags of the passes that the tests of the driver add to the pipelines. The registry of this
// test binary holds them and the flags of the parts the driver links (the Core passes), and not
// those of the generators.
func init() {
	pass.Register(
		pass.Flag{Name: testFlowPass, Kind: pass.KindFlowPass, Default: 1, Doc: "adds a comment"},
		pass.Flag{Name: testExecPass, Kind: pass.KindExecPass, Default: 1, Doc: "documents"},
		pass.Flag{Name: testExecBreak, Kind: pass.KindExecPass, Doc: "breaks"},
	)
}

// Every pass of the Core pipeline is an optimization that can be turned off, under its own name.
func TestEveryCorePassIsAFlag(t *testing.T) {
	for _, name := range append(corepass.Pipeline.Names(), corepass.FlagGuardedClasses) {
		if f, ok := pass.Lookup(name); !ok || f.Kind != pass.KindCorePass {
			t.Errorf("pass %s is not a registered optimization of kind pass", name)
		}
	}
}

// Every pass of the Flow pipeline is an optimization that can be turned off, under its own name, and
// no Flow pass has the name of a Core pass.
func TestEveryFlowPassIsAFlag(t *testing.T) {
	for _, name := range flow.Pipeline.Names() {
		if f, ok := pass.Lookup(name); !ok || f.Kind != pass.KindFlowPass {
			t.Errorf("pass %s is not a registered optimization of kind flow pass", name)
		}
		if slices.Contains(corepass.Pipeline.Names(), name) {
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
	for _, name := range codepass.Pipeline.Names() {
		if f, ok := pass.Lookup(name); !ok || f.Kind != pass.KindExecPass {
			t.Errorf("pass %s is not a registered optimization of kind exec pass", name)
		}
		if slices.Contains(corepass.Pipeline.Names(), name) || slices.Contains(flow.Pipeline.Names(), name) {
			t.Errorf("pass %s is also a Core or Flow pass", name)
		}
	}
	if err := codepass.Pipeline.Validate(); err != nil {
		t.Error(err)
	}
}

func TestCheckStepNames(t *testing.T) {
	if err := checkStepNames(pass.Flags()); err != nil {
		t.Error(err)
	}
	for _, name := range []string{PrintCore, PrintFlow, PrintExec, PrintAll} {
		if err := checkStepNames([]pass.Flag{{Name: name, Kind: pass.KindFlowPass, Doc: "x"}}); err == nil || !strings.Contains(err.Error(), "name of a step") {
			t.Errorf("a flag named %s: %v", name, err)
		}
	}
}

// The speed profile is the defaults of the registered flags. A profile that sets a flag that no
// linked part registered is an error that names the flag: the memory and size profiles set flags of
// the generators, which this test binary does not link, and they must not quietly become speed.
func TestProfile(t *testing.T) {
	for _, name := range []string{"", "speed"} {
		speed, err := Profile(name)
		if err != nil {
			t.Fatalf("%q: %v", name, err)
		}
		for _, f := range pass.Flags() {
			if speed.Value(f.Name) != f.Default {
				t.Errorf("%s is %d in speed, want %d", f.Name, speed.Value(f.Name), f.Default)
			}
		}
	}
	if _, err := Profile("nosuch"); err == nil || !strings.Contains(err.Error(), "valid: speed, memory, size") {
		t.Errorf("unknown profile: %v", err)
	}
	for name, flag := range map[string]string{"memory": "typed-list-slabs", "size": "class-tables"} {
		s, err := Profile(name)
		if err == nil {
			t.Errorf("profile %s resolves without the flags it sets: %v", name, s)
			continue
		}
		for _, want := range []string{"profile " + name, `"` + flag + `"`, "not a registered flag", "not linked"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("profile %s: %v, want an error with %q", name, err, want)
			}
		}
	}
	if err := Check(); err == nil || !strings.Contains(err.Error(), "not a registered flag") {
		t.Errorf("Check without the generators: %v", err)
	}
}

func TestParsePrintAfter(t *testing.T) {
	if got, err := ParsePrintAfter(""); err != nil || got != nil {
		t.Errorf("empty: %v, %v", got, err)
	}
	if got, err := ParsePrintAfter("core, guards,all"); err != nil || !slices.Equal(got, []string{"core", "guards", "all"}) {
		t.Errorf("list: %v, %v", got, err)
	}
	if _, err := ParsePrintAfter("guards,nonsense"); err == nil || !strings.Contains(err.Error(), "unknown pass") {
		t.Errorf("unknown: %v", err)
	}
	// The names follow the order the steps run in: the Core IR as built and the Core passes, then the Flow lowering
	// and passes, then the Exec lowering and passes.
	want := append([]string{"core"}, corepass.Pipeline.Names()...)
	want = append(append(want, "flow"), flow.Pipeline.Names()...)
	want = append(append(want, "exec"), codepass.Pipeline.Names()...)
	want = append(want, "all")
	if got := PrintNames(); !slices.Equal(got, want) {
		t.Errorf("names %v, want %v", got, want)
	}
	if got, err := ParsePrintAfter("flow"); err != nil || !slices.Equal(got, []string{"flow"}) {
		t.Errorf("flow: %v, %v", got, err)
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

// testFlags is a registry with the flags that the profiles set, for the checks of the profiles
// apart from the parts that register them.
func testFlags() []pass.Flag {
	return append(pass.Flags(),
		pass.Flag{Name: "typed-object-slabs", Kind: pass.KindRuntime, Default: 1, Doc: "x"},
		pass.Flag{Name: "typed-list-slabs", Kind: pass.KindRuntime, Default: 1, Doc: "x"},
		pass.Flag{Name: "inline-literals", Kind: pass.KindLowering, Int: true, Default: 4, Doc: "x"},
		pass.Flag{Name: "class-tables", Kind: pass.KindLowering, Int: true, Doc: "x"},
	)
}

func TestProfileDefinitionsAreValid(t *testing.T) {
	flags := testFlags()
	if err := checkProfiles(profileDefs, flags); err != nil {
		t.Error(err)
	}
	for _, c := range []struct {
		name string
		defs []profileDef
		want string
	}{
		{"undocumented", []profileDef{{Name: "a"}}, "no description"},
		{"repeated", []profileDef{{Name: "a", Doc: "x"}, {Name: "a", Doc: "y"}}, "twice"},
		{"unknown flag", []profileDef{{Name: "a", Doc: "x", Values: map[string]int{"nosuch": 1}}}, "not a registered flag"},
		{"bad value", []profileDef{{Name: "a", Doc: "x", Values: map[string]int{"guards": 2}}}, "value 2"},
	} {
		if err := checkProfiles(c.defs, flags); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error with %q", c.name, err, c.want)
		}
	}
	if err := checkProfiles([]profileDef{{Name: "a", Doc: "x", Values: map[string]int{"o": 1}}}, []pass.Flag{{Name: "o", Doc: "x", ChangesOutput: true}}); err == nil || !strings.Contains(err.Error(), "changes the output") {
		t.Errorf("a profile that sets an output flag: %v", err)
	}
	if got := Profiles(); !slices.Equal(got, []string{"speed", "memory", "size"}) {
		t.Fatalf("profiles %v", got)
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
	fake := pass.Flag{Name: "fake-output", Kind: pass.KindPrinter, Doc: "changes the generated API", ChangesOutput: true}
	flags := append(testFlags(), fake)
	if err := pass.CheckFlags(flags); err != nil {
		t.Errorf("an opt-in that changes the output: %v", err)
	}
	on := fake
	on.Default = 1
	if err := pass.CheckFlags(append(testFlags(), on)); err == nil || !strings.Contains(err.Error(), "must be an opt-in") {
		t.Errorf("a nonzero default for a flag that changes the output: %v, want the opt-in error", err)
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
}
