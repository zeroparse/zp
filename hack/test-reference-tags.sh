#!/bin/sh
# Vet and test the engine under each reference build tag, so that a variant cannot rot unnoticed.
#
# The reference tags (zp_reference_*, zp_snapshot_*_reference, zp_snapshot_position_overlays and
# zp_budget_controls_reference) select the simpler or the older form of a mechanism that the default build
# optimizes. The default build does not compile them, so a change to the engine or its tests can break one
# without failing the root suite. For each tag the script runs go vet and go test -short over the engine
# packages and the public API tests:
#
#	hack/test-reference-tags.sh [tag...]
#
# Without arguments it checks every tag. It fails when a tag named in a build constraint of the tree is not
# in its list (add the new tag to the list below), and exits non-zero when a tag fails to build or pass.
# The run takes some tens of minutes, so run it after a change to the engine's reference variants or to the
# tests of the engine packages, not on every commit.
set -u
cd "$(dirname "$0")/.." || exit 1
tags="zp_budget_controls_reference zp_reference_document_pieces zp_reference_document_vm_values zp_reference_sparse_memo zp_reference_stream_frames zp_reference_stream_prefix zp_reference_stream_stack zp_reference_stream_storage zp_reference_stream_text zp_reference_stream_undo zp_snapshot_child_chunks_reference zp_snapshot_position_overlays zp_snapshot_sharing_reference"
found=$(grep -rh '^//go:build' --include='*.go' internal runtime 2>/dev/null | grep -o 'zp_[a-z0-9_]*' | sort -u | grep -v '^zp_long$')
bad=0
for t in $found; do
	case " $tags " in
	*" $t "*) ;;
	*) echo "test-reference-tags: tag $t is used in a build constraint but is not in the list of hack/test-reference-tags.sh" >&2; bad=1 ;;
	esac
done
[ "$bad" = 0 ] || exit 1
[ $# -gt 0 ] && tags="$*"
for t in $tags; do
	echo "test-reference-tags: $t" >&2
	go vet -tags "$t" ./internal/engine/... ./internal/apitest || { echo "test-reference-tags: $t: vet failed" >&2; bad=1; continue; }
	go test -count=1 -short -tags "$t" ./internal/engine/... ./internal/apitest || { echo "test-reference-tags: $t: tests failed" >&2; bad=1; }
done
[ "$bad" = 0 ] && echo "test-reference-tags: all tags pass" >&2
exit "$bad"
