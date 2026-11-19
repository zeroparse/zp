package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The tests name repository files and packages in string literals: the sources that mutant
// runners rewrite, the grammars and goldens they read, and the import paths of the code they
// generate and build. A move that misses such a literal breaks only the test that holds it, and
// often only in the long tier or under a build tag that a targeted run does not build. So this
// test reads the literals of every test file of the module (whatever its build tags) and of test
// support (internal/testing/**), and fails on each literal that names a path of the repository or
// a package of the module that does not exist. Directories with a go.mod of their own (the site)
// are other modules, whose tests are not read.
//
// A literal names a repository path when its first element, after any leading ./ and ../, is a
// top-level directory of the repository, present or removed (repoTopDirs), or when it starts
// with two or more ../; the path must exist relative to the file's directory, where go test runs
// the test, or to the repository root. Import paths of the file itself are not paths. A call of
// filepath.Join or path.Join whose first argument is a literal starting with .., or the module
// root (moduleRoot, ModuleRoot(), RepoRoot()), names the path its leading literal arguments join
// to. A literal names a package of the module wherever it holds zeroparse.dev/zp/<path>, as the
// import blocks of generated code do; the directory must hold Go files.

// repoTopDirs are the top-level directories that a repository-relative path starts with: the
// present ones and those that moves removed, whose stale paths are what the test looks for.
var repoTopDirs = []string{
	"bench", "cmd", "docs", "editors", "examples", "hack", "internal", "pkg", "site", "spec", "testdata",
	"codegen", "core", "editfuzz", "engine", "gen", "grammar", "lint", "runtime", "sample",
}

// generatedLayout are the paths of the layout of a generated Go module, which the tests name
// relative to the module they generate, not to the repository.
var generatedLayout = []string{"internal/libgo"}

// pathAllowlist holds the literals that deliberately name no path of the repository, as
// "<file>: <literal>" with the file relative to the repository root. The test fails on a listed
// literal that it no longer finds, so the list only holds what is still there.
var pathAllowlist = []string{
	// Packages of the synthetic import graphs of the negative controls.
	"internal/testing/archtest/archtest_test.go: internal/fakeaction",
	"internal/testing/archtest/archtest_test.go: internal/fakevm",
	"internal/testing/archtest/archtest_test.go: internal/unplaced",
	// Changed-file lists of the queue scripts' tests.
	"internal/testing/archtest/scripts_test.go: internal/runtime/engine/x.go",
	"internal/testing/archtest/scripts_test.go: docs/a.md",
	"internal/testing/archtest/scripts_test.go: docs/moved.md",
	// The missing sources that the mutant helper must reject.
	"internal/testing/mutanttest/mutanttest_test.go: internal/testing/mutanttest/no_such_file.go",
	"internal/testing/mutanttest/mutanttest_test.go: internal/no/such/package/call_exec.go",
	// A prefix that the test completes with the name of a case.
	"internal/backend/codegen/common/gents_typed_replay_test.go: testdata/typed/splice_recursive_",
	// The display name of an example grammar case, which nothing opens.
	"internal/testing/codegentest/gengo.go: ../../examples/",
}

// pathScanSelf is this file, whose negative control holds stale paths on purpose.
const pathScanSelf = "internal/testing/archtest/paths_test.go"

// A pathFinding is a literal that names a path or package that does not exist.
type pathFinding struct {
	file    string // relative to the root
	line    int
	literal string
	what    string // "path" or "package"
}

func (f pathFinding) key() string { return f.file + ": " + f.literal }

func (f pathFinding) String() string {
	return fmt.Sprintf("%s:%d: %q names a %s that does not exist", f.file, f.line, f.literal, f.what)
}

// pathScanFiles lists the files the test reads under root: every _test.go file and the Go files
// of internal/testing, outside hidden directories, node_modules and other modules.
func pathScanFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == pathScanSelf {
			return nil
		}
		if strings.HasSuffix(name, "_test.go") || strings.HasPrefix(rel, "internal/testing/") && strings.HasSuffix(name, ".go") {
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

var (
	modPathRE = regexp.MustCompile(`zeroparse\.dev/zp((?:/[A-Za-z0-9_.-]+)*)`)
	// declRE is the name of a declaration that follows a package path, as in impl.Kind.
	declRE    = regexp.MustCompile(`\.[A-Z][A-Za-z0-9_]*$`)
	pathLitRE = regexp.MustCompile(`^(?:\.\.?/)*[A-Za-z0-9_.@-]+(?:/[A-Za-z0-9_.@-]+)*/?$`)
)

// scanPaths returns the findings of the files under root that the allowlist does not hold, in
// file and line order, and the allowlist entries and generated layout paths it found no literal
// for.
func scanPaths(root string, allow, layout []string) (findings []pathFinding, unused []string, err error) {
	files, err := pathScanFiles(root)
	if err != nil {
		return nil, nil, err
	}
	used := map[string]bool{}
	allowed := map[string]bool{}
	for _, a := range allow {
		allowed[a] = true
	}
	fset := token.NewFileSet()
	for _, rel := range files {
		f, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, nil, err
		}
		dir := filepath.Join(root, filepath.Dir(rel))
		report := func(pos token.Pos, lit, what string) {
			x := pathFinding{file: rel, line: fset.Position(pos).Line, literal: lit, what: what}
			if allowed[x.key()] {
				used[x.key()] = true
				return
			}
			findings = append(findings, x)
		}
		skip := map[*ast.BasicLit]bool{}
		for _, im := range f.Imports {
			skip[im.Path] = true
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				p, atRoot, ok := joinPath(n, skip)
				base := dir
				if atRoot {
					base = root
				}
				if ok && !existsUnder(p, base, root) {
					report(n.Pos(), p, "path")
				}
			case *ast.BasicLit:
				if n.Kind != token.STRING {
					return true
				}
				s, err := strconv.Unquote(n.Value)
				if err != nil {
					return true
				}
				for _, m := range modPathRE.FindAllStringSubmatch(s, -1) {
					lit := declRE.ReplaceAllString(m[0], "")
					if !packageExists(root, strings.TrimSuffix(strings.TrimPrefix(lit, "zeroparse.dev/zp"), "/...")) {
						report(n.Pos(), lit, "package")
					}
				}
				if skip[n] || !repoPath(s) {
					return true
				}
				if i := slices.IndexFunc(layout, func(l string) bool { return s == l || strings.HasPrefix(s, l+"/") }); i >= 0 {
					used["layout: "+layout[i]] = true
					return true
				}
				if !existsUnder(s, dir, root) {
					report(n.Pos(), s, "path")
				}
			}
			return true
		})
	}
	for _, a := range allow {
		if !used[a] {
			unused = append(unused, a)
		}
	}
	for _, l := range layout {
		if !used["layout: "+l] {
			unused = append(unused, "layout: "+l)
		}
	}
	return findings, unused, nil
}

// repoPath reports whether a literal names a path of the repository.
func repoPath(s string) bool {
	if !pathLitRE.MatchString(s) || s == "." || s == ".." {
		return false
	}
	rest, up, prefixed := s, 0, false
	for {
		if r, ok := strings.CutPrefix(rest, "../"); ok {
			rest, up, prefixed = r, up+1, true
		} else if r, ok := strings.CutPrefix(rest, "./"); ok {
			rest, prefixed = r, true
		} else {
			break
		}
	}
	if up >= 2 {
		return true
	}
	first, _, slash := strings.Cut(rest, "/")
	return (slash || prefixed) && slices.Contains(repoTopDirs, first)
}

// joinPath returns the path that a filepath.Join or path.Join call names with its leading string
// literals, marking those literals in joined, and whether it is relative to the module root; ok
// is false for another call, or one whose base is neither a literal starting with .. nor the
// module root.
func joinPath(c *ast.CallExpr, joined map[*ast.BasicLit]bool) (p string, atRoot, ok bool) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Join" || len(c.Args) == 0 {
		return "", false, false
	}
	if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "filepath" && x.Name != "path" {
		return "", false, false
	}
	args := c.Args
	var parts []string
	if isModuleRoot(args[0]) {
		atRoot = true
		args = args[1:]
	}
	var lits []*ast.BasicLit
	for _, a := range args {
		lit, ok := a.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			break
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil || strings.ContainsAny(s, "%*?{$ \n") {
			return "", false, false
		}
		parts = append(parts, s)
		lits = append(lits, lit)
	}
	if len(lits) == 0 || !atRoot && !strings.HasPrefix(parts[0], "..") {
		return "", false, false
	}
	for _, l := range lits {
		joined[l] = true
	}
	return strings.Join(parts, "/"), atRoot, true
}

// isModuleRoot reports whether an expression is the module root that the tests join paths to.
func isModuleRoot(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name == "moduleRoot" || e.Name == "repoRoot"
	case *ast.CallExpr:
		var name string
		switch f := e.Fun.(type) {
		case *ast.Ident:
			name = f.Name
		case *ast.SelectorExpr:
			name = f.Sel.Name
		}
		return len(e.Args) == 0 && (name == "ModuleRoot" || name == "moduleRoot" || name == "RepoRoot" || name == "repoRoot")
	}
	return false
}

// existsUnder reports whether p exists relative to dir or to root.
func existsUnder(p, dir, root string) bool {
	p = strings.TrimSuffix(strings.TrimSuffix(p, "/..."), "/")
	for _, base := range []string{dir, root} {
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(p))); err == nil {
			return true
		}
	}
	return false
}

// packageExists reports whether the directory of a package of the module, given by its path
// below the module, holds Go files.
func packageExists(root, sub string) bool {
	if sub == "" {
		return true
	}
	m, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(sub), "*.go"))
	return len(m) > 0
}

// TestTestPathsExist checks that the literals of the tests name existing paths and packages.
func TestTestPathsExist(t *testing.T) {
	findings, unused, err := scanPaths(moduleRoot, pathAllowlist, generatedLayout)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		t.Error(f)
	}
	for _, a := range unused {
		t.Errorf("the path allowlist or the generated layout holds %q, which no file has: remove it", a)
	}
}

// TestTestPathsNegativeControl runs the scan over a small repository with a stale path of each
// form and checks that it reports exactly those, under a build tag and in test support too, and
// the allowlist entry and layout path that match nothing.
func TestTestPathsNegativeControl(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pkg/runtime/golib/lib.go", "package golib\n")
	write("internal/runtime/state/resume.go", "package state\n")
	write("internal/runtime/engine/testdata/a.zp", "")
	write("internal/runtime/engine/x_long_test.go", `//go:build zp_long

package engine

import (
	"path/filepath"
	"runtime/debug"
)

var (
	ok1 = "internal/runtime/state/resume.go"
	ok2 = "../state/resume.go"
	ok3 = filepath.Join("testdata", "a.zp")
	ok4 = filepath.Join("..", "..", "..", "pkg", "runtime", "golib", name)
	ok5 = filepath.Join(moduleRoot, "pkg", "runtime", "golib")
	ok6 = "import rt \"zeroparse.dev/zp/pkg/runtime/golib\"; rt.Kind"
	ok7 = "zeroparse.dev/zp/pkg/runtime/golib.Kind"
	ok8 = "./lib.ts"
	ok9 = "internal/libgo/lib.go"
	ok10 = "../rt.js"
	bad1 = "../../../../testdata/grammars/x.zp"
	bad2 = filepath.Join("..", "..", "..", "core", "impl")
	bad3 = filepath.Join(moduleRoot, "codegen", "go", "lib")
	bad4 = `+"`"+`import rt "zeroparse.dev/zp/codegen/go/lib"`+"`"+`
	bad5 = "codegen/ts/lib/trace.ts"
	bad6 = "./engine"
	allowed = "internal/fake"
)
`)
	write("internal/testing/support/support.go", "package support\n\nvar bad = \"../../../../pkg/sample\"\n")
	write("internal/runtime/engine/plain.go", "package engine\n\nvar notRead = \"codegen/x.go\"\n")
	write("other/go.mod", "module example.org/other\n")
	write("other/x_test.go", "package other\n\nvar notRead = \"codegen/x.go\"\n")
	allow := []string{"internal/runtime/engine/x_long_test.go: internal/fake", "internal/runtime/engine/x_long_test.go: gone"}
	findings, unused, err := scanPaths(root, allow, []string{"internal/libgo", "internal/unused"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range findings {
		got = append(got, f.file+": "+f.literal)
	}
	const x = "internal/runtime/engine/x_long_test.go: "
	want := []string{x + "../../../../testdata/grammars/x.zp", x + "../../../core/impl", x + "codegen/go/lib",
		x + "zeroparse.dev/zp/codegen/go/lib", x + "codegen/ts/lib/trace.ts", x + "./engine",
		"internal/testing/support/support.go: ../../../../pkg/sample"}
	if !slices.Equal(got, want) {
		t.Errorf("findings\n%q\nwant\n%q", got, want)
	}
	if want := []string{x + "gone", "layout: internal/unused"}; !slices.Equal(unused, want) {
		t.Errorf("unused entries %q, want %q", unused, want)
	}
}
