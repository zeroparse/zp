package compile

import (
	"maps"
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/cancel"
)

// The run-time kinds of a program's types are the checker's (check.Checker.Kinds): the program
// shares its tables, and a computation that a cancellation stops leaves nothing behind, so that the
// next call computes them whole.
func TestKindsAreTheCheckers(t *testing.T) {
	p := mustCompile(t, "type N = node[string]\ntype S = { A N }\ntype M ~ N\nrule main N = x:\"a\" => N{$x}")
	k := p.Checker().Kinds(nil)
	if len(k.Nodes) == 0 || len(k.Types) == 0 {
		t.Fatalf("kinds: types %v, nodes %v", slices.Sorted(maps.Keys(k.Types)), slices.Sorted(maps.Keys(k.Nodes)))
	}
	for name, n := range k.Nodes {
		if p.NodeKinds[name] != n {
			t.Errorf("node type %s: the program's kinds are not the checker's", name)
		}
	}
	for name, kind := range k.Types {
		if p.TypeKind(name) != kind {
			t.Errorf("type %s: the program's kind is not the checker's", name)
		}
	}

	// A fresh checker of the same model: the first call is cancelled at its first type.
	q := &Program{Model: p.Model, ByName: p.ByName, Typed: p.Typed}
	q.Checker().Typed = p.Typed
	func() {
		defer func() {
			if _, ok := recover().(cancel.Cancellation); !ok {
				t.Fatal("the cancelled call did not unwind with a cancellation")
			}
		}()
		q.Checker().Kinds(func() { panic(cancel.Cancellation{}) })
	}()
	again := q.Checker().Kinds(nil)
	if !slices.Equal(slices.Sorted(maps.Keys(again.Types)), slices.Sorted(maps.Keys(k.Types))) ||
		!slices.Equal(slices.Sorted(maps.Keys(again.Nodes)), slices.Sorted(maps.Keys(k.Nodes))) {
		t.Errorf("after a cancelled call: types %v, nodes %v; want %v, %v",
			slices.Sorted(maps.Keys(again.Types)), slices.Sorted(maps.Keys(again.Nodes)),
			slices.Sorted(maps.Keys(k.Types)), slices.Sorted(maps.Keys(k.Nodes)))
	}
}
