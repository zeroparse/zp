package language_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax"
)

// The generated source parser, with its adapter, must build the grammar that
// the hand-written parser builds, and accept the sources it accepts. The two
// are compared by their JSON form: every field of the AST that is part of the
// grammar and of its source metadata. The raw comment tokens that only the
// formatter reads are not in it.

// adaptCanonical parses src with the canonical grammar and builds the public AST from the tree.
func adaptCanonical(p *zp.Parser, src string) (*grammar.Grammar, error) {
	n, err := p.Parse(src)
	if err != nil {
		return nil, err
	}
	return syntax.FromSourceTree(src, n)
}

// grammarJSON returns the JSON form of g, indented one value to a line, and
// then the Go type of every node the JSON does not name: the JSON of two nodes
// that hold the same fields, like #atomic and #stream, is the same.
func grammarJSON(t *testing.T, g *grammar.Grammar) []string {
	t.Helper()
	b, err := json.MarshalIndent(g, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	lines = append(lines, "node types:")
	return nodeTypes(lines, "", reflect.ValueOf(g))
}

// nodeTypes appends the dynamic type of each interface value under v, with its
// path, to lines. It visits the fields that the JSON form holds.
func nodeTypes(lines []string, path string, v reflect.Value) []string {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			lines = nodeTypes(lines, path, v.Elem())
		}
	case reflect.Interface:
		if !v.IsNil() {
			lines = append(lines, path+" "+v.Elem().Type().String())
			lines = nodeTypes(lines, path, v.Elem())
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			lines = nodeTypes(lines, fmt.Sprintf("%s[%d]", path, i), v.Index(i))
		}
	case reflect.Map:
		keys := v.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(a.String(), b.String()) })
		for _, k := range keys {
			lines = nodeTypes(lines, path+"."+k.String(), v.MapIndex(k))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if f.IsExported() && f.Tag.Get("json") != "-" {
				lines = nodeTypes(lines, path+"."+f.Name, v.Field(i))
			}
		}
	}
	return lines
}

// firstDifference describes where two JSON texts differ.
func firstDifference(want, got []string) string {
	for i := 0; i < len(want) || i < len(got); i++ {
		var w, g string
		if i < len(want) {
			w = want[i]
		}
		if i < len(got) {
			g = got[i]
		}
		if w != g {
			lo := max(0, i-8)
			return fmt.Sprintf("line %d\n context:\n%s\n manual:    %s\n canonical: %s", i+1, strings.Join(want[lo:i], "\n"), w, g)
		}
	}
	return ""
}

// grammarDiff describes how the hand-written parser and the canonical grammar
// with its adapter disagree on src: one accepts it and the other does not, or
// both accept it and build different ASTs. It returns "" when they agree. Where
// both reject src, only the rejection is compared.
func grammarDiff(t *testing.T, p *zp.Parser, src string) string {
	t.Helper()
	manual, merr := zp.ParseGrammar(src)
	got, err := adaptCanonical(p, src)
	switch {
	case merr != nil && err != nil:
		return ""
	case merr != nil:
		return fmt.Sprintf("the hand-written parser rejects a source the canonical frontend accepts: %v", merr)
	case err != nil:
		return fmt.Sprintf("the canonical frontend rejects a source the hand-written parser accepts: %v", err)
	}
	if d := firstDifference(grammarJSON(t, manual), grammarJSON(t, got)); d != "" {
		return "the grammars differ at " + d
	}
	return ""
}

// compareGrammars reports whether the frontends agree on src, as grammarDiff
// tells.
func compareGrammars(t *testing.T, p *zp.Parser, name, src string) bool {
	t.Helper()
	if d := grammarDiff(t, p, src); d != "" {
		t.Errorf("%s: %s", name, d)
		return false
	}
	return true
}

// frontendRejections are sources that both frontends reject: the hand-written
// parser is the reference, so the canonical frontend must not accept more.
var frontendRejections = []string{
	"rule a = x /... y\n",                // "..." is the default action marker, not three of ".".
	"rule a = b #recover(skip=a[-z]+)\n", // a class right after a name is an index.
	"rule a = (b)[a]\n",
	"rule a = x:y[a]\n",
	"rule a node = \"x\"\n",
	"type T = { A node }\n",
	"rule a = b => 1 + nil(2)\n",
	"rule a = b => true{}\n",
	"rule a = \"0\"_[oO]\n",  // the lexer reads _ after a quote as a name, so [ is an index.
	"rule a = \"x\"i[a-z]\n", // the suffix i is a name to the lexer too.
	"rule a = \"x\"in[a]\n",
	"rule a = 1a[a]\n",
}

func TestCanonicalFrontendRejections(t *testing.T) {
	p, _ := sourceGrammar(t)
	for _, src := range frontendRejections {
		if _, err := zp.ParseGrammar(src); err == nil {
			t.Errorf("%q: the hand-written parser accepts it", src)
		}
		if _, err := adaptCanonical(p, src); err == nil {
			t.Errorf("%q: the canonical frontend accepts it", src)
		}
	}
}

// frontendAgreements are sources that both frontends accept with the same AST,
// where the grammar could read them differently.
var frontendAgreements = []string{
	"rule a = b @foo(\"\"x, k=y)\n", // an attribute argument is a whole expression: a sequence here.
	"rule a = b @foo(\"x\" / y, \"z\"i)\n",
	"rule a = b => nil\nrule c = d => true\n",
	"rule a = \"!\" _|_[0-9] / x\n",       // _|_ is a token, not a name
	"rule a = \"x\" [a-z] \"y\"i [0-9]\n", // a space before a class keeps it a class.
}

func TestCanonicalFrontendAgreements(t *testing.T) {
	p, _ := sourceGrammar(t)
	for _, src := range frontendAgreements {
		if _, err := zp.ParseGrammar(src); err != nil {
			t.Errorf("%q: the hand-written parser rejects it: %v", src, err)
		}
		compareGrammars(t, p, src, src)
	}
}
