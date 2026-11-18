package runtime

import (
	"context"
	"slices"

	"zeroparse.dev/zp/pkg/runtime/input"
	"zeroparse.dev/zp/pkg/runtime/instrument"
	"zeroparse.dev/zp/pkg/runtime/value"
)

// ParseOptions holds the settings of one parse, which ParseOption functions set. Parsers read it
// after applying the options in order; programs set it through the options.
type ParseOptions struct {
	// Context cooperatively interrupts this invocation. Nil means no cancellation.
	Context context.Context
	// Limits bound the cost of the parse, the length of its input and the nesting of its
	// recursive rule calls.
	Limits Limits
	// MissingTokens bounds attempted #missing insertions. Zero disables insertion.
	MissingTokens int
	// Expectations records what a syntax error expects (see WithExpectations).
	Expectations bool
	// Unit is the position unit. Matching is done per code point regardless of the unit.
	Unit input.PositionUnit
	// Traces receive an event at the start and at the end of every rule call, each function
	// every event, in order. A parser checks for interruption between the functions.
	Traces []func(instrument.TraceEvent)
	// TraceSelection, if not nil, limits the events that the Traces receive. Options replace it
	// instead of changing it, so a Document's defaults can share theirs with each parse.
	TraceSelection *instrument.TraceSelection
	// Profiles record the rule calls of the parse; each records an event before the Traces
	// receive it.
	Profiles []*instrument.Profile
	// DiagnosticCallback receives committed recoveries during stream parsing, immediately
	// before the corresponding element.
	DiagnosticCallback func(*value.SyntaxError) error
	// DiagnosticLimit retains only the latest committed stream recoveries when positive.
	// DiagnosticLimitSet records that a limit was given, an explicit zero included.
	DiagnosticLimit    int
	DiagnosticLimitSet bool
}

// ParseOption configures one parse, or the defaults of a Document.
type ParseOption func(*ParseOptions)

// WithContext selects invocation-local cooperative cancellation. Nil disables it.
func WithContext(ctx context.Context) ParseOption {
	return func(o *ParseOptions) { o.Context = ctx }
}

// WithLimits sets the invocation-local limits field by field: a nonzero field overrides the value
// of an earlier WithLimits, including the construction defaults of a Document, and a zero field
// leaves it unchanged. A field that no option sets takes its default; Unlimited disables a limit.
func WithLimits(l Limits) ParseOption {
	return func(o *ParseOptions) { o.Limits = o.Limits.merge(l) }
}

// merge returns l with each nonzero field of o in place of its own.
func (l Limits) merge(o Limits) Limits {
	pick := func(v, def uint64) uint64 {
		if v != 0 {
			return v
		}
		return def
	}
	return Limits{Compute: pick(o.Compute, l.Compute), Memory: pick(o.Memory, l.Memory), Input: pick(o.Input, l.Input), Depth: pick(o.Depth, l.Depth)}
}

// WithMissingTokens bounds attempted #missing literal insertions in one parse. Zero disables
// insertion; negative limits are invalid. Only whole-input parsing and recognition on the
// reference parser support insertion; positive limits are invalid with prefix parsing, even
// without #missing in the grammar. Options resolve in order.
func WithMissingTokens(limit int) ParseOption {
	return func(o *ParseOptions) { o.MissingTokens = limit }
}

// WithExpectations records what a syntax error expects. A syntax error then lists the
// expectations at the farthest failure and carries the messages of @error. Without it the error
// reports the farthest failure position, an empty Expected list and no Messages, and parsing does
// not record expectations. A Document fixes it at construction.
func WithExpectations() ParseOption {
	return func(o *ParseOptions) { o.Expectations = true }
}

// WithUnit selects the position unit. Matching always proceeds by code point, whatever the unit.
func WithUnit(u input.PositionUnit) ParseOption {
	return func(o *ParseOptions) { o.Unit = u }
}

// WithTrace calls f at the start and at the end of every rule call of the parse, in order, so that
// the events nest like the calls (the depth of an event is in TraceEvent.Depth). It reports the
// calls the parser makes: a call skipped because the next character cannot start the rule is not
// reported. Tracing does not change the result, but it makes the parse several times slower. The
// event's methods (LineCol, Text, Failure) read the parser's state, so call them from f; its
// fields can be kept. With several WithTrace options, each function receives every event, in the
// order of the options. A nil f is ignored. WithTraceFilter and WithTraceMaxDepth select the
// events the functions receive. Generated Go parsers have options of the same names and meaning.
func WithTrace(f func(instrument.TraceEvent)) ParseOption {
	return func(o *ParseOptions) {
		if f != nil {
			// Clip: the trace functions of a Document's defaults are shared by its parses.
			o.Traces = append(slices.Clip(o.Traces), f)
		}
	}
}

// WithTraceFilter limits the events that the trace functions receive to the calls of the named
// rules and the calls nested in them. A name that is not a rule matches no call; with no names,
// every call is selected. The last WithTraceFilter of a parse applies. It does not change the
// parse or what WithProfile records.
func WithTraceFilter(rules ...string) ParseOption {
	names := slices.Clone(rules)
	return func(o *ParseOptions) { o.traceSelection().Rules = names }
}

// WithTraceMaxDepth limits the events that the trace functions receive to the calls at most n
// deep, counting the start rule as 1. Zero or a negative n sets no limit. The last
// WithTraceMaxDepth of a parse applies. It does not change the parse or what WithProfile records.
func WithTraceMaxDepth(n int) ParseOption {
	return func(o *ParseOptions) { o.traceSelection().MaxDepth = n }
}

// traceSelection gives o a copy of its trace selection to change and returns it: the selection
// of a Document's defaults is shared with each of its parses.
func (o *ParseOptions) traceSelection() *instrument.TraceSelection {
	s := new(instrument.TraceSelection)
	if o.TraceSelection != nil {
		*s = *o.TraceSelection
	}
	o.TraceSelection = s
	return s
}

// WithProfile adds the cost of the parse to prof. It traces the parse (see WithTrace), so the parse
// is several times slower, and the times in prof are meaningful relative to each other only; they
// do not include the time of the functions given WithTrace. prof records every call, whatever
// WithTraceFilter and WithTraceMaxDepth select. With several WithProfile options, each profile
// records the parse. A nil prof is ignored.
func WithProfile(prof *instrument.Profile) ParseOption {
	return func(o *ParseOptions) {
		if prof != nil {
			// Clip: the profiles of a Document's defaults are shared by its parses.
			o.Profiles = append(slices.Clip(o.Profiles), prof)
		}
	}
}

// WithDiagnosticCallback reports each committed #recover error during stream parsing,
// immediately before the corresponding stream element. Speculative recoveries are not reported.
// The callback receives an independent copy; returning an error stops parsing and suppresses that
// element. A nil callback disables delivery. Non-nil callbacks are invalid with whole-input
// parsing, prefix parsing and Documents. With or without this option, recovered errors remain
// available at stream completion.
func WithDiagnosticCallback(f func(*value.SyntaxError) error) ParseOption {
	return func(o *ParseOptions) { o.DiagnosticCallback = f }
}

// WithDiagnosticLimit retains at most n committed stream recoveries, keeping the latest errors.
// Zero retains every error, as in an unconfigured stream parse. Negative values are invalid. This
// option is valid only with stream parsing, including when n is zero. When errors are dropped,
// the terminal error is *value.TruncatedSyntaxErrors.
func WithDiagnosticLimit(n int) ParseOption {
	return func(o *ParseOptions) {
		o.DiagnosticLimit = n
		o.DiagnosticLimitSet = true
	}
}
