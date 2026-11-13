package gentest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

// tierPairs lists the files that give the default tier and the long tier different sizes of the same
// test: the long tier must run at least the default tier's size, so that `go test -tags zp_long` runs
// every test of the default run and the landing gate needs only the long run.
var tierPairs = []struct{ def, long string }{
	{"internal/codegen/term_seeds_test.go", "internal/codegen/term_seeds_long_test.go"},
	{"internal/compile/facts_tier_default_test.go", "internal/compile/facts_tier_long_test.go"},
	{"spec/language/source_adapter_enumerate_default_test.go", "spec/language/source_adapter_enumerate_long_test.go"},
	{"spec/language/source_adapter_generated_default_test.go", "spec/language/source_adapter_generated_long_test.go"},
	{"spec/language/source_adapter_tier_default_test.go", "spec/language/source_adapter_tier_long_test.go"},
}

// tierValues reads the package-level integer constants and the integer list variables of a file.
func tierValues(t *testing.T, path string) (ints map[string]int64, lists map[string][]int64) {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "..", path), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ints, lists = map[string]int64{}, map[string][]int64{}
	num := func(e ast.Expr) int64 {
		l, ok := e.(*ast.BasicLit)
		if !ok || l.Kind != token.INT {
			t.Fatalf("%s: %T is not an integer literal", path, e)
		}
		v, err := strconv.ParseInt(l.Value, 0, 64)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, s := range g.Specs {
			vs, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, n := range vs.Names {
				switch v := vs.Values[i].(type) {
				case *ast.BasicLit:
					ints[n.Name] = num(v)
				case *ast.CompositeLit:
					for _, e := range v.Elts {
						lists[n.Name] = append(lists[n.Name], num(e))
					}
				}
			}
		}
	}
	return ints, lists
}

// TestLongTierIncludesDefaultTier checks that every size and seed list of the default tier is within the
// long tier's. It checks the declared sizes only: that the generators draw the first n cases of a larger
// run from the same stream is by construction.
func TestLongTierIncludesDefaultTier(t *testing.T) {
	for _, p := range tierPairs {
		di, dl := tierValues(t, p.def)
		li, ll := tierValues(t, p.long)
		if len(di)+len(dl) == 0 {
			t.Errorf("%s declares no size", p.def)
		}
		for n, v := range di {
			if lv, ok := li[n]; !ok || lv < v {
				t.Errorf("%s: %s = %d in the default tier but %d in %s", p.def, n, v, lv, p.long)
			}
		}
		for n, vs := range dl {
			have := map[int64]bool{}
			for _, x := range ll[n] {
				have[x] = true
			}
			for _, x := range vs {
				if !have[x] {
					t.Errorf("%s: %s has %d in the default tier but not in %s", p.def, n, x, p.long)
				}
			}
		}
	}
}
