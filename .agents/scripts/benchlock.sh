#!/usr/bin/env bash
# Usage: benchlock.sh <owner-tag> <command...>
# Runs the command while holding the shared heavy-run lock; a lock whose owner PID is dead is taken over.
# The command runs in a process group of its own, recorded in runs/<tag>.<pid> while it runs, so
# benchstop.sh can stop exactly the groups this script started and nothing else.
#
# State: $BENCHLOCK_DIR, default $ZP_AGENT_STATE (see lib.sh). The layout is the one of the earlier
# zsh version of this script, so jobs queued by either version are ranked together.
#
# Deadlock freedom: every waiter is ranked by the key (start, seq). seq is a counter handed out
# under flock, so no two waiters share a key and exactly one live waiter is first; all waiters
# rank with the same function. As a backstop, a waiter that sees the lock free while it waits for
# BENCHLOCK_STALL seconds (default 300) logs the stall to benchlock-stalls.log and takes the lock,
# so the queue always makes progress even if the ranking were wrong.
set -euo pipefail
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[ $# -ge 2 ] || { echo "usage: benchlock.sh <owner-tag> <command...>" >&2; exit 2; }
D=${BENCHLOCK_DIR:-$zp_state}
L=$D/benchlock
R=$D/runs
stall=${BENCHLOCK_STALL:-300}
tag=$1; shift
mkdir -p "$R"
seq=$(perl -e 'use Fcntl qw(:flock); open(my $f, "+>>", $ARGV[0]) or die "seq: $!\n"; flock($f, LOCK_EX) or die; seek($f, 0, 0); my $n = <$f> // 0; $n = int($n) + 1; truncate($f, 0); print $f "$n\n"; close $f; print $n' "$D/benchlock.seq") || { echo "benchlock: cannot allocate a ticket" >&2; exit 125; }
rec=$R/$tag.$$
recline="state=queued seq=$seq tag=$tag wrapper=$$ cwd=$PWD start=$(date +%FT%T) cmd=$*"
printf '%s\n' "$recline" > "$rec"
# A wrapper is live only if its PID still runs this script with its own tag: PIDs are reused, and a
# record whose PID now belongs to another process would otherwise block the queue forever.
# Records of wrappers that are no longer live are moved to runs-stale/.
live() { ps -o command= -p "$1" 2>/dev/null | grep -qF "benchlock.sh $2 "; }
# The rank key of a record: its start= (the integrator may edit it for train priority) and then its
# seq= (records written before seq existed use their wrapper PID). Unique per waiter.
key() {
  local t s
  t=$(grep -o 'start=[^ ]*' "$1" || true); t=${t#start=}
  s=$(grep -o 'seq=[0-9]*' "$1" || true); s=${s#seq=}
  [ -n "$s" ] || s=${1##*.}
  printf '%s %012d' "$t" "$s"
}
# First come, first served: only the first live queued wrapper by key may take a free lock.
first() {
  local f best='' bk='' k w ft
  for f in "$R"/*; do
    [ -f "$f" ] || continue
    grep -q '^state=queued' "$f" || continue
    if [ "$f" != "$rec" ]; then
      w=${f##*.}; ft=${f##*/}; ft=${ft%.*}
      live "$w" "$ft" || { mkdir -p "$D/runs-stale"; mv -f "$f" "$D/runs-stale/" 2>/dev/null || true; continue; }
    fi
    k=$(key "$f")
    if [ -z "$best" ] || [[ "$k" < "$bk" ]]; then best=$f; bk=$k; fi
  done
  [ "$best" = "$rec" ]
}
free=0
until { first || [ "$free" -ge "$stall" ]; } && mkdir "$L" 2>/dev/null; do
  [ -f "$rec" ] || printf '%s\n' "$recline" > "$rec"
  if [ -d "$L" ]; then
    free=0
    pid=$(awk '{print $2}' "$L/owner" 2>/dev/null || true)
    otag=$(awk '{print $1}' "$L/owner" 2>/dev/null || true)
    if [ -n "$pid" ] && ! live "$pid" "$otag"; then rm -f "$L/owner"; rmdir "$L" 2>/dev/null || true; continue; fi
  else
    free=$((free + 20))
  fi
  sleep "${BENCHLOCK_POLL:-20}"
done
if [ "$free" -ge "$stall" ]; then
  printf '%s stall: lock free %ss, %s.%s (seq %s) took it; queue: %s\n' "$(date +%FT%T)" "$free" "$tag" "$$" "$seq" \
    "$(cat "$R"/* | cut -c1-80 | tr '\n' '|')" >> "$D/benchlock-stalls.log"
fi
echo "$tag $$" > "$L/owner"
perl -e 'setpgrp(0,0); exec @ARGV or die "exec: $!\n"' -- "$@" &
child=$!
printf '%s\n' "state=running seq=$seq tag=$tag wrapper=$$ pgid=$child cwd=$PWD start=$(date +%FT%T) cmd=$*" > "$rec"
trap 'kill -TERM -- -$child 2>/dev/null || true' TERM INT HUP
rc=0
wait "$child" || rc=$?
# A trapped signal interrupts the first wait; wait for the group leader to be reaped.
while kill -0 "$child" 2>/dev/null; do wait "$child" || rc=$?; done
rm -f "$rec"
if [ "$(cat "$L/owner" 2>/dev/null || true)" = "$tag $$" ]; then rm -f "$L/owner"; rmdir "$L"; fi
exit "$rc"
