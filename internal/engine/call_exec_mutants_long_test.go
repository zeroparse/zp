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
	mutants := []struct {
		name, old, new string
		// tests is the -run pattern, pkgs the packages, of the tests that detect the mutant.
		tests string
		pkgs  []string
	}{
		{"a failed call keeps its recovered errors",
			"if !r.Leader && !ok {\n\t\tp.Recovered = p.Recovered[:rec]\n", "if !r.Leader && !ok {\n",
			"^TestDocumentFailedEntriesHoldNoErrors$", []string{"./document"}},
		{"an entry examines no input beyond its position", examine,
			"entry.Examined, entry.From, entry.Positional = start, p.lw, false",
			"^TestDocumentMatchesFreshParse$", []string{"."}},
		{"an entry examines nothing before its position", examine,
			"entry.Examined, entry.From, entry.Positional = p.Hw, start, false",
			"^TestDocumentEntriesExamineFromTheirPosition$", []string{"./document"}},
		{"a call reuses a result evaluated in a lookahead", reuse,
			"if found && (!c.ctx || p.ctxAllows(&c, e)) && true && ",
			"^TestMemoLookaheadStateEvaluations$", []string{"."}},
		{"a call in a context ignores the growths present", reuse,
			"if found && true && (!e.silent || p.Silent > 0) && ",
			"^TestLeftRecursionMemberMemoStale$", []string{"."}},
		{"a call leaves the examined end of its caller", "p.Hw = max(oldHigh, p.Hw)", "p.Hw = oldHigh",
			"^TestDocumentMatchesFreshParse$", []string{"."}},
	}
	dir := t.TempDir()
	for i, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			if n := strings.Count(string(src), m.old); n != 1 {
				t.Fatalf("core/call_exec.go has %d copies of %q, not one", n, m.old)
			}
			mutant := filepath.Join(dir, "call_exec_"+string(rune('a'+i))+".go")
			if err := os.WriteFile(mutant, []byte(strings.Replace(string(src), m.old, m.new, 1)), 0o644); err != nil {
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
