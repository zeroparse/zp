package codegen_test

import (
	"bytes"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/codegen"
	"zeroparse.dev/zp/engine"
	igen "zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestMain(m *testing.M) {
	// The tests build generated programs; a build cache private to the test process keeps their
	// entries out of the shared cache.
	removeGoCache := codegentest.SetGoCache()
	code := m.Run()
	removeGoCache()
	os.Exit(code)
}

// protocolName matches the declaration of a protocol constant of the library or of core/noderep.
var protocolName = regexp.MustCompile(`\bconst Protocol([1-9][0-9]*) = ([1-9][0-9]*)\b`)

// bumpProtocol replaces the protocol constants that src declares with those of the version after
// them (by = 1) or before them (by = -1).
func bumpProtocol(src []byte, by int) []byte {
	return protocolName.ReplaceAllFunc(src, func(m []byte) []byte {
		n, _ := strconv.Atoi(string(protocolName.FindSubmatch(m)[1]))
		return []byte("const Protocol" + strconv.Itoa(n+by) + " = " + strconv.Itoa(n+by))
	})
}

// protocolBuild writes a module zeroparse.dev/zp in dir/zp that holds this module's core and
// core/noderep, with noderep's source passed through noderep, and the files libFiles at their paths
// below lib (the directory of the library in it, or "" to leave it out), and the parser files in
// dir/p, a module that requires it; then it builds the parser.
func protocolBuild(t *testing.T, files, libFiles []codegen.File, lib string, noderep func([]byte) []byte) (string, error) {
	t.Helper()
	dir := t.TempDir()
	write := func(rel string, data []byte) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		write("p/"+f.Path, f.Content)
	}
	write("zp/go.mod", []byte("module zeroparse.dev/zp\n\ngo 1.27.1\n"))
	for _, pkg := range []string{"core", "core/noderep"} {
		src := filepath.Join(codegentest.ModuleRoot(), filepath.FromSlash(pkg))
		entries, err := os.ReadDir(src)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(src, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if pkg == "core/noderep" && noderep != nil {
				data = noderep(data)
			}
			write("zp/"+pkg+"/"+e.Name(), data)
		}
	}
	for _, f := range libFiles {
		write(lib+"/"+path.Base(f.Path), f.Content)
	}
	write("p/go.mod", []byte("module example.com/proto\n\ngo 1.27.1\n\nrequire zeroparse.dev/zp v0.0.0\n\nreplace zeroparse.dev/zp => ../zp\n"))
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = filepath.Join(dir, "p")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod -trimpath", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// libProtocol returns the name of the newest protocol constant that the library files declare.
func libProtocol(t *testing.T, libFiles []codegen.File) string {
	t.Helper()
	best := 0
	for _, f := range libFiles {
		for _, m := range protocolName.FindAllSubmatch(f.Content, -1) {
			if n, _ := strconv.Atoi(string(m[1])); n > best {
				best = n
			}
		}
	}
	if best == 0 {
		t.Fatal("the library declares no protocol constant")
	}
	return "Protocol" + strconv.Itoa(best)
}

// TestImportProtocol builds a parser generated with LibImport against the library that the
// generator carries, which builds, and against the same library declaring the next protocol
// version instead, which must fail to build on the protocol constant the parser refers to.
func TestImportProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a generated parser")
	}
	g, err := engine.ParseGrammar(`@start
rule main = [a-z]+ "=" [0-9]+`)
	if err != nil {
		t.Fatal(err)
	}
	files, err := codegen.Go{Module: "example.com/proto", Lib: codegen.LibImport}.Generate(g, "main")
	if err != nil {
		t.Fatal(err)
	}
	libFiles, err := codegen.Go{Module: "example.com/proto"}.GenerateLib()
	if err != nil {
		t.Fatal(err)
	}
	protocol := libProtocol(t, libFiles)
	var parser []byte
	for _, f := range files {
		if f.Path == "parser.go" {
			parser = f.Content
		}
	}
	if want := "const _ = rt." + protocol + "\n"; !bytes.Contains(parser, []byte(want)) {
		t.Fatalf("parser.go does not refer to the protocol with %q", want)
	}
	const lib = "zp/codegen/go/lib"
	if out, err := protocolBuild(t, files, libFiles, lib, nil); err != nil {
		t.Fatalf("the parser does not build against the library of its generator: %v\n%s", err, out)
	}
	bumped := make([]codegen.File, len(libFiles))
	for i, f := range libFiles {
		bumped[i] = codegen.File{Path: f.Path, Content: bumpProtocol(f.Content, 1)}
	}
	out, err := protocolBuild(t, files, bumped, lib, nil)
	if err == nil {
		t.Fatal("the parser builds against a library that does not implement its protocol")
	}
	if !strings.Contains(out, "undefined: rt."+protocol) {
		t.Errorf("the build against another protocol fails for another reason than the protocol constant:\n%s", out)
	}
}

// TestEmbedCoreProtocol builds a parser generated with LibEmbed, whose copy of the library imports
// core and core/noderep from zeroparse.dev/zp, against this module's core, which builds, and against
// a core whose noderep serves the representation of the protocol version after or before the
// library's, which must fail to build on the noderep constant the copy refers to.
func TestEmbedCoreProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a generated parser")
	}
	g, err := engine.ParseGrammar(`@start
rule main = [a-z]+ "=" [0-9]+`)
	if err != nil {
		t.Fatal(err)
	}
	files, err := codegen.Go{Module: "example.com/proto"}.Generate(g, "main")
	if err != nil {
		t.Fatal(err)
	}
	var lib []codegen.File
	for _, f := range files {
		if strings.HasPrefix(f.Path, "internal/libgo/") {
			lib = append(lib, f)
		}
	}
	if len(lib) == 0 {
		t.Fatal("embed mode wrote no copy of the library")
	}
	if out, err := protocolBuild(t, files, nil, "", nil); err != nil {
		t.Fatalf("the parser does not build against the core of its generator: %v\n%s", err, out)
	}
	src, err := os.ReadFile(filepath.Join(codegentest.ModuleRoot(), "core", "noderep", "noderep.go"))
	if err != nil {
		t.Fatal(err)
	}
	m := protocolName.FindSubmatch(src)
	if m == nil {
		t.Fatal("core/noderep declares no protocol constant")
	}
	want := "undefined: noderep.Protocol" + string(m[1])
	for _, by := range []int{1, -1} {
		out, err := protocolBuild(t, files, nil, "", func(src []byte) []byte { return bumpProtocol(src, by) })
		if err == nil {
			t.Fatalf("the copy of the library builds against a core/noderep of protocol %+d", by)
		}
		if !strings.Contains(out, want) {
			t.Errorf("protocol %+d: the build fails for another reason than the noderep constant:\n%s", by, out)
		}
	}
}

// goGenerated matches the line that marks a generated Go file.
var goGenerated = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// TestGeneratedHeader checks that every generated Go and TypeScript file starts with the header that
// names the generator version, and that generating twice gives the same files.
func TestGeneratedHeader(t *testing.T) {
	g, err := engine.ParseGrammar(`type Pair = { Key node[string]; Value node[string] }
@start
rule main Pair = k:[a-z]+ #atomic "=" v:[0-9]+ #atomic => Pair{Key: $k, Value: $v}`)
	if err != nil {
		t.Fatal(err)
	}
	header := igen.Header()
	if !goGenerated.MatchString(header) || !strings.Contains(header, " zp "+igen.Version()+".") {
		t.Fatalf("header %q", header)
	}
	for _, generator := range []codegen.Generator{
		codegen.Go{Module: "example.com/h", Types: true},
		codegen.Go{Module: "example.com/h", Types: true, Lib: codegen.LibImport},
		codegen.Go{Module: "example.com/h", Types: true, TypedOnly: true, Recognize: true},
		codegen.Go{Module: "example.com/h", RecognizeOnly: true},
		codegen.TypeScript{Types: true},
		codegen.TypeScript{Types: true, Lib: codegen.LibImport, LibModule: "./rt.ts"},
	} {
		first, err := generator.Generate(g, "main")
		if err != nil {
			t.Fatal(err)
		}
		second, err := generator.Generate(g, "main")
		if err != nil {
			t.Fatal(err)
		}
		if len(first) != len(second) {
			t.Fatalf("%#v: %d files, then %d", generator, len(first), len(second))
		}
		for i, f := range first {
			if line, _, _ := strings.Cut(string(f.Content), "\n"); line != header {
				t.Errorf("%#v: %s starts with %q, want %q", generator, f.Path, line, header)
			}
			if f.Path != second[i].Path || !bytes.Equal(f.Content, second[i].Content) {
				t.Errorf("%#v: %s differs between two generations", generator, f.Path)
			}
		}
	}
}
