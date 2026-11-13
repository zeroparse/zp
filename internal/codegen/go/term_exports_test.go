package gogen_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen/go"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestTermExportsGolden pins the identifiers of the library that exist only for the terms that
// generated Go parsers print from Exec IR (the functions and methods whose names start with Term, and the
// constants of the kinds of member reads, whose names start with Member): the
// library declares exactly those of testdata/term_exports.golden, and they are the ones the printer of
// lowered terms names, so that the library exports no composition that generated code does not call.
func TestTermExportsGolden(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "term_exports.golden"))
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			want = append(want, l)
		}
	}
	dir := filepath.Join(codegentest.ModuleRoot(), "codegen", "go", "lib")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.CONST {
					for _, sp := range gd.Specs {
						for _, n := range sp.(*ast.ValueSpec).Names {
							if strings.HasPrefix(n.Name, "Member") {
								got = append(got, "const "+n.Name)
							}
						}
					}
					continue
				}
				fd, ok := d.(*ast.FuncDecl)
				if !ok || !strings.HasPrefix(fd.Name.Name, "Term") {
					continue
				}
				name := fd.Name.Name
				if fd.Recv != nil {
					rt := fd.Recv.List[0].Type
					if s, ok := rt.(*ast.StarExpr); ok {
						rt = s.X
					}
					name = rt.(*ast.Ident).Name + "." + name
				}
				got = append(got, name)
			}
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("the library declares\n%s\nwant (testdata/term_exports.golden)\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var named []string
	for _, n := range gogen.LoweredTermNames() {
		named = append(named, n)
	}
	var methods []string
	for _, w := range want {
		if strings.HasPrefix(w, "const ") {
			continue
		}
		_, m, _ := strings.Cut(w, ".")
		if !slices.Contains(methods, m) {
			methods = append(methods, m)
		}
	}
	slices.Sort(named)
	slices.Sort(methods)
	if !slices.Equal(named, methods) {
		t.Errorf("the printer of lowered terms names %v, the golden %v", named, methods)
	}
}
