# 068. Separated Lists

- **Status**: Accepted
- **Author**: @ornew
- **Date**: 2026-11-04

## Summary

A grammar writes a separated list by hand: `e (-"," e)*`, often with an optional trailing separator
`e (-"," e)* -","?`. The hand-written form hides two facts from the classifier and the engine: after a separator the
list is certain to continue with an element, and a trailing separator is the only place where the list may stop right
after a separator. Without these facts the iteration `(-"," e)` is a unit that can fail after it has consumed its
separator, so it is not committing, and the continuation of the list can restart inside the input it examined.

This record adds a **separated list** as a postfix pragma on a repetition, `e+ #sep(",")`, with a trailing-separator
policy of `forbidden` (the default), `optional` or `required`. The list compiles to a loop that **commits after each
separator** (under the `forbidden` policy) or that decides, by a fixed rule, whether the separator belongs to the
list (under the other two). The value is a list of the element values; the separators' values are dropped. The
classifier treats the loop as committing by construction, which is a new case of the committing cases of the cost
proof. The pragma replaces 60 of the iteration reasons and 18 of the option reasons that the classifier reports in the
bundled grammars, and changes no class by itself.

## Motivation

The classifier reports a unit as nested when its continuation restarts inside the input the unit examined while a scan
reachable from it can run again through a recursion. In the bundled grammars most of these reasons sit on repetitions
and options of the shape that follows a separator:

```
args = e (-"," e)*            // the iteration (-"," e) consumes "," and may then fail
args = e (-"," e)* -","?      // the same, with a trailing separator
```

When `e` fails after the `","` of an iteration, the repetition stops and the enclosing sequence continues at the
position before the `","`. The classifier cannot see that the continuation then fails at the same character: the
iteration is a unit that can fail after a source (the separator and the start of `e`) and its first characters are not
disjoint from those of its continuation, since the continuation may be the trailing `","?` or a closing token that the
element's first set also contains. So it reports the iteration, and the option `","?`, as non-committing.

Counting the reasons whose unit is a separator-shaped iteration (`(-s e)*` or `(s e)*` over a single-token separator)
or the trailing option of such a list, over the 8 large bundled grammars (`runSitesOn` off, about 1,010 nested reasons
in total):

| Grammar | Separator iterations | Trailing options on those lists |
|---|---|---|
| cel | 3 | 2 |
| duckdb | 22 | 1 |
| golang | 10 | 1 |
| python | 15 | 2 |
| typescript | 10 | 12 |
| cue, xml, yaml, csv, json | 0 | 0 |
| Total | 60 | 18 |

That is 78 of about 1,010 reasons (about 7.7%). The remaining reasons come from operator loops, scans over
expression-shaped elements and copies, which a separated list does not touch. No grammar changes class from this record
alone. The value is a precise, cheap and local reason removal; a list is also the most common repeated shape in
grammars, and giving it a name gives the engine and the error reports the same facts.

## Design

### Syntax options

The language already has three ways to attach behaviour to a parser expression, and they differ in what they can
express.

**A. A postfix pragma on a repetition** (recommended): `e+ #sep(",")`, `e* #sep(",", trailing=optional)`. Pragmas are
postfix modifiers on the immediately preceding expression; `#repeat(n, m)` takes positional bounds and
`#recover(skip=e)` takes a named argument, so `#sep(s)` with a positional separator and a named `trailing` argument
matches both. The repetition already says how many elements are allowed (`+`, `*`, `#repeat`), so the pragma only adds
what separates them. Pragma modifiers are where the language puts attributes of an expression that the plain
operators cannot carry, and a classifier or generator reads the pragma without parsing a pattern.

**B. A function-like built-in**: `list(e, ",")`. Functions such as `list`, `concat`, `map` and `foldr` exist only in
action terms and compute values; the same spelling in a parser expression would read as a value function but behave as
a parser. This is a semantic inconsistency, and it would need a second function-call grammar in parser expressions.
Rejected.

**C. A new operator**: `e % ","` (as in some PEG dialects). It is short, but it adds a precedence level to the
parser-expression grammar, offers no place for the trailing policy without a second operator, and breaks the rule that
modifiers of an expression are postfix pragmas. Rejected.

**D. A repetition modifier**: `e+ sep ","`, as a keyword. It adds a keyword to the language for a single feature and
still needs a spelling for the policy. Rejected.

With A, the forms are:

| Written | Matches | Trailing separator |
|---|---|---|
| `e* #sep(",")` | zero or more elements separated by `","` | not allowed |
| `e+ #sep(",")` | one or more elements | not allowed |
| `e* #sep(",", trailing=optional)` | zero or more | allowed after the last element |
| `e+ #sep(",", trailing=required)` | one or more | required after every element, including the last |
| `e? #sep(",")` | rejected: a separator needs a repetition | |

Bounds compose: `e #repeat(2, 5) #sep(",")` accepts two to five elements. The separator is any parser expression
without captures. A multi-token separator such as `ws "," ws` is allowed; the commit point is after the separator
expression has matched in full.

The `trailing=required` policy is the terminated list (`stmt ";"`) of statement blocks. It is listed with the other two
so that the trailing-separator reasons caused by statement lists are covered; it does not need extra machinery (see
Compilation).

### Value shape

The value of `e+ #sep(s)` is the list node a repetition of `e` produces: a list of the element values in order. The
separator's value is dropped, as `-s` drops it. A grammar that needs the separators writes the repetition by hand.
Nothing changes for the type system: the type of `e* #sep(s)` is the type of `e*`, a list of the element type, and
`node[T]` elements keep their static types. Element values are not tupled with separators, so the common case needs no
`-` and no projection.

If an element is itself a sequence whose value is a tuple, the list element is that tuple, as in `(a b)*`.

### Compilation: commit after each separator

For the `forbidden` policy the list compiles to

```
list  = e (sep e)*          with a cut after each sep
```

that is: match `e`; then loop: try `sep`; if it fails, stop; if it matches, the next `e` must match, and if it fails
the list fails at the position where that `e` failed (the farthest failure is the failure of `e`, not the end of the
separator). There is no backtracking to the position before the separator, because the language of the list makes that
position unreachable as a successful end: a separator without an element after it is not a list. This is a *local
commit*: the cut is scoped to the list and to the iteration, like a cut in a repetition element, and it never escapes
the list (no enclosing choice is committed). A choice outside the list still tries its next alternative when the list
fails, exactly as for a failing mandatory element.

For `trailing=optional` the iteration cannot commit at the separator, since `e` may legitimately be absent. The list
instead decides at the separator whether an element follows by **one token of lookahead on the first set of `e`**:

```
list  = e (sep &first(e) e)* sep?
```

where `&first(e)` stands for a predicate generated from the first characters of `e` (the classifier already computes
them). If `e` can start with the empty string, or the first set is not a set of characters, the pragma is rejected at
compile time with a message that names the rule (an element that can match the empty input is also rejected by the
repetition rule today). If the first set matches but `e` then fails, the list fails with the failure of `e` (the commit
holds once the lookahead has passed). If it does not match, the separator is the trailing separator and the list
ends after it. With this rule the trailing separator is accepted exactly when it is followed by something that cannot
start an element, which is the common reading ("a comma followed by `)` is trailing"). A grammar that wants a
different reading (a trailing comma only before `)`) writes the hand form.

For `trailing=required` the list is `(e sep)+` (or `(e sep)*`). An iteration that fails at `e` ends the list; an
iteration that matches `e` and then fails at `sep` fails the list, since a missing terminator is an error and not the
end of the list. The commit point is after `e`.

The loop keeps the repetition rule: an iteration that consumes no input ends the list. Under `forbidden`, with an
element that can match empty, the iteration `sep e` always consumes the separator, so only the first element can be
empty, and the repetition rule applies to the first only.

### Errors and expectations

- When `e` fails after `sep`, the reported failure is the farthest failure of `e` at the position after the
  separator; the expected set is the first set of `e`. The user sees "expected expression" after `","`, not
  "expected `)` or `,`", because the alternative of ending the list there does not exist.
- When `sep` fails after an element, the list ends successfully; the farthest failure includes `sep` as an expected
  item as a repetition's stop does today, so a later failure reports `","` among the expectations.
- With `trailing=optional`, a failure after the separator at a position where the first set matches reports `e`'s
  failure.
- `#missing`, `#error`, `#diagnostic` in the element behave as in a repetition.

### Interaction with other language features

**`skip`.** The skip of the grammar applies between the element and the separator and between the separator and the
next element, exactly as in the hand-written sequence `e (sep e)*`; the list does not introduce a new place for
trivia. The commit point is after the separator and before the skip that follows it: the skip runs, then `e` must
match. The separator expression may include its own `skip` handling; `skip` in an Operators Rule keeps its meaning
inside an element.

**`#recover`.** `#recover(skip=x)` on an element works as in the repetition: a failing element recovers inside the
element, so the iteration still counts as matched and the list continues. With the commit-after-separator rule,
recovery inside an element does not cross the separator: a recovery that consumes a separator would hide the
structure of the list, and the recovery expression `x` of an element normally stops at a separator or closer. A
`#recover` on the list itself (`e* #sep(",") #recover(skip=x)`) recovers the whole list at its start; it is accepted
as for the repetition.

**Streams.** `#stream` is allowed on a top-level repetition of the start rule. `rows = row* #sep("\n") #stream`
streams the elements: each element is delivered when it and its following separator have been matched under `trailing`
`forbidden` (the element is final when the separator after it is seen or the input ends), or when the terminator has
been matched under `required`. The stream cannot combine with `#recover` for the same reasons as the repetition. A list
that is not the top-level repetition of the start rule cannot stream.

**Documents.** Incremental reparse treats the list as a repetition with a separator: the examined range of each
iteration covers the separator and the element, and a change inside one element invalidates that iteration and, if the
change touches the separator or the lookahead token of `trailing=optional`, the next iteration. Run records
(record 067) apply to a list the way they apply to a repetition; the separator is part of the keyed iteration.

**Operators Rules in elements.** An element may be an Operators Rule call (`args = expr* #sep(",")`). The list adds no
operator semantics, so the element sees its own levels and its `skip`. A comma used as a binary operator at a level
inside the element, as in `a, b` of a sequence expression, still lets the element consume the comma; the author sees the
same ambiguity as with the hand form, and an element that can consume the separator makes the list reject the commit
guarantee (classifier rule below).

**Lookahead and predicates.** A predicate in the element or in the separator is allowed. A predicate between the
separator and the element would be a different list; it belongs to the element. A list inside `&` or `!` is allowed and
commits locally. The cut of the list does not leak outside `&` and `!`, as for any cut inside a predicate.

**Cut and atomic.** `#cut` inside the element keeps its meaning. `#atomic` on the list turns off skip between its
parts; the commit point is unchanged.

### Classifier rule

The classifier treats a separated list as a **committing loop** under the following conditions, checked per list:

1. **The element does not begin with the separator.** The first characters of `e` are disjoint from the first
   characters of `sep` (the same disjointness test as for the other committing cases). This makes the decision at each
   iteration depend on the token after the previous element: the separator, or anything else.
2. **The element cannot consume the separator at its start.** No path of `e` matches a prefix that the separator also
   matches at the position after `sep`. Without this, `a , , b` would parse the second `,` as the start of an element.
   Condition 1 gives it for single-token separators; for multi-token separators it requires the first set test above.
3. **The commit holds for the policy.** Under `forbidden` and `required` there is nothing more to check. Under
   `optional` the first set of `e` is a set of characters (the pragma is rejected otherwise).

When these hold, the iteration unit has no failure that the continuation can restart from: once the separator has
matched, the iteration either completes or the whole list fails. The list is counted as a **committing unit with
no restart**, as are the other cases of the cost proof. When a condition does not hold, the compiler rejects the
pragma.

### Proof sketch (plugs into Lemma 4)

Lemma 4 bounds the work of a unit that restarts at a position inside the input a previous run of the unit examined,
by showing that the unit fails or succeeds once per position. The committing cases of the lemma are: disjoint first
characters, shared calls, no source after the shared calls, words that exclude and paths that enter the growth again.
A separated list adds a sixth case, **a loop that commits after its separator**.

Claim. Let `L = e (sep e)*` be a separated list with the commit rule, and `U = sep e` the iteration unit. Let `C` be the
continuation of `L` in a sequence. Then every attempt of `U` at a position `p` either fails at `p` without examining
beyond the separator's own first set, or commits and succeeds or fails the whole list without returning to `p`.

Sketch.

1. *Shape of the attempt.* An attempt of `U` at `p` first attempts `sep` at `p`. If `sep` fails, `U` fails having
   examined only the characters `sep` examined. If `sep` matches, the commit rule applies: `e` is attempted at `q`, the
   end of `sep`. If `e` matches, `U` succeeds. If it fails, the list fails; there is no result at `p`, so `C` is not
   restarted at `p`.
2. *No restart at p after the separator matched.* The hand form returns to `p` after `e` fails and then attempts
   `C` at `p`. Because the list returns failure instead, the continuation `C` is attempted at `p` only when `sep`
   itself did not match at `p`. In that case `U` examined the characters of `sep`'s failure only.
3. *Restart count.* The restart of `C` at `p` after a failed `U` costs the work of `C` at `p`. Under condition 1 and 2,
   `sep` fails at `p` exactly when the character at `p` is not in the first set of `sep`, which is the
   disjoint-first-characters test of the other committing cases and costs a constant per `(unit, position)` pair, so the number of
   restarts of `C` charged to `U` is one per position at most. Under `trailing=optional`, the lookahead fails
   exactly when the character at `q` is not in the first set of `e`, bounded the same way; the trailing separator is
   then matched and `C` is attempted once at the end of the separator.
4. *No unit nests.* Because a failure of `e` after `sep` ends the list, the separator cannot be examined twice by
   different restarts: it is examined at `p` once per attempt of `U`, and an attempt of `U` happens at most once per
   iteration position of `L`.

The remaining obligations are the usual ones of the cost proof: the tally of examined characters across attempts (Lemma
5) and the interaction with an outer choice. The latter is not affected, because an outer choice that fails after the
list has failed restarts the outer alternative at its own start, which is charged to the outer unit as for any failing
mandatory element. This sketch does not replace the proof text; the text of Lemmas 4 and 5 is edited elsewhere and the
new case is to be written there once this design is accepted. Condition 2 is the premise the proof leans on and the one
most in need of independent review, since an element that can start with the separator makes the commit unsound: the
hand form would accept `a,,b` as `a`, then fail, whereas the commit form fails the list at the second comma.

Behavioural difference. The commit changes behaviour for lists whose hand form relies on the retry at the separator:
`args = e (-"," e)* -","?` accepts a trailing comma through the retry; the committed `forbidden` list rejects it, and
`trailing=optional` accepts it by lookahead instead. A grammar migrates only where the retry is not used for something
else, for example when the continuation of the list starts with a separator.

### How it removes the reasons in the bundled grammars

In the four large grammars with reasons the removal is mechanical: each affected hand-written list is replaced, the
hand form is verified to have the same language (compare the parse of the test corpus, a differential test), and the
classifier no longer reports the iteration and option. Counts, from the classifier with `runSitesOn` off:

| Grammar | Iterations removed | Options removed |
|---|---|---|
| cel | 3 | 2 |
| duckdb | 22 | 1 |
| golang | 10 | 1 |
| python | 15 | 2 |
| typescript | 10 | 12 |
| Total | 60 | 18 |

For `typescript` the 12 option reasons are almost all `","?` after type argument and parameter lists, which map to
`trailing=optional`. In `duckdb` and `python` the iterations are argument lists, select lists and parameter lists, which
map to `forbidden` or `optional` depending on the dialect. In `golang` the statement lists with a mandatory `;` map
to `trailing=required`. The grammar-by-grammar replacement list is to be done in the stage that adopts the pragma, with
the differential check of the bundled test corpus as the gate. No class changes: the remaining reasons in the same
rules (operator loops, expression-shaped elements) keep the grammars at their present class.

### Compatibility with the planned token layer

A token layer removes the character-level restarts by running a deterministic lexer before the parser; a separator is
a token. The pragma is compatible with it, and it benefits from it:

- With tokens, `sep` is a token and condition 1 (disjoint first set) becomes a test on token kinds, which is
  exact where the character test is conservative.
- The commit rule does not depend on how input is split, so the same pragma means the same list in both layers.
- Under a token layer the iteration reasons that the pragma removes would mostly disappear anyway, since the choice
  between "another element" and "end of the list" is a one-token decision. The pragma still matters for grammars that do
  not use the token layer and for the engine: the commit gives the failure position and the expectation set directly,
  without a farthest-failure comparison.
- A list over tokens keeps `#sep(",")` with the token as separator. No changes to the spelling are needed.

If the token layer lands first, the pragma remains the source-level contract of the list and the reason counts in the
motivation table become the character-level baseline.

## Alternatives and rejected options

- **Do nothing; classify `(-s e)*` specially.** The classifier could recognize the hand form. This makes the
  classification depend on a spelling the compiler cannot guarantee (`(-"," e)*` versus `("," -e)*` versus a rule
  `tail = -"," e`), and gives users no control over the trailing policy. A recognized pattern also drifts from the
  behaviour, since the hand form has the retry semantics and the committed form does not.
- **A general commit-after-prefix operator (`a ~ b`).** It would cover the separated list as a special case. It is a
  larger, language-wide feature with its own scoping and error rules; the list needs only its fixed instance. If the
  language later adds the general operator, the list can be defined by it.
- **Keeping separators in the value.** A list of `(sep, e)` pairs makes the common case noisy and gives the `-` drop a
  different meaning. Grammars that want separators can write the repetition.

## Decisions

1. **Spelling.** The postfix pragma `#sep(s, trailing=...)`. It matches `#repeat` and `#recover` and carries the policy
   without new syntax.
2. **Default trailing policy.** `forbidden`. `optional` and `required` are spelled out.
3. **Commit.** The list always commits after the separator. The compiler rejects the pragma when the commit would be
   unsound: the element can start with the separator, the element can match the empty input, or, under
   `trailing=optional`, the first set of the element is not a set of characters.
4. **Separator in the value.** Dropped. There is no option to keep it; a grammar that needs separators writes the
   repetition by hand.
5. **`trailing=required`.** Included.
6. **Streaming.** `#stream` is allowed on a list that is the top-level repetition of the start rule.
7. **Separator form.** Any capture-free expression; the pragma is rejected where the first-set test cannot prove the
   commit sound.
