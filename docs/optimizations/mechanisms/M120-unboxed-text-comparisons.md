# M120: Unboxed Text Comparisons

Comparing two `text` results through generic interface values can allocate
string headers. Native string equality avoids those boxes.

## Mechanism and Invariants

Eligible equality/inequality expressions extract text and compare strings
directly while preserving operand errors. VMs use `ETEXTCHK`/`ETEXTEQ`
expression instructions; generators emit native comparisons. This changes
comparison plumbing, not substring ownership or arbitrary string-valued action
results.

## Scope and State

Default eligible comparisons in closure, both VMs, generated Go Node/typed Go
and TypeScript. JavaScript strings do not have Go interface-header allocations,
though its emitter also uses native string comparison.

## Reference Control

`-no-opt field-compare` makes the term lowering of typed Go compare string fields in
actions and predicates through boxed values again, from the same generator. No
control covers the other comparisons. Literal-text disjunction specialization is
independently controlled by `text-dispatch` ([G070](G070-literal-text-predicates.md)). None of the ten
language grammars has a comparison of that form, so record 136 shows no effect of the flag.

## Evidence and Implementation

Record 036 measures XML about 19k to 1.1k allocations in closure/generated Go;
044 ports the shape to VMs alongside concat fusion. TypeScript has no
corresponding Go-allocation claim. Record 102 measured a text struct field read that
named node types have since made unnecessary.

Evidence records: [036](../036-comparing-texts-without-boxing-them.md),
[044](../044-changes-36-and-37-in-the-vms-instruction-set-2.md),
[102](../102-read-text-struct-fields-without-a-struct-lookup.md).

Source owners: [`text equality evaluator`](../../../internal/engine/core/eval.go),
[`ETEXTCHK and ETEXTEQ`](../../../internal/compile/bcompile.go), [`native text
comparison instructions`](../../../internal/engine/vm/vm.go), [`text comparison
lowering`](../../../internal/ir/exec/lower/term.go), [`Go text comparison
emission`](../../../internal/gen/gogen/term_exec.go), [`TypeScript text comparison
emission`](../../../internal/gen/tsgen/term_exec.go).

The [comparison policy](../../design/records/024-optimization-mechanism-controls.md)
distinguishes same-source controls, historical changes and full checkpoints.

Related mechanisms: [G070](G070-literal-text-predicates.md), [A120](A120-source-token-text.md).

[Optimization Catalog](../README.md)
