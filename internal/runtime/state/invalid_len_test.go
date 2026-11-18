package state

import "testing"

// TestInvalidLen checks the bytes that decoding invalid UTF-8 examined: the bytes up to the first that
// makes the character invalid, or one more than the input when it ends inside a valid prefix, since
// the end of the input decided the decoding and an insertion there may complete the character.
func TestInvalidLen(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{
		// Bytes that cannot start a character.
		{"\xff", 1}, {"\x80", 1}, {"\xc0\x80", 1}, {"\xc1\xbf", 1}, {"\xf5\x80\x80\x80", 1}, {"\xffb", 1},
		// A second byte that does not continue a valid prefix.
		{"\xc3(", 2}, {"\xe2(", 2}, {"\xf0(", 2}, {"\xe0\x80\x80", 2}, {"\xed\xa0\x80", 2},
		{"\xf0\x80\x80\x80", 2}, {"\xf4\x90\x80\x80", 2},
		// A later byte that does not continue it.
		{"\xe2\x82(", 3}, {"\xf0\x9f(", 3}, {"\xf0\x9f\x98(", 4},
		// The input ends inside a valid prefix.
		{"\xc3", 2}, {"\xe2", 2}, {"\xe2\x82", 3}, {"\xf0", 2}, {"\xf0\x9f", 3}, {"\xf0\x9f\x98", 4},
	} {
		if got := invalidLen([]byte(c.in)); got != c.want {
			t.Errorf("invalidLen(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
