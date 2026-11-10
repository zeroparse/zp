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
	var out []string
	for _, r := range specRows(t, file, head) {
		out = append(out, r[0])
	}
	return out
}

// specRows returns the rows of the tables of the chapter file whose first header cell is head, with
// each cell trimmed and a cell that is one code span replaced by its content.
func specRows(t *testing.T, file, head string) [][]string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(specDir, file))
	if err != nil {
		t.Fatal(err)
	}
	var out [][]string
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
		for i := range cells {
			cells[i] = strings.Trim(strings.TrimSpace(cells[i]), "`")
		}
		first := cells[0]
		switch {
		case strings.HasPrefix(line, "|:") || strings.HasPrefix(line, "|-"):
		case !table:
			// The header row: the rows that follow count only under the header head.
			table, in = true, first == head
		case in:
			out = append(out, cells)
		}
	}
	return out
}

// kindErrors returns the differences between the names that the code declares, want, and those that
// the specification lists, got.
func kindErrors(kind string, want, got []string) []string {
	var errs []string
	for i, w := range want {
		if slices.Contains(want[:i], w) {
			errs = append(errs, fmt.Sprintf("%s %s: named twice by the code, so one has no name of its own", kind, w))
		}
	}
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

// packageFiles returns the syntax of the files of this package that are not tests.
func packageFiles(t *testing.T) []*ast.File {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil || len(names) == 0 {
		t.Fatalf("no files in the package: %v", err)
	}
	var out []*ast.File
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), n, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, f)
	}
	return out
}

// kinds returns the names of the types of this package that implement the interface method method:
// the statements (stmt) or the expressions (expr), declared in any file of the package.
func kinds(t *testing.T, method string) []string {
	t.Helper()
	var out []string
	for _, f := range packageFiles(t) {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != method || len(fn.Recv.List) != 1 || fn.Type.Params.NumFields() != 0 || fn.Type.Results != nil {
				// Not the marker method of a kind: a method of another name or with parameters or results.
				continue
			}
			switch x := fn.Recv.List[0].Type.(type) {
			case *ast.StarExpr:
				out = append(out, x.X.(*ast.Ident).Name)
			case *ast.Ident:
				out = append(out, x.Name)
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("no %s methods in the package", method)
	}
	return out
}

// fields returns the names of the fields of the struct types of this package, in order.
func fields(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, f := range packageFiles(t) {
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				out[ts.Name.Name] = nil
				for _, fl := range st.Fields.List {
					for _, id := range fl.Names {
						out[ts.Name.Name] = append(out[ts.Name.Name], id.Name)
					}
				}
			}
			return true
		})
	}
	return out
}

// registers returns the number of registers: the constants of type Reg that the package declares.
func registers(t *testing.T) int {
	t.Helper()
	n := 0
	for _, f := range packageFiles(t) {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			typed := false
			for i, sp := range gd.Specs {
				vs := sp.(*ast.ValueSpec)
				if id, ok := vs.Type.(*ast.Ident); ok {
					typed = id.Name == "Reg"
				} else if i == 0 || vs.Type != nil {
					typed = false
				}
				if typed {
					n += len(vs.Names)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no registers in the package")
	}
	return n
}

// specKinds is what the specification is checked against.
type specKinds struct{ stmts, exprs, types, regs []string }

func currentKinds(t *testing.T) specKinds {
	k := specKinds{stmts: kinds(t, "stmt"), exprs: kinds(t, "expr")}
	for i := range typeNames {
		k.types = append(k.types, Type(i).String())
	}
	p := &printer{}
	for r := Reg(0); int(r) < registers(t); r++ {
		k.regs = append(k.regs, p.expr(&Read{R: r}))
	}
	return k
}

// fieldErrors returns the differences between the fields that the statements and the expressions
// declare and those that the Fields column of the specification lists for them, in order.
func fieldErrors(t *testing.T, declared map[string][]string) []string {
	var errs []string
	for _, c := range []struct{ file, head string }{{"statements.md", "Statement"}, {"expressions.md", "Expression"}} {
		for _, r := range specRows(t, c.file, c.head) {
			if len(r) < 2 {
				errs = append(errs, fmt.Sprintf("%s: row without a Fields column", r[0]))
				continue
			}
			want, ok := declared[r[0]]
			if !ok {
				continue // reported by kindErrors
			}
			var got []string
			for _, f := range strings.Split(r[1], ",") {
				if f = strings.TrimSpace(f); f != "" {
					got = append(got, f)
				}
			}
			if !slices.Equal(got, want) {
				errs = append(errs, fmt.Sprintf("%s: the specification lists the fields %v, the code declares %v", r[0], got, want))
			}
		}
	}
	return errs
}

func specErrors(t *testing.T, k specKinds) []string {
	var errs []string
	errs = append(errs, kindErrors("statement", k.stmts, specColumn(t, "statements.md", "Statement"))...)
	errs = append(errs, kindErrors("expression", k.exprs, specColumn(t, "expressions.md", "Expression"))...)
	errs = append(errs, kindErrors("type", k.types, specColumn(t, "functions.md", "Type"))...)
	errs = append(errs, kindErrors("register", k.regs, specColumn(t, "functions.md", "Register"))...)
	errs = append(errs, fieldErrors(t, fields(t))...)
	return errs
}

// TestSpecKinds checks the statements, expressions, types and registers of the specification.
func TestSpecKinds(t *testing.T) {
	for _, e := range specErrors(t, currentKinds(t)) {
		t.Error(e)
	}
}

// TestSpecFieldsNegative is the permanent negative control of the fields check: a field renamed in
// the code, a field that the specification lacks and one that the code lacks are each reported.
func TestSpecFieldsNegative(t *testing.T) {
	d := fields(t)
	d["Advance"] = []string{"Count"}
	d["Block"] = append(slices.Clone(d["Block"]), "Extra")
	d["Inc"] = nil
	errs := strings.Join(fieldErrors(t, d), "\n")
	for _, want := range []string{"Advance: the specification lists the fields [N], the code declares [Count]", "Block: ", "Inc: "} {
		if !strings.Contains(errs, want) {
			t.Errorf("no error %q in:\n%s", want, errs)
		}
	}
}

// TestSpecNamesNegative is the permanent negative control for a register that the printer does not
// name: it prints as another register, which kindErrors reports.
func TestSpecNamesNegative(t *testing.T) {
	errs := strings.Join(kindErrors("register", []string{"pos", "env", "pos"}, []string{"pos", "env"}), "\n")
	if !strings.Contains(errs, "register pos: named twice by the code") {
		t.Errorf("no error for the duplicate name in:\n%s", errs)
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
