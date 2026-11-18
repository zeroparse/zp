package mutanttest

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// repo makes a git repository with one commit holding src/a.go and other/b.go and makes it the working directory.
func repo(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"src/a.go", "other/b.go"} {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v: %v\n%s", args, err, out)
		}
	}
	t.Chdir(dir)
}

func TestChanged(t *testing.T) {
	repo(t)
	got, err := Changed("HEAD", "src")
	if err != nil || len(got) != 0 {
		t.Fatalf("a clean tree: got %v, %v", got, err)
	}
	if err := os.WriteFile("other/b.go", []byte("package y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := Changed("HEAD", "src"); err != nil || len(got) != 0 {
		t.Fatalf("a change outside the paths: got %v, %v", got, err)
	}
	if got, err := Changed("HEAD", "other"); err != nil || !slices.Equal(got, []string{"other/b.go"}) {
		t.Fatalf("a modified file under a directory: got %v, %v", got, err)
	}
	if err := os.WriteFile("src/new.go", []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := Changed("HEAD", "src"); err != nil || !slices.Equal(got, []string{"src/new.go"}) {
		t.Fatalf("an untracked file: got %v, %v", got, err)
	}
	if err := os.WriteFile("go.mod", []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := Changed("HEAD", "src"); err != nil || !slices.Contains(got, "go.mod") {
		t.Fatalf("go.mod is a dependency of every runner: got %v, %v", got, err)
	}
	if _, err := Changed("no-such-revision", "src"); err == nil {
		t.Fatal("an unknown base must be an error, so that the runner runs")
	}
	if _, err := Changed("HEAD"); err == nil {
		t.Fatal("no paths must be an error")
	}
}

func TestSkipUnlessChanged(t *testing.T) {
	repo(t)
	t.Run("no base runs", func(t *testing.T) {
		t.Setenv(BaseEnv, "")
		SkipUnlessChanged(t, "src")
		if t.Skipped() {
			t.Fatal("skipped without a base")
		}
	})
	t.Run("unknown base runs", func(t *testing.T) {
		t.Setenv(BaseEnv, "no-such-revision")
		SkipUnlessChanged(t, "src")
		if t.Skipped() {
			t.Fatal("skipped with an unknown base")
		}
	})
	t.Run("unchanged skips", func(t *testing.T) {
		t.Setenv(BaseEnv, "HEAD")
		SkipUnlessChanged(t, "src")
		t.Fatal("did not skip")
	})
}

// TestPipelineCoversImports checks that every module package that the tests of the packages whose mutants use
// Pipeline import lies under it, so that a new import cannot leave a runner unaware of a dependency.
func TestPipelineCoversImports(t *testing.T) {
	t.Chdir("../../..")
	out, err := exec.Command("go", "list", "-deps", "-test", "-f", "{{.ImportPath}}",
		"./internal/backend/...", "./codegen/go/lib/...", "./sample", "./internal/engine/...").Output()
	if err != nil {
		t.Skipf("go list: %v", err)
	}
	const mod = "zeroparse.dev/zp/"
	for _, p := range strings.Fields(string(out)) {
		rel, ok := strings.CutPrefix(p, mod)
		if !ok {
			continue
		}
		rel, _, _ = strings.Cut(rel, " ") // "pkg [pkg.test]"
		rel = strings.TrimSuffix(strings.TrimSuffix(rel, ".test"), "_test")
		covered := false
		for _, d := range Pipeline {
			covered = covered || rel == d || strings.HasPrefix(rel, d+"/")
		}
		if !covered {
			t.Errorf("%s is imported by the tests of the mutant runners but is not under Pipeline", rel)
		}
	}
}
