package ops

import "zeroparse.dev/zp/internal/ir/exec"

// QMoved is the size of the value of the entry that the call moves: for each distinct node in it, one
// and the number of its children and fields.
const QMoved = "moved"

// documentMove adds to m the host operation that moves the value of a memo entry that an edit of a
// Document moved (memo.splice). The positions in a value lie outside the parser VM's state, so the
// operation is checked against the charges of the target that implements it, not against a model.
func documentMove(m *exec.Module) {
	entry := exec.THandle(Entry)
	m.AddOp(&exec.Op{Name: "doc.move", Component: Memo, Params: []exec.Param{exec.P("e", entry)},
		Results: []exec.Type{exec.TBool}, Reads: []string{Memo, Host}, Writes: []string{Memo},
		Features: []string{Document}, Host: true, NoReturn: true,
		Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 2, Quantity: QMoved}}},
		Contract: `Makes the value of entry e valid at the position of e, after memo.splice moved e: moves the
positions in the value by as much as the edits moved e since the value was made, in a copy, since the
value may be part of a result that the parse returned. Returns false, changing nothing, when the
target cannot move the value (a value of the host that is not made of nodes and of basic values);
the call that finds e then evaluates the rule again. Returns true for an entry that no edit moved.
The target checks the value before it copies it; each pass charges, for each distinct node it visits,
one unit and one for each child and field, which is what the cost bounds: a refusal charges no more
than the check of the nodes it reached.`})
}
