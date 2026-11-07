# Terms

Actions, predicates and the values of bindings are terms: a small typed language whose names are resolved. A term
reads a capture slot of the scope that encloses it, a lambda parameter, an Operators Rule local or a variable, and each
term carries its checked type, which is the type of its value. A lambda has no type: only built-in functions call it.

## Term Kinds

| Term | Meaning |
|:--|:--|
| `Const` | A constant: an integer, a string, a boolean or nil |
| `SlotRef` | Reads a capture slot of the enclosing scope, by index and name |
| `Local` | Reads a lambda parameter, `$lhs`, `$rhs` or `$op` of an Operators Rule operator line, or `$value` in the whole-rule action of an Operators Rule |
| `VarRef` | Reads a variable |
| `Member` | Reads a field or a node member of a value |
| `New` | Makes a struct of a type from the fields given. Final marks the struct that is the result of the rule or line, whose range is the rule's. |
| `Convert` | Converts a value to a named node type. Proven records that the checker proved that every value converts |
| `Tuple` | Makes a tuple |
| `Builtin` | Calls a built-in function: `len`, `text`, `foldl`, `foldr`, `map`, `filter`, `list` or `concat` |
| `Lambda` | A function literal, an argument of a built-in |
| `Binary`, `Unary` | An operation on one or two terms |
| `Cast` | Converts a value to a checked type |

## Errors

Evaluating a term can end the parse with an error: a missing field, a type error or a failed conversion or cast.
Constants and reads of slots, locals and variables cannot. A conversion that the checker proved cannot. Every other
term can. The effect of a node that holds a term that can raise includes raising an error (see
[Attributes and Effects](attributes.md#effects)).
