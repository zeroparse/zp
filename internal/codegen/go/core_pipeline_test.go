package gogen_test

import (
	"runtime"
	"sync"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegen/go"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/ir/core"
	irbuild "zeroparse.dev/zp/internal/ir/core/build"
)

// typedEnv returns the environment in which Generator.TypedCore runs the pipeline for g.
func typedEnv(g *gogen.Generator) *core.Env {
	return &core.Env{
		Mode:  core.Mode{Lean: g.Ordinary, Ordinary: g.Ordinary, Observe: g.Observability},
		Flag:  g.Opt.On,
		Local: gogen.CoreLocalRule,
	}
}

// pipelinedTypedCore runs the pipeline on the typed rules of g.
func pipelinedTypedCore(g *gogen.Generator, verifyEach bool) (*core.Module, error) {
	m, err := irbuild.CoreFrom(g.Prog, map[core.Node]grammar.Expr{})
	if err != nil {
		return nil, err
	}
	_, err = core.Run(m, typedEnv(g), core.RunOptions{VerifyEach: verifyEach})
	return m, err
}

// The pipeline runs on the typed rules of every grammar of the corpus in every mode, and the module
// verifies after every pass and gives the same Core IR with or without verification between the passes
// (verification only reads).
func TestCorePipelineVerifiesEveryPass(t *testing.T) {
	modes := []struct {
		name string
		opts codegen.GenOptions
	}{
		{"lean", codegen.GenOptions{Types: true, TypedOnly: true}},
		{"lean observable", codegen.GenOptions{Types: true, TypedOnly: true, Observability: true}},
		{"controlled", codegen.GenOptions{Types: true, TypedOnly: true, LimitControls: true}},
		{"lean boxed", codegen.GenOptions{Types: true, TypedOnly: true, NoOpt: []string{"text-dispatch", "field-compare"}}},
		{"lean without guarded classes", codegen.GenOptions{Types: true, TypedOnly: true, NoOpt: []string{"guarded-classes"}}},
		{"lean without terminals", codegen.GenOptions{Types: true, TypedOnly: true, NoOpt: []string{"terminals"}}},
		{"document", codegen.GenOptions{Types: true, Document: true}},
	}
	cases := append(codegentest.CorpusWithParsers(t), codegentest.CoreExtraGrammars...)
	compared := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for _, mode := range modes {
		for _, c := range cases {
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				pg, err := parseTest(c.Src)
				if err != nil {
					return
				}
				var g *gogen.Generator
				opts := mode.opts
				opts.Module = "example.com/p"
				if _, err := gogen.GenerateParser(pg, codegentest.Compile, codegentest.WithLib(opts), &gogen.Hooks{Generator: func(x *gogen.Generator) { g = x }}); err != nil || g == nil {
					return
				}
				want, err := pipelinedTypedCore(g, false)
				if err != nil {
					t.Errorf("%s: %s: %v", mode.name, c.Name, err)
					return
				}
				if len(want.Rules) == 0 {
					return
				}
				got, err := pipelinedTypedCore(g, true)
				if err != nil {
					t.Errorf("%s: %s: %v", mode.name, c.Name, err)
					return
				}
				if a, b := core.Print(want, true), core.Print(got, true); a != b {
					t.Errorf("%s: %s: verifying between the passes changes the module", mode.name, c.Name)
					return
				}
				mu.Lock()
				compared++
				mu.Unlock()
			})
		}
	}
	wg.Wait()
	if compared < 500 {
		t.Fatalf("compared %d modules", compared)
	}
}
