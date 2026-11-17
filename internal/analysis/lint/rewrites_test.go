package lint

import (
	"context"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// applyRewrite applies r to src, whose characters are all single bytes.
func applyRewrite(src string, r Rewrite) string {
	lines := strings.SplitAfter(src, "\n")
	offset := func(p ast.Pos) int {
		n := 0
		for _, l := range lines[:p.Line-1] {
			n += len(l)
		}
		return n + p.Col - 1
	}
	return src[:offset(r.Start)] + r.NewText + src[offset(r.End):]
}

// rewrites lints src from main and returns the source after the rewrite of each finding of check
// that has one, in the order of the findings.
func rewrites(t *testing.T, src, check string) (fixed []string, findings int) {
	t.Helper()
	g, err := parser.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	fs := runUnchecked(g, "main", options{})
	rs, err := rewritesOf(context.Background(), checked(g), g, fs)
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range fs {
		if f.Check != check {
			continue
		}
		findings++
		if rs[i].IsValid() {
			fixed = append(fixed, applyRewrite(src, rs[i]))
		}
	}
	return fixed, findings
}

func TestRewrites(t *testing.T) {
	for _, tc := range []struct {
		name, src, check string
		want             []string // the source after each rewrite; nil when the finding has none
		findings         int
	}{
		{"overlapping ranges", "rule main = [a-cb-z] \"x\"", CheckCharClass, []string{"rule main = [a-z] \"x\""}, 1},
		{"repeated range", "rule main = [aa0-9]", CheckCharClass, []string{"rule main = [0-9a]"}, 1},
		{"negated class", "rule main = [^a-cb-z]", CheckCharClass, []string{"rule main = [^a-z]"}, 1},
		{"in an inline action", "rule main = ([a-cb-z] => \"x\")", CheckCharClass, []string{"rule main = ([a-z] => \"x\")"}, 1},
		{"a range that spans punctuation", "rule main = [A-z]", CheckCharClass, nil, 1},
		{"a lookahead that always succeeds", "rule main = \"a\" &\"b\"* \"c\"", CheckUselessLookahead, []string{"rule main = \"a\"  \"c\""}, 1},
		{"a lookahead under a discard", "rule main = g:(\"a\" - &\"b\"* \"c\") => len($g.Value)", CheckUselessLookahead, nil, 1},
		{"a lookahead under a discard before a literal", "rule main = \"a\" - &\"b\"* \"c\" => \"k\"", CheckUselessLookahead, nil, 1},
		{"a lookahead in a sequence of two", "rule main = &\"b\"* \"c\"", CheckUselessLookahead, nil, 1},
		{"a lookahead in a sequence of two in an inline action", "rule main = (g:(&\"b\"* \"cc\") => len($g.Value)) \"d\"", CheckUselessLookahead, nil, 1},
		{"a lookahead in a group", "rule main = (\"a\" (&\"b\"*) \"c\")", CheckUselessLookahead, nil, 1},
		{"a lookahead with a call", "rule main = &w* \"c\"\nrule w = \"a\"", CheckUselessLookahead, nil, 1},
		{"a lookahead that never succeeds", "rule main = !\"a\"? \"c\"", CheckUselessLookahead, nil, 1},
		{"an unreachable rule", "rule main = \"a\"\nrule x = \"b\"\nrule y = \"c\"", CheckUnreachableRule,
			[]string{"rule main = \"a\"\n\nrule y = \"c\"", "rule main = \"a\"\nrule x = \"b\"\n"}, 2},
		{"a rule that another unreachable rule calls", "rule main = \"a\"\nrule x = y\nrule y = \"c\"", CheckUnreachableRule,
			[]string{"rule main = \"a\"\n\nrule y = \"c\"", "rule main = \"a\"\nrule x = y\n"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, findings := rewrites(t, tc.src, tc.check)
			if findings != tc.findings || strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("findings=%d rewrites=%q, want %d and %q", findings, got, tc.findings, tc.want)
			}
		})
	}
}

func TestSameCodePoints(t *testing.T) {
	r := func(lo, hi rune) ast.CharRange { return ast.CharRange{Lo: lo, Hi: hi} }
	for _, tc := range []struct {
		name string
		x, y []ast.CharRange
		want bool
	}{
		{"merged overlap", []ast.CharRange{r('a', 'c'), r('b', 'z')}, []ast.CharRange{r('a', 'z')}, true},
		{"adjacent ranges", []ast.CharRange{r('a', 'c'), r('d', 'f')}, []ast.CharRange{r('a', 'f')}, true},
		{"a gap closed", []ast.CharRange{r('a', 'c'), r('e', 'f')}, []ast.CharRange{r('a', 'f')}, false},
		{"a range cut short", []ast.CharRange{r('a', 'z')}, []ast.CharRange{r('a', 'y')}, false},
		{"a range grown", []ast.CharRange{r('b', 'y')}, []ast.CharRange{r('a', 'y')}, false},
		{"a hole inside", []ast.CharRange{r('a', 'z')}, []ast.CharRange{r('a', 'f'), r('h', 'z')}, false},
		{"a single code point added", []ast.CharRange{r('a', 'c')}, []ast.CharRange{r('a', 'c'), r('x', 'x')}, false},
	} {
		if got := sameCodePoints(tc.x, tc.y, nil); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}
