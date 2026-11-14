package value

import (
	"testing"
	"unsafe"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
)

// KindOfShape and core.Node.Kind map shapes to kinds by separate copies of one table.
func TestKindOfShape(t *testing.T) {
	kinds := []core.NodeKind{core.KindString, core.KindTuple, core.KindList, core.KindError, core.KindStruct, core.KindTupleValue, core.KindListValue, core.KindValue}
	for _, k := range kinds {
		n := NewNode(k, 0, 0, "", nil, "T", nil, 1)
		if got := KindOfShape(ShapeOf(n)); got != k || n.Kind() != k {
			t.Errorf("node of kind %v: KindOfShape = %v, Kind = %v", k, got, n.Kind())
		}
	}
	n := Ptr(Data{Kind: coreimpl.MissingKind})
	if KindOfShape(ShapeOf(n)) != n.Kind() {
		t.Errorf("missing token: KindOfShape = %v, Kind = %v", KindOfShape(ShapeOf(n)), n.Kind())
	}
}

// D converts a *core.Node to the representation that it holds as its only field.
func TestDataConversion(t *testing.T) {
	if unsafe.Sizeof(core.Node{}) != unsafe.Sizeof(Data{}) {
		t.Fatalf("core.Node is %d bytes, Data %d", unsafe.Sizeof(core.Node{}), unsafe.Sizeof(Data{}))
	}
	n := NewNode(core.KindString, 3, 5, "ab", nil, "", nil, nil)
	if d := D(n); d.Start != 3 || d.End != 5 || d.Text != "ab" || n.Text() != "ab" || n.Start() != 3 {
		t.Fatalf("D(n) = %+v, n = %v [%d,%d)", *d, n, n.Start(), n.End())
	}
	c := Of(*D(n))
	if c.Text() != "ab" || c.End() != 5 {
		t.Fatalf("Of: %v [%d,%d)", &c, c.Start(), c.End())
	}
}
