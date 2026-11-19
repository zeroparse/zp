#!/usr/bin/env bash
# Rewrites the import blocks of Go files to the repository rule (docs/development.md,
# "Go Style"). Usage: hack/fix-imports.sh [-n] [path ...]; without paths it rewrites
# every tracked Go file, and -n only lists the files it would change.
set -euo pipefail
cd "$(dirname "$0")/.."
exec go run ./internal/testing/importgroups/fiximports "$@"
