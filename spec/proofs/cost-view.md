# Cost View

Arata Furukawa · Last revised 2026-11-08

The classifier decides the [Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)
on the [cost view](grammar-laws.md#normalization) of the rule bodies, and the
[Linear Cost](linear-cost.md) proof bounds a parse of that view. This proof
transfers the bound to the grammar as written: on every input, the parse of the
written grammar costs at most a constant times the parse of its view, in the
units of [Cost](../runtime/parsing.md#cost) and in the compute units of each
backend (the theorem). It covers every rewrite that the view applies, N1 to N6
and N8 to N12 of [Normalization](grammar-laws.md#normalization); N7 and N13 are
off for the classifier.

Lemma 1 shows that no predicate observes a value that a rewrite changes, so
every rewrite keeps what the grammar matches; the rewrites that change a value
are listed under [Rewrites That Change Values](#rewrites-that-change-values),
each with the condition under which the view applies it. Lemma 2 bounds an evaluation that
the written grammar makes again, Lemma 3 compares one rewrite at a time, Lemma 4
shows that the written grammar calls no rule in a state in which the view does
not, and the theorem adds them up with the memo. The values that actions build
from a value that a rewrite reshapes, and the run-time errors of those actions,
are not covered: [Scope](#scope) states what is left open.

## Setting

Fix a grammar $G$ as the classifier reads it: inline actions lowered to hidden
rules and attributes normalized, as `prepareClassifier` in `internal/compile`
does. Its **view** $V$ is what `normalizeRules` in
`internal/compile/normal_view.go` gives with the options `costView`. $V$ has the
rules of $G$ with other bodies; the memoized rules, the left-recursion cycles and
their leaders, the variables and the run sites are those of $G$
(`newViewClassifier`). The **parse of $V$** is the parse with the rule bodies of
$V$ and those decisions of $G$, as the [Setting](linear-cost.md#setting) of the
Linear Cost proof reads it, and with the capture scopes of $G$: a name that a
predicate or an action of $V$ reads denotes the capture of that name in $G$.
States, outcomes and the facts (F1) to (F5) are those of
[Grammar Laws](grammar-laws.md#setting), and the memo is that of
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules).

**Rewrites.** `normalizeRules` rewrites each body from the inside out until no
rule applies (`fix`). Each change replaces one expression $e$ of a body by an
expression $e'$, under conditions that it checks on $e$ as the earlier changes
left it. So $V$ is reached from $G = G_0$ by a sequence
$G_0, G_1, \ldots, G_m = V$ in which $G_{i+1}$ differs from $G_i$ only in one
occurrence of one body, and $m$ depends only on $G$, since `fix` makes at most 64
passes over each body and each pass a bounded number of changes. A rewrite only
removes or regroups parts of $e$: no rewrite adds a call, a predicate, an
action, a recovery, a stream, an Operators Rule or a `#missing`, and none copies
a part, so the parts that $e$ and $e'$ share are the same expressions in $G_i$ and
$G_{i+1}$. No rewrite adds or removes a discard, a lookahead or an `#atomic`
around a call, the constructs that make a call one of the value-free twin of
its rule (`void` in `internal/compile/bcompile.go`): K1 and K2 replace a
lookahead by a lookahead. So every call that the two share calls the same
variant of its rule.

**Cheap expressions.** `cheapRules` computes the **cheap** rules of $G$: the least
set that holds every memoized rule that is on no left-recursion cycle, and every
rule on no such cycle, without an action, whose body is evaluated again
cheaply. An expression is **evaluated again cheaply** (`evaluatedAgainCheaply`)
when every rule that it calls is cheap and called without a level, and it
contains no predicate, inline action, recovery, stream, Operators Rule or
`#missing`. Since the set is the least one, a cheap rule that is not memoized
calls only cheap rules, and the calls of cheap rules that are not memoized form
no cycle: they nest at most $|\text{rules}|$ deep. The bodies of the $G_i$ keep
the property, since no rewrite adds what it excludes.

**Own cost.** A **keyed evaluation** is a body evaluation of a memoized rule that
is neither a growth step nor in a context of growths: the evaluations that
Lemma 3 of the [Linear Cost](linear-cost.md#lemma-3-body-evaluations) proof
counts. Its **key** is its variant, position, binding level and the values of
the variables that the rule reads. The lookahead state is left out: it decides
only whether failures are recorded and what the errors of `#recover` say ((A1)
of [Memoization](memoization.md#assumptions)), which $C$ does not count.
A **growth run** is the growth of a leader that a call outside every context of
growths starts: its growth steps and the evaluations in the context of that
growth. Its key is the key of the call. The **own cost** of an evaluation of an
expression is the part of $C$ that it makes outside the keyed evaluations and growth runs that it
starts: its steps, the input that its terminals examine, its iterations and
operators, the values that its actions and predicates build, and each call that
it makes, which $C$ counts as a constant and the lookup of its key whether or not
the memo answers it. The own cost of a call includes the evaluation of the body of
a rule that is not memoized, since that body is evaluated at every call. The own
cost of a growth run is the sum of the own costs of its steps and of its
evaluations in context. A body evaluation costs at least 1.

## Rewrites That Change Values

By [Values](../language/parser-expressions.md#values-and-the-concrete-syntax-tree),
a sequence is a tuple of the values of its items that have one, even when there
is only one; a parenthesized sequence in a sequence adds its elements to it; a
group creates no value; and a choice has the value of the alternative that
matched, or `nil` when that alternative has none. Measured by these rules, a
rewrite of the view changes the value of the expression that it replaces only
in the cases of the table. `depth` is positive inside a capture that a predicate
of the rule reads and in the whole body of a rule whose value reaches such a
capture, and `observed` holds of an expression that contains such a capture or
calls such a rule in a position that keeps its value (Lemma 1). The lines are
those of `internal/compile/normal_view.go`.

| Rewrite | How the value changes | Condition as the code checks it |
|:--|:--|:--|
| N1: a sequence of one item becomes the item (`seq` 303) | the tuple of one element becomes the value of the item | `depth` zero and the item not `observed`; or the item is a bottom, which never matches |
| N6: a sequence left with one item, or none, once its cuts are dropped becomes the item, or a top (`dropCuts` 732, `seqOf` 699) | as in N1; a tuple of no elements becomes no value | `depth` zero and the alternative whose cuts are dropped not `observed`; otherwise it stays a sequence of the item, or of a top, which keeps the tuple |
| A choice left with one alternative becomes that alternative (`choice` 515), after N2, N3, N4, N6 or N8 dropped or merged the others | an alternative that is a sequence adds its elements to a sequence around the choice instead of being one element; one without a value loses the `nil` of the choice | `depth` zero and the alternative not `observed`; or the alternative is a literal, a class, `.`, a call, a repetition, an optional, an `#atomic` or a bottom (`sameValueBare` 530), whose value alone is its value as the only alternative |
| N9, N10, N11 | one list replaces a tuple that holds a list, or a list of lists | `depth` zero and no item `observed` (`adjacent` 327, `nestedRepeat` 405, `unroll` 364) |
| The left factoring of N12 | the rests of the alternatives become one element after the shared prefix | `depth` zero and no alternative of the run `observed` (`factor` 636) |
| N13 | the body of the rule takes the place of the call | off in the cost view; in the view of matching, only where `depth` is zero (`rewrite` 253) |

The other rewrites keep the value. N1 removes a group, which creates no value,
splices a sequence into a sequence, which adds the same elements, and flattens a
choice into a choice, which has the value of the alternative that matched
either way. N2, N3 and N4 drop alternatives that never give the value of the
choice: one after an alternative that always succeeds or begins with a cut is
never tried, one that an earlier alternative subsumes fails wherever it is
tried, and a bottom never matches; the alternatives left are unchanged (for
the captures in the dropped ones, see Lemma 1). N4 replaces `_|_ a` by `_|_`,
and K4 of N12 replaces `!X X` by `_|_`: neither matches. N5 changes only
lookaheads, which have no value, and K1 only where the expression binds nothing
outside its calls. N6 drops a cut, which has no value, so a sequence that keeps
two items or more is the same tuple, and V1 drops a recovery that never runs its
skip. N8 replaces literals of one code point by a class, and both give a string
node of the matched text.

A value is **reshaped** when it is the value of an expression that a rewrite of
the table replaces where it changes that value (every case of the table but a
bottom in the first row and the alternatives of `sameValueBare` in the third),
or when it includes such a value: the value of a capture around it, of a
sequence or choice around it, of a rule whose body value includes it, and so on
outward. By Lemma 1 no predicate reads a reshaped value; an action can.

## Scope

The parses are those that the classes of
[Classification](../runtime/parsing.md#classification) promise: without
missing-token insertion, which is outside the classes, and stopped by no limit.
A parse of a [Document](../runtime/documents.md) and a stream are covered at the
end of the theorem. The cost is that of the [Linear Cost](linear-cost.md)
proof: the walks of run sites are counted without their records, as that proof
counts them, and the records of scan sites add at most two steps for each walk
(Lemma 5 of [Run Sites](run-sites.md#lemma-5-cost-of-scan-records)).

**What is not covered.** An action of $G$ that reads a reshaped value, through a
capture or the value of a rule, reads another shape than the classifier sees in
the view: `x:("a"* "a"*) => concat($x[0], …)` copies a list in $G$ and an element
in the view, and `len($x)` of `x:("abc" #cut / "z")` is one element in $G$ and
three characters in the view. It can build values of other sizes, and stop the
parse with a run-time error where the view does not, or the reverse. The theorem
therefore bounds $C'_G$, the cost $C_G$ less the values that such actions build,
on inputs on which no action that reads a reshaped value stops either parse with
a run-time error. Step 8 of the [open steps](linear-cost.md#open-steps) states
what remains: prove that the values which actions build from a value that a
rewrite of [Rewrites That Change Values](#rewrites-that-change-values) reshapes
(a sequence of one item or a choice of one alternative replaced by its item
under N1, N6 or the drop of the other alternatives, N9, N10, N11, or the left
factoring of N12), and the run-time errors of those actions, stay within a
constant factor of what the classifier bounds on the view (or make the view keep
the shape of every value that an action reads). Predicates are covered: by
Lemma 1 none reads a reshaped value.

## Lemma 1 (Context)

Let $G_{i+1}$ replace $e$ by $e'$. Then every predicate of $G_i$ reads, in every
evaluation, the value that it reads in $G_{i+1}$ in the same state. In
particular, when the rewrite changes the value of $e$
([Rewrites That Change Values](#rewrites-that-change-values)), no predicate of
$G_i$ observes a value of $e$ or a value that includes one.

*Proof.* A predicate observes a value only by naming a capture: `$x`, the capture
access `$x.$y` into the node of a rule, or the elements of `$xs` through the
parameter of a lambda. A variable is set only by a binding of a predicate, so a
variable that holds a captured value was set by a predicate that names the
capture. Positional references are not part of the language
([Grammar Laws](grammar-laws.md#normalization), the context condition of the
cost view). Let a predicate of the rule $r_0$ name the capture $c$ of $r_0$, and
let the value of $c$ include a value of $e$, which lies in the body of the rule
$r$. A value passes outward through the value of every expression around it,
except a discard, an `#atomic`, a negative lookahead, and a positive lookahead
outside the captures in it, which drop it; a capture in a positive lookahead
keeps its value beside the tuple of its scope
([Capture](../language/parser-expressions.md#capture)). Call the other positions
value positions. So either $r = r_0$ and
$c$ lies around $e$ or inside $e$, or $c$ lies around a call, in a value position,
of a rule $r_1$ whose body calls in a value position $r_2$, and so on to $r$.
`valueReaders` (`normal_view.go`) collects in `reads` the names of the captures
that the predicates of each rule name, and marks `valued` every rule that
`valueCalls` finds inside such a capture, and transitively inside the bodies of
the marked rules. `valueCalls` drops only the calls under a discard, a negative
lookahead, and a positive lookahead outside its captures, so it finds every call
in a value position, and the calls under an `#atomic` besides. In
the first case, $c$ around $e$ makes `depth` positive while the view rewrites
inside $c$ (the `Capture` case of `rewrite`, line 219), and $c$ inside $e$
makes `observed` true for $e$. In the second, $r$ is `valued`, so `depth` is
positive in the whole body of $r$ (`normalizeRules`, line 121).

Each rewrite of the table of
[Rewrites That Change Values](#rewrites-that-change-values) changes a value only
where `depth` is zero and `observed` is false. For N9, N10, N11 and the left
factoring, `observed` is checked on the expressions that $e$ is made of: both
items (`adjacent`), the repetition (`nestedRepeat`), every alternative of the
run (`factor`). So neither case holds, and these rewrites change no value that a
predicate observes. In the first three rows, $e$ is a sequence or a choice of
one expression $x$, besides the cuts that N6 drops, and $e'$ is $x$ itself (or
a top): the captures of $e$ are those of $x$, which keeps them and their values.
Only the value of $e$ itself changes, and a predicate observes it only through a
capture around $e$ or through the value of a rule whose body contains $e$. A
capture around $e$ makes `depth` positive where the view rewrites $e$, except a
capture inside the alternative from which N6 drops the cuts, which
`looseCutsDropped` enters without changing `depth`; that capture makes
`observed` true for the alternative, which is what `dropCuts` checks (line
736). A rule whose body contains $e$ and whose value a predicate reads is
`valued`. In each case the view keeps the sequence of $x$ or the choice of $x$,
unless $x$ has the same value alone: a bottom, or for a choice an expression
that `sameValueBare` accepts. So no rewrite changes a value that a predicate
observes. This is the context condition of
[Normalization](grammar-laws.md#normalization).

The other rewrites keep the value of $e$
([Rewrites That Change Values](#rewrites-that-change-values)), except that N2,
N3 and N4 can drop
the only occurrence of a capture: in an alternative after one that always
succeeds or begins with a cut, which is never tried; in an alternative that an
earlier one subsumes, which fails wherever it is tried; or after `_|_` in a
sequence, which is never evaluated, or before it, in a sequence that fails. In
each case the capture is never set in $G_i$, since a capture made in a match
that backtracking undoes does not survive and one that did not match is `nil`
([Capture](../language/parser-expressions.md#capture)). Under the capture scopes
of $G$, a read of it gives `nil` in both. $\square$

With Lemma 1, each rewrite preserves $\equiv_E$ in its context
([Equivalences](grammar-laws.md#equivalences)), so its law applies inside every
body.

## Lemma 2 (Evaluating Again)

Let $x$ be evaluated again cheaply, and let $G_i$ evaluate $x$ at a state
$\sigma$ and later, within the same evaluation of the enclosing rule body, at
$\sigma$ again. Then the second evaluation has the outcome of the first, makes
its calls with keys of calls of the first, starts no growth run and no keyed
evaluation of a rule of a left-recursion cycle, and has an own cost of at most the
own cost of the first. Neither evaluation evaluates the occurrence of a body
that contains $x$ outside the keyed evaluations that it starts.

*Proof.* The outcome is that of the first by (F1) and (F4). Both start in
$\sigma$, so by (F1) they make the same steps in the same states, except inside
the calls that the memo answers in one and not in the other, and a call that the
memo answers ends as its evaluation would (the theorem of
[Memoization](memoization.md)). A rule that $x$ calls is cheap, so it is on no
left-recursion cycle, and it is not a leader. A cheap rule that is memoized is in
no context of growths, since only the growths of a rule's own cycle form its
context ([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)), so its
body evaluation is a keyed evaluation. A cheap rule that is not memoized is
evaluated at each call, and its body is evaluated again cheaply, so by induction
on the depth of these calls, which the least fixed point bounds, its two
evaluations make the same steps outside the keyed evaluations that they start.
So outside the keyed evaluations the two evaluations examine the same input,
make the same iterations and the same calls with the same keys, and build no
value: $x$ and the bodies of the cheap rules that are not memoized have no
predicate, inline action or rule action. Each call costs the same in both,
whether the memo answers it or not.

Let $x$ lie in the body of the rule $r$. If an evaluation of $x$ evaluated the
body of $r$ outside the keyed evaluations that it starts, a chain of calls of
cheap rules that are not memoized would lead from $x$ to $r$; then $r$ is cheap and
not memoized, and the chain closes a cycle of such rules, which the least fixed
point excludes. $\square$

The memo can answer in the second evaluation a call that it did not answer in
the first, which only removes a keyed evaluation. An entry that answered a call of
the first answers it in the second, since nothing between them removes it: the
growths of $\sigma$ are still in progress, its lookahead state is that of
$\sigma$, and the rules called are on no cycle, so their entries read no growing
result. A [stream](../runtime/streaming.md) discards entries only before the
elements that it has delivered, and after delivering an element it never
evaluates anything that starts before the element's end. If it delivers one
between the two evaluations, the second starts at the position of $\sigma$, so the
element ends at or before it, and every call of the first is at a position at or
after it: no entry of these calls is discarded. In a
[Document](../runtime/documents.md) parse, an entry stored in the parse has no
pending edit.

## Lemma 3 (One Rewrite)

Let $G_{i+1}$ replace $e$ by $e'$ by the rewrite $\rho$. For every state $\sigma$
in which $G_i$ evaluates $e$, the evaluation of $e'$ at $\sigma$ in $G_{i+1}$ has
the same match or failure, end, environment and captures after it and escape of
cuts, every call that $e$ makes there has the key of a call that $e'$ makes
there, and the evaluations that $e$ makes there and $e'$ does not are
evaluations again of expressions evaluated again cheaply, of the kind of Lemma 2,
or make no call. Each evaluation that $e$ makes and $e'$ makes too has the same
own cost in both, up to the occurrence of $e$ itself in the bodies that it
evaluates, and the rest of $\mathrm{own}_{G_i}(e, \sigma)$ is at most
$(a_\rho - 1) \cdot \mathrm{own}_{G_{i+1}}(e', \sigma) + b_\rho$, with the
constants of the table, where $k$ is the number of alternatives that N8 merges,
$j$ the number of alternatives that N12 factors, $\ell$ the length of the literal
that N3 drops, and $d$ the most input that the expression of K4 can examine plus
the iterations of its bounded repetitions. The lines are those of
`internal/compile/normal_view.go`.

| Rewrite | Code | Condition as the code checks it | $a_\rho$ | $b_\rho$ |
|:--|:--|:--|--:|--:|
| N1 splice a group or a sequence in a sequence, flatten a choice, unwrap a sequence of one item; replace a choice of one alternative by it | `rewrite` (Group) 189, `seq` 278, 303, `choice` 446, 515 | a choice that is not the last alternative only without a loose cut in its alternatives; the unwrapping and the replacement as in [Rewrites That Change Values](#rewrites-that-change-values) | 1 | 0 |
| N2 drop the alternatives after one that always succeeds or begins with a cut | `choice` 473 | `AlwaysSucceeds`, `leadingCut` | 1 | 0 |
| N3 drop an alternative that an earlier one equals or prefixes | `choice` 465, `subsumed` 541 | the earlier one is evaluated again cheaply; a literal prefix only between literals matched with case | 2 | $\ell$ for a literal prefix, 0 otherwise |
| N4 drop a bottom alternative, cut a sequence after a bottom | `choice` 459, `seq` 297 | a bottom without `@error` (the attribute keeps it an `Attributed`) | 1 | 0 |
| N5 stacked lookaheads (K1, K2) | `lookahead` 840 | K1 only when the expression binds nothing outside its calls | 1 | 0 |
| N6 cuts that cannot commit, recoveries that never fire (T2, V1) | `choice` 484, 492, `dropCuts` 732, `attributed` 879 | a cut that ends an alternative, any loose cut of the last alternative; a base that always succeeds; the sequence that is left as in [Rewrites That Change Values](#rewrites-that-change-values) | 1 | 0 |
| N8 merge single characters (C9) | `mergeChars` 576, `singleChars` 608 | classes without negation or U+FFFD, literals of one code point with case | $k$ | 0 |
| N9 `X* X*`, `X+ X*` to the first; `X X*` to `X+`; `X? X*` to `X*` | `adjacent` 327-346 | `depth` zero, neither item `observed`, `idempotent(X)`; for `X? X*` also X not nullable | 2 | 1 |
| N10 `(X*)*`, `(X+)*`, `(X*)+` and `(X?)*` to `X*` | `nestedRepeat` 405 | unbounded repetitions, `depth` zero, not `observed`, `idempotent(X)`; for `(X?)*` X not nullable | 2 | 2 |
| N11 `X* (Y X*)*` and `X* (X / Y)*` to `(X / Y)*` (R7) | `adjacent` 327, 347, `unroll` 364 | `depth` zero, neither item `observed`, X not nullable and without a loose cut, `idempotent(X)`; for the unrolling also Y without a loose cut and binding nothing outside its calls | 2 | 2 |
| N12 `!X X` to `_|_` (K4) | `adjacent` 353, `boundedTerminal` 1116 | X made of literals, classes, `.`, sequences, choices, groups and bounded repetitions, without calls | 1 | $2d$ |
| N12 left factoring (C6) | `factor` 636 | `depth` zero, no alternative of the run `observed`, at least two alternatives without a loose cut, factored by their whole shared prefix, which is evaluated again cheaply | $j$ | 0 |

`idempotent(X)` (1067) is `evaluatedAgainCheaply(X)`, no loose cut in X, and X
not nullable or binding nothing outside its calls.

*Proof.* The matching part is the law of the rewrite
([Grammar Laws](grammar-laws.md)) under the condition that the code checks, which
the table shows to imply the condition of the law, in a context that Lemma 1
allows. The parts of $e$ that $e'$ keeps are the same expressions, evaluated in
$G_{i+1}$ in the states in which $G_i$ evaluates them, by the law; so an
evaluation that both make has the same own cost in both, but for the occurrence
of $e$ in the bodies of the rules that are not memoized that it calls, which the
next part bounds. What remains is what $G_i$ evaluates that $G_{i+1}$ does not,
case by case.

- *N1, N2, N4, N5, N6.* $G_i$ makes no evaluation that $G_{i+1}$ does not. N1
  only regroups: a group, a sequence of one item, a choice of one alternative and
  the nesting of sequences and choices are no step of their own. N2: no alternative after one that always
  succeeds is tried, and an alternative that begins with a cut commits the
  choice before it examines anything. N4: `_|_` fails at once and examines
  nothing, and nothing after it in a sequence is evaluated. N5: the inner
  lookahead evaluates the same expression in the same state, and a lookahead
  costs nothing of its own beyond what its expression examines
  ([Linear Cost](linear-cost.md#setting), *Input that a lookahead examines*).
  N6: a cut that the view drops commits nothing that is still to be tried, so
  both evaluate the same, and the sequence of one item or of a top that the view
  keeps in its place is no step of its own; a recovery whose base always succeeds never evaluates
  its skip.
- *N3.* $G_i$ evaluates the dropped alternative $a$ only after the earlier
  alternative $p$ that subsumes it failed without a cut escaping, in the state
  in which $p$ started (F2). If $a$ equals $p$, or $p$ is a proper prefix of
  the items of $a$, $G_i$ evaluates the items of $p$ again in the states in which
  $p$ evaluated them, and they fail as before (F4); by Lemma 2 this costs at most
  the own cost of $p$, which $G_{i+1}$ also evaluates, and no item after them is
  evaluated. If $p$ is a literal $l$ that prefixes the literal $a$, the input
  does not begin with $l$, so $a$ fails after examining at most $\ell$
  characters. Each dropped alternative is one rewrite, which gives $a_\rho = 2$,
  with $b_\rho = \ell$ for a literal prefix and $b_\rho = 0$ for the others,
  where $\ell$ is not defined.
- *N8.* Each of the $k$ alternatives examines at most the one character at the
  position, and the class examines it once; at the end of the input none
  examines any.
- *N9.* `X* X*` and `X+ X*`: the first repetition stops at $q$ because X failed
  there, which leaves the state as it was (F2), or matched without consuming
  input, and then X binds nothing outside its calls (`idempotent`); the second
  evaluates X at $q$ in that state, so by Lemma 2 at most the own cost of the
  evaluation of X at $q$ that the view makes, plus one failed iteration.
  `X X*` and `X+` make the same evaluations. `X? X*`: when X fails at the start,
  `X*` evaluates it again there, as above; when it matches, the two make the
  same evaluations.
- *N10.* `(X*)*`, `(X+)*`, `(X*)+`: after the inner repetition stops at $q$,
  the outer one starts it again at $q$, which evaluates X at $q$ again, as in N9,
  and stops; that adds one evaluation of X, bounded by Lemma 2, and at most two
  iterations. `(X?)*` evaluates X where `X*` does, with one more iteration at the
  end that matches nothing.
- *N11.* At each position both evaluate X first and Y after X fails, in the same
  states (R7). The unrolled form adds an evaluation of X only when Y matches
  without consuming input: then `X*` evaluates X again where it failed, in the
  state after Y, which differs only by bindings that Y makes outside its calls,
  of which there are none; by Lemma 2 it costs at most the evaluation of X there
  that the view makes. Its iterations are those of X and Y, one outer iteration
  for each match of Y, so at most one more failed iteration for each repetition
  that it starts and one at the end: at most twice those of the view and two
  more. The absorption `X* (X / Y)*` evaluates X once more where `X*` stopped, as
  in N9.
- *N12, K4.* X calls no rule and its matches have a bounded length, so `!X` and
  `X` examine at most $2d$ characters and iterations together; `_|_` examines
  none.
- *N12, left factoring.* In `p b₁ / … / p b_j`, $G_i$ evaluates `p` in the state
  $\sigma$ of the choice for each alternative that it tries, after the earlier
  ones failed without a cut escaping (F2); each such evaluation after the first
  costs at most the first by Lemma 2, and the first is the one that
  `p (b₁ / … / b_j)` makes. When `p` matches, each `b_t` starts at the end of
  `p` in the state after it, as in the view (F1), and the view tries the same
  rests in the same order. So $G_i$ makes at most $j - 1$ more evaluations of
  `p`.

The calls that $G_i$ makes in an evaluation that $G_{i+1}$ does not make are those
of an evaluation again, which by Lemma 2 have the keys of calls of the first,
which $G_{i+1}$ makes; N8 and K4 call no rule. $\square$

**One rewrite in every evaluation.** Let the occurrence of $e$ lie in the body
$B$ of the rule $r$, let $M_e$ be the product of the upper bounds of the bounded
repetitions of $B$ around it, at least 1, and let
$c_i = a_\rho + b_\rho M_e$. Then for every body evaluation of a rule at a state,
which $G_i$ and $G_{i+1}$ both make,
$$\mathrm{own}_{G_i} \le c_i \cdot \mathrm{own}_{G_{i+1}}.$$

*Proof.* By induction on the nesting of the bodies of rules that are not memoized
inside the own cost, which is finite in each evaluation. Split the own cost of a
body evaluation in $G_{i+1}$ as $X + \sum_N \mathrm{own}_{G_{i+1}}(N)$, where $N$
ranges over the body evaluations of rules that are not memoized, inside it and
not inside another such $N$, that evaluate the occurrence in their own cost, and
$X$ is the rest. The same $N$ are made in $G_i$, and the induction hypothesis
bounds each by $c_i$. If the body evaluation is not one of $r$, its part outside
the $N$ evaluates no occurrence and is the same in both. Otherwise $e$ is
evaluated in it at most $M_e (1 + I)$ times, where $I$ is the number of
iterations of the unbounded repetitions of $B$ around $e$: each such repetition
evaluates its element once for each iteration and once more where it stops. The
context of $e$ in $B$ is the same in $G_i$ and $G_{i+1}$ and is evaluated in the
same states, and $X \ge 1 + I$, the body evaluation and those iterations. By
Lemma 3, each evaluation of $e$ adds to what $e'$ makes at most $b_\rho$ and the
evaluations again of parts of $e'$ that the table counts in $a_\rho - 1$. Each
evaluation again costs at most the evaluation of $e'$ that it repeats, which by
Lemma 2 evaluates the occurrence nowhere in its own cost, so it lies in $X$ and
costs the same in $G_i$ and $G_{i+1}$. So
$$\mathrm{own}_{G_i} \le a_\rho X + b_\rho M_e (1 + I) + c_i \sum_N \mathrm{own}_{G_{i+1}}(N) \le c_i \cdot \mathrm{own}_{G_{i+1}}.$$
A growth run is a sum of body evaluations, so the bound holds for it too.
$\square$

## Lemma 4 (Calls)

Every call that the parse of $G$ makes has the key, and is in the context of
growths, of a call that the parse of $V$ makes.

*Proof.* Let $\mathcal{R}_i$ be the reference parse of $G_i$, which evaluates
every call ([Memoization](memoization.md#setting)). By Lemmas 1 and 3 and the
congruence of $\equiv_E$ in a context that reads no value of the replaced
expression in a predicate ([Equivalences](grammar-laws.md#equivalences)),
$\mathcal{R}_i$ and $\mathcal{R}_{i+1}$ evaluate every expression outside the
occurrence of $e$ in the same states, and every call that $\mathcal{R}_i$ makes
inside an evaluation of $e$ has the key of a call that $\mathcal{R}_{i+1}$ makes
inside the evaluation of $e'$ in the same state, by induction on the nesting of
the evaluations; the calls that only $\mathcal{R}_i$ makes are in evaluations
again, and by Lemma 2 they have the keys of calls of the first evaluation, which
$\mathcal{R}_{i+1}$ makes. A state fixes the growths open at its position, so the
contexts agree. So the calls of $\mathcal{R}_0$ have keys
among those of $\mathcal{R}_m$. A memoized parse evaluates the same expressions in
the same states as its reference parse, less those inside the calls that the
memo answers (the reference-parse argument of
[Lemma 15](linear-cost.md#lemma-15-regions)); a call that the memo answers has the
key of the call whose evaluation stored the entry, which made the same calls.
$\square$

**Keys of the view.** In the parse of $V$ from scratch, the first call with a key
of a memoized rule outside every context of growths finds no entry under its key,
so it is evaluated: for a rule on no cycle and for a member that is not a leader
it is a keyed evaluation, and for a leader it starts a growth run. So every key of
a keyed evaluation or a growth run of the parse of $G$ is the key of at least one
keyed evaluation or growth run of the parse of $V$, of the same kind, since the
memoized rules and the leaders are those of $G$.

## Theorem (Cost View)

For every input, a parse of $G$ in the scope above and the parse of $V$ from
scratch satisfy
$$C'_G \le 3A \cdot C_V, \qquad A = \prod_{i < m} c_i = \prod_{i < m} \big(a_{\rho_i} + b_{\rho_i} M_{e_i}\big),$$
where $\rho_i$ and $e_i$ are the rewrite and the occurrence of step $i$. $A$
depends only on $G$. In the compute units of each backend $b$,
$$U'_b(G) \le 3A\,u_b \cdot U_b(V),$$
where $U_b(G)$ is the units that $b$ charges for the parse of $G$, $U'_b(G)$ is
$U_b(G)$ less the units that it charges for the values that actions build from
reshaped values, and $u_b$ is the largest number of units that $b$ charges for
one unit of the cost, a constant of the grammar and the backend.

$U_b(V)$ is a definition, not a quantity that a backend measures: the units that
$b$ would charge for the parse of $V$, with the rule bodies of $V$ and the
memoized and transient rules, left-recursion leaders and run sites of $G$. No
backend runs that parse, since the view is never compiled (the comment at the
head of `normal_view.go`), and a compilation of $V$ would decide those for $V$.
The proof uses only that $U_b(V) \ge C_V + I_b$, with $I_b$ as below, which holds
because $b$ charges at least one unit for each unit of the cost of a parse. A
measurement approximates $U_b(V)$ by compiling $V$ as a grammar of its own, as
the [measured controls](#measured-controls) do. Where the compiler decides the
same memoized and transient rules, leaders and run sites for $V$ as for $G$, the
parse of the compiled $V$ is the parse of $V$ and the measurement is $U_b(V)$;
where it decides otherwise, the measurement can differ from $U_b(V)$ in either
direction, and the measured ratio is evidence, not a check of the bound.

*Proof.* Every unit of $C'_G$ is in the own cost of exactly one keyed evaluation,
growth run, or of the evaluation of the start rule where the parse begins. The
own cost of a keyed evaluation is a function of its key: by (F1) the evaluations
with one key make the same steps outside the keyed evaluations and growth runs
that they start, in either lookahead state by (A1), and each call costs the same
whether the memo answers it or not. So is the own cost of a growth run: the growth starts in the state of its call
and evaluates its steps and the calls in its context as that state and the input
decide, and the evaluations again that $G$ adds inside it call no rule of a
cycle (Lemma 2), so they change no result of the context. Applying
*One rewrite in every evaluation* along $G_0, \ldots, G_m$, the own cost in $G$ of
the keyed evaluation, growth run or start with a key or state is at most $A$
times the own cost of the one of $V$ with the same key or state.

By Lemma 3 of the [Linear Cost](linear-cost.md#lemma-3-body-evaluations) proof,
which holds for every grammar, the parse of $G$ makes at most three keyed
evaluations with one key, in both lookahead states together. It starts at most
two growth runs with one key, one in each lookahead state: a leader is memoized
at every call, the result of a growth outside every context reads no growth that
was open when its call started, so it is stored (condition (S) of
[Memoization](memoization.md#setting)), and an entry stored outside a lookahead
answers the calls in both states. By *Keys of the view* the parse of $V$
makes at least one with each such key. The evaluation where the parse begins is
one in both. Summing gives $C'_G \le 3A \cdot C_V$.

Each backend charges at least one unit for each unit of the cost of a parse: a
call (`Parser.Call`, `InvokePlain`), a character that a terminal examines
(`Input.Decode`, `MatchLiteral`), an iteration, an operator applied and each
value that an action builds (`SpendValue`). It charges at most $u_b$ units for
each: the expressions between two charges are bounded by the grammar, and the
recording of an expectation compares at most the expected items of the grammar.
The units that it charges for the input as a whole, before the parse, are the same
for $G$ and $V$; call them $I_b$. So
$U'_b(G) \le u_b C'_G + I_b \le 3A\,u_b C_V + I_b \le 3A\,u_b (C_V + I_b) \le 3A\,u_b\,U_b(V)$,
the last by the definition of $U_b(V)$.
The four backends charge in this way: the closure backend with the charging
graph that a budget selects, the recursive and the iterative bytecode machines,
which share the step of `vm.go`, and the generated Go parsers, which charge the
calls, the iterations and the reads of the input through `runtime/` and leave
straight-line code between charges that the grammar bounds.

**Streams.** A stream parse is a parse in which the streaming repetition of the
start rule delivers its elements and discards the input, the memo entries
before them, and their values. It evaluates what an ordinary parse evaluates
([Lemma 15](linear-cost.md#lemma-15-regions), *The reference parse*), Lemma 3 of
the Linear Cost proof holds of it, and Lemma 2 holds of it by the remark after
Lemma 2, so the count above holds. The evaluations that the view drops contain
no stream (`evaluatedAgainCheaply`) and build no value, so the input that `text`
copies in a stream is copied by actions, which the theorem covers when they read
no reshaped value.

**Documents.** Let $P$ be a parse of a [Document](../runtime/documents.md). It
memoizes the rules that a parse from scratch memoizes and also the transient
rules that lead no cycle (`memoized` and `docOnlyMemo` in
`internal/engine/core/call_ops.go`), whereas the parse of $V$ takes the memoized
rules of a parse of $G$ from scratch (`memoizedRules` in
`internal/compile/complexity.go`, through `newViewClassifier`). Split the cost
of $P$ by the decisions of a parse from scratch: its keyed evaluations and
growth runs are those of the rules that a parse from scratch memoizes, and a
call of a rule that only a Document memoizes counts, in the own cost of its
caller, as a call of a rule that is not memoized: the call, and the evaluation
of the body when $P$ makes it. Let $D$ be the lookups and stores of these calls,
which a parse from scratch does not make.

By the [Documents](documents.md#theorem-document-parses-equal-fresh-parses)
theorem and its proof, $P$ evaluates the expressions of the reference parse of
$G$ on its text at the same states, less those inside the calls that the memo or
a carried entry answers and the iterations that a repetition reuses from a
recorded run. So outside $D$ the keyed evaluation, growth run or start of $P$
with a key makes the steps of the one of the parse of $G$ from scratch with that
key, by (F1) and (A1), with three differences, each of which only removes cost:
a call of a rule that both memoize can be answered in one and evaluated in the
other, and costs the same either way; a call of a rule that only a Document
memoizes can be answered by an entry, which leaves out the evaluation of its
body; and a repetition can resume a recorded run, which leaves out the
evaluation of the iterations that it reuses, whose operations are Document units
([Lemma 12](linear-cost.md#lemma-12-document-units)). So the own cost in $P$
outside $D$ is at most the own cost in $G$ from scratch with the same key, which
the proof above bounds by $A$ times that of $V$.

The calls of $P$ have the keys of calls of that reference parse, which the proof
of Lemma 4 gives to the parse of $V$. Lemma 3 of the Linear Cost proof counts
the parse itself, and the proof of Lemma 12 applies it to a Document parse: at
most three keyed evaluations with one key. No leader is memoized only by a
Document (`docOnlyMemo`), and a carried entry holds no result that read a
growing result, so a growth run with a key is started at most once in each
lookahead state, as above. The count above gives
$C'_P - D \le 3A \cdot C_V$, with $C_V$ of the parse of $V$ from scratch.

$D$ is Document work: the units that the default budget counts for a Document
parse leave out the memo protocol of a call of a rule that only a Document
memoizes, except a constant for each variable of the key of its lookups
([Lemma 12](linear-cost.md#lemma-12-document-units), *The units that the
default budget counts*). Each such lookup belongs to a call that $C'_P - D$
counts, and a key has at most the variables of $G$, so the default budget
counts for $D$ at most a constant of $G$ for each unit of $C'_P - D$. The other
Document units are those of Lemma 12, which bounds them in terms of the parse
itself; this proof adds nothing to them. $\square$

## Measured Controls

The constant is a bound, not an estimate. Each grammar below makes one rewrite,
or a few, repeat its work on every element of the input. The written grammar
and its view, each compiled as a grammar of its own, parse the same input on the
three backends of the engine and in generated Go, with $n = 500$, 1,000 and
2,000 repetitions of the element; the cells give the least and the greatest
ratio of the units that the written grammar charges to those of the view. No
grammar reads a value in a predicate, so the view keeps no sequence of one item
and its formatted text is its syntax tree. Neither the written grammar nor its
view memoizes a rule: `main` is called from nowhere and `k` calls no rule, so
both are transient. The run sites that the compiler chooses for the view are not
compared with those of the written grammar, so the measurement approximates
$U_b(V)$ as the theorem describes.

| Rewrites | Written grammar | Input | $3A$ | Closures | Bytecode | Iterative | Generated Go |
|:--|:--|:--|--:|--:|--:|--:|--:|
| N1, N8 ($k = 2$) | `(("a" ("b" "c")) / ("d" / "e"))* $$` | `abcde` | 6 | 1.07 | 1.07 | 1.07 | 1.10 |
| N2, N1 | `("a"* / "b") "c" $$` | `a`, then `c` | 3 | 1.00 | 1.00 | 1.00 | 1.00 |
| N3 | `("aaaaaaaa" "c" / "aaaaaaaa" "c" "d" / [a-z])* $$` | `a` | 6 | 1.56 | 1.69 | 1.69 | 1.58 to 1.59 |
| N4 | `("x" _\|_ "y" / _\|_ / "a")* $$` | `a` | 3 | 1.00 | 1.17 | 1.17 | 1.00 |
| N5 | `(!!"b" "b" / &(&"a") "a")* $$` | `a` | 3 | 1.00 | 1.00 | 1.00 | 1.00 |
| N6, N8 ($k = 2$) | `("a" #cut / "b" / "c" #cut "d")* $$` | `acd` | 6 | 1.00 | 1.23 to 1.24 | 1.23 to 1.24 | 1.05 |
| N8 ($k = 8$) | `("a" / "b" / … / "h")* $$` | `h` | 24 | 1.17 | 2.40 | 2.40 | 1.17 |
| N9 | `(k* k* [a-c])* $$` | `a` | 9 | 1.66 to 1.67 | 1.70 | 1.70 | 1.67 |
| N10 | `((k*)* [a-c])* $$` | `aaaaaaaaaba` | 12 | 1.34 to 1.35 | 1.32 | 1.32 | 1.33 to 1.34 |
| N11, unrolling | `(k* ("x"? k*)* [a-c])* $$` | `a` | 12 | 1.54 | 1.67 | 1.67 | 1.67 |
| N11, absorption | `(k* (k / "x")* [a-c])* $$` | `a` | 12 | 1.63 | 1.61 | 1.61 | 1.64 |
| K4 ($d = 8$), N4, N1 | `(!"aaaaaaaa" "aaaaaaaa" / "a")* $$` | `a` | 51 | 2.50 to 2.51 | 2.80 to 2.81 | 2.80 to 2.81 | 2.50 to 2.51 |
| Left factoring ($j = 3$), N8 ($k = 3$) | `(k "x" / k "y" / k "z" / [a-c])* $$` | `aaaaaaaaab`, then `a` | 27 | 1.95 to 1.96 | 2.24 to 2.25 | 2.24 to 2.25 | 2.02 to 2.03 |

Each grammar is the body of `rule main`, and `k` is `rule k = "aaaaaaaaab"`.
The ratios do not grow with $n$ and stay below $3A$, since the evaluation that
the written grammar makes again is only a part of the cost of the view around
it.

A second control checks the conditions of
[Rewrites That Change Values](#rewrites-that-change-values). In

```text
rule main = x:("abc" #cut / "z") {len($x) == 1} (e "q" / "x")* $$
rule e = e "x" / "x"
```

the predicate reads the value of `"abc" #cut`, a tuple of one element, so the view
keeps it as a sequence of one item. On `abc` followed by $n$ times `x`, for
$n = 250$, 500 and 1,000, the written grammar and its view charge the same
units on all four targets, ratio 1.000, and both grow as $n^2$ through the
tail. The same holds with the cut in the last alternative,
`("w" / x:("abc" #cut))`, and for a choice left with one alternative that is a
sequence, `x:("a" ("b" "c" / _|_)) {len($x) == 2}`. A view that replaced the
sequence by its item would reject the input at the predicate and skip the
tail, and the ratio would grow linearly in $n$, to about 1,000 at
$n = 1000$. The formatted text of a view writes a sequence of one item and a
choice of one alternative as the bare item, so this control compiles the
syntax tree of the view instead of its text.
