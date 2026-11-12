package conform

import (
	"math/rand/v2"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// docConfig returns the configuration of the randomized case with the seed: every third parse reads no
// Document, and every other Document parse starts the examined range of a call as the empty range.
func docConfig(seed uint64) ref.Config {
	return ref.Config{Document: seed%3 != 2, DocEmptyEnter: seed%2 == 1}
}

// docSuite is a suite of the doc operations over the model with mu, and where its doc.enter starts
// the range of a call as the model with emptyEnter does.
func docSuite(name string, emptyEnter bool, mu *mutant) Suite {
	return Suite{Name: name, Ops: DocOps, DocEmptyEnter: emptyEnter, New: func(c ref.Config) Impl {
		if mu == nil {
			return modelImpl{ref.New(c)}
		}
		mu.modelImpl = modelImpl{ref.New(c)}
		return *mu
	}}
}

func TestDocCasesVerify(t *testing.T) {
	if err := exec.Verify(DocCases()[0].Module); err != nil {
		t.Fatal(err)
	}
	r := RandomDoc("random", rand.New(rand.NewPCG(1, 2)), DocOps, 200, ref.Config{Document: true})
	if err := exec.Verify(r.Module); err != nil {
		t.Fatal(err)
	}
}

// docFails reports whether some handwritten or randomized case of the doc operations fails on s.
func docFails(s Suite) (handwritten, random bool) {
	for _, c := range DocCases() {
		if _, err := Run(s, c); err != nil {
			handwritten = true
		}
	}
	for seed := uint64(0); seed < 200 && !random; seed++ {
		c := RandomDoc("random", rand.New(rand.NewPCG(seed, 7)), DocOps, 100, docConfig(seed))
		_, err := Run(s, c)
		random = err != nil
	}
	return handwritten, random
}

// TestModelPassesDocCases checks the cases of the doc operations against the contract as the model
// states it, with either start of the range of a call.
func TestModelPassesDocCases(t *testing.T) {
	for _, empty := range []bool{false, true} {
		s := docSuite("model", empty, nil)
		for _, c := range DocCases() {
			if skipped, err := Run(s, c); err != nil || skipped {
				t.Errorf("%s: skipped %v, %v", c.Func.Name, skipped, err)
			}
		}
		for seed := uint64(0); seed < 200; seed++ {
			c := RandomDoc("random", rand.New(rand.NewPCG(seed, 7)), DocOps, 100, docConfig(seed))
			if _, err := Run(s, c); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// TestRandomDocCasesUseTheOperations checks that randomized cases exercise every operation.
func TestRandomDocCasesUseTheOperations(t *testing.T) {
	counts := map[string]int{}
	for seed := uint64(0); seed < 200; seed++ {
		c := RandomDoc("random", rand.New(rand.NewPCG(seed, 7)), DocOps, 100, docConfig(seed))
		walkCalls(c.Func.Body, func(x *exec.Call) { counts[x.Callee]++ })
	}
	for _, name := range DocOps {
		if counts[name] < 50 {
			t.Errorf("randomized cases call %s %d times", name, counts[name])
		}
	}
}

var docModule = DocModule()

func docCall(m *ref.Model, name string, args ...any) []any { return m.Call(docModule.Op(name), args) }

// docMutants are broken models of the doc operations, the negative control of their cases.
func docMutants() map[string]mutant {
	return map[string]mutant{
		"active is always true": {op: "doc.active", edit: func(m *ref.Model, a []any) []any { return []any{true} }},
		"enter keeps the range of the enclosing call": {op: "doc.enter", edit: func(m *ref.Model, a []any) []any {
			low, high := m.DocRange()
			r := docCall(m, "doc.enter", a...)
			docCall(m, "doc.touchFrom", low)
			docCall(m, "doc.touch", high)
			return r
		}},
		"enter returns the start first": {op: "doc.enter", edit: func(m *ref.Model, a []any) []any {
			r := docCall(m, "doc.enter", a...)
			return []any{r[1], r[0]}
		}},
		"leave ignores the nested frontier": {op: "doc.leave", edit: func(m *ref.Model, a []any) []any {
			return docCall(m, "doc.leave", int64(0), a[1])
		}},
		"leave ignores the nested start": {op: "doc.leave", edit: func(m *ref.Model, a []any) []any {
			return docCall(m, "doc.leave", a[0], int64(1<<40))
		}},
		"touch stops one short": {op: "doc.touch", edit: func(m *ref.Model, a []any) []any {
			return docCall(m, "doc.touch", a[0].(int64)-1)
		}},
		"touch ignores the frontier":  {op: "doc.touch", edit: func(m *ref.Model, a []any) []any { return nil }},
		"touchFrom ignores the start": {op: "doc.touchFrom", edit: func(m *ref.Model, a []any) []any { return nil }},
		"touchFrom stops one short": {op: "doc.touchFrom", edit: func(m *ref.Model, a []any) []any {
			return docCall(m, "doc.touchFrom", a[0].(int64)+1)
		}},
		"low returns the frontier": {op: "doc.low", edit: func(m *ref.Model, a []any) []any { return docCall(m, "doc.high") }},
		"high returns one more": {op: "doc.high", edit: func(m *ref.Model, a []any) []any {
			return []any{docCall(m, "doc.high")[0].(int64) + 1}
		}},
		"reused counts evaluated":  {op: "doc.reused", edit: func(m *ref.Model, a []any) []any { return docCall(m, "doc.evaluated") }},
		"evaluated counts nothing": {op: "doc.evaluated", edit: func(m *ref.Model, a []any) []any { return nil }},
		"examinedBegin leaves the frontier": {op: "doc.examinedBegin", edit: func(m *ref.Model, a []any) []any {
			_, high := m.DocRange()
			r := docCall(m, "doc.examinedBegin")
			docCall(m, "doc.touch", high)
			return r
		}},
		"examinedEnd drops the enclosing frontier": {op: "doc.examinedEnd", edit: func(m *ref.Model, a []any) []any {
			return docCall(m, "doc.examinedEnd", int64(0))
		}},
		"examinedEnd is unknown": {op: "doc.examinedEnd", edit: func(m *ref.Model, a []any) []any {
			r := docCall(m, "doc.examinedEnd", a...)
			return []any{r[0], false}
		}},
	}
}

// TestDocMutantsFail is the negative control of the cases of the doc operations: each broken model
// fails a handwritten or a randomized case.
func TestDocMutantsFail(t *testing.T) {
	for name, mu := range docMutants() {
		t.Run(name, func(t *testing.T) {
			for _, empty := range []bool{false, true} {
				handwritten, random := docFails(docSuite(name, empty, &mu))
				if !handwritten {
					t.Errorf("no handwritten case detected the mutant (empty enter %v)", empty)
				}
				if !random {
					t.Errorf("no randomized case detected the mutant (empty enter %v)", empty)
				}
			}
		})
	}
}

// TestDocEnterStartsTheRange checks that the cases tell the two starts of a range apart from a
// broken one: an implementation that starts the range of a call as the model with the other start
// does is accepted only where it is configured as such.
func TestDocEnterStartsTheRange(t *testing.T) {
	mu := mutant{op: "doc.enter", edit: func(m *ref.Model, a []any) []any {
		r := docCall(m, "doc.enter", a...)
		docCall(m, "doc.touch", a[0].(int64)+2)
		return r
	}}
	if handwritten, random := docFails(docSuite("range of two units", false, &mu)); !handwritten || !random {
		t.Errorf("a range of two units passes: handwritten %v, random %v", handwritten, random)
	}
}
