package print_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

func TestMemberAccessOnJoinGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testMemberAccessOnJoinGeneratedGoParity)
}

func testMemberAccessOnJoinGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, append(codegentest.JoinMemberCases(), codegentest.TypedJoinMemberCases()...), common.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.JoinMemberCases(), common.GenOptions{ConvertTypes: true}, 0)
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.TypedJoinMemberCases(), common.GenOptions{ConvertTypes: true}, 0)
}
