package language_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/testing/codegentest"
)

// workMutant makes the lexer or the builder of the grammar rescan the source or the tokens in a loop
// that allocates nothing, in the function fn of the file of internal/frontend/parser.
type workMutant struct {
	name, file, fn, old, new string
}

var workMutants = []workMutant{
	{"the lexer counts the column from the start of the line", "lexer.go", "advance",
		"\t} else {\n\t\tl.col++\n\t}",
		"\t} else {\n\t\tl.col = 1\n\t\tfor k := l.lineStart; k < l.i; k++ {\n\t\t\tif l.at(k) != '\\n' {\n\t\t\t\tl.col++\n\t\t\t}\n\t\t}\n\t}"},
	{"the builder looks for the comments of a range among all comments", "canonical.go", "commentsIn",
		"\tfor ; k < len(c.comments); k++ {\n\t\tcharge(builderWork, 1)\n\t\tcm := c.comments[k]\n\t\tr := cm.(ranged)\n\t\tif _, e := r.Range(); e > to {\n\t\t\tbreak\n\t\t}",
		"\tfor k = 0; k < len(c.comments); k++ {\n\t\tcharge(builderWork, 1)\n\t\tcm := c.comments[k]\n\t\tr := cm.(ranged)\n\t\tif s, e := r.Range(); s < from || e > to {\n\t\t\tcontinue\n\t\t}"},
	{"the layout replay looks for the token at a position from the first token", "canonical_layout.go", "seekStart",
		"\tif i, ok := r.starts[pos]; ok {\n\t\tr.seek(i)\n\t}",
		"\tfor i := 0; i < len(r.p.toks); i++ {\n\t\tif r.p.at(i).pos == pos {\n\t\t\tr.seek(i)\n\t\t\tbreak\n\t\t}\n\t}"},
}

// TestSourceAPIWorkMutants is the proof that TestSourceAPIWorkScalesLinearly, which runs in a build
// with the tag zp_workcount, sees a lexer or a builder whose work grows faster than the source without
// allocating: it passes on the source as it is, and with a source overlay of each mutant, which makes
// the work quadratic, it fails.
func TestSourceAPIWorkMutants(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	root := codegentest.ModuleRoot()
	run := func(t *testing.T, overlay string) ([]byte, error) {
		args := []string{"test", "-count=1", "-tags", "zp_workcount", "-run", "^TestSourceAPIWorkScalesLinearly$"}
		if overlay != "" {
			args = append(args, "-overlay", overlay)
		}
		cmd := exec.Command("go", append(args, "./spec/language")...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOFLAGS=")
		return cmd.CombinedOutput()
	}
	t.Run("as it is", func(t *testing.T) {
		t.Parallel()
		if out, err := run(t, ""); err != nil {
			t.Fatalf("the work of the source APIs does not scale linearly:\n%s", out)
		}
	})
	for _, m := range workMutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, "internal", "frontend", "parser", m.file)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			mutated, ok := codegentest.MutateFunc(string(src), m.fn, m.old, m.new)
			if !ok {
				t.Fatalf("%s has no %s with %q", m.file, m.fn, m.old)
			}
			dir := t.TempDir()
			file := filepath.Join(dir, m.file)
			if err := os.WriteFile(file, []byte(mutated), 0o644); err != nil {
				t.Fatal(err)
			}
			overlay, _ := json.Marshal(map[string]map[string]string{"Replace": {path: file}})
			overlayFile := filepath.Join(dir, "overlay.json")
			if err := os.WriteFile(overlayFile, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := run(t, overlayFile)
			if err == nil {
				t.Fatalf("TestSourceAPIWorkScalesLinearly passes with the mutant %q:\n%s", m.name, out)
			}
			if !strings.Contains(string(out), "--- FAIL: TestSourceAPIWorkScalesLinearly") || !strings.Contains(string(out), "grows") {
				t.Fatalf("the mutant %q does not fail TestSourceAPIWorkScalesLinearly on its growth:\n%s", m.name, out)
			}
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "grows") {
					t.Log(strings.TrimSpace(line))
					break
				}
			}
		})
	}
}
