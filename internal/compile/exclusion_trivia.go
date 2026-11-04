package compile

import (
	"maps"
	"slices"
	"strings"

	"zeroparse.dev/zp/grammar"
)

// Trivia rules are rules whose shape lets the analysis compare their calls by position:
//
//   - the body is a greedy repetition X*, an atomic X*, or the unrolled form X* (Y X*)* where X
//     cannot match empty (triviaParts), so a call always succeeds, and a second call where the
//     first ended matches nothing, as X fails there again;
//   - neither the rule nor a rule that it calls runs an action or contains an attributed
//     expression, a recovery, a stream, an Operators Rule or a call of a left-recursion cycle,
//     the body binds no variable that the rule reads, and the body has no #cut of its own
//     (triviaWhy). Then the result depends only on the position and on the variables that the
//     rule reads, and two calls with the same variables end alike (law P2).
//
// triviaOn makes the exclusion analysis rely on it: two calls of one trivia rule at one position
// are passed without counting toward the bound, and a call that follows a call of the same
// trivia rule across symbols that consume nothing is dropped. In strict mode both need a cost
// bound: the second call at one position is cheap only where the memo answers it, which
// triviaMemo provides by treating the trivia rules as memoized per position, and a dropped call,
// which starts where the first ended and repeats its final, failing attempt of X, only when that
// attempt contains no scanning source (triviaSettles), so that it costs a constant.
//
// triviaOnly, when not nil, limits the trivia rules to the names that it lists. All are test
// hooks.
var (
	triviaOn   bool
	triviaOnly map[string]bool
	triviaMemo bool
)

// triviaParts returns the expressions whose attempts a trivia rule with body e makes after its
// last iteration, or nil when e does not have the shape of a trivia rule: X for a greedy
// repetition X* or an atomic X*, and X and the iteration (Y X*) of the unrolled form
// X* (Y X*)*, where X cannot match without consuming input. After the unrolled
// form, X fails where the last X* stopped, and the outer repetition stopped where its iteration
// failed or matched nothing, so a second evaluation matches nothing as well.
func (k *classifier) triviaParts(e grammar.Expr) []grammar.Expr {
	e = triviaUnwrap(e)
	if star(e) {
		return []grammar.Expr{e.(*grammar.Repeat).Expr}
	}
	s, ok := e.(*grammar.Seq)
	if !ok || len(s.Items) != 2 || !star(triviaUnwrap(s.Items[0])) {
		return nil
	}
	first := triviaUnwrap(s.Items[0]).(*grammar.Repeat)
	r, ok := triviaUnwrap(s.Items[1]).(*grammar.Repeat)
	if !ok || !star(r) || k.nullable(first.Expr) {
		return nil
	}
	inner, ok := triviaUnwrap(r.Expr).(*grammar.Seq)
	if !ok || len(inner.Items) < 2 ||
		grammar.FormatExpr(triviaUnwrap(inner.Items[len(inner.Items)-1])) != grammar.FormatExpr(first) {
		return nil
	}
	return []grammar.Expr{first.Expr, r.Expr}
}

func star(e grammar.Expr) bool {
	r, ok := e.(*grammar.Repeat)
	return ok && r.Min == 0 && r.Max < 0
}

func triviaUnwrap(e grammar.Expr) grammar.Expr {
	for {
		switch x := e.(type) {
		case *grammar.Group:
			e = x.Expr
		case *grammar.Atomic:
			e = x.Expr
		default:
			return e
		}
	}
}

// triviaWhy returns "" when the rule name is a trivia rule, and otherwise why it is not.
//
// Reads of variables, captures and predicates are allowed. A body that binds a variable that the
// rule reads is not: then the first call can stop where its own binding makes X fail, and the
// second call, which starts with the variables of the caller, can go on. The comparison treats
// calls of a rule that reads variables as equal only where both sides have the same variables,
// as for any other call.
func (k *classifier) triviaWhy(name string) string {
	def := k.byName[name]
	if def == nil {
		return "not a rule"
	}
	// The first rule, in name order, with each kind of reason; the reasons in a fixed order.
	order := []string{"binds a variable that it reads", "action", "left recursion", "#cut",
		"attributed, recovery, stream or Operators Rule", "operator level call"}
	at := map[string]string{}
	note := func(why, n string) {
		if _, ok := at[why]; !ok {
			at[why] = n
		}
	}
	reads := map[string]bool{}
	for _, v := range k.vars[name] {
		reads[v] = true
	}
	WalkExpr(def.Expr, func(e grammar.Expr) {
		switch e := e.(type) {
		case *grammar.Cut:
			// A cut of the body itself can commit the repetition, which then fails.
			note("#cut", name)
		case *grammar.Predicate:
			WalkTerm(e.Term, func(t grammar.Term) {
				if b, ok := t.(*grammar.Bind); ok && reads[b.Name] {
					note("binds a variable that it reads", name)
				}
			})
		}
	})
	for _, n := range slices.Sorted(maps.Keys(k.reachable(name))) {
		d := k.byName[n]
		if d.Action != nil {
			note("action", n)
		}
		if k.an.cyclic[n] {
			note("left recursion", n)
		}
		WalkExpr(d.Expr, func(e grammar.Expr) {
			switch e := e.(type) {
			case *grammar.InlineAction:
				note("action", n)
			case *grammar.Attributed, *grammar.Recovery, *grammar.Stream, *grammar.OperatorsRule:
				note("attributed, recovery, stream or Operators Rule", n)
			case *grammar.Ref:
				if e.Level != "" {
					note("operator level call", n)
				}
			}
		})
	}
	var whys []string
	if k.triviaParts(def.Expr) == nil {
		whys = append(whys, "body is not X*")
	}
	for _, why := range order {
		if n, ok := at[why]; ok {
			whys = append(whys, why+" ("+n+")")
		}
	}
	return strings.Join(whys, "; ")
}

// isTrivia reports whether r calls a trivia rule that the hooks enable.
func (k *classifier) isTrivia(r *grammar.Ref) bool {
	if !triviaOn && !triviaMemo || r.Level != "" || triviaOnly != nil && !triviaOnly[r.Name] {
		return false
	}
	if k.trivia == nil {
		k.trivia = map[string]bool{}
	}
	v, ok := k.trivia[r.Name]
	if !ok {
		v = k.triviaWhy(r.Name) == ""
		k.trivia[r.Name] = v
	}
	return v
}

// triviaCall reports whether s is a call of a trivia rule that the analysis relies on.
func (x excl) triviaCall(s sym) bool {
	r, ok := s.e.(*grammar.Ref)
	return triviaOn && ok && s.kind == symMatch && x.k.isTrivia(r)
}

// triviaSettles reports whether a call of the trivia rule of s, where a call of it ended, costs a
// constant: it makes only the final attempts of the parts of the body, which fail, and those
// contain no scanning source, so Lemma 1 bounds them. A part that scans, such as a comment, can
// scan an unterminated comment again before it fails.
func (x excl) triviaSettles(s sym) bool {
	for _, part := range x.k.triviaParts(x.k.byName[s.e.(*grammar.Ref).Name].Expr) {
		if x.k.containsSource(part) {
			return false
		}
	}
	return true
}

// collapseTrivia drops from p the calls of the trivia rule of p[i] that follow it across symbols
// that consume nothing: each is evaluated where p[i] ended and matches nothing, with the
// variables of p[i] unless a binding lies between them, which stops the dropping for a rule that
// reads variables.
func (x excl) collapseTrivia(p []sym, i int) []sym {
	name := p[i].e.(*grammar.Ref).Name
	reads := len(x.k.vars[name]) > 0
	for m := i + 1; m < len(p); m++ {
		switch p[m].kind {
		case symBind:
			if reads {
				return p
			}
			continue
		case symSkip, symEnv, symNot, symAnd, symUp:
			continue
		case symMatch:
			if r, ok := p[m].e.(*grammar.Ref); ok && r.Name == name && r.Level == "" {
				p = append(p[:m:m], p[m+1:]...)
				m--
				continue
			}
		}
		break
	}
	return p
}
