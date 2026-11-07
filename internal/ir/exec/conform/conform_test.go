package conform

import (
	"errors"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

var allOps = []string{"memo.lookup", "memo.store", "memo.ok", "memo.end", "memo.silent", "memo.prune", "memo.floor", "memo.examine", "memo.examined", "memo.before", "memo.pin", "memo.splice", "seen.first",
	"budget.charge", "budget.used", "depth.enter", "depth.exit", "depth.current",
	"env.lookup", "env.bind", "env.scope", "env.restore", "env.key",
	"rec.mark", "rec.push", "rec.truncate", "rec.since", "rec.add", "rec.none", "stream.settleErrors"}

// modelImpl is the reference model run as an implementation: every case must pass on it.
type modelImpl struct{ *ref.Model }

func (m modelImpl) Values(id int64) any { return m.Model.Values(id) }

func (m modelImpl) Datum(id int64) any { return ref.Datum{ID: id} }

func (m modelImpl) Rule(index int64) any { return ref.RuleRef{Index: index} }

func (m modelImpl) Err(id int64) any { return &ref.Err{ID: id} }

// mutant breaks one operation of the model.
type mutant struct {
	modelImpl
	op   string
	edit func(m *ref.Model, args []any) []any
}

func (m mutant) Call(o *exec.Op, args []any) []any {
	if o.Name == m.op {
		return m.edit(m.Model, args)
	}
	return m.Model.Call(o, args)
}

func modelSuite() Suite {
	return Suite{Name: "model", Ops: allOps, New: func(c ref.Config) Impl { return modelImpl{ref.New(c)} }}
}

func TestCasesVerify(t *testing.T) {
	cs := Cases()
	if err := exec.Verify(cs[0].Module); err != nil {
		t.Fatal(err)
	}
	r := Random("random", rand.New(rand.NewPCG(1, 2)), allOps, 200, ref.Config{Rules: 3, Limit: 300, MaxDepth: 4})
	if err := exec.Verify(r.Module); err != nil {
		t.Fatal(err)
	}
}

// TestModelPassesItsCases checks the cases against the contract as the model states it.
func TestModelPassesItsCases(t *testing.T) {
	s := modelSuite()
	for _, c := range Cases() {
		if _, err := Run(s, c); err != nil {
			t.Error(err)
		}
	}
	for seed := uint64(0); seed < 200; seed++ {
		c := Random("random", rand.New(rand.NewPCG(seed, 7)), allOps, 100, ref.Config{Rules: 3, Limit: 400, MaxDepth: 5,
			Vars: RandomVars(int(seed)), RuleVars: RandomRuleVars})
		if _, err := Run(s, c); err != nil {
			t.Fatal(err)
		}
	}
}

// docEditOps are the operations of randomized cases that edit the memo of a Document: with only
// these, the entries that a case examines survive its edits and are found again, so that the
// randomized cases themselves check which entries an edit keeps, moves and removes.
var docEditOps = []string{"memo.lookup", "memo.store", "memo.end", "memo.examine", "memo.examined", "memo.before", "memo.pin", "memo.splice"}

// modelMutants are the broken models of the negative control of the harness.
func modelMutants() map[string]mutant {
	return map[string]mutant{
		"lookup ignores the level": {op: "memo.lookup", edit: func(m *ref.Model, a []any) []any {
			return m.Call(Module().Op("memo.lookup"), []any{a[0], a[1], int64(0), a[3]})
		}},
		"store keeps the old entry": {op: "memo.store", edit: func(m *ref.Model, a []any) []any {
			if r := m.Call(Module().Op("memo.lookup"), a[:4]); r[1].(bool) {
				return []any{r[0]}
			}
			return m.Call(Module().Op("memo.store"), a)
		}},
		"seen defers every call at position 100": {op: "seen.first", edit: func(m *ref.Model, a []any) []any {
			fresh := m.Call(Module().Op("seen.first"), a)
			return []any{fresh[0].(bool) || a[1].(int64) == 100}
		}},
		"store keeps the entry not silent": {op: "memo.store", edit: func(m *ref.Model, a []any) []any {
			a = append([]any(nil), a...)
			a[6] = false
			return m.Call(Module().Op("memo.store"), a)
		}},
		"prune removes the entries at its position": {op: "memo.prune", edit: func(m *ref.Model, a []any) []any {
			return m.Call(Module().Op("memo.prune"), []any{a[0].(int64) + 1})
		}},
		"splice keeps an entry that examined the edit's position": {op: "memo.splice", edit: func(m *ref.Model, a []any) []any {
			// The edit starts one unit later and ends where it did.
			start, end, delta := a[0].(int64)+1, a[1].(int64), a[2].(int64)
			if start > end || start > end+delta {
				return m.Call(Module().Op("memo.splice"), a)
			}
			return m.Call(Module().Op("memo.splice"), []any{start, end, delta})
		}},
		"splice moves an entry that examined the edit's end": {op: "memo.splice", edit: func(m *ref.Model, a []any) []any {
			// The edit ends one unit earlier and keeps its length.
			start, end, delta := a[0].(int64), a[1].(int64)-1, a[2].(int64)
			if start > end || start > end+delta {
				return m.Call(Module().Op("memo.splice"), a)
			}
			return m.Call(Module().Op("memo.splice"), []any{start, end, delta})
		}},
		"splice moves entries one unit short": {op: "memo.splice", edit: func(m *ref.Model, a []any) []any {
			start, end, delta := a[0].(int64), a[1].(int64), a[2].(int64)-1
			if start > end+delta {
				return m.Call(Module().Op("memo.splice"), a)
			}
			return m.Call(Module().Op("memo.splice"), []any{start, end, delta})
		}},
		"store drops the recovered errors, so that splice moves the entry": {op: "memo.store", edit: func(m *ref.Model, a []any) []any {
			a = slices.Clone(a)
			a[8] = nil
			return m.Call(Module().Op("memo.store"), a)
		}},
		"push drops the error":      {op: "rec.push", edit: func(m *ref.Model, a []any) []any { return nil }},
		"truncate keeps everything": {op: "rec.truncate", edit: func(m *ref.Model, a []any) []any { return nil }},
		"truncate drops one entry too many": {op: "rec.truncate", edit: func(m *ref.Model, a []any) []any {
			return m.Call(Module().Op("rec.truncate"), []any{max(a[0].(int64)-1, 0)})
		}},
		"since leaves out the first entry": {op: "rec.since", edit: func(m *ref.Model, a []any) []any {
			return m.Call(Module().Op("rec.since"), []any{min(a[0].(int64)+1, m.Entries())})
		}},
		"add records nothing":       {op: "rec.add", edit: func(m *ref.Model, a []any) []any { return nil }},
		"none is always true":       {op: "rec.none", edit: func(m *ref.Model, a []any) []any { return []any{true} }},
		"settling keeps the errors": {op: "stream.settleErrors", edit: func(m *ref.Model, a []any) []any { return nil }},
		"bind changes nothing":      {op: "env.bind", edit: func(m *ref.Model, a []any) []any { return nil }},
		"bind of an equal value makes a new environment": {op: "env.bind", edit: func(m *ref.Model, a []any) []any {
			op := Module().Op("env.bind")
			was := m.Call(Module().Op("env.lookup"), a[:1])[0]
			m.Call(op, []any{a[0], ref.Datum{ID: 999}})
			if was != a[1] {
				return m.Call(op, a)
			}
			return m.Call(op, []any{a[0], was})
		}},
		"restore keeps the current environment": {op: "env.restore", edit: func(m *ref.Model, a []any) []any { return nil }},
		"lookup reads the slot before": {op: "env.lookup", edit: func(m *ref.Model, a []any) []any {
			return m.Call(Module().Op("env.lookup"), []any{max(a[0].(int64)-1, 0)})
		}},
		"key leaves out the last variable": {op: "env.key", edit: func(m *ref.Model, a []any) []any {
			r := a[0].(ref.RuleRef)
			return m.Call(Module().Op("env.key"), []any{ref.RuleRef{Index: min(r.Index, 1)}})
		}},
		"budget charges one unit less": {op: "budget.charge", edit: func(m *ref.Model, a []any) []any {
			n := a[0].(uint64)
			if n > 0 {
				n--
			}
			return m.Call(Module().Op("budget.charge"), []any{n})
		}},
	}
}

// TestMutantsFail is the negative control of the harness: each broken implementation must fail.
func TestMutantsFail(t *testing.T) {
	for name, mu := range modelMutants() {
		t.Run(name, func(t *testing.T) {
			s := Suite{Name: name, Ops: allOps, New: func(c ref.Config) Impl {
				mu.modelImpl = modelImpl{ref.New(c)}
				return mu
			}}
			if !fails(s) {
				t.Fatal("no case detected the mutant")
			}
		})
	}
}

// TestRandomCasesFindEditMutants checks that randomized cases alone, without the handwritten ones,
// notice a broken edit of the memo: the entries that a splice moves, the entries with recovered
// errors that it must remove, and the edit it applies.
func TestRandomCasesFindEditMutants(t *testing.T) {
	for _, name := range []string{
		"splice keeps an entry that examined the edit's position",
		"splice moves an entry that examined the edit's end",
		"splice moves entries one unit short",
		"store drops the recovered errors, so that splice moves the entry",
	} {
		mu := modelMutants()[name]
		s := Suite{Name: name, Ops: docEditOps, New: func(c ref.Config) Impl {
			mu.modelImpl = modelImpl{ref.New(c)}
			return mu
		}}
		found := false
		for seed := uint64(0); seed < 300 && !found; seed++ {
			c := Random("random", rand.New(rand.NewPCG(seed, 7)), s.Ops, 100, ref.Config{Rules: 3, Limit: 400, MaxDepth: 5})
			_, err := Run(s, c)
			found = err != nil
		}
		if !found {
			t.Errorf("no randomized case detected the mutant %q", name)
		}
	}
}

// fails reports whether some handwritten or randomized case fails on s.
func fails(s Suite) bool {
	for _, c := range Cases() {
		if _, err := Run(s, c); err != nil {
			return true
		}
	}
	for seed := uint64(0); seed < 200; seed++ {
		c := Random("random", rand.New(rand.NewPCG(seed, 7)), s.Ops, 100, ref.Config{Rules: 3, Limit: 400, MaxDepth: 5})
		if _, err := Run(s, c); err != nil {
			return true
		}
	}
	return false
}

func TestFailureReport(t *testing.T) {
	s := Suite{Name: "broken", Ops: allOps, New: func(c ref.Config) Impl {
		return mutant{modelImpl: modelImpl{ref.New(c)}, op: "memo.end", edit: func(*ref.Model, []any) []any { return []any{int64(-1)} }}
	}}
	_, err := Run(s, Cases()[0])
	var f *Failure
	if !errors.As(err, &f) || !strings.Contains(err.Error(), "memo.end(#") || !strings.Contains(err.Error(), "func memo_store_lookup") {
		t.Fatalf("report = %v", err)
	}
}

func TestSkipsMissingOps(t *testing.T) {
	s := modelSuite()
	s.Ops = []string{"budget.charge", "budget.used"}
	skipped, err := Run(s, Cases()[0])
	if !skipped || err != nil {
		t.Fatalf("Run = %v, %v; want the memo case skipped", skipped, err)
	}
}

// firstFailure returns the first failure of s over the handwritten and randomized cases, or nil.
func firstFailure(s Suite) *Failure {
	var f *Failure
	for _, c := range Cases() {
		if _, err := Run(s, c); errors.As(err, &f) {
			return f
		}
	}
	for seed := uint64(0); seed < 200; seed++ {
		c := Random("random", rand.New(rand.NewPCG(seed, 7)), s.Ops, 100, ref.Config{Rules: 3, Limit: 400, MaxDepth: 5})
		if _, err := Run(s, c); errors.As(err, &f) {
			return f
		}
	}
	return nil
}

// memoKept is an implementation of the memo window that keeps entries the contract drops: the
// stores before the floor (below), or every store through a memo.prune (all). It also charges
// extra units in each memo.prune.
type memoKept struct {
	modelImpl
	below, all bool
	extra      uint64
	spent      uint64
	kept       map[[3]int64]any
}

func (m *memoKept) Used() uint64 { return m.spent }

func (m *memoKept) Call(o *exec.Op, a []any) []any {
	key := func() [3]int64 { return [3]int64{a[0].(int64), a[1].(int64), a[2].(int64)} }
	switch o.Name {
	case "memo.prune":
		m.spent += m.extra
	case "memo.store":
		below := a[1].(int64) < m.Model.Call(Module().Op("memo.floor"), nil)[0].(int64)
		r := m.Model.Call(o, a)
		if m.all || m.below && below {
			m.kept[key()] = r[0]
		}
		return r
	case "memo.lookup":
		r := m.Model.Call(o, a)
		if e, ok := m.kept[key()]; ok && !r[1].(bool) && a[1].(int64) < m.Model.Call(Module().Op("memo.floor"), nil)[0].(int64) {
			return []any{e, true}
		}
		return r
	}
	return m.Model.Call(o, a)
}

// TestWindowMutantsFail is the negative control of the memo window contract: a cost beyond the
// bound, a store below the floor that is kept, and a prune that frees nothing each fail, and for
// the reason of its own.
func TestWindowMutantsFail(t *testing.T) {
	for _, mu := range []struct {
		name string
		make func() *memoKept
		want string
	}{
		{"prune charges beyond its bound", func() *memoKept { return &memoKept{extra: 1 << 20} }, "more than the bound"},
		{"store below the floor is kept", func() *memoKept { return &memoKept{below: true} }, "result 1: model false, implementation true"},
		{"prune frees no entry", func() *memoKept { return &memoKept{all: true} }, "result 1: model false, implementation true"},
	} {
		t.Run(mu.name, func(t *testing.T) {
			s := Suite{Name: mu.name, Ops: allOps, New: func(c ref.Config) Impl {
				m := mu.make()
				m.modelImpl, m.kept = modelImpl{ref.New(c)}, map[[3]int64]any{}
				return m
			}}
			f := firstFailure(s)
			if f == nil {
				t.Fatal("no case detected the mutant")
			}
			t.Logf("failing case %s, call %d: %s", f.Case, f.Step, f.Msg)
			if !strings.Contains(f.Msg, mu.want) || mu.want != "more than the bound" && !strings.Contains(f.Trace[len(f.Trace)-1], "memo.lookup") {
				t.Fatalf("failed for another reason than %q:\n%v", mu.want, f)
			}
		})
	}
}

// errList is a list of recovered errors of listImpl: one error, or the entries that rec.since made a
// list of.
type errList struct {
	err     int64
	entries []*errList
	n       int
	wrap    bool // made by the mutant "wrap"
}

func (l *errList) ids(out []int64) []int64 {
	if l.entries == nil {
		return append(out, l.err)
	}
	for _, e := range l.entries {
		out = e.ids(out)
	}
	return out
}

func (l *errList) unrolled() int {
	n := 1
	for _, e := range l.entries {
		n += e.unrolled()
	}
	return n
}

// listImpl implements the recovered-error list apart from the model, charging for rec.since through
// the budget of its own model, which answers every other operation. mut breaks one part of rec.since:
// "keep" leaves the entries after the mark in place, "copy" makes a list of the errors one by one, as
// a list of copied errors does, charging one unit for each (an error alone stays the entry it is), and
// "wrap" puts a new list of the one entry
// after the mark in its place, which leaves the errors and the results as they are: only the lists that
// listing the errors visits grow. "wrap3" puts a new list around each list of three errors that it
// makes, and returns it.
type listImpl struct {
	modelImpl
	mut   string
	rec   []*errList
	spent uint64
}

func (m *listImpl) Used() uint64 { return m.spent }

func (m *listImpl) Recovered() []int64 {
	var out []int64
	for _, e := range m.rec {
		out = e.ids(out)
	}
	return out
}

func (m *listImpl) Unrolled() int {
	n := 0
	for _, e := range m.rec {
		n += e.unrolled()
	}
	return n
}

func (m *listImpl) List(l any) (visits, errs int) {
	x := l.(*errList)
	return x.unrolled(), x.n
}

func (m *listImpl) charge(n int) {
	if m.Model.Absorb(uint64(n)) {
		panic(exec.Interrupt{Reason: "limit"})
	}
	m.spent += uint64(n)
}

func (m *listImpl) Call(o *exec.Op, a []any) []any {
	switch o.Name {
	case "rec.mark":
		return []any{int64(len(m.rec))}
	case "rec.push":
		m.rec = append(m.rec, &errList{err: a[0].(*ref.Err).ID, n: 1})
		return nil
	case "rec.truncate":
		m.rec = m.rec[:a[0].(int64)]
		return nil
	case "rec.since":
		mark := a[0].(int64)
		after := m.rec[mark:]
		if m.mut == "copy" && (len(after) > 1 || len(after) == 1 && after[0].entries != nil) {
			l := &errList{entries: []*errList{}}
			for _, e := range after {
				for _, id := range e.ids(nil) {
					m.charge(1)
					l.entries = append(l.entries, &errList{err: id, n: 1})
					l.n++
				}
			}
			m.rec = append(m.rec[:mark:mark], l)
			return []any{l}
		}
		m.charge(len(after))
		switch {
		case len(after) == 0:
			return []any{&errList{entries: []*errList{}}}
		case len(after) == 1 && m.mut == "wrap":
			l := after[0]
			m.rec[mark] = &errList{entries: []*errList{l}, n: l.n, wrap: true}
			for l.wrap {
				l = l.entries[0]
			}
			return []any{l}
		case len(after) == 1:
			return []any{after[0]}
		}
		l := &errList{entries: append([]*errList(nil), after...)}
		for _, e := range after {
			l.n += e.n
		}
		if m.mut == "wrap3" && l.n == 3 {
			l = &errList{entries: []*errList{l}, n: l.n}
		}
		if m.mut != "keep" {
			m.rec = append(m.rec[:mark:mark], l)
		}
		return []any{l}
	case "rec.add":
		if l := a[0].(*errList); l.n > 0 {
			m.rec = append(m.rec, l)
		}
		return nil
	case "rec.none":
		return []any{a[0].(*errList).n == 0}
	case "stream.settleErrors":
		m.rec = nil
		return nil
	}
	return m.Model.Call(o, a)
}

// TestListImplPasses checks a list of shared errors written apart from the model against its cases:
// the positive control of TestListMutantsFail.
func TestListImplPasses(t *testing.T) {
	s := Suite{Name: "shared lists", Ops: allOps, Exact: []string{"rec.since"}, New: func(c ref.Config) Impl { return &listImpl{modelImpl: modelImpl{ref.New(c)}} }}
	if f := firstFailure(s); f != nil {
		t.Fatalf("failing case %s, call %d: %s\n%v", f.Case, f.Step, f.Msg, f.Trace)
	}
}

// TestListMutantsFail is the negative control of the contract of rec.since: a list that is not put in
// place of the entries after the mark fails, a list of copied errors fails for its charges, and a list
// that wraps the one entry after the mark, which changes nothing else, fails for the lists that listing
// the errors visits: the bound of Lemma 14 that the returned entry keeps. A list of three errors made of
// an error and a list of two has the largest unrolled size that the bound allows, 5, so a list around
// it fails the bound on the lists that rec.since returns.
func TestListMutantsFail(t *testing.T) {
	for _, mu := range []struct{ mut, want string }{
		{"keep", ""},
		{"copy", "more than the bound"},
		{"wrap", "visits"},
		{"wrap3", "errors of the list"},
	} {
		t.Run(mu.mut, func(t *testing.T) {
			s := Suite{Name: mu.mut, Ops: allOps, New: func(c ref.Config) Impl { return &listImpl{modelImpl: modelImpl{ref.New(c)}, mut: mu.mut} }}
			f := firstFailure(s)
			if f == nil {
				t.Fatal("no case detected the mutant")
			}
			t.Logf("failing case %s, call %d: %s", f.Case, f.Step, f.Msg)
			if !strings.Contains(f.Msg, mu.want) {
				t.Fatalf("failed for another reason than %q:\n%v", mu.want, f)
			}
		})
	}
}
