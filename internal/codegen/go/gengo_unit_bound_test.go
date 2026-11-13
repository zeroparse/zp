package gogen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

// A generated Go parser with limit controls charges at least one unit, and at most a constant, for
// each call, character, iteration, operator and value ("Cost" in spec/runtime/parsing.md), as the
// engine's backends do (TestBudgetCoarseChargesPerIteration). Each grammar adds one element to the
// input at a time, and the least budget under which the parse finishes has to grow by the same
// step at every size: a constant per element. The step is pinned exactly, so a charge that is lost
// for a call, a character, an iteration, an operator or a value changes it.
func TestGeneratedGoChargesEachElement(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	cases := []struct {
		pkg, src, before, unit, after string
		step                          uint64
	}{
		{"repeat", `rule main = ("a" "b")* $$`, "", "ab", "", 11},
		{"scan", `rule main = [a-z]* $$`, "", "a", "", 6},
		{"call", "rule main = item* $$\nrule item = \"a\"", "", "a", "", 8},
		{"capturecall", "rule main = item* $$\nrule item = x:\"a\" {len(text($x)) == 1}", "", "a", "", 12},
		{"projected", "type L = { N int }\nrule main = xs:(-\",\" r:x)* $$ => L{N: len(map($xs, (e) => e.$r))}\nrule x = [a-z] #atomic", "", ",a", "", 13},
		{"prefix", "rule main = expr $$\nrule expr = #operators {\n    operand \"a\"\n    level {\n        infix left \"+\"\n    }\n    level {\n        prefix \"-\"\n    }\n}", "", "-", "a", 11},
		{"infix", "rule main = expr $$\nrule expr = #operators {\n    operand \"a\"\n    level {\n        infix left \"+\"\n    }\n}", "a", "+a", "", 17},
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gentest.GoMod("fixture")), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		g, err := parseTest(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.pkg, err)
		}
		code, err := gentest.GenerateCode(t, g, codegen.GenOptions{Package: c.pkg, Start: "main"})
		if err != nil {
			t.Fatalf("%s: %v", c.pkg, err)
		}
		consumer := "package " + c.pkg + "\n\nimport \"testing\"\n\nfunc TestChargesEachElement(t *testing.T) { chargesEachElement(t, " +
			`"` + c.before + `", "` + c.unit + `", "` + c.after + `", ` + strconv.FormatUint(c.step, 10) + `, func(text string, budget uint64) error { _, err := Parse(text, WithLimits(Limits{Compute: budget})); return err }) }` + "\n"
		for name, source := range map[string]string{"parser.go": string(code), "parser_test.go": consumer, "bound_test.go": generatedGoUnitBoundHelper(c.pkg)} {
			path := filepath.Join(dir, c.pkg, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS="+os.Getenv("GOFLAGS"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated charges: %v\n%s", err, out)
	}
}

func generatedGoUnitBoundHelper(pkg string) string {
	return "package " + pkg + `

import (
	"errors"
	"strings"
	"testing"

	zpparse "zeroparse.dev/zp/core"
)

func chargesEachElement(t *testing.T, before, unit, after string, want uint64, parse func(text string, budget uint64) error) {
	least := func(k int) uint64 {
		text := before + strings.Repeat(unit, k) + after
		finishes := func(budget uint64) bool {
			err := parse(text, budget)
			var le *zpparse.LimitError
			if errors.As(err, &le) && le.Kind == zpparse.LimitCompute {
				return false
			}
			if err != nil {
				t.Fatalf("parse %q: %v", text, err)
			}
			return true
		}
		lo, hi := uint64(0), uint64(1)
		for !finishes(hi) {
			lo, hi = hi, hi*2
		}
		for hi-lo > 1 {
			if mid := lo + (hi-lo)/2; finishes(mid) {
				hi = mid
			} else {
				lo = mid
			}
		}
		return hi
	}
	var steps []uint64
	for k := 8; k <= 11; k++ {
		steps = append(steps, least(k+1)-least(k))
	}
	for _, step := range steps {
		if step != want {
			t.Fatalf("one more element costs %v units at 9 to 12 elements, want %d", steps, want)
		}
	}
}
`
}
