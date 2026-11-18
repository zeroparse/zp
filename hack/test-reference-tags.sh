#!/bin/sh
# Vet and test the tree under each reference build tag, so that a variant cannot rot unnoticed.
#
# A reference tag (zp_budget_controls_reference, zp_reference_*, zp_snapshot_*_reference) selects the simpler or
# the former form of a mechanism that the default build optimizes, for same-source comparisons; a candidate tag
# (zp_snapshot_position_overlays) selects an opt-in representation that the default build has not adopted. The
# default build does not compile the files of either, so a change to the engine or its tests can break a variant
# without failing the root suite. For each tag the script runs go vet and go test -short over the engine
# packages, the public API tests and every other package with a file constrained by the tag:
#
#	hack/test-reference-tags.sh [tag...]
#
# Without arguments it checks every tag of the tree: each zp_ tag named in a build constraint except zp_long, the
# long test tier, and zp_feature_witness, which the tests of generated parsers set themselves. It rejects a given tag that no build constraint names, so a misspelt tag cannot pass as the
# default build. It exits non-zero when a tag fails to build or pass. A test whose feature a tag removes skips
# under that tag. Tags run 4 at a time (TAG_JOBS), each with its output printed under its own heading once all have ended.
# The run takes tens of minutes.
set -u
cd "$(dirname "$0")/.." || exit 1
constrained() { # constrained [grep flags]: the Go files whose build constraint matches the last argument
	grep -rl --include='*.go' "$@" . | grep -v '^\./\.'
}
if [ $# -gt 0 ]; then
	tags="$*"
else
	tags=$(constrained '^//go:build .*zp_' | xargs grep -h '^//go:build' | grep -o 'zp_[a-z0-9_]*' | sort -u | grep -v -e '^zp_long$' -e '^zp_feature_witness$')
fi
[ -n "$tags" ] || { echo "test-reference-tags: no reference tags found" >&2; exit 1; }
for t in $tags; do
	case $t in
	zp_long | *[!a-z0-9_]* | '') ok= ;;
	*) ok=$(constrained -w "^//go:build .*$t") ;;
	esac
	[ -n "$ok" ] || { echo "test-reference-tags: $t: not a reference tag of the tree" >&2; exit 2; }
done
# Each tag runs in its own subshell, at most TAG_JOBS (default 4) at a time, with its output in a log that is
# printed, with a heading, after all tags ended, so that the lines of two tags do not mix. The Go build cache is safe
# under concurrent use; set GOFLAGS=-p=N to bound the compile and test processes of each tag.
jobs=${TAG_JOBS:-4}
dir=$(mktemp -d "${TMPDIR:-/tmp}/reference-tags.XXXXXX") || exit 1
trap 'rm -rf "$dir"' EXIT
check() { # check TAG: vet then test one tag; the status is the result
	t=$1
	pkgs=$(constrained -w "^//go:build .*$t" | xargs -n1 dirname | sort -u)
	go vet -tags "$t" ./internal/runtime/engine/... ./engine $pkgs || { echo "test-reference-tags: $t: vet failed"; return 1; }
	go test -count=1 -short -tags "$t" ./internal/runtime/engine/... ./engine $pkgs || { echo "test-reference-tags: $t: tests failed"; return 1; }
}
n=0
for t in $tags; do
	( check "$t" >"$dir/$t.log" 2>&1; echo $? >"$dir/$t.rc" ) &
	n=$((n + 1))
	if [ "$n" -ge "$jobs" ]; then wait; n=0; fi
done
wait
bad=0
for t in $tags; do
	rc=$(cat "$dir/$t.rc" 2>/dev/null || echo 1)
	echo "test-reference-tags: == $t (exit $rc)" >&2
	sed "s/^/[$t] /" "$dir/$t.log" >&2
	[ "$rc" = 0 ] || bad=1
done
[ "$bad" = 0 ] && echo "test-reference-tags: all tags pass" >&2
exit "$bad"
