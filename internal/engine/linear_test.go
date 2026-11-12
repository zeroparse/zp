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

// TestDeclaredLinearRewrites checks that each error of @linear gives the rewrite for the kind of its
// cause, with the link to the section of the recipe that shows it, on the grammar of each section
// of the recipe. Each grammar must name its own section and no other, so that a cause mapped to
// the wrong kind fails.
func TestDeclaredLinearRewrites(t *testing.T) {
	const recipe = "https://zeroparse.dev/docs/cookbook/linear-rewrites/"
	sections := map[string]string{
		"restart":   "scan once: put the scan and what may follow it in one alternative (see " + recipe + "#a-repetition-restarted-after-backtracking)",
		"lookahead": "check once after the scan instead of at every iteration (see " + recipe + "#a-lookahead-over-a-scan)",
		"skip":      "write the repetition inside skip= (see " + recipe + "#a-recovery-skip-followed-by-failure)",
		"regrowth":  "call the rule once at a position: factor the caller (see " + recipe + "#a-left-recursive-rule-or-an-operators-rule-called-again-and-again)",
		"binding":   "bind a value that does not depend on the earlier value (see " + recipe + "#diverging-variable-bindings)",
		"bound":     "keep the values of the variables bounded (see " + recipe + "#diverging-variable-bindings)",
		"growing":   "keep a count or another property of the value instead of the value (see " + recipe + "#self-growing-values)",
		"other":     "rewrite it so that the cost stays linear (see " + recipe + ")",
	}
	const head = "@linear\npackage p\n"
	for _, tc := range []struct {
		name, src, want string
	}{
		{"restarted repetition", head + "@start\nrule main = (a / \"a\")* $$\nrule a = \"a\"* \"c\"\n", "restart"},
		{"lookahead over a scan", head + "@start\nrule main = (\"a\" !rest)* \"a\"* $$\nrule rest = \"a\"* \"c\"\n", "lookahead"},
		{"recovery skip", head + "@start\nrule main = (x / \"a\")* $$\nrule x = (y #recover(skip=s)) \"c\"\nrule y = \"b\"\nrule s = [a]+\n", "skip"},
		{"left-recursive rule", head + "@start\nrule main = (e \"q\" / \"x\")* $$\nrule e = e \"x\" / \"x\"\n", "regrowth"},
		{"Operators Rule", head + "@start\nrule main = (e \"q\" / \"x\")* $$\nrule e = #operators {\n    operand \"x\"\n    level add { infix left \"\" }\n}\n", "regrowth"},
		{"diverging binding", head + "var v string\n@start\nrule main = r $$\nrule r = \"(\" {v := v + \"0\"} r \")\" \"y\" / \"(\" {v := v + \"1\"} r \")\" \"z\" / \"n\" {len(v) >= 0}\n", "binding"},
		{"bounded binding", head + "var v string\n@start\nrule main = r $$\nrule r = \"(\" {v := \"0\"} r \")\" \"y\" / \"(\" {v := \"1\"} r \")\" \"z\" / \"n\" {len(v) >= 0}\n", "bound"},
		{"self-growing variable", head + "var s string = \"a\"\n@start\nrule main = r $$\nrule r = \"x\" {s := s + s} r / {len(s) > 0}\n", "growing"},
		{"self-growing fold", head + "@start\nrule main = xs:\"x\"* => len(foldl(\"a\", $xs, (acc, x) => acc + acc))\n", "growing"},
		{"unknown", head + "@start\nrule main = r $$\nrule r = [ab]? (r / [a-c]) (w [ ] [b])\nrule w = [^a]* $$\n", "other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := syntax.Parse(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, err = engine.Compile(g, engine.Options{})
			if err == nil {
				t.Fatal("Compile accepted the grammar")
			}
			found := false
			for _, line := range strings.Split(err.Error(), "\n") {
				want := tc.want
				// A reason for the class unknown has no section of its own, even beside a cause.
				if strings.Contains(line, "the classifier cannot prove its cost linear: ") {
					want = "other"
				}
				found = found || want == tc.want
				if !strings.HasSuffix(line, "; "+sections[want]) {
					t.Errorf("error %q does not end with the rewrite of section %s", line, want)
				}
				for name, other := range sections {
					if name != want && strings.HasSuffix(line, other) {
						t.Errorf("error %q gives the rewrite of section %s, want %s", line, name, want)
					}
				}
			}
			if !found {
				t.Errorf("no error gives the rewrite of section %s:\n%v", tc.want, err)
			}
		})
	}
}
