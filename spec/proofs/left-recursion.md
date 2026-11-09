# Left-Recursion Growth

Arata Furukawa · Last revised 2026-11-09

This proof shows that the growth of a left-recursive rule at a position ends,
after a number of steps that the input it consumes bounds, and that its result
is unique: a growth with a given key, start state and context has one result,
which every parse gives, with or without
[memoization](../runtime/parsing.md#memoization). It covers a growth of
one leader, growths of several leaders of one cycle that are open at the same
position, the members of a cycle that are not leaders, and the dependency memo
that [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)
describes, with its jumps over runs of steps. The definitions are those of the
[Memoization proof](memoization.md#setting), which this proof extends, and its
scope carries over. A parser that does not memoize a call in a context
evaluates it, as [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)
allows; that is a subset of what $\mathcal{M}$ may do, so the proof covers it.

## Definitions From the Specification

The proof cites the following, and no other rule about growth.

- **(G1) Growing a seed.**
  [Left Recursion](../language/parser-expressions.md#left-recursion), steps 1
  and 2. While a leader $L$ is evaluated at $p$, a call of $L$ at $p$ fails.
  If the body then fails, the call fails. Otherwise its value is the current
  result, the body is evaluated again with calls of $L$ at $p$ succeeding with
  the current result, and a body that succeeds and ends farther than the
  current result becomes the current result, until one does not. Then the
  current result is the value of the call. A call at $p$ reads the current
  result whatever the values of the variables that $L$ reads.
- **(G2) Cycles.** The same section: in an indirect left recursion one rule of
  the cycle is grown and the others are evaluated again in each iteration. A
  left-recursion cycle is a set of rules that call one another before
  consuming input, as static analysis finds them.
- **(G3) Left calls and leaders.**
  [Conditions for Linear
  Cost](../runtime/parsing.md#conditions-for-linear-cost):
  a left call is a call of a rule of a cycle, in the body of a rule of the same
  cycle, that can run before the body consumes input. The rules that grow are
  the leaders, fixed by the grammar: until every cycle of left calls passes
  through a leader, the rule declared first on a cycle that passes through none
  becomes one.
- **(G4) Termination.** [Termination](../runtime/parsing.md#termination): a
  call of a leader where it grows reads its current result; a repetition stops
  at the first iteration that consumes no input; a recovery applies only when
  its skip expression consumes input; actions have no recursion.
- **(G5) Steps and contexts.**
  [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules): a
  growing rule evaluates its body once for each step, and the number of steps
  is at most two more than the length of the input it consumes. Only the
  growths of the rule's own cycle form the context of a call. A leader that
  grows where another growth of its cycle is in progress grows again in each
  context in which it is called there; a step of such a growth may reuse an
  earlier step under the same condition, and a run of steps that read no other
  growing result may be taken at once.

## Setting

Fix a grammar, a start rule, an input of length $n$ and a position unit. The
reference parse $\mathcal{R}$ and the memoized parse $\mathcal{M}$ are those of
the Memoization proof, with its outcomes $\omega = (s, q, v, \varepsilon, o)$,
its growths, current results, contexts $\Gamma$ and present sets. The parse
records no insertion of [`#missing`](../language/pragmas.md#missing) tokens,
and no limit stops it, as in that proof.

**The left-call graph.** Let $G$ be the graph whose nodes are the rules and
which has an edge $A \to B$ for each left call of $B$ in the body of $A$ (G3),
$\Lambda$ the set of leaders, and $R$ the number of rules.

**Keys.** A growth is open for a key $\kappa = (L, \ell)$ of a leader $L$ and a
binding level $\ell$, where $\ell$ is none or the level of a
`level(L, name)` call. The grammar has finitely many levels, so the set $K$ of
keys has at most $N = |\Lambda| \cdot (1 + \text{levels})$ elements. A call
reads the current result of the open growth of its own key at its position.

**Growth.** Let a call of $L$ with key $\kappa$ start at $p$ in a state $\sigma$
(the position, variables, captures and cut state that Memoization
[Lemma 1](memoization.md#lemma-1-context) names) and a context $\Gamma$. For a
result $r$, let $B(r)$ be the outcome of evaluating the body of $L$ at $p$ in
$\sigma$ and $\Gamma$ with the calls of $\kappa$ at $p$ answered with $r$, and
let $\bot$ be the failure. The **growth** is the sequence

$$r_0 = \bot,\qquad \omega_1 = B(r_0),\qquad \omega_{i+1} = B(r_i)$$

where $r_i = \omega_i$ for $i \ge 1$ as long as $\omega_i$ matches and, for
$i \ge 2$, ends farther than $r_{i-1}$. It stops at the first $\omega_j$ that
fails ($j = 1$) or does not end farther than $r_{j-1}$ ($j \ge 2$), and then
$k = j - 1$ and the result of the call is $r_k$ (the failure if $k = 0$). This
is (G1). Each $\omega_i$ is a **step**, so the growth makes $k + 1$ steps. The
step $\omega_1$ is the **seed**.

The result of the call is the value, end and recovered errors of $r_k$: the
recovered errors of the step that stopped the growth are dropped, and its
farthest failure and examined range are kept.

## Growth Rules

The proof cites the following rules of
[Growths of Left Recursion](../runtime/parsing.md#growths-of-left-recursion),
numbered as the proofs below use them.

- **(A1) Left calls are complete.** Every call that an evaluation makes at the
  position where the body of its rule started is found by the analysis of
  [Left Recursion](../language/parser-expressions.md#left-recursion) as a call
  that may run before consuming input, so each such call of a rule of the
  caller's cycle is an edge of $G$.
- **(A2) Leaders cut every cycle.** Every cycle of $G$ passes through a
  leader.
- **(A3) Positions do not decrease.** A call is made at a position that is not
  before the start of the body that contains it. Memoization Lemma 2 takes the
  same.
- **(A4) Growths are keyed per level.** A call reads the current result of a
  growth only if it has the same leader and the same binding level.
- **(A5) Runs taken at once consist of steps that adopted their result.**
  A run of steps that (G5) allows to be taken at once contains only steps
  whose result became the current result. The step that stops a growth is
  evaluated, or answered from a memoized step, singly.
- **(A6) A leader always grows.** A leader is memoized whatever the number of
  places that call it, so a call of a leader whose result at the position is
  not yet in the memo grows there.

## Lemma 1 (Chains at One Position)

Let a **chain** be calls $c_1, \dots, c_m$ in $\mathcal{R}$ where each $c_{j+1}$
is made within the evaluation of $c_j$, at the position where the body of $c_j$
started. Then $m \le B$, a number that depends only on the grammar:
$B = (N + 1) \cdot R + N + 1$.

*Proof.* By (A1) each pair $c_j, c_{j+1}$ is an edge of $G$, so the rules of
the chain form a walk in $G$. A call of a leader $L$ with key $\kappa$ whose
growth is open at that position is answered with the current result (G1) and
makes no further call, so it can only be the last call of the chain. Every
other call of a leader opens a growth of its key, which stays open until the
call ends, so no key occurs twice among those calls: at most $N$ calls of the
chain are calls of leaders that evaluate a body. Remove them from the walk. The
pieces that remain are walks in $G$ without its leaders, which has no cycle by
(A2), so no rule repeats in a piece and a piece has at most $R$ calls. There
are at most $N + 1$ pieces. Counting the removed calls and a last call that
reads a seed, $m \le (N + 1) R + N + 1$. $\square$

## Lemma 2 (Steps of a Growth)

A growth makes at most $e - p + 2$ steps, where $e$ is the end of its result and
$p$ the position of the call, and so at most $n - p + 2$ steps, whatever the
step evaluations do.

*Proof.* If $k = 0$ the growth makes one step, and $e = p$. Otherwise
$r_1 = \omega_1$ matches, so its end $e_1 \ge p$, and for $i \ge 2$ the end
$e_i$ of $r_i$ is greater than $e_{i-1}$, so $e_i \ge p + i - 1$. The result
is $r_k$ with $e = e_k \ge p + k - 1$, so $k + 1 \le e - p + 2$. Every end is
at most $n$. $\square$

This is the bound of (G5): at most two more than the length consumed. Both
bounds count the steps that the growth makes, whether $\mathcal{R}$ evaluates
them or $\mathcal{M}$ takes some from the memo.

## Lemma 3 (Termination of Evaluation)

In $\mathcal{R}$ every evaluation of an expression and of a call ends, for an
input of finite length.

*Proof.* By induction on the pair $(n - p, B - m)$ in lexicographic order,
where $p$ is the position where the evaluation starts, and $m$ is the length of
the chain, of the calls made at one position as in Lemma 1, that ends in the
evaluation. By Lemma 1, $B - m \ge 0$.

A body is a finite expression, so an evaluation of it ends if every call and
every iteration in it ends. A repetition has at most $n - p + 1$ iterations,
because it stops at the first iteration that consumes no input (G4), and an
action ends because it has no recursion (G4). A call made at a position
$p' > p$ has $n - p' < n - p$. By (A3) no call is made at $p' < p$. A call
made at $p$ is the next call of the chain, with $m + 1$. By the induction
hypothesis these calls end. A call of a leader whose growth is open for its key
is answered at once (G4). A call that opens a growth makes at most $n - p + 2$
steps (Lemma 2); each is the evaluation of the body of the call, whose calls
at $p$ extend the chain by one and whose calls at later positions are smaller
in the first component, so each step ends. $\square$

## Theorem 1 (Growth Terminates)

Every growth ends, in $\mathcal{R}$ and in $\mathcal{M}$, after at most
$e - p + 2$ steps. $\mathcal{M}$ evaluates the bodies of at most those steps.

*Proof.* For $\mathcal{R}$ this is Lemmas 2 and 3. $\mathcal{M}$ differs only
by answering a call, a step or a run of steps from what it stored, and each
such answer is a finite lookup (a hash or a scan of an entry chain) that makes
no call. Where $\mathcal{M}$ evaluates a body, the evaluation is nested in the
same calls as in $\mathcal{R}$, so the induction of Lemma 3 holds for
$\mathcal{M}$ with the same measure, taking the growths that end as given.
Each pass of the loop of a growth in $\mathcal{M}$ evaluates one step, answers
one from a memoized step, or takes a run. A step that does not end farther than
the current result stops the growth. Any other step, and by (A5) a run of
$m \ge 1$ steps, moves the current result strictly farther, and an end is at
most $n$. So the loop has at most $n - p + 2$ passes, and at most $e - p + 2$
where $e$ is the end of its result, by the argument of Lemma 2. $\square$

## Theorem 2 (Unique Result)

Let a call of a leader with key $\kappa$ start at $p$ in a state $\sigma$ and a
context $\Gamma$. There is exactly one growth $r_0, \dots, r_k$ for it, which
is a function of $\kappa$, $\sigma$, $\Gamma$ and the current results in
$\Gamma$ alone. $\mathcal{M}$ passes through the same current results, in the
same order, with the same $k$, however it chooses what to store and reuse.

*Proof.* **The growth is a function.** Let a call $c$ at $p$ be at chain
length $m$ (Lemma 1). Its context is the sequence of the open growths at $p$ of
the cycle of its rule, each with its leader's key and current result (G5);
by Memoization Lemma 2 it reads no other open growth, so a call in another
cycle gets only the growths of its own cycle. Define the outcome of $c$ by
recursion on $(n - p, B - m)$, the measure of Lemma 3. The outcome $B(r)$ of
a body evaluation is determined by $\sigma$, by the calls it makes at later
positions, which have a smaller first component and depend on their own keys
and states only (Memoization Lemma 1), and by its calls at $p$, which have
chain length $m + 1$ and so a smaller second component. A call at $p$ is one
of

1. a call of a leader whose key is open at $p$ in the cycle of the rule:
   answered with the current result of that growth, which is $r$ for $\kappa$
   and is given in the context for another key;
2. a call of a leader $L'$ whose key is not open: it starts a growth whose
   context is the growths of the cycle of $L'$ among those open at $p$,
   that is, the context of the caller with $(\kappa, r)$ added if $L'$ is in
   the cycle of $L$, and the growths of its own cycle otherwise;
3. a call of a rule that is not a leader: it evaluates its body in the context
   of the growths of its own cycle, again with $(\kappa, r)$ added if it is in
   the cycle of $L$.

In cases 2 and 3 the callee has a smaller measure, so by the recursion its
outcome is a function of its key, its state and its context. By Lemma 2 a
growth makes at most $n - p + 2$ steps, each a body evaluation of a smaller
measure, so each growth is defined. So $B(r)$ is a function of $\sigma$, the
context, $\kappa$ and $r$, and so are the sequence and $k$: (G1) computes each
$r_{i+1}$ from $r_i$ alone. Keys are distinct on a chain (Lemma 1), so a
context has at most $N$ growths.

Several leaders of one cycle at $p$ give nested growths. The growth of $L'$ in
step $i$ of the growth of $L$ reads $r_{i-1}$ and so can be a different
function of its start in each step; it is not shared between steps, which is
(G5) "grows again in each context". Its result is determined as above.

**$\mathcal{M}$ passes through the same results.** By induction on the steps
of the growth, $\mathcal{M}$ and $\mathcal{R}$ have the same current result
$r_{i-1}$ before step $i$. Suppose they do. The step is one of:

- *Evaluated.* The body evaluation in $\mathcal{M}$ has the outcome of
  $B(r_{i-1})$ by the Memoization theorem, item 5 (its nested evaluations
  satisfy (I1)).
- *Answered from a memoized step.* $\mathcal{M}$ takes it under the conditions
  of (D) of the Memoization proof, with the same present set and each growth
  that the step read open with an equal current result, and each observed
  lookahead with the same observation. By Memoization Lemma 4 the outcome is
  $B(r_{i-1})$.
- *Taken in a run.* $\mathcal{M}$ takes steps $i, \dots, i + m - 1$ at once from
  a current result equal to $r_{i-1}$, with the observations of their
  lookaheads unchanged. By the Memoization theorem, item 4, the run has the
  outcomes of the steps $B(r_{i-1}), \dots$. By (A5) each of them adopted its
  result, so $\mathcal{M}$'s current result is $r_{i+m-1}$ as in
  $\mathcal{R}$, and the step $i + m$ is next.

In each case the outcome is $\omega_i$, so the decision whether to adopt it
(a match that ends farther than $r_{i-1}$) is the same, and so is the stop. The
two parses stop after $k + 1$ steps with the result $r_k$. $\square$

## Consequences

**The result is the farthest.** The end of the result of a growth is the
greatest end among the outcomes $\omega_1, \dots, \omega_{k+1}$ that match,
because the ends of the adopted results increase and $\omega_{k+1}$ does not
end farther than $r_k$. The growth stops at the first step that does not end
farther than the current result, which is how (G1) ends it.

**Memoized members.** A rule of a cycle that is not a leader is evaluated in
the context of the growths open at $p$. By (S) and (C) of the Memoization proof
it is answered from a stored result only if that result read no growth that
was open when it started, so such an answer is the result of evaluating the
member, and by (D) a result that read a growth is reused only under an equal
current result. A member therefore never gives a growth another seed than
evaluating it would.

**Several leaders.** A leader that is called at $p$ in a context $\Gamma$ in
which another leader of the cycle grows is a new growth for each context. The
number of nested growths at one position is at most $N$ (Lemma 1), and each
ends, so a growth in a context ends too. Which rule of a cycle is the leader is
decided by the grammar (G3) and enters the proof as given; this proof does
not show that two choices of leaders give the same results.

**Cost.** A growth of a leader costs one evaluation of its body for each of at
most $e - p + 2$ steps, which is the count that
[Cost](../runtime/parsing.md#cost) uses for a growing left-recursive rule.
