package gogen_test

import (
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// termChargePairs are pairs of grammars whose second term handles one value more than the first at a
// place where a predicate or a binding forgets what its term left: the nodes a term created, when the
// term succeeds or fails, and the values it gathered, when it fails. A concat gathers the elements of
// its lists in place, so one more element of its first list, before the second fails, is one more
// value gathered and forgotten. delta is what the second spends more than the first on the input "a",
// on both entries: building and gathering the value, and the unit of forgetting it, which a runtime that
// forgets without charging leaves out (see TestGeneratedTermChargeMutants).
var termChargePairs = []struct {
	name, one, two string
	delta          int64
}{
	{"a predicate forgets the nodes its term created",
		termPredicate(`list(T{N: 1})`), termPredicate(`list(T{N: 1}, T{N: 2})`), 7},
	{"a binding forgets the nodes its term created",
		termBinding(`len(list(T{N: 1}))`), termBinding(`len(list(T{N: 1}, T{N: 2}))`), 7},
	{"a failed predicate forgets the values its term gathered",
		termFailedPredicate(`concat(list(1), list(10 / (1 - len($x))))`), termFailedPredicate(`concat(list(1, 2), list(10 / (1 - len($x))))`), 3},
	{"a failed binding forgets the values its term gathered",
		termFailedBinding(`concat(list(1), list(10 / (1 - len($x))))`), termFailedBinding(`concat(list(1, 2), list(10 / (1 - len($x))))`), 3},
	{"a failed predicate forgets the nodes its term created",
		termFailedPredicate(`list(T{N: 1}, 10 / (1 - len($x)))`), termFailedPredicate(`list(T{N: 1}, T{N: 2}, 10 / (1 - len($x)))`), 4},
	{"a failed binding forgets the nodes its term created",
		termFailedBinding(`list(T{N: 1}, 10 / (1 - len($x)))`), termFailedBinding(`list(T{N: 1}, T{N: 2}, 10 / (1 - len($x)))`), 4},
}

func termPredicate(term string) string {
	return "type T = { N int }\n@start\nrule main = {" + term + "} \"a\" $$\n"
}

func termBinding(term string) string {
	return "type T = { N int }\nvar n int = 0\n@start\nrule main = {n := " + term + "} \"a\" $$ => n\n"
}

// termFailedPredicate and termFailedBinding make a term that fails on the input "a", whose
// alternative then parses it.
func termFailedPredicate(term string) string {
	return "type T = { N int }\n@start\nrule main = x:\"a\" ({len(" + term + ") > 0} \"!\" / \"\") $$\n"
}

func termFailedBinding(term string) string {
	return "type T = { N int }\nvar n int = 7\n@start\nrule main = x:\"a\" ({n := len(" + term + ")} \"!\" / \"\") $$ => n\n"
}

// termChargeDeltas builds the pairs of termChargePairs against the runtime module in rt and returns, by
// pair and entry, what the second grammar spends more than the first.
func termChargeDeltas(t *testing.T, rt string) map[string]int64 {
	t.Helper()
	var cases []gentest.Case
	for _, p := range termChargePairs {
		cases = append(cases, gentest.Case{Name: p.name + " (one)", Src: p.one, Inputs: []string{"a"}},
			gentest.Case{Name: p.name + " (two)", Src: p.two, Inputs: []string{"a"}})
	}
	got, problems := budgetTotalsAt(t, rt, "terms", cases, "main", nil)
	for _, p := range problems {
		t.Error(p)
	}
	spend := func(name, entry string) int64 {
		v, ok := got["terms/"+name+" | 0 "+entry]
		if !ok {
			t.Fatalf("no report for %s, %s", name, entry)
		}
		f := strings.Fields(v)
		if f[2] != `"<nil>"` {
			t.Fatalf("%s, %s: the parse fails: %s", name, entry, v)
		}
		n, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	deltas := map[string]int64{}
	for _, p := range termChargePairs {
		for _, entry := range []string{"parse", "recognize"} {
			deltas[p.name+" | "+entry] = spend(p.name+" (two)", entry) - spend(p.name+" (one)", entry)
		}
	}
	return deltas
}

// termChargeWant returns the deltas of termChargePairs in the form of termChargeDeltas.
func termChargeWant() map[string]int64 {
	want := map[string]int64{}
	for _, p := range termChargePairs {
		for _, entry := range []string{"parse", "recognize"} {
			want[p.name+" | "+entry] = p.delta
		}
	}
	return want
}

// TestGeneratedTermChargesPerValue checks the charges of generated Go for each value that a predicate
// or a binding forgets, which the totals of the budget goldens mix with everything else a parse spends.
func TestGeneratedTermChargesPerValue(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	got := termChargeDeltas(t, gentest.RuntimeCopy())
	for k, want := range termChargeWant() {
		if got[k] != want {
			t.Errorf("%s: one more value costs %d units, want %d", k, got[k], want)
		}
	}
}
