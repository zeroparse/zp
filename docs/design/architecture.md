# Architecture

This page explains how ZeroParse turns a grammar into a parser, at the level of ideas: which stages exist, why they are
separate, and which rules hold across packages. It does not list files, fields or functions; the code is the source of
truth for those, and the package comments say what each package owns. Read this page to learn how the parts fit before
you work in `internal/compile/`, `internal/engine/`, `runtime/` or a generator. The contracts every stage must keep are
in the [specification](../../spec/README.md); the middle of the pipeline is in
[Core, Flow and Exec IR](intermediate-representations.md).

## From Grammar to Parser

A grammar passes through a few stages, and each stage answers one question that the next stage can then take as
settled.

```mermaid
flowchart TD
  src[".zp text or grammar JSON"] --> ast["Grammar syntax tree"]
  ast --> chk["Checked grammar"]
  chk --> prog["Program: rules, flags, types"]
  prog --> run["Run: closures or VM"]
  prog --> ir["Core, Flow and Exec IR"]
  ir --> gen["Generated Go"]
  ir --> ts["Generated TypeScript"]
  prog --> bc["Bytecode, .zpc"]
  bc --> run
```

- **Syntax tree.** Text and JSON are two spellings of one tree, so every tool (formatter, linter, language server,
  generators) works on the same structure and neither spelling is privileged. The tree says what the author wrote and
  nothing else.
- **Checked grammar.** Validation, type checking and the analyses (what calls what, what can match empty input, which
  rules can recurse on the left, what shape each value has) run once, over the whole grammar. Later stages rely on
  their answers instead of recomputing them, so a fact has one definition.
- **Program.** Compilation decides how each rule runs: whether it is memoized, whether it leads a left-recursive
  cycle, whether it builds a value or only recognizes. A `Program` is complete enough to run on its own.
- **Backends.** Executing a program and emitting code from it are separate jobs that share the stages above, not the
  implementation of the matcher.

Keeping the stages apart is what allows a new tool to enter at the right depth: a linter needs only the tree, a
generator needs the checked and compiled forms, and a runtime needs only a program.

## Why There Are Several Backends

The same grammar must behave the same way everywhere, but different uses need different trade-offs.

| Backend | Reason it exists |
|:--|:--|
| Closures | The reference: the simplest correct reading of the grammar, used by default and as the oracle for the others |
| Bytecode VMs | A compact, serializable program that runs without the grammar; the iterative VM keeps deep input off the host stack |
| Compiled grammar files | Ship a program without its source; a loaded program runs on a VM |
| Generated Go | Speed and a typed API: the grammar becomes ordinary Go code over a small runtime |
| Generated TypeScript | The same parsers in browsers and Node |

The reference backend is also a test tool. Differential tests run the same input through several backends and compare
values, errors, expectations and positions, so an optimization is correct when the backends cannot be told apart.

## Three Levels of Representation

The generators are where most of the optimization work happens, and it is organized in layers so that a rewrite is
written once and verified once.

```mermaid
flowchart LR
  prog["Program"] --> core["Core IR: what the rule means"]
  core -->|"semantics-preserving rewrites"| core
  core --> flow["Flow IR: how a parser runs it"]
  flow --> exec["Exec IR: the parser VM that every target implements"]
  exec --> goprint["Go printer"]
  exec --> tsprint["TypeScript printer"]
```

Core IR describes what each rule consumes, expects, captures and can fail with. Flow IR describes how that runs as
variables, blocks and loops. Exec IR specifies the operations of the parser VM (memo table, budget, expectations and
the like) and holds the structured code over them: the algorithms that every runtime repeats, written once, and the
grammar code lowered from Flow IR. The Go and TypeScript printers print Exec IR, so the rules of a generated parser and
the shared parts of the generated runtimes come from the same representation. Optimizations that depend only on the
meaning of the grammar are rewrites of Core IR; the choice of loops and branches is made when lowering to Flow IR;
passes on Exec IR keep the behavior of a function while removing what a build does not need. The reasoning and the
contracts are in [Core, Flow and Exec IR](intermediate-representations.md). Only some backends use these
representations: the closure backend and the bytecode compiler work from the program, so a Core IR change does not by
itself change every backend; [record 062](records/062-backend-independent-optimization-ir.md) states the intended
reach and [record 065](records/065-exec-ir-parser-vm.md) the design of Exec IR.

## Values, Documents and Streams

These three mechanisms change how the engine treats input and results, so they cut across the stages.

- **Values.** A rule either builds a value or only recognizes. The compile stage decides which, per rule, so that a
  recognizing path pays nothing for values it would discard. Typed output is the same decision made with the
  grammar's declared types.
- **Documents.** A Document keeps the input and the memo tables between edits, so after a change only the rules whose
  examined range was touched run again. This requires that every rule call keep exact accounting of what it read; a
  Document therefore parses on the path that keeps that accounting, while a one-shot parse may skip it where nothing
  can observe the difference.
- **Streams.** A stream commits the elements of a top-level repetition and drops the input behind them, so memory
  follows the unconsumed input rather than the whole input. This is why a stream is allowed only where nothing later
  can need what was dropped.

## Observability

Tracing, profiling, explanations and compute limits are not afterthoughts bolted on at run time; every backend that can
run a program also has a way to report what it did. The design rule is that the cost when observation is off must be
a single predictable check, and that observation never changes what the parser accepts or returns. Generated code gets
the same features through variants of the generated text, so the plain build stays free of them. The user-facing
contract is [Observability](../../spec/runtime/observability.md).

## Invariants That Cut Across Packages

- **One behavior.** Every backend returns the same value, the same error and the same expectations at the same
  position for the same grammar and input. A backend that cannot meet this says so (the closure backend alone supports
  `#missing`) instead of approximating.
- **Specification first.** The specification states the contract; the engine, the runtime and the generators
  implement it. A behavior that only one of them has is a defect or a documented limit.
- **Generated code mirrors the engine.** The Go runtime used by generated parsers must match the engine's behavior,
  and tests compare them. Names that generated code calls are not a public API.
- **Analyses are shared.** Facts such as nullability and first terminals have one implementation that the compiler, the
  linter and the optimizer all use.
- **Positions and units.** Input is measured in one chosen unit (code points or bytes) everywhere a position,
  expectation or edit is reported.
- **Not stable before release.** Compiled files, JSON forms and bytecode may change between versions; there are no
  compatibility shims.

## Where to Look

| To work on | Start in |
|:--|:--|
| Grammar syntax, text and JSON forms | `internal/syntax/`, `grammar/` |
| Desugaring: inline actions to hidden rules, attribute normalization, start rules | `internal/desugar/` |
| Compilation, analysis, checking and the bytecode compiler | `internal/compile/`; the type model the checker uses in `internal/types/` |
| The front end: a grammar checked to its program, Core IR and facts, which tools and generators read without a runtime backend | `internal/frontend/`; the program comes from checking in `internal/compile/`, which also lowers it to closures |
| The parser runtime: compilation entry points, streams and limit controls, over the backends and the Document | `internal/engine/` |
| The parse state that every backend shares, and what a backend provides | `internal/engine/core/` |
| The closure backend | `internal/engine/closure/` |
| The bytecode VMs, recursive and iterative | `internal/engine/vm/` |
| The incremental Document, its snapshots and change reports | `internal/engine/document/` |
| Output schemas and structural queries | `schema/` (the types, their validation and `Compare`), `internal/schema/` (the export), `query/` (the queries) |
| Core, Flow and Exec IR and their passes | `internal/ir/core/`, `internal/ir/flow/`, `internal/ir/exec/`; the pass runner in `internal/ir/pipeline/`, the Core builder in `internal/ir/core/build/`, the Flow lowering in `internal/ir/flow/lower/` and the Exec lowering in `internal/ir/exec/lower/` |
| Generated Go and TypeScript | The public entry points in `gen/`, the Go generator in `internal/gen/gogen/`, the TypeScript generator in `internal/gen/tsgen/`, the optimization settings in `internal/gen/opt/` and the runtimes in `runtime/` and `internal/gen/tsgen/tsrt/` |
| The public API | `zp.go`, `grammar/` |
| Commands, linter, language server | `cmd/zp/`, `lint/`, `internal/lsp/` |
| Playground and site | `cmd/playground/`, `site/` |

## Related Topics

- [Core, Flow and Exec IR](intermediate-representations.md): the representations between a program and generated code.
- [Design records](records/): decisions and their alternatives.
- [Development](../development.md): repository layout, tests and checks.
- [Optimization catalog](../optimizations/README.md): measured mechanisms.
