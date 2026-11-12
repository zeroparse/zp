---
name: backlog
description: Keep ZeroParse backlog issues and the Issue #1 work-status page current. Use when starting, finishing, pausing, reviewing or deciding on a backlog issue, filing a new one, or editing Issue #1.
---

# Backlog

## Model

- The backlog is one GitHub issue per item, all in the public project
  https://github.com/orgs/zeroparse/projects/2. Project fields: Status (Backlog, Ready,
  In progress, Pending, In review, Done; Pending means started but not moving because it
  waits on another issue or a landing, or is paused on purpose), Start date, Target date, Tier (Core P0-P3, After the
  core), Category (Bug, Optimization, Feature, Improvement, Quality, Docs, Ready-made
  parser, Release), Priority and Legacy ID (the former two-digit ID, kept for lookup
  only; never allocate new ones). Milestones are M1 and M2 (and M3 for release).
- Parent issues are epics: sub-issues carry the work and real blocked-by links carry
  dependencies. The project's Roadmap view relies on correct Start and Target dates, so
  dates are part of the work, not decoration. Features normally require a design record.
- Issue #1 is the work-status page, not a backlog list: release estimate, milestones,
  lane order, in progress, waiting, and the ten most recent completions. It has no
  status comment. The commit log is the full record of completed work.
- An issue states the problem, decisions with links, remaining work and its acceptance
  criterion; trim landed history from it. Decisions and measurements live in comments.
- Check current state with `gh issue view <n>`, `gh project field-list 2 --owner zeroparse`
  and `gh project item-list 2 --owner zeroparse`.

## Issue lifecycle

Decisions left only in chat are lost to compaction and get asked again, so do each step
in the same step as its event, never later.

- **Start**: rewrite the body (problem, acceptance criterion, decisions with links, a
  checklist of planned steps, the branch). Set Status=In progress, Start date=today
  (JST) and Target date=the planned end. Link the parent and blocked-by issues.
- **Decision**: post a "Decided: ..." comment at once and update the body's decisions.
- **Progress**: tick the checklist; revise the Target date when the plan moves.
- **Blocked/waiting**: set Status=Pending, add a blocked-by link and comment what it
  waits on.
- **Ready**: set Status=In review (under review, or reviewed and queued to land) and
  comment the branch, SHA and checks.
- **Landed/close**: set Status=Done and Target date=the close date (JST). Add the commit
  link and one outcome sentence to Recently completed in the Issue #1 body (drop the
  oldest beyond ten). Fix the parent epic's dates. Keep any unfinished scope open.
- **Pause**: set Status=Pending and comment the reason and where the WIP lives
  (branch); keep the Target date realistic.
- **New issue**: use the .github/ISSUE_TEMPLATE form for its Category (epic.yml for an epic; `gh issue create
  --template` or the same headings), add it to project 2, and set every field (Tier,
  Category, Milestone, Status, dates, parent and blocked-by) before reporting it filed.

Every Status change updates Status, the Start/Target dates and the blocked-by or parent
links in the same step.

Members keep their own issues current but do not edit the Issue #1 body; the backlog
writer owns it, plus the Landed/close step if the integrator has not done it.

## Rules

- Preserve incomplete parent scope. Complete an item only after acceptance gates pass
  and integration is verified.
- Issue numbers and legacy IDs belong in issues and this skill only. Never put them in
  reader docs (spec, guides, cookbook, tutorial), design or optimization records, or
  commits other than `Refs #N`; link those documents descriptively instead.
- Fetch before every edit and read back the affected surface afterwards. Keep local
  checks, remote CI, integration and deployment evidence distinct. Do not trigger or
  poll GitHub Actions while the standing workflow pause applies.
- Each issue appears once in the Issue #1 lane lists; compare issue-number sets before
  uploading a body.
