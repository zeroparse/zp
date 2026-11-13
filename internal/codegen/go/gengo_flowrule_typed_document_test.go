package gogen_test

import (
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestGenerateDirectTypedDocument(t *testing.T) {
	t.Parallel()
	gentest.DirectTypedDocument(t, codegen.GenOptions{})
}
