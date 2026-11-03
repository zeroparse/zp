# Linear Cost

Arata Furukawa · Last revised 2026-11-03

This proof shows that the [Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)
bound the [cost](../runtime/parsing.md#cost) of every parse, so that the
classes Linear and Linear Given an Environment Bound of
[Classification](../runtime/parsing.md#classification) hold their promise. It
splits the cost into the parts that the specification counts and bounds each
part by one lemma, so that a new condition is added as a lemma, or as a case of
one, without changing the others.

Lemmas 1, 2, 3, 7, 8, 9 and 10 are proved, and the theorem is proved from the
others. Lemmas 4 to 6 are given as outlines: each states its claim, the
conditions it uses, and the steps that its proof must still make precise,
listed under [Open Steps](#open-steps).

## Setting

Fix a grammar and a start rule such that every rule that the start rule
reaches satisfies conditions 1 to 5, and an input of length $n$. Let $K$ and
$L$ be as in [Classification](../runtime/parsing.md#classification), and
$\hat{n} = n + 1$. The parse is the memoized parse of the
[memoization proof](memoization.md#setting) with the memoization of
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules).
"A constant" below means a number that depends only on the grammar and the
backend.

**Cost.** By [Cost](../runtime/parsing.md#cost), the cost of a parse is
$C = C_{\mathrm{body}} + C_{\mathrm{in}} + C_{\mathrm{it}} + C_{\mathrm{val}}$:
the rule bodies evaluated with each growth step counted, the input that
terminals, repetitions of character classes and lookaheads examine, the
iterations of repetitions and the operators applied, and the size of the
values that actions and predicates build.

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
bound, an operator that an Operators Rule applies, or an evaluation of the
skip expression of a `#recover` that is not a
[reused skip](../runtime/parsing.md#skip-runs). The growth of a cycle and the copying
operations, the other scanning sources, make no calls of their own beyond the
body evaluations of the growth.

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
grammar, counting the absence of a level as one.

The number of body evaluations of memoized rules that are neither growth
steps nor in a context of growths is at most
$2 \cdot |\text{rules}| \cdot |\text{levels}| \cdot K \hat{n}$.

*Proof.* By [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules),
a rule body is evaluated at most twice at a position for each binding level
and each combination of the values of the variables that the rule reads, with
three exceptions. The first concerns the rules that are not memoized, which the
claim leaves out. The second concerns the calls in a context of growths, whose
evaluations the claim leaves out. The third concerns growing rules, whose body
evaluations are growth steps; by
[Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)
a call of a leader grows it at the position of the call, so every body
evaluation of a leader is a growth step, and the claim leaves out all of
them. A rule on no cycle has no context of growths, since only the growths of
a rule's own cycle form its context, and the result of a rule of a cycle
outside every context is memoized like that of any rule. So every body
evaluation that the claim counts is one of the at most two that the section
allows for its rule, position, level and combination.

A position is one of the $n + 1$ positions of the input. By the definition of
$K$ in [Classification](../runtime/parsing.md#classification), the calls at
one position bind the variables that memoized rules read to at most $K$
distinct combinations of values. Summing two evaluations over the rules, the
levels, the positions and at most $K$ combinations at each position gives the
bound. The bound holds in each parse of a
[Document](../runtime/documents.md), whose memo entries from earlier parses
only answer calls, and in a [stream](../runtime/streaming.md), which discards
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
entry of a rule other than a leader to the second call at a position, which
gives the two evaluations.

## Lemma 4 (Scanned Cost)

*Outline.* Under condition 2, the scanned cost of all body evaluations, together
with the activations of their scanning sources, is at most a constant times
$K(L + 1)\hat{n}$.

For the walks of a [skip-run site](../runtime/parsing.md#skip-runs) in the
skip expression of a `#recover` that is not reused, the
[Skip Runs proof](skip-runs.md#lemma-2-cost) bounds the evaluations of the
element by $2(n + 1)$ plus a constant for each walk, and each walk is one
activation of the skip expression. That lemma is the case of this one for
skip-run sites. A reused skip is no scanning source, and Lemma 9 bounds its
walks.

The proof charges each step of a scanning source to a slot of the input that
it examines, or to the iteration or operator that makes it, and shows that
each slot is charged a constant number of times for each combination of the
values of variables. A slot is charged again only when a scanning source that
examined it is evaluated again over it, which happens when a unit that
contains the source fails after the source examined the slot, and the parser
continues at the start of the unit. Condition 2 requires that such a unit be
committing, which gives the following cases, one lemma each.

1. *Disjoint first characters.* The continuation does not consume the
   character at which it and the unit part, so nothing evaluated after the
   failure starts again inside the input that the unit examined beyond its
   shared calls.
2. *Shared calls.* A call that the unit and a continuation both evaluate first
   has the same result in both, by the [memoization theorem](memoization.md#theorem-memoization-never-changes-the-result)
   when the rule is memoized at both calls, or by repeating only its own scan
   when it calls no rule, so the continuation pays for it at most what the
   unit paid. A rule of a cycle other than a leader is memoized at a call that
   is not a left call: a growth of its cycle at the position of the call that
   was in progress would be evaluating, at that position, every rule from the
   growing leader to the caller, so each of them would have a left call of the
   next and lie on the cycle, and the call would be a left call. The call has
   no growth in its context, its result reads no growing result, and the memo
   stores it and answers the other call
   ([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)).
   A literal that is not empty, with the same case sensitivity, that every path
   of both begins with is shared too: both consume the same characters or both
   fail where they begin, so what follows it starts at the same position in
   both, and the continuation pays for it no more than the unit paid.
3. *No source after the shared calls.* The input that the unit examines beyond
   its shared calls is bounded by Lemma 1.
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

## Lemma 5 (Growth)

*Outline.* Under conditions 2, 3 and 5, and with the growths at the calls
that are not left calls counted as scanning sources by Lemma 4, the
evaluations that Lemma 3 leaves out for memoized rules number at most a
constant times $K \hat{n}$: the growth steps, and the body evaluations of
memoized rules in a context of growths, those of a rule of a cycle other than
a leader outside a growth step included. Their local and scanned cost is at
most a constant times $K(L + 1)\hat{n}$ more than Lemma 4 counts.

A growth at $p$ takes at most two more steps than the length it consumes
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)). By
condition 3 no step evaluates a scanning source at $p$ outside the left call
that grows and outside memoized calls of rules off the cycle, so each step
costs a constant at $p$, and its growing units examine the input from the end
of the seed. The input from the end of one seed to the end of the next is
examined by one step, and what a step examines beyond the end of its own
result is the failure of a growing unit after a scanning source, which
condition 2 checks. A growth is itself a scanning source at each call of a
rule of its cycle other than a left call, so its repetition at another
position is counted by Lemma 4.

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

*Outline.* Under conditions 1 and 4, the size of the values that actions and
predicates build is at most a constant times $K(L + 1)\hat{n}$.

A copy of a value that does not include the value of a call on its own
recursion, nor a fold accumulator, does not add up over the levels of a
recursion; when it is unbounded it is a scanning source, which Lemma 4 counts.
A copy in the lambda of `map`, `filter`, `foldl` or `foldr` over a list that
is not bounded runs once per element, copies at most a constant from
outside the lambda, and copies elements whose sizes add up, over all the
evaluations of the lambda, to at most a constant times $\hat{n}$, by the
[size degrees](#lemma-7-lambdas-over-bounded-lists) of condition 4; over a
bounded list, Lemma 7 makes the copies of the call
those of a constant number of copies outside the lambda. A copy of the
value of a variable costs at most $L$. Condition 1 keeps the value of a
variable from growing from its own earlier value, or, by Lemma 8, keeps it in
a set of constant size, so the number of combinations at a position is $K$, a
constant when every binding is a literal or binds a variable with finite
values.

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
its [size degree](../runtime/parsing.md#conditions-for-linear-cost), as
shown below; so do the three quantities of the initial value, with degrees
$e_S$, $e_M$ and $e_N$, and those of the list, which has $m$ elements, at
most a constant times $n^{l_N}$, of sizes $Y_j$ that add up to at most a
constant times $n^{l_S}$, each at most a constant times $n^{l_M}$. Let
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

*Size degrees.* The proof above does not use what follows, which is an
outline: it justifies the size degrees that the polynomial degrees read, and
step 7 of the [Open Steps](#open-steps) lists what it leaves to be made
precise. An own value has size at most a constant times one more than
the input that its match consumes, by induction on the matches: a literal is
a constant; a capture that is not in a positive lookahead holds an own value
of a part of the match; `+`, `text`, `list`, `concat` and constructors add
the sizes of their parts and a constant, and a member, an element or a
`filter` is at most what it is taken from; and a `map` or a fold over an own
list whose lambda reads nothing unbounded from outside it adds at most a
constant times the size of each element and a constant for it, the fold
reading its first parameter at most once. Distinct captures of a match hold
disjoint parts of the input. Reading a capture several times multiplies its
size by a constant, which does not compound, since a chain of rules that are
not on one recursion is at most as long as the grammar; the value of a call
on a recursion with the rule, read at most once and not as the list of a
lambda, which would add to each of its elements on every level, adds the
parts of each level once. A lambda over the captures of a repetition whose
lambdas read each member of their other parameters at most once, distinct
members being disjoint parts of a match, holds each part of each element, a
distinct match, at most once, and with it the values of the calls on the
recursion in it, so it adds the parts of each level once too; this holds
for the trees of a recursion whose values are not own as well. The levels
of a recursion and the elements of a repetition are distinct matches, and
only a constant number of nested matches can consume no input, since a
repetition stops at an empty iteration and a call at the same position is a
left call, whose growth consumes input on each step; so the own values of
distinct matches add up to at most a constant times $n$. A value built from
values of size degrees $e_i$ is bounded by the rules of condition 4 above
with $n^{e_i}$ in their place. The list of a repetition of elements of
degree $d$ has at most $n + 1$ elements, so its size has degree $d + 1$. For
a recursion whose values are not own, a value that holds the value of each
call on the recursion at most once, and not from a positive lookahead, where
two captures can hold the same value, is a tree whose nodes are distinct
matches, at most a constant times $n$ of them, each adding at most a
constant times $n^d$, where $d$ is the degree of what one level adds; so its
size has degree $d + 1$.

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
its set of values. The variables with finite values are the `bool` variables
and the `int` variables whose value sets are finite; `string` variables and
other `int` variables are not among them. A `bool` has as its set its initial
value and the values that its bindings can bind: a constant binding adds its
value, a binding that fails to evaluate adds none, and any other adds both.
The product of the sizes of the value sets of all variables with finite
values is at most 128.
Then $v$ holds a value of $S_v$ at every step of every parse, and the
variables with finite values contribute at most the product of their $|S_v|$,
a constant, to $K$.

*Proof.* For a `bool`, the initial value is in $S_v$, and every binding of
$v$ that succeeds binds its constant value, which is in $S_v$, or is one whose
value is not constant, in which case $S_v$ holds both values; so every value
that $v$ holds is in $S_v$. For an `int`, by induction on the steps of the
parse. By
[Variables](../language/predicates.md#variables), the parse starts with $v$
bound to its initial value, which is in $S_v$, and afterwards a lookup of $v$
gives the nearest binding of $v$ that has not been undone, or the initial
value. Bindings made in a called rule are not visible to its caller, and
backtracking undoes bindings, so every value that a lookup gives is the
initial value or a value that an evaluation of a binding bound earlier.

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

- a binding of $v$ binds a value that its evaluation gives from the value
  before it, which the step computes as for $b$ below;
- a predicate that reads only $v$ and literals succeeded, so the value passes
  it; the step keeps every value on which its evaluation is true or not known;
- a choice succeeds through one of its alternatives and an option through its
  expression or by matching nothing, so the value after it is in the union of
  the values after its paths;
- a negative lookahead undoes its bindings, and a call returns with the
  value before it, since bindings in a called rule are not visible to its
  caller;
- any other element that binds $v$ leaves a value that $v$ holds at some step
  of the parse, which is in $S_v$ by the induction hypothesis; an element that
  does not bind $v$ keeps it.

So $x$ reaches $b$. The binding reads only $v$, literals and operands
`len($c)`, and the length of the value of each capture $c$ is at most its
bound, since the value is the text of an expression that matches at most that
many characters, a list or tuple with at most that many elements, or the
value of a rule whose action gives such a value. A sequence builds a tuple of
the values of its items, in which a plain group of a sequence is replaced by
the elements of its own tuple, so the tuple has at most the number of items
counted with those groups spliced in; a sequence whose value is that of a
single item has the bound of that item. The value of $b$ is
therefore determined by $x$ and lengths within the bounds, by the rules of
[Operators and Functions](../language/actions.md#operators-and-functions),
for ints within $2^{53}$ in magnitude, which every backend computes alike:
it is in $S_v$, which holds the value that $b$ binds from each value that
reaches it at each combination of lengths, or the evaluation is an error and
binds nothing.

The values of the variables that memoized rules read at a position are
therefore drawn, for the variables with finite values, from the product of
their sets, so they contribute at most the product of the $|S_v|$ to the
number of combinations $K$. $\square$

The bound of 128 on the product keeps the constant $K$ small: a memoized
rule can be evaluated once for each combination of values at a position. The
bounds of 256 for a length and of 4096 combinations of lengths keep $S_v$
cheap to compute. The proof needs only that the bounds are finite. The values that reach a binding depend on the order of the elements
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
$r_d$ where it ends, and each is among the activations counted in $A$. $E(r_d)$ is at most $B$ when $r_d$ is memoized, 0 when it is not
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

## Theorem (Linear Cost)

When every rule that the start rule reaches satisfies conditions 1 to 5, every
parse from the start rule has a cost of at most $c \cdot K \cdot (L + 1) \cdot (n + 1)$
for a constant $c$. When no variable holds a string and every binding binds a
literal or a variable with finite values, $K$ is a constant by Lemma 8 and
$L = 0$, so the cost is at most $c' \cdot (n + 1)$.

*Proof, given Lemmas 4 to 6.* Each unit of $C$ is the local cost of a body
evaluation, the scanned cost of one, a call, or the walk cost. By Lemma 1 the
local costs add up to $c_0 (L + 1)$ times the number of body evaluations. The
body evaluations of memoized rules are at most a constant times $K (L + 1)
\hat{n}$: by Lemma 3 outside the growth steps and the contexts of growths, and
by Lemma 5 for the growth steps and the evaluations in a context of growths. By
Lemma 10 the body evaluations of the other rules are at most a constant times
those plus the activations plus one, and Lemmas 4 and 5 bound the activations.
By Lemma 2 the calls are at most a constant times the body evaluations plus the
activations. The scanned cost is bounded by Lemmas 4 and 5, the walk cost by
Lemma 9, and the values by Lemma 6. Each bound is a constant times $K (L + 1)
\hat{n}$. $\square$

The classifier reports the two classes only when the conditions hold, and it
over-approximates where it cannot decide them
([Classification](../runtime/parsing.md#classification)). That it does so is a
property of its implementation, checked by its tests; this proof covers the
step from the conditions to the bound.

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

The outlines leave these steps to be made precise.

1. **Charging in Lemma 4.** The scheme that assigns each scanned step to a
   slot, and the proof that a slot is charged a constant number of times,
   across nested units, rule calls and the two evaluations that the memo
   allows at a position. The specification argues each case for one unit; the
   proof must combine them over the units that enclose one another.
2. **"A unit that succeeds is not restarted."** A body that contains the unit
   can itself be evaluated again: by the memo, at most twice at a position;
   when its rule is not memoized, at each of its calls. Lemmas 3 and 10 count
   these evaluations; the proof must show that they do not multiply the
   scanned cost beyond a constant. Lemma 10 bounds the evaluations of the
   rules that are not memoized by the activations, so the bound of Lemma 4 on
   the activations must not rest on those evaluations, or the argument closes
   on itself.
3. **Continuations of growing units.** The case where the first step of a
   growth matches a part before the left call, as through `F` in
   `E = F "+" X / Y; F = E / M`, and the unit is also checked as an
   alternative that is not growing.
4. **Operators Rules.** The units of an Operators Rule, its `skip`, and right
   operands that apply operators themselves, in Lemmas 4 and 5.
5. **Contexts of growths in Lemma 5.** The proof must bound the evaluations
   that a context of growths repeats, which Lemma 3 leaves out: the calls
   that find no result with the same growing results and lookahead outcomes,
   and the growths of leaders inside other growths at their position, whose
   steps are shared only through equal results. It must show that the
   comparison of results by structure, which is bounded, loses only reuse,
   and that condition 5 covers every growth that is new in each step.
6. **Variables in Lemma 6.** The bound $L = O(n)$ that
   [Classification](../runtime/parsing.md#classification) states, and the
   comparisons of memo keys that hold strings, which cost up to $L$ per lookup.
   Besides, a copy outside the lambdas of a value whose size degree is above
   1: a string that large was built by copies that cost its length, and a
   list copies only its number of elements, which the copies or the
   repetition that built it paid; the step must charge each such copy to that
   work, which a constant number of actions read.
7. **Size degrees after Lemma 7.** The induction that bounds an own value
   by the input its match consumes, over every form of an action and the
   bodies of the rules it calls, and the bound on the trees of a recursion
   whose values are not own, which must show that a value holding each call
   on the recursion once, directly or through a lambda over a repetition,
   adds each level once.
