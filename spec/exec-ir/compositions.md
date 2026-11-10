# Compositions

A composition is a function of Exec IR that every runtime shares: an algorithm that each runtime would otherwise
write for itself, written once over the operations of the parser VM. A composition declares the features it can be
specialized for, and a runtime prints the specializations it needs (see [Specializations](#specializations)). The
compositions are listed below by the mechanism they write, each with its declaration as the
[text form](text-form.md#catalog-entries) writes it and its meaning.

## Calls

The call protocol of a rule: deferred memoization, the memo with its reuse conditions, the dependency memo of left-recursion contexts, store and restore on failure, nesting, statistics, observation and the bookkeeping of a Document.

| Declaration | Meaning |
|:--|:--|
| `func call(r Rule, min int) (Value, bool) [document, observe, typed, budget]` | Calls rule r at binding level min: unmemoized while its memoization is deferred, and otherwise through the memo and the dependency memo of left-recursion contexts, growing the seed of a left-recursion leader. |
| `func callMemo(r Rule, inv Body) (Value, bool) [document, observe, typed]` | The memoized part of a rule call, for the rule r at binding level 0 invoked by inv: the rule is called at the position again, or its memoization is no longer deferred. |
| `func grow(r Rule, min int) (Value, bool) [typed]` | Grows the seed of a left-recursive leader from a failure: it evaluates the body again while the match gets longer, and returns the last longer result. |
| `func observed(r Rule, min int) (Value, bool) [document, observe]` | A rule call in an observed parse: it reports the call to the observation, with how far it examined the input in a Document parse, around the call without observation. |
| `func unmemoized(r Rule, min int) (Value, bool)` | Calls rule r at binding level min without memo bookkeeping, as call does while the rule's memoization is deferred: its expectations need no isolation, and a failure restores the position and the recovered errors. |
| `func memoized(r Rule, min int) (Value, bool) [document, observe, budget]` | Calls rule r at binding level min through the memo, as call does once the rule's memoization is no longer deferred, with the observation of a reuse and, in a Document parse, the bookkeeping of how far each call examined the input. |
| `func document(r Rule, min int) (Value, bool) [document, budget]` | A rule call in a Document parse, which memoizes the rules that the Document reuses and records the input each call examined. The enclosing call's examined input is restored however the call ends, an interruption included, so that a parse after it starts from the state it would start from otherwise. |
| `func typedDocument(r Rule, min int, inv Body) (Value, bool) [document, observe, typed, budget]` | A typed rule call in a Document parse, for the rule r at binding level min, invoked by inv when a generated call passes it: unmemoized for a rule that the Document does not memoize, and otherwise through the memo, with the Document record of each entry kept beside it. As in document, the enclosing call's frontier is restored however the call ends. |
| `func callBegin(r Rule, min int, st CallState) (Value, bool, bool) [document, observe, missing, budget]` | Begins a memoized call of rule r at binding level min, with the engine's bookkeeping of the input each call examines: it returns hit set with a result that the call reuses, and otherwise saves in st what the end of the call needs. |
| `func callEnd(r Rule, st CallState, v Value, ok bool) (Value, bool) [document, observe, missing, budget]` | Ends the memoized call of rule r whose state its begin saved in st with the value v and success ok of its body, or of the growth of a leader: it records the result where its dependencies send it, and restores the state from before the call. |
| `func engineCall(r Rule, min int) (Value, bool) [document, observe, missing, budget]` | Calls rule r at binding level min in the engine: unmemoized while its memoization is deferred, and otherwise between the begin and the end of the split call protocol, growing the seed of a left-recursion leader. A parse that reads a Document makes the call of `engineCallDoc` instead, which is decided by one test of a field that the parse sets once. |
| `func engineCallDoc(r Rule, min int) (Value, bool) [document, observe, missing, budget]` | `engineCall` for a parse that reads a Document. Under the default limit, what a parse from scratch does not do is Document work and leaves the count against the limit: the unit of the call protocol for a plain rule, and the memo protocol (its deferral, the begin and the end of the call) of a rule that a parse from scratch does not memoize. |
| `func body(r Rule, min int) (Value, bool) [document, budget]` | A rule call on the tracked path without observation: the Document call in a Document parse, and otherwise the unmemoized call while the rule's memoization is deferred and the memoized call after. |
| `func tracked(r Rule, min int) (Value, bool) [document, observe]` | A rule call in an observed or Document parse: the observed call in an observed parse, and otherwise the call without observation. |

## Expectations

Recording an expectation, keeping a set and merging a set at a position.

| Declaration | Meaning |
|:--|:--|
| `func expect(pos int, id ExpID) [document, budget]` | Records that id was expected at pos: an expectation before the farthest failure or in a lookahead is dropped, one past it starts the record anew, and one at it joins the entries. A form specialized for Document parses also extends the examined frontier of the call past pos; a form without it leaves that out. |
| `func expectOrdinary(pos int, id ExpID) [budget]` | `expect` in an ordinary parse: one without limit controls and outside a Document, for the branch of a generated rule body that runs in such parses and leaves out the budget charges and the Document's record of the examined input. Printed only for the generated Go runtime, which parsers of nodes and of typed values share, so that only the nodes column lists it. |
| `func expectSlow(pos int, id ExpID) [budget]` | Records the expectation of `expectOrdinary` at or beyond the farthest position. It is separate so that the guard of `expectOrdinary` inlines into the generated code. Printed only for the generated Go runtime, which parsers of nodes and of typed values share, so that only the nodes column lists it. |
| `func mergeKept(far int, k ExpKept) [document, budget]` | Records the expectations that a memo entry kept at far, as expect does for each of them. |
| `func mergeExpected(far int, s ExpSet) [document, budget]` | Records the expectations of s at far, as expect does for each of them, so that the expectations of a call that a memo entry or a finished isolation kept become those of the enclosing record. s may alias the stack above the current record: writes only happen below the read position. |
| `func keep(s ExpSet) ExpKept [budget]` | Returns the expectations of s as a set that outlives the record, for a memo entry, a context or a recovery to keep: the empty set for none, the set kept last when it has the same entries in the same order, and otherwise a copy, which becomes the set kept last. |

## Streams

The delivery of the elements of a `#stream` repetition and the commitment of the input before them.

| Declaration | Meaning |
|:--|:--|
| `func streamCommit(pos int) [stream]` | Makes it final that the parse never backtracks before pos: the input before pos - 1 is discarded (the preceding unit stays for detecting line starts), the memory the committed elements held is released, and the memo and the records before pos are pruned once at least a window of input was committed since the last pruning. |
| `func streamEmit(v Value, end int) [stream]` | Passes a committed element v of the start rule's #stream repetition to the caller and commits the input before end. The recovered errors committed so far are reported first, so that an error of the diagnostic callback ends the parse before the element is delivered. Elements are final once emitted: a caller error ends the parse unchanged. |
| `func streamsIn(id int, rec bool) bool [stream]` | Reports whether the #stream repetition of the rule id streams: the parse delivers elements, the rule is its start rule, and the rule runs at the top (a recursive rule counts its entries, so its outermost entry is the first; any other rule is entered once). |

## Operators Rules

The loop of an Operators Rule: the longest operator match, binding levels, prefix commitment and the order of application. The functions that end in `Begin`, `Next`, `Step`, `Prefix`, `Rhs`, `Operand`, `Start`, `Loop` and `Led` are the same loop split at each place where it runs grammar code, for a target that runs grammar code between them instead of inside an operation.

| Declaration | Meaning |
|:--|:--|
| `func reset(m Mark) [missing]` | Returns to the backtracking point m: the position, the variable environment, the recovered errors and the captures (undoing the trail) become what they were when m was taken. |
| `func skipTrivia(r Rule) [missing]` | Skips the trivia of the Operators Rule r, if it has any, in a lookahead: a failed skip leaves the parse as it was, and one that matches discards the errors recovered in it, as a lookahead does. |
| `func longest(l Operators) Attempt [budget, missing]` | Tries every operator of l from the position and returns the longest match, the first declared among equals, or the null attempt; it records whether a candidate failed after a cut in the component `operators`. The parse is left at the position, with the captures of each candidate in its frame. |
| `func operatorsParse(r Rule, min int) (Value, bool) [budget, missing]` | Parses an expression of the Operators Rule r with operators that bind tighter than min. Each level of its recursion (the operand of a prefix operator, the right operand of an infix operator) counts against the nesting limit, like the entry of a recursive rule. |
| `func operatorsExpr(r Rule, min int) (Value, bool) [budget, missing]` | Parses an operand, or a prefix application, and then applies the operators that follow it while they bind tighter than min: the longest operator match wins, a non-associative operator does not apply again at its own level, a right-associative one parses its right operand at one level less, and a failed right operand ends the loop before the operator unless the operator cut. An operator that binds tighter than the last one applied, or than the prefix operator of the start, ends the loop: its right operand refused it, so it cannot apply to the operator as a whole. |
| `func operatorsNud(r Rule) (Value, bool) [budget, missing]` | Parses the start of an expression of the Operators Rule r: the application of the longest prefix operator to its operand, or else the first operand line that matches. A failure after a cut fails the expression. |
| `func operatorsParseTyped(r Rule, min int) (Value, bool) [budget, missing, typed]` | Parses an expression of the Operators Rule r with operators that bind tighter than min. Each level of its recursion (the operand of a prefix operator, the right operand of an infix operator) counts against the nesting limit, like the entry of a recursive rule. |
| `func operatorsExprTyped(r Rule, min int) (Value, bool) [budget, missing, typed]` | Parses an operand, or a prefix application, and then applies the operators that follow it while they bind tighter than min: the longest operator match wins, a non-associative operator does not apply again at its own level, a right-associative one parses its right operand at one level less, and a failed right operand ends the loop before the operator unless the operator cut. An operator that binds tighter than the last one applied, or than the prefix operator of the start, ends the loop: its right operand refused it, so it cannot apply to the operator as a whole. |
| `func operatorsNudTyped(r Rule) (Value, int, bool) [budget, missing, typed]` | Parses the start of an expression of the Operators Rule r: the application of the longest prefix operator to its operand, or else the first operand line that matches. A failure after a cut fails the expression. Besides its value it returns where the value starts: a value that is not a node has no span of its own. |
| `func triviaBegin() Mark [budget, missing]` | Begins the trivia of an Operators Rule: the caller runs the trivia expression and passes the mark to triviaEnd. |
| `func triviaEnd(m Mark, ok bool) [budget, missing]` | Ends the trivia that triviaBegin began, whose expression reported ok, as skipTrivia does. |
| `func longestBegin(st LongestState, l Operators) [budget, missing]` | Begins the longest match of the operators l at the position, in the empty record st. |
| `func longestNext(st LongestState) (Line, bool) [budget, missing]` | Returns the first line of the match st, which the caller runs and passes to longestStep, and false if it has none. |
| `func longestStep(st LongestState, v Value, ok bool) (Line, bool) [budget, missing]` | Ends the line of the match st that the caller ran, with its value v and success ok, as longest does for each operator, and returns the next line. It returns false after the last one, when the record holds the match and the parser whether a candidate failed after a cut. It is one step, not two, because the bytecode VM pays for a call at every line of every match. |
| `func nudBegin(st NudState) [budget, missing]` | Begins the start of an expression in the empty record st, after its trivia. |
| `func nudPrefix(r Rule, st NudState) (int, bool) [budget, missing]` | Settles the longest match of the prefix operators in st: it applies the operator and returns the binding level of its operand, or 0 for no operator, with false when the start fails. |
| `func nudRhs(r Rule, st NudState, rhs Value, rok bool) (Value, bool, bool) [budget, missing]` | Ends the application of a prefix operator to its operand rhs, whose parse reported rok: with done set it returns the result of the start, and otherwise the parse goes on with the operand lines. |
| `func nudNext(r Rule, st NudState) (Line, bool) [budget, missing]` | Returns the next operand line of the start st, which the caller runs and passes to nudOperand, and false after the last one, when the start fails. |
| `func nudOperand(r Rule, st NudState, v Value, ok bool) (Value, bool, bool) [budget, missing]` | Ends the operand line that nudNext returned, with its value v and success ok: with done set it returns the result of the start, and otherwise the next line is tried. |
| `func exprStart(r Rule, st ExprState, lhs Value, ok bool) (bool, bool) [budget, missing]` | Takes the start lhs of an expression, whose parse reported ok, into the record st: done is set when nothing can follow it. |
| `func exprLoop(st ExprState) [budget, missing]` | Begins an iteration of the loop of the expression st; the caller then skips the trivia and runs the longest match of the operators into the record. |
| `func exprLed(r Rule, st ExprState) (bool, bool, int) [budget, missing]` | Settles the longest match in the record st: with done set it ends the expression, with the level of a right operand to parse it returns that level, and with a negative level the loop goes on. |
| `func exprRhs(r Rule, st ExprState, rhs Value, rok bool) (bool, bool) [budget, missing]` | Ends the application of an infix operator to its right operand rhs, whose parse reported rok: with done set it ends the expression, and otherwise the loop goes on. |

## Documents

The application of the edits of a Document to a memo entry when a lookup finds it.

| Declaration | Meaning |
|:--|:--|
| `func memoAdvance(e Entry, log EditLog) bool [document, budget]` | Applies to the memo entry e the edits of log it does not account for yet, and reports whether it is still valid. For each edit, in order, an entry that examined only input before it stays, one that examined only input after it moves by the edit's change in length, unless it is pinned or holds recovered errors, and any other is invalid. An entry that lags behind more than 128 edits is invalid. A form specialized for budget charges one unit for each edit it applies, as work of the Document that a parse from scratch does not do. The entry changes only if it is still valid, and after every charge: one that is not valid stays in its slot until a store replaces it, and a lookup before that applies the same edits again. The first test, of memo.seed, is a guard that no production path reaches: the seed of a growth lives within one parse, when no edit is pending, and a lookup applies edits only to an entry of an earlier parse. |

## Terms

The operators, member reads and built-in functions of terms over the operations of [Values](values.md), and the evaluation of predicates and bindings.

| Declaration | Meaning |
|:--|:--|
| `func equal(l Datum, r Datum) (bool, bool)` | Applies == to l and r: it compares two ints, two strings or two bools, and any value with nil, which equals only nil, a box counting as the value it holds. It returns false for ok when the pair has no equality, which is every other pair. |
| `func index(l Datum, r Datum) (Datum, Fail)` | Returns element r of l, a list or a tuple (nil has no elements), with a box opened. It fails if l is another value, if r is not an int, and if r is not an index of l. |
| `func binary(op int, l Datum, r Datum) (Datum, Fail)` | Applies the binary operator that op indexes in the [binary operators](values.md#operators) to l and r. Ints combine with + - * / % and compare with < <= > >=, with 64-bit wraparound; / and % truncate toward zero, and a division by zero fails. Strings concatenate with + and compare with < <= > >= by code point. == and != apply equal and fail where it has no equality. [] is index. Any other operands fail. |
| `func unary(op int, x Datum) (Datum, Fail)` | Applies the unary operator that op indexes in the [unary operators](values.md#operators) to x: - negates an int, with wraparound, and ! negates a bool. Any other operand fails. |
| `func logic(op int, x Datum) (bool, Fail)` | Checks the operand x of the logical operator that op indexes in the [logical operators](values.md#operators), and returns it. An operand that is not a bool fails. The caller short-circuits: it evaluates the right operand of && only if the left is true and of \|\| only if it is false, and checks each operand it evaluates. |
| `func member(c Act, kind int, x Datum, name Name, field Name) (Datum, Fail)` | Reads the member name of x, whose kind is kind (see [Member Kinds](values.md#member-kinds)) and which reads the field or capture field. Start, End and Value of a node read its range and value; a capture of a node reads it with a box opened, nil if it did not match; a field reads what the value holds, and of a struct the zero value of the declared type of a field that is not set. A value that has no members, a capture of a struct and a field that a struct does not declare fail. |
| `func element(c Act, fn Name, v Datum) (Elem, Fail)` | Returns v as an element of a list or a tuple that the built-in function or literal fn makes: nil, a node, a list, a tuple or a struct as it is, and an int, a string or a bool as val.box makes it. Any other value, such as a function, fails. |
| `func items(fn Name, v Datum) (Seq, Fail)` | Returns the elements of v, an argument of the built-in function fn that takes a list: none for nil, and those of a list or a tuple. Any other value fails. |
| `func length(x Datum) (Datum, Fail)` | The built-in function len: the length of a string in the position unit, of the range of a string node, and the number of elements of a list, a tuple or another node; nil has length 0. Any other value fails. |
| `func text(x Datum) (Str, Fail)` | The built-in function text: a string itself, the text that a string node or an error holds, the input of the range of any other node, and the empty string for nil. Any other value fails. |
| `func convert(t NodeType, x Datum) (Datum, Fail)` | The conversion `T{x}` to the named node type t: a string, tuple or list node, of a named node type or not, becomes a node of t. Any other value fails. |
| `func cover(n Obj, v Datum, first bool) bool` | Widens the range of the struct n, which a term is making, to cover the range of the field value v if v is a node, a list, a tuple or a struct, and returns whether n still has no node among its field values: first is whether it had none before, and the first node gives n its range. |
| `func made(c Act, n Obj, first bool)` | Finishes the struct n that c made of field values that cover has gone over: n takes the range of the rule if no field value is a node, and c records n. |
| `func hull(c Act, tuple bool, kids Seq) Obj` | Makes the tuple value, if tuple, or the list value of the elements kids in c, which it takes over, and gives it its range: the one that covers the ranges that the elements add (seq.span), and the empty range at the start of the rule if none adds one. It charges a step for each element. The value model does not show the range of a list or a tuple, but a struct that holds one covers it. |
| `func conforms(t FType, v Datum) bool` | Returns whether v is of the type t of a cast (see [Cast Types](values.md#cast-types)), a box counting as the value it holds. A list type takes a list value and a tuple type a tuple value, not the node of a repetition or a sequence; a node of a type takes a node whose value is of its element type. It charges a step for each element of a list or a tuple that it examines. |
| `func valueConforms(t FType, n Obj, shape int) bool` | Returns whether the value of the node n, whose shape is shape, is of the type t: a string node has a string, and a tuple node or a list node its elements as value. |
| `func elemsConform(t FType, items Seq) bool` | Returns whether every element of items, the elements of a list, is of the element type t of the list, charging a step for each element it examines; the type top takes every element and charges nothing. |
| `func tupleConform(t FType, items Seq) bool` | Returns whether the elements items of a tuple are of the element types of the tuple type t: as many, and each of its own type, charging a step for each element it examines. |
| `func predicate(t Term) bool` | Evaluates the term of a predicate, which fails on the boolean false and on a term that fails, and holds on every other value. |
| `func bind(slot int, t Term) bool` | Binds the value of the term of a binding to the variable in slot. A term that fails and a function, which no variable holds, fail the binding and bind nothing. The compiler has checked that the value fits the variable. |

## Specializations

A runtime prints a composition as its specialization for a set of features (see [Passes](passes.md#specialization)):
the printed function does what the composition does when each feature of the set is on and every other feature is off.
A runtime may print one composition for several sets, and the generated Go runtime prints the compositions for its
node and its typed values separately.

Each row lists, for one composition, the feature sets of the specializations that each runtime prints, in the
form of the [text form](text-form.md#catalog-entries); `[]` is the specialization with every feature off. The engine's
row includes what its backends print into packages of their own.

| Composition | Generated Go, nodes | Generated Go, typed values | The engine | TypeScript |
|:--|:--|:--|:--|:--|
| call | `[budget, document]` | `[budget, document, observe, typed]` |  | `[budget, observe]` |
| callMemo |  | `[document, observe, typed]` |  |  |
| grow | `[]` | `[typed]` | `[]` | `[]` |
| observed | `[document, observe]` | `[document, observe]` |  |  |
| unmemoized | `[]` |  |  |  |
| memoized | `[budget, document, observe]` |  |  |  |
| document | `[budget, document]` |  |  |  |
| typedDocument |  | `[budget, document, observe, typed]` |  |  |
| callBegin |  |  | `[budget, document, missing]` |  |
| callEnd |  |  | `[budget, document, missing]` |  |
| engineCallDoc |  |  | `[budget, document, missing]` |  |
| engineCall |  |  | `[budget, document, missing]` |  |
| body | `[budget, document]` |  |  |  |
| tracked | `[document, observe]` |  |  |  |
| streamCommit | `[stream]` |  | `[stream]` |  |
| streamEmit | `[stream]` |  | `[stream]` |  |
| streamsIn | `[stream]` |  | `[stream]` |  |
| expect | `[budget, document]` `[budget]` |  | `[budget]` | `[budget]` |
| expectSlow | `[]` |  |  |  |
| expectOrdinary | `[]` |  |  |  |
| mergeKept |  |  |  | `[budget]` |
| mergeExpected | `[budget, document]` |  | `[budget]` | `[budget]` |
| keep | `[budget]` |  | `[budget]` | `[budget]` |
| reset | `[]` | `[]` | `[missing]` | `[]` |
| skipTrivia | `[]` | `[]` | `[missing]` | `[]` |
| longest | `[budget]` | `[budget]` | `[budget, missing]` | `[budget]` |
| operatorsParse | `[budget]` |  | `[budget, missing]` | `[budget]` |
| operatorsExpr | `[budget]` |  | `[budget, missing]` | `[budget]` |
| operatorsNud | `[budget]` |  | `[budget, missing]` | `[budget]` |
| operatorsParseTyped |  | `[budget, typed]` |  |  |
| operatorsExprTyped |  | `[budget, typed]` |  |  |
| operatorsNudTyped |  | `[budget, typed]` |  |  |
| triviaBegin |  |  | `[budget, missing]` |  |
| triviaEnd |  |  | `[budget, missing]` |  |
| longestBegin |  |  | `[budget, missing]` |  |
| longestNext |  |  | `[budget, missing]` |  |
| longestStep |  |  | `[budget, missing]` |  |
| nudBegin |  |  | `[budget, missing]` |  |
| nudPrefix |  |  | `[budget, missing]` |  |
| nudRhs |  |  | `[budget, missing]` |  |
| nudNext |  |  | `[budget, missing]` |  |
| nudOperand |  |  | `[budget, missing]` |  |
| exprStart |  |  | `[budget, missing]` |  |
| exprLoop |  |  | `[budget, missing]` |  |
| exprLed |  |  | `[budget, missing]` |  |
| exprRhs |  |  | `[budget, missing]` |  |
| memoAdvance |  |  | `[budget, document]` `[document]` |  |
| equal | `[]` | `[]` | `[]` | `[]` |
| index | `[]` | `[]` | `[]` | `[]` |
| binary | `[]` | `[]` | `[]` | `[]` |
| unary | `[]` | `[]` | `[]` | `[]` |
| logic | `[]` | `[]` | `[]` | `[]` |
| member | `[]` | `[]` | `[]` | `[]` |
| element | `[]` | `[]` | `[]` | `[]` |
| items | `[]` | `[]` | `[]` | `[]` |
| length | `[]` | `[]` | `[]` | `[]` |
| text | `[]` | `[]` | `[]` | `[]` |
| convert | `[]` | `[]` | `[]` | `[]` |
| cover | `[]` | `[]` | `[]` | `[]` |
| made | `[]` | `[]` | `[]` | `[]` |
| hull | `[]` | `[]` | `[]` | `[]` |
| conforms | `[]` | `[]` | `[]` | `[]` |
| valueConforms | `[]` | `[]` | `[]` | `[]` |
| elemsConform | `[]` | `[]` | `[]` | `[]` |
| tupleConform | `[]` | `[]` | `[]` | `[]` |
| predicate | `[]` | `[]` | `[]` | `[]` |
| bind | `[]` | `[]` | `[]` | `[]` |
