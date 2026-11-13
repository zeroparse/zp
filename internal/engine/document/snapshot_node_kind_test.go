package document_test

import (
	"fmt"
	"reflect"
	"testing"

	"zeroparse.dev/zp/core"
	. "zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/engine/state"
)

// compareSnapshotNode checks that Kind, Len and Value of a snapshot node answer as those of the node
// it was frozen from, and follows the elements and the node fields of both.
func compareSnapshotNode(t *testing.T, path string, s SnapshotNode, n *core.Node, kinds map[core.NodeKind]int) {
	t.Helper()
	if n == nil {
		if s.Valid() || s.Len() != 0 || s.Value() != nil {
			t.Fatalf("%s: nil node, snapshot %v len %d value %v", path, s.Valid(), s.Len(), s.Value())
		}
		return
	}
	if s.Kind() != n.Kind() || s.Len() != n.Len() || !reflect.DeepEqual(s.Value(), n.Value()) {
		t.Fatalf("%s: snapshot (%v, %d, %v), node (%v, %d, %v)", path, s.Kind(), s.Len(), s.Value(), n.Kind(), n.Len(), n.Value())
	}
	kinds[n.Kind()]++
	for i := range n.Len() {
		compareSnapshotNode(t, fmt.Sprintf("%s[%d]", path, i), s.Child(i), n.Elem(i), kinds)
	}
	for name, fv := range n.Fields() {
		if c, ok := fv.(*core.Node); ok && n.Kind() == core.KindStruct {
			sv, _ := s.Field(name)
			compareSnapshotNode(t, path+"."+name, sv.(SnapshotNode), c, kinds)
		}
	}
}

func TestSnapshotNodeKindLenValueMatchNode(t *testing.T) {
	prog := mustCompile(t, `type Pair = { All []node[string]; Nums []int; Names []string; N int }
rule main = v1:"a" v2:"b" => Pair{All: list($v1, $v2), Nums: list(7, 8), Names: list("x", text($v2)), N: 1}`)
	snapshotVariants(t, func(t *testing.T, o state.ParseOptions) {
		d, err := prog.NewDocumentWith("main", "ab", o)
		if err != nil {
			t.Fatal(err)
		}
		root, err := d.Parse()
		if err != nil {
			t.Fatal(err)
		}
		snap, err := d.SnapshotWith(o)
		if err != nil {
			t.Fatal(err)
		}
		kinds := map[core.NodeKind]int{}
		compareSnapshotNode(t, "root", snap.Root(), root, kinds)
		if kinds[core.KindListValue] == 0 || kinds[core.KindValue] == 0 || kinds[core.KindStruct] == 0 {
			t.Fatalf("the tree does not cover the kinds: %v", kinds)
		}
		// Frozen again from the snapshot, the same answers.
		compareSnapshotNode(t, "materialized", snap.Root(), snap.Materialize(), map[core.NodeKind]int{})
	})
}

func TestSnapshotNodeZeroValue(t *testing.T) {
	var z SnapshotNode
	if z.Len() != 0 || z.Value() != nil {
		t.Fatal("zero value answers", z.Len(), z.Value())
	}
	defer func() {
		if recover() == nil {
			t.Fatal("Kind of the zero value did not panic, as Node.Kind of nil does not return")
		}
	}()
	z.Kind()
}
