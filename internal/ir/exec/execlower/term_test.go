package execlower_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/core"
	irbuild "zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/execlower"
)

var update = flag.Bool("update", false, "update golden files")

// TestTermDumpGolden lists the terms of every grammar of testdata/grammars lowered to Exec IR, against
// the goldens in testdata/terms: a review aid that shows what a change of the lowering does to the
// printed programs. A term that Term does not lower is listed by the kinds that it holds and Term
// does not lower.
func TestTermDumpGolden(t *testing.T) {
	paths, err := filepath.Glob("../../../../testdata/grammars/*/*.zp")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no grammars: %v", err)
	}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".zp")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			g, err := compiletest.Parse(string(src))
			if err != nil {
				t.Fatal(err)
			}
			prog, err := compiletest.Compile(g, compile.Options{})
			if err != nil {
				t.Fatal(err)
			}
			m, err := irbuild.CoreFrom(prog, nil)
			if err != nil {
				t.Fatal(err)
			}
			got := dumpTerms(t, m)
			golden := filepath.Join("testdata", "terms", name+".golden")
			if *update {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("the lowered terms differ from %s; run go test -run TestTermDumpGolden -update", golden)
			}
		})
	}
}

// dumpTerms lists the terms of the rules of m in the order of the rules: their actions, the actions of
// the lines of Operators Rules, and the terms of predicates and bindings.
func dumpTerms(t *testing.T, m *core.Module) string {
	var b strings.Builder
	dump := func(rule, what string, x core.Term) {
		if x == nil {
			return
		}
		fmt.Fprintf(&b, "## %s %s\n", rule, what)
		l, err := execlower.Term(x, execlower.TermOptions{})
		if err == execlower.ErrUnsupported {
			fmt.Fprintf(&b, "not lowered: %s\n\n", unsupported(x))
			return
		}
		if err != nil {
			t.Fatalf("%s %s: %v", rule, what, err)
		}
		b.WriteString(exec.FuncString(l.Func))
		b.WriteString("\n")
	}
	for _, r := range m.Rules {
		if r == nil {
			continue
		}
		if r.Finish.Kind == core.FinishAction {
			dump(r.Name, "action", r.Finish.Action)
		}
		core.Walk(r.Body, func(n core.Node) {
			switch n := n.(type) {
			case *core.Pred:
				dump(r.Name, "predicate", n.Term)
			case *core.BindNode:
				dump(r.Name, "binding "+n.Var, n.Term)
			case *core.OperatorsRule:
				for i, l := range n.Lines() {
					dump(r.Name, fmt.Sprintf("line %d", i), l.Action)
				}
			}
		})
	}
	return b.String()
}

// unsupported returns the kinds of the subterms of x that Term does not lower, in preorder, each
// once.
func unsupported(x core.Term) string {
	seen := map[string]bool{}
	var kinds []string
	core.WalkTerm(x, func(s core.Term) {
		var k string
		switch s := s.(type) {
		case *core.New:
			k = "New"
		case *core.Tuple:
			k = "Tuple"
		case *core.Lambda:
			k = "Lambda"
		case *core.Local:
			if s.Kind == core.LocalParam {
				k = "param"
			}
		case *core.Builtin:
			if s.Func != "len" && s.Func != "text" {
				k = s.Func
			}
		}
		if k != "" && !seen[k] {
			seen[k] = true
			kinds = append(kinds, k)
		}
	})
	return strings.Join(kinds, ", ")
}
