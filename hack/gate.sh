#!/usr/bin/env bash
# Run the validation levels of docs/development.md with one command, so that every contributor checks the same things.
#
#	hack/gate.sh [--base REV] [--keep-going] [--summary FILE] [--dry-run] LEVEL...
#
# Levels (several may be given; a step that two levels share runs once):
#   docs    go vet ./..., the tests of the site module, and a site build with its link check (without the WebAssembly
#           binary, which the playground checks cover).
#   code    go build ./..., go vet ./..., and go test of the packages that changed since REV and of every package that
#           depends on them (hack/affected-tests.sh, which uses go list).
#   engine  code, with the default-tier tests replaced by the same packages in the long tier (-tags zp_long, which
#           includes the default tier), then go generate ./internal/backend/... ./bench ./internal/frontend/parser
#           ./spec/language with a check that it changed nothing, hack/test-alpha-parsers.sh and
#           hack/test-reference-tags.sh.
#   full    go test -tags zp_long ./... (the whole tree, in the long tier).
#
# REV is the base of the change (default origin/main); the changed packages are those of REV..HEAD, so commit before
# running. Steps run in order and the gate stops at the first failure, so that a heavy step does not run after a
# failed one; --keep-going runs them all. One line per step (name, exit code, seconds) goes to the summary file
# (default: gate-summary.<pid>.txt in $TMPDIR) and is printed at the end. The exit status is non-zero if a step failed.
# --dry-run prints the steps and runs nothing.
#
# For callers (scripts, lock wrappers): the gate never prompts and reads no standard input. Exit status 0: every step
# passed; 1: a step failed (the summary names it); 2: a usage error, before any step ran. The summary file has one
# tab-separated line per step that ran, "NAME<TAB>exit N<TAB>Ss", in execution order; with a fail-fast stop the failed
# step is the last line.
#
# Environment: GATE_NICE (default 0) is the nice(1) increment of every step, and GOMAXPROCS, GOFLAGS and the other Go
# variables pass through. GATE_SELFTEST_FAIL=1 adds a step that fails first, to show that the gate stops.
# GATE_AFFECTED_CMD replaces hack/affected-tests.sh (GATE_AFFECTED_CMD=false shows that a failed computation fails the step).
# The step commands are single-quoted on purpose: they expand when the step runs.
# shellcheck disable=SC2016,SC2028
set -euo pipefail

base=origin/main
keep=0
dry=0
summary=
levels=()
while [ $# -gt 0 ]; do
	case $1 in
	--base) [ $# -ge 2 ] || { echo "gate: --base needs a revision" >&2; exit 2; }; base=$2; shift 2 ;;
	--summary) [ $# -ge 2 ] || { echo "gate: --summary needs a file" >&2; exit 2; }; summary=$2; shift 2 ;;
	--keep-going) keep=1; shift ;;
	--dry-run) dry=1; shift ;;
	-h | --help) sed -n '2,/^set -/p' "$0" | sed '$d; s/^# \{0,1\}//'; exit 0 ;;
	docs | code | engine | full) levels+=("$1"); shift ;;
	*) echo "gate: unknown argument $1 (levels: docs code engine full)" >&2; exit 2 ;;
	esac
done
[ ${#levels[@]} -gt 0 ] || { echo "usage: hack/gate.sh [--base REV] [--keep-going] [--summary FILE] [--dry-run] docs|code|engine|full..." >&2; exit 2; }

[ -z "$summary" ] || case $summary in /*) ;; *) summary=$PWD/$summary ;; esac
root=$(git rev-parse --show-toplevel)
cd "$root"
nice_n=${GATE_NICE:-0}
[ -n "$summary" ] || summary=${TMPDIR:-/tmp}/gate-summary.$$.txt

names=()
descs=()
cmds=()
# step NAME DESCRIPTION COMMAND: append a step unless one of that name is already planned. COMMAND runs through bash -c
# in the root of the tree, so it may be a shell fragment.
step() {
	local n
	for n in ${names[@]+"${names[@]}"}; do [ "$n" = "$1" ] && return 0; done
	names+=("$1")
	descs+=("$2")
	cmds+=("$3")
}

# affected_cmd GO-TEST-ARGS: go test over the packages of the changes since $GATE_BASE and their reverse dependencies.
affected_cmd() {
	echo 'raw=$("${GATE_AFFECTED_CMD:-hack/affected-tests.sh}" "$GATE_BASE" HEAD) || { echo "gate: the affected packages could not be computed" >&2; exit 1; }
pkgs=$(printf "%s\n" "$raw" | grep -v "^#" || true)
[ -n "$pkgs" ] || { echo "gate: no package affected since $GATE_BASE"; exit 0; }
echo "$pkgs" | sed "s/^/gate: affected /"
go test -count=1 '"$1"' $pkgs'
}
# snap hashes the tracked changes and the untracked files, to see that a step left the tree as it found it.
snap_def='snap() { { git diff HEAD --binary; git ls-files -o --exclude-standard -z | xargs -0 shasum 2>/dev/null || true; } | shasum; }'

[ "${GATE_SELFTEST_FAIL:-0}" != 1 ] || step selftest 'a step that fails (GATE_SELFTEST_FAIL)' false

want() { local l; for l in "${levels[@]}"; do [ "$l" = "$1" ] && return 0; done; return 1; }

if want docs; then
	step vet 'go vet ./...' 'go vet ./...'
	step site-test 'cd site && go test ./...' 'cd site && go test -count=1 ./...'
	step site-build 'site/build.sh -wasm=false (builds into a temporary directory, checks links)' 'out=$(cd -P "$(mktemp -d "${TMPDIR:-/tmp}/gate-site.XXXXXX")" && pwd) && trap "rm -rf \"$out\"" EXIT && site/build.sh -out "$out/dist" -wasm=false'
fi
if want code || want engine; then
	step build 'go build ./...' 'go build ./...'
	step vet 'go vet ./...' 'go vet ./...'
	if ! want engine; then
		step test 'go test of the affected packages' "$(affected_cmd '-timeout 45m')"
	fi
fi
if want engine; then
	step test-long 'go test -tags zp_long of the affected packages' "$(affected_cmd '-tags zp_long -timeout 90m')"
	step generate 'go generate ./internal/backend/... ./bench ./internal/frontend/parser ./spec/language; the tree must not change' "$snap_def"'
before=$(snap)
go generate ./internal/backend/... ./bench ./internal/frontend/parser ./spec/language
[ "$(snap)" = "$before" ] || { echo "gate: go generate changed the tree; commit the regenerated files" >&2; git status --short >&2; exit 1; }'
	step alpha-parsers 'hack/test-alpha-parsers.sh' hack/test-alpha-parsers.sh
	step reference-tags 'hack/test-reference-tags.sh' hack/test-reference-tags.sh
fi
if want full; then
	step test-full 'go test -tags zp_long ./...' 'go test -count=1 -tags zp_long -timeout 90m ./...'
fi

if [ "$dry" = 1 ]; then
	echo "gate: base $base; steps:"
	i=0
	for n in "${names[@]}"; do printf '  %-15s %s\n' "$n" "${descs[$i]}"; i=$((i + 1)); done
	case " ${names[*]} " in *" test"*) echo "gate: affected packages since $base:"; "${GATE_AFFECTED_CMD:-hack/affected-tests.sh}" "$base" HEAD | sed 's/^/  /' ;; esac
	exit 0
fi

git rev-parse --verify --quiet "$base^{commit}" >/dev/null || { echo "gate: base $base is not a revision" >&2; exit 2; }
if [ -n "$(git status --porcelain)" ]; then echo "gate: warning: the tree has uncommitted changes; the changed packages are those of $base..HEAD" >&2; fi
export GATE_BASE=$base
: >"$summary"
failed=0
i=0
for n in "${names[@]}"; do
	echo "gate: == $n" >&2
	start=$SECONDS
	rc=0
	nice -n "$nice_n" bash -c "${cmds[$i]}" || rc=$?
	printf '%s\texit %d\t%ds\n' "$n" "$rc" $((SECONDS - start)) >>"$summary"
	if [ "$rc" -ne 0 ]; then
		failed=1
		[ "$keep" = 1 ] || { echo "gate: $n failed (exit $rc); stopping" >&2; break; }
	fi
	i=$((i + 1))
done
echo "gate: summary ($summary):"
cat "$summary"
if [ "$failed" = 0 ]; then echo "gate: ok"; else echo "gate: FAILED"; fi
exit "$failed"
