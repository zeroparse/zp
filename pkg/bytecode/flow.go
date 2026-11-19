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
	// FlowAdd sets Dst[0] to the sum of the ints Args[0] and Args[1].
	FlowAdd
	// FlowSub sets Dst[0] to the int Args[0] minus the int Args[1].
	FlowSub
	// FlowEq sets Dst[0] to whether Args[0] and Args[1] are equal.
	FlowEq
	// FlowNe sets Dst[0] to whether Args[0] and Args[1] differ.
	FlowNe
	// FlowLt sets Dst[0] to whether the integer Args[0] (an int or a character) is less than Args[1].
	FlowLt
	// FlowLe sets Dst[0] to whether the integer Args[0] is at most Args[1].
	FlowLe
	// FlowStep charges one step of budget (flow.step).
	FlowStep
	// FlowSilent enters or leaves the lookahead of the immediate FlowSilenceOf (flow.silent).
	FlowSilent
	// FlowAdvance moves the position forward by the int Args[0] (flow.advance).
	FlowAdvance
	// FlowInClass sets Dst[0] to whether the character Args[0] is in the class of the immediate, an
	// index of Module.Classes (flow.inClass).
	FlowInClass
	// FlowAtAnchor sets Dst[0] to whether the position is at the anchor of the immediate, an int: 0
	// the beginning of the input, 1 its end, 2 the beginning of a line, 3 the end of a line
	// (flow.atAnchor).
	FlowAtAnchor
	// FlowMatchLit matches the literal of the immediate FlowLitOf and sets Dst[0] to the null value
	// and Dst[1] to whether it matched (flow.matchLit).
	FlowMatchLit
	// FlowIsolate begins an expectation scope at the position and sets Dst[0] to its mark
	// (flow.isolate).
	FlowIsolate
	// FlowUnisolate ends the expectation scope Args[0] and sets Dst[0] to the farthest failure inside
	// it and Dst[1] to its expectations (flow.unisolate).
	FlowUnisolate
	// FlowMergeExpected merges the expectations Args[1], recorded at the int Args[0], into the
	// enclosing scope (flow.mergeExpected).
	FlowMergeExpected
	// FlowNewTerminal sets Dst[0] to a terminal node from the int Args[0] to the position with the
	// text Args[1] (flow.newTerminalNode).
	FlowNewTerminal
	// FlowNewSeq sets Dst[0] to a sequence node from the int Args[0] to the position with the children
	// Args[1] (flow.newBranchNode).
	FlowNewSeq
	// FlowNewList is FlowNewSeq for a list node (flow.newBranchNode of a list).
	FlowNewList
	// FlowNewVals sets Dst[0] to an empty list of values with room for the immediate, an int
	// (flow.newVals).
	FlowNewVals
	// FlowAppend sets Dst[0] to the list Args[0] with the value Args[1] appended (flow.append).
	FlowAppend
	// FlowSpread sets Dst[0] to the list Args[0] with the children of the node Args[1] appended
	// (flow.append of a spread value).
	FlowSpread
	// FlowText sets Dst[0] to the text of the input from the int Args[0] to the position (flow.text).
	FlowText
	// FlowLiteralText sets Dst[0] to the text of the literal of the immediate, a string index, matched
	// from the int Args[0] (flow.literalText).
	FlowLiteralText
	// FlowStr sets Dst[0] to the string of the immediate, a string index (flow.str).
	FlowStr
	// FlowKidsLen sets Dst[0] to the height of the stack of gathered values (flow.kidsLen).
	FlowKidsLen
	// FlowKids sets Dst[0] to the values gathered above the int Args[0], which it takes off the stack
	// (flow.kids).
	FlowKids
	// FlowDropKids truncates the stack of gathered values to the int Args[0] (flow.dropKids).
	FlowDropKids
	// FlowPushKid pushes the value Args[0] on the stack of gathered values (flow.pushKid).
	FlowPushKid
	// FlowShapeSingle sets Dst[0] to the value of Args[0], matched from the int Args[1] by an
	// expression of the shape of the immediate FlowShapeOf, and Dst[1] to whether it has one
	// (flow.shapeSingle).
	FlowShapeSingle
	// FlowScopeValue sets Dst[0] to the value of a scope whose body, of the shape of the immediate
	// FlowShapeOf, matched Args[0] from the int Args[1] (flow.scopeValue).
	FlowScopeValue
	// FlowSetCapture sets the capture slot of the immediate, a slot, of the current frame to the
	// value Args[0], recording the old value on the capture trail (flow.setCapture).
	FlowSetCapture
	// FlowMarkPos sets Dst[0] to the position of the mark Args[0] (flow.markPos).
	FlowMarkPos
	// FlowFrame sets Dst[0] to the current capture frame (flow.frame).
	FlowFrame
	// FlowNewFrame sets Dst[0] to a new capture frame with a slot for each name of the scope of the
	// immediate (flow.newFrame).
	FlowNewFrame
	// FlowSetFrame makes the frame Args[0] the current capture frame (flow.setFrame).
	FlowSetFrame
	// FlowClearVals sets the values of the frame Args[0] to nil (flow.clearVals).
	FlowClearVals
	// FlowFrameVal sets Dst[0] to the value of the slot of the immediate of the frame Args[0]
	// (flow.frameVal).
	FlowFrameVal
	// FlowAttachFrame sets Dst[0] to the value Args[0], matched from the int Args[1], with the values
	// of the frame Args[2] attached as the fields named by the scope of the immediate
	// (flow.attachFrame).
	FlowAttachFrame
	// FlowShapeCapture sets Dst[0] to the value of Args[0], matched from the int Args[1] by a
	// captured expression of the shape of the immediate FlowShapeOf: as FlowShapeSingle, or the unit
	// node of the range matched where the expression has no value (flow.shapeCapture).
	FlowShapeCapture
	// FlowRunHot sets Dst[0] to whether the walks of the run site of the immediate keep records
	// (flow.runHot).
	FlowRunHot
	// FlowRunBegin begins a walk of the hot run site of the immediate FlowRunWalkOf at the position
	// (flow.runBegin).
	FlowRunBegin
	// FlowRunWalked ends the walk of the run site of the immediate that was not hot and began at the
	// int Args[0] (flow.runWalked).
	FlowRunWalked
	// FlowSkipVisit ends the iteration of the walk before, if any, and begins one at the position
	// (flow.skipVisit).
	FlowSkipVisit
	// FlowSkipHit sets Dst[0] to whether the walk takes the rest from a record of the position, which
	// it then does (flow.skipHit).
	FlowSkipHit
	// FlowSkipValue adds the value Args[0] of the iteration that matched to the walk (flow.skipValue).
	FlowSkipValue
	// FlowSkipEnd ends the walk, recording the rest of it, and drops the values it holds
	// (flow.skipEnd).
	FlowSkipEnd
	// FlowSkipKids ends a walk that builds a value as FlowSkipEnd does and sets Dst[0] to the list of
	// the values of its iterations (flow.skipKids).
	FlowSkipKids
	// FlowScanReach sets Dst[0] to the count of a walk of the scan site of the immediate that began at
	// the int Args[0] and has matched the int Args[1] characters, taking the rest from a record of its
	// start where the site is hot and has one (flow.scanReach).
	FlowScanReach
	// FlowScanEnd ends a value-free walk of the scan site of the immediate that began at the int
	// Args[0] and matched the int Args[1] characters (flow.scanEnd).
	FlowScanEnd
	// FlowScanKids sets Dst[0] to the elements of a walk of the scan site of the immediate that began
	// at the int Args[0], gathered on the stack from the int Args[1], which matched the int Args[2]
	// characters (flow.scanKids).
	FlowScanKids
	// FlowPred sets Dst[0] to whether the predicate whose term is the immediate holds in the current
	// frame, as the match code's PRED evaluates it (flow.pred).
	FlowPred
	// FlowBind binds the variable of the immediate FlowBindOf to the value of its term, evaluated in
	// the current frame as the match code's BIND evaluates it, and sets Dst[0] to whether the
	// evaluation succeeded (flow.bind).
	FlowBind
	// FlowCut sets Dst[0] to the cut flag of the rule call the body runs in, as the match code's CUT
	// sets it (the register flow.Cut).
	FlowCut
	// FlowSetCut sets the cut flag of the rule call the body runs in to the bool Args[0].
	FlowSetCut
	// FlowKeep sets Dst[0] to the expectations Args[0], kept beyond the scope that recorded them, as
	// the match code keeps those of a failed recovery body (flow.keep).
	FlowKeep
	// FlowMakeError sets Dst[0] to the syntax error at the int Args[0] with the expectations Args[1]
	// (flow.makeError).
	FlowMakeError
	// FlowAppendError appends the syntax error Args[0] to the recovered errors (flow.appendError).
	FlowAppendError
	// FlowNewErrorNode sets Dst[0] to the error node of the syntax error Args[1] from the int Args[0]
	// to the position, as the match code's ENDSKIP builds it (flow.newErrorNode).
	FlowNewErrorNode
	// FlowRecLen sets Dst[0] to the number of recovered errors (flow.recLen).
	FlowRecLen
	// FlowRestore sets the position to the int Args[0] and drops the recovered errors after the first
	// int Args[1] (flow.restore).
	FlowRestore
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
	// FlowImmClass is a class: an int index of Module.Classes.
	FlowImmClass
	// FlowImmAnchor is an anchor: an int from 0 to 3.
	FlowImmAnchor
	// FlowImmLit is a FlowLitOf.
	FlowImmLit
	// FlowImmSilence is a FlowSilenceOf.
	FlowImmSilence
	// FlowImmCount is a count: an int that is not negative.
	FlowImmCount
	// FlowImmStr is a string: an int index of Module.Strings.
	FlowImmStr
	// FlowImmShape is a FlowShapeOf.
	FlowImmShape
	// FlowImmSlot is a capture slot: an int that is not negative, below the number of slots of the
	// largest frame the body may run with (its rule's scope or a scope of its FlowNewFrame).
	FlowImmSlot
	// FlowImmScope is a scope: an int index of Module.Scopes.
	FlowImmScope
	// FlowImmRunSite is a skip-run or run site, numbered as the match code numbers it: an int from 1
	// to the number of instructions of the match code.
	FlowImmRunSite
	// FlowImmRunWalk is a FlowRunWalkOf.
	FlowImmRunWalk
	// FlowImmScanSite is a scan site, numbered as the match code numbers it: an int from 1 to the
	// largest scan site that the match code can name.
	FlowImmScanSite
	// FlowImmTerm is a term: an int index of Module.Terms.
	FlowImmTerm
	// FlowImmBind is a FlowBindOf.
	FlowImmBind
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
	FlowConst:         {Name: "CONST", Dst: 1, Imm: FlowImmConst},
	FlowMove:          {Name: "MOVE", Dst: 1, Args: 1},
	FlowNot:           {Name: "NOT", Dst: 1, Args: 1},
	FlowPos:           {Name: "POS", Dst: 1},
	FlowSetPos:        {Name: "SETPOS", Args: 1},
	FlowJump:          {Name: "JUMP", Target: true},
	FlowJumpFalse:     {Name: "JUMPFALSE", Args: 1, Target: true},
	FlowReturn:        {Name: "RETURN", Args: 2},
	FlowMark:          {Name: "MARK", Dst: 1},
	FlowReset:         {Name: "RESET", Args: 1},
	FlowPeek:          {Name: "PEEK", Dst: 3},
	FlowGuard:         {Name: "GUARD", Dst: 1, Args: 2, Imm: FlowImmGuard},
	FlowExpect:        {Name: "EXPECT", Args: 1, Imm: FlowImmExpect},
	FlowUnfresh:       {Name: "UNFRESH", Args: 1},
	FlowCall:          {Name: "CALL", Dst: 2, Imm: FlowImmCall},
	FlowAdd:           {Name: "ADD", Dst: 1, Args: 2},
	FlowSub:           {Name: "SUB", Dst: 1, Args: 2},
	FlowEq:            {Name: "EQ", Dst: 1, Args: 2},
	FlowNe:            {Name: "NE", Dst: 1, Args: 2},
	FlowLt:            {Name: "LT", Dst: 1, Args: 2},
	FlowLe:            {Name: "LE", Dst: 1, Args: 2},
	FlowStep:          {Name: "STEP"},
	FlowSilent:        {Name: "SILENT", Imm: FlowImmSilence},
	FlowAdvance:       {Name: "ADVANCE", Args: 1},
	FlowInClass:       {Name: "INCLASS", Dst: 1, Args: 1, Imm: FlowImmClass},
	FlowAtAnchor:      {Name: "ATANCHOR", Dst: 1, Imm: FlowImmAnchor},
	FlowMatchLit:      {Name: "MATCHLIT", Dst: 2, Imm: FlowImmLit},
	FlowIsolate:       {Name: "ISOLATE", Dst: 1},
	FlowUnisolate:     {Name: "UNISOLATE", Dst: 2, Args: 1},
	FlowMergeExpected: {Name: "MERGEEXPECTED", Args: 2},
	FlowNewTerminal:   {Name: "NEWTERMINAL", Dst: 1, Args: 2},
	FlowNewSeq:        {Name: "NEWSEQ", Dst: 1, Args: 2},
	FlowNewList:       {Name: "NEWLIST", Dst: 1, Args: 2},
	FlowNewVals:       {Name: "NEWVALS", Dst: 1, Imm: FlowImmCount},
	FlowAppend:        {Name: "APPEND", Dst: 1, Args: 2},
	FlowSpread:        {Name: "SPREAD", Dst: 1, Args: 2},
	FlowText:          {Name: "TEXT", Dst: 1, Args: 1},
	FlowLiteralText:   {Name: "LITERALTEXT", Dst: 1, Args: 1, Imm: FlowImmStr},
	FlowStr:           {Name: "STR", Dst: 1, Imm: FlowImmStr},
	FlowKidsLen:       {Name: "KIDSLEN", Dst: 1},
	FlowKids:          {Name: "KIDS", Dst: 1, Args: 1},
	FlowDropKids:      {Name: "DROPKIDS", Args: 1},
	FlowPushKid:       {Name: "PUSHKID", Args: 1},
	FlowShapeSingle:   {Name: "SHAPESINGLE", Dst: 2, Args: 2, Imm: FlowImmShape},
	FlowScopeValue:    {Name: "SCOPEVALUE", Dst: 1, Args: 2, Imm: FlowImmShape},
	FlowSetCapture:    {Name: "SETCAPTURE", Args: 1, Imm: FlowImmSlot},
	FlowMarkPos:       {Name: "MARKPOS", Dst: 1, Args: 1},
	FlowFrame:         {Name: "FRAME", Dst: 1},
	FlowNewFrame:      {Name: "NEWFRAME", Dst: 1, Imm: FlowImmScope},
	FlowSetFrame:      {Name: "SETFRAME", Args: 1},
	FlowClearVals:     {Name: "CLEARVALS", Args: 1},
	FlowFrameVal:      {Name: "FRAMEVAL", Dst: 1, Args: 1, Imm: FlowImmSlot},
	FlowAttachFrame:   {Name: "ATTACHFRAME", Dst: 1, Args: 3, Imm: FlowImmScope},
	FlowShapeCapture:  {Name: "SHAPECAPTURE", Dst: 1, Args: 2, Imm: FlowImmShape},
	FlowRunHot:        {Name: "RUNHOT", Dst: 1, Imm: FlowImmRunSite},
	FlowRunBegin:      {Name: "RUNBEGIN", Imm: FlowImmRunWalk},
	FlowRunWalked:     {Name: "RUNWALKED", Args: 1, Imm: FlowImmRunSite},
	FlowSkipVisit:     {Name: "SKIPVISIT"},
	FlowSkipHit:       {Name: "SKIPHIT", Dst: 1},
	FlowSkipValue:     {Name: "SKIPVALUE", Args: 1},
	FlowSkipEnd:       {Name: "SKIPEND"},
	FlowSkipKids:      {Name: "SKIPKIDS", Dst: 1},
	FlowScanReach:     {Name: "SCANREACH", Dst: 1, Args: 2, Imm: FlowImmScanSite},
	FlowScanEnd:       {Name: "SCANEND", Args: 2, Imm: FlowImmScanSite},
	FlowScanKids:      {Name: "SCANKIDS", Dst: 1, Args: 3, Imm: FlowImmScanSite},
	FlowPred:          {Name: "PRED", Dst: 1, Imm: FlowImmTerm},
	FlowBind:          {Name: "BIND", Dst: 1, Imm: FlowImmBind},
	FlowCut:           {Name: "CUT", Dst: 1},
	FlowSetCut:        {Name: "SETCUT", Args: 1},
	FlowKeep:          {Name: "KEEP", Dst: 1, Args: 1},
	FlowMakeError:     {Name: "MAKEERROR", Dst: 1, Args: 2},
	FlowAppendError:   {Name: "APPENDERROR", Args: 1},
	FlowNewErrorNode:  {Name: "NEWERRORNODE", Dst: 1, Args: 2},
	FlowRecLen:        {Name: "RECLEN", Dst: 1},
	FlowRestore:       {Name: "RESTORE", Args: 2},
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

// FlowLitOf is the immediate of FlowMatchLit: the literal (an index of Module.Strings; for a
// case-insensitive literal, Fold, its text with each ASCII letter lowercased) and the expectation it
// records on failure, as FlowExpectOf.Exp counts it.
type FlowLitOf struct {
	Str  int
	Exp  int
	Fold bool
}

// FlowSilenceOf is the immediate of FlowSilent: Delta 1 enters a lookahead and -1 leaves it, whose
// expression matched if Ok. Site is the lookahead's site in the parser's observation of lookaheads,
// or -1 for a lookahead that is no observation.
type FlowSilenceOf struct {
	Delta int
	Site  int
	Ok    bool
}

// FlowShapeOf is the immediate of FlowShapeSingle, FlowShapeCapture and FlowScopeValue, the shape of an expression's
// value: Elems if the value is the tuple node of its elements, Shared if that node is shared through
// a splice rule's memo entry, K the number of valued elements, Mode how the value where it takes one
// element is made (0 no value, 1 the element itself, 2 the tuple node of the elements), and Tuple if
// the value of a scope with this body is the tuple node.
type FlowShapeOf struct {
	Elems, Shared, Tuple bool
	K, Mode              int
}

// FlowRunWalkOf is the immediate of FlowRunBegin: the run site, whether the iterations of the walk
// have values (FlowSkipValue), which FlowSkipKids gathers in a list, and whether its records are
// keyed by the values of the variables of the site's run key (Module.RunKeys) besides the position.
type FlowRunWalkOf struct {
	Site          int
	Build, KeyEnv bool
}

// FlowBindOf is the immediate of FlowBind: the variable, an index of Module.Vars, and its term, an
// index of Module.Terms.
type FlowBindOf struct {
	Var, Term int
}

// FlowCallOf is the immediate of FlowCall: the rule's index and the binding level it is called at.
type FlowCallOf struct{ Rule, Level int }

// FixedExps is the number of fixed expectations at the start of the parser's expectation table (any
// character and the four anchors), which the expectation IDs of FlowExpectOf count before the
// strings of the module.
const FixedExps = 5
