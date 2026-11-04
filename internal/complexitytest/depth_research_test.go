package complexitytest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/starttest"
)

// TestDepthBoundResearch classifies each bundled grammar and each rescanned-calls control with
// and without the depth-bound hook and prints the causes that the hook removes. Run with
// ZP_DEPTH_RESEARCH=1.
func TestDepthBoundResearch(t *testing.T) {
	if os.Getenv("ZP_DEPTH_RESEARCH") == "" {
		t.Skip("set ZP_DEPTH_RESEARCH=1")
	}
	type item struct{ name, src, start string }
	var items []item
	for _, tg := range append(append([]TriageGrammar{}, TriageGrammars...), LinearGrammars...) {
		b, err := os.ReadFile(tg.Path)
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, item{tg.Name, string(b), ""})
	}
	for _, c := range Controls() {
		if c.Family == "rescanned calls" || strings.HasPrefix(c.Family, "E") {
			items = append(items, item{"control: " + c.Name, c.Source, "main"})
		}
	}
	classify := func(src, start string, hook bool) *engine.Complexity {
		g, err := zp.ParseGrammar(src)
		if err != nil {
			t.Fatal(err)
		}
		compile.DepthBoundResearch = hook
		defer func() { compile.DepthBoundResearch = false }()
		if start != "" {
			g = starttest.MarkAll(g)
		}
		c, err := engine.ClassifyComplexity(context.Background(), g, start)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	key := func(w bool, x engine.ComplexityCause) string {
		return fmt.Sprintf("%v|%d:%d %s: %s", w, x.Pos.Line, x.Pos.Col, x.Rule, x.Message)
	}
	kind := func(w bool, m string) string {
		switch {
		case w && strings.Contains(m, "this repeats in the recursion"):
			return "W-rec"
		case w && strings.Contains(m, "this repeats in"):
			return "W-rep"
		case w:
			return "W-other"
		case strings.Contains(m, "without bound through a recursion"):
			return "R-nest"
		default:
			return "R-other"
		}
	}
	out := &strings.Builder{}
	for _, it := range items {
		a, b := classify(it.src, it.start, false), classify(it.src, it.start, true)
		set := func(c *engine.Complexity) map[string]string {
			m := map[string]string{}
			for _, x := range c.Causes {
				m[key(true, x)] = kind(true, x.Message)
			}
			for _, x := range c.Reasons {
				m[key(false, x)] = kind(false, x.Message)
			}
			return m
		}
		sa, sb := set(a), set(b)
		removed, added := map[string]int{}, map[string]int{}
		var rl, al []string
		for k, v := range sa {
			if _, ok := sb[k]; !ok {
				removed[v]++
				rl = append(rl, k)
			}
		}
		for k, v := range sb {
			if _, ok := sa[k]; !ok {
				added[v]++
				al = append(al, k)
			}
		}
		remain := map[string]int{}
		for _, v := range sb {
			remain[v]++
		}
		fmt.Fprintf(out, "== %s: %v(%d) W=%d R=%d -> %v(%d) W=%d R=%d removed=%v added=%v remain=%v\n", it.name,
			a.Class, a.Degree, len(a.Causes), len(a.Reasons), b.Class, b.Degree, len(b.Causes), len(b.Reasons), removed, added, remain)
		for _, k := range rl {
			fmt.Fprintf(out, "  - %s\n", k)
		}
		for _, k := range al {
			fmt.Fprintf(out, "  + %s\n", k)
		}
	}
	if err := os.WriteFile(os.Getenv("ZP_DEPTH_RESEARCH"), []byte(out.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
