package vm

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/internal/runtime/state"
)

// flowBody returns the body of rule r run from its Flow form f (bytecode.Flow) with the recursive
// model: the same entry and result as the match code's body, with the operations of f in place of
// the instructions, and each call of a rule a host call, as the match code's calls are.
func flowBody(vm *Program, r *state.Rule, f *bytecode.Flow) func(p *state.Parser, min int) (*core.Node, bool) {
	return func(p *state.Parser, min int) (*core.Node, bool) {
		fr := flowRun{f: f, regs: make([]any, f.Regs)}
		for {
			if !vm.runFlow(p, &fr) {
				return fr.v, fr.ok
			}
			x := fr.call()
			var v *core.Node
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
	v    *core.Node
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
func (fr *flowRun) resume(v *core.Node, ok bool) {
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
		default:
			panic(fmt.Sprintf("internal error: vm: flow operation %d", in.Op))
		}
	}
	// A loaded body ends with JUMP or RETURN (bytecode.ValidateModule), so it never gets here.
	panic("internal error: vm: a Flow body ran past its end")
}

// asNode returns the node held in a register, nil for the null value.
func asNode(v any) *core.Node {
	n, _ := v.(*core.Node)
	return n
}
