# G070: Literal Text Predicates

A pure OR of comparisons against one capture's text can compute that text once
and dispatch by literal, rather than repeat general expression evaluation for
every branch.

## Mechanism and Invariants

The complete predicate must consist of comparisons of the same direct capture
with string literals, joined by OR; a single comparison qualifies, and either
operand order is accepted. The term lowering reads the text once and returns
whether it equals one of the literals, testing them in order. Mixed
expressions, aliases, bindings and different captures remain general,
preserving short-circuit order and side effects. Helper/literal-table sharing
is independent of this shape specialization. Generated Go and TypeScript take
this path on ordinary nil-control calls; active budget-controlled calls keep
the general charged evaluation.

## Scope and State

Default eligible predicates in generated Go Node, typed Go and TypeScript.
Closure and both VMs retain their general action evaluator for this disjunction
shape.

## Reference Control

The generation option `-no-opt text-dispatch` emits the general
predicate from the same generator. It adds no runtime selector to the generated parser.

## Evidence and Implementation

Record 091 compares the same generator with dispatch disabled; it measured
dispatch as a switch over the deduplicated literals, which the comparison chain
has not been measured against. Large Go
workloads have paired geometric-mean time ratios about 0.952/0.955 in
CodePoints/Bytes, while small-workload means are 1.001/0.990. TypeScript has
separate scoped gains and a slower first-match case. No-op source is
byte-identical. These results concern qualifying pure predicates, not arbitrary
OR expressions or all parsers.

Evidence records: [091](../091-specialize-pure-literal-text-disjunctions.md).

Source owners: [`literal text dispatch
lowering`](../../../internal/ir/exec/lower/term.go), [`Go
emission`](../../../internal/gen/gogen/term_exec.go), [`TypeScript
emission`](../../../internal/gen/tsgen/term_exec.go), [`route and unsafe-shape
checks`](../../../internal/gen/gen_literal_text_dispatch_test.go).

The [comparison policy](../../design/records/024-optimization-mechanism-controls.md)
distinguishes same-source controls, historical changes and full checkpoints.

Related mechanisms: [M120](M120-unboxed-text-comparisons.md), [G130](G130-generated-literal-sharing.md).

[Optimization Catalog](../README.md)
