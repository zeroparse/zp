# 138. Scan Interval Records

- **Status**: Implemented
- **Date**: 2026-11-17

## Problem and Mechanism

[Scan records on every target](137-scan-records-on-every-target.md) answered a walk of a one-character scan only
from the position where an earlier walk had begun, and only in a rule that calls no rule. Walks that begin at
successive positions of one run of the class, as the alternatives of a repetition that consume one character and
then try the scan again do, each made a record of their own and walked the rest of the run: the grammar
`rule X = [ab] [b]* "c" / [ab]` repeated over `a` and `n` letters `b` cost quadratic steps on every target, and a
record per start held memory for each of them. A scan in a rule that calls another rule was walked again from every
start.

Two changes close this.

- **A scan site keeps runs, not records of starts.** [Scan Sites](../../spec/runtime/parsing.md#scan-sites) define a
  run as a range of one stretch of the class, from its first position to the end of the stretch, which walks of the
  site have passed. A walk at a hot site that reaches a position where an earlier walk began an iteration takes the
  rest from the run: the end, the number of characters, the elements and the examined range. A walk that began
  before a run and reached its first position extends it back to its own start. A walk that took no run records
  itself as a new run. Successive starts in one stretch, from its start or from its end, then cost one walk of the
  stretch between them. A walk is charged as before: one step for the lookup, one for a run it takes, one for a run
  it makes. A walk inside the encoding of a character of a bytes-unit input takes a run only from 8 units before
  its end, and a walk that met a covered position that is not an iteration start makes no run.
- **Every rule.** A scan site is any unbounded repetition of a character class or any character, in any rule,
  other than a skip-run site or a `#stream` repetition. An iteration tests one character, so a run holds whatever
  the rule around it calls.

A table of runs holds, for the positions between the first and the last that its runs cover, one bit for each
position and four bytes for each 64 positions, in a window that grows by doubling, so at most three bits for each
such position, and a record for each run. A position finds its run through the runs of its 64 positions, at most
five, in constant time. Runs are kept in the order they are made, linked from the head of each block of 64
positions; an earlier version that kept them sorted moved O(runs) entries on each insert and made a stretch walked
from its end quadratic in time. A site has one table for the walks that build a value and one for those that do
not, and the hot state of each is kept apart for walks inside lookaheads.

Generated Go and TypeScript parsers keep the same runs with the same constant of 16 characters and the same
charges, which raises the generated parser protocol to 19. A TypeScript parser copies the elements of a run when a
walk takes it from a position after its first, as it cannot share a part of an array.
[Lemmas 4 to 7 of the run-site proofs](../../spec/proofs/run-sites.md#scan-sites) show the runs exact and bound the
iterations of the walks of a site to a constant times the input length plus a constant per walk, when no walk
begins inside the encoding of a character.

## Applicability

| Execution profile | Applied / Partial / Not applied / Not applicable | Reason |
|:--|:--|:--|
| Closure, bytecode, iterative bytecode | Applied | Every scan site keeps runs; always on |
| Generated Go and TypeScript | Applied | The same runs, constant and charges in the generated runtimes; protocol 19 |
| Documents, closure, bytecode, iterative bytecode | Not applied | A repetition that a Document resumes neither looks up nor makes runs; the runs of the other parses last for one parse |
| Documents, generated Go | Applied | A generated parser resumes no repetition, so its walks look up and make runs as in a plain parse; the runs last for one parse |
| Documents, generated TypeScript | Not applicable | TypeScript parsers do not expose Documents |
| Streams, engine and generated Go | Applied | A walk records only input that is loaded; a stream drops the runs that end at or before its commit |
| Missing-token insertion | Not applied | A parse that inserts tokens keeps no runs |

## Reproduction

Base: `2bf0e9c47`, the revision before the change; candidate: the commits of the change, on that base. Go 1.27.1,
Node 24.13 for TypeScript, Apple M3 Max, `GOMAXPROCS=4`, every run serialized with the benchmark lock of the
machine. Other work kept the load average between 4 and 80 during the runs, which the A/A columns show.

- Growth: the scan interval controls of `internal/testing/codegentest` (a nest of alternatives that consume one
  character and try the scan again, in a rule, in a scan rule, in a rule that calls a rule, building a value, over
  two-byte characters, and a stretch walked from its end) at n = 100 to 800 on the engine backends, base and
  candidate (steps at a granularity of 256). Generated Go and TypeScript parsers at n = 200, 400, 800, with the
  candidate's library and with a copy mutated so that a run answers only walks from its first position, as the
  records of record 137 did (`TestScanSitesGeneratedIntervals`, `TestScanSitesTSIntervals`).
- Time and memory: test binaries of the `bench` package for the base, an identical copy of the base (A/A) and the
  candidate, run in rotated order for 20 rounds over `BenchmarkParse` (code points on the closure, bytecode,
  iterative and generated backends, and generated AST) at 200 ms each with `-benchmem`; TypeScript parsers
  generated from the seven grammars of `bench` by each `zp`, timed by `bench/ts-parse.mjs` for `parse` and
  `parseValue` in the same rotation. Compared with `benchstat`.

## Results

Growth exponents per doubling (1 is linear, 2 quadratic):

| Cases | Engine, base | Engine, candidate | Generated Go, starts only | Generated Go, candidate | TypeScript, starts only | TypeScript, candidate |
|:--|:--|:--|:--|:--|:--|:--|
| Five forward controls | 1.93–1.99 | 1.00–1.02 | 1.96–1.98 | 1.01 | 1.88–1.95 | 1.01–1.02 |
| A stretch walked from its end | 1.73–1.92 | 0.99–1.00 | 1.97–1.99 | 1.00 | 1.91–1.95 | 1.01 |

On the closure backend the inline control costs 21,616, 83,200, 326,656 and 1,294,336 steps at the base and 5,680,
11,488, 23,104 and 46,336 with runs. The 21 negative controls of the complexity tests whose growth restarts a
one-character scan (growth steps, bounded scans, rescanned calls, word exclusion, region bounds, shared literals and
skips) now measure linear on the engine; the recovery skip of a called rule in the
[linear rewrites recipe](../cookbook/linear-rewrites.md#a-recovery-skip-followed-by-failure) costs 7,130, 14,730 and
29,930 steps at n = 100, 200, 400 against 13,772, 47,672 and 175,472 at the base. The classifier is unchanged: it
counts no runs, so those grammars keep their classes.

Time against the base, 20 rounds:

| Set | Candidate | A/A |
|:--|:--|:--|
| Generated Go, AST, 8 rows, geometric mean | +0.32% | +0.04% |
| Generated Go, rows beyond the noise | Arith_LeftRec −1.31% (p = 0.001) | none |
| TypeScript, 14 rows, geometric mean | +0.22% | +0.27% |
| TypeScript, rows beyond the noise | `parse` XML +1.89% (p = 0.001) | none |
| Engine and generated code points, 32 rows | ENGINE_TIME | ENGINE_AA |

Memory per parse against the base:

| Set | Bytes per parse | Allocations |
|:--|:--|:--|
| Generated Go, AST | Outline +0.78% (p < 0.001), the other 7 rows within the noise; geometric mean +0.17% | Outline +0.62%, the others unchanged |
| Engine and generated code points | ENGINE_MEM | ENGINE_ALLOCS |

A prototype that kept a run for each start in a plain table, before the bitmap, cost Outline +30.7% bytes per
parse and +2.6% time.

## Correctness and Tradeoffs

Values, errors, expectations and examined ranges are unchanged by design. The tests compare generated Go and
TypeScript parsers with the engine on a corpus of scan-site grammars, among them a value-free twin of a site that
builds a value and lookaheads over a site before the walks outside, check linear spends at growing sizes, and fail
on mutations that never reuse a run, answer only first positions, or shift the end of a run. The model test of the
table checks it against a list of runs under random and reverse-order inserts. The cost is the bitmap and runs that
a parse allocates once its first walks reach 16 characters, and the elements of runs that a TypeScript parser
copies.

Scan sites in rules that call other rules lower the cost of grammars that the controls of the complexity tests held
up as quadratic; the slope tests of the classifier now label the characters of their scans with `@error`, which
keeps a repetition from being a scan site or a run site, to keep their growth.

## Decision and Limits

Adopted on every target, always on. The classifier still credits the records of scan sites only in rules that call
no rule (scan-only rules of record 137); it is sound but does not credit the wider sites. A walk inside the
encoding of a character of a bytes-unit input is covered by the exactness lemma, not by the bound. Repetitions of
several characters or of single-character literals remain outside the sites.
