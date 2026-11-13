package gengo_test

import (
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gen/gengo"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/pipeline"
)

// brokenGuards is a pass that runs after guards and leaves the first literal guard it finds out of
// step with its alternative, which core.Verify rejects.
var brokenGuards = pipeline.Pass[*core.Module, *core.Env]{
	Name:       "break-guard",
	After:      []string{core.PassGuards},
	Obligation: "none: it breaks the module on purpose",
	Run: func(m *core.Module, _ *core.Env) error {
		for _, r := range m.Rules {
			if r == nil {
				continue
			}
			broke := false
			core.Walk(r.Body, func(n core.Node) {
				c, ok := n.(*core.Choice)
				if !ok || broke {
					return
				}
				for _, a := range c.Alts {
					if a.Guard != nil && a.Guard.Lit {
						g := *a.Guard
						g.Rune++
						a.Guard = &g
						broke = true
						return
					}
				}
			})
			if broke {
				return nil
			}
		}
		return nil
	},
}

// withBrokenGuards returns a Hooks.CoreRun that runs the pipeline of Core passes with brokenGuards
// added after the pass guards.
func withBrokenGuards() func(*core.Module, *core.Env, core.RunOptions) ([]string, error) {
	p := *core.Pipeline
	p.Passes = nil
	for _, ps := range core.Pipeline.Passes {
		p.Passes = append(p.Passes, ps)
		if ps.Name == core.PassGuards {
			p.Passes = append(p.Passes, brokenGuards)
		}
	}
	return func(m *core.Module, env *core.Env, o core.RunOptions) ([]string, error) {
		// The mutant is not a registered optimization, which the settings of the generator would refuse.
		enabled := func(name string) bool { return name == brokenGuards.Name || env.On(name) }
		return p.Run(m, env, pipeline.Options[*core.Module]{Enabled: enabled, VerifyEach: o.VerifyEach, Dump: o.Dump})
	}
}

// A pass that breaks the Core IR is caught: at the end of the run without VerifyEachPass, and by name
// right after the pass with it. The mutant is no pass of the pipeline, so this shows that the check
// between passes works in the generators, not that the passes are right.
func TestVerifyEachPassNamesTheBreakingPass(t *testing.T) {
	pg, err := parseTest(`rule main = (word / num / -" ")* $$
rule word = "if" / "else" / [a-z]+
rule num = [0-9]+
`)
	if err != nil {
		t.Fatal(err)
	}
	opts := gen.GenOptions{Types: true, TypedOnly: true, Module: "example.com/p", Runtime: gen.RuntimeImport}
	hooks := &gengo.Hooks{CoreRun: withBrokenGuards()}

	_, err = gengo.GenerateParser(pg, gentest.Compile, gentest.WithRuntime(opts), hooks)
	if err == nil || !strings.Contains(err.Error(), "verification:") || strings.Contains(err.Error(), "break-guard") {
		t.Fatalf("without VerifyEachPass: %v", err)
	}
	opts.VerifyEachPass = true
	_, err = gengo.GenerateParser(pg, gentest.Compile, gentest.WithRuntime(opts), hooks)
	if err == nil || !strings.Contains(err.Error(), "after pass break-guard:") {
		t.Fatalf("with VerifyEachPass: %v", err)
	}
	if !slices.Contains(core.Pipeline.Names(), core.PassGuards) {
		t.Fatal("no guards pass")
	}
}

// pipelineTables are builds that print functions of the given table from Flow IR and Exec IR: the
// Node table, the recognizer table and the typed table. The label of the table is the one of the
// dumps and errors.
var pipelineTables = []struct {
	table string
	opts  gen.GenOptions
}{
	{"rules", gen.GenOptions{Module: "example.com/p", Runtime: gen.RuntimeImport}},
	{"recRules", gen.GenOptions{Recognize: true, Module: "example.com/p", Runtime: gen.RuntimeImport}},
	{"typed", gen.GenOptions{Types: true, TypedOnly: true, Module: "example.com/p", Runtime: gen.RuntimeImport}},
}

// Every function that the generator prints from Flow IR runs through the Flow pipeline, in each
// table, and a Flow pass that breaks a function is caught by the Flow verifier: at the end of the run
// without VerifyEachPass, and by name right after the pass with it.
func TestVerifyEachPassNamesTheBreakingFlowPass(t *testing.T) {
	pg, err := parseTest(`rule main = (word / num / -" ")* $$
rule word = "if" / "else" / [a-z]+
rule num = [0-9]+
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range pipelineTables {
		t.Run(c.table, func(t *testing.T) {
			seen := 0
			hooks := &gengo.Hooks{FlowRun: gentest.BrokenFlowRun(c.table, &seen)}
			opts := c.opts
			_, err := gengo.GenerateParser(pg, gentest.Compile, gentest.WithRuntime(opts), hooks)
			if err == nil || !strings.Contains(err.Error(), "of table "+c.table+": ") || !strings.Contains(err.Error(), "verification:") || strings.Contains(err.Error(), "break-flow") {
				t.Fatalf("without VerifyEachPass: %v", err)
			}
			if seen == 0 {
				t.Fatal("no function ran through the Flow pipeline")
			}
			opts.VerifyEachPass = true
			_, err = gengo.GenerateParser(pg, gentest.Compile, gentest.WithRuntime(opts), hooks)
			if err == nil || !strings.Contains(err.Error(), "of table "+c.table+": ") || !strings.Contains(err.Error(), "after pass break-flow:") {
				t.Fatalf("with VerifyEachPass: %v", err)
			}
		})
	}
}

// Every function that the generator prints from Exec IR runs through the Exec pipeline, in each
// table, and an Exec pass that breaks a module is caught by the Exec verifier: at the end of the run
// without VerifyEachPass, and by name right after the pass with it.
func TestVerifyEachPassNamesTheBreakingExecPass(t *testing.T) {
	pg, err := parseTest(`rule main = (word / num / -" ")* $$
rule word = "if" / "else" / [a-z]+
rule num = [0-9]+
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range pipelineTables {
		t.Run(c.table, func(t *testing.T) {
			seen := 0
			hooks := &gengo.Hooks{ExecRun: gentest.BrokenExecRun(c.table, &seen)}
			opts := c.opts
			_, err := gengo.GenerateParser(pg, gentest.Compile, gentest.WithRuntime(opts), hooks)
			if err == nil || !strings.Contains(err.Error(), "of table "+c.table+": ") || !strings.Contains(err.Error(), "verification:") || strings.Contains(err.Error(), "break-exec") {
				t.Fatalf("without VerifyEachPass: %v", err)
			}
			if seen == 0 {
				t.Fatal("no module ran through the Exec pipeline")
			}
			opts.VerifyEachPass = true
			_, err = gengo.GenerateParser(pg, gentest.Compile, gentest.WithRuntime(opts), hooks)
			if err == nil || !strings.Contains(err.Error(), "of table "+c.table+": ") || !strings.Contains(err.Error(), "after pass break-exec:") {
				t.Fatalf("with VerifyEachPass: %v", err)
			}
		})
	}
}

// A request to print the Core IR needs a writer to print to.
func TestPrintAfterNeedsAWriter(t *testing.T) {
	pg, err := parseTest("rule main = [a-z]+ $$\n")
	if err != nil {
		t.Fatal(err)
	}
	opts := gen.GenOptions{Types: true, TypedOnly: true, Module: "example.com/p", Runtime: gen.RuntimeImport, PrintAfter: "guards"}
	if _, err := gengo.GenerateParser(pg, gentest.Compile, gentest.WithRuntime(opts), nil); err == nil || !strings.Contains(err.Error(), "PrintTo") {
		t.Fatalf("err %v", err)
	}
}

// An Exec pass keeps the function that the lowering made and the locals of the Flow IR function in
// it, which the printers print: a pass that replaces the function or drops such a local fails the
// generation, though the module it leaves is well formed.
func TestExecPassKeepsTheLoweredFunction(t *testing.T) {
	pg, err := parseTest(`rule main = (word / num / -" ")* $$
rule word = "if" / "else" / [a-z]+
rule num = [0-9]+
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, want string
		run        func(m *exec.Module)
	}{
		{"replace", "replaced or removed the function", func(m *exec.Module) {
			f := *m.Funcs[0]
			m.Funcs[0] = &f
		}},
		{"drop local", "removed the local", func(m *exec.Module) {
			for _, f := range m.Funcs {
				f.Locals = nil
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			hooks := &gengo.Hooks{ExecRun: func(_ string, m *exec.Module, env *exec.Env, o exec.RunOptions) ([]string, error) {
				ran, err := exec.RunPasses(m, env, o)
				c.run(m)
				return ran, err
			}}
			opts := gen.GenOptions{Module: "example.com/p", Runtime: gen.RuntimeImport}
			if _, err := gengo.GenerateParser(pg, gentest.Compile, gentest.WithRuntime(opts), hooks); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err %v, want %q", err, c.want)
			}
		})
	}
}
