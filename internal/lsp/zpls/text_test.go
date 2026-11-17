package zpls

import (
	"context"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/lsp/protocol"
	"zeroparse.dev/zp/internal/testing/linearity"
)

func TestTextIndexPositions(t *testing.T) {
	// "é" is 2 bytes and 1 UTF-16 unit, "😀" 4 bytes and 2 units, "名" 3 bytes and 1 unit.
	text := "aé😀b\r\n名\rx\n\nlast"
	idx := newTextIndex(text)
	for _, tc := range []struct {
		off int
		pos protocol.Position
	}{
		{0, protocol.Position{Line: 0, Character: 0}},
		{1, protocol.Position{Line: 0, Character: 1}},  // é
		{3, protocol.Position{Line: 0, Character: 2}},  // 😀
		{7, protocol.Position{Line: 0, Character: 4}},  // b
		{8, protocol.Position{Line: 0, Character: 5}},  // \r
		{10, protocol.Position{Line: 1, Character: 0}}, // 名
		{13, protocol.Position{Line: 1, Character: 1}}, // \r (a line terminator on its own)
		{14, protocol.Position{Line: 2, Character: 0}}, // x
		{16, protocol.Position{Line: 3, Character: 0}}, // empty line
		{17, protocol.Position{Line: 4, Character: 0}},
		{21, protocol.Position{Line: 4, Character: 4}}, // end of text
	} {
		if got := idx.position(tc.off); got != tc.pos {
			t.Errorf("position(%d) = %v, want %v", tc.off, got, tc.pos)
		}
		if got := idx.offset(tc.pos); got != tc.off {
			t.Errorf("offset(%v) = %d, want %d", tc.pos, got, tc.off)
		}
	}
	// Offsets inside a character or a line terminator.
	for off, want := range map[int]protocol.Position{
		9: {Line: 0, Character: 5}, // between \r and \n
	} {
		if got := idx.position(off); got != want {
			t.Errorf("position(%d) = %v, want %v", off, got, want)
		}
	}
	// Positions that are not on a character boundary or past the end of a line.
	for pos, want := range map[protocol.Position]int{
		{Line: 0, Character: 3}:  3,  // between the two units of 😀: its start
		{Line: 0, Character: 99}: 8,  // past the end of the line: before \r\n
		{Line: 1, Character: 99}: 13, // before the lone \r
		{Line: 9, Character: 0}:  21, // past the last line
		{Line: -1, Character: 0}: 0,
	} {
		if got := idx.offset(pos); got != want {
			t.Errorf("offset(%v) = %d, want %d", pos, got, want)
		}
	}
}

func TestTextIndexZPPositions(t *testing.T) {
	// Columns count code points, and the \r of \r\n is a column of its line.
	text := "a😀b\r\n名\rx\nz"
	idx := newTextIndex(text)
	for _, tc := range []struct {
		pos grammar.Pos
		off int
	}{
		{grammar.Pos{Line: 1, Col: 1}, 0},
		{grammar.Pos{Line: 1, Col: 2}, 1},
		{grammar.Pos{Line: 1, Col: 3}, 5},
		{grammar.Pos{Line: 1, Col: 4}, 6}, // \r
		{grammar.Pos{Line: 2, Col: 1}, 8},
		{grammar.Pos{Line: 2, Col: 2}, 11}, // the lone \r
		{grammar.Pos{Line: 3, Col: 1}, 12}, // x, after the lone \r
		{grammar.Pos{Line: 4, Col: 1}, 14},
		{grammar.Pos{Line: 4, Col: 2}, 15},
	} {
		if got := idx.zpOffset(tc.pos); got != tc.off {
			t.Errorf("zpOffset(%v) = %d, want %d", tc.pos, got, tc.off)
		}
	}
	if got := idx.zpOffset(grammar.Pos{Line: 1, Col: 99}); got != 7 {
		t.Errorf("past the end of the line: %d", got)
	}
	if got := idx.zpOffset(grammar.Pos{Line: 9, Col: 1}); got != len(text) {
		t.Errorf("past the last line: %d", got)
	}
	if got := idx.zpOffset(grammar.Pos{Line: 2, Col: 9}); got != 11 {
		t.Errorf("past the end of a line that ends with \\r: %d", got)
	}
	// The x after the lone \r is on LSP line 2.
	if got := idx.position(idx.zpOffset(grammar.Pos{Line: 3, Col: 1})); got != (protocol.Position{Line: 2, Character: 0}) {
		t.Errorf("x at %v", got)
	}
}

func TestApplyChange(t *testing.T) {
	text := "rule a = \"😀\" b\nrule b = \"x\""
	for _, tc := range []struct {
		r    *protocol.Range
		new  string
		want string
	}{
		{nil, "all", "all"},
		{&protocol.Range{Start: protocol.Position{Line: 0, Character: 5}, End: protocol.Position{Line: 0, Character: 6}}, "名前", "rule 名前 = \"😀\" b\nrule b = \"x\""},
		// After 😀 (two UTF-16 units).
		{&protocol.Range{Start: protocol.Position{Line: 0, Character: 12}, End: protocol.Position{Line: 0, Character: 12}}, "!", "rule a = \"😀!\" b\nrule b = \"x\""},
		// Across the line break.
		{&protocol.Range{Start: protocol.Position{Line: 0, Character: 14}, End: protocol.Position{Line: 1, Character: 4}}, "c\nrule", "rule a = \"😀\" c\nrule b = \"x\""},
		// Reversed range.
		{&protocol.Range{Start: protocol.Position{Line: 1, Character: 4}, End: protocol.Position{Line: 0, Character: 14}}, "", "rule a = \"😀\"  b = \"x\""},
	} {
		if got := applyChange(text, protocol.ContentChange{Range: tc.r, Text: tc.new}); got != tc.want {
			t.Errorf("%v %q: got %q, want %q", tc.r, tc.new, got, tc.want)
		}
	}
}

func TestMinimalEdit(t *testing.T) {
	for _, tc := range []struct{ old, new string }{
		{"abc", "abc"},
		{"abc", "abXc"},
		{"a😀c", "a😁c"}, // the code points share their first bytes
		{"a\r\nb", "a\r\n\r\nb"},
		{"x\r\n", "x\r\ny\r\n"},
		{"", "new"},
		{"old", ""},
		{"名前", "名称"},
	} {
		idx := newTextIndex(tc.old)
		e := minimalEdit(idx, tc.old, tc.new)
		if got := applyChange(tc.old, protocol.ContentChange{Range: &e.Range, Text: e.NewText}); got != tc.new {
			t.Errorf("%q -> %q: edit %+v gives %q", tc.old, tc.new, e, got)
		}
	}
}

// The straightforward conversions, which walk each line from its start, are the reference for the
// indexed ones.

func naiveOffset(t *textIndex, p protocol.Position) int {
	if p.Line < 0 {
		return 0
	}
	if p.Line >= len(t.lines) {
		return len(t.text)
	}
	off, end := t.lines[p.Line], t.lineEnd(p.Line)
	for n := 0; off < end; {
		r, size := utf8.DecodeRuneInString(t.text[off:end])
		if n+utf16Len(r) > p.Character {
			break
		}
		n += utf16Len(r)
		off += size
	}
	return off
}

func naivePosition(t *textIndex, off int) protocol.Position {
	off = max(0, min(off, len(t.text)))
	line := sort.Search(len(t.lines), func(i int) bool { return t.lines[i] > off }) - 1
	off = min(off, t.lineEnd(line))
	col := 0
	for i := t.lines[line]; i < off; {
		r, size := utf8.DecodeRuneInString(t.text[i:])
		if i+size > off {
			break
		}
		col += utf16Len(r)
		i += size
	}
	return protocol.Position{Line: line, Character: col}
}

func naiveZPOffset(t *textIndex, p grammar.Pos) int {
	if p.Line < 1 {
		return 0
	}
	if p.Line > len(t.lines) {
		return len(t.text)
	}
	off := t.lines[p.Line-1]
	end := len(t.text)
	if p.Line < len(t.lines) {
		end = t.lines[p.Line] - 1
	}
	for col := 1; col < p.Col && off < end; col++ {
		_, size := utf8.DecodeRuneInString(t.text[off:])
		off += size
	}
	return off
}

// TestTextIndexMatchesNaive compares the conversions with the reference ones on random texts with
// long lines, every kind of line end and characters of every UTF-8 and UTF-16 width.
func TestTextIndexMatchesNaive(t *testing.T) {
	rnd := rand.New(rand.NewPCG(1, 2))
	pieces := []string{"a", "é", "名", "😀", "\n", "\r\n", "\r", " ", "\xff"}
	for iter := 0; iter < 200; iter++ {
		var b strings.Builder
		n := rnd.IntN(3000)
		for b.Len() < n {
			// Long runs of one piece make lines longer than the index's checkpoint spacing.
			p := pieces[rnd.IntN(len(pieces))]
			for k := rnd.IntN(200); k >= 0; k-- {
				b.WriteString(p)
			}
		}
		text := b.String()
		idx := newTextIndex(text)
		for k := 0; k < 300; k++ {
			off := rnd.IntN(len(text) + 2)
			if got, want := idx.position(off), naivePosition(idx, off); got != want {
				t.Fatalf("text %q: position(%d) = %v, want %v", text, off, got, want)
			}
			p := protocol.Position{Line: rnd.IntN(len(idx.lines)+1) - rnd.IntN(2), Character: rnd.IntN(1000)}
			if got, want := idx.offset(p), naiveOffset(idx, p); got != want {
				t.Fatalf("text %q: offset(%v) = %d, want %d", text, p, got, want)
			}
			pp := grammar.Pos{Line: rnd.IntN(len(idx.lines) + 1), Col: rnd.IntN(1000) + 1}
			if got, want := idx.zpOffset(pp), naiveZPOffset(idx, pp); got != want {
				t.Fatalf("text %q: zpOffset(%v) = %d, want %d", text, pp, got, want)
			}
		}
	}
}

// TestLongLine checks that a document with a long line is analyzed in linear work. The conversions
// used to walk the line from its start for every token, so a line of 64 KB took seconds and one of
// 1 MB did not open. The check counts the work instead of timing it, so that its verdict does not
// depend on the load of the machine: the code points that the conversions walk, the steps of the
// semantic tokens request, and the objects and bytes that the analysis and the request allocate. Each
// count of a document linearity.Factor times as long must be at most linearity.Limit times that of
// the shorter one: linear work grows Factor times and quadratic work Factor squared times.
func TestLongLine(t *testing.T) {
	const small = 1 << 11
	measure := func(n int) (walked, steps, objects, bytes uint64) {
		text := "rule a = " + strings.Repeat(`"😀" b `, n) + "\nrule b = \"x\""
		work := func() {
			walked, steps = 0, 0
			walkHook = func(runes int) { walked += uint64(runes) }
			defer func() { walkHook = nil }()
			control := &renderControl{ctx: context.Background()}
			data := analyze(text).semanticTokensControlled(control)
			steps = uint64(control.steps)
			if len(data) != 5*(2+2*n+3) {
				t.Errorf("%d semantic token integers, want %d", len(data), 5*(2+2*n+3))
			}
		}
		objects, bytes = linearity.Allocs(work)
		return
	}
	sw, ss, so, sb := measure(small)
	lw, ls, lo, lb := measure(small * linearity.Factor)
	for _, m := range []struct {
		name         string
		small, large uint64
	}{{"code points walked", sw, lw}, {"request steps", ss, ls}, {"objects allocated", so, lo}, {"bytes allocated", sb, lb}} {
		ratio, ok := linearity.Linear(m.small, m.large)
		t.Logf("%s: %d at %d tokens, %d at %d tokens, ratio %.1f", m.name, m.small, 2*small, m.large, 2*small*linearity.Factor, ratio)
		if !ok {
			t.Errorf("%s grew %.1f times for a document %d times as long (linear work grows %d times, at most %d allowed)",
				m.name, ratio, linearity.Factor, linearity.Factor, linearity.Limit)
		}
	}
}
