# M140: Terminal Programs

A value-free choice made of many literals and classes, such as a keyword list spelled as a
trie, can run as one table-driven program instead of straight-line code per alternative.

## Mechanism and Invariants

The `terminals` pass marks the outermost value-free expression of a rule that is made only of
literals, classes, sequences, choices, options, negative lookaheads, discards and atomic groups,
contains a choice and has at least 16 literals and classes. The generator lowers a marked
expression to a `TerminalProgram` that `SkipTerminal` interprets. The program consumes the
same input and records the same expectations at the same positions as the straight-line code.
A choice with at least four guarded alternatives gets an ASCII dispatch table that is built once
from the same class tests that the guards use.

## Scope and State

Default in typed Go parsers that have no budget controls, no Documents and no observability,
for rules lowered to Go-local code. Case-insensitive literals, frame bodies, the engine
backends, generated Go Node parsers and TypeScript keep their ordinary form.

## Reference Control

`-no-opt terminals` generates the same grammar with straight-line code, from the same
generator. Over the five grammars where it changes the parser, parse time with the flag off is
1.007 times that of the default, with the same noise as an identical copy, and `parser.go` is
1.036 times the size (record 136).

## Evidence and Implementation

Record 111 measures generated source 8.5% smaller for the DuckDB grammar and parse speed
within the noise of an identical copy. The result concerns parsers that contain keyword
tries, not other grammars.

Evidence records: [111](../111-terminal-programs.md),
[136](../136-measured-effect-of-the-generation-flags.md).

Source owners: [`Terminals pass`](../../../internal/ir/core/passes.go),
[`TerminalProgram`](../../../codegen/go/lib/terminal.go).

The [comparison policy](../../design/records/024-optimization-mechanism-controls.md)
distinguishes same-source controls, historical changes and full checkpoints.

Related mechanisms: [M010](M010-first-character-dispatch.md), [M150](M150-class-runs.md).

[Optimization Catalog](../README.md)
