package flowpass

import (
	"cmp"
	"slices"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
)

// Finish ends the Flow passes on the module m of one lowered function, whichever of them ran: it
// splices the blocks of the forms (flow.Forms) into the statement lists around them, drops the
// variables that only replaced forms named, and orders the variables as the lowering would have made
// them had it written the forms the passes chose: a variable shared by the whole function (ch, size,
// ok) where its first use that is still in the function was written, and a variable a pass made at the
// event it belongs to. The function then has no forms (Attrs.Forms is nil). Finish does nothing to a
// function without forms.
func Finish(m *code.Module) {
	fn := flow.FuncOf(m)
	a := flow.AttrsOf(fn)
	if a == nil || a.Forms == nil {
		return
	}
	forms := a.Forms
	blocks := map[*code.Block]bool{}
	for _, f := range forms.Classes {
		blocks[f.Block] = true
	}
	for _, f := range forms.Regions {
		blocks[f.Block] = true
	}
	splice(fn.Body, blocks)
	a.Forms = nil

	// Drop the variables that the statements no longer name, of those they named after the lowering.
	named := flow.NamedVars(fn.Body)
	gone := map[string]bool{}
	for _, v := range a.Vars {
		if forms.Named[v] && !named[v] {
			gone[v] = true
		}
	}

	// Order the variables: each one the lowering made at its index in forms.Order, a shared one
	// before the variable that was next when its first live use was written, a variable a pass made
	// before the variable that was next at its event. Events at one point keep their order.
	type key struct{ at, seq int }
	index := map[string]int{}
	for i, v := range forms.Order {
		index[v] = i
	}
	keys := map[string]key{}
	for i, v := range a.Vars {
		if j, ok := index[v]; ok {
			keys[v] = key{2 * j, 0}
		} else {
			keys[v] = key{2 * len(forms.Order), i} // a pass made it and placed it nowhere
		}
	}
	shared := map[string]bool{}
	for seq, e := range forms.Events {
		if e.Name != "" && !shared[e.Name] && e.Live() {
			shared[e.Name] = true
			keys[e.Name] = key{2*e.Vars - 1, seq + 1}
		}
	}
	events := map[*flow.Event]int{}
	for seq, e := range forms.Events {
		events[e] = seq + 1
	}
	for v, e := range forms.Placed {
		keys[v] = key{2*e.Vars - 1, events[e]}
	}
	var vars []string
	for _, v := range a.Vars {
		if !gone[v] {
			vars = append(vars, v)
		}
	}
	slices.SortStableFunc(vars, func(x, y string) int {
		kx, ky := keys[x], keys[y]
		if c := cmp.Compare(kx.at, ky.at); c != 0 {
			return c
		}
		return cmp.Compare(kx.seq, ky.seq)
	})
	locals := make([]code.Param, 0, len(fn.Locals))
	byName := map[string]code.Param{}
	isVar := map[string]bool{}
	for _, v := range a.Vars {
		isVar[v] = true
	}
	for _, l := range fn.Locals {
		byName[l.Name] = l
	}
	// The locals follow the variables; Finish makes none, and keeps each once, so that a function
	// that a broken pass left invalid stays as invalid for the verifier to report.
	placed := map[string]bool{}
	for _, v := range vars {
		if l, ok := byName[v]; ok && !placed[v] {
			locals = append(locals, l)
			placed[v] = true
		}
	}
	for _, l := range fn.Locals {
		if !isVar[l.Name] {
			locals = append(locals, l)
		}
	}
	a.Vars, fn.Locals = vars, locals
}

// splice replaces each block of b's statements, at any depth, that is in blocks with its statements.
func splice(b *code.Block, blocks map[*code.Block]bool) {
	if b == nil {
		return
	}
	var out []code.Stmt
	changed := false
	for _, st := range b.Stmts {
		switch st := st.(type) {
		case *code.Block:
			splice(st, blocks)
			if blocks[st] {
				out = append(out, st.Stmts...)
				changed = true
				continue
			}
		case *code.Loop:
			splice(st.Body, blocks)
		case *code.If:
			splice(st.Then, blocks)
			splice(st.Else, blocks)
		case *code.Protect:
			splice(st.Body, blocks)
			splice(st.Finally, blocks)
		}
		out = append(out, st)
	}
	if changed {
		b.Stmts = out
	}
}
