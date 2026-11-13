package gogen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedLookaheadHasNoFirstTerminal checks that a generated Go parser does not skip, by the
// next character, an alternative that begins with a positive lookahead (Evaluations of Rules in
// spec/runtime/parsing.md), as TestLookaheadHasNoFirstTerminal does for the engine: on "bc",
// Parse, ParseValue and Recognize, without and with limit controls, evaluate the body of x of
// gentest.LookaheadFirstSrc once.
func TestGeneratedLookaheadHasNoFirstTerminal(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedLookaheadHasNoFirstTerminal)
}

func testGeneratedLookaheadHasNoFirstTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skip("go command not found")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gentest.GoMod("gentest")), 0o644); err != nil {
		t.Fatal(err)
	}
	var imports, calls strings.Builder
	// g has no limit controls; l has them, under the default limits.
	for _, pkg := range []string{"g", "l"} {
		gentest.GeneratePackage(t, dir, pkg, mustParse(gentest.LookaheadFirstSrc), codegen.GenOptions{Start: "main", Types: true, Recognize: true, Observability: true, LimitControls: pkg == "l"})
		imports.WriteString("\t\"gentest/" + pkg + "\"\n")
		for _, op := range []string{"_, err := %s.Parse(\"bc\", zprt.WithProfile(&p))", "_, err := %s.ParseValue(\"bc\", zprt.WithProfile(&p))", "err := %s.Recognize(\"bc\", zprt.WithProfile(&p))"} {
			calls.WriteString("\t{ var p zprt.Profile; " + strings.ReplaceAll(op, "%s", pkg) + "; fmt.Print(evals(p, err), \" \") }\n")
		}
	}
	main := `package main

import (
	"fmt"

	zprt "zeroparse.dev/zp/codegen/go/lib"
` + imports.String() + `)

// evals returns the body evaluations of x, or the error of the parse.
func evals(p zprt.Profile, err error) string {
	if err != nil {
		return err.Error()
	}
	n := 0
	for _, r := range p.Snapshot().Rules {
		if r.Rule == "x" {
			n += r.Evals
		}
	}
	return fmt.Sprint(n)
}

func main() {
` + calls.String() + `}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(goBin, "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod -trimpath", "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, out)
	}
	if got, want := strings.Join(strings.Fields(string(out)), " "), "1 1 1 1 1 1"; got != want {
		t.Errorf("x evaluated %s times (Parse, ParseValue, Recognize; then with limit controls), want %s", got, want)
	}
}
