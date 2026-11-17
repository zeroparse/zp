package gogen_test

import (
	"bufio"
	"bytes"
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compiletest"
)

// TestUsedFeaturesKeepUserAPI checks that narrowing the library copy keeps every method that a user
// of the generated package can call: the method sets reached from the package's exported objects,
// through exported types of the package and of its library copy, must be the same with the
// optimization used-features on and off. A user reaches library methods the parser never calls, such
// as those of the library Document that the typed Document embeds.
func TestUsedFeaturesKeepUserAPI(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "grammars", "csv", "csv.zp"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := compiletest.Parse(string(src))
	if err != nil {
		t.Fatal(err)
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, c := range []struct {
		name string
		opts codegen.GenOptions
	}{
		{"plain", codegen.GenOptions{}},
		{"types-document", codegen.GenOptions{Types: true, Document: true}},
		{"stream", codegen.GenOptions{Stream: true}},
		{"types-stream", codegen.GenOptions{Types: true, Stream: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			surface := func(off bool) []string {
				o := c.opts
				o.Package, o.Start, o.Lib, o.Module = "p", "main", codegen.LibEmbed, "example.com/p"
				if off {
					o.NoOpt = []string{codegen.UsedFeatures}
				}
				files, err := codegentest.Generate(t, g, o)
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				codegentest.WriteGenerated(t, dir, o.Module, files)
				if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(codegentest.GoMod(o.Module)), 0o644); err != nil {
					t.Fatal(err)
				}
				return userMethods(t, goBin, dir, o.Module)
			}
			on, off := surface(false), surface(true)
			if missing := subtract(off, on); len(missing) > 0 {
				t.Errorf("the narrowed copy lacks user API that the whole library has:\n%s", strings.Join(missing, "\n"))
			}
		})
	}
}

// subtract returns the elements of a that b lacks.
func subtract(a, b []string) []string {
	var out []string
	for _, s := range a {
		if _, ok := slices.BinarySearch(b, s); !ok {
			out = append(out, s)
		}
	}
	return out
}

// userMethods builds the package pkg of the module in dir and returns, sorted, the methods of the
// exported types that a user reaches from the package's exported objects, through signatures, the
// exported fields of structs and the elements of composite types, as "type.method signature".
func userMethods(t *testing.T, goBin, dir, pkg string) []string {
	t.Helper()
	cmd := exec.Command(goBin, "list", "-export", "-deps", "-f", "{{.ImportPath}}={{.Export}}", pkg)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), codegentest.GoFlagsEnv, "GOTOOLCHAIN=local", "GOWORK=off")
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if e, ok := err.(*exec.ExitError); ok {
			stderr = e.Stderr
		}
		t.Fatalf("go list: %v\n%s", err, stderr)
	}
	exports := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if path, file, ok := strings.Cut(sc.Text(), "="); ok {
			exports[path] = file
		}
	}
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok || file == "" {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(file)
	})
	p, err := imp.Import(pkg)
	if err != nil {
		t.Fatal(err)
	}
	qual := func(q *types.Package) string { return q.Name() }
	seen := map[types.Type]bool{}
	var methods []string
	var visit func(types.Type)
	visit = func(typ types.Type) {
		if typ == nil || seen[typ] {
			return
		}
		seen[typ] = true
		switch u := typ.(type) {
		case *types.Named:
			if !u.Obj().Exported() {
				return
			}
			for _, recv := range []types.Type{u, types.NewPointer(u)} {
				ms := types.NewMethodSet(recv)
				for i := range ms.Len() {
					m := ms.At(i).Obj()
					if !m.Exported() {
						continue
					}
					methods = append(methods, types.TypeString(recv, qual)+"."+m.Name()+" "+types.TypeString(m.Type(), qual))
					visit(m.Type())
				}
			}
			visit(u.Underlying())
		case *types.Alias:
			visit(types.Unalias(u))
		case *types.Pointer:
			visit(u.Elem())
		case *types.Slice:
			visit(u.Elem())
		case *types.Array:
			visit(u.Elem())
		case *types.Map:
			visit(u.Key())
			visit(u.Elem())
		case *types.Chan:
			visit(u.Elem())
		case *types.Signature:
			for v := range u.Params().Variables() {
				visit(v.Type())
			}
			for v := range u.Results().Variables() {
				visit(v.Type())
			}
		case *types.Struct:
			for f := range u.Fields() {
				if f.Exported() || f.Embedded() {
					visit(f.Type())
				}
			}
		case *types.Interface:
			for m := range u.Methods() {
				visit(m.Type())
			}
		}
	}
	scope := p.Scope()
	for _, name := range scope.Names() {
		if obj := scope.Lookup(name); obj.Exported() {
			visit(obj.Type())
		}
	}
	slices.Sort(methods)
	return slices.Compact(methods)
}
