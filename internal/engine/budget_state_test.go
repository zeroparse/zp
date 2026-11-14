//go:build !zp_budget_controls_reference

package engine

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"zeroparse.dev/zp/internal/engine/closure"
	ir "zeroparse.dev/zp/internal/ir/core"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/noderep"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/state"
	"zeroparse.dev/zp/internal/value"
)

func stateBudgetParser(limits core.Limits) *state.Parser {
	prog := &state.SharedProgram{}
	state.InitSharedProgram(prog, &compile.Program{})
	return &state.Parser{Prog: prog, Input: state.Input{Control: state.NewBudgetControl(nil, limits)}}
}

func undoFixture(p *state.Parser) (*state.Frame, []*core.Node) {
	old0 := value.Ptr(value.Data{Text: "old-zero", Fields: noderep.Fields{{Name: "keep", Value: "zero"}}})
	first0 := value.Ptr(value.Data{Text: "first-zero"})
	latest0 := value.Ptr(value.Data{Text: "latest-zero"})
	old1 := value.Ptr(value.Data{Text: "old-one"})
	last1 := value.Ptr(value.Data{Text: "last-one"})
	f := &state.Frame{Vals: []*core.Node{latest0, old1}}
	p.Trail = []state.Undo{{F: f, Slot: 0, Old: old0}, {F: f, Slot: 0, Old: first0}, {F: f, Slot: 1, Old: old1}, {F: f, Slot: 1, Old: last1}}
	return f, []*core.Node{old0, first0, old1, last1, latest0}
}

func assertUndoRestored(t *testing.T, f *state.Frame, nodes []*core.Node) {
	t.Helper()
	if f.Vals[0] != nodes[0] || f.Vals[1] != nodes[2] {
		t.Fatalf("restored slots = (%p,%p), want (%p,%p)", f.Vals[0], f.Vals[1], nodes[0], nodes[2])
	}
	if value.D(nodes[0]).Text != "old-zero" || value.D(nodes[0]).Fields[0].Value != "zero" || value.D(nodes[1]).Text != "first-zero" || value.D(nodes[2]).Text != "old-one" || value.D(nodes[3]).Text != "last-one" {
		t.Fatal("undo changed a retained Node pointee")
	}
}

func TestBudgetResetRestoresRepeatedCaptureWritesInReverse(t *testing.T) {
	t.Parallel()
	p := stateBudgetParser(core.Limits{Compute: 4})
	f, nodes := undoFixture(p)
	p.Pos = 71
	p.Frame = f
	p.SetVars([]any{1})
	p.Recovered = []state.RecEntry{state.ErrEntry(&core.SyntaxError{Messages: []string{"temporary"}})}
	m := state.Mark{Pos: 4, Env: nil, Trail: 0, Recovered: 0}

	p.Reset(m)
	assertUndoRestored(t, f, nodes)
	if p.Pos != 4 || p.Env != nil || len(p.Recovered) != 0 || len(p.Trail) != 0 {
		t.Fatalf("reset state: pos=%d env=%p recovered=%d trail=%d", p.Pos, p.Env, len(p.Recovered), len(p.Trail))
	}
}

func TestBudgetResetPollingCancellationDuringUndo(t *testing.T) {
	t.Parallel()
	p := stateBudgetParser(core.Limits{})
	f := &state.Frame{Vals: make([]*core.Node, 257)}
	for i := range f.Vals {
		old := value.Ptr(value.Data{Text: "old"})
		f.Vals[i] = value.Ptr(value.Data{Text: "new"})
		p.Trail = append(p.Trail, state.Undo{F: f, Slot: i, Old: old})
	}
	p.Control = state.NewBudgetControl(newPollingContext(1), core.Limits{})
	err := catchFatal(t, func() { p.Reset(state.Mark{Trail: 0}) })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("undo cancellation = %v, want context.Canceled", err)
	}
	if len(p.Trail) != 257 {
		t.Fatalf("canceled reset truncated trail to %d", len(p.Trail))
	}
	for i := range f.Vals {
		if f.Vals[i] == nil || value.D(f.Vals[i]).Text != "old" && value.D(f.Vals[i]).Text != "new" {
			t.Fatalf("undo cancellation damaged slot %d: %#v", i, f.Vals[i])
		}
	}
}

func TestBudgetAttachCapturesNilFrameAndFieldOrder(t *testing.T) {
	t.Parallel()
	retained := value.Ptr(value.Data{Text: "base", Fields: noderep.Fields{{Name: "existing", Value: "kept"}}})
	allNil := &state.Frame{Vals: make([]*core.Node, 320)}
	p := stateBudgetParser(core.Limits{Compute: 100_000})
	got := p.AttachCaptures(retained, &compile.Scope{Names: make([]string, len(allNil.Vals))}, allNil, 2, 9)
	if got != retained || len(value.D(retained).Fields) != 1 || value.D(retained).Fields[0].Value != "kept" {
		t.Fatal("all-nil capture frame changed the base node")
	}

	fields := []string{"first", "duplicate", "third", "duplicate", "late_" + strings.Repeat("p", 80)}
	values := []*core.Node{
		value.Ptr(value.Data{Text: "first-value"}), value.Ptr(value.Data{Text: "old-duplicate"}), value.Ptr(value.Data{Text: "third-value"}), value.Ptr(value.Data{Text: "new-duplicate"}), value.Ptr(value.Data{Text: "late-value"}),
	}
	base := value.Ptr(value.Data{Text: "fresh-body", Fresh: true})
	result := p.AttachCaptures(base, &compile.Scope{Names: fields}, &state.Frame{Vals: values}, 0, 5)
	if result != base {
		t.Fatal("fresh body was unexpectedly wrapped")
	}
	wantNames := []string{"first", "duplicate", "third", fields[4]}
	if len(value.D(result).Fields) != len(wantNames) {
		t.Fatalf("capture field count = %d, want %d", len(value.D(result).Fields), len(wantNames))
	}
	for i, name := range wantNames {
		if value.D(result).Fields[i].Name != name {
			t.Fatalf("field %d name = %q, want %q", i, value.D(result).Fields[i].Name, name)
		}
	}
	if value.D(result).Fields[1].Value != values[3] || value.D(result).Fields[3].Value != values[4] {
		t.Fatal("capture overwrite/order semantics changed")
	}
}

func TestBudgetAttachCapturesWrapsNonfreshAndSelfCapture(t *testing.T) {
	t.Parallel()
	child := value.Ptr(value.Data{Text: "retained-child"})
	prior := noderep.Fields{{Name: "prior", Value: "unchanged"}}
	shared := value.Ptr(value.Data{Text: "shared", Fields: append(noderep.Fields(nil), prior...)})
	p := stateBudgetParser(core.Limits{Compute: 100_000})
	wrapped := p.AttachCaptures(shared, &compile.Scope{Names: []string{"child"}}, &state.Frame{Vals: []*core.Node{child}}, 4, 7)
	if wrapped == shared || len(value.D(wrapped).Children) != 1 || value.D(wrapped).Children[0] != shared {
		t.Fatal("nonfresh node was not wrapped before capture attachment")
	}
	if got, ok := value.D(wrapped).Fields.Get("child"); !ok || got != child {
		t.Fatalf("wrapped child capture = %v, %t", got, ok)
	}
	if !reflect.DeepEqual(value.D(shared).Fields, prior) || value.D(shared).Text != "shared" || value.D(child).Text != "retained-child" {
		t.Fatal("capture attachment modified a retained node")
	}

	self := value.Ptr(value.Data{Text: "self", Fresh: true})
	got := p.AttachCaptures(self, &compile.Scope{Names: []string{"self"}}, &state.Frame{Vals: []*core.Node{self}}, 1, 2)
	if got == self || len(value.D(got).Children) != 1 || value.D(got).Children[0] != self {
		t.Fatal("self-capture did not wrap the body node")
	}
	if val, ok := value.D(got).Fields.Get("self"); !ok || val != self || len(value.D(self).Fields) != 0 {
		t.Fatal("self-capture fields or source node changed")
	}
}

func TestBudgetAttachCapturesInterruptsDiscoveryAndFieldWalk(t *testing.T) {
	t.Parallel()
	base := value.Ptr(value.Data{Text: "retained", Fields: noderep.Fields{{Name: "prior", Value: "kept"}}})
	wide := &state.Frame{Vals: make([]*core.Node, 320)}
	p := stateBudgetParser(core.Limits{Compute: 128})
	if err := catchFatal(t, func() { _ = p.AttachCaptures(base, &compile.Scope{Names: make([]string, 320)}, wide, 0, 1) }); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("all-nil discovery interruption = %v, want ErrLimit", err)
	}
	if len(value.D(base).Fields) != 1 || value.D(base).Fields[0].Value != "kept" {
		t.Fatal("interrupted all-nil scan changed the retained node")
	}

	const n = 48
	names := make([]string, n)
	vals := make([]*core.Node, n)
	for i := range names {
		names[i] = fmt.Sprintf("capture_%03d_%s", i, strings.Repeat("common_prefix_", 12))
		vals[i] = value.Ptr(value.Data{Text: fmt.Sprintf("value-%d", i)})
	}
	retained := value.Ptr(value.Data{Text: "shared-retained", Fields: noderep.Fields{{Name: "existing", Value: "stable"}}})
	p = stateBudgetParser(core.Limits{Compute: 256})
	if err := catchFatal(t, func() { _ = p.AttachCaptures(retained, &compile.Scope{Names: names}, &state.Frame{Vals: vals}, 3, 9) }); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("capture field walk interruption = %v, want ErrLimit", err)
	}
	if len(value.D(retained).Fields) != 1 || value.D(retained).Fields[0].Name != "existing" || value.D(retained).Fields[0].Value != "stable" || value.D(retained).Text != "shared-retained" {
		t.Fatal("interrupted capture assembly modified the retained nonfresh node")
	}
}

func TestBudgetStateBatchRollbackAndPoolRetry(t *testing.T) {
	t.Parallel()
	var body strings.Builder
	body.WriteString(`c0:"a" `)
	for i := 1; i < 320; i++ {
		fmt.Fprintf(&body, "c%d:_ ", i)
	}
	var predicate strings.Builder
	predicate.WriteString(`text($c0)=="a"`)
	for i := 1; i < 320; i++ {
		fmt.Fprintf(&predicate, ` && text($c%d)==""`, i)
	}
	src := `rule main = (` + body.String() + ` {` + predicate.String() + `} arm "!" / fallback) #recover(skip=.*) $$
rule fallback = .* #atomic
rule arm = _`
	prog := mustCompile(t, src)
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, u := range []core.PositionUnit{core.Bytes, core.CodePoints} {
			for _, recognize := range []bool{false, true} {
				armed := false
				fallback := 0
				var observed *state.Parser
				n, err := prog.ParseWith("main", "a", ParseOptions{Backend: b, Unit: u, Recognize: recognize, Limits: core.Limits{Compute: 1 << 50}, Trace: func(e core.TraceEvent) {
					observed = state.TraceParser(e)
					if e.Kind == core.TraceEnter && e.Rule == "fallback" {
						fallback++
					}
					if e.Kind == core.TraceExit && e.Rule == "arm" && e.Matched {
						if len(state.TraceParser(e).Trail) < 320 {
							t.Fatalf("%v/%v recognize=%t: outer capture trail=%d", b, u, recognize, len(state.TraceParser(e).Trail))
						}
						armed = true
						state.TraceParser(e).Control.Limits.Compute = state.TraceParser(e).Control.Used + 20
					}
				}})
				if !armed || n != nil || !errors.Is(err, core.ErrLimit) || fallback != 0 || observed == nil || observed.Scratch != nil || observed.Control != nil {
					t.Fatalf("%v/%v recognize=%t: armed=%t returned=%t err=%v fallback=%d", b, u, recognize, armed, n != nil, err, fallback)
				}
				n, err = prog.ParseWith("main", "a", ParseOptions{Backend: b, Unit: u, Recognize: recognize})
				if err != nil || (recognize && n != nil) || (!recognize && n == nil) {
					t.Fatalf("%v/%v retry: %v", b, u, err)
				}
			}
		}
	}
}

func TestBudgetStateProjectedFrameClear(t *testing.T) {
	t.Parallel()
	const count = 320
	var items []ir.Node
	var slots []ir.Slot
	for i := range count {
		name := fmt.Sprintf("f%d", i)
		text := &ir.Text{Body: &ir.Succeed{}}
		text.Attrs().Want = true
		items = append(items, &ir.Capture{Slot: i, Name: name, Body: text})
		slots = append(slots, ir.Slot{Name: name})
	}
	rep := &ir.Repeat{Min: 0, Max: 1, Body: &ir.Seq{Items: items}, Scope: slots, Project: "f0"}
	m, err := closure.Matcher(&compile.Program{}, &ir.Module{}, &ir.Capture{Name: "fs", Body: rep, Project: "f0"}, []ir.Slot{{Name: "fs"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	p := stateBudgetParser(core.Limits{Compute: 64})
	if err := catchFatal(t, func() { m(p) }); !errors.Is(err, core.ErrLimit) || len(p.Trail) != 0 {
		t.Fatalf("projected clear err=%v capture writes=%d", err, len(p.Trail))
	}
}
