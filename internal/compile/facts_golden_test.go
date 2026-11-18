package compile

import (
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/testing/factstest"
	"zeroparse.dev/zp/internal/testing/starttest"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// TestFactsGolden requires the rule flags of the program of every repository grammar, which the
// compiler takes from the facts of its Core IR module, and the first terminals of its rules and
// alternatives, to be those pinned (internal/testing/factstest).
func TestFactsGolden(t *testing.T) {
	t.Parallel()
	for _, path := range factstest.Corpus {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			g, err := parser.Parse(factstest.Read(t, path))
			if err != nil {
				t.Fatal(err)
			}
			g = starttest.MarkAll(g)
			names := desugar.InlineNames{}
			prog, err := Check(g, Options{}, nil, names, nil)
			if err != nil {
				t.Fatal(names.Rewrite(err))
			}
			want := factstest.Load(t, path)
			// Cycles are numbered in the golden files in the order of their first rule.
			cycles, insCycles := map[int]int{}, map[int]int{}
			number := func(m map[int]int, c int) string {
				if c == 0 {
					return "0"
				}
				if m[c] == 0 {
					m[c] = len(m) + 1
				}
				return strconv.Itoa(m[c])
			}
			for _, r := range prog.RuleList {
				w := want[r.Name]
				if w == nil {
					t.Fatalf("rule %s is not pinned", r.Name)
				}
				vars := strings.Join(r.Vars, ",")
				if vars == "" {
					vars = "-"
				}
				got := map[string]string{"cyclic": strconv.FormatBool(r.Cycle != 0), "cycle": number(cycles, r.Cycle),
					"leader": strconv.FormatBool(r.Leader), "member": strconv.FormatBool(r.Member), "rec": strconv.FormatBool(r.Rec),
					"positional": strconv.FormatBool(r.Positional), "vars": vars, "memo": strconv.FormatBool(r.Memo),
					"transient": strconv.FormatBool(r.Transient), "missing": strconv.FormatBool(r.Missing),
					"first": terminalFact(firstTerminal(prog, r.Def.Expr, 0, map[string]bool{})).String()}
				if _, ok := w["ins-cycle"]; ok {
					ins := RuleFlags{Cycle: r.Cycle, Leader: r.Leader, Member: r.Member}
					if prog.InsertionFlags != nil {
						ins = prog.InsertionFlags[r.ID]
					}
					got["ins-cycle"], got["ins-leader"], got["ins-member"] = number(insCycles, ins.Cycle), strconv.FormatBool(ins.Leader), strconv.FormatBool(ins.Member)
				}
				for k, v := range got {
					if w[k] != v {
						t.Errorf("rule %s: %s=%s, pinned %s", r.Name, k, v, w[k])
					}
				}
			}
		})
	}
}

func terminalFact(e ast.Expr, depth int) factstest.Terminal {
	switch e := e.(type) {
	case *ast.Literal:
		return factstest.Terminal{Exp: LiteralDesc(e), Depth: depth}
	case *ast.CharClass:
		return factstest.Terminal{Exp: CharClassString(e), Depth: depth}
	}
	return factstest.Terminal{}
}
