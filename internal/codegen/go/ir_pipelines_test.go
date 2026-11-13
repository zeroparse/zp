package gogen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestEveryFlowFunctionRunsTheIRPipelines generates parsers whose functions take every route from
// Flow IR (flat and framed rules, lines of an Operators Rule, recognizer rules, typed left-recursion
// bodies, the stream body) in each table, and checks from the dumps of "flow" and "exec" that every function printed from Exec IR
// went through the Flow pipeline and the Exec pipeline once per lowering.
func TestEveryFlowFunctionRunsTheIRPipelines(t *testing.T) {
	t.Parallel()
	read := func(name string) string {
		src, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "grammars", name, name+".zp"))
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
	base := codegen.GenOptions{Package: "x", Module: "example.com/x", Lib: codegen.LibImport, PrintAfter: "flow,exec"}
	for _, c := range []struct {
		name, grammar string
		set           func(*codegen.GenOptions)
		tables        []string
	}{
		{"node+recognizer", "cel", func(o *codegen.GenOptions) { o.Recognize = true }, []string{"rules", "recRules"}},
		{"typed", "cel", func(o *codegen.GenOptions) { o.Types = true }, []string{"rules", "typed"}},
		{"typed-only", "cel", func(o *codegen.GenOptions) { o.Types, o.TypedOnly = true, true }, []string{"typed"}},
		{"controls+observability", "cel", func(o *codegen.GenOptions) { o.Recognize, o.LimitControls, o.Observability = true, true, true }, []string{"rules", "recRules"}},
		{"typed left recursion", "", func(o *codegen.GenOptions) { o.Types, o.Recognize, o.Start = true, true, "main" }, []string{"rules", "recRules", "typed"}},
		{"stream", "csv", func(o *codegen.GenOptions) { o.Stream = true }, []string{"rules"}},
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
