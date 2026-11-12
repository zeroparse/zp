package compile

import (
	"slices"
	"sync"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/syntax"
)

// viewOf returns the view of the body of rule main of src with opts, and the rewrites of each
// rule.
func viewOf(t *testing.T, src string, opts viewOptions) (string, map[viewRule]int) {
	t.Helper()
	bodies, applied := viewBodies(t, src, opts)
	body, ok := bodies["main"]
	if !ok {
		t.Fatal("no rule main")
	}
	return body, applied
}

// viewBodies returns the view of the body of every rule of src with opts, and the rewrites.
func viewBodies(t *testing.T, src string, opts viewOptions) (map[string]string, map[viewRule]int) {
	t.Helper()
	g, err := syntax.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	g, err = desugar.NormalizeAttributesControlled(g, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	rules := g.Rules()
	rf := testRuleFacts(t, g, rules)
	v := normalizeRules(rules, rf.an, rf.memo, opts, nil)
	bodies := map[string]string{}
	for _, r := range v.rules {
		bodies[r.Name] = grammar.FormatExpr(r.Expr)
	}
	return bodies, v.applied
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
			got, _ := viewBodies(t, c.src+"\n", costView)
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
// normalized. The positional references $0 and $1, which could read an element of a sequence, are
// removed from the language and rejected by the parser, so they cannot reach the view.
func TestNormalViewPredicateValueFlow(t *testing.T) {
	for _, c := range valueFlowCases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := viewBodies(t, c.src+"\n", costView)
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

// TestNormalViewText checks that every expression that the view builds stands for text of the
// grammar, which messages quote: a factored rest for its whole alternative, a factored sequence for
// the first alternative of its run, and an alternative without its cuts for itself.
func TestNormalViewText(t *testing.T) {
	for _, c := range textCases {
		t.Run(c.body, func(t *testing.T) {
			g, err := syntax.Parse("rule main = " + c.body + "\n")
			if err != nil {
				t.Fatal(err)
			}
			rules := g.Rules()
			rf := testRuleFacts(t, g, rules)
			v := normalizeRules(rules, rf.an, rf.memo, costView, nil)
			orig := map[grammar.Expr]bool{}
			WalkExpr(rules[0].Expr, func(x grammar.Expr) { orig[x] = true })
			got := map[string]string{}
			WalkExpr(v.rules[0].Expr, func(x grammar.Expr) {
				if orig[x] {
					return
				}
				text, ok := v.text[x]
				if !ok {
					t.Errorf("%s, built by the view, stands for no text of the grammar", grammar.FormatExpr(x))
				}
				got[grammar.FormatExpr(x)] = text
			})
			for spelling, text := range c.want {
				if got[spelling] != text {
					t.Errorf("%s stands for %q, want %q (view %s)", spelling, got[spelling], text, grammar.FormatExpr(v.rules[0].Expr))
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
			g, err := syntax.Parse(c.src)
			if err != nil {
				t.Fatal(err)
			}
			g, err = desugar.NormalizeAttributesControlled(g, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			rules := g.Rules()
			rf := testRuleFacts(t, g, rules)
			v := normalizeRules(rules, rf.an, rf.memo, costView, nil)
			got := 0
			for _, r := range v.rules {
				WalkExpr(r.Expr, func(x grammar.Expr) {
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
	laws := []viewRule{ruleN1, ruleN2, ruleN3, ruleN4, ruleN5, ruleN6, ruleN7, ruleN8, ruleN9, ruleN10, ruleN11, ruleN12, ruleN13}
	var mu sync.Mutex
	seen := map[viewRule]bool{}
	t.Run("corpus", func(t *testing.T) {
		for _, path := range append(slices.Clone(factstest.Corpus), "laws") {
			t.Run(path, func(t *testing.T) {
				t.Parallel()
				src := lawsGrammar
				if path != "laws" {
					src = factstest.Read(t, path)
				}
				g, err := syntax.Parse(src)
				if err != nil {
					t.Fatal(err)
				}
				for _, matching := range []bool{false, true} {
					all, _, err := normalViewOf(g, matching, nil, nil)
					if err != nil {
						t.Fatal(err)
					}
					mu.Lock()
					for r, n := range all.applied {
						seen[r] = seen[r] || n > 0
					}
					mu.Unlock()
					none, _, _ := normalViewOf(g, matching, []string{}, nil)
					if len(none.applied) != 0 {
						t.Errorf("matching %v, no law: %v", matching, none.applied)
					}
					_, rules, err := classifierRules(g, nil)
					if err != nil {
						t.Fatal(err)
					}
					for i, r := range none.rules {
						if got, want := grammar.FormatExpr(r.Expr), grammar.FormatExpr(rules[i].Expr); got != want {
							t.Errorf("matching %v, no law: rule %s is %s, want %s", matching, r.Name, got, want)
						}
					}
					for _, l := range laws {
						one, _, _ := normalViewOf(g, matching, []string{string(l)}, nil)
						for r, n := range one.applied {
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
		if l != ruleN7 && !seen[l] {
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
	rule viewRule // the rule that applies, or "" for a negative case
	body string
	want string
	opts viewOptions
}{
	{name: "S1 splices a grouped sequence", rule: ruleN1, body: `"ab" ("cd" "ef")`, want: `"ab" "cd" "ef"`},
	{name: "C2 flattens a right-nested choice", rule: ruleN1, body: `"ab" / ("cd" #cut "x" / "ef")`, want: `"ab" / "cd" #cut "x" / "ef"`},
	{name: "C2 flattens a left-nested choice without cuts", rule: ruleN1, body: `("ab" / "cd") / "ef"`, want: `"ab" / "cd" / "ef"`},
	{name: "C2 keeps a left-nested choice with a cut", body: `("ab" #cut "x" / "cd") / "ef"`, want: `("ab" #cut "x" / "cd") / "ef"`},
	{name: "C3 drops alternatives after one that always succeeds", rule: ruleN2, body: `"ab"* / "cd"`, want: `"ab"*`},
	{name: "C3 keeps alternatives after a repetition with a loose cut", body: `("ab" #cut "x")* / "cd"`, want: `("ab" #cut "x")* / "cd"`},
	{name: "C1 drops a duplicate alternative", rule: ruleN3, body: `"ab" / "cd" / "ab"`, want: `"ab" / "cd"`},
	{name: "C5 drops an alternative that an earlier one prefixes", rule: ruleN3, body: `"ab" / "ab" "cd"`, want: `"ab"`},
	{name: "C5 keeps a literal that no earlier alternative prefixes", body: `"=" "x" / "==" "y" / "="`, want: `"=" "x" / "==" "y" / "="`},
	{name: "C5 drops a longer literal", rule: ruleN3, body: `"=" / "=="`, want: `"="`},
	{name: "C1 keeps a duplicate with a predicate", body: `{true} "ab" / {true} "ab"`, want: `{true} "ab" / {true} "ab"`},
	{name: "C1 keeps a duplicate that calls a rule with a predicate", body: `p / p`, want: `p / p`},
	{name: "C4 drops a bottom alternative", rule: ruleN4, body: `_|_ / "ab" / "cd"`, want: `"ab" / "cd"`},
	{name: "C4 keeps a labelled bottom", body: `_|_ @error("e") / "ab"`, want: `_|_ @error(message="e") / "ab"`},
	{name: "S2 drops what follows a bottom", rule: ruleN4, body: `"ab" _|_ "cd"`, want: `"ab" _|_`},
	{name: "K2 removes a positive lookahead under a lookahead", rule: ruleN5, body: `!&"ab" "c"`, want: `!"ab" "c"`},
	{name: "K2 keeps the negation of a negation under a positive lookahead", rule: ruleN5, body: `&!"ab" "c"`, want: `!"ab" "c"`},
	{name: "K1 turns a double negation into a positive lookahead", rule: ruleN5, body: `!!"ab" "c"`, want: `&"ab" "c"`},
	{name: "K1 keeps a double negation of a binding", body: `!!({v := 1}) "c"` + "\nvar v int", want: `!!{v := 1} "c"`},
	{name: "T2 drops a cut that ends an alternative", rule: ruleN6, body: `"ab" #cut / "cd"`, want: `"ab" / "cd"`},
	{name: "T2 drops a cut in the last alternative", rule: ruleN6, body: `"ab" / "cd" #cut "ef"`, want: `"ab" / "cd" "ef"`},
	{name: "T2 drops the alternatives after a leading cut", rule: ruleN6, body: `#cut "ab" / "cd"`, want: `"ab"`},
	{name: "T2 drops a cut nested in the last alternative", rule: ruleN6, body: `"ab" / -("cd" #cut "x")`, want: `"ab" / -("cd" "x")`},
	{name: "T2 drops a nested cut of an only alternative left in a repetition", rule: ruleN6, body: `(-("a" #cut "b") / !"q" "q")* "ac"`, want: `(-("a" "b"))* "ac"`},
	{name: "T2 drops a nested cut of an only alternative left in an option", rule: ruleN6, body: `(x:("a" #cut "b") / !"q" "q")? "ac"`, want: `(x:("a" "b"))? "ac"`},
	{name: "T2 drops a nested cut of an only alternative spliced into a choice", rule: ruleN6, body: `((-("x" #cut "y" "q") / !"z" "z") / "x")* $$`, want: `(-("x" "y" "q") / "x")* $$`},
	{name: "T2 keeps a choice whose only alternative left has a cut in a skip", body: `("a" "b" #recover(skip=#cut [^;]*) / !"q" "q")* "ac"`, want: `("a" "b" #recover(skip=#cut [^;]*) / !"q" "q")* "ac"`},
	{name: "T1 keeps a cut that commits", body: `"ab" #cut "x" / "cd"`, want: `"ab" #cut "x" / "cd"`},
	{name: "V1 drops a recovery that never fires", rule: ruleN6, body: `"ab"* #recover(skip=[^;]*)`, want: `"ab"*`},
	{name: "V1 keeps a recovery that can fire", body: `"ab" #recover(skip=[^;]*)`, want: `"ab" #recover(skip=[^;]*)`},
	{name: "N7 drops labels for analyses of matching", rule: ruleN7, body: `"ab" @error("e")`, want: `"ab"`, opts: viewOptions{labels: true}},
	{name: "N7 is off for the classifier", body: `"ab" @error("e")`, want: `"ab" @error(message="e")`},
	{name: "C9 merges single characters", rule: ruleN8, body: `"a" / [0-9] / "b" "c"`, want: `[0-9a] / "b" "c"`},
	{name: "C9 keeps a literal without case", body: `"a"i / "b"i`, want: `"a"i / "b"i`},
	{name: "C9 keeps a negated class", body: `[^a] / "b"`, want: `[^a] / "b"`},
	{name: "C9 keeps the replacement character", body: `"\u{fffd}" / "b"`, want: "\"\ufffd\" / \"b\""},
	{name: "R3 merges repetitions in a row", rule: ruleN9, body: `"ab"* "ab"*`, want: `"ab"*`},
	{name: "R4 merges a plus and a star", rule: ruleN9, body: `"ab"+ "ab"*`, want: `"ab"+`},
	{name: "R4 merges an option and a star", rule: ruleN9, body: `"ab"? "ab"*`, want: `"ab"*`},
	{name: "R5 folds an item and its repetition", rule: ruleN9, body: `"ab" "ab"*`, want: `"ab"+`},
	{name: "R3 keeps repetitions with a loose cut", body: `("ab" #cut "x")* ("ab" #cut "x")*`, want: `("ab" #cut "x")* ("ab" #cut "x")*`},
	{name: "R3 keeps nullable repetitions that bind", body: `({v == 0} {v := 1} / {v == 1} "a")* ({v == 0} {v := 1} / {v == 1} "a")*` + "\nvar v int",
		want: `({v == 0} {v := 1} / {v == 1} "a")* ({v == 0} {v := 1} / {v == 1} "a")*`},
	{name: "R3 keeps repetitions of a rule with a predicate", body: `p* p*`, want: `p* p*`},
	{name: "R3 merges repetitions of a cheap rule", rule: ruleN9, body: `r* r*`, want: `r*`},
	{name: "R6 flattens a star of a star", rule: ruleN10, body: `("ab"*)*`, want: `"ab"*`},
	{name: "R6 flattens a star of an option", rule: ruleN10, body: `("ab"?)*`, want: `"ab"*`},
	{name: "R6 keeps a nested repetition with a loose cut", body: `(("ab" #cut "x")*)*`, want: `("ab" #cut "x")**`},
	{name: "R7 unrolls whitespace", rule: ruleN11, body: `[ ]* ("//" [^\n]* [ ]*)*`, want: `([ ] / "//" [^\n]*)*`},
	{name: "R7 absorbs a repetition into a choice", rule: ruleN11, body: `"ab"* ("ab" / "cd")*`, want: `("ab" / "cd")*`},
	{name: "R7 keeps a nullable element", body: `("ab" / "")* ("cd" ("ab" / "")*)*`, want: `("ab" / "")* ("cd" ("ab" / "")*)*`},
	{name: "R7 keeps a loose cut", body: `[ ]* ("//" #cut [^\n]* [ ]*)*`, want: `[ ]* ("//" #cut [^\n]* [ ]*)*`},
	{name: "R7 keeps a separator that binds", body: `x* ({v := 1} x*)*` + "\nvar v int = 0\nrule x = {v == 1} a\nrule a = \"a\"",
		want: `x* ({v := 1} x*)*`},
	{name: "R7 keeps an element with a predicate", body: `p* ("c" p*)*`, want: `p* ("c" p*)*`},
	{name: "R7 unrolls a cheap rule", rule: ruleN11, body: `r* ("c" r*)*`, want: `(r / "c")*`},
	{name: "K4 turns self exclusion into bottom", rule: ruleN12, body: `!"ab" "ab"`, want: `_|_`},
	{name: "K4 keeps self exclusion of a call", body: `!r r`, want: `!r r`},
	{name: "C6 factors a shared first item", rule: ruleN12, body: `"(" "xy" / "(" "zw"`, want: `"(" ("xy" / "zw")`},
	{name: "C6 factors a call of a cheap rule", rule: ruleN12, body: `r "xy" / r "zw" / "q"`, want: `r ("xy" / "zw") / "q"`},
	{name: "C6 keeps a cut", body: `"(" #cut "xy" / "(" "zw"`, want: `"(" #cut "xy" / "(" "zw"`},
	{name: "C6 keeps a predicate prefix", body: `{true} "xy" / {true} "zw"`, want: `{true} "xy" / {true} "zw"`},
	{name: "C6 keeps a prefix that calls a rule with a predicate", body: `p "xy" / p "zw"`, want: `p "xy" / p "zw"`},
	{name: "P1 inlines a rule that is not memoized", rule: ruleN13, body: `a "x"` + "\nrule a = \"y\" \"z\"", want: `"y" "z" "x"`, opts: viewOptions{inline: true}},
	{name: "N13 is off for the classifier", body: `a "x"` + "\nrule a = \"y\" \"z\"", want: `a "x"`},
	{name: "P1 keeps a rule with a capture", body: `a "x"` + "\nrule a = c:\"y\" \"z\"", want: `a "x"`, opts: viewOptions{inline: true}},
	{name: "P1 is off for the classifier", body: `a "x"` + "\nrule a = \"y\" \"z\"", want: `a "x"`},
}

// predicateCaptureCases are the cases of TestNormalViewPredicateCaptures.
var predicateCaptureCases = []struct {
	name string
	src  string
	want map[string]string // the view of each rule named
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
		want: map[string]string{"main": `(x:"a")** {len($x) == 1} "b"`}},
	{name: "a repetition of an optional read capture keeps its shape",
		src:  "rule main = ((x:\"a\")?)* {len($x) == 1} \"b\"",
		want: map[string]string{"main": `(x:"a")?* {len($x) == 1} "b"`}},
	{name: "a nested repetition of a read capture keeps its shape",
		src:  "rule main = (x:\"a\"*)* {len($x) == 1} \"b\"",
		want: map[string]string{"main": `(x:"a"*)* {len($x) == 1} "b"`}},
	{name: "a repetition of a call that reaches a read capture keeps its shape",
		src:  "rule main = y:r {len($y) == 1} r r*\nrule r = \"a\"",
		want: map[string]string{"main": `y:r {len($y) == 1} r r*`}},
	{name: "alternatives that share a read capture are not factored",
		src:  "rule main = (x:\"a\" \"b\" / x:\"a\" \"c\") {text($x) == \"a\"}",
		want: map[string]string{"main": `(x:"a" "b" / x:"a" "c") {text($x) == "a"}`}},
	{name: "a rule called under a discard is normalized",
		src:  "rule main = y:(\"q\" -r) {len($y) == 2}\nrule r = \"a\"* \"a\"*",
		want: map[string]string{"r": `"a"*`}},
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
		want: map[string]string{"main": `(x:("a"* "a"*) {len($x) == 2} => $x)`}},
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
	opts viewOptions
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
		opts: viewOptions{inline: true},
		want: map[string]string{"main": `x:("a" r) {len($x) == 2}`}},
	{name: "a call in an unread capture is inlined",
		src:  "rule main = x:(\"a\" r) {true}\nrule r = \"b\" \"c\"",
		opts: viewOptions{inline: true},
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
