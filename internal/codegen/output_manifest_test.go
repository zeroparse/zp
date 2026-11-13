package codegen_test

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"zeroparse.dev/zp/codegen"
	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/codegentest"
)

// manifestFile is the checked-in manifest of the generated output of manifestGrammars in every
// manifestModes option set. Its lines are "grammar mode path sha256 bytes lines funcs", or
// "grammar mode error message" for an option set the generator rejects.
const manifestFile = "testdata/output_manifest.txt"

var outputTree = flag.String("output-tree", "", "with TestGeneratedOutputManifest: write the full text of every manifest output under this directory")

// manifestGrammars are the grammars of the output manifest: the grammars of testdata/grammars and the examples
// that the benchmarks generate. Paths are relative to this package.
var manifestGrammars = []struct{ name, path string }{
	{"cel", "../../testdata/grammars/cel/cel.zp"},
	{"csv", "../../testdata/grammars/csv/csv.zp"},
	{"cue", "../../testdata/grammars/cue/cue.zp"},
	{"duckdb", "../../testdata/grammars/duckdb/duckdb.zp"},
	{"golang", "../../testdata/grammars/golang/golang.zp"},
	{"json", "../../testdata/grammars/json/json.zp"},
	{"python", "../../testdata/grammars/python/python.zp"},
	{"typescript", "../../testdata/grammars/typescript/typescript.zp"},
	{"xml", "../../testdata/grammars/xml/xml.zp"},
	{"yaml", "../../testdata/grammars/yaml/yaml.zp"},
	{"calc", "../../examples/calculator/calc.zp"},
	{"calclr", "../../examples/calculator/calc_lr.zp"},
	{"minilang", "../../examples/minilang/minilang.zp"},
	{"outline", "../../examples/outline/outline.zp"},
}

// manifestModes are the configurations of the output manifest. Each is generated for every grammar.
var manifestModes = []struct {
	name   string
	config codegen.Go
}{
	{"node", codegen.Go{}},
	{"node-exp", codegen.Go{Expectations: true}},
	{"typed-exp", codegen.Go{Types: true, Expectations: true}},
	{"node-rec", codegen.Go{Recognize: true}},
	{"node-wc", codegen.Go{LimitControls: true}},
	{"node-rec-wc", codegen.Go{Recognize: true, LimitControls: true}},
	{"node-doc", codegen.Go{Document: true}},
	{"node-obs", codegen.Go{Observability: true}},
	{"node-rec-obs", codegen.Go{Recognize: true, Observability: true}},
	{"node-doc-wc", codegen.Go{Document: true, LimitControls: true}},
	{"node-stream", codegen.Go{Stream: true}},
	{"typed", codegen.Go{Types: true}},
	{"typed-only", codegen.Go{Types: true, TypedOnly: true}},
	{"typed-rec-wc", codegen.Go{Types: true, Recognize: true, LimitControls: true}},
	{"typed-doc", codegen.Go{Types: true, Document: true}},
	{"typed-stream", codegen.Go{Types: true, Stream: true}},
	{"typed-only-wc", codegen.Go{Types: true, TypedOnly: true, LimitControls: true}},
	{"default-value", codegen.Go{DefaultValue: true}},
	{"source-info", codegen.Go{SourceInfo: true}},
}

// manifestOutputs generates every output of the manifest and returns its lines, sorted by grammar,
// mode and path, and the full text of each output keyed by "grammar/mode/path".
func manifestOutputs(t *testing.T) ([]string, map[string]string) {
	t.Helper()
	type result struct {
		lines []string
		texts map[string]string
		err   error
	}
	// Every grammar and mode is generated on its own, concurrently, from a grammar that it parses
	// itself; the results are joined in the order of the matrix.
	results := make([]result, len(manifestGrammars)*len(manifestModes))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for gi, g := range manifestGrammars {
		for mi, m := range manifestModes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				r := &results[gi*len(manifestModes)+mi]
				r.texts = map[string]string{}
				src, err := os.ReadFile(g.path)
				if err != nil {
					r.err = err
					return
				}
				gr, err := engine.ParseGrammar(string(src))
				if err != nil {
					r.err = fmt.Errorf("%s: %v", g.path, err)
					return
				}
				c := m.config
				c.Module, c.Lib = "codegentest/"+g.name, codegen.LibImport
				files, err := c.Generate(gr, "")
				if err != nil {
					// A combination the generator rejects, such as -stream without a #stream repetition,
					// is a line of the manifest too, so that a change of rejection fails the test.
					r.lines = append(r.lines, fmt.Sprintf("%s %s error %s", g.name, m.name, strings.ReplaceAll(err.Error(), "\n", " ")))
					return
				}
				slices.SortFunc(files, func(a, b codegen.File) int { return strings.Compare(a.Path, b.Path) })
				for _, f := range files {
					sum := sha256.Sum256(f.Content)
					funcs := 0
					for _, l := range strings.Split(string(f.Content), "\n") {
						if strings.HasPrefix(l, "func ") {
							funcs++
						}
					}
					r.lines = append(r.lines, fmt.Sprintf("%s %s %s %s %d %d %d", g.name, m.name, f.Path, hex.EncodeToString(sum[:]), len(f.Content), strings.Count(string(f.Content), "\n"), funcs))
					r.texts[g.name+"/"+m.name+"/"+f.Path] = string(f.Content)
				}
			}()
		}
	}
	wg.Wait()
	var lines []string
	texts := map[string]string{}
	for _, r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		lines = append(lines, r.lines...)
		for k, v := range r.texts {
			texts[k] = v
		}
	}
	return lines, texts
}

// TestGeneratedOutputManifest checks the hash, size, line and function counts of every output of the
// generator's manifest matrix against testdata/output_manifest.txt. A change to the generated text
// of any output therefore fails until the manifest is regenerated with go test ./internal/codegen -run
// TestGeneratedOutputManifest -update, which makes the change reviewable as a diff of the manifest.
// The full text of the outputs is written with -output-tree.
func TestGeneratedOutputManifest(t *testing.T) {
	lines, texts := manifestOutputs(t)
	if *outputTree != "" {
		for key, text := range texts {
			path := filepath.Join(append([]string{*outputTree}, strings.Split(key, "/")...)...)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	got := strings.Join(lines, "\n") + "\n"
	if *codegentest.UpdateGoldens {
		if err := os.MkdirAll(filepath.Dir(manifestFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifestFile, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(manifestFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) == got {
		return
	}
	wantLines := strings.Split(strings.TrimSuffix(string(want), "\n"), "\n")
	reported := 0
	for i := 0; i < max(len(lines), len(wantLines)) && reported < 20; i++ {
		var g, w string
		if i < len(lines) {
			g = lines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Errorf("manifest line %d:\n  manifest %s\n  generated %s", i+1, w, g)
			reported++
		}
	}
	t.Errorf("%s is out of date (%d lines generated, %d in the manifest); run go test ./internal/codegen -run TestGeneratedOutputManifest -update", manifestFile, len(lines), len(wantLines))
}
