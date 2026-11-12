package ref

import (
	"slices"
	"strings"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// The capture frames (ops.Frames): the frame that captures write to, the slots of each frame, the
// trail of the writes that a reset undoes, and the copy of the slots of a frame that frame.save sets
// aside. A backtracking point is the position, the environment, the entries of the recovered-error
// list, the mark of the missing-token record and the length of the trail when frame.mark took it.

// The conformance operations of the capture frames. They are not operations of the parser VM.
const (
	// FrameSlotOp returns the value in a slot of a frame, which nothing else of the parser VM reads
	// apart from the compositions that build values.
	FrameSlotOp = "test.slot"
	// FrameMissingOp makes its argument the mark of the missing-token record, which missing.mark
	// returns and frame.mark takes, as if the record held that many inserted tokens.
	FrameMissingOp = "test.missing"
)

// Frame is the model's capture frame: the values of its slots, nil for an empty slot.
type Frame struct {
	vals []any
	// closed reports a frame that frame.close ended: a target may reuse it for a later frame.
	closed bool
}

// Mark is the model's backtracking point.
type Mark struct {
	pos, rec, missing, trail int64
	env                      *Scope
}

type frameUndo struct {
	f    *Frame
	slot int64
	old  any
}

type frameState struct {
	current *Frame
	trail   []frameUndo
	// empty is the frame of no slots, which every frame.open of none returns.
	empty *Frame
	// saved is the copy of the slots of savedOf that the last frame.save set aside; savedOf is nil
	// when nothing was saved or the parse went on since.
	saved   []any
	savedOf *Frame
	missing int64
}

func isFrameOp(name string) bool {
	return strings.HasPrefix(name, "frame.") || name == FrameSlotOp || name == FrameMissingOp
}

// liveFrame reports whether f is a frame that frame.open returned and frame.close did not end.
func liveFrame(f any) bool {
	x, ok := f.(*Frame)
	return ok && x != nil && !x.closed
}

func (m *Model) allowsFrame(o *exec.Op, args []any) bool {
	s := &m.frm
	switch o.Name {
	case "frame.markPos", "frame.markRec", "frame.markEnv", "frame.markMissing", "frame.markTrail":
		_, ok := args[0].(*Mark)
		return ok
	case "frame.undo":
		return args[0].(int64) >= 0 && args[0].(int64) <= int64(len(s.trail))
	case "frame.use":
		return args[0] == nil || liveFrame(args[0])
	case "frame.open":
		return args[0].(int64) >= 0
	case "frame.set":
		return liveFrame(s.current) && args[0].(int64) >= 0 && args[0].(int64) < int64(len(s.current.vals))
	case "frame.close":
		if !liveFrame(args[0]) {
			return false
		}
		f := args[0].(*Frame)
		return f != s.current && !slices.ContainsFunc(s.trail, func(u frameUndo) bool { return u.f == f })
	case "frame.slots", "frame.save":
		return liveFrame(args[0])
	case "frame.restore":
		return liveFrame(args[0]) && s.savedOf == args[0]
	case FrameSlotOp:
		return liveFrame(args[0]) && args[1].(int64) >= 0 && args[1].(int64) < int64(len(args[0].(*Frame).vals))
	case FrameMissingOp:
		return args[0].(int64) >= 0
	}
	return true
}

// frameQuantity returns ops.QSlots and ops.QTrail for a call of a frame operation.
func (m *Model) frameQuantity(name string, o *exec.Op, args []any) (int64, bool) {
	if o == nil || !isFrameOp(o.Name) {
		return 0, false
	}
	switch name {
	case ops.QSlots:
		if o.Name == "frame.open" {
			return args[0].(int64), true
		}
		return int64(len(args[0].(*Frame).vals)), true
	case ops.QTrail:
		return int64(len(m.frm.trail)) - args[0].(int64), true
	}
	return 0, false
}

// callFrame runs a frame operation, and reports false for any other.
func (m *Model) callFrame(o *exec.Op, args []any) ([]any, bool) {
	s := &m.frm
	switch o.Name {
	case "frame.mark":
		return []any{&Mark{pos: m.pos(), env: m.env, rec: int64(len(m.rec)), missing: s.missing, trail: int64(len(s.trail))}}, true
	case "frame.markPos":
		return []any{args[0].(*Mark).pos}, true
	case "frame.markRec":
		return []any{args[0].(*Mark).rec}, true
	case "frame.markEnv":
		return []any{args[0].(*Mark).env}, true
	case "frame.markMissing":
		return []any{args[0].(*Mark).missing}, true
	case "frame.markTrail":
		return []any{args[0].(*Mark).trail}, true
	case "frame.undo":
		n := args[0].(int64)
		for i := len(s.trail) - 1; i >= int(n); i-- {
			u := s.trail[i]
			u.f.vals[u.slot] = u.old
		}
		s.trail = s.trail[:n]
		return nil, true
	case "frame.current":
		if s.current == nil {
			return []any{nil}, true
		}
		return []any{s.current}, true
	case "frame.use":
		f, _ := args[0].(*Frame)
		s.current = f
		return nil, true
	case "frame.open":
		s.savedOf = nil
		n := args[0].(int64)
		if n == 0 {
			if s.empty == nil {
				s.empty = &Frame{vals: []any{}}
			}
			return []any{s.empty}, true
		}
		return []any{&Frame{vals: make([]any, n)}}, true
	case "frame.set":
		s.savedOf = nil
		slot := args[0].(int64)
		s.trail = append(s.trail, frameUndo{f: s.current, slot: slot, old: s.current.vals[slot]})
		s.current.vals[slot] = args[1]
		return nil, true
	case "frame.close":
		s.savedOf = nil
		// The frame of no slots is shared; it is never ended.
		if f := args[0].(*Frame); f != s.empty {
			f.closed = true
		}
		return nil, true
	case "frame.slots":
		return []any{int64(len(args[0].(*Frame).vals))}, true
	case "frame.save":
		f := args[0].(*Frame)
		s.saved, s.savedOf = slices.Clone(f.vals), f
		return nil, true
	case "frame.restore":
		copy(args[0].(*Frame).vals, s.saved)
		return nil, true
	case FrameSlotOp:
		return []any{args[0].(*Frame).vals[args[1].(int64)]}, true
	case FrameMissingOp:
		s.missing = args[0].(int64)
		return nil, true
	}
	return nil, false
}
