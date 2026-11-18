package print_test

import (
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

func TestGenerateDirectTypedDocument(t *testing.T) {
	t.Parallel()
	codegentest.DirectTypedDocument(t, common.GenOptions{})
}
