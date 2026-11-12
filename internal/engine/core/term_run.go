package core

import (
	"unsafe"

	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/parse"
)

// termRegs holds the registers of the term programs that a parser evaluates, in four files (see
// TermProgram). An evaluation takes its registers above the current tops and gives them back when
// it returns.
type termRegs struct {
	a []any
	n []int
	s [][]*parse.Node
	t []string
}

func (r *termRegs) bytes() uint64 {
	return uint64(cap(r.a))*anyBytes + uint64(cap(r.n))*intBytes +
		uint64(cap(r.s))*uint64(unsafe.Sizeof([]*parse.Node(nil))) + uint64(cap(r.t))*uint64(unsafe.Sizeof(""))
}

// growRegs extends s by n registers. The registers above the tops are zero: Eval clears what it
// used before it gives them back.
func growRegs[T any](s []T, n int32) []T {
	l := len(s) + int(n)
	if l > cap(s) {
		s = append(s[:cap(s)], make([]T, l-cap(s))...)
	}
	return s[:l]
}

// Eval evaluates the term in the context c and returns its value, or the failure of the term. It
// is the term's Evaluator.
func (t *TermProgram) Eval(c *EvalCtx) (any, error) {
	p := c.P
	r := &p.terms
	ab, nb, sb, tb := len(r.a), len(r.n), len(r.s), len(r.t)
	r.a, r.n, r.s, r.t = growRegs(r.a, t.na), growRegs(r.n, t.nn), growRegs(r.s, t.ns), growRegs(r.t, t.nt)
	a, n, s, ts := r.a[ab:], r.n[nb:], r.s[sb:], r.t[tb:]
	v, err := t.run(c, a, n, s, ts)
	clear(a)
	clear(s)
	clear(ts)
	r.a, r.n, r.s, r.t = r.a[:ab], r.n[:nb], r.s[:sb], r.t[:tb]
	return v, err
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (t *TermProgram) run(c *EvalCtx, a []any, n []int, s [][]*parse.Node, ts []string) (any, error) {
	p := c.P
	code := t.code
	var err error
	for pc := 0; ; pc++ {
		in := &code[pc]
		switch in.op {
		case tConst:
			a[in.dst] = t.consts[in.a]
		case tIntK:
			n[in.dst] = int(in.a)
		case tMoveA:
			a[in.dst] = a[in.a]
		case tMoveN:
			n[in.dst] = n[in.a]
		case tMoveS:
			s[in.dst] = s[in.a]
		case tMoveT:
			ts[in.dst] = ts[in.a]
		case tCap:
			a[in.dst] = value.UnboxNode(c.Frame.Vals[in.a])
		case tLocal:
			v, ok := c.local(t.names[in.a])
			if !ok {
				return nil, p.ActionError("undefined capture $", t.names[in.a])
			}
			a[in.dst] = v
		case tVar:
			a[in.dst] = p.VarValue(int(in.a))
		case tUnbox:
			a[in.dst] = unboxed(a[in.a])
		case tFromB:
			a[in.dst] = n[in.a] != 0
		case tFromT:
			a[in.dst] = ts[in.a]
		case tNot:
			n[in.dst] = 1 - n[in.a]
		case tLt:
			n[in.dst] = b2i(n[in.a] < n[in.b])
		case tAdd:
			n[in.dst] = n[in.a] + n[in.b]
		case tEqN:
			n[in.dst] = b2i(n[in.a] == n[in.b])
		case tIsNil:
			n[in.dst] = b2i(a[in.a] == nil)
		case tMember:
			m := &t.members[in.b]
			if a[in.dst], err = p.termMember(c, m.kind, a[in.a], m.name, m.field); err != nil {
				return nil, err
			}
		case tUnary:
			if a[in.dst], err = p.termUnary(int(in.b), a[in.a]); err != nil {
				return nil, err
			}
		case tBinary:
			if a[in.dst], err = p.termBinary(int(in.c), a[in.a], a[in.b]); err != nil {
				return nil, err
			}
		case tLogic:
			var b bool
			if b, err = p.termLogic(int(in.b), a[in.a]); err != nil {
				return nil, err
			}
			n[in.dst] = b2i(b)
		case tText:
			if ts[in.dst], err = p.termText(a[in.a]); err != nil {
				return nil, err
			}
		case tTextEq:
			n[in.dst] = b2i(p.equalText(ts[in.a], ts[in.b]))
		case tLength:
			if a[in.dst], err = p.termLength(a[in.a]); err != nil {
				return nil, err
			}
		case tConvert:
			if a[in.dst], err = p.termConvert(t.nodeTypes[in.b], a[in.a]); err != nil {
				return nil, err
			}
		case tConforms:
			n[in.dst] = b2i(p.termConforms(t.casts[in.b].t, a[in.a]))
		case tCastErr:
			ct := &t.casts[in.b]
			a[in.dst] = p.CastError(a[in.a], ct.t, c.Start, ct.line, ct.col)
		case tKids:
			n[in.dst] = len(p.KidStack)
		case tPush:
			if err = p.termPush(c, t.names[in.b], a[in.a]); err != nil {
				return nil, err
			}
		case tPushAll:
			if err = p.termPushItems(c, t.names[in.b], a[in.a]); err != nil {
				return nil, err
			}
		case tAbove:
			s[in.dst] = p.Kids(n[in.a])
		case tHull:
			a[in.dst] = p.termHull(c, in.b != 0, s[in.a])
		case tItems:
			if s[in.dst], err = p.termItems(t.names[in.b], a[in.a]); err != nil {
				return nil, err
			}
		case tItemAt:
			a[in.dst] = p.termItemAt(s[in.a], n[in.b], in.c != 0)
		case tSeqLen:
			n[in.dst] = len(s[in.a])
		case tStep:
			p.Spend()
		case tKeep:
			p.KidStack = append(p.KidStack, s[in.a][n[in.b]])
		case tFilter:
			var b bool
			if b, err = p.termFilterKeep(a[in.a]); err != nil {
				return nil, err
			}
			n[in.dst] = b2i(b)
		case tCopy:
			a[in.dst] = c.NewList(s[in.a])
		case tStruct:
			st := &t.structs[in.a]
			a[in.dst] = p.NewNode(value.Data{Kind: st.kind, Fields: p.fields(st.fields)})
		case tSetField:
			p.SetValueField(&value.D(a[in.a].(*parse.Node)).Fields, t.names[in.c], a[in.b])
		case tCover:
			n[in.dst] = b2i(p.termCover(a[in.a].(*parse.Node), a[in.b], n[in.c] != 0))
		case tMade:
			p.termMade(c, a[in.a].(*parse.Node), n[in.b] != 0)
		case tJump:
			pc = int(in.dst) - 1
		case tJumpF:
			if n[in.a] == 0 {
				pc = int(in.dst) - 1
			}
		case tRet:
			return a[in.a], nil
		case tRetErr:
			return nil, a[in.a].(error)
		default:
			panic("core: term instruction " + string(rune('0'+in.op)))
		}
	}
}

// local returns the value of the Operators Rule local name ($lhs, $rhs, $op or $value), with a box
// opened, and false if c has none of that name. Under budget controls it charges one unit for each local it passes.
func (c *EvalCtx) local(name string) (any, bool) {
	for l := c.Locals; l != nil; l = l.Next {
		c.P.Spend()
		if c.P.ValueEqual(l.Name, name) {
			return value.UnboxValue(l.Val), true
		}
	}
	return nil, false
}
