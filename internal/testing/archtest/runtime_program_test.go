package archtest

import (
	"slices"
	"strings"
	"testing"
)

// checkedProgramPackage holds the checked program of the compiler (compile.Program): the grammar
// AST, the semantic model and the rule facts that the generators and analyses read.
const checkedProgramPackage = modulePath + "/internal/compile"

// TestRuntimeReadsNoCheckedProgram checks, in every build, that no package of the internal runtime
// (internal/runtime/** and internal/runtime/engine, the run-time entries) reaches the checked program of the
// compiler, directly or through another package. The runtime runs the run-time program
// (internal/runtime/program) and reaches what the compiler builds only through state.Source. Rule 1
// forbids such an edge too, but the allowlist can hold one; this check holds no exceptions.
func TestRuntimeReadsNoCheckedProgram(t *testing.T) {
	for _, g := range loadAll(t) {
		var runtime []string
		for path := range g.pkgs {
			rel, ok := strings.CutPrefix(path, modulePath+"/")
			if ok && (strings.HasPrefix(rel, "internal/runtime/") || rel == "internal/runtime/engine") {
				runtime = append(runtime, path)
			}
		}
		if len(runtime) < 2 {
			t.Fatalf("build %q: found %d runtime packages, want the internal runtime", g.tags, len(runtime))
		}
		slices.Sort(runtime)
		for _, from := range runtime {
			if chain := g.shortestChain([]string{from}, checkedProgramPackage, func(a, b string) bool { return inModule(b) }); chain != nil {
				t.Errorf("build %q: %s reaches the checked program: %s", g.tags, from, strings.Join(chain, " -> "))
			}
		}
	}
}

// modelReaders are the packages outside the compiler that read the checked grammar through its
// semantic model (check.Model), its checker and the facts of its Core IR, not through the checked
// program. Only the drivers (internal/driver/checked) and the bytecode compiler
// (internal/backend/vmlower) are to keep reading it; the list grows as the other readers move.
var modelReaders = []string{
	"internal/backend/actionlower",
	"internal/frontend/schemaexport",
}

// TestModelReadersReadNoCheckedProgram checks, in every build, that no package of modelReaders
// reaches the checked program of the compiler, directly or through another package.
func TestModelReadersReadNoCheckedProgram(t *testing.T) {
	for _, g := range loadAll(t) {
		for _, rel := range modelReaders {
			from := modulePath + "/" + rel
			if _, ok := g.pkgs[from]; !ok {
				t.Fatalf("build %q: no package %s", g.tags, rel)
			}
			if chain := g.shortestChain([]string{from}, checkedProgramPackage, func(a, b string) bool { return inModule(b) }); chain != nil {
				t.Errorf("build %q: %s reaches the checked program: %s", g.tags, rel, strings.Join(chain, " -> "))
			}
		}
	}
}
