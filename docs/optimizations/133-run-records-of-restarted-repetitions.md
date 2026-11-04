# 133. Run Records of Restarted Repetitions

- **Status**: Implemented
- **Date**: 2026-11-04

## Problem and Mechanism

A repetition that a caller walks again from the next position, as `a = "a"* "c"` does in `main = (a / "a")* $$`,
re-walks the same suffix at every position, so the cost of the enclosing scan grows with the square of the input.
The [design record](../design/records/067-repetition-run-records.md) and
[Run Sites](../../spec/runtime/parsing.md#run-sites) define the mechanism: an unbounded repetition that is
restartable (inside an iterating repetition, in a rule such a repetition calls, or in a rule on a call cycle) and
whose element can be replayed is a run site, and a walk of a hot site keeps records that a later walk reuses.

A site keeps nothing until its walks go back. It records the furthest position its walks of 64 units or more reached
and turns hot at the first walk that begins below that position and walks 64 units below it. A cold walk costs its
length, of which the part below the furthest position is under 64 units, so the cold cost is at most the input plus
64 per walk, however the walks alternate. Records are keyed by position and by the environment object.

## Applicability

| Execution profile | Applied / Partial / Not applied / Not applicable | Reason |
|:--|:--|:--|
| Closure, bytecode, iterative bytecode | Applied | `RunHot`, `RunWalked` and the `Skip*` walk operations of the engine core |
| Generated Go and TypeScript | Not applied | The generated runtimes keep no run records; their cost on a restarted repetition is the polynomial order |
| Documents | Not applied | Outside a lookahead, a site that the document can resume is resumed as before and keeps no run records; the records of the other parses of a document last for one parse |
| Streams | Applied | Records last for one parse, and those before a commit are dropped |
| Missing-token insertion | Not applied | A parse that inserts tokens keeps no records |

## Reproduction

Base `origin/main` (`f100b-base` worktree), Go 1.27.1, Apple M3 Max, benchlock. Test binaries of the `bench` package,
one per revision, ran interleaved for 12 rounds, `-test.benchtime=300ms -test.benchmem`, over `Parse` (codepoints) and
`Recognize` for every workload, `Incremental`, `IncrementalLong`, `Stream` and `RunSites`; benchstat compared them.
`BenchmarkRunSites` was copied into the base worktree untracked. Tests: `TestRunSitesMatchPlainRepetitions` and
`TestRunSitesMatchParsesWithoutRecords` at thresholds 64, 2 and 1, `TestRunSitesTurnHotAtTheEngineThreshold`,
`TestRunSitesNegativeControls`, `TestRunSitesAreRestartable`, `TestRunSitesBoundValueControls`,
`TestRunSiteTurnsHotOnRestart`, `TestAlternatingWalksTurnHot` and the E1, E7 and E9 rows of `TestControls`.

## Results

Steps of the closure backend on the restarted scans of the cookbook
([Rewrite a Grammar to Linear Cost](../cookbook/linear-rewrites.md)), `a` n times (the sizes the cookbook prints):

| Grammar | n = 100 | 200 | 400 | Before, n = 400 |
|:--|--:|--:|--:|--:|
| `scan.zp` | 3,078 | 6,178 | 12,378 | 325,888 |
| `look.zp` | 2,078 | 4,178 | 8,378 | 163,840 |

Time, bytes and allocations against the base (12 interleaved rounds):

| Group | Time | Bytes per op | Allocations per op |
|:--|--:|--:|--:|
| Geometric mean over all rows | +0.77% | +0.19% (all rows) | +0.75% |
| Parse, JSON, CSV, XML, Arith, Minilang, Recovery, Outline | 0% to +2.6% (worst: CSV iterative) | +0.00% to +0.03% | 0 to +2% |
| Recognize, JSON, CSV, XML, Arith, Minilang, Recovery | 0% to +2.9% (worst: CSV closure) | CSV +3.0% and +3.6% (about 0.5 KiB), Minilang closure +4.7% (noisy, +-16%), others below 0.7% | CSV, XML and Minilang closure +8% to +10%, others below 4% |
| Recognize, Outline | +3.1%, +5.1%, +2.6% (closure, bytecode, iterative) | +1.6% to +1.7% | +1.0% to +1.4% |
| Incremental, IncrementalLong | no significant change except iterative fresh +0.9% | at most +0.4% | at most +0.2% |
| Stream | no significant change except iterative +1.0% | +0.00% | +0.19% (closure) |
| RunSites (hot site) | closure 692 us, bytecode 737 us, iterative 774 us | 1.54 MiB | 60, 65, 70 |

Cold or hot in these inputs: JSON, CSV, XML, both Arith grammars, Minilang and Recovery keep every site cold in both
modes; Outline turns four sites hot in Recognize and none in Parse, which is where its larger time and allocation
change comes from. The base cannot run `RunSites`: with the default compute limit its parse fails
(`parse compute limit exceeded: limit 2048000`) on every backend, so the table has no base row for it.

The closure Recognize allocation deltas (+8% to +10% on CSV, XML and Minilang) are likely the refill of a pooled
buffer after a collection (the small table of cold sites is pooled with the parser); I did not verify that. Steady-state
allocations per parse through the root API on XML (a 1 KiB input, 5 warm-up parses, `testing.AllocsPerRun` over 20)
were equal to the base at this revision: closure 28 (value) and 16 (recognize), bytecode 42 and 30, iterative 89 and
71. Two copies of the same base binary differed by +0.6% on the geometric mean of the time in an 8-round check of an
earlier revision.

## Correctness and Tradeoffs

A record is reused only in the environment object it was made in. A grammar that rebinds a variable to the same new
value at every position builds a new environment each time and stays quadratic even when its site is hot
(`TestAlternatingWalksTurnHot`). Class scans, left recursion and the Document path keep no run records.

## Decision and Limits

Adopted on the closure and bytecode backends. Not covered: generated parsers, class scans, left-recursive restarts and
walks under environments that equal each other but are built separately.
