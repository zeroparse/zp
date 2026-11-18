// Package compiler makes executable artifacts from ZeroParse grammars: parsers that run on the
// bytecode VM (Compile, CompileGrammar), the reference parser that differential tests compare with
// (NewReferenceParser), compiled grammar files (Marshal), Go and TypeScript parser source
// (GenerateGo, GenerateTypeScript) and output schemas (OutputSchema).
//
// Every function takes the same Options: the start rule (WithStart), source information
// (WithSourceInfo), default values (WithDefaultValue) and the optimization setting (WithOptimize),
// so a grammar means the same parser whatever the artifact. The parsers implement runtime.Parser;
// the parser of Compile is a *vm.Parser, which also parses prefixes, iterates over streams, saves
// itself (Marshal) and runs on the iterative VM.
package compiler

import (
	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/driver/entry"
	"zeroparse.dev/zp/internal/runtime/engine"
	"zeroparse.dev/zp/pkg/grammar"
	"zeroparse.dev/zp/pkg/grammar/ast"
	"zeroparse.dev/zp/pkg/runtime"
	"zeroparse.dev/zp/pkg/runtime/schema"
	"zeroparse.dev/zp/pkg/runtime/vm"
)

// Option configures a compilation.
type Option func(*options)

// options are the settings that Options set.
type options struct {
	start        string
	sourceInfo   bool
	defaultValue bool
	optimize     Optimize
}

// newOptions returns the settings that opts give, in order.
func newOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// entry returns the compile options of the settings.
func (o options) entry() entry.Options {
	return entry.Options{SourceInfo: o.sourceInfo, DefaultValue: o.defaultValue}
}

// WithStart selects the start rule of the parser, which must be a start rule (a rule marked
// @start). An empty name, the default, selects the default start rule.
func WithStart(name string) Option { return func(o *options) { o.start = name } }

// WithSourceInfo makes the parser record the rule of each node and the names of captures in parse
// results.
func WithSourceInfo() Option { return func(o *options) { o.sourceInfo = true } }

// WithDefaultValue makes the parser drop value actions and declared rule types and return the
// default node tree: the value of each rule is what it would be without its action. Actions that
// predicates read are kept. It cannot be combined with typed generated output.
func WithDefaultValue() Option { return func(o *options) { o.defaultValue = true } }

// Optimize is an optimization setting: a profile, then values for single optimizations, then
// optimizations turned off. None of them changes what a parser returns, only how it runs. The
// names are those of the guide to generated code and of zp gen -opt and -no-opt.
type Optimize struct {
	// Profile is "speed" (the default, also for an empty name), "memory" or "size".
	Profile string
	// Values gives optimizations a value other than the profile's: 1 turns on one that the
	// profile leaves off, and a count sets an integer one.
	Values map[string]int
	// Off turns the named optimizations off, after Values.
	Off []string
}

// The optimization profiles.
var (
	// Speed is the fastest parse that does not cost code or memory out of proportion; the default.
	Speed = Optimize{Profile: "speed"}
	// Memory allocates typed values one by one: fewer bytes allocated and less live heap, with
	// many more allocations.
	Memory = Optimize{Profile: "memory"}
	// Size makes smaller generated Go source and binaries, for a little parse time.
	Size = Optimize{Profile: "size"}
)

// WithOptimize selects the optimization setting. An unknown profile or optimization, or a value
// that an optimization does not take, is an error of every function. The optimizations are
// those of the generators today: the bytecode and the reference backends have none to select, so
// for them the setting is checked and has no further effect.
func WithOptimize(o Optimize) Option { return func(opts *options) { opts.optimize = o } }

// check reports an invalid optimization setting.
func (o options) check() error {
	_, err := common.GenOptions{Optimize: o.optimize.Profile, Opt: o.optimize.Values, NoOpt: o.optimize.Off}.Optimizations()
	return err
}

// build checks g and returns the engine parser of the settings, on the program's default
// backend.
func build(g *ast.Grammar, o options) (*engine.Parser, error) {
	if err := o.check(); err != nil {
		return nil, err
	}
	return entry.CompileParser(g, o.start, o.entry())
}

// load loads the grammar of src and compiles it.
func load(src grammar.Source, o options) (*engine.Parser, error) {
	g, err := grammar.Load(src)
	if err != nil {
		return nil, err
	}
	return build(g, o)
}

// Compile compiles the grammar src to a parser that runs on the bytecode VM. The parser is a
// *vm.Parser.
func Compile(src grammar.Source, opts ...Option) (runtime.Parser, error) {
	p, err := load(src, newOptions(opts))
	if err != nil {
		return nil, err
	}
	return engine.VMParser(p), nil
}

// CompileGrammar compiles a grammar given as its syntax tree, such as one a program builds, to a
// parser that runs on the bytecode VM. The parser is a *vm.Parser.
func CompileGrammar(g *ast.Grammar, opts ...Option) (runtime.Parser, error) {
	p, err := build(g, newOptions(opts))
	if err != nil {
		return nil, err
	}
	return engine.VMParser(p), nil
}

// NewReferenceParser compiles the grammar src to the reference parser, which runs the grammar's
// Core IR as Go closures. It is the parser that the other backends are tested against; it is
// slower than the parser of Compile, and the only one that inserts missing tokens
// (runtime.WithMissingTokens). Its Documents have only the methods of runtime.Document.
func NewReferenceParser(src grammar.Source, opts ...Option) (runtime.Parser, error) {
	p, err := load(src, newOptions(opts))
	if err != nil {
		return nil, err
	}
	return engine.Reference(p), nil
}

// Marshal compiles the grammar src and encodes it in the .zpc format in mode: the bytecode that
// mode serves and the start rule. vm.LoadParser loads it without the grammar or the compiler.
func Marshal(src grammar.Source, mode vm.CompiledMode, opts ...Option) ([]byte, error) {
	p, err := load(src, newOptions(opts))
	if err != nil {
		return nil, err
	}
	return p.Marshal(engine.CompiledMode(mode))
}

// OutputSchema exports the output schema of the grammar src: the types of the values that its
// start rule returns, of the rules those values contain and of the types they refer to. The
// parsers' OutputSchema with runtime.WithAllSchemaRules exports every rule and type.
func OutputSchema(src grammar.Source, opts ...Option) (*schema.Output, error) {
	p, err := load(src, newOptions(opts))
	if err != nil {
		return nil, err
	}
	return p.OutputSchema()
}
