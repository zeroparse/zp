#!/usr/bin/env bash
# Usage: train.sh stack <name> <rev>...    build the train worktree
#        train.sh gate <name> [LEVEL...]   run the gate on its tip (default: engine full)
#        train.sh land <name>              push the tip to main and record the landing
# The integrator's landing train: cherry-pick the candidates onto current origin/main in a detached
# worktree $ZP_AGENT_STATE/wt/train-<name>, run one gate on the tip, fast-forward main.
#   <rev> is a commit, or a range A..B (cherry-picked in order). Merge commits are refused: main stays
#   linear. A conflict stops the stack with the worktree left as it is; resolve it there, run
#   `git cherry-pick --continue`, and pick the remaining revs by hand.
#   gate: see run-gate.sh; the log is $ZP_AGENT_STATE/gates/train-<name>.log.
#   land: refuses unless the last gate summary passed for this exact tip, origin/main has not moved
#   since the stack, and the history is linear; pushes without force, then appends
#   `LANDED train-<name> <date>: main <old>..<new>` to landing-train.md. Pushing main is the
#   integrator's act: run it only with the owner's standing permission for main.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=lib.sh
. "$here/lib.sh"
usage() { sed -n '2,/^set /p' "$0" | sed '$d;s/^# \{0,1\}//' >&2; exit 2; }
[ $# -ge 2 ] || usage
cmd=$1; name=$2; shift 2
wt=$zp_state/wt/train-$name
case $cmd in
  stack)
    [ $# -ge 1 ] || usage
    [ ! -e "$wt" ] || die "$wt exists"
    git fetch -q origin
    base=$(git rev-parse origin/main)
    mkdir -p "$zp_state/wt" "$zp_state/gates"
    git worktree add -q --detach "$wt" "$base"
    echo "$base" > "$zp_state/gates/train-$name.base"
    for r in "$@"; do
      case $r in *..*) revs=$(git rev-list --reverse "$r") ;; *) revs=$(git rev-parse "$r^{commit}") ;; esac
      for c in $revs; do
        [ "$(git rev-list --parents -n1 "$c" | wc -w)" -le 2 ] || die "$c is a merge commit"
        if ! git -C "$wt" cherry-pick "$c" >/dev/null 2>&1; then
          echo "train: conflict at $(git rev-parse --short "$c") $(git log -1 --format=%s "$c"); resolve in $wt" >&2
          exit 1
        fi
        echo "picked $(git rev-parse --short "$c") $(git log -1 --format=%s "$c")"
      done
    done
    echo "train-$name: $(git -C "$wt" rev-parse --short HEAD) on $(git rev-parse --short "$base")"
    ;;
  gate)
    [ -d "$wt" ] || die "no train worktree $wt"
    [ $# -ge 1 ] || set -- engine full
    mkdir -p "$zp_state/gates"
    git -C "$wt" rev-parse HEAD > "$zp_state/gates/train-$name.tip"
    exec "$here/run-gate.sh" -w "$wt" -t "train-$name" "$@"
    ;;
  land)
    [ -d "$wt" ] || die "no train worktree $wt"
    g=$zp_state/gates
    tip=$(git -C "$wt" rev-parse HEAD)
    [ "$(cat "$g/train-$name.tip" 2>/dev/null || true)" = "$tip" ] || die "the gate did not run on this tip $tip"
    [ -s "$g/train-$name.sum" ] || die "no gate summary"
    ! grep -vqE $'\texit 0\t' "$g/train-$name.sum" || die "the gate summary has a failed step"
    [ -z "$(git -C "$wt" status --porcelain)" ] || die "the train worktree is dirty"
    git fetch -q origin
    base=$(cat "$g/train-$name.base")
    [ "$(git rev-parse origin/main)" = "$base" ] || die "origin/main moved since the stack; restack"
    [ -z "$(git -C "$wt" rev-list --merges "$base..$tip")" ] || die "merge commit in the train"
    "$here/../../hack/check-commits.sh" "$base" "$tip"
    git -C "$wt" push origin "$tip:refs/heads/main"
    line="LANDED train-$name $(date +%F): main $(git rev-parse --short "$base")..$(git rev-parse --short "$tip")"
    printf '%s\n' "$line" >> "$zp_state/landing-train.md"
    echo "$line"
    ;;
  *) usage ;;
esac
