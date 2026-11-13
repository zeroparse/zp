package gogen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/starttest"
)

// TestDocumentEmptyEnterReuse pins what the two ways of starting the examined range of a call
// (spec/exec-ir doc.enter: the empty range or one unit) can be told apart by: the Document statistics
// alone. An empty match at the start of an insertion is kept by the engine, whose range is empty, and
// dropped by generated Go, whose range is one unit; the parse results are the same.
func TestDocumentEmptyEnterReuse(t *testing.T) {
	gentest.NeedsBudgetControls(t)
	const src = "rule main = pad \"a\"+\nrule pad = \"\""
	g, err := engine.ParseGrammar(src)
	if err != nil {
		t.Fatal(err)
	}
	p, err := engine.Compile(starttest.MarkAll(g), "main")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := p.NewDocument("aa")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.Parse(); err != nil {
		t.Fatal(err)
	}
	if err := doc.Edit(0, 0, "a"); err != nil {
		t.Fatal(err)
	}
	engineNode, err := doc.Parse()
	if err != nil {
		t.Fatal(err)
	}
	engineReused := doc.Stats().Reused

	pg, err := parseTest(src)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gentest.GenerateCode(t, pg, codegen.GenOptions{Package: "fixture", Start: "main", Document: true})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, source := range map[string]string{
		"go.mod":    gentest.GoMod("fixture"),
		"parser.go": string(code),
		"parser_test.go": `package fixture
import ("fmt"; "testing")
func TestEnter(t *testing.T) {
	d, err := NewDocument("aa")
	if err != nil { t.Fatal(err) }
	if _, err := d.Parse(); err != nil { t.Fatal(err) }
	if err := d.Edit(0, 0, "a"); err != nil { t.Fatal(err) }
	n, err := d.Parse()
	if err != nil { t.Fatal(err) }
	fmt.Printf("REUSED %d NODE %v\n", d.Stats().Reused, n)
}
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-v", "-run", "TestEnter", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod -trimpath")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated Document: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`REUSED (\d+) NODE (.*)`).FindSubmatch(out)
	if m == nil {
		t.Fatalf("no statistics in %s", out)
	}
	goReused, _ := strconv.Atoi(string(m[1]))
	t.Logf("reused: engine %d, generated Go %d; engine node %v, Go node %s", engineReused, goReused, engineNode, m[2])
	if int(engineReused) <= goReused {
		t.Errorf("empty match at an insertion: engine reused %d, generated Go %d; want the engine to keep more", engineReused, goReused)
	}
	if engineNode.String() != string(m[2]) {
		t.Errorf("results differ: engine %v, Go %s", engineNode, m[2])
	}
}
