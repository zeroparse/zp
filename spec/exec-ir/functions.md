# Functions

A function of Exec IR is structured code over the operations of the parser VM, other functions and scalar values.
Compositions are functions written once for every runtime (see [Compositions](compositions.md)); the rules of a
grammar are functions lowered from Flow IR (see [Lowering From Flow IR](lowering.md)).

## Function

| Part | Meaning |
|:--|:--|
| Name | Unique in the module |
| Parameters, results | Its parameters, each with a name and a type, and its result types, any number |
| Locals | Locals declared at the function's entry, with the zero value of their type: false, 0 or the null handle. They are visible in the whole body. Code lowered from Flow IR declares its variables here; compositions declare theirs with `Let` where they are needed |
| Features | The features the function is specialized for. It may call an operation only if it is specialized for all of the operation's features |
| Body | A block |
| Doc | A description, which backends print as the comment of the printed function |
| No inline | Asks backends that can to keep the function out of its callers' bodies, so that a caller's fast path stays small and the function runs only when the fast path does not apply |

Running a function binds its parameters to the arguments, sets its locals to their zero values and runs its body. It
ends with a `Return`, which gives its results, or by running to the end of its body, which a function with results
never does (see [Invariants](invariants.md)). A function cannot throw: an operation that may end the parse does so
instead of returning, and the parse ends with it (an interruption).

## Statements

| Statement | Meaning |
|:--|:--|
| `Block` | Runs its statements in order. If it has a label, an `Exit` to the label ends it, and the run continues after it |
| `Loop` | Runs its Init once, if it has one, and then repeats: it checks its condition, if it has one, and ends when the condition does not hold; it runs its body; and it runs its Post, if it has one. A `Break` or an `Exit` to its label leaves it; a `Continue` to its label ends the iteration, after which the Post runs. A loop always has a label |
| `If` | Runs its Init, if it has one, and then its Then block when the condition holds and its Else block, if it has one, otherwise |
| `Let` | Declares new locals with the values of an expression: one name for an expression, or as many names as the results of a call |
| `Set` | Assigns places: with one value for each place, all values are evaluated first and then assigned in order; with one call of as many results as places, its results. A place is a local, a register, or nothing, which discards the value |
| `Do` | Calls an operation or a function for its effects and discards its results |
| `Exit` | Ends the enclosing block, or leaves the enclosing loop, with the label, and continues after it |
| `Break` | Leaves the enclosing loop with the label |
| `Continue` | Ends the iteration of the enclosing loop with the label |
| `Return` | Returns from the function with the values |
| `Protect` | Runs its body and then its cleanup, however the body ends: at its end, by a `Return`, and when an operation ends the parse or the host unwinds through it. The cleanup runs after the values of a `Return` are computed |
| `Assert` | Fails a conformance case when its condition does not hold. Compositions and lowered functions do not use it |
| `Comment` | A comment for readers of printed code. It does nothing |

A `Protect` is the last statement of its function's body, so that its cleanup runs as the function ends. The cleanup
restores state that the body saved: it holds no `Exit`, `Break`, `Continue`, `Return`, `Loop`, nested `Protect` or
`Assert`; it assigns only its own locals; it reads no register; and it calls no function, applies no handle, and
calls no operation that reads or writes a register or may end the parse. It reads the parameters and the locals declared before the `Protect`, not
those that the body declares.

Statements after an `Exit`, `Break`, `Continue` or `Return` in the same block are unreachable. A function may hold
them; running it never reaches them, and backends leave them out.

## Expressions

| Expression | Meaning |
|:--|:--|
| `Const` | A constant of a type: a truth value, an integer of an integer type, or the null handle, which is the only constant of a handle type |
| `Local` | Reads a local or a parameter |
| `Reg` | Reads a register |
| `Feature` | True if the feature is one of the function's features, and false otherwise |
| `Unary` | `!` negates a `bool`; `-` negates an integer, wrapping at its width |
| `Binary` | An operator on two operands (below) |
| `Convert` | Converts an integer to another integer type, wrapping at the width of the type it converts to |
| `Call` | Calls an operation or a function with arguments, which are evaluated in order. As an expression it has exactly one result, except as the value of a `Let` or a `Set` with as many names or places as results. A call of an operation that declares an immediate operand holds it. A call may instead apply a value of a callable handle type to arguments ([Application](#application)) |

The binary operators are the following.

- Arithmetic, `+`, `-`, `*`, `/` and `%`, takes two integers of one type and wraps at its width. `/` and `%` truncate
  toward zero.
- The bit operators, `&`, `|`, `^`, `<<` and `>>`, take two integers of one sized type, `i32`, `u32` or `u64`, because
  `int` has no fixed width.
- The comparisons, `==`, `!=`, `<`, `<=`, `>` and `>=`, take two integers of one type and give `bool`. `==` and `!=` also
  compare two `bool` values and two handles of one type.
- `&&` and `||` take two `bool` values and evaluate the second only when the first does not decide the result.

A division or a remainder by zero, and a shift by a count that is negative or not less than the width, are
unspecified: a function MUST NOT reach them, and backends may differ there.

Expressions are evaluated from left to right, the operands of an operator and the arguments of a call included. Only
calls have effects.

### Application

A handle type that declares a signature is *callable*: its values are code that a target holds in its own form, and
that no function of the module defines, such as the `Term` of a predicate or a binding. An application is a call whose
callee is an expression of a callable handle type instead of the name of an operation or a function. It evaluates the
callee and then the arguments, runs the code of the callee on them, and has the results that the signature declares,
bound as the results of any call are. The verifier requires the callee to have a callable handle type, and the arguments
to have the types of its parameters; an application has no immediate operand. The application of a `Term` to a
context returns the value of the term and its failure, which is null when the term does not fail. A term that fails, on
a list element or a conversion it cannot make, may leave the values it gathered and the nodes it created, which the
operations that release them release. An interruption ends the parse and leaves them too, for the next parse starts without them.

An application is not an operation. It has no contract, no cost bound and no component of its own, because what it
reads, writes and charges, and whether it ends the parse, are those of the code it runs, which is built from the
operations that the module declares. A target prints it as the call of its form of the code. The conformance suite and
the reference interpreter treat a callable handle as a value that the machine they run on applies.

## Interpretation

Exec IR has a reference interpreter that runs a function against an implementation of the operations, by the meaning
this chapter gives. The conformance suite runs its cases with it, against the reference models and against each
implementation that a target binds to it, and the printer tests compare what printed code does with what the
interpreter does for the same functions (see [Printing](printing.md#conformance)).
