package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/noderep"
)

func documentTestRules() (*Rule, *Rule) {
	item := &Rule{Id: 1, Name: "item", Memo: true, DocMemo: true, Seen: -1}
	item.Body = func(p *Parser, _ int) (*core.Node, bool) {
		return p.MatchLiteral([]rune("a"), "a", IdAny, true)
	}
	main := &Rule{Id: 0, Name: "main", Memo: true, DocMemo: true, Seen: -1}
	main.Body = func(p *Parser, _ int) (*core.Node, bool) {
		start := p.Pos
		var children []*core.Node
		for {
			v, ok := p.Call(item, 0)
			if !ok {
				break
			}
			children = append(children, v)
		}
		if !p.AtEnd() {
			return nil, false
		}
		return nodeOfData(&Data{Kind: noderep.ListKind, Start: int32(start), End: int32(p.Pos), Children: children}), true
	}
	return main, item
}

func TestPrivateDocumentPrefixReuseAndFreshResult(t *testing.T) {
	main, _ := documentTestRules()
	for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
		d := testDocumentState(main, "aa", unit)
		o := parseOptions{unit: unit, maxDepth: defaultMaxDepth}
		first, err := d.parse(o)
		if err != nil || first == nil || len(D(first).Children) != 2 {
			t.Fatalf("unit %d: first parse = %v, %v", unit, first, err)
		}
		if err := d.edit(2, 2, "a"); err != nil {
			t.Fatal(err)
		}
		second, err := d.parse(o)
		if err != nil || second == nil || len(D(second).Children) != 3 || d.stats.Reused < 1 {
			t.Fatalf("unit %d: edited parse = %v, %v; stats = %+v", unit, second, err, d.stats)
		}
		if D(first).Children[0] != D(second).Children[0] {
			t.Fatalf("unit %d: safe prefix memo was not reused", unit)
		}
		if err := d.edit(0, 1, "b"); err != nil {
			t.Fatal(err)
		}
		if _, err := d.parse(o); err == nil {
			t.Fatalf("unit %d: changed prefix unexpectedly parsed", unit)
		}
	}
}

// TestPrivateDocumentExaminedSuffixAndEOF checks that an entry whose evaluation examined the slot
// after its end is dropped by an edit of that slot, after which the slot is the end of the text
// ("Reuse Model" in spec/runtime/documents.md: the examined range covers every slot examined).
func TestPrivateDocumentExaminedSuffixAndEOF(t *testing.T) {
	item := &Rule{Id: 1, Name: "item", Memo: true, DocMemo: true, Seen: -1}
	item.Body = func(p *Parser, _ int) (*core.Node, bool) {
		start := p.Pos
		v, ok := p.MatchLiteral([]rune("a"), "a", IdAny, true)
		if !ok {
			return nil, false
		}
		_, _, ok = p.Peek() // lookahead that extends beyond the consumed text
		p.Pos = start + 1
		return v, ok
	}
	main := &Rule{Id: 0, Name: "main", Memo: true, DocMemo: true, Seen: -1}
	main.Body = func(p *Parser, _ int) (*core.Node, bool) { return p.Call(item, 0) }
	d := testDocumentState(main, "ab", core.CodePoints)
	o := parseOptions{maxDepth: defaultMaxDepth}
	if _, err := d.parse(o); err == nil { // suffix remains after the rule
		t.Fatal("expected whole-input failure")
	}
	if e := d.memo.get(item.Id, 0, 0, nil); e == nil || e.examined <= 1 {
		t.Fatalf("lookahead dependency was not recorded: %+v", e)
	}
	if err := d.edit(1, 2, ""); err != nil {
		t.Fatal(err)
	}
	if e := d.memo.get(item.Id, 0, 0, nil); e != nil {
		t.Fatal("lookahead-dependent memo survived edit")
	}
	if _, err := d.parse(o); err == nil {
		t.Fatal("EOF made the lookahead fail; parse must not reuse the old result")
	}
}

func TestPrivateDocumentEditBoundariesAndUnit(t *testing.T) {
	main, _ := documentTestRules()
	d := testDocumentState(main, "é", core.Bytes)
	if err := d.edit(1, 1, "x"); err == nil || d.text != "é" {
		t.Fatalf("interior UTF-8 edit = %v, text = %q", err, d.text)
	}
	if err := d.edit(0, 0, "\xff"); err != nil || !strings.HasPrefix(d.text, "\xff") {
		t.Fatalf("invalid byte insertion = %v, text = %q", err, d.text)
	}
	if _, err := d.parse(parseOptions{unit: core.CodePoints, maxDepth: defaultMaxDepth}); err == nil || !strings.Contains(err.Error(), "unit") {
		t.Fatalf("unit change = %v", err)
	}
	if err := d.edit(-1, 0, "x"); err == nil {
		t.Fatal("invalid range was accepted")
	}
}

func TestPrivateDocumentInterruptionDiscardsMemo(t *testing.T) {
	main, _ := documentTestRules()
	d := testDocumentState(main, "aa", core.CodePoints)
	o := parseOptions{maxDepth: defaultMaxDepth}
	if _, err := d.parse(o); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := d.parse(parseOptions{maxDepth: defaultMaxDepth, context: ctx}); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-execution interruption = %v", err)
	}
	if len(d.memo.slots) == 0 {
		t.Fatal("pre-execution interruption discarded complete memo")
	}
	if err := d.edit(2, 2, "a"); err != nil {
		t.Fatal(err)
	}
	main.Body = func(*Parser, int) (*core.Node, bool) { panic(fatal{err: core.ErrLimit}) }
	if _, err := d.parse(o); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("interruption = %v", err)
	}
	if len(d.memo.slots) != 0 {
		t.Fatalf("execution interruption retained %d memo slots", len(d.memo.slots))
	}
}

func TestDocumentReturnedRecoveryErrorOwnership(t *testing.T) {
	r := &Rule{Id: 0, Name: "main", Memo: true, DocMemo: true, Seen: -1}
	r.Body = func(p *Parser, _ int) (*core.Node, bool) {
		p.Pos = 1
		p.PushRecovered(&core.SyntaxError{Line: 1, Col: 1, Expected: []string{"original"}, Messages: []string{"original"}})
		return nodeOfData(&Data{Kind: noderep.ErrorKind, End: 1}), true
	}
	d := testDocumentState(r, "x", core.CodePoints)
	o := parseOptions{maxDepth: defaultMaxDepth}
	_, first := d.parse(o)
	e, ok := first.(core.SyntaxErrors)
	if !ok || len(e) != 1 {
		t.Fatal(first)
	}
	e[0].Messages[0] = "mutated"
	e[0].Expected[0] = "mutated"
	_, second := d.parse(o)
	next, ok := second.(core.SyntaxErrors)
	if !ok || len(next) != 1 {
		t.Fatal(second)
	}
	if next[0].Messages[0] != "original" || next[0].Expected[0] != "original" {
		t.Fatalf("caller mutation changed cached diagnostics: %+v", next[0])
	}
}

func TestDocumentDiagnosticPublicationBudget(t *testing.T) {
	calls := 0
	r := &Rule{Id: 0, Name: "main", Memo: true, DocMemo: true, Seen: -1}
	r.Body = func(p *Parser, _ int) (*core.Node, bool) {
		calls++
		p.Pos = 1
		expected := make([]string, 2048)
		for i := range expected {
			expected[i] = "original"
		}
		p.PushRecovered(&core.SyntaxError{Line: 1, Col: 1, Expected: expected, Messages: []string{"original"}})
		return nodeOfData(&Data{Kind: noderep.ErrorKind, End: 1}), true
	}
	d := testDocumentState(r, "x", core.CodePoints)
	normal := parseOptions{maxDepth: defaultMaxDepth}
	if _, err := d.parse(normal); err == nil {
		t.Fatal("missing recovered diagnostic")
	}
	if calls != 1 {
		t.Fatalf("initial evaluations: %d", calls)
	}
	limited := normal
	limited.limits = core.Limits{Compute: 64}
	if out, err := d.parse(limited); out != nil || !errors.Is(err, core.ErrLimit) {
		t.Fatalf("diagnostic budget: %v %v", out, err)
	}
	if calls != 1 {
		t.Fatal("warm parse should reuse memo before publication")
	}
	if len(d.memo.slots) != 0 {
		t.Fatal("interrupted publication retained memo")
	}
	if _, err := d.parse(normal); err == nil {
		t.Fatal("retry lost recovered diagnostic")
	}
	if calls != 2 {
		t.Fatal("retry failed to rebuild cleared memo")
	}
}

type diagnosticCancelContext struct {
	context.Context
	done  chan struct{}
	polls int
}

func (c *diagnosticCancelContext) Done() <-chan struct{} { return c.done }
func (c *diagnosticCancelContext) Err() error {
	c.polls++
	if c.polls == 4 {
		close(c.done)
	}
	if c.polls >= 4 {
		return context.Canceled
	}
	return nil
}

func TestDocumentDiagnosticPublicationCancellation(t *testing.T) {
	r := &Rule{Id: 0, Name: "main", Memo: true, DocMemo: true, Seen: -1}
	r.Body = func(p *Parser, _ int) (*core.Node, bool) {
		p.Pos = 1
		p.PushRecovered(&core.SyntaxError{Expected: make([]string, 2048)})
		return nodeOfData(&Data{Kind: noderep.ErrorKind, End: 1}), true
	}
	d := testDocumentState(r, "x", core.CodePoints)
	normal := parseOptions{maxDepth: defaultMaxDepth}
	if _, err := d.parse(normal); err == nil {
		t.Fatal("missing recovered diagnostic")
	}
	ctx := &diagnosticCancelContext{Context: context.Background(), done: make(chan struct{})}
	o := normal
	o.context = ctx
	converted := false
	out, err := DocumentParseConverted(d, o, func(n *core.Node, _ *BudgetControl, _ error) *core.Node {
		converted = true
		return n
	})
	if out != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("diagnostic cancellation: %v %v", out, err)
	}
	if converted {
		t.Fatal("diagnostic copying did not poll cancellation before conversion")
	}
	if len(d.memo.slots) != 0 {
		t.Fatal("cancelled publication retained memo")
	}
}

// defaultMaxDepth is the rule-call limit of the test grammars.
const defaultMaxDepth = 100_000

// testDocumentState gives root a grammar with the fixed expectations and
// returns a document state for it.
func testDocumentState(root *Rule, text string, unit core.PositionUnit) *documentState {
	g := NewGrammar(defaultMaxDepth)
	g.Descs = []string{"any", "begin input", "end input", "begin line", "end line"}
	root.g = g
	return newDocumentState(root, text, unit)
}

// TestDocumentLimitInsideCallThenParseEqualsFresh interrupts a Document parse by its work budget at
// every point of an edited parse, inside the rule calls included, and checks that the next parse of
// the Document gives what a parse of a fresh Document gives.
func TestDocumentLimitInsideCallThenParseEqualsFresh(t *testing.T) {
	text := "aaaaaaaa"
	edited := text[:4] + "a" + text[4:]
	o := parseOptions{maxDepth: defaultMaxDepth}
	fresh := func() *core.Node {
		main, _ := documentTestRules()
		n, err := testDocumentState(main, edited, core.CodePoints).parse(o)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	want := documentNodeString(fresh())
	interrupted := 0
	for budget := uint64(1); ; budget++ {
		main, _ := documentTestRules()
		d := testDocumentState(main, text, core.CodePoints)
		if _, err := d.parse(o); err != nil {
			t.Fatal(err)
		}
		if err := d.edit(4, 4, "a"); err != nil {
			t.Fatal(err)
		}
		limited := o
		limited.limits = core.Limits{Compute: budget}
		if _, err := d.parse(limited); err == nil {
			break
		} else if !errors.Is(err, core.ErrLimit) {
			t.Fatalf("budget %d: %v", budget, err)
		}
		interrupted++
		got, err := d.parse(o)
		if err != nil {
			t.Fatalf("budget %d: the parse after the interruption: %v", budget, err)
		}
		if s := documentNodeString(got); s != want {
			t.Fatalf("budget %d: the parse after the interruption gives\n%s\na fresh Document\n%s", budget, s, want)
		}
	}
	if interrupted < len(edited) {
		t.Fatalf("%d budgets interrupted the parse, want at least one per rule call", interrupted)
	}
}

// documentNodeString lists the kinds and spans of a tree.
func documentNodeString(n *core.Node) string {
	if n == nil {
		return "nil"
	}
	s := fmt.Sprintf("(%v %d-%d", n.Kind(), D(n).Start, D(n).End)
	for _, c := range D(n).Children {
		s += " " + documentNodeString(c)
	}
	return s + ")"
}

// TestCallDocumentRestoresOnPanic checks that a Document call restores the enclosing call's
// frontier when its rule panics, memoized and not: the enclosing call keeps the farther frontier,
// which here is its own.
func TestCallDocumentRestoresOnPanic(t *testing.T) {
	for _, memo := range []bool{true, false} {
		r := &Rule{Id: 0, Name: "main", Memo: memo, DocMemo: memo, Seen: -1}
		r.Body = func(*Parser, int) (*core.Node, bool) { panic(fatal{err: core.ErrLimit}) }
		d := testDocumentState(r, "abc", core.CodePoints)
		p := &Parser{unit: d.unit, MaxDepth: defaultMaxDepth}
		p.setGrammar(d.root.g)
		p.setSource(d.text)
		p.Document, p.memo = d, d.memo
		p.Pos, p.docHigh = 1, 3
		func() {
			defer func() {
				if x := recover(); x == nil {
					t.Fatalf("memo=%v: the call did not panic", memo)
				}
			}()
			p.callDocument(r, 0)
		}()
		if p.docHigh != 3 {
			t.Errorf("memo=%v: the panic left the frontier %d, want 3", memo, p.docHigh)
		}
	}
}

// TestDocumentMoveMarksDoNotCarryAcrossParses checks that a move after an edit does not take the
// mark that a refused move of an earlier parse left on a node for a visit of its own: the refused
// value held a host value that cannot be moved, and a second entry shares that node. The numbers of
// the moves belong to the Document, so each parse numbers its moves after those of the parses before.
func TestDocumentMoveMarksDoNotCarryAcrossParses(t *testing.T) {
	shared := nodeOfData(&Data{Kind: noderep.MatchKind, Start: 1, End: 2, Box: []int{1}})
	first := nodeOfData(&Data{Kind: noderep.ListKind, Start: 1, End: 2, Children: []*core.Node{shared}})
	second := nodeOfData(&Data{Kind: noderep.ListKind, Start: 1, End: 2, Children: []*core.Node{shared}})
	d := &documentState{}
	for i, n := range []*core.Node{first, second} {
		// Each parse starts its moves anew in a Parser of its own.
		p := &Parser{Document: d}
		e := &memoEntry{node: n}
		p.memo.shifts = map[*memoEntry]int{e: 1}
		if p.documentMove(e) {
			t.Fatalf("parse %d: moved a value that holds a host value", i)
		}
		if e.node != n || D(n).Children[0] != shared || D(shared).Start != 1 {
			t.Fatalf("parse %d: a refused move changed the value", i)
		}
	}
}

// TestDocumentMoveMarksRestartWithAnEmptyMemo checks that a Document whose move numbers near the end
// of their range parses without its memo, and numbers its moves from 1 again.
func TestDocumentMoveMarksRestartWithAnEmptyMemo(t *testing.T) {
	main, _ := documentTestRules()
	d := testDocumentState(main, "aa", core.CodePoints)
	o := parseOptions{unit: core.CodePoints, maxDepth: defaultMaxDepth}
	if _, err := d.parse(o); err != nil {
		t.Fatal(err)
	}
	d.moves = moveMarkLimit + 1
	if err := d.edit(2, 2, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.parse(o); err != nil || d.stats.Reused != 0 || d.moves != 0 {
		t.Fatalf("parse after the limit: err %v, %d reused, move number %d", err, d.stats.Reused, d.moves)
	}
	if err := d.edit(3, 3, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.parse(o); err != nil || d.stats.Reused == 0 {
		t.Fatalf("the memo of the restarted Document is not reused: err %v, stats %+v", err, d.stats)
	}
}

// TestDocumentMoveChargesTheCheckAndTheCopy checks the units that a move charges: for each distinct
// node of the value, one and one for each child and field, in the check and again in the copy, and
// the check alone in a refusal.
func TestDocumentMoveChargesTheCheckAndTheCopy(t *testing.T) {
	leaf := func(box any) *core.Node {
		return nodeOfData(&Data{Kind: noderep.MatchKind, Start: 1, End: 2, Box: box})
	}
	// The size of a root of three children, two distinct leaves of which one is held twice, and a field:
	// 5 for the root and 1 for each leaf.
	build := func(box any) *core.Node {
		a, b := leaf(nil), leaf(box)
		return nodeOfData(&Data{Kind: noderep.ListKind, Start: 1, End: 2, Children: []*core.Node{a, b, a}, Fields: noderep.Fields{{Name: "x", Value: "v"}}})
	}
	const size = 5 + 2
	for _, c := range []struct {
		name string
		box  any
		want uint64
	}{
		{"moved", nil, 2 * size},
		// The check charges the root, then the first leaf, then refuses at the second.
		{"refused", []int{1}, 5 + 1 + 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := &Parser{Document: &documentState{}, Control: newBudgetControl(nil, core.Limits{Compute: core.Unlimited})}
			e := &memoEntry{node: build(c.box)}
			p.memo.shifts = map[*memoEntry]int{e: 1}
			before := p.Control.budgetUsed()
			moved := p.documentMove(e)
			if moved != (c.box == nil) {
				t.Fatalf("moved = %v", moved)
			}
			if used := p.Control.budgetUsed() - before; used != c.want {
				t.Fatalf("charged %d units, want %d", used, c.want)
			}
		})
	}
}

// TestDocumentMoveKeepsSharedNodesShared checks that a reused result that holds a node twice, as a
// child and as a field, holds one moved copy of it, not two, and that the result of the parse before
// the edit keeps the positions it had.
func TestDocumentMoveKeepsSharedNodesShared(t *testing.T) {
	item := &Rule{Id: 1, Name: "item", Memo: true, DocMemo: true, Seen: -1}
	item.Body = func(p *Parser, _ int) (*core.Node, bool) {
		return p.MatchLiteral([]rune("a"), "a", IdAny, true)
	}
	pair := &Rule{Id: 2, Name: "pair", Memo: true, DocMemo: true, Seen: -1}
	pair.Body = func(p *Parser, _ int) (*core.Node, bool) {
		start := p.Pos
		v, ok := p.Call(item, 0)
		if !ok {
			return nil, false
		}
		return nodeOfData(&Data{Kind: noderep.ListKind, Start: int32(start), End: int32(p.Pos), Children: []*core.Node{v}, Fields: noderep.Fields{{Name: "x", Value: v}}}), true
	}
	main := &Rule{Id: 0, Name: "main", Memo: true, DocMemo: true, Seen: -1}
	main.Body = func(p *Parser, _ int) (*core.Node, bool) {
		start := p.Pos
		for {
			if _, ok := p.MatchLiteral([]rune("b"), "b", IdAny, true); !ok {
				break
			}
		}
		v, ok := p.Call(pair, 0)
		if !ok || !p.AtEnd() {
			return nil, false
		}
		return nodeOfData(&Data{Kind: noderep.ListKind, Start: int32(start), End: int32(p.Pos), Children: []*core.Node{v}}), true
	}
	d := testDocumentState(main, "ba", core.CodePoints)
	o := parseOptions{unit: core.CodePoints, maxDepth: defaultMaxDepth}
	first, err := d.parse(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.edit(0, 0, "b"); err != nil {
		t.Fatal(err)
	}
	second, err := d.parse(o)
	if err != nil || d.stats.Reused == 0 {
		t.Fatalf("the pair is not reused: err %v, stats %+v", err, d.stats)
	}
	pairOf := func(root *core.Node) (*core.Node, *core.Node, *core.Node) {
		pr := D(root).Children[0]
		return pr, D(pr).Children[0], D(pr).Fields[0].Value.(*core.Node)
	}
	pr, child, field := pairOf(second)
	if child != field {
		t.Fatal("the moved pair holds two copies of the node that it held once")
	}
	if D(pr).Start != 2 || D(child).Start != 2 || D(child).End != 3 {
		t.Fatalf("moved pair [%d,%d), node [%d,%d); want [2,3), [2,3)", D(pr).Start, D(pr).End, D(child).Start, D(child).End)
	}
	if pr0, child0, field0 := pairOf(first); child0 != field0 || D(pr0).Start != 1 || D(child0).Start != 1 {
		t.Fatal("the result of the parse before the edit changed")
	}
}
