package complexity

import "zeroparse.dev/zp/internal/ir/core"

// The rules that drop an evaluation of the module (N3, N9, N11, N12) drop only one that the
// grammar either never makes or makes again in a state in which it made it before, and only when
// that evaluation costs again at most a constant factor of what it cost first
// (evaluatedAgainCheaply): the view keeps the first evaluation, so it keeps the cost of the
// grammar within a constant factor, and a dropped evaluation cannot compound through recursion.

// cheapRules computes the rules whose calls cost, when they are evaluated again in a state in
// which they were evaluated before, at most what the first evaluation cost: the rules that every
// parse memoizes, which the memo answers the second time, and the rules without an action whose
// bodies are evaluated again cheaply and call only such rules. It is the least fixed point, so a
// recursion through rules that are not memoized is never cheap: its evaluations again could
// compound. A rule of a left-recursion cycle is never cheap: a call of it can read the seed of a
// growth, which the classifier analyzes as the grammar is written.
func (b *builder) cheapRules() {
	rules := b.src.Facts.Rules
	b.cheap = make([]bool, len(rules))
	for i := range rules {
		// Every parse memoizes a rule that is neither transient nor on a path to a #missing
		// insertion; a member of a cycle is memoized only outside its growths.
		b.cheap[i] = !rules[i].Cyclic() && !rules[i].Transient && !rules[i].Missing
	}
	for changed := true; changed; {
		changed = false
		for i, r := range b.src.Module.Rules {
			b.control.Poll()
			if !b.cheap[i] && !rules[i].Cyclic() && r.Finish.Kind != core.FinishAction && b.evaluatedAgainCheaply(r.Body) {
				b.cheap[i], changed = true, true
			}
		}
	}
}

// evaluatedAgainCheaply reports whether an evaluation of n made again in a state in which it was
// made before costs at most a constant factor of the first, whatever the input: every rule that n
// calls is cheap (cheapRules), and n has nothing else whose cost the runtime does not bound by the
// input that n examines (a predicate or a binding, which can build values, a recovery, a stream,
// an Operators Rule, a #missing or an inlined call).
func (b *builder) evaluatedAgainCheaply(n core.Node) bool {
	ok := true
	core.Walk(n, func(x core.Node) {
		switch x := x.(type) {
		case *core.Call:
			ok = ok && x.Level == 0 && b.cheap[x.Rule]
		case *core.Pred, *core.BindNode, *core.Recover, *core.Stream, *core.OperatorsRule, *core.Missing, *core.Nest:
			ok = false
		}
	})
	return ok
}

// boundedTerminal reports whether n examines at most a constant amount of input and calls no
// rule: dropping its evaluations drops a constant cost.
func boundedTerminal(n core.Node) bool {
	ok := true
	core.Walk(n, func(x core.Node) {
		switch x := x.(type) {
		case *core.Lit, *core.ClassNode, *core.Any, *core.Seq, *core.Choice:
		case *core.Repeat:
			ok = ok && x.Max >= 0
		default:
			ok = false
		}
	})
	return ok
}
