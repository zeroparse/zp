package action

import (
	"fmt"
	"reflect"
	"strconv"

	"zeroparse.dev/zp/internal/ir/code"
)

// Runtime is the component of the leaves of terms: the state of the target's runtime that a term reads
// and writes besides its values, which are the captures, locals and variables in scope, the budget,
// and the parse that a failure ends.
const Runtime = "runtime"

// Operand is the handle type of an operand of term.compare.
const Operand = "Operand"

// Names of the leaves of terms: the operations that the lowering of an action expression calls besides
// the compositions of the abstract machine. Each is a primitive operation that a target prints in place.
const (
	OpTermConst        = "term.const"
	OpTermRead         = "term.read"
	OpTermName         = "term.name"
	OpTermField        = "term.field"
	OpTermNodeType     = "term.nodeType"
	OpTermCastType     = "term.castType"
	OpTermRaise        = "term.raise"
	OpTermCastError    = "term.castError"
	OpTermTextEqual    = "term.textEqual"
	OpTermFromStr      = "term.fromStr"
	OpTermSpend        = "term.spend"
	OpTermStruct       = "term.struct"
	OpTermSetField     = "term.setField"
	OpTermFromObj      = "term.fromObj"
	OpTermCopyList     = "term.copyList"
	OpTermElemsAbove   = "term.elemsAbove"
	OpTermKeep         = "term.keep"
	OpTermKids         = "term.kids"
	OpTermFieldOperand = "term.fieldOperand"
	OpTermOperand      = "term.operand"
	OpTermCompare      = "term.compare"
	OpTermUncontrolled = "term.uncontrolled"
	OpTermTextIn       = "term.textIn"
	// OpTermNew is the prefix of term.newN, the call of the constructor of a struct type with N
	// declared fields (NewOp).
	OpTermNew = "term.new"
)

// Const is the immediate operand of term.const: an int, a string, a bool or nil.
type Const struct{ Value any }

func (c Const) String() string {
	if s, ok := c.Value.(string); ok {
		return strconv.Quote(s)
	}
	return fmt.Sprint(c.Value)
}

// ReadKind is what term.read reads.
type ReadKind uint8

const (
	// ReadCapture reads a capture of the rule.
	ReadCapture ReadKind = iota
	// ReadLocal reads a local name of the term.
	ReadLocal
	// ReadVar reads a variable of the grammar.
	ReadVar
)

// Read is the immediate operand of term.read: the capture, local or variable it reads.
type Read struct {
	Kind ReadKind
	Name string
}

func (r Read) String() string {
	if r.Kind == ReadVar {
		return "var " + r.Name
	}
	return "$" + r.Name
}

// Field is the immediate operand of term.field: the name of a member (Name) and the name of the field or
// capture that it reads (Field), which is Name without the $ of a capture.
type Field struct{ Name, Field string }

func (f Field) String() string { return f.Field + " of " + f.Name }

// Cast is the immediate operand of term.castType and term.castError: the checked type of a cast, in the
// form the front end gives it (Check), the key by which the program names it (Key), and the position of
// the cast in the grammar.
type Cast struct {
	Check, Key any
	Line, Col  int
}

func (c Cast) String() string { return fmt.Sprintf("cast %d:%d", c.Line, c.Col) }

// New is the immediate operand of term.struct and term.newN: a struct literal's type, the number of
// fields it gives, and whether the struct is the result of its rule or line (its range is the rule's).
type New struct {
	Type   string
	Fields int
	Final  bool
}

func (n New) String() string { return n.Type }

var immTypes = map[string]reflect.Type{
	OpTermConst: reflect.TypeFor[Const](), OpTermRead: reflect.TypeFor[Read](), OpTermName: reflect.TypeFor[string](),
	OpTermField: reflect.TypeFor[Field](), OpTermNodeType: reflect.TypeFor[string](), OpTermCastType: reflect.TypeFor[Cast](),
	OpTermCastError: reflect.TypeFor[Cast](), OpTermStruct: reflect.TypeFor[New](), OpTermSetField: reflect.TypeFor[string](),
	OpTermFieldOperand: reflect.TypeFor[string](), OpTermCompare: reflect.TypeFor[string](), OpTermTextIn: reflect.TypeFor[[]string](),
}

// DeclareLeaves adds the component Runtime, the handle type Operand and the leaves of terms to m, which
// declares the rest of the dialect (Declare).
func DeclareLeaves(m *code.Module) {
	m.AddComponent(Runtime, "The state of the target's runtime that the leaves of terms read and write.")
	datum, failT := code.THandle(Datum), code.THandle(Fail)
	str := code.THandle(Str)
	reads := []string{Runtime}
	op := func(o *code.Op) {
		o.Component, o.Kind, o.Dialect = Runtime, code.OpPrimitive, Dialect
		m.AddOp(o)
	}
	op(&code.Op{Name: OpTermConst, ImmType: immTypes[OpTermConst], Results: []code.Type{datum}, Imm: "the constant (Const)",
		Contract: "Returns the constant in Imm."})
	op(&code.Op{Name: OpTermRead, ImmType: immTypes[OpTermRead], Results: []code.Type{datum}, Reads: reads, Imm: "the capture, local or variable (Read)",
		Contract: "Returns the value of the capture, the local or the variable in Imm, as the target reads a leaf of an action."})
	op(&code.Op{Name: OpTermName, ImmType: immTypes[OpTermName], Results: []code.Type{code.THandle(Name)}, Imm: "the name (a string)",
		Contract: "Returns the name in Imm."})
	op(&code.Op{Name: OpTermField, ImmType: immTypes[OpTermField], Results: []code.Type{code.THandle(Name)}, Imm: "the member and the name it reads (Field)",
		Contract: "Returns the name of the field or the capture that the member in Imm reads. A target whose comparisons of names charge less for the same string data than for equal bytes cuts it from the name of the member, as its evaluation of terms does, so that the charge does not depend on whether equal literals share their data."})
	op(&code.Op{Name: OpTermNodeType, ImmType: immTypes[OpTermNodeType], Results: []code.Type{code.THandle(NodeType)}, Reads: reads, Imm: "the name of the node type (a string)",
		Contract: "Returns the named node type in Imm."})
	op(&code.Op{Name: OpTermCastType, ImmType: immTypes[OpTermCastType], Results: []code.Type{code.THandle(FType)}, Imm: "the cast (Cast)",
		Contract: "Returns the checked type of the cast in Imm."})
	op(&code.Op{Name: OpTermRaise, ImmType: immTypes[OpTermRaise], Params: []code.Param{code.P("f", failT)}, Reads: reads, Writes: reads, NoReturn: true,
		Contract: "Ends the parse with the failure f of a term evaluated in place of an action (the lowering evaluates it in place), as the failure of an action does."})
	op(&code.Op{Name: OpTermCastError, ImmType: immTypes[OpTermCastError], Params: []code.Param{code.P("v", datum)}, Results: []code.Type{failT}, Reads: reads, Writes: reads,
		Imm: "the cast (Cast)", Contract: "Returns the failure of the cast in Imm of the value v."})
	op(&code.Op{Name: OpTermTextEqual, ImmType: immTypes[OpTermTextEqual], Params: []code.Param{code.P("l", str), code.P("r", str)}, Results: []code.Type{code.TBool},
		Reads: reads, Writes: reads, Contract: "Returns whether the strings l and r are equal, comparing them as == compares two strings."})
	op(&code.Op{Name: OpTermSpend, ImmType: immTypes[OpTermSpend], Reads: reads, Writes: reads, NoReturn: true, Cost: code.Cost{Const: 1},
		Contract: "Charges one step of the budget for a subterm, under budget controls."})
	op(&code.Op{Name: OpTermFromStr, ImmType: immTypes[OpTermFromStr], Params: []code.Param{code.P("s", str)}, Results: []code.Type{datum},
		Contract: "Returns the string s as a value."})
	obj, seq := code.THandle(Obj), code.THandle(Seq)
	op(&code.Op{Name: OpTermFromObj, ImmType: immTypes[OpTermFromObj], Params: []code.Param{code.P("n", obj)}, Results: []code.Type{datum},
		Contract: "Returns the struct, list value or tuple value n as a value."})
	op(&code.Op{Name: OpTermStruct, ImmType: immTypes[OpTermStruct], Results: []code.Type{obj}, Reads: reads, Writes: reads, NoReturn: true, Imm: "the struct literal (New)",
		Contract: "Returns a new struct of the type of the literal in Imm with no field set and an empty range, with room for the fields the literal gives."})
	op(&code.Op{Name: OpTermSetField, ImmType: immTypes[OpTermSetField], Params: []code.Param{code.P("n", obj), code.P("v", datum)}, Reads: reads, Writes: reads, NoReturn: true,
		Imm:      "the name of the field (a string)",
		Contract: "Sets the field in Imm of the struct n, which a term is making, to v. A target that bounds the work charges for the fields it scans and the bytes of the names it compares."})
	op(&code.Op{Name: OpTermCopyList, ImmType: immTypes[OpTermCopyList], Params: []code.Param{code.P("items", seq)}, Results: []code.Type{datum}, Reads: reads, Writes: reads,
		Contract: "Returns a new list value of the elements items, which items returned, as they are, charging for copying them under budget controls."})
	m.AddHandle(Operand, Runtime, "An operand of a comparison: a value, or a string field read without making a value of it.")
	operand := code.THandle(Operand)
	op(&code.Op{Name: OpTermFieldOperand, ImmType: immTypes[OpTermFieldOperand], Params: []code.Param{code.P("x", datum)}, Results: []code.Type{operand, failT}, Reads: reads, Writes: reads,
		Imm: "the name of the member (a string)", Contract: "Reads the field in Imm of x as member does, and returns it as an operand and the failure of the read; a string field stays a string, with no value made of it."})
	op(&code.Op{Name: OpTermOperand, ImmType: immTypes[OpTermOperand], Params: []code.Param{code.P("v", datum)}, Results: []code.Type{operand},
		Contract: "Returns the value v as an operand."})
	op(&code.Op{Name: OpTermCompare, ImmType: immTypes[OpTermCompare], Params: []code.Param{code.P("l", operand), code.P("r", operand)}, Results: []code.Type{datum, failT}, Reads: reads, Writes: reads,
		Imm: "the comparison operator (a string)", Contract: "Applies the comparison in Imm to l and r as binary does, comparing two strings directly, and returns the value and the failure."})
	op(&code.Op{Name: OpTermUncontrolled, ImmType: immTypes[OpTermUncontrolled], Results: []code.Type{code.TBool}, Reads: reads,
		Contract: "Returns whether the parse runs without limit controls."})
	op(&code.Op{Name: OpTermTextIn, ImmType: immTypes[OpTermTextIn], Params: []code.Param{code.P("s", str)}, Results: []code.Type{code.TBool},
		Imm: "the literals (a []string of distinct strings)", Contract: "Returns whether s is one of the literals in Imm, charging nothing."})
}

// NewOp returns term.newN, the call of the constructor of a struct type with n declared fields, which a
// target that has constructors (the typed Go library) prints as one call.
func NewOp(n int) *code.Op {
	params := make([]code.Param, n)
	for i := range params {
		params[i] = code.P("f"+strconv.Itoa(i), code.THandle(Datum))
	}
	return &code.Op{Name: OpTermNew + strconv.Itoa(n), Component: Runtime, Kind: code.OpPrimitive, Dialect: Dialect,
		Params: params, Results: []code.Type{code.THandle(Datum)}, Reads: []string{Runtime}, Writes: []string{Runtime},
		Imm: "the struct literal (New)", ImmType: reflect.TypeFor[New](),
		Contract: "Returns the struct that the literal in Imm makes of the field values, given in the order its type declares them; nil for a field the literal does not give."}
}
