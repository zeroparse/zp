//go:build zp_long

package engine_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A storePolicyEdit replaces the only copy of old in the file (relative to this package).
type storePolicyEdit struct{ file, old, new string }

// storePolicyMutant breaks the memo in a copy of source files of the engine. The envelope test
// of the policy, test, must fail when the engine builds with the copy, and the failure must name the
// case, which exercises what the mutant breaks, and the backends, which it must fail on.
type storePolicyMutant struct {
	name  string
	edits []storePolicyEdit
	test  string
	// failing is the case that must fail, and backends the backends that fail it, in the engine's
	// error lines (`<case> <backend> ...`).
	failing  string
	backends []string
}

// TestMemoStorePolicyMutantsFail is the negative control of the envelope tests of the store policy
// (TestMemoStorePolicy and TestMemoStorePolicyDocument): each mutant makes the engine defer a result
// that the policy requires to be stored, in a copy of the source of its memo, so that a rule is
// evaluated more often than a memo that defers every first call evaluates it, and the test must fail
// on the case that has the calls where the mutant differs.
func TestMemoStorePolicyMutantsFail(t *testing.T) {
	const firstCall = "\treturn p.memoFirstCall(p.Pos, r.Seen)\n"
	const dense = "\t\tfirst = t.MarkSeen(pos, r)\n"
	const sparse = "\t\tfirst = markSeenPage(page, pos)\n"
	const document = "\tfm = p.FreeBegin(p.docOnlyMemo(r))\n\tdirect = !p.memoized(r) && !r.Leader || p.FirstCall(r)\n"
	const iterative = "\tdirect := !p.Memoizes(r) || p.FirstCall(r)\n"
	all := []string{"closure", "bytecode", "bytecode-iterative"}
	mutants := []storePolicyMutant{
		{"a call inside a lookahead defers its own first call",
			[]storePolicyEdit{{"state/runtime.go", firstCall, "\tpos := p.Pos\n\tif p.Silent > 0 {\n\t\tpos += 1 << 12\n\t}\n\treturn p.memoFirstCall(pos, r.Seen)\n"}},
			"TestMemoStorePolicy", "inside first", all},
		{"a second call at a position defers as well (dense table)",
			[]storePolicyEdit{{"state/memo.go", dense, "\t\tfirst = t.MarkSeen(pos, r) || t.MarkSeen(pos+1<<12, r)\n"}},
			"TestMemoStorePolicy", "outside only", all},
		{"a second call at a position defers as well (sparse table)",
			[]storePolicyEdit{{"state/memo.go", sparse, "\t\tfirst = markSeenPage(page, pos) || t.MarkSparseSeen(pos+1<<12, r)\n"}},
			"TestMemoStorePolicy", "inside first, sparse", all},
		{"a Document stores no result computed inside a lookahead",
			[]storePolicyEdit{
				// The closure and the recursive machine call callDoc; the iterative machine DirectCallDoc.
				{"state/call_exec.go", document, document[:len(document)-1] + " || p.Silent > 0\n"},
				{"state/call_ops.go", iterative, "\tdirect := !p.Memoizes(r) || p.FirstCall(r) || p.Silent > 0\n"},
			},
			"TestMemoStorePolicyDocument", "inside first", all},
	}
	for i, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			replace := map[string]string{}
			texts := map[string]string{}
			for _, e := range m.edits {
				file, err := filepath.Abs(e.file)
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := texts[file]; !ok {
					src, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					texts[file] = string(src)
				}
				if n := strings.Count(texts[file], e.old); n != 1 {
					t.Fatalf("%s has %d copies of %q, not one", e.file, n, e.old)
				}
				texts[file] = strings.Replace(texts[file], e.old, e.new, 1)
			}
			j := 0
			for file, text := range texts {
				mutant := filepath.Join(dir, fmt.Sprintf("mutant_%d_%d.go", i, j))
				j++
				if err := os.WriteFile(mutant, []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
				replace[file] = mutant
			}
			ov := filepath.Join(dir, fmt.Sprintf("overlay_%d.json", i))
			overlay, err := json.Marshal(map[string]map[string]string{"Replace": replace})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ov, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "test", "-count=1", "-overlay", ov, "-run", "^"+m.test+"$", ".")
			// The tests run in this module, which resolves without -mod=mod (TestMain).
			cmd.Env = append(os.Environ(), "GOFLAGS=-trimpath")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the tests passed with the mutant:\n%s", out)
			}
			if !strings.Contains(string(out), "--- FAIL: "+m.test) {
				t.Fatalf("the tests failed for another reason than a failing test:\n%s", out)
			}
			for _, b := range m.backends {
				line := regexp.MustCompile(`store_policy_test\.go:\d+: ` + regexp.QuoteMeta(m.failing) + ` ` + regexp.QuoteMeta(b) + `[ :]`)
				if !line.Match(out) {
					t.Errorf("the case %q did not fail on %s:\n%s", m.failing, b, out)
				}
			}
		})
	}
}
