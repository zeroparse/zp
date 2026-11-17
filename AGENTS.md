# ZeroParse

A Go parser framework with an extended PEG grammar language (Go 1.27.1). Read on every task.
Start with the `onboarding` skill; use `documentation`, `performance`, `backlog`, `zerosite`
and `team-workflow` when they apply.

## Layout

A test (`internal/testing/archtest`) fails when a path in backticks here does not exist.

- `spec/` normative specs; `docs/` guides, `docs/design/` (as implemented),
  `docs/design/records/` (decisions), `docs/optimizations/`, `docs/development.md`.
- Public packages: `core/`, `engine/`, `codegen/`, `schema/`, `query/`, `lint/`, `grammar/`,
  `sample/`, `editfuzz/` (release layout: `internal/testing/archtest/layout_test.go`).
- `internal/` implementation; `cmd/zp/` CLI; `testdata/grammars/` corpus; `bench/`; `site/`
  (own Go module); `hack/` and `.agents/scripts/` scripts. Ready-made parsers: the separate
  alpha-parsers repository.

## Work

- Work in a git worktree on a `feat/*`, `fix/*`, `perf/*` or `docs/*` branch, never in the
  root checkout. Scratch goes in untracked `.scratch/`.
- Generated files come only from their generators.
- Update affected specs and guides with the code (`spec/bytecode/README.md` for VM changes).
  Docs and comments are English; reader-facing docs carry no chat history, backlog labels or
  issue numbers.
- Unspecified language semantics: check `docs/design/records/` and the issues, then ask the
  owner with alternatives and a recommendation.
- High-impact changes need review by a reviewer independent of the worker.

## Checks

`hack/gate.sh [--base REV] LEVEL` stops at the first failing step and checks `REV..HEAD`, so
commit first. `docs`: docs, spec, site, skills. `code`: Go changes. `engine`: engine,
generator, runtime (long tier, generation, alpha-parsers, reference build). `full`: everything.
A lane runs `docs` or `code`; the landing batch runs `engine` or `full` once. New slow tests go
to the long tier (`zp_long`).

## Agent scripts

State is in `$ZP_AGENT_STATE`; `-h` prints usage. Heavy runs go through the queue only.

- Start a lane: `.agents/scripts/lane-new.sh <name> <branch>`
- Gate in the right run class: `.agents/scripts/run-gate.sh [-t tag] LEVEL...`
- Targeted `go test`: `.agents/scripts/testslot.sh <cmd...>`
- Any other heavy or timed run: `.agents/scripts/benchlock.sh <tag> <cmd...>`
- Stop your own job: `.agents/scripts/benchstop.sh <tag>`
- Queue stuck?: `.agents/scripts/queue-status.sh`
- Alpha-parsers check: `.agents/scripts/alpha-check.sh [-c]`
- Before and for a push: `hack/check-commits.sh [base]`, `.agents/scripts/push-branch.sh [branch...]`
- Submit to the landing train: `.agents/scripts/ready.sh [--base REV] "<scope>" "<checks>"`
- Integrator: `.agents/scripts/train.sh stack|gate|land <name>`

## Git and backlog

- Stage files by name, never `git add -A` or `git add .`. Never force-push. Only the
  integrator pushes main. Delete a remote branch only after verifying it is integrated.
- Commits, issues, PRs: the owner's voice, English, with a body (`docs/commit-messages.md`),
  `Refs #N` never `Closes`, no agent or work-history mentions, no AI attribution.
- [#1](https://github.com/zeroparse/zp/issues/1) is the live backlog status page (use the
  `backlog` skill); close an issue by hand after verified integration.

GitHub Actions are paused: do not trigger, rerun, cancel, poll or enable workflows. 32-bit and
ready-made parser work are deferred.
