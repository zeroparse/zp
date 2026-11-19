// Package driver runs the compiler pipeline under one optimization setting (design 063): it holds the
// optimization profiles, parses the values that a request gives the optimizations, names the steps
// that -print-after selects, and drives the Core, Flow and Exec pass pipelines with their dumps and
// verification. Every backend that lowers from the IR runs its pipelines through a Driver, so a
// profile or a flag means the same passes whatever the backend.
//
// The optimizations themselves are flags of the registry in internal/pass, which each part of the
// pipeline fills with its own. A profile is a preset: a name for fixed values of flags, with no logic
// of its own.
package driver

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"zeroparse.dev/zp/internal/middleend/flowpass"

	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/pass"
)

// ParseValues parses a comma-separated list of name or name=count items, as -opt takes it, into the
// values by name: a name alone is 1. Spaces around an item are ignored. A repeated name, a count
// that is not a decimal number or an empty item is an error; whether a name exists, and whether it
// takes the value, is for Settings.With.
func ParseValues(list string) (map[string]int, error) {
	if strings.TrimSpace(list) == "" {
		return nil, nil
	}
	out := map[string]int{}
	for _, item := range strings.Split(list, ",") {
		name, num, hasValue := strings.Cut(strings.TrimSpace(item), "=")
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("empty optimization name in %q", list)
		}
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("optimization %s is given twice", name)
		}
		v := 1
		if hasValue {
			n, err := strconv.Atoi(strings.TrimSpace(num))
			if err != nil {
				return nil, fmt.Errorf("optimization %s: %q is not a count", name, strings.TrimSpace(num))
			}
			v = n
		}
		out[name] = v
	}
	return out, nil
}

// A profileDef is a preset: the values that differ from the speed values of the flags, and what the
// preset is for. It has no logic of its own.
type profileDef struct {
	Name   string
	Doc    string
	Values map[string]int
}

// profileDefs lists the profiles; the first is the default. The values were chosen by the rule of
// record 136 (docs/optimizations/136-measured-effect-of-the-generation-flags.md): a profile is kept
// if it improves the metric it is named for by at least 1% beyond the noise of an identical copy, on
// at least six of the ten language grammars, costs at most 3% of parse time as a geometric mean, and
// slows no grammar by more than 6%.
var profileDefs = []profileDef{
	{Name: "speed", Doc: "the fastest parse that does not cost generated code or memory out of proportion; the default"},
	{Name: "memory", Doc: "typed values are allocated one by one: fewer bytes allocated and less live heap, with many more allocations",
		Values: map[string]int{"typed-object-slabs": 0, "typed-list-slabs": 0}},
	{Name: "size", Doc: "smaller generated Go source and binaries, for a little parse time",
		Values: map[string]int{"inline-literals": 0, "class-tables": 5}},
}

// checkProfiles reports a profile that breaks the rules: a repeated or undocumented one, a value of
// a flag that flags does not hold or one the flag does not take, and a value for a flag that changes
// the output.
func checkProfiles(defs []profileDef, flags []pass.Flag) error {
	seen := map[string]bool{}
	for _, d := range defs {
		if d.Name == "" || d.Doc == "" || seen[d.Name] {
			return fmt.Errorf("profile %q has no name or no description, or is declared twice", d.Name)
		}
		seen[d.Name] = true
		for _, name := range slices.Sorted(maps.Keys(d.Values)) {
			v := d.Values[name]
			i := slices.IndexFunc(flags, func(f pass.Flag) bool { return f.Name == name })
			switch {
			case i < 0:
				return fmt.Errorf("profile %s sets %q, which is not a registered flag: the part of the pipeline that registers it is not linked", d.Name, name)
			case flags[i].ChangesOutput:
				return fmt.Errorf("profile %s sets %s, which changes the output", d.Name, name)
			case v < 0 || !flags[i].Int && v > 1:
				return fmt.Errorf("profile %s gives %s the value %d", d.Name, name, v)
			}
		}
	}
	return nil
}

// Check reports a registry that breaks the rules that only the driver knows: a flag with the name of
// a step that -print-after selects, and a profile that is not valid over the registered flags. A
// program checks it once all the parts it links have registered their flags.
func Check() error {
	flags := pass.Flags()
	if err := checkStepNames(flags); err != nil {
		return err
	}
	return checkProfiles(profileDefs, flags)
}

// checkStepNames reports a flag that has the name of a step that -print-after selects, which would
// select both.
func checkStepNames(flags []pass.Flag) error {
	for _, f := range flags {
		if f.Name == PrintCore || f.Name == PrintFlow || f.Name == PrintExec || f.Name == PrintAll {
			return fmt.Errorf("flag %s has the name of a step that -print-after selects", f.Name)
		}
	}
	return nil
}

// Profile returns the settings of the named profile; the empty name is the default, speed. A profile
// that sets a flag that no linked part registers is an error that names the flag, never settings
// without it.
func Profile(name string) (pass.Settings, error) {
	if name == "" {
		name = profileDefs[0].Name
	}
	for _, d := range profileDefs {
		if d.Name != name {
			continue
		}
		if err := checkProfiles([]profileDef{d}, pass.Flags()); err != nil {
			return pass.Settings{}, err
		}
		s := pass.Settings{}
		for _, flag := range slices.Sorted(maps.Keys(d.Values)) {
			var err error
			if s, err = s.With(flag, d.Values[flag]); err != nil {
				return pass.Settings{}, err
			}
		}
		return s, nil
	}
	return pass.Settings{}, fmt.Errorf("unknown optimization profile %q (valid: %s)", name, strings.Join(Profiles(), ", "))
}

// Profiles returns the names of the profiles; the first is the default.
func Profiles() []string {
	out := make([]string, len(profileDefs))
	for i, d := range profileDefs {
		out[i] = d.Name
	}
	return out
}

// ProfileDoc returns the one line that says what the profile is for, or "" for an unknown name.
func ProfileDoc(name string) string {
	for _, d := range profileDefs {
		if d.Name == name {
			return d.Doc
		}
	}
	return ""
}

// ProfileValues returns the flags that the profile sets to other than their speed values, by name.
func ProfileValues(name string) map[string]int {
	for _, d := range profileDefs {
		if d.Name == name {
			return maps.Clone(d.Values)
		}
	}
	return nil
}

// The names of the steps that build a unit of the IR, which select its dump as built.
const (
	// PrintCore is the Core IR as the front end built it.
	PrintCore = corepass.Stage
	// PrintFlow is a function of Flow IR as the lowering made it.
	PrintFlow = flow.Stage
	// PrintExec is a function of Exec IR as the lowering from Flow IR made it.
	PrintExec = codepass.Stage
	// PrintAll selects every step and every pass.
	PrintAll = "all"
)

// PrintNames returns the names that select what to print after, in the order the steps run: "core",
// the Core passes, "flow", the Flow passes, "exec", the Exec passes and those of the registered
// targets (golang/pass, typescript/pass), and "all".
func PrintNames() []string {
	out := []string{PrintCore}
	out = append(out, corepass.Pipeline.Names()...)
	out = append(out, PrintFlow)
	out = append(out, flowpass.Pipeline.Names()...)
	out = append(out, PrintExec)
	out = append(out, codepass.Pipeline.Names()...)
	for _, p := range targets {
		out = append(out, p.Names()...)
	}
	return append(out, PrintAll)
}

// ParsePrintAfter validates a comma-separated list of pass names for printing, and returns the names
// in order. The empty string selects nothing.
func ParsePrintAfter(list string) ([]string, error) {
	if list == "" {
		return nil, nil
	}
	valid := PrintNames()
	var out []string
	for _, n := range strings.Split(list, ",") {
		n = strings.TrimSpace(n)
		if !slices.Contains(valid, n) {
			return nil, fmt.Errorf("unknown pass %q to print after (valid: %s)", n, strings.Join(valid, ", "))
		}
		out = append(out, n)
	}
	return out, nil
}
