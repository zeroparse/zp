# Memoization

Arata Furukawa · Last revised 2026-11-12

This proof shows that [memoization](../runtime/parsing.md#memoization) never
changes the result of a parse. A parse that answers calls from the memo returns
the value, the recovered errors and the syntax error of a parse that evaluates
every call, including calls of the rules of left-recursion cycles and calls
whose results read the growing result of a left recursion. The proof also
states what a memo entry must satisfy to be reused, so that the
[Document proof](documents.md) can extend the same induction to entries and
repetition runs that earlier parses stored.

## Setting

Fix a grammar, a start rule, a position unit and an input $t$ of length $n$.
The parse records expectations or not; the proof covers both.

**Calls and their keys.** A call of a rule $A$ at a position $p$ has a binding
level $\ell$ (none, or the level of `level(A, name)`), the values $\eta$ of
the variables that $A$ reads, directly or through the rules it calls (see
[Interaction With Memoization](../language/predicates.md#interaction-with-memoization)),
and a lookahead state $\lambda$. The lookahead state is $1$ inside a
[lookahead](../language/parser-expressions.md#lookahead) or the
[`skip` of an Operators Rule](../language/operators-rule.md#skip), and $0$
elsewhere. The **key** of the call is $\kappa = (A, p, \ell, \eta)$.

**Outcomes.** The evaluation of an expression or a call has an **outcome**

$$\omega = (s, q, v, \varepsilon, o)$$

where $s$ is match or fail, $q$ is the end position ($q = p$ on failure), $v$
is the value, $\varepsilon$ is the sequence of errors that
[`#recover`](../language/pragmas.md#recover) recovered in it, and $o = (f, X)$
is its farthest failure and the expected items there, with the operation
$\cdot$ of [Skip Runs](skip-runs.md#setting). On failure $\varepsilon$ is
empty, because a failure has no effect
([Evaluation](../language/parser-expressions.md#evaluation)). An evaluation can
instead abort with a run-time error $E$ of an action, which is then its
outcome. Each evaluation also has an **examined range** $\rho$, the interval of
the input that it and the calls within it examined, as defined in the
[Document proof](documents.md#setting).

The caller **records** an outcome by continuing at $q$ when $s$ is match,
appending $\varepsilon$ to its recovered errors, replacing its own record $c$
by $c \cdot o$ and its range by the hull with $\rho$.

**Growths.** A call of a leader $L$ at $p$ is evaluated by growing a seed
([Left Recursion](../language/parser-expressions.md#left-recursion)). The
growth of $L$ at $p$ is **open** from the start of that call to its end, and
while it is open it has a **current result**. A call of $L$ at $p$ while the
growth is open is answered with the current result, whatever its $\eta$. A
call **reads** an open growth when it, or a call evaluated within it, is
answered with that growth's current result. The **context** of a call of $A$
at $p$ is $\Gamma$, the sequence of the growths of the leaders of the cycle of
$A$ that are open at $p$ when it starts, each with its leader and its current
result, and its **present set** $P$ is the set of those leaders. By Lemma 2 a
call reads no other open growth. A lookahead at $p$ in a context can **observe** its outcome: whether
its expression matched, and its examined range; (D) states which lookaheads do.

**The two parses.** The **reference parse** $\mathcal{R}$ evaluates every call
as the language chapters define it: the body of the rule is evaluated, except
that a leader grows, and a call of a leader at the position where it grows
reads the current result. Nothing else is reused. The **memoized parse**
$\mathcal{M}$ does the same, and in addition keeps a table of **entries**
$(\kappa, \lambda) \mapsto (\omega, \rho)$. It may store the outcome of a call
that it evaluated, and it may answer a later call from an entry instead of
evaluating it, under these conditions:

- **(S) Storing.** A call that reads a growth that was open when the call
  started is not stored in the table. This applies to every rule, including a
  leader that grew inside the growth of another leader. An entry records the
  present set $P$ of the call that stored it.
- **(L) Lookahead state.** An entry stored in state $\lambda$ answers a call in
  state $\lambda'$ only if $\lambda \le \lambda'$.
- **(C) Contexts.** A call of a rule of a left-recursion cycle with the present
  set $P'$ is answered from an entry with the present set $P$ only if
  $P' \subseteq P$.
- **(D) Dependent results.** While a growth is open at $p$, $\mathcal{M}$ may
  also keep, apart from the table, the outcome and range of a call at $p$ that
  read open growths, with its key, state and present set $P$, the current
  results that it read, and the observations of the lookaheads at $p$ within
  it that read one. It answers a call with the same key, the present set $P$
  and a lookahead state at least the state of the result, as (L) allows for an
  entry, from it only if each growth that it read is open with an
  equal current result and each observed lookahead has the same observation,
  as a result kept in the same way gives it. Only a lookahead that leaves
  nothing after it but whether its expression matched is observed: a negative
  lookahead, or a positive one whose expression makes no capture and binds no
  variable outside the rules that it calls. The growths that another
  lookahead reads count as read by the call. A call that $\mathcal{M}$ answers from what it keeps
  counts, for the calls around it, as reading the growths that it reads in
  $\mathcal{R}$ there with their current results, those that its observed
  lookaheads read included. It treats a step of a growth, the evaluation of
  the body of a leader from a current result, in the same way. It may also
  take at once a run of steps of a growth of $L$ that read no other growth,
  from a current result equal to the one that the run started from, but only
  if each lookahead that the steps observed has the same observation now and
  that observation, as a result kept in the same way gives it, read no current
  result of $L$, directly or through the lookaheads that it observed. It drops
  what it keeps when the first growth open at $p$ ends.

(S) is the last item of
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules) that
concerns the table, and (C) and (D) state the item on contexts; (L) is
assumption (A2) below, which applies to what (D) keeps as to an entry. Which
outcomes $\mathcal{M}$ stores, and when it uses them, is otherwise free. Storing none, storing from the second call at a
position, or not storing the results of some rules are all instances of
$\mathcal{M}$. A test of equality of two current results that only ever
claims equality for equal results, such as one that compares identities given
to equal results, is an instance too: it can only make $\mathcal{M}$ reuse less.

**Parses that record no range.** Examined ranges matter only where a parse
records them, as [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)
lists: in every parse of the engine, whose traces report them, and in a
[Document](../runtime/documents.md) parse of a generated Go parser. A parse that records
no range may, in (D), take two observations to be the same when the
expressions of both lookaheads matched or both failed, and two current results
to be equal whatever their ranges. Lemma 4 asks nothing of ranges and gives
$y$ its outcome all the same: an observed lookahead leaves nothing
after it but whether its expression matched, and its range enters only the
range of the call around it. For such a parse the proof below holds
with the ranges left out, that is, with soundness, Lemma 3, Lemma 4 and the
invariant reduced to outcomes, dropping (I2). The cases of the theorem derive
outcomes only from outcomes, so (I1), and with it the theorem, holds unchanged.

**Scope.** The proof covers a parse that no limit stops: no
[nesting limit](../runtime/parsing.md#limits), cancellation, compute limit or
`MissingTokenLimitError`. A call answered from an entry does not nest, so
whether a parse reaches the nesting limit can depend on memoization, as
[Memoization](../runtime/parsing.md#memoization) states. It covers parses
without insertion of [`#missing`](../language/pragmas.md#missing) tokens,
since an insertion depends on the path to it and on a budget for the whole
parse. A walk of a [skip-run site](../runtime/parsing.md#skip-runs) that
reuses a record ends where evaluating the rest of the walk would end, with
the same farthest failure, expectations and examined range, as the
[Skip Runs proof](skip-runs.md#lemma-1-exactness) shows, so the proof treats
each walk as evaluated in full in both parses.

## Assumptions

The proof relies on the following rules of the specification, which it cites.

- **(A1) Recording in a lookahead.** Inside a lookahead or the `skip` of an
  Operators Rule, an evaluation records neither expected items nor its
  farthest failure, so recording an outcome there is the identity
  ([Syntax Errors](../runtime/parsing.md#syntax-errors)).
- **(A2) Reuse across lookahead states.** Condition (L), which
  [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules) states.
- **(A3) Comparisons.** `==` and `!=` never compare nodes: they compare two
  `int`, two `string` or two `bool` values, and any value with `nil`, and every
  other comparison is an error
  ([Operators](../language/actions.md#operators-and-functions)). A comparison
  of two scalars depends only on the values, and a comparison with `nil` only
  on whether a value is `nil`, which is part of the value. By (I1) of the
  theorem a comparison therefore decides alike in both parses. An error is
  raised for the same operands in both, so it is the run-time error of $s$.

## Agreeing Values

Two values **agree** when they differ at most in the messages of error
values: they are the same `int`, `string` or `bool`, or both `nil`, or both
error values with the same range, or nodes, structs, tuples or lists of the
same kind and the same type name, with the same range where they have one,
whose values, fields, captures and elements agree in order. Agreement is an
equivalence relation.

**Actions on agreeing values.** Let an action or a predicate be evaluated
twice, at the same position, with the same values of the variables and with
captures whose values agree. Then both evaluations raise the same run-time
error, or neither does; if neither does, their values agree, and a predicate
succeeds in both or fails in both.

*Proof.* By induction on the action expression. The rules of
[Actions](../language/actions.md) define each operation, and each run-time
error, from the kinds, type names, ranges, fields, captures, elements and
scalars of its operands, which agreeing values share, and none of them reads
the message of an error value:

- A literal is the same in both. A variable holds an `int`, a `string` or a
  `bool` ([Variables](../language/predicates.md#variables)), which is the same
  in both. A capture reference has values that agree by the assumption, and a
  parameter of a lambda is bound, by the induction hypothesis, to values that
  agree.
- The operators accept scalars, and `==` and `!=` also any value with `nil`
  ([Operators](../language/actions.md#operators-and-functions)). Agreeing
  scalars are equal and agreeing values are `nil` in both or in neither, so
  the result, or the error, is the same (A3).
- A constructor, a typed list literal, a tuple literal, `list` and `concat`
  build from agreeing parts, evaluated in the same order, a value that agrees.
  A constructor checks nothing when it runs
  ([Values and Constructors](../language/actions.md#values-and-constructors)).
  The range that a struct, list or tuple gets covers the ranges of its parts
  ([Value and Range Details](../language/actions.md#value-and-range-details)),
  which are the same in both. A conversion `N{x}` keeps the range and the
  content of a node, and does not accept an error value
  ([Named Node Types](../language/types.md#named-node-types)).
- A field access reads fields that agree, and reads `nil` from an error value
  in both ([Field Access](../language/actions.md#field-access)); its run-time
  errors, on `nil` and on a struct whose type lacks the field, depend on the
  kind and the type of the receiver. `.Start` and `.End` read a range, and
  `.Value` a value that agrees. A capture access reads captures that agree,
  or `nil` for an unmatched capture in both.
- Whether a value fits the Type of a cast depends on its kind, its type name
  and, through its value and elements, on theirs
  ([Casts](../language/actions.md#casts)): an error value fits only a Type that
  admits `error`. The run-time error of a cast names the type of the value,
  `error` for an error value
  ([Type Checking](../language/type-checking.md)), and the position where the
  rule started, which are the same in both.
- An index reads the element of a list or tuple of the same length in both,
  and the elements agree.
- The argument of `len` and `text` is a string or a node, possibly `nil`
  ([Type Checking](../language/type-checking.md)); an error value is neither,
  and a value of type `_` reaches them only through a cast, which an error
  value fails as above. `len` reads the length of a string, the range of a
  string node or the number of elements of a list or tuple, and `text` a
  string or the input of a range, the same in both
  ([Built-In Functions](../language/actions.md#built-in-functions)).
- `foldl`, `foldr`, `map` and `filter` visit lists of the same length, an
  error value as an empty list in both, and call their lambdas in the same
  order with arguments that agree; by the induction hypothesis the results
  agree, so `filter` keeps the same elements and the folds pass on
  accumulators that agree.

A predicate succeeds when its value is `true`, which is a scalar and the same
in both, and a run-time error makes it fail in both
([Types and Errors](../language/actions.md#types-and-errors)). $\square$

An error value can thus be stored, cast, compared with `nil` and passed on,
but no operation reads its message. A value of the type `missing`, whose
fields `expected` and `message` an action can read, is made only by the
insertion of a [`#missing`](../language/pragmas.md#missing) token, which is
outside the scope.

## Lemma 1 (Context)

In $\mathcal{R}$, let a call read no growth open at its start. Then its $s$,
its $q$, its run-time error, if any, and its examined range are functions of
its key $\kappa$, and its outcome is a function $F(\kappa, \lambda)$ of its key
and lookahead state. The two outcomes $F(\kappa, 0)$ and $F(\kappa, 1)$
**agree**: they differ at most in their farthest failures and in the errors that
`#recover` builds, so they have the same $s$ and $q$, recovered errors of the
same number and values that agree ([Agreeing Values](#agreeing-values)), the
same value when they have no recovered error.

*Proof.* By induction on the nesting of evaluations, which is finite
([Termination](../runtime/parsing.md#termination)). The evaluation of the body
of $A$ at $p$ depends on the following state and on nothing else:

- The input, the position $p$ and the level $\ell$.
- The variables. The body sees the variables of its callers, and of those it
  reads only the ones in $\eta$. Its own bindings end with the call
  ([Rule Calls](../language/parser-expressions.md#rule-calls)).
- Captures. A call has a scope of its own, so the captures of the caller are
  not visible ([Rule Calls](../language/parser-expressions.md#rule-calls)).
- Cuts. A cut never extends beyond its rule
  ([Cut](../language/parser-expressions.md#cut)), so the cut state of the
  caller does not enter.
- Actions. An action has no effect other than its value and its run-time
  error ([Memoization](../runtime/parsing.md#memoization)), and the
  evaluation is deterministic
  ([Evaluation](../language/parser-expressions.md#evaluation)).
- The calls it makes. By the induction hypothesis, a call it makes that reads
  no growth open at that call's start is a function of its own key and state.
  A call it makes that reads a growth open at that call's start reads either a
  growth that the evaluation of $A$ started, which is part of this evaluation
  and determined by it, or one that was open at the start of $A$, which the
  assumption excludes.
- The lookahead state. By (A1), $\lambda$ decides only whether failures are
  recorded. That changes $o$. It also changes the errors that `#recover`
  builds, because an error is made from the farthest failure recorded in the
  annotated expression: their positions, their lines and columns, their
  expected items, and the messages of the error values in $v$.

None of what $\lambda$ changes reaches a decision of the parser. Compare the
evaluations in the two states step by step. As long as they have taken the
same steps, the values that they have built agree: a recovery skips the same
input in both, since its skip expression starts at the same position, so its
error value has the same range, and only its message, made from the recorded
error, depends on $\lambda$ ([`#recover`](../language/pragmas.md#recover)); a
capture, a sequence or a repetition puts agreeing values together, and a call
gives a value that agrees, by the induction hypothesis. So an action or a
predicate reads captures that agree, and by
[Actions on agreeing values](#agreeing-values) it raises the same run-time
error in both, or builds values that agree, and a predicate decides alike. A
binding stores an `int`, a `string` or a `bool`, the same in both. No other
step reads a value, so the next step is the same in both. So $s$, $q$, the
run-time error and the examined range are the same for both states, each
`#recover` recovers in both or in neither, $\varepsilon$ has the same length,
and the values agree. When $\varepsilon$ is empty, no recovery took place on
the path that matched, so $v$ holds no error value and is the same. $\square$

The same argument shows that an evaluation in state $1$ whose parts have
outcomes that agree with other ones has an outcome that agrees with the one
that those give, and that an expression that cannot recover, which has no
`#recover` and calls, directly or through the rules it calls, no rule that has
one outside lookaheads and the `skip` of an Operators Rule, has an empty $\varepsilon$ and a value without error values,
so agreeing outcomes of it differ at most in their farthest failures.

## Lemma 2 (Reading an Open Growth)

If a call of $A$ at $p$ reads a growth of $L$ that was open at $p_L$ when the
call started, then $p = p_L$ and $A$ is a rule of the cycle of $L$.

*Proof.* The call started while the growth was open, so it was made within
the evaluation of $L$ at $p_L$, where no position is before $p_L$:
$p \ge p_L$. It reads the current result, so a call of $L$ at $p_L$ is made
within it, where no position is before $p$: $p \le p_L$. So $p = p_L$, and
both chains of calls, from $L$ to $A$ and from $A$ to $L$, are made at $p$
without consuming input. Every call on them is a call that may run before
consuming input, which the static analysis of
[Left Recursion](../language/parser-expressions.md#left-recursion) includes,
so $A$ and $L$ are on one left-recursion cycle. $\square$

## Sound Entries

An entry $(\kappa, \lambda) \mapsto (\omega, \rho)$ is **sound** when
$\omega = F(\kappa, 0)$ if $\lambda = 0$, $\omega$ agrees with $F(\kappa, 1)$
if $\lambda = 1$, and $\rho$ contains the examined range that Lemma 1 gives
for $\kappa$.

## Lemma 3 (Answering From a Sound Entry)

A call of key $\kappa$ in state $\lambda'$ that the conditions allow to be
answered from a sound entry reads no growth open at its start, and recording
the entry gives the outcome that the call has in $\mathcal{R}$ if
$\lambda' = 0$, and one that agrees with it if $\lambda' = 1$, and a range
that contains the range of the call in $\mathcal{R}$.

*Proof.* Suppose that the call reads a growth open at its start. By Lemma 2
it is a call, at the position $p$ of an open growth, of a rule of its cycle, so
by (C) its present set $P'$ is a subset of the present set $P$ of the call
that stored the entry, which read no growth open at its start. Compare the
two evaluations in $\mathcal{R}$ from their start: they take the same steps
until one of them is answered with a current result. A growth that started
within one started within the other too, so the first difference is a call of
a leader $L$ at $p$ answered with the current result of a growth open at the
start, which by Lemma 2 is on the cycle of the rule, so $L \in P'$. Since
$L \in P$, the corresponding call in the evaluation that
stored the entry is answered with a current result too, which it never is. So
the call reads no growth open at its start, and by Lemma 1 its outcome in
$\mathcal{R}$ is $F(\kappa, \lambda')$, with the range that Lemma 1 gives for
$\kappa$, which $\rho$ contains.

If $\lambda' = \lambda$, the outcome is $\omega$ if $\lambda = 0$, and
agrees with $\omega$ if $\lambda = 1$. Otherwise $\lambda = 0$ and
$\lambda' = 1$ by (L), and by Lemma 1 $\omega = F(\kappa, 0)$ agrees with
$F(\kappa, 1)$. $\square$

## Lemma 4 (Answering From a Dependent Result)

Let a call $x$ in $\mathcal{R}$, of key $\kappa$ in state $\lambda$ at $p$
in a context $\Gamma_1$ with the present set $P$, have the outcome $\omega$
and the range $\rho$, and let a call $y$ of the same key in a state
$\lambda' \ge \lambda$, in a context $\Gamma_2$ with the present set $P$, find
each growth that $x$ read open with a current result that **matches** the one
that $x$ read, and each lookahead at $p$ that $x$ observed with an observation
that matches the one that $x$ made. Apart from their examined ranges, a
current result matches another when it is equal to it or, if $\lambda' = 1$,
agrees with it, and an observation matches another when the expressions of
both lookaheads matched or both failed. Then $y$ has the outcome $\omega$ if
$\lambda' = 0$, and one that agrees with $\omega$ if $\lambda' = 1$, and a
range contained in the hull of $\rho$ and of the ranges of the current results
and observed lookaheads that $y$ reads; that hull is $\rho$ itself when those
ranges are the ones that $x$ read. Outside the lookaheads that $x$ observed,
the two evaluations make the same calls at the same positions and take the
same decisions.

*Proof.* Compare the evaluations of $x$ and $y$ from their start. They take
the same steps as long as each step reads the same state, or one that differs
only in what no decision reads, and, as in Lemma 1, the state that a step reads
differs between them only in the growths open at $p$ at their start and in the
lookahead state.

The lookahead state differs only when $\lambda = 0$ and $\lambda' = 1$, and
then, as in Lemma 1, it changes only the farthest failures that the steps
record and the messages of the error values that recoveries build. The values
that the two evaluations build therefore agree, step by step, and by
[Actions on agreeing values](#agreeing-values) no action or predicate decides
differently or raises a different run-time error, as in the proof of Lemma 1.
A step inside a lookahead within $x$ is in state $1$ in both.

A call of a leader at $p$ is answered with a current result in both or in
neither: only a growth of the cycle of the rule can answer it (Lemma 2), and
the present sets are equal; when it is, the results match, since $x$ read it,
so they have the same $s$ and $q$ and values that agree. A
lookahead at $p$ that reads such a growth and that
$x$ observed matched in both or failed in both, and nothing else of it
remains after it: by (A1) its expression records no expected item and no
farthest failure; a failing expression and a negative lookahead have no other
effect; and an observed positive lookahead leaves no capture or binding, those
of the rules that it calls ending with them, and its end discards the errors
recovered in it ([Lookahead](../language/parser-expressions.md#lookahead)). A lookahead that $x$ did not observe is evaluated as part of $x$, and
the growths that it reads are among those that $x$ read. A call at
$p$ within $x$ that grows a leader of the cycle grows it in both, from the same
failing seed; its steps read the growths open at the start of $x$ only in the
ways above, so by induction on the steps they have outcomes that match, equal
when $\lambda' = 0$, and stop at the same step, and so does the growth. A call
at a later position reads no growth open at the start of $x$ (Lemma 2), so by
Lemma 1 its outcomes in the two evaluations are equal when the states are, and
agree otherwise.

So the two evaluations take the same decisions, with the same steps, and
their outcomes differ at most where the current results that they read and the
lookahead states differ: not at all when $\lambda' = 0$, and only in what
agreement allows when $\lambda' = 1$. They examine the same input outside the
current results and observed lookaheads that they read, and a caller takes the
hull with the range of each outcome it records, so the range of $y$ is
contained in the hull of $\rho$ and of the ranges of those that $y$ reads.
$\square$

## Theorem (Memoization Never Changes the Result)

Let $\mathcal{M}$ start with a table of sound entries, empty for an ordinary
parse. Then the parses $\mathcal{M}$ and $\mathcal{R}$ return the same value,
the same recovered errors and the same syntax error, or abort with the same
run-time error.

*Proof.* Order the evaluations of $\mathcal{M}$, of expressions and of calls,
by their completion, so that an evaluation comes after every evaluation nested
in it. By induction on this order, each evaluation $x$ satisfies the
following invariant when it completes, provided that every evaluation that
completed before it satisfies it:

- **(I1)** The outcome of $x$ in $\mathcal{M}$ is its outcome in $\mathcal{R}$
  when evaluated in the same state: the same position, variables, captures,
  lookahead state, open growths and current results. In lookahead state $1$
  the two outcomes agree (Lemma 1), and so do the captures and current results
  of the state.
- **(I2)** The examined range of $x$ in $\mathcal{M}$ contains its examined
  range in $\mathcal{R}$.
- **(I3)** Every entry in the table is sound.

At the start, (I3) holds by assumption. Let $x$ complete; the evaluations
nested in it completed before it and satisfy the invariant. $x$ is one of the
following.

1. *A terminal, an anchor, a predicate or an action.* These read the same state
   in both parses, so they have the same outcome and examine the same input.
   The values they read are equal by (I1), though they may be different nodes,
   and by (A3) a comparison depends only on scalar values and on whether a
   value is `nil`. In state $1$ the values they read agree, and by
   [Actions on agreeing values](#agreeing-values) they decide alike, raise the
   same run-time error and build values that agree.
2. *A combinator.* A sequence, choice, repetition, option, lookahead, cut,
   capture or `#recover` combines the outcomes of its parts by the rules of
   [Rule Expressions](../language/parser-expressions.md) and records them in
   order. Its parts satisfy (I1) and (I2), so it does too: in state $1$ by
   Lemma 1, since no decision reads an error that `#recover` built
   ([Agreeing Values](#agreeing-values)). A
   lookahead or the `skip` of an Operators Rule in state $0$, whose
   expression is in state $1$, has an outcome equal in both parses. Its
   expression matches in both or in neither and records no farthest failure
   (A1). The errors recovered in it are discarded at its end
   ([Lookahead](../language/parser-expressions.md#lookahead),
   [`skip`](../language/operators-rule.md#skip)). A variable holds an `int`, a
   `string` or a `bool`, never an error value
   ([Variables](../language/predicates.md#variables)). A negative lookahead and
   a `skip` leave no capture, and a capture inside a positive lookahead
   captures an expression that cannot recover, so its value is equal in both.
3. *A call answered with the current result of an open growth.* Both parses
   answer it so. The current result is the outcome of a completed body
   evaluation of the growth (item 5), or the failing seed, so it is the same in
   both parses, or agrees in state $1$.
4. *A call answered from an entry.* By (I3) the entry is sound, so by Lemma 3
   (I1) and (I2) hold for $x$. A call answered from a result that (D) keeps,
   in its state or in a smaller one, was answered under the conditions of
   Lemma 4, and that result was the outcome and range of an evaluation $x'$ of
   $\mathcal{M}$ that completed before, which satisfies (I1) and (I2). The
   current results and observations that $x$ and $x'$ read are equal in
   $\mathcal{M}$, and by (I1) their outcomes in $\mathcal{R}$ are those in
   $\mathcal{M}$ in state $0$ and agree with them in state $1$, so they match
   in $\mathcal{R}$ as they do in $\mathcal{M}$. By Lemma 4 the outcome of $x$
   in $\mathcal{R}$ is then that of $x'$ in $\mathcal{R}$ if $x$ is in state
   $0$, and agrees with it if $x$ is in state $1$, and agreement is an
   equivalence, so (I1) holds for $x$. By Lemma 4 the range of $x$ in
   $\mathcal{R}$ is contained in the hull of the range of $x'$ in
   $\mathcal{R}$ and of the ranges in $\mathcal{R}$ of the current results and
   observed lookaheads that $x$ reads. Each of these is contained in the range
   of $x'$ in $\mathcal{M}$, which $\mathcal{M}$ gives $x$: the first by (I2)
   for $x'$, and the others by (I2) for the evaluations that gave them in
   $\mathcal{M}$, whose ranges are those of the current results and
   observations that $x'$ read, which the range of $x'$ contains, since a
   caller takes the hull with each range that it records. So (I2) holds for
   $x$. A step of a growth answered in the same way has by Lemma 4 the outcome
   that evaluating it gives, or one that agrees with it in state $1$, and by
   the same argument a range that its range in $\mathcal{M}$ contains. In a
   run of steps of a growth of $L$ taken at once, from current
   results $r_0, \dots, r_{k-1}$ to $r_1, \dots, r_k$, step $i$ read only the
   growth of $L$, with the current result $r_{i-1}$, and observed lookaheads.
   The run is taken from a current result equal to $r_0$, and each of those
   lookaheads has now the observation that its step recorded, obtained without
   reading a current result of $L$. An evaluation that reads no current result
   of $L$ is the same whatever that result is, so each lookahead also has that
   observation while the current result is $r_{i-1}$. By induction on $i$,
   Lemma 4 gives step $i$ the outcome $r_i$, or one that agrees with it in
   state $1$, and, by the argument for a call, a range in $\mathcal{R}$ that
   the range of $r_i$ in $\mathcal{M}$ contains, since that range contains the
   range of $r_{i-1}$ and of the observations of step $i$. So the run has the
   outcomes that evaluating its steps gives, and (I2) holds for it.
5. *A call that evaluates the body of a rule, or grows a leader.* A body
   evaluation is nested in $x$ and satisfies (I1) and (I2). A growth evaluates
   the body with the current result fixed, compares the end of its outcome with
   that of the current result, and either replaces the current result and
   evaluates the body again or stops. Both parses start from the same failing
   seed, so by induction on the steps of the growth they replace the current
   result with the same outcomes and stop at the same step.

If $x$ is a call whose outcome $\mathcal{M}$ stores, then by (S) $x$ read no
growth open at its start. By Lemma 1 its outcome in $\mathcal{R}$ is
$F(\kappa, \lambda)$, by (I1) and (I2) the stored entry is sound, and (I3)
holds after storing it.

The parse is the call of the start rule at $0$ with no open growth, followed for
a whole-input parse by the check for the end of the input
([Whole Input](../runtime/parsing.md#whole-input)). By (I1) it has the same
outcome in both parses, so it gives the same value, the same recovered errors,
and the same farthest failure with its expected items, from which the syntax
error is made ([Syntax Errors](../runtime/parsing.md#syntax-errors)).

A run-time error is an outcome like the others. An entry holds only the outcome
of a call that completed without one, since the parse aborts otherwise, and by
Lemma 3 the call that it answers has the same outcome in $\mathcal{R}$, so
evaluating that call raises no run-time error there either. So the first
run-time error that either parse meets is met by the other at the evaluation
of the same expression in the same state. $\square$

## Consequences

**Values.** The two parses return equal values: the same nodes with the same
kinds, ranges, children and fields. Whether two calls return the same node
is not part of the outcome, and no action can observe it, by (A3)
([Memoization](../runtime/parsing.md#memoization)).

**Leaders and members.** A result of a rule of a cycle that read the current
result of a growth open at its start is not stored in the table by (S), and
what (D) keeps of it is dropped with the first growth at its position. This
holds for another leader of the cycle too: a later call of it at that position
grows it again. The result of a call therefore never depends on which calls
the parse evaluated before it.

**Variables.** A call is answered from an entry only with an equal $\eta$, so a
rule that reads variables is memoized per combination of their values. The
exception of Left Recursion, where every call of a growing leader at its
position reads the current result whatever $\eta$ is, holds in both parses
alike.

**Order of storing.** The theorem holds for every choice of which outcomes to
store, so deferring memoization to the second call at a position, or not
memoizing a rule that is called from one place, leaves the result unchanged.
