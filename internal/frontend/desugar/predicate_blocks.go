package desugar

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

func predicateBlock(e *ast.Predicate, block *ast.Block, control *cancel.Control) ast.Expr {
	var poll func()
	if control != nil {
		poll = control.Poll
	}
	control.Check()
	if len(block.Statements) == 1 {
		copy := *e
		copy.Term = astfacts.UngroupTermWithPoll(block.Statements[0], poll)
		return &copy
	}
	items := make([]ast.Expr, len(block.Statements))
	for i, statement := range block.Statements {
		control.Poll()
		items[i] = &ast.Predicate{
			SourceInfo: *statement.(ast.SourceNode).Source(),
			Term:       astfacts.UngroupTermWithPoll(statement, poll),
		}
	}
	return &ast.Seq{
		SourceInfo: e.SourceInfo,
		Items:      items,
	}
}

// expandPredicateBlocks splices the predicates of every multi-statement predicate block into the
// list, and reports whether any block was expanded. normalized[i] is the normalized form of
// source[i], and a normalized block is the sequence of its predicates. It returns normalized
// itself when no block has more than one statement.
func expandPredicateBlocks(source, normalized []ast.Expr, control *cancel.Control) ([]ast.Expr, bool) {
	expanded := 0
	for _, item := range source {
		control.Poll()
		if count := predicateStatementCount(item); count > 1 {
			expanded += count - 1
		}
	}
	if expanded == 0 {
		return normalized, false
	}
	control.Check()
	items := make([]ast.Expr, 0, len(normalized)+expanded)
	for i, item := range normalized {
		control.Poll()
		if predicateStatementCount(source[i]) > 1 {
			items = appendNormalizedPredicates(items, item.(*ast.Seq).Items, control)
		} else {
			items = append(items, item)
		}
	}
	return items, true
}

func predicateStatementCount(expr ast.Expr) int {
	predicate, ok := expr.(*ast.Predicate)
	if !ok {
		return 0
	}
	block, ok := predicate.Term.(*ast.Block)
	if !ok {
		return 0
	}
	return len(block.Statements)
}
