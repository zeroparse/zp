// Package noderep is the representation of parse.Node that the parsers share: the engine of
// zeroparse.dev/zp and generated Go parsers build and read nodes through it. It is not a stable
// API. Generated code depends on it through the runtime protocol version, and programs should use
// the methods of parse.Node instead.
package noderep

import (
	"sort"
	"sync"
)

// Names of the reserved node types.
const (
	TypeString  = "string"  // a node whose value is its text (literal, character class, atomic, and so on)
	TypeTuple   = "tuple"   // a sequence, or an operator of an operators rule without an action
	TypeList    = "list"    // a repetition
	TypeMissing = "missing" // literal inserted at an empty source range
	TypeError   = "error"   // range skipped by error recovery

	// Kinds of the value model that are not nodes. Their names cannot be type names.
	typeBox        = "(box)"   // a value other than a node, carried where a node pointer is expected
	TypeTupleValue = "(tuple)" // a tuple value: the Value of a tuple node
	TypeListValue  = "(list)"  // a list value: the Value of a list node, or a list made by an action
)

// Data is the representation of a node. N is parse.Node, which holds a Data as its only field, so
// that a *parse.Node converts to a *Data[parse.Node] and back; the parameter breaks the import
// cycle between the two packages.
type Data[N any] struct {
	// Kind holds the type and rule names (Type, Rule). Nodes share it: names are interned.
	Kind *Kind
	// Start and End are the range in the input, in the parse's position unit (End is exclusive).
	Start, End int32
	// Text is the string value of a string node.
	Text string
	// Children are the elements of a tuple or list value. An optional that did not match is nil.
	Children []*N
	// Fields are the struct fields and captures. Each value is a *N, int, string, bool, or nil.
	Fields Fields

	// Fresh reports that the node was created in the rule body currently being evaluated and has not
	// yet been returned to any rule. That rule may freely modify a fresh node.
	Fresh bool
	// Terminal reports that the node is a terminal.
	Terminal bool
	// Labeled reports that the captures in Fields are output (source information was requested).
	Labeled bool
	// Gen is the number of Document edits that the positions account for.
	Gen uint32
	// Box is the value of an action that is not a node (a box node).
	Box any
}

// Kind is the pair of type and rule names of nodes. Kinds are interned (KindOf), so that a node
// holds one pointer for both names.
type Kind struct {
	Type, Rule string
	Reserved   int // the index of a reserved type in reservedTypes with no rule, or -1
	// Base is the index in reservedTypes of what the nodes are: the reserved type itself, with or
	// without a rule, or for a named node type the string, tuple or list node it converts. It is
	// -1 for a struct.
	Base int
}

// TypeName returns the type name of k, or "" for a nil kind.
func (k *Kind) TypeName() string {
	if k == nil {
		return ""
	}
	return k.Type
}

// IsNodeType reports whether k is the kind of a named node type.
func (k *Kind) IsNodeType() bool {
	return k != nil && k.Base >= 0 && k.Type != reservedTypes[k.Base]
}

// RuleName returns the rule name of k, or "" for a nil kind.
func (k *Kind) RuleName() string {
	if k == nil {
		return ""
	}
	return k.Rule
}

var reservedTypes = [...]string{TypeString, TypeTuple, TypeList, TypeError, TypeMissing, typeBox, TypeTupleValue, TypeListValue}

// The indexes in reservedTypes of the bases of node kinds.
const (
	BaseString = iota
	BaseTuple
	BaseList
	BaseError
	BaseMissing
)

// Kinds of the reserved types without a rule.
var (
	MatchKind   = KindOf(TypeString, "")
	SeqKind     = KindOf(TypeTuple, "")
	ListKind    = KindOf(TypeList, "")
	ErrorKind   = KindOf(TypeError, "")
	MissingKind = KindOf(TypeMissing, "")

	BoxKind        = KindOf(typeBox, "")
	TupleValueKind = KindOf(TypeTupleValue, "")
	ListValueKind  = KindOf(TypeListValue, "")
)

var kinds sync.Map // [2]string → *Kind

// KindOf returns the interned kind of the type and rule names. It is meant for compilation and
// loading; parsing uses kinds prepared then.
func KindOf(typ, rule string) *Kind {
	key := [2]string{typ, rule}
	if k, ok := kinds.Load(key); ok {
		return k.(*Kind)
	}
	k := &Kind{Type: typ, Rule: rule, Reserved: -1, Base: -1}
	for i, t := range reservedTypes {
		if t == typ {
			k.Base = i
			if rule == "" {
				k.Reserved = i
			}
		}
	}
	actual, _ := kinds.LoadOrStore(key, k)
	return actual.(*Kind)
}

// NodeTypeKind returns the interned kind of the nodes of the named node type typ that convert a
// node of the base (BaseString, BaseTuple or BaseList). It has no rule.
func NodeTypeKind(typ string, base int) *Kind {
	// The key cannot be that of a rule, whose name has no NUL.
	key := [2]string{typ, "\x00" + reservedTypes[base]}
	k, ok := kinds.Load(key)
	if !ok {
		k, _ = kinds.LoadOrStore(key, &Kind{Type: typ, Reserved: -1, Base: base})
	}
	return k.(*Kind)
}

// RuleKinds are the kinds of a rule's nodes: those of the reserved types, which the rule gives its
// name to (finish).
type RuleKinds struct {
	reserved [len(reservedTypes)]*Kind
}

// NewRuleKinds returns the kinds of the nodes of the rule.
func NewRuleKinds(rule string) RuleKinds {
	var rk RuleKinds
	for i, t := range reservedTypes {
		rk.reserved[i] = KindOf(t, rule)
	}
	return rk
}

// Named returns the kind k with the rule's name. A node of a named node type keeps no rule.
func (rk *RuleKinds) Named(k *Kind, rule string) *Kind {
	if k.Reserved >= 0 {
		return rk.reserved[k.Reserved]
	}
	if k.Base >= 0 {
		return k
	}
	return KindOf(k.Type, rule)
}

// NodeField is a field of a node (a struct field or a capture).
type NodeField struct {
	Name  string
	Value any
}

// Fields is the list of a node's fields. Each name appears at most once.
type Fields []NodeField

// Get returns the value of the field name.
func (fs Fields) Get(name string) (any, bool) {
	for i := range fs {
		if fs[i].Name == name {
			return fs[i].Value, true
		}
	}
	return nil, false
}

// SetField sets the field name of fs to v (adding it if absent).
func SetField(fs *Fields, name string, v any) {
	for i := range *fs {
		if (*fs)[i].Name == name {
			(*fs)[i].Value = v
			return
		}
	}
	*fs = append(*fs, NodeField{name, v})
}

// Sorted returns a copy sorted lexicographically by name.
func (fs Fields) Sorted() Fields {
	c := append(Fields(nil), fs...)
	sort.Slice(c, func(i, j int) bool { return c[i].Name < c[j].Name })
	return c
}

// Shape is what a node stands for in the value model.
type Shape uint8

const (
	ShapeString     Shape = iota // a node whose value is a string (Text)
	ShapeTuple                   // a node whose value is a tuple (Children)
	ShapeList                    // a node whose value is a list (Children)
	ShapeError                   // an error value of recovery (Error or Missing)
	ShapeBox                     // a box carrying a value other than a node
	ShapeTupleValue              // a tuple value (Children)
	ShapeListValue               // a list value (Children)
	ShapeStruct                  // a struct value (Fields)
)

// baseShapes are the shapes of the nodes of the kinds of each base (Kind.Base).
var baseShapes = [len(reservedTypes)]Shape{ShapeString, ShapeTuple, ShapeList, ShapeError, ShapeError, ShapeBox, ShapeTupleValue, ShapeListValue}

// ShapeOfKind returns what a node of the kind stands for, given whether it is a terminal.
func ShapeOfKind(k *Kind, terminal bool) Shape {
	if k != nil && k.Base >= 0 {
		return baseShapes[k.Base]
	}
	if terminal {
		return ShapeString
	}
	return ShapeStruct
}

// CaptureLocation is where a capture of a node is: a path of tuple indexes into the node's
// value, or, for a capture whose value is not an element (one in a positive lookahead), its value.
type CaptureLocation struct {
	Name  string
	Path  []int
	Side  bool
	Value any
}

// CaptureLocations returns the locations of the captures of the tuple node d, in name order. A
// capture is located by the identity of its value among the elements of d's tuple and of the
// tuples nested in it, nearest first; a capture that did not match is left out. rep returns the
// representation of a node.
func CaptureLocations[N any](d *Data[N], rep func(*N) *Data[N]) []CaptureLocation {
	var out []CaptureLocation
	for _, f := range d.Fields.Sorted() {
		v, ok := f.Value.(*N)
		if !ok || v == nil {
			if f.Value != nil {
				out = append(out, CaptureLocation{Name: f.Name, Side: true, Value: f.Value})
			}
			continue
		}
		if path, found := ElementPath(d, v, rep); found {
			out = append(out, CaptureLocation{Name: f.Name, Path: path})
		} else {
			out = append(out, CaptureLocation{Name: f.Name, Side: true, Value: v})
		}
	}
	return out
}

// ElementPath returns the path of x among the elements of the tuple node d and of the tuple nodes
// nested in it, breadth first. rep returns the representation of a node.
func ElementPath[N any](d *Data[N], x *N, rep func(*N) *Data[N]) ([]int, bool) {
	type item struct {
		d    *Data[N]
		path []int
	}
	queue := []item{{d, nil}}
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		for i, c := range it.d.Children {
			if c == nil {
				continue
			}
			path := append(append([]int{}, it.path...), i)
			if c == x {
				return path, true
			}
			if cd := rep(c); ShapeOfKind(cd.Kind, cd.Terminal) == ShapeTuple && cd.Kind.RuleName() == "" {
				queue = append(queue, item{cd, path})
			}
		}
	}
	return nil, false
}
