# Linear Cost

Arata Furukawa · Last revised 2026-11-12

This proof shows that the [Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)
bound the [cost](../runtime/parsing.md#cost) of every parse, so that the
classes Linear and Linear Given an Environment Bound of
[Classification](../runtime/parsing.md#classification) hold their promise. It
splits the cost into the parts that the specification counts and bounds each
part by one lemma, so that a new condition is added as a lemma, or as a case of
one, without changing the others.

Lemmas 1, 3, 7, 8, 9, 11 and 14 are proved, Lemma 16 given the theorems of
the [Memoization](memoization.md) and [Documents](documents.md) proofs, and
Lemmas 2 and 10 apart from the tries of an Operators Rule outside its activations (step 3 of
[Open Steps](#open-steps)). Lemmas 13 and 15 are proved
for the [cost view](#setting) that the classifier reads, and the
[Cost View](cost-view.md) proof transfers every bound on the view to the grammar
as written, except for the values that actions build from values that the view
reshapes and the run-time errors of those actions, step 7 of
[Open Steps](#open-steps). Lemma 6 is proved given the count
of activations of Lemma 4 (step 1 of [Open Steps](#open-steps)). The theorem is
proved from the others, so the promise of the class Linear rests on Lemmas 4 and
5, which are outlines, on Lemma 6, which is proved given step 1, on step 3, for
the tries of an Operators Rule that Lemmas 2 and 10 leave out, and, for the
values that actions build from reshaped values, on step 7. Lemma 12
and its corollary add what a [Document](../runtime/documents.md) parse charges
beyond the cost: they are proved given the same counts, step 3 and the bounds of
[Work of a Document Parse](../runtime/documents.md#work-of-a-document-parse), in
terms of what the Document retains; over a sequence of parses, the total that it
retains drops out, and what each parse moves and the lengths of its chains
remain, which step 6 of [Open Steps](#open-steps) leaves unbounded. After
Lemma 12 an argument, not a proof, shows that the units
which the default budget counts for a Document parse are at most those of a
parse of the same text from scratch.
Lemma 14 bounds the work of the recovered errors, which $C$ does not count, by a
constant times $C$, and Lemma 16 bounds the number of the recovered errors of an
outcome by a constant times the input, so that listing them outside a Document
takes at most a constant times $C$.
[Correspondence](#correspondence) maps the conditions and terms of the
specification to the lemmas that use them. Lemmas 4 and 5 are given as
outlines: each states its claim, the conditions it uses, and the steps that its
proof must still make precise, listed under [Open Steps](#open-steps). Lemma 4
proves a reduction and bounds the repetitions at one position: the scanned cost comes down to the number of activations, those that do not succeed come down to those that do and to the body evaluations of memoized rules, without resting on Lemma 10, a failure
after a source passes through the forms that condition 2 checks, an activation
is repeated at a position only after an abandoned committing unit or a dropped
growth step, at most one successful activation of a source at a position is
direct in one body evaluation, and every other follows a call that a
committing unit shares with its continuation, which, in the reference parse of
the memoization proof, is evaluated again and repeats the first activation at
the position in it; the memo bounds these for a
memoized rule and the cycles of calls that nest at one end bound for a rule that
calls no rule when the call is the first that the two share, and it shows,
apart from three open points, that evaluations of a call that nest without a
bound and end at one position make the call nest at one end. It proves the sum
over the positions for the activations owned by memoized evaluations of the
sources of rules that call a rule or are memoized at their calls, by charging
the activations of each later evaluation of a rule or its twin at a position to
the first, and by comparing the two evaluations of each memoized link in the
reference parse of the memoization proof, in either lookahead state, either
variant and through the growths that the later one makes again, apart from the
activations that a dropped step of a growth or a path that enters the growth
again repeats, which Lemma 5 counts before it, and with no count of the
evaluations in a context of growths. It leaves the sum open for the sources of rules that call no rule and are not
memoized, for the links of a leader with a growth of its cycle open at the
start of only one of the two evaluations, and, in a Document parse or through results kept in a context of
growths, for the links that the comparison leaves out. Lemmas 2 and 10 and the
bound of Lemma 4 on the activations that do not succeed leave out the operands
and the operator parts that an Operators Rule tries outside its activations,
step 3 of [Open Steps](#open-steps). Lemma 5 proves the
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
view](../runtime/parsing.md#cost-view) of the rule bodies, with the memoized rules,
the left-recursion cycles, the variables and the run sites of the grammar as
written. The proof reads that view as the grammar: its rule bodies match what the
written ones match, and the written grammar costs at most a constant factor more
on every input, which is the condition of the cost view. So a bound on the parse
of the view is a bound on the parse of the grammar. The [Cost View](cost-view.md)
proof shows that condition for every rewrite that the view applies, with explicit
constants, except for the values that actions build from a value that a rewrite
reshapes and the run-time errors of those actions, which step 7 of
[Open Steps](#open-steps) leaves open.

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
elsewhere, is no part of $C$: Lemma 12 counts it as **Document units**, apart
from the extra units of decoding the characters of its text, which the corollary
of Lemma 12 counts with $C$.

**Pending edits of a Document.** A lookup in the memo of a Document parse of
the engine also applies to the entry it finds the edits of the log that the
entry has not accounted for since it was stored or last found, and charges one
unit for each edit it applies, but never applies more than $T = 128$: an entry
that lags behind by more edits is treated as absent, and the call evaluates its
body again ([Work of a Document Parse](../runtime/documents.md#work-of-a-document-parse)).
Let $E$ be the length of the edit log at the parse: the edit that finds 65,536
edits in the log drops the memo and empties the log, so a parse can see a log of
65,536 edits, $E \le 2^{16}$ there, and $E = 0$ outside a Document. Each lookup has at most $\min(E, T)$ of these
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
deletion that inserts nothing. So one edit joins at most two slots. A store of
a rule that a parse from scratch memoizes too, under a key that the slot holds,
adds no entry, and by induction on the edits a slot holds at most one entry
under such a key plus one for each edit that joined slots into it, at most
$E + 1$. A store of a rule that only a Document memoizes looks at the first 32
entries of the chain only, and when the key is not among them it adds an entry
in front, so a chain of more than 32 entries can hold more entries under such a
key. Only one entry under a key matters in a parse: a lookup returns the first
entry of the chain under its key, through the chain or the index alike, and a
store replaces that entry in its place or puts the new entry in front of it, so
the others are not seen until a later edit relinks them. A lookup compares at most 32 entries
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

The proof places every step outside the scanning sources or inside one of their
activations. Inside an Operators Rule this does not hold: the operands and the operator parts that an Operators Rule tries, and that fail, lie inside its scanning source but in no activation of it, and the
proof does not bound how often they are tried. The claim is proved for the calls
outside them; the tries of an Operators Rule are step 3 of [Open
Steps](#open-steps).

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
**variants** of the rule. The parse memoizes a twin exactly when it memoizes
its rule, whatever the number of places that call the twin
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)), so the
exceptions above decide it for both variants. In a recognition, a rule that no call
inside a capture that a predicate can read reaches has no twin, and all its calls
are of one variant. The
**lookahead state** of an
evaluation is $\lambda$ of the [memoization proof](memoization.md#setting): $1$ inside a lookahead and
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

The memo that this lemma counts is that of
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules). It memoizes
a leader in every parse, and a [Document](../runtime/documents.md#reuse-model)
also memoizes the transient rules. A parse with missing-token insertion, which
is outside the classes, leaves unmemoized every rule other than a leader that
contains `#missing`, directly or through the rules it calls. A first evaluation
may leave no result, a result computed inside a lookahead or a `skip` answers
only the calls inside them, and one computed outside them answers every call,
so at most one evaluation in each of the two states stores a result, which
gives the three evaluations.

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
is one of the ways in which condition 2 lets a unit fail after a source.
Condition 2 is a property of every such unit, and the classifier, which
over-approximates where it cannot be exact
([Classification](../runtime/parsing.md#classification)), checks every unit that
it cannot show to be unable to fail after a source.

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
evaluation of a unit that starts at a position $s \le q$, which is committing
when the source is not a bounded scan. From
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

Let $w$ be a unit, and let the source not be a bounded scan; a unit whose
only scanning sources are bounded scans has no scanning source in the sense of
condition 2, which need not check it. $w$ contains the scanning source of
$\alpha$, or a call that leads to it. It is the expression of a lookahead, which condition 2
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
number of times by each, so the activations that do not succeed are bounded by
the body evaluations and the successful activations, as the following claim
shows without resting on a bound on the evaluations of the rules that are not
memoized. It is left to bound the successful activations that start at one
position.

**Activations that do not succeed.** In this claim an activation is an
iteration, an operator applied or an evaluation of a skip expression; the
copying operations, whose activations Lemma 6 counts, and the growths, which
have none, are left out. Let $A_s$ and $A_f$ be the numbers of these
activations that succeed and that do not, and $B$ the number of body
evaluations of memoized rules, growth steps included. Then
$A_f \le c_4 (B + A_s + 1)$ for a constant $c_4$, and the body evaluations of
the rules that are not memoized number at most a constant times
$B + A_s + 1$, when no rule that the start rule reaches is an Operators Rule.
Inside an Operators Rule, the operands and the operator parts that an Operators Rule tries, and that fail, lie inside its scanning source but in no activation of it, so the dichotomy of the proof below does
not hold of a source or a call inside them, and the claim rests there on step 3
of [Open Steps](#open-steps), as Lemmas 2 and 10 do.

*Proof.* Write $E(Q)$ for the number of body evaluations of a rule $Q$, growth
steps included, and $s(\sigma)$ and $f(\sigma)$ for the activations of a
scanning source $\sigma$ that succeed and that do not, in all the body
evaluations of the rule whose body holds $\sigma$; an activation inside a call
belongs to the body of the callee. Each evaluation of $\sigma$ has at most one
activation that does not succeed, its last, as above, so $f(\sigma)$ is at most
the number of evaluations of $\sigma$. An evaluation of $\sigma$ lies either
outside every other source of the body, where it is made at most a constant
number of times in one body evaluation, by the argument of Lemma 1, or inside
an activation of the innermost source $\sigma'$ of the body that encloses it,
where it is made at most a constant number of times in that activation, by the
same argument, since an activation evaluates a fixed part of the grammar once.
A copying operation and a growth evaluate no part of a body inside their
activations, so $\sigma'$ is a repetition, an Operators Rule or a skip
expression. So for a constant $c$,
$f(\sigma) \le c\,(E(Q) + s(\sigma') + f(\sigma'))$, or
$f(\sigma) \le c\,E(Q)$ when no source encloses $\sigma$. The sources of a body
nest to a depth that is a constant of the grammar, so unfolding the inequality
along the sources that enclose $\sigma$ gives
$f(\sigma) \le c' (E(Q) + \sum_{\sigma'} s(\sigma'))$ over those sources, and
summing over the constant number of sources of the body,
$A_f(Q) \le c'' (E(Q) + A_s(Q))$, where $A_s(Q)$ and $A_f(Q)$ sum $s$ and $f$
over the sources of the body of $Q$.

The proof of Lemma 10 bounds the evaluations of the rules that are not memoized
through the quantities $E(Q) + A_Q$, where $A_Q$ counts every activation of the
sources of the body of $Q$, and through nothing else that depends on the
activations. Its activations of copying operations are no evaluation of a place,
since a copy evaluates no part of the grammar, so its argument for one place
holds with $A_Q$ replaced by $A_s(Q) + A_f(Q)$, and by the inequality above
$E(Q) + A_s(Q) + A_f(Q) \le (1 + c'')(E(Q) + A_s(Q))$. Each step of that proof
then holds with $A$ replaced by $A_s$ and its constants multiplied by
$1 + c''$, so the body evaluations of the rules that are not memoized number at
most a constant times $B + A_s + 1$. Summing $A_f(Q)$ over the rules,
$A_f \le c'' (B + E_{\mathrm{nm}} + A_s)$, where $E_{\mathrm{nm}}$ is that
number of evaluations, which gives the bound. $\square$

**Order of the bounds.** The bounds follow one another in this order, and none
rests on a later one. First, the claims of this lemma from *Failure after a
source* to *Nothing in a memoized link is a head*, other than *Activations that do not succeed*,
count nothing: they rest on the conditions, the memo of Lemma 3 and the memoization proof, and hold of every
activation, including those of growth steps and of evaluations in a context of
growths. Second, Lemma 5 bounds $B$, the evaluations in a context of growths,
the activations that growth steps and those evaluations own, and the successful
activations that a dropped step of a growth or a path that enters the growth
again (case 5 below) repeats, from those claims, Lemmas 1 to 3 and conditions 3
and 5; it must not use the bounds that come after it (step 4 of [Open
Steps](#open-steps)). Third, *Activations held by heads* bounds the successful
activations owned by memoized evaluations from Lemma 3, *Replayed activations*
and *Links to heads*, with no
count of the evaluations in a context of growths, and with the second bound
gives $A_s$. Last, this claim bounds the evaluations of the rules that are not
memoized and $A_f$.

The claims from here to *Nothing in a memoized link is a head* concern the
scanning sources other than bounded scans, whose activations Lemma 13 counts.
By the restarts above, a successful activation of such a source is repeated at a
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
   position where the parser leaves or starts no growth step and no evaluation
   in a context of growths between them. A rule of a cycle other than a leader is memoized at a call that is
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
in the evaluation inside $w$ of a call that $w$ and $\kappa$ share, which
starts at or before $q$.

*Proof.* If $\alpha$ is in none of the shared calls, the proof above applies:
$\alpha$ is in $w$ after the shared calls, inside a call of $w$ or not, so the
character at $t$ is a first character of both, and a call of $w$ after the
shared calls that contains $\alpha$ contains a scanning source, so case 3 does
not hold either. So $\alpha$ is in a shared call, which starts at or before
$q$. $\square$

The claim does not place $\alpha'$ in an evaluation of the same call. The part
of the shared call that holds $\alpha$ can be an abandoned unit that examined
input past the end of the call, and the memo can then answer the call in
$\kappa$:

```zp
@start
rule main = s "x" / s "w" / s (R "b")? "y"
rule s = "." (R "a")?
rule R = [ ]*
```

On `.` followed by spaces and `y`, the second activation of the repetition of
`R` at a space lies in the second evaluation of `s`, inside the unit `s "w"`,
whose continuation `s (R "b")? "y"` shares the call of `s` at $0$. The memo
answers that call, and the next activation at the space lies in the call of
`R` that `main` makes after it, in no evaluation of `s`.

So the successful activations of $\sigma$ at $q$, $\alpha_1, \alpha_2, \ldots$
in the order in which they start, outside case 5 and dropped steps, are direct
in distinct body evaluations of $R$, and for $i \ge 2$, $\alpha_{i-1}$ lies in
the evaluation $e_i$ of a call shared by a committing unit $w_i$ and its
continuation, inside $w_i$. When $\alpha_i$ lies in an evaluation $e'_i$ of the
same call at the same position, made after $w_i$ is abandoned, $i$ is
**linked**, and the pair $(e_i, e'_i)$ is the $i$-th **link**; otherwise $i$ is
**free**, as is $i = 1$. *Reference links* below gives each activation of the
reference parse such a pair, which the free ones lack in the parse.

**Memoized links.** If the rule of the $i$-th link is memoized at both calls
and neither $e_i$ nor $e'_i$ is in a context of growths, then either the rule is
no leader and $e_i$ and $e'_i$ are two of the at most six body evaluations of
the rule at its position and level that the memo allows (Lemma 3; the rule reads
no variable), or the rule is a leader and $e_i$ and $e'_i$ are two growths of it
at its position and level, $e_i$ inside a lookahead and $e'_i$ outside them. At
most five such links of $\sigma$ at $q$ use one rule, position and level.

*Proof.* Outside contexts of growths, a leader grows at a call whose result at
the position is not in the memo
([Growths of Left Recursion](../runtime/parsing.md#growths-of-left-recursion)),
and the growth stores its result
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)), which answers every later call there outside contexts in the same lookahead
state or a greater one (*Answered calls* below). So of two growths of a leader
at one position and level outside contexts, the earlier is inside a lookahead and
the later outside them, and there are at most two. The memo answers a call
without evaluating it, and $e'_i$,
which contains $\alpha_i$, is evaluated, and so is $e_i$, made earlier. If links
$i < k$ had $e'_k = e'_i$, then $\alpha_{k-1}$, which starts no earlier than
$\alpha_i$, would lie in $e_k$, which ended before $e'_k$ started, and so before
$\alpha_i$ started. So the links that use one rule, position and level have
distinct $e'_i$, none of which is the earliest of the evaluations. $\square$

**Calls that call no rule.** Let the rule $r$ of the links $i$ to $k$ call no
rule, read no variable and not be memoized at its calls, let the shared
call of each link be the first call that its unit and continuation share, and
let no body evaluation that the parser leaves or starts at the position of the
links, from the end of $w_i$ to $e'_k$, be a growth step or in a context of
growths. Then $k - i$ is at most a constant.

*Proof.* Since $r$ calls no rule, the evaluation of a call of $r$ that
contains an activation is the innermost call evaluation that contains it, so
$e_{l+1} = e'_l$ for $i \le l < k$, and every link evaluates $r$ at one
position $p$. As the shared call is the first, $w_l$ examines no input before
$p$, so it starts at $p$, and $\kappa_l$ reaches $e'_l$ from $p$ without
consuming input. Call the body evaluations in progress the frames. Let $N_l$
be the innermost evaluation that contains $\alpha_{l-1}$ and $\alpha_l$. Since
$w_l$ is the first abandoned evaluation below $N_l$ on the path to
$\alpha_{l-1}$, every evaluation above $w_l$ on that path, up to $N_l$,
succeeds and is not the expression of a lookahead (*Restarts*). $e'_l$, which
starts after $w_l$ ends and contains $\alpha_l$, lies in $N_l$. When the child
of $N_l$ that contains $w_l$ is not $w_l$, it succeeded, $N_l$ went on at its
end, and $e'_l$ lies in a later child, so that child ends at or before $p$.
From the end of $w_l$ to $e'_l$, the parser leaves the frames that enclose
$w_l$ inside $N_l$, and then starts frames at $p$ through calls on the way to
$e'_l$. Each frame that it leaves succeeds and ends at $p$: no later than that
child, which contains it, and no earlier than $p$, before which the parser
starts nothing after $w_l$. A frame is left once. In a frame that it leaves,
the call of the frame inside it, if $w_l$ is not directly in its body, lies
on the path, so it succeeded, and it ended at $p$, where the frame inside it
ended. The frame ends at $p$ too, so what it evaluated after the call, up to
the end of its body, succeeded without consuming input, and the call place
is followed in its caller by what can match without consuming input. The
unit is the first abandoned evaluation on the path, not one inside it: with
`X = (Y "x" / "")` and `Y = (R "a")?` at $p$, and the scan in `R`, the unit
that holds the scan is the alternative `Y "x"`, which lies directly in the
body of `X`, so `X` is left through no call that holds the unit. This
configuration is not a leaf link, since the unit begins with the call of
`Y`, which calls a rule; it shows only which evaluation is the unit.

Frames started at $p$ in this interval nest, through calls made before input
is consumed, at most $|\text{rules}|$ deep. A rule that recurs among them
recurs through a cycle of left calls, which passes through a leader ([Growths
of Left Recursion](../runtime/parsing.md#growths-of-left-recursion)), and a
call of that leader at $p$ is answered from the memo or by the current result
of its growth, which starts no frame, or grows it, which starts a growth step,
which the claim leaves out. By the argument of Lemma 1, and since an
iteration that starts at $p$ either consumes input or ends its repetition, a
body evaluation reaches each place of its body at $p$ at most a constant
number of times, and so makes at most a constant number $c'$ of calls at $p$.
A link that leaves no frame evaluates $r$ again in a frame in progress or one
started at $p$ inside it, at a later place of its body, so the links between
two that leave frames are at most a constant. For a link $l$ that leaves
frames, let $Y_l$ be the last frame that it leaves, $\pi_l$ the call place
through which it leaves $Y_l$, and $Z_l$ the frame of the caller, in which it
then reaches $e'_l$. The link is **outer** when $Y_l$ starts before $p$, and
**inner** when $Y_l$ starts at $p$; every frame that an inner link leaves lies
inside $Y_l$ and so starts at $p$.

From the end of $w_i$ to $e'_k$ the parser starts no evaluation at a position
before $p$: it starts none from the end of each $w_j$ to $e'_j$ (*Restarts*,
since $w_j$ starts at $p$), and from $e'_j = e_{j+1}$ to the end of $w_{j+1}$
it is inside $w_{j+1}$, which starts at $p$.

*Outer links.* Let $l < l'$ be outer links with $\pi_l = \pi_{l'}$. $Y_{l'}$ starts before $p$ and is
in progress at $e_{l'}$, after the end of $w_l$, so it was started before
$w_l$ ended, and it encloses $w_l$. It is still in progress at $e'_l$, so link
$l$ does not leave it: it is $Z_l$ or encloses $Z_l$. No evaluation on the
path from $Y_{l'}$ down to $Y_l$ is abandoned. Otherwise let $v$ be one. It
contains $\alpha_{l-1}$, which lies in $Y_l$, and not $\alpha_{l'}$, which
starts after $Y_{l'}$ ends. Let $m$ be the last index with $\alpha_m$ in $v$,
so that $l - 1 \le m < l'$ and $\alpha_{m+1}$ starts after $v$ ends. The
innermost evaluation $N_{m+1}$ that contains $\alpha_m$ and $\alpha_{m+1}$
encloses $v$, which is abandoned and lies on the path from $N_{m+1}$ to
$\alpha_m$, so $w_{m+1}$, the first such evaluation, encloses $v$ and with it
$Y_l$. But $w_{m+1}$ starts at $p$, and $Y_l$ before $p$. So every evaluation
on the path succeeds, is not the expression of a lookahead and is no dropped
step, and lies in the match of the one above it; since $Y_l$ and $Y_{l'}$
both end at $p$, so does each frame on the path. The parser leaves those
frames at $p$ after the end of $w_l$ and before $e'_{l'}$, so none is a growth
step or in a context of growths. Each call on the path ends at $p$ in a frame
that ends at $p$, so what follows it in its caller matched without consuming
input. These calls lead from the rule of $Y_{l'}$ down to the rule of $Z_l$,
and $\pi_l$ leads from there to the rule of $Y_l$, which is the rule of
$Y_{l'}$, the callee of $\pi_l$: they form a cycle of calls of one recursion,
each followed in its caller by what can match without consuming input. So
the caller of $\pi_l$ is in the set of rules that
defines nesting at one end for $\pi_l$ ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost)), since what follows
$\pi_l$ can match without consuming input, and so is each rule of the cycle,
which calls the next one at such a call, the callee of $\pi_l$ among them; so
$\pi_l$ nests at one end. *Nesting at one end* below proves this in general,
when abandoned evaluations lie on the path. The continuation of $w_l$ reaches
$e'_l$ in what follows $\pi_l$, after leaving the body of $Y_l$, so the call is
not shared, against the choice of the link. So no two outer links have one
call place, and the outer links are at most the number of call places.

*Inner links.* Each inner link leaves at least one frame that starts at $p$,
and a frame is left once, so the inner links are at most the frames started
at $p$ that the parser leaves from the end of $w_i$ to $e'_k$. Those started
before the end of $w_i$ enclose $w_i$ and form one chain, and those left form
its inner part. No rule recurs in that part: a leader's frame between two
frames of one rule, both started at $p$, would be a growth step left at $p$.
So at most $|\text{rules}|$ of them are left. Those started after the end of
$w_i$ are no growth steps, so no rule recurs among nested ones either, and
each is called at $p$ from a frame in progress: another one started after the
end of $w_i$, or a frame that the parser was in before. Such an earlier frame
is either the innermost frame started before $p$ that the parser is in, which
changes only when an outer link leaves it, so that at most the number of call
places plus one frames take that role, or a frame of the chain at $p$ that
the parser is in at the end of $w_i$ or returns to after leaving the frames of
the chain inside it, at most $|\text{rules}| + 2$ frames. Each of those makes
at most $c'$ calls at $p$, and so does each frame started after the end of
$w_i$, nested at most $|\text{rules}|$ deep, so the frames started after the
end of $w_i$ at $p$ are at most a constant. So $k - i$ is at most a
constant. $\square$

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

**Nesting at one end.** Let $t$ be a call of a rule $Y$ from a rule $Z$ of
its recursion, and $E_t$ the smallest set of rules that the definition of a
call that nests at one end ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost)) builds for $t$, so
that $t$ nests at one end when $Y$ is in $E_t$. Let $X$ be an evaluation of a
call of a rule $A$ of the recursion of $t$ that succeeds and ends at a
position $p_0$, and $T$ an evaluation of $t$ inside $X$, other than $X$, that
starts before $p_0$, ends at $p_0$ and holds a successful activation of a
scanning source other than a bounded scan. Let no evaluation on the path from
$X$ to $T$ be a dropped step of a growth, and let two evaluations at one
position of a call that a unit and a continuation share, or that the
comparison of a decided unit passes alike, end at one position. Then $A$ is in
$E_t$, except in the cases that the end of the proof leaves open. So when the
evaluations of $Z$ nest without a bound through $t$, each in the call $t$ of
the one around it, all of those calls ending at one position, each holding a
successful activation of such a source, and no dropped step of a growth lying
between them, $t$ nests at one end: calls that start at that position nest
only through left recursion, which the growths bound, so some call $X$ starts
before the position and holds another, $T$, that does too, with no dropped
step on the path between them, and the claim gives $Y$ in $E_t$. A nesting
with a dropped step of a growth between its calls is left open.

*Proof.* By induction on the number of calls on the path from $X$ to $T$.
Each call on that path is of a rule of the recursion of $t$, since $A$
reaches it and it reaches $Z$.

If no evaluation on the path below $X$ is abandoned, each succeeds and lies in
the match of the one above it, so each call on the path ends at $p_0$, in a
body evaluation that ends at $p_0$, and what follows it in its caller matched
without consuming input. A kept growth step on the path is the last kept step
of its growth, since it ends where the growth ends. So $Z$ is in $E_t$ by the
first case of the definition, as the caller of $T$, and each rule above it on
the path by the second, up to $A$.

Otherwise let $w$ be the first abandoned evaluation on the path below $X$. It
contains $T$, and with it the activation, which succeeded before $w$ was
abandoned, so $w$ is the expression of a lookahead or failed after one of its
scanning sources examined input. It is no dropped step, so it is a unit that
condition 2 checks, and it is committing. This is why the definition needs no
case for the units that can fail only before their scanning sources examine
input: no such unit lies on the path. Let $s < p_0$ be the start of $w$. The
evaluations above $w$ on the path, up to $X$, succeed and are kept, so after
$w$ the parser evaluates from $s$ a continuation $\kappa$ of $w$, through the
ends of the enclosing expressions and, at the end of a rule body, through the
call of the rule, and ends $X$ at $p_0$, returning to no position before $s$
in between (*Restarts*). $\kappa$ consumes the input from $s$ to $p_0$, so it
begins with the calls that $w$ and $\kappa$ share or that the comparison passes
alike, the **alike calls**, each starting at the end of the one before, which
end at the same positions in $w$ and in $\kappa$. Those of them that start
before $p_0$ lie in the match of $X$ in $\kappa$, so they end at or before
$p_0$.

$T$ does not start at or after the end $u$ of the alike calls in $w$. If it
did, $w$ would examine, after the alike calls, the character at $u$, on its
way to $T$ or in $T$, which consumes the input from its start to $p_0$, and
$\kappa$, which reaches $p_0 > u$ after the alike calls, would consume that
character too, which disjoint first characters exclude; and a unit has no
scanning source after the alike calls only if $T$ there holds none. So $T$ is
one of the alike calls, or lies inside one of them, or $w$ looks through it to
reach them, which then lie in its body. In each of these cases the first
alike call $S$ that ends at $p_0$ exists and starts before $p_0$:

- (a) $T$ is an alike call, and $S = T$.
- (b) $w$ looks through $T$: the last alike call inside $T$ ends at $p_0$, since
  otherwise $T$ would consume, after the alike calls, the character where it
  ends, which $\kappa$ consumes too; $S$ is inside $T$, and $w$ reaches it by
  looking through $t$.
- (c) $T$ lies inside an alike call, and no evaluation between them is
  abandoned: that call ends at $p_0$, at the end of $T$, and is $S$. Its evaluation in $w$ succeeds,
  ends at $p_0$ and holds $T$ with fewer calls between, so by induction its
  rule $B$ is in $E_t$.

$\kappa$ evaluates $S$ in the body of a call $K$ on the path from $X$ to $w$:
the call whose body holds $w$, or one whose body $\kappa$ reaches by leaving
the bodies inside it through their ends. $K$ is kept and contains $S$ in its
match, so it ends at $p_0$, no later than $X$ and no earlier than $S$, and
after $S$, $\kappa$ reaches the end of the body of $K$ without consuming
input. $\kappa$ evaluates $S$ either directly in the body of $K$ or inside
calls $Q_1, \dots, Q_m$ that it looks through, $Q_1$ made in the body of $K$
and each $Q_{i+1}$ in the body of $Q_i$, with $S$ in the body of $Q_m$. Each
$Q_i$ contains $S$ in its match, so it ends at $p_0$, and after $S$ the body of
$Q_m$, and after $Q_{i+1}$ the body of $Q_i$, match without consuming input.
In (a) and (b), the rule of $K$
is in $E_t$ by the third case of the definition, with the shared call reached
by looking through $Q_1$ when $\kappa$ evaluates $S$ inside it, in its form
for a continuation that left the body of the unit when $K$ is not the call
whose body holds $w$. In (c), the call $S$ of $B$ is followed by what matched
without consuming input in the body of $Q_m$, or of $K$ when $\kappa$
evaluates $S$ directly there, so the second case adds the rule of $Q_m$, then
those of $Q_{m-1}$ to $Q_1$, each calling the next at such a call, and the
rule of $K$; the rule of each $Q_i$ is of the recursion of $t$, since $K$
reaches it and it reaches $B$, which reaches $Z$. The calls on the path from $X$ to
$K$ end at $p_0$ in body evaluations that end at $p_0$, so the second case
adds each rule above $K$, up to $A$.

Three points of this proof are open. The hypothesis that the alike calls end
at one position in both evaluations is what
[Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)
states for the shared calls and the comparison of a decided unit for the
calls that it passes alike; inside a growth, two evaluations at the growing
position of a call of a rule of the growing cycle can read different growing
results, and that the shared calls of a unit and its continuation there end
at one position is argued from the unit and the continuation starting in one
step, where a read of the seed returns one seed, but not shown. The paragraph
that places $T$ among the alike calls is written for disjoint first
characters and for a unit without a scanning source after the alike calls;
for words that exclude the continuation and for decided units, the proof of
*Direct activations* shows the same for an activation after the word or the
decisive symbol, and it must still be shown for the end of $T$. And when $T$
lies inside an alike call in a unit abandoned inside it, that call can end
before $p_0$, and the cases above do not apply.

**Seed reads.** The following argues, without proving it, that the case of
seed reads in the definition is the one that *Nesting at one end* gives inside
growths. A left call $s$ from a rule of a left-recursion cycle to a rule
of the same cycle nests at one end when some call of a rule of that cycle
does. This is the case in which what follows $s$ is evaluated at a position
$p_0$, with a successful activation, by unboundedly many evaluations. That happens once in each growth step
at $p_0$ of an evaluation of a leader of the cycle whose current result ends
at $p_0$, and each such evaluation has a bounded number of steps there: the
growth stops at the first step that does not grow. So unboundedly many such
evaluations nest, each in the next, and since a new one starts at a new
position, it is made by a call whose callee is a rule of the cycle, as a left
call of a rule of the cycle starts no new evaluation at a new position. One of
the finitely many call places, $t$, makes unboundedly many of them, and
*Nesting at one end*, with $X$ an outer and $T$ an inner one of them, would put
the callee of $t$ in $E_t$, so that $t$ nests at one end. Besides the open
points of that claim, two of its hypotheses are not shown here. First, $X$
and $T$ must end at $p_0$, but the evaluations above are chosen because the
current result of their leader ends at $p_0$, and a leader whose step at
$p_0$ grows ends beyond $p_0$. Second, no dropped step of a growth may lie
on the path from $X$ to $T$, and that path lies inside growths: the claim
allows the kept growth steps on it, but the dropped ones are not excluded.
The claims above leave out the links between which the parser leaves or
starts a growth step at the position, and so do not use this case.

These claims bound the links of one source at one position, memoized links by
the memo and the links of a rule that calls no rule by the cycles of nesting
calls, where the parser leaves or starts no growth step and no evaluation in a
context of growths at the position between them. The claims below sum them over the positions, by charging the
activations of each later evaluation of a memoized rule at a position to an
earlier one.

**Owners.** A **memoized evaluation** is a body evaluation of a rule that the
parse memoizes (Lemma 3), made by a call, that is neither a growth step nor in a
context of growths, or the evaluation where the parse begins. The **owner** of
an activation is the innermost evaluation that contains it and is a memoized
evaluation, a growth step or an evaluation in a context of growths; the
evaluation where the parse begins contains every evaluation, so each activation
has one. Lemma 5 counts the activations owned by growth steps and by
evaluations in a context of growths. In a [Document](../runtime/documents.md)
parse, an entry from an earlier parse evaluates nothing, so the memoized
evaluations below are those of this parse; what its answer does beyond that is
no activation, and Lemma 12 counts it.

The **own region** of a body evaluation is the part of it outside the calls of
rules that the parse memoizes. The calls in it are of rules that the parse does
not memoize, which the memo never answers. An activation owned by a memoized
evaluation lies in its own region: a call of a rule that the parse memoizes,
made inside it, is answered, or evaluated as a memoized evaluation, as the
growth steps of a leader or in a context of growths.

**Own regions.** The own region of a memoized evaluation $M$ holds no growth
step and no evaluation in a context of growths.

*Proof.* A growth step evaluates the body of a leader, which the parse
memoizes, inside the call that grows it, so it lies outside the own region. Let
an evaluation $x$ in the own region be in a context of growths of a cycle at
$p$, so in a step of a growth of that cycle at $p$. That growth did not start
inside $M$, since one that did started at a call of a leader that encloses $x$,
outside the own region. So $M$ lies in the step and starts at $p$, since no
evaluation in a growth at $p$ starts before $p$
([Growths of Left Recursion](../runtime/parsing.md#growths-of-left-recursion))
and $M$ contains $x$ at $p$. $M$ is reached from the step through calls made
before input is consumed and reaches the rule of $x$ in the same way, so the
rule of $M$ is on the cycle and $M$ is in a context of growths, which a
memoized evaluation is not. $\square$

**Keys and classes.** The **key** of a call of a rule that the parse memoizes
is the variant, position, level and combination of the values of the variables
that the rule reads, as in the [memoization proof](memoization.md#setting) with
the variant added (see the counterparts below). Its **class** is the key
without the variant: a rule and its twin have keys of one class at a position.
An **evaluation of a key** is a body evaluation of a rule that the parse
memoizes, other than a leader, made by a call with that key that reads no
growth open at its start. A memoized evaluation is one, since a call reads only
growths of the cycle of its rule at its position ([Lemma 2 of the memoization
proof](memoization.md#lemma-2-reading-an-open-growth)) and a memoized
evaluation is in no context of growths. An evaluation in a context of growths
whose call reads no growing result is one too. The first evaluation of a key of
a class is the **head** of the class, and the evaluation where the parse begins
is a head of its own. By Lemma 3, a class has at most six memoized evaluations,
three of each variant. The parse memoizes a twin exactly when it memoizes its
rule ([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)).

An activation is **held** by an evaluation of a key when it lies in the own
region of that evaluation. It is held by at most one: an evaluation of a key
inside another is made by a call of a rule that the parse memoizes, outside the
own region of the other. An activation owned by a memoized evaluation is held
by it, by the paragraph above, so an activation that no memoized evaluation
holds is owned by a growth step or an evaluation in a context of growths.

**Counterparts in the reference parse.** The [memoization
proof](memoization.md#setting) has no variants: its key is a rule, a position, a
level and the values of the variables. It applies here to the grammar in which
each value-free twin is a rule of its own, whose body is the body of its rule
with, at each call, the rule or the twin that the twin calls there ([Value-Free
Twins](../core-ir/modules.md#value-free-twins)); the parse evaluates that
grammar, and keys its memo by its rules, so the key of the memoization proof is
the key above. Its reference parse $\mathcal{R}$ evaluates a twin wherever the
parse does. What this proof takes over to a twin beyond that is the statement
of [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules) that a
twin evaluates the body of its rule as the rule does: the same expressions at
the same positions, with the same matches and ends, predicates and bindings
that read the same values, and at each call the rule that the rule calls there
or its twin. Only what a twin does not build differs, its value, which no call
uses. A twin is on no left-recursion cycle, so it reads no growing result, and a
call of a rule of a cycle, which has no twin, is the same call in both. For an
evaluation $x$ of the parse, of
an expression, a call or a step of a growth, let $\mathcal{R}(x)$ be the
evaluation that the reference parse $\mathcal{R}$
makes in the state in which the parse makes
$x$: the same position, variables, captures, lookahead state, open growths and
current results. By (I1) of the [memoization
theorem](memoization.md#theorem-memoization-never-changes-the-result), each
evaluation of the parse has the outcome of its counterpart in that state, or in
lookahead state $1$ one that agrees with it and differs only in what reaches no
decision of the parser, so, by induction along the steps of $x$, the parse evaluates what $\mathcal{R}(x)$
evaluates, except inside the calls that it answers, the steps of a growth
that it does not evaluate and the alternatives that it skips by the next
character, and each evaluation that it makes inside $x$ is the counterpart of
one inside $\mathcal{R}(x)$. An alternative that the parse skips fails in
$\mathcal{R}(x)$ at its position without consuming input, inside a lookahead
or not, enters no repetition and makes no call other than those of rules that
are neither leaders nor Operators Rules on the way to a terminal that fails
there, each of which fails there in the same way ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)). So it holds no activation:
no iteration, no operator of an Operators Rule, no skip of a `#recover` and no
step of a growth or copying operation, at any depth of calls. Hence an
evaluation in $\mathcal{R}$ that holds an activation at any depth, such as a
call whose body evaluation holds one, lies in no alternative that the parse
skips. By [Lemma 1 of the memoization
proof](memoization.md#lemma-1-context), the evaluation in $\mathcal{R}$ of a
call that reads no growth open at its start depends only on its key and its
lookahead state, and the state reaches no decision of the parser, so by the
induction of that lemma the evaluations in $\mathcal{R}$ of one key in the two
states take the same steps: the same expressions at the same positions with
the same success and end, and calls with the same keys, which read a growth
open at their start in both or in neither. The variant of each of those calls
is the same in both, since it depends only on the place of the call and on the
variant whose body is evaluated ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)). The evaluations in
$\mathcal{R}$ of the two keys of one class, in either state, take the same
steps up to the variants of their calls: by the statement on twins above, by
induction along the steps, the same expressions at the same positions with the
same success and end, and calls of keys of one class, which read a growth open
at their start in both or in neither, since a call of a twin reads none and a
call of a rule of a cycle is the same in both.

**Replays.** Let $M$ and $M_0$ be evaluations of keys of one class, $M_0$ the
earlier, in lookahead states and variants that may differ. The evaluations in
the own region of $M$ correspond one to one, in order, to those in the own
region of $M_0$, apart from those inside an alternative that the parse skips by
the next character in one of the two and evaluates in the other, which holds no
activation: each is an evaluation of the same expression at the same
position, with the same success and end, and a call of a rule that the parse
memoizes, which either evaluates or the memo answers, corresponds to a call
with a key of the same class inside the same lookaheads of the body.

*Proof.* $M$ and $M_0$ are made by calls with keys of one class that read no
growth open at their start in the parse, and so in $\mathcal{R}$. A growth
open at the start of $M$ is read in $\mathcal{R}(M)$ by a call or step that
the parse evaluates inside $M$, or answers with the current result of that
growth, and then $M$ reads it too; or inside a call $y$ that the parse answers
from an entry, whose call read no growth open at its start ((S)), so that
$\mathcal{R}(y)$, which takes its steps ([Lemma 3 of the memoization
proof](memoization.md#lemma-3-answering-from-a-sound-entry)), reads only
growths that start inside $y$, after the start of $M$; or inside a call or step
that the parse answers from what it keeps or takes in a run of steps, which
counts as reading what it reads in $\mathcal{R}$ ((D)), so that $M$ reads it
too. An alternative that the parse skips by the next character reads no growth
in $\mathcal{R}$: the only calls that it makes there are of rules that are not
leaders, on the way to a terminal that fails at once (*Counterparts*). So their counterparts in $\mathcal{R}$ take the
same steps up to the variants of their calls. Of two corresponding calls, one is
of a rule and the other of the same rule or of its twin, so the parse memoizes
both or neither. In an own region every call is of a rule that the parse does
not memoize, which it evaluates, so the own regions of $M$ and $M_0$ are the
counterparts of the parts of $\mathcal{R}(M)$ and $\mathcal{R}(M_0)$ outside
their calls of rules that the parse memoizes and outside the alternatives that
the parse skips, and those parts take the same steps. An alternative that the
parse skips in one region and evaluates in the other fails in both at the same
position, and its evaluation in $\mathcal{R}$ holds no activation
(*Counterparts*). $\square$

**Replayed activations.** An activation held by a memoized evaluation $M$
corresponds to an activation of the same source at the same position, which
succeeds when it does, held by the head $H$ of the class of $M$, and distinct
activations held by $M$ correspond to distinct ones.

*Proof.* The activation lies in the own region of $M$, outside the alternatives
that the parse skips in it or in the own region of $H$: the alternatives around
it hold an activation in $\mathcal{R}(M)$, and so in $\mathcal{R}(H)$, which
takes the same steps (*Counterparts*). The replays then give the activation at
the same place in the own region of $H$. $\square$

This claim counts nothing. *Activations held by heads* below sums it over the
at most six memoized evaluations of a class, whether the head is a memoized
evaluation or in a context of growths, and bounds the activations held by heads
at each position by the links, with no count of the evaluations in a context of
growths or of the rules that are not memoized, so that bound rests neither on
Lemma 10, which rests on the activations, nor on the counts of Lemma 5 for the
contexts of growths.

**Answered calls.** Once the memo answers a call outside contexts of growths,
or stores the result of one, it answers every later call with the same key
outside contexts of growths in the same lookahead state or a greater one, from
an entry of this parse or, in a [Document](../runtime/documents.md), of an
earlier one.

*Proof.* A call evaluates only when no entry answers it, and a store replaces
the entry that a lookup of its key finds ([Setting](#setting)). An entry stored outside a lookahead answers the calls in
both states, and one stored inside a lookahead those inside one, condition (L)
of the [memoization proof](memoization.md#setting). A store outside a
lookahead therefore makes an entry that answers the calls of both states, and
a store inside one is made only when no entry answers the calls inside, so it
replaces no entry stored outside. So no store makes an entry that answers a
call stop answering the calls of its state, and an entry that answers a call
outside lookaheads answers those inside them too. Entries
stay until the end of the parse, apart from those that a stream discards before
the elements it has delivered, where no later call is made. $\square$

**Covered keys.** Let $x$ be an evaluation of the parse, a call or a step of a
growth, and let $z$ be $\mathcal{R}(x)$ or a call inside it, of a rule that the
parse memoizes other than a leader, that reads no growth open at its start.
Then the parse makes an evaluation of the key of $z$ before $x$ ends, unless,
on the path from $\mathcal{R}(x)$ down to $z$, followed into earlier
evaluations as below, (a) the parse answers a call from an entry of an earlier
parse of a [Document](../runtime/documents.md), or (b) the parse answers a call
or step, or takes it in a run of steps, from what it keeps apart from the
table, (D) of the [memoization proof](memoization.md#setting), and $z$ lies
inside a lookahead that it observed, or the result was kept in a smaller
lookahead state, which the open gap stated with (D) leaves out, or (c) the
parse skips by the next character an alternative on the path. Case (c) does
not arise when $z$ holds an activation in $\mathcal{R}$.

*Proof.* By induction on the order in which the parse completes its
evaluations. Each call $z'$ that the path below reaches in place of $z$ has the
key of $z$ and reads no growth open at its start, so it takes the steps of $z$
in $\mathcal{R}$ (*Counterparts*); when $z$ holds an activation, so does $z'$,
and so does every alternative on each segment of the path, which encloses
$z'$ or a call whose evaluation in $\mathcal{R}$ contains the counterpart of
that activation, so the parse skips none of them (*Counterparts*). Outside case (c), if the parse evaluates every call
and step on the path, $z$
included, its evaluation of $z$ is the counterpart of $z$, made before $x$
ends. It reads what $z$ reads, since a call that the parse answers from what it
keeps counts as reading what it reads in $\mathcal{R}$ ((D)), so it reads no
growth open at its start, and it is an evaluation of the key of $z$. Otherwise
let $y$ be the first call or step on the path that the parse does not evaluate.
The parse does not answer $y$ with the current result of an open growth, since
$\mathcal{R}(y)$ would then make no call, so $y$ would be $z$, which reads no
open growth. If it answers $y$ from an entry that it stored, the entry is that
of an earlier call $y'$ with the key of $y$, which it evaluated and which ended
before $y$, and neither reads a growth open at its start ((S), and [Lemma 3 of
the memoization proof](memoization.md#lemma-3-answering-from-a-sound-entry)),
so $\mathcal{R}(y)$ and $\mathcal{R}(y')$ take the same steps, and the path
goes on in $\mathcal{R}(y')$ to a call $z'$ with the key of $z$, or to
$\mathcal{R}(y')$ itself when $y$ is $z$. $z'$ reads no growth open at its
start: one that started inside $y'$ started inside $y$ too, where $z$ reads
none, and $y'$ reads none that was open at its own start. If the parse answers
$y$ from what it keeps, or takes it in a run of steps at once, the result is
that of an evaluation $y'$ of the parse that ended before $y$, with the same
key and present set (for a step, which (D) treats as a call, the same leader,
position and level and an equal current result to start from), which found each growth that it read with an equal
current result and each lookahead that it observed with the same observation;
outside case (b) it is in the same lookahead state.
By the argument of [Lemma 4 of the memoization
proof](memoization.md#lemma-4-answering-from-a-dependent-result),
$\mathcal{R}(y)$ and $\mathcal{R}(y')$ take the same steps outside the
lookaheads that $y'$ observed, and answer a call of a leader with a current
result in both or in neither, so outside case (b) the path goes on in
$\mathcal{R}(y')$ to a call $z'$ with the key of $z$. $z'$ reads no growth open
at its start: one that started inside $y'$ corresponds to one that started
inside $y$, and $z$ reads neither that nor one open at the start of $y$, which
$y$ finds open with the current result that $y'$ found. In both cases the induction hypothesis for
$y'$ gives an evaluation of the key of $z$ before $y'$ ended, and so before $x$
ends. What remains is case (a). $\square$

**Reference links.** $\mathcal{R}$ is the instance of the parse that stores
nothing, so the restarts, *Direct activations* and *Repeats* hold of it, with
its own dropped steps and its own paths that enter a growth again. Let $\beta'$
be a successful activation of $\sigma$ at $q$ in $\mathcal{R}$, $\beta$ the one
before it, and $w$ and $\kappa$ as in the restarts, with no dropped step and no
path that enters the growth again of $\mathcal{R}$ leading from $w$ to
$\beta'$. Let $e^k$ be the first of the calls that $w$ and $\kappa$ share whose
evaluation inside $w$ holds a successful activation of $\sigma$ at $q$, let
$\beta_f$ be the first such activation in it, and let no shared call up to
$e^k$ read a growth open at its start, neither in its evaluation inside $w$
nor in the one that $\kappa$ makes. Then $\kappa$ evaluates the call of $e^k$ again at its position,
as $e^{k\prime}$, which takes the steps of $e^k$ up to the variants of its
calls, and $\beta'$ is the activation of $e^{k\prime}$ that corresponds to
$\beta_f$. The pair $(e^k, e^{k\prime})$ is the **reference link** of $\beta'$;
it joins $\beta'$ to $\beta_f$, which need not be $\beta$.

*Proof.* By *Repeats*, $\beta$ lies in a shared call, so $e^k$ exists.
$\mathcal{R}$ evaluates every call, and $\kappa$, every path of which that
examines the input begins with the shared calls, evaluates them in order, each
at the position where it ends in $w$ (case 2). The evaluation in $\kappa$ of a
shared call up to $e^k$ and the one inside $w$ read no growth open at their
start, so they take the same steps up to the variants of their calls
(*Counterparts*): a shared call before $e^k$ holds no successful activation of
$\sigma$ at $q$ in $\kappa$ either, and nor does $e^{k\prime}$ before the
activation that corresponds to $\beta_f$. Nothing that the parser evaluates between the end of $w$ and that
activation holds one either, since $\beta'$ is the first after $\beta$, which
lies in $w$. So $\beta'$ is that activation. $\square$

The hypothesis on the growths is not derived from the conditions: the
evaluation of a shared call in $\kappa$ starts after $w$ ends, inside the
growths that start on the way to it, which are not open at the start of its
evaluation inside $w$. An activation for which a shared call up to $e^k$ reads a growth open at its
start, in either evaluation, has no reference link here; step 1 of [Open
Steps](#open-steps) leaves those activations to count.

**Heads in the reference parse.** Let $Z$ be an evaluation of a key in the
parse, the head of its class. Then, outside cases (a) and (b) of the covered
keys, no evaluation in $\mathcal{R}$ of a key of that class that reads no
growth open at its start and holds an activation at any depth ends before
$\mathcal{R}(Z)$ starts.

*Proof.* Let $z$ be one. Follow the path from the evaluation of $\mathcal{R}$
where the parse begins down to $z$. Every alternative on it encloses $z$,
which holds an activation, so the parse skips none of them (*Counterparts*).
If the parse evaluates every call and step
on it, its evaluation of $z$ is an evaluation of a key of the class that ends
before $Z$ starts. Otherwise let $y$ be the first call or step that it does not
evaluate, which it answers.
$\mathcal{R}(y)$ contains $z$ and not $\mathcal{R}(Z)$, whose call the parse
evaluates, so $\mathcal{R}(y)$ ends before $\mathcal{R}(Z)$ starts, and the
parse answers $y$ before $Z$ starts. By the covered keys with $x = y$, where
case (c) does not arise since $z$ holds an activation, the
parse made an evaluation of the key of $z$ before $y$ was answered. Either way
$Z$ is not the first of its class. $\square$

The claim needs the activation. In
`main = (a "x" / "") a? "y"; a = "q" b; b = [a-z]*` on `y`, the parse skips the
alternative `a "x"` by the next character, so its first evaluation of `a` at
$0$ is the one that `a?` makes, the head of its class, while $\mathcal{R}$
evaluates `a` at $0$ in the skipped alternative first, reading no growth and
holding no activation, and ends that evaluation before the head starts.

**Reference links lead to repeated evaluations.** Let the reference link
$(e^k, e^{k\prime})$ of $\beta'$ be of a rule that the parse memoizes, and let an evaluation $Z$ of a
key in the parse hold the activation whose counterpart is $\beta'$. Then $Z$ is
not the head of its class, outside cases (a) and (b) of the covered keys. This
holds whether or not the parse evaluates the call of $e^{k\prime}$.

*Proof.* $\mathcal{R}(Z)$ holds $\beta'$, which lies in $e^{k\prime}$, made by a
call of a rule that the parse memoizes, so $\mathcal{R}(Z)$ is $e^{k\prime}$ or
lies inside it, and the call $z^\circ$ at the same place in $e^k$, which takes
the same steps up to variants, has a key of the class of $Z$ and holds an
activation at the place of $\beta'$. It reads no
growth open at its start: $e^k$ reads none that was open at its own start,
and one that started inside $e^k$ corresponds to one that started at the same
place inside $e^{k\prime}$, which $\mathcal{R}(Z)$, taking the same steps as
$z^\circ$, does not read, since the call of $Z$ reads no growth open at its
start. And $z^\circ$ ends before $e^{k\prime}$ starts, so before $\mathcal{R}(Z)$ starts. By the
heads in the reference parse, $Z$ is not the head of its class. $\square$

**Links lead to repeated evaluations.** Let the $i$-th link of $\sigma$ at $q$
be memoized, and let every growth open at the start of $e_i$ or of $e'_i$ be
open at the start of both. If an evaluation $Z$ of a key holds $\alpha_i$, it is not the head of its
class, unless case (a) or (b) of the covered keys arises on the path from
$\mathcal{R}(e_i)$ down to the call that corresponds to $Z$. Case (a) arises
only at a call that is in a context of growths, or inside a lookahead while
the call of its class that corresponds to it on the way from $e'_i$ to $Z$ is
outside them, or whose corresponding call is in a context of growths or has the
other variant; so it does not arise when $e_i$ and $e'_i$ have one variant, the
lookahead state of $e_i$ is no greater than that of $e'_i$ and no call on
either way is in a context of growths.

*Proof.* $e_i$ and $e'_i$ are evaluations of one call at one position, of a
rule that reads no variable, so their calls have keys of one class: the call
in a committing unit and the call in its continuation can differ in variant, one
evaluating the twin where its value is discarded and the other the rule. A
growth open at the start of both contains both, and so the innermost
evaluation that contains $\alpha_{i-1}$ and $\alpha_i$, which is no growth
(*Restarts*), so that evaluation lies in one step of the growth, during which
its current result does not change. So the two calls find the same growths
open with the same current results. When neither reads one,
$\mathcal{R}(e_i)$ and $\mathcal{R}(e'_i)$ take the same steps up to the
variants of their calls (*Counterparts*). Otherwise the rule of the link is a leader, which
has no twin, and they take the same steps by the argument of [Lemma 4 of the
memoization
proof](memoization.md#lemma-4-answering-from-a-dependent-result): the state
that their steps read differs at most in the lookahead state, which reaches no
decision of the parser (*Counterparts*), and the growths open at their start,
which are the same with the same current results. $Z$ holds $\alpha_i$, which lies in $e'_i$, and $e'_i$ is
made by a call of a rule that the parse memoizes, so $Z$ is $e'_i$ or is
evaluated inside it. So $Z$ is the counterpart of $\mathcal{R}(e'_i)$ or of a
call $z$ inside it, and $\mathcal{R}(e_i)$, or a call $z^\circ$ at the same
place in it, has a key of the class of $Z$. $Z$ is not $e'_i$ when the rule of
the link is a leader, since an evaluation of a key is of no leader, and $z$
reads no growth open at its start, since the call of $Z$ reads none.
$z^\circ$ reads none either: a growth open at its start was open at the start
of $e_i$, and so at that of $e'_i$ with the same current result, or started
inside $\mathcal{R}(e_i)$ and corresponds to one that started at the same
place inside $\mathcal{R}(e'_i)$, and $z$, which takes the same steps, reads
neither. $\mathcal{R}(Z)$ holds the counterpart of
$\alpha_i$, and $\mathcal{R}(e_i)$ takes the steps of $\mathcal{R}(e'_i)$ up to the variants of its calls, so the call
that corresponds to $Z$ holds an activation at the same place, and case (c) of
the covered keys does not arise. By the covered keys, outside cases (a) and (b),
the parse made an evaluation of that key before $e_i$ ended, so before $e'_i$
started, which is made after the unit that contains $e_i$ is abandoned
(*Repeats*), and $Z$ is not the head of its class.

Each call $u$ on the path from $\mathcal{R}(e_i)$, followed into earlier
evaluations, corresponds through the same steps to a call $\hat{u}$ with a
key of its class on the path from $\mathcal{R}(e'_i)$ down to $z$, which the parse
evaluates, since $Z$ lies inside it, after $e_i$ ended. The path goes from
$\mathcal{R}(y)$ to $\mathcal{R}(y')$ only where an entry stored in a state no
greater than that of $y$ answers $y$, (L), or where a result kept in the same
state does, so the lookahead state of $u$ is no greater than that of $\hat{u}$
when that of $e_i$ is no greater than that of $e'_i$. If an entry of an
earlier parse answers $u$, outside contexts of growths, then by *Answered
calls* the memo answers every later call with the key of $u$ outside contexts
of growths in the state of $u$ or a greater one, and it does not answer
$\hat{u}$, so $\hat{u}$ is in a smaller state, in a context of growths or of
the other variant, which only a link whose evaluations differ in variant can
reach. $\square$

**Links to heads.** Let $\alpha_1, \alpha_2, \ldots$
be the successful activations of $\sigma$ at $q$ outside case 5 and dropped
steps, as above. Every link is memoized at both calls or a call of a rule that
calls no rule and is not memoized at its calls, a **leaf link**. The rule $r$
of a leaf link calls no rule, so the body evaluation of $R$ that contains
$\alpha_{i-1}$, which lies in $e_i$, is $e_i$ itself, and $r = R$; a memoized
link between two leaf links returns to the position of an earlier activation
(*Memoized links keep positions* below). Let $R$ call a rule or be memoized at
its calls, so that every link is memoized.

An evaluation of a memoized link reads a growth open at its start only when the
rule of the link is a leader that grows inside a growth of its cycle at the
position of the call and reads it. A rule on no cycle reads no growth (Lemma 2
of the memoization proof), and a rule of a cycle other than a leader is
memoized at a call only when the call is not a left call, which has no growth
in its context (case 2). A call of a leader that is evaluated, as the calls of
$e_i$ and $e'_i$ are, grows it, since a growth of the leader open at the
position would answer it, so it can read only another growth of its cycle at
the position. A growth open at the start of both evaluations contains the
innermost evaluation that contains $\alpha_{i-1}$ and $\alpha_i$, so a growth
open at the start of only one of them started inside that evaluation, at the
position of the link (Lemma 2 of the memoization proof): on the way to $e_i$,
ending before $e'_i$, or on the way to $e'_i$ after $e_i$ ended.

So by the claim above every linked $\alpha_i$ that is held by a head follows a
link of one of two kinds: a link of a leader with a growth open at the start of
one of its evaluations and not at that of the other, or a link with case (a) or
(b) of the covered keys on its path. A free $\alpha_i$ other than $\alpha_1$
is the counterpart of a successful activation of $\mathcal{R}$ at $q$. That
activation has no reference link when a dropped step or a path that enters a
growth again of $\mathcal{R}$ leads to it, or when a shared call up to the one
that holds the first activation repeated, inside the unit or in the
continuation, reads a growth open at its start. When it has one, of a memoized
rule as every link here is, by *Reference links lead to repeated evaluations*
$\alpha_i$ is held by a head only with case (a) or (b) on its path.
When no link of $\sigma$ at $q$ is of these kinds and every free $\alpha_i$
other than $\alpha_1$ is the counterpart of an activation with a reference
link, with neither case on its path, at most one of the
$\alpha_i$ is held by a head, memoized or in a context of growths. This claim counts nothing: it rests on the links and
on the claims before it other than *Activations that do not succeed*, and holds
whoever owns the $\alpha_i$.

**Memoized links keep positions.** Let $R$ call no rule and not be memoized at
its calls, and let $P_i$ be the body evaluation of $R$ or of its twin in which
$\alpha_i$ is direct, at $p_i$. Let $i$ be linked by a memoized link, let
neither $e_i$ nor $e'_i$ read a growth open at its start, and let case (a) or
(b) of the covered keys not arise on the path from $\mathcal{R}(e_i)$ to the
evaluation named below. Then before $e_i$ ended the parse made a body
evaluation of $R$ or of its twin at $p_i$ with a successful activation of
$\sigma$ at $q$ direct in it, so $p_i$ is the position of the body evaluation
of an earlier successful activation of $\sigma$ at $q$: $p_i = p_m$ for some
$m < i$, or the position of one that case 5 or a dropped step repeats. If $i$ is linked
by a leaf link, $p_i = p_{i-1}$.

*Proof.* A leaf link has $e_i = P_{i-1}$ and $e'_i = P_i$. For a memoized link,
as in *Links lead to repeated evaluations*, $\mathcal{R}(e_i)$ and
$\mathcal{R}(e'_i)$ take the same steps up to the variants of their calls, and
$P_i$, evaluated inside $e'_i$, is the counterpart of a body evaluation of $R$
or its twin at $p_i$ inside $\mathcal{R}(e'_i)$ with a successful activation of
$\sigma$ at $q$, so $\mathcal{R}(e_i)$ holds one, $\rho$, at the same place. By
the induction of the covered keys with $\rho$ in the place of $z$, the parse
makes, before $e_i$ ends, a body evaluation of $R$ or its twin at $p_i$ with a
successful activation of $\sigma$ at $q$: where it evaluates every call and
step on the path it evaluates the call of $\rho$ too, since the memo answers no
call of a rule that it does not memoize; a call on the path answered with the
current result of an open growth makes no call in $\mathcal{R}$ and holds no
body evaluation of $R$; an entry or a kept result leads to an earlier
evaluation of a key of the class, as in the covered keys; and a skipped
alternative holds no activation. That activation is one of the $\alpha_m$ or
one that case 5 or a dropped step repeats, which the $\alpha_m$ leave out, and
it ends before $e_i$ ends, so before $\alpha_i$ starts, and $m < i$ in the
first case. $\square$

**Nothing in a memoized link is a head.** Under the hypotheses of *Links lead
to repeated evaluations*, outside cases (a) and (b) of the covered keys, no
evaluation of a key that is $e'_i$ or lies inside it and holds an activation
at any depth is the head of its class. *Proof.* The proof of *Links lead to
repeated evaluations* uses of $Z$ only that it is $e'_i$ or inside it and that
it holds an activation, $\alpha_i$ there, whose counterpart at the same place
in $\mathcal{R}(e_i)$ excludes case (c) of the covered keys. Another
activation at any depth of $Z$ does the same. $\square$

So the body evaluations of $R$ that hold the $\alpha_i$ start at most at one
position for each free $i$, each link of the two kinds that *Links to
heads* leaves out, each memoized link whose two evaluations read a growth open
at their start, which *Memoized links keep positions* leaves out, and each
activation that case 5 or a dropped step repeats. For a leaf link $j$ whose shared call is the first, so that
$w_j$ starts at $p_j$, a head that holds $\alpha_j$ starts at $p_j$ or before
it. The heads that start at $p_j$ are of the classes at $p_j$, one head each.
Those that start before $p_j$ enclose $w_j$, and the frames between are left at
$p_j$; for two of them, the later is in progress at the earlier's activation
and encloses it, or it starts after that activation and strictly encloses
$e'_i$ for a memoized link $i$ between the two. This alternative is stated
here without its proof, which would rest on the restarts and the claim above.
Its proof, a bound on these heads and a bound on the activations that one head
holds are part of step 1 of [Open Steps](#open-steps).

**Activations held by heads.** Let no link of $\sigma$ at $q$ be of the two
kinds above, and let every free $\alpha_i$ other than $\alpha_1$ be the
counterpart of an activation of $\mathcal{R}$ that has a reference link with
neither case (a) nor (b) on its path, or that a dropped step or a path that
enters a growth again of $\mathcal{R}$ leads to. Then, by the replayed
activations and since a class has at most six
memoized evaluations, the successful activations of
$\sigma$ at $q$ held by memoized evaluations are at most six times those
held by heads, which are $\alpha_1$, those that case 5 or a dropped step
repeats, and the free ones that a dropped step or a path that enters a growth
again of $\mathcal{R}$ leads to, which step 1 of [Open Steps](#open-steps)
leaves to count. The free ones without a reference link because a shared call
reads a growth open at its start, and those whose reference link meets case
(a) or (b), are outside the hypothesis, and step 1 leaves them open too. Every activation owned by a memoized evaluation is held by it, so
these bound the successful activations of $\sigma$ at $q$ that memoized
evaluations own, apart from those that case 5 or a dropped step repeats, which
Lemma 5 counts. The comparison with $\mathcal{R}$ needs no order between the
lookahead states of $e_i$ and $e'_i$ and no common variant, and passes through
the growths that $e'_i$ makes again outside lookaheads after $e_i$ made them
inside one, through those that the leader of the link makes again there, and
through the growths open at the start of both evaluations of the link.

This gives the case that the comparison misses without the classes: in
`main = (-r "x" / r "y")* $$; r = [a-c]* t?; t = "d"` on `abcy`, the first
iteration at $0$ is direct in the evaluation of the twin of `r` at $0$, made by
`-r`, and the second in the evaluation of `r` at $0$, made by `r "y"` after the
unit `-r "x"` fails. Their keys differ in variant, so each evaluation is the
first of its key, but the second is not the first of the class, and its
activations are charged to those of the first.

Three parts remain part of step 1 of [Open Steps](#open-steps). When $R$ calls
no rule and is not memoized at its calls, the $\alpha_i$ held by heads are at
most the free ones, $\alpha_1$ among them, the links of the two kinds and the
leaf links that lead to one, and a leaf link can leave a
memoized evaluation that is not a head through the end of its body and reach
the call again in the evaluation around it, which can be a head. A link of a
leader with a growth open at the start of only one of its evaluations compares
two growths whose steps read different results of another leader of the
cycle: the unit can reach $e_i$ inside a step of a growth of that leader which
it starts at the position, from a shorter growing result than the last, and
the continuation reach $e'_i$ after that growth has ended, where the memo
answers the call of that leader with the final result, so that the calls
inside $e'_i$ after it are made at other positions than those inside $e_i$.
And in a Document parse a link can
meet an entry of an earlier parse that answers a call in a context of growths,
or a call whose corresponding call in the later evaluation is in a smaller
lookahead state, in a context of growths or of the other variant, and a link
that meets a result kept in a context of growths can lead into a lookahead that
the result observed, or meet one kept in a smaller lookahead state, which the
open gap stated with (D) of the memoization proof leaves out; such a link can
lead to an activation held by a head.

## Lemma 5 (Growth)

*Outline.* Under conditions 2, 3 and 5, and with the growths at the calls
that are not left calls bounded as scanning sources by the claims of Lemma 4
that count nothing, the
evaluations that Lemma 3 leaves out for memoized rules number at most a
constant times $K \hat{n}$: the growth steps, and the body evaluations of
memoized rules in a context of growths, those of a rule of a cycle other than
a leader outside a growth step included. Their local and scanned cost is at
most a constant times $K(L + 1)\hat{n}$ more than Lemma 4 counts. Lemma 4
counts the activations owned by memoized evaluations, those inside a growth
step included, and this lemma those owned by growth steps and by evaluations in
a context of growths, and the successful activations that a dropped step of a
growth or a path that enters the growth again repeats. Its bounds come before
the count of Lemma 4 on the activations that memoized evaluations own, in the
order that [Lemma 4](#lemma-4-scanned-cost) states, so they must not rest on
it: they rest on the claims of Lemma 4 that count nothing, from *Failure after
a source* to *Nothing in a memoized link is a head* other than *Activations that do not succeed*, on
Lemmas 1 to 3 and on the memo. Outside contexts of growths, the memo bounds the
growths of a leader at one position, level and combination of the values of the variables that it
reads by two, one inside a lookahead and one outside them, since a growth
stores its result ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)), which answers the later
calls there in the same lookahead state or a greater one.

A growth at $p$ takes at most two more steps than the length it consumes
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)). By
condition 3, no step evaluates a
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
constant at $p$. In those steps the activations of $u$ are owned by the growth
step that evaluates it or by an evaluation in a context of growths inside that
step, apart from those inside a memoized evaluation there, which Lemma 4
counts. This lemma counts the others (step 4 of [Open Steps](#open-steps)), from
the claims of Lemma 4 that count nothing and not from its count of the
activations that memoized evaluations own.

The claim covers the pairs that share no call. The next claim shows that a pair
that shares a call under the definition of [Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost) is, in the steps that the
claim does not except, a growing unit followed by a growing unit, apart from one
case that step 2 of [Open Steps](#open-steps) keeps.

**Calls shared with a growing unit.** Let $u$ and $g$ be as in the claim above,
except that they share a call, and let a step of the growth at $p$, other than
the first and the one whose seed is empty, evaluate $u$ at $p$. Then the first
call that they share is a call at $p$ of a leader of the cycle, which $u$ makes
before it evaluates anything that examines the input, through left calls of
rules of the cycle. If $u$ calls no rule before that call other than those
through which it makes it, $u$ is a growing unit, and the step evaluates it
as one: its left call reads the seed or grows a leader in the context of the
growth.

*Proof.* In the step, the first thing that $g$ evaluates is its left call at
$p$, directly or through calls of rules of the cycle: a left call of the
leader that grows at $p$ reads the seed, and one of another leader grows it in
the context of that growth. A call of a rule of the
cycle other than a leader at a left call is not memoized at the call, since
such a rule is memoized only at calls that are not left calls, and it calls a
rule, so it is not a call that a unit and a continuation share; the first thing
that its body evaluates is again a left call. So the first call that $g$
evaluates and that can be shared is a call of a leader $R'$ at $p$, and $g$
examines no input before it, so no shared literal comes before it either. By
the definition of sharing, $u$ makes the same call at $p$ before anything else
that examines the input, directly or as the first thing that the bodies of the
calls that it starts with evaluate. The body that holds $u$ started at $p$,
since the step started at $p$ and positions do not decrease, so each call on
the way from that body to the call of $R'$ is made before its caller's body
consumes input and is found by the analysis of left calls
([Growths of Left Recursion](../runtime/parsing.md#growths-of-left-recursion)).
Each rule on the way is reached from the rule of $u$ through such calls and
reaches it through $R'$, which is on the cycle, so it is on the cycle, and each
of the calls is a left call. If $u$ calls no other rule before the call of
$R'$, the first evaluation in $u$ on the path of the step that examines the
input or calls a rule is the first call on that way, a left call, and what
comes before it examines no input and calls no rule, as a predicate does. The
definition of a growing unit counts such a unit, and when what comes before
the left call can also consume input on another path, the unit is checked
both as a growing unit and as one that is not growing. $\square$

So in the steps that the claim does not except, an evaluation of $u$ at $p$
that calls no other rule before the shared call, as the alternative
`{ok} E R "!"` before `E "+" T` does, is an evaluation of a growing unit,
which the claim leaves out and the paragraphs below count with the
other growing units. What remains is a unit that calls, before the shared call
of the leader, a rule whose evaluation examines no input, as `Z` in an
alternative `Z E R "!"` with `Z = ""`. It is no growing unit by the
definition, since its first call is not a left call, yet in the step it
examines the input from the end of the result of the leader, as a growing unit
does, and condition 2 checks it only as an alternative that is not growing,
against the later alternatives of its choice and not against what follows the
calls that start the growth. Step 2 of [Open Steps](#open-steps) keeps this
case.

The growing units of a step examine the input from the end of the seed. The
input from the end of one seed to the end of the next is examined by one
step, and what a step examines beyond the end of its own result is the
failure of a growing unit after a scanning source, which condition 2 checks.
A growth is itself a scanning source at each call of a rule of its cycle
other than a left call, so its repetitions are those of a scanning source: the
restarts and the links of Lemma 4, which count nothing, relate the repetitions
at one position, and *Memoized links* bounds the links of one leader at one
position and level; the sum over the positions is step 4 of [Open
Steps](#open-steps).

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
condition 2 checks as any other unit. Everything this lemma says of a growing
unit rests on its left call reading the seed, which holds only on the paths that
reach the unit without consuming input. On a path that consumes input first, as
through the `"y"` in `E = "y"? (E / "z") "q" / "x"`, the unit starts at a new
position with nothing to grow from there, and a failure inside it is a failure of
a plain alternative, which only the cases of Lemma 4 bound. The path can consume
that input in a caller as well: in `r = "y"? f "q" / "x"; f = r / "z"`, the call
of `f` after the `"y"` is outside every growth of the cycle at its position, so
`f` and the rules that it reaches through left calls evaluate their growing units
there as plain alternatives. So condition 2 also checks such a unit as an
alternative that is not growing, against its plain continuations, whether the
part that can match nothing is in the unit's rule or before a call that reaches
it. In the first step of a leader R, every
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
  so the growth happens at most twice for each position, level and combination
  of the values of the variables that R reads, once inside a lookahead and once
  outside them
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

A copy of the value of a variable costs at most $L$; a copy of a read whose
binding binds a value that includes the value of a call on a recursion is
checked by condition 4 as a copy of that value, by the rule of the binding.
Condition 1 keeps the
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
largest element; a member is at most its value, or, for a field that was never set,
a zero value, which is a term with $q = 1$ (see below); a `filter` holds at most its
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
  input outside its match; a member that reads a field that was set, and an element, are parts of one, and a
  `filter` keeps at most its list; a member that reads a field that was never set
  is a zero value, which holds no input (see below). A match of a rule outside $\mathcal{S}$
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
that encloses it. A place inside the operands or the operator parts of an
Operators Rule is evaluated also in the tries of those parts that fail, which
are in no activation, so for such a place the claim rests on step 3 of [Open
Steps](#open-steps), as in Lemma 2.

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

The rules that are not memoized are those of the first exception of
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules). The places
that call a rule are those in the grammar, so a rule that the start rule does
not reach can only add places and make a rule memoized. A start rule that its
own body calls, directly or through rules called from one place, is memoized,
and a parse begins only at a start rule, on every interface
([Start Rule](../runtime/parsing.md#start-rule)). A leader is memoized in every
case.

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
its initial value or of its lambda; an element, and a member of a field that
was set, are components of the value that they are taken from; and a member of
a field that was never set is the [zero value](../language/types.md#zero-values)
of the declared type of the field, built by the operation. A zero value is
a scalar, the empty list, nil, a struct with no field set, or a tuple of such
values, and it depends on the declared type alone, so its size is at most the
number of components of that type, a constant of the grammar, and it holds no
input: making one costs that constant whatever the value that it is read from
holds. So a component that one term builds has
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

*Values of variables.* Call a sequence of evaluations of bindings a
**chain** when each evaluation after the first reads a value built, through
captures, rule values and the reads of variables, from the value that the
one before it binds. Let each binding occur at most a constant number $b$ of
times in every chain. Then $L$ is at most a constant times $\hat{n}$.

*Proof.* Every value that a binding binds is built from literals, values of
matches, and the values of variables that its term reads, which earlier
evaluations of bindings, or the initial values, gave. Follow the
values of variables back through the bindings that built them: by the
assumption, a chain of such evaluations is no longer than $b$ times the
number of bindings of the grammar. By *Top sizes*, with the
values of variables that a term reads added to its operands, a binding binds
a value of top size at most a constant times $\hat{n}$ plus a constant
times the top sizes of the values of variables that it reads, since a match
consumes at most $n$ and a capture in a positive lookahead at most the rest
of the input. Induction along the chains gives a bound of a constant times
$\hat{n}$. $\square$

The assumption is what conditions 1 and 4 state together: condition 1 for a
binding whose value can reach a later evaluation of the same binding through
the bindings that its rule keeps in effect, and condition 4 for a value that
reaches the caller through the value of the rule and a read of a variable.
That these are the only ways in which the value reaches a later evaluation
of the binding is step 5 of [Open Steps](#open-steps). The theorem does not use
the bound on $L$: its cost carries the factor $L + 1$, and
[Classification](../runtime/parsing.md#classification) derives from the
bound that the second class costs at most a constant times $K \cdot n^2$.

## Lemma 13 (Bounded Scans)

The activations of each [bounded
scan](../runtime/parsing.md#conditions-for-linear-cost) number at most a
constant times $K \hat{n}$. The proof uses Lemma 3, Lemma 15 for the region
bound, and the definitions: none of the conditions 1 to 5, and neither Lemma 4
nor Lemma 5. It is a proof for the
cost view, and holds of the written grammar by the [Cost View](cost-view.md)
proof.

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
with $X$ the alphabet of $\sigma$. The run bound of a bounded scan says
$d_\sigma \le 32$.

**Starts.** Let $\sigma$ have the run bound. The evaluations of $\sigma$
that reach a position $q$, by an activation at $q$ or by ending there, start in at
most $d_\sigma + 2$ positions. An evaluation at $b < q$ that reaches $q$ consumed
$[b, q)$ in its iterations, so those characters are in $X$. The character at $b$ is
in $X$, so $r(b) = \rho(b) \le d_\sigma$. Let $s = q - r(q)$. Then $s \le b$, and
$r(b) = b - s$, so $b$ is one of the $d_\sigma + 1$ positions from $s$ on. The other
start is $q$. When $\sigma$ has the region bound instead, they start in at
most 2 positions by Lemma 15, and below $d_\sigma$ stands for 0 for such a
$\sigma$, in place of the value of the pass.

**Rigid expressions.** For a rigid expression $e$ there is a constant $w_e$ such
that the evaluations of $e$ that end at a position start in at most $w_e$
positions. $w_e$ is 1 for a lookahead or another expression that consumes
nothing. A terminal matches a fixed text or at most one character, of at most
four bytes. A sequence multiplies the $w$ of its parts, a choice adds them, an
option adds 1, and a bounded repetition of at most $m$ iterations adds the powers
of $w$ up to $m$, since each iteration is a part of a sequence. A repetition with
the run bound or the region bound has $d_\sigma + 2$ by the claim. A call has the $w$ of the body of
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
[cost view](../runtime/parsing.md#cost-view), which removes an evaluation only when
making it again costs a constant factor. That the written grammar costs at most a
constant factor more than the view is the condition of the cost view, which the
[Cost View](cost-view.md) proof shows; the bounded scans make no value, so this
lemma holds of the written grammar.

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

## Lemma 15 (Regions)

Let the classifier have chosen the [region rules](../runtime/parsing.md#conditions-for-linear-cost)
with their four conditions: fixed ends, no nesting, disjoint openers and guarded crossings. Then
no evaluation outside the regions, other than one inside a check, starts at a position $x$ with
$a < x < \mathrm{end}_R(a)$, where a region rule $R$ is called at $a$ and $\mathrm{end}_R(a)$ is
where that call ends. So the evaluations of the body $\sigma = X^*$ of a trivia rule with the
region bound that reach a position $q$ start in at most 2 positions. Like Lemma 13, it is a proof
for the cost view, on whose rule bodies the classifier decides the conditions.

**The reference parse.** The proof is of the reference parse $\mathcal{R}$ of
[Memoization](memoization.md#setting), which evaluates every call. A memoized parse
evaluates the same expressions at the same positions, in the same states, less the evaluations
inside the calls that the memo answers: by (I1) of the theorem of Memoization an answered call
has the outcome that $\mathcal{R}$ gives it, so by induction on the order of evaluation what the
parse evaluates next is what $\mathcal{R}$ evaluates next once it leaves that call. The claim is
about where evaluations start, so it holds of every subset of the evaluations of $\mathcal{R}$:
the parses of the three backends of the engine, of a generated Go parser, which evaluates the same
expressions under the same memo policy, of a [stream](../runtime/streaming.md), whose commits
and default budget, which grows with the input, end a parse or discard entries but change no
evaluation that it makes, and of a [Document](../runtime/documents.md), whose reused results
answer calls as memo entries do. A parse with missing-token insertion is outside the classes.

**Fixed ends.** No rule that a region rule reaches reads a variable, a level, a capture or a
growth, and a cut in one commits a choice within the evaluation of its own rule, so by Lemma 1 of
[Memoization](memoization.md#lemma-1-context), whose key is then the rule and the position, whether an expression in such a rule matches at a position, and
where it ends, is a function of the position and the input. Write $\mathrm{end}_R(a)$ for where
$R$ ends at $a$, which is $a$ itself when $X$ fails at $a$ for a trivia rule $R$.

**Outside the regions.** Call an evaluation **inside** a region when it is nested in an
evaluation of a call of a region rule, and **outside** otherwise. By no nesting, a rule that the
body of a region rule reaches is called only inside a region, and a call of a region rule is
outside, since no region rule is reached from the body of one. So every evaluation outside the
regions is of an expression of a rule that the start rule reaches without passing a region rule,
which the conditions read, and is either a call of a region rule or not nested in one.

**The automaton.** Read a body as the conditions do: lookaheads, predicates, anchors and cuts as
matching nothing, each alternative and each number of iterations of a repetition as possible, and
a call as any match of the body of its rule. Every match of the body is then a word of that
reading, and every character of a word is a character of a terminal of the body or of a rule that
it reaches. So the first character of a match that consumes input is an opener, and when a match
of the body of $R$ at $a$ consumes the text of $[a, x)$ and goes on, the character at $x$ is among
those that can follow that text at the start of a match, as the conditions compute them.

**Settled.** If a trivia rule $R$ is settled at an expression evaluated at $p$ outside the
regions, then an evaluation of a call of $R$ ended at $p$. By induction on the order of
evaluation, along the cases of the definition: the state at a place in a body holds of the
evaluation that is there, a call of $R$ ends at its end, an evaluation that consumes nothing
leaves the position, and a rule starts with what all its calls hold. A growth step evaluates the
body of its rule from the position of the call, where the call holds, and its left call ends
where an earlier body evaluation ended, which holds the end of the body. An evaluation of $X^*$
ends only where $X$ fails, so by fixed ends $X$ fails at $p$ and $\mathrm{end}_R(p) = p$.

**Next checks.** Let a terminal $t$ have the next check $!E$, an element of a sequence $S$. The
forms through which the definition passes the next check on, from $t$ up to $S$, each end when
their last part, chosen alternative or expression ends, and none is the body of a rule. So in an
evaluation of $S$ that goes on past an evaluation of $t$ that ended at $x$, every form between
them ends at $x$ and $S$ evaluates $!E$ at $x$ next.

**Claim.** Suppose the claim fails. Among all calls of region rules $R$ at $a$ and evaluations $g$
outside the regions and outside checks that start at an $x$ with $a < x < \mathrm{end}_R(a)$,
take $x$ least, and then $g$ the first such evaluation at $x$. Its parent, the evaluation that
$g$ is a part of, is outside the regions, since $g$ is outside, and outside checks, since $g$ is,
so it starts at a $y < x$ by the choice of $g$, and $y \le a$ by the choice of $x$. The parent is
a form whose parts follow one another, a sequence, a repetition or an Operators Rule, since the
parts of the other forms start where they do. The part before $g$ ended at $x$; follow back the parts
that the parent evaluated at $x$ before $g$: each is a check, by the choice of $g$, so some part
$s$ started before $x$ and ended at $x$, since the parts of the parent cover $[y, x)$. Descend into
$s$ along the last parts that consume input, through the alternative that matched, the last
iteration, the last part of a sequence that consumes input, the body of a call, the base or skip
of a `#recover`, and, for a left call, the body evaluation whose end it reads. Every evaluation
on the way ends at $x$ and is outside the regions, and the descent stops at a terminal $t$, of
$[z, x)$ with $z < x$, or at a call of a region rule $R'$ at $a'$ with $\mathrm{end}_{R'}(a') =
x$.

- A region rule $R'$ at $a'$. If $a' > a$, the call of $R'$ is outside the regions and starts in
  $(a, \mathrm{end}_R(a))$ before $x$, against the choice of $x$. If $a' < a$, the call of $R$ at
  $a$ is outside the regions and starts in $(a', \mathrm{end}_{R'}(a'))$ before $x$, against the
  choice of $x$. If $a' = a$ and $R' = R$, fixed ends give $\mathrm{end}_R(a) = x$, against
  $x < \mathrm{end}_R(a)$. If $a' = a$ and $R' \ne R$, both match input from $a$, so the character
  at $a$ is an opener of both, against disjoint openers.
- A terminal with $z > a$. It is outside the regions and outside checks (the descent passes no
  lookahead), and starts in $(a, x)$, against the choice of $x$.
- A terminal with $z < a$. It consumes the character at $a$, an opener of $R$, as a later
  character than its first, against guarded crossings.
- A terminal with $z = a$. It is a crossing of $R$, whose first character is the opener at $a$.
  If $R$ is settled there, $\mathrm{end}_R(a) = a$, against $a < x$. Otherwise the match of $R$
  at $a$ consumes the text of $t$ and goes on, so the character at $x$ can follow that text
  (*The automaton*), and the next check $!E$ of $t$ surely consumes it, so $!E$ fails at $x$.
  The sequence $S$ that holds the check is on the descent or is the parent, since the forms
  between $t$ and $S$ are no rule body and the descent passes every form between $t$ and $s$.
  Each evaluation on the descent ended at $x$, and the parent went on past $s$ to $g$, so $S$
  went on past $t$ and evaluated $!E$ at $x$ next (*Next checks*), and went on past it, which a
  sequence does not do after a part fails.

Every case is impossible, so the claim holds. $\square$

**Starts.** Let $R$ be a trivia rule with the region bound and $\sigma$ its body. An evaluation
of $\sigma$ is the body of a call of $R$, at the same position, and by no nesting every call of
$R$ is outside the regions. Let two evaluations of $\sigma$ that reach $q$ start at $b < b' < q$.
The first reaches $q$, so $q \le \mathrm{end}_R(b)$, and the call at $b'$ starts in
$(b, \mathrm{end}_R(b))$, against the claim. So at most one starts before $q$, and the other
starts at $q$. $\square$

**Scope.** The proof reads where evaluations start, not their cost. The records of
[run sites](../runtime/parsing.md#run-sites) and [scan sites](../runtime/parsing.md#scan-sites)
skip evaluations and give what they would have given (Lemmas 1 and 4 of
[Run Sites](run-sites.md)), so a parse that keeps them makes a subset of the evaluations of one
that does not, and the claim holds of it. The region bound replaces only the run
bound: the second bound, which counts the evaluations at one position, is unchanged, and a
scanning source inside the body of a region rule, such as `(!"*/" .)*` in an unclosed comment,
stays a scanning source, so the input that a failed comment examined is still counted against the
units around it.

## Theorem (Linear Cost)

When every rule that the start rule reaches satisfies conditions 1 to 5, every
parse from the start rule has a cost of at most $c \cdot K \cdot (L + 1) \cdot (n + 1)$
for a constant $c$. When no variable holds a string and every binding binds a
literal or a variable with finite values, $K$ is a constant by Lemma 8 and
$L = 0$, so the cost is at most $c' \cdot (n + 1)$.

The theorem rests on Lemmas 4 and 5, which are outlines, on Lemma 6,
which is proved given step 1 of [Open Steps](#open-steps), on step 3, since
Lemmas 2 and 10, which its proof uses, leave out the tries of an Operators Rule
outside its activations, and, for the values that actions build from reshaped
values, on step 7; so the promises of the classes Linear and Linear Given an
Environment Bound do too.

*Proof, given Lemmas 4 to 6 and step 3.* Each unit of $C$ is the local cost of
a body evaluation, the scanned cost of one, a call, or the walk cost. The body
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
that it charges for the work that
[Work of a Document Parse](../runtime/documents.md#work-of-a-document-parse)
bounds and that $C$ does not count, apart from the decoding of characters, which
the corollary counts with $C$: the edits that lookups apply; the steps along
a chain beyond the entries that the lookup of $C$ compares, the removal of
lagging entries, and the hashing and probing of the indexes of chains; the moves
of reused values; the operations of the runs; and the growth of the slots of the
memo.

At the start of the parse, let the **retained size** $W$ be the sum of one plus
the size of the key of each memo entry, the number of iterations of the run
records, and one plus the number of children and fields of each distinct node
that the values of those entries and iterations reach. Let $G$ be the number of
slots of the gap of the memo. Let $B$ be the greatest size of an empty node that
those values reach, an **empty** node being one that starts where it ends and
its **size** the sum of one plus the number of children and fields of each
empty node reachable from it through empty nodes. Let $Q$ be the greatest length
of a chain during the parse if it exceeds 32, and $0$ otherwise. Given Lemmas 4
and 5 and step 3 of [Open Steps](#open-steps), the Document units number at most

$$c \cdot \big( K \big( (L + 1)(Q + 1) + T + \log_2 (W + 2) \big) \hat{n} + (E + B + 1) W + G \big)$$

for a constant $c$.

*Proof.* Let $A$ be the sum of the calls, the body evaluations and the
activations of the parse. The proof of the theorem bounds $A$ by a constant
times $K \hat{n}$, through Lemmas 2 to 5 and 10, each of which counts the parse
itself, so no comparison with a parse of the same text from scratch is needed:
a reused iteration is an activation ([Setting](#setting)), and the entries and
runs of earlier parses evaluate nothing. Every charge below is a bound of
[Work of a Document Parse](../runtime/documents.md#work-of-a-document-parse),
which the proof cites as that section.

*Edits.* A call makes at most one lookup, and by that section a lookup applies
at most $T$ edits, one unit each, so the edits come to at most $A T$: a constant
times the calls, whatever the length of the log.

*Long chains.* A lookup compares at most 32 entries that do not lag before it
consults the index, and comparing a stored key with the key of the call charges
at most a constant times the size of the latter, since strings are compared only
when their lengths are equal; this is the lookup of $C$. A lagging entry is
removed where a walk meets it, at one unit, and is not compared. The parse
stores no lagging entry and no entry starts to lag during it, since the log does
not change, so each entry removed is one that the parse retained, at most $W$
units in all. Past the 32 entries, a lookup or a store hashes its key, at most a
constant times $L + 1$, and its probe compares at most the $Q$ entries of the
chain. A store of a key that the chain holds beyond them also passes at most $Q$
entries to reach it, and a store that replaces one of the first 32 entries of a
chain with an index hashes the key it replaces, which equals its own. Together
these charge at most a constant times $A (L + 1) (Q + 1)$. A parse builds the
index of a position at most once, hashing each key of the chain: at most $W$ for
the entries that the parse retained, and a constant times $L + 1$ for each of the
at most $A$ entries that it stored.

*Moves.* An entry moves at most once in a parse, after which it accounts for
every edit. A parse resumes a record of the previous parse at most once, and
only such a record moves its iterations, so each iteration moves at most once.
So at most $W$ values move. A node that does not start where it ends moves at
most once in a parse, and a move does not visit the nodes that the parse built,
so the visits of such nodes and their children and fields come to at most $W$.
A move visits an empty node once for each reference by which it reaches it,
from a value that moves or from a child or field of a node that moves, and copies
with it at most what its size counts, so at most a constant times $(B + 1) W$.
A move charges at most two units for each of these and for each child and field,
plus the edits it replays. It replays the edits for each group of nodes that
account for the same number of edits once, at most $E$, and a group has a node
that moves, so the replays come to at most $E W$.

*Runs.* Each evaluation of a repetition makes a constant number of operations
beyond those for its iterations, and each iteration reused or evaluated is an
activation or one of a constant number of iterations of a bounded repetition, so
at most a constant times $A$ operations are made in all. An attempt to reuse an
iteration, the end of an evaluated iteration and the start of a run or of an
iteration charge at most a constant, the moves of the values that the iterations
reuse, and, for their expected items, a set of the items of the grammar, at
most two units for each item that an iteration records and the merge of each
item, which charges at most one plus the number of the items of the grammar.
These are constants of the grammar. The growth of the list of the new
iterations of a run charges the number that it held before where their number
reaches a power of two; the list starts empty in each run, so the charges add
up to less than twice its length. Recording a run charges one unit for each of its
iterations, and each is such an iteration of this evaluation. A record of this
parse is resumed whole and needs no search, since no edit separates it from the
parse. For a record of the previous parse, a search for a recorded iteration
after the edit charges at most $1 + \log_2 (W + 2)$ units for each iteration
evaluated, and then scans recorded iterations. A scan that passes every
iteration makes the run reuse them, so it happens once for a record, and each
such record is resumed once, at most $W$ iterations in all. A scan that stops
does so at an iteration that examined input before the end $b$ of the edit. The
scan starts at an iteration that starts at or after $b$, the starts of the
iterations increase, apart from an empty last iteration, and an iteration
examines no slot more than one before its start
([Reuse Model](../runtime/documents.md#reuse-model)), so it stops within two
iterations. Making room for the values of a record charges the values on the
stack only when the stack with the room asked for passes the room that the parse
has paid for, which then at least doubles and covers what was asked. So the
charges add up to at most twice the last room paid for, which is at most twice
the greatest length that the stack with the room asked for reaches. The stack
holds a value for each iteration of the repetitions in progress, and the room
asked for is the iterations of one record, so that length is at most a constant
times $W + A$, and so are the charges.

*Slots.* A store charges for the growth of the slots only where the number of
binary digits of their length grows, by the old length. So the charges of one
parse add up to at most twice the last length, and a store at position $p$ needs
at most $p + 1 + G$ slots, so at most $2 (\hat{n} + G)$.

Summing, with $A$ at most a constant times $K \hat{n}$ and $K \ge 1$, gives the
bound. $\square$

The bound counts what each operation charges at most, by
[Work of a Document Parse](../runtime/documents.md#work-of-a-document-parse); the
operations of a parse are a function of the operations on the Document since
its creation, and so are their charges
([Reservations and Interruptions](../runtime/parsing.md#reservations-and-interruptions)).

*The units that the default budget counts.* The default budget of a Document
parse leaves out the Document units of Lemma 12 that the parse reserves in regions
of Document work: the memo protocol of a call of a rule that only a Document memoizes
(a transient rule, which a parse from scratch does not memoize), except a constant for each
variable of the key of its lookups, the steps of the walk of the chain of memo entries at a
position beyond the entry that answers, and the growth of the table in a lookup and a store, the
edits that a lookup applies, the moves of reused values with the cleanup that ends them, the
operations of recorded runs, the extra units that decoding a piece of the text
costs, and the copy of the diagnostics that the parse publishes. Every other unit
is counted: the unit that opens a call, the evaluation of the body of a rule, the
examination of the input, the values that the parse builds, and the memo protocol of a
rule that a parse from scratch memoizes too, with the unit of each lookup and store, the hashing
of the key of a lookup and its comparison with each entry of this parse that the walk compares it with, and the operations on the
recovered errors of an entry, which Lemma 14 bounds by a constant times $C$. A comparison with an entry that is not current in this parse, one that an earlier text left in the chain and this parse has neither stored nor found to answer a lookup, is Document work, since a parse from scratch has no such entry. The comparison of two strings costs the length of
the string, unless they are the same bytes of the same memory, which costs a constant. A lookup
of a rule that only a Document memoizes compares the strings of the key by identity only, at a
constant for each variable of the key: a string that is equal but not identical is a miss, and a
miss is sound, since the rule is then evaluated and that work counts as a parse from scratch counts it.
So the work on the keys of this parse is not left out of the count: it
depends on the grammar and the text, and a Document does at most the work of a parse from scratch
on it, up to the constant of the lookup. The comparisons with entries of earlier texts are Document
work, which depends on what the Document retains and is bounded by the window of 128 edits
(step 6 of [Open Steps](#open-steps)). The claim that the counted units of a
Document parse are at most the units that a parse of the same text from scratch reserves is
argued here, not proved by a lemma, and the tests of the repository check it on
random edits. The argument: each counted unit belongs to a call, to a
body evaluation, to an examination of the input, to a value that the parse builds, or to the
memo protocol of a rule that a parse from scratch memoizes. The Document parse makes each of these
for the text at most as often as the parse from scratch does: it evaluates the same bodies on
the same input in the same order, except that a memo entry answers a call and the body is not
evaluated, and the entry costs the units of the lookup and the replay of its recovered errors,
which the evaluation it replaces would have built at a unit each. The deferral of the memoization
of a rule is the same in both parses, so the rules that both memoize have the same lookups and
stores. A first parse of a Document is the parse from scratch plus the Document work, and a parse
after edits evaluates a call again where an entry lagged behind by more than 128 edits. This
holds for the three backends of the engine and for generated Go, whose Documents memoize the
same rules as a parse from scratch plus those that only a Document memoizes. So a Document parse
under the default budget is argued not to fail where the parse from scratch of its text does not.
Its default budget, raised by the Document units, is at most that budget plus the bound on the
Document units above (the terms $W$, $B$, $G$ and $Q$ are bounded by the retained state, not
by $n$, $K$ and $L$, as step 6 leaves open). A grammar whose counted units alone pass the
default is stopped by it, whatever the Document retains: the operations on the recovered errors
count (Lemma 14), and the tests run a grammar of recovered errors at lengths that a parse from scratch
completes and expect a Document parse to complete with a constant factor of its units. A grammar whose
cost is high only in work that is left out of the count (the extra steps along the chains, the growth of
the table, the edits, the moves, and the retained state of step 6) is not stopped by the default in
proportion to that work. $\square$

A node counts in $W$ whichever text it was built for, and whether or not a call
of the parse reaches it. No lemma bounds $W$, $B$, $G$ or $Q$ by $n$, $K$ and $L$: see step 6 of
[Open Steps](#open-steps). A chain has at most $E + 1$ entries under one key
of a rule that a parse from scratch memoizes too ([Setting](#setting)), so $Q$ is
at most $E + 1$ times the number of such keys at its position, plus the entries
of rules that only a Document memoizes, which can be several under one key on a
chain of more than 32 entries; these include keys of earlier texts, which the
$K$ of the parse does not count. $G$ grows with deletions and shrinks with insertions.

**Over a sequence of parses.** Let $P_1, \dots, P_m$ be the Document parses, in
order, that follow the last drop of the memo entries and the run records of a
Document, or its creation, and that complete, so that none of them drops the
memo ([Reuse Model](../runtime/documents.md#reuse-model)). For $P_j$, let
$\hat{n}_j$, $K_j$, $L_j$, $Q_j$, $E_j$, $B_j$ and $G_j$ be the terms of
Lemma 12, and $A_j$ its $A$, with $A_0 = 0$. Let $\Lambda_j$ be one plus the
greatest size of a key that $P_j$ hashes to build an index, or $1$ if it builds
none, and let the **moved size** $V_j$ be the sum of one plus the number of
children and fields of each distinct node that a move of $P_j$ visits or
copies, an empty node counted once however often it is copied. Then the Document units of $P_1, \dots, P_m$ add up to at
most

$$c \sum_{j=1}^{m} \Big( \big( (L_j + 1)(Q_j + 1) + \Lambda_j Q_j + T + \log_2 (A_{j-1} + 2) + B_j + 1 \big) A_j + A_{j-1} + (E_j + B_j + 1) V_j + \hat{n}_j + G_j \Big)$$

for a constant $c$, where $A_j$ is at most a constant times $K_j \hat{n}_j$, as in
Lemma 12. No term counts what the Document retains as a whole: each memo entry
counts where a parse stores it and where a walk removes it, the records count in
the parse that made them and the next one, and a node counts in each parse that
moves it.

*Proof.* The proof of Lemma 12 bounds each charge of $P_j$; its terms in $W$
come from four kinds of work, and the others are already in the form above.

- *Removed entries.* Every memo entry was stored by a parse, and no entry is
  stored twice: a store links a new entry. Since the memo was last dropped, every
  store was made by one of $P_1, \dots, P_m$, and a removed entry leaves its chain
  for good, so the entries removed in all of the parses number at most the
  stores of $P_1, \dots, P_{m-1}$, at most $A_1 + \dots + A_{m-1}$.
- *Indexes.* A parse builds an index in a lookup or a store, at most one for
  each, so at most $A_j$ times, and each build hashes the at most $Q_j$ keys of
  its chain, each at most $\Lambda_j$ units, in place of the $W$ of Lemma 12.
- *Records.* A record that a parse searches or scans is a record of the previous
  parse, resumed across one edit; a record resumed whole needs no search, and
  each iteration that it reuses is an activation. The records of $P_{j-1}$ hold
  at most the iterations that $P_{j-1}$ recorded, each charged one unit when it
  recorded the run, so at most $A_{j-1}$ in all. So a search charges at most
  $1 + \log_2 (A_{j-1} + 2)$ units, the scans that pass every iteration of a
  record charge at most $A_{j-1}$ in all, and the room asked for is at most the
  iterations of one such record or those of a record resumed whole, which are
  activations, so the length of the stack and the charges for the room are at
  most a constant times $A_{j-1} + A_j$.
- *Moves.* A value moves when a lookup finds its entry or a run reuses its
  iteration, at most once in a parse, so at most $A_j$ values move. A node that
  does not start where it ends moves at most once in a parse, so the visits of
  such nodes, with their children and fields, come to at most $V_j$. The
  references by which a move reaches an empty node are a value that moves or a
  child or field of a node that the move visits or copies, at most $A_j + V_j$,
  and each copies at most $B_j + 1$ units. A group of nodes that account for the
  same number of edits has a distinct node that the move visits or copies, and
  replays at most $E_j$ edits, so the replays come to at most $E_j V_j$.

The slots charge at most $2 (\hat{n}_j + G_j)$ in $P_j$, as in Lemma 12, and the
other charges are those of Lemma 12, bounded by the terms in $A_j$. Summing over
the parses gives the bound. $\square$

So over the sequence the walks, the indexes and the records cost a constant
times the work of the parses, for given chain lengths and key sizes, and the
total size that the Document retains enters no term. The moves remain: $V_j$ is
at most the $W$ of $P_j$, and a node that every parse after an edit reuses moves
in each of them, so the moves do not add up to the stores. Each move replays at
most $E_j$ edits for each group of nodes, which the bound keeps as a factor.

## Corollary (Compute of a Document Parse)

Under the conditions of the theorem and given Lemmas 4 to 6 and step 3, a parse of a
Document on a backend of the engine, outside the missing-token mode, charges at
most

$$c \cdot \big( K \big( (L + 1)(Q + 1) + T + \log_2 (W + 2) \big) \hat{n} + (E + B + 1) W + G \big)$$

units of Compute, for a constant $c$, whether or not it recovers errors. Here a
parse is the run of the grammar over the text, up to its outcome. Of these
units, the calls and their lookups, with the edits that the lookups apply, take
at most $c \cdot K (L + T + 1) \hat{n}$ when no chain exceeds 32 entries. That
part is linear in $n$ whatever the log; the other terms
depend on what the Document retains, so the whole bound has the form of a
constant times $K \hat{n}$ for a given retained state.

*What the limit on the edits of a lookup bounds.* The edits that lookups apply come to at most
$A\,T$ units, where $A$ counts the calls, the body evaluations and the activations of the
parse, and $A \le c\,K \hat{n}$ by the theorem. So they are a constant times the work of the parse, and the
constant does not depend on the number of pending edits $E$, up to the 65,536 of the log. This is the
whole of the constant factor that the proof gives. The other Document units are not a constant factor of
$K \hat{n}$: the moves cost $(E + B + 1) W$, a term in the size $W$ that the Document retains and in the
length $E$ of the log, and the walks, the growth of the slots and $G$ depend on what earlier texts left
in the memo (step 6 of [Open Steps](#open-steps)). The total of a Document parse is therefore bounded by
the displayed form, and the claim that it is within a constant factor of a parse from scratch holds for
the part of the lookups and their edits only. The tests of the repository measure the
whole for a grammar of 20,000 lines with 0 to 65,536 pending edits spread over the text: at most 4.3
times the units of a parse from scratch on each backend, and 2.2 at 6,000 edits and above, where most
entries lag behind by more than $T$ edits; the test enforces 6, and fails when a lookup applies the edits
without the limit.

*Proof.* The units of Compute of a parse are those of the steps that $C$ counts,
those of the recovered errors, which $C$ does not count, the extra units of
decoding the characters of the text, and the Document units. The theorem bounds
the first by a constant times $K (L + 1) \hat{n}$. Lemma 14
bounds the second by a constant times $C$, and so by the same bound: they are
the units that the takes charge, and no list moves in a Document. Decoding a
character charges at most three units beyond the examination of the input that
$C$ counts for it
([Work of a Document Parse](../runtime/documents.md#work-of-a-document-parse)),
so the third is at most $3C$. Lemma 12 bounds the fourth.
The part of the lookups is the theorem's bound of the calls, plus the edits and
the walks of Lemma 12. $\square$

Listing the errors of the outcome charges no Compute. By Lemma 14 it visits at
most twice as many lists as the outcome has errors, counted as often as they
occur, and by Lemma 16 that number is at most a constant times the input that
the parse matched, which a Document parse need not examine. In the
missing-token mode, the parse records the errors of every reused result one by
one, and no lemma bounds those units.

That these four are all the units of a parse is a reading of the charges of the
engine, not a derivation: no lemma relates a charge to $C$ in general.
A store of a key that a chain holds walks the chain to relink it. Here $Q$ counts
that walk; outside a Document no bound of this proof does, and it has not been
measured. It matters only where $K$ grows with the input, since otherwise a chain
has a constant number of keys.

The corollary does not cover the work that follows the outcome in the same
Control: the capture and sharing of `SnapshotWith`, the comparison of change
notifications, and the publication of diagnostics. Nor does it cover the
generated Document runtimes: they apply each edit to every entry at once, and
they move a copy of a reused value, so a node is not moved once for a parse.

## Lemma 14 (Shared Recovered Errors)

The recovered errors of a parse are the sequence of **entries** of
[Cost](../runtime/parsing.md#cost): errors that recoveries recorded, and lists
that reuses recorded or that stores took. A **list** is an error alone, or the
entries that one store took; it never changes once made, and entries anywhere
may share it. A **take** of the entries after a mark $m$, made where a memo
entry, a result of a growth or an attempt of an Operators Rule is stored,
charges one unit for each entry after $m$. When there is none it gives an empty
list, when there is one it gives that entry, and otherwise it makes a list of
the entries after $m$ and puts it in their place. Recording a list that is not
empty adds it as one entry, an empty one not at all, and charges nothing. A
step of a growth takes the entries after the mark where the growth began, and a
growth that ends removes the entries after that mark and records the list of
its result; backtracking and a stream that forgets delivered errors only remove
entries. Let $P$, $A$ and $S$ be the numbers of errors recorded, of lists that
are not empty recorded, and of takes in a parse. The **unrolled size** of a
list is 1 for an error, and 1 plus the unrolled sizes of its entries for any
other list: the number of lists that listing its errors visits, a shared list
counted where it occurs.

**Lemma 14.** In every parse:

1. the units that the takes charge add up to at most $P + A + S$, and so do
   the entries of the lists that they make;
2. a list of $k \ge 1$ errors, an error counted as often as it occurs, has an
   unrolled size of at most $2k - 1$, and listing $k$ recovered errors visits
   at most $2k$ lists.

*Proof.* (1) Let $\Phi$ be the number of entries, 0 at the start of the parse
and never negative. Recording an error, and recording a list that is not
empty, add one entry each, and nothing else adds one. A take
with $s$ entries after its mark charges $s$ units. With $s \le 1$ it leaves the
entries as they are and charges at most 1. With $s \ge 2$ it puts one entry in
place of $s$, so that $\Phi$ falls by $s - 1$, and it charges
$1 + (s - 1)$. The falls of $\Phi$ add up to at most its rises, which are at
most $P + A$, so the charges add up to at most $S + P + A$. A list that
a take makes holds the $s$ entries it charged for, so its entries add up
to the same bound.

(2) By induction on the order in which the lists are made. An error is a list
of $k = 1$ with unrolled size $1 = 2k - 1$. A list that a take makes holds
$s \ge 2$ entries made before it, of $k_1, \ldots, k_s \ge 1$ errors, since an
empty list is never an entry; with $k = \sum_i k_i$ its unrolled size is
$1 + \sum_i (2 k_i - 1) = 2k - s + 1 \le 2k - 1$. The bound holds wherever a
list is shared, since no list changes once made. The recovered errors are
those of the $m$ entries in order, so listing $k$ of them visits
$\sum_i (2 k_i - 1) = 2k - m \le 2k$ lists. $\square$

The case $s = 1$ is what keeps the second bound. A list made of the one list
after a mark would add a level of unrolling and no error: calls nested $d$ deep,
each storing the one list of the call that it made, would make a list of
unrolled size $d + 1$ for one error.

In a [Document](../runtime/documents.md) no list moves: an entry that holds
recovered errors does not move after an edit. It stays if it examined only
input before the edit, and otherwise stops being valid, so a list holds its
errors at the positions where they were made.

**Units of recovered errors.** By [Cost](../runtime/parsing.md#cost), a recovery records its error,
a call that reuses a memo entry, a result of a growth or an attempt of an
Operators Rule records the list of that result, a store of one of them takes
the entries after the mark where its evaluation began, and a growth that ends
replaces the entries recorded since it began by the list of its result. Each
of these is part of a step that $C$ counts with at least one unit: a recovery,
a call, a store, or a growth step. So by the lemma the units that the takes
charge and the entries of their lists are at most a constant times $C$, and
listing the errors of the outcome visits at most twice as many lists as there
are errors, which Lemma 16 bounds by the input. The lemma does not cover the missing-token mode, which records the
errors of every reused result one by one.

**Measured controls.** The engine on its three backends and the generated Go
and TypeScript runtimes implement these operations. Two grammars of the class
Linear check it. In the first,

```text
rule main = s $$
rule s = item s "!" / item s / item
rule item = ("a" ";") #recover(skip=[^;]* ";")
```

on `b;` repeated $n$ times, the entry of `s` at each item holds the errors of
all the items after it, and each later alternative reuses it. In the second, a
left-recursive leader that recovers in each step,

```text
rule main = e $$
rule e = e "," item / item
rule item = ("a" ";") #recover(skip=[^;]* ";")
```

on `b;` repeated $n$ times with `,` between, each seed holds the errors of all
the items before it. A target that stored a copy of the errors in each entry or
seed, and appended the copy again at each reuse, would charge in proportion to
$n$ at each of $n$ positions. The least budget of Compute that completes a
parse, for each item, is:

| Target | First grammar, $n = 250$ to $4000$ | Leader, $n = 250$ to $4000$ |
| --- | --- | --- |
| Engine, closures | 148.6 to 152.6 | 43.0 to 44.1 |
| Engine, bytecode | 160.6 to 164.6 | 48.0 to 49.1 |
| Engine, iterative bytecode | 159.6 to 163.6 | 48.0 to 49.1 |
| Generated Go | 162.6 to 166.5 | 48.0 to 49.1 |
| Generated TypeScript | 178 | 77 |

The TypeScript cells hold at all three of $n = 250$, 1,000 and 4,000, where the
runtime was measured. At $n = 4000$, 8,000 and 16,000, the engine on its three
backends and the generated Go runtime complete every parse of both grammars
under the default limits, and on each of them the memory that a parse
allocates about doubles with $n$. For the first grammar it is 10.9, 23.7 and
48.2 MB on closures, 12.7, 25.7 and 54.4 MB on bytecode, 13.5, 28.1 and 58.2 MB
on iterative bytecode, and 10.0, 20.3 and 41.4 MB in generated Go; for the
leader it is 3.1, 6.4 and 13.2 to 13.3 MB on every one.

The slope tests of the classifier check both grammars on the three backends of
the engine, and the tests of each generated runtime check that four times the
items cost less than five times the Compute.

## Lemma 16 (Errors of an Outcome)

Lemma 14 bounds the lists that listing the recovered errors of an outcome
visits by twice the number of those errors, counted as often as they occur.
This lemma bounds that number by the input. It uses none of the conditions for
linear cost, so it holds for every grammar.

The recovered errors of a parse are those of the reference parse $\mathcal{R}$
of the [memoization proof](memoization.md#setting), by its
[theorem](memoization.md#theorem-memoization-never-changes-the-result), and a
[Document](../runtime/documents.md) parse returns those of a parse of its text
from scratch ([Documents](documents.md#theorem-document-parses-equal-fresh-parses)).
A [stream](../runtime/streaming.md) lists only the errors that it has not
delivered. So it suffices to count the errors $\varepsilon$ of the outcome of
the start call in $\mathcal{R}$. As in those proofs, the parse inserts no
[`#missing`](../language/pragmas.md#missing) token and no limit stops it.

**Derivations.** The **derivation** of an evaluation in $\mathcal{R}$ that
matches is a tree. Its root is the evaluation, and the children of a node are
the derivations of the outcomes that it records and keeps, in order: an
expression keeps the outcomes of its subexpressions that it ends with, a call
keeps the evaluation of the body of its rule, a call that grows a leader keeps
the last step whose result became the current result, which is the result
that the call returns, a call that reads the current result of a growth keeps
the step that gave that result, evaluated before the call, and a recovery that
recovers keeps its skip expression. A
[lookahead](../language/parser-expressions.md#lookahead) and the
[`skip` of an Operators Rule](../language/operators-rule.md#skip) keep
nothing, since their end discards the errors recovered in them. So the
parent of a node is the evaluation that evaluated it, except below a call
that reads a current result. A step that two calls read is a child of each,
so a node is an occurrence. The **span** of
a node is the interval $[s, e)$ from the position where it starts to the end of
its outcome. A **recovery node** is a node of a recovery that recovers.

The caller records an outcome by appending its errors to its own
([Setting](memoization.md#setting)), a failure has none, and a recovery that
recovers records its own error besides those of its skip expression
([`#recover`](../language/pragmas.md#recover)). So the errors of an
evaluation are those of the recovery nodes of its derivation, one for each,
and $|\varepsilon|$ is their number.

**Lemma 16.** Let the start call of a parse match from $p$ to $q$. Its
recovered errors, counted as often as they occur, number at most
$D_\varepsilon \, (2 (q - p) - 1)$ if $q > p$, and none if $q = p$, where $D_\varepsilon$ depends only
on the grammar. Outside a Document, $q - p \le C$, so listing them visits at
most $4 D_\varepsilon C$ lists.

*Proof.* **Spans nest.** A child's span lies within its parent's, by
[Positions do not decrease](../runtime/parsing.md#growths-of-left-recursion).
The children of a node lie one after another: after it records an outcome that
ends at $e$, an evaluation continues from $e$, and it goes back before $e$
only where a choice, a repetition, an Operators Rule, a recovery or a lookahead
returns to the position where one of its subexpressions started, which drops the outcomes
recorded after that position with their errors
([Cost](../runtime/parsing.md#cost)). So two nodes of which neither is above
the other have disjoint spans, those of the two children of their lowest
common ancestor that lie above them. A recovery node has a span that is not
empty, since its skip expression consumes input. So the spans of the recovery
nodes are, for any two, disjoint or nested, and two recovery nodes with the
same span lie on one path from the root, and so do the nodes between them,
whose spans are the same.

**Distinct spans.** A family of distinct non-empty intervals within
$[p, q)$, any two of which are disjoint or nested, has at most
$2 (q - p) - 1$ members. Order them by inclusion into a forest. A member with
at most one child member holds a character that lies in no child, since it
differs from its child or has none and is not empty. Such a character lies in
no member below the one that holds it and in none disjoint from it, so no two
members hold the same one, and these members number at most $q - p$. A forest has fewer nodes with two or
more children than leaves, and the leaves are among the members just counted,
so the others number at most $q - p - 1$.

**One span.** Let $\pi$ be a path of nodes that all have the span $[a, b)$,
$a < b$. Every node of $\pi$ starts at $a$, so its calls are calls at $a$,
each made by a body that started at $a$. Call a node of $\pi$ that reads the
current result of a growth a **read**. A read's child on $\pi$ was evaluated
before it; every other node of $\pi$ evaluates its child, so the reads cut
$\pi$ into **segments** in which each node is evaluated by the one above it.
The nodes of a segment from a node $u$ down to a node $w$ are then all the
evaluations that are evaluated within $u$ and enclose $w$, and the calls of a
segment form a chain of
[Lemma 1 of Left-Recursion Growth](left-recursion.md#lemma-1-chains-at-one-position).

Let $r$ be a read of a growth $H$ and $u$ the first node of the segment that
ends at $r$. If $u$ is a step of $H$, or the call that grows $H$ lies on the
segment, whose child there is a step of $H$, then $r$ is evaluated within a
step of $H$ that lies on $\pi$ and reads the current result that the step
started from. That result ends before the step does, since the step became
the current result by ending farther (a read within the first step fails), so
$r$ would end before $b$. Hence
neither holds, and $H$, open when $r$ is evaluated, started before $u$: a
growth that started within the evaluation of $u$ and is still open at $r$ is
grown by a call that is evaluated within $u$ and encloses $r$, so a node of
the segment. Now let $r'$ and $r$ be consecutive reads on $\pi$, of growths
$H'$ and $H$, so that the segment after $r'$ starts with a step $S$ of $H'$.
Then $H \ne H'$, $H$ started before $S$ and is open within $S$. The calls that
grow $H$ and $H'$ are both being evaluated when $S$ starts, so one encloses
the other. If the call of $H'$ enclosed that of $H$, $H$ would have started
within an earlier step of $H'$ and ended with it, before $S$. So each growth
read on $\pi$ encloses the one read before it, all are open at once at $a$,
and their keys of leader and level differ, since a call of an open growth's
key at its position reads it. So $\pi$ holds at most $N$ reads and $N + 1$
segments, where $N$ is the number of keys of
[Left-Recursion Growth](left-recursion.md#setting), and each segment holds
at most $B$ calls by its Lemma 1.

Between two calls of a segment, and above its first or below its last, the
nodes are parts of one body, and a recovery node among them is the evaluation of
a `#recover` expression of that body. Within one body a path of nodes enters one
iteration of a repetition, one operand or operator part of an Operators Rule
(which is a whole body, so no `#recover` wraps it), and one choice branch, so
the recovery nodes of a stretch are evaluations of distinct `#recover`
expressions, each nested in the one before. Let $d_\varepsilon$ be the greatest
number of `#recover` expressions that enclose one another, the expression itself
included, in a rule body, and $m \le d_\varepsilon$ the greatest number of
recovery nodes that one stretch holds. The $N + 1$ segments hold at most $B + 1$
stretches each, and the calls of a segment are not recovery nodes, so $\pi$
holds at most $D_\varepsilon = (N + 1)(B + 1)\,m$ recovery nodes, where $N$ and
$B$ are those of Left-Recursion Growth.

**The count.** The recovery nodes with one span lie on one path of that span,
so at most $D_\varepsilon = (N + 1)(B + 1)\,m$ of them share a span, and the distinct spans number at most
$2 (q - p) - 1$; if $q = p$ no recovery node fits in the derivation.

**Matched input.** Outside a Document the
parse starts with an empty memo and no record of a walk. Its position passes
a character only where a terminal matches the character, which $C$ counts as
input examined, or where it takes an outcome, a current result or a record that
this parse made earlier from such matches. A record of a
[skip run](skip-runs.md) is exact and was made by a walk that examined its
characters ([Skip Runs](skip-runs.md#lemma-1-exactness) and
[Cost](skip-runs.md#lemma-2-cost)); so is a record of a
[run site](run-sites.md#lemma-1-exactness), counted by its
[cost once hot](run-sites.md#lemma-2-cost-once-hot), and a record of a scan site
([Exactness](run-sites.md#lemma-4-exactness-of-scan-records) and
[Cost](run-sites.md#lemma-5-cost-of-scan-records)). The generated parsers keep
records only for skip runs ([Scope](run-sites.md#scope)). So each of the
$q - p$ characters was examined by a step that $C$ counts, on every backend, and $q - p \le C$. Listing charges no Compute, so
the lists that it visits are not units.
By Lemma 14 listing $k$ recovered errors visits at most $2k$ lists, so listing
those of the outcome visits at most $4 D_\varepsilon C$. $\square$

**Scope.** The lemma holds in the units of the cost and for every grammar,
without the conditions for linear cost. It excludes a parse with missing-token
insertion and a parse that a limit stops. A stream lists a part of the errors
of the parse of the input that it has read, since its commits change no
evaluation, as in Lemma 15, so the bound holds for it too.

A recovery inside the skip expression of another can share its span: with
`rule item = ("a" ";") #recover(skip=inner)` and
`rule inner = ("b" ";") #recover(skip=[^;]* ";")`, the input `c;` gives two
errors over $[0, 2)$. So the errors of an outcome do not cover disjoint spans,
and the bound counts the recovery nodes of each span instead. The errors of a
step that a growth reads recur in the outcome, but each read keeps the step
over a span shorter than that of the step that reads it, so the steps of one
growth at one position add no errors over one span beyond the bound.

In a Document the count is the same, at most $D_\varepsilon \, (2 (q - p) - 1)$, but the
parse need not examine the input that it reuses, so the bound by $C$ does not
follow. In the missing-token mode an insertion records an error over no input,
and the parse records the errors of every reused result one by one; the lemma
does not cover it.

## Correspondence

The classifier decides the conditions and terms of
[Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost) on the
[cost view](../runtime/parsing.md#cost-view), over-approximating where it cannot
be exact ([Classification](../runtime/parsing.md#classification)). Each entry
names a term or condition of the specification and the lemmas that use it.

- The expressions whose matches are bounded in length and those that can match
  without consuming input, which the terms scanning source, bounded list and
  growing unit read: Lemmas 1, 2, 4, 6 and 11. The first characters: Lemma 4.
- Scanning sources, units and what contains a source: Lemmas 1, 2 and 4.
- Bounded scans, with their alphabets, run bound, region bound and second
  bound: Lemma 13. Region rules, with their fixed ends, nesting, openers,
  crossings, settled rules and guards: Lemma 15.
- The [cost view](../runtime/parsing.md#cost-view), with its rewrites, cheap
  rules and value conditions: the [Cost View](cost-view.md) proof, which
  transfers every bound on the view to the grammar as written.
- Reused skips of [Skip Runs](../runtime/parsing.md#skip-runs), which are no
  scanning source: Lemma 9, and Lemma 4 for the units whose only source was
  such a skip. Skip-run sites: the [Skip Runs proof](skip-runs.md), as the
  case of Lemma 4 for those sites.
- The memoized rules of
  [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules), and the
  calls memoized at the call and shared: Lemmas 3 and 10, and the shared calls
  of Lemma 4.
- Condition 2 (Repeated scans), with a unit that can fail after a source, the
  continuations, committing units, shared calls, calls that nest at one end,
  Operators Rules that nest, words that exclude a continuation and
  alternatives that can fail only after a `#cut`: Lemma 4, its failure after a source, the cases of
  committing and its claims on links.
- The paths that enter a growth again, for which an alternative is committing
  when its sources lie in memoized calls and what follows the leader has no
  source: the fifth case of Lemma 4, with Lemma 5.
- Condition 2 for growing units and their continuations: Lemma 5,
  *Sources at fixed offsets*.
- Condition 3 (Growth) and condition 5 (Carried growth): Lemma 5.
- Condition 1 (Variables), with the bindings that bind a value that the input
  determines, which give the class Linear given an environment bound: Lemma 3
  through $K$, and *Values of variables* in Lemma 11. The variables with
  finite values and the guards of bindings: Lemma 8.
- Condition 4 (Copies), with the copying operations, string values and nodes
  for `len`, bounded lists, calls on a recursion, fold accumulators, values
  from outside a lambda, values of variables, captures in a positive
  lookahead and the elements that a lambda copies: Lemmas 6, 7 and 11. The
  step matrix of a fold: Lemma 7, after its proof. Own values and size
  degrees: Lemma 7, *Size degrees*, with Lemmas 6 and 11. Values whose size
  can come from a capture in a positive lookahead: Lemma 6 and the anchored
  values of Lemma 11.
- The degrees of the class polynomial and the reasons for the class unknown
  of [Classification](../runtime/parsing.md#classification): no lemma, since
  those classes carry no promise.

Every condition and term above is decided by the classifier. Three parts of
the proof are no condition and need no check: the factors $K$ and $L$, which
are quantities of a parse that the classes carry, the three evaluations at a
position for each variant of a rule that the memo of
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules) allows
(Lemma 3), and the alternatives that a parser skips by the next character,
which that section limits so that they hold no activation (Lemma 4,
*Counterparts in the reference parse*).

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

The outlines leave steps 1 to 6 to be made precise, and step 7 is the part of the
condition of the cost view that the [Cost View](cost-view.md) proof leaves open.

1. **Counting activations in Lemma 4.** The claims of Lemma 4 reduce the scanned
   cost to the activations, show that the successful activations of a source
   at a position are direct in distinct body evaluations, and link each to an
   earlier one through a call that a committing unit shares with its
   continuation. They charge the activations held by each memoized evaluation,
   those in its own region, to those held by the first evaluation of its class,
   the head, which can be in a context of growths and of either variant, and
   show, by comparing the two evaluations of a memoized link in the reference
   parse, in either lookahead state and either variant, that the link leads to
   an activation held by an evaluation that is not a head, when every growth
   open at the start of one evaluation is open at the start of the other and
   the comparison meets no
   entry of an earlier Document parse that answers a call in a context of
   growths, or a call whose corresponding call in the later evaluation is in a
   smaller lookahead state, in a context of growths or of the other variant,
   and no result kept in a
   context of growths that was kept in a smaller lookahead state or whose
   observed lookahead it enters. So the sum over the positions is proved for the
   activations owned by memoized evaluations of every source in a rule that
   calls a rule or is memoized at its calls, when its memoized links are of that
   kind, apart from the activations that a dropped step of a growth or a path
   that enters the growth again repeats, which Lemma 5 counts (step 4), and
   with no count of the evaluations in a context of growths. The activations that
   do not succeed, and the evaluations of the rules that are not memoized,
   follow from the successful ones and the body evaluations of memoized rules
   (*Activations that do not succeed*), so what is open concerns the successful
   activations alone. An activation need not follow a link of the parse: the
   memo can answer the shared call in the continuation (*Repeats*), and the
   claims follow it through its reference link instead. The activations of the
   reference parse that a dropped step or a path that enters a growth again of
   the reference parse leads to, which have no reference link, are more than
   those of the parse, and Lemma 5 must count those that are counterparts of
   activations of the parse; this is not shown. Nor are the activations of the
   reference parse that have no reference link because a shared call, up to
   the one that holds the first activation repeated, reads a growth open at its
   start, in its evaluation inside the unit or in the one that the
   continuation makes, nor those whose reference link meets case (a) or (b)
   of the covered keys. Three parts are open.
   The first two are for a source in a rule that calls no rule and is not
   memoized at its calls. First, the leaf links that lead to an
   activation held by a head: such a link can leave a memoized evaluation that
   is not a head through the end of its body. The heads that start at the
   position of the scan are bounded by the classes there (*Memoized links keep
   positions*); those that start before it, and the activations that one head
   holds, are not. Second, the leaf links when the shared call follows other
   shared calls: the continuation then leaves frames at the ends of the
   shared calls before the call, at positions before the scan, and between
   them, as in `e = "(" (e R "x" / e) / "."` with `R = [ ]*`, where the call
   of `e` that both alternatives share comes before the call of `R`. *Calls
   that call no rule* counts only frames left at the position of the scan.
   *Nesting at one end* shows that evaluations of a call that nest without a
   bound and end at one position, through restarts as well, make the call
   nest at one end, which keeps the call of `R` in that grammar from being
   shared, apart from the three points that it leaves open; what remains is
   to bound the frames left at the earlier positions by it. And the leaf links
   between which the parser leaves or starts, at their position, a growth
   step or an evaluation in a context of growths, which that claim leaves
   out, and for which the condition has its case of seed reads (*Seed
   reads*); that case is argued from *Nesting at one end* but not proved:
   besides the three open points of that claim, it is not shown that the
   nested evaluations of a leader that it picks end at the position, since a
   leader whose current result ends there can grow beyond it, nor that no
   dropped step of a growth lies on the path between them. Third, the
   memoized links that the comparison leaves out: a link of a leader with a
   growth of its cycle open at the start of only one of its two evaluations,
   which then read different results of another leader of the cycle; in a Document parse, a link
   whose comparison meets an entry of an earlier parse that answers a call in
   a context of growths, or a call whose corresponding call in the later
   evaluation is in a smaller lookahead state, in a context of growths or of
   the other variant; and a link whose comparison enters a
   lookahead that a result kept in a context of growths observed, or meets a
   result kept in a smaller lookahead state, which the open gap of the
   memoization proof leaves out. Such a link can lead to
   an activation held by a head. The activations include
   those of the copying operations, the units of the own spans that they read
   (Lemma 6). Two points of the claims proved in step 1 are left to make
   precise. The covered keys list the ways in which the parse does not
   evaluate what the reference parse evaluates: it also leaves out the
   iterations that a [run-site](../runtime/parsing.md#run-sites) record
   covers, so the claim must add this case, and *Links lead to repeated
   evaluations* must show that it does not end a link's comparison; the replays must show their
   correspondence of own regions where one region reuses a run record, and
   their argument that a growth read in the reference parse inside $M$ is read
   by $M$ must cover the iterations that a run record answers and the calls
   that an entry of an earlier Document parse answers, for which it cites (S)
   of the memoization proof without showing that such an entry satisfies it;
   and the statement that the bounds hold whether or not a parse keeps the records
   must be argued. And the statement on twins that the counterparts take over is
   tested on the Core IR of the generated parsers and on the calls and their
   outcomes in the engine, for rules on no cycle that read no variable, and
   not for the values that a twin builds for its predicates and bindings.
2. **Continuations of growing units.** The claim on sources at fixed offsets in
   Lemma 5 bounds a unit followed by a growing unit with which it shares no
   call, in all but a constant number of steps, and *Calls shared with a
   growing unit* shows that a pair with a shared call is, in those steps, a
   growing unit followed by a growing unit, unless the unit calls, before the
   shared call of the leader, a rule whose evaluation examines no input. Such
   a unit examines the input from the end of the leader's result, as a growing
   unit does, but condition 2 checks it only as an alternative that is not
   growing. The proof must still bound that case, or the definition of a
   growing unit must take in such a unit, the later
   alternatives that a step evaluates after a growing unit fails, in the first
   step and in the step where the growth stops, and the case where the first
   step of a growth matches a part before the left call, as through `F` in
   `E = F "+" X / Y; F = E / M`, and a part before the unit, as through the
   `"y"` in `E = "y"? (E / "z") "q" / "x"`, or before a call that reaches it,
   as in `r = "y"? f "q" / "x"; f = r / "z"`, where the unit is also checked as
   an alternative that is not growing.
3. **Operators Rules.** Lemma 4 places the operators that an Operators Rule
   applies and its units among the activations and units of its claims, with the
   operator parts that an enclosing level tries again as a continuation of an
   operator part after an operand when the rule nests, through its right
   operands or through a call that parses the rule again at the end of an
   operand or an operator part; its claims on direct activations and links cover
   the operators applied at a position. The tries of operator parts that fail at
   a position when inner levels return there are at most the number of parts for
   each level, so their cost is the cost of the units up to their failure, which
   the claims of step 1 bound with its open parts. The step must also bound the
   number of those tries, and of the evaluations of the operands, by the
   operators applied and the body evaluations of the rule: they lie inside the
   source of the operators but in no activation of it, so Lemma 2, *One place*
   of Lemma 10 and *Activations that do not succeed* of Lemma 4 do not bound
   the calls and the evaluations of sources made in them.
   *Nesting at one end* takes the continuation $\kappa$ of an abandoned unit
   to be what the parser evaluates after it, and for a unit of an Operators
   Rule it applies the continuations that the definition lists: the later
   operands, the operator parts that the rule tries after them and what
   follows the rule. The step must show, for every unit of an Operators Rule,
   that the parse of the rule evaluates, after an operand or an operator part
   that it abandons, nothing beyond those continuations, as *Restarts* does
   for the other constructs; the restarts through a later operand are then
   those that the definition counts. In an Operators Rule `e` with the
   operands `"(" e R "x"`, `"(" e` and `"."`, called as `e R` with
   `R = [ ]*`, the first operand fails at each level after scanning the
   spaces and the second ends where the inner operand ended; the two share
   the call of `e`, so the definition puts `e` in the set for that call, which
   nests at one end. The rule then nests, the call of `R` that the
   continuation reaches after an operand is not shared, and the condition
   excludes the grammar.
4. **Contexts of growths in Lemma 5.** The proof must bound the evaluations
   that a context of growths repeats, which Lemma 3 leaves out: the calls
   that find no result with the same growing results and lookahead outcomes,
   and the growths of leaders inside other growths at their position, whose
   steps are shared only through equal results. It must show that the
   comparison of results by structure, which is bounded, loses only reuse,
   and that condition 5 covers every growth that is new in each step. Lemma 4
   bounds the activations owned by memoized evaluations after this lemma, in
   the order that it states: the growth steps, the evaluations in a context of
   growths, the activations that they own and the successful activations that
   a dropped step or a path that enters the growth again repeats must be bounded
   from the claims of Lemma 4 that count nothing, Lemmas 1 to 3, the memo and
   conditions 3 and 5, and not from the count of Lemma 4 on the activations
   owned by memoized evaluations. The claims that count nothing are *Failure
   after a source*, the restarts, *Direct activations*, *Repeats*, *Memoized
   links*, *Calls that call no rule*, *Nesting at one end*, *Seed reads*, *Own regions*, *Counterparts in the
   reference parse*, *Replays*, *Replayed activations*, *Answered calls*, *Covered keys*,
   *Reference links*, *Heads in the reference parse*, *Reference links lead to
   repeated evaluations*, *Links lead to repeated evaluations*, *Links to heads*,
   *Memoized links keep positions* and *Nothing in a memoized link is a head*,
   with the definitions between them; *Activations that do not succeed* and *Activations held by heads* come
   after this step. The links and *Memoized links* bound the repetitions of an
   activation at one position through one rule, position and level; the sum
   over the positions, which *Activations held by heads* makes for the
   activations that memoized evaluations own by charging them to the heads,
   must be made here for those that growth steps own, which are not
   evaluations of a key, and those that evaluations in a context of growths
   own, and for the growths
   that start at many positions, each at a call of a rule of the cycle that is
   not a left call, as the repetitions of a scanning source.
5. **Chains of bindings in Lemma 11.** The bound $L = O(n)$ that
   [Classification](../runtime/parsing.md#classification) states, which the
   theorem does not use. *Values of variables* in Lemma 11 proves it given
   that each binding occurs a bounded number of times in every chain. The
   step must show that conditions 1 and 4 ensure this: that a value reaches
   a later evaluation of the same binding only through a sequence that holds
   the binding, at any depth of expressions that keep its bindings, and a
   call that can reach its rule again, or a repetition around the binding,
   which condition 1 checks, or through the value of a rule passed to a
   caller and read there through a variable, which condition 4 checks; and
   that a chain that takes neither repeats each binding a bounded number of
   times, as `s = a:A {x := $a} A` does when `A` binds `x`.
6. **What a Document parse moves in Lemma 12.** The bound of Lemma 12 is in
   terms of $W$, $B$, $G$ and $Q$, which depend on the history of the Document.
   *Over a sequence of parses* removes the total retained size $W$: over the
   parses since the memo was last dropped, a removed entry counts where a parse
   stored it, an index and a record count in the parses that use them, and what
   remains of $W$ is the moved size $V_j$ of each parse, with $B_j$, $G_j$, $Q_j$
   and the key sizes $\Lambda_j$. A bound by the $n$, $K$ and $L$ of the parse
   would have to show that the nodes a parse moves are at most a constant times
   what a parse of the same text from scratch builds. That does not follow even
   if the calls of the parse are among those of the parse from scratch, which is
   not proved, and a node moves once in a parse, for two reasons. Nodes that the
   parse from scratch shares can be distinct here, since the values that a parse
   reuses were built by different parses: a call in a context of growths stores
   no entry, a lookup outside a lookahead does not take an entry stored inside
   one, and entries joined by a deletion each keep their value, so the step must
   bound the evaluations of one key whose nodes stay reachable, which needs a
   form of Lemmas 4 and 5 local to a region of the text. And an empty node is
   copied for each reference to it, not once. The replays of the edits in a move
   multiply the nodes by up to $E$, a factor that the bound keeps. $Q$, $G$ and
   the keys that $\Lambda_j$ measures count what earlier texts left in the memo,
   which it keeps until a store, an edit, the walk of a chain or the drop of the
   table removes it. An entry that no parse has stored or found to answer a
   lookup in the last 128 edits lags: a lookup treats it as a miss, and a walk
   that meets it removes it without comparing it
   ([Work of a Document Parse](../runtime/documents.md#work-of-a-document-parse)),
   so the entries of earlier texts that a chain holds are bounded by the work of
   that window of edits, not by $n$, $K$ and $L$. The step is to bound $V_j$,
   $B_j$, $G_j$, $Q_j$ and $\Lambda_j$, or to change what a Document reuses so
   that they are bounded, and then to restate that section with the tighter
   bounds. The memory that a Document keeps between parses is not a cost, and
   this proof does not bound it.
7. **Values built from reshaped values.** The [Cost View](cost-view.md) proof
   bounds the cost of the written grammar by a constant times that of its view,
   except for the values that actions build from a reshaped value: the value of
   an expression that a rewrite replaces where it changes that value, or a value
   that includes one through a capture, a sequence, a choice or the value of a
   rule. The rewrites that change a value are listed in
   [Rewrites That Change Values](cost-view.md#rewrites-that-change-values):
   N9, N10, N11, the left factoring of N12, the replacement of `!X X` by `_|_`
   (N12), the drop of every alternative with a value of a choice that keeps one
   without (N2, N3), the replacement of a sequence of one item, or of a choice
   of one alternative, by its item, which N1, N6 and the drop of the other
   alternatives make, and the drop by V1 of a recovery that makes its
   expression one element. They preserve what the expression
   matches and its recorded outcome, not its value, and the view applies them
   only where no predicate reads such a value; an action of the written grammar
   can still read one, and then reads another shape than the classifier sees in
   the view: `x:("a"* "a"*) => concat($x[0], …)` copies a list where the view
   has an element, and `len($x)` of `x:("abc" #cut / "z")` counts one element
   where the view has three characters. Prove that the values which actions
   build from a value that one of these rewrites reshapes, and the run-time
   errors of those actions, stay within a constant factor of what the
   classifier bounds on the view (or make the view keep the shape of every
   value that an action reads).
