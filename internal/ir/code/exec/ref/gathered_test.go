package ref

import (
	"testing"

	"zeroparse.dev/zp/internal/ir/code/exec"
)

// TestMadeRecordedOnce pins the precondition of act.made: no act.made recorded the struct, ever. A
// struct that a drop forgot is not in the values made any more, and recording it again still breaks the
// precondition.
func TestMadeRecordedOnce(t *testing.T) {
	m := New(Config{})
	made := exec.Parser().Op("act.made")
	n := Struct{ID: 7}
	if !m.Allows(made, []any{Act{}, n}) {
		t.Fatal("a new struct may not be recorded")
	}
	m.Call(made, []any{Act{}, n})
	if m.Allows(made, []any{Act{}, n}) {
		t.Error("a struct in the values made may be recorded again")
	}
	m.Call(exec.Parser().Op("term.dropCreated"), []any{int64(0)})
	if got := m.Made(); len(got) != 0 {
		t.Fatalf("values made after a drop to 0 = %v, want none", got)
	}
	if m.Allows(made, []any{Act{}, n}) {
		t.Error("a struct that a drop forgot may be recorded again")
	}
	if !m.Allows(made, []any{Act{}, Struct{ID: 8}}) {
		t.Error("another new struct may not be recorded")
	}
}
