package complexity

import (
	"maps"
	"slices"
	"sync"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/grammaranalysis"
	"zeroparse.dev/zp/internal/syntax"
)

// viewOf returns the view of the body of rule main of src with opts, and the rewrites of each
// rule.
func viewOf(t *testing.T, src string, opts Options) (string, map[Law]int) {
	t.Helper()
	bodies, applied := viewBodies(t, src, opts)
	body, ok := bodies["main"]
	if !ok {
		t.Fatal("no rule main")
	}
	return body, applied
}

// viewBodies returns the formatted rule bodies of the cost view of the grammar src with opts, by
// rule name, and the rewrites that the view counts.
func viewBodies(t *testing.T, src string, opts Options) (map[string]string, map[Law]int) {
	t.Helper()
	v, _, rules, _ := testView(t, src, opts)
	bodies := map[string]string{}
	for _, r := range rules {
		bodies[r.Name] = grammar.FormatExpr(r.Expr)
	}
	return bodies, v.Applied
}

// builtTexts returns, by spelling, the grammar text that each expression of rule main stands for
// that the cost view of src with opts builds.
func builtTexts(t *testing.T, src string, opts Options) map[string]string {
	t.Helper()
	_, source, rules, text := testView(t, src, opts)
	orig := map[grammar.Expr]bool{}
	got := map[string]string{}
	for _, r := range source {
		grammaranalysis.WalkExpr(r.Expr, func(x grammar.Expr) { orig[x] = true })
	}
	for _, r := range rules {
		if r.Name != "main" {
			continue
		}
		grammaranalysis.WalkExpr(r.Expr, func(x grammar.Expr) {
			if !orig[x] {
				got[grammar.FormatExpr(x)] = text[x]
			}
		})
	}
	return got
}

// testView returns the cost view of the grammar src with opts, the rules it is built from, its
// rules written as grammar rules and the source text of each expression of them that the view
// builds.
func testView(t *testing.T, src string, opts Options) (*View, []*grammar.RuleDef, []*grammar.RuleDef, map[grammar.Expr]string) {
	t.Helper()
	g, err := syntax.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := NewCostSource(g, nil)
	if err != nil {
		t.Fatal(err)
	}
	v := Build(&Source{Module: cs.Module, Origin: cs.Origin, Facts: cs.Facts}, opts, nil)
	rules, text, err := v.GrammarText(cs.Rules)
	if err != nil {
		t.Fatal(err)
	}
	return v, cs.Rules, rules, text
}

// TestNormalView checks each rewrite of spec/runtime/parsing.md (Cost View): where its conditions
// hold it rewrites the expression as its law says, and where one of them fails, as in the
// counterexamples of spec/proofs/grammar-laws.md, it leaves the expression as it is.
func TestNormalView(t *testing.T) {
	for _, c := range normalViewCases {
		t.Run(c.name, func(t *testing.T) {
			got, applied := viewOf(t, "rule main = "+c.body+normalViewHelpers, c.opts)
			if got != c.want {
				t.Errorf("view of %s = %s, want %s", c.body, got, c.want)
			}
			if c.rule != "" && applied[c.rule] == 0 {
				t.Errorf("view of %s: %s did not apply (%v)", c.body, c.rule, applied)
			}
		})
	}
}

// TestNormalViewPredicateCaptures checks that the rules of the view that change values stay off
// inside a capture that a predicate reads, and in the body of every rule whose value reaches such a
// capture, and stay on elsewhere: the congruence ≡E,X of those rules does not hold in a context
// that reads a value of the expression in a predicate. These are the value conditions of
// spec/runtime/parsing.md (Cost View).
func TestNormalViewPredicateCaptures(t *testing.T) {
	for _, c := range predicateCaptureCases {
		t.Run(c.name, func(t *testing.T) {
			if c.rejected {
				g, err := syntax.Parse(c.src + "\n")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := NewCostSource(g, nil); err == nil {
					t.Errorf("the cost source of a rejected grammar was built: %s", c.src)
				}
				return
			}
			got, _ := viewBodies(t, c.src+"\n", Options{})
			for name, want := range c.want {
				if got[name] != want {
					t.Errorf("view of rule %s = %s, want %s", name, got[name], want)
				}
			}
		})
	}
}

// TestNormalViewPredicateValueFlow checks the other ways in which a predicate observes the value
// of a captured expression: through a variable that a binding sets from the capture, through the
// capture access $x.$y into a rule's node, and through the elements of a repetition. Each keeps the
// written shape of the expression that it reads, while an expression that no predicate reaches is
// normalized. $0 and $1, which would read an element of a sequence, are rejected by the parser, so
// they cannot reach the view.
func TestNormalViewPredicateValueFlow(t *testing.T) {
	for _, c := range valueFlowCases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := viewBodies(t, c.src+"\n", Options{})
			for name, want := range c.want {
				if got[name] != want {
					t.Errorf("view of rule %s = %s, want %s", name, got[name], want)
				}
			}
		})
	}
	for _, src := range []string{
		"rule main = (\"a\"* \"a\"*) {len($0) == 2} \"b\"\n",
		"rule main = (\"a\"* \"a\"*) {len($1) == 2} \"b\"\n",
	} {
		if _, err := syntax.Parse(src); err == nil {
			t.Errorf("a positional reference was accepted: %s", src)
		}
	}
}

// TestNormalViewCheapRules checks which calls the view drops as evaluated again cheaply
// (cheapRules): calls of a memoized rule and of a rule whose body is cheap, and not calls of a rule
// that a parse with insertion does not memoize, of a rule with an action, or of an Operators Rule.
func TestNormalViewCheapRules(t *testing.T) {
	for _, c := range cheapRuleCases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := viewBodies(t, c.src+"\n", Options{})
			if got["main"] != c.want {
				t.Errorf("view of rule main = %s, want %s", got["main"], c.want)
			}
		})
	}
}

// TestNormalViewText checks that every expression that the view builds stands for text of the
// grammar, which messages quote: a factored rest for its whole alternative, a factored sequence for
// the first alternative of its run, and an alternative without its cuts for itself.
func TestNormalViewText(t *testing.T) {
	for _, c := range textCases {
		t.Run(c.body, func(t *testing.T) {
			_, source, rules, text := testView(t, "rule main = "+c.body+"\n", Options{})
			orig := map[grammar.Expr]bool{}
			grammaranalysis.WalkExpr(source[0].Expr, func(x grammar.Expr) { orig[x] = true })
			got := map[string]string{}
			grammaranalysis.WalkExpr(rules[0].Expr, func(x grammar.Expr) {
				if orig[x] {
					return
				}
				text, ok := text[x]
				if !ok {
					t.Errorf("%s, built by the view, stands for no text of the grammar", grammar.FormatExpr(x))
				}
				got[grammar.FormatExpr(x)] = text
			})
			for spelling, text := range c.want {
				if got[spelling] != text {
					t.Errorf("%s stands for %q, want %q (view %s)", spelling, got[spelling], text, grammar.FormatExpr(rules[0].Expr))
				}
			}
		})
	}
}

// TestNormalViewCutKeepsTuple checks that dropping the cuts of a sequence leaves a sequence of one
// item, a tuple of one element, where a predicate can read its value, and the bare item where none
// can: `a #cut` is a tuple of one element, and `len($x)` tells it from the item (N6 in
// spec/runtime/parsing.md, Cost View).
func TestNormalViewCutKeepsTuple(t *testing.T) {
	for _, c := range cutTupleCases {
		t.Run(c.name, func(t *testing.T) {
			_, _, rules, _ := testView(t, c.src, Options{})
			got := 0
			for _, r := range rules {
				grammaranalysis.WalkExpr(r.Expr, func(x grammar.Expr) {
					if s, ok := x.(*grammar.Seq); ok && len(s.Items) == 1 {
						got++
					}
				})
			}
			if got != c.want {
				t.Errorf("sequences of one item in the view = %d, want %d", got, c.want)
			}
		})
	}
}

// TestNormalViewKeepsValueShapes checks the rewrites that change a value without being about
// repetitions or cuts: the choice that is left with one alternative, which a sequence would splice
// into the sequence around it, an alternative without a value, which the choice gives the value
// nil, and the inlining of a call, whose body would be spliced in the same way. Each stays where a
// predicate reads the value, and applies elsewhere.
func TestNormalViewKeepsValueShapes(t *testing.T) {
	for _, c := range valueShapeCases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := viewBodies(t, c.src+"\n", c.opts)
			for name, want := range c.want {
				if got[name] != want {
					t.Errorf("view of rule %s = %s, want %s", name, got[name], want)
				}
			}
		})
	}
}

// TestNormalViewLawSwitch requires the law switch of the view to gate every rewrite: on every
// corpus grammar, with one law on, the view counts rewrites of that law only, and with none on it
// rewrites nothing. With every law on, each law but N7 and N13 rewrites some grammar of the corpus
// or of lawsGrammar, so the check is not vacuous for those.
func TestNormalViewLawSwitch(t *testing.T) {
	t.Parallel()
	laws := []Law{N1, N2, N3, N4, N5, N6,
		N7, N8, N9, N10, N11, N12, N13}
	var mu sync.Mutex
	seen := map[Law]bool{}
	t.Run("corpus", func(t *testing.T) {
		for _, path := range append(slices.Clone(factstest.Corpus), "laws") {
			t.Run(path, func(t *testing.T) {
				t.Parallel()
				src := lawsGrammar
				if path != "laws" {
					src = factstest.Read(t, path)
				}
				for _, matching := range []bool{false, true} {
					view := func(laws []Law) (*View, []*grammar.RuleDef, []*grammar.RuleDef) {
						v, source, rules, _ := testView(t, src, Options{Labels: matching, Inline: matching, Laws: laws})
						return v, source, rules
					}
					all, _, _ := view(nil)
					mu.Lock()
					for r, n := range all.Applied {
						seen[r] = seen[r] || n > 0
					}
					mu.Unlock()
					none, source, rules := view([]Law{})
					if len(none.Applied) != 0 {
						t.Errorf("matching %v, no law: %v", matching, none.Applied)
					}
					for i, r := range rules {
						if got, want := grammar.FormatExpr(r.Expr), grammar.FormatExpr(source[i].Expr); got != want {
							t.Errorf("matching %v, no law: rule %s is %s, want %s", matching, r.Name, got, want)
						}
					}
					for _, l := range laws {
						one, _, _ := view([]Law{l})
						for r, n := range one.Applied {
							if r != l && n > 0 {
								t.Errorf("matching %v, law %s alone: %d rewrites of %s", matching, l, n, r)
							}
						}
					}
				}
			})
		}
	})
	for _, l := range laws[:12] {
		if l != N7 && !seen[l] {
			t.Errorf("no corpus grammar applies %s", l)
		}
	}
}

// lawsGrammar applies the laws that no corpus grammar applies: N3 (a duplicate alternative), N5
// (stacked lookaheads), N9 (repetitions in a row) and N10 (a nested repetition).
const lawsGrammar = "@start\nrule main = (\"ab\" / \"ab\") &(&\"b\") \"b\" \"c\"* \"c\"* (\"d\"+)*\n"

// normalViewHelpers are the rules that the cases of TestNormalView call.
const normalViewHelpers = "\nrule r = \"r\" \"r\"\nrule p = {true} \"p\"\n"

// normalViewCases are the cases of TestNormalView: the body of rule main, with normalViewHelpers.
var normalViewCases = []struct {
	name string
	rule Law // the rule that applies, or "" for a negative case
	body string
	want string
	opts Options
}{
	{name: "S1 splices a grouped sequence", rule: N1, body: `"ab" ("cd" "ef")`, want: `"ab" "cd" "ef"`},
	{name: "C2 flattens a right-nested choice", rule: N1, body: `"ab" / ("cd" #cut "x" / "ef")`, want: `"ab" / "cd" #cut "x" / "ef"`},
	{name: "C2 flattens a left-nested choice without cuts", rule: N1, body: `("ab" / "cd") / "ef"`, want: `"ab" / "cd" / "ef"`},
	{name: "C2 keeps a left-nested choice with a cut", body: `("ab" #cut "x" / "cd") / "ef"`, want: `("ab" #cut "x" / "cd") / "ef"`},
	{name: "C3 drops alternatives after one that always succeeds", rule: N2, body: `"ab"* / "cd"`, want: `"ab"*`},
	{name: "C3 keeps alternatives after a repetition with a loose cut", body: `("ab" #cut "x")* / "cd"`, want: `("ab" #cut "x")* / "cd"`},
	{name: "C1 drops a duplicate alternative", rule: N3, body: `"ab" / "cd" / "ab"`, want: `"ab" / "cd"`},
	{name: "C5 drops an alternative that an earlier one prefixes", rule: N3, body: `"ab" / "ab" "cd"`, want: `"ab"`},
	{name: "C5 keeps a literal that no earlier alternative prefixes", body: `"=" "x" / "==" "y" / "="`, want: `"=" "x" / "==" "y" / "="`},
	{name: "C5 drops a longer literal", rule: N3, body: `"=" / "=="`, want: `"="`},
	{name: "C5 keeps a longer literal after one without case", body: `"a"i / "ab"`, want: `"a"i / "ab"`},
	{name: "C5 keeps a longer literal without case", body: `"a" / "ab"i`, want: `"a" / "ab"i`},
	{name: "C1 keeps a duplicate recovery", body: `"a" #recover(skip="z") / "a" #recover(skip="z")`, want: `"a" #recover(skip="z") / "a" #recover(skip="z")`},
	{name: "C1 keeps a duplicate insertion", body: `"a" #missing / "a" #missing`, want: `"a" #missing / "a" #missing`},
	{name: "C6 ends a run at an alternative with a cut", body: `"a" "b" / "a" #cut "c" / "d"`, want: `"a" "b" / "a" #cut "c" / "d"`},
	{name: "C9 keeps a class with the replacement character", body: `[\u{fff0}-\u{ffff}] / "b"`, want: `[\u{fff0}-\u{ffff}] / "b"`},
	{name: "C1 keeps a duplicate with a predicate", body: `{true} "ab" / {true} "ab"`, want: `{true} "ab" / {true} "ab"`},
	{name: "C1 keeps a duplicate that calls a rule with a predicate", body: `p / p`, want: `p / p`},
	{name: "C4 drops a bottom alternative", rule: N4, body: `_|_ / "ab" / "cd"`, want: `"ab" / "cd"`},
	{name: "C4 keeps a labelled bottom", body: `_|_ @error("e") / "ab"`, want: `_|_ @error(message="e") / "ab"`},
	{name: "S2 drops what follows a bottom", rule: N4, body: `"ab" _|_ "cd"`, want: `"ab" _|_`},
	{name: "K2 removes a positive lookahead under a lookahead", rule: N5, body: `!&"ab" "c"`, want: `!"ab" "c"`},
	{name: "K2 keeps the negation of a negation under a positive lookahead", rule: N5, body: `&!"ab" "c"`, want: `!"ab" "c"`},
	{name: "K1 turns a double negation into a positive lookahead", rule: N5, body: `!!"ab" "c"`, want: `&"ab" "c"`},
	{name: "K1 keeps a double negation of a binding", body: `!!({v := 1}) "c"` + "\nvar v int", want: `!!{v := 1} "c"`},
	{name: "T2 drops a cut that ends an alternative", rule: N6, body: `"ab" #cut / "cd"`, want: `"ab" / "cd"`},
	{name: "T2 drops a cut in the last alternative", rule: N6, body: `"ab" / "cd" #cut "ef"`, want: `"ab" / "cd" "ef"`},
	{name: "T2 drops the alternatives after a leading cut", rule: N6, body: `#cut "ab" / "cd"`, want: `"ab"`},
	{name: "T2 drops a cut nested in the last alternative", rule: N6, body: `"ab" / -("cd" #cut "x")`, want: `"ab" / -("cd" "x")`},
	{name: "T2 drops a nested cut of an only alternative left in a repetition", rule: N6, body: `(-("a" #cut "b") / !"q" "q")* "ac"`, want: `(-("a" "b"))* "ac"`},
	{name: "T2 drops a nested cut of an only alternative left in an option", rule: N6, body: `(x:("a" #cut "b") / !"q" "q")? "ac"`, want: `(x:("a" "b"))? "ac"`},
	{name: "T2 drops a nested cut of an only alternative spliced into a choice", rule: N6, body: `((-("x" #cut "y" "q") / !"z" "z") / "x")* $$`, want: `(-("x" "y" "q") / "x")* $$`},
	{name: "T2 keeps a choice whose only alternative left has a cut in a skip", body: `("a" "b" #recover(skip=#cut [^;]*) / !"q" "q")* "ac"`, want: `("a" "b" #recover(skip=#cut [^;]*) / !"q" "q")* "ac"`},
	{name: "T1 keeps a cut that commits", body: `"ab" #cut "x" / "cd"`, want: `"ab" #cut "x" / "cd"`},
	{name: "V1 drops a recovery that never fires", rule: N6, body: `"ab"* #recover(skip=[^;]*)`, want: `"ab"*`},
	{name: "V1 keeps a recovery that can fire", body: `"ab" #recover(skip=[^;]*)`, want: `"ab" #recover(skip=[^;]*)`},
	{name: "N7 drops labels for analyses of matching", rule: N7, body: `"ab" @error("e")`, want: `"ab"`, opts: Options{Labels: true}},
	{name: "N7 is off for the classifier", body: `"ab" @error("e")`, want: `"ab" @error(message="e")`},
	{name: "C9 merges single characters", rule: N8, body: `"a" / [0-9] / "b" "c"`, want: `[0-9a] / "b" "c"`},
	{name: "C9 keeps a literal without case", body: `"a"i / "b"i`, want: `"a"i / "b"i`},
	{name: "C9 keeps a negated class", body: `[^a] / "b"`, want: `[^a] / "b"`},
	{name: "C9 keeps the replacement character", body: `"\u{fffd}" / "b"`, want: "\"\ufffd\" / \"b\""},
	{name: "R3 merges repetitions in a row", rule: N9, body: `"ab"* "ab"*`, want: `"ab"*`},
	{name: "R4 merges a plus and a star", rule: N9, body: `"ab"+ "ab"*`, want: `"ab"+`},
	{name: "R4 merges an option and a star", rule: N9, body: `"ab"? "ab"*`, want: `"ab"*`},
	{name: "R5 folds an item and its repetition", rule: N9, body: `"ab" "ab"*`, want: `"ab"+`},
	{name: "R3 keeps repetitions with a loose cut", body: `("ab" #cut "x")* ("ab" #cut "x")*`, want: `("ab" #cut "x")* ("ab" #cut "x")*`},
	{name: "R3 keeps nullable repetitions that bind", body: `({v == 0} {v := 1} / {v == 1} "a")* ({v == 0} {v := 1} / {v == 1} "a")*` + "\nvar v int",
		want: `({v == 0} {v := 1} / {v == 1} "a")* ({v == 0} {v := 1} / {v == 1} "a")*`},
	{name: "R3 keeps repetitions of a rule with a predicate", body: `p* p*`, want: `p* p*`},
	{name: "R3 merges repetitions of a cheap rule", rule: N9, body: `r* r*`, want: `r*`},
	{name: "R6 flattens a star of a star", rule: N10, body: `("ab"*)*`, want: `"ab"*`},
	{name: "R6 flattens a star of an option", rule: N10, body: `("ab"?)*`, want: `"ab"*`},
	{name: "R6 keeps a nested repetition with a loose cut", body: `(("ab" #cut "x")*)*`, want: `("ab" #cut "x")**`},
	{name: "R7 unrolls whitespace", rule: N11, body: `[ ]* ("//" [^\n]* [ ]*)*`, want: `([ ] / "//" [^\n]*)*`},
	{name: "R7 absorbs a repetition into a choice", rule: N11, body: `"ab"* ("ab" / "cd")*`, want: `("ab" / "cd")*`},
	{name: "R7 keeps a nullable element", body: `("ab" / "")* ("cd" ("ab" / "")*)*`, want: `("ab" / "")* ("cd" ("ab" / "")*)*`},
	{name: "R7 keeps a loose cut", body: `[ ]* ("//" #cut [^\n]* [ ]*)*`, want: `[ ]* ("//" #cut [^\n]* [ ]*)*`},
	{name: "R7 keeps a separator that binds", body: `x* ({v := 1} x*)*` + "\nvar v int = 0\nrule x = {v == 1} a\nrule a = \"a\"",
		want: `x* ({v := 1} x*)*`},
	{name: "R7 keeps an element with a predicate", body: `p* ("c" p*)*`, want: `p* ("c" p*)*`},
	{name: "R7 unrolls a cheap rule", rule: N11, body: `r* ("c" r*)*`, want: `(r / "c")*`},
	{name: "R3 keeps a star before a plus", body: `"ab"* "ab"+`, want: `"ab"* "ab"+`},
	{name: "R3 keeps a repetition with a minimum above one", body: `"ab"#repeat(2,) "ab"*`, want: `"ab" #repeat(2,) "ab"*`},
	{name: "R3 keeps a bounded repetition", body: `"ab"#repeat(1,3) "ab"*`, want: `"ab" #repeat(1, 3) "ab"*`},
	{name: "R5 keeps an item before a plus", body: `"ab" "ab"+`, want: `"ab" "ab"+`},
	{name: "R5 keeps an item before a bounded repetition", body: `"ab" "ab"#repeat(,3)`, want: `"ab" "ab" #repeat(,3)`},
	{name: "R5 keeps a rule with a predicate", body: `p p*`, want: `p p*`},
	{name: "R4 keeps an option of a nullable element", body: `("ab" / "")? ("ab" / "")*`, want: `("ab" / "")? ("ab" / "")*`},
	{name: "R4 keeps an option of a rule with a predicate", body: `p? p*`, want: `p? p*`},
	{name: "R6 flattens a star of a plus", rule: N10, body: `("ab"+)*`, want: `"ab"*`},
	{name: "R6 keeps a plus of a plus", body: `("ab"+)+`, want: `"ab"++`},
	{name: "R6 keeps a bounded repetition of a star", body: `("ab"*)#repeat(,3)`, want: `"ab"* #repeat(,3)`},
	{name: "R6 keeps a star of an option of a nullable element", body: `(("ab" / "")?)*`, want: `("ab" / "")?*`},
	{name: "R6 keeps a plus of an option", body: `("ab"?)+`, want: `"ab"?+`},
	{name: "R7 keeps a plus before the separated repetition", body: `[ ]+ ("//" [ ]*)*`, want: `[ ]+ ("//" [ ]*)*`},
	{name: "R7 keeps a separator followed by a plus", body: `[ ]* ("//" [ ]+)*`, want: `[ ]* ("//" [ ]+)*`},
	{name: "R7 keeps a separator followed by a bounded repetition", body: `[ ]* ("//" [ ]#repeat(,3))*`, want: `[ ]* ("//" [ ] #repeat(,3))*`},
	{name: "R7 keeps the absorption of a rule with a predicate", body: `p* (p / "c")*`, want: `p* (p / "c")*`},
	{name: "R3 and R7 apply in a row", rule: N9, body: `"a"* "a"* ("c" "a"*)*`, want: `[ac]*`},
	{name: "K4 turns self exclusion into bottom", rule: N12, body: `!"ab" "ab"`, want: `_|_`},
	{name: "K4 keeps self exclusion of a call", body: `!r r`, want: `!r r`},
	{name: "K4 turns self exclusion of a bounded repetition into bottom", rule: N12, body: `!"a"#repeat(2) "a"#repeat(2)`, want: `_|_`},
	{name: "K4 keeps self exclusion of an unbounded repetition", body: `!"a"+ "a"+`, want: `!"a"+ "a"+`},
	{name: "C6 factors a shared first item", rule: N12, body: `"(" "xy" / "(" "zw"`, want: `"(" ("xy" / "zw")`},
	{name: "C6 factors a call of a cheap rule", rule: N12, body: `r "xy" / r "zw" / "q"`, want: `r ("xy" / "zw") / "q"`},
	{name: "C6 keeps a cut", body: `"(" #cut "xy" / "(" "zw"`, want: `"(" #cut "xy" / "(" "zw"`},
	{name: "C6 keeps a predicate prefix", body: `{true} "xy" / {true} "zw"`, want: `{true} "xy" / {true} "zw"`},
	{name: "C6 keeps a prefix that calls a rule with a predicate", body: `p "xy" / p "zw"`, want: `p "xy" / p "zw"`},
	{name: "P1 inlines a rule that is not memoized", rule: N13, body: `a "x"` + "\nrule a = \"y\" \"z\"", want: `"y" "z" "x"`, opts: Options{Inline: true}},
	{name: "N13 is off for the classifier", body: `a "x"` + "\nrule a = \"y\" \"z\"", want: `a "x"`},
	{name: "P1 inlines a rule declared @linear", rule: N13, body: `a "x"` + "\n@linear\nrule a = \"y\" \"z\"", want: `"y" "z" "x"`, opts: Options{Inline: true}},
	{name: "P1 keeps a rule with a capture", body: `a "x"` + "\nrule a = c:\"y\" \"z\"", want: `a "x"`, opts: Options{Inline: true}},
	{name: "P1 keeps a rule with a loose cut", body: `a "x"` + "\nrule a = \"y\" #cut \"z\"", want: `a "x"`, opts: Options{Inline: true}},
	{name: "P1 keeps a start rule", body: `s "x"` + "\n@start\nrule s = \"y\" \"z\"", want: `s "x"`, opts: Options{Inline: true}},
	{name: "P1 keeps a rule that calls itself", body: `a` + "\nrule a = \"x\" a m / \"y\"\nrule m = \"m\" #missing", want: `a`, opts: Options{Inline: true}},
	{name: "P1 keeps a rule of a left-recursion cycle", body: `a` + "\nrule a = b \"x\" / \"y\"\nrule b = a \"z\"", want: `a`, opts: Options{Inline: true}},
	{name: "P1 inlines a call whose value no predicate reads of a rule that one reads elsewhere", rule: N13,
		body: `y:a {len($y) == 2} a` + "\nrule a = \"r\"* \"r\"*",
		want: `y:a {len($y) == 2} "r"*`, opts: Options{Inline: true}},
	{name: "P1 is off for the classifier", body: `a "x"` + "\nrule a = \"y\" \"z\"", want: `a "x"`},
}

// predicateCaptureCases are the cases of TestNormalViewPredicateCaptures.
var predicateCaptureCases = []struct {
	name string
	src  string
	want map[string]string // the view of each rule named
	// rejected marks a grammar that the language rejects (a capture under a repetition of a
	// repetition is not in scope after it), so that the view never meets it.
	rejected bool
}{
	{name: "a capture that a predicate reads keeps its repetitions",
		src:  "rule main = x:(\"a\"* \"a\"*) {len($x) == 2} (e \"q\" / \"x\")* $$\nrule e = e \"x\" / \"x\"",
		want: map[string]string{"main": `x:("a"* "a"*) {len($x) == 2} (e "q" / "x")* $$`}},
	{name: "a capture that a predicate reads keeps its nested repetitions and factoring",
		src:  "rule main = x:(\"a\"+ (\"b\"*)* / \"a\" \"c\" / \"a\" \"d\") {len($x) == 2}",
		want: map[string]string{"main": `x:("a"+ "b"** / "a" "c" / "a" "d") {len($x) == 2}`}},
	{name: "a capture that no predicate reads is normalized",
		src:  "rule main = x:(\"a\"* \"a\"*) {true}",
		want: map[string]string{"main": `x:"a"* {true}`}},
	{name: "a capture that another capture's predicate reads is normalized",
		src:  "rule main = x:(\"a\"* \"a\"*) y:(\"b\"* \"b\"*) {len($y) == 2}",
		want: map[string]string{"main": `x:"a"* y:("b"* "b"*) {len($y) == 2}`}},
	{name: "the rest of the rule is normalized",
		src:  "rule main = x:(\"a\"* \"a\"*) {len($x) == 2} \"b\"* \"b\"*",
		want: map[string]string{"main": `x:("a"* "a"*) {len($x) == 2} "b"*`}},
	{name: "a rule whose value reaches the capture keeps its repetitions",
		src:  "rule main = y:r {len($y) == 2}\nrule r = \"a\"* \"a\"*\nrule w = \"a\"* \"a\"*",
		want: map[string]string{"main": `y:r {len($y) == 2}`, "r": `"a"* "a"*`, "w": `"a"*`}},
	{name: "a rule called through value positions keeps its repetitions",
		src:  "rule main = y:(\"q\" r / \"z\") {len($y) == 2}\nrule r = s\nrule s = \"a\"* \"a\"*",
		want: map[string]string{"r": `s`, "s": `"a"* "a"*`}},
	{name: "a capture in a lookahead of a called rule keeps the value of its rules",
		src:  "rule main = y:r {len($y.$z) == 2} (e \"q\" / \"x\")* $$\nrule r = &(z:s) \"a\"*\nrule s = \"a\"* \"a\"*\nrule e = e \"x\" / \"x\"",
		want: map[string]string{"s": `"a"* "a"*`}},
	{name: "a call in a lookahead outside a capture is normalized",
		src:  "rule main = y:r {len($y) == 2}\nrule r = &s \"a\"*\nrule s = \"a\"* \"a\"*",
		want: map[string]string{"s": `"a"*`}},
	{name: "a repetition that follows a read capture keeps its scope",
		src:  "rule main = x:\"a\" (x:\"a\")* {text($x) == \"a\"} \"b\"",
		want: map[string]string{"main": `x:"a" (x:"a")* {text($x) == "a"} "b"`}},
	{name: "an optional before a read capture's repetition keeps its scope",
		src:  "rule main = (x:\"a\")? (x:\"a\")* {text($x) == \"a\"} \"b\"",
		want: map[string]string{"main": `(x:"a")? (x:"a")* {text($x) == "a"} "b"`}},
	{name: "a repetition of a repetition of a read capture keeps its shape",
		src:  "rule main = ((x:\"a\")*)* {len($x) == 1} \"b\"",
		want: map[string]string{"main": `(x:"a")** {len($x) == 1} "b"`}, rejected: true},
	{name: "a repetition of an optional read capture keeps its shape",
		src:  "rule main = ((x:\"a\")?)* {len($x) == 1} \"b\"",
		want: map[string]string{"main": `(x:"a")?* {len($x) == 1} "b"`}, rejected: true},
	{name: "a nested repetition of a read capture keeps its shape",
		src:  "rule main = (x:\"a\"*)* {len($x) == 1} \"b\"",
		want: map[string]string{"main": `(x:"a"*)* {len($x) == 1} "b"`}, rejected: true},
	{name: "a repetition of a call that reaches a read capture keeps its shape",
		src:  "rule main = y:r {len($y) == 1} r r*\nrule r = \"a\"",
		want: map[string]string{"main": `y:r {len($y) == 1} r r*`}},
	{name: "a nested repetition of a call that reaches a read capture keeps its shape",
		src:  "rule main = y:r {len($y) == 1} (r*)*\nrule r = \"a\"",
		want: map[string]string{"main": `y:r {len($y) == 1} r**`}},
	{name: "alternatives that share a read capture are not factored",
		src:  "rule main = (x:\"a\" \"b\" / x:\"a\" \"c\") {text($x) == \"a\"}",
		want: map[string]string{"main": `(x:"a" "b" / x:"a" "c") {text($x) == "a"}`}},
	{name: "a rule called under a discard is normalized",
		src:  "rule main = y:(\"q\" -r) {len($y) == 2}\nrule r = \"a\"* \"a\"*",
		want: map[string]string{"r": `"a"*`}},
}

// cheapRuleCases are the cases of TestNormalViewCheapRules: a grammar and the view of its rule
// main.
var cheapRuleCases = []struct {
	name string
	src  string
	want string
}{
	{name: "a duplicate call of a memoized rule is dropped",
		src: "rule main = m / m\nrule m = q \"x\"\nrule q = \"y\"", want: `m`},
	{name: "a duplicate call of a rule that meets an insertion stays",
		src: "rule main = m / m\nrule m = q \"x\"\nrule q = \"y\" #missing", want: `m / m`},
	{name: "a duplicate call of a rule with a cheap body is dropped",
		src: "rule main = a / a\nrule a = \"x\"", want: `a`},
	{name: "a duplicate call of a rule with an action stays",
		src: "rule main = a / a\nrule a = \"x\" => 1", want: `a / a`},
	{name: "a duplicate call of an Operators Rule stays",
		src:  "rule main = e / e\nrule e = #operators {\n  operand \"x\"\n  level add { infix left \"+\" }\n}",
		want: `e / e`},
}

// valueFlowCases are the cases of TestNormalViewPredicateValueFlow.
var valueFlowCases = []struct {
	name string
	src  string
	want map[string]string // the view of each rule named
}{
	{name: "a variable set from a capture keeps the capture's repetitions",
		src:  "var n int\nrule main = x:(\"a\"* \"a\"*) {n := len($x)} {n == 2} \"q\" \"b\"* \"b\"*",
		want: map[string]string{"main": `x:("a"* "a"*) {n := len($x)} {n == 2} "q" "b"*`}},
	{name: "a capture access into a rule's node keeps the rule's repetitions",
		src:  "rule main = x:r {len($x.$y) == 2}\nrule r = y:(\"a\"* \"a\"*)",
		want: map[string]string{"main": `x:r {len($x.$y) == 2}`, "r": `y:("a"* "a"*)`}},
	{name: "a capture access into the elements of a repetition keeps their repetitions",
		src:  "rule main = xs:r* {len(map($xs, (i) => i.$y)) == 2}\nrule r = y:(\"a\"* \"a\"*) \"q\"",
		want: map[string]string{"r": `y:("a"* "a"*) "q"`}},
	{name: "a capture in an inline action keeps its repetitions",
		src:  "rule main = (x:(\"a\"* \"a\"*) {len($x) == 2} => $x)",
		want: map[string]string{"main": `_main_1`, "_main_1": `x:("a"* "a"*) {len($x) == 2}`}},
}

// textCases are the cases of TestNormalViewText: the body of rule main.
var textCases = []struct {
	body string
	want map[string]string // the view's spelling of a built expression, and the text it stands for
}{
	{body: `"is" !"x" "not" !"x" / "is" !"x"`, want: map[string]string{
		`"is" !"x" ("not" !"x" / _)`: `"is" !"x" "not" !"x"`,
		`"not" !"x" / _`:             `"is" !"x" "not" !"x" / "is" !"x"`,
		`_`:                          `"is" !"x"`,
	}},
	{body: `"y" / "0"* "a" "b" / "0"* "a" "c" / "z"`, want: map[string]string{
		`"0"* "a" [b-c]`: `"0"* "a" "b"`,
	}},
	{body: `"ab" / -("cd" #cut "x")`, want: map[string]string{
		`-("cd" "x")`: `-("cd" #cut "x")`,
	}},
}

// cutTupleTail ends the grammars of cutTupleCases.
const cutTupleTail = " (e \"q\" / \"x\")* $$\nrule e = e \"x\" / \"x\"\n"

// cutTupleCases are the cases of TestNormalViewCutKeepsTuple.
var cutTupleCases = []struct {
	name string
	src  string
	want int // the sequences of one item in the view
}{
	{name: "a cut that ends an alternative of a read capture",
		src: "rule main = x:(\"abc\" #cut / \"z\") {len($x) == 1}" + cutTupleTail, want: 1},
	// The alternatives after a leading cut stay in a read capture (T2 of N2 is off there), so
	// the cut stays too and the view keeps the choice as it is.
	{name: "a leading cut of a read capture",
		src: "rule main = x:(#cut \"abc\" / \"z\") {len($x) == 1}" + cutTupleTail, want: 0},
	{name: "a leading cut of the last alternative of a read capture",
		src: "rule main = x:(\"w\" / #cut \"abc\") {len($x) == 1}" + cutTupleTail, want: 1},
	{name: "a cut of the last alternative around a read capture",
		src: "rule main = (\"w\" / x:(\"abc\" #cut)) {len($x) == 1}" + cutTupleTail, want: 1},
	{name: "a cut of the last alternative of a read capture",
		src: "rule main = x:(\"w\" / \"abc\" #cut) {len($x) == 1}" + cutTupleTail, want: 1},
	{name: "a cut of an alternative that calls a rule that reaches a read capture",
		src: "rule main = y:(r #cut / \"z\") {len($y) == 1}\nrule r = \"abc\"" + cutTupleTail, want: 1},
	{name: "a rule that a read capture reaches keeps the tuple of its cut",
		src: "rule main = y:r {len($y) == 1}\nrule r = \"abc\" #cut / \"z\"" + cutTupleTail, want: 1},
	{name: "a capture that no predicate reads loses the tuple with the cut",
		src: "rule main = x:(\"abc\" #cut / \"z\") {true}" + cutTupleTail, want: 0},
	{name: "a cut outside any capture loses the tuple with the cut",
		src: "rule main = (\"abc\" #cut / \"z\")" + cutTupleTail, want: 0},
	{name: "a capture another capture's predicate reads does not keep it",
		src: "rule main = x:(\"abc\" #cut / \"z\") y:\"q\" {len($y) == 1}" + cutTupleTail, want: 0},
}

// valueShapeCases are the cases of TestNormalViewKeepsValueShapes.
var valueShapeCases = []struct {
	name string
	src  string
	opts Options
	want map[string]string
}{
	{name: "a choice of one sequence in a read capture stays",
		src:  "rule main = x:(\"a\" (\"b\" \"c\" / _|_)) {len($x) == 2}",
		want: map[string]string{"main": `x:("a" ("b" "c")) {len($x) == 2}`}},
	// C1, C3 and C5 can drop every alternative with a value and keep one without, which takes
	// the element nil from the choice; K4 gives an alternative the type _|_, which the choice
	// leaves out of its join. Where a predicate can read the value they stay off.
	{name: "a choice of two equal sequences in a read capture stays",
		src:  "rule main = x:(\"a\" (\"b\" \"c\" / \"b\" \"c\")) {len($x) == 2}",
		want: map[string]string{"main": `x:("a" ("b" "c" / "b" "c")) {len($x) == 2}`}},
	{name: "an alternative after one that always succeeds in a read capture stays",
		src:  "rule main = x:(\"q\" (_ / \"b\")) {len($x) == 2}",
		want: map[string]string{"main": `x:("q" (_ / "b")) {len($x) == 2}`}},
	{name: "a subsumed alternative in a read capture stays",
		src:  "rule main = x:(\"q\" (-\"c\" / -\"c\" \"d\")) {len($x) == 2}",
		want: map[string]string{"main": `x:("q" (-"c" / -"c" "d")) {len($x) == 2}`}},
	{name: "a self exclusion in a read capture stays",
		src:  "rule main = x:(\"q\" (-\"b\" / !\"a\" \"a\")) {len($x) == 2}",
		want: map[string]string{"main": `x:("q" (-"b" / !"a" "a")) {len($x) == 2}`}},
	{name: "an alternative after one that always succeeds in a rule that a read capture reaches stays",
		src:  "rule main = y:r {len($y) == 2}\nrule r = \"q\" (_ / \"b\")",
		want: map[string]string{"r": `"q" (_ / "b")`}},
	{name: "an alternative after one that always succeeds in an unread capture is dropped",
		src:  "rule main = x:(\"q\" (_ / \"b\")) {true}",
		want: map[string]string{"main": `x:("q" _) {true}`}},
	{name: "a self exclusion in an unread capture is dropped",
		src:  "rule main = x:(\"q\" (-\"b\" / !\"a\" \"a\")) {true}",
		want: map[string]string{"main": `x:("q" -"b") {true}`}},
	{name: "a choice of one alternative without a value in a read capture stays",
		src:  "rule main = x:(\"a\" (-\"b\" / _|_)) {len($x) == 2}",
		want: map[string]string{"main": `x:("a" (-"b")) {len($x) == 2}`}},
	{name: "a choice of one sequence in a rule that a read capture reaches stays",
		src:  "rule main = y:r {len($y) == 2}\nrule r = \"a\" (\"b\" \"c\" / _|_)",
		want: map[string]string{"r": `"a" ("b" "c")`}},
	// A call of a splice rule is one element as an alternative and adds the elements of its
	// body alone, so the choice stays when the body is a sequence, directly or through another
	// splice rule, and is replaced otherwise.
	{name: "a choice of one call of a splice rule of a sequence in a read capture stays",
		src:  "rule main = x:(\"q\" (_r / _|_)) {len($x) == 2}\nrule _r = \"a\" \"b\"",
		want: map[string]string{"main": `x:("q" (_r)) {len($x) == 2}`}},
	{name: "a choice of one call of a splice rule that calls one of a sequence in a read capture stays",
		src:  "rule main = x:(\"q\" (_r / _|_)) {len($x) == 2}\nrule _r = (_s)\nrule _s = \"a\" \"b\"",
		want: map[string]string{"main": `x:("q" (_r)) {len($x) == 2}`}},
	{name: "a choice of one call of a splice rule of one item in a read capture is replaced",
		src:  "rule main = x:(\"q\" (_r / _|_)) {len($x) == 2}\nrule _r = \"a\"",
		want: map[string]string{"main": `x:("q" _r) {len($x) == 2}`}},
	{name: "a choice of one call of a rule of a sequence in a read capture is replaced",
		src:  "rule main = x:(\"q\" (r / _|_)) {len($x) == 2}\nrule r = \"a\" \"b\"",
		want: map[string]string{"main": `x:("q" r) {len($x) == 2}`}},
	{name: "a choice of one call of a splice rule of a sequence in an unread capture is replaced",
		src:  "rule main = x:(\"q\" (_r / _|_)) {true}\nrule _r = \"a\" \"b\"",
		want: map[string]string{"main": `x:("q" _r) {true}`}},
	{name: "a choice of one literal in a read capture is replaced",
		src:  "rule main = x:(\"a\" (\"b\" / _|_)) {len($x) == 2}",
		want: map[string]string{"main": `x:("a" "b") {len($x) == 2}`}},
	{name: "a choice of one sequence in an unread capture is replaced",
		src:  "rule main = x:(\"a\" (\"b\" \"c\" / _|_)) {true}",
		want: map[string]string{"main": `x:("a" "b" "c") {true}`}},
	{name: "a call in a read capture is not inlined",
		src:  "rule main = x:(\"a\" r) {len($x) == 2}\nrule r = \"b\" \"c\"",
		opts: Options{Inline: true},
		want: map[string]string{"main": `x:("a" r) {len($x) == 2}`}},
	{name: "a call in an unread capture is inlined",
		src:  "rule main = x:(\"a\" r) {true}\nrule r = \"b\" \"c\"",
		opts: Options{Inline: true},
		want: map[string]string{"main": `x:("a" "b" "c") {true}`}},
	{name: "a recovery of a sequence that always succeeds in a read capture stays",
		src:  "rule main = x:(\"q\" (\"a\"? \"b\"?) #recover(skip=\"z\")) {len($x) == 2}",
		want: map[string]string{"main": `x:("q" ("a"? "b"?) #recover(skip="z")) {len($x) == 2}`}},
	// The recovery leaves an expression without a value without one, so dropping it would keep
	// the value; the gate keeps it all the same, as it does not look at the shape of the value.
	{name: "a recovery of an expression without a value that always succeeds in a read capture stays",
		src:  "rule main = x:(\"q\" (-\"a\")? #recover(skip=\"z\")) {len($x) == 1}",
		want: map[string]string{"main": `x:("q" (-"a")? #recover(skip="z")) {len($x) == 1}`}},
	{name: "a recovery of an expression with a read capture that always succeeds stays",
		src:  "rule main = (x:\"a\"? \"b\"?) #recover(skip=\"z\") {len($x) == 1}",
		want: map[string]string{"main": `(x:"a"? "b"?) #recover(skip="z") {len($x) == 1}`}},
	{name: "a recovery of a sequence that always succeeds in an unread capture is dropped",
		src:  "rule main = x:(\"q\" (\"a\"? \"b\"?) #recover(skip=\"z\")) {true}",
		want: map[string]string{"main": `x:("q" "a"? "b"?) {true}`}},
}

// TestNormalViewConditionEdges checks grammars at the edges of the conditions of the rewrites:
// where a condition holds only through a fact that the expression does not show (an equal
// alternative written as a sequence of one item, a call of the loosest level, a rule that is cheap
// only once the rules that it calls are), or fails only through one (a stream in an inlined body,
// a value that a predicate reads through a call). Each grammar is valid.
func TestNormalViewConditionEdges(t *testing.T) {
	for _, c := range conditionEdgeCases {
		t.Run(c.name, func(t *testing.T) {
			got, applied := viewBodies(t, c.src+"\n", c.opts)
			for name, want := range c.want {
				if got[name] != want {
					t.Errorf("view of rule %s = %s, want %s", name, got[name], want)
				}
			}
			if !maps.Equal(applied, c.applied) {
				t.Errorf("rewrites %v, want %v", applied, c.applied)
			}
			if c.texts != nil {
				texts := builtTexts(t, c.src+"\n", c.opts)
				for spelling, want := range c.texts {
					if texts[spelling] != want {
						t.Errorf("%s stands for %q, want %q", spelling, texts[spelling], want)
					}
				}
			}
		})
	}
}

// matchingView is the view of matching: labels dropped and calls inlined.
var matchingView = Options{Labels: true, Inline: true}

const conditionEdgeOperators = "\nrule o = e\nrule e = #operators {\n  operand num\n  level sum { infix left \"+\" }\n  level mul { infix left \"*\" }\n}\nrule num = \"n\""

// conditionEdgeCases are the cases of TestNormalViewConditionEdges.
var conditionEdgeCases = []struct {
	name    string
	src     string
	opts    Options
	want    map[string]string
	applied map[Law]int
	// texts maps the spelling of an expression of rule main that the view builds to the grammar
	// text it stands for.
	texts map[string]string
}{
	{name: "an alternative equal to an earlier one without its cut is dropped",
		src: "@start\nrule main = (x:\"a\" #cut / x:\"a\") {text($x) == \"a\"}", opts: Options{},
		want:    map[string]string{"main": `(x:"a") {text($x) == "a"}`},
		applied: map[Law]int{N3: 1, N6: 1}},
	{name: "repetitions merge before a later repetition of a sequence",
		src: "@start\nrule main = \"x\" \"x\"* \"x\"* (\"y\" \"x\"*)*", opts: Options{},
		want:    map[string]string{"main": `"x"+ ("y" "x"*)*`},
		applied: map[Law]int{N9: 2}},
	{name: "a rule is cheap once the rules that it calls are",
		src: "@start\nrule main = (a*)* \"z\"\nrule a = b \"q\"\nrule b = \"p\"", opts: Options{},
		want:    map[string]string{"main": `a* "z"`},
		applied: map[Law]int{N10: 1}},
	{name: "a call of a rule that calls a cheap rule is factored",
		src: "@start\nrule main = a \"x\" / a \"y\"\nrule a = b \"q\"\nrule b = \"p\"", opts: Options{},
		want:    map[string]string{"main": `a [x-y]`},
		applied: map[Law]int{N12: 1, N8: 1}},
	{name: "a call of the loosest level is factored as the plain call",
		src: "@start\nrule main = level(e, sum) \"x\" / level(e, sum) \"y\" / o" + conditionEdgeOperators, opts: Options{},
		want:    map[string]string{"main": `level(e, sum) [x-y] / o`},
		applied: map[Law]int{N12: 1, N8: 1}},
	{name: "a call of the loosest level is the same as the plain call",
		src: "@start\nrule main = level(e, sum) \"x\" / e \"y\" / o" + conditionEdgeOperators, opts: Options{},
		want:    map[string]string{"main": `level(e, sum) [x-y] / o`},
		applied: map[Law]int{N12: 1, N8: 1}},
	{name: "a call of a tighter level is not factored",
		src: "@start\nrule main = level(e, mul) \"x\" / level(e, mul) \"y\" / o" + conditionEdgeOperators, opts: Options{},
		want:    map[string]string{"main": `level(e, mul) "x" / level(e, mul) "y" / o`},
		applied: map[Law]int{}},
	{name: "a call of a rule whose body is a stream is not factored",
		src: "@start\nrule main = s \"x\" / s \"y\"\nrule s = \"a\"* #stream", opts: Options{},
		want:    map[string]string{"main": `s "x" / s "y"`},
		applied: map[Law]int{}},
	{name: "a rule whose body is a stream is not inlined",
		src: "@start\nrule main = s \"x\" / s \"y\"\nrule s = \"a\"* #stream", opts: matchingView,
		want:    map[string]string{"main": `s "x" / s "y"`},
		applied: map[Law]int{}},
	{name: "an inlined body is rewritten again at each call",
		src: "@start\nrule main = t \";\" t \";\" c\nrule c = x:t {len(text($x)) > 0}\nrule t = \"a\" (\"b\" \"b\"*)", opts: matchingView,
		want:    map[string]string{"main": `"a" "b"+ ";" "a" "b"+ ";" c`, "c": `x:t {len(text($x)) > 0}`},
		applied: map[Law]int{N1: 4, N13: 2, N9: 2}},
	{name: "a label inside a recovery keeps both attributes around a rewritten operand",
		src: "@start\nrule main = ((\"b\" \"a\"* \"a\"*) @error(message=\"m\")) #recover(skip=sync) / \"x\"\nrule sync = [a-z]*", opts: Options{},
		want: map[string]string{"main": `("b" "a"*) @error(message="m") #recover(skip=sync) / "x"`}, applied: map[Law]int{N9: 1}},
	{name: "a label inside a recovery is dropped from the recovery",
		src: "@start\nrule main = ((\"b\" \"a\"* \"a\"*) @error(message=\"m\")) #recover(skip=sync) / \"x\"\nrule sync = [a-z]*", opts: matchingView,
		want: map[string]string{"main": `("b" "a"*) #recover(skip=sync) / "x"`}, applied: map[Law]int{N7: 1, N9: 1},
		texts: map[string]string{`("b" "a"*) #recover(skip=sync)`: `("b" "a"* "a"*) @error(message="m") #recover(skip=sync)`}},
	{name: "a label inside a recovery of a call is dropped from the recovery",
		src: "@start\nrule main = (sep @error(message=\"m\")) #recover(skip=sync) / \"x\"\nrule sep = \";\"\nrule sync = [a-z]*", opts: matchingView,
		want: map[string]string{"main": `";" #recover(skip=sync) / "x"`}, applied: map[Law]int{N13: 1, N7: 1}},
	{name: "a recovery that never runs is dropped from around a label",
		src: "@start\nrule main = ((\"a\"* \"a\"*) @error(message=\"m\")) #recover(skip=sync) \"x\"\nrule sync = [a-z]*", opts: Options{},
		want: map[string]string{"main": `"a"* @error(message="m") "x"`}, applied: map[Law]int{N1: 1, N6: 1, N9: 1},
		texts: map[string]string{`"a"* @error(message="m")`: `("a"* "a"*) @error(message="m") #recover(skip=sync)`}},
	{name: "a label on a stream keeps the stream around a rewritten element",
		src: "@start\nrule main = ((\"a\" \"a\"*)* #stream) @error(message=\"m\")", opts: matchingView,
		want: map[string]string{"main": `"a"* #stream`}, applied: map[Law]int{N1: 1, N10: 1, N7: 1, N9: 1}},
	{name: "a label on an insertable literal is dropped",
		src: "@start\nrule main = (\"k\" #missing) @error(message=\"m\") \"a\"* \"a\"*", opts: matchingView,
		want: map[string]string{"main": `"k" #missing "a"*`}, applied: map[Law]int{N7: 1, N9: 1}},
}
