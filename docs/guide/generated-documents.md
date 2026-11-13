# Incremental Documents in Generated Parsers

Generate with `zp gen -g grammar.zp -module example.com/parser -document -o parser`, or pass
`Document: true` to `gen.Go`. This adds a mutable Node `Document` alongside the ordinary batch `Parse` and
`ParseRule` functions. The engine's version of the same feature is described in
[Incremental Parsing](incremental.md).

## Editing and Reparsing

```go
doc, err := parser.NewDocument("first draft", parser.WithUnit(parser.CodePoints))
if err != nil {
	return err
}
tree, err := doc.Parse()
if err := doc.Edit(6, 11, "revision"); err != nil {
	return err
}
tree, err = doc.Parse()
stats := doc.Stats() // rule evaluations and reused memo results from this Parse
_ = tree
_ = stats
```

`Edit(start, end, replacement)` replaces the half-open range in the unit selected at construction. It rejects
invalid ranges and positions inside a UTF-8 character in `Bytes` mode without changing the text. `Text()` returns
the exact current input. Construction options become defaults for later `Parse` calls; per-call options can change
depth, context and compute limits, but cannot change the unit. An interruption during execution discards memo history so a
later call can retry. `Document` is single-goroutine, and results can share nodes with later parses. Call
`tree.Clone()` if an earlier tree must remain detached from the Document. It copies Node graphs, including Node
values in `[]any` and `[]*Node` fields, while preserving cycles and repeated references to the same slice view.
Overlapping slice views with different lengths or capacities are copied independently; arbitrary mutable action
values such as maps remain shared. `ParseStats` counts the rule evaluations (`Evaluated`) and reuses (`Reused`) of the last parse; a generated
`Document` reuses only memo results, so `MemoHits` equals `Reused` and `ResumedElements` is zero.

## What Is Kept Between Parses

Both kinds of Document keep the memo results that examined only text before an edit where they are, and move those
that examined only text after it by the edit's change in length; the others are evaluated again. A result of a rule
whose actions or predicates read positions (`.Start`, `.End`), or that recovered errors, is not moved. The Node
Document moves the nodes of a moved result when a parse reuses it, in copies, so a tree returned earlier keeps its
positions; a result that holds an action value other than nodes and basic values is evaluated again instead. `Edit`
copies the kept entries into a new table, so its cost grows with document length. A typed Document relinks its
entries during `Edit` and reuses a moved result only when its value is a basic value, a boolean, a number or a
string; every other value, such as a struct or a list, is evaluated again. The generated batch functions remain available. `-document` can be combined with `-types`: eligible
grammars use direct typed matching with a private memo and fresh invocation arenas. A typed result is
read-only and shared, as Node values are: later parses reuse the values of earlier results, so a result can share
values with the Document and with other results. Modifying a result can change what later parses return, including
through a `node[_]` field. Call `CloneValue(value)` to get a copy that shares nothing with the Document or with other
results; the copy can be modified freely, and aliases within it are preserved. Every returned slice has capacity equal
to its length, so appending to one result cannot write into another. Other grammars use the existing Node Document and
convert its final result on each call; generic `Node` members may share the mutable Document graph, so call
`Node.Clone` when a detached tree is needed. The conversion fallback applies to `any` and node results, struct fields
requiring ZeroParse nodes, or actions and predicates that read struct fields.

## Diagnostics, Limits and Combinations

Both `Document.Parse` and `Document.ParseValue` return owned syntax diagnostics: recovered `SyntaxError` values and
their `Expected` and `Messages` slices are copied; ordinary failures are freshly constructed. Mutating a returned
error cannot affect memoized errors or a later parse. Parsing, diagnostic copying and typed publication share one
budget; interruption returns the result type's zero value and clears reusable state as needed for a retry.
Engine-only Document controls such as `EditBatch` are not generated. See the [direct typed measurements](../design/records/044-generated-incremental-and-stream-parsers.md#local-measurements-for-direct-typed-document-matching)
and the [conversion fallback measurements](../design/records/044-generated-incremental-and-stream-parsers.md#local-measurements-for-typed-go-document-conversion).

## TypeScript Documents

With `-lang ts`, `-document` (or `Document: true` in `gen.TypeScript`) adds `createDocument` to `parser.ts`:

```ts
const { document: doc, error } = createDocument("first draft", { unit: CodePoints });
if (error !== null) throw error;
let result = doc!.parse();
const editError = doc!.edit(6, 11, "revision");
result = doc!.parse();
```

`createDocument(input, options)` copies the input and reads the options once; they become the defaults of
every `parse`, and `document` is null when they are invalid. `edit(start, end, replacement)` follows the Go
contract and returns an `Error` or null; a string replacement is encoded as UTF-8, with lone surrogates as
U+FFFD, and a `Uint8Array` replacement is taken as it is, so edits can join invalid bytes into characters.
`text()` returns the text as the nodes hold it, and `bytes()` returns a copy of the exact input, invalid bytes
included. `parse(options?)` returns the `ParseResult` of `parse`, and with `-types` the Document also has
`parseValue(options?)`. An invocation cannot change the unit or the expectations, and a `parse` or `edit` made
from a trace callback of a parse returns an error. Like the Go `Document`, it keeps the memoized results that
the edits leave valid and reuses them in the next `parse`; `stats()` returns `{ evaluated, reused }` for the
last parse, the same counts as Go's `Stats` after `Parse`; Go's typed `ParseValue` keeps its own memo and can
count differently. A returned tree can share nodes with later results, so do not
mutate a tree while the Document is in use.

## Related Topics

- [Incremental Parsing](incremental.md): the engine's `Document` and what reuse means.
- [The Generated Go API](generated-go-api.md): the entry points that `-document` sits beside.
- [Typed Values in Generated Go](generated-typed-values.md): `-types`, which `Document.ParseValue` needs.
