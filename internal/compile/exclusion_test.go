package compile

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/internal/syntax"
)

// TestDecideAlt checks the exclusion analysis on the first choice of rule main.
func TestDecideAlt(t *testing.T) {
	const rules = "rule ident = [a-z]+\nrule ws = [ ]*\nrule x = \"x\"\nrule kw = \"if\" / \"in\"\nrule r = \"y\"\nrule lr = lr \"+\" \"a\" / \"a\"\n"
	for _, c := range []struct {
		name, main string
		decided    bool
		k, prefix  int
	}{
		{"disjoint first characters", `"a" x / "b" x`, true, 1, 1},
		{"shared call then literals", `ident "(" x / ident "." x`, true, 2, 2},
		{"shared calls through a rule body", `ident ws "(" x / ident ws "." x`, true, 3, 3},
		{"beyond the bound", `ident ws ws "(" x / ident ws ws "." x`, false, 0, 0},
		{"literal prefix of the later one", `"=" . / "==" x`, false, 0, 0},
		{"literal prefix decided after it", `"=" x / "==" x`, true, 2, 2},
		{"longer literal first", `"==" x / "=" x`, true, 2, 1},
		{"negative lookahead against the later alternative", `ident !(ws "(") x / ident ws "(" x`, true, 2, 2},
		{"negative lookahead against a rule alternative", `!kw ident x / "in" x`, true, 1, 1},
		{"negative lookahead against a literal that extends it", `!"i" ident x / "if" x`, true, 1, 1},
		{"later alternative without the lookahead body", `ident !(ws "(") x / ident ws x`, false, 0, 0},
		{"nullable first item", `ws "a" / ws "b"`, true, 2, 2},
		{"option in the later alternative", `"a" x / "b"? x`, true, 1, 1},
		{"option that the alternative can match", `"a" x / "a"? x`, false, 0, 0},
		{"negative lookahead on a class against a literal", `ident ![.(] x / ident "." x`, true, 2, 2},
		{"literal against a class", `"ab" x / [a-z] "c" x`, true, 2, 1},
		{"equal repetitions", `[0-9]+ "." x / [0-9]+ "e" x`, true, 2, 2},
		{"repetition in the later alternative", `ident "(" x / ident* "." x`, false, 0, 0},
		{"overlapping classes then literals", `[a-z] "(" x / [a-f0-9] "." x`, true, 2, 2},
		{"any character against a class", `. "(" x / [0-9] "." x`, true, 2, 2},
		{"overlapping classes then the same literal", `[a-z] "(" x / [a-f0-9] "(" x`, false, 0, 0},
		// A negative lookahead whose body the later alternative matches only through a call: the
		// match goes on inside the callee from where the symbols before it left off.
		{"negative lookahead through a callee", `!("x" "y") "x" r / "x" r "q"`, true, 1, 1},
		{"negative lookahead that a callee does not match", `!("x" "x") "x" r / "x" r "q"`, false, 0, 0},
		{"negative lookahead decided later", `!("x" "x") "x" "z" / "x" r`, true, 2, 3},
		{"later negative lookahead through a callee", `"x" r "z" / !("x" "y") "x" "y"`, true, 1, 2},
		{"later negative lookahead that a callee does not match", `"x" r "z" / !("x" "x") "x" "y"`, false, 0, 0},
		// Two calls of a rule of a left-recursion cycle at one position can differ inside a growth.
		{"calls of a left-recursive rule", `lr "(" x / lr "." x`, false, 0, 0},
		{"equal literals that ignore case", `"if"i "(" x / "IF"i "." x`, true, 2, 2},
	} {
		src := "rule main = (" + c.main + ") x\n" + rules
		g, err := syntax.Parse(src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		k.computeBounded()
		k.computeFirst()
		var ch *grammar.Choice
		WalkExpr(k.byName["main"].Expr, func(x grammar.Expr) {
			if y, ok := x.(*grammar.Choice); ok && ch == nil {
				ch = y
			}
		})
		d := k.decideAlt(ch, 0, 3)
		if d.decided != c.decided || d.decided && (d.k != c.k || d.prefix != c.prefix) {
			t.Errorf("%s: decided %v k %d prefix %d, want %v k %d prefix %d", c.name, d.decided, d.k, d.prefix, c.decided, c.k, c.prefix)
		}
	}
}

// TestDecideAltTrivia checks that the analysis passes calls of a trivia rule without counting them
// and drops a call that follows a call of the same trivia rule, and that rules of other shapes stay
// ordinary calls.
func TestDecideAltTrivia(t *testing.T) {
	defer func() { triviaOn, triviaOnly = false, nil }()
	const rules = "var v int = 0\nrule ident = [a-z]+\nrule x = \"x\"\nrule ws = [ ]*\nrule sp = \" \"\n" +
		"rule ws2 = (sp / \"\\t\")*\nrule wsp = [ ]+\nrule wsv = ({v == 0} \" \")*\nrule wsc = (c:\" \")*\n" +
		"rule wsb = ({v == 0} \" \" {v := 1} / {v == 0} \"\\t\")*\n"
	for _, c := range []struct {
		name, main string
		only       map[string]bool
		off, on    bool
	}{
		{"two calls between the shared prefix and the literals", `ident ws ws "(" x / ident ws ws "." x`, nil, false, true},
		{"calls around the shared literal", `ws "," ws "(" x / ws "," ws "." x`, nil, false, true},
		{"a call that the other side does not make", `ident ws ws "(" x / ident ws "." x`, nil, true, true},
		{"adjacent calls against one call", `ident ws ws "(" x / ident ws "(" "." x`, nil, false, true},
		{"a rule that calls a rule", `ident ws2 ws2 "(" x / ident ws2 ws2 "." x`, nil, false, true},
		{"a rule outside the list", `ident ws ws "(" x / ident ws ws "." x`, map[string]bool{"ws2": true}, false, false},
		{"a repetition that must match once", `ident wsp wsp "(" x / ident wsp wsp "." x`, nil, false, false},
		{"a repetition that reads a variable", `ident wsv wsv "(" x / ident wsv wsv "." x`, nil, false, true},
		{"a repetition that reads a variable after a binding", `ident {v := 1} wsv wsv "(" x / ident wsv wsv "." x`, nil, false, false},
		{"a repetition that binds a variable that it reads", `ident wsb wsb "(" x / ident wsb wsb "." x`, nil, false, false},
		{"a repetition that captures", `ident wsc wsc "(" x / ident wsc wsc "." x`, nil, false, true},
		{"calls of two trivia rules", `ident ws ws2 "(" x / ident ws2 ws "." x`, nil, false, false},
	} {
		g, err := syntax.Parse("rule main = (" + c.main + ") x\n" + rules)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for _, on := range []bool{false, true} {
			triviaOn, triviaOnly = on, c.only
			k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			k.computeBounded()
			k.computeFirst()
			var ch *grammar.Choice
			WalkExpr(k.byName["main"].Expr, func(x grammar.Expr) {
				if y, ok := x.(*grammar.Choice); ok && ch == nil {
					ch = y
				}
			})
			want := c.off
			if on {
				want = c.on
			}
			if d := k.decideAlt(ch, 0, 3); d.decided != want {
				t.Errorf("%s: trivia %v: decided %v (k %d), want %v", c.name, on, d.decided, d.k, want)
			}
		}
	}
}

// TestDecidedUnitsTrivia checks a trailing separator with whitespace on both sides of the comma:
// the strict analysis passes the calls of ws without counting them, and the call of ws before
// the closing bracket after the one of the separator matches nothing.
func TestDecidedUnitsTrivia(t *testing.T) {
	defer func() { decidedUnitsOn, triviaOn = false, false }()
	src := "rule main = value $$\nrule value = \"[\" ws elems? (ws \",\")? ws \"]\" ws / [a-z]+ ws\n" +
		"rule elems = value (ws \",\" ws value)*\nrule ws = [ ]*\n"
	decidedUnitsOn = true
	for _, c := range []struct {
		on   bool
		want ComplexityClass
	}{{false, ClassUnknown}, {true, ClassLinear}} {
		triviaOn = c.on
		if got := classifySource(t, src).Class; got != c.want {
			t.Errorf("trivia %v: class %v, want %v", c.on, got, c.want)
		}
	}
}

// TestTriviaCheap checks that a call of a trivia rule that calls a rule is cheap only where the
// trivia rules are memoized per position: the second call at a position then costs a memo access.
func TestTriviaCheap(t *testing.T) {
	defer func() { triviaOn, triviaMemo = false, false }()
	g, err := syntax.Parse("rule main = ws2 \"x\"\nrule ws2 = (sp)*\nrule sp = \" \"\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, on := range []bool{false, true} {
		triviaOn, triviaMemo = true, on
		k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
		if err != nil {
			t.Fatal(err)
		}
		k.classify()
		if got := newExcl(k, true, 3).cheap(&grammar.Ref{Name: "ws2"}); got != on {
			t.Errorf("memo %v: cheap %v", on, got)
		}
	}
}

// TestCollapseTrivia checks that a call of a trivia rule drops the calls of the same rule that
// follow it across symbols that consume nothing, and only those.
func TestCollapseTrivia(t *testing.T) {
	defer func() { triviaOn = false }()
	triviaOn = true
	g, err := syntax.Parse("rule main = ws ws2 \"x\"\nrule ws = [ ]*\nrule ws2 = [\\t]*\n")
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	x := newExcl(k, true, 3)
	ws, ws2, lit := &grammar.Ref{Name: "ws"}, &grammar.Ref{Name: "ws2"}, &grammar.Literal{Value: "x"}
	p := []sym{{symMatch, ws, 0}, {symSkip, nil, 0}, {symMatch, ws, 1}, {symNot, lit, 1}, {symMatch, ws, 2},
		{symMatch, ws2, 2}, {symMatch, ws, 3}, {symMatch, lit, 3}, {symMatch, ws, 4}}
	if got, want := fmtPath(x.collapseTrivia(p, 0)), fmtPath([]sym{p[0], p[1], p[3], p[5], p[6], p[7], p[8]}); got != want {
		t.Errorf("collapsed %s, want %s", got, want)
	}
	if len(p) != 9 || p[2].e != ws || p[2].item != 1 {
		t.Errorf("the path was changed in place: %s", fmtPath(p))
	}
}

// TestCollapseTriviaBinding checks that a binding between two calls of a trivia rule that reads
// variables keeps the second call: it starts with other variables, so it can match.
func TestCollapseTriviaBinding(t *testing.T) {
	defer func() { triviaOn = false }()
	triviaOn = true
	g, err := syntax.Parse("var v int = 0\nrule main = wv ws \"x\"\nrule wv = ({v == 0} \" \")*\nrule ws = [ ]*\n")
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	x := newExcl(k, true, 3)
	wv, ws := &grammar.Ref{Name: "wv"}, &grammar.Ref{Name: "ws"}
	for _, c := range []struct {
		name string
		p    []sym
		want int
	}{
		{"a rule that reads variables", []sym{{symMatch, wv, 0}, {symBind, nil, 1}, {symMatch, wv, 2}}, 3},
		{"a rule that reads none", []sym{{symMatch, ws, 0}, {symBind, nil, 1}, {symMatch, ws, 2}}, 2},
		{"no binding", []sym{{symMatch, wv, 0}, {symSkip, nil, 1}, {symMatch, wv, 2}}, 2},
	} {
		if got := x.collapseTrivia(c.p, 0); len(got) != c.want {
			t.Errorf("%s: collapsed to %s", c.name, fmtPath(got))
		}
	}
}

// TestTriviaWhy checks which rule shapes are trivia rules.
func TestTriviaWhy(t *testing.T) {
	src := "var v int = 0\nrule main = ws ws2 wsp wsv wsc wsa wsr\nrule ws = ([ ] / \"#\" (!\"\\n\" .)*)*\n" +
		"rule sp = \" \"\nrule ws2 = (sp)*\nrule wsp = [ ]+\nrule wsv = ({v == 0} \" \")*\nrule wsc = (c:\" \")*\n" +
		"rule wsa = [ ]* => 1\nrule wsr = (sp / r)*\nrule r = \"(\" v2 \")\"\nrule v2 = {v == 1} \"a\"\n"
	g, err := syntax.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	for rule, want := range map[string]string{
		"ws": "", "ws2": "", "wsp": "body is not X*", "wsv": "", "wsc": "", "wsa": "action (wsa)", "wsr": "",
	} {
		if got := k.triviaWhy(rule); got != want {
			t.Errorf("%s: %q, want %q", rule, got, want)
		}
	}
}

// TestTriviaWhyCut checks that a #cut of the body of a rule keeps it from being a trivia rule:
// it can commit the repetition, which then fails.
func TestTriviaWhyCut(t *testing.T) {
	g, err := syntax.Parse("rule main = wscut wsin\nrule wscut = (\"#\" #cut [a]*)*\nrule wsin = (in)*\nrule in = \"#\" #cut [a]*\n")
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	// A cut in a called rule is local to the call, which fails as a whole.
	for rule, want := range map[string]string{"wscut": "#cut (wscut)", "wsin": ""} {
		if got := k.triviaWhy(rule); got != want {
			t.Errorf("%s: %q, want %q", rule, got, want)
		}
	}
}

// TestTriviaWhyVariables checks the conditions on variables: reads of variables, captures and
// predicates over them are allowed, and a body that binds a variable that it reads is not, as in
// the rule wsb, whose iteration over b binds the variable that its iteration over a reads.
func TestTriviaWhyVariables(t *testing.T) {
	src := "var v int = 0\nrule main = wsv wsc wsb wsw\nrule wsv = ({v == 0} \" \")*\n" +
		"rule wsc = (c:[ a] {$c == \" \"})*\nrule wsb = ({v == 0} \"b\" {v := 1} / {v == 0} \"a\")*\n" +
		"rule wsw = ({w := 1} \" \")*\nvar w int = 0\n"
	g, err := syntax.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	for rule, w := range map[string]string{"wsv": "", "wsc": "", "wsb": "binds a variable that it reads (wsb)", "wsw": ""} {
		if got := k.triviaWhy(rule); got != w {
			t.Errorf("%s: %q, want %q", rule, got, w)
		}
	}
}

// TestTriviaShapeExtended checks the shapes beyond X*: an atomic X* and the unrolled form
// X* (Y X*)*, but not a form whose last repetition differs from the first.
func TestTriviaShapeExtended(t *testing.T) {
	g, err := syntax.Parse("rule main = wsu wsa wsb wsn\nrule wsu = [ ]* (\"#\" [^\\n]* [ ]*)*\n" +
		"rule wsa = \" \"* #atomic\nrule wsb = [ ]* (\"#\" [a]*)*\nrule wsn = ([ ]?)* (\"#\" ([ ]?)*)*\n")
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	// The unrolled form needs an X that cannot match empty.
	for rule, ok := range map[string]bool{"wsu": true, "wsa": true, "wsb": false, "wsn": false} {
		if got := k.triviaWhy(rule) == ""; got != ok {
			t.Errorf("%s: trivia %v, want %v", rule, got, ok)
		}
	}
}

// TestDecideAltVariables checks that calls of a rule that reads a variable are equal symbols only
// where neither compared prefix binds a variable: the rule b below matches "aa" where x is 1 and
// "a" elsewhere, so after a binding the two calls of b can end at different positions.
func TestDecideAltVariables(t *testing.T) {
	const rules = "var x int = 0\nrule x1 = \"x\"\nrule b = {x == 1} \"aa\" / [a-z]\n" +
		"rule c = {x := 1} b\n"
	for _, c := range []struct {
		name, main string
		decided    bool
		k, prefix  int
	}{
		{"equal calls without bindings", `b "(" x1 / b "a(" x1`, true, 2, 2},
		{"a binding in the earlier prefix", `{x := 1} b "(" x1 / b "a(" x1`, false, 0, 0},
		{"a binding in the later prefix", `b "(" x1 / {x := 1} b "a(" x1`, false, 0, 0},
		{"a binding inside a called rule", `c "(" x1 / b "a(" x1`, false, 0, 0},
	} {
		src := "rule main = (" + c.main + ") x1\n" + rules
		g, err := syntax.Parse(src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		k.computeBounded()
		k.computeFirst()
		var ch *grammar.Choice
		WalkExpr(k.byName["main"].Expr, func(x grammar.Expr) {
			if y, ok := x.(*grammar.Choice); ok && ch == nil {
				ch = y
			}
		})
		d := k.decideAlt(ch, 0, 3)
		if d.decided != c.decided || d.decided && (d.k != c.k || d.prefix != c.prefix) {
			t.Errorf("%s: decided %v k %d prefix %d, want %v k %d prefix %d", c.name, d.decided, d.k, d.prefix, c.decided, c.k, c.prefix)
		}
	}
}

// scanSeparator is a trailing separator whose comma is followed by a scan, which a continuation
// that begins with the same separator would scan again.
const scanSeparator = "rule main = value $$\nrule value = \"[\" ws elems? (\",\" [a-z]* ws)? \"]\" ws / [a-z]+ ws\n" +
	"rule elems = value (\",\" [a-z]* ws value)*\nrule ws = [ ]*\n"

// TestDecidedUnitsCost checks the trailing separator after a scan at a bound that the comparison
// does not reach: the iteration parts from its continuation only past the scan, which the strict
// analysis does not look into, so the unit stays not committing for the cost condition alone.
func TestDecidedUnitsCost(t *testing.T) {
	defer exclHooks{decided: true, k: 8}.set()()
	g, err := syntax.Parse(scanSeparator)
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(starttest.MarkAll(g), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c := k.classify(); c.Class != ClassUnknown {
		t.Errorf("class %v, want %v", c.Class, ClassUnknown)
	}
	var found bool
	for _, u := range k.units {
		if !u.nc || u.kind != unitIteration {
			continue
		}
		found = true
		if loose := exclBlocker(k, u, false, 8); loose != "" {
			t.Errorf("the comparison without the strict conditions stops at %q", loose)
		}
		if strict := exclBlocker(k, u, true, 8); strict != "qstop: part that is not cheap" {
			t.Errorf("the strict comparison stops at %q", strict)
		}
	}
	if !found {
		t.Error("no iteration is not committing")
	}
}

// TestDecidedUnits checks that a unit that the strict analysis decides against its continuation
// is committing with decidedUnitsOn, and that the analysis does not pass a repetition that the
// continuation scans again.
func TestDecidedUnits(t *testing.T) {
	defer func() { decidedUnitsOn = false }()
	const rest = "rule elems = value (\",\" ws value)*\nrule ws = [ ]*\n"
	for _, c := range []struct {
		name, src string
		off, on   ComplexityClass
	}{
		{"trailing separator", "rule main = value $$\nrule value = \"[\" ws elems? (\",\" ws)? \"]\" ws / [a-z]+ ws\n" + rest,
			ClassUnknown, ClassLinear},
		// At k = 3 this stays unknown at the bound as well; TestDecidedUnitsCost checks it where
		// only the cost conditions decide.
		{"trailing separator after a scan", scanSeparator, ClassUnknown, ClassUnknown},
		{"separator whitespace that reads a variable", "var x int = 0\nrule main = value $$\n" +
			"rule value = \"[\" ws elems? (\",\" wsx)? \"]\" ws / [a-z]+ ws\n" +
			"rule elems = value (\",\" wsx value)*\nrule ws = [ ]*\nrule wsx = {x == 1} [ ]* / [ ]*\n",
			ClassUnknown, ClassLinear},
		// The iteration sees the binding of elems, and the continuation after elems does not, so
		// the calls of wsx in them are not equal.
		{"a binding in the rule that the continuation leaves", "var x int = 0\nrule main = value $$\n" +
			"rule value = \"[\" ws elems? (\",\" wsx)? \"]\" ws / [a-z]+ ws\n" +
			"rule elems = {x := 1} value (\",\" wsx value)*\nrule ws = [ ]*\nrule wsx = {x == 1} [ ]* / [ ]*\n",
			ClassUnknown, ClassUnknown},
		// The strict analysis decides the expression of the lookahead against what follows it, but
		// lookahead units are left to the other ways of being committing.
		{"a lookahead unit", "rule main = (!([a-f] id \"=\") [a-z0-9] id \"(\" / \";\")* $$\nrule id = [a-z]+\n",
			ClassPoly, ClassPoly},
	} {
		decidedUnitsOn = false
		if got := classifySource(t, c.src).Class; got != c.off {
			t.Errorf("%s: off: class %v, want %v", c.name, got, c.off)
		}
		decidedUnitsOn = true
		if got := classifySource(t, c.src).Class; got != c.on {
			t.Errorf("%s: on: class %v, want %v", c.name, got, c.on)
		}
	}
}

// exclHooks is a setting of the test hooks of the exclusion analysis.
type exclHooks struct {
	prefixes, decided bool
	k                 int
	trivia            bool
}

// set sets the hooks and returns a function that restores the defaults.
func (h exclHooks) set() func() {
	cutPrefixesOn, decidedUnitsOn, decidedUnitsK = h.prefixes, h.decided, h.k
	triviaOn = h.trivia
	return func() {
		cutPrefixesOn, decidedUnitsOn, decidedUnitsK = false, false, 3
		triviaOn = false
	}
}

// exclBlocker returns where the strict analysis at maxK stops for the first pair of paths of the
// unit and a continuation that it does not decide, or "" when it decides the unit. The stop is the
// tag of the comparison, with the kind of the symbol for stops and expansions.
func exclBlocker(k *classifier, u *unit, strict bool, maxK int) string {
	if u.kind == unitLookahead && strict {
		return "lookahead unit"
	}
	x := newExcl(k, strict, maxK)
	last := ""
	exclTrace = func(tag string, a sym, b *sym) {
		switch tag {
		case "pstop", "noexpand":
			tag += ": " + stopKind(k, a)
		case "qstop":
			tag += ": " + stopKind(k, *b)
		}
		last = tag
	}
	defer func() { exclTrace = nil }()
	var ps [][]sym
	for _, c := range u.self {
		p, ok := x.followPaths(c)
		if !ok {
			return "path limit"
		}
		ps = append(ps, p...)
	}
	for _, cs := range k.withoutReentry(u) {
		for _, c := range cs.paths {
			qs, ok := x.followPaths(c)
			if !ok {
				return "path limit"
			}
			for _, p := range ps {
				for _, q := range qs {
					last = ""
					if ok, _, _ := x.excludes(exclState{p: p, q: q}); !ok {
						return cmp.Or(last, "?")
					}
				}
			}
		}
	}
	return ""
}

// stopKind names the kind of expression of a symbol at which the analysis stops.
func stopKind(k *classifier, s sym) string {
	switch e := s.e.(type) {
	case nil:
		return "end of continuation"
	case *grammar.Bottom:
		switch e {
		case stopFollow:
			return "follow bound"
		case stopLength:
			return "length bound"
		}
		return "bottom"
	case *grammar.Choice, *grammar.Optional, *grammar.Repeat:
		return "part that is not cheap"
	case *grammar.Ref:
		switch {
		case e.Level != "":
			return "operator level call"
		case k.an.cyclic[e.Name]:
			return "left-recursive call"
		}
		return "call"
	case *grammar.CharClass, *grammar.Any, *grammar.Literal:
		return "character"
	case *grammar.Attributed, *grammar.OperatorsRule, *grammar.Recovery:
		return "attributed, Operators Rule or #recover"
	}
	return fmt.Sprintf("%T", s.e)
}

// exclStats counts the decisions of the non-strict analysis over the choices of the reachable
// rules.
type exclStats struct {
	choices, alts int    // choices with two or more alternatives, and their alternatives but the last
	choicesAt     [6]int // choices whose alternatives are all decided within k symbols (index k)
	altsAt        [6]int // alternatives decided within k symbols
	overBudget    int    // comparisons left undecided at exclBudget
}

// exclusionStats decides each alternative of each choice of the reachable rules of g against the
// later alternatives, within up to five symbols.
func exclusionStats(g *grammar.Grammar, start string) (exclStats, error) {
	k, _, err := prepareClassifier(g, start, nil)
	if err != nil {
		return exclStats{}, err
	}
	k.computeBounded()
	k.computeFirst()
	const most = 5
	var st exclStats
	for _, r := range k.rules {
		if !k.reach[r.Name] {
			continue
		}
		WalkExpr(r.Expr, func(e grammar.Expr) {
			ch, ok := e.(*grammar.Choice)
			if !ok || len(ch.Alts) < 2 {
				return
			}
			st.choices++
			worst := 0
			for i := range ch.Alts[:len(ch.Alts)-1] {
				st.alts++
				d := k.decideAlt(ch, i, most)
				if !d.decided {
					worst = most + 1
					continue
				}
				worst = max(worst, d.k)
				for n := d.k; n <= most; n++ {
					st.altsAt[n]++
				}
			}
			for n := worst; n <= most; n++ {
				st.choicesAt[n]++
			}
		})
	}
	st.overBudget = k.exclOverBudget
	return st, nil
}

// TestExclusionReport prints, for the bundled grammars, the choices that the exclusion analysis
// decides within k symbols, the classification with each setting of its hooks, the trivia rules,
// and where the strict analysis stops for the units that stay not committing. It runs when
// ZP_EXCLUSION_REPORT is set; ZP_EXCLUSION_ONLY replaces the list of grammar files, separated by
// commas. The tables of record 069 come from its output.
func TestExclusionReport(t *testing.T) {
	if os.Getenv("ZP_EXCLUSION_REPORT") == "" {
		t.Skip("ZP_EXCLUSION_REPORT is not set")
	}
	files := []string{
		"testdata/grammars/cel/cel.zp", "testdata/grammars/csv/csv.zp", "testdata/grammars/cue/cue.zp",
		"testdata/grammars/duckdb/duckdb.zp", "testdata/grammars/golang/golang.zp",
		"testdata/grammars/python/python.zp", "testdata/grammars/typescript/typescript.zp",
		"testdata/grammars/xml/xml.zp", "testdata/grammars/yaml/yaml.zp", "examples/minilang/minilang.zp",
		"examples/outline/outline.zp", "testdata/grammars/json/json.zp",
	}
	for i := range files {
		files[i] = "../../" + files[i]
	}
	if only := os.Getenv("ZP_EXCLUSION_ONLY"); only != "" {
		files = strings.Split(only, ",")
	}
	settings := []struct {
		name string
		h    exclHooks
	}{
		{"base", exclHooks{k: 3}},
		{"cut prefixes", exclHooks{prefixes: true, k: 3}},
		{"decided k=2", exclHooks{decided: true, k: 2}},
		{"decided k=3", exclHooks{decided: true, k: 3}},
		{"decided k=4", exclHooks{decided: true, k: 4}},
		{"k=3, trivia", exclHooks{decided: true, k: 3, trivia: true}},
	}
	// The settings whose units that stay not committing are sorted by where the analysis stops.
	blockers := map[string]bool{"decided k=3": true, "k=3, trivia": true}
	total := map[string]int{}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		g, err := syntax.Parse(string(src))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		name := strings.TrimSuffix(filepath.Base(f), ".zp")
		st, err := exclusionStats(g, "")
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		fmt.Printf("DECISIONS %s choices %d %v alternatives %d %v over budget %d\n",
			name, st.choices, st.choicesAt[1:], st.alts, st.altsAt[1:], st.overBudget)
		for _, s := range settings {
			restore := s.h.set()
			k, _, err := prepareClassifier(g, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			c := k.classify()
			if k.stringLen {
				if typed, _ := prepareTypedClassifier(g, "", nil); typed != nil {
					k, c = typed, typed.classify()
				}
			}
			if s.h.trivia {
				var rules []string
				for _, r := range k.rules {
					if k.reach[r.Name] && k.isTrivia(&grammar.Ref{Name: r.Name}) {
						rules = append(rules, r.Name)
					}
				}
				fmt.Printf("TRIVIA %s %s: %s\n", name, s.name, strings.Join(rules, " "))
			}
			fmt.Printf("CLASS %s %s: %s w%d r%d, over budget %d\n", name, s.name, c.Class, len(c.Causes), len(c.Reasons), k.exclOverBudget)
			total[s.name+" w"] += len(c.Causes)
			total[s.name+" r"] += len(c.Reasons)
			if blockers[s.name] {
				for _, u := range k.units {
					if !u.nc {
						continue
					}
					why := exclBlocker(k, u, true, s.h.k)
					// The non-strict analysis tells a cost condition and the bound from an overlap.
					switch {
					case why == "lookahead unit":
					case exclBlocker(k, u, false, s.h.k) == "":
						why = "cost condition: " + why
					case exclBlocker(k, u, false, 8) == "":
						why = "bound k"
					}
					total["UNITS "+s.name+" | "+u.kind.String()]++
					total["BLOCKER "+s.name+" | "+why]++
				}
			}
			restore()
		}
	}
	for _, key := range slices.Sorted(maps.Keys(total)) {
		fmt.Printf("TOTAL %s %d\n", key, total[key])
	}
}

// fmtPath formats a path for tests: stops as STOP, the end of the parse as END, a mark of
// changed variables as BIND.
func fmtPath(p []sym) string {
	var b []string
	for _, s := range p {
		switch s.kind {
		case symStop:
			b = append(b, "STOP")
		case symEnd:
			b = append(b, "END")
		case symBind:
			b = append(b, "BIND")
		case symEnv:
			b = append(b, "ENV")
		case symUp:
			b = append(b, "UP")
		case symNot:
			b = append(b, "!"+grammar.FormatExpr(s.e))
		case symAnd:
			b = append(b, "&"+grammar.FormatExpr(s.e))
		case symSkip:
			b = append(b, "_")
		default:
			b = append(b, grammar.FormatExpr(s.e))
		}
	}
	return strings.Join(b, " ")
}

// TestContPathsEnds checks the continuation after the end of an operand of an Operators Rule,
// which goes on with an operator part or with what follows the rule, and after the end of the
// start rule, which is the end of the parse.
// TestFollowPaths checks that a continuation that has more paths through exclFollow rule ends
// than the analysis enumerates is followed through fewer ends instead, with stops where it is cut.
func TestFollowPaths(t *testing.T) {
	src := "rule main = r6 $$\nrule r0 = \"a\"\n"
	for i := 1; i <= 6; i++ {
		src += fmt.Sprintf("rule r%d = r%d \";\" / r%d \",\" / r%d \".\" / r%d \"!\"\n", i, i-1, i-1, i-1, i-1)
	}
	g, err := syntax.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(g, "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	k.collect()
	x := newExcl(k, true, 3)
	c := &cont{end: endRule, rule: "r0"}
	if _, ok := x.contPaths(c, 0); ok {
		t.Fatal("the paths through all ends fit the limit")
	}
	ps, ok := x.followPaths(c)
	if !ok {
		t.Fatal("no paths")
	}
	if len(ps) > exclPaths {
		t.Errorf("%d paths", len(ps))
	}
	stops := 0
	for _, p := range ps {
		if last := p[len(p)-1]; last.kind == symStop && last.e == stopFollow {
			stops++
		}
	}
	if stops == 0 {
		t.Errorf("no path ends at the follow bound: %d paths", len(ps))
	}
}

func TestContPathsEnds(t *testing.T) {
	const src = "rule main = e $$\n" +
		"rule e = #operators {\n  operand atom\n  level post { postfix \"+\" \"-\" }\n  level add { infix left \"*\" }\n}\n" +
		"rule atom = ident (\"+\" ident \"x\")?\nrule ident = [a-z]+\n"
	g, err := syntax.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(g, "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	k.collect()
	x := newExcl(k, true, 3)
	ps, ok := x.contPaths(&cont{end: endRule, rule: "atom"}, 0)
	if !ok {
		t.Fatal("too many paths")
	}
	got := map[string]bool{}
	for _, p := range ps {
		got[fmtPath(p)] = true
	}
	for _, want := range []string{`"+" "-" "+" "-" "+" "-" "+" "-" "+" "-" STOP`, `_ END`} {
		if !got[want] {
			t.Errorf("no path %s in %v", want, got)
		}
	}
	var star bool
	for p := range got {
		if strings.HasPrefix(p, `"*" `) {
			star = true
		}
	}
	if !star {
		t.Errorf("no path with the infix part in %v", got)
	}
}

// TestDecidedUnitsOperand checks a unit at the end of an operand that the strict analysis decides
// against an operator part: "+" ident against the postfix part "+" "-".
func TestDecidedUnitsOperand(t *testing.T) {
	defer func() { decidedUnitsOn = false }()
	src := "rule main = e $$\n" +
		"rule e = #operators {\n  operand atom\n  operand \"(\" e \")\"\n  level post { postfix \"+\" \"-\" }\n}\n" +
		"rule atom = ident (\"+\" ident \"x\")?\nrule ident = [a-z]+\n"
	decidedUnitsOn = false
	if got := classifySource(t, src).Class; got != ClassPoly {
		t.Errorf("off: class %v, want %v", got, ClassPoly)
	}
	decidedUnitsOn = true
	if got := classifySource(t, src).Class; got != ClassLinear {
		t.Errorf("on: class %v, want %v", got, ClassLinear)
	}
}

// comparePaths compares every path of rule p with every path of rule q of src, from one
// position, and returns whether all pairs part and the tag of the first pair that does not.
func comparePaths(t *testing.T, src string, strict bool, maxK int) (bool, string) {
	t.Helper()
	g, err := syntax.Parse("rule main = p q\n" + src)
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	k.classify()
	x := newExcl(k, strict, maxK)
	tag := ""
	exclTrace = func(s string, _ sym, _ *sym) { tag = s }
	defer func() { exclTrace = nil }()
	ps, ok := x.paths(k.byName["p"].Expr, 0)
	qs, ok2 := x.paths(k.byName["q"].Expr, 0)
	if !ok || !ok2 {
		t.Fatal("too many paths")
	}
	for _, p := range ps {
		for _, q := range qs {
			if ok, _, _ := x.excludes(exclState{p: p, q: q}); !ok {
				return false, tag
			}
		}
	}
	return true, ""
}

// TestStrictConditions checks each condition that the strict analysis adds to the comparison, on
// a pair that the comparison without it decides: a part that is not cheap stops it, a lookahead
// whose expression is not cheap is not passed, and an environment mark needs the same variables
// on both sides.
func TestStrictConditions(t *testing.T) {
	const rules = "var v int = 0\nrule w = {v == 1} [ ]+ / [ ]+\nrule id = idc+ semi\nrule idc = [a-z]\nrule semi = \";\" #missing\n"
	for _, c := range []struct {
		name, p, q string
		loose      bool   // decided without the strict conditions
		strict     bool   // decided with them
		tag        string // where the strict comparison stops
	}{
		{"p ends after shared symbols", `"a"`, `"a" "b"`, false, true, ""},
		{"an option that scans", `([a-z]* "x")? "c"`, `"1"`, true, false, "pstop"},
		{"a negative lookahead that scans", `!([a-z]* "x") "c"`, `"d"`, true, false, "pnot-notcheap"},
		{"a positive lookahead that scans", `&([a-z]* "x") "c"`, `"d"`, true, false, "pand-notcheap"},
		{"a later negative lookahead that scans", `"c"`, `!([a-z]* "x") "d"`, true, false, "qnot-notcheap"},
		{"a later positive lookahead that scans", `"c"`, `&([a-z]* "x") "d"`, true, false, "qand-notcheap"},
		// The memo does not answer id, which calls a rule with #missing.
		{"a shared call that scans and calls a rule", `id "c"`, `id "d"`, true, false, "same-notcheap"},
		// w reads v, so the option is cheap only where both sides have the same variables.
		{"an environment mark after a binding in p", `{v := 1} (w)? "c"`, `"d"`, true, false, "env"},
		{"an environment mark where q binds next", `(w)? "c"`, `{v := 1} "d"`, true, false, "env"},
		{"an environment mark in q after its binding", `"d"`, `{v := 1} (w)? "c"`, true, false, "env"},
		{"an environment mark with the same variables", `(w)? "c"`, `"d"`, true, true, ""},
	} {
		src := "rule p = " + c.p + "\nrule q = " + c.q + "\n" + rules
		if got, tag := comparePaths(t, src, false, 3); got != c.loose {
			t.Errorf("%s: loose: decided %v (%s), want %v", c.name, got, tag, c.loose)
		}
		got, tag := comparePaths(t, src, true, 3)
		if got != c.strict || tag != c.tag {
			t.Errorf("%s: strict: decided %v at %q, want %v at %q", c.name, got, tag, c.strict, c.tag)
		}
	}
}

// TestCutPrefixes checks cutPrefixesOn: an alternative with a #cut after items that fail only
// before a scan is a unit of those items, a prefix that fails after its scan is not committing
// as before, and a growing alternative keeps the analysis of its growth.
func TestCutPrefixes(t *testing.T) {
	for _, c := range []struct {
		name, src string
		off, on   ComplexityClass
	}{
		{"a scan after the cut", "rule main = r* $$\nrule r = \"a\" #cut [a-z]* \"x\" / \"a\" [a-z]* \"y\"\n",
			ClassPoly, ClassLinear},
		{"a scan before the cut", "rule main = (\"a\"* \"c\" #cut \"d\" / \"a\")* $$\n", ClassPoly, ClassPoly},
		// The recursion after the cut is not inside the unit: a failure there fails the choice.
		{"a recursion after the cut", "rule main = r $$\nrule r = \"(\" [a-z]* \"a\" #cut r \")\" / \"(\" [a-z]* \"b\" / \"x\"\n",
			ClassUnknown, ClassPoly},
		{"a growing alternative", "rule main = e $$\nrule e = e \"+\" [a-z]* #cut \"x\" / \"a\"\n",
			ClassLinear, ClassLinear},
	} {
		for _, on := range []bool{false, true} {
			restore := exclHooks{prefixes: on, k: 3}.set()
			got := classifySource(t, c.src).Class
			restore()
			want := c.off
			if on {
				want = c.on
			}
			if got != want {
				t.Errorf("%s: cut prefixes %v: class %v, want %v", c.name, on, got, want)
			}
		}
	}
}

// TestTriviaMemo checks triviaMemo: a whitespace rule that a recursion scans again at each level
// is linear once the trivia rules are memoized per position, and a rule that is not a trivia rule
// is not memoized by it.
func TestTriviaMemo(t *testing.T) {
	defer func() { triviaMemo = false }()
	const nest = "rule main = e ws $$\nrule e = \"(\" e? (ws \")\")?\n"
	for _, c := range []struct {
		name, ws string
		off, on  ComplexityClass
	}{
		{"a trivia rule", "rule ws = [ ]*\n", ClassPoly, ClassLinear},
		{"a repetition that must match once", "rule ws = [ ]+\n", ClassPoly, ClassPoly},
	} {
		for _, on := range []bool{false, true} {
			triviaMemo = on
			want := c.off
			if on {
				want = c.on
			}
			if got := classifySource(t, nest+c.ws).Class; got != want {
				t.Errorf("%s: memo %v: class %v, want %v", c.name, on, got, want)
			}
		}
	}
}

// TestTriviaSettles checks that the strict analysis drops a trivia call where another ended only
// when the final attempts of the rule cost a constant: a comment that scans can scan an
// unterminated comment again.
func TestTriviaSettles(t *testing.T) {
	defer func() { triviaOn = false }()
	triviaOn = true
	const rules = "rule sp = [ ]*\nrule cm = ([ ] / \"#\" [^\\n]*)*\n"
	for _, c := range []struct {
		name, p, q    string
		loose, strict bool
	}{
		{"a rule of characters", `sp sp "a"`, `sp " "`, true, true},
		{"a rule with a scan", `cm cm "a"`, `cm "#"`, true, false},
	} {
		src := "rule p = " + c.p + "\nrule q = " + c.q + "\n" + rules
		if got, tag := comparePaths(t, src, false, 3); got != c.loose {
			t.Errorf("%s: loose: decided %v (%s), want %v", c.name, got, tag, c.loose)
		}
		if got, tag := comparePaths(t, src, true, 3); got != c.strict {
			t.Errorf("%s: strict: decided %v (%s), want %v", c.name, got, tag, c.strict)
		}
	}
}

// TestExclusionBudget checks that a comparison whose expansions multiply is undecided at the
// budget, and counted, while a small instance of the same shape is decided: each rule p_i is a
// choice of n alternatives that begin with p_(i-1), so the paths of p_5 number n^5.
func TestExclusionBudget(t *testing.T) {
	grammarOf := func(n int) string {
		src := "rule main = (p5 \"q\" / \"a\" \"y\") $$\nrule p0 = \"a\"\n"
		for i := 1; i <= 5; i++ {
			var alts []string
			for j := range n {
				alts = append(alts, fmt.Sprintf("p%d \"b%c%c\"", i-1, 'a'+j/26, 'a'+j%26))
			}
			src += fmt.Sprintf("rule p%d = %s\n", i, strings.Join(alts, " / "))
		}
		return src
	}
	for _, c := range []struct {
		n                 int
		decided, exceeded bool
	}{{3, true, false}, {40, false, true}} {
		g, err := syntax.Parse(grammarOf(c.n))
		if err != nil {
			t.Fatal(err)
		}
		k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
		if err != nil {
			t.Fatal(err)
		}
		k.computeBounded()
		k.computeFirst()
		ch := ungroup(ungroup(k.byName["main"].Expr).(*grammar.Seq).Items[0]).(*grammar.Choice)
		start := time.Now()
		d := k.decideAlt(ch, 0, 5)
		if d.decided != c.decided || (k.exclOverBudget > 0) != c.exceeded {
			t.Errorf("n %d: decided %v, over budget %d", c.n, d.decided, k.exclOverBudget)
		}
		t.Logf("n %d: %v", c.n, time.Since(start))
	}
}
