package complexity

import (
	"math/bits"
	"slices"
	"unicode/utf8"
	"zeroparse.dev/zp/internal/grammaranalysis"

	"zeroparse.dev/zp/grammar"
)

// Region rules (see "Conditions for Linear Cost" in spec/runtime/parsing.md and Lemma 15 of
// spec/proofs/linear-cost.md).
//
// A trivia rule such as `ws = [ ]* (c [ ]*)*` with `c = "/*" [^*]* "*/"` has an alphabet of every
// character, so the first bound of a bounded scan finds no bound on the run before it. Its
// evaluations are still confined: when nothing outside the regions consumes the first character of
// a region except as a guarded token, no evaluation outside the regions starts strictly inside the
// match of a region, so the evaluations of the repetition that reach a position start in at most
// two positions, where the run bound gives d + 2. regionScans finds the region rules and returns
// the repetitions of the trivia rules among them, which take the region bound in place of the
// first bound.
//
// The region rules are chosen, then checked: a candidate that fails a condition is dropped and the
// check runs again, since dropping a region makes its terminals outer ones. The conditions are
// (D) the rule and the rules it reaches read and bind no variable and have no predicate, action,
// level call, left-recursion cycle, attributed expression, recovery, stream or Operators Rule, so
// that whether an expression of a region matches at a position, and where it ends, depend on the
// position alone; (N) the body of a region reaches no region, and no rule that it reaches is
// reached from the start rule otherwise than through a region; (O) the first characters of
// distinct regions are disjoint; and (G) every crossing, a terminal outside the regions that can
// consume a first character of a region, is guarded.

// regionScans returns the repetitions that are the bodies of trivia rules with the region bound.
// It runs after the run analysis, whose open runs pick the candidates.
func (k *classifier) regionScans() map[*grammar.Repeat]bool {
	regions := map[string]bool{} // the region rules; true for a trivia rule
	for _, r := range k.rules {
		k.control.Poll()
		if !k.reach[r.Name] || r.Name == k.start || k.triviaParts(r.Expr) == nil || !k.regionFree(r.Name) {
			continue
		}
		sigma := triviaUnwrap(r.Expr).(*grammar.Repeat)
		if d, ok := k.scanRun[sigma]; ok && d >= runOpen && k.srcExprs[sigma] {
			regions[r.Name] = true
		}
	}
	tried := map[string]bool{} // the rules that were made auxiliary regions once
	for hasTrivia(regions) {
		k.control.Poll()
		drop, add := k.regionCheck(regions, tried)
		if len(drop) == 0 && len(add) == 0 {
			break
		}
		for _, n := range drop {
			delete(regions, n)
		}
		for _, n := range add {
			regions[n] = false
			tried[n] = true
		}
	}
	out := map[*grammar.Repeat]bool{}
	for n, trivia := range regions {
		if trivia {
			out[triviaUnwrap(k.byName[n].Expr).(*grammar.Repeat)] = true
		}
	}
	return out
}

func hasTrivia(regions map[string]bool) bool {
	for _, t := range regions {
		if t {
			return true
		}
	}
	return false
}

// regionFree reports whether a rule satisfies (D): it and the rules it reaches read and bind no
// variable and have no predicate, action, call of a level, rule of a left-recursion cycle,
// attributed expression, recovery, stream or Operators Rule. Then nothing but the input and the
// position decides whether an expression in them matches and where it ends: no variable, capture
// or level enters, no growth is open or starts inside, and a cut acts within the evaluation of its
// rule.
func (k *classifier) regionFree(name string) bool {
	for n := range k.reachable(name) {
		d := k.byName[n]
		if d.Action != nil || k.an.Cyclic[n] || len(k.vars[n]) > 0 {
			return false
		}
		free := true
		grammaranalysis.WalkExpr(d.Expr, func(e grammar.Expr) {
			// Attribute normalization has made `#recover` and `#stream` attributed expressions, and
			// the lowering of inline actions has made them rules with actions, so no recovery,
			// stream or inline action node is left here.
			switch e := e.(type) {
			case *grammar.Predicate, *grammar.Attributed, *grammar.OperatorsRule:
				free = false
			case *grammar.Ref:
				if e.Level != "" {
					free = false
				}
			}
		})
		if !free {
			return false
		}
	}
	return true
}

// regionCheck checks the conditions (N), (O) and (G) on the regions. It returns the regions that
// fail one, and the rules that hold an unguarded crossing of a trivia region and can be auxiliary
// regions instead, such as a string whose body holds blanks. A rule is made an auxiliary region
// once at most, so the check ends.
func (k *classifier) regionCheck(regions map[string]bool, tried map[string]bool) (drop, add []string) {
	names := sortedKeys(regions)
	// (N): the rules that the body of each region reaches, and the rules that the start rule
	// reaches without entering a region.
	inner := map[string]map[string]bool{}
	for _, n := range names {
		in := map[string]bool{}
		for _, c := range k.calls[n] {
			for m := range k.reachable(c) {
				in[m] = true
			}
		}
		inner[n] = in
	}
	outer := map[string]bool{}
	var visit func(n string)
	visit = func(n string) {
		if outer[n] || k.byName[n] == nil {
			return
		}
		outer[n] = true
		if _, ok := regions[n]; ok {
			return
		}
		for _, c := range k.calls[n] {
			visit(c)
		}
	}
	visit(k.start)
	for _, n := range names {
		for m := range inner[n] {
			if _, ok := regions[m]; ok || outer[m] {
				drop = append(drop, n)
				break
			}
		}
	}
	if len(drop) > 0 {
		return drop, nil
	}
	// (O): the first characters of distinct regions are disjoint. Of two trivia rules that overlap
	// both are dropped; an auxiliary region gives way to a trivia rule.
	nfa := k.regionNFA(names, inner)
	for i, a := range names {
		for _, b := range names[i+1:] {
			if !nfa.opener[a].intersects(nfa.opener[b]) {
				continue
			}
			for _, n := range []string{a, b} {
				if regions[n] == (regions[a] && regions[b]) {
					drop = append(drop, n)
				}
			}
		}
	}
	if len(drop) > 0 {
		return slices.Compact(sortedStrings(drop)), nil
	}
	// (G): every crossing is guarded.
	g := k.regionGuards(regions, names, outer, nfa)
	failed := map[string]bool{}
	adds := map[string]bool{}
	for _, v := range g.violations {
		if !regions[v.region] {
			failed[v.region] = true
			continue
		}
		if m := v.rule; !tried[m] && m != k.start && k.regionFree(m) {
			if _, isRegion := regions[m]; !isRegion {
				adds[m] = true
				continue
			}
		}
		failed[v.region] = true
	}
	if len(failed) > 0 {
		return sortedKeys(failed), nil
	}
	return nil, sortedKeys(adds)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

func sortedStrings(s []string) []string {
	slices.Sort(s)
	return s
}

// bitset is a set of small integers.
type bitset []uint64

func newBitset(n int) bitset { return make(bitset, (n+63)/64) }

func (b bitset) set(i int)      { b[i/64] |= 1 << (i % 64) }
func (b bitset) has(i int) bool { return b[i/64]&(1<<(i%64)) != 0 }

// or adds c to b and reports whether b changed.
func (b bitset) or(c bitset) bool {
	changed := false
	for i := range b {
		if v := b[i] | c[i]; v != b[i] {
			b[i] = v
			changed = true
		}
	}
	return changed
}

func (b bitset) and(c bitset) bitset {
	out := slices.Clone(b)
	for i := range out {
		out[i] &= c[i]
	}
	return out
}

func (b bitset) empty() bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

func (b bitset) each(f func(int)) {
	for i, v := range b {
		for v != 0 {
			j := bits.TrailingZeros64(v)
			f(64*i + j)
			v &^= 1 << j
		}
	}
}

// prefixNFA over-approximates the matches of the region bodies as a Glushkov automaton: a position
// is one character of a terminal outside lookaheads, and an edge joins two positions that can
// consume consecutive characters of a match. Lookaheads and the other expressions that consume
// nothing are read as matching the empty string, alternatives as a union and repetitions as any
// number of iterations, so every match of a region body spells a path from a first position, and
// the characters that can follow a prefix of a match are found by following its characters. A
// rule that two places call joins what follows both, which only adds paths.
type prefixNFA struct {
	k      *classifier
	chars  []charset            // the characters of each position
	lits   map[grammar.Expr]int // the first position of each terminal
	rules  map[string]nfaInfo
	next   []bitset
	start  map[string]bitset  // the first positions of the body of each region
	opener map[string]charset // the first characters of the body of each region
	size   int
}

type nfaInfo struct {
	first, last bitset
	null        bool
}

// regionNFA builds the automaton of the bodies of the regions names and the rules they reach.
func (k *classifier) regionNFA(names []string, inner map[string]map[string]bool) *prefixNFA {
	n := &prefixNFA{k: k, lits: map[grammar.Expr]int{}, rules: map[string]nfaInfo{}, start: map[string]bitset{}, opener: map[string]charset{}}
	var rules []string
	seen := map[string]bool{}
	for _, r := range names {
		for _, m := range append([]string{r}, sortedKeys(inner[r])...) {
			if !seen[m] {
				seen[m] = true
				rules = append(rules, m)
			}
		}
	}
	// Position 0 stands for any expression that the conditions keep out of a region: it consumes
	// any characters in any order.
	n.chars = append(n.chars, allChars)
	for _, r := range rules {
		n.number(k.byName[r].Expr)
	}
	n.size = len(n.chars)
	for _, r := range rules {
		n.rules[r] = nfaInfo{newBitset(n.size), newBitset(n.size), false}
	}
	for changed := true; changed; {
		changed = false
		for _, r := range rules {
			k.control.Poll()
			info := n.info(k.byName[r].Expr, false)
			old := n.rules[r]
			c := old.first.or(info.first)
			c = old.last.or(info.last) || c
			if info.null && !old.null {
				old.null, c = true, true
			}
			n.rules[r] = old
			changed = changed || c
		}
	}
	n.next = make([]bitset, n.size)
	for i := range n.next {
		n.next[i] = newBitset(n.size)
	}
	n.next[0].set(0)
	for _, r := range rules {
		n.info(k.byName[r].Expr, true)
	}
	for _, r := range names {
		s := n.rules[r].first
		n.start[r] = s
		var o charset
		s.each(func(p int) { o = o.union(n.chars[p]) })
		n.opener[r] = o
	}
	return n
}

// number gives a position to each character of each terminal of e outside lookaheads.
func (n *prefixNFA) number(e grammar.Expr) {
	switch e := e.(type) {
	case *grammar.Literal:
		n.lits[e] = len(n.chars)
		for _, c := range e.Value {
			n.chars = append(n.chars, literalChar(e, c))
		}
	case *grammar.CharClass:
		n.lits[e] = len(n.chars)
		n.chars = append(n.chars, classChars(e))
	case *grammar.Any:
		n.lits[e] = len(n.chars)
		n.chars = append(n.chars, allChars)
	case *grammar.And, *grammar.Not, *grammar.Predicate:
	default:
		eachChild(e, n.number)
	}
}

// eachChild calls f on the expressions directly inside e that it evaluates as parts of its match.
func eachChild(e grammar.Expr, f func(grammar.Expr)) {
	switch e := e.(type) {
	case *grammar.Seq:
		for _, it := range e.Items {
			f(it)
		}
	case *grammar.Choice:
		for _, alt := range e.Alts {
			f(alt)
		}
	case *grammar.Repeat:
		f(e.Expr)
	case *grammar.Optional:
		f(e.Expr)
	case *grammar.Group:
		f(e.Expr)
	case *grammar.Capture:
		f(e.Expr)
	case *grammar.Discard:
		f(e.Expr)
	case *grammar.Atomic:
		f(e.Expr)
	}
}

func literalChar(l *grammar.Literal, c rune) charset {
	if l.IgnoreCase {
		return asciiFold(c)
	}
	return charOf(c)
}

// info returns the first and last positions of e and whether it can match the empty string; with
// edges, it also adds the edges inside e.
func (n *prefixNFA) info(e grammar.Expr, edges bool) nfaInfo {
	k := n.k
	k.control.Poll()
	empty := nfaInfo{newBitset(n.size), newBitset(n.size), true}
	one := func(p int) nfaInfo {
		i := nfaInfo{newBitset(n.size), newBitset(n.size), false}
		i.first.set(p)
		i.last.set(p)
		return i
	}
	switch e := e.(type) {
	case *grammar.Literal:
		if e.Value == "" {
			return empty
		}
		p := n.lits[e]
		m := utf8.RuneCountInString(e.Value)
		if edges {
			for i := p; i+1 < p+m; i++ {
				n.next[i].set(i + 1)
			}
		}
		i := one(p)
		i.last = newBitset(n.size)
		i.last.set(p + m - 1)
		return i
	case *grammar.CharClass, *grammar.Any:
		return one(n.lits[e])
	case *grammar.Ref:
		if r, ok := n.rules[e.Name]; ok {
			return r
		}
		if k.byName[e.Name] == nil {
			return empty
		}
		return n.any()
	case *grammar.Seq:
		out := empty
		out.first, out.last = newBitset(n.size), newBitset(n.size)
		last := newBitset(n.size)
		for _, it := range e.Items {
			i := n.info(it, edges)
			if edges {
				last.each(func(p int) { n.next[p].or(i.first) })
			}
			if out.null {
				out.first.or(i.first)
			}
			if i.null {
				last.or(i.last)
			} else {
				last = slices.Clone(i.last)
				out.null = false
			}
		}
		out.last = last
		return out
	case *grammar.Choice:
		out := nfaInfo{newBitset(n.size), newBitset(n.size), false}
		for _, alt := range e.Alts {
			i := n.info(alt, edges)
			out.first.or(i.first)
			out.last.or(i.last)
			out.null = out.null || i.null
		}
		return out
	case *grammar.Repeat:
		if e.Max == 0 {
			return empty
		}
		i := n.info(e.Expr, edges)
		if edges && e.Max != 1 {
			i.last.each(func(p int) { n.next[p].or(i.first) })
		}
		return nfaInfo{i.first, i.last, e.Min == 0 || i.null}
	case *grammar.Optional:
		i := n.info(e.Expr, edges)
		return nfaInfo{i.first, i.last, true}
	case *grammar.Group:
		return n.info(e.Expr, edges)
	case *grammar.Capture:
		return n.info(e.Expr, edges)
	case *grammar.Discard:
		return n.info(e.Expr, edges)
	case *grammar.Atomic:
		return n.info(e.Expr, edges)
	case *grammar.And, *grammar.Not, *grammar.Predicate, *grammar.Cut, *grammar.Top, *grammar.Bottom,
		*grammar.BeginInput, *grammar.EndInput, *grammar.BeginLine, *grammar.EndLine:
		return empty
	}
	// What (D) keeps out of a region: any characters.
	return n.any()
}

func (n *prefixNFA) any() nfaInfo {
	i := nfaInfo{newBitset(n.size), newBitset(n.size), true}
	i.first.set(0)
	i.last.set(0)
	return i
}

// follow returns the characters that can follow the characters cs at the start of a match of the
// body of the region r, one set for each character: the next character inside the match, when the
// match goes on after them.
func (n *prefixNFA) follow(r string, cs []charset) charset {
	s := n.start[r]
	for i, c := range cs {
		if i > 0 {
			s = n.succ(s)
		}
		t := newBitset(n.size)
		s.each(func(p int) {
			if n.chars[p].intersects(c) {
				t.set(p)
			}
		})
		if t.empty() {
			return nil
		}
		s = t
	}
	var out charset
	n.succ(s).each(func(p int) { out = out.union(n.chars[p]) })
	return out
}

func (n *prefixNFA) succ(s bitset) bitset {
	out := newBitset(n.size)
	s.each(func(p int) { out.or(n.next[p]) })
	return out
}

// terminalChars returns the characters that a terminal consumes, one set for each character.
func terminalChars(e grammar.Expr) []charset {
	switch e := e.(type) {
	case *grammar.Literal:
		var out []charset
		for _, c := range e.Value {
			out = append(out, literalChar(e, c))
		}
		return out
	case *grammar.CharClass:
		return []charset{classChars(e)}
	case *grammar.Any:
		return []charset{allChars}
	}
	return nil
}

// regionViolation is a crossing of a region that is not guarded, in the body of rule.
type regionViolation struct{ region, rule string }

// guardPass is the forward pass of the settled regions over the outer rules, and the check of the
// crossings. A state holds the trivia regions that end where the parse is: at every evaluation of
// an expression with a region in its state, an evaluation of that region matches up to its
// position. A rule starts with the regions that all its calls hold, and a call of it ends with
// those that its body ends with.
type guardPass struct {
	k          *classifier
	regions    map[string]bool
	names      []string
	index      map[string]int // the bit of each trivia region
	width      int
	entry      map[string]bitset
	exit       map[string]bitset
	outer      map[string]bool
	nfa        *prefixNFA
	changed    bool
	checking   bool
	rule       string
	violations []regionViolation
	seen       map[regionViolation]bool
}

func (k *classifier) regionGuards(regions map[string]bool, names []string, outer map[string]bool, nfa *prefixNFA) *guardPass {
	g := &guardPass{k: k, regions: regions, names: names, index: map[string]int{}, entry: map[string]bitset{},
		exit: map[string]bitset{}, outer: outer, nfa: nfa, seen: map[regionViolation]bool{}}
	for _, n := range names {
		if regions[n] {
			g.index[n] = g.width
			g.width++
		}
	}
	top := g.top()
	for n := range outer {
		if _, ok := regions[n]; !ok {
			g.entry[n], g.exit[n] = top, top
		}
	}
	g.entry[k.start] = g.none()
	for g.changed = true; g.changed; {
		g.changed = false
		g.each(func(r *grammar.RuleDef) {
			if out := g.walk(r.Expr, g.entry[r.Name], nil).and(g.exit[r.Name]); !slices.Equal(out, g.exit[r.Name]) {
				g.exit[r.Name] = out
				g.changed = true
			}
		})
	}
	g.checking = true
	g.each(func(r *grammar.RuleDef) {
		g.rule = r.Name
		g.walk(r.Expr, g.entry[r.Name], nil)
	})
	return g
}

// each calls f on the outer rules that are not regions, in the order of the grammar.
func (g *guardPass) each(f func(*grammar.RuleDef)) {
	for _, r := range g.k.rules {
		if _, ok := g.regions[r.Name]; g.outer[r.Name] && !ok {
			g.k.control.Poll()
			f(r)
		}
	}
}

func (g *guardPass) none() bitset { return newBitset(g.width) }

func (g *guardPass) top() bitset {
	b := newBitset(g.width)
	for i := range g.width {
		b.set(i)
	}
	return b
}

// isCheck reports whether e is a check: a negative lookahead of an expression of terminals that
// calls no rule. A check is evaluated where it stands and examines a bounded amount of input.
func isCheck(e grammar.Expr) bool {
	n, ok := ungroup(e).(*grammar.Not)
	if !ok {
		return false
	}
	pure := true
	grammaranalysis.WalkExpr(n.Expr, func(x grammar.Expr) {
		switch x.(type) {
		case *grammar.Literal, *grammar.CharClass, *grammar.Any, *grammar.Seq, *grammar.Choice, *grammar.Group,
			*grammar.Optional, *grammar.Repeat:
		default:
			pure = false
		}
	})
	return pure
}

// walk carries the state in through e and returns the state after it. next is the check that
// follows e when e ends where the check is evaluated, or nil.
func (g *guardPass) walk(e grammar.Expr, in bitset, next *grammar.Not) bitset {
	k := g.k
	k.control.Poll()
	switch e := e.(type) {
	case *grammar.Literal, *grammar.CharClass, *grammar.Any:
		if g.checking {
			g.cross(e, in, next)
		}
		if l, ok := e.(*grammar.Literal); ok && l.Value == "" {
			return in
		}
		return g.none()
	case *grammar.Ref:
		if trivia, ok := g.regions[e.Name]; ok {
			out := g.none()
			if trivia {
				out.set(g.index[e.Name])
			}
			return out
		}
		if k.byName[e.Name] == nil {
			return in
		}
		if old := g.entry[e.Name]; !slices.Equal(old.and(in), old) {
			g.entry[e.Name] = old.and(in)
			g.changed = true
		}
		return g.exit[e.Name]
	case *grammar.Seq:
		for i, it := range e.Items {
			var nx *grammar.Not
			if i+1 < len(e.Items) {
				if isCheck(e.Items[i+1]) {
					nx = ungroup(e.Items[i+1]).(*grammar.Not)
				}
			} else {
				nx = next
			}
			in = g.walk(it, in, nx)
		}
		return in
	case *grammar.Choice:
		out := g.top()
		for _, alt := range e.Alts {
			out = out.and(g.walk(alt, in, next))
		}
		return out
	case *grammar.Optional:
		return in.and(g.walk(e.Expr, in, next))
	case *grammar.Repeat:
		if e.Max == 0 {
			return in
		}
		// The state at the start of an iteration holds on every iteration, so the crossings are
		// checked with it only.
		checking := g.checking
		g.checking = false
		s := in
		for {
			t := s.and(g.walk(e.Expr, s, nil))
			if slices.Equal(t, s) {
				break
			}
			s = t
		}
		g.checking = checking
		if checking {
			g.walk(e.Expr, s, nil)
		}
		return s
	case *grammar.Group:
		return g.walk(e.Expr, in, next)
	case *grammar.Capture:
		return g.walk(e.Expr, in, next)
	case *grammar.Discard:
		return g.walk(e.Expr, in, next)
	case *grammar.Atomic:
		return g.walk(e.Expr, in, next)
	case *grammar.InlineAction:
		return g.walk(e.Expr, in, next)
	case *grammar.And:
		g.walk(e.Expr, in, nil)
		return in
	case *grammar.Not:
		if !isCheck(e) {
			g.walk(e.Expr, in, nil)
		}
		return in
	case *grammar.Attributed:
		g.walk(e.Expr, in, nil)
		for _, at := range e.Attrs {
			for _, arg := range at.ExpressionArgs() {
				g.walk(arg, in, nil)
			}
		}
		return g.none()
	case *grammar.Recovery:
		g.walk(e.Expr, in, nil)
		g.walk(e.Skip, in, nil)
		return g.none()
	case *grammar.Stream:
		g.walk(e.Expr, g.none(), nil)
		return g.none()
	case *grammar.OperatorsRule:
		// Its parts follow one another in an order that the pass does not follow.
		g.walk(e.Skip, g.none(), nil)
		for _, o := range e.Operands {
			g.walk(o.Expr, g.none(), nil)
		}
		for _, l := range e.Levels {
			for _, op := range l.Operators {
				g.walk(op.Expr, g.none(), nil)
			}
		}
		return g.none()
	}
	// Predicates, anchors, cuts and the top expression consume nothing.
	return in
}

// cross checks the terminal t, whose state is in and whose next check is next, against each
// region: (G1) a character of the opening characters of a region only as its first character, and
// (G2) a first character of the region only where the region ended, or where the next character
// cannot go on with the match of the region, or before a check that fails wherever it can.
func (g *guardPass) cross(t grammar.Expr, in bitset, next *grammar.Not) {
	cs := terminalChars(t)
	if len(cs) == 0 {
		return
	}
	for _, r := range g.names {
		o := g.nfa.opener[r]
		ok := true
		for _, c := range cs[1:] {
			if c.intersects(o) {
				ok = false
			}
		}
		if ok && cs[0].intersects(o) {
			ok = g.regions[r] && in.has(g.index[r])
			if !ok {
				f := g.nfa.follow(r, cs)
				ok = len(f) == 0 || next != nil && g.k.subset(f, g.k.sure(next.Expr))
			}
		}
		if v := (regionViolation{r, g.rule}); !ok && !g.seen[v] {
			g.seen[v] = true
			g.violations = append(g.violations, v)
		}
	}
}
