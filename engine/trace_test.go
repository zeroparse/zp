package engine_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestWithTrace(t *testing.T) {
	p, err := compileTestSource(`rule main = pair (";" pair)* $$
rule pair = key "=" value
rule key = [a-z]+ #atomic
rule value = [0-9]+ #atomic`, "main")
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	var exits int
	_, err = p.Parse("a=1;b=x", engine.WithExpectations(), engine.WithTrace(func(e engine.TraceEvent) {
		if e.Kind == engine.TraceEnter {
			line, col := e.LineCol(e.Pos)
			calls = append(calls, fmt.Sprintf("%s%s@%d:%d", strings.Repeat(" ", e.Depth-1), e.Rule, line, col))
			return
		}
		if !e.Matched && e.Rule == "value" {
			calls = append(calls, "value failed: "+e.Failure().Error())
		}
	}), engine.WithTrace(func(e engine.TraceEvent) {
		if e.Kind == engine.TraceExit {
			exits++
		}
	}))
	if err == nil || err.Error() != "1:7: syntax error: expected [0-9]" {
		t.Errorf("error %v", err)
	}
	want := []string{
		"main@1:1",
		" pair@1:1", "  key@1:1", "  value@1:3",
		" pair@1:5", "  key@1:5", "  value@1:7",
		"value failed: 1:7: syntax error: expected [0-9]",
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls\n%s\nwant\n%s", strings.Join(calls, "\n"), strings.Join(want, "\n"))
	}
	if exits != len(want)-1 {
		t.Errorf("%d exits, want %d", exits, len(want)-1)
	}
}

func TestWithProfile(t *testing.T) {
	p, err := compileTestSource(`rule main = item+ $$
rule item = word / num
rule word = [a-z]+ #atomic " "?
rule num = [0-9]+ #atomic " "?`, "main")
	if err != nil {
		t.Fatal(err)
	}
	var prof engine.Profile
	if _, err := p.Parse("ab 12 cd", engine.WithProfile(&prof)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Parse("1 x", engine.WithProfile(&prof), engine.WithBackend(engine.BytecodeIterative)); err != nil {
		t.Fatal(err)
	}
	snap := prof.Snapshot()
	counts := map[string]string{}
	for _, r := range snap.Rules {
		counts[r.Rule] = fmt.Sprintf("calls=%d matched=%d failed=%d consumed=%d", r.Calls, r.Matched, r.Failed, r.Consumed)
	}
	want := map[string]string{
		"main": "calls=2 matched=2 failed=0 consumed=11",
		"item": "calls=7 matched=5 failed=2 consumed=11",
		"word": "calls=7 matched=3 failed=4 consumed=6",
		"num":  "calls=4 matched=2 failed=2 consumed=5",
	}
	if fmt.Sprint(counts) != fmt.Sprint(want) || snap.Parses != 2 || len(snap.Hints()) == 0 {
		t.Errorf("profile %v (%d parses), want %v", counts, snap.Parses, want)
	}
}

// TestNilTraceOptions checks that WithTrace(nil) and WithProfile(nil) are ignored, alone and
// combined with other trace functions.
func TestNilTraceOptions(t *testing.T) {
	p, err := compileTestSource(`rule main = [a-z]+ #atomic $$`, "main")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	count := engine.WithTrace(func(engine.TraceEvent) { calls++ })
	for _, opts := range [][]engine.ParseOption{
		{engine.WithTrace(nil)},
		{engine.WithProfile(nil)},
		{count, engine.WithTrace(nil), engine.WithProfile(nil)},
		{engine.WithProfile(nil), count},
	} {
		if n, err := p.Parse("abc", opts...); err != nil || n == nil {
			t.Errorf("got %v, %v", n, err)
		}
	}
	if calls != 4 {
		t.Errorf("%d events, want 4", calls)
	}
}

// TestTraceOptions checks the trace options on every backend and entry point: several WithTrace and
// WithProfile options each receive the whole parse, the trace functions receive the events that
// WithTraceFilter and WithTraceMaxDepth select (the last of each applies), and the profiles do not
// depend on them. A Document's options add to its defaults for one Parse.
func TestTraceOptions(t *testing.T) {
	p, err := compileTestSource(`rule main = item* #stream $$
rule item = pair / word
rule pair = word "=" word
rule word = [a-z]+ #atomic ";"?`, "main")
	if err != nil {
		t.Fatal(err)
	}
	const in = "a=b;c;d=e;"
	rules, depth := []string{"pair", "no such rule"}, 3
	entries := map[string]func(opts ...engine.ParseOption) error{
		"Parse": func(opts ...engine.ParseOption) error { _, err := p.Parse(in, opts...); return err },
		"Recognize": func(opts ...engine.ParseOption) error {
			_, err := p.Parse(in, append(opts, engine.RecognizeOnly())...)
			return err
		},
		"ParseStream": func(opts ...engine.ParseOption) error {
			return p.ParseStream(strings.NewReader(in), func(*core.Node) error { return nil }, opts...)
		},
		"Document": func(opts ...engine.ParseOption) error {
			d, err := p.NewDocument(in, opts...)
			if err != nil {
				return err
			}
			_, err = d.Parse()
			return err
		},
	}
	for _, b := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
		for name, run := range entries {
			var all, second, filtered []engine.TraceEvent
			var prof, prof2, filteredProf engine.Profile
			if err := run(engine.WithBackend(b), engine.WithTrace(func(e engine.TraceEvent) { all = append(all, e) }), engine.WithProfile(&prof),
				engine.WithTrace(func(e engine.TraceEvent) { second = append(second, e) }), engine.WithProfile(&prof2)); err != nil {
				t.Fatalf("%v %s: %v", b, name, err)
			}
			if err := run(engine.WithBackend(b), engine.WithTraceFilter("main"), engine.WithTraceFilter(rules...), engine.WithTraceMaxDepth(1), engine.WithTraceMaxDepth(depth),
				engine.WithTrace(func(e engine.TraceEvent) { filtered = append(filtered, e) }), engine.WithProfile(&filteredProf)); err != nil {
				t.Fatalf("%v %s: %v", b, name, err)
			}
			want := codegentest.SelectedEvents(all, rules, depth)
			switch {
			case len(all) == 0 || !sameEvents(all, second):
				t.Errorf("%v %s: the second trace function received %d events, the first %d", b, name, len(second), len(all))
			case !sameProfiles(prof, prof2):
				t.Errorf("%v %s: the second profile differs from the first", b, name)
			case len(want) == 0 || len(want) == len(all) || !sameEvents(want, filtered):
				t.Errorf("%v %s: filtered events\n%v\nwant\n%v", b, name, filtered, want)
			case !sameProfiles(prof, filteredProf):
				t.Errorf("%v %s: the profile depends on the trace filter", b, name)
			}
		}
	}

	// The options of a Document's Parse add to its defaults for that Parse only.
	var defaults, extra engine.Profile
	var defaultEvents, extraEvents int
	d, err := p.NewDocument(in, engine.WithProfile(&defaults), engine.WithTrace(func(engine.TraceEvent) { defaultEvents++ }))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Parse(engine.WithProfile(&extra), engine.WithTrace(func(engine.TraceEvent) { extraEvents++ })); err != nil {
		t.Fatal(err)
	}
	if defaults.Snapshot().Parses != 1 || extra.Snapshot().Parses != 1 || extraEvents == 0 || defaultEvents != extraEvents {
		t.Errorf("the options of Parse do not add to the defaults: %d and %d parses, %d and %d events",
			defaults.Snapshot().Parses, extra.Snapshot().Parses, defaultEvents, extraEvents)
	}
	if _, err := d.Parse(); err != nil {
		t.Fatal(err)
	}
	if defaults.Snapshot().Parses != 2 || extra.Snapshot().Parses != 1 || defaultEvents == extraEvents {
		t.Error("the options of Parse are kept as defaults")
	}
}

// sameEvents reports whether a and b have the same fields.
func sameEvents(a, b []engine.TraceEvent) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// sameProfiles reports whether a and b have the same counts; times are left out.
func sameProfiles(a, b engine.Profile) bool {
	untimed := func(p engine.Profile) engine.ProfileSnapshot {
		s := p.Snapshot()
		s.Time = 0
		for i := range s.Rules {
			s.Rules[i].Time, s.Rules[i].SelfTime = 0, 0
		}
		return s
	}
	x, _ := json.Marshal(untimed(a))
	y, _ := json.Marshal(untimed(b))
	return string(x) == string(y)
}

// A trace filter or depth limit given to a Document's Parse replaces the default for that Parse only.
// Each Parse follows an Edit of the whole text, so that it makes every call again.
func TestDocumentTraceSelection(t *testing.T) {
	p, err := compileTestSource(`rule main = item* #stream $$
rule item = pair / word
rule pair = word "=" word
rule word = [a-z]+ #atomic ";"?`, "main")
	if err != nil {
		t.Fatal(err)
	}
	const in = "a=b;c;d=e;"
	for _, b := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
		var events []engine.TraceEvent
		record := engine.WithTrace(func(e engine.TraceEvent) { events = append(events, e) })
		full := func(d *engine.Document, opts ...engine.ParseOption) []engine.TraceEvent {
			t.Helper()
			if err := d.Edit(0, len(d.Text()), in); err != nil {
				t.Fatal(err)
			}
			events = nil
			if _, err := d.Parse(opts...); err != nil {
				t.Fatal(err)
			}
			return events
		}
		ref, err := p.NewDocument(in, engine.WithBackend(b), record)
		if err != nil {
			t.Fatal(err)
		}
		d, err := p.NewDocument(in, engine.WithBackend(b), record, engine.WithTraceFilter("pair"), engine.WithTraceMaxDepth(3))
		if err != nil {
			t.Fatal(err)
		}
		for i, step := range []struct {
			opts     []engine.ParseOption
			rules    []string
			maxDepth int
		}{
			{nil, []string{"pair"}, 3},
			{[]engine.ParseOption{engine.WithTraceFilter("word")}, []string{"word"}, 3},
			{nil, []string{"pair"}, 3},
			{[]engine.ParseOption{engine.WithTraceMaxDepth(0)}, []string{"pair"}, 0},
			{nil, []string{"pair"}, 3},
		} {
			all := full(ref)
			want, got := codegentest.SelectedEvents(all, step.rules, step.maxDepth), full(d, step.opts...)
			if len(want) == 0 || len(want) == len(all) || !sameEvents(want, got) {
				t.Fatalf("%v: Parse %d selected\n%v\nwant\n%v", b, i+1, got, want)
			}
		}
	}
}
