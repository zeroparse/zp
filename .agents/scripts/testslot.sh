#!/usr/bin/env bash
# Usage: testslot.sh <cmd...>
# Counting semaphore (TESTSLOT_N slots, default 4) for targeted `go test` runs. Each slot is a
# directory slots/<i> holding a `pid` file; a slot whose holder PID is dead is reclaimed.
# Claiming runs under a short global mutex (slots/.mutex) so that reclaiming a dead holder's slot
# cannot race with a new holder taking it. Holders release without the mutex (pid file first).
# State: $ZP_AGENT_STATE/slots (the layout of the earlier zsh version).
set -euo pipefail
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
D=$zp_state/slots
n=${TESTSLOT_N:-4}
[ $# -gt 0 ] || { echo "usage: testslot.sh <cmd...>" >&2; exit 2; }
mkdir -p "$D"
mine=
locked=
# shellcheck disable=SC2329  # runs from the traps
release() {
  if [ -n "$mine" ]; then rm -f "$mine/pid"; rmdir "$mine" 2>/dev/null || true; mine=; fi
  if [ -n "$locked" ]; then rmdir "$D/.mutex" 2>/dev/null || true; locked=; fi
}
trap 'release; exit 143' TERM INT HUP
trap release EXIT
while :; do
  if mkdir "$D/.mutex" 2>/dev/null; then
    locked=1
    i=1
    while [ "$i" -le "$n" ]; do
      s=$D/$i
      if mkdir "$s" 2>/dev/null; then echo $$ > "$s/pid"; mine=$s; break; fi
      p=$(cat "$s/pid" 2>/dev/null || true)
      if [ -n "$p" ] && ! kill -0 "$p" 2>/dev/null; then
        rm -f "$s/pid"; rmdir "$s" 2>/dev/null || true
        if mkdir "$s" 2>/dev/null; then echo $$ > "$s/pid"; mine=$s; break; fi
      fi
      i=$((i + 1))
    done
    rmdir "$D/.mutex"; locked=
    [ -n "$mine" ] && break
  fi
  sleep 1
done
"$@" &
child=$!
rc=0
wait "$child" || rc=$?
if kill -0 "$child" 2>/dev/null; then kill -TERM "$child" 2>/dev/null || true; wait "$child" 2>/dev/null || true; fi
exit "$rc"
