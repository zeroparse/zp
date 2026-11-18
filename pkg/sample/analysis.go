package sample

import (
	"cmp"
	"slices"
	"sort"
	"unicode"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

// inf is the height or length of an expression that can never match.
const inf = 1 << 30

// Kinds of coverage targets.
const (
	targetRule = iota
	targetAlt
	targetOperand
	targetOperator
)

// target is something the generator tries to exercise: a rule, an alternative of an ordered choice, or
// an operand or operator of an Operators Rule expression.
type target struct {
	kind  int
	rule  string
	index int // alternative, operand or operator index
	expr  ast.Expr
	// possible is false for targets that can never be part of a match: they can never match
	// themselves, or only occur inside an expression that can never match ("(" ("a" / "b") _|_).
	possible bool
}

// ruleInfo is what the generator knows about a rule.
type ruleInfo struct {
	def           *ast.RuleDef
	makesNode     bool // the declared type is a named node type
	index         int
	operatorsRule *ast.OperatorsRule
	levels        map[string]int // level name -> index in operatorsRule.Levels
	height        int            // minimal derivation height: nested rule calls needed to match it
	length        int            // minimal length in bytes of a match
	reach         bitset         // targets that generating the rule can exercise, its own included
	calls         []string
	// always caches alwaysMatches for the body: 0 not computed yet, 1 being computed, 2 yes, 3 no.
	always int
	// stops caches static longest-selection queries per minimum level, never per live-scope state
	// (see gen.operatorsRuleStopCheck).
	stops map[int]*operatorsRuleStop
}

// info is the analysis of a grammar shared by the generator and the matcher.
type info struct {
	rules   map[string]*ruleInfo
	order   []*ruleInfo
	start   *ruleInfo
	targets []target
	// Target ids of the first alternative of each choice, of each Operators Rule operand and operator.
	altBase  map[*ast.Choice]int
	operands map[*ast.OperatorsRuleOperand]int
	ops      map[*ast.OperatorsRuleOperator]int
	// reachable[i] is true if the rule with index i can be called from the start rule in a context
	// that can match; called[i] if it is called at all (outside negative lookaheads and #recover).
	reachable, called []bool
	reachMemo         map[ast.Expr]bitset
	captures          map[ast.Expr]bool                  // memo of hasCaptures
	classes           map[*ast.CharClass][]ast.CharRange // memo of classRanges
	// compared holds the captures of rule calls that a predicate in the same rule reads.
	compared map[*ast.Capture]bool
	// alphabet holds the literal strings and sample characters of the grammar, for mutations.
	alphabet []string
	// terms holds what predicates read of the grammar, and the declared variables with their initial
	// values, from which every attempt starts.
	terms *terms
}

func analyze(g *ast.Grammar, start string) *info {
	in := &info{
		rules:     map[string]*ruleInfo{},
		altBase:   map[*ast.Choice]int{},
		operands:  map[*ast.OperatorsRuleOperand]int{},
		ops:       map[*ast.OperatorsRuleOperator]int{},
		reachMemo: map[ast.Expr]bitset{},
		compared:  map[*ast.Capture]bool{},
		captures:  map[ast.Expr]bool{},
		classes:   map[*ast.CharClass][]ast.CharRange{},
	}
	for i, r := range g.Rules() {
		ri := &ruleInfo{def: r, index: i, height: inf, length: inf, stops: map[int]*operatorsRuleStop{}}
		if pr, _ := ast.RuleOperators(r.Expr); pr != nil {
			ri.operatorsRule = pr
			ri.levels = map[string]int{}
			for j, l := range pr.Levels {
				if l.Name != "" {
					ri.levels[l.Name] = j
				}
			}
		}
		in.rules[r.Name] = ri
		in.order = append(in.order, ri)
		in.targets = append(in.targets, target{kind: targetRule, rule: r.Name})
	}
	in.start = in.rules[start]
	in.terms = newTerms(g)
	for _, ri := range in.order {
		ri.makesNode = in.terms.makesNode(ri.def)
	}
	// Targets inside rule bodies.
	for _, ri := range in.order {
		walk(ri.def.Expr, func(e ast.Expr) {
			switch e := e.(type) {
			case *ast.Ref:
				ri.calls = append(ri.calls, e.Name)
			case *ast.Choice:
				in.altBase[e] = len(in.targets)
				for j, a := range e.Alts {
					in.targets = append(in.targets, target{kind: targetAlt, rule: ri.def.Name, index: j, expr: a})
				}
			case *ast.OperatorsRule:
				for j, o := range e.Operands {
					in.operands[o] = len(in.targets)
					in.targets = append(in.targets, target{kind: targetOperand, rule: ri.def.Name, index: j, expr: o.Expr})
				}
				j := 0
				for _, l := range e.Levels {
					for _, op := range l.Operators {
						in.ops[op] = len(in.targets)
						in.targets = append(in.targets, target{kind: targetOperator, rule: ri.def.Name, index: j, expr: op.Expr})
						j++
					}
				}
			case *ast.Literal:
				if e.Value != "" {
					in.alphabet = append(in.alphabet, e.Value)
				}
			case *ast.CharClass:
				if !e.Negated {
					for _, r := range e.Ranges {
						in.alphabet = append(in.alphabet, string(r.Lo), string(r.Hi))
					}
				}
			}
		})
	}
	in.alphabet = dedupe(in.alphabet)
	for _, ri := range in.order {
		read := map[string]bool{}
		walk(ri.def.Expr, func(e ast.Expr) {
			if p, ok := e.(*ast.Predicate); ok {
				walkTerm(p.Action, func(t ast.ActionExpr) {
					if c, ok := t.(*ast.CaptureRef); ok {
						read[c.Name] = true
					}
				})
			}
		})
		walk(ri.def.Expr, func(e ast.Expr) {
			if c, ok := e.(*ast.Capture); ok && read[c.Name] {
				in.compared[c] = true
			}
		})
	}

	// Resolve callees first and revisit only callers of changed estimates.
	deps := in.dependencies()
	deps.fixedPoint(in, func(ri *ruleInfo) bool {
		h, l := in.height(ri.def.Expr), in.length(ri.def.Expr)
		if h >= ri.height && l >= ri.length {
			return false
		}
		ri.height, ri.length = min(h, ri.height), min(l, ri.length)
		return true
	})
	// Reach sets of rules: their own targets and those of the rules they call, transitively.
	for _, ri := range in.order {
		ri.reach = newBitset(len(in.targets))
		ri.reach.set(ri.index) // the target of a rule is its index
		in.localTargets(ri.def.Expr, ri.reach)
	}
	deps.fixedPoint(in, func(ri *ruleInfo) bool {
		changed := false
		for _, c := range ri.calls {
			if callee := in.rules[c]; callee != nil && ri.reach.union(callee.reach) {
				changed = true
			}
		}
		return changed
	})
	in.called = in.closure(func(ri *ruleInfo) []string { return ri.calls })
	in.possibleTargets()
	return in
}

// localTargets adds the targets inside e, not counting the rules it calls, to s.
func (in *info) localTargets(e ast.Expr, s bitset) {
	walk(e, func(x ast.Expr) {
		switch x := x.(type) {
		case *ast.Choice:
			for j := range x.Alts {
				s.set(in.altBase[x] + j)
			}
		case *ast.OperatorsRule:
			for _, o := range x.Operands {
				s.set(in.operands[o])
			}
			for _, l := range x.Levels {
				for _, op := range l.Operators {
					s.set(in.ops[op])
				}
			}
		}
	})
}

// alwaysMatches reports whether e matches at any position of any input, if only the empty string: an
// optional expression, a repetition with minimum 0, and sequences and choices of such. Cuts (which can
// make an optional expression fail), lookaheads and predicates make it false.
func (in *info) alwaysMatches(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Top:
		return true
	case *ast.Literal:
		return e.Value == ""
	case *ast.Optional:
		return !in.hasCut(e.Expr)
	case *ast.Repeat:
		return e.Min == 0 && !in.hasCut(e.Expr) || e.Min == 1 && in.alwaysMatches(e.Expr)
	case *ast.Seq:
		for _, it := range e.Items {
			if !in.alwaysMatches(it) {
				return false
			}
		}
		return true
	case *ast.Choice:
		for _, a := range e.Alts {
			if in.alwaysMatches(a) {
				return !in.hasCut(e)
			}
		}
		return false
	case *ast.Atomic:
		return in.alwaysMatches(e.Expr)
	case *ast.Discard:
		return in.alwaysMatches(e.Expr)
	case *ast.Capture:
		return in.alwaysMatches(e.Expr)
	case *ast.Attributed:
		return in.alwaysMatches(e.Expr)
	case *ast.Ref:
		ri := in.rules[e.Name]
		if ri == nil || ri.operatorsRule != nil {
			return false
		}
		switch ri.always {
		case 0:
			ri.always = 1 // a recursive call does not count
			v := in.alwaysMatches(ri.def.Expr)
			ri.always = 3
			if v {
				ri.always = 2
			}
			return v
		case 2:
			return true
		}
	}
	return false
}

// binds reports whether e makes captures or defines variables, not counting called rules (whose
// captures and variables are not visible to the caller).
func (in *info) binds(e ast.Expr) bool {
	b := false
	walk(e, func(x ast.Expr) {
		switch x := x.(type) {
		case *ast.Capture:
			b = true
		case *ast.Predicate:
			if _, ok := x.Action.(*ast.Bind); ok {
				b = true
			}
		}
	})
	return b
}

// classRanges returns the code points that the class matches as sorted, disjoint ranges of valid code
// points: the complement for a negated class, and without the surrogates, which no input can contain.
func (in *info) classRanges(c *ast.CharClass) []ast.CharRange {
	if rs, ok := in.classes[c]; ok {
		return rs
	}
	rs := append([]ast.CharRange(nil), c.Ranges...)
	slices.SortFunc(rs, func(a, b ast.CharRange) int { return cmp.Compare(a.Lo, b.Lo) })
	var merged []ast.CharRange
	for _, r := range rs {
		if n := len(merged); n > 0 && r.Lo <= merged[n-1].Hi+1 {
			merged[n-1].Hi = max(merged[n-1].Hi, r.Hi)
		} else {
			merged = append(merged, r)
		}
	}
	if c.Negated {
		var comp []ast.CharRange
		next := rune(0)
		for _, r := range merged {
			if r.Lo > next {
				comp = append(comp, ast.CharRange{Lo: next, Hi: r.Lo - 1})
			}
			next = max(next, r.Hi+1)
		}
		if next <= unicode.MaxRune {
			comp = append(comp, ast.CharRange{Lo: next, Hi: unicode.MaxRune})
		}
		merged = comp
	}
	var valid []ast.CharRange
	for _, r := range merged {
		r.Hi = min(r.Hi, unicode.MaxRune)
		for _, part := range []ast.CharRange{{Lo: r.Lo, Hi: min(r.Hi, 0xD7FF)}, {Lo: max(r.Lo, 0xE000), Hi: r.Hi}} {
			if part.Lo <= part.Hi {
				valid = append(valid, part)
			}
		}
	}
	in.classes[c] = valid
	return valid
}

// hasCut reports whether e contains a cut, not counting called rules (a cut does not reach beyond its
// rule).
func (in *info) hasCut(e ast.Expr) bool {
	cut := false
	walk(e, func(x ast.Expr) {
		if _, ok := x.(*ast.Cut); ok {
			cut = true
		}
	})
	return cut
}

// hasCaptures reports whether e contains a capture, not counting called rules.
func (in *info) hasCaptures(e ast.Expr) bool {
	if v, ok := in.captures[e]; ok {
		return v
	}
	v := false
	walk(e, func(x ast.Expr) {
		if _, ok := x.(*ast.Capture); ok {
			v = true
		}
	})
	in.captures[e] = v
	return v
}

// reach returns the targets that generating e can exercise.
func (in *info) reach(e ast.Expr) bitset {
	if s, ok := in.reachMemo[e]; ok {
		return s
	}
	s := newBitset(len(in.targets))
	in.localTargets(e, s)
	walk(e, func(x ast.Expr) {
		if r, ok := x.(*ast.Ref); ok {
			if ri := in.rules[r.Name]; ri != nil {
				s.union(ri.reach)
			}
		}
	})
	in.reachMemo[e] = s
	return s
}

// height returns the minimal number of nested rule calls needed to match e, or inf if e can never
// match, using the current estimates of the rules' heights.
func (in *info) height(e ast.Expr) int {
	switch e := e.(type) {
	case *ast.Ref:
		if ri := in.rules[e.Name]; ri != nil && ri.height < inf {
			return ri.height + 1
		}
		return inf
	case *ast.Bottom:
		return inf
	case *ast.Seq:
		h := 0
		for _, it := range e.Items {
			h = max(h, in.height(it))
		}
		return h
	case *ast.Choice:
		h := inf
		for _, a := range e.Alts {
			h = min(h, in.height(a))
		}
		return h
	case *ast.Repeat:
		if e.Min == 0 {
			return 0
		}
		return in.height(e.Expr)
	case *ast.Optional, *ast.Not:
		return 0
	case *ast.And:
		return in.height(e.Expr)
	case *ast.Atomic:
		return in.height(e.Expr)
	case *ast.Discard:
		return in.height(e.Expr)
	case *ast.Capture:
		return in.height(e.Expr)
	case *ast.Attributed:
		return in.height(e.Expr)
	case *ast.OperatorsRule:
		h := inf
		for _, o := range e.Operands {
			h = min(h, in.height(o.Expr))
		}
		return h
	}
	return 0
}

// length returns the minimal length in bytes of a match of e, or inf if e can never match.
func (in *info) length(e ast.Expr) int {
	switch e := e.(type) {
	case *ast.Ref:
		if ri := in.rules[e.Name]; ri != nil {
			return ri.length
		}
		return inf
	case *ast.Literal:
		return len(e.Value)
	case *ast.CharClass, *ast.Any:
		return 1
	case *ast.Bottom:
		return inf
	case *ast.Seq:
		l := 0
		for _, it := range e.Items {
			l = int(min(uint(inf), uint(l)+uint(in.length(it))))
		}
		return l
	case *ast.Choice:
		l := inf
		for _, a := range e.Alts {
			l = min(l, in.length(a))
		}
		return l
	case *ast.Repeat:
		if e.Min == 0 {
			return 0
		}
		// Clamp both nonnegative factors before multiplying: their product
		// then fits int64 even when a native int is only 32 bits wide.
		return int(min(int64(inf), int64(min(inf, e.Min))*int64(min(inf, in.length(e.Expr)))))
	case *ast.Optional, *ast.Not, *ast.And:
		return 0
	case *ast.Atomic:
		return in.length(e.Expr)
	case *ast.Discard:
		return in.length(e.Expr)
	case *ast.Capture:
		return in.length(e.Expr)
	case *ast.Attributed:
		return in.length(e.Expr)
	case *ast.OperatorsRule:
		l := inf
		for _, o := range e.Operands {
			l = min(l, in.length(o.Expr))
		}
		return l
	}
	return 0
}

// possibleTargets propagates reachable rules and their weakest OperatorsRule entry together. Only parts
// enabled in a reachable context can introduce more calls: a blocked tail must not lower its own
// rule's entry or make its nested choices and callees appear reachable. Prefixes are unrestricted and
// can open weaker tail levels in their RHS, independently of the named entry.
func (in *info) possibleTargets() {
	type state struct {
		minLevel int
		queued   bool
	}
	states := make([]state, len(in.order))
	for i := range states {
		states[i].minLevel = inf
	}
	in.reachable = make([]bool, len(in.order))
	queue := make([]int, 0, len(in.order))
	enqueue := func(ri *ruleInfo, level int) {
		if ri == nil || ri.height >= inf || level >= states[ri.index].minLevel {
			return
		}
		s := &states[ri.index]
		s.minLevel = level
		if !s.queued {
			s.queued = true
			queue = append(queue, ri.index)
		}
	}
	visit := func(e ast.Expr) {
		switch e := e.(type) {
		case *ast.Ref:
			if callee := in.rules[e.Name]; callee != nil {
				enqueue(callee, callee.levels[e.Level]) // zero for unrestricted/ordinary calls
			}
		case *ast.Choice:
			for j, a := range e.Alts {
				in.targets[in.altBase[e]+j].possible = in.height(a) < inf
			}
		}
	}
	enqueue(in.start, 0)
	for len(queue) > 0 {
		i := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		states[i].queued = false
		ri := in.order[i]
		in.reachable[i] = true
		in.targets[i].possible = true
		if ri.operatorsRule == nil {
			in.walkPossible(ri.def.Expr, visit)
			continue
		}
		pr := ri.operatorsRule
		minLevel := states[i].minLevel
		for l, level := range pr.Levels {
			for _, op := range level.Operators {
				if op.Kind == ast.Prefix && in.height(op.Expr) < inf {
					minLevel = min(minLevel, l+1)
				}
			}
		}
		in.walkPossible(pr.Skip, visit)
		for _, o := range pr.Operands {
			in.targets[in.operands[o]].possible = in.height(o.Expr) < inf
			in.walkPossible(o.Expr, visit)
		}
		for l, level := range pr.Levels {
			for _, op := range level.Operators {
				if op.Kind != ast.Prefix && l < minLevel {
					continue
				}
				in.targets[in.ops[op]].possible = in.height(op.Expr) < inf
				in.walkPossible(op.Expr, visit)
			}
		}
	}
}

// closure returns, for each rule, whether the start rule reaches it through the calls that calls
// returns.
func (in *info) closure(calls func(*ruleInfo) []string) []bool {
	seen := make([]bool, len(in.order))
	var visit func(ri *ruleInfo)
	visit = func(ri *ruleInfo) {
		if seen[ri.index] {
			return
		}
		seen[ri.index] = true
		for _, c := range calls(ri) {
			if callee := in.rules[c]; callee != nil {
				visit(callee)
			}
		}
	}
	if in.start != nil {
		visit(in.start)
	}
	return seen
}

// walkPossible is walk restricted to expressions that can match: it skips an expression that can never
// match and everything inside it.
func (in *info) walkPossible(e ast.Expr, f func(ast.Expr)) {
	if e == nil || in.height(e) >= inf {
		return
	}
	f(e)
	switch e := e.(type) {
	case *ast.Seq:
		for _, it := range e.Items {
			in.walkPossible(it, f)
		}
	case *ast.Choice:
		for _, a := range e.Alts {
			in.walkPossible(a, f)
		}
	case *ast.Repeat:
		in.walkPossible(e.Expr, f)
	case *ast.Optional:
		in.walkPossible(e.Expr, f)
	case *ast.And:
		in.walkPossible(e.Expr, f)
	case *ast.Atomic:
		in.walkPossible(e.Expr, f)
	case *ast.Discard:
		in.walkPossible(e.Expr, f)
	case *ast.Capture:
		in.walkPossible(e.Expr, f)
	case *ast.Attributed:
		in.walkPossible(e.Expr, f)
	case *ast.OperatorsRule:
		in.walkPossible(e.Skip, f)
		for _, o := range e.Operands {
			in.walkPossible(o.Expr, f)
		}
		for _, l := range e.Levels {
			for _, op := range l.Operators {
				in.walkPossible(op.Expr, f)
			}
		}
	}
}

// walk calls f for e and every parsing expression inside it that generation can produce: it does not
// enter called rules, negative lookaheads or attribute arguments (the skip of #recover).
func walk(e ast.Expr, f func(ast.Expr)) {
	if e == nil {
		return
	}
	f(e)
	switch e := e.(type) {
	case *ast.Seq:
		for _, it := range e.Items {
			walk(it, f)
		}
	case *ast.Choice:
		for _, a := range e.Alts {
			walk(a, f)
		}
	case *ast.Repeat:
		walk(e.Expr, f)
	case *ast.Optional:
		walk(e.Expr, f)
	case *ast.And:
		walk(e.Expr, f)
	case *ast.Not:
		// Not walked: generation never produces what a negative lookahead matches.
	case *ast.Atomic:
		walk(e.Expr, f)
	case *ast.Discard:
		walk(e.Expr, f)
	case *ast.Capture:
		walk(e.Expr, f)
	case *ast.Attributed:
		// Attribute arguments (the skip of #recover) are not walked: valid inputs never
		// exercise them.
		walk(e.Expr, f)
	case *ast.OperatorsRule:
		walk(e.Skip, f)
		for _, o := range e.Operands {
			walk(o.Expr, f)
		}
		for _, l := range e.Levels {
			for _, op := range l.Operators {
				walk(op.Expr, f)
			}
		}
	}
}

func dedupe(xs []string) []string {
	sort.Strings(xs)
	out := xs[:0]
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}

// bitset is a set of small non-negative integers.
type bitset []uint64

func newBitset(n int) bitset { return make(bitset, (n+63)/64) }

func (s bitset) set(i int)      { s[i/64] |= 1 << (i % 64) }
func (s bitset) clear(i int)    { s[i/64] &^= 1 << (i % 64) }
func (s bitset) has(i int) bool { return s[i/64]&(1<<(i%64)) != 0 }

// union adds t to s and reports whether s changed.
func (s bitset) union(t bitset) bool {
	changed := false
	for i, w := range t {
		if s[i]|w != s[i] {
			s[i] |= w
			changed = true
		}
	}
	return changed
}

// anyNotIn reports whether s has an element that is in neither a nor b.
func (s bitset) anyNotIn(a, b bitset) bool {
	for i, w := range s {
		if w&^a[i]&^b[i] != 0 {
			return true
		}
	}
	return false
}

// walkTerm calls f for t and every term inside it.
func walkTerm(t ast.ActionExpr, f func(ast.ActionExpr)) {
	if t == nil {
		return
	}
	f(t)
	switch t := t.(type) {
	case *ast.Member:
		walkTerm(t.X, f)
	case *ast.New:
		for _, fi := range t.Fields {
			walkTerm(fi.Value, f)
		}
	case *ast.TupleLit:
		for _, x := range t.Elems {
			walkTerm(x, f)
		}
	case *ast.Call:
		for _, a := range t.Args {
			walkTerm(a, f)
		}
	case *ast.Lambda:
		walkTerm(t.Body, f)
	case *ast.Binary:
		walkTerm(t.L, f)
		walkTerm(t.R, f)
	case *ast.Unary:
		walkTerm(t.X, f)
	case *ast.Cast:
		walkTerm(t.X, f)
	case *ast.Bind:
		walkTerm(t.Value, f)
	}
}
