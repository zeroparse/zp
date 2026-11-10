# Invariants

A module of Exec IR is well formed when it satisfies the rules of this chapter. Verification checks every rule and
reports every violation it finds, not only the first. The module of the parser VM with its compositions, every module
that lowering from Flow IR produces and every module that a pass produces are verified; a backend prints only a
verified module, and its printing rules rely on these guarantees.

## Declarations

- The names of features, components, registers, handle types, operations and functions are not empty, and each is
  declared once in its kind.
- A register holds a scalar: `bool`, `int` or a sized integer, never a handle.
- A handle type belongs to a declared component.
- An operation belongs to a declared component. Each component and register it reads or writes is declared, and each
  feature it names is declared. It writes only its own component and registers, except a transitional operation,
  which may write every component it changes.
- An operation has a contract. It is not both a host and a transitional operation.
- Each term of its cost bound has a positive coefficient and a quantity, and its constant part is not negative (see
  [Costs](machine.md#costs)).
- Every type named in a signature, a parameter or a local is valid, and a handle type it names is declared.

## Functions

- Each feature of a function is declared. Its parameters and its locals have distinct names.
- A function has a body. A `Protect` is the last statement of the function's body and appears nowhere else.
- The end of the body of a function with results is unreachable. A statement completes, so that the point after it
  is reachable, as follows: a block completes when each of its statements completes, or when a statement does not
  but the block has a label that an `Exit` or a `Break` inside it names; a `Loop` completes when it has a condition
  or an `Exit` or a `Break` inside its body names its label; an `If` completes when it has no Else block or either
  branch completes; a `Protect` completes when its body does; `Exit`, `Break`, `Continue` and `Return` never
  complete; every other statement completes. A call of an operation that may end the parse completes, so a function
  with results ends with a `Return` even after such a call.

## Scopes and Labels

- A block opens a scope. A `Let` declares its names in the scope of its block, from the `Let` on. A local is declared
  before it is read or assigned. A name is not declared twice in one scope and does not shadow a parameter, a local of
  the function or a name of an enclosing scope.
- A `Let` of several names has a call with that many results as its value.
- An `Exit` names an enclosing block or loop; a `Break` and a `Continue` name an enclosing loop. A loop has a label.

## Types

- A constant holds a value of its type: a truth value for `bool`, an integer for an integer type, and nothing but
  null for a handle type.
- A local and a register are read and assigned only with values of their types. Reads name declared locals,
  parameters and registers; a `Feature` names a declared feature.
- The operators apply to the types that [Functions](functions.md#expressions) gives them, and `Convert` converts an
  integer to an integer type.
- A `Set` has at least one place and either one value for each place, each of the place's type, or one call with as
  many results as places. A `Return` has one value for each result, each of the result's type.
- A call names a declared operation or function, with one argument for each parameter, each of the parameter's type.
  As an expression it has exactly one result, except as the value of a `Let` or a `Set` (above).
- A function calls an operation only if the function's features include each of the operation's features.
- A call of an operation holds an immediate operand exactly when the operation declares one; a call of a function
  holds none.

## Cleanups

The cleanup of a `Protect` ends normally and changes nothing but its own locals and the state that operations
restore:

- it holds no `Exit`, `Break`, `Continue`, `Return`, `Loop`, `Protect` or `Assert`;
- it assigns only the locals that it declares, and no register;
- it reads no register;
- it calls only operations, never a function, and no operation that may end the parse or that reads or writes a
  register.
