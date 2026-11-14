package libgo

import (
	"fmt"
	"testing"
	"unicode/utf8"

	"zeroparse.dev/zp/core"
)

// examinedEndCases are the cases of the tests of the examined end of terminals: the literal a call
// of main compares at the position after skip characters of the input, the input, and the end that
// the entry of the comparison records in each unit: one past the first unit that differs from the
// literal, or one past the end of the input, and the position after the literal when it matches.
var examinedEndCases = []struct {
	literal, input string
	skip           int
	codePoints     int
	bytes          int
}{
	{"abc", "abc", 0, 3, 3},
	{"abc", "abd", 0, 3, 3},
	{"abc", "xbc", 0, 1, 1},
	{"abc", "ab", 0, 3, 3},
	{"abc", "a", 0, 2, 2},
	{"abc", "", 0, 1, 1},
	{"abc", "zzabx", 2, 5, 5},
	{"abc", "zzab", 2, 5, 5},
	{"abc", "zz", 2, 3, 3},
	{"abc", "zzabc", 2, 5, 5},
	// The units differ for text with characters of more than one byte.
	{"éb", "éx", 0, 2, 3},
	{"éb", "éb", 0, 2, 3},
	{"éb", "é", 0, 2, 3},
	{"éb", "ex", 0, 1, 1},
	{"ab", "éé", 1, 2, 3},
	{"ab", "ééa", 2, 4, 6},
	{"b", "é", 0, 1, 1},
}

// TestPrivateDocumentLiteralExaminedEnd checks that a literal that a Document parse compares with the
// input examines the input up to its first unit that differs, and that a call of one that fails
// at the end of the input examines the unit at the end, in both units, so that the entry of the call
// is dropped by an edit of exactly that unit and no earlier one.
func TestPrivateDocumentLiteralExaminedEnd(t *testing.T) {
	for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
		for _, c := range examinedEndCases {
			t.Run(fmt.Sprintf("unit %d %q in %q", unit, c.literal, c.input), func(t *testing.T) {
				lit := &Rule{Id: 1, Name: "lit", Memo: true, DocMemo: true, Seen: -1}
				lit.Body = func(p *Parser, _ int) (*core.Node, bool) {
					return p.MatchLiteral([]rune(c.literal), c.literal, IdAny, true)
				}
				main := &Rule{Id: 0, Name: "main", Memo: true, DocMemo: true, Seen: -1}
				main.Body = func(p *Parser, _ int) (*core.Node, bool) {
					for range c.skip {
						_, size, _ := p.Peek()
						p.Pos += size
					}
					p.Call(lit, 0)
					return nil, false
				}
				d := testDocumentState(main, c.input, unit)
				d.parse(parseOptions{unit: unit, maxDepth: defaultMaxDepth})
				at, want := 0, c.codePoints
				if unit == core.Bytes {
					at, want = len(c.input[:min(len(c.input), skipBytes(c.input, c.skip))]), c.bytes
				} else {
					at = c.skip
				}
				e := d.memo.get(lit.Id, at, 0, nil)
				if e == nil || e.examined != want {
					t.Fatalf("entry = %+v, want examined %d", e, want)
				}
			})
		}
	}
}

// skipBytes returns the offset in s after n characters.
func skipBytes(s string, n int) int {
	off := 0
	for ; n > 0 && off < len(s); n-- {
		_, size := utf8.DecodeRuneInString(s[off:])
		off += size
	}
	return off
}

// decodedEndCases are the cases of the tests of the end that decoding a character examined, for
// the library and for the engine (invalidLen): the input, the position of the character in bytes,
// and the end that decoding it examined. A valid character examines its bytes; an invalid one the
// bytes up to the first that makes it invalid, or, when the input ends inside a valid prefix, the end
// of the input counted as the unit at its position.
var decodedEndCases = []struct {
	input string
	pos   int
	end   int
}{
	{"a", 0, 1},
	{"é", 0, 2},
	{"€", 0, 3},
	{"\U0001f600", 0, 4},
	{"abé", 2, 4},
	// Bytes that cannot start a character, or continue none.
	{"\xff", 0, 1},
	{"\x80", 0, 1},
	{"\xc0\x80", 0, 1},
	{"\xc1\xbf", 0, 1},
	{"\xf5\x80\x80\x80", 0, 1},
	{"a\xffb", 1, 2},
	// A second byte that does not continue a valid prefix.
	{"\xc3(", 0, 2},
	{"\xe2(", 0, 2},
	{"\xf0(", 0, 2},
	{"\xe0\x80\x80", 0, 2},
	{"\xed\xa0\x80", 0, 2},
	{"\xf0\x80\x80\x80", 0, 2},
	{"\xf4\x90\x80\x80", 0, 2},
	// A later byte that does not continue it.
	{"\xe2\x82(", 0, 3},
	{"\xf0\x9f(", 0, 3},
	{"\xf0\x9f\x98(", 0, 4},
	{"xx\xf0\x9f\x98(", 2, 6},
	// The input ends inside a valid prefix: the end of the input is the unit examined.
	{"\xc3", 0, 2},
	{"\xe2", 0, 2},
	{"\xe2\x82", 0, 3},
	{"\xf0", 0, 2},
	{"\xf0\x9f", 0, 3},
	{"\xf0\x9f\x98", 0, 4},
	{"a\xc3", 1, 3},
	{"ab\xe2\x82", 2, 5},
	{"ab\xf0\x9f\x98", 2, 6},
}

func TestPrivateDecodedEnd(t *testing.T) {
	for _, c := range decodedEndCases {
		_, size := utf8.DecodeRuneInString(c.input[c.pos:])
		if got := decodedEnd(c.input[c.pos:], c.pos, size); got != c.end {
			t.Errorf("decodedEnd(%q, %d) = %d, want %d", c.input, c.pos, got, c.end)
		}
	}
}
