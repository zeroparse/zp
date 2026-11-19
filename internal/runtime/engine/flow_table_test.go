package engine

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/backend/codegen/golang/print"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/driver/checked"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/middleend/flowpass"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// TestFlowTableFollowsGoNodeTable checks that the Node table the bytecode VM runs from Flow IR
// (checked.Source.FlowTable) is the one the generated Go parser with budget controls runs: for every
// rule body that the Go printer prints from Flow IR in its Node table, the VM's table holds a body of
// the same route (flat or framed) whose Flow IR after the Flow passes is the text the printer lowered.
// The parser is generated without inline-literals, the one Flow pass of the Go target that applies to
// the Node table under budget controls: the VM prints none of the target's forms. Rules the printer
// prunes have no Go route and are not compared.
func TestFlowTableFollowsGoNodeTable(t *testing.T) {
	paths, err := filepath.Glob("../../../testdata/grammars/*/*.zp")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no grammars: %v", err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			g, err := parseTest(string(src))
			if err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			texts := map[int][]string{} // the Flow functions of each rule that the Node table lowered
			routes := map[int]bool{}    // the rules the Node table prints, true for the flat route
			var gen *print.Generator
			hooks := &print.Hooks{
				Generator: func(x *print.Generator) {
					driverOf(x).FlowRun = func(table string, m *code.Module, env *flowpass.Env, o flowpass.RunOptions) ([]string, error) {
						ran, err := flowpass.Run(m, env, o)
						if table == "rules" {
							flowpass.Finish(m) // as the driver does after the passes; Finish is idempotent
							if a := flow.AttrsOf(flow.FuncOf(m)); a != nil && a.Rule != nil {
								mu.Lock()
								texts[a.Rule.ID] = append(texts[a.Rule.ID], code.FuncStringIn(m, flow.FuncOf(m)))
								mu.Unlock()
							}
						}
						return ran, err
					}
					if x.Table == "rules" {
						gen = x
					}
				},
				LeanRoute: func(table string, r *common.Rule) {
					if table == "rules" {
						routes[r.ID] = true
					}
				},
				FramedRoute: func(table string, r *common.Rule) {
					if table == "rules" {
						routes[r.ID] = false
					}
				},
			}
			opts := common.GenOptions{Package: "p", Module: "example.com/p", LimitControls: true, NoOpt: []string{flowpass.PassInlineLiterals}}
			if _, err := print.GenerateParser(g, checked.GeneratorCompiler(compileForGenerator), codegentest.Passes(opts), codegentest.WithLib(opts), hooks); err != nil {
				t.Fatal(err)
			}
			if gen == nil || len(routes) == 0 {
				t.Fatal("the Node table prints no rule from Flow IR")
			}
			ft, err := checked.Of(gen.Prog.Elab.(*compile.Program)).FlowTable()
			if err != nil {
				t.Fatal(err)
			}
			flat, framed := 0, 0
			for id, isFlat := range routes {
				m, ok := ft.Bodies[id]
				if !ok {
					t.Errorf("rule %d: printed from Flow IR in Go, no body in the VM's table", id)
					continue
				}
				fn := flow.FuncOf(m)
				if got := !flow.AttrsOf(fn).Frame; got != isFlat {
					t.Errorf("rule %s: flat %v in the VM's table, %v in Go", fn.Name, got, isFlat)
				}
				if text := code.FuncStringIn(m, fn); !slices.Contains(texts[id], text) {
					t.Errorf("rule %s: the VM's function is none of the %d that Go lowered:\n%s", fn.Name, len(texts[id]), text)
				}
				if isFlat {
					flat++
				} else {
					framed++
				}
			}
			t.Logf("%d flat and %d framed rule bodies agree", flat, framed)
		})
	}
}
