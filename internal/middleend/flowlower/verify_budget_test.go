package flowlower

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
)

// stripSteps removes the calls of step from b, at any depth.
func stripSteps(b *code.Block) {
	if b == nil {
		return
	}
	var out []code.Stmt
	for _, s := range b.Stmts {
		switch s := s.(type) {
		case *code.Do:
			if s.Call.Callee == flow.OpStep {
				continue
			}
		case *code.Block:
			stripSteps(s)
		case *code.Loop:
			stripSteps(s.Body)
		case *code.If:
			stripSteps(s.Then)
			stripSteps(s.Else)
		}
		out = append(out, s)
	}
	b.Stmts = out
}

// TestVerifyBudgetCharges checks that a function lowered with budget controls charges before it consumes
// input and in every iteration of a repetition, and that the Flow verifier rejects the same function
// without its steps, and a hand-built loop whose iteration charges on one path only.
func TestVerifyBudgetCharges(t *testing.T) {
	for _, c := range []struct {
		name string
		body core.Node
	}{
		{"literal", &core.Lit{Text: "a"}},
		{"star of literals", &core.Repeat{Min: 0, Max: -1, Body: &core.Lit{Text: "ab"}}},
		{"plus of a sequence", &core.Repeat{Min: 1, Max: -1, Body: &core.Seq{Items: []core.Node{&core.Lit{Text: "a"}, &core.Lit{Text: "b"}}}}},
		{"choice", alts(&core.Lit{Text: "a"}, &core.Lit{Text: "b"})},
	} {
		t.Run(c.name, func(t *testing.T) {
			m, r := valueFree(c.body)
			f, err := Lower(m, r, "r", Options{Budget: true})
			if err != nil {
				t.Fatal(err)
			}
			fn := flow.FuncOf(f)
			a := flow.AttrsOf(fn)
			if !a.Budget {
				t.Fatal("the function does not record that it is lowered with budget controls")
			}
			if !a.Entered {
				t.Fatal("the function of a flat rule does not take its entry as charged by its caller")
			}
			stripSteps(fn.Body)
			a.Entered = false // without the step the caller charges, nothing charges the entry
			err = flow.VerifyModule(f)
			if err == nil || !strings.Contains(err.Error(), "spend:") {
				t.Errorf("verifying the function without its steps: got %v, want a spend error", err)
			}
		})
	}
	// The caller charges the entry of a framed function, and a repetition charges once per iteration: without
	// the iteration step, the entry charge does not cover the loop.
	for _, body := range []core.Node{
		&core.Repeat{Min: 0, Max: -1, Body: &core.Lit{Text: "ab"}},
		&core.Repeat{Min: 1, Max: 3, Body: &core.Lit{Text: "ab"}},
	} {
		m, r := valueFree(body)
		f, err := LowerFrame(m, r, body, nil, false, "r", Options{Budget: true})
		if err != nil {
			t.Fatal(err)
		}
		fn := flow.FuncOf(f)
		if !flow.AttrsOf(fn).Entered {
			t.Fatal("the framed function does not take its entry as charged by its caller")
		}
		// The body of a framed function is its block of failure, then its return on failure.
		for _, s := range fn.Body.Stmts[0].(*code.Block).Stmts {
			if lp, ok := s.(*code.Loop); ok {
				if d, ok := lp.Body.Stmts[0].(*code.Do); !ok || d.Call.Callee != flow.OpStep {
					t.Errorf("a repetition does not charge first in its iteration:\n%s", code.FuncStringIn(f, fn))
				}
				stripSteps(lp.Body)
			}
		}
		if err := flow.VerifyModule(f); err == nil || !strings.Contains(err.Error(), "spend:") {
			t.Errorf("verifying a repetition without its iteration step: got %v, want a spend error", err)
		}
	}
	const loop = "loop"
	newFunc := func(entered bool, body ...code.Stmt) *code.Module {
		m := flow.NewModule("f")
		m.Funcs = []*code.Func{{Name: "f",
			Attrs: &flow.Attrs{Budget: true, Entered: entered, Fail: "fail", Start: "start", Result: "v", Ctx: "c", Vars: []string{"ok", "n"}},
			Locals: []code.Param{code.P("ok", code.TBool), code.P("n", code.TInt), code.P("start", code.TInt),
				code.P("v", code.THandle(flow.TValue)), code.P("c", code.THandle(flow.TCtx))},
			Body: &code.Block{Label: "fail", Stmts: body}}}
		return m
	}
	step := func() code.Stmt { return do(flow.OpStep, nil) }
	advance := func() code.Stmt { return do(flow.OpAdvance, nil, num(1)) }
	then := func(ss ...code.Stmt) *code.Block { return &code.Block{Stmts: ss} }
	oneSided := newFunc(false, &code.Loop{Label: loop, Body: then(
		&code.If{Cond: use("ok"), Then: then(&code.Break{Label: loop})},
		&code.If{Cond: use("ok"), Then: then(step(), &code.Continue{Label: loop}), Else: then(&code.Continue{Label: loop})},
	)})
	if err := flow.VerifyModule(oneSided); err == nil || !strings.Contains(err.Error(), "continues without charging") {
		t.Errorf("a loop that charges on one path: got %v", err)
	}
	always := newFunc(false, &code.Loop{Label: loop, Body: then(
		&code.If{Cond: use("ok"), Then: then(&code.Break{Label: loop})},
		step(), &code.Continue{Label: loop})})
	if err := flow.VerifyModule(always); err != nil {
		t.Errorf("a loop that charges before it continues: %v", err)
	}
	early := newFunc(false, advance())
	if err := flow.VerifyModule(early); err == nil || !strings.Contains(err.Error(), "advance before a charge") {
		t.Errorf("an advance before a charge: got %v", err)
	}
	entered := newFunc(true, advance())
	if err := flow.VerifyModule(entered); err != nil {
		t.Errorf("a function whose caller charged its entry: %v", err)
	}
	// A loop that ends by its bound, perhaps before any iteration, leaves the state uncharged even when
	// its only exit has charged.
	bounded := newFunc(false, &code.Loop{Label: loop, Init: code.Assign("n", num(0)), Cond: code.Lt(use("n"), num(3)), Post: inc("n"),
		Body: then(
			&code.If{Cond: use("ok"), Then: then(step(), &code.Break{Label: loop})},
			step())}, advance())
	if err := flow.VerifyModule(bounded); err == nil || !strings.Contains(err.Error(), "advance before a charge") {
		t.Errorf("an advance after a bounded loop that may run no iteration: got %v", err)
	}
}

// TestLowerRejectsBudgetWithLeanClasses checks that the class options that charge no spend for the input
// cannot be combined with Options.Budget.
func TestLowerRejectsBudgetWithLeanClasses(t *testing.T) {
	for _, opts := range []Options{{Budget: true, ClassTableRanges: 5}, {Budget: true, GuardedClasses: true}} {
		m, r := valueFree(&core.Lit{Text: "a"})
		if _, err := Lower(m, r, "r", opts); err == nil {
			t.Errorf("Lower accepted %+v", opts)
		}
	}
	m, r := valueFree(&core.Lit{Text: "a"})
	if _, err := Lower(m, r, "r", Options{ClassTableRanges: 5, GuardedClasses: true}); err != nil {
		t.Error(err)
	}
}

// TestLowerFrameRejectsBudgetWithLeanClasses is the same check for the framed lowering.
func TestLowerFrameRejectsBudgetWithLeanClasses(t *testing.T) {
	for _, opts := range []Options{{Budget: true, ClassTableRanges: 5}, {Budget: true, GuardedClasses: true}} {
		m, r := valueFree(&core.Lit{Text: "a"})
		for name, lower := range map[string]func() (*code.Module, error){
			"LowerFrame": func() (*code.Module, error) { return LowerFrame(m, r, r.Body, nil, false, "r", opts) },
		} {
			if _, err := lower(); err == nil || !strings.Contains(err.Error(), "Options.Budget") {
				t.Errorf("%s accepted %+v: %v", name, opts, err)
			}
		}
	}
}
