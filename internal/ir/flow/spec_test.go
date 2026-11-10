package flow

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The tests of this file hold the Flow IR specification (spec/flow-ir) current with this package:
// every statement and expression kind, type and register has an entry in the specification, and
// every entry names one that Flow IR has. They live beside the code, so that a change of Flow IR runs
// them; the site's tests run only when documentation changes.

// specDir is the directory of the Flow IR specification, relative to this package.
const specDir = "../../../spec/flow-ir"

// specColumn returns the first cells of the rows of the tables of the chapter file whose first header
// cell is head, with a cell that is one code span replaced by its content.
func specColumn(t *testing.T, file, head string) []string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(specDir, file))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	table, in, fenced := false, false, false
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		if fenced || !strings.HasPrefix(line, "|") {
			table, in = false, false
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		first := strings.Trim(strings.TrimSpace(cells[0]), "`")
		switch {
		case strings.HasPrefix(line, "|:") || strings.HasPrefix(line, "|-"):
		case !table:
			// The header row: the rows that follow count only under the header head.
			table, in = true, first == head
		case in:
			out = append(out, first)
		}
	}
	return out
}

// kindErrors returns the differences between the names that the code declares, want, and those that
// the specification lists, got.
func kindErrors(kind string, want, got []string) []string {
	var errs []string
	seen := map[string]bool{}
	for _, g := range got {
		if seen[g] {
			errs = append(errs, fmt.Sprintf("%s %s: listed twice in the specification", kind, g))
		}
		seen[g] = true
		if !slices.Contains(want, g) {
			errs = append(errs, fmt.Sprintf("%s %s: in the specification but not in Flow IR", kind, g))
		}
	}
	for _, w := range want {
		if !seen[w] {
			errs = append(errs, fmt.Sprintf("%s %s: in Flow IR but not in the specification", kind, w))
		}
	}
	return errs
}

// kinds returns the names of the types of this package that implement the interface method method:
// the statements (stmt) or the expressions (expr).
func kinds(t *testing.T, method string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "flow.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != method || len(fn.Recv.List) != 1 {
			continue
		}
		switch x := fn.Recv.List[0].Type.(type) {
		case *ast.StarExpr:
			out = append(out, x.X.(*ast.Ident).Name)
		case *ast.Ident:
			out = append(out, x.Name)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no %s methods in flow.go", method)
	}
	return out
}

// specKinds is what the specification is checked against.
type specKinds struct{ stmts, exprs, types, regs []string }

func currentKinds(t *testing.T) specKinds {
	k := specKinds{stmts: kinds(t, "stmt"), exprs: kinds(t, "expr")}
	for i := range typeNames {
		k.types = append(k.types, Type(i).String())
	}
	p := &printer{}
	for r := RegPos; r <= RegCut; r++ {
		k.regs = append(k.regs, p.expr(&Read{R: r}))
	}
	return k
}

func specErrors(t *testing.T, k specKinds) []string {
	var errs []string
	errs = append(errs, kindErrors("statement", k.stmts, specColumn(t, "statements.md", "Statement"))...)
	errs = append(errs, kindErrors("expression", k.exprs, specColumn(t, "expressions.md", "Expression"))...)
	errs = append(errs, kindErrors("type", k.types, specColumn(t, "functions.md", "Type"))...)
	errs = append(errs, kindErrors("register", k.regs, specColumn(t, "functions.md", "Register"))...)
	return errs
}

// TestSpecKinds checks the statements, expressions, types and registers of the specification.
func TestSpecKinds(t *testing.T) {
	for _, e := range specErrors(t, currentKinds(t)) {
		t.Error(e)
	}
}

// TestSpecKindsNegative is the permanent negative control of TestSpecKinds: a kind that the
// specification does not list and a kind that Flow IR no longer has are each reported by name.
func TestSpecKindsNegative(t *testing.T) {
	k := currentKinds(t)
	k.stmts = append(k.stmts, "PlantedStmt")
	k.exprs = slices.DeleteFunc(k.exprs, func(s string) bool { return s == "MatchLit" })
	k.regs = append(k.regs, "planted")
	errs := strings.Join(specErrors(t, k), "\n")
	for _, want := range []string{
		"statement PlantedStmt: in Flow IR but not in the specification",
		"expression MatchLit: in the specification but not in Flow IR",
		"register planted: in Flow IR but not in the specification",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("no error %q in:\n%s", want, errs)
		}
	}
}
