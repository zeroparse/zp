package ref

import (
	"fmt"
	"math/big"
	"strings"
	"unicode/utf8"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// Kind is the kind of a value of the model.
type Kind int

// Kinds of values. A node is a node of the input: a string, tuple or list node (Shape), named or not.
const (
	KNil Kind = iota
	KInt
	KStr
	KBool
	KList
	KTuple
	KStruct
	KNode
	// KUnknown is a value that the caller of an evaluation does not know yet (see Unknown).
	KUnknown
)

// V is a value of a term, tagged by its kind and held in the most naive form: an int is an int64, a
// list or a tuple holds its elements, a struct its fields by name. A field that was never set is
// absent from Fields and reads as nil.
type V struct {
	Kind   Kind
	I      int64
	S      string
	B      bool
	Items  []V
	Type   string
	Fields map[string]V
	// Start and End are the range of a node, in the units of the input.
	Start, End int
	// Shape is the shape of a node: its value is the string S (the text of its range), or the tuple
	// or the list of Items. S is the text of the range of every node.
	Shape Shape
	// Caps are the captures of a node by name; a capture that did not match is absent and reads as nil.
	// For a node of UnknownShape, a nil Caps means that its captures are unknown as well.
	Caps map[string]V
}

// Shape is the shape of the value of a node.
type Shape int

// Shapes of nodes.
const (
	StringNode Shape = iota
	TupleNode
	ListNode
	// UnknownShape is the shape of a node whose range and text are known but whose value and type are
	// not, such as a node that a generator has produced without tracking its elements. Every operation
	// that reads its value or its type reads Unknown.
	UnknownShape
)

// Node returns a node of the input of the shape with the text and the range, the elements of a tuple
// or a list node, and the captures. A named node has Type set to the name of its node type.
func Node(shape Shape, text string, start, end int, items []V, caps map[string]V) V {
	return V{Kind: KNode, Shape: shape, S: text, Start: start, End: end, Items: items, Caps: caps}
}

// nodeValue is the value of a node, .Value: its string, its tuple or its list of elements.
func (v V) nodeValue() V {
	switch v.Shape {
	case UnknownShape:
		need(Unknown)
	case TupleNode:
		return Tuple(v.Items...)
	case ListNode:
		return List(v.Items...)
	}
	return Str(v.S)
}

// Constructors of the scalar values.
var Nil = V{}

func Int(i int64) V   { return V{Kind: KInt, I: i} }
func Str(s string) V  { return V{Kind: KStr, S: s} }
func Bool(b bool) V   { return V{Kind: KBool, B: b} }
func List(xs ...V) V  { return V{Kind: KList, Items: xs} }
func Tuple(xs ...V) V { return V{Kind: KTuple, Items: xs} }

// TypeName is the name of the type of v in an error message.
func (v V) TypeName() string {
	switch v.Kind {
	case KNil:
		return "nil"
	case KInt:
		return "int"
	case KStr:
		return "string"
	case KBool:
		return "bool"
	case KList:
		return "[]_"
	case KTuple:
		if len(v.Items) == 1 {
			return "(_,)"
		}
		return "(" + strings.TrimSuffix(strings.Repeat("_, ", len(v.Items)), ", ") + ")"
	case KStruct:
		return v.Type
	case KUnknown:
		return "unknown"
	}
	if v.Type != "" {
		return v.Type // a named node type
	}
	switch v.Shape {
	case TupleNode:
		return "node[" + Tuple(v.Items...).TypeName() + "]"
	case ListNode:
		return "node[[]_]"
	case UnknownShape:
		return "node"
	}
	return "node[string]"
}

func (v V) comparable() bool { return v.Kind <= KBool }

func (v V) String() string {
	switch v.Kind {
	case KNil:
		return "nil"
	case KInt:
		return fmt.Sprint(v.I)
	case KStr:
		return fmt.Sprintf("%q", v.S)
	case KBool:
		return fmt.Sprint(v.B)
	case KList, KTuple:
		parts := make([]string, len(v.Items))
		for i, x := range v.Items {
			parts[i] = x.String()
		}
		open, end := "[", "]"
		if v.Kind == KTuple {
			open, end = "(", ")"
		}
		return open + strings.Join(parts, ", ") + end
	}
	return v.TypeName()
}

// Int64 holds the integers of 64 bits: the result of an operator on ints is its result in the
// integers, reduced to the range of the type by wraparound.
var two64 = new(big.Int).Lsh(big.NewInt(1), 64)

func wrap(x *big.Int) int64 {
	m := new(big.Int).And(x, new(big.Int).Sub(two64, big.NewInt(1)))
	if m.Bit(63) == 1 {
		m.Sub(m, two64)
	}
	return m.Int64()
}

// compareStrings compares by code point.
func compareStrings(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		if ra[i] != rb[i] {
			if ra[i] < rb[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(ra) < len(rb):
		return -1
	case len(ra) > len(rb):
		return 1
	}
	return 0
}

const comparisonHint = "nodes and other structured values cannot be compared; use text(x), or .Start and .End"

func invalidOperands(op string, l, r V) string {
	msg := fmt.Sprintf("invalid operands %s and %s for %s", l.TypeName(), r.TypeName(), op)
	if (op == "==" || op == "!=") && !(l.comparable() && r.comparable()) {
		msg += ": " + comparisonHint
	}
	return msg
}

// Binary applies the operator op to l and r (binary). It returns the value and the text of the
// error, "" if there is none.
func Binary(op string, l, r V) (V, string) {
	switch op {
	case "[]":
		var items []V
		switch l.Kind {
		case KNil:
		case KList, KTuple:
			items = l.Items
		default:
			return Nil, "[]: expected a list, got " + l.TypeName()
		}
		if r.Kind != KInt {
			return Nil, "index must be int, got " + r.TypeName()
		}
		if r.I < 0 || r.I >= int64(len(items)) {
			return Nil, fmt.Sprintf("index %d out of range for %d elements", r.I, len(items))
		}
		return items[r.I], ""
	case "==", "!=":
		var eq bool
		switch {
		case l.Kind == KNil || r.Kind == KNil:
			eq = l.Kind == r.Kind
		case l.Kind == r.Kind && l.Kind == KInt:
			eq = l.I == r.I
		case l.Kind == r.Kind && l.Kind == KStr:
			eq = l.S == r.S
		case l.Kind == r.Kind && l.Kind == KBool:
			eq = l.B == r.B
		default:
			return Nil, invalidOperands(op, l, r)
		}
		return Bool(eq == (op == "==")), ""
	}
	switch {
	case l.Kind == KInt && r.Kind == KInt:
		a, b := big.NewInt(l.I), big.NewInt(r.I)
		switch op {
		case "+":
			return Int(wrap(new(big.Int).Add(a, b))), ""
		case "-":
			return Int(wrap(new(big.Int).Sub(a, b))), ""
		case "*":
			return Int(wrap(new(big.Int).Mul(a, b))), ""
		case "/", "%":
			if r.I == 0 {
				return Nil, "division by zero"
			}
			if op == "/" {
				return Int(wrap(new(big.Int).Quo(a, b))), ""
			}
			return Int(wrap(new(big.Int).Rem(a, b))), ""
		case "<", "<=", ">", ">=":
			return Bool(order(op, a.Cmp(b))), ""
		}
	case l.Kind == KStr && r.Kind == KStr:
		switch op {
		case "+":
			return Str(l.S + r.S), ""
		case "<", "<=", ">", ">=":
			return Bool(order(op, compareStrings(l.S, r.S))), ""
		}
	}
	return Nil, invalidOperands(op, l, r)
}

func order(op string, c int) bool {
	switch op {
	case "<":
		return c < 0
	case "<=":
		return c <= 0
	case ">":
		return c > 0
	}
	return c >= 0
}

// Unary applies the operator op to x (unary).
func Unary(op string, x V) (V, string) {
	switch {
	case op == "-" && x.Kind == KInt:
		return Int(wrap(new(big.Int).Neg(big.NewInt(x.I)))), ""
	case op == "!" && x.Kind == KBool:
		return Bool(!x.B), ""
	}
	return Nil, fmt.Sprintf("invalid operand %s for unary %s", x.TypeName(), op)
}

// Truth checks the operand of a logical operator (logic).
func Truth(op string, x V) (bool, string) {
	if x.Kind != KBool {
		return false, fmt.Sprintf("invalid operand %s for %s", x.TypeName(), op)
	}
	return x.B, ""
}

// Values is the model of the value primitives, a machine for the conformance harness. It holds no
// state. Its arguments and results are V; a nil error is Nil and an error is a Str.
type Values struct{}

func errValue(msg string) V {
	if msg == "" {
		return Nil
	}
	return Str(msg)
}

// Call implements exec.Machine.
func (Values) Call(o *exec.Op, args []any) []any {
	code := func(i int, names []string) string { return names[args[i].(int64)] }
	switch o.Name {
	case "binary":
		v, e := Binary(code(0, ops.BinaryOps), args[1].(V), args[2].(V))
		return []any{v, errValue(e)}
	case "unary":
		v, e := Unary(code(0, ops.UnaryOps), args[1].(V))
		return []any{v, errValue(e)}
	case "logic":
		b, e := Truth(code(0, ops.LogicalOps), args[1].(V))
		return []any{b, errValue(e)}
	}
	panic("ref: unknown value operation " + o.Name)
}

// unitLen is the length of s in the units of the input.
func unitLen(s string, bytes bool) int {
	if bytes {
		return len(s)
	}
	return utf8.RuneCountInString(s)
}
