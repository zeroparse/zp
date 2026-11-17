#!/usr/bin/env bash
# Usage: run-gate.sh [-w worktree] [-t tag] [-b base] LEVEL...
# Runs hack/gate.sh (levels docs, code, engine, full; see its header) in the worktree, in the run
# class its heaviest level needs, logs to $ZP_AGENT_STATE/gates/<tag>.log, and prints the per-step
# summary with the exit status. The gate stops at the first failing step.
#   docs   directly, throttled (nice 15, GOMAXPROCS 4)
#   code   through testslot.sh
#   engine, full   through benchlock.sh under <tag>
# Defaults: worktree = the current one, tag = the worktree directory name, base = origin/main.
# Commit before running: the gate looks at base..HEAD. Exit status: the gate's.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=lib.sh
. "$here/lib.sh"
wt=$PWD; tag=; base=origin/main
while getopts w:t:b: o; do
  case $o in w) wt=$OPTARG ;; t) tag=$OPTARG ;; b) base=$OPTARG ;; *) exit 2 ;; esac
done
shift $((OPTIND - 1))
[ $# -ge 1 ] || { sed -n '2,/^set /p' "$0" | sed '$d;s/^# \{0,1\}//' >&2; exit 2; }
wt=$(cd "$wt" && git rev-parse --show-toplevel)
[ -x "$wt/hack/gate.sh" ] || die "$wt/hack/gate.sh is missing: rebase on a main that has the gate"
[ -n "$tag" ] || tag=$(basename "$wt")
class=docs
for l in "$@"; do
  case $l in
    docs) ;;
    code) [ "$class" = docs ] && class=code ;;
    engine | full) class=heavy ;;
    *) die "unknown level $l (docs, code, engine, full)" ;;
  esac
done
mkdir -p "$zp_state/gates"
log=$zp_state/gates/$tag.log; sum=$zp_state/gates/$tag.sum
rm -f "$sum"
# shellcheck disable=SC2016  # the inner script expands its own arguments
run=(bash -c 'cd "$1" && shift && exec "$@"' _ "$wt" env GATE_NICE=15 GOMAXPROCS="${GOMAXPROCS:-4}" "$wt/hack/gate.sh" --base "$base" --summary "$sum" "$@")
case $class in
  docs) wrap=() ;;
  code) wrap=("$here/testslot.sh") ;;
  heavy) wrap=("$here/benchlock.sh" "$tag") ;;
  *) die "internal: class $class" ;;
esac
echo "run-gate: $* in $wt (class $class), log $log"
rc=0
${wrap[@]+"${wrap[@]}"} "${run[@]}" > "$log" 2>&1 || rc=$?
[ -f "$sum" ] && cat "$sum"
echo "run-gate: exit $rc (log $log)"
[ "$rc" = 0 ] || tail -n 15 "$log"
exit "$rc"
