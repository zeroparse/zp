package flow

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/core"
)

// TestVerifyModule checks that VerifyModule accepts a well-formed function and rejects, for each
// invariant it checks, a function that breaks it and only it.
func TestVerifyModule(t *testing.T) {
	i, b := code.TInt, code.TBool
	h := code.THandle
	type fn struct {
		attrs func(*Attrs)
		vars  []code.Param
		body  []code.Stmt
	}
	build := func(f fn) *code.Module {
		m := NewModule("t")
		a := &Attrs{Start: "start", Result: "v", Ctx: "c", Fail: "fail"}
		if f.attrs != nil {
			f.attrs(a)
		}
		g := &code.Func{Name: "f", Attrs: a, Body: &code.Block{Label: a.Fail, Stmts: f.body}}
		for _, e := range []code.Param{code.P(a.Start, i), code.P(a.Result, h(TValue)), code.P(a.Ctx, h(TCtx))} {
			if e.Name != "" {
				g.Locals = append(g.Locals, e)
			}
		}
		for _, v := range f.vars {
			g.Locals = append(g.Locals, v)
			a.Vars = append(a.Vars, v.Name)
		}
		m.Funcs = append(m.Funcs, g)
		return m
	}
	call := func(name string, imm any, args ...code.Expr) *code.Call {
		return &code.Call{Callee: name, Imm: imm, Args: args}
	}
	do := func(name string, imm any, args ...code.Expr) code.Stmt {
		return &code.Do{Call: call(name, imm, args...)}
	}
	lit := (*core.Lit)(nil)
	class := (*core.Class)(nil)
	ok := code.P("ok", b)
	peek := []code.Param{code.P("ch", h(TRune)), code.P("n", i), ok}
	peekSet := func(okName string) code.Stmt {
		return &code.Set{Dst: []code.Place{{Name: "ch"}, {Name: "n"}, {Name: okName}}, Values: []code.Expr{call(OpPeek, nil)}}
	}
	matchLit := func() code.Stmt {
		return &code.Set{Dst: []code.Place{{Name: "v"}, {Name: "ok"}}, Values: []code.Expr{call(OpMatchLit, lit)}}
	}
	exitIfNot := func(x code.Expr) code.Stmt {
		return &code.If{Cond: &code.Unary{Op: "!", X: x}, Then: &code.Block{Stmts: []code.Stmt{&code.Exit{Label: "fail"}}}}
	}
	loop := func(label string, cond code.Expr, body ...code.Stmt) *code.Loop {
		return &code.Loop{Label: label, Cond: cond, Body: &code.Block{Stmts: body}}
	}
	// guarded is a read of a literal in place, with the recording primitive as its alternative.
	guarded := func() []code.Stmt {
		return []code.Stmt{&code.If{Cond: call(OpHasLit, lit), Then: &code.Block{Stmts: []code.Stmt{do(OpAdvance, nil, code.Int(1))}},
			Else: &code.Block{Stmts: []code.Stmt{matchLit()}}}}
	}
	setNode := func(a *Attrs) { a.Node = true }
	budget := func(a *Attrs) { a.Budget = true }
	document := func(a *Attrs) { a.Document = true }

	valid := fn{attrs: budget, vars: []code.Param{ok}, body: []code.Stmt{
		do(OpStep, nil),
		matchLit(),
		exitIfNot(code.L("ok")),
		loop("L1", nil, do(OpStep, nil), &code.Set{Dst: []code.Place{{Name: "ok"}}, Values: []code.Expr{call(OpMatchClass, class)}},
			&code.If{Cond: code.L("ok"), Then: &code.Block{Stmts: []code.Stmt{&code.Continue{Label: "L1"}}}}, &code.Break{Label: "L1"}),
	}}
	if err := VerifyModule(build(valid)); err != nil {
		t.Fatalf("valid function: %v", err)
	}
	for _, c := range []struct {
		name string
		f    fn
		want string
	}{
		{"entry variable of a framed function", fn{attrs: func(a *Attrs) { a.Frame = true }}, "entry variable missing or in a framed function"},
		{"local function without an entry variable", fn{attrs: func(a *Attrs) { a.Ctx = "" }}, "entry variable missing"},
		{"cut flag of a local function", fn{attrs: func(a *Attrs) { a.Cut = "ok" }, vars: []code.Param{ok}}, "cut flag of a local function"},
		{"variable listed as an entry variable", fn{attrs: func(a *Attrs) { a.Vars = []string{"start"} }}, "listed twice or as an entry variable"},
		{"block without a label", fn{body: []code.Stmt{&code.Block{}}}, "block without a label"},
		{"unbounded loop", fn{body: []code.Stmt{loop("L1", nil, do(OpStep, nil))}}, "unbounded loop that never exits"},
		{"break of an outer loop", fn{body: []code.Stmt{loop("L2", nil, loop("L1", nil, &code.Break{Label: "L2"}, &code.Break{Label: "L1"}))}},
			"exit to a loop that is not the innermost one"},
		{"continue of an outer loop", fn{body: []code.Stmt{loop("L2", nil, loop("L1", nil, &code.Continue{Label: "L2"}, &code.Break{Label: "L1"}), &code.Break{Label: "L2"})}},
			"continue of a loop that is not the innermost one"},
		{"peek without the ok variable", fn{vars: peek, body: []code.Stmt{peekSet("")}}, "peek without a variable"},
		{"scan end without a site", fn{body: []code.Stmt{do(OpScanEnd, Site(0), code.Int(0), code.Int(0))}}, "flow.scanEnd site 0"},
		{"skip-run begin without a site", fn{body: []code.Stmt{do(OpSkipBegin, Site(0))}}, "flow.skipBegin site 0"},
		{"stream start in a typed function", fn{body: []code.Stmt{do(OpStreamStart, nil)}}, "flow.streamStart outside a framed Node function"},
		{"frame release in a Node function", fn{attrs: setNode, vars: []code.Param{code.P("fr", h(TFrame))},
			body: []code.Stmt{do(OpFreeFrame, nil, code.L("fr"))}}, "flow.freeFrame in a Node function"},
		{"typed value in a Node function", fn{attrs: setNode, body: []code.Stmt{code.Assign("v", call(OpNewMatch, nil, code.L("start"), code.L("v")))}},
			"typed match in a Node function"},
		{"Node value in a typed function", fn{body: []code.Stmt{code.Assign("v", call(OpNewTerminalNode, nil, code.L("start"), code.L("v")))}},
			"Node terminal in a typed function"},
		{"in-place read in a traced function", fn{attrs: func(a *Attrs) { a.Traced = true }, vars: []code.Param{ok},
			body: []code.Stmt{code.Assign("ok", call(OpMatchClass, class))}}, "a class table read in a traced function"},
		{"in-place read in a larger condition", fn{attrs: document, vars: []code.Param{ok}, body: []code.Stmt{
			&code.If{Cond: &code.Binary{Op: "||", X: code.L("ok"), Y: call(OpHasLit, lit)}, Then: &code.Block{}, Else: &code.Block{Stmts: []code.Stmt{matchLit()}}}}},
			"part of a larger condition"},
		{"in-place read without the recording alternative", fn{attrs: document, body: []code.Stmt{
			&code.If{Cond: call(OpHasLit, lit), Then: &code.Block{}}}}, "without the recording primitive"},
		{"consumption before a charge", fn{attrs: budget, vars: []code.Param{ok}, body: []code.Stmt{matchLit()}},
			"spend: an expression consumes input before a charge"},
		{"advance before a charge", fn{attrs: budget, body: []code.Stmt{do(OpAdvance, nil, code.Int(1))}}, "spend: an advance before a charge"},
		{"iteration without a charge", fn{attrs: budget, vars: []code.Param{ok}, body: []code.Stmt{loop("L1", code.L("ok"), code.Assign("ok", code.BoolC(false)))}},
			"spend: loop L1 ends an iteration without charging"},
		{"continue without a charge", fn{attrs: budget, vars: []code.Param{ok}, body: []code.Stmt{
			loop("L1", nil, &code.If{Cond: code.L("ok"), Then: &code.Block{Stmts: []code.Stmt{&code.Continue{Label: "L1"}}}}, do(OpStep, nil), &code.Break{Label: "L1"})}},
			"spend: loop L1 continues without charging"},
		// The cases of the verifier of the Flow structs that preceded the Flow dialect: those of the
		// structure are the base's checks now (code.Verify), which VerifyModule runs first.
		{"exit to a block that does not enclose it", fn{body: []code.Stmt{&code.Block{Label: "L1"}, &code.Exit{Label: "L1"}}}, "names no enclosing block"},
		{"continue outside its loop", fn{body: []code.Stmt{&code.Block{Label: "L2", Stmts: []code.Stmt{&code.Continue{Label: "L2"}}}}}, "names no enclosing loop"},
		{"break and continue of an outer loop", fn{body: []code.Stmt{loop("L2", nil, loop("L1", nil,
			&code.Break{Label: "L2"}, &code.Continue{Label: "L2"}, &code.Break{Label: "L1"}))}}, "not the innermost"},
		{"undeclared variable", fn{body: []code.Stmt{&code.Set{Dst: []code.Place{{Name: Pos, Reg: true}}, Values: []code.Expr{code.L("x9")}}}}, "undeclared local x9"},
		{"assignment arity", fn{body: []code.Stmt{&code.Set{Dst: []code.Place{{Name: Pos, Reg: true}, {}}, Values: []code.Expr{code.Int(1)}}}}, "set of 2 places from one value"},
		{"label used twice", fn{body: []code.Stmt{&code.Block{Label: "L1"}, &code.Block{Label: "L1"}}}, "label L1 of more than one block or loop"},
		// A local function has no value of its own: its value is the Result variable.
		{"local function with a value", fn{body: []code.Stmt{&code.Return{Values: []code.Expr{code.Null(h(TValue)), code.BoolC(true)}}}}, "return of 2 values, want 0"},
		{"typed node nested in a Node function", fn{attrs: setNode, vars: []code.Param{code.P("ks", h(TVals))}, body: []code.Stmt{
			do(OpPushKid, nil, call(OpNewNode, List(false), code.L("start"), code.L("ks")))}}, "typed node in a Node function"},
		{"Node branch nested in a typed function", fn{vars: []code.Param{code.P("ks", h(TVals))}, body: []code.Stmt{
			code.Assign("v", call(OpScopeValue, core.Shape{}, call(OpNewBranchNode, List(false), code.L("start"), code.L("ks")), code.L("start")))}},
			"Node branch in a typed function"},
		{"local capture values in a Node function", fn{attrs: setNode, body: []code.Stmt{
			code.Assign("v", call(OpAttachCaptures, Attach{Names: &Names{}}, code.L("v"), code.L("start")))}},
			"typed capture values of local variables in a Node function"},
		{"frame trail drop in a Node function", fn{attrs: setNode, vars: []code.Param{code.P("x1", h(TMark))},
			body: []code.Stmt{do(OpDropTrail, nil, code.L("x1"))}}, "flow.dropTrail in a Node function"},
		{"stream element in a local Node function", fn{attrs: setNode, body: []code.Stmt{do(OpEmitStream, nil, code.L("v"))}},
			"flow.emitStream outside a framed Node function"},
		// A traced function records every read, so even a guarded read in place, which a Document function
		// may have, is an error in it.
		{"guarded in-place literal read in a traced function", fn{attrs: func(a *Attrs) { a.Document, a.Traced = true, true }, vars: []code.Param{ok},
			body: guarded()}, "an in-place literal read in a traced function"},
		{"terminal program in a traced function", fn{attrs: func(a *Attrs) { a.Traced = true }, vars: []code.Param{ok},
			body: []code.Stmt{code.Assign("ok", &code.Unary{Op: "!", X: call(OpSkipTerminal, (*core.Terminal)(nil))})}}, "a terminal program in a traced function"},
		{"class run in a traced function", fn{attrs: func(a *Attrs) { a.Traced = true }, vars: []code.Param{ok},
			body: []code.Stmt{code.Assign("ok", call(OpSkipClasses, (*core.ClassRun)(nil)))}}, "a class run in a traced function"},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := VerifyModule(build(c.f))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error containing %q", err, c.want)
			}
		})
	}
	m := build(valid)
	AttrsOf(m.Funcs[0]).Vars = nil
	if err := VerifyModule(m); err == nil || !strings.Contains(err.Error(), "local ok is neither a variable nor an entry variable") {
		t.Fatalf("unlisted local: got %v", err)
	}
	m = build(fn{attrs: func(a *Attrs) { a.Frame, a.Start, a.Result, a.Ctx, a.Cut = true, "", "", "", "c" }})
	if err := VerifyModule(m); err == nil || !strings.Contains(err.Error(), "cut flag c is not a variable") {
		t.Fatalf("cut flag that is no variable: got %v", err)
	}
	// An exit may leave a loop for a block that encloses it, which the printers write as a goto.
	if err := VerifyModule(build(fn{vars: []code.Param{code.P("n", i)}, body: []code.Stmt{
		&code.Block{Label: "L1", Stmts: []code.Stmt{&code.Set{Dst: []code.Place{{Name: Pos, Reg: true}}, Values: []code.Expr{code.Int(1)}}, &code.Exit{Label: "L1"}}},
		&code.Block{Label: "L3", Stmts: []code.Stmt{loop("L4", code.Lt(code.L("n"), code.Int(1)), &code.Exit{Label: "L3"})}},
		&code.Exit{Label: "fail"},
	}})); err != nil {
		t.Errorf("a valid function with an exit from a loop to an enclosing block: %v", err)
	}
	// The guarded read in place that a traced function may not have is valid in a Document function, and a
	// traced function that reads through the recording primitive only verifies.
	if err := VerifyModule(build(fn{attrs: document, vars: []code.Param{ok}, body: guarded()})); err != nil {
		t.Errorf("a guarded read in place in a Document function: %v", err)
	}
	if err := VerifyModule(build(fn{attrs: func(a *Attrs) { a.Document, a.Traced = true, true }, vars: []code.Param{ok},
		body: []code.Stmt{matchLit()}})); err != nil {
		t.Errorf("a traced function that reads through the recording primitive: %v", err)
	}
	m = build(valid)
	m.Funcs[0].Attrs = nil
	if err := VerifyModule(m); err == nil || !strings.Contains(err.Error(), "no Flow attributes") {
		t.Fatalf("function without attributes: got %v", err)
	}
}
