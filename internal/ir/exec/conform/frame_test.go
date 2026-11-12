package conform

import (
	"math/rand/v2"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

var frameModule = FrameModule()

func frameSuite(name string, mu *mutant) Suite {
	return Suite{Name: name, Ops: FrameSuiteOps(true), New: func(c ref.Config) Impl {
		if mu == nil {
			return modelImpl{ref.New(c)}
		}
		mu.modelImpl = modelImpl{ref.New(c)}
		return *mu
	}}
}

func TestFrameCasesVerify(t *testing.T) {
	if err := exec.Verify(FrameCases()[0].Module); err != nil {
		t.Fatal(err)
	}
	if err := exec.Verify(RandomFrame("random", rand.New(rand.NewPCG(1, 2)), FrameRandomOps(true), 200).Module); err != nil {
		t.Fatal(err)
	}
}

const frameSeeds, frameCalls = 200, 200

func frameFails(s Suite) (handwritten, random bool) {
	for _, c := range FrameCases() {
		if _, err := Run(s, c); err != nil {
			handwritten = true
		}
	}
	for seed := uint64(0); seed < frameSeeds && !random; seed++ {
		_, err := Run(s, RandomFrame("random", rand.New(rand.NewPCG(seed, 11)), FrameRandomOps(true), frameCalls))
		random = err != nil
	}
	return handwritten, random
}

func TestModelPassesFrameCases(t *testing.T) {
	s := frameSuite("model", nil)
	for _, c := range FrameCases() {
		if skipped, err := Run(s, c); err != nil || skipped {
			t.Errorf("%s: skipped %v, %v", c.Func.Name, skipped, err)
		}
	}
	for seed := uint64(0); seed < frameSeeds; seed++ {
		if _, err := Run(s, RandomFrame("random", rand.New(rand.NewPCG(seed, 11)), FrameRandomOps(true), frameCalls)); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRandomFrameCasesUseTheOperations checks that randomized cases exercise every operation.
func TestRandomFrameCasesUseTheOperations(t *testing.T) {
	counts := map[string]int{}
	for seed := uint64(0); seed < frameSeeds; seed++ {
		c := RandomFrame("random", rand.New(rand.NewPCG(seed, 11)), FrameRandomOps(true), frameCalls)
		walkCalls(c.Func.Body, func(x *exec.Call) { counts[x.Callee]++ })
	}
	for _, name := range FrameOps {
		if counts[name] < 50 {
			t.Errorf("randomized cases call %s %d times", name, counts[name])
		}
	}
}

// TestFrameMutantsFail is the negative control of the cases of the capture frames: each broken model
// fails a handwritten and a randomized case.
func TestFrameMutantsFail(t *testing.T) {
	for _, br := range FrameBrokenOps(true) {
		t.Run(br.Name, func(t *testing.T) {
			mu := mutant{op: br.Op, edit: func(m *ref.Model, a []any) []any {
				return br.Edit(func(op string, args ...any) []any { return m.Call(frameModule.Op(op), args) }, a)
			}}
			handwritten, random := frameFails(frameSuite(br.Name, &mu))
			if !handwritten {
				t.Error("no handwritten case detected the mutant")
			}
			if !random {
				t.Error("no randomized case detected the mutant")
			}
		})
	}
}
