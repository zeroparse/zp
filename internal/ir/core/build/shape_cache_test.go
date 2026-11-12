package build_test

import (
	"slices"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/check"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/grammaranalysis"
)

// TestShapeCacheFacts checks that the shape cache the build uses gives every expression of every
// corpus grammar, and of a grammar of nested splice rules, the facts ShapeFacts computes from
// scratch, whether the cache meets the expressions inside an expression before or after it.
func TestShapeCacheFacts(t *testing.T) {
	progs := map[string]*compile.Program{
		"splices": coreProgram(t, "rule main = x:\"a\" _m (y:\"c\" / \"d\")? z:(\"e\" _m)* _m\nrule _m = \"b\" _m?"),
	}
	for _, path := range factstest.Corpus {
		g, err := compiletest.Parse(factstest.Read(t, path))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		prog, err := compiletest.Compile(g, compile.Options{})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		progs[path] = prog
	}
	for name, prog := range progs {
		chk := prog.Checker()
		var exprs []grammar.Expr
		for _, r := range prog.RuleList {
			grammaranalysis.WalkExpr(r.Def.Expr, func(e grammar.Expr) { exprs = append(exprs, e) })
		}
		if len(exprs) == 0 {
			t.Fatalf("%s: no expressions", name)
		}
		for _, order := range []string{"outer first", "inner first"} {
			es := exprs
			if order == "inner first" {
				es = slices.Clone(exprs)
				slices.Reverse(es)
			}
			c := check.NewShapeCache(chk)
			for _, e := range es {
				k, mode, tuple := c.Facts(e)
				wk, wmode, wtuple := check.ShapeFacts(chk, e)
				if k != wk || mode != wmode || tuple != wtuple {
					t.Errorf("%s, %s: %T at %v: cached facts (%d, %v, %v), want (%d, %v, %v)", name, order, e, e, k, mode, tuple, wk, wmode, wtuple)
				}
			}
		}
	}
}
