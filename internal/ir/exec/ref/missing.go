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
		m.resetMissing(args[0].(int64))
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
		return []any{m.insertMissing(missingToken{args[0].(int64), args[1].(int64), args[2].(*Err)})}, true
	case MissingSeedOp:
		e := args[0].(*Entry)
		e.seed = slices.Clone(s.path[args[1].(int64):])
		return nil, true
	}
	return nil, false
}

// insertMissing inserts the token t and records its error, unless a token is at its site and
// position, and reports whether it inserted it.
func (m *Model) insertMissing(t missingToken) bool {
	s := &m.missing
	if s.installed[[2]int64{t.site, t.pos}] {
		return false
	}
	s.install(t)
	m.rec = append(slices.Clip(m.rec), t.err.list())
	return true
}

// resetMissing removes the tokens inserted after mark, in the missing-token mode.
func (m *Model) resetMissing(mark int64) {
	s := &m.missing
	if !m.cfg.Missing {
		return
	}
	for _, t := range s.path[mark:] {
		delete(s.installed, [2]int64{t.site, t.pos})
	}
	s.path = s.path[:mark]
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
		// The errors are copied one by one only to leave out those of tokens already present; otherwise
		// they are shared as rec.add shares them.
		e := args[0].(*Entry)
		if l, ok := e.errs.(*ErrList); ok && m.missing.repeats(e.seed) {
			return int64(l.n)
		}
		return 0
	}
	panic("ref: no quantity errs for " + o.Name)
}

// repeats reports whether recording the tokens again finds one of them present: inserted before, or
// earlier among them.
func (s *missingState) repeats(tokens []missingToken) bool {
	seen := map[[2]int64]bool{}
	for _, t := range tokens {
		k := [2]int64{t.site, t.pos}
		if s.installed[k] || seen[k] {
			return true
		}
		seen[k] = true
	}
	return false
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
	case "rule.invokePlain":
		// The tokens that a failed evaluation inserted, which the call removes.
		e := m.next(args[0].(RuleRef).Index)
		if !m.cfg.Missing || e.OK {
			return 0
		}
		n, seen := int64(0), map[[2]int64]bool{}
		for _, t := range e.Missing {
			k := [2]int64{t.Site, t.Pos}
			if !m.missing.installed[k] && !seen[k] {
				n++
			}
			seen[k] = true
		}
		return n
	}
	panic("ref: no quantity tokens for " + o.Name)
}
