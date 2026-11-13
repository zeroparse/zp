//go:build !zp_budget_controls_reference

package document_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	. "zeroparse.dev/zp/internal/engine"
	ecore "zeroparse.dev/zp/internal/engine/core"
)

// A Document charges by its logical state only: the capacity that its earlier parses left in the
// storage it keeps for the next parse (the stack of repetition values, and the state of the bytecode
// machine with the lists of its runs) changes nothing. The edited line resumes a record of 1000 items
// above the values of the 100 lines before it, so making room for them grows the stack from 100
// values; one Document keeps the storage of its first parse, which holds them all, and the other
// starts its second parse with none.
func TestBudgetDocumentChargesNoReusedCapacity(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = line* $$
rule line = item+ "\n"
rule item = [a-z] " "`)
	text := strings.Repeat("a \n", 100) + strings.Repeat("b ", 1000) + "\n" + strings.Repeat("a \n", 10)
	edit := 100*3 + 500*2 // inside the long line
	budgetDocumentVariants(t, func(t *testing.T, base ParseOptions) {
		used := func(dropKids bool) uint64 {
			d, err := prog.NewDocumentWith("main", text, base)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Parse(); err != nil {
				t.Fatal(err)
			}
			if err := d.Edit(edit, edit+1, "c"); err != nil {
				t.Fatal(err)
			}
			if dropKids {
				d.SetKids(nil)
				d.SetMachine(nil)
			} else if base.Backend == Closure && cap(d.Kids()) < 1100 {
				t.Fatalf("the first parse left a stack of capacity %d", cap(d.Kids()))
			}
			var n uint64
			o := base
			o.Limits = core.Limits{Compute: 1 << 50}
			o.Trace = func(e TraceEvent) { n = ecore.TraceParser(e).Control.Used }
			if _, err := d.ParseWith(o); err != nil {
				t.Fatal(err)
			}
			return n
		}
		if kept, dropped := used(false), used(true); kept != dropped {
			t.Fatalf("with the capacity of the first parse: %d units; without it: %d", kept, dropped)
		}
	})
}
