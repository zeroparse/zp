package codegen_test

import (
	"bytes"
	"strings"
	"testing"

	"zeroparse.dev/zp/codegen"
)

func TestMaxDepth(t *testing.T) {
	parser, err := compileTestSource(`rule main = "x"`, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, ts := range []bool{false, true} {
		generate := func(depth int) ([]byte, error) {
			files, err := codegen.Go{Module: "depth", Lib: codegen.LibImport, MaxDepth: depth}.Generate(parser.Grammar(), "main")
			return joinFiles(files), err
		}
		if ts {
			generate = func(depth int) ([]byte, error) {
				files, err := codegen.TypeScript{MaxDepth: depth}.Generate(parser.Grammar(), "main")
				return joinFiles(files), err
			}
		}
		base, err := generate(0)
		if err != nil {
			t.Fatal(err)
		}
		got, err := generate(100000)
		if err != nil || !bytes.Equal(base, got) {
			t.Fatalf("equivalent default: %v", err)
		}
		got, err = generate(7)
		want := "var zpGrammar = rt.NewGrammar(7)"
		if ts {
			want = "const G = mkGrammar(7, "
		}
		if err != nil || !strings.Contains(string(got), want) {
			t.Fatalf("generated max depth: %v", err)
		}
		if got, err := generate(-1); err == nil || len(got) != 0 {
			t.Fatal("invalid generated max depth accepted")
		}
	}
}

// joinFiles returns the contents of the generated files one after the other.
func joinFiles(files []codegen.File) []byte {
	var out []byte
	for _, f := range files {
		out = append(out, f.Content...)
	}
	return out
}
