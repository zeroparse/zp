package gogen_test

import (
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGenerateDirectTypedDocument(t *testing.T) {
	t.Parallel()
	codegentest.DirectTypedDocument(t, codegen.GenOptions{})
}
