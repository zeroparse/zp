package print_test

import (
	"runtime"
	"sync"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/backend/codegen/golang/print"
	irbuild "zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/testing/codegentest"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// typedEnv returns the environment in which Generator.TypedCore runs the pipeline for g.
func typedEnv(g *print.Generator) *corepass.Env {
	return &corepass.Env{Flag: g.Opt().On}
}

// pipelinedTypedCore runs the pipeline on the typed rules of g.
func pipelinedTypedCore(g *print.Generator, verifyEach bool) (*core.Module, error) {
	m, err := irbuild.CoreFrom(g.Prog.Elab, map[core.Node]ast.Expr{})
	if err != nil {
		return nil, err
	}
	m, _, err = corepass.Run(m, typedEnv(g), corepass.RunOptions{VerifyEach: verifyEach})
	return m, err
}

// The pipeline runs on the typed rules of every grammar of the corpus in every mode, and the module
// verifies after every pass and gives the same Core IR with or without verification between the passes
// (verification only reads).
func TestCorePipelineVerifiesEveryPass(t *testing.T) {
	modes := []struct {
		name string
		opts common.GenOptions
	}{
		{"lean", common.GenOptions{Types: true, TypedOnly: true}},
		{"lean observable", common.GenOptions{Types: true, TypedOnly: true, Observability: true}},
		{"controlled", common.GenOptions{Types: true, TypedOnly: true, LimitControls: true}},
		{"lean boxed", common.GenOptions{Types: true, TypedOnly: true, NoOpt: []string{"text-dispatch", "field-compare"}}},
		{"lean without guarded classes", common.GenOptions{Types: true, TypedOnly: true, NoOpt: []string{"guarded-classes"}}},
		{"lean without terminals", common.GenOptions{Types: true, TypedOnly: true, NoOpt: []string{"terminals"}}},
		{"document", common.GenOptions{Types: true, Document: true}},
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
				var g *print.Generator
				opts := mode.opts
				opts.Module = "example.com/p"
				if _, err := print.GenerateParser(pg, codegentest.Compile, codegentest.Passes(opts), codegentest.WithLib(opts), &print.Hooks{Generator: func(x *print.Generator) { g = x }}); err != nil || g == nil {
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
