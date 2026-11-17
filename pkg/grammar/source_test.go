package grammar_test

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"zeroparse.dev/zp/pkg/grammar"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

const src = "rule main = \"a\"+\n"

func jsonOf(t *testing.T) []byte {
	t.Helper()
	g, err := grammar.Load(grammar.FromString("g.zp", src))
	if err != nil {
		t.Fatal(err)
	}
	data, err := ast.MarshalJSON(g)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func loadedRules(t *testing.T, s grammar.Source) []string {
	t.Helper()
	g, err := grammar.Load(s)
	if err != nil {
		t.Fatalf("%s: %v", s.Name(), err)
	}
	var names []string
	for _, r := range g.Rules() {
		names = append(names, r.Name)
	}
	return names
}

func TestLoadDecidesTheEncoding(t *testing.T) {
	data := jsonOf(t)
	fsys := fstest.MapFS{"dir/g.zp": {Data: []byte(src)}, "dir/g.json": {Data: data}, "dir/g.txt": {Data: data}}
	for _, s := range []grammar.Source{
		grammar.FromString("g.zp", src),
		grammar.FromString("", src),
		grammar.FromBytes("g.json", data),
		grammar.FromBytes("", data),                              // a leading brace means JSON
		grammar.FromBytes("g", append([]byte(" \n"), data...)),   // after white space
		grammar.FromString("g.json", src).As(grammar.EncodingZP), // the explicit encoding wins
		grammar.FromReader("g.zp", strings.NewReader(src)),
		grammar.FromFS(fsys, "dir/g.zp"),
		grammar.FromFS(fsys, "dir/g.json"),
		grammar.FromFS(fsys, "dir/g.txt"),
	} {
		if got := loadedRules(t, s); len(got) != 1 || got[0] != "main" {
			t.Errorf("%s: rules %v", s.Name(), got)
		}
	}
	// The extension decides before the content.
	if _, err := grammar.Load(grammar.FromBytes("g.zp", data)); err == nil {
		t.Error("JSON named .zp loaded")
	}
	if _, err := grammar.Load(grammar.FromString("g", src).As(grammar.EncodingJSON)); err == nil {
		t.Error("grammar syntax loaded as JSON")
	}
}

func TestSourceNameAndEncoding(t *testing.T) {
	s := grammar.FromFS(fstest.MapFS{}, "a/b.zp")
	if s.Name() != "a/b.zp" || s.Encoding() != grammar.EncodingAuto || s.As(grammar.EncodingJSON).Encoding() != grammar.EncodingJSON {
		t.Fatalf("name %q, encoding %v", s.Name(), s.Encoding())
	}
	for e, want := range map[grammar.Encoding]string{grammar.EncodingAuto: "auto", grammar.EncodingZP: "zp", grammar.EncodingJSON: "json", 7: "Encoding(7)"} {
		if e.String() != want {
			t.Errorf("%d: %q, want %q", int(e), e.String(), want)
		}
	}
}

func TestLoadErrors(t *testing.T) {
	if _, err := grammar.Load(grammar.FromBytes("g.zpc", []byte("ZPC\x00rest"))); !errors.Is(err, grammar.ErrCompiled) {
		t.Errorf("compiled grammar: %v", err)
	}
	if _, err := grammar.Load(grammar.FromFS(fstest.MapFS{}, "missing.zp")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: %v", err)
	}
	_, err := grammar.Load(grammar.FromString("g.zp", "rule main = (\n"))
	if err == nil || strings.Contains(err.Error(), "g.zp") || !strings.HasPrefix(err.Error(), "1:") && !strings.HasPrefix(err.Error(), "2:") {
		t.Errorf("syntax error without the name, with its position: %v", err)
	}
	if g, err := grammar.Load(grammar.Source{}); err != nil || len(g.Rules()) != 0 {
		t.Errorf("the empty source: %v", err)
	}
}

func TestConvert(t *testing.T) {
	data, err := grammar.Convert(grammar.FromString("g.zp", src), grammar.EncodingJSON)
	if err != nil || string(data) != string(jsonOf(t)) {
		t.Fatalf("to JSON: %s, %v", data, err)
	}
	back, err := grammar.Convert(grammar.FromBytes("g.json", data), grammar.EncodingZP)
	if err != nil || string(back) != src {
		t.Fatalf("to grammar syntax: %q, %v", back, err)
	}
	if _, err := grammar.Convert(grammar.FromString("g.zp", src), grammar.EncodingAuto); err == nil {
		t.Error("converted to EncodingAuto")
	}
}

func TestValidateAndFormat(t *testing.T) {
	g := &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Expr: &ast.Literal{Value: "a"}}}}}
	if err := grammar.Validate(g); err != nil {
		t.Fatal(err)
	}
	if got := grammar.Format(g); got != "rule main = \"a\"\n" {
		t.Errorf("Format: %q", got)
	}
	if err := grammar.Validate(&ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main"}}}}); err == nil {
		t.Error("a rule without a body validated")
	}
}
