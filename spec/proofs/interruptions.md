# Interruptions

Arata Furukawa · Last revised 2026-11-02

This proof shows that an interruption of a parse is atomic, and what is the
same for every run of a backend. An interruption is a
[cancellation or an exhausted limit](../runtime/parsing.md#cancellation-and-compute-limits).
The proof shows that a reservation of budget either happens in full or not at
all, that no grammar construct absorbs an interruption, that an interrupted
parse returns no partial value and leaves nothing behind that a later parse
could read, that a stream keeps what it delivered, and that the reservations of
a backend, and so the point where a budget stops a parse and the error it
reports, are a function of the grammar, the input and the options.

It covers the reference implementation's three backends (closure, recursive
bytecode, iterative bytecode) and generated Go, for batch parses, recognition,
typed parses, [streams](../runtime/streaming.md) and
[Documents](../runtime/documents.md). Generated TypeScript has its own counter,
which this proof does not analyze. A parser generated without limit controls
has no counter and no checkpoints, and cannot be interrupted; the proof is about
the parsers that have them.

## Definitions From the Specification

The proof cites the following, and no other rule about interruptions.

- **(C1) Reservations.** A reservation is checked before the corresponding work
  begins; if it exceeds the remaining allowance, that work is not performed.
  Decoding is the one exception: a reservation may follow the decode of at most
  one character (at most 4 bytes) and must precede storing or using that
  character. The budget counter is monotonic for one invocation and resets on
  the next call.
- **(C2) Fatal interruptions.** Cancellation and exhausted limits are fatal
  interruptions, not ordinary syntactic mismatches. Ordered choice,
  predicates, recovery and other grammar constructs cannot absorb them.
- **(C3) No partial result.** An interrupted batch parse returns no partial
  Node or typed value.
- **(C4) Streams.** Elements already delivered to the caller remain delivered;
  the parser does not retract them. A blocked reader or caller callback cannot
  be preempted; cancellation is observed after the call returns.
- **(C5) Documents.** A cancellation or option error detected before execution
  has touched reusable state preserves its valid caches and last statistics.
  Once execution starts, an interruption clears execution memo and repetition
  state before the next attempt. Fully completed input tables may be retained,
  but incomplete tables are never published and are discarded. A retry with
  fresh invocation controls must produce the same result as parsing the
  current text from scratch.
- **(C6) Charges.** The budget counter adds the inspected elements of the input
  representation and the backend's execution steps, one unit each. Charges are
  not rolled back by backtracking, and their values depend on the backend, the
  position unit and the generated artifact. For a given artifact, input and
  options they are the same on every run: the storage that an earlier parse
  left in a reused parser, and when a collection releases it, never change
  them. A stream also charges one unit for each call to its reader's `Read`,
  so its spend depends on how the reader splits the input as well.
- **(C7) Errors.** A cancelled context returns the error of the context; an
  exhausted limit returns an error that matches `ErrLimit` and carries the
  limit, the amount used and the amount requested. An input over `Input`
  fails with `LimitInput` before a context is consulted.

The specification does not define the word atomic. This proof uses it for the
conjunction of the following, each of which is stated or implied above.

1. **Atomic reservation.** A reservation is performed whole, with the work it
   covers, or not at all and with its work not performed, and a failed
   reservation leaves the counter unchanged (C1).
2. **Final.** An interruption ends the invocation, and nothing inside it
   absorbs or replaces it (C2).
3. **No partial result.** The invocation returns the interruption and no
   value (C3), except for the elements of a stream that were delivered (C4).
4. **No residue.** No state outlives the invocation from which a later
   invocation could give a result that differs from the one it gives from
   fresh state (C5).

## Setting

An **invocation** is one call of an entry point: a parse, a recognition, a
typed parse, a stream parse, a Document parse or the creation of a Document. It
has a control $\mathcal{C}$ with the limit $B$, a context that can be cancelled
(or none), the counter $u$ of units reserved, and the counter $s$ of units
reserved since the last observation of the context.

Every parse has a budget. $B$ is the resolved budget: the `Compute` limit that the
options give, or the default $\max(2^{20}, 1024\,n)$ for `n` units of input,
which is resolved once per entry point from the input it receives (and for a
Document, at each `Parse`, from its current text). `Unlimited` is the largest
value and disables the budget. An invocation is uncontrolled, and cannot be
interrupted, only if its budget is `Unlimited` and it has no context, or if the
parser was generated without limit controls. The proof is about the others.
A stream does not know `n`, and its default budget is $B_t = \max(2^{20},
1024\,d_t)$, where $d_t$ is the number of units up to the furthest position
that the parser has demanded from the stream at the time of the reservation.
It never shrinks, and it depends on the input and the grammar, not on how the
reader splits the input. So for a stream $B$ is a function of the point of the
reservation, written $B_i$ at the $i$th reservation, and a constant for every
other invocation.

A **reservation** of $n$ units is made before the work it covers. A
**checkpoint** observes the context without reserving. The interval of polling
is $I = 256$ units. The reference reservation $\mathrm{Charge}(n)$ is:

1. $\mathrm{poll}$ holds if $s + n \ge I$, and $\mathrm{exceed}$ holds if
   $u + n > B$ (computed without overflow).
2. If $\mathrm{poll}$ or $\mathrm{exceed}$ holds and the context has an error,
   the invocation is interrupted with the error of the context.
3. Otherwise, if $\mathrm{exceed}$ holds, it is interrupted with a limit error
   whose limit is $B$, whose used is $u$ and whose requested is $n$.
4. Otherwise $u \leftarrow u + n$ (saturating), and $s \leftarrow 0$ if
   $\mathrm{poll}$ and $s + n$ otherwise. The reservation returns and the work
   follows.

A checkpoint interrupts with the error of the context if it has one, and
otherwise sets $s \leftarrow 0$. An interruption is a panic that carries the
error, raised by the reservation or the checkpoint before it returns.

The charge sequence of an invocation is the sequence $n_1, n_2, \dots$ of its
reservations, in order, together with the positions of its checkpoints among
them.

## Assumptions

The specification does not state these. The reference implementation applies
them, and its tests check them.

- **(A1) Precedence.** When several interruptions can apply at one place, one
  is chosen as follows. A context error observed at a reservation that polls
  or exceeds wins over exhaustion (step 2 before step 3). A non-end-of-input
  error that a reader returns together with data wins over a cancellation
  first observed after that read. The specification states only that
  `LimitInput` precedes the context (C7).
- **(A2) The generated counter is the reference counter.** The runtime of
  generated Go keeps the units reserved as a base and the part of a grant that
  its fast path has taken, and reserves on the fast path by one subtraction
  when $n < \mathrm{room}$, where room is the smallest request that would poll
  or exceed, computed from the settled counters. A request that is not below
  room, and a checkpoint, settle the counters and take the reference path. A
  take of $n$ units whose $n$ is not below room is made there as $n$
  reservations of one unit, so the charge sequence of generated Go has $n$
  entries where the reference has one; the sum, and so the point where the
  budget is exceeded, is the same, but the requested amount in a limit error
  is 1 and not $n$.
- **(A3) Reservation precedes work.** Each unit of work that (C6) counts is
  covered by a reservation made before it, in the same straight-line code, so
  that an interruption raised by the reservation precedes the work. Charges may
  be combined within straight-line code without moving across a
  publication (A5).
  Allocation, runtime growth, garbage collection and the callbacks of the
  caller are outside the budget, as (C4) says for callbacks.
- **(A4) Reachable recovery.** The only `recover` calls on the path of an
  evaluation are those at the boundary of an invocation, those that classify
  and re-raise, and those of predicates and bindings in typed parsers, which
  absorb only the panic of a run-time error of an action. Tracing also
  re-raises fatal errors and wraps other panics, and does not absorb an
  interruption. `#recover`, ordered choice, lookaheads and repetitions are
  evaluated by returning a failure, not by recovering a panic.
- **(A5) Publication of input caches.** A cache of a Document's input, such
  as a table built from its text, is assigned in statements between which no
  reservation or checkpoint is made, after the reservations that cover the
  work that builds it, so that it is published only when complete.
- **(A6) Charges depend on logical state.** The size of each reservation is
  computed from the grammar, the input, the options and the values of the
  parse, never from the address, capacity or contents of reusable storage, the
  order of iteration over a map, or time. (C6) requires it for reused parsers; a
  test of generated parsers and the budget golden files check it. A Document's
  reusable state is part of its input to a parse: the charge sequence of a
  Document parse is a function of the sequence of its operations from its
  creation, provided that no earlier operation on it was interrupted by a
  context. A context cancellation arrives at a time that the charges do not
  fix, and what it leaves in the memo and in the tables, although complete,
  changes later charges. An interruption by a limit is a function of the
  charges, and does not have this effect.

## Lemma 1 (Atomic Reservation)

Every reservation takes exactly one of three outcomes: it proceeds with
$u' = u + n$; it interrupts with a limit error and leaves $u$ and $s$ as they
were; or it interrupts with the error of the context and leaves $u$ and $s$ as
they were. In the last two the work it covers is not performed. The counter
satisfies $u \le B$ after every reservation. A take that generated Go
makes as $n$ reservations of one unit (A2) is a sequence of reservations, each
atomic; the work it covers is performed only after the last of them, so a take
is whole or its work is not performed, but a failed take may have reserved
some of its units.

*Proof.* The steps of $\mathrm{Charge}$ change $u$ and $s$ only in step 4, which
is reached only if neither interruption applied, so they are unchanged by the
others. In steps 2 and 3 the panic leaves the reservation before it returns,
so the statements after it, which are the work (A3), do not run. In step 4,
$\mathrm{exceed}$ was false, so $u + n \le B$. For the generated counter (A2),
the fast path is taken only when $n < \mathrm{room}$. Room is the smallest
request that polls or exceeds from the settled counters, minus what the fast
path has taken since, so a request below it neither polls nor exceeds, and
$u' = u + n$ with the effect of step 4. Any other request settles the counters
and takes the reference path. $\square$

The case of a charge that covers decoding is the exception of (C1): the
reservation of the bytes of a chunk precedes the decoding of the chunk, and
the reservation of a character precedes its storing, so the character that
may be decoded before the reservation is neither stored nor used if the
reservation fails.

## Lemma 2 (Interruption Is Final)

After a reservation or checkpoint interrupts, no further step of the parse
runs, and no construct of the grammar absorbs it.

*Proof.* The interruption is a panic. It propagates up the stack of the
evaluation until a `recover`. By (A4) the recovers on the way are the ones
that re-raise a panic that is not a run-time error of an action, and the
boundary of the invocation. The reservations and checkpoints themselves do
not recover. So the panic reaches the boundary. Between the interruption and
the boundary only deferred cleanup runs, and it holds no reservation: its cost
is not charged, because it must not be interrupted (the release of pooled
scratch, the transfer of state after a Document parse, and the discarding of a
Document's state). The ordinary grammar constructs return a
failure, so a failure of a choice alternative or a predicate is an ordinary
return, and an interruption is not. $\square$

## Lemma 3 (No Partial Result)

An invocation that is interrupted returns the error of the interruption and no
node, no typed value and no list of recovered errors.

*Proof.* At the boundary the recovery of an interruption sets the result to its
zero value and the error to the error of the interruption (the same holds
for a recognition, a Document parse and a typed parse, whose boundary also
covers the conversion of a node into a typed value). The recovered errors of
a parse are in the parser, which the boundary does not return. A value built so
far is not returned by any other path, because the interruption leaves the
evaluation by Lemma 2. $\square$

## Lemma 4 (Streams Keep What They Delivered)

When an invocation of a stream parse is interrupted, the elements that its
callback received are a prefix, in order, of the sequence of elements that the
uninterrupted parse of the same input delivers; each was delivered whole; and
the interruption delivers no further element.

*Proof.* A reservation that proceeds changes nothing in the evaluation (Lemma
1), and the evaluation is deterministic, so until the interruption the
interrupted parse takes the same steps as the uninterrupted parse of the same
input, with the same deliveries in the same order. Its deliveries are
therefore a prefix of the deliveries of the uninterrupted parse, whatever
those are. The emission of an element checkpoints the context first, so a
context that is already cancelled stops the parse before the callback is
called, and the element is not delivered. It then calls the callback, and
checkpoints after it returns. If the second checkpoint interrupts, the element
has been delivered and stays so (C4), and nothing is delivered after it. A
callback that returns an error ends the parse with that error. The nodes that
were delivered are owned by the caller, and the parser's discarding of its own
state does not change them. A reader that is blocked is not preempted; the
checkpoint after its return observes the cancellation (C4). $\square$

The diagnostics of a recovered element can be delivered before the element, so
an interruption between the two leaves the caller with diagnostics whose
element was not delivered. Both are deliveries of the uninterrupted sequence,
and a prefix of it can end between them.

## Lemma 5 (Nothing Survives but Complete State)

Let an invocation be interrupted. The state that outlives it is one of:

1. **Pooled scratch.** The buffers of a parser that was acquired from a pool.
   They are returned by cleanup that runs on every exit and clears the memo
   table, so what the next parse reads from them is reset before use; by (A6)
   their capacity never enters a charge.
2. **The caches of a Document's input.** The table of line starts, the table
   of offsets, the full text and the copied counter are each published only
   when complete: a builder that is interrupted discards its table, the table
   of offsets is built in a local slice and assigned when it is complete, and
   the other two are assigned after the work that makes them. So a retained
   cache is complete or absent.
3. **A Document's execution state.** A Document parse that is interrupted
   after it started discards the memo table, the run records and the other
   reusable execution state, and the next parse starts from empty ones. The
   deferred handler of the run marks the Document aborted when it sees a
   panic, and the Document's own deferred function covers a panic that is
   raised later, in the conversion of the result or the copying of its
   diagnostics. A Document parse that is interrupted before it has touched the
   reusable state, by the admission checks or the first checkpoint, has changed
   nothing.
4. **Program-level state** that is built once, outside the charge, such as
   the tables of a program that a parse needs: it is complete or absent, and
   is not affected by the invocation that triggers its construction.
5. **Delivered elements** of a stream (Lemma 4).

*Proof.* Item 1: the parser of a batch parse arms its release before the first
reservation that can interrupt, and the release runs in a deferred call. Item
2: as stated, by (A5), since no reservation lies between the work and the
assignment that publishes it. Item 3: the deferred function that sees a panic
marks the parse aborted before it transfers state, and the transfer of an
aborted parse replaces the memo by a new table and drops the run records and
the other execution state. The state is replaced rather than repaired, so it
does not matter that the interrupted parse changed the memo table in place,
with entries that depended on a growth that had not ended (the cause of this
rule in (C5)). The checks that precede the start of the execution are before
any statement that changes a Document field, and `Edit` is not part of the
parse and is not budgeted. Item 4: construction is not covered by a
reservation, so no interruption occurs inside it. Item 5: Lemma 4. Nothing
else outlives the invocation: the parser, the control and the memo table of a
stream parse or a Document parse are the invocation's. $\square$

A retry with fresh controls starts from state that Lemma 5 shows to be a state
that the Document would have without ever having parsed: empty memo, empty
runs, complete or absent input caches. The Document proof shows that a parse
from such a state gives the result of a fresh parse of the text. A retry
therefore gives the result of parsing the current text from scratch (C5).

## Lemma 6 (the Point of Interruption)

Let two invocations have the same limits $B_i$ and the same charge sequence.
Without a context, the invocations are interrupted at the same reservation
$i$, the least $i$ with $\sum_{j \le i} n_j > B_i$ (if there is one), with the
same limit error: limit $B_i$, used $\sum_{j < i} n_j$, requested $n_i$. With a
context, the reservations at which the context is observed are the same in
both: those with $s_{i-1} + n_i \ge I$ and the checkpoints.

*Proof.* Without a context the context error never applies and a poll has no
effect, so by Lemma 1 the counter is $u_i = \sum_{j \le i} n_j$ until the first
reservation at which $u_{i-1} + n_i > B_i$, which interrupts with the error of
step 3. $B_i$ is a constant, or for a stream a function of the demand so far,
which is the same in both invocations because it depends on the input and
the grammar. The counters $s_i$ are a function of the sequence and of the
checkpoints, so the reservations that poll are fixed by them. Whether the
context has an error at such an observation depends on time, which is the one
input of a cancellation that the charge sequence does not fix. $\square$

Between two observations at most $I - 1$ units are reserved, so a cancellation
is observed after fewer than $I$ more units; a charge of $I$ units or more
observes it itself.

## Theorem (Interruptions Are Atomic, and the Charges of a Backend Repeat)

For each of the backends of the proof, the following hold for every
invocation that is controlled.

1. A reservation is atomic (Lemma 1), and an interruption is final
   (Lemma 2).
2. An interrupted invocation returns the error of the context or a limit error
   that matches `ErrLimit` with the limit, the units used and the units
   requested (C7), and no value (Lemma 3), except for the elements of a stream
   that were delivered, which are a prefix of those of the uninterrupted parse
   (Lemma 4).
3. It leaves no state from which a later invocation differs from one on fresh
   state (Lemma 5), so a retry gives the result of a fresh parse.
4. If the limit is the only interruption, whether the invocation is
   interrupted, at which reservation, and with which error, is a function of
   the charge sequence and the limits, which by (A6) is a function of the
   backend, the artifact, the input, the unit and the options, and for a
   Document of its history, provided that no earlier operation on it was
   interrupted by a context; and so is the same on every run (Lemma 6). A
   stream's charge sequence depends also on how its reader splits its input
   into reads (C6).

*Proof.* Lemmas 1 to 6. The reservations of the three backends of the engine
are made by one implementation of $\mathrm{Charge}$, and those of generated
Go by the runtime's counter, which Lemma 1 shows equal to it (A2). $\square$

## What the Proof Does Not Show

**Across backends the results agree and the budgets do not.** The
[Modes](../runtime/parsing.md#modes) table guarantees that the backends give
identical values, errors and positions, and that they differ in speed and in the
default depth limit. Every backend reaches the depth limit at the same nesting
of the same input; exceeding it is a separate run-time error, not an
interruption, and the proof excludes it from "the same
result and the same error". For a parse that no limit interrupts and that stays
within the depth limit, every backend therefore gives the same result and the
same error. (C6) says that the values of the charges depend on the backend and
on the generated artifact, and generated Go places the charge of a
straight-line sequence as one reservation. The charge sequences of two
backends are therefore different, and a limit that stops a parse on one
backend can let it finish on another. What the backends share when a limit
does interrupt is the form of the interruption (this proof), not its point. A
statement that the charges of different backends are equal for the same input
is not a rule of the specification, and the proof does not prove it.

**Interruption points inside the caller.** A reader or callback that does not
return cannot be interrupted (C4). The proof treats its call as one unit of
work between two checkpoints.

**Allocation.** An allocation or a runtime growth that exceeds memory is not a
reservation (A3).

**Generated TypeScript.** It has its own counter and the options `signal` and
`budget`. The parity tests compare it with its inactive route; the proof does
not cover its counter.
