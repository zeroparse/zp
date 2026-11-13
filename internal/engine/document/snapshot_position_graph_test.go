//go:build zp_snapshot_position_overlays

package document

import (
	"errors"
	"testing"

	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

func TestSnapshotPositionOverlayZeroAndOverflow(t *testing.T) {
	previous := snapshotGraph{snapshotPositionState: snapshotPositionState{positions: make([]snapshotPositionDelta, 128)}, nodes: make([]snapshotRecord, 128), root: 1}
	next := snapshotGraph{nodes: make([]snapshotRecord, 128), root: 1}
	for i := range previous.nodes {
		previous.nodes[i] = snapshotRecord{start: 3, end: 5, gen: 1}
		previous.positions[i] = snapshotPositionDelta{start: -3, end: 4, gen: 9}
		next.nodes[i] = snapshotRecord{start: 0, end: 9, gen: 9}
	}
	p := &core.Parser{Control: core.NewBudgetControl(nil, parse.Limits{})}
	shared := shareSnapshotGraph(p, &previous, next)
	n := snapshotHandle(&shared, 1)
	materialized := (&Snapshot{graph: shared}).Materialize()
	if shared.at(0) != previous.at(0) || shared.positions == nil || n.Start() != 0 || n.End() != 9 || materialized.Start() != 0 || materialized.End() != 9 || value.D(materialized).Mark != 9 {
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
	root := value.Ptr(value.Data{Start: 0, End: 128, Children: make([]*parse.Node, 128)})
	value.D(root).Fields = noderep.Fields{{Name: "self", Value: root}, {Name: "typedNil", Value: (*parse.Node)(nil)}, {Name: "scalarPosition", Value: 42}}
	for i := range value.D(root).Children {
		value.D(root).Children[i] = value.Ptr(value.Data{Start: int32(i), End: int32(i + 1), Mark: 1})
	}
	previous, err := freezeSnapshotGraph(&core.Parser{}, root)
	if err != nil {
		t.Fatal(err)
	}
	shifted := root.Clone()
	sd := value.D(shifted)
	sd.Start, sd.End, sd.Mark = 3, 131, 2
	for _, n := range sd.Children {
		d := value.D(n)
		d.Start += 3
		d.End += 3
		d.Mark = 2
	}
	next, err := freezeSnapshotGraph(&core.Parser{}, shifted)
	if err != nil {
		t.Fatal(err)
	}
	shared := shareSnapshotGraph(&core.Parser{}, &previous, next)
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
	if self, _ := value.D(materialized).Fields.Get("self"); self != materialized {
		t.Fatal("materialized cycle changed")
	}
	for i, n := range value.D(materialized).Children {
		if n.Start() != i+3 || n.End() != i+4 || value.D(n).Mark != 2 {
			t.Fatal("current metadata lost")
		}
	}
}

func TestSnapshotPositionOverlayBudgetAndRetry(t *testing.T) {
	if !core.BudgetControlsEnabled {
		t.Skip("budget control reference")
	}
	previous := snapshotGraph{nodes: make([]snapshotRecord, 128), root: 1}
	next := snapshotGraph{nodes: make([]snapshotRecord, 128), root: 1}
	for i := range next.nodes {
		next.nodes[i] = snapshotRecord{start: 3, end: 5, gen: 1}
	}
	p := &core.Parser{Control: core.NewBudgetControl(nil, parse.Limits{Compute: 264})}
	err := catchFatal(t, func() { shareSnapshotGraph(p, &previous, next) })
	var limit *parse.LimitError
	if !errors.As(err, &limit) || limit.Kind != parse.LimitCompute || limit.Requested != 3 {
		t.Fatal("overlay metadata copy escaped compute limit", err)
	}
	if previous.positions != nil || next.positions != nil || previous.nodes[0].start != 0 || next.nodes[0].start != 3 {
		t.Fatal("interrupted copy changed the input graphs")
	}
	shared := shareSnapshotGraph(&core.Parser{}, &previous, next)
	if shared.positions == nil || snapshotHandle(&shared, 1).Start() != 3 {
		t.Fatal("retry lost shifted positions")
	}
}
