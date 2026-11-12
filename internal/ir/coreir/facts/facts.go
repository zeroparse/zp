// Package facts computes the grammar facts of a Core IR module: per rule, its nullability, whether
// it always succeeds, its left calls and left-recursion cycles with their leaders, whether it is on
// a cycle of calls, whether its value may hold positions or recovered errors, the variables it
// reads and its memo policy; and, through coreir.Analyze, the node attributes that depend on them
// (first terminals and effects).
//
// The facts are those that spec/core-ir/attributes.md ("Rule Facts") defines. They are computed on
// the rules of the module and read nothing back from their Policy: Compute sets the Rec and Leader
// policies, which coreir.Analyze reads, from the facts it computed. A value-free twin has the facts
// of its rule, and a call of a twin counts as a call of its rule.
package facts

import (
	"slices"

	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/ir/coreir"
)

// Facts are the grammar facts of a module.
type Facts struct {
	// Rules holds the facts of each rule, by rule ID.
	Rules []Rule
}

// Rule holds the facts of one rule.
type Rule struct {
	// Nullable reports that the rule can succeed without consuming input when no token is
	// inserted; NullableIns, when token insertion is enabled.
	Nullable, NullableIns bool
	// Succeeds reports a rule that is proven never to fail: one off every left-recursion cycle
	// whose body always succeeds.
	Succeeds bool
	// Rec reports that the rule is on a cycle of calls.
	Rec bool
	// LeftCalls are the rules the body may call before it consumes input, in the order of the
	// body; a rule called at several such places is listed at each.
	LeftCalls []int
	// Cycle numbers the rule's left-recursion cycle from 1, or is 0 for a rule on none: rules
	// with the same number call each other through left calls.
	Cycle int
	// Leader reports a left-recursion leader; Member, a rule on a cycle that is not one.
	Leader, Member bool
	// LeftCallsIns, CycleIns, LeaderIns and MemberIns are LeftCalls, Cycle, Leader and Member
	// for a parse with token insertion enabled, where an insertion consumes no input: a rule may
	// be left-recursive only through an insertion.
	LeftCallsIns []int
	CycleIns     int
	LeaderIns    bool
	MemberIns    bool
	// Positional reports that the rule's actions or predicates, or those of a rule it calls, read
	// positions.
	Positional bool
	// Recovering is a rule whose #recover can put an error value in the rule's value (the rule
	// itself or one it calls), or -1.
	Recovering int
	// Vars are the variables that the rule or a rule it calls reads, sorted.
	Vars []string
	// Memo reports that ordinary parses may memoize the rule; Transient, that only incremental
	// parses do.
	Memo, Transient bool
	// Missing reports that the rule or a rule it calls has a #missing insertion.
	Missing bool
}

// Cyclic reports a rule on a left-recursion cycle.
func (r *Rule) Cyclic() bool { return r.Cycle != 0 }

// Compute computes the facts of the module's rules. It sets the Rec and Leader policies of every
// rule to the computed facts and runs coreir.Analyze, so that the node attributes follow them. The
// rule references of the module must be valid. A non-nil control polls for cancellation.
func Compute(m *coreir.Module, control *cancel.Control) *Facts {
	c := newComputer(m, control)
	c.recursion()
	for _, r := range m.Rules {
		if r != nil {
			r.Policy.Rec, r.Policy.Leader = c.rules[c.canon[r.ID]].Rec, false
		}
	}
	coreir.Analyze(m)
	for _, id := range c.order {
		at := m.Rules[id].Body.Attrs()
		c.rules[id].Nullable, c.rules[id].NullableIns = at.Nullable, at.NullableIns
	}
	c.leftRecursion(true)
	c.leftRecursion(false)
	c.positional()
	c.recovering()
	c.vars()
	c.memo()
	for _, r := range m.Rules {
		if r != nil {
			r.Policy.Leader = c.rules[c.canon[r.ID]].Leader
		}
	}
	coreir.Analyze(m)
	for _, r := range m.Rules {
		if r != nil && c.canon[r.ID] != r.ID {
			c.rules[r.ID] = c.rules[c.canon[r.ID]]
		}
	}
	return &Facts{Rules: c.rules}
}

type computer struct {
	m       *coreir.Module
	control *cancel.Control
	rules   []Rule
	// canon maps each rule ID to its rule: a twin's rule, or the rule itself.
	canon []int
	// order holds the IDs of the rules that are not twins, in order.
	order []int
	// calls holds, for each rule that is not a twin, the rules its body calls, at each call in
	// the order of the body (twins replaced by their rules).
	calls [][]int
	// sure holds the rules proven never to fail.
	sure []bool
	// inserts reports that the module has a Missing node, so that the left calls with token
	// insertion enabled may differ from those without.
	inserts bool
}

func newComputer(m *coreir.Module, control *cancel.Control) *computer {
	c := &computer{m: m, control: control, rules: make([]Rule, len(m.Rules)), canon: make([]int, len(m.Rules)),
		calls: make([][]int, len(m.Rules)), sure: make([]bool, len(m.Rules))}
	for i := range c.canon {
		c.canon[i] = i
	}
	for _, r := range m.Rules {
		if r != nil && r.Twin != nil {
			c.canon[r.Twin.ID] = r.ID
		}
	}
	for _, r := range m.Rules {
		control.Poll()
		if r == nil || c.canon[r.ID] != r.ID {
			continue
		}
		c.order = append(c.order, r.ID)
		c.rules[r.ID].Recovering = -1
		coreir.Walk(r.Body, func(n coreir.Node) {
			if k, ok := c.callee(n); ok {
				c.calls[r.ID] = append(c.calls[r.ID], k)
			}
			if _, ok := n.(*coreir.Missing); ok {
				c.inserts = true
			}
		})
	}
	return c
}

// callee returns the rule that n calls, if n is a call.
func (c *computer) callee(n coreir.Node) (int, bool) {
	switch n := n.(type) {
	case *coreir.Call:
		return c.canon[n.Rule], true
	case *coreir.Nest:
		return c.canon[n.Rule], true
	}
	return 0, false
}

// recursion sets Rec: a rule is on a cycle of calls when its component of the call graph has
// several rules or it calls itself.
func (c *computer) recursion() {
	for _, scc := range c.components(c.calls) {
		if len(scc) > 1 || slices.Contains(c.calls[scc[0]], scc[0]) {
			for _, id := range scc {
				c.rules[id].Rec = true
			}
		}
	}
}

// components returns the strongly connected components of the graph of edges, visiting the rules in
// order and their edges in order. A component comes after every component it reaches; the rules of
// a component are in the order they leave the stack.
func (c *computer) components(edges [][]int) [][]int {
	n := len(c.m.Rules)
	index, low := make([]int, n), make([]int, n)
	for i := range index {
		index[i] = -1
	}
	onStack := make([]bool, n)
	var stack []int
	var out [][]int
	next := 0
	var visit func(v int)
	visit = func(v int) {
		c.control.Poll()
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range edges[v] {
			if index[w] < 0 {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if onStack[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] != index[v] {
			return
		}
		var scc []int
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			scc = append(scc, w)
			if w == v {
				break
			}
		}
		out = append(out, scc)
	}
	for _, id := range c.order {
		if index[id] < 0 {
			visit(id)
		}
	}
	return out
}

// leftRecursion computes the left calls, the rules that always succeed, the left-recursion cycles
// and their leaders. Left calls depend on which rules always succeed, and only a rule off every
// cycle is proven to; so the cycles and the proofs are refined together until the left calls no
// longer change. Each round only removes left calls, so the refinement ends. With ins it computes
// the facts of a parse with token insertion enabled (LeftCallsIns, CycleIns, LeaderIns, MemberIns),
// with proofs of success of its own; it must run before the run without, which owns Succeeds.
func (c *computer) leftRecursion(ins bool) {
	if ins && !c.inserts {
		return // leftRecursion(false) copies its facts
	}
	clear(c.sure) // the analysis starts with no rule proven to succeed
	left := make([][]int, len(c.m.Rules))
	for _, id := range c.order {
		left[id] = c.leftCalls(c.m.Rules[id].Body, nil, ins)
	}
	cyclic := make([]bool, len(c.m.Rules))
	var comps [][]int
	for {
		c.control.Poll()
		comps = c.components(left)
		clear(cyclic)
		for _, scc := range comps {
			if len(scc) > 1 || slices.Contains(left[scc[0]], scc[0]) {
				for _, id := range scc {
					cyclic[id] = true
				}
			}
		}
		c.succeeding(cyclic)
		changed := false
		next := make([][]int, len(c.m.Rules))
		for _, id := range c.order {
			next[id] = c.leftCalls(c.m.Rules[id].Body, nil, ins)
			changed = changed || !slices.Equal(left[id], next[id])
		}
		if !changed {
			break
		}
		left = next
	}
	cycle := make([]int, len(c.m.Rules))
	leaders := make([]bool, len(c.m.Rules))
	for i, scc := range comps {
		if !cyclic[scc[0]] {
			continue
		}
		in := make([]bool, len(c.m.Rules))
		for _, id := range scc {
			in[id] = true
			cycle[id] = i + 1
		}
		// Until every cycle passes through a leader, the rule declared first on a cycle that passes
		// through none becomes a leader.
		for {
			found := c.findCycle(scc, left, in, leaders)
			if found == nil {
				break
			}
			leaders[slices.Min(found)] = true
		}
	}
	for _, id := range c.order {
		r := &c.rules[id]
		if ins {
			r.LeftCallsIns, r.CycleIns, r.LeaderIns, r.MemberIns = left[id], cycle[id], leaders[id], cyclic[id] && !leaders[id]
			continue
		}
		r.LeftCalls, r.Cycle, r.Leader, r.Member = left[id], cycle[id], leaders[id], cyclic[id] && !leaders[id]
		r.Succeeds = c.sure[id]
		if !c.inserts {
			r.LeftCallsIns, r.CycleIns, r.LeaderIns, r.MemberIns = r.LeftCalls, r.Cycle, r.Leader, r.Member
		}
	}
}

// findCycle returns a cycle of left calls among the rules in of the component scc that passes
// through no leader, or nil if there is none.
func (c *computer) findCycle(scc []int, edges [][]int, in, leaders []bool) []int {
	state := map[int]int{} // 0: unvisited, 1: on the path, 2: done
	var path, found []int
	var visit func(v int) bool
	visit = func(v int) bool {
		c.control.Poll()
		state[v] = 1
		path = append(path, v)
		for _, w := range edges[v] {
			if !in[w] || leaders[w] {
				continue
			}
			if state[w] == 1 {
				i := slices.Index(path, w)
				found = slices.Clone(path[i:])
				return true
			}
			if state[w] == 0 && visit(w) {
				return true
			}
		}
		path = path[:len(path)-1]
		state[v] = 2
		return false
	}
	for _, v := range scc {
		if !leaders[v] && state[v] == 0 && visit(v) {
			return found
		}
	}
	return nil
}

// succeeding sets sure to the rules off the cyclic ones whose bodies always succeed, the least
// fixed point.
func (c *computer) succeeding(cyclic []bool) {
	clear(c.sure)
	for changed := true; changed; {
		c.control.Poll()
		changed = false
		for _, id := range c.order {
			if !cyclic[id] && !c.sure[id] && c.alwaysSucceeds(c.m.Rules[id].Body) {
				c.sure[id], changed = true, true
			}
		}
	}
}

// leftCalls appends to acc the rules that n may call before it consumes input, with token
// insertion enabled if ins. A sequence goes on past an item only if the item is nullable (NullableIns
// if ins); a choice, past an alternative only if it can fail; a
// recovery's skip runs only if its body can fail; a repetition of at most zero iterations calls
// nothing. An Operators Rule may call its skip line, its operands up to one that always succeeds and
// its prefix operators.
func (c *computer) leftCalls(n coreir.Node, acc []int, ins bool) []int {
	c.control.Poll()
	switch n := n.(type) {
	case *coreir.Call, *coreir.Nest:
		k, _ := c.callee(n)
		return append(acc, k)
	case *coreir.Seq:
		for _, it := range n.Items {
			acc = c.leftCalls(it, acc, ins)
			if at := it.Attrs(); !at.Nullable && !(ins && at.NullableIns) { // NullableIns holds every Nullable node
				break
			}
		}
	case *coreir.Choice:
		for _, alt := range n.Alts {
			acc = c.leftCalls(alt.Body, acc, ins)
			if c.alwaysSucceeds(alt.Body) {
				break
			}
		}
	case *coreir.Repeat:
		if n.Max != 0 {
			acc = c.leftCalls(n.Body, acc, ins)
		}
	case *coreir.Recover:
		acc = c.leftCalls(n.Body, acc, ins)
		if !c.alwaysSucceeds(n.Body) {
			acc = c.leftCalls(n.Skip, acc, ins)
		}
	case *coreir.OperatorsRule:
		if n.Skip != nil {
			acc = c.leftCalls(n.Skip.Body, acc, ins)
		}
		for _, l := range n.Operands {
			acc = c.leftCalls(l.Body, acc, ins)
			if c.alwaysSucceeds(l.Body) {
				break
			}
		}
		for _, op := range n.Ops {
			if op.Kind == "prefix" {
				acc = c.leftCalls(op.Line.Body, acc, ins)
			}
		}
	case *coreir.Opt, *coreir.Look, *coreir.Text, *coreir.Drop, *coreir.Capture, *coreir.Label, *coreir.Stream, *coreir.Terminal:
		for _, x := range coreir.Children(n) {
			acc = c.leftCalls(x, acc, ins)
		}
	}
	return acc
}

// alwaysSucceeds reports a proof that n cannot fail, at any position, with the rules of sure
// proven. Errors that end the parse are not failures.
func (c *computer) alwaysSucceeds(n coreir.Node) bool {
	c.control.Poll()
	switch n := n.(type) {
	case *coreir.Lit:
		return n.Text == ""
	case *coreir.Succeed, *coreir.Cut:
		return true
	case *coreir.Call:
		return c.sure[c.canon[n.Rule]]
	case *coreir.Seq:
		for _, it := range n.Items {
			if !c.alwaysSucceeds(it) {
				return false
			}
		}
		return true
	case *coreir.Choice:
		for _, alt := range n.Alts {
			if c.alwaysSucceeds(alt.Body) {
				return true
			}
			if looseCut(alt.Body) {
				return false // it may fail after committing the choice
			}
		}
	case *coreir.Opt:
		return !looseCut(n.Body) || c.alwaysSucceeds(n.Body)
	case *coreir.Repeat:
		if n.Max == 0 {
			return n.Min == 0
		}
		if n.Min == 0 {
			return !looseCut(n.Body) || c.alwaysSucceeds(n.Body)
		}
		// A repetition ends at its first empty iteration, so a body that always succeeds proves
		// no minimum above one.
		return n.Min == 1 && c.alwaysSucceeds(n.Body)
	case *coreir.Look:
		if n.Neg {
			return c.alwaysFails(n.Body)
		}
		return c.alwaysSucceeds(n.Body)
	case *coreir.Capture:
		return c.alwaysSucceeds(n.Body)
	case *coreir.Text:
		return c.alwaysSucceeds(n.Body)
	case *coreir.Drop:
		return c.alwaysSucceeds(n.Body)
	case *coreir.Label:
		return c.alwaysSucceeds(n.Body)
	case *coreir.Recover:
		return c.alwaysSucceeds(n.Body)
	case *coreir.Stream:
		return c.alwaysSucceeds(n.Repeat)
	case *coreir.Terminal:
		return c.alwaysSucceeds(n.Body)
	}
	return false
}

// alwaysFails reports a proof that n cannot succeed.
func (c *computer) alwaysFails(n coreir.Node) bool {
	c.control.Poll()
	switch n := n.(type) {
	case *coreir.FailNode:
		return true
	case *coreir.Look:
		if n.Neg {
			return c.alwaysSucceeds(n.Body)
		}
		return c.alwaysFails(n.Body)
	case *coreir.Seq:
		for _, it := range n.Items {
			if c.alwaysFails(it) {
				return true
			}
		}
	case *coreir.Choice:
		for _, alt := range n.Alts {
			if !c.alwaysFails(alt.Body) {
				return false
			}
		}
		return true
	case *coreir.Capture:
		return c.alwaysFails(n.Body)
	case *coreir.Text:
		return c.alwaysFails(n.Body)
	case *coreir.Drop:
		return c.alwaysFails(n.Body)
	case *coreir.Label:
		return c.alwaysFails(n.Body)
	case *coreir.Stream:
		return c.alwaysFails(n.Repeat)
	case *coreir.Terminal:
		return c.alwaysFails(n.Body)
	}
	return false
}

// looseCut reports a cut in n that can commit the enclosing choice, option or repetition: one
// outside nested choices, options, repetitions, lookaheads and calls. A recovery's skip runs in the
// scope of its body.
func looseCut(n coreir.Node) bool {
	switch n := n.(type) {
	case *coreir.Cut:
		return true
	case *coreir.Seq:
		return slices.ContainsFunc(n.Items, looseCut)
	case *coreir.Capture, *coreir.Text, *coreir.Drop, *coreir.Label, *coreir.Recover, *coreir.Terminal:
		return slices.ContainsFunc(coreir.Children(n), looseCut)
	}
	return false
}

// positional sets Positional: a rule whose actions or predicates read .Start or .End, or that calls
// such a rule.
func (c *computer) positional() {
	set := make([]bool, len(c.m.Rules))
	for _, id := range c.order {
		c.terms(c.m.Rules[id], func(t coreir.Term) {
			if m, ok := t.(*coreir.Member); ok && (m.Name == "Start" || m.Name == "End") {
				set[id] = true
			}
		})
	}
	c.propagate(set)
	for _, id := range c.order {
		c.rules[id].Positional = set[id]
	}
}

// terms calls f for every term of the rule: its action, its predicates and bindings, and the actions
// of its Operators Rule lines, with their subterms.
func (c *computer) terms(r *coreir.Rule, f func(coreir.Term)) {
	coreir.WalkTerm(r.Finish.Action, f)
	coreir.Walk(r.Body, func(n coreir.Node) {
		c.control.Poll()
		switch n := n.(type) {
		case *coreir.Pred:
			coreir.WalkTerm(n.Term, f)
		case *coreir.BindNode:
			coreir.WalkTerm(n.Term, f)
		case *coreir.OperatorsRule:
			for _, l := range n.Lines() {
				coreir.WalkTerm(l.Action, f)
			}
		}
	})
}

// propagate adds to set every rule that calls a rule of set, directly or not.
func (c *computer) propagate(set []bool) {
	for changed := true; changed; {
		c.control.Poll()
		changed = false
		for _, id := range c.order {
			if !set[id] && slices.ContainsFunc(c.calls[id], func(k int) bool { return set[k] }) {
				set[id], changed = true, true
			}
		}
	}
}

// recovering sets Recovering: a rule with a #recover whose error value can reach its value, or a
// rule it calls where the value can reach it. Recoveries and calls inside lookaheads and Operators
// Rule skip lines do not count: their values do not reach the rule's value, and their ends discard
// the recovered errors. Neither does #missing, which a lookahead disables.
func (c *computer) recovering() {
	calls := make([][]int, len(c.m.Rules))
	for _, id := range c.order {
		var walk func(n coreir.Node)
		walk = func(n coreir.Node) {
			c.control.Poll()
			switch n := n.(type) {
			case *coreir.Look:
				return
			case *coreir.Recover:
				c.rules[id].Recovering = id
			case *coreir.OperatorsRule:
				for _, l := range n.Operands {
					walk(l.Body)
				}
				for _, op := range n.Ops {
					walk(op.Line.Body)
				}
				return
			}
			if k, ok := c.callee(n); ok {
				calls[id] = append(calls[id], k)
			}
			for _, x := range coreir.Children(n) {
				walk(x)
			}
		}
		walk(c.m.Rules[id].Body)
	}
	for changed := true; changed; {
		c.control.Poll()
		changed = false
		for _, id := range c.order {
			if c.rules[id].Recovering >= 0 {
				continue
			}
			for _, k := range calls[id] {
				if w := c.rules[k].Recovering; w >= 0 {
					c.rules[id].Recovering, changed = w, true
					break
				}
			}
		}
	}
}

// vars sets Vars: the variables that the terms of the rule or of a rule it calls read.
func (c *computer) vars() {
	direct := make([]map[string]bool, len(c.m.Rules))
	some := false
	for _, id := range c.order {
		c.terms(c.m.Rules[id], func(t coreir.Term) {
			if v, ok := t.(*coreir.VarRef); ok {
				if direct[id] == nil {
					direct[id] = map[string]bool{}
				}
				direct[id][v.Name], some = true, true
			}
		})
	}
	if !some {
		return
	}
	// Components come after the components they reach, so the variables of the callees outside a
	// component are known when it is reached; the rules of a component read the same variables.
	for _, scc := range c.components(c.calls) {
		all := map[string]bool{}
		for _, id := range scc {
			for v := range direct[id] {
				all[v] = true
			}
			for _, k := range c.calls[id] {
				for _, v := range c.rules[k].Vars {
					all[v] = true
				}
			}
		}
		if len(all) == 0 {
			continue
		}
		names := make([]string, 0, len(all))
		for v := range all {
			names = append(names, v)
		}
		slices.Sort(names)
		for _, id := range scc {
			c.rules[id].Vars = names
		}
	}
}

// memo sets the memo policy. Every rule but a leader, whose growth the runtime owns, may be
// memoized. Ordinary parses do not memoize a transient rule: one that calls no rule, or that is
// called at fewer than two places, unless it is a start rule on a cycle of rules that are each
// called at one place, which a parse can enter without a call. A rule is Missing when it or a
// rule it calls has a #missing insertion.
func (c *computer) memo() {
	refs := make([]int, len(c.m.Rules))
	caller := make([]int, len(c.m.Rules))
	for _, id := range c.order {
		for _, k := range c.calls[id] {
			refs[k]++
			caller[k] = id
		}
	}
	start := make([]bool, len(c.m.Rules))
	for _, id := range c.m.Starts {
		start[c.canon[id]] = true
	}
	// selfCalled reports that following the one caller of each rule from id leads back to id.
	selfCalled := func(id int) bool {
		seen := map[int]bool{}
		for r := id; refs[r] == 1 && !seen[r]; r = caller[r] {
			seen[r] = true
			if caller[r] == id {
				return true
			}
		}
		return false
	}
	missing := make([]bool, len(c.m.Rules))
	for _, id := range c.order {
		r := &c.rules[id]
		r.Memo = !r.Leader
		r.Transient = len(c.calls[id]) == 0 || refs[id] < 2 && !(start[id] && selfCalled(id))
		coreir.Walk(c.m.Rules[id].Body, func(n coreir.Node) {
			if _, ok := n.(*coreir.Missing); ok {
				missing[id] = true
			}
		})
	}
	c.propagate(missing)
	for _, id := range c.order {
		c.rules[id].Missing = missing[id]
	}
}
