package compiler

import (
	"fmt"
	"io"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/driver/generate"
	"zeroparse.dev/zp/pkg/grammar"
)

// File is a file of generated code. Path is relative to the directory of the generated package
// and uses slashes.
type File struct {
	Path    string
	Content []byte
}

// files converts the files that a generator returns.
func files(generated []common.GeneratedFile, err error) ([]File, error) {
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
	// Go, which depends only on the runtime packages under zeroparse.dev/zp/pkg/runtime, and
	// lib.ts in TypeScript, which depends on nothing. It is the default. GenerateLibGo and
	// GenerateTypeScriptLib refresh the copy without regenerating the parser.
	LibEmbed LibMode = iota
	// LibImport imports the library: zeroparse.dev/zp/pkg/runtime/libgo in Go, the module of
	// TSConfig.LibModule in TypeScript.
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
func (m LibMode) internal() (common.LibMode, error) {
	switch m {
	case LibEmbed:
		return common.LibEmbed, nil
	case LibImport:
		return common.LibImport, nil
	}
	return 0, fmt.Errorf("unknown library mode %v", m)
}

// GoConfig configures the generation of a Go parser. The zero value generates an untyped parser with an
// embedded library; Module must be set for it.
type GoConfig struct {
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
	// Expectations makes the generated parser record what a syntax error expects: the expectations
	// at the farthest failure and the messages of @error, as runtime.WithExpectations does for a
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
	// rule without building a tree, like runtime.Parser.Recognize.
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
	// runtime.Parser.ParseStream does. The start rule must have a #stream repetition. With Types it
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
	// PrintAfter lists, separated by commas, the steps and passes after which the intermediate
	// representation of the generator is written to PrintTo as text, for debugging the generator:
	// core and the names of the Core passes for the Core IR, flow and the names of the Flow passes
	// for the Flow IR, exec and the names of the Exec passes for the Exec IR, or all. It needs
	// PrintTo.
	PrintAfter string
	PrintTo    io.Writer
}

// GenerateGo generates the files of a Go parser for the grammar src in the package with import
// path c.Module: its Parse(input) parses from the start rule that WithStart selects, or the
// default start rule. The files are parser.go, with the
// code generated from the grammar, api.go, which re-exports the library names the parser's users
// need, with Types types/types.go, the package Module/types that declares the Go types of the
// grammar, and, with LibEmbed, the files of a copy of the library in internal/libgo.
func GenerateGo(src grammar.Source, c GoConfig, opts ...Option) ([]File, error) {
	o := newOptions(opts)
	lib, err := c.Lib.internal()
	if err != nil {
		return nil, err
	}
	g, err := grammar.Load(src)
	if err != nil {
		return nil, err
	}
	return files(generate.Go(g, common.GenOptions{
		Module:         c.Module,
		Package:        c.PackageName,
		Start:          o.start,
		Lib:            lib,
		Types:          c.Types,
		DefaultValue:   o.defaultValue,
		SourceInfo:     o.sourceInfo,
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
		Optimize:       o.optimize.Profile,
		Opt:            o.optimize.Values,
		NoOpt:          o.optimize.Off,
		PrintAfter:     c.PrintAfter,
		PrintTo:        c.PrintTo,
	}))
}

// GenerateLibGo generates only the embedded copy of the library for the package with import
// path module, to refresh a parser generated with LibEmbed without regenerating it.
func GenerateLibGo(module string) ([]File, error) {
	return files(generate.Go(nil, common.GenOptions{Module: module, Lib: common.LibOnly}))
}

// TSConfig configures the generation of a TypeScript parser. The zero value generates an
// untyped parser with an embedded library.
type TSConfig struct {
	// Lib selects how the generated code reaches the library (LibEmbed by default).
	// LibImport needs LibModule.
	Lib LibMode
	// LibModule is the module specifier from which a parser generated with LibImport
	// imports its library, such as "@scope/zp-lib" or "../shared/lib.ts". The library
	// module is the lib.ts of GenerateTypeScriptLib (or of any parser generated with LibEmbed).
	// It is an error with another mode.
	LibModule string
	// Types generates declarations for the grammar's types and the entry points parseValue and
	// parseValueWithOptions, which convert the completed Node result into plain objects and arrays.
	// Declared aliases preserve the target value type.
	Types bool
	// Expectations makes the generated parser record what a syntax error expects: the expectations
	// at the farthest failure and the messages of @error, as runtime.WithExpectations does for a
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
	// building a tree, like runtime.Parser.Recognize.
	Recognize bool
	// Document also generates createDocument, which makes a mutable Document of the start rule:
	// edit changes its text, and parse returns what parse of the whole current text returns, as
	// the Go Document does. With Types, the Document also has parseValue.
	Document bool
	// MaxDepth is the generated parser's default nested rule-call limit. Zero selects 100,000;
	// negative values and values that are not exact JavaScript integers fail generation.
	// Individual invocations may override it with limits.depth.
	MaxDepth int
	// PrintAfter lists, separated by commas, the steps and passes after which the intermediate
	// representation of the generator is written to PrintTo as text, for debugging the generator:
	// core and the names of the Core passes for the Core IR, flow and the names of the Flow passes
	// for the Flow IR, exec and the names of the Exec passes for the Exec IR, or all. It needs
	// PrintTo.
	PrintAfter string
	PrintTo    io.Writer
}

// GenerateTypeScript generates the files of a TypeScript parser for the grammar src: its
// parse(input) parses from the start rule that WithStart selects, or the default start rule, and
// returns the same trees and errors as the parser of Compile. The files
// are
//
//   - lib.ts, the library, which does not depend on the grammar (with LibEmbed),
//   - parser.ts, the code generated from the grammar, which imports ./lib.ts, or the
//     LibModule with LibImport, and
//   - types.ts with Types, the declarations of the grammar's types, which parser.ts imports.
//
// Generated TypeScript parsers have no stream parsing.
func GenerateTypeScript(src grammar.Source, c TSConfig, opts ...Option) ([]File, error) {
	o := newOptions(opts)
	lib, err := c.Lib.internal()
	if err != nil {
		return nil, err
	}
	g, err := grammar.Load(src)
	if err != nil {
		return nil, err
	}
	return files(generate.TypeScript(g, common.GenOptions{
		Start:         o.start,
		Lib:           lib,
		LibModule:     c.LibModule,
		Types:         c.Types,
		DefaultValue:  o.defaultValue,
		SourceInfo:    o.sourceInfo,
		Observability: c.Observability,
		LimitControls: c.LimitControls,
		Expectations:  c.Expectations,
		Recognize:     c.Recognize,
		Document:      c.Document,
		MaxDepth:      c.MaxDepth,
		Optimize:      o.optimize.Profile,
		Opt:           o.optimize.Values,
		NoOpt:         o.optimize.Off,
		PrintAfter:    c.PrintAfter,
		PrintTo:       c.PrintTo,
	}))
}

// GenerateTypeScriptLib generates only the library of TypeScript parsers, lib.ts: the module
// that parsers generated with LibImport import, or the copy of a parser generated with LibEmbed.
func GenerateTypeScriptLib() ([]File, error) {
	return files(generate.TypeScript(nil, common.GenOptions{Lib: common.LibOnly}))
}
