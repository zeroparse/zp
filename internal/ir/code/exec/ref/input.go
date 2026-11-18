package ref

// The reference model of the input that grammar code reads (the operations of the Flow dialect that
// match the input, design 065): a text of code points, read at a position in code points. The
// conformance of the Flow operations checks each read of a target against it; a target that reads
// bytes is not modeled.

// Input is the input of a parse, as code points.
type Input struct{ text []rune }

// NewInput returns the model of the input text.
func NewInput(text string) *Input { return &Input{text: []rune(text)} }

// Len returns the length of the input.
func (in *Input) Len() int { return len(in.text) }

// At returns the code point at pos, its size (1) and whether there is one: false at or past the end.
func (in *Input) At(pos int) (r rune, size int, ok bool) {
	if pos < 0 || pos >= len(in.text) {
		return 0, 0, false
	}
	return in.text[pos], 1, true
}

// Slice returns the input from start to end.
func (in *Input) Slice(start, end int) string { return string(in.text[start:end]) }

// HasPrefix reports whether the input at pos begins with lit; with fold, each ASCII letter of the input
// matches either case of the same letter of lit.
func (in *Input) HasPrefix(pos int, lit string, fold bool) bool {
	rs := []rune(lit)
	if pos < 0 || pos+len(rs) > len(in.text) {
		return false
	}
	for i, r := range rs {
		c := in.text[pos+i]
		if c != r && !(fold && lower(c) == lower(r)) {
			return false
		}
	}
	return true
}

func lower(r rune) rune {
	if 'A' <= r && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

// Ranges is a character class: inclusive ranges of code points, or with Negated every code point
// outside them.
type Ranges struct {
	Ranges  [][2]rune
	Negated bool
}

// Contains reports whether the code point r is in the class.
func (c Ranges) Contains(r rune) bool {
	in := false
	for _, x := range c.Ranges {
		if x[0] <= r && r <= x[1] {
			in = true
			break
		}
	}
	return in != c.Negated
}

// MatchClass reports whether the code point at pos is in the class c.
func (in *Input) MatchClass(pos int, c Ranges) bool {
	r, _, ok := in.At(pos)
	return ok && c.Contains(r)
}

// Anchors of positions.
const (
	BeginInput = iota
	EndInput
	BeginLine
	EndLine
)

// AtAnchor reports whether pos is at the anchor: the beginning or the end of the input, the beginning of a
// line (the beginning of the input or after a line feed) or the end of one (the end of the input or
// before a line feed or a carriage return).
func (in *Input) AtAnchor(pos, anchor int) bool {
	switch anchor {
	case BeginInput:
		return pos == 0
	case EndInput:
		return pos == len(in.text)
	case BeginLine:
		return pos == 0 || in.text[pos-1] == '\n'
	default:
		r, _, ok := in.At(pos)
		return !ok || r == '\n' || r == '\r'
	}
}
