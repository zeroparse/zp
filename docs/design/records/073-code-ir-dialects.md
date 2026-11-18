# 073. Code IR Dialects and the Conversion Between Stages

- **Status**: Accepted
- **Author**: @ornew
- **Date**: 2026-11-19

## Summary

Code IR is one base, a module of types, state, operations and functions, and its operations belong to dialects. A module
is at a stage that lists the dialects it may use. Grammar code is at the Flow stage in the Flow dialect and the Action
dialect, and at the Exec stage in the Exec dialect and the Action dialect. Action expressions are operations of the
Action dialect from the start, so the conversion between the stages never touches them. The conversion from the Flow
stage to the Exec stage is complete and keyed on primitives: each Flow operation becomes the primitive that stands for
it, and verification at the Exec stage proves that no Flow operation is left. This record decides these points and the
specification split that follows. [Record 072](072-release-package-layout.md) decided that one Code IR has three
dialects; this record decides how they are defined and connected.

## Motivation

Before this change the Flow stage was its own structure with its own printer and verifier, and its operations were
described by tables that no declaration checked. Action expressions were lowered straight to functions of the Exec stage,
and the conversion from Flow to Exec turned every operation into one transitional operation with the Flow operation as
its immediate operand. Three things followed. The same function was written in two structures. A reader could not tell
from a module which operations it could hold, and an operation could appear at a stage that did not define it. Operations
that were transitional only because the conversion needed a placeholder hid the few that really are transitional.

## Decisions

1. **Flow and Action are dialects of the shared Code IR.** The Flow dialect declares the operations that say how a parser
   runs a rule: matching, backtracking, expectations, calls, values and charges. The Action dialect declares the
   operations that evaluate action expressions, and is legal at both stages. Both are declared with the same operation
   declarations as the Exec dialect, so one verifier, printer and interpreter serve all of them.
2. **A module has a stage, and a stage lists dialects.** The Flow stage allows the Flow and Action dialects, the Exec
   stage the Exec and Action dialects. Verification rejects an operation of a dialect that the stage of the module does not
   allow. The postcondition of a conversion is therefore checked by verifying its result.
3. **`execlower` is a full conversion keyed on primitives.** Each call of a Flow operation becomes a call of the primitive
   `runtime.` followed by its name, of the kind primitive, with the signature, contract, cost, laws and immediate operand
   of the Flow operation. The structure of the functions is kept, so the conversion is a renaming of callees and of the
   declarations. A primitive has no reference model: each target prints it in place, and its conformance is checked per
   Flow operation against its contract.
4. **Immediate operands are typed.** An operation declares the Go type of its immediate operand, which verification checks
   in each call, and how a listing prints it. The static data of a call, such as a literal, a class, a rule or a term, are
   no longer values behind an untyped slot.
5. **The only transitional operations are the five that stay for later work**: the call-tracking and left-recursion growth
   operations. The Flow and Action dialects have none. A transitional operation that exists only as a placeholder of the
   conversion is not kept.
6. **The Flow dump is the Code IR listing.** `-print-after flow` prints each function with the printer of the base and
   the notation of the text form, with the header lines of each dump unchanged. The earlier Flow-specific format is gone.
7. **The specification follows the structure.** The module, the statements and expressions of functions, their invariants
   and the text form are the Code IR specification; the Flow dialect and the Exec dialect (with the Action operations
   under the values chapter) are specified apart. Tests that read the tables of the specification read them from the
   chapter that holds them.

## Alternatives Considered

- **Keep the Flow stage as a separate IR and lower it to Exec with its own printer.** The structure, the printer and the
  verifier would again be written twice, and the relation between the stages could not be checked by one verifier.
- **Convert each Flow operation to a function of the Exec stage written from Exec operations (an expansion).** This keeps
  the Exec stage free of primitives, but the expansions would have to reproduce what each target prints in place today,
  and charges and probes would no longer map to the operation that grammar code names. Primitives keep one operation per
  Flow operation and a contract that each target is checked against.
- **Leave Action expressions as functions lowered at the Exec stage only.** The Flow stage would then hold terms that it
  cannot verify, and the conversion would still have to treat them as a special case. As a dialect legal at both stages
  they are verified wherever they appear and are untouched by the conversion.
- **Keep every Flow operation transitional until each is modeled.** That hides which operations still lack a model and lets
  the number grow. Declaring them primitives with contracts and cost bounds makes the unmodeled ones explicit.

## Testing

Verification at the Exec stage with a negative control that leaves a Flow operation in a converted module; the Flow
verifier on the lowered functions; a check per Flow operation against its contract and cost bound on the Go target, for
reads of the input also against a model of the input; and the specification tests that compare the declarations of the
operations and the kinds of statements and expressions with the tables of the specification.

## Limitations and Open Questions

The five transitional operations are replaced as their mechanisms are written as compositions. The conformance check per
Flow operation covers the Go target; other targets are held to the same contract by their printed tests.
