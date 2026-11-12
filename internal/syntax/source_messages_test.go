package syntax

import (
	"strings"
	"testing"
)

// A spelling that the language no longer has fails with a message in the current syntax, at the
// old spelling.
func TestOldSpellingsReportCurrentSyntax(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`rule a = "x" -- "y"`, `1:14: the cut is written #cut`},
		{`rule a = -- "y"`, `1:10: the cut is written #cut`},
		{`rule a = x:"x" => $1`, `1:19: a capture is read by name: label the element (x:...) and write $x`},
		{`rule a = xs:"x"* => map($xs, (e) => $2)`, `1:37: a capture is read by name: label the element (x:...) and write $x`},
		{`type T terminal`, `1:8: a terminal type is the struct type Name = { Text string }`},
		{`type T struct { A string }`, `1:8: a struct type is defined as type Name = { ... }`},
		{`type T = string`, `1:8: expected '{' or 'node[' after '='; write type Name ~ Type for an alias`},
		{`type T = []A`, `1:8: expected '{' or 'node[' after '='; write type Name ~ Type for an alias`},
		{"rule a = operators {\n  operand x\n}\nrule x = \"x\"", `1:10: an Operators Rule is written #operators { ... }`},
		{`rule a = x:"x" => $x.text`, `1:22: a capture is read with .$text`},
		{`rule a = x:"x" => $x.Name.text`, `1:27: a capture is read with .$text`},
		{`rule a = xs:"x"* => map($xs, (e) => $e)`, `1:37: lambda parameters are written without $: write e`},
		{`rule a Match = "x"`, `1:8: Match is not a type; write node[string]`},
		{`type T = { A []Seq }`, `1:16: Seq is not a type; write node[_]`},
		{`rule a node = "x"`, `1:8: ` + bareNode},
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

// What a syntax error expects leaves out white space and comments, which may stand between any two
// tokens, and names a newline instead of listing its characters.
func TestSyntaxErrorsExpectNoTrivia(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"@start rule a = \"x\"", `1:8: expected newline, found 'rule'`},
		{`rule a = x:"x" => $x.1`, `1:22: expected "(", [a-zA-Z_], found 1`},
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
