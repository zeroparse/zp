#!/usr/bin/env bash
# Usage: ready.sh "<scope, Refs #N>" "<checks run, each with its exit status>" ["<review verdict file>"]
# Run in your worktree after pushing. Appends the READY line to $ZP_AGENT_STATE/landing-train.md:
#   READY <branch> <sha> base <sha> | <scope> | <checks> | <review>
# Refuses a dirty worktree, a branch that is not pushed at HEAD, a bad branch prefix, and a range
# that fails hack/check-commits.sh. The base is the merge base with origin/main.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=lib.sh
. "$here/lib.sh"
[ $# -ge 2 ] && [ $# -le 3 ] || { echo 'usage: ready.sh "<scope>" "<checks with exit status>" ["<review file>"]' >&2; exit 2; }
scope=$1; checks=$2; review=${3:-none}
branch=$(git symbolic-ref --short HEAD) || die "detached HEAD"
zp_branch_ok "$branch" || die "branch $branch has no prefix feat/, fix/, perf/ or docs/"
[ -z "$(git status --porcelain --untracked-files=no)" ] || die "the worktree has uncommitted changes"
git fetch -q origin
sha=$(git rev-parse HEAD)
[ "$(git rev-parse -q --verify "refs/remotes/origin/$branch" || true)" = "$sha" ] || die "origin/$branch is not at HEAD $sha: push first (push-branch.sh)"
base=$(git merge-base origin/main HEAD)
"$here/../../hack/check-commits.sh" origin/main HEAD
mkdir -p "$zp_state"
line="READY $branch $(git rev-parse --short=9 "$sha") base $(git rev-parse --short=9 "$base") | $scope | $checks | $review"
printf '%s\n' "$line" >> "$zp_state/landing-train.md"
echo "$line"
