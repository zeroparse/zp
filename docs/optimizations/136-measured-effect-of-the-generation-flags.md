# 136. Measured Effect of the Generation Flags

- **Status**: Implemented
- **Date**: 2026-11-08

## Problem and Mechanism

The optimizations of the generator are flags (design 063), and a preset (`speed`, `memory`, `size`) fixes their values.
A flag or a preset belongs in a profile only if a measurement shows what it does. This record is that measurement for
every flag of the registry, and the rule that turned it into the profiles.

The rule. Each flag was turned off (or set to other values), one at a time, against the `speed` profile, and the
generated Go parsers of the ten language grammars were measured beside an identical copy of `speed` (the control), whose
ratio is the noise of the machine.

- A flag is **effective** if it improves the parse time, or the metric it exists for, by at least 1% in the geometric
  mean beyond the control.
- An effective flag is on in `speed` if it improves at least six of the ten parsers, costs at most 3% of the geometric
  mean parse time, and slows no parser by more than 6%. A flag with no measured cost in time, allocations, memory or
  source size is on in `speed` as well.
- Otherwise it is an opt-in. A flag that changes the output is an opt-in in any case and is in no profile.
- A profile is kept if it improves the metric it is named for by that rule against `speed`.

## Reproduction

The measurements were taken on main at `f3df70413`, before later changes to main, with the profile registry added to it. Go 1.27.1, Apple M3 Max, under the benchmark lock. `go run ./bench/profilematrix`
(design: `docs/performance.md`) generated each grammar once per variant with the options of
`testdata/grammars/generate.txt`, built a program for size and one for measuring, and ran the measuring programs in fresh
processes in rotated order for 10 rounds with `-benchtime 300ms`; time is the minimum over the rounds. The rows are the
ten grammars of `testdata/grammars` (`json`, `csv`, `xml`, `yaml`, `cel`, `cue`, `golang`, `python`, `typescript`,
`duckdb`) and two use cases of the CSV grammar, `csv-incremental` (an edit in the middle of 50000 records followed by a
reparse of a `Document`) and `csv-stream` (the file as a stream of records). Variants were run in four chunks of at most
nine programs each, every chunk with its own baseline and control. A variant whose generated Go parser is byte-identical to
the baseline's for a grammar was not measured for it: a flag counts only on the grammars where it changes the parser.

The control's time ratio was between 0.993 and 1.011 for the ten grammars in every chunk. The `csv-incremental` row is
noisier (the control reached 1.057 in one chunk), so no decision rests on it.

## Results

Turning the flag off, or setting it to another value, as the ratio to `speed` (above 1 is larger or slower). Geometric
mean over the grammars where the parser changes (`n` of 10); the control in parentheses is the control's geometric mean over
the same grammars.

| Flag | n | Parse time | `parser.go` | Other | Parsers faster with the flag |
|:--|--:|--:|--:|:--|--:|
| `guards` | 10 | 1.091 (1.002) | 0.917 | `parser.ts` 0.902 | 10 |
| `scans` | 10 | 1.029 (1.002) | 1.016 | `parser.ts` 1.017 | 6 |
| `inline-literals` 0 | 10 | 1.016 (1.001) | 0.930 | binary 0.990 | 8 |
| `inline-literals` 2 | 9 | 1.005 (1.001) | 0.988 | | 3 |
| `inline-literals` 8 | 7 | 1.003 (1.002) | 1.018 | | 1 |
| `terminals` | 5 | 1.007 (1.003) | 1.036 | | 2 |
| `guarded-classes` | 6 | 1.005 (1.003) | 1.004 | | 1 |
| `class-runs` | 5 | 1.005 (1.004) | 1.024 | | 1 |
| `class-tables` 0 | 6 | 0.988 (0.999) | 1.034 | | 0 |
| `class-tables` 3 | 6 | 1.012 (1.003) | 0.997 | | 2 |
| `class-tables` 8 | 6 | 0.992 (0.999) | 1.001 | | 0 |
| `literal-sharing` | 10 | 1.001 (1.001) | 1.024 | | 0 |
| `method-sharing` | 7 | 0.996 (1.000) | 1.006 | `parser.ts` 1.406, gzip 1.216 | 0 |
| `typed-object-slabs` | 10 | 0.999 (1.000) | 1.001 | allocations 3.31, bytes 0.946, live heap 0.946 | 3 |
| `typed-list-slabs` | 10 | 0.997 (1.000) | 1.001 | allocations 2.81, bytes 0.913, live heap 0.913 | 2 |
| `typed-output-bootstrap` | 10 | 1.005 (1.000) | 1.001 | allocations 0.801, bytes 1.129, live heap 1.229 (1.005) | 4 |
| `ts-literal-sharing` on | | | | `parser.ts` 1.009, gzip 1.000 | |

`text-dispatch`, `field-compare` and `sparse-memo` changed no generated parser of the ten grammars, in Go or TypeScript. The
`class-tables` rows were measured again in a second run of the six typed grammars (10 rounds): off 0.988 against a control of
0.999, with `typescript` at 0.963, `golang` at 0.982 and the others within 0.3%.

The incremental and stream rows of the CSV grammar: `guards` off 0.953 and 1.014, `scans` off 1.106 and 1.010,
`inline-literals` off 0.910 and 0.995 (time of one edit and reparse, then of the whole stream). The controls of the
chunks were 1.020 and 1.005 (first) and 1.057 and 1.004 (second). The incremental row moved by 5% to 11% in both directions,
by more than its control, with no pattern the other rows share, and the stream row stayed within 1.4%; no decision uses
these two rows.

## Decisions

- `guards` and `scans` are effective by time (9.1% and 2.9%) and stay on. `guards` costs 9% of source size, which is not a
  time cost.
- `inline-literals` at 4 is effective (1.6%, eight parsers); the values 2 and 8 were not better than 4 by a margin beyond
  the control, so 4 stays and 6 and 12 were not tried. It costs 7.5% of source size, and `yaml` and `golang` are 1.7% slower
  with it.
- `terminals`, `guarded-classes` and `class-runs` change time by 0.4% or less beyond the control, shrink the source and
  cost nothing measured, so they stay on as zero-cost.
- `class-tables` costs 1.1% of the parse time beyond the control (3.7% on `typescript`), shrinks the source by 3.4% and does
  so for five of the six typed grammars, so it fails the six-of-ten rule and is an opt-in (`-opt class-tables=5`), a change
  from design 105. Its other values were not better.
- `method-sharing` and `literal-sharing` cost nothing in time and make Go source smaller; `method-sharing` makes the
  TypeScript source 29% smaller. They stay on.
- `typed-object-slabs` and `typed-list-slabs` change no parse time. The flags cut the number of allocations to a third, and
  their price is 6% and 10% more bytes and live heap. They stay on, because the rule counts allocations as an effect. The
  `memory` profile turns them off.
- `typed-output-bootstrap` cuts the live heap by 19% and the bytes allocated by 11%, and costs 25% more allocations.
- `ts-literal-sharing` and `sparse-memo` act on TypeScript parse time and memory, which the harness does not measure; the
  size result for `ts-literal-sharing` is flat, and design 090 measured its time. They are unchanged.
- No flag is removed. `text-dispatch` and `field-compare` showed no effect because none of the ten grammars has the form
  they act on, so this measurement cannot show that they are effective.

## Profiles

Variants of the profiles were measured the same way, in one chunk (control 1.000). The ratios are to `speed`, the
geometric mean over the ten language grammars; the worst parser is the one with the largest parse-time ratio.

| Candidate | Sets | Parse time | Bytes allocated | Live heap | `parser.go` | Binary | Worst parser |
|:--|:--|--:|--:|--:|--:|--:|--:|
| `memory` | slabs off | 0.996 | 0.886 | 0.848 | 1.000 | 0.984 | +3.8% (`xml`) |
| `size` | `inline-literals` 0, `class-tables` 5 | 1.022 | 1.001 | 0.997 | 0.912 | 0.986 | +5.5% (`json`) |
| not registered | `size` and `guards`, `scans`, `guarded-classes` off | 1.137 | 0.997 | 1.010 | 0.846 | 0.971 | +29% (`duckdb`) |

`memory` and `size` meet the rule (`memory` cuts the bytes allocated by more than 1% on nine of ten grammars, `size`
shrinks `parser.go` on all ten; parse time stays within 3% as a geometric mean; no grammar is more than 6% slower). The
third candidate costs 14% of parse time for about 6 points more source reduction than `size` and is dropped. The
`memory` profile raises the allocation count 1.03 to 1.39 times on the typed grammars and 52 to 97 times on the JSON, CSV
and XML parses (the number of allocations per parse of those grammars is small); the geometric mean over the ten is 4.06.

The two use rows of the CSV grammar are not part of these means and no decision uses them. Time of one edit and reparse
of a `Document` (`csv-incremental`, control 0.968): `memory` 0.980, `size` 0.987. Time of the whole stream (`csv-stream`,
control 1.001): `memory` 0.996, `size` 0.995. The incremental row is the noisy one (see Reproduction).

## Runtime Parameters

The chunk sizes of the typed slabs (objects 8 to 256, lists 64 to 1,024, lists of more than 256 on their own) were tried at
64 and 256 and at 1,024 and 4,096 in a copy of the runtime, one run each over the ten grammars. Bytes allocated and live heap
did not change by 1.5% in the geometric mean (0.987 to 1.015); the allocation count moved only for `json`, `csv` and `xml`
(2.1, 1.7 and 1.7 times with the smaller limits, 0.77, 0.83 and 0.85 with the larger); parse time differed by -0.7% and +1.0%
between runs that were not interleaved, within what the control allows. No gain beyond the control appeared, so no runtime
parameter is exposed as an optimization.

## Limits

- The harness measures Go parsers. TypeScript was measured by generated size only.
- Time is the minimum of 10 rounds on one machine; ratios within 1% of the control are not distinguished from it.
- The live heap of a single run varies by 5% to 10% between processes on a few grammars (the control showed it); only
  differences above that are read.
- The numbers are for the ten language grammars and the inputs of `testdata/grammars`; a grammar with other shapes can
  differ, which is why every flag stays selectable.
