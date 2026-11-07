//go:build !zp_budget_controls_reference

package engine

import (
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/limits"
)

// After edits that rewrite the end of the head and keep its length, the chain of each position still
// holds the entries that earlier texts stored there under other strings. A parse from scratch of the
// current text has no such entry, so comparing a key with one is work of the Document: under the default
// limit a Document parse succeeds after every edit, and what it counts stays a constant factor of what a
// parse from scratch of the same text counts.
func TestBudgetDocumentLeftoverEntriesAreNotCounted(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, walkGrammar)
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, n := range []int{100, 300} {
			t.Run(fmt.Sprintf("%v/n=%d", back, n), func(t *testing.T) {
				t.Parallel()
				o := ParseOptions{Backend: back, Unit: Bytes}
				d, err := prog.NewDocumentWith("main", envKeyText(n), o)
				if err != nil {
					t.Fatal(err)
				}
				var used, doc uint64
				od := o
				od.Trace = func(e TraceEvent) { c := core.TraceParser(e).Control; used, doc = c.Used, c.Doc }
				budget := limits.DefaultBudget(uint64(2*n + 1))
				for k := 0; k < 12; k++ {
					rep := fmt.Sprintf("%c%c", 'c'+k/6, 'c'+k%6)
					if err := d.Edit(n-2, n, rep); err != nil {
						t.Fatal(err)
					}
					if _, err := d.ParseWith(od); err != nil {
						t.Fatalf("edit %d: Document parse with the default limit %d: %.100v", k, budget, err)
					}
					counted := used - doc
					var fresh uint64
					of := o
					of.Trace = func(e TraceEvent) { c := core.TraceParser(e).Control; fresh = c.Used - c.Doc }
					cur := strings.Repeat("a", n-2) + rep + ";" + strings.Repeat("b", n)
					if _, err := prog.ParseWith("main", cur, of); err != nil {
						t.Fatalf("edit %d: parse from scratch: %.100v", k, err)
					}
					if counted > 2*fresh {
						t.Fatalf("edit %d: the Document counts %d units, more than twice the %d of a parse from scratch", k, counted, fresh)
					}
				}
			})
		}
	}
}
