# Text Form

The text form of Flow IR is the readable listing of a function, which tests and debugging show. It is an output:
nothing parses it.

A listing starts with the line `func` and the function's name, followed by `framed` for a framed function and `binds`
for one that binds variables. A line `var` follows for each variable, with its name and type. Then come the statements,
one per line, indented two spaces per level, with nested blocks one level deeper. A framed function ends with the line
`cut =` and its cut flag, if it has one, and the line `return` and its value, if it has one.

A block prints as `block` and its label in braces, a loop as `loop` and its label, followed by its bound, `x < n` for a
Counter x and a Max n or `while` and the condition, in braces. A label without a name prints as `loop` followed by its
number in the order of the listing. An If prints as `if`, its Init followed by `;` if it has one, and its condition, then
its branches in braces. An Exit and a Continue print as `exit` and `continue` with their labels, and an assignment as
its places, `=` and its values, a discard as `_` and a register by its name. Every other statement prints as a short
lowercase word for its kind and its fields, such as `step`, `advance n` or `restore x1, r`, except an increment, which
prints as its variable and `++`.

The listing below is the function of a rule that is a choice of the literal "a", a term that always succeeds and the
literal "b", lowered with no budget controls, no Document and no recovery. The second alternative cannot fail, so the
third is not lowered.

```text
func r
  var x1 int
  var ok bool
  x1 = pos
  block L2 {
    block L3 {
      if _, ok = matchlit("a"); !ok {
        exit L3
      }
      exit L2
    }
    pos = x1
    block L4 {
      exit L2
    }
    pos = x1
    exit fail
  }
  v = nil
```
