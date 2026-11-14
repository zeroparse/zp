package ops

import "zeroparse.dev/zp/internal/ir/exec"

// The handle type of applying a term.
const (
	// Term is a term of the grammar, a predicate's or a binding's, in the form its target holds: code that a
	// Call with Fn applies to a context. The application returns the value of the term and its failure, which
	// is null when the term does not fail. A term that fails, on a list element or a conversion it cannot
	// make, may leave the values it gathered and created; term.dropKids and term.dropCreated release them.
	// An interruption ends the parse and leaves them too, for the next parse starts without them.
	Term = "Term"
)

// AddTermCode adds the handle type and the operation of applying a term of a predicate or a binding to m,
// which declares the handles Datum, Fail and Act (AddValue), the register Pos and the component Host (Pilot and
// Transitional). A term is lowered from Core IR to a function of Exec IR that each target holds in its
// own form; the type Term, whose application the verifier checks, is how Exec IR names that form
// without writing the function in the module. Applying a term charges nothing itself: what a term
// charges is what the operations of its code charge.
func AddTermCode(m *exec.Module) {
	datum, fail := exec.THandle(Datum), exec.THandle(Fail)
	act := exec.THandle(Act)
	m.AddCallable(Term, Host, "A term of a predicate or a binding, in the form the target holds: code that is applied to a context.",
		[]exec.Type{act}, []exec.Type{datum, fail})
	m.AddOp(&exec.Op{Name: "term.ctx", Component: Host, Results: []exec.Type{act}, Reads: []string{Host, Pos}, Writes: []string{Host}, Host: true,
		Contract: `Returns the context in which a term runs: the current capture frame, an empty span at the
position, and the mark of the nodes created so far.`})
}
