# Statements

A statement of Flow IR does one of the following. Block, Loop, If, Exit and Continue are the structure of a function;
Set and Inc assign places; every other statement is a primitive on the state of a parse. A field that a statement
leaves empty is absent.

| Statement | Fields | Meaning |
|:--|:--|:--|
| `Block` | Label, Body | Runs its body; an Exit to its label continues after it. |
| `Loop` | Label, Counter, Max, While, Body | Runs its body repeatedly until an Exit to its label leaves it. With Counter, the loop sets Counter to 0 first, adds one after each iteration and ends after Max iterations; with While, it ends when While no longer holds before an iteration. |
| `If` | Init, Cond, Then, Else | Runs Init, if not nil, and then Then when Cond holds, otherwise Else. |
| `Exit` | To | Leaves the block or loop To, which encloses it. |
| `Continue` | To | Starts the next iteration of the loop To, which encloses it. |
| `Set` | Dst, X | Assigns the values X to Dst, in order. A single X may be an operation with as many results as Dst has places. |
| `Inc` | V | Adds one to V. |
| `Step` |  | Charges one step of budget. |
| `Silent` | Delta, Site, Ok | Adds Delta to the silent depth: expectations are not recorded while it is positive. A Silent of a lookahead (Site is not 0) also begins the lookahead of the site, after it enters the silent depth, or ends it, whose body matched if Ok, before it leaves. |
| `Peek` | Ch, Size, OK | Reads the character at the position without consuming it: its code point into Ch, its size into Size and whether there is one into OK. Ch and Size may be nil (not kept). Peek reads as the runtime's read of a character does, which also charges the budget for the read and, in a Document parse, records how far the parse examined the input. A target that inlines the read of a code point in memory must do the same: charge one compute unit unless the function is lowered without budget controls, and leave Document parses to the runtime's read of a character. |
| `Advance` | N | Moves the position forward by N. |
| `Expect` | At, Exp, Message | Records the expectation Exp at At; with Message, as the message of a failed @error. |
| `Restore` | Pos, Rec | Goes back to the position Pos and the number of recovered errors Rec. |
| `MergeExpected` | Far, Exps | Merges the expectations Exps, recorded at Far, into the enclosing expectation scope. |
| `MergeRenamed` | Far, Exps, Exp | Merges the expectations Exps, recorded at Far, into the enclosing expectation scope, renamed to Exp (@diagnostic). |
| `PushKid` | X | Pushes X on the stack of gathered values. |
| `DropKids` | Base | Truncates the stack of gathered values to Base. |
| `StreamStart` |  | Begins the stream of a stream parse: the repetition that delivers its elements is about to run. It is guarded by StreamOn. |
| `EmitStream` | X | Delivers the committed element X, which is nil for a value-free repetition, to the callback of a stream parse; the parse has consumed input up to the position. |
| `SkipBegin` | Site | Begins a walk of the skip-run site Site at the position: the iterations of the repetition that follow are visits of the walk, until SkipEnd. |
| `SkipVisit` |  | Ends the iteration of the walk before, if any, and begins one at the position. |
| `SkipEnd` |  | Ends the iteration of the walk before, if any, ends the walk and records the rest of it from each position it visited. |
| `Append` | Vals, X, Spread | Appends X to the list Vals; with Spread, the children of the tuple node X. |
| `SetWhere` | Name | Names the rule whose action runs, for the messages of its errors. |
| `DropCreated` | C | Drops the structs made since the context C began. |
| `Unfresh` | X | Marks the node X, if X is one, as no longer owned by its maker. |
| `Reset` | M | Goes back to the parser state of the mark M. |
| `SetCapture` | Slot, X | Sets the capture Slot of the current frame to X. |
| `ClearVals` | F | Sets the values of the frame F to nil. |
| `DropTrail` | M | Drops the trail entries recorded since the mark M. |
| `FreeFrame` | F | Returns the frame F to the runtime. |
| `AppendError` | X | Appends the syntax error X to the recovered errors. |
| `DeclareNames` | N | Declares the names of a frame scope, which framed repetitions name before their element and declare after it. |
| `Comment` | Text | Is a comment for readers of the printed code. |
| `ObserveSkip` | Alt | Reports to the parser's observation that the guard of a choice skips the alternative Alt. |
