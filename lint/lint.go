// Package lint reports likely mistakes in ZeroParse grammars: rules and alternatives that can never
// match, expressions that have no effect, captures that are never read, and grammar shapes that
// defeat incremental parsing.
//
// Config.Run checks a grammar with the front end before it lints it. The checks work on the
// grammar AST, with the rule facts of the front end. Each check reports
// only what it can prove from the grammar (the exceptions are the performance hints, which have
// the severity Hint); see docs/design/records/018-grammar-linting.md for the reasoning behind each
// one.
package lint

import (
	"context"

	"zeroparse.dev/zp/grammar"
	impl "zeroparse.dev/zp/internal/lint"
)

// Severity tells how likely a finding is to be a mistake.
type Severity int

const (
	// Hint is a suggestion, usually about performance; the grammar is correct as written.
	Hint Severity = iota + 1
	// Warning is a likely mistake, or an expression that has no effect.
	Warning
	// Error is a certain mistake: part of the grammar can never take effect.
	Error
)

// String returns the name of the severity: hint, warning or error.
func (s Severity) String() string { return impl.Severity(s).String() }

// MarshalText encodes the severity as its name.
func (s Severity) MarshalText() ([]byte, error) { return impl.Severity(s).MarshalText() }

// UnmarshalText decodes the name of a severity.
func (s *Severity) UnmarshalText(text []byte) error {
	v := impl.Severity(*s)
	if err := v.UnmarshalText(text); err != nil {
		return err
	}
	*s = Severity(v)
	return nil
}

// Finding is a problem found in a grammar.
type Finding struct {
	// Pos is where the problem is in the source. It is the zero Pos for a grammar without
	// positions (decoded from JSON, or loaded from a compiled grammar).
	Pos      grammar.Pos
	Severity Severity
	// Check is the name of the check that reported the finding (see Checks).
	Check string
	// Rule is the rule the finding is in, if any.
	Rule    string
	Message string
	// Fix is a suggested change, or "" if there is no single obvious one.
	Fix string
}

// String formats the finding for display: position, severity, message and check.
func (f Finding) String() string { return findingToImpl(f).String() }

// Check describes a check.
type Check struct {
	Name string
	// Severity is the highest severity the check reports.
	Severity Severity
	Summary  string
}

// Names of the checks.
const (
	CheckUnreachableRule    = "unreachable-rule"
	CheckShadowedAlt        = "shadowed-alternative"
	CheckNeverMatches       = "never-matches"
	CheckUselessLookahead   = "useless-lookahead"
	CheckNullableRepetition = "nullable-repetition"
	CheckRedundantOptional  = "redundant-optional"
	CheckGrowingVariable    = "growing-variable"
	CheckUnusedCapture      = "unused-capture"
	CheckDuplicateCapture   = "duplicate-capture"
	CheckCharClass          = "char-class"
	CheckRightRecursion     = "right-recursion"
	CheckPositions          = "positions"
	CheckLongLookahead      = "long-lookahead"
	CheckSuperLinear        = "super-linear"
	CheckCaseInsensitive    = "case-insensitive-literal"
	CheckDirective          = "lint-directive"
)

// Checks returns every check, in the order of the documentation.
func Checks() []Check {
	in := impl.Checks()
	out := make([]Check, len(in))
	for i, c := range in {
		out[i] = Check{Name: c.Name, Severity: Severity(c.Severity), Summary: c.Summary}
	}
	return out
}

// Config configures a lint run. The zero value runs every check except super-linear.
type Config struct {
	// Disable holds the names of the checks not to run (see Checks).
	Disable []string
	// Classify requests the classification of the cost of the grammar from the start rule, and
	// with it the super-linear check, which reports nothing without it.
	Classify bool
}

// Run lints the grammar g from its start rule start: it reports rules the start rules never use,
// alternatives and expressions that can never match or have no effect, captures that are never
// read, variables that can grow exponentially, grammar shapes that slow down incremental parsing
// and, when c.Classify is set, the causes of a cost that can grow faster than the input. The
// findings are sorted by position. unreachable-rule reports the rules that neither start nor
// another @start rule reaches. Comments of the form "// lint:ignore check reason" in the source
// suppress findings (see the directive syntax in docs/guide/linting.md).
//
// An empty start selects the default start rule. Run returns an error, and no findings, if g does
// not compile, if start is not a start rule of g, if c.Disable names an unknown check, or if ctx
// ends; cancellation stops both the compilation and the checks and discards all findings.
func (c Config) Run(ctx context.Context, g *grammar.Grammar, start string) ([]Finding, error) {
	in, err := impl.Config(c).Run(ctx, g, start)
	if err != nil {
		return nil, err
	}
	return findingsFromImpl(in), nil
}

// A Rewrite is a change to the source of a grammar that fixes a finding without changing what the
// grammar matches or the values it builds. Only three findings have one, each when the proof for
// its kind holds, and a tool that applies a Rewrite still checks the result:
//
//   - char-class, for ranges that overlap or repeat: the class is replaced by one with merged
//     ranges. The proof is that both classes contain the same code points, checked at the ends of
//     every range, and that negation is unchanged. A class that also matches punctuation between
//     letters, such as [A-z], has no Rewrite, because the writer must say which characters were meant.
//   - useless-lookahead, for a lookahead that always succeeds: it is removed, but only where it is
//     itself an item of a sequence of at least three items, so that the sequence keeps at least two
//     and stays a tuple. Under a discard "-", a label or any operator it is not removed, because the
//     "-" would move onto the next item and change the values. The proof is that the
//     lookahead matches only a combination of literals, character classes, any-character, groups,
//     sequences, choices, options and repetitions, so it holds no capture, cut, call, predicate,
//     recovery or attribute, and a lookahead consumes nothing and has no value.
//   - unreachable-rule: the rule is removed. The proof is that no start rule reaches it, directly
//     or through an inline action, so nothing calls it and no parse starts at it.
//
// Start and End delimit the source to replace by NewText, with End exclusive. For a rule the span is
// the whole definition including its attributes.
type Rewrite struct {
	Start, End grammar.Pos
	NewText    string
}

// IsValid reports whether the Rewrite holds a change.
func (r Rewrite) IsValid() bool { return impl.Rewrite(r).IsValid() }

// Rewrites returns, for each finding of the lint run of g from start, its Rewrite or the zero
// Rewrite. The findings must come from Config.Run on the same g and start. Rewrites returns an
// error, and no rewrites, if g does not compile or start is not a start rule of g. Cancellation
// discards the result.
func Rewrites(ctx context.Context, g *grammar.Grammar, start string, findings []Finding) ([]Rewrite, error) {
	in := make([]impl.Finding, len(findings))
	for i, f := range findings {
		in[i] = findingToImpl(f)
	}
	out, err := impl.Rewrites(ctx, g, start, in)
	if err != nil {
		return nil, err
	}
	res := make([]Rewrite, len(out))
	for i, r := range out {
		res[i] = Rewrite(r)
	}
	return res, nil
}

// findingToImpl and findingsFromImpl convert at the boundary to the implementation, which the
// language server uses directly.
func findingToImpl(f Finding) impl.Finding {
	return impl.Finding{Pos: f.Pos, Severity: impl.Severity(f.Severity), Check: f.Check, Rule: f.Rule, Message: f.Message, Fix: f.Fix}
}

func findingsFromImpl(in []impl.Finding) []Finding {
	if in == nil {
		return nil
	}
	out := make([]Finding, len(in))
	for i, f := range in {
		out[i] = Finding{Pos: f.Pos, Severity: Severity(f.Severity), Check: f.Check, Rule: f.Rule, Message: f.Message, Fix: f.Fix}
	}
	return out
}
