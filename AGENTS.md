# ZeroParse

A Go parser framework with an extended PEG grammar language (Go 1.27.1). Read on every task:
contracts, rationale and measurements belong in the linked documents. Start with the
`onboarding` skill; use `documentation`, `performance`, `backlog`, `zerosite` and
`team-workflow` when they apply.

## Layout

A test (`internal/testing/archtest`) fails when a path in backticks here does not exist.

- `spec/` normative specs; `docs/` guides, `docs/design/` (as implemented) and
  `docs/design/records/` (decisions), `docs/optimizations/`, `docs/development.md`.
- Public packages: `core/`, `engine/`, `codegen/`, `schema/`, `query/`, `lint/`, `grammar/`,
  `sample/`, `editfuzz/`. Their release layout: `internal/testing/archtest/layout_test.go`.
- `internal/` implementation (frontend, compile, ir, bytecode, runtime engine, generators);
  `cmd/zp/` CLI; `testdata/grammars/` grammar corpus; `bench/`; `site/` (own Go module);
  `hack/` developer scripts; `.agents/scripts/` agent scripts. Ready-made parsers live in the
  separate alpha-parsers repository.

## Work

- Work in a git worktree on a `feat/*`, `fix/*`, `perf/*` or `docs/*` branch, never in the root
  checkout. Scratch goes in untracked `.scratch/`.
- Generated files (`go generate` output, benchmark reports) come only from their generators.
- Update affected specs and guides with the code, and `spec/bytecode/README.md` for VM changes.
  Docs and comments are English; reader-facing docs carry no chat history, backlog labels or
  issue numbers.
- Unspecified language semantics: check `docs/design/records/` and the issues, then ask the
  owner with alternatives and a recommendation; wait before dependent work.
- High-impact changes need review by a reviewer whose context is independent of the worker.

## Checks

`hack/gate.sh [--base REV] [--keep-going] LEVEL` stops at the first failing step; commit first,
it checks `REV..HEAD` (default `origin/main`). Levels: `docs` (docs, spec, site, skills);
`code` (build, vet, tests of affected packages); `engine` (engine, generator or runtime: long
tier, generation without diff, alpha-parsers, reference build); `full` (whole tree, long
tier). A lane runs `docs` or `code`; the landing batch runs `engine` or `full` on its last
commit. New slow tests go to the long tier (`zp_long`).

## Agent scripts

Run `-h` for usage. State is in `$ZP_AGENT_STATE`. Heavy runs go through the queue only.

- `.agents/scripts/lane-new.sh <name> <branch>`: start a lane worktree from fresh `origin/main`.
- `.agents/scripts/run-gate.sh [-t tag] LEVEL...`: run the gate in the right run class, logged.
- `.agents/scripts/testslot.sh <cmd...>`: targeted `go test` while developing.
- `.agents/scripts/benchlock.sh <tag> <cmd...>`: any other heavy or timed run, one at a time.
- `.agents/scripts/benchstop.sh <tag>`: stop your own job; never kill other processes.
- `.agents/scripts/queue-status.sh`: queue state; exit 1 when stuck.
- `.agents/scripts/alpha-check.sh [-c]`: sync and test alpha-parsers (`-c`: private clone).
- `.agents/scripts/push-branch.sh [branch...]`: check commits, push without force.
- `.agents/scripts/ready.sh "<scope>" "<checks>"`: append your READY line to the landing train.
- `.agents/scripts/train.sh stack|gate|land <name>`: integrator only.
- `hack/check-commits.sh [base]`: commit rules over `base..HEAD`; run before any push.

## Git and backlog

- Stage files by name; never `git add -A` or `git add .`; never force-push. Only the
  integrator pushes main. Remove a remote branch only after verifying it is integrated.
- Commits, issues and PRs: the owner's voice, English, with a body (`docs/commit-messages.md`),
  `Refs #N` never `Closes`, no mention of agents or work history, and no AI attribution.
- One issue per backlog item; [#1](https://github.com/zeroparse/zp/issues/1) is the live status
  page (categories B, O, F, Q, I, D, M). Close an issue by hand after verified integration.

GitHub Actions are paused: do not trigger, rerun, cancel, poll or enable workflows. 32-bit and
ready-made parser work are deferred.
