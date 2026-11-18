package engine_test

import (
	"bytes"
	"errors"
	"testing"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/pkg/runtime/input"
	"zeroparse.dev/zp/pkg/runtime/value"
)

func TestTraversalPublicFacade(t *testing.T) {
	parser, err := compileTestSource(`rule main = "é"`, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range []input.PositionUnit{input.CodePoints, input.Bytes} {
		node, err := parser.Parse("é", engine.WithUnit(unit))
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		callback := func(v value.WalkVisit) (value.WalkAction, error) {
			count++
			if v.Depth != 0 || v.Edge.Kind != value.WalkRoot || v.Node != node || v.Parent != nil {
				t.Fatal(v)
			}
			return value.WalkContinue, nil
		}
		if err := value.Walk(node, callback); err != nil || count != 1 {
			t.Fatal(count, err)
		}
		count = 0
		if err := (value.Cursor{Root: node, Unit: unit}).At(0, callback); err != nil || count != 1 {
			t.Fatal(count, err)
		}
		var output bytes.Buffer
		if err := node.WriteJSON(&output); err != nil || output.Len() == 0 {
			t.Fatal(err)
		}
		nodes.SetNodeElems(node, []*value.Node{node})
		var cycle *value.CycleError
		if err := value.Walk(node, func(value.WalkVisit) (value.WalkAction, error) { return value.WalkContinue, nil }); !errors.As(err, &cycle) {
			t.Fatalf("cycle = %v", err)
		}
	}
}
