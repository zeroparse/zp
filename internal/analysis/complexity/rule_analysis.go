package complexity

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// ruleAnalysis holds the rule facts that the classifier reads by rule name, taken from the facts
// of the Core IR module of the rules (classifierFacts), and answers the questions about the
// expressions of the grammar (or of a view of it) that the classifier asks with them.
type ruleAnalysis struct {
	control *cancel.Control
	// Nullable and Succeeding are the nullable and always-succeeding rules.
	Nullable, Succeeding map[string]bool
	// Leaders are the rules that break left-recursion cycles; Cyclic the rules on a cycle; Cycle
	// numbers the cycle of each cyclic rule (the same number for the rules of one cycle).
	Leaders, Cyclic map[string]bool
	Cycle           map[string]int
	// Positional are the rules whose actions or predicates, or those of a rule they call, read
	// positions.
	Positional map[string]bool
	// Recovering maps each rule whose value can hold an error value of #recover to a rule with such
	// a #recover: the rule itself or one it calls.
	Recovering map[string]string
}

func newRuleAnalysis(control *cancel.Control) *ruleAnalysis {
	return &ruleAnalysis{control: control, Nullable: map[string]bool{}, Succeeding: map[string]bool{},
		Leaders: map[string]bool{}, Cyclic: map[string]bool{}, Cycle: map[string]int{},
		Positional: map[string]bool{}, Recovering: map[string]string{}}
}

func (a *ruleAnalysis) isNullable(e ast.Expr) bool {
	a.control.Poll()
	switch e := e.(type) {
	case *ast.Ref:
		return a.Nullable[e.Name]
	case *ast.Literal:
		return e.Value == ""
	case *ast.CharClass, *ast.Any, *ast.Bottom:
		return false
	case *ast.Seq:
		for _, it := range e.Items {
			if !a.isNullable(it) {
				return false
			}
		}
		return true
	case *ast.Choice:
		for _, alt := range e.Alts {
			if a.isNullable(alt) {
				return true
			}
		}
		return false
	case *ast.Repeat:
		return e.Min == 0 || e.Min == 1 && a.isNullable(e.Expr)
	case *ast.Atomic:
		return a.isNullable(e.Expr)
	case *ast.Discard:
		return a.isNullable(e.Expr)
	case *ast.Capture:
		return a.isNullable(e.Expr)
	case *ast.Attributed:
		return a.isNullable(e.Expr)
	case *ast.OperatorsRule:
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

// LeftCalls returns the rules that may be called before any input is consumed.
func (a *ruleAnalysis) LeftCalls(e ast.Expr, acc []string) []string {
	a.control.Poll()
	switch e := e.(type) {
	case *ast.Ref:
		return append(acc, e.Name)
	case *ast.Seq:
		for _, it := range e.Items {
			acc = a.LeftCalls(it, acc)
			if !a.isNullable(it) {
				break
			}
		}
	case *ast.Choice:
		for _, alt := range e.Alts {
			acc = a.LeftCalls(alt, acc)
			if a.AlwaysSucceeds(alt) {
				break
			}
		}
	case *ast.Repeat:
		if e.Max != 0 {
			acc = a.LeftCalls(e.Expr, acc)
		}
	case *ast.Optional:
		acc = a.LeftCalls(e.Expr, acc)
	case *ast.And:
		acc = a.LeftCalls(e.Expr, acc)
	case *ast.Not:
		acc = a.LeftCalls(e.Expr, acc)
	case *ast.Atomic:
		acc = a.LeftCalls(e.Expr, acc)
	case *ast.Discard:
		acc = a.LeftCalls(e.Expr, acc)
	case *ast.Capture:
		acc = a.LeftCalls(e.Expr, acc)
	case *ast.Attributed:
		acc = a.LeftCalls(e.Expr, acc)
		if !a.AlwaysSucceeds(e.Expr) {
			for _, at := range e.Attrs {
				if at.Name == "recover" {
					if skip, ok := at.Arg("skip"); ok {
						acc = a.LeftCalls(skip, acc)
					}
				}
			}
		}
	case *ast.OperatorsRule:
		if e.Skip != nil {
			acc = a.LeftCalls(e.Skip, acc)
		}
		for _, o := range e.Operands {
			acc = a.LeftCalls(o.Expr, acc)
			if a.AlwaysSucceeds(o.Expr) {
				break
			}
		}
		for _, l := range e.Levels {
			for _, op := range l.Operators {
				if op.Kind == ast.Prefix {
					acc = a.LeftCalls(op.Expr, acc)
				}
			}
		}
	}
	return acc
}

// AlwaysSucceeds reports a proof that e cannot fail, with the always-succeeding rules.
func (a *ruleAnalysis) AlwaysSucceeds(e ast.Expr) bool {
	if a.control == nil {
		return astfacts.AlwaysSucceeds(e, a.Succeeding)
	}
	return astfacts.AlwaysSucceedsWithPoll(e, a.Succeeding, a.control.Poll)
}

// tarjan returns the strongly connected components of the graph of edges, starting from the rules
// in order. A component comes after every component it reaches, so callees come before their
// callers. A non-nil control polls for cancellation.
func tarjan(rules []*ast.RuleDef, edges map[string][]string, control *cancel.Control) [][]string {
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
