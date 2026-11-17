# Linear Cost

Arata Furukawa · Last revised 2026-11-17

This proof shows that the [Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)
bound the [cost](../runtime/parsing.md#cost) of every parse whose records
are exact in the sense of its [Setting](#setting), so that the
classes Linear and Linear Given an Environment Bound of
[Classification](../runtime/parsing.md#classification) hold their promise. It
splits the cost into the parts that the specification counts and bounds each
part by one lemma, so that a new condition is added as a lemma, or as a case of
one, without changing the others.

A parse can keep records of the walks of its repetitions, and the proof
assumes that a record gives what the iterations that it answers would give,
(R1) of the [Setting](#setting). A repetition is a recorded site of a parse
when its records in that parse also bound the iterations that its walks
evaluate, (R2) and (R3) there. Lemma 17 bounds the iterations of the walks
of the recorded sites by the number of the walks, with no use of
condition 2. The count of
Lemma 4, with its links between the activations at one position, is then
needed only for the scanning sources without records, which Lemma 17 lists,
among them every repetition that is no recorded site of the parse,
and what this introduction says below of that count and of its open parts
concerns those sources alone. Lemma 17 is proved from (R2) and (R3)
and, for the places inside an Operators Rule, step 3 of
[Open Steps](#open-steps); its bound is in terms of the body evaluations of
the memoized rules, which it reads from Lemmas 3 and 5, and of the
successful activations of the sources without records.

The theorem covers every parse whose records are exact, whatever its
recorded sites are; which repetitions are recorded sites decides only which
lemma counts them. As the [Setting](#setting) states under *Sites that meet
the conditions*, (R2) and (R3) are shown for the run sites free of the
environment in the parses of the three backends of the engine. They are not
shown for a run site in a generated Go or TypeScript parser, for a run site
that is not free of the environment, or for a repetition of a single
character outside the reused skips and the `#stream` repetitions, in any
backend. A parse that walks one of these is covered with that repetition
among the sources without records, and so with the open parts of the count
of Lemma 4 for it.

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
growth step, which condition 2 checks as a unit at the end of its seed, at most one successful activation of a source at a position is
direct in one body evaluation, and every other follows a call that a
committing unit shares with its continuation, which, in the reference parse of
the memoization proof, is evaluated again and repeats the first activation at
the position in it; the memo bounds these for a
memoized rule and the cycles of calls that nest at one end bound for a rule that
calls no rule when the call is the first that the two share, and it shows,
apart from four open points, that evaluations of a call that nest without a
bound and end at one position make the call nest at one end. It proves the sum
over the positions for the activations owned by memoized evaluations of the
sources of rules that call a rule or are memoized at their calls, by charging
the activations of each later evaluation of a rule or its twin at a position to
the first, and by comparing the two evaluations of each memoized link in the
reference parse of the memoization proof, in either lookahead state, either
variant and through the growths that the later one makes again, apart from the
activations that a path that enters the growth again, or a dropped step in a
part at the position of its growth, repeats, which Lemma 5 counts before it, and with no count of the
evaluations in a context of growths. It leaves the sum open for the sources of rules that call no rule and are not
memoized, and, in a Document parse or through results kept in a context of
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
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules), whose
memo stores the result of every evaluation of a memoized rule, or of its
twin, at a position outside every context of growths except possibly the
first of the rule and the first of its twin, as [Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost) requires.
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
$E + 1$. A store of a transient rule, whose results only a Document stores, looks at the first 32
entries of the chain only, and when the key is not among them it adds an entry
in front, so a chain of more than 32 entries can hold more entries under such a
key. Only one entry under a key matters in a parse: a lookup returns the first
entry of the chain under its key, through the chain or the index alike, and a
store puts in that entry's place the join of it and the new result that
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules) describes,
or the new result where the parse cannot use the entry, or puts the new entry in front of it, so
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

**Records of walks.** A **walk** is an evaluation of a repetition without an
upper bound, and the iterations that it evaluates are its activations. A
**candidate site** is a repetition without an upper bound, outside the reused
skips, whose walks Lemma 9 counts, that is a
[run site](../runtime/parsing.md#run-sites), or that repeats a single
character, which a run site does not, and is no `#stream` repetition, in
whatever rule it stands. The **element** of a site is
the expression that it repeats. A candidate site is **free of the environment**
as [Run Sites](../runtime/parsing.md#run-sites) defines it; a repetition of a
single character is, since its element reads no variable.

An implementation can keep **records** of the walks of a candidate site, and
a walk can take a record in place of the iterations that it would evaluate
from a position on. So can it of the walks of a
[skip-run site](../runtime/parsing.md#skip-runs), in a reused skip or in a
skip that is not reused, which is a candidate site only when it repeats a
single character in a skip that is not reused. A candidate site or a
skip-run site **keeps records** in a parse when one
of its walks in that parse takes a record. A walk of a candidate site is
**unresumed** unless a
[Document](../runtime/documents.md) resumes a run at it: a repetition that is
resumed neither looks up nor makes a record
([Scope](run-sites.md#scope) of the Run Sites proof,
[Scan Sites](../runtime/parsing.md#scan-sites)).

The proof assumes of every parse that its records are **exact**:

- (R1) a walk that takes a record leaves what the walk that evaluated its
  remaining iterations would leave: the same end, outcome, examined range,
  values, recovered errors and environment, so that the parse gives what it
  gives without records.

(R1) is shown of every record that an implementation keeps. Every
implementation keeps records of skip-run sites, and
[Lemma 1](skip-runs.md#lemma-1-exactness) of the Skip Runs proof shows (R1)
of them. The three
backends of the engine keep records of run sites, free of the environment
or not, in the parses of the [Scope](run-sites.md#scope) of the Run Sites
proof, every kind of parse that they run, of a loaded program, of a stream
or of a Document, and [Lemma 1](run-sites.md#lemma-1-exactness) of that
proof shows (R1) of them. The engine and the generated Go and TypeScript
parsers keep records of [scan sites](../runtime/parsing.md#scan-sites), the
repetitions of a single character in a rule that calls no rule, and
[Lemma 4](run-sites.md#lemma-4-exactness-of-scan-records) of that proof
shows (R1) of them. A parse with missing-token insertion keeps no records
and is outside this proof as it is outside the classes.

Fix a constant $c_R \ge 1$ that depends only on the grammar and the backend.
A candidate site $\sigma$ is a **recorded site** of a parse when its
unresumed walks in that parse meet the two conditions below, and the
unresumed walks of a recorded site are its **recorded walks**:

- (R2) the unresumed walks of $\sigma$ evaluate the element of $\sigma$, in
  the whole parse and in both variants of its rule, at most
  $c_R (K_\sigma \hat{n} + W_\sigma)$ times, where $W_\sigma$ is the number of
  unresumed walks of $\sigma$, $K_\sigma = 1$ when $\sigma$ is free of the
  environment, and otherwise $K_\sigma$ is the largest number of distinct
  combinations of the values of the variables that the element of $\sigma$
  reads, in its predicates and actions and in those of the rules that it
  calls, directly or indirectly, among the visits that the unresumed walks of
  $\sigma$ make to one position;
- (R3) outside the iterations that it evaluates, an unresumed walk of
  $\sigma$ makes at
  most $c_R$ steps, and at most $c_R$ more for each iteration that it
  evaluates, apart from its **suffix copies**, the elements of a record that
  a walk whose iterations build values copies behind its own when it takes
  the record, and apart from the recovered errors that it takes over with a
  record, whose work $C$ does not count (Lemma 14).

So the recorded sites are a property of the parse, and the proof covers a
parse whatever they are. A candidate site that is no recorded site of the
parse is, in that parse, one of the *Sources without records* of Lemma 17,
whether it keeps records or not: Lemma 17 does not count its iterations, and
the count of Lemma 4 is needed for them. A greater $c_R$ makes more sites
recorded and enters the constants of Lemma 17.

(R2) and (R3) are a requirement on the implementation, as the store of the
memo is, for the sites that Lemma 17 is to count: (R2) asks, for a site that
is not free of the environment, for
records under the values that its element reads, and, for a repetition of a
single character, for records that answer a walk from every position that an
earlier walk passed, in whatever rule the repetition stands. Let $K_R$ be
the largest $K_\sigma$ of the recorded sites of the parse, and 1 when there
is none. When
every variable that the element of a recorded site reads has finite values,
$K_R$ is at most the product of the sizes of their value sets, the constant
of Lemma 8.

**Sites that meet the conditions.** Whether a candidate site is a recorded
site of a parse depends on the records that the implementation keeps:

- *Run sites free of the environment, in the engine.* In a parse of the
  [Scope](run-sites.md#scope) of the Run Sites proof that inserts no missing
  token, the engine keeps the records of a run site free of the environment
  by position alone; in a parse of a loaded program it does so only when the
  instruction of the site marks it free. For such a site that proof shows
  (R2) and the lookups and stores of (R3) in
  [Lemmas 2 and 3](run-sites.md#lemma-2-cost-once-hot). So, with $c_R$ at
  least the constants of those lemmas, such a site is a recorded site of
  every such parse.
- *Run sites that are not free, in the engine.* The engine keeps their
  records under the environment as an object, and so it keeps those of a
  site that the instruction of a loaded program does not mark free. That
  gives no bound $K_\sigma$: two environments that are equal and
  distinct give distinct keys
  ([Lemma 2](run-sites.md#lemma-2-cost-once-hot) of that proof). (R2) is not
  shown for them.
- *Repetitions of a single character.* In a rule that calls no rule, such a
  repetition is a [scan site](../runtime/parsing.md#scan-sites), and the
  engine and the generated Go and TypeScript parsers keep one record for a
  walk of it, at the start of the walk. That does not give (R2): a walk that
  starts at another position of the
  same run finds no record and evaluates its own length
  ([Scan Sites](../runtime/parsing.md#scan-sites)). In a rule that calls a
  rule, a repetition of a single character keeps no record in any
  implementation. (R2) is not shown for a repetition of a single character
  in either case.
- *Generated parsers.* The generated Go and TypeScript parsers keep no
  record of a run site, whose repetition they evaluate as an ordinary one
  ([Scope](run-sites.md#scope) of the Run Sites proof). (R2) is not shown
  for a run site in them.

So a candidate site is shown to be a recorded site of a parse only when it
is a run site free of the environment, which the instruction marks free
where the program is loaded, and the parse is one of a backend of the engine
in that Scope that inserts no missing token. No other candidate site is
shown to be a recorded site of any parse: a run site in a generated Go or
TypeScript parser, a run site that is not free of the environment or that a
loaded program does not mark free, and a repetition of a single character
without an upper bound outside the reused skips and the `#stream`
repetitions, in the engine or in a generated parser. A parse that walks
such a site is covered, with the site among the sources without records:
its successful iterations are in $A_u$ of Lemma 17, and their bound rests on
the count of Lemma 4 and on the parts of it that step 1 of
[Open Steps](#open-steps) leaves open. Where such a site keeps records, as a
run site that is not free does in the engine and a scan site in every
implementation, the bound rests on the seventh part of step 8 as well.

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
evaluation is $\lambda$ of the [memoization proof](memoization.md#setting): $1$ inside a lookahead or
the `skip` of an Operators Rule, and $0$ elsewhere. Where this proof speaks of
the lookahead state, or of memo entries stored, or calls answered, inside or
outside a lookahead, the `skip` of an Operators Rule counts as a lookahead.
Everywhere else a lookahead is a lookahead expression, after which the parser
returns to the position where it started.

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
each of these from one entry, the first under its key ([Setting](#setting)).
An entry stored outside a
lookahead answers the calls in both lookahead states, and one stored inside a
lookahead answers only those inside one, condition (L) of the memoization
proof; a call outside every context of growths does not look at the leaders
that the entry records. A store keeps the join of the entry and the new
result ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)), which answers every call
that either answered, so the entry never narrows, whether the store is made
in a context of growths or not. An entry that the parse cannot use, as one
of an earlier parse that lags behind too many edits, answers no call, and
the store that replaces it narrows nothing. A call that the claim counts is
evaluated only when no entry answers it: when its key has no entry, or, for
a call outside lookaheads, only an entry computed inside one. Neither holds
again once an evaluation in the same state has stored. So in each state at most one
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
a leader in every parse. A [Document](../runtime/documents.md#reuse-model)
also stores results of the transient rules, the rules that the first
exception above leaves out, but this lemma and the ones after it do not take
those rules as memoized: a lookup of such a rule walks only the first 32
entries at its position and misses the others, so with more than 32 keys at
one position every call of the rule there can miss. Such a call is then
evaluated as in a parse from scratch, which memoizes none of them, and
[Lemma 10](#lemma-10-rules-that-are-not-memoized) counts these evaluations by
the calls, as for every rule that is not memoized; where the Document's memo
answers such a call, it saves an evaluation that the count already holds. A
parse with missing-token insertion, which
is outside the classes, leaves unmemoized every rule other than a leader that
contains `#missing`, directly or through the rules it calls. A first evaluation
may leave no result, a result computed inside a lookahead or a `skip` answers
only the calls inside them, and one computed outside them answers every call,
so at most one evaluation in each of the two states stores a result, which
gives the three evaluations.

**Transient calls of a Document.** The claims of the lemmas below on
activations, owners and own regions read a Document parse as if it evaluated
every call of a transient rule: where the Document answers such a call from
its memo, they take in its place the evaluation of its counterpart in the
reference parse $\mathcal{R}$ (*Counterparts in the reference parse* in
Lemma 4), with the activations in it and with the calls of memoized rules in
it taken as answered. A parse from scratch evaluates every such call, and the
answer gives the outcome of that evaluation ([memoization
theorem](memoization.md#theorem-memoization-never-changes-the-result)), so
the parse goes on after it as after the evaluation, and the activations that
the Document makes are among those that this reading counts: it adds the
activations inside the answered calls and drops none. A call of a memoized
rule inside an answered call is not made, and a later call of its key that
the Document then evaluates is an evaluation of the key that Lemma 3 counts
as any other. So a sentence below that says that the memo never answers a
call of a rule that the parse does not memoize, or that the parse evaluates
such a call, states this reading; a bound on the activations that it counts
bounds those that the Document makes.

## Lemma 17 (Recorded Walks)

Let the recorded sites of the parse and their recorded walks be those of the
[Setting](#setting). Let
$A_r$ be the number of activations that are iterations of recorded walks,
$A_u$ the number of the other iterations of repetitions without an upper
bound, of the operators that Operators Rules apply and of the evaluations of
the skip expressions of `#recover` that are not reused, that succeed, that
is, that consume input (*Restarts* in Lemma 4), $B$ the number of body evaluations
of the rules that the parse memoizes, growth steps and evaluations in a
context of growths included, and $O$ the number of evaluations of their parts
that Operators Rules make outside their activations, the tries of step 3 of
[Open Steps](#open-steps). Write
$\Phi = K_R \hat{n} + B + A_u + O + 1$. Then, for a constant $c_{17}$:

1. $A_r \le c_{17} \Phi$, and the recorded walks number at most
   $c_{17} \Phi$;
2. the steps that the recorded walks make outside the iterations that they
   evaluate, apart from their suffix copies, number at most $c_{17} \Phi$.

The lemma uses none of conditions 1 to 5, and of the records it uses (R2)
and (R3) for the recorded sites alone. $A_u$ counts the successful
iterations of every candidate site that is no recorded site of the parse,
whether the site keeps records or not. $A_u$ leaves out the growths and
the copying operations, as *Activations that do not succeed* in Lemma 4
does: they enclose no place in the sense below.

*Proof.* A **place** is an occurrence of an expression in the body of a rule
$Q$, outside its reused skips, which is also an occurrence in the body of the
twin of $Q$. A reused skip calls no rule (Lemma 1), so every call is a place,
and every recorded site is one by its definition. For a place
$x$, let $N(x)$ be the number of evaluations of $x$ in the parse, in both
variants, and let $E(Q)$ be the number of body evaluations of $Q$ and of its
twin. A scanning source $\tau$ of the body of $Q$ **encloses** $x$ when $x$
lies in the part of the grammar that an activation of $\tau$ evaluates, and
is not $\tau$: in the element of $\tau$ when $\tau$ is a repetition without
an upper bound, in $\tau$ when $\tau$ is an Operators Rule, in the skip
expression when $\tau$ is a `#recover` whose skip is not reused. Let
$\tau(x)$ be the innermost scanning source that encloses $x$, when there is
one. For a scanning source $\tau$, let $A(\tau)$ be the number of its
activations in both variants, and $O(\tau)$ the number of evaluations of its
parts that it makes outside its activations when $\tau$ is an Operators
Rule, and 0 otherwise.

*One place.* There is a constant $c_1 \ge 1$ such that
$N(x) \le c_1 E(Q)$ when no scanning source encloses $x$, and
$N(x) \le c_1 (A(\tau(x)) + O(\tau(x)))$ otherwise. This is the step *One
place* of Lemma 10, whose argument holds of every place: outside the scanning
sources of the body, $x$ is evaluated at most as many times in one body
evaluation as the product of the upper bounds of the bounded repetitions that
enclose it; inside them, at most that many times in one activation of
$\tau(x)$ outside the activations of $\tau(x)$ nested in it, or in one
evaluation of a part of an Operators Rule outside its activations. As in
Lemmas 2 and 10, for a place inside an Operators Rule the bound rests on
step 3 of [Open Steps](#open-steps). An iteration that a Document reuses from a run is
an activation that evaluates nothing, and an iteration that a record answers
is neither evaluated nor an activation, so neither adds an evaluation of
$x$.

*Evaluations by call.* For a rule $Q$ that the parse does not memoize,
$E(Q) \le [Q = S] + \sum_s N(s)$, where $S$ is the start rule where the parse
begins and $s$ ranges over the places that call $Q$. This is the step
*Evaluations by call* of Lemma 10.

*Heights.* Let $\mathcal{U}$ be the set of the rules that are evaluated in
the parse and that the parse does not memoize. Consider a sequence
$Q_0, Q_1, \ldots, Q_m$ of rules of $\mathcal{U}$ in which the body of each
$Q_{i+1}$ holds a place that calls $Q_i$. For $i \ge 1$, $Q_i$ calls a rule,
so by the first exception of
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules) it is
called from at most one place, which is the place in the body of $Q_{i+1}$
when $i < m$. If $Q_i = Q_j$ for some $1 \le i < j$, the rules
$Q_i, \ldots, Q_{j-1}$ form a cycle of calls on which every rule is
evaluated, is not memoized and is called from one place, which is in the body
of a rule of the cycle; the step *Rules called from one place* of Lemma 10
shows that there is no such cycle. So $Q_1, \ldots, Q_m$ are distinct and
$m \le |\text{rules}|$. Let the **height** $h(Q)$ of a rule of $\mathcal{U}$
be the greatest $m$ of such a sequence with $Q_0 = Q$, and let $h(Q) = -1$
for a rule that the parse memoizes. If a place in the body of a rule $Q'$
that is evaluated calls a rule $Q$ of $\mathcal{U}$, then $h(Q') < h(Q)$: a
sequence from $Q'$ extends to one from $Q$.

*Induction.* There are constants $c_Q$, $c_x$ and $c_\sigma$, for the rules,
the places and the recorded sites, such that $E(Q) \le c_Q \Phi$ for every
rule $Q$, $N(x) \le c_x \Phi$ for every place $x$, and the recorded walks of
every recorded site $\sigma$ evaluate at most $c_\sigma \Phi$ iterations. The
proof is by induction on the height of the rule, and, for the places and
sites in the body of one rule, on the number of scanning sources that enclose
them.

Let $Q$ be a rule. If $Q$ is not evaluated, $E(Q) = 0$ and $N(x) = 0$ for
every place of its body. If the parse memoizes $Q$, $E(Q) \le B \le \Phi$. If
$Q$ is in $\mathcal{U}$, then $E(Q) \le 1 + \sum_s N(s)$, and each place $s$
of the sum lies in the body of a rule that is not evaluated, so that
$N(s) = 0$, or of a rule of smaller height, for whose places the bound holds
by the induction hypothesis; the sum has at most as many terms as the grammar
has calls. This gives $c_Q$.

Let $x$ be a place of the body of $Q$, and suppose the bounds for the places
and the recorded sites of the body that fewer scanning sources enclose. If
no scanning source encloses $x$, $N(x) \le c_1 E(Q) \le c_1 c_Q \Phi$.
Otherwise let $\tau = \tau(x)$. Every scanning source that encloses $\tau$
encloses $x$, and $\tau$ encloses $x$ and not itself, so fewer scanning
sources enclose $\tau$ than enclose $x$, and the hypothesis gives
$N(\tau) \le c' \Phi$ for the place $\tau$, with a constant $c'$.
$O(\tau) \le O$. Each evaluation of a scanning source has at most one
activation that does not succeed, its last (*Activations that do not
succeed* in Lemma 4), so the activations of $\tau$ that do not succeed
number at most $N(\tau)$. If $\tau$ is no recorded site, its activations
that succeed are counted in $A_u$, so
$N(x) \le c_1 (A_u + N(\tau) + O) \le c_1 (1 + c') \Phi$. If $\tau$ is a
recorded site, its activations are the iterations of its recorded walks, at
most $c_\tau \Phi$ by the hypothesis for the site $\tau$, and the
iterations of the walks at which a Document resumes a run, of which those
that succeed are counted in $A_u$ and the others number at most $N(\tau)$;
so $N(x) \le c_1 (c_\tau \Phi + A_u + N(\tau)) \le c_1 (c_\tau + 1 + c') \Phi$.
This gives $c_x$. Now let $x$ be a recorded site $\sigma$. Its recorded walks are
evaluations of the place $\sigma$, so $W_\sigma \le N(\sigma) \le c_x \Phi$
by the bound just shown, and by (R2) they evaluate at most
$c_R (K_\sigma \hat{n} + W_\sigma) \le c_R (1 + c_x) \Phi$ iterations, since
$K_\sigma \le K_R$. This gives $c_\sigma$.

The constants depend only on the grammar and the backend: the heights are at
most $|\text{rules}|$, the number of scanning sources that enclose a place is
at most the size of its body, and the parse enters the constants only through
which rules are evaluated and memoized and which candidate sites are
recorded, of which there are finitely many
cases. Summing the bound for the sites over the recorded sites gives the bound
on $A_r$, and summing $W_\sigma \le N(\sigma)$ gives the bound on the walks,
which is 1. By (R3) the steps of 2 are at most $c_R$ for each recorded walk
and $c_R$ for each iteration that one evaluates, which with 1 gives 2.
$\square$

The lemma takes the place, for the recorded walks, of the count of Lemma 4:
it bounds their iterations by counting walks, with no use of condition 2, of
the units and their continuations, or of the links between activations at
one position.

**Order.** The lemma bounds $A_r$ by $B$, $A_u$ and $O$. It gives a bound in
terms of the input only when those three are bounded with no use of $A_r$
or of what follows from $A_r$: the activations that do not succeed and the
body evaluations of the rules that are not memoized, which *Activations that
do not succeed* in Lemma 4 derives from $B$ and from all the successful
activations, $A_u + A_r$ here. $A_u$ counts only activations that succeed so
that this order is possible: first $B$, $A_u$ and $O$, then $A_r$, then the
activations that do not succeed and the evaluations of the rules that are
not memoized. The theorem takes the order as a hypothesis, of which this
much is shown:

- $B$ is bounded by Lemmas 3 and 5. The next claim reads the bounds that
  they state and finds none that takes such a count.
- $A_u$ is bounded by Lemma 13 for the bounded scans, whose proof uses
  Lemma 3, Lemma 15 and the definitions, and by the count of Lemma 4 for
  the others. What Lemma 4 proves of that count, *Activations held by heads*,
  bounds the activations at each position by the links and by the six
  memoized evaluations of a class, with no count of the evaluations in a
  context of growths or of the rules that are not memoized (the remark after
  *Replayed activations*). The parts of the count that step 1 of
  [Open Steps](#open-steps) leaves open have no proof, so nothing is shown
  of what their proof uses: step 8 requires that it keep this order.
- $O$ is step 3 of [Open Steps](#open-steps), which is open, with the same
  requirement.

**The bound on the memoized evaluations counts no iterations.** The bounds that Lemmas 3 and 5
state on the body evaluations of the memoized rules take as their inputs the
number of positions, the number of keys at a position, the number of steps
of a growth, the length that a growth consumes and the number of successful
growth activations of a call at a position. None takes the number of
iterations of a repetition, of operators applied or of evaluations of a skip
expression, the number of activations that do not succeed, or the number of
body evaluations of the rules that are not memoized.

*Proof.* By reading what each claim of Lemma 5 counts and from what. $B$ is
the sum of four numbers.

1. The evaluations of Lemma 3, which that lemma bounds by the keys.
2. The steps of the outer growths (*Families of growths*). By part 1 of
   *Growths at many positions* an outer growth takes at most two steps that
   are no growth activations, a key has at most two outer growths, and the
   keys at a position are at most a constant times $K$, so these steps are
   at most a constant times $K\hat{n}$ plus the successful growth
   activations of the outer growths.
3. The steps of the growths that a step of a growth of another leader starts
   at its own position. *Growths inside a step*, with the paragraph after
   it and *Reuse in a context*, bounds them for each step of the enclosing
   growth by a constant, by the length by which the result passes the seed,
   or by the length of one of a constant number of results, and otherwise
   counts the growth as an activation of the call that starts it, a scanning
   source; that makes at most a constant times $\ell + 1$ of them in a
   growth that consumes $\ell$ characters, apart from the growths at those
   calls.
4. The body evaluations in a context of growths. By *Calls in a context of
   growths* a step makes at most $c_5$ of them outside the steps of the
   growths that it starts, a number that the proof takes from the calls of
   the bodies at the position of the growth alone.

The successful growth activations of a call at a position $q$ are counted
at $q$. Those whose growth starts after $q - c_W$ are at most a constant
times $K$, by the keys. Those that lie inside the one before them are
bounded by part 3 of *Growths at many positions* through the number of
outer growths. Those of the second outer growth of a key are matched to
those of the first (*Second outer growths*). *Restarts of growth
activations* and *Direct growth activations* give the others the restarts,
*Direct activations* and *Repeats* of Lemma 4, which count nothing, and so
a sequence of them, at most two direct in one body evaluation, joined by
links; the rule of a link holds the call that starts the growth, so every
link is memoized and no leaf link arises. Of that sequence, the growth
activations that memoized evaluations own are counted by *Activations held
by heads*, from the six memoized evaluations of a class, *Replayed
activations* and *Links to heads*, and the others by the claims from
*Families of growths* to *Answered places*, which charge them to the heads
of the keys of the outer growths and bound the repeats inside one family at
a position by a constant. The claims of Lemma 5 before *Calls in a context
of growths* bound what a step examines at the position of its growth by a
constant amount of the input apart from a constant number of steps of each
growth, of each unit and of each key of a memoized rule there, and state no
count of evaluations. So every count above is of positions, of keys, of
the steps of a growth, of the length that a growth consumes or of the
growth activations at one position, and none is of the iterations of a
repetition, of operators or skip expressions, of activations that do not
succeed or of evaluations of rules that are not memoized. $\square$

The claim is about the bounds as Lemma 5 states them. It does not prove
them: the sums that steps 1 and 4 of [Open Steps](#open-steps) leave open
stay open, among them the sum over the growths of the lengths that they
consume in item 3, and a proof of them must keep to the inputs above. The
activations that Lemma 5 counts beside $B$, those that growth steps and
evaluations in a context of growths own and those that dropped steps and
paths that enter a growth again repeat, are not inputs of the bound on $B$,
except the growth activations; where they are iterations of recorded walks,
this lemma counts them again, from $B$.

**Growths lie outside the recorded walks.** No call of a rule of a
left-recursion cycle is made inside an iteration of a walk of a candidate
site, recorded or not, or of a skip-run site, at any depth of calls, in the parse or in the reference parse
$\mathcal{R}$ of the [memoization proof](memoization.md#setting). So no
growth step, no evaluation in a context of growths, no call that starts a
growth and no growth activation lies inside such an iteration, and neither
does an evaluation that contains one of these.

*Proof.* A candidate site is a run site or repeats a single character. The
element of a run site calls, directly or indirectly, only rules that are in
no left-recursion cycle ([Run Sites](../runtime/parsing.md#run-sites)), and
a single character calls no rule. Neither the element of a skip-run site nor
a rule that it calls, directly or indirectly, is a rule of a left-recursion
cycle ([Skip Runs](../runtime/parsing.md#skip-runs)). An evaluation inside an iteration, at any
depth, is of a part of the element or of the body of a rule that the element
calls, directly or indirectly, so every call inside the iteration is of a
rule in no cycle. A growth step is a body evaluation of a leader, an
evaluation in a context of growths is a body evaluation of a rule of a cycle
(Lemma 3), and a growth starts at a call of a leader; each is, or is made
by, a call of a rule of a cycle. An evaluation inside an iteration contains
only evaluations inside that iteration. $\square$

So a record answers no iteration that holds one of the evaluations that the
bound on $B$ counts beyond those of Lemma 3, and each evaluation that the
claims of Lemma 4 that count nothing take for a growth activation, the body
evaluation in which it is direct, the evaluations that hold or own it and
the evaluations of the shared call of a link that contain it, lies outside
the iterations of the walks of candidate sites and of skip-run sites. The records reach those
claims only through what a walk leaves, which (R1) fixes; *Counterparts in
the reference parse*, *Replays* and *Replayed activations* in Lemma 4 are
stated for a parse that keeps records on that ground, *Restarts in a
parse that keeps records* there shows the claims from *Failure after a
source* to *Memoized links* of such a parse, and *Heads in a parse that
keeps records* those from *Answered calls* to *Evaluated pairs of a shared
call*, with one more case of the covered keys, and *Leaf links in a parse
that keeps records* the first four claims on leaf links, and *Later leaf
links in a parse that keeps records* the claims on leaf links from *Memoized links to the frame of an outer
link* to *Nesting at one end*, as that claim states them: some with
case (d) of the covered keys, one with one more exception, some for
the activations outside the iterations of the walks of the sites that
keep records, and not *Calls of scan-only rules* or *Seed reads*.
*Activations that do not succeed in a parse that keeps records* there
shows that claim and Lemma 10 of such a parse, and *Iterations at one
scan in a parse that keeps records* shows *Iterations at one scan* in
part, under a hypothesis on the shared calls of a link. The
other claims that count nothing, those that follow *Iterations at one
scan*, and the claims of Lemma 5 other than the first case of *Repeats
inside a growth* and those that *Families in a parse that keeps records*
shows for a parse that resumes no run of a Document, have not been read again for such
a parse (step 8 of [Open Steps](#open-steps)); *Families without the
records* bounds the activations that growth steps and evaluations in a
context of growths own in such a parse, when it resumes no run of a
Document, by those of a parse that keeps no records, for which no such
reading is needed, subject to the open parts of the count of Lemma 5 in
that parse, and *Families without the records in a parse that resumes
runs* bounds them, when it resumes runs, by a constant times those of a
parse that keeps no record, resumes no run and stores the results that
read no growing result wherever the memo can answer them, as the engine
stores them.

**Sources without records.** $A_u$ counts the successful activations of the
following, and the count of Lemma 4 is needed for them and for the growths
alone:

- a candidate site that is no recorded site of the parse; *Sites that meet
  the conditions* in the [Setting](#setting) names those that are not shown
  to be recorded in any parse;
- a repetition without an upper bound that is no candidate site: a `#stream`
  repetition of a single character, or a repetition of more than a single
  character that fails a condition of
  [Run Sites](../runtime/parsing.md#run-sites). Such a repetition has a
  `#cut` that applies to it from its element; or its element holds an
  Operators Rule, `#stream`, `#missing`, `#error` or `#diagnostic`, or a
  repetition without an upper bound that is neither a run site nor a
  repetition of a single character; or its element calls, directly or
  indirectly, a rule that is called with a level, that is in a
  left-recursion cycle or that holds one of those constructs; or it is not
  restartable, which the claim below treats in part;
- the walks of a candidate site at which a Document resumes a run: such a walk
  pays one unit for each iteration that it reuses and evaluates the
  iterations after them as a repetition without records does
  ([Work of a Document Parse](../runtime/documents.md#work-of-a-document-parse));
- the operators that an Operators Rule applies, and the evaluations of the
  skip expression of a `#recover` that is not reused.

Two more scanning sources have no records and are not in $A_u$: the growths
of the cycles at the calls that are scanning sources, which Lemma 5 counts,
and the units of the copying operations that are scanning sources, which
Lemma 6 counts.

A source of this list can stand inside the element of a site that
keeps records, or
in a rule that the element calls: a `#recover` whose skip is not reused,
which the element of a run site may hold; a walk that a Document
resumes; a candidate site that is no recorded site of the parse, in the
element or in a rule that it calls; and, in a rule that the element calls,
a repetition that is no candidate site, as one with a `#cut` that applies
to it from its element, and the operators that an Operators Rule applies.
The conditions of
[Run Sites](../runtime/parsing.md#run-sites) ask that every repetition
without an upper bound in the element be a run site or repeat a single
character; of a rule that the element calls they ask only that it be called
without a level, be in no left-recursion cycle and hold no `#stream`,
`#missing`, `#error` or `#diagnostic`, and those of
[Skip Runs](../runtime/parsing.md#skip-runs) ask of such a rule only that
it read no variable, hold no `#recover` and be in no cycle. An Operators
Rule is written only as the whole body of a rule
([Operators Rule](../language/operators-rule.md)), so no element holds one
itself, and the condition that the element contain none excludes nothing; a
rule whose body is an Operators Rule, called without a level and in no
left-recursion cycle, is one that the element of a run site or of a
skip-run site can call, directly or indirectly. Its operators are then
applied inside the iterations of the walks of that site, and so are the
tries of its parts that step 3 of [Open Steps](#open-steps) leaves to
count, the $O$ of this lemma. The copying operations that are scanning
sources, which are not in $A_u$, can stand there as well: the element of a
run site may hold actions and predicates, and so may a rule that it calls;
Lemma 6 counts their units through the count of Lemma 4 for the
evaluations that perform them. For the
activations of such a source inside the iterations of the walks of that
site, and for those units, the count of Lemma 4 is not shown. That count compares the own regions
of the evaluations of one
class (*Replays* in Lemma 4). The slots of a site are kept separately inside
and outside lookaheads ([Skip Runs](../runtime/parsing.md#skip-runs)), so
of two such evaluations in different lookahead states the earlier can take a
record where the later evaluates the iterations that the record answers,
and the later then holds activations to which nothing in the earlier
corresponds. *Descent in the reference parse* in Lemma 4 counts instead
those that memoized evaluations own, inside the iterations as outside,
through their counterparts in the reference parse, whatever the records
answer, and *Families without the records* in Lemma 5 those that growth
steps and evaluations in a context of growths own, in a parse that
resumes no run of a Document, and *Families without the records in a
parse that resumes runs* those of a parse that resumes runs, up to a
constant factor, against a parse that resumes no run and stores as that
claim assumes; both leave the parts that step 8 states. The same holds of the iterations themselves of a candidate site
that keeps records and is no recorded site of the parse, which are
activations of a source of this list: the count of Lemma 4 is not shown for
them. Step 8 of [Open Steps](#open-steps) lists both, as its sixth
and seventh parts.

Beside these, the suffix copies of the walks that take records and whose iterations build
values are in no activation, and (R3) leaves them out. A walk that takes a
record copies at most as many elements as the remaining iterations matched,
which by (R1) are the iterations that the walk would evaluate without the
record. So the suffix copies of a site are at most the iterations that the
walks which take its records would evaluate in their place, and no claim of
this proof counts those. They are not shown to be iterations of a parse
that keeps no record of the site, since that parse is not shown to make the
same walks. A call inside an iteration that a record answers is not made.
The walk that made the record made that call in its own iteration at the
same position, and when its evaluation was the first of its rule at the
position, the memo need not have stored its result
([Setting](#setting)). The parse without the record then evaluates the call
again in the iteration and stores the result, and answers a later call with
that key; the parse with the record evaluates the later call instead, at
another place, with the walks that its body makes. So the parse with records
is not shown to evaluate a subset of what the parse without them evaluates,
nor the reverse. Each of the two evaluates only what the reference parse
$\mathcal{R}$ evaluates (*Counterparts in the reference parse* in Lemma 4).

**Sites that are walked a constant number of times.** Let $\sigma$ be a
repetition without an upper bound in the body of a rule $Q$, and let
$\mathcal{A}$ be the set of the rules that the start rule reaches and from
which $Q$ can be reached through calls, $Q$ included. Call a place **single**
when it lies in no element of a repetition whose maximum is neither 0 nor 1,
in no Operators Rule and in no skip expression. Let no rule of $\mathcal{A}$
call itself, directly or indirectly, let $\sigma$ be single, and let every
place in the body of a rule of $\mathcal{A}$ that calls a rule of
$\mathcal{A}$ be single. Let every expression other than a repetition, an
Operators Rule and a `#recover` evaluate each of its parts at most once in
one evaluation, and a `#recover` its expression outside the skip. Then the
parse evaluates $\sigma$ at most a constant number of times, and its
iterations number at most a constant times $\hat{n}$.

*Proof.* No rule of $\mathcal{A}$ calls itself, so the calls among them form
no cycle and none of them is on a left-recursion cycle. Each call of such a
rule evaluates its body at most once, whether or not the memo answers some
calls, and its body is evaluated otherwise only where the parse begins. So
$E(P) \le [P = S] + \sum_s N(s)$ for $P$ in $\mathcal{A}$, over the places
$s$ that call $P$. Only the places in the bodies of rules that the start rule
reaches are evaluated, and such a rule reaches $Q$ through $P$, so it is in
$\mathcal{A}$ and $s$ is single. A single place lies in no skip expression
and in no Operators Rule, and every repetition whose element holds it has
the maximum 0, which evaluates nothing, or 1, which evaluates its element
at most once. So, by the hypothesis on the other expressions, it is
evaluated at most once in one evaluation of its body: $N(s) \le E(P')$ for
the rule $P'$ of its body. By induction along the calls among the rules of
$\mathcal{A}$, which form no cycle, $E(P)$ is at most a constant for every
$P$ in $\mathcal{A}$, and then $N(\sigma) \le E(Q)$ is too. Each iteration
of a walk that matches consumes input, and the walk ends at the first that
fails or consumes none
([Lemma 3](run-sites.md#lemma-3-cost-before-hot) of the Run Sites proof), so
a walk has at most $\hat{n}$ iterations. $\square$

A repetition that is not restartable in the sense of
[Run Sites](../runtime/parsing.md#run-sites) lies in no element of a
repetition whose maximum is neither 0 nor 1 and in no rule that such an
element calls, directly or indirectly, and its rule does not call itself.
So neither it nor a place that calls a rule of its $\mathcal{A}$ from the
body of one lies in such an element. When, besides, none of these places
lies in an Operators Rule or a skip expression and no rule that reaches its
rule calls itself, the claim bounds its iterations with none of conditions 1
to 5. Otherwise
it is a source without records in the list above.

## Lemma 4 (Scanned Cost)

*Outline.* Under condition 2, the activations of the scanning sources of all
body evaluations, other than the iterations of the recorded walks, which
Lemma 17 counts, and the bounded scans that Lemma 13 counts, number at
most a constant times $K\hat{n}$, and the scanned cost of all body evaluations,
apart from the copying operations, which Lemma 6 bounds, from the suffix
copies of the walks that take records, and from the steps that the walks of
a candidate site that keeps records and is no recorded site of the parse
make outside their iterations (step 8 of [Open Steps](#open-steps)), is at
most a constant times the number of
activations, those of recorded walks and of bounded scans included, plus the
number of recorded walks.

The claims of this lemma are stated for every scanning source and hold of
the recorded sites as of the others. Their counts are needed only for the
*Sources without records* of Lemma 17, and the parts of them that
[Open Steps](#open-steps) leaves open are open for those sources alone.

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
which Lemma 5 counts. A recorded walk also makes steps outside its
iterations, to look up, store and take records; Lemma 17 counts them with
the recorded walks, apart from the suffix copies, which no claim of this
proof bounds (step 8 of [Open Steps](#open-steps)). So the claim of this lemma comes down to a bound on the
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
evaluation $w$ below $N$. Either $w$ is a dropped step of a growth whose seed
ends at a position $e \le q$, which *Dropped steps* below treats, or $w$ is an
evaluation of a unit that starts at a position $s \le q$, which is committing
when the source is not a bounded scan. From
the end of $w$ to the start of $\alpha'$, the parser starts no evaluation at a
position before the start of $w$, apart from the **exception**: the later
steps of the growths on the path from $N$ down to $w$, and the evaluations in
them that follow no read of the seed of their step. An evaluation $x$ in a
step $S$ of a growth **follows a read of the seed** of $S$ when $S$ holds a
call $\rho$ that the growth answers with its growing result, or a call whose
result includes such a call, that succeeds and ends before $x$ starts, and the
innermost evaluation $M$ that contains both $\rho$ and $x$ is not a growth and
holds $\rho$ in a child $c$ before the child that holds $x$, with no abandoned
evaluation from $c$ down to $\rho$. So a read inside the expression of a
lookahead, or inside an alternative that fails, is followed by nothing, and
the evaluation of a growing alternative, which starts at the position of its
growth, follows no read, while its parts after its left call do. Every
evaluation in a later step that follows a read of its seed starts at or after
the end of that seed, which is at or after the start of $w$. The exception
lies inside the growths on the path from $N$ down to $w$, which contain
$\alpha$ and, since $N$ is no growth, not $\alpha'$, so they end before
$\alpha'$ starts and $\alpha'$ lies in no evaluation of the exception.

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
it contains $\alpha$. When $w$ is a dropped step of a growth $G$, $G$ lies on
the path from $c_1$ down to $w$, where every evaluation succeeds and is not
abandoned, so the input that $c_1$ consumes holds the result of $G$, which is
the seed of $w$, and $e \le \mathrm{end}(c_1) \le q$; $c_1$ is not
abandoned, since $w$ is the first abandoned evaluation.

Let $v$ be the first evaluation in progress that started before $s$, is
not in the exception of the claim, and fails after the end of $w$ and before
$\alpha'$. In that time the parser goes back to a position before $s$ only
through the failure of $v$, by starting a later step of a growth that started
before $s$, at the position of the growth, or inside an evaluation in the
exception, which starts no evaluation at a position before its own start. Such
a growth $G$ encloses $w$, since it is in progress
after $w$ ends and was not started inside $w$, so it lies on the path from $N$ to $w$, where every
evaluation succeeds and none is a dropped step: its step that holds $w$ ends
where the parser is, and its later steps read a seed that ends there or
later. Let $x$ follow a read $\rho$ of the seed in a later step $S$ of $G$,
with $M$ and $c$ as in the claim. An evaluation that is not abandoned ends at
or after the end of each child of it that is not abandoned: the parser goes
back inside it only to the start of an abandoned evaluation or, in a growth,
to its position for a later step, and a growth gives the result of its last
kept step, the longest. So $c$ ends at or after
the end of $\rho$, which is the end of the seed or later. $M$ is not a growth,
so after $c$ it goes on from the end of $c$, and $x$, in a later child, starts
at or after the end of the seed. Every other evaluation in $S$ follows no read
of the seed and lies in the exception. $S$ ends with a result longer than its
seed, or it is dropped and $G$ ends at the end of its seed. So, by induction
over the steps that start in that time, every evaluation that starts after
the end of $w$ and before the failure of $v$ starts at $s$ or later, or lies
in the exception of the claim, and each growth that encloses $w$ ends at $s$
or later. So $v$ started before the end of $w$, and since
it is still in progress after $w$ ends, it encloses $w$.
It ends before $\alpha'$, so it does not enclose $\alpha'$, and it lies on the
path from $N$ to $w$, where no evaluation fails. So there is no such $v$.

Let $w$ be a unit, and let the source not be a bounded scan; a unit whose
only scanning sources are bounded scans has no scanning source in the sense of
condition 2, which need not check it. $w$ contains the scanning source of
$\alpha$, or a call that leads to it. It is the expression of a lookahead, which condition 2
checks in any case, or it failed after $\alpha$ succeeded, having examined
input, so by the paragraph above it can fail after one of its scanning
sources has examined input. Condition 2 makes it committing. $\square$

**Dropped steps.** Let the evaluation $w$ of the restarts be a dropped step of
a growth $G$ of a leader $R$ at $p$, whose seed ends at $e \le q$. From the end
of $G$ to the start of $\alpha'$, the parser starts no evaluation at a position
before $e$, apart from the later steps of the growths that enclose $G$ and the
evaluations in them that follow no read of the seed of their step, as in the
restarts, and it evaluates from $e$ what follows the calls that start $G$, the
continuations of a step of $G$ after the first as a unit
([Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost),
condition 2). One of three cases holds.

- *A step that fails.* $\alpha$ follows, in $w$, a read of the seed of $G$ that
  succeeds, or a call whose result includes one, and $w$ fails. Then condition
  2 checks $w$ as a unit that starts at $e$, it is committing when the source
  is not a bounded scan, and the conclusion of the restarts holds for this
  unit with $s = e$.
- *A step that succeeds.* $\alpha$ follows such a read and $w$ succeeds with a
  result no longer than its seed. Then the path from $w$ down to $\alpha$ holds
  a first evaluation $w'$ below $w$ that fails, is the expression of a
  lookahead or is a dropped step, it starts at a position $s' \le e$, and the
  conclusion of the restarts holds for $w'$, with what the parser evaluates
  after it in $w$ and then from $e$ as its continuation.
- *At the position of the growth.* Otherwise $\alpha$ lies in a part that $w$
  evaluates at $p$ on a path that reads no growing result that succeeds, as
  in the second case of *Repeats inside a growth* in Lemma 5.

*Proof.* A growth ends with its first dropped step, and it gives the seed of
that step as its result, so $G$ ends when $w$ does, at $e$. Let $v$ be the
first evaluation in progress that started before $e$ and fails after the end
of $G$ and before $\alpha'$. Every evaluation that starts after the end of $G$
and before the failure of $v$ starts at $e$ or later, apart from the later
steps of the growths that enclose $G$ and the evaluations in them that follow
no read of the seed of their step, as in the restarts, since the parser goes on at $e$ and a dropped step that
starts in that time returns it to the end of its own seed, which is not before
the start of its growth. So $v$ started
before the end of $G$ and encloses $G$; it does not enclose $\alpha'$, so it
lies on the path from $N$ to $w$, where no evaluation fails. So there is no
such $v$. The parser goes on at $e$ after the call that started $G$: a call of
$R$ that is not a left call, or a call of another rule of the cycle that is not
a left call, in whose body $G$ ends after the left call of $R$ that its body
reaches ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost), the left calls). Up to
$\alpha'$ it evaluates from $e$ on what follows there, through the ends of rule
bodies and every call of their rules, the reads of the seeds of the growths
around included: this is what follows the calls that start the growth.

In the first case $\alpha$ lies in $w$ after a left call of $R$ that read the
seed, so it lies in a step tail of $R$, or in a call that a step tail makes.
By *Failure after a source*, applied to $w$, the evaluation $f$ whose failure
makes $w$ fail comes after $\alpha$, after the read, so in the same step
tail, and the innermost evaluation that holds both is a sequence, a required
repetition or a negative lookahead of the tail, or of a rule body that it
calls. So the step tail can fail after its source, condition 2 checks the
step as a unit that starts at $e$, and it is committing when the source is no
bounded scan. The arguments of *Direct activations* and *Repeats* below use of
$w$ only that it is checked against the continuation $\kappa$ that the parser
evaluates from its start, that the parser returns to no position before that
start until $\alpha'$ outside the exception of the restarts, which lies in
growths that end before $\alpha'$, and that $\alpha$ is consumed in $w$ after the calls
that $w$ and $\kappa$ share. Condition 2 compares the step with $\kappa$ one
step tail at a time, and these arguments need only the step tail that holds
$\alpha$: its evaluation $\tau$ in $w$ starts at $e$, at the end of the read,
every path of it that examines the input begins with the calls that it
shares with $\kappa$, and $\alpha$ is consumed in $\tau$ after them, so the
character at the end of the shared calls is a first character of that tail.
What other step tails of $w$ examine, before $\tau$ or after it, lies on no
path from $w$ to $\alpha$, and the calls that they share with $\kappa$, or do
not, play no part. They apply to $w$ with $s = e$, with the shared calls of
the tail of $\tau$ in the place of those of $w$, as they apply to each
growing alternative in *Repeats inside a growth*.

In the second case, if no evaluation below $w$ on the path from $w$ to
$\alpha$ failed, was the expression of a lookahead or was a dropped step, the
input that $\alpha$ consumes from $q \ge e$ would lie in the result of $w$,
which ends at or before $e$; a kept step of a growth on the path is part of the
seed of the next step and so of the result of the growth. So $w'$ exists. The
evaluations between $w$ and $w'$ succeed and are not abandoned, so the
argument of the restarts gives that from the end of $w'$ to the end of $w$ the
parser starts no evaluation before the position where the parent of $w'$ goes
on after it, apart from the exception of the restarts for the growths between
$w$ and $w'$: the start $s'$ of $w'$, or, for a dropped step, the end of its
seed, which this claim then takes as $s'$. These evaluations do not fail, so
$w$ ends at or after $s'$, and since the result of $w$ ends at or before $e$,
$s' \le e$. From the end of $w$ the parser returns to no position before $e$,
apart from the exception of the restarts, which lies in growths that end
before $\alpha'$, as shown above, so the conclusion of the restarts holds for $w'$ at $s'$. What
the parser evaluates after $w'$, through the end of the body of $R$ in $w$
and then from $e$, is a continuation of $w'$, which continues at the end of a
rule body through every call of the rule. $w'$ is then a unit, which the
restarts and *Failure after a source* make committing, or a dropped step, to
which this claim applies again, deeper on the path. $\square$

The activations of the third case are counted by Lemma 5 with those that the
second case of *Repeats inside a growth* gives, as stated there.

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

**Activations that do not succeed in a parse that keeps records.** In a
parse whose records are exact, (R1) of the [Setting](#setting),
*Activations that do not succeed* holds as stated, with $A_s$ and $A_f$
the numbers of the activations that the parse evaluates, the iterations
of the walks of the sites that keep records among them; an iteration
that a record answers is no activation and is in neither number. So does
Lemma 10, on which its proof rests.

*Proof.* The two proofs count evaluations that the parse makes, and they
use three things of it.

1. That each evaluation of a scanning source has at most one activation
   that does not succeed, its last. This is 7 of *Restarts in a parse
   that keeps records*, which shows it of a walk that takes a record.
2. That a place is evaluated at most a constant number of times in one
   body evaluation outside the scanning sources that enclose it, and at
   most that many times in one activation of the innermost such source:
   the argument of Lemma 1 and the *One place* of Lemma 10. Every
   evaluation of a place that the parse makes lies in a body evaluation
   that the parse makes, and, when a scanning source of that body
   encloses the place, in an activation of the innermost one, which the
   parse evaluates, since an iteration that a record answers holds no
   evaluation. Inside that body evaluation or activation the place is
   reached through the repetitions with an upper bound that enclose it,
   as without records: a walk that takes a record starts no evaluation
   between the position of the record and its end (1 of *Restarts in a
   parse that keeps records*), so a record takes evaluations away and
   adds none.
3. In Lemma 10, that each call of a rule that the parse does not memoize
   evaluates its body at most once, and that every body evaluation other
   than the one where the parse begins is made by a call. A walk that
   takes a record makes no call and evaluates no body. The rest of that
   proof, the chain of the rules called from one place, is about the
   grammar and those counts.

The proofs use nothing else of the parse. $\square$

The claim bounds $A_f$ and the evaluations of the rules that are not
memoized by $B$ and by all the successful activations that the parse
evaluates, those of the recorded walks included, which is why Lemma 17
bounds $A_r$ before it is applied (*Order* in Lemma 17). The exception
for the Operators Rules, which step 3 of [Open Steps](#open-steps)
keeps, is the same with records.

**Order of the bounds.** The bounds follow one another in this order, and none
rests on a later one. First, the claims of this lemma from *Failure after a
source* to *Nothing in a memoized link is a head*, other than *Activations that do not succeed*,
count nothing: they rest on the conditions, the memo of Lemma 3 and the memoization proof, and hold of every
activation, including those of growth steps and of evaluations in a context of
growths. Second, Lemma 5 bounds $B$, the evaluations in a context of growths,
the activations that growth steps and those evaluations own, and the successful
activations that a dropped step at the position of its growth (the third case
of *Dropped steps*) or a path that enters the growth again (case 5 below)
repeats, from those claims, Lemmas 1 to 3 and conditions 3
and 5; it must not use the bounds that come after it (step 4 of [Open
Steps](#open-steps)). Third, *Activations held by heads* bounds the successful
activations owned by memoized evaluations from Lemma 3, *Replayed activations*
and *Links to heads*, with no
count of the evaluations in a context of growths, and with the second bound
gives $A_s$. It rests on no bound of Lemma 5, so Lemma 5 uses it, applied to
the growths at calls as scanning sources, for the growth activations that
memoized evaluations own (*Growths at many positions*). Last, this claim bounds the evaluations of the rules that are not
memoized and $A_f$.

The claims from here to *Nothing in a memoized link is a head* concern the
scanning sources other than bounded scans, whose activations Lemma 13 counts.
By the restarts above, a successful activation of such a source is repeated at a
position only after a dropped step of a growth, which *Dropped steps* turns
into a unit at the end of its seed or into one below it, apart from its third
case, which Lemma 5 counts, or after a
committing unit that contains the earlier one is abandoned, and the parser does
not return before the start of that unit until the repetition, apart from what
the later steps of the growths around the unit evaluate before a read of their
seeds, inside growths that end before the repetition. The restarts
leave out two activations whose innermost common evaluation is a growth;
*Repeats inside a growth* in Lemma 5 shows that they are related in the same
way, through a unit whose continuation goes on through the reads of the seed,
unless the later one follows no read of the seed in its step, which Lemma 5
counts. The ways in which
a unit is committing give the following cases.

1. *Disjoint first characters.* The continuation does not consume the
   character at which it and the unit part, so nothing evaluated after the
   failure starts again inside the input that the unit examined beyond its
   shared calls.
2. *Shared calls.* A call that the unit and a continuation both evaluate first
   has the same result in both, by the [memoization
   theorem](memoization.md#theorem-memoization-never-changes-the-result) when
   the rule is memoized at both calls, or by repeating only its own scan when it
   calls no rule, so the continuation pays for it at most what the unit paid.
   For a rule of a left-recursion cycle, the two calls lie in the same growths
   of the cycle: neither is reached through a call of a rule of the cycle that
   the unit or the continuation starts with, so neither starts a growth of the
   cycle that the other is outside, and the continuation reaches its call
   without leaving the body of a rule of the cycle, so no growth of the cycle
   in progress at the first call has ended or gone on to its next step at the
   second, since a step ends only where the body of its leader ends. The
   growing results of the cycle are then the same at both calls, so a leader
   called in a context of growths grows again from the same growing results,
   evaluates what it evaluated the first time and costs at most that, and
   every other call of a rule of the cycle reads the same growing results or
   is answered by the memo. A
   rule that calls no rule repeats its scan in every continuation that evaluates
   it again at the position, so a call of it is not shared where the
   continuation reaches it after leaving a body through a call that nests at one
   end, and *Calls that call no rule* below bounds its re-evaluations at one
   position where the parser leaves or starts no growth step and no evaluation
   in a context of growths between them. A scan-only rule is shared there as
   well, by *Calls of scan-only rules* below. A rule of a cycle other than a leader is memoized at a call that is
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
uses the records of the [Setting](#setting): a candidate site counts here as
any other scanning source. In a parse that keeps records, *Counterparts
in the reference parse*, *Replays* and *Replayed activations* say what
holds, and they leave out what lies inside the iterations of the walks of
the sites that keep records, which *Descent in the reference parse* and
the claims after it treat in part; *Restarts in a parse that keeps records*
shows the claims from *Failure after a source* to *Memoized links* of such
a parse, *Heads in a parse that keeps records* those from *Answered
calls* to *Evaluated pairs of a shared call*, and *Leaf links in a parse
that keeps records* the first four claims on leaf links, and *Later leaf
links in a parse that keeps records* the claims on leaf links from *Memoized links to the frame of an outer
link* to *Nesting at one end*, as that claim states them: some with
case (d) of the covered keys, one with one more exception, some for
the activations outside the iterations of the walks of the sites that
keep records, and not *Calls of scan-only rules* or *Seed reads*;
*Activations that do not succeed in a parse that keeps records* shows
that claim, and *Iterations at one scan in a parse that keeps records*
shows *Iterations at one scan* in part, under a hypothesis on the shared
calls of a link; that the claims after *Iterations at one scan* hold of
it is left to be
argued (step 8 of [Open Steps](#open-steps)). For the iterations of recorded
walks Lemma 17 makes the claims unnecessary.

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
$s$, since no growth lies on the path from $N$ to $w$, so that the exception
of the restarts is empty; let $\kappa$ be the one that it evaluates from $s$ the last time before
$\alpha'$, whose evaluation contains $\alpha'$. Let $t$ be the position where
the calls that $w$ and $\kappa$ share end in $w$. They end there in $\kappa$
too: every path of both that examines the input begins with them, each has the
same result in both (case 2, which places the two calls of a rule of a cycle in
the same growths of the cycle), and each starts at the end of the one before.

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
no path that enters the growth again lead from the abandoned unit to
$\alpha'$, and $\alpha$ not lie in the third case of *Dropped steps*. With $w$
and $\kappa$ as above, where $w$ is the unit that *Dropped steps* gives when
the abandoned evaluation of the restarts is a dropped step, $\alpha$ lies
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
rule main = s "x" / s "w" / s ":" R "y" / P
rule P = (" " " ")* R "?"
rule s = "." (":" R "a")?
rule R = [ ]*
```

`P` evaluates `R` after any number of spaces, with a space next, so the
repetition of `R` is no bounded scan. On `.:` followed by spaces and `y`, the
option in `s` scans the spaces in `R` and fails at `y`, and `s` ends at $1$.
The second activation of the repetition of `R` at a space lies in the second
evaluation of `s`, inside the unit `s "w"`, whose continuation
`s ":" R "y"` shares the call of `s` at $0$. The memo answers that call, and
the next activation at the space lies in the call of `R` that `main` makes
after `:`, in no evaluation of `s`. The conditions accept the grammar, and on `.:` followed
by $2k$ spaces and `y`, the compute of a recognition on the closure backend
is $16k + 121$ for $k$ from $500$ to $4000$.

So the successful activations of $\sigma$ at $q$, $\alpha_1, \alpha_2, \ldots$
in the order in which they start, outside case 5, the third case of *Dropped
steps* and the repeats at the position of a growth that *Repeats inside a growth* of Lemma 5
leaves out, are direct
in distinct body evaluations of $R$, and for $i \ge 2$, $\alpha_{i-1}$ lies in
the evaluation $e_i$ of a call shared by a committing unit $w_i$ and its
continuation $\kappa_i$, inside $w_i$, where $w_i$ can be a step of a growth
after the first, a unit that starts at the end of its seed, whose shared calls
are those of the step tail that holds $\alpha_{i-1}$ (*Dropped steps*), where $\kappa_i$ is the continuation
that the parser evaluates from the start of $w_i$ the last time before
$\alpha_i$, as in *Direct activations*. When $\alpha_i$ lies in the evaluation
$e'_i$ of that call that $\kappa_i$ makes, at the same position (case 2), and
so after $w_i$ is abandoned, $i$ is
**linked**, and the pair $(e_i, e'_i)$ is the $i$-th **link**; otherwise $i$ is
**free**, as is $i = 1$. *Reference links* below gives each activation of the
reference parse such a pair, which the free ones lack in the parse.

**Memoized links.** If the rule of the $i$-th link is memoized at both calls
and neither $e_i$ nor $e'_i$ is in a context of growths, then either the rule is
no leader and $e_i$ and $e'_i$ are two of the at most six body evaluations of
the rule at its position and level that the memo allows (Lemma 3; the rule reads
no variable), or the rule is a leader and $e_i$ and $e'_i$ are two growths of it
at its position and level, $e_i$ in lookahead state $1$ and $e'_i$ in state $0$. At
most five such links of $\sigma$ at $q$ use one rule, position and level.

*Proof.* Outside contexts of growths, a leader grows at a call whose result at
the position is not in the memo
([Growths of Left Recursion](../runtime/parsing.md#growths-of-left-recursion)),
and the growth stores its result
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)), which answers every later call there outside contexts in the same lookahead
state or a greater one (*Answered calls* below). So of two growths of a leader
at one position and level outside contexts, the earlier is in lookahead state
$1$ and the later in state $0$, and there are at most two. The memo answers a call
without evaluating it, and $e'_i$,
which contains $\alpha_i$, is evaluated, and so is $e_i$, made earlier. If links
$i < k$ had $e'_k = e'_i$, then $\alpha_{k-1}$, which starts no earlier than
$\alpha_i$, would lie in $e_k$, which ended before $e'_k$ started, and so before
$\alpha_i$ started. So the links that use one rule, position and level have
distinct $e'_i$, none of which is the earliest of the evaluations. $\square$

**Restarts in a parse that keeps records.** In a parse whose records are
exact, (R1) of the [Setting](#setting), *Failure after a source*,
*Restarts*, *Dropped steps*, *Direct activations*, *Repeats* and
*Memoized links* hold as stated, and so does what *Operators Rules* states.
They hold of
all the successful activations of the parse, inside the iterations of the
walks of the sites that keep records as outside them.

*Proof.* The claims speak of activations, which the parse evaluates; an
iteration that a record answers is none, so it is in no sequence
$\alpha_1, \alpha_2, \ldots$ of the activations at a position, which holds
the iterations that the parse evaluates there and no others. The proofs of
*Restarts*, *Direct activations*, *Repeats* and *Memoized links* use four
kinds of facts about the parse, and 5 to 7 below treat the other three
claims.

1. *Where the parser goes back.* The proof of *Restarts* uses that the
   parser returns to an earlier position only at the start of an abandoned
   evaluation or, in a growth, at its position for a later step, and that an
   evaluation that is not abandoned ends at or after the end of each child
   that is not abandoned. A walk that takes a record moves the parser
   forward, from the position where it takes the record to the end of the
   record, and starts no evaluation between the two
   ([Skip Runs](../runtime/parsing.md#skip-runs),
   [Run Sites](../runtime/parsing.md#run-sites),
   [Scan Sites](../runtime/parsing.md#scan-sites)). So it adds no return,
   and such a walk ends at or after the end of every iteration that it
   evaluated.
2. *What evaluations give.* The proofs use the success and the end of the
   children of $N$, of the evaluations on the path from $N$ down to $\alpha$
   and of the calls that $w$ and $\kappa$ share. By (R1) a walk that takes a
   record has the success and the end of the walk that evaluated its
   remaining iterations, so every evaluation of the parse has the success
   and the end of its counterpart in $\mathcal{R}$ (*Counterparts in the
   reference parse* below, which uses none of these four claims). A shared
   call therefore has the same result in $w$ and in $\kappa$, as case 2
   states, whether or not a walk inside one of its two evaluations took a
   record.
3. *What a unit can consume.* The first characters of $w$ and of $\kappa$
   after their shared calls, the word of case 4 and the scanning sources of
   a unit are properties of the grammar, and the proofs apply them to the
   input that the evaluations consume and examine. Where a walk takes a
   record, the input between the position of the record and its end is the
   input that the remaining iterations would consume, by (R1), so the
   character at $t$ is a first character of $w$, and of $\kappa$, after
   their shared calls when a record covers it as when an evaluation
   consumes it, and a path of $\kappa$ reaches $q$ through a record only
   where the iterations that the record answers would reach it.
4. *The memo.* *Memoized links* uses the store and the answers of the memo
   outside contexts of growths and that an evaluation which contains an
   activation is evaluated. The records change neither.
5. *Failure after a source.* Its proof names the evaluation $f$, made after
   $\alpha$, whose failure $w$ passes on. In a parse that keeps records the
   evaluation $f$ need not be one that the parse makes: a walk of a scan
   site that takes a record fails when the characters that it matched fall
   short of its minimum ([Scan Sites](../runtime/parsing.md#scan-sites)); a
   walk of a run site or of a skip-run site looks up a record only after it
   has matched its minimum, and succeeds
   ([Skip Runs](../runtime/parsing.md#skip-runs)). So the claim is read in
   $\mathcal{R}$, which evaluates every walk in full (the
   [two parses](memoization.md#setting) of the memoization proof).
   $\mathcal{R}(w)$ fails as $w$ does, and the counterpart of $\alpha$ lies
   inside it and succeeds (2), so the proof applies to them as it stands,
   and its conclusion, that the unit of $w$ can fail after one of its
   scanning sources has examined input, is a property of the grammar. Where
   the proof of *Restarts* or the first case of *Dropped steps* applies the
   claim, it uses that conclusion alone.
6. *Dropped steps.* A walk is no growth and no step of one, so the records
   change neither when a growth ends nor the result that it gives. Beyond
   that the proof uses that the parser goes on at $e$ and returns to no
   earlier position outside the exception, and that an evaluation which is
   not abandoned ends at or after the end of each child that is not
   abandoned (1), which a walk that takes a record does for the iterations
   that it evaluated; the outcome and the end of $w$, of $w'$ and of the
   evaluations on the path from $w$ down to $\alpha$ (2); the first
   characters of a step tail after its shared calls (3); and, in its first
   case, the conclusion of *Failure after a source*, as in 5. Its three
   cases are told apart by the outcome of $w$ and by what $w$ evaluates on
   the way to $\alpha$, an activation, which lies in no iteration that a
   record answers. So *Repeats* holds also where its unit is the one that
   *Dropped steps* gives: by the proof of that claim, *Direct activations*
   and *Repeats* use of the unit only what 1 and 3 give.
7. *Operators Rules.* The claim describes how an Operators Rule tries its
   operator parts and its operands and which evaluations of its units the
   return of an inner level ends. A walk is no evaluation of an Operators
   Rule; an evaluation of one that lies inside an iteration is made whole,
   or not at all when a record answers the iteration; and a walk inside it
   that takes a record leaves the outcome and the end of the walk that
   evaluated its remaining iterations (2). That each evaluation of a
   scanning source has at most one activation that does not succeed, its
   last, holds of a walk that takes a record: each iteration that it
   evaluated before the record consumed input, since one that did not
   would have ended the walk, and after the record it evaluates nothing,
   or, for a scan site, the one test that ends the walk
   ([Scan Sites](../runtime/parsing.md#scan-sites)).

The proofs use nothing else of the parse. $\square$

The claim does not cover the claims on leaf links that follow; *Leaf
links in a parse that keeps records* below treats the first four of them,
and *Later leaf links in a parse that keeps records* those from
*Memoized links to the frame of an outer link* to *Nesting at one end*;
*Calls of scan-only rules* and *Seed reads* are shown by neither. And it
counts nothing: of the
activations inside the iterations of the walks of the sites that keep
records it gives the sequence $\alpha_1, \alpha_2, \ldots$ and its links, not
the bound of *Activations held by heads*, which rests on *Replayed
activations*.

**Restarts in a parse that resumes runs.** In a
[Document](../runtime/documents.md) parse whose records of sites are exact,
(R1) of the [Setting](#setting), the claims that *Restarts in a parse that
keeps records* shows hold as stated when the parse resumes runs, and that
claim holds of such a parse: of all its successful activations, the
iterations that a resumed walk reuses among them.

*Proof.* The proof of *Restarts in a parse that keeps records* uses its
seven facts about the parse, and each holds of a walk that resumes a run.
A resumed walk reuses the recorded iterations from its start on, then
evaluates iterations as a repetition does, possibly none, and, across one
edit, may reuse recorded iterations after them; it stops as a repetition
does ([Documents](../runtime/documents.md)). By [Lemma 3 of the Documents
proof](documents.md#lemma-3-resumed-runs) and its proof, the iterations
that it reuses and evaluates are those that its counterpart in
$\mathcal{R}$ visits, in the same order and with the same outcomes, each
starting where the one before ended; an iteration that fails is not
recorded, so it is one that the walk evaluates.

1. *Where the parser goes back.* Across a reused iteration the parser
   moves forward from its start to its end and starts no evaluation
   between the two, so a resumed walk adds no return, and it ends at or
   after the end of every iteration that it reuses or evaluates.
2. *What evaluations give.* Every evaluation of the parse has the success
   and the end of its counterpart in $\mathcal{R}$, a reused iteration and
   the walk around it included (*Counterparts in the reference parse*,
   which uses none of these claims).
3. *What a unit can consume.* The input from the start of a reused
   iteration to its end is the input that its counterpart consumes, so
   the first characters and words that the proofs apply hold of it as of
   the iterations that a walk evaluates.
4. *The memo.* A resumed walk makes no lookup and no store in the
   iterations that it reuses, and *Answered calls* holds as stated. A
   reused iteration contains no activation other than itself, so an
   evaluation that contains an activation other than itself is evaluated,
   as the proof of *Memoized links* uses.
5. *Failure after a source.* The claim is read in $\mathcal{R}$, as 5 of
   that proof reads it, with the counterparts of 2.
6. *Dropped steps.* A walk is no growth and no step of one, and the proof
   uses 1 to 3 and the conclusion of *Failure after a source*, as 6 of that
   proof does. A reused iteration is an activation that the parse makes,
   and the three cases are told apart by the outcome of $w$ and by what
   $w$ evaluates on the way to $\alpha$, which lies outside every reused
   iteration other than $\alpha$ itself.
7. *Operators Rules.* An evaluation of an Operators Rule inside a reused
   iteration is not made, and one inside an iteration that the walk
   evaluates is made whole; a resumed walk leaves the outcome and the end
   of its counterpart (2). Each evaluation of a scanning source has at
   most one activation that does not succeed, its last: the recorded
   iterations start at increasing positions, each ending where the next
   starts, so each reused iteration but the last recorded one consumed
   input, and an iteration that consumed nothing, reused or evaluated,
   ends the repetition, as it ends it in $\mathcal{R}$.

The proofs use nothing else of the parse, so they apply, with a reused
iteration as an activation that makes no call, to the parse with its
records of sites and its resumed walks. $\square$

*Activations that do not succeed in a parse that keeps records* and
Lemma 10 hold of such a parse too, with the iterations that resumed walks
reuse among the activations, in $A_s$ or in $A_f$ as they consume input or
not: the proof of that claim uses 7, given above; that every evaluation of
a place lies in an activation that the parse makes of the innermost
source that encloses it, which holds since a reused iteration holds no
evaluation and the parser starts none across it (1); and that a body is
evaluated only by a call, which a resumed walk does not make in the
iterations that it reuses. So does *Activations that do not succeed* of
a parse that keeps no record of a site and resumes runs.

The claim does not cover the claims on leaf links and the scans, nor
*Activations held by heads*; the eighth part of step 8 of [Open
Steps](#open-steps) lists them for a parse that resumes runs.

**Growths around a leaf link.** Let the rule $r$ of the $l$-th link call no
rule, let $e_l$ and $e'_l$ start at $p$, and let $w_l$ start at $p$, at the end
of its seed when it is a step of a growth after the first (*Dropped steps*).
Let $N_l$ be the innermost evaluation that contains $\alpha_{l-1}$ and
$\alpha_l$, and let $\bar{w}$ be the first abandoned evaluation below $N_l$ on
the path to $\alpha_{l-1}$: $w_l$, or a dropped step that holds it in the
second case of *Dropped steps*. Let no body evaluation that the parser leaves
at $p$ from the end of $w_l$ to the start of $e'_l$ be a growth step. Then
$N_l$ is no growth, no step of a growth on the path from $N_l$ down to $w_l$
ends before $\alpha_l$ starts, and from the end of $w_l$ to the start of
$\alpha_l$ the parser starts no evaluation at a position before $p$.

*Proof.* Call the growths on the path from $N_l$ down to $w_l$, $N_l$ included
when it is a growth and the growth of which $\bar{w}$ is a step excluded, the
growths around $w_l$. Each has one step on the path, which contains $w_l$; the
evaluations on the path other than $w_l$ and $\bar{w}$ succeed and are not
abandoned (*Restarts*, *Dropped steps*, and the first case of *Repeats
inside a growth* in Lemma 5 when $N_l$ is a growth), so that step is kept,
and a later
step of its growth starts only after it ends. When $\bar{w}$ is a dropped step,
dropping it returns the parser to the end of its seed, which is no earlier
than $p$, the start of $w_l$, nor than the end of an evaluation on the path
inside $\bar{w}$, since those succeed and lie in its result, which is no longer
than its seed. As the proofs of the restarts and of *Dropped steps* show, from
the end of $w_l$ the parser returns to a position before one where it is only
through the abandonment of an evaluation that started before that position and
is in progress, which encloses $w_l$ and so lies on the path, or through a
later step of a growth around $w_l$ and the evaluations in it that follow no
read of its seed, the exception of the restarts.

Suppose that a step on the path of a growth around $w_l$ ends before
$\alpha_l$ starts. These steps nest, so those that do form the inner part of
their chain: $S_0, \ldots, S_m$, from the inside out, steps of the growths
$G_0, \ldots, G_m$. $e'_l$ starts after each of them ends: it contains
$\alpha_l$ and starts after the end of $w_l$, so it does not enclose $S_k$, and
if it started inside $S_k$ it would end inside $S_k$, before $\alpha_l$. Until
$S_0$ ends, no later step of a growth around $w_l$ starts, since each starts
only after the step of its growth on the path, which contains $S_0$, ends, and
the only abandoned evaluation on the path is $\bar{w}$, after which the parser
is at $p$ or later. So the parser starts no evaluation before $p$ from the end
of $w_l$ until $S_0$ ends, and $S_0$ ends at $p$ or later. The parser leaves
$S_0$ after the end of $w_l$ and before $e'_l$ starts, so by the hypothesis
not at $p$: $S_0$ ends after $p$. For $k \ge 1$, $G_{k-1}$ lies in $S_k$, and
it ends at the end of the seed of its last step, which is no earlier than the
end of the result of $S_{k-1}$, a kept step, since the seeds of a growth grow
longer. From the end of $G_{k-1}$ until $S_k$ ends, the parser returns before
that position only through the dropping of $\bar{w}$, when $\bar{w}$ encloses
$G_{k-1}$, which leaves it no earlier than the end of $G_{k-1}$, or through a
later step of a growth around $w_l$ that encloses $G_{k-1}$, whose step on the
path contains $S_k$ and so has not ended. So $S_k$ ends no earlier than
$G_{k-1}$, and so after $p$. Let $H = G_m$.

If $e'_l$ did not lie in $H$, it would start after $H$ ends, no earlier than
the end of $S_m$, so after $p$, and up to $\alpha_l$ the parser would return
before the end of $H$ only through the dropping of $\bar{w}$, which leaves it
no earlier than the end of $H$, or through a later step of a growth around
$w_l$ that encloses $H$, whose step on the path is none of the $S_k$ and so
does not end before $\alpha_l$; $e'_l$ would not start at $p$. So $e'_l$, and
with it $\alpha_l$, lies in a later step of $H$, and $\alpha_{l-1}$, in $w_l$,
lies in $S_m$. $H$ contains both, so $N_l$ lies in $H$, and $H$ lies on the
path from $N_l$ down: $H$ is $N_l$, a growth, with $\alpha_{l-1}$ and
$\alpha_l$ in different steps. Both activations are in the sequence, so the
first case of *Repeats inside a growth* of Lemma 5 holds: $\alpha_l$ follows
a read $\rho$ of the seed of its step. $e'_l$, an evaluation of $r$, which
calls no rule, contains $\alpha_l$ and not $\rho$, so it follows $\rho$ too and
starts at or after the end of that seed (*Restarts*), which is no earlier
than the end of the result of $S_m$, after $p$: not at $p$. So no step on the path of a growth around $w_l$ ends
before $\alpha_l$ starts.

If $N_l$ were a growth, its step that holds $\alpha_{l-1}$ would lie on the
path and end before its later step that holds $\alpha_l$ starts; so it is
none, and no later step of a growth around $w_l$ starts before $\alpha_l$.
By the second paragraph, the parser then starts no evaluation before $p$ from
the end of $w_l$ to the start of $\alpha_l$. $\square$

The hypothesis holds for every link of a run of *Calls that call no rule*
below, whose hypothesis covers the time from the end of each unit of the run
to the later evaluation of its link. Without it, the restarts leave open that
a later step of a growth around the unit starts before $p$ before the next
activation, as their exception states.

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
consuming input. A step of a growth after the first that is $w_l$ starts at
$p$ as a unit, at the end of its seed. It returns the parser to $p$ when it is
dropped, so for $l > i$ the hypothesis leaves it out, as a growth step left at
$p$. When $w_i$ is one, the proof uses of it only what it uses of a unit at
$p$: that it starts there as a unit and that from its end the parser starts
no evaluation before $p$ until $\alpha_i$ (*Growths around a leaf link*). An evaluation that
the step starts before $p$ lies inside it and ends before it. Call the body evaluations in progress the frames. Let $N_l$
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
before $p$: it starts none from the end of each $w_j$ to $e'_j$ (*Growths
around a leaf link*, whose hypothesis the one here gives), and from $e'_j = e_{j+1}$ to the end of $w_{j+1}$
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

**Outer links across runs.** Let $l < l'$ be leaf links of $\sigma$ at $q$
whose shared call is the first, both at one position $p$, so that $w_l$ and
$w_{l'}$ start at $p$, and let both leave frames, with $Y_l$, $Y_{l'}$,
$\pi_l$ and $\pi_{l'}$ as in the proof above. Let no body evaluation that
the parser leaves or starts at $p$ from the end of $w_l$ to $e'_l$, or from the
end of $w_{l'}$ to $e'_{l'}$, be a growth step or in a context of growths, as in
a run of *Calls that call no rule*; the proof above uses of the links $i$ to
$k$ for these only that the unit of the link starts at $p$ and that from its
end the parser starts no evaluation before $p$ until the next activation,
which *Growths around a leaf link* gives under this hypothesis.
Let both be outer, with $\pi_l = \pi_{l'}$. Then $Y_{l'}$ encloses $w_l$, or
$Y_{l'}$ starts after $e'_l$ ends and an index $j$ with $l < j < l'$ has
$\alpha_j$ inside $Y_{l'}$ and $w_j$ starting at a position no later than
the start of $Y_{l'}$, so before $p$.

*Proof.* $Y_{l'}$ encloses $w_{l'}$, which contains $\alpha_{l'-1}$, and
$\alpha_{l'-1}$ starts no earlier than $\alpha_l$, which starts after $w_l$
ends, so $Y_{l'}$ is in progress after $w_l$ ends. It was not started inside
$w_l$, since an evaluation started inside $w_l$ ends inside it. If it was
started before $w_l$ ended, it was started before $w_l$ and encloses it.
Otherwise it was started after $w_l$ ended. From the end of $w_l$ to the
start of $\alpha_l$ the parser starts no evaluation before $p$ (*Growths
around a leaf link*), and while $e'_l$ is in progress it starts only
evaluations inside the body of $r$, which calls no rule, at $p$ or later. As
$Y_{l'}$ starts before $p$, it starts after $e'_l$ ends. Let $j$ be the least
index whose $\alpha_j$ starts after $Y_{l'}$ starts. $\alpha_l$ starts before
$Y_{l'}$ and $\alpha_{l'-1}$ after it, so $l < j \le l' - 1$. $\alpha_j$
starts no later than $\alpha_{l'-1}$, which lies in $Y_{l'}$, so it starts
while $Y_{l'}$ is in progress and lies inside it. $w_j$ contains
$\alpha_{j-1}$, which starts before $Y_{l'}$, so $w_j$ started before
$Y_{l'}$; it ends before $\alpha_j$ starts, so it does not enclose $Y_{l'}$,
and it ended before $Y_{l'}$ started. From the end of $w_j$ to the start of
$\alpha_j$ the parser starts no evaluation before the start $s_j$ of $w_j$
apart from the exception of the restarts (*Restarts*, *Dropped steps*), so
$Y_{l'}$ starts at $s_j$ or later unless it lies in that exception. $Y_{l'}$ is
no growth step, since the parser leaves it at $p$ from the end of $w_{l'}$ to
$e'_{l'}$. If it lay in a later step $S$ of a growth $G$ on the path from
$N_j$ down to $w_j$ and followed no read of the seed of $S$, then $\alpha_j$,
inside $Y_{l'}$, would lie in $S$, and $\alpha_{j-1}$, inside $w_j$, in an
earlier step, so $N_j$ would be $G$, and the first case of *Repeats inside a
growth* of Lemma 5 would hold: $\alpha_j$ would follow a read $\rho$ of the
seed of $S$. $Y_{l'}$ contains $\alpha_j$ and follows no read, so it contains
$\rho$: otherwise the innermost evaluation that contains $\rho$ and $\alpha_j$
would contain $Y_{l'}$ in the child that holds $\alpha_j$, and $Y_{l'}$ would
follow $\rho$. It would then start at the position of $G$, where $S$ starts,
reach the read and be reached from the start of $S$ without consuming
input, so its rule would be of the cycle of $G$, and $Y_{l'}$, which $G$ is in
progress around at its position, would be in a context of growths, which the
hypothesis excludes. So $Y_{l'}$ does not lie in the exception and starts at
$s_j$ or later. In both cases $w_j$ starts at a position
no later than the start of $Y_{l'}$, before $p$. $\square$

A frame that a link $m$ leaves succeeds and is no dropped step: it lies on
the path from $N_m$ to $\alpha_{m-1}$ above $w_m$, the first abandoned
evaluation on that path. If it were a step of a growth that started before
$p$, that growth would go on after it with another step from its own
position, before $p$, since a growth ends only with a dropped step; the
parser would start that step before $\alpha_m$, after the end of $w_m$, which
starts at $p$. *Restarts* allows this for a growth on the path down to $w_m$,
which a growth around the frame is, and the hypothesis of the claim above
excludes it for $Y_l$ and $Y_{l'}$: each is left at $p$ after the end of the
unit of its link and before its later evaluation, so neither is a growth
step, and *Growths around a leaf link* gives that from the end of $w_l$ the
parser starts no evaluation before $p$ until $\alpha_l$. The uses below of
these two facts hold for links that meet that hypothesis, as the links of a
run of *Calls that call no rule* do.

Two outer links through one call place of which the later $Y$ does not
enclose the earlier unit therefore have between them an index whose unit
starts before $p$, which is free, linked by a memoized link, linked by a leaf
link whose shared call is not the first, or linked by a leaf link at a
position before $p$; none of these lies in a run of consecutive leaf links at
$p$ whose shared call is the first, so the two lie in different runs. The
other case is the one that *Nesting at one end* rules out, with $Y_{l'}$ as
$X$ and $Y_l$ as $T$, under its hypotheses on the dropped steps of growths
on the path between them and on the alike calls, and apart from the four
points that it leaves open; this is not shown here. So outside that case the outer links at $p$ through
one call place are at most one in each run, and the argument of the claim above does not
bound them without a bound on the runs. The second case occurs in a grammar
that the conditions accept:

```zp
@start
rule main = &B B / P
rule P = (" " " ")* ws "?"
rule B = Y ws "x"
rule Y = S (ws ";")?
rule S = "a"+
rule ws = [ ]*
```

`P` evaluates `ws` after any number of spaces, with a space next, so the
repetition of `ws` is no bounded scan; without `P` it would be one, and the
claims above would not concern it. On `a`s followed by spaces and `x`, the lookahead evaluates `B`, whose call
of `Y` ends at the first space $p$, where the option fails after `ws` and
`B` calls `ws` again after leaving `Y`: a leaf link at $p$ through the call
of `Y` in `B`, outer, since `Y` starts at $0$. The lookahead succeeds and
its expression is abandoned, the entry of `B` made inside it does not answer
the call of `B` outside it, and
that call evaluates `Y` again from $0$, which gives a second outer link
through the same call place. The index between them is the memoized link of
the two evaluations of `B`, whose unit, the expression of the lookahead,
starts at $0$.

**Outer links inside a memoized link.** Let the $j$-th link of $\sigma$ at
$q$ be memoized and meet the hypotheses of *Links lead to repeated
evaluations*. Then, outside cases (a) and (b) of the covered keys, no head
holds an activation that starts while $e'_j$ is in progress. In particular,
in the second case of *Outer links across runs*, when the index $j$ there is
linked by such a link and $\alpha_{l'}$ starts before $e'_j$ ends, no head
holds $\alpha_{l'}$.

*Proof.* An activation $\alpha$ that starts while $e'_j$ is in progress lies
inside $e'_j$. Let an evaluation $Z$ of a key hold $\alpha$. $Z$ and $e'_j$
both contain $\alpha$, so one contains the other. If $Z$ contained $e'_j$ and
were not $e'_j$, the call that makes $e'_j$, of a rule that the parse
memoizes, would lie between $Z$ and $\alpha$, and $\alpha$ would lie outside
the own region of $Z$. So $Z$ is $e'_j$ or lies inside it, and it holds an
activation, $\alpha$, so by *Nothing in a memoized link is a head* it is not
the head of its class. $\square$

The same holds with $E_2$ of an evaluated pair of a shared call in the place
of $e'_j$, under the hypotheses of *Evaluated pairs of a shared call* and
outside cases (a) and (b) of the covered keys on its path: no head holds an
activation that starts while $E_2$ is in progress. The proof above uses of
$e'_j$ only that a call of a rule that the parse memoizes makes it, and that
claim in the place of *Nothing in a memoized link is a head*. The pair need not
be a link, so its unit need not hold an activation of $\sigma$ at $q$.

An activation that no head holds is held by a memoized evaluation that is not
a head, which *Replayed activations* charges to the head of its class, or by
no memoized evaluation, and then a growth step or an evaluation in a context
of growths owns it (Lemma 5). In the grammar above, the second outer link lies
in the evaluation of `B` outside the lookahead, the later evaluation of the
memoized link between the two outer links, so no head holds its activation.
What remains of the second case is an index $j$ that is free, linked by a leaf
link at a position before $p$, or linked by a memoized link whose later
evaluation ends before $\alpha_{l'}$ starts.

In the last case $e'_j$ and $Y_{l'}$ both contain $\alpha_j$, so one contains
the other. $e'_j$ does not contain $Y_{l'}$ other than as $Y_{l'}$ itself:
$Y_{l'}$ is in progress at $\alpha_{l'-1}$, and a frame around it that ends
before $\alpha_{l'}$ would be left by the link $l'$ after $Y_{l'}$, which is the
last frame that the link leaves. So $e'_j$ is $Y_{l'}$, or $Y_{l'}$ contains
$e'_j$ and is not $e'_j$.

**Memoized links to the frame of an outer link.** In the second case of
*Outer links across runs*, let the index $j$ there be linked by a memoized
link with $e'_j = Y_{l'}$, at a position $s_0 < p$, and let the parse keep no
records of scan sites. Then, after $e_j$ ends, $w_j$ evaluates no call of
$r$ at $p$, unless the sequence of the activations of $\sigma$ at $q$ leaves
out the activation at $q$ of that call, and if $w_j$ starts before $w_l$ ends, no evaluation inside $w_j$ outside the shared calls that it evaluates before $e_j$ fails before $e_j$ starts, and $Y_l$ lies in none of those shared calls, this is the case that
*Nesting at one end* rules out, with $e_j$ as $X$ and $Y_l$ as $T$, unless $r$
is a scan-only rule.

*Proof.* $e_j$ and $e'_j$ are evaluations of one call of the rule $Y$ of
$Y_{l'}$ at $s_0$, and $e'_j$ ends at $p$, where the link $l'$ leaves it, so
$e_j$ ends at $p$ too (case 2). If $w_j$ evaluated $r$ at $p$ after $e_j$,
that evaluation would make the activations of $e'_{l'}$, which evaluates $r$
at $p$ on the same input, since $r$ calls no rule and reads no variable; one
of them, which succeeds as $\alpha_{l'}$ does, would start at $q$ after
$\alpha_{j-1}$, which lies in $e_j$, and before $Y_{l'}$ starts, since $w_j$
ends before then, and so before $\alpha_j$, which starts after $Y_{l'}$
starts. Unless the sequence leaves it out, it would be an activation of the
sequence between $\alpha_{j-1}$ and $\alpha_j$. Nothing more follows of what
$w_j$ does after $e_j$: the unit $w_{l'}$ need not share the call of $r$ with
its continuation through every call of $Y$, since it can be committing by
deciding its continuations, which compares each way through a call of $Y$ on
its own ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost)), so what follows the
call in $w_j$ that makes $e_j$ can examine the input at $p$ and fail there.

Let $w_j$ start before $w_l$ ends. It is in progress at $\alpha_{j-1}$, which
starts no earlier than $\alpha_l$, after $w_l$ ends, and it was not started
inside $w_l$, so it encloses $w_l$, as $Y_l$ does. It does not lie inside
$Y_l$, which ends before $\alpha_l$ starts, so it encloses $Y_l$. The shared
calls that $w_j$ evaluates before $e_j$ end at or before $s_0 < p$, and $w_j$
examines no input outside its shared calls before $e_j$ on each way through
it. By the hypothesis no evaluation inside $w_j$ outside those shared calls
fails before $e_j$ starts, so before $e_j$ the parser takes one way through
$w_j$ and evaluates on it, outside the shared calls, only what examines no
input. $Y_l$ examines the input from before $p$ to $p$, so it lies in no such
evaluation, and by the hypothesis it lies in none of the shared calls; so it
is not evaluated before $e_j$; it does not start after $e_j$ ends, since $e_j$ contains
$\alpha_{j-1}$, which starts after $Y_l$ ends. So $Y_l$ lies inside $e_j$, and
is not $e_j$. $e_j$ succeeds and ends at $p$, and $Y_l$ is an evaluation of
$\pi_l$ that starts before $p$, ends at $p$ and contains $\alpha_{l-1}$; $Y$
reaches the caller of $\pi_l$, which calls $Y$, so $Y$ is of the recursion of
$\pi_l$. These are the hypotheses of *Nesting at one end* with $e_j$ as $X$
and $Y_l$ as $T$, which give that $\pi_l$ nests at one end under its
hypotheses on the dropped steps and the alike calls and apart from its open
points; the continuation of $w_l$ then reaches the call of $r$ after leaving
the body of $Y_l$, in what follows $\pi_l$, so the call is not shared unless
$r$ is a scan-only rule. *Calls of scan-only rules* treats that rule in a
parse with the records of scan sites, which this claim excludes, so a
scan-only $r$ in a parse without them is not treated here. $\square$

The end of a shared call before $e_j$ at or before $s_0$ does not keep $Y_l$
out of it: an abandoned evaluation inside the call, such as the expression of
a lookahead, can examine the input up to $p$ and hold the whole of the link
$l$, and no claim here rules that out, so the hypothesis states it. Without
the first part of the hypothesis $Y_l$ can lie in an evaluation inside $w_j$,
outside the shared calls, that fails before $e_j$ starts, such as an earlier alternative, an option or a
repetition whose expression fails, or the expression of a negative lookahead,
in no evaluation of a call of $Y$ around it, and the claim does not apply. In
`main = "[" -(A "x" / C "z")? B "y" / P`, with `A = {v := 1} Z`,
`C = {v := 3} Y {v == 7} ws`, `B = {v := 2} Z`, `Z = Y {v >= 0} ws`,
`Y = S (ws ";")?`, `S = "a"+`, `ws = ([ ] {true})*`, which is not a scan-only
rule, and `P` as in *Outer links across runs*, which the conditions accept,
the unit of the index between the two outer links through the call of `Y`
in `Z` is the choice inside the option, which holds $Y_l$ in `A "x"` and
$e_j$ in `C "z"`; on `[` followed by $n$ `a`s, $n$ spaces and `y`, the
compute of a recognition is $16n + 151$ on the closure backend and $16n +
161$ on the bytecode backend for $n$ from $250$ to $2000$. That case is not
shown.

Apart from that scan-only case and the cases of the paragraph above, in which an evaluation inside $w_j$ outside the shared calls that it evaluates before $e_j$ fails before $e_j$ starts or $Y_l$ lies in one of those shared calls, the case $e'_j = Y_{l'}$ that remains is that of a unit $w_j$ that starts
after $w_l$ ends and evaluates no call of $r$ at $p$ after $e_j$, unless the
sequence of the activations of $\sigma$ at $q$ leaves out the activation at
$q$ of that call. Let $m$ be the least index
after $l$ whose unit starts before $p$, so that $m \le j$. From the end of
$w_l$ to the start of $\alpha_{m-1}$ the parser starts no evaluation before
$p$ (*Growths around a leaf link* for $l$, and *Restarts* and *Dropped
steps* for each index between, whose unit starts at $p$, outside the exception
of the restarts; that no later step of a growth around such a unit starts
before $p$ before the next activation is shown only for an index linked by a
link that meets the hypothesis of *Growths around a leaf link*), so $w_m$, which
contains $\alpha_{m-1}$, started before $w_l$ ended and encloses $Y_l$, as
$w_j$ does above. Here $m < j$, and $w_j$, which contains $\alpha_{j-1}$, starts
after $w_m$ ends: the parser returns before $p$ after $\alpha_l$ through a
unit $w_m$ that contains $Y_l$ and $\alpha_l$, as the expression of the lookahead `&B`
does in `main = &B "b" &Y B / P`, with the other rules as above and
`S = [ab]+`. That expression does not commit against `"b" &Y B`, and the
conditions reject the grammar. The conditions do not keep such a unit from
committing through a call that it shares with its continuation and that holds
$Y_l$, so the case is not ruled out by the units alone; the claim below places
$w_j$ after the last unit that encloses $Y_l$.

**The last unit around the earlier frame.** In the case above, where $w_j$
starts after $w_l$ ends, let $\bar{m}$ be the greatest index below $j$ whose unit
encloses $Y_l$, and let $\bar{s}$ be the start of $w_{\bar{m}}$. Let no step of a
growth that encloses $Y_l$ be dropped before $\alpha_{j-1}$ starts, nor one
that contains $\alpha_{j-1}$. Then
$m \le \bar{m} < j$, from the end of $w_{\bar{m}}$ to the start of $w_j$ the parser
starts no evaluation at a position before $\bar{s}$ apart from the exception
of *Restarts* for the growths that enclose $w_{\bar{m}}$, and $w_j$ lies in the
evaluation from $\bar{s}$ of a continuation of $w_{\bar{m}}$, which goes on at
the end of the body of a leader through the reads of the seed in the later
steps of those growths, or $w_j$ starts at the position of one of those
growths, in a later step, and holds a read of the seed of that step that
$\alpha_{j-1}$ follows; then the part of $w_j$ after that read, which starts at
the end of the seed, lies in that evaluation of the continuation.

*Proof.* $w_m$ encloses $Y_l$, and $w_j$ does not, since it starts after
$w_l$ ends and $Y_l$, a frame that the link $l$ leaves, encloses $w_l$; so
$\bar{m}$ exists and $m \le \bar{m} < j$. For an index $k$, the first abandoned
evaluation on the path from $N_k$ to $\alpha_{k-1}$ is $w_k$ unless it is a
dropped step (*Restarts*, *Dropped steps*); when it encloses $Y_l$ and ends
before $\alpha_{j-1}$ starts, the hypothesis makes it no dropped step, so it
is $w_k$. This holds for $k = \bar{m}$: that evaluation encloses $w_{\bar{m}}$, and
it ends before $\alpha_{\bar{m}}$ starts, so before $\alpha_{j-1}$. So $w_{\bar{m}}$
fails or is the expression of a lookahead, and when it ends the parser is at
$\bar{s}$. Apart from the evaluations in the exception of *Restarts* for the
growths that enclose $w_{\bar{m}}$, which start no evaluation outside that
exception at a position before $\bar{s}$, as the proof of the restarts shows,
only the abandonment of an evaluation $v$ that started before
$\bar{s}$ then returns the parser before $\bar{s}$; let $v$ be the first such
evaluation outside the exception abandoned after the end of $w_{\bar{m}}$ and
before $w_j$ starts. Everything that the parser starts after the end of
$w_{\bar{m}}$ and before $v$ ends starts at $\bar{s}$ or later or lies in the
exception, so $v$ started before the end of $w_{\bar{m}}$, and since it is in
progress after that end, it encloses $w_{\bar{m}}$, and with it $Y_l$ and
$\alpha_{\bar{m}-1}$. It ends before $w_j$ starts, so it does not contain
$\alpha_{j-1}$. Let $k$ be the last index with $\alpha_k$ in $v$, so that
$\bar{m} - 1 \le k < j - 1$. The innermost evaluation $N_{k+1}$ that contains
$\alpha_k$ and $\alpha_{k+1}$ encloses $v$, which is abandoned and lies on
the path from $N_{k+1}$ to $\alpha_k$, so the first abandoned evaluation on
that path is $v$ or encloses it. It encloses $Y_l$ and ends before
$\alpha_{k+1}$ starts, so it is $w_{k+1}$, which encloses $Y_l$. If
$k = \bar{m} - 1$, then $w_{\bar{m}}$ is that evaluation, but $v$ is abandoned and
strictly encloses $w_{\bar{m}}$. Otherwise $\bar{m} < k + 1 < j$, against the choice
of $\bar{m}$. So there is no such $v$, and as in *Direct activations* the parser
evaluates from $\bar{s}$ continuations of $w_{\bar{m}}$ up to the start of $w_j$,
going on at the end of the body of a leader through the reads of the seed, as
in the first case of *Repeats inside a growth* in Lemma 5. $w_j$ lies in the
last of them: otherwise it starts before $\bar{s}$, in the exception, and so it
is a later step $S$ of a growth $G$ that encloses $w_{\bar{m}}$, or the unit
that *Dropped steps* gives for such a step, or it lies in $S$ and follows no
read of the seed of $S$. In the first two cases $S$ is the first abandoned
evaluation on the path from $N_j$ to $\alpha_{j-1}$, and so a dropped step of
$G$, which encloses $Y_l$, that contains $\alpha_{j-1}$, against the
hypothesis. In the third, $\alpha_{j-1}$ lies in $S$, and $\alpha_{\bar{m}-1}$,
inside $w_{\bar{m}}$, in an earlier step of $G$, so $G$ is the innermost
evaluation that contains both. If $\alpha_{j-1}$ follows no read of the seed of
$S$, the sequence leaves it out (second case of *Repeats inside a growth*).
Otherwise it follows such a read $\rho$, and $w_j$, which contains
$\alpha_{j-1}$ and follows no read, contains $\rho$: were $\rho$ outside
$w_j$, the innermost evaluation that contains $\rho$ and $\alpha_{j-1}$ would
contain $w_j$ and hold it in the child that holds $\alpha_{j-1}$, and $w_j$
would follow $\rho$. So $w_j$ starts at the position of $G$, before $\rho$,
and the part of it after $\rho$, which holds $\alpha_{j-1}$, starts at the end
of the seed, which is at or after $\bar{s}$, as in the proof of *Restarts*; the parser reaches it
through $\rho$, a read of the seed in a later step of $G$, so it lies in the
evaluation from $\bar{s}$ of the continuation, which is the second form of the
claim. $\square$

The proof goes on to the start of $\alpha_j$: from the end of $w_{\bar{m}}$ to
the start of $\alpha_j$ the parser starts no evaluation at a position before
$\bar{s}$ apart from the same exception. Let $v$ be as in the proof, abandoned
before $\alpha_j$ starts in place of before $w_j$ starts. It ends before
$\alpha_j$ starts, so $k < j$, and $k = j - 1$ is the one new case: the first
abandoned evaluation on the path from $N_j$ to $\alpha_{j-1}$ then encloses $v$
and with it $Y_l$, and it contains $\alpha_{j-1}$, so by the hypothesis it is
no dropped step and is $w_j$, which does not enclose $Y_l$. So, outside that
exception, each evaluation that the parser starts from the end of $w_{\bar{m}}$
to the start of $\alpha_j$ starts at $\bar{s}$ or later, and one that is still
in progress when $\alpha_j$ starts lies, with $\alpha_j$, in the evaluation
from $\bar{s}$ of the continuation $\hat{\kappa}$ of $w_{\bar{m}}$ that the
parser evaluates there the last time before $\alpha_j$, as in *Direct
activations*. $\hat{\kappa}$ need not be the continuation whose evaluation
holds $w_j$: $w_j$ can start at $\bar{s}$ and be the evaluation of an earlier
continuation from there.

**Frames around the frame of an outer link.** Let a link $l'$ of a run of
*Calls that call no rule* leave frames, and let an evaluation $E$ of a call
hold $Y_{l'}$ inside one of its body evaluations, other than as that body
evaluation. Then $E$ holds $e'_{l'}$, and with it $\alpha_{l'}$. If $E$ is the
later evaluation $E_2$ of a pair that meets the hypotheses of *Evaluated pairs
of a shared call*, then, outside cases (a) and (b) of the covered keys on the
path of that pair, no head holds $\alpha_{l'}$.

*Proof.* Let $B$ be the body evaluation of $E$ that holds $Y_{l'}$. It is not
$Y_{l'}$, so it encloses $Y_{l'}$, which encloses $w_{l'}$, and the parser
leaves it after it leaves $Y_{l'}$, after the end of $w_{l'}$. Suppose that $B$
ended before $e'_{l'}$ started. $B$ contains $\alpha_{l'-1}$, which lies in
$w_{l'}$, and not $e'_{l'}$, and $N_{l'}$ contains both (*Calls that call no
rule*), so $B$ lies inside
$N_{l'}$. $B$ would then be a frame that encloses $w_{l'}$ inside $N_{l'}$ and
that the parser leaves from the end of $w_{l'}$ to $e'_{l'}$, a frame that the
link leaves, and the link would leave it after $Y_{l'}$, which is the last
frame that it leaves. So $B$ is in progress when $e'_{l'}$ starts. $B$ started
before $Y_{l'}$, and so before $e'_{l'}$, so $e'_{l'}$ starts while $B$ is in
progress, lies inside $B$ and so inside $E$, and $\alpha_{l'}$ lies in
$e'_{l'}$. When $E$ is $E_2$, $\alpha_{l'}$ starts while $E_2$ is in progress,
and the claim of *Outer links inside a memoized link* for an evaluated pair in
the place of $e'_j$ gives the rest. $\square$

The claim needs nothing of the index $j$ or of the units between the two
outer links: every frame that encloses $Y_{l'}$ and is not $Y_{l'}$ is still in
progress at $\alpha_{l'}$.

In the case $e'_j = Y_{l'}$ above, $\alpha_j$ is reached through no
evaluation of the exception of the restarts for the growths that enclose
$w_{\bar{m}}$ that follows no read of the seed of its step. Let $\alpha_j$
lie in a later step $S$ of such a growth $G$. $\alpha_{l-1}$, inside
$w_{\bar{m}}$, lies in the step of $G$ that holds $w_{\bar{m}}$, an earlier
one, so $G$ is the innermost evaluation that contains $\alpha_{l-1}$ and
$\alpha_j$, and *Repeats inside a growth* of Lemma 5 applies to them. In its
second case, where $\alpha_j$ follows no read of the seed of $S$, the
sequence of the activations of $\sigma$ at $q$ leaves $\alpha_j$ out, so
that case does not arise for $\alpha_j$. In its first case $\alpha_j$
follows a read of the seed of $S$ and starts at or after the end of that
seed, which is at or after $\bar{s}$ (*Restarts*): the parser reaches it
through that read,
in the evaluation from $\bar{s}$ of $\hat{\kappa}$, which goes on at the end
of the body of a leader through the reads of the seed, as in the claim above
and in that first case. Outside case 5, the argument of *Repeats* then
applies to $w_{\bar{m}}$ and $\hat{\kappa}$, with $\alpha_{l-1}$, which lies in $Y_l$ inside
$w_{\bar{m}}$, in the place of $\alpha$, and $\alpha_j$, which the parser
reaches in the evaluation of $\hat{\kappa}$ from $\bar{s}$ without returning
before $\bar{s}$, in the place of $\alpha'$: $\alpha_{l-1}$ lies in the
evaluation $E_1$ inside $w_{\bar{m}}$ of a call $c$ that $w_{\bar{m}}$ and
$\hat{\kappa}$ share. Let $E_2$ be the first evaluation of $c$ that
$\hat{\kappa}$ makes at the same position (case 2). When $E_2$ holds
$Y_{l'}$ other than as its body evaluation, the claim above applies to
$E_2$, and some of the
hypotheses of the evaluated pairs follow. $w_{\bar{m}}$ is a committing unit
(*Restarts*), since the proof of *The last unit around the earlier frame*
shows that it is the first abandoned evaluation on its path and no dropped
step. The rule of $c$ calls a rule, since its body evaluation holds $Y_{l'}$,
another body evaluation, so the call is shared only as a call of a rule that
reads no variable and that the parse memoizes at both calls. The memo answers
neither call: $E_1$ holds $\alpha_{l-1}$ and $E_2$ holds $Y_{l'}$, and a call
that the memo answers holds no evaluation. Two hypotheses are not shown:
that neither $E_1$ nor $E_2$ is in a context of growths, since the hypothesis
of *Outer links across runs* concerns only the
evaluations that the parser leaves or starts at $p$ between a unit and the
later evaluation of its link, which neither is; and that each growth at their
position that is open at the start of one is open at the start of both or read
by neither.

What remains of this case, besides those two hypotheses, is where $Y_{l'}$
lies against $E_2$. $E_2$ need not hold $\alpha_j$: the argument places
$\alpha_{l-1}$ in $E_1$, not $\alpha_j$ in $E_2$, and after a path through
$E_2$ fails, $\hat{\kappa}$ can evaluate $c$ again at its position, when
$E_2$ left no entry that answers that call, and reach $q$ in that later
evaluation. When $E_2$ holds
$\alpha_j$, $Y_{l'}$, which holds it too, lies inside $E_2$ or encloses it.
The claim above treats $Y_{l'}$ inside a body evaluation of $E_2$ other than
as that body evaluation, and the claim below the case in which
$Y_{l'}$ is the body evaluation of $E_2$, so that $c$ is the call at
$\pi_{l'}$, apart from $Y_l$ being the body evaluation of $E_1$. Not shown
are that remaining case and the case in
which $Y_{l'}$ encloses $E_2$, as the body evaluation of a call with which
$\hat{\kappa}$ starts and whose body evaluates $c$ first. Nor is the case shown
in which $Y_{l'}$ contains $e'_j$ and is not $e'_j$, nor the case of a growth
step around $Y_l$ that is dropped, nor the second form of the claim, in which
the part of $w_j$ after its read of the seed takes the place of $w_j$ and is
checked from the end of the seed as a growing unit, as in the first case of
*Dropped steps*.

**The frame of an outer link as the later evaluation of a shared call.** In
the application above, let $Y_{l'}$ be the body evaluation of $E_2$, so that
the call of $c$ in $\hat{\kappa}$ is $\pi_{l'}$ and its rule is the rule $Y$
of $Y_{l'}$. Then $E_1$ succeeds and ends at $p$, and $Y_l$ is the body
evaluation of $E_1$, lies inside a body evaluation of $E_1$ other than as
it, or contains $E_1$ and is not its body evaluation. In the second case,
this is the case that *Nesting at one end* rules out, with $E_1$ as $X$ and
$Y_l$ as $T$, and in the third, with $Y_l$ as $X$ and $E_1$ as $T$, unless
$r$ is a scan-only rule.

*Proof.* $Y_{l'}$ is no growth step, since the parser leaves it at $p$ from
the end of $w_{l'}$ to $e'_{l'}$ (*Outer links across runs*), so it is the
only body evaluation of $E_2$, and it succeeds and ends at $p$, where the
link $l'$ leaves it; so $E_2$ succeeds and ends at $p$. $E_1$ is an evaluation
of the same call at the same position, and has the same result (case 2), so
it succeeds and ends at $p$ too. $E_1$ and $Y_l$ both contain
$\alpha_{l-1}$, so one contains the other, which gives the three cases. In the
second, $E_1$ is an evaluation of a call of $Y$, and the calls from $E_1$ down
to $Y_l$ lead from $Y$ to the caller of $\pi_l$, which calls $Y$ at $\pi_l$,
so $Y$ is of the recursion of $\pi_l$. $E_1$ succeeds and ends at $p$, and
$Y_l$, an evaluation of $\pi_l$ inside $E_1$ other than $E_1$, starts before
$p$, ends at $p$ and holds $\alpha_{l-1}$, a successful activation of the
scanning source $\sigma$. These are the hypotheses of *Nesting at one end*
with $E_1$ as $X$ and $Y_l$ as $T$, which give that $\pi_l$ nests at one end
under its hypotheses on the dropped steps and the alike calls and apart from
its open points. The continuation of $w_l$ then reaches the call of $r$
after leaving the body of $Y_l$ through $\pi_l$, in what follows $\pi_l$, so
the call is not shared unless $r$ is a scan-only rule, as in the proof of
*Memoized links to the frame of an outer link*.

In the third, let $c_1$ be the call of $c$ in the unit, of $Y$, and $B_1$ the
body evaluation of $E_1$ that holds $\alpha_{l-1}$. $w_l$ and $E_1$ both
contain $\alpha_{l-1}$, and $w_l$ starts at $p$ and $E_1$ before it, so $w_l$
lies inside $B_1$. $B_1$ lies inside $Y_l$, and the parser leaves every
frame between $w_l$ and $Y_l$ before it leaves $Y_l$, so the link $l$ leaves
$B_1$, through $c_1$. The calls from $Y_l$ down to $E_1$ lead from $Y$ to the
caller of $c_1$, which calls $Y$ at $c_1$, so $Y$ is of the recursion of
$c_1$. The call evaluation of $Y_l$, a call of $Y$, succeeds and ends at $p$,
and $E_1$, an evaluation of $c_1$ inside it other than it, starts before $p$,
ends at $p$ and holds $\alpha_{l-1}$. *Nesting at one end* with that call
evaluation as $X$ and $E_1$ as $T$ gives that $c_1$ nests at one end, under
the same hypotheses and open points, and the continuation of $w_l$ reaches
the call of $r$ after leaving $B_1$ through $c_1$, so again the call is not
shared unless $r$ is a scan-only rule. $\square$

The first case is not shown. In it the unit makes its
call of $c$ at $\pi_l$ too, so the parser evaluates the call place $\pi_l$ at
the start of $Y_{l'}$ once inside $w_{\bar{m}}$ and once more in
$\hat{\kappa}$, and the argument above, which needs an evaluation of a call
of $Y$ around $Y_l$ other than $Y_l$, does not apply to $E_1$. As in *Memoized links to the
frame of an outer link*, *Calls of scan-only rules* treats a scan-only $r$ in
a parse with the records of scan sites, and no claim treats it in a parse
without them.

Whenever $Y_{l'}$ is the body evaluation of $E_2$, the two hypotheses on the
growths that the application above leaves open hold for $E_2$. $Y_{l'}$ is its
only body evaluation, and the parser leaves it at $p$ from the end of $w_{l'}$
to $e'_{l'}$, so it is in no context of growths (*Outer links across runs*).
If $E_2$ read a growth of a leader $L$ that was open at its start, the growth
would be at the position of $E_2$ and $Y$ of the cycle of $L$ ([Lemma 2 of the
memoization proof](memoization.md#lemma-2-reading-an-open-growth)), and
$Y_{l'}$ would be a body evaluation of a rule of that cycle at the position of
a growth of the cycle in progress, in a context of growths. So $E_2$ reads no
growth open at its start. In the first case the same holds for $E_1$, whose
only body evaluation $Y_l$ the parser leaves at $p$ from the end of $w_l$ to
$e'_l$. There $E_1$ and $E_2$ meet the hypotheses of *Evaluated pairs of a
shared call*: every growth at their position open at the start of one is read
by neither, since a growth that one of them reads, open at the start of the
other, is still open at its start. So, outside cases (a) and (b) of the
covered keys on the path of the pair, no head holds an activation that starts
while $E_2$ is in progress (*Outer links inside a memoized link*), but
$\alpha_{l'}$ starts after $Y_{l'}$ ends, and so after $E_2$ ends, and the
first case stays open.

An evaluation around $E_2$ is still in progress when $\alpha_{l'}$ starts:
the frame $Z_{l'}$ of the caller of $\pi_{l'}$, in which the link $l'$ reaches
$e'_{l'}$ after leaving $Y_{l'}$ (*Calls that call no rule*). $Y_{l'}$ is the
last frame that the link leaves, so the link does not leave $Z_{l'}$, and
$e'_{l'}$, with $\alpha_{l'}$ in it, starts while $Z_{l'}$ is in progress. So
when a call makes $Z_{l'}$ and its evaluation $F_2$ is the later evaluation of
a pair that meets the hypotheses of *Evaluated pairs of a shared call*,
outside cases (a) and (b) of the covered keys on the path of that pair, no
head holds $\alpha_{l'}$ (*Outer links inside a memoized link*, with $F_2$ in
the place of $E_2$). This uses nothing of what $Z_{l'}$ evaluates after
$\pi_{l'}$, which need not repeat what $Z_l$ evaluates after $\pi_l$. $Z_{l'}$
is in progress at $E_2$, after $w_{\bar{m}}$ ends, and was not started inside
$w_{\bar{m}}$, so it either started after $w_{\bar{m}}$ ended or encloses
$w_{\bar{m}}$. In the first form $Z_{l'}$ is in progress when $\alpha_j$
starts, since it encloses $Y_{l'}$, which is $e'_j$ here. So, outside the
exception of *Restarts* for the growths that enclose $w_{\bar{m}}$, it lies in
the evaluation of $\hat{\kappa}$ from $\bar{s}$, and $\hat{\kappa}$ makes
the call of $Z_{l'}$ (the claim after *The last unit around the earlier
frame*). There what is not shown is that $w_{\bar{m}}$ shares that call with
$\hat{\kappa}$ and evaluates it, unanswered, at the same position, and that
neither evaluation is in a context of growths, and that the pair meets the
hypothesis on the growths open at their position; the argument above for $E_2$ does not apply
to $F_2$, since the parser does not leave $Z_{l'}$ at $p$ before $e'_{l'}$.
Inside that exception $Z_{l'}$ is a later step of a growth $G$ that encloses
$w_{\bar{m}}$, or an evaluation in such a step that follows no read of its
seed, as a later step of the leader that calls $Y$ at $\pi_{l'}$ after its
read of the seed. Then $G$, which encloses
$w_{\bar{m}}$, makes $Z_{l'}$ or the step that holds it, $\hat{\kappa}$ need
not make the call of $Z_{l'}$, and no pair is given. *Caller frames in a
later step* below shows instead that every evaluation of a key that holds
$\alpha_{l'}$ then starts at $p$. In
the second form $Z_{l'}$ holds $w_{\bar{m}}$, and with it $E_1$, as well as
$E_2$, and no such pair is given.

The rule of $Z_{l'}$ may read variables: a unit and its continuation reach a
shared call through calls of rules that read variables, and only the shared
call itself reads none ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost)). The reduction above
holds for such a frame, since it uses of $Z_{l'}$ only that it encloses $E_2$
and is in progress when $\alpha_{l'}$ starts, and nothing that it evaluates
after $\pi_{l'}$, where the values of its variables could take it elsewhere
than $Z_l$. But a call of a rule that reads a variable, or that the parse does
not memoize at the call, is shared with no unit, so for such a $Z_{l'}$ the
route through the call that makes it is closed, not merely unproved. Every
evaluation that encloses $Z_{l'}$ is in progress when $\alpha_{l'}$ starts
too, so the reduction holds with $Z_{l'}$ replaced by any of them. The route
may then take $F_2$ as the evaluation of any call that encloses $Z_{l'}$, or
makes it, and whose rule reads no variable and is memoized at the call, such
as the innermost one, and the two forms above apply to the evaluation of that
call in place of $Z_{l'}$; when no such call exists, no pair is given. Neither
form is shown for such a call.

**Shared caller frames.** In the first form above, outside the exception of
*Restarts*, let $Z_l$ lie inside $w_{\bar{m}}$, and let the calls that make
$Z_l$ and $Z_{l'}$ be calls with which $w_{\bar{m}}$ and $\hat{\kappa}$
start where they evaluate $c$, so that the bodies of those calls evaluate $c$
before anything else that examines the input. Let the rule $Z$ of the two
frames, whose body holds the call place $\pi_l = \pi_{l'}$, read no
variable, be memoized at both calls and lie on no left-recursion cycle, and
let the two calls have the same level restriction, or none. Then the
evaluations $F_1$ and $F_2$ of those calls meet the hypotheses of *Evaluated
pairs of a shared call*, and, outside cases (a) and (b) of the covered keys
on the path from $\mathcal{R}(F_1)$ down to the call that corresponds to the
evaluation that holds $\alpha_{l'}$, no head holds $\alpha_{l'}$.

*Proof.* The two calls are of one rule with one level restriction, of a rule
that reads no variable and that the parse memoizes at each of them, and
$w_{\bar{m}}$ and $\hat{\kappa}$ evaluate each before anything else that
examines the input, where they evaluate $c$; so they share the call of $Z$
([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost)), and the condition
there on the growths of a cycle does not apply, since $Z$ lies on none.
$w_{\bar{m}}$ is a committing unit and $\hat{\kappa}$ its continuation, as
in the application above. The body of each call evaluates $c$ before it
examines the input, so $F_1$ starts where $E_1$ starts and $F_2$ where $E_2$
starts, which is the same position (case 2). $F_1$ lies inside
$w_{\bar{m}}$, since $Z_l$ does, and $\hat{\kappa}$ makes $F_2$ (the first
form). The memo answers neither call, since $F_1$ holds the body evaluation
$Z_l$ and $F_2$ the body evaluation $Z_{l'}$, and a call that the memo
answers holds no evaluation. Neither is in a context of growths, which only a
body evaluation of a rule of a left-recursion cycle can be in. Of the
growths open at its start, a call reads only those of the cycle of its rule
at its position ([Lemma 2 of the
memoization proof](memoization.md#lemma-2-reading-an-open-growth)), so
neither call reads a growth open at its start. Nor does one read a growth
open at the start of the other: $F_2$ starts after $F_1$ ends, so a growth
open at the start of $F_1$ that $F_2$ reads is in progress from the start of
$F_1$ to that read and so open at the start of $F_2$, so that $F_2$ would read a growth open at its start, and a growth open at
the start of $F_2$ that $F_1$ reads was started before that read and is in
progress after $F_1$ ends, so it was not started inside $F_1$ and is open at
the start of $F_1$, so that $F_1$ would read a growth open at its start. So every growth at their position that is open at the
start of one is read by neither. These are the hypotheses of
*Evaluated pairs of a shared call*, and the reduction above, with $F_2$ in the
place of $E_2$, gives the rest. $\square$

Not shown in the first form are the frames $Z_l$ that enclose $w_{\bar{m}}$,
the calls of $Z_l$ or $Z_{l'}$ that are not calls with which $w_{\bar{m}}$ or
$\hat{\kappa}$ start where they evaluate $c$, and a rule $Z$ that reads a
variable, is not memoized at one of the calls, lies on a left-recursion cycle
or is called with two level restrictions; for these the pair, if any, lies at
an enclosing call, which $w_{\bar{m}}$ need not evaluate. When the rule of
$Z_{l'}$ reads a variable, $Z_l$ and $Z_{l'}$ can be evaluations of keys of
different classes, since the variables can have different values in the unit
and in its continuation, and an enclosing call of a rule that reads no
variable can be one that only $\hat{\kappa}$ makes: in
`main = "[" (A "!" / B ws "y") / P`, with `A = {v := 1} Z "x"`,
`B = {v := 2} Z`, `Z = Y {v >= 0} ws`, `Y = "a"+ (ws ";")?`, `ws = [ ]*` and
`P` as in *Outer links across runs*, which the conditions accept, the unit
`A "!"` makes no call of `B`, and the evaluation of `Z` in `B` is the head of
its class and holds the call of `ws` after the call of `Y`. There the index
between the two outer links through that call of `Y` is free, so the grammar
shows the shape, not a case of placement 1; on `[` followed by $n$ `a`s, $n$
spaces and `y`, the compute of a recognition is $14n + 163$ on the closure
backend and $14n + 168$ on the bytecode backend for $n$ from $250$ to $2000$.

In those cases, then, the evaluation that holds $\alpha_{l'}$ can be the head
of its class, so the route through *Outer links inside a memoized link*
cannot close them. They go to the count of the activations held by heads:
$\alpha_{l'}$ is held by a head, at most one of each class, and is counted
among the activations that the head holds, which *Activations held by heads*
bounds only together with the bound on the heads and on the activations that
one head holds that step 1 of [Open Steps](#open-steps) leaves open. Nothing
more is shown for them here.

In the first case the parse evaluates three calls of $Y$ at the start of
$Y_{l'}$ that the memo answers none of: $E_1$, $e_j$ and $E_2$, in this order,
since $e_j$ holds $\alpha_{j-1}$, which starts after $\alpha_l$ and so after
$E_1$ ends. When the three have one variant and one binding level and none is
in a context of growths, which holds of $E_1$ and $E_2$ above, $e_j$ is
evaluated after $E_1$ ends, so $E_1$ stored no entry that answers $e_j$, and
$E_2$ after $e_j$ ends, so $e_j$ stored none that answers $E_2$. $e_j$ is not
the first call of its variant at the position, so it stores a result, which answers $E_2$
unless $e_j$ is inside a lookahead and $E_2$ outside them ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)). An entry of $E_1$ would
then answer $e_j$, inside a lookahead, so $E_1$ is the first call of its variant
at the position and stores no result. Otherwise their variants or binding levels differ,
or $e_j$ is in a context of growths.

**Caller frames in a later step.** In the first form above, let $Z_{l'}$ lie
in the exception of *Restarts* for the growths that enclose $w_{\bar{m}}$: let
it be a later step $S$ of such a growth $G$, or an evaluation in such a step
$S$ that follows no read of the seed of $S$. Then every evaluation of a key
that holds $\alpha_{l'}$ lies inside $Z_{l'}$, starts after $Y_{l'}$ ends and
starts at $p$. So no head that starts before $p$ holds $\alpha_{l'}$, and
when no call of a rule that the parse memoizes is evaluated on the way from
the end of $Y_{l'}$ to $e'_{l'}$, no evaluation of a key holds it.

*Proof.* $Z_{l'}$ encloses $Y_{l'}$, which is $e'_j$ here and holds
$\alpha_j$, so $\alpha_j$ lies in $S$, and $\alpha_{l-1}$, inside
$w_{\bar{m}}$, in the earlier step of $G$ that holds $w_{\bar{m}}$. As in the paragraph
on the case $e'_j = Y_{l'}$ after *Frames around the frame of an outer
link*, $G$ is then
the innermost evaluation that contains $\alpha_{l-1}$ and $\alpha_j$, the
second case of *Repeats inside a growth* of Lemma 5 does not arise for
$\alpha_j$, since the sequence of the activations of $\sigma$ at $q$ holds
it, and in the first case $\alpha_j$ follows a read $\rho$ of the seed of
$S$. $Z_{l'}$ contains $\rho$. This is plain when $Z_{l'}$ is $S$. Otherwise
$Z_{l'}$ follows no read of the seed of $S$. Let $M$ be the innermost
evaluation that contains $\rho$ and $\alpha_j$, which holds $\rho$ in a
child $c$ before the child that holds $\alpha_j$, as in the definition of
the restarts. $M$ and $Z_{l'}$ both contain $\alpha_j$, so one contains the
other. If $Z_{l'}$ did not contain $\rho$, it would not contain $M$, so it
would lie inside the child of $M$ that holds $\alpha_j$, which starts after
$c$ ends and so after $\rho$ ends, and $Z_{l'}$ would follow $\rho$.

Let $B$ be an evaluation of a key that holds $\alpha_{l'}$. $B$ and $Z_{l'}$
both contain $\alpha_{l'}$, so one contains the other. If $B$ contains $S$
and is not $S$, then $S$, a body evaluation of the leader of $G$, lies inside
the call that grows $G$, a call of a rule that the parse memoizes, between
$B$ and $\alpha_{l'}$ (*Own regions*), so $\alpha_{l'}$ lies outside the own
region of $B$. If $B$ is $S$, it is the body evaluation of a leader, which is
no evaluation of a key. If $B$ lies in $S$ and contains $Z_{l'}$, it contains
$\rho$, and the call that makes it reads, through $\rho$, the growth $G$,
which is open at its start, as in the paragraph after *Evaluated pairs of a
shared call*; so it is no evaluation of a key. So $B$ lies inside $Z_{l'}$
and is not $Z_{l'}$. $Y_{l'}$ is the body evaluation of the call at
$\pi_{l'}$, which lies in the body of the rule of $Z_{l'}$, so no body
evaluation lies between $Z_{l'}$ and $Y_{l'}$, and $B$, which lies inside
$Z_{l'}$ and is not $Z_{l'}$, contains $Y_{l'}$ only as $Y_{l'}$ itself. $Y_{l'}$
does not contain $\alpha_{l'}$, which starts after the link $l'$ leaves
$Y_{l'}$ (*Calls that call no rule*). So $B$ does not contain $Y_{l'}$, and
$Y_{l'}$ does not contain $B$, which is in progress at $\alpha_{l'}$; since
$B$ is in progress after $Y_{l'}$ ends, it starts after $Y_{l'}$ ends, and
so after $w_{l'}$ ends, since $Y_{l'}$ encloses $w_{l'}$. From the end of
$w_{l'}$ to the start of $\alpha_{l'}$ the parser starts no evaluation
before $p$ (*Growths around a leaf link*, under the hypothesis of *Outer
links across runs*), so $B$ starts at $p$ or later. $B$ and $e'_{l'}$ both
contain $\alpha_{l'}$, and $B$ does not lie inside $e'_{l'}$, since the only
body evaluation inside $e'_{l'}$ is that of $r$ or its twin, which calls no
rule and which the parse does not memoize at its calls. So $B$ contains
$e'_{l'}$, which starts at $p$ (*Calls that call no rule*), and $B$ starts at
$p$. Then $B$ is reached on the way from the end of $Y_{l'}$ to $e'_{l'}$
through a call of a rule that the parse memoizes, and when no such call is
evaluated there, there is no $B$. $\square$

So in this case $\alpha_{l'}$ is held by no head, or by one of the heads that
start at $p$, which are of the classes at $p$, one head each, as for the
leaf links in the paragraph after *Evaluated pairs of a shared call*. An
activation that no evaluation of a key holds is held by no memoized
evaluation, so a growth step or an evaluation in a context of growths owns
it (*Owners*), and Lemma 5 counts it. The claim uses of $Z_{l'}$ only that it
lies in the exception and encloses $Y_{l'}$; it gives no pair of a shared
call, and what a head that starts at $p$ holds is the bound on the
activations that one head holds that step 1 of [Open Steps](#open-steps)
leaves open. No grammar that the conditions accept is given here in which
$Z_{l'}$ lies in that exception.

**Calls of scan-only rules.** Let the rule $r$ of the links $i$ to $k$ be a
scan-only rule ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost)) that is not memoized
at its calls, in a parse with the records of scan sites, and let every link
evaluate $r$ at one position $p$, as in the claim above, but without its bound
on the parser between them. Then the activations of the links are at most a
constant beyond $T$ for each link. By [Lemma 6 of the Run Sites
proof](run-sites.md#lemma-6-walks-again-from-one-start), all but a constant
number of the evaluations of $r$ at $p$ make each walk of a scan site either in
fewer than $T + 1$ iterations or by taking a record after its first $T$
iterations, so each of them makes at most $w(T + 1)$ activations and two steps
for each record, a constant, which is charged to the call that the link
evaluates and Lemma 2 counts; each of the others makes the activations of the first evaluation at
$p$ again, with the excess of Lemma 5 of that proof, and there are a constant
number of them at each position. So the links through the calls of $r$ at one
position restart a constant number of activations each, plus a constant number
of whole evaluations at that position, which is what the claim above gives for
a rule that calls no rule, and the call is shared in the sense of condition 2.

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
scanning source other than a bounded scan. Let $T$ follow, in each dropped step
of a growth on the path from $X$ to $T$, a read of the seed of that growth that
succeeds, or a call whose result includes one, and let two evaluations at one
position of a call that a unit and a continuation share, or that the
comparison of a decided unit passes alike, end at one position. Then $A$ is in
$E_t$, except in the cases that the end of the proof leaves open. So when the
evaluations of $Z$ nest without a bound through $t$, each in the call $t$ of
the one around it, all of those calls ending at one position, each holding a
successful activation of such a source, and each dropped step of a growth
between them holding the inner one after a read of its seed, $t$ nests at one end: calls that start at that position nest
only through left recursion, which the growths bound, so some call $X$ starts
before the position and holds another, $T$, that does too, and the claim gives
$Y$ in $E_t$. A nesting through a part that a dropped step evaluates at the
position of its growth before a read of its seed is left open.

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
scanning sources examined input, or is a dropped step. When it is no dropped
step, it is a unit that condition 2 checks, and it is committing. When it is a
dropped step of a growth $G$ whose seed ends at $e$, $T$ follows a read of the
seed in it, so it starts at $e$ or after it, and *Dropped steps*, with the
activation of $T$ in the place of $\alpha$ and the end of $X$ in the place of
$\alpha'$, gives a unit in its place: the step itself, which condition 2
checks as a unit that starts at $e$, when it fails, or the first evaluation
$w'$ below it on the path that fails or is the expression of a lookahead,
which starts at or before $e$, when it succeeds, or the unit that the claim
gives again for a dropped step $w'$. Let $w$ be that unit from here on; it
starts before $p_0$, since $T$ starts at $e$ or after it and before $p_0$. This is why the definition needs no
case for the units that can fail only before their scanning sources examine
input: no such unit lies on the path. Let $s < p_0$ be the start of $w$. The
evaluations above $w$ on the path, up to $X$, succeed and are kept, so after
$w$ the parser evaluates from $s$ a continuation $\kappa$ of $w$, through the
ends of the enclosing expressions and, at the end of a rule body, through the
call of the rule, and ends $X$ at $p_0$, returning to no position before $s$
in between apart from the exception of the restarts for the growths on the
path from $X$ to $w$ (*Restarts*), whose later steps $\kappa$ goes on through
at the reads of their seeds. That $X$ ends at $p_0$ through those reads is
not shown when a later kept step of such a growth gives its result through an
alternative whose every read of the seed is abandoned, as the second step of
`E` does in
`E = E "x" / &(E "b") "a" "b" "c" / "a"` on `abc`. For the unit that a dropped step of $G$ gives,
*Dropped steps* gives the same, with the continuation that it names, which
goes on from $e$ after $G$ ends. $\kappa$ consumes the input from $s$ to $p_0$, so it
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
the bodies inside it through their ends. For the unit that a dropped step of
$G$ gives, $S$ ends at $p_0$, after the result of the step, which ends at or
before $e < p_0$, so $\kappa$ evaluates $S$ after $G$ ends, and $K$ is the call
that starts $G$ or a call above it. $K$ is kept and contains $S$ in its
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

Four points of this proof are open. That $X$ ends at $p_0$ through the reads
of the seeds in the later steps of the growths between $X$ and $w$, and so
through $\kappa$, is not shown when a later kept step gives its result through
an alternative whose every read of the seed is abandoned, as inside a
lookahead. The hypothesis that the alike calls end
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
$p_0$ grows ends beyond $p_0$. Second, $T$ must follow a read of the seed in
each dropped step of a growth on the path from $X$ to $T$, and that path lies
inside growths: the claim allows the kept growth steps on it and those
dropped steps, but a dropped step that holds $T$ in a part that it evaluates
at the position of its growth is not excluded.
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
not memoize, which the memo never answers (in a Document as *Transient calls
of a Document* in Lemma 3 reads it). An activation owned by a memoized
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
growth open at its start. A memoized evaluation is one, since, of the growths
open at its start, a call reads only those of the cycle of its rule at its
position ([Lemma 2 of the memoization
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
skips. In a parse that keeps records ([Setting](#setting)), a walk
that takes a record also evaluates none of the iterations that the record
answers. By (R1) it leaves what the walk that evaluated them would leave,
with the same end, outcome, values, recovered errors and environment, and
by (I1), applied to the iterations that this walk would evaluate, that is
what the walk of $\mathcal{R}$ leaves in that state; so the induction goes
on past the walk, and the parse evaluates what
$\mathcal{R}(x)$ evaluates except, besides the three kinds above, inside
the iterations that a record answers. In a
[Document](../runtime/documents.md) parse a walk that resumes a run
reuses recorded iterations, each an activation that makes no call, and
evaluates nothing inside them. By [Lemma 3 of the Documents
proof](documents.md#lemma-3-resumed-runs) and its proof, under the invariant of the
memoization theorem for the evaluations that completed before it in this
parse and in the parse that stored the record, which the induction of the
[theorem of that
proof](documents.md#theorem-document-parses-equal-fresh-parses) over the
parses of the Document gives, with the walks that took records of sites in
either parse carried past by (R1) as above, the walk visits the iterations that the
walk of $\mathcal{R}$ visits, in the same order and with the same
outcomes, each starting where the one before ended, and records into the
enclosing evaluation the same farthest failure and expected items and an
examined range that contains theirs, as (I2) of the memoization theorem
asks of any evaluation; only the decisions to reuse an entry or a record read a range,
and the induction here follows outcomes. Its element reads no variable
and no capture from outside. So the induction
goes on past each reused iteration, and the parse evaluates what
$\mathcal{R}(x)$ evaluates except, besides the four kinds above, inside
the iterations that a resumed walk reuses. A reused iteration is the
evaluation of the parse whose counterpart is the iteration of
$\mathcal{R}$ that it stands for. By [Lemma 1 of the memoization
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
activation, in a parse that keeps records, apart from those inside an
iteration of a walk of a site that keeps records, and, in a
[Document](../runtime/documents.md) parse, apart from those inside an
iteration of a walk of a repetition, in its rule or in the twin, at which a walk of the parse resumes a run: each is an evaluation of the same expression at the same
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
too. In a parse that keeps records, no growth is read in $\mathcal{R}(M)$
inside an iteration that a record answers, since no call of a rule of a
cycle is made there (*Growths lie outside the recorded walks*, which
follows Lemma 17). In a Document parse, no growth open at the start of $M$
is read in $\mathcal{R}(M)$ inside an iteration that a resumed walk
reuses: the growth is open at the start of that iteration too, which reads
none open at its start in $\mathcal{R}$ ([Lemma
4](documents.md#lemma-4-recorded-iterations-read-no-open-growth) of the
Documents proof). An alternative that the parse skips by the next character reads no growth
in $\mathcal{R}$: the only calls that it makes there are of rules that are not
leaders, on the way to a terminal that fails at once (*Counterparts*). So their counterparts in $\mathcal{R}$ take the
same steps up to the variants of their calls. Of two corresponding calls, one is
of a rule and the other of the same rule or of its twin, so the parse memoizes
both or neither. In an own region every call is of a rule that the parse does
not memoize, which it evaluates (in a Document as *Transient calls of a
Document* in Lemma 3 reads it), so the own regions of $M$ and $M_0$ are the
counterparts of the parts of $\mathcal{R}(M)$ and $\mathcal{R}(M_0)$ outside
their calls of rules that the parse memoizes and outside the alternatives that
the parse skips, and those parts take the same steps. An alternative that the
parse skips in one region and evaluates in the other fails in both at the same
position, and its evaluation in $\mathcal{R}$ holds no activation
(*Counterparts*). In a parse that keeps records, the two own regions are
those parts outside the iterations that a record answers as well
(*Counterparts*). An evaluation in the part of $\mathcal{R}(M)$ that lies
in no iteration of a walk of a site that keeps records corresponds, in the same steps,
to one of $\mathcal{R}(M_0)$ that lies in none, since it is reached through
the same expressions, so a record answers neither, and the correspondence
above holds of the two. A walk of such a site that lies in no such iteration is one of
these evaluations, with the same success and end whether or not it takes a
record, by (R1). Inside the iterations the correspondence can fail: one of
the two walks can take a record where the other evaluates. In a Document
parse the two own regions are those parts outside the iterations that a
resumed walk reuses as well (*Counterparts*). An evaluation in the part of
$\mathcal{R}(M)$ that lies in no iteration of a walk of a repetition, in its rule or in the twin, at which a walk of the parse resumes a run corresponds, in the same steps, to
one of $\mathcal{R}(M_0)$ that lies in none, since it is reached through
the same expressions, so neither lies in an iteration that a resumed walk
reuses, and the correspondence above holds of the two. A walk of such a
repetition that lies in no such iteration is one of these evaluations,
with the same success and end whether or not it resumes, and its
iterations, reused or evaluated, are those of its counterpart, in order
([Lemma 3 of the Documents proof](documents.md#lemma-3-resumed-runs) and
its proof, through *Counterparts*), so they correspond one to one to the
iterations of the other walk, each an evaluation of the element at the same
position with the same success and end. Inside them the correspondence can
fail: one of the two walks can reuse an iteration that the other
evaluates, as when a head in lookahead state $0$ resumes a run whose
walk a later evaluation of its class in lookahead state $1$ evaluates, a
repetition resuming a record only outside a lookahead. $\square$

**Replayed activations.** An activation held by a memoized evaluation $M$
corresponds to an activation of the same source at the same position, which
succeeds when it does, held by the head $H$ of the class of $M$, and distinct
activations held by $M$ correspond to distinct ones. In a parse that keeps
records, this holds of the activations that are not iterations of a walk of
a site that keeps records and lie inside none; by the claim *Growths lie outside the
recorded walks*, which follows Lemma 17, every growth activation is one of
these. In a [Document](../runtime/documents.md) parse, it holds of those
among them that lie inside no iteration of a walk of a repetition, in its rule or in the twin, at which a walk of the parse resumes a run, the iterations of such walks
included.

*Proof.* The activation lies in the own region of $M$, outside the alternatives
that the parse skips in it or in the own region of $H$: the alternatives around
it hold an activation in $\mathcal{R}(M)$, and so in $\mathcal{R}(H)$, which
takes the same steps (*Counterparts*). The replays then give the activation at
the same place in the own region of $H$; in a parse that keeps records they
give it for an activation outside the iterations of the walks of the sites
that keep records, which is an evaluation that the replays do not leave out,
and in a Document parse for one inside no iteration of a walk of a
repetition, in its rule or in the twin, at which a walk of the parse resumes a run, which they do not
leave out either; an iteration of such a walk that lies in no such
iteration corresponds to one of the walk at its place in $H$, which
succeeds when it does, whether either is reused or evaluated. $\square$

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

*Proof.* A call evaluates only when no entry answers it, and a store keeps
the join of the entry that a lookup of its key finds and the new result
([Setting](#setting), [Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)), which answers every call
that either answered, a store in a context of growths included. An entry
stored outside a lookahead answers the calls in
both states, and one stored inside a lookahead those inside one, condition (L)
of the [memoization proof](memoization.md#setting), and a call outside
contexts of growths does not look at the leaders that it records. So no store
makes an entry that answers a
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
parse of a [Document](../runtime/documents.md), or reuses an iteration that
comes from an earlier parse (*Covered keys through resumed runs*), or (b) the parse answers a call
or step, or takes it in a run of steps, from what it keeps apart from the
table, (D) of the [memoization proof](memoization.md#setting), and $z$ lies
inside a lookahead that it observed, or (c) the
parse skips by the next character an alternative on the path, or (d), in a
parse that keeps records, a record answers an iteration on the path: the
walk of the parse whose counterpart evaluates that iteration took a record
in its place, or (e), in a Document parse, an iteration on the path is
one that a walk which resumes a run reuses. Case (c) does
not arise when $z$ holds an activation in $\mathcal{R}$. Where a claim below
leaves out cases (a) and (b), or supposes that neither arises, it does the
same with cases (d) and (e); *Heads in a parse that keeps records* below says where
case (d) arises, and where case (e) arises is part of the eighth part of
step 8 of [Open Steps](#open-steps).

*Proof.* By induction on the order in which the parse completes its
evaluations. Each call $z'$ that the path below reaches in place of $z$ has the
key of $z$ and reads no growth open at its start, so it takes the steps of $z$
in $\mathcal{R}$ (*Counterparts*); when $z$ holds an activation, so does $z'$,
and so does every alternative on each segment of the path, which encloses
$z'$ or a call whose evaluation in $\mathcal{R}$ contains the counterpart of
that activation, so the parse skips none of them (*Counterparts*). Outside cases (c) and (d), if the parse evaluates every call
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
current result and each lookahead that it observed with the same observation,
in the lookahead state of $y$ or a smaller one.
By the argument of [Lemma 4 of the memoization
proof](memoization.md#lemma-4-answering-from-a-dependent-result), which covers
an answer in a larger lookahead state and applies to a step as to a call,
$\mathcal{R}(y)$ and $\mathcal{R}(y')$ take the same steps outside the
lookaheads that $y'$ observed, and answer a call of a leader with a current
result in both or in neither, so outside case (b) the path goes on in
$\mathcal{R}(y')$ to a call $z'$ with the key of $z$. $z'$ reads no growth open
at its start: one that started inside $y'$ corresponds to one that started
inside $y$, and $z$ reads neither that nor one open at the start of $y$, which
$y$ finds open with the current result that $y'$ found. In both cases the induction hypothesis for
$y'$ gives an evaluation of the key of $z$ before $y'$ ended, and so before $x$
ends. What remains is case (a). In a parse that keeps records the path is
followed in the same way. Outside case (d) no record answers an iteration
on it, so the evaluations of $\mathcal{R}$ on it that lie in no call or step
that the parse answers have their counterparts in the parse
(*Counterparts*). And what the proof uses of $y'$ holds whether or not a
walk inside $y'$ took a record: the walk leaves what its iterations would
leave, (R1), and $y'$ reads the growths that it would read without the
record, since no call inside an iteration reads a growth (*Growths lie
outside the recorded walks*). In a Document parse that resumes runs the
same holds of case (e): outside it no reused iteration lies on the path,
and a walk inside $y'$ that resumed a run leaves what its iterations would
leave ([Lemma 3 of the Documents proof](documents.md#lemma-3-resumed-runs)),
and $y'$ reads the growths that it would read without the resumption, since
the reused iterations read no growth open at their start in $\mathcal{R}$
([Lemma 4](documents.md#lemma-4-recorded-iterations-read-no-open-growth)
of that proof). $\square$

**Steps with an empty seed.** Let a step of a growth $G$ of a leader $R$ at
$p$ after the first have an empty seed, so that condition 2 takes it as a unit
that starts at $p$, and let $c$ be a call that the step and its continuation
$\kappa$ share. Then the rule of $c$ is not on the cycle of $R$, and no
evaluation inside either evaluation of $c$ reads $G$.

*Proof.* $\kappa$ is what follows the calls that start $G$, which the parser
evaluates after it leaves the body of $R$ that the step evaluates, so a call
of a rule of the cycle in $\kappa$ is reached after leaving the body of a rule
of the cycle, and the two share no such call (case 2). A call that reads $G$
is a call at $p$ of a rule of the cycle made while $G$ is in progress ([Lemma 2
of the memoization proof](memoization.md#lemma-2-reading-an-open-growth)), so
$G$ is evaluating, at $p$, every rule on the way from $R$ to it, and all of
them are on the cycle ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost), the left calls). If it
were inside the evaluation of $c$ in the step, $c$ would be at $p$ and its rule
among them. Inside the evaluation of $c$ in $\kappa$, $G$ has ended. $\square$

So $G$, open at the start of the evaluation of $c$ in the step and not of the
one in $\kappa$, is read by neither. A read would not differ either: during a
dropped step the current result of its growth is the seed, the result that the
growth ends with.

**Reference links.** $\mathcal{R}$ is the instance of the parse that stores
nothing, so the restarts, *Direct activations* and *Repeats* hold of it, with
its own dropped steps and its own paths that enter a growth again. Let $\beta'$
be a successful activation of $\sigma$ at $q$ in $\mathcal{R}$, $\beta$ the one
before it, and $w$ and $\kappa$ as in the restarts, where $w$ is the unit that
*Dropped steps* gives when the abandoned evaluation is a dropped step, with no
dropped step at the position of its growth and no
path that enters the growth again of $\mathcal{R}$ leading from $w$ to
$\beta'$. Let $e^k$ be the first of the calls that $w$ and $\kappa$ share whose
evaluation inside $w$ holds a successful activation of $\sigma$ at $q$, let
$\beta_f$ be the first such activation in it. Then $\kappa$ evaluates the call of $e^k$ again at its position,
as $e^{k\prime}$, which takes the steps of $e^k$ up to the variants of its
calls, and $\beta'$ is the activation of $e^{k\prime}$ that corresponds to
$\beta_f$. The pair $(e^k, e^{k\prime})$ is the **reference link** of $\beta'$;
it joins $\beta'$ to $\beta_f$, which need not be $\beta$.

*Proof.* By *Repeats*, $\beta$ lies in a shared call, so $e^k$ exists.
$\mathcal{R}$ evaluates every call, and $\kappa$, every path of which that
examines the input begins with the shared calls, evaluates them in order, each
at the position where it ends in $w$ (case 2). The evaluation in $\kappa$ of a
shared call up to $e^k$ and the one inside $w$ take the same steps up to the
variants of their calls. Where neither reads a growth open at its start, this
is *Counterparts*. A call that reads a growth open at its start is at the
position of that growth and of a rule of its cycle ([Lemma 2 of the
memoization proof](memoization.md#lemma-2-reading-an-open-growth)), and two
such calls that a unit and its continuation share lie in the same growths of
the cycle, which have the same growing results at both (case 2), so the two
evaluations read the same results and take the same steps. When $w$ is a step
whose seed is empty, the growth of the step is open at the start of the
evaluation inside $w$ only, and neither reads it (*Steps with an empty
seed*). So a shared call before $e^k$ holds no successful activation of
$\sigma$ at $q$ in $\kappa$ either, and nor does $e^{k\prime}$ before the
activation that corresponds to $\beta_f$. Nothing that the parser evaluates between the end of $w$ and that
activation holds one either, since $\beta'$ is the first after $\beta$, which
lies in $w$. So $\beta'$ is that activation. $\square$

The claim needs no hypothesis on the growths open at the start of the shared
calls: of the growths open at its start, a call reads only those of its own
cycle at its position, and
sharing keeps the two calls of a rule of a cycle in the same growths of the
cycle.

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
growth open at its start. A growth that it reads and that was open at the
start of $e^k$ is at the position of $z^\circ$, which is then the position of
$e^k$, and of the cycle of the rule of $e^k$, since every call between them is
made there without consuming input ([Lemma 2 of the memoization
proof](memoization.md#lemma-2-reading-an-open-growth)); so it is open at the
start of $e^{k\prime}$ too (case 2), since the growth of a step whose seed is
empty, open at the start of $e^k$ alone, is read by no call inside it (*Steps
with an empty seed*). One that started inside $e^k$ corresponds
to one that started at the same place inside $e^{k\prime}$. $\mathcal{R}(Z)$,
taking the same steps as $z^\circ$, would read the corresponding growth,
which it does not, since the call of $Z$ reads no growth open at its
start. And $z^\circ$ ends before $e^{k\prime}$ starts, so before $\mathcal{R}(Z)$ starts. By the
heads in the reference parse, $Z$ is not the head of its class. $\square$

**Links lead to repeated evaluations.** Let the $i$-th link of $\sigma$ at $q$
be memoized, and let every growth at the position of the link that is open at
the start of $e_i$ or of $e'_i$ be open at the start of both or be read by
neither. If an evaluation $Z$ of a key holds $\alpha_i$, it is not the head of its
class, unless case (a) or (b) of the covered keys arises on the path from
$\mathcal{R}(e_i)$ down to the call that corresponds to $Z$. Case (a) arises
at an entry only at a call that is in a context of growths, or inside a lookahead while
the call of its class that corresponds to it on the way from $e'_i$ to $Z$ is
outside them, or whose corresponding call is in a context of growths or has the
other variant; so it does not arise when $e_i$ and $e'_i$ have one variant, the
lookahead state of $e_i$ is no greater than that of $e'_i$ and no call on
either way is in a context of growths. At an iteration that comes from an
earlier parse, case (a) arises only where case (e) does.

*Proof.* $e_i$ and $e'_i$ are evaluations of one call at one position, of a
rule that reads no variable, so their calls have keys of one class: the call
in a committing unit and the call in its continuation can differ in variant, one
evaluating the twin where its value is discarded and the other the rule. A
growth open at the start of both contains both, and so the innermost
evaluation $N$ that contains $\alpha_{i-1}$ and $\alpha_i$. When $N$ is no
growth (*Restarts*), it lies in one step of the growth, during which its
current result does not change. When $N$ is a growth, the two activations are
of the first case of *Repeats inside a growth* of Lemma 5, in different steps
of $N$. Every other growth open at the start of both contains $N$, and so
lies around one step of it as above. Neither call reads the growing result of
$N$, which is open at the start of both: of the growths open at its start, a
call reads only those of the cycle of its rule at its position
([Lemma 2 of the memoization
proof](memoization.md#lemma-2-reading-an-open-growth)), and a call of a rule
of that cycle that $w_i$ and $\kappa_i$ share is reached in $\kappa_i$ without
leaving the body of a rule of the cycle (case 2), so in the step of $N$ that
holds $w_i$, while $e'_i$ holds $\alpha_i$, which lies in a later step. A
growth open at the start of only one of them is read by neither. So the
two calls find the growths that they read open with the same current results.
When neither reads one,
$\mathcal{R}(e_i)$ and $\mathcal{R}(e'_i)$ take the same steps up to the
variants of their calls (*Counterparts*). Otherwise the rule of the link is a leader, which
has no twin, and they take the same steps by the argument of [Lemma 4 of the
memoization
proof](memoization.md#lemma-4-answering-from-a-dependent-result): the state
that their steps read differs at most in the lookahead state, which reaches no
decision of the parser (*Counterparts*), and the growths that they read, which
are open at both with the same current results. $Z$ holds $\alpha_i$, which lies in $e'_i$, and $e'_i$ is
made by a call of a rule that the parse memoizes, so $Z$ is $e'_i$ or is
evaluated inside it. So $Z$ is the counterpart of $\mathcal{R}(e'_i)$ or of a
call $z$ inside it, and $\mathcal{R}(e_i)$, or a call $z^\circ$ at the same
place in it, has a key of the class of $Z$. $Z$ is not $e'_i$ when the rule of
the link is a leader, since an evaluation of a key is of no leader, and $z$
reads no growth open at its start, since the call of $Z$ reads none.
$z^\circ$ reads none either: a growth open at its start that it reads was
open at the start of $e_i$, at the position of $e_i$ and of the cycle of its
rule, since every call between them is made there without consuming input, so
$e_i$ reads it, it is not the growth $N$ above, and so it is open at the start of
$e'_i$ with the same current result; or it started
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
at the position of the link open at the start of only one of them started
inside that evaluation (Lemma 2 of the memoization proof): on the way to $e_i$,
ending before $e'_i$, or on the way to $e'_i$ after $e_i$ ended. Case 2 rules
this out, apart from the growth of a step below: $e_i$ and $e'_i$ are evaluations of a call that $w_i$ and its
continuation share, and two such calls of a rule of a cycle lie in the same
growths of the cycle, so the growths of its cycle open at the start of $e_i$
are those open at the start of $e'_i$. A growth of another cycle open at the
start of only one of them is at the position of the link, so that evaluation
is in a context of growths, where case (a) of the covered keys arises. When
$w_i$ is a step of a growth $G$ after the first, $G$ is open at the start of
$e_i$ and not of $e'_i$, which starts after $G$ ends; $G$ is at the position of
the link only when the seed of the step is empty, and then neither evaluation
reads it (*Steps with an empty seed*), so the claim above applies.

So by the claim above every linked $\alpha_i$ that is held by a head follows a
link with case (a) or (b) of the covered keys on its path. A free $\alpha_i$ other than $\alpha_1$
is the counterpart of a successful activation of $\mathcal{R}$ at $q$. That
activation has no reference link when a dropped step at the position of its
growth or a path that enters a growth again of
$\mathcal{R}$ leads to it. When it has one, of a memoized
rule as every link here is, by *Reference links lead to repeated evaluations*
$\alpha_i$ is held by a head only with case (a) or (b) on its path.
When no link of $\sigma$ at $q$ has either case on its path and every free $\alpha_i$
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
$m < i$, or the position of one that case 5 or a dropped step at the position of its growth repeats. If $i$ is linked
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
one that case 5 or a dropped step at the position of its growth repeats, which the $\alpha_m$ leave out, and
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

**Evaluated pairs of a shared call.** Let a committing unit $w$ and its
continuation $\kappa$ share a call of a rule that reads no variable and that the
parse memoizes at both calls, let $E_1$ be the evaluation of the call inside $w$ and $E_2$ the
evaluation of the call that $\kappa$ makes at the same position (case 2),
where the memo answers neither and neither is in a context of growths, and let
every growth at their position that is open at the start of $E_1$ or of $E_2$
be open at the start of both or be read by neither. Then no evaluation of a key
that is $E_2$ or lies inside it and holds an activation at any depth is the
head of its class, outside cases (a) and (b) of the covered keys on the path
from $\mathcal{R}(E_1)$ down to the call that corresponds to it. No activation
of $\sigma$ at $q$ need lie in $E_1$ or $E_2$, and $E_1$ need not have stored a
result: a first evaluation may leave none
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)), and then
the memo answers neither call.

*Proof.* The proofs of *Links lead to repeated evaluations* and of *Nothing in a
memoized link is a head* use of the $i$-th link only what holds of $E_1$ and
$E_2$ in the place of $e_i$ and $e'_i$, with $N$ the innermost evaluation that
contains $E_1$ and $E_2$ in the place of the one that contains $\alpha_{i-1}$
and $\alpha_i$: the two are evaluations of one call at one position whose keys
are of one class; $E_2$ is evaluated after $w$ is abandoned, since $\kappa$ is
(*Repeats*), and so after $E_1$ ends; and when $N$ is a growth, $E_1$ and $E_2$
lie in different children of $N$, which are its steps, $E_2$ in a later one.
Only the activation that $Z$ holds, at
the same place in $\mathcal{R}(E_1)$, excludes case (c). $\square$

So a link is the case of such a pair where $E_1$ holds $\alpha_{i-1}$ and
$E_2$ holds $\alpha_i$, and the claims about outer links below need of a
memoized link only that it is an evaluated pair. The first evaluation of a
key that stores no result does not repeat: among the memoized evaluations of
a key, which the proof of Lemma 3 counts, the only one that stores none is
that of the first call of the variant at the position, so a key has at most
one, and the evaluations that it lets the memo make again are at most the
three of its key. An evaluation of a key in a context of growths, which
Lemma 3 does not count, is no memoized evaluation, and the evaluated pairs
above exclude it. The head of a class is its first evaluation, whether or not
it stores a result; a no-result evaluation only makes a later call of its key
evaluated instead of answered, and that evaluation is one of the three of its
key, and so of the six memoized evaluations of its class that *Activations
held by heads* allows.

So the body evaluations of $R$ that hold the $\alpha_i$ start at most at one
position for each free $i$, each link that *Links to heads* leaves
out, each memoized link whose two evaluations read a growth open
at their start, which *Memoized links keep positions* leaves out, and each
activation that case 5 or a dropped step at the position of its growth repeats. For a leaf link $j$ whose shared call is the first, so that
$w_j$ starts at $p_j$, a head that holds $\alpha_j$ starts at $p_j$ or before
it. The heads that start at $p_j$ are of the classes at $p_j$, one head each.
Those that start before $p_j$ enclose $w_j$: such a head $H$ contains $P_j$,
since both contain $\alpha_j$, $H$ is not $P_j$, as the parse does not
memoize $R$ at its calls, and $P_j$, which makes no call, contains no other
body evaluation; so $H$ started before $\alpha_j$. From the end of $w_j$ to
$\alpha_j$ the parser starts no evaluation before $p_j$ apart from the
exception of the restarts (*Restarts*, *Dropped steps*, and the first case of
*Repeats inside a growth* in Lemma 5 when $N_j$ is a growth, the second
leaving $\alpha_j$ out of the sequence). $H$ is no evaluation of the
exception. When $N_j$ is no growth, the exception lies in growths that do not
contain $\alpha_j$. When $N_j$ is a growth, $\alpha_j$ follows a read $\rho$
of the seed of its step; an evaluation in the exception that contains
$\alpha_j$ follows no read, so it contains $\rho$, as in *The last unit
around the earlier frame*. A body evaluation that contains $\rho$ is a later
step, the body evaluation of a leader, or lies in one and is made by a call
that reads, through $\rho$, the growth open at its start; either way it is no
evaluation of a key and no head. So $H$ started before
$w_j$ ended, and not inside $w_j$, since an evaluation started inside $w_j$ ends
inside it, while $P_j$ starts after $w_j$ ends. As $\alpha_j$ lies in the own
region of $H$, no call of a rule that the parse memoizes lies between $H$ and
$P_j$. For two such heads, $H$ holding $\alpha_j$ and $H'$ holding
$\alpha_{j'}$ with $j < j'$, $H'$ did not start during $\alpha_j$, which lies
in a call of $R$, which calls no rule; so either $H'$ started before
$\alpha_j$ and, being in progress after it, encloses it, or $H'$ started
after $\alpha_j$ ended. In the second case $H'$ encloses $w_{j'}$
and with it $\alpha_{j'-1}$, so $j' - 1 > j$, and the least $i > j$ with
$\alpha_i$ inside $H'$ has $\alpha_{i-1}$ before $H'$ and $i < j'$. That $i$ is
free, or linked by a leaf link whose unit $H'$ does not enclose, or linked by a
memoized link; in the last case, under the hypotheses of *Links lead to
repeated evaluations* and outside cases (a) and (b) of the covered keys, $H'$
is not $e'_i$ and does not lie inside it (*Nothing in a memoized link is a
head*), so it strictly encloses $e'_i$. A bound on these heads and a bound on
the activations that one head holds are part of step 1 of [Open
Steps](#open-steps).

**Heads in a parse that keeps records.** In a parse whose records are
exact, (R1) of the [Setting](#setting), *Answered calls*, *Steps with an
empty seed*, *Reference links* and *Links to heads* hold as stated, and
*Covered keys*, *Heads in the reference parse*, *Reference links lead to
repeated evaluations*, *Links lead to repeated evaluations*, *Nothing in a
memoized link is a head* and *Evaluated pairs of a shared call* hold with
case (d) of the covered keys beside cases (a) and (b). Case (d) arises only
as follows, where an iteration **of the kind** is an iteration of a walk of
a site that keeps records.

1. In *Covered keys*, only when $z$ lies, inside $\mathcal{R}(x)$, in an
   iteration of the kind.
2. In *Heads in the reference parse*, only when $z$ lies in $\mathcal{R}$
   in an iteration of the kind. In *Reference links lead to repeated
   evaluations*, only when $Z$ lies in an iteration of the kind, or $e^k$
   does in $\mathcal{R}$.
3. In *Links lead to repeated evaluations*, *Nothing in a memoized link is
   a head* and *Evaluated pairs of a shared call*, only when $Z$ lies,
   inside $e'_i$ or $E_2$, in an iteration of the kind. So it does not
   arise when the activation that $Z$ holds lies inside no iteration of
   the kind.

And *Memoized links keep positions* holds of an $\alpha_i$ that is no
iteration of the kind and lies inside none.

*Proof.* *Answered calls* is about the lookups and the stores of the memo,
and a walk that takes a record makes neither. *Steps with an empty seed*
is about a growth, its step and the calls that read it, and uses case 2
and Lemma 2 of the memoization proof alone. *Reference links* is a claim
about $\mathcal{R}$, which keeps no record and evaluates every walk in
full (the [two parses](memoization.md#setting) of the memoization proof);
the parse enters where a claim takes an activation of the parse as the
counterpart of one of $\mathcal{R}$, which *Counterparts* gives of a parse
that keeps records. The proof of *Covered keys* above treats case (d), and
the proofs of the five claims after it use of the parse, beside *Covered
keys*, *Counterparts*, *Restarts*, *Repeats* and *Answered calls*, only
which calls it evaluates, what they read and in what order. Where the
innermost evaluation that contains the two activations of a link is a
growth, *Links lead to repeated evaluations* also rests on the first case
of *Repeats inside a growth* of Lemma 5, which *Repeats inside a growth in
a parse that keeps records* below shows of such a parse. The argument of *Links to heads* is about the
growths open at the start of the two evaluations of a link, which lie
outside the iterations of the kind (*Growths lie outside the recorded
walks*), and it uses case 2, Lemma 2 of the memoization proof and those
claims.

The following is used below. By *Counterparts*, the evaluations of the
parse are the counterparts of the evaluations of $\mathcal{R}$ that lie in
no call or step that the parse answers, in no alternative that it skips and
in no iteration that a record answers, and the parse nests them as
$\mathcal{R}$ does. So when an iteration $\iota$ of a walk of
$\mathcal{R}$ contains the counterpart of an evaluation $Z$ of the parse,
$\iota$ is the counterpart of an iteration that the parse evaluates, of a
walk of the same site, which contains $Z$.

1\. Let a record answer an iteration $\iota$ on the path; its site keeps
records, since a walk of it took that record. The path is made of segments:
the first leads from $\mathcal{R}(x)$ down to $z$ or to the first call or
step $y$ that the parse does not evaluate, and each later one leads from
$\mathcal{R}(y')$ down to the call $z'$ that takes the place of $z$, or to
the next such $y$. If $\iota$ is on the first segment, it contains $z$ or
$y$, and so $z$, which $\mathcal{R}(y)$ contains. On a later segment,
$\mathcal{R}(y)$ and $\mathcal{R}(y')$ take the same steps on the way to
$z$ and to $z'$, up to the variants of their calls, by the proof of the
covered keys. So an iteration that contains $z'$ inside $\mathcal{R}(y')$
corresponds to an iteration of the same repetition, in the rule or in its
twin, that contains $z$ inside $\mathcal{R}(y)$, and $\mathcal{R}(y)$ lies
in the segment before. By induction over the segments, an iteration of a
walk of the site of $\iota$ contains $z$ inside $\mathcal{R}(x)$.

2\. The proof of *Heads in the reference parse* follows the path from the
evaluation where $\mathcal{R}$ begins down to $z$ and applies the covered
keys at the first call or step $y$ that the parse does not evaluate. A
record that answers an iteration before $y$ on that path answers one that
contains $z$, and one that arises in the covered keys contains $z$ inside
$\mathcal{R}(y)$, by 1. In *Reference links lead to repeated evaluations*
the claim is applied to the call $z^\circ$ at the place in $e^k$ where
$\mathcal{R}(Z)$ lies in $e^{k\prime}$. An iteration of the kind that
contains $z^\circ$ contains $e^k$ or lies inside it. One that lies inside
it corresponds, in the same steps, to an iteration of the same repetition
inside $e^{k\prime}$ that contains $\mathcal{R}(Z)$, and then $Z$ lies in
an iteration of the kind, by the paragraph above.

3\. In *Links lead to repeated evaluations* the covered keys are applied
with $e_i$ as $x$, which the parse evaluates, and with the call $z^\circ$
of $\mathcal{R}(e_i)$ that corresponds to $Z$. By 1, case (d) arises only
when an iteration of the kind contains $z^\circ$ inside $\mathcal{R}(e_i)$.
$\mathcal{R}(e_i)$ and $\mathcal{R}(e'_i)$ take the same steps up to the
variants of their calls, so an iteration of the same repetition contains
$\mathcal{R}(Z)$ inside $\mathcal{R}(e'_i)$, and $Z$ lies inside $e'_i$ in
an iteration of the kind, by the paragraph above. The activation that $Z$
holds lies inside $Z$. The other two claims rest on that proof, with $E_1$
and $E_2$ as $e_i$ and $e'_i$.

*Memoized links keep positions.* Let $\alpha_i$ be no iteration of the
kind and lie inside none, so that $P_i$, which contains it, lies inside
none. The proof follows the path of the covered keys from
$\mathcal{R}(e_i)$ to the body evaluation $\rho$ at the place of the
counterpart of $P_i$ in $\mathcal{R}(e'_i)$. By the argument of 1 and 3
with $\rho$ as $z$, no record answers an iteration on that path, since
$P_i$ would lie inside $e'_i$ in an iteration of the kind. So the parse
makes, before $e_i$ ends, a body evaluation $P$ of $R$ or of its twin at
$p_i$ whose counterpart takes the steps of $\rho$, as that proof shows.
$\rho$ holds an activation $\beta$ of $\sigma$ at $q$ at the place of the
counterpart of $\alpha_i$. $P$ makes no call, since $R$ calls none, and a
skipped alternative holds no activation, so the parse evaluates the
counterpart of $\beta$ in $P$ unless a record answers $\beta$ or an
iteration inside $\rho$ that contains it. That iteration would be of the
kind and would correspond, in the same steps, to the counterpart of
$\alpha_i$ or to an iteration that contains it, which the parse evaluates:
then $\alpha_i$ is an iteration of the kind or lies inside one. So $P$
holds a successful activation of $\sigma$ at $q$, and the rest of that
proof applies. $\square$

So for the activations that are no iterations of the kind and lie inside
none, which are those of *Replayed activations*, no link has case (d) on
its path, and the links that *Links to heads* leaves out are those with
case (a) or (b) as in a parse without records. A free activation among
them can still have case (d) on the path of its reference link, when the
earlier evaluation $e^k$ of that link lies in $\mathcal{R}$ in an
iteration that a record answers in the parse; *Covered keys through
records* below follows the path past that iteration. The claim does not
cover the claims on leaf links, nor the paragraph before it on the heads
that hold the $\alpha_j$ of leaf links, which uses an argument of *The
last unit around the earlier frame*; *Leaf links in a parse that keeps
records* and *Later leaf links in a parse that keeps records* below treat
those, other than *Calls of scan-only rules* and *Seed reads*. Nor does it
cover the claims that follow *Activations held by
heads*.

**Heads in a parse that resumes runs.** Call an iteration **resumable** in
a [Document](../runtime/documents.md) parse when it is an iteration of a
walk of a repetition, in its rule or in the twin, at which a walk of the
parse resumes a run. In a Document parse whose records of sites are
exact, (R1) of the [Setting](#setting), *Heads in a parse that keeps
records* holds as stated, and the claims that it shows with case (d) of
the covered keys hold with case (e) beside it. Where the innermost
evaluation that contains the two activations of a link is a growth,
*Links lead to repeated evaluations* and the claims that rest on it, from
*Links to heads* to *Evaluated pairs of a shared call*, rest on the first
case of *Repeats inside a growth* of Lemma 5, which *Repeats inside a
growth in a parse that resumes runs* below shows of such a parse. Case (e) arises
only as 1 to 3 of that claim state of case (d), with a resumable iteration
in place of an iteration of the kind. And *Memoized links keep positions*
holds of an $\alpha_i$ that is no resumable iteration and lies inside
none.

*Proof.* The proof of *Heads in a parse that keeps records* applies, with
*Restarts in a parse that resumes runs* for the claims of Lemma 4 that it
uses, and with the following in place of its paragraph on the
counterparts. By *Counterparts*, the evaluations of the parse are the
counterparts of the evaluations of $\mathcal{R}$ that lie in no call or
step that the parse answers, in no alternative that it skips, in no
iteration that a record answers and in no iteration that a resumed walk
reuses, other than such an iteration itself, which stands for its
counterpart, and the parse nests them as $\mathcal{R}$ does. A reused
iteration contains no evaluation of the parse other than itself. So when
an iteration $\iota$ of a walk of $\mathcal{R}$ contains the counterpart of
an evaluation $Z$ of the parse other than $\iota$'s own, $\iota$ is the
counterpart of an iteration that the parse evaluates, of a walk of the same
repetition, which contains $Z$. *Answered calls* holds as stated (*Restarts
in a parse that resumes runs*, 4), and *Links to heads* follows the growths
of the parse open at the start of the two evaluations of a link, which a
resumed walk does not change, since a reused iteration starts no growth.

In 1 to 3, a resumed walk reuses an iteration $\iota$ on the path, and
$\iota$ is resumable; the segments of 1 carry it, through the same steps
of $\mathcal{R}(y)$ and $\mathcal{R}(y')$, to an iteration of the same
repetition, in the rule or in its twin, which is resumable too, since the
class is one of repetitions and not of positions or walks. 2 and 3 then
give a resumable iteration that contains $e^k$ in $\mathcal{R}$ or lies
inside it, or one that contains $Z$, by the paragraph above. For
*Memoized links keep positions*, let $\alpha_i$ be no resumable iteration
and lie inside none. No resumed walk reuses an iteration on the path from
$\mathcal{R}(e_i)$ to $\rho$, since $P_i$ would lie inside $e'_i$ in a
resumable iteration. In $P$ the parse makes the counterpart of $\beta$,
unless a resumed walk reuses an iteration inside
$\rho$ that contains $\beta$ other than as $\beta$ itself; that iteration
would be resumable and would correspond, in the same steps, to an
iteration that contains $\alpha_i$. And $\beta$ itself is not reused,
since $\alpha_i$, an iteration of the same repetition, in the rule or in
its twin, would then be resumable. So $P$ evaluates $\beta$ and holds a
successful activation of $\sigma$ at $q$. $\square$

So for the activations that are no resumable iterations and lie inside
none, which are those of *Replayed activations* in a Document parse, no
link has case (e) on its path. A free activation among them can still
have case (e) on the path of its reference link, when the earlier
evaluation $e^k$ of that link lies in $\mathcal{R}$ in an iteration that a
resumed walk reuses. *Covered keys through resumed runs* below follows
the path past that iteration, as *Covered keys through records* follows
it past a record, and such an activation is held by no head unless case
(a) or (b) arises on the path followed through the records and the
resumed runs, an iteration that comes from an earlier parse being case
(a). The claim does not cover
the claims on leaf links and on the scans of a parse that resumes runs.

**Covered keys through records.** This claim uses, beyond (R1), how the
records of a run site and of a skip-run site are made
([Skip Runs](../runtime/parsing.md#skip-runs),
[Run Sites](../runtime/parsing.md#run-sites), and the proofs of
[Lemma 1](skip-runs.md#lemma-1-exactness) of the Skip Runs proof and of
[Lemma 1](run-sites.md#lemma-1-exactness) of the Run Sites proof, which
read them so):

- (M1) a record of a position is made by a walk of the site in the same
  parse, when that walk ends, and that walk evaluated an iteration at the
  position; when the iteration consumed input, the walk had, at the
  position where the iteration ended, a record that it found there or made
  there itself, and the record of the first position stands for that
  iteration followed by what the second record stands for; otherwise the
  record stands for that iteration alone;
- (M2) a walk finds a record only at the position of the record, and, at a
  site that is not free of the environment, only in an environment in
  which the variables that the element reads have the values with which
  the iteration of (M1) began; where the walk of (M1) found the second
  record, it found it in the environment that its iteration left. A walk
  may find fewer records than that. By
  [Run Sites](../runtime/parsing.md#run-sites) and [Record
  Kinds](run-sites.md#record-kinds) in the Run Sites proof, the
  records of the walks inside lookaheads are kept apart from those of the
  walks outside them, a position has one record at a time for one
  lookahead state and one key, which at a site that is not free of the
  environment is the environment as an object and not the values in it,
  and a walk whose iterations have values takes no record that a walk
  without values made, which holds no elements: it records the position
  again, and its record takes the place of that one. A record without
  values never takes the place of one with them, so a key holds a record
  without values only until a walk that builds values records it, and
  never after. A walk whose iterations have no values takes a record of
  either kind.

An implementation that makes the records of such a site in another way
needs the claim shown anew. A record of any other site answers no
iteration that contains a call: a candidate site that is no run site
repeats a single character. In a parse whose records are exact and made
so:

1. *Answered iterations were evaluated.* Let a walk $W$ of the parse, of a
   run site or a skip-run site $\tau$, take a record, and let $\iota$ be
   one of the iterations that $\mathcal{R}(W)$ evaluates from the position
   of that record on. Then, before $W$ took the record, the parse evaluated
   an iteration $\iota_0$ of a walk of $\tau$ at the position of $\iota$,
   and $\mathcal{R}(\iota_0)$ and $\iota$ take the same steps up to the
   variants of their calls. So a call inside $\iota$ of a rule that the
   parse memoizes corresponds to a call at the same place inside
   $\mathcal{R}(\iota_0)$ with a key of the same class, which holds an
   activation at any depth when the first does, and neither reads a growth
   open at its start.
2. *Covered keys.* Let $x$ be a call or a step of a growth of the parse, or
   an iteration that the parse evaluates of a walk of a run site or of a
   skip-run site, and let $z$ be as in *Covered keys*: $\mathcal{R}(x)$
   when $x$ is a call, or a call inside $\mathcal{R}(x)$, of a rule that
   the parse memoizes other than a leader, that reads no growth open at its
   start. Then the parse makes an evaluation of a key of the class of $z$
   before $x$ ends, unless case (a), (b) or (c) of the covered keys arises
   on the path from $\mathcal{R}(x)$ down to $z$, **followed through the
   records**: followed into earlier evaluations as in *Covered keys* and,
   at an iteration $\iota$ that a record answers, into the iteration
   $\mathcal{R}(\iota_0)$ of 1. Case (c) does not arise when $z$ holds an
   activation at any depth.
3. *Heads.* *Heads in the reference parse* and *Reference links lead to
   repeated evaluations* hold outside cases (a) and (b) on their paths
   followed through the records, with no case (d). So a free activation
   whose reference link has case (d) on its path is held by no head,
   unless case (a) or (b) arises on that path followed through the records.

*Proof.* 1\. An iteration of $\mathcal{R}$ at a position $r$, of the
element of $\tau$ in its rule or in the twin, takes steps that depend only
on $r$, on the values of the variables that the element reads and on the
input, up to the variants of its calls: inside the element only the
captures of the iteration are visible
([Predicates](../language/predicates.md)), no call inside the iteration
reads a growth (*Growths lie outside the recorded walks*, and
[Lemma 2 of the memoization
proof](memoization.md#lemma-2-reading-an-open-growth)), the lookahead
state reaches no decision of the parser, and a body and its twin take the
same steps up to the variants of their calls (*Counterparts*). When
$\tau$ is free of the environment its element reads no variable.

Let $W$ take its record at $s$, and let $\iota_0', \iota_1', \ldots$ be
the iterations that $\mathcal{R}(W)$ evaluates from $s$ on, at
$s = r_0 < r_1 < \cdots$; each but the last consumes input, since an
iteration that fails or consumes none ends its walk. By induction on $t$:
before $W$ took its record the parse had a record of $\tau$ at $r_t$
whose iteration of (M1), $\iota_0$, began with the values with which
$\iota_t'$ begins, in the variables that the element reads. For $t = 0$
the record is the one that $W$ took: the iterations that $W$ evaluated
before $s$ are the counterparts of those of $\mathcal{R}(W)$ before $s$
and leave what they leave (*Counterparts*), so $\iota_0'$ begins with the
values of the environment in which $W$ found the record, which by (M2)
are those with which the iteration of the record began. Given such a
record at $r_t$, its iteration $\iota_0$ was evaluated at $r_t$ by a walk
of $\tau$ in the parse before the record was made, (M1), and
$\mathcal{R}(\iota_0)$ is an iteration of $\mathcal{R}$ at $r_t$ that
begins with the values with which $\iota_t'$ begins, so by the paragraph
above $\mathcal{R}(\iota_0)$ and $\iota_t'$ take the same steps up to
the variants of their calls, which is the claim for $\iota = \iota_t'$.
They have the same success and end, $r_{t+1}$ when they consume input,
and leave the same values in those variables, and $\iota_0$ has the
success and the end of $\mathcal{R}(\iota_0)$ and leaves what it leaves
(*Counterparts*). So when $\iota_t'$ is not the last, $\iota_0$ consumed
input and ended at $r_{t+1}$, and by (M1) its walk had a record at
$r_{t+1}$, made no later than the record at $r_t$. If the walk made that
record itself, the iteration of the record is the next iteration of the
walk, which began with the values that $\iota_0$ left. If it found the
record, it found it in the environment that $\iota_0$ left, whose values
are by (M2) those with which the iteration of that record began. Either
way that iteration began with the values with which $\iota_{t+1}'$
begins. A call at the same place
in two evaluations that take the same steps up to variants has keys of
one class in both (*Counterparts*), and holds an activation at any depth
in both or in neither, since the bodies of both take the same steps. No
call inside an iteration of $\mathcal{R}$ of a run site or a skip-run
site reads a growth, as above.

2\. By induction on the order in which the parse completes its
evaluations, as in *Covered keys*. Each call $z'$ that the path reaches in
place of $z$ has a key of the class of $z$ and reads no growth open at its
start, so it takes the steps of $z$ in $\mathcal{R}$ up to the variants of
its calls (*Counterparts*) and holds an activation when $z$ does. The path
is made of segments, each from $\mathcal{R}(v)$ down to such a $z'$,
where $v$ is $x$ or an evaluation of the parse, a call, a step or an
iteration, that ended before $x$ ends. On a segment, take the first
evaluation of $\mathcal{R}$ that has no counterpart in the parse. If
there is none, the parse evaluates the counterpart of $z'$ inside $v$,
which is an evaluation of a key of the class of $z$, as in *Covered keys*,
made before $x$ ends. If it lies in an alternative that the parse skips,
case (c) arises; it does not when $z$ holds an activation, since the
alternative then holds one in $\mathcal{R}$ (*Counterparts*). If it is a
call or a step $y$ that the parse answers, the proof of *Covered keys*
gives, outside cases (a) and (b), an evaluation $y'$ of the parse that
ended before $y$ was answered and a call $z''$ in $\mathcal{R}(y')$, or
$\mathcal{R}(y')$ itself, with the key of $z'$, which reads no growth
open at its start; the next segment leads from $\mathcal{R}(y')$ to
$z''$. Otherwise, by *Counterparts*, it is an iteration $\iota$ that a
record answers: the walk $W$ of the parse whose counterpart evaluates
$\iota$ is evaluated inside $v$, since nothing before $\iota$ on the
segment lacks a counterpart, and it took a record at or before the
position of $\iota$, before $v$ ended. $\iota$ contains the call $z'$,
so its site is a run site or a skip-run site, and 1 gives the iteration
$\iota_0$, evaluated before $W$ took the record and so ended before $x$
ends, and the call $z''$ at the place of $z'$ in
$\mathcal{R}(\iota_0)$, with a key of the class of $z$, which reads no
growth open at its start; the next segment leads from
$\mathcal{R}(\iota_0)$ to $z''$. In the last two cases the evaluation
$y'$ or $\iota_0$ completed before $x$ does, and the induction
hypothesis for it and $z''$ gives an evaluation of a key of the class of
$z$ before it ended, so before $x$ ends, unless case (a), (b) or (c)
arises further on the path.

3\. In the proof of *Heads in the reference parse*, follow the path from
the evaluation where $\mathcal{R}$ begins down to $z$, which holds an
activation, so that the parse skips no alternative on it. If every
evaluation on it has its counterpart, the parse evaluates $z$ before $Z$
starts, as there. If the first without one is a call or step $y$ that the
parse answers, the parse answers it before $Z$ starts, as there, and 2
with $x = y$ gives an evaluation of a key of the class of $z$ before $y$
was answered. Otherwise it is an iteration $\iota$ that a record answers,
of a walk $W$ that the parse evaluates and that took the record at or
before the position of $\iota$. $\mathcal{R}(W)$ does not contain
$\mathcal{R}(Z)$: $z$ lies in $\iota$ and ends before $\mathcal{R}(Z)$
starts, so inside $\mathcal{R}(W)$ the evaluation $\mathcal{R}(Z)$
would lie in $\iota$ or in an iteration after it, each of which a record
answers, and an evaluation of $\mathcal{R}$ inside one is the counterpart
of none of the parse (*Heads in a parse that keeps records*). And
$\mathcal{R}(Z)$, which starts after $\mathcal{R}(W)$ does, does not
contain it. So $\mathcal{R}(W)$ ends before $\mathcal{R}(Z)$ starts, and
$W$ took its record before $Z$ started. By 1 the parse evaluated
$\iota_0$ before that, and by 2 with $x = \iota_0$ and the call at the
place of $z$ in $\mathcal{R}(\iota_0)$ it made an evaluation of a key of
the class of $z$ before $\iota_0$ ended. In each case $Z$ is not the
first of its class. The proof of *Reference links lead to repeated
evaluations* applies *Heads in the reference parse* to the call $z^\circ$
of $e^k$, which holds an activation, reads no growth open at its start and
ends before $\mathcal{R}(Z)$ starts, and uses nothing else of the parse.
$\square$

The claim counts nothing, and it changes no path of a link: *Heads in a
parse that keeps records* shows that no link of an activation of
*Replayed activations* has case (d). Where case (a) or (b) arises on a
path followed through the records is not examined here beyond what
*Covered keys* states of them: the statement of *Links lead to repeated
evaluations* on where case (a) arises is of the paths of links and is not
extended to the paths followed through the records.

**Covered keys through resumed runs.** This claim uses, beyond
[Lemma 3](documents.md#lemma-3-resumed-runs) and [Lemma
4](documents.md#lemma-4-recorded-iterations-read-no-open-growth) of the
Documents proof, how a [Document](../runtime/documents.md) stores and
resumes the records of runs (*Runs* in [Reuse
Model](../runtime/documents.md#reuse-model)), read so:

- (N1) a record of a run of a repetition at a position is stored by a
  walk of the repetition that starts at the position, when that walk ends,
  and it holds, with its start, each iteration of the walk that matched,
  whether the walk evaluated it or reused it from the record that it
  resumed; it is stored only when no call during the walk read the current
  result of a growth, directly or through an entry, nor reused a result
  that read one;
- (N2) a walk resumes whole only a record of its repetition at its own
  start, stored earlier in the same parse or by an earlier parse, and
  across one edit only a record of the previous parse.

Whether a walk of a twin resumes a record that a walk of its rule stored
does not enter: the claim follows a repetition in its rule and in the twin
alike. In a Document parse whose records of sites, if it keeps any, are
exact, (R1) of the [Setting](#setting), and whose records of runs are
stored and resumed so:

1. *Reused iterations were evaluated.* Let a walk $W$ of the parse resume
   a run, and let $\iota$ be an iteration of $\mathcal{R}(W)$ that $W$
   reuses. Then either $\iota$ **comes from an earlier parse**: $W$
   reuses it from a record of an earlier parse, or from a record of this
   parse that holds it as an iteration that the walk which stored the
   record reused, and so on through records of this parse to a record of
   an earlier parse; or, before $W$ started, the parse evaluated an
   iteration $\iota_0$ of a walk of the same repetition, in its rule or in
   the twin, at the position of $\iota$, and $\mathcal{R}(\iota_0)$ and
   $\iota$ take the same steps up to the variants of their calls. So a
   call inside $\iota$ of a rule that the parse memoizes corresponds to a
   call at the same place inside $\mathcal{R}(\iota_0)$ with a key of the
   same class, which holds an activation at any depth when the first
   does, and neither reads a growth open at its start.
2. *Covered keys.* 2 of *Covered keys through records* holds with $x$
   also an iteration that the parse evaluates of a walk of any repetition,
   and with the path **followed through the records and the resumed
   runs**: followed as there and, at an iteration $\iota$ that a resumed
   walk reuses and that does not come from an earlier parse, into the
   iteration $\mathcal{R}(\iota_0)$ of 1; an iteration on the path that
   comes from an earlier parse is case (a), whose definition names it,
   since what answers it was stored by an earlier parse, as an entry of
   case (a) was.
3. *Heads.* *Heads in the reference parse* and *Reference links lead to
   repeated evaluations* hold outside cases (a) and (b) on their paths
   followed through the records and the resumed runs, with no case (d)
   or (e). So a free activation whose reference link has case (e) on its
   path is held by no head, unless case (a) or (b) arises on that path
   followed through the records and the resumed runs.

*Proof.* 1\. By (N2), $W$ resumed a record $\rho$ at its start $s$, and
by Lemma 3 of the Documents proof and its proof $W$ visits the
iterations that $\mathcal{R}(W)$ visits, in the same order, so it reuses
an iteration of $\rho$ as $\iota$. If $\rho$ is of an earlier parse,
$\iota$ comes from an earlier parse. Otherwise $W$ resumed $\rho$ whole,
$\iota$ starts at the start $r$ of that iteration, and by (N1) $\rho$ was stored,
before $W$ started, by a walk $W_1$ of the same repetition at $s$ when
$W_1$ ended, and its iteration at $r$ is the iteration of $W_1$ at $r$,
which $W_1$ evaluated or reused. If $W_1$ reused it, from the record
$\rho_1$ that $W_1$ resumed, which was stored before $W_1$ started and so
before $\rho$, the same holds of $W_1$, $\rho_1$ and $r$. The records of
the parse are stored one after another, so by induction on that order
the chain ends at a record of an earlier parse, and then $\iota$ comes
from an earlier parse, or at a walk $W_n$ of the same repetition at $s$
that evaluated an iteration $\iota_0$ at $r$, before $W_n$ ended and so
before $W$ started; $W_n$ stored a record, so no call during it read the
current result of a growth (N1).

$\mathcal{R}(\iota_0)$ and $\iota$ are iterations of $\mathcal{R}$ at $r$
of the element of one repetition, in its rule or in the twin, on the text
of this parse. Neither, nor a call inside either, reads the current result
of a growth: $\mathcal{R}(\iota_0)$ by the proof of Lemma 3 of the
Documents proof, since $\iota_0$ read none, and $\iota$ by Lemma 4 of that
proof and its proof, which compare it with the evaluation in $\mathcal{R}$
of the recorded iteration, here $\mathcal{R}(\iota_0)$, reading none. The
element reads no variable and no capture from outside, its cut decides
only between its own alternatives and whether the repetition fails, and
the lookahead state reaches no decision of the parser, so by the first
paragraph of the proof of *Covered keys through records*, with this in
place of *Growths lie outside the recorded walks*, $\mathcal{R}(\iota_0)$
and $\iota$ take the same steps up to the variants of their calls. The
rest is as in 1 of that claim.

2\. As 2 of *Covered keys through records*, with one more case for the
first evaluation of $\mathcal{R}$ on a segment from $\mathcal{R}(v)$ down
to $z'$ that has no counterpart in the parse: it lies inside an iteration
$\iota$ that a resumed walk $W$ reuses, by *Counterparts*, where $\iota$
is itself the evaluation of the parse whose counterpart it is. $\iota$
contains $z'$, and $W$, whose counterpart contains $\iota$, is evaluated
inside $v$, since nothing before the first evaluation without a
counterpart on the segment lacks one. If $\iota$ comes from an earlier
parse, case (a) arises on the path. Otherwise 1 gives the iteration
$\iota_0$, evaluated before $W$ started and so ended before $x$ ends, and
the call $z''$ at the place of $z'$ in $\mathcal{R}(\iota_0)$, with a key
of the class of $z$, which reads no growth open at its start; the next
segment leads from $\mathcal{R}(\iota_0)$ to $z''$, and the induction
hypothesis for $\iota_0$, which completed before $x$ does, gives an
evaluation of a key of the class of $z$ before it ended, unless case (a),
(b) or (c) arises further on the path. At an iteration that a record of
a site answers, 1 of *Covered keys through records* holds in such a parse
as stated, since its proof uses the parse only through (M1), (M2) and
*Counterparts*, which carries its induction past the resumed walks.

3\. As 3 of *Covered keys through records*, with one more case: the
first evaluation without a counterpart on the path from the evaluation
where $\mathcal{R}$ begins down to $z$ lies inside an iteration $\iota$
that a resumed walk $W$ of the parse reuses. $\mathcal{R}(W)$ starts
before $z$, which lies in $\iota$, and $z$ ends before $\mathcal{R}(Z)$
starts, so $\mathcal{R}(Z)$ does not contain $\mathcal{R}(W)$:
$\mathcal{R}(W)$ contains $\mathcal{R}(Z)$, in an iteration that $W$
evaluates after $\iota$, or ends before $\mathcal{R}(Z)$ starts. In
either case the parse starts $W$ before $Z$: by the induction along the
steps in *Counterparts*, the evaluations of the parse are the
counterparts of a subsequence of those of $\mathcal{R}$, made in the
order in which $\mathcal{R}$ makes them. If $\iota$ comes from an earlier parse, case (a) arises
on the path. Otherwise by 1 the parse evaluated $\iota_0$ before $W$
started, and by 2 with $x = \iota_0$ and the call at the place of $z$ in
$\mathcal{R}(\iota_0)$ it made an evaluation of a key of the class of $z$
before $\iota_0$ ended. So $Z$ is not the first of its class. The proof of
*Reference links lead to repeated evaluations* uses nothing else of the
parse, as there. $\square$

The claim counts nothing and changes no path of a link: *Heads in a parse
that resumes runs* shows that no link of an activation of *Replayed
activations* has case (e). In the first parse of a Document, and in the
first parse after a batch, no walk resumes a record of an earlier parse,
since a batch drops every run, so no iteration comes from an earlier
parse. Where case (a)
arises, at an entry or at an iteration that comes from an earlier parse,
is not examined here.

**Descent in the reference parse.** This claim bounds the activations
that memoized evaluations own, those inside the iterations of the kind
among them, by a count in $\mathcal{R}$ alone. It uses of the records
only that they are exact, (R1), through *Counterparts*; it uses neither
(M1) nor (M2), nor the covered keys, and it does not ask which records a
walk finds. Let
$\beta^1, \beta^2, \ldots$ be the successful activations of $\sigma$ at
$q$ in $\mathcal{R}$, in the order in which they start. Call $\beta^m$ a
**root** when $m = 1$ or when $\beta^m$ has no reference link of a rule
that the parse memoizes; otherwise *Reference links* joins $\beta^m$ to an
earlier activation $\beta_f$, its **parent**, and the root of $\beta^m$ is
the root of its parent. The **owner call** of an activation of
$\mathcal{R}$ is the innermost evaluation of $\mathcal{R}$ that contains
it and is made by a call of a rule that the parse memoizes, or the
evaluation where $\mathcal{R}$ begins.

1. The owner call of $\beta^m$ and the owner call of its root are made by
   calls with keys of one class, or both are the evaluation where
   $\mathcal{R}$ begins, and $\beta^m$ lies in the first at the place
   where the root lies in the second.
2. Let memoized evaluations $M$ and $M'$ of the parse own successful
   activations $\alpha$ and $\alpha'$ of $\sigma$ at $q$ whose
   counterparts have one root. Then $M$ and $M'$ are evaluations of keys
   of one class, or both are the evaluation where the parse begins, and
   $\alpha = \alpha'$ when $M = M'$.
3. So the successful activations of $\sigma$ at $q$ that memoized
   evaluations own are at most six times the roots of their
   counterparts, wherever they lie: inside an iteration of a walk of a
   site that keeps records, whatever its slot held and whichever records
   its walk could find, or outside.

*Proof.* 1\. By induction on $m$. Let $\beta^m$ have the reference link
$(e^k, e^{k\prime})$ of a rule that the parse memoizes and the parent
$\beta_f$. $e^{k\prime}$ takes the steps of $e^k$ up to the variants of
its calls, and $\beta^m$ is the activation of $e^{k\prime}$ that
corresponds to $\beta_f$ (*Reference links*). $e^{k\prime}$ is made by a
call of a rule that the parse memoizes and contains $\beta^m$, so the
owner call $O$ of $\beta^m$ is $e^{k\prime}$ or lies inside it. The
evaluation $O_f$ at the place of $O$ in $e^k$, or $e^k$ itself, is made
by a call of the same rule or of its twin, which the parse memoizes when
it memoizes the rule (Lemma 3), at the same position, with a key of the
class of that of $O$, as in the proof of *Reference links lead to
repeated evaluations*, and it holds $\beta_f$ at the place where $O$
holds $\beta^m$. No evaluation made by a call of a rule that the parse
memoizes lies between $O_f$ and $\beta_f$: the one at its place between
$O$ and $\beta^m$ would be made by a call of the same rule or of its
twin. So $O_f$ is the owner call of $\beta_f$, and the induction
hypothesis for $\beta_f$, which starts before $\beta^m$, gives the
statement.

2\. $\mathcal{R}(M)$ is made by a call of a rule that the parse memoizes,
or is the evaluation where $\mathcal{R}$ begins, and contains the
counterpart of $\alpha$. An evaluation of $\mathcal{R}$ made by such a
call between $\mathcal{R}(M)$ and that counterpart would contain the
counterpart of an evaluation of the parse, so it would be the counterpart
of an evaluation of the parse (*Heads in a parse that keeps records*),
made by a call of a rule that the parse memoizes between $M$ and
$\alpha$, and $\alpha$ would lie outside the own region of $M$. So
$\mathcal{R}(M)$ is the owner call of the counterpart of $\alpha$, and
likewise for $M'$. By 1 the two owner calls have keys of the class of
the owner call of the root, or are the evaluation where $\mathcal{R}$
begins, and hold the two counterparts at one place. When $M = M'$ the two
counterparts are one evaluation of $\mathcal{R}$, and so are $\alpha$ and
$\alpha'$.

3\. By 2 the memoized evaluations that own the activations of one root
are distinct evaluations of one class, at most six (Lemma 3), or the one
evaluation where the parse begins. $\square$

The claim holds of the parse whatever it answers from the memo or from a
record: it compares no two evaluations of the parse, so it needs no
replay, and a record that one evaluation of a class takes and another
does not changes nothing in it. The argument of 2 holds of every
evaluation of a key and the activations in its own region, so the class
of an evaluation of a key that holds a successful activation of $\sigma$
at $q$ is the class of the owner call of a root.

The roots are a count of $\mathcal{R}$, not of the parse. By *Reference
links* and the claims that it rests on, $\beta^m$ with $m \ge 2$ has a
reference link unless a dropped step at the position of its growth or a
path that enters the growth again of $\mathcal{R}$ leads to it, or it is
one of the activations that the restarts leave out: of case 5, of the
third case of *Dropped steps*, or a repeat at the position of a growth
(*Repeats inside a growth* of Lemma 5). Its reference link is of a rule
that the parse memoizes when the rule of $\sigma$ calls a rule or is
memoized at its calls, and otherwise it can be a leaf link (*Links to
heads*). So, for a source whose rule calls a rule or is memoized at its
calls, $\beta^1$ is the only root at $q$ when none of those arises at
$q$ in $\mathcal{R}$, as in a grammar without left recursion, and then
at most six successful activations of $\sigma$ at $q$ are owned by
memoized evaluations, the first iterations of a walk up to the minimum,
the last iteration of a walk that ends plain, the iterations of a walk
made before the site turns hot and the iterations at a site whose
records are kept under the environment as an object included. Where they
arise, the roots that they make in $\mathcal{R}$ can outnumber the
activations that they make in the parse, since $\mathcal{R}$ evaluates
what the parse answers; the number of the roots at $q$ whose descent
holds a counterpart of an activation of the parse is not bounded here;
*Fresh roots* below reduces it to a count that the evaluations which
$\mathcal{R}$ repeats do not enter.

**Fresh roots.** This claim removes from the count of the roots what
$\mathcal{R}$ evaluates more than once because it stores nothing. It is a
claim on $\mathcal{R}$ and uses of the parse only *Counterparts*, the
claim above and, in 2, the first paragraph of the proof of *Replays*,
that a call of the parse that reads no growth open at its start reads
none in $\mathcal{R}$, which rests on (S), (D) and *Growths lie outside
the recorded walks*. It uses neither the covered keys nor the
correspondence that *Replays* states, so none of the cases (a) to (d). Call a successful activation $\beta$ of $\sigma$ at $q$ in
$\mathcal{R}$ **fresh** when its owner call is the evaluation where
$\mathcal{R}$ begins, or is an evaluation in $\mathcal{R}$ of a key that
reads no growth open at its start and no evaluation in $\mathcal{R}$ of a
key of its class that reads no growth open at its start starts before it.
An activation whose owner call is made by a call of a leader, or reads a
growth open at its start, is not fresh.

1. A fresh activation is a root.
2. Let a memoized evaluation $M$ of the parse own a successful activation
   $\alpha$ of $\sigma$ at $q$, and let $z_1$ be the first evaluation in
   $\mathcal{R}$ of a key of the class of $M$ that reads no growth open at
   its start, or the evaluation where $\mathcal{R}$ begins when $M$ is the
   evaluation where the parse begins. Then $z_1$ holds, at the place where
   $\mathcal{R}(M)$ holds the counterpart of $\alpha$, a fresh activation
   $\beta^\circ$ of $\sigma$ at $q$ whose owner call is $z_1$.
3. So the successful activations of $\sigma$ at $q$ that memoized
   evaluations own are at most six times the fresh roots at $q$ whose
   owner call has the class of a memoized evaluation of the parse.
4. Let $y$ be an evaluation in $\mathcal{R}$ of a key, or a growth in
   $\mathcal{R}$ of a key (*Families of growths* in Lemma 5), whose call
   reads no growth open at its start, and let an evaluation in
   $\mathcal{R}$ of a key of its class, or a growth in $\mathcal{R}$ of
   its key, whose call reads none either, start before $y$. Then no fresh
   activation whose owner call is a call lies in $y$.

*Proof.* 1\. Let $\beta$ be fresh with the owner call $O$, and suppose
that it has a reference link $(e^k, e^{k\prime})$ of a rule that the parse
memoizes. If $O$ is the evaluation where $\mathcal{R}$ begins, no
evaluation made by a call of a rule that the parse memoizes contains
$\beta$, and $e^{k\prime}$ is one. Otherwise $e^{k\prime}$ is made by such
a call and contains $\beta$, so $O$, the innermost such evaluation, is
$e^{k\prime}$ or lies inside it. The proof of *Reference links lead to
repeated evaluations* then applies with $O$ in the place of
$\mathcal{R}(Z)$, of which it uses only that it is $e^{k\prime}$ or lies
inside it and that its call reads no growth open at its start: the
evaluation $z^\circ$ at the place of $O$ in $e^k$ is made by a call with a
key of the class of $O$, reads no growth open at its start, and ends
before $e^{k\prime}$ starts, so it starts before $O$, and $\beta$ is not
fresh.

2\. Let $M$ be made by a call. Its call reads no growth open at its start
in the parse, and so in $\mathcal{R}$ (the proof of *Replays*), so
$\mathcal{R}(M)$ is an evaluation in $\mathcal{R}$ of a key of the class
of $M$ that reads none, and $z_1$ exists. $z_1$ and $\mathcal{R}(M)$ take
the same steps up to the variants of their calls (*Counterparts*), so
$z_1$ holds a successful activation $\beta^\circ$ of $\sigma$ at $q$ at
the place of the counterpart of $\alpha$. $\mathcal{R}(M)$ is the owner
call of that counterpart (the proof of 2 of the claim above), so no
evaluation made by a call of a rule that the parse memoizes lies between
them, and none lies between $z_1$ and $\beta^\circ$, where the calls are
of the same rules or of their twins. So $z_1$ is the owner call of
$\beta^\circ$, and $\beta^\circ$ is fresh by the choice of $z_1$. Let $M$
be the evaluation where the parse begins. Then $\mathcal{R}(M)$ is the
evaluation where $\mathcal{R}$ begins and is the owner call of the
counterpart of $\alpha$, which is fresh.

3\. By 2, each owned $\alpha$ has a fresh activation $\beta^\circ$, a root
by 1, whose owner call has the class of the owner $M$ of $\alpha$, at the
place of the counterpart of $\alpha$. If $\alpha$ and $\alpha'$, owned by
$M$ and $M'$, have one $\beta^\circ$, then $M$ and $M'$ are evaluations
of keys of the class of its owner call, or both are the evaluation where
the parse begins, and the counterparts of $\alpha$ and $\alpha'$ lie at
one place in $\mathcal{R}(M)$ and $\mathcal{R}(M')$; when $M = M'$ they
are one evaluation, and so are $\alpha$ and $\alpha'$. A class has at
most six memoized evaluations (Lemma 3), so at most six owned activations
have one $\beta^\circ$.

4\. Let $y_1$ be the earlier one. $y_1$ and $y$ take the same steps up to
the variants of their calls (*Counterparts*; a leader has no twin, and
two growths of one key take the same steps, as in *Heads hold what later
growths hold*). $y_1$ does not contain $y$: if $y$ lay at a place strictly
inside $y_1$, the evaluation at that place inside $y$ would take the steps
of $y$ again, strictly inside it, and so on without end, while
$\mathcal{R}$ makes finitely many evaluations: the reasons of
[Termination](../runtime/parsing.md#termination) use no memo, and [Lemma 1
of the memoization proof](memoization.md#lemma-1-context) takes the
nesting of the evaluations of $\mathcal{R}$ as finite by it. So $y_1$ ended
before $y$ started. Let a fresh $\beta$ whose owner call $O$ is a call lie
in $y$. $O$ is made by a call of a rule other than a leader, and $y$ is
made by a call of a rule that the parse memoizes and contains $\beta$, so
$O$ is $y$ or lies inside it. The evaluation $O_1$ at the place of $O$ in
$y_1$ is made by a call with a key of the class of $O$. It reads no
growth open at its start. A growth that started inside $y_1$, or $y_1$
itself when it is a growth, corresponds to the growth at its place inside
$y$, or to $y$, which is open at the start of $O$, and $O$, taking the
steps of $O_1$, would read it, which it does not. A growth open at the
start of $y_1$ is read by no evaluation inside $y_1$, whose call reads
none. $O_1$ starts before $y_1$ ends, so before $O$ starts, and $\beta$
is not fresh. $\square$

By 4, the evaluations that $\mathcal{R}$ makes of a call that it has
evaluated before with a key of the class, and the growths that it makes
again at a call of a leader whose key it has grown, hold no fresh root
with an owner call, whether the parse answers those calls or not: this is
where the roots outnumber what the parse evaluates through the memo, and
3 does not count it. An evaluation in a context of growths that
$\mathcal{R}$ makes again in each step, or takes again where the parse
reuses a result or a step kept in a context, is of one of two kinds. If
its call reads a growth open at its start, it is the owner call of no
fresh root by the definition. If its call reads none, it is an evaluation
of a key (*Keys and classes*), and by 4 only the first evaluation in
$\mathcal{R}$ of its class is the owner call of a fresh root or contains
one with an owner call; a later step or a later growth evaluates the
class again and no longer first. So the count
of 3 is a count of the classes: for each class of memoized evaluations of
the parse, the roots at $q$ that the first evaluation of the class in
$\mathcal{R}$ holds outside its calls of rules that the parse memoizes.
The fresh roots play in $\mathcal{R}$ the part that the activations held
by heads play in the parse (*Activations held by heads*), with
the first evaluation of a class in $\mathcal{R}$ in the place of the
head; this is a comparison, and no correspondence between the two is
shown or used. They need no replay, so the iterations that a record answers
and the entries and kept results of cases (a) and (b) do not enter.

The fresh roots at $q$ are not bounded by this claim. For a source whose
rule calls a rule or is memoized at its calls, a fresh root other than
$\beta^1$ is led to by a dropped step at the position of its growth or by
a path that enters a growth again, or is one of the activations that the
restarts leave out (the paragraph before this claim). Two numbers are
open for them: the classes, at the positions at or before $q$, whose
first evaluation in $\mathcal{R}$ holds such a root at $q$ as its owner
call, and the roots of that kind at $q$ that one such evaluation holds.
For a root of the second case of *Repeats inside a growth*, in a step of
a growth at $p$, the owner call lies inside that step on a path that
follows no read of its seed, and is first of its class in $\mathcal{R}$.
The claims of Lemma 5 that bound such parts, *Sources at fixed offsets*
and *Failures at the position of the growth*, are proved of the parse,
where they leave out a constant number of steps for each growth and one
step for each rule that the memo answers afterwards; they are not read
here for the first evaluations of the classes in $\mathcal{R}$, where
freshness would take the place of the memo, and in the steps that they
leave out a part at $p$ can reach a position far from $p$, so the growths
at the positions before $q$ are not bounded in number by them. *Fresh
roots in one evaluation* below settles the second number for a source
whose rule the parse memoizes, apart from case 5.

**Fresh roots in one evaluation.** Let the parse memoize the rule $Q$ of
$\sigma$, let $Q$ be no leader, and let $\sigma$ be no bounded scan, as
in the claims from *Direct activations* on. For a leader $Q$ there is
nothing to count here: an activation of $\sigma$ is then direct in a
growth step, which owns it in the parse, and no memoized evaluation does
(*Own regions*).

1. A fresh activation of $\sigma$ is direct in its owner call, which is a
   body evaluation of $Q$ or of its twin and no growth step.
2. Of the fresh activations of $\sigma$ at $q$ that have one owner call,
   every one after the first is an activation that a path that enters the
   growth again leads to (case 5), from a unit that is abandoned in the
   body of that evaluation, below every call.
3. So the successful activations of $\sigma$ at $q$ that memoized
   evaluations own are at most six times the sum of two numbers: the
   classes of the keys of $Q$, at the positions at or before $q$, whose
   first evaluation in $\mathcal{R}$ that reads no growth open at its
   start is the owner call of a fresh root at $q$, with one more for the
   evaluation where $\mathcal{R}$ begins, and the fresh roots at $q$ of
   2 that case 5 leads to.

*Proof.* 1\. Let $\beta$ be fresh, and let $E$ be the body evaluation of
$Q$ or of its twin in which it is direct. $Q$ is no leader and a twin is
on no cycle, so $E$ is no growth step. No evaluation made by a call lies
between $E$ and $\beta$. If $E$ is the evaluation where $\mathcal{R}$
begins, it is the owner call of $\beta$, since no evaluation made by a
call contains $\beta$ inside it. Otherwise $E$ is made by a call of $Q$
or of its twin, which the parse memoizes (Lemma 3 for the twin), so $E$
is the innermost evaluation that contains $\beta$ and is made by a call
of a rule that the parse memoizes: the owner call.

2\. Let $\beta$ and $\beta'$ be two of them, $\beta$ first, with the owner
call $E$. By 1 both are direct in $E$, which is no growth step, and they
are successful activations of $\sigma$ that start at $q$. *Direct
activations* holds of $\mathcal{R}$ (*Reference links*), so a path that
enters the growth again leads from the abandoned unit to $\beta'$, and
by its proof that unit lies in the body of $E$ below every call.

3\. By 3 of *Fresh roots* the owned activations are at most six times the
fresh roots at $q$ whose owner call has the class of a memoized
evaluation of the parse. By 1 the owner call of each is a body evaluation
of $Q$ or of its twin that holds it directly, so it starts at or before
$q$, and it is the evaluation where $\mathcal{R}$ begins or the first
evaluation in $\mathcal{R}$ of its class that reads no growth open at its
start, by the definition of a fresh activation. A class has one such
first evaluation, and by 2 the fresh roots at $q$ with one owner call are
one and those that case 5 leads to. $\square$

The claim says nothing of a source whose rule the parse does not
memoize: its fresh activations lie in the own region of an evaluation of
another rule, where two of them at one position can be direct in two
body evaluations of the rule of the source. And it leaves the first
number as it was: the positions $s \le q$ at which the first evaluation
of a class of $Q$ holds a root at $q$ that a growth case makes. For a
root of the second case of *Repeats inside a growth*, in a step $j'$ of
a growth $g$ at $p$, that evaluation lies in step $j'$, since the root
is direct in it and lies in $g$, which a call makes; it starts at a position from $p$ to $q$ and follows
no read of the seed of that step, since an activation inside an
evaluation that follows a read follows it too; and no evaluation of a
key of its class that reads no growth open at its start starts before
it, in an earlier step of $g$ or elsewhere in $\mathcal{R}$. When $q$ is at most a constant
$c$ after $p$, those classes are the classes of $Q$ at $c + 1$
positions. When it is farther, a bound needs both that few steps of one
growth hold such an evaluation and that few growths at the positions
before $q - c$ do, and the second is not a count of the steps of one
growth: each such growth holds an earlier successful activation of
$\sigma$ at $q$, in its step $j$, but that one need not be fresh, and
$\mathcal{R}$ evaluates a class again in every growth that reaches it,
so the classes of the owner calls of those earlier activations give no
bound of the growths.

**Heads without the records.** This claim bounds the activations that
memoized evaluations own, those inside the iterations of the kind among
them, by the activations that the heads hold in a parse with no records,
no entries of an earlier parse and no reuse in a context of growths. Let
the records of the parse be exact, (R1) of the [Setting](#setting), and
let $\mathcal{X}$ be the parse of the same grammar and input, of the same
kind, a [Document](../runtime/documents.md) parse when the parse is one,
that memoizes the rules that the parse memoizes, has no entries from
earlier parses, keeps no record of any site, resumes no run, reuses
nothing kept in a context of growths, stops at no limit, and stores as
the [Setting](#setting) requires. Then the successful activations of
$\sigma$ at $q$ that memoized evaluations own in the parse are at most
six times the successful activations of $\sigma$ at $q$ that the heads
of $\mathcal{X}$ hold, the evaluation where $\mathcal{X}$ begins
included.

*Proof.* $\mathcal{X}$ is a parse of the Setting: its records are exact,
since it keeps none, and its memo meets the requirement of the Setting
by its definition. Its reference parse is $\mathcal{R}$, since
$\mathcal{R}$ evaluates every call and uses no entry and no record, so
it depends neither on the entries of earlier parses nor on the records
(*Counterparts*).

Let a memoized evaluation $M$ of the parse own a successful activation
$\alpha$ of $\sigma$ at $q$. By 2 of *Descent in the reference parse*,
$\mathcal{R}(M)$ is the owner call of the counterpart of $\alpha$: that
counterpart lies in the own region of $\mathcal{R}(M)$, with no
evaluation made by a call of a rule that the parse memoizes between
them. Let $M$ be other than the evaluation where the parse begins. It is
an evaluation of a key, of a class $C$ at a position $p$, so the call
$z$ of which $\mathcal{R}(M)$ is the body evaluation is of a rule that
the parse memoizes other than a leader and reads no growth open at its
start, and it contains a successful activation in $\mathcal{R}$. Let
$z_0$ be the outermost call on the path from the evaluation where
$\mathcal{R}$ begins down to $z$, $z$ itself possibly. The path above
$z_0$ holds no call, so no step of a growth either, each of which lies
inside the call of a leader; no alternative on it is skipped, since each
holds the activation (*Counterparts*); and $\mathcal{X}$ takes no record
and resumes no walk. So $\mathcal{X}$ makes an evaluation $x$, a call,
with $\mathcal{R}(x) = z_0$. Apply *Covered keys* to $\mathcal{X}$ with
this $x$ and $z$: case (a) does not arise, since $\mathcal{X}$ has no
entries of an earlier parse; case (b) does not arise, since it reuses
nothing kept in a context of growths and so answers no call or step, and
takes no run of steps, from what it keeps apart from the table; case (c)
does not arise, since $z$ holds an activation in $\mathcal{R}$; and case
(d) does not arise, since $\mathcal{X}$ keeps no record. So $\mathcal{X}$
makes an evaluation of the key of $z$, of the class $C$, and so has a
head $H$ of the class $C$, its first evaluation of a key of that class.

$\mathcal{R}(H)$ and $\mathcal{R}(M)$ are body evaluations in
$\mathcal{R}$ of calls with keys of one class. The calls of $H$ and $M$
read no growth open at their start, in $\mathcal{X}$ and in the parse,
as evaluations of keys, and so their counterparts read none in
$\mathcal{R}$ (the first paragraph of the proof of *Replays*). So they take the same steps up to the variants of their
calls (*Counterparts*), and the evaluations inside one correspond one to
one, in order, to those inside the other. A call of a rule that the
parse memoizes corresponds to a call of the same rule or of its twin,
which the parse memoizes too (Lemma 3), so the own regions correspond.
Let $\beta$ be the evaluation inside $\mathcal{R}(H)$ that corresponds to
the counterpart of $\alpha$: a successful activation of $\sigma$ at $q$
in the own region of $\mathcal{R}(H)$. $\mathcal{X}$ makes, inside $H$,
an evaluation whose counterpart is $\beta$. On the path from
$\mathcal{R}(H)$ down to $\beta$ every call is of a rule that the parse
does not memoize, which the memo never answers (in a Document as *Transient
calls of a Document* in Lemma 3 reads it); no step of a growth lies
there, since one lies inside the call of a leader, which the parse
memoizes; no alternative on it is skipped, since each holds $\beta$; and
$\mathcal{X}$ takes no record and resumes no walk. So $\mathcal{X}$
evaluates every evaluation on that path (*Counterparts*), and makes an
activation $\alpha_{\mathcal{X}}$ with the counterpart $\beta$, which
succeeds, as $\beta$ does, and lies in the own region of $H$, since no
call on the path is of a rule that the parse memoizes. So $H$ holds it.
When $M$ is the evaluation where the parse begins, the same holds with
$H$ the evaluation where $\mathcal{X}$ begins, a head of its own, whose
counterpart is the evaluation where $\mathcal{R}$ begins, as that of $M$
is.

Map $\alpha$ to $\alpha_{\mathcal{X}}$. For one $M$ the map is injective:
distinct activations held by $M$ have distinct counterparts in
$\mathcal{R}(M)$, these correspond to distinct evaluations inside
$\mathcal{R}(H)$, and distinct evaluations of $\mathcal{X}$ have distinct
counterparts, so distinct counterparts come from distinct activations of
$\mathcal{X}$. Memoized evaluations of distinct classes map to distinct
heads, and an activation lies in the own region of at most one of the
heads of $\mathcal{X}$, the evaluation where it begins included: an
evaluation of a key inside another evaluation is made by a call of a
rule that the parse memoizes, outside the own region of the other. A class has at
most six memoized evaluations (Lemma 3), so at most six activations of
the parse map to one activation of $\mathcal{X}$. $\square$

The claim uses of the parse only its evaluations $M$ and $\alpha$, their
counterparts and 2 of *Descent in the reference parse*, whose path from
$\mathcal{R}(M)$ down to the counterpart of $\alpha$ holds only
counterparts of evaluations of the parse. So it holds whatever the parse
answers from the memo, from an entry of an earlier parse or from a
record, and whatever runs of a Document it resumes. 2 of *Descent* rests
on the nesting of the counterparts that *Counterparts* gives, which takes
in the iterations that a resumed walk reuses as its fifth kind: an
evaluation of $\mathcal{R}$ that contains the counterpart of an
evaluation of the parse lies inside no reused iteration other than as
that iteration itself, which is the counterpart of one, so it is the
counterpart of an evaluation of the parse, and a reused iteration, which
contains no evaluation of the parse, lies on the path only as $\alpha$
itself. And the claim compares no two evaluations of the parse.
It gives the parse no credit for its records: a walk of the head of a
class is counted in $\mathcal{X}$ with all its iterations.

What it leaves is the number of the successful activations of $\sigma$
at $q$ that the heads of $\mathcal{X}$ hold. That is the count of
*Links to heads* and *Activations held by heads* in a parse that keeps
no records, has no entries of an earlier parse and reuses nothing in a
context of growths, so that cases (a), (b) and (d) of the covered keys
and the third open part of step 1 of [Open Steps](#open-steps) do not
arise in it, with the open parts of that count that remain: for a source
in a rule that calls no rule and is not memoized at its calls, the leaf
links that lead to an activation held by a head, which step 1 leaves
open. So the leaf reference links of the activations that memoized
evaluations own in the parse, inside the iterations of the kind or
outside, are counted as the leaf links of such a parse, with the open
parts of step 1 for that parse, and the records add no open part of
their own to them. Whatever bound step 1 gives for $\mathcal{X}$ is in
terms of the evaluations of $\mathcal{X}$, whose calls at one position
are counterparts of calls of $\mathcal{R}$ there, as those of the parse
are. The $K$ of [Classification](../runtime/parsing.md#classification)
counts the combinations of values among the calls at one position of a
parse, and the records of the parse do not raise it in $\mathcal{X}$
when they are made as (M1) and (M2) of *Covered keys through records*
state: a record answers an iteration at a position only after a walk of
the parse evaluated an iteration of the site there, under the same values
of the variables for a site that is not free of them, and the iterations
of a free or skip-run site read no variable from outside, so the calls of
$\mathcal{X}$ inside the iterations that the records answer repeat calls
of the parse with the same values; and the calls that $\mathcal{X}$
evaluates inside a call that the parse answers there repeat, with the same
values, calls of the evaluation of this parse that stored or kept what
answers it ([Lemma 1 of the memoization
proof](memoization.md#lemma-1-context)), or lie in the body of an entry of
an earlier parse. That the $K$ of $\mathcal{X}$ is at
most that of the parse is not shown when the parse answers calls from
entries of an earlier parse, whose bodies $\mathcal{X}$ evaluates,
resumes a run of a Document, or keeps records not made as (M1) and (M2),
which (R1) alone allows. In the class Linear the $K$ of every parse is at
most the constant of Lemma 8, so the comparison matters only for the
class Linear given an environment bound.

**Leaf links in a parse that keeps records.** In a parse whose records are
exact, (R1) of the [Setting](#setting), *Growths around a leaf link*,
*Calls that call no rule* and *Outer links across runs* hold as stated,
and *Outer links inside a memoized link* holds with case (d) of the
covered keys beside cases (a) and (b), which arises there only for an
activation that lies, inside $e'_j$, in an iteration of the kind. Where
the innermost evaluation that contains the two activations of a link is a
growth, the first three rest, as they do without records, on the first
case of *Repeats inside a growth* of Lemma 5, which *Repeats inside a
growth in a parse that keeps records* below shows of such a parse.

*Proof.* The first three claims are about the activations
$\alpha_1, \alpha_2, \ldots$, their links and units, and the frames, the
body evaluations in progress, all of which the parse evaluates; an
iteration that a record answers, and a call inside one, is none of them.
Their proofs use the following of the parse.

1. *Restarts*, *Dropped steps* and *Repeats*, which hold (*Restarts in a
   parse that keeps records*), and with them where the parser goes back,
   the exception of the restarts included, and the success and the end of
   the evaluations on the path from $N_l$ down to an activation or a
   frame, which are 1 and 2 of that claim.
2. That an evaluation started inside another ends inside it, that a frame
   is left once, and the order in which the parser starts and ends its
   evaluations. A walk that takes a record starts no evaluation at a
   position before the end of the record and ends none but itself.
3. What a growth does: that a step is kept or dropped, that a later step
   starts after the one before it ends, that the seeds grow longer, that a
   growth ends only with a dropped step, which calls read a growth, and
   how the parse answers or grows a call of a leader. A walk is no growth
   and no step of one, and no growth step, no evaluation in a context of
   growths and no call that starts a growth lies in an iteration of a
   walk of a site that keeps records (*Growths lie outside the recorded
   walks*), so a record answers none of them, and by (R1) a growth around
   a walk that takes a record has the steps and the results that it has
   when the walk evaluates its iterations.
4. In *Calls that call no rule*, that a body evaluation reaches each place
   of its body at $p$ at most a constant number of times. The evaluations
   that a body evaluation of the parse makes are the counterparts of
   distinct evaluations of its counterpart in $\mathcal{R}$, at the same
   positions (*Counterparts*), so it reaches a place at $p$ no more often
   than its counterpart does, and $\mathcal{R}$, which keeps no record,
   is a parse of which the argument of that proof holds. The bound $c'$
   that the argument gives depends on the grammar alone, so one $c'$
   serves every parse, with records or without.
5. Properties of the grammar, applied to what evaluations consume and
   examine: that a unit whose first shared call is the call of the link
   examines no input before it, that the continuation reaches the later
   evaluation of the link without consuming input, that what follows a
   call in a frame that ends where the call ended can match without
   consuming input, and the definition of nesting at one end. By (R1) a
   walk that takes a record ends where the walk that evaluated its
   remaining iterations would end and examines what it would examine, so
   each of these holds of an evaluation that contains such a walk as of
   its counterpart in $\mathcal{R}$ (3 of *Restarts in a parse that keeps
   records*).
6. That $r$ calls no rule and reads no variable, so that the evaluation
   of a call of $r$ that contains an activation is the innermost call
   evaluation that contains it, and that while it is in progress the
   parser starts only evaluations inside the body of $r$. Both are about
   the evaluations that the parse makes.

The proofs use nothing else of the parse. The proof of *Outer links
inside a memoized link* uses that an evaluation of a key which holds an
activation inside $e'_j$ is $e'_j$ or lies inside it, which is 2 above and
the definition of an own region, and *Nothing in a memoized link is a
head*, which holds with case (d), and that case arises only when the
evaluation of the key lies, inside $e'_j$, in an iteration of the kind
(*Heads in a parse that keeps records*); the activation that it holds
then lies in that iteration. $\square$

The claim does not cover the claims on leaf links from *Memoized links
to the frame of an outer link* on. *Later leaf links in a parse that
keeps records* below treats those up to *Nesting at one end*, and not
*Calls of scan-only rules* or *Seed reads*. The first case of *Repeats inside a
growth* is shown of such a parse next.

**Repeats inside a growth in a parse that keeps records.** In a parse
whose records are exact, (R1) of the [Setting](#setting), *Repeats inside
a growth* of Lemma 5 holds as stated up to the bound of its second case:
step $j$ is kept, and when $\alpha'$ follows a read of the seed of step
$j'$, the conclusion of its first case holds. So two activations of the
sequence $\alpha_1, \alpha_2, \ldots$ whose innermost common evaluation is
a growth are in the first case with records as without, the sequence
leaving out the activations that follow no such read. That, for an
activation which follows no such read, step $j'$ is one of a constant
number of steps of its growth or $q$ lies at most a constant after $p$,
rests on the claims of Lemma 5 before *Calls in a context of growths*,
which have not been read again for a parse that keeps records.

*Proof.* $\alpha$ and $\alpha'$ are activations, which the parse
evaluates, and so are the steps that hold them. The proof of the first
case uses the following of the parse.

1. *What a growth does.* That a growth drops at most its last step, that
   the result of a kept step is the seed of the next, and that the seeds
   grow longer. This is 3 of *Leaf links in a parse that keeps records*: a
   walk is no growth and no step of one, and by (R1) a growth around a
   walk that takes a record has the steps and the results that it has
   when the walk evaluates its iterations.
2. *The reads of the seed.* A read of the seed is a call of a rule of the
   cycle that the growth answers with its growing result, or a call whose
   result includes one, and that $\alpha'$ follows it is stated of
   evaluations that the parse makes. No call of a rule of a
   left-recursion cycle is made inside an iteration of a walk of a
   candidate site or of a skip-run site, in the parse or in $\mathcal{R}$
   (*Growths lie outside the recorded walks*), so a record answers no
   iteration that holds a read of a seed, and the parse answers the reads
   that it answers without records.
3. *Where the parser goes back and what evaluations give.* That an
   evaluation which is not abandoned ends at or after the end of each
   child that is not abandoned, which gives $q \ge e_{j'}$ and the
   existence of $w$; the success of the evaluations above $w$ on the path;
   and, by the argument of the restarts, that from the end of $w$ the
   parser returns to no position before $s$ in step $j$ apart from the
   exception, and that an evaluation in a later step which follows a read
   of its seed starts at or after the end of that seed. These are 1 and 2
   of *Restarts in a parse that keeps records*, under which the proof of
   *Restarts* holds.
4. *The continuation.* That what the parser evaluates from $s$ on, up to
   $\alpha'$, is a continuation of $w$ in the sense of
   [Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)
   is a property of the grammar, read from the expressions whose
   evaluations the parser is in: what follows $w$ in step $j$, the end of
   the body of the leader, and what follows the reads of the seed. The
   arguments of *Direct activations* and *Repeats* then use of the
   continuation that the parser consumes through it the input from the
   end of the shared calls to $q$, and they hold where a record covers a
   part of that input (3 of *Restarts in a parse that keeps records*).
5. *Dropped steps.* Where $w$ is a dropped step, the claim of that name
   treats it, and it holds (6 of *Restarts in a parse that keeps
   records*).

The proof of the first case uses nothing else of the parse. $\square$

**Repeats inside a growth in a parse that resumes runs.** In a
[Document](../runtime/documents.md) parse whose records of sites, if it
keeps any, are exact, (R1) of the [Setting](#setting), the first case of
*Repeats inside a growth* of Lemma 5 holds when the parse resumes runs,
as *Repeats inside a growth in a parse that keeps records* states it:
step $j$ is kept, and when $\alpha'$ follows a read of the seed of step
$j'$, the conclusion of its first case holds. That an activation which
follows no such read lies in one of a constant number of steps of its
growth, or at most a constant after $p$, rests, as there, on the claims of
Lemma 5 before *Calls in a context of growths*, which have not been read
again for such a parse.

*Proof.* $\alpha$ and $\alpha'$ are activations that the parse makes,
evaluated or reused, and a reused iteration contains no evaluation of the
parse other than itself (*Counterparts in the reference parse*), so the
growth that contains both and its steps $j$ and $j'$ are evaluations that
the parse makes. The proof of the first case uses of the parse the five
facts that the proof of *Repeats inside a growth in a parse that keeps
records* lists. Each holds of a walk that takes a record of a site as
that proof shows, its fact 1 through 3 of the proof of *Leaf links in a
parse that keeps records*, and of a walk that resumes a run as follows.

1. *What a growth does.* That a growth drops at most its last step, that
   the result of a kept step is the seed of the next, and that the seeds
   grow longer are properties of how the parse grows a leader, which a
   resumed walk does not change: a walk is no growth and no step of one,
   the parse starts no growth step, no evaluation in a context of growths
   and no call that starts a growth inside an iteration that it reuses,
   since it evaluates nothing there, and the outcomes and ends that the
   steps of a growth around a resumed walk compare are those of their
   counterparts in $\mathcal{R}$ (2 of *Restarts in a parse that resumes
   runs*).
2. *The reads of the seed.* That $\alpha'$ follows a read of the seed is
   stated of evaluations that the parse makes. A reused iteration holds no
   read of a seed: neither it nor a call inside its counterpart in
   $\mathcal{R}$ reads the current result of a growth ([Lemma
   4](documents.md#lemma-4-recorded-iterations-read-no-open-growth) of the
   Documents proof and its proof, as in 1 of *Covered keys through resumed
   runs*), so the parse answers outside the reused iterations every read
   of a seed that it answers.
3. *Where the parser goes back and what evaluations give.* These are 1
   and 2 of *Restarts in a parse that resumes runs*, under which the proof
   of *Restarts* holds.
4. *The continuation.* What the parser evaluates from $s$ on, up to
   $\alpha'$, is a continuation of $w$ by the grammar, as there, and the
   input that the parser consumes through it is the same where a reused
   iteration covers a part of it (3 of *Restarts in a parse that resumes
   runs*).
5. *Dropped steps.* Where $w$ is a dropped step, the claim of that name
   treats it, and it holds (6 of *Restarts in a parse that resumes runs*).

The proof of the first case uses nothing else of the parse, and it does
not use the claims on leaf links. $\square$

**Later leaf links in a parse that keeps records.** In a parse whose
records are exact, (R1) of the [Setting](#setting), with an iteration of
the kind as in *Heads in a parse that keeps records*:

1. *The last unit around the earlier frame*, with the paragraph after it
   that carries its proof on to the start of $\alpha_j$, and *Caller
   frames in a later step* hold as stated. So does *Nesting at one end*,
   with one reading: where an alike call, the call $S$ or a call through
   which $\kappa$ reaches $S$ lies in an iteration that a record answers,
   its end is the end of the evaluation that $\mathcal{R}$ makes of it
   there, and the hypothesis that two evaluations of an alike call at one
   position end at one position is made of that evaluation.
2. *Frames around the frame of an outer link*, *Shared caller frames*,
   the statements on $E_2$ and on $F_2$ in the text after *The frame of an
   outer link as the later evaluation of a shared call*, and the statement
   on an evaluated pair after *Outer links inside a memoized link* hold
   with case (d) of the covered keys beside cases (a) and (b). Case (d)
   arises there only when the activation that no head is to hold,
   $\alpha_{l'}$ or the activation that starts while $E_2$ is in progress,
   lies in an iteration of the kind. *The frame of an outer link as the
   later evaluation of a shared call* itself holds as stated.
3. *Memoized links to the frame of an outer link* holds as stated when
   $\alpha_{l'}$ is no iteration of the kind and lies inside none.
   Otherwise its first conclusion has one more exception: after $e_j$
   ends, $w_j$ evaluates no call of $r$ at $p$ unless the sequence leaves
   out the activation at $q$ of that call, or a record answers, in the
   evaluation of that call, the iteration of $\sigma$ at $q$ or an
   iteration that contains it. Its second conclusion, on a unit $w_j$
   that starts before $w_l$ ends, holds as stated in either case. The
   text after the claim that states the case which remains repeats the
   first conclusion, and holds with the same exception.
4. The paragraph after *Evaluated pairs of a shared call* on the heads
   that hold the activations of leaf links holds with two changes. The
   positions at which the body evaluations of $R$ that hold the
   $\alpha_i$ start can change also at a memoized link whose $\alpha_i$
   is an iteration of the kind or lies inside one. And that a head $H'$
   which holds $\alpha_{j'}$ strictly encloses $e'_i$ is shown outside
   cases (a) and (b) when $\alpha_{j'}$ lies in no iteration of the kind.
5. Of an activation that no head holds, the text after *Outer links
   inside a memoized link* says that *Replayed activations* charges it to
   the head of its class when a memoized evaluation holds it. That holds
   of an activation that is no iteration of the kind and lies inside
   none, as *Replayed activations* does. The paragraph after it, on a
   memoized link whose later evaluation ends before $\alpha_{l'}$ starts,
   that $e'_j$ is $Y_{l'}$ or lies strictly inside it, holds as stated.

These claims keep the hypotheses, the cases that they leave unshown and
the open points that they state without records. *Calls of scan-only
rules* is a claim about a parse that keeps the records of scan sites and
rests on Lemmas 5 and 6 of the Run Sites proof and on no claim of this
lemma; a scan-only rule holds no source but scan sites, so no other
record is kept inside it. *Seed reads* proves nothing. Neither is shown
here.

*Proof.* The claims are about the activations $\alpha_1, \alpha_2,
\ldots$, their links and units, the frames and the evaluations of calls,
all of which the parse evaluates. Their proofs use the claims that
*Restarts in a parse that keeps records*, *Heads in a parse that keeps
records*, *Leaf links in a parse that keeps records* and *Repeats inside
a growth in a parse that keeps records* show, the facts 1 to 4 of the
first of these and 2, 3, 5 and 6 of the third, and what follows.

*The last unit around the earlier frame.* Its proof uses *Restarts* and
*Dropped steps* for the first abandoned evaluation on the path from $N_k$
to $\alpha_{k-1}$; that, apart from the exception, only the abandonment of
an evaluation that started before $\bar{s}$ returns the parser before
$\bar{s}$ (1 of *Restarts in a parse that keeps records*); that an
evaluation in progress after another ends, and not started inside it,
encloses it (2 of *Leaf links in a parse that keeps records*); the
argument of *Direct activations*; both cases of *Repeats inside a
growth*, the second only in that the sequence leaves its activations
out, which is the definition of the sequence; and that the end of a seed
read in a later step lies at or after $\bar{s}$, from the proof of
*Restarts*. The paragraph after the claim uses the same.

*Frames around the frame of an outer link.* Its proof uses *Calls that
call no rule*, that the link $l'$ leaves $Y_{l'}$ last, and the order in
which the parser starts and leaves frames. Its last step is the statement
on an evaluated pair after *Outer links inside a memoized link*, whose
proof is that of *Outer links inside a memoized link* with *Evaluated
pairs of a shared call* in the place of *Nothing in a memoized link is a
head*. That claim holds with case (d), which arises only when the
evaluation $Z$ of a key lies, inside $E_2$, in an iteration of the kind
(3 of *Heads in a parse that keeps records*); the activation that $Z$
holds lies inside $Z$, and so in that iteration.

*The text between that claim and the next.* It applies *Repeats inside a
growth*, in its second case by the definition of the sequence alone, and
the argument of *Repeats* to $w_{\bar{m}}$ and $\hat{\kappa}$, with
$\alpha_{l-1}$ and $\alpha_j$, which the parse evaluates, in the places of
$\alpha$ and $\alpha'$; that $w_{\bar{m}}$ is a committing unit
(*Restarts*); and that a call which the memo answers holds no evaluation,
which the records do not change.

*The frame of an outer link as the later evaluation of a shared call.*
Its proof uses that $Y_{l'}$ is no growth step, from *Outer links across
runs*; that two evaluations of a shared call at one position have one
result (2 of *Restarts in a parse that keeps records*); which of two
evaluations that contain one activation contains the other; that the
calls on a path of evaluations lead from one rule to another, a property
of the grammar; and *Nesting at one end*, with $Y_l$ or $E_1$ as $T$,
which holds $\alpha_{l-1}$, an activation that the parse evaluates. The
text after it uses in addition
[Lemma 2 of the memoization proof](memoization.md#lemma-2-reading-an-open-growth),
which is about the calls that read a growth, none of them inside an
iteration of the kind (*Growths lie outside the recorded walks*); *Calls
that call no rule*; the stores and the answers of the memo; and the
statement on an evaluated pair, with case (d) as above.

*Shared caller frames.* Its proof uses the definition of a shared call,
case 2, that a call which the memo answers holds no evaluation, Lemma 2
of the memoization proof, the order of the starts and the ends of
growths and of $F_1$ and $F_2$, and *Evaluated pairs of a shared call*
through the statement on an evaluated pair. The computes that the text
states of two grammars are measurements of the engine as it is.

*Caller frames in a later step.* Its proof uses *Repeats inside a
growth* as above; containment among $Z_{l'}$, $S$, $\rho$, $M$ and $B$;
*Own regions* and the definition of an evaluation of a key, a call that
reads no growth open at its start; *Growths around a leaf link* and
*Calls that call no rule*; and that the only body evaluation inside
$e'_{l'}$ is that of $r$ or of its twin (6 of *Leaf links in a parse
that keeps records*).

*Nesting at one end.* $X$ and $T$ are evaluations that the parse makes,
and so is every evaluation on the path from $X$ to $T$. Its proof uses
that an evaluation which succeeds and is kept lies in the match of the
one above it; *Restarts*, *Dropped steps* and the conclusion of *Failure
after a source* for the first abandoned evaluation $w$, which holds the
activation of $T$, one that the parse evaluates; that after $w$ the
parser evaluates a continuation $\kappa$ of $w$ from $s$ and returns
before $s$ only in the exception (1 of *Restarts in a parse that keeps
records*); the first characters of $w$ and of $\kappa$ after the alike
calls, applied to the input that $T$ and $\kappa$ consume (3 of that
claim); and, of each alike call and of each call $K$, $Q_1, \ldots, Q_m$
through which $\kappa$ reaches $S$, where it ends and that what follows it
matched without consuming input. Of the alike calls, of $S$ and of the
$Q_i$ the proof uses the ends alone, and not that the parse evaluates
them: the memo can answer such a call, and with records an iteration
that holds one can be answered by a record, as when $\kappa$ begins with
a run site of minimum 0 whose element begins with the shared call. (R1)
gives the end of the walk that takes the record, not the end of a call
inside an iteration that the record answers. That end is read in
$\mathcal{R}$: the walk of $\mathcal{R}$ in the state of that walk
evaluates the iteration, with the same end of the walk (*Counterparts*),
and the end of the call is the end of the evaluation that this walk of
$\mathcal{R}$ makes of it. The hypothesis that two evaluations of an
alike call at one position end at one position is then made of that
evaluation and of the other evaluation of the call, or of its
counterpart in $\mathcal{R}$, which has the same end (2 of *Restarts in a
parse that keeps records*). For a call that the unit and the
continuation share, this is case 2 in $\mathcal{R}$, as in that point.
For a call that the comparison of a decided unit passes alike, it is a
further reading of the hypothesis, which the claim states. With the ends
read so, the positions that the proof compares, the ends of the alike
calls in $w$ and in $\kappa$, the end of $S$ at $p_0$ and the ends of
$K$ and of the $Q_i$, are those of the input that the parser passes,
whether it evaluates a call there or a record covers it (3 of that
claim). The call of case (c) holds $T$, and $K$ lies on the path from
$X$ to $w$, so the parse evaluates both.

*The text after Outer links inside a memoized link.* The statement on an
evaluated pair is treated above. That *Replayed activations* charges an
activation held by a memoized evaluation that is no head to the head of
its class is that claim, which with its *Replays* is stated of the
activations that are no iterations of the kind and lie inside none (the
text before *Direct activations*). The last paragraph, on $e'_j$ and
$Y_{l'}$, uses which of two evaluations that contain $\alpha_j$ contains
the other and that $Y_{l'}$ is the last frame that the link $l'$ leaves
(2 of *Leaf links in a parse that keeps records*).

*Memoized links to the frame of an outer link.* That $e_j$ ends at $p$
is case 2. Suppose that $w_j$ evaluates $r$ at $p$ after $e_j$, in an
evaluation $x$. $r$ calls no rule and reads no variable, so
$\mathcal{R}(x)$ takes the steps of $\mathcal{R}(e'_{l'})$, an evaluation
of $r$ or of its twin at $p$ on the same input, and holds an activation
$\beta$ of $\sigma$ at $q$ at the place of the counterpart of
$\alpha_{l'}$, which succeeds as $\alpha_{l'}$ does. $x$ makes no call,
and an alternative that the parse skips holds no activation, so the parse
evaluates the counterpart of $\beta$ in $x$ unless a record answers
$\beta$ or an iteration inside $\mathcal{R}(x)$ that contains it. If the
parse evaluates it, the proof goes on as it stands: that activation
starts at $q$ after $\alpha_{j-1}$ and before $\alpha_j$. Otherwise the
iteration that the record answers is an iteration of a walk of a site
that keeps records, since a walk of it took that record, and it
corresponds, in the same steps, to an iteration of the same repetition
in $\mathcal{R}(e'_{l'})$ that is the counterpart of $\alpha_{l'}$ or
contains it; the parse evaluates $\alpha_{l'}$, so that iteration is the
counterpart of one that the parse evaluates (the paragraph on
*Counterparts* in the proof of *Heads in a parse that keeps records*),
and $\alpha_{l'}$ is an iteration of the kind or lies inside one. This is
the argument of *Memoized links keep positions* in that proof. The
second conclusion uses the nesting of $w_j$, $w_l$ and $Y_l$; that the
shared calls which $w_j$ evaluates before $e_j$ end at or before $s_0$
and that $w_j$ examines no input outside them before $e_j$, properties
of the grammar applied to what the evaluations examine (5 of *Leaf links
in a parse that keeps records*); and *Nesting at one end*. The text
after the claim uses *Growths around a leaf link*, *Restarts* and
*Dropped steps*, and where it repeats the first conclusion, the exception
above goes with it; the compute that it states of a grammar is a
measurement of the engine as it is.

*The heads that hold the activations of leaf links.* The first sentence
of that paragraph rests on *Memoized links keep positions*, which is
shown of an $\alpha_i$ that is no iteration of the kind and lies inside
none, and on *Links to heads*. The rest uses that $P_j$ makes no call (6
of *Leaf links in a parse that keeps records*); *Restarts*, *Dropped
steps* and both cases of *Repeats inside a growth*, the second by the
definition of the sequence; the argument of *The last unit around the
earlier frame* for an evaluation that contains $\rho$; that a body
evaluation which contains a read of a growth open at its start is no
evaluation of a key; the order of the starts and the ends of $H$, $H'$,
$w_j$ and $\alpha_j$; and *Nothing in a memoized link is a head* for
$H'$ as $e'_i$ or inside it, where case (d) arises only when $H'$ lies,
inside $e'_i$, in an iteration of the kind, and then $\alpha_{j'}$, which
$H'$ holds, lies in that iteration.

The proofs use nothing else of the parse. $\square$

**Leaf links in a parse that resumes runs.** In a
[Document](../runtime/documents.md) parse whose records of sites, if it
keeps any, are exact, (R1) of the [Setting](#setting), that resumes runs,
*Leaf links in a parse that keeps records* and *Later leaf links in a
parse that keeps records* hold as they state, with case (e) of the
covered keys beside case (d), a resumable iteration (*Heads in a parse
that resumes runs*) beside an iteration of the kind, and a resumed walk
that reuses an iteration beside a record that answers one. In a parse
that keeps no record of any site, case (d) and the iterations of the kind
do not arise, and the claims on leaf links that those two claims cover
hold with case (e) and the resumable iterations alone. Where the
innermost evaluation that contains the two activations of a link is a
growth, they rest on the first case of *Repeats inside a growth* of
Lemma 5, which *Repeats inside a growth in a parse that resumes runs*
shows of such a parse. *Calls of scan-only rules* and *Seed reads* are
not shown, as there.

*Proof.* The claims are about the activations $\alpha_1, \alpha_2,
\ldots$, their links and units, the frames and the evaluations of calls,
all of which the parse makes; a reused iteration can be an $\alpha_i$,
an activation that makes no call, and a call inside one is none of them,
since the parse evaluates nothing inside a reused iteration. The proofs
of the two claims use the facts that they list, and each holds of a
walk that takes a record of a site as they show and of a walk that
resumes a run as follows. By *Restarts in a parse that resumes runs*,
*Restarts*, *Dropped steps*, *Repeats*, *Direct activations*, *Failure
after a source* and *Memoized links* hold of the parse, with where the
parser goes back, the exception of the restarts included, the success
and the end of its evaluations, and what the input that a unit consumes
gives (1 to 3 of that claim); and *Repeats inside a growth in a parse
that resumes runs* gives the first case of *Repeats inside a growth*.

1. *Order and nesting* (2 of *Leaf links in a parse that keeps
   records*). A resumed walk moves the parser over a reused iteration
   from its start to its end, starts no evaluation inside it and ends
   none but the walk itself; it is an ordinary evaluation in every other
   respect.
2. *What a growth does* (3 of that claim). A walk is no growth and no
   step of one, the parse makes no growth step, no evaluation in a
   context of growths, no call that starts a growth and no call that reads
   one inside a reused iteration, since it evaluates nothing there, and
   the counterpart in $\mathcal{R}$ of a reused iteration reads the
   current result of no growth ([Lemma
   4](documents.md#lemma-4-recorded-iterations-read-no-open-growth) of the
   Documents proof and its proof, as in 1 of *Covered keys through resumed
   runs*). The steps of a growth around a resumed walk have the outcomes
   and the ends of their counterparts in $\mathcal{R}$ (2 of *Restarts in
   a parse that resumes runs*), and the parse grows and answers a leader
   as without resumption, as in 1 of *Repeats inside a growth in a parse
   that resumes runs*. The same holds where the text after *The frame of
   an outer link as the later evaluation of a shared call* uses [Lemma 2
   of the memoization proof](memoization.md#lemma-2-reading-an-open-growth)
   of the calls that read a growth: none of them lies inside a reused
   iteration.
3. *Places of a body* (4 of that claim). The evaluations that a body
   evaluation of the parse makes, the reused iterations among them, are
   the counterparts of distinct evaluations of its counterpart in
   $\mathcal{R}$, at the same positions (*Counterparts in the reference
   parse*), so it reaches a place at $p$ no more often than its
   counterpart does.
4. *Properties of the grammar* (5 of that claim). A resumed walk ends
   where its counterpart in $\mathcal{R}$ ends, with its success, and the
   parser passes over a reused iteration the input that its counterpart
   consumes and examines no input there (2 and 3 of *Restarts in a parse
   that resumes runs*). So an evaluation that contains a resumed walk
   consumes the input that its counterpart in $\mathcal{R}$ consumes, ends
   where it ends, and examines a subset of what it examines; the
   properties are equalities of consumed input and of ends, or upper bounds
   on what an evaluation examines, so each holds of the evaluation as of
   its counterpart. The examined range that the walk records into the
   enclosing evaluation, which can be larger (Lemma 3 of the Documents
   proof), only decides reuse and is not read here.
5. *The rule of the link* (6 of that claim). That $r$ calls no rule and reads
   no variable is of the grammar, and while an evaluation of a call of
   $r$ is in progress the parser starts only evaluations inside the body
   of $r$, a resumed walk among them, which starts none inside the
   iterations that it reuses.
6. *The covered keys and the heads.* Where the two claims hold with
   case (d) beside cases (a) and (b), and case (d) arises only for an
   activation in an iteration of the kind, *Heads in a parse that resumes
   runs* gives the same of case (e) with a resumable iteration, by 1 to 3
   of *Heads in a parse that keeps records* with a resumable iteration in
   the place of an iteration of the kind; and *Memoized links keep
   positions* holds of an $\alpha_i$ that is no resumable iteration and
   lies inside none (the same claim).
7. *Ends read in the reference parse.* Where *Nesting at one end* reads
   in $\mathcal{R}$ the end of an alike call, of $S$ or of a call through
   which $\kappa$ reaches $S$ that lies in an iteration that a record
   answers, the same reading serves a call that lies in a reused
   iteration: the walk of $\mathcal{R}$ in the state of the resumed walk
   evaluates that iteration, with the same end of the walk
   (*Counterparts*), and the end of the call is the end of the evaluation
   that this walk of $\mathcal{R}$ makes of it.
8. *Memoized links to the frame of an outer link.* Where the proof in
   *Later leaf links in a parse that keeps records* lets a record answer
   $\beta$ or an iteration inside $\mathcal{R}(x)$ that contains it, a
   resumed walk inside $x$ can reuse such an iteration instead. That
   iteration is then resumable, so the iteration of the same repetition
   in $\mathcal{R}(e'_{l'})$ to which it corresponds in the same steps,
   which is the counterpart of $\alpha_{l'}$ or contains it, is an
   iteration of a walk of that repetition, in its rule or in the twin,
   and $\alpha_{l'}$ is a resumable iteration or lies inside one; this is
   the argument of *Memoized links keep positions* in the proof of *Heads
   in a parse that resumes runs*. The first conclusion then has the
   exception that 3 of *Later leaf links in a parse that keeps records*
   states, with a resumed walk that reuses the iteration beside a record
   that answers it. The branch of a record stays as there: a repetition
   that the Document resumes is not walked as a run site in that parse
   ([Scope](run-sites.md#scope) of the Run Sites proof), so an iteration
   that a record answers is of a site that keeps records and not
   resumable, and the two branches do not meet in one iteration.
9. *Replayed activations.* Where the text after *Outer links inside a
   memoized link* charges an activation to the head of its class, the
   charge holds of an activation that is no resumable iteration and lies
   inside none, as *Replayed activations* does in a Document parse.

The proofs use nothing else of the parse. $\square$

**Activations held by heads.** Let no link of $\sigma$ at $q$ have case (a) or
(b) of the covered keys on its path, and let every free $\alpha_i$ other than $\alpha_1$ be the
counterpart of an activation of $\mathcal{R}$ that has a reference link with
neither case (a) nor (b) on its path, or that a dropped step at the position of its growth
or a path that enters a growth again of $\mathcal{R}$ leads to. Then, by the replayed
activations and since a class has at most six
memoized evaluations, the successful activations of
$\sigma$ at $q$ held by memoized evaluations are at most six times those
held by heads, which are $\alpha_1$, those that case 5 or a dropped step at the position of its growth
repeats, and the free ones that a dropped step at the position of its growth
or a path that enters a growth again of $\mathcal{R}$ leads to, which step 1 of [Open Steps](#open-steps)
leaves to count. The free ones whose reference link meets case (a) or (b) are
outside the hypothesis, and step 1 leaves them open too. Every activation owned by a memoized evaluation is held by it, so
these bound the successful activations of $\sigma$ at $q$ that memoized
evaluations own, apart from those that case 5 or a dropped step at the position of its growth
repeats, which Lemma 5 counts. The comparison with $\mathcal{R}$ needs no order between the
lookahead states of $e_i$ and $e'_i$ and no common variant, and passes through
the growths that $e'_i$ makes again in lookahead state $0$ after $e_i$ made
them in state $1$, through those that the leader of the link makes again there, and
through the growths open at the start of both evaluations of the link.
In a parse that keeps records, the factor six is that of *Replayed
activations*, so the bound is of the activations of $\sigma$ at $q$ that
are not iterations of a walk of a site that keeps records and lie inside
none; by *Heads in a parse that keeps records* no link of such an
activation has case (d) of the covered keys on its path, and by *Covered
keys through records* a free one whose reference link has it is held by no
head unless case (a) or (b) arises on the path of that link followed
through the records, where the records are made as that claim asks; the
hypothesis on the free ones is then read with those paths. The activations of a source without records inside such an
iteration, and the
iterations themselves of a site that keeps records and is no recorded site
of the parse, are not bounded here
(step 8 of [Open Steps](#open-steps)); *Descent in the reference parse*
above bounds, of the first, those that memoized evaluations own, by the
roots of their counterparts in the reference parse. In a
[Document](../runtime/documents.md) parse, the factor six is likewise that
of *Replayed activations*, so the bound is of the activations that lie
inside no iteration of a walk of a repetition, in its rule or in the twin, at which a walk of the parse
resumes a run. By *Heads in a parse that resumes runs* no link of such an
activation has case (e) of the covered keys on its path, and by *Covered
keys through resumed runs* a free one whose reference link has it is held
by no head unless case (a) or (b) arises on the path of that link
followed through the records and the resumed runs, an iteration that
comes from an earlier parse being case (a); the hypothesis on the free
ones is then read with those paths. The bound uses none of the claims on
leaf links or on the scans. The activations inside such an iteration
that memoized evaluations own are bounded by *Descent in the reference
parse* and *Heads without the records*, which hold whatever runs the
parse resumes.

This gives the case that the comparison misses without the classes: in
`main = (-r "x" / r "y")* $$; r = [a-c]* t?; t = "d"` on `abcy`, the first
iteration at $0$ is direct in the evaluation of the twin of `r` at $0$, made by
`-r`, and the second in the evaluation of `r` at $0$, made by `r "y"` after the
unit `-r "x"` fails. Their keys differ in variant, so each evaluation is the
first of its key, but the second is not the first of the class, and its
activations are charged to those of the first.

Two parts remain part of step 1 of [Open Steps](#open-steps). When $R$ calls
no rule and is not memoized at its calls, the $\alpha_i$ held by heads are at
most the free ones, $\alpha_1$ among them, the links with case (a) or (b) on
their path and the leaf links that lead to one, and a leaf link can leave a
memoized evaluation that is not a head through the end of its body and reach
the call again in the evaluation around it, which can be a head. A head can
be reached again in this way more than once at one position, through frames
that are heads themselves. In
`main = "[" (Y1 ws "x" / Y2 ws "z" / Y3 ws "y") / Q / P`, with
`Q = "!" Y1 ws "x" Y2 ws "z" Y3 ws "y"`, `Y1 = S (ws ";")?` and `Y2` and `Y3`
alike, `S = "a"+`, `ws = [ ]*` and `P` as in *Outer links across runs*,
which the conditions accept, each of `Y1`, `Y2` and `Y3` calls a rule and is
called from two places, in `main` and in `Q`, so the parse memoizes it
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)); this
is decided by the grammar, whether or not a parse evaluates `Q`. On `[`
followed by `a`s, spaces and `y`, the parse evaluates `Y1`, `Y2` and `Y3` in
turn at the first `a`. Each of these evaluations matches the `a`s, and its
option then evaluates `ws` at the first space, which scans all the spaces,
and the option fails at `y`, so the evaluation ends at the first space. The leaf link
from that scan leaves it there and reaches the call of `ws` that follows it
in the body of `main`, which scans the spaces again. So at each space three
activations of the repetition of `ws` are held by the evaluation where the
parse begins, a head, each after a leaf link that leaves a different
evaluation of a `Yi`. Each of those evaluations is the first of its class,
and so a head too; the case in which the frame that the link leaves is not
a head has no witness here. In this grammar the number of these activations
at each space is the number of the three alternatives, by its construction;
the compute of a recognition on $n$ `a`s and $n$ spaces, $16n + 228$ on the
closure backend and $16n + 236$ on the bytecode backend for $n$ from $250$ to
$2000$, is consistent with it. No grammar that the conditions accept is
given here in which that number grows
with the input. And in a Document parse a link can
meet an entry of an earlier parse that answers a call in a context of growths,
or a call whose corresponding call in the later evaluation is in a smaller
lookahead state, in a context of growths or of the other variant, and a link
that meets a result kept in a context of growths can lead into a lookahead that
the result observed; such a link can lead to an activation held by a head.

**Iterations at one scan.** Let $\alpha_1, \alpha_2, \ldots$, the units
$w_i$, the continuations $\kappa_i$ and the links be as in *Repeats*, outside
case 5, the third case of *Dropped steps* and the repeats at the position of
a growth that *Repeats inside a growth* of Lemma 5 leaves out. Let $T$ be an
evaluation of a repetition, and call an iteration of $T$ **reaching** when
it contains one of the successful activations $\alpha_1, \alpha_2, \ldots$
of $\sigma$ at $q$. Let $it_k$ and $it_{k'}$, $k < k'$, be consecutive
reaching iterations of $T$, starting at $s_k < s_{k'}$, let $\alpha_{i-1}$ be
the last activation in $it_k$, and let the first abandoned evaluation below
$T$ on the path to $\alpha_{i-1}$ be a unit, with no growth on the path from
$T$ down to it. Then:

1. $\alpha_i$ is the first activation in $it_{k'}$, the innermost evaluation
   $N_i$ that contains $\alpha_{i-1}$ and $\alpha_i$ is $T$, and $w_i$, that
   unit, lies strictly inside $it_k$.
2. If $i$ is linked, $e'_i$ lies inside $it_{k'}$, so $s_{k'} \le t_i$, the
   position of $e_i$ and $e'_i$. Let $c_1, \ldots, c_j$ be the calls that
   $w_i$ shares with $\kappa_i$ up to the call of $e_i$, evaluated in $w_i$
   at $\tau_0 = $ the start of $w_i$, $\tau_1, \ldots, \tau_{j-1} = t_i$, and
   call $B_i = \{\tau_0, \ldots, \tau_{j-1}\}$ the **boundaries** of the link.
   Every iteration $it_x$ with $k \le x < k'$ that ends after the last start
   of $\kappa_i$ ends at a position in $B_i$, and so does $it_k$ when
   $\kappa_i$ last starts inside it; in particular $s_{k'} \in B_i$, whatever
   iterations that reach nothing lie between $it_k$ and $it_{k'}$.
3. If $i$ is linked, then either $e'_i$ succeeds and no evaluation from
   $it_{k'}$ down to $e'_i$ is abandoned, and then $it_{k'}$ is the last
   reaching iteration of $T$ when $e'_i$ ends after $q$, and $e'_i$ is a
   **$q$-frame**, an evaluation of a call that succeeds, ends at or before
   $q$ and contains a successful activation of $\sigma$ at $q$, when it ends
   at or before $q$; or an evaluation from $it_{k'}$ down to $e'_i$ is
   abandoned, and then, when $\alpha_i$ is also the last activation in
   $it_{k'}$ and a later reaching iteration follows such that $it_{k'}$ and
   that iteration meet the hypothesis above, with the first abandoned
   evaluation below $T$ on the path to $\alpha_i$ a unit, $w_{i+1}$ contains
   $e'_i$, and $e_{i+1}$ contains $e'_i$ or lies inside it, the second only
   when the rule of $e'_i$ calls a rule. At most two iterations of $T$ are
   charged to one $q$-frame position by the first case: those whose start
   and end enclose it.
4. Let $it_{k_1}, \ldots, it_{k_n}$ be consecutive reaching iterations, with
   $i_a$ the index of the first activation in $it_{k_{a+1}}$, each meeting
   the hypothesis above, no growth on the paths from $T$ down to the
   evaluations of the shared calls of the units and of their continuations
   up to $e'_{i_a}$, each $i_a$ linked with $e_{i_{a+1}} = e'_{i_a}$ for
   $a < n - 1$, and no shared call of $w_{i_{a+1}}$ before $e_{i_{a+1}}$
   inside the evaluation that $\kappa_{i_a}$ makes of a shared call of
   $w_{i_a}$ (**no refinement**). Then $B_{i_{a+1}} \subseteq B_{i_a}$ for
   each $a$, and $n - 1$ is at most the number $j$ of shared calls of
   $w_{i_1}$ up to the call of $e_{i_1}$: $n \le 2$ when that call is the
   first.

*Proof.* The activations start in the order of their indices, and the
iterations of $T$ follow one another in time, so the activations in $it_k$
have consecutive indices, and since the iterations between $it_k$ and
$it_{k'}$ reach nothing, the first activation in $it_{k'}$ is $\alpha_i$.
The two lie in different children of $T$, so $N_i = T$. $T$ evaluates
$it_{k'}$ after $it_k$, so $it_k$ succeeded; it is not the expression of a
lookahead, and by the hypothesis no dropped step lies on the path, so the
first abandoned evaluation below $T$ on the path to $\alpha_{i-1}$ lies
strictly inside $it_k$, and it is $w_i$ (*Restarts*, *Repeats*). This is 1.

For 2, $e'_i$ starts after $w_i$ ends, while $T$ is in progress, so it lies
inside $T$; it contains $\alpha_i$, which lies in $it_{k'}$, and as an
evaluation of a call it is no iteration of $T$, so it lies inside $it_{k'}$,
and $s_{k'} \le t_i$. After an iteration $it_x$ of $T$ ends, the parser
starts no evaluation before $\mathrm{end}(it_x)$ until $\alpha_i$: an
evaluation in progress that started before that end encloses $it_x$ or ended
inside it, so it is $T$ or encloses $T$, which contains $\alpha_i$ and is not
abandoned before it; the parser returns to an earlier position only by
abandoning such an evaluation or by a later step of a growth that encloses
$it_x$, which encloses $T$ and starts its later step only after its step
that holds $T$ ends (*Restarts*). So the last start of $\kappa_i$, at
$\tau_0$, is inside $it_k$ or at its end, and from it to $e'_i$ the parser
keeps, on its way to $e'_i$, the evaluations of $c_1, \ldots, c_{j-1}$ at
$\tau_0, \ldots, \tau_{j-2}$, which end at $\tau_1, \ldots, \tau_{j-1}$, and
examines no other input (*Direct activations*: every path of $\kappa_i$ that
examines the input begins with the shared calls, each with the result that
it has in $w_i$). An iteration $it_x$ that ends in that time contains every
evaluation of a call that starts in it, and is contained in none, since its
parent is $T$; so it ends between two of these evaluations, at a position of
$B_i$. This argument does not use the order of kept evaluations that
the proof of 4 shows.

For 3, in the first case $e'_i$ lies in the match of $it_{k'}$, which ends
at or after $\mathrm{end}(e'_i)$. If $e'_i$ ends after $q$, so does
$it_{k'}$, and every later iteration starts at or after the end of $it_{k'}$,
after $q$, and contains no activation at $q$. If $e'_i$ ends at or before
$q$, it is a $q$-frame, since it succeeds and contains $\alpha_i$, whether
$it_{k'}$ ends after $q$ or not; when it does, $it_{k'}$ is also the last
reaching iteration, which the claim does not need. In the second case,
under its condition, the first abandoned evaluation below $T$ on the path to
$\alpha_i$ is below $it_{k'}$, which succeeded, and at or above $e'_i$, since
one of those is abandoned; it is $w_{i+1}$ by 1 applied to $it_{k'}$ and the
next reaching iteration, which meet the hypothesis by that condition. So
$w_{i+1}$ contains $e'_i$, and $e_{i+1}$, an evaluation of a call that
contains $\alpha_i$ (*Repeats*), contains $e'_i$ or lies inside it, and then
it is a call made in the body of the rule of $e'_i$. The iterations of $T$
whose start and end enclose a position $t$ satisfy $s_x \le t \le
\mathrm{end}(it_x) = s_{x+1}$ for all but the last, and the starts increase,
so there are at most two.

For 4, an evaluation $x$ is **kept before** an evaluation $z$ when $x$ ends
before $z$ starts and no evaluation from the child, of the innermost
evaluation $A$ that contains both, that contains $x$ down to $x$ is
abandoned. If $x$ and $y$ are kept before $z$ and $y$ starts after $x$
ends, $y$ starts at or after $\mathrm{end}(x)$: the evaluations in progress
at the end of $x$ that started before it are the evaluations from $x$ up to
$A$, which are not abandoned and end before $z$, and those that enclose $A$,
which enclose $z$; with no growth on these paths the parser returns before
$\mathrm{end}(x)$ through neither. So two such evaluations nest or have
spans in their order, the earlier ending at or before the start of the
later. The evaluations of $c_1, \ldots, c_{j-1}$ that $\kappa_{i_a}$ keeps on
its way to $e'_{i_a}$, and those of the shared calls of $w_{i_{a+1}}$ before
$e_{i_{a+1}} = e'_{i_a}$, are kept before $e'_{i_a}$. These are the
evaluations on the path to $e_{i_{a+1}}$, not ones in an alternative that
was abandoned before it: every path of $w_{i_{a+1}}$ begins with its shared
calls, so the path to $e_{i_{a+1}}$ passes an evaluation of each of them, at
the positions where $w_{i_{a+1}}$ evaluates them. In the same way the
evaluation $c$ of a shared call of $w_{i_a}$ named below is the one that
$\kappa_{i_a}$ keeps on its way to $e'_{i_a}$. $w_{i_{a+1}}$ lies in
$it_{k_{a+1}}$, which starts at $s_{k_{a+1}} \in B_{i_a}$ by 2, so every
boundary $\tau'$ of $w_{i_{a+1}}$, the start or end of the evaluation $d$ of
one of its shared calls, lies between $\tau_0$ and $\tau_{j-1} = t_{i_a}$
of $B_{i_a}$. If $\tau'$ lay strictly between two consecutive members of
$B_{i_a}$, the boundaries of the evaluation $c$ that $\kappa_{i_a}$ makes of
one shared call, $d$ and $c$ would not have spans in their order, so they
would nest, and $d$ would not contain $c$, since $\tau'$ lies strictly
inside the span of $c$; so $d$ would lie inside $c$, which the hypothesis
of no refinement excludes. So $B_{i_{a+1}} \subseteq B_{i_a}$, and the starts
$s_{k_2} < \cdots < s_{k_n}$ are distinct members of $B_{i_1}$, which has $j$
members. $\square$

Call a **run** a maximal sequence of consecutive reaching iterations of
$T$ that meets the hypotheses of 4. The claim bounds a run of linked
reaching iterations of one repetition at one position without the growth
hypothesis of *Calls that call no rule*,
and whether or not the shared call is the first, but only inside $T$; the
frames left outside it stay as in step 1 of [Open Steps](#open-steps). Five
parts of a bound on the reaching iterations of $T$, and through them on the
activations that one head holds, are not shown. A refinement can occur when
the evaluation of a shared call in $\kappa_{i_a}$ has calls or literals
inside it that $w_{i_{a+1}}$ shares by looking through it: a literal in the
body of a rule that calls no rule, or a memoized call evaluated again, in a
lookahead state or variant other than that of its first evaluation, instead
of answered; that the depth of refinement is a constant is not shown. A
**memoized break**, the second case of 3 in which $e_{i+1}$ strictly
contains $e'_i$, is placed by *Memoized breaks* below: a run continues
through it only at a member of the boundaries of its first link and to a
later evaluation of its class, but the number of runs in $T$ is not
bounded. The first case of 3 charges iterations to $q$-frame positions, at
most two to each, but the number of distinct $q$-frame positions per $q$ is
not bounded here either. When a reaching iteration holds more than one
activation, $e$ of its last need not be $e'$ of its first, and that case is
not followed. And the hypothesis leaves out dropped steps and growths on the
path from $T$ down to the unit, case 5, the exception of the restarts, and
the cases of a Document parse.

**Iterations at one scan in a parse that keeps records.** In a parse
whose records are exact, (R1) of the [Setting](#setting), let the
iterations of $T$ be those that the parse evaluates; a reaching iteration
is one of them, since it contains an activation. Then 1 and 3 of
*Iterations at one scan* hold as stated, and so does the first statement
of 2, that $e'_i$ lies inside $it_{k'}$ and $s_{k'} \le t_i$, for every
link. Say that a link **meets the hypothesis** when no shared call of the
unit $w_i$ up to the call of $e_i$ lies, in $w_i$ or on the way of
$\kappa_i$ from its last start to $e'_i$, in an iteration that a record
answers. The rest of 2 holds as stated for a link that meets the
hypothesis, and 4 for a run all of whose links meet it, the link of each
$i_a$ of the run. For the other links the rest of 2 is not shown, nor is
4 for a run that holds one of them.

*Proof.* When $T$ is a walk that takes a record, it evaluates its
iterations first, each from the end of the one before, and after the
record it evaluates none, so the iterations that it evaluates follow one
another in time and in the input as those of a repetition without
records do, and each of them but the last consumed input and succeeded
(7 of *Restarts in a parse that keeps records*). The proof uses the
following of the parse.

1. For 1: the order in which the parser starts the activations and the
   iterations of $T$, that $T$ evaluates an iteration after $it_k$ only
   when $it_k$ succeeded, and *Restarts* and *Repeats*, which hold
   (*Restarts in a parse that keeps records*).
2. For 3: the first statement of 2, that $e'_i$ lies inside $it_{k'}$,
   which uses only that $e'_i$ starts after $w_i$ ends and inside $T$,
   that it contains $\alpha_i$ and that an evaluation of a call is no
   iteration of $T$, and so holds for every link; in its second case,
   *Repeats* and 1 of the claim, which hold as 1 above shows;
   that an evaluation which succeeds and is not abandoned lies in
   the match of the one above it and ends no later (1 of that claim);
   that an iteration after $it_{k'}$ starts at or after its end; which of
   two evaluations that contain $\alpha_i$ contains the other; and that
   the evaluated iterations of $T$ but the last end where the next
   starts, with starts that increase, which the paragraph above gives.
3. For the rest of 2: that $e'_i$ starts after $w_i$ ends and inside $T$; that after
   an iteration of $T$ ends the parser starts no evaluation before its
   end until $\alpha_i$, which rests on where the parser goes back (1 of
   that claim: a walk that takes a record adds no return); and, from
   *Direct activations*, that from the last start of $\kappa_i$ to $e'_i$
   the parser keeps the evaluations of $c_1, \ldots, c_{j-1}$ and
   examines no other input. Under the hypothesis each of those calls is
   evaluated, or answered by the memo, in $w_i$ and on that way, at the
   positions and with the ends that the proof names. A walk that takes a
   record on that way, outside the evaluations of those calls, moves the
   parser over the input that the iterations
   which the record answers would consume (3 of that claim), and those
   iterations would consume it through shared calls, since that way
   examines no other input; by the hypothesis no shared call lies in
   them, so the record covers no input, and the parser moves from
   $\tau_0$ to $t_i$ through the evaluations of the shared calls alone.
   So an iteration of $T$ that ends in that time ends between two of
   them, as the proof states.
4. For 4: the order of the evaluations that are kept before another,
   which rests on where the parser goes back, as in 3; 2 of the claim,
   under the hypothesis; and that the evaluations of the shared calls of
   $w_{i_{a+1}}$ before $e_{i_{a+1}}$ and those that $\kappa_{i_a}$
   keeps on its way to $e'_{i_a}$ are evaluations of calls that the parse
   makes, which the hypothesis gives for both links, so for every pair
   of consecutive links of the run when all its links meet it.

The proof uses nothing else of the parse. $\square$

Without the hypothesis, a shared call of $w_i$ can lie in an iteration
that a record answers, in $w_i$ before $e_i$ or on the way of $\kappa_i$,
when the unit and its continuation begin with a repetition whose
iterations hold their shared calls. The boundaries $B_i$ are then
positions at which the parse makes no evaluation of a call, and the
hypothesis of no refinement in 4, which compares evaluations, is not
stated for such a call. The claims that follow, from *Memoized breaks*
to *Consecutive activations in one own region*, rest on 1 to 4 and have
not been read again for a parse that keeps records.

**Iterations at one scan in a parse that resumes runs.** In a
[Document](../runtime/documents.md) parse whose records of sites, if it
keeps any, are exact, (R1) of the [Setting](#setting), that resumes runs,
let the iterations of $T$ be those that the parse makes, those that a
resumed walk reuses included. Then *Iterations at one scan in a parse that
keeps records* holds as stated, with a link that **meets the hypothesis**
when no shared call of the unit $w_i$ up to the call of $e_i$ lies, in
$w_i$ or on the way of $\kappa_i$ from its last start to $e'_i$, in an
iteration that a record answers or that a resumed walk reuses. In a parse
that keeps no record of any site, only the reused iterations enter the
hypothesis.

*Proof.* A reused iteration contains no activation other than itself, so a
reaching iteration is an evaluated iteration of $T$ or a reused iteration
that is itself one of $\alpha_1, \alpha_2, \ldots$, an activation that
the parse makes. When $T$ is a walk that resumes a run, the iterations
that it reuses and those that it evaluates are those that its counterpart
in $\mathcal{R}$ visits, in the same order, each starting where the one
before ended, with starts that increase, and each of them but the last
consumed input and succeeded (the paragraph before 1 and 7 of *Restarts in
a parse that resumes runs*), so they follow one another in time and in the
input as the iterations of a repetition that reuses none do; when $T$
takes a record of a site, the proof of *Iterations at one scan in a parse
that keeps records* treats it. A reused reaching iteration holds the claim
only vacuously. If $it_k$ is one, it is $\alpha_{i-1}$, and no evaluation
below $T$ on the path to it is abandoned, so the hypothesis of the claim
fails. If $it_{k'}$ is one, it is $\alpha_i$, and $i$ is not linked:
$e'_i$ would contain $\alpha_i$ and lie inside $it_{k'}$ by the first
statement of 2, so 2 to 4 say nothing of that index. The facts 1 to 4 of
that proof then hold of the parse as follows.

1. The order in which the parser starts the activations and the
   iterations of $T$, the reused ones included, is that order; $T$ makes
   an iteration after $it_k$ only when $it_k$ succeeded, a reused
   iteration being one that succeeded; and *Restarts* and *Repeats* hold
   (*Restarts in a parse that resumes runs*).
2. The first statement of 2 uses only that $e'_i$ starts after $w_i$ ends
   and inside $T$, that it contains $\alpha_i$ and that an evaluation of a
   call is no iteration of $T$, and so holds for every link; the rest of
   what 3 uses holds by 1 above and by the order and the ends of the
   iterations of $T$ given above.
3. For the rest of 2: after an iteration of $T$ ends, the parser starts
   no evaluation before its end until $\alpha_i$, since a resumed walk
   adds no return (1 of *Restarts in a parse that resumes runs*). Under
   the hypothesis each of the calls $c_1, \ldots, c_{j-1}$ is evaluated,
   or answered by the memo, in $w_i$ and on the way of $\kappa_i$, at the
   positions and with the ends that the proof names. A resumed walk on
   that way, outside the evaluations of those calls, moves the parser
   over the input that the counterparts of the iterations that it reuses
   consume (3 of that claim), and those would consume it through shared
   calls, since that way examines no other input; by the hypothesis no
   shared call lies in them, so they cover no input, and the parser moves
   from $\tau_0$ to $t_i$ through the evaluations of the shared calls
   alone.
4. For 4: the order of the evaluations that are kept before another, as
   in 3, 2 of the claim under the hypothesis, and that the evaluations of
   the shared calls of $w_{i_{a+1}}$ before $e_{i_{a+1}}$ and those that
   $\kappa_{i_a}$ keeps on its way to $e'_{i_a}$ are evaluations of calls
   that the parse makes, which the hypothesis gives for both links.

The proof uses nothing else of the parse. $\square$

Without the hypothesis, a shared call of $w_i$ can lie in a reused
iteration as in an iteration that a record answers, and the claims from
*Memoized breaks* to *Consecutive activations in one own region*, which
rest on 1 to 4, have not been read again for a parse that resumes runs
either.

**Memoized breaks.** Under the hypotheses of *Iterations at one scan*, let
$i$ be linked, let an evaluation from $it_{k'}$ down to $e'_i$ be abandoned,
let the condition of the second case of 3 hold, and let $e_{i+1}$ strictly
contain $e'_i$. Then:

1. $e_{i+1}$ is an evaluation of a call of a rule that the parse memoizes
   at that call, and when it is an evaluation of a key, no evaluation of a
   key that strictly contains it holds $\alpha_i$.
2. $t_{i+1} \in B_i$.
3. When the memo answers the call that $\kappa_{i+1}$ makes at $t_{i+1}$,
   $i + 1$ is free. Otherwise, when $i + 1$ is linked, $e'_{i+1}$ is a
   later evaluation of the rule of $e_{i+1}$ or its twin at the same
   position and level, and when both are evaluations of keys, $e'_{i+1}$
   is no head.
4. Item 4 of *Iterations at one scan* holds with "$e_{i_{a+1}}$ contains
   $e'_{i_a}$" in place of "$e_{i_{a+1}} = e'_{i_a}$".

*Proof.* By *Repeats*, $e_{i+1}$ is the evaluation inside $w_{i+1}$ of a
call that $w_{i+1}$ and $\kappa_{i+1}$ share. It strictly contains
$e'_i$, the evaluation of a call, so it is no literal and its rule calls a
rule, and a shared call of such a rule is of a rule that the parse
memoizes at the call (the definition of sharing in [Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost), with [Evaluations
of Rules](../runtime/parsing.md#evaluations-of-rules) for the calls at which
a rule is memoized). An evaluation of a key inside another lies outside
the own region of the other (*Keys and classes*), so when $e_{i+1}$ is an
evaluation of a key, $\alpha_i$, which lies in it, lies in the own region of
none that strictly contains it. This is 1.

For 2, $e_{i+1}$ lies in $w_{i+1}$, which lies inside $it_{k'}$ by 1 applied
to $it_{k'}$ and the next reaching iteration, so it starts after $it_k$
ends. $\kappa_i$ starts from the start of $w_i$ when $w_i$ is abandoned,
while $it_k$, which contains $w_i$, is in progress, so its last start is
before $e_{i+1}$ starts. $e_{i+1}$ contains $\alpha_i$, so it is in
progress when $e'_i$ starts. By the proof of 2, from the last start of
$\kappa_i$ to $e'_i$ the parser keeps the evaluations of $c_1, \ldots,
c_{j-1}$, which start at $\tau_0, \ldots, \tau_{j-2}$ and end at
$\tau_1, \ldots, \tau_{j-1}$, and examines no other input, so outside
them it is at $\tau_0$ before the first, at $\tau_y$ between the $y$-th
and the next, and at $t_i$ after the last. $e_{i+1}$ starts inside none of
them, since an evaluation of a call that starts inside another ends inside
it, and so before $e'_i$ starts. So $t_{i+1}$ is a member of $B_i$.

For 3, when the memo answers the call, $\kappa_{i+1}$ makes no evaluation of
it at $t_{i+1}$, so $\alpha_{i+1}$ lies in none and $i + 1$ is free. When
$i + 1$ is linked, $e'_{i+1}$ is the evaluation of the shared call that
$\kappa_{i+1}$ makes at $t_{i+1}$, after $e_{i+1}$: a call of the same rule,
or of its twin, with the same level restriction, of a rule that reads no
variable, so the key of $e'_{i+1}$ has the class of the key of $e_{i+1}$,
and when both are evaluations of keys, $e_{i+1}$ is an earlier evaluation
of a key of that class.

For 4, let $x$ be the evaluation of a shared call of $w_{i_{a+1}}$ before
$e_{i_{a+1}}$, kept before $e_{i_{a+1}}$. It is kept before $e'_{i_a}$
too: it ends before $e'_{i_a}$ starts, and the evaluations that contain
$e'_{i_a}$ but not $e_{i_{a+1}}$ lie inside $e_{i_{a+1}}$, which $x$ does not
meet, so the innermost evaluation that contains $x$ and $e'_{i_a}$ is the
one that contains $x$ and $e_{i_{a+1}}$. The members of $B_{i_{a+1}}$ are
at most $t_{i_{a+1}} \le t_{i_a}$, since $w_{i_{a+1}}$ evaluates its shared
calls one after another at its boundaries, and $t_{i_{a+1}}$ is a member of
$B_{i_a}$ by 2. With these, the proof of 4 of *Iterations at one scan*
applies to the boundaries that are starts or ends of such an $x$, and so
$B_{i_{a+1}} \subseteq B_{i_a}$. $\square$

So a memoized break holds $\alpha_i$ through the link $i$, which enters the
evaluation of its call, and no link leaves that evaluation. Of the
activations of $\sigma$ at $q$, $e_{i+1}$ contains only $\alpha_i$:
$\alpha_{i-1}$ lies in $w_i$, which is abandoned before the last start of
$\kappa_i$ and so before $e_{i+1}$ starts, and $\alpha_{i+1}$ starts after
$w_{i+1}$, which contains $e_{i+1}$, is abandoned. A frame that a link $m$
leaves is an evaluation that encloses $w_m$ on the path to $\alpha_{m-1}$
(*Calls that call no rule*, as in *Outer links across runs*), so it
contains $\alpha_{m-1}$; were it $e_{i+1}$, $m$ would be $i + 1$, but a
frame of the link $i + 1$ strictly contains $w_{i+1}$ and so $e_{i+1}$.
With 4 read with containment, a run goes on through a break only to a later
evaluation of the class of the break, and its breaks fall at members of the
boundaries of its first link, at most $j$ positions; a break whose call the
memo answers ends the run at a free index. So the breaks of a run are
charged to the run and
not to the memo; the number of runs in $T$, which begin at the first
reaching iteration of $T$, at a free index, after a $q$-frame or where
$e_{i+1}$ lies strictly inside $e'_i$, is not bounded here.

**Restarts through a frame.** Under the hypotheses of *Iterations at one
scan*, let $i$ be linked, let $e'_i$ succeed with no evaluation from
$it_{k'}$ down to $e'_i$ abandoned and end at or before $q$, so that it is a
$q$-frame (the first case of 3), and let $\alpha_i$ be the last activation in
$it_{k'}$ and the next reaching iteration $it_{k''}$ follow, such that
$it_{k'}$ and $it_{k''}$ meet the hypothesis of *Iterations at one scan*.
Then:

1. $w_{i+1}$ and $e_{i+1}$ lie strictly inside $e'_i$, and $e'_i$ is an
   evaluation of a call of a rule that calls a rule and that the parse
   memoizes at that call.
2. When $e'_i$ is an evaluation of a key, no evaluation of a key that
   strictly contains it holds $\alpha_i$. The link $i$ is memoized, its
   rule memoized at both calls (*Memoized links*), so
   under the hypothesis on growths of *Links lead to repeated evaluations*
   no evaluation of a key that holds $\alpha_i$ is the head of its class,
   unless case (a) or (b) of the covered keys arises as that claim states.
3. The parser leaves $e'_i$, which succeeds, by its end between the end of
   $w_{i+1}$ and the start of $\alpha_{i+1}$.
4. When $i + 1$ is linked, a sequence of consecutive reaching iterations
   that contains $it_{k'}$ and $it_{k''}$ and meets the hypotheses of 4 of
   *Iterations at one scan*, or those of 4 of *Memoized breaks*, begins at
   $it_{k'}$, and the starts of its iterations after the first are distinct
   members of $B_{i+1}$ at or after $\mathrm{end}(e'_i)$. So it has at most
   two iterations when $t_{i+1} \le \mathrm{end}(e'_i)$.

*Proof.* By 1 of *Iterations at one scan* applied to $it_{k'}$ and
$it_{k''}$, $\alpha_{i+1}$ is the first activation in $it_{k''}$, and
$w_{i+1}$ is the first abandoned evaluation below $T$ on the path to
$\alpha_i$ and lies strictly inside $it_{k'}$. The path passes $e'_i$, which
contains $\alpha_i$, and no evaluation from $it_{k'}$ down to $e'_i$ is
abandoned, so $w_{i+1}$ lies strictly inside $e'_i$. By *Repeats*,
$e_{i+1}$ lies inside $w_{i+1}$, so strictly inside $e'_i$, and it is the
evaluation of a call that contains $\alpha_i$. That call is made inside the
body evaluation of the rule of $e'_i$, directly or inside the evaluation of
another call made there, so the rule of $e'_i$ calls a rule. $e'_i$ is the
evaluation in $\kappa_i$ of a call that $w_i$ and $\kappa_i$ share, and a
shared call of a rule that calls a rule is of a rule that the parse memoizes
at each of the two calls (the definition of sharing in [Conditions for
Linear Cost](../runtime/parsing.md#conditions-for-linear-cost)). This is 1.

For 2, an evaluation of a key inside another lies outside the own region of
the other (*Keys and classes*), so when $e'_i$ is an evaluation of a key,
$\alpha_i$, which lies in it, lies in the own region of none that strictly
contains it. By 1 the rule of the link $i$ is memoized at both calls, and
the rest is *Links lead to repeated evaluations*.

For 3, $e'_i$ succeeds by the hypothesis and contains $w_{i+1}$ by 1, so
it ends after $w_{i+1}$ ends. It lies in $it_{k'}$, while $\alpha_{i+1}$
lies in $it_{k''}$, which starts after $it_{k'}$ ends, so $e'_i$ ends
before $\alpha_{i+1}$ starts.

For 4, let the sequence be $it_{k_1}, \ldots, it_{k_n}$ with $it_{k_a} =
it_{k'}$ and $it_{k_{a+1}} = it_{k''}$. If $a \ge 2$, then $i_{a-1} = i$
and $i_a = i + 1$, the indices of the first activations in $it_{k'}$ and
$it_{k''}$, and $a - 1 < n - 1$, so the hypotheses require that
$e_{i+1}$ be $e'_i$ or contain it, while by 1 it lies strictly inside it.
So $a = 1$ and $i_1 = i + 1$, and by the proof of 4 of *Iterations at one
scan*, which *Memoized breaks* applies to its form, the starts $s_{k_2} <
\cdots < s_{k_n}$ are distinct members of $B_{i+1}$. $e'_i$ lies in the
match of $it_{k'}$, since nothing from $it_{k'}$ down to it is abandoned,
so $s_{k_2} = s_{k''} \ge \mathrm{end}(it_{k'}) \ge \mathrm{end}(e'_i)$.
The largest member of $B_{i+1}$ is $t_{i+1}$, so when $t_{i+1} \le
\mathrm{end}(e'_i)$ at most one member is at or after
$\mathrm{end}(e'_i)$, and $n \le 2$. $\square$

So a run that ends at a $q$-frame, with a later reaching iteration, ends
because the next unit lies inside the $q$-frame, and the restart is a
return through that frame, a frame that the parse memoizes at its call.
When the $q$-frame is an evaluation of a key, the activation of the
iteration that holds it is held inside it, by 2, and so by no evaluation
of a key that contains $T$. Item 4 gives two iterations only when
$t_{i+1} \le \mathrm{end}(e'_i)$: the shared calls of $w_{i+1}$ lie inside
the $q$-frame but in $w_{i+1}$, which is abandoned, so they are not held
to the match of the frame, and the claim does not exclude members of
$B_{i+1}$ after its end. Nor does it bound the number of $q$-frames in $T$,
and so the number of runs.

**Consecutive restarts through frames.** Under the hypotheses of *Restarts
through a frame*, let $i + 1$ be linked and $e_{i+1}$ an evaluation of a key.
Let $e_i$ and $e'_i$ have one variant, the lookahead state of $e_i$ be no
greater than that of $e'_i$, every growth at the position of the link $i$
that is open at the start of $e_i$ or of $e'_i$ be open at the start of both
or be read by neither, and no call be in a context of growths on the way
from $e'_i$, itself included, down to $e_{i+1}$ or on the way from $\mathcal{R}(e_i)$, followed
into earlier evaluations, down to the call that corresponds to $e_{i+1}$.
Then $e'_{i+1}$ has the other variant than $e_{i+1}$, or is in a context of
growths, or $e_{i+1}$ is in lookahead state $1$ and $e'_{i+1}$ in state $0$.

*Proof.* By 1 and 2 of *Restarts through a frame*, $e_{i+1}$ lies strictly
inside $e'_i$, and the link $i$ is memoized, its rule memoized at both calls.
$e_{i+1}$ contains $\alpha_i$ (*Repeats*), so $\mathcal{R}(e_{i+1})$ holds
the counterpart of $\alpha_i$ at some depth. The proof of *Links lead to
repeated evaluations* then applies with $Z = e_{i+1}$: it uses of $Z$ only
that it is an evaluation of a key made inside $e'_i$ whose counterpart holds
the counterpart of $\alpha_i$, so that the call $z^\circ$ at the same place in
$\mathcal{R}(e_i)$ has a key of the class of $e_{i+1}$, reads no growth open
at its start and holds an activation at any depth, and case (c) of the
covered keys does not arise. Case (a) does not arise, by the statement of
that claim, and case (b) arises only at a call that the parse answers from
what it keeps apart from the table, which it keeps only in a context of
growths, and such a call is in a context of growths itself, since by (D) of
the [memoization proof](memoization.md#setting) what it is answered from read
a growth that is open at its position, with the same current result, when it
is answered. So the parse made an evaluation $y$ of the key of $z^\circ$, which
ended before $e_i$ ended. $y$ is evaluated in the lookahead state of the call
on that way that it corresponds to, which the last paragraph of that proof
bounds by the lookahead state of $e_{i+1}$. The variant of each call
on the two ways depends only on its place and on the variant of the body
that makes it (*Counterparts*), and the way follows an answered call only
into an earlier evaluation of its key, so with $e_i$ and $e'_i$ of one
variant, $y$ has the variant of $e_{i+1}$, and its key is the key of
$e_{i+1}$.

$e_{i+1}$ starts after $e'_i$ starts, which is after $w_i$, which contains
$e_i$, is abandoned, so after $y$ ends. $y$ and $e_{i+1}$ are outside every
context of growths, on the two ways. Had $y$ stored its result, the memo
would answer every later call with its key outside contexts of growths in
its lookahead state or a greater one (*Answered calls*), $e_{i+1}$ among
them, which the parse evaluates. So $y$ stored no result, and by the
condition on the memo in [Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost) it is the first
evaluation of its variant at its position, and $e_{i+1}$, a later one there,
stores its result in its lookahead state. $e'_{i+1}$ is the evaluation that
$\kappa_{i+1}$ makes of the call of $e_{i+1}$ at the same position after
$w_{i+1}$, which contains $e_{i+1}$, is abandoned (*Repeats*), a call of
the same rule or of its twin with the same level restriction, of a rule that
reads no variable, so with a key of the class of $e_{i+1}$. If it had the
variant of $e_{i+1}$, were outside every context of growths and were in a
lookahead state no smaller than that of $e_{i+1}$, the memo would answer it
(*Answered calls*), while the parse evaluates it, since $i + 1$ is
linked. $\square$

Call a link **plain** when its evaluations have one variant and the earlier
is in a lookahead state no greater than the later. When a run restarts
through the $q$-frame of a plain link that meets the hypotheses above, the
next link, if linked, is not plain, or its later evaluation is in a context
of growths. So in a sequence of restarts through $q$-frames whose links all
meet those hypotheses, with their later evaluations outside every context of
growths, no two consecutive links are plain. The claim rests on the
condition on the memo: the bound of Lemma 3 on the evaluations at one
position, which holds whatever results are stored, gives none on such a
sequence, whose frames can lie at distinct positions. The links that are not
plain are not bounded here. The lookahead state of $e_{i+1}$ is that of
$e'_i$, or $1$ when a lookahead lies between them, and that of $e'_{i+1}$ is
that of $T$, or $1$ when a lookahead lies between them, so a link that is
not plain by its lookahead states needs only a lookahead around the next
unit that does not enclose the next frame; and the variant of $e_{i+1}$ depends on its place in
the body of $e'_i$, that of $e'_{i+1}$ on its place below $T$.

Two consecutive links $m + 1$ and $m + 2$ that are not plain by their
lookahead states, with $e_{m+1}$ and $e_{m+2}$ in state $1$ and $e'_{m+1}$ and
$e'_{m+2}$ in state $0$, all of one variant, do not exclude, by the argument
above, a third link $m + 3$ of that variant with $e_{m+3}$ in state $1$. Let the three be restarts through $q$-frames that
meet the hypotheses of the claim above for $i = m$, $m + 1$ and $m + 2$, apart
from the order of the lookahead states, and let no call on the ways below,
followed into earlier evaluations, be in a context of growths or be answered
from an entry of an earlier parse. The covered keys
give two evaluations of the key of $e_{m+3}$ before it, in a state no greater
than $1$, that of $e_{m+3}$: $y''$, before $e_{m+1}$ ends, through the
counterparts in $\mathcal{R}(e_{m+1})$ of the calls from $e'_{m+1}$ down to
$e_{m+2}$ and then of those from $e'_{m+2}$ down to $e_{m+3}$, and $y'$, before
$e_{m+2}$ ends, through the latter alone. Were they distinct, the later would
not be the first evaluation of its variant at $t_{m+3}$, it would store its
result, and the memo would answer $e_{m+3}$ (*Answered calls*). They are
one evaluation only when the parse answers a call on the way from
$\mathcal{R}(e_{m+2})$ down to the call that corresponds to $e_{m+3}$, and
then the first such call $u$ is above the lookahead between $e'_{m+2}$ and
$e_{m+3}$, with an entry stored in state $1$.

*Proof.* If the parse answers
no call on that way, $y'$ is the evaluation of the key of $e_{m+3}$ that it
makes inside $e_{m+2}$, which starts after $e_{m+1}$ ends, so $y' \ne y''$.
Otherwise $u$ lies inside $e_{m+2}$, in state $1$, and an entry stored in
state $0$ or $1$ answers it. The call $\hat{u}$ at its place on the way from
$e'_{m+2}$ down to $e_{m+3}$ has the key of $u$, since the two ways have one
variant, and the parse evaluates it after $u$, since $e_{m+3}$ lies inside
it. Were $\hat{u}$ in state $1$, or the entry stored in state $0$, the entry
would answer $\hat{u}$ (*Answered calls*). $\square$

The condition on the memo does not rule this case out: it lets the first
evaluation of each rule, and of each twin, at each position leave no result
or store it, a choice made for each rule and position on its own. So an
evaluation of the key of $u$ inside the evaluation of the key of $e_{m+2}$
that the covered keys give before $e_{m+1}$ ends, the first of the rule of
$u$ at the position of $u$, can store its result, while the evaluation of
the key of $e_{m+3}$ inside it, the first of the rule of $e_{m+3}$ at
$t_{m+3}$, stores none: two first evaluations, of two rules or of one rule
at two positions, with different choices. So the argument above does
not bound, through the memo, the number of links in a row that are not
plain; whether the memo alone bounds it is not shown, and a bound would rest
on the activations that the frames hold, which the parse must evaluate and
not answer.

**One activation in a frame.** Under the hypotheses of *Restarts through a
frame*, of the activations $\alpha_1, \alpha_2, \ldots$ the $q$-frame $e'_i$
contains $\alpha_i$ alone, and so does $e_{i+1}$. So when $\mathcal{R}(e'_i)$
contains a successful activation $\beta$ of $\sigma$ at $q$ other
than the counterpart of $\alpha_i$, the parse answers a call or leaves out a
step of a growth on the way from $\mathcal{R}(e'_i)$ down to $\beta$, or it
evaluates the activation whose counterpart is $\beta$ as one that the
sequence $\alpha_1, \alpha_2, \ldots$ leaves out: of case 5, of the third
case of *Dropped steps*, or a repeat at the position of a growth that
*Repeats inside a growth* of Lemma 5 leaves out. The same holds for
$\mathcal{R}(e_{i+1})$ and $e_{i+1}$ in place of $\mathcal{R}(e'_i)$ and
$e'_i$ when $e_{i+1}$ is an evaluation of a key.

*Proof.* By 1 of *Iterations at one scan* applied to $it_{k'}$ and
$it_{k''}$, $\alpha_i$ is the first activation in $it_{k'}$, and by the
hypothesis it is the last, so of the $\alpha$'s $it_{k'}$ contains
$\alpha_i$ alone. $e'_i$ lies inside $it_{k'}$ (2 of *Iterations at one scan*) and
contains $\alpha_i$, and $e_{i+1}$ lies inside $e'_i$ and contains
$\alpha_i$ (1 of *Restarts through a frame* and *Repeats*). The parse
evaluates inside $e'_i$ the counterparts of what $\mathcal{R}(e'_i)$
evaluates, apart from what lies inside the calls that it answers, the steps
of a growth that it does not evaluate and the alternatives that it skips by
the next character, and an alternative that contains $\beta$ is not skipped
(*Counterparts*). An evaluation that the parse makes has the outcome of its
counterpart, so if it evaluated the activation whose counterpart is $\beta$,
that activation would be a successful activation of $\sigma$ at $q$ inside
$e'_i$, at a place other than that of $\alpha_i$, and so one that the
sequence leaves out. The same argument applies to $e_{i+1}$, which contains
$\alpha_i$ alone too. $\square$

The claim is what *Positions of the holders* below uses to compare the
activation that one restart enters with the one that the next restart
enters.

**Holders of the frames' activations.** Let the hypotheses of *Restarts
through a frame* hold for each $k$ from $i$ to $i + m - 1$ in place of $i$,
with $k + 1$ linked, so that the run restarts through the $q$-frames
$e'_i, \ldots, e'_{i+m-1}$ in turn. Let each $e_{k+1}$ be an evaluation of a
key, and let no evaluation on the way from $e_{k+1}$, itself included, down to
$\alpha_k$ be a step of a growth or in a context of growths. For each $k$ let
$h_k$ be the innermost evaluation of a key that contains $\alpha_k$ and lies
inside $e_{k+1}$, at the position $p_k$. Then $h_k$ holds $\alpha_k$,
$t_{k+1} \le p_k \le q$, the $h_k$ are distinct, and at most three of them
are evaluations of one key, at most two in each lookahead state.

*Proof.* $e_{k+1}$ is an evaluation of a key that contains $\alpha_k$
(*Repeats*), so $h_k$ exists. The parse evaluates every call on the way from
$e_{k+1}$ down to $\alpha_k$, since it runs $\alpha_k$. A body evaluation on
that way of a rule that the parse memoizes is no step of a growth, so it is
of no leader, whose body evaluations are all growth steps (Lemma 3), and it
is in no context of growths, so it reads no growth open at its start, since a
call reads only the growths of the cycle of its rule at its position
([Lemma 2 of the memoization
proof](memoization.md#lemma-2-reading-an-open-growth)); so it is an
evaluation of a key. Below $h_k$ the way therefore passes only calls of
rules that the parse does not memoize, and $\alpha_k$ lies in the own region
of $h_k$. $h_k$ lies inside $e_{k+1}$, which starts at $t_{k+1}$, and contains
$\alpha_k$, which starts at $q$. $e_{k+1}$ lies inside $w_{k+1}$, which is
abandoned before $\kappa_{k+1}$ makes $e'_{k+1}$ (*Repeats*), and for
$k + 1 < i + m$, $e_{k+2}$ lies inside $e'_{k+1}$ (1 of *Restarts through a
frame* for $k + 1$), so $e_{k+1}$ ends before $e_{k+2}$ starts: the
$e_{k+1}$ follow one another in time, and the $h_k$ inside them are distinct.
Each $h_k$ is a body evaluation of a memoized rule that is neither a growth
step nor in a context of growths, so Lemma 3 allows at most three for its
variant, position, level and combination of the values of the variables that
its rule reads, which make its key, and at most two in each lookahead
state. $\square$

So a run that restarts through $m$ $q$-frames in turn has $m$ at most three
times the number of distinct keys of the $h_k$, whether or not its links are
plain, and the count does not rest on which first evaluation stores its
result. At one position the calls have at most
$2 \cdot |\text{rules}| \cdot |\text{levels}| \cdot K$ keys, so a long such
run needs evaluations $h_k$ at many distinct positions between $t_{i+1}$ and
$q$, each of which holds the activation of its link in its own region, below
which the way to it passes only calls of rules that the parse does not
memoize. That condition 2 bounds the number of these positions is not shown
here.

**Positions of the holders.** Under the hypotheses of *Holders of the frames'
activations*, let $k + 1 < i + m$, let $e'_{k+1}$ be an evaluation of a key,
and let no evaluation on the way from $e'_{k+1}$, itself included, down to
$\alpha_{k+1}$ be a step of a growth or in a context of growths. Then
$\mathcal{R}(e'_{k+1})$ contains, at the place of the counterpart of
$\alpha_k$ in $\mathcal{R}(e_{k+1})$, a successful activation $\beta_k$ of
$\sigma$ at $q$. If $\beta_k$ is the counterpart of $\alpha_{k+1}$, then
$h_{k+1}$ is made at the place of $h_k$, at $p_{k+1} = p_k$, with a key of
the class of the key of $h_k$. Otherwise *One activation in a frame*, with
$k + 1$ in place of $i$, applies to $\beta_k$.

*Proof.* $e_{k+1}$ and $e'_{k+1}$ are evaluations of one call that $w_{k+1}$
and $\kappa_{k+1}$ share, of the same rule or of its twin with the same level
restriction, of a rule that reads no variable (*Repeats*), so their keys have
one class, and as evaluations of keys neither reads a growth open at its
start. So $\mathcal{R}(e_{k+1})$ and $\mathcal{R}(e'_{k+1})$ take the same
steps up to the variants of their calls (*Counterparts*): the same
expressions at the same positions with the same success and end, and calls
with keys of one class, whose evaluations in $\mathcal{R}$ take the same
steps in turn. The counterpart of $\alpha_k$ is a successful activation of
$\sigma$ at $q$, so the activation $\beta_k$ at its place in
$\mathcal{R}(e'_{k+1})$ is one too, and $\mathcal{R}(e'_{k+1})$ contains it.

Let $\beta_k$ be the counterpart of $\alpha_{k+1}$. The parse runs
$\alpha_k$ and $\alpha_{k+1}$, so it answers no call on the way from
$e_{k+1}$ down to $\alpha_k$ or on the way from $e'_{k+1}$ down to
$\alpha_{k+1}$, and each evaluation on these ways is the counterpart of one on
the way from $\mathcal{R}(e_{k+1})$ down to the counterpart of $\alpha_k$, or
from $\mathcal{R}(e'_{k+1})$ down to $\beta_k$ (*Counterparts*). These two
ways lead to activations at one place, so they pass, call by call, calls at
places that correspond, at the same positions and with keys of one class.
Of two such calls, one is of a rule and the other of the same rule or of its
twin, so the parse memoizes both or neither (Lemma 3). By the hypotheses,
as in the proof of *Holders of the frames' activations*, every body
evaluation on either way of a rule that the parse memoizes is an evaluation
of a key. So the innermost evaluations of keys on the two ways are made at
places that correspond. On the first way it is $h_k$, since $e_{k+1}$ is an
evaluation of a key on it; on the second it is $h_{k+1}$, since $e_{k+2}$,
which contains $\alpha_{k+1}$ and lies inside $e'_{k+1}$ (1 of *Restarts
through a frame* for $k + 1$), is an evaluation of a key on it. So $h_{k+1}$
is made at the position of $h_k$, with a key of its class.

Otherwise $\beta_k$ is a successful activation of $\sigma$ at $q$ that
$\mathcal{R}(e'_{k+1})$ contains, other than the counterpart of
$\alpha_{k+1}$, and the hypotheses of *Restarts through a frame* hold for
$k + 1$, since $k + 1 < i + m$. $\square$

So along such a run, when the hypotheses above hold for every $k$ with
$k + 1 < i + m$, each $e'_{k+1}$ an evaluation of a key with no step of a
growth and no evaluation in a context of growths on the way from it, itself
included, down to $\alpha_{k+1}$, the holder keeps its class and position
across each restart at which the activation that the next frame enters lies at the place
of the one that the earlier restart entered. The $h_k$ are distinct and at
most six have keys of one class at one position, three for each variant
(*Holders of the frames' activations*), so a stretch of the run with one
class and position of the holder has at most six links, and $m$ is at most
six times one more than the number of restarts of the other kind. At each of
those, the way from $\mathcal{R}(e'_{k+1})$ down to $\beta_k$ is not run by
the parse as an activation of the sequence: it answers a call on it, leaves
out a step of a growth on it, or runs $\beta_k$ as an activation that the
sequence leaves out. *Restarts of the other kind* below shows that such a
restart has a link that is not plain, unless the parse leaves out a step of a
growth on the way from $e_{k+1}$ to the place of the counterpart of
$\alpha_{k+1}$, evaluates an activation that the sequence leaves out there, or
makes a call on that way in a context of growths.

**Restarts of the other kind.** Under the hypotheses of *Positions of the
holders*, let $\beta_k$ not be the counterpart of $\alpha_{k+1}$, let $\gamma$
be the activation of $\mathcal{R}(e_{k+1})$ at the place of the counterpart of
$\alpha_{k+1}$ in $\mathcal{R}(e'_{k+1})$, and let no call that the parse makes
on the way from $e_{k+1}$ down to that place be in a context of growths. Then
$\gamma$ is a successful activation of $\sigma$ at $q$ other than the
counterpart of $\alpha_k$, and the parse leaves out a step of a growth on the
way from $\mathcal{R}(e_{k+1})$ down to $\gamma$, or evaluates the activation
whose counterpart is $\gamma$ as one that the sequence $\alpha_1, \alpha_2,
\ldots$ leaves out, as in *One activation in a frame*, or it answers a call
$g$ on that way and evaluates the call $\hat{g}$ at the place of $g$ on the
way from $e'_{k+1}$ down to $\alpha_{k+1}$. In the last case the link $k + 1$
is not plain: $e_{k+1}$ and $e'_{k+1}$ have different variants, or $e_{k+1}$
is in lookahead state $1$ and $e'_{k+1}$ in state $0$, with no lookahead
between $e'_{k+1}$ and $\hat{g}$.

*Proof.* As in the proof of *Positions of the holders*,
$\mathcal{R}(e_{k+1})$ and $\mathcal{R}(e'_{k+1})$ take the same steps up to
the variants of their calls, so $\gamma$, at the place of a successful
activation of $\sigma$ at $q$, is one too. Its place is not that of the
counterpart of $\alpha_k$, since at that place $\mathcal{R}(e'_{k+1})$ has
$\beta_k$, which is not the counterpart of $\alpha_{k+1}$. Of the activations
$\alpha_1, \alpha_2, \ldots$, $e_{k+1}$ contains $\alpha_k$ alone (*One
activation in a frame* with $k$ in place of $i$, whose hypotheses hold by
those of *Holders of the frames' activations*), and its statement for
$\mathcal{R}(e_{k+1})$, with $\gamma$ in place of $\beta$, gives: the parse
answers a call or leaves out a step of a growth on the way from
$\mathcal{R}(e_{k+1})$ down to $\gamma$, or it evaluates the activation whose
counterpart is $\gamma$ as one that the sequence leaves out.

In the case of an answered call, let $g$ be the outermost call on that way
that the parse answers, with no step of a growth left out above it, since
otherwise the first case holds. Above $g$ it evaluates the counterparts of the
evaluations on the way, so it makes $g$ at the place of its counterpart. The
parse runs $\alpha_{k+1}$, so it answers no call on the way from $e'_{k+1}$
down to $\alpha_{k+1}$, and, as in the proof of *Positions of the holders*,
that way and the way from $\mathcal{R}(e_{k+1})$ down to $\gamma$ pass, call
by call, calls at places that correspond, at the same positions and with keys
of one class. So the parse evaluates a call $\hat{g}$ at the place of $g$, at
the position of $g$, with a key of the class of the key of $g$, and the
expressions from $e_{k+1}$ down to $g$ and from $e'_{k+1}$ down to $\hat{g}$
are the same, so a lookahead lies between both pairs or neither. The variant
of each call on the two ways depends only on its place and on the variant of
the body that makes it (*Counterparts*), so when $e_{k+1}$ and $e'_{k+1}$ have
one variant, $g$ and $\hat{g}$ have one key. $e_{k+1}$ lies inside
$w_{k+1}$, which is abandoned before $\kappa_{k+1}$ makes $e'_{k+1}$
(*Repeats*), so the parse makes $\hat{g}$ after it answers $g$, and neither
is in a context of growths, by the hypotheses. Were the link $k + 1$ plain,
$g$ and $\hat{g}$ would have one key and $\hat{g}$ a lookahead state no
smaller than that of $g$: both are in state $1$ when a lookahead lies between,
and otherwise in the states of $e_{k+1}$ and $e'_{k+1}$. The memo would then
answer $\hat{g}$ (*Answered calls*), which the parse evaluates. So the link is
not plain, and when its evaluations have one variant, $\hat{g}$ is in a
smaller lookahead state than $g$: no lookahead lies between, $e_{k+1}$ is in
state $1$ and $e'_{k+1}$ in state $0$. $\square$

So a restart of the other kind has a link that is not plain, or a step of a
growth left out or an activation that the sequence leaves out on the way to
$\gamma$, and a run whose links meet the hypotheses of *Positions of the
holders* and of this claim has $m$ at most six times one more than the number of its restarts with a
step of a growth left out on the way to $\gamma$, of its restarts with an
activation that the sequence leaves out at the place of $\gamma$, and of its
links that are not plain. The bound is for runs in which no call that the
parse makes on the way from $e_{k+1}$ down to the place of $\gamma$ is in a
context of growths, the hypothesis of this claim. The links that are not
plain are those that *Consecutive restarts through frames* leaves open, in
either form, with two variants or with two lookahead states, and a bound on
their number in a run is not shown here.

**Units below the earlier evaluations.** Under the hypotheses of *Positions
of the holders*, let $I$ be the iteration of $T$ that holds $e'_{k+1}$. Then
an evaluation below $e_{k+1}$ on the way to $\alpha_k$ is abandoned; let
$v_k$ be the first, and $v'_k$ the evaluation at its place in
$\mathcal{R}(e'_{k+1})$, and let no growth lie on the way in $\mathcal{R}$
from $\mathcal{R}(T)$ down to $v'_k$. Then $v_k$ is a committing unit that
starts at a position $s_k$ with $t_{k+1} \le s_k \le \mathrm{end}(e'_{k+1})$,
and $v'_k$ is an evaluation of the same unit at $s_k$ that takes the same
steps up to the variants of its calls. Let $\kappa$ be the continuation
that $\mathcal{R}$ evaluates from $s_k$ the last time after $v'_k$ before
$\mathcal{R}(T)$ ends, let the calls that $v_k$ and $\kappa$
share, $j_k$ of them, end in $v_k$ at $r_k$, let $\alpha_k$ lie in $v_k$
outside the evaluations of those calls, and let $v_k$ be committing for
$\kappa$ because their first characters after those calls are disjoint, or
because its words exclude $\kappa$, with $|W_k|$ the length of the word of
its path that holds $\alpha_k$; let $|W_k| = 0$ in the first case. Then
$T$ ends at or before $r_k + |W_k|$, and after $I$, $T$ has at most $j_k +
|W_k|$ iterations that consume input and one more at its end. So the run has
at most $j_k + |W_k| + 1$ links after $k + 1$. Both $j_k$ and $|W_k|$ depend
only on the grammar: each round of the removal of the shared calls takes one
call from the start of every path of the unit and of the continuation, the
classifier removes at most sixteen in this way (condition 2), and a word is a
literal of the grammar. The calls that $v_k$ shares with $\kappa$ are the
ones that the removal takes, and condition 2 compares what remains after
them, so $j_k \le 16$; when the two share more, what remains begins with a
call that both evaluate, and a unit committing by first characters needs
that call to examine no input. A unit committing by its words reads its word
after the removed calls, through the body of that call when the word begins
inside it, so the input at $r_k$ begins with the word in that case too.

*Proof.* As in the proof of *Positions of the holders*, $e_{k+1}$ and
$e'_{k+1}$ are evaluations of keys of one class, so $\mathcal{R}(e_{k+1})$
and $\mathcal{R}(e'_{k+1})$ take the same steps up to the variants of their
calls, and each evaluation of the parse has the outcome of its counterpart
(*Counterparts*). So $e_{k+1}$ succeeds and ends where $e'_{k+1}$ does, at or
before $q$, since $e'_{k+1}$ is a $q$-frame by the hypotheses of *Restarts
through a frame* for $k + 1$. $\alpha_k$
consumes input from $q$, so, as in the proof of *Restarts*, an evaluation
from $e_{k+1}$ down to $\alpha_k$ is abandoned; it is not $e_{k+1}$, which
succeeds and is no expression of a lookahead. No evaluation on that way is a
step of a growth, so $v_k$ is no dropped step: its parent goes on after it,
and $v_k$ is a unit. It contains $\alpha_k$, so, as in the proof of
*Restarts*, it is the expression of a lookahead or fails after $\alpha_k$
has examined input, and condition 2 makes it committing. It lies inside
$e_{k+1}$, so $s_k \ge t_{k+1}$; its parent goes on from $s_k$, and every
evaluation from $e_{k+1}$ down to that parent is not abandoned, so each ends
at or after the end of its child on that way, and $\mathrm{end}(e_{k+1}) \ge
s_k$.

$v'_k$ is at the place of $v_k$, so it evaluates the same unit at $s_k$, is
abandoned as $v_k$ is, and evaluates the calls that it shares with $\kappa$
with the results that $v_k$ has, ending at $r_k$. As in the proof of
*Direct activations*, the input from $r_k$ to the end of $\alpha_k$ is
consumed in $v_k$ after those calls, so $r_k \le q$ and the character at
$r_k$ is a first character of $v_k$ after them; and when $v_k$ commits by
its word $W$, its path that holds $\alpha_k$ begins after them with $W$, so
the input at $r_k$ begins with $W$. No evaluation from $\mathcal{R}(e'_{k+1})$
down to the parent of $v'_k$ is abandoned, since none from $e_{k+1}$ down to
the parent of $v_k$ is, and none from $\mathcal{R}(I)$ down to
$\mathcal{R}(e'_{k+1})$, since none from $I$ down to $e'_{k+1}$ is, by the
hypotheses of *Restarts through a frame* for $k + 1$. So from the end of $v'_k$ to the end of
$\mathcal{R}(T)$, $\mathcal{R}$ returns to no position before $s_k$: it
would do so only by abandoning an evaluation that encloses $v'_k$ and starts
before $s_k$, or by a later step of a growth that encloses $v'_k$
(*Restarts*), and the evaluations that enclose $v'_k$ up to
$\mathcal{R}(T)$ are not abandoned and are no growths. So $\kappa$ goes on
from $s_k$ through the rest of $\mathcal{R}(e'_{k+1})$ and of
$\mathcal{R}(I)$ and through the later iterations of $\mathcal{R}(T)$, and
consumes the input from $s_k$ to $\mathrm{end}(T)$, which is at least
$\mathrm{end}(e'_{k+1}) \ge s_k$.

Every path of $\kappa$ that examines the input begins with the shared calls,
each with the result that it has in $v'_k$ (*Direct activations*), so
$\kappa$ consumes the input from $s_k$ to $r_k$ inside their evaluations,
and if $T$ ended after $r_k$, $\kappa$ would consume the character at $r_k$
after them, and it would be a first character of $\kappa$ after them. When
the first characters are disjoint, this is a contradiction, since that
character is a first character of $v_k$ after them. When the words exclude
$\kappa$, every path
of $\kappa$ on that input fails, examines nothing more or evaluates a
negative lookahead that is known to match before it reaches the end of $W$,
and $\kappa$ goes on without failing to $\mathrm{end}(T)$, so $T$ does not
end after $r_k + |W_k|$.

The iterations of $T$ after $I$ start at or after $\mathrm{end}(I) \ge
s_k$ and end at or before $\mathrm{end}(T) \le r_k + |W_k|$. A repetition
stops at the first iteration that fails or consumes nothing, so each of them
but the last consumes input; the last can hold the later evaluation of a link
and is counted on its own.
Let such an iteration start at $x < r_k$. The character at $x$ is consumed
in the evaluation of a shared call that $\kappa$ makes. That evaluation is
made after $v'_k$; one made while $\mathcal{R}(I)$ is in progress lies inside
it and ends at or before $\mathrm{end}(I) \le x$, so it is made after
$\mathcal{R}(I)$ ends, while $\mathcal{R}(T)$ is in progress, and lies inside
the iteration that consumes $x$. These evaluations are distinct for
distinct iterations, so at most $j_k$ iterations start before $r_k$, and at most $|W_k|$ start at or
after it, each consuming a character before $r_k + |W_k|$, and one more
iteration ends $T$. Each link of the
run after $k + 1$ has its later evaluation in a later reaching iteration of
$T$ (1 and 2 of *Iterations at one scan*), so there are at most
$j_k + |W_k| + 1$ such links. $\square$

The claim does not depend on whether the link is plain: it applies
condition 2 to the first abandoned unit below $e_{k+1}$, which lies inside
the evaluation of a call that $w_{k+1}$ shares, where condition 2 on
$w_{k+1}$ says nothing about it, and it transfers that unit to $\mathcal{R}(e'_{k+1})$, whose
continuation goes on through the later iterations of $T$. So in a run whose
links meet its hypotheses, a link $k + 1$ followed by more than
$j_k + |W_k| + 1$ links has $\alpha_k$ inside the evaluation, in $v_k$, of a
call that $v_k$ shares with $\kappa$, or $v_k$ commits against $\kappa$ in
another way: it decides $\kappa$, or it is committing for a path that enters
the growth again; or a growth lies on the way in $\mathcal{R}$ from
$\mathcal{R}(T)$ down to $v'_k$, which the claim leaves out. In the first case $\kappa$ evaluates that call again at
the same position with the same result (*Direct activations*). That
evaluation lies in the rest of $\mathcal{R}(I)$ or in one of the next $j_k$
iterations of $T$: $\kappa$ evaluates each of its $j_k$ shared calls once, and
one made after $\mathcal{R}(I)$ ends lies inside an iteration of $T$, as in
the proof. *Shared calls of the units below* follows that evaluation, and
the calls that the abandoned units inside it share, down to $\alpha_k$ at any
depth, and bounds the links after $k + 1$ apart from those that pass a pair
of evaluations of a shared call that is not plain. The cases in which a unit
on that way decides its continuation, commits through a path that enters the
growth again, or lies below a growth in $\mathcal{R}$ are not treated.

**Shared calls of the units below.** Let the hypotheses of *Units below the
earlier evaluations* hold, apart from those on where $\alpha_k$ lies in $v_k$
and on the way in which $v_k$ is committing, and let $m > k + 1$ index a link
of the run, whose activation $\alpha_m$ lies in the iteration $I_m$ of $T$.
Let no evaluation on the way from $T$ down to $\alpha_m$ be a step of a
growth or in a context of growths, and let no growth lie, in $\mathcal{R}$,
on the way from $\mathcal{R}(T)$ down to an evaluation made after $v'_k$ that
holds a successful activation of $\sigma$ at $q$ at any depth. Call the
abandoned evaluations on the way from $e_{k+1}$ down to $\alpha_k$ the
**units below** $e_{k+1}$, and let each of them, $v$, be committing by first
characters or by its words for every continuation $\kappa'$ such that
$\alpha_k$ lies in $v$ outside the evaluations of the calls that $v$ shares
with $\kappa'$, where, as in that claim, these calls are the ones that the
removal takes. Then $\alpha_m$ lies inside an evaluation $f'_m$ that the
parse makes in $I_m$ of a call at a position $x_m$, and a unit below
$e_{k+1}$ makes, on its way to $\alpha_k$, a call $f_m$ at $x_m$, of the same
rule or of its twin with the same level restriction, of a rule that reads no
variable, that it shares with the continuation that $\mathcal{R}$ evaluates
after the evaluation at its place. One of the following holds:

1. $f_m$ contains $\alpha_k$, and $\alpha_m$ lies in $f'_m$ at the place of
   $\alpha_k$ in $f_m$. Either $f_m$ is an evaluation of a key, and the
   innermost evaluation of a key that contains $\alpha_m$ has the class and
   position of $h_k$; or $f_m$ and $f'_m$ are evaluations of the rule of
   $\sigma$, which calls no rule, and $\alpha_k$ and $\alpha_m$ are direct in
   them.
2. The parse answers $f_m$, or a call $g$ inside the evaluation $f_m$ on the
   way down to the place of $\alpha_m$, outside contexts of growths, and
   evaluates the call $\hat{g}$ at its place in $f'_m$; and $f_m$ and $f'_m$
   have different variants, or $f_m$ is in lookahead state $1$ and $f'_m$ in
   state $0$, with no lookahead between $f'_m$ and $\hat{g}$.
3. On the way from $\mathcal{R}(e_{k+1})$ down to the activation at the place
   of $\alpha_m$ in $\mathcal{R}(f_m)$, the parse leaves out a step of a
   growth or answers a call in a context of growths, or it evaluates the
   activation whose counterpart that is as one that the sequence $\alpha_1,
   \alpha_2, \ldots$ leaves out.

At most five links after $k + 1$ have the first form of 1.

*Proof.* By 1 and 2 of *Iterations at one scan*, $I_m$ is a reaching
iteration after $I$. Let $\rho$ be the counterpart of $\alpha_m$ in
$\mathcal{R}(I_m)$, a successful activation of $\sigma$ at $q$ that consumes
input from $q$, as $\alpha_m$ does (*Counterparts*). By the proof of *Units
below the earlier evaluations*, which does not use where $\alpha_k$ lies in
$v_k$ or how $v_k$ is committing, $\mathcal{R}$ evaluates, from the end of
$v'_k$ to the end of $\mathcal{R}(T)$, a continuation of $v'_k$ from $s_k$ and
returns to no position before $s_k$, and $\rho$, which starts after
$\mathcal{R}(I)$ ends, lies in it. So the following holds for $l = 1$, with
$v^{(1)} = v_k$ and $u_1 = v'_k$: $v^{(l)}$ is a unit below $e_{k+1}$ that
contains $\alpha_k$ and starts at $s^{(l)}$, $u_l$ is an evaluation of
$\mathcal{R}$ of the same unit at $s^{(l)}$ that takes the steps of
$\mathcal{R}(v^{(l)})$ up to the variants of its calls, and from the end of
$u_l$ to $\rho$, $\mathcal{R}$ evaluates a continuation $\kappa^{(l)}$ of
$u_l$ from $s^{(l)}$ and returns to no position before $s^{(l)}$.

Let the calls that $v^{(l)}$ shares with $\kappa^{(l)}$ end in $u_l$ at
$r^{(l)}$. Every path of $\kappa^{(l)}$ that examines the input begins with
them, each evaluated where $u_l$ evaluates it and with the result that it has
there (*Direct activations*), and $\rho$ consumes input, so on its way to
$\rho$, $\mathcal{R}$ keeps evaluations of them, one after another from
$s^{(l)}$, and examines no other input before it has kept the last. The path
of $v^{(l)}$ that holds $\alpha_k$ begins with them too, so $\alpha_k$ lies
inside the evaluation $d$ of one of them in $v^{(l)}$, and $v^{(l)}$ makes
the calls up to that one, or $\alpha_k$ follows them all and $v^{(l)}$ makes
them all. Three cases remain.

In the first, $\rho$ lies inside the evaluation $F$ that $\mathcal{R}$ keeps
on its way to $\rho$ of a call that $v^{(l)}$ makes, at a position $x$; let
$f$ be that call in $v^{(l)}$, made by the parse inside $e_{k+1}$. The parse
runs $\alpha_m$, so it evaluates the counterparts of the evaluations on the
way from $\mathcal{R}(I_m)$ down to $\rho$, and $f'_m$, whose counterpart is
$F$, is one. A call of a rule of a left-recursion cycle is shared only when
the two calls lie in the same growths of the cycle, and no growth lies on the
way down to $F$, so neither $F$ nor $\mathcal{R}(f)$ reads a growth open at
its start, and the two, of the same rule or of its twin with the same level
restriction, of a rule that reads no variable, take the same steps up to the
variants of their calls (*Counterparts*). So $\mathcal{R}(f)$ holds, at the
place of $\rho$, a successful activation $\rho'$ of $\sigma$ at $q$, inside
$\mathcal{R}(e_{k+1})$.

If $\rho'$ is the counterpart of $\alpha_k$, $f$ contains $\alpha_k$. The
parse runs $\alpha_k$ and $\alpha_m$, so it answers no call on the way from
$f$ down to $\alpha_k$ or from $f'_m$ down to $\alpha_m$, and, as in the proof
of *Positions of the holders*, the two ways pass, call by call, calls at
places that correspond, at the same positions and with keys of one class, of
which the parse memoizes both or neither, and every body evaluation on either
way of a rule that the parse memoizes is an evaluation of a key, by the
hypotheses of *Holders of the frames' activations* and of this claim. When $f$
is an evaluation of a key, the innermost evaluations of keys on the two ways
are at places that correspond: $h_k$, since $f$ contains $\alpha_k$, and the
innermost evaluation of a key that contains $\alpha_m$. Otherwise $f$ is a
call of a rule that the parse does not memoize, which as a shared call calls
no rule, so $\alpha_k$ is direct in it, its rule is the rule of $\sigma$,
and so for $f'_m$ and $\alpha_m$. This is 1. The innermost evaluations of
keys that contain the activations of the links of the first form lie in
their iterations after $I$, and $h_k$ lies in $e_{k+1}$, inside the iteration
that holds $e'_k$, before $I$ (1 of *Restarts through a frame*). They are
distinct evaluations of keys of one class at one position, neither steps of a
growth nor in a context of growths, so by Lemma 3 they number at most six,
$h_k$ among them.

Otherwise $\rho'$ is a successful activation of $\sigma$ at $q$ in
$\mathcal{R}(e_{k+1})$ other than the counterpart of $\alpha_k$. Of the
activations $\alpha_1, \alpha_2, \ldots$, $e_{k+1}$ contains $\alpha_k$ alone,
and the statement of *One activation in a frame* for $\mathcal{R}(e_{k+1})$,
with $\rho'$ in place of $\beta$, gives: the parse answers a call or leaves
out a step of a growth on the way from $\mathcal{R}(e_{k+1})$ down to
$\rho'$, or it evaluates the activation whose counterpart is $\rho'$ as one
that the sequence leaves out. In the case of an answered call, let $g$ be the
outermost on that way, with no step of a growth left out above it, since
otherwise 3 holds. The parse makes $f$, so above $\mathcal{R}(f)$ it
evaluates the counterparts of the evaluations on the way, and $g$ is $f$ or
lies inside it. If the parse answers $g$ in a context of growths, 3 holds.
Otherwise, as above, the ways from $\mathcal{R}(f)$ down to $\rho'$ and from
$F$ down to $\rho$ pass calls at places that correspond, so the parse,
which answers no call on the way from $f'_m$ down to $\alpha_m$, evaluates
the call $\hat{g}$ at the place of $g$, at its position, with a key of the
class of its key, and the expressions from $f$ down to $g$ and from $f'_m$
down to $\hat{g}$ are the same, so a lookahead lies between both pairs or
neither. The variant of each call on the two ways depends only on its place
and on the variant of the body that makes it (*Counterparts*), so $g$ and
$\hat{g}$ have one variant when $f$ and $f'_m$ do. $e_{k+1}$ ends before $I$,
so the parse makes $\hat{g}$ after it answers $g$, and $\hat{g}$ is in no
context of growths. Were $f$ and $f'_m$ of one variant, with $f'_m$ in a
lookahead state no smaller than that of $f$, $g$ and $\hat{g}$ would have one
key and $\hat{g}$ a lookahead state no smaller than that of $g$: both are in
state $1$ when a lookahead lies between, and otherwise in the states of $f$
and $f'_m$. The memo would then answer $\hat{g}$ (*Answered calls*), which
the parse evaluates. This is 2.

In the second case, $\alpha_k$ lies inside the evaluation $d$ in $v^{(l)}$ of
a shared call, at a position $p$, and $\rho$ lies after the evaluation $D$ of
that call at $p$ that $\mathcal{R}$ keeps on its way to $\rho$. A shared
call that fails ends every path of $\kappa^{(l)}$ through it, so $D$
succeeds, and so does $d$, which has its result. So $q \ge
\mathrm{end}(D) = \mathrm{end}(d)$, since $D$ and $\mathcal{R}(d)$ take the
same steps up to the variants of their calls, as $F$ and $\mathcal{R}(f)$
do. If no evaluation below $d$ on the way to $\alpha_k$ were abandoned,
$\alpha_k$ would consume input from $q$ inside the match of $d$, and
$\mathrm{end}(d) > q$. So let $v^{(l+1)}$ be the first, at $s^{(l+1)}$: as
in the proof of *Units below the earlier evaluations*, no evaluation on the
way from $e_{k+1}$ down to $\alpha_k$ is a step of a growth, so $v^{(l+1)}$ is
a unit below $e_{k+1}$, and it contains $\alpha_k$. $D$ holds, at the place
of $v^{(l+1)}$, an evaluation $u_{l+1}$ of the same unit at $s^{(l+1)}$ that
takes its steps up to the variants of its calls. None from $D$ down to the
parent of $u_{l+1}$ is abandoned, since none from $d$ down to the parent of
$v^{(l+1)}$ is; none from $D$ up to the innermost evaluation that contains
$D$ and $\rho$ is abandoned before $\rho$, since $D$ is kept on the way to
$\rho$, and those above it contain $\rho$; and no growth lies on the way from
$\mathcal{R}(T)$ down to $u_{l+1}$: none down to $D$, which holds a successful
activation of $\sigma$ at $q$ at the place of $\alpha_k$, and none below,
since none lies on the way from $d$ down to $v^{(l+1)}$. So, as in the proof
of *Units below the earlier evaluations*, from the end of $u_{l+1}$ to $\rho$,
which lies after $D$ and so after $u_{l+1}$, $\mathcal{R}$ returns to no
position before $s^{(l+1)}$ and evaluates a continuation $\kappa^{(l+1)}$ of
$u_{l+1}$ from there, and the statement above holds for $l + 1$.

In the third case, $\alpha_k$ follows the shared calls in $v^{(l)}$, and
$\rho$ follows them on the way of $\mathcal{R}$ to it. As in the proof of
*Direct activations*, the input from $r^{(l)}$ to the end of $\alpha_k$ is
consumed in $v^{(l)}$ after the shared calls, so $r^{(l)} \le q$ and the
character at $r^{(l)}$ is a first character of $v^{(l)}$ after them. On its
way to $\rho$, $\mathcal{R}$ consumes the input from $r^{(l)}$ to $q$ after
them, and $\rho$ the character at $q$, so the character at $r^{(l)}$ is a
first character of $\kappa^{(l)}$ after them too, and the first characters of
the two are not disjoint. When $v^{(l)}$ commits by its words, the path that
holds $\alpha_k$ begins after the removed calls with its word $W$, read as in
*Units below the earlier evaluations*, and $\sigma$ follows the word, so $q
\ge r^{(l)} + |W|$; on that input every path of $\kappa^{(l)}$ fails,
examines nothing more or evaluates a negative lookahead that is known to
match before it reaches the end of $W$, and $\mathcal{R}$ does not reach $q$
on it. Both contradict the hypothesis on the units below.

$v^{(l+1)}$ lies strictly inside $v^{(l)}$, on the way from $e_{k+1}$ down to
$\alpha_k$, so the second case occurs at finitely many levels, and the first
case holds at the last. $\square$

So the depth of the units below $e_{k+1}$ does not enter the count. Each link
after $k + 1$ lies in its own iteration, inside the later evaluation of a call
that a unit below $e_{k+1}$ shares with the continuation that $\mathcal{R}$
evaluates after the evaluation at its place, and the parse evaluates that
call there: the later evaluations that the memo answers hold no link, however
many units below lie between $e_{k+1}$ and $\alpha_k$ and however far their
shared calls reach past the ends of the rule bodies that hold them. A pair of
the two evaluations of such a call with one variant, the earlier in a
lookahead state no greater than the later, holds a link only at the place of
$\alpha_k$, and then, when the call is of a rule that the parse memoizes, the
holder of the link has the class and position of $h_k$. So a run whose links
after $k + 1$ meet the hypotheses has at most five of them besides those of
the second form of 1, of 2 and of 3, and the later evaluations $f'_m$ of the
links of 2 lie in distinct iterations of $T$. So do their calls $\hat{g}$,
which the parse evaluates on the way from $T$ down to $\alpha_m$, with keys
of the rule of $g$, which it memoizes since it answers $g$, or of its twin:
they are distinct evaluations of keys, neither steps of a growth nor in a
context of growths, and by Lemma 3 at most six of them have keys of one class
at one position. When $f_m$ and $f'_m$ have one variant and the entry that
answers $g$ is not one of an earlier parse, that entry was stored in state $1$,
since one stored in state $0$ would answer $\hat{g}$, by an evaluation of the
key of $g$, which is that of $\hat{g}$. So $\hat{g}$ is not the first
evaluation of its key, it stores its result in state $0$, and the memo answers
every later call of that key outside contexts of growths: such links have
their calls $\hat{g}$ at distinct keys. A bound on the links of 2 in a run is
therefore, up to a factor that depends only on the grammar, six times the
number of classes of keys, a bound on the number of positions of their calls
$\hat{g}$, each
inside the evaluation $f'_m$ of a call that a unit below shares, from
$t_{k+1}$ to $q$. Each unit below shares at most sixteen calls, but the units
below are not bounded in number, and this proof does not bound the units
below whose shared calls hold the later evaluations of links. *Holders of
the later links* below shows that those of these links that meet its
hypotheses hold their activations only in
evaluations whose class and position an evaluation made before $e_{k+1}$ ends
has already, so that the run adds none through them, and a bound on them
follows from a bound on the heads that hold an activation at the position of
the scan. Three parts
stay open: a bound
on the links of 2 beyond that reduction, at each of which a unit below evaluates a shared call as
the twin and the continuation as the rule, or the reverse, or the unit below
evaluates it inside a lookahead and the continuation outside every lookahead
down to the call $\hat{g}$; the links of the second form of 1, leaf links of
the kind of *Calls that call no rule* through the rule of $\sigma$; and the
units below that commit only by deciding their continuation or for a path
that enters the growth again. The hypothesis on the units below holds for a
unit that shares more than sixteen calls with a continuation and holds
$\alpha_k$ in the seventeenth, the first shared call after the sixteenth,
only when that unit commits by its words: that call begins what remains of both, and since it examines
input, the first characters do not part.

**Holders of the later links.** Let the hypotheses of *Shared calls of the
units below* hold, and let the link $m > k + 1$ have the form 2, with $f_m$,
$f'_m$, $g$ and $\hat{g}$ as there. Let $h_m$ be the innermost evaluation of a
key that contains $\alpha_m$, at the position $z_m$, and let the call at the
place of $h_m$ in $\mathcal{R}(f_m)$ read no growth open at its start. Then
$h_m$ lies inside $\hat{g}$ and holds $\alpha_m$, and, outside cases (a) and
(b) of the covered keys on the path from $\mathcal{R}(e_{k+1})$ down to that
call, the parse makes, before $e_{k+1}$ ends, an evaluation $h^\circ_m$ of a
key of the class of the key of $h_m$ at $z_m$ that holds a successful
activation of $\sigma$ at $q$, at the place of $\alpha_m$ in $h_m$. The
$h_m$ of distinct links are distinct, and at most six of them have keys of one
class at one position.

*Proof.* $\hat{g}$ is a body evaluation, on the way from $T$ down to
$\alpha_m$, of a rule that the parse memoizes, since the parse answers the
call $g$ of that rule or of its twin. By the hypothesis of *Shared calls of
the units below* no evaluation on that way is a step of a growth or in a
context of growths, so, as in the proof of *Holders of the frames'
activations*, every body evaluation on it of a rule that the parse memoizes
is an evaluation of a key. So $\hat{g}$ is one, $h_m$ exists and is $\hat{g}$
or lies inside it, below $h_m$ the way passes only calls of rules that the
parse does not memoize, and $\alpha_m$ lies in the own region of $h_m$.

Let $F$ be the counterpart of $f'_m$, as in the proof of *Shared calls of the
units below*. $F$ and $\mathcal{R}(f_m)$ take the same steps up to the
variants of their calls, and the parse evaluates the counterparts of the
evaluations on the way from $F$ down to the counterpart $\rho$ of $\alpha_m$,
of which $\mathcal{R}(h_m)$ is one. So $\mathcal{R}(f_m)$ makes, at the place
of $\mathcal{R}(h_m)$, a call $z$ at $z_m$ with a key of the class of the key
of $h_m$, of a rule that the parse memoizes and that is no leader, since
$h_m$ is an evaluation of a key and the parse memoizes a twin exactly when it
memoizes its rule; and the evaluation of $z$ contains, at the place of $\rho$,
a successful activation of $\sigma$ at $q$. By the hypothesis $z$ reads no
growth open at its start. The parse makes $f_m$ inside $e_{k+1}$, so
$\mathcal{R}(f_m)$, and $z$ with it, lies inside $\mathcal{R}(e_{k+1})$
(*Counterparts*), so $z_m \ge t_{k+1}$, the position at which $e_{k+1}$
starts, and $z_m \le q$, since the evaluation of $z$ holds an activation that
consumes from $q$. The covered keys, with $e_{k+1}$ as $x$, then give an
evaluation $h^\circ_m$ of the key of $z$ that the parse makes before
$e_{k+1}$ ends; case (c) does not arise, since $z$ holds an activation in
$\mathcal{R}$. $e_{k+1}$ ends before $I$ starts, and $h_m$ lies in $I_m$,
after $I$, so $h^\circ_m$ is the earlier of the two. They are evaluations of
keys of one class, so by the replays the evaluations in the own region of
$h_m$ correspond one to one to those in the own region of $h^\circ_m$, each
to an evaluation of the same expression at the same position with the same
success, apart from those inside an alternative that the parse skips by the
next character, which holds no activation. So $h^\circ_m$ holds a successful
activation of $\sigma$ at $q$ at the place of $\alpha_m$.

The $h_m$ of distinct links lie in the iterations of their links, which are
distinct. They are body evaluations of rules that the parse memoizes, neither
steps of a growth nor in a context of growths, so by Lemma 3 at most six of
them have keys of one class at one position. $\square$

A link of the first form of 1 meets the same statement with $h_k$ as the
earlier evaluation: its holder has the class and position of $h_k$, which
lies inside $e_{k+1}$ and holds $\alpha_k$. So the links after $k + 1$ of the
first form of 1 and of 2 that meet the hypotheses above number at most six
times the classes and positions, from $t_{k+1}$ to $q$, of the evaluations of
keys that the parse makes before $e_{k+1}$ ends and that hold a successful
activation of $\sigma$ at $q$. The run adds none of these: each of its later
links that meets the hypotheses repeats, in a later evaluation of a class at a position, an activation
that an evaluation of that class at that position held before $e_{k+1}$
ended, whether the pair that the link passes is plain or not, and however
many units below lie on the way. The claim counts nothing, and it does not
rest on which first evaluation stores its result. It gives no bound that
depends only on the grammar: those earlier activations are among $\alpha_1,
\ldots, \alpha_k$ and those that the sequence leaves out, and the head of
each of those classes holds one at the same place (the replays),
so their number is part of the number of heads that start at or before the
position of the scan and hold an activation there, which step 1 of [Open
Steps](#open-steps) leaves open. A bound on the links of 2 in a run is
therefore no open part of its own: it follows, with the factor six, from a
bound on those heads, and this claim gives no other.

The claims above follow the sequence $\alpha_1, \alpha_2, \ldots$ of all the
successful activations of $\sigma$ at $q$, whoever holds them. The three
claims below count inside the own region of one memoized evaluation instead,
for a source in a rule that calls no rule. In an own region no call is of a
rule that the parse memoizes, so neither a memoized link nor a $q$-frame of
such a rule lies in it. The claims reduce the activations that one evaluation
holds to the iterations of one evaluation of a repetition, and show what
joins two activations that one evaluation holds one after the other.

**Own expressions.** Let $H$ be a memoized evaluation, of a rule $Q$ or of its
twin. Of the calls in the own region of $H$ that are nested in one another,
each made in the body evaluation of the one before, no two are of one rule, a
rule and its twin counting as one rule, so at most $|\text{rules}|$ of them
nest. The own region of $H$ is therefore an evaluation of the **own
expression** of $Q$: the body of $Q$ in which, to that depth, each call of a
rule that the parse does not memoize is replaced by the body of that rule.
The own expression is finite, and its size depends only on the grammar. A
**place** is an occurrence of an expression in it, a call of a rule that the
parse does not memoize together with the body that replaces it being one
place. Every evaluation in the own region of $H$ is an evaluation of one
place, the outermost place, the body of $Q$, is evaluated once, by $H$, and
every other evaluation is made by an evaluation of the place that directly
encloses its place.

*Proof.* A call in the own region is of a rule that the parse does not
memoize, and each such call evaluates the body of its rule or of its twin
once. Suppose that two calls of such a chain are of one rule $X$.
The rules of the calls from the first to the second form a cycle of calls
$X = X_0, X_1, \ldots, X_m = X$, each $X_{i+1}$ called in the body of $X_i$.
Each $X_i$ is not memoized and calls a rule, so by the first exception of
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules) it is
called from at most one place in the grammar, and that place is the one in
the body of the rule before it on the cycle. Take the first call of the chain
whose rule is on the cycle. It is made in the body evaluation of the call
before it on the chain, whose rule is not on the cycle, or in $H$. Its place
is in the body of a rule of the cycle, so it is made in $H$, and $Q$ is a rule
of the cycle. Every rule of the cycle is the rule of a call of the chain,
which the parse does not memoize. Then the parse does not memoize $Q$, so $H$ is the evaluation
where the parse begins and $Q$ is a start rule that its own body calls through
rules called from one place, which the parse memoizes by the same exception,
a contradiction. So no rule occurs twice. The body of a twin is the body of its rule with, at
each call, the rule or the twin (*Counterparts in the reference parse*), so
the places of the two are the same. The rest follows from the definition of
the own region: an evaluation in it other than $H$ is an evaluation of a part
of the body of $Q$ or of a rule called in the own region, made by the
evaluation of the expression of which it is a part, or, for a body, by its
call. $\square$

**Activations that one evaluation holds.** Let $R$ call no rule, and let $H$
be a memoized evaluation of a rule $Q$ other than $R$ or of its twin. Let the
places of the own expression of $Q$ that enclose a call of $R$ be bodies,
sequences, choices, options, groups, captures, lookaheads and repetitions,
none of them a place inside an Operators Rule or a `#recover`. Then each
place $x$ that encloses a call of $R$ or is one is evaluated, in one
evaluation of the place $y$ that directly encloses it, at most once, or at
most $b$ times when $y$ is a repetition with an upper bound, or once in each
iteration when $y$ is a repetition without one. Let $I \ge 1$ be such that
every evaluation, in the own region of $H$, of a repetition without an upper
bound has at most $I$ iterations, the one that fails included, that contain a
successful activation of $\sigma$ at $q$ held by $H$. Then, outside case 5
of *Direct activations*,
$H$ holds at most $c\, b^{D} I^{d}$ successful activations of $\sigma$ at $q$,
where $c$ is the number of calls of $R$ in the own expression of $Q$, $D$ the
greatest number of places that enclose one of them, and $d \le D$ the
greatest number of repetitions without an upper bound among those; $c$, $b$,
$D$ and $d$ depend only on the grammar. When $Q$ is $R$, $H$ holds at most
one.

*Proof.* A successful activation of $\sigma$ at $q$ held by $H$ is direct in
a body evaluation of $R$ or of its twin in the own region of $H$, which is $H$
when $Q$ is $R$ and otherwise the evaluation of a place that is a call of
$R$. A body evaluation in an own region is no growth step (*Own regions*), so
by *Direct activations*, outside case 5, at most one such activation is
direct in it. This gives the last statement. For a place $x$, let $n(x)$ be
the number of evaluations of $x$ in the own region of $H$ that contain such
an activation. For the body of $Q$, $n = 1$ at most. Let $x$ be directly
enclosed by $y$, and let $x$ enclose a call of $R$ or be one. Each evaluation
of $x$ counted in $n(x)$ is made by an evaluation of $y$, which contains it
and so is counted in $n(y)$. One evaluation of $y$ makes at most one
evaluation of $x$, or at most $b$, or, when $y$ is a repetition without an
upper bound, one in each iteration, of which at most $I$ contain such an
activation. So $n(x) \le n(y)$, $n(x) \le b\, n(y)$ or $n(x) \le I\, n(y)$,
and along the at most $D$ places that enclose a call of $R$, $n$ of that call
is at most $b^{D} I^{d}$. The activations are at most the sum of $n$ over the
$c$ calls of $R$. $\square$

**Consecutive activations in one own region.** Let $R$ call no rule, let
$\sigma$ be no bounded scan, and let $H$ be a memoized evaluation. Let $\beta$
and $\beta'$ be successful activations of $\sigma$ at $q$ held by $H$, $\beta$
first, such that no successful activation of $\sigma$ at $q$ that starts
between them is held by $H$, and let no path that enters the growth again
lead from the abandoned unit named below to $\beta'$ (case 5). Let $N$ be the
innermost evaluation that contains both. Then:

1. $N$ lies in the own region of $H$, and the first abandoned evaluation $w$
   below $N$ on the path to $\beta$ is a committing unit in the own region of
   $H$ that starts at a position $s \le q$. From the end of $w$ to the start
   of $\beta'$ the parser starts no evaluation before $s$; the exception of
   the restarts is empty.
2. Let $\kappa$ be the continuation of $w$ that the parser evaluates from $s$
   the last time before $\beta'$. Then $\beta$ is direct in the evaluation
   $P$, inside $w$, of a call $c$ of $R$ or of its twin that $w$ and $\kappa$
   share, at a position $p$ with $s \le p \le q$, and $R$ reads no variable.
   So when $R$ reads a variable, $H$ holds at most one successful activation
   of $\sigma$ at $q$ outside case 5.
3. Let $c_1, \ldots, c_j = c$ be the calls that $w$ shares with $\kappa$ up
   to $c$, evaluated in $w$ at $\tau_0 = s, \ldots, \tau_{j-1} = p$; $j \le
   16$. Then one of two cases holds.
   - (i) $\beta'$ is direct in the evaluation that $\kappa$ makes of one of
     the calls $c_x$, $x \le j$, a call of $R$ or of its twin at
     $\tau_{x-1} \le p$. When $x = j$, it is direct in an evaluation of the
     call of $P$ at the position of $P$.
   - (ii) $\kappa$ evaluates $c$ at $p$ in an evaluation $P^\circ$ that ends
     before $\beta'$ starts. If no record of a scan site answers a walk of
     $\sigma$ in $P^\circ$, then $P^\circ$ holds, at the place of $\beta$ in
     $P$, a successful activation of $\sigma$ at $q$ that $H$ does not hold,
     and $P^\circ$ lies inside the evaluation of a call of a rule that the
     parse memoizes, made inside $N$ after $w$ ends.
4. Let $\beta$ and $\beta'$ lie in different iterations $it$ and $it'$ of an
   evaluation $T$ of a repetition. Then $N = T$ and $w$ lies strictly inside
   $it$, and in case (i) every iteration of $T$ that ends after the last
   start of $\kappa$ and before $\beta'$ starts ends at one of
   $\tau_0, \ldots, \tau_{x-1}$; so $it'$ starts at one of these at most
   sixteen positions.

*Proof.* $H$ contains $\beta$ and $\beta'$, so $N$ is $H$ or lies inside it.
An evaluation inside $H$ that contains $\beta$ is in the own region of $H$,
since $\beta$ is in no call, made inside $H$, of a rule that the parse
memoizes. So $N$ and every evaluation from $N$ down to $\beta$ are in the own
region. A growth is the evaluation of a call of a leader, which the parse
memoizes, so none of these is a growth or a growth step. The restarts
therefore give $w$, which is no dropped step, so it is a unit that starts at
$s \le q$, committing since $\sigma$ is no bounded scan, and since no growth
lies on the path from $N$ down to $w$, their exception is empty. This is 1.

$\beta$ and $\beta'$ are direct in body evaluations of $R$ or of its twin in
the own region of $H$, which are no growth steps, and in different ones by
*Direct activations*, outside case 5. So *Repeats* applies to them, with $w$
and $\kappa$ as here: $\beta$ lies in the evaluation $e$, inside $w$, of a
call that $w$ and $\kappa$ share, which starts at or before $q$. $e$ lies on
the path from $w$ down to $\beta$, in the own region, so its rule is one that
the parse does not memoize, and a shared call is of a rule that reads no
variable and that is memoized at the call or calls no rule (condition 2, as in
*Links to heads*). So the rule of $e$ calls no rule, the body evaluation in
which $\beta$ is direct is the body evaluation of $e$, and the rule is $R$ or
its twin, which reads no variable. $e$ starts at $p \ge s$, inside $w$. If
$R$ reads a variable, no call of it is shared, so $H$ holds no two successful
activations of $\sigma$ at $q$: of any two, the one that starts next after
the earlier among those that $H$ holds would be a $\beta'$ for it. This is 2.

For 3, the removal of the shared calls takes at most sixteen (condition 2),
so $j \le 16$. Every path of $\kappa$ that examines the input begins with the
shared calls, each with the result that it has in $w$ and each starting at
the end of the one before (*Direct activations*, case 2). So an evaluation
that the parser makes from the last start of $\kappa$ on and that examines
the input lies in the evaluation that $\kappa$ makes of one of $c_1, \ldots,
c_j$ at $\tau_0, \ldots, \tau_{j-1}$, or starts after $\kappa$ has evaluated
all of them. $\beta'$ starts after that last start and examines the input.
If it lies in the evaluation that $\kappa$ makes of $c_x$, that evaluation is
in the own region of $H$, since it contains $\beta'$, so the rule of $c_x$ is
not memoized by the parse and calls no rule, as for $e$ above; $\beta'$ is
direct in its body evaluation, and its rule is $R$ or its twin. This is (i).
Otherwise $\kappa$ evaluates $c_1, \ldots, c_j$ before $\beta'$ starts; the
memo answers no call of $R$, which the parse does not memoize (in a Document
as *Transient calls of a Document* in Lemma 3 reads it), so the last of
these is a body evaluation $P^\circ$ of $R$ or of its twin at $p$, which ends
before $\beta'$ starts. $P$ and $P^\circ$ are evaluations of calls of one
rule, up to the variant, with one level restriction, at one position, and
$R$ reads no variable and, calling no rule, is on no left-recursion cycle
and reads no growth. So $\mathcal{R}(P)$ and $\mathcal{R}(P^\circ)$ take the
same steps (*Counterparts in the reference parse*): the same expressions at
the same positions with the same success and end. $\beta$ is the counterpart
of a successful activation in $\mathcal{R}(P)$, so $\mathcal{R}(P^\circ)$
holds one at the same place, in no alternative that the parse skips by the
next character. $P^\circ$ makes no call, so the parse evaluates there what
$\mathcal{R}(P^\circ)$ evaluates, apart from the iterations that a record of
a scan site answers. Under the hypothesis on the records, $P^\circ$ holds a
successful activation $\beta^\circ$ of $\sigma$ at $q$ at the place of
$\beta$. It starts after $w$ ends, so after $\beta$, and before $\beta'$, so
by the hypothesis on $\beta$ and $\beta'$ it is not held by $H$. $P^\circ$
starts after $w$ ends and before $\beta'$, while $N$ is in progress, so it
lies inside $N$, and inside $H$. Since $\beta^\circ$ is not in the own region
of $H$, it lies in a call $g$, made inside $H$, of a rule that the parse
memoizes, whose evaluation contains $P^\circ$. That evaluation does not
contain $N$, which is in the own region, so it lies inside $N$. It is in
progress when $P^\circ$ starts. Had it started before $w$ ended, it would
contain $w$, which is in the own region, or have ended inside or before $w$.
So $g$ is made after $w$ ends. This is (ii).

For 4, $\beta$ and $\beta'$ lie in different children of $T$, and every
evaluation that contains both contains $T$, so $N = T$. $T$ evaluates $it'$
after $it$, so $it$ succeeded and is not the expression of a lookahead: it is
not abandoned, and $w$ lies strictly inside it. The rest is the argument of 2
of *Iterations at one scan*, which uses of its two activations only that they
lie in different iterations of $T$, that $w$ is the first abandoned
evaluation below $T$ on the path to the earlier one, and that the later one
lies in the evaluation that $\kappa$ makes of a shared call, here $c_x$: after
an iteration of $T$ ends, the parser starts no evaluation before its end
until $\beta'$, and from the last start of $\kappa$ to its evaluation of
$c_x$ the parser keeps the evaluations of $c_1, \ldots, c_{x-1}$ and examines
no other input, so an iteration that ends in that time ends between two of
them. $it'$ starts where the iteration before it ends. If that iteration ends
after the last start of $\kappa$, it ends at one of these positions. If it
ends before that start, $\kappa$ last starts while $it'$ is in progress, at
$\tau_0 = s$, which is no later than the end of $it$, since the parser
starts nothing before $s$ after $w$ ends, and no earlier than the start of
$it'$, since the parser starts nothing before the end of an iteration after
it ends; the start of $it'$ is no earlier than the end of $it$, so it is
$\tau_0$. $\square$

These claims prove no bound on the activations that one evaluation holds.
With the second claim, such a bound follows from a bound $I$ on the
iterations of one evaluation $T$ of a repetition in the own region that
contain an activation held by $H$. Take two such iterations with no other
between them, the last activation $\beta$ that $H$ holds in the earlier and
the first, $\beta'$, in the later; no activation between them is held by $H$,
so the third claim applies. Neither of its cases is bounded here. In case
(i) the later iteration starts at one of at most sixteen positions that the
unit of the earlier one fixes, but the unit of the later iteration fixes
positions of its own, which need not be among those of the earlier when it
shares a call or a literal inside the evaluation that the continuation makes
of a shared call (the refinement of *Iterations at one scan*). In case (ii)
the claim proves only that the continuation's evaluation of the call of $R$
lies in the evaluation of a call of a memoized rule made inside $N$ after
the unit ends, so that an evaluation other than $H$, nested in $H$,
holds the repeated activation; the claims above on $q$-frames and on the
holders of the activations concern that evaluation and what the parser does
after it. What the third claim adds is that these are the only two ways from
one activation held by $H$ to the next, that both pass through an evaluation
of $R$ at a position at which the abandoned unit evaluated a call of $R$
that it shares with the continuation, and that in case (ii) the activation
in between is held by another evaluation. The number of
memoized evaluations that hold an activation of $\sigma$ at $q$ is not
bounded by these claims either: it is the count of the heads of step 1 of
[Open Steps](#open-steps).

The three claims, and with them the leaf links, the runs, the $q$-frames and
the count of the heads of step 1, concern only a source in a rule $R$ that
calls no rule, which the parse does not memoize. They are needed because a
call of $R$ is evaluated again wherever it is made again. For a recorded
site in $R$ conditions (R2) and (R3) take their place: Lemma 17 counts the
iterations of its recorded walks, and the hypothesis on the records in case
(ii) marks where a record enters, since the evaluation $P^\circ$ may then
hold no activation at $q$. The claims remain needed for the sources without
records in a rule that calls no rule: a repetition there that is no recorded
site, a `#recover` whose skip is not reused, the walks
that a Document resumes, and, through the parse without records, the suffix
copies of a site whose iterations build values. If the parse memoized $R$ at
its calls, every link of such a source would be memoized, and *Links to
heads* and *Activations held by heads*, which use none of the claims on leaf
links, would apply to it as they do to a source in a rule that calls a rule;
that is not assumed here.

## Lemma 5 (Growth)

*Outline.* Under conditions 2, 3 and 5, and with the growths at the calls
that are scanning sources, those that are not left calls and the calls of a
leader in the body of another rule, bounded as scanning sources by the claims of Lemma 4
that count nothing, the
evaluations that Lemma 3 leaves out for memoized rules number at most a
constant times $K \hat{n}$: the growth steps, and the body evaluations of
memoized rules in a context of growths, those of a rule of a cycle other than
a leader outside a growth step included. Their local and scanned cost is at
most a constant times $K(L + 1)\hat{n}$ more than Lemma 4 counts. Lemma 4
counts the activations owned by memoized evaluations, those inside a growth
step included, and this lemma those owned by growth steps and by evaluations in
a context of growths, and the successful activations that a dropped step in a
part at the position of its growth or a path that enters the growth again
repeats. Its bounds come before
the count of Lemma 4 on the activations that memoized evaluations own, in the
order that [Lemma 4](#lemma-4-scanned-cost) states, so they must not rest on
it: they rest on the claims of Lemma 4 that count nothing, from *Failure after
a source* to *Nothing in a memoized link is a head* other than *Activations that do not succeed*, on
Lemmas 1 to 3 and on the memo. Outside contexts of growths, the memo bounds the
growths of a leader at one position, level and combination of the values of the variables that it
reads by two, one in lookahead state $1$ and one in state $0$, since a growth
stores its result ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)), which answers the later
calls there in the same lookahead state or a greater one.

A growth at $p$ takes at most two more steps than the length it consumes
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)). By
condition 3, no step evaluates a
scanning source at $p$ itself outside the left calls of rules of the cycle and outside
memoized calls of rules off the cycle. A step can still evaluate a scanning
source at a position after $p$ that it reaches without reading the seed, as
`[a-z]*` in the first alternative of `E = "x" [a-z]* "!" / E "+" T / "x"`, and
it does so at the same position in every step. Condition 3 does not bound
these; condition 2 does, as the following claim shows for a unit followed by a
growing unit.

**Sources at fixed offsets.** Let $u$ be a unit of a body of the cycle that
condition 2 checks as an alternative that is not growing, a unit that is not a
growing unit or one that the definition checks in both ways, that can fail
after one of its scanning sources has examined input or is the expression of
a lookahead, which condition 2 checks whether or not it fails, and that has
among its
continuations a part $g$ that can make a left call of the leader that grows at
$p$ before it consumes input, as a growing unit of that leader does, and let
$u$ and $g$ share no call. In every step of the growth at
$p$ except the first, the one whose seed is empty and, when $u$ is committing
for $g$ by its words or decides it, a constant number of others, an evaluation
of $u$ at $p$ examines at most a constant amount of the input.

*Proof.* As a continuation of $u$, $g$ examines the input from $p$ through the
seed. The seeds of the steps are the results of the steps before them and grow
longer at each step, so at most one of them is empty. In any other step after
the first, the seed consumed the character at $p$ on a path of $g$ through the
seed, so that character is among the first characters of $g$. By condition 2,
$u$ is committing for $g$, in one of four ways. If the first characters of $u$
and $g$ are disjoint, they do not include that character, so $u$ cannot consume
it, nor can a lookahead that it evaluates before it consumes input, and $u$
examines only the slot at $p$. If $u$ has no scanning source, it examines only
what Lemma 1 bounds. If the words of $u$ exclude $g$, then $u$ matches its word
$w$ at $p$ only in a step whose seed ends before the end of $w$, since a path of
$g$ through a longer seed would reach the end of $w$ on that input; there are at
most $|w|$ such steps, and in every other step $u$ fails within its word, after
examining at most the word and the character after it. If $u$ decides $g$, each
path of $u$ parts from each path of $g$ within three symbols at a decisive
symbol that at most one of them can pass, after symbols that both pass alike.
The left call of $g$ stands for the paths of the body of the leader, and the
seed is the match of one of them at $p$; the seed of each later step is a match
that begins with that of the step before and consumes at least one more
symbol, so apart from the first three steps with a non-empty seed, the path of
$g$ in the step passes its first three symbols, through the seed. $u$ then fails
at the decisive symbol or before it, after the symbols that it passed alike.
Each of these is cheap, and by *Evaluations that read no growing result* below
each costs a constant in every step after the first two that evaluate it alike,
except a call of a rule that calls no rule, which repeats its own scan;
*Decided units at the position of the growth* below shows that such a call
examines a constant amount of the input. $\square$

The proof does not ask how $u$ reaches the input at $p$. A path of $u$ that
reads the seed has the first characters of the leader among those of $u$,
since first characters are taken through rule calls, and the character at $p$
is among them, so on such a path only the last three ways apply, and they
bound $u$ on it as on any other. For the expression of a lookahead, $g$ is what
follows the lookahead, which the parser evaluates at $p$ when a positive
lookahead matches or a negative one fails, as after `&("A" [a-z]*)` in
`&("A" [a-z]*) E [a-z]`; the proof bounds what the expression examines,
whatever its outcome.

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
claim does not except, either such a pair once the shared calls that examine
no input are removed, or a growing unit followed by a growing unit.

**Calls shared with a growing unit.** Let $u$ and $g$ be as in the claim above,
except that they share a call, and let a step of the growth at $p$, other than
the first and the one whose seed is empty, evaluate $u$ at $p$ and evaluate a
left call of $g$ at $p$ before $g$ examines the input, as a growing unit does.
Then the shared calls with which the two begin and that examine no input in
the step end at $p$ in both. After them, either the two share no further call,
and *Sources at fixed offsets* applies to the parts of $u$ and $g$ after those
calls, or the next call that they share is a call at $p$ of a leader of the
cycle, which $u$ makes before it evaluates anything that examines the input,
through left calls of rules of the cycle. In the second case $u$ is a growing
unit, and the step evaluates it as one: its left call reads the seed or grows
a leader in the context of the growth.

*Proof.* In the step, $g$ evaluates before its left call at $p$ only what
examines no input there: predicates and calls of rules off the cycle that
match without examining the input, as `Z` with `Z = ""`. A shared call has the
same result in both, since the memo answers it the second time, or the rule
repeats only its own scan, or, for a leader called in a context of growths at
$p$, it grows again from the same growing results, since the two calls lie in
the same growths of its cycle, in the same steps (case 2 of Lemma 4), so the
second growth evaluates what the first evaluated and costs at most what the
first cost. So each such call that
$u$ shares ends at $p$ in $u$ as well and examines no input there. Condition 2 compares $u$ and $g$ after
the shared calls are removed from their starts, one after another, so the
parts after them start at $p$, are compared as the unit and the continuation,
and $g$ still makes its left call at $p$ through the seed; when they share no
further call, *Sources at fixed offsets* applies to them as stated, and what
$u$ evaluated before them examined no input.

Otherwise, the next call that $g$ evaluates after those calls and that can be
shared is its left call at $p$, directly or through calls of rules of the
cycle: a left call of the leader that grows at $p$ reads the seed, and one of
another leader grows it in the context of that growth. A call of a rule of the
cycle other than a leader at a left call is not memoized at the call, since
such a rule is memoized only at calls that are not left calls, and it calls a
rule, so it is not a call that a unit and a continuation share; the first thing
that its body evaluates is again a left call, after what examines no input. So
the next call that $g$ evaluates and that can be shared is a call of a leader
$R'$ at $p$, and $g$ examines no input before it, so no shared literal comes
before it either. By the definition of sharing, $u$ makes the same call at $p$
before anything else that examines the input, directly or as the first thing
that the bodies of the calls that it starts with evaluate. The body that holds
$u$ started at $p$, since the step started at $p$ and positions do not
decrease, so each call on the way from that body to the call of $R'$ is made
before its caller's body consumes input and is found by the analysis of left
calls ([Growths of Left
Recursion](../runtime/parsing.md#growths-of-left-recursion)). Each rule on the
way is reached from the rule of $u$ through such calls and reaches it through
$R'$, which is on the cycle, so it is on the cycle, and each of the calls is a
left call. So on the path of the step, $u$ evaluates a left call before it
examines the input, and what it evaluates before that call examines no input
there, whether it is a predicate or a call of a rule off the cycle that
matches without examining the input. The definition of a growing unit counts
such a unit, and since condition 2 also checks $u$ as an alternative that is
not growing, $u$ is one of the units that it checks in both ways. A unit that
is not an alternative and contains a scanning source is not such a $u$:
condition 2 rejects it, since it can evaluate a left call before it examines
the input. $\square$

So in the steps that the claim does not except, an evaluation of $u$ at $p$
that shares a call with $g$ either costs what *Sources at fixed offsets* allows
after shared calls that examine nothing, as the alternative `Z "a" [a-z]* "?"`
does against `Z E [a-z]` with `Z = ""`, which condition 2 compares as
`"a" [a-z]* "?"` against `E [a-z]`, or, as the alternatives `{ok} E R "!"` and
`Z E R "!"` do with `E "+" T`, it is an evaluation of a growing unit, which
condition 2 checks against the continuations of a growing unit and the
paragraphs below count with the other growing units.

The growing units of a step examine the input from the end of the seed. The
input from the end of one seed to the end of the next is examined by one
step, and what a step examines beyond the end of its own result is the
failure of a growing unit after a scanning source, which condition 2 checks.
A growth is itself a scanning source at each call of a rule of its cycle
other than a left call, and at each call of a leader in the body of another
rule, where it can start, so its repetitions are those of a scanning source: the
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
the paths of a unit of R that examine the input before a left call of R can
examine input in that step.

The growing units are not the only parts of a step that repeat at $p$. Before
a step goes on through a part that reads the seed, it can evaluate at $p$
parts that read no growing result: later alternatives after a growing unit
fails, as `"x" [a-z]* "!"` in `E = E "+" "x" / "x" [a-z]* "!" / E "-" "x" / "y"`,
a part that a unit goes on with at its start after a path through its left call
fails, as `M` through `F` in `E = F "+" X / Y; F = E "z" / M`, and what a
unit examines at $p$ before a left call without consuming input, as the `"y"`
of `("y")? E S "q"` when it does not match or a positive lookahead that
matches before the left call. The next two claims
bound them in all but a constant number of steps; in those steps, the first,
the one whose seed is empty, the last and those that the second claim counts,
a step costs one body evaluation, which Lemma 1 and the activations that the
step owns bound.

**Evaluations that read no growing result.** Let $x$ be an evaluation at $p$,
in a step of the growth at $p$, that calls no rule of the cycle at $p$. The
steps that evaluate $x$ with the same values of the variables that it reads
evaluate it alike: with the same outcome, over the same input and with the
same calls, and from the third of them on the memo answers each call of $x$
that is memoized at the call.

*Proof.* Only a call at $p$ of a rule of the cycle reads a growing result of
the cycle at $p$ ([Lemma 2 of the memoization
proof](memoization.md#lemma-2-reading-an-open-growth)), and $x$ makes none. A
call of $x$ at a later position finishes its own growth outside the growth at
$p$ and is memoized like any call there, and a call of $x$ at $p$ of a rule of
another cycle reads only the growing results of its own cycle
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)), which
belong to growths that were in progress when the growth at $p$ started and do
not change before it ends. The steps evaluate $x$ in the same lookahead state,
so with the same values of the variables everything that $x$ evaluates reads
the same input, results and values in each of them. A call memoized at the
call is outside every context of growths, and its result read no growing
result that changes. It stores its result in the first of them, or, when that
is the first call of its variant at its position, which may leave no result
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)), in the
second. A store never narrows the calls that the entry of its key answers,
whether a later store under the key is made in a context of growths or in
the other lookahead state (*Answered calls* in Lemma 4), so the memo answers
the call in every later step. $\square$

**Results that do not include the seed.** At most a constant number of the
steps of a growth whose values of variables do not change have a result that
does not include the seed.

*Proof.* Such a result is the match of a path of the body of the leader that
makes no successful left call at $p$ whose result includes the seed. On it,
the evaluations at $p$ call no rule of the cycle at $p$ outside the left calls
that failed and the left calls of other leaders whose results do not include
the seed, and the outcomes of the other evaluations at $p$ are fixed by the
previous claim; what follows them starts after $p$ and reads no growing result
of the cycle at $p$. The reads of those other growths are plain (*Plain
reads*, below), so each of their results is one of a constant number
(*Growths inside a step*, at the end of this lemma). So
the result is fixed by the alternatives that the path takes at $p$, in the
body of the leader and in the bodies of the rules of the cycle that its left
calls of rules other than leaders reach, which are a constant number of
choices, since every cycle of left calls passes through a leader, and by those
results. Each step except the last ends with a result longer than its seed, so
no such result ends two steps. $\square$

**Values bound from a growing result.** Let a step of a growth of a leader $R$
at $p$ have in effect at $p$ a binding, made at $p$ or inside a positive
lookahead at $p$, of a variable that an evaluation of the step reads, to a
value that reads a growing result of the cycle at $p$, or let a predicate at
$p$ read a capture of such a value made inside a positive lookahead at $p$
without a variable, as `{text($c) != "zz"}` after `&(c:E)`. Then $R$ is the only
leader of its cycle, and either no rule that $R$ reaches contains a scanning
source other than bounded scans, or no evaluation that reads the value
consumes input, and a step that succeeds on a path through the binding ends
the growth. So the values of the variables, and the outcomes of the
predicates at $p$, change in the growth only through such bindings and
captures. In the first case the steps evaluate no scanning source
outside the left calls, so the claims of this lemma on scanning sources and
on failures after them have nothing to bound in the growth, and the claims
on nested growths do not apply, since those need two leaders. In the second
case a step whose path goes through the binding or the capture either fails
on that path, which undoes the binding, or ends the growth, so the claims,
*Results that do not include the seed* among them, apply to the other paths,
on which the values do not change, except that the steps that *Failures at the
position of the growth* leaves out include one for each key at $p$ of a
memoized rule that reads the value, at most $K$ for each rule.

*Proof.* The values that the step reads at $p$ are those of the call that
starts the growth, the same in every step, apart from the bindings that the
step makes that are in effect at $p$. A binding whose value reads no growing
result binds the same value in every step (*Evaluations that read no growing
result*). One whose value reads one, through a capture, the value of a call
whose result includes a read, or the value of a call that holds a capture made
in a lookahead of its own, as `y:M` with `M = &(c:E) => $c`, is in effect at
$p$. When the seed is not empty, the read ends after $p$, so it lies in a
positive lookahead at $p$; a negative lookahead undoes its bindings
([Variables](../language/predicates.md#variables)). When the seed is empty,
the read matches nothing and the binding needs no lookahead, but that is the
step whose seed is empty, which the claims of this lemma leave out. The same
holds for a capture that a predicate at $p$ reads without a variable. The read is a
call at $p$ of a rule of the cycle, so a left call, made in a body of the
cycle (*Calls in a context of growths*), and condition 5 forbids a lookahead
that makes a left call in a cycle with more than one leader. The variable is
read by an evaluation of the step, so the cycle reads it, and its binding may
not run before a call into the cycle that may run before consuming input
([Left Recursion](../language/parser-expressions.md#left-recursion)): no
evaluation that the binding reaches makes a call at $p$ of a rule of the
cycle, and the value reaches the reads of the growing result neither directly
nor through a binding that it reaches. A capture that a predicate reads
without a variable has its readers after the lookahead: the predicates and
the action of the rule, and through its value the callers, so for it only
the paragraphs below are needed.

The expression of the lookahead reaches every rule of the cycle through its
left call, and every rule that $R$ reaches. If one of those contains a
scanning source other than a bounded scan, the expression contains it, and
condition 2 checks it, as the expression of a lookahead, whether or not it
fails. It cannot evaluate the left call before it examines the input, since
it is not an alternative; it is not decided, since only a unit that is not a
lookahead is; and it has no word on the path to the left call, which comes
before it consumes input. The left call is not among the calls that it shares with its continuations,
since a read of the seed inside the expression of a lookahead is not shared
with what follows the lookahead ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost)). So, after the calls
that it shares with its continuations, which end at $p$ before the left call,
its first characters
and those of its continuations are disjoint, or no scanning source remains
in it, which the left call that remains rules out. Its first characters
include those of $R$, and in a step with a non-empty seed the character at
$p$ is among them, so nothing that the parser evaluates after the lookahead,
through the ends of the enclosing expressions and the bodies, consumes it.
Every evaluation that reads the value, directly or through the value of a
rule whose body holds the binding, comes after the lookahead on the path
through the binding, so none consumes input, and a step that succeeds on
such a path ends at $p$, with a result no longer than its seed, and ends the
growth.

In the first case, every evaluation of a step lies in a body of a rule
that $R$ reaches, and a nested growth at $p$ needs a second leader of the
cycle (*Calls in a context of growths*). In the second case, an evaluation that reads the value consumes no input, so it
fails after a scanning source has examined input only inside a memoized call
at $p$ (condition 3), which the memo answers in every step after the first
with the same key, and the steps whose path through the binding fails end
as where it fails, with values that do not change. $\square$

**Failures at the position of the growth.** Leave out the steps of *Results that
do not include the seed* and of *Values bound from a growing result*, and
the first, the one whose seed is empty and the last. In every other step, let
$x$ be an evaluation at $p$ that calls no rule of the cycle at $p$ and fails
after one of its scanning sources has examined input. Then, apart from a
constant number of steps for each unit and each memoized rule, $x$ examines
at most a constant amount of the input.

*Proof.* By induction, first on the number of leaders that do not grow at $p$
around $x$, then on the number of evaluations that the step makes after $x$
and before the successful left call below: the claim holds for an evaluation
that lies in a step of a growth of another leader that the step nests at $p$,
and for those that come later in the step. The first number is finite, since
the growths at $p$ that enclose one another have distinct leaders (*Calls in a
context of growths*), and the second is finite, since a step is. The step goes on after $x$ fails and ends with a result that includes
the seed, so it makes a successful left call at $p$ after $x$. After a unit
fails, the parser evaluates only its continuations at the unit's start, so let
$U$ be the outermost unit that holds $x$ and fails before the step makes that
call. The parser then evaluates the continuations of $U$, in the order of their
choice, and one of them, $g$, evaluates that left call before it consumes
input: if each one evaluated before $g$ failed and no later one could make the
call, the choice would fail and so would a unit around $U$, against the choice
of $U$. Since $g$ makes a left call before it consumes input, the first
characters of the leader are among its own, and as in *Sources at fixed
offsets* the character at $p$ is among them.

$U$ is checked as an alternative that is not growing. On the path of the step,
either $U$ makes no left call at $p$ before $x$, and then it examines the input
before its left call, since $x$ examines input, so it is not a growing unit or
is one of the units that the definition checks in both ways; or a left call of
$U$ at $p$ comes before $x$. That call is not inside a lookahead, nor inside
another unit that is not an alternative: such a unit would evaluate a left
call before it examines the input, and it would contain the scanning source of
$x$, since the call reaches every rule of the cycle and, through them, the
body that holds $x$, so condition 2 rejects it. Outside a lookahead the
position does not go back, and a successful left call at $p$ ends at the end of
a non-empty seed or of a longer result, so since $x$ is still at $p$, a path
through that left call failed and $U$ went on at its start. The definition
then checks $U$ as an alternative that is not growing on the paths that go on
there, from there, $x$ among them. If $U$ and $g$ share no call,
*Sources at fixed offsets* applies to $U$ and $g$ on those paths, and bounds
the input that $U$, and so $x$, examines at $p$ by a constant outside a
constant number of steps.

Otherwise let $P$ be the part of $U$ that condition 2 compares with $g$ on the
path of the step to $x$: $U$ itself when no left call of $U$ at $p$ whose path
failed comes before $x$, and otherwise the path that goes on at the start of
$U$ after the last such call. If $P$ and $g$ share no call, *Sources at fixed
offsets* applies to them as to $U$ and $g$ above. Let them share the calls
$c_1, \dots, c_k$, in the order in which condition 2 removes them from their
starts, the shared literals included. Each $c_i$ has the same outcome and
examines the same input in $P$ and in $g$: the memo answers it the second
time, or its rule calls no rule and reads no variable, so that its evaluation
depends only on its position and the input, or it is a call of a leader that
grows again in the context from the same growing results, since the two calls
lie in the same growths of its cycle (*Calls shared with a growing unit*).
Each side evaluates the shared calls before anything else that examines the
input, and $x$ examines input, so $x$ lies after $c_1, \dots, c_k$ or inside
one of them.

- *After the shared calls.* Each $c_i$ ends in $P$ before $x$ starts, at $p$,
  so it ends at $p$ in $g$ as well, and $g$ reaches its left call at $p$ after
  them without consuming input; none of them is a read of the seed, which ends
  after $p$. The parts of $P$ and $g$ after them start at $p$, share no further
  call, and are compared by condition 2 as the unit and the continuation, $g$
  still making its left call at $p$ before it consumes input, whether or not it
  examines the input first. So *Sources at fixed offsets* applies to them, and
  bounds the input that $x$ examines at $p$ by a constant outside a constant
  number of steps.
- *Inside a shared call.* Let $x$ lie inside $c_i$. The calls before $c_i$ end
  at $p$ on both sides, as above, so $c_i$ is a call at $p$ on both sides. If
  its rule is on the cycle, it is a leader $R$ that grows in the context, since a
  read of the seed evaluates no body and a left call of another rule of the
  cycle is not shared, and $x$ lies in a step $t$ of that growth, which the
  step nests at $p$. The claim is not applied to the growth of $R$ with its own
  exceptions: of the second and third kinds of *Growths inside a step*, that
  growth starts anew in each step at $p$, so its first, empty-seed and last
  steps recur once in each of them. The argument above is applied instead to
  $x$ in $t$ with the seed $s$ of the step at $p$, which is not empty, in place
  of the seed of $R$. Let $U'$ be the outermost unit that holds $x$ and fails in
  $t$ before $t$ ends or fails. The successful left call of the step lies inside
  $c_i$ or after it in $g$.
  - When the call lies inside $c_i$, the result of $c_i$ includes the growing result $s$, so the
    growth of $R$ is of the second kind, and each of its results from the first
    that is not empty includes $s$ and begins with it, since the read starts at
    $p$ and what comes before it consumes nothing. If $t$ makes after $x$ a
    successful read at $p$ of a result that is not empty, that result is $s$ or
    a seed of $R$ that begins with $s$. Otherwise the paths of $t$ through its
    reads of the seed of $R$ fail or keep the position at $p$, and $t$ ends with
    a result that passes through no read of a result that is not empty, which by
    *Steps that begin anew* is the result of the first step of $R$, so that
    result is empty; the step with the empty seed then ends with a result that
    includes $s$, through a left call of $R$ that reads the empty seed and is
    followed, before it consumes input, by a read of $s$. Since the result of
    the first step is empty, the continuation of $U'$ in $t$ reaches the end of
    the body of $R$ without consuming input, and goes on through every call of
    $R$, that left call included, wherever it lies in the body, also in an
    alternative before $U'$: what follows that left call is a continuation of
    $U'$.
  - When it lies after $c_i$, $g$ makes the left call before it consumes input, so $c_i$
    ends at $p$ and the growth of $R$ ends with an empty result. The part of $g$
    after $c_i$, through the end of the body of $R$ and the call $c_i$, is a
    continuation of $U'$, or of the step $t$ as a unit when $t$ is the step with
    the empty seed and fails, whose continuations follow the calls that start
    the growth at the end of that seed, here $p$.

  In each case $U'$, or the step, has a continuation $g'$ that makes a left call
  at $p$ before it consumes input and has a path through $s$ or through a seed
  of $R$ that begins with $s$. The argument above, with $U'$ and $g'$ in place of
  $U$ and $g$, then bounds $x$ as in *Sources at fixed offsets*, with $s$ in
  place of the seed: the character at $p$ is the first of $s$; a word $w$ of
  $U'$ is matched at $p$ only in the steps at $p$ whose seed ends before the end
  of $w$; and a path of $g'$ through $s$ passes its first three symbols apart
  from the first three steps at $p$ with a seed that is not empty. A further
  shared call that holds $x$ is handled by the induction, on a leader that
  grows inside $R$ or on a later evaluation. So the steps left out are steps
  at $p$, a constant number for each unit, whichever step of $R$ holds $x$. A
  growth of the first kind takes the steps of an earlier growth of $R$ in the
  context again, so it evaluates $x$ again in none of them. If its rule is
  off the cycle and memoized at the call, it calls no rule of the cycle at $p$,
  since a rule reached from the cycle through left calls that reaches the
  cycle again through left calls is on it, and it reads no variable, so by
  *Evaluations that read no growing result* the memo answers it in every step
  after the first two that evaluate it, and $x$ is evaluated in two steps of
  the growth at most. Otherwise its rule calls no rule, and $g$ evaluates $c_i$ at
  $p$ again in the same step, before its left call, which comes after $c_i$ in
  $g$ since $c_i$ examines input. That evaluation evaluates an evaluation $x'$
  of the expression of $x$ at $p$ again, over the same input and with the same
  outcome. $x'$ comes after $x$ in the step and before its successful left
  call, and it fails at $p$ after a scanning source has examined input as $x$
  does, so by induction it examines at most a constant amount of the input
  outside the steps that the claim leaves out, and so does $x$.

So a pair that shares a call is bounded as one that shares none: the steps
left out are those of *Sources at fixed offsets* for the parts after the
shared calls, and one step for each memoized rule. $\square$

**Decided units at the position of the growth.** In the steps that
*Failures at the position of the growth* does not leave out, let the unit $V$
that holds $x$ be decided by its continuation $g$, and let $C$ be a call of a
rule that calls no rule that $V$ passes alike before it parts. Then $C$
examines at most a constant amount of the input, and $V$ fails at $p$ at the
decisive symbol, within the symbols that the alternative of a decisive
negative lookahead reads, or inside a unit that condition 2 checks at $p$.

*Proof.* The step reaches $V$ at $p$ through symbols that it passes at $p$
without consuming input, its **route**: the left calls on the way, the
lookaheads and the parts that match nothing. The left call of $g$ stands for
the paths of the body of the leader, so $g$ has a path made of its symbols
before that call, which pass at $p$ without consuming input since $g$ makes
the call before it consumes input, then the route, then the path $P$ of $V$
in the step: the copy of $P$ that the left call reaches. $V$ must part from
it. When $g$ has no symbol before its left call and the route is empty, $P$ is
compared with itself; the
two pass the same symbols alike and reach the same symbol, at which either
both go on or neither can, and neither is decisive ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost)), so $V$ is not
decided. So the copy is preceded by at least one symbol. Each symbol of $P$
that is passed alike faces one of those symbols, which the step passed at $p$
without consuming input, or an earlier symbol of $P$; by induction along $P$, both are at $p$
and consume nothing, since symbols passed alike evaluate alike at one
position. So $C$ is a call at $p$, evaluated in each step in a body of the
cycle or in a call at $p$ that is not memoized at the call. If $C$ is inside
a call memoized at the call of a rule off the cycle, so is $V$, and by
*Evaluations that read no growing result* the memo answers that call in every
step after the first two that evaluate it. Otherwise the rule of $C$ calls no
rule and is not memoized, so by condition 3 it holds no scanning source, and
$C$ examines what Lemma 1 bounds.

$V$ also fails at $p$ at the decisive symbol. The symbol on the side of the
copy was passed at $p$ by the step: it precedes the copy or is an earlier
symbol of $P$. If it is a literal, a class or a symbol whose first characters
are disjoint from those of the symbol of $V$, $V$ fails there, at $p$. If it
is a positive lookahead $\&K$ that matched at $p$, the next symbol of $V$ is
disjoint from $K$ and fails at $p$. If it is a negative lookahead $!K$ whose
expression failed at $p$, the rest of $V$ must match $K$ in the form that
condition 2 gives a decisive $!K$: every path of the rest of $V$ passes, in
order, the symbols of one alternative $A$ of $K$, made of calls, literals,
classes and `.`, and no alternative of $K$ before $A$ has a cut that commits
the choice, so $K$ failed only after it tried $A$, and $A$ failed at $p$.
Each symbol of $V$ that passes literals, classes or `.` of $A$ consumes,
character by character, what those symbols of $A$ match in order, at the
same positions; each call of $V$ that passes a
call of $A$ is alike with it there, with the same outcome; every other call
or repetition on the way stands for its paths, which pass the rest of $A$ in
the same way; and the lookaheads that $V$ passes over cost a constant. So no
evaluation of $V$ gets past the symbol at which $A$ failed: $V$ fails within
the symbols that $A$ reads, at $p$, and examines only input that $A$
examined. $K$ is the expression of a lookahead and so a unit, which can be
committing in any of its ways, by its words among them; this case uses only
the form of a decisive $!K$, not the way in which $K$ is committing. The
literals, classes and lookaheads of $V$ there cost a constant. Each call of
$V$ that passes a call of $A$ is alike with it, a call of the same rule, and
holds no scanning source: when $!K$ precedes the copy, it is on the side of
the continuation, where the form of a decisive $!K$ excludes a call of $A$
that the unit passes alike and that contains a scanning source; when it is
an earlier symbol of $P$, it is a negative lookahead that $V$ passes before
it parts, and $K$ contains no scanning source at all. So each such call
examines what Lemma 1 bounds, the memo answering it or not.

A symbol of $V$'s side that decides, a dead symbol, a lookahead or the end of
the parse, fails $V$ at $p$ as well: a negative lookahead of $V$'s side
contains no scanning source by the form of a decisive $!K$, so it examines
what Lemma 1 bounds, and a positive lookahead of $V$'s side is evaluated by
$V$ at $p$ as part of the unit that condition 2 checks. So $V$ examines at most a constant amount of the input beyond
what Lemma 1 bounds in every step that the claims above do not leave out.
$\square$

So in each step, apart from a constant number of steps for each growth and
each unit, what a step evaluates at $p$ outside the parts that read the seed
costs a constant beyond its local cost: the later alternatives after a failed
growing unit, the parts that a unit goes on with at its start, as through `F`,
and what a unit examines at $p$ before a left call without consuming input, as
the `"y"` of `"y"? (E / "z") "q"` when it does not match, or as a positive
lookahead that matches before the left call, such as `&("A" [a-z]*)` in
`&("A" [a-z]*) E [a-z]`, which is a unit that *Sources at fixed offsets*
bounds with what follows it as its continuation. When the `"y"` or a
part before a call that reaches the unit does match, the unit starts after
$p$, outside every growth of the cycle at its position, and is a plain
alternative there, which the cases of Lemma 4 bound as the paragraph above
states.

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
  of the values of the variables that R reads, once in lookahead state $1$ and
  once in state $0$
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

**Calls in a context of growths.** Let a growth of a leader $S$ be in progress
at $p$. Every call at $p$ of a rule of its cycle made while it is in progress
is made in a step of that growth, or of a growth of another leader that a step
starts at $p$, through left calls of rules of the cycle from the body of the
leader of that step. Outside an Operators Rule, a body evaluation at $p$ makes
each call of its body at $p$ at most once, so a step makes at most a constant
$c_5$ of evaluations in the context of the growth outside the steps of the
growths of other leaders that it starts, and those growths are nested at most
as deep as the cycle has leaders.

*Proof.* The growth starts at $p$ and ends before its caller goes on, so a
call made while it is in progress is made inside one of its steps, each of
which evaluates the body of $S$ from $p$. Positions do not decrease, so every
body on the way from that body to a call at $p$ started at $p$, and each call
on the way was made before its caller's body consumed input. The analysis of
left calls finds each of them ([Growths of Left
Recursion](../runtime/parsing.md#growths-of-left-recursion)), and each rule on
the way is reached from $S$ and reaches the rule called at $p$, which is on
the cycle, so it is on the cycle and the calls are left calls. In one body
evaluation, an expression starts at $p$ at most once: a choice and a lookahead
go back only to the start of one of their own parts, a repetition stops at the
first iteration that consumes nothing
([Repetition](../language/parser-expressions.md#repetition)), so at most one of its
iterations starts at $p$, and a `#recover` evaluates its base and its skip
expression once each. So a body evaluation at $p$ makes at most $b$ calls at
$p$, where $b$ is the largest number of calls in a body. A left call of $S$
reads the growing result and evaluates no body. A left call of a rule other
than a leader is not memoized in the context and evaluates its body; a chain of
such calls has fewer than $|\text{rules}|$ calls, since every cycle of left
calls passes through a leader. A left call of another leader $R$ reads the
growing result of $R$ when $R$ grows at $p$ already, and otherwise grows $R$
there, in the context, or takes the steps of an earlier growth of $R$ in it.
So outside those growths, the evaluations of a step in the context form a tree
of fewer than $|\text{rules}|$ levels with at most $b$ children at each, and
$c_5 = b^{|\text{rules}|}$ bounds them. A growth of $R$ inside a growth of
another leader at $p$ starts no growth of a leader that grows at $p$ already,
so the growths at $p$ that enclose one another have distinct leaders. $\square$

A **read** of the growing result of a leader $R$ at $p$ is a call of $R$ at
$p$ while $R$ grows there, made in a step of $R$ or in a step of a growth that
such a step nests at $p$. A result **includes** that growing result when its
match passes through a read that succeeds, directly or through the result of a
call that includes it.

**Steps that begin anew.** Let the values of the variables not change in a
growth of a leader $R$ at $p$, and let its reads be **plain**: no lookahead
evaluated at $p$ in its steps, or in the steps of the growths that they nest
at $p$, reads the growing result of the leader of the step that it is in, and
no cut in a body evaluated at $p$ is passed after such a read or after a call
whose result includes one. Let $\bar{r}$ be the first result
of the growth that is not empty, the result of its first or second step. Then
every result of the growth after $\bar{r}$ begins with the result before it, so
every result from $\bar{r}$ on includes $\bar{r}$. A growth of another leader $T$ that
a step of $R$ nests at $p$ either has only results, from its first result that
is not empty on, that include the growing result of $R$, or reads it only on
paths that fail and takes the steps it takes where that read fails.

*Proof.* By induction on the number of leaders that do not grow at $p$, the
claim for $T$ first. In a step of $R$, the evaluations at $p$ read the same
input and values in every step, and only the reads of the growing result of
$R$ return something else: in the first step they fail, and once a result
that is not empty is the growing result, they succeed and the path goes on
after $p$. Positions do not decrease, and outside a lookahead nothing that
follows a call goes back to the position where the call started, other than a
failure that goes back to a choice, an option or a repetition, as a failure of
the call itself would; the bindings made after that point are undone
([Variables](../language/predicates.md#variables)), and no cut is passed
after the read. A step of a nested growth of
$T$ goes back to $p$, but by the claim for $T$, $T$ either returns a result
that includes the growing result of $R$, which ends after $p$, or returns what
it returns where that read fails. So in a step after $\bar{r}$, every path at $p$
through a read either succeeds with a result that begins with the growing
result, or fails, or behaves as where the read fails. If the step ends with a
result that does not begin with the growing result, it took the same choices at
$p$ as the first step and ends with the result of the first step, which is not
longer than its seed, so the growth ends with it. The results from $\bar{r}$ on
each begin with the one before, so they include $\bar{r}$. For $T$, the same
argument applies to its steps, with the reads of the growing result of $R$ in
the role of the first step's failing reads: if the first result of $T$ that is
not empty passes through a read that succeeds, every later one begins with it
and includes it; if not, every path through such a read in a step of $T$ fails
or behaves as where the read fails, so the steps of $T$ are those it takes
where the read fails. $\square$

**Plain reads.** Under condition 5, the reads of every growth of a leader $R$
at $p$ whose steps nest, or are nested in, a growth of another leader at $p$
are plain.

*Proof.* The growths of two leaders nest at $p$ only when both are on one
cycle with more than one leader (*Calls in a context of growths*). A read at
$p$ is a call at $p$ of a rule of the cycle, so it is a left call, made in a
body of the cycle or inside a call of a rule of the cycle at $p$, which is a
left call too; a body off the cycle evaluated at $p$ makes no call at $p$ of a
rule of the cycle. A lookahead that reads a growing result, or the `skip` of
an Operators Rule of the cycle that reads one, therefore makes a left call in
a body of the cycle, which condition 5 excludes. A cut commits
only the innermost choice, option or repetition around it in its own rule
([Cut](../language/parser-expressions.md#cut)). A cut whose committed
alternative, iteration or option does not hold the read before the cut is
passed whether or not the read succeeds, or in a choice that starts after the
read; passing it fails that choice only where it fails when the read fails as
well, so the paths through the read behave as the proof of *Steps that begin
anew* states. A cut whose committed part holds the read, or a call whose
result includes one, before the cut is passed after a left call of that part,
which condition 5 excludes. $\square$

So the claims of this lemma that apply *Steps that begin anew* to the growths
that nest at $p$ need no hypothesis on the reads: condition 5 makes them plain. Without that clause of condition 5, a cut passed after a
read can make the enclosing choice fail where the first step went on, and a
lookahead can change its outcome between steps, so that a later step takes an
alternative that the first step did not reach.

**Growths inside a step.** Let a step of the growth of a leader $S$ at $p$,
after the first, start a growth of another leader $R$ at $p$, let $e$ be the
end of the seed of the step, and let the values of the variables not change in
the growths of $R$ and $S$. Their reads are plain (*Plain reads*), so *Steps
that begin anew* applies to both. Then the growth of $R$ is of one of three
kinds.

1. It reads no growing result of $S$, or growing results that the parse finds
   equal to those that an earlier growth of $R$ in the context read, and it
   takes the steps of that growth again, at a constant cost for each step of
   $S$ beyond their first evaluation.
2. Its result includes the growing result of $S$, and it takes at most three
   steps more than the length by which its result passes $e$.
3. Its result does not include the growing result of $S$, and it takes at
   most two steps more than the length of its result, which is one of a
   constant number of results.

*Proof.* A step of $R$ from a result that read no growing result of $S$, or
the same growing results as an earlier step, is the step that an earlier
growth of $R$ in the context took from it ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)), and the growth of $S$ is
in progress, so the context keeps it; a run of steps that read only the result
of $R$ is taken at once. By *Steps that begin anew* for the growth of $S$,
applied to the growth of $R$ that its step nests, that growth either includes
the growing result of $S$ in all its results from the first that is not
empty, or reads it only on paths that fail and takes the steps it takes where
that read fails. In the first
case the results that include the growing result of $S$ end at $e$ or after
it and each is longer than the one before, so there are at most one more of
them than the length by which the last passes $e$, one step ends the growth,
and a first result that is empty, which includes no growing result that is not
empty, adds one more. In the second case, the steps of $R$ depend
on the growing result of $S$ only through the outcomes of the paths through
its reads: the reads are calls of $S$ at $p$, which return the same result in
every step of $R$, and what follows them starts at $e$, after $p$, where
nothing reads a growing result at $p$ (*Evaluations that read no growing
result*). By *Calls in a context of growths* there are at most a constant
number of such paths, so the outcomes take a constant number of combinations,
and each fixes the growth of $R$ and its result. The growth takes at most two
steps more than the length of its result ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)). $\square$

So the growths of $R$ of the first kind cost a constant for each step of $S$
beyond one growth. For the second kind, by condition 5, either the step of $S$
keeps the result of $R$, and the length by which it passes $e$ is at most what
the step adds to the result of $S$, so their sum over the steps of $S$ is at
most the length that the growth of $S$ consumes; or the step can throw the
result away, and $R$ passes $e$ only in the step where the growth of $S$
stops, once per growth of $S$. For the third kind, when the step of $S$ keeps
the result of $R$, the result of the step does not include the seed, which
*Results that do not include the seed* allows in a constant number of steps,
and the growth of $R$ takes at most two steps more than the length of that
result, at most $\ell + 2$ for a growth of $S$ that consumes $\ell$
characters. When the step can throw the result away, in the second kind where
the growth of $S$ stops or in the third kind in any step, a growth of $S$ that
stops can throw away what $R$ consumed, and the growths of $S$ at other
positions can cover that input again. The call of $R$ is a scanning source,
since it is a call of a leader in the body of another rule, so condition 2
checks the unit around it that fails after $R$ has grown, and the growth of
$R$ is an activation of that source, which the cases of Lemma 4 bound with
what the parser evaluates again after that unit. The same holds when the body
of a rule M of the cycle other than a leader calls R where no growth of the
cycle is in progress: the growth of R there is a scanning source of the unit
of M that holds the call. So a growth of $S$ that consumes $\ell$ characters
evaluates in its context at most a constant times $\ell + 1$ body evaluations,
apart from the growths at those sources, and the growths that it nests, which
have other leaders, are bounded in the same way.

**Reuse in a context.** The bounds of this lemma hold when the parse reuses no
result and no step kept in a context of growths. Such a reuse needs every
growing result that the result or the step read to be equal to the one there
now, and every lookahead that it observed to have the same outcome
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)). A parse
may compare two results by their structure only up to a bound and take results
that it has not shown equal as different, and where it records the input that
evaluations examine, it takes results with different examined ranges as
different too. Either way it evaluates again what it could have reused.

*Proof.* *Calls in a context of growths* bounds the evaluations of a step by
the calls of the bodies alone. *Evaluations that read no growing result* rests
on the memo, whose entries hold results that read no growing result and are
found by their key, not by a comparison of results. The second and the third
kind of *Growths inside a step* bound the steps of a growth of $R$ by the
length of its result, as every growth is bounded
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)), and charge
them to the input that the growth of $S$ consumes, to the constant number of
steps of *Results that do not include the seed*, or to a scanning source. A
growth of the first kind that the parse takes again in full, because it did not
find the growing results equal, is a growth of the second or the third kind:
by *Steps that begin anew*, every growth of $R$ that a step of $S$ nests either
includes the growing result of $S$ in all its results from the first that is
not empty, or reads it only on paths that fail, or not at all. So it is
bounded as those kinds are, and the comparison changes the constant of the
bound, not its form. $\square$

**Repeats inside a growth.** The restarts of Lemma 4 leave out two successful
activations whose innermost common evaluation is a growth. Let $\alpha$ and
$\alpha'$ be successful activations of a scanning source $\sigma$ that start at
$q$, $\alpha$ first, let the innermost evaluation that contains both be a
growth at $p$, with $\alpha$ in its step $j$ and $\alpha'$ in a later step
$j'$, and let $e_k$ be the end of the seed of step $k$. Then step $j$ is kept,
and one of two cases holds.

- *After the seed.* $\alpha'$ follows a read of the seed of step $j'$, a read
  of the growing result that succeeds or a call whose result includes one
  (*Restarts* in Lemma 4). Then the path
  from step $j$ down to $\alpha$ holds a first abandoned evaluation $w$, a
  dropped step of a growth or a unit that starts at a position $s$ with
  $s \le q$ and $s \le e_{j+1}$, and the conclusion of the restarts holds for
  $\alpha$ and $\alpha'$ with this $w$, where the continuation of $w$ that the
  parser evaluates goes on, at the end of the body of the leader, through the
  reads of the seed in the steps after $j$.
- *At the position of the growth.* Otherwise $\alpha'$ follows no such read,
  and so lies in a part that step $j'$ evaluates at $p$ and that follows no
  such read either, as after a lookahead that reads the seed and returns to $p$:
  the parts at $p$ that the claims of this lemma before *Calls in a context of
  growths* bound. Then step $j'$ is one of the constant number of steps of the
  growth that those claims leave out, or $q$ is at most a constant after $p$.

*Proof.* A growth drops at most its last step, so step $j$, which comes before
step $j'$, is kept, and its result, from $p$ to $e_{j+1}$, is the seed of step
$j + 1$; the seeds grow longer, so $e_{j+1} \le e_{j'}$. In the first case
$\alpha'$ starts at or after the end of a read that succeeds or of a result
that includes one, so $q \ge e_{j'} \ge e_{j+1}$. If no evaluation on the path
from step $j$ down to $\alpha$ were abandoned, the input that $\alpha$ consumes
from $q$ would lie in the result of step $j$, which ends at $e_{j+1}$, so $w$
exists. As in the restarts, the evaluations above $w$ on that path succeed,
are no expression of a lookahead and no dropped step, so $w$ is a dropped step
or a unit, and its parent goes on after it at $s$, inside the result of step
$j$, so $s \le e_{j+1}$. From the end of $w$ the parser returns to no position
before $s$ in step $j$, apart from the exception of the restarts for the
growths between step $j$ and $w$, by the argument of the restarts, and then
starts the steps after $j$ at $p$; in each of them, an evaluation that follows
a read of the seed starts at or after the end of the seed, at or after
$e_{j+1}$, and so at or after $s$, and the others follow no read of the seed,
as in that exception. Up to
$\alpha'$, which follows such a read, the parser therefore evaluates, from
$s$ on, what follows $w$ in step $j$ up to the end of the body of the leader
and then what follows the reads of the seed, step after step: this is a
continuation of $w$ in the sense of [Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost), which continues at the
end of a rule body through every call of the rule, the reads of the seed
included. So condition 2 checks $w$ against it, the parts at $p$ that the
steps evaluate in between are no part of it, and the arguments of *Direct
activations* and *Repeats*, which use of the continuation only that the parser
consumes through it the input from the end of the shared calls to $q$, apply
to $w$ and that continuation. In the second case $\alpha'$ lies in no part
that goes on from the end of a read that succeeds, so in a part that step
$j'$ evaluates at $p$, a lookahead that reads the seed and returns to $p$
included, through the later
alternatives after a growing unit fails, the parts that a unit goes on with at
its start or what a unit examines at $p$ before a left call; *Evaluations that
read no growing result*, *Results that do not include the seed*, *Sources at
fixed offsets*, *Calls shared with a growing unit* and *Failures at the
position of the growth* bound these as stated, whether $\alpha'$ lies in the
own region of the step or in a call that the part makes. $\square$

So the sequence $\alpha_1, \alpha_2, \ldots$ of Lemma 4 leaves out, beside case
5 and the third case of *Dropped steps*, the activations of the second case: an activation
that lies in a step of a growth, on a path that goes on from the end of no
growing result that succeeds, after an activation of $\sigma$ at $q$ in an earlier step of that
growth. Two activations of the sequence whose innermost common evaluation is a
growth are then in the first case, so the claims of Lemma 4 apply to them as
to the others. This lemma counts the activations of the second case. In a step
that the claims at $p$ do not leave out, they lie in parts that examine at most
a constant amount of the input, so they number at most a constant for each
step. In the constant number of steps of a growth that the claims leave out,
the parts that hold them read no growing result and are evaluated alike in
every step that reaches them (*Evaluations that read no growing result*), so
at each position they number at most a constant times what one evaluation of
those parts holds there, which *Consecutive repeats inside a family* below
bounds by a constant for each family of growths. Each of them follows an
activation of $\sigma$ at $q$ in an earlier step of the same growth, so a
growth that holds one also holds one of the sequence, or one of case 5 or of
the third case of *Dropped steps*, and they add no growth to the count over the
positions below. An activation $\alpha'$ of that third case is the next
successful activation of $\sigma$ at $q$ after an activation $\alpha$ that a
dropped step holds in a part that it evaluates at $p$ without reading a
growing result that succeeds, so distinct ones follow distinct such $\alpha$,
and $q \ge e$ beyond the seed. These parts are those of the second case, and
the same claims bound the activations of $\sigma$ at $q$ that they hold: at
most a constant for each step in a step that the claims at $p$ do not leave
out, which, beyond the seed, leaves only the steps of growths at most a
constant before $q$, and in the constant number of steps of a growth that the
claims leave out, at most a constant times what one evaluation of those parts
holds at $q$ (*Consecutive repeats inside a family*). So these $\alpha'$
number at most the activations that dropped steps hold in such parts, which
these claims count with those of the second case, a growth having at most one
dropped step.

**Families of growths.** A growth of a leader at $p$ is **outer** when no
growth of its cycle is in progress at $p$ where its call is made. The
**family** of a growth $g$ at $p$ is its steps with every growth step and every
evaluation in a context of growths that the parse makes at $p$ inside them, and
the **family region** of $g$ is the part of the family outside the calls that
are memoized evaluations and outside the growths at other positions. An
activation owned by a growth step or by an evaluation in a context of growths
lies in the family region of exactly one outer growth: its owner is at a
position $p'$ where a growth of the cycle of its rule is in progress, so it is
made in a step of the first of those growths, which is outer (*Calls in a
context of growths*), and it lies in no memoized evaluation inside that step,
which would own it, nor in a growth at another position, whose steps would.
A **growth of a key** is a growth whose call reads no growth open at its start;
its key is the leader, position, level and combination of the values of the
variables that the leader reads. An outer growth is one, since its call can
read only a growth of its cycle at $p$ ([Lemma 2 of the memoization
proof](memoization.md#lemma-2-reading-an-open-growth)). The first growth of a
key is the **head** of the key. A key has at most two outer growths, one in
lookahead state $1$ and one in state $0$ (the outline above).

**Covered growths.** Let $x$ be an evaluation of the parse, a call or a step
of a growth, and let $z$ be a call of a leader inside $\mathcal{R}(x)$ that
reads no growth open at its start. Then the parse makes a growth of the key of
$z$ before $x$ ends, outside cases (a), (b) and (c) of the covered keys.

*Proof.* The induction of the covered keys applies with $z$ a call of a
leader. Where the parse evaluates every call and step on the path, it
evaluates the call of $z$, which a growth of the leader that is open at the
position would answer, so the call grows the leader there and reads no growth
open at its start: a growth of the key. Where the first call $y$ that the parse
does not evaluate is $z$ itself, an entry that answers it was stored by an
earlier growth of the leader at the position, since a call of a leader that
evaluates grows ([Growths of Left
Recursion](../runtime/parsing.md#growths-of-left-recursion)), and the call of
that growth read no growth open at its start ((S)); a result or step that the
parse keeps comes from an earlier growth of the key in the same way. The other
cases of the induction lead, as there, to an earlier evaluation that holds a
call with the key of $z$ that reads no growth open at its start. $\square$

**Heads hold what later growths hold.** Let a growth $g$ of a key hold, in its
family region, a successful activation of a source $\sigma$ at $q$. Then the
head of its key holds a successful activation of $\sigma$ at $q$ in its family
region, unless a result or a step that the parse reuses inside the head was
kept in a smaller lookahead state, or holds the place of the activation inside
a lookahead that it observed, which the open gap stated with (D) of the
memoization proof leaves out.

*Proof.* The calls of $g$ and of the head $h$ read no growth open at their
start and have one key, so $\mathcal{R}(g)$ and $\mathcal{R}(h)$ take the same
steps, a leader having no twin (*Counterparts*). The activation of $g$ is the
counterpart of one in $\mathcal{R}(g)$, which corresponds to one of the same
source at the same position, which succeeds, in $\mathcal{R}(h)$, at a place
that lies, as in $\mathcal{R}(g)$, at $p$ in a step, in a context of growths
or in a call of a rule that the parse does not memoize. Follow the path from
$\mathcal{R}(h)$ down to that place. If the parse evaluates every call and
step on it, $h$ holds the activation there. Otherwise let $y$ be the first
call or step on it that the parse does not evaluate. The parse does not skip
an alternative on the path, which encloses an activation (*Counterparts*), and
does not answer $y$ with the current result of an open growth, since
$\mathcal{R}(y)$ would then make no call. The calls on the path are made at
$p$, where $h$ is in their context, or are calls of rules that the parse does
not memoize, which it never answers (in a Document as *Transient calls of a
Document* in Lemma 3 reads it). So it answers $y$ from a result or a step
computed while at least the growths of the context of $y$ were in progress, or
takes $y$ in a run of such steps ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)), and that result or step is
that of an evaluation $y'$ made earlier inside $h$. By the argument of [Lemma
4 of the memoization
proof](memoization.md#lemma-4-answering-from-a-dependent-result), as in the
covered keys, $\mathcal{R}(y)$ and $\mathcal{R}(y')$ take the same steps
outside the lookaheads that $y'$ observed, so outside the gap above the path
goes on to the same place in $\mathcal{R}(y')$, which lies in the family region
of $h$ as well. By induction on the order of the evaluations, $h$ holds a
successful activation of $\sigma$ at $q$ there. $\square$

The claim maps no activation of $g$ to one of $h$ one to one. Two growths of a
key take the same steps in $\mathcal{R}$, but the parse can reuse a result or
a step in a context of growths in one of them and evaluate it again in the
other, since it takes results that it has not shown equal as different
(*Reuse in a context*): the activations that $g$ then evaluates again are
repeats of activations of its own family at the same position. The count below
therefore charges to the heads only whether a growth holds an activation at
$q$, and counts the repeats inside each family on their own, with no reuse,
as *Reuse in a context* requires of every bound of this lemma.

**Links lead to later growths.** Let the $i$-th link of $\sigma$ at $q$ satisfy
the hypotheses of *Links lead to repeated evaluations*, and let $\alpha_i$ lie
in the family region of a growth $g$ of a key, with $e'_i$ outside the family
of $g$. Then $g$ is not the head of its key, unless case (a) or (b) of the
covered keys arises on the path from $\mathcal{R}(e_i)$ to the call that
corresponds to that of $g$.

*Proof.* $e'_i$ contains $\alpha_i$, which lies inside $g$. An evaluation
inside $g$ outside its family is a memoized evaluation, a growth at another
position or an evaluation inside one, and an activation in it lies in no
family region of $g$; so $e'_i$ is not inside $g$, and it is the call that
grows $g$ or encloses it. As in *Links lead to repeated evaluations*, $\mathcal{R}(e_i)$ and
$\mathcal{R}(e'_i)$ take the same steps up to the variants of their calls, so
the call $z^\circ$ at the place of the call of $g$ in $\mathcal{R}(e_i)$ is a
call of the leader of $g$ with its key, which reads no growth open at its
start by the argument given there, and holds an activation at the place of
$\alpha_i$, which excludes case (c). By *Covered growths*, outside cases (a)
and (b), the parse made a growth of that key before $e_i$ ended, and so before
$g$ started. $\square$

The same argument, with *Heads in the reference parse* for growths of a key in
place of evaluations of a key, shows that a free $\alpha_i$ held by a head of a
key is the counterpart of an activation of $\mathcal{R}$ that has no reference
link, or whose reference link meets case (a) or (b), or whose reference link
$(e^k, e^{k\prime})$ has $e^{k\prime}$ inside the family of the head.

So the successful activations of $\sigma$ at $q$ that growth steps and
evaluations in a context of growths own are counted in two parts. First, the
outer growths whose family regions hold one: by the claim above and since a
key has at most two outer growths, they are at most two for each key whose
head holds one; a head that holds one holds an $\alpha_i$ that is $\alpha_1$,
free or linked through a link inside its family, or that a link of a kind
that *Links to heads* leaves out or a leaf link leads to; and an $\alpha_i$
lies in the family regions of at most as many heads as the cycle has leaders,
since the growths at one position that enclose one another have distinct
leaders (*Calls in a context of growths*). Second, in the family region of one outer
growth, the activations of $\sigma$ at $q$ after the first one there, counted
with no reuse in a context of growths. Two of them are related through a call
that the family evaluates at $p$ or a call of a rule that calls no rule
(*Repeats* and *Repeats inside a growth*): a shared call whose later
evaluation is a memoized evaluation would own the later activation. The
following claims bound the second part and, in *First activations of heads*,
the heads of the first part.

**Repeats inside a family.** Let $g$ be an outer growth at $p$, let the values
of the variables not change in the growths of its family, and let the parse
reuse no result and no step kept in a context of growths. Let $\alpha$ and
$\alpha'$ be successful activations of $\sigma$ at $q$ in the family region of
$g$, $\alpha$ first, neither of them inside a call of a rule that calls no
rule, outside case 5 and the dropped steps, and let $N$ be the innermost
evaluation that contains both.

1. If $N$ is not a growth, $\alpha$ lies in a growth of a leader $R$ at $p$
   that the abandoned unit $w$ of the restarts starts with a call that it
   shares with its continuation, and the continuation calls $R$ at $p$ again
   in the same step of the innermost growth at $p$ that contains $N$, where
   $R$ grows again.
2. If $N$ is a growth at $p$, with $\alpha$ in its step $j$ and $\alpha'$ in a
   later step, then either $\alpha'$ is in the second case of *Repeats inside
   a growth*, or $w$ starts at $p$ and $\alpha$ lies in a growth of a leader
   at $p$ that $w$ starts with a call that it shares with its continuation, in
   step $j$, and reaches $q \ge e_{j+1}$.

*Proof.* A call made in the family region that evaluates a body there is a
call of a rule that the parse does not memoize, a call of a leader that grows
at $p$, or a call in a context of growths at $p$; a call of a rule that the
parse memoizes, made in it at another position or outside a context of
growths, is answered or is a memoized evaluation or a growth at another
position, whose activations lie in no family region of $g$. In the
first case the restarts give $w$, a unit, since no dropped step lies on the
path. If $\alpha$ and $\alpha'$ are direct in one body evaluation, the proof
of *Direct activations* applies, since $N$ lies in that body below every call,
and leaves only case 5. Otherwise *Repeats* places $\alpha$ in the evaluation
inside $w$ of a call $c$ that $w$ and its continuation $\kappa$ share, which
is a call of a rule that calls no rule or that is memoized at the call. $c$
contains $\alpha$, so it is not the former, and it is evaluated in the family
region, so it is made in a context of growths at $p$. A rule of a cycle other
than a leader is not memoized at a left call, and a read of a growing result
evaluates nothing, so $c$ grows a leader $R$ at $p$, and $w$ starts at $p$. $N$ starts at $p$ and
lies in one step of the innermost growth at $p$ that contains it, and so does
the evaluation of $c$ in $\kappa$, which starts after $w$ ends and lies in
$N$; with no reuse in a context, $R$ grows again there. In
the second case, outside the second case of *Repeats inside a growth*, the
first case of that claim gives $w$ and its continuation through the reads of
the seed, to which *Direct activations* and *Repeats* apply as stated there;
the call that they place $\alpha$ in contains $\alpha$, so by the argument
above it grows a leader at $p$, $w$ starts at $p$, and $\alpha$ lies inside
step $j$ at $q \ge e_{j+1}$. $\square$

**Consecutive repeats inside a family.** Under the hypotheses of *Repeats
inside a family*, call $\alpha$ and $\alpha'$ **consecutive** when no successful
activation of $\sigma$ at $q$ in the family region of $g$, outside calls of
rules that call no rule and outside the dropped steps, starts after $\alpha$
and before $\alpha'$. Then the second alternative of item 2 does not hold for
consecutive $\alpha$ and $\alpha'$: if $N$ is a growth, $\alpha'$ is in the
second case of *Repeats inside a growth*.

*Proof.* Suppose that $w$ starts at $p$ with a call $c$ of a leader $R$ that it
shares with its continuation $\kappa$, and that $\alpha$ lies in the growth of
$R$ that $c$ makes in step $j$ of $N$, while $\alpha'$ lies in a later step.
Every path of $\kappa$ that examines the input begins with the shared calls,
and the evaluation of $\kappa$ contains $\alpha'$, so $\kappa$ makes its call
of $c$ after $w$ ends and before $\alpha'$. By case 2 of the ways in which a
unit is committing, that call lies in the same growths of the cycle as the one
in $w$, so in step $j$, and $R$ grows again there from the same growing
results and evaluates what it evaluated the first time. Every evaluation on
the path from the growth of $R$ in $w$ down to $\alpha$ lies in the family
region: it is a growth step at $p$, a call at $p$ in a context of growths or a
call of a rule that the parse does not memoize, and not a call of a rule that
calls no rule. The second growth evaluates each of them again: a call at $p$
in a context is answered only by a growing result, which would hold no
activation, or by a result or a step kept in a context, which the parse does
not reuse here, and a call of a rule that the parse does not memoize is never
answered (in a Document as *Transient calls of a Document* in Lemma 3 reads
it). The calls off that path have the same results in both growths, by
the [memoization
theorem](memoization.md#theorem-memoization-never-changes-the-result) where
the memo answers one of them, and the values of the variables do not change,
so each evaluation on the path takes the same path in both. The second growth
therefore holds a successful activation of $\sigma$ at $q$ at the place of
$\alpha$, in the family region, outside a call of a rule that calls no rule and
outside a dropped step, as $\alpha$ is. It starts after $\alpha$, and before
$\alpha'$, since the growth ends inside step $j$; so $\alpha$ and $\alpha'$ are
not consecutive. $\square$

The claim holds whether $\alpha$ lies in a result of the growth of $R$ or in a
part of one of its steps that the step abandons, and whether the growth of $R$
includes the growing result of the growth $N$ or not: the second growth takes
the same steps, abandoned parts included. It rests on the absence of reuse in
a context: where the parse reuses a result or a step of the first growth in
the second, the activation at the place of $\alpha$ is not evaluated again, and
the next one can lie in a later step, which is why the count is made with no
reuse, as *Reuse in a context* requires.

So the activations of $\sigma$ at $q$ in the family region of $g$, outside
calls of rules that call no rule, case 5 and the dropped steps, in the order
in which they start, form consecutive pairs of two kinds. In the first kind of
*Repeats inside a family*, the pair lies in one step of the innermost growth
at $p$ that contains it, and comes from two growths of one leader that the
step starts at $p$, at most $c_5$ of them for each step (*Calls in a context
of growths*); the growths that they nest have other leaders. Otherwise the
later activation of the pair lies in a later step, in the second case of
*Repeats inside a growth*: in a step that the claims at $p$ do not leave out,
at most a constant after $p$. So, by induction on the depth of the growths at
$p$, which have distinct leaders, one step holds at most a constant of them:
the earlier activation of each pair in the step lies in one of the at most
$c_5$ growths that the step starts at $p$, each of which holds at most a
constant by the induction. This holds in the steps that the claims at $p$
leave out too, whose parts at $p$ can examine the input far from $p$. When
$q$ is more than a constant after $p$, the steps of one growth in the family
that hold such an activation are the first of them, the constant number of
steps that the claims at $p$ leave out and the second step of a growth whose first
result is empty. So the activations of $\sigma$ at $q$ in
the family region of $g$ number at most a constant, apart from a constant for
each step of the family when $q$ is at most a constant after $p$, the
activations inside calls of rules that call no rule, case 5 and the dropped
steps.

**First activations of heads.** Let $h$ be the head of a key whose family
region holds a successful activation of $\sigma$ at $q$ of the sequence
$\alpha_1, \alpha_2, \ldots$, and let $\alpha_j$ be the first one there. Then
$\alpha_j$ is $\alpha_1$; or it lies in a call of a rule that calls no rule;
or it follows a link, or is free with a reference link, with case (a) or (b)
of the covered keys on its path; or it is free and the counterpart of an
activation of $\mathcal{R}$ that a dropped step or a path that enters a growth
again of $\mathcal{R}$ leads to; or a result or a step that the parse reuses
inside $h$ meets the gap of *Heads hold what later growths hold*.

*Proof.* Let $\alpha_j$ be none of these, so $j \ge 2$. If it is linked, the
link is memoized, since $\alpha_j$ lies in no call of a rule that calls no
rule. By *Links lead to later growths*, $e'_j$ lies inside the family of $h$,
so it is a call in a context of growths at the position $p'$ of a growth $G$
of that family: an evaluation of the family that is a memoized evaluation
would own $\alpha_j$. The rule of the link is memoized at the call and on
the cycle of $G$, since a call of a rule on no cycle of the growths at $p'$
is in no context of growths ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)), so it is a leader, a rule
of a cycle other than a leader being memoized only at a call that is no left
call. Then $e_j$, made by the call that $w_j$ shares with its
continuation, lies in the same growths of the cycle (case 2), so inside $G$
at $p'$, in its context and in the family region of $h$, and $e_j$ and
$e'_j$ take the same steps. If it is free, its counterpart in $\mathcal{R}$
has a reference link $(e^k, e^{k\prime})$ with $e^{k\prime}$ inside the family
of $\mathcal{R}(h)$, by the remark after *Links lead to later growths*, and
$e^k$ lies in the family region of $\mathcal{R}(h)$ for the same reasons.
Either way, the place of $\alpha_j$ inside the later evaluation, or of its
counterpart, corresponds to a place inside the earlier one, which is evaluated
first, at $q$, in the family region of $h$ or of $\mathcal{R}(h)$. Follow the
path from $h$ down to that place as in the proof of *Heads hold what later
growths hold*: the parse evaluates every call and step on it, or answers the
first one $y$ that it does not evaluate from an evaluation $y'$ made earlier
inside $h$ in the family region, whose path goes on to the same place outside
the gap. By induction on the order of the evaluations, $h$ holds a successful
activation of $\sigma$ at $q$ in its family region before $\alpha_j$, which
lies outside case 5 and the dropped steps as $\alpha_j$ does, or one that
*Repeats inside a growth* leaves out of the sequence and that follows an
earlier one of the same growth; either way $\alpha_j$ is not the first one
there. $\square$

So for each position $q$, the heads whose family regions hold an activation
of the sequence are at most as many as the cycle has leaders for $\alpha_1$,
since the growths at one position that enclose one another have distinct
leaders, and as many for each activation of the kinds that the claim lists.
Those kinds are the open parts of step 1 of [Open Steps](#open-steps), the
gap stated with (D) of the memoization proof, the activations that a dropped
step or a path that enters a growth again of $\mathcal{R}$ leads to, which
step 1 leaves to count too. With the at most two outer growths of each
key and the constant of *Consecutive repeats inside a family* for each family,
the successful activations of $\sigma$ at $q$ that growth steps and
evaluations in a context of growths own are at most a constant, apart from
those kinds and the parts that that claim leaves.

**Families in a parse that keeps records.** In a parse whose records are
exact, (R1) of the [Setting](#setting), and that resumes no run of a
[Document](../runtime/documents.md), let an iteration **of the kind** be
an iteration of a walk of a site that keeps records, as in *Heads in a
parse that keeps records*. Then *Covered growths*, *Repeats inside a
family* and *Links lead to later growths* hold as stated, the last for an
$\alpha_i$ that is no iteration of the kind and lies inside none, and so do
*Heads hold what later growths hold*, *Consecutive repeats inside a
family* and *First activations of heads* for the activations that are no
iterations of the kind and lie inside none, with consecutive activations
and the first activation of a head taken among those activations.

*Proof.* Call an activation **outside** when it is no iteration of the
kind and lies inside none. The following fact is used throughout.

(P) Let $\alpha$ be an outside activation of the parse, let $X$ be an
evaluation of $\mathcal{R}$ that contains the counterpart of $\alpha$,
and let an evaluation $Y$ of $\mathcal{R}$ reach a place through the
expressions, in their rules or in the twins, through which $X$ reaches
that counterpart: as $Y$ does when it takes the steps of $X$ up to the
variants of its calls, or takes them outside lookaheads that the path does
not enter. Then no iteration of the kind lies on the path from $Y$ down to
that place. An iteration of $\mathcal{R}$ that contains the
counterpart of $\alpha$ is the counterpart of an iteration that the parse
evaluates, of a walk of the same site, which contains $\alpha$ (the
paragraph before 1 in the proof of *Heads in a parse that keeps
records*), so none is of the kind. $Y$ reaches that place
through the same expressions, so the iterations on its path are
iterations of the same repetitions, in a rule or in its twin, and so of
the same sites, as in 1 of that proof, which keep no records.

*Covered growths.* By 1 of *Heads in a parse that keeps records*, case
(d) arises on the path of the covered keys only when an iteration of the
kind contains $z$ inside $\mathcal{R}(x)$. Here $z$ is a call of a leader,
which lies in no iteration of a walk of a candidate site or of a skip-run
site, in the parse or in $\mathcal{R}$ (*Growths lie outside the recorded
walks*), so the case does not arise, and the proof holds as stated.

*Repeats inside a family.* Its proof uses the restarts, *Direct
activations* and *Repeats*, which hold of every activation of such a parse
(*Restarts in a parse that keeps records*), the first case of *Repeats
inside a growth*, which *Repeats inside a growth in a parse that keeps
records* shows, and which calls the family region evaluates and which the
memo answers. A walk that takes a record answers iterations and no call,
so the last is as without records.

*Links lead to later growths.* Its proof uses that $\mathcal{R}(e_i)$ and
$\mathcal{R}(e'_i)$ take the same steps, which *Links lead to repeated
evaluations* gives with case (d) only when $Z$ lies inside $e'_i$ in an
iteration of the kind (3 of *Heads in a parse that keeps records*), and
$Z$ holds $\alpha_i$, which is outside; and it applies *Covered growths*
to a call of a leader, where case (d) does not arise.

*Heads hold what later growths hold.* The proof follows the path from
$\mathcal{R}(h)$ down to the place that corresponds to the counterpart of
the activation of $g$, and, at each call or step $y$ that the parse does
not evaluate, goes on from $\mathcal{R}(y')$, which takes the steps of
$\mathcal{R}(y)$ outside the lookaheads that $y'$ observed. Each segment of that path, from $\mathcal{R}(h)$ or from an
$\mathcal{R}(y')$, reaches the place that corresponds to the counterpart
of the activation of $g$ through the expressions through which the
matching evaluation inside $\mathcal{R}(g)$ reaches that counterpart, and
each segment from an $\mathcal{R}(y')$ lies outside the lookaheads that
$y'$ observed, so by (P), applied to
each segment in turn, no iteration of the kind lies on any segment of that
path, so no record answers an
evaluation on it, and the parse evaluates every call and step on it that
it neither answers nor takes in a run of steps. The rest of the proof
uses of the parse which calls and steps it evaluates and from what it
answers the others, the memo and [Lemma 4 of the memoization
proof](memoization.md#lemma-4-answering-from-a-dependent-result), which
the records do not change. The activation that $h$ holds at the end of the
path is outside, by (P).

*Consecutive repeats inside a family.* Let $\alpha$ and $\alpha'$ be
outside. Every evaluation on the path from the growth of $R$ in $w$ down to
$\alpha$ contains $\alpha$, so none is an iteration of the kind; the second
growth of $R$ evaluates the evaluations at the same places, of the same
expressions, so none of those is one either, and no record answers them.
The calls off that path have the same results in both growths, and a walk
off it has the success and the end of its counterpart in $\mathcal{R}$
whether or not it takes a record (2 of *Restarts in a parse that keeps
records*), so each evaluation on the path takes the same path in both. The
second growth therefore holds, at the place of $\alpha$, an activation
that is outside, and the proof goes on as stated.

*First activations of heads.* Let $\alpha_j$ be outside. If it is linked,
*Links lead to later growths* applies as shown, and $e_j$ and $e'_j$ are
calls of a leader in contexts of growths. If it is free, the remark after
*Links lead to later growths* rests on *Heads in the reference parse* for
growths of a key, with case (d) only when a call of a leader lies in an
iteration of the kind in $\mathcal{R}$, which it does not, and on
*Reference links lead to repeated evaluations*, with case (d) only when
$Z$ lies in an iteration of the kind or $e^k$ does in $\mathcal{R}$; $Z$
holds $\alpha_j$, and $e^k$ is, as in the proof, a call of a leader in a
context of growths, which lies in no iteration. The path from $h$ to the
place inside the earlier evaluation is then followed as for *Heads hold
what later growths hold*, and the activation that $h$ holds there is
outside. $\square$

The claim asks that the parse resume no run. A walk that a Document
resumes evaluates no element in the iterations that it reuses, and (P)
says nothing of it, since it is no walk of a site that keeps records. A
repetition resumes a record only outside a lookahead and a `skip`
([Documents](../runtime/documents.md)), so a head in lookahead state $0$
can resume a run whose walk a later growth of its key in lookahead state
$1$ evaluates; that growth then holds activations inside the iterations
to which nothing in the head corresponds, and *Heads hold what later
growths hold* is not shown. And the first growth of *Consecutive repeats
inside a family* can evaluate a walk and record its run, and the second
resume the run whole from that record, so that it holds nothing at the
place of $\alpha$. *Families in a parse that resumes runs* below leaves
these activations out, as this claim leaves out those of the iterations
of the kind.

So the count above holds of the successful activations of $\sigma$ at $q$
that growth steps and evaluations in a context of growths own and that are
outside, in a parse that resumes no run, up to the bound of *Consecutive repeats inside a family* in a step
that the claims at $p$ do not leave out, which rests on the claims of this
lemma before *Calls in a context of growths*, not read again for a parse
that keeps records. It does not bound those inside the iterations of the
kind as it stands. The second growth of *Consecutive repeats inside a
family* can take a record where the first evaluated, so it holds nothing at
the place of $\alpha$ and the next activation there can lie in a later
step; and a head can take a record where a later growth of its key
evaluates, so it need not hold what that growth holds. The next claim
bounds them, and the outside ones with them, by the same count in a parse
that keeps no records, when the parse resumes no run of a Document.

**Families in a parse that resumes runs.** In a
[Document](../runtime/documents.md) parse whose records of sites, if it
keeps any, are exact, (R1) of the [Setting](#setting), that resumes runs,
call an activation **outside** when it is no iteration of the kind and no
resumable iteration (*Heads in a parse that resumes runs*) and lies
inside none. Then *Repeats inside a family* holds as stated; *Covered
growths* holds with the path followed through the records and the
resumed runs, as in 2 of *Covered keys through resumed runs*;
*Links lead to later growths* holds for an $\alpha_i$ that is outside,
with that path from $\mathcal{R}(e_i)$; the remark after it holds with
the paths of reference links followed so; and *Heads hold what later
growths hold*, *Consecutive repeats inside a family* and *First
activations of heads* hold for the activations that are outside, with
consecutive activations and the first activation of a head taken among
those activations, except, for *First activations of heads*, a free
$\alpha_j$ when the path in $\mathcal{R}$ from $\mathcal{R}(h)$, for the
head $h$ of the claim, whose family region holds $\alpha_j$, down to the earlier
evaluation $e^k$ of its reference link passes an iteration that a resumed walk of the parse reuses. An iteration on such
a path that comes from an earlier parse is case (a), as everywhere.

*Proof.* The proof of *Families in a parse that keeps records* applies
with the following changes. (P) holds with outside activations and with
no iteration of the kind and no resumable iteration on the path from $Y$
down to the place: an iteration of $\mathcal{R}$ that contains the
counterpart of $\alpha$ is the counterpart of an iteration that the parse
evaluates, of a walk of the same repetition, in its rule or in the twin,
which contains $\alpha$ (the paragraph on the counterparts in the proof of
*Heads in a parse that resumes runs*), so it is not resumable, the class
being one of repetitions; and the iterations on the path of $Y$, which
reaches the place through the same expressions, are iterations of the
same repetitions, so none is resumable either, and no resumed walk reuses
an evaluation on that path.

*Covered growths.* A call of a leader can lie in a resumable iteration,
since the element of a repetition that a Document resumes can call a rule
of a cycle. At an iteration $\iota$ on the path that a resumed walk reuses
and that comes from no earlier parse, 1 of *Covered keys through resumed
runs* gives the iteration $\iota_0$, evaluated before the walk started,
and the call at the place of $z$ in $\mathcal{R}(\iota_0)$, a call of the
leader with the key of $z$, a leader having no twin, which reads no growth
open at its start; the induction of *Covered growths* goes on from it,
with $x$ taken to be an iteration that the parse evaluates, of a walk of
any repetition, as well as a call or a step of a growth, as 2 of that
claim takes it for the covered keys. Case (d) does not arise, as
there.

*Repeats inside a family.* Its proof uses the restarts, *Direct
activations* and *Repeats*, which hold of every activation of such a
parse, a reused iteration included (*Restarts in a parse that resumes
runs*), the first case of *Repeats inside a growth*, which *Repeats
inside a growth in a parse that resumes runs* shows, and which calls the
family region evaluates and which the memo answers. A resumed walk reuses
iterations, makes no call inside them and answers none, so the last is as
without resumption.

*Links lead to later growths.* $\mathcal{R}(e_i)$ and $\mathcal{R}(e'_i)$
take the same steps by *Links lead to repeated evaluations*, with case (e)
only when $Z$ lies inside $e'_i$ in a resumable iteration (*Heads in a
parse that resumes runs*), and $Z$ holds $\alpha_i$, which is outside; and
*Covered growths* applies to the call of a leader as above.

*Heads hold what later growths hold.* By (P), applied to each segment of
the path as there, no resumable iteration lies on that path, so no
resumed walk reuses an evaluation on it, and the rest is as there.

*Consecutive repeats inside a family.* Let $\alpha$ and $\alpha'$ be
outside. Every evaluation on the path from the growth of $R$ in $w$ down
to $\alpha$ contains $\alpha$, so none is a resumable iteration; the
second growth of $R$ evaluates the evaluations at the same places, of the
same expressions, so none of those is resumable either, and no resumed
walk reuses them. A walk off that path has the success and the end of its
counterpart in $\mathcal{R}$ whether or not it resumes a run (2 of
*Restarts in a parse that resumes runs*), and the proof goes on as there.

*First activations of heads.* For a linked $\alpha_j$ that is outside,
*Links lead to later growths* applies as above. For a free one, the
remark after that claim rests on *Heads in the reference parse* for
growths of a key and on *Reference links lead to repeated evaluations*;
followed through the records and the resumed runs, as 3 of *Covered keys
through resumed runs* follows them, with *Covered growths* above for the
growths of a key, they give the remark outside cases (a) and (b) on those
paths. The path from $h$ to the place inside the earlier evaluation has
two segments. The segment from $\mathcal{R}(h)$ down to $e^k$, a call of a
leader in a context of growths reached through the unit of the link and
not through the expressions that reach the place, passes no iteration
that a resumed walk reuses, by the exception of the claim; (P) does not
show this, and unlike a call in an iteration of the kind, which
*Growths lie outside the recorded walks* excludes, a call of a leader can
lie in a resumable iteration. The segment from $e^k$ down to the place
reaches it through the same expressions as the path from $e^{k'}$ to the
counterpart of $\alpha_j$, so (P) shows that it passes no resumable
iteration. The path is then followed as for *Heads hold what later
growths hold*, and the activation that $h$ holds there is outside.
$\square$

The two cases that the remark after *Families in a parse that keeps
records* gives are of activations that are not outside: when a head
resumes a run whose walk a later growth of its key evaluates, the
iterations of that walk are resumable, and when the second growth of
*Consecutive repeats inside a family* resumes the run that the first
recorded, the iteration of the first that contains $\alpha$ is resumable.
The exception of the claim is the case in which a walk inside $h$
resumes a run whose iteration holds, in $\mathcal{R}$, the call $e^k$ of
a leader in a context of growths; $h$ then evaluates nothing at the
place. A run that holds a growth is recorded only when no call in it read
a growing result ([Documents](../runtime/documents.md)), which a growth
whose steps succeed before they reach a left call meets, so the proof
does not exclude the case. So the count of
Lemma 5 holds of the outside activations of such a parse, apart from
that exception, up to the bound of *Consecutive repeats inside a family* in a step that
the claims at $p$ do not leave out, as with records. *Families without
the records* does not apply to such a parse as stated: the element of a
repetition that a Document resumes can call rules of a cycle, so growth
steps and evaluations in a context of growths can lie inside a resumable
iteration, which its first step leaves out only for the sites that keep
records; and a resumed walk stores no entry inside the iterations that
it reuses, so a later evaluation can evaluate calls that the parse
$\mathcal{P}$ of that claim answers. *Families without the records in a
parse that resumes runs* below bounds them, the exception above
included, with a constant factor.

**Families without the records.** Call a rule **cycle-free** when no rule
that it reaches, itself included, is a rule of a left-recursion cycle. Let
the parse, whose records are exact, (R1) of the [Setting](#setting),
reuse no result and no step kept in a context of growths, resume no run
of a [Document](../runtime/documents.md) and stop at no limit, and let
$\mathcal{P}$ be the parse of the same grammar and input, of the same
kind, with the same entries from earlier parses, that memoizes the rules
that the parse memoizes, keeps no record of any site, resumes no run,
reuses nothing kept in a context of growths, stops at no limit, and
stores the result of an evaluation of a rule that is not cycle-free
exactly where the parse stores it. A Document also stores results of
the transient rules, which Lemma 3 does not take as memoized; the claim
reads both parses as *Transient calls of a Document* in Lemma 3 does,
evaluating every call of such a rule. Then $\mathcal{P}$ is a parse of the
[Setting](#setting), and every successful activation of a source $\sigma$
at $q$ that a growth step or an evaluation in a context of growths owns in
the parse has the counterpart of an activation of $\sigma$ at $q$ that
succeeds in $\mathcal{P}$, owned there by the evaluation whose counterpart
is that of the owner in the parse, in the family region of the outer
growth whose counterpart is that of the one in the parse; distinct
activations have distinct such activations.

*Proof.* The rules that the element of a site that keeps records calls,
directly or indirectly, are cycle-free: every rule that such a rule
reaches is called by the element indirectly, and so is in no
left-recursion cycle (*Growths lie outside the recorded walks*, whose
proof shows this of every rule that the element calls, directly or
indirectly). A cycle-free rule calls only cycle-free rules. So no call of
a rule that is not cycle-free lies inside an iteration of the kind or
inside the evaluation of a call of a cycle-free rule, at any depth; nor
does a growth step, an evaluation in a context of growths or a growth,
each of which is or is made by a call of a rule of a cycle.

Call an evaluation $Y$ of $\mathcal{R}$ **framed** when no evaluation
that contains it, itself included, is an iteration of a walk of a site
that keeps records in the parse, and none that strictly contains it is
the body evaluation of a call of a cycle-free rule that the parse
memoizes. A framed $Y$ is **shared** when it is not
itself the body evaluation of such a call. The first step is that the
parse makes an evaluation whose counterpart is a shared $Y$ exactly when
$\mathcal{P}$ makes one, and, when $Y$ is a call of a rule that is not
cycle-free or that the parse does not memoize, the parse evaluates it,
and does not answer it, exactly when $\mathcal{P}$ does. Each parse makes
its evaluations in the order in which their counterparts start, since it
evaluates what $\mathcal{R}$ evaluates except inside what it answers,
skips or does not take (*Counterparts in the reference parse*); the proof
is by induction on that order of $\mathcal{R}$. A parse makes an
evaluation whose counterpart is $Y$ when it makes one whose counterpart is
the evaluation $Y_0$ that immediately contains $Y$, evaluates it if it is
a call, does not skip the alternative that holds $Y$ by the next
character and, when $Y$ is a step of a growth, takes that step; and the
outcomes of the evaluations of both parses are those of their counterparts
up to what reaches no decision of the parser (*Counterparts in the
reference parse*). If $Y$ is framed, so is $Y_0$, since every evaluation
that contains $Y_0$ strictly contains $Y$. $Y_0$ is shared: were it the body
evaluation of a call of a cycle-free rule that the parse memoizes, $Y$
would lie strictly inside it. If $Y_0$ is a call, it is not of a
cycle-free rule that the parse memoizes, since its body evaluation $Y$
would then not be shared. So the induction hypothesis gives $Y_0$ to both
parses or to neither, and evaluated by both or by neither when it is a
call. The skip depends on the input alone. Whether a step of a growth is
taken depends on the results of the earlier steps, which are those of
their counterparts in both. With no reuse of a step kept in a context of
growths, each parse evaluates every step that it takes and takes no run
of steps at once, since a run is taken from kept steps (*Covered keys*).
With no resumed run, a walk evaluates every iteration that no record
answers, and $\mathcal{P}$ keeps no record. Whether a call is evaluated depends, for a
rule that the parse does not memoize, on nothing, since the memo never
answers it in that reading; for a rule that is not cycle-free and that the parse
memoizes, on the growths open at its position, which started at calls of
rules of a cycle, and on the entries that the memo holds for its key in
its lookahead state or in a smaller one, which can answer it by (L) of
the [memoization proof](memoization.md#setting), and which were stored by evaluations of calls of that
rule or of its twin, or of growths of it, whose counterparts start before
$Y$, or come from earlier parses. Those calls and growths are framed and
shared by the paragraph above, so by the induction hypothesis both parses
made the same of them before, with the outcomes of the same counterparts, and stored the same entries,
$\mathcal{P}$ storing where the parse stores; with no reuse in a context
of growths, nothing else decides the call.

$\mathcal{P}$ is a parse of the [Setting](#setting): its records are
exact, since it keeps none, and its memo meets the requirement of the
Setting. For a rule that is not cycle-free, the evaluations of its calls
and of its twin's calls are framed, so by the first step they are made in
the same order in both parses, and $\mathcal{P}$ stores the result of
every one of them that the parse stores, which is each one outside every
context of growths except possibly the first of the rule and the first of
its twin. For a cycle-free rule $\mathcal{P}$ follows the requirement of
the Setting as it stands.

Now let $G$ be a growth step or an evaluation in a context of growths of
the parse that owns a successful activation $\alpha$ of $\sigma$ at $q$.
$G$ is the body evaluation of a call of a rule of a cycle, so
$\mathcal{R}(G)$ is framed and shared, and by the first step
$\mathcal{P}$ makes an evaluation $G'$ whose counterpart is
$\mathcal{R}(G)$, a growth step or an evaluation in a context of growths
as $G$ is, since that depends on the rule, the call and the growths open
at its position. A call that the parse evaluates strictly inside $G$ and
around $\alpha$ is of a rule that the parse does not memoize: the body
evaluation of a call of a rule that the parse memoizes is a memoized
evaluation, a growth step or an evaluation in a context of growths
(*Owners* in Lemma 4), and would lie strictly inside $G$ and
contain $\alpha$, while $G$ is the innermost of those. Follow the path
from $\mathcal{R}(G)$ down to the counterpart of $\alpha$; every
evaluation on it is the counterpart of one of the parse inside $G$ that
contains $\alpha$. Before the first iteration of the kind on the path, if
there is one, every evaluation on it is framed and shared, since no call
on it is of a rule that the parse memoizes, so $\mathcal{P}$ makes the
counterpart of each by the first step, and evaluates each call among
them. That iteration $\iota$ is one of a
walk $W$ on that part, whose counterpart $\mathcal{P}$ evaluates with no
record: it evaluates every iteration of the walk up to the end, at the
positions where $\mathcal{R}(W)$ evaluates them, since each iteration has
the outcome and the end of its counterpart, so it evaluates one whose
counterpart is that of $\iota$. Below $\iota$ the evaluations on the path
are evaluations of expressions, iterations of walks, which $\mathcal{P}$
evaluates in full, and calls of rules that the parse does not memoize,
which $\mathcal{P}$ evaluates; no alternative on the path is skipped by
the next character, since the parse does not skip it; and no step of a
growth lies there (*Growths lie outside the recorded walks*). So
$\mathcal{P}$ evaluates an activation $\alpha'$ whose counterpart is that
of $\alpha$: an activation of $\sigma$ at $q$ that succeeds, as its
counterpart does. Between $G'$ and $\alpha'$ the calls are those of the
path, of rules that the parse does not memoize, and none of their body
evaluations is in a context of growths: whether one is depends only on
its rule and the growths open at its position, which are those of its
counterpart in both parses, and in the parse none is, since $G$ is the
innermost owner of $\alpha$. So $G'$ owns $\alpha'$.
Let $g$ be the outer growth whose family region holds $\alpha$. The path
from $\mathcal{R}(g)$ down to $\mathcal{R}(G)$ holds the counterparts of
no memoized evaluation and of no growth at another position, by the
definition of the family region, and of no iteration of the kind, which
would contain $G$; so it is framed and shared, $\mathcal{P}$ makes the
counterpart of each evaluation on it, of the same kind, and $\alpha'$
lies in the family region of the growth $g'$ of $\mathcal{P}$ whose
counterpart is $\mathcal{R}(g)$, an outer growth as $g$ is. Distinct activations of the parse have
distinct counterparts, and so do their images. $\square$

In $\mathcal{P}$ no iteration is of the kind, so every activation that a
growth step or an evaluation in a context of growths owns there is
outside, and the count above, the families, *Consecutive repeats inside a
family* and *First activations of heads* with the claims at $p$ that they
rest on, holds of $\mathcal{P}$ as of a parse without records. By the
claim, the successful activations of $\sigma$ at $q$ that growth steps and
evaluations in a context of growths own in the parse, inside the
iterations of the kind or outside, are at most those of $\mathcal{P}$,
with no reuse in a context of growths in either, as *Reuse in a context*
requires of every bound of this lemma. So that count bounds them, apart
from the kinds and the parts that it leaves, read in $\mathcal{P}$: the
claim moves those open parts to a parse without records and adds none.
The bound gives the parse no credit for its records: a walk that a growth
step makes is counted in $\mathcal{P}$ with all its iterations, whatever
records the parse could take there.

The claim leaves out a parse that resumes a run of a Document. Such a
parse can resume a run whole from a record that it stored earlier
([Documents](../runtime/documents.md)), so whether it resumes a walk
depends on the walks that it evaluated before. A parse without records of
sites evaluates the walks inside the iterations that the records of the
parse answer, so if it resumes runs it can resume a walk that the parse
evaluates; if it resumes none, it evaluates elements that the parse
never evaluates, and its stores of the calls made there have no
counterpart in the parse. So the one-to-one map of the claim does not
carry over: a resumed walk stores no entry inside the iterations that it
reuses, and a later call of the parse can evaluate what $\mathcal{P}$
answers from an entry stored there. The next claim maps such a parse to
$\mathcal{P}$ with a constant number of activations of the parse to
each activation of $\mathcal{P}$.

**Families without the records in a parse that resumes runs.** Let the
parse be as in *Families without the records*, except that it is a
[Document](../runtime/documents.md) parse that may resume runs, whose
records of runs are stored and resumed as (N1) and (N2) of *Covered keys
through resumed runs* read them, and let $\mathcal{P}$ be as there, a
parse that resumes no run, except that it stores as follows in place of
storing where the parse stores. Let both parses store the result of an
evaluation of a rule that is not cycle-free that read no growing result
wherever the memo can answer a later call of its key in the lookahead
state and with the growths open where it was made, except possibly the
first evaluation of its variant at its position outside every context of
growths, each store keeping the join of the entry and the new result that
[Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)
describes, and let whether the memo can answer a call there depend on the
same things in both, as in the engine. Then $\mathcal{P}$ is a parse of the
[Setting](#setting), and every successful activation $\alpha$ of a source
$\sigma$ at $q$ that a growth step or an evaluation in a context of
growths owns in the parse has an **image**: a successful activation of
$\sigma$ at $q$ in $\mathcal{P}$ that a growth step or an evaluation in a
context of growths owns there. Each activation of $\mathcal{P}$ is the
image of at most a constant number of activations of the parse.

*Proof.* $\mathcal{P}$ is a parse of the Setting as in *Families without
the records*: it keeps no record and resumes no run, and stores where the
Setting requires.

*Images of owners.* Let $G$ be a growth step or an evaluation in a
context of growths of the parse. Follow the path in $\mathcal{R}$ from
the evaluation where $\mathcal{R}$ begins down to $\mathcal{R}(G)$.
$\mathcal{P}$ evaluates every iteration on it, keeping no record and
resuming no run, skips no alternative on it, since the skip depends on
the input alone and the parse skips none, and takes the steps of growths
that the parse takes, by their results, which are those of their
counterparts (*Counterparts in the reference parse*). If $\mathcal{P}$
evaluates every call and step on the path, the evaluation of
$\mathcal{P}$ whose counterpart is $\mathcal{R}(G)$ is the **image**
$G'$ of $G$. Otherwise let $y$ be the first call on it that
$\mathcal{P}$ answers; the parse evaluates it, since it makes $G$. The
answer is not the current result of an open growth, since
$\mathcal{R}(y)$ would then make no call, and with no reuse in a context
of growths $\mathcal{P}$ answers $y$ from an entry of the memo, whose result read no
growing result and observed no lookahead that read one ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)). Up to $y$ both
parses evaluate the same path, so $y$ is in the same lookahead state and
the same context of growths in both. The entry of $\mathcal{P}$ under the
key of $y$ is the join of the entry of an earlier parse under the key, if
the parse can use one, and of the results that $\mathcal{P}$ has stored
under the key. The parse holds the same entry of the earlier parse, and its
own stores keep joins, which never narrow (*Answered calls* in Lemma 4, a
store in a context of growths included), so its entry answers every call
that the earlier entry answers. Were the entry of $\mathcal{P}$ the earlier
entry alone, the parse would therefore answer $y$, which it evaluates, since
it makes $G$. So $\mathcal{P}$ has stored under the key the result of an
evaluation $y_1$ of a call with the key of $y$, or of a growth of it, whose
counterpart starts before that of $y$. Neither $y$ nor $y_1$ reads a growth
open at its start, and two such evaluations of one key take the same steps
(the proof of [Lemma 3 of the memoization
proof](memoization.md#lemma-3-answering-from-a-sound-entry)), so, as in
*Heads hold what later growths hold*, $\mathcal{R}(y)$ and
$\mathcal{R}(y_1)$ take the same steps. The path goes on from
$\mathcal{R}(y_1)$ to the place of $\mathcal{R}(G)$, and by induction on
the order of $\mathcal{R}$, in which each such $y_1$ starts earlier, it
reaches an evaluation $G'$ of $\mathcal{P}$, the image of $G$, such that
$\mathcal{R}(G)$ and $\mathcal{R}(G')$ take the same steps.

$G'$ is a growth step or an evaluation in a context of growths, as $G$
is. A step of a growth of a leader at its position is matched by a step
of the same rank of a growth of the same leader there, since a call of a
leader that is evaluated grows. If $G$ is in a context of growths, a
growth of its cycle is open at its position, and $G$ lies in a step of
it. If the path reaches that step's place through no answered call, $G'$
lies at the same position in a step of the image of that growth. If an
answered call $y$ lies between that step and $G$, $G'$ lies inside the
evaluation $y_1$ above, whose entry answers a call in a context with
that growth and so holds its leader in the set of leaders that grew where
$y_1$ was made: a growth of the same leader was in progress at that
position around $y_1$. If every answered call on the path lies above that
step, the last of them, $y$, contains the growth, and $\mathcal{R}(y_1)$
takes the steps of $\mathcal{R}(y)$, so $\mathcal{P}$ evaluates inside
$y_1$ the counterpart of that growth, or reaches it by a further jump,
and by the induction $G'$ lies in a step of a growth of the same leader at
that position. Either way $G'$ is in a context of growths too.

*Images of activations.* Let $G$ own $\alpha$. The calls on the path from
$G$ down to $\alpha$ are of rules that the parse does not memoize, since
$G$ is the innermost owner (*Owners* in Lemma 4), so $\mathcal{P}$, which
memoizes the same rules, evaluates them (in a Document as *Transient calls of
a Document* in Lemma 3 reads it); it evaluates every iteration on
it and skips no alternative on it; and no step of a growth lies there,
whose call, of a leader, would be memoized. So $\mathcal{P}$ evaluates,
at the place of $\alpha$ in $G'$, an activation $\alpha'$ of $\sigma$ at
$q$ that succeeds, as $\alpha$ does, the steps being the same: the
**image** of $\alpha$. It is owned by $G'$ or by an evaluation between
$G'$ and $\alpha'$ in a context of growths, a call of a rule that
$\mathcal{P}$ does not memoize, which owns it only as a growth step or
an evaluation in a context of growths. The images of the owners whose
activations have the image $\alpha'$ contain $\alpha'$, so they lie on
one path down to it; let $G'_2$ be the outermost and $G'_1$ another. On
the path from $G'_2$ down to $\alpha'$ no step of a growth lies and every
call is of a rule that is not memoized, so $G'_1$ is an evaluation in a
context of growths of such a rule. A context of growths at a position is
one of a growth of the rule's own cycle there (the parser enters a
context only for a call of a rule of a cycle that grows at the call's
position), and $G'_1$ lies in a step of that growth, which is not
strictly between them, so it is $G'_2$ or contains it, and $G'_2$ and $G'_1$ start at the same position and the
calls from $G'_2$ down to $G'_1$ are left calls of rules of the cycle
other than leaders, a call of a leader being memoized. Such a chain has
fewer than $|\text{rules}|$ calls (*Calls in a context of growths*), so
at most $|\text{rules}|$ distinct images contain $\alpha'$ at the places
of activations that their owners own, and the activations of the parse
with image $\alpha'$ are owned by owners with one of those images, one
activation at the place of $\alpha'$ in each.

*Owners with one image.* It remains to bound by a constant the owners
$G$ of the parse with one image $G'$; they have the key of $G'$ and, for
a step, its rank. A call $y$ above, which the parse evaluates and
$\mathcal{P}$ answers, read no growing result, since $\mathcal{R}(y)$
takes the steps of $\mathcal{R}(y_1)$, whose result read none; so it
contains no evaluation that reads the growing result of a growth open at
its start. Let $G$ be the body evaluation of a call $x$ or a step of a
growth of $x$.

(i) Let the result of $x$ read no growing result, and let the memo be
able to answer a call of its key where $x$ is made. The parse evaluates
$x$ only when the entry of its key does not answer it, and then stores its
result, unless $x$ is the first evaluation of its variant at its position,
in the lookahead state of $x$ and with the leaders that grow in its
context. The store keeps the join, which answers the call of $x$, which
the entry did not, and every call that the entry answered. An entry
answers the calls in the states at least its own and with sets of growing
leaders contained in the set it records, so the entries that the key has
in turn answer strictly growing sets of calls, ordered as the pairs of a
lookahead state and a set of leaders of the cycle of its rule. A chain of
such pairs has at most two more members than the cycle has leaders, so the
parse makes at most three more such evaluations of the key than the cycle
has leaders, a constant of the grammar, and a growth of each has one step
of each rank: the owners of this kind with one image are at most that
constant.

(ii) Let the result of $x$ read the growing result of a growth $h$ open
at the start of $x$, the innermost such growth, so that $x$, and with it
$G$, lies in a step $H$ of $h$. For a step $G$ of a growth of $x$ the
growth of which it is a step is not taken as $h$: $h$ is open at the start
of the call $x$, and $G$ falls under (ii) when some step of the growth of
$x$ reads $h$, whether or not $G$ does. No call between $H$ and $x$ is a
$y$ above, since it would contain $x$, whose result reads the growing
result of $h$, open at its start, and no call lies between $x$ and $G$.
So the path to $G'$ passes the image $H'$ of $H$, the image of $x$ lies at
the place of $x$ in $H'$, and $G'$ is its body evaluation or the step of
the same rank of its growth; an owner with image $G'$ lies at that place,
which a step holds once, in a step with image $H'$: the owners of this
kind with image $G'$ are at most the steps with image $H'$.

(iii) Otherwise the result of $x$ reads no growing result and the memo
answers no call of its key where $x$ is made, which is then in a context
of a growth $h$ open at its position $q$, the innermost, and lies in a
step $H$ of $h$. If no $y$ lies between $H$ and $G$, $G'$ lies at the
place of $G$ in the image $H'$ of $H$, and the owners so are at most the
steps with image $H'$, as in (ii). Otherwise let $y$ be the last such
call on the path, which (i) applies to: the parse makes at most a
constant number of evaluations of the key of $y$ that $\mathcal{P}$
answers, and below each of them one owner at the place of $G$. Its
$y_1$ is a call at $q$ that contains $G'$ in $\mathcal{P}$. The calls
$y_1$ at $q$ that contain one another form a chain
of left calls at $q$, and a rule repeats on it only past a leader whose
left call there grows it: a left call of a leader that grows at $q$
already reads its growing result and evaluates no body. In $\mathcal{P}$
such a growth can start inside a $y_1$ without making $y_1$ read it,
since a result records only the growths open at its start, but the
growths open at $q$ in one context are of distinct leaders (*Calls in a
context of growths*). So the chain passes at most as many growths as the
cycle has leaders, and between two of them its calls are of distinct
rules, a cycle of left calls passing through a leader: it has at most
$|\text{rules}|$ times one more than the number of leaders calls, and the
owners so are at most that number times the constant of (i).

In (ii) and (iii) the steps with image $H'$ are steps of growths at $q$
of one leader, whose calls fall under (i), (ii) or (iii) in turn, and
the growths open at $q$ in one context are of distinct leaders of one
cycle, so (i) ends the descent after a constant number of levels, and
the owners with one image are at most a constant.

So each activation of $\mathcal{P}$ is the image of at most a constant
number of activations of the parse. $\square$

By the claim, the successful activations of $\sigma$ at $q$ that growth
steps and evaluations in a context of growths own in such a parse,
inside the iterations of the kind, inside resumable iterations or
outside, are at most a constant times those of $\mathcal{P}$, with no
reuse in a context of growths in either, and the count of Lemma 5 read
in $\mathcal{P}$ bounds them as for a parse that resumes no run, apart
from the kinds and the parts that it leaves. The constant is built from
those of (i) to (iii) and the factor $|\text{rules}|$ of the images that
contain one activation, a function of the grammar alone. The claim compares the
two parses; it does not show that the parse makes fewer evaluations than
$\mathcal{P}$, though a resumed walk evaluates nothing in the iterations
that it reuses: a call of the parse can evaluate what $\mathcal{P}$
answers from an entry stored inside them, at most the constant of (i)
times for each key, as an iteration that comes from an earlier parse
whose entries lag behind by more than $T$ edits shows.

The growth steps themselves remain. By *Calls in a context of growths* and
*Growths inside a step*, the steps of the growths that are not outer are
charged to the outer growths whose families hold them, to the input that those
consume and to scanning sources, so what is left is the steps of the outer
growths. Each starts at a call of a rule of the cycle that is not a left call
or a call of a leader in the body of another rule, or through the left calls
of the cycle from such a call, which is the scanning source that [Conditions
for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost) names at
the call, and an outer
growth can start at each position, as the growths of `R` at each `a` in its
example do. The following claim counts them as the activations of that source.

**Growths at many positions.** Let $g$ be an outer growth at $p$ whose
first step succeeds, and let $e_1 < e_2 < \dots < e_m$ be the ends of
its results that become its current result, in order. For $1 \le i < m$, the
step of $g$ whose seed ends at $e_i$ and whose result ends at $e_{i+1}$ is a
**growth activation** at $e_i$ of the scanning source at the call that
starts $g$; it
**succeeds**, since it consumes $e_{i+1} - e_i > 0$ past its seed. Then:

1. $g$ takes at most two steps that are not growth activations, and the
   outer growths number at most two for each key, so the steps of the outer
   growths number at most a constant times $K\hat{n}$ plus the number of
   successful growth activations.
2. Let $\alpha$ and $\alpha'$ be successful growth activations of one source
   at $q$, of outer growths $g$ at $p$ and $g'$ at $p'$, with $\alpha'$ inside
   $\alpha$. Then $p < p' = q$ and $g'$ is the outer growth of its key at $q$,
   one of at most two; or $p < p' < q$ and $g'$ lies in a part of $\alpha$
   on a path that passes through no read of the growing result of $g$ that
   succeeds and no call whose result includes one. When that path reads no
   growing result at $p$ at all, the conclusion of the second case of
   *Repeats inside a growth* holds for the step $\alpha$.
3. In the second case of part 2, let the path read a growing result at $p$,
   and let the values of the variables not change in $g$. Then the steps of
   $g$ that start an outer growth on such a path number at most a constant.
   So the successful growth activations of a source at a position that lie
   inside the one before them, in the order in which the successful growth
   activations of the source at that position start, number at most a
   constant times $K\hat{n}$, apart from those for which the conclusion of
   the second case of *Repeats inside a growth* holds for that one.

*Proof.* 1. A growth evaluates the body again while the result gets longer
([Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)): its
first step gives the result that ends at $e_1$, each later step that gives a
longer result is a growth activation, and the first step that does not ends
the growth, so $g$ takes at most $m + 1$ steps, of which $m - 1$ are growth
activations. A growth whose first step fails takes one step. An outer growth
is a growth of a key, and a key has at most two (*Families of growths*); the
keys at a position number at most a constant times $K$.

2. $\alpha'$ is a step of $g'$, so $g'$ starts inside $\alpha$, at
$p' \ge p$. If $p' = p$, $g'$ starts while $g$ grows at $p$; the two calls
are of one source, so of rules of one cycle, and $g'$ is not outer. So
$p' > p$, and $p' \le e'_1 \le q$, where $e'_1$ ends the first result of
$g'$. Step $\alpha$ evaluates the body of the leader of $g$ from $p$, and
an evaluation inside it that starts at a position after $p$ and before $q$
lies on a path that passes through no read of the growing result of $g$ that
succeeds and no call whose result includes one, since such a read ends at the
end $q$ of the seed and such a result at $q$ or later. If $p' < q$, $g'$ lies
on such a path. When the path reads no growing result at $p$, it is a part
at $p$ that the claims before *Calls in a context of growths* bound, and since
$g'$ consumes input past $q$ for its activation $\alpha'$, so does the unit of
$\alpha$ that contains it: those claims make $\alpha$
one of the constant number of steps of $g$ that they leave out, or $q$ at
most a constant after $p$, as in the second case of
*Repeats inside a growth*. If $p' = q$, $g'$ is an outer growth at $q$, and its
key has at most two.

3. A read of a growing result at $p$ is a call at $p$ of a rule of a cycle
that reads a growth of its own cycle open at $p$ ([Lemma 2 of the memoization
proof](memoization.md#lemma-2-reading-an-open-growth)). On the path none
reads the growing result of $g$ and succeeds, and no result on it includes
one, so each read is of one of three kinds. A read of a growth of another
cycle that was in progress when $g$ started reads a result that does not
change before $g$ ends (*Evaluations that read no growing result*). A growth
of another cycle that the step starts at $p$ reads no growing result of the
cycle of $g$, since a rule of another cycle reads only those of its own, and
its call is in no context of growths ([Evaluations of
Rules](../runtime/parsing.md#evaluations-of-rules)), so the memo stores its
result and answers the call in the later steps, which make it in the same
lookahead state. A growth of another leader of the cycle of $g$ that the step
starts at $p$ is of the first or the third kind of *Growths inside a step*,
since a result of the second kind includes the growing result of $g$, so it
has the result of an earlier growth that it takes again or one of a constant
number of results. The other evaluations that the path makes at $p$ call no
rule of the cycle at $p$, and the steps evaluate them alike (*Evaluations
that read no growing result*). So the alternatives that the path takes at $p$
and the results that it reads there take a constant number of combinations
over the steps of $g$, and each fixes where the path leaves $p$. What the path
evaluates after that reads no growing result at $p$, so the steps evaluate it
alike for one combination. The call that starts $g'$ is a call of a leader,
which the parse memoizes, and no growth of its cycle is in progress at $p'$,
since $g'$ is outer, so the call is in no context of growths: the memo stores
the result of $g'$ when $g'$ ends and answers the call in every later step
that makes it for the same combination. So the outer growths on such paths
start in at most one step of $g$ for each combination.

For the second sentence, let $\alpha'$ lie inside the activation $\alpha$
before it, a step of an outer growth $g$. By part 2, the growth of $\alpha'$
starts in $\alpha$ on a path of the second case or at $q$, where it is one of
at most two for its key. If the path reads no growing result at $p$, the
conclusion of *Repeats inside a growth* holds for $\alpha$. Otherwise
$\alpha$ is one of the constant number of steps of $g$ above. Each $\alpha'$
has one activation before it, so these $\alpha'$ number at most as many as
those steps over all the outer growths and the outer growths at the
positions, at most a constant times the number of outer growths, which part
1 bounds by a constant times $K\hat{n}$. $\square$

Lemma 4 bounds the successful activations of a source at a position from the
restarts, *Direct activations* and the links, which it proves for
iterations, operators and skip expressions. Part 2 is the first point of the
restarts for growth activations, where an iteration has $\alpha'$ inside
$\alpha$ only through a left call that condition 3 excludes: a step starts at
the start of its growth, before the position of its activation. Part 3
counts the activations that lie inside the one before them. The claims below
give the rest of the restarts, *Direct activations* and *Repeats* for growth
activations, which differ from an iteration in that they start at $p$, not
at $q$, and read the input from $p$ to $q$ through their seed. Let $c_W$ be
the length of the longest word of a unit that is committing by its words
(case 4), a constant of the grammar. The successful growth activations at $q$
whose growth starts after $q - c_W$ number at most a constant times $K$,
since the outer growths at a position are at most two for each key there;
they are counted so, and the claims leave them out.

**Restarts of growth activations.** Let $\alpha$ and $\alpha'$ be successful
growth activations of one source at $q$, $\alpha$ first, of outer growths $g$
at $p$ and $g'$ at $p'$, let $\alpha'$ not lie inside $\alpha$, and let $N$
be the innermost evaluation that contains both. If $N$ is no growth, the
conclusion of the restarts holds with the start of $g'$ for the start of
$\alpha'$: on the path from $N$ down to $\alpha$ there is a first abandoned
evaluation $w$ below $N$, a dropped step of a growth or a committing unit
that starts at a position $s \le p$, and from the end of $w$ to the start of
$g'$ the parser starts no evaluation at a position before $s$, apart from the
exception of the restarts. If $N$ is a
growth, the two cases of *Repeats inside a growth* hold with the start of
$g'$ for $\alpha'$.

*Proof.* $\alpha$ starts first, so it does not lie inside $\alpha'$. Let $N$
be no growth. It has a child $c_1$ that contains $\alpha$ and a later child
$c_2$ that contains $\alpha'$. If $N$ tries $c_2$ after $c_1$ failed, $c_1$ is
abandoned. Otherwise $c_1$ succeeded and $N$ continues at its end, so
$p' \ge \mathrm{end}(c_1)$. If no evaluation on the path from $c_1$ down to
$\alpha$ were abandoned, each would succeed with a result that contains the
result of the next; $\alpha$ is kept, since its result becomes the current
result of $g$, and $g$ returns a result no shorter, so $\mathrm{end}(c_1)$
would be at or after the end of the result of $\alpha$, which is after $q$,
while $p' \le q$. So $w$ exists. It contains the call that starts $g$, so
$s \le p$, and the rest of the proof of the restarts applies with the start
of $g'$ for that of $\alpha'$: an evaluation that returns the parser before
$s$ would enclose $w$ and fail before $g'$ starts, on the path from $N$ to
$w$, where nothing fails; and a unit $w$ holds the call that starts $g$, a
scanning source, and is the expression of a lookahead or failed after
$\alpha$ succeeded, so condition 2 makes it committing. When $w$ is a dropped
step of a growth whose seed ends at $e$, $e \le \mathrm{end}(c_1) \le p'$ as in
the restarts, and *Dropped steps* applies with the start of $g'$ for that of
$\alpha'$ and the call that starts $g$ for the source: in its first case that
call follows the read of the seed, so $e \le p$, and the step is a committing
unit at $e$; in its second case the evaluation $w'$ that it gives holds the
call that starts $g$, since $g$ and the steps of $g$ above $\alpha$ are not
abandoned, so $s' \le p$; and in its third case $g$ starts in a part that the
dropped step evaluates at the position of its growth, which the count after
*Repeats inside a growth* charges $\alpha'$ to. Let $N$ be a growth
at $p_N$, with $\alpha$ in its step $j$ and $\alpha'$ in a later step $j'$.
In the first case of *Repeats inside a growth*, $g'$ starts after a read of
the growing result of $N$ that succeeds, or after a call whose result
includes one, so $p' \ge e_{j'} \ge e_{j+1}$ for the ends $e_k$ of the seeds
of $N$; if nothing on the path from step $j$ down to $\alpha$ were
abandoned, the result of $\alpha$, which ends after $q \ge p'$, would lie in
the result of step $j$, which ends at $e_{j+1}$. The rest of that proof
applies as written. In the second case $g'$ lies in a part that step $j'$
evaluates at $p_N$ on a path that reads no growing result of $N$ that
succeeds, and it consumes input past $q$, so the unit that holds it does,
and the claims at the position of the growth bound it as they bound
$\alpha'$ there. $\square$

**Direct growth activations.** A growth activation is **direct** in the body
evaluation that makes the call that starts its growth. Let $E$ be a body
evaluation of a rule $Z$ that is not a growth step, and $\sigma$ the growth
at a call of the body of $Z$. Of the successful growth activations of
$\sigma$ at $q$ whose growth starts at or before $q - c_W$, at most two are
direct in $E$, unless a path that enters the growth again leads from the
abandoned unit to a later one (case 5); when there are two, their growths
are the two outer growths of one key, one in lookahead state $1$ and one in
state $0$. Let $\alpha$ and $\alpha'$ be such activations direct in different
body evaluations of $Z$, their innermost common evaluation no growth, $w$
as in *Restarts of growth activations*, the unit that *Dropped steps* gives
when the abandoned evaluation is a dropped step, and let no dropped step at
the position of its growth and no path that enters the growth again lead from
$w$ to $\alpha'$. Then $\alpha$ lies in the evaluation inside $w$ of a call
that $w$ and its continuation share and that contains the body evaluation in
which $\alpha$ is direct, or $g'$ is the second outer growth of its key.

*Proof.* Let $\alpha$ and $\alpha'$ be two, $\alpha$ first, of growths $g$ at
$p$ and $g'$ at $p'$, started by evaluations $c_1$ and $c_2$ of the call made
by $E$. They are distinct, since the steps of one growth have distinct seeds,
and $c_2$, made by $E$, is not inside $c_1$, so $\alpha'$ is not inside
$\alpha$. Their innermost common evaluation lies in the body of $E$ below
every call, so it is no growth, and the claim above gives $w$. Below $c_1$
the path to $\alpha$ holds only $g$, which succeeds, and its kept step
$\alpha$, so $w$ is a unit of the body of $E$ that holds $c_1$. Let $\kappa$
and $t$ be as in *Direct activations*. If $c_1$ is not one of the calls that
$w$ and $\kappa$ share, it comes after them, so $p \ge t$, and the path of
$w$ that holds $\alpha$ consumes after them the input from $t$ to the end of
the result of $\alpha$, after $q$: before $c_1$ when $p > t$, in $g$
otherwise. The character at $t$ is then a first character of $w$ after the
shared calls. $c_2$ is not a shared call in $\kappa$: such a call ends where
the call at the same place in $w$ ends, at or before $t \le q$, and the
result of $g'$ ends after $q$. So $g'$ starts at $p' \ge t$ after them and
consumes the input past $q$, and every path of $\kappa$ reaches the positions
after $t$ only through the shared calls and the character at $t$, which is
then a first character of $\kappa$ after them too. In case 1 this
contradicts disjointness. Case 3 does not hold, since $c_1$ starts a growth,
a scanning source, after the shared calls. In case 4 the path of $w$ that
holds $\alpha$ matched its word $W$ from $t$, and $\kappa$ consumes the input
up to past $q$ on every path that does not fail, stop or meet a known
negative lookahead before $t + |W|$, so $q < t + |W| \le p' + c_W$, which the
claim leaves out. So, outside case 5, $c_1$ is a shared call. Its evaluation
in $\kappa$ has the same result (case 2), which ends after $q$, so $g'$, which
starts at or before $q$ and ends after it, lies in that evaluation, as the
shared calls before it end at or before $p$ and what follows it starts after
$q$. That evaluation is made in the body of $E$ before $\kappa$ leaves it,
and $c_2$ lies in it with no call between, so it is $c_2$: a call of the
leader of $g$ at $p$ with the key of $c_1$. The call is in no context of
growths, since $g'$ is outer, so the memo answers it in the lookahead state
of $c_1$ or a greater one, and $g'$ is the other outer growth of that key.
So a third activation direct in $E$ would be in a third outer growth of the
key, which does not exist.

For the last sentence, the argument above, with any $w$ on the path from the
innermost common evaluation to $\alpha$, shows that $\alpha$ lies in a call
that $w$ and $\kappa$ share, outside case 5 and the activations that start
after $q - c_W$, as in *Repeats*. It uses of $w$ only what *Dropped steps*
gives of the unit that it gives: when that unit is a step at the end of its
seed $e \le p$, the parser returns to no position before $e$ until
$\alpha'$ apart from the exception of the restarts, which lies in growths
that end before $\alpha'$, and its shared calls and its first characters after them are
those of the step tail that holds $c_1$, so the character at $t$ is one of
them; when it is an evaluation
$w'$ below the step, it holds $c_1$, and what the parser evaluates after it
is its continuation. The shared call contains $c_1$, so it is $c_1$ or
contains $E$. If it is $c_1$, $g'$ lies in the evaluation of that call in
$\kappa$, which has the key of $c_1$ and is in no context of growths, by the
paragraph above. The memo does not answer it, since $g'$ lies in it, so it
grows the leader as the other outer growth of that key, which takes the steps
of the first (*Second outer growths*). If $g'$ is that growth, it is the other
outer growth of the key of $c_1$. Otherwise the call that starts $g'$ has a
counterpart at the same place in the first outer growth, or, where that
growth found a call above the place in the memo, in the earlier evaluation
that stored the entry; either way the counterpart has the same key, is made
before it and is in no context of growths, since $g'$ is outer; that call
grew the leader or the memo answered it from an earlier growth of the key, so
$g'$ is the second outer growth of its own key. $\square$

A shared call that is the call $c_1$ itself, made by the unit directly or as
the first thing that the body of a call that it starts with evaluates, arises
only for a growth activation. In *Direct activations*, $w$ is a unit of the
body of $E$ and $\alpha$ lies in no call of $w$, so it lies in no shared call,
whether the unit makes that call directly or inside a call that it starts with;
such a call changes only the position $t$ where the shared calls end, and the
argument at $t$ holds as written. *Repeats* places $\alpha$ in the evaluation
inside $w$ of some shared call, wherever the unit makes it, and a link takes
that evaluation and the one of the same call that $\kappa$ makes at the same
position; neither identifies the shared call with a call of a given body
evaluation, which the paragraph above needs of the call that starts a growth.

So the successful growth activations of $\sigma$ at $q$, outside those that
start after $q - c_W$, case 5, the dropped steps at the position of their
growth, those of second outer growths, the second case of
*Repeats inside a growth* and those of part 3, are at most two for each body
evaluation in which they are direct, and the first of each, in the order in
which they start, form a sequence $\alpha_1, \alpha_2, \ldots$ as in
*Repeats*: direct in distinct body evaluations of $Z$, with $\alpha_{i-1}$ in
the evaluation $e_i$ of a call shared by a committing unit and its
continuation, which contains the body evaluation of $\alpha_{i-1}$, and the
links, free activations and reference links defined from them. The owner of a
growth activation is that of the call that starts its growth, and the
evaluations that hold it are those that hold that call. The rule of a link
holds a call, so every link is memoized and no leaf link arises, and the
holder of $\alpha_i$ lies in $e'_i$ when $\alpha_i$ follows a link, since
$e'_i$ contains the body evaluation in which $\alpha_i$ is direct. The
growth activations owned by memoized evaluations are those of *Activations
held by heads*, whose count rests on Lemma 3, *Replayed activations* and
*Links to heads* and on no bound of this lemma, so part 1 can use it in the
order of Lemma 4; the others are those of the families of growths.

**Second outer growths.** The outer growths of a key are at most two, the
first in lookahead state $1$ and the second in state $0$. They read no growth
open at their start, so their steps have the same results, which the
lookahead state does not change (*Counterparts*), and each successful growth
activation of the second at $q$ is matched to the one of the first at $q$.
An outer growth has at most one step whose seed ends at $q$, so the
successful growth activations at $q$ of the second outer growths, of all
sources, are at most those of the first outer growths, which are none of
them. A bound on the others that leaves out any of the former therefore
bounds them too, doubled.

Two kinds of growth activation that the claims above leave out are of second
outer growths. First, a reference link of $\mathcal{R}$, which stores
nothing, can be the call that starts the growth itself, $e^{k\prime}$, the
counterpart of the call $c_2$ that starts the growth of a free $\alpha_i$,
with $e^k$ an earlier call of the same leader at the same position, level
and values; *Reference links lead to repeated evaluations* then places the
activation outside $e^{k\prime}$, in the holder of $c_2$, and does not
apply. The two calls lie in the same growths of the cycle (case 2), and
$c_2$ is in none, since its growth is outer, so $e^k$ reads no growth open
at its start. Follow the path from the evaluation of $\mathcal{R}$ where the
parse begins down to $e^k$, as in *Heads in the reference parse*; the parse
skips no alternative on it, since $e^k$ holds $\beta_f$. If the parse
evaluates every call and step on it, it grows the leader at the counterpart
of $e^k$, outside contexts of growths, before $c_2$. Otherwise the first call
or step $y$ that it does not evaluate contains $e^k$ in $\mathcal{R}$ and not
$e^{k\prime}$, since the parse evaluates $c_2$, so it is answered before
$c_2$ starts, and the induction of the covered keys gives a growth of the key
of $e^k$ before then, outside cases (a) and (b): it applies to a call of a
leader that reads no growth open at its start as to the others, since the
parse answers such a call only from an entry that a growth of its key
outside contexts stored. Either way an outer growth of the key of $c_2$
precedes it, and the growth of $\alpha_i$ is the second outer growth of its
key. Second, a path that enters the growth again (case 5) leaves out of
condition 2 only an alternative that evaluates its scanning sources inside
calls that are memoized at the call and read no variable, the call that
starts a growth among them, and what follows the call of the leader that the
path reaches holds no scanning source ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost)). Where the
alternative is evaluated again, and in the steps of that growth after the
first, which evaluate the bodies of the cycle again before the left call at
the same positions, the memo answers each call that started an outer growth
before, in the same lookahead state or a greater one, so a growth activation
that the path repeats there is of the second outer growth of its key.

The first step of the growth that the path enters repeats what the path left
only when the path starts that growth: when a growth of the cycle is in
progress at the position of the alternative, the path reads its growing
result, and the step that evaluates the alternative again is a later one.
Let the path start the growth, at the position $p_0$ of the alternative. Its
first step evaluates the alternative again with the values of the variables
that the alternative read when the path left it. These are variables that the
rules of the cycle read, since the alternative is part of the body of one, the
rule $r$ that holds it. The alternative and the path start at $p_0$, where $r$
is called, since the path reaches a left call; the path reaches the call of
the leader through left calls of rules of the cycle, and the first step
reaches the call of $r$ that evaluates the alternative again from the body of
the leader through calls made at $p_0$ before input is consumed, so through
rules that are on the cycle too. A binding of a variable that the rules of a
cycle read is a compile-time error where it may run before a call into the
cycle that may run before consuming input, in the body of a rule of the
cycle, and a binding in a called rule ends with that call ([Left
Recursion](../language/parser-expressions.md#left-recursion)). A binding
that runs in the body of a rule on these two chains of calls before the next
call of the chain is of that kind, and so is a binding in $r$ before the
alternative, since it also precedes the left call of the path. So these
variables have the same values at the start of the alternative, at the call
of the leader, in the body of the leader that the first step evaluates and
at the start of the alternative in it. The alternative
makes no left call of the cycle, so it reads no growing result and evaluates
again as before, up to its calls, which read no variable and have the same
keys. Those that are memoized at the call are outside contexts of growths: a
call at a position after $p_0$ lies in no growth at its position, which would
contain the alternative, started at $p_0$; a call at $p_0$ is of a rule off
the cycle, since a call of a rule of the cycle there would be a left call, and
it is no left call of its own cycle, so it is outside every growth of that
cycle at $p_0$ ([Conditions for Linear
Cost](../runtime/parsing.md#conditions-for-linear-cost), the left calls). So
the memo stored or answered each of them when the alternative made it, and
answers it in the first step, made inside the evaluation that holds the
alternative and at the same place in it, so in the same lookahead state
(*Answered calls*). The scanning sources of the alternative, the calls that
start a growth among them, lie inside these calls, so the first step repeats
none of their activations.

The growth activations that a dropped step repeats in a part that it
evaluates at the position of its growth are charged as the count after
*Repeats inside a growth* charges the third case of *Dropped steps*, which
*Restarts of growth activations* applies to them.

The family claims apply to the growth activations that growth steps and
evaluations in a context of growths own, with the place of a growth activation
that of the call that starts its growth, which is in no context of growths,
since its growth is outer. *Families of growths* and *Covered growths* use the
owner of an activation, not its position. *Heads hold what later growths hold*
follows the path from the head $h$ down to that place: if the parse evaluates
every call and step on it, the call at that place in $h$ has the key of the
call $c$ that starts the growth of the activation and comes before $c$, so
either it grows the leader there and $h$ holds a successful growth
activation at $q$, or the memo answers it from an earlier growth of the key
and the growth of $c$ is the second outer growth of its key, which *Second
outer growths* leaves out; the other cases of its proof apply as written.
*Links lead to later growths* and the remark after it use of an activation
the evaluations of its link that contain the call that starts its growth and
its counterpart in $\mathcal{R}$, as the claims from *Memoized links* on do,
and *First activations of heads* uses these and follows the path as *Heads
hold what later growths hold* does, with the same exception. *Repeats inside a family* takes
*Restarts of growth activations* and *Direct growth activations* for the
restarts, *Direct activations* and *Repeats*, outside the growth activations
that those leave out: the shared call that contains the call $c_1$ that starts
the growth of $\alpha$ is $c_1$ itself, which makes $g'$ a second outer
growth, or a call that the family evaluates and that contains the body
evaluation that makes $c_1$, which the proof makes a growth of a leader at
$p$; in item 2, $q \ge e_{j+1}$ holds since *Restarts of growth activations*
starts $g'$ at or after $e_{j'}$.

*Consecutive repeats inside a family* does not apply as written. The second
growth of $R$ takes the same steps as the first down to the place of $c_1$,
but the call there has the key of $c_1$ and is in no context of growths, so
the memo answers it, or it starts the second outer growth of that key: no
growth activation of a first outer growth repeats $\alpha$ between $\alpha$
and $\alpha'$. When $\alpha'$ lies in the evaluation of the shared call that
$\kappa$ makes, where $R$ grows again, the call that starts the growth of
$\alpha'$ has a counterpart at the same place in the first growth of $R$, or
in the earlier evaluation that stored an entry that growth found above the
place, with the same key, made before it and in no context of growths; that call
grew the leader or the memo answered it from an earlier growth of the key,
so the growth of $\alpha'$ is the second outer growth of its key. The claim
below takes the pairs where $\alpha'$ lies outside that evaluation, and
replaces the repeat of $\alpha$ by the call that the memo answers in its
place.

**Answered places.** Let $g$, $\sigma$ and $q$ be as in *Repeats inside a
family*. A **stand-in** at $q$ is a call of the leader at a call whose
scanning source is $\sigma$, made in the family region of $g$ at a position at
or before $q$, which would start a growth of $\sigma$, and which the memo
answers with the result of an earlier outer growth of its key, a result that
ends after $q$. Let $\alpha$ and
$\alpha'$ be consecutive successful growth activations of $\sigma$ at $q$
under the hypotheses of *Repeats inside a family*, and let $\alpha'$ be none
of the growth activations that *Restarts of growth activations* and *Direct
growth activations* leave out: those whose growth starts after $q - c_W$,
those of case 5, of a dropped step at the position of its growth, of a second
outer growth, of the second case of *Repeats inside a growth* and of part 3
of *Growths at many positions*. Then the second alternative of item 2 of
*Repeats inside a family* does not hold for $\alpha$ and $\alpha'$, unless
$\alpha'$ lies in the second step of a growth whose first result is empty, and
item 1 holds only when $\alpha'$ lies in a growth at $p$ that starts after
$\alpha$, in a later step of it than a stand-in at $q$, such as a growth of a
leader of another cycle started in $\kappa$.

*Proof.* Suppose that one of them holds. Then $w$ starts at $p$ with a call
$c$ of a leader $R$ that it shares with its continuation $\kappa$, $\alpha$
lies in the growth of $R$ that $c$ makes, in a step $j$ of the innermost
growth at $p$ that contains $N$, and, by the paragraph above, $\alpha'$ lies
outside the evaluation of $c$ that $\kappa$ makes. Let $x_0 = \alpha$, and let
$x_k$, $w_k$, $c_k$, $R_k$ and $\kappa_k$ be such a call or activation and
such a unit, shared call, leader and continuation, with $x_k$ in the growth
of $R_k$ that $c_k$ makes in a step $j_k$ of the innermost growth at $p$ that
holds $w_k$, and $\alpha'$ outside the evaluation of
$c_k$ that $\kappa_k$ makes, starting with $w$, $c$, $R$ and $\kappa$ for
$k = 0$ and $j_0 = j$.

As in the proof of *Consecutive repeats inside a family*, $\kappa_k$ makes its
call of $c_k$ after $w_k$ ends and before $\alpha'$, in step $j_k$, and $R_k$
grows again there and takes the same steps as the first time down to the
place of $x_k$: every evaluation on that path lies in the family region and
is evaluated again, with no reuse in a context, the calls off the path have
the same results and the values of the variables do not change. The call at
that place has the key of the call that starts the growth of $\alpha$, is in
no context of growths, and is made after the growth of $\alpha$ ended. If it
starts a growth, that growth is the second outer growth of the key and takes
the steps of the first, so it holds a successful growth activation of
$\sigma$ at $q$ in the family region, after $\alpha$ and before $\alpha'$,
which contradicts that they are consecutive. So the memo answers it with the
result of a growth of the key, which ends after $q$, and the call is a
stand-in $x_{k+1}$, made after $x_k$ and before $\alpha'$. It lies in no call
of a rule that calls no rule, as $x_k$ does not, and in no dropped step: the
call of $c_k$ in $\kappa_k$ lies in the same steps of the growths at $p$ as the
one in $w_k$, which hold $x_k$ and so are not dropped, and the growth of
$R_k$, and every growth at $p$ that it nests on the path down to $x_{k+1}$,
drops the same steps as the first time, since it takes the same steps.

*Restarts of growth activations*, *Direct growth activations*, *Repeats* and
*Repeats inside a family* use of the earlier of the two activations only that
it lies inside the evaluations above it on its path, that it is a call that
starts a growth of $\sigma$, a scanning source of every unit that holds it,
that it starts at or before $q$ and that the path consumes through it the
input from its start to past $q$, that an outer growth of its key starts no
later than it does, that no dropped step and no call of a rule that calls no
rule holds it, and that every evaluation on the path from $g$ down to it lies
in the family region. A stand-in has all of these, so they apply to $x_{k+1}$
and $\alpha'$.

Three kinds that they leave out are defined by the pair, through the
abandoned evaluation $w_{k+1}$ that they take on the path down to $x_{k+1}$
or the innermost evaluation $N_{k+1}$ that contains $x_{k+1}$ and $\alpha'$,
and the counts of these kinds need an activation in place of $x_{k+1}$. They
do not apply here, apart from the one case below. The third case of *Dropped
steps* needs $w_{k+1}$ to be a dropped step, and $w_{k+1}$ holds $x_{k+1}$,
which no dropped step holds. Case 5 places $\alpha'$ where the growth that the
path from $w_{k+1}$ enters evaluates that alternative again; there, by
*Second outer growths*, which uses of the pair only that place, the first
step of the growth repeats no growth activation of the alternative, and a
later evaluation of it finds each call that started an outer growth in the
memo, so $\alpha'$ is of a second outer growth, which is excluded. The second
case of *Repeats inside a growth* needs $N_{k+1}$ to be a growth at $p$ with
$x_{k+1}$ in an earlier step than $\alpha'$, and $\alpha'$ on a path of its
step that reads no growing result that succeeds. $N_{k+1}$ lies in $N$, which
contains $x_{k+1}$ and $\alpha'$, since $x_{k+1}$ is made after $\alpha$ and
before $\alpha'$. If $N_{k+1}$ contains $\alpha$, it contains $N$, so it is
$N$; then $\alpha$ lies in a step of it no later than the one of $x_{k+1}$, and
$\alpha'$ is of the second case for $\alpha$, which the hypothesis excludes.
For $k = 0$, $N_1$ contains $\alpha$ when it is a growth of the cycle of $R$:
it contains the call of $c$ in $\kappa$, which holds $x_1$ and not $\alpha'$,
so also the call of $c$ in $w$, which holds $\alpha$, since the two calls lie
in the same growths of that cycle (case 2). There remains a growth $N_{k+1}$
at $p$ that starts after $\alpha$, with $\alpha'$ in a later step of it than
$x_{k+1}$, on a path that reads no growing result that succeeds: for $k = 0$
a growth of a leader of another cycle than that of $R$, started in $\kappa$
after $w$ ends and before its call of $c$, which it contains. $N_{k+1}$ lies in
$N$ and is not $N$, which contains $\alpha$. If $N$ is not a growth, this is
the case of item 1 that the claim states. If $N$ is a growth, with $\alpha$ in
its step $j$ and $\alpha'$ in a later step $j'$, then $N_{k+1}$, which $N$
nests and which contains $\alpha'$, lies in step $j'$ and starts at $p$. A read
of the growing result of $N$ that succeeds in step $j'$ ends at the end
$e_{j'}$ of the seed of the step, and a call whose result includes one ends
there or later. When that seed is not empty, $e_{j'} > p$, so the path from
step $j'$ down to the call that starts $N_{k+1}$ at $p$ goes on from the end
of no such read or call, and inside $N_{k+1}$ the path down to $\alpha'$ reads
no growing result that succeeds. So $\alpha'$ lies in a part that step $j'$
evaluates on a path that reads no growing result that succeeds, in the second
case of *Repeats inside a growth* for $\alpha$, which the hypothesis excludes.
When the seed is empty, step $j'$ is the second step of $N$ and the first
result of $N$ is empty, the case that the claim states.

Outside that case, the claims give a unit $w_{k+1}$ at $p$ and its
continuation $\kappa_{k+1}$ that share a call containing $x_{k+1}$, made in
the step $j_{k+1}$ of the innermost growth at $p$ that holds $w_{k+1}$. That
call is $x_{k+1}$ itself or a call $c_{k+1}$ of a leader $R_{k+1}$ that grows
at $p$ and contains it. In the first case, by *Direct growth activations*,
the growth of $\alpha'$ lies in the evaluation of that call that
$\kappa_{k+1}$ makes, a call with the key of the growth of $\alpha$ that the
memo does not answer, since $\alpha'$ lies in it. So it grows the leader as
the second outer growth of that key, and the growth of $\alpha'$ is that
growth or, by the end of the proof of *Direct growth activations*, the second
outer growth of its own key, which is excluded either way. In the second case, if $\alpha'$ lies in the evaluation of
$c_{k+1}$ that $\kappa_{k+1}$ makes, its growth is the second outer growth of
its key by the paragraph above, which is excluded; otherwise $k + 1$
satisfies what $k$ did. The calls $x_1, x_2, \ldots$ are made one after
another before $\alpha'$, so there are finitely many of them, and the
construction cannot go on forever. $\square$

So the conclusion of *Consecutive repeats inside a family* holds for growth
activations outside the kinds above: if the innermost common evaluation of
two consecutive ones is a growth, the later one is in the second case of
*Repeats inside a growth*, whose count needs only an activation of $\sigma$
at $q$ in an earlier step of the same growth, which $\alpha$ is, or it lies
in the second step of a growth whose first result is empty, a single step of
each growth, which the count lists with the steps that hold such an activation. The count after that claim then applies to them as written.
For the pairs of the first kind, which the claim leaves where the later
activation lies in a growth that starts after the earlier one, it uses of a
pair only that both lie in one step of the innermost growth at $p$ that
contains them and that the earlier one lies in a growth at $p$ that the step
starts, which item 1 gives; it charges the pair to the earlier one, and
consecutive pairs have distinct earlier ones.

The claims from *Memoized links* to *Activations held by heads* apply to
growth activations by the correspondences above. They use of an activation
the evaluations that contain it, the order in which the activations start and
its counterpart in $\mathcal{R}$, but not its position. *Memoized links*,
*Links lead to repeated evaluations* and *Links to heads* speak of the two
evaluations of a link, which contain the call that starts the growth of the
activation. *Calls that call no rule* and *Memoized links keep positions* take
a rule of the link that calls no rule, which no link of a growth activation
has. *Reference links* needs that the evaluation in $\kappa$ of a shared call
holds a successful growth activation at $q$ where the one in $w$ does. The two
take the same steps, and a growth inside them is outer in both or in neither.
A growth of its cycle that is open at its call and is at a position after the
start of the link started inside the evaluation. One at the position of the
link is open at the start of both evaluations, or at the start of one of
them only. In the second case it is case (a) of the covered keys, or the
growth of a step with an empty seed, and then no call at that position of a
rule of the cycle of that growth is made inside the shared call (*Steps with
an empty seed*).

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
can answer a call of $r$ from its memo, evaluates it no more often. Apart from its calls, the body of
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
parse evaluates next is what $\mathcal{R}$ evaluates next once it leaves that call. A parse
that keeps records of [run sites](../runtime/parsing.md#run-sites) or
[scan sites](../runtime/parsing.md#scan-sites) evaluates, besides, none of the iterations that a
record answers: the walk that takes the record ends where the walk that evaluated them would end,
with the same outcome and environment (Lemmas 1 and 4 of [Run Sites](run-sites.md)), which by
(I1) is where the walk of $\mathcal{R}$ ends, so the same induction goes on past that walk, and
such a parse evaluates the same expressions at the same positions, in the same states, less the
evaluations inside the answered calls and inside the iterations that records answer. The claim is
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
[Run Sites](run-sites.md)), so a parse that keeps them makes a subset of the evaluations of
$\mathcal{R}$ (*The reference parse* above), and the claim holds of it. It is not shown to make
a subset of the evaluations of the memoized parse that keeps no records, since a call inside an
iteration that a record answers stores nothing in the memo, and a later call that the parse
without records answers from that store may then be evaluated in the parse with them. The proof
does not use that comparison. The region bound replaces only the run
bound: the second bound, which counts the evaluations at one position, is unchanged, and a
scanning source inside the body of a region rule, such as `(!"*/" .)*` in an unclosed comment,
stays a scanning source, so the input that a failed comment examined is still counted against the
units around it.

## Theorem (Linear Cost)

When every rule that the start rule reaches satisfies conditions 1 to 5, every
parse from the start rule whose records are exact, (R1) of the
[Setting](#setting), has a cost,
apart from the suffix copies of the walks that take records, of at most
$c \cdot K \cdot (L + 1) \cdot (n + 1)$
for a constant $c$, where $K$ is here the greater of the $K$ of
[Classification](../runtime/parsing.md#classification) and $K_R$ of the
[Setting](#setting). When no variable holds a string and every binding binds a
literal or a variable with finite values, $K$ is a constant by Lemma 8 and
$L = 0$, so the cost is at most $c' \cdot (n + 1)$.

The theorem covers every parse whose records are exact, which the
[Setting](#setting) shows of the records of every implementation. Its
recorded sites decide which lemma counts a repetition: Lemma 17 counts the
iterations of the recorded sites of the parse, and the count of Lemma 4 the
successful iterations of every other repetition. By *Sites that meet the
conditions* in the [Setting](#setting), a candidate site is shown to be
recorded only when it is a run site free of the environment, marked free
where the program is loaded, in a parse of a backend of the engine, in the
[Scope](run-sites.md#scope) of the Run Sites proof and without missing-token
insertion. A run site in a generated Go or TypeScript parser, a run site
that is not free of the environment and a repetition of a single character
without an upper bound outside the reused skips and the `#stream`
repetitions, in any implementation, are not shown to be recorded, so for a
parse that walks one of them the bound rests on the count of Lemma 4 for
that repetition, with the parts that step 1 of [Open Steps](#open-steps)
leaves open, and on the seventh part of step 8 where the repetition keeps
records.

The theorem rests on Lemmas 4 and 5, which are outlines, on Lemma 6,
which is proved given step 1 of [Open Steps](#open-steps), on step 3, since
Lemmas 2, 10 and 17, which its proof uses, leave out the tries of an Operators Rule
outside its activations, and, for the values that actions build from reshaped
values, on step 7. It takes as a hypothesis the order of the bounds that
*Order* after Lemma 17 states, and it leaves the parts that step 8 lists. So
the promises of the classes Linear and Linear Given an Environment Bound
rest on these too.

*Proof, given Lemmas 4 to 6 and step 3.* Each unit of $C$ is the local cost of
a body evaluation, the scanned cost of one, a call, or the walk cost. The body
evaluations of memoized rules are at most a constant times $K \hat{n}$: by
Lemma 3 outside the growth steps and the contexts of growths, and by Lemma 5
for the growth steps and the evaluations in a context of growths. By Lemmas 4
and 13 the successful activations other than the iterations of recorded
walks are at most a constant times $K \hat{n}$, and by Lemma 17, with step 3
for the tries of the Operators Rules, so are the iterations of the recorded
walks, the recorded walks and the steps that they make outside their
iterations and their suffix copies. The activations that do not succeed
follow from these and the body evaluations (*Activations that do not
succeed* in Lemma 4). So the activations are at most a constant times $K \hat{n}$, and by Lemma 10 the
body evaluations of the other rules are at most a constant times those two
counts plus one. By Lemma 2 the calls are at most a constant times the body
evaluations plus the activations. By Lemma 1 each body evaluation has a local
cost of at most $c_0 (L + 1)$. A call costs a constant and its memo lookup,
which by [Evaluations of Rules](../runtime/parsing.md#evaluations-of-rules)
costs in proportion to the size of its key: the number of variables that the
rule reads, a constant, and the lengths of their string values, each at most
$L$ by its definition; so each call costs at most a constant times $L + 1$.
The scanned cost outside the suffix copies is at most a constant times the
activations and the recorded walks by Lemmas 4 and 17, plus
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
of Document work: the memo protocol of a call of a transient rule, whose results only a Document stores
(a parse from scratch memoizes none of them), except a constant for each
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
of a transient rule compares the strings of the key by identity only, at a
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
same rules as a parse from scratch and also store the results of the transient rules. So a Document parse
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
of the transient rules, which can be several under one key on a
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
Lemma 12. A Document parse that does not complete drops the memo and ends the
sequence; its own Document units are those of Lemma 12, with its $W$, and the
next parse starts a new sequence. No term counts what the Document retains as a whole: each memo entry
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
- [Run sites](../runtime/parsing.md#run-sites), their conditions, the sites
  free of the environment and the repetitions of a single character: the
  candidate sites and the recorded sites of the [Setting](#setting) and
  Lemma 17, with the
  [Run Sites](run-sites.md) proof for the records that an implementation
  keeps.
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
  scan-only rules (with Lemma 6 of the Run Sites proof), Operators Rules that nest, words that exclude a continuation and
  alternatives that can fail only after a `#cut`: Lemma 4, its failure after a source, the cases of
  committing and its claims on links.
- Condition 2 for the steps of a growth after the first, checked as units at
  the end of the seed and compared one step tail at a time: *Dropped steps*
  in Lemma 4, with *Restarts of growth activations* and the count after
  *Repeats inside a growth* in Lemma 5.
- The paths that enter a growth again, for which an alternative is committing
  when its sources lie in memoized calls and what follows the leader has no
  source: the fifth case of Lemma 4, with Lemma 5.
- Condition 2 for growing units and their continuations: Lemma 5,
  *Sources at fixed offsets*.
- Condition 3 (Growth) and condition 5 (Carried growth): Lemma 5. The cuts
  passed after left calls, and the lookaheads and `skip`s that make left
  calls, in a cycle with more than one leader: Lemma 5, *Plain reads*.
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

The outlines leave steps 1 and 3 to 6 to be made precise, step 2 lists what
Lemma 5 proves of the continuations of growing units, step 7 is the part of the
condition of the cost view that the [Cost View](cost-view.md) proof leaves open,
and step 8 lists what the records leave.

Step 1 concerns only the *Sources without records* of Lemma 17, among them
the candidate sites that are no recorded sites of the parse: for the
iterations of a recorded walk, Lemma 17 gives the count from (R2), and none
of the links, runs, frames and heads of step 1 is
needed. Where step 1 speaks of a source, of a scan or of a rule that calls
no rule, it means a source without records and its rule; where it leaves out
a parse without records of scan sites, that case stays open, since the
theorem covers such a parse.

1. **Counting the activations of the sources without records in Lemma 4.** The claims of Lemma 4 reduce the scanned
   cost to the activations, show that the successful activations of a source
   at a position are direct in distinct body evaluations, and link each to an
   earlier one through a call that a committing unit shares with its
   continuation. They charge the activations held by each memoized evaluation,
   those in its own region, to those held by the first evaluation of its class,
   the head, which can be in a context of growths and of either variant, and
   show, by comparing the two evaluations of a memoized link in the reference
   parse, in either lookahead state and either variant, that the link leads to
   an activation held by an evaluation that is not a head, which holds for
   the growths open at the start of the two evaluations since the two calls
   of a rule of a cycle that a unit and its continuation share lie in the
   same growths of the cycle, when the comparison meets no
   entry of an earlier Document parse that answers a call in a context of
   growths, or a call whose corresponding call in the later evaluation is in a
   smaller lookahead state, in a context of growths or of the other variant,
   and no result kept in a
   context of growths whose observed lookahead it enters. So the sum over the positions is proved for the
   activations owned by memoized evaluations of every source in a rule that
   calls a rule or is memoized at its calls, when its memoized links are of that
   kind, apart from the activations that a dropped step at the position of its growth or a path
   that enters the growth again repeats, which Lemma 5 counts (step 4), and
   with no count of the evaluations in a context of growths. The activations that
   do not succeed, and the evaluations of the rules that are not memoized,
   follow from the successful ones and the body evaluations of memoized rules
   (*Activations that do not succeed*), so what is open concerns the successful
   activations alone. An activation need not follow a link of the parse: the
   memo can answer the shared call in the continuation (*Repeats*), and the
   claims follow it through its reference link instead. The activations of the
   reference parse that a dropped step at the position of its growth or a
   path that enters a growth again of the reference parse leads to, which have no reference link, are more than
   those of the parse, and Lemma 5 must count those that are counterparts of
   activations of the parse; this is not shown. Nor are the activations of the
   reference parse whose reference link meets case (a) or (b) of the covered
   keys, case (a) including the iterations that come from an earlier parse
   in a Document parse that resumes runs. Three parts are open.
   The first two are for a source in a rule that calls no rule and is not
   memoized at its calls. First, the leaf links that lead to an
   activation held by a head: such a link can leave a memoized evaluation that
   is not a head through the end of its body. The heads that start at the
   position of the scan are bounded by the classes there (*Memoized links keep
   positions*); those that start before it, and the activations that one head
   holds, are not. *Calls that call no rule* bounds only a run of consecutive
   leaf links whose shared call is the first. The runs are separated by free
   activations, by links of other kinds and by the growth steps and
   evaluations in a context of growths that it leaves out. Among those links
   are the memoized links, and the count of the memoized links rests on the
   bound on the heads, so it cannot in turn bound the runs. Across such links, two outer
   links $l < l'$ at one position $p$ through one call place still contradict
   the sharing of their call when $Y_{l'}$ encloses $w_l$: *Nesting at one end*
   applies with $Y_{l'}$ as $X$ and $Y_l$ as $T$, under its hypotheses and
   with its open points, when both links meet the hypothesis on growth steps
   of *Calls that call no rule*, which keeps both frames from being growth
   steps and the later steps of the growths around the units from starting
   before the position of the scan (*Growths around a leaf link*). In the
   other case the parser starts $Y_{l'}$ after $e'_l$, and an index between
   them has its unit before $p$, so the two links lie in different runs
   (*Outer links across runs*), which occurs in a grammar that the conditions
   accept. When that index is linked by a memoized link whose later evaluation
   is still in progress at the later outer link, no head holds the activation
   of the later link (*Outer links inside a memoized link*), which covers that
   grammar. When that later evaluation ends before the later outer link and is
   the frame that the later outer link leaves, a unit of the index that
   starts before the unit of the earlier outer link ends, in which no
   evaluation outside the shared calls before the earlier evaluation of the
   link fails before that evaluation starts, and none of whose shared calls
   before that evaluation holds the frame of the earlier outer link, meets
   *Nesting at one end*, apart from a scan-only rule of the scan in a parse without
   records of scan sites, which no claim treats, and in a parse that keeps no records of scan sites the unit of
   the index evaluates no call of the rule of the scan at the position of the
   scan after its shared call, unless the sequence of activations leaves out
   the activation of that call (*Memoized links to the frame of an outer
   link*); what else that unit examines there is not determined, since a unit
   that decides its continuations compares each way through a call of a rule
   on its own. That the parser starts nothing before the position of the scan
   from the earlier outer link to the first later index whose unit starts
   before it is shown only when the indices between are linked by links that
   meet that hypothesis. When the unit of the index starts after the unit of the earlier
   outer link ends, it lies in a continuation of the last unit before it that
   encloses the frame of the earlier link, or, in the second form, only its
   part after its read of the seed of a later growth step lies there, if no
   growth step around that frame is dropped (*The last unit around the earlier
   frame*); the second form is not followed further. When the frame of the
   later outer link is the later evaluation of the memoized link of the
   index, the earlier outer activation lies in the evaluation, inside that
   last unit, of a call that the unit shares with the continuation that holds
   the index's later activation. When the frame of the later outer link lies
   inside a body evaluation of the later evaluation of that call, other than
   as that body evaluation, the later outer activation lies in that later
   evaluation too (*Frames around the frame of an outer link*), and no head
   holds it if the two evaluations of the call meet the hypotheses of
   *Evaluated pairs of a shared call*; that neither is in a context of
   growths, and the hypothesis on the growths open at their position, are not
   shown. When that frame is the body evaluation of the later evaluation of
   the call and the frame of the earlier outer link lies inside a body
   evaluation of the earlier one other than as it, or contains the earlier
   one other than as its body evaluation, *Nesting at one end* rules the case
   out, apart from a scan-only rule of the scan in a parse without records of
   scan sites, which no claim treats (*The frame of an outer link as the later
   evaluation of a shared call*). Not shown is the case in
   which the frame of the earlier outer link is the body evaluation of the
   earlier evaluation, where the two evaluations meet the hypotheses of
   *Evaluated pairs of a shared call* but the later outer activation starts
   after the later one ends; that activation lies in the frame of the caller
   of the later call, and no head holds it when the evaluation of the call
   that makes that frame is the later evaluation of such a pair, but it is not
   shown that the unit shares that call with the continuation and evaluates
   it, unanswered, at the same position when the continuation makes it, nor
   that the pair meets the two hypotheses on growths, and no pair is given
   when that frame is a later step of a growth that encloses the unit or an
   evaluation in such a step that follows no read of its seed, or when that
   frame encloses the unit. In the first of these two, every evaluation of a
   key that holds the later outer activation starts at the position of the
   scan, inside that frame, after the frame of the later outer link ends
   (*Caller frames in a later step*), so a head that holds it is one of the
   heads that start at that position, whose activations are counted with
   the activations that one head holds, open above. When the rule of that frame reads a variable or
   is not memoized at the call, its call is shared with no unit, and the pair
   may be sought for any enclosing call whose rule reads no variable and is
   memoized there, to which the same forms apply; this is not shown either.
   In the first form, *Shared caller frames* gives the pair at the calls of
   the caller frames themselves when those frames lie inside the unit, are
   made by calls with which the unit and its continuation start where they
   evaluate the shared call, and their rule reads no variable, is memoized at
   both calls, lies on no left-recursion cycle and is called with one level
   restriction; in the other cases of the first form outside the exception
   of the restarts a head, which can start before the position of the scan,
   can hold the later outer activation, which then belongs to the count of the activations that
   heads hold, open above. Nor is the case shown in which the unit of the
   memoized link between two outer links starts before the unit of the
   earlier outer link ends and an evaluation inside it, outside its shared
   calls before the earlier evaluation of the link, fails before that
   evaluation starts, or one of those shared calls holds the frame of the
   earlier outer link. Nor are the cases
   shown in which the frame of the later outer link encloses the later
   evaluation of the call, or in which the continuation
   reaches the activation of the index in a later evaluation of the call. A later
   evaluation that lies inside that frame, an index that is free and one
   linked by a leaf link at a position before the scan are not treated. So the outer links do not bound the
   runs, and the count of the
   runs, of the heads that start before the position of the scan and of the
   activations that one head holds stays open. For the last of these,
   *Activations that one evaluation holds* reduces the activations that one
   memoized evaluation holds, outside an Operators Rule and a `#recover`, to
   the iterations of one evaluation of a repetition in its own region that
   contain one, and *Consecutive activations in one own region* shows that
   from one activation that the evaluation holds to the next the abandoned
   unit shares a call of the rule of the source with its continuation, and
   that the continuation evaluates that call again in the own region, or
   inside a call of a memoized rule with which it starts, where another
   evaluation holds the repeated activation; neither case is bounded. Inside
   one evaluation of a
   repetition, *Iterations at one scan* bounds a run of linked reaching
   iterations at one position by the shared calls of its first unit, and
   charges an iteration whose linked evaluation it keeps to the position of
   a $q$-frame, at most two per position; *Memoized breaks* keeps a run
   that passes through a memoized break at those boundaries and ends it at
   a free index when the memo answers the call. *Restarts through a
   frame* shows, under the hypotheses of that claim, that a run ends at a
   $q$-frame with a later reaching iteration only when the next unit lies inside the $q$-frame, an
   evaluation of a rule that the parse memoizes at its call, and the
   activation of its iteration is then held inside it when it is an
   evaluation of a key, and that the next run starts its
   later iterations at boundaries of its first link at or after the end of
   the $q$-frame. *Consecutive restarts through frames* shows, under the
   hypotheses of that claim and the condition on the memo, that two consecutive links of restarts through
   $q$-frames are not both plain, each with one variant and the earlier
   evaluation in a lookahead state no greater than the later. *Holders of
   the frames' activations* bounds a run that restarts through $m$
   $q$-frames, plain or not, by three times the number of distinct keys of
   the evaluations that hold the activations of its links, and *Positions of
   the holders* keeps the holder at one class and position across each
   restart in which the next frame enters its activation at the place of the
   earlier one, so that at most six links of a run share one class and
   position of the holder, and *Restarts of the other kind* shows that every
   other restart has a link that is not plain unless a step of a growth or an
   activation that the sequence leaves out lies on the way to the place of
   the next activation, so that $m$ is at most six times one more than the
   number of such links and restarts in the run. *Units below the earlier
   evaluations* shows that when the first abandoned unit below the earlier
   evaluation of a link holds the activation outside the calls that it shares
   with its continuation in the reference parse, and is committing for it by
   first characters or by its word, at most one more link follows than those
   calls and the length of the word, which depend only on the grammar, so in a longer run the activation lies
   inside a call that the unit shares, or the unit commits in another way.
   *Shared calls of the units below* follows the activation through the
   calls that the abandoned units below the earlier evaluation share, at
   any depth: each later link lies, in its own iteration, inside the later
   evaluation of such a call, which the parse evaluates there, and at most
   five of them pass a pair of the two evaluations with one variant and the
   earlier in a lookahead state no greater than the later, apart from leaf
   links through the rule of the source; the others pass a pair that is not
   plain in this way, or meet a growth or an activation that the sequence
   leaves out. *Holders of the later links* shows that each link that passes
   a pair that is not plain and meets the hypotheses of that claim holds its activation in an evaluation whose class
   and position an evaluation made before the earlier evaluation of the first
   link ends has already, with an activation at the same place, so that the
   run adds no class and position of a holder and these links number at most
   six times the classes and positions of such earlier evaluations; a bound
   on them that depends only on the grammar is thus part of the bound on the
   heads that start at or before the position of the scan, open above. The units
   below that commit only by deciding their
   continuation or for a path that enters the growth again stay open.
   Refinements,
   the links that are not plain, the number of
   runs in one evaluation $T$ of a repetition, the number of repetition
   evaluations $T$ that reach $q$, the number of $q$-frame positions,
   iterations that hold more than one activation, and dropped steps,
   growths, case 5 and the exception of the restarts below the repetition
   stay open. Second, the leaf links when the shared call follows other
   shared calls: the continuation then leaves frames at the ends of the
   shared calls before the call, at positions before the scan, and between
   them, as in `e = "(" (e R "x" / e) / "."` with `R = [ ]*`, where the call
   of `e` that both alternatives share comes before the call of `R`. *Calls
   that call no rule* counts only frames left at the position of the scan.
   *Nesting at one end* shows that evaluations of a call that nest without a
   bound and end at one position, through restarts as well, make the call
   nest at one end, which keeps the call of `R` in that grammar from being
   shared, apart from the four points that it leaves open and a nesting
   through a part that a dropped step evaluates at the position of its growth
   before a read of its seed; what remains is
   to bound the frames left at the earlier positions by it. And the leaf links
   between which the parser leaves or starts, at their position, a growth
   step or an evaluation in a context of growths, which that claim leaves
   out, and for which the condition has its case of seed reads (*Seed
   reads*); that case is argued from *Nesting at one end* but not proved:
   besides the four open points of that claim, it is not shown that the
   nested evaluations of a leader that it picks end at the position, since a
   leader whose current result ends there can grow beyond it, nor that each
   dropped step of a growth on the path between them holds the inner one after
   a read of its seed. Third, the
   memoized links that the comparison leaves out: in a Document parse, a link
   whose comparison meets an entry of an earlier parse that answers a call in
   a context of growths, or a call whose corresponding call in the later
   evaluation is in a smaller lookahead state, in a context of growths or of
   the other variant; and a link whose comparison enters a
   lookahead that a result kept in a context of growths observed. Such a link can lead to
   an activation held by a head. The activations include
   those of the copying operations, the units of the own spans that they read
   (Lemma 6). Two points of the claims proved in step 1 are left to make
   precise. The covered keys list the ways in which the parse does not
   evaluate what the reference parse evaluates, among them the iterations
   that a record answers, their case (d), and *Heads in a parse that keeps
   records* shows that this case ends the comparison of a link only for an
   activation inside an iteration of a walk of a site that keeps records;
   the replays leave out the evaluations inside such iterations; what is
   then not shown of a parse that keeps records is in step 8. The
   argument of the replays that a growth read in the reference parse inside $M$ is read
   by $M$ must cover the calls
   that an entry of an earlier Document parse answers, for which it cites (S)
   of the memoization proof without showing that such an entry satisfies it. And the statement on twins that the counterparts take over is
   tested on the Core IR of the generated parsers and on the calls and their
   outcomes in the engine, for rules on no cycle that read no variable, and
   not for the values that the captures of a twin build.
2. **Continuations of growing units.** Lemma 5 bounds, in all but a constant
   number of steps of a growth, a unit followed by a growing unit with which it
   shares no call (*Sources at fixed offsets*), shows that a pair with a shared
   call is then such a pair after shared calls that examine no input, or a
   growing unit followed by a growing unit (*Calls shared with a growing
   unit*), and bounds what a step evaluates at the position of the
   growth without reading a growing result (*Failures at the position of the growth*):
   the later alternatives after a growing unit fails, the parts that a unit
   goes on with at its start after a path through its left call fails, and
   what a unit examines there before a left call without consuming input,
   also when the unit and the continuation share calls, and shows that the
   values of the variables change in a growth only through a binding at its
   position from a growing result, which leaves those claims in force (*Values
   bound from a growing result*), and that a decided unit examines a constant
   amount of the input there (*Decided units at the position of the growth*).
   No part of this step remains open.
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
4. **Contexts of growths in Lemma 5.** Lemma 5 bounds the evaluations that a
   step of a growth makes in its context outside the growths that it nests
   (*Calls in a context of growths*), shows that condition 5 makes the reads
   of the growths that nest at one position plain (*Plain reads*), bounds the
   steps of a growth of a leader inside a step of another one by the input
   that the outer growth consumes, apart from the growths at calls that are
   scanning sources (*Steps that begin anew*, *Growths inside a step*), and
   shows that these bounds do not rest on reuse in a context (*Reuse in a
   context*). It also relates two activations at one position whose innermost
   common evaluation is a growth, which the restarts of Lemma 4 leave out, as
   the restarts relate the others, unless the later one lies in a part that a
   step evaluates at the position of the growth without reading a growing
   result (*Repeats inside a growth*). It splits the sum over the positions
   for the activations that growth steps and evaluations in a context of
   growths own into the outer growths whose families hold one, which it
   charges to the heads of their keys, and the repeats inside one family
   (*Families of growths* to *Links lead to later growths*), bounds the
   repeats inside one family by a constant at each position (*Repeats inside
   a family*, *Consecutive repeats inside a family*), and bounds the heads
   whose family regions hold an activation at a position by the kinds of
   first activation that *First activations of heads* lists. What remains of
   that sum is described below.
   Lemma 4
   bounds the activations owned by memoized evaluations after this lemma, in
   the order that it states: the growth steps, the evaluations in a context of
   growths, the activations that they own and the successful activations that
   a dropped step at the position of its growth or a path that enters the growth again repeats must be bounded
   from the claims of Lemma 4 that count nothing, Lemmas 1 to 3, the memo and
   conditions 3 and 5, and not from the count of Lemma 4 on the activations
   owned by memoized evaluations. The claims that count nothing are *Failure
   after a source*, the restarts, *Direct activations*, *Repeats*, *Memoized
   links*, *Calls that call no rule*, *Nesting at one end*, *Seed reads*, *Own regions*, *Counterparts in the
   reference parse*, *Replays*, *Replayed activations*, *Answered calls*, *Covered keys*,
   *Steps with an empty seed*, *Reference links*, *Heads in the reference parse*, *Reference links lead to
   repeated evaluations*, *Links lead to repeated evaluations*, *Links to heads*,
   *Memoized links keep positions* and *Nothing in a memoized link is a head*,
   with the definitions between them; *Activations that do not succeed* and *Activations held by heads* come
   after this step. Three counts remain. First, the repeats inside one family,
   with no reuse in a context of growths. *Repeats inside a family* and
   *Consecutive repeats inside a family* bound the activations of a source at a
   position in the family region of one outer growth by a constant, apart from
   a constant for each step at the positions at most a constant after the
   growth, and leave the activations inside calls of rules that call no rule, the leaf
   links inside a family, with the open parts of step 1. Second, the heads of keys
   whose first activation at a position is of a kind that *First activations
   of heads* lists: the open parts of step 1, the activations that a dropped step at the position of its growth
   or a path that enters a growth again of the
   reference parse leads to,
   and the activations
   that *Heads hold what later growths hold* leaves out, through a result or
   step kept in a smaller lookahead state or the lookaheads that it observed,
   with the open gap stated with (D) of the memoization proof. Third, the growths that start at many positions, each at
   a call of a rule of the cycle that is not a left call or at a call of a
   leader in the body of another rule, as the repetitions of a scanning
   source. *Growths at many positions* bounds the steps of the outer growths
   by a constant times $K\hat{n}$ plus their successful growth activations,
   and with *Restarts of growth activations* and *Direct growth activations*
   gives these the restarts, *Direct activations* and *Repeats* of Lemma 4,
   apart from a constant times $K\hat{n}$ of them. *Second outer growths*
   matches the growth activations of the second outer growth of a key to those
   of the first, and places there the free activations whose reference link
   is the call that starts the growth itself and the growth activations that
   a path that enters the growth again repeats, in the first step of that
   growth as well, where the alternative reads the values of its variables
   that it read when the path left it. *Dropped steps* and *Restarts of growth activations* give a
   dropped step on the path of a restart, ordinary or of growth activations,
   a unit at the end of its seed or below it, and charge the repeats after a
   part that it evaluates at the position of its growth to the activations
   there. The claims from *Memoized links* to *Activations held by heads* take
   a step of a growth after the first as $w_i$, as a unit at the end of its
   seed, also when the seed is empty and the growth of the step is at the
   position of the link, since no shared call reads it (*Steps with an empty
   seed*). *Direct growth activations* takes the same units, and gives a
   shared call that is the call that starts the growth to the second outer
   growth of its key. The family claims apply to growth activations with the
   second outer growths left out, except *Consecutive repeats inside a
   family*, whose second evaluation of the shared call finds the growth of
   the earlier activation in the memo. *Answered places* takes the call that
   the memo answers there in place of the repeat and gives that claim's
   conclusion for growth activations, apart from the second step of a growth
   whose first result is empty, which the count takes with the first steps.
   Where the later activation lies in a later step of a growth that starts
   after the earlier activation, with the answered call in an earlier step,
   the pair lies in one step of the innermost growth that contains it, and
   the count charges it to the earlier activation as it does the first kind
   of *Repeats inside a family*.
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
8. **What the records leave.** The theorem assumes that the records are
   exact, (R1) of the [Setting](#setting), and excludes the suffix copies.
   Eight parts are open. First, (R2) and (R3) are shown only for the sites
   that *Sites that meet the conditions* in the [Setting](#setting) names,
   the run sites free of the environment in the parses of the backends of
   the engine, marked free where the program is loaded: for a site that is
   not free, (R2) asks for records under the values that its element reads,
   and for a repetition of a single character, for records that answer a
   walk from every position that an earlier walk passed, in every rule, and
   neither is shown of an implementation here, nor is (R2) for a run site in
   a generated Go or TypeScript parser, which keeps no record of it. Each of
   these is a source without records in the parses that walk it, under
   step 1 and the seventh part below. For a
   site that is not free,
   $K_R$ is not the $K$ of
   [Classification](../runtime/parsing.md#classification), which counts
   the combinations among the calls at one position and not among the visits
   of walks, so the theorem states its bound with the greater of the two.
   Second, the suffix copies of the walks that take records and whose
   iterations build values: they are at most the iterations that the walks which take the
   records would evaluate in their place, and no claim counts those. The
   parse that keeps no record of the site is not shown to make the same
   walks, since a call inside an iteration that a record answers stores
   nothing in the memo (*Sources without records* in Lemma 17), so the count
   of step 1 for the site in that parse would not give them as it stands.
   Third, the walks at which a Document
   resumes a run are walks without records, and so are the repetitions that
   are not restartable and that *Sites that are walked a constant number of
   times* does not cover; both stay under step 1. What else a parse that
   resumes a run leaves open is the eighth part. Fourth, the order of the
   bounds, which the theorem takes as a hypothesis (*Order* in Lemma 17):
   $B$, $A_u$ and $O$ must be bounded with no use of the iterations of the
   recorded walks, of the activations that do not succeed or of the
   evaluations of the rules that are not memoized. *The bound on the memoized evaluations counts
   no iterations* shows it of the bounds that Lemmas 3 and 5 state, and
   Lemma 13 and *Activations held by heads* state no such count; the proofs
   of the parts that steps 1, 3 and 4 leave open must keep the order. Fifth,
   the claims of Lemma 4 that count nothing in a parse that keeps records:
   *Counterparts in the reference parse*, *Replays* and *Replayed
   activations* are stated and proved for such a parse, outside the
   iterations of the walks of the sites that keep records, which by *Growths lie outside
   the recorded walks* covers every growth activation; *Restarts in a parse
   that keeps records* shows *Failure after a source*, *Restarts*,
   *Dropped steps*, *Operators Rules*, *Direct activations*, *Repeats* and
   *Memoized links* of such a parse; *Heads in a parse that keeps records*
   shows the claims from *Answered calls* to *Evaluated pairs of a shared
   call* of it, with one more case of the covered keys, an iteration that a
   record answers. That case ends the comparison of a link only when the
   activation of the link lies inside an iteration of a walk of a site that
   keeps records, which is the sixth part. For a free activation it can
   also arise on the path of the reference link, when the earlier
   evaluation of that link lies, in the reference parse, in an iteration
   that a record answers in the parse; *Covered keys through records*
   shows that the parse then made an evaluation of the class before, in an
   iteration of a walk that made a record, for records of run sites and of
   skip-run sites made as Skip Runs and Run Sites describe, (M1) and (M2)
   of that claim, which (R1) does not state; cases (a) and (b) are then
   read on the path followed through the records. *Leaf links in a parse
   that keeps records* shows *Growths around a leaf link*, *Calls that call
   no rule*, *Outer links across runs* and *Outer links inside a memoized
   link* of such a parse. *Later leaf links in a parse that keeps records*
   shows the other claims on leaf links, from *Memoized links to the frame
   of an outer link* to *Nesting at one end*, and the paragraph on the
   heads that hold the activations of leaf links, some of them with case
   (d), one with one more exception, some for an activation outside the
   iterations of the walks of the sites that keep records, and *Nesting
   at one end* with its hypothesis on the alike calls read in the
   reference parse for a call that a record covers, as it states; *Calls of scan-only rules*,
   which rests on the Run Sites proof, and *Seed reads*, which proves
   nothing, are not among them. *Repeats inside a growth in a parse that
   keeps records* shows the first case of *Repeats inside a growth* of
   Lemma 5, on which the claims on leaf links rest. *Families in a parse
   that keeps records* shows *Covered growths*, *Repeats inside a family*
   and *Links lead to later growths* of such a parse that resumes no run
   of a Document, and *Heads hold what
   later growths hold*, *Consecutive repeats inside a family* and *First
   activations of heads* for the activations that lie in no iteration of
   a walk of a site that keeps records, up to the claims at the position
   of a growth that the count after them uses. *Families without the
   records* bounds every activation that a growth step or an evaluation in
   a context of growths owns in such a parse that resumes no run of a
   Document, inside those iterations or not, by one of a parse that keeps
   no records, so that count, with the claims at the position of a
   growth, applies to them as in a parse without records, subject to the
   open parts of that count. *Activations that do
   not succeed in a parse that keeps records* shows that claim and Lemma
   10 of such a parse, and *Iterations at one scan in a parse that keeps
   records* shows 1 and 3 of *Iterations at one scan* and the first
   statement of 2 for every link, the rest of 2 for a link none of whose
   shared calls lies in an iteration that a record answers, and 4 for a
   run all of whose links are such. The claims that follow it, from *Memoized breaks* to
   *Consecutive activations in one own region*, and the
   other claims of Lemma 5, the bound of the second case of *Repeats
   inside a growth* and the claims at the position of a growth among them,
   have not been read again for a parse that keeps records, for the
   sources without records or for the growths. Sixth, a source without
   records inside the element of a site that keeps records, or in
   a rule that the
   element calls: a `#recover` whose skip is not reused, a walk
   that a Document resumes, a candidate site that is no recorded site of
   the parse, or, in a rule that the element calls, a repetition that is no
   candidate site or the operators of an Operators Rule that is the body of
   that rule (*Sources without records* in Lemma 17). *Replayed activations* does not hold of its
   activations inside the iterations of the walks of that site, where one
   evaluation of a class can take a record and another evaluate, so
   *Activations held by heads* does not bound them. *Descent in the
   reference parse* bounds the activations that memoized
   evaluations own, inside the iterations as outside and with (R1) alone,
   by six times the roots of their counterparts in $\mathcal{R}$: the
   first successful activation of the source at the position and those
   that have no reference link of a rule that the parse memoizes. For a
   source whose rule calls a rule or is memoized at its calls, in a
   reference parse in which no dropped step at the position of its
   growth, no path that enters a growth again and none of the cases that
   the restarts leave out leads to an activation at the position, that is
   six activations, whatever the slots hold and however the records are
   keyed. *Fresh roots* counts, of the roots, only those whose owner call
   is the first evaluation of its class in $\mathcal{R}$, six owned
   activations for each, so that what $\mathcal{R}$ evaluates again of a
   call or a growth whose key it has evaluated adds nothing. What is open
   in this count is the number of the classes, at the positions at or
   before the position of the scan, whose first evaluation in
   $\mathcal{R}$ holds outside its calls of rules that the parse memoizes
   a root that those cases make, and the number of such roots at one
   position in one such evaluation, which *Fresh roots in one evaluation*
   shows to be one beside those that case 5 leads to when the parse
   memoizes the rule of the source, that rule is no leader and the source
   is no bounded scan; the claims of Lemma 5 on the parts
   that a step evaluates at the position of its growth are not read for
   those evaluations of $\mathcal{R}$, and leave out steps in which such
   a part reaches a position far from its growth. *Heads without the
   records* bounds the same activations, inside the iterations as outside,
   by six times those that the heads hold in a parse that keeps no
   records, has no entries of an earlier parse and reuses nothing in a
   context of growths, so the leaf reference links, which make roots of
   their own, are counted as the leaf links of step 1 in such a parse and
   stay open with them, and the records add no open part to them; that
   the $K$ of that parse is at most the $K$ of the parse is not shown when
   the parse answers from entries of an earlier parse, resumes a run or
   keeps records not made as (M1) and (M2), and matters only for the class
   Linear given an environment bound. The
   activations inside the iterations that a growth step or an evaluation
   in a context of growths owns are bounded, with those outside them, by
   the count of Lemma 5 in a parse that keeps no records and reuses
   nothing in a context of growths (*Families without the records*), so
   what is open of them is what is open of that count in such a parse,
   when the parse resumes no run of a Document; in a parse that resumes
   one they are open, as the eighth part states.
   The same holds of the units of a copying operation that is a scanning
   source, in an action or a predicate of the element or of a rule that it
   calls, which Lemma 6 counts through the count of Lemma 4, and of the
   tries of an Operators Rule called there, which step 3 is to bound by its
   operators applied and its body evaluations.
   The list is that of the conditions of
   [Run Sites](../runtime/parsing.md#run-sites) as they stand. Were they to
   ask, of the element and of every rule that it calls, that each repetition
   without an upper bound be a run site, a skip-run site or a repetition of
   a single character, and that no `#recover` have a skip that is not
   reused, the sources of this part would be the candidate sites that are
   no recorded sites of the parse, which are the repetitions of a single
   character and the run sites of the first part, the operators of an
   Operators Rule in a rule that the element calls with its tries, the
   walks that a Document resumes, and the copying operations.
   (R2) does not bound them either. It counts the iterations that the walks
   of a recorded site evaluate, and not the input that a source inside
   those iterations examines. An activation of the source at a position $q$
   lies in an evaluated iteration of the site that starts at some
   $p \le q$, and two numbers are bounded for the parse and not for $q$.
   One is the number of the positions $p$ at which an evaluated iteration
   starts whose source has an activation at $q$: a record answers a walk
   only at a position where an iteration of an earlier walk started. The
   other is the number of the walks that evaluate the iteration at one $p$
   whatever its slot holds: a walk evaluates its first iterations up to the
   minimum of the repetition, a walk that ends plain its last iteration,
   and a walk made before the site turns hot all of its iterations
   ([Skip Runs](../runtime/parsing.md#skip-runs),
   [Run Sites](../runtime/parsing.md#run-sites)). So the records give no count at one position of the activations that
   the iterations hold, and what is proved of Lemma 4 gives one only as
   *Descent in the reference parse* and *Fresh roots* do: for the
   activations that memoized evaluations own, by the roots, with the
   parts stated above left open. Seventh, a
   candidate site that keeps records and is no recorded site of the parse,
   as a run site that is not free of the environment in the engine and a
   scan site in every implementation: its iterations are iterations of a
   walk of a site that keeps records, which *Replayed activations*, and so
   *Activations held by heads*, leaves out as it leaves out the activations
   of the sixth part, and the steps that its walks make outside their
   iterations, the lookups, the stores and the suffix copies, which (R3)
   would bound, are counted by no claim. Eighth, the parses that resume
   runs of a [Document](../runtime/documents.md). A walk that a Document
   resumes reuses recorded iterations, each an activation that makes no
   call, so the parse evaluates nothing inside them. *Counterparts in the
   reference parse* lists them among the evaluations of $\mathcal{R}$ that
   the parse does not make and carries its induction past them by [Lemma 3
   of the Documents proof](documents.md#lemma-3-resumed-runs) and its
   proof, whose
   outcome holds the recovered errors, none, since a run is recorded only
   when no recovered error remains at its end, and the farthest failure
   with its expected items ([Outcomes](memoization.md#setting) of the
   memoization proof), and whose examined range contains that of
   $\mathcal{R}$, which only the decisions to reuse read, given the
   invariant of the memoization theorem in this parse and in the parse
   that stored the record; and *Covered keys* has them as its case (e), which
   the claims that leave out case (d) leave out too. *Heads in a parse
   that resumes runs* shows where case (e) arises, as *Heads in a parse
   that keeps records* shows it for case (d), and the claims from *Links
   lead to repeated evaluations* to *Evaluated pairs of a shared call* of
   such a parse with case (e) beside case (d), and *Repeats inside a
   growth in a parse that resumes runs* shows the first case of *Repeats
   inside a growth* of Lemma 5 of such a parse, with records of sites or
   without, on which they rest where a link lies in a growth. It leaves a free activation
   whose reference link has case (e) on its path, when the earlier
   evaluation of the link lies in $\mathcal{R}$ in an iteration that a
   resumed walk reuses; *Covered keys through resumed runs* holds such an
   activation by no head unless case (a) or (b) arises on the path followed through
   the records and the resumed runs, an iteration that comes from an
   earlier parse being case (a), as an entry of an earlier parse is.
   *Replays* and *Replayed activations* are stated with the iterations of
   the walks of the repetitions, in their rules or in the twins, at which the
   parse resumes a run left out,
   and of the activations inside them those that memoized evaluations own
   are bounded by *Descent in the reference parse* and *Heads without the
   records*; *Answered calls* holds as stated, since a resumed walk makes
   no lookup and no store in the iterations that it reuses. *Restarts in
   a parse that resumes runs* shows the claims that *Restarts in a parse
   that keeps records* shows, from *Failure after a source* to *Memoized
   links*, of a parse that resumes runs, with records of sites or
   without, and *Activations that do not succeed* with and without
   records of sites. *Leaf links in a parse that resumes runs* shows the
   claims on leaf links that *Leaf links in a parse that keeps records*
   and *Later leaf links in a parse that keeps records* show, with case
   (e) beside case (d), with records of sites or without, and
   *Iterations at one scan in a parse that resumes runs* shows what
   *Iterations at one scan in a parse that keeps records* shows, under the
   hypothesis that no shared call of a link lies in a reused iteration
   either. Not shown of a parse that resumes runs, with records of sites
   or without, are what the fifth part leaves of the scans for a parse
   that keeps records: the rest of 2 of *Iterations at one scan* for a
   link that does not meet that hypothesis, 4 for a run that holds such a
   link, and the claims from *Memoized breaks* to *Consecutive activations
   in one own region*, which rest on 1 to 4 and are stated for a parse in
   which every walk evaluates its iterations. Nor are the bound of the
   second case of *Repeats inside a growth* and the claims of Lemma 5
   before *Calls in a context of growths*, on which it rests, read again
   for such a parse, with records of sites or without: a later step of a
   growth can resume a run that an earlier step of the same growth
   recorded at the same start, so that *Evaluations that read no growing
   result*, which has a part that reads no growing result evaluated alike
   in every step that reaches it, does not hold as it stands.
   *Activations held by heads* states its reading for a Document parse
   with *Heads in a parse that resumes runs* and *Covered keys through
   resumed runs*, and uses none of the claims on leaf links or on the
   scans. *Families in a parse that resumes runs* shows the claims of
   Lemma 5 from *Covered growths* to *First activations of heads* of a
   parse that resumes runs, with records of sites or without, for the
   activations that are no iteration of the kind and no resumable
   iteration and lie inside none, as *Families in a parse that keeps
   records* shows them with records; the two cases of the remark after
   that claim, a head that resumes a run whose walk a later growth of its
   key evaluates and the second growth of *Consecutive repeats inside a
   family* that resumes whole the run that the first recorded, are of
   activations inside resumable iterations; and the claim leaves out a
   free outside activation where the path from the counterpart of the
   head $h$ of that claim down to the earlier evaluation of its
   reference link passes an iteration that a resumed walk reuses. *Families without the
   records in a parse that resumes runs* bounds all the activations that
   growth steps and evaluations in a context of growths own in such a
   parse, those left out above included, by a constant times those of the
   parse $\mathcal{P}$ of that claim, which keeps no record and resumes no
   run and stores the results that read no growing result wherever the
   memo can answer them (a hypothesis of the claim, which the engine
   meets), subject to the open parts of the count of Lemma 5 in that
   parse. Its constant is a function of the grammar: a later call can
   evaluate what that parse answers from an entry stored inside an
   iteration that a resumed walk reuses, at most a constant number of
   times for each key.
