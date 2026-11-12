package gogen_test

import (
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gen/gogen"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/ir/coreir"
	"zeroparse.dev/zp/internal/ir/pipeline"
)

// brokenGuards is a pass that runs after guards and leaves the first literal guard it finds out of
// step with its alternative, which coreir.Verify rejects.
var brokenGuards = pipeline.Pass[*coreir.Module, *coreir.Env]{
	Name:       "break-guard",
	After:      []string{coreir.PassGuards},
	Obligation: "none: it breaks the module on purpose",
	Run: func(m *coreir.Module, _ *coreir.Env) error {
		for _, r := range m.Rules {
			if r == nil {
				continue
			}
			broke := false
			coreir.Walk(r.Body, func(n coreir.Node) {
				c, ok := n.(*coreir.Choice)
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
func withBrokenGuards() func(*coreir.Module, *coreir.Env, coreir.RunOptions) ([]string, error) {
	p := *coreir.Pipeline
	p.Passes = nil
	for _, ps := range coreir.Pipeline.Passes {
		p.Passes = append(p.Passes, ps)
		if ps.Name == coreir.PassGuards {
			p.Passes = append(p.Passes, brokenGuards)
		}
	}
	return func(m *coreir.Module, env *coreir.Env, o coreir.RunOptions) ([]string, error) {
		// The mutant is not a registered optimization, which the settings of the generator would refuse.
		enabled := func(name string) bool { return name == brokenGuards.Name || env.On(name) }
		return p.Run(m, env, pipeline.Options[*coreir.Module]{Enabled: enabled, VerifyEach: o.VerifyEach, Dump: o.Dump})
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
	hooks := &gogen.Hooks{CoreRun: withBrokenGuards()}

	_, err = gogen.GenerateParser(pg, gentest.Compile, opts, hooks)
	if err == nil || !strings.Contains(err.Error(), "verification:") || strings.Contains(err.Error(), "break-guard") {
		t.Fatalf("without VerifyEachPass: %v", err)
	}
	opts.VerifyEachPass = true
	_, err = gogen.GenerateParser(pg, gentest.Compile, opts, hooks)
	if err == nil || !strings.Contains(err.Error(), "after pass break-guard:") {
		t.Fatalf("with VerifyEachPass: %v", err)
	}
	if !slices.Contains(coreir.Pipeline.Names(), coreir.PassGuards) {
		t.Fatal("no guards pass")
	}
}

// A request to print the Core IR needs a writer to print to.
func TestPrintAfterNeedsAWriter(t *testing.T) {
	pg, err := parseTest("rule main = [a-z]+ $$\n")
	if err != nil {
		t.Fatal(err)
	}
	opts := gen.GenOptions{Types: true, TypedOnly: true, Module: "example.com/p", Runtime: gen.RuntimeImport, PrintAfter: "guards"}
	if _, err := gogen.GenerateParser(pg, gentest.Compile, opts, nil); err == nil || !strings.Contains(err.Error(), "PrintTo") {
		t.Fatalf("err %v", err)
	}
}
