# Incremental Documents

A `Document` holds a text that changes over time, applies edits to it and parses
the current text. Reparsing after an edit reuses results of the previous parse
that the edit did not affect. Reuse never changes the outcome: a parse of a
document is the same as a parse of the current text from scratch. This chapter
defines the document API, its versions and snapshots, change notifications and
reuse-history controls.

## Scope and Terminology

A **document** is a `Document`, created by `Parser.NewDocument`. The **text** is
the current content of the document. A **version** is the opaque token returned
by `Document.Version`. A **snapshot** is an immutable capture of one revision's
text, parse outcome and value graph. A **batch** is a set of replacements that
apply simultaneously to one base revision.

Positions and edit ranges use the document's position unit, which is fixed when
the document is created; see [Positions](parsing.md#positions).

## Contract

### Construction and Ownership

| Operation | Behavior |
|:--|:--|
| `p.NewDocument(text, opts...)` | Returns a document holding `text`. `WithUnit` fixes the position unit (also the unit of edits), `WithBackend` fixes the backend and `WithExpectations` fixes whether syntax errors [record expectations](parsing.md#recording-expectations) (a later `Parse` or `Snapshot` that asks for the other mode is an error); `WithContext` and `WithLimits` set defaults for initialization and later parses. It fails for `RecognizeOnly`, for an unavailable backend, for an unknown start rule and when initialization is interrupted. Nothing is parsed |
| `doc.Text()` | The current text |
| `doc.Unit()` | The fixed position unit |
| `doc.Version()` | A comparable opaque token identifying the current text revision. A new document starts at sequence 1. Parsing does not advance the version. The token does not retain the document, the text or parse caches |

A `Document` has one owner and is not safe for concurrent use; a `Parser` is safe
for concurrent use. A document is always parsed as a whole: `#stream` has no
effect on it and it is not a stream parse. Grammars that use `#missing` are
rejected.

For `Bytes`, construction can charge no budget, because a large immutable
input need not be scanned or copied. For `CodePoints`, initialization decodes and
normalizes the input, and the inspected input is charged.

### Edits

`doc.Edit(start, end, text)` replaces `[start, end)` with `text`, in the
document's unit. `start == end` inserts and an empty `text` deletes. It returns
an error if the range is outside the text and, in `Bytes`, if `start` or `end`
lies inside a character. It does not parse. A successful `Edit` advances the
version once, including an edit that changes nothing. A failed edit changes
nothing.

`doc.EditBatch(base, edits...)` applies replacements simultaneously to the text
identified by the token `base`.

- Every range uses coordinates in the original base text and the document's unit.
- Starts MUST be strictly increasing and ranges MUST NOT overlap. A repeated
  start, including several insertions at one position, is rejected.
- A token from another document or from an older revision is rejected, even for
  an empty batch.
- An empty valid batch changes neither text nor version. A successful nonempty
  batch advances the version once and does not parse. Version sequence overflow
  is rejected before any change.
- The complete batch is validated and prepared before it is published. A
  rejected batch leaves the text, version, statistics, reusable state and
  previously returned nodes unchanged.
- In `Bytes`, endpoints are checked against character boundaries of the base
  text. Replacements can create or remove UTF-8 sequences at their seams; the
  next parse decodes them. In `CodePoints`, invalid UTF-8 in replacement text is
  normalized, one replacement character per invalid byte.
- After a successful batch the next parse equals a fresh parse of the resulting
  text. Committing a batch clears reusable parse caches; it does not promise the
  reuse that the same replacements applied as separate `Edit` calls could give.

In `Bytes`, only the interior of a valid UTF-8 character is excluded as an edit
boundary. Each invalid byte is a separate replacement character, so text can be
replaced at it, or inserted before or after it. An edit can join an incomplete
prefix and continuation bytes into a valid character; the next parse
invalidates results that depended on the earlier decoding, including a prefix
completed by an insertion at the end of the text. In `CodePoints`, invalid bytes
have already been replaced with U+FFFD, and editing the normalized text does not
reassemble the original bytes.

`Edit` and `EditBatch` are outside parse compute limits.

### Parsing

`doc.Parse(opts...)` parses the current text using the construction defaults
overridden by the options of the call; `WithLimits` overrides only the nonzero
fields it sets ([Default Limits](parsing.md#default-limits)). Context, limits and
tracing MAY vary per call; the unit, backend, start rule and non-recognition mode
MUST NOT, and an option that tries to change them is an error. Counters reset on
each call. Results and errors are those of `Parser.Parse`, including recovered
`SyntaxErrors`.

`Parse` publishes recovered parser-produced syntax diagnostics as caller-owned
values: it copies each `SyntaxError` and its `Expected` and `Messages` slices, so
a caller's mutation cannot change memoized errors or a later parse. Ordinary parse
failures are newly constructed; caller-supplied errors such as `Context.Err` keep
their identity. This does not detach the mutable `Node` graph, which MAY share
nodes with the document, and nodes that an earlier parse returned MAY be moved in
place by a later one. A later parse moves only the nodes it reuses, so even when it
completes a tree from an earlier parse can be left partly moved, and when it is
interrupted while it moves them no later parse moves them again. The positions in a
tree from an earlier parse are unspecified once a parse after an edit has started; to
keep a tree as it was, take a `Clone` before that parse.
Typed values returned by a generated parser's
`Document.ParseAST` are read-only and shared with the document and earlier results;
its generated `CloneAST` returns an independent copy (see
[Generated Parsers](generated-parsers.md)). Parsing and diagnostic publication use
the same invocation controls. If publication is interrupted, no result is returned and reusable
execution state is cleared for retry.

Under a finite [memory limit](parsing.md#memory-limit), the count of a `Parse`
starts at the bytes the document retains from earlier parses: its memo table,
its repetition records and edits, and the stacks it keeps. If they alone exceed
`Memory`, the document drops them before parsing, as `ResetHistory` does, and the
parse runs from scratch. Retained bytes within `Memory` are kept, so a parse
that reuses them can exceed the limit where a parse from scratch would not; that
interruption clears them, and the next parse starts without them. The retained
bytes are counted as the memory limit counts storage, not as `EstimatedBytes`
estimates it. A generated document counts its memo table, and a typed generated
document an estimate of its retained memo slots and entries.

### Statistics

| Operation | Behavior |
|:--|:--|
| `doc.Stats()` | `ParseStats` of the last parse. `Evaluated` counts rule bodies run, `MemoHits` and `ResumedElements` count the two kinds of reuse, and `Reused` is their sum |
| `doc.CacheStats()` | Point-in-time counts of the retained reuse cache, read by the owner of the document |
| `doc.ResetHistory()` | Drops memo, repetition and reusable parser history and keeps the text, the version, the construction options and the last statistics. The next parse starts fresh. No threshold applies; the caller chooses when to reset. Nodes returned earlier stop moving through reuse; movement from parses before the reset is not undone |

`CacheStats` has the fields `LiveMemoEntries` (memo entries linked for reuse),
`RetiredMemoEntries` (retired entries retained for reuse), `RepetitionElements`
(elements in resumable repetition runs) and `EstimatedBytes` (approximate
cache-owned backing storage). All four are zero immediately after
`ResetHistory`. `EstimatedBytes` excludes the text, caller-held result trees,
shared strings, shared expectation, error and environment payloads, map buckets
and allocator overhead; it is neither process heap use nor a limit.

### Snapshots

`doc.Snapshot(opts...)` captures the current text revision, parse outcome and
value graph in one immutable `Snapshot`. It resolves invocation overrides once
over the construction defaults, with the same variable and fixed options as
`Parse`. Parsing and freezing use the same cancellation context and the same
cumulative spend counter. Copying the text, freezing
nodes and fields and copying diagnostics are part of that invocation.

A normally returned parse outcome produces a snapshot, including an ordinary
syntax error and a recovered error. For an ordinary syntax failure the snapshot
keeps the text and version, has a zero root as `Document.Parse` does, and exposes
a defensive copy of the `SyntaxError`. A recovered outcome keeps its root and
`SyntaxErrors`. A parse that nests deeper than the depth limit is a normally returned
outcome too: the snapshot keeps a copy of its `LimitError` of kind `LimitDepth`.
Cancellation or an exhausted compute or memory limit during parsing or
freezing returns a nil snapshot and a fatal interruption error, which grammar
choice, predicates and recovery cannot absorb. No partial graph, text or
diagnostics are published. If execution began, reusable execution state is
cleared before retry; completed input tables MAY remain and partial tables are
discarded. An error before execution keeps existing caches and statistics.
Cleanup is not charged. A trace callback panic propagates unchanged after
cleanup. Snapshot accessors and `Materialize` run after capture and do not
inherit its context or compute limits.

| Accessor | Meaning |
|:--|:--|
| `snapshot.Root()` | The captured root, a `SnapshotNode` |
| `snapshot.Text()` | The exact text of the revision |
| `snapshot.Version()` | The revision token |
| `snapshot.Error()` | The diagnostic of that revision |
| `snapshot.Materialize()` | A new independent mutable `*Node` graph. Each call creates a separate graph |

`SnapshotNode` is a read-only handle. Its zero value represents a nil node.
`ChildCount` and `Child` address the elements of a tuple or list node, and `ChildCount` includes nil slots. `Field(name)` returns a value and a
presence flag that distinguishes an absent field from a present nil value; a typed
nil `*Node` field becomes a zero handle and an untyped nil stays nil. `Fields()`
returns an owned slice in original order. Handles and snapshots are immutable and
safe for concurrent reads while the document has one editing and parsing owner.

Freezing preserves shared node identities, cycles, nil child slots and nil versus
allocated-empty collections. It accepts field values of dynamic type `nil`, `int`,
`string`, `bool` and `*Node`, including a typed nil `*Node`. Any other dynamic
type returns no snapshot and a `*SnapshotCaptureError` that contains the
deterministic path and the `reflect.Type`, without retaining the unsupported
value; the error also carries a defensive copy of the ordinary parse error, if one
existed, and preserves `errors.Is` and `errors.As` behavior for it. `Materialize`
preserves identities, cycles, nil slots and nil versus allocated-empty
collections.

### Interruption

A cancellation or option error detected before execution touches reusable state
preserves valid caches and the last statistics. Once execution starts, an
interruption clears memo and repetition state before the next attempt, because
entries can depend on unfinished left-recursion work and a result that was being
moved in place can be left partly moved. Fully completed input
tables MAY be retained; incomplete tables are never published. The text and
options stay available for retry or editing. A retry with fresh controls MUST
produce the result of parsing the current text from scratch. Syntax errors and
recovered errors are completed outcomes and keep their reusable caches.

### Reuse

Reuse is a performance property, not a result property. An implementation MAY
retain, shift, drop or recompute any intermediate result, provided that every
parse returns what a parse of the current text from scratch returns. Reuse is
observable only through the statistics. In the engine, a lookup that finds a memo entry
applies the edits since the entry was stored or last found, and counts one unit of Compute
for each edit it applies, against an explicit `Compute` limit only
([Default Limits](parsing.md#default-limits)). An entry that lags behind by more than 128
edits is treated as absent and the call evaluates again, so a lookup applies at most 128 edits and the
[linear cost proof](../proofs/linear-cost.md#setting) bounds their work by that
constant. Generated runtimes apply each edit to every entry at once, and a lookup there applies none.
Each edit costs them time in proportion to the memo, and after more than 128 edits since the last
parse they drop the memo instead, so that further edits cost no more than the edit itself and the next
parse evaluates every rule, as a parse from scratch does. The memo holds at most one entry for each
call of the parse before, so the edits between two parses cost at most 128 times the work of the parse
before, whatever their number, and a sequence of an edit, a parse, an edit and a parse costs per round
what its parse costs plus that bound. The number 128 is where reuse stops paying when the edits are
spread over the whole text; it is one constant shared by every backend. The two forms differ in the statistics alone: the engine
counts the lag of each entry from the parse that stored it or last found it, and a generated parser
counts the edits since the last parse for the whole memo, so either may reuse entries that the other
treats as absent (`Reused` differs), and the results are the same.

A result that read the growing result of a left recursion in progress when it
started depends on that growth, not only on the text. A parse keeps it only
while the growth at its position lasts (see
[Evaluations of Rules](parsing.md#evaluations-of-rules)), so a later parse
evaluates it again, even when the text is unchanged. A result that a later
parse reuses read no such growing result.

### Reuse Model

The engine and the generated runtimes reuse the results of a Document parse in
the following way. Wherever a rule below keeps a result, evaluating again would
give the same result; wherever it does not, the parse evaluates again.
[Reuse](#reuse) states the guarantee that results do not change.

**Slots.** Slot i of a text of length n is the character at position i
when i < n, and the end of the text when i = n. A terminal, a character
class, `.`, an anchor or the decoding of a character **examines** a slot when
its result depends on what the slot holds:

- matching a character examines its slot, and rejecting one examines it too;
- testing for the end of the input or of a line at i examines slot i;
- testing for the beginning of a line (`^`) or of the input (`^^`) at i
  examines slot i − 1, or at i = 0 the absence of one, written slot -1;
- decoding the character at position i in bytes examines every byte that it
  reads, and reads no byte before i.

**Examined range.** The examined range of an evaluation is the interval
[l, h) from the least to one past the greatest slot that it, or a call
within it, examined. For an evaluation at p, l ≤ p. Every failure that
an evaluation records is at a slot that it examined. A Document may keep a
larger interval in place of [l, h), such as the `Examined` frontier of
[Generated Go Document Tracing and Profiling](observability.md#generated-go-document-tracing-and-profiling);
a larger interval only drops more entries and resumes fewer iterations.

**Position-free evaluations.** An evaluation is **position-free** when no
action or predicate in it, or in a call within it, reads a position (the
`.Start` and `.End` members), and its outcome has no recovered errors, whose
lines and columns depend on positions. **Shifting** the outcome of a
position-free evaluation by δ adds δ to its end, to the ranges
of its nodes and to its farthest failure.

**Entries under edits.** At each edit that replaces [a, b) and changes the
length of the text by δ, a memo entry with recorded range [l, h) is:

- kept unchanged if h ≤ a;
- shifted by δ, together with its position and its range, if l ≥ b
  and its evaluation was position-free;
- dropped otherwise.

A backend may drop an entry that these rules would shift: a generated Go
parser drops one whose value holds a host value other than a node or a basic
value, which it cannot move.

Edits are applied to an entry in order, either all at once or when the entry is
next looked up. The engine applies them to an entry when a lookup finds it, and
at once to the entries at the positions a to b, both included; the
generated runtimes apply each edit to every entry at once. An entry found to be
dropped may stay where it is, where no lookup returns it, until a store under
its key replaces it, a later edit whose positions include its position drops
it, a walk of its chain finds that it lags behind by more than 128 edits
([Work of a Document Parse](#work-of-a-document-parse)), or the whole memo is
dropped. A batch is one edit for the text and drops
every entry and run ([Edits](#edits)).

**Results that are stored.** A parse stores entries under the rules of
[Evaluations of Rules](parsing.md#evaluations-of-rules), except that it also
memoizes the rules that the first exception there leaves unmemoized in an
ordinary parse (the transient rules); since a leader is memoized in every
case, no leader is memoized only by a Document. A result that read the
growing result of a left recursion is dropped within the parse and never
reaches a later one. A parse that does not complete, because it is
[interrupted](#interruption) or aborted by a run-time error of an action, the
nesting limit or another fatal error, drops the memo entries and the runs, and
publishes no partial table.

**Runs.** A **run record** of a repetition at p holds, for each iteration
that matched, its start, its end, its examined range, its value, and its
farthest failure with its expected items. A Document may record only runs of
at least a minimum number of iterations.

- *Recording.* A run is recorded for a repetition that matched outside a
  lookahead and a `skip`, whose element has no predicate, calls no rule that
  reads variables, and writes no capture to an enclosing scope. No recovered
  error remains at the end of the run. During the run no call read the
  current result of a growth, directly or through an entry stored by a call
  that did, even of a growth that started and ended within the run, nor
  reused a result that read one; a run during which such a read or reuse
  happened is not recorded.
- *Resuming.* A repetition resumes a record only outside a lookahead and a
  `skip`, in one of two ways. **Whole**: the record of the same repetition at
  the same position, stored earlier in the same parse, or by an earlier parse
  of the same text with no edit since; it reuses every recorded iteration.
  **Across one edit**: the record of the previous parse when exactly one edit
  of [a, b) by δ came since, stored at p when p ≤ a, or at p − δ when
  p ≥ b + δ; at a position between them no record is resumed. It reuses the recorded iterations from the first on
  while their examined range ends at or before a, whatever δ is. It
  then evaluates iterations, possibly none. When the current position is
  s + δ for the start s of a recorded iteration j such that every
  recorded iteration from j on examined only slots at or after b, it reuses
  those iterations shifted by δ, which requires δ = 0 or
  position-free iterations. After the reused iterations it goes on as a
  repetition does.

### Work of a Document Parse

A Document parse of the engine, on each of its backends, does work that a parse
of the same text from scratch does not: it applies pending edits, walks and
indexes the chains of memo entries that earlier parses left, moves the values it
reuses, works through the records of runs, grows the memo table and decodes the
characters of its text. This
section bounds the units of Compute that each of these operations charges, as an
explicit `Compute` limit counts them; the default limit leaves most of them out
([Default Limits](parsing.md#default-limits)). Each bound is an upper bound: an
operation may charge less. The generated runtimes apply each edit to every
entry at once and keep no runs ([Reuse](#reuse)), and are not covered here.

The charges are those of a function of the logical state, as for every parse
([Reservations and Interruptions](parsing.md#reservations-and-interruptions),
"Charges depend on logical state only"): the same Document after the same
operations charges the same units for its next parse.

A **chain** is the sequence of memo entries at one position, live or dropped
([Reuse Model](#reuse-model)), in the order a lookup passes them. An entry
**lags** when it lags behind by more than 128 edits, the entry that a lookup
treats as absent ([Reuse](#reuse)). The **size** of a key is one, plus one for each variable it
holds, plus the length of each string among their values.

**Edits.** An edit charges no Compute. A Document keeps a log of the edits since
its memo was last dropped. The edit that finds 65,536 edits in the log drops the
memo entries and the run records and empties the log instead of adding to it, so
a parse finds at most 65,536 pending edits.

**Lookups.** A lookup charges one unit, and then:

- for each entry of the chain that it passes, one unit, and for an entry of the
  same rule, the comparison of the keys: at most one unit for each variable,
  and for a string compared with a string of the same length, at most twice its
  length; a lookup of a rule that only a Document memoizes compares strings by
  identity, at one unit for each variable;
- for each lagging entry that it meets, one unit, and it removes the entry from
  the chain without comparing it;
- it compares at most 32 entries that do not lag; on a chain that holds more, a
  lookup of a rule that a parse from scratch memoizes too then removes every
  lagging entry of the chain, at one unit each, builds the index of the position
  if it has none, hashes its own key and probes the index, comparing its key with
  at most as many entries as the chain holds. A lookup of a rule that only a
  Document memoizes stops after the 32 entries and misses;
- building an index hashes the key of each entry of the chain, and hashing a key
  charges its size. Only an edit and the removal of a lagging entry drop the
  index of a position, so a parse builds the index of a position at most once;
- when the entry it finds does not account for every edit of the log, it
  applies the pending edits to the entry, one unit for each, at most 128
  ([Reuse](#reuse)), since an entry that lags is never found.

**Stores.** A store charges one unit, walks and indexes the chain as a lookup
does to find the entry it replaces, and charges one more unit to link the new
entry. When the entry it replaces lies beyond the first 32, it walks the chain
up to it, one unit for each entry it passes; when it lies among them and the
position has an index, it hashes the key it replaces. A store of a rule that only a Document memoizes walks at most
32 entries, as its lookup does, and when none of them is the entry it replaces,
it links the new entry in front of the chain and replaces none. A store at a position
beyond the slots of the memo table grows them: when the number of binary digits
of the new length exceeds that of the old one, it charges the old length, and
otherwise nothing.

**Moves.** A lookup that finds an entry whose value an edit shifted
([Reuse Model](#reuse-model)) moves the value, once: the entry then accounts for
every pending edit. A move makes one pass over the value. It charges one unit
for each node it visits and one for each child and field of it. It does not
visit a node that starts where it ends (an **empty** node) in place: it copies it
for the move, with the empty nodes below it, and charges the copy as a visit and
at most one more unit to forget it at the end; a reference to a node that the
move has already copied charges one unit and copies nothing. A node that does not start where it ends is moved in place, and a
move does not visit a node that this parse built or has already moved, so such a
node moves at most once in a parse. Each node of the value accounts for the
edits up to the parse that last moved or built it. For the nodes that account
for the same number of edits, the move replays the edits that they do not
account for once, at one unit for each, and charges at most one unit to forget
them at the end. A move therefore charges at most two units for each node it visits,
counting an empty node once for each reference by which it reaches it, and for
each child and field of those nodes, plus one unit for each edit it replays.

**Runs.** The operations of a repetition that may resume a record
([Reuse Model](#reuse-model)) merge the expected items of an iteration into
those of the farthest failure. A **merge** charges one unit for each item, and,
when the Document records expected items (`WithExpectations`), at most one more
unit for each item that the record of the farthest failure holds when it adds
the item: it may look for the item among them. That record holds each item at
most once, so a merge charges at most one plus the number of distinct expected
items of the grammar for each item it merges. The operations charge:

- starting the run, and starting the evaluation of an iteration: nothing;
- each attempt to reuse the next recorded iteration: one unit, and when it
  reuses one, the merge of its expected items, and, for an iteration after the
  edit, the move of its value;
- ending an evaluated iteration: the merge of its expected items, and, when it
  matched, at most two units for each expected item that it records (it
  compares them with the last set of expected items that the parse kept and
  copies them unless the two are the same) and, when the number of new
  iterations that it then holds is a power of two, the number that it held
  before;
- looking for a recorded iteration after the edit, before an iteration after the
  edit is evaluated: a binary search over the recorded iterations that the run
  has not passed, at most 1 + log₂(m + 1) units for m such iterations, then one
  unit for each recorded iteration that it scans from the one it finds, which
  stops at the first that examined input before the end of the edit or at the
  end of the record; when it passes them all the run reuses them;
- making room for the values of the record that it resumes: room for every
  iteration of the record on the stack of values. The parse keeps an amount of
  room that it has paid for, zero when it starts. When the values on the stack
  and the room asked for exceed it, the parse charges the number of values on
  the stack, and the room paid for becomes at least twice what it was and at
  least what is asked for; otherwise nothing;
- recording the run when it ends: one unit for each iteration, and nothing for a
  run too short to record.

A parse resumes a record of the previous parse at most once: resuming it removes
it from the records that the parse can resume.

These charges for growing storage, like those of the slots of the memo table,
are functions of lengths and of the operations of the parse: the capacity that
earlier parses left in storage that the Document keeps for its next parse
changes no charge.

**Decoding.** A parse in `Bytes` positions that decodes a character of the text
of a Document charges, besides the unit of a parse from scratch, one unit for
each further byte that it reads for the decoding: at most three more units for
each examination of the input.

## Change Notifications

Notifications are off by default. `doc.EnableChanges(limit)` turns them on for
one document before the edits and parses to be observed. The positive `limit`
bounds the number of queued events; it does not bound the bytes of replacement
text or the size of the comparison snapshot. Enabling twice is rejected.
`doc.DrainChanges()` transfers owned records out and clears the queue. Tracking
retains one comparison snapshot.

There are three kinds of event.

| Kind | Meaning |
|:--|:--|
| `ChangeEdit` | An accepted edit or atomic batch. `From` is the coordinate base and `To` the resulting source revision. Batch edits remain simultaneous replacements in the base coordinates and original order. Failed operations and empty batches emit nothing. Identity edits still advance the version and emit an event |
| `ChangeOutcome` | A `Parse` or `Snapshot` outcome. `From` identifies the preceding comparable capture and `To` the current revision. It MAY cover several previously delivered edits and does not advance an edit-replay cursor, so source revisions and displayed-output revisions are tracked separately |
| `ChangeFullRefresh` | Replaces history that overflowed the limit. Consumers rebuild from the latest state; the marker cannot reconstruct dropped edits or intermediate comparisons. A gap in edit revisions also requires a rebuild |

Edit events mark the whole output conservatively invalidated, and their
re-evaluation is reported as `unknown`, because invalidation and memo statistics
cannot establish execution ranges. Notifications never promise minimal tree
differences.

The first capture reports `OutputMayHaveChanged`. Later complete captures report
`OutputUnchanged` or a conservative `OutputChanged`, comparing immutable node
metadata, positions, string text, elements, scalar fields, syntax diagnostics
and graph sharing. Alias topology and nil versus empty collection differences
count as changes; this is a refresh policy and not language value equality.
Unknown non-syntax diagnostic types count as changed. Source text and parse
statistics are not part of output equality.

With tracking enabled, both `Parse` and `Snapshot` capture and compare, using the
invocation's cumulative context and compute limits. A cancellation or compute limit
interruption returns nil and emits `OutputUnavailable`, which clears comparison
continuity: the next successful capture reports `OutputMayHaveChanged`. Completed
syntax failures stay comparable. Unsupported captures also report unavailable.

Snapshots that consumers retain stay immutable. `Parse` still returns its mutable
`Node`, which later parses of the document can change; a `Snapshot` never changes.
The `To` of a `ChangeEdit` is the document version after the edit, and the `To` of a
`ChangeOutcome` is the document version when the `Parse` or `Snapshot` ran. The
version of the latest outcome can therefore be older than the version of the latest
edit: edits that follow an outcome are not part of it.

## Validation and Errors

| Condition | Result |
|:--|:--|
| `NewDocument` with `RecognizeOnly`, an unavailable backend or an unknown start rule | Error; no document |
| A grammar that uses `#missing`, or a `WithMissingTokens` limit other than zero, in `NewDocument` or `Parse` | Error; no document or result |
| `Edit` range outside the text, or inside a character in `Bytes` | Error; text and version unchanged |
| `EditBatch` with a foreign or stale version, unordered or overlapping edits, repeated starts or invalid ranges | The whole batch is rejected |
| Parse option that changes unit, backend, start rule or mode | Error before execution; caches and statistics kept |
| Cancellation, or a compute or memory limit, during `Parse` | Error and no result; execution state cleared |
| Cancellation, or a compute or memory limit, during `Snapshot` | Nil snapshot and a fatal error |
| Unsupported field value type while freezing | Nil snapshot and `*SnapshotCaptureError` |
| `EnableChanges` twice | Error |
| Stream diagnostic options (`WithDiagnosticCallback`, `WithDiagnosticLimit`) | Rejected |

## Compatibility and Support

All three engine backends and both position units support documents. Generated Go
parsers provide an opt-in `Document` with `NewDocument`, `Text`, `Edit`, `Parse`,
`Stats` and `Node.Clone`, and, with typed values, `Document.ParseAST`; they have
neither `EditBatch`, `Version`, `Unit`, `Snapshot`, `CacheStats`, `ResetHistory`
nor change notifications, and their `Stats` has only `Evaluated` and `Reused`.
Generated TypeScript parsers provide an opt-in `Document` with `createDocument`,
`text`, `bytes`, `edit`, `parse`, `stats` and, with typed values, `parseAST`. It
reuses the memoized results of earlier parses as the generated Go `Document`
does, and its `stats` has only `evaluated` and `reused`. See
[Generated Parsers](generated-parsers.md#incremental-documents).

## Conformance

The behavior is established by the document tests of the repository, which
compare random edits with fresh parses, and by the edit fuzzer of
`zp edit-fuzz`, which compares every reparse with a fresh parse.
