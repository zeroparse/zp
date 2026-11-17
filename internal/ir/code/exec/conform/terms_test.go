package conform

import (
	"math/rand/v2"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/exec/ref"
)

var termStateModule = TermStateModule()

// termStateFamily is the cases of the elements gathered or of the values made.
type termStateFamily struct {
	name   string
	ops    []string
	cases  func() []Case
	random func(name string, rng *rand.Rand, n int) Case
	broken func() []TermStateBroken
}

var termStateFamilies = []termStateFamily{
	{"gathered", GatheredOps, GatheredCases, RandomGathered, GatheredBrokenOps},
	{"made", MadeOps, MadeCases, RandomMade, MadeBrokenOps},
}

const termStateSeeds, termStateCalls = 200, 120

func termStateSuite(fam termStateFamily, mu *mutant) Suite {
	return Suite{Name: fam.name, Ops: fam.ops, New: func(c ref.Config) Impl {
		if mu == nil {
			return modelImpl{ref.New(c)}
		}
		mu.modelImpl = modelImpl{ref.New(c)}
		return *mu
	}}
}

func termStateFails(fam termStateFamily, s Suite) (handwritten, random bool) {
	for _, c := range fam.cases() {
		if _, err := Run(s, c); err != nil {
			handwritten = true
		}
	}
	for seed := uint64(0); seed < termStateSeeds && !random; seed++ {
		_, err := Run(s, fam.random("random", rand.New(rand.NewPCG(seed, 13)), termStateCalls))
		random = err != nil
	}
	return handwritten, random
}

func TestTermStateCasesVerify(t *testing.T) {
	for _, fam := range termStateFamilies {
		if err := code.Verify(fam.cases()[0].Module); err != nil {
			t.Fatal(fam.name, err)
		}
		if err := code.Verify(fam.random("random", rand.New(rand.NewPCG(1, 2)), 200).Module); err != nil {
			t.Fatal(fam.name, err)
		}
	}
}

func TestModelPassesTermStateCases(t *testing.T) {
	for _, fam := range termStateFamilies {
		s := termStateSuite(fam, nil)
		for _, c := range fam.cases() {
			if skipped, err := Run(s, c); err != nil || skipped {
				t.Errorf("%s: skipped %v, %v", c.Func.Name, skipped, err)
			}
		}
		for seed := uint64(0); seed < termStateSeeds; seed++ {
			if _, err := Run(s, fam.random("random", rand.New(rand.NewPCG(seed, 13)), termStateCalls)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// TestRandomTermStateCasesUseTheOperations checks that randomized cases exercise every operation.
func TestRandomTermStateCasesUseTheOperations(t *testing.T) {
	for _, fam := range termStateFamilies {
		counts := map[string]int{}
		for seed := uint64(0); seed < termStateSeeds; seed++ {
			c := fam.random("random", rand.New(rand.NewPCG(seed, 13)), termStateCalls)
			walkCalls(c.Func.Body, func(x *code.Call) { counts[x.Callee]++ })
		}
		for _, name := range fam.ops {
			if counts[name] < 50 {
				t.Errorf("randomized cases call %s %d times", name, counts[name])
			}
		}
	}
}

// TestTermStateMutantsFail is the negative control of the cases of the elements gathered and the values
// made: each broken model fails a handwritten and a randomized case.
func TestTermStateMutantsFail(t *testing.T) {
	for _, fam := range termStateFamilies {
		for _, br := range fam.broken() {
			t.Run(br.Name, func(t *testing.T) {
				mu := mutant{op: br.Op, edit: func(m *ref.Model, a []any) []any {
					return br.Edit(func(op string, args ...any) []any { return m.Call(termStateModule.Op(op), args) }, a)
				}}
				handwritten, random := termStateFails(fam, termStateSuite(fam, &mu))
				if !handwritten {
					t.Error("no handwritten case detected the mutant")
				}
				if !random {
					t.Error("no randomized case detected the mutant")
				}
			})
		}
	}
}
