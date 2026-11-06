# Linear Cost

Arata Furukawa · Last revised 2026-11-07

This proof shows that the [Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)
bound the [cost](../runtime/parsing.md#cost) of every parse, so that the
classes Linear and Linear Given an Environment Bound of
[Classification](../runtime/parsing.md#classification) hold their promise. It
splits the cost into the parts that the specification counts and bounds each
part by one lemma, so that a new condition is added as a lemma, or as a case of
one, without changing the others.

Lemmas 1, 2, 3, 7, 8, 9, 10 and 11 are proved. Lemma 13 is proved for the
[cost view](#setting) that the classifier reads, given its condition, step 9 of
[Open Steps](#open-steps). Lemma 6 is proved given the count
of activations of Lemma 4 (step 1 of [Open Steps](#open-steps)). The theorem is
proved from the others, so the promise of the class Linear rests on Lemmas 4 and
5, which are outlines, on Lemma 6, which is proved given step 1, and on the
condition of the cost view, step 9. Lemma 12
and its corollary add what a [Document](../runtime/documents.md) parse charges
beyond the cost: they are proved given the same counts, in terms of what the
Document retains, which step 7 of [Open Steps](#open-steps) leaves unbounded,
and of the recovered errors that a parse copies and replays, which $C$ does not
count and step 8 leaves unbounded. Lemma 12 also proves that the units which
the default budget counts for a Document parse are at most those of a parse of
the same text from scratch.
[Correspondence](#correspondence) maps the checks of the classifier to the
conditions and to the lemmas that use them. Lemmas 4 and 5 are given as
outlines: each states its claim, the conditions it uses, and the steps that its
proof must still make precise, listed under [Open Steps](#open-steps). Lemma 4
proves a reduction and bounds the repetitions at one position: the scanned cost comes down to the number of activations, a failure
after a source passes through the forms that condition 2 checks, an activation
is repeated at a position only after an abandoned committing unit or a dropped
growth step, at most one successful activation of a source at a position is
direct in one body evaluation, and every other lies in a re-evaluation of a call
that a committing unit shares with its continuation, which the memo bounds for a
memoized rule and the cycles of calls that nest at one end bound for a rule that
calls no rule when the call is the first that the two share. It proves the sum
over the positions for the activations owned by memoized evaluations of the
sources of rules that call a rule or are memoized at their calls, when the two
evaluations of each memoized link have one lookahead state and one variant of
the rule and are in no context of growths, by charging the activations of each
second evaluation of a memoized rule to the first. It leaves the sum open for
the sources of rules that call no rule and are not memoized, and for the
memoized links that the condition leaves out. Lemma 5 proves the
bound on the sources that a growth step evaluates at fixed offsets after the
position where it grows.

## Setting

Fix a grammar and a start rule such that every rule that the start rule
reaches satisfies conditions 1 to 5, and an input of length $n$. Let $K$ and
$L$ be as in [Classification](../runtime/parsing.md#classification), and
$\hat{n} = n + 1$. The parse is the memoized parse of the
[memoization proof](memoization.md#setting) with the memoization of
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules).
"A constant" below means a number that depends only on the grammar and the
backend.

The classifier decides the conditions on the [cost
view](grammar-laws.md#normalization) of the rule bodies, with the memoized rules,
the left-recursion cycles, the variables and the run sites of the grammar as
written. The proof reads that view as the grammar: its rule bodies match what the
written ones match, and the written grammar costs at most a constant factor more
on every input, which is the condition of the cost view. So a bound on the parse
of the view is a bound on the parse of the grammar. That condition is not proved:
it is step 9 of [Open Steps](#open-steps).

**Cost.** By [Cost](../runtime/parsing.md#cost), the cost of a parse is
$C = C_{\mathrm{body}} + C_{\mathrm{in}} + C_{\mathrm{it}} + C_{\mathrm{val}}$:
the rule bodies evaluated with each growth step counted, the input that
terminals, repetitions of character classes and lookaheads examine, the
iterations of repetitions and the operators applied, and the size of the
values that actions and predicates build. A step of a rule body includes the
calls it makes, so $C_{\mathrm{body}}$ counts each call: a constant, and the
lookup of its key, which costs in proportion to the size of the key
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)). In every
parse $C$ counts these and nothing else of a lookup. What a
[Document](../runtime/documents.md) parse does beyond them, at a lookup or
elsewhere, is no part of $C$: Lemma 12 counts it as **Document units**.

**Pending edits of a Document.** A lookup in the memo of a Document parse of
the reference implementation also applies to the entry it finds the edits of
the log that the entry has not accounted for since it was stored or last found,
and charges one unit for each edit it applies, but never applies more than
$T = 128$: an entry that lags behind by more edits is treated as absent, and
the call evaluates its body again. Let $E$ be the length of the
edit log at the parse: the reference implementation tests the length before it
records an edit, and drops the memo and restarts the log at the edit that finds
65,536 edits in it, so a parse can see a log of 65,536 edits, $E \le 2^{16}$
there, and $E = 0$ outside a Document. Each lookup has at most $\min(E, T)$ of these
units: an entry that edits dropped but that stays in its slot applies the edits
up to the one that drops it again at each lookup that finds it, and an entry
that stays valid applies them once.

The lookup also passes the entries that earlier edits dropped and left in their
slots. A slot can hold several entries under one key, live or dropped: an edit
relinks the entries that it keeps without comparing their keys. Only the
entries at the two ends of the edit can stay, since an entry at $p$ examined a
range $[l, h)$ with $l \le p \le h$: an entry kept unchanged has $h \le a$, so
$p = a$ among the positions $a$ to $b$ that the edit decides, and an entry that
moves has $l \ge b$, so $p = b$. They come to one slot when $b + \delta = a$, a
deletion that inserts nothing. So one edit joins at most two slots, a store
under a key that the slot holds adds no entry, and by induction on the edits a
slot holds at most one entry under a key plus one for each edit that joined
slots into it, at most $E + 1$. Only one of them matters in a parse: a lookup
returns the first entry of the chain under its key, through the chain or the
index alike, and a store replaces that entry in its place, so the others are
not seen until a later edit relinks them. A lookup compares at most 32 entries
of a chain before it consults an index, and Lemma 12 counts the walks and
indexes of longer chains. Calls outside a Document have none of these
units, and the memoization, the evaluations and the other costs are those of
the setting.

**Input that a lookahead examines.** This proof counts each examination of
the input once, for the terminal, character class or `.` that makes it,
whether or not a lookahead encloses it. The cost of a lookahead itself is
then a constant for entering and leaving it, and what its expression costs is
counted for that expression: its scanning sources, such as the repetition in
`&([^!]* "!")`, are scanning sources of the body, and its other parts are
local cost. [Cost](../runtime/parsing.md#cost) lists the input that lookaheads
examine beside the input that terminals examine. If a lookahead were to count
again what the terminals inside it examine, the count would grow with the
number of lookaheads that enclose a terminal, which rule calls do not bound.

**Activations.** A growth step is a body evaluation of the leader. An
**activation** of a scanning source is one unit of its work that evaluates a
fixed part of the grammar once: an iteration of a repetition without an upper
bound, an operator that an Operators Rule applies, or an evaluation of the skip
expression of a `#recover` that is not a [reused
skip](../runtime/parsing.md#skip-runs). The growth of a cycle and the copying
operations, the other scanning sources, make no calls of their own beyond the
body evaluations of the growth. An iteration of a repetition of a character
class is an activation of its own. An iteration of a repetition that a
[Document](../runtime/documents.md) parse reuses from a recorded run is an
iteration of that repetition, and so an activation, one that makes no call. A
copying operation that is a scanning
source reads the input of the own spans of the evaluations that it reads,
as Lemma 6 defines them, and each unit of that input is an activation of the
operation.

**Local and scanned cost.** An evaluation of a rule body has a **local cost**,
the cost of the steps that it makes outside the calls it makes and outside its
scanning sources, and a **scanned cost**, the cost of the steps of its
scanning sources outside the calls they make. Each call is counted once, in
the evaluation that makes it, as a body evaluation of the callee, the steps of
a growth, or the reuse of an entry. The iterations, lookups and records of the
walks of the sites of reused skips are counted apart, as their **walk cost**.
Every unit of $C$ belongs to exactly one of these: the local or scanned cost of
one body evaluation, one call, or the walk cost.

## Lemma 1 (Local Cost)

The local cost of one body evaluation is at most a constant $c_0$ times
$L + 1$.

*Proof.* Outside its scanning sources and the sites of its reused skips, a
body contains no repetition without an upper bound, no `#recover` skip that
is not reused, no operators of an Operators Rule, and no copying operation
whose operand is unbounded in size other than the value of a variable
([Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost),
the definition of a scanning source); a reused skip calls no rule. The body is a finite expression, and
each of its parts is evaluated at most as many times as the product of the
upper bounds of the repetitions that enclose it, a constant. Each such
evaluation outside a call costs a constant, except a copy of the value of a
variable, which costs at most $L$ by the definition of $L$. $\square$

## Lemma 2 (Calls)

*The number of calls is at most a constant times the number of body
evaluations, growth steps included, plus the number of activations.*

*Proof.* Every call is made by a step of a body evaluation, growth steps
included. That step is either outside the scanning sources of the body, or
within an activation of one of them, outside the activations of the scanning
sources nested in it. By the argument of Lemma 1 the first kind makes a
constant number of calls per body evaluation. An activation evaluates a fixed
part of the grammar once, and outside its nested scanning sources that part
makes a constant number of calls, again by the argument of Lemma 1. $\square$

## Lemma 3 (Body Evaluations)

A rule is **memoized** in the parse unless the first exception of
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules) leaves it
out: unless it is a rule other than a leader that calls no rule or that is
called from at most one place, and is not a start rule that its own body calls
through rules called from one place. A body evaluation of a rule of a
left-recursion cycle is **in a context of growths** when a growth of that
cycle is in progress at its position, as the second exception of that section
describes. Let $|\text{levels}|$ be the number of binding levels of the
grammar, counting the absence of a level as one. A call where the value of a
rule is discarded calls the value-free **twin** of the rule, which has calls,
keys and memo entries of its own; this proof counts a rule and its twin as two
**variants** of the rule. The **lookahead state** of an evaluation is $\lambda$
of the [memoization proof](memoization.md#setting): $1$ inside a lookahead and
$0$ elsewhere.

The number of body evaluations of memoized rules that are neither growth
steps nor in a context of growths is at most
$6 \cdot |\text{rules}| \cdot |\text{levels}| \cdot K \hat{n}$. For each
variant, position, level and combination of the values of the variables that
the rule reads, at most two of them are in each lookahead state and at most
three in all.

*Proof.* By [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules),
a rule body is evaluated at most three times at a position for each variant,
binding level and combination of the values of the variables that the rule
reads, with three exceptions. The first concerns the rules that are not
memoized, which the claim leaves out. The second concerns the calls in a context of growths, whose
evaluations the claim leaves out. The third concerns growing rules, whose body
evaluations are growth steps; by
[Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)
a call of a leader grows it at the position of the call, so every body
evaluation of a leader is a growth step, and the claim leaves out all of
them. A rule on no cycle has no context of growths, since only the growths of
a rule's own cycle form its context, and the result of a rule of a cycle
outside every context is memoized like that of any rule. So every body
evaluation that the claim counts is one of the at most three that the section
allows for its variant, position, level and combination. The memo answers
each of these from one entry, the first under its key ([Setting](#setting)),
and a store replaces that entry. An entry stored outside a
lookahead answers the calls in both lookahead states, and one stored inside a
lookahead answers only those inside one, condition (L) of the memoization
proof. A call inside a lookahead is evaluated only when no entry answers it, so
its store replaces no entry stored outside. So in each state at most one
evaluation stores an entry, after which every call in that state is answered, and the only
evaluation that stores none is the one of the first call of the variant at the
position, which is in one of the two states.

A position is one of the $n + 1$ positions of the input. By the definition of
$K$ in [Classification](../runtime/parsing.md#classification), the calls at
one position bind the variables that memoized rules read to at most $K$
distinct combinations of values. Summing three evaluations over the two
variants of each rule, the levels, the positions and at most $K$ combinations
at each position gives the bound. The bound holds in each parse of a
[Document](../runtime/documents.md), whose memo entries from earlier parses
evaluate no body: they answer calls, and what an answer does beyond that, such
as moving the value of the entry, makes no evaluation and is counted by
Lemma 12. It also holds in a [stream](../runtime/streaming.md), which discards
memo entries only before the elements it has delivered, where the input is
gone and no later call is made. $\square$

The evaluations that the claim leaves out are counted elsewhere: those of
rules that are not memoized by Lemma 10, and the growth steps and the
evaluations in a context of growths by Lemma 5. In a context of growths, a
call that finds no result computed with the same growing results and the same
outcomes of the lookaheads it observed is evaluated again, a result that read
the growing result of an open growth is evaluated again in the steps whose
growing results differ, and a leader that grows again at its position inside
another growth there grows once for each context; Lemma 5 bounds these under
condition 5.

In the reference implementation, `Parser.Memoizes` in `internal/engine/core`
decides which calls the memo answers. It memoizes a leader in every parse, and
a rule that `transientRules` in `internal/compile` marks transient only in a
Document parse. A parse with missing-token insertion, which is outside the
classes, memoizes no rule other than a leader that contains `#missing`,
directly or through the rules it calls. `Parser.FirstCall` defers the memo
entry of a rule other than a leader to the second call of the variant at a
position, unless the rule has turned eager and stores at its first call, and
`Parser.reusable` answers a call inside a lookahead from every entry and a
call outside one only from an entry stored outside, which gives the three
evaluations.

## Lemma 4 (Scanned Cost)

*Outline.* Under condition 2, the activations of the scanning sources of all
body evaluations, other than the bounded scans that Lemma 13 counts, number at
most a constant times $K\hat{n}$, and the scanned cost of all body evaluations,
apart from the copying operations, which Lemma 6 bounds, is at most a constant
times the number of activations, those of bounded scans included.

In this proof, as in the [Setting](#setting) and Lemmas 1 and 2, a bounded scan
is a scanning source: its iterations are activations. The conditions and the
terms of the specification leave it out, so where this lemma says that a unit
or an expression has no scanning source, it can hold bounded scans, whose
activations Lemma 13 has counted wherever they occur.

For the walks of a [skip-run site](../runtime/parsing.md#skip-runs) in the
skip expression of a `#recover` that is not reused, the
[Skip Runs proof](skip-runs.md#lemma-2-cost) bounds the evaluations of the
element by $2(n + 1)$ plus a constant for each walk, and each walk is one
activation of the skip expression. That lemma is the case of this one for
skip-run sites. A reused skip is no scanning source, and Lemma 9 bounds its
walks.

**Activations carry the scanned cost.** The scanned cost of all body
evaluations, apart from the copying operations, which Lemma 6 bounds by their
activations, is at most a constant $c_3$ times
the number of activations. A step of a scanning source outside the calls that
it makes lies in one activation of the innermost scanning source of the body
that encloses it. An activation evaluates a fixed part of the grammar once:
an iteration evaluates the expression of its repetition, an operator its
operator part with its right operand, and an evaluation of a skip expression
that expression. Outside the calls that it makes and the scanning sources
nested in it, that part is a finite expression, each part of which is
evaluated at most as many times as the product of the upper bounds of the
bounded repetitions that enclose it, and each evaluation of a terminal, a
character class or `.` examines a constant amount of the input. The growth
of a cycle makes no steps of its own beyond the body evaluations of its steps,
which Lemma 5 counts. So the claim of this lemma comes down to a bound on the
number of activations, and the scanned cost of each is a constant.

**Failure after a source.** Condition 2 applies to a unit that can fail after
one of its scanning sources has examined input, a property of the grammar. It
covers every evaluation that fails after an activation in it succeeded. Let an
evaluation $w$ of a unit fail after an activation $\alpha$ inside it succeeded,
and let $f$ be the evaluation, made after $\alpha$, whose failure the
evaluations between $f$ and $w$ pass on to $w$. Let $M$ be the innermost
evaluation that contains both $\alpha$ and $f$. If $M$ is a sequence, $f$ is in
a later element than $\alpha$. If $M$ is a repetition, $f$ is in a later
iteration, which a failure makes the repetition fail only when it is required.
If $M$ is a negative lookahead, its expression contains $\alpha$ and succeeded,
and $M$ itself fails. If $M$ is a choice, a `#recover` or an Operators Rule,
which tries another part at the same position after one fails, the part that
contains $\alpha$ failed after $\alpha$, and the same argument applies inside
it. In each case, inside the unit or in the body of a rule that it calls, this
is one of the ways in which condition 2 lets a unit fail after a source. The
classifier checks condition 2 in `checkUnits`, and the three ways of being
committing in `committing`. It decides whether a unit can fail after a source in
`unitFailsAfterSource`, which refines the answer for alternatives that are not
growing, iterations and options, and returns true for every other kind of unit.

**Restarts.** An activation **succeeds** when it consumes input: its iteration,
its operator part with its right operand, or its skip expression consumes input.
An operator applied without consuming input does not succeed in this sense. An
evaluation is **abandoned** when it fails, when it is the expression of a
lookahead, or when it is the step of a growth whose result the growth drops. Let
$\alpha$ and $\alpha'$ be successful activations of one scanning source that
start at one position $q$, with $\alpha$ evaluated first, and let the innermost
evaluation $N$ that contains both not be a growth. Then $\alpha'$ is not inside
$\alpha$, and on the path from $N$ down to $\alpha$ there is a first abandoned
evaluation $w$ below $N$. Either $w$ is a dropped step of a growth, or $w$ is an
evaluation of a unit that starts at a position $s \le q$ and is committing. From
the end of $w$ to the start of $\alpha'$, the parser starts no evaluation at a
position before the start of $w$.

*Proof.* Suppose that $\alpha'$ is inside $\alpha$. Every evaluation from
$\alpha$ down to $\alpha'$ starts at $q$. The source is in the body of a rule
$R$, and an iteration of it does not contain the source again, so $\alpha'$
is in another evaluation of the body of $R$, which a call of $R$ at $q$
inside $\alpha$ starts. In that evaluation the source is reached without
consuming input, and its iteration reaches the call of $R$ without consuming
input, so that call can run before the body of $R$ consumes input: it is a
left call, and $R$ is on a left-recursion cycle. The body of $R$ can then
evaluate the source at its own start, before it consumes input and outside a
call of a rule off the cycle, so at the position where the cycle grows, which
condition 3 excludes.

So $N$ has a child $c_1$ that contains $\alpha$ and a later child $c_2$ that
contains $\alpha'$. If $N$ tries $c_2$ after $c_1$ failed, as a choice, a
`#recover` or an Operators Rule does, $c_1$ is abandoned. Otherwise $c_1$
succeeded and $N$ continues at its end, as a sequence, a repetition or an
Operators Rule does, so $\alpha'$ starts at $q \ge \mathrm{end}(c_1)$. Then
some evaluation on the path from $c_1$ down to $\alpha$ is abandoned: if none
were, the input that $\alpha$ consumes from $q$ would lie inside the input
that $c_1$ consumes, which ends at or before $q$. So $w$ exists, and the
evaluations above it on the path, up to $N$, succeed, are not the expression
of a lookahead, and are not dropped steps. Its parent goes on after it, so
$w$ is a dropped step or a unit, and it starts at a position $s \le q$, since
it contains $\alpha$.

Let $v$ be the first evaluation in progress that started before $s$ and
fails after the end of $w$ and before $\alpha'$; only such a failure returns
the parser to a position before $s$ in that time. Every evaluation that
starts after the end of $w$ and before the failure of $v$ starts at $s$ or
later, so $v$ started before the end of $w$, and since it is still in
progress after $w$ ends, it encloses $w$. It ends before $\alpha'$, so it does
not enclose $\alpha'$, and it lies on the path from $N$ to $w$, where no
evaluation fails. So there is no such $v$.

Let $w$ be a unit. It contains the scanning source of $\alpha$, or a call
that leads to it. It is the expression of a lookahead, which condition 2
checks in any case, or it failed after $\alpha$ succeeded, having examined
input, so by the paragraph above it can fail after one of its scanning
sources has examined input. Condition 2 makes it committing. $\square$

**Operators Rules.** The operators that an Operators Rule applies are a scanning
source, and each operator applied is an activation. An operator applied without
consuming input ends the expression that applies it ([Parsing
Rules](../language/operators-rule.md#parsing-rules)), so each evaluation of an
Operators Rule expression has at most one such activation, its last, which the
count of activations that do not succeed below covers. Two evaluations of
Operators Rules in sequence can each end with such an operator at one position,
and the restarts above, which need consumed input, do not apply to those
activations. The parser selects an operator at a position by trying the operator
parts there, and can try them again at the same position when an inner level
returns there because the operator it selected binds less tightly or its right
operand fails. Each such return ends an evaluation of a unit of the Operators
Rule, an operator part with its right operand, or of the right operand that a
unit contains, so the claims above treat these evaluations as those of any other
unit: an Operators Rule is a choice of its operator parts and `operand` items
when one fails and a sequence of its operands and operators when they succeed,
and its units are checked by condition 2 whether or not they can fail after a
source. When the rule nests, the return of an inner level ends the right operand
of an enclosing operator part, or an operand or operator part that ends with a
call that parses the rule again, where the inner operand ended, and the
enclosing level tries the operator parts after an operand there again, so for an
operator part after an operand those parts are a continuation ([Conditions for
Linear Cost](../runtime/parsing.md#conditions-for-linear-cost), the
continuations of a unit of an Operators Rule). A right operand that can apply an
operator contains that source at a new position, as a call of a rule of a cycle
after input is. The `skip` of an Operators Rule is a scanning source unless it
is a reused skip, and its first call is shared with that of a body that is the
Operators Rule, as [Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost) states.

What remains is to count the activations. Each evaluation of a scanning source
has at most one activation that does not succeed, its last: an iteration that
consumes no input ends its repetition, an operator applied without consuming
input ends the expression that applies it, and an evaluation of a skip
expression is one activation. The evaluations of a source are made by the body
evaluations and the activations of the sources that enclose it, a constant
number of times by each. The sources of a body nest to a depth that is a
constant of the grammar, so by induction on that depth the activations that do
not succeed are at most a constant times the body evaluations and the successful
activations. It is left to bound the successful activations that start at one
position. By the restarts above, a successful activation is repeated at a
position only after a dropped step of a growth, which Lemma 5 counts, or after a
committing unit that contains the earlier one is abandoned, and the parser does
not return before the start of that unit until the repetition. The ways in which
a unit is committing give the following cases.

1. *Disjoint first characters.* The continuation does not consume the
   character at which it and the unit part, so nothing evaluated after the
   failure starts again inside the input that the unit examined beyond its
   shared calls.
2. *Shared calls.* A call that the unit and a continuation both evaluate first
   has the same result in both, by the [memoization
   theorem](memoization.md#theorem-memoization-never-changes-the-result) when
   the rule is memoized at both calls, or by repeating only its own scan when it
   calls no rule, so the continuation pays for it at most what the unit paid. A
   rule that calls no rule repeats its scan in every continuation that evaluates
   it again at the position, so a call of it is not shared where the
   continuation reaches it after leaving a body through a call that nests at one
   end, and *Calls that call no rule* below bounds its re-evaluations at one
   position. A rule of a cycle other than a leader is memoized at a call that is
   not a left call: a growth of its cycle at the position of the call that was
   in progress would be evaluating, at that position, every rule from the
   growing leader to the caller, so each of them would have a left call of the
   next and lie on the cycle, and the call would be a left call. The call has no
   growth in its context, its result reads no growing result, and the memo
   stores it and answers the other call ([Evaluations of
   Rules](../runtime/parsing.md#evaluations-of-rules)). A literal that is not
   empty, with the same case sensitivity, that every path of both begins with is
   shared too: both consume the same characters or both fail where they begin,
   so what follows it starts at the same position in both, and the continuation
   pays for it no more than the unit paid.
3. *No source after the shared calls.* The input that the unit examines beyond
   its shared calls is bounded by Lemma 1, apart from the bounded scans that it
   evaluates there, which are no scanning sources of the conditions and whose
   activations Lemma 13 counts, repeated or not.
4. *Words that exclude.* The continuation fails, or stops examining, before the
   end of the word with which the unit began, which the unit examined at most
   once more, with the character after it.
5. *Paths that enter the growth again.* A path of a later alternative that
   [enters the growth again](../runtime/parsing.md#conditions-for-linear-cost)
   examines nothing that the alternative examined before its call of a
   leader, and makes only local cost after it; Lemma 5 counts the steps that
   the call starts.

A unit that fails only before its sources examined input gives back no more
input than its parts before the sources examined, which Lemma 1 bounds. A unit
that succeeds is not restarted by its own continuations, and a failure after
it is a failure of an enclosing unit, to which the same cases apply. The
expression of a lookahead is abandoned whether it matches or not, so it is
checked as a unit in every case.

The claims below turn these cases into bounds at one position. None of them
uses the [run records](../runtime/parsing.md#run-sites) of the engine: a
repetition that is a run site counts here as any other scanning source, so the
bounds hold whether or not a parse keeps the records.

**Direct activations.** An activation of a scanning source $\sigma$ of the
body of a rule $R$ is **direct** in the body evaluation of $R$ that contains it
with no call between them. Let $E$ be a body evaluation of $R$ that is not a
growth step. At most one successful activation of $\sigma$ that is direct in
$E$ starts at a position $q$, unless a path that enters the growth again leads
from the abandoned unit to the later one (case 5).

*Proof.* Let $\alpha$ and $\alpha'$ be two, $\alpha$ first. The innermost
evaluation $N$ that contains both lies in the body of $E$, below every call,
so it is no growth, and the restarts give the abandoned evaluation $w$ on the
path from $N$ to $\alpha$, which is no growth step, since no call lies on that
path. So $w$ is a committing unit that starts at $s \le q$, and no call made
in $w$ contains $\alpha$. From the end of $w$ until $\alpha'$, the parser
evaluates continuations of $w$ from $s$ and returns to no position before
$s$; let $\kappa$ be the one that it evaluates from $s$ the last time before
$\alpha'$, whose evaluation contains $\alpha'$. Let $t$ be the position where
the calls that $w$ and $\kappa$ share end in $w$. They end there in $\kappa$
too: every path of both that examines the input begins with them, each has the
same result in both (case 2), and each starts at the end of the one before.

Since $\alpha$ is in no call of $w$, it is not in the shared calls, so
$q \ge t$, and the input from $t$ to the end of $\alpha$ is consumed in $w$
after the shared calls: the character at $t$ is a first character of $w$ after
them. In $\kappa$, every path reaches $t$ through the shared calls, and a path
that the parser evaluates again from a position before $t$ evaluates them
again, so the parser reaches $q$ only by consuming the character at $t$ after
them: it is a first character of $\kappa$ after them too. In case 1 this
contradicts disjointness. Case 3 does not hold, since $\sigma$ is a scanning
source of $w$ outside the shared calls. In case 4 the path of $w$ that
contains $\alpha$ matched its word $W$ from $t$, and $\sigma$ follows the word,
so $q \ge t + |W|$; on that input every path of $\kappa$ fails, examines
nothing more or meets a known negative lookahead before $t + |W|$, so the
parser does not reach $q$. Only case 5 remains. $\square$

The claim covers the iterations and skip evaluations of a body and the
operators that an Operators Rule applies, which are direct in its body
evaluation, since the parse of an operand is part of the same call. It covers
a lookahead unit that matched too: the lookahead is abandoned, and the parser
continues from its start.

**Repeats.** Let $\alpha$ and $\alpha'$ be successful activations of $\sigma$
at $q$, $\alpha$ first, direct in different body evaluations of $R$, and let
no dropped step of a growth and no path that enters the growth again lead from
the abandoned unit to $\alpha'$. With $w$ and $\kappa$ as above, $\alpha$ lies
in the evaluation $e$ of a call that $w$ and $\kappa$ share, and $\alpha'$ in
an evaluation $e'$ of the same call at the same position, made by $\kappa$
after $w$ is abandoned.

*Proof.* If $\alpha$ is in none of the shared calls, the proof above applies:
$\alpha$ is in $w$ after the shared calls, inside a call of $w$ or not, so the
character at $t$ is a first character of both, and a call of $w$ after the
shared calls that contains $\alpha$ contains a scanning source, so case 3 does
not hold either. So $\alpha$ is in the shared call that starts at $t_{j-1}$ and
ends at $t_j$, with $t_{j-1} \le q < t_j \le t$. After $w$, the parser
evaluates what of $\kappa$ starts before $t$ only as the shared calls or as
expressions of $\kappa$ that begin with them, and $\alpha'$, which consumes the
character at $q$, is in an evaluation of the $j$-th shared call, the only one
that covers $q$. $\square$

So the successful activations of $\sigma$ at $q$, $\alpha_1, \alpha_2, \ldots$
in the order in which they start, outside case 5 and dropped steps, are direct
in distinct body evaluations of $R$, and for $i \ge 2$, $\alpha_i$ lies in an
evaluation $e'_i$ of a call shared by a committing unit $w_i$, made after $w_i$
is abandoned, and $\alpha_{i-1}$ in the evaluation $e_i$ of the same call at
the same position inside $w_i$. The pair $(e_i, e'_i)$ is the $i$-th **link**.

**Memoized links.** If the rule of the $i$-th link is memoized at both calls
and $e'_i$ is not in a context of growths, the rule is no leader, $e_i$ and
$e'_i$ are two of the at most six body evaluations of the rule at its position
and level that the memo allows (Lemma 3; the rule reads no variable), and at
most five such links of $\sigma$ at $q$ use one rule, position and level.

*Proof.* Outside contexts of growths, a leader grows at its first call at a
position and the memo answers every later call there, so a leader would not
evaluate $e'_i$. The memo answers a call without evaluating it, and $e'_i$,
which contains $\alpha_i$, is evaluated, and so is $e_i$, made earlier. If links
$i < k$ had $e'_k = e'_i$, then $\alpha_{k-1}$, which starts no earlier than
$\alpha_i$, would lie in $e_k$, which ended before $e'_k$ started, and so before
$\alpha_i$ started. So the links that use one rule, position and level have
distinct $e'_i$, none of which is the earliest of the evaluations. $\square$

**Calls that call no rule.** Let the rule $r$ of the links $i$ to $k$ call no
rule, read no variable and not be memoized at its calls, and let the shared
call of each link be the first call that its unit and continuation share. Then
$k - i$ is at most a constant.

*Proof.* Since $r$ calls no rule, the evaluation of a call of $r$ that
contains an activation is the innermost call evaluation that contains it, so
$e_{l+1} = e'_l$ for $i \le l < k$, and every link evaluates $r$ at one
position $p$. As the shared call is the first, $w_l$ starts examining the
input at $p$, and $\kappa_l$ reaches $e'_l$ from $p$ without consuming input.
Call the body evaluations in progress the frames. From the end of $w_l$ to
$e'_l$, the parser leaves some of the frames that enclose $w_l$, at their ends,
all at $p$, and then starts frames at $p$ through calls on the way to $e'_l$.
A frame is left once. When a frame is left at $p$, what follows, in its body,
the call of the frame inside it that was left before, or that holds $w_l$,
matched without consuming input, so that call place is followed in its caller
by what can match without consuming input.

Frames started at $p$ nest, through calls made before input is consumed, at
most $|\text{rules}|$ deep outside left-recursion cycles, whose growths and
contexts are Lemma 5's. A link that leaves no frame evaluates $r$ again in a
frame in progress or one started at $p$ inside it, at a later place of its
body; by the argument of Lemma 1, and since an iteration that starts at $p$
either consumes input or ends its repetition, the links between two that leave
frames are at most a constant. For a link $l$ that leaves frames, let $\pi_l$
be the call place through which it leaves its last frame $Y_l$ into the frame
$Z_l$ in which it then reaches $e'_l$. If more links than there are call places
leave frames, two of them, $l < l'$, have $\pi_l = \pi_{l'}$. The frames from
$Z_l$ up to $Y_{l'}$ are all left at $p$, and the places through which each
of them called the one inside it, from $\pi_l$ at $Z_l$ up to the place at
$Y_{l'}$, lead from the rule of $Y_l$ back to the rule of $Y_{l'}$, which is
the same rule, the callee of $\pi_l$. So they form a cycle of calls of rules of
one recursion, each followed in its caller by what can match without consuming
input, and $\pi_l$ nests at one end. The continuation of $w_l$ reaches $e'_l$
in what follows $\pi_l$, after leaving the body of $Y_l$, so the call is not
shared ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost)), against the choice of
the link. $\square$

An Operators Rule that nests repeats in the same way: each enclosing level tries
the operator parts after an operand again where the inner operand ended, so a
call of a rule that calls no rule that is reached there is evaluated once for
each level, as `ws` is in the calculator on `-`^n `1` followed by n spaces, and
in `x` with an operand `"(" x?` and a part `postfix ws "!"` on `(`^n followed by
n spaces. The call is not shared there, so no link passes through it. When the
rule does not nest, the levels whose evaluations end at one position are
bounded: their chain passes through right operands of operator parts, each of a
higher binding level than the part, so at most the number of binding levels in a
row, and through calls each followed in its caller by what matched without
consuming input, which repeat no rule, since a repeated rule closes a cycle of
such calls, which nests at one end and makes the rule nest.

These claims bound the links of one source at one position, memoized links by
the memo and the links of a rule that calls no rule by the cycles of nesting
calls. The claims below sum them over the positions, by charging the
activations of each second evaluation of a memoized rule to the first.

**Owners.** A **memoized evaluation** is a body evaluation of a rule made by a
call at which the rule is memoized, that is neither a growth step nor in a
context of growths, or the evaluation where the parse begins. The **owner** of
an activation is the innermost evaluation that contains it and is a memoized
evaluation, a growth step or an evaluation in a context of growths; the
evaluation where the parse begins contains every evaluation, so each activation
has one. Lemma 5 counts the activations owned by growth steps and by
evaluations in a context of growths. The variant, position, level and
combination of the values of the variables that the rule reads form the
**key** of the call of a memoized evaluation, as in the [memoization
proof](memoization.md#setting), and the key with the lookahead state of the
call its **slot**. By Lemma 3, the memoized evaluations of a slot are at most
two in the parse. The earlier is the **first** evaluation of the slot and the
later, if any, the **second**; the evaluation where the parse begins is a first
one. In a [Document](../runtime/documents.md) parse, an entry from an earlier
parse evaluates nothing, so the two evaluations of a slot are those of this
parse; what its answer does beyond that is no activation, and Lemma 12 counts
it. The **own region** of a memoized evaluation is the part of it outside
the calls of rules memoized at the call.

**Answered calls.** Once the memo answers a call outside contexts of growths,
or stores the result of one, it answers every later call with the same key and
lookahead state outside contexts of growths.

*Proof.* A call evaluates only when no entry answers it, and a store replaces
the entry that a lookup of its key finds ([Setting](#setting)). An entry stored outside a lookahead answers the calls in
both states, and one stored inside a lookahead those inside one, condition (L)
of the [memoization proof](memoization.md#setting). A store outside a
lookahead therefore makes an entry that answers the calls of both states, and
a store inside one is made only when no entry answers the calls inside, so it
replaces no entry stored outside. So no store makes an entry that answers a
call stop answering the calls of its state. Entries
stay until the end of the parse, apart from those that a stream discards before
the elements it has delivered, where no later call is made. $\square$

**Replays.** Let $M$ be the second evaluation of a slot and $M_0$ the first.
The evaluations in the own region of $M$ correspond one to one, in order, to
those in the own region of $M_0$:
each is an evaluation of the same expression at the same position, with the
same success and end, and a call of a rule memoized at the call, which either
evaluates or the memo answers, corresponds to a call with the same key.

*Proof.* By [Lemma 1 of the memoization
proof](memoization.md#lemma-1-context), the evaluation of a body that reads no
growth open at its start depends on the input, its position, level and the
values of the variables that it reads, the calls it makes, and its lookahead
state, which reaches no decision of the parser. $M$ and $M_0$ are in no context
of growths, so they read no open growth, and they are evaluations of one
variant with the same position, level, values and lookahead state. By the
[memoization theorem](memoization.md#theorem-memoization-never-changes-the-result), each
call has, whether the memo answers it or not, the success and end of the
reference parse in its state, and its value there except, inside a lookahead,
in error values, which reach no decision of the parser, (I1) of its proof. So the two bodies take the same
steps, by induction along them: each step starts in the same state in both, an
expression or a call of a rule that is not memoized at the call is evaluated in
both in the same way, by the same induction inside its body, and a call of a
rule memoized at the call has the same outcome in both. $\square$

An activation owned by $M$ lies in the own region of $M$, so it corresponds to
an activation of the same source at the same position owned by $M_0$, which
succeeds when it does. So the activations owned by memoized evaluations are at
most twice those owned by first evaluations, and the successful ones at most
twice the successful ones owned by first evaluations at each position. This
counts no body evaluation of a rule that is not memoized, so the bound does
not rest on Lemma 10, which rests on the activations.

**Own regions of second evaluations.** The own region of a second evaluation
$M$ holds no growth step and no evaluation in a context of growths.

*Proof.* A growth step is made by a call of a leader, which is memoized at
every call. A call of a leader in the own region of $M$ corresponds, by the
replays, to a call with the same key and lookahead state in $M_0$. That call is
outside contexts of growths, by the argument of the end of this proof for
$M_0$ and a growth in progress at its position, which would enclose $M_0$; and a
leader is not deferred, so the memo answered it or it grew and stored its
result. By *Answered calls*, the
memo answers the call in $M$, which starts no growth. So a growth in progress
at the position of an evaluation in the own region of $M$ encloses $M$, since
one that starts inside a call memoized at the call stays inside it. An
evaluation in a context of growths of a cycle at a position $p$ there lies with
$M$ in a step of a growth of that cycle at $p$; then $M$ starts at $p$, is
reached from that step through calls made before input is consumed and reaches
the rule of the evaluation in the same way, so the rule of $M$ is on the cycle
and $M$ is in a context of growths, which a memoized evaluation is not.
$\square$

**Memoized links in one slot lead to second evaluations.** Let the $i$-th link
of $\sigma$ at $q$ be memoized, with $e'_i$ not in a context of growths and in
the slot of $e_i$. Then the owner of $\alpha_i$ is a second evaluation.

*Proof.* By *Memoized links*, $e'_i$ is a memoized evaluation, the second of its
slot, with $e_i$ the first. Let $Y_0 = e'_i$. While $Y_j$ is a second
evaluation, with first $Y^0_j$, and not the owner of $\alpha_i$, let $Y_{j+1}$
be the outermost evaluation inside $Y_j$ that contains $\alpha_i$ and is a
memoized evaluation, a growth step or in a context of growths. It is made by a
call in the own region of $Y_j$, so by the own regions it is a memoized
evaluation, and by the replays a call with the same key and lookahead state
lies in $Y^0_j$, made before $Y_j$ started. The memo stores the result of a
slot at the latest at its second evaluation, and it stores the result of the
first unless that is the first call of the variant at the position. If the
memo had answered the earlier call, or if that call had stored its result, the
memo would answer the call of $Y_{j+1}$ (*Answered calls*). So the earlier call
made the first evaluation of the slot without storing it, and $Y_{j+1}$ is the
second. The chain ends at the owner of $\alpha_i$, which is a second
evaluation. $\square$

**Activations owned by first evaluations.** Let $\alpha_1, \alpha_2, \ldots$ be
the successful activations of $\sigma$ at $q$ outside case 5 and dropped steps,
as above. Every link is memoized at both calls or a call of a rule that calls
no rule and is not memoized at its calls, a **leaf link**. By the claim above,
an $\alpha_i$ owned by a first evaluation is $\alpha_1$, follows a leaf link,
or follows a memoized link whose second evaluation $e'_i$ is in a context of
growths or in a slot other than that of $e_i$. A memoized link passes between
slots when one of its calls is in a lookahead or discards the value of the rule
and the other is not or does not. The rule $r$ of a leaf link calls no rule, so
the body evaluation of $R$ that contains $\alpha_{i-1}$, which lies in $e_i$, is
$e_i$ itself, and $r = R$. So when $R$ calls a rule or is memoized at its calls,
and no memoized link of $\sigma$ at $q$ passes between slots or has its second
evaluation in a context of growths, at most one of the $\alpha_i$ is owned by a
first evaluation. By the replays, the successful activations of $\sigma$ at $q$
owned by memoized evaluations are then at most twice that one, and the others
are owned by growth steps or evaluations in a context of growths, or repeated
by case 5 or a dropped step, which Lemma 5 counts. Two parts remain part of
step 1 of [Open Steps](#open-steps). When $R$ calls no rule and is not memoized
at its calls, the $\alpha_i$ owned by first evaluations are at most one more
than the leaf links that lead to one, and a leaf link can leave a second
evaluation through the end of its body and reach the call again in the
evaluation around it, which can be a first one. And a memoized link between
slots, or with its second evaluation in a context of growths, can lead to an
activation owned by a first evaluation; Lemma 3 bounds the links of one rule,
position and level, but not the activations at $q$ that such links lead to
across rules and positions.

## Lemma 5 (Growth)

*Outline.* Under conditions 2, 3 and 5, and with the growths at the calls
that are not left calls counted as scanning sources by Lemma 4, the
evaluations that Lemma 3 leaves out for memoized rules number at most a
constant times $K \hat{n}$: the growth steps, and the body evaluations of
memoized rules in a context of growths, those of a rule of a cycle other than
a leader outside a growth step included. Their local and scanned cost is at
most a constant times $K(L + 1)\hat{n}$ more than Lemma 4 counts. Lemma 4
counts the activations owned by memoized evaluations, those inside a growth
step included, and this lemma those owned by growth steps and by evaluations in
a context of growths.

A growth at $p$ takes at most two more steps than the length it consumes
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)). By
condition 3, which the classifier checks in `checkGrowth`, no step evaluates a
scanning source at $p$ itself outside the left call that grows and outside
memoized calls of rules off the cycle. A step can still evaluate a scanning
source at a position after $p$ that it reaches without reading the seed, as
`[a-z]*` in the first alternative of `E = "x" [a-z]* "!" / E "+" T / "x"`, and
it does so at the same position in every step. Condition 3 does not bound
these; condition 2 does, as the following claim shows for a unit followed by a
growing unit.

**Sources at fixed offsets.** Let $u$ be a unit of a body of the cycle that is
not a growing unit, can fail after one of its scanning sources has examined
input, and has among its continuations a growing unit $g$ of the leader that
grows at $p$, and let $u$ and $g$ share no call. In every step of the growth at
$p$ except the first, the one whose seed is empty and, when $u$ is committing
for $g$ by its words, a constant number of others, an evaluation of $u$ at $p$
examines at most a constant amount of the input.

The classifier finds no shared call for such a pair. `committing` removes the
shared calls with `strip`, which removes a call or literal only when
`commonLead` finds it first on every path of both, directly or, through
`deepLeads`, inside the bodies of shareable calls. A path of $g$ begins with a
left call that reaches a leader of the cycle. `leads` reports the call of the
leader, which is memoized and so shareable, while $u$ does not begin with that
call, as it is not a growing unit; and looking through the call, the body of the
leader reaches the leader again by left calls without examining input, so the
search ends only when its budget runs out, with a lead that is no call. A pair
that shares a call under the definition of [Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost) is left to step 3 of
[Open Steps](#open-steps).

*Proof.* As a continuation of $u$, $g$ examines the input from $p$ through the
seed. The seeds of the steps are the results of the steps before them and grow
longer at each step, so at most one of them is empty. In any other step after
the first, the seed consumed the character at $p$ on a path of $g$ through the
seed, so that character is among the first characters of $g$. By condition 2,
$u$ is committing for $g$, in one of three ways. If the first characters of $u$
and $g$ are disjoint, they do not include that character, so $u$ cannot consume
it, nor can a lookahead that it evaluates before it consumes input, and $u$
examines only the slot at $p$. If $u$ has no scanning source, it examines only
what Lemma 1 bounds. If the words of $u$ exclude $g$, then $u$ matches its word
$w$ at $p$ only in a step whose seed ends before the end of $w$, since a path of
$g$ through a longer seed would reach the end of $w$ on that input; there are at
most $|w|$ such steps, and in every other step $u$ fails within its word, after
examining at most the word and the character after it. $\square$

So outside a constant number of steps of each growth, such a unit costs a
constant at $p$, and in those steps it costs what an
evaluation of it at $p$ costs, which Lemma 4 charges as for any evaluation of a
body.

The growing units of a step examine the input from the end of the seed. The
input from the end of one seed to the end of the next is examined by one
step, and what a step examines beyond the end of its own result is the
failure of a growing unit after a scanning source, which condition 2 checks.
A growth is itself a scanning source at each call of a rule of its cycle
other than a left call, so its repetition at another position is counted by
Lemma 4.

After a growing unit fails in a step, a later growing unit of the step can
succeed. The step then ends with a longer result, and the next step reads it
as the seed and evaluates the growing units again from its end, which can lie
inside the input that the failed unit examined. Condition 2 therefore checks
the failed unit against the later growing unit continued, at the end of the
rule body, through every call of the rule, the reads of the seed included, and
not only through the calls that start the growth, which continue only after
the growth ends. With that continuation the cases of Lemma 4 apply to the
failed unit as to any other: what the parser evaluates next, in this step or
the next, includes everything that can examine again the input that the unit
examined.

A growth can also start at a call of a rule M of the cycle other than a
leader that is not a left call: M's body reaches a leader through left calls,
and that leader grows at the position. When the growth ends, the parser
continues in M's body after the call of the leader and then after the call of
M, so condition 2 checks the growing units against those paths. The same holds
for what follows the growth at the end of any body of the cycle that a step
completes: the call of the leader in M's body, or in the body of a rule that
M reaches through left calls, is a seed read in the steps of other growths but
the start of this one, so the end of the leader's body continues after it as
after any call that starts a growth. Outside the steps, the call of M evaluates
M's growing units once more, and those of the rules that it reaches through left
calls, after the leader has grown, as alternatives that are not growing, which
condition 2 checks as any other unit. In the first step of a leader R, every
left call of R reads the failed seed and fails without examining input, so only
the paths of a unit of R that evaluate something else first can examine input in
that step.

A path of a later alternative that enters the growth again calls a leader R
of the cycle at the position $q$ where the failed alternative A started,
before it examines input. Two cases bound the call.

- *A growth of the cycle is in progress there.* At $q$, the call reads the
  growing result and makes no further call, by (G1) as used in
  [Lemma 1 of the left-recursion proof](left-recursion.md#lemma-1-chains-at-one-position),
  so it costs a constant. By
  [Lemma 2 of the memoization proof](memoization.md#lemma-2-reading-an-open-growth)
  only a call at $q$ of a rule of the cycle reads that growth, and this call
  is one.
- *No growth of the cycle is in progress there.* If the growth of R at $q$
  has ended, the memo answers the call with its result, which read no growth
  open at the start of the call
  ([Lemma 3 of the memoization proof](memoization.md#lemma-3-answering-from-a-sound-entry)),
  at a constant cost. Otherwise the call starts that growth. R is memoized,
  so the growth happens once for each position, level and combination of the
  values of the variables that R reads
  ([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)), and it
  takes at most two
  more steps than the length it consumes
  ([Lemma 2 of the left-recursion proof](left-recursion.md#lemma-2-steps-of-a-growth)):
  these are the steps that this lemma counts for every growth. Growths that
  start at many positions, each over a long remainder, are not bounded here:
  each starts at a call of a rule of the cycle that is not a left call, which
  is a scanning source, so condition 2 checks the unit that contains it.

Each step evaluates the bodies of the cycle at $q$ again, A among them when
its rule is reached. A makes no left call of the cycle and evaluates its
scanning sources only inside calls that are memoized at the call and read no
variable. The bodies of the cycle evaluate every scanning source in the same
way, at the positions that are the same in each step, before a left call of a
leader that they always evaluate. So from the second step on, the memo answers
those calls, and a step evaluates A again at a constant cost. What follows the
call of R contains no scanning source up to the end of the parse, so its cost
is local cost of the body evaluations that it continues, which Lemma 1 counts.
Leaving these paths out of condition 2 therefore adds a constant for each
evaluation of A, beyond the steps that are counted here.

A leader R called at $p$ in a step of the growth of another leader S of its
cycle grows there in the context of that growth. A step of R from a result
that read no growing result of S, or the same one, is the step that an earlier
growth of R took from it: equal results of a step share their identity, the
steps are reused, and a run of steps that read only R's own result is taken at
once, so these steps cost a constant for each step of S beyond their first
evaluation. A growth of R from a result that includes the growing result of S
is new in each step of S. By condition 5 either the step of S keeps the result
of R, and the input that R examines past the end of the seed of S is the input
that this step consumes, or R grows past that end only where the growth of S
stops, which happens once per growth.

## Lemma 6 (Values and Copies)

Under conditions 1, 2 and 4, the size of the values that actions and
predicates build is at most a constant times $K(L + 1)\hat{n}$, given the
count of activations of Lemma 4 (step 1 of [Open Steps](#open-steps)).

For a body evaluation $e$ of a rule $R$ whose match succeeds, the **own span**
of $e$ is the input that its match consumes outside the matches of the calls
that it makes, directly or through other rules, of the rules of a recursion
with $R$. The own spans of two matches of rules of one recursion, one nested
in the other, are disjoint, since the inner match is part of a call that the
outer one makes on the recursion.

*Proof.* By [Cost](../runtime/parsing.md#cost) and the definition of a
[copying operation](../runtime/parsing.md#conditions-for-linear-cost), an
operation of an action or a predicate that is not a copying operation builds
a value of constant size, and a copying operation costs at most a constant
times the size that it copies or examines, plus a constant; taking the text
of a node is no copying operation. The operations of constant cost, and the
copying operations that are no
[scanning source](../runtime/parsing.md#conditions-for-linear-cost), whose
operands are bounded in size by a constant or are values of variables, are
part of the local cost, which Lemma 1 bounds with the factor $L + 1$. What
remains are the copying operations that are scanning sources. Let one of them
be in the action or a predicate of $e$.

*Outside lambdas.* By condition 4, the size that the operation copies does not
include the value of a call on a recursion with $R$, nor a fold accumulator, nor
a value whose size can come from a capture made in a positive lookahead. Its
operand is then built from literals, values of variables and values of matches
that hold no call on the recursion: captures and calls of $e$ that hold none, as
the text of a capture around such a call would, and parts of the values of calls
on the recursion that hold none, which the operand reaches through members, as
`$z.$y` reaches the capture `y` of `F` in `E = "(" z:F ")" => text($z.$y) + "!"`
with `F = y:"b"* E?`. A call of a rule outside the recursion does not reach the
recursion again, since its rule would be on it. So each match that the operand
reads lies in the own span of $e$, or in the own span of a body evaluation of a
rule of the recursion nested in $e$ that a chain of calls on the recursion
reaches, each call made by the evaluation before it outside its other calls on
the recursion; the chain is no longer than the members that the operand takes, a
constant $D$ of the grammar. These evaluations, $e$ included, are the
**evaluations that the operation reads**, and their own spans its **read
input**. Let $\rho$ be one more than the size of the read input plus the number
of iterations of the repetitions of these evaluations, as the list of the
repetition `es:E*` in a body of the recursion has one element for each call on
the recursion that it makes. The value of a match need not lie inside it:
`L = &(x:W) => $x` consumes nothing and returns the rest of the input, so that
`y:L` is an empty match whose value is as large as the input after it. Condition
4 excludes such a value by its size, whatever the match consumes, and the
definition of an [own](../runtime/parsing.md#conditions-for-linear-cost) value
excludes it too, since a capture in a positive lookahead is part of the value of
`L`. By *Top sizes* in Lemma 11, the operation copies, of each value that it
reads other than the value of a variable, at most a constant times $\rho$,
whether the value is own or not, as an element of a list that `map` builds, or a
string that a lambda builds. The operation reads a constant number of such
values and of values of variables, so it copies at most a constant times
$\rho + L$. A value read through a lambda over a bounded list is copied a
constant number of times, by Lemma 7, so the same bound holds for the copies in
such a lambda, with a larger constant.

*Inside lambdas over lists that are not bounded.* By condition 4, such a lambda
copies nothing from outside it that is not bounded in size by a constant, nor
its fold accumulator, so each of its copies copies a constant or parts of the
elements of the lists of the lambdas around it. Over the evaluations of the
lambda in one evaluation of the action, the elements that it copies add up to
the size of its list times the numbers of elements of the lists of the lambdas
around it, by the [size
degrees](../runtime/parsing.md#conditions-for-linear-cost) that condition 4
adds; the condition admits the sum only when it is a size degree of at most 1.
By *Sizes in the read input* in Lemma 11, the bound of a size degree $d$ holds
with $\rho$ in place of $n$, for values that read no variable, so the sum is at
most a constant times $\rho$ when that degree is at most 1, as the class Linear
requires, whether the list is own or built by `map` or a lambda; without that
admission it is not, as in a lambda over a list of calls on the recursion that
maps the same list again for each element. When the list is own and the lambdas
read only its elements, the degree leaves out the number of its elements, and
those are own captures of a repetition, distinct parts of the read input, so the
sum is at most a constant times $\rho$ as well. A list built from the values of
variables adds at most a constant times $L$. A fold over such a list costs, by
the sums after the proof of Lemma 7, at most a constant times $\rho^d$ with $d$
the degree of its cost, which the class Linear admits only when it is at most 1;
then each term of its cost is a constant or the size of the element of the step,
since a term that reads a quantity of the first parameter has degree at least 2
there, every quantity having degree at least 1 and the number of steps of a list
that is not bounded degree at least 1, so its steps cost at most a constant
times one more than the size of its list, which is bounded as above.

*Sum.* Each such operation costs, in $e$, at most a constant times $\rho + L$.
The term $L + 1$ is paid once for each body evaluation and each activation,
which Lemmas 3 to 5 and 10 count. The iterations in $\rho$ are activations of
the repetitions of the evaluations that the operation reads, each charged a
constant. The read input is the input that the operation examines: the operation
is a scanning source that reads, once, the input of the own spans that it reads,
and condition 2 treats a later alternative that starts inside that input as one
that starts inside the input of a repetition. Lemma 4 counts each unit of that
input as an activation of the operation. The own spans of the evaluations of one
recursion, one nested in the other, are disjoint, and an evaluation is read by
the operations of itself and of at most $D$ evaluations of the recursion around
it, those whose chains reach it, so each unit of the read input and each
iteration is charged by the copies of at most $D + 1$ evaluations that the parse
keeps. An evaluation that performs the operation and is then abandoned is
evaluated again only as Lemma 4 allows for any activation that a failure
repeats: condition 2 classifies a unit that can fail after the copy as it does
one that can fail after a repetition, so that `(A "q" / "a")*` with
`A = x:W => text($x) + "!"` is classified polynomial. So the copies cost at most
a constant times the activations of Lemma 4 plus $L + 1$ times the body
evaluations and the activations, which is the claim. $\square$

A unit of input is charged only a constant number of times outside the
repetitions that a failure makes. The successful matches that hold it and are
kept in the tree of the parse are nested in one another. Of those of the rules
of one recursion, only the innermost holds the unit in its own span, it and at
most $D$ of those around it read it, and the recursions around a unit, each a
strongly connected component of the call graph, are no more than the components
of the grammar. The other evaluations whose own spans hold the unit are those
that a failure or a lookahead abandons, which Lemma 4 counts. A capture in a
positive lookahead would break this: its value lies outside the own span, so
that `A = "a" &(x:W) => text($x) + "!"` in `A*` copies the rest of the input at
each position, the reason why condition 4 excludes it.

A copy of the value of a variable costs at most $L$. Condition 1 keeps the
value of a variable from growing from its own earlier value, or, by Lemma 8,
keeps it in a set of constant size, so the number of combinations at a
position is $K$, a constant when every binding is a literal or binds a
variable with finite values.

Taking the text of a node is not a copy: by the
[Constant-Cost Text proof](constant-cost-text.md#lemma-1-text-is-a-constant-step)
it is a step of constant cost, apart from work of at most a constant times
$\hat{n}$ in the whole parse, which the constant of the theorem absorbs. A
stream pays the length of each text besides, as
[Lemma 4 of that proof](constant-cost-text.md#lemma-4-streams) states.

## Lemma 7 (Lambdas Over Bounded Lists)

Let a call of `map`, `filter`, `foldl` or `foldr` satisfy condition 4 and
have a [bounded](../runtime/parsing.md#conditions-for-linear-cost) list, with at most
$m$ elements, where $m$ is a constant. In one evaluation of the call, let $I$
be the size of the initial value of a fold ($I = 0$ otherwise), $E$ the total
size of the elements, and $S$ the total size of the values that the lambda
reads other than its parameters, each measured as condition 4 measures what a
copy of it copies. Then the call evaluates the lambda at most
$m$ times, the first parameter of a fold has size at most $a^m (I + E + S + d)$,
and the copying operations of the lambda cost at most $c (I + E + S + 1)$, for
constants $a$, $c$ and $d$.

*Proof.* By [Built-In Functions](../language/actions.md#built-in-functions),
the call evaluates its lambda once for each element of the list, so at most
$m$ times. Let $r$ be the number of references to parameters, captures and
variables in the lambda, and $d$ the total size of its literals; both are
constants. A value that the lambda builds is made of the values that it
references and of its literals, so its size is at most $r$ times the largest
value that it references, plus $d$. This holds as well when the lambda
contains a call with a lambda of its own, by induction on the nesting of
lambdas: over a bounded list, by this lemma for the inner call, with larger
constants; over a list that is not bounded, the inner lambda copies nothing
from outside it that is not bounded by a constant, nor its fold accumulator,
by condition 4, so the value of the inner call is at most a constant times the
sizes of its list and its initial value, which the lambda references. One evaluation of the lambda references
the first parameter, an element and the values counted in $S$, so with $A$ the
size of the first parameter and $B = E + S + d$, its value has size at most
$r(A + B) + d \le r A + (r + 1) B$, and its copying operations, each of which
costs at most a constant times the sizes that it copies, cost at most a
constant times $A + B$.

In a fold, let $A_j$ be the size of the first parameter after $j$ steps, so
$A_0 = I$ and $A_{j+1} \le r A_j + (r + 1) B$. With $a = 2r + 1$, induction
gives $A_j \le a^j (I + B)$: the case $j = 0$ holds, and
$A_{j+1} \le r a^j (I + B) + (r + 1) B \le (2r + 1) a^j (I + B)$. So
$A_j \le a^m (I + B)$ for $j \le m$. The first parameter of a `map` or a
`filter` is an element, of size at most $E$. Summing the copying operations
over at most $m$ evaluations gives at most a constant times
$m (a^m (I + B) + B)$, which is $c (I + E + S + 1)$ for a constant $c$.
$\square$

When the [step matrix](../runtime/parsing.md#conditions-for-linear-cost) of
the fold cannot double the first parameter, the factor $a^m$ shrinks to a
polynomial. Let $n$ be the size of the input. Each value that the lambda
reads from outside the fold has size at most a constant times $n^e$, with $e$
its [size degree](../runtime/parsing.md#conditions-for-linear-cost), by
*Size degrees* below; so do the three quantities of the initial
value, with degrees $e_S$, $e_M$ and $e_N$, and those of the list, which has
$m$ elements, at most a constant times $n^{l_N}$, of sizes $Y_j$ that add up
to at most a constant times $n^{l_S}$, each at most a constant times
$n^{l_M}$. Let
$v_j = (S_j, M_j, N_j)$ be the size, the size of the
largest element and the number of elements of the first parameter after $j$
steps, where a list of $N$ elements of sizes $E_i$ has size
$1 + N + \sum_i E_i$. Each rule of condition 4 bounds a quantity of a value by
a sum of terms $c \, q \, n^a Y_j^b$, with $c$ a constant and $q$ a quantity
of the values that it is built from, or 1: a list, a tuple, `+` and `concat`
hold their operands, and a concatenation is no larger than the sum of its
operands; an element taken by an index, which is an int, is at most the
largest element; a member is at most its value; a `filter` holds at most its
list; a `map` holds one value of its lambda, plus one, for each element of its
list, and a term $\alpha E_i + \beta$ of the lambda in the size $E_i$ of the
element sums over the elements to at most $\alpha S + (\beta - \alpha) N$,
since $\sum_i E_i \le S - N$; an inner fold that joins its own first
parameter once holds its initial value and, once each, the values that its
steps join, which sum over its list in the same way; and a term that the
rules do not follow, or a product of two quantities of the first parameter, is
bounded by many copies of everything that it reads. Where a value can be
either of two values, as an element of a list built from several, the larger
is at most their sum. So each quantity of $v_{j+1}$ is at most such a sum over
the quantities of $v_j$.

Let $\sigma(0) = l_N$ and $\sigma(b) = \min(b \, l_S, l_N + b \, l_M)$ for
$b \ge 1$: the sum of $Y_j^b$ over the steps is at most a constant times
$n^{\sigma(b)}$, since there are $m$ steps, $\sum_j Y_j^b \le (\sum_j Y_j)^b$
and $\sum_j Y_j^b \le m \max_j Y_j^b$. Give each quantity $x$ a degree $d_x$,
and the constant 1 the degree 0, in the
graph with an edge from $x$ to $q$ for each term of $x$ that reads $q$. By the
condition, each strongly connected component with a cycle is a single cycle
whose terms have coefficient 1 and no factor $n$ or $Y_j$. For $x$ on no
cycle, $d_x$ is the largest of 1, $e_x$ and $d_q + a + b \, l_M$ over its
terms; for a component $C$ with a cycle, every member has the largest of 1,
the $e_x$ of its members and $d_q + a + \sigma(b)$ over the terms of its
members that read outside $C$.
We show by induction on the components, from those with no edges out, that
$v_j(x) \le c_x n^{d_x}$ for every $j \le m$. For $x$ on no cycle,
$v_0(x) \le c \, n^{e_x}$, and $v_{j+1}(x)$ is a sum of terms
$c \, q \, n^a Y_j^b \le c \, c_q n^{d_q + a + b \, l_M}$ over quantities of
lower components. For a component $C$ with a cycle, let $u_j$ be the largest
of $v_j(x)$ over $x \in C$. Each member has one term in $C$, which reads a
member with coefficient 1, so $u_{j+1} \le u_j + t_j$, where $t_j$ is the sum
of the other terms, and $u_m \le u_0 + \sum_{j < m} t_j$. A term sums over the
steps to at most
$c \, c_q n^{d_q + a} \sum_j Y_j^b \le c' n^{d_q + a + \sigma(b)}$.
Over a bounded list, $m$
is a constant, and the same sums over $m$ steps replace the factor $a^m$ of
the proof above by a polynomial in $m$.

The copying operations of one step cost at most a sum of such terms, each a
product of quantities of $v_j$ times $n^a Y_j^b$: a string copy costs its
characters and a list copy its elements, at most the sizes that condition 4
bounds, a comparison of values that are not ints and `len` of a string
examine their operands, and a fold inside the lambda copies its own first parameter on each
of its steps, at most its number of steps times the size of its value. By the
same sums, the steps of a fold over a list that is not bounded cost at most a
constant times $n^d$, where $d$ is the largest of
$\sum_q d_q + a + \sigma(b)$ over the terms of the cost. This is the degree
that the classifier reports when it is 3 or more; a cost of degree 2 is that
of a copy of the first parameter in each step, which condition 4 reports with
degree 2, or of a value read from outside, for which condition 4 fails
without a witness, so that the class is not linear. When the step matrix fails the
condition, no bound is claimed: the classifier reports the fold exponential,
which over-approximates it.

A fold inside the lambda of a call over a list that is not bounded runs once
for each element of that list, at most a constant times $n^{l}$ times, with
$l$ the size degree of its number of elements, so each such lambda around it
multiplies its cost, and the cost of each copy of its first parameter, by
$n^{l}$: the classifier adds $l$ to both degrees for each of them. When the
list of the fold reads only elements of the innermost of them, those are own
captures of a repetition, and the list has size degree 1, the list of each
run is built from one element by forms of degree 1 in its size, so it has
size $\ell_i$ at most a constant times one more than the input that the
element consumes, and $\sum_i \ell_i \le c \, n$, since the captures are
distinct parts of the input. An element is own only when no capture in a
positive lookahead is part of it and every action in it builds an own value,
through the bodies of the rules it calls: a capture in `&(...)` stays in
effect over input that the element does not consume, so the elements can
overlap, and an action can build a value larger than its input, such as that
of a string variable that the caller binds, the same for every element. A
run takes at most $\ell_i$ steps, and every term of its cost sums over the
steps, so the run costs at most a constant times $\ell_i n^{d - 1}$: a term
with $b = 0$ sums over at most $\ell_i$ steps, and one with $b \ge 1$ to at
most $(\sum_j Y_j)^b \le \ell_i n^{b - 1}$. Summed over the runs, the cost is
at most a constant times $n^d$, so the innermost lambda adds nothing. For the
elements of any other list, as one that holds a capture once for each element
of another, or one whose elements capture in a lookahead or are built by an
action that is not own, the sizes need not add up to the input, and the
lambda adds its degree.

*Size degrees.* The proof of the claim above does not use what follows, which
proves the size degrees of
[Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)
that the polynomial degrees of the paragraphs after that proof read, and that
Lemma 6 cites. Sizes are measured as that definition measures them: a list
has one more than its number of elements plus the sizes of its elements, and
a node the size of its text or of the values of the captures in it, whichever
is larger. For a match $m$, let $|m|$ be the input that it consumes.

**Matches nested in one match.** Let $S$ be a set of rules and $N_S(m)$ the
number of matches of rules of $S$ in the parse tree of $m$, $m$ included if
its rule is in $S$. Then $N_S(m) \le h(|m| + 1)$ for a constant $h$. Take the
tree whose nodes are the matches of rules inside $m$ and the iterations of
repetitions inside them, each node under the one whose evaluation made it.
Two nodes are nested or disjoint, so the distinct ranges of the nodes that
consume input form a laminar family of nonempty intervals of $|m|$
positions, which has at most $2|m| - 1$ members. Nodes with the same range
are nested in one another. In a chain of them, each match is a call at the
position of the match around it before that match consumes input, and
between two matches of the chain lie at most as many iterations as one body
nests repetitions. A chain with more matches than there are rules repeats a
rule, so it contains a cycle of left calls, which passes through a leader of
its cycle; the leader grows, each step of its growth consumes more input
than the step before it, and so the leader cannot have the same range as a
match of its own rule nested in it. So a constant number of nodes share a
range. A node has a constant number of children that consume no input, since
the expression that it evaluates once makes a constant number of calls
outside repetitions, and a repetition stops at an iteration that consumes
none. The nodes that consume no input nested in such a child have its empty
range, so they are a constant number too. Together this gives the constant
$h$.

**Own values.** Let $\mathcal{S}$ be a strongly connected component of the
call graph, and let every rule of $\mathcal{S}$, and every action in it,
build an own value as the definition states. We show by induction on the
components, from those that call no other, that the value of a match $m$ of
a rule of $\mathcal{S}$ has size at most
$\alpha_{\mathcal{S}}(|m| + N_{\mathcal{S}}(m))$, for a constant
$\alpha_{\mathcal{S}}$; by the bound on $N$, this is at most a constant times
$|m| + 1$. Write the own span of $m$
for the input that it consumes outside the matches of the calls on
$\mathcal{S}$ that it makes, as in Lemma 6. The value of $m$ is built by the
forms of the definition from literals, ints and Booleans, of constant size,
from captures of $m$, and from the values of the calls that its body makes:

- A capture that is not in a positive lookahead, and a call, is a match inside
  $m$, and the value of a call is own only when the value of its rule is,
  which no capture in a positive lookahead is part of, so that no call returns
  input outside its match; a member and an element are parts of one, and a
  `filter` keeps at most its list. A match of a rule outside $\mathcal{S}$
  lies in the own span, since a call on $\mathcal{S}$ inside it would put its
  rule in $\mathcal{S}$; by the induction its value is at most a constant
  times its input plus one, and distinct captures of one expression hold
  disjoint parts of the input. A capture can be read several times, at most a
  constant number of times by the grammar, which multiplies it by a constant;
  the chain of such readings through rules outside $\mathcal{S}$ is no longer
  than the grammar, so the constants do not compound. The list of a repetition
  of such matches has one element for each iteration, at most one more than
  the input of the repetition, since only the last iteration can consume none.
- `+`, `text`, `list`, `concat` and constructors add the sizes of their parts
  and a constant. A `map` over an own list whose lambda reads nothing from
  outside it that is not bounded by a constant adds, for each element, at most
  a constant times its size plus a constant, which sum over the elements to at
  most a constant times the size of the list; a fold over such a list that
  reads its first parameter at most once adds as much to its initial value.
- The value of a call on $\mathcal{S}$, read at most once and not as the list
  of a lambda, which would add to each of its elements on every level, is at
  most $\alpha_{\mathcal{S}}(|m'| + N_{\mathcal{S}}(m'))$ for that call $m'$
  by the induction on the depth of the tree. A lambda over the captures of a
  repetition that reads each member of each of its other parameters at most
  once, distinct members being disjoint parts of a match, holds each part of
  each element, a distinct match, at most once, and the calls on
  $\mathcal{S}$ in it with them, so it adds each of their values at most once
  as well.

So the value of $m$ is at most a constant $\beta$ times one more than its own
span, plus the sum of $\alpha_{\mathcal{S}}(|m'| + N_{\mathcal{S}}(m'))$ over
the calls $m'$ on $\mathcal{S}$ that it holds, which are disjoint matches
inside $m$. With $\alpha_{\mathcal{S}} \ge \beta$, and since $|m|$ is the own
span plus the sum of the $|m'|$, and $N_{\mathcal{S}}(m)$ is one plus the sum
of the $N_{\mathcal{S}}(m')$, the value is at most
$\alpha_{\mathcal{S}}(|m| + N_{\mathcal{S}}(m))$. Distinct matches that are
not nested, such as the iterations of a repetition, have disjoint inputs, so
their own values add up to at most a constant times $\hat{n}$. A capture in a
positive lookahead is no part of the match, and its input is outside the
input that the match consumes, so it is excluded, also when a rule that
consumes nothing returns it; so is the value of a variable, whose size $L$
does not depend on the match.

**Size degrees above 1.** A value built from values of size degrees $e_i$ by
the forms of condition 4 is bounded by the rules after the proof above with
$n^{e_i}$ in place of the quantities. The list of a repetition of elements of
degree $d$ has at most $\hat{n}$ elements, so its size has degree $d + 1$.
Let the rules of $\mathcal{S}$ have values that are not own, and let each of
them hold the value of each call on $\mathcal{S}$ at most once, as a capture
not made in a positive lookahead that is read once, outside lambdas or as the
list of such a lambda over a repetition, and let one level, the value of a
match apart from the values of the calls on $\mathcal{S}$ that it holds, have
size at most a constant times $n^d$. The same induction, with $n^d$ in place
of $\beta$ times one more than the own span, bounds the value of $m$ by a
constant times $n^d N_{\mathcal{S}}(m)$, which is at most a constant times
$n^d \hat{n}$; so its size has degree $d + 1$. Two captures in a positive
lookahead can hold the same match, which the bound on $N$ does not count, and
are excluded.

The copies of such a call are therefore those of a constant number of copies,
in the context of the call, of the initial value, the list and the values from
outside the lambda. This is how condition 4 checks them: they repeat once for
each element only when a lambda around the call is over a list that is not
bounded, and the first parameter of a fold includes what the initial value
and the value of the lambda include, so that a copy of it that includes the
value of a call on the recursion is checked as such a copy. The lemma itself
needs $a^m$ only to be a constant; the restriction to folds that cannot double
the first parameter keeps the constant of the class polynomial in $m$.

## Lemma 8 (Finite Values)

Let $v$ be a variable with
[finite values](../runtime/parsing.md#conditions-for-linear-cost) and $S_v$
its set of values. The variables with finite values are the `bool` variables,
the `int` variables whose value sets are finite, and the variables whose
initial value and bindings are all constants, each with at most 65536 values,
taken from the smallest set up while the product of the sizes of their sets is
at most $2^{32}$. A `bool` has as its set its initial value and the values that
its bindings can bind: a constant binding adds its value, a binding that fails
to evaluate adds none, and any other adds both. Then $v$ holds a value of $S_v$
at every call and at every evaluation of a binding in every parse, and the
variables with finite values contribute at most the product of their $|S_v|$,
a constant, to $K$.

*Proof.* For a `bool`, the initial value is in $S_v$, and every binding of
$v$ that succeeds binds its constant value, which is in $S_v$, or is one whose
value is not constant, in which case $S_v$ holds both values; so every value
that $v$ holds is in $S_v$. For a variable bound only to constants, each
constant term evaluates to one value or is an error and binds nothing, so the
distinct terms, with the initial value, bound the values that it holds. For an
`int`, by induction on the steps of the parse. By
[Variables](../language/predicates.md#variables), the parse starts with $v$
bound to its initial value, which is in $S_v$, and afterwards a lookup of $v$
gives the nearest binding of $v$ that has not been undone, or the initial
value. Bindings made in a called rule are not visible to its caller, and
backtracking undoes bindings, so every value that a lookup gives is the
initial value or a value that an evaluation of a binding bound earlier.

The guard of a binding $b$ of $v$ holds the tests next to it with only
predicates between, which consume no input and call no rule. Let $b$ bind a value $y$ and let
a test after $b$ fail on $y$: the sequence that holds $b$ and the test fails,
and backtracking undoes $b$ before anything else is evaluated, so no call and
no binding reads $y$ from $b$. A pragma such as `#recover` around the test
could turn the failure into a success that keeps $b$, which is why a test
under a pragma is not part of a guard; `@` attributes change only diagnostics.
A test before $b$ that evaluates the value $e$ of $b$, written alike, reads the
same variables and captures as $b$, since no binding lies between them and
predicates capture nothing, and every term of the action language is a
function of the values that it reads, so it evaluates $e$ to the value that
$b$ binds. So a value that $b$ leaves bound at a call or a binding is one on
which every test of its guard succeeds, which lies within the bounds of the
comparisons of the guard. The values bound between $b$ and a test after it
are read only by predicates, which the claim does not count.

Let a binding $b$ of $v$ be evaluated, and let $x$ be the value of $v$ that
it reads. We show that $x$ is one of the values that reach $b$, computed from
$S_v$. Let $F$ be the outermost sequence that $b$ is in through groups,
options, choices, captures, discards and atomic expressions only. In the
evaluation of the rule body that evaluates $b$, $F$ starts with a value $x_0$
of $v$ that is in $S_v$ by the induction hypothesis, and the earlier elements
of $F$ and of the sequences inside it that hold $b$, then the earlier
statements of the block of $b$, have each been evaluated once on this path
and succeeded, in order, or $b$ would not be evaluated. Each step maps a set
that holds the value of $v$ before it to a set that holds the value after it:

- a binding of $v$ leaves a value that its evaluation gives from the value
  before it and that its guard passes, which the step computes as for $b$
  below;
- a predicate that reads only $v$ and literals succeeded, so the value passes
  it; the step keeps every value on which its evaluation is true or not known;
- a choice succeeds through one of its alternatives and an option through its
  expression or by matching nothing, so the value after it is in the union of
  the values after its paths, and `@` attributes do not change matching;
- a negative lookahead undoes its bindings, and a call returns with the
  value before it, since bindings in a called rule are not visible to its
  caller;
- any other element that binds $v$ leaves a value that $v$ holds at some step
  of the parse after a binding and its guard, which is in $S_v$ by the
  induction hypothesis; an element that does not bind $v$ keeps it.

So $x$ reaches $b$. When the classifier evaluates $b$, it reads only $v$,
literals and operands `len($c)`, and the length of the value of each capture
$c$ is at most its bound, since the value is the text of an expression that
matches at most that many characters, a list or tuple with at most that many
elements, or the value of a rule whose action gives such a value. A sequence
builds a tuple of the values of its items, in which a plain group of a
sequence is replaced by the elements of its own tuple, so the tuple has at
most the number of items counted with those groups spliced in; a sequence
whose value is that of a single item has the bound of that item. The value of
$b$ is therefore determined by $x$ and lengths within the bounds, by the rules
of [Operators and Functions](../language/actions.md#operators-and-functions),
for ints within $2^{53}$ in magnitude, which every backend computes alike, or
the evaluation is an error and binds nothing. Otherwise the value lies in the
range of $e$, which holds every value that a literal, $x$, a length, which is
not negative, and their sums, differences, negations and products can give,
and it passes the guard only within the bounds of its comparisons. Either
way, $S_v$ holds every value that $b$ leaves bound from each value that reaches
it.

The values of the variables that memoized rules read at a position are
therefore drawn, for the variables with finite values, from the product of
their sets, so they contribute at most the product of the $|S_v|$ to the
number of combinations $K$. $\square$

The classification reports the product as its bound on $K$ for the class
Linear, where every variable that a binding binds has finite values or is
bound only to constants, and a variable bound only to constants that the cap
on the product leaves out makes the class Linear Given an Environment Bound.
The bound of 65536 values of one variable keeps $S_v$ cheap to compute, as do
the bounds of 256 for a length and of 4096 combinations of lengths; the bound
of $2^{32}$ on the product keeps it exact, and a larger $K$ would promise
nothing that a parse can use. The proof needs only that the bounds are
finite. The values that reach a binding depend on the order of the elements
before it: a test after the binding, or after a call that follows it, has not
been evaluated when the binding reads $v$, so it filters nothing that the
binding reads.

## Lemma 9 (Reused Skips)

The walk cost is at most a constant times $\hat{n}$ plus a constant times the
number of evaluations of the skip expressions of reused skips, which is at most
a constant times the number of body evaluations, growth steps included, plus
the number of activations.

*Proof.* Fix a site $\sigma$ of a reused skip and a lookahead state $\lambda$.
The expression of $\sigma$ has no repetition without an upper bound, no rule
call, no predicate, no `#recover` and no Operators Rule, so it is a finite
expression of terminals, character classes, `.`, lookaheads, choices,
options, sequences and bounded repetitions. Each of its evaluations makes a
constant number of steps, examines a constant amount of the input, however
many characters it consumes, and builds no value, since a skip expression has
no captures. The expectations that it can record are among a constant number
of terminals. By [Lemma 2](skip-runs.md#lemma-2-cost) of the Skip Runs proof,
in a parse with $W_{\sigma\lambda}$ walks of $(\sigma, \lambda)$ the
expression is evaluated at most $2(n + 1) + W_{\sigma\lambda}(c_{\min} + 1)$
times, at most $n + 1 + W_{\sigma\lambda}(c_{\min} + 1)$ slots are looked up,
and at most $n + 1 + W_{\sigma\lambda} c_{\min}$ records are stored. A lookup
costs 1, and storing or reusing a record costs a constant plus the charges of
its expectations, which are a constant by the above. Marking a slot is free
([Skip Runs](../runtime/parsing.md#skip-runs)). The walks of
$(\sigma, \lambda)$ therefore cost at most $c_1 (\hat{n} + W_{\sigma\lambda})$,
and there are a constant number of sites, each with two lookahead states.

Outside its sites, a reused skip is a finite expression without calls, so an
evaluation of it costs a constant, which is local cost as in Lemma 1, and
starts a constant number of walks: each walk is an evaluation of a site, and
each part of the skip outside the sites is evaluated at most as many times as
the product of the upper bounds of the bounded repetitions that enclose it. So
$\sum_{\sigma\lambda} W_{\sigma\lambda} \le c_2 E$, where $E$ is the number of
evaluations of the skip expressions of reused skips. The parser evaluates a
skip expression only after the base of its `#recover` fails, once for each
evaluation of the `#recover` expression. That expression is a part of a body
outside the scanning sources, evaluated a constant number of times in each
body evaluation by the argument of Lemma 1, or a part of a scanning source,
evaluated a constant number of times in each of its activations. So $E$ is at
most a constant times the body evaluations and activations, which Lemmas 3,
4, 5 and 10 bound. $\square$

Removing a reused skip from the scanning sources changes which units are
committing: a unit whose only scanning source was such a skip has none, though
its skip can examine input up to the end. No step of a reused skip is charged
to a slot in Lemma 4; this lemma pays for every walk, whichever unit abandons
it and however often its skip is evaluated again. A continuation that examines
again the input that the skip examined charges its own steps as before, and
the slots that the skip examined carry no earlier charge from it. Slots last
one parse: a [Document](../runtime/documents.md) parse starts without them,
which leaves the bound of each parse unchanged. A
[stream](../runtime/streaming.md) parse drops the slots before the committed
position, which no later walk reaches because a walk only moves forward, so
the bound holds for it as well. An error value that a recovery builds carries
its range and message, not the skipped text, so it costs a constant
whatever the skip. A parse with missing-token
insertion keeps no slots, and is outside the classes, as
[Classification](../runtime/parsing.md#classification) states.

## Lemma 10 (Rules That Are Not Memoized)

Let $B$ be the number of body evaluations of memoized rules, growth steps
included, and $A$ the number of activations.

The number of body evaluations of rules that are not memoized is at most a
constant times $B + A + 1$.

The lemma uses none of conditions 1 to 5.

*Proof.* Let $S$ be the start rule where the parse begins, and write $E(r)$
for the number of body evaluations of a rule $r$.

*One place.* Let a call of $r$ be written at a place $s$ in the body of a rule
$Q$. The number of evaluations of $s$ is at most $c_1 (E(Q) + A_Q)$, where
$A_Q$ is the number of activations of the scanning sources of the body
evaluations of $Q$, growth steps included, and $c_1 \ge 1$ is a constant. This
is the argument of Lemma 2 for one place: outside the scanning sources of the
body, $s$ is evaluated at most as many times in one body evaluation as the
product of the upper bounds of the repetitions that enclose it; inside them,
at most that many times in one activation of the innermost scanning source
that encloses it.

*Evaluations by call.* A rule $r$ that is not memoized is no leader, so it
does not grow, and the memo of an ordinary parse answers no call of it. Each
call of $r$ therefore evaluates its body at most once; a Document parse, which
memoizes $r$, evaluates it no more often. Apart from its calls, the body of
$r$ is evaluated only when the parse begins at $r$: the parse of an operand
inside an Operators Rule is part of the same call
([Whole-Rule Action](../language/operators-rule.md#whole-rule-action)), and
its operators are activations. So $E(r) \le [r = S] + \sum_s N(s)$, where
$s$ ranges over the places that call $r$, $N(s)$ is the number of evaluations
of $s$, and $[r = S]$ is 1 if $r = S$ and 0 otherwise.

*Rules called from one place.* Let $r_0$ be a rule that is not memoized and
that calls a rule, so that it is called from at most one place. Define a
chain: while $r_i$ is such a rule, is evaluated in the parse, and is called
from one place, let $r_{i+1}$ be the rule in whose body that place is. The
chain ends at a rule $r_d$ that is memoized, that is not evaluated, or that is
called from no place, which is then $S$, since every other evaluation of a rule
is made by a call. No rule that calls no rule is on the chain beyond $r_0$,
since each $r_{i+1}$ calls $r_i$.

The chain has no repetition. Suppose $r_i = r_j$ with $i < j$. The rules
$r_i, \dots, r_{j-1}$ form a cycle of calls $C$ on which every rule is
evaluated, is not memoized and is called from one place, which is in the body
of a rule of $C$. If $S$ is not on $C$, consider the first evaluation of a
rule of $C$ in the parse. It is made by a call, in a body evaluation that
began before it and is therefore of a rule off $C$, which the one place of the
rule excludes. So $S$ is on $C$. The parse begins only at a start rule, and
following the one caller of each rule from $S$ along $C$ returns to $S$, so
$S$ is a start rule that its own body calls through rules called from one
place. Such a rule is memoized, while no rule of $C$ is. So the rules of the
chain are distinct, and $d < |\text{rules}|$: $r_d$ differs from the earlier
rules because it ends the chain and they do not.

By the two steps above, $E(r_i) \le 1 + c_1 (E(r_{i+1}) + A_{r_{i+1}})$ for
$i < d$, and unfolding the chain gives
$E(r_0) \le c_2 (1 + E(r_d) + \sum_{0 < i \le d} A_{r_i})$ with
$c_2 = d \, c_1^d + c_1^d$, a constant. The activations in the sum are those of
the body evaluations of the rules of the chain above $r_0$, up to the rule
$r_d$ where it ends, and each is among the activations counted in $A$.
$E(r_d)$ is at most $B$ when $r_d$ is memoized, 0 when it is not
evaluated, and 1 when it is $S$ called from no place. Summed over the rules,
the evaluations of the rules that are not memoized and call a rule are at
most $|\text{rules}| \, c_2 (B + A + 2)$.

*Rules that call no rule.* Every call of a rule that calls no rule is made in
the body of a rule that calls one, which is memoized or is a rule called from
one place that is not memoized. By the first two steps, the evaluations of
such a rule are at most $1 + c_1 \sum_Q (E(Q) + A_Q)$, over the at most
$|\text{rules}|$ rules $Q$ whose bodies call it, which is a constant times
$B + A + 1$ by the bound above. $\square$

A rule of a left-recursion cycle other than a leader that is not memoized is
covered by the same argument: each of its calls, in a context of growths or
not, evaluates its body at most once, and the growth steps that evaluate its
one place are in $B$.

In the reference implementation, `transientRules` in `internal/compile` marks
the rules that are not memoized. It counts the places that call each rule over
the whole grammar, so a rule that the start rule does not reach can only add
places and make a rule memoized. It exempts a rule marked `@start`, which
`startMarked` reports, when following the one caller of each rule from it
returns to it. Every entry point admits only a rule marked `@start` as the
rule where a parse begins: `Program.CheckStart` in the engine, for whole,
stream, prefix and Document parses and for loaded modules,
`grammar.StartRule` in a generated Go parser, and `startIn` in a generated
TypeScript parser. `Parser.Memoizes` memoizes a leader whatever
`transientRules` marks.

## Lemma 11 (Sizes)

Let the rules that the start rule reaches satisfy conditions 1 and 4. The
**top size** of a value is the size that a copying operation copies of it:
the characters of a string, or the number of elements of a list. The
**components** of a value are the value itself and the components of its
elements and members. A value is **anchored** when no component of it is the
value of a variable, and its size cannot come from a capture made in a
positive lookahead, as condition 4 measures it.

*Top sizes.* Every anchored component of the value of a match $m$, and of a
value that an action, a predicate or a binding of $m$ builds, has top size at
most a constant times $|m| + 1$. A copying operation in a body evaluation $e$
copies, of each anchored value that it reads, at most a constant times the
$\rho$ of Lemma 6.

*Proof.* A component is a literal, an int or a Boolean, of constant size; a
node, whose text is the input of a match inside $m$, since a capture in a
positive lookahead is not anchored; the list of a repetition, with
one element for each iteration, at most $|m| + 1$ of them, since only the last
iteration can consume nothing; a component of the value of a call, whose
match lies inside $m$; or a value that an operation of $m$ builds. Among the
operations, a list, a tuple and a constructor have a constant number of
items; `+` and `concat` have a top size that is the sum of those of a constant
number of operands; `map` and `filter` have at most the number of elements of
their list; `text` has the top size of its operand; a fold has the value of
its initial value or of its lambda; and an element or a member is a component
of the value that it is taken from. So a component that one term builds has
top size at most a constant $r$ times the largest top size of the
components that it reads, and the terms of one action nest to a depth that is
a constant of the grammar, the lambdas over bounded lists repeating them a
constant number of times by Lemma 7.

Three ways remain in which a top size could grow without a constant bound, and
condition 4 closes each. A lambda over a list that is not bounded runs once for
each element, but each run builds its components from the element, from values
that are not copied, and from constants: by condition 4 it copies nothing from
outside it that is not bounded by a constant. A fold over a list that is not
bounded passes its first parameter on from step to step, but no copying
operation copies it, though a term such as `list(s)` may read it, so the
components of the first parameter are components of the initial value, of the
elements, of values read from outside, or built in one step by terms that copy
none of them, and their top sizes do not grow from step to step. Over a bounded
list, a fold may copy its first parameter, and Lemma 7 bounds it by a constant
multiple. A recursion passes the values of its calls up from level to level, but
no copying operation copies the value of a call on a recursion with its rule,
nor a part of it that holds such a call; other terms, such as `list($w)`, read
such a value without copying it. A copy reads only parts of such values that
hold none, through members: the value of a capture inside the match of the call,
which is a node, a list of a repetition, or a value of a rule outside the
recursion.

Induction on the strongly connected components of the call graph, from those
that call no other, and within one component on the nesting of matches, then
gives the claim. Let $C_h$ bound the components of the values of the rules of
the components below a component $\mathcal{S}$, as $C_h(|m'| + 1)$ for their
matches $m'$. In an evaluation of a rule of $\mathcal{S}$ with match $m$, a
copying operation reads components of the captures and calls that hold no call
on $\mathcal{S}$, of the parts of calls on $\mathcal{S}$ that hold none, and
constants, each of top size at most $\max(C_h, 1)(|m| + 1)$; the other terms
build a value of constant top size or pass a component on. So a component that
the evaluation builds has top size at most $r^{t}\max(C_h, 1)(|m| + 1)$, with
$t$ the largest depth of the terms, and a component that it passes on from a
call on $\mathcal{S}$ has the bound of the match of that call, inside $m$, by
the induction on nesting. The constant does not compound over the levels of the
recursion, since a level copies no value of the level below.

For the operation in $e$, the matches that its anchored operands read lie in
the read input of Lemma 6, and their lists of repetitions have as many
elements as the iterations that $\rho$ counts, so each value that it reads
has top size at most a constant times $\rho$. $\square$

*Sizes in the read input.* The size degrees of
[Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)
bound the size of a value of size degree $d$ by a constant times $n^d$
(*Size degrees* in Lemma 7). For an anchored value that a copying operation in
$e$ reads, the same bound holds with $\rho$ in place of $n$.

*Proof.* The claim is a substitution: every inequality of *Size degrees* and of
the sums after the proof of Lemma 7 stays true when $n$ is replaced by any
number $X \ge 1$ that bounds each of the quantities for which those proofs use
$n$, since those proofs use $n$ only as such a bound and combine the bounds by
sums, products and powers, which are monotone in $X$. We list each use and show
that $\rho$ bounds it, up to a constant, for an anchored value that the
operation in $e$ reads.

- *Own values* bounds a value by
  $\alpha_{\mathcal{S}}(|m| + N_{\mathcal{S}}(m))$ for its match $m$, without
  $n$, and *Matches nested in one match* bounds $N_{\mathcal{S}}(m)$ by
  $h(|m| + 1)$. The matches that the value reads lie in the read input, by Lemma
  6, so each such bound is at most a constant times $\rho$.
- *Size degrees above 1* bounds the number of elements of the list of a
  repetition by $\hat{n}$, and the number of matches of a recursion inside a
  match by $N_{\mathcal{S}}(m) \le h \hat{n}$. For a repetition and a match that
  the value reads, these are at most the iterations that $\rho$ counts and
  $h(|m| + 1)$ with $m$ in the read input, so at most a constant times $\rho$.
  The bound $n^d$ on one level is, by the induction of that proof, the bound of
  a value of degree $d$ inside the match, so it is at most a constant times
  $\rho^d$ by the same substitution at the lower degree.
- The sums after the proof of Lemma 7 use $n$ to the size degree of each value
  that a lambda or a fold reads from outside it, of the three quantities of the
  initial value, and of those of the list, from which the number of steps and
  $\sigma(b)$ come, and to the size degrees of the numbers of elements of the
  lists of the lambdas around a fold; the factor $n^a$ of a term is that of a
  value read from outside. Each of these values is a capture of $e$, a part of
  one, or a value built from them, so by the induction on the forms its
  quantities are at most a constant times $\rho$ to their degrees.
- The innermost lambda over own captures of a repetition uses
  $\sum_i \ell_i \le c\,n$, since the captures are distinct parts of the input.
  They are distinct parts of the read input, so the sum is at most $c\rho$.

No other use of $n$ occurs in those proofs. A value that is not anchored is
excluded: a value of a variable has size up to $L$ whatever the match, and a
capture in a positive lookahead holds input outside it, so for them the bound
holds only with $n$. $\square$

*Values of variables.* Let no evaluation of a binding read a value that is
built, through bindings, captures and rule values, from an earlier
evaluation of the same binding. Then $L$ is at most a constant times
$\hat{n}$.

*Proof.* Every value that a binding binds is built from literals, values of
matches, and the values of variables that its term reads, which earlier
evaluations of other bindings, or the initial values, gave. Follow the
values of variables back through the bindings that built them: by the
assumption, a chain of such evaluations repeats no binding, so it is no
longer than the number of bindings of the grammar. By *Top sizes*, with the
values of variables that a term reads added to its operands, a binding binds
a value of top size at most a constant times $\hat{n}$ plus a constant
times the top sizes of the values of variables that it reads, since a match
consumes at most $n$ and a capture in a positive lookahead at most the rest
of the input. Induction along the chains gives a bound of a constant times
$\hat{n}$. $\square$

The assumption is what condition 1 states for a binding whose value can
reach a later evaluation of the same binding, through the forms that the
condition lists; that those forms are the only ways in which the value
reaches it is step 6 of [Open Steps](#open-steps). The theorem does not use
the bound on $L$: its cost carries the factor $L + 1$, and
[Classification](../runtime/parsing.md#classification) derives from the
bound that the second class costs at most a constant times $K \cdot n^2$.

## Lemma 13 (Bounded Scans)

The activations of each [bounded
scan](../runtime/parsing.md#conditions-for-linear-cost) number at most a
constant times $K \hat{n}$. The proof uses Lemma 3 and the definitions: none of
the conditions 1 to 5, and neither Lemma 4 nor Lemma 5. It is a proof for the
cost view, and holds of the written grammar given the condition of the cost view
(step 9 of [Open Steps](#open-steps)).

Fix a set of characters $X$. For a position $p$, let $r(p)$ be the number of
characters of $X$ directly before $p$, and let $\rho(p)$ be $-1$ when the
character at $p$ is not in $X$ or $p$ is the end of the input, and $r(p)$
otherwise. Every position that a parse evaluates an expression at lies on the
one sequence of characters that decoding the input from its start gives, since
every literal of a grammar is valid text, so $r$ and $\rho$ are well defined
there. Consuming a character $c$ takes $p$ to $p'$ with
$\rho(p') \le \rho(p) + 1$ when $c \in X$, and $\rho(p') \le 0$ when
$c \notin X$: in the first case $\rho(p) = r(p)$ and $r(p') = r(p) + 1$, and in
the second $r(p') = 0$.

**Surely consumes.** Let $S(e)$ be the characters that the
[specification](../runtime/parsing.md#conditions-for-linear-cost) says $e$ surely
consumes, which its rules build from below. Every evaluation of $e$ at a position
with a character of $S(e)$ next succeeds and consumes input, and no evaluation of
an $e$ with $S(e)$ not empty fails after a cut that is loose in $e$, one that
commits the choice, option or repetition around $e$. The proof is by induction on
the derivation of the character, with the second claim first. A cut that is loose
in a sequence is in the part that decides or in a part after it, since the options
and repetitions before have none and a lookahead delimits its own; the parts after
it always succeed, and the part that decides does not fail after its own by
induction. The other forms pass the claim to their expression or delimit the cut.

For the first claim, a terminal is immediate. A repetition with a minimum of 1
succeeds in its first iteration by induction, and a later iteration that fails only
stops it, by the second claim. In a choice, each alternative before the one that
surely consumes $c$ either succeeds, and it consumes input since it cannot match
nothing, or fails with no loose cut in it, so the next is tried. A `#recover` or
`#missing` there can only make an alternative succeed, and what it matches then
counts in whether the alternative can match nothing. In a sequence, the character
is outside the first characters of each part before the one that decides, so the
expression of an option or a repetition matches nothing or fails, at no loose cut,
and a negative lookahead succeeds, since an expression that cannot match nothing
fails where the next character is outside its first characters. The part that
decides then starts at the same position, and the parts after it succeed. A call
is its body, the same evaluation, once the three kinds of call are left out. A
failure reaches further than the part that fails in two ways only, and the rules
exclude both. One is a cut. The other is a call of a rule on a left-recursion
cycle, which reads the failing seed of a growth at the position of the growth.
A rule off every cycle cannot be in progress at the position of the call, or the
call would close a cycle of left calls, and a predicate, a stream, a `#recover`
and an Operators Rule surely consume nothing, so their failures and recoveries
reach no derivation.

**Transfer functions.** The analysis gives each expression $e$ a function
$f_e(d) = \max(a, d + b)$ of the run, or a constant, and a value above 32 stands
for no bound and stays there. These functions are closed under composition,
pointwise maximum and iteration, and a rule takes the least fixed point of its
body over the calls. When $e$ is evaluated at $p$ with $\rho(p) \le d$
and succeeds at $p'$, then $\rho(p') \le f_e(d)$. The proof is by induction on the
evaluation. A terminal is the step above, character by character: the function of
a character is $0$ when no character of the set is in $X$, $d + 1$ when all are,
and the maximum of both otherwise. A sequence, a choice, an option and a bounded
repetition compose and join their parts. A repetition without an upper bound
iterates its body. When $X$ lies in the characters on which its expression surely
consumes input, it stops only where the next character is outside $X$ or at the
end of the input, so $\rho(p') = -1$. A negative lookahead of such an expression
succeeds only there. A positive lookahead of an expression that cannot match
nothing, with first characters outside $X$, succeeds only where the character at
$p$ is outside $X$, and $\$\$$ only at the end of the input. Other lookaheads
leave the position. A `#recover` ends after its base or after its skip, each of
which starts at the start of the `#recover`, and `#missing` adds the identity. An
Operators Rule makes any number of steps, each of which is an operand, an operator
part or the skip, and its function iterates their maximum. A call has the function
of the rule, whose evaluation, or the evaluation that stored the memo entry that
answers the call, is a smaller case at the same position. A growth is a sequence of
steps, each a body evaluation from the same position in which the left call ends
where the step before ended, so the same induction goes through each step.

**Runs before an evaluation.** A forward pass starts at $\rho(0) \le 0$ and carries
a value through each body, with the function of each part: into the expression of
a repetition or an Operators Rule it carries the iterated function, into a
lookahead the value before it, and into a rule the maximum over its calls. By the
claim above and induction on the evaluations, each evaluation of a repetition
$\sigma$ at $b$ has $\rho(b) \le d_\sigma$, the value that the pass records there,
with $X$ the alphabet of $\sigma$. The first bound of a bounded scan says
$d_\sigma \le 32$.

**Starts.** Let $\sigma$ have the first bound. The evaluations of $\sigma$
that reach a position $q$, by an activation at $q$ or by ending there, start in at
most $d_\sigma + 2$ positions. An evaluation at $b < q$ that reaches $q$ consumed
$[b, q)$ in its iterations, so those characters are in $X$. The character at $b$ is
in $X$, so $r(b) = \rho(b) \le d_\sigma$. Let $s = q - r(q)$. Then $s \le b$, and
$r(b) = b - s$, so $b$ is one of the $d_\sigma + 1$ positions from $s$ on. The other
start is $q$.

**Rigid expressions.** For a rigid expression $e$ there is a constant $w_e$ such
that the evaluations of $e$ that end at a position start in at most $w_e$
positions. $w_e$ is 1 for a lookahead or another expression that consumes
nothing. A terminal matches a fixed text or at most one character, of at most
four bytes. A sequence multiplies the $w$ of its parts, a choice adds them, an
option adds 1, and a bounded repetition of at most $m$ iterations adds the powers
of $w$ up to $m$, since each iteration is a part of a sequence. A repetition with
the first bound has $d_\sigma + 2$ by the claim. A call has the $w$ of the body of
its rule, which is finite because the rule is not recursive.

**Evaluations at one position.** Let $E_e(p)$ be the number of evaluations of an
occurrence $e$ at $p$. There is a constant $m$ such that
$E_R(p) \le m K$ for every counted rule $R$ and every position $p$, and
$E_e(p) \le m K$ for each expression $e$ that the second bound reaches. For a
memoized rule off every left-recursion cycle, no evaluation is a growth step or
in a context of growths, so Lemma 3 gives at most three for each variant, level
and combination of values at $p$, at most $6 \cdot |\text{levels}| \cdot K$ in all.
A rule that is not memoized is not recursive, so the counted rules that are not
memoized can be ordered so that each comes after those that call it. Take such a
rule $R$ after its callers. Its evaluations at $p$ are the evaluations of its
calls at $p$, and one for the start rule at position 0, and each of its calls is
an occurrence that the second bound reaches in the body of a counted rule, which
the order or the case of a memoized rule settles. Within a body, the body has
the bound of its rule. The parts of a choice, an option, a lookahead and the base
and skip of a `#recover` start where the expression does, so each has the bound of
the expression, as the first element of a sequence does. A later element of a
sequence starts where a rigid prefix ended, in at most $w$ positions of the start
of the sequence, and one evaluation of the sequence evaluates the element at most
once, so its bound is $w$ times that of the sequence. The expression of a rigid
repetition starts at $p$ in at most $w$ evaluations of the repetition by the
definition of $w$ for the iterations before it, and in one evaluation only once,
since its iterations that consume input start at increasing positions and the first
that consumes nothing stops it, so its bound is $w$ times that of the repetition.
The grammar is finite, so the largest bound is a constant.

**Conclusion.** Let $\sigma$ be a bounded scan and $q$ a position. Each activation
of $\sigma$ at $q$ is in an evaluation of $\sigma$ that reaches $q$. These start
in at most $d_\sigma + 2$ positions, at most $m K$ of them at each, and each has
at most one activation at $q$. So there are at most $34 m K$ activations at $q$,
and at most $34 m K \hat{n}$ in all. The grammar has finitely many bounded scans.
$\square$

**The cost view.** The analysis, the bounds and the definitions above are of the
view, and the claims hold of a parse of the view by the same proof: Lemma 3 reads
which rules are memoized, which the written grammar decides, and the other claims
read what bodies match, in the rule bodies of the view. The activations that the
parse of the written grammar makes are then counted by the
[cost view](grammar-laws.md#normalization), which removes an evaluation only when
making it again costs a constant factor. That the written grammar costs at most a
constant factor more than the view is the condition of the cost view, which step 9
of [Open Steps](#open-steps) leaves to prove; this lemma holds of the written
grammar given it.

**What the conditions leave out.** The conditions read a grammar as if a bounded
scan were no scanning source. This cannot lose a cost, because the lemma counts
the activations of a bounded scan however often the units around it are evaluated
or abandoned, and it asks nothing of them. Each other claim of this proof that an
expression without a scanning source costs a constant, as case 3 of Lemma 4 and
the bound on what follows a call in the fifth case do, is then a claim on its cost
outside the bounded scans. A scanning source inside the expression of a bounded
scan stays one: a unit that contains it contains a scanning source. Calls in an
activation of a bounded scan are counted by Lemma 2, and the evaluations they
start by Lemma 3, Lemma 5 and Lemma 10 as for any activation. A bounded scan is
not in the body of a rule of a left-recursion cycle, and a rule that is not
memoized and has one is not called from such a body, so the steps of a growth
reach one only through a call of a memoized rule off the cycle, which Lemma 3
counts. The lemma does not use the [run records](../runtime/parsing.md#run-sites): it
holds for a parse that has none.

## Theorem (Linear Cost)

When every rule that the start rule reaches satisfies conditions 1 to 5, every
parse from the start rule has a cost of at most $c \cdot K \cdot (L + 1) \cdot (n + 1)$
for a constant $c$. When no variable holds a string and every binding binds a
literal or a variable with finite values, $K$ is a constant by Lemma 8 and
$L = 0$, so the cost is at most $c' \cdot (n + 1)$.

The theorem rests on Lemmas 4 and 5, which are outlines, on Lemma 6,
which is proved given step 1 of [Open Steps](#open-steps), and on the condition
of the cost view, step 9; so the promises of the classes Linear and Linear Given
an Environment Bound do too.

*Proof, given Lemmas 4 to 6.* Each unit of $C$ is the local cost of a body
evaluation, the scanned cost of one, a call, or the walk cost. The body
evaluations of memoized rules are at most a constant times $K \hat{n}$: by
Lemma 3 outside the growth steps and the contexts of growths, and by Lemma 5
for the growth steps and the evaluations in a context of growths. By Lemmas 4
and 13 the activations are at most a constant times $K \hat{n}$, and by Lemma 10 the
body evaluations of the other rules are at most a constant times those two
counts plus one. By Lemma 2 the calls are at most a constant times the body
evaluations plus the activations. By Lemma 1 each body evaluation has a local
cost of at most $c_0 (L + 1)$. A call costs a constant and its memo lookup,
which by [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)
costs in proportion to the size of its key: the number of variables that the
rule reads, a constant, and the lengths of their string values, each at most
$L$ by its definition; so each call costs at most a constant times $L + 1$.
The scanned cost is at most a constant times the activations by Lemma 4, plus
what Lemma 5 adds, the walk cost is bounded by Lemma 9, and the values by
Lemma 6. Each bound is a constant times $K (L + 1) \hat{n}$, so the sum is too.
$\square$

The theorem holds for a parse of a [Document](../runtime/documents.md) as for
any other, in the units of $C$. Compute charges that parse the Document units
as well, which the corollary of Lemma 12 bounds.

The classifier reports the two classes only when the conditions hold, and it
over-approximates where it cannot decide them
([Classification](../runtime/parsing.md#classification)). That it does so is a
property of its implementation, checked by its tests; this proof covers the
step from the conditions to the bound, and [Correspondence](#correspondence)
names the check that decides each condition.

## Lemma 12 (Document Units)

Let a [Document](../runtime/documents.md) parse of the engine, on any of its
backends, satisfy the setting. Its **Document units** are the units of Compute
that it charges for the following work, which $C$ does not count: the edits that
lookups apply; the walks and hash indexes of chains of more than 32 entries, the
number `memoScan` of entries that a lookup walks; `doc.move`, whose declared
charge is `2*moved + move_edits`, with the cleanup that ends a move; the
operations of recorded runs; and the growth of the slots of the memo.

At the start of the parse, let the **retained size** $W$ be the sum of one plus
the size of the key of each memo entry, the number of elements of the run
records, and one plus the number of children and fields of each distinct node
that the values of those entries and elements reach. Let $G$ be the number of
slots of the gap of the memo. Let $B$ be the greatest size of an empty node that
those values reach, an **empty** node being one that starts where it ends and
its **size** the sum of one plus the number of children and fields of each
empty node reachable from it through empty nodes. Let $Q$ be the greatest length
of a chain during the parse if it exceeds 32, and $0$ otherwise. Given Lemmas 4
and 5, the Document units number at most

$$c \cdot \big( K \big( (L + 1)(Q + 1) + T + \log_2 (W + 2) \big) \hat{n} + (E + B + 1) W + G \big)$$

for a constant $c$.

*Proof.* Let $A$ be the sum of the calls, the body evaluations and the
activations of the parse. The proof of the theorem bounds $A$ by a constant
times $K \hat{n}$, through Lemmas 2 to 5 and 10, each of which counts the parse
itself, so no comparison with a parse of the same text from scratch is needed:
a reused iteration is an activation ([Setting](#setting)), and the entries and
runs of earlier parses evaluate nothing.

*Edits.* A call makes at most one lookup, in `CallBegin`, and a lookup has at
most $\min(E, T)$ units ([Setting](#setting)), so the edits come to at most $A T$:
a constant times the calls, whatever the length of the log.

*Long chains.* A lookup walks at most 32 entries before it consults the index,
and comparing a stored key with the key of the call charges at most the size of
the latter, since string values are compared only when their lengths are equal;
this is the lookup of $C$. Past that, a lookup or a store hashes its key, at a
charge of at most a constant times $L + 1$, and its probe compares at most the
$Q$ entries of the chain. A store of a key that the chain holds also walks at
most $Q$ entries to relink it. Together these charge at most a constant times
$A (L + 1) Q$. Hashing the keys to build an index takes place at most once for
each position in a parse, since only an edit drops the indexes. It charges the
size of each key of the chain: at most $W$ for the entries that the parse found,
and a constant times $L + 1$ for each of the at most $A$ entries that it stored.

*Moves.* An entry is marked to move only when it applies edits. No edit comes
during a parse, so once a lookup has applied the pending edits to an entry that
stays valid it has none left, and an entry moves at most once, since
`moveResult` clears the mark.
An element of a run moves at most once: only a record of the previous parse
moves its elements, and the parse takes over such a record at most once, by
deleting it from the records that it can take over. So at most $W$ values move.
A non-empty node moves at most once: each move tests that its generation is
older than that of the parse, and sets it to that of the parse before it visits
the children, and the nodes that the parse builds have that generation. Each
node that moves charges one unit and one for each child and field, so these
come to at most $W$. Each reference to an empty node, from a value that moves or
from a child or field of a non-empty node that moves, copies at most what its
size counts, with the second unit that the contract of `doc.move` declares, so
at most a constant times $(B + 1) W$. The edits that `move_edits` counts are
replayed at most once for each generation in a move, and only for a non-empty
node that moves there, at most $E$ for each, so at most $E W$. The cleanup
charges one unit for each copy and each generation that the move recorded,
which the same counts bound.

*Runs.* Each evaluation of a repetition makes a constant number of operations
beyond those for its elements, and each element reused or evaluated is an
activation or one of a constant number of iterations of a bounded repetition, so
at most a constant times $A$ operations are made in all. `run.prefix`,
`run.tail`, outside their moves, and `run.end` charge a constant and the
expectations of an element, a set of the items of the grammar. The growth of the
new elements that `run.end` charges doubles their storage, so it adds up to a
constant times their number. `run.finish` charges one unit for each element of
the run, and each is such an element of this evaluation. `run.resync` returns at
once for a record of this parse, which no edit separates from it. For a record
of the previous parse it searches at most $1 + \log_2 (W + 2)$ units for each
element evaluated, and then scans elements. A scan that passes every element
sets the run to reuse its tail, so it happens once for a record, and each such
record is taken over once, at most $W$ elements in all. A scan that stops does so at an
element that examined input before the end $b$ of the edit. The scan starts at
an element that starts at or after $b$, the starts of the elements increase,
apart from an empty last element, and an element examines no slot more than one
before its start, so it stops within two elements. `ResumeRepeat` reserves the
stack of values for the old elements, which grows it to at most its size plus
$W$, doubling, so the part of that growth that comes from the reservation is at
most a constant times $W + A$.

*Slots.* A store charges for the growth of the slots only where their length
crosses a power of two, by the old length. So the charges of one parse add up
to at most twice the last length, and a store at position $p$ needs at most
$p + 1 + G$ slots, so at most $2 (\hat{n} + G)$.

Summing, with $A$ at most a constant times $K \hat{n}$ and $K \ge 1$, gives the
bound. $\square$

*The units that the default budget counts.* The default budget of a Document
parse leaves out the Document units of Lemma 12 that the parse reserves in regions
of Document work: the memo protocol of every rule call (the test of whether the
call is memoized, its lookup with the walk of a chain of memo entries and the
comparison of keys, the begin and the end of the call that store its result), the edits that
a lookup applies, the moves of reused values with the cleanup that ends them, the
operations of recorded runs, the extra units that decoding a piece of the text
costs, and the copy of the diagnostics that the parse publishes. Every other unit
is counted: the unit that opens a call, the evaluation of the body of a rule, the
examination of the input, the values that the parse builds. The counted units of a Document parse are at most the
units that a parse of the same text from scratch reserves. Each counted unit belongs to a
call, to a body evaluation, to an examination of the input or to a value that
the parse builds, and the Document parse makes each of these for the text at most as often as
the parse from scratch does: it evaluates the same bodies on the same input in the same
order, except that a memo entry answers a call and the body is not evaluated, and the
entry costs the units of the call only, and its recovered errors, which the evaluation it
replaces would have built at a unit each. A lookup and a store count none, however long the chain
they walk, so no duplicate or order of entries adds to the counted units. This holds for the
three backends of the engine and for generated Go; the tests of the reference
implementation check it on random edits as well. So a Document parse under the default
budget does not fail where the parse from scratch of its text does not. Its default
budget, raised by the Document units, is at most that budget plus the bound on the Document
units above (the terms $W$, $B$, $G$ and $Q$ are bounded by the retained state, not by
$n$, $K$ and $L$, as step 7 leaves open). A grammar whose cost is above the default
stays stopped: the counted units alone pass the default. $\square$

A node counts in $W$ whichever text it was built for, and whether or not a call
of the parse reaches it. No lemma bounds $W$, $B$, $G$ or $Q$ by $n$, $K$ and $L$: see step 7 of
[Open Steps](#open-steps). A chain has at most $E + 1$ entries under one key
([Setting](#setting)), so $Q$ is at most $E + 1$ times the number of keys at
its position; these include keys of earlier texts, which the $K$ of the parse
does not count. $G$ grows with deletions and shrinks with insertions.

## Corollary (Compute of a Document Parse)

Let $R$ be the number of recovered errors that the parse copies and replays:
the sum, over each store of a memo entry and each call that an entry answers, of
the recovered errors that the entry holds. Under the conditions of the theorem
and given Lemmas 4 to 6, a parse of a Document on a backend of the engine charges
at most

$$c \cdot \big( K \big( (L + 1)(Q + 1) + T + \log_2 (W + 2) \big) \hat{n} + (E + B + 1) W + G + R \big)$$

units of Compute, for a constant $c$. Here a parse is the run of the grammar
over the text, up to its outcome. Of these units, the calls and their lookups,
with the edits that the lookups apply, take at most
$c \cdot K (L + T + 1) \hat{n}$ when no chain exceeds 32 entries. That part is
linear in $n$ whatever the log; the other terms depend on
what the Document retains and on $R$. The whole bound has the form of a constant
times $K \hat{n}$ for a given retained state only when no error is
recovered, so that $R = 0$: $R$ can grow with the square of $n$ in a grammar of
the class Linear (step 8 of [Open Steps](#open-steps)).

*What the limit on the edits of a lookup bounds.* The edits that lookups apply come to at most
$A\,T$ units, where $A$ counts the calls, the body evaluations and the activations of the
parse, and $A \le c\,K \hat{n}$ by the theorem. So they are a constant times the work of the parse, and the
constant does not depend on the number of pending edits $E$, up to the 65,536 of the log. This is the
whole of the constant factor that the proof gives. The other Document units are not a constant factor of
$K \hat{n}$: the moves cost $(E + B + 1) W$, a term in the size $W$ that the Document retains and in the
length $E$ of the log, and the walks, the growth of the slots and $G$ depend on what earlier texts left
in the memo (step 7 of [Open Steps](#open-steps)). The total of a Document parse is therefore bounded by
the displayed form, and the claim that it is within a constant factor of a parse from scratch holds for
the part of the lookups and their edits only. The tests of the reference implementation measure the
whole for a grammar of 20,000 lines with 0 to 65,536 pending edits spread over the text: at most 4.3
times the units of a parse from scratch on each backend, and 2.2 at 6,000 edits and above, where most
entries lag behind by more than $T$ edits; the test enforces 6, and fails when a lookup applies the edits
without the limit.

*Proof.* The units of Compute of a parse are those of the steps that $C$ counts,
the Document units, and the $R$ units of the error lists, which $C$ does not
count: a store copies the list of its entry, one unit for each error, and a call
that an entry answers appends the list of the entry in the same way. The theorem
bounds the first by a constant times $K (L + 1) \hat{n}$, and Lemma 12 bounds the
second. The part of the lookups is the theorem's bound of the calls, plus the
edits and the walks of Lemma 12. $\square$

That these three are all the units of a parse is a reading of the charges of the
engine, not a derivation: no lemma relates a charge to $C$ in general.
A store of a key that a chain holds walks the chain to relink it. Here $Q$ counts
that walk; outside a Document no bound of this proof does, and it has not been
measured. It matters only where $K$ grows with the input, since otherwise a chain
has a constant number of keys.

The corollary does not cover the work that follows the outcome in the same
Control: the capture and sharing of `SnapshotWith`, the comparison of change
notifications, and the publication of diagnostics. Nor does it cover the
generated Document runtimes: they apply each edit to every entry at once, and a
`doc.move` that copies does not move a node once for a parse.

## Correspondence

The classifier in `internal/compile` decides the conditions, and the lemmas use
them. Each entry names the functions that decide a term or condition of
[Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost),
that term or condition, and the lemmas that use it. The list names the deciders,
not every helper that they call: `collect` and the walks `walkRule`,
`walkLookahead`, `walkRecovery` and `walkOperators` record the scanning sources,
the units, the bindings and the copying operations; `sameCycle` and
`onRecursion` tell the rules of one left-recursion cycle and the calls on a
recursion; `computeFollow`, `followSets`, `followFor` and `keepFor` give what
follows a call for the continuations; `consumesNothing` tells the expressions
that never consume input; `matchesWord` and `wordExcludes` evaluate a word
against a continuation; and `foldDegrees` gives the degrees of a fold from its
step matrix. `classify` runs the checks in the order `checkUnits`,
`checkGrowth`, `checkCarriedGrowth`, `checkNesting` and `checkValues`, after
`collect` and `computeBoundedScans`.

- `computeBounded`, `bounded` and `nullable` decide the expressions whose
  matches are bounded in length and those that can match nothing, which the
  terms scanning source, bounded list and growing unit read; `computeFirst`
  and `first` decide the first characters. Lemmas 1, 2, 4, 6 and 11 use the
  bounds, and Lemma 4 the first characters.
- `addSource`, `computeRuleSources` and `containsSource` decide the scanning
  sources and what contains one: Lemmas 1, 2 and 4.
- `computeBoundedScans` decides the bounded scans: `consumed` the alphabets,
  `runAnalysis` the first bound, with `sure` for what surely consumes a
  character, and `countedWalk` and `rigid` the second: Lemma 13. `newViewClassifier`
  gives them the cost view that the Setting reads.
- `reusedSkip`, `constantInput` and `costly` decide the reused skips of
  [Skip Runs](../runtime/parsing.md#skip-runs), which are no scanning
  source: Lemma 9, and Lemma 4 for the units whose only source was such a
  skip.
- `memoizedRules` decides the rules that are memoized, and `shareable` and
  `memoAt` the calls that are shared and those memoized at the call, after
  [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules): Lemmas
  3 and 10, and the shared calls of Lemma 4.
- `checkUnits` decides condition 2, with `unitFailsAfterSource` and
  `failsAfterSource` for a unit that can fail after a source, and `committing`
  for the term committing, which removes shared calls through `leads` and
  `commonLead`, keeps a call that `rescans` finds the continuation reaching
  through a call that nests at one end (`nestsAt`) or after an operand of an
  Operators Rule that nests (`nestsOps`, through its right operands or a call
  from the rule that nests at one end), for which `addRetried` adds the operator
  parts that an enclosing level tries again as a continuation of the operator
  parts that `walkOperators` records, compares first characters, excludes by
  words in `excludedByWord`, and drops the later alternatives after a `#cut` in
  `cutCommits`: Lemma 4, its failure after a source, the cases of committing and
  its claims on links.
- `withoutReentry`, `fixedMemoized`, `memoOnly` and `restBounded` decide the
  paths that enter the growth again: the fifth case of Lemma 4, with Lemma 5.
- `checkGrowingUnit`, `startsGrowth` and `growthStarts` decide condition 2
  for growing units, with their continuations: Lemma 5, *Sources at fixed
  offsets*.
- `checkGrowth` decides condition 3, and `checkCarriedGrowth`, `dropsResult`,
  `carryPaths` and `growthGoesOn` condition 5: Lemma 5.
- `checkValues`, with `selfDependent` and `reaches`, decides condition 1, and
  `constant` the bindings that bind a value that the input determines, which
  give the class Linear given an environment bound: Lemma 3 through $K$, and
  *Values of variables* in Lemma 11. `finiteVars`, `finiteBool`,
  `finiteInt` and `constantValues` decide the variables with finite values,
  and `guardOf` the guards of bindings: Lemma 8.
- `walkTerm` finds the copying operations, `mayBeString` and `stringIn` tell
  a string value from a node for `len`, and `copyOp` and `copied` decide
  condition 4 for each copy from what `sizeOf`, `sizeOfAny`, `exprSize` and
  `deepSize` say that it can copy: a call on the recursion, a fold
  accumulator, a value from outside a lambda over a list that is not bounded,
  a value of a variable, a capture in a positive lookahead, and the sum of
  the elements that a lambda copies. `lambdaCtx` decides the bounded lists:
  Lemmas 6, 7 and 11.
- `checkFoldGrowth`, `growthOf`, `growthOrder` and `doubles` decide the step
  matrix of condition 4: Lemma 7, after its proof.
- `ownRules`, `ownExpr` and `recOnce` decide the own values and the size
  degrees of the values of recursions, and `valueShape`, `ruleShape` and
  `levelShape` the size degrees: Lemma 7, *Size degrees*, with Lemmas 6
  and 11.
- `lookCaptures`, `looks`, `inLook`, `exprLooks` and `termLooks` decide the
  values whose size can come from a capture in a positive lookahead, which
  condition 4 excludes from copies: Lemma 6 and the anchored values of
  Lemma 11.
- `checkNesting` and `leaf` give the degrees of the class polynomial and the
  reasons for the class unknown of
  [Classification](../runtime/parsing.md#classification). No lemma uses
  them, since those classes carry no promise.

Every condition and term above has a check. Two parts of the proof have no
check of their own, and need none: the factors $K$ and $L$, which are
quantities of a parse that the classes carry rather than conditions, and the
three evaluations at a position for each variant of a rule that the memo
allows, which `Parser.FirstCall` and `Parser.reusable` in
`internal/engine/core` give (Lemma 3). The checks that decide no
condition are `growingBindings` and `canHoldNonEmpty`, which serve the
`growing-variable` finding of `zp lint` rather than the classification.

## Adding a Condition

A new condition enters the proof in one of two ways.

- A new way for a unit to be committing is a new case of Lemma 4. Its lemma
  shows that after the unit is abandoned, what the parser evaluates next
  examines again at most a constant amount of the input that the unit examined
  beyond its shared calls.
- A new bound on a part of the cost, or on a cost that the specification
  starts to count, is a new lemma with the part that it bounds, and the
  theorem adds its bound.

## Open Steps

The outlines leave steps 1 to 8 to be made precise, and step 9 is a condition
that the proof assumes of the cost view.

1. **Counting activations in Lemma 4.** The claims of Lemma 4 reduce the scanned
   cost to the activations, show that the successful activations of a source
   at a position are direct in distinct body evaluations, and link each to the
   one before it through a call that a committing unit shares with its
   continuation. They charge the activations of each second evaluation of a
   memoized rule to the first, and show that a memoized link whose two
   evaluations have one slot, its key and lookahead state, leads to an
   activation owned by a second evaluation when its second evaluation is in no
   context of growths. So the sum over the positions is proved for the
   activations owned by memoized evaluations of every source in a rule that
   calls a rule or is memoized at its calls, when its memoized links are of that
   kind. Three parts are open. The first two are for a source in a rule that
   calls no rule and is not memoized at its calls. First, the leaf links that
   lead to an activation owned by a first evaluation: such a link can leave a
   second evaluation through the end of its body, and the claims bound the leaf
   links in a row but not those between memoized links. Second, the leaf links when
   the shared call follows other shared calls: the frames that the
   continuation leaves are then left at the start of the unit, before the
   shared calls, or after some of them, so the frames left between two links
   need not be followed by what matches without consuming input, and the
   cycle argument of *Calls that call no rule* must be carried over. Third,
   the memoized links between slots, where one call is in a lookahead or
   discards the value of the rule and the other is not or does not, and those
   whose second evaluation is in a context of growths: Lemma 3 bounds their
   number for one rule, position and level, but the activation that such a
   link leads to can be owned by a first evaluation. The activations include
   those of the copying operations, the units of the own spans that they read
   (Lemma 6).
2. **"A unit that succeeds is not restarted."** A body that contains the unit
   can itself be evaluated again: by the memo, at most three times at a
   position for each variant of its rule; when its rule is not memoized, at
   each of its calls. Lemmas 3 and 10 count these
   evaluations; the proof must show that they do not multiply the scanned cost
   beyond a constant. Lemma 10 bounds the evaluations of the rules that are not
   memoized by the activations, so the bound of Lemma 4 on the activations must
   not rest on those evaluations, or the argument closes on itself. The claims
   of Lemma 4 proved so far do not: they count links by the memo of Lemma 3 and
   by the call places of the grammar, and charge the activations of a second
   evaluation to the first through the memoized evaluations alone. What
   remains is the activations that do not succeed, which Lemma 4 counts by the
   body evaluations that evaluate a source, those of rules that are not
   memoized included.
3. **Continuations of growing units.** The claim on sources at fixed offsets in
   Lemma 5 bounds a unit followed by a growing unit with which it shares no
   call, in all but a constant number of steps. The proof must still bound such
   a pair when the definition of sharing gives it a shared call, the later
   alternatives that a step evaluates after a growing unit fails, in the first
   step and in the step where the growth stops, and the case where the first
   step of a growth matches a part before the left call, as through `F` in
   `E = F "+" X / Y; F = E / M`, where the unit is also checked as an
   alternative that is not growing.
4. **Operators Rules.** Lemma 4 places the operators that an Operators Rule
   applies and its units among the activations and units of its claims, with the
   operator parts that an enclosing level tries again as a continuation of an
   operator part after an operand when the rule nests, through its right
   operands or through a call that parses the rule again at the end of an
   operand or an operator part; its claims on direct activations and links cover
   the operators applied at a position. The tries of operator parts that fail at
   a position when inner levels return there are at most the number of parts for
   each level, so their cost is the cost of the units up to their failure, which
   the claims of step 1 bound with its open parts.
5. **Contexts of growths in Lemma 5.** The proof must bound the evaluations
   that a context of growths repeats, which Lemma 3 leaves out: the calls
   that find no result with the same growing results and lookahead outcomes,
   and the growths of leaders inside other growths at their position, whose
   steps are shared only through equal results. It must show that the
   comparison of results by structure, which is bounded, loses only reuse,
   and that condition 5 covers every growth that is new in each step.
6. **Chains of bindings in Lemma 11.** The bound $L = O(n)$ that
   [Classification](../runtime/parsing.md#classification) states, which the
   theorem does not use. *Values of variables* in Lemma 11 proves it given
   that no evaluation of a binding reads a value built from an earlier
   evaluation of the same binding. The step must show that condition 1
   ensures this: that such a value reaches a later evaluation of the binding,
   through the bindings of other variables, the captures of callers and the
   values of rules, only through a sequence that holds the binding and a call
   that can reach its rule again, or a repetition around the binding, as the
   condition lists.
7. **What a Document retains in Lemma 12.** The bound of Lemma 12 is in terms of
   $W$, $B$, $G$ and $Q$, which depend on the history of the Document. A bound by
   the $n$, $K$ and $L$ of the parse would have to show that the nodes a parse
   moves are at most a constant times what a parse of the same text from scratch
   builds. That does not follow even if the calls of the parse are among those
   of the parse from scratch, which is not proved, and a node moves once in a
   parse, for three reasons. Nodes that the parse
   from scratch shares can be distinct here, since the values that a parse
   reuses were built by different parses: a call in a context of growths stores
   no entry, a lookup outside a lookahead does not take an entry stored inside
   one, and entries joined by a deletion each keep their value, so the step must
   bound the evaluations of one key whose nodes stay reachable, which needs a
   form of Lemmas 4 and 5 local to a region of the text. An empty node is copied
   for each reference to it, not once. And the replays of `move_edits` multiply
   the nodes by up to $E$. Apart from what moves, $Q$, $G$ and the entries of
   $W$ count what earlier texts left in the memo, which it keeps until a store,
   an edit or the drop of the table removes it.
8. **Recovered errors in Compute and memory.** $C$ counts no recovered error, so
   the theorem holds and says nothing of them, but each entry stores a copy of the
   errors recovered in its call, and each call that it answers appends them
   again. No bound on $R$ exists, in a Document or outside one, and the grammar

   ```text
   rule main = s $$
   rule s = item s "!" / item s / item
   rule item = ("a" ";") #recover(skip=[^;]* ";")
   ```

   which is classified Linear, shows that none by $n$ can hold. On `b;` repeated
   $n$ times the entry of `s` at each item holds the errors of all the items after
   it, so its copy and its replay cost in proportion to $n$ at each of $n$
   positions. Compute for each item rises from 518 at $n = 250$ to 6,147 at
   $n = 4000$, a first parse of a Document, where $E$, $W$, $B$, $G$ and $Q$ are
   zero, grows in the same way, and the default limits end the parse at 2,000
   items on all three backends. The memory allocated grows with it: 75.8 MB,
   289 MB and 1,091 MB at $n = 4000$, 8,000 and 16,000. A proof of linear Compute
   for parses that recover errors needs an implementation that does not copy or
   replay these lists in proportion to their length.

9. **The condition of the cost view.** The classifier decides the conditions and
   Lemma 13 counts on the [cost view](grammar-laws.md#normalization), and the
   proof transfers each bound to the written grammar through one condition: on
   every input, the parse of the written grammar costs at most a constant times
   the parse of the view. The proof must show it for each rewrite that the view
   applies, N1 to N12 without N7, under the further conditions that the view sets
   on them: that a rewrite removes only evaluations that the original never makes
   or makes again in the same state, and only cheap ones, and that N12 drops and
   left-factors only what those conditions allow.
