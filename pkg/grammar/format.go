package grammar

import (
	"context"

	"zeroparse.dev/zp/internal/syntax/fmt"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Format writes g as grammar syntax.
//
// Comments and line breaks recorded in the AST (see ast.LineBreak) are kept, so formatting parsed
// source preserves its comments. Struct field types and trailing comments on consecutive lines
// are aligned. A grammar without such layout information (for example, one decoded from JSON) is
// printed with a blank line between definitions and one item per line in blocks.
func Format(g *ast.Grammar) string { return fmt.Format(g) }

// FormatContext is Format with cooperative cancellation. Cancellation returns an empty string
// and ctx.Err(), never partial source. The caller must not mutate g during formatting.
func FormatContext(ctx context.Context, g *ast.Grammar) (string, error) {
	return fmt.FormatContext(ctx, g)
}

// FormatExpr writes the parser expression e as grammar syntax.
func FormatExpr(e ast.Expr) string { return fmt.FormatExpr(e) }

// FormatExprContext is FormatExpr with cooperative cancellation, as for FormatContext.
func FormatExprContext(ctx context.Context, e ast.Expr) (string, error) {
	return fmt.FormatExprContext(ctx, e)
}

// FormatActionExpr writes the action expression t of an action or a predicate as grammar syntax.
func FormatActionExpr(t ast.ActionExpr) string { return fmt.FormatActionExpr(t) }

// FormatActionExprContext is FormatActionExpr with cooperative cancellation, as for FormatContext.
func FormatActionExprContext(ctx context.Context, t ast.ActionExpr) (string, error) {
	return fmt.FormatActionExprContext(ctx, t)
}

// FormatType writes the type t as grammar syntax.
func FormatType(t ast.TypeExpr) string { return fmt.FormatType(t) }

// FormatTypeContext is FormatType with cooperative cancellation, as for FormatContext.
func FormatTypeContext(ctx context.Context, t ast.TypeExpr) (string, error) {
	return fmt.FormatTypeContext(ctx, t)
}

// FormatSyntax writes the parsing syntax of g without modifying it. It omits type declarations,
// result annotations, Value Actions, captures, metadata attributes and recovery wrappers, and
// keeps Predicate Actions, matching operators and other pragmas. It is a documentation view, not
// an equivalent grammar: a predicate may depend on an omitted capture, action or type.
func FormatSyntax(g *ast.Grammar) string { return fmt.FormatSyntax(g) }
