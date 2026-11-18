package engine

import (
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/flowlower"
)

// TestFlowVerifiesCorpus lowers every typed rule of the corpus that Flow IR covers, from the Core
// IR of each mode of the Core comparison with the generator's lowering options (observation in
// the observable mode), and verifies the functions: the rules that LocalRule
// accepts to local functions, and the bodies of the other rules and the lines of Operators Rules
// to framed ones.
func TestFlowVerifiesCorpus(t *testing.T) {
	// The short tier leaves out the grammars over 20000 bytes (codegentest.CorpusWithParsers), which
	// hold most of the rules (952 of 4209 lowered rules remain), so it has its own floor.
	minLowered, minFramed := 1000, 100
	if testing.Short() {
		minLowered = 900
	}
	for _, mode := range coreGenModes {
		t.Run(mode.name, func(t *testing.T) {
			lowered, framed, observed, skips := 0, 0, 0, 0
			for _, gen := range coreGenerations(t, mode) {
				g := gen.gen
				m, err := g.TypedCore(map[core.Node]grammar.Expr{})
				if err != nil {
					t.Fatalf("%s: %v", gen.name, err)
				}
				// The options of the generator, which lowers while it generates the typed table.
				table := g.Table
				g.Table = "trules"
				opts := g.FlowOptions(m)
				g.Table = table
				if opts.Observe != g.Observability {
					t.Fatalf("%s: lowering options do not follow the observability of the mode", gen.name)
				}
				if g.Observability {
					observed++
				}
				check := func(f *code.Module, err error) {
					if err == nil {
						err = flow.VerifyModule(f)
						skips += observeSkips(flow.FuncOf(f).Body)
					}
					if err != nil {
						t.Errorf("%s: %v", gen.name, err)
					}
				}
				for _, r := range m.Rules {
					if flowlower.LocalRule(r) {
						check(flowlower.Lower(m, r, r.Name, opts))
						lowered++
						continue
					}
					if o, ok := r.Body.(*core.OperatorsRule); ok {
						for _, l := range o.Lines() {
							if flowlower.FramedBody(l.Body) {
								check(flowlower.LowerFrame(m, r, l.Body, l.Scope, l.Body.Attrs().Want, r.Name, opts))
								framed++
							}
						}
					} else if flowlower.FramedBody(r.Body) {
						check(flowlower.LowerFrame(m, r, r.Body, r.Scope, r.Want, r.Name, opts))
						framed++
					}
				}
			}
			t.Logf("lowered %d rules and %d framed bodies; %d grammars observed, %d observed skips", lowered, framed, observed, skips)
			if lowered < minLowered || framed < minFramed {
				t.Errorf("lowered %d rules and %d framed bodies, want at least %d and %d", lowered, framed, minLowered, minFramed)
			}
			if mode.opts.Observability && (observed < 100 || skips < 100) || !mode.opts.Observability && skips != 0 {
				t.Errorf("lowered %d grammars with observation, with %d observed skips", observed, skips)
			}
		})
	}
}

// observeSkips counts the calls of observeSkip in b and in the blocks nested in it.
func observeSkips(b *code.Block) int {
	n := 0
	code.Walk(b, nil, func(x code.Expr) {
		if c, ok := x.(*code.Call); ok && c.Callee == flow.OpObserveSkip {
			n++
		}
	})
	return n
}
