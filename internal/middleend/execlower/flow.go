// Package execlower converts grammar code from the Flow stage of Code IR to the Exec stage (design
// 065), so that the backends of the Exec stage print the rules of a grammar as they print the
// compositions of the abstract machine. It also lowers the action expressions of the engine's term
// programs (Term).
//
// The conversion keeps the structure of each function (blocks, loops, branches, exits, assignments
// and the arithmetic and logic of conditions), its locals and its registers. Each call of a Flow
// operation becomes a call of the primitive operation of the Exec stage that stands for it (Prim):
// the same parameters, results and immediate, of the Exec dialect and the component Component, which
// each target prints in place in its own form. The result is checked by verifying it at the Exec
// stage, where an operation of the Flow dialect is not legal.
package execlower

import (
	"fmt"
	"slices"
	"strings"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
)

// Component is the state component of the primitive operations: the state of the runtime that each
// target implements.
const Component = "runtime"

// Stage is the name of the Exec stage of Code IR, and Dialect the name of its dialect.
const (
	Stage   = "exec"
	Dialect = "exec"
)

// Names of the registers of lowered functions, as at the Flow stage.
const (
	Pos = flow.Pos
	Cut = flow.Cut
)

// primPrefix begins the names of the primitive operations.
const primPrefix = Component + "."

// Prim returns the name of the primitive operation of the Exec stage that stands for the Flow
// operation name ("flow.matchLit" -> "runtime.matchLit").
func Prim(name string) string { return primPrefix + strings.TrimPrefix(name, flow.Dialect+".") }

// FlowOp returns the name of the Flow operation that the primitive operation name stands for, or ""
// if name is not a primitive operation.
func FlowOp(name string) string {
	if !strings.HasPrefix(name, primPrefix) {
		return ""
	}
	return flow.Dialect + "." + strings.TrimPrefix(name, primPrefix)
}

// Lowered is a function of grammar code at the Exec stage.
type Lowered struct {
	// Module declares the registers, handle types and operations that Func calls, and holds Func.
	Module *code.Module
	Func   *code.Func
	// Vars are the locals of Func that are variables of the lowered function (flow.Attrs.Vars); the
	// other locals are its entry variables, which the caller declares.
	Vars []string
}

// Lower converts the module m from the Flow stage to the Exec stage. The functions of m move to the
// result, which declares the registers and handle types of m and the primitive operation of each
// Flow operation; m is not used after. The result is verified, which checks that no call of a Flow
// operation remains.
func Lower(m *code.Module) (*code.Module, error) {
	if m.Stage != flow.Stage {
		return nil, fmt.Errorf("lower %s: a module at the %q stage, not the Flow stage", m.Name, m.Stage)
	}
	out := code.NewModule(m.Name, m.Features...)
	out.Stage, out.Dialects = Stage, []string{Dialect, "action"}
	out.AddComponent(Component, "The state of the runtime that the target implements.")
	out.Registers = m.Registers
	for _, h := range m.Handles {
		out.AddHandle(h.Name, Component, h.Doc)
	}
	for _, o := range m.Ops {
		if o.Dialect != flow.Dialect {
			out.AddOp(o)
			continue
		}
		out.AddOp(PrimOp(o))
	}
	for _, f := range m.Funcs {
		code.Walk(f.Body, nil, func(e code.Expr) {
			if c, ok := e.(*code.Call); ok && c.Fn == nil && strings.HasPrefix(c.Callee, flow.Dialect+".") {
				c.Callee = Prim(c.Callee)
			}
		})
		out.Funcs = append(out.Funcs, f)
	}
	m.Funcs = nil
	if err := code.Verify(out); err != nil {
		return nil, fmt.Errorf("lower %s: %w", m.Name, err)
	}
	return out, nil
}

// PrimOp returns the primitive operation of the Exec stage that stands for the Flow operation o: its
// signature, contract, cost, laws and immediate, with the state component of the runtime in place of
// the Flow component.
func PrimOp(o *code.Op) *code.Op {
	p := *o
	p.Name, p.Dialect, p.Component, p.Kind = Prim(o.Name), Dialect, Component, code.OpPrimitive
	state := func(names []string) []string {
		out := slices.Clone(names)
		for i, n := range out {
			if n == flow.State {
				out[i] = Component
			}
		}
		return out
	}
	p.Reads, p.Writes = state(o.Reads), state(o.Writes)
	p.Params, p.Results = slices.Clone(o.Params), slices.Clone(o.Results)
	return &p
}
