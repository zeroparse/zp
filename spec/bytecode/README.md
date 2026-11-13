# ZeroParse Bytecode Specification

This document specifies the bytecode (the *module*) that a ZeroParse grammar compiles to, and the behavior of the virtual machine (VM) that executes it.
A runtime for another language must implement this document. The meaning of the language that the bytecode implements is defined by the [Grammar Language Specification](../language/README.md); for the results a parse must produce, see [Parsing](../runtime/parsing.md).

A bytecode VM must produce the same results as the closure backend.

## Terminology

| Term | Meaning |
|:--|:--|
| match code | The instruction sequence that matches input (rule bodies, Operators Rule parts) |
| term code | The code of one term: an action, the term of a Predicate Action or the value of a binding ([Term Code](#term-code)) |
| entry stack | The stack of backtrack entries pushed by match instructions ([Failure](#failure)) |
| expectation | A string describing what was expected at a position, used in syntax errors |
| farthest failure | The farthest position at which a failure was recorded, together with the set of expectations recorded at that position |
| level | The Operators Rule binding level passed to a rule call (`min`); 0 for an ordinary call |
| value-free twin | A copy of a rule compiled so that it builds no value ([Rule table](#rule-table)) |
| transient rule | A rule that is not memoized in normal parses ([Rule calls](#rule-calls)) |

## Values

| Value | Description |
|:--|:--|
| Node | Type name, rule name (optional), range `[start, end)`, text, list of children, and fields (name → value). A node also carries a flag that marks it as holding text: a string node or an error node. |
| Integer, string, boolean | Used by Value Actions and Predicate Actions |
| nil | The absence of a value |

The reserved bytecode node types are the [kinds](../language/types.md#node-type-names) `string`, `tuple`, `list` and `error`.
The Closure-only `#missing` pragma and `missing` type have no bytecode encoding;
VM parsing, binary serialization and module construction reject those grammars.
A node is *fresh* only while the body of the rule that created it is executing ([Rule completion](#rule-completion)).

## Input and Positions

The input is a sequence of characters. The position unit is chosen for each parse: Unicode code points or UTF-8 bytes.
Reading a character returns the character (a code point) at the current position and its size in position units. Each invalid UTF-8 byte is read as one U+FFFD character whose size is one unit (one byte, or one code point).

## Module

A module consists of the following tables and code sequences.

| Table | Elements |
|:--|:--|
| String table | Strings. All other tables and instructions refer to strings by index. |
| Character class table | A list of ranges (pairs of lower and upper bounds), a negation flag, and an expectation string |
| Type table | Type name, and either the field names and zero descriptors of a struct type or, for a named node type `type N = node[T]`, the text of its underlying node type `node[T]` |
| Scope table | The capture names of a repetition element, in slot order |
| Variable table | The declared variables: name, type (`int`, `string` or `bool`) and initial value. Every parse begins with the environment holding each variable bound to its initial value. |
| Rule table | See [Rule table](#rule-table) |
| Operators Rule table | See [Operators Rule table](#operators-rule-table) |
| Match code | A sequence of instructions |
| Wide-integer table | Implementation-int values: the repetition bounds that do not fit an operand |
| Term table | The term code of the module's terms ([Term Code](#term-code)) |

An instruction consists of an opcode and up to three integer operands, `A`, `B` and `C`. A jump target is the index of an instruction in the same code sequence.

Operands are signed 32-bit integers. Repetition bounds that exceed this range
use `REPEATW` or `SCANW`, whose bound operands index the wide-integer table,
which preserves the full implementation-int range. Ordinary instructions
retain their compact representation.

The public grammar AST treats every negative repetition maximum as unbounded.
Compilation canonicalizes it to -1 before narrowing operands or creating wide
bound constants. The AST's original field is preserved; serialized bytecode
uses the existing -1 sentinel for both `REPEAT` and `SCAN` variants.

### Rule Table

| Item | Description |
|:--|:--|
| Name | Rule name |
| Body | Start of the body in the match code. The body pushes the value of its scope (or none, if the body is value-free) and ends with `RETURN`: the single value of the body, or a tuple node of its values when the scope is a tuple. |
| Scope | The capture names of the body, in slot order |
| Action | The action's term (an index of the term table), or -1 if there is none |
| Node type | Name of the named node type a rule declared with one converts its value to (see Rule Completion), or -1 if there is none. For a rule with an Operators Rule and no action, it is the named node type every operator's value is converted to (see Operators Rule); with an action, it is the type the action's result is converted to |
| Memoize, left-recursion leader, member, position-dependent | Results of static analysis. A member is a rule of a left-recursion cycle that is not its leader. |
| Cycle | The number of the left-recursion cycle of a leader or member, from 1, the same for the rules of one cycle; 0 for any other rule |
| Operators Rule | Index into the Operators Rule table, or -1 if there is none |
| Value-free twin | Whether this entry is a value-free twin (see below). The value of a value-free twin is always nil. |
| Transient | The rule is not memoized in normal parses ([Rule calls](#rule-calls)) |
| Recursive | The rule is on a cycle of the call graph or calls itself; only calls of such rules count against the nesting limit ([Limits](../runtime/parsing.md#limits)) |
| Where | The text that names the rule in the message of a failed action, as an index into the string table, or -1 for the name. The hidden rule of an inline action is named by the rule that holds the action. |
| Splice | The rule makes no node of its own (a hidden rule without an action). Its body pushes the tuple node of its elements, if the body builds one, else its one element or nil; the value of the rule is that result as it is, without the captures of the body, and its callers take its elements with `SPREAD`. |
| Variables | Names (string table indices, sorted) of the variables read by predicates in the rule or the rules it calls; memo entries are keyed by their values |

**Value-free twins.** When a rule that returns a CST is called from a place where its value is discarded (`#atomic`, `-`, `!`, or a value-free body), the compiler compiles a second version of its body that builds no value. The twin is placed in the rule table after all original rules, and the `CALL` refers to it. The twin has the same name as the original rule; lookups of a start rule by name ignore twins. Because the twin matches the same input in the same way, results do not change. Rules that belong to a left-recursive cycle have no twin.

**Value-free bodies.** The body of a rule with an action reads only captures, so it is compiled without building its own value. Capture targets still build their values.

### Operators Rule Table

| Item | Description |
|:--|:--|
| skip | Start of the skip code in the match code, or -1 if there is none. The code builds no value and ends with `END`. |
| operand | A list of lines |
| Prefix operators / infix and postfix operators | Lists of operators (index, kind, associativity, level, line) |

A *line* has the start of its code in the match code (the code pushes one value and ends with `END`), a scope, an action (an index of the term table, or -1), whether its body is a sequence, and the type its value is checked against when the line produces it (as the rule table's check, with the rule's name in the error), or -1. An operand line pushes the value of its scope, as a rule body does. An operator line pushes the operator's value: when its body is a sequence, that value is the tuple node of the operator's values, and the line's captures are attached to it.

Operator kinds are encoded as 0 = prefix, 1 = postfix, 2 = infix. Associativity is encoded as 0 = left, 1 = right, 2 = none.

## VM State

| State | Description |
|:--|:--|
| `pos` | Current position |
| Value stack | Values built during matching |
| Entry stack | Points to return to on failure (see [Failure](#failure)). Each rule body has its own base. |
| Frame | The capture slots of the current scope |
| Capture trail | Writes to captures (frame, slot, previous value) |
| Environment | The values of the declared variables, as a persistent vector indexed by variable slot (the position in the variable table). Saved environments are never modified: rebinding a variable copies only the path to its slot. A parse starts from the environment built from the variable table, so every declared variable is bound before the start rule runs. |
| Recovered errors | Syntax errors recovered by `#recover` |
| `silent` | While positive, expectations are not recorded |
| Farthest failure | A position and the set of expectations recorded there. A parse that does not [record expectations](../runtime/parsing.md#recording-expectations) tracks the position only: the set holds at most a placeholder that stands for a failure at the position and describes nothing |
| Repetition state | For each active repetition: count, value-stack base, maximum, and scope |
| Memo | (rule, position, level) → result |

**Save and restore.** Saving the state records `pos`, the height of the value stack, the height of the entry stack, the number of repetition states, the length of the capture trail, the environment, the number of recovered errors, and the frame. Restoring returns each of these to the recorded value.
Unwinding the capture trail writes each recorded previous value back into its slot.

Value-free recognition uses `NEXT` mode 0 even for a source repetition marked
`#stream`. Mode 2 requires an element value on the stack; it belongs to a
value-building stream-capable program. Element capture frames remain available
to predicates in mode 0, but are not attached to an element value.

## Failure

When an instruction fails, the VM pops entries from the entry stack of the current rule body and handles each according to its kind. If there are no entries left, the rule body fails.

| Entry | Pushed by | When popped on failure |
|:--|:--|:--|
| Choice | `CHOICE` | Restore. If the entry is marked as cut, continue failing; otherwise jump to the recorded target. |
| Iteration | `ITER` | Restore. If the entry is marked as cut, continue failing; otherwise jump to the recorded target. |
| Positive lookahead | `LOOK` (neg = 0) | Restore `silent`, restore the state, and continue failing. |
| Negative lookahead | `LOOK` (neg = 1) | Restore `silent`, restore the state, and jump to the recorded target (the negative lookahead succeeds). |
| Label | `LABEL` | Restore the saved farthest failure. For mode B=0, report A as an explicit error message; for B=1, use A as the diagnostic label for ordinary expectations recorded at the wrapped expression's farthest position. Preserve explicit messages and continue failing. |
| Recovery | `RECOVER` | See [Recovery](#recovery). |
| Skip | Recovery handling | Restore, add the expectations recorded inside to the farthest failure, and continue failing. |

## Match Instructions

`b` denotes an operand that selects whether the instruction pushes a value (1) or not (0). Matching behaves the same either way.

| Code | Instruction | Operands | Behavior |
|--:|:--|:--|:--|
| 1 | `STR` | A string, B expectation, C b | If the characters of string A follow the current position, advance past them and, if b, push a string node. Otherwise record B as an expectation at the start position and fail. |
| 2 | `CLASS` | A class, B expectation, C b | If the current character is in the class (not in it, for a negated class), advance one character and, if b, push a string node. Otherwise record B at the current position and fail. |
| 3 | `ANY` | A b | If there is a character, advance one character (and, if b, push a string node). Otherwise record `any character` and fail. |
| 33 | `SCAN` | A class, B min, C max | A value-free repetition of one character. Advance over characters that are in class A (any character if A is -1) until a character does not match or C characters have been read (no limit if C is -1). If it stopped at a non-matching character (or at the end of input), record the class's expectation (`any character` if A is -1) at that position. Fail if fewer than B characters were read. |
| 34 | `GUARD` | A class, B depth, C target | First-character dispatch, emitted before the `CHOICE` of an alternative that must begin with a given terminal. If the next character is not in class A (or there is none) and the current call depth plus B does not exceed the nesting limit (B counts the calls of recursive rules that the alternative makes before its first terminal), record the class's expectation at the current position and go to C (the next alternative); otherwise continue. For a literal, A is a class of its first character whose expectation is the literal's; for a case-insensitive literal the class also contains the other case of an ASCII letter. The alternative would have failed there recording just that expectation, so skipping it changes nothing but speed. |
| 4 | `TOP` | A b | If b, push an empty string node. |
| 5 | `FAIL` | | Fail. |
| 6 | `ASSERT` | A kind | 0: the position is 0. 1: at the end of input. 2: the position is 0 or the preceding character is `\n`. 3: at the end of input, or the current character is `\n` or `\r`. If the condition does not hold, record `beginning of input`, `end of input`, `beginning of line` or `end of line` respectively, and fail. |
| 7 | `JUMP` | A target | Jump to A. |
| 8 | `CHOICE` | A target | Save the state and push a choice entry with target A. |
| 9 | `COMMIT` | A target | Remove the top entry (a choice) and jump to A. |
| 10 | `CUT` | | Scan the entry stack from the top and mark the first choice or iteration entry as cut. If a lookahead entry or the base of the rule body is reached first, do nothing. Label and recovery entries are skipped over. |
| 11 | `PUSHPOS` | | Push the current position as a value. |
| 12 | `PUSHNIL` | | Push nil. |
| 13 | `SEQ` | A n | Pop n values and the position below them, and push a tuple node whose children are those n values, whose range runs from that position to the current position, and which is fresh. |
| 14 | `ATOMIC` | A b | Pop a position and, if b, push a string node of the text from that position to the current position. |
| 15 | `CAPTURE` | A slot, B pop | Write the top value into slot A of the current frame and record the write in the capture trail. If B is 1, also pop the value (a capture in a value-free context). |
| 16 | `REPEAT` | A min, B max, C scope | Push a repetition state (count 0, value-stack base at the current height). B = -1 means no maximum. C is a scope-table index, or -1 if elements have no scope. When the repetition builds a value, a `PUSHPOS` immediately before it pushes the start position. |
| 35 | `REPEATW` | A min constant, B max constant, C scope | Like `REPEAT`, with the bounds read from the wide-integer table at indices A and B. The scope, iteration, streaming and resumption semantics are unchanged. |
| 36 | `SCANW` | A class, B min constant, C max constant | Like `SCAN`, with the bounds read from the wide-integer table at indices B and C. |
| 37 | `SPREAD` | A n, B tuple | After the `CALL` of a rule that splices, pop its result and push its n elements: if B is 1 the children of the tuple node it returned, otherwise the result itself when n is 1, and nothing when n is 0. |
| 38 | `STRF` | A folded string, B expectation, C b | Like `STR`, for a case-insensitive literal. A is the literal with each ASCII letter A-Z replaced by its lowercase letter. Each input character is folded the same way before it is compared, so only ASCII letters are case-insensitive. B is the literal with its suffix `i`. If b is 1, the pushed string node's text is the consumed input, not the literal. |
| 39 | `SKIPREPEAT` | A min, B site, C flags | Start the repetition of skip-run site B (from 1; see "Skip Runs" in [parsing](../runtime/parsing.md#skip-runs)): push a repetition state as `REPEAT` with minimum A, no maximum and no scope, and begin a walk of the site. C is 0 to 3: if bit 0 is set, the minimum is read from the wide-integer table at index A; if bit 1 is set, the site is free of the environment (see "Run Sites" in [parsing](../runtime/parsing.md#run-sites)), and its records are kept by position alone; otherwise they are keyed by the environment, which is always correct. `ITER`, `NEXT` and `ENDREPEAT` run the elements as for `REPEAT`, with the steps of the walk that their rows give. |
| 40 | `SCANSITE` | A class, B min, C site | `SCAN` without a maximum, for scan site C (from 1; see "Scan Sites" in [parsing](../runtime/parsing.md#scan-sites)). After the first 16 characters, if the site is hot, it looks for the record of the position where the walk began: if there is one, the walk takes its end from it and the following character test fails there as it did when the walk was recorded; otherwise the walk goes on and records itself where it ends, if the input it examined is available. The records are those of the site, kept by position alone. |
| 17 | `ITER` | A exit, B flags, C site | If the repetition has a finite maximum and its count has reached it, jump to A without executing the element. If bit 2 of B is set, C is the scan site of the repetition, which builds values and has no run site (see "Scan Sites" in [parsing](../runtime/parsing.md#scan-sites)): when its count reaches 16 and no Document run resumes it, the walk looks for the record of the position where it began, as `SCANSITE` does; on a hit, the elements of the record replace the values gathered so far, the count is that of the record and the position is its end, and `ENDREPEAT` makes the list from the elements of the record without copying them, and otherwise the walk records the list at its end. If bit 2 is clear, if C is not 0 and this is the first iteration of a repetition that no Document run resumes, the repetition becomes a `SKIPREPEAT` repetition of run site C (see "Run Sites" in [parsing](../runtime/parsing.md#run-sites)), with the walk starting here. B is 0 to 7, and 0 where C is 0; with bit 2 clear it is 0 to 3: bit 0 is set if the repetition builds values, which the walk then takes from `NEXT` and `ENDREPEAT` lists, and bit 1 if the site is free of the environment, whose records are kept by position alone; otherwise they are keyed by the environment, which is always correct. For a `SKIPREPEAT` repetition, end the walk's running iteration; then, if the count has reached the minimum and the current position has a record, reuse the record and jump to A, and otherwise begin an iteration of the walk. Then save the state and push an iteration entry whose target is A (the exit of the loop). If the repetition has a scope, switch to a new frame. |
| 18 | `NEXT` | A loop, B mode, C slot | One iteration succeeded. Remove the iteration entry and restore the frame. If the repetition has a scope and the mode is 1 or 2, attach the captures to the element value (with mode 0 the scope exists only for predicates inside the element). If the repetition is a run site that builds values, give the element value to the walk instead of keeping it. Increment the count. Mode 0 keeps no value (a value-free repetition), 1 keeps the value, 2 passes the value to the stream consumer, and 3 pushes the value of slot C of the element's frame as the element value: a projected repetition, whose elements are matched without values (`map($x, (e) => e.$f)` reads only that capture). Then, if the iteration consumed no input, or the count has reached the maximum, jump to the exit recorded by `ITER`; otherwise jump to A. |
| 19 | `ENDREPEAT` | A b | Pop the repetition state. For a `SKIPREPEAT` repetition, end the walk, which records the rest of it from each position it visited. Fail if the count is below the minimum. If b, pop the values above the value-stack base and the start position below them, and push a list node (fresh); for a run site that builds values, the elements are those the walk gathered, which the list may share with the records of the walk. |
| 20 | `LOOK` | A target, B neg, C keeps | Save the state, push a lookahead entry and increment `silent`. A is used only by a negative lookahead (B = 1). C = 1 marks a positive lookahead whose expression can leave captures or variable bindings after it; its outcome is not observed in a left-recursion context (see [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)). C is 0 for a negative lookahead. |
| 21 | `ENDLOOK` | A neg | The inner expression succeeded. Positive: remove the entry, restore `silent`, restore only `pos`, the value-stack height and the recovered errors, and continue. Negative: restore `silent`, restore the state, and fail. |
| 22 | `CALL` | A rule, B level, C keep | Call rule A at level B ([Rule calls](#rule-calls)). On success, push the value if keep is 1. |
| 23 | `OPERATORS_RULE` | A table | Parse Operators Rule table A at level `min` and push its value. `min` is the level of the `CALL` that invoked this body. |
| 24 | `PRED` | A term | Evaluate term A. Fail if evaluation fails or produces `false`. |
| 25 | `BIND` | A variable, B term, C 0 | Evaluate term B. Fail if evaluation fails; otherwise bind variable A (an index into the variable table) in the environment. The value is not checked: the compiler has proved it assignable to the declared type of variable A, so a module is trusted to bind values of that type, and a variable holds any value, so a module that does not cannot break memory safety. C is 0. |
| 26 | `LABEL` | A message, B mode | Save the farthest failure, reset it to (current position, empty), and push a label entry. B=0 applies `@error`; B=1 applies `@diagnostic` and renames ordinary expected items from the wrapped failure. |
| 27 | `ENDLABEL` | | Remove the entry, add the expectations recorded inside to the saved farthest failure, and make that the farthest failure again. |
| 28 | `RECOVER` | A target | Save the state, save the farthest failure and reset it to (current position, empty), and push a recovery entry with target A (the skip code). |
| 29 | `ENDRECOVER` | A target | The inner expression succeeded. Remove the entry, merge the recorded expectations back as for `ENDLABEL`, and jump to A. |
| 30 | `ENDSKIP` | A b | The skip succeeded. Fail if it consumed no input. Otherwise add the error to the recovered errors and, if b, push an error node. |
| 31 | `RETURN` | | The rule body succeeded. |
| 32 | `END` | | An Operators Rule part succeeded. |

**Streaming.** `NEXT` with mode 2 is emitted only for the repetition marked `#stream` in the start rule. When the VM is parsing a stream and this repetition runs in the start rule's body (call depth 1), the element value is not kept: it is passed to the consumer (no longer fresh), the input up to the current position is committed (earlier input and memo entries are discarded), and the loop exits if the iteration consumed no input and otherwise jumps to A. On reentry, `ITER` enforces the finite maximum before executing another element; a zero maximum executes no elements. In any other parse, mode 2 behaves like mode 1.

### Recovery

When a recovery entry is popped on failure, the VM:

1. Takes the farthest failure recorded inside (a position and its expectations) and restores the saved farthest failure.
2. Restores the state.
3. Pushes a skip entry (holding a saved state for restoring and the failure from step 1) and jumps to the recorded target (the skip code).

`ENDSKIP` builds a syntax error from the failure held by the skip entry. The error node spans from the start of the recovery to the current position; its text is the text of that range and its `message` field is the string form of the syntax error.

## Rule Calls

`CALL r, min` calls rule r as follows. The call is memoized if the rule is a leader, or if it is marked for memoization and is not transient. In an incremental parse, a transient rule is memoized when it is marked for memoization, so that results from the previous parse can be reused.

A rule is marked transient when it calls no other rule, or when it is referenced only once in the grammar. Re-evaluating the former only scans the input once more; the latter is re-evaluated at the same position only when its single caller is. In both cases a memo entry would never be reused.

1. Look up the memo. A leader whose growth for (r, pos, min) is in progress reads its growing result, whatever the variable values. Otherwise look up a result for (r, pos, min) and, when the rule reads variables, for the current values of those variables (the rule table's variable list). In the context of the growths of the rule's cycle at the current position, a result in the memo is used only if it was computed while at least those growths were in progress, and a result kept for the context is used under the conditions of [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules). If there is one, and it is growing, or it was not computed inside a lookahead, or the VM is currently inside a lookahead, use it: record the expectations stored with the result (unless it is growing) and its recovered errors; on success, advance the position and return the value.
2. If the call is memoized, save the farthest failure and reset it to (start position, empty).
3. If the rule is a leader, evaluate it by [growing the seed](#left-recursion); otherwise [evaluate the body](#body-evaluation) once. On failure, remove the errors recovered during this call.
4. If the call is memoized, store in the memo, replacing the entry for the same key, the result, the end position, the recovered errors, whether the call ran inside a lookahead, and the expectations recorded inside; then merge those expectations into the saved farthest failure and restore it. A call in a context records the leaders of the growths of its context with its result. A result that read a growing result of its context, or observed a lookahead at the position that read one, is kept for the context instead, with what it read and observed ([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)).
5. On failure, reset the position to the start position.

### Body Evaluation

To evaluate a body, the VM creates a new frame (sized for the rule's scope), saves the frame, the environment and the length of the capture trail, and runs the body's code.
While the body runs, it uses its own base of the entry stack and of the value stack.
When the body finishes, the VM restores the frame and the capture trail; on success, it performs [rule completion](#rule-completion); finally it restores the environment, so variables defined in the body are not visible to the caller.

### Left Recursion

A leader rule is evaluated as follows:

1. Start the growth of (r, pos, min) with a failure as its growing result.
2. Reset the position to the start position, reset the recovered errors to their state before the call, and evaluate the body.
3. If the body fails, or does not advance beyond the previous success, stop. Otherwise make the result the growing result and go back to step 2.
4. End the growth and return the last result.

A growth that begins where another growth of its cycle is in progress reuses, in step 2, the steps that an earlier growth of the leader took in the same context ([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)).

### Rule Completion

The value of the rule is built from the body value v and the start position:

- **With an action:** evaluate the action with the captures of the scope. For a rule with an Operators Rule, evaluate it with the Operators Rule local `$value`, the folded value v, as well; it is the only local, so `LOCAL` charges one step to find it. If the result is a node newly created by the action, its range becomes the range of the rule. An `int`, `string` or `bool` result becomes a value node with the range of the rule.
- **Otherwise:** if the scope has captures, attach them. If v is fresh, give it the rule name if it has none, and clear its fresh mark.

When the rule's node type is a named node type, the value built above is then checked against the rule's check, which names the underlying node type, and converted to the named node type as `convert` converts it ([Term Code](#term-code)); a value that is not a string, tuple or list node fails the parse with `cannot convert <value type> to <type> in result of rule <name>`.

**Attaching captures:** if no capture is non-nil, do nothing. If v is nil, is not fresh, or is itself one of the captures, wrap v in a tuple node whose only child is v, whose range runs from the start position to the current position, and which is fresh. Then add each non-nil capture as a field under its name.

## Operators Rule

The algorithm is the same as `operatorsRuleParse` in the closure backend; see the parsing rules in [Operators Rule](../language/operators-rule.md#parsing-rules). Each part's code runs with its own entry-stack base and frame.

- **Skip:** increment `silent` and run the skip code; on success remove the errors recovered in it, and on failure reset the position.
- **Longest match:** for each candidate operator, save the state, run the line's code in a new frame, and on success record the end position, the value, the frame, the environment, whether a cut was passed, and the recovered errors; then restore the state (writing the recorded captures back into the frame). A prefix or postfix candidate that succeeds without consuming input is treated as not matching. Select the candidate that advanced farthest; on a tie, select the one declared first. Also record whether some candidate failed after passing a cut, including an empty prefix/postfix match rejected as a candidate. A successful selected candidate takes precedence over this aggregate failure; each candidate's cut scope is isolated.
- **Head:** after skip, try the longest match among the prefix operators. If none matches and some candidate failed after passing a cut, the whole expression fails without trying operands. Otherwise, if one matches, apply it and parse the right operand at the operator's level. If the right operand fails, fail if a cut was passed; otherwise backtrack and try the operands. Operands are tried in declaration order; if an operand fails after passing a cut, the whole expression fails.
- **Tail:** after skip, try the longest match among the infix and postfix operators. If there is no candidate, stop (or fail, if a candidate failed after a cut). Each loop keeps a bound, the tightest level that may follow the start or the last applied operator. It starts as the level of the prefix operator that began the expression, or unbounded if an operand began it; an infix application sets it to the operator's level (level - 1 for a right-associative operator), and a postfix application makes it unbounded. If the operator's level is less than or equal to `min` or greater than the bound, or the operator is non-associative and a non-associative operator of the same level was just applied, backtrack and stop. A postfix operator is applied immediately. For an infix operator, parse the right operand at the operator's level (level - 1 for a right-associative operator); if that fails, fail if a cut was passed, and otherwise backtrack and stop. The bound is what keeps a non-associative operator from chaining in the right operand of a looser operator: `a | a < a < a` ends after `a | a < a`, because the loop of the right operand of `|` stops at the second `<` and the loop of `|` does not accept an operator tighter than `|`.
- **Values:** if the operator has an action, evaluate it with the Operators Rule locals `$rhs` (except for a postfix operator), `$lhs` (except for a prefix operator) and `$op` (a string node of the operator part's text); `LOCAL` looks a name up by walking them in that order, and charges a step for each local it reaches, the one it finds included. Otherwise build a tuple node whose children are `[op, rhs]`, `[lhs, op]` or `[lhs, op, rhs]` according to the operator kind, whose `operator` field is the operator's index, and which carries the rule name. The value of an operand is its action's result if it has an action; otherwise its captures are attached and, if it is fresh, it is given the rule name. When the rule's node type is a named node type and the rule has no action, the value of every operator (the action's result, or the tuple node, which is then built with that type) is converted to it as `convert` converts it, with the failure `cannot convert <value type> to <type> in result of rule <name>`; the value of an operand is not converted, and the rule's result as a whole is not converted.

## Execution Models

A runtime implements one or both of two execution models, which differ in how rule calls and the Operators Rule are executed. Both must produce the same results.

- **Recursive model:** `CALL` and `OPERATORS_RULE` invoke the procedures in [Rule calls](#rule-calls) and [Operators Rule](#operators-rule) as host-language functions. The maximum nesting depth is bounded by the host stack.
- **Iterative model:** execution suspends at `CALL` and `OPERATORS_RULE` and resumes when the result is available (after a `CALL`, on success it pushes the value if keep is 1 and continues with the next instruction; on failure it proceeds as in [Failure](#failure)). Rule calls, left-recursion growth, the Operators Rule loop, longest match and skip are implemented as state machines whose state is kept in stack entries, and they perform the steps above in the same order. Host stack usage is constant, independent of how deeply the input nests.

Both models perform the same instruction steps and the same stages of a rule call: the call begins, a left-recursive rule grows its seed, the rule body is invoked, and the call ends. They differ only in the order in which these are invoked.
The reference implementation also limits the nesting depth of recursive rule calls, the rules on a cycle of the call graph (`Rec` in the rule table; 100,000 by default, 10,000,000 for the iterative model; configurable with `core.Limits.Depth`; see [Limits](../runtime/parsing.md#limits)), and reports a parse error when the limit is exceeded. A call of a rule without `Rec` does not change the depth.
Actions and predicates may be evaluated with host function calls in either model, because their nesting depth is fixed by the grammar and does not depend on the input.

## Term Code

A term of the action language, an action, the term of a Predicate Action or the value of a binding, is evaluated by its
term code: the instructions of the function that the [lowering of terms](../exec-ir/lowering.md#terms) makes of the term,
which call the [compositions of terms](../exec-ir/compositions.md#terms) in the order the term evaluates its operands.
A failure of a composition ends the evaluation with that failure. A failure in an action is an error of the whole
parse (`action in <rule name>: <message>`); in a predicate or a binding it is a failure.

Each evaluation has registers of four files, by the type of what they hold: values (`a`), integers and booleans (`n`,
a boolean as 0 or 1), sequences of elements (`s`) and strings (`t`). A term gives the size of each file; every register
starts as nil, 0, an empty sequence or the empty string. An instruction has an opcode, a destination D and operands
A, B and C; the table names the file of each register operand (`a[A]` is register A of the values) and the pool of
each pool operand. Lambdas are evaluated in place, so no value of a term is a function.

| Code | Instruction | Behavior |
|--:|:--|:--|
| 0 | `NOP` | Nothing. |
| 1 | `CONST` | a[D] = constant A. |
| 2 | `INT` | n[D] = A. |
| 3–6 | `MOVEA`, `MOVEN`, `MOVES`, `MOVET` | D = A in the values, integers, sequences or strings. |
| 7 | `CAP` | a[D] = the value of capture slot A of the frame, with a box opened. |
| 8 | `LOCAL` | a[D] = the Operators Rule local that name A names (`$lhs`, `$rhs`, `$op` or `$value`), with a box opened, charging a step for each local it passes; the evaluation fails if there is none. |
| 9 | `VAR` | a[D] = variable A. |
| 10 | `UNBOX` | a[D] = a[A] with a box opened. |
| 11 | `FROMB` | a[D] = the boolean n[A]. |
| 12 | `FROMT` | a[D] = the string t[A]. |
| 13 | `NOT` | n[D] = 1 − n[A]. |
| 14–16 | `LT`, `ADD`, `EQN` | n[D] = n[A] < n[B], n[A] + n[B], n[A] = n[B]. |
| 17 | `ISNIL` | n[D] = whether a[A] is nil. |
| 18 | `MEMBER` | a[D] = `member` of a[A] with member read B. |
| 19 | `UNARY` | a[D] = `unary`(B, a[A]); B indexes the [unary operators](../exec-ir/values.md#operators). |
| 20 | `BINARY` | a[D] = `binary`(C, a[A], a[B]); C indexes the binary operators. |
| 21 | `LOGIC` | n[D] = `logic`(B, a[A]); B indexes the logical operators. |
| 22 | `TEXT` | t[D] = `text`(a[A]). |
| 23 | `TEXTEQ` | n[D] = whether t[A] and t[B] are equal (`term.textEqual`). |
| 24 | `LENGTH` | a[D] = `length`(a[A]). |
| 25 | `CONVERT` | a[D] = `convert`(node type B, a[A]). |
| 26 | `CONFORMS` | n[D] = `conforms`(the type of cast B, a[A]). |
| 27 | `CASTERR` | a[D] = the failure of cast B of a[A] (`term.castError`): `cannot cast <value type> to <type> at input position <N> (line <L>, column <C>)`, where N is the position where the evaluation started, followed by `; cast at grammar position <line>:<column>` when the cast's line is positive. |
| 28 | `KIDS` | n[D] = the number of gathered elements (`term.kids`). |
| 29 | `PUSH` | `push`(name B, a[A]). |
| 30 | `PUSHALL` | `pushItems`(name B, a[A]). |
| 31 | `ABOVE` | s[D] = the elements gathered above n[A], taken off (`term.elemsAbove`). |
| 32 | `HULL` | a[D] = `hull`(B = 1, s[A]). |
| 33 | `ITEMS` | s[D] = `items`(name B, a[A]). |
| 34 | `ITEMAT` | a[D] = `itemAt`(s[A], n[B], C = 1). |
| 35 | `SEQLEN` | n[D] = the number of elements of s[A]. |
| 36 | `STEP` | Charge one step (`seq.step`). |
| 37 | `KEEP` | Gather element n[B] of s[A]. |
| 38 | `FILTER` | n[D] = `filterKeep`(a[A]). |
| 39 | `COPY` | a[D] = a new list value of the elements s[A]. |
| 40 | `STRUCT` | a[D] = a new struct of struct literal A, with as many unset fields as it has. |
| 41 | `SETFIELD` | Set field name C of the struct a[A] to a[B]. |
| 42 | `COVER` | n[D] = `cover`(a[A], a[B], n[C] ≠ 0). |
| 43 | `MADE` | `made`(a[A], n[B] ≠ 0). |
| 44 | `JUMP` | Continue at instruction D. |
| 45 | `JUMPF` | Continue at instruction D if n[A] is 0. |
| 46 | `RET` | End the evaluation with the value a[A]. |
| 47 | `RETERR` | End the evaluation with the failure a[A]. |
| 48 | `FIELD` | Charge one step, set field name C of the struct a[A] to a[B], and n[D] = `cover`(a[A], a[B], n[D] ≠ 0): `STEP`, `SETFIELD` and `COVER` with D = C in one instruction. |

The pools of a term hold its constants (nil, an int, a string or a bool), its names (the names of built-in functions,
Operators Rule locals and struct fields), its member reads (the [member kind](../exec-ir/values.md#member-kinds), the
name the term reads and the field or capture it selects), the named node types of its conversions, its casts (the
resolved type text, written as the type table writes an underlying node type, and the grammar position, or 0 and 0
without source information) and its struct literals (the struct type and its number of fields). A cast to a type that
every value fits has no instruction.

A runtime checks the term code of a module when it loads it, and rejects the file if any check fails: the pools refer
to strings of the string table, a conversion to a named node type, a struct literal to a struct type, a cast to a type
text of the form the load-time checks of casts accept, and a member read to a member kind; a term has code; every
opcode is in the table; every operand is in range for its kind: a register within its file, a pool operand within its
pool, a jump target within the code, a variable within the variable table, an operator within its operators, a flag 0
or 1 and a capture slot not negative; and the code ends with `JUMP`, `RET` or `RETERR`, so that evaluation cannot run
past it. As for match code, a runtime does not check that registers hold what an instruction reads.

## Error Messages

A syntax error (see [Syntax errors](../runtime/parsing.md#syntax-errors)) from a parse that records expectations is formatted as `line:column: syntax error: expected A, B`, with the expectations in the order and with the cut after ten that the syntax error contract defines. If `@error` messages apply, it is formatted as `line:column: message` (multiple messages joined with `; `). `@diagnostic` changes ordinary expected-item descriptions but does not replace explicit messages. A parse that does not record expectations formats it as `line:column: syntax error`.
Expectation strings are those the compiler placed in the string table: literals in quotes, character classes in `[...]` form, `any character`, and so on.

## File Format

A compiled grammar uses container version 8. It stores one execution-only compile result: the module of one mode and the start rule, with no grammar AST and no output schema (see [Compiled Grammars](../runtime/compiled-grammars.md)). The container version and the module's instruction-set version cover separate layers: the container version covers the outer frame, the header and the order of the parts below, and the instruction-set version covers the encoding of the module's contents, term code included. A change to the module layout therefore bumps only the instruction-set version. A loader accepts only container version 8. Integers are variable-length: unsigned integers use LEB128 and signed integers use zigzag-encoded LEB128. A boolean is one byte (0 or 1).

| Part | Contents |
|:--|:--|
| Magic | `ZPC\x00` (6 bytes) |
| Version | 1 byte: 8. A loader refuses any other value. |
| Mode | 1 byte: 0 parse, 1 recognize, 2 document. The module serves the operations of this mode only. A parse-mode or document-mode module is the module of the grammar; a recognize-mode module is the module of its recognizer. A loader refuses any other value. |
| Instruction-set version | Unsigned integer for this module: 35. Every module uses this version, whatever instructions it contains; a runtime rejects a module of another version and asks for the grammar to be compiled again. |
| String table | Count, then each string (byte length and UTF-8 bytes), all unsigned. The module's string table is a prefix of this table. |
| Start rule | String index, unsigned: the start rule the saved parser uses. It is a start rule of the module, or the empty string for the module's default start rule. |
| Module | See below |
| Checksum | CRC-32 (IEEE) of everything from the magic up to this point, 4 bytes little-endian |

The module is laid out in the following order. A *list* is a count (unsigned) followed by its elements. Indices, positions and other integers inside the module are signed unless stated otherwise.

1. The number of module strings, unsigned (this many strings from the start of the string table form the module's string table).
2. Source information, a boolean: whether the values a parse returns carry the rule names and capture names of the grammar. A loaded program keeps it.
3. Default value, a boolean: whether the program was compiled with default values, which drop value actions and the declared types of rules (except for the rules that make values read by predicates) before compilation. A loaded program keeps it.
4. Character class table: a list of (negated, expectation string, number of ranges (unsigned), followed by the lower and upper bound of each range).
5. Type table: a list of (name, list of field names, list of the zero descriptors of the fields, underlying node type). A zero descriptor is a string index; it tells how a field that a constructor left unset reads (see `member` in [Term Code](#term-code)), and is written as in the type chapter of the language specification: `i`, `s` and `b` for 0, `""` and false, `n` for nil, `l` for the empty list, the descriptors of the elements in parentheses for a tuple, `{Name}` for the zero value of the struct type Name, and `-` for a type that has no zero value. The underlying node type is a string index, or -1 for a struct type.
6. Scope table: a list of lists of string indices.
7. Variable table: a list of (name, type, integer, string). The type is 0 for `int`, 1 for `string` and 2 for `bool`. The integer is the initial value of an `int` variable, and 0 or 1 for a `bool` variable. The string is the string table index of the initial value of a `string` variable, and -1 for the other types.
8. Rule table: a list of (name, body, scope, action, node type, memoize, leader, member, cycle, position-dependent, Operators Rule, value-free twin, transient, recursive, where, splice, variables).
9. Start rules: a list of the rule table indices of the start rules, the rules marked `@start`, in increasing order, followed by the rule table index of the default start rule. Parsing begins only at these rules.
10. Operators Rule table: a list of (skip, list of operand lines, list of prefix operators, list of infix and postfix operators). A line is (start, scope, action, body is a sequence, check); an operator is (index, kind, associativity, level, line).
11. Match code: a list of instructions (opcode as 1 byte, then `A`, `B`, `C`).
12. Wide-integer table: a list of signed integers.
13. Term table: a list of terms. A term is the sizes of its four register files (unsigned: values, integers, sequences, strings); its constants, a list of (kind as 1 byte: 0 nil, 1 int, 2 string, 3 bool; signed integer: the int, the string index, or 0 or 1); its names, a list of string indices; its member reads, a list of (member kind, name, field); its node types, a list of string indices; its casts, a list of (type text, line, column), the type text -1 for a cast that checks nothing; its struct literals, a list of (type name, number of fields); and its code, a list of instructions. Between the register sizes and the constants, a byte whose bits 0 to 5 mark the pools that are not empty (constants, names, member reads, node types, casts, struct literals) precedes them; only those pools are written, and a marked pool is not empty. An instruction is its opcode (1 byte), a byte whose bits 0 to 3 mark the operands D, `A`, `B` and `C` that are not zero, and those operands; a marked operand is not zero.

### Load-Time Validation

On load, a runtime checks the following and rejects the file if any check fails:

- The checksum, the version, the mode and the instruction-set version are valid, the start rule is empty or a start rule of the module, and no data remains after the module.
- Each variable is named once, has a valid type, and has an initial value that matches its type.
- Every reference in the tables and instructions (strings, character classes, scopes, rules, Operators Rule tables, terms, wide integers) is in range.
- There is at least one start rule. The start rules are in increasing order and have distinct names, and the default start rule is one of them.
- Every opcode is known, and every flag operand is within its range.
- The node type of a rule names a named node type, or a declared type with a field `Text`. The node type of a rule with an Operators Rule names a named node type.
- A named node type has no fields, and its underlying node type is a type text, parsed as the type of a cast is, of the form `node[T]`.
- Wide bounds fit the runtime's implementation `int`, and satisfy minimum ≥ 0 and maximum = -1 or maximum ≥ minimum.
- Term code satisfies the checks of [Term Code](#term-code).
- The line and column of a cast are not negative. The type text of a cast parses as a field type naming only declared types, `error`, `node[T]`, tuple types `(T, ...)`, `int`, `string`, `bool` and `nil`. These type texts are resolved types, which contain no `&` or `_|_`, and no `_` other than the argument of `node[_]`.
- Every jump target of the match code is forward. Only `NEXT` jumps backward.
- Every `RECOVER` body ends at an `ENDRECOVER` immediately before its skip target; that instruction jumps past an `ENDSKIP`. These recovery regions are nested and contain no stream-mode `NEXT` (mode 2) in either the protected body or skip. Element-local recovery ends before the enclosing streaming repetition's `NEXT`.

A runtime does not verify that instruction sequences use the value stack and the entry stack correctly, and it trusts the mode as it trusts the module's types: it checks that the mode byte names a mode, not that the module is the one a compiler writes for that mode. A file whose mode does not match its module MAY return wrong results without an error, such as no value from a parse. Runtime errors caused by invalid code are reported as parse errors, but termination is not guaranteed, so load files only from trusted sources.
