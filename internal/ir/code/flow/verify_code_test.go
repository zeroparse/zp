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
	m = build(valid)
	m.Funcs[0].Attrs = nil
	if err := VerifyModule(m); err == nil || !strings.Contains(err.Error(), "no Flow attributes") {
		t.Fatalf("function without attributes: got %v", err)
	}
}
