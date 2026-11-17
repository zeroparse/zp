#!/usr/bin/env bash
# Usage: queue-status.sh
# Prints the run queue: the lock owner, every record of runs/ in rank order (start, seq), the stall
# log, and a STUCK line when jobs are queued but none runs. Exit status 1 when stuck or when the
# stall log is not empty, so that a poll can act on it.
set -euo pipefail
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
D=${BENCHLOCK_DIR:-$zp_state}
echo "state: $D"
echo "lock owner: $(cat "$D/benchlock/owner" 2>/dev/null || echo none)"
for f in "$D"/runs/*; do
  [ -f "$f" ] || continue
  line=$(cut -c1-200 "$f")
  start=$(grep -o 'start=[^ ]*' "$f" || true); seq=$(grep -o 'seq=[0-9]*' "$f" || true)
  printf '%s %s\t%s\n' "${start#start=}" "${seq#seq=}" "$line"
done | sort
q=0; r=0
for f in "$D"/runs/*; do
  [ -f "$f" ] || continue
  if grep -q '^state=queued' "$f"; then q=$((q + 1)); fi
  if grep -q '^state=running' "$f"; then r=$((r + 1)); fi
done
rc=0
if [ -s "$D/benchlock-stalls.log" ]; then echo "STALLS (a queue bug, report it):"; cat "$D/benchlock-stalls.log"; rc=1; fi
echo "queued: $q running: $r"
if [ "$q" -gt 0 ] && [ "$r" -eq 0 ]; then echo "STUCK: jobs are queued and none runs"; rc=1; fi
exit "$rc"
