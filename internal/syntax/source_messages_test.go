package syntax

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// A spelling that is not in the language fails with a message in the current syntax, at the
// spelling, that says what to write.
func TestOldSpellingsReportCurrentSyntax(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`rule a = "x" -- "y"`, `1:14: the cut is written #cut`},
		{`rule a = -- "y"`, `1:10: the cut is written #cut`},
		{`rule a = x:"x" => $1`, `1:19: a capture is read by name: label the element (x:...) and write $x instead of $1`},
		{`rule a = xs:"x"* => map($xs, (e) => $2)`, `1:37: a capture is read by name: label the element (x:...) and write $x instead of $2`},
		{`type T terminal`, `1:8: a terminal type is the struct type T = { Text string }`},
		{`type T struct { A string }`, `1:8: a struct type is defined as type T = {...}`},
		{`type T = string`, `1:8: expected '{' or 'node[' after '='; write type T ~ <type> for an alias`},
		{`type U = []A`, `1:8: expected '{' or 'node[' after '='; write type U ~ <type> for an alias`},
		{"rule a = operators {\n  operand x\n}\nrule x = \"x\"", `1:10: an Operators Rule is written #operators { ... }`},
		{`rule a = x:"x" => $x.text`, `1:22: a capture is read with .$text`},
		{`rule a = x:"x" => $x.Name.text`, `1:27: a capture is read with .$text`},
		{`rule a = xs:"x"* => map($xs, (e) => $e)`, `1:37: lambda parameters are written without $: write e`},
		{`rule a Match = "x"`, `1:8: Match is not a type; write node[string]`},
		{`type T = { A []Seq }`, `1:16: Seq is not a type; write node[_]`},
		{`rule a node = "x"`, `1:8: ` + bareNode + ``},
	} {
		_, err := Parse(c.src)
		if err == nil {
			t.Errorf("Parse(%q) succeeded, want %s", c.src, c.want)
			continue
		}
		if got := err.Error(); got != c.want {
			t.Errorf("Parse(%q):\n got %s\nwant %s", c.src, got, c.want)
		}
	}
}

// A value expected on the line after an operator, where a line break ends the statement, is a
// value that does not start on the line of its operator; inside parentheses, or at the end of the
// source, it is a value that is missing.
func TestMultilineValues(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"rule a = x => 1 +\nrule b = c", `2:1: multiline expressions require their own open delimiter`},
		{"rule a = x => {\n 1 +\n 2\n}", `3:2: multiline expressions require their own open delimiter`},
		{"rule a = x => (1 +\n)", `2:1: expected a value, found ')'`},
		{"rule a = x => 1 +\n", `2:1: expected a value, found end of file`},
	} {
		if _, err := Parse(c.src); err == nil || err.Error() != c.want {
			t.Errorf("Parse(%q): %v, want %s", c.src, err, c.want)
		}
	}
}

// What a syntax error expects leaves out white space and comments, which may stand between any two
// tokens, and names a newline instead of listing its characters.
func TestSyntaxErrorsExpectNoTrivia(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"@start rule a = \"x\"", `1:8: expected newline, found 'rule'`},
		{`rule a = x:"x" => $x.1`, `1:22: expected identifier, "(", found 1`},
	} {
		_, err := Parse(c.src)
		if err == nil {
			t.Errorf("Parse(%q) succeeded, want %s", c.src, c.want)
			continue
		}
		if got := err.Error(); got != c.want {
			t.Errorf("Parse(%q):\n got %s\nwant %s", c.src, got, c.want)
		}
	}
	// Every syntax error of a broken source, at every place where a token may end.
	const src = "package p\n@start\nrule a T = x:(\"x\" / [a-z]+)* #recover(skip=b) => T{A: $x, B: map($x, (e) => e.$y)}\nrule b = \"b\"\ntype T = { A []node[string]; B []int }\nvar v int = 1\n"
	for i := 1; i < len(src); i++ {
		for _, cut := range []string{src[:i], src[:i] + "?", src[:i] + ")", src[:i] + " @"} {
			_, err := Parse(cut)
			if err == nil {
				continue
			}
			for _, line := range strings.Split(err.Error(), "\n") {
				expected, ok := strings.CutPrefix(line[strings.Index(line, " ")+1:], "expected ")
				if !ok {
					continue
				}
				for _, item := range []string{"whitespace", "comment", `"/*"`, `"//"`, `"\r\n"`, `[\r\n]`, `[ \t`} {
					if strings.Contains(expected, item) {
						t.Errorf("Parse(%q): %s lists %s", cut, line, item)
					}
				}
			}
		}
	}
}

// An old spelling, a lowercase member and a variable value that fails are errors where they stand,
// also when the declaration fails again further on: the first error of the source is the one that
// the reference parser reports first, with its message.
func TestFirstErrorStaysAtTheMistake(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`rule a =--#operators { operand "1" }`, `1:9: the cut is written #cut`},
		{"rule a = (\n b\n @error(me--sage=\"x\") rule\n", `3:11: the cut is written #cut; expected closing parenthesis for attribute arguments`},
		{`rule a = x:"x" => $1 +`, `1:19: a capture is read by name: label the element (x:...) and write $x instead of $1`},
		{`rule a = b => W{A: $a, B: $,b C: $c}`, `1:27: expected initializer value, found '$'`},
		{`rule a = b => $ l + "+"`, `1:15: expected a value, "...", "{", found '$'`},
		{`rule a = b => N{A: x.yA: 1}`, `1:22: a capture is read with .$yA`},
		{"rule a = {\n type > 0\n ru.e == 1& & y\n}", `3:5: a capture is read with .$e`},
		{"var x int /* a */ = /\nvar y bool", `1:21: expected the initial value, found '/'`},
		{"var x int\nvar y = \n", `3:1: expected the initial value, found end of file`},
	} {
		first := func(err error) string {
			var list ErrorList
			if !errors.As(err, &list) || len(list) == 0 {
				return fmt.Sprint(err)
			}
			f := list[0]
			for _, e := range list[1:] {
				if e.Pos.Line < f.Pos.Line || e.Pos.Line == f.Pos.Line && e.Pos.Col < f.Pos.Col {
					f = e
				}
			}
			return f.Error()
		}
		_, err := Parse(c.src)
		if got := first(err); got != c.want {
			t.Errorf("Parse(%q):\n got %s\nwant %s", c.src, got, c.want)
		}
		_, ref := ParseReference(c.src)
		if g, r := strings.SplitN(first(err), ": ", 2)[0], strings.SplitN(first(ref), ": ", 2)[0]; g != r {
			t.Errorf("Parse(%q): first error at %s, the reference at %s", c.src, g, r)
		}
	}
}
