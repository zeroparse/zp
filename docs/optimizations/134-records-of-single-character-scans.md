# 134. Records of Single-Character Scans

- **Status**: Implemented
- **Date**: 2026-11-07

## Problem and Mechanism

A rule that matches input and calls no rule, such as a run of blanks, is not memoized, so every failed alternative
that leads back to its start walks the run again. A [run site](133-run-records-of-restarted-repetitions.md) does not
cover it, since it does not repeat a single character. A parse of nested conditionals followed by a long run of blanks
cost quadratic steps.
[Scan Sites](../../spec/runtime/parsing.md#scan-sites) define the mechanism: an unbounded repetition of a character
class or of any character in a pure scan rule (called from two places, no attribute but `@start`, no type, no action,
no called rule, no capture, predicate or lookahead) is a scan site. A walk matches its first 16 characters
(`core.ScanHotSpan`, separate from the 64 of run sites) as an ordinary repetition. At a hot site the walk then looks
for the record of its start. On a hit it moves to the recorded end, takes the count and the elements, and examines
the recorded range. Otherwise it goes on and records itself, only if the input it examined is available. A site is
hot by the rule of run sites, at 16 characters. The charge is one step for the lookup, one for the slot of a record
that is found and one for a record that is made; the characters a record skips cost nothing.
[Lemma 4](../../spec/proofs/run-sites.md#scan-sites) shows each record exact.

## Applicability

| Execution profile | Applied / Partial / Not applied / Not applicable | Reason |
|:--|:--|:--|
| Closure, bytecode, iterative bytecode | Applied | `ScanRepeat` and `scanSiteList` of the closure builder and `SCANSITE` and `ITER` of the VMs, over the operations of `core/scan_sites.go`; always on |
| Generated Go and TypeScript | Not applied | The generated runtimes keep no records of scan sites |
| Documents | Not applied | A repetition that a Document resumes neither looks up nor makes records; the records of the other parses last for one parse |
| Streams | Applied | A record is made only for input that is loaded |
| Missing-token insertion | Not applied | A parse that inserts tokens keeps no records |

## Reproduction

Base: the revision before the change, Go 1.27.1, Apple M3 Max, serialized with benchlock, test binaries of the `bench`
package (one per variant), `BenchmarkParse` (codepoints), `BenchmarkRecognize`, `BenchmarkIncremental`,
`BenchmarkStream` and an untracked `BenchmarkO91` with `ws15`, `ws31`, `ws40` (comma lists with blanks of that
length between the items) and `longstr`; variants ran interleaved, medians of the ratios to the base. Rounds: 10
over 46 rows (A), 20 with separate binaries (B), and 20 with one binary and a switch that turns the marking off
(C). A last check compared the final revision with the revision before the review fixes over 10 rounds. Tests:
`internal/engine/scan_sites_test.go` and the linear controls of `internal/complexitytest`.

## Results

Steps of a parse of the probe `(ws "a" / ws "b" / ws "c" / ws "d" / ws "e")*` with `ws = [ ]*` over a run of blanks and
one `e`, with scan sites and without:

| Run | Closure | Bytecode | Iterative |
|:--|--:|--:|--:|
| 16 | 374 / 432 | 347 / 373 | 327 / 353 |
| 20 | 430 / 520 | 391 / 441 | 371 / 421 |

Twelve negative controls of the complexity table are linear on all three backends; the prefix loop of an Operators
Rule stays quadratic (not a scan site).

Time against the base (medians):

| Set | Result |
|:--|:--|
| A, 46 rows | geometric mean +0.3% (+0.2% with scan sites off), B/op +0.0%. Above +1%: ws15, ws31, ws40 parse and recognize (+1.0% to +3.8%; +0.2% to +1.6% with the marking off), Arith_LeftRec closure recognize (+1.1%; -0.2% off), XML iterative (+0.8%) |
| B, 20 rounds | ws31 parse +1.1% (off -0.2%), ws31 recognize +1.4% (+0.1%), ws40 parse +1.0% (-0.1%), ws40 recognize +0.5% (-0.9%), Arith_LeftRec closure recognize +0.9% (+0.2%), longstr about 0 |
| C, same binary | ws31 parse +1.6% on against 0.0% off, ws31 recognize +1.8% against +1.1%, ws40 parse +1.3% against +1.0%, ws40 recognize +1.8% against +1.0%, Arith_LeftRec recognize +0.5% against -0.3% |
| Final revision against the revision before the review fixes, 18 rows | geometric mean -0.2%; each row within -1.0% and +0.7%; B/op and allocations unchanged |

## Correctness and Tradeoffs

The walks past 16 characters on the rows above cost 0.3% to 1.8% more time. That is of the order of the three extra
steps and the record of each walk, but I did not profile it, so the cause is not established. Scan sites stay always on
as a decision, with these rows accepted as measured. Unchanged by design: values, errors, expectations and examined
ranges, compared against parses without records on the three backends in characters and bytes. A record is shared
between lookaheads and the code outside them. The elements of a record are never changed after it is made.

## Decision and Limits

Adopted on the closure and bytecode backends, always on. Not covered: generated parsers; a repetition that a Document
resumes; the bound for walks that start at different positions of one run is a draft (Lemma 5).
