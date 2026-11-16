package zpls

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/lsp/protocol"
)

// BenchmarkApplyChanges applies one didChange notification to a 1 MiB document: 1,024 single-line
// insertions at column 0 (a multi-cursor edit) in bottom-up order, as a client may send them, the
// same insertions top-down, and a single insertion.
func BenchmarkApplyChanges(b *testing.B) {
	line := "rule r = \"0123456789\" / \"abcdefghijklmnopqrstuvw\"\n"
	text := strings.Repeat(line, (1<<20)/len(line))
	lines := strings.Count(text, "\n")
	edit := func(i int) protocol.ContentChange {
		r := protocol.Range{Start: protocol.Position{Line: i, Character: 0}, End: protocol.Position{Line: i, Character: 0}}
		return protocol.ContentChange{Range: &r, Text: "// "}
	}
	step := lines / 1024
	var bottomUp, topDown []protocol.ContentChange
	for i := range 1024 {
		bottomUp = append(bottomUp, edit((1023-i)*step))
		// Each earlier insertion stays on its own line, so the line numbers are unchanged.
		topDown = append(topDown, edit(i*step))
	}
	for _, c := range []struct {
		name    string
		changes []protocol.ContentChange
	}{{"bottom-up", bottomUp}, {"top-down", topDown}, {"single", bottomUp[:1]}} {
		b.Run(c.name, func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			b.ReportAllocs()
			for b.Loop() {
				applyChanges(text, c.changes)
			}
		})
	}
}
