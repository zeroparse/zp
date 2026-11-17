package exec

import "zeroparse.dev/zp/internal/ir/code"

// The handle type of applying a term.
const (
	// Term is a term of the grammar, a predicate's or a binding's, in the form its target holds: code that a
	// Call with Fn applies to a context. The application returns the value of the term and its failure, which
	// is null when the term does not fail. A term that fails, on a list element or a conversion it cannot
	// make, may leave the values it gathered and made; term.dropKids and term.dropCreated release them.
	// An interruption ends the parse and leaves them too, for the next parse starts without them.
	Term = "Term"
)

// The components of what terms make.
const (
	// Gathered is the stack of the elements gathered for the lists and tuples that terms make.
	Gathered = "gathered"
	// Made is the list of the structs that terms and actions made.
	Made = "made"
)

// QDropped is the number of the entries above the mark of the call, which it drops: elements gathered
// (term.dropKids) or values made (term.dropCreated), or which it takes off the elements gathered
// (term.elemsAbove).
const QDropped = "dropped"

// addTermCode adds the handle type and the operation of applying a term of a predicate or a binding to m,
// which declares the handles Datum, Fail and Act and the component Made (addValue), the register Pos and the
// component Host (memoizedCall and addParsing). A term is lowered from Core IR to a function of Exec IR that each
// target holds in its own form; the type Term, whose application the verifier checks, is how Exec IR names
// that form without writing the function in the module. Applying a term charges nothing itself: what a term
// charges is what the operations of its code charge.
func addTermCode(m *code.Module) {
	datum, fail := code.THandle(Datum), code.THandle(Fail)
	act := code.THandle(Act)
	m.AddCallable(Term, Host, "A term of a predicate or a binding, in the form the target holds: code that is applied to a context.",
		[]code.Type{act}, []code.Type{datum, fail})
	m.AddOp(&code.Op{Name: "term.ctx", Component: Host, Results: []code.Type{act}, Reads: []string{Host, Pos, Made}, Writes: []string{Host}, Host: true,
		Contract: `Returns the context in which a term runs: the current capture frame, an empty span at the
position, and the mark of the values made so far, as term.created returns it.`})
}

// addGathered adds the component Gathered and its operations, which push the elements of the lists and
// tuples that a term makes on the stack, take them off in one piece when the list or the tuple is made,
// and drop the elements of a term that the parser abandons. The stack is the parser's: grammar code that
// collects the elements of a repetition may use it too, above the elements it finds and taking off what
// it pushes, so that what a term finds is what it left.
func addGathered(m *code.Module, act code.Type, items code.Type) {
	m.AddComponent(Gathered, `The elements gathered for the lists and tuples that terms make: a stack, on which
act.push, act.pushAll and term.keep push, from which a list or a tuple takes its elements when it is made
(term.elemsAbove), and from which term.dropKids drops the elements of a term that the parser abandons.`)
	c := code.P("c", act)
	m.AddOp(&code.Op{Name: "act.push", Component: Gathered, Params: []code.Param{c, code.P("e", code.THandle(Elem))},
		Writes: []string{Gathered}, NoReturn: true,
		Contract: `Pushes e on the elements gathered, as the next element of the innermost list or tuple that c is
making.`})
	m.AddOp(&code.Op{Name: "act.pushAll", Component: Gathered, Params: []code.Param{c, code.P("items", items)},
		Writes: []string{Gathered}, NoReturn: true, Cost: code.Cost{Terms: []code.CostTerm{{Coef: 1, Quantity: QSeqLen}}},
		Contract: `Pushes the elements of items in order, as act.push pushes each. A target that bounds the work charges one
unit for each element it copies.`,
		Requires: `items came from seq.items.`})
	m.AddOp(&code.Op{Name: "term.kids", Component: Gathered, Results: []code.Type{code.TInt}, Reads: []string{Gathered},
		Contract: `Returns the number of the elements gathered, which term.dropKids takes as a mark.`})
	m.AddOp(&code.Op{Name: "term.dropKids", Component: Gathered, Params: []code.Param{code.P("mark", code.TInt)},
		Writes: []string{Gathered}, NoReturn: true, Cost: code.Cost{Terms: []code.CostTerm{{Coef: 1, Quantity: QDropped}}},
		Contract: `Drops the elements gathered after the mark, and charges one unit for each.`,
		Requires: `mark came from term.kids and is at most the number of the elements gathered.`})
	m.AddOp(&code.Op{Name: "term.elemsAbove", Component: Gathered, Params: []code.Param{code.P("mark", code.TInt)}, Results: []code.Type{items},
		Reads: []string{Gathered}, Writes: []string{Gathered}, Cost: code.Cost{Terms: []code.CostTerm{{Coef: 2, Quantity: QDropped}}},
		Contract: `Takes the elements gathered after the mark off the elements gathered and returns them in order, for a
list or a tuple to take over. It charges two units for each: one for copying it and one for taking it off, as
term.dropKids charges for dropping it, in one reservation before it moves any.`,
		Requires: `mark came from term.kids and is at most the number of the elements gathered.`})
	m.AddOp(&code.Op{Name: "term.keep", Component: Gathered, Params: []code.Param{code.P("items", items), code.P("i", code.TInt)},
		Writes: []string{Gathered}, NoReturn: true,
		Contract: `Pushes element i of items as it is (a box stays a box), as act.push pushes an element, as the next
element of the list that filter makes.`,
		Requires: `items came from seq.items, and i is at least 0 and less than the number of its elements.`})
}

// addMade adds the component Made and its operations, which record the structs that terms and actions make
// and forget those of a term that the parser abandons. The context of a term or an action holds the mark
// of the list when it is made (term.ctx); a struct recorded after the mark of an action's context that is
// the action's result takes the range of the rule, and the parser then forgets what the action recorded.
func addMade(m *code.Module, act code.Type, n code.Param) {
	m.AddComponent(Made, `The values that terms and actions made: a list of the structs that act.made recorded,
which the context of a term or an action marks when it is made. The struct that an action returns takes the
range of the rule if the action recorded it, and what a term or an action recorded is forgotten when it ends.`)
	m.AddOp(&code.Op{Name: "act.made", Component: Made, Params: []code.Param{code.P("c", act), n}, Writes: []string{Made}, NoReturn: true,
		Contract: `Records n after the values made, as a value that c made.`,
		Requires: `n is a struct that c made, and no act.made recorded it.`})
	m.AddOp(&code.Op{Name: "term.created", Component: Made, Results: []code.Type{code.TInt}, Reads: []string{Made},
		Contract: `Returns the number of the values made, which term.dropCreated takes as a mark.`})
	m.AddOp(&code.Op{Name: "term.dropCreated", Component: Made, Params: []code.Param{code.P("mark", code.TInt)},
		Writes: []string{Made}, NoReturn: true, Cost: code.Cost{Terms: []code.CostTerm{{Coef: 1, Quantity: QDropped}}},
		Contract: `Forgets the values made after the mark, without changing them, and charges one unit for each.`,
		Requires: `mark came from term.created and is at most the number of the values made.`})
}
