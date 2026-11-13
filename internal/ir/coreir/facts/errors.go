package facts

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/internal/ir/coreir"
)

// Error is a grammar that the facts reject: a construct that is valid in itself but not on a
// left-recursion cycle, or in a cycle of splice rules. Node is the construct the message is
// about, for the caller to find its source position; Rule is nil when the message is about the
// rule itself (a splice cycle).
type Error struct {
	Rule *coreir.Rule
	Node coreir.Node
	Msg  string
}

// Errors returns the errors that depend on the facts, in the order of the rules: first the
// splice cycle (at most one), then, per rule, a #stream on a left-recursion cycle, and the
// bindings of variables that a left-recursion cycle reads (see bindings). If bindings is false the
// bindings are accepted.
func (f *Facts) Errors(m *coreir.Module, bindings bool) []Error {
	var errs []Error
	errs = f.spliceCycle(m, errs)
	for _, r := range m.Rules {
		if r == nil || r.Policy.NoValue {
			continue
		}
		if f.Rules[r.ID].Cyclic() {
			coreir.Walk(r.Body, func(n coreir.Node) {
				if _, ok := n.(*coreir.Stream); ok {
					errs = append(errs, Error{Rule: r, Node: n, Msg: fmt.Sprintf("#stream cannot be used in rule %s, which is on a left-recursion cycle; growing the recursion would re-evaluate elements already delivered", r.Name)})
				}
			})
		}
	}
	if bindings {
		seen := map[coreir.Node]bool{}
		errs = append(errs, f.bindings(m, false, seen)...)
		if f.c.inserts {
			errs = append(errs, f.bindings(m, true, seen)...)
		}
	}
	return errs
}

// bindings rejects a binding of a variable that a left-recursion cycle reads when the binding may
// run before a left call into the cycle. Such a binding would give the re-entered call another
// variable environment than the call being grown; the language defines left recursion only for
// re-entries in the same environment. The bindings before a call are those that may run before it
// in the same rule body and before any input is consumed; bindings in a called rule end with the
// call. The cycles are those with token insertion enabled if ins. Bindings in seen are already
// reported.
func (f *Facts) bindings(m *coreir.Module, ins bool, seen map[coreir.Node]bool) []Error {
	if seen == nil {
		seen = map[coreir.Node]bool{}
	}
	var errs []Error
	for _, r := range m.Rules {
		if r == nil || r.Policy.NoValue {
			continue
		}
		fr := &f.Rules[r.ID]
		cycle := fr.Cycle
		if ins {
			cycle = fr.CycleIns
		}
		if cycle == 0 || len(fr.Vars) == 0 {
			continue
		}
		f.leftBindings(r.Body, nil, ins, func(callee int, bound []*coreir.BindNode) {
			c := &f.Rules[callee]
			if k := c.Cycle; ins && c.CycleIns != cycle || !ins && k != cycle {
				return
			}
			for _, b := range bound {
				if !seen[b] && slices.Contains(fr.Vars, b.Var) {
					seen[b] = true
					errs = append(errs, Error{Rule: r, Node: b, Msg: fmt.Sprintf("rule %s binds variable %s before its left-recursive call of %s; the left recursion reads %s", r.Name, b.Var, f.c.m.Rules[callee].Name, b.Var)})
				}
			}
		})
	}
	return errs
}

// leftBindings visits the calls that leftCalls finds in n, each with the bindings that may run
// before it, given the bindings in that may run before n. It returns the bindings that may run
// before a node following n. The result over-approximates: a binding in a choice alternative or an
// option counts even when input is consumed or the binding is undone before the call runs.
func (f *Facts) leftBindings(n coreir.Node, in []*coreir.BindNode, ins bool, visit func(int, []*coreir.BindNode)) []*coreir.BindNode {
	f.c.control.Poll()
	nullable := func(x coreir.Node) bool {
		at := x.Attrs()
		return at.Nullable || ins && at.NullableIns
	}
	switch n := n.(type) {
	case *coreir.Call, *coreir.Nest:
		k, _ := f.c.callee(n)
		visit(k, in)
	case *coreir.BindNode:
		in = append(slices.Clip(in), n)
	case *coreir.Seq:
		for _, it := range n.Items {
			in = f.leftBindings(it, in, ins, visit)
			if !nullable(it) {
				break
			}
		}
	case *coreir.Choice:
		out := in
		for _, alt := range n.Alts {
			out = unionBinds(out, f.leftBindings(alt.Body, in, ins, visit))
			if f.Succeeds(alt.Body) {
				break
			}
		}
		in = out
	case *coreir.Repeat:
		if n.Max != 0 {
			out := f.leftBindings(n.Body, in, ins, visit)
			if nullable(n.Body) && len(out) > len(in) {
				// A later iteration may start without consuming input after an earlier one bound.
				out = f.leftBindings(n.Body, out, ins, visit)
			}
			in = out
		}
	case *coreir.Look:
		f.leftBindings(n.Body, in, ins, visit)
		if !n.Neg {
			// A positive lookahead returns to its start and keeps its bindings, so every binding
			// in its body may run before what follows. A negative one succeeds only when its body
			// fails, which undoes its bindings.
			coreir.Walk(n.Body, func(x coreir.Node) {
				if b, ok := x.(*coreir.BindNode); ok {
					in = append(slices.Clip(in), b)
				}
			})
		}
	case *coreir.Opt, *coreir.Drop, *coreir.Capture, *coreir.Label, *coreir.Text, *coreir.Terminal:
		for _, x := range coreir.Children(n) {
			in = f.leftBindings(x, in, ins, visit)
		}
	case *coreir.Recover:
		in = f.leftBindings(n.Body, in, ins, visit)
		if !f.Succeeds(n.Body) {
			in = f.leftBindings(n.Skip, in, ins, visit)
		}
	case *coreir.OperatorsRule:
		if n.Skip != nil {
			in = f.leftBindings(n.Skip.Body, in, ins, visit)
		}
		out := in
		for _, l := range n.Operands {
			out = unionBinds(out, f.leftBindings(l.Body, in, ins, visit))
			if f.Succeeds(l.Body) {
				break
			}
		}
		for _, op := range n.Ops {
			if op.Kind == "prefix" {
				out = unionBinds(out, f.leftBindings(op.Line.Body, in, ins, visit))
			}
		}
		in = out
	}
	return in
}

// unionBinds returns the bindings of a followed by those of b that a does not hold. Every list
// extends a common prefix, so the result is small and keeps the order of discovery.
func unionBinds(a, b []*coreir.BindNode) []*coreir.BindNode {
	for _, x := range b {
		if !slices.Contains(a, x) {
			a = append(slices.Clip(a), x)
		}
	}
	return a
}

// spliceCycle reports a splice rule whose elements include its own: it calls itself in the
// sequence of its elements, so that its elements would be infinite in number. A call in a choice,
// an option, a repetition or a capture is one element of its own, and may recur.
func (f *Facts) spliceCycle(m *coreir.Module, errs []Error) []Error {
	flat := map[int][]int{}
	for _, r := range m.Rules {
		if r != nil && r.Policy.Splice && r.Twin == nil && !r.Policy.NoValue {
			flat[r.ID] = f.flatCalls(r.Body, nil)
		}
	}
	state := map[int]int{} // 1 visiting, 2 done
	var visit func(id int) bool
	visit = func(id int) bool {
		switch state[id] {
		case 1:
			return true
		case 2:
			return false
		}
		state[id] = 1
		for _, to := range flat[id] {
			if _, ok := flat[to]; ok && visit(to) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	for _, r := range m.Rules {
		f.c.control.Poll()
		if r == nil {
			continue
		}
		if _, ok := flat[r.ID]; ok && state[r.ID] == 0 && visit(r.ID) {
			return append(errs, Error{Rule: r, Msg: fmt.Sprintf("rule %s contains itself in its sequence of elements, so it has no fixed number of them", r.Name)})
		}
	}
	return errs
}

// flatCalls appends to out the rules n calls where their elements are elements of n.
func (f *Facts) flatCalls(n coreir.Node, out []int) []int {
	switch n := n.(type) {
	case *coreir.Seq:
		for _, it := range n.Items {
			out = f.flatCalls(it, out)
		}
	case *coreir.Label:
		out = f.flatCalls(n.Body, out)
	case *coreir.Call:
		k, _ := f.c.callee(n)
		out = append(out, k)
	}
	return out
}
