package main

import (
	"bytes"
	"math"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestGeomean(t *testing.T) {
	if got := geomean(nil); got != 1 {
		t.Errorf("geomean of nothing = %v, want 1", got)
	}
	if got := geomean([]float64{0.5, 2}); math.Abs(got-1) > 1e-12 {
		t.Errorf("geomean(0.5, 2) = %v, want 1", got)
	}
	if got := geomean([]float64{4, 9}); math.Abs(got-6) > 1e-12 {
		t.Errorf("geomean(4, 9) = %v, want 6", got)
	}
}

func TestRotateStartsEachVariantOnce(t *testing.T) {
	v := []string{"a", "b", "c"}
	first := map[string]int{}
	for round := 0; round < 6; round++ {
		r := rotate(v, round)
		if len(r) != 3 {
			t.Fatalf("round %d: %v", round, r)
		}
		first[r[0]]++
	}
	if !reflect.DeepEqual(first, map[string]int{"a": 2, "b": 2, "c": 2}) {
		t.Errorf("first positions over six rounds = %v", first)
	}
	if got := rotate(v, 1); !reflect.DeepEqual(got, []string{"b", "c", "a"}) {
		t.Errorf("rotate(v, 1) = %v", got)
	}
	if v[0] != "a" {
		t.Errorf("rotate changed its argument: %v", v)
	}
}

func TestSummarize(t *testing.T) {
	c := cells{}
	for g, vals := range map[string][2]float64{"x": {100, 90}, "y": {200, 210}, "z": {50, 25}} {
		c.set(g, "base", "src_bytes", vals[0])
		c.set(g, "small", "src_bytes", vals[1])
	}
	c.set("w", "base", "src_bytes", 0) // a zero base has no ratio
	c.set("w", "small", "src_bytes", 5)
	s := c.summarize([]string{"x", "y", "z", "w", "missing"}, "small", "base", "src_bytes", 0.05)
	if len(s.Ratios) != 3 {
		t.Errorf("ratios = %v, want three", s.Ratios)
	}
	if s.Better != 2 {
		t.Errorf("better = %d, want 2 (x at 0.9 and z at 0.5)", s.Better)
	}
	if s.Worst != 1.05 {
		t.Errorf("worst = %v, want 1.05", s.Worst)
	}
	// A ratio exactly at 1-threshold is not better, and with no threshold only a smaller ratio is.
	if s := c.summarize([]string{"x", "y", "z"}, "small", "base", "src_bytes", 0.5); s.Better != 0 {
		t.Errorf("better = %d at threshold 0.5, want 0 (z is exactly 0.5)", s.Better)
	}
	c.set("same", "base", "src_bytes", 10)
	c.set("same", "small", "src_bytes", 10)
	if s := c.summarize([]string{"same", "y", "x"}, "small", "base", "src_bytes", 0); s.Better != 1 {
		t.Errorf("better = %d at threshold 0, want 1 (x only: same is unchanged)", s.Better)
	}
	want := math.Cbrt(0.9 * 1.05 * 0.5)
	if math.Abs(s.Geomean-want) > 1e-12 {
		t.Errorf("geomean = %v, want %v", s.Geomean, want)
	}
}

func TestKeepBest(t *testing.T) {
	a := measurement{NS: 10, GCShare: 0.5, Bytes: 8, Allocs: 3, Retained: 100, RSS: 50}
	b := measurement{NS: 9, GCShare: 0.1, Bytes: 9, Allocs: 2, Retained: 120, RSS: 60}
	got := keepBest(a, b)
	want := measurement{NS: 9, GCShare: 0.1, Bytes: 8, Allocs: 2, Retained: 100, RSS: 50}
	if got != want {
		t.Errorf("keepBest = %+v, want %+v", got, want)
	}
}

func TestFormatValue(t *testing.T) {
	for in, want := range map[float64]string{12: "12", 0: "0", 0.25: "0.25", 1234567.5: "1.23457e+06"} {
		if got := formatValue(in); got != want {
			t.Errorf("formatValue(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestRunValidatesItsConfiguration(t *testing.T) {
	var out, log bytes.Buffer
	if err := run(config{Root: "../..", Rounds: 1}, &out, &log); err == nil || !strings.Contains(err.Error(), "no -variant") {
		t.Errorf("no variant: %v", err)
	}
	cfg := config{Root: "../..", Rounds: 0, Variants: []variant{{Name: "a"}}}
	if err := run(cfg, &out, &log); err == nil || !strings.Contains(err.Error(), "-rounds") {
		t.Errorf("no rounds: %v", err)
	}
	cfg = config{Root: "../..", Rounds: 1, Variants: []variant{{Name: "a"}, {Name: "a"}}}
	if err := run(cfg, &out, &log); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("duplicate variant: %v", err)
	}
	cfg = config{Root: "../..", Rounds: 1, Control: true, Variants: []variant{{Name: "control"}}}
	if err := run(cfg, &out, &log); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("variant named control with -control: %v", err)
	}
}

// parseCells reads the cell lines of an output into value[grammar][variant][metric].
func parseCells(t *testing.T, out string) cells {
	t.Helper()
	c := cells{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) != 5 || f[0] != "cell" {
			continue
		}
		v, err := strconv.ParseFloat(f[4], 64)
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		c.set(f[1], f[2], f[3], v)
	}
	return c
}

// TestRunMeasuresEveryMetric generates json twice, with and without the guards, builds and measures
// both and a control copy of the first, and checks what the output must satisfy: every metric is
// present; the sizes of the control equal the baseline's and its allocation is close, so a difference
// between variants is the option and not the harness; and turning the guards off changes the generated source.
func TestRunMeasuresEveryMetric(t *testing.T) {
	if testing.Short() {
		t.Skip("generates, builds and runs parsers")
	}
	cfg := config{
		Root: "../..", Grammars: []string{"json"}, Rounds: 2, Benchtime: "30x", Control: true, TS: true,
		Variants: []variant{{Name: "speed"}, {Name: "noguards", Args: []string{"-no-opt", "guards"}}},
	}
	var out, log bytes.Buffer
	if err := run(cfg, &out, &log); err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	c := parseCells(t, out.String())
	for _, v := range []string{"speed", "noguards", "control"} {
		for _, m := range metrics {
			x, ok := c.get("json", v, m.name)
			if !ok {
				t.Errorf("%s %s is missing", v, m.name)
				continue
			}
			if x < 0 {
				t.Errorf("%s %s = %v", v, m.name, x)
			}
			if x == 0 && m.name != "gc_share" && !(m.name == "rss" && runtime.GOOS == "windows") && m.name != "compile_ms" {
				t.Errorf("%s %s is zero", v, m.name)
			}
		}
	}
	for _, m := range []string{"src_bytes", "bin_bytes", "ts_raw", "ts_gzip"} {
		a, _ := c.get("json", "speed", m)
		b, _ := c.get("json", "control", m)
		if a != b {
			t.Errorf("control %s = %v, the baseline has %v", m, b, a)
		}
	}
	// Allocation depends on when the collector runs, so these are equal only approximately.
	for _, m := range []string{"b_op", "allocs_op", "retained"} {
		if r, _ := c.ratio("json", "control", "speed", m); r < 0.9 || r > 1.1 {
			t.Errorf("control %s is %.3f times the baseline's", m, r)
		}
	}
	a, _ := c.get("json", "speed", "src_bytes")
	b, _ := c.get("json", "noguards", "src_bytes")
	if a == b {
		t.Errorf("turning the guards off left the source at %v bytes", a)
	}
	if !strings.Contains(out.String(), "ratio to speed: src_bytes") {
		t.Errorf("no ratio table for src_bytes:\n%s", out.String())
	}
}
