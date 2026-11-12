package language_test

import (
	"strings"
	"testing"
	"unicode"

	"zeroparse.dev/zp/internal/syntax"
	"zeroparse.dev/zp/internal/syntax/genparser"
)

// The positions of the errors of the canonical frontend come from SourceErrors, which moves the
// offset where the generated parser failed to the token where the hand-written parser reports.
// These controls run the corpora of the error comparison through other ways to place the errors
// and require the comparison to see the difference, so that a change to SourceErrors that
// the corpora cannot tell apart from the right one is found.

// rawErrors returns the errors that the generated parser returns for src, with its offsets.
func rawErrors(src string, err error) []syntax.SourceError {
	errs, _ := sourceErrorsOf(err)
	return errs
}

// placements are other ways to put an error at the offset where the generated parser failed.
// Each returns the offset that stands for the error. The first returns the offset itself.
var placements = []struct {
	name string
	// why is the property of the sources that only this placement gets wrong.
	why string
	// through is whether SourceErrors places the moved offset, as it does one that it is given.
	through bool
	move    func(rs []rune, off int) int
}{
	{"offset itself", "the generated parser fails inside a token", false, func(rs []rune, off int) int { return off }},
	{"end of a string or class", "the generated parser fails inside a string or a class", true, func(rs []rune, off int) int {
		// A string or a class that starts before the offset and ends after it: the offset moves past it.
		for i := 0; i < len(rs); i++ {
			var closer rune
			switch rs[i] {
			case '"':
				closer = '"'
			case '[':
				closer = ']'
			case '/':
				if i+1 < len(rs) && rs[i+1] == '/' {
					for i < len(rs) && rs[i] != '\n' {
						i++
					}
				}
				continue
			default:
				continue
			}
			j := i + 1
			for j < len(rs) && rs[j] != closer && rs[j] != '\n' {
				if rs[j] == '\\' {
					j++
				}
				j++
			}
			if i < off && off <= j && j < len(rs) && rs[j] == closer {
				return j + 1
			}
			i = j
		}
		return off
	}},
	{"text after the offset", "the generated parser fails inside a comment that the lexer reads as white space", false, func(rs []rune, off int) int {
		// The next token found by reading the text from the offset, comments included.
		for off < len(rs) {
			switch {
			case unicode.IsSpace(rs[off]):
				off++
			case rs[off] == '/' && off+1 < len(rs) && rs[off+1] == '/':
				for off < len(rs) && rs[off] != '\n' {
					off++
				}
			case rs[off] == '/' && off+1 < len(rs) && rs[off+1] == '*':
				end := strings.Index(string(rs[off+2:]), "*/")
				if end < 0 {
					return len(rs)
				}
				off += 2 + len([]rune(string(rs[off+2:])[:end])) + 2
			default:
				return off
			}
		}
		return off
	}},
}

// firstOffsetOf returns the offset of the first error of a list in the form of the frontends.
func firstOffsetOf(src string, err error) (int, bool) {
	errs := errorsOf(src, err)
	if len(errs) == 0 {
		return 0, false
	}
	return first(errs).off, true
}

// TestPositionPlacementsAreSeen runs, for the sources that both frontends reject with the same
// first error, each other placement and requires that some source gets a different first error.
func TestPositionPlacementsAreSeen(t *testing.T) {
	seen := make([]int, len(placements))
	example := make([]string, len(placements))
	for _, src := range rejectedSources(t) {
		_, merr := syntax.ParseReference(src)
		text, invalid := syntax.WithoutInvalidCharacters(src)
		if invalid || merr == nil {
			continue
		}
		_, perr := genparser.ParseAST(text)
		if perr == nil {
			continue
		}
		want, ok := firstOffsetOf(src, merr)
		if !ok {
			continue
		}
		if got, _ := firstOffsetOf(src, canonicalErrors(src, perr)); got != want {
			continue
		}
		rs := []rune(src)
		raw := rawErrors(src, perr)
		for i, pl := range placements {
			var errs []errorAt
			if pl.through {
				moved := make([]syntax.SourceError, len(raw))
				for j, e := range raw {
					moved[j] = syntax.SourceError{Offset: pl.move(rs, e.Offset), Msg: e.Msg}
				}
				errs = errorsOf(src, syntax.SourceErrors(src, moved))
			} else {
				for _, e := range raw {
					errs = append(errs, errorAt{pl.move(rs, e.Offset), e.Msg})
				}
			}
			if first(errs).off != want {
				seen[i]++
				if example[i] == "" {
					example[i] = src
				}
			}
		}
	}
	for i, pl := range placements {
		if seen[i] == 0 {
			t.Errorf("no source tells %q from SourceErrors: %s", pl.name, pl.why)
		} else {
			t.Logf("%s: %d sources, for example %q", pl.name, seen[i], example[i])
		}
	}
}
