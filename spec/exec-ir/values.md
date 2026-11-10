# Values

The component `value` holds the operations on the values of terms: actions, predicates and the values of bindings.
They read and write no state of the parse. A value is a datum of the host, opaque to the rest of the parser VM; these
operations look inside it. Each acts on one kind of value and has none of the policy of an operator of the grammar:
the compositions `binary`, `unary`, `equal`, `index`, `logic`, `member`, `element`, `items`, `length`, `text` and `convert` (see
[Compositions](compositions.md#terms)) decide which operation applies, which operands are valid and which failure a term
reports. The tables of this chapter are shared by every target: a target numbers the operators, kinds and failures as
they are numbered here, and prints the messages of the failures as they are written here, so that they read the same
on all targets.

## Value Kinds

The kind of a value, as `val.kind` returns it, is a code.

| Code | Kind | Values |
|--:|:--|:--|
| 0 | nil | nil |
| 1 | int | an int |
| 2 | string | a string |
| 3 | bool | a bool |
| 4 | other | every other value: a node, a list, a tuple, a struct, and a box that holds a value, which `val.unbox` opens |

## Operators

A call of the composition `binary`, `unary` or `logic` passes the code of its operator: its index in the table of its
group. The codes of the three groups are separate, so the first operator of each has the code 0.

| Group | Operators, by code from 0 |
|:--|:--|
| Binary | `+` add, `-` sub, `*` mul, `/` quo, `%` rem, `==` eq, `!=` ne, `<` lt, `<=` le, `>` gt, `>=` ge, `[]` index |
| Unary | `-` neg, `!` not |
| Logical | `&&` and, `\|\|` or |

The last binary operator, the index of a list or a tuple, is not a scalar operator and takes the composition `index`.
The names after the operators are the names by which targets name the codes.

## Member Kinds

A member name of a term is the name of a field, a range or value member of a node (`Start`, `End`, `Value`), or a
capture (`$` and the name of the capture). The composition `member` takes the kind of a member read as a code. A range or value member reads the
node itself only if the value is a node; of a struct or a list it is the field of that name.

| Code | Kind | Spelling | Reads |
|--:|:--|:--|:--|
| 0 | field | every other name | A field |
| 1 | start | `Start` | The start of the range of a node |
| 2 | end | `End` | The end of the range of a node |
| 3 | value | `Value` | The value of a node |
| 4 | capture | a name that begins with `$` | A capture |

Every target classifies a name by this table: the whole name for the members of a node, the prefix for a capture.

## Failures of a Term

A failure of a term ends the parse with an error whose message is the text of the failure. `val.fail` takes its code.
In a text, `{a}` and `{b}` are the type names of the first and second value; `{name}` is the second value as a string,
a member name or the name of a built-in function; `{op}` is the name of the operator that the operator code indexes,
in the binary operators for invalid operands, invalid comparison and not a list, in the unary operators for invalid unary
operand and in the logical operators for invalid logical operand; `{i}` is the first value as an int; and `{n}` is a
count. A hint, where a failure has one, ends its message.

| Code | Failure | Text | Hint | Meaning |
|--:|:--|:--|:--|:--|
| 0 | division by zero | `division by zero` | | A division or remainder by zero |
| 1 | invalid operands | `invalid operands {a} and {b} for {op}` | | A binary operator applied to operands it does not accept |
| 2 | invalid comparison | `invalid operands {a} and {b} for {op}` | `: nodes and other structured values cannot be compared; use text(x), or .Start and .End` | `==` or `!=` with a value that is not nil, an int, a string or a bool |
| 3 | invalid unary operand | `invalid operand {a} for unary {op}` | | A unary operator applied to an operand it does not accept |
| 4 | invalid logical operand | `invalid operand {a} for {op}` | | A logical operator applied to an operand that is not a bool |
| 5 | not a list | `{op}: expected a list, got {a}` | | A value that is not a list or a tuple used as one |
| 6 | index not an int | `index must be int, got {a}` | | An index that is not an int |
| 7 | index out of range | `index {i} out of range for {n} elements` | | An index out of range: `{i}` is the index, `{n}` the length |
| 8 | no members | `cannot access .{name} of {a}` | | A member read of a value that has no members |
| 9 | no capture | `{a} has no capture {name}` | | The read of a capture of a struct, which has none |
| 10 | no field | `{a} has no field {name}` | | The read of a field that a struct type does not declare |
| 11 | not an element | `{name}: elements must be values, got {a}` | | An element of a list or a tuple that is not a value, such as a function |
| 12 | argument not a list | `{name}: expected a list, got {a}` | | An argument of a built-in function that takes a list and is not one |
| 13 | invalid len argument | `len: invalid argument {a}` | | An argument of `len` that has no length |
| 14 | invalid text argument | `text: invalid argument {a}` | | An argument of `text` that has no text |
| 15 | not convertible | `cannot convert {a} to {name}` | | The conversion to a named node type of a value that is not a string, tuple or list node |

## Operations on Values

| Declaration | State | Cost | Contract |
|:--|:--|:--|:--|
| `op val.kind(x Datum) int` | none | `0` | Returns the kind of x: the [kind](#value-kinds) nil for nil, int for an int, string for a string, bool for a bool and other for every other value, a box that holds one of these included. Law: pure: equal arguments give equal results. |
| `op val.unbox(x Datum) Datum` | none | `0` | Returns the value that x stands for: the value a box holds, and x for any other value. Law: pure: equal arguments give equal results. |
| `op val.fromBool(b bool) Datum` | none | `0` | Returns the bool value b. |
| `op val.fromInt(i int) Datum` | none | `0` | Returns the int value i. |
| `op val.toBool(x Datum) bool` | none | `0` | Returns the bool that x is. Requires: x is a bool: val.kind(x) is the kind bool. |
| `host op val.fail(code int, a Datum, b Datum, op int, n int) Fail noreturn` | none | `0 + 2*error_parts` | Returns the [failure](#failures-of-a-term) with the code code: {a} and {b} read as the names of the types of a and b, {op} as the name of the operator that op indexes (a binary operator for invalid operands and invalid comparison, a unary operator for invalid unary operand and a logical operator for invalid logical operand), {i} as a, an int, and {n} as n. A failure is a Fail, not a value of a term: the caller returns it and the term evaluation reports it, each target in its own form of failure. A target that bounds the work assembles the text piece by piece, charging for each piece. Requires: code indexes TermErrors, and the arguments are those its message names. |
| `op int.add(l Datum, r Datum) Datum` | none | `0` | Returns l + r with 64-bit wraparound. Requires: l and r are ints. Law: pure: equal arguments give equal results. |
| `op int.sub(l Datum, r Datum) Datum` | none | `0` | Returns l - r with 64-bit wraparound. Requires: l and r are ints. Law: pure: equal arguments give equal results. |
| `op int.mul(l Datum, r Datum) Datum` | none | `0` | Returns l * r with 64-bit wraparound. Requires: l and r are ints. Law: pure: equal arguments give equal results. |
| `op int.quo(l Datum, r Datum) Datum` | none | `0` | Returns l / r truncated toward zero; the least int divided by -1 is the least int. Requires: l and r are ints, and r is not zero. Law: pure: equal arguments give equal results. |
| `op int.rem(l Datum, r Datum) Datum` | none | `0` | Returns the remainder of l / r, with the sign of l; the remainder of the least int by -1 is zero. Requires: l and r are ints, and r is not zero. Law: pure: equal arguments give equal results. |
| `op int.neg(x Datum) Datum` | none | `0` | Returns -x with 64-bit wraparound: the negation of the least int is itself. Requires: x is an int. Law: pure: equal arguments give equal results. |
| `op int.zero(x Datum) bool` | none | `0` | Reports whether x is zero. Requires: x is an int. |
| `op int.cmp(l Datum, r Datum) int` | none | `0` | Returns -1 if l is less than r, 0 if they are equal and 1 if l is greater. Requires: l and r are ints. Law: pure: equal arguments give equal results. |
| `op int.index(x Datum, n int) (int, bool)` | none | `0` | Returns x as an int and true if 0 <= x < n, and 0 and false otherwise. Requires: x is an int. Law: pure: equal arguments give equal results. |
| `host op str.add(l Datum, r Datum) Datum noreturn` | none | `1 + 4*value_bytes` | Returns the concatenation of l and r. A target that bounds the work charges for the bytes it reads and copies. Requires: l and r are strings. |
| `host op str.cmp(l Datum, r Datum) int noreturn` | none | `1 + 4*value_bytes` | Returns -1 if l is less than r, 0 if they are equal and 1 if l is greater, comparing by code point: the order of the bytes of UTF-8 and not of the code units of UTF-16. A target that bounds the work charges for the bytes it compares. Requires: l and r are strings. |
| `host op str.eq(l Datum, r Datum) bool noreturn` | none | `1 + 4*value_bytes` | Reports whether l and r are the same string. A target that bounds the work charges for the bytes it compares. Requires: l and r are strings. |
| `op seq.items(x Datum) (Seq, bool)` | none | `0` | Returns the elements of x and true if x is nil (no elements), a list or a tuple, and false otherwise. Law: pure: equal arguments give equal results. |
| `op node.of(x Datum) (Obj, bool)` | none | `0` | Returns x and true if x is a value that has members (a node, a list, a tuple or a struct), and false for every other value: nil, an int, a string and a bool. Law: pure: equal arguments give equal results. |
| `op node.isNode(n Obj) bool` | none | `0` | Reports whether n is a node, which has a range and a value: a value of a node type, a string node or an error, and not a struct, a list value or a tuple value. |
| `op node.start(n Obj) Datum` | none | `0` | Returns the start of the range of n as an int. Requires: node.isNode(n). |
| `op node.end(n Obj) Datum` | none | `0` | Returns the end of the range of n as an int. Requires: node.isNode(n). |
| `op node.value(n Obj) Datum` | none | `0` | Returns the value of n: its text, or the value its node type holds. Requires: node.isNode(n). |
| `op node.isStruct(n Obj) bool` | none | `0` | Reports whether n is a value of a struct type. |
| `host op node.field(n Obj, name Name) (Datum, bool) noreturn` | none | `0 + 2*fields_read + 4*value_bytes` | Returns the field name of n and true if n holds it, and nil and false otherwise. A struct of a target that represents it with every field holds them all. A target that bounds the work charges for the fields it scans and the bytes of the names it compares. |
| `host op node.zero(c Act, n Obj, name Name) (Datum, bool) noreturn` | none | `0 + 2*zero_parts` | Returns the zero value of the declared type of the field name of the struct n, made in c, and true, or nil and false if the type of n declares no such field or the target holds every field of a struct (node.field then holds it). A target that bounds the work charges for the values it makes. Requires: node.isStruct(n). |
| `op node.typeName(t NodeType) Name` | none | `0` | Returns the name of the named node type t, as the {name} of a failure reads it. Law: pure: equal arguments give equal results. |
| `op node.hasBase(n Obj) bool` | none | `0` | Reports whether n is a string node, a tuple node or a list node, of a named node type or not, which a conversion converts: not an error, a box, a list value, a tuple value or a struct. Law: pure: equal arguments give equal results. |
| `op node.convert(n Obj, t NodeType) Datum` | none | `0` | Returns n as a node of the named node type t: n itself if it has the type, and otherwise a new node with the range, the value and the captures of n, the type t and no rule name, which keeps what n holds now. Requires: node.hasBase(n). |
| `op val.name(name Name) Datum` | none | `0` | Returns name as a string value, as the {name} of a failure reads it. |
| `op node.elem(n Obj) Elem` | none | `0` | Returns n as an element of a list or a tuple: n itself. Law: pure: equal arguments give equal results. |
| `host op val.box(c Act, x Datum) Elem noreturn` | none | `1` | Returns x as an element of a list or a tuple: in a target whose elements are nodes, a box that holds x with the range of the input of c, made in c, and x itself in a target whose elements are any values. A target that bounds the work charges for the box it makes. Requires: x is an int, a string or a bool. |
| `op node.hasText(n Obj) bool` | none | `0` | Reports whether n is a string node or an error, whose text is the input of its range. |
| `op node.width(n Obj) Datum` | none | `0` | Returns the length of the range of n, in the position unit, as an int. |
| `op node.count(n Obj) Datum` | none | `0` | Returns the number of elements or children of n as an int. |
| `op node.heldText(n Obj) (Str, bool)` | none | `0` | Returns the text that n holds and true if n is a string node or an error, and false otherwise; the text is then not meaningful. |
| `host op node.rangeText(n Obj) Str noreturn` | none | `1 + 1*value_bytes` | Returns the input of the range of n. A target that bounds the work may charge for the read of the input, and for the bytes it copies out of a stream. |
| `host op str.len(x Datum) Datum noreturn` | none | `0 + 1*value_bytes` | Returns the length of the string x in the position unit as an int. A target that bounds the work and counts characters charges for the bytes it reads. Requires: x is a string. |
| `op str.of(x Datum) Str` | none | `0` | Returns the string x. Requires: x is a string. |
| `op str.empty() Str` | none | `0` | Returns the empty string. |
| `op seq.len(items Seq) int` | none | `0` | Returns the number of elements. Requires: items came from seq.items. |
| `op seq.at(items Seq, i int) Datum` | none | `0` | Returns element i as a value: the value a box holds, and nil for a missing element. Requires: items came from seq.items, and 0 <= i < seq.len(items). |
