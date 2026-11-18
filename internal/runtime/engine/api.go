package engine

import (
	"context"
	"io"

	"zeroparse.dev/zp/internal/runtime/document"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/runtime"
	"zeroparse.dev/zp/pkg/runtime/input"
	"zeroparse.dev/zp/pkg/runtime/instrument"
	"zeroparse.dev/zp/pkg/runtime/schema"
	"zeroparse.dev/zp/pkg/runtime/value"
)

// This file is the option-driven API of the engine: Parser and Document run a Program with
// ParseOption functions. The public parsers (package vm and the reference parser of package
// compiler) wrap them with the options of package runtime, which Use converts; the tests and tools
// of the repository use them directly, with the options that select the backend and recognition.

// ParseOption configures one parse, or the defaults of a Document: the public options through
// Use and the forwarders below, and the backend (WithBackend) and recognition (WithRecognize).
type ParseOption func(*ParseOptions)

// DefaultBackend selects the backend of the program: Closure for a program compiled from a
// grammar and Bytecode for one loaded from a compiled grammar.
const DefaultBackend = Default

// Use applies the public options opts, in order, over the options of the parse.
func Use(opts ...runtime.ParseOption) ParseOption {
	return func(o *ParseOptions) { Apply(o, opts) }
}

// Apply applies the public options opts, in order, to o. The fields that the public options do
// not have (Backend, Recognize and the references of the dependency memo) keep their values.
func Apply(o *ParseOptions, opts []runtime.ParseOption) {
	if len(opts) == 0 {
		return
	}
	p := runtime.ParseOptions{
		Context: o.Context, Limits: o.Limits, MissingTokens: o.MissingTokens, Expectations: o.Expectations,
		Unit: o.Unit, TraceSelection: o.TraceSelection, Profiles: o.Profiles,
		DiagnosticCallback: o.DiagnosticCallback, DiagnosticLimit: o.DiagnosticLimit, DiagnosticLimitSet: o.DiagnosticLimitSet,
	}
	if o.Trace != nil {
		p.Traces = []func(TraceEvent){o.Trace}
	}
	for _, opt := range opts {
		opt(&p)
	}
	o.Context, o.Limits, o.MissingTokens, o.Expectations = p.Context, p.Limits, p.MissingTokens, p.Expectations
	o.Unit, o.TraceSelection, o.Profiles = p.Unit, p.TraceSelection, p.Profiles
	o.DiagnosticCallback, o.DiagnosticLimit, o.DiagnosticLimitSet = p.DiagnosticCallback, p.DiagnosticLimit, p.DiagnosticLimitSet
	o.Trace = nil
	for _, f := range p.Traces {
		if o.Trace == nil {
			o.Trace = f
		} else {
			o.Trace = ComposeTrace(o.Trace, f)
		}
	}
}

// Options returns the options that opts give.
func Options(opts ...ParseOption) ParseOptions {
	var o ParseOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithBackend selects the backend that runs the parse.
func WithBackend(b Backend) ParseOption { return func(o *ParseOptions) { o.Backend = b } }

// WithRecognize makes Parse recognize: it returns a nil node and the errors of Recognize.
func WithRecognize() ParseOption { return func(o *ParseOptions) { o.Recognize = true } }

// The forwarders of the public options; see package runtime.

func WithContext(ctx context.Context) ParseOption      { return Use(runtime.WithContext(ctx)) }
func WithLimits(l runtime.Limits) ParseOption          { return Use(runtime.WithLimits(l)) }
func WithMissingTokens(limit int) ParseOption          { return Use(runtime.WithMissingTokens(limit)) }
func WithExpectations() ParseOption                    { return Use(runtime.WithExpectations()) }
func WithUnit(u input.PositionUnit) ParseOption        { return Use(runtime.WithUnit(u)) }
func WithTrace(f func(TraceEvent)) ParseOption         { return Use(runtime.WithTrace(f)) }
func WithTraceFilter(rules ...string) ParseOption      { return Use(runtime.WithTraceFilter(rules...)) }
func WithTraceMaxDepth(n int) ParseOption              { return Use(runtime.WithTraceMaxDepth(n)) }
func WithProfile(prof *instrument.Profile) ParseOption { return Use(runtime.WithProfile(prof)) }
func WithDiagnosticLimit(n int) ParseOption            { return Use(runtime.WithDiagnosticLimit(n)) }
func WithDiagnosticCallback(f func(*value.SyntaxError) error) ParseOption {
	return Use(runtime.WithDiagnosticCallback(f))
}

// VMParser returns the parser of package vm (*vm.Parser) for p, and VMEngine the engine parser
// behind a parser of package vm, or nil for another parser. Package vm sets both when it is
// initialized, so that a program that imports vm, as package compiler does, makes vm parsers of
// programs compiled from grammars, and the tools of the module read the program of a public
// parser, without a constructor or an accessor in the API of vm.
var (
	VMParser func(p *Parser) runtime.Parser
	VMEngine func(p runtime.Parser) *Parser
)

// Of returns the engine parser behind p, a parser of package vm or a reference parser, or nil for
// a parser of another implementation of runtime.Parser.
func Of(p runtime.Parser) *Parser {
	if r, ok := p.(reference); ok {
		return r.p
	}
	if VMEngine != nil {
		return VMEngine(p)
	}
	return nil
}

// Parser parses with a Program from one start rule, on a default backend that WithBackend
// overrides. It is safe for concurrent use.
type Parser struct {
	prog    *Program
	start   string
	backend Backend
}

// NewParser returns a parser for prog that starts at the rule start, which must be a start rule;
// an empty start selects the default start rule. It runs on the default backend of prog.
func NewParser(prog *Program, start string) (*Parser, error) {
	if start == "" {
		start = prog.DefaultStart()
	} else if err := prog.CheckStart(start); err != nil {
		return nil, err
	}
	return &Parser{prog: prog, start: start}, nil
}

// LoadParser loads a parser saved by Marshal (see LoadProgram).
func LoadParser(data []byte) (*Parser, error) {
	prog, start, err := LoadProgram(data)
	if err != nil {
		return nil, err
	}
	return NewParser(prog, start)
}

// Program returns the program of the parser.
func (p *Parser) Program() *Program { return p.prog }

// Backend returns the default backend of the parser.
func (p *Parser) Backend() Backend { return p.backend }

// OnBackend returns a parser that shares the program and the start rule and runs on b by default.
func (p *Parser) OnBackend(b Backend) *Parser {
	return &Parser{prog: p.prog, start: p.start, backend: b}
}

// options returns the options of a parse: the default backend of p, then opts.
func (p *Parser) options(opts []ParseOption) ParseOptions {
	o := ParseOptions{Backend: p.backend}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// Parse parses the whole input and returns the value of the start rule.
func (p *Parser) Parse(input string, opts ...ParseOption) (*value.Node, error) {
	return p.prog.ParseWith(p.start, input, p.options(opts))
}

// Recognize decides whether the input matches the grammar without building a tree.
func (p *Parser) Recognize(input string, opts ...ParseOption) error {
	o := p.options(opts)
	o.Recognize = true
	_, err := p.prog.ParseWith(p.start, input, o)
	return err
}

// ParseStream parses input read from r and passes each element of the #stream repetition at the
// top level of the start rule to emit.
func (p *Parser) ParseStream(r io.Reader, emit func(*value.Node) error, opts ...ParseOption) error {
	return p.prog.ParseStreamWith(p.start, r, emit, p.options(opts))
}

// ParsePrefix parses the named start rule at the beginning of input without requiring
// whole-input consumption; an empty name selects the grammar's default start rule.
func (p *Parser) ParsePrefix(name, input string, opts ...ParseOption) (PrefixResult, error) {
	return p.prog.ParsePrefixWith(name, input, p.options(opts))
}

// Iterate validates a #stream parser and returns a lazy pull iterator.
func (p *Parser) Iterate(r io.Reader, opts ...ParseOption) (*runtime.StreamIterator, error) {
	return p.prog.Iterate(p.start, r, p.options(opts))
}

// NewDocument creates a Document for text; opts select its fixed unit and backend and the
// defaults of its parses.
func (p *Parser) NewDocument(text string, opts ...ParseOption) (*Document, error) {
	d, err := p.prog.NewDocumentWith(p.start, text, p.options(opts))
	if err != nil {
		return nil, err
	}
	return &Document{d}, nil
}

// WithStart returns a parser that starts at the rule name, which must be a start rule; an empty
// name selects the default start rule. The program and the default backend are shared.
func (p *Parser) WithStart(name string) (*Parser, error) {
	if err := p.prog.CheckStart(name); err != nil {
		return nil, err
	}
	if name == "" {
		name = p.prog.DefaultStart()
	}
	return &Parser{prog: p.prog, start: name, backend: p.backend}, nil
}

// Start returns the name of the parser's start rule.
func (p *Parser) Start() string { return p.start }

// Starts returns the start rules of the grammar in declaration order.
func (p *Parser) Starts() []string { return p.prog.Starts() }

// DefaultStart returns the default start rule of the grammar.
func (p *Parser) DefaultStart() string { return p.prog.DefaultStart() }

// DefaultValue reports whether the program makes default values instead of running actions.
func (p *Parser) DefaultValue() bool { return p.prog.DefaultValue() }

// SourceInfo reports whether the program records the rule of each node and the capture names.
func (p *Parser) SourceInfo() bool { return p.prog.SourceInfo() }

// MarshalBinary encodes the parser in parse mode; see Marshal.
func (p *Parser) MarshalBinary() ([]byte, error) { return p.prog.Marshal(p.start, CompiledParse) }

// Marshal encodes the parser in the .zpc format: the bytecode of mode and the start rule.
func (p *Parser) Marshal(mode CompiledMode) ([]byte, error) { return p.prog.Marshal(p.start, mode) }

// CompiledMode returns the mode of a parser loaded from a compiled grammar and true, or false
// for a parser compiled from a grammar, which serves every mode.
func (p *Parser) CompiledMode() (CompiledMode, bool) { return p.prog.Mode() }

// SchemaSource is a program source that exports output schemas: the source of a program compiled
// from a grammar.
type SchemaSource interface {
	OutputSchema(start string, allRules bool) (*schema.Output, error)
}

// OutputSchema exports the output schema of the start rule, or of every rule and type with
// runtime.WithAllSchemaRules. A parser loaded from a compiled grammar has no schema.
func (p *Parser) OutputSchema(opts ...runtime.SchemaOption) (*schema.Output, error) {
	if p == nil || p.prog == nil {
		return nil, runtime.ErrSchemaUnavailable
	}
	var o runtime.SchemaOptions
	for _, opt := range opts {
		opt(&o)
	}
	if err := program.EntryError(p.start); err != nil {
		return nil, err
	}
	src, ok := p.prog.Source().(SchemaSource)
	if !ok {
		return nil, program.ErrSchemaNeedsSource
	}
	return src.OutputSchema(p.start, o.AllRules)
}

// Document is a text that is edited repeatedly and parsed incrementally, with ParseOption
// functions. It is not safe for concurrent use.
type Document struct {
	doc *document.Document
}

// State returns the document state that d drives.
func (d *Document) State() *document.Document { return d.doc }

// Version returns the current text revision.
func (d *Document) Version() DocumentVersion { return d.doc.Version() }

// Unit returns the fixed position unit of the document.
func (d *Document) Unit() input.PositionUnit { return d.doc.Unit() }

// Text returns the current text.
func (d *Document) Text() string { return d.doc.Text() }

// Edit replaces the range [start, end) of the text, in the document's position unit, with text.
func (d *Document) Edit(start, end int, text string) error { return d.doc.Edit(start, end, text) }

// EditBatch applies edits, ordered by their position in the base revision, atomically.
func (d *Document) EditBatch(base DocumentVersion, edits ...input.TextEdit) error {
	return d.doc.EditBatch(base, edits...)
}

// defaults returns the construction defaults of d with opts applied.
func (d *Document) defaults(opts []ParseOption) ParseOptions {
	o := d.doc.ParseDefaults()
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// Parse parses the current text with the construction defaults and opts.
func (d *Document) Parse(opts ...ParseOption) (*value.Node, error) {
	return d.doc.ParseWith(d.defaults(opts))
}

// Snapshot parses and freezes this revision with the construction defaults and opts.
func (d *Document) Snapshot(opts ...ParseOption) (*Snapshot, error) {
	return d.doc.SnapshotWith(d.defaults(opts))
}

// Stats returns the evaluation and reuse counts of the last Parse.
func (d *Document) Stats() runtime.ParseStats { return d.doc.Stats() }

// CacheStats inspects the retained incremental history without changing it.
func (d *Document) CacheStats() runtime.CacheStats { return d.doc.CacheStats() }

// ResetHistory discards the incremental reuse history.
func (d *Document) ResetHistory() { d.doc.ResetHistory() }

// EnableChanges opts into bounded change notifications; see document.Document.EnableChanges.
func (d *Document) EnableChanges(queueLimit int) error { return d.doc.EnableChanges(queueLimit) }

// DrainChanges transfers the queued change notifications.
func (d *Document) DrainChanges() []DocumentChange { return d.doc.DrainChanges() }
