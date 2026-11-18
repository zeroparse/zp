# ZeroParse Flow Dialect Specification

The Flow dialect is the form of a rule that says how a parser runs it: the body of one rule variant as operations on the
state of a parse with structured control. A function at the Flow stage of [Code IR](../code-ir/README.md) has locals that
hold the saved state of backtracking points, the values of captures, intermediate values and flags, a body of the
base's structured control (labeled blocks and loops, branches, exits and assignments), and other steps that are calls of
the operations of the dialect, which match terminals, record expectations, call rules, build values, evaluate terms and
charge the budget. Action expressions are calls of the operations of the Action dialect (see
[Values](../exec-ir/values.md)), which are legal at the Flow stage too. A function holds no text of a target language,
so a backend has little to decide.

A module at the Flow stage is lowered from [Core IR](../core-ir/README.md); that lowering is not specified here, so this
specification does not say which functions a grammar produces. The module is converted in turn to the Exec stage (see
[Lowering From the Flow Stage](../exec-ir/lowering.md)), where each operation of the Flow dialect becomes a primitive,
and the backends print the rules of generated parsers from there. This specification defines the functions, the
operations of the dialect, the invariants that the verifier enforces, the text form and the passes of the compiler
pipeline that run at the Flow stage. The Flow dialect is internal to the ZeroParse engine and written for engine
maintainers and backend authors: it is not a public API, and nothing here changes how a grammar behaves, which the
[Grammar Language Specification](../language/README.md) and the [Parser and Runtime Specification](../runtime/README.md)
define.

This specification is authoritative for the operations, their immediate operands, the types and registers, and the
invariants. A change to the Flow dialect changes this specification in the same change: a test of the implementation
fails when an operation, its operands or immediate, a type or a register has no entry here, or when an entry names one
that the Flow dialect does not have.

## Contents

| | Chapter | Defines |
|--:|:--|:--|
| 1 | [Functions](functions.md) | Functions, variables, types, registers, labels and places, and local and framed functions |
| 2 | [Operations](operations.md) | Every operation of the Flow dialect and its contract |
| 3 | [Invariants](invariants.md) | What a well-formed function guarantees and what verification checks |
| 4 | [Text Form](text-form.md) | The printed form of a function |
| 5 | [Passes](passes.md) | The passes of the compiler pipeline that every lowered function goes through at the Flow stage |

The [Glossary](../glossary.md) lists the terms of the specifications.

## Conventions

The key words MUST, MUST NOT and MAY are used as in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) and only where
a requirement is normative. Positions, expectations, recovered errors, memoization, left recursion, budgets and
Documents follow the [Parser and Runtime Specification](../runtime/README.md); this specification names a behavior of
a parse and does not restate it. The operands of an operation are written with their names, such as start and x.
