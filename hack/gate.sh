#!/usr/bin/env bash
# Run the validation levels of docs/development.md with one command, so that every contributor checks the same things.
#
#	hack/gate.sh [--base REV] [--keep-going] [--summary FILE] [--dry-run] LEVEL...
#
# Levels (several may be given; a step that two levels share runs once):
#   docs    go vet ./..., the tests of the site module, and a site build with its link check (without the WebAssembly
#           binary, which the playground checks cover).
#   code    go build ./..., go vet ./..., and go test of the packages that changed since REV and of every package that
#           depends on them (hack/affected-tests.sh, which uses go list), the longest packages first.
#   engine  code, with the default-tier tests replaced by the same packages in the long tier (-tags zp_long, which
#           includes the default tier), then go generate ./internal/backend/... ./bench ./internal/frontend/parser
#           ./spec/language with a check that it changed nothing, hack/test-alpha-parsers.sh and
#           hack/test-reference-tags.sh, the last only when the affected packages include internal/engine,
#           internal/engine/state or internal/engine/document (the reference tags build the engine; it also fails
#           closed: it is planned when the affected packages cannot be computed).
#   full    only the long tier of the affected packages (the test-long step that engine also plans; a step that two
#           levels share runs once).
#   all     go test -tags zp_long ./... (the whole tree, in the long tier), with the longest packages named first,
#           for a release; it replaces the affected-package test steps that engine and full plan.
#
# The gate runs what the change can affect and nothing more: the generate step only when the affected computation
# reports a generator, a generator input or a generated file, alpha-parsers only when it reports a codegen or runtime
# library, the site steps only when docs/, spec/, site/ or README.md changed since REV. vet and build always run. A
# step that is left out is planned and recorded as "NAME skipped (REASON)", never silently. Every condition fails
# closed: when the affected packages or the changed files cannot be computed, the step runs, and the test steps then
# run every package.
#
# REV is the base of the change (default origin/main); the changed packages are those of REV..HEAD, so commit before
# running. Steps run in order and the gate stops at the first failure, so that a heavy step does not run after a
# failed one; --keep-going runs them all. One line per step (name, exit code, seconds) goes to the summary file
# (default: gate-summary.<pid>.txt in $TMPDIR) and is printed at the end. The exit status is non-zero if a step failed.
# --dry-run prints the steps and runs nothing.
#
# For callers (scripts, lock wrappers): the gate never prompts and reads no standard input. Exit status 0: every step
# passed; 1: a step failed (the summary names it); 2: a usage error, before any step ran. The summary file has one
# tab-separated line per step, "NAME<TAB>exit N<TAB>Ss" for a step that ran and "NAME<TAB>skipped (REASON)" for one that
# was left out, in plan order; with a fail-fast stop the failed step is the last line that ran.
#
# The gate exports ZP_MUTANTS_BASE (the base revision) for the tests that run mutants; see docs/development.md.
# GATE_SHARDS (default 2; 1 disables) is the number of concurrent shards of the tests of internal/backend/codegen/golang/print.
# Environment: GATE_NICE (default 0) is the nice(1) increment of every step, and GOMAXPROCS, GOFLAGS and the other Go
# variables pass through. GATE_SELFTEST_FAIL=1 adds a step that fails first, to show that the gate stops.
# GATE_AFFECTED_CMD replaces hack/affected-tests.sh (GATE_AFFECTED_CMD=false shows that a failed computation runs everything).
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
	docs | code | engine | full | all) levels+=("$1"); shift ;;
	*) echo "gate: unknown argument $1 (levels: docs code engine full all)" >&2; exit 2 ;;
	esac
done
[ ${#levels[@]} -gt 0 ] || { echo "usage: hack/gate.sh [--base REV] [--keep-going] [--summary FILE] [--dry-run] docs|code|engine|full|all..." >&2; exit 2; }

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

# The packages whose tests take longest, longest first. go test starts the test runs of its packages in the order of
# its arguments, at most -p (GOMAXPROCS) at a time, so a long package named late starts after the short ones and ends
# the run alone; naming these first lets them overlap with each other and with the rest. Only the order changes: the
# same packages run with the same flags. A package that no longer exists makes go test fail, so the list cannot go
# stale silently.
export GATE_HEAVY='zeroparse.dev/zp/internal/backend/codegen/golang/print zeroparse.dev/zp/internal/backend/codegen/common zeroparse.dev/zp/internal/engine zeroparse.dev/zp/internal/backend/codegen/typescript/print zeroparse.dev/zp/spec/language zeroparse.dev/zp/codegen/go/lib zeroparse.dev/zp/internal/testing/complexitytest'

# affected_cmd GO-TEST-ARGS: go test over the packages of the changes since $GATE_BASE and their reverse dependencies,
# those of $GATE_HEAVY first. internal/backend/codegen/golang/print, the longest package, runs as GATE_SHARDS (default 2) concurrent
# invocations, each with the top-level tests (Test, Fuzz and Example functions, from go test -list) of one shard, beside a
# third invocation for the other packages; only when it is affected. The sharding fails closed: when the list fails, is
# empty or the shards do not add up to it, the package runs unsharded with the others. A failed shard fails the step.
affected_cmd() {
	local tpl
	IFS= read -r -d '' tpl <<'EOS' || true
args='@ARGS@'
raw=$("${GATE_AFFECTED_CMD:-hack/affected-tests.sh}" "$GATE_BASE" HEAD) || { echo "gate: the affected packages could not be computed; testing every package" >&2; exec go test -count=1 $args $GATE_HEAVY ./...; }
pkgs=$(printf "%s\n" "$raw" | grep -v "^#" || true)
[ -n "$pkgs" ] || { echo "gate: no package affected since $GATE_BASE"; exit 0; }
echo "$pkgs" | sed "s/^/gate: affected /"
first=$(for h in $GATE_HEAVY; do printf "%s\n" "$pkgs" | grep -Fx "$h" || true; done)
rest=$(printf "%s\n" "$pkgs" | grep -Fvx -f <(printf "%s\n" $GATE_HEAVY) || true)
shardpkg=zeroparse.dev/zp/internal/backend/codegen/golang/print
shards=${GATE_SHARDS:-2}
if [ "$shards" -gt 1 ] && printf "%s\n" "$pkgs" | grep -Fxq "$shardpkg"; then
	list=$(go test $args -list . "$shardpkg" 2>/dev/null | grep -E "^(Test|Fuzz|Example)" | sort || true)
	n=$(printf "%s\n" "$list" | grep -c . || true)
	if [ "$n" -ge "$shards" ]; then
		d=$(mktemp -d "${TMPDIR:-/tmp}/gate-shards.XXXXXX")
		k=0; joined=
		while [ "$k" -lt "$shards" ]; do
			part=$(printf "%s\n" "$list" | awk -v k="$k" -v s="$shards" "NR % s == k")
			printf "%s\n" "$part" > "$d/part.$k"
			joined="$joined$part"$'\n'
			k=$((k + 1))
		done
		if [ "$(printf "%s" "$joined" | sort)" = "$list" ]; then
			echo "gate: $shardpkg: $n tests in $shards shards"
			others=$(printf "%s\n" $first $rest | grep -Fvx "$shardpkg" || true)
			k=0
			while [ "$k" -lt "$shards" ]; do
				re="^($(paste -sd"|" "$d/part.$k"))\$"
				( go test -count=1 $args -run "$re" "$shardpkg" >"$d/out.$k" 2>&1; echo $? >"$d/rc.$k" ) &
				k=$((k + 1))
			done
			if [ -n "$others" ]; then ( go test -count=1 $args $others >"$d/out.o" 2>&1; echo $? >"$d/rc.o" ) & else echo 0 >"$d/rc.o"; : >"$d/out.o"; fi
			wait
			bad=0
			for t in $(seq 0 $((shards - 1))) o; do
				echo "gate: == shard $t (exit $(cat "$d/rc.$t" 2>/dev/null || echo 1))"
				cat "$d/out.$t" 2>/dev/null
				[ "$(cat "$d/rc.$t" 2>/dev/null || echo 1)" = 0 ] || bad=1
			done
			rm -rf "$d"
			exit $bad
		fi
		echo "gate: the shards of $shardpkg do not add up to its test list; running it unsharded" >&2
		rm -rf "$d"
	else
		echo "gate: no test list for $shardpkg; running it unsharded" >&2
	fi
fi
go test -count=1 $args $first $rest
EOS
	printf '%s' "${tpl//@ARGS@/$1}"
}
# snap hashes the tracked changes and the untracked files, to see that a step left the tree as it found it.
snap_def='snap() { { git diff HEAD --binary; git ls-files -o --exclude-standard -z | xargs -0 shasum 2>/dev/null || true; } | shasum; }'

[ "${GATE_SELFTEST_FAIL:-0}" != 1 ] || step selftest 'a step that fails (GATE_SELFTEST_FAIL)' false

# What a change affects, computed once and only when a conditional step asks. Every answer fails closed: when the
# affected packages or the changed files cannot be computed, a step is planned. aff_has_pkg PKG..., aff_has_marker NAME
# (a "# NAME" line of hack/affected-tests.sh) and docs_changed succeed when the step has to run.
aff_state=
aff_raw=
chg_state=
chg=
affected_load() {
	[ -z "$aff_state" ] || return 0
	if aff_raw=$("${GATE_AFFECTED_CMD:-hack/affected-tests.sh}" "$base" HEAD 2>/dev/null); then aff_state=ok; else aff_state=failed; fi
	if chg=$(git diff --name-only "$base" HEAD 2>/dev/null); then chg_state=ok; else chg_state=failed; fi
}
aff_has_pkg() { # aff_has_pkg PATH...: one of the module-relative packages is affected
	affected_load
	[ "$aff_state" = ok ] || return 0
	local p
	for p in "$@"; do printf '%s\n' "$aff_raw" | grep -qx "zeroparse.dev/zp/$p" && return 0; done
	return 1
}
aff_has_marker() { affected_load; [ "$aff_state" = ok ] || return 0; printf '%s\n' "$aff_raw" | grep -qx "# $1"; }
docs_changed() { affected_load; [ "$chg_state" = ok ] || return 0; printf '%s\n' "$chg" | grep -qE '^(docs/|spec/|site/|README\.md$)'; }
# skipstep NAME REASON: a step that the gate leaves out, listed in the plan and the summary as "NAME skipped (REASON)".
skipstep() { step "$1" "skipped ($2)" ":skip:$2"; }

want() { local l; for l in "${levels[@]}"; do [ "$l" = "$1" ] && return 0; done; return 1; }

if want docs; then
	step vet 'go vet ./...' 'go vet ./...'
	if docs_changed; then
		step site-test 'cd site && go test ./...' 'cd site && go test -count=1 ./...'
		step site-build 'site/build.sh -wasm=false (builds into a temporary directory, checks links)' 'out=$(cd -P "$(mktemp -d "${TMPDIR:-/tmp}/gate-site.XXXXXX")" && pwd) && trap "rm -rf \"$out\"" EXIT && site/build.sh -out "$out/dist" -wasm=false'
	else
		skipstep site-test "no change under docs/, spec/, site/ or README.md since $base"
		skipstep site-build "no change under docs/, spec/, site/ or README.md since $base"
	fi
fi
if want code || want engine; then
	step build 'go build ./...' 'go build ./...'
	step vet 'go vet ./...' 'go vet ./...'
	if ! want engine && ! want full && ! want all; then
		step test 'go test of the affected packages (internal/backend/codegen/golang/print in shards when affected)' "$(affected_cmd '-timeout 45m')"
	fi
fi
if want engine || want full || want all; then
	if want all; then
		step test-all 'go test -tags zp_long ./... (the whole tree, the longest packages first)' 'go test -count=1 -tags zp_long -timeout 90m $GATE_HEAVY ./...'
	else
		step test-long 'go test -tags zp_long of the affected packages (internal/backend/codegen/golang/print in shards when affected)' "$(affected_cmd '-tags zp_long -timeout 90m')"
	fi
fi
if want engine || want all; then
	if aff_has_marker generate; then
		step generate 'go generate ./internal/backend/... ./bench ./internal/frontend/parser ./spec/language; the tree must not change' "$snap_def"'
before=$(snap)
go generate ./internal/backend/... ./bench ./internal/frontend/parser ./spec/language
[ "$(snap)" = "$before" ] || { echo "gate: go generate changed the tree; commit the regenerated files" >&2; git status --short >&2; exit 1; }'
	else
		skipstep generate "no generator, generator input or generated file is affected since $base"
	fi
	if aff_has_marker alpha; then
		step alpha-parsers 'hack/test-alpha-parsers.sh' hack/test-alpha-parsers.sh
	else
		skipstep alpha-parsers "no codegen or runtime library is affected since $base"
	fi
	if aff_has_pkg internal/engine internal/engine/state internal/engine/document; then
		step reference-tags 'hack/test-reference-tags.sh (tags 4 at a time)' hack/test-reference-tags.sh
	else
		skipstep reference-tags "internal/engine, internal/engine/state and internal/engine/document are not affected since $base"
	fi
fi

if [ "$dry" = 1 ]; then
	echo "gate: base $base; steps:"
	i=0
	for n in "${names[@]}"; do printf '  %-15s %s\n' "$n" "${descs[$i]}"; i=$((i + 1)); done
	case " ${names[*]} " in *" test"* | *" test-long "* | *" test-all "*) echo "gate: affected packages since $base:"; "${GATE_AFFECTED_CMD:-hack/affected-tests.sh}" "$base" HEAD | sed 's/^/  /' ;; esac
	if [ "${GATE_SHARDS:-2}" -gt 1 ] && case " ${names[*]} " in *" test"* | *" test-long "*) true ;; *) false ;; esac && "${GATE_AFFECTED_CMD:-hack/affected-tests.sh}" "$base" HEAD 2>/dev/null | grep -qx zeroparse.dev/zp/internal/backend/codegen/golang/print; then
		echo "gate: internal/backend/codegen/golang/print is affected: its tests run in ${GATE_SHARDS:-2} concurrent shards (go test -list), beside the other packages"
	fi
	exit 0
fi

git rev-parse --verify --quiet "$base^{commit}" >/dev/null || { echo "gate: base $base is not a revision" >&2; exit 2; }
if [ -n "$(git status --porcelain)" ]; then echo "gate: warning: the tree has uncommitted changes; the changed packages are those of $base..HEAD" >&2; fi
export GATE_BASE=$base
# Mutant runners skip when what they depend on is unchanged since the base (internal/testing/mutanttest).
export ZP_MUTANTS_BASE=$base
: >"$summary"
failed=0
i=0
for n in "${names[@]}"; do
	if [ "${cmds[$i]#:skip:}" != "${cmds[$i]}" ]; then
		reason=${cmds[$i]#:skip:}
		echo "gate: $n skipped ($reason)" >&2
		printf '%s\tskipped (%s)\n' "$n" "$reason" >>"$summary"
		i=$((i + 1))
		continue
	fi
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
