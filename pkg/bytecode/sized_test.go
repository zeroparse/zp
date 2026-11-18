package bytecode

import "testing"

// TestSizedBoundsRoom checks that the room reserved for the elements of a count is bounded by the
// elements the bytes left could hold: a valid count gets exactly its room, a forged one no more than
// the data allows.
func TestSizedBoundsRoom(t *testing.T) {
	r := &Decoder{Data: make([]byte, 14), Pos: 2} // 12 bytes left
	if got := cap(sized[Term](r, 2, 6)); got != 2 {
		t.Errorf("valid count: room %d, want 2", got)
	}
	if got := cap(sized[Term](r, 12, 6)); got != 2 {
		t.Errorf("forged count: room %d, want 2", got)
	}
	if s := sized[Term](r, 0, 6); s != nil {
		t.Errorf("no elements: %v, want nil", s)
	}
}
