# ZeroParse Proofs

The runtimes reuse results to bound the cost of a parse, and they stop a parse
that a limit or a cancellation interrupts. Each mechanism is stated in the
[Parser and Runtime Specification](../runtime/README.md). A reuse is correct
only if it gives the result that evaluating again would give. An interruption
is atomic only if it returns no partial value and leaves no state that a later
parse could read. The proofs here show that, and the cost bounds
that the mechanisms claim, as lemmas over the definitions of the specifications
that they cite.

Each proof names its author and the date of its last revision under its title.
A change to a statement or a proof updates the date. From the first release
on, each proof also lists the releases that changed it. To cite a proof, cite
ZeroParse and name the proof.

## Contents

| Proof | Shows | Status |
|:--|:--|:--|
| [Skip Runs](skip-runs.md) | A reused record of a [skip run](../runtime/parsing.md#skip-runs) gives what the rest of the walk would, and the walks of a site evaluate its element a linear number of times | Proved |
| [Memoization](memoization.md) | Answering a call from a memo entry, including the members of a left-recursion cycle and results that depend on the order of evaluation, gives the outcome that evaluating it again would give | Proved, given the assumptions that it lists |
| [Left-Recursion Growth](left-recursion.md) | The growth of a left-recursive rule at a position ends after at most two more steps than the input it consumes, and gives one result, which every parse gives, including the members of a cycle, several leaders at one position and the dependency memo | Proved, given the assumptions that it lists |
| [Documents](documents.md) | A parse of a [Document](../runtime/documents.md) after edits, reusing carried memo entries and recorded runs, gives the result of a fresh parse of its text | Proved, given the assumptions that it lists |
| [Interruptions](interruptions.md) | A [cancellation or exhausted limit](../runtime/parsing.md#cancellation-and-compute-limits) is atomic: a reservation happens whole or not at all, nothing absorbs an interruption, a parse returns no partial value and leaves no state that a later parse could read, a stream keeps what it delivered, and the point where a limit stops a parse repeats on every run of a backend | Proved, given the assumptions that it lists |
| [Linear Cost](linear-cost.md) | How the [Conditions for Linear Cost](../runtime/parsing.md#conditions-for-linear-cost) bound the cost of a parse | Lemmas 1, 2, 3, 7, 8, 9 and 10 proved; Lemma 6 proved given the count of activations of Lemma 4 and the charge of copies of values that are neither own nor values of variables; Lemmas 4 and 5 outlined, with proved claims and the steps still open; the theorem proved given Lemmas 4, 5 and 6, so the class Linear rests on open step 6 as well |
| [Constant-Cost Text](constant-cost-text.md) | Taking the [text](../language/actions.md#text) of a node is a step of constant cost given the guarantee of [Cost](../runtime/parsing.md#cost), so the conditions need not count it as a copy, and a comparison of strings may count only one operand | Proved |
