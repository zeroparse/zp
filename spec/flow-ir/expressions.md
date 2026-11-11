# Expressions

An expression of Flow IR gives a value. Use, Read, Nil, Int, Bool, Str, Not, Or and Cmp read variables and registers
and combine values; every other expression is a primitive on the state of a parse. MatchLit, CallRule, CallOutline, Unisolate and
ShapeSingle have two results and are the value of an assignment of two places; every other expression has one.

| Expression | Fields | Meaning |
|:--|:--|:--|
| `Use` | V | Reads a variable. |
| `Read` | R | Reads a register. |
| `RecLen` |  | Is the number of recovered errors. |
| `KidsLen` |  | Is the height of the stack of gathered values. |
| `StreamOn` |  | Reports that the invocation delivers the elements of its #stream repetition: a stream parse with a callback, at the top call of its start rule. |
| `SkipHit` |  | Ends the iteration of the walk before, if any, and reports whether the walk has a record of the position; it then takes the rest of the walk from the record, which moves the position to its end, and the walk stops. |
| `ScanReach` | Site, Start, Count | Is called by a walk of the scan site Site that began at Start when it has matched Count characters, the hot span of scan sites. At a hot site with a record of Start it moves the position to the recorded end and is the recorded count; otherwise it is Count. |
| `ScanKids` | Site, Start, Base, Count | Is the list of the elements of a walk of the scan site Site that builds a value, which began at Start, gathered its elements above Base and matched Count characters: for a walk that took the rest from a record, the elements of the record, which it shares, and the gathered ones are removed; otherwise the gathered ones, which it removes from the stack and, for a walk of the hot span or more at a hot site, records. |
| `Nil` |  | Is the nil value. |
| `Int` | N | Is an integer constant. |
| `Bool` | B | Is a boolean constant. |
| `Not` | X | Negates X. |
| `Or` | X, Y | Is X \|\| Y. |
| `Cmp` | Op, L, R | Compares two integers: Op is ==, != or <. |
| `HasLit` | Lit | Reports that the input at the position begins with the literal, read in place without spending budget or recording anything; it is false where the input cannot be read in place. |
| `MatchLit` | Lit | Matches the literal, recording its expectation on failure. Its results are its value (not used) and whether it matched. |
| `MatchClass` | Class | Matches one character of the class through its membership table, recording the class's expectation on failure, and reports whether it matched. |
| `InClass` | Class, Ch | Reports whether the code point Ch is in the class. |
| `GuardFails` | Guard, Ch, More | Reports that the guard skips its alternative: the character that More (there is one) and Ch describe cannot begin it, and the nesting limit allows the guard's depth of calls. |
| `AtAnchor` | Kind | Reports whether the position is at the anchor. |
| `SkipTerminal` | T | Runs the terminal region as one program and reports whether it matched. |
| `SkipClasses` | Run | Matches the classes of the run in turn and reports whether they matched. |
| `CallRule` | Rule, Level | Calls rule Rule from Operators Rule level Level. Its results are the rule's value and whether it matched. |
| `CallOutline` | Node, Slots, Build | Runs the node Node, which a function of its own lowers, under the current frame, whose capture scope has the slots Slots. Its results are the value of the node, built if Build, and whether it matched. |
| `Isolate` |  | Begins an expectation scope at the position and is its mark. |
| `Unisolate` | Mark | Ends the expectation scope Mark. Its results are the farthest failure inside it and its expectations. |
| `Text` | Start | Is the input from Start to the position. |
| `LiteralText` | Start, Lit | Is the text of the literal Lit matched from Start, which differs from Lit where Lit holds the replacement character. |
| `Str` | S | Is a string constant. |
| `NewMatch` | Start, Text | Is a match value from Start to the position with the text Text. |
| `NewNode` | List, Start, Kids | Is a tuple node, or with List a list node, from Start to the position with the children Kids. |
| `NewTerminalNode` | Start, Text | Is a terminal of the Node runtime from Start to the position, with the text Text. Only a Node function builds it; a typed function builds NewMatch. |
| `NewBranchNode` | List, Start, Kids | Is a branch of the Node runtime from Start to the position: a sequence, or with List a list, of the children Kids. Only a Node function builds it; a typed function builds NewNode. |
| `NewVals` | N | Is an empty list of values with room for N. |
| `Kids` | Base | Is the list of the values gathered above Base, which it removes from the stack. |
| `ShapeSingle` | Shape, X, Start | Is the value of X, matched from Start by an expression of the shape, where it takes one element. Its results are the value and whether it is a single element (not used). |
| `ShapeCapture` | Shape, X, Start | Is the value of X, matched from Start by an expression of the shape, where it is captured. |
| `ScopeValue` | Shape, X, Start | Is the value of a scope whose body, of the shape, matched X from Start. |
| `AttachCaptures` | X, Names, Vals, Frame, Start | Is X, matched from Start, with the captures of the scope whose slots are Names attached: the values of the variables Vals, or of the frame Frame if it is not nil. |
| `FreshNode` | X | Is X if it is a node its maker still owns, or nil. |
| `ConvertResult` | Rule, X, Owned | Is the value X of rule Rule converted to the rule's node type; Owned, if not nil, is the fresh node of the body, which converts in place. |
| `UseCtx` | Start | Is a new context for the action of the rule entered at Start. |
| `Pred` | Node, Caps | Evaluates the term of the predicate Node with the captures Caps and reports whether it holds. |
| `Bind` | Node, Caps | Binds the variable of Node to the value of its term, evaluated with the captures Caps, and reports whether the evaluation succeeded. |
| `Eval` | Term, C, Caps | Is the value of the action Term of the function's rule, evaluated in the context C with the captures Caps. |
| `CtxFinish` | C, X | Is the value X of an action finished in the context C. |
| `MarkState` | Env, Trail, Rec | Is a mark of the whole parser state, which Reset returns to. The position is always part of it; Env, Trail and Rec report whether the code between the mark and its Reset may change the variable environment, add to the capture trail, or append recovered errors, so that a target may restore only those. |
| `MarkPos` | M | Is the position of the mark M. |
| `NewFrame` | N | Is a new frame for the scope of the names N. |
| `FrameVal` | F, Slot | Is the value of the slot Slot of the frame F. |
| `Keep` | Exps | Is the list of expectations Exps, kept beyond the expectation scope that recorded them. |
| `MakeError` | Far, Exps | Is the syntax error at Far with the expectations Exps. |
| `ErrorValue` | Start, X | Is the value of a recovered error X over the input from Start to the position. |
| `NewErrorNode` | Start, X | Is the value of a recovered error X in the Node runtime over the input from Start to the position: an error node. Only a Node function builds it; a typed function builds ErrorValue. |
