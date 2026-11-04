package compile

import (
	"slices"

	"zeroparse.dev/zp/grammar"
)

// This file is a prototype of the exclusion analysis over symbols. It has two uses, both reached
// only from tests:
//
//   - Automatic cuts: an alternative of an ordered choice after a prefix of which no later
//     alternative can succeed at the same position takes a #cut after that prefix, which does not
//     change what the choice matches (AutoCut).
//   - Committing units: a unit and a continuation that evaluate the same symbols from one
//     position and then part at a symbol that decides between them restart only those shared
//     symbols (decidedUnitsOn).

// cutPrefixesOn makes the classifier treat an alternative whose top-level sequence has a #cut
// after items that can fail as a unit of those items alone: when the alternative fails after the
// cut, the choice fails and its later alternatives do not run.
var cutPrefixesOn bool

// decidedUnitsOn makes the classifier accept as committing a unit that the strict exclusion
// analysis decides against each continuation within decidedUnitsK symbols.
var (
	decidedUnitsOn bool
	decidedUnitsK  = 3
)

const (
	autoCutPaths  = 256 // the most paths of an expression that the analysis enumerates
	autoCutInline = 6   // the most rule bodies that the analysis expands for one pair of paths
	autoCutFollow = 6   // the most rule and operand ends through which a continuation is followed
	autoCutLen    = 12  // the most symbols of a continuation path
)

type symKind uint8

const (
	symMatch symKind = iota // a call, literal, class or any character
	symNot                  // a negative lookahead, which consumes nothing
	symAnd                  // a positive lookahead, which consumes nothing
	symSkip                 // consumes nothing and tells nothing (predicates, anchors, cuts)
	symStop                 // an expression that the analysis does not look into
	// symBind consumes nothing and may change the variables in scope: a binding, an expression
	// that contains one, or the end of a rule that makes bindings.
	symBind
	// symEnv consumes nothing and marks a part that is cheap only when the unit and the
	// continuation evaluate it with the same variables.
	symEnv
	// symEnd is the end of the parse after the start rule: whole-input parsing checks the end of
	// the input there, which costs a constant, and prefix parsing stops.
	symEnd
)

// sym is a symbol of a path; item is the index of the top-level item of the alternative that it
// comes from.
type sym struct {
	kind symKind
	e    grammar.Expr
	item int
}

// excl is one run of the analysis. In strict mode it also requires what makes a decision a bound
// on the input that the continuation examines again: the symbols before the decision are cheap,
// and the analysis looks only into options, repetitions, choices and lookaheads whose parts are
// all cheap, as a path does not show the attempts that fail in them.
type excl struct {
	k      *classifier
	strict bool
	maxK   int
	// env reports that the unit and the continuation are at a point where they have the same
	// variables in scope; excludes sets it for the comparison at hand.
	env    bool
	cheapR map[cheapKey]int8 // 1 while computed, 2 cheap, 3 not
}

type cheapKey struct {
	rule string
	env  bool
}

func newExcl(k *classifier, strict bool, maxK int) excl {
	return excl{k: k, strict: strict, maxK: maxK, cheapR: map[cheapKey]int8{}}
}

// cheap reports whether evaluating e again at a position costs no more than a constant or what
// the first evaluation there cost: a terminal, a call answered alike, an expression without a
// scanning source, or a call of a rule whose body is cheap in all its parts.
func (x excl) cheap(e grammar.Expr) bool {
	r, ok := e.(*grammar.Ref)
	if !ok {
		return !x.k.containsSource(e)
	}
	if x.k.shareable(r) || !x.k.containsSource(r) || triviaOn && x.k.isTrivia(r) {
		return true
	}
	reads := len(x.k.vars[r.Name]) > 0
	if reads && !x.env {
		return false
	}
	// With the same variables, the memo answers the second call of a rule that reads them as it
	// answers any other, and a rule that calls no rule repeats only its own scan.
	if reads && r.Level != rhsLevel && (x.k.memoAt(r) || len(x.k.calls[r.Name]) == 0) {
		return true
	}
	def := x.k.byName[r.Name]
	if r.Level != "" || def == nil || x.k.an.cyclic[r.Name] {
		return false
	}
	key := cheapKey{r.Name, x.env}
	switch x.cheapR[key] {
	case 1, 3:
		return false
	case 2:
		return true
	}
	x.cheapR[key] = 1
	ok = x.cheapAll(def.Expr)
	x.cheapR[key] = 3
	if ok {
		x.cheapR[key] = 2
	}
	return ok
}

// cheapAll reports whether every part of e is cheap: e has no scanning source of its own, and each
// call in it is cheap. Evaluating e again at a position, or an attempt of it that fails, then costs
// no more than a constant or what the calls cost the first time.
func (x excl) cheapAll(e grammar.Expr) bool {
	ok := true
	WalkExpr(e, func(y grammar.Expr) {
		if x.k.srcExprs[y] {
			ok = false
		}
		if r, isRef := y.(*grammar.Ref); isRef && !x.cheap(r) {
			ok = false
		}
	})
	return ok
}

// paths returns the paths of e: sequences of symbols such that every successful evaluation of e
// evaluates, and succeeds on, the symbols of one of them in order. ok is false when there are more
// than autoCutPaths of them.
func (x excl) paths(e grammar.Expr, item int) (paths [][]sym, ok bool) {
	one := func(s sym) ([][]sym, bool) { return [][]sym{{s}}, true }
	if x.strict {
		switch e.(type) {
		case *grammar.Choice, *grammar.Optional, *grammar.Repeat:
			if !x.cheapAll(e) {
				y := x
				y.env = true
				if !y.cheapAll(e) {
					return one(sym{symStop, e, item})
				}
				// The part is cheap where the variables are the same, which the comparison
				// checks when it reaches the mark.
				envSym := []sym{{symEnv, e, item}}
				ps, ok := x.pathsOf(e, item)
				for i, p := range ps {
					ps[i] = append(slices.Clip(envSym), p...)
				}
				return ps, ok
			}
		}
	}
	return x.pathsOf(e, item)
}

// pathsOf is paths without the check of strict mode on e itself.
func (x excl) pathsOf(e grammar.Expr, item int) (paths [][]sym, ok bool) {
	one := func(s sym) ([][]sym, bool) { return [][]sym{{s}}, true }
	switch e := e.(type) {
	case nil:
		return [][]sym{nil}, true
	case *grammar.Ref, *grammar.Literal, *grammar.CharClass, *grammar.Any:
		return one(sym{symMatch, e, item})
	case *grammar.Not:
		if binds(e.Expr) {
			return [][]sym{{{symNot, e.Expr, item}, {symBind, e, item}}}, true
		}
		return one(sym{symNot, e.Expr, item})
	case *grammar.And:
		if binds(e.Expr) {
			return [][]sym{{{symAnd, e.Expr, item}, {symBind, e, item}}}, true
		}
		return one(sym{symAnd, e.Expr, item})
	case *grammar.Predicate:
		if binds(e) {
			return one(sym{symBind, e, item})
		}
		return one(sym{symSkip, e, item})
	case *grammar.Cut, *grammar.Top, *grammar.BeginInput, *grammar.EndInput,
		*grammar.BeginLine, *grammar.EndLine:
		return one(sym{symSkip, e, item})
	case *grammar.Bottom:
		return nil, true
	case *grammar.Group:
		return x.paths(e.Expr, item)
	case *grammar.Capture:
		return x.paths(e.Expr, item)
	case *grammar.Discard:
		return x.paths(e.Expr, item)
	case *grammar.Atomic:
		return x.paths(e.Expr, item)
	case *grammar.Stream:
		return x.paths(e.Expr, item)
	case *grammar.InlineAction:
		return x.paths(e.Expr, item)
	case *grammar.Seq:
		paths = [][]sym{nil}
		for _, it := range e.Items {
			ps, ok := x.paths(it, item)
			if !ok {
				return nil, false
			}
			if paths, ok = concatPaths(paths, ps); !ok {
				return nil, false
			}
		}
		return paths, true
	case *grammar.Choice:
		for _, alt := range e.Alts {
			ps, ok := x.paths(alt, item)
			if !ok {
				return nil, false
			}
			paths = append(paths, ps...)
		}
		return paths, len(paths) <= autoCutPaths
	case *grammar.Optional:
		ps, ok := x.paths(e.Expr, item)
		return append([][]sym{nil}, ps...), ok && len(ps) < autoCutPaths
	case *grammar.Repeat:
		if e.Max == 0 {
			return [][]sym{nil}, true
		}
		if e.Max != 1 {
			// A repetition is one symbol, which the analysis expands only when it must: two
			// equal repetitions at one position match alike, as they are greedy.
			if binds(e) {
				return [][]sym{{{symMatch, e, item}, {symBind, e, item}}}, true
			}
			return one(sym{symMatch, e, item})
		}
		return x.repeatPaths(e, item)
	}
	// Attributed expressions, #recover, Operators Rules.
	return one(sym{symStop, e, item})
}

// repeatPaths returns the paths of a repetition: no iteration when its minimum is 0, and its
// expression followed, when it can repeat, by a stop.
func (x excl) repeatPaths(e *grammar.Repeat, item int) ([][]sym, bool) {
	{
		ps, ok := x.paths(e.Expr, item)
		if !ok {
			return nil, false
		}
		if e.Max != 1 {
			for i, p := range ps {
				ps[i] = append(slices.Clip(p), sym{symStop, e, item})
			}
		}
		if e.Min == 0 {
			ps = append([][]sym{nil}, ps...)
		}
		return ps, len(ps) <= autoCutPaths
	}
}

// body returns the paths that a symbol stands for: the body of the rule of a call, or the paths of
// a repetition.
func (x excl) body(s sym) ([][]sym, bool) {
	if r, ok := s.e.(*grammar.Repeat); ok {
		return x.repeatPaths(r, s.item)
	}
	def, ok := x.inlinable(s.e)
	if !ok {
		return nil, false
	}
	return x.paths(def.Expr, s.item)
}

// concatPaths returns every path of ps followed by a path of qs; a path that ends with a stop, or
// has autoCutLen symbols, is not extended.
func concatPaths(ps, qs [][]sym) ([][]sym, bool) {
	var next [][]sym
	for _, p := range ps {
		if n := len(p); n > 0 && p[n-1].kind == symStop {
			next = append(next, p)
			continue
		}
		if len(p) >= autoCutLen {
			next = append(next, append(slices.Clip(p), sym{kind: symStop, e: stopLength}))
			continue
		}
		for _, q := range qs {
			next = append(next, append(slices.Clip(p), q...))
		}
	}
	return next, len(next) <= autoCutPaths
}

// contPaths returns the paths of a continuation, through the calls of a rule at its end, as far
// as autoCutFollow rule ends. A path that the analysis cuts short ends with a stop.
func (x excl) contPaths(c *cont, depth int) ([][]sym, bool) {
	paths := [][]sym{nil}
	for ; c != nil; c = c.next {
		for _, it := range c.items {
			ps, ok := x.paths(it, 0)
			if !ok {
				return nil, false
			}
			if paths, ok = concatPaths(paths, ps); !ok {
				return nil, false
			}
		}
		if c.next != nil {
			continue
		}
		var after [][]sym
		var ok bool
		switch c.end {
		case endNone:
			return paths, true
		case endRule:
			after, ok = x.ruleEnd(c.rule, depth)
		case endOps:
			after, ok = x.opsEnd(c.rule, depth)
		}
		if !ok {
			return nil, false
		}
		return concatPaths(paths, after)
	}
	return paths, true
}

// Stops that end a continuation that the analysis follows no further, told apart for diagnostics.
var (
	stopFollow = &grammar.Bottom{} // more than autoCutFollow rule or operator ends
	stopLength = &grammar.Bottom{} // autoCutLen symbols
)

// ruleEnd returns the paths of what follows the end of rule: what follows each of its calls, and
// for the start rule the end of the parse.
func (x excl) ruleEnd(rule string, depth int) ([][]sym, bool) {
	var after [][]sym
	if rule == x.k.start {
		after = append(after, []sym{{kind: symEnd}})
	}
	for _, s := range x.k.sites {
		if s.callee != rule {
			continue
		}
		if depth >= autoCutFollow {
			after = append(after, []sym{{kind: symStop, e: stopFollow}})
			continue
		}
		ps, ok := x.contPaths(s.cont, depth+1)
		if !ok {
			return nil, false
		}
		if def := x.k.byName[rule]; def != nil && binds(def.Expr) {
			// The caller does not see the bindings of the rule, which the unit may have seen.
			for i, p := range ps {
				ps[i] = append([]sym{{kind: symBind}}, p...)
			}
		}
		after = append(after, ps...)
	}
	if len(after) == 0 {
		after = append(after, []sym{{kind: symStop}})
	}
	return after, len(after) <= autoCutPaths
}

// opsEnd returns the paths of what follows the end of an operand of the Operators Rule of rule:
// an operator part that applies after it, with its right operand, or the end of the rule. The
// parts are those that the classifier takes as the continuations of an operand.
func (x excl) opsEnd(rule string, depth int) ([][]sym, bool) {
	if depth >= autoCutFollow {
		return [][]sym{{{kind: symStop, e: stopFollow}}}, true
	}
	var after [][]sym
	ok := true
	x.k.opsLeads(rule, func(c *cont) {
		if !ok {
			return
		}
		var ps [][]sym
		if ps, ok = x.contPaths(c, depth+1); ok {
			after = append(after, ps...)
		}
	})
	if !ok {
		return nil, false
	}
	ps, ok := x.ruleEnd(rule, depth)
	if !ok {
		return nil, false
	}
	after = append(after, ps...)
	return after, len(after) <= autoCutPaths
}

// inlinable reports whether the analysis may expand a call into the paths of its body: the callee
// is not a rule of a left-recursion cycle. A callee that reads variables can be expanded too: every
// successful evaluation follows one of the paths of its body, whatever the variables, and the
// bindings in the body appear on the paths.
func (x excl) inlinable(e grammar.Expr) (*grammar.RuleDef, bool) {
	r, ok := e.(*grammar.Ref)
	if !ok || r.Level != "" {
		return nil, false
	}
	def := x.k.byName[r.Name]
	if def == nil || x.k.an.cyclic[r.Name] {
		return nil, false
	}
	return def, true
}

// sameSym reports whether two symbols at one position succeed alike and end alike: a parsing
// expression evaluates deterministically, so two calls of one rule from one position, with the same
// values of the variables that it reads, have one result. A rule that reads variables is equal to
// itself only where x.env holds. Calls of rules of left-recursion cycles are left aside, as a call
// inside a growth sees the seed of the growth.
func (x excl) sameSym(a, b grammar.Expr) bool {
	switch a := a.(type) {
	case *grammar.Ref:
		b, ok := b.(*grammar.Ref)
		// A right operand is parsed at the level of its operator, which the reference does not
		// tell, so two of them are not equal.
		return ok && a.Name == b.Name && a.Level == b.Level && a.Level != rhsLevel &&
			(x.env || len(x.k.vars[a.Name]) == 0) &&
			!x.k.an.cyclic[a.Name]
	case *grammar.Literal:
		b, ok := b.(*grammar.Literal)
		return ok && a.Value == b.Value && a.IgnoreCase == b.IgnoreCase
	case *grammar.CharClass:
		b, ok := b.(*grammar.CharClass)
		return ok && classChars(a).equal(classChars(b))
	case *grammar.Any:
		_, ok := b.(*grammar.Any)
		return ok
	case *grammar.Repeat:
		b, ok := b.(*grammar.Repeat)
		return ok && (a == b || grammar.FormatExpr(a) == grammar.FormatExpr(b)) && x.pure(a)
	}
	return false
}

// pure reports whether e calls only rules that are not rules of left-recursion cycles, and read no
// variable unless x.env holds, and has no predicate itself, so that its result at a position is
// one.
func (x excl) pure(e grammar.Expr) bool {
	ok := true
	WalkExpr(e, func(y grammar.Expr) {
		switch y := y.(type) {
		case *grammar.Ref:
			if !x.env && len(x.k.vars[y.Name]) > 0 || x.k.an.cyclic[y.Name] {
				ok = false
			}
		case *grammar.Predicate, *grammar.Recovery, *grammar.Attributed, *grammar.OperatorsRule, *grammar.Cut:
			ok = false
		}
	})
	return ok
}

// disjoint reports whether two expressions cannot both succeed at one position: each consumes a
// first character, and their first characters are disjoint. Whichever fails, it fails at the
// character at that position, which the other one would consume.
func (x excl) disjoint(a, b grammar.Expr) bool {
	return !x.k.nullable(a) && !x.k.nullable(b) && !x.k.first(a).intersects(x.k.first(b))
}

// litStep compares two literals at one position. When one is a prefix of the other it returns the
// rest of the longer one, on the side of a (restA) or of b; excl reports that they cannot both
// match.
func litStep(a, b *grammar.Literal) (restA, restB *grammar.Literal, excl, ok bool) {
	if a.Value == "" || b.Value == "" {
		return nil, nil, false, false
	}
	fold := a.IgnoreCase || b.IgnoreCase
	av, bv := []rune(a.Value), []rune(b.Value)
	n := min(len(av), len(bv))
	for i := range n {
		x, y := av[i], bv[i]
		if fold {
			x, y = foldASCII(x), foldASCII(y)
		}
		if x != y {
			return nil, nil, true, true
		}
	}
	if a.IgnoreCase != b.IgnoreCase {
		// The characters agree up to case; the analysis does not follow mixed literals further.
		return nil, nil, false, false
	}
	switch {
	case len(av) < len(bv):
		return nil, &grammar.Literal{Value: string(bv[n:]), IgnoreCase: b.IgnoreCase}, false, true
	case len(bv) < len(av):
		return &grammar.Literal{Value: string(av[n:]), IgnoreCase: a.IgnoreCase}, nil, false, true
	}
	return nil, nil, false, true
}

// litInChar reports whether a literal and a symbol that matches one character from a class, or
// any character, both match the first character of the literal wherever the literal matches. It
// returns the rest of the literal, or nil when the literal has one character.
func litInChar(l, c grammar.Expr) (rest *grammar.Literal, ok bool) {
	lit, isLit := l.(*grammar.Literal)
	if !isLit || lit.Value == "" {
		return nil, false
	}
	switch c := c.(type) {
	case *grammar.CharClass:
		if literalFirst(lit).intersects(classChars(c).complement()) {
			return nil, false
		}
	case *grammar.Any:
	default:
		return nil, false
	}
	rs := []rune(lit.Value)
	if len(rs) == 1 {
		return nil, true
	}
	return &grammar.Literal{Value: string(rs[1:]), IgnoreCase: lit.IgnoreCase}, true
}

func foldASCII(r rune) rune {
	if 'A' <= r && r <= 'Z' {
		return r + 'a' - 'A'
	}
	return r
}

// replaced returns p with p[i] replaced by xs.
func replaced(p []sym, i int, xs ...sym) []sym {
	out := make([]sym, 0, len(p)-1+len(xs))
	out = append(out, p[:i]...)
	out = append(out, xs...)
	return append(out, p[i+1:]...)
}

// exclState is one step of the comparison of a path p with a path q from one position.
type exclState struct {
	p, q   []sym
	i, j   int
	steps  int // the symbols passed that both evaluate alike
	inline int
	// boundP and boundQ report that p or q passed a symbol that may change the variables in
	// scope since the position where both start, with the same variables.
	boundP, boundQ bool
}

// zeroBinds reports whether the symbols of p from i up to the first one that consumes input
// include one that may change the variables.
func zeroBinds(p []sym, i int) bool {
	for ; i < len(p); i++ {
		switch p[i].kind {
		case symBind:
			return true
		case symMatch, symStop:
			return false
		}
	}
	return false
}

// nextMatch returns the index of the first symbol of q from j that is not a skip or a lookahead,
// and whether the lookaheads skipped on the way are cheap in strict mode.
func (x excl) nextMatch(q []sym, j int) (int, bool) {
	for ; j < len(q); j++ {
		switch q[j].kind {
		case symSkip, symBind, symEnv:
		case symNot, symAnd:
			if x.strict && !x.cheapAll(q[j].e) {
				return j, false
			}
		default:
			return j, true
		}
	}
	return j, true
}

// excludes reports whether no evaluation can succeed on q from a position where the prefix of p up
// to its decisive symbol succeeds. item is the top-level item of that symbol and n the number of
// symbols that the decision examines (those passed alike and the decisive one).
func (x excl) excludes(s exclState) (ok bool, item, n int) {
	k := x.k
	for {
		k.control.poll()
		x.env = !s.boundP && !s.boundQ
		if s.i == len(s.p) {
			// In strict mode every symbol of p was evaluated alike by q or is cheap, so a unit
			// that ends here examined, beyond what q evaluates alike, no more than a constant.
			if !x.strict {
				return x.no("p-exhausted", sym{}, nil)
			}
			return true, 0, s.steps
		}
		a := s.p[s.i]
		switch a.kind {
		case symStop, symEnd:
			return x.no("pstop", a, nil)
		case symSkip:
			s.i++
			continue
		case symBind:
			s.boundP = true
			s.i++
			continue
		case symEnv:
			if !x.env || zeroBinds(s.q, s.j) {
				return x.no("env", a, nil)
			}
			s.i++
			continue
		case symNot:
			if x.implies(s.q[s.j:], a.e) {
				return true, a.item, s.steps + 1
			}
			if x.strict && !x.cheapAll(a.e) {
				return x.no("pnot-notcheap", a, nil)
			}
			s.i++
			continue
		case symAnd:
			if j, _ := x.nextMatch(s.q, s.j); j < len(s.q) && s.q[j].kind == symMatch && x.disjoint(a.e, s.q[j].e) {
				return true, a.item, s.steps + 1
			}
			if s.j < len(s.q) && s.q[s.j].kind == symAnd && x.disjoint(a.e, s.q[s.j].e) {
				return true, a.item, s.steps + 1
			}
			if x.strict && !x.cheapAll(a.e) {
				return x.no("pand-notcheap", a, nil)
			}
			s.i++
			continue
		}
		if s.j == len(s.q) {
			return x.no("q-exhausted", a, nil)
		}
		b := s.q[s.j]
		switch b.kind {
		case symEnd:
			// The continuation ends the parse after the symbols that both evaluate alike, which
			// in strict mode are cheap, so it examines again no more than what they cost.
			if x.strict {
				return true, a.item, s.steps
			}
			return x.no("q-end", a, &b)
		case symStop:
			return x.no("qstop", a, &b)
		case symSkip:
			s.j++
			continue
		case symBind:
			s.boundQ = true
			s.j++
			continue
		case symEnv:
			if !x.env {
				return x.no("env", a, &b)
			}
			s.j++
			continue
		case symNot:
			if end, ok := x.impliesAt(s.p[s.i:], b.e); ok {
				return true, s.p[s.i+end].item, s.steps + 1
			}
			if x.strict && !x.cheapAll(b.e) {
				return x.no("qnot-notcheap", a, &b)
			}
			s.j++
			continue
		case symAnd:
			if x.disjoint(a.e, b.e) {
				return true, a.item, s.steps + 1
			}
			if x.strict && !x.cheapAll(b.e) {
				return x.no("qand-notcheap", a, &b)
			}
			s.j++
			continue
		}
		// A call of a trivia rule matches nothing after a call of the same rule.
		if x.triviaCall(a) {
			if p := x.collapseTrivia(s.p, s.i); len(p) != len(s.p) {
				s.p = p
			}
		}
		if x.triviaCall(b) {
			if q := x.collapseTrivia(s.q, s.j); len(q) != len(s.q) {
				s.q = q
			}
		}
		if x.disjoint(a.e, b.e) {
			return true, a.item, s.steps + 1
		}
		// Two calls of one trivia rule at one position end alike, and in strict mode the second
		// costs a constant, so they do not count toward the bound.
		if x.triviaCall(a) && x.sameSym(a.e, b.e) {
			s.i, s.j = s.i+1, s.j+1
			continue
		}
		if x.sameSym(a.e, b.e) {
			if s.steps+1 >= x.maxK || x.strict && !x.cheap(a.e) {
				if s.steps+1 >= x.maxK {
					return x.no("same-bound", a, &b)
				}
				return x.no("same-notcheap", a, &b)
			}
			s.i, s.j, s.steps = s.i+1, s.j+1, s.steps+1
			continue
		}
		la, aLit := a.e.(*grammar.Literal)
		lb, bLit := b.e.(*grammar.Literal)
		if aLit && bLit {
			ra, rb, excl, ok := litStep(la, lb)
			if !ok {
				return x.no("lit-fold", a, &b)
			}
			if excl {
				return true, a.item, s.steps + 1
			}
			if s.steps+1 >= x.maxK {
				return x.no("lit-bound", a, &b)
			}
			// One literal is a prefix of the other: both evaluations are at the end of the shorter
			// one, and the longer one goes on with its rest.
			switch {
			case ra != nil:
				s.p = replaced(s.p, s.i, sym{symMatch, ra, a.item})
				s.j++
			case rb != nil:
				s.q = replaced(s.q, s.j, sym{symMatch, rb, b.item})
				s.i++
			}
			s.steps++
			continue
		}
		// Two classes, or a class and any character, that overlap: split each into the part that
		// the other matches and the rest. If either matches in its rest, the other fails, so
		// where both succeed they consume one character of the intersection and end alike.
		if oneChar(a.e) && oneChar(b.e) {
			if s.steps+1 >= x.maxK {
				return x.no("char-bound", a, &b)
			}
			s.i, s.j, s.steps = s.i+1, s.j+1, s.steps+1
			continue
		}
		// A literal and a class that contains its first character both consume that character.
		if rest, ok := litInChar(a.e, b.e); ok {
			if s.steps+1 >= x.maxK {
				return x.no("char-bound", a, &b)
			}
			if rest != nil {
				s.p = replaced(s.p, s.i, sym{symMatch, rest, a.item})
			} else {
				s.i++
			}
			s.j, s.steps = s.j+1, s.steps+1
			continue
		}
		if rest, ok := litInChar(b.e, a.e); ok {
			if s.steps+1 >= x.maxK {
				return x.no("char-bound", a, &b)
			}
			if rest != nil {
				s.q = replaced(s.q, s.j, sym{symMatch, rest, b.item})
			} else {
				s.j++
			}
			s.i, s.steps = s.i+1, s.steps+1
			continue
		}
		if s.inline >= autoCutInline {
			return x.no("inline-limit", a, &b)
		}
		// Expand a call or a repetition into its paths; every path must be excluded. The side
		// whose callee reaches the other callee is expanded first.
		expandP := func() (bool, int, int) {
			return x.expand(s, s.p, s.i, func(t *exclState, p []sym) { t.p = p })
		}
		expandQ := func() (bool, int, int) {
			return x.expand(s, s.q, s.j, func(t *exclState, q []sym) { t.q = q })
		}
		if ra, ok := a.e.(*grammar.Ref); ok {
			if rb, ok := b.e.(*grammar.Ref); ok && k.reachable(rb.Name)[ra.Name] && !k.reachable(ra.Name)[rb.Name] {
				expandP, expandQ = expandQ, expandP
			}
		}
		if ok, item, n := expandP(); ok {
			return true, item, n
		}
		return expandQ()
	}
}

// expand replaces the symbol side[idx] by each path that it stands for and requires each to be
// excluded.
func (x excl) expand(s exclState, side []sym, idx int, set func(*exclState, []sym)) (bool, int, int) {
	body, ok := x.body(side[idx])
	if !ok {
		return x.no("noexpand", side[idx], nil)
	}
	item, n := 0, 0
	for _, bp := range body {
		for i := range bp {
			bp[i].item = side[idx].item
		}
		t := s
		t.inline++
		set(&t, replaced(side, idx, bp...))
		ok, item1, n1 := x.excludes(t)
		if !ok {
			return false, 0, 0
		}
		item, n = max(item, item1), max(n, n1)
	}
	return true, item, n
}

// implies reports whether every evaluation that succeeds on q also matches the lookahead body at
// its start, so that a negative lookahead on the body fails there.
func (x excl) implies(q []sym, body grammar.Expr) bool {
	_, ok := x.impliesAt(q, body)
	return ok
}

// impliesAt is implies; end is the index of the last symbol of q that the match of body reads.
func (x excl) impliesAt(q []sym, body grammar.Expr) (end int, ok bool) {
	for _, kp := range x.exactPaths(body) {
		if end, ok := x.matchesExact(q, kp, 0); ok {
			return end, true
		}
	}
	return 0, false
}

// exactPaths returns the paths of a lookahead body for which a match of the path is a match of the
// body: the alternatives of a top-level choice, through calls that make up the whole body, whose
// items are calls, literals, classes and any characters. A path through an option, a repetition
// or a nested choice can match where the body, which is greedy and ordered, does not.
func (x excl) exactPaths(body grammar.Expr) [][]grammar.Expr {
	for range autoCutInline {
		def, ok := x.inlinable(ungroup(body))
		if !ok {
			break
		}
		body = def.Expr
	}
	var alts []grammar.Expr
	if c, ok := ungroup(body).(*grammar.Choice); ok {
		alts = c.Alts
	} else {
		alts = []grammar.Expr{body}
	}
	var out [][]grammar.Expr
	for _, alt := range alts {
		var p []grammar.Expr
		ok := true
		var add func(e grammar.Expr)
		add = func(e grammar.Expr) {
			switch e := e.(type) {
			case *grammar.Group:
				add(e.Expr)
			case *grammar.Discard:
				add(e.Expr)
			case *grammar.Capture:
				add(e.Expr)
			case *grammar.Atomic:
				add(e.Expr)
			case *grammar.Seq:
				for _, it := range e.Items {
					add(it)
				}
			case *grammar.Ref, *grammar.Literal, *grammar.CharClass, *grammar.Any:
				p = append(p, e)
			default:
				ok = false
			}
		}
		add(alt)
		if ok {
			out = append(out, p)
		}
	}
	return out
}

// matchesExact reports whether every evaluation that succeeds on q matches the exact path kp at its
// start. In strict mode the symbols that both evaluate are cheap.
func (x excl) matchesExact(q []sym, kp []grammar.Expr, depth int) (end int, ok bool) {
	j, m := 0, 0
	for m < len(kp) {
		var ok bool
		if j, ok = x.nextMatch(q, j); !ok || j == len(q) || q[j].kind != symMatch {
			return 0, false
		}
		a, b := q[j].e, kp[m]
		y := x
		for _, z := range q[:j] {
			switch {
			case z.kind == symBind:
				y.env = false
			case z.kind == symEnv && !y.env:
				return 0, false
			}
		}
		if y.sameSym(a, b) {
			if y.strict && !y.cheap(a) {
				return 0, false
			}
			j, m = j+1, m+1
			continue
		}
		la, aLit := a.(*grammar.Literal)
		lb, bLit := b.(*grammar.Literal)
		if aLit && bLit {
			ra, rb, excl, ok := litStep(la, lb)
			if !ok || excl {
				return 0, false
			}
			switch {
			case ra != nil:
				q = replaced(q, j, sym{symMatch, ra, q[j].item})
				m++
			case rb != nil:
				kp = append([]grammar.Expr{rb}, kp[m+1:]...)
				m = 0
				j++
			default:
				j, m = j+1, m+1
			}
			continue
		}
		// A class of q that the class of the body contains matches it.
		if ca, ok := a.(*grammar.CharClass); ok {
			if cb, ok := b.(*grammar.CharClass); ok && !classChars(ca).intersects(classChars(cb).complement()) {
				j, m = j+1, m+1
				continue
			}
		}
		// A literal of q matches a class of the body that contains its first character.
		if rest, ok := litInChar(a, b); ok {
			if rest != nil {
				q = replaced(q, j, sym{symMatch, rest, q[j].item})
			} else {
				j++
			}
			m++
			continue
		}
		// Expand a call or a repetition of q: every path of it must match.
		if depth >= autoCutInline {
			return 0, false
		}
		body, ok := x.body(q[j])
		if !ok {
			return 0, false
		}
		for _, bp := range body {
			if _, ok := x.matchesExact(replaced(q, j, bp...), kp[m:], depth+1); !ok {
				return 0, false
			}
		}
		// The match ends inside the call.
		return j, true
	}
	return max(j-1, 0), true
}

// altDecision is the result of the analysis for one alternative of a choice.
type altDecision struct {
	decided bool
	k       int // the most symbols that a decision examines
	cut     int // the index of the top-level item after which a #cut goes
	items   int // the number of top-level items of the alternative
}

// topItems returns the top-level items of an alternative.
func topItems(alt grammar.Expr) []grammar.Expr {
	if s, ok := ungroup(alt).(*grammar.Seq); ok {
		return s.Items
	}
	return []grammar.Expr{alt}
}

// decideAlt decides whether the alternative i of ch excludes every later alternative after a
// prefix of its top-level items, examining at most maxK symbols.
func (k *classifier) decideAlt(ch *grammar.Choice, i, maxK int) altDecision {
	x := newExcl(k, false, maxK)
	items := topItems(ch.Alts[i])
	d := altDecision{items: len(items)}
	ps := [][]sym{nil}
	for n, it := range items {
		ips, ok := x.paths(it, n)
		if !ok {
			return d
		}
		if ps, ok = concatPaths(ps, ips); !ok {
			return d
		}
	}
	cut, kk := 0, 0
	for _, later := range ch.Alts[i+1:] {
		qs, ok := x.paths(later, 0)
		if !ok {
			return d
		}
		for _, p := range ps {
			for _, q := range qs {
				ok, item, n := x.excludes(exclState{p: p, q: q})
				if !ok {
					return d
				}
				cut, kk = max(cut, item+1), max(kk, n)
			}
		}
	}
	d.decided, d.k, d.cut = true, kk, cut
	return d
}

// unitDecided reports whether the strict analysis decides the unit against each of its
// continuations within decidedUnitsK symbols, or finds that the unit evaluates nothing but
// symbols that the continuation evaluates alike and cheap ones. The continuation then evaluates
// again only the symbols that both evaluate alike, which are cheap, and fails at the decisive one
// at its first character, or examines again only the constant input of the cheap symbols.
func (k *classifier) unitDecided(u *unit, conts []contSpec) bool {
	if u.kind == unitLookahead {
		return false
	}
	x := newExcl(k, true, decidedUnitsK)
	var ps [][]sym
	for _, c := range u.self {
		p, ok := x.contPaths(c, 0)
		if !ok {
			return false
		}
		ps = append(ps, p...)
	}
	for _, cs := range conts {
		for _, c := range cs.paths {
			qs, ok := x.contPaths(c, 0)
			if !ok {
				return false
			}
			for _, p := range ps {
				for _, q := range qs {
					if ok, _, _ := x.excludes(exclState{p: p, q: q}); !ok {
						return false
					}
				}
			}
		}
	}
	return true
}

// AutoCutStats counts the decisions of the analysis over the choices of the reachable rules.
type AutoCutStats struct {
	Choices, Alts int    // choices with two or more alternatives, and their alternatives but the last
	ChoicesAt     [6]int // choices whose alternatives are all decided within k symbols (index k)
	AltsAt        [6]int // alternatives decided within k symbols
	Cuts          int    // alternatives that get a #cut (the decision ends before the last item)
	Committed     int    // alternatives that already commit with a #cut
}

// choiceKey locates an alternative of a choice by the position of the choice.
type choiceKey struct{ line, col, alt int }

// AutoCut returns g with a #cut inserted after the decisive prefix of every alternative that the
// analysis decides within maxK symbols, and the counts of the decisions up to 5 symbols. The
// analysis runs on the grammar that the classifier analyzes (inline actions lowered, attributes
// normalized); the cuts go into a copy of g at the same positions.
func AutoCut(g *grammar.Grammar, start string, maxK int) (*grammar.Grammar, AutoCutStats, error) {
	k, _, err := prepareClassifier(g, start, nil)
	if err != nil {
		return nil, AutoCutStats{}, err
	}
	k.computeBounded()
	k.computeFirst()
	const most = 5
	var st AutoCutStats
	cuts := map[choiceKey]altDecision{}
	dup := map[choiceKey]bool{}
	for _, r := range k.rules {
		if !k.reach[r.Name] {
			continue
		}
		WalkExpr(r.Expr, func(e grammar.Expr) {
			ch, ok := e.(*grammar.Choice)
			if !ok || len(ch.Alts) < 2 {
				return
			}
			st.Choices++
			worst := 0
			for i := range ch.Alts[:len(ch.Alts)-1] {
				st.Alts++
				if k.cutCommits(ch.Alts[i]) {
					st.Committed++
				}
				d := k.decideAlt(ch, i, most)
				if !d.decided {
					worst = most + 1
					continue
				}
				worst = max(worst, d.k)
				for n := d.k; n <= most; n++ {
					st.AltsAt[n]++
				}
				if d.k <= maxK && d.cut < d.items && !k.cutCommits(ch.Alts[i]) {
					p := exprPos(ch)
					key := choiceKey{p.Line, p.Col, i}
					if _, seen := cuts[key]; seen {
						dup[key] = true
					}
					cuts[key] = d
				}
			}
			for n := worst; n <= most; n++ {
				st.ChoicesAt[n]++
			}
		})
	}
	out := g.Clone()
	for _, r := range out.Rules() {
		WalkExpr(r.Expr, func(e grammar.Expr) {
			ch, ok := e.(*grammar.Choice)
			if !ok {
				return
			}
			p := exprPos(ch)
			for i, alt := range ch.Alts {
				key := choiceKey{p.Line, p.Col, i}
				d, ok := cuts[key]
				if !ok || dup[key] {
					continue
				}
				s, ok := ungroup(alt).(*grammar.Seq)
				if !ok || len(s.Items) != d.items {
					continue
				}
				s.Items = slices.Insert(slices.Clone(s.Items), d.cut, grammar.Expr(&grammar.Cut{}))
				if len(s.Breaks) >= d.cut {
					s.Breaks = slices.Insert(slices.Clone(s.Breaks), d.cut, nil)
				}
				st.Cuts++
				delete(cuts, key)
			}
		})
	}
	return out, st, nil
}

// binds reports whether evaluating e can bind a variable in the scope that it is evaluated in: e
// contains a binding outside the rules that it calls, whose bindings end with the call.
func binds(e grammar.Expr) bool {
	found := false
	WalkExpr(e, func(y grammar.Expr) {
		if p, ok := y.(*grammar.Predicate); ok {
			WalkTerm(p.Term, func(t grammar.Term) {
				if _, ok := t.(*grammar.Bind); ok {
					found = true
				}
			})
		}
	})
	return found
}

// exclTrace, when set, receives the reason for each comparison of paths that the analysis does not
// decide, with the symbols at which it stops. Tests use it to sort undecided units by cause.
var exclTrace func(tag string, a sym, b *sym)

// no reports an undecided comparison to exclTrace.
func (x excl) no(tag string, a sym, b *sym) (bool, int, int) {
	if exclTrace != nil {
		exclTrace(tag, a, b)
	}
	return false, 0, 0
}

// oneChar reports whether e matches exactly one character wherever it succeeds: a character class
// or any character.
func oneChar(e grammar.Expr) bool {
	switch e.(type) {
	case *grammar.CharClass, *grammar.Any:
		return true
	}
	return false
}
