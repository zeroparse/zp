package ref

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/internal/ir/exec"
)

// The reference model of the missing-token mode (the operations of component recovered with the
// feature missing): the tokens inserted so far, in order, each at a site of the grammar and a
// position with the error it recovered, and the tokens that a memo entry's call inserted. A token
// is inserted at most once at a site and position, until missing.reset removes it.

// Names of the operations that conformance cases use to insert a token and to record the tokens that
// a call inserted. They are not operations of the parser VM: the grammar's code makes them.
const (
	MissingInsertOp = "test.missingInsert"
	MissingSeedOp   = "test.missingSeed"
)

// missingToken is a token inserted at a site and position, with the error it recovered.
type missingToken struct {
	site, pos int64
	err       *Err
}

// missingState is the state of the missing-token mode.
type missingState struct {
	path      []missingToken
	installed map[[2]int64]bool
}

// allowsMissing reports whether a call of an operation of the missing-token mode meets its
// precondition.
func (m *Model) allowsMissing(o *exec.Op, args []any) (allowed, ok bool) {
	switch o.Name {
	case "missing.reset":
		mark := args[0].(int64)
		return mark >= 0 && mark <= int64(len(m.missing.path)), true
	case "missing.replay":
		return m.cfg.Missing && !args[0].(*Entry).pruned, true
	case MissingInsertOp:
		return m.cfg.Missing, true
	case MissingSeedOp:
		mark := args[1].(int64)
		return m.cfg.Missing && !args[0].(*Entry).pruned && mark >= 0 && mark <= int64(len(m.missing.path)), true
	}
	return false, false
}

// callMissing runs an operation of the missing-token mode, and reports false for any other.
func (m *Model) callMissing(o *exec.Op, args []any) ([]any, bool) {
	s := &m.missing
	switch o.Name {
	case "missing.active":
		return []any{m.cfg.Missing}, true
	case "missing.recompute":
		i := args[0].(RuleRef).Index
		return []any{m.cfg.Missing && int(i) < len(m.cfg.MissingRules) && m.cfg.MissingRules[i]}, true
	case "missing.mark":
		return []any{int64(len(s.path))}, true
	case "missing.reset":
		if !m.cfg.Missing {
			return nil, true
		}
		mark := args[0].(int64)
		for _, t := range s.path[mark:] {
			delete(s.installed, [2]int64{t.site, t.pos})
		}
		s.path = s.path[:mark]
		return nil, true
	case "missing.replay":
		e := args[0].(*Entry)
		dup := map[*Err]bool{}
		for _, t := range e.seed {
			if s.installed[[2]int64{t.site, t.pos}] {
				dup[t.err] = true
			} else {
				s.install(t)
			}
		}
		l, _ := e.errs.(*ErrList)
		switch {
		case l == nil || l.n == 0:
		case len(dup) == 0:
			m.rec = append(slices.Clip(m.rec), l)
		default:
			for _, err := range l.errPtrs(nil) {
				if !dup[err] {
					m.rec = append(slices.Clip(m.rec), err.list())
				}
			}
		}
		return nil, true
	case MissingInsertOp:
		t := missingToken{args[0].(int64), args[1].(int64), args[2].(*Err)}
		if s.installed[[2]int64{t.site, t.pos}] {
			return []any{false}, true
		}
		s.install(t)
		m.rec = append(slices.Clip(m.rec), t.err.list())
		return []any{true}, true
	case MissingSeedOp:
		e := args[0].(*Entry)
		e.seed = slices.Clone(s.path[args[1].(int64):])
		return nil, true
	}
	return nil, false
}

func (s *missingState) install(t missingToken) {
	if s.installed == nil {
		s.installed = map[[2]int64]bool{}
	}
	s.installed[[2]int64{t.site, t.pos}] = true
	s.path = append(s.path, t)
}

// errPtrs appends the errors of l, in order, to errs.
func (l *ErrList) errPtrs(errs []*Err) []*Err {
	if l.entries == nil {
		return append(errs, l.ptr)
	}
	for _, e := range l.entries {
		errs = e.errPtrs(errs)
	}
	return errs
}

// observeMissing returns the tokens inserted so far, for a parse in the missing-token mode, and nil
// for any other parse.
func (m *Model) observeMissing() []string {
	if !m.cfg.Missing {
		return nil
	}
	out := []string{}
	for _, t := range m.missing.path {
		out = append(out, fmt.Sprintf("site %d at %d error %d", t.site, t.pos, t.err.ID))
	}
	return out
}

// errsQuantity returns ops.QErrs for a call of o with args, in the state before the call.
func (m *Model) errsQuantity(o *exec.Op, args []any) int64 {
	switch o.Name {
	case "missing.replay":
		if l, ok := args[0].(*Entry).errs.(*ErrList); ok {
			return int64(l.n)
		}
		return 0
	}
	panic("ref: no quantity errs for " + o.Name)
}

// tokensQuantity returns ops.QTokens for a call of o with args, in the state before the call.
func (m *Model) tokensQuantity(o *exec.Op, args []any) int64 {
	switch o.Name {
	case "missing.reset":
		if !m.cfg.Missing {
			return 0
		}
		return int64(len(m.missing.path)) - args[0].(int64)
	case "missing.replay":
		return int64(len(args[0].(*Entry).seed))
	}
	panic("ref: no quantity tokens for " + o.Name)
}
