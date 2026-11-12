package conform

import (
	"math/rand/v2"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

var missingModule = MissingModule()

func missingCall(m *ref.Model, name string, args ...any) []any {
	return m.Call(missingModule.Op(name), args)
}

// missingMutants are broken models of the missing-token mode, the negative control of its cases.
func missingMutants() map[string]mutant {
	return map[string]mutant{
		"active is always false": {op: "missing.active", edit: func(m *ref.Model, a []any) []any { return []any{false} }},
		"recompute ignores the rule": {op: "missing.recompute", edit: func(m *ref.Model, a []any) []any {
			return missingCall(m, "missing.active")
		}},
		"recompute is always false": {op: "missing.recompute", edit: func(m *ref.Model, a []any) []any { return []any{false} }},
		"mark counts one token less": {op: "missing.mark", edit: func(m *ref.Model, a []any) []any {
			return []any{max(missingCall(m, "missing.mark")[0].(int64)-1, 0)}
		}},
		"reset keeps the tokens": {op: "missing.reset", edit: func(m *ref.Model, a []any) []any { return nil }},
		"reset removes one token more": {op: "missing.reset", edit: func(m *ref.Model, a []any) []any {
			return missingCall(m, "missing.reset", max(a[0].(int64)-1, 0))
		}},
		"insert answers that a repeated token is new": {op: MissingInsertOp, edit: func(m *ref.Model, a []any) []any {
			missingCall(m, MissingInsertOp, a...)
			return []any{true}
		}},
		"replay inserts nothing": {op: "missing.replay", edit: func(m *ref.Model, a []any) []any { return nil }},
		"replay forgets the errors": {op: "missing.replay", edit: func(m *ref.Model, a []any) []any {
			n := m.Entries()
			missingCall(m, "missing.replay", a...)
			return missingCall(m, "rec.truncate", n)
		}},
	}
}

func missingRandom(seed uint64) Case {
	return RandomMissing("random", rand.New(rand.NewPCG(seed, 7)), 100, seed%4 != 3)
}

// missingFails reports whether some handwritten or randomized case of the missing-token mode fails on s.
func missingFails(s Suite) (handwritten, random bool) {
	for _, c := range MissingCases() {
		if _, err := Run(s, c); err != nil {
			handwritten = true
		}
	}
	for seed := uint64(0); seed < 200 && !random; seed++ {
		_, err := Run(s, missingRandom(seed))
		random = err != nil
	}
	return handwritten, random
}

func TestMissingCasesVerify(t *testing.T) {
	if err := exec.Verify(MissingCases()[0].Module); err != nil {
		t.Fatal(err)
	}
	if err := exec.Verify(missingRandom(1).Module); err != nil {
		t.Fatal(err)
	}
}

// TestModelPassesMissingCases checks the cases of the missing-token mode against the contract as the
// model states it.
func TestModelPassesMissingCases(t *testing.T) {
	s := Suite{Name: "model", Ops: MissingOps, New: func(c ref.Config) Impl { return modelImpl{ref.New(c)} }}
	for _, c := range MissingCases() {
		if skipped, err := Run(s, c); err != nil || skipped {
			t.Errorf("%s: skipped %v, %v", c.Func.Name, skipped, err)
		}
	}
	for seed := uint64(0); seed < 200; seed++ {
		if _, err := Run(s, missingRandom(seed)); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRandomMissingCasesUseTheMode checks that randomized cases exercise what they are for.
func TestRandomMissingCasesUseTheMode(t *testing.T) {
	counts := map[string]int{}
	for seed := uint64(0); seed < 200; seed++ {
		walkCalls(missingRandom(seed).Func.Body, func(x *exec.Call) { counts[x.Callee]++ })
	}
	for _, name := range MissingOps {
		// Cases read the length of the record from the model, not from rec.mark.
		if name != "rec.mark" && counts[name] < 50 {
			t.Errorf("randomized cases call %s %d times", name, counts[name])
		}
	}
}

// TestMissingMutantsFail is the negative control of the cases of the missing-token mode: each broken
// model fails a handwritten case and a randomized one.
func TestMissingMutantsFail(t *testing.T) {
	for name, mu := range missingMutants() {
		t.Run(name, func(t *testing.T) {
			s := Suite{Name: name, Ops: MissingOps, New: func(c ref.Config) Impl {
				mu.modelImpl = modelImpl{ref.New(c)}
				return mu
			}}
			handwritten, random := missingFails(s)
			if !handwritten {
				t.Error("no handwritten case detected the mutant")
			}
			if !random {
				t.Error("no randomized case detected the mutant")
			}
		})
	}
}

// TestMissingPrecondition checks that the model refuses what the contract does not allow: replaying
// outside the mode, and resetting to a mark beyond the tokens.
func TestMissingPrecondition(t *testing.T) {
	reset := missingModule.Op("missing.reset")
	m := ref.New(MissingConfig(true))
	if !m.Allows(reset, []any{int64(0)}) || m.Allows(reset, []any{int64(1)}) {
		t.Error("the model allows a mark beyond the tokens or refuses one that holds")
	}
	missingCall(m, MissingInsertOp, int64(0), int64(0), &ref.Err{ID: 1})
	if !m.Allows(reset, []any{int64(1)}) {
		t.Error("the model refuses the mark of a token that was inserted")
	}
	if ref.New(MissingConfig(false)).Allows(missingModule.Op(MissingInsertOp), []any{int64(0), int64(0), &ref.Err{}}) {
		t.Error("the model allows inserting a token outside the mode")
	}
}
