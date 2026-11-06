//go:build zp_long

package engine_test

import "testing"

// TestTermMutantsFail is the negative control of the tests of the printed term compositions of the
// engine's backends: each mutant breaks one part of predicate or bind in a copy of the printed file.
//
// Two edits have no mutant, because they are equivalent. Leaving out term.dropKids in the closure
// backend changes nothing: its list built-ins drop what they gathered where they fail, and the
// composition finds nothing above its mark; the bytecode backends gather without such a drop, and
// their mutant below fails. Leaving out the refusal of a function changes nothing for a compiled
// grammar: the compiler lets a function reach only filter, foldl, foldr and map, so no term of a
// binding evaluates to one; TestTerms holds the composition to the refusal.
func TestTermMutantsFail(t *testing.T) {
	const predicateEnd = "\tif len(p.Created) > created {\n\t\tp.DropCreated(created)\n\t}\n\treturn !core.IsFalse(v)\n"
	const bindEnd = "\tif len(p.Created) > created {\n\t\tp.DropCreated(created)\n\t}\n\tif isFunction(v) {\n"
	common := func() []printedMutant {
		return []printedMutant{
			{"a predicate holds on false",
				[]printedEdit{{"return !core.IsFalse(v)", "_ = v\n\treturn true"}},
				"^TestPredicateOutcomes$", []string{"."}},
			{"a predicate fails on any value but true",
				[]printedEdit{{"return !core.IsFalse(v)", "b, _ := v.(bool)\n\treturn b"}},
				"^TestPredicateOutcomes$", []string{"."}},
			{"a predicate holds on a term that failed",
				[]printedEdit{{"\t\treturn false\n\t}\n" + predicateEnd, "\t\treturn true\n\t}\n" + predicateEnd}},
				"^TestPredicateOutcomes$", []string{"."}},
			{"a binding binds what a failed term left",
				[]printedEdit{{"\t\treturn false\n\t}\n" + bindEnd, "\t\tp.Bind(slot, 0)\n\t\treturn true\n\t}\n" + bindEnd}},
				"^TestBindingFailure$", []string{"."}},
			{"a predicate keeps the nodes its term created",
				[]printedEdit{{predicateEnd, "\treturn !core.IsFalse(v)\n"}},
				"^TestTermChargesCreatedNodes$", []string{"."}},
		}
	}
	t.Run("closure", func(t *testing.T) { runPrintedMutants(t, "closure/term_exec.go", common()) })
	t.Run("vm", func(t *testing.T) {
		const drop = "\t\tif len(p.Created) > created {\n\t\t\tp.DropCreated(created)\n\t\t}\n\t\treturn false\n\t}\n" + predicateEnd
		runPrintedMutants(t, "vm/term_exec.go", append(common(), printedMutant{"a failed predicate keeps what its term gathered",
			[]printedEdit{{"\t\tp.DropKids(kids)\n" + drop, "\t\t_ = kids\n" + drop}},
			"^TestTermChargesGatheredValues$", []string{"."}}))
	})
}
