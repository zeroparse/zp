package compiler_test

import (
	"bytes"
	"strings"
	"testing"

	"zeroparse.dev/zp/pkg/compiler"
	"zeroparse.dev/zp/pkg/grammar"
)

// WithOptimize and the PrintAfter fields of GoConfig and TSConfig are the flags -optimize, -no-opt
// and -print-after of zp gen: a name that is turned off changes the code, an unknown name or profile is
// an error that lists the valid ones, and a request to print needs a writer.
func TestOptimizationFields(t *testing.T) {
	src := grammar.FromString("opt.zp", `@start
rule main = (word / num / -" ")* $$
rule word = "if" / "else" / [a-z]+
rule num = [0-9]+
`)
	type gens struct {
		name     string
		generate func(optimize string, noOpt []string, printAfter string, w *bytes.Buffer) ([]compiler.File, error)
	}
	for _, c := range []gens{
		{"go", func(optimize string, noOpt []string, printAfter string, w *bytes.Buffer) ([]compiler.File, error) {
			c := compiler.GoConfig{Module: "opt", Lib: compiler.LibImport, PrintAfter: printAfter}
			if w != nil {
				c.PrintTo = w
			}
			return compiler.GenerateGo(src, c, compiler.WithStart("main"), compiler.WithOptimize(compiler.Optimize{Profile: optimize, Off: noOpt}))
		}},
		{"ts", func(optimize string, noOpt []string, printAfter string, w *bytes.Buffer) ([]compiler.File, error) {
			c := compiler.TSConfig{PrintAfter: printAfter}
			if w != nil {
				c.PrintTo = w
			}
			return compiler.GenerateTypeScript(src, c, compiler.WithStart("main"), compiler.WithOptimize(compiler.Optimize{Profile: optimize, Off: noOpt}))
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			base, err := c.generate("", nil, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			speed, err := c.generate("speed", nil, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(joinFiles(base), joinFiles(speed)) {
				t.Error("the empty profile and speed differ")
			}
			off, err := c.generate("", []string{"guards"}, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(joinFiles(base), joinFiles(off)) {
				t.Error("-no-opt guards changes nothing")
			}
			if _, err := c.generate("", []string{"nonsense"}, "", nil); err == nil || !strings.Contains(err.Error(), "valid: guards, scans, field-compare,") {
				t.Errorf("unknown optimization: %v", err)
			}
			if _, err := c.generate("nosuch", nil, "", nil); err == nil || !strings.Contains(err.Error(), "valid: speed") {
				t.Errorf("unknown profile: %v", err)
			}
			if _, err := c.generate("", nil, "guards", nil); err == nil || !strings.Contains(err.Error(), "PrintTo") {
				t.Errorf("print without a writer: %v", err)
			}
			if _, err := c.generate("", nil, "nonsense", new(bytes.Buffer)); err == nil || !strings.Contains(err.Error(), "unknown pass") {
				t.Errorf("unknown pass: %v", err)
			}
			var w bytes.Buffer
			printed, err := c.generate("", nil, "guards", &w)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(joinFiles(base), joinFiles(printed)) {
				t.Error("printing changes the output")
			}
			if !strings.Contains(w.String(), " after guards --\n") || !strings.Contains(w.String(), "alt guard=") {
				t.Errorf("printed:\n%s", w.String())
			}
		})
	}
}
