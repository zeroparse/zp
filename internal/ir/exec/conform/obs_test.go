package conform

import (
	"math/rand/v2"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

var obsModule = ObsModule()

func obsCall(m *ref.Model, name string, args ...any) []any { return m.Call(obsModule.Op(name), args) }

// obsMutants are broken models of the observation, the negative control of its cases.
func obsMutants() map[string]mutant {
	return map[string]mutant{
		"memoHit reports nothing": {op: "obs.memoHit", edit: func(m *ref.Model, a []any) []any { return nil }},
		"enter reports a later position": {op: "obs.enter", edit: func(m *ref.Model, a []any) []any {
			return obsCall(m, "obs.enter", a[0], a[1].(int64)+1, a[2])
		}},
		"enter drops the level": {op: "obs.enter", edit: func(m *ref.Model, a []any) []any {
			return obsCall(m, "obs.enter", a[0], a[1], int64(0))
		}},
		"exit reports a failure for a match": {op: "obs.exit", edit: func(m *ref.Model, a []any) []any {
			return obsCall(m, "obs.exit", !a[0].(bool), a[1], a[2], a[3])
		}},
		"exit forgets how far the call examined": {op: "obs.exit", edit: func(m *ref.Model, a []any) []any {
			return obsCall(m, "obs.exit", a[0], a[1], a[2], false)
		}},
		"exit reports the start of the call": {op: "obs.exit", edit: func(m *ref.Model, a []any) []any {
			return obsCall(m, "obs.exit", a[0], int64(0), a[2], a[3])
		}},
		"active is always true":  {op: "obs.active", edit: func(m *ref.Model, a []any) []any { return []any{true} }},
		"active is always false": {op: "obs.active", edit: func(m *ref.Model, a []any) []any { return []any{false} }},
	}
}

// obsFails reports whether some handwritten or randomized case of the observation fails on s.
func obsFails(s Suite) (handwritten, random bool) {
	for _, c := range ObsCases() {
		if _, err := Run(s, c); err != nil {
			handwritten = true
		}
	}
	for seed := uint64(0); seed < 200 && !random; seed++ {
		c := RandomObs("random", rand.New(rand.NewPCG(seed, 7)), 100, seed%4 != 3)
		_, err := Run(s, c)
		random = err != nil
	}
	return handwritten, random
}

func TestObsCasesVerify(t *testing.T) {
	if err := exec.Verify(ObsCases()[0].Module); err != nil {
		t.Fatal(err)
	}
	r := RandomObs("random", rand.New(rand.NewPCG(1, 2)), 200, true)
	if err := exec.Verify(r.Module); err != nil {
		t.Fatal(err)
	}
}

// TestModelPassesObsCases checks the cases of the observation against the contract as the model states
// it.
func TestModelPassesObsCases(t *testing.T) {
	s := Suite{Name: "model", Ops: ObsOps, New: func(c ref.Config) Impl { return modelImpl{ref.New(c)} }}
	for _, c := range ObsCases() {
		if skipped, err := Run(s, c); err != nil || skipped {
			t.Errorf("%s: skipped %v, %v", c.Func.Name, skipped, err)
		}
	}
	for seed := uint64(0); seed < 200; seed++ {
		c := RandomObs("random", rand.New(rand.NewPCG(seed, 7)), 100, seed%4 != 3)
		if _, err := Run(s, c); err != nil {
			t.Fatal(err)
		}
	}
}

// TestObsMutantsFail is the negative control of the cases of the observation: each broken model fails
// a handwritten case and a randomized one.
func TestObsMutantsFail(t *testing.T) {
	for name, mu := range obsMutants() {
		t.Run(name, func(t *testing.T) {
			s := Suite{Name: name, Ops: ObsOps, New: func(c ref.Config) Impl {
				mu.modelImpl = modelImpl{ref.New(c)}
				return mu
			}}
			handwritten, random := obsFails(s)
			if !handwritten {
				t.Error("no handwritten case detected the mutant")
			}
			if !random {
				t.Error("no randomized case detected the mutant")
			}
		})
	}
}

// TestObsPrecondition checks that the model refuses what the contract does not allow: reporting a
// call to a parse that is not observed, and the end of a call that has not begun.
func TestObsPrecondition(t *testing.T) {
	enter, exit := obsModule.Op("obs.enter"), obsModule.Op("obs.exit")
	args := []any{ref.RuleRef{}, int64(0), int64(0)}
	exitArgs := []any{true, int64(0), int64(0), false}
	if m := ref.New(ObsConfig(false)); m.Allows(enter, args) || m.Allows(exit, exitArgs) {
		t.Error("the model allows reporting to a parse that is not observed")
	}
	m := ref.New(ObsConfig(true))
	if !m.Allows(enter, args) || m.Allows(exit, exitArgs) {
		t.Error("the model must allow a call to begin, and not to end before")
	}
	m.Call(enter, args)
	if !m.Allows(exit, exitArgs) {
		t.Error("the model refuses the end of a call that began")
	}
	m.Call(exit, exitArgs)
	if m.Allows(exit, exitArgs) {
		t.Error("the model allows the end of a call that ended")
	}
}
