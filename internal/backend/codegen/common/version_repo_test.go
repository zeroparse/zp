package common

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommittedGeneratedFilesAreDevel checks that every generated Go and TypeScript file in the
// repository carries the "(devel)" stamp, so that committed output does not change with each
// commit; the files are regenerated with go run or go generate, which build without a module
// version.
func TestCommittedGeneratedFilesAreDevel(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	want := HeaderFor(develVersion)
	n := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == ".git" || name == "node_modules" || strings.HasPrefix(name, ".scratch") {
				return fs.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext != ".go" && ext != ".ts" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		line, _ := bufio.NewReader(f).ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if !IsGenerated([]byte(line)) {
			return nil
		}
		n++
		if line != want {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s: header %q, want %q; regenerate with go run ./cmd/zp or go generate", rel, line, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Error("found no generated file; the walk is broken")
	}
}
