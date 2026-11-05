//go:build zp_long

package engine_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCallProtocolMutantsFail is the negative control of the tests of the engine's call protocol,
// which is printed from Exec IR (core/call_exec.go): each mutant breaks one part of the protocol
// in a copy of the printed file, and the tests named for it must fail when they build with the
// copy in place of the file.
func TestCallProtocolMutantsFail(t *testing.T) {
	const reuse = "if found && (!c.ctx || p.ctxAllows(&c, e)) && (!e.silent || p.Silent > 0) && "
	const examine = "entry.Examined, entry.From, entry.Positional = p.Hw, p.lw, false"
	const route = "\tif store == storeMemo {\n"
	const copyErrs = "Errs: p.copyErrors(p.Recovered[rec:])}"
	runPrintedMutants(t, []printedMutant{
		{"a failed call keeps its recovered errors",
			[]printedEdit{{"if !r.Leader && !ok {\n\t\tp.Recovered = p.Recovered[:rec]\n", "if !r.Leader && !ok {\n"}},
			"^TestDocumentFailedEntriesHoldNoErrors$", []string{"./document"}},
		{"an entry examines no input beyond its position",
			[]printedEdit{{examine, "entry.Examined, entry.From, entry.Positional = start, p.lw, false"}},
			"^TestDocumentMatchesFreshParse$", []string{"."}},
		{"an entry examines nothing before its position",
			[]printedEdit{{examine, "entry.Examined, entry.From, entry.Positional = p.Hw, start, false"}},
			"^TestDocumentEntriesExamineFromTheirPosition$", []string{"./document"}},
		{"a call reuses a result evaluated in a lookahead",
			[]printedEdit{{reuse, "if found && (!c.ctx || p.ctxAllows(&c, e)) && true && "}},
			"^TestMemoLookaheadStateEvaluations$", []string{"."}},
		{"a call in a context ignores the growths present",
			[]printedEdit{{reuse, "if found && true && (!e.silent || p.Silent > 0) && "}},
			"^TestLeftRecursionMemberMemoStale$", []string{"."}},
		{"a call leaves the examined end of its caller",
			[]printedEdit{{"p.Hw = max(oldHigh, p.Hw)", "p.Hw = oldHigh"}},
			"^TestDocumentMatchesFreshParse$", []string{"."}},
		{"a result that goes nowhere is stored in the memo",
			[]printedEdit{{route, "\tif store != storeCtx {\n"}},
			"^TestBudgetUnrecordedContextCalls$", []string{"."}},
		{"a result that read a growing seed is stored in the memo",
			[]printedEdit{{route, "\tif store != storeNowhere {\n"}},
			"^TestLeftRecursionMemberMemoStale$", []string{"."}},
		{"a call copies its recovered errors before it keeps its expectations",
			[]printedEdit{
				{"\tfar, inner := p.Unisolate(m)\n\tkept := p.Keep(inner)\n\tstore := storeMemo\n",
					"\terrs := p.copyErrors(p.Recovered[rec:])\n\tfar, inner := p.Unisolate(m)\n\tkept := p.Keep(inner)\n\tstore := storeMemo\n"},
				{"Gen: p.Gen, Vgen: p.Gen, " + copyErrs, "Gen: p.Gen, Vgen: p.Gen, Errs: errs}"},
				{"Positional: r.Positional, " + copyErrs, "Positional: r.Positional, Errs: errs}"},
			},
			"^TestBudgetCallEndChargesErrorsAfterExpectations$", []string{"."}},
	})
}

// TestExpectationRecordMutantsFail is the negative control of the tests of the engine's expectation
// record, which is printed from Exec IR into the same file as the call protocol. Its operations are
// transitional and have no conformance cases, so these mutants are what binds the printed code. Two
// edits are left out because they change nothing: without expectations a record holds at most the
// placeholder and its filter stays empty, so a merge past the farthest position that keeps the
// record or the filter leaves the state that resetting them gives.
func TestExpectationRecordMutantsFail(t *testing.T) {
	const (
		dropped   = "if p.Silent > 0 || pos < p.Farthest {\n\t\treturn"
		reset     = "\n\t\tp.Exp = p.Exp[:p.ExpBase]\n\t\tp.ExpBits = 0\n"
		off       = "\tif !p.Expectations {\n\t\tif len(p.Exp) == p.ExpBase {\n\t\t\tp.Exp = append(p.Exp, ExpFar)\n\t\t}\n\t\treturn\n\t}\n"
		mergeOff  = "if !(p.Silent > 0 || far < p.Farthest) {"
		mergeFill = "\t\t\t\tif len(p.Exp) == p.ExpBase {\n\t\t\t\t\tp.Exp = append(p.Exp, ExpFar)\n\t\t\t\t}\n"
		last      = "\tp.LastKept = p.Arena[off:len(p.Arena):len(p.Arena)]\n"

		scan   = "^TestBudgetExpectationCollisionScanBudgetsAndShortCircuits$"
		filter = "^TestBudgetExpectationFilterFollowsTheRecord$"
		merge  = "^TestBudgetMergeExpectedForwardAliasAndBudgets$"
		noExp  = "^TestBudgetMergeExpectedWithoutExpectations$"
		unrec  = "^TestBudgetUnrecordedContextCalls$"
		kept   = "^TestBudgetExpectationKeepSnapshotsAndAtomicCopy$"
		chunks = "^TestBudgetExpectationKeepFillsChunks$"
	)
	here := []string{"."}
	runPrintedMutants(t, []printedMutant{
		{"an expectation in a lookahead is recorded",
			[]printedEdit{{dropped, "if pos < p.Farthest {\n\t\treturn"}}, scan, here},
		{"an expectation before the farthest failure is recorded",
			[]printedEdit{{dropped, "if p.Silent > 0 {\n\t\treturn"}}, scan, here},
		{"an expectation past the farthest failure leaves the position",
			[]printedEdit{{"\t\tp.Farthest = pos\n\t\tp.Exp = p.Exp[:p.ExpBase]\n", "\t\tp.Exp = p.Exp[:p.ExpBase]\n"}}, merge, here},
		{"an expectation past the farthest failure keeps the entries",
			[]printedEdit{{reset, "\n\t\tp.ExpBits = 0\n"}}, merge, here},
		{"an expectation past the farthest failure keeps the filter",
			[]printedEdit{{reset, "\n\t\tp.Exp = p.Exp[:p.ExpBase]\n"}}, filter, here},
		{"a record without expectations has no placeholder",
			[]printedEdit{{off, "\tif !p.Expectations {\n\t\treturn\n\t}\n"}}, unrec, here},
		{"a record without expectations grows a placeholder for each",
			[]printedEdit{{"\tif !p.Expectations {\n\t\tif len(p.Exp) == p.ExpBase {\n", "\tif !p.Expectations {\n\t\tif true {\n"}}, unrec, here},
		{"a record without expectations records them",
			[]printedEdit{{off, ""}}, unrec, here},
		{"a repeated expectation joins the record",
			[]printedEdit{{"\t\t\tif p.Exp[p.ExpBase+i] == id {\n\t\t\t\treturn\n\t\t\t}\n", "\t\t\t_ = i\n"}}, scan, here},
		{"the scan reads the enclosing record",
			[]printedEdit{{"for i = 0; i < (len(p.Exp) - p.ExpBase); i++ {\n\t\t\tp.Spend()\n\t\t\tif p.Exp[p.ExpBase+i] == id {",
				"for i = 0; i < len(p.Exp); i++ {\n\t\t\tp.Spend()\n\t\t\tif p.Exp[i] == id {"}}, filter, here},
		{"the scan charges nothing",
			[]printedEdit{{"i++ {\n\t\t\tp.Spend()\n\t\t\tif p.Exp[p.ExpBase+i]", "i++ {\n\t\t\tif p.Exp[p.ExpBase+i]"}}, scan, here},
		{"every expectation scans the record",
			[]printedEdit{{"\tif p.ExpBits&expBit(id) != 0 {\n", "\tif true {\n"}}, merge, here},
		{"an added expectation stays out of the filter",
			[]printedEdit{{"\tp.ExpBits |= expBit(id)\n", ""}}, filter, here},
		{"a merge charges nothing for the expectations it records",
			[]printedEdit{{"\t\t\tp.Spend()\n\t\t\tp.Expect(far, s[i])\n", "\t\t\tp.Expect(far, s[i])\n"}}, merge, here},
		{"a merge without expectations charges nothing",
			[]printedEdit{{"\t\t\tp.Spend()\n\t\t\t" + mergeOff, "\t\t\t" + mergeOff}}, noExp, here},
		{"a merge without expectations ignores a lookahead",
			[]printedEdit{{mergeOff, "if !(far < p.Farthest) {"}}, noExp, here},
		{"a merge without expectations ignores the farthest failure",
			[]printedEdit{{mergeOff, "if !(p.Silent > 0) {"}}, noExp, here},
		{"a merge without expectations leaves the position",
			[]printedEdit{{"\t\t\t\t\tp.Farthest = far\n", ""}}, noExp, here},
		{"a merge without expectations has no placeholder",
			[]printedEdit{{mergeFill, ""}}, noExp, here},
		{"a merge without expectations grows a placeholder for each",
			[]printedEdit{{mergeFill, "\t\t\t\tp.Exp = append(p.Exp, ExpFar)\n"}}, noExp, here},
		{"an empty set is kept like the last",
			[]printedEdit{{"\tif n == 0 {\n\t\treturn nil\n\t}\n\tlast := p.LastKept\n", "\tlast := p.LastKept\n"}}, kept, here},
		{"a prefix of the set kept last reuses it",
			[]printedEdit{{"\tif len(last) != n {\n", "\tif len(last) < n {\n"}}, chunks, here},
		{"a set of the same length reuses the last",
			[]printedEdit{{"\t\tif last[i] != s[i] {\n\t\t\tgoto copy\n\t\t}\n", "\t\t_ = last[i]\n"}}, kept, here},
		{"comparing with the set kept last charges nothing",
			[]printedEdit{{"i < n; i++ {\n\t\tp.Spend()\n\t\tif last[i]", "i < n; i++ {\n\t\tif last[i]"}}, kept, here},
		{"an equal set is copied",
			[]printedEdit{{"\treturn last\ncopy:\n", "\tgoto copy\ncopy:\n"}}, kept, here},
		{"a chunk charges no memory",
			[]printedEdit{{"\t\tp.MemUsed += uint64(cap(p.Arena)) * expIDBytes\n", ""}}, chunks, here},
		{"a chunk is smaller than a large set",
			[]printedEdit{{"max(1024, len(s))", "1024"}}, chunks, here},
		{"a set overflows the room left in the chunk",
			[]printedEdit{{"if cap(p.Arena)-len(p.Arena) < len(s) {", "if cap(p.Arena) < len(s) {"}}, chunks, here},
		{"a copy does not become the set kept last",
			[]printedEdit{{last + "\tk := p.LastKept\n\treturn k\n", "\tk := p.Arena[off:len(p.Arena):len(p.Arena)]\n\treturn k\n"}}, kept, here},
		{"a kept set can grow into the next",
			[]printedEdit{{last, "\tp.LastKept = p.Arena[off:len(p.Arena)]\n"}}, kept, here},
	})
}

// A printedMutant is one broken variant of the printed file.
type printedMutant struct {
	name  string
	edits []printedEdit
	// tests is the -run pattern, pkgs the packages, of the tests that detect the mutant.
	tests string
	pkgs  []string
}

// A printedEdit replaces the only copy of old in the printed file.
type printedEdit struct{ old, new string }

// runPrintedMutants builds the tests of each mutant with the mutated copy of core/call_exec.go in
// place of the file, and requires that they fail.
func runPrintedMutants(t *testing.T, mutants []printedMutant) {
	src, err := os.ReadFile("core/call_exec.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := filepath.Abs("core/call_exec.go")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for i, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			text := string(src)
			for _, e := range m.edits {
				if n := strings.Count(text, e.old); n != 1 {
					t.Fatalf("core/call_exec.go has %d copies of %q, not one", n, e.old)
				}
				text = strings.Replace(text, e.old, e.new, 1)
			}
			mutant := filepath.Join(dir, fmt.Sprintf("call_exec_%d.go", i))
			if err := os.WriteFile(mutant, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			overlay, err := json.Marshal(map[string]map[string]string{"Replace": {file: mutant}})
			if err != nil {
				t.Fatal(err)
			}
			ov := mutant + ".json"
			if err := os.WriteFile(ov, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"test", "-count=1", "-overlay", ov, "-run", m.tests}, m.pkgs...)
			cmd := exec.Command("go", args...)
			// The tests run in this module, which resolves without -mod=mod (TestMain).
			cmd.Env = append(os.Environ(), "GOFLAGS=-trimpath")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the tests passed with the mutant:\n%s", out)
			}
			if !strings.Contains(string(out), "--- FAIL: "+strings.Trim(m.tests, "^$")) {
				t.Fatalf("the tests failed for another reason than a failing test:\n%s", out)
			}
		})
	}
}
