package complexity

import (
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/grammaranalysis"
)

// TestClassifyReusedSkips checks that a #recover skip whose repetitions are skip-run sites with
// elements of constant cost is not a scanning source, and that each condition keeps the skip a
// source when it fails.
func TestClassifyReusedSkips(t *testing.T) {
	restart := func(skip string) string {
		return "rule main = (x / \"a\")* $$\nrule x = (y #recover(skip=" + skip + ")) \"c\"\nrule y = \"b\"\nrule s = [a]+\n"
	}
	for _, tc := range []struct {
		name, src string
		class     ComplexityClass
		degree    int
	}{
		{"one character", restart("[a]+"), ClassLinear, 0},
		{"choice of characters", restart(`("a" / "b")+`), ClassLinear, 0},
		{"two characters", restart(`("aa" / "a")+`), ClassLinear, 0},
		{"bounded repetition in the element", restart(`([a] #repeat(1, 3))+`), ClassLinear, 0},
		{"lookahead in the element", restart(`(!"c" .)+`), ClassLinear, 0},
		{"constant parts around the site", restart(`"b"? [a]+ ";"?`), ClassLinear, 0},
		{"no repetition", restart(`"a"`), ClassLinear, 0},
		{"called rule", restart("s"), ClassPoly, 2},
		{"call in the element", restart(`(s / "b")+`), ClassPoly, 2},
		{"unbounded repetition in the element", restart(`("a" "b"*)+`), ClassPoly, 2},
		{"predicate in the element", restart(`([a] {1 > 0})+`), ClassPoly, 2},
		{"escaping cut in the element", restart(`("a" #cut "a"?)+`), ClassPoly, 2},
		{"call outside the site", restart(`[a]+ s`), ClassPoly, 2},
		{"nested recovery", "rule main = (s \"!\" / [a(]*) $$\nrule s = (\"(\" s \")\") #recover(skip=[a(]+)\n", ClassLinear, 0},
		{"nested recovery with a called skip", "rule main = (s \"!\" / [a(]*) $$\nrule s = (\"(\" s \")\") #recover(skip=t)\nrule t = [a(]+\n", ClassUnknown, 0},
		{"alternating regions", "rule main = (p / \"a\" / \"b\")* $$\nrule p = x \"b\" x \"z\"\nrule x = \"c\" #recover(skip=[a]+)\n", ClassLinear, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := classifySource(t, tc.src)
			if c.Class != tc.class || classDegree(c) != tc.degree {
				t.Fatalf("class %v degree %d, want %v degree %d\ncauses %v\nreasons %v", c.Class, c.Degree, tc.class, tc.degree, c.Causes, c.Reasons)
			}
		})
	}
}

// TestReusedSkipSites checks that the unbounded repetitions of a reused skip are exactly the
// skip-run sites that the compiler marks in it, so that the classifier relies only on walks that
// the parsers record.
func TestReusedSkipSites(t *testing.T) {
	for _, skip := range []string{
		"[a]+", `("a" / "b")+`, `("aa" / "a")+`, `"x" [a]* "y"+`, `([a]*) #repeat(0, 3)`, `(!"c" .)+`,
		`(&("a" #cut) "a")*`, `("a" #cut "x" / "y")*`, `"a"`, `[a] #repeat(1, 3)`,
		// Not reused: the compiler decides the sites by its own conditions.
		`("a" #cut "x")*`, `("x" [a]*)*`, "s*", `([a] {1 > 0})*`,
		`&([a]+ "z") [a]+`, `[a]+ {1 > 0}`, `[a] {1 > 0} [a]+`,
	} {
		t.Run(skip, func(t *testing.T) {
			prog := compiletest.Program(t, "rule main = \"b\" #recover(skip="+skip+")\nrule s = \"a\"\n", compile.Options{})
			var marked []*grammar.Repeat
			var skipExpr grammar.Expr
			for _, r := range prog.RuleList {
				grammaranalysis.WalkExpr(r.Def.Expr, func(x grammar.Expr) {
					if s := skipOf(x); s != nil {
						skipExpr = s
					}
					if rep, ok := x.(*grammar.Repeat); ok && prog.SkipSite(rep) != 0 {
						marked = append(marked, rep)
					}
				})
			}
			if skipExpr == nil {
				t.Fatal("no skip expression")
			}
			sites, ok := reusedSkip(skipExpr)
			if !ok {
				return
			}
			if len(sites) != len(marked) {
				t.Fatalf("reused skip has repetitions %d, compiler marks %d sites", len(sites), len(marked))
			}
			for i := range sites {
				if sites[i] != marked[i] {
					t.Errorf("repetition %s is not the site %s", grammar.FormatExpr(sites[i]), grammar.FormatExpr(marked[i]))
				}
			}
		})
	}
}
