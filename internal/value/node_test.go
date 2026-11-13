package value

import (
	"testing"
	"unsafe"

	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

// KindOfShape and parse.Node.Kind map shapes to kinds by separate copies of one table.
func TestKindOfShape(t *testing.T) {
	kinds := []parse.NodeKind{parse.KindString, parse.KindTuple, parse.KindList, parse.KindError, parse.KindStruct, parse.KindTupleValue, parse.KindListValue, parse.KindValue}
	for _, k := range kinds {
		n := NewNode(k, 0, 0, "", nil, "T", nil, 1)
		if got := KindOfShape(ShapeOf(n)); got != k || n.Kind() != k {
			t.Errorf("node of kind %v: KindOfShape = %v, Kind = %v", k, got, n.Kind())
		}
	}
	n := Ptr(Data{Kind: noderep.MissingKind})
	if KindOfShape(ShapeOf(n)) != n.Kind() {
		t.Errorf("missing token: KindOfShape = %v, Kind = %v", KindOfShape(ShapeOf(n)), n.Kind())
	}
}

// D converts a *parse.Node to the representation that it holds as its only field.
func TestDataConversion(t *testing.T) {
	if unsafe.Sizeof(parse.Node{}) != unsafe.Sizeof(Data{}) {
		t.Fatalf("parse.Node is %d bytes, Data %d", unsafe.Sizeof(parse.Node{}), unsafe.Sizeof(Data{}))
	}
	n := NewNode(parse.KindString, 3, 5, "ab", nil, "", nil, nil)
	if d := D(n); d.Start != 3 || d.End != 5 || d.Text != "ab" || n.Text() != "ab" || n.Start() != 3 {
		t.Fatalf("D(n) = %+v, n = %v [%d,%d)", *d, n, n.Start(), n.End())
	}
	c := Of(*D(n))
	if c.Text() != "ab" || c.End() != 5 {
		t.Fatalf("Of: %v [%d,%d)", &c, c.Start(), c.End())
	}
}
