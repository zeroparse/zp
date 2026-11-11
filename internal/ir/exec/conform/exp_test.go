package conform

import (
	"math/rand/v2"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

func (m modelImpl) Exp(id int64) any { return ref.ExpID{ID: id} }

func (m modelImpl) Placeholder() any { return ref.Placeholder }

// expConfig returns the configuration of the randomized case with the seed: every third parse
// records no expectations.
func expConfig(seed uint64) ref.Config { return ref.Config{NoExpectations: seed%3 == 2} }

func TestExpCasesVerify(t *testing.T) {
	if err := exec.Verify(ExpCases()[0].Module); err != nil {
		t.Fatal(err)
	}
	r := RandomExp("random", rand.New(rand.NewPCG(1, 2)), RecordOps, 200, ref.Config{})
	if err := exec.Verify(r.Module); err != nil {
		t.Fatal(err)
	}
}

// TestModelPassesExpCases checks the cases of the expectation record against the contract as the
// model states it.
func TestModelPassesExpCases(t *testing.T) {
	s := Suite{Name: "model", Ops: ExpOps, New: func(c ref.Config) Impl { return modelImpl{ref.New(c)} }}
	for _, c := range ExpCases() {
		if skipped, err := Run(s, c); err != nil || skipped {
			t.Errorf("%s: skipped %v, %v", c.Func.Name, skipped, err)
		}
	}
	for seed := uint64(0); seed < 200; seed++ {
		c := RandomExp("random", rand.New(rand.NewPCG(seed, 7)), RecordOps, 100, expConfig(seed))
		if _, err := Run(s, c); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRandomExpCasesUseTheRecord checks that randomized cases exercise what they are for: sets read
// after a merge into the restored record, copies and nested isolations.
func TestRandomExpCasesUseTheRecord(t *testing.T) {
	counts := map[string]int{}
	for seed := uint64(0); seed < 200; seed++ {
		c := RandomExp("random", rand.New(rand.NewPCG(seed, 7)), RecordOps, 100, expConfig(seed))
		walkCalls(c.Func.Body, func(x *exec.Call) { counts[x.Callee]++ })
	}
	for _, name := range RecordOps {
		if counts[name] < 50 {
			t.Errorf("randomized cases call %s %d times", name, counts[name])
		}
	}
}

var expModule = ExpModule()

func expCall(m *ref.Model, name string, args ...any) []any { return m.Call(expModule.Op(name), args) }

// expMutants are broken models of the expectation record, the negative control of its cases.
func expMutants() map[string]mutant {
	return map[string]mutant{
		"add leaves out an entry that is in the record": {op: "exp.add", edit: func(m *ref.Model, a []any) []any {
			if _, ids := m.ExpRecord(); !contains(ids, a[0].(ref.ExpID)) {
				return expCall(m, "exp.add", a...)
			}
			return nil
		}},
		"at reads from the end": {op: "exp.at", edit: func(m *ref.Model, a []any) []any {
			_, ids := m.ExpRecord()
			return expCall(m, "exp.at", int64(len(ids))-1-a[0].(int64))
		}},
		"maybe answers false": {op: "exp.maybe", edit: func(m *ref.Model, a []any) []any { return []any{false} }},
		"reset keeps the entries": {op: "exp.reset", edit: func(m *ref.Model, a []any) []any {
			_, ids := m.ExpRecord()
			expCall(m, "exp.reset", a...)
			for _, id := range ids {
				expCall(m, "exp.add", id)
			}
			return nil
		}},
		"reset keeps a farther position": {op: "exp.reset", edit: func(m *ref.Model, a []any) []any {
			far, _ := m.ExpRecord()
			return expCall(m, "exp.reset", max(far, a[0].(int64)))
		}},
		"recording is always on": {op: "exp.recording", edit: func(m *ref.Model, a []any) []any { return []any{true} }},
		"fresh leaves out the placeholder": {op: "exp.fresh", edit: func(m *ref.Model, a []any) []any {
			_, ids := m.ExpRecord()
			for _, id := range ids {
				if id != ref.Placeholder {
					return []any{false}
				}
			}
			return []any{true}
		}},
		"isolate keeps the farthest position": {op: "exp.isolate", edit: func(m *ref.Model, a []any) []any {
			far, _ := m.ExpRecord()
			return expCall(m, "exp.isolate", far)
		}},
		"unisolate returns the restored farthest position": {op: "exp.unisolate", edit: func(m *ref.Model, a []any) []any {
			r := expCall(m, "exp.unisolate", a...)
			far, _ := m.ExpRecord()
			return []any{far, r[1]}
		}},
		"unisolate keeps a farther position": {op: "exp.unisolate", edit: func(m *ref.Model, a []any) []any {
			r := expCall(m, "exp.unisolate", a...)
			if far, _ := m.ExpRecord(); r[0].(int64) > far {
				expCall(m, "exp.reset", r[0])
			}
			return r
		}},
		"setAt reads from the end": {op: "exp.setAt", edit: func(m *ref.Model, a []any) []any {
			n := expCall(m, "exp.setCount", a[0])[0].(int64)
			return expCall(m, "exp.setAt", a[0], n-1-a[1].(int64))
		}},
		"copy returns the set kept last when it is equal": {op: "exp.copy", edit: func(m *ref.Model, a []any) []any {
			last := expCall(m, "exp.last")[0]
			n := expCall(m, "exp.keptCount", last)[0].(int64)
			if n == expCall(m, "exp.setCount", a[0])[0].(int64) {
				same := true
				for i := range n {
					same = same && expCall(m, "exp.keptAt", last, i)[0] == expCall(m, "exp.setAt", a[0], i)[0]
				}
				if same {
					return []any{last}
				}
			}
			return expCall(m, "exp.copy", a...)
		}},
		"last is always the empty set": {op: "exp.last", edit: func(m *ref.Model, a []any) []any { return expCall(m, "exp.empty") }},
		"keptAt reads the set kept last": {op: "exp.keptAt", edit: func(m *ref.Model, a []any) []any {
			last := expCall(m, "exp.last")[0]
			if a[1].(int64) < expCall(m, "exp.keptCount", last)[0].(int64) {
				return expCall(m, "exp.keptAt", last, a[1])
			}
			return expCall(m, "exp.keptAt", a...)
		}},
		"setExpected keeps the first record": {op: "memo.setExpected", edit: func(m *ref.Model, a []any) []any {
			if !m.Allows(expModule.Op("memo.far"), a[:1]) {
				return expCall(m, "memo.setExpected", a...)
			}
			return nil
		}},
	}
}

func contains(ids []ref.ExpID, id ref.ExpID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// expFails reports whether some handwritten or randomized case of the expectation record fails on s,
// and which kind of case found it first.
func expFails(s Suite) (handwritten, random bool) {
	for _, c := range ExpCases() {
		if _, err := Run(s, c); err != nil {
			handwritten = true
		}
	}
	for seed := uint64(0); seed < 200 && !random; seed++ {
		c := RandomExp("random", rand.New(rand.NewPCG(seed, 7)), RecordOps, 100, expConfig(seed))
		_, err := Run(s, c)
		random = err != nil
	}
	return handwritten, random
}

// TestExpMutantsFail is the negative control of the cases of the expectation record: each broken
// model fails a handwritten case, and each that breaks an operation of the record alone fails a
// randomized case too.
func TestExpMutantsFail(t *testing.T) {
	for name, mu := range expMutants() {
		t.Run(name, func(t *testing.T) {
			s := Suite{Name: name, Ops: ExpOps, New: func(c ref.Config) Impl {
				mu.modelImpl = modelImpl{ref.New(c)}
				return mu
			}}
			handwritten, random := expFails(s)
			if !handwritten {
				t.Error("no handwritten case detected the mutant")
			}
			if !random && mu.op[:4] == "exp." {
				t.Error("no randomized case detected the mutant")
			}
		})
	}
}

// TestExpPrecondition checks that the model refuses what the contract does not allow: reading a set
// after a write at or past the index of its entry, copying it then, and ending an isolation that is
// not the innermost one.
func TestExpPrecondition(t *testing.T) {
	m := ref.New(ref.Config{})
	op := expModule.Op
	expCall(m, "exp.add", ref.ExpID{ID: 1})
	outer := expCall(m, "exp.isolate", int64(0))[0]
	inner := expCall(m, "exp.isolate", int64(0))[0]
	if m.Allows(op("exp.unisolate"), []any{outer}) {
		t.Error("the model allows ending an isolation that is not the innermost one")
	}
	expCall(m, "exp.unisolate", inner)
	expCall(m, "exp.add", ref.ExpID{ID: 2})
	expCall(m, "exp.add", ref.ExpID{ID: 3})
	s := expCall(m, "exp.unisolate", outer)[1]
	// The restored record has one entry; the set's entries follow it where a target keeps them.
	if !m.Allows(op("exp.setAt"), []any{s, int64(0)}) || !m.Allows(op("exp.copy"), []any{s}) {
		t.Fatal("the model refuses reading a set before any write")
	}
	expCall(m, "exp.add", expCall(m, "exp.setAt", s, int64(0))[0])
	if m.Allows(op("exp.copy"), []any{s}) || !m.Allows(op("exp.setAt"), []any{s, int64(1)}) {
		t.Error("after a write at index 1, only the entry at index 1 of the set may be read")
	}
	expCall(m, "exp.add", ref.ExpID{ID: 4})
	if m.Allows(op("exp.setAt"), []any{s, int64(1)}) {
		t.Error("the model allows reading an entry of the set after a write past its index")
	}
	expCall(m, "exp.reset", int64(9))
	expCall(m, "exp.isolate", int64(9))
	if m.Allows(op("exp.setAt"), []any{s, int64(1)}) || m.Allows(op("exp.copy"), []any{s}) {
		t.Error("the model allows reading a set after an isolation began")
	}
}

// TestExpMaybeMayAnswerTrue checks the other side of the filter: an implementation whose exp.maybe
// always answers true meets the contract, and the cases go on with its answer.
func TestExpMaybeMayAnswerTrue(t *testing.T) {
	mu := mutant{op: "exp.maybe", edit: func(*ref.Model, []any) []any { return []any{true} }}
	s := Suite{Name: "maybe answers true", Ops: ExpOps, New: func(c ref.Config) Impl {
		mu.modelImpl = modelImpl{ref.New(c)}
		return mu
	}}
	if handwritten, random := expFails(s); handwritten || random {
		t.Fatalf("a filter that answers true fails: handwritten %v, random %v", handwritten, random)
	}
}
