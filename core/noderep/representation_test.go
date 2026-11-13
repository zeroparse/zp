package noderep

import (
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var updateRepresentation = flag.Bool("update-representation", false, "rewrite testdata/representation.golden with the current representation")

// representationGolden records the protocol constant of the package and the representation it serves.
const representationGolden = "testdata/representation.golden"

// TestRepresentation fails when the node representation (every declaration of the package, exported
// or not: the layout of its types, the values of its constants and the signatures of its functions
// and methods) differs from the one recorded for the protocol constant the package declares, so that
// a change of the representation comes with a new constant. A copy of the library embedded in a
// generated parser refers to the constant it was written for, and only the new constant keeps an old
// copy from building against a representation it would misread. After replacing ProtocolN with the
// version of the library protocol that changes the representation (and the library's reference to
// it), go test -run TestRepresentation -update-representation records the new representation; it
// refuses to record a changed representation under the same constant.
func TestRepresentation(t *testing.T) {
	version, got := representation(t, ".")
	data, err := os.ReadFile(representationGolden)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	wantVersion, want := 0, ""
	if len(data) > 0 {
		first, rest, _ := strings.Cut(string(data), "\n")
		if wantVersion, err = strconv.Atoi(strings.TrimPrefix(first, "protocol ")); err != nil {
			t.Fatalf("%s: the first line must be \"protocol N\": %q", representationGolden, first)
		}
		want = rest
	}
	if version == wantVersion && got == want {
		return
	}
	changed := got != want
	if changed && version == wantVersion {
		t.Fatalf("the node representation changed without a new protocol constant: replace Protocol%d with the constant of the library protocol version that makes the change (and the library's reference to it), then run go test -run TestRepresentation -update-representation\n%s",
			version, linesDiff(want, got))
	}
	if !*updateRepresentation {
		t.Fatalf("%s records protocol %d, the package declares Protocol%d: run go test -run TestRepresentation -update-representation\n%s",
			representationGolden, wantVersion, version, linesDiff(want, got))
	}
	if err := os.WriteFile(representationGolden, fmt.Appendf(nil, "protocol %d\n%s", version, got), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRepresentationSeesLayout is the negative control of TestRepresentation: reordering the fields
// of Data or changing the value of a reserved type name, neither of which renames anything, changes
// the representation.
func TestRepresentationSeesLayout(t *testing.T) {
	_, want := representation(t, ".")
	for _, c := range []struct{ name, old, new string }{
		{"field order", "\tStart, End int32\n", "\tEnd, Start int32\n"},
		{"constant value", `TypeTuple   = "tuple"`, `TypeTuple   = "tuples"`},
	} {
		dir := t.TempDir()
		entries, err := os.ReadDir(".")
		if err != nil {
			t.Fatal(err)
		}
		replaced := false
		for _, e := range entries {
			n := e.Name()
			if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
				continue
			}
			data, err := os.ReadFile(n)
			if err != nil {
				t.Fatal(err)
			}
			if s := string(data); strings.Contains(s, c.old) {
				data, replaced = []byte(strings.Replace(s, c.old, c.new, 1)), true
			}
			if err := os.WriteFile(filepath.Join(dir, n), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if !replaced {
			t.Fatalf("%s: %q is not in the package", c.name, c.old)
		}
		if _, got := representation(t, dir); got == want {
			t.Errorf("%s: the change does not change the representation", c.name)
		}
	}
}

// protocolConst matches the name of the constant that declares the protocol version served.
var protocolConst = regexp.MustCompile(`^Protocol([1-9][0-9]*)$`)

// representation type-checks the package in dir and returns its protocol constant (exactly one is
// declared) and its representation: one line per declaration of the package scope but the protocol
// constant, and per method, sorted.
func representation(t *testing.T, dir string) (int, string) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("zeroparse.dev/zp/core/noderep", fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	qualifier := func(p *types.Package) string {
		if p == pkg {
			return ""
		}
		return p.Path()
	}
	var versions []int
	var lines []string
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if m := protocolConst.FindStringSubmatch(name); m != nil {
			if _, ok := obj.(*types.Const); ok {
				n, _ := strconv.Atoi(m[1])
				versions = append(versions, n)
				continue
			}
		}
		switch obj := obj.(type) {
		case *types.Const:
			lines = append(lines, fmt.Sprintf("const %s %s = %s", name, types.TypeString(obj.Type(), qualifier), obj.Val().ExactString()))
		case *types.Var:
			lines = append(lines, fmt.Sprintf("var %s %s", name, types.TypeString(obj.Type(), qualifier)))
		case *types.Func:
			lines = append(lines, "func "+name+strings.TrimPrefix(types.TypeString(obj.Type(), qualifier), "func"))
		case *types.TypeName:
			if obj.IsAlias() {
				lines = append(lines, fmt.Sprintf("type %s = %s", name, types.TypeString(obj.Type(), qualifier)))
				continue
			}
			named := obj.Type().(*types.Named)
			tparams := ""
			if tp := named.TypeParams(); tp.Len() > 0 {
				var ps []string
				for i := range tp.Len() {
					ps = append(ps, tp.At(i).Obj().Name()+" "+types.TypeString(tp.At(i).Constraint(), qualifier))
				}
				tparams = "[" + strings.Join(ps, ", ") + "]"
			}
			lines = append(lines, fmt.Sprintf("type %s%s %s", name, tparams, types.TypeString(named.Underlying(), qualifier)))
			for i := range named.NumMethods() {
				m := named.Method(i)
				lines = append(lines, fmt.Sprintf("method (%s) %s%s", types.TypeString(m.Signature().Recv().Type(), qualifier), m.Name(),
					strings.TrimPrefix(types.TypeString(m.Signature(), qualifier), "func")))
			}
		}
	}
	if len(versions) != 1 {
		t.Fatalf("the package declares the protocol constants %v, want exactly one", versions)
	}
	slices.Sort(lines)
	return versions[0], strings.Join(lines, "\n") + "\n"
}

// linesDiff lists the lines that only one of want and got has.
func linesDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	for _, l := range w {
		if !slices.Contains(g, l) {
			fmt.Fprintf(&b, "-%s\n", l)
		}
	}
	for _, l := range g {
		if !slices.Contains(w, l) {
			fmt.Fprintf(&b, "+%s\n", l)
		}
	}
	return b.String()
}
