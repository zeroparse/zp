package compile

import (
	"slices"
	"testing"

	"zeroparse.dev/zp/grammar"
)

// TestPredicateCaptures checks which captures predicateCaptures finds a predicate can read: those
// of its scope that can hold the value of the name it reads when it runs. A capture is numbered by
// its place among the captures of the rule body in preorder.
func TestPredicateCaptures(t *testing.T) {
	cases := []struct {
		body string
		want []int
	}{
		// Another alternative: undone by backtracking before the predicate runs.
		{`x:a "1" / x:a "2" / -a "3" / x:b { len($x) > 0 }`, []int{2}},
		// After the predicate.
		{`x:a { len($x) > 0 } x:b`, []int{0}},
		// Overwritten by a later capture of the same name on every path.
		{`x:a x:b { len($x) > 0 }`, []int{1}},
		// Overwritten on one path only.
		{`x:a (x:b)? { len($x) > 0 }`, []int{0, 1}},
		{`(x:a / "q") { len($x) > 0 }`, []int{0}},
		// A repetition element is a scope of its own.
		{`x:a (x:b { len($x) > 0 })*`, []int{1}},
		{`x:a (x:b)* { len($x) > 0 }`, []int{0}},
		// Kept from a positive lookahead.
		{`&(x:a) { len($x) > 0 }`, []int{0}},
		// Inside the capture itself: its value is not recorded yet.
		{`x:(a { len($x) > 0 })`, nil},
		// Read in a negative lookahead after the capture.
		{`x:a !({ len($x) > 1 } "z")`, []int{0}},
		{`x:a y:b`, nil},
		// A predicate that reads a name no capture of which can hold a value when it runs.
		{`"+" b:a / "-" { len(text($b)) == 0 }`, nil},
	}
	for _, c := range cases {
		g, err := parseTest("rule main = " + c.body + "\nrule a = \"a\"\nrule b = \"b\"\n")
		if err != nil {
			t.Fatalf("%s: %v", c.body, err)
		}
		body := g.Rules()[0].Expr
		var all []*grammar.Capture
		walkExprControlled(body, func(x grammar.Expr) {
			if cp, ok := x.(*grammar.Capture); ok {
				all = append(all, cp)
			}
		}, nil)
		read := predicateCaptures(body, nil).Read
		var got []int
		for i, cp := range all {
			if read[cp] {
				got = append(got, i)
			}
		}
		if len(read) != len(got) {
			t.Errorf("%s: %d captures read, %d of them in the body", c.body, len(read), len(got))
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: captures read %v, want %v", c.body, got, c.want)
		}
	}
}
