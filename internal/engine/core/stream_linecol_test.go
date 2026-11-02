package core

import (
	"math/rand"
	"testing"
)

// naiveLineCol is the line and column of pos in text, counted from the start, clamped to the end.
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

// TestStreamLineColMatchesScan checks lineCol of a stream buffer against a scan from the start of
// the input, for queries in any order, positions beyond the loaded end and discards between them,
// and that every unit is scanned once however many queries there are.
func TestStreamLineColMatchesScan(t *testing.T) {
	t.Parallel()
	for _, unit := range []PositionUnit{CodePoints, Bytes} {
		rng := rand.New(rand.NewSource(1))
		for round := 0; round < 200; round++ {
			text := make([]rune, 1+rng.Intn(300))
			for i := range text {
				text[i] = rune("ab\n"[rng.Intn(3)])
			}
			in := Input{Unit: unit, BaseLine: 1, BaseCol: 1}
			if unit == Bytes {
				for _, r := range text {
					in.Bs = append(in.Bs, byte(r))
				}
			} else {
				in.In = append(in.In, text...)
			}
			for q := 0; q < 100; q++ {
				if rng.Intn(8) == 0 {
					keep := in.Base + rng.Intn(len(text)-in.Base+1)
					before := in.Base
					in.Discard(keep)
					if in.Base != before && in.Base != keep {
						t.Fatalf("base %d, want %d", in.Base, keep)
					}
				}
				pos := rng.Intn(len(text) + 20)
				line, col := in.lineCol(pos)
				wl, wc := naiveLineCol(text, max(pos, in.Base))
				if pos < in.Base {
					wl, wc = in.BaseLine, in.BaseCol
				}
				if line != wl || col != wc {
					t.Fatalf("unit %v round %d: lineCol(%d) with base %d = %d:%d, want %d:%d", unit, round, pos, in.Base, line, col, wl, wc)
				}
			}
		}
	}
}

// TestStreamLineColScansOnce checks that the units of a stream buffer are charged once however
// many positions are queried, in either direction.
func TestStreamLineColScansOnce(t *testing.T) {
	t.Parallel()
	const n = 2000
	in := Input{Unit: CodePoints, BaseLine: 1, BaseCol: 1, In: make([]rune, n), Control: &BudgetControl{}}
	for i := range in.In {
		in.In[i] = 'a'
	}
	for round := 0; round < 3; round++ {
		for pos := 0; pos <= n; pos += 3 {
			in.lineCol(pos)
			in.lineCol(n - pos)
		}
	}
	if in.streamLines.scanned != n {
		t.Fatalf("scanned %d units, want %d", in.streamLines.scanned, n)
	}
	if BudgetControlsEnabled && in.Control.Used > n {
		t.Fatalf("charged %d, want at most %d", in.Control.Used, n)
	}
}
