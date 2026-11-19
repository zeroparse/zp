# Text Form

The text form of a function at the Flow stage is the listing of Code IR (see [Text Form](../code-ir/text-form.md)): the
readable form that tests and debugging show, and that the dumps of the Flow stage write under a header line of their
own. It is an output: nothing parses it.

A listing starts with the line `func`, the function's name, its parameters and its results in parentheses, and its features
in brackets, if it has any, and a brace. A line `var` follows for each local, with its name and type. Then come the
statements, one per line, indented two spaces per level, with nested blocks one level deeper, and the closing brace.

A block prints as `block` and its label in braces, a loop as `loop` and its label, followed by its clauses (`init`,
`while` and `post`) and a brace. An `if` prints with its init followed by `;` if it has one, and its condition, then its
branches in braces. An `exit` and a `continue` print with their labels, and an assignment as its places, `=` and its
values, a discard as `_`. A call of an operation prints as its name, its immediate operand in brackets, if the call has
one, before its arguments, and its arguments in parentheses, such as `flow.advance(1)` and `flow.expect([exp 5], $pos)`.
The immediate prints as the operation's declaration formats it, without the addresses of the nodes of Core IR it refers
to, so that the listing of a function is the same in every run. The registers print with a `$`, as `$pos`. A `return`
prints its values, and `null(` the type `)` is the null handle of a type.

The listing below is the function of the rule `"a" / "b"`, lowered for a recognizer with no budget controls, no Document
and no recovery.

```text
func r() (value, bool) {
  var x1 mark
  var v2 value
  var x4 rune
  var x5 bool
  var x8 int
  var ok bool
  var v9 value
  var x12 int
  var v13 value
  block fail {
    x1 = flow.markState([state pos])
    block L3 {
      x4, _, x5 = flow.peek()
      block L6 {
        if flow.guardFails([guard exp 5 depth 0], x4, x5) {
          flow.expect([exp 5], $pos)
          exit L6
        }
        block L7 {
          x8 = $pos
          if flow.hasLit(["a"]) {
            flow.advance(1)
          } else {
            if _, ok = flow.matchLit(["a"]); !ok {
              exit L7
            }
          }
          v9 = flow.newTerminalNode(x8, flow.str(["a"]))
          v2 = v9
          exit L3
        }
        flow.reset(x1)
      }
      exit fail
    }
    return v2, true
  }
  return null(value), false
}
```

The second alternative, which the full listing of the function holds, is omitted here.
