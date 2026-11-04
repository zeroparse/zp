package engine

import (
	"os"
	"path/filepath"
	"testing"

	"zeroparse.dev/zp/internal/compile"
)

// BenchmarkCompileBundledGrammars measures the compilation of each grammar of testdata/grammars
// from its parsed form, which includes the inference of value types.
func BenchmarkCompileBundledGrammars(b *testing.B) {
	files, err := filepath.Glob("../../testdata/grammars/*/*.zp")
	if err != nil || len(files) == 0 {
		b.Fatalf("no bundled grammars: %v", err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(filepath.Base(f), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				g, err := parseTest(string(src))
				if err != nil {
					b.Fatal(err)
				}
				if _, err := Compile(g, compile.Options{}); err != nil {
					b.Skipf("does not compile alone: %v", err)
				}
			}
		})
	}
}
