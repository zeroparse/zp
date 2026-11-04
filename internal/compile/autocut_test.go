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
