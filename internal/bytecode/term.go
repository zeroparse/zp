package bytecode

// Term is the term code of an action, a predicate or the value of a binding: the instructions of the
// function that the lowering of terms makes of the term, over registers of four files, with the
// pools they index. The instruction set and its checks belong to the engine, which compiles and runs
// term code (internal/engine/core); the module stores it and checks that the pools refer to strings
// of the string table. See "Term Code" in spec/bytecode/README.md.
type Term struct {
	// Regs holds the sizes of the register files: values, integers and booleans, element
	// sequences and strings.
	Regs [4]int
	Code []TermInstr
	// Consts holds the constants of the term.
	Consts []TermConst
	// Names holds string table indices: built-in names, the names of Operators Rule locals and of
	// struct fields.
	Names []int
	// Members holds the member reads.
	Members []TermMember
	// NodeTypes holds the named node types that conversions make (string table indices).
	NodeTypes []int
	// Casts holds the types of casts.
	Casts []TermCast
	// Structs holds the struct types that literals make.
	Structs []TermStruct
}

// TermInstr is an instruction of term code: an opcode, a destination register or jump target, and
// up to three operands.
type TermInstr struct {
	Op      uint8
	Dst     int32
	A, B, C int32
}

// Kinds of a term constant.
const (
	ConstNil = iota
	ConstInt
	ConstString
	ConstBool
)

// TermConst is a constant: nil, an implementation int, a string (string table index in Int) or a
// bool (0 or 1 in Int).
type TermConst struct {
	Kind int
	Int  int64
}

// TermMember is a member read: the kind of member (a field, Start, End, Value or a capture), the
// name the term reads and the field or capture it names (string table indices).
type TermMember struct {
	Kind        int
	Name, Field int
}

// TermCast is the type of a cast: its resolved type text (string table index, as the type table
// writes an underlying node type) and its grammar position, 0 and 0 without source information.
type TermCast struct {
	Type      int
	Line, Col int
}

// TermStruct is a struct literal: its struct type (string table index) and number of fields.
type TermStruct struct {
	Type   int
	Fields int
}
