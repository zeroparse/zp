package compile

import (
	"maps"
	"slices"
	"strings"

	"zeroparse.dev/zp/grammar"
)

// Trivia rules are rules whose shape lets the analysis compare their calls without cost:
//
//   - the body is a greedy repetition X* (through groups), so a call always succeeds, and a
//     second call where the first ended matches nothing, as X fails there again;
//   - neither the rule nor a rule that it calls reads a variable, captures, runs an action or a
//     predicate, or contains an attributed expression, a recovery, a stream, an Operators Rule
//     or a call of a left-recursion cycle, so the result depends only on the position.
//
// A trivia rule can therefore be memoized per position on every backend. With that memo, a
// second call at the position of a first costs a constant. A call where a first one ended is a
// different position: it repeats the final, failing attempt of X of the first call (such as the
// scan of an unterminated comment), so it costs no more than the first call did, not a constant.
//
// triviaOn makes the exclusion analysis rely on it: a call of a trivia rule is cheap, two calls of
// one trivia rule at one position are passed without counting toward the bound, and a call that
// follows a call of the same trivia rule across symbols that consume nothing is dropped.
// triviaOnly, when not nil, limits the trivia rules to the names that it lists. triviaMemo makes
// the classifier treat the trivia rules as memoized. All three are test hooks.
var (
	triviaOn   bool
	triviaOnly map[string]bool
	triviaMemo bool
	// triviaExtended widens the shape of a trivia rule (triviaShape).
	triviaExtended bool
)

// triviaShape reports whether e is a greedy repetition X*. With triviaExtended it also accepts an
// atomic X* and the unrolled form X* (Y X*)*: after it, X fails where the last X* stopped, and the
// outer repetition stopped where Y failed or an iteration matched nothing, so a second evaluation
// matches nothing as well.
func triviaShape(e grammar.Expr) bool {
	e = triviaUnwrap(e)
	if star(e) {
		return true
	}
	s, ok := e.(*grammar.Seq)
	if !triviaExtended || !ok || len(s.Items) != 2 || !star(triviaUnwrap(s.Items[0])) {
		return false
	}
	r, ok := triviaUnwrap(s.Items[1]).(*grammar.Repeat)
	if !ok || !star(r) {
		return false
	}
	inner, ok := triviaUnwrap(r.Expr).(*grammar.Seq)
	return ok && len(inner.Items) >= 2 &&
		grammar.FormatExpr(triviaUnwrap(inner.Items[len(inner.Items)-1])) == grammar.FormatExpr(triviaUnwrap(s.Items[0]))
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
			if !triviaExtended {
				return e
			}
			e = x.Expr
		default:
			return e
		}
	}
}

// triviaWhy returns "" when the rule name is a trivia rule, and otherwise why it is not.
func (k *classifier) triviaWhy(name string) string {
	if k.byName[name] == nil {
		return "not a rule"
	}
	// The first rule, in name order, with each kind of reason; the reasons in a fixed order.
	order := []string{"reads a variable", "action", "left recursion", "capture", "predicate",
		"attributed, recovery, stream or Operators Rule", "operator level call"}
	at := map[string]string{}
	note := func(why, n string) {
		if _, ok := at[why]; !ok {
			at[why] = n
		}
	}
	for _, n := range slices.Sorted(maps.Keys(k.reachable(name))) {
		d := k.byName[n]
		if len(k.vars[n]) > 0 {
			note("reads a variable", n)
		}
		if d.Action != nil {
			note("action", n)
		}
		if k.an.cyclic[n] {
			note("left recursion", n)
		}
		WalkExpr(d.Expr, func(e grammar.Expr) {
			switch e := e.(type) {
			case *grammar.Capture:
				note("capture", n)
			case *grammar.InlineAction:
				note("action", n)
			case *grammar.Predicate:
				note("predicate", n)
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
	if !triviaShape(k.byName[name].Expr) {
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

// collapseTrivia drops from p the calls of the trivia rule of p[i] that follow it across symbols
// that consume nothing: each is evaluated where p[i] ended and matches nothing, at the cost of the
// final attempt of X in p[i], which p[i] paid.
func (x excl) collapseTrivia(p []sym, i int) []sym {
	name := p[i].e.(*grammar.Ref).Name
	for m := i + 1; m < len(p); m++ {
		switch p[m].kind {
		case symSkip, symBind, symEnv, symNot, symAnd:
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
