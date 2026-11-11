# 137. Scan Records on Every Target

- **Status**: Implemented
- **Date**: 2026-11-12

## Problem and Mechanism

[Records of single-character scans](134-records-of-single-character-scans.md) answer the rescans of a run of one
character class, but only where the scan stood in a rule called from two places with no capture, action, attribute or
predicate, and only on the engine backends. A scan of blanks under a capture, a scan in a rule called once, or any scan
in a generated parser was walked again by every failed alternative that led back to its start. Of 25 small grammars,
each a nesting rule whose levels fail and rescan a run of blanks once per level, 20 cost quadratic steps on the engine
and 24 took quadratic time in generated Go and TypeScript parsers.

Two changes close this.

- **Every one-character scan of a rule that calls no rule is a site.** [Scan Sites](../../spec/runtime/parsing.md#scan-sites)
  take every unbounded repetition of a character class or of any character in a rule that calls no rule, other than
  the site of a skip run or a `#stream` repetition, whatever captures, actions and attributes surround it. An iteration
  tests one character and nothing else, so the walk from a start ends at a position, matches a number of characters,
  builds elements and examines a range that depend on the start and the input alone; a capture or an action reads only
  the list that the walk gives. [Lemma 4](../../spec/proofs/run-sites.md#lemma-4-exactness-of-scan-records) shows
  these records exact, and [Lemma 6](../../spec/proofs/run-sites.md#lemma-6-walks-again-from-one-start) bounds the
  walks again from one start. The cost
  classifier treats a call of a **scan-only rule** (one that reads no variable, calls no rule, has no `#recover`,
  Operators Rule or `#stream`, and whose every unbounded repetition is a scan site with no copying operation around
  it) as a call whose rescans the records answer, and accepts such grammars as linear
  ([linear-cost proof](../../spec/proofs/linear-cost.md)).
- **Generated parsers keep the same records.** The Core IR marks a scan site on its repetition, and the Flow IR lowers
  it to a counted loop that, at the hot span of 16 characters, looks up the record of its start, and at the end either
  makes the record or notes the walk for the hot test. The generated Go runtime (package `runtime`) and the TypeScript
  runtime hold the records, with the same hot span and the same charges as the engine: one step for a lookup, one for a
  record that is found and one for a record that is made. Generated Go parsers require version 7 of the runtime
  protocol.

## Applicability

| Execution profile | Applied / Partial / Not applied / Not applicable | Reason |
|:--|:--|:--|
| Closure, bytecode, iterative bytecode | Applied | Every scan of a rule that calls no rule is marked; always on |
| Generated Go and TypeScript | Applied | The same records, hot span and charges as the engine, in the generated runtimes; always on |
| Documents, closure, bytecode, iterative bytecode | Not applied | A repetition that a Document resumes neither looks up nor makes records; the records of the other parses last for one parse |
| Documents, generated Go | Applied | A generated parser resumes no repetition, so its walks look up and make records as in a plain parse; the records last for one parse |
| Documents, generated TypeScript | Not applicable | TypeScript parsers do not expose Documents |
| Streams | Applied | A record is made only for input that is loaded |
| Missing-token insertion | Not applied | A parse that inserts tokens keeps no records |

## Reproduction

Base: the revision before the change; candidate: the commits of the change and their test updates. Go 1.27.1,
Node 24.13 for TypeScript, Apple M3 Max, every run serialized with the benchmark lock of the machine; the load average
was noted before and after each run, and a run whose A/A rows spread by more than 1.5% was discarded.

- Growth: the 25 grammars of the problem and 13 controls, each a grammar with an input
  `prefix × n + middle + suffix × n`. On the engine backends the steps of a parse at three doubling sizes give the
  exponent of each doubling. Generated Go and TypeScript parsers were generated from the same grammars and timed (the
  best of repeated parses) at n = 1000, 2000, 4000 for Go (the median of three runs) and n = 500, 1000, 2000 for
  TypeScript; a few Go parsers were also timed at n = 4000 and 16000 with the garbage collector off.
- Time: test binaries of the `bench` package for the base, an identical copy of the base (A/A) and the candidate,
  run in rotated order for 20 rounds over `BenchmarkParse` (code points and generated AST), `BenchmarkRecognize`,
  `BenchmarkTerms`, `BenchmarkIncremental` and `BenchmarkStream`, 99 rows at 200 ms each; medians of the per-round
  ratios to the base. Two such runs were made; the first also rotated in the variant described under Decision.
- Memory: `bench/profilematrix -variant speed= -control -rounds 8`, run once with the base tree and once with the
  candidate tree.

## Results

Growth exponents per doubling (1 is linear, 2 quadratic):

| Cases | Engine (closure, bytecode, iterative), base | Engine, candidate | Generated Go, base | Generated Go, candidate | TypeScript, base | TypeScript, candidate |
|:--|:--|:--|:--|:--|:--|:--|
| 20 grammars, quadratic on the engine | 1.98–2.00 | 1.00 | 1.90–2.33 | 0.99–1.73 | 1.62–2.56 | 0.98–1.03 |
| 4 grammars, linear on the engine | 1.00 | 1.00 | 1.97–2.34 | 1.07–1.57 | 2.14–2.48 | 0.98–1.02 |
| 1 grammar whose input fails at its first character | 0.98–0.99 | 0.98–0.99 | 0.92–0.96 | 0.90–0.97 | 0.76–0.80 | 0.78–0.82 |

The engine counts steps, so its exponents are exact; the generated parsers were timed, and their exponents carry the
noise of runs of one to seven milliseconds, most of it from the garbage collector: the three Go parsers with the
highest exponents gave 1.02–1.05 from n = 4000 to n = 16000 with the collector off, where the base gave 1.97–2.00 at
the smaller sizes. The classifier accepts 20 of the 25 as linear (none at the base); five stay unknown: accepting them
needs the calls of scan-only rules counted in the bound for bounded scans, which the linear-cost proof does not cover
yet. Of the controls, eight grammars measure linear everywhere; a scan followed by a
call in the same rule and an Operators Rule in place of the scan stay quadratic on every target (not sites);
repetitions of a choice of literals or of a two-character literal are linear on the engine but quadratic in generated
parsers (1.96–2.45); a scan whose text an action concatenates grows with exponent 1.90–1.95 on the engine, by the
copy, 1.16–1.33 in generated Go and 1.01 in TypeScript.

The triage of the language grammars follows: the whitespace before nested conditionals in cel, the nested operators
before long whitespace in calc and cue, and the comment text of minilang in generated Go measure linear.

Time against the base, 99 rows, two runs of 20 rounds:

| Set | First run | Second run |
|:--|:--|:--|
| Geometric mean | +0.12% (A/A −0.02%) | +0.02% (A/A −0.03%) |
| Largest A/A row | ±0.99% | ±0.61% |
| Rows beyond the A/A spread, slower | `Terms` TermsHeavy iterative +1.5% (its A/A row +1.0%), TermsHeavy closure +1.3%, TermsHeavy bytecode +1.1%; `Recognize` CSV closure +1.3%, Arith_OperatorsRule generated +1.2%, XML bytecode +1.1%, Arith_LeftRec bytecode +1.1% | 23 rows from +0.6% to +1.1%, the largest `Parse` Outline closure +1.1%, `Terms` TermsHeavy iterative +1.0%, `Recognize` XML bytecode +1.0%, CSV closure +1.0% |
| Rows beyond the A/A spread, faster | 5, from −1.2% to −5.0% (`Terms` TermsHeavy generated recognizer) | 11, from −0.6% to −5.1% (the same row) |
| Incremental and Stream | within the spread except `Incremental` generated fresh −1.6% | within the spread except `Incremental` generated fresh −1.6% and two document rows −0.7% |

Five slower rows exceed the spread in both runs: CSV closure and XML bytecode recognition, Arith_OperatorsRule
generated recognition, and TermsHeavy on the closure and iterative backends, all by at most 1.5%. JSON recognition in a generated parser, a parser
whose scans are short, is +0.2% and +0.05%.

Memory of the generated Go parsers of the ten language grammars (bytes per pass and live heap of the results):

| Grammar | Bytes per pass | Retained |
|:--|:--|:--|
| golang | 58.4 KB → 70.4 KB (+20.6%) | unchanged |
| yaml | 68.8 KB → 73.7 KB (+7.1%) | 40.8 KB → 35.5 KB (−13%) |
| cue | 53.3 KB → 56.4 KB (+5.8%) | within the noise of the control copies (up to ±10%) |
| the other seven | within −1.4% and +0.2% | within the noise of the control copies |

Peak resident set sizes are within −0.9% and +2.1% (the control copies vary by up to 4.5%); the CSV incremental and
stream rows are unchanged. The added bytes per pass come from the per-site hot state and the record
tables that a parse allocates when its first walks reach the hot span. The memory of a parse grows with the number of
records, that is, with the number of starts of walks that reach the hot span: O(records), not a fixed amount; a
grammar whose walks start at many positions of long runs holds one record, with its elements when the walk builds a
value, for each. Generated Go sources grow by 0.0% to 6.7%
(csv), binaries by at most 0.7%.

## Correctness and Tradeoffs

Values, errors, expectations and examined ranges are unchanged by design; the tests compare generated Go and TypeScript
parsers with the engine on a corpus of scan-site grammars in nodes and typed values, check linear spends at growing
sizes, and fail on mutations that never reuse a record (quadratic) or shift its start (wrong result). A grammar whose
cubic cost comes from an inner scan stays within the default budget once the scan is a site, so the budget tests use
a scan that is not a site. The cost is at most 1.5% of time on a few rows, and memory that grows with the number of records (a few kilobytes per parse on the grammars measured).

## Decision and Limits

Adopted on every target, always on, with the single counted loop described above. Alternatives measured and rejected:

- Memoizing every rule that calls no rule and scans makes the same grammars linear with no classifier change, but in
  eight rounds it cost +13% to +33% of time on the engine backends, +24% to +68% on generated parsers, up to +122% on
  recognition, and up to 16 times the bytes per recognition (JSON on the closure backend).
- Two loops chosen when a walk begins, a plain loop at sites that are not hot and the counted loop at hot ones, with
  the hot sites as bits: in the first run above it gave a geometric mean of +0.19% against +0.12% for the counted
  loop, and slower rows where sites turn hot (`Recognize` Arith_OperatorsRule generated +3.2%, Outline generated
  +2.0%; `Parse` Outline and Arith_OperatorsRule generated AST +1.6%), because each walk pays the hot test and the end
  test. Keeping the hot state of each site in a map, in an earlier version of that variant, cost up to +5.8% on
  Outline recognition, by a lookup per walk.
- A separate loop for the first 16 characters keeps a count in each iteration and adds a second loop per site to the
  generated code; the variant above removes the count entirely and still does not gain, so this one was not built.

Not covered: repetitions of several characters or of single-character literals (`"x"*`, `(" " / "!")*`), which are
not scan sites; the skip and leaf scans of an Operators Rule, which measure linear but which the classifier does not
exempt; five of the 25 grammars, which measure linear but which the classifier leaves unknown.
