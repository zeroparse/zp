package vm

import (
	"fmt"

	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/internal/runtime/state"
	"zeroparse.dev/zp/pkg/bytecode"
	"zeroparse.dev/zp/pkg/runtime/value"
)

// flowBody returns the body of rule r run from its Flow form f (bytecode.Flow): the same entry and
// result as the match code's body, with the operations of f in place of the instructions.
func flowBody(vm *Program, r *state.Rule, f *bytecode.Flow) func(p *state.Parser, min int) (*value.Node, bool) {
	return func(p *state.Parser, min int) (*value.Node, bool) {
		return vm.runFlow(p, f)
	}
}

// runFlow runs the Flow body f on the parser p. Each operation calls the parser's primitive that the
// Go form of the same operation calls, so that results and charges agree.
func (vm *Program) runFlow(p *state.Parser, f *bytecode.Flow) (*value.Node, bool) {
	regs := make([]any, f.Regs)
	set := func(d int, v any) {
		if d >= 0 {
			regs[d] = v
		}
	}
	for pc := 0; pc < len(f.Code); {
		in := &f.Code[pc]
		pc++
		switch in.Op {
		case bytecode.FlowConst:
			set(in.Dst[0], in.Imm)
		case bytecode.FlowMove:
			set(in.Dst[0], regs[in.Args[0]])
		case bytecode.FlowNot:
			set(in.Dst[0], !regs[in.Args[0]].(bool))
		case bytecode.FlowPos:
			set(in.Dst[0], p.Pos)
		case bytecode.FlowSetPos:
			p.Pos = regs[in.Args[0]].(int)
		case bytecode.FlowJump:
			pc = in.Target
		case bytecode.FlowJumpFalse:
			if !regs[in.Args[0]].(bool) {
				pc = in.Target
			}
		case bytecode.FlowReturn:
			return asNode(regs[in.Args[0]]), regs[in.Args[1]].(bool)
		case bytecode.FlowMark:
			set(in.Dst[0], p.Mark())
		case bytecode.FlowReset:
			p.Reset(regs[in.Args[0]].(state.Mark))
		case bytecode.FlowPeek:
			ch, size, ok := p.Peek()
			set(in.Dst[0], ch)
			set(in.Dst[1], size)
			set(in.Dst[2], ok)
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
			set(in.Dst[0], !hit && p.Depth+g.Depth <= p.MaxDepth)
		case bytecode.FlowExpect:
			x := in.Imm.(bytecode.FlowExpectOf)
			id := state.ExpID(x.Exp)
			if x.Message {
				id |= state.MsgBit
			}
			p.Expect(regs[in.Args[0]].(int), id)
		case bytecode.FlowCall:
			x := in.Imm.(bytecode.FlowCallOf)
			var v *value.Node
			var ok bool
			if r := vm.rules[x.Rule]; vm.direct[x.Rule] {
				v, ok = p.InvokePlain(r, x.Level) // what the call of the match code does for such a rule
			} else {
				v, ok = p.Call(r, x.Level)
			}
			set(in.Dst[0], v)
			set(in.Dst[1], ok)
		case bytecode.FlowUnfresh:
			if n := asNode(regs[in.Args[0]]); n != nil {
				nodes.D(n).Fresh = false
			}
		default:
			panic(fmt.Sprintf("internal error: vm: flow operation %d", in.Op))
		}
	}
	return nil, false
}

// asNode returns the node held in a register, nil for the null value.
func asNode(v any) *value.Node {
	n, _ := v.(*value.Node)
	return n
}
