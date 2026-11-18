package pass

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The registry of the optimizations (design 063). An optimization is a flag with a name. A pass of
// the Core, Flow or Exec pipeline is one (its name is the pass's); so are the options of the
// lowerings, what the printers emit, and the controls of the runtime structures that they emit. Each
// part of the pipeline registers its own flags with Register when it is initialized, so the registry
// of a program holds the flags of the parts it links. A Settings value says which are on, and an
// integer parameter where there is one. A flag keeps the results, errors and events of a generated
// parser and changes only its code and its resource use, unless it is marked ChangesOutput.

// Kind says what an optimization changes. The order of the kinds is the order in which the registry
// lists them.
type Kind int

const (
	// KindCorePass is a pass over Core IR, or a variant of one.
	KindCorePass Kind = iota
	// KindLowering is an option of a lowering.
	KindLowering
	// KindFlowPass is a pass over Flow IR.
	KindFlowPass
	// KindExecPass is a pass over Exec IR.
	KindExecPass
	// KindPrinter is a choice of what the Go and TypeScript printers emit: the form of a term of an
	// action or a predicate, or whether equal generated bodies and tables share one helper.
	KindPrinter
	// KindRuntime is a control of the runtime structures that the printers emit.
	KindRuntime
)

func (k Kind) String() string {
	switch k {
	case KindCorePass:
		return "pass"
	case KindLowering:
		return "lowering"
	case KindFlowPass:
		return "flow pass"
	case KindExecPass:
		return "exec pass"
	case KindPrinter:
		return "printer"
	case KindRuntime:
		return "runtime"
	}
	return "kind(" + strconv.Itoa(int(k)) + ")"
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

// DefaultText is the default value as the guide and the usage text write it: "on" or "off" for a
// flag, "off" or the number for a flag with a count.
func (f Flag) DefaultText() string {
	switch {
	case f.Int && f.Default != 0:
		return strconv.Itoa(f.Default)
	case f.Default != 0:
		return "on"
	}
	return "off"
}

// Summary is the name of the flag with its kind and default, as `class-tables (lowering, off; count)`:
// a flag with a count says so, and one that changes the output says that.
func (f Flag) Summary() string {
	s := f.Name + " (" + f.Kind.String() + ", " + f.DefaultText()
	if f.Int {
		s += "; count"
	}
	if f.ChangesOutput {
		s += "; changes output"
	}
	return s + ")"
}

// CheckFlags reports a flag list that breaks the rules of the registry: a name or a description
// missing or repeated, a kind that is not one of the kinds, a value that is neither 0 or 1 nor a
// count, and an optimization that changes the output without being an opt-in.
func CheckFlags(flags []Flag) error {
	seen := map[string]bool{}
	for _, f := range flags {
		switch {
		case f.Name == "" || f.Doc == "":
			return fmt.Errorf("flag %q has no name or no description", f.Name)
		case seen[f.Name]:
			return fmt.Errorf("flag %s is declared twice", f.Name)
		case f.Kind < KindCorePass || f.Kind > KindRuntime:
			return fmt.Errorf("flag %s has the unknown %s", f.Name, f.Kind)
		case f.Default < 0 || !f.Int && f.Default > 1:
			return fmt.Errorf("flag %s is on or off but its speed value is %d", f.Name, f.Default)
		case f.ChangesOutput && f.Default != 0:
			return fmt.Errorf("flag %s changes the output but is on in the speed profile: it must be an opt-in", f.Name)
		}
		seen[f.Name] = true
	}
	return nil
}

// registry holds the registered flags in the order Flags lists them.
var registry []Flag

// Register adds flags to the registry. The parts of the pipeline call it from their init functions,
// before any Settings value is read. It panics if the registry with the flags breaks a rule of
// CheckFlags, which is a mistake in the part that registers them.
func Register(flags ...Flag) {
	next := append(slices.Clone(registry), flags...)
	if err := CheckFlags(next); err != nil {
		panic("pass: " + err.Error())
	}
	SortFlags(next)
	registry = next
}

// SortFlags sorts flags in the order the registry lists them: by kind, in the order of the kinds,
// and by name within a kind.
func SortFlags(flags []Flag) {
	slices.SortFunc(flags, func(a, b Flag) int {
		if c := cmp.Compare(a.Kind, b.Kind); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
}

// Flags returns the registered flags, by kind and by name within a kind.
func Flags() []Flag { return slices.Clone(registry) }

// Lookup returns the registered flag of the name.
func Lookup(name string) (Flag, bool) {
	for _, f := range registry {
		if f.Name == name {
			return f, true
		}
	}
	return Flag{}, false
}

// Names returns the names of the registered flags in the order of Flags.
func Names() []string {
	out := make([]string, len(registry))
	for i, f := range registry {
		out[i] = f.Name
	}
	return out
}

// Summaries returns the Summary of every registered flag in the order of Flags, joined by ", ".
func Summaries() string {
	out := make([]string, len(registry))
	for i, f := range registry {
		out[i] = f.Summary()
	}
	return strings.Join(out, ", ")
}

// Settings says which optimizations are on. The zero value is the speed profile: every flag has its
// Default.
type Settings struct {
	// over holds the values that differ from the defaults.
	over map[string]int
}

// On reports whether the optimization is on. It panics for a name that is not registered, which is a
// mistake in the part that reads it and not in a request.
func (s Settings) On(name string) bool { return s.Value(name) != 0 }

// Value returns the value of the optimization: 0 or 1, or the parameter of an integer flag.
func (s Settings) Value(name string) int {
	f, ok := Lookup(name)
	if !ok {
		panic(fmt.Sprintf("pass: unknown optimization %q", name))
	}
	if v, ok := s.over[name]; ok {
		return v
	}
	return f.Default
}

// With returns the settings with the optimization set to the value: 0 or 1, or the parameter of an
// integer flag. A request to the generator turns on an optimization that the speed profile leaves off
// with it (-opt), and a preset sets the values of its flags with it. An unknown name, or a value
// that the flag does not take, is an error.
func (s Settings) With(name string, value int) (Settings, error) {
	f, ok := Lookup(name)
	if !ok {
		return Settings{}, fmt.Errorf("unknown optimization %q (valid: %s)", name, strings.Join(Names(), ", "))
	}
	if value < 0 || !f.Int && value > 1 {
		return Settings{}, fmt.Errorf("optimization %s does not take the value %d", name, value)
	}
	out := s.clone()
	out.over[name] = value
	return out, nil
}

// Without returns the settings with the named optimizations off; spaces around a name are ignored. An
// unknown name is an error that lists the valid ones.
func (s Settings) Without(names ...string) (Settings, error) {
	out := s.clone()
	for _, n := range names {
		n = strings.TrimSpace(n)
		if _, ok := Lookup(n); !ok {
			return Settings{}, fmt.Errorf("unknown optimization %q (valid: %s)", n, strings.Join(Names(), ", "))
		}
		out.over[n] = 0
	}
	return out, nil
}

func (s Settings) clone() Settings {
	out := Settings{over: make(map[string]int, len(s.over)+1)}
	for k, v := range s.over {
		out.over[k] = v
	}
	return out
}
