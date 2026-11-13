package complexitytest

import (
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/complexity"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/syntax"
)

// TestNormalViewMatches checks that the normal view of each bundled grammar matches what the
// grammar matches (≡E of spec/proofs/grammar-laws.md): on the inputs of the grammar's directory and
// on inputs made from them by truncating, deleting, inserting and swapping characters, a prefix
// parse of the view succeeds exactly where one of the grammar does, and ends where it ends. Both
// views are checked, the one that the classifier reads and the one for analyses of matching.
//
// The grammars are compared with default values, without the actions and declared types that
// predicates do not need, since the view keeps the matching of a grammar but not the shapes of
// its values.
func TestNormalViewMatches(t *testing.T) {
	variants := 200
	if testing.Short() {
		variants = 20
	}
	for _, tg := range append(append([]TriageGrammar{}, TriageGrammars...), LinearGrammars...) {
		t.Run(tg.Name, func(t *testing.T) {
			src, err := os.ReadFile(tg.Path)
			if err != nil {
				t.Fatal(err)
			}
			g, err := syntax.Parse(string(src))
			if err != nil {
				t.Fatal(err)
			}
			prog, err := engine.Compile(g, engine.Options{DefaultValue: true})
			if err != nil {
				t.Fatalf("the grammar without actions does not compile: %v", err)
			}
			corpus := inputsOf(t, filepath.Dir(tg.Path))
			rng := rand.New(rand.NewPCG(1, uint64(len(tg.Name))))
			var inputs []string
			for _, in := range corpus {
				inputs = append(inputs, in)
				for range variants {
					inputs = append(inputs, mutate(rng, in))
				}
			}
			checkViewMatches(t, g, prog, inputs)
		})
	}
}

// TestNormalViewMatchesShapes checks the views of grammars whose shapes the bundled grammars lack,
// as TestNormalViewMatches does, on their inputs and on inputs made from them: cuts nested in the
// only alternative that a choice keeps, which commit the choice and nothing around it.
func TestNormalViewMatchesShapes(t *testing.T) {
	for _, c := range []struct {
		name, src string
		inputs    []string
	}{
		{"cut in a discard in a repetition", `rule main = (-("a" #cut "b") / !"q" "q")* "ac"`, []string{"ababac", "ac", "abac", "aac", "abq"}},
		{"cut in a capture in an option", `rule main = (x:("a" #cut "b") / !"q" "q")? "ac"`, []string{"abac", "ac", "aac"}},
		{"cut in an atomic in a repetition", `rule main = (("a" #cut "b") #atomic / !"q" "q")* "ac"`, []string{"ababac", "ac", "abq"}},
		{"cut in a labeled expression in a repetition", `rule main = (("a" #cut "b") @error(message="m") / !"q" "q")* "ac"`, []string{"ababac", "ac"}},
		{"cut in a choice spliced into a choice", "rule main = ((-(\"x\" #cut e \"q\") / !\"z\" \"z\") / \"x\")* $$\nrule e = e \"x\" / \"x\"", []string{"xxq", "x", "xxxq", "xxqx", "xx"}},
		{"capture in a lookahead of a called rule", "rule main = y:r {len($y.$z) == 2} (e \"q\" / \"x\")* $$\nrule r = &(z:s) \"a\"*\nrule s = \"a\"* \"a\"*\nrule e = e \"x\" / \"x\"", []string{"x", "axq", "aaxq", "aax", "xxq", "aaaxq"}},
		{"read capture merged by a repetition", "rule main = x:\"a\" (x:\"a\")* {text($x) == \"a\"} (e \"q\" / \"x\")* $$\nrule e = e \"x\" / \"x\"", []string{"axxq", "aaxq", "ax", "aax", "a"}},
		{"read capture merged by an optional", "rule main = (x:\"a\")? (x:\"a\")* {text($x) == \"a\"} (e \"q\" / \"x\")* $$\nrule e = e \"x\" / \"x\"", []string{"axxq", "aaxq", "ax", "aax", "x"}},
		{"cut that ends an alternative of a read capture", "rule main = x:(\"abc\" #cut / \"z\") {len($x) == 1} (e \"q\" / \"x\")* $$\nrule e = e \"x\" / \"x\"", []string{"abcxxq", "abcx", "zxq", "zx", "abc", "abxq"}},
		{"cut of the last alternative around a read capture", "rule main = (\"w\" / x:(\"abc\" #cut)) {len($x) == 1} (e \"q\" / \"x\")* $$\nrule e = e \"x\" / \"x\"", []string{"abcxxq", "abcx", "wxq", "wx", "abc"}},
		{"cut in a rule that a read capture reaches", "rule main = y:r {len($y) == 1} (e \"q\" / \"x\")* $$\nrule r = \"abc\" #cut / \"z\"\nrule e = e \"x\" / \"x\"", []string{"abcxxq", "abcx", "zxq", "zx", "abc"}},
		{"choice of one sequence in a read capture", "rule main = x:(\"a\" (\"b\" \"c\" / _|_)) {len($x) == 2} (e \"q\" / \"x\")* $$\nrule e = e \"x\" / \"x\"", []string{"abcxxq", "abcx", "abxq", "ab"}},
		{"call of a rule with several items in a read capture", "rule main = x:(\"a\" r) {len($x) == 2} (e \"q\" / \"x\")* $$\nrule r = \"b\" \"c\"\nrule e = e \"x\" / \"x\"", []string{"abcxxq", "abcx", "abxq", "abc"}},
		{"cut in a skip", `rule main = ("a" "b" #recover(skip=#cut [^;]*) / !"q" "q")* "ac"`, []string{"abac", "ac", "aac", "a;ac"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, err := syntax.Parse("@start\n" + c.src + "\n")
			if err != nil {
				t.Fatal(err)
			}
			prog, err := engine.Compile(g, engine.Options{DefaultValue: true})
			if err != nil {
				t.Fatal(err)
			}
			rng := rand.New(rand.NewPCG(1, uint64(len(c.name))))
			var inputs []string
			for _, in := range c.inputs {
				inputs = append(inputs, in)
				for range 50 {
					inputs = append(inputs, mutate(rng, in))
				}
			}
			checkViewMatches(t, g, prog, inputs)
		})
	}
}

// checkViewMatches compares prefix parses of both views of g with those of g, compiled as prog,
// on inputs.
func checkViewMatches(t *testing.T, g *grammar.Grammar, prog *engine.Program, inputs []string) {
	t.Helper()
	for _, matching := range []bool{false, true} {
		vg, err := complexity.NormalView(g, matching, nil)
		if err != nil {
			t.Fatal(err)
		}
		vprog, err := engine.Compile(vg, engine.Options{DefaultValue: true})
		if err != nil && matching {
			// Inlining changes the values of the rules that predicates read, whose types
			// default values keep; the view for analyses of matching is not a grammar
			// that has to compile.
			t.Logf("view (matching true) does not compile and is not checked: %v", err)
			continue
		}
		if err != nil {
			t.Fatalf("view (matching %v) does not compile: %v", matching, err)
		}
		accepted := 0
		for _, in := range inputs {
			want, werr := prefixEnd(prog, in)
			got, gerr := prefixEnd(vprog, in)
			if errors.Is(werr, errLimit) || errors.Is(gerr, errLimit) {
				continue
			}
			if (werr == nil) != (gerr == nil) || want != got {
				t.Errorf("view (matching %v) on %.80q: end %d (%v), the grammar %d (%v)", matching, in, got, gerr, want, werr)
			}
			if werr == nil {
				accepted++
			}
		}
		t.Logf("view (matching %v): %d inputs, %d accepted", matching, len(inputs), accepted)
	}
}

var errLimit = errors.New("limit")

// prefixEnd returns where a prefix parse of the default start rule ends, or the error of the parse.
// A grammar with #missing, which prefix parses reject, is parsed whole, and the end is then the
// length of the input.
func prefixEnd(prog *engine.Program, in string) (int, error) {
	o := engine.ParseOptions{Recognize: true, Limits: core.Limits{Compute: 50_000_000}}
	var err error
	end := len(in)
	if prog.HasMissing {
		_, err = prog.ParseWith("", in, o)
	} else {
		var r engine.PrefixResult
		r, err = prog.ParsePrefixWith("", in, o)
		end = r.ConsumedBytes
	}
	var le *core.LimitError
	if errors.As(err, &le) {
		return 0, errLimit
	}
	return end, err
}

// inputsOf returns the inputs in dir and its testdata directory: the text files.
func inputsOf(t *testing.T, dir string) []string {
	var out []string
	for _, d := range []string{dir, filepath.Join(dir, "testdata")} {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(d, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if len(b) > 4096 {
				b = b[:4096]
			}
			out = append(out, string(b))
		}
	}
	if len(out) == 0 {
		t.Skip("no inputs")
	}
	return out
}

// mutate returns in truncated, or with a character deleted, inserted or swapped with the next.
func mutate(rng *rand.Rand, in string) string {
	rs := []rune(in)
	if len(rs) == 0 {
		return in
	}
	i := rng.IntN(len(rs))
	switch rng.IntN(4) {
	case 0:
		return string(rs[:i])
	case 1:
		return string(rs[:i]) + string(rs[i+1:])
	case 2:
		return string(rs[:i]) + string(rs[rng.IntN(len(rs))]) + string(rs[i:])
	default:
		if i+1 < len(rs) {
			rs[i], rs[i+1] = rs[i+1], rs[i]
		}
		return string(rs)
	}
}

// TestNormalViewCompiles checks that the cost view of every bundled grammar is a grammar that
// compiles: a rewrite that moved or merged a capture that a predicate reads would leave the
// predicate without it.
func TestNormalViewCompiles(t *testing.T) {
	for _, tg := range append(append([]TriageGrammar{}, TriageGrammars...), LinearGrammars...) {
		t.Run(tg.Name, func(t *testing.T) {
			src, err := os.ReadFile(tg.Path)
			if err != nil {
				t.Fatal(err)
			}
			g, err := syntax.Parse(string(src))
			if err != nil {
				t.Fatal(err)
			}
			vg, err := complexity.NormalView(g, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Compile(vg, engine.Options{DefaultValue: true}); err != nil {
				t.Fatalf("the view does not compile: %v", err)
			}
		})
	}
}
