package ref

import "zeroparse.dev/zp/grammar"

// Unknown stands for a value that the caller of an evaluation does not know yet, such as a part of the
// input that a generator has not produced. It enters an evaluation only through TermEnv.Captures and
// TermEnv.Vars, as a value or as a part of a node, list, tuple or struct. Every operation that reads it
// makes its result unknown, except that && and || take the value that their known operands settle.
// Neither the result nor the failure of an evaluation that does not read Unknown depends on it.
var Unknown = V{Kind: KUnknown}

// unknownSignal is the panic value of an operation that reads Unknown.
type unknownSignal struct{}

// need stops the evaluation as unknown if one of vs is Unknown.
func need(vs ...V) {
	for _, v := range vs {
		if v.Kind == KUnknown {
			panic(unknownSignal{})
		}
	}
}

// outcome is a set of the outcomes that a term may have as an operand of && , || or !: true, false or
// a failure, which includes a value that is not a bool.
type outcome uint8

const (
	oTrue outcome = 1 << iota
	oFalse
	oFail
	oAny = oTrue | oFalse | oFail
)

// try evaluates t and reports whether its value, or the failure that stops it, is unknown.
func (e *TermEnv) try(t grammar.Term, p params) (v V, msg string, unknown bool) {
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(unknownSignal); ok {
				v, msg, unknown = Unknown, "", true
				return
			}
			f, ok := x.(failure)
			if !ok {
				panic(x)
			}
			v, msg = Nil, string(f)
		}
	}()
	v = e.eval(t, p)
	return v, "", v.Kind == KUnknown
}

// logic applies && or ||. It evaluates as the language does unless an operand that it reads is
// unknown; then the value is the one that every outcome the operands may have gives, and unknown if
// they give more than one or may fail.
func (e *TermEnv) logic(t *grammar.Binary, p params) V {
	var v V
	unknown := func() (unknown bool) {
		defer func() {
			if x := recover(); x != nil {
				if _, ok := x.(unknownSignal); !ok {
					panic(x)
				}
				unknown = true
			}
		}()
		v = e.strictLogic(t, p)
		return false
	}()
	if !unknown {
		return v
	}
	switch e.outcomes(t, p, t.Op) {
	case oTrue:
		return Bool(true)
	case oFalse:
		return Bool(false)
	}
	panic(unknownSignal{})
}

// logical returns the operand of t if t is &&, || or ! (through parentheses), and nil otherwise.
func logical(t grammar.Term) grammar.Term {
	for {
		g, ok := t.(*grammar.TermGroup)
		if !ok {
			break
		}
		t = g.Term
	}
	switch x := t.(type) {
	case *grammar.Binary:
		if x.Op == "&&" || x.Op == "||" {
			return x
		}
	case *grammar.Unary:
		if x.Op == "!" {
			return x
		}
	}
	return nil
}

// outcomes returns the outcomes that t may have as an operand of op, which is &&, || or !.
func (e *TermEnv) outcomes(t grammar.Term, p params, op string) outcome {
	switch x := logical(t).(type) {
	case *grammar.Binary:
		l := e.outcomes(x.L, p, x.Op)
		out := l & oFail
		if x.Op == "&&" {
			if l&oFalse != 0 {
				out |= oFalse
			}
			if l&oTrue != 0 {
				out |= e.outcomes(x.R, p, x.Op)
			}
		} else {
			if l&oTrue != 0 {
				out |= oTrue
			}
			if l&oFalse != 0 {
				out |= e.outcomes(x.R, p, x.Op)
			}
		}
		return out
	case *grammar.Unary:
		in := e.outcomes(x.X, p, "!")
		out := in & oFail
		if in&oTrue != 0 {
			out |= oFalse
		}
		if in&oFalse != 0 {
			out |= oTrue
		}
		return out
	}
	v, msg, unknown := e.try(t, p)
	switch {
	case unknown:
		return oAny
	case msg != "":
		return oFail
	}
	b, msg := Truth(op, v)
	switch {
	case msg != "":
		return oFail
	case b:
		return oTrue
	}
	return oFalse
}

// Decide decides the predicate t: holds is whether it holds, which it does on every value but false and
// a failure, and known is false when holds depends on an unknown value. A failure is known when it
// does not, since a predicate that fails does not hold.
func (e *TermEnv) Decide(t grammar.Term) (holds, known bool) {
	if logical(t) != nil {
		switch o := e.outcomes(t, nil, ""); {
		case o == oTrue:
			return true, true
		case o&oTrue == 0:
			return false, true
		}
		return false, false
	}
	v, msg, unknown := e.try(t, nil)
	if unknown {
		return false, false
	}
	return Holds(v, msg), true
}
