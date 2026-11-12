# Passes

A pass transforms a well-formed function into a function that does the same: for every state of a parse, running the
result consumes the same input, records the same expectations, builds the same values, calls the same rules, charges
the same budget and ends the same way as running the original. A pass relies only on the meaning of the statements and
expressions of this specification and on the way the function was lowered (local or framed, with or without budget
controls or a Document), never on how a target prints it.

## The Pass Pipeline

Every function that lowering from Core IR produces, and that a backend prints from Flow IR, goes through the pass
pipeline of Flow IR: the passes in their declared order, before the function is lowered to Exec IR or printed.

- A pass has a name that no pass of Core IR or Exec IR has. The name is also the name of the optimization that turns
  the pass off, and of the dump that shows the function after it.
- A pass that is off does not run, and neither does one that does not apply to the build, whatever its optimization
  says. A build that reports rule events applies only the passes that keep them.
- After the last pass the function MUST satisfy the [invariants](invariants.md). Verification runs after the last pass,
  or after every pass when asked, and a violation names the pass after which it was found. Lowering has verified the
  function, so a run in which no pass runs does not verify it again.
- A dump of the function in the [text form](text-form.md) can be asked for as the lowering made it and after each
  pass that ran.

This version defines no pass: the pipeline runs on every function and leaves it as the lowering made it.
