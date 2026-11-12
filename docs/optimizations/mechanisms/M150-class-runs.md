# M150: Class Runs

The classes of a keyword, written one after another, repeat a read, a range test and an
expectation record for each letter. Matching the run through one table of steps removes that
repetition.

## Mechanism and Invariants

The `class-runs` pass replaces every run of two or more consecutive classes in a value-free
sequence by one run that consumes one character of each class in turn. The run consumes the
same input and records the same expectations as the classes it replaces. It leaves alone the
first class of an alternative whose guard has already read the character (see
[M010](M010-first-character-dispatch.md)), and the inside of a terminal program.

## Scope and State

Default in typed Go parsers that have no budget controls and no Documents, because the pass is
sound only where reading and testing a character charges no budget. The pass never runs
elsewhere, whatever the flag says. The engine backends, generated Go Node parsers and
TypeScript keep the individual classes.

## Reference Control

`-no-opt class-runs` generates the same grammar with the classes one by one, from the same
generator. Over the five grammars where it changes the parser, parse time with the flag off is
1.005 times that of the default, within the noise of an identical copy, and `parser.go` is
1.024 times the size (record 136).

## Evidence and Implementation

Record 105 introduced the run as `ClassesSkip` over a table of steps and measures the generated
source of six language grammars 6% to 14% smaller together with large-class tables, and parse
time between -0.7% and +1.2% on the three grammars it timed. The numbers are for the grammars
of that record.

Evidence records: [105](../105-character-class-tables.md),
[136](../136-measured-effect-of-the-generation-flags.md).

Source owners: [`ClassRuns pass`](../../../internal/ir/coreir/passes.go),
[`ClassesSkip`](../../../runtime/class.go).

The [comparison policy](../../design/records/024-optimization-mechanism-controls.md)
distinguishes same-source controls, historical changes and full checkpoints.

Related mechanisms: [M020](M020-character-class-tests.md), [M140](M140-terminal-programs.md), [M160](M160-class-tables.md).

[Optimization Catalog](../README.md)
