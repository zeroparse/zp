package complexity

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/internal/frontend/check"
)

// The classifier's model of the sites that the compiler numbers on Core IR
// (internal/middleend/corepass, ComputeSites), on the expressions of a grammar or a view of it.

// restartableRepeats returns the repetitions that an iteration of a repetition can reach: those in
// the element of another repetition that iterates (its maximum is not 0 or 1), and those in a rule
// that such an element calls, or that calls itself, directly or indirectly. Every other repetition is reached by a
// derivation once, from the start rule through sequences, choices and calls, so one walk of it
// starts at each position at most once per environment, and a record could not be reused.
func restartableRepeats(names []string, exprOf func(string) grammar.Expr, calls map[string][]string) map[*grammar.Repeat]bool {
	out := map[*grammar.Repeat]bool{}
	iterated := map[string]bool{}
	var queue []string
	mark := func(n string) {
		if !iterated[n] {
			iterated[n] = true
			queue = append(queue, n)
		}
	}
	for _, n := range names {
		astfacts.WalkExpr(exprOf(n), func(x grammar.Expr) {
			r, ok := x.(*grammar.Repeat)
			if !ok || r.Max == 0 || r.Max == 1 {
				return
			}
			astfacts.WalkExpr(r.Expr, func(y grammar.Expr) {
				switch y := y.(type) {
				case *grammar.Repeat:
					out[y] = true
				case *grammar.Ref:
					mark(y.Name)
				}
			})
		})
	}
	// A rule that calls itself, directly or through others, repeats as an iteration does: a right
	// recursion walks the repetitions in its rules from each position its alternatives advance to.
	for _, n := range names {
		seen := map[string]bool{}
		stack := append([]string(nil), calls[n]...)
		for len(stack) > 0 && !iterated[n] {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if c == n {
				mark(n)
			} else if !seen[c] {
				seen[c] = true
				stack = append(stack, calls[c]...)
			}
		}
	}
	for len(queue) > 0 {
		n := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, c := range calls[n] {
			mark(c)
		}
	}
	for _, n := range names {
		if !iterated[n] {
			continue
		}
		astfacts.WalkExpr(exprOf(n), func(x grammar.Expr) {
			if r, ok := x.(*grammar.Repeat); ok {
				out[r] = true
			}
		})
	}
	return out
}

// isScan reports whether e repeats a single character, which a scan walks without a record.
func isScan(e *grammar.Repeat) bool {
	switch e.Expr.(type) {
	case *grammar.CharClass, *grammar.Any:
		return true
	}
	return false
}

// blocksRunAttrs reports whether the attributes of x keep a repetition that holds it from being a
// run site: #stream, #missing, #error and #diagnostic.
func blocksRunAttrs(x *grammar.Attributed) bool {
	for _, a := range x.Attrs {
		switch a.Name {
		case "stream", "missing", "error", "diagnostic":
			return true
		}
	}
	return false
}

// scanSiteRepeats calls f for each repetition without an upper bound in e, the body of a rule that
// calls no rule, in order, with whether it is a scan site: it repeats a character class or any
// character and does not deliver the elements of a #stream. A repetition in a #recover skip is
// reported as a site; it is one unless it is a skip-run site, which records its walks as one.
func scanSiteRepeats(e grammar.Expr, control *cancel.Control, f func(rep *grammar.Repeat, site bool)) {
	streamed := map[*grammar.Repeat]bool{}
	astfacts.WalkExprWithPoll(e, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Attributed:
			if rep, ok := x.Expr.(*grammar.Repeat); ok && check.IsStream(x) {
				streamed[rep] = true
			}
		case *grammar.Stream:
			if rep, ok := x.Expr.(*grammar.Repeat); ok {
				streamed[rep] = true
			}
		case *grammar.Repeat:
			if x.Max < 0 {
				f(x, isScan(x) && !streamed[x])
			}
		}
	}, control.Poll)
}
