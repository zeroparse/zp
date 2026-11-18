#!/usr/bin/env bash
# Usage: train.sh stack <name> <rev>...    build the train worktree
#        train.sh gate <name> [LEVEL...]   run the gate on its tip (default: engine full)
#        train.sh land <name>              push the tip to main and record the landing
# The integrator's landing train: cherry-pick the candidates onto current origin/main in a detached
# worktree $ZP_AGENT_STATE/wt/train-<name>, run one gate on the tip, fast-forward main.
#   <rev> is a commit, or a range A..B (cherry-picked in order). Merge commits are refused: main stays
#   linear. A conflict stops the stack with the worktree left as it is; resolve it there, run
#   `git cherry-pick --continue`, and pick the remaining revs by hand.
#   gate: see run-gate.sh; the log is $ZP_AGENT_STATE/gates/train-<name>.log. Without levels it
#   runs the ones the stack needs: `docs` for a stack of documentation, spec, site and skill paths;
#   `engine` for any other path, and `full` besides when a path of the engine, the generators
#   or the runtime changed: internal/runtime/engine, internal/backend/codegen/common, internal/compile, internal/ir,
#   pkg/bytecode, engine, codegen and core (the list in needed_levels; a package that
#   moves to a new layout must be added there). Other code, such as internal/frontend or the
#   schema, query and lint packages, gets engine only: its dependents are covered by the
#   engine level's affected-package tests. It uses the base recorded at stack time, and writes the
#   completion marker (tip and levels) only after the gate exited 0.
#   land: refuses unless a completed gate marker names this exact tip and covers every level the
#   stack needs, origin/main has not moved since the stack, and the history is linear; pushes without force, then appends
#   `LANDED train-<name> <date>: main <old>..<new>` to landing-train.md. Pushing main is the
#   integrator's act: run it only with the owner's standing permission for main.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=lib.sh
. "$here/lib.sh"
usage() { sed -n '2,/^set /p' "$0" | sed '$d;s/^# \{0,1\}//' >&2; exit 2; }
# needed_levels BASE TIP prints the gate levels the changed paths of BASE..TIP need:
# docs for documentation, spec, site and skill paths (alone when nothing else changed);
# engine for any other path; and full for engine, generator, compiler, IR, bytecode or
# runtime paths. Renames count as a delete and an add, so a file moved out of those paths
# still needs full.
needed_levels() {
  local paths docs=0 code=0 full=0 f
  paths=$(git -C "$wt" diff --no-renames --name-only "$1..$2")
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    case $f in
      docs/* | spec/* | site/* | .agents/* | AGENTS.md | README.md | SECURITY.md) docs=1 ;;
      internal/runtime/engine/* | internal/backend/codegen/common/* | internal/compile/* | internal/ir/* | pkg/bytecode/* | \
        engine/* | codegen/* | core/*) code=1; full=1 ;;
      *) code=1 ;;
    esac
  done <<< "$paths"
  local out=
  [ "$docs" = 1 ] && out="docs"
  [ "$code" = 1 ] && out="$out engine"
  [ "$full" = 1 ] && out="$out full"
  [ -n "$out" ] || out=docs
  echo "$out"
}
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
    g=$zp_state/gates
    base=$(cat "$g/train-$name.base")
    tip=$(git -C "$wt" rev-parse HEAD)
    if [ $# -eq 0 ]; then read -r -a lv <<< "$(needed_levels "$base" "$tip")"; set -- "${lv[@]}"; fi
    rm -f "$g/train-$name.ok"
    "$here/run-gate.sh" -w "$wt" -t "train-$name" -b "$base" "$@"
    [ "$(git -C "$wt" rev-parse HEAD)" = "$tip" ] || die "the tip moved during the gate"
    # The marker is written only after the gate exited 0, with the tip and the levels it covered.
    echo "tip=$tip levels=$(printf '%s\n' "$@" | sort -u | tr '\n' ' ')" > "$g/train-$name.ok"
    echo "train-$name: gated $tip with $*"
    ;;
  land)
    [ -d "$wt" ] || die "no train worktree $wt"
    g=$zp_state/gates
    tip=$(git -C "$wt" rev-parse HEAD)
    base=$(cat "$g/train-$name.base")
    ok=$(cat "$g/train-$name.ok" 2>/dev/null || true)
    [ -n "$ok" ] || die "no completed gate for train-$name (run: train.sh gate $name)"
    case $ok in "tip=$tip levels="*) ;; *) die "the completed gate did not run on this tip $tip" ;; esac
    have=" ${ok#*levels=}"
    for l in $(needed_levels "$base" "$tip"); do
      case $have in *" $l "*) ;; *) die "this stack needs the gate level $l; the gate ran:${have}" ;; esac
    done
    [ -z "$(git -C "$wt" status --porcelain)" ] || die "the train worktree is dirty"
    git fetch -q origin
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
