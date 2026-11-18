package lint

import (
	"context"
	"strings"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend"
	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/syntax/fmt"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// checked runs the front end on g without cancellation. Many test grammars are one rule without
// @start; the front end checks a copy whose first rule starts, which has the same rule names and
// so the same rule facts for the analysis.
func checked(g *ast.Grammar) *frontend.Result {
	res, err := frontend.Run(g, compile.Options{}, nil)
	if err == nil {
		return res
	}
	text := fmt.Format(g)
	if i := strings.Index(text, "rule "); i >= 0 && !strings.Contains(text, "@start") {
		text = text[:i] + "@start\n" + text[i:]
	}
	copied, perr := parser.Parse(text)
	if perr == nil {
		if res, err2 := frontend.Run(copied, compile.Options{}, nil); err2 == nil {
			return res
		}
	}
	panic(err)
}

// runUnchecked lints g from start without a cancellation control.
func runUnchecked(g *ast.Grammar, start string, opts options) []Finding {
	return run(checked(g), g, start, opts, nil)
}

// testRunContext lints g from start, with cancellation by ctx for the lint and not the compilation.
func testRunContext(ctx context.Context, g *ast.Grammar, start string, opts options) ([]Finding, error) {
	return runContext(ctx, checked(g), g, start, opts)
}

func newAnalysis(g *ast.Grammar) *analysis { return testNewAnalysisControlled(g, nil) }

func testNewAnalysisControlled(g *ast.Grammar, control *lintControl) *analysis {
	return newAnalysisControlled(g, control, tablesOf(checked(g)))
}
