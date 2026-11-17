package core

// ActionExpr is an action expression: the expression of an action, a predicate or a binding, with
// its names resolved and its checked type (TypeOf).
type ActionExpr interface{ typ() *Type }

// termBase holds the checked type of a term.
type termBase struct{ t Type }

func (b *termBase) typ() *Type { return &b.t }

// TypeOf returns the checked type of the term t: the type of its value, nil for a lambda, which
// only built-ins call.
func TypeOf(t ActionExpr) Type { return *t.typ() }

// SetType sets the checked type of the term t.
func SetType(t ActionExpr, typ Type) { *t.typ() = typ }

// Const is a constant: an int, a string, a bool or nil.
type Const struct {
	termBase
	Value any
}

// SlotRef reads a capture slot of the scope that encloses the term.
type SlotRef struct {
	termBase
	Slot int
	Name string
}

// LocalKind is the kind of a local name.
type LocalKind uint8

const (
	// LocalParam is a lambda parameter.
	LocalParam LocalKind = iota
	// LocalLhs, LocalRhs and LocalOp are $lhs, $rhs and $op of an Operators Rule operator line.
	LocalLhs
	LocalRhs
	LocalOp
	// LocalValue is $value, the folded value, in the whole-rule action of an Operators Rule.
	LocalValue
)

// Local reads a lambda parameter or an Operators Rule local.
type Local struct {
	termBase
	Kind LocalKind
	Name string
	// Lambda is the lambda that binds a parameter.
	Lambda *Lambda
}

// VarRef reads a variable.
type VarRef struct {
	termBase
	Name string
}

// Member reads a member of a value: a field or a node member ($o, .Start, ...).
type Member struct {
	termBase
	X    ActionExpr
	Name string
	// Field is the capture of the value that a projection of an element can stand for: the member
	// name without its $ for a read of a capture (x.$f and x.f give f), and empty for a node member
	// (.Start, .End, ...).
	Field string
}

// New makes a struct.
type New struct {
	termBase
	// Type is the struct type.
	Type string
	// Fields are the fields given, in the order written.
	Fields []FieldInit
	// Declared holds the struct's field names in declaration order.
	Declared []string
	// Final reports that the struct is the rule's or line's result: its range is the rule's.
	Final bool
}

// FieldInit is a field given to New.
type FieldInit struct {
	Name  string
	Value ActionExpr
}

// Convert converts a value to a named node type.
type Convert struct {
	termBase
	NodeType string
	X        ActionExpr
	// Proven reports that the checker proved that every value of X converts.
	Proven bool
}

// Tuple makes a tuple.
type Tuple struct {
	termBase
	Elems []ActionExpr
}

// Builtin calls a built-in function: len, text, foldl, foldr, map, filter, list or concat.
type Builtin struct {
	termBase
	Func string
	Args []ActionExpr
}

// Lambda is a function literal, an argument of a built-in.
type Lambda struct {
	termBase
	Params []string
	Body   ActionExpr
}

// Binary is a binary operation.
type Binary struct {
	termBase
	Op   string
	L, R ActionExpr
}

// Unary is a unary operation.
type Unary struct {
	termBase
	Op string
	X  ActionExpr
}

// Cast converts a value to a checked type. Check and Key are the backend's handles to the
// checked type and to the cast; Check is nil for a type that every value fits, which checks nothing.
type Cast struct {
	termBase
	X         ActionExpr
	Check     any
	Key       any
	Line, Col int
}

// ActionExprChildren returns the direct subexpressions of t.
func ActionExprChildren(t ActionExpr) []ActionExpr {
	switch t := t.(type) {
	case *Member:
		return []ActionExpr{t.X}
	case *New:
		xs := make([]ActionExpr, len(t.Fields))
		for i, f := range t.Fields {
			xs[i] = f.Value
		}
		return xs
	case *Convert:
		return []ActionExpr{t.X}
	case *Tuple:
		return t.Elems
	case *Builtin:
		return t.Args
	case *Lambda:
		return []ActionExpr{t.Body}
	case *Binary:
		return []ActionExpr{t.L, t.R}
	case *Unary:
		return []ActionExpr{t.X}
	case *Cast:
		return []ActionExpr{t.X}
	}
	return nil
}

// WalkActionExpr calls f for t and its subexpressions in preorder.
func WalkActionExpr(t ActionExpr, f func(ActionExpr)) {
	if t == nil {
		return
	}
	f(t)
	for _, c := range ActionExprChildren(t) {
		WalkActionExpr(c, f)
	}
}

// Raises reports whether evaluating t can end the parse with an error. Constants and reads of
// slots, locals and variables cannot, nor can a conversion that the checker proved; every other
// operation can (a missing field, a type error, a failed conversion or cast).
func Raises(t ActionExpr) bool {
	switch t := t.(type) {
	case *Const, *SlotRef, *Local, *VarRef:
		return false
	case *Lambda:
		return Raises(t.Body)
	case *Convert:
		return !t.Proven || Raises(t.X)
	}
	return true
}
