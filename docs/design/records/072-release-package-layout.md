# 072. Release Package Layout

- **Status**: Accepted
- **Author**: @ornew
- **Date**: 2026-11-16

> [!WARNING]
> The package layout is being migrated to the target this record decides, so parts of the current code still differ
> from it. [Migration](../architecture.md#migration) lists the stages that reach the target.

## Summary

ZeroParse is released with one package layout. A public layer `pkg/` is the only Go API, the runtime side of the module
never links the compiler side, and the middle of the pipeline is one Code IR with three dialects. This record fixes that
layout as the **target** and gives the reasons for it. The tree does not have it yet: the repository is being moved to
it in stages, listed under [Implementation Sequence](#implementation-sequence), and until a stage lands the packages of
that stage keep their current names. The complete architecture (the package tree with the responsibility of each
package, the public API, the import rules, the pass model, the driver and the stages in detail) is
[Architecture](../architecture.md); the design pages describe that target and say at their top that the code is still
being migrated to it. This record keeps the decision: the problems, what was decided, why, and what was rejected.

## Motivation

The packages grew with the features, and four problems follow from how they grew.

- **The public surface is spread over the root.** Public packages sit beside internal ones with no common prefix, so
  neither a reader nor a tool can tell the API from the rest, and a package that carries both runtime and compiler
  vocabulary (one package joins the runtime to the output schema and also compiles) has no single reason to change.
- **The runtime links the compiler.** A parser loaded from a compiled file, or a generated parser, needs only the parts
  that run a parse. Nothing forbids it from importing the grammar checker, so the dependency can grow by accident and
  the size of a loaded parser follows.
- **The middle of the pipeline has grown by accretion.** The Flow and Exec representations and the passes between them
  each define their own structure, printer and verifier, the pass runners are per-pipeline, and facts about rules are
  computed in more than one place.
- **Knowledge has several addresses.** A checked program, the Core IR and the runtime tables each hold part of what a
  rule is, and one idea ("check input without building a tree") has several names across commands, API and documents.

## Goals

- One place for the public Go API, with a stable-in-intent contract, apart from everything internal.
- A runtime that cannot depend on the compiler, enforced mechanically.
- Every piece of knowledge has one home, and each structure has one printer and one verifier. A rule's policy flags and
  site numbers come from Core IR facts. There is no "checked program" layer: its content lives in the semantic model,
  Core IR and the runtime program.
- A pass model that every pipeline shares, so that a pass says what it needs, what it keeps and when it applies.

## Non-Goals

- Changing what a parse does. Parse results, errors, recovery, limits and the output schema keep their behavior; stages
  that move packages leave generated output unchanged, and a stage that changes it measures and explains the change.
- Stability beyond what the release promises. The layout is the shape of the packages, not a promise that every
  exported name inside them is frozen.

The layout does change some names and formats, and these are decided parts of it rather than side effects: the field
`"term"` of the grammar JSON and the syntax symbol of an action expression become `"action"` and `=> expr` (see
[Naming](#naming)); the import paths of generated Go parsers change when the public runtime packages are introduced; a
compiled file becomes the serialization of the runtime program and a closed instruction set; and bytecode is selected
from Code IR at the Flow stage. None of these is kept compatible with the earlier form before the first release.

## Design

The sections below state each decision with its reason. The full package tree, the signatures of the public API and the
code of the pass model are in [Architecture](../architecture.md).

### The Public Layer

Everything a user imports lives under `pkg/`; everything else is `internal/` or a command. The packages are cut by the
vocabulary they own, not by the stage that implements them.

- `pkg/grammar`: the grammar AST and its JSON form, loading, formatting and conversion. The AST follows the language,
  which changes slowly, rather than an IR, which changes often, and the front end receives it as the grammar syntax
  tree (GST). A grammar source is a value with a name and a format, made by `FromBytes`, `FromString`, `FromReader` or
  `FromFS` and given an explicit format with `As`. `Load` reads `.zp` and JSON and rejects a compiled file with an
  explicit error; the command line reads grammars through the same constructors. A grammar source is separate from the
  parse input, which has positions, streaming, edits and budgets; the two share only the naming convention of their
  constructors.
- `pkg/compiler`: turning a grammar into something executable. `Compile` returns the parser on the bytecode VM, the
  default backend; `CompileGrammar` takes an AST built by a program; `NewReferenceParser` returns the reference parser
  for differential testing; `Marshal` writes a compiled file; `GenerateGo` and `GenerateTypeScript` write source; and
  `OutputSchema` returns the schema. One `Optimize` setting, with the profiles `Speed`, `Memory` and `Size` and per-pass
  overrides, applies to every backend alike.
- `pkg/lint`, `pkg/sample` and `pkg/editfuzz`: the checks and the complexity classification, input generation, and edit
  fuzzing. Each has its own vocabulary (check names and severities, generation options, a versioned edit-sequence
  format), so each is a small package rather than a part of the compiler.
- `pkg/bytecode`: the instruction set and the compiled-file codec. It is public but marked unstable until release.
- `pkg/runtime` holds the parser interface, options, limits and statistics, and has these subpackages: `input` (position
  units, locations, text edits and change kinds), `value` (nodes, cursors, traversal, JSON, syntax errors), `value/impl`
  (the node representation, package `valueimpl`), `schema` (the output schema, its JSON form, validation and `Compare`),
  `query` (structural queries), `instrument` (trace events, filters, depth and the per-rule profile), `vm` (loading a
  compiled file into a parser) and `golib` (the support declarations that generated Go parsers import; it is not an API
  for direct calls). The node representation sits below the value package because a generated parser copied into a
  user's module imports it, which an internal package cannot allow. The output schema is runtime-side because a parser,
  structural queries and the typed API use it while running, and `Compare`, a pure function over schemas, lives with
  the types. The library of generated Go is a package of its own, apart from `pkg/runtime`, because it holds many
  support declarations that the library protocol versions, and keeping them out of the user API avoids clashes of names
  such as `Parser`, `Document` and `ParseOption`.

### The Runtime and Compiler Sides

The module is divided into a **runtime side** (`pkg/runtime/**`, `pkg/bytecode` and `internal/runtime/**`) and a
**compiler side** (everything else). The compiler side may use the runtime side; the runtime side must not reach the
compiler side. The rules, which a test enforces, are these.

1. **Rule 1.** For every non-test package P on the runtime side, every package of the module in the dependencies of P is
   also on the runtime side, and the runtime side imports no third-party package.
2. **Rule 1a (generated parsers).** The generated-parser library `golib` imports only `pkg/runtime`, `input`, `value`,
   `value/impl` and `instrument` (and `schema` where the typed API needs it); it imports neither `vm` nor
   `internal/runtime/**`. A generated parser therefore links neither the VM nor the engine. The embedded copy of the
   library compiles by itself in a temporary module that imitates a user's module, and the test checks parsers generated
   in both library modes (an imported library and an embedded one).
3. **Rule 2 (layers).**
   - `internal/pass` imports no other package of the module.
   - The IR packages (Core IR and Code IR with its dialects) import only IR packages, `internal/pass`, the cancellation
     helper and `pkg/grammar`. Inside them, the Code IR base imports no dialect; the Flow dialect imports the base and
     Core IR (for immediates) only; the Action dialect and the Exec dialect import the base only, and the packages
     under Exec may also import Exec and Action. Only the lowerings of the middle end and the checks that span
     dialects import more than one dialect.
   - The front end imports no middle end, backend, analysis tool or language server; the middle end imports no backend,
     analysis tool or language server; a backend imports no analysis tool or language server.
   - The utility packages (diagnostics, cancellation and command-line flag names) may import `pkg/grammar`, because
     diagnostics carry source positions. The fixture suite of `zp test` is not a utility: it is a tool that uses the
     compiler, in the same layer as the public tool packages.
   - Only the commands and the repository's test support import `internal/lsp/**`, and only test support imports
     `internal/testing/**` from non-test code.

The library printer prints the compositions of the Exec dialect into the generated-parser library and into the shared
engine state. That is an edge of generation through `go generate`, not an import edge, so it does not make the runtime
side depend on the printer; a freshness test keeps the printed files and the embedded copy current.

The consequences are concrete. Loading a compiled file never constructs a grammar AST or runs the type checker, because
the runtime side cannot name them: a compiled file holds a closed instruction set and the runtime program, not a
grammar. The reference parser needs Core IR, so it belongs to the compiler side and is public only as a constructor of
`pkg/compiler`.

The test lists the dependencies of every package under the default tags and under the reference tags, and prints the
shortest import chain of a violation. It carries an allowlist of the violations that remain during the migration; the
list must match the violations exactly, so it can only shrink, and it is empty when the runtime separation lands. A
deliberately wrong edge in the test is a negative control.

### The Internal Tree

The compiler side is a pipeline with a driver on top; the tree names the stages. The front end returns the semantic
model and Core IR; the facts about rules are computed by the Core analyses of the middle end, which a pass or a tool
asks for through the analysis manager, and the front end does not return them.

```mermaid
flowchart TD
  cmd["cmd/zp, cmd/playground"] --> pub["pkg/compiler, pkg/lint, pkg/sample, pkg/editfuzz"]
  pub --> driver["internal/driver"]
  cmd --> fixture["fixture suites of zp test"] --> pub
  driver --> backend["internal/backend: closure, vmlower, llvm (future), codegen/{common, golang, typescript}"]
  driver --> middle["internal/middleend: corepass, flowlower, flowpass, execlower, codepass"]
  driver --> front["internal/frontend: parser, desugar, check, types, astfacts, elaborate, schemaexport"]
  middle --> ir["IR packages: core, code/{flow, action, exec}"]
  backend --> ir
  front --> ir
  ir --> pass["internal/pass: Manager, AnalysisManager, Registry"]
  util["utilities: diagnostics, cancellation, command-line flag names"] --> gram["pkg/grammar"]
  ir --> gram
  cmd --> lsp["internal/lsp: protocol, zpls"] --> analysis["internal/analysis: lint, complexity"]
  driver --> analysis
  analysis --> backend
  backend --> middle
  middle --> front
  front --> gram
  driver --> rt["internal/runtime: program, state, document, vm, engine, nodes, limits"]
```

The IR packages hold data, attribute computation, verifiers and printers; the passes that transform and analyze are in
the middle end. A verifier checks that the facts agree with the structure by recomputing the attributes, so it lives
with the attribute computation. The dialects are nested
under the Code IR base, so the tree and the import paths say that they are dialects of one representation, and the
lowerings are packages of their own, outside the IR packages and apart from the passes of each dialect. Names are
chosen so that packages that are imported together do not clash: the grammar language server is not named like the
grammar package, and the compositions are not named like Core IR.

`internal/runtime/**` is the runtime side and holds the runtime program (the tables a parse needs, which a compiled file
serializes), the shared parser state, the incremental Document, the VM drivers, the entry points for parse, document
and stream use, and the budget arithmetic. Repository test support lives under `internal/testing`, and the architecture
test lives with it.

### One Code IR with Three Dialects

There are two internal representations, not three.

- **Core IR** holds the meaning of a grammar after desugaring: typed, with explicit facts, and with no knowledge of the
  mode (budgets, Documents, observation) or the target. Rewrites of meaning and the facts about rules, such as which
  rules need memo records or can be inlined, belong here. It is elaborated once from the semantic model, which answers
  every question tied to a source range (hover, lint findings, type errors) and transforms nothing.
- **Code IR** is one structured-code representation (modules, functions, statements, expressions) with a verifier, a
  printer and a reference interpreter, shared by everything below Core. What differs between levels is the table of
  operations, which is data, in three dialects:
  - the **Flow** dialect has the operations of grammar control: matching, marking and restoring for backtracking,
    expectations, captures, rule calls under a policy, charging for budgets and recovery;
  - the **Action** dialect has the operations that evaluate action expressions (value actions, predicate actions and
    bindings), and is valid both at the Flow stage and at the Exec stage;
  - the **Exec** dialect has the operations of the abstract parser machine and the compositions built on them: the call
    protocol, memoization, left-recursion growth, operator rules, Documents and streams.

The terms are fixed. A dialect is a set of operations; a program state is a stage of a Code IR module. Write "Code IR at
the Flow stage" and "Code IR at the Exec stage", or "the Flow stage" and "the Exec stage" once the context is clear, and
do not call either one "Flow IR" or "Exec IR". The pages that use those names for today's separate representations move
to these terms when Code IR is introduced. The abstract machine is what the Exec dialect defines; "the VM" is only the
bytecode interpreter.

A Flow operation states its contract, cost, the features it needs and its laws, and carries a reference expansion into
Exec operations. An Exec operation has a reference model, and a conformance suite compares every implementation with the
reference expansions and models.

Lowering between levels is a separate step with its own invariant, never a pass: `flowlower` takes Core to the Flow
stage in a uniform shape, and the Flow passes choose the shape; `execlower` takes the Flow stage to the Exec stage
completely, and the verifier checks that no Flow operation remains; `vmlower` selects bytecode. The names follow LLVM
and MLIR: lowering between dialects is *lower*, and *legalize* is used only for adjustments inside a target.

**Bytecode comes from the Flow stage.** The instruction set is at the level of grammar control, so instructions can be
chosen from Flow operations almost one for one, with superinstruction selection: a call of a composition is one
instruction and common match patterns fuse into high-level ones. A mark that names a set of state to save maps to the
choice instruction, a sound over-approximation. The compositions are native in the VM and checked by the Exec
conformance suite. The reason is that the VM, Go, TypeScript and a future LLVM backend then all take grammar code from
one Code IR, so charging and option propagation cannot drift between two lowerings of grammar code. Go and TypeScript
are produced from the Exec stage after language-specific passes that legalize and optimize it for the target, then
printed, and a printer prints the legalized Exec stage as it is, without carrying transformations as string
processing or regeneration. The closure backend deliberately does not share this lowering: it is built from Core IR so
that a bug in the lowering shows up as a difference between backends.

The complexity classifier works on its own Core IR module, the cost view, derived by normalization laws. The proofs
that the classification is sound speak about the cost view, so it is kept apart from the Core module that the
optimizations rewrite; it reaches users through lint.

### The Pass Model

A pass is a named unit with a declared shape, and a manager runs a list of them.

- **Registration.** A pass declares its name (the same as its flag), its default, its applicability condition (a
  feature such as budgeting, Documents or tracing), the property it must preserve (its obligation), the ordering it
  needs, the analyses it needs and the ones it keeps. The registry is shared; each pipeline (Core, Flow, Exec and each
  target language) registers its own passes, and printers and library controls register under their own kind, so no name
  is declared twice.
- **Analyses.** An analysis is computed when a pass asks for it and cached. What a pass preserves is a set that defaults
  to nothing: forgetting to declare it invalidates everything, which is the safe side. An analysis key is registered
  by name, and a second key with the same name is rejected, so two analyses of one name and different result types
  cannot confuse the cache. Invalidating an analysis
  invalidates the analyses that used it, transitively; an analysis that depends on itself is an error; a transform that
  returns a different IR value invalidates the whole cache; a failed analysis returns an error and never degrades
  silently. Facts about rules are computed once on Core, through the analysis manager, and read by the generators, the
  engine, the linter and the complexity classifier. Attributes that live on Core nodes (nullability, first sets,
  effects) are invariants that the verifier recomputes, not cached analyses.
- **Verification.** After a pass, the stage's verifier runs on request and names the pass that broke an invariant. The
  Flow verifier checks structure, variables, one runtime per function, the budget invariants and the rules about reading
  input under a Document or a trace. A dump after any stage (`core`, `flow`, `exec`) shows the module as the next stage
  receives it.
- **Decisions.** Control-level decisions (call specialization, inlining, dispatch, the form of a match, removal of
  redundant saves, merging failure paths, where to charge, sharing slots and outlining) are Flow passes with stated
  obligations; target-specific legalization is in the language pipelines; passes that depend only on an operation's
  reads and writes are shared across dialects.
- **Profiles.** A user chooses speed, memory or size. The table from profile to pass flags lives in the driver, and the
  public option carries the profile and the overrides.

### Naming

- **Action expressions.** The grammar AST uses `ActionExpr` and `ActionGroup`, the JSON field is `"action"`, the syntax
  symbol is `=> expr`, and Core IR uses `ActionExpr`: all match the specification's Action Expression.
- **Recognition.** Recognizing input decides, without building a tree, whether the input matches the grammar, and
  returns the same success and the same syntax errors (recovered ones included) as a parse. Predicates, and the actions
  that predicates read, run; the runtime errors of other actions and the conversion to typed values are not checked, so
  a recognition can succeed where a parse fails. It is expressed only by `Parser.Recognize` and the command `parse
  -recognize`; there is no option that switches a parse into a recognition, and generated parsers offer the same
  choice. "Validate" and "validation" mean only the structural check of an artifact (a grammar AST, a schema, a
  module).

## Implementation Sequence

The layout is reached in stages that keep the behavior of the tree. Each stage is checked by the tests of the changed
packages and the import-graph test; stages that touch the engine, generator or runtime also pass the full gate with
generated output and the ready-made parsers. Moving a package is a separate commit from changing behavior. The stages,
with their content, prerequisites and acceptance checks, are in [Migration](../architecture.md#migration); in short:

- **S0.** Enforce the dependency rule: the import-graph test with the allowlist of today's violations.
- **S1.** Correct the documents that describe the tree, and record this layout.
- **S2.** Reorganize the front end, analysis and test support; the semantic-model part of the compile package moves into
  the checker and the type model, and the builder of Core IR moves to `elaborate` with them.
- **S3.** Reorganize the IR and the passes: `internal/pass` with a generic registry, driving in `internal/driver`, the
  Exec infrastructure as the Code IR base, `corepass`, `flowlower` and `codepass`, the rename of the Core IR `Term` to
  `ActionExpr`, and the `-print-after` stage `build` renamed to `core`.
- **S3a.** Unify the rule facts on Core IR.
- **S4.** Reorganize the backends.
- **S5.** Separate the runtime; the allowlist becomes empty.
- **S6a.** Publish the tool packages and rename action expressions in the AST, JSON and syntax.
- **S6b.** Publish the runtime and compiler packages; generated parsers satisfy rule 1a.
- **S6c.** One term for recognition, landed with S6b.
- **S7.** Implement the analysis manager and share facts among lint, complexity and the generators.
- **S7a.** Make Flow and Action dialects of Code IR, with `execlower` as the complete conversion.
- **S7b.** Move control-level decisions into Flow passes and register the language passes.
- **S8.** Select bytecode from the Flow stage, only once compiled-file size and VM throughput are within the noise of the
  previous construction.

The recommended order is S0 and S1, S2, S3, S3a and S7, S4, S6a, S5, S7a, S6b with S6c, S7b and S8; S7 and S7a touch
only the IR and the middle end and can run in parallel with S4 to S6. S0 to S6a do not change names that the
ready-made parsers import; S6b changes the import paths of generated code. Each stage lands with the specification,
guides and design pages that it changes, and specifications and guides describe public names only after the stage that
introduces them.

## Alternatives Considered

- **Keep three representations, with Flow and Exec as separate IRs.** Each representation needs its own structure,
  printer, verifier and interpreter, and the shared parts (action evaluation, the call protocol) are written twice. One
  base with dialects states each of these once and lets a verifier check the relation between stages.
- **Make Flow a control-flow graph.** Flow is a structured language over the shared base. A graph is built when a pass
  needs it, as an analysis, so that the representation that is printed and verified stays structured.
- **One public compiler package for every tool.** The linter, the sampler and the edit fuzzer would then share a package
  with options that have nothing in common. Small packages with their own vocabulary are easier to use and to keep
  stable.
- **Enforce the runtime separation by review.** Dependencies grow silently and the loss shows up as binary size. A test
  that fails on the first wrong edge is cheaper and does not depend on who reviews.
- **Let the closure backend share the lowering of grammar code.** One lowering for every backend would be less code,
  but the closure backend is the oracle of the differential tests, and an oracle that shares the lowering cannot reveal
  a bug in it. It is built from Core IR instead.
- **Keep generating bytecode from Core IR.** Bytecode would then come from a second lowering of grammar code beside the
  one that feeds Go and TypeScript, and charging and the propagation of options would have to be kept equal in both by
  hand. Selecting instructions from the Flow stage gives every practical backend one source of grammar code; the switch
  waits until size and throughput match.
- **Keep a checked-program layer.** A layer between the semantic model and Core IR held part of what a rule is, so the
  same knowledge had several homes and analyses were written on both sides. Its content is divided among the semantic
  model, Core IR and the runtime program.
- **Share one type for the grammar source and the parse input.** The parse input is about positions, streams, edits and
  budgets; a grammar source is a named, formatted set of files read once at compile time. A shared type would carry the
  concerns of both. They share only the naming convention of their constructors.
- **Classify complexity on the Core module of the optimizations.** The soundness proofs are stated over a module in a
  normal form, and the optimizations rewrite their module freely; one module could not serve both without the proofs
  tracking every rewrite. The cost view is a separate module.
- **Call the transformations between levels passes, or legalizations.** A lowering changes the dialect and has its own
  postcondition, so it is a separate step; the words follow LLVM and MLIR, with *lower* between dialects and *legalize*
  only inside a target.

## Testing

The import-graph test covers rules 1, 1a and 2 with a negative control. Each stage is checked by a generated-output
comparison against the previous tree, the differential and conformance suites, and the documentation tests. The stage
that selects bytecode from Flow, and the one that separates the runtime, are also measured with interleaved runs against
a copy of the previous tree.

## Limitations and Open Questions

The layout is the target. The tree still has the current public packages, the pass runners per pipeline, and the
violations in the allowlist until the stages above land; this record changes status as they do. The signatures of the
grammar source and `Load`, of `Compile`, of the query and instrument packages and of the pass infrastructure are
decided and given in [Architecture](../architecture.md#the-public-api); the other signatures of the public layer are
rough shapes, fixed stage by stage in the package documentation.
