// Package flow is the Flow dialect of Code IR (design 062): grammar code with explicit parser state
// and structured control, one function per rule variant, from which the host-language backends print
// code after execlower converts it to the Exec stage.
//
// A function at the Flow stage is a code.Func with Flow attributes (Attrs) in a module that NewModule
// makes. Its locals are the saved state of backtracking points, capture values, intermediate values
// and flags; its control is the base's structured control: a Block has a label, and an Exit to it
// continues after the block; a Loop repeats its body until a Break leaves it, and a Continue starts
// its next iteration; an If branches. The calls are of the operations of the dialect (ops.go), which
// mirror the runtime's helpers: reading and moving the position, matching terminals, recording
// expectations, calling rules, building values, evaluating terms and spending budget.
//
// A function is local or framed. The body of a local function, a whole rule, succeeds by running to
// the end of the block labeled Attrs.Fail, with the rule's value in the Result variable, and fails by
// an Exit to that label; its captures are variables. A framed function matches an expression whose
// captures the runtime keeps in a frame (the body of a left-recursion leader or of a rule that
// recovers, or a line of an Operators Rule): it returns its value and true, or the null value and
// false, and the runtime owns the frame, the rule's value and its finish. Statements after an Exit,
// up to the end of a block that is exited to or of a loop, are unreachable; the lowering keeps them,
// and a printer leaves out what is unreachable. VerifyModule checks the invariants of a function.
package flow

// Type is the type of a variable.
type Type uint8

const (
	// TypeInt is a position, a count or a length.
	TypeInt Type = iota
	TypeBool
	TypeRune
	// TypeValue is a value of the runtime.
	TypeValue
	// TypeEnv is a variable environment.
	TypeEnv
	// TypeExpMark is a mark of the recorded expectations.
	TypeExpMark
	// TypeExpIDs is a list of expectations.
	TypeExpIDs
	// TypeVals is a list of values being gathered.
	TypeVals
	// TypeCtx is the context of an action.
	TypeCtx
	// TypeMark is a mark of the whole parser state (markState).
	TypeMark
	// TypeFrame is a capture frame of the runtime.
	TypeFrame
	// TypeError is a syntax error.
	TypeError
)

var typeNames = [...]string{"int", "bool", "rune", "value", "env", "expmark", "expids", "vals", "ctx", "mark", "frame", "error"}

func (t Type) String() string { return typeNames[t] }

// ScanHotSpan is the number of characters that a walk of a scan site matches before it looks for the
// record of its start (scanReach); the runtimes and the engine use the same number.
const ScanHotSpan = 16

// Names are the slot names of a capture scope, which printers declare once.
type Names struct {
	List []string
	// Declared reports that a declareNames call declares them; otherwise their first use does.
	Declared bool
}
