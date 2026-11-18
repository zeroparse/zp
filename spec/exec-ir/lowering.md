# Lowering From the Flow Stage

Conversion from the Flow stage to the Exec stage turns a module of [Code IR](../code-ir/README.md) whose functions
call the operations of the [Flow dialect](../flow-ir/README.md) into a module at the Exec stage, so that the backends
print the rules of a grammar as they print the compositions. Conversion is complete: the structure of each function,
its blocks, loops, branches, exits, assignments and the arithmetic and logic of its conditions, its locals and its
registers are kept, and every call of an operation of the Flow dialect becomes a call of the primitive operation that
stands for it. The result is verified at the Exec stage (see [Invariants](invariants.md)), where an operation of the
Flow dialect is not legal, so that a call left unconverted is reported. Operations of the Action dialect, which are
legal at both stages, are kept as they are.

## The Module

The module at the Exec stage lists the Exec and Action dialects as the dialects legal in it. It declares:

- the component `runtime`, the state of the runtime that the primitives change;
- the registers and handle types of the Flow module, the handle types in the component `runtime`;
- every operation of the Action dialect that the Flow module declares, unchanged;
- a primitive operation for each operation of the Flow dialect that the Flow module declares (below).

## Operations of Primitives

The primitive that stands for an operation of the Flow dialect is named `runtime.` followed by the operation's name
without `flow.`, as `runtime.matchLit` stands for `flow.matchLit`; the correspondence is one to one. It is an operation
of the kind primitive (printed `primitive op`) of the Exec dialect and the component `runtime`, and has the signature,
contract, cost bound, laws, features and immediate operand of the Flow operation, of the same type; the state
component of the Flow dialect is `runtime`, in the places where the Flow operation names it. The arguments of a call
are those of the Flow call, in the same order, so that passes see the reads that the Flow dialect declares. A primitive
has no reference model, because each target prints it in place, in its own form; the conformance of a target per
primitive is checked as [Printing](printing.md#conformance) describes.

## Variables, Registers and Labels

- The variables, registers and labels of the Flow function keep their names, types and meaning in the Exec function:
  the registers `pos` and `cut` are the registers of the machine, and a register that holds a handle, the environment
  or the frame, is read and assigned with the calls that the Flow function holds.
- A label without a name, which no printer shows, is named `loop.` followed by a number.

## The Function

The function keeps its name, its parameters, its locals, its attributes and its body. The body is a block labeled with
the function's failure label, so that an exit to that label leaves the body; the code that holds the printed body
continues after success and after failure. The functions of the Flow dialect are described in
[Functions](../flow-ir/functions.md).

## Terms

A term of the action language, the action of a rule or of a line of an Operators Rule, the term of a predicate or the
value of a binding, is lowered from Core IR into a function of its own, in a module that declares the abstract machine, the
compositions of the operators of terms ([Values](values.md)) with every function they call, the component `runtime`,
and the operations of the leaves of terms below, which are operations of the Action dialect and legal at both stages. Two optimizations of a target are options of the
lowering:

- With `field-compare` (a typed parser of Go), a comparison (`==`, `!=`, `<`, `<=`, `>`, `>=`) of which an operand reads
  a field is a call of `term.compare` on two operands: a field read is read by `term.fieldOperand`, after the value it
  reads from, and any other operand is its value made an operand by `term.operand`, in the order written. A string
  field is then compared without making a value of it.
- With `text-dispatch`, a predicate that compares the text of one capture with string literals, `text($x) == "a" ||
  text($x) == "b"` and so on, first tests `term.uncontrolled`: without limit controls it takes the text of the capture
  with `text` and tests whether `term.textIn` finds it among the distinct literals; with them it evaluates the
  term as written. With two or more distinct literals the result is bound to a local first, which a
  target may print as one switch over the literals; with one distinct literal (repeats collapse) it is returned
  directly, as a plain equality.

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

A tuple literal and a call of `list` take the mark of the elements gathered (the component `gathered`) with `term.kids`, push the
value of each element with `push`, charging a step for it with `seq.step` before, take the elements gathered above the
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
a parameter is read with `val.unbox`, since it may hold a box. `map` takes the mark of the elements gathered with
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
function takes the context that the term is applied to as its one parameter, which `member`, `push`, `pushItems`, `hull` and
`made` take. A term lowered for its value returns it and has the signature of the callable type `Term`. A term that the
target evaluates in place, which assigns the value as its last statement to a local of the function that the target
declares, has no results and is never a `Term` handle.

The operations of the leaves are primitive operations of the Action dialect, of the component `runtime`, each with an
immediate operand of the type that the table names:

| Operation | Immediate operand | Contract |
|:--|:--|:--|
| `term.const` | the constant | Returns the constant. |
| `term.read` | the capture, local or variable | Returns its value. |
| `term.name` | a name | Returns the name, an operand of `member`, `push` and `pushItems`. |
| `term.field` | a member and the name it reads | Returns the name of the field or the capture that the member reads, an operand of `member`. A target whose comparison of names charges less for the same string data than for equal bytes cuts it from the name of the member, so that the charge does not depend on whether equal literals share their data. |
| `term.nodeType` | the name of a named node type | Returns the node type, an operand of `convert`. |
| `term.castType` | the cast | Returns the checked type of the cast, an operand of `conforms`. |
| `term.raise` | none | Ends the parse with the failure it is given, for an action evaluated in place. |
| `term.castError` | the cast | Returns the failure of the cast of the value it is given. |
| `term.textEqual` | none | Returns whether two strings are equal, as `==` compares them. |
| `term.fromStr` | none | Returns a string as a value. |
| `term.spend` | none | Charges one step of the budget, under budget controls. |
| `term.struct` | the struct literal | Returns a new struct of the type of the literal with no field set and an empty range. |
| `term.setField` | the name of the field | Sets the field of a struct that the term is making to a value. |
| `term.fromObj` | none | Returns a struct, list value or tuple value as a value. |
| `term.copyList` | none | Returns a new list value of the elements that `items` returned, as they are, charging for copying them under budget controls. |
| `term.new`*N* | the struct literal | Returns the struct that the constructor of its type makes of the values of its N declared fields. |
| `term.fieldOperand` | the name of the member | Reads the field of a value as `member` does and returns it as an operand of `term.compare`, with the failure of the read; a string field stays a string. |
| `term.operand` | none | Returns a value as an operand of `term.compare`. |
| `term.compare` | the comparison operator | Applies the comparison to two operands as `binary` does, comparing two strings directly, and returns the value and the failure. |
| `term.uncontrolled` | none | Returns whether the parse runs without limit controls. |
| `term.textIn` | the distinct literals | Returns whether a string is one of the literals, charging nothing. |
