---
name: backlog
description: File, work, log and close ZeroParse backlog issues and keep the project and the Issue #1 status page current.
---

# Backlog

Every backlog item is one issue in `zeroparse/zp`. The public project
[ZeroParse backlog](https://github.com/orgs/zeroparse/projects/2) holds the fields
and views. [Issue #1](https://github.com/zeroparse/zp/issues/1) is the live work
status page. Fetch the issue and its comments before proposing or changing
anything, and read back what you saved.

## Filing

- File with the form of the matching category (`.github/ISSUE_TEMPLATE/`). The
  issue number is the only ID; there are no category IDs.
- Category, by the kind of change: Bug (defects), Optimization (changes that
  actually affect speed, memory or size, visible or not; enabling infrastructure
  with no performance effect of its own is Improvement), Feature (user-visible;
  nearly always has a design record, including the flag and profile mechanism
  that lets users control optimizations; one optimization exposed as a flag
  stays Optimization), Quality (tests, benchmarks, proofs, reference builds,
  flaky tests, developer-only tooling), Improvement (refactoring and
  reorganization other than for performance, and the precision of analyses),
  Docs (guides, specification text, design records; proofs are not
  documentation), Ready-made parser (a problem specific to one parser), Release
  (tagging, announcements, launch). Tools are categorized by the kind of change.
- Project fields: Category, Tier (order of the work; core first-release work
  before ecosystem, ready-made parsers and compatibility extensions), Status
  (Backlog, Ready, In progress, In review, Done), Priority (a number; lower
  first, within a tier). Labels: `paused` (reason and resume condition in a
  comment), `needs-decision` (waiting for the owner), `release-blocker`.
- Fix a wrong category by changing the field; the number stays.

## Body and comments

- The body is the current truth: the problem, the acceptance criterion, the
  decisions in force and a checklist of remaining work. Keep it current: tick
  the checklist as work lands and rewrite stale text instead of appending.
- Comments are the log: trials and dead ends; measurements with the numbers and
  where they came from; decisions with the alternatives and why each was
  rejected; review verdicts; progress with commit links. When a comment changes
  a decision, update the body and link the comment.
- Whoever works an item posts its decision and measurement log to the issue as
  the work happens, not only at the end. Write in the project's voice: no
  agents, AI or internal IDs; avoid private paths and paste the numbers
  themselves.

Example comment (numbers illustrative):

> Tried memoizing at the shared position only for rules that scan: peak memory
> +4.1% on the JSON corpus, parse time -0.8% (A/A spread 0.6%, 20 runs each,
> `bench/` JSON rows). Rejected: the memory cost is outside the budget in the
> acceptance criterion. Next: restrict it to rules the classifier marks as
> rescanning (checklist updated). Commit: abc1234.

## Splitting, merging and order

- Split by deliverable or acceptance criterion. When a checklist entry becomes
  its own stream or needs its own review or criterion, make it a sub-issue; a
  parent keeps the scope of its incomplete children and stays open until they
  close.
- Group naturally related items under an epic: a parent issue whose
  sub-issues are the items, with the goal, scope, children and completion
  criterion in its body. Reuse an existing issue as the parent when it already
  is the umbrella, and leave singletons alone. Use sub-issues, not labels, for
  epics. An epic's Tier and Priority are its earliest child's; its dates span
  its dated children.
- Use blocked-by only for real order (a stated prerequisite), never as a
  guess. Close a duplicate with a link to the survivor.
- Keep Priority and Tier current; the views group and sort by them. Keep the
  planned Start and Target dates of Core P1 and Core P2 issues in Tier and
  Priority order, so the Roadmap shows the sequence; later tiers stay undated
  until they come near.

## Working and closing

- Starting work begins by bringing the issue up to its template: the full
  problem with its context, the acceptance criterion, the decisions in force, the
  remaining-work checklist and links. Correct anything stale or wrong. Issues
  migrated from the former single-issue backlog carry only a short text until
  someone picks them up.
- Set Status when work starts and when it goes to review.
- Set the project's Start date when work starts (Status In progress) and a
  Target date as the planned finish, so the Roadmap view shows the work. Revise
  the Target date when the plan changes, and set it to the actual finish when
  the issue closes. Enrich the body at the start (see above) and keep it current.
- Commits carry `Refs #N`. Never use `Closes`, `Fixes` or `Resolves`.
- Close an issue by hand only after its acceptance criterion and gates pass and
  integration is verified (live branch and remote state). The closing comment is
  one outcome sentence plus the commit links; the project moves it to Done.
  Closed issues and the commit log are the record of completed work.
- Reader-facing docs, specifications, proofs and design records carry no issue
  numbers; link them from the issue instead.
- Keep local checks, remote CI, integration and deployment evidence distinct. Do
  not trigger or poll Actions while the standing workflow pause applies.

## Issue #1, the work status page

Keep it short and in the project's voice (no agents or internal IDs): what is in
progress with links to its issues and branches, what waits on whom (decisions,
blocked issues), what is next, and links to the project and its views. Update it
when the set of active work changes; it is not a list of items and not a log.

## Roles

The documentation sidekick maintains issues, project fields and Issue #1 after
handoff. The primary owns implementation, tests, site code and Git integration,
reviews changes and verifies their saved state.
