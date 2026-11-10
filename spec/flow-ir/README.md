# ZeroParse Flow IR Specification

Flow IR is the form of a rule that says how a parser runs it: the body of one rule variant as operations on the state
of a parse with structured control. A function of Flow IR has variables, which hold the saved state of backtracking
points, the values of captures, intermediate values and flags, and a body of statements: labeled blocks and loops,
branches, exits and assignments, and primitives that match terminals, record expectations, call rules, build values,
evaluate terms and charge the budget. It holds no text of a target language, so a backend has little to decide.

Flow IR is lowered from [Core IR](../core-ir/README.md); that lowering is not specified here, so this specification
does not say which functions a grammar produces. Flow IR is lowered in turn to [Exec IR](../exec-ir/README.md) (see
[Lowering From Flow IR](../exec-ir/lowering.md)), from which the backends print the rules of generated parsers.
This specification defines the functions, their statements and expressions, the invariants that the verifier enforces
and the text form. Flow IR is internal to the ZeroParse engine and written for engine maintainers and backend authors:
it is not a public API, and nothing here changes how a grammar behaves, which the
[Grammar Language Specification](../language/README.md) and the [Parser and Runtime Specification](../runtime/README.md)
define.

This specification is authoritative for the kinds of statements and expressions, the types and registers, and the
invariants. A change to Flow IR changes this specification in the same change: a test of the implementation fails when
a statement or an expression kind, a type or a register has no entry here, or when an entry names one that Flow IR
does not have.

## Contents

| | Chapter | Defines |
|--:|:--|:--|
| 1 | [Functions](functions.md) | Functions, variables, types, registers, labels and places, and local and framed functions |
| 2 | [Statements](statements.md) | Every kind of statement and what it does |
| 3 | [Expressions](expressions.md) | Every kind of expression and what it gives |
| 4 | [Invariants](invariants.md) | What a well-formed function guarantees and what verification checks |
| 5 | [Text Form](text-form.md) | The printed form of a function |

The [Glossary](../glossary.md) lists the terms of the specifications.

## Conventions

The key words MUST, MUST NOT and MAY are used as in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) and only where
a requirement is normative. Positions, expectations, recovered errors, memoization, left recursion, budgets and
Documents follow the [Parser and Runtime Specification](../runtime/README.md); this specification names a behavior of
a parse and does not restate it. The fields of a statement or an expression are written with their names, such as
Label and Body.
