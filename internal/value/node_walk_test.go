package value_test

import (
	"errors"
	"reflect"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/internal/value"
)

func TestWalkEdgeOrderAndSharedParents(t *testing.T) {
	t.Parallel()
	shared := value.Ptr(value.Data{Text: "shared"})
	left := value.Ptr(value.Data{Children: []*core.Node{shared}})
	right := value.Ptr(value.Data{Fields: coreimpl.Fields{{Name: "nested", Value: []any{shared, nil, 7}}}})
	root := value.Ptr(value.Data{Children: []*core.Node{left, right, nil}, Fields: coreimpl.Fields{{Name: "z", Value: false}, {Name: "a", Value: shared}}})
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
	if value.D(root).Fields[0].Name != "z" {
		t.Fatal("walk sorted fields in place")
	}
}

func TestWalkCyclesAndControls(t *testing.T) {
	t.Parallel()
	n := &core.Node{}
	value.D(n).Children = []*core.Node{n}
	var cycle *core.CycleError
	err := core.Walk(n, func(core.WalkVisit) (core.WalkAction, error) { return core.WalkContinue, nil })
	if !errors.As(err, &cycle) || len(cycle.Path) != 2 || cycle.Path[1].Kind != core.WalkChild {
		t.Fatalf("cycle = %#v, %v", cycle, err)
	}
	list := make([]any, 1)
	list[0] = list
	n = value.Ptr(value.Data{Fields: coreimpl.Fields{{Name: "x", Value: list}}})
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
	t.Parallel()
	n := &core.Node{}
	const depth = 20000
	for i := 0; i < depth; i++ {
		n = value.Ptr(value.Data{Fields: coreimpl.Fields{{Name: "next", Value: []any{n}}}})
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
	t.Parallel()
	for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
		a, b, z := value.Ptr(value.Data{Start: 1, End: 3}), value.Ptr(value.Data{Start: 3, End: 4}), value.Ptr(value.Data{Start: 3, End: 3})
		root := value.Ptr(value.Data{Start: 20, End: 21, Children: []*core.Node{a, b, z, a}})
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
	t.Parallel()
	x := make([]any, 1, 2)
	x[0] = x[:1:1]
	root := value.Ptr(value.Data{Fields: coreimpl.Fields{{Name: "x", Value: x}}})
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
