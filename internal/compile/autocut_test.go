package compile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/internal/syntax"
)

// TestDecideAlt checks the exclusion analysis on the first choice of rule main.
func TestDecideAlt(t *testing.T) {
	const rules = "rule ident = [a-z]+\nrule ws = [ ]*\nrule x = \"x\"\nrule kw = \"if\" / \"in\"\n"
	for _, c := range []struct {
		name, main string
		decided    bool
		k, cut     int
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
		if d.decided != c.decided || d.decided && (d.k != c.k || d.cut != c.cut) {
			t.Errorf("%s: decided %v k %d cut %d, want %v k %d cut %d", c.name, d.decided, d.k, d.cut, c.decided, c.k, c.cut)
		}
	}
}

// TestDecideAltTrivia checks that the analysis passes calls of a trivia rule without counting them
// and drops a call that follows a call of the same trivia rule, and that rules of other shapes stay
// ordinary calls.
func TestDecideAltTrivia(t *testing.T) {
	defer func() { triviaOn, triviaOnly = false, nil }()
	const rules = "var v int = 0\nrule ident = [a-z]+\nrule x = \"x\"\nrule ws = [ ]*\nrule sp = \" \"\n" +
		"rule ws2 = (sp / \"\\t\")*\nrule wsp = [ ]+\nrule wsv = ({v == 0} \" \")*\nrule wsc = (c:\" \")*\n"
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
		{"a repetition that reads a variable", `ident wsv wsv "(" x / ident wsv wsv "." x`, nil, false, false},
		{"a repetition that captures", `ident wsc wsc "(" x / ident wsc wsc "." x`, nil, false, false},
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

// TestTriviaCheap checks that a call of a trivia rule that the memo does not answer is cheap only
// with the hook: the second call at a position costs a constant once the rule is memoized there.
func TestTriviaCheap(t *testing.T) {
	defer func() { triviaOn = false }()
	g, err := syntax.Parse("rule main = ws2 \"x\"\nrule ws2 = (sp)*\nrule sp = \" \"\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, on := range []bool{false, true} {
		triviaOn = on
		k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
		if err != nil {
			t.Fatal(err)
		}
		k.classify()
		if got := newExcl(k, true, 3).cheap(&grammar.Ref{Name: "ws2"}); got != on {
			t.Errorf("trivia %v: cheap %v", on, got)
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
		"ws": "", "ws2": "", "wsp": "body is not X*", "wsv": "reads a variable (wsv); predicate (wsv)",
		"wsc": "capture (wsc)", "wsa": "action (wsa)",
	} {
		if got := k.triviaWhy(rule); got != want {
			t.Errorf("%s: %q, want %q", rule, got, want)
		}
	}
	if got := k.triviaWhy("wsr"); !strings.HasPrefix(got, "reads a variable") {
		t.Errorf("wsr: %q, want a variable read through a call", got)
	}
}

// TestTriviaShapeExtended checks the shapes that triviaExtended adds: an atomic X* and the
// unrolled form X* (Y X*)*, but not a form whose last repetition differs from the first.
func TestTriviaShapeExtended(t *testing.T) {
	defer func() { triviaExtended = false }()
	g, err := syntax.Parse("rule main = wsu wsa wsb\nrule wsu = [ ]* (\"#\" [^\\n]* [ ]*)*\n" +
		"rule wsa = \" \"* #atomic\nrule wsb = [ ]* (\"#\" [a]*)*\n")
	if err != nil {
		t.Fatal(err)
	}
	k, _, err := prepareClassifier(starttest.MarkAll(g), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, ext := range []bool{false, true} {
		triviaExtended = ext
		for rule, ok := range map[string]bool{"wsu": ext, "wsa": ext, "wsb": false} {
			if got := k.triviaWhy(rule) == ""; got != ok {
				t.Errorf("extended %v: %s: trivia %v, want %v", ext, rule, got, ok)
			}
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
		k, cut     int
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
		if d.decided != c.decided || d.decided && (d.k != c.k || d.cut != c.cut) {
			t.Errorf("%s: decided %v k %d cut %d, want %v k %d cut %d", c.name, d.decided, d.k, d.cut, c.decided, c.k, c.cut)
		}
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
		{"trailing separator after a scan", "rule main = value $$\nrule value = \"[\" ws elems? (\",\" [a-z]* ws)? \"]\" ws / [a-z]+ ws\n" +
			"rule elems = value (\",\" [a-z]* ws value)*\nrule ws = [ ]*\n", ClassUnknown, ClassUnknown},
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

// TestAutoCutReport prints the decisions of the analysis and the classification of the bundled
// grammars with automatic cuts. It runs when ZP_AUTOCUT_REPORT is set; ZP_AUTOCUT_OUT names a
// directory for the grammars with the cuts, and ZP_AUTOCUT_EXTRA lists further grammar files,
// separated by commas; ZP_AUTOCUT_ONLY replaces the bundled list.
func TestAutoCutReport(t *testing.T) {
	if os.Getenv("ZP_AUTOCUT_REPORT") == "" {
		t.Skip("ZP_AUTOCUT_REPORT is not set")
	}
	defer func() { cutPrefixesOn, decidedUnitsOn = false, false }()
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
	if only := os.Getenv("ZP_AUTOCUT_ONLY"); only != "" {
		files = nil
		for _, f := range strings.Split(only, ",") {
			files = append(files, f)
		}
	}
	if extra := os.Getenv("ZP_AUTOCUT_EXTRA"); extra != "" {
		files = append(files, strings.Split(extra, ",")...)
	}
	out := os.Getenv("ZP_AUTOCUT_OUT")
	count := func(c *Complexity) string { return fmt.Sprintf("%s w%d r%d", c.Class, len(c.Causes), len(c.Reasons)) }
	fmt.Println("grammar | choices k<=1..5 | alts k<=1..5 committed cuts | base | cut-prefix units | auto-cut | decided units k=2,3,4 | auto-cut+decided k=3")
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		g, err := syntax.Parse(string(src))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		classify := func(g *grammar.Grammar, prefixes, decided bool, n int) *Complexity {
			cutPrefixesOn, decidedUnitsOn, decidedUnitsK = prefixes, decided, n
			defer func() { cutPrefixesOn, decidedUnitsOn, decidedUnitsK = false, false, 3 }()
			c, err := ClassifyComplexity(g, "", nil)
			if err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			return c
		}
		base := classify(g, false, false, 3)
		basePrefix := classify(g, true, false, 3)
		cut, st, err := AutoCut(g, "", 5)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		auto := classify(cut, true, false, 3)
		var decided []*Complexity
		for n := 2; n <= 4; n++ {
			decided = append(decided, classify(g, false, true, n))
		}
		commit := classify(cut, true, true, 3)
		name := strings.TrimSuffix(filepath.Base(f), ".zp")
		fmt.Printf("%s | %d %v | %d %v %d %d | %s | %s | %s | %s, %s, %s | %s\n", name,
			st.Choices, st.ChoicesAt[1:], st.Alts, st.AltsAt[1:], st.Committed, st.Cuts,
			count(base), count(basePrefix), count(auto), count(decided[0]), count(decided[1]), count(decided[2]), count(commit))
		if os.Getenv("ZP_AUTOCUT_VERBOSE") != "" {
			for _, c := range []struct {
				what string
				x    *Complexity
			}{{"base", base}, {"auto", auto}, {"decided3", decided[1]}, {"commit", commit}} {
				for _, r := range c.x.Causes {
					fmt.Printf("  %s w %d:%d %s\n", c.what, r.Pos.Line, r.Pos.Col, r.Message)
				}
				for _, r := range c.x.Reasons {
					fmt.Printf("  %s r %d:%d %s\n", c.what, r.Pos.Line, r.Pos.Col, r.Message)
				}
			}
		}
		if out != "" {
			if err := os.WriteFile(filepath.Join(out, name+".zp"), []byte(grammar.Format(cut)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
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
// TestFollowPaths checks that a continuation that has more paths through autoCutFollow rule ends
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
	if len(ps) > autoCutPaths {
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
