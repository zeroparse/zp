// Package opt is the registry of the optimizations and their settings (design 063).
//
// An optimization is a flag with a name. A pass of the Core IR pipeline is one (its name is the
// pass's); so are the options of Flow lowering, what the printers emit, and the
// controls of the runtime structures that they emit. A Settings value says which are on, and an
// integer parameter where there is one. A profile is a preset: a name for fixed values of flags,
// with no logic of its own. A flag keeps the results, errors and events of a generated parser and
// changes only its code and its resource use, unless it is marked ChangesOutput.
package opt

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"zeroparse.dev/zp/internal/ir/core"
)

// Kind says what an optimization changes.
type Kind int

const (
	// Pass is a pass over Core IR, or a variant of one (internal/ir/core, Pipeline).
	Pass Kind = iota
	// Lowering is an option of Flow lowering.
	Lowering
	// Printer is a choice of what the Go and TypeScript printers emit: the form of a term of an
	// action or a predicate, or whether equal generated bodies and tables share one helper.
	Printer
	// Runtime is a control of the runtime structures that the printers emit.
	Runtime
)

func (k Kind) String() string {
	switch k {
	case Pass:
		return "pass"
	case Lowering:
		return "lowering"
	case Printer:
		return "printer"
	}
	return "runtime"
}

// A Flag is one optimization.
type Flag struct {
	Name string
	Kind Kind
	// Doc is one line that says what the optimization does.
	Doc string
	// Int reports that the flag has an integer parameter; 0 turns it off. Otherwise the value is 0 or 1.
	Int bool
	// Default is the value in the speed profile.
	Default int
	// ChangesOutput reports that turning the optimization on changes the generated API or the
	// representation of what a parser returns, not only its code and resource use. Such a flag is an
	// opt-in (Default 0), no preset sets it, and it has a contract test of its own in place of the
	// disabled-form tests of the flags that keep behavior.
	ChangesOutput bool
}

// Flags lists the optimizations in the order the reference documents them.
var Flags = []Flag{
	{Name: core.PassTerminals, Kind: Pass, Default: 1, Doc: "value-free choices of many literals and classes run as one program (lean typed Go)"},
	{Name: "text-dispatch", Kind: Printer, Default: 1, Doc: "the printers emit a predicate that compares the text of a capture with literals as one switch"},
	{Name: "field-compare", Kind: Printer, Default: 1, Doc: "the Go printer compares fields in the actions and predicates of typed rules without boxing string fields"},
	{Name: core.PassGuards, Kind: Pass, Default: 1, Doc: "skip an alternative when the next character cannot start it"},
	{Name: core.FlagGuardedClasses, Kind: Pass, Default: 1, Doc: "a leading class of a guarded alternative consumes the character its guard read (ordinary Go parsers)"},
	{Name: core.PassScans, Kind: Pass, Default: 1, Doc: "a value-free repetition of a class or of any character runs as a scan"},
	{Name: core.PassClassRuns, Kind: Pass, Default: 1, Doc: "consecutive classes of a value-free sequence run as one (lean typed Go)"},
	{Name: "class-tables", Kind: Lowering, Int: true, Default: 0, Doc: "match a class of at least this many ranges through a membership table (lean typed Go); smaller source against a slower parse, so an opt-in: 0 in speed, 5 is the measured choice"},
	{Name: "inline-literals", Kind: Lowering, Int: true, Default: 4, Doc: "compare literals of at most this many code points in place first; 0 turns it off"},
	{Name: "method-sharing", Kind: Printer, Default: 1, Doc: "emit one helper for generated bodies that are exactly equal (Go and TypeScript)"},
	{Name: "literal-sharing", Kind: Printer, Default: 1, Doc: "emit one table for equal literals (Go; TypeScript also needs ts-literal-sharing)"},
	{Name: "ts-literal-sharing", Kind: Printer, Default: 0, Doc: "share literal tables in TypeScript parsers; off because a small cold parse measured about 1.5 times slower (design 090)"},
	{Name: "typed-object-slabs", Kind: Runtime, Default: 1, Doc: "allocate typed objects in slabs (typed Go)"},
	{Name: "typed-list-slabs", Kind: Runtime, Default: 1, Doc: "allocate typed lists in slabs (typed Go)"},
	{Name: "typed-output-bootstrap", Kind: Runtime, Default: 1, Doc: "size the first slab of the typed output from the input (typed Go)"},
	{Name: "sparse-memo", Kind: Runtime, Default: 1, Doc: "keep the first-call table of large TypeScript parsers sparse"},
}

// check reports a flag list that breaks the rules of the registry: a name or a description missing or
// repeated, a value that is neither 0 or 1 nor a count, and an optimization that changes the output
// without being an opt-in.
func check(flags []Flag) error {
	seen := map[string]bool{}
	for _, f := range flags {
		switch {
		case f.Name == "" || f.Doc == "":
			return fmt.Errorf("flag %q has no name or no description", f.Name)
		case seen[f.Name]:
			return fmt.Errorf("flag %s is declared twice", f.Name)
		case f.Default < 0 || !f.Int && f.Default > 1:
			return fmt.Errorf("flag %s is on or off but its speed value is %d", f.Name, f.Default)
		case f.ChangesOutput && f.Default != 0:
			return fmt.Errorf("flag %s changes the output but is on in the speed profile: it must be an opt-in", f.Name)
		}
		seen[f.Name] = true
	}
	return nil
}

// Names returns the names of the flags in order.
func Names() []string {
	out := make([]string, len(Flags))
	for i, f := range Flags {
		out[i] = f.Name
	}
	return out
}

func lookup(name string) (Flag, bool) {
	for _, f := range Flags {
		if f.Name == name {
			return f, true
		}
	}
	return Flag{}, false
}

// Settings says which optimizations are on. The zero value is the speed profile.
type Settings struct {
	// over holds the values that differ from the defaults.
	over map[string]int
}

// On reports whether the optimization is on. It panics for a name that is not registered, which is a
// mistake in the generator and not in a request.
func (s Settings) On(name string) bool { return s.Value(name) != 0 }

// Value returns the value of the optimization: 0 or 1, or the parameter of an integer flag.
func (s Settings) Value(name string) int {
	f, ok := lookup(name)
	if !ok {
		panic(fmt.Sprintf("opt: unknown optimization %q", name))
	}
	if v, ok := s.over[name]; ok {
		return v
	}
	return f.Default
}

// With returns the settings with the optimization set to the value: 0 or 1, or the parameter of an
// integer flag. A request to the generator turns on an optimization that the speed profile leaves off
// with it (GenOptions.Opt, -opt), and a preset sets the values of its flags with it. An unknown name,
// or a value that the flag does not take, is an error.
func (s Settings) With(name string, value int) (Settings, error) {
	f, ok := lookup(name)
	if !ok {
		return Settings{}, fmt.Errorf("unknown optimization %q (valid: %s)", name, strings.Join(Names(), ", "))
	}
	if value < 0 || !f.Int && value > 1 {
		return Settings{}, fmt.Errorf("optimization %s does not take the value %d", name, value)
	}
	out := Settings{over: map[string]int{}}
	for k, v := range s.over {
		out.over[k] = v
	}
	out.over[name] = value
	return out, nil
}

// Without returns the settings with the named optimizations off; spaces around a name are ignored. An
// unknown name is an error that lists the valid ones.
func (s Settings) Without(names ...string) (Settings, error) {
	out := Settings{over: map[string]int{}}
	for k, v := range s.over {
		out.over[k] = v
	}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if _, ok := lookup(n); !ok {
			return Settings{}, fmt.Errorf("unknown optimization %q (valid: %s)", n, strings.Join(Names(), ", "))
		}
		out.over[n] = 0
	}
	return out, nil
}

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

// Profile returns the settings of the named profile; the empty name is the default, speed.
func Profile(name string) (Settings, error) {
	switch name {
	case "", "speed":
		return Settings{}, nil
	}
	return Settings{}, fmt.Errorf("unknown optimization profile %q (valid: %s)", name, strings.Join(Profiles(), ", "))
}

// Profiles returns the names of the profiles.
func Profiles() []string { return []string{"speed"} }

// PrintNames returns the names that select the module to print after: "build", the passes, and
// "all".
func PrintNames() []string {
	return append([]string{"build"}, append(corePassNames(), "all")...)
}

func corePassNames() []string { return core.Pipeline.Names() }

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
