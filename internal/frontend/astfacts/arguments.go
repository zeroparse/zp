package astfacts

import (
	"cmp"
	"slices"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

// ExpressionArgsWithPoll preserves Attribute.ExpressionArgs order and fresh
// storage while sharing an analysis invocation's polling counter. This avoids
// starting a separate context operation for every small attribute. A polling
// panic passes unchanged to the enclosing analysis entry point.
func ExpressionArgsWithPoll(a *ast.Attribute, poll func()) []ast.Expr {
	if poll == nil {
		return a.ExpressionArgs()
	}
	var result []ast.Expr
	for _, value := range a.Args {
		poll()
		if e, ok := value.(ast.Expr); ok {
			result = append(result, e)
		}
	}
	keys := make([]string, 0, len(a.KwArgs))
	for key := range a.KwArgs {
		poll()
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int { poll(); return cmp.Compare(a, b) })
	for _, key := range keys {
		poll()
		if e, ok := a.KwArgs[key].(ast.Expr); ok {
			result = append(result, e)
		}
	}
	return result
}
