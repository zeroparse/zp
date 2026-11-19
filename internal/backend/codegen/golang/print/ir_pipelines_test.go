package print_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// TestEveryFlowFunctionRunsTheIRPipelines generates parsers whose functions take every route from
// Flow IR (flat and framed rules, lines of an Operators Rule, recognizer rules, typed left-recursion
// bodies, the stream body) in each table, and checks from the dumps of "flowlower" and "execlower" that every function printed from Exec IR
// went through the Flow pipeline and the Exec pipeline once per lowering.
func TestEveryFlowFunctionRunsTheIRPipelines(t *testing.T) {
	t.Parallel()
	read := func(name string) string {
		src, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "testdata", "grammars", name, name+".zp"))
		if err != nil {
			t.Fatal(err)
		}
		return string(src)
	}
	lr := ""
	for _, c := range codegentest.TypedLRBodyCases {
		if !strings.HasSuffix(c.Name, "_fallback") {
			lr = c.Src
			break
		}
	}
	base := common.GenOptions{Package: "x", Module: "example.com/x", Lib: common.LibImport, PrintAfter: "flowlower,execlower"}
	for _, c := range []struct {
		name, grammar string
		set           func(*common.GenOptions)
		tables        []string
	}{
		{"node+recognizer", "cel", func(o *common.GenOptions) { o.Recognize = true }, []string{"rules", "recRules"}},
		{"typed", "cel", func(o *common.GenOptions) { o.Types = true }, []string{"rules", "typed"}},
		{"typed-only", "cel", func(o *common.GenOptions) { o.Types, o.TypedOnly = true, true }, []string{"typed"}},
		{"controls+observability", "cel", func(o *common.GenOptions) { o.Recognize, o.LimitControls, o.Observability = true, true, true }, []string{"rules", "recRules"}},
		{"typed left recursion", "", func(o *common.GenOptions) { o.Types, o.Recognize, o.Start = true, true, "main" }, []string{"rules", "recRules", "typed"}},
		{"stream", "csv", func(o *common.GenOptions) { o.Stream = true }, []string{"rules"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			src := lr
			if c.grammar != "" {
				src = read(c.grammar)
			}
			g, err := parseTest(src)
			if err != nil {
				t.Fatal(err)
			}
			var dump bytes.Buffer
			o := base
			o.PrintTo = &dump
			c.set(&o)
			if _, err := codegentest.Generate(t, g, o); err != nil {
				t.Fatal(err)
			}
			codegentest.CheckIRPipelines(t, dump.String(), c.tables...)
		})
	}
}
