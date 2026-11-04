package complexitytest

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine"
)

// TestVarConstraintResearch classifies the grammars with variables under research hooks that
// model declared constraints on variables and prints what changes. Run with
// ZP_VARS_RESEARCH=<output file>.
func TestVarConstraintResearch(t *testing.T) {
	out := os.Getenv("ZP_VARS_RESEARCH")
	if out == "" {
		t.Skip("set ZP_VARS_RESEARCH")
	}
	paths := map[string]string{}
	for _, tg := range TriageGrammars {
		paths[tg.Name] = tg.Path
	}
	names := []string{"cue", "duckdb", "python", "yaml", "typescript", "outline", "xml", "cel", "golang", "minilang"}
	type exp struct {
		name  string
		setup func() func()
	}
	exps := []exp{
		{"base", func() func() { return func() {} }},
		{"const-strings", func() func() {
			compile.ResearchConstStrings = true
			return func() { compile.ResearchConstStrings = false }
		}},
		{"cap-65536", func() func() {
			old := compile.SetResearchMaxFiniteValues(65536)
			return func() { compile.SetResearchMaxFiniteValues(old) }
		}},
		{"vars-shareable (ceiling)", func() func() {
			compile.ResearchVarsShareable = true
			return func() { compile.ResearchVarsShareable = false }
		}},
		{"ranges+cap-65536+const-strings", func() func() {
			old := compile.SetResearchMaxFiniteValues(65536)
			compile.ResearchConstStrings = true
			compile.ResearchVarRange = map[string][2]int{"lv": {0, 3*8192 + 4096 + 4*512 + 511}}
			return func() {
				compile.SetResearchMaxFiniteValues(old)
				compile.ResearchConstStrings = false
				compile.ResearchVarRange = nil
			}
		}},
	}
	kind := func(w bool, m string) string {
		switch {
		case w && strings.Contains(m, "the binding"):
			return "W-binding"
		case w && strings.Contains(m, "this repeats in the recursion"):
			return "W-rec"
		case w:
			return "W-other"
		case strings.Contains(m, "without bound through a recursion"):
			return "R-nest"
		default:
			return "R-other"
		}
	}
	var b strings.Builder
	for _, name := range names {
		src, err := os.ReadFile(paths[name])
		if err != nil {
			t.Fatal(err)
		}
		var base map[string]bool
		for _, e := range exps {
			done := e.setup()
			g, err := zp.ParseGrammar(string(src))
			if err != nil {
				t.Fatal(err)
			}
			finite, sizes, err := compile.ResearchFiniteVars(g, "")
			if err != nil {
				t.Fatal(err)
			}
			c, err := engine.ClassifyComplexity(context.Background(), g, "")
			done()
			if err != nil {
				t.Fatal(err)
			}
			set := map[string]bool{}
			counts := map[string]int{}
			for _, x := range c.Causes {
				k := kind(true, x.Message)
				set[fmt.Sprintf("W|%d:%d %s", x.Pos.Line, x.Pos.Col, x.Message)] = true
				counts[k]++
			}
			for _, x := range c.Reasons {
				set[fmt.Sprintf("R|%d:%d %s", x.Pos.Line, x.Pos.Col, x.Message)] = true
				counts[kind(false, x.Message)]++
			}
			for _, x := range c.Env {
				set[fmt.Sprintf("E|%d:%d %s", x.Pos.Line, x.Pos.Col, x.Message)] = true
				counts["Env"]++
			}
			fmt.Fprintf(&b, "== %s [%s]: %v(%d) finite=%v sizes=%v counts=%v\n", name, e.name, c.Class, c.Degree, finite, sizes, counts)
			if base == nil {
				base = set
				var es []string
				for k := range set {
					if strings.HasPrefix(k, "E|") || strings.Contains(k, "the binding") {
						es = append(es, k)
					}
				}
				sort.Strings(es)
				for _, k := range es {
					fmt.Fprintf(&b, "   . %s\n", k)
				}
				continue
			}
			var diff []string
			for k := range base {
				if !set[k] {
					diff = append(diff, "  - "+k)
				}
			}
			for k := range set {
				if !base[k] {
					diff = append(diff, "  + "+k)
				}
			}
			sort.Strings(diff)
			for _, d := range diff {
				fmt.Fprintln(&b, d)
			}
		}
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
