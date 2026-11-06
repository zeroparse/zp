package ops

import "zeroparse.dev/zp/internal/ir/exec"

// QMoved is the size of the value of the entry that the call moves: for each distinct node in it, one
// and the number of its children and fields.
const QMoved = "moved"

// QMoveEdits is the edits that the call replays to learn how far a node moves that another result
// moved since the entry's value was made: for each generation of the value's nodes other than the
// entry's, the edits since that generation. It is zero for a target that keeps no generations.
const QMoveEdits = "move_edits"

// Cost quantities of the repetition runs of the engine.
const (
	// QRunSearch is the number of bits of the count of old elements that the run has not passed: what
	// a binary search for the next element inspects.
	QRunSearch = "run_search"
	// QRunScan is the number of old elements from the one the search finds to the end of the run.
	QRunScan = "run_scan"
	// QRunGrowth is the number of new elements the run holds, when one more makes the storage of
	// those elements grow, and 0 otherwise.
	QRunGrowth = "run_growth"
	// QRunElems is the number of elements of the finished run.
	QRunElems = "run_elems"
)

// Names of the handle types of the operations of Document edits and runs.
const (
	// EditLog is the edits of a Document, in order.
	EditLog = "EditLog"
	// Run is a repetition run in progress in a Document parse.
	Run = "Run"
	// RunElem is an element of an old run that the parse reuses.
	RunElem = "RunElem"
	// Site is a repetition that a Document parse can resume.
	Site = "RunSite"
)

// Runs is the repetition runs of a Document: for each repetition of the last parse that a parse may
// resume, the elements it matched.
const Runs = "runs"

// documentMove adds to m the host operation that moves the value of a memo entry that an edit of a
// Document moved (memo.splice). The positions in a value lie outside the parser VM's state, so the
// operation is checked against the charges of the target that implements it, not against a model.
func documentMove(m *exec.Module) {
	entry := exec.THandle(Entry)
	m.AddOp(&exec.Op{Name: "doc.move", Component: Memo, Params: []exec.Param{exec.P("e", entry)},
		Results: []exec.Type{exec.TBool}, Reads: []string{Memo, Host}, Writes: []string{Memo},
		Features: []string{Document}, Host: true, NoReturn: true,
		Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 2, Quantity: QMoved}, {Coef: 1, Quantity: QMoveEdits}}},
		Contract: `Makes the value of entry e valid at the position of e, after memo.splice moved e: moves the
positions in the value by as much as the edits moved e since the value was made. A target that may
not change a value that the parse returned moves a copy; one whose contract says that a reuse moves
the nodes of earlier results moves them in place, where each node records how many edits its
positions account for. Returns false, changing nothing, when the target cannot move the value (a
value of the host that is not made of nodes and of basic values); the call that finds e then evaluates
the rule again. Returns true for an entry that no edit moved.
The cost has two parts. Each pass over the value charges, for each distinct node it visits, one unit
and one for each child and field. A target that copies checks the value and then copies it, in two
passes, and a refusal charges no more than the check of the nodes it reached. A target that moves in
place makes one pass, charges a second unit for each reference to an empty node, which it copies,
and for each copy it forgets at the end, and replays the edits since the generation of a node that
another result moved, once for each generation.`})
}

// documentEdits adds to m the host operations of the edits that a target applies to a memo entry when
// it finds the entry. They exist only on such a target, as what its implementation of memo.splice
// reads and writes, and have no reference model: a lazy edit changes no result that a composition
// could observe, so conformance checks them through memo.splice and memo.lookup.
func documentEdits(m *exec.Module) {
	entry := exec.THandle(Entry)
	log := m.AddHandle(EditLog, Memo, "The edits of a Document, in order.")
	host := func(o *exec.Op) {
		o.Host = true
		o.Features = []string{Document}
		m.AddOp(o)
	}
	attribute := func(name string, t exec.Type, doc string) {
		host(&exec.Op{Name: name, Component: Memo, Params: []exec.Param{exec.P("e", entry)}, Results: []exec.Type{t},
			Reads: []string{Memo}, Contract: doc})
	}
	attribute("memo.seed", exec.TBool, `Reports whether entry e is the seed of a growth, which no edit keeps. It guards memoAdvance: a seed
lives within one parse, when no edit is pending, so the guard is not reached in a parse; only a case of
conformance, which stores seeds without a parse, reaches it.`)
	attribute("memo.from", exec.TInt, `Returns the start of the input that entry e examined, as of the edits it accounts for.`)
	attribute("memo.pinned", exec.TBool, `Reports whether a call pinned entry e (memo.pin).`)
	attribute("memo.shifted", exec.TBool, `Reports whether an edit moved entry e since its value was last made valid (doc.move).`)
	attribute("memo.applied", exec.TInt, `Returns the number of edits that entry e accounts for.`)
	host(&exec.Op{Name: "memo.advance", Component: Memo, Writes: []string{Memo},
		Params: []exec.Param{exec.P("e", entry), exec.P("from", exec.TInt), exec.P("examined", exec.TInt), exec.P("shift", exec.TInt),
			exec.P("shifted", exec.TBool), exec.P("applied", exec.TInt)},
		Contract: `Records that entry e accounts for the first applied edits: its examined input is [from, examined),
its position, its end and the far position of its expectations move by shift, the shift of its value
grows by shift, and shifted tells whether its value has to move.`})
	count := []exec.Param{exec.P("log", log)}
	at := []exec.Param{exec.P("log", log), exec.P("i", exec.TInt)}
	host(&exec.Op{Name: "edit.count", Component: Memo, Params: count, Results: []exec.Type{exec.TInt}, Reads: []string{Memo},
		Contract: `Returns the number of edits of the log.`})
	for _, a := range [][2]string{{"edit.start", "Returns the start of the input that edit i replaced."},
		{"edit.end", "Returns the end of the input that edit i replaced."},
		{"edit.delta", "Returns how many units longer the replacement of edit i is than the input it replaced."}} {
		host(&exec.Op{Name: a[0], Component: Memo, Params: at, Results: []exec.Type{exec.TInt}, Reads: []string{Memo},
			Contract: a[1], Requires: `0 <= i < edit.count(log).`})
	}
}

// documentRuns adds to m the host operations of the repetition runs of a Document. Runs exist in
// the engine alone. A backend calls these operations where a repetition of a Document parse starts,
// reuses elements, and runs its element expression, which they leave to it, so that none of them runs
// grammar code. Their effects on the parse are those of evaluating the elements they stand for; what
// to reuse is decided from the run, not from a composition. No target declares a reference model of
// them, and tests of the engine measure their charges at these bounds.
func documentRuns(m *exec.Module, value exec.Type) {
	m.AddComponent(Runs, `The repetition runs of a Document: for each repetition of the last parse that a parse may
resume, the positions, the examined input, the value and the expectations of its elements.`)
	run := m.AddHandle(Run, Runs, "A repetition run in progress in a Document parse.")
	elem := m.AddHandle(RunElem, Runs, "An element of an old run that the parse reuses, or none.")
	site := m.AddHandle(Site, Runs, "A repetition that a Document parse can resume.")
	host := func(o *exec.Op) {
		o.Host = true
		o.Component = Runs
		o.Reads = []string{Runs}
		o.Writes = []string{Runs}
		o.Features = []string{Document}
		m.AddOp(o)
	}
	reuse := exec.Cost{Const: 1, Terms: []exec.CostTerm{{Coef: 1, Quantity: QMerge}}}
	moved := exec.Cost{Const: 1, Terms: []exec.CostTerm{{Coef: 1, Quantity: QMerge}, {Coef: 2, Quantity: QMoved}, {Coef: 1, Quantity: QMoveEdits}}}
	host(&exec.Op{Name: "run.start", Params: []exec.Param{exec.P("r", run), exec.P("site", site)},
		Contract: `Starts the run r of the repetition at the position. It takes over the record that this parse left
there, or the record of the previous parse here or where the one edit since moved it; with no such
record it has nothing to reuse.`})
	host(&exec.Op{Name: "run.prefix", Params: []exec.Param{exec.P("r", run)}, Results: []exec.Type{elem}, NoReturn: true, Cost: reuse,
		Contract: `Reuses the next old element if it examined only input before the one edit since, replaying what
running it did: the examined input, the expectations, the position, the reuse statistics. Returns it,
or none when the prefix of the run ends.`})
	host(&exec.Op{Name: "run.resync", Params: []exec.Param{exec.P("r", run)}, Results: []exec.Type{exec.TBool}, NoReturn: true,
		Cost:     exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QRunSearch}, {Coef: 1, Quantity: QRunScan}}},
		Contract: `Reports whether an old element after the edit starts where the next element would, and every element from it on examined only input after the edit; run.tail then reuses them.`})
	host(&exec.Op{Name: "run.tail", Params: []exec.Param{exec.P("r", run)}, Results: []exec.Type{elem}, NoReturn: true, Cost: moved,
		Contract: `Reuses the next old element after the edit, moved by the edit with its value (doc.move), replaying
what running it did as run.prefix does. Returns it, or none at the end of the run or its bound.`})
	host(&exec.Op{Name: "run.begin", Params: []exec.Param{exec.P("r", run)},
		Contract: `Starts evaluating an element: its examined input and its expectations are recorded apart from the enclosing call's.`})
	host(&exec.Op{Name: "run.end", Params: []exec.Param{exec.P("r", run), exec.P("ok", exec.TBool), exec.P("v", value)}, NoReturn: true,
		Cost:     exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QMerge}, {Coef: 1, Quantity: QRunGrowth}}},
		Contract: `Ends evaluating an element, adding its examined input and its expectations to those of the enclosing call, and records it if it matched with the value v.`})
	host(&exec.Op{Name: "run.finish", Params: []exec.Param{exec.P("r", run)}, NoReturn: true,
		Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QRunElems}}},
		Contract: `Records the run of a repetition that matched for the next parse, unless it is too short, or recovered
errors, or read the seed of a growth.`})
}

// PendingEditLimit is the number of edits that a memo entry may lag behind when a lookup finds it. A
// lookup applies the edits it does not account for to the entry, one step for each; past this limit
// the entry is dropped and the call evaluates again, whose cost is that of the work the entry stands
// for, so the work of applying edits stays within a constant factor of the work a parse from
// scratch does. Where edits are spread over a 2,000-line input, a parse that reuses a memo that lags by
// 64 edits costs about a third of a parse from scratch and one that lags by 256 edits costs as much;
// where they land in a few places, reuse pays at every count measured, up to 1,024, and the limit gives
// that up for the bound.
const PendingEditLimit = 128
