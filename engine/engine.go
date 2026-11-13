// Package engine compiles ZeroParse grammars and parses input with them at run time: batch parses,
// streams, incremental Documents and traces, on the closure backend and the bytecode VMs. Compiled
// grammars (.zpc) load with LoadParser. The types a parse shares with generated parsers are in
// package core, and generating parser source code is package codegen.
//
// A grammar is written in the ZeroParse grammar language (see ParseGrammar), built as an AST with the grammar
// package, or decoded from JSON. Compile turns it into a Parser, and Parser.Parse parses input with it.
package engine

import (
	"context"
	"io"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/limits"
	ischema "zeroparse.dev/zp/internal/schema"
	"zeroparse.dev/zp/internal/syntax"
	"zeroparse.dev/zp/schema"
)

// ParseGrammar parses ZeroParse source code into a grammar AST.
func ParseGrammar(src string) (*grammar.Grammar, error) {
	return syntax.Parse(src)
}

// Parser is a compiled grammar. It is safe for concurrent use by multiple goroutines.
type Parser struct {
	prog  *engine.Program
	start string
}

// CompileOption configures compilation.
type CompileOption func(*engine.Options)

// WithSourceInfo records source information in parse results: the rule that produced each node
// and where its captures are. Node.Rule, Node.Field, the rule and capture names of JSON and
// S-expression output, and rule constraints in queries need it. It costs time and memory, so
// parsers omit it by default.
func WithSourceInfo() CompileOption {
	return func(o *engine.Options) { o.SourceInfo = true }
}

// WithDefaultValue makes every rule produce the default value of its expression. Value actions
// (=>) and the declared types of rules are dropped, so the parser returns the default node tree
// even for grammars with actions, and nothing is checked against the types the actions would
// have made. Predicates still run, because they decide the match, and the rules that
// make the values predicates read keep their actions and types. It is a property of the
// parser: a compiled grammar written from it keeps it.
func WithDefaultValue() CompileOption {
	return func(o *engine.Options) { o.DefaultValue = true }
}

// Compile compiles the grammar g into a parser that starts parsing at the rule start, which must
// be a start rule, a rule marked @start. An empty start selects the default start rule.
func Compile(g *grammar.Grammar, start string, opts ...CompileOption) (*Parser, error) {
	var o engine.Options
	for _, opt := range opts {
		opt(&o)
	}
	prog, err := engine.Compile(g, o)
	if err != nil {
		return nil, err
	}
	if start == "" {
		start = prog.DefaultStart()
	} else if err := prog.CheckStart(start); err != nil {
		return nil, err
	}
	return &Parser{prog: prog, start: start}, nil
}

// DefaultValue reports whether the parser produces default values (WithDefaultValue).
func (p *Parser) DefaultValue() bool { return p.prog.DefaultValue() }

// SourceInfo reports whether the parser records source information (WithSourceInfo).
func (p *Parser) SourceInfo() bool { return p.prog.SourceInfo() }

// CompileSource compiles ZeroParse source code into a parser that starts parsing at the rule start, as
// Compile does.
func CompileSource(src, start string, opts ...CompileOption) (*Parser, error) {
	g, err := ParseGrammar(src)
	if err != nil {
		return nil, err
	}
	return Compile(g, start, opts...)
}

// ParseOption configures a single parse.
type ParseOption func(*engine.ParseOptions)

// WithContext selects invocation-local cooperative cancellation. Nil disables it.
func WithContext(ctx context.Context) ParseOption {
	return func(o *engine.ParseOptions) { o.Context = ctx }
}

// WithLimits sets the invocation-local limits field by field: a nonzero field overrides the value
// of an earlier WithLimits, including the construction defaults of a Document, and a zero field
// leaves it unchanged. A field that no option sets takes its default; Unlimited disables a limit.
func WithLimits(l core.Limits) ParseOption {
	return func(o *engine.ParseOptions) { o.Limits = limits.Merge(o.Limits, l) }
}

// WithMissingTokens bounds attempted #missing literal insertions in one parse.
// Zero disables insertion; negative limits are invalid. Only Closure whole-input
// parsing and recognition support insertion. Positive limits are invalid with
// ParsePrefix, even without #missing in the grammar. ParsePrefix also rejects
// grammars containing #missing when insertion is disabled. Options resolve in order.
func WithMissingTokens(limit int) ParseOption {
	return func(o *engine.ParseOptions) { o.MissingTokens = limit }
}

// WithExpectations records what a syntax error expects. A syntax error then lists the
// expectations at the farthest failure and carries the messages of @error. Without it the error
// reports the farthest failure position, an empty Expected list and no Messages, and parsing does
// not record expectations. A Document fixes it at construction.
func WithExpectations() ParseOption {
	return func(o *engine.ParseOptions) { o.Expectations = true }
}

// WithUnit selects the position unit. Matching always proceeds by code point, whatever the unit.
func WithUnit(u core.PositionUnit) ParseOption {
	return func(o *engine.ParseOptions) { o.Unit = u }
}

// Backend selects how a grammar is executed. All backends produce identical results.
type Backend int

const (
	// DefaultBackend uses Closure for a parser compiled from a grammar and Bytecode for one
	// loaded from a compiled grammar.
	DefaultBackend = Backend(engine.Default)
	// Closure compiles parsing expressions into Go closures.
	Closure = Backend(engine.Closure)
	// Bytecode runs language-independent bytecode (see spec/bytecode/README.md) on a VM that uses Go
	// recursion for rule calls.
	Bytecode = Backend(engine.Bytecode)
	// BytecodeIterative runs the bytecode on a VM that keeps rule calls on its own stack, so deeply
	// nested input does not consume the Go stack.
	BytecodeIterative = Backend(engine.BytecodeIterative)
)

// String returns the name of the backend.
func (b Backend) String() string { return engine.Backend(b).String() }

// WithBackend selects the backend.
func WithBackend(b Backend) ParseOption {
	return func(o *engine.ParseOptions) { o.Backend = engine.Backend(b) }
}

// RecognizeOnly checks whether the input matches the grammar without building a tree. Parse then returns a
// nil node and the same syntax errors as a full parse. Actions are not evaluated, so runtime errors in
// actions are not reported; captures that predicates read are still built. Recognition is faster and
// allocates less than a full parse. It cannot be used with ParseStream or Document.
func RecognizeOnly() ParseOption {
	return func(o *engine.ParseOptions) { o.Recognize = true }
}

// TraceEvent describes the start (TraceEnter) or the end (TraceExit) of a rule call. See WithTrace.
type TraceEvent = engine.TraceEvent

// TraceKind tells whether a TraceEvent starts or ends a rule call.
type TraceKind = engine.TraceKind

const (
	// TraceEnter is reported when a rule is called, before the memo is consulted.
	TraceEnter = engine.TraceEnter
	// TraceExit is reported when a rule call returns.
	TraceExit = engine.TraceExit
)

// WithTrace calls f at the start and at the end of every rule call of the parse, in order, so that the
// events nest like the calls (the depth of an event is in TraceEvent.Depth). It reports the calls the
// backend makes: a call skipped because the next character cannot start the rule (first-character
// dispatch) is not reported, and the backends do not skip the same calls. Tracing does not change the
// result, but it makes the parse several times slower. The event's methods (LineCol, Text, Failure) read
// the parser's state, so call them from f; its fields can be kept. With several WithTrace options, each
// function is called in order. A nil f is ignored.
//
// Tracing works with every backend, with Parse, ParsePrefix, RecognizeOnly, ParseStream and Document (whose
// option applies to every Document.Parse), but not with generated Go parsers.
func WithTrace(f func(TraceEvent)) ParseOption {
	return func(o *engine.ParseOptions) {
		if f == nil {
			return
		}
		if prev := o.Trace; prev != nil {
			o.Trace = engine.ComposeTrace(prev, f)
			return
		}
		o.Trace = f
	}
}

// WithDiagnosticCallback reports each committed #recover error during ParseStream or Iterate,
// immediately before the corresponding stream element. Speculative recoveries are not reported.
// The callback receives an independent copy; returning an error stops parsing and suppresses
// that element. A nil callback disables delivery. Non-nil callbacks are invalid with Parse,
// ParsePrefix and Document. Recoveries in a successful suffix are reported before the terminal result. With or
// without this option, recovered errors remain available at stream completion.
func WithDiagnosticCallback(f func(*core.SyntaxError) error) ParseOption {
	return func(o *engine.ParseOptions) { o.DiagnosticCallback = f }
}

// WithDiagnosticLimit retains at most n committed stream recoveries, keeping the latest errors.
// Zero retains every error, as in an unconfigured stream parse. Negative values are invalid.
// This option is valid only with ParseStream and Iterate, including when n is zero.
// When errors are dropped, the terminal error is *TruncatedSyntaxErrors.
func WithDiagnosticLimit(n int) ParseOption {
	return func(o *engine.ParseOptions) {
		o.DiagnosticLimit = n
		o.DiagnosticLimitSet = true
	}
}

// Profile accumulates the cost of each rule over the parses given WithProfile: calls, body evaluations,
// memo hits, matches and failures, the input consumed and the input examined by failed calls, repeated
// evaluations at a position, and time. Hints summarizes where to look. A Profile is not safe for
// concurrent use.
type Profile = engine.Profile

// RuleProfile is the cost of one rule in a Profile.
type RuleProfile = engine.RuleProfile

// Location is a position with its line and column.
type Location = engine.Location

// WithProfile adds the cost of the parse to prof. It traces the parse (see WithTrace), so the parse is
// several times slower, and the times in prof are meaningful relative to each other only. A nil prof is
// ignored.
func WithProfile(prof *Profile) ParseOption {
	if prof == nil {
		return WithTrace(nil)
	}
	return WithTrace(prof.Trace)
}

func parseOptions(opts []ParseOption) engine.ParseOptions {
	var o engine.ParseOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// Parse parses the whole input and returns the value of the start rule.
// If the input does not match, it returns a *SyntaxError. If the parse recovered from errors with
// #recover, it returns both the node and the SyntaxErrors.
func (p *Parser) Parse(input string, opts ...ParseOption) (*core.Node, error) {
	return p.prog.ParseWith(p.start, input, parseOptions(opts))
}

// ParseStream parses input read from r and passes each element of the #stream repetition at the top level
// of the start rule to emit as soon as it matches. Emitted elements and the input before them are not
// retained, so the input never has to fit in memory. If emit returns an error, parsing stops and
// ParseStream returns that error. An element without a value (for example, one that is discarded with -)
// is passed as nil. As in any repetition, an element that matches without consuming input ends it.
// WithContext and WithLimits apply to this invocation. Cancellation cannot interrupt a blocked
// reader or callback; the caller must arrange for it to return. A returned reader or emit error wins
// over cancellation first observed after that call. Delivered elements remain caller-owned, and
// retrying an interrupted stream requires caller-managed input replay.
// WithDiagnosticCallback delivers committed recoveries immediately before their elements; its
// error stops parsing before that element is emitted. Recovered errors are still returned at EOF.
// WithDiagnosticLimit bounds the retained recovery tail and explicitly marks a truncated result.
func (p *Parser) ParseStream(r io.Reader, emit func(*core.Node) error, opts ...ParseOption) error {
	return p.prog.ParseStreamWith(p.start, r, emit, parseOptions(opts))
}

// Document is a text that is edited repeatedly. Parsing after an edit reuses the results of the previous
// parse that the edit did not affect (incremental parsing), as needed by editors. A Document is not safe
// for concurrent use.
//
// Reused results are shared with the trees returned by earlier parses, and the nodes after an edit are
// moved to their new positions in place: an earlier tree changes when Parse runs again, and is then a
// mix of nodes at new positions (those reused) and at old ones. Parse writes it, so it must not be read
// concurrently with Parse. Use Node.Clone to keep a tree as it was.
type Document struct {
	doc *engine.Document
}

// DocumentVersion identifies one text revision of a Document. Its zero value
// is invalid; tokens are comparable and do not retain document contents.
type DocumentVersion struct {
	v engine.DocumentVersion
}

// Sequence returns the text revision number, starting at one.
func (v DocumentVersion) Sequence() uint64 { return v.v.Sequence() }

// Version returns the current text revision, initially sequence one.
func (d *Document) Version() DocumentVersion { return DocumentVersion{d.doc.Version()} }

// Unit returns the position unit selected when the document was constructed.
func (d *Document) Unit() core.PositionUnit { return d.doc.Unit() }

// EditBatch atomically applies ordered, nonoverlapping edits in base coordinates.
// It rejects stale or foreign tokens, repeated starts, invalid ranges and byte
// boundaries. An empty batch validates base but changes nothing. A nonempty
// batch advances one revision and clears parse caches without parsing.
func (d *Document) EditBatch(base DocumentVersion, edits ...core.TextEdit) error {
	return d.doc.EditBatch(base.v, edits...)
}

// Snapshot is an immutable document parse outcome for one text revision. Readers may use it
// concurrently with later edits of its Document.
type Snapshot struct {
	s *engine.Snapshot
}

// Version returns the captured text revision.
func (s *Snapshot) Version() DocumentVersion { return DocumentVersion{s.s.Version()} }

// Text returns the captured text.
func (s *Snapshot) Text() string { return s.s.Text() }

// Unit returns the position unit of the captured tree.
func (s *Snapshot) Unit() core.PositionUnit { return s.s.Unit() }

// Root returns the captured root, or the zero handle for a nil result.
func (s *Snapshot) Root() SnapshotNode { return SnapshotNode{s.s.Root()} }

// Error returns a copy of the captured syntax diagnostic, or nil.
func (s *Snapshot) Error() error { return s.s.Error() }

// Stats returns the evaluation and memo counts of the captured parse.
func (s *Snapshot) Stats() core.ParseStats { return s.s.Stats() }

// Materialize makes an independent mutable tree, preserving sharing, cycles, nil child slots,
// and nil versus allocated-empty slices.
func (s *Snapshot) Materialize() *core.Node { return s.s.Materialize() }

// SnapshotNode is a comparable read-only node handle; its zero value is nil.
type SnapshotNode struct {
	n engine.SnapshotNode
}

// Valid reports whether the handle denotes a non-nil node.
func (n SnapshotNode) Valid() bool { return n.n.Valid() }

// Kind returns what the node stands for, as Node.Kind does. Like Node.Kind, it panics on a nil
// node.
func (n SnapshotNode) Kind() core.NodeKind { return n.n.Kind() }

// Type returns the node type, or an empty string for nil.
func (n SnapshotNode) Type() string { return n.n.Type() }

// Rule returns the originating rule, or an empty string for nil.
func (n SnapshotNode) Rule() string { return n.n.Rule() }

// Start returns the inclusive start position.
func (n SnapshotNode) Start() int32 { return n.n.Start() }

// End returns the exclusive end position.
func (n SnapshotNode) End() int32 { return n.n.End() }

// Text returns terminal text.
func (n SnapshotNode) Text() string { return n.n.Text() }

// IsTerminal reports whether the node is terminal.
func (n SnapshotNode) IsTerminal() bool { return n.n.IsTerminal() }

// ChildCount returns the number of child slots, nil slots included.
func (n SnapshotNode) ChildCount() int { return n.n.ChildCount() }

// Child returns an indexed child; an invalid index panics like slice indexing.
func (n SnapshotNode) Child(i int) SnapshotNode { return SnapshotNode{n.n.Child(i)} }

// Len returns the number of elements of a tuple or a list (KindTuple, KindList, KindTupleValue,
// KindListValue), counting nil elements, and 0 for other nodes and for nil, as Node.Len does.
func (n SnapshotNode) Len() int { return n.n.Len() }

// Value returns the value of a KindValue node (such as an int, a string or a bool) and the
// string of a KindString node; it returns nil for other nodes and for nil, as Node.Value does.
func (n SnapshotNode) Value() any { return snapshotValue(n.n.Value()) }

// Field returns the value of the named field, a scalar or a SnapshotNode, and whether the field
// is present: a present field may hold nil.
func (n SnapshotNode) Field(name string) (any, bool) {
	v, ok := n.n.Field(name)
	return snapshotValue(v), ok
}

// Fields returns the fields in stored order, in a slice the caller owns. A nil node value is a
// zero SnapshotNode.
func (n SnapshotNode) Fields() []SnapshotField {
	fs := n.n.Fields()
	if fs == nil {
		return nil
	}
	out := make([]SnapshotField, len(fs))
	for i, f := range fs {
		out[i] = SnapshotField{Name: f.Name, Value: snapshotValue(f.Value)}
	}
	return out
}

// SnapshotField is a field of a SnapshotNode. Its Value is an immutable scalar or a SnapshotNode,
// never a mutable Node.
type SnapshotField struct {
	Name  string
	Value any
}

// snapshotValue gives a field or node value with node handles as SnapshotNode.
func snapshotValue(v any) any {
	if h, ok := v.(engine.SnapshotNode); ok {
		return SnapshotNode{h}
	}
	return v
}

// Snapshot parses and freezes this revision using construction defaults plus
// opts. Cancellation and compute limits cover parsing, text, tree and diagnostics;
// interruption returns nil. Ordinary syntax errors retain the failed version.
func (d *Document) Snapshot(opts ...ParseOption) (*Snapshot, error) {
	o := d.doc.ParseDefaults()
	for _, opt := range opts {
		opt(&o)
	}
	s, err := d.doc.SnapshotWith(o)
	if s == nil {
		return nil, err
	}
	return &Snapshot{s}, err
}

// NewDocument creates a Document for text. opts select its fixed unit and backend
// and the defaults for later Parse calls. Construction and each Parse use fresh
// budget counters. Edit uses the same unit and is outside parse budget controls.
// It returns an error if the backend is unavailable (Closure on a loaded parser) or the parser
// is loaded from a compiled grammar that is not in document mode.
func (p *Parser) NewDocument(text string, opts ...ParseOption) (*Document, error) {
	d, err := p.prog.NewDocumentWith(p.start, text, parseOptions(opts))
	if err != nil {
		return nil, err
	}
	return &Document{doc: d}, nil
}

// Text returns the current text.
func (d *Document) Text() string { return d.doc.Text() }

// Edit replaces the range [start, end) of the text, in the document's position unit, with text.
// With byte positions, start and end must be on character boundaries.
// Each invalid UTF-8 byte is a separate character; edits may join bytes into
// a valid UTF-8 character.
func (d *Document) Edit(start, end int, text string) error { return d.doc.Edit(start, end, text) }

// Parse parses the current text. opts apply once over construction defaults and
// may change context, compute limits, depth and trace, but not unit, effective backend
// or recognition mode. Overrides do not change future defaults. Pre-execution
// interruption preserves caches and statistics; execution or publication
// interruption discards reusable state while preserving text for retry.
// Parser-produced syntax diagnostics belong to that call: mutating them does
// not affect later parses. Returned Nodes may share the mutable Document graph.
func (d *Document) Parse(opts ...ParseOption) (*core.Node, error) {
	o := d.doc.ParseDefaults()
	for _, opt := range opts {
		opt(&o)
	}
	return d.doc.ParseWith(o)
}

// Stats returns the evaluation and reuse counts of the last Parse.
func (d *Document) Stats() core.ParseStats { return d.doc.Stats() }

// CompiledMode is the mode of a compiled grammar file, chosen when it is saved. A file holds the
// module of one mode and serves only the parses of that mode; the closure backend, Grammar and
// OutputSchema need the grammar source.
type CompiledMode uint8

const (
	// CompiledParse serves Parse, ParsePrefix, ParseStream and Iterate without WithTrace or
	// WithProfile.
	CompiledParse = CompiledMode(engine.CompiledParse)
	// CompiledRecognize serves RecognizeOnly parses without WithTrace or WithProfile.
	CompiledRecognize = CompiledMode(engine.CompiledRecognize)
	// CompiledDocument serves NewDocument, and Parse, ParsePrefix, ParseStream and Iterate with
	// WithTrace or WithProfile.
	CompiledDocument = CompiledMode(engine.CompiledDocument)
)

// String returns the name of the mode: "parse", "recognize" or "document".
func (m CompiledMode) String() string { return engine.CompiledMode(m).String() }

// MarshalBinary encodes the parser in parse mode (CompiledParse); see Marshal.
func (p *Parser) MarshalBinary() ([]byte, error) {
	return p.prog.Marshal(p.start, engine.CompiledParse)
}

// Marshal encodes the parser in the .zpc format: the bytecode of mode and the start rule.
// LoadParser restores it without parsing, analyzing, type checking or compiling the grammar again.
// A loaded parser is saved only in its own mode (CompiledMode), which reproduces the file it was
// loaded from; saving another mode needs the grammar source.
func (p *Parser) Marshal(mode CompiledMode) ([]byte, error) {
	return p.prog.Marshal(p.start, engine.CompiledMode(mode))
}

// IsCompiled reports whether data is a compiled grammar written by MarshalBinary or Marshal.
func IsCompiled(data []byte) bool { return engine.IsCompiled(data) }

// LoadParser loads a parser saved by MarshalBinary or Marshal. Corruption is detected by a checksum and
// structural validation, but types are not checked again: load only data from trusted sources, such as
// the output of zp compile or MarshalBinary. The parser runs on the bytecode backends and serves
// the parses of the file's mode only; any other parse returns an error that names the mode it needs.
func LoadParser(data []byte) (*Parser, error) {
	prog, start, err := engine.LoadProgram(data)
	if err != nil {
		return nil, err
	}
	if start == "" {
		start = prog.DefaultStart()
	}
	return &Parser{prog: prog, start: start}, nil
}

// CompiledMode returns the mode of a parser loaded with LoadParser and true, or false for a parser
// compiled from a grammar, which serves every mode.
func (p *Parser) CompiledMode() (CompiledMode, bool) {
	m, ok := p.prog.Mode()
	return CompiledMode(m), ok
}

// WithStart returns a parser that starts at the rule name, which must be a start rule; an empty
// name selects the grammar's default start rule. The compiled grammar is shared.
func (p *Parser) WithStart(name string) (*Parser, error) {
	if err := p.prog.CheckStart(name); err != nil {
		return nil, err
	}
	if name == "" {
		name = p.prog.DefaultStart()
	}
	return &Parser{prog: p.prog, start: name}, nil
}

// Start returns the name of the parser's start rule.
func (p *Parser) Start() string { return p.start }

// Starts returns the start rules of the grammar, the rules marked @start where parsing may begin,
// in declaration order. The one marked @start(default), or the only one, is the default.
func (p *Parser) Starts() []string { return p.prog.Starts() }

// DefaultStart returns the default start rule of the grammar, which Compile selects for an empty
// start.
func (p *Parser) DefaultStart() string { return p.prog.DefaultStart() }

// Grammar returns the parser's grammar, or nil for a parser loaded from a compiled grammar.
func (p *Parser) Grammar() *grammar.Grammar { return p.prog.SourceGrammar() }

// PrefixResult reports a rule's value and successful cursor. Consumption comes
// from the runtime cursor, even when Node is nil or its span differs.
type PrefixResult struct {
	Node          *core.Node
	Consumed      int // Selected parser Unit; defaults to CodePoints.
	ConsumedBytes int // Exact UTF-8 byte endpoint in the supplied input.
}

// ParsePrefix parses the named start rule at the beginning of input without requiring
// whole-input consumption; an empty name selects the grammar's default start rule. EOF assertions still refer to the complete input.
// Positions remain local to input. For a source slice, add its starting byte
// offset to ConsumedBytes to find the consumed endpoint in the original source.
// Recovered success returns the result and SyntaxErrors. Unrecovered syntax
// failures and fatal errors return a zero result. A zero-width success returns
// zero consumption; a nil Node does not imply failure. Parse remains whole-input.
// WithDiagnosticCallback and positive WithMissingTokens limits are invalid for
// this operation. Grammars containing #missing are unsupported even when
// insertion is disabled.
func (p *Parser) ParsePrefix(name, input string, opts ...ParseOption) (PrefixResult, error) {
	r, err := p.prog.ParsePrefixWith(name, input, parseOptions(opts))
	return PrefixResult{Node: r.Node, Consumed: r.Consumed, ConsumedBytes: r.ConsumedBytes}, err
}

// Iterate validates a #stream parser and returns a lazy pull iterator. Reader
// access begins on Next; lookahead and buffering may still read ahead. With a
// diagnostic callback, Next reports committed recoveries before their element.
func (p *Parser) Iterate(r io.Reader, opts ...ParseOption) (*core.StreamIterator, error) {
	return p.prog.Iterate(p.start, r, parseOptions(opts))
}

// DocumentChange owns an edit operation or an outcome comparison. Edit From/To
// advance source revisions; outcome From/To identify compared snapshots. Consumers
// track those streams separately. FullRefresh requires rebuilding current state.
type DocumentChange struct {
	Kind        core.DocumentChangeKind
	From, To    DocumentVersion
	Unit        core.PositionUnit
	Edits       []core.TextEdit
	FullRefresh bool
	// Invalidated is conservative: an accepted edit may affect the whole output.
	// Reevaluation remains unknown; invalidation is not evidence of execution.
	Invalidated  bool
	Reevaluation string
	Output       core.OutputChange
}

// EnableChanges opts into bounded notifications and one retained comparison
// snapshot. queueLimit bounds record count, not replacement or snapshot bytes.
// It must be positive; enabling twice is rejected. Future Parse and Snapshot calls
// capture and compare immutable outcomes inside their cumulative spend.
// Comparison is conservative about aliases and nil-versus-empty collections;
// this policy does not define language value equality. Interrupted invocations
// return nil and notify unavailable, breaking the comparison chain.
func (d *Document) EnableChanges(queueLimit int) error { return d.doc.EnableChanges(queueLimit) }

// DrainChanges transfers queued events. Overflow produces one full-refresh
// marker; draining resets the queue but retains the last comparison snapshot.
func (d *Document) DrainChanges() []DocumentChange {
	cs := d.doc.DrainChanges()
	if cs == nil {
		return nil
	}
	out := make([]DocumentChange, len(cs))
	for i, c := range cs {
		out[i] = DocumentChange{
			Kind: c.Kind, From: DocumentVersion{c.From}, To: DocumentVersion{c.To}, Unit: c.Unit,
			Edits: c.Edits, FullRefresh: c.FullRefresh, Invalidated: c.Invalidated,
			Reevaluation: c.Reevaluation, Output: c.Output,
		}
	}
	return out
}

// ResetHistory discards incremental reuse history while preserving text,
// construction options, version and the statistics of the last parse. Earlier
// result trees are detached from later node movement.
func (d *Document) ResetHistory() { d.doc.ResetHistory() }

// CacheStats inspects retained incremental history without changing it.
// Document methods, including this one, require exclusive ownership.
func (d *Document) CacheStats() core.CacheStats { return d.doc.CacheStats() }

// ErrSchemaUnavailable reports that checked grammar metadata is unavailable.
// In particular, a parser loaded from a compiled file cannot export a schema.
var ErrSchemaUnavailable = ischema.ErrUnavailable

type schemaOptions struct{ allRules bool }

// SchemaOption configures one output schema export.
type SchemaOption func(*schemaOptions)

// WithAllSchemaRules includes unreachable rules and declared types in the schema.
// By default, export starts at the parser's configured start rule.
func WithAllSchemaRules() SchemaOption {
	return func(o *schemaOptions) { o.allRules = true }
}

// OutputSchema exports checked grammar metadata. It does not inspect parse
// results or change parsing.
func (p *Parser) OutputSchema(opts ...SchemaOption) (*schema.Output, error) {
	if p == nil || p.prog == nil {
		return nil, ErrSchemaUnavailable
	}
	var o schemaOptions
	for _, opt := range opts {
		opt(&o)
	}
	return outputSchemaPrototype(p.prog, p.start, o.allRules)
}

// outputSchemaPrototype exports the start rule of prog, the rules it calls or whose values its
// values contain, and the struct types they refer to; or every rule and type with allRules. It
// has no effect on parsing or generation. A program loaded from a compiled grammar has no schema
// (engine.Program.SchemaProgram).
func outputSchemaPrototype(prog *engine.Program, start string, allRules bool) (*schema.Output, error) {
	sp, err := prog.SchemaProgram(start)
	if err != nil {
		return nil, err
	}
	return ischema.Export(sp, start, allRules)
}
