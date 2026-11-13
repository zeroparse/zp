package document

import (
	"context"
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

func changeGraphSnapshot(t *testing.T, root *parse.Node) *Snapshot {
	t.Helper()
	graph, err := freezeSnapshotGraph(&core.Parser{}, root)
	if err != nil {
		t.Fatal(err)
	}
	return &Snapshot{graph: graph}
}

func TestDocumentChangesConservativeGraphEquality(t *testing.T) {
	leaf := func() *parse.Node {
		return value.Ptr(value.Data{Kind: noderep.KindOf("Leaf", "leaf"), Start: 1, End: 2, Terminal: true, Text: "x"})
	}
	shared := leaf()
	a := value.Ptr(value.Data{Children: []*parse.Node{shared, shared, nil}})
	value.D(a).Fields = noderep.Fields{{Name: "self", Value: a}, {Name: "nil", Value: (*parse.Node)(nil)}, {Name: "value", Value: 4}, {Name: "flag", Value: true}}
	b := a.Clone()
	if !equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("equal cyclic graph")
	}
	value.D(b).Children[1] = leaf()
	if equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("changed alias topology omitted")
	}
	b = a.Clone()
	value.D(value.D(b).Children[0]).Start++
	if equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("position omitted")
	}
	b = a.Clone()
	value.D(b).Fields[2].Value = 5
	if equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("scalar omitted")
	}
	b = a.Clone()
	value.D(b).Fields[1].Value = nil
	if equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("typed nil omitted")
	}
	b = a.Clone()
	value.D(b).Fields[2], value.D(b).Fields[3] = value.D(b).Fields[3], value.D(b).Fields[2]
	if equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("field order omitted")
	}
	for _, pair := range [][2]*parse.Node{{{}, value.Ptr(value.Data{Children: []*parse.Node{}})}, {{}, value.Ptr(value.Data{Fields: noderep.Fields{}})}} {
		if equalChangeSnapshots(changeGraphSnapshot(t, pair[0]), changeGraphSnapshot(t, pair[1])) {
			t.Fatal("nil versus empty omitted")
		}
	}
	x, y := changeGraphSnapshot(t, a), changeGraphSnapshot(t, a.Clone())
	x.parseError = &parse.SyntaxError{Pos: 1, Line: 1, Col: 2, Expected: []string{"x"}}
	y.parseError = &parse.SyntaxError{Pos: 2, Line: 1, Col: 2, Expected: []string{"x"}}
	if equalChangeSnapshots(x, y) {
		t.Fatal("diagnostic position hidden by formatting")
	}
}

func TestDocumentChangesComparisonUsesCumulativeBudget(t *testing.T) {
	leaf := value.Ptr(value.Data{Kind: noderep.KindOf("Leaf", "leaf"), Terminal: true, Text: strings.Repeat("x", 8192)})
	a, b := changeGraphSnapshot(t, leaf), changeGraphSnapshot(t, leaf.Clone())
	control := core.NewBudgetControl(nil, parse.Limits{Compute: 8192})
	control.Charge(4096) // Budget already spent by parsing/capture in this invocation.
	p := &core.Parser{Control: control}
	if err := catchFatal(t, func() { equalChangeSnapshotsControlled(p, a, b) }); !errors.Is(err, parse.ErrLimit) {
		t.Fatal("comparison escaped cumulative spend", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := catchFatal(t, func() {
		equalChangeSnapshotsControlled(&core.Parser{Control: core.NewBudgetControl(ctx, parse.Limits{})}, a, b)
	}); !errors.Is(err, context.Canceled) {
		t.Fatal("comparison ignored cancellation", err)
	}
}
