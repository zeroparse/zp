package bench

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/engine"
)

// runSitesGrammar restarts a scan at each position: without run records the cost of a parse grows
// with the square of the input, with them it is linear. The repetition of a is a run site that turns
// hot on the second walk, so the benchmark measures the hot path (records, bits and replays).
const runSitesGrammar = `@start
rule main = (a / "a")* $$

rule a = "a"* "c"
`

// BenchmarkRunSites parses the restarted scan: a hot run site on every backend.
func BenchmarkRunSites(b *testing.B) {
	p, err := engine.CompileSource(runSitesGrammar, "main")
	if err != nil {
		b.Fatal(err)
	}
	input := strings.Repeat("a", 2000)
	for _, be := range backends {
		b.Run(be.name, func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := p.Parse(input, engine.WithBackend(be.b), engine.RecognizeOnly()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
