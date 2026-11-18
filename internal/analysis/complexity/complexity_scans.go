package complexity

import (
	"fmt"
	"unicode/utf8"
	"zeroparse.dev/zp/internal/compile"

	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Bounded scans (see "Conditions for Linear Cost" in spec/runtime/parsing.md and Lemma 13 of
// spec/proofs/linear-cost.md).
//
// Every iteration of a repetition consumes characters of its alphabet, so the repetition reaches a
// position q only from a start inside the run of the alphabet that ends at q. The run-length
// analysis bounds how many characters of the alphabet can stand right before an evaluation of the
// repetition, which confines those starts to a window at the beginning of the run. With a constant
// number of evaluations at each start (countedWalk), the iterations at q number a constant, whether
// or not the units around the repetition commit. Memoization alone gives no such bound: it counts
// the evaluations at one position, and a run has many positions.

// runMax is the longest run before a repetition that the analysis tells from a run without a bound.
const runMax = 32

const (
	runNone  = -2         // no value: the path does not arrive
	runClear = -1         // the next character is not in the alphabet, or there is none
	runOpen  = runMax + 1 // no bound on the run
)

// runFlow is a transfer function of the run length: from a run of d characters of the alphabet
// before an expression, the run after it is at most max(a, d+b); a is runNone for no constant part,
// and b is runNone for no part that depends on d.
type runFlow struct{ a, b int }

var (
	runID     = runFlow{runNone, 0}
	runBottom = runFlow{runNone, runNone}
)

func runSat(d int) int {
	if d >= runOpen {
		return runOpen
	}
	return d
}

func (f runFlow) apply(d int) int {
	if d == runNone {
		return f.a
	}
	if f.b == runNone {
		return f.a
	}
	if f.b >= runOpen {
		return runOpen
	}
	return max(f.a, runSat(d+f.b))
}

// then returns f followed by g.
func (f runFlow) then(g runFlow) runFlow {
	out := runFlow{g.a, runNone}
	if f.a != runNone {
		out.a = max(out.a, g.apply(f.a))
	}
	if f.b != runNone && g.b != runNone {
		out.b = runSat(f.b + g.b)
	}
	return out
}

func (f runFlow) join(g runFlow) runFlow { return runFlow{max(f.a, g.a), max(f.b, g.b)} }

// star returns f applied any number of times, none included.
func (f runFlow) star() runFlow {
	out := runID
	for {
		next := out.join(out.then(f))
		if next == out {
			return out
		}
		out = next
	}
}

// runAnalysis is the run-length analysis of one alphabet.
type runAnalysis struct {
	k     *classifier
	x     charset
	rule  map[string]runFlow
	exprs map[ast.Expr]runFlow
	fixed map[ast.Expr]runFlow // the transfer functions that depend on no rule
	sure  map[ast.Expr]bool    // what clears has found
	pre   map[string]int
	at    map[*ast.Repeat]int // the longest run before each repetition
}

func (k *classifier) subset(a, b charset) bool {
	return !a.intersects(b.complement())
}

// charFlow is the transfer function of consuming one character of s.
func (r *runAnalysis) charFlow(s charset) runFlow {
	if !s.intersects(r.x) {
		return runFlow{0, runNone}
	}
	if r.k.subset(s, r.x) {
		return runFlow{runNone, 1}
	}
	return runFlow{0, 1}
}

func (r *runAnalysis) flow(e ast.Expr) runFlow {
	if e == nil {
		return runID
	}
	if f, ok := r.exprs[e]; ok {
		return f
	}
	if f, ok := r.fixed[e]; ok {
		return f
	}
	f := r.compute(e)
	switch e.(type) {
	case *ast.Literal, *ast.CharClass, *ast.Any, *ast.And, *ast.Not:
		r.fixed[e] = f
	default:
		r.exprs[e] = f
	}
	return f
}

// clears reports whether the next character is outside the alphabet where e fails to consume one.
func (r *runAnalysis) clears(e ast.Expr) bool {
	v, ok := r.sure[e]
	if !ok {
		v = r.k.subset(r.x, r.k.sureOf(e))
		r.sure[e] = v
	}
	return v
}

func (r *runAnalysis) compute(e ast.Expr) runFlow {
	k := r.k
	k.control.Poll()
	switch e := e.(type) {
	case nil:
		return runID
	case *ast.Ref:
		if k.byName[e.Name] == nil {
			return runID
		}
		return r.rule[e.Name]
	case *ast.Literal:
		f := runID
		for s := e.Value; s != ""; {
			c, n := utf8.DecodeRuneInString(s)
			s = s[n:]
			cs := charOf(c)
			if e.IgnoreCase {
				cs = asciiFold(c)
			}
			f = f.then(r.charFlow(cs))
		}
		return f
	case *ast.CharClass:
		return r.charFlow(classChars(e))
	case *ast.Any:
		return r.charFlow(allChars)
	case *ast.Seq:
		f := runID
		for _, it := range e.Items {
			f = f.then(r.flow(it))
		}
		return f
	case *ast.Choice:
		f := runBottom
		for _, alt := range e.Alts {
			f = f.join(r.flow(alt))
		}
		return f
	case *ast.Repeat:
		if e.Max == 0 {
			return runID
		}
		body := r.flow(e.Expr)
		if e.Max < 0 {
			if r.clears(e.Expr) {
				return runFlow{runClear, runNone}
			}
			return body.star()
		}
		if e.Max <= runMax {
			f, step := runID, runID
			for range e.Max {
				step = step.then(body)
				f = f.join(step)
			}
			return f
		}
		return body.star()
	case *ast.Optional:
		return runID.join(r.flow(e.Expr))
	case *ast.And:
		if !k.nullable(e.Expr) && !k.first(e.Expr).intersects(r.x) {
			return runFlow{runClear, runNone}
		}
		return runID
	case *ast.Not:
		if r.clears(e.Expr) {
			return runFlow{runClear, runNone}
		}
		return runID
	case *ast.Group:
		return r.flow(e.Expr)
	case *ast.Capture:
		return r.flow(e.Expr)
	case *ast.Discard:
		return r.flow(e.Expr)
	case *ast.Atomic:
		return r.flow(e.Expr)
	case *ast.Attributed:
		f := r.flow(e.Expr)
		if skip := compile.RecoverSkip(e); skip != nil {
			f = f.join(r.flow(skip))
		}
		if compile.HasPragma(e, "missing") {
			f = f.join(runID)
		}
		return f
	case *ast.OperatorsRule:
		return r.operators(e).star()
	case *ast.EndInput:
		return runFlow{runClear, runNone}
	case *ast.Bottom:
		return runBottom
	}
	// Predicates, anchors, cuts and the top expression consume nothing.
	return runID
}

// operators is the transfer function of one step of an Operators Rule: its skip, an operand or an
// operator part. Steps follow one another in any order, which over-approximates the order that the
// rule evaluates them in.
func (r *runAnalysis) operators(e *ast.OperatorsRule) runFlow {
	f := runID
	if e.Skip != nil {
		f = f.join(r.flow(e.Skip))
	}
	for _, o := range e.Operands {
		f = f.join(r.flow(o.Expr))
	}
	for _, l := range e.Levels {
		for _, op := range l.Operators {
			f = f.join(r.flow(op.Expr))
		}
	}
	return f
}

// solve computes the transfer function of every rule, then the longest run before every rule and
// repetition.
func (r *runAnalysis) solve() {
	k := r.k
	for _, d := range k.rules {
		r.rule[d.Name] = runBottom
	}
	for changed := true; changed; {
		changed = false
		clear(r.exprs)
		for _, d := range k.rules {
			if !k.reach[d.Name] {
				continue
			}
			if f := r.rule[d.Name].join(r.flow(d.Expr)); f != r.rule[d.Name] {
				r.rule[d.Name] = f
				changed = true
			}
		}
	}
	clear(r.exprs)
	r.pre[k.start] = 0
	for changed := true; changed; {
		changed = false
		for _, d := range k.rules {
			if in, ok := r.pre[d.Name]; ok && k.reach[d.Name] {
				r.visit(d.Expr, in, &changed)
			}
		}
	}
}

// visit carries the run before e into e, recording it at calls and repetitions.
func (r *runAnalysis) visit(e ast.Expr, in int, changed *bool) {
	k := r.k
	k.control.Poll()
	switch e := e.(type) {
	case *ast.Ref:
		if k.byName[e.Name] == nil {
			return
		}
		if old, ok := r.pre[e.Name]; !ok || in > old {
			r.pre[e.Name] = in
			*changed = true
		}
	case *ast.Seq:
		for _, it := range e.Items {
			r.visit(it, in, changed)
			in = r.flow(it).apply(in)
		}
	case *ast.Choice:
		for _, alt := range e.Alts {
			r.visit(alt, in, changed)
		}
	case *ast.Repeat:
		if e.Max == 0 {
			return
		}
		if old, ok := r.at[e]; !ok || in > old {
			r.at[e] = in
			*changed = true
		}
		r.visit(e.Expr, r.flow(e.Expr).star().apply(in), changed)
	case *ast.Optional:
		r.visit(e.Expr, in, changed)
	case *ast.And:
		r.visit(e.Expr, in, changed)
	case *ast.Not:
		r.visit(e.Expr, in, changed)
	case *ast.Group:
		r.visit(e.Expr, in, changed)
	case *ast.Capture:
		r.visit(e.Expr, in, changed)
	case *ast.Discard:
		r.visit(e.Expr, in, changed)
	case *ast.Atomic:
		r.visit(e.Expr, in, changed)
	case *ast.Attributed:
		r.visit(e.Expr, in, changed)
		if skip := compile.RecoverSkip(e); skip != nil {
			r.visit(skip, in, changed)
		}
	case *ast.OperatorsRule:
		in = r.operators(e).star().apply(in)
		if e.Skip != nil {
			r.visit(e.Skip, in, changed)
		}
		for _, o := range e.Operands {
			r.visit(o.Expr, in, changed)
		}
		for _, l := range e.Levels {
			for _, op := range l.Operators {
				r.visit(op.Expr, in, changed)
			}
		}
	}
}

// sureOf returns what sure returns for e, once for each expression: the analysis of each alphabet
// asks again in each pass.
func (k *classifier) sureOf(e ast.Expr) charset {
	s, ok := k.sureMemo[e]
	if !ok {
		s = k.sure(e)
		k.sureMemo[e] = s
	}
	return s
}

// looseCut reports whether e has a cut that commits the choice, option or repetition around it.
func (k *classifier) looseCut(e ast.Expr) bool {
	return astfacts.HasLooseCutWithPoll(e, k.control.Poll)
}

// sure returns characters on which e surely consumes: an evaluation of e where the next character
// is in the set succeeds and consumes it or more, whatever else the input holds. It
// under-approximates. A failure of a part reaches further than the part in two ways, and both end
// the proof: a part that fails after a cut fails the choice, option or repetition around it, and a
// call of a rule on a left-recursion cycle can read the failing seed of a growth. Each rule is
// followed once; a call that returns to a rule being followed answers nothing, which the
// under-approximation allows. Variables reach matching only through predicates, which sure
// answers nothing for and AlwaysSucceeds does not prove, so the answer holds whatever the variables
// that a rule reads hold, as it does for the input.
func (k *classifier) sure(e ast.Expr) charset {
	k.control.Poll()
	switch e := e.(type) {
	case *ast.CharClass:
		return classChars(e)
	case *ast.Any:
		return allChars
	case *ast.Literal:
		if utf8.RuneCountInString(e.Value) == 1 {
			return literalFirst(e)
		}
	case *ast.Group:
		return k.sure(e.Expr)
	case *ast.Capture:
		return k.sure(e.Expr)
	case *ast.Discard:
		return k.sure(e.Expr)
	case *ast.Atomic:
		return k.sure(e.Expr)
	case *ast.Choice:
		var s charset
		for _, alt := range e.Alts {
			if k.nullable(alt) {
				break
			}
			s = s.union(k.sure(alt))
			if k.looseCut(alt) {
				break
			}
		}
		return s
	case *ast.Repeat:
		if e.Min == 1 && e.Max != 0 {
			return k.sure(e.Expr)
		}
	case *ast.Ref:
		if e.Level != "" || k.byName[e.Name] == nil || k.an.Cyclic[e.Name] {
			return nil
		}
		s, ok := k.sureRule[e.Name]
		if !ok {
			k.sureRule[e.Name] = nil
			s = k.sure(k.byName[e.Name].Expr)
			k.sureRule[e.Name] = s
		}
		return s
	case *ast.Seq:
		var away charset // first characters of what the sequence tries before the part that decides
		for i, it := range e.Items {
			switch x := ungroup(it).(type) {
			case *ast.Optional:
				if k.looseCut(x.Expr) {
					return nil
				}
				away = away.union(k.first(x.Expr))
				continue
			case *ast.Repeat:
				if x.Min == 0 {
					if k.looseCut(x.Expr) {
						return nil
					}
					away = away.union(k.first(x.Expr))
					continue
				}
			case *ast.Not:
				away = away.union(k.first(x.Expr))
				if k.nullable(x.Expr) {
					return nil
				}
				continue
			}
			for _, later := range e.Items[i+1:] {
				if !k.an.AlwaysSucceeds(later) {
					return nil
				}
			}
			s := k.sure(it)
			return intersect(s, away.complement())
		}
	}
	return nil
}

// intersect returns the characters of both sets.
func intersect(a, b charset) charset {
	return a.complement().union(b.complement()).complement()
}

// consumed returns the characters that an evaluation of e can consume, through calls, outside
// lookaheads.
func (k *classifier) consumed(e ast.Expr, rules map[string]charset) charset {
	var s charset
	var walk func(e ast.Expr)
	walk = func(e ast.Expr) {
		switch e := e.(type) {
		case *ast.Ref:
			s = s.union(rules[e.Name])
		case *ast.Literal:
			for _, c := range e.Value {
				if e.IgnoreCase {
					s = s.union(asciiFold(c))
				} else {
					s = s.union(charOf(c))
				}
			}
		case *ast.CharClass:
			s = s.union(classChars(e))
		case *ast.Any:
			s = allChars
		case *ast.Seq:
			for _, it := range e.Items {
				walk(it)
			}
		case *ast.Choice:
			for _, alt := range e.Alts {
				walk(alt)
			}
		case *ast.Repeat:
			if e.Max != 0 {
				walk(e.Expr)
			}
		case *ast.Optional:
			walk(e.Expr)
		case *ast.Group:
			walk(e.Expr)
		case *ast.Capture:
			walk(e.Expr)
		case *ast.Discard:
			walk(e.Expr)
		case *ast.Atomic:
			walk(e.Expr)
		case *ast.Attributed:
			walk(e.Expr)
			if skip := compile.RecoverSkip(e); skip != nil {
				walk(skip)
			}
		case *ast.OperatorsRule:
			if e.Skip != nil {
				walk(e.Skip)
			}
			for _, o := range e.Operands {
				walk(o.Expr)
			}
			for _, l := range e.Levels {
				for _, op := range l.Operators {
					walk(op.Expr)
				}
			}
		}
	}
	walk(e)
	return s
}

// computeBoundedScans finds the repetitions that are bounded scans and takes them out of the
// scanning sources that the conditions check. Lemma 13 counts their iterations wherever they are
// evaluated, so a check that takes one for an expression without a scanning source leaves out a
// cost that is counted already.
func (k *classifier) computeBoundedScans() {
	alphabet := map[string]charset{}
	for changed := true; changed; {
		changed = false
		for _, r := range k.rules {
			k.control.Poll()
			if !k.reach[r.Name] {
				continue
			}
			if s := k.consumed(r.Expr, alphabet).union(alphabet[r.Name]); !s.equal(alphabet[r.Name]) {
				alphabet[r.Name] = s
				changed = true
			}
		}
	}
	k.sureMemo = map[ast.Expr]charset{}
	k.sureRule = map[string]charset{}
	runs := map[string]*runAnalysis{}
	k.scanRun = map[*ast.Repeat]int{}
	for e := range k.srcExprs {
		rep, ok := e.(*ast.Repeat)
		if !ok || k.again[rep] == nil {
			continue
		}
		x := k.consumed(rep.Expr, alphabet)
		key := fmt.Sprint(x)
		r := runs[key]
		if r == nil {
			r = &runAnalysis{k: k, x: x, rule: map[string]runFlow{}, exprs: map[ast.Expr]runFlow{},
				fixed: map[ast.Expr]runFlow{}, sure: map[ast.Expr]bool{}, pre: map[string]int{}, at: map[*ast.Repeat]int{}}
			r.solve()
			runs[key] = r
		}
		if d, ok := r.at[rep]; ok {
			k.scanRun[rep] = d
		}
	}
	k.regionScan = k.regionScans()
	// counted are the rules with a constant number of evaluations at one position: the memoized
	// rules off left-recursion cycles, by Lemma 3, and the rules that only such evaluations reach
	// at a bounded distance, which countedWalk clears the others from. The rules that are not
	// memoized must not be recursive, so that the counts of their callers settle theirs.
	counted := map[string]bool{}
	for _, r := range k.rules {
		if k.reach[r.Name] && !k.an.Cyclic[r.Name] && (k.memo[r.Name] || !k.recursive[r.Name]) {
			counted[r.Name] = true
		}
	}
	k.rigidRule = map[string]bool{}
	k.scanCounted = map[*ast.Repeat]bool{}
	for changed := true; changed; {
		changed = false
		clear(k.scanCounted)
		for _, r := range k.rules {
			if !k.reach[r.Name] {
				continue
			}
			k.countedWalk(r.Expr, counted[r.Name], counted, &changed)
		}
	}
	for e := range k.srcExprs {
		rep, ok := e.(*ast.Repeat)
		if !ok {
			continue
		}
		if k.firstBound(rep) && k.scanCounted[rep] {
			delete(k.srcExprs, rep)
			delete(k.srcExprs, k.again[rep])
		}
	}
}

// firstBound reports whether a constant bounds the positions from which the evaluations of rep
// that reach a position start: the run bound, at most runMax characters of its alphabet before it,
// or the region bound of the body of a trivia rule (complexity_regions.go).
func (k *classifier) firstBound(rep *ast.Repeat) bool {
	d, ok := k.scanRun[rep]
	return ok && d < runOpen && d != runNone || k.regionScan[rep]
}

// rigid reports whether a constant bounds the positions from which e can match up to any one
// position. A rule takes one answer, found once: only rules that are not recursive have one to find.
func (k *classifier) rigid(e ast.Expr) bool {
	k.control.Poll()
	switch e := e.(type) {
	case *ast.Ref:
		if e.Level == rhsLevel || k.byName[e.Name] == nil || k.recursive[e.Name] {
			return false
		}
		v, ok := k.rigidRule[e.Name]
		if !ok {
			v = k.rigid(k.byName[e.Name].Expr)
			k.rigidRule[e.Name] = v
		}
		return v
	case *ast.Seq:
		for _, it := range e.Items {
			if !k.rigid(it) {
				return false
			}
		}
		return true
	case *ast.Choice:
		for _, alt := range e.Alts {
			if !k.rigid(alt) {
				return false
			}
		}
		return true
	case *ast.Repeat:
		if e.Max >= 0 {
			return k.rigid(e.Expr)
		}
		// A repetition that no path reaches (runNone) is rigid as well: it has no evaluations.
		d, ok := k.scanRun[e]
		return ok && d < runOpen || k.regionScan[e]
	case *ast.Optional:
		return k.rigid(e.Expr)
	case *ast.Group:
		return k.rigid(e.Expr)
	case *ast.Capture:
		return k.rigid(e.Expr)
	case *ast.Discard:
		return k.rigid(e.Expr)
	case *ast.Atomic:
		return k.rigid(e.Expr)
	case *ast.Attributed, *ast.OperatorsRule:
		// The classifier reads the lowered and normalized grammar, where #recover and #stream are
		// attributes and inline actions are calls of hidden rules.
		return false
	}
	return true
}

// countedWalk walks e with ok saying that an evaluation of e at a position is one of a constant
// number there, marks the repetitions for which that holds, and clears the rules called where it
// does not hold.
func (k *classifier) countedWalk(e ast.Expr, ok bool, counted map[string]bool, changed *bool) {
	k.control.Poll()
	switch e := e.(type) {
	case *ast.Ref:
		if !ok && counted[e.Name] && !k.memo[e.Name] {
			counted[e.Name] = false
			*changed = true
		}
	case *ast.Seq:
		for _, it := range e.Items {
			k.countedWalk(it, ok, counted, changed)
			ok = ok && k.rigid(it)
		}
	case *ast.Choice:
		for _, alt := range e.Alts {
			k.countedWalk(alt, ok, counted, changed)
		}
	case *ast.Repeat:
		if e.Max == 0 {
			return
		}
		if ok && e.Max < 0 {
			k.scanCounted[e] = true
		}
		k.countedWalk(e.Expr, ok && k.rigid(e), counted, changed)
	case *ast.Optional:
		k.countedWalk(e.Expr, ok, counted, changed)
	case *ast.And:
		k.countedWalk(e.Expr, ok, counted, changed)
	case *ast.Not:
		k.countedWalk(e.Expr, ok, counted, changed)
	case *ast.Group:
		k.countedWalk(e.Expr, ok, counted, changed)
	case *ast.Capture:
		k.countedWalk(e.Expr, ok, counted, changed)
	case *ast.Discard:
		k.countedWalk(e.Expr, ok, counted, changed)
	case *ast.Atomic:
		k.countedWalk(e.Expr, ok, counted, changed)
	case *ast.Attributed:
		k.countedWalk(e.Expr, ok, counted, changed)
		if skip := compile.RecoverSkip(e); skip != nil {
			k.countedWalk(skip, ok, counted, changed)
		}
	case *ast.OperatorsRule:
		if e.Skip != nil {
			k.countedWalk(e.Skip, false, counted, changed)
		}
		for _, o := range e.Operands {
			k.countedWalk(o.Expr, false, counted, changed)
		}
		for _, l := range e.Levels {
			for _, op := range l.Operators {
				k.countedWalk(op.Expr, false, counted, changed)
			}
		}
	}
}
