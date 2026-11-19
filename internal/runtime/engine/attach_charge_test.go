//go:build !zp_budget_controls_reference

package engine

import (
	"errors"
	"testing"

	value "zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/internal/runtime/state"
	"zeroparse.dev/zp/pkg/runtime"
	"zeroparse.dev/zp/pkg/runtime/libgo"
	zpvalue "zeroparse.dev/zp/pkg/runtime/value"
)

// attachCase is a value and a capture frame to attach to it: the value is nil, a fresh node of the
// body, or a node of another rule; slots says which slots hold a capture, and self captures the
// value itself in the first slot.
type attachCase struct {
	name  string
	value string // "nil", "fresh" or "other"
	slots []bool
	self  bool
	want  uint64
}

// attachCases charge a unit for each slot looked at and, when a capture is attached, a unit for the
// tuple node that wraps a value that is not the body's own and a unit for each name (the cost bound
// of flow.attachFrame, spec/flow-ir/operations.md).
var attachCases = []attachCase{
	{"empty frame", "other", nil, false, 0},
	{"no capture", "other", []bool{false, false, false}, false, 3},
	{"fresh body", "fresh", []bool{true, false}, false, 4},
	{"other rule's node", "other", []bool{true}, false, 3},
	{"no value", "nil", []bool{false, true}, false, 5},
	{"self capture", "fresh", []bool{true}, true, 3},
}

// TestAttachCapturesChargesAsGoLibrary checks that the match code of the engine (state.AttachCaptures,
// which the bytecode VM's ATTACHFRAME also runs) charges for each attachment what the Go library's
// AttachCaptures charges, which is what the cost bound of flow.attachFrame names: a parse spends
// the same units for its captures whichever backend runs the rule.
func TestAttachCapturesChargesAsGoLibrary(t *testing.T) {
	for _, c := range attachCases {
		if got := engineAttachCharge(c); got != c.want {
			t.Errorf("%s: the engine charges %d, want %d", c.name, got, c.want)
		}
		if got := libgoAttachCharge(t, c); got != c.want {
			t.Errorf("%s: the Go library charges %d, want %d", c.name, got, c.want)
		}
	}
}

func engineAttachCharge(c attachCase) uint64 {
	p := stateBudgetParser(runtime.Limits{Compute: 1 << 30})
	var v *zpvalue.Node
	switch c.value {
	case "fresh":
		v = value.Ptr(value.Data{Text: "body", Fresh: true})
	case "other":
		v = value.Ptr(value.Data{Text: "other"})
	}
	f := &state.Frame{Vals: make([]*zpvalue.Node, len(c.slots))}
	for i, set := range c.slots {
		if set {
			f.Vals[i] = value.Ptr(value.Data{Text: "capture"})
		}
	}
	if c.self {
		f.Vals[0] = v
	}
	before := p.Input.Control.Used
	p.AttachCaptures(v, &program.Scope{Names: names(len(c.slots))}, f, 0, 0)
	return p.Input.Control.Used - before
}

// libgoAttachCharge returns the units that the Go library's AttachCaptures charges for c: the least
// compute limit of a parse whose rule attaches c, less that of a parse whose rule does not.
func libgoAttachCharge(t *testing.T, c attachCase) uint64 {
	return libgoLeastLimit(t, &c) - libgoLeastLimit(t, nil)
}

func libgoLeastLimit(t *testing.T, c *attachCase) uint64 {
	t.Helper()
	g := libgo.NewGrammar(100)
	r := &libgo.Rule{Name: "S", Body: func(p *libgo.Parser, _ int) (*zpvalue.Node, bool) {
		if c == nil {
			return nil, true
		}
		var v *zpvalue.Node
		switch c.value {
		case "fresh":
			v = libgo.NewNode(zpvalue.KindTuple, 0, 0, "", nil, "", nil, nil)
			libgo.D(v).Fresh = true
		case "other":
			v = libgo.NewNode(zpvalue.KindString, 0, 0, "other", nil, "", nil, nil)
		}
		f := &libgo.CaptureFrame{Vals: make([]*zpvalue.Node, len(c.slots))}
		for i, set := range c.slots {
			if set {
				f.Vals[i] = libgo.NewNode(zpvalue.KindString, 0, 0, "capture", nil, "", nil, nil)
			}
		}
		if c.self {
			f.Vals[0] = v
		}
		p.AttachCaptures(v, names(len(c.slots)), f, 0, 0)
		return nil, true
	}}
	g.SetRules([]*libgo.Rule{r}, 0)
	parses := func(limit uint64) bool {
		_, err := libgo.RunParse(r, "", []libgo.ParseOption{libgo.WithLimits(runtime.Limits{Compute: limit})})
		if err != nil && !errors.Is(err, runtime.ErrLimit) {
			t.Fatal(err)
		}
		return err == nil
	}
	lo, hi := uint64(1), uint64(1<<16)
	if !parses(hi) {
		t.Fatalf("no parse within %d units", hi)
	}
	for lo < hi {
		if mid := (lo + hi) / 2; parses(mid) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

func names(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = string(rune('a' + i))
	}
	return out
}
