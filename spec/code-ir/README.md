# ZeroParse Code IR Specification

Code IR is the form of the code that ZeroParse compiles a grammar to and that its runtimes are written in: a module
that declares state, types and operations, and functions in structured code over them. The base of Code IR defines
the module, the statements and expressions of functions and what running them does, the invariants that every module
satisfies and the text form. It does not give any operation a meaning: the operations belong to dialects, each of
which declares its operations with their contracts. A module is at a stage, which names the dialects whose operations
may appear in it.

| Stage | Dialects | Specified in |
|:--|:--|:--|
| Flow | Flow, Action | [Flow Dialect](../flow-ir/README.md), [Values](../exec-ir/values.md) |
| Exec | Exec, Action | [Exec Dialect](../exec-ir/README.md), [Values](../exec-ir/values.md) |

[Core IR](../core-ir/README.md) is lowered to Code IR at the Flow stage, which says how a parser runs each rule;
conversion from the Flow stage to the Exec stage replaces the operations of the Flow dialect by those of the
abstract machine, and the backends print the Exec stage as Go and TypeScript. Code IR is internal to the ZeroParse
engine and written for engine maintainers and backend authors: it is not a public API, and nothing here changes how a
grammar behaves, which the [Grammar Language Specification](../language/README.md) and the
[Parser and Runtime Specification](../runtime/README.md) define.

This specification is authoritative for the declarations of a module, the meaning of statements and expressions and
the invariants of every module. A disagreement between an implementation and this specification is a bug, fixed on
whichever side is wrong. A change to Code IR changes this specification in the same change: a test of the
implementation fails when a statement or expression kind has no entry here.

## Contents

| | Chapter | Defines |
|--:|:--|:--|
| 1 | [The Module](module.md) | Types, declarations, how an operation is declared, operation kinds, immediate operands, dialects and stages, costs |
| 2 | [Functions](functions.md) | Functions, statements and expressions, and what running them does |
| 3 | [Invariants](invariants.md) | What a well-formed module guarantees and what verification checks |
| 4 | [Text Form](text-form.md) | The printed form of a module |

The [Glossary](../glossary.md) lists the terms of the specifications.

## Conventions

The key words MUST, MUST NOT and MAY are used as in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) and only where
a requirement is normative. Positions, expectations, recovered errors, memoization, left recursion, budgets and
Documents follow the [Parser and Runtime Specification](../runtime/README.md); this specification names a behavior
of a parse and does not restate it. Declarations are written in the [text form](text-form.md), in `text` blocks
and in backticks.
