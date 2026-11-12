# 069. Exclusion Analysis of Ordered Choice

- **Status**: Partially Implemented
- **Author**: @ornew
- **Date**: 2026-11-04

## Summary

The linear-cost classifier compares the first characters of a unit with those of its continuations. Many units that it
cannot prove committing are not slow: two alternatives share a call such as an identifier, and part one or two symbols
later at a literal. This record describes an analysis over **symbols** instead of characters, where a call of a rule is
one symbol, that shows when a unit and a continuation from one position **part**: after the symbols that both evaluate
alike, they reach a symbol that at most one of them can pass.

The analysis changes no parser. Every parser accepts what it accepted before, with the same values, messages and
recovery. The analysis is a cost argument: where it shows, under strict conditions, that a unit and each continuation
part within k symbols, a continuation evaluates again, before it parts, only symbols whose second evaluation costs a
constant or no more than the unit paid for them at the same position (see [Cost of a Decided
Unit](#cost-of-a-decided-unit)). The classifier then accepts the unit as committing, which generalizes the ways of being
committing of [Conditions for Linear Cost](../../../spec/runtime/parsing.md#conditions-for-linear-cost). For an
alternative, the continuations include the later alternatives, so a choice whose alternatives are all decided is a
choice in which a later alternative fails after bounded work; a declared linear mode, proposed separately, can treat it
as decided and report a choice that the analysis cannot decide.

Whitespace follows a convention, with a lint. A rule whose shape makes it **trivia** (a greedy repetition that depends
only on the position and the variables that it reads) is recognized from its body, and the analysis compares its calls
by position. A token layer is not introduced: on the bundled grammars, no overlap that the measurements found needs
one.

The classifier uses the analysis by default: decided units at k = 3 and trivia rules are on. The argument that the
classification stays sound with them is a sketch, not yet a proof; [Soundness](#soundness) states it and its gaps, and
a proof is still to be written. Until then the differential tests, which measure every random grammar that the
classifier calls linear, with a family whose continuations share prefixes, separators, trivia calls and lookaheads,
are the evidence for the defaults.

On the bundled grammars, the strict analysis at k = 3 leaves 1315 of the 1437 nested reasons, and 1261 with trivia
rules; no grammar changes class. Most of what remains is lookahead units, which the analysis leaves to the
other cases, and units whose continuations reach parts that are not cheap.

## Motivation

The classifier of [record 064](064-grammar-complexity.md) reports about 1400 nested reasons in the bundled grammars: a
unit that is not committing against a continuation, nested without bound through rule calls. Grouped by shape:

- **Shared calls followed by a literal**: `ident "(" args ")"` against `ident "." name`, or a message type, a call and a
  variable that begin with one dotted name.
- **Whitespace as a first character**: rules that begin with `ws` put the whitespace characters into the first set of
  every option and iteration that a `ws` follows, and no number of characters of lookahead sees past a run of
  whitespace.
- **Trailing separators**: an iteration `("," ws value)*` followed by an optional `("," ws)?`.
- **Keywords against identifiers**, and alternatives of one kind of lexeme, such as kinds of numbers.

The character-level first sets cannot separate the first shape: both alternatives begin with the characters of an
identifier. Yet a parsing expression evaluates deterministically: two evaluations of a call of one rule that reads no
variable, at one position, have one result. After the shared call, the two alternatives are at one position again, and
a literal decides between them. Counting a call as one symbol of lookahead makes the decision visible at depth two,
without a lexer. The later alternative then evaluates the shared call again, which the memo answers, and fails at the
literal, so it examines nothing more of the input that the earlier alternative examined.

## Goals

- Decide units against their continuations over symbols with a small bound k, accounting for variables, `#recover`,
  left recursion, nullable rules, lookaheads and nested evaluations that end at one position.
- State what a continuation costs when the analysis decides, against the `cheap` condition of the analysis and the cases
  of Lemma 4 of the [linear-cost proof](../../../spec/proofs/linear-cost.md).
- Let the classifier use the analysis, and name what a linear mode needs from it.
- Measure on the bundled grammars how many choices are decided at each k and which reasons disappear, with every
  measurement reproducible from a committed test.

## Non-Goals

- Changing what a parser does. In particular, the compiler does not insert cuts where the analysis decides; see
  [Alternatives Considered](#alternatives-considered).
- A new choice operator, a token layer, trivia declarations or lexer modes.
- The declared linear mode itself, which is proposed separately. This record names what it needs from the analysis.
- Longest match. A choice between `<` and `<=` is ordered by the author; linearity does not need the longest match, and
  a local convenience for it can come later.

## Design

### Symbols and Paths

The analysis reads a parsing expression as a set of **paths**. A path is a sequence of symbols such that every
successful evaluation of the expression evaluates the symbols of one path in order, each succeeding, each starting where
the previous one ended. A symbol is one of:

- a **match**: a call, a literal, a character class, `.`, or a repetition with an upper bound other than 1, taken whole;
- a **negative** or **positive lookahead** `!K` or `&K`;
- a **skip**: an expression that consumes nothing and decides nothing for the analysis (a predicate, a cut, an anchor);
- a **stop**: an expression that the analysis does not look into, after which it decides nothing;
- in strict mode, a **dead** symbol: a part that no evaluation passes, such as `_|_`, the normal view's form of `!X X`.
  It ends the path, so the paths of an expression also cover the evaluations that fail there, and a unit that never
  succeeds has the paths of what it evaluates before it fails instead of no path at all, which would decide every
  continuation. The comparison without the strict conditions keeps no such path: an alternative that never succeeds
  excludes every later one.

A sequence concatenates the paths of its items, a choice and an option collect the paths of their parts, and a
repetition with an upper bound of 1 is an option of its expression. The paths of an expression are a superset in this
sense: they contain one path for every successful evaluation, and may contain paths that no evaluation follows.
`#recover`, attributed expressions with an error or a missing value, and Operators Rules are stops.

Three further symbols consume nothing and carry what the cost argument needs:

- a **binding** mark, for an expression that can bind a variable in the scope it runs in (see [Exclusion](#exclusion));
- an **environment** mark, before an option, a choice or a repetition that is cheap only where both sides have the same
  variables;
- a **nesting** mark, where a continuation leaves a rule body through a call whose evaluations can nest without a bound
  and end at one position (see [Cost of a Decided Unit](#cost-of-a-decided-unit)).

A **continuation** is what the parser evaluates after a unit: the rest of the body, then what follows each call of the
rule, through rule ends. At the end of an operand of an Operators Rule it goes on with the operator parts that can
follow the operand, their right operands, and what follows the rule. At the end of the start rule it reaches the **end
of the parse**: a whole-input parse checks the end of the input there, which costs a constant, and a prefix parse stops.

### Limits

The analysis bounds its own work. The number of paths of an expression is bounded (256), and so is the length of a
continuation path (12 symbols). A call is expanded into the paths of its rule body only as deep as the comparison needs,
at most six rule bodies for one pair of paths. A continuation is followed through at most six rule and operand ends, and
through fewer where the paths through six would pass the limit on the number of paths; a path cut short at a bound ends
with a stop.

These limits do not bound the product of the expansions on both sides, which grows as a power of the number of
alternatives: with `p_i = p_(i-1) L_1 / ... / p_(i-1) L_n` over five levels, where each `L_j` is a distinct literal,
comparing `p_5` with an alternative that begins as `p_0` does expands up to n^5 paths of `p_5`. A **work budget** of
2^16 steps, where a step is a symbol examined or a path of an expansion, bounds each comparison of a unit against its
continuations and of an alternative against the later ones. A comparison that exhausts it is undecided, and the
classifier counts it, so a grammar that meets the budget is visible in the measurements rather than slow to compile.

### Exclusion

Two paths p and q from one position **part** at a decisive symbol when the analysis shows that p up to that symbol and q
cannot both succeed. It walks both from the start:

- **Equal symbols** advance both. Two calls of one rule are equal when the rule is not on a left-recursion cycle, and
  either reads no variable or is called with the same variables on both sides: a call inside a growth sees the seed of
  the growth, so two calls of a rule of a cycle at one position can differ. Equal literals, equal classes, `.` and
  structurally equal repetitions whose calls are equal in the same sense are equal too; repetitions are greedy, so equal
  repetitions end alike.
- **Same variables.** Both sides start at one position with the same variables in scope, since a failed alternative,
  iteration or option undoes its bindings. They keep the same variables until either passes a binding mark: a predicate
  that binds, a lookahead or a whole repetition that contains a binding, or the end of a rule whose body binds, which
  the continuation leaves while the unit may have seen the bindings. After a mark, calls of rules that read variables
  are no longer equal, and an environment mark stops the comparison; an environment mark in p also stops it where q
  binds before its next symbol.
- **Trivia calls.** Two calls of one trivia rule are passed without counting toward k, and a call of a trivia rule that
  follows a call of the same rule across symbols that consume nothing is dropped (see [Whitespace](#whitespace)).
- **Disjoint first characters** decide: when neither symbol can match empty, and no character begins both, they cannot
  both succeed at one position.
- **Literals** decide when neither is a prefix of the other, and advance by the shorter one otherwise; two literals that
  are equal up to case, both ignoring case, advance both. A literal and a class or `.` that matches its first character
  advance by that character.
- **Overlapping classes**: two classes, or a class and `.`, that share characters are split into their intersection and
  the rest of each. Where either matches in its rest the other fails, so where both succeed they consume one character
  of the intersection and advance together.
- **The end of the parse** in a continuation decides in strict mode: the continuation examines nothing beyond the
  symbols that both evaluated alike.
- **A negative lookahead** `!K` in p decides when the rest of q must match K at that position: every path of q begins
  with a match of an alternative of K. The match follows q into a call by matching each path of the callee against the
  rest of that alternative, from where the symbols before the call left off. **A positive lookahead** `&K` in p decides
  when K and the next symbol of q are disjoint. Lookaheads in q decide symmetrically.
- **A dead symbol** on one side decides: that side fails there, having examined nothing beyond the symbols passed alike.
- **Neither side can go on**: in strict mode none of the above decides where both sides are stuck before they consume
  more input, each at a dead symbol, a symbol that no input matches, a `!K` that the rest of its own path must
  match, or a `&K` disjoint from its own next symbol. Two paths that fail at one point after the symbols passed alike
  bound nothing, and a path of a unit compared with its own copy, which a continuation reaches through a left call of
  the unit's rule, can only reach such a point: in a growth that call reads the growing result while each step
  evaluates the unit again.
- **Expansion**: otherwise, a call or repetition on one side is replaced by its paths, the side whose callee reaches the
  other first, and each path must part.

k is the number of symbols that the decision examines: those passed alike and the decisive one. An alternative is
decided at k when it parts from every later alternative on all pairs of paths within k symbols, and a choice when all
its alternatives but the last are. `!kw ident x / "in" x` is decided at k = 1, as is `"a" x / "b" x`;
`ident "(" x / ident "." x` and `"=" x / "==" x` at k = 2.

The four hypotheses of the analysis are FIRST_k over symbols where a call is one symbol, exclusion by negative
lookahead, whitespace by convention, and no longest match. The measurements below test the first three.

### Cost of a Decided Unit

The comparison above is a statement about the language: p and q cannot both succeed. The classifier needs a statement
about cost, which the **strict** form of the comparison adds. It requires that every symbol passed alike be **cheap**,
and it looks into an option, a choice, a repetition or a lookahead only when all its parts are cheap, since a path does
not show the attempts that fail inside them, which can examine input.

A symbol is cheap when its second evaluation at the position where the unit evaluated it costs the continuation a
constant, or no more than the unit paid for it there:

| Symbol | Second evaluation | Case of Lemma 4 |
|:--|:--|:--|
| An expression without a scanning source | at most a constant, by Lemma 1 | 3 |
| A call that the memo answers | a memo access | 2 |
| A call of a rule that calls no rule, reading no variable or with the same variables | its own scan again, which the unit paid at that position | 2 |
| A call of a rule whose body is cheap in all its parts | the sum of the above for its parts | 2 and 3 |
| A call of a rule that reads variables, under the same variables | as above, with the memo keyed by their values | 2 |
| A lookahead that only C evaluates, or its alternatives before the one that U's match implies (*fresh*) | its first evaluation at the position, a constant: no scanning source of its own, and each call answered by the memo or reaching no scanning source | 2 and 3 |

The last row is not a second evaluation. U does not evaluate a lookahead of C, so there is no first evaluation of it to
charge to, and a rule that calls no rule but scans, which is cheap in the rows above, costs a scan there. A fresh symbol
must cost a constant by itself: the memo answers a call at a position after its first evaluation, which Lemma 4
counts once (case 2), and anything else reaches no scanning source (case 3). In
`e = "a" e "x" / !s "b" / "c"` with a scanning rule `s` that calls no rule, `!s` is evaluated by the continuation of the
first alternative and by nothing else, so it is not fresh, and the first alternative is not decided against it.

A call of a rule that calls no rule is not memoized, and its second evaluation repeats its scan. Lemma 4 bounds how
often such a scan is repeated at one position (*Calls that call no rule*) only where the continuation does not reach
the call after leaving a body through a call that nests at one end. The continuation paths carry a nesting mark at such
an end, and after it a call that the memo does not answer and that scans is not cheap: each nested evaluation that
ends at the position evaluates it again, which no constant bounds, as in `e = "(" e? (ws ")")?`.

Whether a call nests at one end depends on this comparison too. A unit that fails after its call of a rule ended at a
position, and a continuation that passes the same call alike and then ends a body without consuming input, restart the
nesting there: in `e = "(" (e R "x" / e) / "."` every level ends where the innermost one ends, through the memo answer
of the call that the failed alternative made. The body is that of the rule of the unit, or that of the caller in which
the continuation passed the call after it left the body of the unit, whose evaluation contains the unit, as `e` in
`e = "(" f e / "."` with `f = (e R "x")?`. The calls that the comparison passes alike therefore take part
in the definition of nesting at one end in
[Conditions for Linear Cost](../../../spec/runtime/parsing.md#conditions-for-linear-cost). Before the checks, the classifier reads the
shared calls of the units of a recursion that do not grow, contain a scanning source and call a rule of their
recursion, against each of their continuations (a unit that fails only before its sources restarts no scan, but
leaving it out saves nothing). The calls that
the decided comparison passes alike, and those that the checks of growing units, of continuations without the
paths that re-enter a cycle and of the operator parts that the enclosing levels of a nesting Operators Rule try
again share, come only from the checks, so the classifier records the calls that each
comparison passed alike; one that would make a call nest where the classifier assumed it did not becomes a reason, so no decision
rests on a nesting that was missed. These records change classes, not only reasons, and only toward rejection: they
can reject a linear grammar, as `main = e R` with `e = "<" f e / "."`, `f = (R e)*`, `R = sp*` and `sp = [ ]`, where the
decided comparison passes `R` and the call of `e` in `f` alike and the continuation then ends `e` through the call of
`e` after `f`, so the call in `f` is reported to nest at one end although every backend parses the grammar in linear
time. A left call from a rule of a left-recursion cycle to a rule of the same cycle nests
at one end when a call of a rule of the cycle does, since each nested evaluation of a
leader that ends at the position grows there and evaluates what follows its left calls again.

Let a unit U and a continuation C start at position s, let U fail, and let the strict comparison of every path of U
with every path of C pass the symbols σ_1, ..., σ_m alike, with m < k, and part at a decisive symbol d. Then:

- Up to d, C evaluates σ_1, ..., σ_m again, each from the position where U evaluated it, and pays for each at most a
  constant or what U paid for it there, by the table. The work of C before d is therefore at most m constants plus the
  scans of the shared calls of rules that call no rule, each of which U paid once at the same position.
- The lookaheads that C evaluates before d, and the alternatives of a negative lookahead that C evaluates before the one
  that U's match implies, are fresh. In strict mode each costs C a constant by the last row of the table: it has no
  scanning source of its own, its calls are answered by the memo at a cost that Lemma 4 charges to the first evaluation
  at that position (case 2), or reach no scanning source (case 3).
- At d, at most one of U and C passes. If d is a literal, a class or a symbol whose first characters decide, C does not
  consume the character at which they part, which is case 1 of Lemma 4 at the position after the shared symbols. If d
  is a lookahead of U whose expression C must match, C fails within that match, and in strict mode the expression is
  cheap in all its parts, so the match costs C what the shared symbols do. If d is the end of the parse, C ends the
  parse after the shared symbols at a constant cost.
- If U ends after shared and cheap symbols, it examined nothing more than C evaluates again at the costs of the table,
  which is case 3.

So nothing that the parser evaluates after U starts a scanning source again inside the input that U examined beyond its
cheap shared symbols. For an alternative of a choice, the later alternatives are among its continuations: when the
choice is decided in strict mode, each later alternative re-evaluates only cheap symbols and fails at the decisive
symbol or before it. This is the sense in which a linear mode can treat the choice as decided.

### Units in the Classifier

The classifier uses the strict analysis as one more way of being committing: a unit that the existing cases do not find
committing is committing when the strict analysis decides it against each of its continuations within k = 3 symbols,
with the calls of trivia rules passed as described in [Whitespace](#whitespace). The memo of trivia rules and the cut
prefixes below are off; they exist as test hooks.
Lookahead units are left to the existing cases. Their expression is abandoned after it succeeds too, so C follows a
successful U, and the argument above, written for a U that fails, does not cover that case as stated.

A second, independent refinement concerns cuts that the author writes. An alternative of a choice that is not growing,
with a `#cut` at the top level of its sequence after items that can fail, is a unit of those items alone: a failure
after the cut makes the choice fail, so its later alternatives do not run after it, and the items after the cut are not
inside the unit. A growing alternative keeps the analysis of its growth.

### Whitespace

Whitespace is handled by convention: a lexeme rule consumes the whitespace that follows it, and no rule other than the
start rule begins with whitespace. Then every unit begins with a symbol of the language, and the analysis compares
lexemes. With leading whitespace, a continuation that begins with `ws` meets a unit that ends with `ws` before its next
lexeme, and the analysis must compare `ws ws` with `ws`, which no bounded comparison decides.

A lint reports a whitespace rule in the first set of a unit or a continuation: a rule that can match empty, matches only
whitespace characters or comments, and is called at the start of a rule body or of an Operators Rule skip. The lint
names the rule that begins with it and suggests the trailing convention.

Whitespace is never skipped implicitly: a parser that skipped it between any two symbols would accept it where the
grammar does not write it. The calls stay where the grammar writes them, and the analysis compares them through a
property that the compiler reads off the rule. A rule is **trivia** when:

- its body is a greedy repetition `X*` or an atomic `X*`, where X cannot match empty, so a call always succeeds, and a
  second call where the first ended matches nothing: X fails where the last iteration stopped. The unrolled form
  `X* (Y X*)*` in which comment rules are often written is the repetition `(X / Y)*` in the normal view, so it has this
  shape;
- neither it nor a rule that it calls runs an action or contains `#recover`, an attributed expression, a stream, an
  Operators Rule or a call of a left-recursion cycle, its body binds no variable that it reads, and its body has no
  `#cut` of its own, which could commit the repetition.

These are the conditions of law P2 of the grammar-laws catalog, under which `Z* Z*` matches as `Z*`. Z may read
variables and capture and test the captures with predicates; it may not bind a variable that it reads, since then
`w w` and `w` can match differently, as on `ba` for `w = ({v == 0} "b" {v := 1} / {v == 0} "a")*`. The result of a
call then depends only on the position and the variables that the rule reads. A call of a trivia rule that reads
variables is compared as any call of a rule that reads variables: equal only where both sides have the same
variables, cheap only there, and not dropped across a binding.

The analysis passes two calls of one trivia rule at one position without counting them toward k, and drops a call that
follows a call of the same rule across symbols that consume nothing, since it matches nothing there. With leading
whitespace, `ws "," ws value` against `ws "," ws "]"` then parts at k = 2 instead of reaching the bound at the third
`ws`. In strict mode each step needs its cost bound:

- **A second call at one position** is cheap by the table where the rule calls no rule: it repeats its own scan, which
  the unit paid at that position, and no memo is needed. A trivia rule that calls a rule is cheap only where the memo
  answers it. A trivia rule can be memoized per position on every backend, but no backend does so yet, and the
  classifier treats such rules as memoized only where the memo is assumed (the measurements say where).
- **A dropped call** starts where the first one ended, a different position, which the memo does not answer. It makes
  one final, failing attempt of X, which costs a constant when X contains no scanning source. When X scans, such as a
  comment that can be unterminated, the attempt scans as far as the last attempt of the first call did, so the call is
  not dropped in strict mode.

The property needs no declaration. A rule `X+` is not idempotent but annihilates itself: `s s` with
`s = [ \t\r\n]+` fails, so the comparison could decide on it, as on a literal, instead of passing it. This is not
measured here.

### Linear Mode

A declared linear mode, proposed separately, turns the classification into a guarantee: in that mode a choice that the
strict analysis cannot decide is a compile error that names the two alternatives, the paths that overlap and the symbol
where the comparison stopped, or the limit that it reached. The measurements show that alternatives are a minority of
the units that are not committing; options, iterations, lookaheads and operator parts are the majority, so a linear mode
needs a rule for them as well (see [Questions for the Owner](#questions-for-the-owner)).

## Soundness

This section argues the classifier's use against [Lemma 4](../../../spec/proofs/linear-cost.md#lemma-4-scanned-cost)
and states the gaps.

**Determinism of equal symbols.** A parsing expression evaluated at one position with one environment and one memo state
of growths has one result. Equal calls are not calls of rules of a cycle, and either read no variable or are evaluated
with one environment, so their results at one position agree; equal literals, classes and `.` examine the same
characters; equal repetitions of such symbols iterate alike, since they are greedy. Two paths that pass equal symbols
from one position are therefore at one position after each of them, whether the symbols succeed or fail.

**One environment.** The unit and its continuation start at one position with one environment: the bindings of a failed
alternative, iteration or option are undone, and a callee's bindings end with its call. Each side changes its
environment only by evaluating a binding in its own scope, which its paths show as a binding mark (a predicate that
binds, or a lookahead or a whole repetition that contains one, found without entering callees), or by leaving a rule
whose body binds, where the caller does not see the bindings that the unit may have seen, which the continuation shows
as a binding mark at that rule end. Before the first binding mark on either side, the environments are equal, and calls
of rules that read variables are equal symbols. A rule's paths do not depend on variables, so such a rule can still be
expanded into the superset of its paths. For the cost bound, the memo is keyed by the values of the variables that a
rule reads, so with one environment the second call is answered as any other memoized call.

**Overlapping classes.** A class or `.` consumes exactly one character where it succeeds. If the character at the
position lies outside one of two overlapping classes, that one fails; otherwise both consume it and end at the next
position.

**Continuations.** The paths of a continuation through an operand end are a superset: after an operand, the parser
evaluates an operator part that applies after it, its right operand, or what follows the rule. At the end of the start
rule the parse ends, so a continuation that reaches it examines nothing more there, beyond the constant check of the end
of the input.

**Lookahead implication.** An exact path of K is a sequence of calls, literals, classes and `.` such that a match of the
path is a match of K. q implies K when every evaluation that succeeds on q matches such a path at its start. Matching
q's symbols against the path from the left, a call of q that is not equal to the next item of the path is replaced by
each of its body's paths, each of which must match the rest of the path from that point; the items before the call
stay matched. A negative lookahead `!K` fails wherever q's rest succeeds, so p, which needs `!K` to succeed, and q
cannot both succeed.

**Trivia calls.** A trivia rule always succeeds and has one result at a position and an environment. Two calls of one
trivia rule at one position, under the same variables, end at one position, which keeps the sides together without
examining anything that distinguishes them. A call that follows a call of the same rule, with nothing consumed and no
binding between them, starts where the first X* stopped, in the variables of the first call's caller, so X fails at
once and the call matches nothing (law P2); dropping it keeps the set of paths. Its cost is the final attempts of X
which Lemma 1 bounds by a constant when they contain no scanning source.

**Exclusion.** Each decisive case is a statement about two expressions at one position: disjoint first characters cannot
both be consumed; two literals that are not prefixes of each other cannot both match; a negative lookahead `!K` fails
wherever q's rest matches K, and q's rest must match K to succeed; a positive lookahead `&K` succeeds only where K
matches, and K and q's next symbol cannot both match there. Expansion replaces a symbol by the paths of the expression
it stands for, which is a superset, so a decision on every expanded path is a decision on every evaluation.

**Cut prefixes against the restarts.** The restarts of Lemma 4 take the first abandoned evaluation w on the path from N
down to α, and use that w's parent goes on after it, so w is a dropped step or a unit. An alternative with a top-level
cut that fails after the cut makes its choice fail, so its parent does not go on after it, and w is not that
alternative. When it fails before the cut, the input it examined is what its prefix examined, and the prefix is the unit
that condition 2 checks. Growing alternatives are excluded, as the growth reenters them.

**Decided units against the cases of Lemma 4.** [Cost of a Decided Unit](#cost-of-a-decided-unit) gives the argument:
up to the decisive symbol, the continuation evaluates again only cheap symbols, each bounded by case 2 or 3, and at the
decisive symbol it does not consume the character where the two part (case 1 at a later position), fails within a
cheap lookahead expression, or ends the parse.

**Gaps.**

- The argument is a sketch. It needs a formal statement of the path superset property, of the strict cheap condition as
  a cost bound, and of each decisive case as a lemma, as cases 1 to 5 have.
- Lookahead units are not decided: the argument for a continuation that follows a successful lookahead expression is
  not written.
- The cost of a second call of a trivia rule that calls a rule assumes a memo per position that no backend builds; the
  classifier assumes it only behind a test hook, and without it such a call is cheap only by the other rows of the
  table, as a call of a trivia rule that calls no rule is.
- Calls of rules that read variables are equal only before a binding mark; equality of calls whose variables are equal
  after a binding, which needs the values, is not implemented.
- Calls of left-recursive rules are never equal symbols, and the growth argument of case 5 is not extended.
- `#recover`, attributed expressions with an error or a missing value, and Operators Rules are stops; operator parts are
  not analyzed.
- The proof depends on the open count of activations of Lemma 4, as the rest of it does.

## Measurements

All numbers come from two committed tests in `internal/compile`:

```sh
ZP_EXCLUSION_REPORT=1 go test -count=1 -run 'TestExclusionReport$' -v ./internal/compile
go test -count=1 -run 'TestExclusionControls$' -v ./internal/compile
```

`TestExclusionReport` classifies the bundled grammars, in the normal view that the compiler reads, under each setting of
the test hooks; `ZP_EXCLUSION_ONLY` limits
it to a comma-separated list of grammar files. The settings are:

- **base**: the classifier without the analysis;
- **cut prefixes**: base, with author-written cuts making a unit of the items before them;
- **k = 2, 3, 4**: decided units with the strict analysis at that bound;
- **trivia**: k = 3, with the analysis relying on trivia rules: the setting of the classifier.

### Decisions of the Language-Level Comparison

Choices and alternatives decided within k symbols by the comparison without the cost conditions, the statement about the
language. An alternative counts when it parts from every later one; a choice when all its alternatives but the last do.

| Grammar | Choices | k=1 | k=2 | k=3 | k=4 | k=5 | Alternatives | k=1 | k=2 | k=3 | k=4 | k=5 |
|:--|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| cel | 44 | 30 | 31 | 31 | 31 | 31 | 94 | 72 | 75 | 75 | 75 | 75 |
| csv | 4 | 2 | 2 | 2 | 2 | 2 | 5 | 2 | 2 | 2 | 2 | 2 |
| cue | 139 | 62 | 64 | 64 | 68 | 68 | 302 | 151 | 156 | 157 | 161 | 161 |
| duckdb | 573 | 355 | 376 | 394 | 405 | 426 | 1288 | 828 | 882 | 938 | 958 | 1002 |
| golang | 172 | 72 | 77 | 79 | 79 | 80 | 344 | 206 | 215 | 219 | 219 | 221 |
| python | 159 | 76 | 85 | 87 | 87 | 88 | 345 | 206 | 224 | 229 | 229 | 230 |
| typescript | 306 | 177 | 191 | 195 | 196 | 196 | 804 | 567 | 596 | 605 | 608 | 608 |
| xml | 35 | 27 | 30 | 31 | 31 | 33 | 54 | 43 | 46 | 48 | 48 | 50 |
| yaml | 91 | 24 | 26 | 26 | 26 | 26 | 128 | 32 | 36 | 36 | 36 | 36 |
| minilang | 10 | 8 | 8 | 8 | 8 | 8 | 27 | 19 | 19 | 20 | 20 | 20 |
| outline | 1 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 |
| json | 5 | 5 | 5 | 5 | 5 | 5 | 9 | 9 | 9 | 9 | 9 | 9 |

The comparison exhausted the work budget for one alternative of duckdb and one of typescript.

### Classification

The class, and the nested reasons under each setting. The number of causes (the units that the reasons name) is in
parentheses where it differs from base.

| Grammar | Class | base | cut prefixes | k=2 | k=3 | k=4 | trivia |
|:--|:--|--:|--:|--:|--:|--:|--:|
| cel | polynomial | 19 | 19 | 19 | 19 | 19 | 19 |
| csv | polynomial | 0 | 0 | 0 | 0 | 0 | 0 |
| cue | exponential | 59 | 59 | 59 | 59 (53) | 55 (53) | 55 (53) |
| duckdb | polynomial | 666 | 666 | 644 | 619 | 603 | 604 |
| golang | polynomial | 165 | 165 | 153 | 151 | 147 | 147 |
| python | exponential | 199 | 199 | 174 (88) | 168 (88) | 153 (88) | 153 (88) |
| typescript | polynomial | 269 | 269 | 246 | 243 | 230 | 227 |
| xml | polynomial | 9 | 9 | 7 | 7 (4) | 7 (4) | 7 (4) |
| yaml | exponential | 39 | 39 | 37 | 37 | 37 | 37 |
| minilang | polynomial | 10 | 10 | 10 | 10 | 10 | 10 |
| outline | polynomial | 2 | 2 | 2 | 2 | 2 | 2 |
| json | linear | 0 | 0 | 0 | 0 | 0 | 0 |
| **total** | | 1437 | 1437 | 1351 | 1315 | 1263 | 1261 |

The causes in base are cel 10, csv 1, cue 54, duckdb 19, golang 21, python 107, typescript 239, xml 5, yaml 34, minilang
1, outline 2 and json 0, 493 in all, 474 at k = 2 and 472 from k = 3 on. No grammar changes class: each polynomial or
exponential grammar keeps causes that the analysis does not reach. Cut prefixes change nothing on the bundled grammars.
The classifier exhausted the work budget for one unit of duckdb from k = 3 on and for five units of typescript from
k = 2 on.

The trivia rules that the check finds:

| Grammar | Trivia rules |
|:--|:--|
| cel | ws |
| csv | |
| cue | sp nlws ws idchars dqbody sqbody gs_body ml_body ml_frag |
| duckdb | ws e_body |
| golang | sp ws blockText dirSegBlock idRest |
| python | ws |
| typescript | identifierRest templateCharacters rawTemplateCharacters ws sp |
| xml | ws, comment_text, pi_data, cdata and the eight quoted bodies |
| yaml | the six one-line scalar bodies |
| minilang | strchars ws |
| outline | spaces |
| json | chars ws |

### What Remains

The units that stay not committing at k = 3 without trivia, 2125 in all: 697 alternatives, 510 lookaheads, 431 options,
288 iterations, 183 operator parts, 15 operands and one base of a `#recover`. Where the strict analysis stops for them:

| Where it stops | k=3 | trivia |
|:--|--:|--:|
| A lookahead unit, left to the other cases | 510 | 510 |
| A continuation reaches a part that is not cheap | 566 | 604 |
| A unit reaches a part that is not cheap | 101 | 106 |
| A symbol that is not expanded, such as a class or `.` | 351 | 363 |
| The continuation is cut at the follow bound | 149 | 152 |
| The paths part only beyond k | 96 | 30 |
| Equal symbols reach k | 94 | 12 |
| An equal symbol that is not cheap | 47 | 57 |
| A stop: an attributed expression, an Operators Rule or `#recover` | 38 | 38 |
| The inline limit | 30 | 30 |
| A cost condition alone: the comparison without it decides | 91 | 105 |
| Other: environment marks, lookaheads that are not cheap, exhausted paths, path and literal bounds, operator level calls, the budget | 52 | 56 |

Trivia rules mostly turn units that reached k into units that stop at a part that is not cheap: the trivia
call is passed, and the next difference is one the cost conditions reject. The cost conditions alone block 91 units at
k = 3. The comparison of the language decides them, but the strict comparison does not.

### Controls

`TestExclusionControls` classifies every grammar of the linear-cost controls in `internal/complexitytest` under four
settings: decided units alone and with trivia, and trivia rules treated as memoized, without and with decided units.
Every positive control stays linear, and no negative control is classified linear without the memo assumption. With the memo
assumption, eight negative controls are classified linear. Their whitespace rules are trivia, and their measured cost of
Poly(2) is that of parsers that do not memoize them. This is why the memo assumption is a test hook and not part of the
classifier.

## Alternatives Considered

- **Automatic cuts.** Where the language-level comparison decides an alternative after a prefix, the compiler would
  insert a `#cut` after the prefix, so that a failure after it does not try the later alternatives, which would fail
  anyway. The match of every choice would stay, but other results would not: the later alternatives no longer add their
  failures to the farthest failure, so messages report nearer positions or other expected items; an enclosing
  `#recover` sees a failure at another position and recovers with another value and other errors; and an action in a
  later alternative that raises a fatal error while that alternative fails is no longer run, so an input that was an
  error is accepted. A cut that commits an iteration or an option changes what it matches. The cost argument gives the
  classifier what the cut would give the parser, without changing any parser.
- **A token layer.** Lexical declarations (`token`, `fragment`, `trivia`) matched longest-first, in modes that the
  parser selects, with first sets over token kinds. It separates keywords from identifiers and removes whitespace from
  first sets by construction. It adds a second layer to the language, its own memo, a new lemma, declarations for every
  literal, and a migration of every grammar. Modes are needed only for sets of tokens matched by longest match, which
  linearity does not require. On the bundled grammars, the overlaps that remain are between sequences of lexemes, which
  a token layer would leave as they are.
- **A deterministic choice operator `|`.** The compiler checks its alternatives disjoint and the parser never tries
  another. It gives the author a construct with a local error, but it is a second choice operator with the same matches
  as `/` wherever it is accepted. The linear mode gives the error without a new operator.
- **Character first sets with k = 2.** It separates `r"` from `r` and `<` from `<=`, squares the sets, and does not see
  past names or whitespace.
- **LL(*) prediction.** Unbounded lookahead decides more overlaps, at the cost of prediction scans that can be as long
  as the input at every decision.
- **Classifier refinement by shape.** A refinement for each shape of [Motivation](#motivation) adds a case to the
  classifier per shape. The symbol analysis is one mechanism for the shapes of the first kind.

## Testing

In `internal/compile`:

- `TestDecideAlt`, `TestDecideAltVariables`, `TestDecideAltTrivia`: the language-level comparison, for each
  decisive case, lookahead implication through a callee and after a prefix, literals that are equal up to case, calls
  of rules that read variables and of left-recursive rules, and trivia calls. `TestDecideAlt` and `TestDecideAltTrivia`
  read the rules as written, since they test how the comparison counts symbols across shared prefixes, and the normal
  view left-factors alternatives that begin alike (N12), which leaves no shared prefix to count. `TestFollowPaths` reads
  them as written for the same reason. The other tests read the normal view.
- `TestStrictConditions`: each condition of the strict comparison, with a case that passes and one that the condition
  rejects: the environment mark, bindings at a rule end, parts that are not cheap on either side, lookaheads that are
  not cheap, a call after a nesting end, and a path exhausted in strict mode.
- `TestDecidedUnits`, `TestDecidedUnitsCost`, `TestDecidedUnitsOperand`, `TestDecidedUnitsTrivia`: the classification
  of units, including a lookahead unit, a separator after a scan that only the cost conditions keep undecided, and the
  continuations of operands.
- `TestCutPrefixes`: a scan after a cut, a scan before it, a recursion after it, and a growing alternative.
- `TestTriviaWhy`, `TestTriviaWhyCut`, `TestTriviaWhyVariables`, `TestTriviaShapeExtended`, `TestCollapseTrivia`,
  `TestCollapseTriviaBinding`, `TestTriviaCheap`, `TestTriviaMemo`, `TestTriviaSettles`: the trivia check, its shapes
  and the conditions of law P2, dropping calls, and the cost of a trivia call.
- `TestFollowPaths`, `TestContPathsEnds`: continuations through rule and operand ends and the end of the parse.
- `TestExclusionBudget`: a grammar whose comparison grows as a power of the number of alternatives is decided for a
  small number and counted as over the budget for a large one, in milliseconds.
- `TestExclusionControls`: the linear-cost controls, described in [Controls](#controls).
- `TestSharedContinuationsFlip`: the grammars of `complexitytest.SharedContinuationGrammars`, a family whose
  continuations share prefixes, trailing separators, trivia calls and lookaheads, include grammars that the classifier
  calls linear with decided units and trivia rules on and does not without them. `TestLinearClaims` in
  `internal/complexitytest` measures those claims on the input families of the family, so it fails when the analysis
  accepts a unit that costs more: it fails with a continuation lookahead taken as cheap instead of fresh, with trivia
  calls passed without the cheap condition, and with the nesting mark left out of the cheap condition.
- `TestExclusionReport`: the measurements, behind `ZP_EXCLUSION_REPORT`.

The controls in `internal/complexitytest` for the fresh symbols are "Scanning lookahead of a later alternative" and
"Scanning first alternative of a later lookahead", which `TestExclusionControls` classifies with each setting.

## Limitations and Open Questions

- The measurements are counts of the classifier; they are not soundness claims.
- The limits of [Limits](#limits) are constants chosen for the measurements; a compiler that runs the analysis needs
  limits that are part of a contract, and a diagnostic when they are reached.
- See [Soundness](#soundness) for the gaps of the proof.
- **Lookahead units.** The strict analysis would decide some lookahead units, such as `!([a-f] id "=") [a-z0-9] id "("`,
  that the other cases do not. They stay undecided until an argument is written for a continuation that follows a
  successful lookahead expression; they are 510 of the units that stay not committing at k = 3.
- **Memo of trivia rules.** The cost argument for a second call of a trivia rule at one position needs a memo per
  position, which the backends do not build for trivia rules that call a rule, so the classifier does not assume it. Whether
  memoizing trivia rules per position on every backend pays for its memo entries is an optimization to measure; with it,
  the classifier could treat those calls as cheap.

### Questions for the Owner

1. **Units other than alternatives in the linear mode.** Options, iterations, lookaheads and operator parts are most of
   the units that are not committing.
   - Apply the strict analysis to every kind of unit, with a compile error naming the unit, its continuation and where
     the comparison stopped: one rule, and the guarantee covers what Lemma 4 needs.
   - Check only choices in the mode, and leave the other units to the classifier: simpler, but the mode no longer
     guarantees linearity.
   - A separate construct for the common iteration shapes, such as a separated list, with the analysis for the rest.

   Recommendation: the first, with the separated list as a convenience where it is accepted separately.
2. **The bound k.**
   - k = 3: covers what the bundled grammars gain, except a few duckdb and xml choices, with small comparisons and
     readable errors.
   - k = 2: smaller, and loses the decisions through a rule body.
   - A pragma per choice to raise it: an escape for the rare deep case, at the cost of another construct.

   Recommendation: k = 3 fixed, and revisit when a grammar needs more.
3. **How the whitespace lint finds whitespace.**
   - By shape: a rule that can match empty and matches only whitespace characters or comments.
   - By the `skip` of the Operators Rules and the rules it calls.
   - By an attribute on the rule.

   Recommendation: by shape, with the `skip` of Operators Rules as a seed, and no new attribute.

## Literature

Citations are from memory and are not verified against the publications.

- B. Ford. Parsing Expression Grammars: A Recognition-Based Syntactic Foundation. POPL 2004.
- K. Mizushima, A. Maeda, Y. Yamaguchi. Packrat Parsers Can Handle Practical Grammars in Mostly Constant Space. PASTE
  2010. Cuts inserted automatically where first sets are disjoint.
- R. R. Redziejowski. Cut points in PEG. Fundamenta Informaticae, 2016. Points after which an alternative's failure
  cannot be followed by a success of another.
- T. Parr, K. Fisher. LL(*): The Foundation of the ANTLR Parser Generator. PLDI 2011.
- T. Reps. "Maximal-Munch" Tokenization in Linear Time. ACM TOPLAS 20(2), 1998.
