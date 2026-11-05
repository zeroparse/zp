package document_test

import (
	"math/rand"
	"testing"

	. "zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/engine/core"
)

// TestDocumentEntriesExamineFromTheirPosition checks the bound that the call protocol records
// with an entry as whether its examined input starts before the entry (memo.examine): a call
// examines the input from its position, or from the unit before it to tell a line or input start,
// and never further back, so every entry's From is its position or the one before. The grammars
// read the unit before a call (^ and ^^) inside memoized rules, left recursion and repetitions,
// through random edits on every backend.
func TestDocumentEntriesExamineFromTheirPosition(t *testing.T) {
	t.Parallel()
	cases := []struct{ grammar, text string }{
		{incrementalGrammar, "x = 1+2\nab,cd\n@pos\n\ny = (3)*-4\n"},
		{"rule main = item* $$\nrule item = start / word / \" \"\nrule start = ^^ ^ word \":\"\nrule word = ^ [a-z]+ #atomic / [a-z]+ #atomic", "ab: cd ef\ngh: ij\n"},
	}
	pieces := []string{"a", "b", "x = ", "1", "+", "(", ")", ",", "\n", "@", " ", ":", "zz "}
	before := 0
	for _, c := range cases {
		prog := mustCompile(t, c.grammar)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			doc, err := prog.NewDocumentWith("main", c.text, ParseOptions{Backend: b})
			if err != nil {
				t.Fatal(err)
			}
			rng := rand.New(rand.NewSource(11))
			for i := 0; i < 300; i++ {
				doc.Parse()
				doc.Memo().Each(func(e *core.MemoEntry) {
					if !core.AdvanceEntry(e, doc.Edits()) {
						return
					}
					if e.From != e.Pos && e.From != e.Pos-1 {
						t.Fatalf("%v, edit %d: entry of rule %d at %d examined from %d", b, i, e.Rule, e.Pos, e.From)
					}
					if e.From < e.Pos {
						before++
					}
				})
				cur := []rune(doc.Text())
				start := rng.Intn(len(cur) + 1)
				end := start + rng.Intn(min(4, len(cur)-start)+1)
				ins := ""
				for k := rng.Intn(3); k > 0; k-- {
					ins += pieces[rng.Intn(len(pieces))]
				}
				if err := doc.Edit(start, end, ins); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if before == 0 {
		t.Error("no entry examined the unit before it")
	}
}

// TestDocumentFailedEntriesHoldNoErrors checks that the call protocol discards the errors that a
// failed call recovered before it records the failure: the entry of a failed call holds no
// recovered errors. The grammar recovers inside a rule that then fails, through random edits on
// every backend.
func TestDocumentFailedEntriesHoldNoErrors(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, "rule main = (item / [^\\n] / \"\\n\")* $$\n"+
		"rule item = \"(\" (word (\",\" word)* \")\") #recover(skip=[^)]* \")\") \"!\"\n"+
		"rule word = [a-z]+ #atomic")
	pieces := []string{"a", "b", ",", "(", ")", "!", " ", "x", "\n"}
	failed := 0
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		doc, err := prog.NewDocumentWith("main", "(ab,cd)!(a b)x(ef)!\n(a b)!(c d)\n", ParseOptions{Backend: b})
		if err != nil {
			t.Fatal(err)
		}
		rng := rand.New(rand.NewSource(12))
		for i := 0; i < 300; i++ {
			doc.Parse()
			doc.Memo().Each(func(e *core.MemoEntry) {
				if e.Ok || !core.AdvanceEntry(e, doc.Edits()) {
					return
				}
				failed++
				if len(e.Errs) != 0 {
					t.Fatalf("%v, edit %d: failed entry of rule %d at %d holds %d recovered errors", b, i, e.Rule, e.Pos, len(e.Errs))
				}
			})
			cur := []rune(doc.Text())
			start := rng.Intn(len(cur) + 1)
			end := start + rng.Intn(min(4, len(cur)-start)+1)
			ins := ""
			for k := rng.Intn(3); k > 0; k-- {
				ins += pieces[rng.Intn(len(pieces))]
			}
			if err := doc.Edit(start, end, ins); err != nil {
				t.Fatal(err)
			}
		}
	}
	if failed == 0 {
		t.Error("no call failed")
	}
}
