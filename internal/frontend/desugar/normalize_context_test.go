package desugar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"zeroparse.dev/zp/internal/cancel"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func TestNormalizationContextCancellation(t *testing.T) {
	t.Parallel()
	const size = 2048
	sequence := &ast.Seq{Items: make([]ast.Expr, size)}
	args := make([]any, size)
	keywords := make(map[string]any, size)
	block := &ast.Block{Statements: make([]ast.ActionExpr, size)}
	fields := make([]*ast.Field, size)
	var groupedType ast.TypeExpr = &ast.TypeRef{Name: "Match"}
	var groupedTerm ast.ActionExpr = &ast.IntLit{Value: 1}
	for i := 0; i < size; i++ {
		sequence.Items[i] = &ast.Literal{Value: "a"}
		args[i] = i
		keywords[fmt.Sprintf("K%d", i)] = i
		block.Statements[i] = &ast.BoolLit{Value: true}
		fields[i] = &ast.Field{Name: fmt.Sprintf("Field%d", i), Type: &ast.TypeGroup{Type: &ast.TypeRef{Name: "Match"}}}
		groupedType = &ast.TypeGroup{Type: groupedType}
		groupedTerm = &ast.ActionGroup{Action: groupedTerm}
	}
	fixtures := map[string]*ast.Grammar{}
	for name, expr := range map[string]ast.Expr{
		"sequence":             sequence,
		"positional-arguments": &ast.Attributed{Expr: &ast.Literal{Value: "a"}, Attrs: []*ast.Attribute{{Name: "custom", Args: args}}},
		"keyword-arguments":    &ast.Attributed{Expr: &ast.Literal{Value: "a"}, Attrs: []*ast.Attribute{{Name: "custom", Args: []any{}, KwArgs: keywords}}},
		"predicate-block":      &ast.Predicate{Action: block},
	} {
		fixtures[name] = &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Expr: expr}}}}
	}
	fixtures["type-groups"] = &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Type: groupedType, Expr: &ast.Literal{Value: "a"}}}}}
	fixtures["action-groups"] = &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Expr: &ast.Literal{Value: "a"}, Action: groupedTerm}}}}
	fixtures["struct-fields"] = &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.TypeDef{Name: "T", Type: &ast.StructType{Fields: fields}}}}}
	union := &ast.JoinType{Types: make([]ast.TypeExpr, size)}
	action := &ast.Call{Func: "list", Args: make([]ast.ActionExpr, size)}
	for i := 0; i < size; i++ {
		union.Types[i] = &ast.TypeRef{Name: "Match"}
		action.Args[i] = &ast.BoolLit{Value: true}
	}
	union.Types[0] = &ast.TypeGroup{Type: union.Types[0]}
	action.Args[0] = &ast.ActionGroup{Action: action.Args[0]}
	fixtures["grouped-type-copy"] = &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Type: union, Expr: &ast.Literal{Value: "a"}}}}}
	fixtures["grouped-action-copy"] = &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Expr: &ast.Literal{Value: "a"}, Action: action}}}}
	for name, g := range fixtures {
		for _, eager := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/eager=%v", name, eager), func(t *testing.T) {
				before, err := json.Marshal(g)
				if err != nil {
					t.Fatal(err)
				}
				ctx := pollTestContext(t, 3)
				var result *ast.Grammar
				var diagnostics error
				defer func() {
					failure := recover()
					cancelled, ok := failure.(cancel.Cancellation)
					if !ok || !errors.Is(cancelled.Err, context.Canceled) || ctx.left != 0 {
						t.Fatalf("normalization did not cancel during traversal: %v", failure)
					}
					if result != nil || diagnostics != nil {
						t.Fatal("cancelled normalization published partial output")
					}
					after, err := json.Marshal(g)
					if err != nil || string(after) != string(before) {
						t.Fatalf("cancelled normalization changed source: %v", err)
					}
				}()
				result, diagnostics = NormalizeAttributesControlled(g, eager, &cancel.Control{Ctx: ctx})
				t.Fatal("normalization finished without cancellation")
			})
		}
	}
}

func TestNormalizationCopiesObserveCancellation(t *testing.T) {
	t.Parallel()
	const size = 2048
	values := make([]int, size)
	keywords := make(map[string]any, size)
	attrs := make([]*ast.Attribute, size)
	terms := make([]ast.ActionExpr, size)
	expressions := make([]ast.Expr, size)
	for i := range values {
		values[i] = i
		keywords[fmt.Sprint(i)] = i
		attrs[i] = &ast.Attribute{Name: "custom"}
		terms[i] = &ast.BoolLit{Value: true}
		expressions[i] = &ast.Predicate{Action: terms[i]}
	}
	for _, phase := range []string{"slice", "keywords", "attribute-append", "predicate-flatten"} {
		t.Run(phase, func(t *testing.T) {
			before, err := json.Marshal([]any{values, keywords, attrs, terms, expressions})
			if err != nil {
				t.Fatal(err)
			}
			ctx := pollTestContext(t, 3)
			control := &cancel.Control{Ctx: ctx}
			defer func() {
				failure := recover()
				cancelled, ok := failure.(cancel.Cancellation)
				if !ok || !errors.Is(cancelled.Err, context.Canceled) || ctx.left != 0 {
					t.Fatalf("copy did not cancel: %v", failure)
				}
				after, err := json.Marshal([]any{values, keywords, attrs, terms, expressions})
				if err != nil || string(after) != string(before) {
					t.Fatal("interrupted copy changed source")
				}
			}()
			switch phase {
			case "slice":
				normalizationClone(values, control)
			case "keywords":
				cloneAttributeKeywords(keywords, control)
			case "attribute-append":
				appendNormalizedAttribute(attrs, &ast.Attribute{Name: "other"}, control)
			case "predicate-flatten":
				source := []ast.Expr{&ast.Predicate{Action: &ast.Block{Statements: terms}}}
				normalized := []ast.Expr{&ast.Seq{Items: expressions}}
				expandPredicateBlocks(source, normalized, control)
			}
			t.Fatal("copy finished without cancellation")
		})
	}
}

func TestNormalizationFinalCheckpoint(t *testing.T) {
	t.Parallel()
	g := &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Expr: &ast.Literal{Value: "a"}}}}}
	ctx := pollTestContext(t, 2)
	defer func() {
		cancelled, ok := recover().(cancel.Cancellation)
		if !ok || !errors.Is(cancelled.Err, context.Canceled) || ctx.left != 0 {
			t.Fatal("short normalization ignored final cancellation")
		}
	}()
	NormalizeAttributesControlled(g, false, &cancel.Control{Ctx: ctx})
	t.Fatal("normalization returned a cancelled result")
}

// pollContext cancels at a deterministic poll, without timing or global hooks.
type pollContext struct {
	context.Context
	cancel context.CancelFunc
	left   int
}

func (c *pollContext) Err() error {
	c.left--
	if c.left == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

// pollTestContext returns a context that is cancelled at its polls-th check.
func pollTestContext(t *testing.T, polls int) *pollContext {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &pollContext{Context: ctx, cancel: cancel, left: polls}
}
