package archtest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"zeroparse.dev/zp/codegen"
	"zeroparse.dev/zp/engine"
)

// The graphs are loaded once per test binary: every test of the package reads them.
var (
	graphsOnce sync.Once
	graphsVal  []*graph
	graphsErr  error
)

// loadAll loads the graph of the default build and of every reference tag.
func loadAll(t *testing.T) []*graph {
	t.Helper()
	graphsOnce.Do(func() {
		var tags []string
		tags, graphsErr = referenceTags(moduleRoot)
		if graphsErr == nil {
			graphsVal, graphsErr = loadGraphs(moduleRoot, tags)
		}
	})
	if graphsErr != nil {
		t.Fatal(graphsErr)
	}
	return graphsVal
}

// genModule is the import path of the parsers that the test generates.
const genModule = "example.com/archgen"

// The generated parsers are generated once per test binary.
var (
	gensOnce sync.Once
	gensVal  []generatedParser
)

// generatedParsers returns the imports of Go parsers generated in both library modes.
func generatedParsers(t *testing.T) []generatedParser {
	t.Helper()
	gensOnce.Do(func() { gensVal = generateParsers(t) })
	if gensVal == nil {
		t.Fatal("generating the parsers failed")
	}
	return gensVal
}

// generateParsers generates Go parsers in both library modes, with every feature that adds
// imports, and returns the imports of their files.
func generateParsers(t *testing.T) []generatedParser {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(moduleRoot, "testdata/grammars/csv/csv.zp"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := engine.ParseGrammar(string(src))
	if err != nil {
		t.Fatal(err)
	}
	var gens []generatedParser
	for _, lib := range []codegen.LibMode{codegen.LibImport, codegen.LibEmbed} {
		var imports []string
		for _, c := range []codegen.Go{
			{Types: true, Document: true, Stream: true, Observability: true, LimitControls: true, Recognize: true, Expectations: true, SourceInfo: true},
			{RecognizeOnly: true},
		} {
			c.Module, c.Lib = genModule, lib
			files, err := c.Generate(g, "main")
			if err != nil {
				t.Fatalf("generate with -lib %v: %v", lib, err)
			}
			for _, f := range files {
				if !strings.HasSuffix(f.Path, ".go") {
					continue
				}
				af, err := parser.ParseFile(token.NewFileSet(), f.Path, f.Content, parser.ImportsOnly)
				if err != nil {
					t.Fatalf("generated %s: %v", f.Path, err)
				}
				for _, s := range af.Imports {
					p, _ := strconv.Unquote(s.Path.Value)
					if !slices.Contains(imports, p) {
						imports = append(imports, p)
					}
				}
			}
		}
		slices.Sort(imports)
		// Guard against a check that passes because the generated code imports nothing of the
		// module: a parser in either mode reaches the runtime through the module.
		golibPath := modulePath + "/codegen/go/lib"
		switch {
		case lib == codegen.LibImport && !slices.Contains(imports, golibPath):
			t.Fatalf("a parser generated with -lib import does not import %s: %v", golibPath, imports)
		case lib == codegen.LibEmbed && (slices.Contains(imports, golibPath) || !slices.ContainsFunc(imports, inModule)):
			t.Fatalf("a parser generated with -lib embed imports %s or no package of the module: %v", golibPath, imports)
		}
		gens = append(gens, generatedParser{name: "generated parser (-lib " + lib.String() + ")", module: genModule, imports: imports})
	}
	return gens
}

// TestImportGraph checks the import graph of every build against the rules and the allowlist.
func TestImportGraph(t *testing.T) {
	graphs := loadAll(t)
	r := check(graphs, generatedParsers(t))
	for _, m := range r.compare(allowlist, len(graphs)) {
		t.Error(m)
	}
	t.Logf("checked %d builds; %d allowlisted edges", len(graphs), len(allowlist))
}

// TestNegativeControls adds fake edges to the default build's graph, and removes an allowlisted
// one, and checks that each change fails the check with the expected message.
func TestNegativeControls(t *testing.T) {
	g := loadAll(t)[0].clone()
	if g.tags != "" {
		t.Fatalf("the first graph is of build %q, not the default build", g.tags)
	}
	mod := func(p string) string { return modulePath + "/" + p }
	addEdge := func(from, to string) {
		p := g.pkgs[from]
		if p == nil {
			t.Fatalf("no package %s", from)
		}
		if slices.Contains(p.Imports, to) {
			t.Fatalf("%s already imports %s", from, to)
		}
		p.Imports = append(p.Imports, to)
	}
	// Rule 1: a runtime-side package that a public one imports links the compiler.
	addEdge(mod("internal/limits"), mod("internal/ir/core"))
	// Rule 1: a runtime-side package imports a third-party package.
	g.pkgs["example.org/thirdparty"] = &pkg{ImportPath: "example.org/thirdparty"}
	addEdge(mod("schema"), "example.org/thirdparty")
	// Rule 1a: the library links the VM, so do the parsers generated with -lib import.
	addEdge(mod("codegen/go/lib"), mod("internal/engine/vm"))
	// Rule 1a: the node representation links the internal runtime, so do the parsers generated
	// with -lib embed.
	addEdge(mod("core/impl"), mod("internal/value"))
	// Rule 2: the Code IR base imports a dialect.
	addEdge(mod("internal/ir/exec"), mod("internal/ir/flow"))
	// A package that no layout row places.
	g.pkgs[mod("internal/unplaced")] = &pkg{ImportPath: mod("internal/unplaced"), Module: &struct{ Path string }{modulePath}}
	addEdge(mod("cmd/zp"), mod("internal/unplaced"))
	// An allowlisted edge that disappears.
	doc := g.pkgs[mod("internal/engine/document")]
	doc.Imports = slices.DeleteFunc(doc.Imports, func(p string) bool { return p == mod("internal/compile") })

	msgs := check([]*graph{g}, generatedParsers(t)).compare(allowlist, 1)
	for _, m := range msgs {
		t.Log(m)
	}
	want := []string{
		"rule 1: internal/limits -> internal/ir/core\n\tbreaks: the runtime side (internal/runtime/limits) imports only the runtime side, not internal/ir/core\n\tchain: query -> internal/limits -> internal/ir/core\n",
		"rule 1: schema -> example.org/thirdparty\n\tbreaks: the runtime side imports no third-party package\n\tchain: schema -> example.org/thirdparty\n",
		"rule 1a: codegen/go/lib -> internal/engine/vm\n",
		"rule 1a: internal/bench/parsers/json -> internal/engine/vm\n\tbreaks: a generated parser links neither pkg/runtime/vm nor internal/runtime/**\n\tchain: internal/bench/parsers/json -> codegen/go/lib -> internal/engine/vm\n",
		"rule 1a: generated parser (-lib import) -> internal/engine/vm\n\tbreaks: a generated parser links neither pkg/runtime/vm nor internal/runtime/**\n\tchain: generated parser (-lib import) -> codegen/go/lib -> internal/engine/vm\n",
		"rule 1a: generated parser (-lib embed) -> internal/value\n\tbreaks: a generated parser links neither pkg/runtime/vm nor internal/runtime/**\n\tchain: generated parser (-lib embed) -> core/impl -> internal/value\n",
		"rule 2: internal/ir/exec -> internal/ir/flow\n\tbreaks: internal/ir/code imports no dialect (internal/ir/code -> internal/ir/code/flow)\n",
		"internal/unplaced: no row of layout places the package",
		"rule 1: internal/engine/document -> internal/compile: no build has this edge any more; remove it from the allowlist",
	}
	for _, w := range want {
		if !slices.ContainsFunc(msgs, func(m string) bool { return strings.HasPrefix(m+"\n", w) }) {
			t.Errorf("no failure begins with %q", w)
		}
	}
	if len(msgs) == 0 {
		t.Error("the fake edges pass the check")
	}
}
