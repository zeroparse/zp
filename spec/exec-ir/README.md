# ZeroParse Exec IR Specification

Exec IR is the executable form of a parse: the specification of the **parser VM**, the abstract machine that every
ZeroParse runtime hosts, and the programs that drive it. It declares the abstract state of a parse and the operations
that change it, each with a contract, and it holds functions in structured code over those operations. Some functions
are written once for every runtime, the compositions; the others are the rules of a grammar, lowered from
[Flow IR](../flow-ir/README.md). A backend prints both as source code of its target language over its own
implementation of the operations.

This specification defines the module of the parser VM, the types, statements and expressions of functions and what
running them does, every operation with its contract, the compositions, the invariants that the verifier enforces, the
passes, the lowering from Flow IR, the printing to Go and TypeScript, and the text form. It does not define how a
target represents the state of a parse or implements an operation: an operation states what happens, not how. Exec IR is
internal to the ZeroParse engine and written for engine maintainers and backend authors: it is not a public API, and
nothing here changes how a grammar behaves, which the [Grammar Language Specification](../language/README.md) and the
[Parser and Runtime Specification](../runtime/README.md) define.

This specification is authoritative for the declarations of the parser VM, the meaning of statements and expressions,
the contracts of the operations, the registry of compositions and the invariants. A disagreement between an
implementation and this specification is a bug, fixed on whichever side is wrong. A change to Exec IR changes this
specification in the same change: a test of the implementation fails when an operation, a composition, a statement or
an expression kind, a feature, a component, a register or a handle type has no entry here, or when its entry declares
a different signature, state, features or cost bound.

## Contents

| | Chapter | Defines |
|--:|:--|:--|
| 1 | [The Parser VM](machine.md) | Types, features, state components, registers, handle types and how an operation is declared |
| 2 | [Functions](functions.md) | Functions, statements and expressions, and what running them does |
| 3 | [Operations](operations.md) | The operations on the state of a parse, with their contracts |
| 4 | [Values](values.md) | The operations on the values of terms, with the tables that every target shares |
| 5 | [Compositions](compositions.md) | The functions written once for every runtime, and their specializations |
| 6 | [Invariants](invariants.md) | What a well-formed module guarantees and what verification checks |
| 7 | [Passes](passes.md) | The transformations of functions that keep their behavior |
| 8 | [Lowering From Flow IR](lowering.md) | How the function of a rule variant, and a term of the action language, becomes a function of Exec IR |
| 9 | [Printing](printing.md) | How a function becomes Go or TypeScript, and the runtime protocol |
| 10 | [Text Form](text-form.md) | The printed form of a module |

The [Glossary](../glossary.md) lists the terms of the specifications.

## Conventions

The key words MUST, MUST NOT and MAY are used as in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) and only where
a requirement is normative. Positions, expectations, recovered errors, memoization, left recursion, budgets and
Documents follow the [Parser and Runtime Specification](../runtime/README.md); this specification names a behavior
of a parse and does not restate it. Declarations are written in the [text form](text-form.md), in `text` blocks
and in backticks.
