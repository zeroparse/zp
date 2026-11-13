package engine

import (
	"math/rand"
	"slices"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/engine/state"
)

func TestDocumentPiecesMatchFlatEdits(t *testing.T) {
	t.Parallel()
	insertions := []string{"", "a", "é", "日", "😀", "\n", "\xc3", "\xa9", "\xff", "\xe6\xbc"}
	for _, unit := range []core.PositionUnit{core.Bytes, core.CodePoints} {
		t.Run(unit.String(), func(t *testing.T) {
			const initial = "first\né日\xff\nlast"
			d := state.NewDocumentPieces(initial, unit)
			flatBytes := []byte(initial)
			flatRunes := []rune(initial)
			rng := rand.New(rand.NewSource(76))
			for edit := range 600 {
				n := len(flatBytes)
				if unit == core.CodePoints {
					n = len(flatRunes)
				}
				start := rng.Intn(n + 1)
				end := start + rng.Intn(n-start+1)
				insert := insertions[rng.Intn(len(insertions))]
				var wantDelta int
				if unit == core.Bytes {
					wantDelta = len(insert) - (end - start)
					flatBytes = slices.Replace(flatBytes, start, end, []byte(insert)...)
				} else {
					runes := []rune(insert)
					wantDelta = len(runes) - (end - start)
					flatRunes = slices.Replace(flatRunes, start, end, runes...)
				}
				if got := d.Replace(start, end, insert); got != wantDelta {
					t.Fatalf("edit %d: delta %d, want %d", edit, got, wantDelta)
				}
				want := string(flatBytes)
				if unit == core.CodePoints {
					want = string(flatRunes)
				}
				if got := d.Text(0, d.N); got != want {
					t.Fatalf("edit %d: text %x, want %x", edit, got, want)
				}
				if edit%17 == 0 {
					for pos := 0; pos < d.N; pos++ {
						got, ok := d.At(pos)
						var expected rune
						if unit == core.Bytes {
							expected = rune(flatBytes[pos])
						} else {
							expected = flatRunes[pos]
						}
						if !ok || got != expected {
							t.Fatalf("edit %d position %d: %U, want %U", edit, pos, got, expected)
						}
					}
					if d.N > 0 {
						a := rng.Intn(d.N)
						b := a + rng.Intn(d.N-a+1)
						wantRange := string(flatBytes[a:b])
						if unit == core.CodePoints {
							wantRange = string(flatRunes[a:b])
						}
						if got := d.Text(a, b); got != wantRange {
							t.Fatalf("edit %d range [%d,%d): %x, want %x", edit, a, b, got, wantRange)
						}
					}
				}
			}
		})
	}
}

func TestDocumentPieceLargeByteOffsets(t *testing.T) {
	t.Parallel()
	// Model a slice of a >2 GiB code-point chunk without allocating that input.
	p := state.DocumentPiece{
		Text: "éa", Runes: []rune{'é', 'a'},
		LargeOffs: []int{1<<31 + 4, 1<<31 + 6, 1<<31 + 7},
	}
	if got := p.Substring(0, 1, core.CodePoints); got != "é" {
		t.Fatalf("substring: %q", got)
	}
	sub := p.Sub(1, 2, core.CodePoints)
	if got := sub.Substring(0, 1, core.CodePoints); got != "a" {
		t.Fatalf("sliced large-offset piece: %q", got)
	}
}
