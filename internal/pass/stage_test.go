package pass

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
)

// text is a unit of a second IR, which the lowering of a unit makes.
type text struct{ s string }

var (
	unitIR = IR[*unit]{Stage: "u", Verify: func(u *unit) error {
		if u.broken != "" {
			return errors.New("broken by " + u.broken)
		}
		return nil
	}, Print: func(w io.Writer, l Label, after string, u *unit) {
		fmt.Fprintf(w, "%s/%s after %s: %s\n", l.Table, l.Func, after, strings.Join(u.log, ","))
	}}
	textIR = IR[*text]{Stage: "t", Verify: func(t *text) error {
		if strings.Contains(t.s, "bad") {
			return errors.New("bad text")
		}
		return nil
	}, Print: func(w io.Writer, l Label, after string, t *text) {
		fmt.Fprintf(w, "%s after %s: %s\n", l.Func, after, t.s)
	}}
)

// recorder is an observer that records the events and the dumps, and skips the runs whose
// numbers are in skip.
type recorder struct {
	events []string
	dumps  strings.Builder
	skip   map[int]bool
}

func (o *recorder) Before(e Event) bool { return !o.skip[e.Seq] }

func (o *recorder) After(e Event, print func(io.Writer), _ any) {
	o.events = append(o.events, fmt.Sprintf("%d:%s:%s:%s", e.Seq, e.Step.Name, e.Step.Dump, e.Label.Func))
	print(&o.dumps)
}

func transforms(p *Pipeline[*unit, mode], entry string) Stage[*unit, *unit] {
	return Transforms(Transform[*unit, mode]{IR: unitIR, Pipeline: p, Entry: entry,
		Env: func(*Run, Label, *unit) mode { return mode{} }})
}

func lowerText(name string) Stage[*unit, *text] {
	return Lower(name, "u", textIR, func(u *unit) (*text, error) { return &text{s: strings.Join(u.log, "+")}, nil })
}

// exclaim is a required step that appends "!" to a copy of the text.
func exclaim(x *text) *text { return &text{s: x.s + "!"} }

func TestStageComposition(t *testing.T) {
	s := Then(Then(transforms(fakePipeline(logging("a", nil, none), logging("b", nil, none)), "u"), lowerText("lower")),
		Do("finish", textIR, exclaim))
	if err := s.Check(); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, st := range s.Steps() {
		names = append(names, fmt.Sprintf("%s:%v:%s>%s", st.Name, st.Required, st.In, st.Out))
	}
	if want := []string{"a:false:u>u", "b:false:u>u", "lower:true:u>t", "finish:true:t>t"}; !slices.Equal(names, want) {
		t.Errorf("steps %v, want %v", names, want)
	}
	rec := &recorder{}
	r := &Run{Observers: []Observer{rec}}
	out, err := s.Run(r, Label{Table: "rules", Func: "f"}, &unit{})
	if err != nil || out.s != "a+b+analyze!" {
		t.Fatalf("got %q, %v", out.s, err)
	}
	// The final verification computes the analysis that it requires, after the last dump.
	if want := []string{"0::u:f", "1:a:a:f", "2:b:b:f", "0:lower:t:f", "0:finish::f"}; !slices.Equal(rec.events, want) {
		t.Errorf("events %v, want %v", rec.events, want)
	}
	if want := "rules/f after u: \nrules/f after a: a\nrules/f after b: a,b\nf after t: a+b+analyze\n"; rec.dumps.String() != want {
		t.Errorf("dumps %q, want %q", rec.dumps.String(), want)
	}
}

func TestStageCheck(t *testing.T) {
	twice := Then(transforms(fakePipeline(logging("a", nil, none)), ""), transforms(fakePipeline(logging("a", nil, none)), ""))
	if err := twice.Check(); err == nil || !strings.Contains(err.Error(), "step a is in the pipeline twice") {
		t.Errorf("duplicate name: %v", err)
	}
	// A lowering declared from the wrong stage: the types meet, the stages do not.
	wrong := Then(lowerText("lower"), Lower("again", "u", textIR, func(x *text) (*text, error) { return x, nil }))
	if err := wrong.Check(); err == nil || !strings.Contains(err.Error(), `step again takes the stage "u" but follows lower, which returns "t"`) {
		t.Errorf("stage mismatch: %v", err)
	}
}

// TestStageNumbering checks that the runs of optional passes are numbered in the order they
// happen across units, once per unit, and that an observer skips a run by its number.
func TestStageNumbering(t *testing.T) {
	s := transforms(fakePipeline(logging("a", nil, none), logging("b", nil, none)), "")
	rec := &recorder{skip: map[int]bool{3: true}}
	r := &Run{Observers: []Observer{rec}, Enabled: func(n string) bool { return true }}
	if _, err := s.Run(r, Label{Func: "f"}, &unit{}); err != nil {
		t.Fatal(err)
	}
	v, err := s.Run(r, Label{Func: "g"}, &unit{})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"1:a:a:f", "2:b:b:f", "4:b:b:g"}; !slices.Equal(rec.events, want) {
		t.Errorf("events %v, want %v", rec.events, want)
	}
	if !slices.Equal(v.log, []string{"b", "analyze"}) || r.Runs() != 4 {
		t.Errorf("second unit %v, runs %d", v.log, r.Runs())
	}
	// A pass that the settings turn off takes no number.
	r = &Run{Enabled: func(n string) bool { return n != "a" }}
	if _, err := s.Run(r, Label{}, &unit{}); err != nil || r.Runs() != 1 {
		t.Errorf("runs %d, %v", r.Runs(), err)
	}
}

// TestStageFailure checks that after a failing pass only the required steps run, and that the
// first error is the one returned.
func TestStageFailure(t *testing.T) {
	failing := Pass[*unit, mode]{Name: "bad", Run: func(u *unit, _ mode) (*unit, PreservedSet, error) {
		return u.with("half"), none, errors.New("no")
	}}
	later := transforms(fakePipeline(logging("c", nil, none)), "")
	s := Then(Then(Then(transforms(fakePipeline(logging("a", nil, none), failing), ""), lowerText("lower")),
		Do("finish", textIR, exclaim)), Lower("back", "t", unitIR, func(x *text) (*unit, error) { return &unit{log: []string{x.s}}, nil }))
	s2 := Then(s, later)
	rec := &recorder{}
	out, err := s2.Run(&Run{Observers: []Observer{rec}}, Label{}, &unit{})
	if err == nil || err.Error() != "pass bad: no" {
		t.Fatalf("error %v", err)
	}
	// The required steps go on with the unit as a left it, not with what bad made before it failed.
	if !slices.Equal(out.log, []string{"a!"}) {
		t.Errorf("unit %v, want [a!]: the optional pass c ran after the failure, or the failed pass's unit went on", out.log)
	}
	if want := []string{"1:a:a:", "0:lower:t:", "0:finish::", "0:back:u:"}; !slices.Equal(rec.events, want) {
		t.Errorf("events %v, want %v", rec.events, want)
	}
}

// TestStageLoweringPostcondition checks that the verifier of a lowering's output runs right after
// it, that the error names the lowering, and that nothing runs after it.
func TestStageLoweringPostcondition(t *testing.T) {
	ran := false
	s := Then(Lower("lower", "u", textIR, func(u *unit) (*text, error) { return &text{s: "bad"}, nil }),
		Do("finish", textIR, func(x *text) *text { ran = true; return x }))
	out, err := s.Run(&Run{}, Label{}, &unit{})
	var se *StepError
	if !errors.As(err, &se) || se.Step.Name != "lower" || err.Error() != "bad text" {
		t.Fatalf("error %v", err)
	}
	if out != nil || ran {
		t.Errorf("a step ran after the failed lowering")
	}
}

// TestStageVerifyEach checks that the verification after every pass names the pass that broke the
// unit.
func TestStageVerifyEach(t *testing.T) {
	breaks := Pass[*unit, mode]{Name: "breaks", Run: func(u *unit, _ mode) (*unit, PreservedSet, error) {
		v := u.with("breaks")
		v.broken = "breaks"
		return v, none, nil
	}}
	s := transforms(fakePipeline(logging("a", nil, none), breaks, logging("c", nil, none)), "")
	u, err := s.Run(&Run{VerifyEach: true}, Label{}, &unit{})
	if err == nil || err.Error() != "after pass breaks: broken by breaks" || slices.Contains(u.log, "c") {
		t.Errorf("error %v, log %v", err, u.log)
	}
	if u.broken != "" {
		t.Errorf("the stage returned the unit that breaks made")
	}
}

// pair is a unit with a part, the unit of a projected stage.
type pair struct {
	part *unit
	tag  string
}

// TestStageProject checks that a projected stage runs on the part, that the unit it returns holds
// the part the stage returned, and that the input keeps its part.
func TestStageProject(t *testing.T) {
	s := Project(func(p *pair) *unit { return p.part }, func(p *pair, u *unit) (*pair, error) { return &pair{part: u, tag: p.tag}, nil },
		transforms(fakePipeline(logging("a", nil, none)), ""))
	in := &pair{part: &unit{}, tag: "x"}
	out, err := s.Run(&Run{}, Label{}, in)
	if err != nil {
		t.Fatal(err)
	}
	if out == in || out.tag != "x" || !slices.Equal(out.part.log, []string{"a", "analyze"}) || len(in.part.log) != 0 {
		t.Errorf("out %v (same as in: %v), part log %v, input part log %v", out, out == in, out.part.log, in.part.log)
	}
	// A stage that returns its part unchanged returns the unit itself.
	same := Project(func(p *pair) *unit { return p.part }, func(*pair, *unit) (*pair, error) { panic("put of an unchanged part") },
		transforms(fakePipeline(), ""))
	if out, err := same.Run(&Run{}, Label{}, in); err != nil || out != in {
		t.Errorf("unchanged part: out %v, err %v", out, err)
	}
	// A part that put refuses fails the stage, which returns the unit as it was.
	cause := errors.New("refused")
	refuses := Project(func(p *pair) *unit { return p.part }, func(*pair, *unit) (*pair, error) { return nil, cause },
		transforms(fakePipeline(logging("a", nil, none)), ""))
	if out, err := refuses.Run(&Run{}, Label{}, in); err != cause || out != in {
		t.Errorf("refused part: out %v, err %v", out, err)
	}
}
