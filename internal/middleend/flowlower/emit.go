package flowlower

import (
	"fmt"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
)

// Emit writes the lowered function f as a module of Code IR at the Flow stage, which holds the
// declarations of the Flow dialect and the function. The structure of f (blocks, loops, branches,
// exits, assignments and the arithmetic and logic of its conditions) becomes the same structure of
// Code IR, its variables become locals and its registers registers; each primitive becomes a call of
// its Flow operation, with its dynamic operands as arguments and its static data as the immediate.
//
// The body of the function is a block labeled with f's Fail label, which an exit on failure leaves; the
// caller prints what follows success and failure. A framed function ends its block by publishing its
// cut flag (Func.Cut) and returning its value and true; after the block, it publishes the cut flag
// again and returns the null value and false. The attributes of the function (flow.Attrs) record the
// rest of f.
func Emit(f *flow.Func) (*code.Module, *code.Func, error) {
	e := &emitter{m: flow.NewModule("flow " + f.Name), types: map[*flow.Var]code.Type{},
		loops: map[*flow.Label]bool{}, labels: map[*flow.Label]string{}}
	fn := &code.Func{Name: f.Name}
	at := &flow.Attrs{Rule: f.Rule, Binds: f.Binds, Frame: f.Frame, Node: f.Node, Budget: f.Budget,
		Document: f.Document, Traced: f.Traced, Entered: f.Entered}
	fn.Attrs = at
	for _, v := range f.Vars {
		t := flow.CodeType(v.Type)
		e.types[v] = t
		fn.Locals = append(fn.Locals, code.P(v.Name, t))
		at.Vars = append(at.Vars, v.Name)
	}
	// The entry variables, which the caller declares.
	for _, v := range []*flow.Var{f.Start, f.Result, f.Ctx} {
		if v != nil && e.types[v] == (code.Type{}) {
			t := flow.CodeType(v.Type)
			e.types[v] = t
			fn.Locals = append(fn.Locals, code.P(v.Name, t))
		}
	}
	name := func(v *flow.Var) string {
		if v == nil {
			return ""
		}
		return v.Name
	}
	at.Start, at.Result, at.Ctx, at.Cut = name(f.Start), name(f.Result), name(f.Ctx), name(f.Cut)
	at.Fail = e.label(f.Fail)
	body := &code.Block{Label: at.Fail, Stmts: e.stmts(f.Body)}
	fn.Body = body
	if f.Frame {
		value := code.THandle(flow.TValue)
		fn.Results = []code.Type{value, code.TBool}
		publish := func() []code.Stmt {
			if f.Cut == nil {
				return nil
			}
			return []code.Stmt{&code.Set{Dst: []code.Place{{Name: flow.Cut, Reg: true}}, Values: []code.Expr{code.L(f.Cut.Name)}}}
		}
		var v code.Expr = code.Null(value)
		if f.Value != nil {
			v = e.expr(f.Value, value)
		}
		body.Stmts = append(append(body.Stmts, publish()...), &code.Return{Values: []code.Expr{v, code.BoolC(true)}})
		fn.Body = &code.Block{Stmts: append(append([]code.Stmt{body}, publish()...),
			&code.Return{Values: []code.Expr{code.Null(value), code.BoolC(false)}})}
	}
	if e.err != nil {
		return nil, nil, fmt.Errorf("emit %s: %w", f.Name, e.err)
	}
	e.m.Funcs = append(e.m.Funcs, fn)
	if err := flow.VerifyModule(e.m); err != nil {
		return nil, nil, fmt.Errorf("emit %s: %w", f.Name, err)
	}
	return e.m, fn, nil
}

type emitter struct {
	m      *code.Module
	types  map[*flow.Var]code.Type
	loops  map[*flow.Label]bool
	anon   int
	labels map[*flow.Label]string
	err    error
}

func (e *emitter) fail(err error) {
	if e.err == nil {
		e.err = err
	}
}

// label returns the Code IR label of the Flow label x: its name, or for a label without one (which
// nothing prints, since a printer names only the labels of blocks), a name of its own.
func (e *emitter) label(x *flow.Label) string {
	if name, ok := e.labels[x]; ok {
		return name
	}
	name := ""
	if x != nil {
		name = x.Name
	}
	if name == "" {
		e.anon++
		name = flow.UnnamedLabel(e.anon)
	}
	e.labels[x] = name
	return name
}

func (e *emitter) stmts(ss []flow.Stmt) []code.Stmt {
	var out []code.Stmt
	for _, s := range ss {
		out = append(out, e.stmt(s))
	}
	return out
}

// call returns a call of the Flow operation name with the immediate imm and the arguments args.
func call(name string, imm any, args ...code.Expr) *code.Call {
	return &code.Call{Callee: name, Args: args, Imm: imm}
}

func do(name string, imm any, args ...code.Expr) code.Stmt {
	return &code.Do{Call: call(name, imm, args...)}
}

func (e *emitter) stmt(s flow.Stmt) code.Stmt {
	i, b := code.TInt, code.TBool
	h := code.THandle
	switch s := s.(type) {
	case *flow.Block:
		return &code.Block{Label: e.label(s.Label), Stmts: e.stmts(s.Body)}
	case *flow.Loop:
		e.loops[s.Label] = true
		lp := &code.Loop{Label: e.label(s.Label)}
		switch {
		case s.Counter != nil:
			c := s.Counter.Name
			lp.Init = code.Assign(c, code.Int(0))
			lp.Cond = &code.Binary{Op: "<", X: code.L(c), Y: code.Int(int64(s.Max))}
			lp.Post = code.Assign(c, &code.Binary{Op: "+", X: code.L(c), Y: code.Int(1)})
		case s.While != nil:
			lp.Cond = e.expr(s.While, b)
		}
		lp.Body = &code.Block{Stmts: e.stmts(s.Body)}
		return lp
	case *flow.If:
		st := &code.If{Cond: e.expr(s.Cond, b), Then: &code.Block{Stmts: e.stmts(s.Then)}}
		if s.Init != nil {
			st.Init = e.set(s.Init)
		}
		if s.Else != nil {
			st.Else = &code.Block{Stmts: e.stmts(s.Else)}
		}
		return st
	case *flow.Exit:
		if e.loops[s.To] {
			return &code.Break{Label: e.label(s.To)}
		}
		return &code.Exit{Label: e.label(s.To)}
	case *flow.Continue:
		return &code.Continue{Label: e.label(s.To)}
	case *flow.Set:
		if r, ok := s.Dst[0].(flow.Reg); ok && (r == flow.RegEnv || r == flow.RegFrame) {
			// The assignment of a register that holds a handle.
			if len(s.Dst) != 1 || len(s.X) != 1 {
				e.fail(fmt.Errorf("assignment of the register %d with other places", r))
				return &code.Comment{}
			}
			if r == flow.RegEnv {
				return do(flow.OpSetEnv, nil, e.expr(s.X[0], h(flow.TEnv)))
			}
			return do(flow.OpSetFrame, nil, e.expr(s.X[0], h(flow.TFrame)))
		}
		return e.set(s)
	case *flow.Inc:
		return code.Assign(s.V.Name, &code.Binary{Op: "+", X: code.L(s.V.Name), Y: code.Int(1)})
	case *flow.Peek:
		dst := make([]code.Place, 3)
		for k, v := range []*flow.Var{s.Ch, s.Size, s.OK} {
			if v != nil {
				dst[k] = code.Place{Name: v.Name}
			}
		}
		return &code.Set{Dst: dst, Values: []code.Expr{call(flow.OpPeek, nil)}}
	case *flow.Append:
		return code.Assign(s.Vals.Name, call(flow.OpAppend, flow.Spread(s.Spread), code.L(s.Vals.Name), e.expr(s.X, h(flow.TValue))))
	case *flow.Comment:
		return &code.Comment{Text: s.Text}
	case *flow.Step:
		return do(flow.OpStep, nil)
	case *flow.Silent:
		return do(flow.OpSilent, flow.Silence{Delta: s.Delta, Site: s.Site, Ok: s.Ok})
	case *flow.Advance:
		return do(flow.OpAdvance, nil, e.expr(s.N, i))
	case *flow.Expect:
		return do(flow.OpExpect, flow.Expectation{Exp: s.Exp, Message: s.Message}, e.expr(s.At, i))
	case *flow.ObserveSkip:
		return do(flow.OpObserveSkip, s.Alt)
	case *flow.Restore:
		return do(flow.OpRestore, nil, e.expr(s.Pos, i), e.expr(s.Rec, i))
	case *flow.MergeExpected:
		return do(flow.OpMergeExpected, nil, e.expr(s.Far, i), e.expr(s.Exps, h(flow.TExpIDs)))
	case *flow.MergeRenamed:
		return do(flow.OpMergeRenamed, s.Exp, e.expr(s.Far, i), e.expr(s.Exps, h(flow.TExpIDs)))
	case *flow.PushKid:
		return do(flow.OpPushKid, nil, e.expr(s.X, h(flow.TValue)))
	case *flow.DropKids:
		return do(flow.OpDropKids, nil, e.expr(s.Base, i))
	case *flow.StreamStart:
		return do(flow.OpStreamStart, nil)
	case *flow.SkipBegin:
		if s.Run {
			return do(flow.OpRunBegin, flow.RunWalk{Site: s.Site, Build: s.Build, KeyEnv: s.KeyEnv})
		}
		return do(flow.OpSkipBegin, flow.Site(s.Site))
	case *flow.SkipValue:
		return do(flow.OpSkipValue, nil, e.expr(s.X, h(flow.TValue)))
	case *flow.RunWalked:
		return do(flow.OpRunWalked, flow.Site(s.Site), e.expr(s.Start, i))
	case *flow.SkipVisit:
		return do(flow.OpSkipVisit, nil)
	case *flow.SkipEnd:
		return do(flow.OpSkipEnd, nil)
	case *flow.ScanEnd:
		return do(flow.OpScanEnd, flow.Site(s.Site), e.expr(s.Start, i), e.expr(s.Count, i))
	case *flow.EmitStream:
		return do(flow.OpEmitStream, nil, e.expr(s.X, h(flow.TValue)))
	case *flow.SetWhere:
		return do(flow.OpSetWhere, flow.Chars(s.Name))
	case *flow.DropCreated:
		return do(flow.OpDropCreated, nil, e.expr(s.C, h(flow.TCtx)))
	case *flow.Unfresh:
		return do(flow.OpUnfresh, nil, e.expr(s.X, h(flow.TValue)))
	case *flow.Reset:
		return do(flow.OpReset, nil, e.expr(s.M, h(flow.TMark)))
	case *flow.SetCapture:
		return do(flow.OpSetCapture, flow.Slot(s.Slot), e.expr(s.X, h(flow.TValue)))
	case *flow.ClearVals:
		return do(flow.OpClearVals, nil, code.L(s.F.Name))
	case *flow.DropTrail:
		return do(flow.OpDropTrail, nil, e.expr(s.M, h(flow.TMark)))
	case *flow.FreeFrame:
		return do(flow.OpFreeFrame, nil, code.L(s.F.Name))
	case *flow.AppendError:
		return do(flow.OpAppendError, nil, e.expr(s.X, h(flow.TError)))
	case *flow.DeclareNames:
		return do(flow.OpDeclareNames, s.N)
	}
	e.fail(fmt.Errorf("statement %T", s))
	return &code.Comment{}
}

// set lowers the assignment s, which assigns no register that holds a handle.
func (e *emitter) set(s *flow.Set) *code.Set {
	dst := make([]code.Place, len(s.Dst))
	types := make([]code.Type, len(s.Dst))
	for k, d := range s.Dst {
		switch d := d.(type) {
		case *flow.Var:
			dst[k], types[k] = code.Place{Name: d.Name}, e.types[d]
		case flow.Reg:
			switch d {
			case flow.RegEnv, flow.RegFrame:
				e.fail(fmt.Errorf("assignment of the register %d with other places", d))
			case flow.RegCut:
				dst[k], types[k] = code.Place{Name: flow.Cut, Reg: true}, code.TBool
			default:
				dst[k], types[k] = code.Place{Name: flow.Pos, Reg: true}, code.TInt
			}
		case *flow.Discard:
		}
	}
	out := &code.Set{Dst: dst}
	if len(s.X) == 1 && len(s.Dst) > 1 {
		// One operation of several results.
		out.Values = []code.Expr{e.expr(s.X[0], code.Type{})}
		return out
	}
	for k, x := range s.X {
		out.Values = append(out.Values, e.expr(x, types[k]))
	}
	return out
}

// caps returns the arguments of the captures c of an action expression that reads the slots read:
// the variable of each slot that the expression reads, and the null value for the others; none
// where the expression reads the current frame.
func (e *emitter) caps(c flow.Captures, read func(func(*flow.Var))) []code.Expr {
	if c.Vars == nil {
		return nil
	}
	used := map[*flow.Var]bool{}
	read(func(v *flow.Var) { used[v] = true })
	out := make([]code.Expr, len(c.Vars))
	for k, v := range c.Vars {
		if v != nil && used[v] {
			out[k] = code.L(v.Name)
		} else {
			out[k] = code.Null(code.THandle(flow.TValue))
		}
	}
	return out
}

func capture(c flow.Captures) flow.Capture { return flow.Capture{Names: c.Names, Frame: c.Vars == nil} }

// expr lowers the expression x, of the type t where x is a constant.
func (e *emitter) expr(x flow.Expr, t code.Type) code.Expr {
	i, b := code.TInt, code.TBool
	h := code.THandle
	value := h(flow.TValue)
	switch x := x.(type) {
	case *flow.Use:
		return code.L(x.V.Name)
	case *flow.Read:
		switch x.R {
		case flow.RegEnv:
			return call(flow.OpEnv, nil)
		case flow.RegFrame:
			return call(flow.OpFrame, nil)
		case flow.RegCut:
			return code.R(flow.Cut)
		}
		return code.R(flow.Pos)
	case *flow.Nil:
		return code.Null(t)
	case *flow.Int:
		return code.Int(int64(x.N))
	case *flow.Bool:
		return code.BoolC(x.B)
	case *flow.Not:
		return &code.Unary{Op: "!", X: e.expr(x.X, b)}
	case *flow.Or:
		return &code.Binary{Op: "||", X: e.expr(x.X, b), Y: e.expr(x.Y, b)}
	case *flow.Cmp:
		ot := i
		if u, ok := x.L.(*flow.Use); ok {
			ot = e.types[u.V]
		} else if u, ok := x.R.(*flow.Use); ok {
			ot = e.types[u.V]
		}
		return &code.Binary{Op: x.Op, X: e.expr(x.L, ot), Y: e.expr(x.R, ot)}
	case *flow.RecLen:
		return call(flow.OpRecLen, nil)
	case *flow.KidsLen:
		return call(flow.OpKidsLen, nil)
	case *flow.StreamOn:
		return call(flow.OpStreamOn, nil)
	case *flow.SkipHit:
		return call(flow.OpSkipHit, nil)
	case *flow.RunHot:
		return call(flow.OpRunHot, flow.Site(x.Site))
	case *flow.SkipKids:
		return call(flow.OpSkipKids, nil)
	case *flow.ScanReach:
		return call(flow.OpScanReach, flow.Site(x.Site), e.expr(x.Start, i), e.expr(x.Count, i))
	case *flow.ScanKids:
		return call(flow.OpScanKids, flow.Site(x.Site), e.expr(x.Start, i), e.expr(x.Base, i), e.expr(x.Count, i))
	case *flow.HasLit:
		return call(flow.OpHasLit, x.Lit)
	case *flow.MatchLit:
		return call(flow.OpMatchLit, x.Lit)
	case *flow.MatchClass:
		return call(flow.OpMatchClass, x.Class)
	case *flow.InClass:
		return call(flow.OpInClass, x.Class, code.L(x.Ch.Name))
	case *flow.GuardFails:
		return call(flow.OpGuardFails, x.Guard, code.L(x.Ch.Name), code.L(x.More.Name))
	case *flow.AtAnchor:
		return call(flow.OpAtAnchor, x.Kind)
	case *flow.SkipTerminal:
		return call(flow.OpSkipTerminal, x.T)
	case *flow.SkipClasses:
		return call(flow.OpSkipClasses, x.Run)
	case *flow.CallRule:
		return call(flow.OpCallRule, flow.RuleCall{Rule: x.Rule, Level: x.Level})
	case *flow.CallOutline:
		return call(flow.OpCallOutline, flow.Outline{Node: x.Node, Slots: x.Slots, Build: x.Build})
	case *flow.Isolate:
		return call(flow.OpIsolate, nil)
	case *flow.Unisolate:
		return call(flow.OpUnisolate, nil, e.expr(x.Mark, h(flow.TExpMark)))
	case *flow.Text:
		return call(flow.OpText, nil, e.expr(x.Start, i))
	case *flow.LiteralText:
		return call(flow.OpLiteralText, flow.Chars(x.Lit), e.expr(x.Start, i))
	case *flow.Str:
		return call(flow.OpStr, flow.Chars(x.S))
	case *flow.NewMatch:
		return call(flow.OpNewMatch, nil, e.expr(x.Start, i), e.expr(x.Text, value))
	case *flow.NewNode:
		return call(flow.OpNewNode, flow.List(x.List), e.expr(x.Start, i), e.expr(x.Kids, h(flow.TVals)))
	case *flow.NewTerminalNode:
		return call(flow.OpNewTerminalNode, nil, e.expr(x.Start, i), e.expr(x.Text, value))
	case *flow.NewBranchNode:
		return call(flow.OpNewBranchNode, flow.List(x.List), e.expr(x.Start, i), e.expr(x.Kids, h(flow.TVals)))
	case *flow.NewVals:
		return call(flow.OpNewVals, flow.Room(x.N))
	case *flow.Kids:
		return call(flow.OpKids, nil, e.expr(x.Base, i))
	case *flow.ShapeSingle:
		return call(flow.OpShapeSingle, x.Shape, e.expr(x.X, value), e.expr(x.Start, i))
	case *flow.ShapeCapture:
		return call(flow.OpShapeCapture, x.Shape, e.expr(x.X, value), e.expr(x.Start, i))
	case *flow.ScopeValue:
		return call(flow.OpScopeValue, x.Shape, e.expr(x.X, value), e.expr(x.Start, i))
	case *flow.AttachCaptures:
		args := []code.Expr{e.expr(x.X, value), e.expr(x.Start, i)}
		if x.Frame != nil {
			return call(flow.OpAttachFrame, flow.Attach{Names: x.Names, Frame: true}, append(args, code.L(x.Frame.Name))...)
		}
		for _, v := range x.Vals {
			args = append(args, code.L(v.Name))
		}
		return call(flow.OpAttachCaptures, flow.Attach{Names: x.Names}, args...)
	case *flow.MarkState:
		return call(flow.OpMarkState, flow.StateMark{Env: x.Env, Trail: x.Trail, Rec: x.Rec})
	case *flow.MarkPos:
		return call(flow.OpMarkPos, nil, code.L(x.M.Name))
	case *flow.NewFrame:
		return call(flow.OpNewFrame, x.N)
	case *flow.FrameVal:
		return call(flow.OpFrameVal, flow.Slot(x.Slot), code.L(x.F.Name))
	case *flow.Keep:
		return call(flow.OpKeep, nil, e.expr(x.Exps, h(flow.TExpIDs)))
	case *flow.MakeError:
		return call(flow.OpMakeError, nil, e.expr(x.Far, i), e.expr(x.Exps, h(flow.TExpIDs)))
	case *flow.ErrorValue:
		return call(flow.OpErrorValue, nil, e.expr(x.Start, i), e.expr(x.X, h(flow.TError)))
	case *flow.NewErrorNode:
		return call(flow.OpNewErrorNode, nil, e.expr(x.Start, i), e.expr(x.X, h(flow.TError)))
	case *flow.FreshNode:
		return call(flow.OpFreshNode, nil, e.expr(x.X, value))
	case *flow.ConvertResult:
		args := []code.Expr{e.expr(x.X, value)}
		if x.Owned != nil {
			args = append(args, e.expr(x.Owned, value))
		}
		return call(flow.OpConvertResult, flow.Conversion{Rule: x.Rule, Owned: x.Owned != nil}, args...)
	case *flow.UseCtx:
		return call(flow.OpUseCtx, nil, e.expr(x.Start, i))
	case *flow.Pred:
		read := func(f func(*flow.Var)) { flow.Uses(x, f) }
		return call(flow.OpPred, flow.Predicate{Node: x.Node, Capture: capture(x.Caps)}, e.caps(x.Caps, read)...)
	case *flow.Bind:
		read := func(f func(*flow.Var)) { flow.Uses(x, f) }
		return call(flow.OpBind, flow.Binding{Node: x.Node, Capture: capture(x.Caps)}, e.caps(x.Caps, read)...)
	case *flow.Eval:
		read := func(f func(*flow.Var)) { flow.Uses(x, f) }
		args := append([]code.Expr{code.L(x.C.Name)}, e.caps(x.Caps, read)...)
		return call(flow.OpEval, flow.Action{Term: x.Term, Capture: capture(x.Caps)}, args...)
	case *flow.CtxFinish:
		return call(flow.OpCtxFinish, nil, code.L(x.C.Name), e.expr(x.X, value))
	}
	e.fail(fmt.Errorf("expression %T", x))
	return code.BoolC(false)
}
