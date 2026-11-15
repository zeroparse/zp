package libgo

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
)

// observeCalls reports a fixed sequence of calls to the observation that opts select: main calls
// item, which calls pair (calling word twice) and skips word; main then calls item again, which
// calls word.
func observeCalls(opts ...ParseOption) {
	var o parseOptions
	for _, opt := range opts {
		opt(&o)
	}
	silent := 0
	ob := newObservation(&o.observe, &silent)
	if ob == nil {
		return
	}
	defer ob.release()
	ob.Enter("main", 0, 0)
	ob.Enter("item", 0, 0)
	ob.Enter("pair", 0, 0)
	ob.Enter("word", 0, 0)
	ob.Exit(true, 1, 2)
	ob.Enter("word", 2, 0)
	ob.Exit(true, 3, 4)
	ob.Exit(true, 3, 4)
	ob.Skipped("word", 3, 0)
	ob.Exit(true, 3, 4)
	ob.Enter("item", 3, 0)
	ob.Enter("word", 3, 0)
	ob.Exit(false, 3, 4)
	ob.Exit(false, 3, 4)
	ob.Exit(true, 3, 4)
}

// eventLog records events as "kind rule depth".
type eventLog []string

func (l *eventLog) trace(e core.TraceEvent) {
	*l = append(*l, fmt.Sprintf("%v %s %d", e.Kind, e.Rule, e.Depth))
}

// TestTraceOptionSelection checks what the trace functions receive with WithTraceFilter and
// WithTraceMaxDepth, of which the last of each applies.
func TestTraceOptionSelection(t *testing.T) {
	for _, c := range []struct {
		name string
		opts []ParseOption
		want string
	}{
		{"all", nil, "enter main 1,enter item 2,enter pair 3,enter word 4,exit word 4,enter word 4,exit word 4,exit pair 3,skipped word 3,exit item 2,enter item 2,enter word 3,exit word 3,exit item 2,exit main 1"},
		{"pair and nested", []ParseOption{WithTraceFilter("pair")},
			"enter pair 3,enter word 4,exit word 4,enter word 4,exit word 4,exit pair 3"},
		{"skipped and called", []ParseOption{WithTraceFilter("word", "no such rule")},
			"enter word 4,exit word 4,enter word 4,exit word 4,skipped word 3,enter word 3,exit word 3"},
		{"depth", []ParseOption{WithTraceMaxDepth(2)},
			"enter main 1,enter item 2,exit item 2,enter item 2,exit item 2,exit main 1"},
		{"filter and depth", []ParseOption{WithTraceFilter("item"), WithTraceMaxDepth(3)},
			"enter item 2,enter pair 3,exit pair 3,skipped word 3,exit item 2,enter item 2,enter word 3,exit word 3,exit item 2"},
		{"last filter and depth", []ParseOption{WithTraceFilter("main"), WithTraceMaxDepth(1), WithTraceFilter("pair"), WithTraceMaxDepth(3)},
			"enter pair 3,exit pair 3"},
		{"no depth limit", []ParseOption{WithTraceMaxDepth(1), WithTraceMaxDepth(0), WithTraceFilter("no such rule"), WithTraceFilter()},
			"enter main 1,enter item 2,enter pair 3,enter word 4,exit word 4,enter word 4,exit word 4,exit pair 3,skipped word 3,exit item 2,enter item 2,enter word 3,exit word 3,exit item 2,exit main 1"},
		{"unknown rule", []ParseOption{WithTraceFilter("no such rule")}, ""},
	} {
		var got eventLog
		var prof core.Profile
		observeCalls(append(c.opts, WithTrace(got.trace), WithProfile(&prof))...)
		if s := strings.Join(got, ","); s != c.want {
			t.Errorf("%s: got\n%s\nwant\n%s", c.name, s, c.want)
		}
		if snap := prof.Snapshot(); snap.Entered != 7 || snap.Skipped != 1 {
			t.Errorf("%s: the profile counts %d calls and %d skipped, want 7 and 1", c.name, snap.Entered, snap.Skipped)
		}
	}
}

// TestTraceOptionComposition checks that several WithTrace and WithProfile options each receive the
// whole parse in order, that nil ones are ignored, and that the parses of a Document do not share the
// profiles they add to its defaults.
func TestTraceOptionComposition(t *testing.T) {
	var all, a, b eventLog
	observeCalls(WithTrace(all.trace))
	var order []string
	var pa, pb core.Profile
	observeCalls(WithTrace(a.trace), WithTrace(nil), WithProfile(&pa), WithProfile(nil), WithTrace(func(core.TraceEvent) { order = append(order, "a") }),
		WithTrace(b.trace), WithProfile(&pb), WithTrace(func(core.TraceEvent) { order = append(order, "b") }))
	if !slices.Equal(a, all) || !slices.Equal(b, all) {
		t.Errorf("trace functions received\n%v\n%v\nwant\n%v", a, b, all)
	}
	if len(order) != 2*len(all) || order[0] != "a" || order[1] != "b" {
		t.Errorf("trace functions called in the order %v", order)
	}
	if sa, sb := pa.Snapshot(), pb.Snapshot(); sa.Parses != 1 || sb.Parses != 1 || sa.Entered != sb.Entered || sa.Entered != 7 {
		t.Errorf("profiles recorded %d and %d parses, %d and %d calls", sa.Parses, sb.Parses, sa.Entered, sb.Entered)
	}

	// The defaults of a Document are copied for each parse, which may add profiles.
	var defaults parseOptions
	var p1, p2, p3, p4, pc core.Profile
	for _, pr := range []*core.Profile{&p1, &p2, &p3, &p4} {
		WithProfile(pr)(&defaults)
	}
	first, second := defaults, defaults
	WithProfile(&pb)(&first)
	WithProfile(&pc)(&second)
	if len(defaults.observe.more) != 3 || len(first.observe.more) != 4 || len(second.observe.more) != 4 {
		t.Fatal("profiles added to a copy of the defaults")
	}
	if first.observe.more[3] != &pb || second.observe.more[3] != &pc {
		t.Error("a parse's profile was replaced by one that another parse added to the defaults")
	}
}
