package document

import (
	"context"
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/internal/runtime/state"
	"zeroparse.dev/zp/pkg/runtime"
	"zeroparse.dev/zp/pkg/runtime/value"
	"zeroparse.dev/zp/pkg/runtime/value/impl"
)

func changeGraphSnapshot(t *testing.T, root *value.Node) *Snapshot {
	t.Helper()
	graph, err := freezeSnapshotGraph(&state.Parser{}, root)
	if err != nil {
		t.Fatal(err)
	}
	return &Snapshot{graph: graph}
}

func TestDocumentChangesConservativeGraphEquality(t *testing.T) {
	leaf := func() *value.Node {
		return nodes.Ptr(nodes.Data{Kind: valueimpl.KindOf("Leaf", "leaf"), Start: 1, End: 2, Terminal: true, Text: "x"})
	}
	shared := leaf()
	a := nodes.Ptr(nodes.Data{Children: []*value.Node{shared, shared, nil}})
	nodes.D(a).Fields = valueimpl.Fields{{Name: "self", Value: a}, {Name: "nil", Value: (*value.Node)(nil)}, {Name: "value", Value: 4}, {Name: "flag", Value: true}}
	b := a.Clone()
	if !equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("equal cyclic graph")
	}
	nodes.D(b).Children[1] = leaf()
	if equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("changed alias topology omitted")
	}
	b = a.Clone()
	nodes.D(nodes.D(b).Children[0]).Start++
	if equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("position omitted")
	}
	b = a.Clone()
	nodes.D(b).Fields[2].Value = 5
	if equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("scalar omitted")
	}
	b = a.Clone()
	nodes.D(b).Fields[1].Value = nil
	if equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("typed nil omitted")
	}
	b = a.Clone()
	nodes.D(b).Fields[2], nodes.D(b).Fields[3] = nodes.D(b).Fields[3], nodes.D(b).Fields[2]
	if equalChangeSnapshots(changeGraphSnapshot(t, a), changeGraphSnapshot(t, b)) {
		t.Fatal("field order omitted")
	}
	for _, pair := range [][2]*value.Node{{{}, nodes.Ptr(nodes.Data{Children: []*value.Node{}})}, {{}, nodes.Ptr(nodes.Data{Fields: valueimpl.Fields{}})}} {
		if equalChangeSnapshots(changeGraphSnapshot(t, pair[0]), changeGraphSnapshot(t, pair[1])) {
			t.Fatal("nil versus empty omitted")
		}
	}
	x, y := changeGraphSnapshot(t, a), changeGraphSnapshot(t, a.Clone())
	x.parseError = &value.SyntaxError{Pos: 1, Line: 1, Col: 2, Expected: []string{"x"}}
	y.parseError = &value.SyntaxError{Pos: 2, Line: 1, Col: 2, Expected: []string{"x"}}
	if equalChangeSnapshots(x, y) {
		t.Fatal("diagnostic position hidden by formatting")
	}
}

func TestDocumentChangesComparisonUsesCumulativeBudget(t *testing.T) {
	leaf := nodes.Ptr(nodes.Data{Kind: valueimpl.KindOf("Leaf", "leaf"), Terminal: true, Text: strings.Repeat("x", 8192)})
	a, b := changeGraphSnapshot(t, leaf), changeGraphSnapshot(t, leaf.Clone())
	control := state.NewBudgetControl(nil, runtime.Limits{Compute: 8192})
	control.Charge(4096) // Budget already spent by parsing/capture in this invocation.
	p := &state.Parser{Control: control}
	if err := catchFatal(t, func() { equalChangeSnapshotsControlled(p, a, b) }); !errors.Is(err, runtime.ErrLimit) {
		t.Fatal("comparison escaped cumulative spend", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := catchFatal(t, func() {
		equalChangeSnapshotsControlled(&state.Parser{Control: state.NewBudgetControl(ctx, runtime.Limits{})}, a, b)
	}); !errors.Is(err, context.Canceled) {
		t.Fatal("comparison ignored cancellation", err)
	}
}
