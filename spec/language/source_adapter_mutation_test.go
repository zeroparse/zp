package language_test

import (
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
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
	corpus := append(append([]string(nil), ownershipCorpus...), nodeBases...)
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

// TestCanonicalFrontendMutations mutates grammars at random and requires the
// canonical frontend to accept what the hand-written parser accepts and build
// the same grammar. A small window around the edit keeps each case fast.
func TestCanonicalFrontendMutations(t *testing.T) {
	p, _ := sourceGrammar(t)
	corpus := mutationCorpus(t)
	rng := rand.New(rand.NewSource(7))
	failed := 0
	accepted := 0 // sources the hand-written parser accepts
	for i := 0; i < mutationCases; i++ {
		src := []rune(corpus[rng.Intn(len(corpus))])
		lo := 0
		if len(src) > 400 {
			lo = rng.Intn(len(src) - 400)
			src = src[lo : lo+400]
		}
		for range 1 + rng.Intn(2) {
			src = mutate(rng, src)
		}
		s := string(src)
		if _, err := zp.ParseGrammar(s); err == nil {
			accepted++
		}
		if !compareGrammars(t, p, fmt.Sprintf("mutation %d (%q)", i, s), s) {
			failed++
			if failed >= 8 {
				t.Fatal("too many differences")
			}
		}
	}
	// A comparison of sources that are all rejected shows nothing about the AST.
	if accepted < mutationCases/20 {
		t.Fatalf("only %d of %d mutated sources are accepted", accepted, mutationCases)
	}
	t.Logf("%d of %d mutated sources are accepted", accepted, mutationCases)
}
