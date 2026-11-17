#!/usr/bin/env bash
# Usage: benchstop.sh <tag>
# Stops the process groups that benchlock.sh started under <tag>, as recorded in runs/. It signals
# only a recorded group whose leader is still alive in that group; anything else is left alone.
set -euo pipefail
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
R=${BENCHLOCK_DIR:-$zp_state}/runs
tag=${1:-}
[ -n "$tag" ] || { echo "usage: benchstop.sh <tag>" >&2; exit 2; }
found=0
for rec in "$R/$tag".*; do
  [ -f "$rec" ] || continue
  pgid=$(grep -o 'pgid=[0-9]*' "$rec" | cut -d= -f2 || true)
  wrapper=$(grep -o 'wrapper=[0-9]*' "$rec" | cut -d= -f2 || true)
  if [ -z "$pgid" ]; then
    echo "queued, stopping wrapper $wrapper: $(cat "$rec")"
    if ps -o command= -p "$wrapper" 2>/dev/null | grep -q "benchlock.sh $tag "; then
      kill -TERM "$wrapper" && rm -f "$rec"
    fi
    found=1; continue
  fi
  if [ "$(ps -o pgid= -p "$pgid" 2>/dev/null | tr -d ' ')" != "$pgid" ]; then
    echo "stale record, group $pgid is gone: $rec"; rm -f "$rec"; continue
  fi
  echo "stopping group $pgid: $(cat "$rec")"
  kill -TERM -- "-$pgid"
  found=1
done
[ "$found" = 1 ] || echo "no live run recorded for tag $tag"
