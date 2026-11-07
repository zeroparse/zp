//go:build !zp_budget_controls_reference

package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/limits"
)

// envKeyGrammars memoize the rule u under a key that holds the string s, which the text of the head
// sets: a lookup of the key compares a string that grows with the text. In "leaf" u is a leaf rule, which
// only a Document parse memoizes; in "nonleaf" it calls another rule, so a parse from scratch memoizes
// it too. Both are called twice at each position.
var envKeyGrammars = map[string]string{
	"leaf":    "var s string\nrule main = h:head { s := text($h) } \";\" (u \"!\" / u)* $$\nrule head = [a-z]* #atomic\nrule u = { s != \"\" } \"b\"",
	"nonleaf": "var s string\nrule main = h:head { s := text($h) } \";\" (u \"!\" / u)* $$\nrule head = [a-z]* #atomic\nrule u = { s != \"\" } bb\nrule bb = \"b\"",
}

func envKeyText(n int) string { return strings.Repeat("a", n) + ";" + strings.Repeat("b", n) }

// freshAndDocumentUnits returns the units that a parse from scratch and the first parse of a Document
// reserve in all, counted or not, and the error of each.
func freshAndDocumentUnits(t *testing.T, prog *Program, text string, o ParseOptions) (fresh, doc uint64, ferr, derr error) {
	t.Helper()
	of := o
	of.Trace = func(e TraceEvent) { fresh = core.TraceParser(e).Control.Used }
	_, ferr = prog.ParseWith("main", text, of)
	d, err := prog.NewDocumentWith("main", text, o)
	if err != nil {
		t.Fatal(err)
	}
	od := o
	od.Trace = func(e TraceEvent) { c := core.TraceParser(e).Control; doc = c.Used }
	_, derr = d.ParseWith(od)
	return
}

// The comparison of memo keys costs what it does: a key whose string is the same bytes compares in a
// constant, in a parse from scratch and in a Document, and a key that only equals does not match in a
// Document at all. A Document parse of the texts completes and does a constant factor of the work of a
// parse from scratch, counted or not, however long the text: it does not grow with the product of the
// text and the key.
func TestBudgetDocumentCountsMemoKeyWork(t *testing.T) {
	t.Parallel()
	for name, src := range envKeyGrammars {
		prog := mustCompile(t, src)
		for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
			t.Run(fmt.Sprintf("%s/%v", name, back), func(t *testing.T) {
				t.Parallel()
				o := ParseOptions{Backend: back, Unit: Bytes}
				for _, n := range []int{2000, 64000} {
					fresh, doc, ferr, derr := freshAndDocumentUnits(t, prog, envKeyText(n), o)
					if ferr != nil || derr != nil {
						t.Fatalf("n=%d: parse from scratch: %.80v; Document parse: %.80v", n, ferr, derr)
					}
					if doc > 3*fresh {
						t.Errorf("n=%d: the Document reserves %d units, more than 3 times the %d of a parse from scratch", n, doc, fresh)
					}
				}
			})
		}
	}
}

// identityMissGrammar binds s to a string that each alternative allocates anew, with the same content,
// and calls the rule u under it at the same position in both. A parse from scratch finds the second
// call in the memo if it memoizes u; a Document finds no entry for a rule that only it memoizes unless
// the string is the same bytes, and evaluates u again.
const identityMissGrammar = "var s string\nrule main = line* $$\n" +
	"rule line = h:head { s := text($h) + \"x\" } u \"!\" / h:head { s := text($h) + \"x\" } u \"\\n\"\n" +
	"rule head = \"a\"* #atomic\nrule u = { s == \"aax\" } \"b\""

// sameBytesGrammar is identityMissGrammar with the string of s taken from the text, which both
// alternatives bind as the same bytes.
const sameBytesGrammar = "var s string\nrule main = line* $$\n" +
	"rule line = h:head { s := text($h) } u \"!\" / h:head { s := text($h) } u \"\\n\"\n" +
	"rule head = \"a\"* #atomic\nrule u = { s == \"aa\" } \"b\""

// A value that equals the key of an entry but is not the same bytes is a miss for a rule that only a
// Document memoizes, and a miss gives the result of a parse from scratch for the work that the rule
// takes: the Document parse returns the tree of the parse from scratch and does a constant factor of
// its work.
func TestBudgetDocumentIdentityMissIsSound(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, identityMissGrammar)
	text := strings.Repeat("aab\n", 3000)
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		t.Run(fmt.Sprint(back), func(t *testing.T) {
			t.Parallel()
			o := ParseOptions{Backend: back, Unit: Bytes}
			want, err := prog.ParseWith("main", text, o)
			if err != nil {
				t.Fatal(err)
			}
			fresh, doc, ferr, derr := freshAndDocumentUnits(t, prog, text, o)
			if ferr != nil || derr != nil {
				t.Fatalf("parse from scratch: %.80v; Document parse: %.80v", ferr, derr)
			}
			if doc > 3*fresh {
				t.Errorf("the Document reserves %d units, more than 3 times the %d of a parse from scratch", doc, fresh)
			}
			d, err := prog.NewDocumentWith("main", text, o)
			if err != nil {
				t.Fatal(err)
			}
			got, err := d.ParseWith(o)
			if err != nil {
				t.Fatal(err)
			}
			// The second call of u under the same bytes is found in the memo, and under equal bytes
			// that are not the same memory it is not: one miss for each line.
			same, err := mustCompile(t, sameBytesGrammar).NewDocumentWith("main", text, o)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := same.ParseWith(o); err != nil {
				t.Fatal(err)
			}
			if lines := 3000; same.Stats().MemoHits-d.Stats().MemoHits < lines {
				t.Errorf("the Document finds %d entries where the string is the same bytes and %d where it is only equal: want at least %d fewer",
					same.Stats().MemoHits, d.Stats().MemoHits, lines)
			}
			w, _ := json.Marshal(want)
			g, _ := json.Marshal(got)
			if string(w) != string(g) {
				t.Errorf("the Document parse returns a different tree than the parse from scratch")
			}
		})
	}
}

// The recovered errors of a rule that only a Document memoizes, on the parse after an edit: the units
// that the reparse reserves stay within a constant factor of those of a parse from scratch of the
// edited text, counted or not, and the Document parse stays within the default limit where the parse
// from scratch does.
func TestBudgetDocumentRecoveredErrorsOfDocumentOnlyRuleAfterEdit(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, "rule main = item* $$\nrule item = (\"a\" \";\") #recover(skip=[^;]* \";\")")
	text := strings.Repeat("b;", 2000)
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		t.Run(fmt.Sprint(back), func(t *testing.T) {
			t.Parallel()
			o := ParseOptions{Backend: back}
			d, err := prog.NewDocumentWith("main", text, o)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.ParseWith(o); err != nil && errors.Is(err, limits.ErrLimit) {
				t.Fatalf("first parse: %.80v", err)
			}
			if err := d.Edit(2000, 2001, "a"); err != nil {
				t.Fatal(err)
			}
			var reparse uint64
			od := o
			od.Trace = func(e TraceEvent) { reparse = core.TraceParser(e).Control.Used }
			_, derr := d.ParseWith(od)
			edited := text[:2000] + "a" + text[2001:]
			var fresh uint64
			of := o
			of.Trace = func(e TraceEvent) { fresh = core.TraceParser(e).Control.Used }
			_, ferr := prog.ParseWith("main", edited, of)
			if errors.Is(derr, limits.ErrLimit) || errors.Is(ferr, limits.ErrLimit) {
				t.Fatalf("limit error: reparse %.80v, fresh %.80v", derr, ferr)
			}
			if reparse > 3*fresh {
				t.Errorf("the reparse reserves %d units, more than 3 times the %d of a parse from scratch", reparse, fresh)
			}
		})
	}
}
