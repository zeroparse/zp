package engine_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"zeroparse.dev/zp"
)

// termBackends are the backends that print the term compositions.
var termBackends = []struct {
	name string
	b    zp.Backend
}{{"closure", zp.Closure}, {"bytecode", zp.Bytecode}, {"iterative", zp.BytecodeIterative}}

// termResult returns the tree of input as JSON, or the error, on the backend b.
func termResult(t *testing.T, src, input string, b zp.Backend) string {
	t.Helper()
	p, err := zp.CompileSource(src, "main")
	if err != nil {
		t.Fatal(err)
	}
	n, err := p.Parse(input, zp.WithBackend(b))
	if err != nil {
		return "error: " + err.Error()
	}
	out, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestTermFailureDropsGatheredValues checks that a term that fails in the middle of a list built-in
// leaves none of the list's elements for the repetition around it, whose values share their stack.
func TestTermFailureDropsGatheredValues(t *testing.T) {
	// The first fails inside map, which drops what it gathered itself; the second fails in the second
	// list of a concat whose lists the bytecode backends gather in place, after the first list's
	// elements, which only the composition drops.
	fails := []string{`{len(map(list(1, 2, 3), (x) => 10 / (2 - x))) > 0}`, `{len(concat(list(1, 2), list(10 / (1 - len($a))))) > 0}`}
	const holds = `{true}`
	// An item with an action keeps only what the action makes; one without builds its node from
	// what the stack holds, where what a term left would show.
	srcs := []func(string) string{
		func(term string) string {
			return "@start\nrule main = l:item+ $$ => $l\nrule item = a:\"a\" (" + term + " \"!\" / \"\") => $a\n"
		},
		func(term string) string {
			return "@start\nrule main = item+ $$\nrule item = a:\"a\" (" + term + " \"!\" / \"\")\n"
		},
	}
	for i, term := range fails {
		for j, src := range srcs {
			for _, be := range termBackends {
				t.Run(fmt.Sprint(i, "/", j, "/", be.name), func(t *testing.T) {
					if got, want := termResult(t, src(term), "aa", be.b), termResult(t, src(holds), "aa", be.b); got != want {
						t.Errorf("after a failed term the tree is\n%s\nwant\n%s", got, want)
					}
				})
			}
		}
	}
}

// TestPredicateOutcomes checks what a predicate answers for each value of its term.
func TestPredicateOutcomes(t *testing.T) {
	for _, c := range []struct {
		term string
		ok   bool
	}{
		{"true", true}, {"false", false}, {"1", true}, {"0", true}, {`""`, true}, {"1 / 0", false}, {"nil", true},
	} {
		src := "@start\nrule main = {" + c.term + "} \"a\" $$\n"
		for _, be := range termBackends {
			t.Run(c.term+"/"+be.name, func(t *testing.T) {
				got := termResult(t, src, "a", be.b)
				if failed := got[:6] == "error:"; failed == c.ok {
					t.Errorf("{%s} gives %s", c.term, got)
				}
			})
		}
	}
}

// TestBindingFailure checks that a binding whose term fails fails, binds nothing and leaves the
// alternative that follows the old value.
func TestBindingFailure(t *testing.T) {
	const src = `var n int = 7
@start
rule main = ({n := 1 / 0} "a" / "a") $$ => n
`
	for _, be := range termBackends {
		t.Run(be.name, func(t *testing.T) {
			if got := termResult(t, src, "a", be.b); got != "7" {
				t.Errorf("the variable is %s, want 7", got)
			}
		})
	}
}

// minimumCompute returns the smallest compute limit that parses input.
func minimumCompute(t *testing.T, p *zp.Parser, input string, b zp.Backend) uint64 {
	t.Helper()
	lo, hi := uint64(0), uint64(1)<<20 // lo fails, hi parses
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if _, err := p.Parse(input, zp.WithBackend(b), zp.WithLimits(zp.Limits{Compute: mid})); err != nil {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi
}

// TestTermChargesCreatedNodes checks that a predicate charges for forgetting each node its term
// created: one more node costs the five units of building it and gathering it, and the unit of
// forgetting it.
func TestTermChargesCreatedNodes(t *testing.T) {
	src := func(term string) string {
		return "type T = { N int }\n@start\nrule main = {" + term + "} \"a\" $$\n"
	}
	one, two := src(`list(T{N: 1})`), src(`list(T{N: 1}, T{N: 2})`)
	for _, be := range termBackends {
		t.Run(be.name, func(t *testing.T) {
			p1, err := zp.CompileSource(one, "main")
			if err != nil {
				t.Fatal(err)
			}
			p2, err := zp.CompileSource(two, "main")
			if err != nil {
				t.Fatal(err)
			}
			if d := minimumCompute(t, p2, "a", be.b) - minimumCompute(t, p1, "a", be.b); d != 6 {
				t.Errorf("one more node costs %d units, want 6", d)
			}
		})
	}
}

// TestTermChargesGatheredValues checks that a predicate whose term fails forgets each element its
// term gathered: a concat gathers the elements of its lists in place, so one more element of its
// first list, before the second fails, costs the unit of gathering it and the unit of forgetting
// it. Grammar code builds no node from what a term gathers, so the charge is where a value that
// stayed behind would show.
func TestTermChargesGatheredValues(t *testing.T) {
	src := func(first string) string {
		return "@start\nrule main = x:\"a\" ({len(concat(list(" + first + "), list(10 / (1 - len($x))))) > 0} \"!\" / \"\") $$\n"
	}
	for _, be := range termBackends {
		t.Run(be.name, func(t *testing.T) {
			p1, err := zp.CompileSource(src("1"), "main")
			if err != nil {
				t.Fatal(err)
			}
			p2, err := zp.CompileSource(src("1, 2"), "main")
			if err != nil {
				t.Fatal(err)
			}
			if d := minimumCompute(t, p2, "a", be.b) - minimumCompute(t, p1, "a", be.b); d != 2 {
				t.Errorf("one more gathered element costs %d units, want 2", d)
			}
		})
	}
}
