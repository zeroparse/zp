package common_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// The optimizations of a generator are the settings of its Runner, the one holder of them, so that
// the printers (Generator.Opt) and the passes that the Runner runs cannot disagree.
func TestGeneratorOptIsTheRunnerSettings(t *testing.T) {
	t.Parallel()
	p := codegentest.Passes(common.GenOptions{NoOpt: []string{"method-sharing"}})
	r := p.Runner(nil)
	g := &common.Generator{Runner: r}
	if g.Opt().On("method-sharing") || r.Settings().On("method-sharing") {
		t.Fatal("NoOpt method-sharing is on in the generator or in the runner")
	}
	if got, want := g.Opt().On("literal-sharing"), r.Settings().On("literal-sharing"); got != want || !got {
		t.Fatalf("literal-sharing: generator %v, runner %v, want both on", got, want)
	}
}

// A generator without a Runner is a programming error that fails with a message that says so.
func TestGeneratorWithoutRunnerPanicsClearly(t *testing.T) {
	t.Parallel()
	for name, use := range map[string]func(g *common.Generator){
		"Opt":        func(g *common.Generator) { g.Opt() },
		"Core":       func(g *common.Generator) { _, _ = g.Core("rules", nil) },
		"FlowPasses": func(g *common.Generator) { _, _ = g.FlowPasses(flowlower.Unit{}) },
		"ExecPasses": func(g *common.Generator) { _, _ = g.ExecPasses(nil, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				v := recover()
				if s, ok := v.(string); !ok || !strings.Contains(s, "no Runner") {
					t.Fatalf("panic %v, want a message about the missing Runner", v)
				}
			}()
			use(&common.Generator{})
		})
	}
}
