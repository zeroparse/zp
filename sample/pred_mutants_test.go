package sample_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// predicateMutants each break one way in which the generator gives predicates the values that the
// parser builds.
var predicateMutants = []struct{ name, file, old, new string }{
	{"positions in bytes", "pred.go", "end := start + utf8.RuneCountInString(text)", "end := start + len(text)"},
	{"a list has no elements", "pred.go", "items[i] = g.refValue(it, false)", "items[i] = ref.Unknown; _ = it"},
	{"an iteration has no captures", "gen.go", "caps: inner, capsKnown: true}", "caps: inner}"},
	{"a rule's node has no captures", "gen.go", "caps: caps, capsKnown: known}", "caps: caps}"},
	{"a hidden rule makes a node", "gen.go", "case strings.HasPrefix(ri.def.Name, \"_\"):", "case strings.HasPrefix(ri.def.Name, \"_\") && false:"},
	{"the oldest variable wins", "pred.go", "if _, ok := m[v.name]; !ok {\n\t\t\tm[v.name] = v.v", "if true {\n\t\t\tm[v.name] = v.v"},
	{"an undecided predicate fails", "pred.go", "return holds || !known, nil", "return holds || !known && false, nil"},
	{"aliases are unknown", "pred.go", "Zeros: t.zeros, Aliases: t.aliases}\n}", "Zeros: t.zeros}\n}"},
	{"every declared type makes a node", "gen.go", "case ri.makesNode || caps != nil", "case ri.def.Type != nil || caps != nil"},
	{"aliases of a node type are not followed", "pred.go", "if a, ok := t.aliases[x.Name]; ok {\n\t\t\t\tty = a", "if a, ok := t.aliases[x.Name]; ok && false {\n\t\t\t\tty = a"},
	{"a capture that did not match is unknown", "pred.go", "if capture {\n\t\t\treturn ref.Nil", "if false {\n\t\t\treturn ref.Nil"},
}

// TestPredicateMutants is the proof that TestPredicateSemantics holds the generator to the values that
// the parser builds: each mutant, built over the package's sources with -overlay, fails it.
func TestPredicateMutants(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range predicateMutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(dir, m.file)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(string(src), m.old); n != 1 {
				t.Fatalf("%s has %d copies of %q", m.file, n, m.old)
			}
			tmp := t.TempDir()
			mutated := filepath.Join(tmp, m.file)
			if err := os.WriteFile(mutated, []byte(strings.Replace(string(src), m.old, m.new, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			overlay, _ := json.Marshal(map[string]any{"Replace": map[string]string{path: mutated}})
			ov := filepath.Join(tmp, "overlay.json")
			if err := os.WriteFile(ov, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "test", "-count=1", "-overlay", ov, "-run", "^TestPredicateSemantics$", ".")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the test passes with the mutant:\n%s", out)
			}
			if !strings.Contains(string(out), "--- FAIL: TestPredicateSemantics") {
				t.Fatalf("the mutant does not fail TestPredicateSemantics:\n%s", out)
			}
		})
	}
}
