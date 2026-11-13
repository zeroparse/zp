package core

import (
	"fmt"
	"math"

	"zeroparse.dev/zp/runtime/parse"
)

// Keep only small recovery scratch slices across stream elements. The
// diagnostic collector, not this scratch, owns committed errors.
const streamRecoveryScratchMax = 16

// ValidateStreamDiagnostics checks the diagnostic options of o for a stream parse (stream) or
// another parse, which takes none of them.
func ValidateStreamDiagnostics(o ParseOptions, stream bool) error {
	if o.DiagnosticLimit < 0 {
		return fmt.Errorf("diagnostic limit must be nonnegative")
	}
	if !stream && (o.DiagnosticCallback != nil || o.DiagnosticLimitSet || o.DiagnosticLimit != 0) {
		return fmt.Errorf("diagnostic options are only supported for stream parsing")
	}
	return nil
}

// StreamDiagnosticTail is populated only at irreversible stream boundaries.
// The ring avoids shifting a large tail for each committed recovery.
type StreamDiagnosticTail struct {
	Limit           int
	Errors          parse.SyntaxErrors
	head            int
	Total           uint64
	Dropped         uint64
	truncated       bool
	countsSaturated bool
	// bytes counts the errors in Errors (syntaxErrorSize), for the memory limit.
	bytes uint64
}

func (t *StreamDiagnosticTail) Add(e *parse.SyntaxError) {
	t.bytes += syntaxErrorSize(e)
	if t.Total == math.MaxUint64 {
		t.countsSaturated = true
	} else {
		t.Total++
	}
	if len(t.Errors) < t.Limit {
		t.Errors = append(t.Errors, e)
		return
	}
	t.truncated = true
	if t.Dropped == math.MaxUint64 {
		t.countsSaturated = true
	} else {
		t.Dropped++
	}
	t.bytes -= syntaxErrorSize(t.Errors[t.head])
	t.Errors[t.head] = e
	t.head++
	if t.head == t.Limit {
		t.head = 0
	}
}

func (t *StreamDiagnosticTail) Result(p *Parser) error {
	p.Control.Check()
	if len(t.Errors) == 0 {
		return nil
	}
	if t.head == 0 {
		if !t.truncated {
			return t.Errors
		}
		return &parse.TruncatedSyntaxErrors{Errors: t.Errors, Total: t.Total, Dropped: t.Dropped, CountsSaturated: t.countsSaturated}
	}
	ordered := make(parse.SyntaxErrors, 0, len(t.Errors))
	ordered = t.appendLast(p, ordered, len(t.Errors))
	p.Control.Check()
	return &parse.TruncatedSyntaxErrors{Errors: ordered, Total: t.Total, Dropped: t.Dropped, CountsSaturated: t.countsSaturated}
}

// Preview includes recoveries from the successful suffix, which TraceExit
// observes before ParseStreamWith collects them for the terminal result.
func (t *StreamDiagnosticTail) Preview(p *Parser, suffix []*parse.SyntaxError) []*parse.SyntaxError {
	n := min(t.Limit, len(t.Errors)+len(suffix))
	if n == 0 {
		return nil
	}
	p.Control.Check()
	out := make([]*parse.SyntaxError, 0, n)
	if len(suffix) >= n {
		return AppendSpend(p, out, suffix[len(suffix)-n:])
	}
	out = t.appendLast(p, out, n-len(suffix))
	return AppendSpend(p, out, suffix)
}

// appendLast copies the last n retained errors in occurrence order. Each
// contiguous segment is charged and copied in bounded chunks by AppendSpend.
func (t *StreamDiagnosticTail) appendLast(p *Parser, dst parse.SyntaxErrors, n int) parse.SyntaxErrors {
	if n == 0 {
		return dst
	}
	length := len(t.Errors)
	start := length - n
	index := t.head
	if start >= length-index {
		index = start - (length - index)
	} else {
		index += start
	}
	first := min(n, length-index)
	dst = AppendSpend(p, dst, t.Errors[index:index+first])
	if first < n {
		dst = AppendSpend(p, dst, t.Errors[:n-first])
	}
	return dst
}
