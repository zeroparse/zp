#!/bin/sh
# Run the tests of the alpha-parsers checkout against this working tree.
#
# alpha-parsers (zeroparse.dev/alpha-parsers) holds the ready-made parsers, generated from the grammars of
# testdata/grammars/, which are the source of truth: the script first checks that every grammar, every sample
# input and duckdb/inspect.go of the checkout equals this tree's copy and that its generator options are those of
# testdata/grammars/generate.txt, and fails when one differs. It is a separate repository, so a change of the generator or the engine is checked
# against it from a checkout next to this one, joined by a go.work file:
#
#	hack/test-alpha-parsers.sh [go test arguments]
#
# $ZP_ALPHA_PARSERS names the checkout (default: ../alpha-parsers). The script fails when there is no checkout:
# a check that is skipped without notice hides a stale checkout. ZP_ALPHA_PARSERS=skip skips it on purpose.
# hack/sync-alpha-parsers.sh brings a stale checkout up to date. The script tests the tree it is run from.
set -u
. "$(dirname "$0")/alpha-parsers-common.sh" || exit 1
alpha_setup
# Sync check: the options of generate.go against testdata/grammars/generate.txt, and testdata/grammars/<n>/ here against <n>/ (grammar, duckdb/inspect.go) and <n>/testdata/ (inputs) there.
bad=0
alpha_options_drift || bad=1
for d in "$root"/testdata/grammars/*/; do
	n=$(basename "$d")
	for f in "$d"*.zp "$d"inspect.go; do
		[ -f "$f" ] || continue
		cmp -s "$f" "$dir/$n/$(basename "$f")" || { echo "alpha-parsers: $n/$(basename "$f") differs from testdata/grammars/$n" >&2; bad=1; }
	done
	for f in "$d"*.txt; do
		[ -f "$f" ] || continue
		cmp -s "$f" "$dir/$n/testdata/$(basename "$f")" || { echo "alpha-parsers: $n/testdata/$(basename "$f") differs from testdata/grammars/$n" >&2; bad=1; }
	done
	for f in "$dir/$n/testdata/"*.txt; do
		[ -f "$f" ] || continue
		[ -f "$d$(basename "$f")" ] || { echo "alpha-parsers: $n/testdata/$(basename "$f") is not in testdata/grammars/$n" >&2; bad=1; }
	done
done
[ "$bad" = 0 ] || { echo "alpha-parsers: run hack/sync-alpha-parsers.sh to copy and regenerate; the options of generate.go are edited by hand (testdata/grammars is the source of truth)" >&2; exit 1; }
cd "$dir" || exit 1
GOWORK="$work/go.work" ./test.sh "$@"
