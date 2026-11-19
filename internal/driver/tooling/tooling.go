// Package tooling is what the driver gives the interactive tools, the grammar language server
// above all: the operations on a grammar source that is being edited (tokens, partial parses,
// formatting), its compilation and checked front-end result, the lint findings and their
// rewrites, and the facts and lowerings that editor features read. The language server imports
// the compiler only through this package and the grammar AST, so the layer graph keeps it above
// the driver; the names here are those of the packages that implement them.
package tooling

import (
	"context"

	"zeroparse.dev/zp/internal/analysis/lint"
	"zeroparse.dev/zp/internal/driver/entry"
	"zeroparse.dev/zp/internal/frontend"
	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/runtime/engine"
	syntaxfmt "zeroparse.dev/zp/internal/syntax/fmt"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Sources: parsing, tokens and names.

// A ParseError is a syntax error of a grammar source, and a ParseErrorList the errors of one parse.
type (
	ParseError     = parser.Error
	ParseErrorList = parser.ErrorList
)

// A Token is a lexical token of a grammar source, of one TokenKind.
type (
	Token     = parser.Token
	TokenKind = parser.TokenKind
)

// The kinds of tokens.
const (
	TokenIdent     = parser.TokenIdent
	TokenInt       = parser.TokenInt
	TokenString    = parser.TokenString
	TokenCharClass = parser.TokenCharClass
	TokenCapture   = parser.TokenCapture
	TokenIndex     = parser.TokenIndex
	TokenPunct     = parser.TokenPunct
	TokenComment   = parser.TokenComment
)

// Parse parses a grammar source. A rejected source returns a ParseErrorList.
func Parse(src string) (*ast.Grammar, error) { return parser.Parse(src) }

// ParsePartial parses a grammar source and returns the declarations it could parse with the errors
// of the source.
func ParsePartial(src string) (*ast.Grammar, ParseErrorList) { return parser.ParsePartial(src) }

// ParsePartialContext is ParsePartial with cancellation; a cancelled call returns no grammar or
// errors.
func ParsePartialContext(ctx context.Context, src string) (*ast.Grammar, ParseErrorList, error) {
	return parser.ParsePartialContext(ctx, src)
}

// TokenizeContext returns the tokens of a grammar source, comments included, with cancellation; a
// cancelled call returns no tokens.
func TokenizeContext(ctx context.Context, src string) ([]Token, error) {
	return parser.TokenizeContext(ctx, src)
}

// IsIdentifier reports whether s is an ASCII identifier of the grammar language; names beginning
// with "_" are not identifiers.
func IsIdentifier(s string) bool { return parser.IsIdentifier(s) }

// IsRuleName reports whether s can name a rule: an identifier, or the name of a hidden rule.
func IsRuleName(s string) bool { return parser.IsRuleName(s) }

// Formatting.

// Format returns the canonical source of g.
func Format(g *ast.Grammar) string { return syntaxfmt.Format(g) }

// FormatContext is Format with cancellation.
func FormatContext(ctx context.Context, g *ast.Grammar) (string, error) {
	return syntaxfmt.FormatContext(ctx, g)
}

// FormatType returns the canonical source of a type expression.
func FormatType(t ast.TypeExpr) string { return syntaxfmt.FormatType(t) }

// FormatTypeContext is FormatType with cancellation.
func FormatTypeContext(ctx context.Context, t ast.TypeExpr) (string, error) {
	return syntaxfmt.FormatTypeContext(ctx, t)
}

// Compilation.

// A Program is a compiled grammar; CompileOptions are the options of its compilation.
type (
	Program        = engine.Program
	CompileOptions = entry.Options
)

// CompileContext compiles g, with cancellation.
func CompileContext(ctx context.Context, g *ast.Grammar, opts CompileOptions) (*Program, error) {
	return entry.CompileContext(ctx, g, opts)
}

// A FrontResult is the checked front-end result of a compiled grammar: its semantic model and Core
// IR.
type FrontResult = frontend.Result

// FrontResultOf returns the front-end result of p from the checked program that p keeps, without
// compiling the grammar again.
func FrontResultOf(p *Program) (*FrontResult, error) {
	return frontend.FromProgram(entry.Checked(p), nil)
}

// Lint.

// A LintFinding is a finding of a lint check, of one LintSeverity; a LintRewrite is the edit of the
// source that a finding proves. A LintCheck describes a check.
type (
	LintFinding  = lint.Finding
	LintSeverity = lint.Severity
	LintRewrite  = lint.Rewrite
	LintCheck    = lint.Check
)

// The severities of lint findings.
const (
	LintWarning = lint.Warning
	LintError   = lint.Error
)

// The names of the lint checks that the language server treats on their own.
const (
	CheckCharClass        = lint.CheckCharClass
	CheckDirective        = lint.CheckDirective
	CheckUnreachableRule  = lint.CheckUnreachableRule
	CheckUselessLookahead = lint.CheckUselessLookahead
)

// LintChecks returns the lint checks.
func LintChecks() []LintCheck { return lint.Checks() }

// RunLint lints g, whose front-end result is res, from the start rule start with the default
// configuration. g must be the grammar res was built from and start one of its start rules.
func RunLint(ctx context.Context, res *FrontResult, g *ast.Grammar, start string) ([]LintFinding, error) {
	return lint.RunChecked(ctx, res, g, start)
}

// LintRewritesOf returns the rewrites that the findings of RunLint on the same g prove, or the zero
// LintRewrite for a finding that proves none.
func LintRewritesOf(ctx context.Context, res *FrontResult, g *ast.Grammar, findings []LintFinding) ([]LintRewrite, error) {
	return lint.RewritesChecked(ctx, res, g, findings)
}

// Facts and lowerings.

// ExpressionArgsWithPoll returns the expression arguments of an attribute in order, calling poll
// as it goes so that an analysis can stop it; a panic of poll passes unchanged.
func ExpressionArgsWithPoll(a *ast.Attribute, poll func()) []ast.Expr {
	return astfacts.ExpressionArgsWithPoll(a, poll)
}

// InlineActionNames maps the hidden rules that LowerInlineActions generates to the inline actions
// of the grammar as written.
type InlineActionNames = desugar.InlineActionNames

// LowerInlineActions returns g with every inline action replaced by a reference to the hidden rule
// that the compiler runs in its place; g is not changed.
func LowerInlineActions(g *ast.Grammar) (*ast.Grammar, *InlineActionNames, error) {
	return desugar.LowerInlineActions(g)
}
