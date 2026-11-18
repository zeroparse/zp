//go:build !zp_snapshot_sharing_reference

package document_test

import (
	"runtime"
	"testing"

	. "zeroparse.dev/zp/internal/runtime/engine"
	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/pkg/runtime/value"
	valueimpl "zeroparse.dev/zp/pkg/runtime/value/impl"
)

func TestSnapshotSharingCyclesAndTypedNil(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = "a"`)
	d, err := prog.NewDocument("main", "a")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := d.Parse()
	if err != nil {
		t.Fatal(err)
	}
	nodes.D(legacy).Children = []*value.Node{legacy, nil}
	nodes.D(legacy).Fields = valueimpl.Fields{{Name: "self", Value: legacy}, {Name: "typedNil", Value: (*value.Node)(nil)}, {Name: "int", Value: 7}, {Name: "text", Value: "a"}}
	first, err := d.SnapshotWith(ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.SnapshotWith(ParseOptions{})
	if err != nil {
		t.Fatal("shared cycle", err)
	}
	if second.Root().Child(0) != second.Root() {
		t.Fatal("cycle identity")
	}
	if v, ok := second.Root().Field("typedNil"); !ok || v != (SnapshotNode{}) {
		t.Fatal("typed nil")
	}
	materialized := second.Materialize()
	if nodes.D(materialized).Children[0] != materialized || nodes.D(materialized).Fields[0].Value != materialized {
		t.Fatal("materialized cycle")
	}
	nodes.D(legacy).Fields[3].Value = "changed"
	if first.Root().Fields()[3].Value != "a" || second.Root().Fields()[3].Value != "a" {
		t.Fatal("legacy mutation reached snapshots")
	}
	runtime.KeepAlive(first)
}
