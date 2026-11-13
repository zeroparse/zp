//go:build !zp_budget_controls_reference

package engine

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	ecore "zeroparse.dev/zp/internal/engine/core"
)

func TestBudgetDocumentPieceConstruction(t *testing.T) {
	t.Parallel()
	if !ecore.DocumentPieceInput {
		t.Skip("piece input is disabled")
	}
	for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
		for _, text := range []string{strings.Repeat("é😀x", 2048), strings.Repeat("x\xff", 4096)} {
			c := ecore.NewBudgetControl(nil, core.Limits{Compute: 1 << 30})
			in := ecore.NewControlledDocumentInput(text, unit, c)
			want := ecore.NewDocumentPieces(text, unit)
			if in.Pieces == nil || !reflect.DeepEqual(in.Pieces, want) {
				t.Fatalf("%s: controlled preparation changed piece contents", unit)
			}
			if unit == core.Bytes {
				if c.Used != 0 {
					t.Fatal("immutable byte string retention was charged as a copy")
				}
			} else if c.Used == 0 {
				t.Fatal("code point decoding bypassed construction counters")
			}
		}
	}
	c := ecore.NewBudgetControl(nil, core.Limits{Compute: 1})
	if err := catchFatal(t, func() { ecore.NewControlledDocumentInput(strings.Repeat("a", 8192), core.CodePoints, c) }); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("piece preparation interruption: %v", err)
	}
}

// TestBudgetDocumentPieceTextPublication pins "Input caches are published complete" of
// "Reservations and Interruptions" in spec/runtime/parsing.md.
func TestBudgetDocumentPieceTextPublication(t *testing.T) {
	t.Parallel()
	if !ecore.DocumentPieceInput {
		t.Skip("piece input is disabled")
	}
	for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
		pieces := ecore.NewDocumentPieces(strings.Repeat("x", 8192), unit)
		pieces.Replace(4096, 4097, "é")
		in := ecore.Input{Unit: unit, Eof: true, Pieces: pieces, Control: ecore.NewBudgetControl(nil, core.Limits{Compute: 4096})}
		if err := catchFatal(t, func() { in.Text(0, pieces.N) }); !errors.Is(err, core.ErrLimit) {
			t.Fatalf("%s: piece materialization bypassed the limit: %v", unit, err)
		}
		if pieces.FullOK || pieces.Full != "" {
			t.Fatal("interrupted materialization published a cache")
		}
		in.Control = ecore.NewBudgetControl(nil, core.Limits{Compute: 1 << 30})
		got := in.Text(0, pieces.N)
		if want := strings.Repeat("x", 4096) + "é" + strings.Repeat("x", 4095); got != want || !pieces.FullOK {
			t.Fatalf("%s: retry materialization differs", unit)
		}
	}
}
