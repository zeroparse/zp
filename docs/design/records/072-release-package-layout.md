# 072. Release Package Layout

- **Status**: Accepted
- **Author**: @ornew
- **Date**: 2026-11-16

## Summary

ZeroParse is released with one package layout in which a public layer `pkg/` is the only Go API, the runtime side of the
module never links the compiler side, and the middle of the pipeline is one Code IR with three dialects. This record
fixes that layout as the **target**. The tree does not have it yet: the repository is being moved to it in stages,
listed under [Implementation Sequence](#implementation-sequence), and until a stage lands the packages of that stage
keep their current names. Documents that describe today's tree describe today's tree; this record describes where it is
going and why.

## Motivation

The packages grew with the features, and three problems follow from how they grew.

- **The public surface is spread over the root.** Public packages sit beside internal ones with no common prefix, so
  neither a reader nor a tool can tell the API from the rest, and a package that carries both runtime and compiler
  vocabulary (one package joins the runtime to the output schema and also compiles) has no single reason to change.
- **The runtime links the compiler.** A parser loaded from a compiled file, or a generated parser, needs only the parts
  that run a parse. Nothing forbids it from importing the grammar checker, so the dependency can grow by accident and
  the size of a loaded parser follows.
- **The middle of the pipeline has grown by accretion.** The Flow and Exec representations and the passes between them each define their
  own structure, printer and verifier, the pass runners are per-pipeline, and facts about rules are computed in more than
  one place.

## Goals

- One place for the public Go API, with a stable-in-intent contract, apart from everything internal.
- A runtime that cannot depend on the compiler, enforced mechanically.
- Every piece of knowledge has one home: a rule's policy flags and site numbers come from Core IR facts, and each
  structure has one printer and one verifier.
- A pass model that every pipeline shares, so that a pass says what it needs, what it keeps and when it applies.

## Non-Goals

- Changing the grammar language, the output schema, the compiled-file format or generated code. The migration keeps
  parse behavior and generated output the same unless a stage says otherwise and measures it.
- Stability beyond what the release promises. The layout is the shape of the packages, not a promise that every
  exported name inside them is frozen.

## Design

### The Public Layer

Everything a user imports lives under `pkg/`; everything else is `internal/` or a command. The packages are cut by the
vocabulary they own, not by the stage that implements them.

- `pkg/grammar`: the grammar AST and its JSON form, loading a grammar from a named, typed source, formatting and
  conversion. The AST follows the language, which changes slowly, rather than an IR, which changes often.
- `pkg/compiler`: turning a grammar into something executable: the default parser on the bytecode VM, a reference
  parser for differential testing, a compiled file, generated Go and TypeScript, and the output schema. One optimization
  setting (speed, memory or size, with per-pass overrides) applies to every backend.
- `pkg/lint`, `pkg/sample` and `pkg/editfuzz`: the checks and the complexity classification, input generation and edit
  fuzzing. Each has its own vocabulary (check names and severities, generation options, an edit-sequence format), so each
  is a small package rather than a part of the compiler.
- `pkg/bytecode`: the instruction set and the compiled-file codec. It is public but marked unstable until release.
- `pkg/runtime` and its subpackages: the parser interface, options and limits, the input model, the parse values and
  their nodes, the output schema, structural queries, the instrumentation model, the VM loader and the support library
  that generated Go parsers import. The node representation sits below the value package because a generated parser
  copied into a user's module imports it, which an internal package cannot allow.

A grammar source is a value with a name and a format, read once at compile time. It is separate from the parse input,
which has positions, streaming, edits and budgets; the two share only the naming convention of their constructors.

### The Runtime and Compiler Sides

The module is divided into a **runtime side** (`pkg/runtime/**`, `pkg/bytecode` and `internal/runtime/**`) and a
**compiler side** (everything else). The compiler side may use the runtime side; the runtime side must not reach the
compiler side, and must not import third-party packages. This rule has the following consequences.

- Loading a compiled file never constructs a grammar AST or runs the type checker, because the runtime side cannot name
  them. A compiled file holds a closed instruction set and the tables a parse needs, not a grammar.
- A generated Go parser imports only the runtime-side packages that its library lists, never the VM or the engine, so a
  generated parser does not link a second parser runtime.
- The reference parser needs Core IR, so it belongs to the compiler side and is public only as a constructor of
  `pkg/compiler`.

The rule is enforced by a test that lists the dependencies of every package under each build tag set the repository
supports and prints the shortest import chain of a violation. The test carries an allowlist of the violations that remain
during the migration; the list must match the violations exactly, so it can only shrink, and it is empty when the
runtime separation lands. A deliberately wrong edge in the test is a negative control. The layers inside the compiler
side are ordered the same way: IR packages import no pass or backend, the front end imports no middle end or backend,
and only commands import the language server.

### One Code IR with Three Dialects

There are two internal representations, not three.

- **Core IR** holds the meaning of a grammar after desugaring: typed, with explicit facts, and with no knowledge of the
  mode (budgets, Documents, observation) or the target. Rewrites of meaning and the facts about rules, such as which
  rules need memo records or can be inlined, belong here.
- **Code IR** is one structured-code representation (modules, functions, statements, expressions) with a verifier, a
  printer and a reference interpreter, shared by everything below Core. What differs between levels is the table of
  operations, which is data, in three dialects:
  - the **Flow** dialect has the operations of grammar control: matching, marking and restoring for backtracking,
    expectations, captures, rule calls under a policy, charging for budgets and recovery;
  - the **Action** dialect has the operations that evaluate action expressions (value actions, predicate actions and
    bindings), and is valid both while the code is still Flow and after it is Exec;
  - the **Exec** dialect has the operations of the abstract parser machine and the compositions built on them: the call
    protocol, memoization, left-recursion growth, operator rules, Documents and streams.

The terms are fixed. A dialect is a set of operations; a program state is a stage of a Code IR module. Write "Code IR at
the Flow stage" and "Code IR at the Exec stage", or "the Flow stage" and "the Exec stage" once the context is clear, and
do not call either one "Flow IR" or "Exec IR". The pages that use those names for today's separate representations move to
these terms when Code IR is introduced. The abstract machine
is what the Exec dialect defines; "the VM" is only the bytecode interpreter. A dialect is data over the shared base, so
each operation states its contract, its cost, the features it needs and its laws, and the operations of Exec have a
reference model that a conformance suite checks every implementation against.

Lowering between levels is a separate step with its own invariant, never a pass. Core is lowered to Flow once, in a
uniform shape; the choice of shape is made by Flow passes. Flow is lowered to Exec completely, and the verifier checks
that no Flow operation remains. The bytecode compiler selects instructions from the Flow stage, because the instruction
set is at the level of grammar control and most instructions come from one Flow operation each; a collection of marks
maps to one choice instruction, which over-approximates safely. Go and TypeScript are produced from the Exec stage, after
language-specific passes that legalize and optimize it for the target, and then printed.

### The Pass Model

A pass is a named unit with a declared shape, and a manager runs a list of them.

- **Registration.** A pass declares its name (the same as its flag), its default, its applicability condition (a
  feature such as budgeting, Documents or tracing), the property it must preserve (its obligation), the analyses it
  needs and the ones it keeps. The registry is shared; each pipeline (Core, Flow, Exec and each target language)
  registers its own passes, and printers and library controls register under their own kind, so no name is declared
  twice.
- **Analyses.** An analysis is computed when a pass asks for it and cached until a pass that does not preserve it runs.
  Facts about rules are computed once on Core and read by the generators, the engine, the linter and the complexity
  classifier. An analysis that fails returns an error; it never degrades silently.
- **Verification.** After a pass, the stage's verifier runs on request and names the pass that broke an invariant. The
  Flow verifier checks structure, variables, one runtime per function, the budget invariants and the rules about reading
  input under a Document or a trace. A dump after any stage (`core`, `flow`, `exec`) shows the module as the next stage
  receives it.
- **Order and decisions.** Control-level decisions (call specialization, inlining, dispatch, the form of a match, removal
  of redundant saves, merging failure paths, where to charge, sharing slots and outlining) are Flow passes with stated
  obligations; target-specific legalization is in the language pipelines; passes that depend only on an operation's reads
  and writes are shared across dialects.
- **Profiles.** A user chooses speed, memory or size. The table from profile to pass flags lives in the driver, and the
  public option carries the profile and the overrides.

The names of lowering steps follow LLVM and MLIR: lowering between dialects is *lower*, and *legalize* is used only for
the adjustments inside a target.

### Layers Inside the Compiler Side

The compiler side is ordered as a pipeline: the front end (parse, desugar, check, elaborate), the middle end (Core passes,
the lowerings, Flow passes, shared Exec passes), the backends (reference parser, bytecode lowering, Go and TypeScript
generators) and the analysis tools. A driver runs the stages, and the commands and `pkg/compiler` call the driver.
The semantic model that answers source-located questions (the language server, lint findings, type errors) is one
place; it reads the grammar AST and does not transform it.

## Implementation Sequence

The layout is reached in stages, each of which keeps the behavior of the tree and is checked by the tests of the changed
packages, the import-graph test and, for stages that touch the engine, generator or runtime, the full gate with generated
output and the ready-made parsers. Moves of packages are separate commits from changes of behavior.

1. **Enforce the dependency rule.** Add the import-graph test with the allowlist of today's violations.
2. **Correct the documents** that describe the tree, and record this layout.
3. **Reorganize the front end, analysis and test support.** Move the checker, the type model, the facts and the schema
   export, and the analysis tools and the language server, into the compiler-side layout.
4. **Reorganize the IR and the passes.** Move the pass infrastructure to the shared module, make the Exec infrastructure
   the Code IR base, and separate the Core passes, the lowering and the shared passes.
5. **Unify the rule facts.** Make the generators and the engine read policy flags and site numbers from Core facts and
   remove the grammar-AST analysis they duplicated.
6. **Reorganize the backends.**
7. **Separate the runtime.** Introduce the runtime-side program, lower action expressions at compile time, remove the
   checked-program type, and move the runtime packages under `internal/runtime`. The allowlist becomes empty and the
   performance of the loaded parser is measured against the previous one.
8. **Publish the layer.** Introduce the public packages, first the tool packages (grammar, lint, sample, editfuzz, schema,
   query), then the runtime and compiler packages together with the generated-parser library and its embedded copy, and
   remove the old public packages. The same stage settles the single term for checking input without building values
   across the commands, the API and the documents.
9. **Make Flow and Action dialects of Code IR** and give every operation its contract; then move control-level decisions
   into Flow passes and language-specific legalization into the language pipelines.
10. **Select bytecode from the Flow stage** and let the VM implement the compositions natively. The switch happens only
    when the size of compiled files and the throughput of the VM are within the noise of the previous construction.

Each stage lands with the specification, guides and design pages that it changes. Specifications and guides describe
public names only after the stage that introduces them.

## Alternatives Considered

- **Keep three representations, with Flow and Exec as separate IRs.** Each representation needs its own structure,
  printer, verifier and interpreter, and the shared parts (action evaluation, the call protocol) are written twice. One
  base with dialects states each of these once and lets a verifier check the relation between stages.
- **Make Flow a control-flow graph.** Flow is a structured language over the shared base. A graph is built when a pass
  needs it, as an analysis, so that the representation that is printed and verified stays structured.
- **One public compiler package for every tool.** The linter, the sampler and the edit fuzzer would then share a package
  with options that have nothing in common. Small packages with their own vocabulary are easier to use and to keep stable.
- **Enforce the runtime separation by review.** Dependencies grow silently and the loss shows up as binary size. A test
  that fails on the first wrong edge is cheaper and does not depend on who reviews.

## Testing

The import-graph test covers the runtime separation, the generated-parser dependency rule and the layer order, with a
negative control. Each stage is checked by a generated-output comparison against the previous tree, the differential
and conformance suites, and the documentation tests. The stage that selects bytecode from Flow, and the one that
separates the runtime, are also measured with interleaved runs against a copy of the previous tree.

## Limitations and Open Questions

The layout is the target. The tree still has the current public packages, the pass runners per pipeline, and the
violations in the allowlist until the stages above land; this record changes status as they do. The signatures in the
public layer are fixed stage by stage in the package documentation, not here.
