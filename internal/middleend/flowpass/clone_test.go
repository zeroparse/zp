package flowpass

import (
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/middleend/flowlower"
)

// TestCloneKeepsTheForms checks that a copy of a lowered function shares no statement or
// expression with it, that the record of its forms refers to the statements of the copy, and that
// the passes run on the copy make what they make on the function while the function stays as the
// lowering made it.
func TestCloneKeepsTheForms(t *testing.T) {
	m, r := forms(t)
	f, err := flowlower.Lower(m, r, "r", flowlower.Options{})
	if err != nil {
		t.Fatal(err)
	}
	before := code.FuncStringIn(f, flow.FuncOf(f))
	c := f.Clone()
	if got := code.FuncStringIn(c, flow.FuncOf(c)); got != before {
		t.Fatalf("copy prints differently:\n%s\nwant:\n%s", got, before)
	}
	reach := func(m *code.Module) map[any]bool {
		out := map[any]bool{}
		code.Walk(flow.FuncOf(m).Body, func(s code.Stmt) { out[s] = true }, func(e code.Expr) { out[e] = true })
		return out
	}
	orig, copied := reach(f), reach(c)
	for x := range copied {
		if orig[x] {
			t.Fatalf("the copy shares a %T", x)
		}
	}
	a := flow.AttrsOf(flow.FuncOf(c))
	if a == flow.AttrsOf(flow.FuncOf(f)) || a.Forms == flow.AttrsOf(flow.FuncOf(f)).Forms {
		t.Fatal("the copy shares the attributes")
	}
	for _, fm := range append(append([]*flow.Form(nil), a.Forms.Classes...), a.Forms.Regions...) {
		if !copied[fm.Block] {
			t.Errorf("a form of the copy refers to a block outside it")
		}
	}
	for _, ch := range a.Forms.Choices {
		if ch.Peek != nil && !copied[ch.Peek] {
			t.Errorf("a choice of the copy refers to a peek outside it")
		}
	}
	env := &Env{Target: all, Value: values}
	if _, err := Run(c, env, RunOptions{VerifyEach: true}); err != nil {
		t.Fatal(err)
	}
	Finish(c)
	if got := code.FuncStringIn(f, flow.FuncOf(f)); got != before {
		t.Errorf("the passes on the copy changed the function:\n%s\nwant:\n%s", got, before)
	}
	want := lower(t, flowlower.Options{}, &Env{Target: all})
	if got := code.FuncStringIn(c, flow.FuncOf(c)); got != want.text {
		t.Errorf("the passes on the copy made:\n%s\nwant:\n%s", got, want.text)
	}
}
