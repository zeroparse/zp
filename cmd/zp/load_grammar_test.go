package main

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadGrammarReportsThePath checks that the errors of loading a grammar name the file as the
// user gave it.
func TestLoadGrammarReportsThePath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "sub", "missing.zp")
	_, err := loadGrammar(missing, "lint")
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) || pathErr.Path != missing || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: %v", err)
	}
	bad := writeFile(t, "bad.zp", "rule main = (\n")
	if _, err := loadGrammar(bad, "lint"); err == nil || !strings.HasPrefix(err.Error(), bad+":") {
		t.Errorf("syntax error: %v", err)
	}
	compiled := writeFile(t, "g.zpc", "ZPC\x00")
	if _, err := loadGrammar(compiled, "lint"); err == nil || err.Error() != compiledNeedsSource(compiled, "lint").Error() {
		t.Errorf("compiled grammar: %v", err)
	}
}

// TestLoadGrammarDetectsJSON checks that a grammar file whose name has no known extension is
// decided by its content.
func TestLoadGrammarDetectsJSON(t *testing.T) {
	src := writeFile(t, "g.zp", "rule main = \"a\"\n")
	var out strings.Builder
	if err := convertCmd([]string{"-to", "json", src}, &out); err != nil {
		t.Fatal(err)
	}
	g, err := loadGrammar(writeFile(t, "grammar", out.String()), "lint")
	if err != nil || len(g.Rules()) != 1 {
		t.Fatalf("JSON without an extension: %v", err)
	}
	// Without -to, convert writes the other encoding of the one Load decides.
	var back strings.Builder
	if err := convertCmd([]string{writeFile(t, "grammar2", out.String())}, &back); err != nil || back.String() != "rule main = \"a\"\n" {
		t.Errorf("convert without -to: %q, %v", back.String(), err)
	}
	compiled := writeFile(t, "g.zpc", "ZPC\x00")
	if err := convertCmd([]string{compiled}, &back); err == nil || err.Error() != compiledNeedsSource(compiled, "convert").Error() {
		t.Errorf("compiled grammar: %v", err)
	}
}
