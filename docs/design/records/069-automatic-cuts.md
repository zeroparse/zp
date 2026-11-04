# 069. Automatic Cuts and Decided Units

- **Status**: Proposed
- **Author**: @ornew
- **Date**: 2026-11-04

## Summary

The linear-cost classifier compares the first characters of a unit with those of its continuations. Many units that it
cannot prove committing are not slow: two alternatives share a call such as an identifier, and part one or two symbols
later at a literal. This record proposes an analysis over **symbols** instead of characters, where a call of a
rule is one symbol, and two uses of it that keep the language unchanged:

1. **Automatic cuts.** When the analysis shows that no later alternative of an ordered choice can succeed where a prefix
   of an earlier one succeeded, the compiler inserts a `#cut` after that prefix. The choice matches what it matched
   before; a failure after the prefix no longer tries the later alternatives.
2. **Decided units.** The classifier accepts a unit as committing when it and each continuation evaluate the same
   symbols from one position and then part at a symbol that at most one of them can pass, which generalizes the ways of
   being committing of [Conditions for Linear Cost](../../../spec/runtime/parsing.md#conditions-for-linear-cost).

`/` stays the only choice operator, and the language does not change. The guarantee that a grammar is linear comes from
a declared linear mode, proposed separately, in which a choice that the analysis cannot decide is a compile error that
names the overlap; this record supplies the analysis that such a mode would run. Whitespace follows a convention, with a
lint, instead of a trivia declaration. A token layer is not introduced: on the bundled grammars, no overlap that the
measurements found needs one.

On the bundled grammars, counting a call as one symbol decides 61% of the alternatives at k = 1 and 65% within three
symbols. The nested reasons of the classifier fall from 1428 to 1394; on cel written with trailing whitespace, from 9 to
4. The grammars with automatic cuts give the same results and messages as the originals on all 446 test inputs. Most of
the units that stay not committing are options, iterations, lookaheads and operator parts, which a linear mode must also
cover.

## Motivation

The classifier of [record 064](064-grammar-complexity.md) reports about 1400 nested reasons in the bundled
grammars: a unit that is not committing against a continuation, nested without bound through rule calls. Grouped by
shape:

- **Shared calls followed by a literal**: `ident "(" args ")"` against `ident "." name`, or a message type, a call and a
  variable that begin with one dotted name.
- **Whitespace as a first character**: rules that begin with `ws` put the whitespace characters into the first set of
  every option and iteration that a `ws` follows, and no number of characters of lookahead sees past a run of
  whitespace.
- **Trailing separators**: an iteration `("," ws value)*` followed by an optional `("," ws)?`.
- **Keywords against identifiers**, and alternatives of one kind of lexeme, such as kinds of numbers.

The character-level first sets cannot separate the first shape: both alternatives begin with the characters of an
identifier. Yet a parsing expression evaluates deterministically: two evaluations of a call of one rule that reads no
variable, at one position, have one result. After the shared call, the two alternatives are at one position again, and a
literal decides between them. Counting a call as one symbol of lookahead makes the decision visible at depth two,
without a lexer.

The same observation gives a cut. In `ident "(" args ")" / ident "." name`, once `ident "("` has matched, the second
alternative cannot succeed at that position, so a failure inside `args` need not try it. A `#cut` after `"("` is a pure
optimization, and the compiler can place it where the analysis proves it.

## Goals

- Decide choices over symbols with a small bound k, accounting for variables, `#recover`, left recursion, nullable rules
  and lookaheads.
- Insert cuts that keep what every choice matches, with the differences that a cut can make to messages listed.
- Let the classifier use the same analysis, sound against Lemma 4 of the [linear-cost
  proof](../../../spec/proofs/linear-cost.md).
- Measure on the bundled grammars how many choices are decided at each k, and which reasons disappear.

## Non-Goals

- A new choice operator, a token layer, trivia declarations or lexer modes.
- The declared linear mode itself, which is proposed separately. This record names what it needs from the analysis.
- Longest match. A choice between `<` and `<=` is ordered by the author; linearity does not need the longest match, and
  a local convenience for it can come later.
- Cuts after a prefix of an iteration or an option; see [Iterations and Options](#iterations-and-options).

## Design

### Symbols and Paths

The analysis reads a parsing expression as a set of **paths**. A path is a sequence of symbols such that every
successful evaluation of the expression evaluates the symbols of one path in order, each succeeding, each starting where
the previous one ended. A symbol is one of:

- a **match**: a call, a literal, a character class, `.`, or a repetition with an upper bound other than 1, taken whole;
- a **negative** or **positive lookahead** `!K` or `&K`;
- a **skip**: an expression that consumes nothing and decides nothing for the analysis (a predicate, a cut, an anchor);
- a **stop**: an expression that the analysis does not look into, after which it decides nothing.

A sequence concatenates the paths of its items, a choice and an option collect the paths of their parts, and a
repetition with an upper bound of 1 is an option of its expression. The paths of an expression are a superset in this
sense: they contain one path for every successful evaluation, and may contain paths that no evaluation follows.
`#recover`, attributed expressions with an error or a missing value, and Operators Rules are stops. The number of paths
and their length are bounded (256 paths, 12 symbols), and a call is expanded into the paths of its rule body only as
deep as the comparison needs, three rule ends through a continuation.

### Exclusion

Two paths p and q from one position **part** at a decisive symbol when the analysis shows that p up to that symbol and q
cannot both succeed. It walks both from the start:

- **Equal symbols** advance both. Two calls of one rule are equal when the rule reads no variable and is not on a
  left-recursion cycle: a call inside a growth sees the seed of the growth, so two calls of a rule of a cycle at one
  position can differ. Equal literals, equal classes, `.` and structurally equal repetitions whose calls are equal in
  the same sense are equal too; repetitions are greedy, so equal repetitions end alike.
- **Disjoint first characters** decide: when neither symbol can match empty, and no character begins both, they cannot
  both succeed at one position.
- **Literals** decide when neither is a prefix of the other, and advance by the shorter one otherwise; a literal and a
  class or `.` that matches its first character advance by that character.
- **A negative lookahead** `!K` in p decides when the rest of q must match K at that position: q begins with K, or with
  an alternative of K. **A positive lookahead** `&K` in p decides when K and the next symbol of q are disjoint.
  Lookaheads in q decide symmetrically.
- **Expansion**: otherwise, a call or repetition on one side is replaced by its paths, the side whose callee reaches the
  other first, and each path must part.

k is the number of symbols that the decision examines: those passed alike and the decisive one. A choice is decided at k
when every alternative but the last parts from every later alternative on all pairs of paths within k symbols. `!kw
ident x / "in" x` is decided at k = 1, as is `"a" x / "b" x`; `ident "(" x / ident "." x` and `"=" x / "==" x` at k = 2.

The four hypotheses of the analysis are FIRST_k over symbols where a call is one symbol, exclusion by negative
lookahead, whitespace by convention, and no longest match. The measurements below test the first three.

### Automatic Cuts

For an alternative that parts from every later alternative of its choice, the decisive symbol lies in a top-level item
of the alternative. When that item is not the last, the compiler inserts `#cut` after it. The prototype inserts cuts
only into choices whose alternatives are sequences and only where the alternative has no cut of its own.

**Same result as ordered choice.** Let alternative a_i of a choice evaluated at position s have the prefix P, which ends
with the decisive item, and the rest R. If P fails at s, the cut is not reached, and the choice tries the later
alternatives as before. If P succeeds, every path of P that a successful evaluation follows parts from every path of
every later alternative, so no later alternative succeeds at s. If R then succeeds, the choice returns a_i as before; if
R fails, without the cut the choice tries the later alternatives, which all fail, and fails; with the cut it fails at
once. The match and the value are the same.

What can differ is what the failed alternatives would have recorded:

- **Expectations.** The later alternatives are no longer tried, so their failures do not contribute to the farthest
  failure. When one of them would have failed farther than the rest of the parse, the message reports a nearer position
  or other expected items.
- **Errors in abandoned branches.** An action that raises a run-time error in a later alternative that fails anyway is
  no longer run.
- **Recovery.** A later alternative that contains `#recover` would succeed by recovering; such alternatives are stops,
  so the analysis does not decide against them.

### Iterations and Options

A cut inside an iteration or an option commits the enclosing repetition or option, and a cut there does not keep the
language. In `"[" elems? ","? "]"` with `elems = value ("," value)*`, a cut after `","` in the iteration of `elems`
makes `[1,]` fail: the iteration fails after the cut, and the repetition fails with it, where without the cut it stops
before the `,` that `","?` then matches. Automatic cuts are therefore inserted into alternatives of choices only.

### Decided Units in the Classifier

The classifier uses the analysis in two ways.

**Cut prefixes.** An alternative of a choice that is not growing, with a `#cut` at the top level of its sequence after
items that can fail, is a unit of those items alone. A failure after the cut makes the choice fail, so its later
alternatives do not run after it. This holds for cuts that the author wrote and for automatic cuts.

**Decided units.** A unit that the classifier does not find committing by the existing cases is committing when the
strict form of the analysis decides it against each of its continuations within k symbols. The strict form adds what
makes a decision a bound on the input that the continuation examines again:

- Every symbol passed alike is **cheap**: a call answered by the memo, a call of a rule that calls no rule, an
  expression without a scanning source, or a call of a rule whose body is cheap in all its parts. The continuation
  examines it again at no more than a constant or what the unit paid.
- The analysis looks into an option, a choice, a repetition or a lookahead only when all its parts are cheap: a path
  does not show the attempts that fail inside them, which can examine input.
- A unit whose paths end with every symbol passed alike or cheap is decided too: it examines nothing beyond shared and
  cheap symbols, which generalizes the case of no source after the shared calls.
- Lookahead units are not decided this way.

### Whitespace

Whitespace is handled by convention: a lexeme rule consumes the whitespace that follows it, and no rule other than the
start rule begins with whitespace. Then every unit begins with a symbol of the language, and the analysis compares
lexemes. With leading whitespace, a continuation that begins with `ws` meets a unit that ends with `ws` before its next
lexeme, and the analysis must compare `ws ws` with `ws`, which no bounded comparison decides.

A lint reports a whitespace rule in the first set of a unit or a continuation: a rule that can match empty, matches only
whitespace characters or comments, and is called at the start of a rule body or of an Operators Rule skip. The lint
names the rule that begins with it and suggests the trailing convention.

### Linear Mode

A declared linear mode, proposed separately, turns the classification into a guarantee: in that mode a choice that the
analysis cannot decide is a compile error that names the two alternatives, the paths that overlap and the symbol where
the comparison stopped. The automatic cut and the error then come from the same decision. The measurements show that
alternatives are a minority of the units that are not committing; options, iterations, lookaheads and operator parts are
the majority, so a linear mode needs a rule for them as well (see [Questions for the Owner](#questions-for-the-owner)).

## Soundness

This section argues the two uses against [Lemma 4](../../../spec/proofs/linear-cost.md#lemma-4-scanned-cost) and states
the gaps. The automatic cut needs only the exclusion; the classifier needs the exclusion and the cost bound.

**Determinism of equal symbols.** A parsing expression evaluated at one position with one environment and one memo state
of growths has one result. Equal calls read no variable and are not calls of rules of a cycle, so their results at one
position agree; equal literals, classes and `.` examine the same characters; equal repetitions of such symbols iterate
alike, since they are greedy. Two paths that pass equal symbols from one position are therefore at one position after
each of them, whether the symbols succeed or fail.

**Exclusion.** Each decisive case is a statement about two expressions at one position: disjoint first characters cannot
both be consumed; two literals that are not prefixes of each other cannot both match; a negative lookahead `!K` fails
wherever q's rest matches K, and q's rest must match K to succeed; a positive lookahead `&K` succeeds only where K
matches, and K and q's next symbol cannot both match there. Expansion replaces a symbol by the paths of the expression
it stands for, which is a superset, so a decision on every expanded path is a decision on every evaluation. With the
superset property of paths, the automatic cut keeps the result by the argument of [Automatic Cuts](#automatic-cuts).

**Cut prefixes against the restarts.** The restarts of Lemma 4 take the first abandoned evaluation w on the path from N
down to α, and use that w's parent goes on after it, so w is a dropped step or a unit. An alternative with a top-level
cut that fails after the cut makes its choice fail, so its parent does not go on after it, and w is not that
alternative. When it fails before the cut, the input it examined is what its prefix examined, and the prefix is the unit
that condition 2 checks. Growing alternatives are excluded, as the growth reenters them.

**Decided units against the cases of Lemma 4.** Let a unit U and a continuation C, at one position, pass equal symbols
and then part at a decisive symbol. Up to the decisive symbol, C evaluates again what U evaluated, and every such symbol
is cheap: a memoized call costs C a memo access (case 2), a call of a rule that calls no rule repeats only its own scan,
which U paid for (case 2), and an expression without a scanning source is bounded by Lemma 1 (case 3). At the decisive
symbol, at most one of U and C passes it. If U failed at or before it, U examined the shared symbols and a bounded
amount more. If U passed it and failed later, C fails at the decisive symbol, having examined the shared symbols and the
decisive one, which is a literal, a class, or a symbol whose first character decides (case 1 at a later position), or a
lookahead whose expression C must match, so that C fails within that match. Either way nothing that the parser evaluates
next starts again inside the input that U examined beyond its shared and cheap symbols. A unit that is exhausted after
shared and cheap symbols examines nothing more, which is case 3.

**Gaps.**

- The argument is a sketch. It needs a formal statement of the path superset property, of the strict cheap condition as
  a cost bound, and of each decisive case as a lemma, as cases 1 to 5 have.
- The cost of a lookahead decision: when C fails inside the match of K, it may have examined as much as K examines. The
  prototype requires K to be cheap in all its parts in strict mode; the proof must state that bound.
- Rules that read variables are never equal symbols. Two calls with one environment have one result, and the memo is
  keyed by environment, so equality of calls whose arguments are equal is sound in principle but not implemented.
- Calls of left-recursive rules are never equal symbols, and the growth argument of case 5 is not extended.
- `#recover`, attributed expressions with an error or a missing value, and Operators Rules are stops; operator parts are
  not analyzed.
- A decision that lies inside a callee cannot become a cut, since a cut does not reach the caller's choice. The
  classifier still uses it.
- Cuts after a prefix of an iteration or an option do not keep the language.
- The proof depends on the open count of activations of Lemma 4, as the rest of it does.

## Measurements

The prototype is `internal/compile/autocut.go` on the branch `feat/auto-cut`, reached only from tests.
`TestAutoCutReport` classifies each bundled grammar with the analysis off and on and writes the grammars with automatic
cuts. Counts are of choices with two or more alternatives in reachable rules, of their alternatives but the last, and of
the witnesses (w) and nested reasons (r) of the classifier.

**Decisions.** A choice is decided at k when all its alternatives but the last are; cuts are the alternatives that get a
`#cut` when decisions up to k = 5 are used.

| Grammar | Choices | k ≤ 1 | k ≤ 2 | k ≤ 3 | k ≤ 5 | Alternatives | k ≤ 1 | k ≤ 2 | k ≤ 3 | k ≤ 5 | Cuts |
|:--|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| cel | 42 | 27 | 29 | 29 | 29 | 96 | 73 | 77 | 77 | 77 | 13 |
| csv | 2 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 |
| cue | 138 | 59 | 63 | 63 | 63 | 318 | 146 | 153 | 154 | 154 | 39 |
| duckdb | 565 | 341 | 364 | 371 | 399 | 1303 | 826 | 870 | 890 | 930 | 472 |
| golang | 166 | 68 | 74 | 76 | 77 | 350 | 208 | 217 | 221 | 223 | 63 |
| python | 153 | 71 | 75 | 75 | 75 | 344 | 202 | 214 | 214 | 214 | 60 |
| typescript | 302 | 162 | 174 | 174 | 174 | 808 | 553 | 571 | 571 | 571 | 96 |
| xml | 35 | 27 | 30 | 31 | 33 | 54 | 43 | 46 | 48 | 50 | 8 |
| yaml | 91 | 24 | 24 | 24 | 24 | 130 | 33 | 33 | 33 | 33 | 10 |
| minilang | 10 | 8 | 8 | 8 | 8 | 27 | 19 | 19 | 20 | 20 | 0 |
| outline | 1 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 |
| json | 5 | 5 | 5 | 5 | 5 | 9 | 9 | 9 | 9 | 9 | 0 |
| Total | 1510 | 792 | 846 | 856 | 887 | 3443 | 2112 | 2209 | 2237 | 2281 | 761 |

Counting a call as one symbol decides 61% of the alternatives at k = 1 and 65% at k = 3; beyond k = 3 only duckdb and
xml gain more than a few. The alternatives that stay undecided share an unbounded prefix (two Go declarations that both
begin with a list of identifiers), separate keywords from identifiers with predicates or variables (TypeScript's
reserved words), decide inside a callee (an alternative that is one call), or overlap (an integer and a decimal both
match `123`).

**Classifier.** Witnesses (w) and nested reasons (r). Automatic cuts are classified with cut prefixes as units; decided
units use the strict analysis at k = 2, 3 and 4.

| Grammar | Class | Base | Automatic cuts | Decided k = 2 | k = 3 | k = 4 | Both, k = 3 |
|:--|:--|:--|:--|--:|--:|--:|:--|
| cel | polynomial | w8 r12 | w8 r12 | r12 | r12 | r12 | w7 r12 |
| cue | exponential | w52 r59 | w52 r59 | r59 | r59 | r59 | w51 r59 |
| duckdb | polynomial | w19 r672 | w19 r663 | r663 | r656 | r647 | w19 r655 |
| golang | polynomial | w24 r165 | w24 r165 | r155 | r153 | r149 | w24 r153 |
| python | exponential | w147 r199 | w148 r199 | r199 | r199 | r199 | w147 r199 |
| typescript | polynomial | w241 r269 | w241 r269 | r266 | r266 | r256 | w241 r266 |
| xml | polynomial | w5 r9 | w5 r9 | r7 | r7 | r7 | w4 r7 |
| Total of all twelve | | w533 r1428 | w534 r1419 | r1404 | r1395 | r1372 | w530 r1394 |

csv, yaml, minilang, outline and json do not change; json is Linear throughout. The class of no grammar changes.
Automatic cuts alone remove few reasons, because most decided alternatives were committing already by their first
characters; the cut matters where the decision comes after a shared call. In python one witness moves: an n^4 witness
becomes n^3 and a new n^2 witness appears, so the count rises by one while the worst degree falls. The units that stay
not committing are mostly not alternatives: with decided units on, about a third are alternatives of choices, and the
rest are options, iterations, lookaheads and operator parts.

**Whitespace.** The cel grammar begins its rules with `ws`. Three scratch variants keep its language: lexemes that
consume their trailing whitespace instead, the same with one rule per kind of lexeme, and the trailing form with the
digit rules memoized.

| Variant | Base | Automatic cuts | Decided k = 3 | Both, k = 3 |
|:--|:--|:--|:--|:--|
| `cel.zp`, leading whitespace | w8 r12 | w8 r12 | w7 r12 | w7 r12 |
| Trailing whitespace | w7 r9 | w6 r7 | w7 r6 | w6 r4 |
| Trailing, one rule per lexeme | w8 r8 | w8 r7 | w8 r5 | w8 r4 |
| Trailing, memoized digit rules | w9 r9 | w8 r7 | w9 r6 | w8 r4 |

With leading whitespace the analysis gains nothing on cel: every comparison meets `ws ws` against `ws`. With trailing
whitespace, the decided units remove the trailing separators and the analysis halves the reasons. The four that remain
are an overlap of `-` as negation and as the sign of a literal, the `.` selection against the `.` call among the
operator parts of a postfix level, which the analysis does not look into, and two alternatives against which a message
type is not decided, because a positive lookahead scans the dotted name before them. Each needs factoring in the
grammar; a token layer would need the same factoring, since the overlaps are between sequences of lexemes, not inside
one.

**Parity.** Each grammar with automatic cuts against the original, on the test inputs of the grammar, each file whole
and each line alone: 446 inputs over the twelve grammars and the three cel variants, 208 accepted and 238 rejected, with
the same exit status, value and messages for every one.

## Alternatives Considered

- **A token layer.** Lexical declarations (`token`, `fragment`, `trivia`) matched longest-first, in modes that the
  parser selects, with first sets over token kinds. It separates keywords from identifiers and removes whitespace from
  first sets by construction. It adds a second layer to the language, its own memo, a new lemma, declarations for every
  literal, and a migration of every grammar. Modes are needed only for sets of tokens matched by longest match, which
  linearity does not require. On the bundled grammars, every overlap that it removes is also removed by the symbol
  analysis with the trailing whitespace convention or by factoring: the kinds of numbers by one memoized rule per
  lexeme, the dotted names of a message, a call and a variable by reading the name once. No measured case needs tokens.
- **A deterministic choice operator `|`.** The compiler checks its alternatives disjoint and the parser never tries
  another. It gives the author a construct with a local error, but it is a second choice operator with the same matches
  as `/` wherever it is accepted. The automatic cut gives the same run-time commitment without a new operator, and the
  linear mode gives the error.
- **Character first sets with k = 2.** It separates `r"` from `r` and `<` from `<=`, squares the sets, and does not see
  past names or whitespace.
- **LL(*) prediction.** Unbounded lookahead decides more overlaps, at the cost of prediction scans that can be as long
  as the input at every decision.
- **Cuts placed by the author only.** The compiler would not check that a cut keeps the language, and every cut is
  placed by hand.
- **Classifier refinement by shape.** Refinements per shape removed about 8% of the reasons at 3.5 times the classifier
  time. The symbol analysis is one mechanism for the shapes of the first kind.

## Testing

- `TestDecideAlt` checks the decision, k and cut position on choices that cover each decisive case and the undecided
  ones: a literal that is a prefix of the next alternative's, a repetition in the later alternative, a lookahead whose
  expression the later alternative does not begin with.
- `TestDecidedUnits` checks a trailing separator classified Linear with decided units, and the same shape with a scan
  between the separator and the whitespace still unknown.
- Parity: each grammar with automatic cuts against the original on the test inputs of the grammar, each file whole and
  each line alone, comparing the exit status, the printed value and the messages.

## Limitations and Open Questions

- The measurements are counts of the classifier; they are not soundness claims for the prototype.
- The prototype bounds paths and expansions by constants chosen for the measurements; the compiler needs bounds that are
  part of a contract or a diagnostic when they are reached.
- See [Soundness](#soundness) for the gaps of the proof.

### Questions for the Owner

1. **Units other than alternatives in the linear mode.** Options, iterations, lookaheads and operator parts are most of
   the units that are not committing.
   - Apply the strict analysis to every kind of unit, with a compile error naming the unit, its continuation and where
     the comparison stopped: one rule, and the guarantee covers what Lemma 4 needs.
   - Check only choices in the mode, and leave the other units to the classifier: simpler, but the mode no longer
     guarantees linearity.
   - A separate construct for the common iteration shapes, such as a separated list, with the analysis for the rest.

   Recommendation: the first, with the separated list as a convenience where it is accepted separately.
2. **The bound k.**
   - k = 3: covers what the bundled grammars gain, except a few duckdb and xml choices, with small comparisons and
     readable errors.
   - k = 2: smaller, and loses the decisions through a rule body.
   - A pragma per choice to raise it: an escape for the rare deep case, at the cost of another construct.

   Recommendation: k = 3 fixed, and revisit when a grammar needs more.
3. **Messages under automatic cuts.** A later alternative that is no longer tried does not add to the farthest failure.
   - Accept it: the position and expected items can differ when a later alternative would have failed farther; none of
     the 446 test inputs differ.
   - Record the expectations of the skipped alternatives without running them: the first symbols are known from the
     analysis, at a cost in the compiler.
   - Insert cuts only when messages are not requested: two parsers per grammar.

   Recommendation: accept it, and record first-symbol expectations later if a message regresses.
4. **Decisions inside a callee.** A cut is local to its rule, so an alternative that is one call cannot be cut where its
   callee decides.
   - Classifier only: the classifier counts the decision, and the run time keeps backtracking, which is linear in that
     case.
   - Inline small callees into the choice before placing cuts.
   - A cut that commits the caller's choice: a language change.

   Recommendation: classifier only.
5. **How the whitespace lint finds whitespace.**
   - By shape: a rule that can match empty and matches only whitespace characters or comments.
   - By the `skip` of the Operators Rules and the rules it calls.
   - By an attribute on the rule.

   Recommendation: by shape, with the `skip` of Operators Rules as a seed, and no new attribute.
6. **Action errors in abandoned alternatives.** An action that raises a run-time error in a later alternative that fails
   anyway is no longer run under an automatic cut.
   - Accept it as part of the optimization.
   - Do not cut before alternatives whose actions can raise errors.

   Recommendation: accept it; such an error in a failing alternative is not a result of the parse.

## Literature

Citations are from memory and are not verified against the publications.

- B. Ford. Parsing Expression Grammars: A Recognition-Based Syntactic Foundation. POPL 2004.
- K. Mizushima, A. Maeda, Y. Yamaguchi. Packrat Parsers Can Handle Practical Grammars in Mostly Constant Space. PASTE
  2010. Cuts inserted automatically where first sets are disjoint.
- R. R. Redziejowski. Cut points in PEG. Fundamenta Informaticae, 2016. Points after which an alternative's failure
  cannot be followed by a success of another.
- T. Parr, K. Fisher. LL(*): The Foundation of the ANTLR Parser Generator. PLDI 2011.
- T. Reps. "Maximal-Munch" Tokenization in Linear Time. ACM TOPLAS 20(2), 1998.
