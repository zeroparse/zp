# Conditional Laws

Arata Furukawa · Last revised 2026-11-04

**Status:** Draft.

Some [Grammar Laws](grammar-laws.md) hold only under a condition that the
syntax of a rule does not show: that two alternatives never both match, that a
rule is called only at a bounded depth, or that a variable takes few values.
Such a condition becomes available in one of two ways. A **transformation**
rewrites the grammar into an equivalent one in which the condition is visible,
as left factoring makes two alternatives start differently. A **declaration**
states the condition, and the compiler checks it, or a parser checks it when it
binds a variable or calls a rule, or no one checks it. A declaration that no
one checks is trust in the author, and a wrong declaration then changes the
result of a parse silently; the conditions below prefer the other two.

This document lists the conditions, what each makes provable about the
[Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost),
how it is checked and what it costs an author. It then starts from the
reasons that the [classification](../runtime/parsing.md#classification) gives
and names, for each kind of reason, the condition that would remove it. The
notation and the relations $\equiv_E$, $\equiv_V$, $\equiv_X$ and $\equiv_O$
are those of [Grammar Laws](grammar-laws.md#equivalences).

## Conditions From Transformations

The transformations below preserve $\equiv_O$ or the relation stated, so a
compiler can apply them without a declaration.

### Normal Forms

The level E rules of [Normalization](grammar-laws.md#normalization) give a
view of each rule in which unrolled whitespace is one repetition, a run of
single characters is one class, and annotations that affect only values or
messages are gone. On that view (P2) and (P3) decide whether a rule is
idempotent or annihilates itself, and the first characters of a unit are
those of its normal form.

**Checked:** statically. **Author cost:** none.

### Left Factoring

`a b / a c` becomes `a (b / c)` (C6). The alternatives `b` and `c` then start
at one position after a shared prefix, where their first items can differ.
The rewrite needs a prefix and alternatives without loose cuts; it changes
the value when `b` or `c` is a sequence, so an optimizer applies it to a
copy that keeps the original value shape, as Core IR can by building the
tuple after the choice.

**Checked:** statically. **Author cost:** none.

### Specialization per Call Site

An option, a repetition or a lookahead at the end of a rule is a unit whose
continuation is what follows each call of the rule. The classifier must take
every caller's continuation at once. Cloning the rule per caller, or
inlining it where (P1) of [Grammar Laws](grammar-laws.md#rule-calls) allows,
gives each copy one continuation, which is often disjoint from the unit.

**Checked:** statically. **Author cost:** none. Code size grows with the
number of clones; the memo of a cloned rule is split by clone.

### Filtered First Characters

(F2) lets a unit `&L e` with a single character test `L` be compared with its
continuations by the intersection of the first characters of `L` and `e`.
Grammars that dispatch on the first character, as `&[0-9] number`, then have
disjoint units.

**Checked:** statically. **Author cost:** none.

### Equal Environments and Free Variables

(D2) lets a shared call at the start of a unit read variables, and (D3) lets
the key of a call keep only its free variables. A rule that binds a variable
before it reads it then has a smaller key, which (D1) uses both for sharing
and for the memo.

**Checked:** statically, by a definite-binding analysis. **Author cost:**
none.

### Words Without Case

A sequence of classes that each accept the upper and lower case of one letter,
as `[sS][eE][lL][eE][cC][tT]`, matches what a case-insensitive literal
matches. Treating it as such a literal lets the words condition apply to
`[sS][eE]... !identchar`, as it applies to `"select"i !identchar`.

**Checked:** statically. **Author cost:** none.

## Conditions From Declarations

### Exclusive Alternatives

**Condition.** No state lets two alternatives of a choice both match.
Equivalent to a `#cut` after the part that decides the alternative (T1), which
the author can write, or an analysis can insert where it proves the
exclusion.

**Checked:** statically where first sets of length $k$ or the exclusion of
(K5) prove it, and at run time only by evaluating the other alternatives,
which costs what the condition saves; otherwise not at all. A written cut
needs no check: it defines the behavior rather than asserting it.

**Author cost:** a cut per choice, and the author must know that the language
needs no later alternative after the cut, which is the knowledge that the
condition asserts.

### Bounded Nesting

**Condition.** The calls of a recursive rule nest at most $d$ deep. A unit
that is not committing and nests without bound costs, under the bound, at
most a factor exponential in $d$ more than linear.

**Checked:** at run time, by a nesting limit as in [Limits](../runtime/parsing.md#limits), which
ends the parse with a run-time error when the bound is exceeded. A bound per
rule needs a counter per rule.

**Author cost:** a number per rule, which also limits the inputs that the
parser accepts. The class that results is linear for a fixed $d$, not linear.

### Variables With Few Values

**Condition.** A variable takes values from a set fixed by the grammar, as a
level that stays between $0$ and $K$, or only increases along a nesting so
that its number of values is bounded by the depth. The key of a call that
reads it then takes boundedly many values at a position, so the
memo still bounds the evaluations of the rule at a position.

**Checked:** statically, without a declaration, where the condition is a
range: a test next to each binding that reads the earlier value, as
`{v := v + 1} {v <= 3}` or `{0 <= e && e <= 3} {v := e}`, is a **guard** that
bounds the values that the binding leaves bound, and the classifier counts the
values of the variable from the guards
([Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost),
[Lemma 8](linear-cost.md#lemma-8-finite-values)). The guard is part of the
grammar: at run time a value outside the range fails the sequence of the
binding, and backtracking undoes it. A declaration is needed only for what a
guard cannot express: a set that is not a range of constants, or a value that
only increases along a nesting, which no constant bounds. Such a declaration is
checked at binding time, by comparing the value bound with the declared set or
with the earlier value; a violation is a run-time error of the action.

**Author cost:** a guard per binding, which a grammar that already limits
the variable may have, or a declaration per variable for what guards cannot
express. A level that grows with the input, as an indentation level, has no
fixed set; bounding it limits the inputs, as bounded nesting does.

### Never Failing

**Condition.** A rule always matches. (C3) and (R1) then hold for its calls.

**Checked:** statically by `AlwaysSucceeds` where the shape proves it, and at
run time by failing with a run-time error where a declared rule fails.

**Author cost:** none when the shape proves it.

### Prefix-Free Matches and Token Boundaries

**Condition.** A rule ends only where its caller can start a new item: no
continuation of the caller can extend a match of the rule, as when the rule
ends with `!identchar` or each match ends with a closing delimiter. A unit
that ends with such a call then commits at the end of the call.

**Checked:** statically where the rule ends with a negative lookahead that
covers the first characters of every continuation, which the words condition
already uses; otherwise not at all.

**Author cost:** none where the lookahead is written.

### Idempotent and Position-Only Rules

**Condition.** A call of the rule at the end of a match of it matches nothing
and depends only on its position. (P2) and (P3) prove this from the normal
form; for a settled rule (P4) such as one that reads an indentation `n`, a
declaration `n > 0` turns `R R` into `_|_`.

**Checked:** statically for (P2) and (P3); at binding time for a declared
range of the variable. **Author cost:** none, or one range per variable.

## From Reasons to Conditions

Every reason that the classification gives names a unit that is not committing
and a continuation that it is compared with. Grouped by what keeps the
unit from committing, the reasons and the conditions that would remove them
are as follows.

| Blocker | Shape | Condition |
|:--|:--|:--|
| Nesting through recursion | The unit and its continuation both reach a recursive rule | Bounded nesting; or one of the conditions below, which make the unit commit |
| First items overlap through calls | Alternatives start with different calls whose first sets overlap, or with a call that reads variables | Left factoring; first sets of length $k > 1$; free variables (D3); exclusive alternatives |
| Follow of a rule | An option, repetition or lookahead at the end of a rule | Specialization per call site |
| Trivia first | A unit and its continuation start with a whitespace call | Normal forms with (P2), so that the shared whitespace call is removed as shared |
| Operator parts | An operator part against another part or an operand | Exclusive alternatives over the operator table; first sets of length $k > 1$ |
| Lookahead first | A unit that starts with `&L` | Filtered first characters |
| Nullable continuation | A continuation that can match nothing | Comparison with what follows the nullable item, as for the follow of a rule |
| Variables in a shared call | A shared call reads a variable | Equal environments (D2) |

A unit that commits is linear without a depth bound. A depth bound removes the
nesting from every reason at once, but the class it gives grows with the bound.
The conditions from transformations remove fewer reasons each, and cost
nothing.
