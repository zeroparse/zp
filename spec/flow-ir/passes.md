# Passes

A pass transforms a well-formed function into a function that does the same: for every state of a parse, running the
result consumes the same input, records the same expectations, builds the same values, calls the same rules, charges
the same budget and ends the same way as running the original. A pass relies only on the meaning of the statements and
expressions of this specification and on the way the function was lowered (local or framed, with or without budget
controls or a Document), never on how a target prints it.

## Passes in the Pipeline

The compiler is one pipeline of passes. Every function that lowering from Core IR produces, and that a backend prints
from the Flow stage, goes through the passes of that pipeline that run at the Flow stage, in their declared order, before the
function is converted to the Exec stage or printed.

- A pass has a name that no pass of Core IR or of the Exec stage has. The name is also the name of the optimization that turns
  the pass off, and of the dump that shows the function after it.
- A pass that is off does not run, and neither does one that does not apply to the build, whatever its optimization
  says. A build that reports rule events applies only the passes that keep them.
- Where the pipeline leaves the stage the function MUST satisfy the [invariants](invariants.md). Verification runs right
  after the lowering and again where the pipeline leaves the stage, and also after every pass when asked, and a
  violation names the pass after which it was found. Lowering has verified the function, so a run in which no pass runs
  does not verify it again.
- A dump of the function in the [text form](text-form.md) can be asked for as the lowering made it and after each
  pass that ran.

This version defines no pass at the Flow stage: every function leaves the stage as the lowering made it.
