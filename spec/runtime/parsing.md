# Parsing

The preceding chapters define what a grammar means rule by rule. This chapter
defines a parse as a whole: how it starts, what the input and its positions are,
what a parse produces, how a syntax error is determined and reported, what
memoization can and cannot change, what a parse costs in time and memory, and
the limits and modes of a parse.

## Start Rule

A parse starts at a rule, the **start rule** of the parse. It MUST be one of the
start rules of the grammar, the rules marked `@start` (see
[Start Rules](../language/grammar-files.md#start-rules)). When no rule is named, the parse
uses the default start rule of the grammar. On every interface that takes the
name of a start rule, an empty name selects the default start rule of the
grammar; it does not select the start rule a parser was compiled or generated
for.

| Interface | How the start rule is selected |
|:--|:--|
| Go API | The `start` argument of `zp.Compile` and `zp.CompileSource`, where an empty name selects the default start rule; `Parser.WithStart` makes a parser with another start rule for whole-input parsing; `Parser.ParsePrefix` selects a start rule for one prefix parse without changing the configured start rule. Both take an empty name for the default start rule |
| Command line | `zp parse -s <rule>`. It defaults to the start rule saved in a compiled grammar (`.zpc`), and otherwise to the default start rule |
| Generated parsers | `Parse` uses the start rule selected when the parser was generated, the default start rule unless another was named; `ParseRule` and `ParseRuleAST` (`parseRule` and `parseRuleWithOptions` in TypeScript) take the name of a start rule, or an empty name for the default start rule |

Naming a rule that is not a start rule is an error that names the rule and says
it is not a start rule; naming an undefined rule is an error that says it is not
defined. The start rule is called at the beginning of the input
with no binding level, so an Operators Rule in it parses the operators of all
levels (see [Level-restricted calls](../language/operators-rule.md#level-restricted-calls)).

## Input and Positions

The input of a parse is a sequence of Unicode characters. The input is given as
text encoded in UTF-8; a byte that is not part of a valid UTF-8 sequence is read
as U+FFFD (REPLACEMENT CHARACTER) occupying one position.

### Positions

A **position** is a number that identifies a place between two characters of the
input: 0 is the beginning, and the length of the input is the end. The **range**
of a node is a pair of positions `[start, end)`.

Positions are measured in a **position unit**, which is selected for each parse.
The unit applies to node ranges, the `.Start` and `.End` members and the
`len` function in actions, and the line and column of syntax errors.

| Position unit | A position is the number of ... from the beginning of the input |
|:--|:--|
| Code points (default) | Unicode code points |
| Bytes | bytes of the UTF-8 encoding |

The unit affects only how positions are counted. Matching is always performed
one code point at a time: a character class or `.` matches exactly one code
point in either unit. In bytes, a code point that is encoded in several bytes
advances the position by that many, and each byte that is not part of a valid
UTF-8 sequence counts as one position (and one character U+FFFD) in either
unit.

This decoding also applies to literals. A U+FFFD in a literal matches either
its valid UTF-8 encoding or one invalid input byte, in either position unit.
For example, `rule main = "�"` accepts the single byte `0xff`; the
matched range ends at position 1. Two invalid bytes require two replacement
characters. Position units do not change which Unicode characters match.
The text of the string node is the matched input: byte mode keeps the invalid byte,
while code-point mode substitutes U+FFFD. The literal's spelling need not have
the same byte length as the matched input.

The unit is selected with `zp.WithUnit(zp.Bytes)` in the Go API and with
`zp parse -unit bytes` on the command line.

### Lines and Columns

Lines and columns in syntax errors are 1-based. A new line starts after each
line feed (U+000A); a carriage return does not start a line. The column is
counted in the position unit. (This differs from grammar source files, where a
carriage return can end a line; see
[Line terminators](../language/lexical.md#line-terminators).)

## Whole Input

Whole-input entry points such as `Parser.Parse` succeed only if the start rule
succeeds **and** consumes the entire input. Parsing evaluates the start rule at
position 0. If the rule fails, the parse fails with a
[syntax error](#syntax-errors). If the rule succeeds without reaching the end of
the input, `end of input` is recorded as expected at the position where it
stopped, and the parse fails with a syntax error. The error is reported at the
farthest position that parsing reached, which can be beyond where the start
rule stopped.

```zp
rule main = "a" ("b" "c")?
```

For the input `abd` a whole-input parse fails at `1:3` expecting `"c"`; for
the input `ad` it fails at `1:2` expecting `"b"` or the end of input.
`Parser.ParsePrefix(name, input, ...)` instead evaluates the named rule at
position 0 without an implicit end-of-input check. A successful prefix returns
both the rule value and the consumed cursor in the selected position unit and
original input bytes; an unconsumed suffix is allowed. An explicit
[anchor](../language/parser-expressions.md#anchors) `$$` inside that rule still requires the
actual end of the supplied input. The API does not change the parser's
configured start rule or shift positions to an enclosing document.

## Results

A parse has one of the following outcomes.

| Outcome | What the parse returns |
|:--|:--|
| Success | The value of the start rule, which is a [node](#nodes) or `nil` |
| Success with recovered errors | The value, together with the list of [syntax errors](#syntax-errors) that [`#recover`](../language/pragmas.md#recover) recovered from |
| Syntax error | No value; the [syntax error](#syntax-errors). If errors were recovered before, only the final syntax error is returned |
| Run-time error | No value; the error of an [action](../language/actions.md#types-and-errors) or a [limit](#limits) that was exceeded. It aborts the whole parse |

The value of the start rule is nil if the rule's value is an optional that did not
match. For example, `rule main = "a"?` on empty input succeeds with `nil`. A rule
whose expression has no valued element, such as `rule main = -"a"`, has the unit
`()` as its value.

### Nodes

The value of a rule without an action is a node. The properties of a node, the
Go type `zp.Node`, traversal, and the text and JSON renderings are defined in
[Nodes and Values](nodes.md).

## Prefix Parsing

`Parser.ParsePrefix(name, input, opts...)` selects the rule `name` for one call
and returns its successful endpoint without requiring the rule to consume the
whole input. It does not change the start rule that the parser was configured
with. The rule MUST be defined; an undefined name is an error. The input is
decoded as for a whole-input parse.

The result is a `PrefixResult`:

| Field | Meaning |
|:--|:--|
| `Node` | The value of the selected rule, as for a whole-input parse (a node, a value node or `nil`) |
| `Consumed` | The successful cursor in the parser's position unit (code points by default) |
| `ConsumedBytes` | The endpoint in bytes of the original UTF-8 input |

Each malformed UTF-8 byte counts as one replacement code point in `Consumed`,
while `ConsumedBytes` still refers to the original bytes. Positions inside `Node`
are local to `input`; a caller that passes a slice of a larger source adds the
slice's starting byte offset to `ConsumedBytes` to find the endpoint in the
source. Both consumption fields come from the parser's cursor and are
independent of the span of `Node`: a rule whose action returns a smaller node, or
`nil`, still reports everything it consumed. With the `Bytes` unit, node spans and
both consumption fields are in bytes. A node span in code points is converted to
bytes before a base offset is added. A zero-width success returns zero
consumption, and a nil `Node` does not imply failure. Only the runtime `Parser`
provides `ParsePrefix`; generated parsers do not.

Outcomes:

| Outcome | Result |
|:--|:--|
| The rule matches | The `PrefixResult` and a nil error |
| The rule matches after `#recover` | The `PrefixResult` and `SyntaxErrors`, as a whole-input parse returns a node with recovered errors |
| Syntax failure, including a failure after an earlier recovery, or a fatal error | The zero `PrefixResult` and the error. No speculative endpoint is exposed |

A zero result alone cannot distinguish failure from a successful zero-width rule
that returns nil; the error does.

`ParsePrefix` accepts position units, backends, recognition, tracing, nesting
limits, context and compute limits. Stream diagnostic options are rejected, as for
`Parse`. A grammar that contains `#missing` is rejected, even if insertion is not
enabled; a positive `WithMissingTokens` limit is rejected for every grammar. Zero
keeps strict prefix matching. Options resolve in order. An explicit `$$` sees the
actual end of the entire supplied string, so a fragment is confined by slicing
both its beginning and its end. Cuts and left recursion keep their ordinary
meaning. Prefix parsing shares no memo entries with a `Document`, does not
shift positions to absolute coordinates and does not map escaped or noncontiguous
fragments. It can run concurrently on one parser. A compiled grammar supports it
on the bytecode backends in the mode that serves the parse: a value parse in parse
mode, recognition in recognize mode and a traced parse in document mode (see
[Compiled Grammars](compiled-grammars.md#operations-of-a-loaded-parser)).

## Incremental Documents

A `Document` owns mutable text and the reuse state of earlier parses. A parse of
a document returns what a parse of the current text from scratch returns. The
document API, versions, snapshots, interruption and change notifications are
defined in [Incremental Documents](documents.md).

## Syntax Errors

When the input does not match the grammar, the parser reports the **farthest
position** that parsing reached. When the parse [records
expectations](#recording-expectations), the error also lists the items that were
expected at that position: literals, character classes, `any character` (for `.`)
and anchors (`beginning of input`, `end of input`, `beginning of line`,
`end of line`).

```text
1:5: syntax error: expected "(", "-", [0-9]
```

- The position is given as a 1-based line and column. The column is counted in
  the [position unit](#positions) (code points by default).
- The expected items are listed without duplicates: first the names (any item
  whose text starts with neither `"` nor `[`, such as `end of input` or a name
  given by [`@diagnostic`](../language/attributes.md#diagnostic)), then the
  literals (starting with `"`), then the character classes (starting with `[`).
  Within each group they are in the byte order of their text. `Expected` holds
  them all in this order.
- The message lists at most the first ten items. A longer list is cut after the
  tenth and ends with `, ... and N more`, where `N` is the number of items left
  out:

  ```text
  1:1: syntax error: expected end of input, "(", "-", "[", "false", "null", "true", "{", [ \t\r\n], [0-9], ... and 2 more
  ```
- Failures inside [lookahead](../language/parser-expressions.md#lookahead) and inside the
  [`skip` of an Operators Rule](../language/operators-rule.md#skip) are not included. Predicates
  and bottom (`_|_`) contribute no expected items.
- If the start rule matches but input remains, `end of input` is expected at the
  position where the start rule stopped (see [Whole input](#whole-input)).
- If nothing was expected at the farthest position, or the parse does not record
  expectations, the message is just `syntax error`.
- A [`@error`](../language/attributes.md#error) attribute replaces the expected items inside
  an expression with a message. If a message applies at the farthest position,
  the message is reported instead of the expected items. Messages are reported only
  when expectations are recorded.

### Recording Expectations

Recording what each failed terminal expected costs time and code on every
alternative, and most parses never show the list. A parse therefore records
expectations only on request. Without it:

- A syntax error has the same position, line and column, and the parse succeeds
  or fails exactly when it does with recording. The farthest position is
  tracked either way.
- `Expected` is empty, `Messages` is empty and the message is `syntax error`.
  The messages of [`@error`](../language/attributes.md#error) and the names of
  [`@diagnostic`](../language/attributes.md#diagnostic) take part in recording,
  so a grammar that uses them reports them only when expectations are recorded.
- Errors that [`#recover`](../language/pragmas.md#recover) and `#missing` return
  as values have the same positions and spans, and the message `syntax error`.
- Trees, spans and values are identical.

Recording is requested as follows.

| Parse | Request |
|:--|:--|
| `Parser.Parse`, `ParseStream`, `ParsePrefix`, `Document` | The option `zp.WithExpectations()`. A `Document` fixes the mode when it is created; a later parse that asks for the other mode is an error |
| Generated Go and TypeScript parsers | Generate with the option `Expectations` (`-expectations` in `zp gen`), which gives the parser its table of expectations and a parse option `WithExpectations` (Go) or `expectations` (TypeScript). A parse records them only when it passes the option; a generated parser without the generation option has no table and no such option, and a parse of it that passes the runtime's `WithExpectations` or `expectations: true` returns an error |
| `zp parse`, `zp debug`, `zp trace`, `zp explain`, `zp profile`, `zp test`, `zp editfuzz` and the playground | Always on |

### Errors in Go and on the Command Line

In the Go API, a failed parse returns a `*zp.SyntaxError` with the fields `Pos`
(the position in the position unit), `Line`, `Col`, `Expected` (every expected
item, in the order above) and `Messages` (the messages of `@error` attributes), both empty unless
expectations are recorded, and the methods
`Error()`, which returns the text above with its position, and `Message()`, which
returns it without the position. A parse that recovered errors returns the node and
an error of type `zp.SyntaxErrors`, a list of `*SyntaxError`; `errors.As` tells it
apart from a fatal `*SyntaxError`, and the node is valid. The list holds only the
recoveries that the result keeps: one that backtracking undoes, or that is
inside a [lookahead](../language/parser-expressions.md#lookahead) or the
[`skip` of an Operators Rule](../language/operators-rule.md#skip), is not in it. The command `zp parse`
prints the tree and then each error on its own line, and exits with status 1 in
both cases. Run-time errors of actions, nesting-limit errors and interruptions are
other error values, not `SyntaxError`.

## Memoization

An implementation memoizes the result of a rule at a position, so that the rule
is not evaluated twice there (packrat parsing). Memoization has the following
observable consequences, and no others.

- **It never changes the result of a parse.** The value, the recovered errors
  and the syntax error are the same with and without memoization. Memoization
  changes only the cost. Whether two calls of a rule at one position return one
  node is not observable, because `==` and `!=` do not compare nodes (see
  [Operators](../language/actions.md#operators-and-functions)).
- **Rules that read variables are memoized per combination of values.** The
  result of a rule that reads [variables](../language/predicates.md#variables) directly or
  through the rules it calls depends on their values when it is called, so a
  memoized result is reused only if the variables have the same values; see
  [Predicates and Variables](../language/predicates.md#interaction-with-memoization).
  Finding a memoized result costs in proportion to the size of the key (the
  number of variables and the length of their string values), not to the
  number of combinations of values that the position holds. In a parse of a
  [Document](documents.md#reuse), a lookup also counts the edits it applies to the
  entry it finds.
- **Actions run when a rule is evaluated.** An action is evaluated every time
  the body of its rule is evaluated and matches, even if the enclosing
  alternative later fails, and also inside a lookahead, under `#atomic` and under `-`.
  An action has no effect other than its value and its [run-time
  errors](../language/actions.md#types-and-errors), so how many times it runs is not
  observable, but whether it runs is: an error in an action aborts the parse
  even in a branch that is abandoned.
- **Left recursion** is evaluated by growing a seed, which relies on the memo
  (see [Left recursion](../language/parser-expressions.md#left-recursion)).
  While a rule grows at a position, every call of it there reads the growing
  result, whatever the values of the variables it reads.
- **The nesting limit can depend on it.** A call that is answered from the memo
  does not nest, so whether a parse that comes close to the [nesting
  limit](#limits) succeeds can depend on memoization.
- **Traces and evaluation counts can differ between backends.** Which calls a
  parse evaluates and which it answers from the memo is part of the cost. The
  rules above leave implementations a choice, such as whether examined ranges
  keep the results of left-recursion contexts apart ([Evaluations of
  Rules](#evaluations-of-rules)), so the calls in a trace and the evaluations
  that a profile counts can differ between the engine and generated parsers.
  The value, the positions, the recovered errors and the syntax error do not.

## Time and Memory

The cost of a parse depends on the grammar as well as on the length n of the
input. Memoization bounds how often a rule body is evaluated at a position, but
some expressions have a cost that the memo does not share. This section defines how
that cost is measured, what holds for every grammar, and the classes of growth
that a grammar can have.

### Cost

The **cost** of a parse is the sum of the following:

- the rule bodies that are evaluated, counting each step of a growing
  left-recursive rule;
- the input that terminals, repetitions of character classes and lookaheads
  examine, counted again each time it is examined;
- the iterations of repetitions and the operators that an Operators Rule
  applies;
- the size of the values that actions and predicates build.

A **text value**, the result of `text` of a node, denotes a range of the
input. In a parse of a whole input, in bytes or in code points and whether or
not the input is valid UTF-8, and in a parse of a [Document](documents.md),
`text` costs a constant, whatever the length of the range: it is not a value
that the action builds in proportion to its size. Materializing a text value
as a string of the host language is not part of the cost of the parse, so a
backend can return a view of the input or a string that it builds when the
caller reads it. To make texts constant, a backend can do work in a parse
that adds up to at most a constant times the length of the input, such as
decoding an input that is not valid UTF-8 when the first text is taken, or
copying the texts that cross the pieces of an edited Document until it joins
the pieces; a parse that takes no such text does not do that work. A [stream](streaming.md) discards input, so in a
stream `text` costs the length of its result. Comparing two strings examines
no more than the shorter one, so it costs at most the length of the shorter
operand, plus a constant.

Outside a [Document](documents.md), and apart from the exceptions below, time is
proportional to cost. The memory that a parse allocates is at most proportional
to its cost plus the input, because each memo entry, node and value is built by
cost that is counted, apart from the copied error lists below. The cost of a
parse is measured in compute units, which differ between backends, and the budget
of [compute limits](#cancellation-and-compute-limits) caps it.

Compute units also count work that the cost does not, and the budget caps it too.
In a parse that recovers errors, a memo entry stores a copy of the errors
recovered in its call, and each call that the entry answers appends them again.
These copied error lists are not cost: their time, compute units and memory can
grow with the square of the length of the input in a grammar of the class Linear,
so that such a parse can exceed the default limits
([linear cost proof](../proofs/linear-cost.md#open-steps), step 8). A store under a
key that a long chain of memo entries holds walks the chain, which cost does not
count either; that matters only where K grows with the input, and has not been
measured.

A parse of a Document also does work that the cost does not count: it applies pending
edits to the memo entries it finds, moves the values it reuses, and works through
what the Document retains from earlier parses. Its time and its compute units
include that work, and the budget caps them with it, so they are not proportional
to its cost; the [linear cost proof](../proofs/linear-cost.md#corollary-compute-of-a-document-parse)
states their bound. The [memory limit](#memory-limit) caps the storage a parse
takes into use in bytes.

### Termination

A parse of a finite input terminates, unless a reader or a callback that the
caller supplies does not return.

- A call of a left-recursive rule at a position where that rule is growing
  reads its current result instead of evaluating its body, whatever the values
  of variables (see [Left Recursion](../language/parser-expressions.md#left-recursion)).
  A rule therefore cannot call itself at the same position without end.
- A repetition stops at the first iteration that consumes no input, an infix
  operator applied without consuming input ends its Operators Rule expression,
  and a recovery applies only when its skip expression consumes input.
- Actions have no recursion: a lambda is only an argument of a built-in
  function, which visits a list once.

No bound that is a polynomial in n holds for every grammar.

### Evaluations of Rules

In the reference implementation and the generated parsers, a rule body is
evaluated at most three times at a position for each binding level and each
combination of the values of the variables that the rule reads, with the
exceptions below. A first evaluation may leave no result in the memo. A result
computed inside a lookahead or the `skip` of an Operators Rule answers only the
calls inside them, and a result computed outside them answers every call, so
at most one evaluation in each of the two states stores a result. A rule and
its value-free twin count as two rules. The exceptions are these.

- A rule that calls no rule, itself included, or that is called from at most
  one place in the grammar, its own body included, is not memoized in an
  ordinary parse. It is evaluated as often as its calls are evaluated. A
  [start rule](#start-rule) that its own body calls, directly or through
  rules called from one place, is memoized all the same: a parse can enter
  it without a call, so the place in the grammar is not its only one. A
  leader (see [Conditions for Linear Cost](#conditions-for-linear-cost)) is
  memoized in every case. A call with a level restriction, `level(e, name)`,
  is a call of `e`; the parse of a right operand inside an Operators Rule is
  not a call.
- While rules grow at a position, a call there of a rule of their
  left-recursion cycle, a leader (see
  [Conditions for Linear Cost](#conditions-for-linear-cost)) or another rule,
  is in the context of those growths. Only the growths of the rule's own
  cycle form its context: a call reads no growing result of another cycle.
  Its result is the result of evaluating its body with the growing results
  that they have reached. The call reuses a memoized result only if that
  result read no growing result, observed no lookahead that read one, and was
  computed while at least the growths of its context were in progress.
  Otherwise it reuses a result computed in a context of the same growths only
  if every growing result that it read is equal to the one there now and every
  such lookahead has the same outcome. Where the parse records the input that
  evaluations examine, equal growing results and equal outcomes also need
  equal examined ranges: in every parse of the engine, whose traces report the
  range, and in a [Document](documents.md) parse of a generated Go parser; a
  generated parser outside a Document parse and a TypeScript parser compare no
  range. A result that read a growing result is kept only until the first
  growth of its context ends, and never enters the memo of a later parse.
  A lookahead counts as observed by its outcome only if nothing else of it
  remains after it: a positive lookahead whose expression makes a capture or
  binds a variable outside the rules that it calls is not observed, and the
  growing results that it read count as read by the result around it. The
  errors recovered inside a lookahead do not remain after it.
- A growing rule evaluates its body once for each step of the growth, and the
  number of steps is at most two more than the length of the input it
  consumes. A leader that grows at a position where another growth of its
  cycle is in progress grows again in each context in which it is called
  there. A step of such a growth reuses the step that an earlier growth of the
  leader took from the same growing result under the same condition, and a run
  of steps that read no other growing result is taken at once when every
  lookahead that those steps observed has the same outcome now and that
  outcome read no growing result of the leader itself.

Every other result of a rule of a left-recursion cycle is memoized like that of
any rule. Calls at a later position finish their own growth, outside the
growth at the earlier position, and are memoized. A growth that begins from a
result that includes the growing result of another growth at its position is
new in each step of that growth, since each step reads another growing result;
when the steps drop its result and it goes on over the input that later steps
cover, it covers that input again in each step. Condition 5 of
[Conditions for Linear Cost](#conditions-for-linear-cost) excludes it.

An ordinary parse without missing-token insertion therefore memoizes every rule
that it reaches, a rule of a left-recursion cycle outside the contexts of
growths, except the rules that the first exception of this section leaves out,
none of which is a leader: a rule that calls no rule, and a rule called from at
most one place. The one call of a rule of the second kind is in the body of a
memoized rule or of another rule of that kind, and following those calls ends at
a memoized rule or at the start rule where the parse begins: when every rule of
a cycle of calls is called from one place, that place is on the cycle, so a
parse reaches the cycle only by beginning at a start rule on it, and that start
rule is memoized. A parse with missing-token insertion also leaves unmemoized
every rule other than a leader that contains `#missing`, directly or through the
rules it calls; such a parse is outside the classes of
[Classification](#classification).

### Expressions That Repeat Cost

The memo shares the result of a rule at a position. It does not share cost
inside a rule body, nor between results at different positions. The following
expressions have a cost that grows with the input they cover, and repeat it when
they are evaluated again over the same input.

| Expression | Cost of one evaluation | Repeated when |
|:--|:--|:--|
| A repetition | Its iterations, up to the input it matches | The body that contains it is evaluated again at an earlier position, so the repetition covers the same input again |
| A lookahead | The cost of its expression | It is evaluated at each of many positions and its expression examines the input after them |
| `#recover(skip=e)` | The cost of the base expression and of `e` | Its result is abandoned and parsing continues inside the skipped input, except for the walks of [skip runs](#skip-runs), which share their cost |
| A growing left-recursive rule | One evaluation of its body per growth step | It is called at many positions of one input, because its result at one position is not built from its result at the next |
| An Operators Rule | One step per operator it applies | As for left recursion |
| A rule that reads variables | As for any rule, once for each combination of values | Calls at one position bind its variables to many different values |
| An action or predicate | Proportional to the sizes of the values it reads and builds | A value grows from its own earlier value, as `s + s` does |

### Skip Runs

The skip expression of a `#recover` runs at every position where the base
expression fails, and a repetition in it walks forward from there. When the
base fails at many positions one after another, as when a repetition retries
a failing recovery at each position, each walk covers the input that the walks
before it covered. The walk of an unbounded repetition from a position ends
where the walk from any later position it passes ends, so the implementations
record the rest of each walk and share it.

A **skip-run site** is a repetition written in a skip expression, not inside
another repetition of that expression that is a site, for which all of the
following hold:

- it has no upper bound;
- its expression contains no predicate that reads or binds a variable (a skip
  expression has no captures);
- its expression contains no `#cut` that applies to the repetition, that is,
  one outside every choice, option, repetition and lookahead of the
  expression;
- neither its expression nor a rule that the expression calls, directly or
  indirectly, reads a variable, contains a `#recover`, or is a rule of a
  left-recursion cycle.

A repetition in a rule that a skip expression calls is not a site, nor is a
repetition inside a site. Bindings made inside a called rule are undone when
the rule returns, so only the predicates of the expression itself count.

An evaluation of a site is a **walk** that visits the positions where its
iterations start. The parser keeps a **slot** for each position, per site and
separately inside and outside lookaheads; a slot is fresh, visited or recorded.
A recorded slot holds the **record** of what the rest of a walk from its
position did: where it ended, its farthest failure and the expectations there,
and the range of the input it examined.

A walk starts **plain**: it records nothing, and marks the position of each
iteration as visited when it moves past it, at the next iteration or when it
starts recording there. A walk that ends at its first iteration marks nothing.
At its first iteration at a position whose slot is not fresh,
which an earlier walk of the site visited, it starts **recording**, and stays
recording to its end. A recording walk that has matched the minimum number of
iterations looks up the slot of the position before each further iteration.
When the slot is recorded, the walk ends where the record ends, records its
expectations again, and extends the range that the enclosing results examined
by the recorded range, as a memoized result does. When a recording walk ends,
at an iteration that fails or consumes no input, or at a recorded position, it
records every position it visited while recording, from the last back to the
first: the record of a position combines its own iteration with the record of
the next position. A walk that does not end, because the parse stops, records
nothing.

Records never change the result of a parse, and a parse in which the base of
no `#recover` fails evaluates no skip expression, so it keeps no slots;
neither does a parse whose walks all end at their first iteration, as when the
base fails at the end of a list and the skip fails at once. The
slots of a parse last for that parse: an [incremental document](documents.md)
starts each parse without them, and its results examine what the walks behind
them examined. A [stream](streaming.md) drops the slots of the positions
before the input it commits. A parse with missing-token insertion keeps no
slots.

A walk under [compute limits](#cancellation-and-compute-limits) charges these
units in addition to those of its iterations; marking a slot is free:

- in a recording walk, 1 for the lookup before each iteration from the
  minimum on;
- 1 more when the slot is recorded, and the charges of recording its
  expectations, as for a memoized result;
- for each iteration of a recording walk, the charges of keeping its
  expectations and of recording them in the enclosing result;
- 1 for each record stored.

Per site and lookahead state, the element of a site is evaluated at each
position at most once while its slot is fresh and once more before it is
recorded, plus, per walk, at most the minimum number of iterations, the last
iteration of a walk that ends plain, and one lookup; the
[proof](../proofs/skip-runs.md) gives the bound. The slots of a site take a
bit for each position between the first and the last position its walks
visit, and a record for each position that recording walks visit.

A skip expression is a **reused skip** when every repetition without an upper
bound written in it, other than inside another such repetition, is a site
whose expression examines at most a constant amount of the input, and the skip
expression calls no rule and contains no predicate, `#recover` or Operators
Rule. The expression of such a site has no repetition without an upper bound
and none of those either; it may consume any constant number of characters in
an iteration, as `[^;\n]` and `("ab" / "a")` do. A reused skip costs a
constant in each of its evaluations outside its sites, and the walks of its
sites evaluate their expressions a number of times linear in the input plus a
constant per walk, whatever the number and order of the walks, so the
[classification](#classification) does not count it as a scanning source
([Lemma 9](../proofs/linear-cost.md#lemma-9-reused-skips)). A skip that calls
a rule stays a scanning source, because a called rule can examine any amount
of the input in one iteration, and so does a repetition in a rule that a skip
calls, which is not a site.

### Run Sites

A repetition outside a skip expression walks the same input again each time a
failed alternative leads back to its start. A **run site** is a repetition
whose walks keep the records of a skip-run site, with the walks, slots and
charges described in [Skip Runs](#skip-runs), extended as below. It is a
repetition for which all of the following hold:

- it has no upper bound and no `#cut` applies to it from its expression, as for
  a skip-run site, and it does not repeat a single character, which an
  implementation scans without records;
- its expression contains no Operators Rule and no `#stream`, `#missing`,
  `#error` or `#diagnostic`; it may contain captures (which live in one
  iteration), actions, predicates, variables, `#recover` and calls;
- every repetition without an upper bound in its expression is a run site or
  repeats a single character;
- every rule its expression calls, directly or indirectly, is called without a
  level, is not in a left-recursion cycle and contains no `#stream`,
  `#missing`, `#error` or `#diagnostic`;
- it is restartable: it is in the expression of another repetition whose
  maximum is neither 0 nor 1, or in a rule that such an expression calls,
  directly or indirectly, or in a rule that calls itself, directly or
  indirectly. Any other repetition is reached once by a derivation through
  sequences, alternatives and calls, so a walk of it begins at each position
  at most once per environment and no walk could reuse the record of another.

The result of the rest of a walk from a position is a function of the
position, the variables and the input: an iteration reads the variables that
its predicates, its actions and the rules it calls read, and the captures of
a repetition element are visible to the iteration alone. A run site is **free
of the environment** when its expression has no predicate that reads or binds
a variable, no [inline action](../language/actions.md#inline-value-actions) that reads one, and
calls no rule that reads one, directly or indirectly, in a predicate or an
action. A binding
made in a called rule is undone when the rule returns, so the rest of a walk
of such a site is a function of the position and the input, and it ends in
the environment it began in. Its records are kept by position alone, and a
walk that reuses one keeps its own environment. Skip-run sites are free of
the environment by their conditions.

The records of any other run site are kept for a position and the
environment (the values of the variables) in which the walk started there,
and a walk reuses one only in the same environment: a record for the same
position in another environment is neither used nor replaced, so the
environments of two walks at a position never share a record. Two
environments are the same if they are the same object (bindings that leave a
variable's value alone leave the environment as it is, and equal environments
that were built separately differ). A walk under a freshly rebound but equal
environment therefore does not reuse the records of a walk under the earlier
one, and a grammar that rebinds a variable to the same new value at each
position, and walks a site that is not free of the environment there, can
cost more than linear in the input even when its sites keep records.

Besides what a record of a skip-run site holds, it holds
- for a site that is not free of the environment, the environment at the end
  of the rest of the walk, which the parse takes as its own when a walk reuses
  the record;
- the elements of the rest of the walk, for a run site whose iterations have
  values: a walk that reuses the record lists them after its own elements. The
  list a walk returns and the records of its positions share one sequence of
  elements, which later lists hold as a suffix; a list that is made from its
  own elements and a recorded suffix copies the suffix, except where the
  sequence has room before the suffix that no record uses. The elements of a
  record are final: no later step of the parse changes them;
- the errors that the rest of the walk recovered: a walk that reuses the
  record recovers them again, as a memoized result does.

An implementation may keep records only once the walks of a site have shown
that they restart. One way: the site keeps the furthest position that its
walks of a minimum length (a constant) reached, and keeps records from the
first walk that begins below that position and walks at least that constant
length of input below it. Until then the site is an ordinary repetition. Each
walk then costs its length, of which the part below the furthest position is
shorter than the constant, so the walks of the site cost in total at most the
input length plus the constant for each walk, however the walks alternate
between positions. A site whose walks never go back, as a list that a parse
reads once, keeps nothing.

A walk that reuses a record does not enter the calls that the recorded walk
made, so their nesting does not count toward the [nesting limit](#limits), as
for a call answered from the memo: whether a parse that comes close to the
limit succeeds can depend on records.

The walks of run sites nest: the expression of a site can call rules that walk
sites of their own, and the walks in progress are a stack. Records never
change the result of a parse, and they last for one parse, as those of skip
runs do. The [Run Sites](../proofs/run-sites.md) proof shows that a reused
record gives what the rest of the walk would, and bounds the evaluations of a
site's element. In a parse of an [incremental document](documents.md) outside a
lookahead, a run site that the document can resume is resumed as before.

### Classes of Growth

A grammar's cost as a function of n falls into one of these classes. A class
describes the worst input of each length.

| Class | Cost | Example grammar | Input |
|:--|:--|:--|:--|
| Linear | At most proportional to n | `main = (A / "a")* $$; A = "a" "c"` | `a` repeated n times |
| Linear given an environment bound | Proportional to n times the number of combinations of variable values that occur at one position and the size of the strings they hold | A rule that reads `indent`, bound from the length of a captured run of spaces | Lines of increasing indentation |
| Polynomial of degree k | Proportional to n^k on its worst inputs | `main = (A / "a")* $$; A = "a"* "c"` (k = 2) | `a` repeated n times |
| Exponential | Can double when n grows by a constant | `var v string`; `R = "(" {v := v + "0"} R ")" "y" / "(" {v := v + "1"} R ")" "z" / "n" {len(v) >= 0}` | `(` n times, `n`, then `)z` n times |

The following grammars have the class shown on the inputs shown.

| Grammar | Input | Class | Cause |
|:--|:--|:--|:--|
| `main = (E "q" / "x")* $$; E = "x" E / "x"` | `x` n times | Linear | The result of `E` at a position is built from its memoized result at the next |
| `main = (E "q" / "x")* $$; E = E "x" / "x"` | `x` n times | Degree 2 | `E` grows to the end of the input at every position, and `"q"` then fails |
| `main = (E "q" / "x")* $$` with `E` an Operators Rule with the operand `"x"` and the infix operator `""` at one left-associative level | `x` n times | Degree 2 | As above, for the operators that `E` applies |
| `main = ("a" !Rest)* "a"* $$; Rest = "a"* "c"` | `a` n times | Degree 2 | The lookahead scans to the end of the input at every position |
| `main = A* $$; A = "a" &(x:W) => text($x) + "!"; W = "a" W / ""` | `a` n times | Degree 2 | The lookahead is linear, since `W` is memoized, but each iteration copies the rest of the input, which the capture in the lookahead holds |
| `main = A* $$; A = "a" y:L => text($y) + "!"; L = &(x:W) => $x; W = "a" W / ""` | `a` n times | Degree 2 | As above: `L` consumes nothing, but its value is the capture in its lookahead |
| `main = (X / "a")* $$; X = (Y #recover(skip=[a]+)) "c"; Y = "b"` | `a` n times | Linear | The skip consumes the rest of the input, `"c"` then fails, and the recovery is abandoned, but `[a]+` is a [skip run](#skip-runs): the walk from each later position takes the rest of the walk from its record |
| `main = (X / "a")* $$; X = (Y #recover(skip=S)) "c"; Y = "b"; S = [a]+` | `a` n times | Degree 2 | As above, but the repetition is in a rule that the skip calls, so it is not a skip run, and each skip walks to the end of the input again |
| `main = (A / "a")* $$; A = (B / "a")* "c"; B = "a"* "d"` | `a` n times | Degree 3 | A repetition that can restart inside another one that can restart |
| `main = S $$; S = &R S "q" / "y"; R = R "q" / S` | `y`, then `q` n times | Degree 2 | In each step of the growth of `S`, the lookahead grows `R` from the growing result of `S` over the rest of the input, and drops it |
| `main = S $$; S = &R S "q" / "y"; R = R ("q" / "r") / S "t" / "y"` | `y`, then `q` n times | Linear | `R` grows from the growing result of `S` only after a `"t"`, where `S` cannot grow; its growth from `"y"` is the same in every step and is reused |
| `main = C $$; A = C "+" / "n"; C = "(" C ")" "y" / "(" C ")" "z" / A` | `(` n times, `n`, `)z` n times | Linear | `A` and `C` form a left-recursion cycle grown at `A`. The result of `C` at a position outside the growth of `A` there is memoized, so the two calls of `C` at the next position share it |
| `var v string`; `R = "(" {v := v + "0"} R ")" "y" / "(" {v := v + "1"} R ")" "z" / "n" {len(v) >= 0}` | As above | Exponential | The two calls of `R` at a position see different values of `v`, so neither reuses the other's result |
| `main = xs:"x"* => len(foldl("a", $xs, (acc, x) => acc + acc))` | `x` n times | Linear evaluations, exponential value size | The accumulator doubles for each element |

The variable grammar with the bindings `{v := "0"}` and `{v := "1"}` grows
linearly. The classification reports it as linear given an environment bound,
since it counts `len(v)` as copying all of `v`, whose size only the values of
the variables bound. The [classification](#classification) reports the
left-recursive grammar as unknown, not linear: its cause is a growth restarted
inside a recursion of rule calls, which the classifier does not bound.

### Conditions for Linear Cost

The conditions below use these terms.

- A **copying operation** is an operation of an action, predicate or binding
  whose cost the classifier counts in proportion to the size of the values it
  builds or examines: joining strings, converting a value to a string, building a list with `list`, `concat`, `map`
  or `filter`, taking the length of a string value with `len`, and comparing
  two strings. The size of a string is its number of characters,
  and the size of a list its number of elements; the elements themselves are
  shared, not copied. So `list` copies one element for each argument, which
  the grammar bounds; `concat`, `map` and `filter` copy the elements of the
  lists they read, and the list of a repetition has one element for each
  iteration; `len` of a string value is counted as copying all of it, whatever
  the position unit, a conservative reading, since the engine can take some of
  these lengths in constant time, while `len` of a node or a list is not
  counted, since it takes constant time; the classifier tells a string value
  from a node by the types of the checker, and counts an operand whose type it
  does not know as a string value; and a comparison examines no more than the shorter string,
  so it is counted as copying only one of its operands, and comparing with a
  literal costs a constant. Taking the text of a node is not a copying
  operation, since it costs a constant (see [Cost](#cost)); its result has
  the size of the range, which counts where another operation copies it.
- The **size degree** of a value is the degree, in the size n of the input,
  of the bound on its size that the classifier knows, where the size of a
  list is one more than its number of elements plus the sizes of its
  elements, and a node has the size of its text or of the values of the
  captures in it, whichever is larger. A value is **own** when its size is at
  most a constant times one more than the input that its match consumes, so
  that the own values of distinct matches, such as the elements of a
  repetition, add up to at most a constant times n. The value of a match is
  own when no capture in a positive lookahead, which stays in effect over
  input that the match does not consume, is part of it, and every rule and
  action in it builds an own value. An action builds an own value from
  literals, ints, Booleans and own captures that are not in a positive
  lookahead, by `+`, `text`, `list`, `concat`, `filter`, constructors,
  members and elements, and by `map` and folds over own lists whose lambdas
  read nothing from outside them that is not bounded by a constant and read
  the first parameter of a fold at most once, when it reads a capture that
  holds a call on a recursion with its rule at most once, and as the list of
  a lambda only when it is a capture of a repetition, whose elements are
  distinct matches, and that lambda and every lambda in it read each member
  of each of their other parameters at most once, where distinct members of
  an element, such as `x.$a` and `x.$b`, are disjoint parts of its match; the
  value of a variable is not own. An own value has size degree 1. A capture
  or a call has the size degree of its value, the list of a repetition one
  more than its elements unless they are own, and a value built by an action
  the degree that the forms of condition 4 give it from the size degrees of
  what it reads. The values of the rules of a recursion that are not own have
  one more than the degree of what one level of the recursion adds, when each
  of them holds the value of each call on the recursion at most once: as a
  capture not made in a positive lookahead that is read once, outside
  lambdas or as the list of such a lambda over a repetition. Such a value is
  a tree whose nodes are distinct matches, at most a constant times n of
  them. Otherwise their size degree is not known.
- A **left call** is a call of a rule of a left-recursion cycle, in the body
  of a rule of the same cycle, that can run before that body consumes input.
  The rules that grow are the **leaders**, fixed by the grammar: until every
  cycle of left calls passes through a leader, the rule declared first on a
  cycle that passes through none becomes one, so a cycle can have several. A
  call of a leader grows it at the position of the call, and a left call of a
  leader at that position reads the seed that the growth has reached. A call
  of another rule of the cycle evaluates its body, unless it reuses a memoized
  result outside a growth at its position (see
  [Evaluations of Rules](#evaluations-of-rules)), and its left calls reach a
  leader, which grows there. A call that is not a left call is outside every
  growth of its cycle at its position: such a growth would be evaluating the
  caller, and every rule that it evaluates on the way from the growing leader
  to the call, at that position, so all of them would be on the cycle and the
  call would be a left call. So a call of a rule of the cycle other than a
  leader that is not a left call is **memoized at the call** unless the rule
  is not memoized for another reason, as one called from one place is not.
  The growth that such a call starts at its position ends, after its last
  step, in the body of the rule that the call evaluates, after the left call
  of the leader that grew, and the parser continues from there and then after
  the call. Condition 3 covers what each
  step of the growth evaluates again, and the first step of a growing unit and
  condition 3 are checked with the leaders as the rules that grow.
- A **scanning source** is one of the following: a repetition without an upper
  bound, the skip expression of a `#recover` that is not a
  [reused skip](#skip-runs), the growth of a left-recursion
  cycle at every call of a rule of the cycle other than a left call, the
  operators that an Operators Rule applies, and a copying operation with an
  operand, other than the value of a variable, that is not bounded in size by
  a constant of the grammar. A call of a rule of the cycle after input has
  been consumed in a body of the cycle, such as the second `E` in
  `E = E "+" (E "q" / "x") / "x"`, starts a growth of its own at a new
  position. In the same way, the right operand of an operator part is parsed
  at a new position and can apply operators itself (see
  [Parsing Rules](../language/operators-rule.md#parsing-rules)), so a unit of
  an Operators Rule contains a scanning source when its right operand can
  apply an operator. Copying the
  value of a variable costs at most L, the bound of
  [Classification](#classification), in each evaluation, which condition 4
  counts instead. An expression **contains** a scanning source when the
  source is part of it, or of the body of a rule that it calls, directly or
  indirectly.
- A **bounded scan** is a repetition without an upper bound whose iterations at
  any one position number a constant, however the units around it fail. It
  costs a constant times the input in all, wherever it is evaluated, so the
  conditions do not check it: in the conditions and the terms below, and in the
  classes of [Classification](#classification), a scanning source is one that
  is not a bounded scan, and the cost of a bounded scan is left out of what a
  term says is bounded or repeated. A bounded scan needs two bounds, and neither
  implies the other.

  The **alphabet** of a repetition is the set of characters that an evaluation
  of its expression can consume, through its calls and outside its lookaheads.
  The first bound is on its **run**: at most 32 characters of the alphabet stand
  directly before any position where the repetition is evaluated and the next
  character is in the alphabet. A repetition that reaches a position has consumed
  only characters of its alphabet since it started, so it can have started only
  in the first positions of the longest stretch of the alphabet that ends there.
  The classifier follows the length of the run forward through the grammar from
  0 at the start of the input. A character outside the alphabet ends the run,
  one in the alphabet extends it, and one that can be either extends it or ends
  it. The run is also empty where the next character is known to be outside the
  alphabet: after a greedy repetition of an expression that surely consumes every
  character of the alphabet, after a negative lookahead of such an expression,
  after a positive lookahead that cannot match nothing and whose first characters
  are outside the alphabet, and at the end of the input. It takes every path that
  an Operators Rule, a `#recover` or a `#missing` allows, and a repetition or an
  Operators Rule whose body extends the run has no bound.

  An expression **surely consumes** a character when every evaluation of it with
  that character next succeeds and consumes input, whatever else the input holds.
  The classifier knows this of a terminal of one character, of a repetition with
  a minimum of 1 of an expression that surely consumes it, and of a choice when
  an alternative surely consumes it and no alternative before that one can match
  nothing or has a [cut](../language/parser-expressions.md#cut) that commits the
  choice, a cut in the skip of a `#recover` included: an alternative that fails
  after such a cut fails the whole choice, so the alternatives after it are not
  tried. It knows it of a sequence when one part surely consumes it, the parts
  after that one always succeed, and the parts before it are options, repetitions
  with a minimum of 0 and negative lookaheads of expressions that cannot match
  nothing, whose first characters do not hold it and whose options and repetitions
  have no such cut. And it knows it of a call of a rule whose body surely consumes
  it, when the rule is not on a left-recursion cycle, reads no variable and takes
  no level: a call on a cycle can read the failing seed of a growth, however its
  body would match.

  The second bound is on its **evaluations**: the repetition stands at a
  **counted place**. An expression stands at a counted place when it is in the
  body of a **counted** rule, outside the skip, the operands and the operator
  parts of Operators Rules, only **rigid** expressions come before it in that
  body, and every repetition that encloses it there is rigid. A rule is counted
  when it is memoized and not on a left-recursion cycle, or when it is not
  memoized, not recursive and not on a left-recursion cycle, and every call of it
  stands at a counted place. An expression is
  rigid when a constant bounds the positions from which an evaluation of it can
  end at a given position: a terminal, a lookahead, a sequence, choice or option
  of rigid expressions, a repetition of a rigid expression with an upper bound, a
  repetition without an upper bound that has the first bound, and a call of a rule
  that is not recursive and whose body is rigid. A `#recover`, an attributed
  expression, an Operators Rule and a stream are not rigid.

  For example, in `main = (n ws)* $$`, `n = double / int`,
  `double = [0-9]+ "." [0-9]+`, `int = [0-9]+`, `ws = [ ]*`, each `[0-9]+` is a
  bounded scan, so `int` reading again what `double` read costs one more pass.
  Memoization alone gives no such bound: it counts the evaluations at one
  position, and a run has many positions. `[0-9]+` in `(t "y" / t "z" / .)*` with
  `t = d "x"`, `d = [0-9]+` is memoized and quadratic, since `.` extends the run
  and starts it at every digit. Nor does a bound on the run suffice: `sp = [ ]*`
  in `e = "a" e (sp "!")? / "b"` is evaluated once for each level of the
  recursion at the end of the same nest, which is quadratic on `a` n times, `b`
  and n spaces.

  The classifier decides both bounds, as it does the conditions, on the
  [cost view](../proofs/grammar-laws.md#normalization) of the rule bodies, a rewriting that keeps
  what they match and is meant to keep their cost within a constant factor,
  which is not yet proved (step 9 of the
  [open steps of the linear cost proof](../proofs/linear-cost.md#open-steps)),
  with the memoized rules and the left-recursion cycles of the grammar as
  written.
- A **unit** is an expression after whose failure the parser continues at the
  position where the unit started: an alternative of a choice other than the
  last, one iteration of a repetition, the expression of an option, the
  expression of a lookahead, which is abandoned whether it matches or not, the
  base of a `#recover`, and, in an Operators Rule, each operator part together
  with its right operand and each `operand` item other than the last. A unit
  fails when something inside it fails, including a later part of its own
  sequence and a rule that it calls.
- A **growing unit** is a unit whose evaluation begins with a left call: the
  left call is the first expression that it evaluates, directly or through
  calls of rules of the cycle, such as the alternative `E "+" T` in
  `E = E "+" T / T`. A unit that evaluates anything before its left call, such
  as `("y")? E S "q"`, is not a growing unit. In each step of a growth after
  the first, the left call reads the seed, and the unit examines the input
  from the end of the seed. In the first step, the left call fails before the
  unit examines any input when the unit reaches the leader only through
  rules of the cycle that cannot succeed without their own left call, as
  `add` cannot in `expr = add / term; add = expr "+" term`. Otherwise, as
  through `F` in `E = F "+" X / Y; F = E / M`, the first step can match `M`
  and continue the unit after it, and the unit is also checked as an
  alternative that is not growing: from its start, against the later
  alternatives of its choice. A body of a leader is evaluated only in the
  steps of its growth, and in the first step its own left calls read the
  failed seed, so for a unit of a leader this check follows only the paths
  that evaluate something other than a left call of that leader first, from
  there. A unit of another rule of the cycle is checked as an alternative that
  is not growing also when a call that is not a left call evaluates its rule,
  directly or through left calls of rules of the cycle other than leaders:
  that call evaluates the unit once more, outside the steps, where the leader
  that its left call reaches grows and gives its final result.
- The **continuations** of a unit are the expressions that the parser can
  evaluate next at the position where the unit started:
  - for a growing unit, the growing units that the same step can evaluate
    after it, which also start at the end of the seed, and what follows the
    calls that start the growth, at the end of the seed: a call of a leader of
    the cycle that is not a left call, followed by what follows it, and a call
    of another rule of the cycle that is not a left call, followed by what
    follows the left calls of leaders that its body reaches through left calls
    of rules of the cycle and then by what follows the call. Such a left call
    of a leader starts the growth when that call evaluates it, and reads the
    seed only in the steps of a growth, so at the end of the body of a leader
    what follows the growth continues after it too. A later growing unit
    that succeeds ends the step with a longer result, which the next step
    reads as the seed, so at the end of a rule body it continues through every
    call of the rule, the reads of the seed included: as in
    `A = A S [^!]* "!" / A S / "n"`, the next step can scan again, from the
    end of the new seed, the input that the unit examined. What the step
    evaluates after the unit fails runs at the position where the rule grows,
    and condition 3 covers its cost. When the left call fails in the first
    step, that evaluation is the one of the first step after the same failure,
    except that later growing units read the seed, so unless one of them
    succeeds, the step ends where the first step ended, at the end of the
    seed. Otherwise the check as an alternative that is not growing keeps the
    later alternatives from consuming the character at the position where the
    rule grows, so the step again ends no farther than the seed. Either way
    the growth ends with the seed and the parser continues at its end;
  - for any other alternative, the later alternatives of the choice, unless
    every way in which the alternative can fail lies after a `#cut` in it;
  - for an iteration, an option or a lookahead, what follows the repetition,
    option or lookahead, through the ends of the enclosing expressions and, at
    the end of a rule body, through every call of the rule;
  - for the base of a `#recover`, its skip expression, followed by what follows
    the `#recover`;
  - for a unit of an Operators Rule, the other operator parts and `operand`
    items that the rule can try at the same position, and what follows the
    Operators Rule expression. The Operators Rule **nests** when it has a prefix
    operator part or a right-associative infix one, whose right operand applies
    operators of its own level, or a call from the rule that nests at one end
    (below), such as the call of `x` in an operand `"(" x?` of `x`, which parses
    the rule again at the end of an operand or an operator part. An inner level
    can then end where every enclosing operator part or operand ends, and each
    enclosing level tries the operator parts after an operand again there: for
    an operator part after an operand, those operator parts are a continuation
    too. An Operators Rule without an infix or a postfix operator part tries
    nothing after an operand or after the application of a prefix operator and
    evaluates no `skip` there, so a chain of its prefix operators does not
    evaluate the `skip` again at each level.
- The **first characters** of an expression are the characters with which an
  evaluation of it can examine the input beyond its start position: those it
  can consume first, and those that every lookahead it can evaluate before it
  consumes input can consume first, through rule calls. The first characters
  of the continuations include, where a continuation can match without
  consuming input, the first characters of what follows it. The first
  characters of a growing unit, compared with its own continuations, and of a
  growing unit that is a continuation of another growing unit, are those of
  the part after its left call. As a continuation of a unit that is not
  growing, a growing unit examines the input from that unit's start, through
  the seed.
- A unit and a continuation **share** a call when, at the position where the
  unit starts examining the input, both evaluate the same call before
  anything else that examines the input, directly or as the first thing that
  the bodies of the calls they start with evaluate: a call of the same rule with the
  same level restriction, or with none in both, of a rule that reads no
  variable and that is memoized at each of the two calls or calls no rule,
  itself included. A rule memoized in every parse is memoized at each of its
  calls, and a rule of a left-recursion cycle other than a leader at a call
  that is not a left call. The call has the same result in both: the memo
  answers it the second time, or the rule repeats only its own scan, so the
  continuation pays for it at most what the unit paid. In the same way the
  two share a literal that is not empty when every path of both that examines
  the input begins with it, and the literal has the same case sensitivity in
  both: both consume it or both fail where they begin, so the calls after it
  are made at the same position in both. Below, a shared literal counts among
  the shared calls. Calls of one rule with different level restrictions, such
  as `e` and `level(e, hi)`, are different calls with different results. An
  Operators Rule evaluates its `skip` before anything else at its start,
  before an operand or a prefix operator part, and continues at the same
  position when the `skip` does not match, so the call that its `skip`
  evaluates first is a call that a rule whose body is the Operators Rule
  evaluates first. This holds for a body that is the Operators Rule alone or
  under attributes other than `#recover` and `#missing`, which change what
  the parser does when the body fails. Because the parser continues when the
  `skip` does not match, a `skip` that cannot match without consuming input is
  optional wherever it is tried, before an operand or an operator part, and
  the first characters there include those of what follows it. A call
  of any other rule is not shared, because evaluating its body again also
  evaluates again the units inside it, at every level of a recursion.
  A call of a rule that is not memoized at the call and contains a scanning
  source is not shared either when the continuation reaches it after the end
  of a rule body, in what follows a call that **nests at one end**, or after
  an operand of an Operators Rule that nests. A call nests at one end when it
  is a call of a rule from a rule of its recursion that lies on a cycle of
  such calls, each followed in its caller by what can match without consuming
  input, so that the nested evaluations of the cycle can all end where the
  innermost one ends. Each of them that ends at the position then evaluates
  the call there again: in `e = "(" e? (ws ")")?` with `ws = [ ]*`, the
  option at each level fails after `ws` and the option of the level around it
  evaluates `ws` again over the same spaces, so the continuations together pay
  for the call once for each level. The same holds when the continuation
  reaches the call there after other calls or literals that the two share, as
  `ws` in `e = "(" e? ("x" ws ")")?`, where each level matches `x` again before
  it scans the same spaces, and in `e = "(" m e? (m ws ")")?` with `m = mx`,
  which calls a rule and is therefore memoized, where each level matches `m`
  again, from the memo once its entry is stored, and `ws` scans the spaces
  again.
- A unit is **committing** when, for each of its continuations, the first
  characters of the unit and of the continuation are disjoint after the calls
  that the two share are removed from their starts, one after another, each
  at the end of the one before. After
  a committing unit is abandoned, nothing that the parser evaluates next
  consumes the character at which the unit and the continuation part, so
  nothing starts again inside the input that the unit examined after its
  shared calls, and the shared calls are not repeated beyond their own cost.
  A unit is also committing for a continuation when no scanning source
  remains in it after those calls are removed: its scanning sources lie
  entirely inside the shared calls. The input that the unit examines beyond
  the shared calls, apart from what bounded scans examine, is then bounded by
  the other conditions, as for a unit without a scanning source, and the
  continuation pays for the shared calls no more than the unit paid, so a
  restart repeats no scan but a bounded one, whose iterations at each position
  are a constant however often it restarts.
  A unit is also committing for a continuation that its **words exclude**. The
  word of a path of a unit is the text with which the path begins after the
  shared calls, through groups, captures, discarded expressions `-e`,
  `#atomic`, actions, choices and calls of rules that read no variable and
  have no level restriction: a case-sensitive literal `w` that is not empty,
  optionally followed by a negative lookahead `!Y`, where `Y` is a character
  class or a call of a rule whose body is one, as in `"let" !idchar`. Every
  path of the unit must have a word. A unit that fails within a word examines
  no more of the input than the word and the character after it. After a unit
  matches a word, the input at its start begins with `w`, followed, when the
  word has `!Y`, by a character outside `Y` or by the end of the input.
  A word excludes a continuation when every path of the continuation,
  evaluated at the same position on that input, fails, examines nothing more,
  or evaluates a negative lookahead `!K` where `K` is known to match, before
  it reaches the end of `w`. Along a path, an expression that cannot examine
  the next character of `w` and can match without consuming input is passed
  over, and one that cannot match without consuming input fails there; a
  literal, a character class or `.` consumes the characters of `w` that it
  matches, and a literal fails where it differs from `w`; `!K` where `K` is
  known to fail is passed over. `K` is evaluated only when it is a choice of
  case-sensitive literals that are not empty, or one such literal, optionally
  followed by a negative lookahead of a character class, reached through
  groups, captures, `#atomic` and calls of rules that read no variable and
  have no level restriction, with no predicate, other lookahead, cut,
  `#recover`, action or level restriction in it. Such a `K` does not depend on
  how the parser reaches it. Its literals are tried in order against the rest
  of `w`: a literal that differs from it fails, the first that matches
  decides, and one that `w` ends inside of leaves `K` unknown. The lookahead
  of `K` is evaluated on the next character of `w`, or, at the end of `w`, it
  is known to succeed only when it has the same characters as `Y`. So
  `ident = !keyword [a-z]+`, with `keyword = ("if" / "let") !idchar`, is
  excluded at the start of a unit that begins with `"let" !idchar`, and a
  continuation `"=" !"="` at the start of an operator part `"=="` with its
  right operand.
  A unit that is not a lookahead is also committing for a continuation that
  it **decides**. A **path** of an expression is a sequence of symbols, each a
  call, a literal, a character class, `.` or a lookahead, such that every
  evaluation of the expression that succeeds succeeds on the symbols of one
  path in order, each starting where the one before it ended; an expression
  that is not looked into, such as `#recover` or an Operators Rule, ends the
  path without deciding anything. The unit decides the continuation when every
  path of the unit, compared with every path of the continuation from their
  common start, passes symbols that both evaluate alike and then **parts** at a
  decisive symbol that at most one of them can pass, within three symbols: the
  symbols passed alike and the decisive one together. Two symbols are alike
  when they are calls of one rule that is not on a left-recursion cycle and
  reads no variable, or reads variables and is called with the same variables
  on both sides, equal literals, equal character classes or `.`, or calls of
  one trivia rule (below). A literal that is a prefix of the other, literals
  that are equal up to case, overlapping character classes, and a literal and
  a class that contains its first character are passed by the characters that
  they have in common, each step counting as one symbol; a call that is not
  alike stands for the paths of the body of its rule, each of which must part.
  The decisive symbol is one on each side that cannot both match at the
  position: first characters that are disjoint, two literals of which neither
  is a prefix of the other, or two character classes with no character in
  common; a negative lookahead `!K` on one side that the rest of the other side
  must match, or a positive lookahead `&K` on one side whose expression and the
  next symbol of the other side are disjoint; or, in the continuation, the end
  of the parse, which costs a constant. The strict conditions make what the
  continuation evaluates again bounded:
  - every symbol passed alike is **cheap**: evaluating it again where the unit
    evaluated it costs the continuation at most a constant or what the unit
    paid for it there. An expression without a scanning source is cheap, as is
    a call that the memo answers, a call of a rule that calls no rule, which
    repeats only its own scan, and a call of a rule whose body is cheap in all
    its parts. A call of a rule that reads variables is cheap only where both
    sides have the same variables, which holds until either side passes a
    predicate that binds a variable or the end of a rule body that binds. A
    call that the memo does not answer and that contains a scanning source is
    not cheap when the continuation reaches it after the end of a rule body,
    in what follows a call that nests at one end, or after an operand of an
    Operators Rule that nests;
  - an option, choice, repetition or lookahead that a path passes is looked
    into only when all its parts are cheap;
  - a lookahead that only the continuation evaluates, or an alternative of it
    before the one that the unit's match implies, costs a constant: it has no
    scanning source of its own, and each call in it is answered by the memo or
    reaches no scanning source.

  A **trivia rule** is a rule whose body is a greedy repetition `X*`, atomic or
  not, where `X` cannot match without consuming input, and that neither it nor
  a rule that it calls runs an action or contains `#recover`, an attributed
  expression, a stream, an Operators Rule or a call of a left-recursion cycle,
  whose body binds no variable that it reads and has no `#cut` of its own. A
  call of it always succeeds, and the position where it ends depends only on
  the position where it starts and the variables that it reads, so two calls at
  one position with the same variables end alike, and they are passed without
  counting toward the three symbols. A call that follows a call of the same
  rule across symbols that consume nothing, with no predicate that binds a
  variable between them when the rule reads variables, starts where the first
  one ended, where `X` fails again, so it matches nothing and is dropped from
  the path. The dropped call costs a constant only when `X` has no scanning
  source, so it is not dropped otherwise.

  After a unit that its continuation decides is abandoned, the continuation
  evaluates again only cheap symbols before it parts. At the decisive symbol it
  does not consume the character at which the two part, fails within a cheap
  lookahead, or ends the parse, and a unit that ends after cheap symbols
  examined no more than the continuation evaluates again at their costs.
  Nothing then starts a scanning source again inside the input that the unit
  examined beyond the cheap symbols that both evaluate alike.
- A path of a later alternative of an alternative of a rule of a
  left-recursion cycle **enters the growth again** when it reaches a left call
  of a leader of the cycle before it examines the input, directly or through
  left calls of other rules of the cycle, as the last alternative `a` does in
  `c = "(" c ")" "y" / "(" c ")" "z" / a` with `a = c "+" / "n"`. The call of
  the leader examines nothing that the alternative examined: while a growth of
  the cycle is in progress at the position, it reads the growing result, and
  otherwise the memo answers it after the growth there has ended, or it starts
  that growth, once for each position. Each step of that growth evaluates the
  bodies of the cycle again at the position, the alternative included. The
  alternative is committing for such a path when all of the following hold:
  the alternative makes no left call of the cycle and evaluates its scanning
  sources only inside calls that are memoized at the call and read no
  variable; the bodies of the rules of the cycle evaluate in the same way
  every scanning source that they can evaluate before a left call of a leader
  that they always evaluate, at positions that are the same in each step; and
  what follows the call of the leader, through every call of a rule whose body
  it ends, up to the end of the parse, contains no scanning source. The steps
  after the first then find those calls in the memo, and what follows the
  call examines a bounded amount of the input. Otherwise the path is compared
  as any other.

A grammar's cost from a start rule is at most proportional to n, times a
constant that depends only on the grammar, when every rule that the start rule
reaches satisfies all of the following. The conditions are sufficient, not
necessary.

1. **Variables.** No binding of a variable reads, through any chain of
   bindings, captures and rule values, the variable's own earlier value, when
   the value it binds can reach a later evaluation of the same binding: when a
   sequence of the rule body has the binding in one element and a call that
   can reach its rule again in a later element, each at any depth of groups,
   options and choices, or when the binding is part of the element of a
   repetition or of an Operators Rule, which applies its parts again at each
   operator, outside a negative lookahead. A binding and a call in
   different alternatives of one choice are not on one path of evaluation,
   because a failed alternative undoes its bindings before the next is tried.
   A binding after the recursive call binds a value that no later evaluation
   of it sees, because bindings in a called rule are not visible to its
   caller. The variables with **finite values** are exempt from this
   condition: the `bool` variables, the `int` variables whose values form a
   finite set, the least set that holds the initial value and, for each of the
   variable's bindings, the values that the binding leaves bound from the values
   that can reach it, and the variables whose initial value and bindings are all
   constants, such as a `string` bound only to literals, each with at most 65536
   values. The value set of a `bool` holds its initial value and the
   values that its bindings can bind: a constant binding adds its value, a
   binding that fails to evaluate adds none, and any other adds both, so a
   `bool` never bound, or bound only to its initial value, counts 1. A variable
   bound only to constants holds at most one value for each distinct constant.
   The value sets are taken from the smallest up while the product of their
   sizes is at most $2^{32}$; a variable whose set would bring the product past
   that has no finite values. Its bindings are then subject to the condition
   above: bindings that satisfy it, such as bindings to constants, make the
   cost linear given an environment bound, while a binding that reads the
   variable's own earlier value, as a counter `{u := u + 1}` before a
   recursive call, fails it. No value set depends on another variable, so a
   variable without finite values leaves the others as they are. A binding of
   an `int` variable whose values the classifier evaluates reads only the
   variable itself, `int` and `bool` literals, and operands `len($c)` of a
   capture `c` whose value has a length of at most 256 that the classifier
   can see: the text of an expression that matches a bounded number of characters, a list
   or tuple with a bounded number of elements, or the value of a rule whose
   action gives such a value. The tuple of a sequence has one element for each
   item, where a plain group of a sequence splices its own elements into it,
   so `x:("#" ("a" "b"))` has 3, while a capture, an option, a choice, a
   repetition or a call is one element. It combines them with `+`, `-`, `*`, `/`, `%`,
   comparisons, `&&`, `||`, unary `-` and `!` and parentheses. The binding is
   evaluated at every length of each operand up to its bound, at most 4096
   combinations, and a value whose evaluation is an error, as a division by
   zero, is not bound; a value beyond $2^{53}$ in magnitude is not one that the
   classifier evaluates. A binding that reads anything else, or whose value it
   does not evaluate, binds the values within the bounds of its **guard**, and
   leaves the variable without finite values when the guard does not bound
   them.

   The guard of a binding `v := e` is made of the tests that run next to it
   with only predicates between them, so that nothing between them reads the
   input or calls a rule. Its **tests after** are the predicates or block
   statements after the binding, up to the first element that is not a
   predicate or the next binding of `v`, that read only `v` and literals, as in
   `{v := e} {lo <= v && v <= hi}` or `{v := e; v <= hi}`. A failed test fails
   the sequence that holds the binding, and backtracking undoes the binding,
   so a value that a test rejects reaches no call and no later binding; the
   binding leaves bound only the values on which every test after it can
   succeed. Its **tests before** are the predicates or block statements
   before the binding, back to the first element that is not a predicate or
   to a binding of any variable, as in `{lo <= e && e <= hi} {v := e}`: they
   evaluate `e`, written alike, with the same variables and captures as the
   binding, and every term of the action language is a function of those, so
   the binding binds only values on which the tests succeed. A predicate with
   attributes counts as a predicate when all of them are `@` attributes, such
   as `@error`, which do not change matching; a predicate under a pragma, such
   as `#recover`, does not, since a recovery can succeed after the test fails.
   Each test that is a comparison with `<`, `<=`, `>`, `>=` or `==` of `v`, for
   a test after, or of `e`, for a test before, with a constant `int`
   expression on either side, or a conjunction with `&&` of such comparisons
   and other tests, bounds the value on one side or both. A side that no test
   bounds is bounded by the range of `e`: a literal is its value, `v` ranges
   over the values that reach the binding, `len` gives at least 0, and unary
   `-`, and binary `+`, `-` and `*`, combine the ranges of their operands. So
   `x:"#"+ {v := len($x)} {v < 10}` binds the values 0 to 9, while
   `{v := v + len($x)} r {v < 10}`, with the call `r` between the binding and
   the test, `({v := v + len($x)} / {v := 0}) {v < 10}`, where the test is not
   next to the binding, and `{v := v + len($x)} {v <= w}`, which compares with
   a variable, bound nothing.

   The values that **reach** a binding are those of the
   set after the earlier elements of the sequences around the binding, from
   the outermost sequence that it is in through groups, options, choices,
   captures, discards and atomic expressions only, and after the earlier
   statements of its predicate block. A binding of the variable applies to the
   values as above; a predicate that reads only the variable and literals keeps the
   values on which it can succeed; a choice or an option joins the values of
   its paths; `@` attributes change nothing; a negative lookahead keeps the values before it, since it undoes
   its bindings; a call keeps them, since bindings in a called rule are not
   visible to its caller; and any other element that binds the variable, as a
   repetition or a positive lookahead around a binding, can leave any value of
   the set. So in `("0" {v := 64} / {v := 0}) {v := v + 2}` the second binding
   reads only 64 and 0, and in `{v < 3} {v := v + 1} r` the binding reads only
   values below 3, while a test after the call `r` filters nothing that the
   binding reads. When every binding is a literal or binds a variable
   with finite values, a constant number of combinations of values occurs at a
   position, at most the product of the sizes of the value sets, which the
   class Linear reports as its bound on K (see [Classification](#classification)).
   When another binding reads a capture or a variable, the cost is
   linear given an environment bound.
2. **Repeated scans.** Every unit that contains a scanning source and can
   fail after one of its scanning sources has examined input is committing. A
   unit can fail after a source when something that can fail is evaluated
   after the source: a later element of a sequence that contains the source, a
   later required iteration of a repetition that contains it, or a negative
   lookahead around it, in the unit or in the body of a rule that it calls. A
   source that is a copying operation in a predicate can make the predicate
   itself fail. A unit that
   fails only before its sources have examined input, such as an iteration of
   `([^"]+ / "\"\"")*`, gives back to its continuation no more input than its
   parts before the sources examined, which the other conditions bound as for
   a unit without a scanning source; a unit that succeeds is not restarted, and
   a failure after it is a failure of an enclosing unit, which this condition
   checks in turn. The expression of a lookahead is abandoned even when it
   matches, and a growing unit and a unit of an Operators Rule are checked
   whether or not they can fail after a source. A unit whose only scanning
   sources are bounded scans contains no scanning source in this sense: a
   restart repeats them, and their bound counts the repetition.
3. **Growth.** No scanning source can be evaluated, outside the left call that
   grows, at the position where a left-recursive rule grows, in its body or in
   the body of another rule of its cycle, other than inside a call, memoized
   at the call, of a rule that is not on the cycle. Each step of the growth
   evaluates these bodies again at that position.
4. **Copies.** No copying operation copies a string or list whose size can
   include the size of the value of a call on a recursion with the rule that
   performs the operation, nor a fold accumulator: the first parameter of the
   lambda of `foldl` or `foldr` over a list that is not bounded. A list is
   **bounded** when its number of elements is bounded by a constant of the
   grammar that the classifier can see: the value of a capture of an
   expression whose matches are bounded in length, or of a repetition with an
   upper bound, such as `x:a #repeat(,2)`, through choices and options; a
   `list(...)` or a list literal; and a value built only from bounded lists
   and literals, by `concat`, `map`, `filter` or a fold, whose list operands
   are not a variable, a lambda parameter or the value of a call on a
   recursion with the rule; the lambda may read them, so `map($b, (y) => y)`
   is bounded when `$b` is. The
   size is the one that the operation copies: the
   characters of a string, or the elements of a list. A value includes the
   value of such a call when it is the value of the call, passes it on, as `$x`
   does, is an element of it, as `$x[0]` is, or is built from it by `+`,
   `text`, `concat`, `map` or `filter`, or by a fold over a bounded list whose
   initial value or lambda includes it. So the text of a capture of the
   recursion includes the text of the calls nested in it, which counts where
   the text is joined or compared, while a list of nodes of the recursion, as
   built by `list($x)`, has one element per node whatever the node contains;
   `concat(list($x), $t)` with `t` the list of the rule's own recursion
   includes it. A value of a rule on a cycle of such values, and the value of
   an Operators Rule, are taken to include it. A copy whose size does not include
   such a value does not add up over the levels of the recursion; when it is
   not bounded by a constant, it is a scanning source, which condition 2
   covers. No copying operation copies a value that is not bounded by a
   constant and whose size can come from a capture made in a positive
   lookahead, directly, as a member or a positional capture, or through the
   values of the rules and captures that it reads, measured as above, so that
   `list($x)` has one element whatever `$x` holds, and whatever input those
   rules consume. Such a capture holds input that the match does not consume,
   so it can be as large at every position: in
   `A = "a" &(x:W) => text($x) + "!"` each iteration of `A*` copies the rest of
   the input, and so it does in `A = "a" y:L => text($y) + "!"` with
   `L = &(x:W) => $x`, a rule that consumes nothing; the classification
   reports the class unknown for such a copy.
   A copying operation in the lambda of `map`, `filter`, `foldl` or
   `foldr` over a list that is not bounded runs once for each element, as in a
   repetition: it copies nothing from outside the lambda that is not bounded in
   size by a constant of the grammar, and the elements that it copies add up,
   over all the evaluations of the lambda, to at most a constant times n. In
   each evaluation of the call they add up to the size of its list, and the
   call is evaluated once for each element of the lists of the lambdas around
   it that are not bounded, except that the elements of the innermost of them
   add up to at most the input when they are own and the list of the call
   reads only them; so the degree of the sum is the size degree of the list
   plus the size degrees of the numbers of elements of those lists, and it is
   not known when one of them is not. The lambda over a bounded list runs at
   most a constant number of times, so its copying operations are checked as
   copies outside it, in the lambdas around it, and the first parameter of a
   fold over it holds what the initial value and the value of the lambda hold,
   at most a constant multiple of their sizes, unless the fold can double it.
   The classifier bounds one step of a fold by its **step matrix**: it bounds
   the size, the size of the largest element and the number of elements of
   the value of the lambda each by a sum of terms, each a constant times one
   of the same three quantities of the first parameter, or times none, and
   times powers of n and of the size of the element of the step, where a value
   read from outside the fold counts as n to its size degree. The size of a
   list is one more than its number of elements plus the sizes of its
   elements. A list, a tuple or a
   struct adds the quantities of its items, and `+` and `concat` add those of
   their operands; an element taken by `[]` is at most the largest element,
   whatever its index; a member is at most the value it is taken from; a
   `filter` counts as its whole list, since it can keep every element; a
   `map` holds, for each element of its list, the value of its lambda for that
   element, so a part of the lambda that does not depend on the element counts
   once for each element; a fold inside the lambda that does not read its own
   first parameter holds its initial value or the value of its lambda, and one
   that joins its own first parameter once by `concat` or `+` with values that
   do not read it holds its initial value and those values summed over the
   elements of its list. A part that multiplies two quantities of the first
   parameter, as the first parameter read inside a lambda over a list that
   grows with it, and a term whose value the classifier does not follow, such
   as a block or a fold of another form, count as many copies of what they
   read. The fold can double its first parameter when, in the graph of the
   three quantities with an edge for each term that reads one, a cycle has a
   coefficient above 1 or a factor of a size, or two cycles share a quantity,
   as `acc + acc`, `list(acc[0] + acc[0])`, `map(acc, (e) => e + e)`,
   `map($c, (z) => acc)` and
   `foldl(list(), map(acc, (e) => list(e, e)), (s, l) => concat(s, l))` do.
   Such a fold is checked as a fold over a list that is not bounded. Otherwise
   each quantity is bounded over all the steps by a polynomial in the size of
   the input: keeping one element twice, as
   `foldl(list(), acc, (s, l) => list(l, l))` does, does not grow the first
   parameter; adding to every element, as `map(acc, (e) => e + "x")` does,
   grows the size by the number of elements on each step; and adding a value
   read from outside, as `acc + text($c)` does, grows it by up to the size of
   that value on each step. The quantities start from those of the initial
   value, by their size degrees; the steps are the elements of the list, and
   the sizes of their elements add up to the size of the list, by the size
   degrees of the three quantities of the list. The classifier bounds the
   cost of one step by such terms as well, where a term may multiply
   quantities: a comparison of values
   that are not ints and `len` of a string examine their operands, and a fold
   inside the lambda copies its own first parameter on each of its steps. A
   fold inside the lambda of a call over a list that is not bounded runs once
   for each element of that list, so the degree of its cost and of each copy
   of its first parameter gains the size degree of the number of elements of
   each such list around it, but not that of the innermost of them when its
   list reads only elements of that list and those are own captures of a
   repetition, distinct parts of the input whose sizes add up to at most the
   input. A fold over a list that is not bounded that reads a value whose
   size degree is not known fails this condition. A copying
   operation that copies the value of a variable is linear given an
   environment bound.
5. **Carried growth.** No leader R grows, at the position where another leader
   S of its cycle grows, from a result that includes the growing result of S,
   where the step of S that calls R can drop the result of R and still grow,
   and the growth of R can go on over input with which the growth of S goes on.
   Each step of S reads another growing result, so no two steps share the
   growth of R, which covers again the input that later steps cover. The step
   can drop the result when the call is in a lookahead, an option, a
   repetition or the base of a `#recover`, in an alternative followed by an
   alternative that begins with a left call, or in a rule of the cycle that is
   not a leader. The growth of R can go on with the growth of S when what
   follows the left calls of S in the body of R, followed by what follows R,
   and what follows the reads of the growing results of the cycle outside the
   body of R can begin with the same character. Otherwise R grows past the
   result of S only where the growth of S stops.

### Classification

`zp lint -classify` (`lint.Config.Classify` in Go) classifies the cost of
the start rule that it checks and shows the class. The classes are ordered
from best to worst, as in the table: each is a weaker claim than the classes
before it. A grammar has the worst class of its causes. A witnessed
polynomial or exponential cause outranks a reason for the class unknown,
because it is evidence of growth, while unknown is only the absence of a
proof. The classes polynomial and exponential produce a
[`super-linear`](lint.md#super-linear) hint at each cause, with its witness.
The other classes produce no finding; the reasons for the class unknown are
shown with the classification. Without the request, there is no
classification and no `super-linear` finding.

| Class | Promise |
|:--|:--|
| Linear | Every parse from the start rule has a cost of at most c · (n + 1) |
| Linear given an environment bound | Every parse has a cost of at most c · K · (L + 1) · (n + 1) |
| Unknown | None; the classifier cannot bound the cost |
| Polynomial of degree k | None; an estimate |
| Exponential | None; an estimate |

In a [stream](streaming.md), where `text` costs the length of its result (see
[Cost](#cost)), the promises of the first two classes add the total length of
the results of `text`.

The constant c depends only on the grammar and on the backend. K is the largest
number of distinct combinations of the values of the variables that memoized
rules read, among the calls at one position of the parse. L is the largest size
of a variable's value that a copying operation copies or that a memo lookup
compares; it is zero when no variable holds a string. Under the conditions, a
variable's value is built from literals, captures and other variables without
feeding back into itself, so L is at most proportional to n, and the cost of
the second class is at most proportional to K · n². A caller that needs a cost
proportional to n needs the class Linear.

In the class Linear, c includes K, and the classification reports a bound on
it: the product of the sizes of the value sets of the variables with
[finite values](#conditions-for-linear-cost), 1 when there are none. The bound
is a constant of the grammar, but a large one shows that the memo can keep that
many results for one rule at one position: until the input is long enough for
the parse to reach them all, the cost can grow faster than n.

The first two classes are guarantees under the memoization that
[Evaluations of Rules](#evaluations-of-rules) describes and the skip runs that
[Skip Runs](#skip-runs) describes; an implementation or a parse that keeps
fewer of either, as one can for testing and as a parse with missing-token
insertion does, is outside them. The
classifier reports them only when the
conditions of [Conditions for Linear Cost](#conditions-for-linear-cost) hold,
and it over-approximates where it cannot be exact. It assumes that every
predicate can both succeed and fail, that every continuation is reachable, and
that first characters do not depend on the path that reaches an expression.

The other classes carry no promise. Deciding the cost of a grammar exactly is
impossible, because it would decide whether a grammar's language is empty, so
a class worse than linear can be pessimistic: no slow input need exist. The
classes polynomial and exponential come with a **witness**, a path of rule
calls and expressions that shows the cause, and the class unknown comes with a
reason:

- **Polynomial.** A scanning source in a unit that is not committing, the
  continuation that can start again inside it, and the enclosing repetition or
  recursion. The estimated degree is one more than the number of such sources
  nested in one another through rule calls, so that a repetition restarted
  inside a restarted repetition gives 3. A copying operation whose size
  includes the value of a call on its own recursion, or a fold accumulator,
  gives 2, and so does a scanning source evaluated in each step of
  a growth. A fold over a list that is not bounded gives the degree in the size
  of the input of the cost of all its steps, which the step matrix of
  condition 4 bounds, when that is 3 or more: a term of the cost adds the
  degrees of the quantities that it multiplies, the size degrees of the values
  read from outside, and the degree of the number of steps, or of the sum of
  the powers of the element, for the sum over the steps. A fold inside
  lambdas over lists that are not bounded adds to either degree the size
  degrees of the numbers of elements of those lists, as condition 4
  describes, so that `map($b, (y) => foldl("", $b, (s, l) => s + text(l)))`
  gives 3. A copying operation in such a lambda whose copies of elements add
  up to more than linear size gives the degree of their sum, as condition 4
  describes, so that `map(map($b, (q) => text($b)), (y) => y + "x")` gives
  2.
- **Exponential.** A binding that makes a variable depend on its own earlier
  value, or a copying operation that copies a value of its own recursion, or a
  fold accumulator, more than once, as `s + s` does, or a fold whose step
  matrix of condition 4 can double its first parameter, though no single
  copying operation copies it twice, as by duplicating its elements and
  joining them. The step matrix bounds the growth from above, so a fold that
  it classes exponential can grow more slowly.
- **Unknown.** A condition for linear cost fails without a witness of a
  polynomial or exponential cause, as when sources that are not committing are nested in one
  another without bound through a recursion of rule calls. Memoized calls in
  the recursion can keep such a grammar linear, which the classifier does not
  decide. Its reason is the first unit that is not committing, or the
  condition that fails, with the expression where it fails.

### Memory

- **Memo table.** It holds at most one entry per memoized rule, position,
  binding level and combination of variable values, and a lookup finds one. The
  table of a [Document](documents.md) keeps the entries of earlier parses, and
  a deletion can bring entries of one key to a position, at most one more for
  each edit since the table was last dropped. While growths are in
  progress at a position, the results there that read their growing results
  are kept apart, one for each combination of the growing results and
  lookahead outcomes that they read, and dropped when the first of those
  growths ends; each is built by counted cost.
- **Values.** A parse keeps the value of its start rule. Values built by
  abandoned alternatives are not kept, but their construction is cost.
- **Call stack.** It holds at most as many calls as the [nesting limit](#limits)
  allows.
- **Streams and Documents.** A [stream](streaming.md) parse discards memo entries
  and input before the elements that it has delivered. A
  [Document](documents.md) keeps memo entries and repetition records for its
  whole text between parses.

### Bounding Cost

[Compute limits](#cancellation-and-compute-limits) bound the cost of any parse,
whatever its class. `Compute` is one counter for all of it: it counts the input
that terminals, repetitions of character classes and lookaheads examine,
including input examined again, rule calls, repetition iterations, growth
steps, and the strings and lists that actions and predicates build, in
proportion to their size, except texts, which cost what [Cost](#cost) states. Each unit adds one to the counter, so a loop that
only scans input, a deep recursion and a value that grows without reading the
input are all bounded by it.

## Limits

An implementation MAY limit the nesting depth of recursion, so that input of
unbounded nesting cannot exhaust the memory of the host. A rule is
**recursive** if it is on a cycle of the call graph: it can reach itself through
rule references, directly or through other rules (a rule that references only
itself is on a cycle). The nesting depth is the number of entries of recursive
rules that are active at once, plus the entries of Operators Rules (below),
whether or not they are recursive; the entry of any other rule does not count.
A rule outside every cycle appears at most once on the call stack of a parse, so
the rules that do not count add a bounded depth that depends only on the
grammar. The value-free twin of a rule is as recursive as the rule.

The operand of a prefix operator and the right operand of an infix operator of
an Operators Rule nest like entries of recursive rules, and so does the entry of
an Operators Rule; chains of left-associative operators do not nest. When the
limit is exceeded, the parse fails with a run-time error and no value. In the
reference implementation the error is a `LimitError` of kind `LimitDepth`, whose
message is `nesting too deep: more than N nested recursive rule calls`; the default
limit is 100,000 (10,000,000 for the iterative bytecode VM), and the Go API changes
it with `Limits.Depth`. `Unlimited` disables the limit on the iterative bytecode VM
only, whose call stack is on the heap; on a backend that nests rule calls on the
host stack it is an option error. Every backend reaches the limit at the same
nesting of the same input.

## Cancellation and Compute Limits

The reference Go API accepts invocation-local cancellation and a compute limit
through `WithContext` and `WithLimits`; generated Go exposes corresponding
package-local options.
These controls are cooperative and backend-local. A zero limit selects the
[default of that limit](#default-limits) and the value `Unlimited` disables
it. The budget counter is monotonic for one invocation and resets on the next
call. A reservation is checked before the corresponding work begins; if it
exceeds the remaining allowance, that work is not performed. Decoding is the
one exception: a reservation may follow the decode of at most one character (at
most 4 bytes) and must precede storing or using that character. Compute counts
are implementation-specific and are not elapsed-time, memory, or buffered-input
limits; the [memory limit](#memory-limit) counts bytes.

### Default Limits

`Limits` has four fields, and a zero value selects the default of its field:

| Field | Bounds | Default |
| --- | --- | --- |
| `Compute` | the cost of the parse | `max(2^20, 1024 * n)` |
| `Memory` | the storage the parse takes into use, in bytes ([Memory Limit](#memory-limit)) | `Unlimited` |
| `Input` | the length of the input | `Unlimited` |
| `Depth` | the nesting of recursive rule calls ([Limits](#limits)) | 100,000; 10,000,000 on the iterative bytecode VM; the generated default |

Here `n` is the length of the input in the position unit of the parse: bytes
for `Bytes`, code points for `CodePoints`. `Input` has no finite default
because the default `Compute` already grows with `n`, and `Memory` has none
because the memory a parse allocates is at most proportional to its cost
plus the input ([Time and Memory](#time-and-memory)).

`WithLimits` sets the fields field by field. A nonzero field overrides the
value an earlier `WithLimits` of the same invocation set, and a zero field
leaves it unchanged; a field that no option sets takes its default. The
options of a `Document` call apply in the same way over the limits the
`Document` was constructed with. No value resets a field to its default once
an option has set it.

Every parse has a budget. A
parse that has neither a limit nor a context therefore runs controlled. The
value `Unlimited` (the largest `uint64` in Go, `Number.MAX_SAFE_INTEGER` in
TypeScript) disables the budget; with no context and no finite `Memory`
either, the parse runs without a control. `LimitError.Limit` reports the resolved limit, and the
default is `Unlimited - 1` at most. One constant disables every field of
`Limits`, and a field added to `Limits` takes the same zero and `Unlimited`
meaning.

`Input` bounds the length of the input in the position unit. A batch parse,
a recognition and the creation of a Document compare the length of the input
with it before any parsing work; a Document does so again at every `Parse`, for
the text it holds after its edits. An input of exactly `Input` units is
accepted. A longer input fails with a `LimitError` of kind `LimitInput`: `Limit`
is `Input`, `Used` is zero and `Requested` is the length of the input. The
check costs O(1), except that `CodePoints` counts the code points of an input
whose byte length passes the limit. It needs no budget and no context: a
finite `Input` alone does not make a parse controlled. It is checked before
a context is consulted, so an input over the limit fails with `LimitInput` even
if the context is already canceled. A generated parser checks it whether or
not it has limit controls.

A stream does not know its length. With a finite `Input`, it fails when the
parser asks for a unit beyond the limit and the stream has one, and `Requested`
is the number of units the parser asked for, or the length of the stream if that
is shorter. A stream of exactly `Input` units completes. As with the default
budget, the outcome does not depend on how the reader splits the input into
reads. With `Input` at its default, a stream tracks no length.

The default is resolved once per entry point, from the input the entry point
receives. For a `Document`, `n` is the length of the text at each `Parse`. A
stream does not know `n` up front, so its default is `max(2^20, 1024 * d)`,
where `d` is the number of units up to the furthest position the parser has
demanded from the stream so far, and it never shrinks. `d` depends on the input
and the grammar, not on how the reader splits the input into reads. A parser generated
without limit controls spends no budget and accepts only a zero `Compute` and
`Unlimited`.

Cancellation and exhausted limits are fatal interruptions, not ordinary
syntactic mismatches. Ordered choice, predicates, recovery, and other grammar
constructs cannot absorb them. An interrupted batch parse returns no partial
Node or typed value. In streaming mode, elements already delivered to the
caller remain delivered; the parser does not retract them. A blocked reader or
caller callback cannot be preempted by a synchronous parser; cancellation is
observed after the call returns.

For a `Document`, a cancellation or option error detected before execution has
touched reusable state preserves its valid caches and last statistics. Once
execution starts, an interruption clears execution memo/repetition state before
the next attempt, because entries can depend on unfinished left-recursion work.
Fully completed input tables may be retained, but incomplete tables are never
published and are discarded. A retry with fresh invocation controls must produce
the same result as parsing the current text from scratch.

In the Go API, `WithContext(ctx)` supplies the context and
`WithLimits(Limits{Compute})` the limit. The compute counter adds the
inspected elements of the input representation and the backend's execution
steps, one unit each. Charges are not rolled back by backtracking, and their values
depend on the backend, the position unit and the generated artifact. For a given
artifact, input and options they are the same on every run: the storage that an
earlier parse left in a reused parser, and when a collection releases it, never
change them. A stream also charges one unit for each call to its reader's
`Read`, so its spend depends on how the reader splits the input as well; its
default limit does not. Reading
input charges the budget counter in proportion to the elements examined, so a
loop that only scans input, such as a repeated character class, is bounded by
`Compute`. A cancelled
context returns the error of the context (`errors.Is(err, context.Canceled)` or
`context.DeadlineExceeded`); an exhausted limit returns an error that matches
`zp.ErrLimit`, and `errors.As` with `*zp.LimitError` gives the
details. Stream parses and documents are specified in [Streaming](streaming.md)
and [Incremental Documents](documents.md).

### Memory Limit

`Memory` bounds the storage a parse takes into use, in bytes. The count is
the element count of each buffer the parser takes into use times the size of
its element type, and for a map the size of its keys and values: node, list,
field and capture chunks, memo entries, slots, first-call sets and hash
indexes, typed values, the decoded input and its line table, a stream's
buffers, the working stacks by their capacity, the records of left recursion,
context memoization, skipped runs, missing-token insertion and Document
reuse, the variable environments and syntax errors the parse builds, and the
strings that actions concatenate and the text they copy out of the input. Not
counted are the caller's input string, which `Input` bounds; the compiled
grammar and other state shared by every parse of it; values handed to the
caller, such as the elements a stream delivers, after they are handed over;
trace and observation output, which goes to the caller's sink; and what the
allocator adds beyond the logical size, such as size-class rounding and map
overhead. Storage abandoned by backtracking is not returned; a stream
returns the storage of the elements it has delivered, so the count of a stream
bounds its window, not its length. Recovered errors are the exception: a stream
retains them for its result unless a diagnostic limit keeps only the latest
ones, so without one they grow the count of a stream
([Streaming](streaming.md#memory)). Counts depend on the backend, the position
unit and the generated artifact, and for a given artifact, input and options
they are the same on every run, whatever an earlier parse left in a reused
parser.

A finite `Memory` makes a parse controlled, and the control then polls every
256 compute units even without a context. At a poll, the context is
consulted first, then the memory count, then the compute counter; the context
and the memory count are checked again at the end of the parse. A count above `Memory` fails the parse with a `LimitError` of kind
`LimitMemory`, message `parse memory limit exceeded: limit N, used M`, whose
`Limit` is `Memory`, `Used` the count, and `Requested` zero: memory is checked
at polls, not reserved before an allocation, so the count can pass `Memory` by
what the parse allocates between two polls. A parse whose final count is at
most `Memory` succeeds.

A `Document` parse counts the caches the Document retains from earlier parses
from its start. If they alone exceed `Memory`, the Document drops them before
the parse, as `ResetHistory` does, and parses from scratch. Retained caches
within `Memory` are kept, so a parse that reuses them can fail where a parse
from scratch would fit; the interruption clears them, and the next parse starts
without them.

The reference engine and generated Go have the memory limit; generated
TypeScript has no `memory` limit.

## Modes

The same grammar can be run in several modes. Except where stated, each mode
gives the results that an ordinary parse gives.

| Mode | Difference from an ordinary parse |
|:--|:--|
| Backends | The reference implementation has three backends (closure, recursive bytecode and iterative bytecode). They give identical values, errors and positions, and differ in speed and in the limit of nesting depth. Generated Go and TypeScript parsers implement the same matching semantics; the entry points, features and limits that differ are defined in [Generated Parsers](generated-parsers.md) |
| Recognition | The parse only tells whether the input matches. It gives the same success or failure and the same syntax errors, builds no tree, and skips actions whose values are not needed by predicates. Actions needed by predicates still run and can report run-time errors. The captures that [predicates](../language/predicates.md) read are still built |
| Stream | A parse that reads its input incrementally and hands each element of the [`#stream`](../language/pragmas.md#stream) repetition of the start rule to the caller as it matches. The elements are not part of the value. See [Streaming](streaming.md). Generated TypeScript parsers have no stream mode |
| Incremental | A document that is edited and reparsed reuses results of the previous parse. The result of a parse is the same as that of a parse of the whole new text. See [Incremental Documents](documents.md). Generated TypeScript parsers have no document, and generated Go parsers support a subset |
| Prefix | A named rule is parsed at the beginning of the input without requiring the whole input to be consumed. See [Prefix Parsing](#prefix-parsing) |
