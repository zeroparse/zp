# ZeroParse

ZeroParse is a Go parser framework using an extended PEG grammar language. Keep this
file concise; put contracts, design rationale, measurements and workflow detail
in the linked project documents.

At the start of every development session, use the `onboarding` skill in
`.agents/skills/onboarding/` to locate current sources and choose relevant checks.

## Documentation map

- `README.md`: project overview and documentation index.
- `spec/`: normative specifications: `language/` (grammar), `runtime/` (parser and runtime),
  `core-ir/` and `bytecode/`.
- `docs/tutorial/`, `docs/guide/`, `docs/cookbook/`: learning and usage.
- `docs/design/`: architecture and internal representations as implemented;
  `docs/design/records/`: decisions; `docs/optimizations/`: measured mechanisms.
- `docs/development.md`, `docs/site.md`, `docs/performance.md`: development,
  site maintenance and performance method/status.
- `docs/why-zeroparse.md`, `docs/strategy.md`: positioning, comparison and project
  strategy, including performance targets.
- `site/`: ZeroSite configuration and wrapper; `bench/`: benchmark sources.

Use the `documentation`, `performance`, `backlog` and `zerosite` skills when
relevant.
Keep reader-facing docs independent of chat history and backlog labels. Write
repository docs and comments in English. Update affected contracts and guides
with implementation changes; regenerate generated files.

## Development and checks

- Public APIs: `zp.go` (which joins the runtime to the output schema),
  `gen/` (code generation), `lint/`, `grammar/`, `sample/`, `editfuzz/`; parser/compiler: `internal/syntax/`,
  `internal/compile/`; runtime: `internal/engine/`; generators:
  `internal/gen/`; grammars: `testdata/grammars/`; ready-made parsers: the separate
  `alpha-parsers` repository.
- Use Go 1.27.1. Choose checks by what a change touches. Docs or spec only:
  `cd site && go test ./...` and the site build/check in `docs/site.md`. Code:
  `go vet` and the tests of the changed packages and their dependents. Engine,
  generator or runtime: also `go generate ./gen ./bench` and
  `hack/test-alpha-parsers.sh` (it fails without an alpha-parsers checkout;
  `hack/sync-alpha-parsers.sh` updates one); budget-control code: also the reference build
  in `docs/development.md`. The default root `go test ./... ./runtime/...` takes
  minutes; run it on the final commit of each landing batch, and add
  `-tags zp_long` (the long tier in `docs/development.md`) when the batch touches
  the engine, generator or runtime. New slow tests go to the long tier.
- Keep generated benchmark reports/results unedited by hand and align
  `spec/bytecode/README.md` with VM changes.
- GitHub Actions are temporarily paused: do not trigger, rerun, cancel, poll or
  enable workflows. Keep 32-bit and ready-made parser work deferred.
  Required local validation still applies.
- Ask the owner about unspecified language semantics, with alternatives,
  tradeoffs and a recommendation; wait for answers before dependent work.
- Reuse the established Luna documentation/backlog sidekick and independent
  reviewer where appropriate. The primary owns implementation, tests, site
  code, Git operations and main integration; the documentation sidekick owns
  documentation context and backlog issue management after handoff.

- For high-impact changes, request critical review from a reviewer whose context
  is independent of the worker; reuse existing reviewers. Share requirements,
  diffs and validation evidence, and resolve material findings before completion.

## Backlog, commits and branches

- The backlog is one issue per item in this repository, tracked in the public
  [ZeroParse backlog project](https://github.com/orgs/zeroparse/projects/2)
  (Category, Tier, Status, Priority); use the `backlog` skill. Issue
  [#1](https://github.com/zeroparse/zp/issues/1) is the live work status page.
  The body states the current truth, comments log decisions and measurements as
  the work happens, and an issue closes by hand only after verified
  integration. Issue numbers do not appear in reader-facing docs.
- Write commits, issues and PRs in the owner's voice: omit references to agents,
  AI assistance, the owner/user, requests, delegation or work history; cite
  the backlog issue as `Refs #N`, never `Closes`. Follow
  `docs/commit-messages.md`; include a body explaining the problem and rationale.
  Do not add agent signatures.
- Branch prefixes: `feat/`, `fix/`, `perf/`, `docs/`; do not use `codex/` or
  `claude/`. Standing permission allows pushes to main and work branches, but
  follow the current task's scope and authorization. Remove remote branches
  only after verifying integration.
