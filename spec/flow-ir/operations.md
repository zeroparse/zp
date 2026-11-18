# Operations

A function at the Flow stage is Code IR: its structure (blocks, loops, branches, exits, breaks, continues and
assignments) and the arithmetic and logic of its conditions are the base's ([Code IR](../exec-ir/README.md)), and
every other step is a call of one of the operations below, which act on the state of a parse. An operation names its
dynamic operands, which are arguments of the call, and its results; its static data (a literal, a class, a rule, a
site) is the immediate of the call, of the kind the Immediate column names. A variadic operation repeats its last
operand zero or more times. The calls that read or write the position and the cut flag use the registers `$pos` and
`$cut` instead; the environment and the current frame are reached through `flow.env`, `flow.setEnv`, `flow.frame` and
`flow.setFrame`.

The contract of each operation is that of its declaration in the Flow dialect, which the verifier, the conversion to
the Exec stage and the backends rely on.

| Operation | Operands | Results | Immediate | Contract |
|:--|:--|:--|:--|:--|
| `flow.step` |  |  |  | Charges one step of budget; ends the parse when the budget is exhausted. |
| `flow.silent` |  |  | `silence` | Adds Delta to the silent depth, inside which expectations are not recorded. For a lookahead (Site is not 0) it begins the lookahead of the site after it enters the depth, or ends it, whose body matched if Ok, before it leaves. |
| `flow.peek` |  | rune, int, bool |  | Returns the code point at the position, its size and whether there is one, without consuming it. The read charges one unit in a function with budget controls and, in a Document parse, records how far the parse examined the input. |
| `flow.advance` | n int |  |  | Moves the position forward by n. |
| `flow.expect` | at int |  | `expectation` | Records the expectation at the position at, unless the silent depth is positive. |
| `flow.observeSkip` |  |  | `node` | Reports to the observation of the parse that the guard of a choice skips the alternative. |
| `flow.restore` | pos int, rec int |  |  | Moves the position back to pos and drops the recovered errors after the first rec. |
| `flow.mergeExpected` | far int, exps expids |  |  | Merges the expectations exps, recorded at far, into the enclosing expectation scope. |
| `flow.mergeRenamed` | far int, exps expids |  | `expid` | Merges the expectations exps, recorded at far, into the enclosing scope, renamed to the expectation of the immediate (@diagnostic). |
| `flow.pushKid` | x value |  |  | Pushes x on the stack of gathered values. |
| `flow.dropKids` | base int |  |  | Truncates the stack of gathered values to base. |
| `flow.streamStart` |  |  |  | Begins the stream of a stream parse: the repetition that delivers its elements is about to run. |
| `flow.skipBegin` |  |  | `site` | Begins a walk of the skip-run site at the position: the iterations that follow are its visits, until flow.skipEnd. |
| `flow.skipVisit` |  |  |  | Ends the iteration of the walk before, if any, and begins one at the position. |
| `flow.skipEnd` |  |  |  | Ends the iteration of the walk before, if any, ends the walk and records the rest of it from each position it visited. |
| `flow.scanEnd` | start int, count int |  | `site` | Ends a walk of the value-free scan site that began at start and matched count characters, at least ScanHotSpan: unless the walk took the rest from a record, it records itself at a hot site, or notes that it walked at a site that is not hot. |
| `flow.emitStream` | x value |  |  | Delivers the committed element x (nil for a value-free repetition) to the callback of a stream parse, which has consumed the input up to the position. |
| `flow.append` | vals vals, x value | vals | `spread` | Returns the list vals with x appended; with Spread, the children of the tuple node x. |
| `flow.setWhere` |  |  | `chars` | Names the rule whose action runs, for the messages of its errors. |
| `flow.dropCreated` | c ctx |  |  | Drops the structs made since the context c began. |
| `flow.unfresh` | x value |  |  | Marks the node x, if x is one, as no longer owned by its maker. |
| `flow.reset` | m mark |  |  | Returns to the parser state of the mark m: the position and the parts that flow.markState named. |
| `flow.setCapture` | x value |  | `slot` | Sets the capture slot of the current frame to x. |
| `flow.clearVals` | f frame |  |  | Sets the values of the frame f to nil. |
| `flow.dropTrail` | m mark |  |  | Drops the trail entries recorded since the mark m. |
| `flow.freeFrame` | f frame |  |  | Returns the frame f to the runtime. |
| `flow.appendError` | x error |  |  | Appends the syntax error x to the recovered errors. |
| `flow.declareNames` |  |  | `names` | Declares the slot names of a frame scope, which framed repetitions name before their element and declare after it. It does nothing to the state. |
| `flow.env` |  | env |  | Returns the variable environment. |
| `flow.setEnv` | e env |  |  | Sets the variable environment to e. |
| `flow.frame` |  | frame |  | Returns the current capture frame. |
| `flow.setFrame` | f frame |  |  | Makes f the current capture frame. |
| `flow.recLen` |  | int |  | Returns the number of recovered errors. |
| `flow.kidsLen` |  | int |  | Returns the height of the stack of gathered values. |
| `flow.streamOn` |  | bool |  | Reports that the invocation delivers the elements of its #stream repetition: a stream parse with a callback, at the top call of its start rule. |
| `flow.skipHit` |  | bool |  | Ends the iteration of the walk before, if any, and reports whether the walk has a record of the position; it then takes the rest of the walk from the record, which moves the position to its end, and the walk stops. |
| `flow.scanReach` | start int, count int | int | `site` | For a walk of the scan site that began at start and has matched ScanHotSpan characters, count of them: at a hot site with a record of start it moves the position to the recorded end and returns the recorded count; otherwise it returns count. |
| `flow.scanKids` | start int, base int, count int | vals | `site` | Returns the elements of a walk of the scan site that began at start, gathered on the stack from base, which matched count characters: for a walk that reached ScanHotSpan characters it ends the walk as flow.scanEnd does, and the result is the elements of the record that the walk took, if it took one (the stack is dropped to base), or the gathered elements, which the record that it makes shares. |
| `flow.hasLit` |  | bool | `lit` | Reports that the input at the position begins with the literal, read in place without recording anything; false where the input cannot be read in place. With budget controls a match charges a unit per code point. |
| `flow.matchLit` |  | value, bool | `lit` | Matches the literal at the position and moves past it, recording its expectation on failure. Its results are its value (not used) and whether it matched; on failure the position is unchanged. |
| `flow.matchClass` |  | bool | `class` | Matches one character of the class through its membership table and moves past it, recording the class's expectation on failure; reports whether it matched. |
| `flow.inClass` | ch rune | bool | `class` | Reports whether the code point ch is in the class. |
| `flow.guardFails` | ch rune, more bool | bool | `first` | Reports that the guard skips its alternative: the character that more (there is one) and ch describe cannot begin it, and the nesting limit allows the guard's depth of calls. |
| `flow.atAnchor` |  | bool | `anchorkind` | Reports whether the position is at the anchor. |
| `flow.skipTerminal` |  | bool | `terminal` | Runs the terminal region as one program from the position and reports whether it matched; on a match the position moves past it. |
| `flow.skipClasses` |  | bool | `classrun` | Matches the classes of the run in turn and reports whether they matched; on a match the position moves past them. |
| `flow.callRule` |  | value, bool | `rulecall` | Calls the rule from the Operators Rule level. Its results are the rule's value and whether it matched; on a match the position is the end of the rule's match, on failure it is unchanged. |
| `flow.callOutline` |  | value, bool | `outline` | Runs the node, which a function of its own lowers, under the current frame. Its results are the value of the node, built if Build, and whether it matched. |
| `flow.isolate` |  | expmark |  | Begins an expectation scope at the position and returns its mark. |
| `flow.unisolate` | m expmark | int, expids |  | Ends the expectation scope m. Its results are the farthest failure inside it and its expectations. |
| `flow.text` | start int | value |  | Returns the input from start to the position. |
| `flow.literalText` | start int | value | `chars` | Returns the text of the literal matched from start, which differs from the literal where it holds the replacement character. |
| `flow.str` |  | value | `chars` | Returns the string constant. |
| `flow.newMatch` | start int, text value | value |  | Returns a match value from start to the position with the text. |
| `flow.newNode` | start int, kids vals | value | `list` | Returns a tuple node, or with List a list node, from start to the position with the children kids. |
| `flow.newTerminalNode` | start int, text value | value |  | Returns a terminal of the Node runtime from start to the position, with the text. |
| `flow.newBranchNode` | start int, kids vals | value | `list` | Returns a branch of the Node runtime from start to the position: a sequence, or with List a list, of the children kids. |
| `flow.newVals` |  | vals | `room` | Returns an empty list of values with room for the immediate. |
| `flow.kids` | base int | vals |  | Returns the list of the values gathered above base, which it removes from the stack. |
| `flow.shapeSingle` | x value, start int | value, bool | `shape` | Returns the value of x, matched from start by an expression of the shape, where it takes one element, and whether it is a single element (not used). |
| `flow.shapeCapture` | x value, start int | value | `shape` | Returns the value of x, matched from start by an expression of the shape, where it is captured. |
| `flow.scopeValue` | x value, start int | value | `shape` | Returns the value of a scope whose body, of the shape, matched x from start. |
| `flow.attachCaptures` | x value, start int, caps value (variadic) | value | `attach` | Returns x, matched from start, with the captures of the scope attached: the values caps of the scope's slots. |
| `flow.attachFrame` | x value, start int, f frame | value | `attach` | Returns x, matched from start, with the captures of the scope attached: the values of the frame f. |
| `flow.markState` |  | mark | `statemark` | Returns a mark of the parser state, which flow.reset returns to: the position, and the variable environment, the capture trail and the recovered errors as the immediate names them. |
| `flow.markPos` | m mark | int |  | Returns the position of the mark m. |
| `flow.newFrame` |  | frame | `names` | Returns a new frame for the scope of the names. |
| `flow.frameVal` | f frame | value | `slot` | Returns the value of the slot of the frame f. |
| `flow.keep` | exps expids | expids |  | Returns the expectations exps, kept beyond the scope that recorded them. |
| `flow.makeError` | far int, exps expids | error |  | Returns the syntax error at far with the expectations exps. |
| `flow.errorValue` | start int, x error | value |  | Returns the value of the recovered error x over the input from start to the position. |
| `flow.newErrorNode` | start int, x error | value |  | Returns the error node of the recovered error x in the Node runtime, over the input from start to the position. |
| `flow.freshNode` | x value | value |  | Returns x if it is a node its maker still owns, or nil. |
| `flow.convertResult` | x value, owned value (variadic) | value | `conversion` | Returns the value x of the rule converted to the rule's node type; with Owned, the only owned argument is the fresh node of the body, which converts in place. |
| `flow.useCtx` | start int | ctx |  | Returns a new context for the action of the rule entered at start. |
| `flow.pred` | caps value (variadic) | bool | `predicate` | Evaluates the predicate with the captures caps (or the current frame) and reports whether it holds. |
| `flow.bind` | caps value (variadic) | bool | `binding` | Binds the variable of the binding to the value of its expression, evaluated with the captures caps (or the current frame), and reports whether the evaluation succeeded. |
| `flow.eval` | c ctx, caps value (variadic) | value | `action` | Returns the value of the action of the function's rule, evaluated in the context c with the captures caps (or the current frame). |
| `flow.ctxFinish` | c ctx, x value | value |  | Returns the value x of an action finished in the context c. |
