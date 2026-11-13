package core

import "fmt"

// Cursor filters edge-oriented node visits using spans in Unit. Unit must match
// the parse that populated Root's positions; it is never inferred or converted.
// Parent spans do not prune traversal. Scalar and list values have no spans.
type Cursor struct {
	Root *Node
	Unit PositionUnit
}

func (c Cursor) validate(start, end int32) error {
	if c.Unit != CodePoints && c.Unit != Bytes {
		return fmt.Errorf("zp: invalid cursor unit %d", c.Unit)
	}
	if start < 0 || end < start {
		return fmt.Errorf("zp: invalid cursor range [%d,%d)", start, end)
	}
	return nil
}

// At visits nodes containing position, including zero-width nodes at position.
func (c Cursor) At(position int32, visit func(WalkVisit) (WalkAction, error)) error {
	if err := c.validate(position, position); err != nil {
		return err
	}
	return c.filter(visit, func(n *Node) bool {
		return n.d.Start == n.d.End && n.d.Start == position || n.d.Start <= position && position < n.d.End
	})
}

// Range visits nodes overlapping [start,end). An empty range matches no nodes.
// Zero-width nodes match when start <= node.Start < end.
func (c Cursor) Range(start, end int32, visit func(WalkVisit) (WalkAction, error)) error {
	if err := c.validate(start, end); err != nil {
		return err
	}
	return c.filter(visit, func(n *Node) bool {
		if start == end {
			return false
		}
		if n.d.Start == n.d.End {
			return start <= n.d.Start && n.d.Start < end
		}
		return n.d.Start < end && start < n.d.End
	})
}

func (c Cursor) filter(visit func(WalkVisit) (WalkAction, error), matches func(*Node) bool) error {
	if visit == nil {
		return fmt.Errorf("zp: nil cursor callback")
	}
	return Walk(c.Root, func(v WalkVisit) (WalkAction, error) {
		if v.Node != nil && matches(v.Node) {
			return visit(v)
		}
		return WalkContinue, nil
	})
}
