package archtest

import (
	"fmt"
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
	// Rule 2: the front end imports the analysis layer.
	addEdge(mod("internal/desugar"), mod("internal/analysis/lint"))
	// Rule 2: an analysis package imports two dialects.
	addEdge(mod("internal/analysis/lint"), mod("internal/ir/flow"))
	addEdge(mod("internal/analysis/lint"), mod("internal/ir/exec/ops"))
	// Rule 2, one edge for each clause.
	addEdge(mod("internal/ir/pipeline"), mod("internal/ir/core"))             // internal/pass
	addEdge(mod("internal/ir/core"), mod("internal/diagnostic"))              // internal/ir/** imports only ...
	addEdge(mod("internal/ir/flow"), mod("internal/ir/exec/ops"))             // code/flow
	addEdge(mod("internal/ir/exec/ops"), mod("internal/ir/exec/core"))        // code/exec
	addEdge(mod("internal/ir/exec/core"), mod("internal/ir/core"))            // the packages under code/exec
	addEdge(mod("bench/report"), mod("internal/lsp"))                         // internal/lsp
	addEdge(mod("internal/analysis/lint"), mod("internal/testing/starttest")) // internal/testing
	addEdge(mod("grammar"), mod("internal/diagnostic"))                       // the utilities are above the grammar AST
	// Rule 2: edges against the arrows of the layer graph (TestLayerGraph checks every pair).
	addEdge(mod("internal/desugar"), mod("internal/ir/exec/lower"))
	addEdge(mod("internal/engine/closure"), mod("internal/analysis/lint"))
	// Packages at paths that no current package has: an Action dialect package and the public VM.
	fake := func(path, target string) {
		layout = append(layout, place{pkg: path, target: target})
		g.pkgs[mod(path)] = &pkg{ImportPath: mod(path), Module: &struct{ Path string }{modulePath}}
	}
	saved := layout
	layout = slices.Clone(layout)
	t.Cleanup(func() { layout = saved })
	fake("internal/fakeaction", "internal/ir/code/action")
	fake("internal/fakevm", "pkg/runtime/vm")
	// Rule 1a: the library imports the public VM.
	addEdge(mod("codegen/go/lib"), mod("internal/fakevm"))
	// Rule 2: a package under code/exec may import the Action dialect, code/exec itself and
	// code/flow may not.
	addEdge(mod("internal/ir/exec/core"), mod("internal/fakeaction"))
	addEdge(mod("internal/ir/exec/ops"), mod("internal/fakeaction"))
	addEdge(mod("internal/ir/flow"), mod("internal/fakeaction"))
	// Rule 2: the Action dialect imports another dialect.
	addEdge(mod("internal/fakeaction"), mod("internal/ir/exec/ops"))
	// Rule 1a: a generated parser imports a third-party package.
	addEdge(mod("internal/bench/parsers/json"), "example.org/thirdparty")
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
	// Each failure begins with the key and contains the parts.
	want := []struct {
		key   string
		parts []string
	}{
		{"rule 1: internal/limits -> internal/ir/core", []string{"\tbreaks: the runtime side (internal/runtime/limits) imports only the runtime side, not internal/ir/core\n", "\tchain: query -> internal/limits -> internal/ir/core\n"}},
		{"rule 1: schema -> example.org/thirdparty", []string{"\tbreaks: the runtime side imports no third-party package\n", "\tchain: schema -> example.org/thirdparty\n"}},
		// One failure for the edge, listing the generated parsers that reach it.
		{"rule 1a: codegen/go/lib -> internal/engine/vm", []string{
			"pkg/runtime/golib imports only pkg/runtime and its input, value, value/impl, instrument and schema packages, not internal/runtime/vm",
			"a generated parser links neither pkg/runtime/vm nor internal/runtime/**",
			"\tchain: codegen/go/lib -> internal/engine/vm\n",
			"generated parser (-lib import)", "internal/bench/parsers/json,", "internal/syntax/grammarparser,"}},
		{"rule 1a: core/impl -> internal/value", []string{"codegen/go/lib -> core/impl -> internal/value\n\tbuilds", "\tgenerated parsers: generated parser (-lib embed), generated parser (-lib import), internal/bench/parsers/"}},
		{"rule 2: internal/ir/exec -> internal/ir/flow", []string{"\tbreaks: internal/ir/code imports no dialect (internal/ir/code -> internal/ir/code/flow)\n"}},
		{"rule 2: internal/desugar -> internal/analysis/lint", []string{"the front end layer does not import the analysis layer (internal/frontend/desugar -> internal/analysis/lint)"}},
		{"rule 2: internal/analysis/lint -> dialects exec, flow", []string{"only the conversions of the middle end and the back end and test support import more than one dialect (internal/analysis/lint)"}},
		{"rule 2: internal/ir/pipeline -> internal/ir/core", []string{"internal/pass imports nothing of the module (internal/pass -> internal/ir/core)"}},
		{"rule 2: internal/ir/core -> internal/diagnostic", []string{"internal/ir/** imports only internal/ir/**, internal/pass, internal/cancel and pkg/grammar (internal/ir/core -> internal/diagnostic)"}},
		{"rule 2: internal/ir/flow -> internal/ir/exec/ops", []string{"within internal/ir, code/flow imports only code and ir/core (internal/ir/code/flow -> internal/ir/code/exec)"}},
		{"rule 2: internal/ir/exec/ops -> internal/ir/exec/core", []string{"within internal/ir, code/exec imports only code (internal/ir/code/exec -> internal/ir/code/exec/compose)"}},
		{"rule 2: internal/ir/exec/core -> internal/ir/core", []string{"within internal/ir, the packages under code/exec import only code, code/exec and code/action (internal/ir/code/exec/compose -> internal/ir/core)"}},
		{"rule 2: bench/report -> internal/lsp", []string{"only cmd/** and internal/testing/** import internal/lsp/** (bench/report -> internal/lsp/zpls)"}},
		{"rule 2: internal/analysis/lint -> internal/testing/starttest", []string{"only internal/testing/** imports internal/testing/** outside tests (internal/analysis/lint -> internal/testing/starttest)"}},
		{"rule 2: grammar -> internal/diagnostic", []string{"the grammar layer does not import the utility layer (pkg/grammar -> internal/diagnostic)"}},
		{"rule 2: internal/desugar -> internal/ir/exec/lower", []string{"the front end layer does not import the middle end layer"}},
		{"rule 2: internal/engine/closure -> internal/analysis/lint", []string{"the back end layer does not import the analysis layer"}},
		{"rule 1a: codegen/go/lib -> internal/fakevm", []string{"pkg/runtime/golib imports only pkg/runtime and its input, value, value/impl, instrument and schema packages, not pkg/runtime/vm"}},
		{"rule 2: internal/ir/exec/ops -> internal/fakeaction", []string{"within internal/ir, code/exec imports only code (internal/ir/code/exec -> internal/ir/code/action)"}},
		{"rule 2: internal/ir/flow -> internal/fakeaction", []string{"within internal/ir, code/flow imports only code and ir/core"}},
		{"rule 2: internal/ir/flow -> dialects action, exec, flow", []string{"only the conversions of the middle end and the back end and test support import more than one dialect (internal/ir/code/flow)"}},
		{"rule 2: internal/fakeaction -> internal/ir/exec/ops", []string{"within internal/ir, code/action imports only code (internal/ir/code/action -> internal/ir/code/exec)"}},
		{"rule 1a: internal/bench/parsers/json -> example.org/thirdparty", []string{"a generated parser imports no third-party package"}},
		{"internal/unplaced: no row of layout places the package", nil},
		{"rule 1: internal/engine/document -> internal/compile: no build has this edge any more; remove it from the allowlist", nil},
	}
	for _, w := range want {
		i := slices.IndexFunc(msgs, func(m string) bool { return strings.HasPrefix(m, w.key) })
		if i < 0 {
			t.Errorf("no failure begins with %q", w.key)
			continue
		}
		for _, part := range w.parts {
			if !strings.Contains(msgs[i]+"\n", part) {
				t.Errorf("the failure %q does not contain %q", msgs[i], part)
			}
		}
	}
	// A package under code/exec imports the Action dialect: no failure.
	for _, m := range msgs {
		if strings.HasPrefix(m, "rule 2: internal/ir/exec/core -> internal/fakeaction") || strings.HasPrefix(m, "rule 2: internal/ir/exec/core -> dialects") {
			t.Errorf("a package under code/exec may import the Action dialect: %s", m)
		}
	}
	if len(msgs) == 0 {
		t.Error("the fake edges pass the check")
	}
}

// TestLayerGraph adds an edge between every two layers of the compiler side and checks that the
// layer check fails exactly the edges that the layer graph of the design does not allow. The
// allowed layers are written out here, closed under the arrows, apart from the arrows in
// layerArrows.
func TestLayerGraph(t *testing.T) {
	g := loadAll(t)[0].clone()
	// One package of each layer.
	rep := map[layer]string{
		layerCmd: "cmd/zp", layerPublic: "lint", layerLSP: "internal/lsp", layerDriver: "internal/ir/opt",
		layerAnalysis: "internal/analysis/lint", layerBackend: "internal/codegen", layerMiddle: "internal/ir/flow/lower",
		layerFront: "internal/desugar", layerIR: "internal/ir/core", layerPass: "internal/ir/pipeline",
		layerGrammar: "grammar", layerUtil: "internal/diagnostic", layerTesting: "internal/testing/starttest",
		layerOutside: "bench/report", layerRuntime: "query",
	}
	below := []layer{layerDriver, layerAnalysis, layerBackend, layerMiddle, layerFront, layerIR, layerPass, layerGrammar}
	allowed := map[layer][]layer{
		layerCmd:      append([]layer{layerPublic, layerLSP}, below...),
		layerPublic:   below,
		layerLSP:      below[1:],
		layerDriver:   below[1:],
		layerAnalysis: below[2:],
		layerBackend:  below[3:],
		layerMiddle:   below[4:],
		layerFront:    below[5:],
		layerIR:       below[6:],
		layerPass:     nil,
		layerGrammar:  nil,
		layerUtil:     {layerGrammar},
	}
	var layers []layer
	for l, p := range rep {
		pl, err := placeOf(p)
		if err != nil || layerOf(pl.target) != l {
			t.Fatalf("the representative %s of layer %s is in layer %s (%v)", p, l, layerOf(pl.target), err)
		}
		layers = append(layers, l)
	}
	slices.Sort(layers)
	for _, a := range layers {
		if a == layerRuntime {
			continue // rule 1
		}
		for _, b := range layers {
			if a == b {
				continue
			}
			from, to := modulePath+"/"+rep[a], modulePath+"/"+rep[b]
			if p := g.pkgs[from]; !slices.Contains(p.Imports, to) {
				p.Imports = append(p.Imports, to)
			}
		}
	}
	r := check([]*graph{g}, nil)
	for _, a := range layers {
		if a == layerRuntime {
			continue
		}
		for _, b := range layers {
			if a == b {
				continue
			}
			ok := a == layerTesting || a == layerOutside || b == layerRuntime ||
				b == layerUtil && a != layerGrammar || slices.Contains(allowed[a], b)
			failed := false
			if v := r.violations[fmt.Sprintf("rule 2: %s -> %s", rep[a], rep[b])]; v != nil {
				for _, why := range v.why {
					failed = failed || strings.Contains(why, "layer does not import")
				}
			}
			if failed == ok {
				t.Errorf("%s -> %s (%s -> %s): the layer check fails it: %v, want %v", a, b, rep[a], rep[b], failed, !ok)
			}
		}
	}
}
