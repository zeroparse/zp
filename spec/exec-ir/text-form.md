# Text Form

The text form of Exec IR is the readable listing of a module, which tests, golden files and the messages of
verification show. It is an output: nothing parses it. This specification uses it to write declarations.

## Module Listing

A listing starts with the line `module` and the module's name, followed by a line `features` with the features, if the
module declares any. Each declaration follows after an empty line, in this order: the components, the registers, the
handle types, the operations and the functions. A doc prints as lines starting with `//`, indented one level below
the line it describes; indentation is two spaces per level.

```text
module parser
features budget, document, observe, missing, stream, typed

component memo
  // The memo table: results of rule calls by key. ...

register pos int
  // The position of the parse in the input.

handle Entry of memo
  // A memo entry: the result of a rule call with its outcome and end.
```

An operation prints its kind (`op`, `host op` or `transitional op`), its name, its parameters and its results, and
then, one level deeper, its component, the components and registers it reads and writes, its features, `noreturn` if
it may end the parse, its cost bound after `cost <=` (see [Costs](machine.md#costs)), its contract, its precondition
and each law.

```text
op memo.floor() int
  component memo
  reads memo
  features stream
  cost <= 0
  contract:
    // Returns the floor of the memo: 0 at the start of a parse, then the position of the last
    // memo.prune.
```

A function prints its doc as `//` lines, then `func`, its name, parameters, results and, in brackets, its features, and its body in
braces: its locals as `var` lines, then one statement per line, nested statements indented one level deeper.

| Statement | Form |
|:--|:--|
| `Block` | `block label {` ... `}`, or `{` ... `}` without a label |
| `Loop` | `loop label init s; while c; post s {` ... `}`, each clause only when the loop has it |
| `If` | `if s; c {` ... `} else {` ... `}`, with the Init `s;` and the Else only when present |
| `Protect` | `protect {` ... `} finally {` ... `}` |
| `Let` | `let a, b = x` |
| `Set` | `a, $r, _ = x, y, z`: a register is written with `$`, a discarded place as `_` |
| `Do` | The call |
| `Exit`, `Break`, `Continue` | `exit label`, `break label`, `continue label` |
| `Return` | `return x, y` |
| `Assert` | `assert c "message"` |
| `Comment` | `// text` |

| Expression | Form |
|:--|:--|
| `Const` | `true`, `false` and an `int` as themselves; a sized integer as `i32(5)`; the null handle as `null(T)` |
| `Local` | Its name |
| `Reg` | `$` and its name |
| `Feature` | `feature(name)` |
| `Unary` | The operator before its operand: `!x`, `-x` |
| `Binary` | `(x op y)`, always in parentheses |
| `Convert` | `T(x)` |
| `Call` | `name(args)`, with the immediate operand first, in brackets, when the call holds one: `name([imm], args)` |

```text
// skipTrivia skips the trivia of the Operators Rule r, if it has any, in a lookahead: a failed skip
// leaves the parse as it was, and one that matches discards the errors recovered in it, as a
// lookahead does.
func skipTrivia(r Rule) [missing] {
  if !opr.hasSkip(r) {
    return
  }
  let m = frame.mark()
  $silent = ($silent + 1)
  let ok = opr.skip(r)
  $silent = ($silent - 1)
  if !ok {
    reset(m)
  } else {
    if feature(missing) {
      missing.reset(frame.markMissing(m))
    }
    rec.truncate(frame.markRec(m))
  }
}
```

## Catalog Entries

The tables of [Operations](operations.md), [Values](values.md) and [Compositions](compositions.md) declare each
operation and function on one line, the header line of its listing, with the parts that the listing writes on lines
of their own moved onto it.

- An operation: its kind, its name, its parameters and its results, then its features in brackets, if it has any, and
  `noreturn`, if it may end the parse, as in `op budget.charge(n u64) [budget] noreturn`. The table gives the state it
  reads and writes, its cost bound and its contract in columns of their own.
- A function: `func`, its name, its parameters, its results and its features in brackets, as in
  `func grow(r Rule, min int) (Value, bool) [typed]`.

Results print bare when there is one and in parentheses when there are several. A specialization of a composition is
written as the composition's name followed by the set of features it is specialized for in brackets, as in
`call [budget, document]`, with `[]` for the specialization with no feature on.
