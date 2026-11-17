#!/usr/bin/env bash
# Usage: lane-new.sh <name> <branch> [base]
# Creates the worktree $ZP_AGENT_STATE/wt/<name> on a new branch <branch> from a freshly fetched
# <base> (default origin/main), refuses a bad branch prefix, an existing worktree or branch, and
# prepares the untracked .scratch/ directory. Run it from any worktree of the repository, never to
# work in the root checkout. Prints the worktree path.
set -euo pipefail
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[ $# -ge 2 ] && [ $# -le 3 ] || { echo "usage: lane-new.sh <name> <branch> [base]" >&2; exit 2; }
name=$1; branch=$2; base=${3:-origin/main}
case $name in */* | '' | .*) die "bad lane name: $name" ;; *) ;; esac
zp_branch_ok "$branch" || die "branch $branch needs a prefix feat/, fix/, perf/ or docs/"
wt=$zp_state/wt/$name
[ ! -e "$wt" ] || die "$wt exists"
git rev-parse -q --verify "refs/heads/$branch" >/dev/null && die "branch $branch exists"
git fetch -q origin
git rev-parse -q --verify "$base^{commit}" >/dev/null || die "unknown base $base"
mkdir -p "$zp_state/wt"
git worktree add -q "$wt" -b "$branch" "$base"
mkdir -p "$wt/.scratch"
# .scratch/ must never be staged; keep it out of status through the worktree's own exclude file.
excl=$(git -C "$wt" rev-parse --git-path info/exclude)
case $excl in /*) ;; *) excl=$wt/$excl ;; esac
mkdir -p "$(dirname "$excl")"
grep -qx '/.scratch/' "$excl" 2>/dev/null || echo '/.scratch/' >> "$excl"
echo "$wt"
echo "branch $branch at $(git -C "$wt" rev-parse --short HEAD) (base $base)" >&2
