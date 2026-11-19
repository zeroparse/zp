// Package vm runs grammars compiled to bytecode on the ZeroParse virtual machine. LoadParser loads a
// compiled grammar (a .zpc file, written by zp compile or compiler.Marshal) without parsing,
// checking or compiling the grammar again, so a program that only loads parsers links no compiler.
// compiler.Compile returns a *Parser too, for a grammar compiled in the same program.
//
// A Parser runs on a VM that uses Go recursion for rule calls; Iterative returns one that keeps
// rule calls on its own stack, so deeply nested input does not consume the Go stack. Both produce
// the same results. A Document parses a text incrementally; besides runtime.Document it has
// versions, batch edits, immutable snapshots and change notifications.
package vm

import (
	"io"

	"zeroparse.dev/zp/internal/runtime/engine"
	"zeroparse.dev/zp/pkg/runtime"
	zpinput "zeroparse.dev/zp/pkg/runtime/input"
	"zeroparse.dev/zp/pkg/runtime/schema"
	"zeroparse.dev/zp/pkg/runtime/value"
)

// Parser parses with a grammar compiled to bytecode, from one start rule. It is safe for
// concurrent use.
type Parser struct {
	p *engine.Parser
}

var _ runtime.Parser = (*Parser)(nil)

func init() {
	engine.VMParser = func(p *engine.Parser) runtime.Parser { return newParser(p) }
	engine.VMEngine = func(p runtime.Parser) *engine.Parser {
		if q, ok := p.(*Parser); ok && q != nil {
			return q.p
		}
		return nil
	}
}

// newParser returns the vm parser of p, on the recursive VM.
func newParser(p *engine.Parser) *Parser { return &Parser{p.OnBackend(engine.Bytecode)} }

// LoadParser loads a parser saved by compiler.Marshal or zp compile. Corruption is detected by a
// checksum and structural validation, but types are not checked again: load only data from trusted
// sources, such as the output of zp compile or compiler.Marshal. The parser runs on the bytecode backends and serves
// the parses of the file's mode only; any other parse returns an error that names the mode it needs.
func LoadParser(data []byte) (*Parser, error) {
	p, err := engine.LoadParser(data)
	if err != nil {
		return nil, err
	}
	return newParser(p), nil
}

// IsCompiled reports whether data is a compiled grammar written by compiler.Marshal or zp compile.
func IsCompiled(data []byte) bool { return engine.IsCompiled(data) }

// Iterative returns a parser that shares the program and the start rule and runs on the VM that
// keeps rule calls on its own stack, so deeply nested input does not consume the Go stack. Its
// default depth limit is larger (runtime.Limits.Depth).
func (p *Parser) Iterative() *Parser { return &Parser{p.p.OnBackend(engine.BytecodeIterative)} }

// Parse parses the whole input and returns the value of the start rule. If the input does not
// match, it returns a *value.SyntaxError. If the parse recovered from errors with #recover, it
// returns both the node and the value.SyntaxErrors.
func (p *Parser) Parse(input string, opts ...runtime.ParseOption) (*value.Node, error) {
	return p.p.Parse(input, engine.Use(opts...))
}

// Recognize decides whether the input matches the grammar without building a tree. It returns the
// same success, failure and syntax errors as Parse, recovered errors included. Predicates, and the
// actions they read, run; the run-time errors of other actions and typed conversions are not
// checked. A parser loaded in document mode does not recognize.
func (p *Parser) Recognize(input string, opts ...runtime.ParseOption) error {
	return p.p.Recognize(input, engine.Use(opts...))
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
func (p *Parser) ParseStream(r io.Reader, emit func(*value.Node) error, opts ...runtime.ParseOption) error {
	return p.p.ParseStream(r, emit, engine.Use(opts...))
}

// Iterate validates a #stream parser and returns a lazy pull iterator. Reader
// access begins on Next; lookahead and buffering may still read ahead. With a
// diagnostic callback, Next reports committed recoveries before their element.
func (p *Parser) Iterate(r io.Reader, opts ...runtime.ParseOption) (*runtime.StreamIterator, error) {
	return p.p.Iterate(r, engine.Use(opts...))
}

// PrefixResult reports a rule's value and successful cursor. Consumption comes
// from the runtime cursor, even when Node is nil or its span differs.
type PrefixResult struct {
	Node          *value.Node
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
func (p *Parser) ParsePrefix(name, input string, opts ...runtime.ParseOption) (PrefixResult, error) {
	r, err := p.p.ParsePrefix(name, input, engine.Use(opts...))
	return PrefixResult{Node: r.Node, Consumed: r.Consumed, ConsumedBytes: r.ConsumedBytes}, err
}

// NewDocument creates a Document for text. opts select its fixed unit and backend
// and the defaults for later Parse calls. Construction and each Parse use fresh
// budget counters. Edit uses the same unit and is outside parse budget controls.
// It returns an error if the backend is unavailable (Closure on a loaded parser) or the parser
// is loaded from a compiled grammar that is not in document mode.
func (p *Parser) NewDocument(text string, opts ...runtime.ParseOption) (runtime.Document, error) {
	d, err := p.p.NewDocument(text, engine.Use(opts...))
	if err != nil {
		return nil, err
	}
	return &Document{doc: d}, nil
}

// WithStart returns a parser that starts at the rule name, which must be a start rule; an empty
// name selects the grammar's default start rule. The compiled grammar is shared.
func (p *Parser) WithStart(name string) (*Parser, error) {
	q, err := p.p.WithStart(name)
	if err != nil {
		return nil, err
	}
	return &Parser{q}, nil
}

// Start returns the name of the parser's start rule.
func (p *Parser) Start() string { return p.p.Start() }

// Starts returns the start rules of the grammar, the rules marked @start where parsing may begin,
// in declaration order. The one marked @start(default), or the only one, is the default.
func (p *Parser) Starts() []string { return p.p.Starts() }

// DefaultStart returns the default start rule of the grammar.
func (p *Parser) DefaultStart() string { return p.p.DefaultStart() }

// Marshal encodes the parser in the .zpc format: the bytecode of mode and the start rule.
// LoadParser restores it without parsing, analyzing, type checking or compiling the grammar again.
// A loaded parser is saved only in its own mode (CompiledMode), which reproduces the file it was
// loaded from; saving another mode needs the grammar source.
func (p *Parser) Marshal(mode CompiledMode) ([]byte, error) {
	return p.p.Marshal(engine.CompiledMode(mode))
}

// CompiledMode returns the mode of a parser loaded with LoadParser and true, or false for a parser
// compiled from a grammar, which serves every mode.
func (p *Parser) CompiledMode() (CompiledMode, bool) {
	m, ok := p.p.CompiledMode()
	return CompiledMode(m), ok
}

// OutputSchema exports the output schema of the start rule, or of every rule and type with
// runtime.WithAllSchemaRules. It does not inspect parse results or change parsing. A parser loaded
// from a compiled grammar has no grammar metadata and returns an error that wraps
// runtime.ErrSchemaUnavailable.
func (p *Parser) OutputSchema(opts ...runtime.SchemaOption) (*schema.Output, error) {
	return p.p.OutputSchema(opts...)
}

// CompiledMode is the mode of a compiled grammar file, chosen when it is saved. A file holds the
// module of one mode and serves only the parses of that mode; the closure backend, Grammar and
// OutputSchema need the grammar source.
type CompiledMode uint8

const (
	// CompiledParse serves Parse, ParsePrefix, ParseStream and Iterate without WithTrace or
	// WithProfile.
	CompiledParse CompiledMode = iota
	// CompiledRecognize serves Recognize without WithTrace or WithProfile.
	CompiledRecognize
	// CompiledDocument serves NewDocument, and Parse, ParsePrefix, ParseStream and Iterate with
	// WithTrace or WithProfile.
	CompiledDocument
)

// String returns the name of the mode: "parse", "recognize" or "document".
func (m CompiledMode) String() string { return engine.CompiledMode(m).String() }

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
func (d *Document) Unit() zpinput.PositionUnit { return d.doc.Unit() }

// EditBatch atomically applies ordered, nonoverlapping edits in base coordinates.
// It rejects stale or foreign tokens, repeated starts, invalid ranges and byte
// boundaries. An empty batch validates base but changes nothing. A nonempty
// batch advances one revision and clears parse caches without parsing.
func (d *Document) EditBatch(base DocumentVersion, edits ...zpinput.TextEdit) error {
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
func (s *Snapshot) Unit() zpinput.PositionUnit { return s.s.Unit() }

// Root returns the captured root, or the zero handle for a nil result.
func (s *Snapshot) Root() SnapshotNode { return SnapshotNode{s.s.Root()} }

// Error returns a copy of the captured syntax diagnostic, or nil.
func (s *Snapshot) Error() error { return s.s.Error() }

// Stats returns the evaluation and memo counts of the captured parse.
func (s *Snapshot) Stats() runtime.ParseStats { return s.s.Stats() }

// Materialize makes an independent mutable tree, preserving sharing, cycles, nil child slots,
// and nil versus allocated-empty slices.
func (s *Snapshot) Materialize() *value.Node { return s.s.Materialize() }

// SnapshotNode is a comparable read-only node handle; its zero value is nil.
type SnapshotNode struct {
	n engine.SnapshotNode
}

// Valid reports whether the handle denotes a non-nil node.
func (n SnapshotNode) Valid() bool { return n.n.Valid() }

// Kind returns what the node stands for, as Node.Kind does. Like Node.Kind, it panics on a nil
// node.
func (n SnapshotNode) Kind() value.NodeKind { return n.n.Kind() }

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
func (d *Document) Snapshot(opts ...runtime.ParseOption) (*Snapshot, error) {
	s, err := d.doc.Snapshot(engine.Use(opts...))
	if s == nil {
		return nil, err
	}
	return &Snapshot{s}, err
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
func (d *Document) Parse(opts ...runtime.ParseOption) (*value.Node, error) {
	return d.doc.Parse(engine.Use(opts...))
}

// Stats returns the evaluation and reuse counts of the last Parse.
func (d *Document) Stats() runtime.ParseStats { return d.doc.Stats() }

// DocumentChange owns an edit operation or an outcome comparison. Edit From/To
// advance source revisions; outcome From/To identify compared snapshots. Consumers
// track those streams separately. FullRefresh requires rebuilding current state.
type DocumentChange struct {
	Kind        zpinput.DocumentChangeKind
	From, To    DocumentVersion
	Unit        zpinput.PositionUnit
	Edits       []zpinput.TextEdit
	FullRefresh bool
	// Invalidated is conservative: an accepted edit may affect the whole output.
	// Reevaluation remains unknown; invalidation is not evidence of execution.
	Invalidated  bool
	Reevaluation string
	Output       zpinput.OutputChange
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
func (d *Document) CacheStats() runtime.CacheStats { return d.doc.CacheStats() }
