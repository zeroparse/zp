package runtime

import "testing"

func TestActionTrackingReleasesReferences(t *testing.T) {
	// Any value but a function is a result: a value other than a node is boxed. An action fails by
	// returning its failure ("failure"); a function as its value ends the parse too ("invalid").
	for _, name := range []string{"nil", "node", "value", "invalid", "failure"} {
		t.Run(name, func(t *testing.T) {
			prefix := &Node{RawText: "keep"}
			made := &Node{RawFresh: true}
			p := &Parser{created: []*Node{prefix}}
			c := &ActCtx{P: p, cbase: 1, start: 3, end: 4}
			var got *Node
			var panicValue any
			func() {
				defer func() { panicValue = recover() }()
				got = c.result(func(*ActCtx) (any, any) {
					p.created = append(p.created, made)
					switch name {
					case "node":
						return made, nil
					case "value":
						return 1, nil
					case "invalid":
						return func(...any) any { return nil }, nil
					case "failure":
						return nil, evalError{"failed"}
					}
					return nil, nil
				}, "test")
			}()
			if (panicValue != nil) != (name == "invalid" || name == "failure") {
				t.Fatalf("unexpected panic: %v", panicValue)
			}
			if f, ok := panicValue.(fatal); name == "failure" && (!ok || f.err.Error() != "action in test: failed") {
				t.Fatalf("failure ends the parse with %v", panicValue)
			}
			if f, ok := panicValue.(fatal); name == "invalid" && (!ok || f.err.Error() != "action in test: result must be a value, got a function") {
				t.Fatalf("a function as the result ends the parse with %v", panicValue)
			}
			if len(p.created) != 1 || p.created[0] != prefix || p.created[:cap(p.created)][1] != nil {
				t.Fatal("tracking retains a discarded reference or changed its prefix")
			}
			if name == "node" && (got != made || got.RawStart != 3 || got.RawEnd != 4 || got.RawFresh) {
				t.Fatal("returned node changed identity or has wrong span/freshness")
			}
		})
	}
}

func TestTypedActionTrackingReleasesReferences(t *testing.T) {
	for _, name := range []string{"nil", "node", "value", "invalid"} {
		t.Run(name, func(t *testing.T) {
			prefix := &TSeq{Typ: "keep"}
			made := &TSeq{fresh: true}
			p := &TParser{Parser: &Parser{}, Where: "test", Created: []TVal{prefix, made}}
			c := &TActCtx{P: p, Cbase: 1, Start: 3, End: 4}
			var got any
			var panicValue any
			func() {
				defer func() { panicValue = recover() }()
				var value any
				if name == "node" {
					value = made
				} else if name == "value" {
					value = 1
				} else if name == "invalid" {
					value = func(...any) any { return nil }
				}
				got = c.Finish(value)
			}()
			if (panicValue != nil) != (name == "invalid") {
				t.Fatalf("unexpected panic: %v", panicValue)
			}
			if f, ok := panicValue.(fatal); name == "invalid" && (!ok || f.err.Error() != "action in test: result must be a value, got a function") {
				t.Fatalf("a function as the result ends the parse with %v", panicValue)
			}
			if len(p.Created) != 1 || p.Created[0] != prefix || p.Created[:cap(p.Created)][1] != nil {
				t.Fatal("tracking retains a discarded reference or changed its prefix")
			}
			if name == "node" && (got != made || made.Start != 3 || made.End != 4 || made.fresh) {
				t.Fatal("returned node changed identity or has wrong span/freshness")
			}
		})
	}
}
