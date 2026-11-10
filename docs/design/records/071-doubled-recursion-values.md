# 071. Doubled Recursion Values

- **Status**: Implemented
- **Author**: @ornew
- **Date**: 2026-11-10

## Summary

A copying operation that copies the value of a call on a recursion more than once is exponential only when the copies
become part of the value of its rule and that value feeds back into the recursion: then the value of each level holds
the value of the level inside it twice and doubles with the depth. Copies made anywhere else, such as in a predicate or
of a value that the next level does not read, cost the size of the value of the call each time, which adds up to n² over
the levels, and are polynomial of degree 2.

```zp
@start
rule main = E $$

rule E string = l:A rest:(o:"+" r:A)* => foldl($l, $rest, (acc, i) => acc + text(i.$o) + i.$r + i.$r)
rule A string = ("x" => "x") / ("(" v:E ")" => $v)
```

The value of `E` holds the value of the `E` nested in it twice, so the classification of this grammar is exponential.

## Motivation

The classification had one rule for copies of a value of a recursion: a copying operation that copied it once gave
degree 2, and one that copied it more than once was exponential. The rule missed the grammar above and over-reported
others.

- **Missed growth.** The grammar above was classed unknown with a witnessed degree of 2. The copy is in the lambda of a
  fold, where the cause for the fold accumulator took precedence over the cause for the value of the recursion, and the
  doubling was not seen. Its cost measures exponential on every backend: with `x+(` nested 3, 6 and 12 times, the closure
  backend spends 378, 1 117 and 37 888 steps, and the others agree within a constant.
- **Over-reported growth.** In `L = ("(" x:L ")" {len($x + $x) >= 0} => $x + "y") / ("a" => "a")` the predicate copies
  `$x` twice on each level, but the value of `L` grows by one character per level. The rule classed it exponential;
  its cost measures n², with slopes of 1.98 and 1.99 at n = 200, 400 and 800 on every backend. The same holds for
  `text($s) + text($s)` of a node `s` of the recursion, whose text is the input that it matches.

A copy doubles the size of a value only if the result is the value that the next copy reads. Whether a copy is more than
one copy of a value is not enough to tell.

## Decision

[Classification](../../../spec/runtime/parsing.md#classification) defines when a value **holds** another (it is that
value, passes it on, or builds a string or a list from it by `+`, `text`, `list`, `concat`, `filter`, `map` or a fold),
what the **value of a rule** is, and when the value of a call on a recursion **feeds back** into a rule (it holds the
value of the rule, directly or through the values of the rules that it holds).

- A copying operation whose result the value of its rule holds, and which copies more than once the value of a call
  on a recursion with the rule that feeds back into the rule, is an exponential cause.
- Every other copy of the value of a call on a recursion with its rule is a polynomial cause of degree 2, however many
  times it copies the value.
- Copies of a fold accumulator keep their rules: more than once is exponential.

Like the step matrix of a fold, the exponential cause takes the values to be non-empty: copies of an empty value do not
grow.

## Alternatives Considered

- **Keep the count alone.** It is simple, but it calls a quadratic grammar exponential, and the class is the first thing
  a user reads.
- **Use the size degree to decide feeding back.** The size degree says whether a value can include the value of the
  recursion, and a rule whose value is a choice of inline actions counts as including it even when every alternative
  returns a literal. Exponential needs more: that the value is the one that the next level copies again. Holding follows
  the flow of values only, so a rule that returns a literal does not feed back.
- **Report the doubled copy as unknown.** It would be safe, but the growth is witnessed and certain for non-empty values,
  and an exponential class names the cause where unknown names only a condition.

## Compatibility

No bundled grammar changes class: the reasons of the value-size groups of the python and typescript grammars are not
doubled copies of a value that feeds back.

## Testing

The class tests hold the grammar above, the same with `text($v)` in `A`, and `$x + "(" + $x` returned from a recursion
to the class exponential; the same fold copying once, a value of `A` that is a literal, the predicate copy and the
doubled text of a node to degree 2. The controls measure the grammar above as exponential and the predicate copy as n² on
every backend, and the grammar whose `A` returns a literal as linear. The growth tests of the slope gate measure the
grammar above on every backend and fail if its class claims a degree below a slope of 4. With the previous rule, the
class tests and the class checks of the controls fail.
