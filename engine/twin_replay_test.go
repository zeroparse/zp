package engine_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
)

// TestBodyEvaluationsReplay checks, on the sample inputs of the grammars in testdata/grammars and
// on both engine backends, that two evaluations of the body of a rule at one position make the
// same calls with the same outcomes, whether one of them is inside a lookahead and the other not,
// or one evaluates the value-free twin of the rule and the other the rule (Evaluations of Rules in
// spec/runtime/parsing.md: a twin evaluates the body of its rule as the rule does). The trace
// names a twin as its rule, so the evaluations of one name, position and level are compared:
// those of rules on no left-recursion cycle that read no variable, whose evaluations no growth
// can tell apart.
func TestBodyEvaluationsReplay(t *testing.T) {
	chdirModuleRoot(t)
	dirs, err := filepath.Glob("testdata/grammars/*/*.zp")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no grammars: %v", err)
	}
	// Each case evaluates r as its twin and as the rule at one position, and inside and outside a
	// lookahead: r is called from two places, so it is memoized, and calls t from an optional.
	cases := []struct{ src, input string }{
		{"@start\nrule main = (-r \"x\" / r \"y\")* $$\nrule r = s t? s\nrule s = [a-c]*\nrule t = \"d\" [e-f]*\n", "adeyabdfyaxadey"},
		{"@start\nrule main = (&(r \"x\") r \"x\" / r \"y\")* $$\nrule r = s t? s\nrule s = [a-c]*\nrule t = \"d\" [e-f]*\n", "adexadeyabdfyax"},
	}
	type source struct {
		path, src string
		inputs    []string
	}
	var sources []source
	for i, c := range cases {
		sources = append(sources, source{fmt.Sprintf("case %d", i), c.src, []string{c.input}})
	}
	for _, path := range dirs {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.txt"))
		var inputs []string
		for _, in := range files {
			text, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			inputs = append(inputs, string(text))
		}
		sources = append(sources, source{path, string(src), inputs})
	}
	type frame struct {
		name     string
		level    int
		pos      int
		children []string
	}
	for _, so := range sources {
		path, src := so.path, so.src
		g, err := compiletest.Parse(src)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		prog, err := compiletest.Compile(g, compile.Options{})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		stable := map[string]bool{}
		for _, r := range prog.RuleList {
			stable[r.Name] = r.Cycle == 0 && len(r.Vars) == 0
		}
		p, err := engine.CompileSource(src, "")
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode} {
			compared, crossed := 0, 0
			for i, text := range so.inputs {
				seen := map[string]string{}
				states := map[string]bool{}
				var stack []*frame
				_, _ = p.Parse(text, engine.WithBackend(backend), engine.WithTrace(func(e engine.TraceEvent) {
					if e.Kind == engine.TraceEnter {
						stack = append(stack, &frame{name: e.Rule, level: e.Level, pos: e.Pos})
						return
					}
					f := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					if len(stack) > 0 {
						parent := stack[len(stack)-1]
						parent.children = append(parent.children, fmt.Sprintf("%s/%d@%d:%v:%d", e.Rule, e.Level, e.Pos, e.Matched, e.End))
					}
					if e.Memo || e.Evals != 1 || !stable[f.name] {
						return
					}
					key := fmt.Sprintf("%s/%d@%d", f.name, f.level, f.pos)
					calls := fmt.Sprintf("%v:%d [%s]", e.Matched, e.End, strings.Join(f.children, " "))
					if prev, ok := seen[key]; ok {
						compared++
						if states[key] != e.Lookahead {
							crossed++
						}
						if prev != calls {
							t.Errorf("%s input %d %v: %s evaluated as\n  %s\nand as\n  %s", path, i, backend, key, prev, calls)
						}
						return
					}
					seen[key], states[key] = calls, e.Lookahead
				}))
			}
			t.Logf("%s %v: %d repeated evaluations compared, %d across lookahead states", path, backend, compared, crossed)
		}
	}
}
