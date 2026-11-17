#!/usr/bin/env bash
# Usage: alpha-check.sh [-c] [-w worktree] [-t tag]
# Syncs the alpha-parsers checkout from the worktree (hack/sync-alpha-parsers.sh) and tests it
# (hack/test-alpha-parsers.sh), through the run queue under <tag> (default: the worktree name).
# ZP_ALPHA_PARSERS names the checkout. With -c, it first clones that checkout into
# <worktree>/.scratch/alpha and uses the clone: lanes never touch the integrator's checkout.
# The sync rewrites the checkout, so it is never run on a checkout another job uses.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=lib.sh
. "$here/lib.sh"
wt=$PWD; tag=; clone=0
while getopts cw:t: o; do
  case $o in c) clone=1 ;; w) wt=$OPTARG ;; t) tag=$OPTARG ;; *) exit 2 ;; esac
done
wt=$(cd "$wt" && git rev-parse --show-toplevel)
[ -n "$tag" ] || tag=$(basename "$wt")
src=${ZP_ALPHA_PARSERS:-$wt/../alpha-parsers}
if [ "$clone" = 1 ]; then
  [ -d "$src/.git" ] || die "no alpha-parsers checkout at $src (set ZP_ALPHA_PARSERS)"
  [ -d "$wt/.scratch/alpha" ] || git clone -q "$src" "$wt/.scratch/alpha"
  src=$wt/.scratch/alpha
fi
[ -d "$src" ] || die "no alpha-parsers checkout at $src (set ZP_ALPHA_PARSERS)"
export ZP_ALPHA_PARSERS=$src
# shellcheck disable=SC2016  # the inner script expands its own arguments
exec "$here/benchlock.sh" "$tag" bash -c 'cd "$1" && hack/sync-alpha-parsers.sh && hack/test-alpha-parsers.sh' _ "$wt"
