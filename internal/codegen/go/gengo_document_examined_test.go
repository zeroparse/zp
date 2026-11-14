package gogen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// The examined end, the evaluations and the reuse that a Document reports for a rule that fails
// after its callee matched must be the same for the typed route and the Node route, and the end must
// be exact: a direct rule without captures is generated as one body for its call and its
// invocation, whose rollback on failure changes nothing that the Document records.
func TestGeneratedDocumentExaminedAfterFailure(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedDocumentExaminedAfterFailure)
}

func testGeneratedDocumentExaminedAfterFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("builds standalone generated Document consumers")
	}
	const grammar = `var x int
rule main = part "b" / part / "a" [a-z]
rule part = leaf "a" {x == 1}
rule leaf = &"a"
`
	g, err := parseTest(grammar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", codegentest.GoMod("examined"))
	const consumer = `package PACKAGE

import (
	"fmt"
	"strings"
	"testing"

	zprt "zeroparse.dev/zp/codegen/go/lib"
)

func TestExamined(t *testing.T) {
	d, err := NewDocument("ab")
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	for i := 0; i < 2; i++ {
		_, _ = d.PARSE(zprt.WithTrace(func(e zprt.TraceEvent) {
			if e.Kind == zprt.TraceExit {
				fmt.Fprintf(&out, "exit %s pos=%d memo=%v evals=%d examined=%d\n", e.Rule, e.Pos, e.Memo, e.Evals, e.Examined)
			}
		}))
		fmt.Fprintf(&out, "stats %+v\n", d.Stats())
		if i == 0 {
			if err := d.Edit(1, 2, "c"); err != nil {
				t.Fatal(err)
			}
		}
	}
	fmt.Print("RESULT\n", out.String(), "END\n")
}
`
	results := map[string]string{}
	for _, route := range []string{"node", "typed"} {
		code, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Package: route, Start: "main", Document: true, Types: route == "typed", Recognize: true, Observability: true})
		if err != nil {
			t.Fatal(route, err)
		}
		write(route+"/parser.go", string(code))
		parse := "Parse"
		if route == "typed" {
			parse = "ParseValue"
		}
		write(route+"/examined_test.go", strings.NewReplacer("PACKAGE", route, "PARSE", parse).Replace(consumer))
		cmd := exec.Command("go", "test", "-trimpath", "-count=1", "-v", "./"+route)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", route, err, out)
		}
		_, b, ok := strings.Cut(string(out), "RESULT\n")
		if !ok {
			t.Fatalf("%s: no result\n%s", route, out)
		}
		results[route], _, _ = strings.Cut(b, "END\n")
	}
	if results["node"] != results["typed"] {
		t.Errorf("the typed route reports differently from the Node route\nnode:\n%s\ntyped:\n%s", results["node"], results["typed"])
	}
	// The failing part examines the unit of its leaf and its "a" and nothing after: its predicate
	// fails at 1 without reading the unit there. Its examined end is exact, so the edit of [1, 2)
	// keeps its entry, and the second parse reuses it twice and evaluates main alone.
	if !strings.Contains(results["typed"], "exit part pos=0 memo=false evals=1 examined=1") {
		t.Errorf("part does not report the exact end of what it examined:\n%s", results["typed"])
	}
	if !strings.Contains(results["typed"], "stats {Evaluated:1 Reused:2 MemoHits:2 ResumedElements:0}") {
		t.Errorf("the parse after the edit does not reuse part:\n%s", results["typed"])
	}
}
