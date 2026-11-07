package runtime

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// TestMemoIndexMatchesChain drives random lookups and stores of the three memo
// paths (the table of Nodes, with and without budget controls, and the typed
// memo, likewise) against a plain map. The keys cluster on few positions, so
// the chains pass memoScan and the index is built, extended and used for
// replacement.
func TestMemoIndexMatchesChain(t *testing.T) {
	envs := [][]any{nil, {1}, {int64(1)}, {"a"}, {"ab"}, {true}, {false}, {0.0}, {1.5}, {nil}, {[2]int{1, 2}}, {&struct{}{}}}
	for i := 0; i < 40; i++ {
		envs = append(envs, []any{i, fmt.Sprint("s", i%7)})
	}
	envKey := func(env []any) string {
		s := ""
		for _, v := range env {
			if _, ok := v.(*struct{}); ok {
				s += fmt.Sprintf("%p|", v)
			} else {
				s += fmt.Sprintf("%T:%v|", v, v)
			}
		}
		return s
	}
	type kind struct {
		name     string
		budgeted bool
		typed    bool
	}
	for _, k := range []kind{{"nodes", false, false}, {"nodes budgeted", true, false}, {"typed", false, true}, {"typed budgeted", true, true}} {
		t.Run(k.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(1))
			p := &Parser{}
			if k.budgeted {
				p.Control = newBudgetControl(nil, Limits{Compute: 1 << 40})
			}
			tp := &TParser{Parser: p}
			var table *memoTable = &p.memo
			type key struct {
				rule, pos, min int
				env            string
			}
			want := map[key]any{}
			for step := 0; step < 20000; step++ {
				env := envs[rng.Intn(len(envs))]
				rule, pos, min := rng.Intn(4), rng.Intn(5), rng.Intn(2)
				wk := key{rule, pos, min, envKey(env)}
				var got any
				if k.typed {
					if e := tp.memoGet(rule, pos, min, env, false); e != nil {
						got = e
					}
				} else if e := table.getAny(p, rule, pos, min, env); e != nil {
					got = e
				}
				if got != want[wk] {
					t.Fatalf("step %d: get %+v = %p, want %p", step, wk, got, want[wk])
				}
				if rng.Intn(3) == 0 {
					continue
				}
				if k.typed {
					e := new(tmemoEntry)
					if got != nil && rng.Intn(5) == 0 {
						e = got.(*tmemoEntry)
					}
					tp.memoPut(rule, pos, min, env, false, e)
					want[wk] = e
				} else {
					e := table.alloc()
					if got != nil && rng.Intn(5) == 0 {
						e = got.(*memoEntry)
					}
					table.putAny(p, rule, pos, min, env, e)
					want[wk] = e
				}
			}
			n := 0
			if k.typed {
				for _, head := range tp.memoSlots {
					for e := head; e != nil; e = e.next {
						n++
					}
				}
				if len(tp.memoIndexes) == 0 {
					t.Fatal("no position was indexed")
				}
			} else {
				for _, head := range table.slots {
					for e := head; e != nil; e = e.next {
						n++
					}
				}
				if len(table.index) == 0 {
					t.Fatal("no position was indexed")
				}
			}
			if n != len(want) {
				t.Fatalf("chains hold %d entries, want %d", n, len(want))
			}
		})
	}
}

func (t *memoTable) getAny(p *Parser, rule, pos, min int, env []any) *memoEntry {
	if p.Control != nil {
		return p.memoGetControlled(rule, pos, min, env, false)
	}
	return t.get(rule, pos, min, env)
}

func (t *memoTable) putAny(p *Parser, rule, pos, min int, env []any, e *memoEntry) {
	if p.Control != nil {
		p.memoPutControlled(rule, pos, min, env, false, e)
		return
	}
	t.put(rule, pos, min, env, e)
}

// TestMemoIndexStreamPrune checks that pruning a stream's memo keeps the
// indexes of the positions that remain, at their new offsets.
func TestMemoIndexStreamPrune(t *testing.T) {
	p := &Parser{Stream: &streamInput{}}
	for pos := 0; pos < 6; pos++ {
		for i := 0; i < 40; i++ {
			p.memo.put(1, pos, 0, []any{i}, p.memo.alloc())
		}
		p.memo.get(1, pos, 0, []any{-1}) // builds the index
	}
	p.pruneStreamMemo(3)
	if len(p.memo.index) != 3 {
		t.Fatalf("%d indexes after the prune, want 3", len(p.memo.index))
	}
	for pos := 0; pos < 3; pos++ {
		for i := 0; i < 40; i++ {
			if p.memo.get(1, pos, 0, []any{i}) == nil {
				t.Fatalf("entry %d at position %d lost by the prune", i, pos)
			}
		}
	}
}

// TestDocOnlyLookupComparesByIdentity checks the lookup of a rule that only a Document parse memoizes: a
// key whose string has the same bytes in other memory is a miss that compares nothing by content, and a
// key with the same memory is a hit that costs a constant, whatever the length. The lookup of any other
// rule finds the equal string, at a cost of the length.
func TestDocOnlyLookupComparesByIdentity(t *testing.T) {
	big := strings.Repeat("x", 1<<16)
	clone := strings.Clone(big)
	newParser := func() *Parser {
		p := &Parser{}
		p.Control = newBudgetControl(context.Background(), Limits{Compute: 1 << 40})
		return p
	}
	cost := func(p *Parser, f func()) uint64 {
		before := p.Control.budgetUsed()
		f()
		return p.Control.budgetUsed() - before
	}
	p := newParser()
	e := p.memo.alloc()
	p.memoPutControlled(1, 0, 0, []any{big}, true, e)
	var got *memoEntry
	if c := cost(p, func() { got = p.memoGetControlled(1, 0, 0, []any{big}, true) }); got != e || c > 64 {
		t.Errorf("the same memory: entry %v at %d units, want the entry at a constant cost", got == e, c)
	}
	// The comparison of the same memory in the lookup of any other rule costs more than that of a value that
	// is not a string, and the same whatever the length.
	hit := func(key any) uint64 {
		q := newParser()
		q.memoPutControlled(1, 0, 0, []any{key}, false, q.memo.alloc())
		return cost(q, func() { q.memoGetControlled(1, 0, 0, []any{key}, false) })
	}
	intCost, smallCost, bigCost := hit(int64(7)), hit("x"), hit(big)
	if bigCost <= intCost || bigCost != smallCost {
		t.Errorf("the same memory costs %d units for %d bytes, %d for one byte, %d for an integer; want a constant above the integer's", bigCost, len(big), smallCost, intCost)
	}
	if c := cost(p, func() { got = p.memoGetControlled(1, 0, 0, []any{clone}, true) }); got != nil || c > 64 {
		t.Errorf("equal bytes in other memory: found %v at %d units, want a miss at a constant cost", got != nil, c)
	}
	if c := cost(p, func() { got = p.memoGetControlled(1, 0, 0, []any{clone}, false) }); got != e || c < uint64(len(big)) {
		t.Errorf("a rule a fresh parse memoizes: found %v at %d units, want the entry at a cost of the length", got == e, c)
	}
}
