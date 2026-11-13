package vm

import (
	"os"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
)

// BenchmarkBytecodeLowering isolates the construction of the module, match code and term code, from
// the Core IR of an already checked grammar after the bytecode pipeline; source parsing, checking,
// the Core IR build and VM setup are out.
func BenchmarkBytecodeLowering(b *testing.B) {
	for _, c := range []struct{ name, path string }{
		{"JSON", "../../../testdata/grammars/json/json.zp"},
		{"Minilang", "../../../examples/minilang/minilang.zp"},
	} {
		b.Run(c.name, func(b *testing.B) {
			source, err := os.ReadFile(c.path)
			if err != nil {
				b.Fatal(err)
			}
			p := compiletest.Program(b, string(source), compile.Options{})
			m, err := build.CoreControlled(p, nil)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := ir.Run(m, &ir.Env{}, ir.RunOptions{}); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := CompileCore(p, m); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
