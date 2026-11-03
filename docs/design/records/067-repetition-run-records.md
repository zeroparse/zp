# 067. Repetition Run Records

- **Status**: Proposed
- **Author**: @ornew
- **Date**: 2026-11-04

## Summary

The linear-cost classifier reports a unit as **nested** when its continuation can restart inside the input the unit
examined and a scan reachable from it can run again over the same input through a recursion of rule calls. This record
removes that reason for repetitions by **memoizing the repetitions the classifier points at**. Such a repetition, a
**run site**, records for each `(site, position, environment)` where its iterations ended, so a second activation at
the same key replays the result instead of rescanning. The mechanism generalizes the skip-run sites of record 064 from
skip-only loops to ordinary repetitions of value-free elements. Everything that makes the classifier prove Linear today
stays; a unit whose only unbounded scans are run sites is proved Linear.

The record specifies the behaviour as Exec IR operations and engine semantics first. Generated runtimes follow the Exec
IR; none is edited by hand.

## Motivation

The nine bundled grammars carry about 1,400 nested reasons. A prototype of the classifier marking, measured without an
engine, shows that marking value-free repetitions alone turns about 23% of them into proved Linear (duckdb 672 to 314,
typescript 266 to 250; the other grammars unchanged). The marked share of repetitions per grammar is cel 41 of 47, cue
45 of 59, duckdb 42 of 120, golang 83 of 141, python 70 of 143, typescript 45 of 99, xml 29 of 38, yaml 74 of 93,
minilang 7 of 10 and outline 6 of 7.

The remaining nested reasons come from other scan sources that repetition records do not cover: repetitions whose
iterations build values (golang 56, cel 6), rules that read variables (python 63), operator loops and right operands of
Operators Rules, copying operations whose cost is the value size, and left-recursion growth at a new position. Two of
those widen under this design (values and environments, below); the rest need their own answer and are out of scope.

Two quadratic controls show the gap. In `main = t a? $$; t = a t "!" / . t / ""; a = x+; x = "x"` the repetition `a`
is rescanned at every position of `t`; in `S = (A "y" / .)*` with `A` a repetition, the same input is rescanned after
each abandoned alternative. Memoizing the repetition makes each position cost one scan in total.

## Design

### Which repetitions are marked

The classifier decides, per repetition `e` with no maximum, whether `e` is a **run site**:

- the repetition has no maximum and no escaping cut in its element;
- every inner unbounded repetition is itself a run site;
- the element contains no `#recover`, Operators Rule, stream operator or missing-token attribute;
- the element contains no capture or inline action and no predicate (first version; see Values and environments);
- every called rule is not left-recursive, does not read variables and contains none of the above.

A marked repetition is no longer a scanning source for `checkNesting` and `checkGrowth`; unmarked ones remain sources.
Marking is one decision of the classifier and the compiler consumes it, as it does for skip sites. A run site whose
element is a class or a literal sequence has the **interval** form (below); the others have the **walk** form.

### Key and stored value

The key is `(site, position, environment)`. A value-free element reads no variables, so the environment is the empty
tuple in the first version; the key is `(site, position)`. A record stores:

- the end position of the run, that is the position after the last successful iteration;
- the farthest failure position and the expectation set of the failed iteration that ended the run;
- the examined range, for Document reuse and the budget.

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

### Values and environments (second version)

Repetitions whose iterations build values need the record to hold the values. A copied suffix costs O(rest) per hit and
would break linearity, so the record stores a **persistent tail**: iteration values form a cons chain and a hit shares
the suffix. This widens marking for golang (83 to 123 of 141), cel (41 to 47 of 47) and xml (29 to 38 of 38) in the
classifier prototype, but the nested counts do not move, because other scan sources keep those units nested. That is why
values are a second version, justified once those sources are covered.

Variables enter the key: the environment is the tuple of variable values the element reads, and two activations share a
record only when the tuple is equal. Comparing the tuple costs its size, which is bounded by the grammar. Python's 63
variable-reading rules become markable with this. Left-recursive and `#recover` callees stay excluded.

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

**`#recover`.** The element of a run site contains no `#recover`, and a callee containing one is excluded. The run is
therefore a pure function of its key; the error recovery of an outer unit does not interact with its records. Records are
off when missing-token insertion is on, as the skip-run records are.

## Exec IR

The operations are added to the Exec IR and defined once; each target implements them.

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
produced by the Exec IR lowering once F95 settles the runtime core; no generated runtime is edited by hand before that.

## Cost and trade-offs

- A record costs memory per iteration boundary in the walk form. The interval form costs one record per run; walk form on
  value-free elements is expected to be the cost driver and is what the measurement must show.
- Every marked repetition pays a lookup at activation and the walk bookkeeping on its first pass. Repetitions that are
  never reactivated at the same key pay this for nothing; marking only the repetitions the classifier points at keeps the
  overhead to those that block a Linear proof.
- A parse with no marked repetition is unchanged.

## Alternatives

- **Alignment of scan starts** bounds starts to positions the grammar can reach, and reclassified about 14 of 1,400
  nested reasons in a prototype. It belongs to a separate piece of work.
- **Memoizing every rule** (packrat) removes the nested reason everywhere but charges memory per rule and position and
  costs speed on grammars that never needed it.
- **Copying value suffixes** in records is simple and breaks linearity; persistent tails avoid it.

## Open questions

- The nested reasons that remain after this record (operators, copying operations, left-recursion growth) need their own
  design; whether to treat Operators Rule loops as run sites with per-level keys is not decided here.
- The exact interval-form criteria (class of one character, literal sequence) are fixed by the measurement of the
  engine prototype.
