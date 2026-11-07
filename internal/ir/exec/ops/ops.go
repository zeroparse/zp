// Package ops declares the operations of the ZeroParse parser VM in Exec IR (design 065): their
// signatures, behavior contracts, preconditions, effects and cost bounds. The declarations are
// shared by every target; each target implements them, and the conformance suite
// (internal/ir/exec/conform) checks an implementation against a reference model
// (internal/ir/exec/ref).
//
// The module holds the operations of the pilot set: the memo table, deferred memoization, the
// budget and the nesting depth. Cost quantities are named by the constants Q…, which the reference
// models compute.
package ops

import "zeroparse.dev/zp/internal/ir/exec"

// Features of the parser VM.
const (
	Budget   = "budget"
	Document = "document"
	Observe  = "observe"
	Missing  = "missing"
	Stream   = "stream"
	Typed    = "typed"
)

// State components.
const (
	Memo  = "memo"
	Seen  = "seen"
	Bud   = "budget"
	Depth = "depth"
	Env   = "env"
	// Host is the host: the values and nodes it builds, opaque to the parser VM.
	Host = "host"
	// Rec is the recovered-error list.
	Rec = "recovered"
	// Rules is the grammar's rules: their descriptors and bodies.
	Rules = "rules"
)

// Cost quantities: abstract measures of the state before a call, computed by the reference model.
const (
	// QKeyScan is the sum, over the entries stored at the call's position for any rule, of one more
	// than the number of their key values, a string counting two more for each of its bytes: what
	// scanning the position and comparing keys inspects.
	// It counts the entries that memo.splice removed and that a target may still hold there, because it
	// applies edits to an entry when it finds the entry.
	QKeyScan = "key_scan"
	// QPending is the number of edits since the entry under the call's key was stored or last found,
	// counting an entry that memo.splice removed and a target may still hold: the edits that a target
	// which applies them to an entry when it finds it has yet to apply.
	QPending = "pending_edits"
	// QSlotGrowth is the number of positions the memo covers from its floor before the call when
	// storing at the call's position makes that number cross a power of two, and 0 otherwise: the copy
	// that doubling a position-indexed table stands for. A target whose table keeps a gap for edits
	// counts its slots, the gap among them.
	QSlotGrowth = "slot_growth"
	// QPageIndex is the call's position divided by 1,024.
	QPageIndex = "page_index"
	// QPages is the number of 1,024-position regions of all rules that deferred memoization has
	// recorded calls in.
	QPages = "pages"
	// QSeenIndex is the call's position times the number of rules plus its rule, divided by 64: the
	// size of a record that keeps one bit per call in position order.
	QSeenIndex = "seen_index"
	// QN is the argument n of budget.charge.
	QN = "n"
	// QPruned is the number of memo entries stored before the call's position argument.
	QPruned = "pruned"
	// QCovered is the number of positions the memo covers from its floor: the slots that a target
	// keeps per position pays for when it prunes them, moves the rest and drops the indexes of
	// crowded positions.
	QCovered = "covered"
	// QVars is the number of variables of the rule's memo key.
	QVars = "vars"
	// QEnvReads is QVars times QEnvPath.
	QEnvReads = "env_reads"
	// QEnvPath is the number of nodes on the path to a slot of a persistent vector of the variables
	// with 16 children for each node: the levels above the leaves, and the leaf. It is the same for
	// every slot.
	QEnvPath = "env_path"
	// QEnvRebind is QEnvPath when the value of the call differs from the one the slot holds, and 0
	// when they are equal: the nodes that a rebind copies.
	QEnvRebind = "env_rebind"
	// QErrs is the number of recovered errors that the call copies: those of its argument, or those
	// recovered after its mark.
	QErrs = "errs"
	// QValueCompare is what comparing the value of the call with the one the slot holds inspects: two
	// for each byte of two strings of the same length that are not the same bytes of the same memory,
	// one for two such strings that are (the comparison reads none of them), and 0 for anything else.
	QValueCompare = "value_compare"
)

// Names of handle types.
const (
	Entry  = "Entry"
	Values = "Values"
	// Value is a value or node of a rule's result.
	Value = "Value"
	// Errs is a list of recovered errors that a memo entry keeps.
	Errs = "Errs"
	// Err is one recovered error.
	Err = "Err"
	// Datum is a value that a variable holds or a term produces.
	Datum = "Datum"
	// Scope is a variable environment.
	Scope = "Scope"
	// Rule is a rule's descriptor.
	Rule = "Rule"
)

// Pilot returns the module of the pilot operations, without functions.
func Pilot() *exec.Module {
	m := exec.NewModule("parser", Budget, Document, Observe, Missing, Stream, Typed)
	m.AddComponent(Memo, `The memo table: results of rule calls by key. A key is a rule, a position,
a binding level and the values of the rule's variables.`)
	m.AddComponent(Seen, `The record of deferred memoization: which rules were called at which positions,
and per rule the counts that decide when it is memoized eagerly. It is created with the number of
rules whose memoization is deferred.`)
	m.AddComponent(Bud, `The work budget of the parse: the units charged so far and the limit, zero for
none.`)
	m.AddComponent(Depth, `The nesting depth of rule calls and its limit.`)
	m.AddComponent(Env, `The variable environment: one current value for each declared variable, in a slot
that the grammar fixes. An environment is persistent: a binding makes a new one and changes none that a
backtracking point holds. Values handles stand for the values of a rule's variables, compared by the
host's value equality.`)

	m.AddComponent(Host, `The host: values and nodes, which the parser VM passes on without reading.`)
	m.AddComponent(Rec, `The list of errors recovered so far in the parse.`)
	m.AddComponent(Rules, `The rules of the grammar: their descriptors and bodies.`)

	entry := m.AddHandle(Entry, Memo, "A memo entry: the result of a rule call with its outcome and end.")
	values := m.AddHandle(Values, Env, "The values of a rule's variables, as a memo key.")
	value := m.AddHandle(Value, Host, "The value of a rule's result: a node or a typed value.")
	errs := m.AddHandle(Errs, Rec, "The errors recovered in a rule call, kept with its memo entry.")
	datum := m.AddHandle(Datum, Host, "A value that a variable holds or a term produces, which only the host reads.")
	scope := m.AddHandle(Scope, Env, "A variable environment, which bindings replace and never change.")
	rule := m.AddHandle(Rule, Rules, "A rule's descriptor.")
	result := []exec.Param{exec.P("silent", exec.TBool), exec.P("value", value), exec.P("errs", errs)}
	key := []exec.Param{exec.P("rule", exec.TInt), exec.P("pos", exec.TInt), exec.P("level", exec.TInt), exec.P("key", values)}
	scan := exec.Cost{Const: 1, Terms: []exec.CostTerm{{Coef: 1, Quantity: QKeyScan}, {Coef: 1, Quantity: QPending}}}
	write := exec.Cost{Const: 2, Terms: []exec.CostTerm{{Coef: 1, Quantity: QKeyScan}, {Coef: 1, Quantity: QSlotGrowth}}}
	with := func(ps []exec.Param, more ...exec.Param) []exec.Param {
		return append(append([]exec.Param(nil), ps...), more...)
	}

	m.AddOp(&exec.Op{Name: "memo.lookup", Component: Memo, Params: key, Results: []exec.Type{entry, exec.TBool},
		Reads: []string{Memo}, Cost: scan,
		Contract: `Returns the entry stored under the key, if one is there and not replaced, and true;
otherwise false and an entry that must not be used. Two keys are equal when their rules, positions
and levels are equal and the host's value equality holds for their values.`,
		Laws: []string{"two lookups with no write of memo between them return the same result"}})
	m.AddOp(&exec.Op{Name: "memo.store", Component: Memo, Params: with(with(key, exec.P("ok", exec.TBool), exec.P("end", exec.TInt)), result...),
		Results: []exec.Type{entry}, Reads: []string{Memo}, Writes: []string{Memo}, Cost: write,
		Contract: `Stores a complete entry with the outcome ok, the end, whether it was computed in a silent
context, its value and its recovered errors under the key, replacing the entry stored under the same
key, and returns it. Entries under other keys are unchanged.`,
		Requires: `end is at least pos. The handle of the entry that the store replaces is not used after
it: an implementation may reuse it for a later entry.`})
	m.AddOp(&exec.Op{Name: "memo.prune", Component: Memo, Params: []exec.Param{exec.P("pos", exec.TInt)},
		Reads: []string{Memo}, Writes: []string{Memo}, Features: []string{Stream}, NoReturn: true,
		Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QPruned}, {Coef: 3, Quantity: QCovered}}},
		Contract: `Removes the entries stored at positions before pos and raises the floor of the memo to pos.
Below the floor the memo keeps nothing: a lookup there finds no entry, and a store there returns its
entry without keeping it. The removed entries are no longer valid; entries at or after pos are
unchanged.`,
		Requires: `pos is at least the floor (memo.floor). The handles of the removed entries are not used
after the prune: an implementation may reuse them for later entries.`})
	m.AddOp(&exec.Op{Name: "memo.floor", Component: Memo, Results: []exec.Type{exec.TInt}, Reads: []string{Memo},
		Features: []string{Stream},
		Contract: `Returns the floor of the memo: 0 at the start of a parse, then the position of the last
memo.prune.`})
	doc := []string{Document}
	m.AddOp(&exec.Op{Name: "memo.examine", Component: Memo,
		Params: []exec.Param{exec.P("e", entry), exec.P("before", exec.TBool), exec.P("end", exec.TInt)},
		Reads:  []string{Memo}, Writes: []string{Memo}, Features: doc,
		Contract: `Records with entry e the input that the call which computed it examined, for the reuse of e
after an edit (memo.splice): from the position of e, or from the unit before it if before (a call
reads at most that unit before its position, to tell a line start), to end, exclusive. The record
replaces those that memo.examine and memo.pin made before.`,
		Requires: `end is at or after the position of e.`})
	m.AddOp(&exec.Op{Name: "memo.examined", Component: Memo, Params: []exec.Param{exec.P("e", entry)},
		Results: []exec.Type{exec.TInt}, Reads: []string{Memo}, Features: doc,
		Contract: `Returns the end of the input that memo.examine recorded with entry e, or 0 if none was
recorded.`})
	m.AddOp(&exec.Op{Name: "memo.before", Component: Memo, Params: []exec.Param{exec.P("e", entry)},
		Results: []exec.Type{exec.TBool}, Reads: []string{Memo}, Features: doc,
		Contract: `Reports whether the input that memo.examine recorded with entry e starts before its position;
false if none was recorded.`})
	m.AddOp(&exec.Op{Name: "memo.pin", Component: Memo, Params: []exec.Param{exec.P("e", entry)},
		Reads: []string{Memo}, Writes: []string{Memo}, Features: doc,
		Contract: `Marks entry e as depending on positions, not only on the input it examined: an edit before
it removes it instead of moving it (memo.splice).`})
	m.AddOp(&exec.Op{Name: "memo.splice", Component: Memo,
		Params: []exec.Param{exec.P("start", exec.TInt), exec.P("end", exec.TInt), exec.P("delta", exec.TInt)},
		Reads:  []string{Memo}, Writes: []string{Memo}, Features: doc,
		Contract: `Applies an edit of the input that replaces [start, end) with input delta units longer. An
entry whose recorded examined input ends at or before start stays where it is. An entry whose recorded
examined input starts at or after end, that is not pinned and has no recovered errors, moves by delta:
it is found at its position plus delta, and its end and its examined input move with it, and its
value moves when it is reused (doc.move). Every other entry is removed. The handles of all entries from
before the edit are no longer valid: a target may copy the entries it keeps. What a target records with
an entry besides these moves with it as that record says (memo.setExpected). The operation charges
nothing: a target may apply the edit to an entry when it next finds the entry instead of at once, so
that memo.lookup pays for it, one unit for each edit applied. Lookups then see what they would have
seen had the edit applied at once.`,
		Requires: `0 <= start <= end, and start <= end + delta.`})
	for _, a := range []struct {
		name string
		t    exec.Type
		doc  string
	}{
		{"memo.ok", exec.TBool, "Returns whether the call of entry e succeeded."},
		{"memo.end", exec.TInt, "Returns the end of the match of entry e; for a failure, the position of the call."},
		{"memo.silent", exec.TBool, "Returns whether entry e was computed in a silent context."},
		{"memo.value", value, "Returns the value of entry e."},
		{"memo.errs", errs, "Returns the recovered errors of entry e."},
	} {
		m.AddOp(&exec.Op{Name: a.name, Component: Memo, Params: []exec.Param{exec.P("e", entry)},
			Results: []exec.Type{a.t}, Reads: []string{Memo}, Contract: a.doc})
	}

	m.AddOp(&exec.Op{Name: "seen.first", Component: Seen,
		Params:  []exec.Param{exec.P("rule", exec.TInt), exec.P("pos", exec.TInt)},
		Results: []exec.Type{exec.TBool}, Reads: []string{Seen}, Writes: []string{Seen},
		Cost: exec.Cost{Const: 66, Terms: []exec.CostTerm{{Coef: 1, Quantity: QPageIndex}, {Coef: 1, Quantity: QPages}, {Coef: 1, Quantity: QSeenIndex}}},
		Contract: `Records a call of the rule at pos and reports whether its memoization is deferred: true
when the rule is not eager and has not been called at pos before. Each rule counts its calls and its
repeated calls, the calls at a position where it was called before. A call that finds the rule eager
returns false and records nothing. Otherwise the call count grows by one, and when it reaches 2^31
both counts are halved (rounding down); a repeated call then grows the repeat count, and when the
repeats exceed the calls divided by 16 (rounding down), the rule becomes eager for the rest of the
parse. The policy decides how
many rule bodies run, which the statistics report, so every target follows it exactly.`,
		Requires: `0 <= rule < the number of rules the component was created with; pos >= 0.`})

	m.AddOp(&exec.Op{Name: "budget.charge", Component: Bud, Params: []exec.Param{exec.P("n", exec.TU64)},
		Reads: []string{Bud}, Writes: []string{Bud}, Features: []string{Budget}, NoReturn: true,
		Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QN}}},
		Contract: `Charges n units. When the limit is not zero and the units used plus n exceed it, the parse
ends with a limit interruption and the units used are unchanged; otherwise n is added to them.
Charges are never rolled back. A target may also observe cancellation here, at intervals of its
choice, and end the parse with a cancellation interruption.`,
		Laws: []string{"charge(a); charge(b) may merge into charge(a+b) when nothing observable lies between them and no commit separates them"}})
	m.AddOp(&exec.Op{Name: "budget.used", Component: Bud, Results: []exec.Type{exec.TU64},
		Reads: []string{Bud}, Features: []string{Budget},
		Contract: `Returns the units charged so far: by budget.charge and inside other operations.`})

	m.AddOp(&exec.Op{Name: "budget.freeBegin", Component: Bud, Params: []exec.Param{exec.P("on", exec.TBool)}, Results: []exec.Type{exec.TU64},
		Reads: []string{Bud}, Writes: []string{Bud}, Features: []string{Budget, Document},
		Contract: `Opens a region of Document work when on is true, and returns the mark that budget.freeEnd takes.
The units that the region charges are work that only a Document parse does: a parse under the default
Compute limit leaves them out of the count against it, and a parse under an explicit limit counts them
as any other. The region must end before any work that a parse from scratch counts starts, and a region
does not nest. When on is false, or the parse has no default limit, it opens no region and charges nothing.`})
	m.AddOp(&exec.Op{Name: "budget.freeEnd", Component: Bud, Params: []exec.Param{exec.P("mark", exec.TU64)},
		Reads: []string{Bud}, Writes: []string{Bud}, Features: []string{Budget, Document}, NoReturn: true,
		Contract: `Ends the region that budget.freeBegin opened with mark, if it opened one. It charges nothing.`})

	m.AddOp(&exec.Op{Name: "depth.enter", Component: Depth, Reads: []string{Depth}, Writes: []string{Depth}, NoReturn: true,
		Contract: `Enters a rule call: adds one to the depth. When the depth then exceeds the limit, the parse
ends with a nesting interruption.`})
	m.AddOp(&exec.Op{Name: "depth.exit", Component: Depth, Reads: []string{Depth}, Writes: []string{Depth},
		Contract: `Leaves a rule call: subtracts one from the depth.`,
		Requires: `The depth is greater than zero.`})
	m.AddOp(&exec.Op{Name: "depth.current", Component: Depth, Results: []exec.Type{exec.TInt}, Reads: []string{Depth},
		Contract: `Returns the depth.`})
	addEnvironment(m, datum, scope, rule, values)
	addRecovery(m, errs)
	return m
}

// addRecovery adds the operations of the recovered-error list. A copy charges one unit for each
// error, as a copy of values does; recording an error charges nothing, because making it is what the
// parse pays for, and recovered errors stay outside the cost of the input.
func addRecovery(m *exec.Module, errs exec.Type) {
	err := m.AddHandle(Err, Rec, "A syntax error that a recovery made.")
	mark := []exec.Param{exec.P("mark", exec.TInt)}
	copies := exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QErrs}}}
	const atMost = `mark is at most the number of recovered errors.`
	m.AddOp(&exec.Op{Name: "rec.mark", Component: Rec, Results: []exec.Type{exec.TInt}, Reads: []string{Rec},
		Contract: `Returns the number of recovered errors, which rec.truncate and rec.since take.`})
	m.AddOp(&exec.Op{Name: "rec.push", Component: Rec, Params: []exec.Param{exec.P("e", err)}, Writes: []string{Rec},
		Contract: `Records e after the recovered errors.`})
	m.AddOp(&exec.Op{Name: "rec.truncate", Component: Rec, Params: mark, Writes: []string{Rec},
		Contract: `Drops the errors recovered after the mark.`, Requires: atMost})
	m.AddOp(&exec.Op{Name: "rec.since", Component: Rec, Params: mark, Results: []exec.Type{errs}, Reads: []string{Rec},
		Writes: []string{Rec}, NoReturn: true, Cost: copies,
		Contract: `Returns a copy of the errors recovered after the mark: later writes of the list do not change
it.`, Requires: atMost})
	m.AddOp(&exec.Op{Name: "rec.add", Component: Rec, Params: []exec.Param{exec.P("errs", errs)}, Writes: []string{Rec},
		NoReturn: true, Cost: copies,
		Contract: `Records the errors again, after the recovered errors.`})
	m.AddOp(&exec.Op{Name: "rec.none", Component: Rec, Params: []exec.Param{exec.P("errs", errs)}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether the list of errors is empty.`})
	m.AddOp(&exec.Op{Name: "stream.settleErrors", Component: Rec, Writes: []string{Rec}, Features: []string{Stream},
		NoReturn: true, Cost: exec.Cost{Const: 16},
		Contract: `Forgets the recovered errors, which were delivered and collected, and counts none as delivered.
It charges one unit for each slot it clears of the scratch it keeps for the next element, which holds at
most 16; a larger list is released without clearing it.`,
		Requires: `The parse retains the tail (stream.tail), and no call holds a mark of the list.`})
}

// addEnvironment adds the operations of the variable environment. Each is one step over the
// representation of the environment, so no composition divides them further; a target chooses that
// representation, and shares the key of the initial environment if it likes.
func addEnvironment(m *exec.Module, datum, scope, rule, values exec.Type) {
	slot := exec.P("slot", exec.TInt)
	m.AddOp(&exec.Op{Name: "env.lookup", Component: Env, Params: []exec.Param{slot}, Results: []exec.Type{datum},
		Reads: []string{Env}, NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QEnvPath}}},
		Contract: `Returns the current value of the variable in slot.`,
		Requires: `slot is the slot of a declared variable.`,
		Laws:     []string{"two lookups with no write of env between them return the same value"}})
	m.AddOp(&exec.Op{Name: "env.bind", Component: Env, Params: []exec.Param{slot, exec.P("v", datum)},
		Reads: []string{Env}, Writes: []string{Env}, NoReturn: true,
		Cost: exec.Cost{Const: 1, Terms: []exec.CostTerm{{Coef: 1, Quantity: QValueCompare}, {Coef: 1, Quantity: QEnvRebind}}},
		Contract: `Makes v the value of the variable in slot, in a new environment that shares what it can with
the current one. When the host's value equality holds between v and the value the slot holds, the
environment stays the one it is, so that equal bindings keep what depends on its identity. Environments
that env.scope returned earlier do not change.`,
		Requires: `slot is the slot of a declared variable, and v is a value that it may hold.`})
	m.AddOp(&exec.Op{Name: "env.scope", Component: Env, Results: []exec.Type{scope}, Reads: []string{Env},
		Contract: `Returns the current environment, which env.restore takes.`})
	m.AddOp(&exec.Op{Name: "env.restore", Component: Env, Params: []exec.Param{exec.P("s", scope)}, Writes: []string{Env},
		Contract: `Makes s the current environment: the bindings made since it was current are gone.`})
	m.AddOp(&exec.Op{Name: "env.key", Component: Env, Params: []exec.Param{exec.P("r", rule)}, Results: []exec.Type{values},
		Reads: []string{Env}, NoReturn: true,
		Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QVars}, {Coef: 1, Quantity: QEnvReads}}},
		Contract: `Returns the values of the variables the rule reads, as its memo key. Two keys are equal when
they have the same length and the host's value equality holds for each pair of their values. A rule that
reads no variable has an empty key, which costs nothing: the charge pays for reading values, and the call
paid for itself.`})
}
