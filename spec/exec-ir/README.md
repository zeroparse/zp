# ZeroParse Exec Dialect Specification

The Exec dialect is the executable form of a parse in [Code IR](../code-ir/README.md): it specifies the
**abstract machine** that every ZeroParse runtime hosts, and the programs that drive it. It declares the abstract state
of a parse and the operations that change it, each with a contract; a module at the Exec stage holds functions in
structured code over those operations and over the operations of the Action dialect. Some functions are written once
for every runtime, the compositions; the others are the rules of a grammar, converted from the
[Flow stage](../flow-ir/README.md). A backend prints both as source code of its target language over its own
implementation of the operations.

This specification defines the state and handle types of the abstract machine, every operation with its contract, the
operations on the values of terms (the Action dialect), the compositions, the invariants that the verifier enforces
beyond those of Code IR, the passes, the conversion from the Flow stage, and the printing to Go and TypeScript. The
module, its functions and their text form are the base of [Code IR](../code-ir/README.md). It does not define how a
target represents the state of a parse or implements an operation: an operation states what happens, not how. The Exec
dialect is internal to the ZeroParse engine and written for engine maintainers and backend authors: it is not a public
API, and nothing here changes how a grammar behaves, which the [Grammar Language Specification](../language/README.md)
and the [Parser and Runtime Specification](../runtime/README.md) define.

This specification is authoritative for the declarations of the abstract machine, the contracts of the operations, the
registry of compositions and the invariants. A disagreement between an implementation and this specification is a bug,
fixed on whichever side is wrong. A change to the Exec dialect changes this specification in the same change: a test of
the implementation fails when an operation, a composition, a feature, a component, a register or a handle type has no
entry here, or when its entry declares a different signature, state, features or cost bound.

## Contents

| | Chapter | Defines |
|--:|:--|:--|
| 1 | [The Abstract Machine](machine.md) | Features, state components, registers, handle types and the quantities of cost bounds |
| 2 | [Operations](operations.md) | The operations on the state of a parse, with their contracts |
| 3 | [Values](values.md) | The Action dialect: the operations on the values of terms, with the tables that every target shares |
| 4 | [Compositions](compositions.md) | The functions written once for every runtime, and their specializations |
| 5 | [Invariants](invariants.md) | What a well-formed module at the Exec stage guarantees beyond Code IR |
| 6 | [Passes](passes.md) | The transformations of functions that keep their behavior |
| 7 | [Lowering From the Flow Stage](lowering.md) | How a function at the Flow stage, and a term of the action language, becomes a function at the Exec stage |
| 8 | [Printing](printing.md) | How a function becomes Go or TypeScript, the runtime protocol and the entries of the catalog tables |

The module, the statements and expressions of functions, their invariants and the text form are in
[Code IR](../code-ir/README.md). The [Glossary](../glossary.md) lists the terms of the specifications.

## Conventions

The key words MUST, MUST NOT and MAY are used as in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) and only where
a requirement is normative. Positions, expectations, recovered errors, memoization, left recursion, budgets and
Documents follow the [Parser and Runtime Specification](../runtime/README.md); this specification names a behavior
of a parse and does not restate it. Declarations are written in the [text form](../code-ir/text-form.md), in `text`
blocks and in backticks.
