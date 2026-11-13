package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/vm"
	"zeroparse.dev/zp/internal/gentest"
)

// bytecodeEqualityCorpus returns the grammars of the module-equality test: the backend corpus,
// the schema corpus and every grammar file of the repository.
func bytecodeEqualityCorpus(t *testing.T) []gentest.Case {
	cases := slices.Concat(gentest.GenCorpus(t), gentest.ConstructorCases(), gentest.FieldCheckCases(), gentest.ResultCheckCases())
	var files []string
	err := filepath.WalkDir("../..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "site" || d.Name() == ".scratch" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".git")) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".zp") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, gentest.Case{Name: strings.TrimPrefix(f, "../../"), Src: string(src)})
	}
	return cases
}

// TestBytecodeFromCoreMatchesGrammarCompiler checks that the bytecode compiled from Core IR has
// the rule table, match code, string table and character class table of the module the
// grammar-driven compiler makes, main and recognition, with and without memoization, for every
// grammar of the corpus. The term code is left out: it is the engine's translation of each term's
// Core IR in both, appended after the match code's strings.
func TestBytecodeFromCoreMatchesGrammarCompiler(t *testing.T) {
	t.Parallel()
	compared := 0
	for _, c := range bytecodeEqualityCorpus(t) {
		for _, o := range []compile.Options{{}, {DisableMemo: true}} {
			g, err := parseTest(c.Src)
			if err != nil {
				continue
			}
			prog, err := Compile(g, o)
			if err != nil || prog.HasMissing {
				continue
			}
			progs := []*Program{prog}
			if rec, err := prog.Recognizer(); err == nil {
				progs = append(progs, rec)
			}
			for i, p := range progs {
				old := p.Static().BytecodeModule()
				got, err := vm.Compile(p.Static())
				if err != nil {
					t.Errorf("%s (memo off %t, recognizer %t): %v", c.Name, o.DisableMemo, i == 1, err)
					continue
				}
				if diff := moduleDifference(old, got); diff != "" {
					t.Errorf("%s (memo off %t, recognizer %t): %s", c.Name, o.DisableMemo, i == 1, diff)
				}
				compared++
			}
		}
	}
	if compared < 200 {
		t.Fatalf("compared %d modules", compared)
	}
}

// moduleDifference describes how the module got differs from want apart from the term code: the
// terms and the strings after want's are left out.
func moduleDifference(want, got *bytecode.Module) string {
	w, g := *want, *got
	w.Terms, g.Terms = nil, nil
	if len(g.Strings) >= len(w.Strings) {
		g.Strings = g.Strings[:len(w.Strings)]
	}
	if reflect.DeepEqual(w, g) {
		return ""
	}
	var parts []string
	if !slices.Equal(w.Strings, g.Strings) {
		parts = append(parts, "strings differ")
	}
	if !reflect.DeepEqual(w.Classes, g.Classes) {
		parts = append(parts, "classes differ")
	}
	if !reflect.DeepEqual(w.Rules, g.Rules) {
		parts = append(parts, "rules differ")
	}
	if !reflect.DeepEqual(w.Scopes, g.Scopes) {
		parts = append(parts, "scopes differ")
	}
	if !reflect.DeepEqual(w.OperatorsRules, g.OperatorsRules) {
		parts = append(parts, "operators rules differ")
	}
	if !slices.Equal(w.Code, g.Code) {
		for i := range min(len(w.Code), len(g.Code)) {
			if w.Code[i] != g.Code[i] {
				parts = append(parts, fmt.Sprintf("code differs first at %d: want %+v, got %+v", i, w.Code[i], g.Code[i]))
				break
			}
		}
	}
	// The first differing line of the listings.
	wl, gl := strings.Split(w.Disassemble(), "\n"), strings.Split(g.Disassemble(), "\n")
	for i := range max(len(wl), len(gl)) {
		var a, b string
		if i < len(wl) {
			a = wl[i]
		}
		if i < len(gl) {
			b = gl[i]
		}
		if a != b {
			lo := max(0, i-3)
			return strings.Join(parts, ", ") + "\n want: " + strings.Join(wl[lo:min(len(wl), i+3)], "\n       ") + "\n got:  " + strings.Join(gl[lo:min(len(gl), i+3)], "\n       ")
		}
	}
	return strings.Join(parts, ", ")
}
