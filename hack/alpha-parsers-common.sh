# Shared by test-alpha-parsers.sh and sync-alpha-parsers.sh; sourced, not run.
#
# Sets $root (this tree), $dir (the alpha-parsers checkout) and $work (a temporary directory, removed on exit)
# holding a go.work file that joins this tree and the checkout. $ZP_ALPHA_PARSERS names the
# checkout (default: ../alpha-parsers); ZP_ALPHA_PARSERS=skip makes the caller succeed without doing anything.
# alpha_options_drift prints the differences between the generator options of testdata/grammars/generate.txt and
# the go:generate lines of the checkout's generate.go, and fails when there is one.

alpha_setup() {
	root=$(cd "$(dirname "$0")/.." && pwd) || exit 1
	if [ "${ZP_ALPHA_PARSERS:-}" = skip ]; then
		echo "alpha-parsers: ZP_ALPHA_PARSERS=skip; skipped" >&2
		exit 0
	fi
	dir=${ZP_ALPHA_PARSERS:-$root/../alpha-parsers}
	if [ ! -f "$dir/test.sh" ]; then
		echo "alpha-parsers: no checkout at $dir" >&2
		echo "alpha-parsers: clone https://github.com/zeroparse/alpha-parsers there, or set ZP_ALPHA_PARSERS to its directory (ZP_ALPHA_PARSERS=skip to skip the check)" >&2
		exit 1
	fi
	dir=$(cd "$dir" && pwd) || exit 1
	echo "alpha-parsers: checkout $dir, tree $root" >&2
	work=$(mktemp -d) || exit 1
	trap 'rm -rf "$work"' EXIT
	# The replace directives of this tree's go.work, with their directories made absolute.
	{
		printf 'go 1.27.1\n\nuse (\n\t%s\n\t%s\n)\n' "$root" "$dir"
		grep '^replace ' "$root/go.work" | sed "s#=> \./#=> $root/#"
	} >"$work/go.work" || exit 1
}

alpha_options_drift() {
	grep -v '^#' "$root/testdata/grammars/generate.txt" | grep -v '^$' | sort >"$work/want" || return 1
	sed -n 's#^//go:generate go run zeroparse.dev/zp/cmd/zp gen ##p' "$dir/generate.go" |
		sed -E 's# -o ([^ ]+)$# \1#; s#-g [^ ]+ ##; s#-module [^ ]+ ##' |
		awk '{ out = $NF; $NF = ""; sub(/ +$/, ""); print out " " $0 }' | sort >"$work/have" || return 1
	cmp -s "$work/want" "$work/have" && return 0
	echo "alpha-parsers: the options of generate.go differ from testdata/grammars/generate.txt (< this tree, > the checkout):" >&2
	diff "$work/want" "$work/have" | grep '^[<>]' >&2
	return 1
}
