//go:build zp_snapshot_position_overlays

package document

// PositionOverlay reports whether the graph keeps its positions as an overlay of deltas over
// the records of the previous snapshot.
func (g SnapshotGraph) PositionOverlay() bool { return g.g.positions != nil }
