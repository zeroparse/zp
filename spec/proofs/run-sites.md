# Run Sites

Arata Furukawa · Last revised 2026-11-07

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
predicate that reads or binds a variable, and calls no rule that reads one,
directly or indirectly. Then $E(r, \eta)$ gives the same for every $\eta$,
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
[Run Sites](../runtime/parsing.md#run-sites), a constant (64 in the
reference implementation).

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
sites. Generated Go and TypeScript parsers keep records only for skip-run
sites. Their run-site repetitions are ordinary repetitions, whose cost these
lemmas do not bound.
