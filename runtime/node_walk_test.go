package runtime

import (
	"errors"
	"reflect"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/noderep"
)

func TestWalkEdgeOrderAndSharedParents(t *testing.T) {
	shared := nodeOfData(&Data{Text: "shared"})
	left := nodeOfData(&Data{Children: []*core.Node{shared}})
	right := nodeOfData(&Data{Fields: noderep.Fields{{Name: "nested", Value: []any{shared, nil, 7}}}})
	root := nodeOfData(&Data{Children: []*core.Node{left, right, nil}, Fields: noderep.Fields{{Name: "z", Value: false}, {Name: "a", Value: shared}}})
	var got []string
	var parents []*core.Node
	err := core.Walk(root, func(v core.WalkVisit) (core.WalkAction, error) {
		switch v.Edge.Kind {
		case core.WalkRoot:
			got = append(got, "root")
		case core.WalkChild:
			got = append(got, "child")
		case core.WalkField:
			got = append(got, v.Edge.FieldName)
		case core.WalkListElement:
			got = append(got, "list")
		}
		if v.Node == shared {
			parents = append(parents, v.Parent)
		}
		return core.WalkContinue, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"root", "child", "child", "child", "nested", "list", "list", "list", "child", "a", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visits = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(parents, []*core.Node{left, right, root}) {
		t.Fatal("shared edge parents lost")
	}
	if D(root).Fields[0].Name != "z" {
		t.Fatal("walk sorted fields in place")
	}
}

func TestWalkCyclesAndControls(t *testing.T) {
	n := nodeOfData(&Data{})
	D(n).Children = []*core.Node{n}
	var cycle *core.CycleError
	err := core.Walk(n, func(core.WalkVisit) (core.WalkAction, error) { return core.WalkContinue, nil })
	if !errors.As(err, &cycle) || len(cycle.Path) != 2 || cycle.Path[1].Kind != core.WalkChild {
		t.Fatalf("cycle = %#v, %v", cycle, err)
	}
	list := make([]any, 1)
	list[0] = list
	n = nodeOfData(&Data{Fields: noderep.Fields{{Name: "x", Value: list}}})
	err = core.Walk(n, func(core.WalkVisit) (core.WalkAction, error) { return core.WalkContinue, nil })
	if !errors.As(err, &cycle) || len(cycle.Path) != 3 {
		t.Fatalf("list cycle = %v", err)
	}
	sentinel := errors.New("callback")
	if err := core.Walk(n, func(core.WalkVisit) (core.WalkAction, error) { return core.WalkContinue, sentinel }); err != sentinel {
		t.Fatal(err)
	}
	for _, action := range []core.WalkAction{core.WalkSkip, core.WalkStop} {
		count := 0
		if err := core.Walk(n, func(core.WalkVisit) (core.WalkAction, error) { count++; return action, nil }); err != nil || count != 1 {
			t.Fatalf("control %d = %d, %v", action, count, err)
		}
	}
	count := 0
	if err := core.Walk(nil, func(v core.WalkVisit) (core.WalkAction, error) {
		count++
		if v.Value != nil || v.Node != nil || v.Depth != 0 {
			t.Fatal(v)
		}
		return core.WalkContinue, nil
	}); err != nil || count != 1 {
		t.Fatal(err, count)
	}
}

func TestWalkDeep(t *testing.T) {
	n := nodeOfData(&Data{})
	const depth = 20000
	for i := 0; i < depth; i++ {
		n = nodeOfData(&Data{Fields: noderep.Fields{{Name: "next", Value: []any{n}}}})
	}
	count := 0
	if err := core.Walk(n, func(v core.WalkVisit) (core.WalkAction, error) {
		if v.Node != nil {
			count++
		}
		return core.WalkContinue, nil
	}); err != nil || count != depth+1 {
		t.Fatal(count, err)
	}
}

func TestCursorBoundariesAndUncontainedChildren(t *testing.T) {
	for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
		a, b, z := nodeOfData(&Data{Start: 1, End: 3}), nodeOfData(&Data{Start: 3, End: 4}), nodeOfData(&Data{Start: 3, End: 3})
		root := nodeOfData(&Data{Start: 20, End: 21, Children: []*core.Node{a, b, z, a}})
		c := core.Cursor{Root: root, Unit: unit}
		var got []*core.Node
		callback := func(v core.WalkVisit) (core.WalkAction, error) {
			got = append(got, v.Node)
			return core.WalkContinue, nil
		}
		if err := c.At(3, callback); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []*core.Node{b, z}) {
			t.Fatalf("point = %v", got)
		}
		got = nil
		if err := c.Range(1, 3, callback); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []*core.Node{a, a}) {
			t.Fatal("range must preserve shared edges and exclude upper bound")
		}
		got = nil
		if err := c.Range(3, 3, callback); err != nil || len(got) != 0 {
			t.Fatal(got, err)
		}
		if err := c.Range(4, 3, callback); err == nil {
			t.Fatal("reversed range accepted")
		}
		if err := c.At(-1, callback); err == nil {
			t.Fatal("negative position accepted")
		}
	}
	if err := (core.Cursor{Unit: core.PositionUnit(99)}).At(0, func(core.WalkVisit) (core.WalkAction, error) { return core.WalkContinue, nil }); err == nil {
		t.Fatal("invalid unit accepted")
	}
}

func TestWalkSliceIdentityIgnoresCapacity(t *testing.T) {
	x := make([]any, 1, 2)
	x[0] = x[:1:1]
	root := nodeOfData(&Data{Fields: noderep.Fields{{Name: "x", Value: x}}})
	visits := 0
	err := core.Walk(root, func(core.WalkVisit) (core.WalkAction, error) {
		visits++
		return core.WalkContinue, nil
	})
	var cycle *core.CycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("capacity-limited slice cycle = %v, want CycleError", err)
	}
	want := []core.WalkEdge{{Kind: core.WalkRoot}, {Kind: core.WalkField, FieldName: "x"}, {Kind: core.WalkListElement, ListIndex: 0}}
	if visits != 2 || !reflect.DeepEqual(cycle.Path, want) {
		t.Fatalf("cycle detected after %d visits at %v, want 2 visits at %v", visits, cycle.Path, want)
	}
}

// A capture that labels an element of a node with source information is visited as that element
// only; a capture that is not an element is visited as a field.
func TestWalkVisitsCapturedElementsOnce(t *testing.T) {
	elem := nodeOfData(&Data{Text: "e"})
	beside := nodeOfData(&Data{Text: "b"})
	root := nodeOfData(&Data{Labeled: true, Children: []*core.Node{elem}, Fields: noderep.Fields{{Name: "x", Value: elem}, {Name: "s", Value: beside}}})
	visits := map[*core.Node]int{}
	err := core.Walk(root, func(v core.WalkVisit) (core.WalkAction, error) {
		visits[v.Node]++
		return core.WalkContinue, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if visits[elem] != 1 || visits[beside] != 1 || visits[root] != 1 {
		t.Fatalf("visits = elem %d, beside %d, root %d, want 1 each", visits[elem], visits[beside], visits[root])
	}
}
