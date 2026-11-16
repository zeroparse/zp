# Design

How ZeroParse is built, and the records of the decisions that shaped it. Read these pages to find your way in the
compiler and the runtimes; read the [specification](../../spec/README.md) for what an implementation must do.

> [!WARNING]
> The package layout is being migrated to the architecture these pages describe, so parts of the current code still
> differ from them. [Migration](architecture.md#migration) lists the stages that reach the target.

| Page | Contents |
|:--|:--|
| [Architecture](architecture.md) | The architecture as a whole: the execution model, Core IR and Code IR with the pipeline between them, the backends, the tools and the command line, the Go packages with their public API and import rules, the pass model, the driver and the stages of the migration |
| [Core IR and Code IR](intermediate-representations.md) | Why there are two internal representations, how Code IR is divided into the Flow, Action and Exec dialects, and how optimization works on them, with a worked example |
| [The optimization pipeline](optimization-pipeline.md) | How the rewrites are organized: declared passes, the analysis manager, the registry of named optimizations, the `speed`, `memory` and `size` profiles, and verification |
| [Cost analysis](cost-analysis.md) | How `zp lint -classify` reaches its answer: the normal view of a grammar, the classifier, the exclusion analysis and the run records of the engine |
| [Design records](records/) | One record per decision, with its alternatives, validation and status; [record 072](records/072-release-package-layout.md) is the decision on the release package layout |

The pages above describe the ideas and invariants that the code does not show on its own; the code is the source of truth for details. A record explains why a decision was taken and may describe work that is
proposed or only partly implemented; its status says which: Proposed, Accepted, Partially Implemented, Implemented or
Superseded.
