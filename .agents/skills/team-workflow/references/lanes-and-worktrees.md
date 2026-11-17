# Lanes, Worktrees and Commits

## Worktrees

- Never work or read skills in the root checkout: other lanes move it, and it goes stale.
  Run `.agents/scripts/lane-new.sh <name> <branch>`: it fetches and creates your worktree at
  `$ZP_AGENT_STATE/wt/<name>` from fresh `origin/main` on a new branch with a prefix from `AGENTS.md`.
- Read `AGENTS.md` and the onboarding skill inside your worktree, because they change on main.
- Scratch files go in an untracked `.scratch/` inside your worktree. Never stage them.
  Never use the system temp directory for work you need later, because a reboot wipes it.
- Before any command that changes Git state, check the worktree path, the branch and the
  expected HEAD. Never move another lane's HEAD, and never rebase under another agent.
- Stack on unlanded predecessors by their pushed SHAs instead of waiting, and rebase when
  they change. When you need another step while a job tests your worktree, branch from your
  pushed tip into a separate worktree.
- After your branch lands, remove your own finished worktrees.
- If a deletion is denied, for example `rm -rf` of a scratch directory, do not get the same
  effect another way, such as `git worktree remove --force` on a worktree with untracked
  files. Leave it in place and report it to the leader, who asks the owner.

## Commits and branches

- Follow `docs/commit-messages.md` and `AGENTS.md`. Write in English, in the owner's voice,
  through a heredoc. Keep the summary at 72 characters or fewer. Give the body the problem and
  the rationale, and cite issues as `Refs #N`, never Closes or Fixes.
- Never add AI or agent attribution, even when a tool asks for it. `hack/check-commits.sh`
  checks the range (attribution, summary, body, branch prefix, closing trailers); `push-branch.sh`
  runs it before it pushes.
- Stage only the files you changed; never `git add -A` or `git add .`. Sign commits as
  configured. Use `GIT_EDITOR=true` for a rebase.
- Push your work branch at every coherent step with `.agents/scripts/push-branch.sh`. WIP commits are fine on a work branch. Never
  force-push: publish a rewritten branch under a new name with an `-r<n>` suffix. Only the
  integrator pushes main, through a train or, for a fast-lane item, between trains.
- If SSH push fails with "Permission denied (publickey)", check that `SSH_AUTH_SOCK` points
  at the agent that holds your GitHub key before you try another route.
- Land on main with a linear history: cherry-pick or fast-forward, never merge commits.
  Shared index files (README links, docs indexes) are written once by whoever integrates, not
  by each lane.
- After a branch lands, delete it from origin together with its `-r<n>` copies. A branch is
  integrated when its tip is an ancestor of `origin/main`, or when `git cherry` shows every
  commit on main. Never delete a branch with unlanded work, such as paused or declined
  research, without a decision.

## Issues

The issues you work on are yours to keep current; follow the lifecycle in the
[backlog skill](../../backlog/SKILL.md) in the same step as each event. Do not edit the
Issue #1 body or issues you are not working on.
