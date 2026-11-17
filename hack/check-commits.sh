#!/usr/bin/env bash
# Check the commits of a range against the commit rules before pushing them.
#
#	hack/check-commits.sh [base [head]]     default base origin/main, default head HEAD
#
# Reports every violation, then exits 1; exits 0 when the range is clean. Checked per commit: no AI or
# agent attribution (Co-Authored-By trailers naming a model or vendor, "Generated with" footers, the robot
# emoji), no merge commit, a summary of at most 72 characters that does not end in a period, a body, and no
# closing trailer (Closes, Fixes, Resolves). Checked for the range: no file under .scratch/, and, when head
# is a branch checked out here, the branch prefix of AGENTS.md (feat/, fix/, perf/, docs/) and not main.
# It reads history only and changes nothing.
set -euo pipefail
base=${1:-origin/main}
head=${2:-HEAD}
case ${1:-} in -h | --help) sed -n '2,/^set /p' "$0" | sed '$d;s/^# \{0,1\}//'; exit 0 ;; esac
git rev-parse -q --verify "$base^{commit}" >/dev/null || { echo "check-commits: unknown base $base" >&2; exit 2; }
git rev-parse -q --verify "$head^{commit}" >/dev/null || { echo "check-commits: unknown head $head" >&2; exit 2; }
bad=0
fail() { echo "check-commits: $*" >&2; bad=1; }
n=0
for c in $(git rev-list --reverse "$base..$head"); do
	n=$((n + 1))
	short=$(git rev-parse --short "$c")
	subject=$(git log -1 --format=%s "$c")
	body=$(git log -1 --format=%b "$c")
	if [ "$(git rev-list --parents -n1 "$c" | wc -w)" -gt 2 ]; then fail "$short is a merge commit: $subject"; fi
	if [ "${#subject}" -gt 72 ]; then fail "$short summary is ${#subject} characters (72 at most): $subject"; fi
	case $subject in *.) fail "$short summary ends with a period: $subject" ;; esac
	if [ -z "$(printf '%s' "$body" | tr -d '[:space:]')" ]; then fail "$short has no body: $subject"; fi
	if git log -1 --format=%B "$c" | grep -qiE 'co-authored-by:.*(claude|anthropic|openai|codex|copilot|gemini|noreply@)|generated (with|by)|🤖'; then
		fail "$short carries AI attribution: $subject"
	fi
	if git log -1 --format=%B "$c" | grep -qiE '^(close[sd]?|fix(e[sd])?|resolve[sd]?):? +#?[0-9]+'; then
		fail "$short has a closing trailer (use Refs): $subject"
	fi
done
if git diff --name-only "$base...$head" | grep -q '^\.scratch/'; then fail "the range changes files under .scratch/"; fi
branch=$(git symbolic-ref -q --short HEAD || true)
if [ -n "$branch" ] && [ "$(git rev-parse "$head")" = "$(git rev-parse HEAD)" ]; then
	case $branch in
	main | master) fail "the checked-out branch is $branch; work on a branch" ;;
	feat/* | fix/* | perf/* | docs/*) ;;
	*) fail "branch $branch has no prefix feat/, fix/, perf/ or docs/" ;;
	esac
fi
[ "$bad" = 0 ] || exit 1
echo "check-commits: $n commit(s) in $base..$head are clean"
