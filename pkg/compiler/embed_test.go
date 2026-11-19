package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/testing/codegentest"
	"zeroparse.dev/zp/pkg/compiler"
	"zeroparse.dev/zp/pkg/grammar"
)

// TestGenerateGoEmbeddedPackageBuilds generates a parser with the library embedded, as zp gen does
// by default, and builds and runs it in a module that requires only the runtime packages of
// zeroparse.dev/zp: the copy of the library must compile against them, and the parser must work
// with it.
func TestGenerateGoEmbeddedPackageBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a generated parser")
	}
	g := grammar.FromString("embedded.zp", `type Pair = { Key node[string]; Value node[string] }
@start
rule main Pair = k:[a-z]+ #atomic "=" v:[0-9]+ #atomic => Pair{Key: $k, Value: $v}`)
	files, err := compiler.GenerateGo(g, compiler.GoConfig{Module: "example.com/embedded/v2", Types: true, Recognize: true, Document: true, Observability: true}, compiler.WithStart("main"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var libFiles int
	for _, f := range files {
		if strings.HasPrefix(f.Path, "internal/libgo/") {
			libFiles++
		}
		path := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, f.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if libFiles == 0 {
		t.Fatal("the embedded mode wrote no library")
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The package and its copy of the library import only the runtime packages of
	// zeroparse.dev/zp, which a partial copy of this module holds.
	write("go.mod", codegentest.GoModAt("example.com/embedded/v2", codegentest.LibCopy()))
	write("embedded_test.go", `package embedded

import "testing"

func TestEmbedded(t *testing.T) {
	v, err := ParseValue("ab=12")
	if err != nil {
		t.Fatal(err)
	}
	if v.Key.Value != "ab" || v.Value.Value != "12" {
		t.Errorf("got %+v", v)
	}
	n, err := Parse("ab=1")
	if err != nil || n == nil {
		t.Fatal(n, err)
	}
	if _, err := Parse("ab"); err == nil {
		t.Error("an incomplete pair was accepted")
	}
}
`)
	for _, args := range [][]string{{"vet", "./..."}, {"test", "-count=1", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", codegentest.GoFlagsEnv, "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", args[0], err, out)
		}
	}
}
