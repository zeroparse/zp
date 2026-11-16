# Design

How ZeroParse is built, and the records of the decisions that shaped it. Read these pages to find your way in the
compiler and the runtimes; read the [specification](../../spec/README.md) for what an implementation must do.

> [!WARNING]
> The code is being migrated to the architecture these pages describe and still differs from it in places. The package
> comments describe the code as it is.

| Page | Contents |
|:--|:--|
| [Architecture](architecture.md) | The architecture as a whole: the execution model, Core IR and Code IR with the pipeline between them, the backends, the tools and the command line, the package layout and its placement principles, the driver flow and the invariants that cut across packages |
| [Core IR and Code IR](intermediate-representations.md) | Why there are two internal representations, how Code IR is divided into the Flow, Action and Exec dialects, and how optimization works on them, with examples of rewrites |
| [The pass pipeline](pass-pipeline.md) | How the passes are organized: declared passes, the pass model and analysis manager, the Flow passes, the dialect-independent passes of Code IR, the registry of named optimizations, the `speed`, `memory` and `size` profiles, host language passes, and verification |
| [Observability](observability.md) | The event and profile model shared by the backends, and what observation costs |
| [Cost analysis](cost-analysis.md) | How `zp lint` reaches its complexity classification: the normal view of a grammar, the classifier, the exclusion analysis and the run records of the engine |
| [Design records](records/) | One record per decision, with its alternatives, validation and status; [record 072](records/072-release-package-layout.md) is the decision on the release package layout |

The pages above describe the ideas and invariants that the code does not show on its own; the code is the source of truth for details. A record explains why a decision was taken and may describe work that is
proposed or only partly implemented; its status says which: Proposed, Accepted, Partially Implemented, Implemented or
Superseded.
