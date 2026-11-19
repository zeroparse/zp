package print_test

import (
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/backend/codegen/golang/print"
	"zeroparse.dev/zp/internal/driver"
	"zeroparse.dev/zp/internal/driver/generate"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/pass"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// brokenGuards is a pass that runs after guards and leaves the first literal guard it finds out of
// step with its alternative, which core.Verify rejects.
var brokenGuards = pass.Pass[*core.Module, *corepass.Env]{
	Name:       "break-guard",
	After:      []string{corepass.PassGuards},
	Obligation: "none: it breaks the module on purpose",
	Run: func(m *core.Module, _ *corepass.Env) error {
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
func withBrokenGuards() func(*core.Module, *corepass.Env, corepass.RunOptions) ([]string, error) {
	p := *corepass.Pipeline
	p.Passes = nil
	for _, ps := range corepass.Pipeline.Passes {
		p.Passes = append(p.Passes, ps)
		if ps.Name == corepass.PassGuards {
			p.Passes = append(p.Passes, brokenGuards)
		}
	}
	return func(m *core.Module, env *corepass.Env, o corepass.RunOptions) ([]string, error) {
		// The mutant is not a registered optimization, which the settings of the generator would refuse.
		enabled := func(name string) bool { return name == brokenGuards.Name || env.On(name) }
		po := o.Options(env)
		po.Enabled = enabled
		return p.Run(m, env, po)
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
	opts := common.GenOptions{Types: true, TypedOnly: true, Module: "example.com/p", Lib: common.LibImport}
	hooks := driver.Driver{CoreRun: withBrokenGuards()}

	_, err = print.GenerateParser(pg, codegentest.Compile, generate.Passes(opts, hooks), codegentest.WithLib(opts), nil)
	if err == nil || !strings.Contains(err.Error(), "verification:") || strings.Contains(err.Error(), "break-guard") {
		t.Fatalf("without VerifyEachPass: %v", err)
	}
	opts.VerifyEachPass = true
	_, err = print.GenerateParser(pg, codegentest.Compile, generate.Passes(opts, hooks), codegentest.WithLib(opts), nil)
	if err == nil || !strings.Contains(err.Error(), "after pass break-guard:") {
		t.Fatalf("with VerifyEachPass: %v", err)
	}
	if !slices.Contains(corepass.Pipeline.Names(), corepass.PassGuards) {
		t.Fatal("no guards pass")
	}
}

// pipelineTables are builds that print functions of the given table from Flow IR and Exec IR: the
// Node table, the recognizer table and the typed table. The label of the table is the one of the
// dumps and errors.
var pipelineTables = []struct {
	table string
	opts  common.GenOptions
}{
	{"rules", common.GenOptions{Module: "example.com/p", Lib: common.LibImport}},
	{"recRules", common.GenOptions{Recognize: true, Module: "example.com/p", Lib: common.LibImport}},
	{"typed", common.GenOptions{Types: true, TypedOnly: true, Module: "example.com/p", Lib: common.LibImport}},
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
			hooks := driver.Driver{FlowRun: codegentest.BrokenFlowRun(c.table, &seen)}
			opts := c.opts
			_, err := print.GenerateParser(pg, codegentest.Compile, generate.Passes(opts, hooks), codegentest.WithLib(opts), nil)
			if err == nil || !strings.Contains(err.Error(), "of table "+c.table+": ") || !strings.Contains(err.Error(), "verification:") || strings.Contains(err.Error(), "break-flow") {
				t.Fatalf("without VerifyEachPass: %v", err)
			}
			if seen == 0 {
				t.Fatal("no function ran through the Flow pipeline")
			}
			opts.VerifyEachPass = true
			_, err = print.GenerateParser(pg, codegentest.Compile, generate.Passes(opts, hooks), codegentest.WithLib(opts), nil)
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
			hooks := driver.Driver{ExecRun: codegentest.BrokenExecRun(c.table, &seen)}
			opts := c.opts
			_, err := print.GenerateParser(pg, codegentest.Compile, generate.Passes(opts, hooks), codegentest.WithLib(opts), nil)
			if err == nil || !strings.Contains(err.Error(), "of table "+c.table+": ") || !strings.Contains(err.Error(), "verification:") || strings.Contains(err.Error(), "break-exec") {
				t.Fatalf("without VerifyEachPass: %v", err)
			}
			if seen == 0 {
				t.Fatal("no module ran through the Exec pipeline")
			}
			opts.VerifyEachPass = true
			_, err = print.GenerateParser(pg, codegentest.Compile, generate.Passes(opts, hooks), codegentest.WithLib(opts), nil)
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
	opts := common.GenOptions{Types: true, TypedOnly: true, Module: "example.com/p", Lib: common.LibImport, PrintAfter: "guards"}
	if _, err := print.GenerateParser(pg, codegentest.Compile, codegentest.Passes(opts), codegentest.WithLib(opts), nil); err == nil || !strings.Contains(err.Error(), "PrintTo") {
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
		run        func(m *code.Module)
	}{
		{"replace", "replaced or removed the function", func(m *code.Module) {
			f := *m.Funcs[0]
			m.Funcs[0] = &f
		}},
		{"drop local", "removed the local", func(m *code.Module) {
			for _, f := range m.Funcs {
				f.Locals = nil
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			hooks := driver.Driver{ExecRun: func(_ string, m *code.Module, env *codepass.Env, o codepass.RunOptions) ([]string, error) {
				ran, err := codepass.RunPasses(m, env, o)
				c.run(m)
				return ran, err
			}}
			opts := common.GenOptions{Module: "example.com/p", Lib: common.LibImport}
			if _, err := print.GenerateParser(pg, codegentest.Compile, generate.Passes(opts, hooks), codegentest.WithLib(opts), nil); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err %v, want %q", err, c.want)
			}
		})
	}
}
