package parse

import (
	"bytes"
	"fmt"
	"iter"
	"strconv"
	"strings"

	"zeroparse.dev/zp/runtime/noderep"
)

// Node is a value produced by parsing. It is a CST node, a terminal, a list, or a struct built by
// an action. In JSON it is an object with the members type, rule (if any), start, end, text (if
// any), children (if any) and fields (if any).
type Node struct {
	d noderep.Data[Node]
}

// Type returns the node's type name: the kind of a node of the value model (string, tuple, list,
// error, or Missing for an inserted literal), or otherwise the name of a type defined in the grammar.
func (n *Node) Type() string { return n.d.Kind.TypeName() }

// Rule returns the name of the rule that produced the node, if it was produced by a rule without
// an action, or "".
func (n *Node) Rule() string { return n.d.Kind.RuleName() }

// MarshalJSON encodes the node as an object (see Node).
func (n *Node) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	if err := n.WriteJSON(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Clone returns a deep copy of the tree n. Subtrees shared within the tree stay shared in the
// copy. A Document moves the nodes it reuses after an edit, which changes trees returned by its
// earlier parses; a clone keeps a tree as it was.
func (n *Node) Clone() *Node {
	return cloneNode(n, map[*Node]*Node{})
}

func cloneNode(n *Node, seen map[*Node]*Node) *Node {
	if n == nil {
		return nil
	}
	if c, ok := seen[n]; ok {
		return c
	}
	c := new(Node)
	*c = *n
	seen[n] = c
	if n.d.Children != nil {
		c.d.Children = make([]*Node, len(n.d.Children))
		for i, ch := range n.d.Children {
			c.d.Children[i] = cloneNode(ch, seen)
		}
	}
	if n.d.Fields != nil {
		c.d.Fields = make(noderep.Fields, len(n.d.Fields))
		for i, f := range n.d.Fields {
			if vn, ok := f.Value.(*Node); ok {
				f.Value = cloneNode(vn, seen)
			}
			c.d.Fields[i] = f
		}
	}
	return c
}

// IsTerminal reports whether the node holds text: a string node or an error value.
func (n *Node) IsTerminal() bool { return n != nil && n.d.Terminal }

// NodeKind is what a Node stands for in the value model.
type NodeKind uint8

const (
	// KindString is a node whose value is a string, the text it matched: Text.
	KindString NodeKind = iota
	// KindTuple is a node whose value is a tuple: Len and Elem.
	KindTuple
	// KindList is a node whose value is a list: Len and Elem.
	KindList
	// KindError is an error value of recovery: its range is the skipped input and its Text() is its
	// message. It stores no text of the range.
	KindError
	// KindStruct is a struct built by an action: Type and Field.
	KindStruct
	// KindTupleValue is a tuple built by an action, without a range: Len and Elem.
	KindTupleValue
	// KindListValue is a list built by an action, without a range: Len and Elem.
	KindListValue
	// KindValue is any other value built by an action, such as an int or a bool: Value.
	KindValue
)

var kindNames = [...]string{"string", "tuple", "list", "error", "struct", "tuple value", "list value", "value"}

func (k NodeKind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("NodeKind(%d)", k)
}

// Kind returns what the node stands for.
func (n *Node) Kind() NodeKind { return kindOfShape(shapeOf(n)) }

// kindOfShape returns the NodeKind of a node that has the shape.
func kindOfShape(shape noderep.Shape) NodeKind {
	switch shape {
	case noderep.ShapeString:
		return KindString
	case noderep.ShapeTuple:
		return KindTuple
	case noderep.ShapeList:
		return KindList
	case noderep.ShapeError:
		return KindError
	case noderep.ShapeBox:
		return KindValue
	case noderep.ShapeTupleValue:
		return KindTupleValue
	case noderep.ShapeListValue:
		return KindListValue
	}
	return KindStruct
}

// Start returns the start of the range of the input that the node spans, in the position unit of
// the parse.
func (n *Node) Start() int { return int(n.d.Start) }

// End returns the end of the range of the input that the node spans (exclusive), in the position
// unit of the parse.
func (n *Node) End() int { return int(n.d.End) }

// Text returns the string value of a KindString node, and the message of a KindError value; it
// returns "" for other nodes and nil.
func (n *Node) Text() string {
	if n == nil {
		return ""
	}
	switch shapeOf(n) {
	case noderep.ShapeString:
		return n.d.Text
	case noderep.ShapeError:
		msg, _ := n.d.Fields.Get("message")
		s, _ := msg.(string)
		return s
	}
	return ""
}

// Len returns the number of elements of a tuple or a list (KindTuple, KindList, KindTupleValue,
// KindListValue), and 0 for other nodes and nil.
func (n *Node) Len() int {
	if n == nil {
		return 0
	}
	switch shapeOf(n) {
	case noderep.ShapeTuple, noderep.ShapeList, noderep.ShapeTupleValue, noderep.ShapeListValue:
		return len(n.d.Children)
	}
	return 0
}

// Elem returns the element i of a tuple or a list: a node, or nil for an optional that did not
// match. It panics if i is out of the range [0, Len()).
func (n *Node) Elem(i int) *Node {
	if i < 0 || i >= n.Len() {
		panic(fmt.Sprintf("zp: element %d of a node with %d elements", i, n.Len()))
	}
	return n.d.Children[i]
}

// Elems returns the elements of a tuple or a list in order, as Elem does.
func (n *Node) Elems() iter.Seq2[int, *Node] {
	return func(yield func(int, *Node) bool) {
		for i := range n.Len() {
			if !yield(i, n.d.Children[i]) {
				return
			}
		}
	}
}

// Value returns the value of a KindValue node (such as an int, a string or a bool) and the string
// of a KindString node; it returns nil for other nodes and nil.
func (n *Node) Value() any {
	if n == nil {
		return nil
	}
	switch shapeOf(n) {
	case noderep.ShapeBox:
		return n.d.Box
	case noderep.ShapeString:
		return n.d.Text
	}
	return nil
}

// Field returns the value of the field name of a struct, and, with source information, the value
// of the capture name of a node: a *Node, an int, a string, a bool, or nil.
func (n *Node) Field(name string) any {
	if !n.fieldsVisible() {
		return nil
	}
	v, _ := n.d.Fields.Get(name)
	return v
}

// fieldsVisible reports whether the fields of n are public: those of a struct, and the captures
// of a node with source information.
func (n *Node) fieldsVisible() bool {
	return n != nil && (n.d.Labeled || shapeOf(n) == noderep.ShapeStruct)
}

// Fields returns the fields of a struct, and, with source information, the captures of a node,
// in the order the action wrote them.
func (n *Node) Fields() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		if !n.fieldsVisible() {
			return
		}
		for _, f := range n.d.Fields {
			if !yield(f.Name, f.Value) {
				return
			}
		}
	}
}

// ShapeOf returns what n stands for in the value model.
func shapeOf(n *Node) noderep.Shape { return noderep.ShapeOfKind(n.d.Kind, n.d.Terminal) }

// captureLocations returns the locations of the captures of the tuple node n, in name order.
func captureLocations(n *Node) []noderep.CaptureLocation {
	return noderep.CaptureLocations(&n.d, rep)
}

// rep returns the representation of n.
func rep(n *Node) *noderep.Data[Node] { return &n.d }

// String renders the value as an S-expression of its shape, without positions, for tests and
// debugging: a string is quoted, a tuple is ( ), a list [ ], nil is nil, a struct is
// Name{Field: value}, and an error is error("message"). A node of a named node type N is written
// with N before its brackets: N("text"), N( ) and N[ ]. With source information, a node is
// followed by @rule, a captured element is preceded by name:, and a capture whose value is not an
// element is written after the elements as &name:value.
func (n *Node) String() string {
	var b strings.Builder
	writeValue(&b, n)
	return b.String()
}

// sexprFrame is a step of writeValue, which keeps its own stack for deep trees.
type sexprFrame struct {
	v     any
	i     int
	enter bool
	// labels maps the elements of the tuples being written to the names that capture them.
	labels map[*Node][]string
	side   []noderep.CaptureLocation
	close  string
}

func writeValue(b *strings.Builder, v any) {
	stack := []sexprFrame{{v: v, enter: true}}
	for len(stack) > 0 {
		f := &stack[len(stack)-1]
		n, isNode := f.v.(*Node)
		if !isNode || n == nil {
			writeScalar(b, f.v)
			stack = stack[:len(stack)-1]
			continue
		}
		sh := shapeOf(n)
		if f.enter {
			f.enter = false
			switch sh {
			case noderep.ShapeBox:
				f.v, f.enter = n.d.Box, true
				continue
			case noderep.ShapeString:
				if n.d.Kind.IsNodeType() {
					b.WriteString(n.Type() + "(" + strconv.Quote(n.d.Text))
					f.close = ")" + ruleSuffix(n)
				} else {
					b.WriteString(strconv.Quote(n.d.Text))
					f.close = ruleSuffix(n)
				}
				f.i = -1
			case noderep.ShapeError:
				msg, _ := n.d.Fields.Get("message")
				b.WriteString("error(" + strconv.Quote(fmt.Sprint(msg)))
				if expected, ok := n.d.Fields.Get("expected"); ok {
					b.WriteString(" expected " + strconv.Quote(fmt.Sprint(expected)))
				}
				b.WriteString(")")
				f.i = -1
			case noderep.ShapeTuple, noderep.ShapeTupleValue:
				b.WriteString(nodeTypePrefix(n) + "(")
				f.close = ")" + ruleSuffix(n)
				if sh == noderep.ShapeTuple && n.d.Labeled {
					f.labels = map[*Node][]string{}
					for _, c := range captureLocations(n) {
						if c.Side {
							f.side = append(f.side, c)
							continue
						}
						x := n
						for _, i := range c.Path {
							x = x.d.Children[i]
						}
						f.labels[x] = append(f.labels[x], c.Name)
					}
				}
			case noderep.ShapeList, noderep.ShapeListValue:
				b.WriteString(nodeTypePrefix(n) + "[")
				f.close = "]" + ruleSuffix(n)
			case noderep.ShapeStruct:
				b.WriteString(n.Type() + "{")
				f.close = "}"
			}
		}
		if f.i < 0 {
			b.WriteString(f.close)
			stack = stack[:len(stack)-1]
			continue
		}
		if sh == noderep.ShapeStruct {
			fields := n.d.Fields.Sorted()
			if f.i == len(fields) {
				b.WriteString(f.close)
				stack = stack[:len(stack)-1]
				continue
			}
			if f.i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fields[f.i].Name + ": ")
			f.i++
			stack = append(stack, sexprFrame{v: fields[f.i-1].Value, enter: true})
			continue
		}
		if f.i == len(n.d.Children) {
			for j, c := range f.side {
				if f.i > 0 || j > 0 {
					b.WriteString(" ")
				}
				b.WriteString("&" + c.Name + ":")
				var side strings.Builder
				writeValue(&side, c.Value)
				b.WriteString(side.String())
			}
			b.WriteString(f.close)
			stack = stack[:len(stack)-1]
			continue
		}
		if f.i > 0 {
			b.WriteString(" ")
		}
		c := n.d.Children[f.i]
		f.i++
		labels := f.labels
		for _, name := range labels[c] {
			b.WriteString(name + ":")
		}
		next := sexprFrame{v: c, enter: true}
		if c != nil && shapeOf(c) == noderep.ShapeTuple && c.Rule() == "" {
			// A tuple nested in the node's own tuple holds the node's captures too.
			next.labels = labels
		}
		stack = append(stack, next)
	}
}

// nodeTypePrefix returns the name that a node of a named node type puts before its opening
// bracket, or "".
func nodeTypePrefix(n *Node) string {
	if n.d.Kind.IsNodeType() {
		return n.Type()
	}
	return ""
}

func ruleSuffix(n *Node) string {
	if r := n.Rule(); r != "" {
		return "@" + r
	}
	return ""
}

func writeScalar(b *strings.Builder, v any) {
	switch v := v.(type) {
	case nil:
		b.WriteString("nil")
	case *Node:
		b.WriteString("nil")
	case string:
		b.WriteString(strconv.Quote(v))
	case int:
		b.WriteString(strconv.Itoa(v))
	case bool:
		b.WriteString(strconv.FormatBool(v))
	default:
		fmt.Fprintf(b, "%v", v)
	}
}
