package gengo_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// transferBody is a struct whose fields of node, named node, join and list types actions read back; its
// joins and lists hold Error values where recovery skipped input.
const transferBody = `type N = node[string]
type P = node[(N, N)]
type Word = { Name node[string] }
type Item ~ Word | N | P | error
type Body = { Name node[string]; K N; Pair P; Item Item; Items []Item; Opt *Item; Ws []Word }
rule body Body = nm:([a-z] #atomic) "=" k:n p:pr i:item "[" is:item* "]" o:("?" x:item => $x)? ws:word* => Body{Name: $nm, K: $k, Pair: $p, Item: $i, Items: $is.Value, Opt: $o, Ws: $ws.Value}
rule item Item = it #recover(skip=[0-9]+)
rule it Item = word / n / pr
rule word Word = "w" x:([a-z] #atomic) => Word{Name: $x}
rule n N = ("n" [a-z]) #atomic
rule pr P = "<" t:(n n) ">" => P{$t}
`

// transferInputs reach every field with values, without the optional ones, and with Error values in
// the joins and lists.
var transferInputs = []string{"a=na<nbnc>wx[]", "a=na<nbnc>wx[nd<nenf>wy]?nzwawb", "a=na<nbnc>12[3wx45]?6", "a=na<nbnc>wx[", "a=", ""}

// transferCases are the grammars of TestGeneratedTypedTransfers: those whose actions only carry
// field values into fields of the same type (direct), and those with another read of such a field
// (converted).
func transferCases() (direct, converted []gentest.Case) {
	direct = []gentest.Case{
		{Name: "copies", Src: transferBody + `type Out = { Name node[string]; K N; Pair P; Item Item; Items []Item; Opt *Item; Ws []Word; Span Body }
rule main Out = b:body $$ => Out{Name: $b.Name, K: $b.K, Pair: $b.Pair, Item: $b.Item, Items: $b.Items, Opt: $b.Opt, Ws: $b.Ws, Span: $b}`},
		{Name: "list builders", Src: transferBody + `type Out = { All []Item; Opts []*Item; Names []node[string]; Ks []N }
rule main Out = b:body $$ => Out{All: concat($b.Items, list($b.Item), map(list($b), (x) => x.Item)), Opts: list($b.Opt, (($b.Opt))), Names: map($b.Ws, (w) => w.Name), Ks: concat(list($b.K), list())}`},
		{Name: "nested constructors", Src: transferBody + `type Inner = { Item Item; Ws []Word }
type Out = { In Inner; More []Inner }
rule main Out = b:body $$ => Out{In: Inner{Item: $b.Item, Ws: $b.Ws}, More: list(Inner{Item: $b.Item, Ws: list()})}`},
	}
	converted = []gentest.Case{
		{Name: "text of a node field", Src: transferBody + `type Out = { Name node[string]; T string }
rule main Out = b:body $$ => Out{Name: $b.Name, T: text($b.Name)}`},
		{Name: "length of a list field", Src: transferBody + `type Out = { Items []Item; Count int }
rule main Out = b:body $$ => Out{Items: $b.Items, Count: len($b.Items)}`},
		{Name: "copy into another type", Src: transferBody + `type Out = { K node[string] }
rule main Out = b:body $$ => Out{K: $b.K}`},
		{Name: "cast of a join field", Src: transferBody + `type Out = { Item Item }
rule main Out = b:body $$ => Out{Item: $b.Opt.(Item)}`},
		{Name: "join in a predicate", Src: transferBody + `type Out = { Item Item }
rule main Out = b:body {$b.Item != nil} $$ => Out{Item: $b.Item}`},
	}
	for _, cs := range [][]gentest.Case{direct, converted} {
		for i := range cs {
			cs[i].Inputs = transferInputs
		}
	}
	return direct, converted
}

// sourceGrammarCase is the grammar of the language, whose actions copy the fields of the struct values
// of declaration bodies, on grammars of the repository, on a copy of one in which every fifth line ends
// with a stray parenthesis, so that the parse recovers, and on attribute arguments.
func sourceGrammarCase(t *testing.T) gentest.Case {
	t.Helper()
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join("..", "..", "..", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	c := gentest.Case{Name: "source grammar", Src: read("zp.zp")}
	for _, name := range []string{"testdata/grammars/json/json.zp", "testdata/grammars/csv/csv.zp", "testdata/grammars/xml/xml.zp"} {
		c.Inputs = append(c.Inputs, read(name))
	}
	lines := strings.Split(read("testdata/grammars/xml/xml.zp"), "\n")
	for i := 4; i < len(lines); i += 5 {
		lines[i] += ")"
	}
	// Positional attribute arguments of named node types, and an omitted one.
	c.Inputs = append(c.Inputs, strings.Join(lines, "\n"), "package p\n@a(1, \"s\", true, false, nil, , x y)\nrule r = \"x\"\n")
	return c
}

// TestGeneratedTypedTransfers checks that a generated typed parser builds its values directly when its
// actions read fields of node, named node, join and list types only to carry them into fields of the
// same type, as those of the grammar of the language do, and converts the nodes of Parse when an action
// reads such a field otherwise. Both give what the conversion of the nodes gives, and what the engine
// gives.
func TestGeneratedTypedTransfers(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedTypedTransfers)
}

func testGeneratedTypedTransfers(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skip("go command not found")
	}
	direct, converted := transferCases()
	direct = append(direct, sourceGrammarCase(t))
	isDirect := map[string]bool{}
	for _, c := range direct {
		isDirect[c.Name] = true
	}
	for _, c := range append(append([]gentest.Case(nil), direct...), converted...) {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		code, err := gentest.GenerateCode(t, g, gen.GenOptions{Package: "p", Start: "main", Types: true})
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		typed := strings.Contains(string(code), "return rt.TParse(trules[") && !strings.Contains(string(code), "rt.RunParseConverted(")
		if typed != isDirect[c.Name] {
			t.Errorf("%s: generated typed Go uses the typed runtime: %v, want %v", c.Name, typed, isDirect[c.Name])
		}
	}
	gentest.GeneratedTypesCorpus(t, goBin, append(direct, converted...), gen.GenOptions{ConvertTypes: true}, len(direct))
}
