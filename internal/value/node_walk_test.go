package value_test

import (
	"errors"
	"reflect"
	"testing"

	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

func TestWalkEdgeOrderAndSharedParents(t *testing.T) {
	t.Parallel()
	shared := value.Ptr(value.Data{Text: "shared"})
	left := value.Ptr(value.Data{Children: []*parse.Node{shared}})
	right := value.Ptr(value.Data{Fields: noderep.Fields{{Name: "nested", Value: []any{shared, nil, 7}}}})
	root := value.Ptr(value.Data{Children: []*parse.Node{left, right, nil}, Fields: noderep.Fields{{Name: "z", Value: false}, {Name: "a", Value: shared}}})
	var got []string
	var parents []*parse.Node
	err := parse.Walk(root, func(v parse.WalkVisit) (parse.WalkAction, error) {
		switch v.Edge.Kind {
		case parse.WalkRoot:
			got = append(got, "root")
		case parse.WalkChild:
			got = append(got, "child")
		case parse.WalkField:
			got = append(got, v.Edge.FieldName)
		case parse.WalkListElement:
			got = append(got, "list")
		}
		if v.Node == shared {
			parents = append(parents, v.Parent)
		}
		return parse.WalkContinue, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"root", "child", "child", "child", "nested", "list", "list", "list", "child", "a", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visits = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(parents, []*parse.Node{left, right, root}) {
		t.Fatal("shared edge parents lost")
	}
	if value.D(root).Fields[0].Name != "z" {
		t.Fatal("walk sorted fields in place")
	}
}

func TestWalkCyclesAndControls(t *testing.T) {
	t.Parallel()
	n := &parse.Node{}
	value.D(n).Children = []*parse.Node{n}
	var cycle *parse.CycleError
	err := parse.Walk(n, func(parse.WalkVisit) (parse.WalkAction, error) { return parse.WalkContinue, nil })
	if !errors.As(err, &cycle) || len(cycle.Path) != 2 || cycle.Path[1].Kind != parse.WalkChild {
		t.Fatalf("cycle = %#v, %v", cycle, err)
	}
	list := make([]any, 1)
	list[0] = list
	n = value.Ptr(value.Data{Fields: noderep.Fields{{Name: "x", Value: list}}})
	err = parse.Walk(n, func(parse.WalkVisit) (parse.WalkAction, error) { return parse.WalkContinue, nil })
	if !errors.As(err, &cycle) || len(cycle.Path) != 3 {
		t.Fatalf("list cycle = %v", err)
	}
	sentinel := errors.New("callback")
	if err := parse.Walk(n, func(parse.WalkVisit) (parse.WalkAction, error) { return parse.WalkContinue, sentinel }); err != sentinel {
		t.Fatal(err)
	}
	for _, action := range []parse.WalkAction{parse.WalkSkip, parse.WalkStop} {
		count := 0
		if err := parse.Walk(n, func(parse.WalkVisit) (parse.WalkAction, error) { count++; return action, nil }); err != nil || count != 1 {
			t.Fatalf("control %d = %d, %v", action, count, err)
		}
	}
	count := 0
	if err := parse.Walk(nil, func(v parse.WalkVisit) (parse.WalkAction, error) {
		count++
		if v.Value != nil || v.Node != nil || v.Depth != 0 {
			t.Fatal(v)
		}
		return parse.WalkContinue, nil
	}); err != nil || count != 1 {
		t.Fatal(err, count)
	}
}

func TestWalkDeep(t *testing.T) {
	t.Parallel()
	n := &parse.Node{}
	const depth = 20000
	for i := 0; i < depth; i++ {
		n = value.Ptr(value.Data{Fields: noderep.Fields{{Name: "next", Value: []any{n}}}})
	}
	count := 0
	if err := parse.Walk(n, func(v parse.WalkVisit) (parse.WalkAction, error) {
		if v.Node != nil {
			count++
		}
		return parse.WalkContinue, nil
	}); err != nil || count != depth+1 {
		t.Fatal(count, err)
	}
}

func TestCursorBoundariesAndUncontainedChildren(t *testing.T) {
	t.Parallel()
	for _, unit := range []parse.PositionUnit{parse.CodePoints, parse.Bytes} {
		a, b, z := value.Ptr(value.Data{Start: 1, End: 3}), value.Ptr(value.Data{Start: 3, End: 4}), value.Ptr(value.Data{Start: 3, End: 3})
		root := value.Ptr(value.Data{Start: 20, End: 21, Children: []*parse.Node{a, b, z, a}})
		c := parse.Cursor{Root: root, Unit: unit}
		var got []*parse.Node
		callback := func(v parse.WalkVisit) (parse.WalkAction, error) {
			got = append(got, v.Node)
			return parse.WalkContinue, nil
		}
		if err := c.At(3, callback); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []*parse.Node{b, z}) {
			t.Fatalf("point = %v", got)
		}
		got = nil
		if err := c.Range(1, 3, callback); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []*parse.Node{a, a}) {
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
	if err := (parse.Cursor{Unit: parse.PositionUnit(99)}).At(0, func(parse.WalkVisit) (parse.WalkAction, error) { return parse.WalkContinue, nil }); err == nil {
		t.Fatal("invalid unit accepted")
	}
}

func TestWalkSliceIdentityIgnoresCapacity(t *testing.T) {
	t.Parallel()
	x := make([]any, 1, 2)
	x[0] = x[:1:1]
	root := value.Ptr(value.Data{Fields: noderep.Fields{{Name: "x", Value: x}}})
	visits := 0
	err := parse.Walk(root, func(parse.WalkVisit) (parse.WalkAction, error) {
		visits++
		return parse.WalkContinue, nil
	})
	var cycle *parse.CycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("capacity-limited slice cycle = %v, want CycleError", err)
	}
	want := []parse.WalkEdge{{Kind: parse.WalkRoot}, {Kind: parse.WalkField, FieldName: "x"}, {Kind: parse.WalkListElement, ListIndex: 0}}
	if visits != 2 || !reflect.DeepEqual(cycle.Path, want) {
		t.Fatalf("cycle detected after %d visits at %v, want 2 visits at %v", visits, cycle.Path, want)
	}
}
