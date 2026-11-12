# Operations

This chapter lists the operations on the state of a parse, by the state component they belong to; [Values](values.md)
lists those on the values of terms. Each entry gives the declaration on one line, as the [text form](text-form.md#catalog-entries)
writes it, the components and registers the operation reads and writes, its cost bound (see
[Costs](machine.md#costs)) and its contract, with its precondition and its laws where it has them.

## Memo Table

The operations of the component `memo`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op memo.lookup(rule int, pos int, level int, key Values, docOnly bool) (Entry, bool)` | reads memo | `1 + 1*key_scan + 1*pending_edits` | Returns the entry stored under the key, if one is there and not replaced, and true; otherwise false and an entry that must not be used. Two keys are equal when their rules, positions and levels are equal and the host's value equality holds for their values. A lookup with docOnly set, for a rule that only a Document memoizes, finds an entry only if the values of the key are the same values as those of the entry (the same bytes for a string), not merely equal, and finds none beyond the first entries of a long chain: a miss is always sound, since the rule is then evaluated. Law: two lookups with no write of memo between them return the same result. |
| `op memo.store(rule int, pos int, level int, key Values, ok bool, end int, silent bool, value Value, errs Errs, docOnly bool) Entry` | reads memo; writes memo | `2 + 1*key_scan + 1*slot_growth` | Stores a complete entry with the outcome ok, the end, whether it was computed in a silent context, its value and its recovered errors under the key, replacing the entry stored under the same key, and returns it. Entries under other keys are unchanged. With docOnly set, the key of a stored entry matches only the key of a lookup with the same values (see memo.lookup), so the store replaces the entry of the same values only. Requires: end is at least pos. The handle of the entry that the store replaces is not used after it: an implementation may reuse it for a later entry. |
| `op memo.prune(pos int) [stream] noreturn` | reads memo; writes memo | `0 + 1*pruned + 3*covered` | Removes the entries stored at positions before pos and raises the floor of the memo to pos. Below the floor the memo keeps nothing: a lookup there finds no entry, and a store there returns its entry without keeping it. The removed entries are no longer valid; entries at or after pos are unchanged. Requires: pos is at least the floor (memo.floor). The handles of the removed entries are not used after the prune: an implementation may reuse them for later entries. |
| `op memo.floor() int [stream]` | reads memo | `0` | Returns the floor of the memo: 0 at the start of a parse, then the position of the last memo.prune. |
| `op memo.examine(e Entry, before bool, end int) [document]` | reads memo; writes memo | `0` | Records with entry e the input that the call which computed it examined, for the reuse of e after an edit (memo.splice): from the position of e, or from the unit before it if before (a call reads at most that unit before its position, to tell a line start), to end, exclusive. The record replaces those that memo.examine and memo.pin made before. Requires: end is at or after the position of e. |
| `op memo.examined(e Entry) int [document]` | reads memo | `0` | Returns the end of the input that memo.examine recorded with entry e, or 0 if none was recorded. |
| `op memo.before(e Entry) bool [document]` | reads memo | `0` | Reports whether the input that memo.examine recorded with entry e starts before its position; false if none was recorded. |
| `op memo.pin(e Entry) [document]` | reads memo; writes memo | `0` | Marks entry e as depending on positions, not only on the input it examined: an edit before it removes it instead of moving it (memo.splice). |
| `op memo.splice(start int, end int, delta int) [document]` | reads memo; writes memo | `0` | Applies an edit of the input that replaces [start, end) with input delta units longer. An entry whose recorded examined input ends at or before start stays where it is. An entry whose recorded examined input starts at or after end, that is not pinned and has no recovered errors, moves by delta: it is found at its position plus delta, and its end and its examined input move with it, and its value moves when it is reused (doc.move). Every other entry is removed. The handles of all entries from before the edit are no longer valid: a target may copy the entries it keeps. What a target records with an entry besides these moves with it as that record says (memo.setExpected). The operation charges nothing: a target may apply the edit to an entry when it next finds the entry instead of at once, so that memo.lookup pays for it, one unit for each edit applied. Lookups then see what they would have seen had the edit applied at once. Requires: 0 <= start <= end, and start <= end + delta. |
| `op memo.ok(e Entry) bool` | reads memo | `0` | Returns whether the call of entry e succeeded. |
| `op memo.end(e Entry) int` | reads memo | `0` | Returns the end of the match of entry e; for a failure, the position of the call. |
| `op memo.silent(e Entry) bool` | reads memo | `0` | Returns whether entry e was computed in a silent context. |
| `op memo.value(e Entry) Value` | reads memo | `0` | Returns the value of entry e. |
| `op memo.errs(e Entry) Errs` | reads memo | `0` | Returns the recovered errors of entry e. |
| `transitional op memo.far(e Entry) int` | reads memo | `0` | Returns the farthest position of the expectations of entry e. Requires: memo.setExpected recorded expectations with e. |
| `transitional op memo.expected(e Entry) ExpKept` | reads memo | `0` | Returns the expectations of entry e. Requires: memo.setExpected recorded expectations with e. |
| `transitional op memo.setExpected(e Entry, far int, k ExpKept)` | writes memo | `0` | Records the expectations k at far of the call that computed entry e. When memo.splice moves e, far moves with it, by the same amount as its end. |
| `transitional op memo.forgetDoc(e Entry) [document]` | writes memo | `0` | Forgets the Document record of the entry e, which the call replaces, where the target keeps the records beside the entries. |
| `op memo.setPresent(e Entry, c Ctx)` | reads growth; writes memo | `0` | Records with the entry e the present set of the context state c, for ctx.allows. |
| `host op doc.move(e Entry) bool [document] noreturn` | reads memo, host; writes memo | `0 + 2*moved + 1*move_edits` | Makes the value of entry e valid at the position of e, after memo.splice moved e: moves the positions in the value by as much as the edits moved e since the value was made. A target that may not change a value that the parse returned moves a copy; one whose contract says that a reuse moves the nodes of earlier results moves them in place, where each node records how many edits its positions account for. Returns false, changing nothing, when the target cannot move the value (a value of the host that is not made of nodes and of basic values); the call that finds e then evaluates the rule again. Returns true for an entry that no edit moved. The cost has two parts. Each pass over the value charges, for each distinct node it visits, one unit and one for each child and field. A target that copies checks the value and then copies it, in two passes, and a refusal charges no more than the check of the nodes it reached. A target that moves in place makes one pass, charges a second unit for each reference to an empty node, which it copies, and for each copy it forgets at the end, and replays the edits since the generation of a node that another result moved, once for each generation. |
| `host op memo.seed(e Entry) bool [document]` | reads memo | `0` | Reports whether entry e is the seed of a growth, which no edit keeps. It guards memoAdvance: a seed lives within one parse, when no edit is pending, so the guard is not reached in a parse; only a case of conformance, which stores seeds without a parse, reaches it. |
| `host op memo.from(e Entry) int [document]` | reads memo | `0` | Returns the start of the input that entry e examined, as of the edits it accounts for. |
| `host op memo.pinned(e Entry) bool [document]` | reads memo | `0` | Reports whether a call pinned entry e (memo.pin). |
| `host op memo.shifted(e Entry) bool [document]` | reads memo | `0` | Reports whether an edit moved entry e since its value was last made valid (doc.move). |
| `host op memo.applied(e Entry) int [document]` | reads memo | `0` | Returns the number of edits that entry e accounts for. |
| `host op memo.advance(e Entry, from int, examined int, shift int, shifted bool, applied int) [document]` | writes memo | `0` | Records that entry e accounts for the first applied edits: its examined input is [from, examined), its position, its end and the far position of its expectations move by shift, the shift of its value grows by shift, and shifted tells whether its value has to move. |
| `host op edit.count(log EditLog) int [document]` | reads memo | `0` | Returns the number of edits of the log. |
| `host op edit.start(log EditLog, i int) int [document]` | reads memo | `0` | Returns the start of the input that edit i replaced. Requires: 0 <= i < edit.count(log). |
| `host op edit.end(log EditLog, i int) int [document]` | reads memo | `0` | Returns the end of the input that edit i replaced. Requires: 0 <= i < edit.count(log). |
| `host op edit.delta(log EditLog, i int) int [document]` | reads memo | `0` | Returns how many units longer the replacement of edit i is than the input it replaced. Requires: 0 <= i < edit.count(log). |

## Deferred Memoization

The operations of the component `seen`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op seen.first(rule int, pos int) bool` | reads seen; writes seen | `66 + 1*page_index + 1*pages + 1*seen_index` | Records a call of the rule at pos and reports whether its memoization is deferred: true when the rule is not eager and has not been called at pos before. Each rule counts its calls and its repeated calls, the calls at a position where it was called before. A call that finds the rule eager returns false and records nothing. Otherwise the call count grows by one, and when it reaches 2^31 both counts are halved (rounding down); a repeated call then grows the repeat count, and when the repeats exceed the calls divided by 16 (rounding down), the rule becomes eager for the rest of the parse. The policy decides how many rule bodies run, which the statistics report, so every target follows it exactly. Requires: 0 <= rule < the number of rules the component was created with; pos >= 0. |

## Budget

The operations of the component `budget`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op budget.charge(n u64) [budget] noreturn` | reads budget; writes budget | `0 + 1*n` | Charges n units, as n charges of one unit do. When the limit is not zero and the units used plus n exceed it, the parse ends with a limit interruption at the first unit that does not fit, which reports the limit as the units used and one unit requested; otherwise n is added to them. Charges are never rolled back. A target may also observe cancellation here, at intervals of its choice, and end the parse with a cancellation interruption. Law: charge(a); charge(b) may merge into charge(a+b) when nothing observable lies between them and no commit separates them. |
| `op budget.used() u64 [budget]` | reads budget | `0` | Returns the units charged so far: by budget.charge and inside other operations. |
| `op budget.freeBegin(on bool) u64 [budget, document]` | reads budget; writes budget | `0` | Opens a region of Document work when on is true, and returns the mark that budget.freeEnd takes. The units that the region charges are work that only a Document parse does: a parse under the default Compute limit leaves them out of the count against it, and a parse under an explicit limit counts them as any other. The region must end before any work that a parse from scratch counts starts, and a region does not nest. An operation that does work a parse from scratch counts inside a region, such as the hash of a memo key and its comparison with each entry of the walk, counts it: it lifts the region for that charge and restores it. When on is false, or the parse has no default limit, it opens no region and charges nothing. |
| `op budget.freeEnd(mark u64) [budget, document] noreturn` | reads budget; writes budget | `0` | Ends the region that budget.freeBegin opened with mark, if it opened one. It charges nothing. |
| `host op stream.check() [stream] noreturn` | reads budget; writes budget | `0` | Observes cancellation and the limits of the parse at a stream boundary, and ends the parse with the interruption that applies. |

## Nesting Depth

The operations of the component `depth`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op depth.enter() noreturn` | reads depth; writes depth | `0` | Enters a rule call: adds one to the depth. When the depth then exceeds the limit, the parse ends with a nesting interruption. |
| `op depth.exit()` | reads depth; writes depth | `0` | Leaves a rule call: subtracts one from the depth. Requires: The depth is greater than zero. |
| `op depth.current() int` | reads depth | `0` | Returns the depth. |

## Variable Environment

The operations of the component `env`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op env.lookup(slot int) Datum noreturn` | reads env | `0 + 1*env_path` | Returns the current value of the variable in slot. Requires: slot is the slot of a declared variable. Law: two lookups with no write of env between them return the same value. |
| `op env.bind(slot int, v Datum) noreturn` | reads env; writes env | `1 + 1*value_compare + 1*env_rebind` | Makes v the value of the variable in slot, in a new environment that shares what it can with the current one. When the host's value equality holds between v and the value the slot holds, the environment stays the one it is, so that equal bindings keep what depends on its identity. Environments that env.scope returned earlier do not change. Requires: slot is the slot of a declared variable, and v is a value that it may hold. |
| `op env.scope() Scope` | reads env | `0` | Returns the current environment, which env.restore takes. |
| `op env.restore(s Scope)` | writes env | `0` | Makes s the current environment: the bindings made since it was current are gone. |
| `op env.key(r Rule) Values noreturn` | reads env | `0 + 1*vars + 1*env_reads` | Returns the values of the variables the rule reads, as its memo key. Two keys are equal when they have the same length and the host's value equality holds for each pair of their values. A rule that reads no variable has an empty key, which costs nothing: the charge pays for reading values, and the call paid for itself. |

## Recovered Errors

The operations of the component `recovered`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op rec.mark() int` | reads recovered | `0` | Returns the number of entries of the list, which rec.truncate and rec.since take. |
| `op rec.push(e Err)` | writes recovered | `0` | Records e after the recovered errors, as one entry. |
| `op rec.truncate(mark int)` | writes recovered | `0` | Drops the entries after the mark, and with them their errors. Requires: mark is a mark of the list: a result of rec.mark at most its current length, and no rec.since with a smaller mark has run since rec.mark returned it. |
| `op rec.since(mark int) Errs noreturn` | reads recovered; writes recovered | `0 + 1*segs` | Returns the list of the errors recovered after the mark, and puts it in place of the entries after the mark, so that the errors stay the same and the list has one entry after the mark. When there is one entry after the mark, an error or a list, returns it as the list, and the list does not change. When there is none, returns an empty list and the list does not change. A list never changes once made: later writes of the list do not change it. Requires: mark is a mark of the list: a result of rec.mark at most its current length, and no rec.since with a smaller mark has run since rec.mark returned it. |
| `op rec.add(errs Errs)` | writes recovered | `0` | Records the errors of errs again, after the recovered errors, as one entry that shares errs; an empty list records nothing. |
| `op rec.none(errs Errs) bool` | none | `0` | Reports whether the list of errors is empty. |
| `op stream.settleErrors() [stream] noreturn` | writes recovered | `16` | Forgets the recovered errors, which were delivered and collected, and counts none as delivered. It charges one unit for each slot it clears of the scratch it keeps for the next element, which holds at most 16; a larger list is released without clearing it. Requires: The parse retains the tail (stream.tail), and no call holds a mark of the list. |
| `op missing.active() bool [missing]` | reads recovered | `0` | Reports whether the parse runs in the missing-token mode. |
| `op missing.recompute(r Rule) bool [missing]` | reads recovered, rules | `0` | Reports whether a result of the rule is evaluated again instead of being reused from the memo: the parse runs in the missing-token mode and the rule or a rule it calls can insert a missing token. |
| `op missing.mark() int [missing]` | reads recovered | `0` | Returns a mark of the missing-token record, for missing.reset. |
| `op missing.reset(mark int) [missing]` | writes recovered | `0 + 1*tokens` | Removes the tokens inserted after mark, in the missing-token mode, charging one unit for each; does nothing outside it. Requires: mark is a result of missing.mark at or before every later mark that is still in use. |
| `op missing.replay(e Entry) [missing] noreturn` | writes recovered | `0 + 1*tokens + 1*errs` | Records again the tokens that the call of entry e inserted and the errors it recovered, as rec.add does for the errors outside the missing-token mode. It charges one unit for each token; when one of the tokens is already present, it records the errors one by one without that token's, for one unit each, and otherwise it shares them as rec.add does. Requires: The parse runs in the missing-token mode. |
| `transitional op opr.replay(a Attempt) [missing] noreturn` | reads operators; writes recovered | `0 + 1*errs` | Records again the tokens that the match a inserted and the errors it recovered, as rec.add does for the errors outside the missing-token mode. |

## Expectations

The operations of the component `expected`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `transitional op exp.isolate(pos int) ExpMark` | writes expected | `0` | Sets the record aside and starts an empty one at pos. |
| `transitional op exp.unisolate(m ExpMark) (int, ExpSet)` | writes expected | `0` | Ends the record that the isolation m started and restores the one set aside; returns the farthest position of the ended record and its expectations. Requires: m is the innermost isolation that has not ended. |
| `transitional op exp.farthest() int` | reads expected | `0` | Returns the farthest position of the current record. |
| `transitional op exp.recording() bool` | reads expected | `0` | Reports whether the parse records expectations. A parse that does not keeps one placeholder entry in the record of a failure, which stands for the failure at the farthest position and is not an expectation. |
| `transitional op exp.reset(pos int)` | writes expected | `0` | Starts the current record anew at pos: the farthest position becomes pos and the entries are dropped. |
| `transitional op exp.fresh() bool` | reads expected | `0` | Reports whether the current record has no entry. |
| `transitional op exp.placeholder()` | reads expected; writes expected | `0` | Adds the placeholder entry to the current record. |
| `transitional op exp.maybe(id ExpID) bool` | reads expected | `0` | Reports whether id may be in the current record: false means it is not; true means it may be. Requires: id is an expectation, not the placeholder. |
| `transitional op exp.count() int` | reads expected | `0` | Returns the number of entries of the current record. |
| `transitional op exp.at(i int) ExpID` | reads expected | `0` | Returns entry i of the current record, in the order they were added. Requires: 0 <= i < exp.count. |
| `transitional op exp.add(id ExpID)` | reads expected; writes expected | `0` | Adds id to the current record, after its other entries. |
| `transitional op exp.setCount(s ExpSet) int` | none | `0` | Returns the number of expectations of s. |
| `transitional op exp.setAt(s ExpSet, i int) ExpID` | none | `0` | Returns expectation i of s, in the order they were recorded. Requires: 0 <= i < exp.setCount(s); s is the set that exp.unisolate returned last, no isolation began or ended since, and every entry written to the record since then was written at an index below n + i, where n is the number of entries of the record when exp.unisolate returned s. Merging s into the record entry by entry, reading each entry before the write that follows it, meets this. |
| `transitional op exp.keptCount(k ExpKept) int` | none | `0` | Returns the number of expectations of k. |
| `transitional op exp.keptAt(k ExpKept, i int) ExpID` | none | `0` | Returns expectation i of k, in the order they were recorded. Requires: 0 <= i < exp.keptCount(k). |
| `transitional op exp.empty() ExpKept` | none | `0` | Returns the kept set with no expectations. |
| `transitional op exp.last() ExpKept` | reads expected | `0` | Returns the set that exp.copy made last in the parse, and the empty set before the first. |
| `transitional op exp.copy(s ExpSet) ExpKept noreturn` | reads expected; writes expected | `0 + 1*set` | Copies the expectations of s, in order, into a set that outlives the record, makes it the set that exp.copy made last and returns it. The copy is charged as the target charges copies of values, and the memory it takes as the target accounts it. Requires: s has at least one expectation, and exp.setAt may read each of them. |

## Left-Recursion Growth

The operations of the component `growth`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op ctx.enter(r Rule, min int) (Ctx, CtxResult) noreturn` | reads growth, rules; writes growth | `1` | Begins a memoized call of the rule at the position: returns its context state and, if the rule is a leader that grows at the position, its current seed, which the call reads (with the dependency on its identity); otherwise the null result. The context of a call of a rule of a left-recursive cycle is the growths of its cycle in progress at the position; another call has none. |
| `op ctx.found(s CtxResult) bool` | none | `0` | Reports whether s is a result, not the null result. |
| `op ctx.ok(s CtxResult) bool` | reads growth | `0` | Returns whether the result s succeeded. |
| `op ctx.value(s CtxResult) Value` | reads growth | `0` | Returns the value of the result s. |
| `transitional op ctx.replay(s CtxResult, seed bool) noreturn` | reads growth; writes growth, recovered, pos, silent | `0 + 1*merge + 1*errs` | Applies the result s as the call that it records would have: counts a reuse, extends the examined frontier of a Document parse, adds its expectations unless it is a seed, and on success moves the position to its end and records its recovered errors again. Outside the missing-token mode it records them as rec.add does, as one entry that shares their list, at no cost; in that mode it records them as missing.replay does, one by one when the tokens they inserted repeat. |
| `op ctx.allows(c Ctx, e Entry) bool` | reads growth, memo | `0` | Reports whether the call with the context state c may reuse the memo entry e: always outside a context, and in one only when the leaders growing there are among those that grew where e was stored (its present set). |
| `op ctx.active(c Ctx) bool` | none | `0` | Reports whether the call with the context state c is in a context. |
| `op ctx.lookup(c Ctx, r Rule, min int, key Values) CtxResult noreturn` | reads growth; writes growth | `0 + 1*table + 1*deps` | Returns the result kept in the context of the call for the rule at the position, the level and the key that holds there: the seeds it read have the same identities, and the lookaheads it observed the same outcomes, in the current context; it records those dependencies for the enclosing call. Otherwise the null result. Requires: c is in a context. |
| `op ctx.push(c Ctx)` | writes growth | `0` | Starts recording the dependencies of the evaluation of the call, if it is in a context. |
| `op ctx.pop(c Ctx) int` | reads growth; writes growth | `0 + 1*deps` | Ends recording the dependencies of the evaluation and returns where its result goes: ctx.storeMemo for the memo (it read no seed and made no observation), ctx.storeCtx for the table of the context, or another value for nowhere (its context's leaders have no present set). |
| `op ctx.storeMemo() int` | none | `0` | Returns the value of ctx.pop for a result stored in the memo. |
| `op ctx.storeCtx() int` | none | `0` | Returns the value of ctx.pop for a result kept in the table of its context. |
| `op ctx.keep(c Ctx, r Rule, pos int, min int, key Values, ok bool, end int, silent bool, value Value, errs Errs, far int, k ExpKept) noreturn` | reads growth, call; writes growth | `0 + 1*table + 1*deps` | Keeps the result of the call in the table of its context with the dependencies that ctx.pop ended, and the examined frontier of a Document parse. Requires: The last ctx.pop of c returned ctx.storeCtx. |
| `op ctx.done(c Ctx) noreturn` | writes growth | `0 + 1*deps` | Passes the dependencies that ctx.pop ended to the evaluation that encloses the call. |
| `transitional op grow.begin(r Rule, min int) Grow noreturn` | reads growth, rules; writes growth | `1` | Starts the growth of the leader at the binding level min at the position from a failure, charging one unit: the growth is in a context when other leaders of its cycle grow at the position. |
| `transitional op grow.next(g Grow) bool noreturn` | reads growth; writes growth, pos, silent | `0 + 1*steps + 1*table + 1*deps + 1*merge + 1*run` | Reports whether the body must be evaluated for the next step of the growth g. In a context it first takes the steps that the memoized steps and jumps of the context give, and starts recording what the step it reports depends on. |
| `transitional op grow.step(g Grow, value Value, ok bool) noreturn` | reads growth; writes growth, recovered, pos, silent | `66 + 1*merge + 1*segs + 2*deps + 1*table + 1*run` | Receives the result of a step of the growth g: adopts it as the seed, charging one unit, and returns to the start of the growth if the match grew, dropping the entries after the mark that the growth took when it began, and otherwise ends the growth. A step that succeeded holds the errors it recovered as the list that rec.since returns for that mark; a step that failed holds none. |
| `transitional op grow.end(g Grow) CtxResult noreturn` | reads growth; writes growth, recovered, pos, silent | `0 + 1*run + 1*errs` | Ends the growth g and returns its last seed: the position moves to its end, and its recovered errors replace the entries recorded since the growth began. Outside the missing-token mode they are recorded as rec.add does, at no cost; in that mode as missing.replay does. |

## Call Bookkeeping

The operations of the component `call`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `transitional op call.tracked() bool [document]` | reads call | `0` | Reports whether a rule call takes the target's tracked path: in a Document parse, and in the node runtime also in an observed parse. |
| `transitional op call.track(r Rule, min int) (Value, bool) [document] noreturn` | writes call, pos, silent | `0` | Calls the rule at the binding level min on the target's tracked path and returns its result. |
| `transitional op call.trackBody(r Rule, inv Body) (Value, bool) [document] noreturn` | writes call, pos, silent | `0` | Calls the rule through the invocation inv on the target's tracked path and returns its result. |
| `transitional op call.first(r Rule) bool noreturn` | reads call; writes call | `66 + 1*page_index + 1*pages + 1*seen_index` | Records a call of the rule at the position and reports whether its memoization is deferred: seen.first for a rule whose memoization the target defers, false for one it memoizes eagerly (in a stream, or for a rule without a seen index). |
| `op doc.examinedBegin() int [document]` | reads call, pos; writes call | `0` | Starts measuring how far an observed call examines the input: in a Document parse, the frontier of the call starts at the position. Returns the enclosing call's frontier, for doc.examinedEnd. The call itself, and any memo entry it reads or records, extends the frontier as in an unobserved parse, so that observation does not change reuse. Outside a Document parse it starts nothing and returns 0. |
| `op doc.examinedEnd(enclosing int) (int, bool) [document]` | reads call; writes call | `0` | Ends measuring the frontier of an observed call and merges it into the enclosing one; returns the call's frontier and true in a Document parse, and false otherwise. Requires: enclosing is the result of the matching doc.examinedBegin. |
| `op doc.enter(pos int) (int, int) [document]` | reads call; writes call | `0` | Begins a rule call at pos in a Document parse: returns the examined input of the enclosing call, its end and its start, and starts the call's examined input at pos: as [pos, pos+1), or, in a target whose matches extend the examined input to the end of each unit they read and whose calls do not extend it past their end, as the empty range at pos. Requires: The parse reads a Document. |
| `op doc.leave(high int, low int) [document]` | reads call; writes call | `0` | Ends a rule call in a Document parse: the examined input of the enclosing call becomes the smallest range that holds [low, high) and the call's. Requires: The parse reads a Document; high and low are the results of the matching doc.enter. |
| `op doc.active() bool [document]` | reads call | `0` | Reports whether the parse reads a Document. |
| `op doc.touch(end int) [document]` | reads call; writes call | `0` | Extends the examined frontier of the current call to end, if end is past it. A parse that reads no Document keeps none. |
| `op doc.touchFrom(from int) [document]` | reads call; writes call | `0` | Extends the examined input of the current call back to from, if from is before its start. A parse that reads no Document keeps none. |
| `op doc.low() int [document]` | reads call | `0` | Returns the start of the input that the current call examined so far. Requires: The parse reads a Document. |
| `op doc.high() int [document]` | reads call | `0` | Returns the examined frontier of the current call: the end of the input it examined so far, exclusive. Requires: The parse reads a Document. |
| `op doc.reused() [document]` | writes call | `0` | Counts a reused memo entry in the statistics of the Document. Requires: The parse reads a Document. |
| `op doc.evaluated() [document]` | writes call | `0` | Counts an evaluated rule body in the statistics of the Document. Requires: The parse reads a Document. |
| `transitional op call.body(r Rule, min int) (Value, bool) noreturn` | writes call, pos, silent | `0` | Calls the rule at the binding level min without observation: the call, or the plain invocation, that the target's observed function wraps. |

## Observation

The operations of the component `observe`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op obs.memoHit() [observe]` | writes observe | `0` | Counts a reuse of a memo entry in the observation, if the parse is observed. |
| `op obs.active() bool [observe]` | reads observe | `0` | Reports whether the parse is observed. |
| `op obs.enter(r Rule, pos int, min int) [observe] noreturn` | writes observe | `0` | Reports to the observation of the parse that a call of the rule at pos and binding level min begins. Requires: The parse is observed. |
| `op obs.exit(ok bool, pos int, examined int, known bool) [observe] noreturn` | writes observe | `0` | Reports to the observation of the parse that the call that the last unmatched obs.enter began ends, with its success ok, at pos, and with the end of the input it examined if known. Requires: The parse is observed, and a call that obs.enter began has not ended. |

## Rules

The operations of the component `rules`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op rule.memo(r Rule) bool` | none | `0` | Reports whether the rule is memoized. A parse in the missing-token mode memoizes no rule that can insert a missing token. |
| `op rule.leader(r Rule) bool` | none | `0` | Reports whether the rule leads a left-recursive cycle. |
| `op rule.plain(r Rule) bool` | none | `0` | Reports whether the rule has no captures, so that its plain body serves its invocation. |
| `op rule.docMemo(r Rule) bool` | none | `0` | Reports whether a Document parse memoizes the rule. |
| `op rule.docOnlyMemo(r Rule) bool` | none | `0` | Reports whether a Document parse memoizes the rule and a parse from scratch does not; never for a rule that can insert a missing token in a parse in the missing-token mode. |
| `op rule.docPlain(r Rule) bool` | none | `0` | Reports whether a parse from scratch calls the plain body of the rule directly, at a call site, without the call protocol. |
| `op rule.positional(r Rule) bool` | none | `0` | Reports whether the results of the rule may depend on positions, not only on the input they examine. |
| `op rule.id(r Rule) int` | none | `0` | Returns the rule's index, the rule of its memo keys. |
| `op rule.invoke(r Rule, min int) (Value, bool) noreturn` | writes rules, pos, silent | `1 + 1*captures + 1*capture_fields + 1*body` | Runs the rule's body at the binding level min in a frame of its own and makes the rule's value; on failure the position is unspecified. The evaluation counts in the statistics of a Document parse, and that of a rule on a cycle of calls takes a nesting level while it runs. The unit of the call is charged before the nesting level is taken, so that a call that both the budget and the nesting limit stop stops on the budget. |
| `op rule.invokePlain(r Rule, min int) (Value, bool) noreturn` | writes rules, pos, silent | `1 + 1*body + 1*tokens` | Runs the plain body of a rule without captures and returns its result, at the start position and without the errors it recovered on failure; in the missing-token mode a failure also removes the missing tokens that the body inserted, as missing.reset does. The evaluation counts, and the unit of the call comes before the nesting level, as in rule.invoke. Requires: rule.plain is true of r. |
| `transitional op rule.run(inv Body) (Value, bool) noreturn` | writes rules, pos, silent | `0 + 1*body` | Runs the invocation inv of a rule at binding level 0 and returns its result; on failure the position is unspecified. The evaluation does not count in the statistics of a Document parse, which the caller counts with doc.evaluated. |
| `op opr.led(r Rule) Operators` | none | `0` | Returns the infix and postfix operators of the Operators Rule r, in declaration order. |
| `op opr.prefix(r Rule) Operators` | none | `0` | Returns the prefix operators of the Operators Rule r, in declaration order. |
| `op opr.count(l Operators) int` | none | `0` | Returns the number of operators of l. |
| `op opr.at(l Operators, i int) Operator` | none | `0` | Returns operator i of l. Requires: 0 <= i < opr.count(l). |
| `op opr.operands(r Rule) int` | none | `0` | Returns the number of operand lines of the Operators Rule r. |
| `op opr.operand(r Rule, i int) Line` | none | `0` | Returns operand line i of the Operators Rule r, in declaration order. Requires: 0 <= i < opr.operands(r). |
| `op opr.line(op Operator) Line` | none | `0` | Returns the line of the operator op. |
| `op opr.level(op Operator) int` | none | `0` | Returns the binding power of the operator op: the higher, the tighter it binds. |
| `op opr.infix(op Operator) bool` | none | `0` | Reports whether the operator op is an infix operator. |
| `op opr.postfix(op Operator) bool` | none | `0` | Reports whether the operator op is a postfix operator. |
| `op opr.right(op Operator) bool` | none | `0` | Reports whether the operator op associates to the right. |
| `op opr.nonAssoc(op Operator) bool` | none | `0` | Reports whether the operator op does not associate. |
| `op opr.scope(l Line) int` | none | `0` | Returns the number of capture slots of the line l. |
| `op opr.hasSkip(r Rule) bool` | none | `0` | Reports whether the Operators Rule r skips trivia between its operands and operators. |
| `transitional op opr.skip(r Rule) bool noreturn` | writes rules, pos, silent, cut | `0` | Runs the trivia expression of the Operators Rule r and reports whether it matched; on failure the position is unspecified. Requires: opr.hasSkip(r). |
| `transitional op opr.match(l Line) (Value, bool) noreturn` | writes rules, pos, silent, cut | `0` | Runs the expression of the line l in the current frame and returns its value and success; on failure the position is unspecified. A cut in the expression sets the cut register. |

## Host

The operations of the component `host`. The operations that write the range of a node and the values that an action made are specified with the
values in [Operations That Write Host State](values.md#operations-that-write-host-state).

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `transitional op term.eval(t Term) (Datum, bool) noreturn` | reads host; writes host | `0 + 1*term` | Evaluates t over the current capture frame with an empty span at the position, and returns its value and true. A term that fails, a list element or a conversion it cannot make, returns false and may leave the values it gathered and created; term.dropKids and term.dropCreated release them. An interruption ends the parse and leaves them too, for the next parse starts without them. |
| `transitional op term.created() int` | reads host | `0` | Returns the mark of the nodes that terms and actions created, which term.dropCreated takes. |
| `transitional op term.dropCreated(mark int) noreturn` | writes host | `0 + 1*dropped` | Forgets the nodes created after the mark, without changing them, and charges one unit for each. Requires: mark came from term.created, and nothing dropped it since. |
| `transitional op term.kids() int` | reads host | `0` | Returns the mark of the values that list built-ins gather, which term.dropKids takes. |
| `transitional op term.dropKids(mark int) noreturn` | writes host | `0 + 1*dropped` | Drops the values gathered after the mark and charges one unit for each. Requires: mark came from term.kids, and nothing dropped it since. |
| `transitional op term.isFalse(v Datum) bool` | none | `0` | Reports whether v is the boolean false. Any other value, nil among them, is not. |
| `transitional op term.isFunction(v Datum) bool` | none | `0` | Reports whether v is a function, which no variable may hold. |
| `transitional op opr.lineValue(r Rule, l Line, f Frame, v Value, start int) Value noreturn` | reads frames; writes host | `0` | Returns the value of the operand line l of the rule r that matched from start to the position with the value v and the captures of f: the line's action, or its shaped value with its captures. |
| `transitional op opr.build(r Rule, a Attempt, lhs Value, rhs Value) Value noreturn` | reads operators, frames; writes host | `0` | Returns the value of the application of the operator of a to lhs (null for a prefix operator) and rhs (null for a postfix operator), which ends at the position: the action of the operator's line, or the node of the tuple (lhs, operator, rhs) with the line's captures. |
| `transitional op opr.buildTyped(r Rule, a Attempt, lhs Value, lstart int, rhs Value) (Value, int) [typed] noreturn` | reads operators, frames; writes host | `0` | Is opr.build for typed values, where lhs, if it is not a node, starts at lstart; it also returns where the value of the application starts. |

## Capture Frames

The operations of the component `frames`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op frame.mark() Mark` | reads frames, env, recovered | `0` | Returns a backtracking point at the position. |
| `op frame.markPos(m Mark) int` | none | `0` | Returns the position of the backtracking point m. |
| `op frame.markRec(m Mark) int` | none | `0` | Returns the number of the errors recovered before the backtracking point m, as rec.mark does. |
| `op frame.markEnv(m Mark) Scope` | none | `0` | Returns the variable environment of the backtracking point m. |
| `op frame.markMissing(m Mark) int [missing]` | none | `0` | Returns the mark of the missing-token record at the backtracking point m, as missing.mark does. |
| `op frame.markTrail(m Mark) int` | none | `0` | Returns the length of the trail at the backtracking point m. |
| `op frame.undo(trail int) noreturn` | reads frames; writes frames | `0 + 1*trail` | Undoes the writes of the trail after its first trail writes, in reverse order, charging one unit for each, and drops them: every slot they wrote has the value it had before. Requires: trail is at most the length of the trail. |
| `op frame.current() Frame` | reads frames | `0` | Returns the frame that captures write to. |
| `op frame.use(f Frame)` | writes frames | `0` | Makes f the frame that captures write to. |
| `op frame.open(n int) Frame noreturn` | writes frames | `0 + 1*slots` | Returns a new frame of n empty slots, which lives until the parse ends. A frame of no slots is never written, so it may be shared. |
| `op frame.set(slot int, v Value)` | reads frames; writes frames | `0` | Writes v to the slot of the current frame and adds the slot's old value to the trail. The write is free: its size depends on nothing, the match that made v paid for itself, and frame.undo charges for each write it undoes. Requires: slot is a slot of the current frame. |
| `op frame.close(f Frame)` | writes frames | `0` | Ends f, which nothing refers to any more: a rule's frame when the rule returns. A target may reuse it, and one without a pool of frames does nothing. Requires: No trail entry and no current frame refer to f. |
| `op frame.slots(f Frame) int` | none | `0` | Returns the number of slots of f. |
| `op frame.save(f Frame) noreturn` | reads frames; writes frames | `0 + 1*slots` | Copies the slots of f aside, charging the copy as the target charges copies of values. |
| `op frame.restore(f Frame) noreturn` | reads frames; writes frames | `0 + 1*slots` | Copies the slots that the last frame.save set aside back into f, charging the copy as frame.save does. Requires: The last frame.save copied the slots of f, and nothing parsed since. |

## Operators Rule Loop

The operations of the component `operators`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op opr.attempt(op Operator, f Frame, v Value, start int, cut bool, errs Errs) Attempt` | reads env, pos | `0` | Returns the match of op from start to the position with the value v, the captures of f, the variable environment, the cut flag cut and the recovered errors errs. |
| `transitional op opr.attemptMissing(op Operator, f Frame, v Value, start int, cut bool, errs Errs, mark int) Attempt [missing] noreturn` | reads env, pos, recovered | `0` | Is opr.attempt in a parse that can insert missing tokens: the match also holds a copy of the tokens inserted after mark. |
| `op opr.found(a Attempt) bool` | none | `0` | Reports whether a is a match, not the null attempt. |
| `op opr.op(a Attempt) Operator` | none | `0` | Returns the operator of the match a. Requires: opr.found(a). |
| `op opr.end(a Attempt) int` | none | `0` | Returns where the match a ended. Requires: opr.found(a). |
| `op opr.cut(a Attempt) bool` | none | `0` | Reports whether the match a cut. Requires: opr.found(a). |
| `op opr.env(a Attempt) Scope` | none | `0` | Returns the variable environment after the match a. Requires: opr.found(a). |
| `op opr.errs(a Attempt) Errs` | none | `0` | Returns the errors that the match a recovered. Requires: opr.found(a). |
| `op opr.setCutFailed(failed bool)` | writes operators | `0` | Records whether a candidate of the longest match failed after a cut. |
| `op opr.cutFailed() bool` | reads operators | `0` | Reports what the last opr.setCutFailed recorded. |
| `op opr.setNudBound(bound int)` | writes operators | `0` | Records the binding level above which no operator can follow the start of an expression. |
| `op opr.nudBound() int` | reads operators | `0` | Returns what the last opr.setNudBound recorded. |
| `op opr.setAfterEmpty(empty bool)` | writes operators | `0` | Records whether the right operand that the next expression parses follows an infix operator applied without consuming input. |
| `op opr.afterEmpty() bool` | reads operators | `0` | Returns what the last opr.setAfterEmpty recorded. |

## Suspended Calls

The operations of the component `suspended`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op call.state() CallState` | writes suspended | `0` | Returns a record for the state of a call that the caller holds until the call ends. |
| `op call.save(st CallState, start int, min int, key Values, c Ctx, rec int, m ExpMark)` | writes suspended | `0` | Saves in st the state that the end of the call needs: its position, its binding level, the values of its key, its context state, the mark of its recovered errors and the record that its isolation set aside. |
| `op call.saveExamined(st CallState, high int, low int) [document]` | writes suspended | `0` | Saves in st the examined input of the enclosing call, which doc.enter returned. |
| `op call.saveMissing(st CallState, mark int) [missing]` | writes suspended | `0` | Saves in st the mark of the missing-token record, which missing.mark returned. |
| `op call.start(st CallState) int` | reads suspended | `0` | Returns the position of the call that st holds. Requires: st holds a call that call.save saved. |
| `op call.min(st CallState) int` | reads suspended | `0` | Returns the binding level of the call that st holds. Requires: st holds a call that call.save saved. |
| `op call.key(st CallState) Values` | reads suspended | `0` | Returns the values of the key of the call that st holds. Requires: st holds a call that call.save saved. |
| `op call.ctx(st CallState) Ctx` | reads suspended | `0` | Returns the context state of the call that st holds. Requires: st holds a call that call.save saved. |
| `op call.rec(st CallState) int` | reads suspended | `0` | Returns the mark of the recovered errors of the call that st holds. Requires: st holds a call that call.save saved. |
| `op call.mark(st CallState) ExpMark` | reads suspended | `0` | Returns the record that the isolation of the call set aside that st holds. Requires: st holds a call that call.save saved. |
| `op call.high(st CallState) int [document]` | reads suspended | `0` | Returns the end of the examined input of the enclosing call that st holds. Requires: st holds a call that call.save saved. |
| `op call.low(st CallState) int [document]` | reads suspended | `0` | Returns the start of the examined input of the enclosing call that st holds. Requires: st holds a call that call.save saved. |
| `op call.missing(st CallState) int [missing]` | reads suspended | `0` | Returns the mark of the missing-token record that st holds. Requires: st holds a call that call.save saved. |
| `op oprLongest.l(st LongestState) Operators` | reads suspended | `0` | Returns the field l of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprLongest.setL(st LongestState, x Operators)` | writes suspended | `0` | Sets the field l of the record st to x. |
| `op oprLongest.i(st LongestState) int` | reads suspended | `0` | Returns the field i of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprLongest.setI(st LongestState, x int)` | writes suspended | `0` | Sets the field i of the record st to x. |
| `op oprLongest.m0(st LongestState) Mark` | reads suspended | `0` | Returns the field m0 of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprLongest.setM0(st LongestState, x Mark)` | writes suspended | `0` | Sets the field m0 of the record st to x. |
| `op oprLongest.best(st LongestState) Attempt` | reads suspended | `0` | Returns the field best of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprLongest.setBest(st LongestState, x Attempt)` | writes suspended | `0` | Sets the field best of the record st to x. |
| `op oprLongest.cutFailed(st LongestState) bool` | reads suspended | `0` | Returns the field cutFailed of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprLongest.setCutFailed(st LongestState, x bool)` | writes suspended | `0` | Sets the field cutFailed of the record st to x. |
| `op oprLongest.prevFrame(st LongestState) Frame` | reads suspended | `0` | Returns the field prevFrame of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprLongest.setPrevFrame(st LongestState, x Frame)` | writes suspended | `0` | Sets the field prevFrame of the record st to x. |
| `op oprLongest.prevCut(st LongestState) bool` | reads suspended | `0` | Returns the field prevCut of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprLongest.setPrevCut(st LongestState, x bool)` | writes suspended | `0` | Sets the field prevCut of the record st to x. |
| `op oprLongest.f(st LongestState) Frame` | reads suspended | `0` | Returns the field f of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprLongest.setF(st LongestState, x Frame)` | writes suspended | `0` | Sets the field f of the record st to x. |
| `op oprNud.m0(st NudState) Mark` | reads suspended | `0` | Returns the field m0 of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprNud.setM0(st NudState, x Mark)` | writes suspended | `0` | Sets the field m0 of the record st to x. |
| `op oprNud.a(st NudState) Attempt` | reads suspended | `0` | Returns the field a of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprNud.setA(st NudState, x Attempt)` | writes suspended | `0` | Sets the field a of the record st to x. |
| `op oprNud.i(st NudState) int` | reads suspended | `0` | Returns the field i of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprNud.setI(st NudState, x int)` | writes suspended | `0` | Sets the field i of the record st to x. |
| `op oprNud.prevFrame(st NudState) Frame` | reads suspended | `0` | Returns the field prevFrame of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprNud.setPrevFrame(st NudState, x Frame)` | writes suspended | `0` | Sets the field prevFrame of the record st to x. |
| `op oprNud.prevCut(st NudState) bool` | reads suspended | `0` | Returns the field prevCut of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprNud.setPrevCut(st NudState, x bool)` | writes suspended | `0` | Sets the field prevCut of the record st to x. |
| `op oprNud.f(st NudState) Frame` | reads suspended | `0` | Returns the field f of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprNud.setF(st NudState, x Frame)` | writes suspended | `0` | Sets the field f of the record st to x. |
| `op oprExpr.min(st ExprState) int` | reads suspended | `0` | Returns the field min of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprExpr.setMin(st ExprState, x int)` | writes suspended | `0` | Sets the field min of the record st to x. |
| `op oprExpr.lhs(st ExprState) Value` | reads suspended | `0` | Returns the field lhs of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprExpr.setLhs(st ExprState, x Value)` | writes suspended | `0` | Sets the field lhs of the record st to x. |
| `op oprExpr.lastNone(st ExprState) int` | reads suspended | `0` | Returns the field lastNone of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprExpr.setLastNone(st ExprState, x int)` | writes suspended | `0` | Sets the field lastNone of the record st to x. |
| `op oprExpr.bound(st ExprState) int` | reads suspended | `0` | Returns the field bound of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprExpr.setBound(st ExprState, x int)` | writes suspended | `0` | Sets the field bound of the record st to x. |
| `op oprExpr.guard(st ExprState) int` | reads suspended | `0` | Returns the field guard of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprExpr.setGuard(st ExprState, x int)` | writes suspended | `0` | Sets the field guard of the record st to x. |
| `op oprExpr.m0(st ExprState) Mark` | reads suspended | `0` | Returns the field m0 of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprExpr.setM0(st ExprState, x Mark)` | writes suspended | `0` | Sets the field m0 of the record st to x. |
| `op oprExpr.a(st ExprState) Attempt` | reads suspended | `0` | Returns the field a of the record st: what the last set of it stored, and before the first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame, value or match). |
| `op oprExpr.setA(st ExprState, x Attempt)` | writes suspended | `0` | Sets the field a of the record st to x. |

## Stream Window

The operations of the component `window`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `host op stream.emits() bool [stream]` | reads window | `0` | Reports whether the parse delivers the elements of a #stream repetition to a callback. |
| `host op stream.start() int [stream]` | reads window | `0` | Returns the id of the start rule of the stream parse. Requires: The parse delivers elements (stream.emits). |
| `host op stream.report() [stream] noreturn` | reads window; writes window | `0` | Delivers the recovered errors committed since the last report to the diagnostic callback and the retained tail. An error of the callback ends the parse with it, before the element is delivered. |
| `host op stream.deliver(v Value) [stream] noreturn` | reads window; writes window | `0` | Passes the element v, nil for a value-free repetition, to the callback; v is final from then on. An error of the callback ends the parse unchanged. Requires: The parse delivers elements. |
| `host op stream.discard(keep int) [stream] noreturn` | reads window; writes window | `0 + 1*stream_scan` | Discards the input before keep, when at least half of the buffer can go; later reads before keep are not supported. The line and column of the new base are kept. |
| `host op stream.release() [stream] noreturn` | reads window; writes window | `0 + 2*stream_frames` | Releases what the elements committed so far held: the trail, the capture frames and the node chunks that no active call refers to, so that a delivered element is not kept alive through the parser. The effect is memory held, which the parse result does not show. Requires: Every call of the delivered elements has returned. |
| `host op stream.dropRecords(pos int) [stream] noreturn` | reads window; writes window | `0 + 1*stream_inactive` | Discards what the parse keeps about the positions before pos besides the memo, which no later call reads: the records of skips and of the profile, and what the inactive capacity of the trail and of a target's value stack refers to. Later calls never read them, so the parse result does not show the difference. |
| `host op stream.tail() bool [stream]` | reads window | `0` | Reports whether the parse retains the tail of its recovered errors (a diagnostic limit is set). |

## Repetition Runs

The operations of the component `runs`.

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `host op run.start(r Run, site RunSite) [document]` | reads runs; writes runs | `0` | Starts the run r of the repetition at the position. It takes over the record that this parse left there, or the record of the previous parse here or where the one edit since moved it; with no such record it has nothing to reuse. |
| `host op run.prefix(r Run) RunElem [document] noreturn` | reads runs; writes runs | `1 + 1*merge` | Reuses the next old element if it examined only input before the one edit since, replaying what running it did: the examined input, the expectations, the position, the reuse statistics. Returns it, or none when the prefix of the run ends. |
| `host op run.resync(r Run) bool [document] noreturn` | reads runs; writes runs | `0 + 1*run_search + 1*run_scan` | Reports whether an old element after the edit starts where the next element would, and every element from it on examined only input after the edit; run.tail then reuses them. |
| `host op run.tail(r Run) RunElem [document] noreturn` | reads runs; writes runs | `1 + 1*merge + 2*moved + 1*move_edits` | Reuses the next old element after the edit, moved by the edit with its value (doc.move), replaying what running it did as run.prefix does. Returns it, or none at the end of the run or its bound. |
| `host op run.begin(r Run) [document]` | reads runs; writes runs | `0` | Starts evaluating an element: its examined input and its expectations are recorded apart from the enclosing call's. |
| `host op run.end(r Run, ok bool, v Value) [document] noreturn` | reads runs; writes runs | `0 + 1*merge + 1*run_growth` | Ends evaluating an element, adding its examined input and its expectations to those of the enclosing call, and records it if it matched with the value v. |
| `host op run.finish(r Run) [document] noreturn` | reads runs; writes runs | `0 + 1*run_elems` | Records the run of a repetition that matched for the next parse, unless it is too short, or recovered errors, or read the seed of a growth. |
