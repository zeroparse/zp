#!/usr/bin/env bash
# Rewrites the import blocks of Go files to the repository rule (docs/development.md,
# "Go Style"). Usage: hack/fix-imports.sh [-n] [path ...]; without paths it rewrites
# every tracked Go file, and -n only lists the files it would change and exits with
# status 1 if there are any. Paths are relative to the directory it is run from.
set -euo pipefail
args=()
for a in "$@"; do
	if [[ $a != -* && $a != /* ]]; then
		a=$PWD/$a
	fi
	args+=("$a")
done
cd "$(dirname "$0")/.."
exec go run ./internal/testing/importgroups/fiximports ${args[@]+"${args[@]}"}
