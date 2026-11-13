// Package lint reports likely mistakes in ZeroParse grammars: rules and alternatives that can never
// match, expressions that have no effect, captures that are never read, and grammar shapes that
// defeat incremental parsing.
//
// Config.Run checks a grammar with the front end before it lints it. The checks work on the
// grammar AST, with the rule facts of the front end. Each check reports only what it can prove from
// the grammar (the exceptions are the performance hints, which have the severity Hint); see
// docs/design/records/018-grammar-linting.md for the reasoning behind each one.
package lint

import (
	"context"

	"zeroparse.dev/zp/grammar"
	impl "zeroparse.dev/zp/internal/lint"
)

// Severity tells how likely a finding is to be a mistake. It has the methods String, MarshalText
// and UnmarshalText, which use the names "hint", "warning" and "error".
type Severity = impl.Severity

const (
	// Hint is a suggestion, usually about performance; the grammar is correct as written.
	Hint = impl.Hint
	// Warning is a likely mistake, or an expression that has no effect.
	Warning = impl.Warning
	// Error is a certain mistake: part of the grammar can never take effect.
	Error = impl.Error
)

// Finding is a problem found in a grammar: its position Pos (zero for a grammar without
// positions), Severity, the name Check of the check that reported it, the Rule it is in, if any,
// its Message and a suggested Fix, or "" if there is no single obvious one. Its String method
// formats it for display.
type Finding = impl.Finding

// Check describes a check: its Name, the highest Severity it reports and a Summary.
type Check = impl.Check

// Names of the checks.
const (
	CheckUnreachableRule    = impl.CheckUnreachableRule
	CheckShadowedAlt        = impl.CheckShadowedAlt
	CheckNeverMatches       = impl.CheckNeverMatches
	CheckUselessLookahead   = impl.CheckUselessLookahead
	CheckNullableRepetition = impl.CheckNullableRepetition
	CheckRedundantOptional  = impl.CheckRedundantOptional
	CheckGrowingVariable    = impl.CheckGrowingVariable
	CheckUnusedCapture      = impl.CheckUnusedCapture
	CheckDuplicateCapture   = impl.CheckDuplicateCapture
	CheckCharClass          = impl.CheckCharClass
	CheckRightRecursion     = impl.CheckRightRecursion
	CheckPositions          = impl.CheckPositions
	CheckLongLookahead      = impl.CheckLongLookahead
	CheckSuperLinear        = impl.CheckSuperLinear
	CheckCaseInsensitive    = impl.CheckCaseInsensitive
	CheckDirective          = impl.CheckDirective
)

// Checks returns every check, in the order of the documentation.
func Checks() []Check { return impl.Checks() }

// Config configures a lint run. The zero value runs every check except super-linear: Disable
// holds the names of the checks not to run (see Checks), and Classify requests the classification
// of the cost of the grammar from the start rule, and with it the super-linear check, which
// reports nothing without it.
//
// Its method Run lints the grammar g from its start rule start: it reports rules the start rules
// never use, alternatives and expressions that can never match or have no effect, captures that
// are never read, variables that can grow exponentially, grammar shapes that slow down incremental
// parsing and, when Classify is set, the causes of a cost that can grow faster than the input. The
// findings are sorted by position. Comments of the form "// lint:ignore check reason" in the
// source suppress findings (see the directive syntax in docs/guide/linting.md). An empty start
// selects the default start rule. Run returns an error, and no findings, if g does not compile, if
// start is not a start rule of g, if Disable names an unknown check, or if ctx ends; cancellation
// stops both the compilation and the checks and discards all findings.
type Config = impl.Config

// Rewrite is a source change that fixes a finding: Start and End delimit the source to replace by
// NewText, with End exclusive; for a rule the span is the whole definition including its
// attributes. Its method IsValid reports whether it holds a change.
type Rewrite = impl.Rewrite

// Rewrites returns, for each finding of the lint run of g from start, its Rewrite or the zero
// Rewrite. The findings must come from Config.Run on the same g and start. Rewrites returns an
// error, and no rewrites, if g does not compile or start is not a start rule of g. Cancellation
// discards the result.
func Rewrites(ctx context.Context, g *grammar.Grammar, start string, findings []Finding) ([]Rewrite, error) {
	return impl.Rewrites(ctx, g, start, findings)
}
