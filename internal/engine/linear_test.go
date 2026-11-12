package engine_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/syntax"
)

// TestDeclaredLinear checks @linear on the package clause and on rules: a declaration whose start
// the classifier proves linear compiles, and any other is an error at each cause or reason of
// the classification, with the declaration and a pointer to the rewrites.
func TestDeclaredLinear(t *testing.T) {
	const quadratic = "rule main = (A / \"a\")* $$\nrule A = \"a\"* \"c\"\n"
	for _, tc := range []struct {
		name, src string
		// want lists the errors as "line:col: subject", each followed by " is declared @linear";
		// empty when the grammar compiles.
		want []string
	}{
		{"linear grammar", "@linear\npackage p\n@start\nrule main = (A / \"a\")* $$\nrule A = \"a\" \"c\"\n", nil},
		{"quadratic grammar", "@linear\npackage p\n@start\n" + quadratic, []string{"5:10: the grammar from start rule main"}},
		{"quadratic start rule", "@start\n@linear\n" + quadratic, []string{"4:10: rule main"}},
		// The scanning source is in A, but parsing that begins at A is linear: the restart is the
		// fault of main.
		{"linear rule called by a quadratic one", "@start\n" + quadratic + "@linear\nrule B = A\n", nil},
		{"linear rule in a quadratic grammar", "@start\nrule main = (A / \"a\")* $$\n@linear\nrule A = \"a\"* \"c\"\n", nil},
		{"quadratic rule reached from a rule", "@start\nrule main = B $$\n@linear\nrule B = (A / \"a\")*\nrule A = \"a\"* \"c\"\n", []string{"5:10: rule B"}},
		{"every start rule", "@linear\npackage p\n@start(default)\nrule main = \"x\"\n@start\nrule other = (A / \"a\")* $$\nrule A = \"a\"* \"c\"\n", []string{"7:10: the grammar from start rule other"}},
		{"one report for two declarations", "@linear\npackage p\n@start\n@linear\n" + quadratic, []string{"6:10: the grammar from start rule main"}},
		// A recovery skip in the scope is checked: a repetition in a rule that the skip calls is
		// walked again from each position.
		{"recovery skip walked again", "@linear\npackage p\n@start\nrule main = (X / \"a\")* $$\nrule X = (Y #recover(skip=S)) \"c\"\nrule Y = \"b\"\nrule S = [a]+\n", []string{"5:1: the grammar from start rule main"}},
		{"recovery skip run", "@linear\npackage p\n@start\nrule main = (X / \"a\")* $$\nrule X = (Y #recover(skip=[a]+)) \"c\"\nrule Y = \"b\"\n", nil},
		{"linear given an environment bound", "@linear\npackage p\nvar k int\n@start\nrule main = e $$\nrule e = e \"x\" {k := k + 1} / \"x\" {k >= 0}\n", []string{"the grammar from start rule main is declared @linear, but its cost is linear only given a bound"}},
		{"unknown", "@linear\npackage p\n@start\nrule main = r $$\nrule r = [ab]? (r / [a-c]) (w [ ] [b])\nrule w = [^a]* $$\n", []string{"the grammar from start rule main is declared @linear, but the classifier cannot prove its cost linear"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := syntax.Parse(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, err = engine.Compile(g, engine.Options{})
			if len(tc.want) == 0 {
				if err != nil {
					t.Fatalf("Compile: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Compile accepted the grammar")
			}
			lines := strings.Split(err.Error(), "\n")
			if len(lines) != len(tc.want) {
				t.Fatalf("errors:\n%v\nwant %d", err, len(tc.want))
			}
			for i, line := range lines {
				if !strings.Contains(line, tc.want[i]) || !strings.Contains(line, "is declared @linear") ||
					!strings.Contains(line, "https://zeroparse.dev/docs/cookbook/linear-rewrites/") {
					t.Errorf("error %q, want %q", line, tc.want[i])
				}
			}
			// The editor entry point reports the same errors.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if _, cerr := engine.CompileContext(ctx, g, engine.Options{}); cerr == nil || cerr.Error() != err.Error() {
				t.Errorf("CompileContext: %v", cerr)
			}
		})
	}
}

func TestDeclaredLinearAttributeErrors(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"@linear(1)\npackage p\n@start\nrule main = \"x\"\n", "1:1: @linear takes no arguments"},
		{"@linear @linear\npackage p\n@start\nrule main = \"x\"\n", "1:9: duplicate @linear on the package clause"},
		{"@start\npackage p\n@start\nrule main = \"x\"\n", "1:1: unknown declaration attribute @start of the package clause"},
		{"@start @linear @linear\nrule main = \"x\"\n", "1:16: duplicate @linear on rule main"},
		{"@start @linear(x=1)\nrule main = \"x\"\n", "1:8: @linear takes no arguments"},
	} {
		g, err := syntax.Parse(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Compile(g, engine.Options{}); err == nil || err.Error() != tc.want {
			t.Errorf("%q\n got %v\nwant %s", tc.src, err, tc.want)
		}
	}
}

// TestDeclaredLinearKeepsClassification checks that @linear only adds a check: the
// classification of a grammar does not change when its rules declare it.
func TestDeclaredLinearKeepsClassification(t *testing.T) {
	for _, src := range []string{
		"@start\nrule main = (A / \"a\")* $$\n%srule A = \"a\"* \"c\"\n",
		"@start\nrule main = (X / \"a\")* $$\nrule X = (Y #recover(skip=S)) \"c\"\nrule Y = \"b\"\n%srule S = [a]+\n",
		"@start\nrule main = (B \"q\" / \"x\")* $$\n%srule B = sp [a-z]*\nrule sp = \" \"*\n",
	} {
		var got [2]string
		for i, decl := range []string{"\n", "@linear\n"} { // a blank line keeps the positions
			g, err := syntax.Parse(fmt.Sprintf(src, decl))
			if err != nil {
				t.Fatal(err)
			}
			c, err := engine.ClassifyComplexity(context.Background(), g, "")
			if err != nil {
				t.Fatal(err)
			}
			got[i] = fmt.Sprintf("%v %d %v %v", c.Class, c.Degree, c.Causes, c.Reasons)
		}
		if got[0] != got[1] {
			t.Errorf("%q: without @linear %s\nwith @linear %s", src, got[0], got[1])
		}
	}
}
