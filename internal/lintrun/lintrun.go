// Package lintrun lets the language server lint with a front-end result it already has, which
// the public package lint does not offer: lint registers its entries here when it is imported.
//
// The findings and rewrites are values of package lint (a []lint.Finding and a []lint.Rewrite);
// they cross this package as any because lint, which defines them, cannot be imported here.
package lintrun

import (
	"context"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/frontend"
)

// Run lints the grammar g, whose front-end result is res, from the start rule start with the
// default configuration, and returns a []lint.Finding. g must be the grammar res was built from
// and start one of its start rules.
var Run func(ctx context.Context, res *frontend.Result, g *grammar.Grammar, start string) (any, error)

// Rewrites returns the []lint.Rewrite that prove the findings (a []lint.Finding of Run on the same
// g and start), as lint.Rewrites does, with the front-end result res of g.
var Rewrites func(ctx context.Context, res *frontend.Result, g *grammar.Grammar, start string, findings any) (any, error)
