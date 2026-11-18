package compiler_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/pkg/compiler"
	"zeroparse.dev/zp/pkg/grammar"
)

func TestLibMode(t *testing.T) {
	if compiler.LibEmbed.String() != "embed" || compiler.LibImport.String() != "import" || compiler.LibMode(2).String() != "LibMode(2)" {
		t.Errorf("names: %v %v %v", compiler.LibEmbed, compiler.LibImport, compiler.LibMode(2))
	}
	src := grammar.FromString("lib.zp", "@start\nrule main = \"x\"")
	if _, err := compiler.GenerateGo(src, compiler.GoConfig{Module: "example.com/m", Lib: compiler.LibMode(2)}); err == nil || !strings.Contains(err.Error(), "unknown library mode") {
		t.Errorf("Go: %v", err)
	}
	if _, err := compiler.GenerateTypeScript(src, compiler.TSConfig{Lib: compiler.LibMode(-1)}); err == nil || !strings.Contains(err.Error(), "unknown library mode") {
		t.Errorf("TypeScript: %v", err)
	}
	// GenerateLib writes only the library, whatever the mode.
	files, err := compiler.GenerateGoLib("example.com/m")
	if err != nil || len(files) == 0 {
		t.Fatalf("Go library: %d files, %v", len(files), err)
	}
	for _, f := range files {
		if !strings.HasPrefix(f.Path, "internal/libgo/") {
			t.Errorf("Go library file %s", f.Path)
		}
	}
	files, err = compiler.GenerateTypeScriptLib()
	if err != nil || len(files) != 1 || files[0].Path != "lib.ts" {
		t.Errorf("TypeScript library: %v, %v", files, err)
	}
}
