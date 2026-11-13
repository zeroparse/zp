package apitest_test

import (
	"bytes"
	"errors"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/parse"
)

func TestTraversalPublicFacade(t *testing.T) {
	parser, err := compileTestSource(`rule main = "é"`, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range []parse.PositionUnit{parse.CodePoints, parse.Bytes} {
		node, err := parser.Parse("é", zp.WithUnit(unit))
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		callback := func(v parse.WalkVisit) (parse.WalkAction, error) {
			count++
			if v.Depth != 0 || v.Edge.Kind != parse.WalkRoot || v.Node != node || v.Parent != nil {
				t.Fatal(v)
			}
			return parse.WalkContinue, nil
		}
		if err := parse.Walk(node, callback); err != nil || count != 1 {
			t.Fatal(count, err)
		}
		count = 0
		if err := (parse.Cursor{Root: node, Unit: unit}).At(0, callback); err != nil || count != 1 {
			t.Fatal(count, err)
		}
		var output bytes.Buffer
		if err := node.WriteJSON(&output); err != nil || output.Len() == 0 {
			t.Fatal(err)
		}
		value.SetNodeElems(node, []*parse.Node{node})
		var cycle *parse.CycleError
		if err := parse.Walk(node, func(parse.WalkVisit) (parse.WalkAction, error) { return parse.WalkContinue, nil }); !errors.As(err, &cycle) {
			t.Fatalf("cycle = %v", err)
		}
	}
}
