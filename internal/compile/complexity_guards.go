package compile

import (
	"math"
	"slices"

	"zeroparse.dev/zp/grammar"
)

// guard is what the predicates next to a binding of an int variable test of the value that it
// binds. A failed test fails the sequence that holds the binding, and backtracking undoes the
// binding, so a value that the guard rejects reaches no call and no later binding.
type guard struct {
	// tests are the tests after the binding that read only its variable and literals.
	tests []grammar.Term
	// bounds holds every value that passes the tests after the binding and the tests before it.
	bounds valueRange
}

// passes reports whether the value y of the variable v can pass the guard: it is within the
// bounds and no test is false or an error on it.
func (g guard) passes(v string, y int) bool {
	if y < g.bounds.lo || y > g.bounds.hi {
		return false
	}
	for _, t := range g.tests {
		r, res := eval(t, domainEnv{v: v, x: y})
		if b, isBool := r.(bool); res == evalFailed || res == evalOK && isBool && !b {
			return false
		}
	}
	return true
}

// guardOf returns the guard of the binding b of an int variable v := e. Between the binding and
// the tests after it, and between the tests before it and the binding, only predicates run (see
// adjacentStatements), so nothing reads the input or calls a rule there.
//
//   - A test after the binding, up to the next binding of v, that reads only v and literals
//     tests the value that b binds: `{v := e} {lo <= v && v <= hi}`. Its comparisons of v with
//     constants bound the value.
//   - A test before the binding, after the last binding of any variable before it, evaluates e
//     with the same variables and captures as b does, and every term of the action language is a
//     function of them: `{lo <= e && e <= hi} {v := e}`. Its comparisons of e, as written, with
//     constants bound the value.
//
// A test of a conjunction bounds the value by each of its comparisons that has the form; a
// single comparison bounds one side, which the range of e (see termRange) can bound on the other.
func guardOf(b *binding) guard {
	g := guard{bounds: fullRange}
	v, e := b.b.Name, ungroupTerm(b.b.Value)
	isVar := func(t grammar.Term) bool {
		r, ok := ungroupTerm(t).(*grammar.VarRef)
		return ok && r.Name == v
	}
	text := grammar.FormatTerm(e)
	isValue := func(t grammar.Term) bool { return grammar.FormatTerm(ungroupTerm(t)) == text }
	after, before := adjacentStatements(b)
	for _, s := range after {
		if bd, ok := s.(*grammar.Bind); ok && bd.Name == v {
			break
		}
		if readsOnly(s, v, nil) {
			g.tests = append(g.tests, s)
			g.bounds = g.bounds.meet(termBounds(s, isVar))
		}
	}
	for _, s := range before {
		if _, ok := s.(*grammar.Bind); ok {
			break
		}
		g.bounds = g.bounds.meet(termBounds(s, isValue))
	}
	return g
}

// adjacentStatements returns the statements of the predicates that run right after the binding b
// and right before it, each list in the order of evaluation away from b: the other statements of
// its block, then those of the predicates next to the predicate of b in the sequence that holds
// it, up to the first element on either side that is not a predicate. A predicate with attributes
// counts when they are all @ attributes, which do not change matching.
func adjacentStatements(b *binding) (after, before []grammar.Term) {
	if b.p == nil {
		return nil, nil
	}
	own := statements(b.p.Term)
	i := slices.Index(own, grammar.Term(b.b))
	if i < 0 {
		return nil, nil
	}
	after = slices.Clone(own[i+1:])
	for j := i - 1; j >= 0; j-- {
		before = append(before, own[j])
	}
	if len(b.c.seqs) == 0 {
		return after, before
	}
	f := b.c.seqs[len(b.c.seqs)-1]
	if predicateOf(f.seq.Items[f.index]) != b.p {
		return after, before
	}
	for _, it := range f.seq.Items[f.index+1:] {
		q := predicateOf(it)
		if q == nil {
			break
		}
		after = append(after, statements(q.Term)...)
	}
	for j := f.index - 1; j >= 0; j-- {
		q := predicateOf(f.seq.Items[j])
		if q == nil {
			break
		}
		s := statements(q.Term)
		for m := len(s) - 1; m >= 0; m-- {
			before = append(before, s[m])
		}
	}
	return after, before
}

// predicateOf returns the predicate that e is, through @ attributes, or nil.
func predicateOf(e grammar.Expr) *grammar.Predicate {
	for {
		switch x := e.(type) {
		case *grammar.Predicate:
			return x
		case *grammar.Attributed:
			if !labelsOnly(x) {
				return nil
			}
			e = x.Expr
		default:
			return nil
		}
	}
}

// labelsOnly reports whether every attribute of e is an @ attribute, as @error and @diagnostic
// are: they change diagnostics, not matching, backtracking or bindings.
func labelsOnly(e *grammar.Attributed) bool {
	for _, a := range e.Attrs {
		if a.Pragma {
			return false
		}
	}
	return true
}

// statements returns the statements of a predicate term: those of a block, or the term itself.
func statements(t grammar.Term) []grammar.Term {
	if blk, ok := t.(*grammar.Block); ok {
		return blk.Statements
	}
	return []grammar.Term{t}
}

// valueRange is an inclusive range of ints. A bound beyond maxExact in magnitude stands for no
// bound on that side; an empty range has hi < lo.
type valueRange struct{ lo, hi int }

var fullRange = valueRange{-maxExact - 1, maxExact + 1}

func (r valueRange) meet(s valueRange) valueRange {
	return valueRange{max(r.lo, s.lo), min(r.hi, s.hi)}
}

func (r valueRange) bounded() bool { return r.lo >= -maxExact && r.hi <= maxExact }

// size returns the number of values in r, or math.MaxInt when r is not bounded.
func (r valueRange) size() int {
	switch {
	case r.hi < r.lo:
		return 0
	case !r.bounded():
		return math.MaxInt
	}
	return r.hi - r.lo + 1
}

// rangeOf returns the range from lo to hi, with a bound beyond maxExact in magnitude dropped.
func rangeOf(lo, hi int) valueRange {
	if lo < -maxExact {
		lo = fullRange.lo
	}
	if hi > maxExact {
		hi = fullRange.hi
	}
	return valueRange{lo, hi}
}

// termBounds returns the range that t bounds the subject to when t is true: t is a comparison
// with <, <=, >, >= or == of the subject with a constant int (see constInt), on either side, or a
// conjunction with && of such tests and others, which do not bound it.
func termBounds(t grammar.Term, subject func(grammar.Term) bool) valueRange {
	bin, ok := ungroupTerm(t).(*grammar.Binary)
	if !ok {
		return fullRange
	}
	if bin.Op == "&&" {
		return termBounds(bin.L, subject).meet(termBounds(bin.R, subject))
	}
	op, c := bin.Op, 0
	if n, isConst := constInt(bin.R); isConst && subject(bin.L) {
		c = n
	} else if n, isConst := constInt(bin.L); isConst && subject(bin.R) {
		c = n
		op = map[string]string{"<": ">", "<=": ">=", ">": "<", ">=": "<=", "==": "=="}[op]
	} else {
		return fullRange
	}
	switch op {
	case "<":
		return rangeOf(fullRange.lo, c-1)
	case "<=":
		return rangeOf(fullRange.lo, c)
	case ">":
		return rangeOf(c+1, fullRange.hi)
	case ">=":
		return rangeOf(c, fullRange.hi)
	case "==":
		return valueRange{c, c}
	}
	return fullRange
}

// constInt returns the value of t when t is a constant int expression that eval knows.
func constInt(t grammar.Term) (int, bool) {
	if !constant(t) {
		return 0, false
	}
	r, res := eval(t, domainEnv{})
	n, isInt := r.(int)
	return n, res == evalOK && isInt
}

// termRange returns a range that holds every int that t can give when the variable v holds a
// value of xs: a literal is its value, v ranges over xs, len gives at least 0, unary -, + and *
// combine the ranges of their operands, and any other term has no bound.
func termRange(t grammar.Term, v string, xs valueSet) valueRange {
	switch t := t.(type) {
	case *grammar.TermGroup:
		return termRange(t.Term, v, xs)
	case *grammar.IntLit:
		if n, ok := constInt(t); ok {
			return valueRange{n, n}
		}
	case *grammar.VarRef:
		if t.Name == v && len(xs) > 0 {
			lo, hi := math.MaxInt, math.MinInt
			for x := range xs {
				lo, hi = min(lo, x), max(hi, x)
			}
			return valueRange{lo, hi}
		}
	case *grammar.Call:
		if t.Func == "len" && len(t.Args) == 1 {
			return valueRange{0, fullRange.hi}
		}
	case *grammar.Unary:
		if t.Op == "-" {
			r := termRange(t.X, v, xs)
			return valueRange{-r.hi, -r.lo}
		}
	case *grammar.Binary:
		l, r := termRange(t.L, v, xs), termRange(t.R, v, xs)
		switch t.Op {
		case "+":
			return addRanges(l, r)
		case "-":
			return addRanges(l, valueRange{-r.hi, -r.lo})
		case "*":
			return mulRanges(l, r)
		}
	}
	return fullRange
}

// addRanges returns the range of the sums of the values of l and r.
func addRanges(l, r valueRange) valueRange {
	out := fullRange
	if l.lo >= -maxExact && r.lo >= -maxExact {
		out.lo = rangeOf(l.lo+r.lo, 0).lo
	}
	if l.hi <= maxExact && r.hi <= maxExact {
		out.hi = rangeOf(0, l.hi+r.hi).hi
	}
	return out
}

// mulRanges returns the range of the products of the values of l and r, when both are bounded.
func mulRanges(l, r valueRange) valueRange {
	if !l.bounded() || !r.bounded() || l.size() == 0 || r.size() == 0 {
		return fullRange
	}
	lo, hi := math.MaxInt, math.MinInt
	for _, a := range []int{l.lo, l.hi} {
		for _, b := range []int{r.lo, r.hi} {
			if a != 0 && abs(b) > maxExact/abs(a) {
				return fullRange
			}
			lo, hi = min(lo, a*b), max(hi, a*b)
		}
	}
	return rangeOf(lo, hi)
}
