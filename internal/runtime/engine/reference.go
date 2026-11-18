package engine

import (
	"io"

	"zeroparse.dev/zp/pkg/runtime"
	"zeroparse.dev/zp/pkg/runtime/input"
	"zeroparse.dev/zp/pkg/runtime/schema"
	"zeroparse.dev/zp/pkg/runtime/value"
)

// Reference returns the public reference parser of p: p on the closure backend, with the options
// of package runtime.
func Reference(p *Parser) runtime.Parser { return reference{p.OnBackend(Closure)} }

// reference is a runtime.Parser that runs p.
type reference struct{ p *Parser }

func (r reference) Parse(input string, opts ...runtime.ParseOption) (*value.Node, error) {
	return r.p.Parse(input, Use(opts...))
}

func (r reference) Recognize(input string, opts ...runtime.ParseOption) error {
	return r.p.Recognize(input, Use(opts...))
}

func (r reference) ParseStream(rd io.Reader, emit func(*value.Node) error, opts ...runtime.ParseOption) error {
	return r.p.ParseStream(rd, emit, Use(opts...))
}

func (r reference) NewDocument(text string, opts ...runtime.ParseOption) (runtime.Document, error) {
	d, err := r.p.NewDocument(text, Use(opts...))
	if err != nil {
		return nil, err
	}
	return referenceDocument{d}, nil
}

func (r reference) OutputSchema(opts ...runtime.SchemaOption) (*schema.Output, error) {
	return r.p.OutputSchema(opts...)
}

// referenceDocument is a runtime.Document that runs d.
type referenceDocument struct{ d *Document }

func (d referenceDocument) Text() string                           { return d.d.Text() }
func (d referenceDocument) Unit() input.PositionUnit               { return d.d.Unit() }
func (d referenceDocument) Edit(start, end int, text string) error { return d.d.Edit(start, end, text) }
func (d referenceDocument) Stats() runtime.ParseStats              { return d.d.Stats() }
func (d referenceDocument) CacheStats() runtime.CacheStats         { return d.d.CacheStats() }
func (d referenceDocument) ResetHistory()                          { d.d.ResetHistory() }
func (d referenceDocument) Parse(opts ...runtime.ParseOption) (*value.Node, error) {
	return d.d.Parse(Use(opts...))
}
