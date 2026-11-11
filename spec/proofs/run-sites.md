# Run Sites

Arata Furukawa · Last revised 2026-11-09

This proof covers the records of [run sites](../runtime/parsing.md#run-sites).
A reused record gives what the rest of the walk would have given, in the
environment where the walk reuses it (Lemma 1). The walks of a site evaluate
its element a number of times linear in the input and the number of walks,
once the site keeps records (Lemma 2). A walk of a site that is not hot costs
its length, and those lengths add up to the input plus a constant per walk
(Lemma 3). It extends the [Skip Runs](skip-runs.md) proof, whose definitions
of outcomes, ranges, slots and walks it uses. It differs from that proof where a
run site differs from a skip-run site: the element calls rules, captures,
builds values, recovers errors, reads and binds variables, and the walks of a
site nest.

## Setting

Fix a parse, a run site $\sigma$ and a lookahead state $\lambda$. Let $n$ be
the length of the input. The element of $\sigma$ is $e$, and its minimum is
$c_{\min}$. By the conditions on run sites, $e$ contains no Operators Rule,
`#stream`, `#missing`, `#error` or `#diagnostic`. Every repetition without an
upper bound in $e$ is a run site or repeats a single character. Every rule
that $e$ calls, directly or indirectly, is called without a level, is in no
left-recursion cycle, and contains none of those constructs. No `#cut` in $e$
applies to $\sigma$. A parse with missing-token insertion keeps no records, so
assume that this parse inserts none.

**Iterations.** Let $E(r, \eta)$ be the evaluation of $e$ at position $r$ in
the environment $\eta$. It fails or matches up to some $r' \ge r$. It has a
farthest failure $f$ with expectations $X$, examines a range $[l, h]$, and
recovers a sequence of errors $R$. When it matches, it also leaves an
environment $\eta'$ and a value $v$, or no value when the iterations of
$\sigma$ build none. What $E(r, \eta)$ gives depends only on $r$, $\eta$,
$\lambda$ and the input:

- a capture of $e$ is visible to the iteration alone, so no iteration reads
  what an earlier one captured;
- no rule that $e$ calls is in a left-recursion cycle, so no growth in progress
  outside $e$ changes what a call inside it returns, and a call of such a rule
  at $r$ does not reach $\sigma$ again at $r$ before it consumes input;
- memoized results that $E(r, \eta)$ reads or stores change its cost, never
  what it gives ([Memoization](memoization.md)), and neither do the records
  of other sites, by Lemma 1 for them and by the
  [Skip Runs](skip-runs.md#lemma-1-exactness) proof;
- a value that $E(r, \eta)$ adds to the walk is final, since a node in a
  record is not changed by a later step
  ([Run Sites](../runtime/parsing.md#run-sites)).

**Environments.** The site is **free of the environment** when $e$ has no
predicate that reads or binds a variable, no inline action that reads one,
and calls no rule that reads one, directly or indirectly, in a predicate or
an action. (An inline action is a call of a hidden rule whose action is the
inline action, so the last condition covers it.) Then $E(r, \eta)$ gives the same for every $\eta$,
and when it matches, $\eta' = \eta$:

- a predicate or action of $e$ or of a rule that $e$ calls reads no variable,
  so its outcome and value do not depend on $\eta$;
- a call of a rule that reads no variable is memoized without the
  environment in its key ([Interaction With
  Memoization](../language/predicates.md#interaction-with-memoization)), so a
  result that it reuses was stored under any environment and is the same;
- $e$ binds no variable outside its calls, and a binding that a called rule
  makes is undone when that rule returns, so the environment after an
  iteration is the one before it.

For such a site, write $E(r)$ for $E(r, \eta)$. Every skip-run site is free of
the environment by its conditions.

**The rest of a walk.** Define $\mathrm{Rest}(r, \eta)$, the end, end
environment, outcome, range, values and errors of the walk from $r$ on in
$\eta$, as in the Skip Runs proof:

- if $E(r, \eta)$ fails or matches up to $r$, then
  $\mathrm{Rest}(r, \eta)$ ends at $r$ in $\eta$, with the outcome and range
  of $E(r, \eta)$, no values, and the errors $R$ that the iteration recovered;
- if $E(r, \eta)$ matches up to $r' > r$ in $\eta'$ with value $v$, and
  $\mathrm{Rest}(r', \eta')$ ends at $\mathrm{end}'$ in $\eta''$ with outcome
  $o'$, range $\rho'$, values $V'$ and errors $R'$, then
  $\mathrm{Rest}(r, \eta)$ ends at $\mathrm{end}'$ in $\eta''$, with outcome
  $(f, X) \cdot o'$, range $[l, h] \sqcup \rho'$, values $v\,V'$ and errors
  $R\,R'$.

An iteration that fails recovers no error that survives, since its errors are
dropped with the failure. The recursion terminates because $r' > r$. For a
site free of the environment, $\mathrm{Rest}(r, \eta)$ does not depend on
$\eta$, by induction on $n - r$, and its end environment is $\eta$. Write
$\mathrm{Rest}(r)$ for it, without the end environment.

**Keys.** A record of a site that is not free of the environment is stored
under its position and the environment object in which the walk visited that
position. A walk looks one up under its position and its current environment
object. A record of a site free of the environment is stored and looked up
under its position alone. Two environment objects that are equal but distinct
give distinct keys. Since that changes only which records a walk finds, never
what one holds, the lemmas below hold for keys by object as stated.

## Lemma 1 (Exactness)

For a site that is not free of the environment, every record stored under
$(r, \eta)$ equals $\mathrm{Rest}(r, \eta)$. For a site free of the
environment, every record stored under $r$ equals $\mathrm{Rest}(r)$. A walk
that reuses a record ends where the walk that evaluated the iterations from
there would end. Its environment is then the record's end environment, or its
own for a site free of the environment. It leaves the enclosing result with
the same outcome and range, lists the same values after its own, recovers the
same errors after its own, and succeeds or fails as that walk would.

*Proof.* By induction on the order in which records are stored, as for
[Lemma 1](skip-runs.md#lemma-1-exactness) of the Skip Runs proof.

A recording walk visits $r_j < \dots < r_k$ while recording, in the
environments $\eta_j, \dots, \eta_k$ in which the iterations at those
positions begin. Each visit is an iteration that the parser isolates: its
expectations and examined range are set aside and then recorded into the
enclosing result. The walk notes the number of values that it has collected
and the errors that the iteration recovered. Each $\eta_{m+1}$ is the
environment that $E(r_m, \eta_m)$ left. The walk ends in one of two ways.

- At an iteration that fails or matches up to its own start. That is the last
  visit $r_k$. Its record ends at $r_k$ in $\eta_k$ with the outcome, range and
  errors of $E(r_k, \eta_k)$ and no values, which is
  $\mathrm{Rest}(r_k, \eta_k)$.
- At a position $r_{k+1}$ with a record found under its key. The record was
  stored earlier, so it is $\mathrm{Rest}(r_{k+1}, \eta_{k+1})$ by the
  induction hypothesis. For a site free of the environment it is
  $\mathrm{Rest}(r_{k+1})$, which is the same.

The other records are stored from $r_k$ back to $r_j$. Each combines the
iteration at $r_m$ with the record just stored for $r_{m+1}$:

- the outcome and range by $\cdot$ and $\sqcup$;
- the end environment of the record after it;
- the values as the suffix of the walk's list from the value of the iteration
  at $r_m$ on, whose tail is the list that the next record holds;
- the errors as those of the iteration followed by those of the next record.

That is the second case of the definition, so the record is
$\mathrm{Rest}(r_m, \eta_m)$. For a site free of the environment, the walk
keeps no environment in the record, and the record is $\mathrm{Rest}(r_m)$.
This proves the first claim.

A walk looks up a slot only after it has matched $c_{\min}$ iterations, so the
walk that evaluates on from there succeeds, and so does the walk that reuses
the record. The evaluating walk records the outcome and range of each
iteration into the enclosing result in order, appends each value, and
recovers each error. By associativity, a reuse that does these at once leaves
the same result. A reuse in a site free of the environment keeps the walk's
environment. By the setting, that is the environment that evaluating the
iterations would leave. $\square$

The walks of run sites nest. An iteration of $\sigma$ can call rules that walk
other sites, or $\sigma$ itself at a later position. Lemma 1 holds for each
site, because the records that an inner walk stores are stored before the
iteration around it ends, and the induction runs over the order of storing,
across all the walks of the parse. Whether a walk is plain or recording, and
whether the site is hot, decides only which positions get records. So neither
changes the result of a parse.

## Lemma 2 (Cost Once Hot)

Fix a key class $\kappa$. For a site free of the environment, all walks of
$(\sigma, \lambda)$ share one class. Otherwise, each environment object in
which a walk visits a position is a class of its own. In a parse with $W$
walks of $(\sigma, \lambda)$ while the site is hot, the element is evaluated
in those walks at most $2(n + 1)$ times at positions whose slot is fresh or
not yet recorded under $\kappa$, summed over the classes that occur, plus
$W (c_{\min} + 1)$ times. The walks look up at most
$n + 1 + W (c_{\min} + 1)$ slots for each class and store at most as many
records as they evaluate iterations while recording.

*Proof.* The marks of visited positions are one bit for each position, for
every class. A mark moves a slot from fresh to visited for all classes at
once. A record moves the slot of its key to recorded. Sort the evaluations as
in [Lemma 2](skip-runs.md#lemma-2-cost) of the Skip Runs proof.

1. *A plain walk at a fresh slot.* Each position is marked at most once, so
   there are at most $n + 1$ such evaluations at positions that the walk
   marks, plus at most one per walk at the position that it does not mark.
2. *A recording walk at a slot not recorded under its key.* The walk stores a
   record under that key for every position it visited while recording, once
   it ends. While it runs, an inner walk of $\sigma$ can begin, but only inside
   an iteration at $r_m$ and at a position after $r_m$. A walk at $r_m$ itself
   would be a call that reaches $\sigma$ again at $r_m$ before it consumes
   input, which the conditions exclude. The inner walk ends, and stores its
   records, before the iteration at $r_m$ ends. The outer walk visits its later
   positions only after that, so it finds the inner walk's records under the
   same key. So each slot and class is evaluated this way at most once before
   it is recorded: at most $n + 1$ evaluations per class.
3. *A recording walk at a recorded slot.* This happens only within the first
   $c_{\min}$ iterations of the walk: at most $W c_{\min}$ evaluations.

A walk that a limit or a cancellation stops stores nothing, and no evaluation
follows it. Lookups precede evaluations of cases 2 and 3 or end a walk.
$\square$

For a site free of the environment there is one class, so the walks of
$(\sigma, \lambda)$ evaluate the element at most $2(n + 1) + W(c_{\min} + 1)$
times while hot. For a site that is not free of the environment, the bound
grows with the number of environment objects in which its walks visit one
position. Nothing bounds that number by the input: a binding before each walk
makes a new object, and
[Run Sites](../runtime/parsing.md#run-sites) gives a grammar where this costs
more than linear.

## Lemma 3 (Cost Before Hot)

In a parse with $W$ walks of $(\sigma, \lambda)$ before the site turns hot,
those walks evaluate the element at most
$2(n + 1) + (c_{\mathrm{hot}} + 1) W$ times.

Here $c_{\mathrm{hot}}$ is the minimum length of
[Run Sites](../runtime/parsing.md#run-sites), a constant.

*Proof.* A walk from $s$ that ends at $t$ evaluates the element at most
$t - s + 1$ times: each iteration that matches consumes input, and the last
one fails or consumes none. The site tells its state of a walk only when
$t - s \ge c_{\mathrm{hot}}$, and keeps the furthest end $F$ of those
walks ([Run Sites](../runtime/parsing.md#run-sites)). Sort the walks that do
not turn the site hot.

- A walk with $t - s < c_{\mathrm{hot}}$ evaluates the element at most
  $c_{\mathrm{hot}}$ times.
- A walk with $t - s \ge c_{\mathrm{hot}}$ walks less than
  $c_{\mathrm{hot}}$ of the input below $F$, since otherwise it would turn
  the site hot. Above $F$ it walks the input from $\max(s, F)$ to $t$ and moves
  $F$ to $t$. These ranges above $F$ are disjoint across the walks, so they
  add up to at most $n$.

The walk that turns the site hot evaluates the element at most $n + 1$ times,
and it is the last walk before the site is hot. Later walks fall under
Lemma 2. $\square$

So the walks of a site cost, before and after it turns hot, at most a constant
times $n + W$ evaluations of the element when it is free of the environment.

## Values

Lemma 2 counts evaluations of the element, not the cost of the list of values
that a walk returns. A walk of a site whose iterations build values places its
own values before the recorded suffix that it reaches. When that suffix
starts at the front of its buffer, the walk writes them in place. Otherwise it
copies the suffix. Walks that reach a record in the middle of another walk's
buffer, as walks from positions of different parity do in
`rule main = (r "!" / [ab])* $$; rule r = ("ab" / "b")*` on `ab` repeated
$n$ times, each copy a suffix of length $\Theta(n)$, so the copies cost
$\Theta(n^2)$. The bounds above hold for sites whose walks build no value. A
site whose walks build values is bounded only by the copies of its lists.

## Scope

The lemmas cover the engine's three backends, which keep records for run
sites, in every kind of parse they run:

- **Loaded programs.** A [compiled grammar](../runtime/compiled-grammars.md)
  marks in the [instruction](../bytecode/README.md) of each site whether it
  is free of the environment. A program loaded from one keys the records of a
  site by the environment unless its instruction marks it free, so it keeps
  the keys of the program it was compiled from, and a site of which nothing
  is marked is keyed, for which Lemma 1 holds as for any site that is not
  free.
- **Streams.** A [stream](../runtime/streaming.md) drops the records of the
  positions before the input it commits. A dropped record is never looked up
  again, since no walk returns before a commit, so Lemma 1 holds of the
  records that remain, and Lemmas 2 and 3 count fewer evaluations.
- **Documents.** The records of a parse last for that parse: a parse of an
  [incremental document](../runtime/documents.md) starts without them. A
  repetition that the document resumes is not walked as a run site in that
  parse, and runs as an ordinary repetition, which the lemmas do not need.
  Every other site walks as in a fresh parse.

 Generated Go and TypeScript parsers keep records only for skip-run
sites. Their run-site repetitions are ordinary repetitions, whose cost these
lemmas do not bound.

## Scan Sites

This section covers the records of [scan sites](../runtime/parsing.md#scan-sites). Lemma 4 is the
exactness of a reused record, Lemma 5 bounds what the records add to the cost of a parse, and Lemma 6
bounds what evaluating a scan-only rule again at one position costs.

**Setting.** A scan site $\sigma$ repeats a character class $C$ (or any character) without an upper
bound, with minimum $c_{\min}$, in a rule that calls no rule; it is neither a site of a skip run nor
a `#stream` repetition. The iteration of $\sigma$ is the test of $C$ and nothing else: it calls no
rule, binds and reads no variable, and has no predicate, lookahead, capture or action inside it,
whatever the rule has around $\sigma$. So an iteration reads the one character at its position and
nothing else, and no state of the parse outside the input and the position changes what it gives.
What the rule does around $\sigma$ (a capture or label of it, an action that reads its list, a
predicate, a lookahead, an attribute) runs after the walk or before it and sees only what the walk
gives, so it is evaluated as without records once the walk gives the same. Positions are in the unit of the input
(characters, or bytes when the unit is bytes), and a character of the input takes one or more units.
For a position $r$ let $\mathrm{end}(r)$ be the least position $q \ge r$ at which the input holds no
character of $C$ (the end of the input counts as such a position). The walk of $\sigma$ from $r$
matches the characters that begin at $r, \dots, \mathrm{end}(r)$, up to $q = \mathrm{end}(r)$, and
nothing else. Define $\mathrm{Rest}(r)$ as the tuple of

- the end $q = \mathrm{end}(r)$, a position,
- the number $k$ of characters matched (the iterations that succeed), which equals $q - r$ only when
  every character takes one unit,
- the examined range, which ends at the furthest position that an iteration examined: just after the
  character at $q$ (or at the end of the input), since the iteration that fails at $q$ looks at that
  character and no later one, or further when an earlier iteration read an invalid byte of a
  bytes-unit input together with the bytes that follow it. It is a function of $r$ and the input too,
  and a record holds it whole, which the iteration at $q$ alone does not give,
- the elements: one value for each of the $k$ characters, which depends on the character and its
  position alone,
- the failure of the iteration at $q$: the expectation of $C$ at $q$.

$\mathrm{Rest}(r)$ is a function of $r$ and the input alone, by the setting. A parse with missing-token
insertion keeps no records, so assume it inserts none. The record holds $q$ and $k$ as separate fields,
so nothing below derives one from the other.

**Silence.** A lookahead only silences the expectations that a failed iteration reports
(`Expect` returns when `Silent` is positive). The walk puts none of them into the record, and
$\mathrm{Rest}(r)$ does not depend on them, so a walk in a lookahead and a walk outside it that begin
at $r$ make equal records. This is why a site keeps one table of records for both (the hot state is kept
apart for each, and decides only which walks look).

## Lemma 4 (Exactness of Scan Records)

Every record stored by $\sigma$ at $s$ equals $\mathrm{Rest}(s)$ without the failure, which a walk
that takes the record derives again. A walk that takes the record at its $T$-th iteration ends where the
walk that went on from there would end, with the same outcome, farthest failure and expectations,
examined range, count and elements. No node that a record holds is changed after the record is made.

*Proof.* A walk of $\sigma$ that began at $s$ stores a record when it ends at $q$ after it matched at
least $T$ characters, and only if the input it examined, up to just after $q$, is available (the parse
has all of its input, or that range lies in the part of a stream that is loaded; the input is filled up
to each character before it is examined, so the second always holds unless the input has ended, and the
test only guards that). The iterations it ran are those of the definition, so the record holds
$q = \mathrm{end}(s)$, the count $k$ of characters it matched (not $q - s$), the range it examined (the
first $T$ iterations and the rest, whose union is the range of $\mathrm{Rest}(s)$) and the elements it
built, which are $\mathrm{Rest}(s)$'s. The availability condition makes $q$ the same on any longer prefix
of a stream: the character at $q$ is in the loaded input, so it is not in $C$ whatever follows. This
proves the first claim.

A walk that began at $s$ and has matched $T$ characters is at $s'$, the position after those
characters, and would go on to match the remaining $k - T$ characters and fail at $q$. Taking the record
moves the position to $q$, examines the recorded range and sets the count to $k$ and the elements to the
record's, which hold the first $T$ iterations' as well; they equal those the walk had built (the same
characters at the same positions). The count is the stored $k$, which is the number of characters in
$[s, q)$, so it is the count of the walk that went on, in every unit; no position is subtracted from
another. The loop then runs the iteration at $q$, which fails there with the expectation of $C$ at $q$,
or at the end of the input with that of the end, and examines that character: the outcome, farthest
failure and expectations of the walk that went on. Examined ranges only grow, so touching the recorded
range and then that of the iteration at $q$ gives the range of the walk that went on. The check of the
minimum uses the count $k$. The walk that went on and the walk that took the record leave the same
result.

It remains that a node a record holds is not changed. The elements are nodes that the repetition made
for the characters, and a rule changes a node only if the node is still fresh when it receives it as
the value of its body, to label it or to attach captures; a node that the rule that made it may still
change is final before another holder can see it, as the elements of the records of run sites are
([Run Sites](../runtime/parsing.md#run-sites)). A walk marks every element final when it stores its record, before any other
walk can reach the record, and a walk that takes a record gets elements that are final already. The
element of a scan site is never the value of a rule body in any case: an iteration has no capture and
no action, so the value that the walk gives is the list node that it makes, and what the rule makes of
it is that list, a node that holds it, or the value of an action, which reads the elements and changes
none of them; a capture of the list attaches to the list or to a node around it, never to an element. The list node is new for each walk (it is the node a rule labels) and holds the
shared elements as children only. The same holds for the list node that a walk of a scan site makes at its end: it is made after the elements
are marked, and shares nothing else. So the lists of two walks share nodes that nothing changes, and the
sharing changes no list.

Last, the sharing across [document](../runtime/documents.md) parses. The records are in the tables of
one parse and are dropped with it; no record is carried to the next parse. What persists is a tree,
whose lists may share elements. A reused tree is moved in place when an edit comes before
it, and a node moves only if its generation is below the current one and takes the current one when it
moves, so a node that two lists share moves once. Both lists cover the same range, as the walks began
at one position and ended at one, so the shift that an edit gives to the one is the shift it gives to
the other (the nodes of a reused result move together; an edit never falls inside the input that a
reused result examined). The elements are not empty (a character is at least one unit), so none is
copied as an empty node is, which would break the sharing, and the write of the moved pointer into the
shared slice of children stores the pointer already there. The same sharing of elements between lists
already happens for the nodes that rule memo entries and the records of run sites share. $\square$

Which positions have records, and the hot state of the site, decide only which walks find a record,
never what a record holds, so none changes the result of a parse. A record is looked up and made only
when no [document](../runtime/documents.md) run resumes the repetition, and the records last for one
parse.

## Lemma 5 (Cost of Scan Records)

A parse with the records of scan sites charges at most two steps more for each walk of a scan site
that reaches $T$ characters than the same parse without them, and no more for the other walks. So,
where the walks charge their tests, its steps are at most $1 + 2/(T + 1)$ times those of the parse
without them.

*Proof.* By Lemma 4 a walk that takes a record ends where it would have ended, with the same count,
elements, examined range and expectation, so the parse with records makes the same walks, at the
same positions and in the same order, as the parse without them, and differs only inside the walks
that reach $T$ characters. Without records a walk that matches $k$ characters charges one step for
each test of a character, $k + 1$ in all. (A build that charges no test of a character charges
only the steps of the records below, at most two for each walk, and the ratio is then not
claimed.) With records a walk that matches fewer than $T$ characters
is the same walk. A walk that reaches $T$ characters charges the first $T$ tests, then:

- at a site that is not hot, nothing more than the rest of its tests: noting the walk in the cold
  state charges no step, so it costs $k + 1$;
- at a hot site with a record of its start, one step for the lookup and one for the slot of the
  record, and one for the test of the character at the recorded end, which fails, so it costs
  $T + 3 \le k + 3$;
- at a hot site with no record, one step for the lookup, the rest of its tests and one step for the
  record that it makes, so it costs $k + 3$.

These are the charges that [Scan Sites](../runtime/parsing.md#scan-sites) states for every
backend that keeps records.

Each case costs at most $k + 3$, which is two steps above $k + 1$. A walk that reaches $T$ characters costs at least
$T + 1$ steps without records, which gives the ratio. $\square$

The bound is per walk, so it needs no count of the walks: the walks that start at different
positions of one run, which find no record, each pay at most the two steps, and the cold state,
kept apart for the walks inside lookaheads, only decides which walks look up. Parsers generated as
Go source keep no records.

## Lemma 6 (Walks Again From One Start)

Let a parse insert no missing tokens and resume no repetition of a document. For a scan site $\sigma$
and a start $s$, at most three walks of $\sigma$ from $s$ in each mode match more than $T$
characters and do not take a record, where the modes are the four pairs of whether the walk is
inside a lookahead and whether it builds a value. Every other walk of $\sigma$ from $s$ costs at most
$T + 3$ steps. So let $\rho$ be a scan-only rule
([Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)) and $p$ a position:
all the evaluations of calls of $\rho$ at $p$ but a constant number cost at most a constant each, and
each of the others costs at most $1 + 2/(T + 1)$ times what the first evaluation at $p$ would cost
without records.

*Proof.* A walk takes a record of $s$ when its site is hot in its lookahead state and the table of
its value mode holds a record of $s$. The hot state of a pair of a site and a lookahead state only
turns on, and the record of a start, once stored, stays for the parse, since the stream drops only the
records of starts that no later walk reaches. A walk of $\sigma$ from $s$ that matches $T$ characters
or more at a site that is not hot notes itself in the cold state: the furthest end becomes at least
the end of the walk, $\mathrm{end}(s) \ge s + T$. So the next such walk from $s$ in the same
lookahead state starts below the furthest end and walks $T$ characters below it, and turns the site
hot. So at most two walks from $s$ in a mode find the site cold, and once it is hot, the first walk
from $s$ without a record stores one, which every later walk from $s$ in the value mode takes:
at most three walks in a mode take no record. A walk that takes one costs $T + 3$ steps by Lemma 5,
and a walk that matches fewer than $T$ characters costs at most $T$.

Since $\rho$ calls no rule and reads no variable, and the walks give what they give without records
(Lemma 4), every evaluation of $\rho$ at $p$ takes the same path through the body and gives the same
result; only the expectations differ, which a lookahead silences. Every repetition without an upper
bound in $\rho$ is a scan site and an iteration of a scan site calls nothing, so the path makes the
same walks of sites, from the same starts, in each evaluation, a number $w$ of them bounded by the
grammar (the product of the upper bounds of the bounded repetitions around each site), and outside
the walks it is a finite expression that examines a constant amount of the input, with no scanning
source other than the sites. So an evaluation costs a constant $c$ outside its walks. By the first
part, at most $12w$ walks in all the evaluations at $p$ match more than $T$ characters and take no
record, so at most $12w$ evaluations make such a walk. Every other evaluation costs at most
$c + w(T + 3)$, and those that make one cost at most what the parse without records pays for the
evaluation, with the excess of Lemma 5, which is the same evaluation as the first at $p$. $\square$

A parse with missing-token insertion keeps no records and is outside the classes. A document that
resumes a repetition resumes it without records, as Lemma 4 states; the bound is for the walks that a
parse makes. Walks from different starts each pay their own length, which this lemma does not bound.
