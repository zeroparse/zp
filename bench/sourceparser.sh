#!/bin/sh
# Grammar-source frontend benchmark: runs BenchmarkFrontend of internal/syntax in interleaved rounds
# and prints the raw results, which benchstat summarizes:
#
#	bench/sourceparser.sh > frontend.txt && benchstat -col /fe frontend.txt
#
# Every round runs each source with each frontend once, in an order that rotates from round to
# round, so that drift of the machine falls on all frontends alike. manual-aa is the hand-written
# frontend again: its difference from manual is the noise of the run. ROUNDS (default 20) and
# BENCHTIME (default 500ms) set the size of the run, SOURCES (a regular expression, default all)
# the sources. The bytes of a parse that reuses the memory of earlier parses depend on the number of
# iterations, so compare B/op only between runs of the same BENCHTIME.
set -eu
sourcebench_repo=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
sourcebench_dir=$(mktemp -d "${TMPDIR:-/tmp}/zp-sourcebench.XXXXXX")
trap 'rm -rf "$sourcebench_dir"' 0
sourcebench_rounds=${ROUNDS:-20}
sourcebench_time=${BENCHTIME:-500ms}
sourcebench_sources=${SOURCES:-.*}
case "$sourcebench_rounds" in ''|*[!0-9]*|0) echo 'ROUNDS must be a positive integer.' >&2; exit 1;; esac
cd "$sourcebench_repo"
go test -c -o "$sourcebench_dir/syntax.test" ./internal/syntax
cd internal/syntax
"$sourcebench_dir/syntax.test" -test.run '^TestFrontendSamples$' >&2
sourcebench_frontends='manual manual-aa generated-typed'
sourcebench_round=0
while [ "$sourcebench_round" -lt "$sourcebench_rounds" ]; do
	# Rotate the order of the frontends by the round number.
	set -- $sourcebench_frontends
	sourcebench_shift=$((sourcebench_round % $#))
	while [ "$sourcebench_shift" -gt 0 ]; do
		sourcebench_first=$1
		shift
		set -- "$@" "$sourcebench_first"
		sourcebench_shift=$((sourcebench_shift - 1))
	done
	for sourcebench_fe in "$@"; do
		"$sourcebench_dir/syntax.test" -test.run '^$' \
			-test.bench "^BenchmarkFrontend$/^src=($sourcebench_sources)\$/^fe=$sourcebench_fe\$" \
			-test.benchtime "$sourcebench_time" -test.count 1
	done
	sourcebench_round=$((sourcebench_round + 1))
done
