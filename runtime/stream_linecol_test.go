package runtime

import (
	"math/rand"
	"testing"

	"zeroparse.dev/zp/core"
)

func naiveLineCol(text []rune, pos int) (int, int) {
	line, col := 1, 1
	for i := 0; i < min(pos, len(text)); i++ {
		if text[i] == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return line, col
}

// TestStreamLineColMatchesScan checks streamLineCol of a stream buffer against a scan from the
// start of the input, for queries in any order, positions beyond the loaded end and discards
// between them.
func TestStreamLineColMatchesScan(t *testing.T) {
	t.Parallel()
	for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
		rng := rand.New(rand.NewSource(1))
		for round := 0; round < 200; round++ {
			text := make([]rune, 1+rng.Intn(300))
			for i := range text {
				text[i] = rune("ab\n"[rng.Intn(3)])
			}
			p := &Parser{unit: unit, Stream: &streamInput{baseLine: 1, baseCol: 1}}
			s := p.Stream
			for _, r := range text {
				if unit == core.Bytes {
					s.bs = append(s.bs, byte(r))
				} else {
					s.in = append(s.in, r)
				}
			}
			for q := 0; q < 100; q++ {
				if rng.Intn(8) == 0 {
					p.streamDiscard(s.base + rng.Intn(len(text)-s.base+1))
				}
				pos := rng.Intn(len(text) + 20)
				line, col := p.streamLineCol(pos)
				wl, wc := naiveLineCol(text, pos)
				if pos < s.base {
					wl, wc = s.baseLine, s.baseCol
				}
				if line != wl || col != wc {
					t.Fatalf("unit %v round %d: streamLineCol(%d) with base %d = %d:%d, want %d:%d", unit, round, pos, s.base, line, col, wl, wc)
				}
			}
		}
	}
}
