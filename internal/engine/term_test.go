package engine_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
	ecore "zeroparse.dev/zp/internal/engine/core"
)

// termBackends are the backends that print the term compositions.
var termBackends = []struct {
	name string
	b    engine.Backend
}{{"closure", engine.Closure}, {"bytecode", engine.Bytecode}, {"iterative", engine.BytecodeIterative}}

// termResult returns the tree of input as JSON, or the error, on the backend b.
func termResult(t *testing.T, src, input string, b engine.Backend) string {
	t.Helper()
	p, err := engine.CompileSource(src, "main")
	if err != nil {
		t.Fatal(err)
	}
	n, err := p.Parse(input, engine.WithBackend(b))
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
func minimumCompute(t *testing.T, p *engine.Parser, input string, b engine.Backend) uint64 {
	t.Helper()
	lo, hi := uint64(0), uint64(1)<<20 // lo fails, hi parses
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if _, err := p.Parse(input, engine.WithBackend(b), engine.WithLimits(core.Limits{Compute: mid})); err != nil {
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
	if !ecore.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	src := func(term string) string {
		return "type T = { N int }\n@start\nrule main = {" + term + "} \"a\" $$\n"
	}
	one, two := src(`list(T{N: 1})`), src(`list(T{N: 1}, T{N: 2})`)
	for _, be := range termBackends {
		t.Run(be.name, func(t *testing.T) {
			p1, err := engine.CompileSource(one, "main")
			if err != nil {
				t.Fatal(err)
			}
			p2, err := engine.CompileSource(two, "main")
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
	if !ecore.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	src := func(first string) string {
		return "@start\nrule main = x:\"a\" ({len(concat(list(" + first + "), list(10 / (1 - len($x))))) > 0} \"!\" / \"\") $$\n"
	}
	for _, be := range termBackends {
		t.Run(be.name, func(t *testing.T) {
			p1, err := engine.CompileSource(src("1"), "main")
			if err != nil {
				t.Fatal(err)
			}
			p2, err := engine.CompileSource(src("1, 2"), "main")
			if err != nil {
				t.Fatal(err)
			}
			if d := minimumCompute(t, p2, "a", be.b) - minimumCompute(t, p1, "a", be.b); d != 2 {
				t.Errorf("one more gathered element costs %d units, want 2", d)
			}
		})
	}
}

// TestTermChargesOperatorsLocals checks that reading an Operators Rule local charges one unit for
// each local the lookup passes: an infix action holds $rhs, $lhs and $op, in the order the lookup
// walks them, so finding $op passes two locals more than finding $rhs. Neither "rhs" nor "lhs" has
// the length of "op", so passing them compares no text, which has charges of its own.
func TestTermChargesOperatorsLocals(t *testing.T) {
	if !ecore.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	src := func(local string) string {
		return `@start
rule main = e $$
rule e = #operators {
    operand [a-z]
    level { infix left "+" => ` + local + ` }
}
`
	}
	for _, be := range termBackends {
		t.Run(be.name, func(t *testing.T) {
			cost := map[string]uint64{}
			for _, local := range []string{"$rhs", "$op"} {
				p, err := engine.CompileSource(src(local), "main")
				if err != nil {
					t.Fatal(err)
				}
				cost[local] = minimumCompute(t, p, "a+b", be.b)
			}
			if d := cost["$op"] - cost["$rhs"]; d != 2 {
				t.Errorf("$op costs %d units more than $rhs, want 2", d)
			}
		})
	}
}

// TestStructFieldValueReadOnce checks that a struct literal reads each field value once although the
// term both sets the field and covers the struct's range with the value: reading an Operators Rule
// local instead of a constant costs as much in a field as in the argument of a call, which uses its
// value once.
func TestStructFieldValueReadOnce(t *testing.T) {
	src := func(action string) string {
		return "type N = { Text string }\ntype T = { A _ }\n@start\nrule main = e $$\nrule e = #operators {\n" +
			"    operand \"n\" => N{Text: \"n\"}\n    level { infix left \"+\" => " + action + " }\n}\n"
	}
	for _, be := range termBackends {
		t.Run(be.name, func(t *testing.T) {
			cost := func(action string) uint64 {
				p, err := engine.CompileSource(src(action), "main")
				if err != nil {
					t.Fatal(err)
				}
				return minimumCompute(t, p, "n+n", be.b)
			}
			field := int64(cost(`T{A: $lhs}`)) - int64(cost(`T{A: 1}`))
			arg := int64(cost(`list($lhs)`)) - int64(cost(`list(1)`))
			if field != arg {
				t.Errorf("reading the local instead of a constant costs %d units in a field, %d in an argument", field, arg)
			}
		})
	}
}
