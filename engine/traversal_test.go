package engine_test

import (
	"bytes"
	"errors"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/value"
)

func TestTraversalPublicFacade(t *testing.T) {
	parser, err := compileTestSource(`rule main = "é"`, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
		node, err := parser.Parse("é", engine.WithUnit(unit))
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		callback := func(v core.WalkVisit) (core.WalkAction, error) {
			count++
			if v.Depth != 0 || v.Edge.Kind != core.WalkRoot || v.Node != node || v.Parent != nil {
				t.Fatal(v)
			}
			return core.WalkContinue, nil
		}
		if err := core.Walk(node, callback); err != nil || count != 1 {
			t.Fatal(count, err)
		}
		count = 0
		if err := (core.Cursor{Root: node, Unit: unit}).At(0, callback); err != nil || count != 1 {
			t.Fatal(count, err)
		}
		var output bytes.Buffer
		if err := node.WriteJSON(&output); err != nil || output.Len() == 0 {
			t.Fatal(err)
		}
		value.SetNodeElems(node, []*core.Node{node})
		var cycle *core.CycleError
		if err := core.Walk(node, func(core.WalkVisit) (core.WalkAction, error) { return core.WalkContinue, nil }); !errors.As(err, &cycle) {
			t.Fatalf("cycle = %v", err)
		}
	}
}
