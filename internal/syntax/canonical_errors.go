package syntax

import (
	"slices"
	"sort"
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax/sourceast"
)

// SourceError is a syntax error of the canonical grammar: the code point
// offset in the source where parsing failed, and its message.
type SourceError struct {
	Offset int
	Msg    string
}

// SourceErrors returns the errors of a source that the canonical grammar
// rejected, in the form that Parse returns: the lexical errors of the source,
// then the errors of the grammar.
//
// The lexical errors (unterminated strings, classes and block comments,
// invalid characters and escapes) are those of the lexer that the
// hand-written parser and the literal decoding of FromSourceTree share. The
// grammar sees the same text as an unrecognized token and fails after it, at
// the end of the line or of the input.
//
// The grammar fails at the first character that cannot continue the parse. An
// error is reported where the hand-written parser reports it, at the token
// that cannot continue the parse: the first token after the white space and
// comments that follow the failure, and the start of a punctuation mark, name
// or number that the failure falls inside, such as the second character of
// ":=" or "=>", which the grammar reads as two characters. The token is found in
// the token table of the lexer, so the text of a comment is never read again.
// The failure of the grammar at the end of an unterminated string or class is not reported: it is that
// lexical error, which is. Nor is the failure exactly at the start of the declaration after an
// unterminated string or class (a line with a keyword in the first column), if no declaration starts between them: the literal swallowed the closing
// bracket of a group, and the hand-written parser ends the group there without another error. The list is capped and de-duplicated as the hand-written parser's is.
//
// The lexer skips the characters that start no token and goes on, so the
// hand-written parser reads the rest of the source as if they were not there.
// The grammar cannot skip them, so a caller that parses src with it should
// parse WithoutInvalidCharacters(src) when that differs, and pass the offsets
// of the failure there: the offsets are those of src.
func SourceErrors(src string, errs []SourceError) ErrorList {
	out := &errorList{}
	toks := lex(src, out)
	rs := []rune(src)
	pos := sourceast.NewPositions(src)
	offsets := make(map[grammar.Pos]int, len(rs)+1)
	for i := len(rs); i >= 0; i-- {
		offsets[pos.At(i)] = i
	}
	// The tokens are those of the lexer, which reads comments as white space: an
	// offset is never matched against the text of a comment. unterminated is where
	// a block comment that is never closed starts, which swallows the rest.
	type span struct {
		kind       tokenKind
		pos        grammar.Pos
		start, end int
	}
	var spans []span
	for _, t := range toks {
		if t.kind != tEOF {
			spans = append(spans, span{t.kind, t.pos, offsets[t.pos], offsets[t.end]})
		}
	}
	unterminated := -1
	var unterminatedAt grammar.Pos
	for _, e := range out.list {
		if e.Msg == "unterminated block comment" {
			unterminated, unterminatedAt = offsets[e.Pos], e.Pos
		}
	}
	// An unterminated string or class is a token of the lexer, and the grammar fails at its end, where it
	// expects the closing quote or bracket. That failure is the lexical error again, and it would put an
	// error on the next line, which has none: the declaration there may be valid.
	var open [][2]int
	for _, e := range out.list {
		if e.Msg == "unterminated string" || e.Msg == "unterminated character class" {
			for _, sp := range spans {
				if sp.pos == e.Pos {
					open = append(open, [2]int{sp.start, sp.end})
				}
			}
		}
	}
	// lineStart reports whether span i starts a line, in the first column: a declaration of a grammar file
	// starts there, and an indented line with a keyword is a statement or a field of a group.
	lineStart := func(i int) bool { return spans[i].start == 0 || rs[spans[i].start-1] == '\n' }
	keyword := func(i int) bool {
		if spans[i].kind != tIdent || !lineStart(i) {
			return false
		}
		switch string(rs[spans[i].start:spans[i].end]) {
		case "rule", "type", "var":
			return true
		}
		return false
	}
	// swallowed reports whether the grammar fails at span i, the start of a declaration, because an
	// unterminated string or class swallowed the closing bracket of a group in the declaration before it. The
	// lexical error is the report of that declaration, and the hand-written parser ends the group at the
	// declaration without another.
	swallowed := func(i int) bool {
		if !lineStart(i) || !keyword(i) && rs[spans[i].start] != '@' {
			return false
		}
		from := 0
		for j := i - 1; j >= 0; j-- {
			if keyword(j) {
				from = spans[j].start
				break
			}
		}
		return slices.ContainsFunc(open, func(r [2]int) bool { return r[0] > from && r[1] <= spans[i].start })
	}
	for _, e := range errs {
		if slices.ContainsFunc(open, func(r [2]int) bool { return r[0] <= e.Offset && e.Offset <= r[1] }) {
			continue
		}
		// The error is at the first token that ends after the failure: the token
		// that contains it, or else the next one. Inside a string or a class the
		// failure stays where it is.
		at := pos.At(min(e.Offset, len(rs)))
		k := sort.Search(len(spans), func(i int) bool { return spans[i].end > e.Offset })
		switch {
		case k < len(spans) && e.Offset < spans[k].start:
			at = spans[k].pos
		case k < len(spans):
			if spans[k].kind != tString && spans[k].kind != tCharClass {
				if swallowed(k) && e.Offset == spans[k].start {
					continue
				}
				at = spans[k].pos
			}
		case unterminated >= 0:
			at = unterminatedAt
		default:
			at = pos.At(len(rs))
		}
		out.add(at, e.Msg)
	}
	return out.list
}

// WithoutInvalidCharacters returns src with each run of characters that start no
// token replaced by spaces, and whether there was any. The lexer reports such a
// run as an error and skips it, so the hand-written parser parses the source
// without it. The offsets of the result are those of src.
func WithoutInvalidCharacters(src string) (string, bool) {
	errs := &errorList{}
	lex(src, errs)
	var runs []*Error
	for _, e := range errs.list {
		if strings.HasPrefix(e.Msg, "unexpected character") {
			runs = append(runs, e)
		}
	}
	if len(runs) == 0 {
		return src, false
	}
	rs := []rune(src)
	pos := sourceast.NewPositions(src)
	k := 0
	for i := range rs {
		at := pos.At(i)
		for k < len(runs) && !posLess(at, runs[k].End) {
			k++
		}
		if k < len(runs) && !posLess(at, runs[k].Pos) {
			rs[i] = ' '
		}
	}
	return string(rs), true
}

func posLess(a, b grammar.Pos) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col
}
