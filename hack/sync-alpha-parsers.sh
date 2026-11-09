#!/bin/sh
# Bring the alpha-parsers checkout up to date with this tree: copy the grammars, the sample inputs and
# duckdb/inspect.go of testdata/grammars/, remove the sample inputs this tree no longer has, and regenerate every
# parser with the generator of this tree (go generate in the checkout, under the go.work file that
# test-alpha-parsers.sh uses too). It leaves the changes uncommitted in the checkout. The generator options of
# generate.go are not rewritten: when they differ from testdata/grammars/generate.txt the script fails.
#
#	hack/sync-alpha-parsers.sh
#
# $ZP_ALPHA_PARSERS names the checkout (default: ../alpha-parsers); see test-alpha-parsers.sh.
set -u
. "$(dirname "$0")/alpha-parsers-common.sh" || exit 1
alpha_setup
for d in "$root"/testdata/grammars/*/; do
	n=$(basename "$d")
	for f in "$d"*.zp "$d"inspect.go; do
		[ -f "$f" ] || continue
		cp "$f" "$dir/$n/" || exit 1
	done
	mkdir -p "$dir/$n/testdata" || exit 1
	for f in "$d"*.txt; do
		[ -f "$f" ] || continue
		cp "$f" "$dir/$n/testdata/" || exit 1
	done
	for f in "$dir/$n/testdata/"*.txt; do
		[ -f "$f" ] || continue
		[ -f "$d$(basename "$f")" ] || rm "$f" || exit 1
	done
done
alpha_options_drift || exit 1
cd "$dir" || exit 1
GOWORK="$work/go.work" go generate . || exit 1
git status --short >&2
