package codegen_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

// fieldRangeSource makes lists and tuples in one rule (two, at offset 4) and reads them from the fields of the
// structs in another (read, at offset 2). A value read from a field keeps the range it had where it was made,
// so every struct that read makes from one of them has the range the node library gives it: the range of the
// elements that add one, and that of the rule two for an empty list or a list of scalars, never that of read.
// The field types are plain, so that a typed parser reads them from the Go values it builds.
const fieldRangeSource = `type Leaf = { T string }
type Bag = { Ints []int; Strs []string; Leaves []Leaf; Pair (int, []int); Nest [][]int }
type Both = { Nest [][]int; Pairs [](int, []int) }
type Wrap = { L []int }
type WrapS = { L []string }
type WrapL = { L []Leaf }
type WrapN = { L [][]int }
type WrapP = { P (int, []int) }
type Out = { A Wrap; B WrapS; C WrapL; D WrapN; E WrapP; F Wrap; G WrapN; H WrapP }
rule leaf Leaf = t:[a-z] #atomic => Leaf{T: text($t)}
rule empty Bag = "<" ">" => Bag{Ints: []int{}, Strs: []string{}, Leaves: []Leaf{}, Pair: (1, []int{}), Nest: [][]int{}}
rule full Bag = "[" x:leaf y:leaf "]" => Bag{Ints: []int{1, 2}, Strs: []string{"p"}, Leaves: []Leaf{$x, $y}, Pair: (1, []int{3}), Nest: [][]int{[]int{1}, []int{}}}
rule bag = empty / full
rule two Both = " " a:bag "|" b:bag => Both{Nest: [][]int{$a.Ints, $b.Ints}, Pairs: [](int, []int){$a.Pair, $b.Pair}}
rule read Out = "ab" t:two "!" => Out{A: Wrap{L: $t.Nest[0]}, B: WrapS{L: []string{}}, C: WrapL{L: []Leaf{}}, D: WrapN{L: $t.Nest}, E: WrapP{P: $t.Pairs[1]}, F: Wrap{L: $t.Nest[1]}, G: WrapN{L: [][]int{}}, H: WrapP{P: (0, $t.Nest[0])}}
@start
rule main Out = "xx" m:read => $m`

// fieldRangeCases read the lists and tuples of struct fields in actions.
func fieldRangeCases() []gentest.Case {
	return []gentest.Case{
		{Name: "range of a value read from a field", Src: fieldRangeSource,
			Inputs: []string{"xxab <>|<>!", "xxab [ab]|<>!", "xxab <>|[ab]!", "xxab [ab]|[ab]!", "xxab <>|<>", ""}},
	}
}

func TestGeneratedFieldRanges(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedFieldRanges)
}

func testGeneratedFieldRanges(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	cases := fieldRangeCases()
	for _, c := range cases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		code, err := gentest.GenerateCode(t, g, codegen.GenOptions{Package: "p", Start: "main", Types: true})
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if !strings.Contains(string(code), "TParse(trules") {
			t.Errorf("%s: the typed library does not build the values", c.Name)
		}
		if !strings.Contains(string(code), "zpr_") {
			t.Errorf("%s: the structs keep no range of the lists and tuples that actions read", c.Name)
		}
	}
	// A list that no action reads keeps no range, so the structs of such a parser are as they were.
	g, err := parseTest("type A = { L []int; P (int, int) }\ntype Out = { N int }\nrule main Out = a:one => Out{N: 1}\nrule one A = \"a\" => A{L: []int{1}, P: (1, 2)}")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gentest.GenerateCode(t, g, codegen.GenOptions{Package: "p", Start: "main", Types: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(code), "TParse(trules") || strings.Contains(string(code), "zpr_") {
		t.Errorf("a parser whose actions read no field keeps ranges, or does not run the typed library")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.GeneratedParsersCorpus(t, goBin, cases, codegen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{ConvertTypes: true}, len(cases))
	if node, err := exec.LookPath("node"); err == nil {
		gentest.TSParsersCorpus(t, node, cases, codegen.GenOptions{})
	}
}
