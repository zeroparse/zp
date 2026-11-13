package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestMemberAccessOnJoinGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testMemberAccessOnJoinGeneratedGoParity)
}

func testMemberAccessOnJoinGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, append(codegentest.JoinMemberCases(), codegentest.TypedJoinMemberCases()...), codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.JoinMemberCases(), codegen.GenOptions{ConvertTypes: true}, 0)
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.TypedJoinMemberCases(), codegen.GenOptions{ConvertTypes: true}, 0)
}
