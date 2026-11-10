# The Parser VM

The parser VM is the abstract machine that runs a parse. Every runtime of ZeroParse hosts it: the engine's backends and
every generated parser. A module of Exec IR declares it: the features that a function can be specialized for, the
abstract state of a parse, as state components and registers, the handle types and the operations, followed by the
functions (see [Functions](functions.md)). The state is abstract: a contract talks about it, and a target chooses how
to represent it.

## Types

A value of Exec IR has one of the following types.

| Type | Values |
|:--|:--|
| `bool` | A truth value |
| `int` | A position or a count: at least 32 bits, and exact up to 2^53, on every target |
| `i32`, `u32`, `u64` | Sized integers, signed 32-bit and unsigned 32-bit and 64-bit, whose arithmetic wraps at their width |
| A handle type | An opaque value of a declared handle type, or the null handle of that type |

There are no records, arrays or maps: data structures live inside operations, where targets choose them. A handle
is opaque: functions pass it on, compare it with `==` and `!=`, and give it to operations; only operations look
inside.

## Features

A feature is a boolean constant of a build that a function can be specialized for (see [Passes](passes.md)). An
operation that belongs to features exists only in builds that have all of them.

| Feature | The build |
|:--|:--|
| `budget` | Has budget controls: compute and memory limits, the input limit and cancellation |
| `document` | Can parse a Document, reusing the results of an earlier parse after edits |
| `observe` | Can observe a parse: tracing and profiling |
| `missing` | Has missing-token insertion |
| `stream` | Can parse a stream, delivering the elements of a `#stream` repetition as they complete |
| `typed` | Makes typed values, as a typed parser does, instead of nodes |

## State

### Components

A state component is a part of the abstract state that only the operations declaring it may write. The parser VM
has the following components.

| Component | Holds |
|:--|:--|
| `memo` | The memo table: results of rule calls by key. A key is a rule, a position, a binding level and the values of the rule's variables. |
| `seen` | The record of deferred memoization: which rules were called at which positions, and per rule the counts that decide when it is memoized eagerly. It is created with the number of rules whose memoization is deferred. |
| `budget` | The work budget of the parse: the units charged so far and the limit, zero for none. |
| `depth` | The nesting depth of rule calls and its limit. |
| `env` | The variable environment: one current value for each declared variable, in a slot that the grammar fixes. An environment is persistent: a binding makes a new one and changes none that a backtracking point holds. Values handles stand for the values of a rule's variables, compared by the host's value equality. |
| `host` | The host: values and nodes, which the parser VM passes on without reading. |
| `recovered` | The list of errors recovered so far in the parse. |
| `rules` | The rules of the grammar: their descriptors and bodies. |
| `expected` | The expectation record: the expectations at the farthest position. |
| `call` | The call bookkeeping of the target outside the call protocol: the tracked path of Documents and observation. |
| `observe` | The observation of a parse. |
| `growth` | The left-recursion growths in progress with their seeds, the frames that record what the calls in their contexts depend on, and the tables of the results kept in each context (the dependency memo). |
| `window` | The window of a stream parse: the input read and kept, the callback that receives the elements of the #stream repetition, the recovered errors delivered so far, and the memory that the delivered elements held. |
| `runs` | The repetition runs of a Document: for each repetition of the last parse that a parse may resume, the positions, the examined input, the value and the expectations of its elements. |
| `suspended` | The calls whose rule body runs between the begin and the end of the split call protocol, each with the state that its begin saved for its end in a record that its caller holds. |
| `frames` | The capture frames: the frame that captures write to, the slots of each frame and the trail of the writes that a reset undoes. |
| `operators` | The bookkeeping of the Operators Rule loop: whether a candidate of the last longest match failed after a cut. |
| `value` | The operations on the values of terms. They read and write no state. |

### Registers

A register is a scalar of the abstract state that functions read and assign directly.

| Register | Type | Holds |
|:--|:--|:--|
| `pos` | `int` | The position of the parse in the input. |
| `silent` | `int` | The depth of lookaheads, inside which expectations are not recorded. |
| `cut` | `bool` | The cut flag of the scope the parse entered last. |


### Handle Types

A handle type is a type of opaque values. Its component is the state component whose operations make its values.

| Handle type | Component | Stands for |
|:--|:--|:--|
| `Entry` | `memo` | A memo entry: the result of a rule call with its outcome and end. |
| `Values` | `env` | The values of a rule's variables, as a memo key. |
| `Value` | `host` | The value of a rule's result: a node or a typed value. |
| `Errs` | `recovered` | The errors recovered in a rule call, kept with its memo entry. |
| `Datum` | `host` | A value that a variable holds or a term produces, which only the host reads. |
| `Scope` | `env` | A variable environment, which bindings replace and never change. |
| `Rule` | `rules` | A rule's descriptor. |
| `FType` | `value` | The type that a cast checks a value against. |
| `NodeType` | `value` | A named node type, to which a conversion converts a value. |
| `Err` | `recovered` | A syntax error that a recovery made. |
| `Body` | `rules` | The invocation of a rule's body that a generated call passes. |
| `ExpMark` | `expected` | The record that an isolation set aside. |
| `ExpSet` | `expected` | The expectations an isolation recorded, valid until the next record. |
| `ExpID` | `expected` | An expectation: the identifier of a description or message of the grammar. |
| `ExpKept` | `expected` | Expectations kept beyond the record, as a memo entry keeps them. |
| `Ctx` | `growth` | The context state of a memoized call: the growths of its rule's cycle at its position, their present set, and whether the call records its dependencies. |
| `CtxResult` | `growth` | A result that a context keeps (a seed, or the result of a call or a growth step that read a seed), or the last result of a growth. |
| `Grow` | `growth` | The state of the growth of a left-recursion leader. |
| `EditLog` | `memo` | The edits of a Document, in order. |
| `Run` | `runs` | A repetition run in progress in a Document parse. |
| `RunElem` | `runs` | An element of an old run that the parse reuses, or none. |
| `RunSite` | `runs` | A repetition that a Document parse can resume. |
| `Term` | `host` | A term of a predicate or a binding, in the form the target evaluates. |
| `CallState` | `suspended` | The record of a suspended call, which its caller holds. |
| `Frame` | `frames` | A capture frame: the slots of the captures of a rule body or a line. |
| `Mark` | `frames` | A backtracking point: the position, the variable environment, the recovered errors and the length of the trail when it was taken. |
| `Operators` | `rules` | The prefix operators of an Operators Rule, or its other operators. |
| `Operator` | `rules` | An operator of an Operators Rule: its kind, binding power, associativity and line. |
| `Line` | `rules` | A line of an Operators Rule: an operand, or the expression of an operator. |
| `Attempt` | `operators` | A match of an operator: the operator, where it started and ended, its value, the frame of its captures, the environment after it, whether it cut, and the errors it recovered. The null attempt is no match. |
| `LongestState` | `suspended` | The record of a suspended longest match. |
| `NudState` | `suspended` | The record of the suspended start of an expression. |
| `ExprState` | `suspended` | The record of the suspended loop of an expression. |
| `Seq` | `value` | The elements of a list or a tuple, as seq.items returns them. |
| `Fail` | `value` | The failure of a term, as val.fail makes it: a value of the target that the caller reports. |
| `Obj` | `value` | A value that has members: a node, a list, a tuple or a struct, as node.of returns it. |
| `Name` | `value` | The name of a member: a string of the target. |
| `Act` | `value` | The context of the term being evaluated, in which a value is made. |
| `Str` | `value` | A string of the target that is not a value, as text returns it. |
| `Elem` | `value` | An element of a list or a tuple that a term makes, as a Seq holds it. |

## Operations

An operation is a transition of the abstract state. Its declaration has the following parts.

| Part | Meaning |
|:--|:--|
| Name | The operation's name, unique in the module. By convention it is the component's short name, a dot and a verb |
| Component | The state component the operation belongs to |
| Signature | Its parameters, each with a name and a type, and its result types, any number |
| Reads, writes | The components and registers it reads and writes. It writes only its own component and registers, except a transitional operation (below), which declares every component it changes, so that a pass sees each state it changes |
| Features | The features it exists in. None means every build |
| No return | It may end the parse instead of returning: an interruption, such as an exhausted budget |
| Contract | What every implementation does, observable through its results, the state afterwards, the statistics it counts and what it charges |
| Requires | The precondition that its callers guarantee. Behavior outside it is unspecified: implementations may differ on states that no function reaches, and conformance never exercises them |
| Cost | The bound on the units that one call charges the budget (below) |
| Laws | Facts about the operation that passes may use, such as that equal arguments give equal results |
| Immediate operand | A static datum of a call that is not a value of Exec IR, such as a literal, a character class, an expectation or a term of the grammar. A call of an operation that declares one holds it, and passes keep it |

An operation is of one of three kinds.

- An **operation** of the parser VM has a reference model of its component, and a conformance suite compares every
  implementation with the model: its results, the state afterwards and its charges against the bound.
- A **host operation** has a contract and a cost bound but no reference model, because its effect lies outside the
  parser VM's state or is not observable in the result of a parse: input buffers, memory that the parse lets go of,
  callbacks and cancellation. It has no conformance cases; its charges are checked against its bound.
- A **transitional operation** stands for a function of a runtime, with its current behavior, until its mechanism is
  written as a composition of operations of the first kind. It has no conformance cases. An operation is not both a
  host and a transitional operation.

An operation never runs grammar code and never chooses between outcomes of the parse: reusing or recomputing a
result, growing or stopping, which error or expectation survives. Those choices are made in functions, which read
the results of operations. The exceptions are the transitional operations that invoke a rule's body, a line of an
Operators Rule or a term, which say so in their contracts. [Operations](operations.md) and [Values](values.md) list
every operation.

## Costs

The cost of an operation bounds the units of budget that one call charges: at most a constant plus, for each term, a
positive coefficient times the value of a quantity. A quantity is an abstract measure of the state before the call,
which the reference model of the component computes. The charges themselves are the implementation's: for a given
target they are deterministic, and they stay within the bound. The bound is the worst case over the representations
that a target may choose. Charges inside an operation spend the same budget that `budget.charge` and `budget.used`
see, and an implementation may end the parse with an interruption inside an operation only where charges within the
bound could exceed the limit.

A charge that pays for one call, step, iteration or line is made by the function that runs it, with `budget.charge`;
an operation of constant work charges nothing, because the call or step around it has paid. An operation that runs a
rule body or a term declares no bound: what it charges includes the grammar code or the term it runs.

A bound is written `c + k*q + ...`, where `c` is the constant and each `k*q` a coefficient and a quantity. The
quantities are the following.

| Quantity | Measures |
|:--|:--|
| `covered` | The number of positions the memo covers from its floor: the slots that a target keeps per position pays for when it prunes them, moves the rest and drops the indexes of crowded positions. |
| `deps` | The number of dependencies that the call passes to the enclosing evaluation or stores: those that the frame of the evaluation it ends recorded, and those stored with the results, steps and jumps it reuses, with the observations they lead to. |
| `dropped` | The number of values above the mark of the call. |
| `env_path` | The number of nodes on the path to a slot of a persistent vector of the variables with 16 children for each node: the levels above the leaves, and the leaf. It is the same for every slot. |
| `env_reads` | `vars` times `env_path`. |
| `env_rebind` | `env_path` when the value of the call differs from the one the slot holds, and 0 when they are equal: the nodes that a rebind copies. |
| `error_parts` | The number of fragments of the text of a failure: the fixed pieces and the values between them, which a target that bounds the work assembles piece by piece, charging each. |
| `errs` | The number of recovered errors that the call copies: those of its argument, or those recovered after its mark. |
| `fields_read` | The number of fields of a value that a read of one of them scans. |
| `key_scan` | The sum, over the entries stored at the call's position for any rule, of one more than the number of their key values, a string counting two more for each of its bytes: what scanning the position and comparing keys inspects. It counts the entries that memo.splice removed and that a target may still hold there, because it applies edits to an entry when it finds the entry. |
| `merge` | The units that the compositions over the expectation record (keep, mergeKept, mergeExpected, and the calls of expect they make) charge for the sets that the call keeps or merges. |
| `move_edits` | The edits that the call replays to learn how far a node moves that another result moved since the entry's value was made: for each generation of the value's nodes other than the entry's, the edits since that generation. It is zero for a target that keeps no generations. |
| `moved` | The size of the value of the entry that the call moves: for each distinct node in it, one and the number of its children and fields. |
| `n` | The argument n of budget.charge. |
| `page_index` | The call's position divided by 1,024. |
| `pages` | The number of 1,024-position regions of all rules that deferred memoization has recorded calls in. |
| `pending_edits` | The number of edits since the entry under the call's key was stored or last found, counting an entry that memo.splice removed and a target may still hold: the edits that a target which applies them to an entry when it finds it has yet to apply. |
| `pruned` | The number of memo entries stored before the call's position argument. |
| `run_elems` | The number of elements of the finished run. |
| `run_growth` | The number of new elements the run holds, when one more makes their count a power of two, and 0 otherwise: the copy that doubling storage stands for, whatever capacity it has. |
| `run_scan` | The number of old elements from the one the search finds to the end of the run. |
| `run_search` | The number of bits of the count of old elements that the run has not passed: what a binary search for the next element inspects. |
| `run` | For each step of the run of the growth that the call records as jumps (the steps since the last step that read another seed): one, one for each jump recorded before from its seed, and the expectations of its set when its farthest position is that of the steps after it. |
| `seen_index` | The call's position times the number of rules plus its rule, divided by 64: the size of a record that keeps one bit per call in position order. |
| `segs` | The number of entries of the recovered-error list after the mark of the call, or for grow.step after the mark that the growth took when it began: errors that rec.push recorded and lists that rec.add recorded or rec.since put in their place. |
| `set` | The number of expectations of the set that the call takes. |
| `slot_growth` | The number of positions the memo covers from its floor before the call when storing at the call's position makes that number cross a power of two, and 0 otherwise: the copy that doubling a position-indexed table stands for. A target whose table keeps a gap for edits counts its slots, the gap among them. |
| `slots` | The number of slots of the frame that the call takes or opens. |
| `steps` | The number of memoized steps and jumps that the call takes. |
| `stream_frames` | The number of capture frames that the scratch of the stream holds. A target may visit each twice: a scan for the frame in use, which may lie outside the scratch, and a clear of those after it. |
| `stream_inactive` | The capacity of the trail and the inactive capacity of a target's value stack. |
| `stream_scan` | The number of input elements that the call copies when it compacts the buffer, and of those it scans for line starts: every loaded element when the whole input is loaded from its first element and no table of its lines exists yet, otherwise those before its argument that no call scanned. |
| `table` | The work of the call's lookups and stores in the context tables at its position: one for each lookup or store; one for each dependency set of the head it scans, each result under the values the set selects and each key value those results compare, a string counting two for each byte compared; and one for each jump it scans for a seed. A lookup that the outcome of an observation leads to counts again on each path through the observations that outcomes depend on. |
| `term` | What evaluating the term of the call charges, which the term's expression decides. |
| `trail` | The number of trail writes after the mark that the call undoes. |
| `value_bytes` | The number of bytes of the string operands of a primitive, which it reads and, for a concatenation, copies. |
| `value_compare` | What comparing the value of the call with the one the slot holds inspects: two for each byte of two strings of the same length that are not the same bytes of the same memory, one for two such strings that are (the comparison reads none of them), and 0 for anything else. |
| `vars` | The number of variables of the rule's memo key. |
| `zero_parts` | The number of values of which a zero value is made, and, in a target that looks the field up among the declared ones, the declared fields it scans. |
