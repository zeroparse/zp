package fmt_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/syntax/fmt"
)

var formatBenchmarkOutput string

func BenchmarkGrammarFormatCancellation(b *testing.B) {
	sources := map[string]string{"long-literal": `rule main = "` + strings.Repeat("x", 8192) + `"`}
	for name, path := range map[string]string{"json": "../../../testdata/grammars/json/json.zp", "source-grammar": "../../../zp.zp"} {
		data, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		sources[name] = string(data)
	}
	for _, name := range []string{"json", "source-grammar", "long-literal"} {
		g, err := parser.Parse(sources[name])
		if err != nil {
			b.Fatal(err)
		}
		want := fmt.Format(g)
		for _, active := range []bool{false, true} {
			mode := "ordinary"
			if active {
				mode = "cancellable"
			}
			b.Run(name+"/"+mode, func(b *testing.B) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				b.ReportAllocs()
				for b.Loop() {
					if active {
						formatBenchmarkOutput, err = fmt.FormatContext(ctx, g)
					} else {
						formatBenchmarkOutput = fmt.Format(g)
					}
					if err != nil {
						b.Fatal(err)
					}
				}
				if formatBenchmarkOutput != want {
					b.Fatal("formatting output changed")
				}
			})
		}
	}
}
