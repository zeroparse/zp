package syntax

import (
	"fmt"
	"strings"
	"testing"
)

// nestingFamilies make sources whose nesting, as the lexer counts it, is n levels. Each is valid at
// any n, so the source of maxLevels levels must parse and the one of a level more must fail with
// one error, "nesting too deep", at the token that opens the extra level, whose code point offset
// is at(n+1).
var nestingFamilies = []struct {
	name string
	src  func(n int) string
	at   func(n int) int // the offset of the token that opens level n
}{
	{"parentheses", func(n int) string { return "rule a = " + strings.Repeat("(", n) + "x" + strings.Repeat(")", n) },
		func(n int) int { return 9 + n - 1 }},
	{"struct types", func(n int) string { return "type T = " + strings.Repeat("{A ", n) + "string" + strings.Repeat("}", n) },
		func(n int) int { return 9 + 3*(n-1) }},
	{"lookaheads", func(n int) string { return "rule a = " + strings.Repeat("!", n) + "x" },
		func(n int) int { return 9 + n - 1 }},
	{"captures", func(n int) string { return "rule a = " + strings.Repeat("a:", n) + "x" },
		func(n int) int { return 9 + 2*(n-1) + 1 }},
	{"negations", func(n int) string { return "rule a = x => " + strings.Repeat("!", n-1) + "true" },
		func(n int) int { return 14 + n - 2 }},
	{"lambdas", func(n int) string { return "var f = " + strings.Repeat("(y) => ", n) + "y" },
		func(n int) int { return 8 + 7*(n-1) }},
	{"list types", func(n int) string { return "type T ~ " + strings.Repeat("[]", n) + "string" },
		func(n int) int { return 9 + 2*(n-1) }},
	{"calls", func(n int) string {
		return "rule a = x => " + strings.Repeat("f(", n-1) + "1" + strings.Repeat(")", n-1)
	},
		func(n int) int { return 14 + 2*(n-2) + 1 }},
	{"block comments", func(n int) string { return strings.Repeat("/*", n) + strings.Repeat("*/", n) + "\nrule a = x" },
		func(n int) int { return 2 * (n - 1) }},
	{"comment in parentheses", func(n int) string {
		return "rule a = " + strings.Repeat("(", n-1) + "/* c */ x" + strings.Repeat(")", n-1)
	}, func(n int) int { return 9 + n - 1 }},
}

// TestNestingLimit checks the nesting limit of the language specification at its boundary for
// every kind of level, with Parse, ParsePartial and the reference parser: the source of maxLevels
// levels is accepted, and the one of maxLevels+1 levels fails with the error at the token that
// opens the extra level. The parsers bound their own recursion far beyond maxLevels, so that
// they never reject a source that the limit accepts.
func TestNestingLimit(t *testing.T) {
	for _, f := range nestingFamilies {
		t.Run(f.name, func(t *testing.T) {
			src := f.src(maxLevels)
			if _, err := Parse(src); err != nil {
				t.Errorf("Parse of %d levels: %v", maxLevels, err)
			}
			if _, errs := ParsePartial(src); errs != nil {
				t.Errorf("ParsePartial of %d levels: %v", maxLevels, errs)
			}
			if _, err := ParseReference(src); err != nil {
				t.Errorf("ParseReference of %d levels: %v", maxLevels, err)
			}
			src = f.src(maxLevels + 1)
			want := fmt.Sprintf("%v: nesting too deep", offsetPosition(src, f.at(maxLevels+1)))
			for name, err := range map[string]error{"Parse": errOf(Parse(src)), "ParseReference": errOf(ParseReference(src))} {
				if err == nil || err.Error() != want {
					t.Errorf("%s of %d levels: %v, want %s", name, maxLevels+1, err, want)
				}
			}
			if _, errs := ParsePartial(src); errs.Error() != want {
				t.Errorf("ParsePartial of %d levels: %v, want %s", maxLevels+1, errs, want)
			}
		})
	}
}

// TestNestingKeepsSyntaxErrors checks that a source nested too deep still reports its syntax
// errors, and the nesting where it is: a type in parentheses is not a type definition, and a run
// of operators without parentheses is too deep at the operator that opens the extra level.
func TestNestingKeepsSyntaxErrors(t *testing.T) {
	src := "type T = " + strings.Repeat("(", 1000) + "string" + strings.Repeat(")", 1000)
	_, err := Parse(src)
	if err == nil || !strings.Contains(err.Error(), "1:8: ") || !strings.Contains(err.Error(), "1:510: nesting too deep") {
		t.Errorf("parenthesized type: %.200v", err)
	}
	src = "rule a = " + strings.Repeat("!", 200000)
	_, err = Parse(src)
	if err == nil || !strings.Contains(err.Error(), "1:510: nesting too deep") {
		t.Errorf("operators: %.200v", err)
	}
}

func errOf(_ any, err error) error { return err }

// offsetPosition returns the position of the code point offset i in src.
func offsetPosition(src string, i int) string {
	line, col := 1, 1
	for _, r := range []rune(src)[:i] {
		if r == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return fmt.Sprintf("%d:%d", line, col)
}
