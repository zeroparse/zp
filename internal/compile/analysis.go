package compile

import (
	"slices"
	"sort"
	"zeroparse.dev/zp/internal/cancel"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/grammaranalysis"
)

// Analysis is the result of static analysis across rules.
type Analysis struct {
	Control    *cancel.Control
	nullable   map[string]bool
	succeeding map[string]bool
	// leaders are the rules that break left-recursion cycles. These rules are evaluated with
	// grow-the-seed.
	leaders map[string]bool
	// cyclic are the rules that belong to a left-recursion cycle (including leaders).
	cyclic map[string]bool
	// cycle numbers the left-recursion cycles: rules with the same number are in the same
	// strongly connected component of the left-call graph. Only cyclic rules have an entry.
	cycle map[string]int
	// positional are the rules whose actions or predicates read positions (.Start, .End),
	// directly or through callees. Their results may contain position values, so incremental
	// parsing cannot shift them past an edit.
	positional map[string]bool
	// recovering maps each rule whose value can hold an error value of #recover to a rule that has
	// such a #recover: the rule itself or one that it calls (recoveringRules).
	recovering map[string]string
}

// analyze computes the Analysis of rules without a compilation control (AnalyzeControlled).
func analyze(rules []*grammar.RuleDef) *Analysis {
	return AnalyzeControlled(rules, nil)
}

// AnalyzeControlled computes the Analysis of rules: the nullable and always-succeeding rules,
// the left-recursion cycles and their leaders, and the rules that read positions. A non-nil
// control polls for cancellation.
func AnalyzeControlled(rules []*grammar.RuleDef, control *cancel.Control) *Analysis {
	a := &Analysis{
		Control:    control,
		nullable:   map[string]bool{},
		leaders:    map[string]bool{},
		cyclic:     map[string]bool{},
		cycle:      map[string]int{},
		positional: map[string]bool{},
	}
	byName := map[string]*grammar.RuleDef{}
	for _, r := range rules {
		control.Poll()
		byName[r.Name] = r
	}

	// Fixed-point computation of nullability.
	for changed := true; changed; {
		control.Poll()
		changed = false
		for _, r := range rules {
			control.Poll()
			if !a.nullable[r.Name] && a.isNullable(r.Expr) {
				a.nullable[r.Name] = true
				changed = true
			}
		}
	}

	// Propagate variable reads.
	calls := map[string][]string{}
	for _, r := range rules {
		control.Poll()
		calls[r.Name] = allCallsControlled(r.Expr, nil, control)
	}
	for _, r := range rules {
		control.Poll()
		if ruleUsesPositions(r, control) {
			a.positional[r.Name] = true
		}
	}
	propagate(rules, calls, a.positional, control)
	a.recovering = recoveringRules(rules, control)

	// Find left-recursion cycles from the strongly connected components of the left-call graph.
	order := map[string]int{}
	left := map[string][]string{}
	for i, r := range rules {
		control.Poll()
		order[r.Name] = i
		for _, c := range a.leftCalls(r.Expr, nil) {
			control.Poll()
			if _, ok := byName[c]; ok {
				left[r.Name] = append(left[r.Name], c)
			}
		}
	}
	// Removing unreachable calls can break a cycle and make another rule's
	// success provable. Repeat until the graph stabilizes; each pass only
	// removes edges, so there are finitely many refinements.
	var components [][]string
	for {
		control.Poll()
		components = Tarjan(rules, left, control)
		clear(a.cyclic)
		for _, scc := range components {
			control.Poll()
			if len(scc) > 1 || contains(left[scc[0]], scc[0]) {
				for _, n := range scc {
					control.Poll()
					a.cyclic[n] = true
				}
			}
		}
		if control == nil {
			a.succeeding = grammaranalysis.SucceedingRules(rules, a.cyclic)
		} else {
			a.succeeding = grammaranalysis.SucceedingRulesWithPoll(rules, a.cyclic, control.Poll)
		}
		next := map[string][]string{}
		changed := false
		for _, r := range rules {
			control.Poll()
			for _, n := range a.leftCalls(r.Expr, nil) {
				control.Poll()
				if byName[n] != nil {
					next[r.Name] = append(next[r.Name], n)
				}
			}
			changed = changed || !slices.Equal(left[r.Name], next[r.Name])
		}
		if !changed {
			break
		}
		left = next
	}
	for i, scc := range components {
		control.Poll()
		if len(scc) == 1 && !contains(left[scc[0]], scc[0]) {
			continue
		}
		in := map[string]bool{}
		for _, n := range scc {
			control.Poll()
			in[n] = true
			a.cyclic[n] = true
			a.cycle[n] = i
		}
		// Until every cycle passes through some leader, make the first rule of a found cycle a leader.
		for {
			control.Poll()
			cycle := findCycle(scc, left, in, a.leaders, control)
			if cycle == nil {
				break
			}
			best := cycle[0]
			for _, n := range cycle {
				control.Poll()
				if order[n] < order[best] {
					best = n
				}
			}
			a.leaders[best] = true
		}
	}
	return a
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func (a *Analysis) isNullable(e grammar.Expr) bool {
	a.Control.Poll()
	switch e := e.(type) {
	case *grammar.Ref:
		return a.nullable[e.Name]
	case *grammar.Literal:
		return e.Value == ""
	case *grammar.CharClass, *grammar.Any, *grammar.Bottom:
		return false
	case *grammar.Seq:
		for _, it := range e.Items {
			if !a.isNullable(it) {
				return false
			}
		}
		return true
	case *grammar.Choice:
		for _, alt := range e.Alts {
			if a.isNullable(alt) {
				return true
			}
		}
		return false
	case *grammar.Repeat:
		return e.Min == 0 || e.Min == 1 && a.isNullable(e.Expr)
	case *grammar.Atomic:
		return a.isNullable(e.Expr)
	case *grammar.Discard:
		return a.isNullable(e.Expr)
	case *grammar.Capture:
		return a.isNullable(e.Expr)
	case *grammar.Attributed:
		return a.isNullable(e.Expr)
	case *grammar.OperatorsRule:
		for _, o := range e.Operands {
			if a.isNullable(o.Expr) {
				return true
			}
		}
		return false
	default:
		// Optional, lookahead, predicate, anchor, cut, top
		return true
	}
}

// leftCalls returns the rules that may be called before any input is consumed.
func (a *Analysis) leftCalls(e grammar.Expr, acc []string) []string {
	a.Control.Poll()
	switch e := e.(type) {
	case *grammar.Ref:
		return append(acc, e.Name)
	case *grammar.Seq:
		for _, it := range e.Items {
			acc = a.leftCalls(it, acc)
			if !a.isNullable(it) {
				break
			}
		}
	case *grammar.Choice:
		for _, alt := range e.Alts {
			acc = a.leftCalls(alt, acc)
			if a.AlwaysSucceeds(alt) {
				break
			}
		}
	case *grammar.Repeat:
		if e.Max != 0 {
			acc = a.leftCalls(e.Expr, acc)
		}
	case *grammar.Optional:
		acc = a.leftCalls(e.Expr, acc)
	case *grammar.And:
		acc = a.leftCalls(e.Expr, acc)
	case *grammar.Not:
		acc = a.leftCalls(e.Expr, acc)
	case *grammar.Atomic:
		acc = a.leftCalls(e.Expr, acc)
	case *grammar.Discard:
		acc = a.leftCalls(e.Expr, acc)
	case *grammar.Capture:
		acc = a.leftCalls(e.Expr, acc)
	case *grammar.Attributed:
		acc = a.leftCalls(e.Expr, acc)
		if !a.AlwaysSucceeds(e.Expr) {
			for _, at := range e.Attrs {
				if at.Name == "recover" {
					if skip, ok := at.Arg("skip"); ok {
						acc = a.leftCalls(skip, acc)
					}
				}
			}
		}
	case *grammar.OperatorsRule:
		if e.Skip != nil {
			acc = a.leftCalls(e.Skip, acc)
		}
		for _, o := range e.Operands {
			acc = a.leftCalls(o.Expr, acc)
			if a.AlwaysSucceeds(o.Expr) {
				break
			}
		}
		for _, l := range e.Levels {
			for _, op := range l.Operators {
				if op.Kind == grammar.Prefix {
					acc = a.leftCalls(op.Expr, acc)
				}
			}
		}
	}
	return acc
}

// AllCalls returns every rule called in the expression.
func AllCalls(e grammar.Expr, acc []string) []string {
	return allCallsControlled(e, acc, nil)
}

func allCallsControlled(e grammar.Expr, acc []string, control *cancel.Control) []string {
	walkExprControlled(e, func(x grammar.Expr) {
		control.Poll()
		if r, ok := x.(*grammar.Ref); ok {
			acc = append(acc, r.Name)
		}
	}, control)
	return acc
}

// ruleVariables returns, for each rule, the sorted names of the variables read by
// predicates and actions in the rule or in the rules it calls, directly or
// indirectly. A rule's result depends on the environment
// only through the values of these variables at the call: definitions made inside a rule are
// undone when it returns.
func ruleVariables(rules []*grammar.RuleDef) map[string][]string {
	return ruleVariablesControlled(rules, nil)
}

func ruleVariablesControlled(rules []*grammar.RuleDef, control *cancel.Control) map[string][]string {
	direct := map[string]map[string]bool{}
	for _, r := range rules {
		control.Poll()
		var set map[string]bool
		walkRuleTermsControlled(r, func(t grammar.Term) {
			control.Poll()
			if v, ok := t.(*grammar.VarRef); ok {
				if set == nil {
					set = map[string]bool{}
				}
				set[v.Name] = true
			}
		}, control)
		if len(set) > 0 {
			direct[r.Name] = set
		} else {
			delete(direct, r.Name)
		}
	}
	out := map[string][]string{}
	// With no direct variable reads there is nothing to propagate, even
	// through cycles. Avoid building a graph or visiting transitive closures.
	if len(direct) == 0 {
		return out
	}
	calls := make(map[string][]string, len(rules))
	for _, r := range rules {
		control.Poll()
		calls[r.Name] = allCallsControlled(r.Expr, nil, control)
	}
	// Tarjan emits callees before their callers. Every rule in a recursive
	// component reaches all its peers, so compute their union only once.
	// A worklist repeatedly copying evolving sets is costly when many peers
	// introduce different variables, including in long acyclic chains.
	for _, component := range Tarjan(rules, calls, control) {
		control.Poll()
		all := map[string]bool{}
		for _, name := range component {
			control.Poll()
			for v := range direct[name] {
				control.Poll()
				all[v] = true
			}
			for _, callee := range calls[name] {
				control.Poll()
				// Internal edges have no published result yet; direct peer reads
				// above already account for them. Missing rules contribute nothing.
				for _, v := range out[callee] {
					control.Poll()
					all[v] = true
				}
			}
		}
		if len(all) == 0 {
			continue
		}
		names := make([]string, 0, len(all))
		for v := range all {
			control.Poll()
			names = append(names, v)
		}
		sort.Strings(names)
		// Rule metadata only reads these sorted keys. Share the immutable
		// result within the component, rather than allocating it per peer.
		for _, name := range component {
			control.Poll()
			out[name] = names
		}
	}
	return out
}

// walkExpr visits the expression and its subexpressions in preorder.
func walkExprControlled(e grammar.Expr, f func(grammar.Expr), control *cancel.Control) {
	if control == nil {
		WalkExpr(e, f)
		return
	}
	control.Poll()
	if e == nil {
		return
	}
	f(e)
	switch e := e.(type) {
	case *grammar.Group:
		walkExprControlled(e.Expr, f, control)
	case *grammar.Seq:
		for _, it := range e.Items {
			walkExprControlled(it, f, control)
		}
	case *grammar.Choice:
		for _, alt := range e.Alts {
			walkExprControlled(alt, f, control)
		}
	case *grammar.Repeat:
		walkExprControlled(e.Expr, f, control)
	case *grammar.InlineAction:
		walkExprControlled(e.Expr, f, control)
	case *grammar.Optional:
		walkExprControlled(e.Expr, f, control)
	case *grammar.And:
		walkExprControlled(e.Expr, f, control)
	case *grammar.Not:
		walkExprControlled(e.Expr, f, control)
	case *grammar.Atomic:
		walkExprControlled(e.Expr, f, control)
	case *grammar.Stream:
		walkExprControlled(e.Expr, f, control)
	case *grammar.Recovery:
		walkExprControlled(e.Expr, f, control)
		walkExprControlled(e.Skip, f, control)
	case *grammar.Discard:
		walkExprControlled(e.Expr, f, control)
	case *grammar.Capture:
		walkExprControlled(e.Expr, f, control)
	case *grammar.Attributed:
		walkExprControlled(e.Expr, f, control)
		for _, at := range e.Attrs {
			control.Poll()
			for _, arg := range expressionArgs(control, at) {
				walkExprControlled(arg, f, control)
			}
		}
	case *grammar.OperatorsRule:
		walkExprControlled(e.Skip, f, control)
		for _, o := range e.Operands {
			walkExprControlled(o.Expr, f, control)
		}
		for _, l := range e.Levels {
			control.Poll()
			for _, op := range l.Operators {
				walkExprControlled(op.Expr, f, control)
			}
		}
	}
}

// WalkTerm visits the value expression and its subexpressions in preorder.
func WalkTerm(t grammar.Term, f func(grammar.Term)) {
	if t == nil {
		return
	}
	f(t)
	switch t := t.(type) {
	case *grammar.TermGroup:
		WalkTerm(t.Term, f)
	case *grammar.Block:
		for _, stmt := range t.Statements {
			WalkTerm(stmt, f)
		}
	case *grammar.Member:
		WalkTerm(t.X, f)
	case *grammar.New:
		for _, fi := range t.Fields {
			WalkTerm(fi.Value, f)
		}
		for _, x := range t.Args {
			WalkTerm(x, f)
		}
	case *grammar.TupleLit:
		for _, x := range t.Elems {
			WalkTerm(x, f)
		}
	case *grammar.ListConstructor:
		for _, item := range t.Items {
			WalkTerm(item, f)
		}
	case *grammar.Call:
		for _, a := range t.Args {
			WalkTerm(a, f)
		}
	case *grammar.Lambda:
		WalkTerm(t.Body, f)
	case *grammar.Binary:
		WalkTerm(t.L, f)
		WalkTerm(t.R, f)
	case *grammar.Unary:
		WalkTerm(t.X, f)
	case *grammar.Cast:
		WalkTerm(t.X, f)
	case *grammar.Bind:
		WalkTerm(t.Value, f)
	}
}

// Tarjan returns the strongly connected components of the graph of edges, starting from the
// rules in order. A component comes after every component it reaches, so callees come before
// their callers. A non-nil control polls for cancellation.
func Tarjan(rules []*grammar.RuleDef, edges map[string][]string, control *cancel.Control) [][]string {
	index := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var sccs [][]string
	next := 0
	var visit func(v string)
	visit = func(v string) {
		control.Poll()
		index[v] = next
		low[v] = next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range edges[v] {
			control.Poll()
			if _, seen := index[w]; !seen {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if onStack[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] == index[v] {
			var scc []string
			for {
				control.Poll()
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			sccs = append(sccs, scc)
		}
	}
	for _, r := range rules {
		control.Poll()
		if _, seen := index[r.Name]; !seen {
			visit(r.Name)
		}
	}
	return sccs
}

func findCycle(scc []string, edges map[string][]string, in, leaders map[string]bool, control *cancel.Control) []string {
	state := map[string]int{} // 0: unvisited, 1: in progress, 2: done
	var path []string
	var found []string
	var visit func(v string) bool
	visit = func(v string) bool {
		control.Poll()
		state[v] = 1
		path = append(path, v)
		for _, w := range edges[v] {
			control.Poll()
			if !in[w] || leaders[w] {
				continue
			}
			if state[w] == 1 {
				for i, p := range path {
					control.Poll()
					if p == w {
						found = append([]string(nil), path[i:]...)
						return true
					}
				}
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
		control.Poll()
		if !leaders[v] && state[v] == 0 && visit(v) {
			return found
		}
	}
	return nil
}

// recoveringRules returns, for each rule whose value can hold an error value that #recover made,
// a rule with such a #recover: the rule itself, or a rule that it calls, directly or indirectly.
// Recoveries and calls inside lookaheads and Operators Rule skips do not count, since their values
// do not reach the rule's value and their ends discard the recovered errors; nor does #missing,
// which a lookahead disables.
func recoveringRules(rules []*grammar.RuleDef, control *cancel.Control) map[string]string {
	witness := map[string]string{}
	calls := map[string][]string{}
	for _, r := range rules {
		control.Poll()
		direct, cs := valueRecovery(r.Expr, control)
		if direct {
			witness[r.Name] = r.Name
		}
		calls[r.Name] = cs
	}
	for changed := true; changed; {
		control.Poll()
		changed = false
		for _, r := range rules {
			control.Poll()
			if witness[r.Name] != "" {
				continue
			}
			for _, c := range calls[r.Name] {
				if w := witness[c]; w != "" {
					witness[r.Name] = w
					changed = true
					break
				}
			}
		}
	}
	return witness
}

// valueRecovery reports whether e has a #recover outside lookaheads and Operators Rule skips, and
// returns the rules that it calls outside them (recoveringRules).
func valueRecovery(e grammar.Expr, control *cancel.Control) (bool, []string) {
	silent := map[grammar.Expr]bool{}
	mark := func(x grammar.Expr) {
		walkExprControlled(x, func(y grammar.Expr) { silent[y] = true }, control)
	}
	walkExprControlled(e, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.And:
			mark(x.Expr)
		case *grammar.Not:
			mark(x.Expr)
		case *grammar.OperatorsRule:
			mark(x.Skip)
		}
	}, control)
	found := false
	var calls []string
	walkExprControlled(e, func(x grammar.Expr) {
		if silent[x] {
			return
		}
		switch x := x.(type) {
		case *grammar.Ref:
			calls = append(calls, x.Name)
		case *grammar.Recovery:
			found = true
		case *grammar.Attributed:
			found = found || hasRecover(x.Attrs)
		}
	}, control)
	return found, calls
}

func propagate(rules []*grammar.RuleDef, calls map[string][]string, set map[string]bool, control *cancel.Control) {
	for changed := true; changed; {
		control.Poll()
		changed = false
		for _, r := range rules {
			control.Poll()
			if set[r.Name] {
				continue
			}
			for _, c := range calls[r.Name] {
				control.Poll()
				if set[c] {
					set[r.Name] = true
					changed = true
					break
				}
			}
		}
	}
}

// walkRuleTermsControlled is walkRuleTerms polling control for cancellation.
func walkRuleTermsControlled(r *grammar.RuleDef, f func(grammar.Term), control *cancel.Control) {
	control.Poll()
	WalkTerm(r.Action, f)
	walkExprControlled(r.Expr, func(e grammar.Expr) {
		switch e := e.(type) {
		case *grammar.Predicate:
			WalkTerm(e.Term, f)
		case *grammar.OperatorsRule:
			for _, o := range e.Operands {
				control.Poll()
				WalkTerm(o.Action, f)
			}
			for _, l := range e.Levels {
				control.Poll()
				for _, op := range l.Operators {
					control.Poll()
					WalkTerm(op.Action, f)
				}
			}
		}
	}, control)
}

func ruleUsesPositions(r *grammar.RuleDef, control *cancel.Control) bool {
	found := false
	walkRuleTermsControlled(r, func(x grammar.Term) {
		control.Poll()
		if m, ok := x.(*grammar.Member); ok && (m.Name == "Start" || m.Name == "End") {
			found = true
		}
	}, control)
	return found
}

// AlwaysSucceeds preserves the ordinary proof and shares the invocation control.
func (a *Analysis) AlwaysSucceeds(e grammar.Expr) bool {
	if a.Control == nil {
		return grammaranalysis.AlwaysSucceeds(e, a.succeeding)
	}
	return grammaranalysis.AlwaysSucceedsWithPoll(e, a.succeeding, a.Control.Poll)
}

// WalkExpr visits the expression and its subexpressions in preorder, including the expression
// arguments of attributes and the operand and operator expressions of an Operators Rule.
func WalkExpr(e grammar.Expr, f func(grammar.Expr)) {
	if e == nil {
		return
	}
	f(e)
	switch e := e.(type) {
	case *grammar.Group:
		WalkExpr(e.Expr, f)
	case *grammar.Seq:
		for _, it := range e.Items {
			WalkExpr(it, f)
		}
	case *grammar.Choice:
		for _, alt := range e.Alts {
			WalkExpr(alt, f)
		}
	case *grammar.Repeat:
		WalkExpr(e.Expr, f)
	case *grammar.InlineAction:
		WalkExpr(e.Expr, f)
	case *grammar.Optional:
		WalkExpr(e.Expr, f)
	case *grammar.And:
		WalkExpr(e.Expr, f)
	case *grammar.Not:
		WalkExpr(e.Expr, f)
	case *grammar.Atomic:
		WalkExpr(e.Expr, f)
	case *grammar.Stream:
		WalkExpr(e.Expr, f)
	case *grammar.Recovery:
		WalkExpr(e.Expr, f)
		WalkExpr(e.Skip, f)
	case *grammar.Discard:
		WalkExpr(e.Expr, f)
	case *grammar.Capture:
		WalkExpr(e.Expr, f)
	case *grammar.Attributed:
		WalkExpr(e.Expr, f)
		for _, at := range e.Attrs {
			for _, arg := range at.ExpressionArgs() {
				WalkExpr(arg, f)
			}
		}
	case *grammar.OperatorsRule:
		WalkExpr(e.Skip, f)
		for _, o := range e.Operands {
			WalkExpr(o.Expr, f)
		}
		for _, l := range e.Levels {
			for _, op := range l.Operators {
				WalkExpr(op.Expr, f)
			}
		}
	}
}

// walkRuleTerms visits every term in a rule's action and predicates, including Operators Rule
// operand and operator actions.
func walkRuleTerms(r *grammar.RuleDef, f func(grammar.Term)) { walkRuleTermsControlled(r, f, nil) }
