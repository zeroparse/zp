# Printing

A backend prints a function of a verified module as source code of its target language. The printed code does what
running the function does (see [Functions](functions.md)), over the target's own implementation of the operations:
for each operation, the target gives the code of a call, given the call with its immediate operand and the printed
arguments, and for each register and handle type, how the code reaches the register and which type of the target
language stands for the handle type. A backend prints only the code that the function holds; it never adds a call of
an operation or a decision of the parse.

ZeroParse has two backends, Go and TypeScript. They print the compositions into each runtime (see
[Compositions](compositions.md#specializations)) and the rules of a grammar, lowered from Flow IR (see
[Lowering From Flow IR](lowering.md)), into generated parsers.

## Common Rules

- The printed code is structured as the function is: a block, a loop and an `If` print as the corresponding
  statements of the target, and a block that declares locals prints in braces, so that its locals end with it.
- An `Exit` or a `Break` of a loop prints as a break and a `Continue` as a continue, labeled only when the loop is not
  the innermost one. Labels are unique in a printed function.
- Statements at an unreachable point (see [Dead Stores](passes.md#dead-stores)) are not printed.
- An operator or a conversion on constants prints as its value (see [Simplification](passes.md#simplification)).
- The locals that nothing reads are not declared, and the assignments that the dead-store analysis leaves out are not
  printed; a dead place of an assignment that stays prints as a discarded value.
- The doc of a function prints as its comment.
- An `Assert` prints as a check that fails with the message "assertion failed: " followed by its message.

## Go

- `bool` is `bool`, `int` is `int`, and `i32`, `u32` and `u64` are `int32`, `uint32` and `uint64`, whose arithmetic
  wraps as Exec IR's does. A handle type is the Go type that the target gives it.
- A function prints as a function, or as a method of the receiver that the target gives, and a call of a function of
  the module goes through that receiver. A function marked no-inline prints with the directive that keeps the Go
  compiler from inlining it.
- A labeled block that an `Exit` leaves prints its label after its statements, and the `Exit` prints as a `goto` to it.
  A `goto` never skips a declaration, because every local that it could skip is inside braces that end before the
  label.
- A `Protect` prints its cleanup as a deferred function, followed by its body: since the `Protect` ends the function,
  the cleanup runs as the function returns, after the values of a `Return` are computed, and as the stack unwinds when
  an operation ends the parse.
- A function with several results returns them as Go's multiple results.

## TypeScript

- A function prints as a method of a class.
- `bool` is `boolean` and `int` is `number`, exact up to 2^53. `i32` and `u32` are numbers, normalized with `| 0` and
  `>>> 0` after every operation that can leave their range, and multiplied with `Math.imul`. `u64` is `bigint`,
  reduced with `BigInt.asUintN`. A handle type is the TypeScript type that the target gives it.
- A labeled block prints as a labeled block statement and an `Exit` to it as a labeled `break`.
- A `Protect` prints as `try` with its body and `finally` with its cleanup.
- A function with several results returns them as an array, except a pair of a handle type that the target packs and
  `bool`, which is one value: the handle when the `bool` is true, and `undefined` when it is false.
- The rules of a grammar lowered from Flow IR print by the conventions of the TypeScript code of the runtime, so that
  they read as the code that the Flow IR printers write.

## The Runtime Protocol

The printed compositions are part of the runtime of generated Go parsers, and the interface between generated code and
that runtime is versioned as the runtime protocol (see
[Generated Parsers](../runtime/generated-parsers.md)). A change to a printed composition that changes what generated
code calls, or what a call means, changes the runtime protocol and needs a new protocol version, as any other change of
that interface does.

## Conformance

Three kinds of tests hold implementations to this specification.

- The **conformance suite** runs conformance cases, functions of Exec IR that call operations, in lock-step on the
  reference model of the parser VM and on an implementation under test, and fails on the first call where they
  differ: in its results, in which handle a result is, in whether the call ended the parse, or in units charged beyond
  the operation's cost bound. Cases are written by hand or generated at random within the operations' preconditions.
  Each case is checked against the reference model first, so a case that is wrong about a contract fails before it
  judges an implementation.
- The **printer tests** print small functions with each backend, run the printed code and compare what it does with
  what the reference interpreter does for the same functions.
- The **value tables** state what the compositions of terms return on the values of fixed fixtures (see
  [Values](values.md)); each target runs them through its printed compositions.
