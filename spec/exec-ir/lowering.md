# Lowering From Flow IR

Lowering turns the function of a rule variant in [Flow IR](../flow-ir/README.md) into a module of Exec IR that holds
one function, so that the backends of Exec IR print the rules of a grammar as they print the compositions. The
structure of the Flow IR function, its blocks, loops, branches, exits, assignments and the arithmetic and logic of its
conditions, becomes the same structure in Exec IR. Every other primitive of Flow IR, such as matching a literal,
building a value, recording an expectation or calling a rule, becomes a call of a transitional operation that holds
the primitive as its immediate operand and does what the primitive does. The lowered module is verified (see
[Invariants](invariants.md)).

## The Module

The module of a lowered function declares:

- the component `runtime`, the state of the runtime that the primitives change;
- the registers `pos`, of type `int`, the position, and `cut`, of type `bool`, the cut flag of the scope entered last;
- a handle type of the component `runtime` for each type of Flow IR that is not `int` or `bool` and that the function
  uses, with the name of the type, and the handle type `any` for results that the function discards;
- an operation for each kind of primitive and shape of its call (below).

## Operations of Primitives

The operation of a primitive is named `flow.` followed by the name of the primitive's kind, as in
`transitional op flow.MatchLit`; an assignment of a register that holds a handle is of the kind SetReg. A kind called
with several shapes, the types of its arguments and its results, has one operation for each shape: the first is named
by the kind, and the second and later ones add a dot and their number, as in `transitional op flow.MatchLit.2`.

Each such operation is transitional and may end the parse. It reads and writes the component `runtime` and the
registers `pos` and `cut`, its parameters are named `a0`, `a1` and so on, and its immediate operand is the Flow IR
primitive. Its contract is that it does what the primitive in its immediate operand does. The arguments of a call are
the variables that the primitive reads, each once and in the order the primitive reads them, so that passes see its
reads; a primitive that assigns variables is the value of a `Set` of them.

## Variables, Registers and Labels

- Each variable of the function becomes a local of the lowered function: of type `int` or `bool` for the Flow IR types
  `int` and `bool`, and otherwise of the handle type named by its Flow IR type. The entry variables, the start, the
  result and the context, become locals too, which the code that holds the printed body declares.
- The Flow IR registers of the position and the cut flag become the registers `pos` and `cut`. A register that holds a
  handle, the environment or the frame, is read with a call of the operation of the kind Read and assigned with
  a call of the operation of the kind SetReg, because a register of Exec IR holds a scalar. Such an assignment assigns that
  register only.
- A label keeps its name. A label without a name, which no printer shows, is named `loop.` followed by a number.

## Statements and Expressions

| Flow IR | Exec IR |
|:--|:--|
| A block | A block with the same label |
| A loop with a counter | A loop that sets the counter to 0, runs while the counter is less than the maximum, and adds 1 to it after each iteration |
| A loop with a condition | A loop with the condition |
| Any other loop | A loop without a condition |
| An if, with its init | An `If` with the assignment as its Init |
| An exit to a loop | A `Break` |
| An exit to a block | An `Exit` |
| A continue | A `Continue` |
| An assignment | A `Set` of the same places, with a discarded place for a discarded result; one primitive with several results is the call of its operation with as many results |
| An increment | A `Set` of the variable to itself plus 1 |
| A peek | A `Set` of the character, the size and the success from the call of its operation, a discarded place standing for each of them that the peek does not assign |
| An append | A `Set` of the list to the result of the call of its operation, which takes the list and the variables that the appended expression reads |
| A comment | A `Comment` |
| Any other statement | A `Do` of the call of its operation |
| A use of a variable | A `Local` |
| A read of the position or the cut flag | A `Reg` |
| `nil` | The null handle of the expected type |
| An integer or a truth value | A `Const` |
| A negation | The operator `!` |
| An or | The operator `\|\|` |
| A comparison | The same operator, on the type of the variable it compares |
| Any other expression | The call of its operation, with one result of the expected type |

## The Function

The function has the name of the Flow IR function. Its body is a block labeled with the function's failure label, so
that an exit to that label leaves the body; the code that holds the printed body continues after success and after
failure.

A framed function, whose captures the runtime keeps in a frame (see
[Functions](../flow-ir/functions.md)), returns its value, a handle of type `value`, and whether it matched. After the
statements of its body, the labeled block assigns the function's cut flag to the register `cut`, when the
function has one, and returns its value, or the null value when it has none, and true. After the labeled block, which a failure leaves, the function assigns the cut flag again and
returns the null value and false.
