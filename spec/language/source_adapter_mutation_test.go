package language_test

import (
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp"
)

// mutationTokens are the pieces a mutation inserts: punctuation, keywords,
// escapes and comments that decide how the grammar is read.
var mutationTokens = []string{
	"(", ")", "{", "}", "[", "]", "[]", "=", "=>", "\"", "'", "@", "#", "/", "//", "/*", "*/", ";", ",", ":", ":=", "$", "$$", "^", ".",
	"-", "--", "*", "+", "?", "&", "!", "|", "~", "_", "_|_", "\n", "\r\n", " ", "\\", "\\u{110000}", "\\x", "\\u12", "9223372036854775808",
	"rule", "type", "var", "package", "level", "skip", "operand", "prefix", "postfix", "infix", "left", "node", "true", "nil", "#cut",
	"#operators", "#repeat(", "#recover(skip=", "#atomic", "#missing", "#ignorecase", "@start", "...", "level(a, b)", "x.y", "x.$y", "(a) => ",
	// Characters outside ASCII: letters and symbols that start no token, a no-break space, a zero-width
	// space, and a letter of two code units.
	"é", "日本", "→", "\u00a0", "\u200b", "\U0001f600",
}

// mutate applies one edit to src at a pseudo-random place.
func mutate(rng *rand.Rand, src []rune) []rune {
	if len(src) == 0 {
		return src
	}
	k := rng.Intn(len(src))
	out := append([]rune(nil), src[:k]...)
	switch rng.Intn(6) {
	case 0: // delete a rune
		out = append(out, src[k+1:]...)
	case 1: // insert a token
		out = append(out, []rune(mutationTokens[rng.Intn(len(mutationTokens))])...)
		out = append(out, src[k:]...)
	case 2: // truncate
	case 3: // replace a rune with a token
		out = append(out, []rune(mutationTokens[rng.Intn(len(mutationTokens))])...)
		out = append(out, src[k+1:]...)
	case 4: // delete a range
		end := min(len(src), k+1+rng.Intn(12))
		out = append(out, src[end:]...)
	case 5: // swap two runes
		if k+1 < len(src) {
			out = append(out, src[k+1], src[k])
			out = append(out, src[k+2:]...)
		} else {
			out = append(out, src[k:]...)
		}
	}
	return out
}

// mutationCorpus returns grammar sources to mutate: the repository grammars,
// the documentation examples and the corpora of the parity tests.
func mutationCorpus(t *testing.T) []string {
	t.Helper()
	corpus := append(append(append([]string(nil), ownershipCorpus...), nodeBases...), keywordNameCorpus...)
	corpus = append(corpus, keywordFormCorpus(100)...)
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != "../.." && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".zp" {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			corpus = append(corpus, string(b))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return corpus
}

// declarationStarts returns the offsets of the lines of src that start a declaration: with rule, type,
// var or package and a space, or with an attribute.
func declarationStarts(src []rune) []int {
	var starts []int
	prev := "" // the text of the line before, without blanks
	for i := 0; i < len(src); i++ {
		if i > 0 && src[i-1] != '\n' {
			continue
		}
		end := i
		for end < len(src) && src[end] != '\n' {
			end++
		}
		line := strings.TrimSpace(string(src[i:end]))
		rest := string(src[i:min(len(src), i+8)])
		for _, kw := range []string{"rule ", "type ", "var ", "package ", "@"} {
			// The attributes of a rule and the rule are one declaration. The package clause is the first one.
			if strings.HasPrefix(rest, kw) && !(kw == "rule " && strings.HasPrefix(prev, "@")) && !(kw == "package " && len(starts) > 0) {
				starts = append(starts, i)
				break
			}
		}
		if line != "" && !strings.HasPrefix(line, "//") {
			prev = line
		}
	}
	return starts
}

// mutatedSource returns a window of whole declarations of a grammar of the corpus, up to about 400
// code points, with one or two edits. A quarter of the windows use CRLF line breaks.
func mutatedSource(rng *rand.Rand, corpus []string) string {
	src, _ := mutatedSourceEdits(rng, corpus)
	return src
}

// mutatedSourceEdits is mutatedSource and the code point ranges of the result that the edits changed. An
// error on a declaration that no range touches, and that the reference reports no error in, is on text
// that the edits left as it was.
func mutatedSourceEdits(rng *rand.Rand, corpus []string) (string, [][2]int) {
	src := []rune(corpus[rng.Intn(len(corpus))])
	if len(src) > 400 {
		starts := declarationStarts(src)
		lo := rng.Intn(len(src) - 400)
		hi := lo + 400
		if len(starts) > 0 {
			lo = starts[rng.Intn(len(starts))]
			hi = lo + 400 // one declaration longer than the window is cut
			for _, st := range starts {
				if st > lo && st <= lo+400 {
					hi = st
				}
			}
		}
		src = src[lo:min(hi, len(src))]
	}
	if rng.Intn(4) == 0 {
		src = []rune(strings.ReplaceAll(string(src), "\n", "\r\n"))
	}
	var edits [][2]int
	for range 1 + rng.Intn(2) {
		before := slices.Clone(src)
		src = mutate(rng, src)
		edits = noteEdit(edits, before, src)
	}
	return string(src), edits
}

// noteEdit returns edits, which are ranges of before, moved to the ranges of after, and the range that
// differs between the two (the longest common prefix and suffix are outside it).
func noteEdit(edits [][2]int, before, after []rune) [][2]int {
	p := 0
	for p < len(before) && p < len(after) && before[p] == after[p] {
		p++
	}
	q := 0
	for q < len(before)-p && q < len(after)-p && before[len(before)-1-q] == after[len(after)-1-q] {
		q++
	}
	oldEnd, newEnd := len(before)-q, len(after)-q
	d := newEnd - oldEnd
	var out [][2]int
	lo, hi := p, newEnd
	for _, r := range edits {
		switch {
		case r[1] <= p:
			out = append(out, r)
		case r[0] >= oldEnd:
			out = append(out, [2]int{r[0] + d, r[1] + d})
		default:
			lo, hi = min(lo, r[0]), max(hi, r[1]+d)
		}
	}
	return append(out, [2]int{lo, hi})
}

// rejectedSources returns sources for the controls of the error positions: those that the
// canonical frontend must reject, and mutated grammars, most of which both frontends reject.
func rejectedSources(t *testing.T) []string {
	t.Helper()
	out := append([]string(nil), frontendRejections...)
	corpus := mutationCorpus(t)
	rng := rand.New(rand.NewSource(11))
	for i := 0; i < 1500; i++ {
		out = append(out, mutatedSource(rng, corpus))
	}
	return out
}

// TestCanonicalFrontendMutations mutates grammars at random and requires the
// canonical frontend to accept what the hand-written parser accepts and build
// the same grammar, for each seed of the tier. A window of whole declarations
// keeps each case fast. For the sources that both frontends reject it counts
// how the errors differ, as harmReport tells.
func TestCanonicalFrontendMutations(t *testing.T) {
	p, _ := sourceGrammar(t)
	corpus := mutationCorpus(t)
	for _, seed := range mutationSeeds {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			failed := 0
			accepted := 0 // sources the hand-written parser accepts
			stats := newHarmStats()
			for i := 0; i < mutationCases; i++ {
				s, edits := mutatedSourceEdits(rng, corpus)
				if _, err := zp.ParseGrammar(s); err == nil {
					accepted++
				}
				if !compareGrammars(t, p, fmt.Sprintf("mutation %d (%q)", i, s), s) {
					failed++
					if failed >= 8 {
						t.Fatal("too many differences")
					}
				}
				stats.add(t, p, s, edits)
			}
			// A comparison of sources that are all rejected shows nothing about the AST.
			if accepted < mutationCases/20 {
				t.Fatalf("only %d of %d mutated sources are accepted", accepted, mutationCases)
			}
			t.Logf("%d of %d mutated sources are accepted", accepted, mutationCases)
			stats.report(t, mutationCases)
			stats.check(t)
		})
	}
}
