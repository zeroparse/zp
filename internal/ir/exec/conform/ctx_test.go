package conform

import (
	"math/rand/v2"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

var ctxModule = CtxModule()

func ctxSuite(name string, mu *mutant) Suite {
	return Suite{Name: name, Ops: CtxSuiteOps(), New: func(c ref.Config) Impl {
		if mu == nil {
			return modelImpl{ref.New(c)}
		}
		mu.modelImpl = modelImpl{ref.New(c)}
		return *mu
	}}
}

func TestCtxCasesVerify(t *testing.T) {
	if err := exec.Verify(CtxCases()[0].Module); err != nil {
		t.Fatal(err)
	}
	if err := exec.Verify(RandomCtx("random", rand.New(rand.NewPCG(1, 2)), 200).Module); err != nil {
		t.Fatal(err)
	}
}

const ctxSeeds, ctxCalls = 200, 250

func ctxFails(s Suite) (handwritten, random bool) {
	for _, c := range CtxCases() {
		if _, err := Run(s, c); err != nil {
			handwritten = true
		}
	}
	for seed := uint64(0); seed < ctxSeeds && !random; seed++ {
		_, err := Run(s, RandomCtx("random", rand.New(rand.NewPCG(seed, 9)), ctxCalls))
		random = err != nil
	}
	return handwritten, random
}

func TestModelPassesCtxCases(t *testing.T) {
	s := ctxSuite("model", nil)
	for _, c := range CtxCases() {
		if skipped, err := Run(s, c); err != nil || skipped {
			t.Errorf("%s: skipped %v, %v", c.Func.Name, skipped, err)
		}
	}
	for seed := uint64(0); seed < ctxSeeds; seed++ {
		if _, err := Run(s, RandomCtx("random", rand.New(rand.NewPCG(seed, 9)), ctxCalls)); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRandomCtxCasesUseTheOperations checks that randomized cases exercise every operation, and reach
// results kept in a context and the outcomes of lookaheads.
func TestRandomCtxCasesUseTheOperations(t *testing.T) {
	counts := map[string]int{}
	found := 0
	for seed := uint64(0); seed < ctxSeeds; seed++ {
		c := RandomCtx("random", rand.New(rand.NewPCG(seed, 9)), ctxCalls)
		walkCalls(c.Func.Body, func(x *exec.Call) { counts[x.Callee]++ })
		walkCalls(c.Func.Body, func(x *exec.Call) {
			if x.Callee != "ctx.replay" {
				return
			}
			if b, ok := x.Args[1].(*exec.Const); ok && b.Value == false {
				found++
			}
		})
	}
	for _, name := range append(CtxOps, CtxTestOps...) {
		if counts[name] < 20 {
			t.Errorf("randomized cases call %s %d times", name, counts[name])
		}
	}
	if found < 30 {
		t.Errorf("randomized cases replay %d results kept in a context", found)
	}
}

// TestCtxMutantsFail is the negative control of the cases of the dependency memo: each broken model
// fails a handwritten or a randomized case.
func TestCtxMutantsFail(t *testing.T) {
	for _, br := range CtxBrokenOps() {
		t.Run(br.Name, func(t *testing.T) {
			mu := mutant{op: br.Op, edit: func(m *ref.Model, a []any) []any {
				return br.Edit(func(op string, args ...any) []any { return m.Call(ctxModule.Op(op), args) }, a)
			}}
			handwritten, random := ctxFails(ctxSuite(br.Name, &mu))
			if !handwritten && !random {
				t.Error("no case detected the mutant")
			}
			if !random {
				t.Error("no randomized case detected the mutant")
			}
		})
	}
}
