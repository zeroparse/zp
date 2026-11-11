package ops

import "zeroparse.dev/zp/internal/ir/exec"

// Val is the component of the value primitives: the operations on the values of terms. A primitive
// acts on one kind of value and has none of the policy of an operator: the compositions binary,
// unary, equal, index and logic (internal/ir/exec/core) decide which primitive applies, which
// operands are valid, and which failure a term reports.
const Val = "value"

// Binary operators of the composition binary, by the code a call passes. The last, the index of a
// list or a tuple, is not a scalar operator and takes the composition index.
var BinaryOps = []string{"+", "-", "*", "/", "%", "==", "!=", "<", "<=", ">", ">=", "[]"}

// Codes of the binary operators the compositions branch on. The codes of the three groups (binary,
// unary and logical) are separate: each is an index into the table of its own group, so OpAdd, OpNeg
// and OpAnd are all 0. A call passes the code of the group of the method it calls.
const (
	OpAdd = iota
	OpSub
	OpMul
	OpQuo
	OpRem
	OpEq
	OpNe
	OpLt
	OpLe
	OpGt
	OpGe
	OpIndex
)

// UnaryOps are the unary operators of the composition unary, by the code a call passes.
var UnaryOps = []string{"-", "!"}

// BinaryNames, UnaryNames and LogicalNames are the names of the operators of the three tables, which
// targets use to name their codes.
var (
	BinaryNames  = []string{"Add", "Sub", "Mul", "Quo", "Rem", "Eq", "Ne", "Lt", "Le", "Gt", "Ge", "Index"}
	UnaryNames   = []string{"Neg", "Not"}
	LogicalNames = []string{"And", "Or"}
)

// Ident returns the identifier of an operator code in a target: Ident("Add") is "OpAdd".
func Ident(name string) string { return "Op" + name }

// OperatorIdent returns the identifier of the operator sym of the binary operators, or of the unary
// operators if unary is set, or of the logical operators if logical is set: "+" is "OpAdd".
func OperatorIdent(sym string, unary, logical bool) string {
	list, names := BinaryOps, BinaryNames
	switch {
	case unary:
		list, names = UnaryOps, UnaryNames
	case logical:
		list, names = LogicalOps, LogicalNames
	}
	for i, s := range list {
		if s == sym {
			return Ident(names[i])
		}
	}
	panic("ops: no operator " + sym)
}

// BinaryCode returns the code of the binary operator sym.
func BinaryCode(sym string) int { return code(BinaryOps, sym) }

// UnaryCode returns the code of the unary operator sym.
func UnaryCode(sym string) int { return code(UnaryOps, sym) }

func code(list []string, sym string) int {
	for i, s := range list {
		if s == sym {
			return i
		}
	}
	panic("ops: no operator " + sym)
}

// Codes of the unary operators.
const (
	OpNeg = iota
	OpNot
)

// LogicalOps are the operators whose operands the composition logic checks, by the code a call passes.
var LogicalOps = []string{"&&", "||"}

// Codes of the logical operators.
const (
	OpAnd = iota
	OpOr
)

// Kinds of a value, as val.kind returns them.
const (
	// KindNil is nil.
	KindNil = iota
	// KindInt is an int.
	KindInt
	// KindString is a string.
	KindString
	// KindBool is a bool.
	KindBool
	// KindOther is every other value: a node, a list, a tuple, a struct, and a box that holds a
	// value (val.unbox opens a box).
	KindOther
)

// Kinds of the type a cast checks a value against (FType), as ftype.kind returns them: the letter that the
// declaration of the type in the target spells the kind with, as an int.
const (
	// FtInt is int, FtString string, FtBool bool and FtNil nil.
	FtInt    = 'i'
	FtString = 's'
	FtBool   = 'b'
	FtNil    = 'z'
	// FtNode is node[_], a node whose value is a string, a tuple or a list.
	FtNode = 'n'
	// FtStringNode is node[string], FtError error, the error value of recovery, and FtMissing the error value
	// of a missing token, which only the closure backend of the engine makes.
	FtStringNode = 'S'
	FtError      = 'E'
	FtMissing    = 'M'
	// FtNamed is a struct type or a named node type, which a value has by its type name.
	FtNamed = 'N'
	// FtList is a list type, whose element type is its Elem, and FtTuple a tuple type, whose element types
	// are its Alts.
	FtList  = 'l'
	FtTuple = 't'
	// FtNodeOf is node[T] for a T other than string and _: a node whose value is of type Elem.
	FtNodeOf = 'v'
	// FtOptional is an optional type of Elem, and FtJoin a join of Alts.
	FtOptional = 'o'
	FtJoin     = 'u'
	// FtTop is the type _, which every value fits.
	FtTop = '_'
)

// Shapes of a node, as node.shape returns them: what the node stands for in the value model.
const (
	// ShapeString is a node whose value is a string.
	ShapeString = iota
	// ShapeTuple and ShapeList are nodes whose value is a tuple or a list of its children.
	ShapeTuple
	ShapeList
	// ShapeError is an error value of recovery.
	ShapeError
	// ShapeBox is a box that carries a value other than a node.
	ShapeBox
	// ShapeTupleValue and ShapeListValue are a tuple value and a list value, of the elements that are its children.
	ShapeTupleValue
	ShapeListValue
	// ShapeStruct is a struct value.
	ShapeStruct
)

// TermError is a failure that a term reports. Text is its message with the placeholders {a} and
// {b}, the type names of the first and second value; {name}, the second value as a string, a member
// name or the name of a built-in function; {op}, the name of the operator that op indexes in BinaryOps for ErrOperands, ErrCompare and ErrNotList, in UnaryOps for ErrUnary and in LogicalOps for ErrLogical; {i}, the first
// value as an int; and {n}, a count. Hint ends the message of a failure that has one. Every target
// prints these messages and puts the text of a failure together from them, so that they read the
// same on all targets.
type TermError struct {
	Name, Text, Hint string
}

// The failures of a term, by the code val.fail takes: the index of TermErrors.
const (
	// ErrDivZero is a division or remainder by zero.
	ErrDivZero = iota
	// ErrOperands is an operator applied to operands it does not accept (a is the left and b the right
	// operand, op an index of BinaryOps).
	ErrOperands
	// ErrCompare is ErrOperands for a comparison with == or != of a value that is not nil, an int, a
	// string or a bool.
	ErrCompare
	// ErrUnary is a unary operator applied to an operand it does not accept (op an index of UnaryOps).
	ErrUnary
	// ErrLogical is a logical operator applied to an operand that is not a bool (op an index of
	// LogicalOps).
	ErrLogical
	// ErrNotList is the use of a value that is not a list or a tuple as one (op an index of BinaryOps, OpIndex).
	ErrNotList
	// ErrIndexType is an index that is not an int.
	ErrIndexType
	// ErrIndexRange is an index out of range (i is the index, n the length).
	ErrIndexRange
	// ErrMemberOf is a member read of a value that has no members (a is the value, name the member as
	// the term spells it).
	ErrMemberOf
	// ErrNoCapture is the read of a capture of a struct, which has none (a is the struct, name the
	// capture).
	ErrNoCapture
	// ErrNoField is the read of a field that a struct type does not declare (a is the struct, name the
	// field).
	ErrNoField
	// ErrElement is an element of a list or a tuple that is not a value, such as a function (a is the
	// element, name the built-in function or literal that makes the list).
	ErrElement
	// ErrArgList is an argument of a built-in function that takes a list and is not one (a is the
	// argument, name the function).
	ErrArgList
	// ErrLenArg is an argument of len that has no length (a is the argument).
	ErrLenArg
	// ErrTextArg is an argument of text that has no text (a is the argument).
	ErrTextArg
	// ErrConvert is the conversion to a named node type of a value that is not a string, tuple or list
	// node (a is the value, name the named node type).
	ErrConvert
	// ErrFilter is a result of the function of filter that is not a bool (a is the result).
	ErrFilter
)

// TermErrors are the messages of the failures of a term.
var TermErrors = []TermError{
	ErrDivZero:    {Name: "division by zero", Text: "division by zero"},
	ErrOperands:   {Name: "invalid operands", Text: "invalid operands {a} and {b} for {op}"},
	ErrCompare:    {Name: "invalid comparison", Text: "invalid operands {a} and {b} for {op}", Hint: ": nodes and other structured values cannot be compared; use text(x), or .Start and .End"},
	ErrUnary:      {Name: "invalid unary operand", Text: "invalid operand {a} for unary {op}"},
	ErrLogical:    {Name: "invalid logical operand", Text: "invalid operand {a} for {op}"},
	ErrNotList:    {Name: "not a list", Text: "{op}: expected a list, got {a}"},
	ErrIndexType:  {Name: "index not an int", Text: "index must be int, got {a}"},
	ErrIndexRange: {Name: "index out of range", Text: "index {i} out of range for {n} elements"},
	ErrMemberOf:   {Name: "no members", Text: "cannot access .{name} of {a}"},
	ErrNoCapture:  {Name: "no capture", Text: "{a} has no capture {name}"},
	ErrNoField:    {Name: "no field", Text: "{a} has no field {name}"},
	ErrElement:    {Name: "not an element", Text: "{name}: elements must be values, got {a}"},
	ErrArgList:    {Name: "argument not a list", Text: "{name}: expected a list, got {a}"},
	ErrLenArg:     {Name: "invalid len argument", Text: "len: invalid argument {a}"},
	ErrTextArg:    {Name: "invalid text argument", Text: "text: invalid argument {a}"},
	ErrConvert:    {Name: "not convertible", Text: "cannot convert {a} to {name}"},
	ErrFilter:     {Name: "filter result not a bool", Text: "filter: predicate must produce a bool, got {a}"},
}

// Kinds of a member read, by the code the composition member takes. A member name of a term is the
// name of a field, a range or value member of a node (Start, End, Value), or a capture ($ and the name
// of the capture). A range or value member reads the node itself only if the value is a node; of a
// struct or a list it is the field of that name.
const (
	// MemberField reads a field.
	MemberField = iota
	// MemberStart reads the start of the range of a node.
	MemberStart
	// MemberEnd reads the end of the range of a node.
	MemberEnd
	// MemberValue reads the value of a node.
	MemberValue
	// MemberCapture reads a capture.
	MemberCapture
)

// MemberKinds names the kinds of member reads, by code, with the spelling that selects each: the
// whole name for the members of a node, the prefix of the name for a capture, and none for a field,
// which every other name is. Every target classifies a name from this table.
var MemberKinds = []struct{ Name, Spelling string }{
	MemberField:   {"Field", ""},
	MemberStart:   {"Start", "Start"},
	MemberEnd:     {"End", "End"},
	MemberValue:   {"Value", "Value"},
	MemberCapture: {"Capture", "$"},
}

// ValueModule returns the pilot declarations with the value primitives. The primitives are not part
// of Pilot until a composition calls them.
func ValueModule() *exec.Module {
	m := Pilot()
	AddValue(m)
	return m
}

// Seq is the handle type of the elements of a list or a tuple.
const Seq = "Seq"

// Fail is the handle type of the failure of a term, as val.fail makes it: a value of the target that
// the caller reports, and null where a composition does not fail.
const Fail = "Fail"

// Obj is the handle type of a value that has members: a node, a list, a tuple or a struct, as node.of
// returns it.
const Obj = "Obj"

// Name is the handle type of the name of a member, a string of the target.
const Name = "Name"

// Act is the handle type of the context of the term being evaluated, in which a value is made.
const Act = "Act"

// Elem is the handle type of an element of a list or a tuple that a term makes, as Seq holds it: a node
// or nil in a target whose elements are nodes, and any value in a target whose elements are any values.
const Elem = "Elem"

// NodeType is the handle type of a named node type, to which a conversion converts a value.
const NodeType = "NodeType"

// Str is the handle type of a string of the target that is not a value, such as the result of text,
// which a comparison of texts reads without making a value of it.
const Str = "Str"

// FType is the handle type of the type that a cast checks a value against, which the target declares for
// each cast: its kind (the constants FtInt and the like), and the element type, the alternatives and the
// name that its kind uses.
const FType = "FType"

// QFieldsRead is the number of fields of a value that a read of one of them scans.
const QFieldsRead = "fields_read"

// QZeroParts is the number of values of which a zero value is made, and, in a target that looks the
// field up among the declared ones, the declared fields it scans.
const QZeroParts = "zero_parts"

// QValueBytes is the number of bytes of the string operands of a primitive, which it reads and, for a
// concatenation, copies.
const QValueBytes = "value_bytes"

// QSeqLen is the number of elements of the Seq argument of a primitive, which it copies.
const QSeqLen = "seq_len"

// QErrParts is the number of fragments of the text of a failure: the fixed pieces and the values
// between them, which a target that bounds the work assembles piece by piece, charging each.
const QErrParts = "error_parts"

// AddValue adds the value component and its primitives to m, which declares the handle Datum.
//
// A value is nil, an int, a string, a bool, or a structured value (a node, list, tuple or struct)
// that the primitives accept only where they say so. An int is 64 bits wide with wraparound. A
// primitive is pure unless its contract says it charges: the string primitives read their operands,
// and val.fail makes a text, which a target that bounds the work does piece by piece.
func AddValue(m *exec.Module) {
	m.AddComponent(Val, `The operations on the values of terms. They read and write no state.`)
	datum := exec.THandle(Datum)
	items := m.AddHandle(Seq, Val, "The elements of a list or a tuple, as seq.items returns them.")
	m.AddHandle(Fail, Val, "The failure of a term, as val.fail makes it: a value of the target that the caller reports.")
	pure := func(o *exec.Op) {
		o.Component = Val
		m.AddOp(o)
	}
	x, l, r := exec.P("x", datum), exec.P("l", datum), exec.P("r", datum)
	bytes := exec.Cost{Const: 1, Terms: []exec.CostTerm{{Coef: 4, Quantity: QValueBytes}}}
	pure(&exec.Op{Name: "val.kind", Params: []exec.Param{x}, Results: []exec.Type{exec.TInt},
		Contract: `Returns the kind of x: KindNil for nil, KindInt for an int, KindString for a string, KindBool
for a bool and KindOther for every other value, a box that holds one of these included.`,
		Laws: []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "val.unbox", Params: []exec.Param{x}, Results: []exec.Type{datum},
		Contract: `Returns the value that x stands for: the value a box holds, and x for any other value.`,
		Laws:     []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "val.fromBool", Params: []exec.Param{exec.P("b", exec.TBool)}, Results: []exec.Type{datum},
		Contract: `Returns the bool value b.`})
	pure(&exec.Op{Name: "val.fromInt", Params: []exec.Param{exec.P("i", exec.TInt)}, Results: []exec.Type{datum},
		Contract: `Returns the int value i.`})
	pure(&exec.Op{Name: "val.toBool", Params: []exec.Param{x}, Results: []exec.Type{exec.TBool},
		Contract: `Returns the bool that x is.`, Requires: `x is a bool: val.kind(x) is KindBool.`})
	pure(&exec.Op{Name: "val.fail", Params: []exec.Param{exec.P("code", exec.TInt), exec.P("a", datum), exec.P("b", datum),
		exec.P("op", exec.TInt), exec.P("n", exec.TInt)},
		Results: []exec.Type{exec.THandle(Fail)}, NoReturn: true, Host: true,
		Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 2, Quantity: QErrParts}}},
		Contract: `Returns the failure with the message TermErrors[code]: {a} and {b} read as the names of the types of
a and b, {op} as the name of the operator that op indexes (BinaryOps for ErrOperands and ErrCompare, UnaryOps for ErrUnary,
LogicalOps for ErrLogical), {i} as a, an int, and {n} as n. A failure is a Fail, not a value of a term: the caller
returns it and the term evaluation reports it, each target in its own form of failure. A target that bounds the work
assembles the text piece by piece, charging for each piece.`,
		Requires: `code indexes TermErrors, and the arguments are those its message names.`})
	pure(&exec.Op{Name: "int.add", Params: []exec.Param{l, r}, Results: []exec.Type{datum},
		Contract: `Returns l + r with 64-bit wraparound.`, Requires: `l and r are ints.`,
		Laws: []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "int.sub", Params: []exec.Param{l, r}, Results: []exec.Type{datum},
		Contract: `Returns l - r with 64-bit wraparound.`, Requires: `l and r are ints.`,
		Laws: []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "int.mul", Params: []exec.Param{l, r}, Results: []exec.Type{datum},
		Contract: `Returns l * r with 64-bit wraparound.`, Requires: `l and r are ints.`,
		Laws: []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "int.quo", Params: []exec.Param{l, r}, Results: []exec.Type{datum},
		Contract: `Returns l / r truncated toward zero; the least int divided by -1 is the least int.`,
		Requires: `l and r are ints, and r is not zero.`,
		Laws:     []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "int.rem", Params: []exec.Param{l, r}, Results: []exec.Type{datum},
		Contract: `Returns the remainder of l / r, with the sign of l; the remainder of the least int by -1 is zero.`,
		Requires: `l and r are ints, and r is not zero.`,
		Laws:     []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "int.neg", Params: []exec.Param{x}, Results: []exec.Type{datum},
		Contract: `Returns -x with 64-bit wraparound: the negation of the least int is itself.`, Requires: `x is an int.`,
		Laws: []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "int.zero", Params: []exec.Param{x}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether x is zero.`, Requires: `x is an int.`})
	pure(&exec.Op{Name: "int.cmp", Params: []exec.Param{l, r}, Results: []exec.Type{exec.TInt},
		Contract: `Returns -1 if l is less than r, 0 if they are equal and 1 if l is greater.`, Requires: `l and r are ints.`,
		Laws: []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "int.index", Params: []exec.Param{x, exec.P("n", exec.TInt)}, Results: []exec.Type{exec.TInt, exec.TBool},
		Contract: `Returns x as an int and true if 0 <= x < n, and 0 and false otherwise.`, Requires: `x is an int.`,
		Laws: []string{"pure: equal arguments give equal results"}})
	str := func(o *exec.Op) {
		o.Host, o.NoReturn, o.Cost = true, true, bytes
		pure(o)
	}
	str(&exec.Op{Name: "str.add", Params: []exec.Param{l, r}, Results: []exec.Type{datum},
		Contract: `Returns the concatenation of l and r. A target that bounds the work charges for the bytes it reads and copies.`,
		Requires: `l and r are strings.`})
	str(&exec.Op{Name: "str.cmp", Params: []exec.Param{l, r}, Results: []exec.Type{exec.TInt},
		Contract: `Returns -1 if l is less than r, 0 if they are equal and 1 if l is greater, comparing by code point: the
order of the bytes of UTF-8 and not of the code units of UTF-16. A target that bounds the work charges for the bytes it
compares.`,
		Requires: `l and r are strings.`})
	str(&exec.Op{Name: "str.eq", Params: []exec.Param{l, r}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether l and r are the same string. A target that bounds the work charges for the bytes it compares.`,
		Requires: `l and r are strings.`})
	pure(&exec.Op{Name: "seq.items", Params: []exec.Param{x}, Results: []exec.Type{exec.THandle(Seq), exec.TBool},
		Contract: `Returns the elements of x and true if x is nil (no elements), a list or a tuple, and false otherwise.`,
		Laws:     []string{"pure: equal arguments give equal results"}})
	obj := m.AddHandle(Obj, Val, "A value that has members: a node, a list, a tuple or a struct, as node.of returns it.")
	name := m.AddHandle(Name, Val, "The name of a member: a string of the target.")
	act := m.AddHandle(Act, Val, "The context of the term being evaluated, in which a value is made.")
	n, nm := exec.P("n", obj), exec.P("name", name)
	pure(&exec.Op{Name: "node.of", Params: []exec.Param{x}, Results: []exec.Type{exec.THandle(Obj), exec.TBool},
		Contract: `Returns x and true if x is a value that has members (a node, a list, a tuple or a struct), and
false for every other value: nil, an int, a string and a bool.`,
		Laws: []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "node.isNode", Params: []exec.Param{n}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether n is a node, which has a range and a value: a value of a node type, a string
node or an error, and not a struct, a list value or a tuple value.`})
	pure(&exec.Op{Name: "node.start", Params: []exec.Param{n}, Results: []exec.Type{datum},
		Contract: `Returns the start of the range of n as an int.`, Requires: `node.isNode(n).`})
	pure(&exec.Op{Name: "node.end", Params: []exec.Param{n}, Results: []exec.Type{datum},
		Contract: `Returns the end of the range of n as an int.`, Requires: `node.isNode(n).`})
	pure(&exec.Op{Name: "node.value", Params: []exec.Param{n}, Results: []exec.Type{datum},
		Contract: `Returns the value of n: its text, or the value its node type holds.`, Requires: `node.isNode(n).`})
	pure(&exec.Op{Name: "node.isStruct", Params: []exec.Param{n}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether n is a value of a struct type.`})
	pure(&exec.Op{Name: "node.field", Params: []exec.Param{n, nm}, Results: []exec.Type{datum, exec.TBool},
		Host: true, NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 2, Quantity: QFieldsRead}, {Coef: 4, Quantity: QValueBytes}}},
		Contract: `Returns the field name of n and true if n holds it, and nil and false otherwise. A struct of a
target that represents it with every field holds them all. A target that bounds the work charges for the fields
it scans and the bytes of the names it compares.`})
	pure(&exec.Op{Name: "node.zero", Params: []exec.Param{exec.P("c", act), n, nm}, Results: []exec.Type{datum, exec.TBool},
		Host: true, NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 2, Quantity: QZeroParts}}},
		Contract: `Returns the zero value of the declared type of the field name of the struct n, made in c, and
true, or nil and false if the type of n declares no such field or the target holds every field of a struct
(node.field then holds it). A target that bounds the work charges for the values it makes.`,
		Requires: `node.isStruct(n).`})
	pure(&exec.Op{Name: "val.name", Params: []exec.Param{nm}, Results: []exec.Type{datum},
		Contract: `Returns name as a string value, as the {name} of a failure reads it.`})
	m.AddHandle(Str, Val, "A string of the target that is not a value, as text returns it.")
	m.AddHandle(Elem, Val, "An element of a list or a tuple that a term makes, as a Seq holds it.")
	pure(&exec.Op{Name: "node.elem", Params: []exec.Param{n}, Results: []exec.Type{exec.THandle(Elem)},
		Contract: `Returns n as an element of a list or a tuple: n itself.`,
		Laws:     []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "val.box", Params: []exec.Param{exec.P("c", act), x}, Results: []exec.Type{exec.THandle(Elem)},
		Host: true, NoReturn: true, Cost: exec.Cost{Const: 1},
		Contract: `Returns x as an element of a list or a tuple: in a target whose elements are nodes, a box that holds x
with the range of the input of c, made in c, and x itself in a target whose elements are any values. A target that
bounds the work charges for the box it makes.`,
		Requires: `x is an int, a string or a bool.`})
	pure(&exec.Op{Name: "node.hasText", Params: []exec.Param{n}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether n is a string node or an error, whose text is the input of its range.`})
	pure(&exec.Op{Name: "node.width", Params: []exec.Param{n}, Results: []exec.Type{datum},
		Contract: `Returns the length of the range of n, in the position unit, as an int.`})
	pure(&exec.Op{Name: "node.count", Params: []exec.Param{n}, Results: []exec.Type{datum},
		Contract: `Returns the number of elements or children of n as an int.`})
	pure(&exec.Op{Name: "node.heldText", Params: []exec.Param{n}, Results: []exec.Type{exec.THandle(Str), exec.TBool},
		Contract: `Returns the text that n holds and true if n is a string node or an error, and false otherwise; the
text is then not meaningful.`})
	pure(&exec.Op{Name: "node.rangeText", Params: []exec.Param{n}, Results: []exec.Type{exec.THandle(Str)},
		Host: true, NoReturn: true, Cost: exec.Cost{Const: 1, Terms: []exec.CostTerm{{Coef: 1, Quantity: QValueBytes}}},
		Contract: `Returns the input of the range of n. A target that bounds the work may charge for the read of the
input, and for the bytes it copies out of a stream.`})
	pure(&exec.Op{Name: "str.len", Params: []exec.Param{x}, Results: []exec.Type{datum},
		Host: true, NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QValueBytes}}},
		Contract: `Returns the length of the string x in the position unit as an int. A target that bounds the work and
counts characters charges for the bytes it reads.`,
		Requires: `x is a string.`})
	pure(&exec.Op{Name: "str.of", Params: []exec.Param{x}, Results: []exec.Type{exec.THandle(Str)},
		Contract: `Returns the string x.`, Requires: `x is a string.`})
	pure(&exec.Op{Name: "str.empty", Results: []exec.Type{exec.THandle(Str)},
		Contract: `Returns the empty string.`})
	nt := exec.P("t", m.AddHandle(NodeType, Val, "A named node type, to which a conversion converts a value."))
	pure(&exec.Op{Name: "node.typeName", Params: []exec.Param{nt}, Results: []exec.Type{exec.THandle(Name)},
		Contract: `Returns the name of the named node type t, as the {name} of a failure reads it.`})
	pure(&exec.Op{Name: "node.hasBase", Params: []exec.Param{n}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether n is a string node, a tuple node or a list node, of a named node type or not, which
a conversion converts: not an error, a box, a list value, a tuple value or a struct.`})
	pure(&exec.Op{Name: "node.convert", Params: []exec.Param{n, nt}, Results: []exec.Type{datum},
		Contract: `Returns n as a node of the named node type t: n itself if it has the type, and otherwise a new node with
the range, the value and the captures of n, the type t and no rule name, which keeps what n holds now.`,
		Requires: `node.hasBase(n).`})
	pure(&exec.Op{Name: "node.span", Params: []exec.Param{n}, Results: []exec.Type{exec.TInt, exec.TInt},
		Contract: `Returns the start and the end of the range of n, a node, a struct, a list value or a tuple value, as
ints.`})
	m.AddOp(&exec.Op{Name: "node.setSpan", Component: Host, Params: []exec.Param{n, exec.P("start", exec.TInt), exec.P("end", exec.TInt)},
		Writes: []string{Host}, NoReturn: true,
		Contract: `Sets the range of n to start and end. It is for a value that a term is making, which no one has read.`})
	pure(&exec.Op{Name: "act.span", Params: []exec.Param{exec.P("c", act)}, Results: []exec.Type{exec.TInt, exec.TInt},
		Contract: `Returns the start and the end of the range of the rule that c evaluates the term of, as ints.`})
	m.AddOp(&exec.Op{Name: "act.made", Component: Host, Params: []exec.Param{exec.P("c", act), n}, Writes: []string{Host}, NoReturn: true,
		Contract: `Records n as a value that c made, which the parser drops with the values of a term that it abandons.`,
		Requires: `n is a struct that c made and has not recorded.`})
	m.AddOp(&exec.Op{Name: "act.push", Component: Host, Params: []exec.Param{exec.P("c", act), exec.P("e", exec.THandle(Elem))},
		Writes: []string{Host}, NoReturn: true,
		Contract: `Pushes e above the elements gathered for the lists and tuples that c is making, as the next element of the
innermost one. The host gathers them so that a list or a tuple takes its elements in one piece when it is made, and
drops them with the values of a term that it abandons.`})
	m.AddOp(&exec.Op{Name: "act.pushAll", Component: Host, Params: []exec.Param{exec.P("c", act), exec.P("items", items)},
		Writes: []string{Host}, NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QSeqLen}}},
		Contract: `Pushes the elements of items in order, as act.push pushes each. A target that bounds the work charges one
unit for each element it copies.`,
		Requires: `items came from seq.items.`})
	pure(&exec.Op{Name: "val.seq", Params: []exec.Param{exec.P("c", act), exec.P("tuple", exec.TBool), exec.P("kids", items)},
		Results: []exec.Type{exec.THandle(Obj)}, Host: true, NoReturn: true, Cost: exec.Cost{Const: 1},
		Contract: `Returns a new tuple value if tuple and a new list value otherwise, made in c, whose elements are kids, which it
takes over, and whose range is empty. A target that bounds the work charges for the value it makes.`})
	pure(&exec.Op{Name: "seq.span", Params: []exec.Param{exec.P("c", act), exec.P("items", items), exec.P("i", exec.TInt)},
		Results: []exec.Type{exec.TInt, exec.TInt, exec.TBool},
		Contract: `Returns the start and the end of the range that element i of a list value or a tuple value being made in c
adds to the range of the value, as ints, and true if it adds one, and false (and no meaningful range) if it adds none.
An element that is a node, a list, a tuple or a struct adds its own range. An int, a string or a bool adds the range of
the rule c evaluates the term of, which a target whose elements are nodes has given its box; nil adds none.`,
		Requires: `items came from seq.items or is the element list of a value being made in c, and 0 <= i < seq.len(items).`})
	ft := exec.P("t", m.AddHandle(FType, Val, "The type that a cast checks a value against."))
	pure(&exec.Op{Name: "ftype.kind", Params: []exec.Param{ft}, Results: []exec.Type{exec.TInt},
		Contract: `Returns the kind of the type t: the code of the letter of FtInt, FtString and the others.`,
		Laws:     []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "ftype.elem", Params: []exec.Param{ft}, Results: []exec.Type{exec.THandle(FType)},
		Contract: `Returns the element type of the list type, the optional type or the node type t whose value has a type.`,
		Requires: `t is of kind FtList, FtOptional or FtNodeOf.`, Laws: []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "ftype.count", Params: []exec.Param{ft}, Results: []exec.Type{exec.TInt},
		Contract: `Returns the number of the alternatives of the join type or the element types of the tuple type t.`,
		Requires: `t is of kind FtJoin or FtTuple.`, Laws: []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "ftype.alt", Params: []exec.Param{ft, exec.P("i", exec.TInt)}, Results: []exec.Type{exec.THandle(FType)},
		Contract: `Returns alternative i of the join type or element type i of the tuple type t.`,
		Requires: `t is of kind FtJoin or FtTuple, and 0 <= i < ftype.count(t).`, Laws: []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "node.shape", Params: []exec.Param{n}, Results: []exec.Type{exec.TInt},
		Contract: `Returns the shape of n: ShapeString, ShapeTuple, ShapeList, ShapeError, ShapeBox, ShapeTupleValue,
ShapeListValue or ShapeStruct, which a value of a node type has by its base.`,
		Laws: []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "node.isType", Params: []exec.Param{n, ft}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether the type name of n is the name of the type t: a struct type or a named node type.`,
		Requires: `t is of kind FtNamed.`})
	pure(&exec.Op{Name: "node.isMissing", Params: []exec.Param{n}, Results: []exec.Type{exec.TBool},
		Contract: `Reports whether n is the error value of a missing token. A target that has no missing tokens reports false.`})
	pure(&exec.Op{Name: "node.children", Params: []exec.Param{n}, Results: []exec.Type{exec.THandle(Seq)},
		Contract: `Returns the elements of the list value or the tuple value n, and the children of any other node.`,
		Laws:     []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "seq.step", Host: true, NoReturn: true, Cost: exec.Cost{Const: 1},
		Contract: `Charges the step of one element of a list or a tuple: an element that is examined against its type,
walked by a built-in function or made by a constructor. A target that bounds the work charges one unit; a target
that does not charges nothing.`})
	pure(&exec.Op{Name: "seq.len", Params: []exec.Param{exec.P("items", items)}, Results: []exec.Type{exec.TInt},
		Contract: `Returns the number of elements.`, Requires: `items came from seq.items.`})
	pure(&exec.Op{Name: "seq.at", Params: []exec.Param{exec.P("items", items), exec.P("i", exec.TInt)}, Results: []exec.Type{datum},
		Contract: `Returns element i as a value: the value a box holds, and nil for a missing element.`,
		Requires: `items came from seq.items, and 0 <= i < seq.len(items).`})
}
