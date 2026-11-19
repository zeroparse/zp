# Passes

A pass transforms a function of a verified module into a function that does the same: for every state and arguments,
running the result gives the same results, calls the same operations with the same arguments in the same order and
ends the same way as running the original. Passes rely only on the meaning of statements and expressions and on the
declarations of operations (their state, features and laws), never on how a target implements an operation.

## Passes in the Pipeline

The functions that a backend prints from the Exec stage are converted from the Flow stage one at a time, each into a module of its own
(see [Lowering From the Flow Stage](lowering.md)). Each such module then goes through the passes of the one compiler pipeline
that run at the Exec stage, in their declared order. A pass has a name that no pass of Core IR or of the Flow stage has, and that
is also the name of the optimization that turns it off; a pass that is off, or that does not apply to the build, does
not run. A pass does not change the module it is given; it returns the module after it, in which it keeps the function
that lowering made, at the same place and with the same name, and in it the locals that lowering declared for the
variables of the function of the Flow stage, because the backend prints the body of that function with those locals;
it may change the body, add locals and add the declarations that its changes need. A module that lost the function or
one of those locals is an error of the passes. The
module MUST satisfy the [invariants](invariants.md) where the pipeline leaves the stage: verification runs right after
the lowering and again where the pipeline leaves the stage, and also after every pass when asked, and a violation names
the pass after which it was found. Lowering has verified the module, so a run in which no pass runs does not verify it
again. No pass runs at the Exec stage yet; the transformations below are used by the compositions and the printers.

## Specialization

Specializing a function for a set of features, which are features of the module, makes a copy of it in which each
`Feature` expression is the constant true when its feature is in the set and false otherwise; the copy is then
canonicalized (below). The copy's features are the set, so that it may call the operations of those features and no
others. A specialization does what the function does in a build that has each feature of the set and no other.

Every printed composition is a specialization (see [Compositions](compositions.md#specializations)). A function
written for several builds tests a feature where its builds differ; after specialization, the code of the features
that are off is gone, and with it the calls of their operations.

## Canonicalization

The target is one pass, `canonicalize`, that applies named local patterns to a function until none applies, trying them
in a fixed order, so that the result is the same on every run. Each pattern is an optimization with its own name, which
turns it off, and each application of a pattern is counted on its own when passes are bisected. The patterns of the Exec
dialect fold constant expressions and remove code that cannot run, in place:

- an operator or a conversion whose operands are constants becomes its value, computed as running it computes it,
  except a division or a remainder by zero, which stays;
- `!` of a constant becomes its value, and `&&` and `||` with a constant operand that decides them become that
  constant, or their other operand when the constant does not decide them; an operand that is dropped never holds a
  call;
- an `If` whose condition is a constant becomes its Init and the branch that the condition selects, and a `Loop` whose
  condition is the constant false becomes its Init;
- the statements after an `Exit`, `Break`, `Continue` or `Return` in the same block are removed;
- a block without a label that declares no local is spliced into the block that holds it, and the label of a block
  that no `Exit` names is removed.

Backends also print an operator on constants as its value, because target languages evaluate constant expressions in
arithmetic of their own: a constant expression of Code IR that wraps would not be accepted, or would not wrap, there.

## Dead Stores

Finding the dead stores of a function, among the candidate locals that a backend names, tells the backend which
locals nothing reads, so that it does not declare them, and which assignments it may leave out.

- Reachability follows the printers: after an `Exit`, a `Break`, a `Continue` or a `Return`, the point is unreachable
  up to the end of the enclosing block; the point after a block is reachable when its end is or when a reachable jump
  leaves it; the point after an `If` is reachable when the point before it is; and the point after a loop always is.
  Only reachable statements read locals, except the Init, condition and Post of a loop, which read whatever their
  reachability.
- An assignment, a `Set` that is a statement of a block with one value for each place, is left out when every value is
  pure and every place is a dead local. A value is pure when evaluating it has no effect and cannot fail. Whether a call
  of an operation is pure is derived from its declaration: it writes no component or register, cannot end the parse,
  and, in a build with the feature `budget`, has a cost of zero. A law of the operation never makes a call pure. Leaving
  an assignment out may leave the locals that its values read unread, so the removal repeats until no more locals become
  dead. The reads of a pure assignment of its own places, such as an increment, do not keep them.
- A local assigned in the Init of an `If` or of a loop, which is never left out, stays; so does a local assigned
  opposite a null handle in an assignment of several values.
- An assignment that is not left out keeps its values, and the backend prints its dead places as discarded.
