package ref

import "testing"

func TestInput(t *testing.T) {
	in := NewInput("aé\nIf\r")
	if r, size, ok := in.At(1); r != 'é' || size != 1 || !ok {
		t.Fatalf("At(1) = %q %d %v", r, size, ok)
	}
	if _, _, ok := in.At(in.Len()); ok {
		t.Fatal("a code point past the end")
	}
	if !in.HasPrefix(3, "if", true) || in.HasPrefix(3, "if", false) || !in.HasPrefix(3, "If", false) || in.HasPrefix(4, "f\r\n", false) {
		t.Fatal("HasPrefix")
	}
	if got := in.Slice(1, 3); got != "é\n" {
		t.Fatalf("Slice = %q", got)
	}
	az := Ranges{Ranges: [][2]rune{{'a', 'z'}}}
	if !in.MatchClass(0, az) || in.MatchClass(3, az) || in.MatchClass(in.Len(), az) {
		t.Fatal("MatchClass")
	}
	if !(Ranges{Ranges: az.Ranges, Negated: true}).Contains('I') {
		t.Fatal("a negated class")
	}
	for _, c := range []struct {
		pos, anchor int
		want        bool
	}{
		{0, BeginInput, true}, {1, BeginInput, false}, {6, EndInput, true}, {5, EndInput, false},
		{3, BeginLine, true}, {4, BeginLine, false}, {2, EndLine, true}, {5, EndLine, true}, {6, EndLine, true}, {4, EndLine, false},
	} {
		if got := in.AtAnchor(c.pos, c.anchor); got != c.want {
			t.Errorf("AtAnchor(%d, %d) = %v", c.pos, c.anchor, got)
		}
	}
}
