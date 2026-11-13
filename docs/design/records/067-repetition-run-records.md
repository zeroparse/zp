# 067. Repetition Run Records

- **Status**: Partially Implemented
- **Author**: @ornew
- **Date**: 2026-11-04

## Summary

The linear-cost classifier reports a unit as **nested** when its continuation can restart inside the input the unit
examined and a scan reachable from it can run again over the same input through a recursion of rule calls. This record
removes that reason for repetitions by **memoizing the repetitions the classifier points at**. Such a repetition, a
**run site**, records for each `(site, position, environment)` where its iterations ended, so a second activation at
the same key replays the result instead of rescanning. The mechanism generalizes the skip-run sites of record 064 from
skip-only loops to ordinary repetitions of value-free elements. Everything that makes the classifier prove Linear today
stays; a unit whose only unbounded scans are run sites is proved Linear. Run sites alone are not enough: the stages
below, run records (A), records for the Operators Rule (B) and a size argument for copying (C), are each necessary
to make every nested reason Linear.

The record specifies the behaviour as Exec IR operations and engine semantics first. Generated runtimes follow the Exec
IR; none is edited by hand.

**Status.** Stage (A) is implemented in the engine for the closure backend and both bytecode VMs, with the
exactness and eligibility rules of [Run Sites](../../../spec/runtime/parsing.md#run-sites) and the measurements of
[Optimization 133](../../optimizations/133-run-records-of-restarted-repetitions.md). Not implemented: the classifier
does not yet count a run site as Linear (its marking is off, so the classes do not move), the records for the
Operators Rule (B) and the size argument for copying (C), the operations in Exec IR, and records in the generated Go
and TypeScript runtimes. The sections below keep the full design.

## Motivation

The ten bundled grammars carry 1,400 nested reasons. A prototype of the classifier marking, measured without an
engine, shows that marking value-free repetitions alone turns about 23% of them into proved Linear (duckdb 672 to 314,
typescript 266 to 250; the other grammars unchanged). The marked share of repetitions per grammar is cel 41 of 47, cue
45 of 59, duckdb 42 of 120, golang 83 of 141, python 70 of 143, typescript 45 of 99, xml 29 of 38, yaml 74 of 93,
minilang 7 of 10 and outline 6 of 7.

Two quadratic controls show the gap. In `main = t a? $$; t = a t "!" / . t / ""; a = x+; x = "x"` the repetition `a` is
rescanned at every position of `t`; in `S = (A "y" / .)*` with `A` a repetition, the same input is rescanned after each
abandoned alternative. Memoizing the repetition makes each position cost one scan in total.

## Stages and evidence

A scanning source keeps a unit nested when the unit can reach it through a recursion of rule calls. The classifier
reports four kinds of source besides `#recover` skips: unbounded repetitions, Operators Rule operator loops and right
operands, copying operations, and the growth of a left-recursive rule at a new position. The table drops source kinds
in a classifier prototype and counts the nested reasons that remain (a count, not a soundness claim: a dropped kind
is a kind a stage must make sound). "Values and environment" means that repetitions whose elements build values, read
variables or contain predicates are run sites too.

Starting from "every kind dropped" (2 left), each row puts back one kind as a scanning source:

| Kind put back as a source | Nested reasons left |
|---|---|
| none | 2 |
| left-recursion growth | 2 |
| operator loops and right operands | 959 |
| copying operations | 1016 |
| repetitions whose elements build values | 1015 |
| repetitions whose elements read variables or contain predicates | 1008 |
| all repetitions (no run sites at all) | 1400 |

Without stage (A), none of the other kinds changes the count at all (operators, copying or growth dropped alone: no change). With
value-free run sites only, 1400 falls to 1026 (duckdb 672 to 314, typescript 266 to 250, the others unchanged); with
values and environments 1016. Run sites with values and environments plus operators plus copying dropped give 2 (the
two left are explained below); dropping any one of the three leaves 959 to 1016. Left-recursion growth needs nothing: it
does not move any count.

The per-grammar counts, base to the three stages together, are cel 12, cue 56, duckdb 672, golang 165, python 183,
typescript 266, xml 4, yaml 38, minilang 2, outline 2 (total 1400) down to 0 everywhere but minilang. The marked share of
repetitions per grammar with values and environments allowed is cel 47 of 47, golang 123 of 141, xml 38 of 38; with
value-free elements only: cel 41 of 47, cue 45 of 59, duckdb 42 of 120, golang 83 of 141, python 70 of 143, typescript
45 of 99, xml 29 of 38, yaml 74 of 93, minilang 7 of 10, outline 6 of 7.

**(A) Run records for repetitions**, with the values and the environment in the key (this record's Design). They carry
the largest single drop (value-free: 374 of 1400, almost all duckdb) and are required for the rest. Repetitions whose
element calls a `#recover` rule must be markable too: in minilang `(-ws s:stmt)*` repeats a rule that contains
`#recover`, and the option `else?` stays nested only through that repetition. With recovery allowed in run sites, the
all-stages count for minilang is 0. A record then holds the recorded errors of the run and replays them.

**(B) Records for the Operators Rule.** The operator loop of an Operators Rule is a repetition and becomes a run site
keyed by position and level. The right operand of an operator part that can apply operators is a recursive call at a
higher level; it needs an operand record keyed by (position, level) holding the operand value, which is a packrat memo
for operands only.

**(C) Copying is a value-size question, not memoization.** A copying operation costs the size of the value it copies,
so recording where a scan ended does not help. Of the copying sources in the ten grammars, 268 of 291 copy only own
values, whose size is bounded by the input their match consumes; 23 (cue 3, python 20) copy variables, fold
accumulators or values of a call on a recursion. With stages (A) and (B) in place, dropping exactly the own-value
copies leaves 236 of 1400 nested reasons: cue 56 and python 178, every other grammar 0 (minilang 2, from the
recovery above). So most of (C) reduces to the size-degree proof of the cost proof, which bounds the copy of an own
value by the input of its match, and needs no new runtime mechanism. The remaining 23 sources in cue and python need
the size argument for variables, accumulators and recursive values, which is separate work.

## Design

### Which repetitions are marked

The classifier decides, per repetition `e` with no maximum, whether `e` is a **run site**:

- the repetition has no maximum and no escaping cut in its element, and does not repeat a single character (a scan);
- every inner unbounded repetition is itself a run site or repeats a single character;
- the element contains no Operators Rule, `#stream`, `#missing`, `#error` or `#diagnostic`; captures, actions,
  predicates, variables, `#recover` and calls are allowed;
- every called rule is called without a level, is not left-recursive and contains none of the above;
- the repetition is restartable: it is in the element of another repetition that iterates, or in a rule such an element
  calls or that calls itself (directly or indirectly). Any other repetition is reached once by a derivation, so a walk of
  it starts at each position at most once and no record could be reused.

A marked repetition is no longer a scanning source for `checkNesting` and `checkGrowth`; unmarked ones remain sources.
Marking is one rule that the classifier and the compiler both apply (`RestartableRepeats` and the eligibility tests are
shared in meaning, and a test compares the repetitions they mark on the bundled grammars). The classifier's use of the
marking stays off until the proofs of the bounds exist, so the classes do not move.

### Key and stored value

The key is `(site, position, environment)`: the environment is the object that holds the values of the variables at the
start of the walk (none without variables), compared by identity. Two separately built environments with equal values
do not share a record, which only loses a reuse. A record stores:

- the end position of the run, that is the position after the last successful iteration, and the environment there;
- the farthest failure position and the expectation set of the failed iteration that ended the run;
- the examined range, for Document reuse and the budget;
- for an element that builds values, the values of the rest of the run as a **persistent tail** (below);
- the errors that the rest of the run recovered, as a list shared like the tail, which a hit recovers again as a memoized
  result does.

A run activation at `p` yields exactly what the plain loop yields from `p`: one successful result at the recorded end,
and the recorded failure information merged as the plain loop would merge it. This is the exactness lemma of
spec/proofs/skip-runs.md applied to a larger class of elements.

The first activation at a key walks plainly while the engine keeps a visited mark; an activation that meets a visited
mark of an enclosing walk is in a cycle and walks plainly. After the walk, the record is written. A walk that reaches a
position `q` already holding a record of the same site stops there, takes `end(q)`, and writes records for the positions
it passed. Intermediate positions of a loop are written only for the form that needs them: the interval form writes one
record per run, the walk form one record per iteration boundary the walk crossed.

The **interval** form is for class runs such as whitespace. Its record is one interval `[start, end)` per run; a lookup
at any position inside it answers `end` with one comparison, so the per-iteration visit cost of the walk form is not paid.

A site keeps no records until its walks show that they restart. The engine keeps, for each site and lookahead state, the
furthest position that its walks of at least 64 units reached, and the site turns **hot** at the first walk that begins
below that position and walks at least 64 units below it. Until then a walk is the plain repetition and the site holds one
small entry. The cold cost is linear in the input and the number of walks: a cold walk costs its length, and the part
below the furthest position is under 64 units (or the site would be hot), so the walks cost in total at most the new
input they reach plus 64 per walk, however they alternate between positions. A comparison with the last walk only is not
enough: two walks that alternate between disjoint spans never overlap the one before them and would stay cold. A site
whose walks never go back, as a list that a parse reads once, allocates nothing. The test is conservative about gaps: a
walk that begins in a gap below the furthest position counts as walking input again, which only makes a site hot earlier.

The key of a record is the identity of the environment, so equal environments built separately do not share records. A
rule that rebinds a variable at each position to the same new value (`{ m := 1 }` after a backtrack) builds a fresh
environment each time, and its walks stay quadratic even when the site is hot; keying by value, or building equal
environments once, would lift that, at the price of a hash or a cache on each binding.

A replay does not enter the calls of the recorded walk, so their nesting does not count toward the depth limit, as for
a call answered from the memo.

### Why every nested reason becomes Linear

Proof idea for a lemma in spec/proofs/linear-cost.md.

1. **Activations repeat only after abandoned committing units** (Lemma 4 on restarts). A source activates at a position
   `p` more than once only if a unit that examined `p` was abandoned and a later alternative reached `p` again.
2. A run site is a function of `(site, p)`: its element reads no variable and no state outside the input, so its result
   is determined by its key. This holds because every rule it calls is variable-free and not left-recursive.
3. With records, the cost of all activations of a run site at all positions is the cost of one plain walk per distinct
   key that no earlier record covers, plus O(1) per lookup. A walk from `p` stops at the first position holding a
   record, so every iteration boundary is walked at most once per parse. Total element evaluations are at most
   `2(n + 1) + W(c_min + 1)`, the bound of skip runs, with `W` the number of walks and `c_min` the least element cost.
4. The nested reason said that a scan could run again over the same input; step 3 shows a marked scan runs once per
   position. The unit's cost per start is therefore its own bounded work plus O(1) lookups, and the Linear argument for
   committing and non-nested units applies unchanged.

The proof does not need alignment: it does not bound where activations start, only that repeated ones are free.

### Values and environments

Repetitions whose iterations build values need the record to hold the values. A copied suffix costs O(rest) per hit and
would break linearity, so the record stores a **persistent tail**: the walks of a site write their elements into shared
buffers, a record holds a suffix of a buffer, and a hit at the start of a walk shares the suffix without copying. A walk
that reaches a record in the middle of its own elements places them before the suffix when the buffer has room before it,
and copies the suffix otherwise (O(tail) for that merge). The elements of a record are final.

Variables enter the key as the environment. Left-recursive callees stay excluded.

### Interactions

**Documents.** A Document keeps the run records it produced (`Runs` and `LastRuns`, keyed by `RunKey{Site, start}`).
An edit invalidates every record whose examined range intersects the edited range, shifted by the delta for records
after it, as it already does for resumable run sites. The examined range of a run record includes the failed iteration
that ended the run, so an edit after the last iteration that could change why it ended invalidates the record. Interval
records are split or dropped when an edit falls inside. A reused record is charged what a hit costs, not what the
original walk cost.

**Streams.** Records are tied to positions that the stream has not committed. When the stream commits input and prunes
the buffer, it drops all records whose start is before the new buffer origin. A run that is still open at the end of the
buffered input is not recorded, since more input can extend it.

**Budget.** The compute budget charges each element evaluation of a walk as the plain loop does. A hit charges one unit
plus the examined-range scan the plain engine would charge for the inspected input, which keeps the examined-input
measure of the limits record exact. The memory budget charges a record when it is stored: one record per interval or
one per iteration boundary, at the engine's record size.

**Left-recursion seeds.** A run site is not allowed in a left-recursive rule, and a rule that calls a left-recursive rule
is excluded as well, so a seed never sees a record that depends on a grown value. A run site outside any cycle is
unaffected by seeds.

**`#recover`.** The element of a run site and its callees may contain `#recover`. A record keeps the errors the rest of
the run recovered, and a hit recovers them again, so the parse's recovered errors are those of the plain walk. Records are
off when missing-token insertion is on, as the skip-run records are.

## Exec IR

The operations below are the intended Exec IR definition; Exec IR does not contain them yet, and the engine implements
the behavior directly.

- `RunBegin site` at the start of the repetition at position `p`: if a record exists for `(site, p)`, set the position
  to its end, merge its farthest failure and expectations, and jump to the loop exit; otherwise, mark the key visited and
  enter the walk.
- `RunStep site` at the end of each successful iteration: if a record exists at the new position, take its end and exit
  the loop; otherwise continue.
- `RunEnd site` on loop exit after a walk: write the records for the positions passed, with end, failure and examined
  range; clear the visited mark. On a failed enclosing alternative the visited mark is cleared and no record is written.
- Interval form: `RunSpan site class` scans the class once and writes one interval, so a hit is a comparison.

The engine implements the three operations by extending the skip-run machinery of the core (`SkipBegin`, `SkipHit`,
`SkipVisit`, `SkipEnd`) from skip loops to ordinary repetitions; the walk state is not nested. The generated runtimes are
produced by the Exec IR lowering once Exec IR holds the runtime core; no generated runtime is edited by hand before that.

## Cost and trade-offs

- A record costs memory per iteration boundary in the walk form. The interval form costs one record per run; walk form on
  value-free elements is expected to be the cost driver and is what the measurement must show.
- A marked repetition that is walked once from each position pays a test and a store per walk and keeps nothing: the
  hot test above, and the restartable condition, keep the cost of grammars that gain nothing from records close to
  that of the engine without run sites. The measurements are in [Optimization 133](../../optimizations/133-run-records-of-restarted-repetitions.md).
- Records are kept only for the walks of a hot site. A hot site pays a bit per position between its first and last walk,
  and a record per iteration boundary of the walks that visit a position again.
- A parse with no marked repetition is unchanged.

## Alternatives

- **Alignment of scan starts** bounds starts to positions the grammar can reach, and reclassified about 14 of 1,400
  nested reasons in a prototype. It belongs to a separate piece of work.
- **Memoizing every rule** (packrat) removes the nested reason everywhere but charges memory per rule and position and
  costs speed on grammars that never needed it.
- **Copying value suffixes** in records is simple and breaks linearity; persistent tails avoid it.

## Open questions

- A record hit replays the errors of the rest of the run in time proportional to their number; whether that needs a
  sharper bound depends on grammars that recover often inside a restarted repetition.
- The operand record of (B) holds values; whether it shares the persistent-tail representation of (A) is open.
- The 23 non-own copying sources (cue 3, python 20) need a size argument for variables, accumulators and recursive
  values; the own-value copies are covered by the size proof.
- The exact interval-form criteria (class of one character, literal sequence) are fixed by the measurement of the
  engine prototype.
