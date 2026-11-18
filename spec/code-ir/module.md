# The Module

A module of Code IR is the unit that a pass transforms, a verifier checks and a backend prints. It declares what its
functions may use: the features that a function can be specialized for, the abstract state of a parse, as state
components and registers, the handle types, and the operations, followed by the functions (see
[Functions](functions.md)). A module holds no operation of its own making: a dialect declares the operations (see
[Dialects and Stages](#dialects-and-stages)), and the module takes those that its stage allows. The state is abstract: a
contract talks about it, and a target chooses how to represent it.

## Types

A value of Code IR has one of the following types.

| Type | Values |
|:--|:--|
| `bool` | A truth value |
| `int` | A position or a count: at least 32 bits, and exact up to 2^53, on every target |
| `i32`, `u32`, `u64` | Sized integers, signed 32-bit and unsigned 32-bit and 64-bit, whose arithmetic wraps at their width |
| A handle type | An opaque value of a declared handle type, or the null handle of that type |

There are no records, arrays or maps: data structures live inside operations, where targets choose their
representation. A handle is opaque: functions pass it on, compare it with `==` and `!=`, and give it to operations; only
operations look inside.

## Declarations

A module declares its features, state components, registers and handle types, each by name and with a doc.

- A **feature** is a boolean constant of a build that a function can be specialized for. An operation that belongs to
  features exists only in builds that have all of them.
- A **state component** is a part of the abstract state that only the operations declaring it may write.
- A **register** is a scalar of the abstract state, of a scalar type, that functions read and assign directly.
- A **handle type** is a type of opaque values. Its component is the state component whose operations make its
  values. A handle type may be callable: its values are code that [an application](functions.md#application) runs, and
  its declaration states the types of the arguments and results of that application.

The features, components, registers and handle types of the abstract machine are declared by the
[Exec dialect](../exec-ir/machine.md); the Flow dialect declares its own types and registers (see
[Functions of the Flow Dialect](../flow-ir/functions.md)).

## Operations

An operation is a transition of the abstract state. Its declaration has the following parts.

| Part | Meaning |
|:--|:--|
| Name | The operation's name, unique in the module. By convention it is the component's short name, a dot and a verb |
| Component | The state component the operation belongs to |
| Signature | Its parameters, each with a name and a type, and its result types, any number |
| Reads, writes | The components and registers it reads and writes. It writes only its own component and registers, except a transitional operation (see [Operation Kinds](#operation-kinds)), which declares every component it changes, so that a pass sees each state it changes |
| Features | The features it exists in. None means every build |
| No return | It may end the parse instead of returning: an interruption, such as an exhausted budget |
| Contract | What every implementation does, observable through its results, the state afterwards, the statistics it counts and what it charges |
| Requires | The precondition that its callers guarantee. Behavior outside it is unspecified: implementations may differ on states that no function reaches, and conformance never exercises them |
| Cost | The bound on the units that one call charges the budget (below) |
| Laws | Facts about the operation that passes may use. Each law has a kind that passes read: equal results (equal arguments give equal results), a stable read (two calls with no write of the component between them give the same result) or the merge of two charges. Each kind has a conformance property that tests the law, and a pass relies only on laws of a kind it names. A law states something about results only; it never permits removing a call, which depends on what the operation writes and charges |
| Immediate operand | A static datum of a call that is not a value of Code IR, such as a literal, a character class, an expectation or a term of the grammar. The declaration gives its kind, the type that the datum has (see [Immediate Operands](#immediate-operands)). A call of an operation that declares one holds it, and passes keep it |


## Operation Kinds

An operation is of one of four kinds.

- An **operation** (modeled) has a reference model of its component, and a conformance suite compares every
  implementation with the model: its results, the state afterwards and its charges against the bound.
- A **host operation** has a contract and a cost bound but no reference model, because its effect lies outside the
  abstract machine's state or is not observable in the result of a parse: input buffers, memory that the parse lets go
  of, callbacks and cancellation. It has no conformance cases; its charges are checked against its bound.
- A **primitive operation** is a leaf of grammar code that each target prints in place, in its own form. It has a
  contract and a cost bound; the conformance and printed tests bind it per target. A conversion from the Flow stage to
  the Exec stage makes the primitive that stands for each operation of the Flow dialect (see
  [Lowering From the Flow Stage](../exec-ir/lowering.md)).
- A **transitional operation** stands for a function of a runtime, with its current behavior, until its mechanism is
  written as a composition of operations of the first kind. It has no conformance cases. An operation is not both a
  host and a transitional operation. Only the operations that the Exec dialect lists as transitional are of this kind:
  the Flow and Action dialects have none.

An operation never runs grammar code and never chooses between outcomes of the parse: reusing or recomputing a
result, growing or stopping, which error or expectation survives. Those choices are made in functions, which read
the results of operations. The exceptions are the transitional operations that invoke a rule's body or a line of an
Operators Rule, which say so in their contracts. A term is not run by an operation: a function applies it
([Application](functions.md#application)).

## Immediate Operands

An operation that declares an immediate operand takes, in each call, a static datum that is not a value of Code IR.
The declaration names the kind of the datum and the type that it has: a call whose datum has another type is
ill-formed, and verification reports it. The operation may also declare how a listing prints the datum (see
[Text Form](text-form.md)); otherwise a listing prints it in the default form of its type. A listing writes the
datum in brackets before the arguments of the call. An operation may be **variadic**: the last of its parameters
repeats, and a call passes zero or more arguments of its type in its place.

## Dialects and Stages

A **dialect** is a named set of operations, with their contracts, that a part of the compiler declares: the Flow
dialect (see the [Flow Dialect Specification](../flow-ir/README.md)), the Action dialect (the operations on the values of terms, see
[Values](../exec-ir/values.md)) and the Exec dialect (the operations of the abstract machine, see
[Exec Dialect Specification](../exec-ir/README.md)). Every operation belongs to one dialect, except an operation that a module declares
for itself, such as the helpers of a conformance case.

A **stage** names the form that a module is in, and lists the dialects whose operations may appear in it.

| Stage | Dialects | Holds |
|:--|:--|:--|
| Flow | Flow, Action | The functions of the rule variants of a grammar, as they come from Core IR |
| Exec | Exec, Action | The same functions and the compositions, converted so that no operation of the Flow dialect remains |

A module that has a stage declares only operations of the dialects legal at it, and verification rejects any other.
That check is also the postcondition of a conversion: converting from one stage to the next is complete when the
result verifies at the later stage. A module of no stage, such as a conformance case, may use any operations that it
declares.

## Costs

The cost of an operation bounds the units of budget that one call charges: at most a constant plus, for each term, a
positive coefficient times the value of a quantity. A quantity is an abstract measure of the state before the call,
which the reference model of the component computes. The charges themselves are the implementation's: for a given
target they are deterministic, and they stay within the bound. The bound is the worst case over the representations
that a target may choose. Charges inside an operation spend the same budget that `budget.charge` and `budget.used`
see, and an implementation may end the parse with an interruption inside an operation only where charges within the
bound could exceed the limit.

A charge that pays for one call, step, iteration or line is made by the function that runs it, with an operation that charges, such as `budget.charge` of the Exec dialect;
an operation of constant work charges nothing, because the call or step around it has paid. An operation that runs a
rule body bounds its charges by what the grammar code it runs charges (the quantity `body`), and its own work
besides. The application of a term is not an operation and charges nothing itself: the operations of the code of the
term charge.


The bound of an operation is written in the text form as `cost <= c + k*q + ...`, where `c` is the constant and each
`k*q` a coefficient and a quantity. The quantities of the Exec dialect are listed in
[The Abstract Machine](../exec-ir/machine.md#quantities).
