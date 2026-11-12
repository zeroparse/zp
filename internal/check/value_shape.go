package check

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/runtime/parse"

	"zeroparse.dev/zp/internal/value"
)

// The static shapes of values in the value model. Every backend builds the values of an
// expression from these, so that they agree with the inferred types (value_types.go).

// ShapeElems returns the elements e contributes to the tuple of its scope, without types of rule
// calls (they do not change the shape).
func ShapeElems(p *Checker, e grammar.Expr) []ValueElem {
	var control *cancel.Control
	if p != nil {
		control = p.shapeControl
	}
	return (&ValueInference{Prog: p, control: control}).elems(e)
}

// SpliceBody returns the body of the rule name if it is a splice rule: a rule that makes no node of
// its own, so that a call of it contributes what its body does, as a group (Rule.Splice).
func (p *Checker) SpliceBody(name string) (grammar.Expr, bool) {
	if p == nil {
		return nil, false
	}
	if r := p.Rule(name); r.Splice {
		return r.Def.Expr, true
	}
	return nil, false
}

// isSeq reports whether e is a sequence, as grouped, with attributes other than recovery, or as the
// body of a splice rule it calls.
func isSeq(p *Checker, e grammar.Expr) bool { return IsSeqOf(p.SpliceBody, e) }

// IsSeqOf is isSeq with spliceBody, which returns the body of a splice rule, in place of a program.
func IsSeqOf(spliceBody func(string) (grammar.Expr, bool), e grammar.Expr) bool {
	var seen map[string]bool
	for {
		switch x := UngroupExpr(e).(type) {
		case *grammar.Seq:
			return true
		case *grammar.Attributed:
			// An attribute other than recovery only labels e; recovery makes it one element.
			if Recovers(x) {
				return false
			}
			e = x.Expr
		case *grammar.Ref:
			body, ok := spliceBody(x.Name)
			if !ok || seen[x.Name] {
				return false
			}
			if seen == nil {
				seen = map[string]bool{}
			}
			seen[x.Name] = true
			e = body
		default:
			return false
		}
	}
}

// ValuedCount returns the number of tuple elements e contributes: its elements with a value.
// Elements without one only hold captures kept beside the tuple.
func ValuedCount(p *Checker, e grammar.Expr) int { return valuedCount(ShapeElems(p, e)) }

func valuedCount(elems []ValueElem) int {
	n := 0
	for _, el := range elems {
		if el.t != nil {
			n++
		}
	}
	return n
}

// IsElems reports whether e builds the sequence of its elements, a tuple node that a sequence
// containing e flattens: a sequence, possibly grouped or with attributes other than recovery.
func IsElems(p *Checker, e grammar.Expr) bool {
	return isElemsSeen(p, e, nil)
}

func isElemsSeen(p *Checker, e grammar.Expr, seen map[string]bool) bool {
	switch e := UngroupExpr(e).(type) {
	case *grammar.Seq:
		return true
	case *grammar.Attributed:
		return !Recovers(e) && isElemsSeen(p, e.Expr, seen)
	case *grammar.Ref:
		if body, ok := p.SpliceBody(e.Name); ok && !seen[e.Name] {
			if seen == nil {
				seen = map[string]bool{}
			}
			seen[e.Name] = true
			return isElemsSeen(p, body, seen)
		}
	}
	return false
}

// Recovers reports whether e has #recover or #missing, which make it one element.
func Recovers(e *grammar.Attributed) bool {
	for _, a := range e.Attrs {
		if a.Name == "recover" || a.Name == "missing" {
			return true
		}
	}
	return false
}

// SingleMode is how single (value_types.go) takes the value of an expression where it takes one
// element: an alternative, the operand of ?, a capture or a recovered expression.
type SingleMode uint8

const (
	SingleNone  SingleMode = iota // no value
	SinglePass                    // the one element
	SingleTuple                   // the node of the tuple of the elements
)

// SingleModeOf returns how single takes the value of e.
func SingleModeOf(p *Checker, e grammar.Expr) SingleMode {
	return singleMode(ValuedCount(p, e), isSeq(p, e))
}

func singleMode(k int, seq bool) SingleMode {
	switch {
	case k == 0:
		return SingleNone
	case k == 1 && !seq:
		return SinglePass
	}
	return SingleTuple
}

// ScopeTuple reports whether the value of a scope with the body e (a rule body, a repetition
// element or an Operators Rule line) is the node of its tuple, rather than its one element.
func ScopeTuple(p *Checker, e grammar.Expr) bool { return scopeTuple(ShapeElems(p, e), isSeq(p, e)) }

func scopeTuple(elems []ValueElem, seq bool) bool {
	return seq || len(elems) != 1 || elems[0].t == nil || len(elems[0].caps) != 0
}

// ShapeFacts returns what ValuedCount, SingleModeOf and ScopeTuple return for e, from one
// computation of its elements.
func ShapeFacts(p *Checker, e grammar.Expr) (k int, mode SingleMode, tuple bool) {
	elems := ShapeElems(p, e)
	seq := isSeq(p, e)
	k = valuedCount(elems)
	return k, singleMode(k, seq), scopeTuple(elems, seq)
}

// Shaper converts the result of an expression's matcher to the value where it takes one element
// or is a scope.
type Shaper struct {
	Elems  bool // the matcher returns the tuple node of its elements
	Shared bool // the tuple node is the result of a splice rule: a memo entry shares it
	K      int  // the number of valued elements
}

// ShaperOf returns the Shaper of e.
func ShaperOf(p *Checker, e grammar.Expr) Shaper {
	return Shaper{Elems: IsElems(p, e), Shared: SharesTuple(p, e), K: ValuedCount(p, e)}
}

// SharesTuple reports whether the tuple node e builds (isElems) is the result of a splice rule,
// shared through its memo entry: a node that takes it as its own takes a copy.
func SharesTuple(p *Checker, e grammar.Expr) bool {
	for {
		switch x := UngroupExpr(e).(type) {
		case *grammar.Attributed:
			if Recovers(x) {
				return false
			}
			e = x.Expr
		case *grammar.Ref:
			_, ok := p.SpliceBody(x.Name)
			return ok
		default:
			return false
		}
	}
}

// One returns the one element of the result v.
func (s Shaper) One(v *parse.Node) *parse.Node {
	if s.Elems {
		if v == nil || len(value.D(v).Children) == 0 {
			return nil
		}
		return value.D(v).Children[0]
	}
	return v
}
