#!/usr/bin/env bash
# Usage: ready.sh [--base REV] "<scope, Refs #N>" "<checks run, each with its exit status>" ["<review verdict file>"]
# Run in your worktree after pushing. Appends the READY line to $ZP_AGENT_STATE/landing-train.md:
#   READY <branch> <sha> base <sha> | <scope> | <checks> | <review>
# The base is the merge base with origin/main. For a branch stacked on another, --base REV (the
# parent branch or commit, an ancestor of HEAD) makes the base the parent tip and records it as
#   base <sha> (parent <REV>)
# Refuses a dirty worktree, a branch that is not pushed at HEAD, a bad branch prefix, and a range
# that fails hack/check-commits.sh on base..HEAD.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=lib.sh
. "$here/lib.sh"
pbase=
if [ "${1:-}" = --base ]; then [ $# -ge 2 ] || die "--base needs a revision"; pbase=$2; shift 2; fi
[ $# -ge 2 ] && [ $# -le 3 ] || { echo 'usage: ready.sh [--base REV] "<scope>" "<checks with exit status>" ["<review file>"]' >&2; exit 2; }
scope=$1; checks=$2; review=${3:-none}
branch=$(git symbolic-ref --short HEAD) || die "detached HEAD"
zp_branch_ok "$branch" || die "branch $branch has no prefix feat/, fix/, perf/ or docs/"
[ -z "$(git status --porcelain --untracked-files=no)" ] || die "the worktree has uncommitted changes"
git fetch -q origin
sha=$(git rev-parse HEAD)
[ "$(git rev-parse -q --verify "refs/remotes/origin/$branch" || true)" = "$sha" ] || die "origin/$branch is not at HEAD $sha: push first (push-branch.sh)"
if [ -n "$pbase" ]; then
  base=$(git rev-parse -q --verify "$pbase^{commit}") || die "unknown --base $pbase"
  git merge-base --is-ancestor "$base" HEAD || die "--base $pbase is not an ancestor of HEAD"
  baseinfo="$(git rev-parse --short=9 "$base") (parent $pbase)"
  range=$base
else
  base=$(git merge-base origin/main HEAD)
  baseinfo=$(git rev-parse --short=9 "$base")
  range=origin/main
fi
"$here/../../hack/check-commits.sh" "$range" HEAD
mkdir -p "$zp_state"
line="READY $branch $(git rev-parse --short=9 "$sha") base $baseinfo | $scope | $checks | $review"
printf '%s\n' "$line" >> "$zp_state/landing-train.md"
echo "$line"
