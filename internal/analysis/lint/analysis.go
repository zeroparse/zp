package lint

import (
	"strings"
	"unicode/utf8"

	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// analysis holds what the checks need to know about a grammar. Every property is either an
// over-approximation or an under-approximation, chosen so that a check built on it reports only
// what is certain:
//
//   - nullable (may succeed without consuming input, at the end of the input or elsewhere) and
//     canMatch (may succeed at all) are over-approximations: "not nullable" and "cannot match"
//     are certain.
//   - succeeds (matchPrefix: certainly succeeds on every input that begins with a given string)
//     and fails (certainly fails on such inputs) are under-approximations.
//   - prefix (a string that every match begins with) is an under-approximation.
//
// None of them account for the nesting limit (Limits.Depth), which can make any rule call fail.
type analysis struct {
	control *lintControl
	g       *ast.Grammar
	order   []*ast.RuleDef
	rules   map[string]*ast.RuleDef

	// nullable[end] holds the rules that may succeed without consuming input at the end of the
	// input (end = true) or elsewhere (false).
	nullable [2]map[string]bool
	canMatch map[string]bool
	// baseCase holds the rules that can match if _|_ and ! of an expression that always
	// succeeds are taken to match: the rules not in it can only match by matching themselves
	// (or another rule not in it) first.
	baseCase map[string]bool
	// leftRecursive holds the rules on a left-recursion cycle. While such a rule grows its
	// match, a call of it inside the cycle returns the match found so far, which can be a
	// failure: a call of it is never certain to succeed.
	leftRecursive map[string]bool
	// engineNullable is nullability as the engine computes it to find left recursion (cruder
	// than nullable: every lookahead counts as nullable).
	engineNullable   map[string]bool
	engineSucceeding map[string]bool

	// Memos of the analyses that follow rule calls, and the rules being followed (a call of a
	// rule that is being followed is not followed again: the result is "not certain").
	matchMemo  map[matchKey]matchResult
	prefixMemo map[string]prefixResult
	busy       map[string]bool
	// cycles counts the calls not followed because of busy, so that results that depend on
	// them are not memoized.
	cycles int
}

func newAnalysisControlled(g *ast.Grammar, control *lintControl, tables ruleTables) *analysis {
	control.poll()

	a := &analysis{control: control, g: g, rules: map[string]*ast.RuleDef{}, nullable: [2]map[string]bool{{}, {}}, canMatch: map[string]bool{},
		baseCase:  map[string]bool{},
		matchMemo: map[matchKey]matchResult{}, prefixMemo: map[string]prefixResult{}, busy: map[string]bool{}}
	for _, r := range control.rules(g) {
		control.poll()
		if a.rules[r.Name] == nil {
			a.rules[r.Name] = r
			a.order = append(a.order, r)
		}
	}
	// The rule tables of the front end's facts: the other analyses use them (through matchPrefix).
	a.engineNullable, a.leftRecursive, a.engineSucceeding = tables.nullable, tables.cyclic, tables.succeeds
	for _, end := range []bool{false, true} {
		control.poll()
		set := a.nullable[b2i(end)]
		for changed := true; changed; {
			control.poll()
			changed = false
			for _, r := range a.order {
				control.poll()
				if !set[r.Name] && a.nullableAt(r.Expr, end) {
					set[r.Name], changed = true, true
				}
			}
		}
	}
	for _, m := range []struct {
		set     map[string]bool
		lenient bool
	}{{a.canMatch, false}, {a.baseCase, true}} {
		for changed := true; changed; {
			control.poll()
			changed = false
			for _, r := range a.order {
				control.poll()
				if !m.set[r.Name] && a.mayMatchIn(r.Expr, m.set, m.lenient) {
					m.set[r.Name], changed = true, true
				}
			}
		}
	}
	return a
}

// isEngineNullable reports whether e may succeed without consuming input, as the engine
// computes it.
func (a *analysis) isEngineNullable(e ast.Expr) bool {
	control := a.control
	control.poll()

	switch e := analysisExpr(e, control).(type) {
	case *ast.Ref:
		return a.engineNullable[e.Name]
	case *ast.Literal:
		return e.Value == ""
	case *ast.CharClass, *ast.Any, *ast.Bottom:
		return false
	case *ast.Seq:
		for _, it := range e.Items {
			control.poll()
			if !a.isEngineNullable(it) {
				return false
			}
		}
		return true
	case *ast.Choice:
		for _, alt := range e.Alts {
			control.poll()
			if a.isEngineNullable(alt) {
				return true
			}
		}
		return false
	case *ast.Repeat:
		return e.Min == 0 || e.Min == 1 && a.isEngineNullable(e.Expr)
	case *ast.Atomic:
		return a.isEngineNullable(e.Expr)
	case *ast.Discard:
		return a.isEngineNullable(e.Expr)
	case *ast.Capture:
		return a.isEngineNullable(e.Expr)
	case *ast.Attributed:
		return a.isEngineNullable(e.Expr)
	case *ast.OperatorsRule:
		for _, o := range e.Operands {
			control.poll()
			if a.isEngineNullable(o.Expr) {
				return true
			}
		}
		return false
	}
	return true
}

// leftCalls returns the rules that e may call before consuming input, as the engine finds them.
func (a *analysis) leftCalls(e ast.Expr, acc []string) []string {
	control := a.control
	control.poll()

	switch e := analysisExpr(e, control).(type) {
	case *ast.Ref:
		return append(acc, e.Name)
	case *ast.Seq:
		for _, it := range e.Items {
			control.poll()
			acc = a.leftCalls(it, acc)
			if !a.isEngineNullable(it) {
				break
			}
		}
	case *ast.Choice:
		for _, alt := range e.Alts {
			control.poll()
			acc = a.leftCalls(alt, acc)
			if a.alwaysSucceeds(alt) {
				break
			}
		}
	case *ast.Repeat:
		if e.Max != 0 {
			acc = a.leftCalls(e.Expr, acc)
		}
	case *ast.Optional:
		acc = a.leftCalls(e.Expr, acc)
	case *ast.And:
		acc = a.leftCalls(e.Expr, acc)
	case *ast.Not:
		acc = a.leftCalls(e.Expr, acc)
	case *ast.Atomic:
		acc = a.leftCalls(e.Expr, acc)
	case *ast.Discard:
		acc = a.leftCalls(e.Expr, acc)
	case *ast.Capture:
		acc = a.leftCalls(e.Expr, acc)
	case *ast.Attributed:
		acc = a.leftCalls(e.Expr, acc)
		if !a.alwaysSucceeds(e.Expr) {
			for _, at := range e.Attrs {
				control.poll()
				if at.Name == "recover" {
					if skip, ok := at.Arg("skip"); ok {
						acc = a.leftCalls(skip, acc)
					}
				}
			}
		}
	case *ast.OperatorsRule:
		if e.Skip != nil {
			acc = a.leftCalls(e.Skip, acc)
		}
		for _, o := range e.Operands {
			control.poll()
			acc = a.leftCalls(o.Expr, acc)
			if a.alwaysSucceeds(o.Expr) {
				break
			}
		}
		for _, l := range e.Levels {
			control.poll()
			for _, op := range l.Operators {
				control.poll()
				if op.Kind == ast.Prefix {
					acc = a.leftCalls(op.Expr, acc)
				}
			}
		}
	}
	return acc
}

// mayMatch reports whether e may succeed.
func (a *analysis) mayMatch(e ast.Expr) bool {
	control := a.control
	control.poll()
	return a.mayMatchIn(e, a.canMatch, false)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// isNullable reports whether e may succeed without consuming input.
func (a *analysis) isNullable(e ast.Expr) bool {
	control := a.control
	control.poll()

	return a.nullableAt(e, false) || a.nullableAt(e, true)
}

// nullableAt reports whether e may succeed without consuming input at the end of the input (end
// set) or at another position. Telling the two apart makes "!$$ x" not nullable when x can only
// match nothing at the end of the input, as in a line that ends with ("\n" / $$).
func (a *analysis) nullableAt(e ast.Expr, end bool) bool {
	control := a.control
	control.poll()

	switch e := analysisExpr(e, control).(type) {
	case *ast.Ref:
		return a.nullable[b2i(end)][e.Name]
	case *ast.Literal:
		return e.Value == ""
	case *ast.CharClass, *ast.Any, *ast.Bottom:
		return false
	case *ast.EndInput:
		return end
	case *ast.Seq:
		for _, it := range e.Items {
			control.poll()
			if !a.nullableAt(it, end) {
				return false
			}
		}
		return true
	case *ast.Choice:
		for _, alt := range e.Alts {
			control.poll()
			if a.nullableAt(alt, end) {
				return true
			}
		}
		return false
	case *ast.Repeat:
		return e.Min == 0 || e.Min == 1 && a.nullableAt(e.Expr, end)
	case *ast.And:
		// At the end of the input, the operand can only match without consuming input.
		return !end || a.nullableAt(e.Expr, true)
	case *ast.Not:
		return !a.succeedsAt(e.Expr, end)
	case *ast.Atomic:
		return a.nullableAt(e.Expr, end)
	case *ast.Discard:
		return a.nullableAt(e.Expr, end)
	case *ast.Capture:
		return a.nullableAt(e.Expr, end)
	case *ast.Attributed:
		// A recovery must consume input.
		return a.nullableAt(e.Expr, end)
	case *ast.OperatorsRule:
		for _, o := range e.Operands {
			control.poll()
			if a.nullableAt(o.Expr, end) {
				return true
			}
		}
		return false
	}
	// Optional, predicate, other anchors, cut, top
	return true
}

// succeedsAt reports whether e certainly succeeds at the end of the input (end set) or at any
// other position.
func (a *analysis) succeedsAt(e ast.Expr, end bool) bool {
	control := a.control
	control.poll()

	switch e := neutral(e, true, control).(type) {
	case *ast.EndInput:
		return end
	case *ast.Any:
		return !end
	case *ast.Not:
		return a.failsAt(e.Expr, end)
	}
	return a.succeeds(e)
}

// failsAt reports whether e certainly fails at the end of the input (end set) or at any other
// position.
func (a *analysis) failsAt(e ast.Expr, end bool) bool {
	control := a.control
	control.poll()

	switch e := neutral(e, true, control).(type) {
	case *ast.EndInput:
		return !end
	case *ast.Any, *ast.CharClass:
		return end
	case *ast.Literal:
		return end && e.Value != ""
	case *ast.Bottom:
		return true
	case *ast.Not:
		return a.succeedsAt(e.Expr, end)
	}
	return false
}

// mayMatchIn reports whether e may succeed, given the rules in set that may (a least fixed
// point: a rule that can only match by matching itself first never matches). With lenient set,
// _|_ and ! of an expression that always succeeds are taken to match.
//
// Anchors that contradict their neighbors (anchorConflict) are not taken into account, so that
// canMatch is known before prefix, which uses it, runs.
func (a *analysis) mayMatchIn(e ast.Expr, set map[string]bool, lenient bool) bool {
	control := a.control
	control.poll()

	switch e := analysisExpr(e, control).(type) {
	case *ast.Ref:
		return set[e.Name]
	case *ast.Bottom:
		return lenient
	case *ast.Seq:
		for _, it := range e.Items {
			control.poll()
			if !a.mayMatchIn(it, set, lenient) {
				return false
			}
		}
	case *ast.Choice:
		for _, alt := range e.Alts {
			control.poll()
			if a.mayMatchIn(alt, set, lenient) {
				return true
			}
		}
		return false
	case *ast.Repeat:
		return e.Min == 0 || a.mayMatchIn(e.Expr, set, lenient)
	case *ast.And:
		return a.mayMatchIn(e.Expr, set, lenient)
	case *ast.Not:
		return lenient || !a.succeeds(e.Expr)
	case *ast.Atomic:
		return a.mayMatchIn(e.Expr, set, lenient)
	case *ast.Discard:
		return a.mayMatchIn(e.Expr, set, lenient)
	case *ast.Capture:
		return a.mayMatchIn(e.Expr, set, lenient)
	case *ast.Attributed:
		return hasAttr(e, "recover", control) || hasAttr(e, "missing", control) || a.mayMatchIn(e.Expr, set, lenient)
	case *ast.OperatorsRule:
		for _, o := range e.Operands {
			control.poll()
			if a.mayMatchIn(o.Expr, set, lenient) {
				return true
			}
		}
		return false
	}
	return true
}

func hasAttr(e *ast.Attributed, name string, control *lintControl) bool {
	for _, at := range e.Attrs {
		control.poll()
		if at.Name == name {
			return true
		}
	}
	return false
}

// anchorConflict returns the anchor of the sequence that can never match where it is, if any:
//   - $$ followed by items that cannot match without consuming input at the end of the input;
//   - ^^ preceded by items that must consume input;
//   - $ followed by items whose matches all begin with a character other than a line break;
//   - ^ preceded by items that always consume the same text, which does not end with "\n".
func (a *analysis) anchorConflict(s *ast.Seq) ast.Expr {
	control := a.control
	control.poll()

	for i, it := range s.Items {
		control.poll()
		after := &ast.Seq{Items: s.Items[i+1:]}
		before := &ast.Seq{Items: s.Items[:i]}
		switch it.(type) {
		case *ast.EndInput:
			if !a.nullableAt(after, true) {
				return it
			}
		case *ast.BeginInput:
			if !a.isNullable(before) {
				return it
			}
		case *ast.EndLine:
			if p := a.prefix(after); p.s != "" && p.s[0] != '\n' && p.s[0] != '\r' {
				return it
			}
		case *ast.BeginLine:
			if p := a.prefix(before); p.exact && p.s != "" && !strings.HasSuffix(p.s, "\n") {
				return it
			}
		}
	}
	return nil
}

// succeeds reports whether e certainly succeeds, whatever the input.
func (a *analysis) succeeds(e ast.Expr) bool {
	control := a.control
	control.poll()
	return a.matchPrefix(e, "").ok
}

// matchResult is the result of matchPrefix.
type matchResult struct {
	ok bool
	// exact reports that e consumes exactly the part of t before rest.
	exact bool
	rest  string
}

type matchKey struct {
	rule string
	t    string
}

// matchPrefix reports whether e certainly succeeds on every input that begins with t.
func (a *analysis) matchPrefix(e ast.Expr, t string) matchResult {
	control := a.control
	control.poll()

	ok := func(rest string) matchResult { return matchResult{ok: true, exact: true, rest: rest} }
	inexact := matchResult{ok: true}
	switch e := analysisExpr(e, control).(type) {
	case *ast.Literal:
		// A case-insensitive literal matches the input that begins with any case of it, and the
		// folding keeps byte lengths, so the rest of t begins after len(e.Value) bytes.
		if e.IgnoreCase && hasPrefixFold(t, e.Value) || !e.IgnoreCase && strings.HasPrefix(t, e.Value) {
			return ok(t[len(e.Value):])
		}
	case *ast.CharClass:
		if r, size := utf8.DecodeRuneInString(t); t != "" && classAcceptsControlled(e, r, control) {
			return ok(t[size:])
		}
	case *ast.Any:
		if _, size := utf8.DecodeRuneInString(t); t != "" {
			return ok(t[size:])
		}
	case *ast.Top, *ast.Cut:
		return ok(t)
	case *ast.Seq:
		cur := ok(t)
		for _, it := range e.Items {
			control.poll()
			if cur.exact {
				if cur = a.matchPrefix(it, cur.rest); !cur.ok {
					return cur
				}
			} else if !a.succeeds(it) {
				return matchResult{}
			}
		}
		return cur
	case *ast.Choice:
		for k, alt := range e.Alts {
			control.poll()
			if r := a.matchPrefix(alt, t); r.ok {
				if k == 0 {
					return r
				}
				return inexact
			}
			// A cut that commits the choice to an earlier alternative can make it fail before
			// it gets here.
			if a.hasLooseCut(alt) {
				break
			}
		}
	case *ast.Repeat:
		cur := ok(t)
		for i := range e.Min {
			control.poll()
			if cur.exact {
				before := cur.rest
				if cur = a.matchPrefix(e.Expr, cur.rest); !cur.ok {
					return cur
				}
				if cur.exact && cur.rest == before {
					if i+1 < e.Min {
						return matchResult{}
					}
					return cur // the first empty iteration ends the repetition
				}
				// An inexact match may consume nothing and stop before the
				// minimum. Do not prove another iteration from its success.
				if !cur.exact && i+1 < e.Min {
					return matchResult{}
				}
			} else if !a.succeeds(e.Expr) {
				return matchResult{}
			}
		}
		if e.Max == e.Min {
			return cur
		}
		// Further iterations are optional, unless one fails after a cut.
		if a.hasLooseCut(e.Expr) && !a.succeeds(e.Expr) {
			return matchResult{}
		}
		return inexact
	case *ast.Optional:
		if r := a.matchPrefix(e.Expr, t); r.ok {
			return r
		}
		if !a.hasLooseCut(e.Expr) {
			return inexact
		}
	case *ast.Capture:
		return a.matchPrefix(e.Expr, t)
	case *ast.Atomic:
		return a.matchPrefix(e.Expr, t)
	case *ast.Discard:
		return a.matchPrefix(e.Expr, t)
	case *ast.Attributed:
		// #recover only adds ways to succeed; @error and #stream do not change matching.
		return a.matchPrefix(e.Expr, t)
	case *ast.And:
		if a.matchPrefix(e.Expr, t).ok {
			return ok(t)
		}
	case *ast.Not:
		if a.fails(e.Expr, t) {
			return ok(t)
		}
	case *ast.Ref:
		r := a.rules[e.Name]
		if r == nil || e.Level != "" || a.leftRecursive[e.Name] {
			break
		}
		if pr, _ := ast.RuleOperators(r.Expr); pr != nil {
			break
		}
		key := matchKey{e.Name, t}
		if m, ok := a.matchMemo[key]; ok {
			return m
		}
		if a.busy[e.Name] {
			a.cycles++
			break
		}
		a.busy[e.Name] = true
		before := a.cycles
		m := a.matchPrefix(r.Expr, t)
		delete(a.busy, e.Name)
		if a.cycles == before {
			a.matchMemo[key] = m
		}
		return m
	}
	// Predicates, anchors, bottom and Operators Rule expressions are not certain to succeed.
	return matchResult{}
}

// fails reports whether e certainly fails on every input that begins with t.
func (a *analysis) fails(e ast.Expr, t string) bool {
	control := a.control
	control.poll()

	switch e := analysisExpr(e, control).(type) {
	case *ast.Literal:
		if e.IgnoreCase {
			return !hasPrefixFold(t, e.Value) && !hasPrefixFold(e.Value, t)
		}
		return !strings.HasPrefix(t, e.Value) && !strings.HasPrefix(e.Value, t)
	case *ast.CharClass:
		r, _ := utf8.DecodeRuneInString(t)
		return t != "" && !classAcceptsControlled(e, r, control)
	case *ast.Bottom:
		return true
	case *ast.Seq:
		cur := t
		for _, it := range e.Items {
			control.poll()
			if a.fails(it, cur) {
				return true
			}
			r := a.matchPrefix(it, cur)
			if !r.ok || !r.exact {
				return false
			}
			cur = r.rest
		}
	case *ast.Choice:
		for _, alt := range e.Alts {
			control.poll()
			if !a.fails(alt, t) {
				return false
			}
		}
		return true
	case *ast.Repeat:
		return e.Min > 0 && a.fails(e.Expr, t)
	case *ast.Capture:
		return a.fails(e.Expr, t)
	case *ast.Atomic:
		return a.fails(e.Expr, t)
	case *ast.Discard:
		return a.fails(e.Expr, t)
	case *ast.Attributed:
		return !hasAttr(e, "recover", control) && !hasAttr(e, "missing", control) && a.fails(e.Expr, t)
	case *ast.Ref:
		r := a.rules[e.Name]
		if r == nil || e.Level != "" || a.busy[e.Name] {
			return false
		}
		if pr, _ := ast.RuleOperators(r.Expr); pr != nil {
			return false
		}
		a.busy[e.Name] = true
		defer delete(a.busy, e.Name)
		return a.fails(r.Expr, t)
	}
	return false
}

// hasLooseCut reports whether e contains a cut that commits a choice, repetition or optional
// expression around e: one that is not inside a choice, repetition, optional expression or
// lookahead of its own (which stop it), or in another rule (cuts do not cross rules).
func hasLooseCut(e ast.Expr) bool {
	return astfacts.HasLooseCut(e)
}

// hasPrefixFold reports whether s begins with p when the ASCII letters of both are compared in
// lowercase, as for case-insensitive literals. It works on bytes, so invalid UTF-8 is kept as it is.
func hasPrefixFold(s, p string) bool {
	if len(p) > len(s) {
		return false
	}
	for i := 0; i < len(p); i++ {
		if asciiLower(s[i]) != asciiLower(p[i]) {
			return false
		}
	}
	return true
}

// asciiLower maps A-Z to a-z and leaves every other byte as it is.
func asciiLower(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// prefixResult is the result of prefix.
type prefixResult struct {
	s string
	// exact reports that every match consumes exactly s.
	exact bool
}

// prefix returns a string that every match of e begins with.
func (a *analysis) prefix(e ast.Expr) prefixResult {
	control := a.control
	control.poll()

	switch e := analysisExpr(e, control).(type) {
	case *ast.Literal:
		if e.IgnoreCase {
			// Every match begins with some case of the literal, not with one string: no prefix is known.
			return prefixResult{"", false}
		}
		return prefixResult{e.Value, true}
	case *ast.CharClass:
		if len(e.Ranges) == 1 && e.Ranges[0].Lo == e.Ranges[0].Hi && !e.Negated {
			return prefixResult{string(e.Ranges[0].Lo), true}
		}
	case *ast.And, *ast.Not, *ast.Predicate, *ast.Cut, *ast.Top,
		*ast.BeginInput, *ast.EndInput, *ast.BeginLine, *ast.EndLine:
		return prefixResult{"", true}
	case *ast.Seq:
		var b strings.Builder
		for _, it := range e.Items {
			control.poll()
			p := a.prefix(it)
			b.WriteString(p.s)
			if !p.exact {
				return prefixResult{b.String(), false}
			}
		}
		return prefixResult{b.String(), true}
	case *ast.Choice:
		var out prefixResult
		first := true
		for _, alt := range e.Alts {
			control.poll()
			if !a.mayMatch(alt) {
				continue // contributes no matches
			}
			p := a.prefix(alt)
			if first {
				out, first = p, false
				continue
			}
			n := 0
			for n < len(out.s) && n < len(p.s) && out.s[n] == p.s[n] {
				control.poll()
				n++
			}
			for n > 0 && n < len(out.s) && !utf8.RuneStart(out.s[n]) {
				control.poll()
				n-- // keep whole characters
			}
			out = prefixResult{out.s[:n], out.exact && p.exact && out.s == p.s}
		}
		return out
	case *ast.Repeat:
		if e.Min == 0 {
			return prefixResult{"", e.Max == 0}
		}
		p := a.prefix(e.Expr)
		if !p.exact {
			return p
		}
		return prefixResult{strings.Repeat(p.s, e.Min), e.Max == e.Min}
	case *ast.Capture:
		return a.prefix(e.Expr)
	case *ast.Atomic:
		return a.prefix(e.Expr)
	case *ast.Discard:
		return a.prefix(e.Expr)
	case *ast.Attributed:
		if !hasAttr(e, "recover", control) && !hasAttr(e, "missing", control) { // recovery invalidates a guaranteed prefix
			return a.prefix(e.Expr)
		}
	case *ast.Ref:
		r := a.rules[e.Name]
		if r == nil || e.Level != "" {
			break
		}
		if p, ok := a.prefixMemo[e.Name]; ok {
			return p
		}
		if a.busy[e.Name] {
			a.cycles++
			break
		}
		a.busy[e.Name] = true
		before := a.cycles
		p := a.prefix(r.Expr)
		delete(a.busy, e.Name)
		if a.cycles == before {
			a.prefixMemo[e.Name] = p
		}
		return p
	}
	return prefixResult{}
}

// classAccepts reports whether the character class matches r.
func classAccepts(c *ast.CharClass, r rune) bool {
	for _, rg := range c.Ranges {
		if rg.Lo <= r && r <= rg.Hi {
			return !c.Negated
		}
	}
	return c.Negated
}

// --- Structure ---

// neutral strips the wrappers that do not change whether an expression matches or how much
// input it consumes: #atomic, -, @error and #stream, and captures if caps is set.
func neutral(e ast.Expr, caps bool, control *lintControl) ast.Expr {
	for {
		control.poll()
		e = analysisExpr(e, control)
		switch x := e.(type) {
		case *ast.Atomic:
			e = x.Expr
		case *ast.Discard:
			e = x.Expr
		case *ast.Capture:
			if !caps {
				return e
			}
			e = x.Expr
		case *ast.Attributed:
			if hasAttr(x, "recover", control) {
				return e
			}
			e = x.Expr
		default:
			return e
		}
	}
}

// items returns the items of e as a sequence.
func items(e ast.Expr, caps bool, control *lintControl) []ast.Expr {
	if s, ok := neutral(e, caps, control).(*ast.Seq); ok {
		return s.Items
	}
	return []ast.Expr{e}
}

// same reports whether x and y match the same inputs in the same way, by comparing their
// structure. If caps is set, the names of captures are ignored.
func same(x, y ast.Expr, caps bool, control *lintControl) bool {
	control.poll()
	x, y = neutral(x, caps, control), neutral(y, caps, control)
	switch x := x.(type) {
	case *ast.Ref:
		y, ok := y.(*ast.Ref)
		return ok && x.Name == y.Name && x.Level == y.Level
	case *ast.Literal:
		y, ok := y.(*ast.Literal)
		return ok && x.Value == y.Value && x.IgnoreCase == y.IgnoreCase
	case *ast.CharClass:
		y, ok := y.(*ast.CharClass)
		return ok && x.Negated == y.Negated && sameRangesControlled(normalizeControlled(x.Ranges, control), normalizeControlled(y.Ranges, control), control)
	case *ast.Seq:
		y, ok := y.(*ast.Seq)
		return ok && sameList(x.Items, y.Items, caps, control)
	case *ast.Choice:
		y, ok := y.(*ast.Choice)
		return ok && sameList(x.Alts, y.Alts, caps, control)
	case *ast.Repeat:
		y, ok := y.(*ast.Repeat)
		return ok && x.Min == y.Min && x.Max == y.Max && same(x.Expr, y.Expr, caps, control)
	case *ast.Optional:
		y, ok := y.(*ast.Optional)
		return ok && same(x.Expr, y.Expr, caps, control)
	case *ast.And:
		y, ok := y.(*ast.And)
		return ok && same(x.Expr, y.Expr, caps, control)
	case *ast.Not:
		y, ok := y.(*ast.Not)
		return ok && same(x.Expr, y.Expr, caps, control)
	case *ast.Capture:
		y, ok := y.(*ast.Capture)
		return ok && x.Name == y.Name && same(x.Expr, y.Expr, caps, control)
	case *ast.Predicate:
		y, ok := y.(*ast.Predicate)
		return ok && control.formatTerm(x.Term) == control.formatTerm(y.Term)
	case *ast.Attributed:
		y, ok := y.(*ast.Attributed)
		return ok && control.formatExpr(x) == control.formatExpr(y) && same(x.Expr, y.Expr, caps, control)
	case *ast.Any, *ast.Cut, *ast.Top, *ast.Bottom, *ast.BeginInput, *ast.EndInput,
		*ast.BeginLine, *ast.EndLine:
		return sameType(x, y)
	}
	return false
}

func sameType(x, y ast.Expr) bool {
	switch x.(type) {
	case *ast.Any:
		_, ok := y.(*ast.Any)
		return ok
	case *ast.Cut:
		_, ok := y.(*ast.Cut)
		return ok
	case *ast.Top:
		_, ok := y.(*ast.Top)
		return ok
	case *ast.Bottom:
		_, ok := y.(*ast.Bottom)
		return ok
	case *ast.BeginInput:
		_, ok := y.(*ast.BeginInput)
		return ok
	case *ast.EndInput:
		_, ok := y.(*ast.EndInput)
		return ok
	case *ast.BeginLine:
		_, ok := y.(*ast.BeginLine)
		return ok
	case *ast.EndLine:
		_, ok := y.(*ast.EndLine)
		return ok
	}
	return false
}

func sameList(xs, ys []ast.Expr, caps bool, control *lintControl) bool {
	if len(xs) != len(ys) {
		return false
	}
	for i := range xs {
		control.poll()
		if !same(xs[i], ys[i], caps, control) {
			return false
		}
	}
	return true
}

// hasPredicate reports whether e contains a predicate (in the rule, not in the rules it calls).
func hasPredicate(e ast.Expr, control *lintControl) bool {
	found := false
	walkExprControlled(e, func(x ast.Expr) {
		control.poll()
		if _, ok := x.(*ast.Predicate); ok {
			found = true
		}
	}, control)
	return found
}

// --- Walking ---

// walkExpr visits the expression and its subexpressions in preorder, including attribute
// arguments and the parts of Operators Rule expressions.
func walkExprControlled(e ast.Expr, f func(ast.Expr), control *lintControl) {
	if control == nil {
		walkExpr(e, f)
		return
	}
	control.poll()
	if e == nil {
		return
	}
	f(e)
	switch e := e.(type) {
	case *ast.Group:
		walkExprControlled(e.Expr, f, control)
	case *ast.Seq:
		for _, it := range e.Items {
			walkExprControlled(it, f, control)
		}
	case *ast.Choice:
		for _, alt := range e.Alts {
			walkExprControlled(alt, f, control)
		}
	case *ast.Repeat:
		walkExprControlled(e.Expr, f, control)
	case *ast.Optional:
		walkExprControlled(e.Expr, f, control)
	case *ast.And:
		walkExprControlled(e.Expr, f, control)
	case *ast.Not:
		walkExprControlled(e.Expr, f, control)
	case *ast.Atomic:
		walkExprControlled(e.Expr, f, control)
	case *ast.Stream:
		walkExprControlled(e.Expr, f, control)
	case *ast.Recovery:
		walkExprControlled(e.Expr, f, control)
		walkExprControlled(e.Skip, f, control)
	case *ast.Discard:
		walkExprControlled(e.Expr, f, control)
	case *ast.Capture:
		walkExprControlled(e.Expr, f, control)
	case *ast.Attributed:
		walkExprControlled(e.Expr, f, control)
		for _, at := range e.Attrs {
			control.poll()
			for _, arg := range control.expressionArgs(at) {
				walkExprControlled(arg, f, control)
			}
		}
	case *ast.OperatorsRule:
		walkExprControlled(e.Skip, f, control)
		for _, o := range e.Operands {
			walkExprControlled(o.Expr, f, control)
		}
		for _, l := range e.Levels {
			control.poll()
			for _, op := range l.Operators {
				walkExprControlled(op.Expr, f, control)
			}
		}
	}
}

// walkTerm visits the value expression and its subexpressions in preorder.
func walkTerm(t ast.Term, f func(ast.Term)) {
	if t == nil {
		return
	}
	f(t)
	switch t := t.(type) {
	case *ast.TermGroup:
		walkTerm(t.Term, f)
	case *ast.Block:
		for _, stmt := range t.Statements {
			walkTerm(stmt, f)
		}
	case *ast.Member:
		walkTerm(t.X, f)
	case *ast.New:
		for _, fi := range t.Fields {
			walkTerm(fi.Value, f)
		}
		for _, x := range t.Args {
			walkTerm(x, f)
		}
	case *ast.ListConstructor:
		for _, item := range t.Items {
			walkTerm(item, f)
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

// calls returns the rules that e calls, in order of appearance.
func calls(e ast.Expr) []string {
	return callsControlled(e, nil)
}

func callsControlled(e ast.Expr, control *lintControl) []string {
	control.poll()

	var out []string
	walkExprControlled(e, func(x ast.Expr) {
		control.poll()
		switch x := x.(type) {
		case *ast.Ref:
			out = append(out, x.Name)
		case *ast.InlineAction:
			// An inline action calls a hidden rule whose body is its expression, so the rules
			// that expression calls are called from here. The walkers do not enter it.
			out = append(out, callsControlled(x.Expr, control)...)
		}
	}, control)
	return out
}

// posOf returns the position where e begins, or the zero Pos if it is unknown.
func posOf(e ast.Expr) ast.Pos { return posOfControlled(e, nil) }

func posOfControlled(e ast.Expr, control *lintControl) ast.Pos {
	control.poll()
	switch e := analysisExpr(e, control).(type) {
	case *ast.Ref:
		return e.Diagnostic()
	case *ast.Literal:
		return e.Diagnostic()
	case *ast.CharClass:
		return e.Diagnostic()
	case *ast.Any:
		return e.Diagnostic()
	case *ast.Seq:
		if len(e.Items) > 0 {
			return posOfControlled(e.Items[0], control)
		}
	case *ast.Choice:
		if len(e.Alts) > 0 {
			return posOfControlled(e.Alts[0], control)
		}
	case *ast.Repeat:
		return e.Diagnostic()
	case *ast.Optional:
		return e.Diagnostic()
	case *ast.And:
		return e.Diagnostic()
	case *ast.Not:
		return e.Diagnostic()
	case *ast.Atomic:
		return e.Diagnostic()
	case *ast.Discard:
		return e.Diagnostic()
	case *ast.Capture:
		return e.Diagnostic()
	case *ast.Cut:
		return e.Diagnostic()
	case *ast.Top:
		return e.Diagnostic()
	case *ast.Bottom:
		return e.Diagnostic()
	case *ast.BeginInput:
		return e.Diagnostic()
	case *ast.EndInput:
		return e.Diagnostic()
	case *ast.BeginLine:
		return e.Diagnostic()
	case *ast.EndLine:
		return e.Diagnostic()
	case *ast.Predicate:
		return e.Diagnostic()
	case *ast.Attributed:
		return posOfControlled(e.Expr, control)
	case *ast.OperatorsRule:
		return e.Diagnostic()
	}
	return ast.Pos{}
}

// show formats e for a message, shortened if it is long.
func show(e ast.Expr, control *lintControl) string {
	s := control.formatExpr(e)
	// Keep only the displayed prefix and a bounded lookahead. The rune
	// conversion preserves the old normalization of invalid UTF-8 even in
	// caller-built ASTs that have not passed validation.
	var prefix [37]rune
	count := 0
	for _, r := range s {
		if count == 40 {
			return "`" + string(prefix[:]) + "...`"
		}
		if count < len(prefix) {
			prefix[count] = r
		}
		count++
	}
	return "`" + s + "`"
}

// analysisExpr presents behavior pragmas to the existing matching proofs without
// mutating the source tree or losing the wrapped expression's positions.
func analysisExpr(e ast.Expr, control *lintControl) ast.Expr {
	switch x := e.(type) {
	case *ast.Group:
		control.poll()
		return analysisExpr(x.Expr, control)
	case *ast.Stream:
		return &ast.Attributed{Expr: x.Expr, Attrs: []*ast.Attribute{{Pos: x.Diagnostic(), Name: "stream", Pragma: true}}}
	case *ast.Recovery:
		return &ast.Attributed{Expr: x.Expr, Attrs: []*ast.Attribute{{Pos: x.Diagnostic(), Name: "recover", Pragma: true, KwArgs: map[string]any{"skip": x.Skip}}}}
	}
	return e
}

// alwaysSucceeds preserves the ordinary proof and shares the invocation control.
func (a *analysis) alwaysSucceeds(e ast.Expr) bool {
	if a.control == nil {
		return astfacts.AlwaysSucceeds(e, a.engineSucceeding)
	}
	return astfacts.AlwaysSucceedsWithPoll(e, a.engineSucceeding, a.control.poll)
}

func (a *analysis) hasLooseCut(e ast.Expr) bool {
	if a.control == nil {
		return hasLooseCut(e)
	}
	return astfacts.HasLooseCutWithPoll(e, a.control.poll)
}

func walkExpr(e ast.Expr, f func(ast.Expr)) {
	if e == nil {
		return
	}
	f(e)
	switch e := e.(type) {
	case *ast.Group:
		walkExpr(e.Expr, f)
	case *ast.Seq:
		for _, it := range e.Items {
			walkExpr(it, f)
		}
	case *ast.Choice:
		for _, alt := range e.Alts {
			walkExpr(alt, f)
		}
	case *ast.Repeat:
		walkExpr(e.Expr, f)
	case *ast.Optional:
		walkExpr(e.Expr, f)
	case *ast.And:
		walkExpr(e.Expr, f)
	case *ast.Not:
		walkExpr(e.Expr, f)
	case *ast.Atomic:
		walkExpr(e.Expr, f)
	case *ast.Stream:
		walkExpr(e.Expr, f)
	case *ast.Recovery:
		walkExpr(e.Expr, f)
		walkExpr(e.Skip, f)
	case *ast.Discard:
		walkExpr(e.Expr, f)
	case *ast.Capture:
		walkExpr(e.Expr, f)
	case *ast.Attributed:
		walkExpr(e.Expr, f)
		for _, at := range e.Attrs {
			for _, arg := range at.ExpressionArgs() {
				walkExpr(arg, f)
			}
		}
	case *ast.OperatorsRule:
		walkExpr(e.Skip, f)
		for _, o := range e.Operands {
			walkExpr(o.Expr, f)
		}
		for _, l := range e.Levels {
			for _, op := range l.Operators {
				walkExpr(op.Expr, f)
			}
		}
	}
}
