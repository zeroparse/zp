# Interruptions

Arata Furukawa · Last revised 2026-11-09

This proof shows that an interruption of a parse is atomic, and what is the
same for every run of a backend. An interruption is a
[cancellation or an exhausted limit](../runtime/parsing.md#cancellation-and-compute-limits).
The proof shows that a reservation of budget either proceeds in full or
interrupts before the work it covers, that no grammar construct absorbs an interruption, that an interrupted
parse returns no partial value and leaves nothing behind that a later parse
could read, that a stream keeps what it delivered, and that the reservations of
a backend, and so the point where a budget stops a parse and the error it
reports, are a function of the grammar, the input and the options.

It covers the engine's three backends (closure, recursive
bytecode, iterative bytecode) and generated Go, for batch parses, recognition,
typed parses, [streams](../runtime/streaming.md) and
[Documents](../runtime/documents.md). Generated TypeScript has its own counter,
which this proof does not analyze. A parser generated without limit controls
has no counter and no checkpoints: only the input and depth limits, which it
checks without them, can stop it. The proof is about the parsers that have them.

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

[Reservations and Interruptions](../runtime/parsing.md#reservations-and-interruptions)
states when a reservation is atomic. This proof uses the word for an
interruption as the conjunction of the following, each of which is stated or
implied above.

1. **Atomic reservation.** A reservation proceeds whole, with the work it
   covers, or interrupts, and the work it covers is not performed (C1).
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
reader splits the input. A Document parse under the default budget leaves out
the work that only a Document does (its **Document units**: the memo protocol of a
call, which includes the walks of chains of memo entries, the application of
pending edits, the moves of reused values, the recorded runs, the extra units
of decoding the characters of its text, and the copy of the diagnostics): the
reservation of $d$ Document units raises $B$ by $d$ before it is made, and a
limit error reports $B - \delta$ and $u - \delta$ for the $\delta$ Document units reserved so
far. An explicit budget counts every unit, and then $B$ is constant. So for a
stream and for a Document under the default budget $B$ is a function of the
point of the reservation, written $B_i$ at the $i$th reservation, and never
shrinks, and a constant for every other invocation.

A **reservation** of $n$ units is made before the work it covers. A
**checkpoint** observes the context without reserving. The interval of polling
is $I = 256$ units. The reference reservation $\mathrm{Charge}(n)$, which
[Reservations and Interruptions](../runtime/parsing.md#reservations-and-interruptions)
states, is:

1. $\mathrm{poll}$ holds if $s + n \ge I$, and $\mathrm{exceed}$ holds if
   $u + n > B$ (computed without overflow).
2. If $\mathrm{poll}$ or $\mathrm{exceed}$ holds and the context has an error,
   the invocation is interrupted with the error of the context.
3. Otherwise, if $\mathrm{exceed}$ holds, it is interrupted with a limit error
   (C7).
4. Otherwise $u \leftarrow u + n$ (saturating), and $s \leftarrow 0$ if
   $\mathrm{poll}$ and $s + n$ otherwise. The reservation returns and the work
   follows.

A checkpoint interrupts with the error of the context if it has one, and
otherwise sets $s \leftarrow 0$. An interruption leaves the reservation or the
checkpoint that raises it without returning to the work after it. The rules
that the proof cites do not fix the units used and requested that a limit error
reports, nor the counters that an interruption leaves; the interruption ends
the invocation, and no step of it reads them again.

The charge sequence of an invocation is the sequence $n_1, n_2, \dots$ of its
reservations, in order, together with the positions of its checkpoints among
them and, for a Document, which of the reservations are of Document units.

## Rules of Reservations

The proof also cites the following rules of [Reservations and Interruptions](../runtime/parsing.md#reservations-and-interruptions),
numbered as the proofs below use them.

- **(A1) Precedence.** A context error observed at a reservation that polls
  or exceeds wins over exhaustion (step 2 before step 3). A non-end-of-input
  error that a reader returns together with data wins over a cancellation
  first observed after that read. `LimitInput` precedes the context (C7).
- **(A2) Atomic reservations on every backend.** A reservation of $n$ units
  either proceeds and adds $n$ to the units used, or interrupts with a limit
  error or with the error of the context, and the work that it covers is
  performed only if it proceeds. The runtime of generated Go reaches the same
  sums of units used as $\mathrm{Charge}$, and so exhausts the budget at the
  same reservation. Where it observes the context is a function of the
  reservations and checkpoints before, but need not be where $\mathrm{Charge}$
  observes it.
- **(A3) Reservation precedes work.** Each unit of work that (C6) counts is
  covered by a reservation made before it, in the same straight-line work or
  before a loop whose number of steps is a function of the logical state
  before the loop (A6) and which has no exit other than an interruption or a
  fatal error inside a step.
  So an interruption raised by the reservation precedes the work, and the
  reservation of such a loop is a function of the logical state (A6). The
  units of the steps that such an exit leaves unperformed stay reserved and
  counted in the units used, as (C7) permits. Combining charges within
  straight-line work moves no reservation across a publication (A5): the
  reservation of a loop only moves earlier, before the loop. Allocation,
  runtime growth, garbage collection and the callbacks of the caller are
  outside the budget, as (C4) says for callbacks.
- **(A4) Nothing absorbs an interruption.** `#recover`, ordered choice,
  lookaheads and repetitions are evaluated by failing, not by catching an
  interruption; predicates and bindings in typed parsers catch only the
  run-time error of an action; tracing passes an interruption on.
- **(A5) Publication of input caches.** A cache of a Document's input is
  published only after the reservations that cover building it, with no
  reservation or checkpoint between the steps that publish it.
- **(A6) Charges depend on logical state.** The size of each reservation is a
  function of the grammar, the input, the options and the values of the
  parse, and the charge sequence of a Document parse is a function of the
  sequence of its operations from its creation, provided that no earlier
  operation on it was interrupted by a context. A context cancellation arrives
  at a time that the charges do not fix, and what it leaves in the memo and in
  the tables, although complete, changes later charges. An interruption by a
  limit is a function of the charges, and does not have this effect.

## Lemma 1 (Atomic Reservation)

Every reservation takes exactly one of three outcomes: it proceeds with
$u' = u + n$; it interrupts with a limit error; or it interrupts with the error
of the context. In the last two the work it covers is not performed. The
counter satisfies $u \le B$ after every reservation that proceeds.

*Proof.* On every backend, by (A2). For $\mathrm{Charge}$, steps 2 and 3
interrupt before the reservation returns, so the statements after it, which are
the work (A3), do not run, and step 4, which proceeds, is reached only when
$\mathrm{exceed}$ is false, so $u + n \le B$. Generated Go reaches the same
sums of units used (A2), so it proceeds at the same reservations, with the same
counter. $\square$

The case of a charge that covers decoding is the exception of (C1): the
reservation of the bytes of a chunk precedes the decoding of the chunk, and
the reservation of a character precedes its storing, so the character that
may be decoded before the reservation is neither stored nor used if the
reservation fails.

## Lemma 2 (Interruption Is Final)

After a reservation or checkpoint interrupts, no further step of the parse
runs, and no construct of the grammar absorbs it.

*Proof.* The interruption leaves the reservation or checkpoint that raised it
without returning to the work after it, so that work does not run (A3). By
(A4) no construct on the way absorbs it: the only constructs that catch
anything catch a run-time error of an action, and the others fail without
catching, so a failure of a choice alternative or a predicate is an ordinary
return, and an interruption is not. So the interruption is fatal and reaches
the boundary of the invocation (C2), and no step of the parse runs after it.
$\square$

## Lemma 3 (No Partial Result)

An invocation that is interrupted returns the error of the interruption and no
node, no typed value and no list of recovered errors.

*Proof.* The invocation ends with the interruption (Lemma 2). Its error is the
error of the context or a limit error (C7), and it returns no partial node or
typed value (C3); for a Document parse, an interruption of the publication of
its diagnostics returns no result either
([Parsing](../runtime/documents.md#parsing)). The list of recovered errors is
part of the outcome of a parse that succeeds; an exhausted limit is a run-time
error, whose outcome is no value and that error alone
([Results](../runtime/parsing.md#results)), and a cancellation returns the
error of the context (C7), so no list of recovered errors is returned.
$\square$

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
callback that returns an error ends the parse with that error. The commit
follows the second checkpoint and reserves units for its charged work, so a
reservation of the commit can interrupt the invocation too. The element stays
delivered, nothing is delivered after it, and the state of a commit cut short
is the invocation's own, which nothing reads again (Lemma 5). The commit
demands no input, so the default budget of a stream does not grow between its
reservations. It evaluates no grammar code, and it publishes no cache (A5),
because a stream keeps no input cache past the invocation. Every backend orders
these steps in this way
([Reservations and Interruptions](../runtime/parsing.md#reservations-and-interruptions)). The
nodes that were delivered are owned by the caller, and the parser's discarding
of its own state does not change them. A reader that is blocked is not
preempted; the checkpoint after its return observes the cancellation (C4).
$\square$

The diagnostics of a recovered element can be delivered before the element, so
an interruption between the two leaves the caller with diagnostics whose
element was not delivered. Both are deliveries of the uninterrupted sequence,
and a prefix of it can end between them.

## Lemma 5 (What Survives an Interruption)

Let an invocation be interrupted. The state that outlives it is one of:

1. **The state of a parser.** Whatever a parser keeps from one invocation for
   the next, such as storage it reuses or tables of the program it builds once.
   The specification defines the result of an invocation from the grammar, the
   input and the options, and its charges likewise (C6, A6): neither depends on
   what storage an earlier invocation, interrupted or not, left in the parser.
2. **The caches of a Document's input.** Each is published only after the
   reservations that cover building it, with no reservation or checkpoint
   between the steps that publish it (A5), and an incomplete table is never
   published (C5). So a retained cache is complete or absent.
3. **A Document's execution state.** A Document parse that is interrupted
   before execution has touched the reusable state keeps its valid caches and
   its last statistics, and has changed nothing else (C5). One that is
   interrupted after that clears the memo and the repetition state before the
   next attempt (C5): a parse that does not complete drops the memo entries and
   the runs, and publishes no partial table
   ([Results that are stored](../runtime/documents.md#reuse-model)). The state
   is dropped, not repaired, so it does not matter that the entries could
   depend on a growth that had not ended, or that a result that was being moved
   in place was left partly moved
   ([Interruption](../runtime/documents.md#interruption)).
4. **Delivered elements** of a stream (Lemma 4).
5. **Nodes that the caller holds from earlier parses.** A tree that an
   earlier `Parse` returned may share its nodes with the Document, and a later
   parse may move the nodes it reuses in place. A parse that is interrupted
   while it moves them can leave a tree partly moved, and no later parse moves
   them again ([Parsing](../runtime/documents.md#parsing)). Take a tree $T$
   from `Parse`, then `Edit`, then a `Parse` with a compute limit small enough
   to stop inside the move of a reused value: $T$ can be left partly moved. The
   interruption promises the caller nothing about the positions in $T$, which
   are unspecified once a parse after an edit has started (the Document proof
   does not make them part of the outcome), only that the next parse returns
   the result of parsing the current text from scratch (C5).

*Proof.* Item 1: as stated. Item 2: by (A5) and (C5). Item 3: by (C5) and the
rule for a parse that does not complete. Item 4: Lemma 4. Item 5: by
[Parsing](../runtime/documents.md#parsing). Nothing else of the invocation
outlives it: the counters and the state of a parse, a stream parse or a
Document parse are the invocation's (C1: the counter resets on the next call),
and what the invocation changed in state that the caller owns is item 5.
$\square$

A retry with fresh controls starts from state that Lemma 5 shows to be a state
that the Document would have without ever having parsed: empty memo, empty
runs, complete or absent input caches. The nodes of item 5 are the caller's,
and no parse reads them again. The Document proof shows that a parse
from such a state gives the result of a fresh parse of the text. A retry
therefore gives the result of parsing the current text from scratch (C5).

## Lemma 6 (the Point of Interruption)

Let two invocations have the same limits $B_i$ and the same charge sequence.
Without a context, the invocations are interrupted at the same reservation
$i$, the least $i$ with $\sum_{j \le i} n_j > B_i$ (if there is one), with the
a limit error (C7). With a context, the reservations at which the context is
observed are the same in both: those with $s_{i-1} + n_i \ge I$ and the
checkpoints for $\mathrm{Charge}$, and a function of the same reservations and
checkpoints for generated Go (A2).

*Proof.* Without a context the context error never applies and a poll has no
effect, so by Lemma 1 the counter is $u_i = \sum_{j \le i} n_j$ until the first
reservation at which $u_{i-1} + n_i > B_i$, which interrupts with the error of
step 3. $B_i$ is a constant, or for a stream a function of the demand so far,
which is the same in both invocations because it depends on the input and
the grammar, or for a Document under the default budget a function of the Document units
among the reservations before $i$, which the charge sequence holds. The counters $s_i$ are a function of the sequence and of the
checkpoints, so the reservations that poll are fixed by them, and so are the
observations of generated Go (A2). Whether the
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
   interrupted, at which reservation, and with which kind of error, is a function of
   the charge sequence and the limits, which by (A6) is a function of the
   backend, the artifact, the input, the unit and the options, and for a
   Document of its history, provided that no earlier operation on it was
   interrupted by a context; and so is the same on every run (Lemma 6). A
   stream's charge sequence depends also on how its reader splits its input
   into reads (C6).

*Proof.* Lemmas 1 to 6. The reservations of every backend are atomic and
reach the sums of $\mathrm{Charge}$ (A2), which Lemma 1 covers. $\square$

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
straight-line sequence, or of the lines of a longest match of operators, as one
reservation. The charge sequences of two
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

**The amounts of a limit error.** The units used and requested that a limit
error reports, and the counters that an interruption leaves, are not among the
rules that the proof cites, so it shows nothing about them.

**Generated TypeScript.** It has its own counter and the options `signal` and
`limits.compute`; the proof does not cover its counter.
