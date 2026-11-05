//go:build zp_long

package engine_test

import (
	"encoding/json"
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
	src, err := os.ReadFile("core/call_exec.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := filepath.Abs("core/call_exec.go")
	if err != nil {
		t.Fatal(err)
	}
	const reuse = "if found && (!c.ctx || p.ctxAllows(&c, e)) && (!e.silent || p.Silent > 0) && "
	const examine = "entry.Examined, entry.From, entry.Positional = p.Hw, p.lw, false"
	const route = "\tif store == storeMemo {\n"
	const copyErrs = "Errs: p.copyErrors(p.Recovered[rec:])}"
	type edit struct{ old, new string }
	mutants := []struct {
		name  string
		edits []edit
		// tests is the -run pattern, pkgs the packages, of the tests that detect the mutant.
		tests string
		pkgs  []string
	}{
		{"a failed call keeps its recovered errors",
			[]edit{{"if !r.Leader && !ok {\n\t\tp.Recovered = p.Recovered[:rec]\n", "if !r.Leader && !ok {\n"}},
			"^TestDocumentFailedEntriesHoldNoErrors$", []string{"./document"}},
		{"an entry examines no input beyond its position",
			[]edit{{examine, "entry.Examined, entry.From, entry.Positional = start, p.lw, false"}},
			"^TestDocumentMatchesFreshParse$", []string{"."}},
		{"an entry examines nothing before its position",
			[]edit{{examine, "entry.Examined, entry.From, entry.Positional = p.Hw, start, false"}},
			"^TestDocumentEntriesExamineFromTheirPosition$", []string{"./document"}},
		{"a call reuses a result evaluated in a lookahead",
			[]edit{{reuse, "if found && (!c.ctx || p.ctxAllows(&c, e)) && true && "}},
			"^TestMemoLookaheadStateEvaluations$", []string{"."}},
		{"a call in a context ignores the growths present",
			[]edit{{reuse, "if found && true && (!e.silent || p.Silent > 0) && "}},
			"^TestLeftRecursionMemberMemoStale$", []string{"."}},
		{"a call leaves the examined end of its caller",
			[]edit{{"p.Hw = max(oldHigh, p.Hw)", "p.Hw = oldHigh"}},
			"^TestDocumentMatchesFreshParse$", []string{"."}},
		{"a result that goes nowhere is stored in the memo",
			[]edit{{route, "\tif store != storeCtx {\n"}},
			"^TestBudgetUnrecordedContextCalls$", []string{"."}},
		{"a result that read a growing seed is stored in the memo",
			[]edit{{route, "\tif store != storeNowhere {\n"}},
			"^TestLeftRecursionMemberMemoStale$", []string{"."}},
		{"a call copies its recovered errors before it keeps its expectations",
			[]edit{
				{"\tfar, inner := p.Unisolate(m)\n\tkept := p.Keep(inner)\n\tstore := storeMemo\n",
					"\terrs := p.copyErrors(p.Recovered[rec:])\n\tfar, inner := p.Unisolate(m)\n\tkept := p.Keep(inner)\n\tstore := storeMemo\n"},
				{"Gen: p.Gen, Vgen: p.Gen, " + copyErrs, "Gen: p.Gen, Vgen: p.Gen, Errs: errs}"},
				{"Positional: r.Positional, " + copyErrs, "Positional: r.Positional, Errs: errs}"},
			},
			"^TestBudgetCallEndChargesErrorsAfterExpectations$", []string{"."}},
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
			mutant := filepath.Join(dir, "call_exec_"+string(rune('a'+i))+".go")
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
