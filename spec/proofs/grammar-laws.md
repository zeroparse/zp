# Grammar Laws

Arata Furukawa · Last revised 2026-11-09

**Status:** Draft. The laws are proved as stated, except the law of
[Free Variables](#free-variables), which is unproved in full. The
confluence of the [Normalization](#normalization) is not proved. The
condition of its [cost view](#normalization), that the written grammar costs
at most a constant factor more than the view, is proved in
[Cost View](cost-view.md), except for the values that actions build from a
value that the view reshapes and their run-time errors.

This document states the equivalences ("laws") that hold between Rule
Expressions of ZeroParse grammars, such as `X* X*` and `X*`, and a rewrite
system that normalizes expressions with them. Each law says exactly which
parts of an evaluation it preserves: whether the expression matches and where
it ends, its value, the expectations of a syntax error, and the cost. A law
about matching alone can be wrong about values or messages, and an analysis or
an optimizer must use the strongest equivalence that its result depends on.

Analyses such as the [classification](../runtime/parsing.md#classification),
the [lint checks](../runtime/lint.md) and the passes over Core IR rely on
several of these laws already; they are cited where they are used. Laws that
become true only after a transformation or under a declaration that an author
states are in [Conditional Laws](conditional-laws.md).

## Setting

Fix a grammar, a position unit and an input $t$. The semantics are those of
[Rule Expressions](../language/parser-expressions.md),
[Pragmas](../language/pragmas.md) and
[Predicates and Variables](../language/predicates.md).

**States.** An expression is evaluated in a **state**
$\sigma = (p, \eta, C, \lambda, \Gamma)$: the position $p$, the variable
environment $\eta$ of the current rule invocation, the captures $C$ of the
current scope, the lookahead state $\lambda$ (inside a lookahead or the
`skip` of an Operators Rule, or not) and the growths $\Gamma$ of left
recursion open at $p$, as in the [Memoization proof](memoization.md#setting).

**Outcomes.** An evaluation either aborts with a run-time error of an action
or ends with an **outcome** $(s, q, v, \varepsilon, o)$ as in the
[Memoization proof](memoization.md#setting): match or fail, the end position,
the value, the recovered errors and the recorded farthest failure with its
expected items. It also leaves a state after it: the environment $\eta'$ and
the captures $C'$ in which the next element of a sequence is evaluated, and
whether a `#cut` that it passed commits the enclosing choice, option or
repetition (the cut **escapes**). A cut **is loose** in an expression when it
can escape from it: a cut outside every choice, option, repetition, lookahead
and rule call within the expression, as `HasLooseCut` in
`internal/grammaranalysis/success.go` decides.

**Scope.** The laws hold for parses that no limit stops (no nesting limit,
cancellation or compute limit) and that insert no `#missing` tokens, as for
the [Memoization proof](memoization.md#setting): an insertion depends on a
budget for the whole parse, which a second evaluation of the same expression
consumes again. The nesting limit ends a parse with a run-time error, so a
law that removes or adds calls of recursive rules can change whether a parse
near that limit ends that way.

**Facts.** The laws rest on the following.

- **(F1) Determinism.** The outcome and the state after an evaluation are a
  function of the expression, the state and the input
  ([Evaluation](../language/parser-expressions.md#evaluation)). An abort is
  too.
- **(F2) Failure has no effect.** A failed evaluation leaves the position,
  the captures and the variables as they were, and appends no recovered
  error; it records only its farthest failure.
- **(F3) Recording is idempotent.** The operation $\cdot$ of
  [Skip Runs](skip-runs.md#setting) that records an outcome is associative,
  and $(f, X) \cdot (f, X) = (f, X \mathbin{\Vert} X) = (f, X)$, so
  $c \cdot o \cdot o = c \cdot o$: recording an outcome that is already
  recorded changes nothing. Inside a lookahead nothing is recorded
  (assumption (A1) of the [Memoization proof](memoization.md#assumptions)).
- **(F4) Repeated evaluation.** By (F1) to (F3), evaluating an expression
  that failed in a state again in the same state fails with the same outcome
  and records nothing new. Only its cost repeats. An abort cannot repeat: the
  first one ended the parse.
- **(F5) States at the start of units.** A unit (an alternative other than
  the last, an iteration, the expression of an option or of a lookahead, the
  base of a `#recover`; see
  [Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost))
  and each of its continuations begin in the same $\eta$, $\lambda$ and
  $\Gamma$, and at the same position, except after a positive lookahead that
  matched: a failed alternative, iteration or option undoes its bindings
  (F2), a negative lookahead undoes its bindings, and the skip of a
  `#recover` starts at the start of the base. The same holds after a call or a
  literal that both begin with, since bindings in a called rule end with the
  call.

## Equivalences

Four relations compare two expressions $e$ and $e'$. Each requires its
condition in every state.

| Relation | Requires | Preserves |
|:--|:--|:--|
| $e \equiv_E e'$ | The same match or failure, the same end, the same $\eta'$, the same escape of cuts, and the same captures under every name that a predicate or an action reads | Which inputs a parse accepts, and where every enclosing expression ends, in a context that reads no value of $e$ in a predicate |
| $e \equiv_V e'$ | $\equiv_E$, the same value and the same recovered errors | The parse result: trees, values and recovered errors |
| $e \equiv_X e'$ | $\equiv_V$ and the same recorded farthest failure and expected items, `@error` messages included | Everything a parse returns, syntax errors included |
| $e \equiv_O e'$ | $\equiv_X$, the same aborts, and a cost within a constant factor of the grammar | Everything observable except traces and evaluation counts, and the cost class of the [classification](../runtime/parsing.md#classes-of-growth) |

Each relation is a congruence: replacing $e$ by $e'$ inside any expression
keeps the relation for the enclosing expression, because every operator reads
its operands only through the parts of the outcome that the relation fixes.
For $\equiv_E$ the context must not read the value of $e$ in a predicate, as
`{len($x) > 2}` reads `x:e`, since a predicate decides matching. A law that
holds only for $\equiv_E$ is a law about the **recognizer** of a grammar; the
analyses that bound cost or decide which alternative can match need no more.

A law written $e \equiv_{E,X} e'$ preserves the matching and the recorded
outcome but not the value. Cost is stated separately where a law changes it.

## Determinism and Calls

### Call Determinism

**Law (D1).** Two calls of a rule $A$ at one position with the same binding
level, the same values of the variables that $A$ reads, the same lookahead
state and the same open growths of the cycle of $A$ have one outcome
(Lemma 1 and the theorem of the [Memoization proof](memoization.md)), up to
the recorded outcome, which inside a lookahead is not recorded.

**Consequences.** A call is a symbol: an analysis that compares what two
evaluations do from one position may treat two such calls as one, as the
classifier does for **shared calls** and the auto-cut exclusion analysis does
for equal symbols. A FIRST set over calls, $\mathrm{FIRST}_k$ with calls as
symbols, is well defined for rules outside left-recursion cycles.

**Used.** Shared calls of
[Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost),
which require a rule that reads no variable; the memo
([Memoization](../runtime/parsing.md#memoization)).

### Equal Environments at Unit Starts

**Law (D2).** A call that a unit and its continuation both evaluate first, or
after the same shared calls and literals, is evaluated in the same $\eta$ in
both (F5). Its outcome is therefore one by (D1) even when the rule reads
variables, except where the unit is a positive lookahead whose expression
binds a variable.

**Proof.** By (F5) the unit and the continuation start in one state but for
the captures, which a call does not read. A shared call or literal leaves
$\eta$ unchanged, since bindings in a called rule end with the call and a
literal binds nothing. So the next shared call starts in equal states too.

**Counterexample.** After `&({v := 1})`, what follows runs with $v = 1$,
while the expression of the lookahead started with the earlier value.

The classifier requires a shared call to read no variable; (D2) shows that
the restriction is not needed at the start of a unit and its continuation.

### Free Variables

**Law (D3).** The outcome of a call of $A$ depends on the environment only
through the **free** variables of $A$: those that $A$, or a rule that it
calls, reads where no binding in the same invocation precedes the read on
every path. A variable that $A$ binds on every path before each read, as
`dollar_string = "$" t:dq_tag "$" {tag := text($t)} ... dq_close` binds `tag`
before `dq_close` reads it, does not reach the outcome. Unproved in full: it
needs the definite-binding analysis stated over every expression form, with
lookaheads (a negative lookahead's bindings never count), choices (the
intersection of the alternatives' bindings) and repetitions (none of the
element's bindings count after it).

**Used.** Not used. The memo key and the classifier take every variable that
a rule reads directly or through its callees (`ruleVariables` in
`internal/compile/analysis.go`).

## Sequences

**Law (S1) Associativity.** `(a b) c` $\equiv_O$ `a (b c)` $\equiv_O$
`a b c`. A parenthesized sequence in a sequence adds its elements to the
surrounding tuple ([Values](../language/parser-expressions.md#values-and-the-concrete-syntax-tree)),
and each element is evaluated at the end of the one before.

**Law (S2) Bottom.** `_|_ a` $\equiv_O$ `_|_`: `a` is never evaluated and
`_|_` records nothing. `a _|_` $\equiv_E$ `_|_` only: `a` records its
expectations and can abort.

**Law (S3) Top.** As an element of a sequence that keeps at least one other
element with a value, `_` can be removed ($\equiv_O$): it has no value and
consumes nothing. Removing it from `_ "x"` gives `"x"`, whose value is a
string node and not a tuple of one, so the condition is needed.

## Ordered Choice

**Law (C1) Idempotence.** `a / a` $\equiv_O$ `a`.

**Proof.** If `a` matches, both match alike. If `a` fails without a cut
escaping, the second `a` fails in the same state with the same outcome
(F4). If a cut escapes before `a` fails, the choice fails at once, as `a`
alone does. The second attempt at most doubles the cost of the site.

**Used.** `shadowed-alternative`, case 3 (`covers` in `lint/checks.go`).

**Law (C2) Associativity.** `a / (b / c)` $\equiv_O$ `a / b / c` always.
`(a / b) / c` $\equiv_O$ `a / b / c` when neither `a` nor `b` has a loose
cut.

**Proof.** A cut in `b` or `c` of `a / (b / c)` commits the inner choice; the
inner choice is the last alternative, so its failure fails the outer one, as
the commit of the flat choice does. In `(a / b) / c` a cut in `a` commits only
the inner choice, and `c` is still tried.

**Counterexample.** `("a" #cut "b" / "x") / "a" "c"` matches `ac`;
`"a" #cut "b" / "x" / "a" "c"` does not.

**Law (C3) Dead Suffix.** If `a` always succeeds, `a / b` $\equiv_O$ `a`.
Always succeeding is decided by `AlwaysSucceeds`
(`internal/grammaranalysis/success.go`), which treats an alternative that
can fail after a loose cut as not proving success.

**Used.** `shadowed-alternative`, case 1; the left-recursion analysis, which
leaves out choice suffixes after an alternative that always succeeds
([Left Recursion](../language/parser-expressions.md#left-recursion)).

**Law (C4) Bottom Alternatives.** `_|_ / b` $\equiv_O$ `b` and
`a / _|_` $\equiv_O$ `a`, unless the `_|_` carries an `@error` attribute,
whose message it records ($\equiv_V$ only).

**Law (C5) Prefix Subsumption.** `a / a b` $\equiv_O$ `a`: when `a` fails,
`a b` fails in the same state with the same outcome (F4). More generally
`a / b` $\equiv_O$ `a` when every input on which `b` matches begins with
input on which `a` matches, as for `"=" / "=="`.

**Used.** `shadowed-alternative`, case 2 (`covers`, `matchPrefix`).

**Law (C6) Left Factoring.** `a b / a c` $\equiv_X$ `a (b / c)` when `a`, `b`
and `c` have no loose cut. It is $\equiv_O$ when moreover `b` and `c` are each
a single element with a value; otherwise only $\equiv_{E,X}$.

**Proof.** If `a` fails, both fail with the outcome of `a` (F4). If `a`
matches and `b` matches, both match with the same elements. If `a` matches
and `b` fails, the left side undoes `a`, evaluates `a` again in the same
state, which gives the same outcome (F1, F3), and then `c` at the same
position and in the same state as the right side does. The left side
evaluates `a` twice; the right side once, which can lower the cost class when
`a` contains a scanning source.

**Counterexamples.** With a cut in the shared prefix,
`"a" #cut "b" / "a" #cut "c"` rejects `ac`, and `"a" #cut ("b" / "c")`
accepts it. With a cut in `b` and a later alternative `d`, the cut of
`a b / a c / d` commits the whole choice, while in `a (b / c) / d` it commits
only the inner one. The values differ when `b` is a sequence: `"a" "b" "d" /
"a" "c"` produces a tuple of three elements on `abd`, and
`"a" ("b" "d" / "c")` a tuple of two, the second a tuple.

**Law (C7) Right Factoring.** `a c / b c` $\equiv_V$ `(a / b) c` when `a` and
`b` are **exclusive** (no state in which both match) and have no loose cut,
and `a` and `b` are each a single element with a value. Without exclusion it
fails: `"a" "c" / "ab" "c"` accepts `abc`, and `("a" / "ab") "c"` rejects it.
It is not $\equiv_X$: when `a c` fails after `a` matched, the left side also
records the failure of `b`.

**Law (C8) Exclusive Commutativity.** `a / b` $\equiv_V$ `b / a` when `a` and
`b` are exclusive and have no loose cut. Two expressions that cannot match
without consuming input and whose first characters are disjoint are
exclusive. The recorded expectations at the start position can differ, since
each order attempts only the alternatives before the one that matches.

**Counterexamples.** Without exclusion, `"a" / "ab"` and `"ab" / "a"` differ
on `ab`. With a cut, `"a" #cut "x"` and `"a" "y"` are exclusive, but
`"a" #cut "x" / "a" "y"` rejects `ay` and `"a" "y" / "a" #cut "x"` accepts it.

**Law (C9) Character Merge.** A run of consecutive alternatives that each
match one character, a character class, a case-sensitive literal of one code
point or `.`, without captures, is $\equiv_V$ one class of their union: each
alternative produces a string node of the character that it matched, with
its range. The expected items differ: the alternatives record `"a"` and
`[0-9]`, the merged class records `[0-9a]`. The order of the alternatives does
not matter, because at most one character is consumed and every alternative
that matches consumes the same one.

## Repetition

**Law (R1) Never Failing.** `X*` and `X?` always match when `X` has no loose
cut. A loose cut makes them fail when an iteration fails after it
([Cut](../language/parser-expressions.md#cut)): `("a" #cut "b")* "a" "c"`
rejects `ac`, while `("a" "b")* "a" "c"` accepts it.

**Used.** `AlwaysSucceeds` checks loose cuts for options and repetitions. The
effect summary of Core IR (`Analyze` in `internal/ir/core/attrs.go`) clears
the effect "may fail" of every option, every repetition with a minimum of 0
and every `#recover`, whatever cuts or skips they contain, so it
under-approximates failure there; nothing reads that effect today.

**Law (R2) Greedy Maximality.** `X* X` $\equiv_E$ `_|_` when `X` cannot match
without consuming input.

**Proof.** When `X*` matches up to $q$, it stopped because `X` failed at $q$
in the state after the last iteration, or because an iteration matched
without consuming input, which `X` cannot. The `X` after it is evaluated in
that same state and fails again (F4). When `X*` fails, the sequence fails.
The recorded outcome is that of `X* _|_`, so `X* X` $\equiv_X$ `X* _|_`.

**Law (R3) Idempotence.** `X* X*` $\equiv_{E,X}$ `X*` when `X` cannot match
without consuming input, or `X` makes no binding that `X` reads.

**Proof.** If the first `X*` fails, so does `X*`. Otherwise it ended at $q$
either because `X` failed at $q$ without a cut escaping, in a state that the
second `X*` starts in, so its first iteration fails alike (F4) and it matches
nothing; or because an iteration matched at $q$ without consuming input. In
that case the second `X*` evaluates `X` at $q$ in the state after that
iteration, which differs from the state of the iteration only by its
bindings; when `X` reads none of them, `X` matches without consuming input
again, and the second `X*` ends at $q$ after one iteration. The value is a
tuple of two lists, not one list.

**Counterexample.** With `X = {v == 0} {v := 1} / {v == 1} "a"` written in
place, `X* X* $$` accepts `a` and `X* $$` rejects it: the empty iteration
binds `v`, and the second repetition consumes `a`. A call of a rule whose body
is `X` does not show this, because the binding ends with the call.

**Law (R4) Repetitions in a Row.** For an `X` that cannot match without
consuming input: `X+ X*` $\equiv_{E,X}$ `X+`; `X? X*` $\equiv_{E}$ `X*`;
`X* X+` $\equiv_E$ `_|_` and `X+ X+` $\equiv_E$ `_|_`, by (R2). The last two
are not $\equiv_X$, since the left sides record the failure of `X`.

**Law (R5) Unfolding.** `X+` $\equiv_{E,X}$ `X X*` and `X*`
$\equiv_{E,X}$ `(X X*)?` when `X` has no loose cut and (R3) holds for `X`. A
loose cut of the first `X` in `X X*` commits the choice around the sequence
instead of the repetition.

**Law (R6) Nested Repetitions.** `(X*)*`, `(X+)*`, `(X*)+` and, for an `X`
that cannot match without consuming input, `(X?)*` are $\equiv_{E,X}$ `X*`
when `X` has no loose cut and (R3) holds for `X`. The outer repetition stops
after its second iteration, which matches nothing; its list has that empty
element.

**Used.** The fix of `nullable-repetition` suggests these forms.

**Law (R7) Unrolling.** `X* (Y X*)*` $\equiv_{E,X}$ `(X / Y)*` when `X`
cannot match without consuming input, neither `X` nor `Y` has a loose cut, and
`Y` makes no binding that `X` reads.

**Proof.** At each position both evaluate `X` first and `Y` after `X` fails,
in the same order and the same states. An iteration of `(X / Y)*` that matches
without consuming input is one of `Y`, after `X` failed; the unrolled form
then evaluates `X*` there, in the state after `Y`, which differs from the state
in which `X` failed only by the bindings of `Y`. `X` reads none of them, so it
fails again (F4), `X*` matches nothing, and the outer repetition stops, as
`(X / Y)*` does. That evaluation of `X` is the one cost that the unrolled form
adds: one evaluation of `X` again in a state in which it failed, for each
evaluation of the whole expression in which `Y` matches without consuming input.

**Counterexamples.** With a nullable `X`, `"a"? ("b" "a"?)* $$` accepts `b`,
while `("a"? / "b")* $$` stops after the empty match of `"a"?` and rejects it.
With `var v int = 0` and `x = {v == 1} "a"`, `x* ({v := 1} x*)* $$` accepts
`a`: the empty match of `{v := 1}` binds `v`, and `x*` after it consumes `a`.
`(x / {v := 1})* $$` stops after that empty iteration and rejects `a`.

## Lookahead

**Law (K1) Double Negation.** `!!X` $\equiv_O$ `&X` when `X` makes no binding
outside the rules that it calls. A positive lookahead keeps its bindings and
a negative one undoes them; both discard the errors recovered inside them
(V2). `!!({v := 1}) {v == 1}` fails where `&({v := 1}) {v == 1}` matches.
(`!` admits no capture.)

**Law (K2) Stacked Lookaheads.** `&&X` $\equiv_O$ `&X`, `!&X` $\equiv_O$
`!X`, `&!X` $\equiv_O$ `!X` and `!!!X` $\equiv_O$ `!X`, without conditions.

**Law (K3) Repeated Lookahead.** `&X &X` $\equiv_E$ `&X`, and `&X X`
$\equiv_V$ `X`, when `X` makes no binding that `X` reads: the second
evaluation starts in the state after the first, which differs only by
bindings, and the lookahead discards the errors that it recovered (V2), so
they are appended once.
`&({v := v + 1} "a") {v := v + 1} "a" {v == 1}` fails where
`{v := v + 1} "a" {v == 1}` matches.

**Law (K4) Self Exclusion.** `!X X` $\equiv_E$ `_|_`, for every `X`: when
`X` matches, `!X` fails; otherwise `X` fails in the same state. It is not
$\equiv_X$: the right `X` records its expected items. Grammars use this to
add expected items to a syntax error without matching anything.

**Law (K5) Exclusion.** `!X Y` $\equiv_E$ `_|_` when every input on which `Y`
matches begins with input on which `X` matches, as in `!"if" "if" [a-z]*`.
Its weaker forms decide the **words** of
[Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost),
which exclude `ident = !keyword [a-z]+` after a word `"let" !idchar`.

**Law (K6) Trivial Lookaheads.** If `X` always succeeds, `!X` $\equiv_O$
`_|_`, and `&X` $\equiv_O$ `_` when `X` makes no capture or binding. If `X`
never matches, `!X` $\equiv_O$ `_`.

**Used.** `useless-lookahead` and its rewrite (`lint/rewrites.go`), which
removes such a positive lookahead only where the shape of `X` proves the
condition.

## First Characters

**Law (F1) First Character.** If `e` cannot match without consuming input
and the character $c$ at $p$ is not one of the first characters of `e`, then
`e` fails at $p$, examining only $c$, and records the failures of its first
terminals at $p$.

**Used.** `Guards` in `internal/ir/core/passes.go` skips an alternative whose
first terminal cannot match, recording the expectation of that terminal and
entering the calls of recursive rules that the evaluation would enter, so
that the nesting limit is kept ($\equiv_O$).

**Law (F2) Filtered First Characters.** For a unit `&L e` in which `L` is a
character class, a literal or `.`, and `e` cannot match without consuming
input, the unit consumes a first character only from the intersection of the
first characters of `L` and `e`, and when $c$ is outside the first characters
of `L` it fails examining only $c$. A disjointness check of a unit against its
continuations may therefore use the intersection with the first character of
`L` in place of the union.

**Used.** Not used. The classifier takes the first characters of `&L e` as
the union of those of `L` and `e` (`first` in
`internal/compile/complexity_chars.go`), which is needed for a lookahead that
examines more than one character.

## Rule Calls

**Law (P1) Inlining.** A call of a rule `A` that is not on a left-recursion
cycle and has no action is $\equiv_E$ its body when the body has no loose
cut, makes no binding and has no capture. A loose cut would escape into the
caller, a binding would stay visible after it, and a capture would join the
scope of the caller. The value is that of the body, except that nodes that
the body creates carry the name of `A` with
[source information](../runtime/nodes.md#source-information). The memo, the
nesting limit and the cost class can change.

**Law (P2) Idempotent Rules.** A call `A A` $\equiv_{E,X}$ `A` when the body of
`A` is, through groups, `#atomic` and discards, `Z*` or the unrolled form
`X* (Y X*)*` of (R7), and `Z` (or `X` and `Y`) makes no binding that it
reads, calls no rule of a left-recursion cycle, and, for the unrolled form,
`X` cannot match without consuming input.

**Proof.** The second call starts at the end $q$ of the first, in the
caller's environment, since the bindings of the first call ended with it. The
first call stopped at $q$ because `Z` failed there, or matched there without
consuming input, in the environment of the last iteration; that environment
differs from the caller's only by bindings that `Z` does not read, so `Z`
behaves alike in the second call, which ends at $q$ (R3, R7). The second call
records the failure of `Z` at $q$ again, which is already recorded (F3). It
repeats the cost of that last attempt, which is not a constant: a `Z` that
scans an unterminated comment before failing scans it again. Predicates over
captures, captures and reads of variables that `Z` does not bind are allowed.

**Counterexample.** With
`w = ({v == 0} "b" {v := 1} / {v == 0} "a")*`, `w w $$` accepts `ba` and
`w $$` rejects it: the first call stops at `a` because it bound `v`, and the
second call starts with the caller's `v`.

**Law (P3) Self-Annihilating Rules.** `A A` $\equiv_E$ `_|_` when the body of
`A` is `Z+` with a `Z` as in (P2) that cannot match without consuming input,
by (R4). `[ \t\r\n]+` is such a body.

**Law (P4) Settled Rules.** A rule `A` is **settled** when a call of `A` at
the end of a match of `A` examines at most a constant number of characters
and its outcome depends only on the position and the variables that `A`
reads. Rules of the shapes of (P2) and (P3) whose `Z` begins with a single
character test and contains no repetition or call that scans further are
settled. A `Z` that scans further is not: in
`ws = ([ \t] / "/*" (!"*/" .)* "*/")*`, a second call at the end of a match
that stopped before an unterminated comment scans the comment to the end of
the input again. `s_indent = s:" "* #atomic {len($s) == n}` is
settled but not idempotent: the second call matches nothing and succeeds only
when `n` is 0. `[ \t]+ / ^` is settled: after spaces, both alternatives fail
at once, and after `^` the call can consume spaces, so `A A` is not `A`.

## Cut

**Law (T1) Redundant Cut.** `a #cut b / c` $\equiv_V$ `a b / c` when no
state in which `a` matches lets `c` match, that is, `a` and `c` are exclusive
after the prefix. It is not $\equiv_X$: with the cut, `c` is not attempted and
records nothing. The cut can lower the cost class, since `c` no longer starts
again inside the input that `a b` examined.

**Law (T2) Harmless Cuts.** A cut that ends an alternative, `a #cut / b`, and a
cut that is loose in the last alternative of a choice, `a / b #cut c` or
`a / -(b #cut c)`, give the same match, end, captures and recorded outcome
($\equiv_{E,X}$) as the same expressions without the cut: nothing can fail
after the first, and no alternative follows the second. A cut at the start,
`#cut a / b`, is $\equiv_{E,X}$ `a`: `b` is never attempted.

They are $\equiv_O$ only when the value stays the same. A cut has no value, so
`b #cut c` and `b c` are both a tuple of the elements `b` and `c`. But
`a #cut` is a tuple of one element, whereas `a` is the value of `a` itself
([Values](../language/parser-expressions.md#values-and-the-concrete-syntax-tree),
rule 2), and `#cut a` too; a predicate that reads the capture `x:(a #cut / b)`
sees the difference, as `len($x) == 1`. The sequence that is left with one
item, or with none, is $\equiv_O$ to the sequence of that item (`a #cut` to a
sequence of `a`) and to the bare item only where no predicate reads the value.
The same holds for a choice that is left with one alternative after the others
are dropped (C1 to C5, T2): that alternative is $\equiv_O$ to the choice of it,
and to the alternative alone only where its value is the same alone as it is
as the only alternative of a choice, that is, unless it is a sequence, or a
call of a splice rule (a hidden rule without an action) whose body is one,
which adds its elements to a sequence around it. An alternative without a value
has none alone, and a choice of which it is the only alternative has none
either: a choice of which no alternative has a value adds no element. So
`(_r / _|_)` with `_r = -"a"` is $\equiv_O$ to `_r`, while a choice that keeps
another alternative with a value, as `(-"a" / "b")`, is `nil` when the
alternative without a value matched.

A choice of which one alternative is left after the others are dropped is
replaced by that alternative only without its loose cuts: a loose cut left in
it would commit the choice, option or repetition around the choice instead, as
`(-("a" #cut "b") / !"q" "q")* "ac"`, which accepts `ac`, and
`(-("a" #cut "b"))* "ac"`, which rejects it.

**Law (T3) Cuts Stop at Calls.** A cut in the body of a rule never escapes
its call ([Cut](../language/parser-expressions.md#cut)), so inlining a rule
with a loose cut changes the choice that the cut commits (P1).

## Recovery

**Law (V1) Recovery That Never Fires.** `e #recover(skip=s)` $\equiv_{E,X}$ `e`
when `e` always succeeds, and $\equiv_O$ `e` when its value also stays the same.
When `s` cannot consume input, the recovery never applies and the expression
is $\equiv_E$ `e`, and $\equiv_V$ `e` when its value stays the same; it is not
$\equiv_X$, since a failure then also expects the first items of `s`.

The value stays the same unless the recovery shapes it. A recovery makes an
`e` with a value one element, so an `e` that a sequence around it splices (a
sequence, grouped or with attributes other than recovery, or a call of a hidden
rule without an action whose body is one) gives its tuple node, where without
the recovery that sequence adds the elements of `e`; a predicate that reads
the capture `x:("q" ("a"? "b"?) #recover(skip="z"))` sees the difference, as
`len($x) == 2`. An `e` that no sequence splices has the same value either way,
and so does an `e` without a value, which the recovery leaves without one
([`#recover`](../language/pragmas.md#recover)).

**Law (V2) Recovery in a Lookahead.** A lookahead, positive or negative,
discards the errors recovered inside it when it ends: `&X` matches where `X`
matches by recovering, and appends no recovered error. So the recovered
errors of a lookahead are never observable, and (K1) and (K3) hold for an `X`
that recovers.

## Normalization

A normal form decides several laws by comparing shapes. The rewrite system
below has two levels. The rules of **level O** preserve $\equiv_O$, so an
optimizer may apply them to the program it runs. The rules of **level E**
preserve $\equiv_E$, and most also the recorded outcome, so an analysis of
matching or cost may apply them to a view of the grammar, but a parser must
not run the result. Every rule applies to an expression in any context,
since each relation is a congruence, under the conditions of the law it
cites.

| Rule | Level | Rewrite | Law |
|:--|:--|:--|:--|
| N1 | O | Splice groups of sequences into sequences; flatten right-nested choices; flatten left-nested choices without loose cuts | S1, C2 |
| N2 | O | Drop the alternatives after one that always succeeds | C3 |
| N3 | O | Drop an alternative equal to an earlier one, or one that an earlier one subsumes | C1, C5 |
| N4 | O | Drop `_|_` alternatives without `@error`; replace `_|_ a` by `_|_` | C4, S2 |
| N5 | O | Canonicalize stacked lookaheads; replace trivial lookaheads | K1, K2, K6 |
| N6 | O | Drop harmless cuts and recoveries that never fire; a sequence left with one item or none becomes a sequence of it, and a recovery that makes its expression one element stays | T2, V1 |
| N7 | E | Remove `@error`, `@diagnostic`, `#atomic`, discards and captures that no predicate reads | (values and messages only) |
| N8 | E | Merge runs of single-character alternatives into one class | C9 |
| N9 | E | Replace `X* X*` by `X*`, `X+ X*` by `X+`, `X? X*` by `X*`, `X X*` by `X+` | R3, R4, R5 |
| N10 | E | Flatten nested repetitions | R6 |
| N11 | E | Replace `X* (Y X*)*` by `(X / Y)*`, and `X* (X / Y)*` by `(X / Y)*` | R7 |
| N12 | E | Replace `!X X` by `_|_`; left-factor alternatives | K4, C6 |
| N13 | E | Inline calls of rules that (P1) allows, outside left-recursion cycles | P1 |

N8 is level O when the merged class keeps the expected items of the
alternatives it replaces, which Core IR can represent by giving the class the
list of expectations that it records.

**Termination.** Every rule but N13 shortens the expression or keeps its
length and lowers the number of alternatives, repetitions or annotations; N11
removes one copy of `X`. N13 replaces a call by a body that calls only rules
lower in the order of the acyclic part of the call graph, so the multiset of
the ranks of calls decreases. The order that compares first that multiset and
then the size is well founded, so rewriting terminates.

**Confluence.** Not proved. Soundness does not need it: every rewrite
preserves its relation, so any normal form that the rules reach decides a law
soundly. Confluence would make the decision complete for the rules. One
critical pair is known and closed: from `X* X* (Y X*)*`, N9 gives
`X* (Y X*)*` and then `(X / Y)*`, while N11 first gives `X* (X / Y)*`, which
the second form of N11 joins.

**Normal form.** An expression in normal form has no grouped sequence, no
nested repetition, no adjacent repetitions of one expression, no alternative
after one that always succeeds, no two adjacent single-character
alternatives and, at level E, no annotation that affects only values or
messages.

**Decided laws.** On normal forms the following are decided by the shape.

- A rule is **idempotent** (P2) when its normal form is `Z*` and the conditions
  of (P2) on `Z` hold. `ws = ([\n]* / [\t]*) @error("e")` normalizes by N7 and
  N2 to `[\n]*`, so `ws ws` $\equiv_{E,X}$ `ws`; the `@error` message is
  recorded twice and kept once (F3), so the law holds for the original rule
  too. `ws` is not `([\n] / [\t])*`: `[\n]*` always succeeds, so `[\t]*` is
  never attempted, and `ws $$` rejects `\n\t`.
- A rule is **self-annihilating** (P3) when its normal form is `Z+`.
- An alternative is **dead** when N2 or N3 removes it.
- The **first characters** of a normal form are computed after N8 and N11,
  which turn unrolled whitespace such as `[ \t]* ("//" [^\n]* [ \t]*)*` into
  `([ \t] / "//" [^\n]*)*`.

The whitespace rules of the bundled grammars normalize as follows. Rules of
the form `[...]* ("//" [^\n]* [...]*)*` or `[...]* (comment [...]*)*`
normalize by N11 to `Z*` and are idempotent. `" "* #atomic` normalizes by N7
to `" "*`. `[ \t\r\n]+` is self-annihilating, not idempotent. A body that is
`Z*` whose `Z` makes captures and predicates over them, or reads a variable
that it does not bind, is idempotent by (P2). `[ \t]+ / ^` and
`s:" "* #atomic {len($s) == n}` are settled (P4) but not idempotent.

**Cost view.** A level E rule preserves what a grammar matches, not what its
parse costs: dropping or merging an expression drops evaluations that the
original makes. An analysis of cost may read a view only when the original
costs at most a constant factor more than the view on every input. The
classifier reads a view that applies the rules of both levels except N7 and
N13, under these further conditions, which give that bound for each rule that
the view applies ([Cost View](cost-view.md)), except for the values that actions
build from a value that a rewrite reshapes, and their run-time errors: step 8 of
the [open steps of the linear cost proof](linear-cost.md#open-steps). The
rewrites that reshape a value are N9, N10, N11, the left factoring of N12, the
replacement of `!X X` by `_|_` (N12), the drop of every alternative with a value
of a choice that keeps one without (N2, N3), the replacement of a sequence of
one item, or a choice of one alternative, by its item (in N1, in N6 and after
the other alternatives are dropped), and the drop of a recovery that makes its
expression one element (V1 in N6), where the conditions below let them change
the value.

- A rule removes an evaluation only when the original either never makes it
  or makes it again in the same state, and only when that evaluation is
  cheap to make again: it calls only memoized rules outside call cycles, or
  rules without actions whose bodies are cheap in this sense, and it contains
  no predicate, inline action, recovery, stream, operator rule or `#missing`.
  Repeating such an evaluation costs a constant factor.
- N12 replaces `!X X` by `_|_` only when `X` matches a bounded number of
  characters without calls, and left-factors a run of alternatives only by
  its whole shared prefix, only when that prefix is cheap, and never when an
  alternative has a loose cut.
- N9, N10, N11 and the left factoring of N12 are $\equiv_{E,X}$ at most, so
  the view applies them only where no predicate can observe the value of
  the rewritten expression. A predicate observes a value only by naming a
  capture, as `$x`, `$x.$y` or the elements of `$xs`; a variable that a
  binding sets from a capture is set by a predicate that names it, and
  positional references are not part of the language. So the view applies
  them neither inside a capture whose name a predicate of the rule reads,
  nor in the body of a rule whose value reaches such a capture, through the
  rules it calls in positions that keep their value, transitively. A call
  keeps its value except under a discard, a negative lookahead, and a
  positive lookahead outside its captures: a capture in a positive
  lookahead keeps its value, which a predicate reads. Nor does the view
  apply them to an expression that contains such a capture, or that calls
  such a rule, since they would move or merge the capture that the
  predicate names.
- The same holds for the parts of the other rules that change a value. A
  sequence left with one item (or none) after N6 drops its cuts, or whose
  items N9 to N11 merged, would become the bare item, which has another
  value than the tuple of one element; a choice left with one alternative
  that is a sequence, or a call of a splice rule whose body is one, or that
  has no value would add its elements to the sequence around it, or lose the
  element `nil`; an inlined call (N13) does the same
  with the body of its rule. The view applies them only where the conditions
  above hold, and otherwise keeps the sequence of one item or the choice of
  one alternative, and the call. Dropping the cut is not changing a value, so
  it applies everywhere. A recovery that V1 drops makes an expression with
  a value that a sequence splices one element; it stays where the conditions
  above do not hold.
- N2 and N3 drop alternatives that never give the value of the choice, but
  they can drop every alternative with a value and keep one without: the
  choice of `_ / "b"` is one element, `nil` or the string, and `_` alone none.
  N12 replaces `!X X`, which has the type of `X`, by `_|_`, which a choice
  leaves out of the join of its alternatives, so `-"b" / !"a" "a"` is one
  element and `-"b" / _|_` none. The view applies N2, N3 and the replacement
  of `!X X` neither inside a capture whose name a predicate of the rule reads
  nor in the body of a rule whose value reaches one. A `_|_` alternative adds
  nothing to the value of the choice, so N4 applies everywhere.
- N7 is off because `@error` and `@diagnostic` decide where the parser keeps
  repetition runs, and N13 is off because inlining changes which rules are
  memoized. For the same reason the classifier decides memoization, call
  cycles, variables and repetition runs on the original rules and reads the
  view only for the shape of each body.
