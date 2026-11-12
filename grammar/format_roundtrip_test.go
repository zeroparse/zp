package grammar_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/grammar"
)

// TestFormatRoundTripThroughParseGrammar checks, through the public API only, that Format is the
// inverse of zp.ParseGrammar on the grammar AST: the printed text parses back to the same grammar
// (apart from source ranges) and printing it again gives the same text.
func TestFormatRoundTripThroughParseGrammar(t *testing.T) {
	var paths []string
	for _, pattern := range []string{"../testdata/grammars/*/*.zp", "../examples/*/*.zp", "../zp.zp"} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, m...)
	}
	if len(paths) < 10 {
		t.Fatalf("found only %d grammars", len(paths))
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			g, err := zp.ParseGrammar(string(src))
			if err != nil {
				t.Skipf("does not parse on its own: %v", err)
			}
			text := grammar.Format(g)
			g2, err := zp.ParseGrammar(text)
			if err != nil {
				t.Fatalf("printed grammar does not parse: %v", err)
			}
			if got, want := withoutSources(t, g2), withoutSources(t, g); got != want {
				t.Errorf("printed grammar parses back differently:\n got %s\nwant %s", got, want)
			}
			if again := grammar.Format(g2); again != text {
				t.Errorf("printing is not stable")
			}
		})
	}
}

// withoutSources returns the JSON form of g without source ranges (source and fields such as
// skipSource).
func withoutSources(t *testing.T, g *grammar.Grammar) string {
	t.Helper()
	data, err := grammar.MarshalJSON(g)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	var strip func(any)
	strip = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			for k, x := range v {
				if k == "source" || strings.HasSuffix(k, "Source") {
					delete(v, k)
				} else {
					strip(x)
				}
			}
		case []any:
			for _, x := range v {
				strip(x)
			}
		}
	}
	strip(v)
	data, err = json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
