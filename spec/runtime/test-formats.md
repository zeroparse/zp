# Fixture Suites and Edit Sequences

Two JSON files carry test data between a user and the tools: the fixture suite that `zp test` runs and the saved edit
sequence that `zp edit-fuzz` writes and replays. This chapter defines both formats. Each is versioned; a reader
rejects a version it does not know.

## Scope and Terminology

A **fixture suite** is a JSON object that names a grammar and lists cases: an input and the expected outcome of
parsing it. An **outcome** is `accepted` (the whole input matched without recovery), `recovered` (the whole input
matched after `#recover`) or `rejected` (the parse failed). A **golden** is an expected value recorded in the suite
(a node, a span or the locations of syntax errors). An **edit sequence** is the log of one run of the edit fuzzer: the
initial text, then edit, undo, redo and checkpoint operations.

## Contract

### Fixture Suites

A suite is a JSON object with these members. Unknown members are errors.

| Member | Meaning |
|:--|:--|
| `version` | The format version, the integer 1. Any other value is an error |
| `grammar` | Required. The grammar file: read as grammar JSON if its name ends in `.json`, and as ZeroParse source otherwise. A relative path is relative to the suite file |
| `start` | The start rule. Default: the grammar's default start rule |
| `unit` | `codepoints` or `bytes`. Default: `codepoints` |
| `backends` | A list of `default`, `closure`, `bytecode` and `bytecode-iterative`. Default: `["default"]`, which is the closure backend. Two names for the same backend, such as `default` and `closure`, are an error; repeated names are ignored |
| `equivalence` | A boolean. When true, the backends must agree on every case (see below) |
| `cases` | Required, nonempty. The cases |

A case is an object with `name` (required, unique in the suite), `input` (required, a string, possibly empty),
`expect` (required, an expectation object) and `byBackend` (optional, a map from a backend name to an expectation
override).

An expectation has the members `outcome`, `node`, `start`, `end`, `errors` and `clear`. Unknown members and `null`
values are errors, except that `"node": null` is a valid golden for a parse that has no node.

| Member | Meaning |
|:--|:--|
| `outcome` | `accepted`, `recovered` or `rejected`. Required in `expect` for every backend after overrides are applied |
| `node` | The expected node as the JSON form of [Nodes and Values](nodes.md). Compared as canonical JSON |
| `start`, `end` | The expected span of the root node, in the suite's unit; nonnegative integers |
| `errors` | The expected syntax errors in order, each `{"pos": n, "line": n, "column": n}` with all three members, in the suite's unit. `[]` asserts that there are none |
| `clear` | Only in a `byBackend` expectation: a list of `node`, `start`, `end` and `errors` whose values are not inherited from `expect`. A field cannot be both cleared and given |

A member that an expectation omits is not checked. The expectation for a backend is `expect` with the members of its
`byBackend` entry laid over it: a given member replaces the inherited one, and a name in `clear` removes it.

Each case runs on each selected backend and produces one result per pair. A runtime error is reported for that pair
and later cases continue. With `equivalence`, or with `zp test -equivalence`, which overrides the member in either
direction, the backends must also agree for each case on the outcome, whether a runtime error occurred, the node,
the root span and the syntax-error locations; the report lists the fields that differ. The expectations and the
equivalence are judged independently.

`zp test -update` writes the observed node, span and syntax-error locations of one explicitly selected backend as
goldens, in the `expect` of each case or, when the case has `byBackend` entries, in the entry of the selected backend.
It changes no input or outcome, and it refuses to write when a selected case has a runtime error or an outcome that
differs from its expectation. It cannot be combined with `-equivalence`.

### Edit Sequences

A saved sequence is a JSON object with exactly these members.

| Member | Meaning |
|:--|:--|
| `version` | The format version, the integer 1 |
| `grammar_sha256` | The SHA-256 of the exact bytes of the grammar source, as 64 lowercase hexadecimal digits |
| `grammar_normalization` | The string `none`: the hash is of the bytes as they are, so a change of whitespace changes it |
| `start` | The start rule; required and nonempty |
| `backend` | `closure`, `bytecode` or `bytecode-iterative` |
| `unit` | `codepoints` or `bytes` |
| `seed` | The seed of the run that made the sequence, an unsigned integer |
| `text_encoding` | The string `base64` |
| `initial_base64` | The initial text as base64 bytes, so invalid UTF-8 survives |
| `operations` | The list of operations |

An operation is an object with `kind`. An `edit` has `start` and `end`, the range `[start, end)` of the text just
before the operation in the sequence's unit, and `text_base64`, the replacement as base64 bytes. `undo` restores the
text before the latest edit that is not undone. `redo` reapplies the latest undone edit; a new edit discards the edits
that could be redone. `checkpoint` parses the Document and compares the result with a parse of its current text from
scratch: values with their spans, and errors with their positions, expectations and messages. The sequence is also
compared at its start.

A replay is refused when the grammar's hash differs, and the file is rejected when the start rule, backend or unit is
unknown or when the object has a missing, unknown or mistyped member. The tool never replaces a file that it saved.

## Validation and Errors

A suite that has a wrong version, a missing or empty required member, an unknown member, a duplicate case name, an
unknown backend or outcome or a malformed expectation is rejected before any case runs, with a message that names the
file and, for a case, its name. A sequence that is not valid is rejected before any operation runs.

## Compatibility and Support

Both formats are at version 1. A reader supports only the versions it names. A compiled grammar is not a
supported `grammar` of a suite.

## Conformance

The tests of `internal/fixture` and `editfuzz` cover the validation rules, the override and `clear` semantics, the
equivalence report, `-update` and the round trip of sequences.
