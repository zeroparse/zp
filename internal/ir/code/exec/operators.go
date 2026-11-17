package exec

import (
	"strings"

	"zeroparse.dev/zp/internal/ir/code"
)

// State components of the capture frames and the Operators Rule loop.
const (
	// Frames is the capture frames: the current frame, the slots of each frame and the trail of the
	// writes that a reset undoes.
	Frames = "frames"
	// Opr is the bookkeeping of the Operators Rule loop: the outcome of the last longest match.
	Opr = "operators"
)

// Names of the handle types of the capture frames and the Operators Rule loop.
const (
	// Frame is a capture frame: the slots of the captures of a rule body or a line.
	Frame = "Frame"
	// Backtrack is a backtracking point: the position, the variable environment, the recovered
	// errors and the length of the trail when it was taken.
	Backtrack = "Mark"
	// Operators is a list of the operators of an Operators Rule: its prefix or its other operators.
	Operators = "Operators"
	// Operator is an operator of an Operators Rule.
	Operator = "Operator"
	// Line is a line of an Operators Rule: an operand, or the expression of an operator.
	Line = "Line"
	// Attempt is a match of an operator: the operator, its span, value, captures, environment,
	// cut and recovered errors.
	Attempt = "Attempt"
)

// Cost quantities of the capture frames.
const (
	// QSlots is the number of slots of the frame that the call takes or opens.
	QSlots = "slots"
	// QTrail is the number of trail writes after the mark that the call undoes.
	QTrail = "trail"
)

// The names of the handle types of the records of the split forms of the Operators Rule loop.
const (
	LongestState = "LongestState"
	NudState     = "NudState"
	ExprState    = "ExprState"
)

// Cut is the register of the cut flag: whether the scope the parse entered last committed.
const Cut = "cut"

// framesAndOperators adds the operations of the capture frames and the Operators Rule loop, as add
// adds transitional operations.
func framesAndOperators(m *code.Module, add func(*code.Op), rule, value, errs code.Type) {
	m.AddRegister(Cut, code.TBool, "The cut flag of the scope the parse entered last.")
	m.AddComponent(Frames, `The capture frames: the frame that captures write to, the slots of each frame and the
trail of the writes that a reset undoes.`)
	m.AddComponent(Opr, `The bookkeeping of the Operators Rule loop: whether a candidate of the last longest match
failed after a cut.`)
	frame := m.AddHandle(Frame, Frames, "A capture frame: the slots of the captures of a rule body or a line.")
	mark := m.AddHandle(Backtrack, Frames, `A backtracking point: the position, the variable environment, the
recovered errors and the length of the trail when it was taken.`)
	scope := code.THandle(Scope)
	opsList := m.AddHandle(Operators, Rules, "The prefix operators of an Operators Rule, or its other operators.")
	op := m.AddHandle(Operator, Rules, "An operator of an Operators Rule: its kind, binding power, associativity and line.")
	line := m.AddHandle(Line, Rules, "A line of an Operators Rule: an operand, or the expression of an operator.")
	attempt := m.AddHandle(Attempt, Opr, `A match of an operator: the operator, where it started and ended, its value,
the frame of its captures, the environment after it, whether it cut, and the errors it recovered. The
null attempt is no match.`)
	regs := []string{Pos, Silent, Cut}
	r := []code.Param{code.P("r", rule)}
	f := []code.Param{code.P("f", frame)}
	m1 := []code.Param{code.P("m", mark)}
	a := []code.Param{code.P("a", attempt)}
	o := []code.Param{code.P("op", op)}
	slots := code.Cost{Terms: []code.CostTerm{{Coef: 1, Quantity: QSlots}}}

	// The capture frames and backtracking.
	m.AddOp(&code.Op{Name: "frame.mark", Component: Frames, Results: []code.Type{mark}, Reads: []string{Frames, Env, Rec},
		Contract: `Returns a backtracking point at the position.`})
	m.AddOp(&code.Op{Name: "frame.markPos", Component: Frames, Params: m1, Results: []code.Type{code.TInt},
		Contract: `Returns the position of the backtracking point m.`})
	m.AddOp(&code.Op{Name: "frame.markRec", Component: Frames, Params: m1, Results: []code.Type{code.TInt},
		Contract: `Returns the number of the errors recovered before the backtracking point m, as rec.mark does.`})
	m.AddOp(&code.Op{Name: "frame.markEnv", Component: Frames, Params: m1, Results: []code.Type{scope},
		Contract: `Returns the variable environment of the backtracking point m.`})
	m.AddOp(&code.Op{Name: "frame.markMissing", Component: Frames, Params: m1, Results: []code.Type{code.TInt}, Features: []string{Missing},
		Contract: `Returns the mark of the missing-token record at the backtracking point m, as missing.mark does.`})
	m.AddOp(&code.Op{Name: "frame.markTrail", Component: Frames, Params: m1, Results: []code.Type{code.TInt},
		Contract: `Returns the length of the trail at the backtracking point m.`})
	m.AddOp(&code.Op{Name: "frame.undo", Component: Frames, Params: []code.Param{code.P("trail", code.TInt)}, Reads: []string{Frames},
		Writes: []string{Frames}, NoReturn: true, Cost: code.Cost{Terms: []code.CostTerm{{Coef: 1, Quantity: QTrail}}},
		Contract: `Undoes the writes of the trail after its first trail writes, in reverse order, charging one unit
for each, and drops them: every slot they wrote has the value it had before.`,
		Requires: `trail is at most the length of the trail.`})
	m.AddOp(&code.Op{Name: "frame.current", Component: Frames, Results: []code.Type{frame}, Reads: []string{Frames},
		Contract: `Returns the frame that captures write to.`})
	m.AddOp(&code.Op{Name: "frame.use", Component: Frames, Params: f, Writes: []string{Frames},
		Contract: `Makes f the frame that captures write to.`})
	m.AddOp(&code.Op{Name: "frame.open", Component: Frames, Params: []code.Param{code.P("n", code.TInt)}, Results: []code.Type{frame},
		Writes: []string{Frames}, NoReturn: true, Cost: slots,
		Contract: `Returns a new frame of n empty slots, which lives until the parse ends. A frame of no slots is
never written, so it may be shared.`})
	m.AddOp(&code.Op{Name: "frame.set", Component: Frames, Params: []code.Param{code.P("slot", code.TInt), code.P("v", value)},
		Reads: []string{Frames}, Writes: []string{Frames},
		Contract: `Writes v to the slot of the current frame and adds the slot's old value to the trail. The write
is free: its size depends on nothing, the match that made v paid for itself, and frame.undo charges for
each write it undoes.`,
		Requires: `slot is a slot of the current frame.`})
	m.AddOp(&code.Op{Name: "frame.close", Component: Frames, Params: f, Writes: []string{Frames},
		Contract: `Ends f, which nothing refers to any more: a rule's frame when the rule returns. A target may
reuse it, and one without a pool of frames does nothing.`,
		Requires: `No trail entry and no current frame refer to f.`})
	m.AddOp(&code.Op{Name: "frame.slots", Component: Frames, Params: f, Results: []code.Type{code.TInt},
		Contract: `Returns the number of slots of f.`})
	m.AddOp(&code.Op{Name: "frame.save", Component: Frames, Params: f, Reads: []string{Frames}, Writes: []string{Frames},
		NoReturn: true, Cost: slots,
		Contract: `Copies the slots of f aside, charging the copy as the target charges copies of values.`})
	m.AddOp(&code.Op{Name: "frame.restore", Component: Frames, Params: f, Reads: []string{Frames}, Writes: []string{Frames},
		NoReturn: true, Cost: slots,
		Contract: `Copies the slots that the last frame.save set aside back into f, charging the copy as
frame.save does.`,
		Requires: `The last frame.save copied the slots of f, and nothing parsed since.`})

	// The descriptors of an Operators Rule.
	m.AddOp(&code.Op{Name: "opr.led", Component: Rules, Params: r, Results: []code.Type{opsList},
		Contract: `Returns the infix and postfix operators of the Operators Rule r, in declaration order.`})
	m.AddOp(&code.Op{Name: "opr.prefix", Component: Rules, Params: r, Results: []code.Type{opsList},
		Contract: `Returns the prefix operators of the Operators Rule r, in declaration order.`})
	m.AddOp(&code.Op{Name: "opr.count", Component: Rules, Params: []code.Param{code.P("l", opsList)}, Results: []code.Type{code.TInt},
		Contract: `Returns the number of operators of l.`})
	m.AddOp(&code.Op{Name: "opr.at", Component: Rules, Params: []code.Param{code.P("l", opsList), code.P("i", code.TInt)},
		Results: []code.Type{op}, Requires: `0 <= i < opr.count(l).`, Contract: `Returns operator i of l.`})
	m.AddOp(&code.Op{Name: "opr.operands", Component: Rules, Params: r, Results: []code.Type{code.TInt},
		Contract: `Returns the number of operand lines of the Operators Rule r.`})
	m.AddOp(&code.Op{Name: "opr.operand", Component: Rules, Params: append(r, code.P("i", code.TInt)), Results: []code.Type{line},
		Requires: `0 <= i < opr.operands(r).`, Contract: `Returns operand line i of the Operators Rule r, in declaration order.`})
	m.AddOp(&code.Op{Name: "opr.line", Component: Rules, Params: o, Results: []code.Type{line},
		Contract: `Returns the line of the operator op.`})
	m.AddOp(&code.Op{Name: "opr.level", Component: Rules, Params: o, Results: []code.Type{code.TInt},
		Contract: `Returns the binding power of the operator op: the higher, the tighter it binds.`})
	for _, x := range []struct{ name, doc string }{
		{"opr.infix", "Reports whether the operator op is an infix operator."},
		{"opr.postfix", "Reports whether the operator op is a postfix operator."},
		{"opr.right", "Reports whether the operator op associates to the right."},
		{"opr.nonAssoc", "Reports whether the operator op does not associate."},
	} {
		m.AddOp(&code.Op{Name: x.name, Component: Rules, Params: o, Results: []code.Type{code.TBool}, Contract: x.doc})
	}
	m.AddOp(&code.Op{Name: "opr.scope", Component: Rules, Params: []code.Param{code.P("l", line)}, Results: []code.Type{code.TInt},
		Contract: `Returns the number of capture slots of the line l.`})
	m.AddOp(&code.Op{Name: "opr.hasSkip", Component: Rules, Params: r, Results: []code.Type{code.TBool},
		Contract: `Reports whether the Operators Rule r skips trivia between its operands and operators.`})

	// The grammar code and the values of an Operators Rule.
	add(&code.Op{Name: "opr.skip", Component: Rules, Params: r, Results: []code.Type{code.TBool},
		Writes: append([]string{Rules}, regs...), NoReturn: true,
		Contract: `Runs the trivia expression of the Operators Rule r and reports whether it matched; on failure
the position is unspecified.`, Requires: `opr.hasSkip(r).`})
	add(&code.Op{Name: "opr.match", Component: Rules, Params: []code.Param{code.P("l", line)}, Results: []code.Type{value, code.TBool},
		Writes: append([]string{Rules}, regs...), NoReturn: true,
		Contract: `Runs the expression of the line l in the current frame and returns its value and success; on
failure the position is unspecified. A cut in the expression sets the cut register.`})
	add(&code.Op{Name: "opr.lineValue", Component: Host, Params: append(r, code.P("l", line), code.P("f", frame),
		code.P("v", value), code.P("start", code.TInt)), Results: []code.Type{value}, Reads: []string{Frames},
		Writes: []string{Host}, NoReturn: true,
		Contract: `Returns the value of the operand line l of the rule r that matched from start to the position
with the value v and the captures of f: the line's action, or its shaped value with its captures.`})
	add(&code.Op{Name: "opr.build", Component: Host, Params: append(r, code.P("a", attempt), code.P("lhs", value), code.P("rhs", value)),
		Results: []code.Type{value}, Reads: []string{Opr, Frames}, Writes: []string{Host}, NoReturn: true,
		Contract: `Returns the value of the application of the operator of a to lhs (null for a prefix operator)
and rhs (null for a postfix operator), which ends at the position: the action of the operator's line,
or the node of the tuple (lhs, operator, rhs) with the line's captures.`})
	add(&code.Op{Name: "opr.buildTyped", Component: Host, Params: append(r, code.P("a", attempt), code.P("lhs", value),
		code.P("lstart", code.TInt), code.P("rhs", value)), Results: []code.Type{value, code.TInt}, Reads: []string{Opr, Frames},
		Writes: []string{Host}, NoReturn: true, Features: []string{Typed},
		Contract: `Is opr.build for typed values, where lhs, if it is not a node, starts at lstart; it also returns
where the value of the application starts.`})

	// The matches of operators.
	m.AddOp(&code.Op{Name: "opr.attempt", Component: Opr, Params: append(o, code.P("f", frame), code.P("v", value),
		code.P("start", code.TInt), code.P("cut", code.TBool), code.P("errs", errs)), Results: []code.Type{attempt},
		Reads: []string{Env, Pos},
		Contract: `Returns the match of op from start to the position with the value v, the captures of f, the
variable environment, the cut flag cut and the recovered errors errs.`})
	add(&code.Op{Name: "opr.attemptMissing", Component: Opr, Params: append(o, code.P("f", frame), code.P("v", value),
		code.P("start", code.TInt), code.P("cut", code.TBool), code.P("errs", errs), code.P("mark", code.TInt)), Results: []code.Type{attempt},
		Reads: []string{Env, Pos, Rec}, Features: []string{Missing}, NoReturn: true,
		Contract: `Is opr.attempt in a parse that can insert missing tokens: the match also holds a copy of the tokens
inserted after mark.`})
	add(&code.Op{Name: "opr.replay", Component: Rec, Params: a, Reads: []string{Opr}, Writes: []string{Rec}, Features: []string{Missing}, NoReturn: true,
		Cost: code.Cost{Terms: []code.CostTerm{{Coef: 1, Quantity: QErrs}}},
		Contract: `Records again the tokens that the match a inserted and the errors it recovered, as rec.add does for
the errors outside the missing-token mode.`})
	m.AddOp(&code.Op{Name: "opr.found", Component: Opr, Params: a, Results: []code.Type{code.TBool},
		Contract: `Reports whether a is a match, not the null attempt.`})
	m.AddOp(&code.Op{Name: "opr.op", Component: Opr, Params: a, Results: []code.Type{op},
		Requires: `opr.found(a).`, Contract: `Returns the operator of the match a.`})
	m.AddOp(&code.Op{Name: "opr.end", Component: Opr, Params: a, Results: []code.Type{code.TInt},
		Requires: `opr.found(a).`, Contract: `Returns where the match a ended.`})
	m.AddOp(&code.Op{Name: "opr.cut", Component: Opr, Params: a, Results: []code.Type{code.TBool},
		Requires: `opr.found(a).`, Contract: `Reports whether the match a cut.`})
	m.AddOp(&code.Op{Name: "opr.env", Component: Opr, Params: a, Results: []code.Type{scope},
		Requires: `opr.found(a).`, Contract: `Returns the variable environment after the match a.`})
	m.AddOp(&code.Op{Name: "opr.errs", Component: Opr, Params: a, Results: []code.Type{errs},
		Requires: `opr.found(a).`, Contract: `Returns the errors that the match a recovered.`})
	m.AddOp(&code.Op{Name: "opr.setCutFailed", Component: Opr, Params: []code.Param{code.P("failed", code.TBool)},
		Writes: []string{Opr}, Contract: `Records whether a candidate of the longest match failed after a cut.`})
	m.AddOp(&code.Op{Name: "opr.cutFailed", Component: Opr, Results: []code.Type{code.TBool}, Reads: []string{Opr},
		Contract: `Reports what the last opr.setCutFailed recorded.`})
	m.AddOp(&code.Op{Name: "opr.setNudBound", Component: Opr, Params: []code.Param{code.P("bound", code.TInt)},
		Writes: []string{Opr}, Contract: `Records the binding level above which no operator can follow the start of an expression.`})
	m.AddOp(&code.Op{Name: "opr.nudBound", Component: Opr, Results: []code.Type{code.TInt}, Reads: []string{Opr},
		Contract: `Returns what the last opr.setNudBound recorded.`})
	m.AddOp(&code.Op{Name: "opr.setAfterEmpty", Component: Opr, Params: []code.Param{code.P("empty", code.TBool)},
		Writes: []string{Opr}, Contract: `Records whether the right operand that the next expression parses follows an infix operator applied without consuming input.`})
	m.AddOp(&code.Op{Name: "opr.afterEmpty", Component: Opr, Results: []code.Type{code.TBool}, Reads: []string{Opr},
		Contract: `Returns what the last opr.setAfterEmpty recorded.`})

	// The records of the split forms of the loop, which their caller holds between two steps.
	field := func(n string, t code.Type) splitField { return splitField{n, t} }
	lines := []splitField{field("prevFrame", frame), field("prevCut", code.TBool), field("f", frame)}
	for _, rec := range []struct {
		prefix, handle, doc string
		fields              []splitField
	}{
		{"oprLongest", LongestState, "The record of a suspended longest match.", append([]splitField{field("l", opsList), field("i", code.TInt),
			field("m0", mark), field("best", attempt), field("cutFailed", code.TBool)}, lines...)},
		{"oprNud", NudState, "The record of the suspended start of an expression.", append([]splitField{field("m0", mark), field("a", attempt),
			field("i", code.TInt)}, lines...)},
		{"oprExpr", ExprState, "The record of the suspended loop of an expression.", []splitField{field("min", code.TInt), field("lhs", value),
			field("lastNone", code.TInt), field("bound", code.TInt), field("guard", code.TInt), field("m0", mark), field("a", attempt)}},
	} {
		st := code.P("st", m.AddHandle(rec.handle, Suspended, rec.doc))
		for _, f := range rec.fields {
			m.AddOp(&code.Op{Name: rec.prefix + "." + f.name, Component: Suspended, Params: []code.Param{st}, Results: []code.Type{f.t},
				Reads: []string{Suspended},
				Contract: `Returns the field ` + f.name + ` of the record st: what the last set of it stored, and before the
first the zero value of its type (0, false, the list of no operators, the zero backtracking point, no frame,
value or match).`})
			m.AddOp(&code.Op{Name: rec.prefix + ".set" + strings.ToUpper(f.name[:1]) + f.name[1:], Component: Suspended,
				Params: []code.Param{st, code.P("x", f.t)}, Writes: []string{Suspended},
				Contract: `Sets the field ` + f.name + ` of the record st to x.`})
		}
	}
}

// A splitField is a field of a record of the split forms of the Operators Rule loop.
type splitField struct {
	name string
	t    code.Type
}
