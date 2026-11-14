package ref

import "zeroparse.dev/zp/internal/ir/exec"

// The reference model of the examined input of a Document parse (the doc operations of the component
// call): the input that the active rule call examined is the range [low, examined), which a call starts
// at its position, widens as it reads, and merges into the enclosing call's when it ends. A fresh
// parse starts with the empty range at 0. Calls of a rule that a Document parse does not make
// (the model's Config.Document is false) leave it unchanged.

// docState is the state of the doc operations.
type docState struct {
	low, examined int64
	// enters are the ranges that the calls in progress began with, innermost last (doc.enter,
	// doc.leave); begins are the examined ends that observed calls in progress began with (doc.examinedBegin,
	// doc.examinedEnd).
	enters, begins    []docRange
	reused, evaluated int64
}

type docRange struct{ examined, low int64 }

// UseRegisters hands the model the registers of the run (exec.RegisterMachine), which doc.examinedBegin
// reads.
func (m *Model) UseRegisters(regs map[string]any) { m.regs = regs }

// DocStat returns the statistics of the Document: the memo entries reused (0) and the rule bodies
// evaluated (1).
func (m *Model) DocStat(i int64) int64 {
	if i == 0 {
		return m.doc.reused
	}
	return m.doc.evaluated
}

// DocRange returns the examined range of the current call, for tests.
func (m *Model) DocRange() (low, examined int64) { return m.doc.low, m.doc.examined }

// allowsDoc reports whether a call of an operation of the doc operations meets its precondition.
func (m *Model) allowsDoc(o *exec.Op, args []any) bool {
	s := &m.doc
	switch o.Name {
	case "doc.enter", "doc.leave", "doc.low", "doc.examined", "doc.reused", "doc.evaluated":
		if !m.cfg.Document {
			return false
		}
		if o.Name == "doc.leave" {
			return len(s.enters) > 0 && s.enters[len(s.enters)-1] == docRange{args[0].(int64), args[1].(int64)}
		}
	case "doc.examinedEnd":
		return len(s.begins) > 0 && s.begins[len(s.begins)-1].examined == args[0].(int64)
	}
	return true
}

// callDoc runs an operation of the doc operations, and reports false for any other.
func (m *Model) callDoc(o *exec.Op, args []any) ([]any, bool) {
	s := &m.doc
	switch o.Name {
	case "doc.active":
		return []any{m.cfg.Document}, true
	case "doc.enter":
		old := docRange{s.examined, s.low}
		s.enters = append(s.enters, old)
		pos := args[0].(int64)
		s.low, s.examined = pos, pos
		return []any{old.examined, old.low}, true
	case "doc.leave":
		s.enters = s.enters[:len(s.enters)-1]
		s.examined = max(args[0].(int64), s.examined)
		s.low = min(args[1].(int64), s.low)
		return nil, true
	case "doc.touch":
		if m.cfg.Document {
			s.examined = max(s.examined, args[0].(int64))
		}
		return nil, true
	case "doc.touchFrom":
		if m.cfg.Document {
			s.low = min(s.low, args[0].(int64))
		}
		return nil, true
	case "doc.low":
		return []any{s.low}, true
	case "doc.examined":
		return []any{s.examined}, true
	case "doc.reused":
		s.reused++
		return nil, true
	case "doc.evaluated":
		s.evaluated++
		return nil, true
	case "doc.examinedBegin":
		enclosing := s.examined
		if m.cfg.Document {
			s.examined = m.regs["pos"].(int64)
		}
		s.begins = append(s.begins, docRange{examined: enclosing})
		return []any{enclosing}, true
	case "doc.examinedEnd":
		enclosing := args[0].(int64)
		s.begins = s.begins[:len(s.begins)-1]
		if !m.cfg.Document {
			return []any{int64(0), false}, true
		}
		x := s.examined
		s.examined = max(enclosing, x)
		return []any{x, true}, true
	}
	return nil, false
}
