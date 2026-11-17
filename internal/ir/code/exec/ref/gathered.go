package ref

import (
	"slices"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/exec"
)

// What terms make (exec.Gathered and exec.Made): the stack of the elements gathered for the lists and
// tuples being made, and the list of the structs made. Each is a sequence of identifiers with marks: its
// length, which a drop cuts it back to.

// Elem is the model's handle of an element that a case pushes: a conformance identifier. The null
// element is the null handle, which the model lists as 0.
type Elem struct{ ID int64 }

// Items is the model's handle of the elements of a list or a tuple (exec.Seq) that a case pushes at once:
// the identifiers of its elements.
type Items struct{ IDs []int64 }

// Struct is the model's handle of a struct that a case records as made: a conformance identifier.
type Struct struct{ ID int64 }

// Act is the model's handle of the context of a term (exec.Act). Pushing and recording do not read it,
// so a case has one.
type Act struct{}

type termState struct {
	gathered, made []int64
	// recorded holds every struct that act.made recorded, those dropped since included: a struct is
	// recorded at most once in a parse.
	recorded map[int64]bool
}

// Gathered returns the identifiers of the elements gathered, from the bottom of the stack, 0 for a null
// element.
func (m *Model) Gathered() []int64 { return slices.Clone(m.terms.gathered) }

// Made returns the identifiers of the structs made, in the order they were recorded.
func (m *Model) Made() []int64 { return slices.Clone(m.terms.made) }

func isTermStateOp(name string) bool {
	switch name {
	case "act.push", "act.pushAll", "term.kids", "term.dropKids", "term.elemsAbove", "term.keep",
		"act.made", "term.created", "term.dropCreated":
		return true
	}
	return false
}

// allowsTermState checks the preconditions of the operations of the elements gathered and the values
// made, and reports false for any other operation.
func (m *Model) allowsTermState(o *code.Op, args []any) (allowed, ok bool) {
	s := &m.terms
	switch o.Name {
	case "act.pushAll":
		_, isItems := args[1].(*Items)
		return isItems, true
	case "term.dropKids", "term.elemsAbove":
		return args[0].(int64) >= 0 && args[0].(int64) <= int64(len(s.gathered)), true
	case "term.keep":
		items, isItems := args[0].(*Items)
		return isItems && args[1].(int64) >= 0 && args[1].(int64) < int64(len(items.IDs)), true
	case "act.made":
		n, isStruct := args[1].(Struct)
		return isStruct && !s.recorded[n.ID], true
	case "term.dropCreated":
		return args[0].(int64) >= 0 && args[0].(int64) <= int64(len(s.made)), true
	case "act.push", "term.kids", "term.created":
		return true, true
	}
	return false, false
}

// termStateQuantity returns exec.QDropped and exec.QSeqLen for a call of an operation of the elements
// gathered or the values made.
func (m *Model) termStateQuantity(name string, o *code.Op, args []any) (int64, bool) {
	if o == nil || !isTermStateOp(o.Name) {
		return 0, false
	}
	switch {
	case name == exec.QDropped && (o.Name == "term.dropKids" || o.Name == "term.elemsAbove"):
		return int64(len(m.terms.gathered)) - args[0].(int64), true
	case name == exec.QDropped && o.Name == "term.dropCreated":
		return int64(len(m.terms.made)) - args[0].(int64), true
	case name == exec.QSeqLen && o.Name == "act.pushAll":
		return int64(len(args[1].(*Items).IDs)), true
	}
	return 0, false
}

// callTermState runs an operation of the elements gathered or the values made, and reports false for
// any other.
func (m *Model) callTermState(o *code.Op, args []any) ([]any, bool) {
	s := &m.terms
	switch o.Name {
	case "act.push":
		id := int64(0)
		if e, ok := args[1].(Elem); ok {
			id = e.ID
		}
		s.gathered = append(s.gathered, id)
	case "act.pushAll":
		s.gathered = append(s.gathered, args[1].(*Items).IDs...)
	case "term.kids":
		return []any{int64(len(s.gathered))}, true
	case "term.dropKids":
		s.gathered = s.gathered[:args[0].(int64)]
	case "term.elemsAbove":
		k := args[0].(int64)
		items := &Items{IDs: slices.Clone(s.gathered[k:])}
		s.gathered = s.gathered[:k]
		return []any{items}, true
	case "term.keep":
		s.gathered = append(s.gathered, args[0].(*Items).IDs[args[1].(int64)])
	case "act.made":
		id := args[1].(Struct).ID
		if s.recorded == nil {
			s.recorded = map[int64]bool{}
		}
		s.recorded[id] = true
		s.made = append(s.made, id)
	case "term.created":
		return []any{int64(len(s.made))}, true
	case "term.dropCreated":
		s.made = s.made[:args[0].(int64)]
	default:
		return nil, false
	}
	return nil, true
}
