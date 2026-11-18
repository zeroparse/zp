package vm

import (
	"testing"

	"zeroparse.dev/zp/internal/backend/vmlower"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/testing/compiletest"
)

// compileModule compiles src with source information to a module.
func compileModule(t *testing.T, src string) *bytecode.Module {
	t.Helper()
	m, err := vmlower.Compile(compiletest.Program(t, src, compile.Options{SourceInfo: true}))
	if err != nil {
		t.Fatal(err)
	}
	return m
}
