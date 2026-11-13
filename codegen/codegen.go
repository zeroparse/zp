// Package codegen generates the source code of parsers from ZeroParse grammars: Go packages and
// TypeScript modules. A generated parser parses without the grammar and without the engine; it
// needs only the library of generated parsers, which it imports or carries a copy of (LibMode),
// and, in Go, the package core that the library depends on.
//
// Go and TypeScript are configurations: a value holds the settings of one generation, and its
// Generate method generates a parser for a grammar. Both implement Generator.
package codegen

import (
	"embed"
	"fmt"
	"io"
	"io/fs"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegen/go"
	"zeroparse.dev/zp/internal/codegen/ts"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/desugar"
)

// Generator generates the source code of parsers in one language.
type Generator interface {
	// Generate generates the files of a parser for g whose entry points parse from the rule
	// start, which must be a start rule (a rule marked @start); an empty start selects the
	// default start rule.
	Generate(g *grammar.Grammar, start string) ([]File, error)
	// GenerateLib generates only the library, which does not depend on a grammar.
	GenerateLib() ([]File, error)
}

var (
	_ Generator = Go{}
	_ Generator = TypeScript{}
)

// File is a file of generated code. Path is relative to the directory of the generated package
// and uses slashes.
type File struct {
	Path    string
	Content []byte
}

// files converts the files that a generator returns.
func files(generated []codegen.GeneratedFile, err error) ([]File, error) {
	if err != nil {
		return nil, err
	}
	out := make([]File, len(generated))
	for i, f := range generated {
		out[i] = File{Path: f.Path, Content: f.Content}
	}
	return out, nil
}

// LibMode selects how generated code reaches the library of generated parsers.
type LibMode int

// The library modes.
const (
	// LibEmbed writes a copy of the library next to the parser: the package internal/libgo in
	// Go, which depends only on zeroparse.dev/zp/core, and lib.ts in TypeScript, which depends on
	// nothing. It is the default. GenerateLib refreshes the copy without regenerating the parser.
	LibEmbed LibMode = iota
	// LibImport imports the library: zeroparse.dev/zp/codegen/go/lib in Go, the module of
	// TypeScript.LibModule in TypeScript.
	LibImport
)

// String returns the lowercase name of the mode, embed or import, and LibMode(n) for a value
// that names no mode.
func (m LibMode) String() string {
	switch m {
	case LibEmbed:
		return "embed"
	case LibImport:
		return "import"
	}
	return fmt.Sprintf("LibMode(%d)", int(m))
}

// internal returns the mode of the generators for m, or an error for a value that names no mode.
func (m LibMode) internal() (codegen.LibMode, error) {
	switch m {
	case LibEmbed:
		return codegen.LibEmbed, nil
	case LibImport:
		return codegen.LibImport, nil
	}
	return 0, fmt.Errorf("unknown library mode %v", m)
}

// Go configures the generation of a Go parser. The zero value generates an untyped parser with an
// embedded library; Module must be set for it.
type Go struct {
	// Module is the import path of the generated package. It is required, except with
	// LibImport and a PackageName and without Types.
	Module string
	// PackageName is the package name of the generated code. It defaults to the last element of
	// Module, without a major version suffix (example.com/m/v2 gives m).
	PackageName string
	// Lib selects how the generated code reaches the library (LibEmbed by default).
	Lib LibMode
	// Types generates declarations for the grammar's types and typed results: ParseValue for the
	// start rule and ParseRuleValue for named rules. The types are declared in the package
	// Module/types, so Module is required.
	Types bool
	// DefaultValue makes the generated parser produce default values, as engine.WithDefaultValue
	// does for a compiled parser. It cannot be combined with Types.
	DefaultValue bool
	// SourceInfo makes the generated parser record source information, as engine.WithSourceInfo
	// does for a compiled parser.
	SourceInfo bool
	// Expectations makes the generated parser record what a syntax error expects: the expectations
	// at the farthest failure and the messages of @error, as engine.WithExpectations does for a
	// compiled parser. Without it the parser keeps no table of expectations, and a syntax error
	// reports the farthest failure position with an empty Expected list and no Messages.
	Expectations bool
	// Observability emits rule tracing and profiling for batch parsers and Documents, including
	// their typed entry points. Collection is selected per invocation.
	Observability bool
	// LimitControls generates the limit-controlled form of the parser, which supports cancellation
	// and the compute limit and re-exports the option WithContext. Without it the generated parser
	// is smaller and faster to build, and a parse that passes a context or a finite Compute limit
	// returns an error; Limits.Input and Limits.Depth work in both forms. The controlled form is the only form of the parser: a
	// parse that sets no limit runs it too, and is slower than with a parser generated without
	// limit controls.
	LimitControls bool
	// Recognize also generates a function Recognize, which checks the input against the start
	// rule without building a tree, like a parse with engine.RecognizeOnly.
	Recognize bool
	// MaxDepth is the generated parser's default nested rule-call limit. Zero selects 100,000;
	// negative values fail generation. Individual invocations may override it with Limits.Depth.
	MaxDepth int
	// Document generates a mutable incremental Document. With Types, it also generates
	// Document.ParseValue with direct incremental matching for eligible grammars and Node
	// conversion otherwise.
	Document bool
	// Stream generates ParseStream, which reads the input from an io.Reader and passes each
	// committed element of the start rule's #stream repetition to a callback, as
	// engine.Parser.ParseStream does. The start rule must have a #stream repetition. With Types it
	// also generates ParseStreamValue and IterateValue, which pass the typed value of each element.
	Stream bool
	// TypedOnly generates only the typed entry points of a parser: ParseValue and its siblings, and
	// no Parse or ParseRule. Recognize may be added. The generated package is smaller and builds
	// faster. It needs Types and cannot be combined with Document or Stream, which build untyped
	// Nodes.
	TypedOnly bool
	// RecognizeOnly generates only Recognize, which checks the input against the start rule: no
	// Parse, ParseRule or typed entry point. The generated package is the smallest. It implies
	// Recognize and cannot be combined with Types, TypedOnly, Document or Stream.
	RecognizeOnly bool
	// OmitPackageDoc leaves the package comment out of the generated parser, for a package that
	// has its own documentation in another file.
	OmitPackageDoc bool
	// Optimize selects the optimization profile: "speed", the default, "memory" or "size". Opt
	// then gives optimizations a value other than the profile's, as -opt of zp gen does: 1 turns
	// on one that the profile leaves off, and a count sets an integer one.
	// NoOpt finally turns the named optimizations off, to isolate a miscompilation or to measure
	// one optimization, as -no-opt does, and wins over Opt. The names are those of the guide.
	// None of them changes what a generated parser returns, only its code. An unknown name, or a
	// value that the optimization does not take, is an error.
	Optimize string
	Opt      map[string]int
	NoOpt    []string
	// PrintAfter lists, separated by commas, the steps and passes after which the intermediate
	// representation of the generator is written to PrintTo as text, for debugging the generator:
	// build and the names of the Core passes for the Core IR, flow and the names of the Flow passes
	// for the Flow IR, exec and the names of the Exec passes for the Exec IR, or all. It needs
	// PrintTo.
	PrintAfter string
	PrintTo    io.Writer
}

// Generate generates the files of a Go parser for g in the package with import path c.Module:
// its Parse(input) parses from the rule start (see Generator). The files are parser.go, with the
// code generated from the grammar, api.go, which re-exports the library names the parser's users
// need, with Types types/types.go, the package Module/types that declares the Go types of the
// grammar, and, with LibEmbed, the files of a copy of the library in internal/libgo.
func (c Go) Generate(g *grammar.Grammar, start string) ([]File, error) {
	lib, err := c.Lib.internal()
	if err != nil {
		return nil, err
	}
	return files(gogen.Generate(g, compileForGenerator, codegen.GenOptions{
		Module:         c.Module,
		Package:        c.PackageName,
		Start:          start,
		Lib:            lib,
		GoLib:          goLib,
		Types:          c.Types,
		DefaultValue:   c.DefaultValue,
		SourceInfo:     c.SourceInfo,
		Observability:  c.Observability,
		LimitControls:  c.LimitControls,
		Expectations:   c.Expectations,
		Recognize:      c.Recognize,
		MaxDepth:       c.MaxDepth,
		Document:       c.Document,
		Stream:         c.Stream,
		TypedOnly:      c.TypedOnly,
		RecognizeOnly:  c.RecognizeOnly,
		OmitPackageDoc: c.OmitPackageDoc,
		Optimize:       c.Optimize,
		Opt:            c.Opt,
		NoOpt:          c.NoOpt,
		PrintAfter:     c.PrintAfter,
		PrintTo:        c.PrintTo,
	}))
}

// GenerateLib generates only the embedded copy of the library for the package with import
// path c.Module, to refresh a parser generated with LibEmbed without regenerating it. The
// other settings are not used.
func (c Go) GenerateLib() ([]File, error) {
	return files(gogen.Generate(nil, compileForGenerator, codegen.GenOptions{Module: c.Module, Lib: codegen.LibOnly, GoLib: goLib}))
}

// TypeScript configures the generation of a TypeScript parser. The zero value generates an
// untyped parser with an embedded library.
type TypeScript struct {
	// Lib selects how the generated code reaches the library (LibEmbed by default).
	// LibImport needs LibModule.
	Lib LibMode
	// LibModule is the module specifier from which a parser generated with LibImport
	// imports its library, such as "@scope/zp-lib" or "../shared/lib.ts". The library
	// module is the lib.ts of GenerateLib (or of any parser generated with LibEmbed).
	// It is an error with another mode.
	LibModule string
	// Types generates declarations for the grammar's types and the entry points parseValue and
	// parseValueWithOptions, which convert the completed Node result into plain objects and arrays.
	// Declared aliases preserve the target value type.
	Types bool
	// DefaultValue makes the generated parser produce default values, as engine.WithDefaultValue
	// does for a compiled parser. It cannot be combined with Types.
	DefaultValue bool
	// SourceInfo makes the generated parser record source information, as engine.WithSourceInfo
	// does for a compiled parser.
	SourceInfo bool
	// Expectations makes the generated parser record what a syntax error expects: the expectations
	// at the farthest failure and the messages of @error, as engine.WithExpectations does for a
	// compiled parser. Without it the parser keeps no table of expectations, and a syntax error
	// reports the farthest failure position with an empty expected list and no messages.
	Expectations bool
	// Observability emits rule tracing and profiling for batch parsing, including the typed entry
	// points. Collection is selected per invocation.
	Observability bool
	// LimitControls generates the limit-controlled form of the parser, which supports the signal
	// option and the compute limit. Without it the generated parser is smaller, and a parse that
	// passes a signal or a finite compute limit returns an error; the input and depth limits work in
	// both forms. The controlled form is the only form of the
	// parser: a parse that sets no limit runs it too, and is slower than with a parser generated
	// without limit controls.
	LimitControls bool
	// Recognize also generates recognize, which checks the input against the start rule without
	// building a tree, like a parse with engine.RecognizeOnly.
	Recognize bool
	// Document also generates createDocument, which makes a mutable Document of the start rule:
	// edit changes its text, and parse returns what parse of the whole current text returns, as
	// the Go Document does. With Types, the Document also has parseValue.
	Document bool
	// MaxDepth is the generated parser's default nested rule-call limit. Zero selects 100,000;
	// negative values and values that are not exact JavaScript integers fail generation.
	// Individual invocations may override it with limits.depth.
	MaxDepth int
	// Optimize selects the optimization profile: "speed", the default, "memory" or "size". Opt
	// then gives optimizations a value other than the profile's, as -opt of zp gen does: 1 turns
	// on one that the profile leaves off, and a count sets an integer one.
	// NoOpt finally turns the named optimizations off, to isolate a miscompilation or to measure
	// one optimization, as -no-opt does, and wins over Opt. The names are those of the guide.
	// None of them changes what a generated parser returns, only its code. An unknown name, or a
	// value that the optimization does not take, is an error.
	Optimize string
	Opt      map[string]int
	NoOpt    []string
	// PrintAfter lists, separated by commas, the steps and passes after which the intermediate
	// representation of the generator is written to PrintTo as text, for debugging the generator:
	// build and the names of the Core passes for the Core IR, flow and the names of the Flow passes
	// for the Flow IR, exec and the names of the Exec passes for the Exec IR, or all. It needs
	// PrintTo.
	PrintAfter string
	PrintTo    io.Writer
}

// Generate generates the files of a TypeScript parser for g: its parse(input) parses from the
// rule start (see Generator) and returns the same trees and errors as a engine.Parser. The files
// are
//
//   - lib.ts, the library, which does not depend on the grammar (with LibEmbed),
//   - parser.ts, the code generated from the grammar, which imports ./lib.ts, or the
//     LibModule with LibImport, and
//   - types.ts with Types, the declarations of the grammar's types, which parser.ts imports.
//
// Generated TypeScript parsers have no stream parsing.
func (c TypeScript) Generate(g *grammar.Grammar, start string) ([]File, error) {
	lib, err := c.Lib.internal()
	if err != nil {
		return nil, err
	}
	return files(tsgen.Generate(g, compileForGenerator, codegen.GenOptions{
		Start:         start,
		Lib:           lib,
		LibModule:     c.LibModule,
		TSLib:         tsLib,
		Types:         c.Types,
		DefaultValue:  c.DefaultValue,
		SourceInfo:    c.SourceInfo,
		Observability: c.Observability,
		LimitControls: c.LimitControls,
		Expectations:  c.Expectations,
		Recognize:     c.Recognize,
		Document:      c.Document,
		MaxDepth:      c.MaxDepth,
		Optimize:      c.Optimize,
		Opt:           c.Opt,
		NoOpt:         c.NoOpt,
		PrintAfter:    c.PrintAfter,
		PrintTo:       c.PrintTo,
	}))
}

// GenerateLib generates only the library of TypeScript parsers, lib.ts: the module that
// parsers generated with LibImport import, or the copy of a parser generated with LibEmbed.
// The settings are not used.
func (c TypeScript) GenerateLib() ([]File, error) {
	return files(tsgen.Generate(nil, compileForGenerator, codegen.GenOptions{Lib: codegen.LibOnly, TSLib: tsLib}))
}

// compileForGenerator is the codegen.Compiler that the generators compile grammars with: it checks g
// to its static program, which is all a generator reads, and checks the recognizer of the program
// (as the engine's Program.Recognizer makes it) on first use. No backend of the engine is built.
func compileForGenerator(g *grammar.Grammar, opts compile.Options) (*compile.Program, func() (*compile.Program, error), error) {
	prog, err := check(g, opts, nil)
	if err != nil {
		return nil, nil, err
	}
	recognizer := func() (*compile.Program, error) {
		source := prog.Grammar
		if prog.Source != nil {
			source = prog.Source
		}
		o := prog.Opts
		o.Recognize, o.Recognizer = true, true
		return check(source, o, prog.Flags())
	}
	return prog, recognizer, nil
}

// check checks g with opts and, if not nil, the rule flags flags, naming the hidden rules of
// inline actions in errors as written.
func check(g *grammar.Grammar, opts compile.Options, flags []compile.RuleFlags) (*compile.Program, error) {
	names := desugar.InlineNames{}
	prog, err := compile.Check(g, opts, flags, names, nil)
	if err != nil {
		return nil, names.Rewrite(err)
	}
	return prog, nil
}

// goLibFiles and tsLibFiles hold the Go library of generated Go parsers (the package
// zeroparse.dev/zp/codegen/go/lib, in go/lib) and the TypeScript library of generated TypeScript
// parsers (ts/lib). Go generation copies the Go library into packages generated with LibEmbed and
// reads it to qualify the names generated code uses from it; TypeScript generation assembles lib.ts
// from the TypeScript library. They live in this package rather than in package engine so that only
// programs that generate code carry them. The Go library is listed file by file, without its tests,
// which a pattern cannot leave out; TestGoLibEmbedsItsSources keeps the list complete.
//
//go:embed go/lib/budget.go go/lib/budget_memory.go go/lib/call_exec.go go/lib/chunk.go
//go:embed go/lib/class.go go/lib/context_memo.go go/lib/document.go go/lib/expected.go
//go:embed go/lib/fieldcheck.go go/lib/lib.go go/lib/memo_index.go go/lib/node_build.go
//go:embed go/lib/node_rep.go go/lib/operators_exec.go go/lib/profile.go go/lib/protocol.go
//go:embed go/lib/recovered.go go/lib/scan_sites.go go/lib/skip_runs.go go/lib/stream.go
//go:embed go/lib/stream_exec.go go/lib/term_exec.go go/lib/term_value.go go/lib/terminal.go
//go:embed go/lib/trace.go go/lib/typed.go go/lib/typed_document.go
var goLibFiles embed.FS

//go:embed ts/lib/*.ts
var tsLibFiles embed.FS

// goLib and tsLib are the embedded libraries, which generation gives to the generators.
var (
	goLib = subFS(goLibFiles, "go/lib")
	tsLib = subFS(tsLibFiles, "ts/lib")
)

func subFS(fsys embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
