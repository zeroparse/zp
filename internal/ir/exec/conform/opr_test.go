package conform

import (
	"math/rand/v2"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

func (m modelImpl) Handle(name string) any { return m.Model.OprHandle(name) }

var oprModule = OprModule()

func oprCall(m *ref.Model, name string, args ...any) []any { return m.Call(oprModule.Op(name), args) }

func oprSuite(name string, mu *mutant) Suite {
	return Suite{Name: name, Ops: OprSuiteOps(), New: func(c ref.Config) Impl {
		if mu == nil {
			return modelImpl{ref.New(c)}
		}
		mu.modelImpl = modelImpl{ref.New(c)}
		return *mu
	}}
}

func TestOprCasesVerify(t *testing.T) {
	for _, c := range OprCases() {
		if err := exec.Verify(c.Module); err != nil {
			t.Fatal(err)
		}
		break
	}
	r := RandomOpr("random", rand.New(rand.NewPCG(1, 2)), OprOps, 200, OprConfig())
	if err := exec.Verify(r.Module); err != nil {
		t.Fatal(err)
	}
}

const oprSeeds, oprCalls = 200, 250

// oprFails reports whether some handwritten or randomized case of the Operators Rule family fails on s.
func oprFails(s Suite) (handwritten, random bool) {
	for _, c := range OprCases() {
		if _, err := Run(s, c); err != nil {
			handwritten = true
		}
	}
	for seed := uint64(0); seed < oprSeeds && !random; seed++ {
		c := RandomOpr("random", rand.New(rand.NewPCG(seed, 7)), OprOps, oprCalls, OprConfig())
		_, err := Run(s, c)
		random = err != nil
	}
	return handwritten, random
}

func TestModelPassesOprCases(t *testing.T) {
	s := oprSuite("model", nil)
	for _, c := range OprCases() {
		if skipped, err := Run(s, c); err != nil || skipped {
			t.Errorf("%s: skipped %v, %v", c.Func.Name, skipped, err)
		}
	}
	for seed := uint64(0); seed < oprSeeds; seed++ {
		c := RandomOpr("random", rand.New(rand.NewPCG(seed, 7)), OprOps, oprCalls, OprConfig())
		if _, err := Run(s, c); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRandomOprCasesUseTheOperations checks that randomized cases exercise every operation.
func TestRandomOprCasesUseTheOperations(t *testing.T) {
	counts := map[string]int{}
	for seed := uint64(0); seed < oprSeeds; seed++ {
		c := RandomOpr("random", rand.New(rand.NewPCG(seed, 7)), OprOps, oprCalls, OprConfig())
		walkCalls(c.Func.Body, func(x *exec.Call) { counts[x.Callee]++ })
	}
	for _, name := range OprOps {
		if counts[name] < 20 {
			t.Errorf("randomized cases call %s %d times", name, counts[name])
		}
	}
}

// TestOprMutantsFail is the negative control of the cases of the Operators Rule family: each broken
// model fails a handwritten or a randomized case.
func TestOprMutantsFail(t *testing.T) {
	for _, br := range OprBrokenOps() {
		t.Run(br.Name, func(t *testing.T) {
			mu := mutant{op: br.Op, edit: func(m *ref.Model, a []any) []any {
				return br.Edit(func(op string, args ...any) []any { return oprCall(m, op, args...) }, a)
			}}
			handwritten, random := oprFails(oprSuite(br.Name, &mu))
			if !handwritten && !random {
				t.Error("no case detected the mutant")
			}
			if !random {
				t.Error("no randomized case detected the mutant")
			}
		})
	}
}
