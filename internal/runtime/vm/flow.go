package vm

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/internal/runtime/state"
	"zeroparse.dev/zp/pkg/bytecode"
	"zeroparse.dev/zp/pkg/runtime/value"
	valueimpl "zeroparse.dev/zp/pkg/runtime/value/impl"
)

// flowBody returns the body of rule r run from its Flow form f (bytecode.Flow) with the recursive
// model: the same entry and result as the match code's body, with the operations of f in place of
// the instructions, and each call of a rule a host call, as the match code's calls are.
func flowBody(vm *Program, r *state.Rule, f *bytecode.Flow) func(p *state.Parser, min int) (*value.Node, bool) {
	return func(p *state.Parser, min int) (*value.Node, bool) {
		fr := flowRun{f: f, regs: make([]any, f.Regs)}
		for {
			if !vm.runFlow(p, &fr) {
				return fr.v, fr.ok
			}
			x := fr.call()
			var v *value.Node
			var ok bool
			if r := vm.rules[x.Rule]; vm.direct[x.Rule] {
				v, ok = p.InvokePlain(r, x.Level) // what the call of the match code does for such a rule
			} else {
				v, ok = p.Call(r, x.Level)
			}
			fr.resume(v, ok)
		}
	}
}

// flowRun is a run of a Flow body: its registers and the instruction it is at. A run that reaches a
// CALL suspends there, so that a driver can run the call (with host recursion in the recursive
// model, as a frame of its own in the iterative model) and resume it with the result. When the body
// returns, v and ok hold its result.
type flowRun struct {
	f    *bytecode.Flow
	regs []any
	pc   int
	v    *value.Node
	ok   bool
}

// start makes fr a run of f from its first instruction, reusing the registers of fr. The registers
// past those of the last run are zero, so clearing those of the last run clears them all.
func (fr *flowRun) start(f *bytecode.Flow) {
	clear(fr.regs)
	*fr = flowRun{f: f, regs: slices.Grow(fr.regs[:0], f.Regs)[:f.Regs]}
}

// call returns the immediate of the CALL that fr is suspended at.
func (fr *flowRun) call() bytecode.FlowCallOf {
	return fr.f.Code[fr.pc].Imm.(bytecode.FlowCallOf)
}

// resume stores the result of the CALL that fr is suspended at and continues after it.
func (fr *flowRun) resume(v *value.Node, ok bool) {
	in := &fr.f.Code[fr.pc]
	fr.set(in.Dst[0], v)
	fr.set(in.Dst[1], ok)
	fr.pc++
}

func (fr *flowRun) set(d int, v any) {
	if d >= 0 {
		fr.regs[d] = v
	}
}

// runFlow runs the Flow body of fr on the parser p until it returns, with its result in fr, or
// reaches a CALL, which it reports. Each operation calls the parser's primitive that the Go form
// of the same operation calls, so that results and charges agree.
func (vm *Program) runFlow(p *state.Parser, fr *flowRun) (call bool) {
	f, regs := fr.f, fr.regs
	for pc := fr.pc; pc < len(f.Code); {
		in := &f.Code[pc]
		pc++
		switch in.Op {
		case bytecode.FlowConst:
			fr.set(in.Dst[0], in.Imm)
		case bytecode.FlowMove:
			fr.set(in.Dst[0], regs[in.Args[0]])
		case bytecode.FlowNot:
			fr.set(in.Dst[0], !regs[in.Args[0]].(bool))
		case bytecode.FlowPos:
			fr.set(in.Dst[0], p.Pos)
		case bytecode.FlowSetPos:
			p.Pos = regs[in.Args[0]].(int)
		case bytecode.FlowJump:
			pc = in.Target
		case bytecode.FlowJumpFalse:
			if !regs[in.Args[0]].(bool) {
				pc = in.Target
			}
		case bytecode.FlowReturn:
			fr.v, fr.ok = asNode(regs[in.Args[0]]), regs[in.Args[1]].(bool)
			fr.pc = len(f.Code)
			return false
		case bytecode.FlowMark:
			fr.set(in.Dst[0], p.Mark())
		case bytecode.FlowReset:
			p.Reset(regs[in.Args[0]].(state.Mark))
		case bytecode.FlowPeek:
			ch, size, ok := p.Peek()
			fr.set(in.Dst[0], ch)
			fr.set(in.Dst[1], size)
			fr.set(in.Dst[2], ok)
		case bytecode.FlowGuard:
			g := in.Imm.(bytecode.FlowGuardOf)
			ch, ok := regs[in.Args[0]].(rune), regs[in.Args[1]].(bool)
			var hit bool
			switch {
			case !ok:
			case g.Lit:
				hit = ch == g.Rune || g.Fold && ch == g.Rune-('a'-'A')
			default:
				hit = vm.has(p, int32(g.Class), ch)
			}
			fr.set(in.Dst[0], !hit && p.Depth+g.Depth <= p.MaxDepth)
		case bytecode.FlowExpect:
			x := in.Imm.(bytecode.FlowExpectOf)
			id := state.ExpID(x.Exp)
			if x.Message {
				id |= state.MsgBit
			}
			p.Expect(regs[in.Args[0]].(int), id)
		case bytecode.FlowCall:
			fr.pc = pc - 1
			return true
		case bytecode.FlowUnfresh:
			if n := asNode(regs[in.Args[0]]); n != nil {
				nodes.D(n).Fresh = false
			}
		case bytecode.FlowAdd:
			fr.set(in.Dst[0], regs[in.Args[0]].(int)+regs[in.Args[1]].(int))
		case bytecode.FlowSub:
			fr.set(in.Dst[0], regs[in.Args[0]].(int)-regs[in.Args[1]].(int))
		case bytecode.FlowEq:
			fr.set(in.Dst[0], flowEqual(regs[in.Args[0]], regs[in.Args[1]]))
		case bytecode.FlowNe:
			fr.set(in.Dst[0], !flowEqual(regs[in.Args[0]], regs[in.Args[1]]))
		case bytecode.FlowLt:
			fr.set(in.Dst[0], flowInt(regs[in.Args[0]]) < flowInt(regs[in.Args[1]]))
		case bytecode.FlowLe:
			fr.set(in.Dst[0], flowInt(regs[in.Args[0]]) <= flowInt(regs[in.Args[1]]))
		case bytecode.FlowStep:
			p.Spend()
		case bytecode.FlowSilent:
			// As the match code's LOOK and END_LOOK, and its unwinding of a lookahead.
			x := in.Imm.(bytecode.FlowSilenceOf)
			if x.Delta > 0 {
				p.Silent++
				p.LookBegin(x.Site)
			} else {
				p.LookEnd(x.Ok)
				p.Silent--
			}
		case bytecode.FlowAdvance:
			p.Pos += regs[in.Args[0]].(int)
		case bytecode.FlowInClass:
			fr.set(in.Dst[0], vm.has(p, int32(in.Imm.(int)), regs[in.Args[0]].(rune)))
		case bytecode.FlowAtAnchor:
			// As the match code's ASSERT.
			var at bool
			switch in.Imm.(int) {
			case 0:
				at = p.AtInputStart()
			case 1:
				at = p.AtEOF()
			case 2:
				at = p.AtLineStart()
			default:
				ch, _, more := p.Peek()
				at = !more || ch == '\n' || ch == '\r'
			}
			fr.set(in.Dst[0], at)
		case bytecode.FlowMatchLit:
			// As the match code's STR and STR_FOLD, except that a failure leaves the position where it
			// was, as flow.matchLit does.
			x := in.Imm.(bytecode.FlowLitOf)
			start := p.Pos
			var ok bool
			if x.Fold {
				ok = p.MatchLiteralFold(vm.runes[x.Str])
			} else {
				ok = p.MatchLiteral(vm.runes[x.Str], vm.bytes[x.Str])
			}
			if !ok {
				p.Expect(start, state.ExpID(x.Exp))
				p.Pos = start
			}
			fr.set(in.Dst[0], nil)
			fr.set(in.Dst[1], ok)
		case bytecode.FlowIsolate:
			fr.set(in.Dst[0], p.Isolate(p.Pos))
		case bytecode.FlowUnisolate:
			far, inner := p.Unisolate(regs[in.Args[0]].(state.ExpMark))
			fr.set(in.Dst[0], far)
			fr.set(in.Dst[1], inner)
		case bytecode.FlowMergeExpected:
			exps, _ := regs[in.Args[1]].([]state.ExpID)
			p.MergeExpected(regs[in.Args[0]].(int), exps)
		case bytecode.FlowNewTerminal:
			// As the Go library's NewTerminal: a unit for the value (spec/runtime/parsing.md#cost).
			p.Spend()
			text, _ := regs[in.Args[1]].(string)
			fr.set(in.Dst[0], p.NewNode(nodes.Data{Kind: valueimpl.MatchKind, Start: int32(regs[in.Args[0]].(int)), End: int32(p.Pos), Text: text, Terminal: true, Fresh: true}))
		case bytecode.FlowNewSeq, bytecode.FlowNewList:
			kind := valueimpl.SeqKind
			if in.Op == bytecode.FlowNewList {
				kind = valueimpl.ListKind
			}
			p.Spend()
			fr.set(in.Dst[0], p.NewNode(nodes.Data{Kind: kind, Start: int32(regs[in.Args[0]].(int)), End: int32(p.Pos), Children: flowVals(regs[in.Args[1]]), Fresh: true}))
		case bytecode.FlowNewVals:
			fr.set(in.Dst[0], p.Nodes(in.Imm.(int))[:0])
		case bytecode.FlowAppend:
			fr.set(in.Dst[0], append(flowVals(regs[in.Args[0]]), asNode(regs[in.Args[1]])))
		case bytecode.FlowSpread:
			fr.set(in.Dst[0], append(flowVals(regs[in.Args[0]]), nodes.D(asNode(regs[in.Args[1]])).Children...))
		case bytecode.FlowText:
			fr.set(in.Dst[0], p.Text(regs[in.Args[0]].(int), p.Pos))
		case bytecode.FlowLiteralText:
			fr.set(in.Dst[0], p.LiteralText(regs[in.Args[0]].(int), vm.literals[in.Imm.(int)]))
		case bytecode.FlowStr:
			// A string constant keeps its bytes: it is no matching text (nodes.LiteralValue).
			fr.set(in.Dst[0], vm.m.Strings[in.Imm.(int)])
		case bytecode.FlowKidsLen:
			fr.set(in.Dst[0], len(p.KidStack))
		case bytecode.FlowKids:
			fr.set(in.Dst[0], p.Kids(regs[in.Args[0]].(int)))
		case bytecode.FlowDropKids:
			p.TruncateKids(regs[in.Args[0]].(int))
		case bytecode.FlowPushKid:
			// As a framed body of the Go library pushes a value: the charge of the gathered value is
			// made where it is taken off (flow.kids).
			p.KidStack = append(p.KidStack, asNode(regs[in.Args[0]]))
		case bytecode.FlowShapeSingle:
			x := in.Imm.(bytecode.FlowShapeOf)
			v, start := asNode(regs[in.Args[0]]), regs[in.Args[1]].(int)
			switch x.Mode {
			case int(program.SinglePass):
				fr.set(in.Dst[0], flowShaper(x).One(v))
				fr.set(in.Dst[1], true)
			case int(program.SingleTuple):
				fr.set(in.Dst[0], flowShapeTuple(p, x, v, start))
				fr.set(in.Dst[1], true)
			default:
				fr.set(in.Dst[0], nil)
				fr.set(in.Dst[1], false)
			}
		case bytecode.FlowScopeValue:
			x := in.Imm.(bytecode.FlowShapeOf)
			v, start := asNode(regs[in.Args[0]]), regs[in.Args[1]].(int)
			if x.Tuple {
				fr.set(in.Dst[0], flowShapeTuple(p, x, v, start))
			} else {
				fr.set(in.Dst[0], flowShaper(x).One(v))
			}
		case bytecode.FlowSetCapture:
			p.SetCapture(flowSlot(p, p.Frame, in.Imm.(int)), asNode(regs[in.Args[0]]))
		case bytecode.FlowMarkPos:
			fr.set(in.Dst[0], regs[in.Args[0]].(state.Mark).Pos)
		case bytecode.FlowFrame:
			fr.set(in.Dst[0], p.Frame)
		case bytecode.FlowNewFrame:
			fr.set(in.Dst[0], p.NewFrame(len(vm.scopes[in.Imm.(int)])))
		case bytecode.FlowSetFrame:
			p.Frame = regs[in.Args[0]].(*state.Frame)
		case bytecode.FlowClearVals:
			clear(regs[in.Args[0]].(*state.Frame).Vals)
		case bytecode.FlowFrameVal:
			f := regs[in.Args[0]].(*state.Frame)
			fr.set(in.Dst[0], f.Vals[flowSlot(p, f, in.Imm.(int))])
		case bytecode.FlowAttachFrame:
			sc, f := vm.scopeOf[in.Imm.(int)], regs[in.Args[2]].(*state.Frame)
			if len(sc.Names) != len(f.Vals) {
				p.Fail("vm: a scope of %d names attached from a frame of %d slots", len(sc.Names), len(f.Vals))
			}
			fr.set(in.Dst[0], p.AttachCaptures(asNode(regs[in.Args[0]]), sc, f, regs[in.Args[1]].(int), p.Pos))
		case bytecode.FlowShapeCapture:
			// As the Go library's ShapeCapture: ShapeSingle, or the unit node of the range, which
			// charges a unit for the value (spec/runtime/parsing.md#cost).
			x := in.Imm.(bytecode.FlowShapeOf)
			v, start := asNode(regs[in.Args[0]]), regs[in.Args[1]].(int)
			switch x.Mode {
			case int(program.SinglePass):
				v = flowShaper(x).One(v)
			case int(program.SingleTuple):
				v = flowShapeTuple(p, x, v, start)
			default:
				p.Spend()
				v = p.NewNode(nodes.Data{Kind: valueimpl.SeqKind, Start: int32(start), End: int32(p.Pos), Fresh: true})
			}
			fr.set(in.Dst[0], v)
		default:
			panic(fmt.Sprintf("internal error: vm: flow operation %d", in.Op))
		}
	}
	// A loaded body ends with JUMP or RETURN (bytecode.ValidateModule), so it never gets here.
	panic("internal error: vm: a Flow body ran past its end")
}

// flowSlot returns the capture slot of the frame f that an instruction names, and fails the parse
// if f has no such slot: a loaded body names slots of the frames it may run with
// (bytecode.ValidateModule), but which frame is current is known only when it runs.
func flowSlot(p *state.Parser, f *state.Frame, slot int) int {
	if slot >= len(f.Vals) {
		p.Fail("vm: capture slot %d of a frame of %d slots", slot, len(f.Vals))
	}
	return slot
}

// asNode returns the node held in a register, nil for the null value.
func asNode(v any) *value.Node {
	n, _ := v.(*value.Node)
	return n
}

// flowInt returns the integer that a register holds: an int or a character.
func flowInt(v any) int {
	if r, ok := v.(rune); ok {
		return int(r)
	}
	return v.(int)
}

// flowEqual reports whether two registers hold equal values; an int and a character compare as
// integers.
func flowEqual(a, b any) bool {
	switch a.(type) {
	case int, rune:
		switch b.(type) {
		case int, rune:
			return flowInt(a) == flowInt(b)
		}
	}
	return a == b
}

// flowVals returns the list of values that a register holds; a register of a list never set holds
// nil.
func flowVals(v any) []*value.Node {
	vs, _ := v.([]*value.Node)
	return vs
}

func flowShaper(x bytecode.FlowShapeOf) program.Shaper {
	return program.Shaper{Elems: x.Elems, Shared: x.Shared, K: x.K}
}

// flowShapeTuple returns the tuple node of the result v of a match from start, as the Go library's
// shapeTuple does: a node that it builds charges a unit for the value (spec/runtime/parsing.md#cost).
func flowShapeTuple(p *state.Parser, x bytecode.FlowShapeOf, v *value.Node, start int) *value.Node {
	if x.Elems && !(x.Shared && v != nil) {
		return v
	}
	p.Spend()
	return state.ShapeTuple(p, flowShaper(x), v, start)
}
