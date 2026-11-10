# Functions

A function of Flow IR is the body of one rule variant: a rule, or a part of its body that runs as a function of its own,
lowered for one combination of the options below (Node, Budget and Document). Its parts are the following.

| Part | Meaning |
|:--|:--|
| Name | The function's name |
| Rule | The rule of Core IR it is the body of |
| Variables | Its variables, each with a name unique in the function and a type, in the order they were made |
| Body | Its statements. Running to the end of the body is success; an exit to the failure label is failure |
| Failure label | The label that an exit takes to fail |
| Entry variables | The start, the position at which the rule was entered, set before the body runs; the result, the rule's value; and the context of the rule's action. The code that holds the function declares them. A framed function has none |
| Binds | The body may bind variables of the grammar, which the entry restores |
| Framed | The function is framed (below) |
| Value | The value with which a framed function succeeds; none where it cannot succeed |
| Cut | The cut flag of the outermost scope of a framed function, a variable, which it reads from the register of the cut flag on entry |
| Node | The function builds the values of the node runtime, and none of the typed runtime |
| Budget | The function is lowered with budget controls, and charges the budget as [Invariants](invariants.md#budget) requires |
| Document | The function is lowered for a build with Documents, and reads input in place only as [Invariants](invariants.md#documents) allows |
| Entered | The function is lowered with budget controls and its entry is charged by the code that holds it: the function charges nothing for its entry and starts charged |

## Local and Framed Functions

A function is local or framed. A local function is a whole rule: it succeeds by running to its end, with the rule's
value in the result variable, and fails by an exit to its failure label; its captures are variables. A framed function
matches an expression whose captures the runtime keeps in a frame: the body of a left-recursion leader or of a rule
that recovers, or a line of an Operators Rule. It succeeds with its value, and the runtime owns the frame, the rule's
value and its finish. On both returns, success and failure, a framed function with a cut flag publishes it to the
register of the cut flag again; the body holds no statement for that, and the code that holds the function writes it
where the function returns.

## Types

| Type | Values |
|:--|:--|
| `int` | A position, a count or a length |
| `bool` | A truth value |
| `rune` | A code point |
| `value` | A value of the runtime |
| `env` | A variable environment |
| `expmark` | A mark of the recorded expectations |
| `expids` | A list of expectations |
| `vals` | A list of values being gathered |
| `ctx` | The context of an action |
| `mark` | A mark of the whole state of a parse |
| `frame` | A capture frame of the runtime |
| `error` | A syntax error |

## Registers

A register is a part of the state of a parse that functions read and assign directly.

| Register | Holds |
|:--|:--|
| `pos` | The position |
| `env` | The variable environment |
| `frame` | The current capture frame |
| `cut` | The cut flag of the scope the runtime entered last |

## Variables, Labels and Places

Variables are declared for the whole function, so that any statement may exit to any enclosing label. A label names a
block or a loop; a label without a name is never printed by a target, which names only the labels of blocks. A place,
which an assignment assigns, is a variable, a register, or a discard, whose value is dropped.

Statements after an exit, up to the end of the block that is exited to or of the loop, are unreachable. Flow IR keeps
them, and a backend leaves out what is unreachable.
