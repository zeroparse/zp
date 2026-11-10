# 070. Unknown Outranks Polynomial

- **Status**: Implemented
- **Author**: @ornew
- **Date**: 2026-11-10

## Summary

The classifier ranks a reason for the class unknown above polynomial causes. A grammar is polynomial only when every
condition for linear cost that fails has a witnessed cause, so the degree that it reports bounds all of them. A grammar
with a reason and no exponential cause is unknown; when it also has polynomial causes, the classification reports the
largest of their degrees as its **witnessed degree**, a lower estimate rather than a bound:

```text
grammar.zp: the cost from start rule main is unknown (witnessed n^2)
```

This revises the chain of verdicts of [record 064](064-grammar-complexity.md#classifier), which ranked any witnessed
cause above a reason.

## Motivation

Under record 064 the chain was `Linear < Linear·Env < Unknown < Poly(k) < Exp`: a witness of growth outranked a
reason, because it is evidence while a reason is only the absence of a proof. The degree of a polynomial grammar with
reasons was the degree of its causes alone, and the specification said that what the reasons leave open can cost more.

A reason is, by its definition, a place where the classifier knows no bound: units that are not committing nested
without bound through a recursion of rule calls, a construct or a growth that the classifier does not analyze, or a
copy of a value whose size it does not bound. Where such a place multiplies the cost of a cause, the degree of the
cause does not bound the grammar. Measured with the random grammars of the slope gate and with hand-written grammars,
three grammars that the old chain classed polynomial of degree 2 cost n^3 on every backend:

| Grammar | Reasons | Measured slope |
|:--|:--|:--|
| `main = r* $$; r = ([ab]? (r / [a-c]) (w [ ]))*; w = [ab]*` | nesting without bound around a scan evaluated in each step of the growth of `r` | 2.96 at n = 640 |
| `main = r $$; p = (!"a" "b")?; f = r / [a-c]; r = (p f (w [ ] [b]))*; w = [^ ]*` | the same, and a growth from a rule that is not a leader | 2.85 at n = 160 |
| `main = v:V (B / "a")* $$; V = xs:"x"* y:"y"* => map($xs, (p) => map($xs, (q) => text($y) + "!")); B = "a"* "c"` | copies, for each element, of a value from outside the lambda | 2.95 at n = 200 |

Of the first 10 000 random grammars of the gate, 4497 were polynomial with reasons, and the gate did not hold their
degree. Two of them exceeded it.

## Decision

The chain, from best to worst, is:

```text
Linear  <  Linear·Env(vars)  <  Poly(k, witnesses)  <  Unknown(reasons)  <  Exp(witness)
```

- `Poly(k)` is assigned when causes are found and no reason is. Its degree k is the largest degree of the causes, and
  it estimates a bound on the whole cost.
- `Unknown` is assigned when a reason is found and no exponential cause is. Its polynomial causes stay witnesses and
  `super-linear` hints. The largest of their degrees is reported as the witnessed degree, with the text
  `unknown (witnessed n^k)` and the JSON member `witnessedDegree`. It is a lower estimate from the witnesses.
- `Exp` outranks a reason: it is evidence of growth that no polynomial bounds, and it reports no degree.

The `super-linear` check reports every polynomial or exponential cause, whatever the class.

## Alternatives Considered

- **Keep the old chain and call the degree a lower estimate.** It keeps the degree visible, but a polynomial class
  would then bound nothing, and the gate could not hold any grammar with reasons.
- **Fold the reasons into the degree.** Some reasons have a finite estimate, such as a copy of an outer value whose size
  degree is known, and could become causes with a degree. Nesting without bound through a recursion has no finite
  estimate by definition, and it is the most common reason in the bundled grammars, so this alone would not bound
  them. It remains a way to make the classifier more precise, independent of this decision.
- **Report no degree with the class unknown.** It loses the witnessed degree, which still tells a user how fast the
  witnessed paths can grow.

## Compatibility

The classes of the bundled grammars cel, duckdb, golang, typescript, xml, minilang and outline change from polynomial
to unknown with a witnessed degree. The order of the class constants in the Go API changes to match the chain.

## Testing

The class tests of the classifier hold the three grammars above to the class unknown with their witnessed degree. The
growth tests of the slope gate measure them on every backend and fail if the class claims a degree below the measured
growth. The slope gate fails on any polynomial class that has reasons, and no longer leaves such grammars out. Under
the old order of the chain each of these checks fails.
