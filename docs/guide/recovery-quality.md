# Recovery Quality

This page reports how well [`#recover`](recovery.md) repairs invalid input on two ready-made languages, JSON and
CEL, and what a grammar author can expect from it. It follows the method of the CPCT+ evaluation (Diekmann and
Tratt, "Don't Panic! Better, Fewer, Syntax Errors for LR Parsers"): take valid inputs, break them, run the
recovering parser and count what it repairs.

## Method

- **Inputs.** The corpus is made of near misses: valid inputs generated from the ready-made grammar with
  [`zp sample`](sampling-and-fuzzing.md), each changed by one mutation (`delete`, `insert`, `replace`, `duplicate`,
  `swap` or `truncate`) that the ready-made grammar rejects. Only inputs with a separator (a `,`) are kept, because
  recovery here works on the elements of a list, so an input without one has no place to recover at. Each language has
  about 300 inputs, fixed in `testdata/recovery/`, together with the offset of the mutation.
- **Grammars.** The ready-made grammars have no recovery. The recovering variants (`testdata/recovery/json.zp` and
  `cel.zp`) add `#recover` to one element of the language and nothing else: the members of a JSON object and the
  elements of an array, and the arguments, list elements, map entries and message fields of CEL. Each skips up to the
  next `,` or bracket, the pattern of [Recover Inside a List](recovery.md#recover-inside-a-list), and labels the
  closers.
- **Measures.** For each input the recovering parser either fails (no tree) or returns a tree with recovered errors.
  A recovered parse is *one error* if it reports exactly one error, the number of mutations, and *located* if every
  error value overlaps the 8 bytes around the mutation. *Skipped* is the share of the input that the error values
  cover: the input given up to repair the rest.
- **Results** are recomputed by `go test ./internal/recoveryquality`, which fails when this table is stale.

## Results

<!-- recovery-quality-table:begin -->
| Language | Invalid inputs | Accepted by the recovering grammar | Recovered | Failed | One error | Located | Skipped, median | Skipped, 90th percentile |
|:--|--:|--:|--:|--:|--:|--:|--:|--:|
| json | 282 | 0 | 30% | 70% | 94% | 96% | 23% | 48% |
| cel | 300 | 0 | 4% | 96% | 92% | 92% | 6% | 24% |

| Mutation | json recovered | cel recovered |
|:--|--:|--:|
| delete | 31% | 2% |
| insert | 40% | 12% |
| replace | 42% | 4% |
| duplicate | 29% | 6% |
| swap | 46% | 11% |
| truncate | 0% | 0% |
<!-- recovery-quality-table:end -->

Reading the table:

- **Recovery repairs a bracketed element, not the structure around it.** Where the mutation breaks an element
  (`[1, x, 3]`), the parse continues and loses only that element. A mutation that breaks a delimiter, a bracket or a
  string quote, or removes the closer (`truncate` never recovers), fails the whole parse: skipping cannot rebuild the
  nesting, and a failed parse returns no tree and no recovered errors.
- **When it recovers, it recovers well.** The recovered parses nearly always report the single error that was made,
  and place it next to the mutation.
- **The rate depends on the grammar's recovery points.** JSON, where nearly every value sits in a list or an object
  member, recovers about a third of the broken inputs; CEL, where an operator expression has no element to skip,
  recovers few. More recovery points (a statement or line level, as in the [recipes](recovery.md#recipes)) raise the
  rate; the cost is the skipped input.
- **A recovered tree is partial.** The median recovered parse gives up a fifth of a JSON input and a twentieth of
  a CEL expression, and the 90th percentile about half and a quarter.

The time per repair is not measured here.

## Related Topics

- [Recovering from Errors](recovery.md): `#recover`, `#missing` and the recipes.
- [Sampling Inputs and Fuzzing](sampling-and-fuzzing.md): the near misses.
