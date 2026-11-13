package codegen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/codegen"
)

func TestLibMode(t *testing.T) {
	if codegen.LibEmbed.String() != "embed" || codegen.LibImport.String() != "import" || codegen.LibMode(2).String() != "LibMode(2)" {
		t.Errorf("names: %v %v %v", codegen.LibEmbed, codegen.LibImport, codegen.LibMode(2))
	}
	parser, err := compileTestSource(`rule main = "x"`, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []codegen.Generator{
		codegen.Go{Module: "example.com/m", Lib: codegen.LibMode(2)},
		codegen.TypeScript{Lib: codegen.LibMode(-1)},
	} {
		if _, err := g.Generate(parser.Grammar(), "main"); err == nil || !strings.Contains(err.Error(), "unknown library mode") {
			t.Errorf("%T: %v", g, err)
		}
	}
	// GenerateLib writes only the library, whatever the mode.
	files, err := codegen.Go{Module: "example.com/m", Lib: codegen.LibImport}.GenerateLib()
	if err != nil || len(files) == 0 {
		t.Fatalf("Go library: %d files, %v", len(files), err)
	}
	for _, f := range files {
		if !strings.HasPrefix(f.Path, "internal/libgo/") {
			t.Errorf("Go library file %s", f.Path)
		}
	}
	files, err = codegen.TypeScript{}.GenerateLib()
	if err != nil || len(files) != 1 || files[0].Path != "lib.ts" {
		t.Errorf("TypeScript library: %v, %v", files, err)
	}
}
