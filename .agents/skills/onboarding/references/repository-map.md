# Repository Map and Source Authority

Start with [README](../../../../README.md) for the product and documentation index,
then [development](../../../../docs/development.md) for the current architecture,
directory layout and implementation status. Search the relevant package and its tests
before choosing an edit location:

| Concern | First stop |
|:--|:--|
| Public Go API and grammar AST | `engine/`, `pkg/grammar/ast/` |
| Grammar source parsing | `internal/frontend/parser/` |
| Compiler and static analysis | `internal/compile/` |
| Runtime: execution, bytecode VMs, Documents, streams and limit controls | `internal/engine/` (shared state in `state/`, the VMs in `vm/`, the Document in `document/`); backends in `internal/backend/` (`closure/`, `vmlower/` the bytecode compiler, `codegen/{common,golang,typescript}/`) |
| Code generation entry points | `codegen/` (`codegen.Go`, `codegen.TypeScript`, embedding the library sources `codegen/go/lib/` and `codegen/ts/lib/`), `internal/backend/codegen/common/` |
| Output schema and structural queries | `pkg/runtime/schema/` (the types, their validation and `Compare`), `internal/frontend/schemaexport/` (the export), `pkg/runtime/query/` (the queries) |
| Generated Go parser source | Generator `internal/backend/codegen/golang/print/generator.go` and library package `codegen/go/lib/` (imported or copied into generated parsers; depends only on `core/`); generated parsers under `internal/bench/parsers/` are outputs |
| CLI, linter, editor service and sample generation | `cmd/zp/`, `pkg/lint/`, `internal/lsp/`, `pkg/sample/` |
| Grammars of ten languages, playground and site | `testdata/grammars/`, `cmd/playground/`, `site/` (ready-made parsers: the separate alpha-parsers repository) |

Use [specification index](../../../../spec/README.md) for supported language and
parsing contracts. Use [design records](../../../../docs/design/records/000-template.md) for
decisions and proposals, and [development status](../../../../docs/development.md#language-feature-status)
to distinguish them from shipped behavior. Use the [guide index](../../../../docs/guide/README.md)
for user-facing workflows. For optimization mechanisms and applicability, use the
[catalog](../../../../docs/optimizations/README.md) and
[performance method](../../../../docs/performance.md).

The backlog (one issue per item, in the
[ZeroParse backlog project](https://github.com/orgs/zeroparse/projects/2)) and the work
status page [Issue #1](https://github.com/zeroparse/zp/issues/1) are not a language
contract. Fetch the present issue and its comments before planning or updating an
item; branch, worktree and integration status also need live checks.
Use the [documentation](../../documentation/SKILL.md),
[backlog](../../backlog/SKILL.md), [performance](../../performance/SKILL.md) or
[zerosite](../../zerosite/SKILL.md) skill when that work is in scope.
