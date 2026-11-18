package corepass

import (
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/core"
)

// passes runs the passes on m in turn, each followed by Analyze and Verify.
func passes(t *testing.T, m *core.Module, ps ...func(*core.Module)) {
	t.Helper()
	analyzed(t, m)
	for i, p := range ps {
		p(m)
		core.Analyze(m)
		if err := core.Verify(m); err != nil {
			t.Fatalf("after pass %d: %v", i, err)
		}
	}
}

// regions returns the terminal regions of the first rule of m, after Analyze.
func regions(t *testing.T, m *core.Module) map[core.Node]bool {
	t.Helper()
	analyzed(t, m)
	return TerminalRegions(m.Rules[0])
}

func valueFree(m *core.Module) *core.Module {
	for _, r := range m.Rules {
		r.Want = false
	}
	return m
}

// TestPassesGuardedClasses checks the guards and the GuardedClass marks: the alternative that begins
// with a class is marked, whatever the target. The class runs of these sequences are made by the Flow
// pass class-runs (internal/middleend/flowpass TestClassRunsGuarded).
func TestPassesGuardedClasses(t *testing.T) {
	// a = [a-z] [0-9] [x-y] "q" / "r" [a-z] [0-9]   (value-free)
	alt0 := seq(class('a', 'z'), class('0', '9'), class('x', 'y'), lit("q"))
	alt1 := seq(lit("r"), class('a', 'z'), class('0', '9'))
	m := valueFree(module(rule(0, "a", choice(alt0, alt1))))
	passes(t, m, Guards, Scans)
	got := core.PrintNode(m, m.Rules[0].Body, false)
	want := `choice
  alt guard=[a-z] class
    seq
      class [a-z]
      class [0-9]
      class [x-y]
      lit "q"
  alt guard='r'
    seq
      lit "r"
      class [a-z]
      class [0-9]
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPassesTerminals(t *testing.T) {
	var alts []core.Node
	for _, w := range strings.Fields("ab cd ef gh ij kl mn op") {
		alts = append(alts, seq(lit(w[:1]), lit(w[1:])))
	}
	small := choice(lit("x"), lit("y"))
	body := seq(&core.Drop{Body: seq(lit("k"), choice(alts...))}, small, &core.Any{}) // any character is no terminal form
	m := valueFree(module(rule(0, "a", body)))
	rs := regions(t, m)
	passes(t, m, Guards)
	if !rs[body.Items[0]] || len(rs) != 1 {
		t.Fatalf("regions %v, want the 17 leaves of item 0 only", rs)
	}
	if small.Alts[0].Guard == nil {
		t.Errorf("no guard outside the region")
	}
	// The guards inside a region are set too: the Flow pass that matches the region as one program
	// replaces them with it, and a target that does not keeps them.
	guarded := false
	core.Walk(body.Items[0], func(n core.Node) {
		if c, ok := n.(*core.Choice); ok {
			for _, a := range c.Alts {
				guarded = guarded || a.Guard != nil
			}
		}
	})
	if !guarded {
		t.Errorf("no guard inside the region")
	}
	if f := body.Attrs().First; f == nil || firstString(m, f) != "'k'" {
		t.Errorf("first: %+v", f)
	}
}

// TestPassesTerminalWant checks that TerminalRegions leaves expressions whose value is wanted.
func TestPassesTerminalWant(t *testing.T) {
	var alts []core.Node
	for _, w := range strings.Fields("ab cd ef gh ij kl mn op") {
		alts = append(alts, seq(lit(w[:1]), lit(w[1:])))
	}
	body := choice(alts...)
	body.Attrs().Want = true
	m := module(rule(0, "a", body))
	if rs := regions(t, m); len(rs) != 0 {
		t.Errorf("wanted choice: regions %v, want none", rs)
	}
}

// TestPassesTerminalContext checks that a lookahead or a discarded expression in a sequence that
// builds a value is no terminal region, though its value-free body is one.
func TestPassesTerminalContext(t *testing.T) {
	keywords := func() core.Node {
		var alts []core.Node
		for _, w := range strings.Fields("ab cd ef gh ij kl mn op") {
			alts = append(alts, seq(lit(w[:1]), lit(w[1:])))
		}
		return choice(alts...)
	}
	not := &core.Look{Body: keywords(), Neg: true}
	drop := &core.Drop{Body: keywords()}
	x := lit("x")
	x.Attrs().Want = true
	body := seq(not, drop, x)
	body.Attrs().Want = true
	m := module(rule(0, "a", body))
	m.Rules[0].Want = true
	rs := regions(t, m)
	if rs[not] || rs[drop] {
		t.Fatalf("regions %v: the lookahead or the discarded expression is one", rs)
	}
	if !rs[not.Body] {
		t.Errorf("lookahead body is no region")
	}
	if !rs[drop.Body] {
		t.Errorf("discarded body is no region")
	}
	// In a value-free sequence, the lookahead itself is one.
	not = &core.Look{Body: keywords(), Neg: true}
	body = seq(not, &core.Any{}) // any character is no terminal form
	m = valueFree(module(rule(0, "a", body)))
	if rs := regions(t, m); !rs[body.Items[0]] {
		t.Errorf("value-free lookahead is no region: %v", rs)
	}
}

// TestPassesTerminalInStream checks that TerminalRegions reaches the element of a #stream repetition,
// so that a discarded expression of the element is a region as it is in any repetition.
func TestPassesTerminalInStream(t *testing.T) {
	var alts []core.Node
	for _, w := range strings.Fields("ab cd ef gh ij kl mn op") {
		alts = append(alts, seq(lit(w[:1]), lit(w[1:])))
	}
	drop := &core.Drop{Body: choice(alts...)}
	x := lit("x")
	x.Attrs().Want = true
	body := seq(drop, x)
	body.Attrs().Want = true
	rep := &core.Repeat{Max: -1, Body: body}
	rep.Attrs().Want = true
	st := &core.Stream{Repeat: rep}
	st.Attrs().Want = true
	m := module(rule(0, "a", st))
	m.Rules[0].Want = true
	m.Starts = []int{0}
	if rs := regions(t, m); !rs[drop.Body] {
		t.Errorf("discarded body in a stream element is no region: %v", rs)
	}
}

// TestPassesOperatorLines checks that the passes reach the lines of an Operators Rule.
func TestPassesOperatorLines(t *testing.T) {
	operand := choice(class('0', '9'), lit("("))
	op := &core.Operator{ID: 0, Kind: "infix", Level: 1, Line: &core.OpLine{Body: &core.Repeat{Max: -1, Body: class('a', 'z')}}}
	m := valueFree(module(rule(0, "e", &core.OperatorsRule{Operands: []*core.OpLine{{Body: operand}}, Ops: []*core.Operator{op}})))
	passes(t, m, Guards, Scans)
	if operand.Alts[0].Guard == nil || !operand.Alts[0].GuardedClass || operand.Alts[1].Guard == nil {
		t.Errorf("operand guards: %+v %+v", operand.Alts[0], operand.Alts[1])
	}
	if _, ok := op.Line.Body.(*core.Scan); !ok {
		t.Errorf("operator line: %T, want a scan", op.Line.Body)
	}
}

// TestPassesScanPreconditions checks that Scans leaves repetitions whose elements have values,
// scopes or projections.
func TestPassesScanPreconditions(t *testing.T) {
	wanted := &core.Repeat{Max: -1, Body: class('a', 'z')}
	wanted.Attrs().Want = true
	scoped := &core.Repeat{Max: -1, Scope: []core.Slot{}, Body: class('a', 'z')}
	projected := &core.Repeat{Max: -1, Project: "x", Body: class('a', 'z')}
	plain := &core.Repeat{Min: 1, Max: 3, Body: &core.Any{}}
	m := module(rule(0, "a", seq(wanted, scoped, projected, plain)))
	m.Rules[0].Want = false
	passes(t, m, Scans)
	items := m.Rules[0].Body.(*core.Seq).Items
	for i, want := range []string{"*core.Repeat", "*core.Repeat", "*core.Repeat", "*core.Scan"} {
		if got := fmt.Sprintf("%T", items[i]); got != want {
			t.Errorf("item %d: %s, want %s", i, got, want)
		}
	}
	if s := items[3].(*core.Scan); s.Class != nil || s.Min != 1 || s.Max != 3 {
		t.Errorf("scan of any character: %+v", s)
	}
}

// TestPassesGuardFoldedLiteral checks the guard of a case-insensitive literal: an ASCII letter
// stands for both its cases, and anything else for itself.
func TestPassesGuardFoldedLiteral(t *testing.T) {
	fold := func(s string) *core.Lit { l := lit(s); l.Fold = true; return l }
	// a = "Select"i "x" / "from"i / "1"i / "é"i / "r"
	m := valueFree(module(rule(0, "a", choice(seq(fold("Select"), lit("x")), fold("from"), fold("1"), fold("é"), lit("r")))))
	passes(t, m, Guards)
	got := core.PrintNode(m, m.Rules[0].Body, false)
	want := `choice
  alt guard='s'i
    seq
      lit "Select"i
      lit "x"
  alt guard='f'i
    lit "from"i
  alt guard='1'
    lit "1"i
  alt guard='é'
    lit "é"i
  alt guard='r'
    lit "r"
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestPassesScanThroughStream checks that a #stream repetition of a rule that is not a start rule,
// which delivers no elements, becomes the Scan that its repetition alone becomes, and that the
// #stream repetition of a start rule keeps its repetition.
func TestPassesScanThroughStream(t *testing.T) {
	// main = "x"; r = [a-z]* #stream; s = [a-z]*   (value-free; main, or r, is the start rule)
	mod := func(start int) *core.Module {
		r := &core.Stream{Repeat: &core.Repeat{Max: -1, Body: class('a', 'z')}}
		s := &core.Repeat{Max: -1, Body: class('a', 'z')}
		m := valueFree(module(rule(0, "main", lit("x")), rule(1, "r", r), rule(2, "s", s)))
		m.Starts = []int{start}
		passes(t, m, Scans)
		return m
	}
	m := mod(0)
	got, want := core.PrintNode(m, m.Rules[1].Body, false), core.PrintNode(m, m.Rules[2].Body, false)
	if got != want {
		t.Errorf("non-start #stream rule after scans:\n%s\nwant, as the repetition alone:\n%s", got, want)
	}
	if _, ok := m.Rules[1].Body.(*core.Scan); !ok {
		t.Errorf("non-start #stream rule after scans: %T, want *core.Scan", m.Rules[1].Body)
	}
	m = mod(1)
	st, ok := m.Rules[1].Body.(*core.Stream)
	if !ok {
		t.Fatalf("start #stream rule after scans: %T, want *core.Stream", m.Rules[1].Body)
	}
	if st.Repeat == nil || st.Repeat.Body == nil {
		t.Errorf("start #stream rule lost its repetition")
	}
}
