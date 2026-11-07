package language_test

import (
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
	"unicode"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax"
	"zeroparse.dev/zp/internal/syntax/sourceast"
)

// errorAt is an error of a frontend: its place in the source as a code point offset, and its message.
type errorAt struct {
	off int
	msg string
}

// errorsOf returns the errors of err, which a frontend returned for src, in the order of the list.
func errorsOf(src string, err error) []errorAt {
	var list syntax.ErrorList
	if !errors.As(err, &list) {
		return nil
	}
	pos := sourceast.NewPositions(src)
	offsets := map[grammar.Pos]int{}
	for i := len([]rune(src)); i >= 0; i-- {
		offsets[pos.At(i)] = i
	}
	out := make([]errorAt, len(list))
	for i, e := range list {
		out[i] = errorAt{offsets[e.Pos], e.Msg}
	}
	return out
}

// harmStats counts how the errors of the canonical frontend differ from those of the hand-written
// parser on the sources that both reject.
type harmStats struct {
	rejected int
	// sameList counts the sources whose errors are at the same offsets in both frontends, in the order
	// of the source. withSecond counts those where the hand-written parser reports two errors or more,
	// and sameSecond those of them where the second error, in the order of the source, is at the same
	// offset.
	sameList, withSecond, sameSecond int
	// first counts the sources by how the first error in the source differs.
	counts   map[string]int
	examples map[string][]string
	// maxLines is the largest number of line breaks between the first errors of a class.
	maxLines map[string]int
}

func newHarmStats() *harmStats {
	return &harmStats{counts: map[string]int{}, examples: map[string][]string{}, maxLines: map[string]int{}}
}

func (s *harmStats) note(label, example string) {
	s.counts[label]++
	if len(s.examples[label]) < 12 {
		s.examples[label] = append(s.examples[label], example)
	}
}

// first returns the first error in the source.
func first(errs []errorAt) errorAt {
	f := errs[0]
	for _, e := range errs[1:] {
		if e.off < f.off {
			f = e
		}
	}
	return f
}

func bucket(d int) string {
	switch {
	case d <= 1:
		return "<=1"
	case d <= 4:
		return "<=4"
	case d <= 16:
		return "<=16"
	case d <= 64:
		return "<=64"
	}
	return ">64"
}

func shortMsg(m string) string {
	if i := strings.Index(m, ", found"); i >= 0 {
		m = m[:i]
	}
	if len(m) > 44 {
		m = m[:44]
	}
	return m
}

// add compares the errors of the frontends on src, which edits changed in the ranges of code points that
// edits holds (nil when it is not known).
func (s *harmStats) add(t *testing.T, p *zp.Parser, src string, edits [][2]int) {
	_, merr := zp.ParseGrammar(src)
	_, err := adaptCanonical(p, src)
	if merr == nil || err == nil {
		return
	}
	h, g := errorsOf(src, merr), errorsOf(src, err)
	if len(h) == 0 || len(g) == 0 {
		t.Errorf("%q: errors without positions: %v, %v", src, merr, err)
		return
	}
	s.rejected++
	rs := []rune(src)
	starts := declarationStarts(rs)
	region := func(off int) int { return declarationOf(starts, off) }
	describe := func() string {
		return fmt.Sprintf("%q\n  hand-written: %v\n  canonical:    %v", src, merr, err)
	}
	fh, fg := first(h), first(g)
	oh, og := sortedOffsets(h), sortedOffsets(g)
	if slices.Equal(oh, og) {
		s.sameList++
	}
	if len(oh) >= 2 {
		s.withSecond++
		if len(og) >= 2 && oh[1] == og[1] {
			s.sameSecond++
		}
	}
	label := classify(rs, starts, h, fh, fg)
	s.note(label, describe())
	if d := lines(rs, min(fh.off, fg.off), max(fh.off, fg.off)); d > s.maxLines[label] {
		s.maxLines[label] = d
	}
	// Errors are matched by declaration. An offset at the start of a declaration may belong to the
	// declaration before it (a closing delimiter is missing) or to the one it starts.
	same := func(a, b int) bool {
		return region(a) == region(b) || sort.SearchInts(starts, a+1) == sort.SearchInts(starts, b+1)
	}
	// An error after an unterminated block comment follows from it: the comment takes the rest of the
	// source, and the hand-written parser then reports what is missing at the end.
	independent := h
	for i, e := range h {
		if strings.HasPrefix(e.msg, "unterminated block comment") {
			independent = h[:i+1]
			break
		}
	}
	for _, e := range independent {
		found := false
		for _, o := range g {
			found = found || same(o.off, e.off) || related(rs, e.off, o.off)
		}
		if !found {
			s.note("missed: "+missedClass(e), describe())
		}
	}
	for _, e := range g {
		found := false
		for _, o := range h {
			found = found || same(o.off, e.off) || related(rs, e.off, o.off)
		}
		if !found || edits != nil && unedited(rs, starts, edits, h, g, e) {
			s.note("extra", describe())
		}
	}
}

// unedited reports whether the error e of the canonical parser is on a declaration that the edits did not
// touch, where the reference reports no error, and where no brace or parenthesis is open: an error on
// text that is valid as it is. An offset at the start of a declaration belongs to the declaration that it
// starts, so the error that a broken declaration causes at the next one is not hidden by the declaration
// before it. This is stricter than the matching by declaration above, which allows an error at the start
// of the declaration after one that the reference reports. One exception stays: the only error that the
// canonical parser reports for a broken declaration, at the start of the next one, is the report of what
// the broken declaration lacks at its end (see the class end), so it is not an error on valid text. The
// declaration before is then edited or has an error of the reference, and has no other error of the
// canonical parser.
func unedited(rs []rune, starts []int, edits [][2]int, h, g []errorAt, e errorAt) bool {
	lo, hi := 0, len(rs)
	i := sort.SearchInts(starts, e.off+1)
	if i > 0 {
		lo = starts[i-1]
		if i < len(starts) {
			hi = starts[i]
		}
	} else if len(starts) > 0 {
		hi = starts[0]
	}
	touched := func(lo, hi int) bool {
		for _, o := range h {
			if lo <= o.off && o.off <= hi {
				return true
			}
		}
		for _, r := range edits {
			if r[0] <= hi && lo <= r[1] {
				return true
			}
		}
		return false
	}
	if touched(lo, hi) {
		return false
	}
	if i > 1 && e.off == lo {
		plo := starts[i-2]
		alone := true
		for _, o := range g {
			alone = alone && (o == e || o.off < plo || o.off >= lo)
		}
		if alone && touched(plo, lo) {
			return false
		}
	}
	return len(openAt(rs, e.off)) == 0
}

// harmCaps are drift alarms, not bounds that the design guarantees. Each is the share of the sources that
// both frontends reject, in parts per ten thousand, that a class took on average in 12 seeds of 100,000
// sources (the 8 of the long tier and 4 more) plus three standard deviations of the share in one seed
// (binomial, about 75,000 rejected sources), rounded up. The long tier is therefore not a held-out check:
// seeds 13 to 22, which did not set the caps, were run once with them (see the record). The caps say what
// this generator produces on this grammar and corpus: a change of the grammar, of the reference, of the
// corpus or of the classification that moves a share past its cap shows here, and the cause is then looked
// at, not the cap raised. A class of a cap above 0 may have 2 sources more than the share allows, and in a
// small sample (the default tier) also three standard deviations of a count of that mean, so that the
// spread of 3,000 sources does not fail it. A class that has no entry has the cap 0.
var harmCaps = map[string]int{
	"near":          534,
	"end":           104,
	"shadowed":      39,
	"head":          15,
	"unclosed":      11,
	"dropped":       10,
	"missed: other": 22,
	"hint":          3,
	"extra":         3,
	"late":          1,
	"missed: hint":  1,
	"other earlier": 1,
	"other later":   1,
}

// smallSample is the number of rejected sources below which check allows the spread of a small sample.
const smallSample = 20000

// check requires each class to stay within its cap.
func (s *harmStats) check(t *testing.T) {
	t.Helper()
	byClass := map[string]int{}
	for l, n := range s.counts {
		if l == "same" {
			continue
		}
		// "other later <=16" and "other later >64" are one class for the cap.
		if i := strings.Index(l, " <="); i >= 0 {
			l = l[:i]
		} else if i := strings.Index(l, " >"); i >= 0 {
			l = l[:i]
		}
		byClass[l] += n
	}
	for l, n := range byClass {
		allowed := 0
		if c := harmCaps[l]; c > 0 {
			mean := float64(c*s.rejected) / 10000
			allowed = int(math.Ceil(mean)) + 2
			if s.rejected < smallSample {
				// A small sample varies more than the seeds that the caps come from: allow three
				// standard deviations of a count of this mean.
				allowed = int(math.Ceil(mean+3*math.Sqrt(mean))) + 2
			}
		}
		if n > allowed {
			t.Errorf("%d of %d rejected sources are in the class %q, which allows %d (%d in 10,000)", n, s.rejected, l, allowed, harmCaps[l])
		}
	}
}

// report logs the counts.
func (s *harmStats) report(t *testing.T, total int) {
	t.Helper()
	var labels []string
	for l := range s.counts {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	t.Logf("%d of %d mutated sources are rejected by both frontends", s.rejected, total)
	t.Logf("the errors are at the same offsets in %d of them; the second error is at the same offset in %d of the %d that have two", s.sameList, s.sameSecond, s.withSecond)
	for _, l := range labels {
		t.Logf("%6d  %s", s.counts[l], l)
	}
	for _, l := range labels {
		if l != "same" && s.maxLines[l] > 0 {
			t.Logf("max %d lines apart  %s", s.maxLines[l], l)
		}
	}
	if only := os.Getenv("ZP_HARM_EXAMPLES"); only != "" {
		for _, l := range labels {
			if l == "first: same" || !strings.Contains(l, only) && only != "1" {
				continue
			}
			for _, ex := range s.examples[l] {
				t.Logf("example of %s: %s", l, strings.TrimSpace(ex))
			}
		}
	}
}

// sortedOffsets returns the offsets of the errors, in the order of the source, without repeats.
func sortedOffsets(errs []errorAt) []int {
	var out []int
	for _, e := range errs {
		out = append(out, e.off)
	}
	sort.Ints(out)
	return slices.Compact(out)
}

// declarationOf returns the index of the declaration that holds off. An offset at the start of a
// declaration belongs to the declaration before it: a closing delimiter that is missing is found there.
func declarationOf(starts []int, off int) int { return sort.SearchInts(starts, off) }

// hintMessages are the starts of the messages of the checks that the hand-written parser makes while
// it reads the tokens of a declaration. The canonical parser makes them in the adapter only on a source
// that its grammar accepts, so it reports the next error of the grammar instead, and the hint when that
// is fixed.
var hintMessages = []string{
	"a capture is read with .$",
	"an index [ must directly follow",
	"a character class directly after a name",
	"positional captures are removed",
	"node takes a type argument",
	"a node type takes one type argument",
	"lambda parameters are written without $",
	"unknown pragma",
	"invalid number",
	"invalid range",
	"invalid unicode escape",
	"invalid escape",
	"multiline expressions require their own open delimiter",
}

func isHint(msg string) bool {
	for _, h := range hintMessages {
		if strings.HasPrefix(msg, h) {
			return true
		}
	}
	return false
}

func missedClass(e errorAt) string {
	if isHint(e.msg) {
		return "hint"
	}
	return "other"
}

// classify names how the first error of the canonical parser, fg, differs from the first error of the
// hand-written parser, fh, on a source that both reject. Every class but same and other describes an
// error that points at a real mistake of the source, only at another place or for another mistake of the
// same declaration or of a construct that the source does not close, never at a declaration without a
// mistake. A brace or a parenthesis is open at a place when a { or ( before it has no match before it,
// outside strings and comments.
//
//	same     the same offset
//	near     the canonical parser is at most 16 code points after or before, on the same line or the next,
//	         in the same declaration: the next token or two of the same mistake
//	shadowed the canonical parser reports an error that the hand-written parser reports too, but not
//	         the error of the hand-written parser before it, which a recovery inside a declaration that
//	         fails later drops
//	hint     the hand-written parser reports a lexical check that the grammar does not make, and the
//	         canonical parser reports the next error of the declaration, or one that the hand-written
//	         parser reports too
//	end      the canonical parser reports at the end of the declaration, where a closing delimiter that
//	         is missing was expected, and the hand-written parser reports another mistake before it
//	head     the canonical parser stops at the start of a declaration (a line, or a statement after a
//	         semicolon, that has the shape of one) with a brace or a parenthesis open there, and the
//	         hand-written parser reads the declaration as an item and fails later
//	late     the hand-written parser reports a check of the whole declaration (a hint) at its end, and the
//	         canonical parser reports the mistake itself, earlier in the same declaration
//	unclosed the canonical parser reports later than the first error of the hand-written parser, where
//	         a brace or a parenthesis that is open at the error of the hand-written parser is still open,
//	         in its declaration or in a later one that the construct swallows: the recovery inside it is
//	         dropped with the construct that fails, and the error is where the end is missing
//	dropped  the canonical parser reports later in the declaration of the first error of the hand-written
//	         parser with nothing open: the error of the hand-written parser is in a part that the canonical
//	         parser's recovery drops when the declaration fails after it
//	other    anything else; the class is reported with the sources that fall in it
func classify(rs []rune, starts []int, h []errorAt, fh, fg errorAt) string {
	if fh.off == fg.off {
		return "same"
	}
	rh, rg := declarationOf(starts, fh.off), declarationOf(starts, fg.off)
	// An offset at the start of a declaration may belong to the declaration before it or to the one it
	// starts.
	sameDecl := rh == rg || sort.SearchInts(starts, fh.off+1) == sort.SearchInts(starts, fg.off+1)
	// end is the end of the declaration of fh: the next declaration, or the end of the source.
	end := len(rs)
	if rh < len(starts) {
		end = starts[rh]
	}
	near := abs(fg.off-fh.off) <= 16 && lines(rs, min(fg.off, fh.off), max(fg.off, fh.off)) <= 1
	switch {
	case near && sameDecl:
		return "near"
	case fg.off > fh.off && rg == rh && reports(h, fg.off):
		return "shadowed"
	case fg.off > fh.off && isHint(fh.msg) && (rg == rh || reports(h, fg.off)):
		return "hint"
	case fg.off > fh.off && rg == rh && fg.off <= end && strings.TrimSpace(string(rs[fg.off:end])) == "":
		return "end"
	case fg.off < fh.off && declarationHead(rs, fg.off) && len(openAt(rs, fg.off)) > 0:
		return "head"
	case fg.off < fh.off && isHint(fh.msg) && rg == rh:
		return "late"
	case fg.off > fh.off && enclosed(rs, fh.off, fg.off):
		return "unclosed"
	case fg.off > fh.off && rg == rh:
		return "dropped"
	}
	return fmt.Sprintf("other %s %s", map[bool]string{true: "earlier", false: "later"}[fg.off < fh.off], bucket(abs(fg.off-fh.off)))
}

// reports reports whether one of errs is at off.
func reports(errs []errorAt, off int) bool {
	for _, e := range errs {
		if e.off == off {
			return true
		}
	}
	return false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// declarationHead reports whether off is in or just after a word rule, type or var, or in the blanks
// after it, and the word starts a declaration in the sense of the decl_shape rule of the grammar: a bare =
// or ~ before the end of the line, a comment or a ; { } ( or ) (an operator such as := or == does not
// count), or the end of the source after at most a name. The word is not required to start a line: the
// recovery of a field or an item list ends the list at one that follows a brace.
func declarationHead(rs []rune, off int) bool {
	isIdent := func(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }
	blank := func(r rune) bool { return r == ' ' || r == '\t' }
	// A word is the one that off is in or at the start of, or the one before the blanks that off is in.
	wordStart := func(pos int) int {
		for pos > 0 && isIdent(rs[pos-1]) {
			pos--
		}
		return pos
	}
	blanks := off
	for blanks > 0 && blank(rs[blanks-1]) {
		blanks--
	}
	for _, start := range []int{wordStart(min(off, len(rs))), wordStart(blanks)} {
		if declarationAt(rs, start, off) {
			return true
		}
	}
	return false
}

// declarationAt reports whether the word at start is rule, type or var with the shape of a declaration,
// and off is in the word or in the blanks after it.
func declarationAt(rs []rune, start, off int) bool {
	isIdent := func(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }
	blank := func(r rune) bool { return r == ' ' || r == '\t' }
	for _, kw := range []string{"rule", "type", "var"} {
		end := start + len(kw)
		if end > len(rs) || string(rs[start:end]) != kw || end < len(rs) && isIdent(rs[end]) {
			continue
		}
		i := end
		for i < len(rs) && blank(rs[i]) {
			i++
		}
		shaped := false
		if i == len(rs) {
			shaped = true
		} else if i > end && rs[i] != ':' && rs[i] != '/' {
		scan:
			for j := i; j < len(rs) && rs[j] != '\n' && rs[j] != '\r'; j++ {
				switch {
				case rs[j] == '/' && j+1 < len(rs) && rs[j+1] == '/':
					break scan
				case strings.ContainsRune(";{}()", rs[j]):
					break scan
				case rs[j] == '=' && j+1 < len(rs) && (rs[j+1] == '=' || rs[j+1] == '>'):
					j++
				case rs[j] == '=' || rs[j] == '~':
					shaped = true
					break scan
				case strings.ContainsRune("!<>:", rs[j]) && j+1 < len(rs) && rs[j+1] == '=':
					j++
				}
			}
		}
		if !shaped && i > end {
			// A name and then the end of the source.
			j := i
			for j < len(rs) && isIdent(rs[j]) {
				j++
			}
			for j < len(rs) && blank(rs[j]) {
				j++
			}
			shaped = j == len(rs)
		}
		return shaped && off >= start && off <= i
	}
	return false
}

// lines returns the number of line breaks in rs[a:b].
func lines(rs []rune, a, b int) int {
	return strings.Count(string(rs[a:b]), "\n")
}

// openAt returns the offsets of the braces and parentheses that are open before b, outermost first.
func openAt(rs []rune, b int) []int {
	var open []int
	for i := 0; i < b && i < len(rs); i++ {
		switch c := rs[i]; {
		case c == '"':
			for i++; i < len(rs) && rs[i] != '"' && rs[i] != '\n'; i++ {
				if rs[i] == '\\' {
					i++
				}
			}
		case c == '/' && i+1 < len(rs) && rs[i+1] == '/':
			for i < len(rs) && rs[i] != '\n' && rs[i] != '\r' {
				i++
			}
		case c == '/' && i+1 < len(rs) && rs[i+1] == '*':
			for i += 2; i+1 < len(rs) && !(rs[i] == '*' && rs[i+1] == '/'); i++ {
			}
			i++
		case c == '{' || c == '(':
			open = append(open, i)
		case (c == '}' || c == ')') && len(open) > 0 && rs[open[len(open)-1]] == map[rune]rune{'}': '{', ')': '('}[c]:
			open = open[:len(open)-1]
		}
	}
	return open
}

// enclosed reports whether a brace or parenthesis that is open at off a is still open at off b, after a:
// the error at a is inside a construct that the source does not close before b.
func enclosed(rs []rune, a, b int) bool {
	open := openAt(rs, b)
	return len(open) > 0 && open[0] < a
}

// related reports whether the errors at a and b are inside one construct that the source does not close
// between them.
func related(rs []rune, a, b int) bool {
	if a > b {
		a, b = b, a
	}
	return enclosed(rs, a, b)
}

// TestErrorPositionClasses pins one source of each class that the mutation tiers count, so that a change
// of the classification, of the grammar or of the reference parser that moves an error from one class to
// another shows here, and not only in a share of the mutated sources.
func TestErrorPositionClasses(t *testing.T) {
	p, _ := sourceGrammar(t)
	cases := []struct{ class, src string }{
		{"same", "rule main= _t\nrule _t = "},
		{"near", "rule x = #operators { opera"},
		{"near", "rule a = \"x\" #cut(1, 2) \"y\"\n"},
		{"near", "rule a = b #atomic #stream c #missing d #recover(skip=x) e #repeat(2) f {repeat(1,3) g #repeat(,2) h\n"},
		{"end", "rule x = \"a\" => {NA: 1, // a1\r\n // lead b\r\n B: 2 /"},
		{"end", "rule a = {x == 1 && y {z := 1; w} => ...\r\n"},
		{"same", "@start(\n var/ in\n default, // d.)\nrule a = \"x\""},
		{"same", "@start(\n // in\n default, // d\n\nrule#recover(skip= a = \"x\""},
		{"head", "type A = { type strtypeing  ~ A\n"},
		{"hint", "rule ops string = #operators {\n    skip (\" \" / \"#\" [^#]* #recover(skip=[^ ]*))*\n    operand d:([0-9]+ #reco-z]*)) => \"n\"\n    operand \"(\" e:ops \")\" => \"(\" + $e + \")\"\n"},
		{"late", "type T = node[str,ing] // n\n// ext\ntype U ~ T"},
		{"shadowed", "rule a = #operators {\n oper b /* b */ => /* c */ N{} /* d */\n level { infix /* e */ left /* f */ \"+\" /* g */ }\né\n"},
		{"dropped", "rule x = #operators { opernd \"1\" } /* a */ ignorecase /* b */ => ...\n"},
		{"dropped", "rule e = #operators {\n operand \"1\"\n level a {... prefix \"-\" ;\n infix left \"+\" ; infix right \"^\"\n}\n} => ...=>\n"},
		{"unclosed", "type R = { S string; N int; T string) =>\nvar k1 int\nrule main R = {k1 := 1} a:\"x\"\n"},
	}
	for _, c := range cases {
		_, merr := zp.ParseGrammar(c.src)
		_, err := adaptCanonical(p, c.src)
		h, g := errorsOf(c.src, merr), errorsOf(c.src, err)
		if len(h) == 0 || len(g) == 0 {
			t.Errorf("%q: both frontends must reject it: %v, %v", c.src, merr, err)
			continue
		}
		rs := []rune(c.src)
		if got := classify(rs, declarationStarts(rs), h, first(h), first(g)); got != c.class {
			t.Errorf("%q: the class is %q, want %q\n  hand-written: %v\n  canonical:    %v", c.src, got, c.class, merr, err)
		}
	}
}

// TestUneditedErrorIsExtra pins the rule that an error on a declaration which the edits did not touch,
// where the reference reports none and nothing is open, is counted as extra, also at the start of that
// declaration; and that the only error for a broken declaration, at the start of the next, is not.
func TestUneditedErrorIsExtra(t *testing.T) {
	src := "rule a = x\nrule b = y\nrule c = z\n"
	rs := []rune(src)
	starts := declarationStarts(rs)
	b, c := strings.Index(src, "rule b"), strings.Index(src, "rule c")
	edits := [][2]int{{8, 9}} // in the first declaration
	href := []errorAt{{off: 9, msg: "ref"}}
	for _, tc := range []struct {
		name string
		g    []errorAt
		e    errorAt
		want bool
	}{
		{"inside an untouched declaration", []errorAt{{off: 9}, {off: b + 5}}, errorAt{off: b + 5}, true},
		{"at the start of an untouched one, with another error in the broken one", []errorAt{{off: 9}, {off: b}}, errorAt{off: b}, true},
		{"the only error for the broken one, at the start of the next", []errorAt{{off: b}}, errorAt{off: b}, false},
		{"in the broken declaration", []errorAt{{off: 9}}, errorAt{off: 9}, false},
		{"at the start of an untouched one, whose predecessor is untouched", []errorAt{{off: c}}, errorAt{off: c}, true},
	} {
		if got := unedited(rs, starts, edits, href, tc.g, tc.e); got != tc.want {
			t.Errorf("%s: unedited = %v, want %v", tc.name, got, tc.want)
		}
	}
}
