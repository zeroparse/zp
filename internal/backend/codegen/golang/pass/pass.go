// Package pass holds the Go backend's Exec passes: the forms of a lowered function that the Go printer
// prints, made after the Exec passes that every backend runs. The Go printer prints each operation of
// the Exec stage as it is, so the pipeline has no passes yet.
package pass

import (
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/middleend/execlower"
	"zeroparse.dev/zp/internal/pass"
)

// Pipeline is the pipeline of the Go Exec passes. The unit is a lowered function with its module.
var Pipeline = &pass.Pipeline[*execlower.Lowered, *codepass.Env]{
	Analyses: map[pass.Analysis]func(*execlower.Lowered){},
	Verify:   func(l *execlower.Lowered) error { return code.Verify(l.Module) },
}
