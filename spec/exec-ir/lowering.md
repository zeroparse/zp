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

## Terms

A term of the action language, the action of a rule or of a line of an Operators Rule, the term of a predicate or the
value of a binding, is lowered from Core IR into a function of its own, in a module that declares the parser VM, the
compositions of the operators of terms ([Values](values.md)) with every function they call, the component `runtime`,
and the transitional operations of the leaves of terms below. Two optimizations of a target are options of the
lowering:

- With `field-compare` (a typed parser of Go), a comparison (`==`, `!=`, `<`, `<=`, `>`, `>=`) of which an operand reads
  a field is a call of `term.compare` on two operands: a field read is read by `term.fieldOperand`, after the value it
  reads from, and any other operand is its value made an operand by `term.operand`, in the order written. A string
  field is then compared without making a value of it.
- With `text-dispatch`, a predicate that compares the text of one capture with string literals, `text($x) == "a" ||
  text($x) == "b"` and so on, first tests `term.uncontrolled`: without limit controls it takes the text of the capture
  with `text` and returns whether `term.textIn` finds it among the literals; with them it evaluates the term as written.

A cast to a type that every value fits checks nothing; a target that charges a step for each subterm charges one for it
too.

The function evaluates the term's operands in the order that the term evaluates them and calls one composition for
each operation of the term: `binary` for a binary operator, `unary` for a unary one, `member` for a member read, with
the kind of the member read and the name of the field or capture that the name of the member selects ([Member
Kinds](values.md#member-kinds)), `length` and `text` for the built-in functions, `convert` for a conversion to a
named node type and `conforms` for a cast. The function returns the term's value and its failure, null when the term
does not fail, as a composition does. Each composition that can fail is the value of a `Let` of its value and its
failure, followed by a branch that returns the null value and the failure when there is one. A cast keeps its operand
in a local and, when `conforms` rejects it, returns the failure that `term.castError` makes of it. The caller handles
the failure: a predicate or a binding fails, and an action ends the parse with an evaluation error. A target that
evaluates an action in place, inside the function of its rule, instead of calling the term's function, ends the parse
at each failure with `term.raise`. A logical operator calls `logic` on its left
operand, and evaluates its right operand and calls `logic` on it only when the left one does not decide the result.
An equality or inequality of two calls of `text` compares the two strings with `term.textEqual`, without making a
value of either.

A tuple literal and a call of `list` take the mark of the gathered values with the parser VM's `term.kids`, push the
value of each element with `push`, charging a step for it with `seq.step` before, take the values gathered above the
mark with `term.elemsAbove` and make the tuple or list value of them with `hull`. A call of `concat` pushes the
elements of each argument with `pushItems`, charging a step for the argument; when every argument is a call of `list`
or such a call of `concat`, the elements of the arguments are pushed as `list` pushes them, without making the lists
of the arguments; a call of `map` among them pushes its elements as `map` pushes them, below. A struct literal evaluates its field values in the order written. A target whose structs are made of
their fields makes an empty struct with `term.struct` and, for each field in that order, charges a step with
`seq.step`, sets the field with `term.setField` and widens the range of the struct with `cover`; `made` then records
the struct. A target with a constructor for each struct type calls it once, with `term.new` followed by the number
of fields that the type declares, passing the values in the order the type declares the fields and nil for a field
that the literal does not give; a value is kept in a local before the next one is evaluated only when the literal
gives the fields in another order.

A built-in function that takes a function (`map`, `filter`, `foldl` and `foldr`) evaluates its arguments other than
the lambda in the order written, reads the elements of its list argument with `items`, which fails for a value that
is not a list, a tuple or nil, and loops over them. Each step of the loop charges a step with `seq.step`, takes the
element with `itemAt` and evaluates the body of the lambda in place, with its parameters bound to locals of the loop;
a parameter is read with `val.unbox`, since it may hold a box. `map` takes the mark of the gathered values with
`term.kids` after its list argument, pushes the value of the body for each element with `push` and makes the list
value of the values above the mark with `hull`. `filter` does the same, but calls `filterKeep` on the value of the
body and pushes the element itself, as it is, with `term.keep` when it keeps it. `foldl` and `foldr` keep the
accumulator, their first argument, in a local, bind it and the element to the parameters, and assign the value of the
body to it; `foldr` takes the elements from the last. A call `map($x, (e) => e.$f)` of a projected capture `x`, a
capture whose elements the target keeps as the captures `f` of the elements, makes a list value of the elements of
`x` as they are with `term.copyList`; when it is an argument of a `concat` whose arguments are pushed as they are
made, its elements are pushed with `pushItems`, as those of an argument of `concat`.

A target that does not bound the work leaves out the calls of `seq.step`. A target that charges a step for each subterm of a term under budget controls lowers it with a call
of `term.spend` at the start of each subterm, before its operands; it also calls `term.spend` once for each tuple
literal and each call of `list`, including those whose elements a `concat` pushes, after the values of the elements
and before the first `push`, once for each argument of a `concat` whose arguments are not pushed as `list`
pushes them and for each projected `map` among the arguments of one whose are, before its `pushItems`, and once for
each other call of `map`, before `items`. A variable is read into a local where the term
reads it, since reading it charges the budget; the other leaves are operands of the calls that read them. The
function returns the term's value, or, when the target places the value itself, assigns it as its last statement to
a local of the function that the target declares.

The operations of the leaves are transitional, of the component `runtime`:

| Operation | Immediate operand | Contract |
|:--|:--|:--|
| `term.const` | the constant | Returns the constant. |
| `term.read` | the capture, local or variable | Returns its value. |
| `term.ctx` | none | Returns the context of the evaluation, which `member` takes. |
| `term.name` | a name | Returns the name, an operand of `member`, `push` and `pushItems`. |
| `term.field` | a member and the name it reads | Returns the name of the field or the capture that the member reads, an operand of `member`. A target whose comparison of names charges less for the same string data than for equal bytes cuts it from the name of the member, so that the charge does not depend on whether equal literals share their data. |
| `term.nodeType` | the name of a named node type | Returns the node type, an operand of `convert`. |
| `term.castType` | the cast | Returns the checked type of the cast, an operand of `conforms`. |
| `term.raise` | none | Ends the parse with the failure it is given, for an action evaluated in place. |
| `term.castError` | the cast | Returns the failure of the cast of the value it is given. |
| `term.textEqual` | none | Returns whether two strings are equal, as `==` compares them. |
| `term.fromStr` | none | Returns a string as a value. |
| `term.spend` | none | Charges one step of the budget, under budget controls. |
| `term.elemsAbove` | none | Takes the values gathered above a mark of `term.kids` off the gathered values and returns them, for `hull` to take over. |
| `term.struct` | the struct literal | Returns a new struct of the type of the literal with no field set and an empty range. |
| `term.setField` | the name of the field | Sets the field of a struct that the term is making to a value. |
| `term.fromObj` | none | Returns a struct, list value or tuple value as a value. |
| `term.keep` | none | Pushes an element of the elements that `items` returned, as it is, as the next element of the list that `filter` makes, charging nothing. |
| `term.copyList` | none | Returns a new list value of the elements that `items` returned, as they are, charging for copying them under budget controls. |
| `term.new`*N* | the struct literal | Returns the struct that the constructor of its type makes of the values of its N declared fields. |
| `term.fieldOperand` | the name of the member | Reads the field of a value as `member` does and returns it as an operand of `term.compare`, with the failure of the read; a string field stays a string. |
| `term.operand` | none | Returns a value as an operand of `term.compare`. |
| `term.compare` | the comparison operator | Applies the comparison to two operands as `binary` does, comparing two strings directly, and returns the value and the failure. |
| `term.uncontrolled` | none | Returns whether the parse runs without limit controls. |
| `term.textIn` | the literals | Returns whether a string is one of the literals, charging nothing. |
