#!/usr/bin/env bash
# Usage: push-branch.sh [branch...]
# Pushes work branches to origin without force, after hack/check-commits.sh passes on
# origin/main..<branch>. Default: the current branch. Refuses main. A rejected (non-fast-forward)
# push is reported, never forced: publish a rewritten branch under a new name with an -r<n> suffix.
# When SSH_AUTH_SOCK is unset, it points at the gpg-agent SSH socket, if gpgconf knows one.
# Retries a transient failure up to 5 times. Exit status 1 when any branch failed.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=lib.sh
. "$here/lib.sh"
if [ -z "${SSH_AUTH_SOCK:-}" ] && command -v gpgconf >/dev/null 2>&1; then
  sock=$(gpgconf --list-dirs agent-ssh-socket 2>/dev/null || true)
  if [ -S "$sock" ]; then export SSH_AUTH_SOCK=$sock; fi
fi
if [ $# -eq 0 ]; then set -- "$(git symbolic-ref --short HEAD)"; fi
git fetch -q origin
fail=0
err=$(mktemp)
trap 'rm -f "$err"' EXIT
for b in "$@"; do
  case $b in main | master) echo "push-branch: refusing $b" >&2; fail=1; continue ;; *) ;; esac
  zp_branch_ok "$b" || { echo "push-branch: $b has no prefix feat/, fix/, perf/ or docs/" >&2; fail=1; continue; }
  git rev-parse -q --verify "refs/heads/$b" >/dev/null || { echo "skip $b (missing)"; continue; }
  "$here/../../hack/check-commits.sh" origin/main "$b" || { echo "FAIL $b: commit check" >&2; fail=1; continue; }
  ok=0
  for _ in 1 2 3 4 5; do
    if git push -q origin "refs/heads/$b:refs/heads/$b" 2> "$err"; then ok=1; break; fi
    if grep -q 'rejected\|non-fast-forward' "$err"; then break; fi
    sleep 2
  done
  if [ "$ok" = 1 ]; then echo "ok   $b $(git rev-parse --short "$b")"
  else echo "FAIL $b: $(grep -v '^remote:' "$err" | tail -n 2)" >&2; fail=1; fi
done
exit "$fail"
