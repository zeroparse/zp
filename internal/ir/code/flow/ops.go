package flow

import (
	"reflect"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/core"
)

// The Flow dialect of Code IR.
//
// Grammar code at the Flow stage is a module of Code IR whose functions call the operations that this
// file declares: matching the input, backtracking to a mark, recording expectations, calling rules,
// building values, evaluating action expressions and charging the budget. The structure (blocks,
// loops, branches, exits, assignments and the arithmetic of conditions) is the base's. Each operation
// has a contract, a cost bound, the features it exists in, laws that passes may use, and an
// immediate operand of a declared Go type that holds its static data (a literal, a class, a rule);
// its dynamic operands are arguments. execlower (internal/middleend/execlower) converts each call
// into the operation's reference expansion at the Exec stage.

// Dialect is the name of the Flow dialect, the Dialect of its operations.
const Dialect = "flow"

// State is the component of the Flow operations: the parser state that grammar code reads and
// writes, as one component. The reference expansion of each operation names the Exec components
// it acts on.
const State = "parser"

// Registers of the Flow stage.
const (
	// Pos is the position.
	Pos = "pos"
	// Cut is the cut flag of the scope the runtime entered last.
	Cut = "cut"
)

// Quantities of the cost bounds of Flow operations.
const (
	// QLit is the number of code points of the operation's literal.
	QLit = "lit"
	// QTerminal is the number of steps of the operation's terminal program.
	QTerminal = "terminal"
	// QClasses is the number of classes of the operation's class run.
	QClasses = "classes"
	// QBody is the units the body of a called rule, outlined node or action expression charges.
	QBody = "body"
	// QText is the number of code points of the input a value copies.
	QText = "text"
)

// Op names of the Flow dialect.
const (
	OpStep            = "flow.step"
	OpSilent          = "flow.silent"
	OpPeek            = "flow.peek"
	OpAdvance         = "flow.advance"
	OpExpect          = "flow.expect"
	OpObserveSkip     = "flow.observeSkip"
	OpRestore         = "flow.restore"
	OpMergeExpected   = "flow.mergeExpected"
	OpMergeRenamed    = "flow.mergeRenamed"
	OpPushKid         = "flow.pushKid"
	OpDropKids        = "flow.dropKids"
	OpStreamStart     = "flow.streamStart"
	OpSkipBegin       = "flow.skipBegin"
	OpRunBegin        = "flow.runBegin"
	OpSkipValue       = "flow.skipValue"
	OpRunWalked       = "flow.runWalked"
	OpSkipVisit       = "flow.skipVisit"
	OpSkipEnd         = "flow.skipEnd"
	OpScanEnd         = "flow.scanEnd"
	OpEmitStream      = "flow.emitStream"
	OpAppend          = "flow.append"
	OpSetWhere        = "flow.setWhere"
	OpDropCreated     = "flow.dropCreated"
	OpUnfresh         = "flow.unfresh"
	OpReset           = "flow.reset"
	OpSetCapture      = "flow.setCapture"
	OpClearVals       = "flow.clearVals"
	OpDropTrail       = "flow.dropTrail"
	OpFreeFrame       = "flow.freeFrame"
	OpAppendError     = "flow.appendError"
	OpDeclareNames    = "flow.declareNames"
	OpEnv             = "flow.env"
	OpSetEnv          = "flow.setEnv"
	OpFrame           = "flow.frame"
	OpSetFrame        = "flow.setFrame"
	OpRecLen          = "flow.recLen"
	OpKidsLen         = "flow.kidsLen"
	OpStreamOn        = "flow.streamOn"
	OpSkipHit         = "flow.skipHit"
	OpRunHot          = "flow.runHot"
	OpSkipKids        = "flow.skipKids"
	OpScanReach       = "flow.scanReach"
	OpScanKids        = "flow.scanKids"
	OpHasLit          = "flow.hasLit"
	OpMatchLit        = "flow.matchLit"
	OpMatchClass      = "flow.matchClass"
	OpInClass         = "flow.inClass"
	OpGuardFails      = "flow.guardFails"
	OpAtAnchor        = "flow.atAnchor"
	OpSkipTerminal    = "flow.skipTerminal"
	OpSkipClasses     = "flow.skipClasses"
	OpCallRule        = "flow.callRule"
	OpCallOutline     = "flow.callOutline"
	OpIsolate         = "flow.isolate"
	OpUnisolate       = "flow.unisolate"
	OpText            = "flow.text"
	OpLiteralText     = "flow.literalText"
	OpStr             = "flow.str"
	OpNewMatch        = "flow.newMatch"
	OpNewNode         = "flow.newNode"
	OpNewTerminalNode = "flow.newTerminalNode"
	OpNewBranchNode   = "flow.newBranchNode"
	OpNewVals         = "flow.newVals"
	OpKids            = "flow.kids"
	OpShapeSingle     = "flow.shapeSingle"
	OpShapeCapture    = "flow.shapeCapture"
	OpScopeValue      = "flow.scopeValue"
	OpAttachCaptures  = "flow.attachCaptures"
	OpAttachFrame     = "flow.attachFrame"
	OpMarkState       = "flow.markState"
	OpMarkPos         = "flow.markPos"
	OpNewFrame        = "flow.newFrame"
	OpFrameVal        = "flow.frameVal"
	OpKeep            = "flow.keep"
	OpMakeError       = "flow.makeError"
	OpErrorValue      = "flow.errorValue"
	OpNewErrorNode    = "flow.newErrorNode"
	OpFreshNode       = "flow.freshNode"
	OpConvertResult   = "flow.convertResult"
	OpUseCtx          = "flow.useCtx"
	OpPred            = "flow.pred"
	OpBind            = "flow.bind"
	OpEval            = "flow.eval"
	OpCtxFinish       = "flow.ctxFinish"
)

// Immediate operands of Flow operations.

// Silence is the immediate of flow.silent: the change Delta of the silent depth, and for a
// lookahead (Site is not 0) its site and, at its end, whether its body matched.
type Silence struct {
	Delta int
	Site  int
	Ok    bool
}

// Site is the immediate of the operations of skip-run and scan sites: the site's number.
type Site int

// RunWalk is the immediate of flow.runBegin: the run site, whether its iterations have values
// (flow.skipValue), which flow.skipKids gathers in a list, and whether its records are keyed by the
// values of the variables of the site's run key (program.RunKey), which the printed parser holds; a
// site free of the environment (core.Repeat.RunEnvFree) is keyed by the position alone.
type RunWalk struct {
	Site          int
	Build, KeyEnv bool
}

// Slot is the immediate of flow.setCapture and flow.frameVal: a capture slot of a frame.
type Slot int

// Expectation is the immediate of flow.expect: the expectation, and whether it is the message of a
// failed @error.
type Expectation struct {
	Exp     core.ExpID
	Message bool
}

// Spread is the immediate of flow.append: whether the children of a tuple node are appended
// instead of the node.
type Spread bool

// RuleCall is the immediate of flow.callRule: the rule and the Operators Rule level it is called
// from.
type RuleCall struct{ Rule, Level int }

// Outline is the immediate of flow.callOutline: the node that a function of its own lowers, the
// slots of the capture scope of the current frame, and whether the call builds the node's value.
type Outline struct {
	Node  core.Node
	Slots []core.Slot
	Build bool
}

// Chars is the immediate of flow.literalText, flow.str and flow.setWhere: a string.
type Chars string

// List is the immediate of flow.newNode and flow.newBranchNode: whether the node is a list.
type List bool

// Room is the immediate of flow.newVals: the room of the new list.
type Room int

// Attach is the immediate of flow.attachCaptures and flow.attachFrame: the slot names of the scope,
// and whether the captures are the values of a frame (flow.attachFrame) instead of variables.
type Attach struct {
	Names *Names
	Frame bool
}

// StateMark is the immediate of flow.markState: which parts of the state besides the position the
// code between the mark and its reset may change.
type StateMark struct{ Env, Trail, Rec bool }

// Conversion is the immediate of flow.convertResult: the rule, and whether the fresh node of the
// body is passed to convert in place.
type Conversion struct {
	Rule  int
	Owned bool
}

// Capture names the captures of an action expression: the slot names of the scope whose variables
// a call passes, or Frame when the expression reads the current frame and the call passes none.
type Capture struct {
	Names []string
	Frame bool
}

// Predicate is the immediate of flow.pred.
type Predicate struct {
	Node *core.Pred
	Capture
}

// Binding is the immediate of flow.bind.
type Binding struct {
	Node *core.BindNode
	Capture
}

// Action is the immediate of flow.eval: the action of the function's rule.
type Action struct {
	Term core.ActionExpr
	Capture
}

// Handle types of the Flow stage: the types of variables besides int and bool.
const (
	TValue   = "value"
	TEnv     = "env"
	TExpMark = "expmark"
	TExpIDs  = "expids"
	TVals    = "vals"
	TCtx     = "ctx"
	TMark    = "mark"
	TFrame   = "frame"
	TError   = "error"
	TRune    = "rune"
)

// HandleTypes are the handle types of the Flow stage, in the order Declare declares them.
var HandleTypes = []string{TValue, TEnv, TExpMark, TExpIDs, TVals, TCtx, TMark, TFrame, TError, TRune}

// NewModule returns an empty module at the Flow stage with the declarations of the Flow dialect.
func NewModule(name string) *code.Module {
	m := code.NewModule(name)
	m.Stage, m.Dialects = Stage, []string{Dialect, "action"}
	Declare(m)
	return m
}

// Declare adds the declarations of the Flow dialect to m: its component, registers, handle types and
// operations.
func Declare(m *code.Module) {
	m.AddComponent(State, "The parser state that grammar code reads and writes.")
	m.AddRegister(Pos, code.TInt, "The position.")
	m.AddRegister(Cut, code.TBool, "The cut flag of the scope the runtime entered last.")
	for _, h := range HandleTypes {
		m.AddHandle(h, State, "A "+h+" of the parser.")
	}
	for _, o := range Ops() {
		m.AddOp(o)
	}
}

func immOf[T any]() reflect.Type { return reflect.TypeFor[T]() }

// Ops returns new declarations of the operations of the Flow dialect.
func Ops() []*code.Op {
	h := code.THandle
	value, env, expmark, expids, vals, ctx := h(TValue), h(TEnv), h(TExpMark), h(TExpIDs), h(TVals), h(TCtx)
	mark, frame, errT, runeT := h(TMark), h(TFrame), h(TError), h(TRune)
	b, i := code.TBool, code.TInt
	P := code.P
	all := []string{State, Pos, Cut}
	pos := []string{State, Pos}
	unit := code.Cost{Const: 1}
	per := func(q string) code.Cost { return code.Cost{Terms: []code.CostTerm{{Coef: 1, Quantity: q}}} }
	pure := "pure: the operation reads the state and changes nothing, so a call whose result is unused may be removed."
	var ops []*code.Op
	op := func(o *code.Op) *code.Op {
		o.Dialect, o.Component = Dialect, State
		if o.Reads == nil {
			o.Reads = []string{State}
		}
		ops = append(ops, o)
		return o
	}
	withImm := func(o *code.Op, doc string, t reflect.Type) *code.Op {
		o.Imm, o.ImmType = doc, t
		return o
	}

	// Budget and input.
	op(&code.Op{Name: OpStep, Writes: []string{State}, NoReturn: true, Cost: unit,
		Contract: "Charges one step of budget; ends the parse when the budget is exhausted."})
	withImm(op(&code.Op{Name: OpSilent, Writes: []string{State}, NoReturn: true,
		Contract: `Adds Delta to the silent depth, inside which expectations are not recorded. For a lookahead
(Site is not 0) it begins the lookahead of the site after it enters the depth, or ends it, whose body
matched if Ok, before it leaves.`}), "the change of the depth and the lookahead (Silence)", immOf[Silence]())
	op(&code.Op{Name: OpPeek, Results: []code.Type{runeT, i, b}, Reads: pos, Writes: []string{State}, NoReturn: true, Cost: unit,
		Contract: `Returns the code point at the position, its size and whether there is one, without consuming it.
The read charges one unit in a function with budget controls and, in a Document parse, records how far the
parse examined the input.`})
	op(&code.Op{Name: OpAdvance, Params: []code.Param{P("n", i)}, Reads: pos, Writes: []string{Pos},
		Contract: "Moves the position forward by n.", Requires: "n code units of input follow the position."})
	withImm(op(&code.Op{Name: OpExpect, Params: []code.Param{P("at", i)}, Writes: []string{State},
		Contract: "Records the expectation at the position at, unless the silent depth is positive."}),
		"the expectation (Expectation)", immOf[Expectation]())
	withImm(op(&code.Op{Name: OpObserveSkip, Writes: []string{State}, NoReturn: true,
		Contract: "Reports to the observation of the parse that the guard of a choice skips the alternative."}),
		"the alternative (core.Node)", immOf[core.Node]())
	op(&code.Op{Name: OpRestore, Params: []code.Param{P("pos", i), P("rec", i)}, Writes: []string{State, Pos},
		Contract: "Moves the position back to pos and drops the recovered errors after the first rec."})
	op(&code.Op{Name: OpMergeExpected, Params: []code.Param{P("far", i), P("exps", expids)}, Writes: []string{State},
		Contract: "Merges the expectations exps, recorded at far, into the enclosing expectation scope."})
	withImm(op(&code.Op{Name: OpMergeRenamed, Params: []code.Param{P("far", i), P("exps", expids)}, Writes: []string{State},
		Contract: "Merges the expectations exps, recorded at far, into the enclosing scope, renamed to the expectation of the immediate (@diagnostic)."}),
		"the expectation (core.ExpID)", immOf[core.ExpID]())
	op(&code.Op{Name: OpPushKid, Params: []code.Param{P("x", value)}, Writes: []string{State},
		Contract: "Pushes x on the stack of gathered values."})
	op(&code.Op{Name: OpDropKids, Params: []code.Param{P("base", i)}, Writes: []string{State},
		Contract: "Truncates the stack of gathered values to base.", Requires: "base is at most the height of the stack."})
	op(&code.Op{Name: OpStreamStart, Writes: []string{State}, NoReturn: true,
		Contract: "Begins the stream of a stream parse: the repetition that delivers its elements is about to run.",
		Requires: "flow.streamOn holds."})
	withImm(op(&code.Op{Name: OpSkipBegin, Writes: []string{State},
		Contract: "Begins a walk of the skip-run site at the position: the iterations that follow are its visits, until flow.skipEnd."}),
		"the site (Site)", immOf[Site]())
	withImm(op(&code.Op{Name: OpRunBegin, Writes: []string{State},
		Contract: `Begins a walk of the hot run site at the position, as flow.skipBegin does for a skip-run site: the
iterations that follow are its visits, until flow.skipEnd or flow.skipKids. With Build its iterations have
values, which flow.skipValue adds and flow.skipKids gathers in a list; with KeyEnv its records are kept for
the position and the values of the variables of the site's key, otherwise for the position alone.`,
		Requires: "flow.runHot of the site holds."}),
		"the walk (RunWalk)", immOf[RunWalk]())
	op(&code.Op{Name: OpSkipValue, Params: []code.Param{P("x", value)}, Writes: []string{State},
		Contract: `Adds the value x of the iteration that matched to the walk begun with Build, and marks x, if it is a
node, as no longer owned by its maker: the records of the walk share it.`,
		Requires: "the walk in progress was begun with Build."})
	op(&code.Op{Name: OpSkipVisit, Reads: pos, Writes: []string{State},
		Contract: "Ends the iteration of the walk before, if any, and begins one at the position."})
	op(&code.Op{Name: OpSkipEnd, Reads: pos, Writes: []string{State},
		Contract: "Ends the iteration of the walk before, if any, ends the walk and records the rest of it from each position it visited."})
	withImm(op(&code.Op{Name: OpRunWalked, Params: []code.Param{P("start", i)}, Reads: pos, Writes: []string{State},
		Contract: `Ends the walk of the run site that was not hot (flow.runHot), which began at start and ends at the
position: a walk that began below the furthest position that the walks of the site passed, and walked
RunHotSpan or more of the input below it, turns the site hot; the furthest position then moves to the
position. A walk shorter than RunHotSpan changes nothing.`,
		Requires: "start is at most the position."}), "the site (Site)", immOf[Site]())
	withImm(op(&code.Op{Name: OpScanEnd, Params: []code.Param{P("start", i), P("count", i)}, Reads: pos, Writes: []string{State},
		Contract: `Ends a walk of the value-free scan site that began at start and matched count characters, at least
ScanHotSpan: unless the walk took the rest from a record, it records itself at a hot site, or notes that it
walked at a site that is not hot.`}), "the site (Site)", immOf[Site]())
	op(&code.Op{Name: OpEmitStream, Params: []code.Param{P("x", value)}, Reads: pos, Writes: []string{State}, NoReturn: true,
		Contract: "Delivers the committed element x (nil for a value-free repetition) to the callback of a stream parse, which has consumed the input up to the position."})
	withImm(op(&code.Op{Name: OpAppend, Params: []code.Param{P("vals", vals), P("x", value)}, Results: []code.Type{vals},
		Contract: "Returns the list vals with x appended; with Spread, the children of the tuple node x."}),
		"whether the children are appended (Spread)", immOf[Spread]())
	withImm(op(&code.Op{Name: OpSetWhere, Writes: []string{State},
		Contract: "Names the rule whose action runs, for the messages of its errors."}), "the rule's name (Chars)", immOf[Chars]())
	op(&code.Op{Name: OpDropCreated, Params: []code.Param{P("c", ctx)}, Writes: []string{State},
		Contract: "Drops the structs made since the context c began."})
	op(&code.Op{Name: OpUnfresh, Params: []code.Param{P("x", value)}, Writes: []string{State},
		Contract: "Marks the node x, if x is one, as no longer owned by its maker."})
	op(&code.Op{Name: OpReset, Params: []code.Param{P("m", mark)}, Writes: []string{State, Pos},
		Contract: "Returns to the parser state of the mark m: the position and the parts that flow.markState named."})
	withImm(op(&code.Op{Name: OpSetCapture, Params: []code.Param{P("x", value)}, Writes: []string{State},
		Contract: "Sets the capture slot of the current frame to x."}), "the slot (Slot)", immOf[Slot]())
	op(&code.Op{Name: OpClearVals, Params: []code.Param{P("f", frame)}, Writes: []string{State},
		Contract: "Sets the values of the frame f to nil."})
	op(&code.Op{Name: OpDropTrail, Params: []code.Param{P("m", mark)}, Writes: []string{State},
		Contract: "Drops the trail entries recorded since the mark m."})
	op(&code.Op{Name: OpFreeFrame, Params: []code.Param{P("f", frame)}, Writes: []string{State},
		Contract: "Returns the frame f to the runtime.", Requires: "f is not used after the call."})
	op(&code.Op{Name: OpAppendError, Params: []code.Param{P("x", errT)}, Writes: []string{State},
		Contract: "Appends the syntax error x to the recovered errors."})
	withImm(op(&code.Op{Name: OpDeclareNames,
		Contract: "Declares the slot names of a frame scope, which framed repetitions name before their element and declare after it. It does nothing to the state."}),
		"the names (*Names)", immOf[*Names]())

	// Registers that hold handles.
	op(&code.Op{Name: OpEnv, Results: []code.Type{env}, Contract: "Returns the variable environment.", Laws: []string{pure}})
	op(&code.Op{Name: OpSetEnv, Params: []code.Param{P("e", env)}, Writes: []string{State}, Contract: "Sets the variable environment to e."})
	op(&code.Op{Name: OpFrame, Results: []code.Type{frame}, Contract: "Returns the current capture frame.", Laws: []string{pure}})
	op(&code.Op{Name: OpSetFrame, Params: []code.Param{P("f", frame)}, Writes: []string{State}, Contract: "Makes f the current capture frame."})

	// Expressions.
	op(&code.Op{Name: OpRecLen, Results: []code.Type{i}, Contract: "Returns the number of recovered errors.", Laws: []string{pure}})
	op(&code.Op{Name: OpKidsLen, Results: []code.Type{i}, Contract: "Returns the height of the stack of gathered values.", Laws: []string{pure}})
	op(&code.Op{Name: OpStreamOn, Results: []code.Type{b},
		Contract: "Reports that the invocation delivers the elements of its #stream repetition: a stream parse with a callback, at the top call of its start rule.", Laws: []string{pure}})
	op(&code.Op{Name: OpSkipHit, Results: []code.Type{b}, Reads: pos, Writes: []string{State, Pos},
		Contract: `Ends the iteration of the walk before, if any, and reports whether the walk has a record of the
position; it then takes the rest of the walk from the record, which moves the position to its end, and the
walk stops.`})
	withImm(op(&code.Op{Name: OpRunHot, Results: []code.Type{b}, Laws: []string{pure},
		Contract: `Reports whether the walks of the run site keep records, separately inside and outside lookaheads. A
walk of a site that is not hot is an ordinary repetition, which flow.runWalked ends.`}), "the site (Site)", immOf[Site]())
	op(&code.Op{Name: OpSkipKids, Results: []code.Type{vals}, Reads: pos, Writes: []string{State},
		Contract: `Ends the walk begun with Build as flow.skipEnd does, and returns the child list of the values of its
iterations, followed by those of the record it took, if any.`})
	withImm(op(&code.Op{Name: OpScanReach, Params: []code.Param{P("start", i), P("count", i)}, Results: []code.Type{i}, Reads: pos, Writes: []string{State, Pos},
		Contract: `For a walk of the scan site that began at start and has matched ScanHotSpan characters, count of
them: at a hot site with a record of start it moves the position to the recorded end and returns the
recorded count; otherwise it returns count.`}), "the site (Site)", immOf[Site]())
	withImm(op(&code.Op{Name: OpScanKids, Params: []code.Param{P("start", i), P("base", i), P("count", i)}, Results: []code.Type{vals}, Reads: pos, Writes: []string{State},
		Contract: `Returns the elements of a walk of the scan site that began at start, gathered on the stack from
base, which matched count characters: for a walk that reached ScanHotSpan characters it ends the walk as
flow.scanEnd does, and the result is the elements of the record that the walk took, if it took one (the
stack is dropped to base), or the gathered elements, which the record that it makes shares.`}), "the site (Site)", immOf[Site]())
	withImm(op(&code.Op{Name: OpHasLit, Results: []code.Type{b}, Reads: pos, Writes: []string{State}, NoReturn: true, Cost: per(QLit),
		Contract: `Reports that the input at the position begins with the literal, read in place without recording
anything; false where the input cannot be read in place. With budget controls a match charges a unit per
code point.`, Laws: []string{"implies flow.matchLit: where it holds, flow.matchLit of the literal matches."}}),
		"the literal (*core.Lit)", immOf[*core.Lit]())
	withImm(op(&code.Op{Name: OpMatchLit, Results: []code.Type{value, b}, Reads: pos, Writes: []string{State, Pos}, NoReturn: true, Cost: per(QLit),
		Contract: "Matches the literal at the position and moves past it, recording its expectation on failure. Its results are its value (not used) and whether it matched; on failure the position is unchanged."}),
		"the literal (*core.Lit)", immOf[*core.Lit]())
	withImm(op(&code.Op{Name: OpMatchClass, Results: []code.Type{b}, Reads: pos, Writes: []string{State, Pos}, NoReturn: true, Cost: unit,
		Contract: "Matches one character of the class through its membership table and moves past it, recording the class's expectation on failure; reports whether it matched."}),
		"the class (*core.Class)", immOf[*core.Class]())
	withImm(op(&code.Op{Name: OpInClass, Params: []code.Param{P("ch", runeT)}, Results: []code.Type{b},
		Contract: "Reports whether the code point ch is in the class.", Laws: []string{pure}}), "the class (*core.Class)", immOf[*core.Class]())
	withImm(op(&code.Op{Name: OpGuardFails, Params: []code.Param{P("ch", runeT), P("more", b)}, Results: []code.Type{b},
		Contract: "Reports that the guard skips its alternative: the character that more (there is one) and ch describe cannot begin it, and the nesting limit allows the guard's depth of calls.",
		Laws:     []string{pure}}), "the guard (*core.First)", immOf[*core.First]())
	withImm(op(&code.Op{Name: OpAtAnchor, Results: []code.Type{b}, Reads: pos,
		Contract: "Reports whether the position is at the anchor.", Laws: []string{pure}}), "the anchor (core.AnchorKind)", immOf[core.AnchorKind]())
	withImm(op(&code.Op{Name: OpSkipTerminal, Results: []code.Type{b}, Reads: pos, Writes: []string{State, Pos}, NoReturn: true, Cost: per(QTerminal),
		Contract: "Runs the terminal region as one program from the position and reports whether it matched; on a match the position moves past it."}),
		"the terminal region (*core.Terminal)", immOf[*core.Terminal]())
	withImm(op(&code.Op{Name: OpSkipClasses, Results: []code.Type{b}, Reads: pos, Writes: []string{State, Pos}, NoReturn: true, Cost: per(QClasses),
		Contract: "Matches the classes of the run in turn and reports whether they matched; on a match the position moves past them."}),
		"the class run (*core.ClassRun)", immOf[*core.ClassRun]())
	withImm(op(&code.Op{Name: OpCallRule, Results: []code.Type{value, b}, Reads: all, Writes: all, NoReturn: true,
		Cost:     code.Cost{Const: 1, Terms: []code.CostTerm{{Coef: 1, Quantity: QBody}}},
		Contract: "Calls the rule from the Operators Rule level. Its results are the rule's value and whether it matched; on a match the position is the end of the rule's match, on failure it is unchanged."}),
		"the rule and the level (RuleCall)", immOf[RuleCall]())
	withImm(op(&code.Op{Name: OpCallOutline, Results: []code.Type{value, b}, Reads: all, Writes: all, NoReturn: true, Cost: per(QBody),
		Contract: "Runs the node, which a function of its own lowers, under the current frame. Its results are the value of the node, built if Build, and whether it matched."}),
		"the outlined node (Outline)", immOf[Outline]())
	op(&code.Op{Name: OpIsolate, Results: []code.Type{expmark}, Reads: pos, Writes: []string{State},
		Contract: "Begins an expectation scope at the position and returns its mark."})
	op(&code.Op{Name: OpUnisolate, Params: []code.Param{P("m", expmark)}, Results: []code.Type{i, expids}, Writes: []string{State},
		Contract: "Ends the expectation scope m. Its results are the farthest failure inside it and its expectations.",
		Requires: "m is the innermost scope that has not ended."})
	op(&code.Op{Name: OpText, Params: []code.Param{P("start", i)}, Results: []code.Type{value}, Reads: pos,
		Contract: "Returns the input from start to the position.", Laws: []string{pure}})
	withImm(op(&code.Op{Name: OpLiteralText, Params: []code.Param{P("start", i)}, Results: []code.Type{value},
		Contract: "Returns the text of the literal matched from start, which differs from the literal where it holds the replacement character.", Laws: []string{pure}}),
		"the literal's text (Chars)", immOf[Chars]())
	withImm(op(&code.Op{Name: OpStr, Results: []code.Type{value}, Contract: "Returns the string constant.", Laws: []string{pure}}),
		"the string (Chars)", immOf[Chars]())
	op(&code.Op{Name: OpNewMatch, Params: []code.Param{P("start", i), P("text", value)}, Results: []code.Type{value}, Reads: pos, Writes: []string{State},
		Contract: "Returns a match value from start to the position with the text."})
	withImm(op(&code.Op{Name: OpNewNode, Params: []code.Param{P("start", i), P("kids", vals)}, Results: []code.Type{value}, Reads: pos, Writes: []string{State},
		Contract: "Returns a tuple node, or with List a list node, from start to the position with the children kids."}),
		"whether the node is a list (List)", immOf[List]())
	op(&code.Op{Name: OpNewTerminalNode, Params: []code.Param{P("start", i), P("text", value)}, Results: []code.Type{value}, Reads: pos, Writes: []string{State},
		Contract: "Returns a terminal of the Node runtime from start to the position, with the text."})
	withImm(op(&code.Op{Name: OpNewBranchNode, Params: []code.Param{P("start", i), P("kids", vals)}, Results: []code.Type{value}, Reads: pos, Writes: []string{State},
		Contract: "Returns a branch of the Node runtime from start to the position: a sequence, or with List a list, of the children kids."}),
		"whether the node is a list (List)", immOf[List]())
	withImm(op(&code.Op{Name: OpNewVals, Results: []code.Type{vals}, Writes: []string{State},
		Contract: "Returns an empty list of values with room for the immediate."}), "the room (Room)", immOf[Room]())
	op(&code.Op{Name: OpKids, Params: []code.Param{P("base", i)}, Results: []code.Type{vals}, Writes: []string{State},
		Contract: "Returns the list of the values gathered above base, which it removes from the stack."})
	withImm(op(&code.Op{Name: OpShapeSingle, Params: []code.Param{P("x", value), P("start", i)}, Results: []code.Type{value, b}, Reads: pos, Writes: []string{State},
		Contract: "Returns the value of x, matched from start by an expression of the shape, where it takes one element, and whether it is a single element (not used)."}),
		"the shape (core.Shape)", immOf[core.Shape]())
	withImm(op(&code.Op{Name: OpShapeCapture, Params: []code.Param{P("x", value), P("start", i)}, Results: []code.Type{value}, Reads: pos, Writes: []string{State},
		Contract: "Returns the value of x, matched from start by an expression of the shape, where it is captured."}),
		"the shape (core.Shape)", immOf[core.Shape]())
	withImm(op(&code.Op{Name: OpScopeValue, Params: []code.Param{P("x", value), P("start", i)}, Results: []code.Type{value}, Reads: pos, Writes: []string{State},
		Contract: "Returns the value of a scope whose body, of the shape, matched x from start."}),
		"the shape (core.Shape)", immOf[core.Shape]())
	withImm(op(&code.Op{Name: OpAttachCaptures, Params: []code.Param{P("x", value), P("start", i), P("caps", value)}, Variadic: true,
		Results: []code.Type{value}, Reads: pos, Writes: []string{State},
		Contract: "Returns x, matched from start, with the captures of the scope attached: the values caps of the scope's slots."}),
		"the names and the source of the captures (Attach)", immOf[Attach]())
	withImm(op(&code.Op{Name: OpAttachFrame, Params: []code.Param{P("x", value), P("start", i), P("f", frame)},
		Results: []code.Type{value}, Reads: pos, Writes: []string{State},
		Contract: "Returns x, matched from start, with the captures of the scope attached: the values of the frame f."}),
		"the names, with Frame set (Attach)", immOf[Attach]())
	withImm(op(&code.Op{Name: OpMarkState, Results: []code.Type{mark}, Reads: all, Writes: []string{State},
		Contract: "Returns a mark of the parser state, which flow.reset returns to: the position, and the variable environment, the capture trail and the recovered errors as the immediate names them."}),
		"the parts the mark restores (StateMark)", immOf[StateMark]())
	op(&code.Op{Name: OpMarkPos, Params: []code.Param{P("m", mark)}, Results: []code.Type{i},
		Contract: "Returns the position of the mark m.", Laws: []string{pure}})
	withImm(op(&code.Op{Name: OpNewFrame, Results: []code.Type{frame}, Writes: []string{State},
		Contract: "Returns a new frame for the scope of the names."}), "the names (*Names)", immOf[*Names]())
	withImm(op(&code.Op{Name: OpFrameVal, Params: []code.Param{P("f", frame)}, Results: []code.Type{value},
		Contract: "Returns the value of the slot of the frame f.", Laws: []string{pure}}), "the slot (Slot)", immOf[Slot]())
	op(&code.Op{Name: OpKeep, Params: []code.Param{P("exps", expids)}, Results: []code.Type{expids}, Writes: []string{State},
		Contract: "Returns the expectations exps, kept beyond the scope that recorded them."})
	op(&code.Op{Name: OpMakeError, Params: []code.Param{P("far", i), P("exps", expids)}, Results: []code.Type{errT}, Writes: []string{State},
		Contract: "Returns the syntax error at far with the expectations exps."})
	op(&code.Op{Name: OpErrorValue, Params: []code.Param{P("start", i), P("x", errT)}, Results: []code.Type{value}, Reads: pos, Writes: []string{State},
		Contract: "Returns the value of the recovered error x over the input from start to the position."})
	op(&code.Op{Name: OpNewErrorNode, Params: []code.Param{P("start", i), P("x", errT)}, Results: []code.Type{value}, Reads: pos, Writes: []string{State},
		Contract: "Returns the error node of the recovered error x in the Node runtime, over the input from start to the position."})
	op(&code.Op{Name: OpFreshNode, Params: []code.Param{P("x", value)}, Results: []code.Type{value},
		Contract: "Returns x if it is a node its maker still owns, or nil.", Laws: []string{pure}})
	withImm(op(&code.Op{Name: OpConvertResult, Params: []code.Param{P("x", value), P("owned", value)}, Variadic: true, Results: []code.Type{value}, Writes: []string{State}, NoReturn: true,
		Contract: "Returns the value x of the rule converted to the rule's node type; with Owned, the only owned argument is the fresh node of the body, which converts in place."}),
		"the rule and whether the body's node is passed (Conversion)", immOf[Conversion]())
	op(&code.Op{Name: OpUseCtx, Params: []code.Param{P("start", i)}, Results: []code.Type{ctx}, Reads: pos, Writes: []string{State},
		Contract: "Returns a new context for the action of the rule entered at start."})
	withImm(op(&code.Op{Name: OpPred, Params: []code.Param{P("caps", value)}, Variadic: true, Results: []code.Type{b}, Reads: all, Writes: []string{State}, NoReturn: true, Cost: per(QBody),
		Contract: "Evaluates the predicate with the captures caps (or the current frame) and reports whether it holds."}),
		"the predicate and its captures (Predicate)", immOf[Predicate]())
	withImm(op(&code.Op{Name: OpBind, Params: []code.Param{P("caps", value)}, Variadic: true, Results: []code.Type{b}, Reads: all, Writes: []string{State}, NoReturn: true, Cost: per(QBody),
		Contract: "Binds the variable of the binding to the value of its expression, evaluated with the captures caps (or the current frame), and reports whether the evaluation succeeded."}),
		"the binding and its captures (Binding)", immOf[Binding]())
	withImm(op(&code.Op{Name: OpEval, Params: []code.Param{P("c", ctx), P("caps", value)}, Variadic: true, Results: []code.Type{value}, Reads: all, Writes: []string{State}, NoReturn: true, Cost: per(QBody),
		Contract: "Returns the value of the action of the function's rule, evaluated in the context c with the captures caps (or the current frame)."}),
		"the action and its captures (Action)", immOf[Action]())
	op(&code.Op{Name: OpCtxFinish, Params: []code.Param{P("c", ctx), P("x", value)}, Results: []code.Type{value}, Writes: []string{State},
		Contract: "Returns the value x of an action finished in the context c."})
	return ops
}

// Attrs are the attributes of a function of grammar code at the Flow stage, which code.Func.Attrs
// holds.
type Attrs struct {
	Rule *core.Rule
	// Vars are the locals that are variables of the function, in the order they were made; the
	// other locals are its entry variables Start, Result and Ctx (empty where it has none), which
	// the caller declares.
	Vars               []string
	Start, Result, Ctx string
	// Fail is the label of the block that the body leaves on failure.
	Fail string
	// Cut is the local of a framed function's cut flag, or empty.
	Cut string
	// Binds, Frame, Node, Budget, Document, Traced and Entered are as in Func.
	Binds, Frame, Node, Budget, Document, Traced, Entered bool
}

// AttrsOf returns the Flow attributes of f, or nil.
func AttrsOf(f *code.Func) *Attrs {
	a, _ := f.Attrs.(*Attrs)
	return a
}

// TypeOf returns the type of a variable of the Code IR type t at the Flow stage.
func TypeOf(t code.Type) Type {
	switch t.Kind {
	case code.KInt:
		return TypeInt
	case code.KBool:
		return TypeBool
	}
	for i, n := range typeNames {
		if n == t.Handle {
			return Type(i)
		}
	}
	panic("flow: no variable type " + t.String())
}

// CodeType returns the Code IR type of a variable of the type t.
func CodeType(t Type) code.Type {
	switch t {
	case TypeInt:
		return code.TInt
	case TypeBool:
		return code.TBool
	}
	return code.THandle(t.String())
}
