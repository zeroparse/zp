package build_test

import (
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
)

func TestCoreBuildShapes(t *testing.T) {
	src := `rule main = a:x b:("c" / d)* ("e" [0-9]) #atomic -"f" !"g" &(h:"h") $$ => $a
rule x = "x" "y"
rule d = "d"
`
	prog := coreProgram(t, src)
	if prog.HasMissing {
		t.Fatal("does not compile")
	}
	m, err := build.CoreFrom(prog, nil)
	if err != nil {
		t.Fatal(err)
	}
	core.Analyze(m)
	if err := core.Verify(m); err != nil {
		t.Fatal(err)
	}
	got := core.Print(m, false)
	for _, want := range []string{
		"scope=[a b h] => $0:a",
		"  seq\n    capture 0 a\n      call 1:x\n",
		"    capture 1 b\n      repeat 0 -1\n",
		"    text\n      seq\n",
		"    drop\n      lit \"f\"\n",
		"    not\n      lit \"g\"\n",
		"    and\n      capture 2 h\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Print does not contain %q:\n%s", want, got)
		}
	}
	main := m.Rules[0]
	if main.Want || main.Finish.Kind != core.FinishAction {
		t.Errorf("main: want=%v finish=%v; want a lean rule with an action", main.Want, main.Finish.Kind)
	}
	// The capture builds the value of its body, a call of x itself rather than its twin.
	capture := main.Body.(*core.Seq).Items[0].(*core.Capture)
	if call := capture.Body.(*core.Call); !call.Attrs().Want || m.Rules[call.Rule].Policy.NoValue {
		t.Errorf("captured call: want=%v novalue=%v", call.Attrs().Want, m.Rules[call.Rule].Policy.NoValue)
	}
	if x := m.Rules[1]; !x.Want || x.Finish.Kind != core.FinishScope || !x.Finish.Shape.Tuple {
		t.Errorf("x: want=%v finish=%v shape=%+v", x.Want, x.Finish.Kind, x.Finish.Shape)
	}
}

// coreProgram compiles src to its static program, failing the test if it does not compile.
func coreProgram(t *testing.T, src string) *compile.Program {
	t.Helper()
	return compiletest.Program(t, src, compile.Options{})
}

// typeString returns the checker's spelling of a Core type handle.
func typeString(x core.Type) string {
	if x == nil {
		return "<nil>"
	}
	return x.(compile.Ty).String()
}

// TestCoreBuildTypes checks that the module carries the checked types: named types, rule values,
// capture slots of rules and of repetition elements, and every term.
func TestCoreBuildTypes(t *testing.T) {
	prog := coreProgram(t, `type P = { N int; S string }
var k int = 2
rule main P = p:pair xs:(-"," q:pair)* => P{N: $p.N + len($xs) * k, S: $p.S}
rule pair P = n:[0-9]+ s:[a-z] => P{N: len($n), S: text($s)}
`)
	m, err := build.CoreFrom(prog, nil)
	if err != nil {
		t.Fatal(err)
	}
	core.Analyze(m)
	if err := core.Verify(m); err != nil {
		t.Fatal(err)
	}
	if !m.Typed || typeString(m.Types["P"]) != "P" {
		t.Fatalf("typed %v, types %v", m.Typed, m.Types)
	}
	if len(m.Vars) != 1 || m.Vars[0].Name != "k" || typeString(m.Vars[0].Type) != "int" || m.Vars[0].Init != 2 {
		t.Errorf("vars %+v", m.Vars)
	}
	main, pair := m.Rules[0], m.Rules[1]
	if typeString(main.Type) != "P" || typeString(pair.Type) != "P" {
		t.Errorf("rule types %s %s", typeString(main.Type), typeString(pair.Type))
	}
	slotTypes := func(s []core.Slot) string {
		var out []string
		for _, x := range s {
			out = append(out, x.Name+" "+typeString(x.Type))
		}
		return strings.Join(out, ", ")
	}
	// A capture of a call has the type of the rule's node.
	if got := slotTypes(main.Scope); got != `p pair, xs node[[]node[(pair,), {"q": 0}]]` {
		t.Errorf("main scope %s", got)
	}
	if got := slotTypes(pair.Scope); got != "n node[[]node[string]], s node[string]" {
		t.Errorf("pair scope %s", got)
	}
	var element []core.Slot
	core.Walk(main.Body, func(n core.Node) {
		if r, ok := n.(*core.Repeat); ok && r.Scope != nil {
			element = r.Scope
		}
	})
	if got := slotTypes(element); got != "q pair" {
		t.Errorf("element scope %s", got)
	}
	var terms []string
	core.WalkTerm(main.Finish.Action, func(x core.Term) {
		terms = append(terms, core.TermString(x)+" : "+typeString(core.TypeOf(x)))
	})
	if len(terms) == 0 || !strings.HasSuffix(terms[0], " : P") {
		t.Errorf("action terms %v", terms)
	}
	for _, s := range terms {
		if strings.HasSuffix(s, " : <nil>") {
			t.Errorf("untyped term %s", s)
		}
	}
	if !slices.Contains(terms, "var:k : int") && !slices.ContainsFunc(terms, func(s string) bool { return strings.HasSuffix(s, "k : int") }) {
		t.Errorf("variable term not typed int: %v", terms)
	}
}

// TestCoreBuildStreamAndMissing checks that #stream at the top of a rule body builds a Stream in
// every rule, also one that starts no parse, where it delivers nothing (no Commit effect), and that
// #missing builds a Missing of its literal.
func TestCoreBuildStreamAndMissing(t *testing.T) {
	prog := coreProgram(t, `@start
rule main = x:item* #stream $$
rule item = "a" ";" #missing / inner
rule inner = "b"* #stream
`)
	m, err := build.CoreFrom(prog, nil)
	if err != nil {
		t.Fatal(err)
	}
	core.Analyze(m)
	if err := core.Verify(m); err != nil {
		t.Fatal(err)
	}
	got := core.Print(m, false)
	for _, want := range []string{
		"    capture 0 x\n      stream\n        repeat 0 -1\n",
		"      seq\n        lit \"a\"\n        missing\n          lit \";\"\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Print does not contain %q:\n%s", want, got)
		}
	}
	if _, ok := m.Rules[2].Body.(*core.Stream); !ok {
		t.Errorf("inner: %T, want the Stream of a rule that starts no parse", m.Rules[2].Body)
	}
	if m.Rules[0].Body.Attrs().Eff.Set&core.EffCommit == 0 || m.Rules[2].Body.Attrs().Eff.Set&core.EffCommit != 0 {
		t.Errorf("commit effects: main %v, inner %v", m.Rules[0].Body.Attrs().Eff, m.Rules[2].Body.Attrs().Eff)
	}
}

// TestCoreBuildStreamUnderNestedCaptures checks that a #stream repetition that a top-level item
// captures through nested captures is built as a Stream under those captures, and verifies, in a
// start rule and in a rule that is not one.
func TestCoreBuildStreamUnderNestedCaptures(t *testing.T) {
	prog := coreProgram(t, `@start
rule main = x:(y:"a"* #stream) inner $$
rule inner = u:(v:"b"* #stream)
`)
	m, err := build.CoreFrom(prog, nil)
	if err != nil {
		t.Fatal(err)
	}
	core.Analyze(m)
	if err := core.Verify(m); err != nil {
		t.Fatal(err)
	}
	got := core.Print(m, false)
	for _, want := range []string{
		"    capture 0 x\n      capture 1 y\n        stream\n          repeat 0 -1\n",
		"  capture 0 u\n    capture 1 v\n      stream\n        repeat 0 -1\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Print does not contain %q:\n%s", want, got)
		}
	}
}

// TestFollowRepeatsThroughStream checks that the Scan that takes the place of the #stream
// repetition of a rule that is not a start rule gets the origin of that repetition.
func TestFollowRepeatsThroughStream(t *testing.T) {
	prog := coreProgram(t, `@start
rule main = inner $$
rule inner = [a-z]* #stream
`)
	origin := map[core.Node]grammar.Expr{}
	m, err := build.CoreFrom(prog, origin)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range m.Rules {
		r.Want = false
		core.Walk(r.Body, func(n core.Node) { n.Attrs().Want = false })
	}
	core.Analyze(m)
	repeats := build.FollowRepeats(m, origin)
	core.Scans(m)
	for _, f := range repeats {
		f()
	}
	s, ok := m.Rules[1].Body.(*core.Scan)
	if !ok {
		t.Fatalf("inner after scans: %T, want *core.Scan", m.Rules[1].Body)
	}
	if _, ok := origin[s].(*grammar.Repeat); !ok {
		t.Errorf("origin of the scan: %T, want *grammar.Repeat", origin[s])
	}
}

// TestCoreBuildCallLevels checks the minimum binding level of calls of an Operators Rule, and that
// a level the rule lacks, or a level of a rule that has none, is an error.
func TestCoreBuildCallLevels(t *testing.T) {
	prog := coreProgram(t, `rule main = level(e, product) "+" e
rule e = #operators {
    operand [0-9]
    level sum { infix left "+" }
    level product { infix left "*" }
}
`)
	m, err := build.CoreFrom(prog, nil)
	if err != nil {
		t.Fatal(err)
	}
	var levels []int
	core.Walk(m.Rules[0].Body, func(n core.Node) {
		if c, ok := n.(*core.Call); ok {
			levels = append(levels, c.Level)
		}
	})
	if !slices.Equal(levels, []int{1, 0}) {
		t.Errorf("levels %v, want [1 0]", levels)
	}
	e := prog.CoreInput().Rules[1]
	if _, err := build.CallLevel(&grammar.Ref{Name: "e", Level: "power"}, e); err == nil {
		t.Error("unknown level: no error")
	}
	if _, err := build.CallLevel(&grammar.Ref{Name: "main", Level: "sum"}, prog.CoreInput().Rules[0]); err == nil {
		t.Error("level of a rule without levels: no error")
	}
}
