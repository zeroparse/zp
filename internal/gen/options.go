// Package gen holds what the Go and TypeScript code generators share: the generation options, the
// runtime mode of generated Go parsers, generated files and the description of expressions in
// generated messages.
package gen

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/ir/opt"
)

// GenOptions configures code generation.
type GenOptions struct {
	// TypedOnly generates only the typed entry points (ParseAST, ParseRuleAST and their
	// siblings): no Parse, ParseRule or Recognize, and no function that builds an untyped Node.
	// It needs Types and excludes Recognize, Document and Stream.
	TypedOnly bool
	// LimitControls emits the limit-controlled form of the generated functions, which charges
	// cancellation and the compute limit (WithContext and Limits.Compute; signal and the
	// compute limit in TypeScript), and re-exports WithContext. Without it the functions have no charges (the ordinary form, or with Document
	// the form that serves a Document), and a parse with a control option fails with an error.
	LimitControls bool
	// Expectations makes the generated parser record what a syntax error expects: the expectations
	// at the farthest failure and the messages of @error. Without it the generated code keeps
	// no table of expectations, and a syntax error reports the farthest failure position with an
	// empty Expected list and no Messages.
	Expectations bool
	// Observability emits opt-in tracing and profiling for generated Go batch
	// and Document parsing, and for TypeScript batch parsing.
	Observability bool
	// Document includes the generated Go Document runner. With Types, eligible
	// outputs use private typed memoization; other outputs use Node conversion.
	Document bool
	// Stream generates ParseStream, which reads the input from an io.Reader
	// and passes the committed elements of the start rule's #stream
	// repetition to a callback. The start rule must have one.
	Stream bool
	// SourceInfo records source information in the values: the rule that produced each node and
	// where its captures are (Options.SourceInfo).
	SourceInfo bool
	// DefaultValue drops value actions and declared rule types, so the parser returns the default
	// node tree (Options.DefaultValue). It cannot be combined with Types.
	DefaultValue bool
	// Module is the import path of the generated package (required for Go output). The embedded
	// runtime is imported from its internal/runtime directory.
	Module string
	// Package is the package name of the generated code. It defaults to the last element of Module,
	// without a major version suffix (DefaultPackageName).
	Package string
	// Runtime selects how the generated Go code reaches the runtime: embedded in the package
	// (the default), imported from this module, or generated alone.
	Runtime RuntimeMode
	// RuntimeModule is the specifier from which TypeScript parsers import the runtime with RuntimeImport,
	// such as "@scope/zp-runtime" or "../shared/runtime.ts".
	RuntimeModule string
	// MaxDepth is the generated default limit on nested recursive rule calls. Zero selects 100,000.
	MaxDepth int
	// Start is the start rule of the generated Parse function, a rule marked @start. Empty
	// selects the default start rule.
	Start string
	// Types generates Go types, ParseAST for the start rule, and ParseRuleAST
	// for a named rule as values of those types (see gogen/types.go).
	Types bool
	// Recognize also generates Recognize, which checks the input without building a tree (as the
	// engine's ParseOptions.Recognize does).
	Recognize bool
	// OmitPackageDoc leaves out the package comment, for a package that documents itself in another
	// file.
	OmitPackageDoc bool
	// ConvertTypes makes typed entry points convert Node results even when the
	// typed runtime could build them (for tests).
	ConvertTypes bool
	// LeftRecursionBindings compiles with the compiler option leftRecursionBindings (for tests).
	LeftRecursionBindings bool
	// KeepNodes keeps the untyped Node rule functions of a typed-only parser. generate sets it
	// itself, when a typed entry point converts the Node result.
	KeepNodes bool
	// KeepUnreachable emits the bodies of the rules that no entry point or generated call reaches
	// (gogen/reach.go), as before they were left out, for differential tests and size comparisons.
	KeepUnreachable bool
	// KeepComments keeps the comments of unexported generated code, which tests read as route markers.
	KeepComments bool
	// DisableKeepNodes leaves the Node rule functions out of a typed-only parser that needs them
	// (a negative control for tests).
	DisableKeepNodes bool
	// Optimize selects the optimization profile (opt.Profiles); empty selects
	// speed. Opt then gives optimizations (opt.Names) a value other than the profile's: 1 turns on
	// one that the profile leaves off, 0 turns one off, and a count sets an integer one. NoOpt
	// finally turns the named optimizations off, for isolating a miscompilation, for differential
	// tests and for same-generator measurements, and wins over Opt. An optimization that does not
	// have opt.Flag.ChangesOutput changes the code and the resource use of a parser, never what it
	// returns.
	Optimize string
	Opt      map[string]int
	NoOpt    []string
	// PrintAfter lists the steps and passes (opt.PrintNames) after which the IR is written to PrintTo,
	// as text: the Core IR once for each Core module the generator builds, and the Flow IR and the
	// Exec IR once for each function it prints from them.
	PrintAfter string
	PrintTo    io.Writer
	// VerifyEachPass verifies the Core, Flow and Exec IR after every pass instead of once at the end, so that an
	// internal error names the first pass that broke an invariant. Tests and diagnosis set it; it
	// costs generation time and changes no output.
	VerifyEachPass bool
}

// Optimizations returns the settings that Optimize, Opt and NoOpt select, in that order, or an error for a
// profile or an optimization that does not exist, a value that it does not take, or a name that Opt gives twice
// (keys that are the same once their spaces are trimmed), and validates PrintAfter.
func (o GenOptions) Optimizations() (opt.Settings, error) {
	s, err := opt.Profile(o.Optimize)
	if err != nil {
		return opt.Settings{}, err
	}
	if names, err := opt.ParsePrintAfter(o.PrintAfter); err != nil {
		return opt.Settings{}, err
	} else if len(names) > 0 && o.PrintTo == nil {
		return opt.Settings{}, fmt.Errorf("PrintAfter %q needs a PrintTo writer", o.PrintAfter)
	}
	given := map[string]string{}
	for _, n := range slices.Sorted(maps.Keys(o.Opt)) {
		name := strings.TrimSpace(n)
		if prev, dup := given[name]; dup {
			return opt.Settings{}, fmt.Errorf("optimization %s is given twice in Opt (%q and %q)", name, prev, n)
		}
		given[name] = n
		if s, err = s.With(name, o.Opt[n]); err != nil {
			return opt.Settings{}, err
		}
	}
	return s.Without(o.NoOpt...)
}

// ValidateDefaultValue rejects typed output with default values: the types come from the actions and
// declarations that default values drop.
func (opts GenOptions) ValidateDefaultValue() error {
	if opts.DefaultValue && opts.Types {
		return fmt.Errorf("typed output is not available with default Values: the types come from the actions and declared types that default values drop")
	}
	return nil
}

// ValidateMaxDepth reports a negative MaxDepth and, for TypeScript output (ts), one above the
// largest safe JavaScript integer.
func (o GenOptions) ValidateMaxDepth(ts bool) error {
	if o.MaxDepth < 0 {
		return fmt.Errorf("generated max depth must be non-negative: %d", o.MaxDepth)
	}
	if ts && uint64(o.MaxDepth) > 9007199254740991 {
		return fmt.Errorf("generated max depth must be a safe JavaScript integer: %d", o.MaxDepth)
	}
	return nil
}

// RuntimeMode selects how a generated Go parser reaches the runtime.
type RuntimeMode int

const (
	// RuntimeEmbed writes a copy of the runtime into the generated package's internal/runtime
	// directory, so that the generated code depends only on the standard library.
	RuntimeEmbed RuntimeMode = iota
	// RuntimeImport imports the runtime from this module.
	RuntimeImport
	// RuntimeOnly generates only the embedded copy of the runtime, for refreshing a package
	// generated with RuntimeEmbed without regenerating its parser.
	RuntimeOnly
)

// String returns the lowercase name of the mode: embed, import or only, and RuntimeMode(n) for
// a value that names no mode.
func (m RuntimeMode) String() string {
	switch m {
	case RuntimeEmbed:
		return "embed"
	case RuntimeImport:
		return "import"
	case RuntimeOnly:
		return "only"
	}
	return fmt.Sprintf("RuntimeMode(%d)", int(m))
}

// GeneratedFile is a file of generated code. Path is relative to the directory of the generated
// package and uses slashes.
type GeneratedFile struct {
	Path    string
	Content []byte
}

// Describe returns the source form of the expression e for messages, or "" when it spans lines
// or is longer than 100 bytes.
func Describe(e grammar.Expr) string {
	s := grammar.FormatExpr(e)
	if strings.Contains(s, "\n") || len(s) > 100 {
		return ""
	}
	return s
}

// Compiler compiles a grammar for a generator. It returns the compiled program and a function
// that returns the program of its recognizer (the program in which no rule builds a value),
// which it compiles on first use.
type Compiler func(g *grammar.Grammar, opts compile.Options) (prog *compile.Program, recognizer func() (*compile.Program, error), err error)
