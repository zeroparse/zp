package state

import (
	"unsafe"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/runtime/nodes"
)

// termRegs holds the registers of the term programs that a parser evaluates, in four files (see
// TermProgram). An evaluation takes its registers above the current tops and gives them back when
// it returns.
type termRegs struct {
	a []any
	n []int
	s [][]*core.Node
	t []string
}

func (r *termRegs) bytes() uint64 {
	return uint64(cap(r.a))*anyBytes + uint64(cap(r.n))*intBytes +
		uint64(cap(r.s))*uint64(unsafe.Sizeof([]*core.Node(nil))) + uint64(cap(r.t))*uint64(unsafe.Sizeof(""))
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
	r.a, r.n, r.s, r.t = growRegs(r.a, t.NA), growRegs(r.n, t.NN), growRegs(r.s, t.NS), growRegs(r.t, t.NT)
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

func (t *TermProgram) run(c *EvalCtx, a []any, n []int, s [][]*core.Node, ts []string) (any, error) {
	p := c.P
	code := t.Code
	var err error
	for pc := 0; ; pc++ {
		in := &code[pc]
		switch in.Op {
		case TConst:
			a[in.Dst] = t.Consts[in.A]
		case TIntK:
			n[in.Dst] = int(in.A)
		case TMoveA:
			a[in.Dst] = a[in.A]
		case TMoveN:
			n[in.Dst] = n[in.A]
		case TMoveS:
			s[in.Dst] = s[in.A]
		case TMoveT:
			ts[in.Dst] = ts[in.A]
		case TCap:
			a[in.Dst] = nodes.UnboxNode(c.Frame.Vals[in.A])
		case TLocal:
			v, ok := c.local(t.Names[in.A])
			if !ok {
				return nil, p.ActionError("undefined capture $", t.Names[in.A])
			}
			a[in.Dst] = v
		case TVar:
			a[in.Dst] = p.VarValue(int(in.A))
		case TUnbox:
			a[in.Dst] = unboxed(a[in.A])
		case TFromB:
			a[in.Dst] = n[in.A] != 0
		case TFromT:
			a[in.Dst] = ts[in.A]
		case TNot:
			n[in.Dst] = 1 - n[in.A]
		case TLt:
			n[in.Dst] = b2i(n[in.A] < n[in.B])
		case TAdd:
			n[in.Dst] = n[in.A] + n[in.B]
		case TEqN:
			n[in.Dst] = b2i(n[in.A] == n[in.B])
		case TIsNil:
			n[in.Dst] = b2i(a[in.A] == nil)
		case TMember:
			m := &t.Members[in.B]
			if a[in.Dst], err = p.termMember(c, m.Kind, a[in.A], m.Name, m.Field); err != nil {
				return nil, err
			}
		case TUnary:
			if a[in.Dst], err = p.termUnary(int(in.B), a[in.A]); err != nil {
				return nil, err
			}
		case TBinary:
			if a[in.Dst], err = p.termBinary(int(in.C), a[in.A], a[in.B]); err != nil {
				return nil, err
			}
		case TLogic:
			var b bool
			if b, err = p.termLogic(int(in.B), a[in.A]); err != nil {
				return nil, err
			}
			n[in.Dst] = b2i(b)
		case TText:
			if ts[in.Dst], err = p.termText(a[in.A]); err != nil {
				return nil, err
			}
		case TTextEq:
			n[in.Dst] = b2i(p.equalText(ts[in.A], ts[in.B]))
		case TLength:
			if a[in.Dst], err = p.termLength(a[in.A]); err != nil {
				return nil, err
			}
		case TConvert:
			if a[in.Dst], err = p.termConvert(t.NodeTypes[in.B], a[in.A]); err != nil {
				return nil, err
			}
		case TConforms:
			n[in.Dst] = b2i(p.termConforms(t.Casts[in.B].Type, a[in.A]))
		case TCastErr:
			ct := &t.Casts[in.B]
			a[in.Dst] = p.CastError(a[in.A], ct.Type, c.Start, ct.Line, ct.Col)
		case TKids:
			n[in.Dst] = len(p.KidStack)
		case TPush:
			if err = p.termPush(c, t.Names[in.B], a[in.A]); err != nil {
				return nil, err
			}
		case TPushAll:
			if err = p.termPushItems(c, t.Names[in.B], a[in.A]); err != nil {
				return nil, err
			}
		case TAbove:
			s[in.Dst] = p.Kids(n[in.A])
		case THull:
			a[in.Dst] = p.termHull(c, in.B != 0, s[in.A])
		case TItems:
			if s[in.Dst], err = p.termItems(t.Names[in.B], a[in.A]); err != nil {
				return nil, err
			}
		case TItemAt:
			a[in.Dst] = p.termItemAt(s[in.A], n[in.B], in.C != 0)
		case TSeqLen:
			n[in.Dst] = len(s[in.A])
		case TStep:
			p.Spend()
		case TKeep:
			p.KidStack = append(p.KidStack, s[in.A][n[in.B]])
		case TFilter:
			var b bool
			if b, err = p.termFilterKeep(a[in.A]); err != nil {
				return nil, err
			}
			n[in.Dst] = b2i(b)
		case TCopy:
			a[in.Dst] = c.NewList(s[in.A])
		case TStruct:
			st := &t.Structs[in.A]
			a[in.Dst] = p.NewNode(nodes.Data{Kind: st.Kind, Fields: p.fields(st.Fields)})
		case TSetField:
			p.SetValueField(&nodes.D(a[in.A].(*core.Node)).Fields, t.Names[in.C], a[in.B])
		case TField:
			p.Spend()
			x := a[in.A].(*core.Node)
			p.SetValueField(&nodes.D(x).Fields, t.Names[in.C], a[in.B])
			n[in.Dst] = b2i(p.termCover(x, a[in.B], n[in.Dst] != 0))
		case TCover:
			n[in.Dst] = b2i(p.termCover(a[in.A].(*core.Node), a[in.B], n[in.C] != 0))
		case TMade:
			p.termMade(c, a[in.A].(*core.Node), n[in.B] != 0)
		case TJump:
			pc = int(in.Dst) - 1
		case TJumpF:
			if n[in.A] == 0 {
				pc = int(in.Dst) - 1
			}
		case TRet:
			return a[in.A], nil
		case TRetErr:
			return nil, a[in.A].(error)
		default:
			panic("core: term instruction " + string(rune('0'+in.Op)))
		}
	}
}

// local returns the value of the Operators Rule local name ($lhs, $rhs, $op or $value), with a box
// opened, and false if c has none of that name. Under budget controls it charges one unit for each
// local it passes, the one it finds included; the names are the compiler's, so comparing them is
// part of that step.
func (c *EvalCtx) local(name string) (any, bool) {
	for l := c.Locals; l != nil; l = l.Next {
		c.P.Spend()
		if l.Name == name {
			return nodes.UnboxValue(l.Val), true
		}
	}
	return nil, false
}
