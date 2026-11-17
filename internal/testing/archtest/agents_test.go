package archtest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestAgentsPathsExist checks that every repository path AGENTS.md names in backticks exists, so
// that the layout map there cannot go stale when a package or document moves.
func TestAgentsPathsExist(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	span := regexp.MustCompile("`([^`]+)`")
	checked := 0
	for _, m := range span.FindAllStringSubmatch(string(data), -1) {
		p := m[1]
		// A path has no space, placeholder, variable, glob or flag, and names a directory or a file.
		if strings.ContainsAny(p, " <>$*[]|=:") || strings.HasPrefix(p, "-") || strings.Contains(p, "...") {
			continue
		}
		if !strings.Contains(p, "/") && filepath.Ext(p) == "" {
			continue
		}
		// Git revisions and untracked scratch space are not tracked paths.
		if strings.Contains(p, "..") || strings.HasPrefix(p, "origin/") || strings.HasPrefix(p, ".scratch") {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			t.Errorf("AGENTS.md names %q, which does not exist: %v", p, err)
		}
		checked++
	}
	if checked < 20 {
		t.Errorf("checked only %d paths: the extraction rule no longer matches AGENTS.md", checked)
	}
}
