package archtest

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/testing/importgroups"
)

// importGroupExceptions lists tracked Go files that the import grouping rule does not
// cover, each with its reason. It is empty: every tracked Go file follows the rule.
var importGroupExceptions = map[string]string{}

// TestImportGroups checks that every tracked Go file, and every Go file the generator
// prints, groups its imports by the rule in docs/development.md: at most one blank
// line, between the standard library and the rest, each group sorted by path.
// hack/fix-imports.sh rewrites a file to the rule.
func TestImportGroups(t *testing.T) {
	out, err := exec.Command("git", "-C", moduleRoot, "ls-files", "-z", "*.go").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	checked := 0
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p == "" || importGroupExceptions[p] != "" {
			continue
		}
		src, err := os.ReadFile(filepath.Join(moduleRoot, p))
		if err != nil {
			continue // listed but deleted in the working tree
		}
		checked++
		for _, m := range importgroups.Check(p, src) {
			t.Errorf("%s: %s (run hack/fix-imports.sh)", p, m)
		}
	}
	if checked < 500 {
		t.Fatalf("checked only %d files; the listing of tracked files is wrong", checked)
	}
	for _, g := range generatedParsers(t) {
		for _, path := range slices.Sorted(maps.Keys(g.sources)) {
			for _, m := range importgroups.Check(path, g.sources[path]) {
				t.Errorf("%s: %s: %s (fix the generator)", g.name, path, m)
			}
		}
	}
}

// TestImportGroupsNegative runs the check on sources that break each part of the rule,
// and on sources that follow it.
func TestImportGroupsNegative(t *testing.T) {
	const head = "package p\n\nimport (\n"
	for _, c := range []struct {
		name, imports string
		want          string // substring of the violation; empty for none
	}{
		{"conforming", "\t\"fmt\"\n\t\"os\"\n\n\t\"example.com/m/a\"\n\t\"example.com/m/b\"\n", ""},
		{"only standard", "\t\"fmt\"\n\t\"os\"\n", ""},
		{"only other", "\t\"example.com/m/a\"\n", ""},
		{"named and blank", "\t\"fmt\"\n\n\t_ \"example.com/m/a\"\n\tx \"example.com/m/b\"\n", ""},
		{"comments", "\t// fmt prints.\n\t\"fmt\"\n\n\t\"example.com/m/a\" // a\n", ""},
		{"three groups", "\t\"fmt\"\n\n\t\"example.com/m/a\"\n\n\t\"example.com/m/b\"\n", "3 import groups"},
		{"other in std group", "\t\"fmt\"\n\t\"example.com/m/a\"\n\n\t\"example.com/m/b\"\n", "non-standard import"},
		{"std in second group", "\t\"fmt\"\n\n\t\"os\"\n\t\"example.com/m/a\"\n", "standard library import"},
		{"mixed in one group", "\t\"fmt\"\n\t\"example.com/m/a\"\n", "share one group"},
		{"unsorted std", "\t\"os\"\n\t\"fmt\"\n", "not sorted"},
		{"unsorted other", "\t\"fmt\"\n\n\t\"example.com/m/b\"\n\t\"example.com/m/a\"\n", "not sorted"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := importgroups.Check("p.go", []byte(head+c.imports+")\n"))
			switch {
			case c.want == "" && len(got) > 0:
				t.Errorf("unexpected violations: %v", got)
			case c.want != "" && !strings.Contains(strings.Join(got, "\n"), c.want):
				t.Errorf("violations %v do not mention %q", got, c.want)
			}
			// The fixer must turn every source into one that passes and keep all lines.
			fixed, skipped, err := importgroups.Fix("p.go", []byte(head+c.imports+")\n"))
			if err != nil || len(skipped) > 0 {
				t.Fatalf("fix: %v, skipped %v", err, skipped)
			}
			if again := importgroups.Check("p.go", fixed); len(again) > 0 {
				t.Errorf("fixed source still violates: %v\n%s", again, fixed)
			}
		})
	}
}
