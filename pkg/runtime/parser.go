package runtime

import (
	"errors"
	"io"

	"zeroparse.dev/zp/pkg/runtime/input"
	"zeroparse.dev/zp/pkg/runtime/schema"
	"zeroparse.dev/zp/pkg/runtime/value"
)

// Parser parses inputs with one grammar from one start rule. A parser compiled from a grammar
// (package compiler) and one loaded from a compiled grammar (package vm) implement it; the
// concrete types offer more, such as a choice of start rule or prefix parsing. A Parser is safe for
// concurrent use; the Documents it creates are not.
type Parser interface {
	// Parse parses the whole input and returns the value of the start rule. If the input does
	// not match, it returns a *value.SyntaxError. If the parse recovered from errors with
	// #recover, it returns both the node and the value.SyntaxErrors.
	Parse(input string, opts ...ParseOption) (*value.Node, error)
	// Recognize decides whether the input matches the grammar without building a tree. It
	// returns the same success, failure and syntax errors as Parse, recovered errors included.
	// Predicates, and the actions they read, run; the run-time errors of other actions and
	// typed conversions are not checked.
	Recognize(input string, opts ...ParseOption) error
	// ParseStream parses input read from r and passes each element of the #stream repetition at
	// the top level of the start rule to emit as soon as it matches. Emitted elements and the
	// input before them are not retained. If emit returns an error, parsing stops and ParseStream
	// returns that error.
	ParseStream(r io.Reader, emit func(*value.Node) error, opts ...ParseOption) error
	// NewDocument creates a Document for text. opts select its fixed unit and the defaults of
	// later parses.
	NewDocument(text string, opts ...ParseOption) (Document, error)
	// OutputSchema exports the types of the values that the parser returns. A parser loaded
	// from a compiled grammar has no grammar metadata and returns ErrSchemaUnavailable.
	OutputSchema(opts ...SchemaOption) (*schema.Output, error)
}

// Document is a text that is edited repeatedly. Parsing after an edit reuses the results of the
// previous parse that the edit did not affect (incremental parsing), as editors need. A Document
// is not safe for concurrent use.
//
// Reused results are shared with the trees returned by earlier parses, and the nodes after an
// edit are moved to their new positions in place: an earlier tree changes when Parse runs again.
// Use value.Node.Clone to keep a tree as it was. The Document of a parser loaded from a compiled
// grammar (vm.Document) also offers versions, batch edits, snapshots and change notifications.
type Document interface {
	// Text returns the current text.
	Text() string
	// Unit returns the position unit of the document, fixed at construction.
	Unit() input.PositionUnit
	// Edit replaces the range [start, end) of the text, in the document's position unit, with
	// text.
	Edit(start, end int, text string) error
	// Parse parses the current text. opts apply once over the construction defaults and may
	// change the context, limits, depth and trace, but not the unit.
	Parse(opts ...ParseOption) (*value.Node, error)
	// Stats returns the evaluation and reuse counts of the last Parse.
	Stats() ParseStats
	// CacheStats inspects the retained incremental history without changing it.
	CacheStats() CacheStats
	// ResetHistory discards the incremental reuse history while keeping the text, the
	// construction options, the version and the statistics of the last parse.
	ResetHistory()
}

// ErrSchemaUnavailable reports that the grammar metadata a schema is exported from is
// unavailable. In particular, a parser loaded from a compiled grammar cannot export a schema.
var ErrSchemaUnavailable = errors.New("output schema unavailable")

// SchemaOptions are the settings of one output schema export, which SchemaOption functions set.
type SchemaOptions struct {
	// AllRules includes the rules the start rule does not reach and every declared type.
	AllRules bool
}

// SchemaOption configures one output schema export.
type SchemaOption func(*SchemaOptions)

// WithAllSchemaRules includes unreachable rules and declared types in the schema. By default,
// the export starts at the parser's start rule.
func WithAllSchemaRules() SchemaOption {
	return func(o *SchemaOptions) { o.AllRules = true }
}
