# Invariants

A function of Flow IR is well formed when it satisfies the rules of this chapter, from lowering on. Verification checks
them and reports the first violation it finds; every function that lowering from Core IR produces is verified.

## Structure

- Every block and loop has a label of its own.
- An Exit names an enclosing block, the innermost enclosing loop or the function's failure label. A Continue names the
  innermost enclosing loop. A loop is left or continued only from its own body, outside nested loops, as a break or a
  continue without a label is.
- A loop without a bound, neither a Counter nor a While, can be left: its body exits to it.

## Variables

- Every variable that a statement or an expression reads or assigns is an entry variable of the function or one of
  its variables, and each is listed once, not both as a variable and as an entry variable.
- A local function has its entry variables, start, result and context; a framed function has none. Only a framed
  function has a value or a cut flag, and its cut flag is one of its variables.
- A Set has as many places as values, or one value with that many results. A Peek has a variable for whether there is
  a character.

## Runtimes

- A function builds the values of one runtime. A Node function builds no match, tuple or list node, error value, fresh
  node or result conversion, uses no action context, and attaches no captures from the values of variables; a typed
  function builds no terminal, branch or error node of the node runtime.
- StreamStart and EmitStream appear only in a framed Node function.
- DropTrail and FreeFrame appear only in a typed function.
- The site of a SkipBegin is positive.

## Budget

A function lowered with budget controls keeps the work of a parse bounded by its limits. A charge is a Step, a Peek,
which charges the read of the character, or a CallRule, which the runtime charges as a rule call.

- The function charges before it consumes input: before an Advance, a MatchLit, a MatchClass, a SkipTerminal, a
  SkipClasses or a SkipHit.
- Every iteration of a loop without a Counter charges, on each path from the start of its body to a Continue or to the
  end of the body. A repetition of a character class is a loop of Peek and Advance, whose iteration is charged by its
  Peek.
- A function whose entry the code that holds it charges starts charged. A path is followed through ifs, blocks and
  exits; a path that ends in an exit leaves the block or loop it names with the charge state it has.

## Documents

In a function lowered for a build with Documents, a read of the input in place, HasLit, is the whole condition of an
If, and the If's alternative is the recording primitive, MatchLit, which the target takes when the parse is a
Document's. Otherwise a Document parse would read input without recording how far it examined it.
