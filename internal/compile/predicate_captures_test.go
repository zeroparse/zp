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
		// A lambda parameter reads no capture of its name.
		{`x:a ys:b* { len(map($ys, (x) => x)) > 0 }`, []int{1}},
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
		pc := predicateCaptures(body, nil)
		read := pc.Read
		var got []int
		for i, cp := range all {
			if read[cp] {
				got = append(got, i)
			}
			if pc.Records(cp) != read[cp] {
				t.Errorf("%s: capture %d records %v, read %v", c.body, i, pc.Records(cp), read[cp])
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

// TestValueNeededRulesScoped checks that the rules which build values in a program that builds no
// tree are those that a call inside a capture that a predicate can read reaches, by the scoped
// analysis and not by name: b is called inside a capture named x made after the predicate that
// reads $x, and c inside a capture named y where the predicate names only a lambda parameter y.
func TestValueNeededRulesScoped(t *testing.T) {
	g, err := parseTest("rule main = x:a { len($x) > 0 } x:b / y:c ys:d* { len(map($ys, (y) => y)) > 0 }\nrule a = \"a\" e\nrule b = \"b\"\nrule c = \"c\"\nrule d = \"d\"\nrule e = \"e\"\n")
	if err != nil {
		t.Fatal(err)
	}
	needed := valueNeededRules(g.Rules(), nil, false)
	var got []string
	for name, ok := range needed {
		if ok {
			got = append(got, name)
		}
	}
	slices.Sort(got)
	if want := []string{"a", "d", "e"}; !slices.Equal(got, want) {
		t.Errorf("value-needed rules %v, want %v", got, want)
	}
}
