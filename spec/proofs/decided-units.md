# Decided Units

Arata Furukawa · Last revised 2026-11-12

A unit that is not a lookahead is committing for a continuation that it
**decides** ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost)): every path of the
unit, compared with every path of the continuation from their common start,
passes symbols that both evaluate alike and then parts at a decisive symbol
that at most one of them can pass. This proof shows that such a unit is a case
of [Lemma 4](linear-cost.md#lemma-4-scanned-cost) of the Linear Cost proof, as
[Adding a Condition](linear-cost.md#adding-a-condition) requires: Lemmas 1 to 4
state what an evaluation does along a path, that symbols passed alike end
alike, what a cheap symbol costs again, and that at most one side passes a
decisive symbol, and the theorem gives the conclusion of *Direct activations*
for a decided unit. The points that it leaves open are listed at the end.

## Setting

Fix a grammar, an input and a parse. Units, continuations, scanning sources,
bounded scans, paths, symbols that are alike, decisive symbols, cheap symbols
and trivia rules are as defined in [Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost). As in Lemma 4 of the
Linear Cost proof, a scanning source below is one that is not a bounded scan,
and its activations are its iterations, the operators that it applies or the
evaluations of a skip expression.

**Paths as the comparison builds them.** The paths of an expression are built
from its structure, with the symbols that the specification lists. A call, a
literal, a character class, `.`, a predicate, a `#cut`, an anchor and a
repetition that the comparison passes whole are paths of one symbol, and so is
a lookahead, whose expression is evaluated inside the symbol. Groups, captures,
discarded
expressions, `#atomic`, actions and streams have the paths of their expression.
A sequence has the concatenations of the paths of its items, a choice the paths
of its alternatives, an option the empty path and the paths of its expression,
and a repetition the concatenations of any number of paths of its expression,
with at least as many as its lower bound and at most as many as its upper
bound. As the specification states, the comparison **looks into** an option,
a choice, a repetition or a lookahead only when it is cheap in all its parts,
the expression itself included, and a lookahead of the unit that it passes, or
a negative lookahead of the unit on which it decides, reaches no scanning
source, through the rules that it calls as well. Every other expression, `#recover`, attributed expressions and Operators
Rules among them, is a **stop**: a path that reaches it ends with it, and the
comparison decides nothing at it. In particular a repetition without an upper
bound that is not a bounded scan is never looked into.

The paths of a continuation are built in the same way from what the parser
evaluates after the unit: the rest of each enclosing sequence; at the end of
an iteration, the repetition again, followed by what follows it; at the end of
an alternative, what follows the choice; at the end of a rule body, what
follows each call of the rule; at the end of the start rule, the end of the
parse; and, at the end of an operand of an Operators Rule, what the rule tries
after it. The comparison drops a call of a trivia rule from a path where it
follows a call of the same rule across symbols that consume nothing, as the
specification states.

**The comparison.** A pair of paths, one of the unit and one of the
continuation, is **settled** when, from their common start, they pass a
sequence of **steps** alike and then either reach a decisive symbol, at which
they **part**, or the path of the unit ends. A step is a pair of alike symbols,
or, where the comparison passes two symbols by the characters that they have in
common, one character of each. Between the steps, either side may pass a
predicate, a `#cut`, an anchor or a lookahead of its own that the comparison
passes, and the continuation a dropped call. Where the comparison expands a call that is not
alike, the path stands for the paths in which the call is replaced by a path
of the body of its rule, and each of those pairs must be settled. The unit
decides the continuation when every pair is settled, with the further conditions of the strict
comparison on what is cheap. A stop ends a path, so a settled pair has no stop
before the point where it parts, and a path of the unit that ends has none.

## Lemma 1 (Paths)

Let $x$ be an evaluation that starts at a position $s$, of an expression or of
a continuation, and fix a moment during it. Then there is a path $\pi$, the
**current path** of $x$ at that moment, such that:

1. the symbols of $\pi$ that $x$ has started are a prefix of $\pi$; each
   starts where the one before it ended, and each but the last succeeded;
2. every other evaluation that $x$ has made is an evaluation of a part of the
   expression whose current path is a segment of $\pi$, lies inside one of
   those symbols or inside the evaluation of a stop of $\pi$, or is an attempt
   that failed in
   an option, a choice or a repetition that $\pi$ passes and that the
   comparison looks into, or a call that the comparison drops;
3. $x$ starts the evaluation of a stop of $\pi$ only after every symbol of
   $\pi$ before it succeeded.

*Proof.* By induction on the structure of the expression, following the
construction of the paths. A symbol is its own path, and a stop is a path
that $x$ starts at once. The items of a sequence are evaluated in order, each
at the end of the one before, and the sequence goes on to an item only when the
item before it succeeded, so the current paths of the items concatenate. A
choice that is looked into tries its alternatives in order: those before the
one in progress failed, and the current path is that of the alternative in
progress. An option or a repetition that is looked into does the same with its
expression: an attempt that failed ends it, and its current path is that of
the iterations that succeeded and of the one in progress. Groups, captures,
discarded expressions, `#atomic`, actions and streams evaluate their
expression once. Predicates, cuts and anchors consume nothing. A continuation
is the evaluation of the remaining items of the enclosing expressions, one
after another, so its current path is the concatenation of theirs, through the
same ends as its paths. A dropped call is evaluated by $x$ but is no symbol of
$\pi$. In each case a symbol of $\pi$ starts only after the symbols before it
succeeded, which gives items 1 and 3, and an evaluation outside the symbols
and stops is one of the failed attempts of item 2. $\square$

The lemma holds as well with any call of $\pi$ that $x$ has started replaced
by the current path of the evaluation of its body, which is how the expansions
of the comparison follow an evaluation into a call.

The failed attempts of item 2 lie in expressions that are cheap in all their
parts, so they contain no scanning source of their own: an activation of a
scanning source in them lies inside a call that they make.

## Lemma 2 (Alike Symbols)

Let two symbols be evaluated at one position $p$, by two evaluations that have
the same values of the variables that the symbols read. If the symbols are
alike, both fail, or both succeed and end at one position. A call of a trivia
rule that the comparison drops matches nothing.

*Proof.* Calls of one rule $A$ that is on no left-recursion cycle, with the
same level restriction or with none in both, have the same key: the rule, the
position, the level and the values of the variables that $A$ reads ([Setting
of the Memoization proof](memoization.md#setting)). A call of a rule that is on
no cycle reads no open growth, since a call reads only the growths of the
leaders of its own cycle ([Lemma 2 of the Memoization
proof](memoization.md#lemma-2-reading-an-open-growth)). By [Lemma 1 of the Memoization
proof](memoization.md#lemma-1-context), whether such a call matches and where
it ends are functions of its key, in either lookahead state, and by the
[theorem](memoization.md#theorem-memoization-never-changes-the-result) the
parse gives the outcome of the reference parse, whether the memo answers the
call or not. The calls in a pair of alike symbols read the same variables:
either the rule reads none, or the two sides have the same variables, which
holds until either side passes a predicate that binds a variable or the end of
a rule body that binds. Equal literals with the same case sensitivity, equal
character classes and `.` examine the same character at $p$ and agree on it.
Where the comparison passes two symbols by a character that they have in
common, the step is a single character that both accept at $p$, which both
consume or neither does: two literals passed so have the same case
sensitivity, and are compared up to case only when both are case-insensitive.

Two equal repetitions that the comparison passes alike evaluate the same
expression, which contains no predicate, `#cut`, `#recover`, attributed
expression or Operators Rule, whose terminals agree as above and whose calls
are alike in the sense above, and which reads the same variables on both
sides. So by induction on the structure of the expression each iteration of
one, started where the same iteration of the other started, fails in both or
ends at one position in both. A repetition
is greedy: it stops at the same iteration in both, after the same number of
iterations, so both match or both fail, and both end at one position.

A trivia rule has the body `X*`, atomic or not, where `X` cannot match without
consuming input, and neither it nor a rule that it calls runs an action or
contains `#recover`, an attributed expression, a stream, an Operators Rule or
a call of a left-recursion cycle. So a call of it always succeeds, and an
evaluation of `X` at a position depends only on that position and the
variables that the rule reads: two calls of one trivia rule at one position
with the same variables are alike calls, as above. A call that the comparison
drops follows a call of the same rule across symbols that consume nothing, with
no predicate that binds a variable between them when the rule reads variables,
so it starts where the first one ended, with the same values of the variables
that it reads. The first call stopped there because `X` failed, since `X`
cannot match without consuming input, so `X` fails again, and the dropped call
matches nothing. $\square$

So along a settled pair, an evaluation of the unit and an evaluation of the
continuation that both start at $s$ and pass every step evaluate the steps at
the same positions $s = t_0 \le t_1 \le \dots \le t_m$, and both reach the
decisive symbol, or the end of the path of the unit, at $t = t_m$: the steps
consume the same input on both sides, and the symbols between them consume
nothing.

## Lemma 3 (Cheap Symbols)

Let a unit $w$ decide its continuation $\kappa$, and let $\kappa$ evaluate
either a cheap symbol at a position where $w$ evaluated an alike symbol, with
the same values of the variables that it reads, or a lookahead that only
$\kappa$ evaluates, or an alternative of a negative lookahead of $\kappa$
before the one that the match of $w$ implies. Then every activation of a
scanning source in that evaluation lies:

- in an evaluation of a call that the memo of the parse answers or that is
  memoized there, which [Lemma 3](linear-cost.md#lemma-3-body-evaluations) of
  the Linear Cost proof counts with the evaluations of its key; or
- in an evaluation of a call of a rule that calls no rule, at a position where
  $w$ evaluated the same call, with the same variables, and examined the same
  input.

Outside such calls it makes no activation, so each body evaluation that it
makes there examines the input that [Lemma 1](linear-cost.md#lemma-1-local-cost)
of the Linear Cost proof bounds, as in case 3 of Lemma 4.

*Proof.* A cheap symbol is one of four kinds. An expression without a scanning
source makes no activation, nor do the calls that it makes, which are again
without a scanning source. A call that the memo answers evaluates nothing. A call of a rule that
calls no rule evaluates only the body of that rule, and by Lemma 2 it matches
alike in both, and its evaluation repeats the steps of the first, since the
body reads nothing that differs between them; so it examines the same input
and makes the same activations. A call of a rule whose body is cheap in all its
parts evaluates that body at the position where $w$ evaluated it, with the
same key, and the two evaluations take the same steps: by [Lemma 1 of the
Memoization proof](memoization.md#lemma-1-context) the body depends on
nothing else, and in either lookahead state they differ at most in farthest
failures and in the errors that `#recover` builds, which decide nothing in a
body without `#recover`. Its parts are cheap, so the claim holds of each call
in it by induction on the depth of these calls, which is finite, since a rule
whose body is cheap in all its parts is cheap only through rules below it. A
call of a rule that reads variables is cheap only where both sides have the
same variables, as in Lemma 2.

A lookahead that only $\kappa$ evaluates, or an alternative of a negative
lookahead of $\kappa$ before the one that the match of $w$ implies, has no
scanning source of its own, and each call in it is answered by the memo or
reaches no scanning source, as the specification requires. A call of a
memoized rule that the memo does not yet answer at the position is evaluated
once there in its lookahead state and stored; Lemma 3 of the Linear Cost
proof bounds those evaluations by key. $\square$

## Lemma 4 (Decisive Symbols)

Let a pair of paths of the unit $w$ and the continuation $\kappa$ be settled
at a decisive symbol, and let both evaluations reach it at the position $t$
after passing every step. Then at most one of them passes it.

The decisive symbols are those of the specification: symbols by first
characters cannot match without consuming input, and a positive lookahead
decides against a symbol, or against a positive lookahead of the other side,
that, like its expression, cannot. Two literals decide when they have the same
case sensitivity and neither is a prefix of the other, compared up to case
when both are case-insensitive, so that neither can match at a position where
the other matches. A negative lookahead `!K` on one side decides when every
evaluation that passes the next symbols of the other side's path, in order from
$t$, matches an alternative of $K$ at $t$; the other side passes the decisive
point when it passes those symbols. The end of the parse is
decisive in the continuation: nothing follows it on its path.

*Proof.* Two symbols that cannot match without consuming input consume, where
they match at $t$, the character at $t$, which is a first character of each;
if the first characters are disjoint, at most one matches. Two literals that
both match at $t$ match where the other does, which the condition excludes.
Two character classes with no character in common cannot both accept the
character at $t$. If one side passes `!K`, then $K$ does not match at $t$, so
the other side does not pass the symbols whose match would match $K$ there:
the alternative of $K$ that they match consists of calls, literals, classes and
`.`, each matching where the corresponding symbols matched, and by Lemma 2 a
call of $K$ matches as the alike call of that side does, although $K$ is
evaluated in another lookahead state. If one side passes `&K`, then $K$ matches
at $t$ and consumes the character at $t$, which is a first character of $K$;
the next symbol of the other side, or the expression of its positive
lookahead, matching at $t$, would consume it as one of its own. When the continuation's path ends with the end of the parse, the
continuation evaluates nothing after it. $\square$

## Theorem (Decided Units in Lemma 4)

In the setting of [*Direct activations*](linear-cost.md#lemma-4-scanned-cost)
of Lemma 4 of the Linear Cost proof, let $\alpha$ and $\alpha'$ be successful
activations of a scanning source $\sigma$ of the body of a rule $R$, both
direct in a body evaluation $E$ of $R$ that is not a growth step and both
starting at $q$, with $\alpha$ first, let $w$ be the abandoned unit that
*Restarts* gives and $\kappa$ the continuation of $w$ that the parser
evaluates from the start $s$ of $w$ the last time before $\alpha'$, and let
$w$ decide $\kappa$. If $\sigma$ is the operators of an Operators Rule, let
its evaluation that contains $\alpha$ not contain $w$. Then no such pair
exists: a decided unit gives the conclusion of case 1.

Moreover, until $\kappa$ passes the point where its current path parts from
the path of $w$ along which $w$ failed, every activation of a scanning source
that $\kappa$ makes lies in one of the calls of Lemma 3; outside them its body
evaluations make no activation, as in case 3 of Lemma 4.

*Proof.* $w$ is not a lookahead, so it is abandoned by failing.

First, $\alpha$ lies inside the evaluation of a stop of the current path $\pi$
of $w$ at the moment $\alpha$ starts. By Lemma 1, $\alpha$ lies inside a
symbol of $\pi$, inside a stop of $\pi$, or in a failed attempt or a dropped
call. It is direct in $E$, so it lies in no call made in $E$, which rules out a
call symbol, a dropped call and the calls of a failed attempt; a literal, a
class or `.` holds no activation; a lookahead of $w$ that the comparison passes,
or a negative one on which it decides, reaches no scanning source, and a
repetition that it passes whole and a failed attempt are cheap in all their
parts, so none of them contains a scanning source of its own. A positive
lookahead of $w$ on which the comparison decides may contain one; it is taken
below. What remains is that the activation is
made by a construct that encloses several symbols of $\pi$: an iteration of a
repetition that $\pi$ passes, an operator applied by an Operators Rule whose
parts $\pi$ passes, or an evaluation of a skip expression that $\pi$ passes. A
repetition that is a scanning source is never looked into, so it is a stop; an
Operators Rule that $w$ evaluates as a whole is a stop, and one whose parts
$\pi$ passes contains $w$, which the hypothesis excludes; and a skip expression
of a `#recover` that $w$ evaluates lies in that `#recover`, which is a stop,
since $w$ is no base of it, the base of a `#recover` holding no evaluation of
its own skip expression. So $\alpha$ lies in a stop of $\pi$, or in a positive
lookahead of $w$ on which the comparison decides.

The same holds of $\alpha'$ in $\kappa$, with its current path $\rho$ at the
moment $\alpha'$ starts, by the same argument, since a lookahead of $\kappa$
that only $\kappa$ evaluates has no scanning source of its own, with two more
cases for a
construct that encloses symbols of $\rho$ and started before $w$ ended. An
evaluation of the place of $\sigma$ that encloses $w$ cannot hold $\alpha$,
which lies in $w$, since the place of $\sigma$ is not evaluated inside its own
activation without a call; so when $\sigma$ is the operators of an Operators
Rule whose evaluation, in progress after $w$ ended, holds $\alpha'$, that
evaluation contains $w$ and $\alpha$ too, which the hypothesis excludes. When
$\kappa$ evaluates the skip expression of a `#recover` as the continuation of
its base, that `#recover` contains $w$, and $\alpha$, an evaluation of the
same skip expression inside $w$, would lie in an evaluation of the `#recover`
inside its own base with no call between, which cannot be. For a repetition,
the evaluation of $\sigma$ that holds $\alpha'$ does not enclose $w$ for the
same reason, so it is a new evaluation of a stop.

Replace in $\pi$ and $\rho$ each call that the comparison expands by the
current path of the evaluation of its body (Lemma 1); this moves no stop or
lookahead that holds $\alpha$ or $\alpha'$, since neither lies in a call. The
pair $(\pi, \rho)$ so obtained is a pair of paths of $w$ and $\kappa$, so it
is settled. If the path of $w$ ends before they part, $\pi$ has neither a stop
nor a decisive lookahead, which contradicts the first step. Otherwise a stop
that holds $\alpha$ or $\alpha'$ lies after the point where the paths part,
and a lookahead that holds one of them is the decisive symbol of its side. By
item 3 of Lemma 1, each side passed every step before it started the stop or
the lookahead, and by Lemma 2 both reached the decisive point at the same
position $t$.

If both activations lie in stops, both sides passed their decisive symbols
before them, and by Lemma 4 at most one of them did; if the decisive symbol of
$\rho$ is the end of the parse, $\rho$ has no stop after it. Otherwise one of
them lies in the expression $K$ of a positive lookahead `&K` that its side
evaluates at $t$. It is a successful activation, so it consumes the character
at its start $q \ge t$, and the evaluation of $K$ from $t$ that holds it
reaches $q$ only through evaluations that consume input from $t$ on. The
first of them consumes the character at $t$ before $K$, or a lookahead in it,
consumed anything, so by the definition of first characters that character
is a first character of $K$. The decisive symbol of the other side, or its
expression when it is a positive lookahead, cannot match without consuming
input and has first characters disjoint from those of $K$, so it fails at
$t$. The activation of the other side then lies neither
in a stop after its decisive symbol, which it did not pass, nor in that
symbol, which would make it a positive lookahead whose expression consumed
the character at $t$ as a first character. Each case contradicts the first
step.

For the second claim, let $\pi$ be the path along which $w$ failed. By Lemma 1
what $\kappa$ evaluates before the point where its current path parts from
$\pi$ consists of the steps, which by Lemma 2 it evaluates at the positions
where $w$ evaluated them, the symbols that the comparison passes on its side,
the dropped calls and the failed attempts of parts that it looks into. A
failed attempt is the prefix of another current path of $\kappa$, which is
settled with $\pi$ too, so up to the point where that path parts from $\pi$ its
symbols are steps evaluated where $w$ evaluated them; what it evaluates beyond
that point, it evaluates after its current path passed the point where it parts
from $\pi$, which the claim leaves out. The steps are cheap, and the
lookaheads of its own are those of the last cases of Lemma 3. A dropped call
matches
nothing (Lemma 2): its rule's `X` fails once at a position, which costs a
constant when `X` has no scanning source, the condition under which the
specification drops it. Lemma 3 places every activation of these evaluations
in the calls that it lists. At the decisive point itself, when the symbols
decide by first characters, the symbol of the side whose first characters do
not contain the character at $t$ examines no input beyond $t$: by the
definition of first characters, they include the characters that it can
consume first and those that each lookahead that it can evaluate before it
consumes input can consume first, through rule calls, so it examines input
beyond $t$ neither by consuming nor through such a lookahead. Literals and
classes examine at most their own characters. So if $\kappa$ passes its
symbol, $w$ examined nothing past $t$ there, and otherwise $\kappa$ examines
nothing past $t$. A lookahead of $\kappa$ is one of the last cases of
Lemma 3; the symbols with which $\kappa$ meets a negative lookahead of $w$ are
calls, literals, classes and `.` evaluated where $w$ evaluated the matching
items of the lookahead's expression, with which they match alike (Lemma 4), and
that expression reaches no scanning source, so they make no activation;
and the end of the parse costs a constant. $\square$

The proof does not use the bound of three symbols. That bound and the work
budget of the comparison keep the classification finite; the cost argument
holds for a settled pair of any length.

## Open Points

1. **Repeated activations in alike calls.** The theorem places every
   activation that the continuation makes again before the point where the
   paths part in a call that the unit evaluated at the same position (Lemma 3).
   *Repeats* and the claims of Lemma 4 of the Linear Cost proof that count
   the links, *Memoized links* and *Calls that call no rule*, are stated for
   the calls that a unit and a continuation share. They must still be shown
   for the calls that a decided comparison passes alike, including those that
   are cheap only through their bodies, the calls of trivia rules and the
   calls in failed attempts of parts that the comparison looks into, before
   case 6 of Lemma 4 is complete.
2. **Units of an Operators Rule.** The theorem leaves out the activations of an
   Operators Rule whose evaluation contains the unit, where the paths of the
   unit and of the continuation pass the parts of that rule. They rest on the
   treatment of Operators Rules in Lemma 4 (step 3 of the Linear Cost proof's
   open steps).
3. **Calls inside growths.** Calls of rules of a left-recursion cycle are never
   alike, so inside a growth the comparison parts before such a call or not at
   all. This limits the units that are decided; it leaves no case of the proof
   open.
