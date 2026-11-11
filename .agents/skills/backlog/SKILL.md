---
name: backlog
description: Maintain ZeroParse's Issue #1 backlog, its status comment, and recent completions.
---

# Backlog

- Use ZeroParse Issue #1: https://github.com/zeroparse/zp/issues/1. Retrieve current
  issue content and the status comment before proposing or making updates.
- Categories use two-digit IDs and are defined by the kind of change: B bug
  and other defect fixes; O optimization, meaning changes that actually affect
  performance, visible to users or not (enabling infrastructure with no
  performance effect of its own is I); F user-visible new features, with a
  design record close to mandatory (the flag and profile mechanism for
  optimizations is F; one optimization exposed as a flag stays O); Q quality
  and verification (tests, benchmarks, proofs, reference builds, flaky tests)
  and developer-only tooling; I improvements to internal mechanisms and
  functions, meaning refactoring and reorganization other than for
  performance, of any size, including the precision of analyses such as the
  classifier; D documentation work (guides, specification text, design
  records), where proofs are not documentation; M problems specific to one
  bundled or ready-made parser; R release and launch (tagging, announcements,
  posts, launch measurement). Tools are categorized by the kind of change.
  Preserve existing IDs and never reuse one. A new ID is the category's entry
  in the body's last-used-ID line plus one; update that line when allocating.
  Moving an item to another category allocates a new ID and retires the old one.
- Keep the issue body to its rules, the last used IDs, the status link, the
  ten most recent completions and incomplete items. The commit log is the full
  record of completed work; there are no archive comments.
- Keep order, dependencies and execution state in one status comment, linked
  from the body: the principle, work in progress, paused branches, the order
  with dependencies, a category-by-tier table, and a final count line of open
  items. Keeping them in one comment stops one part from going stale while
  another is updated. The status is current state, decisions and blockers, not
  a conversation log.
- Every open item appears in the status table exactly once and the count line
  equals the number of open items; compare the ID sets before each upload.
- Preserve incomplete parent scope. Complete an item only after acceptance gates
  pass and integration is verified. Then remove it from the open list, add its
  ID, linked commit and one outcome sentence to the top of the recent
  completions (dropping the oldest beyond ten), and update the status comment
  in the same pass.
- Remove or rewrite stale items, move items filed under the wrong category, and
  trim landed history from item text; an item states the problem, decisions,
  remaining work and its acceptance criterion.
- IDs are for Issue #1 only. Never put backlog IDs in reader docs, commits or
  design/optimization records; link those documents descriptively instead.
- Fetch before edits and read back affected surfaces. Keep local checks, remote
  CI, integration and deployment evidence distinct. Do not trigger or poll
  Actions while the standing workflow pause applies.
- The documentation sidekick owns Issue #1 after handoff; the primary owns
  implementation, tests, site code and Git integration. The primary reviews
  changes and verifies their saved state.
