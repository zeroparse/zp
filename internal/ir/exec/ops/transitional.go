package ops

import "zeroparse.dev/zp/internal/ir/exec"

// State components of the transitional operations.
const (
	// Exp is the expectation record.
	Exp = "expected"
	// Call is the call bookkeeping of the target outside the pilot: the tracked path of Documents and
	// observation.
	Call = "call"
	// Obs is the observation of a parse.
	Obs = "observe"
	// Growth is the left-recursion growths in progress and the dependency memo of their contexts.
	Growth = "growth"
	// Win is the window of a stream parse: the input read and kept, the elements delivered and the
	// memory the delivered elements held.
	Win = "window"
)

// Names of the handle types of the transitional operations.
const (
	Body = "Body"
	Mark = "ExpMark"
	Set  = "ExpSet"
	Kept = "ExpKept"
	// ExpID identifies one expectation: a description the grammar lists, or a message of @error.
	ExpID = "ExpID"
	// Ctx is the context state of a memoized call (ctx.enter).
	Ctx = "Ctx"
	// Res is a result that a context keeps, a seed included, or the result of a growth.
	Res = "CtxResult"
	// Grow is the state of the growth of a leader (grow.begin).
	Grow = "Grow"
	// Term is a term of the grammar, a predicate's or a binding's, in the form its target evaluates.
	Term = "Term"
)

// Cost quantities of the transitional operations: measures of their arguments and of the state
// before the call.
const (
	// QSet is the number of expectations of the set that the call takes.
	QSet = "set"
	// QMerge is the units that the compositions over the expectation record (keep, mergeKept,
	// mergeExpected, and the calls of expect they make) charge for the sets that the call keeps or
	// merges.
	QMerge = "merge"
	// QDeps is the number of dependencies that the call passes to the enclosing evaluation or stores:
	// those that the frame of the evaluation it ends recorded, and those stored with the results,
	// steps and jumps it reuses, with the observations they lead to.
	QDeps = "deps"
	// QTable is the work of the call's lookups and stores in the context tables at its position: one
	// for each lookup or store; one for each dependency set of the head it scans, each result under
	// the values the set selects and each key value those results compare, a string counting two
	// for each byte compared; and one for each jump it scans for a seed. A lookup that the outcome
	// of an observation leads to counts again on each path through the observations that outcomes
	// depend on.
	QTable = "table"
	// QRun is, for each step of the run of the growth that the call records as jumps (the steps since
	// the last step that read another seed): one, one for each jump recorded before from its seed,
	// and the expectations of its set when its farthest position is that of the steps after it.
	QRun = "run"
	// QSteps is the number of memoized steps and jumps that the call takes.
	QSteps = "steps"
	// QTerm is what evaluating the term of the call charges, which the term's expression decides.
	QTerm = "term"
	// QDropped is the number of values above the mark of the call.
	QDropped = "dropped"
	// QStreamScan is the number of input elements that the call copies when it compacts the buffer,
	// and of those it scans for line starts: every loaded element when the whole input is loaded from
	// its first element and no table of its lines exists yet, otherwise those before its argument
	// that no call scanned.
	QStreamScan = "stream_scan"
	// QStreamFrames is the number of capture frames that the scratch of the stream holds. A target may
	// visit each twice: a scan for the frame in use, which may lie outside the scratch, and a clear of
	// those after it.
	QStreamFrames = "stream_frames"
	// QStreamInactive is the capacity of the trail and the inactive capacity of a target's value
	// stack.
	QStreamInactive = "stream_inactive"
)

// Registers of the parser VM.
const (
	Pos    = "pos"
	Silent = "silent"
)

// Transitional adds to m, a module made by Pilot, the registers the call protocol reads and the
// transitional operations it calls: existing functions of the hand-written runtimes, declared with
// their current behavior and no conformance cases (design 065, Migration Order), until the stage
// that migrates their mechanism replaces them. A rule body runs grammar code, which may change any
// state; the declarations name only the operation's own component and the registers. It also adds
// the host operations of the stream window (streamWindow).
func Transitional(m *exec.Module) {
	m.AddRegister(Pos, exec.TInt, "The position of the parse in the input.")
	m.AddRegister(Silent, exec.TInt, "The depth of lookaheads, inside which expectations are not recorded.")
	m.AddComponent(Exp, `The expectation record: the expectations at the farthest position.`)
	m.AddComponent(Call, `The call bookkeeping of the target outside the call protocol: the tracked path of
Documents and observation.`)
	m.AddComponent(Obs, `The observation of a parse.`)
	m.AddComponent(Growth, `The left-recursion growths in progress with their seeds, the frames that record
what the calls in their contexts depend on, and the tables of the results kept in each context (the
dependency memo).`)

	rule := exec.THandle(Rule)
	body := m.AddHandle(Body, Rules, "The invocation of a rule's body that a generated call passes.")
	mark := m.AddHandle(Mark, Exp, "The record that an isolation set aside.")
	set := m.AddHandle(Set, Exp, "The expectations an isolation recorded, which a target may keep where the record grows next: valid as exp.setAt requires.")
	expID := m.AddHandle(ExpID, Exp, "An expectation: the identifier of a description or message of the grammar.")
	kept := m.AddHandle(Kept, Exp, "Expectations kept beyond the record, as a memo entry keeps them.")
	ctx := m.AddHandle(Ctx, Growth, `The context state of a memoized call: the growths of its rule's cycle at its
position, their present set, and whether the call records its dependencies.`)
	res := m.AddHandle(Res, Growth, `A result that a context keeps (a seed, or the result of a call or a growth step
that read a seed), or the last result of a growth.`)
	grow := m.AddHandle(Grow, Growth, "The state of the growth of a left-recursion leader.")
	value, errs, entry, values := exec.THandle(Value), exec.THandle(Errs), exec.THandle(Entry), exec.THandle(Values)
	result := []exec.Type{value, exec.TBool}
	regs := []string{Pos, Silent}
	add := func(o *exec.Op) {
		o.Transitional = true
		m.AddOp(o)
	}
	ruleArg := []exec.Param{exec.P("r", rule)}
	call := []exec.Param{exec.P("r", rule), exec.P("min", exec.TInt)}

	add(&exec.Op{Name: "call.tracked", Component: Call, Results: []exec.Type{exec.TBool}, Reads: []string{Call},
		Features: []string{Document},
		Contract: `Reports whether a rule call takes the target's tracked path: in a Document parse, and in the
node runtime also in an observed parse.`})
	add(&exec.Op{Name: "call.track", Component: Call, Params: call, Results: result, Writes: append([]string{Call}, regs...),
		Features: []string{Document}, NoReturn: true,
		Contract: `Calls the rule at the binding level min on the target's tracked path and returns its result.`})
	add(&exec.Op{Name: "call.trackBody", Component: Call, Params: []exec.Param{exec.P("r", rule), exec.P("inv", body)},
		Results: result, Writes: append([]string{Call}, regs...), Features: []string{Document}, NoReturn: true,
		Contract: `Calls the rule through the invocation inv on the target's tracked path and returns its
result.`})
	add(&exec.Op{Name: "call.first", Component: Call, Params: ruleArg, Results: []exec.Type{exec.TBool},
		Reads: []string{Call}, Writes: []string{Call}, NoReturn: true, Cost: m.Op("seen.first").Cost,
		Contract: `Records a call of the rule at the position and reports whether its memoization is deferred:
seen.first for a rule whose memoization the target defers, false for one it memoizes eagerly (in a
stream, or for a rule without a seen index).`})

	for _, a := range []struct{ name, doc string }{
		{"rule.memo", "Reports whether the rule is memoized."},
		{"rule.leader", "Reports whether the rule leads a left-recursive cycle."},
		{"rule.plain", "Reports whether the rule has no captures, so that its plain body serves its invocation."},
		{"rule.docMemo", "Reports whether a Document parse memoizes the rule."},
		{"rule.docOnlyMemo", "Reports whether a Document parse memoizes the rule and a parse from scratch does not."},
		{"rule.docPlain", "Reports whether a parse from scratch calls the plain body of the rule directly, at a call site, without the call protocol."},
		{"rule.positional", "Reports whether the results of the rule may depend on positions, not only on the input they examine."},
	} {
		add(&exec.Op{Name: a.name, Component: Rules, Params: ruleArg, Results: []exec.Type{exec.TBool}, Contract: a.doc})
	}
	add(&exec.Op{Name: "rule.id", Component: Rules, Params: ruleArg, Results: []exec.Type{exec.TInt},
		Contract: `Returns the rule's index, the rule of its memo keys.`})
	add(&exec.Op{Name: "rule.invoke", Component: Rules, Params: call, Results: result, Writes: append([]string{Rules}, regs...),
		NoReturn: true,
		Contract: `Runs the rule's body at the binding level min in a frame of its own and makes the rule's
value; on failure the position is unspecified.`})
	add(&exec.Op{Name: "rule.invokePlain", Component: Rules, Params: call, Results: result, Writes: append([]string{Rules}, regs...),
		NoReturn: true,
		Contract: `Runs the plain body of a rule without captures and returns its result, at the start
position on failure.`})
	add(&exec.Op{Name: "rule.run", Component: Rules, Params: []exec.Param{exec.P("inv", body)}, Results: result,
		Writes: append([]string{Rules}, regs...), NoReturn: true,
		Contract: `Runs the invocation inv of a rule at binding level 0 and returns its result.`})

	add(&exec.Op{Name: "exp.isolate", Component: Exp, Params: []exec.Param{exec.P("pos", exec.TInt)}, Results: []exec.Type{mark},
		Writes:   []string{Exp},
		Contract: `Sets the record aside and starts an empty one at pos.`})
	add(&exec.Op{Name: "exp.unisolate", Component: Exp, Params: []exec.Param{exec.P("m", mark)}, Results: []exec.Type{exec.TInt, set},
		Writes: []string{Exp}, Requires: `m is the innermost isolation that has not ended.`,
		Contract: `Ends the record that the isolation m started and restores the one set aside; returns the
farthest position of the ended record and its expectations.`})

	// The operations of recording one expectation.
	expArg := []exec.Param{exec.P("id", expID)}
	add(&exec.Op{Name: "exp.farthest", Component: Exp, Results: []exec.Type{exec.TInt}, Reads: []string{Exp},
		Contract: `Returns the farthest position of the current record.`})
	add(&exec.Op{Name: "exp.recording", Component: Exp, Results: []exec.Type{exec.TBool}, Reads: []string{Exp},
		Contract: `Reports whether the parse records expectations. A parse that does not keeps one placeholder
entry in the record of a failure, which stands for the failure at the farthest position and is not an
expectation.`})
	add(&exec.Op{Name: "exp.reset", Component: Exp, Params: []exec.Param{exec.P("pos", exec.TInt)}, Writes: []string{Exp},
		Contract: `Starts the current record anew at pos: the farthest position becomes pos and the entries are
dropped.`})
	add(&exec.Op{Name: "exp.fresh", Component: Exp, Results: []exec.Type{exec.TBool}, Reads: []string{Exp},
		Contract: `Reports whether the current record has no entry.`})
	add(&exec.Op{Name: "exp.placeholder", Component: Exp, Reads: []string{Exp}, Writes: []string{Exp},
		Contract: `Adds the placeholder entry to the current record.`})
	add(&exec.Op{Name: "exp.maybe", Component: Exp, Params: expArg, Results: []exec.Type{exec.TBool}, Reads: []string{Exp},
		Requires: `id is an expectation, not the placeholder.`,
		Contract: `Reports whether id may be in the current record: false means it is not; true means it may be.`})
	add(&exec.Op{Name: "exp.count", Component: Exp, Results: []exec.Type{exec.TInt}, Reads: []string{Exp},
		Contract: `Returns the number of entries of the current record.`})
	add(&exec.Op{Name: "exp.at", Component: Exp, Params: []exec.Param{exec.P("i", exec.TInt)}, Results: []exec.Type{expID},
		Reads: []string{Exp}, Requires: `0 <= i < exp.count.`,
		Contract: `Returns entry i of the current record, in the order they were added.`})
	add(&exec.Op{Name: "exp.add", Component: Exp, Params: expArg, Reads: []string{Exp}, Writes: []string{Exp},
		Contract: `Adds id to the current record, after its other entries.`})
	add(&exec.Op{Name: "exp.setCount", Component: Exp, Params: []exec.Param{exec.P("s", set)}, Results: []exec.Type{exec.TInt},
		Contract: `Returns the number of expectations of s.`})
	add(&exec.Op{Name: "exp.setAt", Component: Exp, Params: []exec.Param{exec.P("s", set), exec.P("i", exec.TInt)},
		Results: []exec.Type{expID}, Requires: `0 <= i < exp.setCount(s); s is the set that exp.unisolate returned last, no
isolation began or ended since, and every entry written to the record since then was written at an
index below n + i, where n is the number of entries of the record when exp.unisolate returned s.
Merging s into the record entry by entry, reading each entry before the write that follows it, meets
this.`,
		Contract: `Returns expectation i of s, in the order they were recorded.`})
	add(&exec.Op{Name: "exp.keptCount", Component: Exp, Params: []exec.Param{exec.P("k", kept)}, Results: []exec.Type{exec.TInt},
		Contract: `Returns the number of expectations of k.`})
	add(&exec.Op{Name: "exp.keptAt", Component: Exp, Params: []exec.Param{exec.P("k", kept), exec.P("i", exec.TInt)},
		Results: []exec.Type{expID}, Requires: `0 <= i < exp.keptCount(k).`,
		Contract: `Returns expectation i of k, in the order they were recorded.`})

	// The operations of keeping a set beyond the record.
	add(&exec.Op{Name: "exp.empty", Component: Exp, Results: []exec.Type{kept},
		Contract: `Returns the kept set with no expectations.`})
	add(&exec.Op{Name: "exp.last", Component: Exp, Results: []exec.Type{kept}, Reads: []string{Exp},
		Contract: `Returns the set that exp.copy made last in the parse, and the empty set before the first.`})
	add(&exec.Op{Name: "exp.copy", Component: Exp, Params: []exec.Param{exec.P("s", set)}, Results: []exec.Type{kept},
		Reads: []string{Exp}, Writes: []string{Exp}, NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QSet}}},
		Requires: `s has at least one expectation, and exp.setAt may read each of them.`,
		Contract: `Copies the expectations of s, in order, into a set that outlives the record, makes it the set
that exp.copy made last and returns it. The copy is charged as the target charges copies of values,
and the memory it takes as the target accounts it.`})

	entryArg := []exec.Param{exec.P("e", entry)}
	add(&exec.Op{Name: "memo.far", Component: Memo, Params: entryArg, Results: []exec.Type{exec.TInt}, Reads: []string{Memo},
		Requires: `memo.setExpected recorded expectations with e.`,
		Contract: `Returns the farthest position of the expectations of entry e.`})
	add(&exec.Op{Name: "memo.expected", Component: Memo, Params: entryArg, Results: []exec.Type{kept}, Reads: []string{Memo},
		Requires: `memo.setExpected recorded expectations with e.`,
		Contract: `Returns the expectations of entry e.`})
	add(&exec.Op{Name: "memo.setExpected", Component: Memo, Params: append(entryArg, exec.P("far", exec.TInt), exec.P("k", kept)),
		Writes: []string{Memo},
		Contract: `Records the expectations k at far of the call that computed entry e. When memo.splice moves e,
far moves with it, by the same amount as its end.`})
	add(&exec.Op{Name: "memo.forgetDoc", Component: Memo, Params: entryArg, Writes: []string{Memo}, Features: []string{Document},
		Contract: `Forgets the Document record of the entry e, which the call replaces, where the target keeps
the records beside the entries.`})
	ctxArg := exec.P("c", ctx)
	resArg := []exec.Param{exec.P("s", res)}
	add(&exec.Op{Name: "ctx.enter", Component: Growth, Params: call, Results: []exec.Type{ctx, res},
		Reads: []string{Growth, Rules}, Writes: []string{Growth}, NoReturn: true, Cost: exec.Cost{Const: 1},
		Contract: `Begins a memoized call of the rule at the position: returns its context state and, if the
rule is a leader that grows at the position, its current seed, which the call reads (with the
dependency on its identity); otherwise the null result. The context of a call of a rule of a
left-recursive cycle is the growths of its cycle in progress at the position; another call has
none.`})
	add(&exec.Op{Name: "ctx.found", Component: Growth, Params: resArg, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether s is a result, not the null result.`})
	add(&exec.Op{Name: "ctx.ok", Component: Growth, Params: resArg, Results: []exec.Type{exec.TBool}, Reads: []string{Growth},
		Contract: `Returns whether the result s succeeded.`})
	add(&exec.Op{Name: "ctx.value", Component: Growth, Params: resArg, Results: []exec.Type{value}, Reads: []string{Growth},
		Contract: `Returns the value of the result s.`})
	add(&exec.Op{Name: "ctx.replay", Component: Growth, Params: append(resArg, exec.P("seed", exec.TBool)),
		Reads: []string{Growth}, Writes: append([]string{Growth, Rec}, regs...), NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QMerge}, {Coef: 1, Quantity: QErrs}}},
		Contract: `Applies the result s as the call that it records would have: counts a reuse, extends the
examined frontier of a Document parse, adds its expectations unless it is a seed, and on success moves
the position to its end and records its recovered errors again. Outside the missing-token mode it
records them as rec.add does, as one entry that shares their list, at no cost; in that mode it records
them as missing.replay does, one by one when the tokens they inserted repeat.`})
	add(&exec.Op{Name: "ctx.allows", Component: Growth, Params: append([]exec.Param{ctxArg}, entryArg...),
		Results: []exec.Type{exec.TBool}, Reads: []string{Growth, Memo},
		Contract: `Reports whether the call with the context state c may reuse the memo entry e: always outside a
context, and in one only when the leaders growing there are among those that grew where e was stored
(its present set).`})
	add(&exec.Op{Name: "ctx.active", Component: Growth, Params: []exec.Param{ctxArg}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether the call with the context state c is in a context.`})
	add(&exec.Op{Name: "ctx.lookup", Component: Growth, Params: []exec.Param{ctxArg, exec.P("r", rule), exec.P("min", exec.TInt),
		exec.P("key", values)}, Results: []exec.Type{res}, Reads: []string{Growth}, Writes: []string{Growth},
		NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QTable}, {Coef: 1, Quantity: QDeps}}},
		Contract: `Returns the result kept in the context of the call for the rule at the position, the level
and the key that holds there: the seeds it read have the same identities, and the lookaheads it
observed the same outcomes, in the current context; it records those dependencies for the enclosing
call. Otherwise the null result.`,
		Requires: `c is in a context.`})
	add(&exec.Op{Name: "ctx.push", Component: Growth, Params: []exec.Param{ctxArg}, Writes: []string{Growth},
		Contract: `Starts recording the dependencies of the evaluation of the call, if it is in a context.`})
	add(&exec.Op{Name: "ctx.pop", Component: Growth, Params: []exec.Param{ctxArg}, Results: []exec.Type{exec.TInt},
		Reads: []string{Growth}, Writes: []string{Growth},
		Contract: `Ends recording the dependencies of the evaluation and returns where its result goes:
ctx.storeMemo for the memo (it read no seed and made no observation), ctx.storeCtx for the table of
the context, or another value for nowhere (its context's leaders have no present set).`})
	for _, a := range []struct{ name, doc string }{
		{"ctx.storeMemo", "Returns the value of ctx.pop for a result stored in the memo."},
		{"ctx.storeCtx", "Returns the value of ctx.pop for a result kept in the table of its context."},
	} {
		add(&exec.Op{Name: a.name, Component: Growth, Results: []exec.Type{exec.TInt}, Contract: a.doc})
	}
	add(&exec.Op{Name: "memo.setPresent", Component: Memo, Params: append(entryArg, ctxArg), Reads: []string{Growth},
		Writes:   []string{Memo},
		Contract: `Records with the entry e the present set of the context state c, for ctx.allows.`})
	add(&exec.Op{Name: "ctx.keep", Component: Growth, Params: []exec.Param{ctxArg, exec.P("r", rule), exec.P("pos", exec.TInt),
		exec.P("min", exec.TInt), exec.P("key", values), exec.P("ok", exec.TBool), exec.P("end", exec.TInt), exec.P("silent", exec.TBool),
		exec.P("value", value), exec.P("errs", errs), exec.P("far", exec.TInt), exec.P("k", kept)},
		Reads: []string{Growth, Call}, Writes: []string{Growth}, NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QTable}, {Coef: 1, Quantity: QDeps}}},
		Contract: `Keeps the result of the call in the table of its context with the dependencies that ctx.pop
ended, and the examined frontier of a Document parse.`,
		Requires: `The last ctx.pop of c returned ctx.storeCtx.`})
	add(&exec.Op{Name: "ctx.done", Component: Growth, Params: []exec.Param{ctxArg}, Writes: []string{Growth},
		NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QDeps}}},
		Contract: `Passes the dependencies that ctx.pop ended to the evaluation that encloses the call.`})

	add(&exec.Op{Name: "grow.begin", Component: Growth, Params: call, Results: []exec.Type{grow},
		Reads: []string{Growth, Rules}, Writes: []string{Growth}, NoReturn: true, Cost: exec.Cost{Const: 1},
		Contract: `Starts the growth of the leader at the binding level min at the position from a failure,
charging one unit: the growth is in a context when other leaders of its cycle grow at the position.`})
	add(&exec.Op{Name: "grow.next", Component: Growth, Params: []exec.Param{exec.P("g", grow)}, Results: []exec.Type{exec.TBool},
		Reads: []string{Growth}, Writes: append([]string{Growth}, regs...), NoReturn: true,
		Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QSteps}, {Coef: 1, Quantity: QTable}, {Coef: 1, Quantity: QDeps}, {Coef: 1, Quantity: QMerge}, {Coef: 1, Quantity: QRun}}},
		Contract: `Reports whether the body must be evaluated for the next step of the growth g. In a context
it first takes the steps that the memoized steps and jumps of the context give, and starts recording
what the step it reports depends on.`})
	add(&exec.Op{Name: "grow.step", Component: Growth, Params: []exec.Param{exec.P("g", grow), exec.P("value", value),
		exec.P("ok", exec.TBool)}, Reads: []string{Growth}, Writes: append([]string{Growth, Rec}, regs...), NoReturn: true,
		Cost: exec.Cost{Const: 66, Terms: []exec.CostTerm{{Coef: 1, Quantity: QMerge}, {Coef: 1, Quantity: QSegs}, {Coef: 2, Quantity: QDeps}, {Coef: 1, Quantity: QTable}, {Coef: 1, Quantity: QRun}}},
		Contract: `Receives the result of a step of the growth g: adopts it as the seed, charging one unit, and
returns to the start of the growth if the match grew, dropping the entries after the mark that the
growth took when it began, and otherwise ends the growth. A step that succeeded holds the errors it
recovered as the list that rec.since returns for that mark; a step that failed holds none.`})
	add(&exec.Op{Name: "grow.end", Component: Growth, Params: []exec.Param{exec.P("g", grow)}, Results: []exec.Type{res},
		Reads: []string{Growth}, Writes: append([]string{Growth, Rec}, regs...), NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QRun}, {Coef: 1, Quantity: QErrs}}},
		Contract: `Ends the growth g and returns its last seed: the position moves to its end, and its recovered
errors replace the entries recorded since the growth began. Outside the missing-token mode they are
recorded as rec.add does, at no cost; in that mode as missing.replay does.`})

	add(&exec.Op{Name: "obs.memoHit", Component: Obs, Writes: []string{Obs}, Features: []string{Observe},
		Contract: `Counts a reuse of a memo entry in the observation, if the parse is observed.`})
	add(&exec.Op{Name: "obs.active", Component: Obs, Results: []exec.Type{exec.TBool}, Reads: []string{Obs}, Features: []string{Observe},
		Contract: `Reports whether the parse is observed.`})
	add(&exec.Op{Name: "obs.enter", Component: Obs, Params: []exec.Param{exec.P("r", rule), exec.P("pos", exec.TInt), exec.P("min", exec.TInt)},
		Writes: []string{Obs}, Features: []string{Observe}, NoReturn: true,
		Contract: `Reports to the observation of the parse that a call of the rule at pos and binding level
min begins.`,
		Requires: `The parse is observed.`})
	add(&exec.Op{Name: "obs.exit", Component: Obs, Params: []exec.Param{exec.P("ok", exec.TBool), exec.P("pos", exec.TInt),
		exec.P("examined", exec.TInt), exec.P("known", exec.TBool)}, Writes: []string{Obs}, Features: []string{Observe}, NoReturn: true,
		Contract: `Reports to the observation of the parse that the call that the last unmatched obs.enter began
ends, with its success ok, at pos, and with the end of the input it examined if known.`,
		Requires: `The parse is observed, and a call that obs.enter began has not ended.`})
	add(&exec.Op{Name: "doc.examinedBegin", Component: Call, Results: []exec.Type{exec.TInt}, Reads: []string{Call},
		Writes: []string{Call}, Features: []string{Document},
		Contract: `Starts measuring how far an observed call examines the input: in a Document parse, the
frontier of the call starts at the position. Returns the enclosing call's frontier, for
doc.examinedEnd. The call itself, and any memo entry it reads or records, extends the frontier as in
an unobserved parse, so that observation does not change reuse.`})
	add(&exec.Op{Name: "doc.examinedEnd", Component: Call, Params: []exec.Param{exec.P("enclosing", exec.TInt)},
		Results: []exec.Type{exec.TInt, exec.TBool}, Reads: []string{Call}, Writes: []string{Call}, Features: []string{Document},
		Contract: `Ends measuring the frontier of an observed call and merges it into the enclosing one; returns
the call's frontier and true in a Document parse, and false otherwise.`,
		Requires: `enclosing is the result of the matching doc.examinedBegin.`})
	add(&exec.Op{Name: "doc.enter", Component: Call, Params: []exec.Param{exec.P("pos", exec.TInt)},
		Results: []exec.Type{exec.TInt, exec.TInt}, Reads: []string{Call}, Writes: []string{Call}, Features: []string{Document},
		Contract: `Begins a rule call at pos in a Document parse: returns the examined input of the enclosing call,
its end and its start, and starts the call's examined input at pos: as [pos, pos+1), or, in a target
whose matches extend the examined input to the end of each unit they read and whose calls do not
extend it past their end, as the empty range at pos.`})
	add(&exec.Op{Name: "doc.leave", Component: Call, Params: []exec.Param{exec.P("high", exec.TInt), exec.P("low", exec.TInt)},
		Reads: []string{Call}, Writes: []string{Call}, Features: []string{Document},
		Contract: `Ends a rule call in a Document parse: the examined input of the enclosing call becomes the smallest
range that holds [low, high) and the call's.`,
		Requires: `high and low are the results of the matching doc.enter.`})
	doc := []string{Document}
	inDoc := `The parse reads a Document.`
	add(&exec.Op{Name: "doc.active", Component: Call, Results: []exec.Type{exec.TBool}, Reads: []string{Call}, Features: doc,
		Contract: `Reports whether the parse reads a Document.`})
	add(&exec.Op{Name: "doc.touch", Component: Call, Params: []exec.Param{exec.P("end", exec.TInt)}, Reads: []string{Call},
		Writes: []string{Call}, Features: doc,
		Contract: `Extends the examined frontier of the current call to end, if end is past it.`})
	add(&exec.Op{Name: "doc.touchFrom", Component: Call, Params: []exec.Param{exec.P("from", exec.TInt)}, Reads: []string{Call},
		Writes: []string{Call}, Features: doc,
		Contract: `Extends the examined input of the current call back to from, if from is before its start.`})
	add(&exec.Op{Name: "doc.low", Component: Call, Results: []exec.Type{exec.TInt}, Reads: []string{Call},
		Features: doc, Contract: `Returns the start of the input that the current call examined so far.`, Requires: inDoc})
	add(&exec.Op{Name: "doc.high", Component: Call, Results: []exec.Type{exec.TInt}, Reads: []string{Call},
		Features: doc, Contract: `Returns the examined frontier of the current call: the end of the input it examined
so far, exclusive.`, Requires: inDoc})
	add(&exec.Op{Name: "doc.reused", Component: Call, Writes: []string{Call}, Features: doc,
		Contract: `Counts a reused memo entry in the statistics of the Document.`, Requires: inDoc})
	add(&exec.Op{Name: "doc.evaluated", Component: Call, Writes: []string{Call}, Features: doc,
		Contract: `Counts an evaluated rule body in the statistics of the Document.`, Requires: inDoc})
	add(&exec.Op{Name: "call.body", Component: Call, Params: call, Results: result, Writes: append([]string{Call}, regs...),
		NoReturn: true,
		Contract: `Calls the rule at the binding level min without observation: the call, or the plain invocation,
that the target's observed function wraps.`})

	// The host operations of the stream window.
	streamWindow(m, value)
	documentMove(m)
	documentEdits(m)
	documentRuns(m, value)
	addTerms(m, add)
	splitCall(m, add, rule, values, ctx, mark, entry)
	framesAndOperators(m, add, rule, value, errs)
}

// addTerms adds the operations of evaluating a term of a predicate or a binding. What a term means
// stays with each target: these operations are its boundary, the compositions predicate and bind
// what surrounds it.
func addTerms(m *exec.Module, add func(*exec.Op)) {
	term := m.AddHandle(Term, Host, "A term of a predicate or a binding, in the form the target evaluates.")
	datum := exec.THandle(Datum)
	mark := exec.P("mark", exec.TInt)
	dropped := exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QDropped}}}
	add(&exec.Op{Name: "term.eval", Component: Host, Params: []exec.Param{exec.P("t", term)}, Results: []exec.Type{datum, exec.TBool},
		Reads: []string{Host}, Writes: []string{Host}, NoReturn: true,
		Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QTerm}}},
		Contract: `Evaluates t over the current capture frame with an empty span at the position, and returns its
value and true. A term that fails, a list element or a conversion it cannot make, returns false and may
leave the values it gathered and created; term.dropKids and term.dropCreated release them. An interruption
ends the parse and leaves them too, for the next parse starts without them.`})
	add(&exec.Op{Name: "term.created", Component: Host, Results: []exec.Type{exec.TInt}, Reads: []string{Host},
		Contract: `Returns the mark of the nodes that terms and actions created, which term.dropCreated takes.`})
	add(&exec.Op{Name: "term.dropCreated", Component: Host, Params: []exec.Param{mark}, Writes: []string{Host},
		NoReturn: true, Cost: dropped,
		Contract: `Forgets the nodes created after the mark, without changing them, and charges one unit for each.`,
		Requires: `mark came from term.created, and nothing dropped it since.`})
	add(&exec.Op{Name: "term.kids", Component: Host, Results: []exec.Type{exec.TInt}, Reads: []string{Host},
		Contract: `Returns the mark of the values that list built-ins gather, which term.dropKids takes.`})
	add(&exec.Op{Name: "term.dropKids", Component: Host, Params: []exec.Param{mark}, Writes: []string{Host},
		NoReturn: true, Cost: dropped,
		Contract: `Drops the values gathered after the mark and charges one unit for each.`,
		Requires: `mark came from term.kids, and nothing dropped it since.`})
	add(&exec.Op{Name: "term.isFalse", Component: Host, Params: []exec.Param{exec.P("v", datum)}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether v is the boolean false. Any other value, nil among them, is not.`})
	add(&exec.Op{Name: "term.isFunction", Component: Host, Params: []exec.Param{exec.P("v", datum)}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether v is a function, which no variable may hold.`})
}
