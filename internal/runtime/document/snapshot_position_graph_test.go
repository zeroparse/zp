//go:build zp_snapshot_position_overlays

package document

import (
	"errors"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/internal/runtime/state"
)

func TestSnapshotPositionOverlayZeroAndOverflow(t *testing.T) {
	previous := snapshotGraph{snapshotPositionState: snapshotPositionState{positions: make([]snapshotPositionDelta, 128)}, nodes: make([]snapshotRecord, 128), root: 1}
	next := snapshotGraph{nodes: make([]snapshotRecord, 128), root: 1}
	for i := range previous.nodes {
		previous.nodes[i] = snapshotRecord{start: 3, end: 5, gen: 1}
		previous.positions[i] = snapshotPositionDelta{start: -3, end: 4, gen: 9}
		next.nodes[i] = snapshotRecord{start: 0, end: 9, gen: 9}
	}
	p := &state.Parser{Control: state.NewBudgetControl(nil, core.Limits{})}
	shared := shareSnapshotGraph(p, &previous, next)
	n := snapshotHandle(&shared, 1)
	materialized := (&Snapshot{graph: shared}).Materialize()
	if shared.at(0) != previous.at(0) || shared.positions == nil || n.Start() != 0 || n.End() != 9 || materialized.Start() != 0 || materialized.End() != 9 || nodes.D(materialized).Mark != 9 {
		t.Fatal("zero start or current generation was lost during exact re-sharing")
	}
	for i := range previous.nodes {
		previous.nodes[i] = snapshotRecord{start: -(1 << 31), end: -(1 << 31)}
		next.nodes[i] = snapshotRecord{start: (1 << 31) - 1, end: (1 << 31) - 1}
	}
	previous.positions = nil
	result := shareSnapshotGraph(p, &previous, next)
	if result.refs != nil || result.positions != nil || result.at(0).start != (1<<31)-1 {
		t.Fatal("unrepresentable relative offsets must remain owned records")
	}
}

func TestSnapshotPositionOverlayCyclesAndTypedNil(t *testing.T) {
	root := nodes.Ptr(nodes.Data{Start: 0, End: 128, Children: make([]*core.Node, 128)})
	nodes.D(root).Fields = coreimpl.Fields{{Name: "self", Value: root}, {Name: "typedNil", Value: (*core.Node)(nil)}, {Name: "scalarPosition", Value: 42}}
	for i := range nodes.D(root).Children {
		nodes.D(root).Children[i] = nodes.Ptr(nodes.Data{Start: int32(i), End: int32(i + 1), Mark: 1})
	}
	previous, err := freezeSnapshotGraph(&state.Parser{}, root)
	if err != nil {
		t.Fatal(err)
	}
	shifted := root.Clone()
	sd := nodes.D(shifted)
	sd.Start, sd.End, sd.Mark = 3, 131, 2
	for _, n := range sd.Children {
		d := nodes.D(n)
		d.Start += 3
		d.End += 3
		d.Mark = 2
	}
	next, err := freezeSnapshotGraph(&state.Parser{}, shifted)
	if err != nil {
		t.Fatal(err)
	}
	shared := shareSnapshotGraph(&state.Parser{}, &previous, next)
	s := &Snapshot{graph: shared}
	if shared.positions == nil {
		t.Fatal("shifted cyclic graph did not share payloads")
	}
	if self, _ := s.Root().Field("self"); self != s.Root() {
		t.Fatal("cycle identity changed")
	}
	if nilNode, _ := s.Root().Field("typedNil"); nilNode != (SnapshotNode{}) {
		t.Fatal("typed nil changed")
	}
	if scalar, _ := s.Root().Field("scalarPosition"); scalar != 42 {
		t.Fatal("scalar action value shifted")
	}
	materialized := s.Materialize()
	if self, _ := nodes.D(materialized).Fields.Get("self"); self != materialized {
		t.Fatal("materialized cycle changed")
	}
	for i, n := range nodes.D(materialized).Children {
		if n.Start() != i+3 || n.End() != i+4 || nodes.D(n).Mark != 2 {
			t.Fatal("current metadata lost")
		}
	}
}

func TestSnapshotPositionOverlayBudgetAndRetry(t *testing.T) {
	if !state.BudgetControlsEnabled {
		t.Skip("budget control reference")
	}
	previous := snapshotGraph{nodes: make([]snapshotRecord, 128), root: 1}
	next := snapshotGraph{nodes: make([]snapshotRecord, 128), root: 1}
	for i := range next.nodes {
		next.nodes[i] = snapshotRecord{start: 3, end: 5, gen: 1}
	}
	p := &state.Parser{Control: state.NewBudgetControl(nil, core.Limits{Compute: 264})}
	err := catchFatal(t, func() { shareSnapshotGraph(p, &previous, next) })
	var limit *core.LimitError
	if !errors.As(err, &limit) || limit.Kind != core.LimitCompute || limit.Requested != 3 {
		t.Fatal("overlay metadata copy escaped compute limit", err)
	}
	if previous.positions != nil || next.positions != nil || previous.nodes[0].start != 0 || next.nodes[0].start != 3 {
		t.Fatal("interrupted copy changed the input graphs")
	}
	shared := shareSnapshotGraph(&state.Parser{}, &previous, next)
	if shared.positions == nil || snapshotHandle(&shared, 1).Start() != 3 {
		t.Fatal("retry lost shifted positions")
	}
}
