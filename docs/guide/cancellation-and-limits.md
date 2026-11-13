# Cancellation and Compute Limits

A parse of untrusted or very large input should be able to stop: when a caller gives up, when a deadline passes, or when
the cost exceeds a budget. This guide shows the per-call options that do that and what a stopped parse returns. The
nesting limit, `Limits.Depth`, is summarized [below](#bounding-the-nesting-depth) and described with the
[backends](backends.md#deep-nesting-and-the-depth-limit).

The examples use a parser `p` compiled as in [Running Parsers](running-parsers.md). The repository's security policy,
`SECURITY.md` at the root of the repository, lists what a parse bounds and what the caller must bound.

## Cancelling a Parse

`Parse` accepts per-invocation `WithContext` and `WithLimits` options. A zero
limit selects the default described [below](#the-default-compute-limit); `core.Unlimited` disables the limit. The counter
resets for the next call. Generated Go parsers offer the context and a finite `Compute` or `Memory` only when
generated with `-limits`; see [the generated Go API](generated-go-api.md#cancellation-and-compute-limits). Generated TypeScript parsers
offer the `signal` option and the `compute` limit only with `-limits` too; see
[TypeScript](typescript.md#cancellation-and-compute-limits). `Input` and `Depth` work in every form.

`WithLimits` merges field by field: a nonzero field overrides the value of an earlier `WithLimits`, and a zero field
leaves it unchanged, so `WithLimits(core.Limits{Input: n})` followed by `WithLimits(core.Limits{Depth: d})` sets both. The
same holds between the defaults a `Document` is constructed with and the options of one of its `Parse` calls. A field
that no option sets takes its default; there is no way to reset a field to its default once an earlier option set it.

```go
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
node, err := p.Parse(input,
    engine.WithContext(ctx),
    engine.WithLimits(core.Limits{Compute: 5_000_000}),
)
switch {
case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
    // The invocation was cancelled.
case errors.Is(err, core.ErrLimit):
    // A compute limit was reached; use errors.As for *core.LimitError details.
case err != nil:
    // Syntax or another parse error.
case node != nil:
    // The parse completed.
}
```

`Compute` counts inspected input elements and backend execution steps in one counter, one unit each; it is neither a
time nor a memory limit, and backtracking never refunds it. Scanning input, such as a repeated character class,
charges it in proportion to the elements examined, so one limit bounds the cost of large input, rule calls and
values that grow. The memory a parse allocates, long values included, is at most proportional to the budget it
spends plus the input, so a smaller budget also bounds memory; [`Memory`](#bounding-memory) bounds it in bytes
without cutting the time a parse may take. An interruption is fatal to the parse and `#recover` cannot consume it,
so a batch parse returns no partial result. Cancellation is cooperative: a parser cannot preempt a blocked reader or
a callback of yours. The exact rules are in
[Cancellation and Compute Limits](../../spec/runtime/parsing.md#cancellation-and-compute-limits); the
[Streaming](streaming.md#cancelling-a-stream) guide shows how to cancel caller-owned I/O.

## The Default Compute Limit

A parse with a zero `Compute` is not unbounded. It gets a budget of `max(2^20, 1024 * n)`, where `n` is the length of
the input in the position unit, so a parse of untrusted input stops without your choosing a limit. Input whose
cost per element is higher than 1024 on average, such as a grammar that backtracks heavily, fails with a
`*core.LimitError`, whose `Limit` is the resolved budget. Pass a larger `Compute` when you expect that cost, or
`core.Unlimited` when the input is trusted:

```go
node, err := p.Parse(input, engine.WithLimits(core.Limits{Compute: core.Unlimited}))
```

A parse with `Unlimited` and no context runs without any control, which is the fastest form. A stream does not know
its length in advance: its default is `max(2^20, 1024 * d)`, where `d` is the furthest position the parser has asked the stream for, so it does not
depend on how your reader splits the input. A `Document` resolves the
default from the text at each `Parse`. Generated Go and TypeScript parsers apply the default when they are generated
with `-limits`; a parser generated without it spends no budget and accepts only a zero or `Unlimited` `Compute`.

## Bounding the Input Length

`Limits.Input` bounds the length of the input in the position unit. Its default is `core.Unlimited`: the default
budget already grows with the input, so a long input costs in proportion to its length. Set it when you want to
refuse input above a size before spending any work on it:

```go
node, err := p.Parse(input, engine.WithLimits(core.Limits{Input: 1 << 20}))
var le *core.LimitError
if errors.As(err, &le) && le.Kind == core.LimitInput {
    // le.Limit is the Input limit and le.Requested is the length of the input.
}
```

An input of exactly `Input` units is accepted; a longer one fails at once with `LimitInput`, before the parser
runs and even before a canceled context is noticed. A `Document` checks the text it holds at each `Parse`, so an edit
that makes the text too long fails the next `Parse` and a later edit that shortens it makes the `Parse` succeed. A
stream fails when the parser asks for a unit beyond the limit and the stream has one, however your reader chunks the
input, and the error reports the number of units the parser asked for. A finite `Input` needs no budget: with
`Compute: core.Unlimited` and no context the parse still runs without a control.

## Bounding Memory

`Limits.Memory` bounds, in bytes, the storage a parse takes into use: nodes and their lists, memo entries and
their indexes, typed values, the decoded input, the working stacks, the parser's bookkeeping tables, and the strings
that actions concatenate. Its default is
`core.Unlimited`, because the default compute limit already bounds memory in proportion to the work. Set it when you
want a bound in bytes that does not also cut the time a parse may take:

```go
node, err := p.Parse(input, engine.WithLimits(core.Limits{Memory: 64 << 20}))
var le *core.LimitError
if errors.As(err, &le) && le.Kind == core.LimitMemory {
    // le.Limit is the Memory limit and le.Used is the count when the parse stopped.
}
```

The count is checked every 256 compute units, so a parse can pass the limit by what it allocates in that interval
before it fails; a parse whose final count is at most the limit succeeds. It counts the parser's own buffers from the
sizes of their elements, not the bytes the Go allocator hands out, and it does not count your input string, which
`Input` bounds. A finite `Memory` makes the parse controlled, like a finite `Compute`, so a parser generated without
`-limits` rejects it. A stream returns what it has delivered, so its count follows the largest element and the
working set around it, not the length of the stream, except for recovered errors, which it keeps for the result
unless `WithDiagnosticLimit` keeps only the latest ones ([Streaming](streaming.md#memory)). A `Document` counts the caches it keeps from earlier parses:
when they alone pass the limit it drops them and parses from scratch, and when a parse that reuses them passes the
limit, the next parse starts without them. Counts differ between backends and position units, as compute counts do,
but they are the same on every run of the same parse.

## Bounding the Nesting Depth

`Limits.Depth` bounds the nesting of recursive rule calls. Unlike the other limits it is always on: zero selects
100,000, or 10,000,000 for `BytecodeIterative`, and a deeper parse fails with a `*core.LimitError` of kind
`core.LimitDepth`. It needs no control, so setting it alone keeps the fastest form of the parse:

```go
node, err := p.Parse(input, engine.WithLimits(core.Limits{Depth: 1000}))
var le *core.LimitError
if errors.As(err, &le) && le.Kind == core.LimitDepth {
    // le.Limit is the depth limit.
}
```

`core.Unlimited` disables the limit only on `BytecodeIterative`, which keeps rule calls on the heap; the Closure and
Bytecode backends and generated parsers nest rule calls on the goroutine stack and reject it as an option error. See
[Deep Nesting](backends.md#deep-nesting-and-the-depth-limit) for how far each backend can go.

## Limits on the Command Line

The `zp` commands that run a parse (`parse`, `test`, `trace`, `profile`, `explain` and `edit-fuzz`) take `-max-compute`,
`-max-memory`, `-max-input` and `-max-depth`, which set `Limits.Compute`, `Limits.Memory`, `Limits.Input` and
`Limits.Depth`. Each takes a number, `0`
for the default, or `unlimited`:

```bash
zp parse -g grammar.zp -max-compute unlimited -max-input 1048576 -i "$input"
```

A parse that reaches a limit prints the `*core.LimitError` message and exits with status 1. `zp test` reports it as
the runtime error of the case. A saved `edit-fuzz` sequence does not record the limits, so `edit-fuzz -replay` takes
`-max-compute`, `-max-memory`, `-max-input` and `-max-depth` again. A `Document` reuses earlier work and spends less than a fresh parse, so `edit-fuzz`
compares nothing at a check where either parse exhausts its budget.

## Related Topics

- [Running Parsers](running-parsers.md): results, errors and concurrency.
- [Streaming](streaming.md): cancelling a stream whose input is caller-owned I/O.
- [Cancellation and Compute Limits](../../spec/runtime/parsing.md#cancellation-and-compute-limits): the exact rules.
- [Time and Memory](../../spec/runtime/parsing.md#time-and-memory): which grammars have a cost that grows faster than the
  input, and what each compute limit bounds.
