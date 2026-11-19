package bytecode

import "fmt"

// Flow is the body of a rule that the VM runs from Flow IR (Module.Flows): a flat list of
// instructions over the registers of one frame, in which the operations of the Flow dialect
// (internal/ir/code/flow) are the instructions that touch the parser. The rule's entry and its
// value are those of the rule's bytecode form: a body returns its value and true, or nil and false.
//
// The immediates of the instructions are Go values of the types below; FlowShapes says which one each
// operation has and how many registers it names.
type Flow struct {
	// Regs is the number of registers of the frame.
	Regs int
	Code []FlowInstr
}

// FlowOp is the operation of a FlowInstr.
type FlowOp uint8

const (
	// FlowConst sets Dst[0] to the immediate (a bool, an int, or nil for the null value).
	FlowConst FlowOp = iota
	// FlowMove sets Dst[0] to Args[0].
	FlowMove
	// FlowNot sets Dst[0] to the negation of the bool Args[0].
	FlowNot
	// FlowPos sets Dst[0] to the position.
	FlowPos
	// FlowSetPos sets the position to the int Args[0].
	FlowSetPos
	// FlowJump continues at Target.
	FlowJump
	// FlowJumpFalse continues at Target when the bool Args[0] is false.
	FlowJumpFalse
	// FlowReturn returns the value Args[0] and the bool Args[1].
	FlowReturn
	// FlowMark sets Dst[0] to the mark of the whole parser state (flow.markState).
	FlowMark
	// FlowReset restores the parser state to the mark Args[0] (flow.reset).
	FlowReset
	// FlowPeek sets Dst[0] to the character, Dst[1] to its width and Dst[2] to whether there is one
	// (flow.peek); a Dst of -1 is discarded.
	FlowPeek
	// FlowGuard sets Dst[0] to whether the guard of the immediate FlowGuardOf fails for the
	// character Args[0] that was read when Args[1] (the peek's ok) holds (flow.guardFails).
	FlowGuard
	// FlowExpect records the expectation of the immediate FlowExpectOf at the position Args[0]
	// (flow.expect).
	FlowExpect
	// FlowUnfresh marks the node Args[0], if it is one, as no longer owned by its maker
	// (flow.unfresh).
	FlowUnfresh
	// FlowCall calls the rule of the immediate FlowCallOf and sets Dst[0] to its value and Dst[1] to
	// its success (flow.callRule).
	FlowCall
)

// FlowImm is the kind of immediate of an operation.
type FlowImm uint8

const (
	// FlowImmNone is no immediate: Imm is nil.
	FlowImmNone FlowImm = iota
	// FlowImmConst is a constant: nil, a bool or an int.
	FlowImmConst
	// FlowImmGuard is a FlowGuardOf.
	FlowImmGuard
	// FlowImmExpect is a FlowExpectOf.
	FlowImmExpect
	// FlowImmCall is a FlowCallOf.
	FlowImmCall
)

// FlowShape is what the instructions of an operation hold: the number of destination and argument
// registers, whether the instruction has a jump target, and its kind of immediate.
type FlowShape struct {
	Name      string
	Dst, Args int
	Target    bool
	Imm       FlowImm
}

// FlowShapes holds the shape of each operation, by FlowOp. A module file writes an instruction's
// registers, target and immediate as its shape says, and loading a module checks each instruction
// against it.
var FlowShapes = [...]FlowShape{
	FlowConst:     {Name: "CONST", Dst: 1, Imm: FlowImmConst},
	FlowMove:      {Name: "MOVE", Dst: 1, Args: 1},
	FlowNot:       {Name: "NOT", Dst: 1, Args: 1},
	FlowPos:       {Name: "POS", Dst: 1},
	FlowSetPos:    {Name: "SETPOS", Args: 1},
	FlowJump:      {Name: "JUMP", Target: true},
	FlowJumpFalse: {Name: "JUMPFALSE", Args: 1, Target: true},
	FlowReturn:    {Name: "RETURN", Args: 2},
	FlowMark:      {Name: "MARK", Dst: 1},
	FlowReset:     {Name: "RESET", Args: 1},
	FlowPeek:      {Name: "PEEK", Dst: 3},
	FlowGuard:     {Name: "GUARD", Dst: 1, Args: 2, Imm: FlowImmGuard},
	FlowExpect:    {Name: "EXPECT", Args: 1, Imm: FlowImmExpect},
	FlowUnfresh:   {Name: "UNFRESH", Args: 1},
	FlowCall:      {Name: "CALL", Dst: 2, Imm: FlowImmCall},
}

// Shape returns the shape of o; ok is false for an operation that does not exist.
func (o FlowOp) Shape() (s FlowShape, ok bool) {
	if int(o) >= len(FlowShapes) {
		return FlowShape{}, false
	}
	return FlowShapes[o], true
}

func (o FlowOp) String() string {
	if s, ok := o.Shape(); ok {
		return s.Name
	}
	return fmt.Sprintf("FLOWOP%d", uint8(o))
}

// FlowInstr is an instruction of a Flow body. Dst, Args and Target are used as the operation
// documents and its shape (FlowShapes) counts; a destination of -1 discards the value. Imm is its
// immediate.
type FlowInstr struct {
	Op     FlowOp
	Dst    []int
	Args   []int
	Target int
	Imm    any
}

// FlowGuardOf is the immediate of FlowGuard: a literal's first rune (Lit, with Fold for a
// case-insensitive literal), or the class Class (an index of Module.Classes), and the depth the
// guard adds to the parser's depth.
type FlowGuardOf struct {
	Lit   bool
	Rune  rune
	Fold  bool
	Class int
	Depth int
}

// FlowExpectOf is the immediate of FlowExpect: the expectation's ID in the parser's table (below
// FixedExps one of the fixed expectations, otherwise FixedExps plus an index of Module.Strings), and
// whether it is the message of a failed @error.
type FlowExpectOf struct {
	Exp     int
	Message bool
}

// FlowCallOf is the immediate of FlowCall: the rule's index and the binding level it is called at.
type FlowCallOf struct{ Rule, Level int }

// FixedExps is the number of fixed expectations at the start of the parser's expectation table (any
// character and the four anchors), which the expectation IDs of FlowExpectOf count before the
// strings of the module.
const FixedExps = 5
