package value

import (
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/noderep"
)

var benchmarkNodeJSONBytes []byte

func BenchmarkNodeJSON(b *testing.B) {
	for _, workload := range []struct {
		name string
		root *core.Node
	}{
		{name: "shallow", root: benchmarkShallowJSONNode()},
		{name: "deep", root: benchmarkDeepJSONNode(512)},
	} {
		b.Run(workload.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				data, err := workload.root.MarshalJSON()
				if err != nil {
					b.Fatal(err)
				}
				benchmarkNodeJSONBytes = data
			}
		})
	}
}

func benchmarkShallowJSONNode() *core.Node {
	children := make([]*core.Node, 8)
	for i := range children {
		children[i] = Ptr(Data{
			Kind:  noderep.KindOf("Token", "item"),
			Start: int32(i * 3),
			End:   int32(i*3 + 2),
			Text:  "item<&>",
		})
	}
	return Ptr(Data{
		Kind:     noderep.KindOf("Record", "entry"),
		Start:    0,
		End:      24,
		Children: children,
		Fields: noderep.Fields{
			{Name: "name", Value: "example"},
			{Name: "count", Value: int64(len(children))},
			{Name: "items", Value: []any{"one", int32(2), true}},
		},
	})
}

func benchmarkDeepJSONNode(depth int) *core.Node {
	root := Ptr(Data{Kind: noderep.KindOf("Tree", "branch")})
	node := root
	for i := 1; i < depth; i++ {
		child := Ptr(Data{Kind: noderep.KindOf("Tree", "branch"), Start: int32(i), End: int32(i + 1)})
		D(node).Children = []*core.Node{child}
		node = child
	}
	D(node).Kind = noderep.KindOf("Token", "leaf")
	D(node).Text = "end"
	return root
}
