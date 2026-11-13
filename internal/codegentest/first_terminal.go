package codegentest

// LookaheadFirstSrc is a grammar whose alternative x begins with a positive lookahead, so that x has
// no first terminal: a parser may skip an alternative by the next character only where it must
// begin with a terminal reached through expressions that fail when it fails (Evaluations of Rules
// in spec/runtime/parsing.md), and a lookahead does not fail when the terminal after it fails. On
// "bc" a parser evaluates x at 0 once: its lookahead succeeds after an iteration of [a-z]* that
// consumes input inside it, and "q" then fails. A parser that looked through the lookahead to "q"
// would skip x there and leave that iteration out.
const LookaheadFirstSrc = "@start\nrule main = x / [a-z]+\nrule x = &([a-z]*) \"q\"\n"
