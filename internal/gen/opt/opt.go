// Package opt is the registry of the generation-time optimizations and their settings (design 063).
//
// An optimization is a flag with a name. A pass of the Core IR pipeline is one (its name is the
// pass's); so are the options of Flow lowering and the generation controls of the runtime
// structures that the printers emit. A Settings value says which are on, and an integer parameter
// where there is one. A profile is a named Settings. Whatever the settings, a generated parser
// returns the same results, errors and events; only its code and its resource use change.
package opt

import (
	"fmt"
	"slices"
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
	// Runtime is a control of the runtime structures that the printers emit.
	Runtime
)

func (k Kind) String() string {
	switch k {
	case Pass:
		return "pass"
	case Lowering:
		return "lowering"
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
}

// Flags lists the optimizations in the order the reference documents them.
var Flags = []Flag{
	{Name: core.PassTerminals, Kind: Pass, Default: 1, Doc: "value-free choices of many literals and classes run as one program (lean typed Go)"},
	{Name: core.PassTermRewrites, Kind: Pass, Default: 1, Doc: "rewrite terms of actions and predicates into forms that need no intermediate values"},
	{Name: core.FlagTextDispatch, Kind: Pass, Default: 1, Doc: "a predicate that compares the text of a capture with literals becomes one switch"},
	{Name: core.FlagFieldCompare, Kind: Pass, Default: 1, Doc: "compare fields without boxing them"},
	{Name: core.PassGuards, Kind: Pass, Default: 1, Doc: "skip an alternative when the next character cannot start it"},
	{Name: core.FlagGuardedClasses, Kind: Pass, Default: 1, Doc: "a leading class of a guarded alternative consumes the character its guard read (ordinary Go parsers)"},
	{Name: core.PassScans, Kind: Pass, Default: 1, Doc: "a value-free repetition of a class or of any character runs as a scan"},
	{Name: core.PassClassRuns, Kind: Pass, Default: 1, Doc: "consecutive classes of a value-free sequence run as one (lean typed Go)"},
	{Name: "class-tables", Kind: Lowering, Default: 1, Doc: "match a class of many ranges through a membership table (lean typed Go)"},
	{Name: "inline-literals", Kind: Lowering, Int: true, Default: 4, Doc: "compare literals of at most this many code points in place first; 0 turns it off"},
	{Name: "typed-object-slabs", Kind: Runtime, Default: 1, Doc: "allocate typed objects in slabs (typed Go)"},
	{Name: "typed-list-slabs", Kind: Runtime, Default: 1, Doc: "allocate typed lists in slabs (typed Go)"},
	{Name: "typed-output-bootstrap", Kind: Runtime, Default: 1, Doc: "size the first slab of the typed output from the input (typed Go)"},
	{Name: "sparse-memo", Kind: Runtime, Default: 1, Doc: "keep the first-call table of large TypeScript parsers sparse"},
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

// Without returns the settings with the named optimizations off. An unknown name is an error that
// lists the valid ones.
func (s Settings) Without(names ...string) (Settings, error) {
	out := Settings{over: map[string]int{}}
	for k, v := range s.over {
		out.over[k] = v
	}
	for _, n := range names {
		if _, ok := lookup(n); !ok {
			return Settings{}, fmt.Errorf("unknown optimization %q (valid: %s)", n, strings.Join(Names(), ", "))
		}
		out.over[n] = 0
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
