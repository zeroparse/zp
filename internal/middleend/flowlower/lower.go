// Package flowlower lowers a rule of Core IR (internal/ir/core) to a function of Code IR at the Flow
// stage (internal/ir/code/flow), in a module of its own that the Flow verifier accepts.
//
// Lower threads a failure label through each node: a node's statements leave the block of that
// label when the node fails, and run to their end when it succeeds, leaving the node's value in a
// variable. Captures are variables of the function; those of a repetition element with captures
// of its own are cleared at the start of each iteration. A backtracking point (choice, option,
// repetition element, negative lookahead) saves the position, the number of recovered errors if
// the module can recover, the environment if the expression binds variables, and the capture
// slots the expression may write (Effects.Slots), and restores them when the expression fails.
// Cuts are flags of the function, one per backtracking point inside a rule whose body cuts.
//
// A framed function (LowerFrame) keeps the captures in the runtime's frames instead: a capture
// sets its slot of the current frame, a repetition element with captures of its own runs under a
// frame of its own, and a backtracking point marks the whole parser state, frames included, and
// resets to the mark when it fails. Its outermost scope cuts the flag the runtime gave it.
//
// The lowering follows reachability as it goes: after an Exit the point is unreachable until a
// block that is exited to from a reachable point ends, or a loop ends. A node that begins at an
// unreachable point is not lowered, nor are the alternatives of a choice after one that cannot
// fail, nor the failure path of a backtracking point whose body cannot fail. The other statements
// written at unreachable points are kept (see the package comment of flow).
//
// Variables and labels are numbered from one counter in the order they are made; the variables
// shared by the whole function (ok, ch, size, bv) are made once, at their first use.
package flowlower

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
)

// Options selects the forms the lowering writes.
type Options struct {
	// Budget charges one compute unit on each loop iteration and for each character a Peek reads;
	// the method that holds a rule's body charges its entry.
	Budget bool
	// Recovers reports that the module can append recovered errors (Recovers), so backtracking
	// points save their number.
	Recovers bool
	// ClassTableRanges, if positive, matches a class of at least that many ranges through its
	// membership table. It needs Budget off: a table match charges no budget for the input.
	ClassTableRanges int
	// GuardedClasses keeps the size of the character a choice's guards read, for the classes
	// that begin alternatives marked GuardedClass, which only consume it. It needs Budget off: the
	// consuming advance charges no budget for the input.
	GuardedClasses bool
	// InlineLiterals compares literals of at most this many code points in place first.
	InlineLiterals int
	// Observe reports the alternatives that the guards of choices skip (ObserveSkip).
	Observe bool
	// Node builds the values of the Node runtime (NewTerminalNode, NewBranchNode) in place of the typed
	// ones, in the functions it lowers (flow.Attrs.Node). It holds for every function of a module's lowering,
	// so the options of the Node and recognizer tables are never shared with the typed table's.
	Node bool
	// Stream lowers the #stream repetition (core.Stream) of a framed Node function as the repetition that
	// delivers its elements in a stream parse: it hands each committed element to the callback instead of
	// gathering it, when the invocation streams (StreamOn), and gathers it otherwise. The function is the
	// body of the start rule that ParseStream runs.
	Stream bool
	// Document lowers for a build with Documents. A parse of a Document records how far it examined the input
	// in the primitives that read it; the form that reads input in place (HasLit) is then valid only with
	// the recording primitive (MatchLit) as its alternative, which the target takes when the parse is a
	// Document's. The Flow verifier rejects an in-place read without one in a function lowered with it (flow.Attrs.Document).
	Document bool
	// Traced lowers the traced copy of a rule that an observed parse runs (flow.Attrs.Traced): every read of the
	// input records how far the parse examined, and the Flow verifier rejects a read in place in the function.
	Traced bool
	// Outline, if not nil, selects the nodes of a framed function that run as functions of their own: the
	// function calls them (CallOutline) under its frame and lowers them with LowerOutline. A node it selects
	// must be Outlinable; the lowering keeps every node in a function lowered with Observe.
	Outline func(core.Node) bool
}

// Recovers reports whether the module has a Recover, which appends recovered errors.
func Recovers(m *core.Module) bool {
	found := false
	for _, r := range m.Rules {
		if r != nil {
			core.Walk(r.Body, func(n core.Node) {
				if _, ok := n.(*core.Recover); ok {
					found = true
				}
			})
		}
	}
	return found
}

// LocalRule reports whether Lower can lower the rule r: its captures can be variables of its
// function. Left-recursion leaders, whose growth the runtime owns, Operators Rules and rules with
// recovery keep runtime frames.
func LocalRule(r *core.Rule) bool {
	if r.Policy.Leader {
		return false
	}
	if _, ok := r.Body.(*core.OperatorsRule); ok {
		return false
	}
	ok := true
	core.Walk(r.Body, func(n core.Node) {
		switch n.(type) {
		case *core.Recover, *core.Nest, *core.OperatorsRule:
			ok = false
		}
	})
	return ok
}

// FramedBody reports whether LowerFrame can lower the body n: it has no Operators Rule, inlined
// call or terminal region, which the runtime or a local function matches.
func FramedBody(n core.Node) bool {
	ok := true
	core.Walk(n, func(n core.Node) {
		switch n.(type) {
		case *core.Nest, *core.OperatorsRule, *core.Terminal:
			ok = false
		}
	})
	return ok
}

// LowerFrame lowers the body n with the capture scope slots, building its value if build, to the
// framed function name of the rule r: the body of a left-recursion leader or of a rule that
// recovers, or a line of an Operators Rule of r. Analyze must have run on m. With Options.Budget the
// function does not charge its entry: the method that calls it does (flow.Attrs.Entered).
func LowerFrame(m *core.Module, r *core.Rule, n core.Node, slots []core.Slot, build bool, name string, opts Options) (*code.Module, error) {
	return lowerFrameBody(m, r, n, slots, build, name, opts)
}

// Outlinable reports whether a node can run as a function of its own (LowerOutline): it is a framed
// body without a cut, whose flag the function would have to share with the enclosing backtracking points.
func Outlinable(n core.Node) bool {
	if !FramedBody(n) {
		return false
	}
	ok := true
	core.Walk(n, func(n core.Node) {
		switch n.(type) {
		case *core.Cut, *core.Stream:
			ok = false
		}
	})
	return ok
}

// LowerOutline lowers the node n, which a CallOutline of a framed function runs, to the framed function
// name. The function belongs to the rule of its caller, which charged the entry of the rule: it
// charges only the repetitions below n. Analyze must have run on m.
func LowerOutline(m *core.Module, r *core.Rule, n core.Node, slots []core.Slot, build bool, name string, opts Options) (*code.Module, error) {
	return lowerFrameBody(m, r, n, slots, build, name, opts)
}

func lowerFrameBody(m *core.Module, r *core.Rule, n core.Node, slots []core.Slot, build bool, name string, opts Options) (*code.Module, error) {
	if err := checkOptions(opts); err != nil {
		return nil, err
	}
	if !FramedBody(n) {
		return nil, fmt.Errorf("a body of rule %s is not lowered to a framed function", r.Name)
	}
	l := newLowerer(m, opts, name, &flow.Attrs{Rule: r, Frame: true, Node: opts.Node, Budget: opts.Budget, Document: opts.Document,
		Traced: opts.Traced, Entered: opts.Budget, Binds: n.Attrs().Eff.Set&core.EffBind != 0})
	l.frame = true
	l.top = n
	core.Walk(n, func(n core.Node) {
		if _, ok := n.(*core.Cut); ok {
			l.localCuts = true
		}
	})
	if l.localCuts {
		l.at.Cut = l.decl("cut", flow.TypeBool)
		l.set(l.at.Cut, code.R(flow.Cut))
		l.cut = l.at.Cut
	}
	v := l.node(n, newScope(slots), build, l.at.Fail)
	if l.dead {
		v = nil
	}
	return l.module(v)
}

// checkOptions rejects the options that lower without work only.
func checkOptions(opts Options) error {
	if opts.Budget && (opts.ClassTableRanges > 0 || opts.GuardedClasses) {
		return fmt.Errorf("ClassTableRanges and GuardedClasses lower functions without budget controls, but Options.Budget is set")
	}
	return nil
}

// Lower lowers the rule r of the module m, a LocalRule, to the function name. Analyze must have
// run on m.
func Lower(m *core.Module, r *core.Rule, name string, opts Options) (*code.Module, error) {
	if err := checkOptions(opts); err != nil {
		return nil, err
	}
	if !LocalRule(r) {
		return nil, fmt.Errorf("rule %s is not lowered to a local function", r.Name)
	}
	l := newLowerer(m, opts, name, &flow.Attrs{Rule: r, Start: "start", Result: "v", Ctx: "c",
		Node:     opts.Node,
		Budget:   opts.Budget,
		Document: opts.Document,
		Traced:   opts.Traced,
		Entered:  opts.Budget, // the method that calls the function charges the step of entering the rule
		Binds:    r.Body.Attrs().Eff.Set&core.EffBind != 0})
	core.Walk(r.Body, func(n core.Node) {
		if _, ok := n.(*core.Cut); ok {
			l.localCuts = true
		}
	})
	s := newScope(r.Scope)
	body := l.node(r.Body, s, r.Want, l.at.Fail)
	if !l.dead {
		l.finish(r, s, body)
	}
	return l.module(nil)
}

type lowerer struct {
	// top is the root of a framed function, which stays in it.
	top  core.Node
	m    *core.Module
	opts Options
	// fn is the function being lowered and at its attributes; body is the block that the body
	// leaves on failure (Attrs.Fail).
	fn   *code.Func
	at   *flow.Attrs
	body *code.Block
	n    int
	// loops holds the labels of the loops; an exit to one is a break.
	loops map[string]bool
	anon  int
	// stack holds the statement lists being written, the innermost last.
	stack []*[]code.Stmt
	// dead reports that the current point is unreachable; used holds the labels exited to from
	// reachable points.
	dead bool
	used map[string]bool
	// localCuts reports that the rule's body cuts; cut is the flag of the innermost backtracking
	// point, empty where cuts are discarded (at the rule's top level and inside lookaheads).
	localCuts bool
	cut       string
	// guarded is the class that begins the alternative guardedAlt being lowered, which the guard
	// has read and accepted; guardedSize holds the size of its character. node forgets guarded at
	// any other node.
	guarded     *core.ClassNode
	guardedAlt  core.Node
	guardedSize string
	// frame reports a framed function.
	frame bool
	// streaming is the repetition of the Stream node being lowered in a stream parse (Options.Stream).
	streaming *core.Repeat
}

// newLowerer returns a lowerer of the function name with the attributes at, whose body is the block
// that it leaves on failure.
func newLowerer(m *core.Module, opts Options, name string, at *flow.Attrs) *lowerer {
	at.Fail = "fail"
	l := &lowerer{m: m, opts: opts, at: at, fn: &code.Func{Name: name, Attrs: at}, body: &code.Block{Label: at.Fail},
		used: map[string]bool{}, loops: map[string]bool{}}
	l.stack = []*[]code.Stmt{&l.body.Stmts}
	return l
}

// module returns the module of the lowered function, at the Flow stage, if the Flow verifier accepts it:
// a function that breaks an invariant is a lowering error, not something for the printer to write.
//
// The body of the function is the block labeled Attrs.Fail, which an exit on failure leaves; the
// caller prints what follows success and failure. A framed function ends its block by publishing its
// cut flag (Attrs.Cut) and returning value (the null value where it is nil, because the function cannot
// succeed) and true; after the block, it publishes the cut flag again and returns the null value and
// false. A local function declares its entry variables (Start, Result, Ctx) as locals after its
// variables (Attrs.Vars).
func (l *lowerer) module(value code.Expr) (*code.Module, error) {
	fn, at := l.fn, l.at
	fn.Body = l.body
	for _, e := range []struct {
		name string
		t    flow.Type
	}{{at.Start, flow.TypeInt}, {at.Result, flow.TypeValue}, {at.Ctx, flow.TypeCtx}} {
		if e.name != "" {
			fn.Locals = append(fn.Locals, code.P(e.name, flow.CodeType(e.t)))
		}
	}
	if at.Frame {
		t := code.THandle(flow.TValue)
		fn.Results = []code.Type{t, code.TBool}
		publish := func() []code.Stmt {
			if at.Cut == "" {
				return nil
			}
			return []code.Stmt{&code.Set{Dst: []code.Place{{Name: flow.Cut, Reg: true}}, Values: []code.Expr{code.L(at.Cut)}}}
		}
		if value == nil {
			value = null()
		}
		l.body.Stmts = append(append(l.body.Stmts, publish()...), &code.Return{Values: []code.Expr{value, code.BoolC(true)}})
		fn.Body = &code.Block{Stmts: append(append([]code.Stmt{l.body}, publish()...),
			&code.Return{Values: []code.Expr{null(), code.BoolC(false)}})}
	}
	unshare(fn.Body)
	m := flow.NewModule("flow " + fn.Name)
	m.Funcs = append(m.Funcs, fn)
	if err := flow.VerifyModule(m); err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}
	return m, nil
}

// scope is a capture scope: its slots and the variables made for them.
type scope struct {
	slots []core.Slot
	vars  []string
}

func newScope(slots []core.Slot) *scope {
	return &scope{slots: slots, vars: make([]string, len(slots))}
}

func (l *lowerer) emit(s code.Stmt) {
	p := l.stack[len(l.stack)-1]
	*p = append(*p, s)
}

// open begins a block of the label lb in the current statement list; close ends it.
func (l *lowerer) open(lb string) *code.Block {
	b := &code.Block{Label: lb}
	l.emit(b)
	l.stack = append(l.stack, &b.Stmts)
	return b
}

func (l *lowerer) close(b *code.Block) {
	l.stack = l.stack[:len(l.stack)-1]
	if l.used[b.Label] {
		l.dead = false
	}
}

// loop begins the loop lp in the current statement list; endLoop ends it. The point after a loop
// is reachable: every loop can be left.
func (l *lowerer) loop(lp *code.Loop) {
	l.loops[lp.Label] = true
	lp.Body = &code.Block{}
	l.emit(lp)
	l.stack = append(l.stack, &lp.Body.Stmts)
}

func (l *lowerer) endLoop() {
	l.stack = l.stack[:len(l.stack)-1]
	l.dead = false
}

// decl makes a variable of the type t, named with prefix and the counter.
func (l *lowerer) decl(prefix string, t flow.Type) string {
	l.n++
	return l.local(fmt.Sprintf("%s%d", prefix, l.n), t)
}

func (l *lowerer) local(name string, t flow.Type) string {
	l.fn.Locals = append(l.fn.Locals, code.P(name, flow.CodeType(t)))
	l.at.Vars = append(l.at.Vars, name)
	return name
}

// shared returns the variable name shared by the whole function, making it at its first use.
func (l *lowerer) shared(name string, t flow.Type) string {
	for _, v := range l.at.Vars {
		if v == name {
			return v
		}
	}
	return l.local(name, t)
}

func (l *lowerer) label() string {
	l.n++
	return fmt.Sprintf("L%d", l.n)
}

// loopLabel returns the label of a new loop, which the lowering leaves unnamed (flow.UnnamedLabel).
func (l *lowerer) loopLabel() string {
	l.anon++
	return flow.UnnamedLabel(l.anon)
}

// jump returns the statement that leaves the block or loop to.
func (l *lowerer) jump(to string) code.Stmt {
	if l.loops[to] {
		return &code.Break{Label: to}
	}
	return &code.Exit{Label: to}
}

func (l *lowerer) ok() string { return l.shared("ok", flow.TypeBool) }

// exit leaves the block or loop to; the point after it is unreachable.
func (l *lowerer) exit(to string) {
	if !l.dead {
		l.used[to] = true
	}
	l.emit(l.jump(to))
	l.dead = true
}

func (l *lowerer) cont(to string) {
	l.emit(&code.Continue{Label: to})
	l.dead = true
}

// failIf leaves the block fail, after the statements pre, when cond holds.
func (l *lowerer) failIf(init *code.Set, cond code.Expr, fail string, pre ...code.Stmt) {
	if !l.dead {
		l.used[fail] = true
	}
	l.emit(&code.If{Init: init, Cond: cond, Then: &code.Block{Stmts: append(pre, l.jump(fail))}})
}

// set assigns x to the variable dst.
func (l *lowerer) set(dst string, x code.Expr) {
	l.emit(code.Assign(dst, x))
}

// setPos, setEnv and setFrame assign x to the registers of the position, the environment and the frame.
func (l *lowerer) setPos(x code.Expr) {
	l.emit(&code.Set{Dst: []code.Place{{Name: flow.Pos, Reg: true}}, Values: []code.Expr{x}})
}

func (l *lowerer) setEnv(x code.Expr) { l.emit(do(flow.OpSetEnv, nil, x)) }

func (l *lowerer) setFrame(x code.Expr) { l.emit(do(flow.OpSetFrame, nil, x)) }

// step charges one budget step. A rule charges one on entering, which the method that holds the lowered
// function does (Attrs.Entered), and the lowered function charges one on each iteration of a repetition.
// No other point of straight-line code charges: between two charges only the straight-line code of one
// rule body runs, whose length is bounded by the size of the grammar, so the steps still bound the spend.
func (l *lowerer) step() {
	if l.opts.Budget {
		l.emit(do(flow.OpStep, nil))
	}
}

// capVar returns the variable of the slot of s, making it at its first use.
func (l *lowerer) capVar(s *scope, slot int) string {
	if s.vars[slot] == "" {
		s.vars[slot] = l.decl("k", flow.TypeValue)
	}
	return s.vars[slot]
}

// mark is the state a backtracking point restores: in a framed function, the mark state of the
// whole parser state.
type mark struct {
	pos, rec, env string
	saves         [][2]string // capture variable, saved value
	state         string
}

// at is the position of the mark.
func (m mark) at() code.Expr {
	if m.state != "" {
		return call(flow.OpMarkPos, nil, use(m.state))
	}
	return use(m.pos)
}

// mark saves the state before n; with saves, also the capture slots of s that n may write.
func (l *lowerer) mark(s *scope, n core.Node, saves bool) mark {
	if l.frame {
		m := mark{state: l.decl("x", flow.TypeMark)}
		eff := n.Attrs().Eff
		st := flow.StateMark{Env: eff.Set&core.EffBind != 0, Rec: eff.Set&core.EffRecover != 0}
		core.Walk(n, func(n core.Node) {
			if _, ok := n.(*core.Capture); ok {
				st.Trail = true
			}
		})
		l.set(m.state, call(flow.OpMarkState, st))
		return m
	}
	m := mark{pos: l.decl("x", flow.TypeInt)}
	if l.opts.Recovers {
		m.rec = l.decl("x", flow.TypeInt)
		l.emit(&code.Set{Dst: []code.Place{{Name: m.pos}, {Name: m.rec}}, Values: []code.Expr{pos(), call(flow.OpRecLen, nil)}})
	} else {
		l.set(m.pos, pos())
	}
	eff := n.Attrs().Eff
	if eff.Set&core.EffBind != 0 {
		m.env = l.decl("x", flow.TypeEnv)
		l.set(m.env, call(flow.OpEnv, nil))
	}
	if saves {
		for _, slot := range eff.Slots {
			k := l.capVar(s, slot)
			sv := l.decl("x", flow.TypeValue)
			l.set(sv, use(k))
			m.saves = append(m.saves, [2]string{k, sv})
		}
	}
	return m
}

// reset restores the state saved by m.
func (l *lowerer) reset(m mark) {
	if m.state != "" {
		l.emit(do(flow.OpReset, nil, use(m.state)))
		return
	}
	if m.rec == "" {
		l.setPos(use(m.pos))
	} else {
		l.emit(do(flow.OpRestore, nil, use(m.pos), use(m.rec)))
	}
	if m.env != "" {
		l.setEnv(use(m.env))
	}
	for _, kv := range m.saves {
		l.set(kv[0], use(kv[1]))
	}
}

// cutNode lowers a backtracking point's body with a cut flag of its own, if the rule cuts.
func (l *lowerer) cutNode(n core.Node, s *scope, build bool, fail string) (code.Expr, string) {
	if !l.localCuts {
		return l.node(n, s, build, fail), ""
	}
	prev := l.cut
	cut := l.decl("cut", flow.TypeBool)
	l.set(cut, code.BoolC(false))
	l.cut = cut
	v := l.node(n, s, build, fail)
	l.cut = prev
	return v, cut
}

// cutFail fails, after the statements cleanup, when the body of a backtracking point cut.
func (l *lowerer) cutFail(cut string, fail string, cleanup ...code.Stmt) {
	if cut != "" {
		l.failIf(nil, use(cut), fail, cleanup...)
	}
}

var anchorExps = map[core.AnchorKind]core.ExpID{core.BeginInput: 1, core.EndInput: 2, core.BeginLine: 3, core.EndLine: 4}

// node lowers n, leaving fail when it fails, and returns its value: nil unless build.
func (l *lowerer) node(n core.Node, s *scope, build bool, fail string) code.Expr {
	if l.dead {
		return null()
	}
	if l.guarded != nil && n != l.guarded && n != l.guardedAlt {
		// Only the first node the alternative matches may consume the guard's character.
		l.guarded = nil
	}
	if l.outlines(n) {
		return l.outlined(n, s, build, fail)
	}
	switch n := n.(type) {
	case *core.Terminal:
		l.failIf(nil, code.Not(call(flow.OpSkipTerminal, n)), fail)
		return null()
	case *core.Lit:
		return l.lit(n, build, fail)
	case *core.Missing:
		return l.node(n.Body, s, build, fail)
	case *core.ClassNode:
		if n == l.guarded {
			return l.guardedClass(build)
		}
		if l.opts.ClassTableRanges > 0 && len(n.Class.Ranges) >= l.opts.ClassTableRanges {
			if !build {
				l.failIf(nil, code.Not(call(flow.OpMatchClass, n.Class)), fail)
				return null()
			}
			start := l.decl("x", flow.TypeInt)
			l.set(start, pos())
			l.failIf(nil, code.Not(call(flow.OpMatchClass, n.Class)), fail)
			v := l.decl("v", flow.TypeValue)
			l.set(v, l.match(use(start), call(flow.OpText, nil, use(start))))
			return use(v)
		}
		ch, size, ok := l.shared("ch", flow.TypeRune), l.shared("size", flow.TypeInt), l.ok()
		l.emit(peek(ch, size, ok))
		l.failIf(nil, code.Bin("||", code.Not(use(ok)), code.Not(call(flow.OpInClass, n.Class, use(ch)))), fail,
			do(flow.OpExpect, flow.Expectation{Exp: n.Class.Exp}, pos()))
		return l.single(size, build)
	case *core.Any:
		size, ok := l.shared("size", flow.TypeInt), l.ok()
		l.emit(peek("", size, ok))
		l.failIf(nil, code.Not(use(ok)), fail, do(flow.OpExpect, flow.Expectation{Exp: 0}, pos()))
		return l.single(size, build)
	case *core.Call:
		ok := l.ok()
		rc := call(flow.OpCallRule, flow.RuleCall{Rule: n.Rule, Level: n.Level})
		if !build {
			l.failIf(&code.Set{Dst: []code.Place{{}, {Name: ok}}, Values: []code.Expr{rc}}, code.Not(use(ok)), fail)
			return null()
		}
		v := l.decl("v", flow.TypeValue)
		l.failIf(&code.Set{Dst: []code.Place{{Name: v}, {Name: ok}}, Values: []code.Expr{rc}}, code.Not(use(ok)), fail)
		return use(v)
	case *core.Seq:
		return l.seq(n, s, build, fail)
	case *core.ClassRun:
		l.failIf(nil, code.Not(call(flow.OpSkipClasses, n)), fail)
		return null()
	case *core.Choice:
		return l.choice(n, s, build, fail)
	case *core.Stream:
		if l.opts.Stream {
			if !l.frame || !l.opts.Node || n.Repeat.Project != "" {
				panic("flow: a stream repetition is lowered in a framed Node function and is not projected")
			}
			l.streaming = n.Repeat
		}
		return l.repeat(n.Repeat, s, build, fail)
	case *core.Repeat:
		return l.repeat(n, s, build, fail)
	case *core.Scan:
		return l.scan(n, fail)
	case *core.Opt:
		m := l.mark(s, n.Body, true)
		var v string
		if build {
			v = l.decl("v", flow.TypeValue)
		}
		f, done := l.label(), l.label()
		bd := l.open(done)
		bf := l.open(f)
		x, cut := l.cutNode(n.Body, s, build, f)
		if build {
			l.set(v, l.shapeOne(n.Body, x, m.at(), false))
		}
		if l.used[f] {
			l.exit(done)
			l.close(bf)
			l.reset(m)
			l.cutFail(cut, fail)
			if build {
				l.set(v, null())
			}
			l.close(bd)
		} else {
			l.close(bf)
			l.close(bd)
		}
		if !build {
			return null()
		}
		return use(v)
	case *core.Look:
		if n.Neg {
			return l.not(n, s, fail)
		}
		// The lookahead goes back to its start and discards the errors recovered inside it, and
		// keeps its captures and bindings.
		start := l.decl("x", flow.TypeInt)
		var rec string
		if l.opts.Recovers && n.Body.Attrs().Eff.Set&core.EffRecover != 0 {
			rec = l.decl("x", flow.TypeInt)
			l.emit(&code.Set{Dst: []code.Place{{Name: start}, {Name: rec}}, Values: []code.Expr{pos(), call(flow.OpRecLen, nil)}})
		} else {
			l.set(start, pos())
		}
		back := func() {
			if rec == "" {
				l.setPos(use(start))
			} else {
				l.emit(do(flow.OpRestore, nil, use(start), use(rec)))
			}
		}
		l.emit(do(flow.OpSilent, flow.Silence{Delta: 1, Site: n.Site}))
		f, done := l.label(), l.label()
		bd := l.open(done)
		bf := l.open(f)
		prev := l.cut
		l.cut = ""
		l.node(n.Body, s, n.Body.Attrs().Want, f)
		l.cut = prev
		l.emit(do(flow.OpSilent, flow.Silence{Delta: -1, Site: n.Site, Ok: true}))
		back()
		if l.used[f] {
			l.exit(done)
			l.close(bf)
			l.emit(do(flow.OpSilent, flow.Silence{Delta: -1, Site: n.Site}))
			back()
			l.exit(fail)
			l.close(bd)
		} else {
			l.close(bf)
			l.close(bd)
		}
		return null()
	case *core.Text:
		var start string
		if build {
			start = l.decl("x", flow.TypeInt)
			l.set(start, pos())
		}
		l.node(n.Body, s, false, fail)
		if !build {
			return null()
		}
		v := l.decl("v", flow.TypeValue)
		l.set(v, l.match(use(start), call(flow.OpText, nil, use(start))))
		return use(v)
	case *core.Drop:
		l.node(n.Body, s, false, fail)
		return null()
	case *core.Capture:
		if l.frame {
			return l.frameCapture(n, s, build, fail)
		}
		k := l.capVar(s, n.Slot)
		var v code.Expr
		if n.Project != "" {
			v = l.projectRepeat(n.Body.(*core.Repeat), fail)
		} else {
			start := l.decl("x", flow.TypeInt)
			l.set(start, pos())
			v = l.shapeOne(n.Body, l.node(n.Body, s, true, fail), use(start), true)
		}
		l.set(k, v)
		if !build {
			return null()
		}
		return v
	case *core.Succeed:
		return null()
	case *core.FailNode:
		l.exit(fail)
		return null()
	case *core.Cut:
		if l.cut != "" {
			l.set(l.cut, code.BoolC(true))
		}
		return null()
	case *core.Anchor:
		l.failIf(nil, code.Not(call(flow.OpAtAnchor, n.Kind)), fail, do(flow.OpExpect, flow.Expectation{Exp: anchorExps[n.Kind]}, pos()))
		return null()
	case *core.Pred:
		c, args := l.termCaps(n.Term, s)
		l.failIf(nil, code.Not(call(flow.OpPred, flow.Predicate{Node: n, Capture: c}, args...)), fail)
		return null()
	case *core.BindNode:
		c, args := l.termCaps(n.Term, s)
		l.failIf(nil, code.Not(call(flow.OpBind, flow.Binding{Node: n, Capture: c}, args...)), fail)
		return null()
	case *core.Label:
		return l.label_(n, s, build, fail)
	case *core.Recover:
		if l.frame {
			return l.recover(n, s, build, fail)
		}
	}
	panic(fmt.Sprintf("flow: cannot lower %T", n))
}

// lit lowers a literal.
func (l *lowerer) lit(n *core.Lit, build bool, fail string) code.Expr {
	var start string
	if build {
		start = l.decl("x", flow.TypeInt)
		l.set(start, pos())
	}
	ok := l.ok()
	match := &code.Set{Dst: []code.Place{{}, {Name: ok}}, Values: []code.Expr{call(flow.OpMatchLit, n)}}
	if k := len([]rune(n.Text)); k > 0 && k <= l.opts.InlineLiterals && !n.Fold {
		// The code points compared in place; MatchLit records the expectation when they differ.
		if !l.dead {
			l.used[fail] = true
		}
		l.emit(&code.If{Cond: call(flow.OpHasLit, n), Then: &code.Block{Stmts: []code.Stmt{do(flow.OpAdvance, nil, num(k))}}, Else: &code.Block{Stmts: []code.Stmt{&code.If{Init: match, Cond: code.Not(use(ok)), Then: &code.Block{Stmts: []code.Stmt{l.jump(fail)}}}}}})
	} else {
		l.failIf(match, code.Not(use(ok)), fail)
	}
	if !build {
		return null()
	}
	v := l.decl("v", flow.TypeValue)
	var text code.Expr = call(flow.OpStr, flow.Chars(n.Text))
	switch {
	case n.Fold:
		// The input matched may differ in case from the literal: the node holds the input.
		text = call(flow.OpText, nil, use(start))
	case strings.ContainsRune(n.Text, '�'):
		text = call(flow.OpLiteralText, flow.Chars(n.Text), use(start))
	}
	l.set(v, l.match(use(start), text))
	return use(v)
}

// single consumes the character of the given size, after a successful peek.
func (l *lowerer) single(size string, build bool) code.Expr {
	if !build {
		l.emit(do(flow.OpAdvance, nil, use(size)))
		return null()
	}
	start, v := l.decl("x", flow.TypeInt), l.decl("v", flow.TypeValue)
	l.set(start, pos())
	l.emit(do(flow.OpAdvance, nil, use(size)))
	l.set(v, l.match(use(start), call(flow.OpText, nil, use(start))))
	return use(v)
}

// guardedClass consumes the character the guard read for the class l.guarded.
func (l *lowerer) guardedClass(build bool) code.Expr {
	if !build {
		l.emit(do(flow.OpAdvance, nil, use(l.guardedSize)))
		return null()
	}
	start, v := l.decl("x", flow.TypeInt), l.decl("v", flow.TypeValue)
	l.set(start, pos())
	l.emit(do(flow.OpAdvance, nil, use(l.guardedSize)))
	l.set(v, l.match(use(start), call(flow.OpText, nil, use(start))))
	return use(v)
}

func (l *lowerer) seq(n *core.Seq, s *scope, build bool, fail string) code.Expr {
	var start, kids string
	if build {
		start, kids = l.decl("x", flow.TypeInt), l.decl("x", flow.TypeVals)
		l.set(start, pos())
		l.set(kids, call(flow.OpNewVals, flow.Room(n.Attrs().Shape.K)))
	}
	// The value is the tuple node of the elements, flattened through nested sequences.
	for _, it := range n.Items {
		v := l.node(it, s, build, fail)
		if !build {
			continue
		}
		switch sh := it.Attrs().Shape; {
		case sh.Elems:
			l.emit(code.Assign(kids, call(flow.OpAppend, flow.Spread(true), use(kids), v)))
		case sh.K == 1:
			l.emit(code.Assign(kids, call(flow.OpAppend, flow.Spread(false), use(kids), v)))
		}
	}
	if !build {
		return null()
	}
	v := l.decl("v", flow.TypeValue)
	l.set(v, l.branch(false, use(start), use(kids)))
	return use(v)
}

// leadingClass returns the class that begins the alternative body n, also inside a Terminal.
func leadingClass(n core.Node) *core.ClassNode {
	if t, ok := n.(*core.Terminal); ok {
		n = t.Body
	}
	if s, ok := n.(*core.Seq); ok && len(s.Items) > 0 {
		n = s.Items[0]
	}
	c, _ := n.(*core.ClassNode)
	return c
}

func (l *lowerer) choice(n *core.Choice, s *scope, build bool, fail string) code.Expr {
	m := l.mark(s, n, true)
	var v string
	if build {
		v = l.decl("v", flow.TypeValue)
	}
	done := l.label()
	// The guards keep the size of the character they read when an alternative begins with a
	// class.
	var size string
	if l.opts.GuardedClasses {
		for _, a := range n.Alts {
			if leadingClass(a.Body) != nil {
				size = l.decl("x", flow.TypeInt)
				break
			}
		}
	}
	bd := l.open(done)
	var ch, more string
	for _, a := range n.Alts {
		if l.dead {
			break // the alternatives after one that cannot fail are never tried
		}
		var bs *code.Block
		var guarded *core.ClassNode
		if a.Guard != nil {
			// The alternative is skipped when the next character cannot begin it.
			if ch == "" {
				ch, more = l.decl("x", flow.TypeRune), l.decl("x", flow.TypeBool)
				l.emit(peek(ch, size, more))
			}
			if size != "" && a.GuardedClass {
				guarded = leadingClass(a.Body)
			}
			skip := l.label()
			bs = l.open(skip)
			pre := []code.Stmt{do(flow.OpExpect, flow.Expectation{Exp: a.Guard.Exp}, pos())}
			if l.opts.Observe {
				pre = append(pre, do(flow.OpObserveSkip, a.Body))
			}
			l.failIf(nil, call(flow.OpGuardFails, a.Guard, use(ch), use(more)), skip, pre...)
		}
		f := l.label()
		bf := l.open(f)
		l.guarded, l.guardedAlt, l.guardedSize = guarded, a.Body, size
		x, cut := l.cutNode(a.Body, s, build, f)
		if build {
			l.set(v, l.shapeOne(a.Body, x, m.at(), false))
		}
		l.exit(done)
		l.close(bf)
		l.reset(m)
		l.cutFail(cut, fail)
		if bs != nil {
			l.close(bs)
		}
	}
	l.exit(fail)
	l.close(bd)
	if !build {
		return null()
	}
	return use(v)
}

// shapeOne converts the value x of n, matched from start, to its value where it takes one
// element; with capture, where it is captured.
func (l *lowerer) shapeOne(n core.Node, x code.Expr, start code.Expr, capture bool) code.Expr {
	sh := n.Attrs().Shape
	if sh.Mode == core.SinglePass && !sh.Elems {
		return x
	}
	v := l.decl("v", flow.TypeValue)
	if capture {
		l.set(v, call(flow.OpShapeCapture, sh, x, start))
	} else {
		l.emit(&code.Set{Dst: []code.Place{{Name: v}, {}}, Values: []code.Expr{call(flow.OpShapeSingle, sh, x, start)}})
	}
	return use(v)
}

// not lowers a negative lookahead.
func (l *lowerer) not(n *core.Look, s *scope, fail string) code.Expr {
	m := l.mark(s, n.Body, true)
	l.emit(do(flow.OpSilent, flow.Silence{Delta: 1, Site: n.Site}))
	f := l.label()
	bf := l.open(f)
	prev := l.cut
	l.cut = ""
	l.node(n.Body, s, false, f)
	l.cut = prev
	l.emit(do(flow.OpSilent, flow.Silence{Delta: -1, Site: n.Site, Ok: true}))
	l.reset(m)
	l.exit(fail)
	l.close(bf)
	l.emit(do(flow.OpSilent, flow.Silence{Delta: -1, Site: n.Site}))
	l.reset(m)
	return null()
}

func (l *lowerer) repeat(n *core.Repeat, s *scope, build bool, fail string) code.Expr {
	if n.Site != 0 {
		return l.skipRepeat(n, s, fail)
	}
	if n.ScanSite != 0 {
		return l.scanSiteRepeat(n, s, build, fail)
	}
	if l.frame {
		return l.frameRepeat(n, s, build, fail)
	}
	// An element with captures has its own scope per iteration.
	own := n.Scope != nil
	es := s
	if own {
		es = newScope(n.Scope)
	}
	var start, base string
	if build {
		start, base = l.decl("x", flow.TypeInt), l.decl("x", flow.TypeInt)
		l.emit(&code.Set{Dst: []code.Place{{Name: start}, {Name: base}}, Values: []code.Expr{pos(), call(flow.OpKidsLen, nil)}})
	}
	run := l.runBegin(n, build, start)
	count := l.decl("x", flow.TypeInt)
	l.set(count, num(0))
	lp := l.newLoop(n.Max)
	run.visit(l, count, lp)
	m := l.mark(s, n.Body, !own)
	if own {
		l.clearScope(es, n.Body)
	}
	f := l.label()
	bf := l.open(f)
	v, cut := l.cutNode(n.Body, es, build, f)
	if build {
		sv := l.decl("v", flow.TypeValue)
		l.set(sv, call(flow.OpScopeValue, n.Body.Attrs().Shape, v, use(m.pos)))
		v = use(sv)
	}
	if own && build {
		attached := l.decl("v", flow.TypeValue)
		l.set(attached, attach(v, use(m.pos), &flow.Names{List: core.SlotNames(es.slots)}, l.scopeVals(es)))
		v = use(attached)
	}
	l.emit(inc(count))
	if build {
		l.emit(run.push(do(flow.OpPushKid, nil, v), v))
	}
	l.emit(&code.If{Cond: code.Bin("==", pos(), use(m.pos)), Then: &code.Block{Stmts: []code.Stmt{l.jump(lp.Label)}}})
	if l.used[f] {
		l.cont(lp.Label)
		l.close(bf)
		l.reset(m)
		if build {
			l.cutFail(cut, fail, run.drop(do(flow.OpDropKids, nil, use(base)))...)
		} else {
			l.cutFail(cut, fail, run.drop(nil)...)
		}
		l.exitLoop(lp)
	} else {
		l.close(bf)
	}
	l.endLoop()
	if n.Min > 0 {
		cond := code.Bin("<", use(count), num(n.Min))
		if build {
			l.failIf(nil, cond, fail, run.drop(do(flow.OpDropKids, nil, use(base)))...)
		} else {
			l.failIf(nil, cond, fail, run.drop(nil)...)
		}
	}
	if !build {
		run.end(l, nil)
		return null()
	}
	list := l.decl("v", flow.TypeValue)
	run.end(l, func(hot bool) {
		var kids code.Expr = call(flow.OpKids, nil, use(base))
		if hot {
			kids = call(flow.OpSkipKids, nil)
		}
		l.set(list, l.branch(true, use(start), kids))
	})
	return use(list)
}

// runSite is the lowering of a repetition that is a run site (core.Repeat.Run), whose walks keep
// records once the site is hot (flow.runHot); nil for any other repetition, whose methods then emit
// what a plain repetition does. A walk tests hot once, when it begins: a hot walk is a walk of the
// site (flow.runBegin), and a walk of a site that is not hot is an ordinary repetition that tells
// the site where it walked (flow.runWalked). The engine's backends walk the same way.
type runSite struct {
	n          *core.Repeat
	hot, start string
}

// runBegin begins the lowering of the repetition n, whose walk begins at the position, held in
// start if not empty (the variable of a walk that builds a value). It returns nil if n is no run
// site.
func (l *lowerer) runBegin(n *core.Repeat, build bool, start string) *runSite {
	if n.Run == 0 || n.Project != "" {
		return nil
	}
	r := &runSite{n: n, hot: l.decl("x", flow.TypeBool), start: start}
	if start == "" {
		r.start = l.decl("x", flow.TypeInt)
		l.set(r.start, pos())
	}
	l.set(r.hot, call(flow.OpRunHot, flow.Site(n.Run)))
	walk := flow.RunWalk{Site: n.Run, Build: build, KeyEnv: !n.RunEnvFree}
	l.emit(&code.If{Cond: use(r.hot), Then: &code.Block{Stmts: []code.Stmt{do(flow.OpRunBegin, walk)}}})
	return r
}

// visit emits the beginning of an iteration of a hot walk, after the charge of the iteration: from
// the minimum on, the walk takes the rest from the record of the position, if there is one, and
// leaves the loop lp; otherwise the iteration is a visit of the walk.
func (r *runSite) visit(l *lowerer, count string, lp *code.Loop) {
	if r == nil {
		return
	}
	hit := []code.Stmt{&code.If{Cond: call(flow.OpSkipHit, nil), Then: &code.Block{Stmts: []code.Stmt{l.jump(lp.Label)}}}}
	if r.n.Min > 0 {
		hit = []code.Stmt{&code.If{Cond: code.Bin(">=", use(count), num(r.n.Min)), Then: &code.Block{Stmts: hit}}}
	}
	l.emit(&code.If{Cond: use(r.hot), Then: &code.Block{Stmts: append(hit, do(flow.OpSkipVisit, nil))}})
}

// push returns the statement that adds the value v of an iteration: plain, the statement keep.
func (r *runSite) push(keep code.Stmt, v code.Expr) code.Stmt {
	if r == nil {
		return keep
	}
	return &code.If{Cond: use(r.hot), Then: &code.Block{Stmts: []code.Stmt{do(flow.OpSkipValue, nil, v)}}, Else: &code.Block{Stmts: []code.Stmt{keep}}}
}

// drop returns the statements that end a walk that fails: plain, the statement plain (none if nil).
func (r *runSite) drop(plain code.Stmt) []code.Stmt {
	var els []code.Stmt
	if plain != nil {
		els = []code.Stmt{plain}
	}
	if r == nil {
		return els
	}
	b := &code.If{Cond: use(r.hot), Then: &code.Block{Stmts: []code.Stmt{do(flow.OpSkipEnd, nil)}}}
	if els != nil {
		b.Else = &code.Block{Stmts: els}
	}
	return []code.Stmt{b}
}

// end emits the end of a walk that matched. list, if not nil, emits the list of the values, from
// flow.skipKids in a hot walk (hot) and from the stack in a plain one. A hot walk that builds no
// value ends with flow.skipEnd; a plain walk tells the site where it walked.
func (r *runSite) end(l *lowerer, list func(hot bool)) {
	if r == nil {
		if list != nil {
			list(false)
		}
		return
	}
	b := &code.If{Cond: use(r.hot), Then: &code.Block{}, Else: &code.Block{}}
	l.emit(b)
	l.stack = append(l.stack, &b.Then.Stmts)
	if list != nil {
		list(true)
	} else {
		l.emit(do(flow.OpSkipEnd, nil))
	}
	l.stack[len(l.stack)-1] = &b.Else.Stmts
	l.emit(do(flow.OpRunWalked, flow.Site(r.n.Run), use(r.start)))
	if list != nil {
		list(false)
	}
	l.stack = l.stack[:len(l.stack)-1]
}

// scanSiteRepeat lowers the scan site n, a repetition of one character without a maximum or a scope of
// its own, in a local or a framed function. A walk that matches ScanHotSpan characters takes the
// rest from the record of its start or goes on (ScanReach), and records itself when it ends
// (ScanEnd; ScanKids for a walk that builds a value, whose records are those of the next site).
func (l *lowerer) scanSiteRepeat(n *core.Repeat, s *scope, build bool, fail string) code.Expr {
	if n.Max >= 0 || n.Scope != nil || n.Project != "" {
		panic("flow: a scan site has no maximum, no scope of its own and no projection")
	}
	site := n.ScanSite
	start, count := l.decl("x", flow.TypeInt), l.decl("x", flow.TypeInt)
	var base string
	if build {
		site++
		base = l.decl("x", flow.TypeInt)
		l.emit(&code.Set{Dst: []code.Place{{Name: start}, {Name: base}, {Name: count}}, Values: []code.Expr{pos(), call(flow.OpKidsLen, nil), num(0)}})
	} else {
		l.emit(&code.Set{Dst: []code.Place{{Name: start}, {Name: count}}, Values: []code.Expr{pos(), num(0)}})
	}
	lp := l.newLoop(-1)
	m := l.mark(s, n.Body, true)
	f := l.label()
	bf := l.open(f)
	v, cut := l.cutNode(n.Body, s, build, f)
	if build {
		sv := l.decl("v", flow.TypeValue)
		l.set(sv, call(flow.OpScopeValue, n.Body.Attrs().Shape, v, m.at()))
		l.emit(do(flow.OpPushKid, nil, use(sv)))
	}
	l.emit(inc(count))
	l.scanReach(site, start, count)
	if l.used[f] {
		l.cont(lp.Label)
		l.close(bf)
		l.reset(m)
		if build {
			l.cutFail(cut, fail, do(flow.OpDropKids, nil, use(base)))
		} else {
			l.cutFail(cut, fail)
		}
		l.exitLoop(lp)
	} else {
		l.close(bf)
	}
	l.endLoop()
	if !build {
		l.scanEnd(site, start, count)
	}
	if n.Min > 0 {
		cond := code.Bin("<", use(count), num(n.Min))
		if build {
			l.failIf(nil, cond, fail, do(flow.OpDropKids, nil, use(base)))
		} else {
			l.failIf(nil, cond, fail)
		}
	}
	if !build {
		return null()
	}
	list := l.decl("v", flow.TypeValue)
	l.set(list, l.branch(true, use(start), call(flow.OpScanKids, flow.Site(site), use(start), use(base), use(count))))
	return use(list)
}

// scanReach emits the test of a walk of the scan site that began at start after its count was
// raised: at ScanHotSpan characters, the walk takes the count from ScanReach.
func (l *lowerer) scanReach(site int, start, count string) {
	l.emit(&code.If{Cond: code.Bin("==", use(count), num(flow.ScanHotSpan)), Then: &code.Block{Stmts: []code.Stmt{
		&code.Set{Dst: []code.Place{{Name: count}}, Values: []code.Expr{call(flow.OpScanReach, flow.Site(site), use(start), use(count))}},
	}}})
}

// scanEnd emits the end of a value-free walk of the scan site that began at start: a walk that
// reached ScanHotSpan characters records itself or notes that it walked (ScanEnd).
func (l *lowerer) scanEnd(site int, start, count string) {
	l.emit(&code.If{Cond: code.Bin("<", num(flow.ScanHotSpan-1), use(count)), Then: &code.Block{Stmts: []code.Stmt{
		do(flow.OpScanEnd, flow.Site(site), use(start), use(count)),
	}}})
}

// skipRepeat lowers the skip-run site n, a value-free repetition without a maximum or a scope of
// its own, whose iterations are the visits of a walk (SkipBegin): from the minimum on, an iteration
// first takes the rest of the walk from the record of the position, if there is one.
func (l *lowerer) skipRepeat(n *core.Repeat, s *scope, fail string) code.Expr {
	if n.Max >= 0 || n.Scope != nil {
		panic("flow: a skip-run site has no maximum and no scope of its own")
	}
	l.emit(do(flow.OpSkipBegin, flow.Site(n.Site)))
	count := l.decl("x", flow.TypeInt)
	l.set(count, num(0))
	lp := l.newLoop(-1)
	hit := []code.Stmt{&code.If{Cond: call(flow.OpSkipHit, nil), Then: &code.Block{Stmts: []code.Stmt{l.jump(lp.Label)}}}}
	if n.Min > 0 {
		hit = []code.Stmt{&code.If{Cond: code.Bin(">=", use(count), num(n.Min)), Then: &code.Block{Stmts: hit}}}
	}
	for _, st := range hit {
		l.emit(st)
	}
	l.emit(do(flow.OpSkipVisit, nil))
	m := l.mark(s, n.Body, false)
	f := l.label()
	bf := l.open(f)
	_, cut := l.cutNode(n.Body, s, false, f)
	l.emit(inc(count))
	l.emit(&code.If{Cond: code.Bin("==", pos(), m.at()), Then: &code.Block{Stmts: []code.Stmt{l.jump(lp.Label)}}})
	if l.used[f] {
		l.cont(lp.Label)
		l.close(bf)
		l.reset(m)
		l.cutFail(cut, fail, do(flow.OpSkipEnd, nil))
		l.exitLoop(lp)
	} else {
		l.close(bf)
	}
	l.endLoop()
	l.emit(do(flow.OpSkipEnd, nil))
	if n.Min > 0 {
		l.failIf(nil, code.Bin("<", use(count), num(n.Min)), fail)
	}
	return null()
}

// newLoop begins a loop of at most max iterations (unbounded if max < 0). Each iteration charges one
// step first, whatever its body is.
func (l *lowerer) newLoop(max int) *code.Loop {
	lp := &code.Loop{Label: l.loopLabel()}
	if max >= 0 {
		c := l.decl("x", flow.TypeInt)
		lp.Init = code.Assign(c, num(0))
		lp.Cond = code.Lt(use(c), num(max))
		lp.Post = inc(c)
	}
	l.loop(lp)
	l.step()
	return lp
}

// exitLoop leaves the loop lp from its body.
func (l *lowerer) exitLoop(lp *code.Loop) {
	l.emit(l.jump(lp.Label))
	l.dead = true
}

// clearScope sets the capture slots of the element n in its own scope es to nil.
func (l *lowerer) clearScope(es *scope, n core.Node) {
	for _, slot := range n.Attrs().Eff.Slots {
		l.set(l.capVar(es, slot), null())
	}
}

// scopeVals returns the variables of the slots of es, in slot order.
func (l *lowerer) scopeVals(es *scope) []string {
	vals := make([]string, len(es.slots))
	for i := range es.slots {
		vals[i] = l.capVar(es, i)
	}
	return vals
}

// scan lowers a value-free repetition of one character.
func (l *lowerer) scan(n *core.Scan, fail string) code.Expr {
	var ch string
	if n.Class != nil {
		ch = l.shared("ch", flow.TypeRune)
	}
	size, ok := l.shared("size", flow.TypeInt), l.ok()
	var start, count string
	if n.Site != 0 {
		start = l.decl("x", flow.TypeInt)
		l.set(start, pos())
	}
	if n.Max >= 0 || n.Min > 0 || n.Site != 0 {
		count = l.decl("x", flow.TypeInt)
		l.set(count, num(0))
	}
	lp := &code.Loop{Label: l.loopLabel()}
	if n.Max >= 0 {
		lp.Cond = code.Bin("<", use(count), num(n.Max))
	}
	l.loop(lp)
	l.emit(peek(ch, size, ok))
	var stop code.Expr = code.Not(use(ok))
	exp := core.ExpID(0)
	if n.Class != nil {
		stop = code.Bin("||", stop, code.Not(call(flow.OpInClass, n.Class, use(ch))))
		exp = n.Class.Exp
	}
	l.emit(&code.If{Cond: stop, Then: &code.Block{Stmts: []code.Stmt{do(flow.OpExpect, flow.Expectation{Exp: exp}, pos()), l.jump(lp.Label)}}})
	l.emit(do(flow.OpAdvance, nil, use(size)))
	if count != "" {
		l.emit(inc(count))
	}
	if n.Site != 0 {
		l.scanReach(n.Site, start, count)
	}
	l.endLoop()
	if n.Site != 0 {
		l.scanEnd(n.Site, start, count)
	}
	if n.Min > 0 {
		l.failIf(nil, code.Bin("<", use(count), num(n.Min)), fail)
	}
	return null()
}

// projectRepeat lowers a repetition that gathers the values of the capture Project of its
// elements.
func (l *lowerer) projectRepeat(n *core.Repeat, fail string) code.Expr {
	es := newScope(n.Scope)
	start, base, count := l.decl("x", flow.TypeInt), l.decl("x", flow.TypeInt), l.decl("x", flow.TypeInt)
	l.emit(&code.Set{Dst: []code.Place{{Name: start}, {Name: base}, {Name: count}}, Values: []code.Expr{pos(), call(flow.OpKidsLen, nil), num(0)}})
	lp := l.newLoop(n.Max)
	m := l.mark(es, n.Body, false)
	l.clearScope(es, n.Body)
	f := l.label()
	bf := l.open(f)
	_, cut := l.cutNode(n.Body, es, false, f)
	field := -1
	for i, sl := range es.slots {
		if sl.Name == n.Project {
			field = i
		}
	}
	fv := l.capVar(es, field)
	l.emit(inc(count))
	l.emit(do(flow.OpPushKid, nil, use(fv)))
	l.emit(&code.If{Cond: code.Bin("==", pos(), use(m.pos)), Then: &code.Block{Stmts: []code.Stmt{l.jump(lp.Label)}}})
	if l.used[f] {
		l.cont(lp.Label)
		l.close(bf)
		l.reset(m)
		l.cutFail(cut, fail, do(flow.OpDropKids, nil, use(base)))
		l.exitLoop(lp)
	} else {
		l.close(bf)
	}
	l.endLoop()
	if n.Min > 0 {
		l.failIf(nil, code.Bin("<", use(count), num(n.Min)), fail, do(flow.OpDropKids, nil, use(base)))
	}
	v := l.decl("v", flow.TypeValue)
	l.set(v, l.branch(true, use(start), call(flow.OpKids, nil, use(base))))
	return use(v)
}

// termCaps returns the captures of the scope s for a term of a predicate or a binding, with their
// arguments (capArgs), making the variables of the slots it refers to. In a framed function, and
// where the scope has no slots, the term reads the current frame and has no arguments.
func (l *lowerer) termCaps(t core.ActionExpr, s *scope) (flow.Capture, []code.Expr) {
	c := flow.Capture{Names: core.SlotNames(s.slots), Frame: true}
	if l.frame {
		return c, nil
	}
	core.WalkActionExpr(t, func(t core.ActionExpr) {
		if r, ok := t.(*core.SlotRef); ok {
			l.capVar(s, r.Slot)
		}
	})
	if len(s.vars) == 0 {
		return c, nil
	}
	c.Frame = false
	return c, capArgs(t, s)
}

// capArgs returns the arguments of the captures of the scope s for the term t, one per slot: the
// variable of each slot that t reads, and the null value for the others.
func capArgs(t core.ActionExpr, s *scope) []code.Expr {
	read := map[int]bool{}
	core.WalkActionExpr(t, func(t core.ActionExpr) {
		if r, ok := t.(*core.SlotRef); ok {
			read[r.Slot] = true
		}
	})
	out := make([]code.Expr, len(s.vars))
	for k, v := range s.vars {
		if read[k] && v != "" {
			out[k] = use(v)
		} else {
			out[k] = null()
		}
	}
	return out
}

// label_ lowers @error and @diagnostic: the expectations recorded inside are isolated, so that
// the label changes only those of a failed body.
func (l *lowerer) label_(n *core.Label, s *scope, build bool, fail string) code.Expr {
	mk, far, exps := l.decl("x", flow.TypeExpMark), l.decl("x", flow.TypeInt), l.decl("x", flow.TypeExpIDs)
	l.set(mk, call(flow.OpIsolate, nil))
	f, done := l.label(), l.label()
	bd := l.open(done)
	bf := l.open(f)
	v := l.node(n.Body, s, build, f)
	unisolate := &code.Set{Dst: []code.Place{{Name: far}, {Name: exps}}, Values: []code.Expr{call(flow.OpUnisolate, nil, use(mk))}}
	l.emit(unisolate)
	l.emit(do(flow.OpMergeExpected, nil, use(far), use(exps)))
	if l.used[f] {
		l.exit(done)
		l.close(bf)
		l.emit(&code.Set{Dst: []code.Place{{Name: far}, {Name: exps}}, Values: []code.Expr{call(flow.OpUnisolate, nil, use(mk))}})
		if n.Diagnostic {
			l.emit(do(flow.OpMergeRenamed, n.Exp, use(far), use(exps)))
		} else {
			l.emit(do(flow.OpExpect, flow.Expectation{Exp: n.Exp, Message: true}, use(far)))
		}
		l.exit(fail)
		l.close(bd)
	} else {
		l.close(bf)
		l.close(bd)
	}
	return v
}

// finish makes the rule's value from the body's value body and the captures s, leaving it in
// the Result variable.
func (l *lowerer) finish(r *core.Rule, s *scope, body code.Expr) {
	f := l.at
	var owned code.Expr
	if r.Finish.NodeType != "" && (r.Finish.Kind == core.FinishSplice || r.Finish.Kind == core.FinishScope) {
		bv := l.shared("bv", flow.TypeValue)
		l.set(bv, call(flow.OpFreshNode, nil, body))
		owned = use(bv)
	}
	switch r.Finish.Kind {
	case core.FinishNone:
		l.set(f.Result, null())
	case core.FinishAction:
		l.scopeVals(s)
		c := flow.Capture{Names: core.SlotNames(s.slots)}
		args := append([]code.Expr{use(f.Ctx)}, capArgs(r.Finish.Action, s)...)
		l.set(f.Ctx, call(flow.OpUseCtx, nil, use(f.Start)))
		l.emit(do(flow.OpSetWhere, flow.Chars(r.Where)))
		l.set(f.Result, call(flow.OpEval, flow.Action{Term: r.Finish.Action, Capture: c}, args...))
		switch r.Finish.Action.(type) {
		case *core.New, *core.Convert:
			// A struct made with the rule's range; structs made inside it are dropped.
			nested := 0
			core.WalkActionExpr(r.Finish.Action, func(t core.ActionExpr) {
				switch t.(type) {
				case *core.New, *core.Convert:
					nested++
				}
			})
			if nested > 1 {
				l.emit(do(flow.OpDropCreated, nil, use(f.Ctx)))
			}
		default:
			l.set(f.Result, call(flow.OpCtxFinish, nil, use(f.Ctx), use(f.Result)))
		}
	case core.FinishSplice:
		l.set(f.Result, body)
		l.emit(do(flow.OpUnfresh, nil, use(f.Result)))
	case core.FinishScope:
		l.set(f.Result, call(flow.OpScopeValue, r.Finish.Shape, body, use(f.Start)))
		if len(s.slots) > 0 {
			l.set(f.Result, attach(use(f.Result), use(f.Start), &flow.Names{List: core.SlotNames(s.slots)}, l.scopeVals(s)))
		}
		l.emit(do(flow.OpUnfresh, nil, use(f.Result)))
	}
	if r.Finish.NodeType != "" {
		args := []code.Expr{use(f.Result)}
		if owned != nil {
			args = append(args, owned)
		}
		l.set(f.Result, call(flow.OpConvertResult, flow.Conversion{Rule: r.ID, Owned: owned != nil}, args...))
	}
}

// frameCapture lowers a capture in a framed function: it sets the slot of the current frame.
func (l *lowerer) frameCapture(n *core.Capture, s *scope, build bool, fail string) code.Expr {
	var v code.Expr
	if n.Project != "" {
		v = l.frameRepeat(n.Body.(*core.Repeat), s, true, fail)
	} else {
		start := l.decl("x", flow.TypeInt)
		l.set(start, pos())
		v = l.shapeOne(n.Body, l.node(n.Body, s, true, fail), use(start), true)
	}
	l.emit(do(flow.OpSetCapture, flow.Slot(n.Slot), v))
	if !build {
		return null()
	}
	return v
}

// frameRepeat lowers a repetition in a framed function. An element with captures of its own runs
// under a new frame, which a projected repetition (Project) makes once and clears for each
// element.
func (l *lowerer) frameRepeat(n *core.Repeat, s *scope, build bool, fail string) code.Expr {
	field := n.Project
	own := n.Scope != nil
	es := s
	var start, base, frame, prev string
	var names *flow.Names
	if build {
		start, base = l.decl("x", flow.TypeInt), l.decl("x", flow.TypeInt)
		l.emit(&code.Set{Dst: []code.Place{{Name: start}, {Name: base}}, Values: []code.Expr{pos(), call(flow.OpKidsLen, nil)}})
	}
	if own {
		es = newScope(n.Scope)
		frame, prev = l.decl("x", flow.TypeFrame), l.decl("x", flow.TypeFrame)
		names = &flow.Names{List: core.SlotNames(es.slots), Declared: true}
		if field != "" {
			l.set(frame, call(flow.OpNewFrame, names))
		}
	}
	stream := n == l.streaming
	var run *runSite
	if !stream {
		run = l.runBegin(n, build && field == "", start)
	}
	count := l.decl("x", flow.TypeInt)
	l.set(count, num(0))
	if stream {
		l.emit(&code.If{Cond: call(flow.OpStreamOn, nil), Then: &code.Block{Stmts: []code.Stmt{do(flow.OpStreamStart, nil)}}})
	}
	lp := l.newLoop(n.Max)
	run.visit(l, count, lp)
	if field != "" {
		l.emit(do(flow.OpClearVals, nil, use(frame)))
	}
	m := l.mark(s, n.Body, true)
	frameReg := call(flow.OpFrame, nil)
	if own {
		l.set(prev, frameReg)
		if field == "" {
			l.set(frame, call(flow.OpNewFrame, names))
		}
		l.setFrame(use(frame))
	}
	f := l.label()
	bf := l.open(f)
	v, cut := l.cutNode(n.Body, es, build && field == "", f)
	if build && field == "" {
		sv := l.decl("v", flow.TypeValue)
		l.set(sv, call(flow.OpScopeValue, n.Body.Attrs().Shape, v, m.at()))
		v = use(sv)
	}
	if own {
		l.setFrame(use(prev))
		if build {
			if field != "" {
				v = call(flow.OpFrameVal, flow.Slot(slotOf(es.slots, field)), use(frame))
			} else {
				attached := l.decl("v", flow.TypeValue)
				l.set(attached, call(flow.OpAttachFrame, flow.Attach{Names: names, Frame: true}, v, m.at(), use(frame)))
				v = use(attached)
			}
		}
		l.emit(do(flow.OpDeclareNames, names))
		if field == "" && !l.opts.Node {
			// The Node runtime does not recycle capture frames or keep a trail of them.
			l.emit(do(flow.OpDropTrail, nil, use(m.state)))
			l.emit(do(flow.OpFreeFrame, nil, use(frame)))
		}
	}
	l.emit(inc(count))
	if stream {
		// The element is delivered and committed, not retained.
		var x code.Expr = null()
		if build {
			x = v
		}
		l.emit(&code.If{Cond: call(flow.OpStreamOn, nil), Then: &code.Block{Stmts: []code.Stmt{
			do(flow.OpEmitStream, nil, x),
			&code.If{Cond: code.Bin("==", pos(), m.at()), Then: &code.Block{Stmts: []code.Stmt{l.jump(lp.Label)}}},
			&code.Continue{Label: lp.Label},
		}}})
	}
	if build {
		l.emit(run.push(do(flow.OpPushKid, nil, v), v))
	}
	l.emit(&code.If{Cond: code.Bin("==", pos(), m.at()), Then: &code.Block{Stmts: []code.Stmt{l.jump(lp.Label)}}})
	if l.used[f] {
		l.cont(lp.Label)
		l.close(bf)
		if own {
			l.setFrame(use(prev))
		}
		l.reset(m)
		if build {
			l.cutFail(cut, fail, run.drop(do(flow.OpDropKids, nil, use(base)))...)
		} else {
			l.cutFail(cut, fail, run.drop(nil)...)
		}
		l.exitLoop(lp)
	} else {
		l.close(bf)
	}
	l.endLoop()
	if n.Min > 0 {
		cond := code.Bin("<", use(count), num(n.Min))
		if build {
			l.failIf(nil, cond, fail, run.drop(do(flow.OpDropKids, nil, use(base)))...)
		} else {
			l.failIf(nil, cond, fail, run.drop(nil)...)
		}
	}
	if !build {
		run.end(l, nil)
		return null()
	}
	list := l.decl("v", flow.TypeValue)
	run.end(l, func(hot bool) {
		var kids code.Expr = call(flow.OpKids, nil, use(base))
		if hot {
			kids = call(flow.OpSkipKids, nil)
		}
		l.set(list, l.branch(true, use(start), kids))
	})
	return use(list)
}

// slotOf returns the index of the slot name among slots.
func slotOf(slots []core.Slot, name string) int {
	for i, sl := range slots {
		if sl.Name == name {
			return i
		}
	}
	panic(fmt.Sprintf("flow: no capture slot %s", name))
}

// recover lowers #recover in a framed function. The expectations recorded inside are isolated;
// when the body fails, the parser state goes back to the mark, the skip expression consumes at
// least one character, and the error is recorded and is the value.
func (l *lowerer) recover(n *core.Recover, s *scope, build bool, fail string) code.Expr {
	l.emit(&code.Comment{Text: "Recovery inlined under the runtime-owned frame."})
	m := l.mark(s, n, true)
	mk, far, exps := l.decl("x", flow.TypeExpMark), l.decl("x", flow.TypeInt), l.decl("x", flow.TypeExpIDs)
	l.set(mk, call(flow.OpIsolate, nil))
	f, done := l.label(), l.label()
	var v string
	if build {
		v = l.decl("v", flow.TypeValue)
	}
	bd := l.open(done)
	bf := l.open(f)
	inner := l.node(n.Body, s, build, f)
	if build {
		l.set(v, l.shapeOne(n.Body, inner, m.at(), true))
	}
	l.emit(&code.Set{Dst: []code.Place{{Name: far}, {Name: exps}}, Values: []code.Expr{call(flow.OpUnisolate, nil, use(mk))}})
	l.emit(do(flow.OpMergeExpected, nil, use(far), use(exps)))
	if l.used[f] {
		l.exit(done)
		l.close(bf)
		l.emit(&code.Set{Dst: []code.Place{{Name: far}, {Name: exps}}, Values: []code.Expr{call(flow.OpUnisolate, nil, use(mk))}})
		l.set(exps, call(flow.OpKeep, nil, use(exps)))
		l.reset(m)
		skipFail := l.label()
		bs := l.open(skipFail)
		l.node(n.Skip, s, false, skipFail)
		l.failIf(nil, code.Bin("==", pos(), m.at()), skipFail)
		err := l.decl("x", flow.TypeError)
		l.set(err, call(flow.OpMakeError, nil, use(far), use(exps)))
		l.emit(do(flow.OpAppendError, nil, use(err)))
		if build {
			l.set(v, l.errorValue(m.at(), use(err)))
		}
		l.exit(done)
		l.close(bs)
		l.reset(m)
		l.emit(do(flow.OpMergeExpected, nil, use(far), use(exps)))
		l.exit(fail)
		l.close(bd)
	} else {
		l.close(bf)
		l.close(bd)
	}
	if !build {
		return null()
	}
	return use(v)
}

// errorValue returns the value of the recovered error err over the input from start, built for the runtime of
// the function.
func (l *lowerer) errorValue(start, err code.Expr) code.Expr {
	if l.opts.Node {
		return call(flow.OpNewErrorNode, nil, start, err)
	}
	return call(flow.OpErrorValue, nil, start, err)
}

// match returns the value of a match from start with the text, built for the runtime of the function.
func (l *lowerer) match(start, text code.Expr) code.Expr {
	if l.opts.Node {
		return call(flow.OpNewTerminalNode, nil, start, text)
	}
	return call(flow.OpNewMatch, nil, start, text)
}

// branch returns the node of the children kids from start, built for the runtime of the function.
func (l *lowerer) branch(list bool, start, kids code.Expr) code.Expr {
	if l.opts.Node {
		return call(flow.OpNewBranchNode, flow.List(list), start, kids)
	}
	return call(flow.OpNewNode, flow.List(list), start, kids)
}

// outlines reports whether the node n runs as a function of its own.
func (l *lowerer) outlines(n core.Node) bool {
	return l.opts.Outline != nil && n != l.top && !l.opts.Observe && l.streaming == nil &&
		l.opts.Outline(n) && (l.frame || noCaptures(n))
}

// noCaptures reports whether the node n neither sets nor reads captures, so that a function of its own
// can run it for a function whose captures are variables.
func noCaptures(n core.Node) bool {
	ok := true
	core.Walk(n, func(n core.Node) {
		switch n.(type) {
		case *core.Capture, *core.Pred, *core.BindNode, *core.Recover:
			ok = false
		}
	})
	return ok
}

// outlined lowers the node n to a call of its function.
func (l *lowerer) outlined(n core.Node, s *scope, build bool, fail string) code.Expr {
	ok := l.ok()
	rc := call(flow.OpCallOutline, flow.Outline{Node: n, Slots: s.slots, Build: build})
	if !build {
		l.failIf(&code.Set{Dst: []code.Place{{}, {Name: ok}}, Values: []code.Expr{rc}}, code.Not(use(ok)), fail)
		return null()
	}
	v := l.decl("v", flow.TypeValue)
	l.failIf(&code.Set{Dst: []code.Place{{Name: v}, {Name: ok}}, Values: []code.Expr{rc}}, code.Not(use(ok)), fail)
	return use(v)
}

// attach returns x, matched from start, with the captures of the scope whose slots are names
// attached: the values of the variables vals.
func attach(x, start code.Expr, names *flow.Names, vals []string) code.Expr {
	args := []code.Expr{x, start}
	for _, v := range vals {
		args = append(args, use(v))
	}
	return call(flow.OpAttachCaptures, flow.Attach{Names: names}, args...)
}
