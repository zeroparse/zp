package language_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax"
	"zeroparse.dev/zp/internal/syntax/genparser"
)

// partialCanonical parses src as ParsePartial does, with the generated parser of the canonical
// grammar and the partial mode of its adapter: the declarations that are complete, and the
// syntax errors followed by the errors of the checks.
func partialCanonical(src string) (*grammar.Grammar, syntax.ErrorList) {
	text, _ := syntax.WithoutInvalidCharacters(src)
	f, err := genparser.ParseAST(text, genparser.WithExpectations())
	var errs []syntax.SourceError
	if err != nil {
		errs, _ = sourceErrorsOf(err)
	}
	list := syntax.SourceErrors(src, errs)
	g, checks := syntax.FromPartialSourceTree(src, f)
	return g, append(list, checks...)
}

// declJSON returns the JSON form of every declaration of g by its kind, name and start.
func declJSON(t *testing.T, g *grammar.Grammar) map[string]string {
	t.Helper()
	out := map[string]string{}
	if g == nil {
		return out
	}
	for _, d := range g.File.Decls {
		b, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		pos := reflect.Indirect(reflect.ValueOf(d)).FieldByName("Pos").Interface()
		key := fmt.Sprintf("%T@%v", d, pos)
		out[key] = string(b)
	}
	return out
}

// TestPartialSourceTree compares the partial parse of the canonical grammar with the partial
// parse of the hand-written parser on mutated grammars: it keeps at least the declarations the
// hand-written parser keeps, each equal to it, and reports the first error at the same place.
func TestPartialSourceTree(t *testing.T) {
	corpus := mutationCorpus(t)
	rng := rand.New(rand.NewSource(7))
	cases, rejected, fewer, firstErr, differ, manualKept, canonicalKept := 0, 0, 0, 0, 0, 0, 0
	for i := 0; i < 3000; i++ {
		s := mutatedSource(rng, corpus)
		cases++
		mg, merrs := syntax.ParsePartial(s)
		if len(merrs) == 0 {
			continue
		}
		rejected++
		gg, gerrs := partialCanonical(s)
		if len(gerrs) == 0 || gerrs[0].Pos != merrs[0].Pos {
			firstErr++
			if firstErr <= 5 {
				t.Logf("first error differs for %q: manual %v, canonical %v", s, merrs, gerrs)
			}
		}
		want, got := declJSON(t, mg), declJSON(t, gg)
		manualKept += len(want)
		canonicalKept += len(got)
		for k, w := range want {
			g, ok := got[k]
			if !ok {
				fewer++
				if fewer <= 5 {
					t.Logf("missing %s in %q (errors %v)", k, s, gerrs)
				}
				continue
			}
			if g != w {
				differ++
				if differ <= 5 {
					t.Logf("declaration %s differs in %q:\nmanual %s\ncanonical %s", k, s, w, g)
				}
			}
		}
	}
	t.Logf("declarations kept: manual %d, canonical %d", manualKept, canonicalKept)
	// The hand-written parser also keeps a declaration with an error in it (a string that is not
	// closed, text after the declaration); the generated parser leaves such a declaration out.
	// Bound the loss, so that a regression of the recovery shows.
	if canonicalKept*100 < manualKept*85 {
		t.Errorf("the canonical partial parse keeps %d declarations, the hand-written parser %d", canonicalKept, manualKept)
	}
	t.Logf("%d sources, %d rejected, %d declarations missing, %d differ, %d first errors differ", cases, rejected, fewer, differ, firstErr)
}

// TestPartialKeepsDeclarationsOfRepositoryGrammars breaks the end of every tenth rule of the
// grammars of the repository and requires the canonical partial parse to keep at least as many
// declarations as the hand-written parser, which is what an editor that shows a grammar with a
// mistake in one rule needs of the rest of the file.
func TestPartialKeepsDeclarationsOfRepositoryGrammars(t *testing.T) {
	files, err := filepath.Glob("../../testdata/grammars/*/*.zp")
	if err != nil || len(files) == 0 {
		t.Fatalf("grammars of the repository: %v, %v", files, err)
	}
	manualKept, canonicalKept := 0, 0
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.SplitAfter(string(b), "\n")
		rules := 0
		for i, line := range lines {
			if strings.HasPrefix(line, "rule ") {
				if rules%10 == 0 {
					lines[i] = strings.TrimSuffix(line, "\n") + " )\n"
				}
				rules++
			}
		}
		src := strings.Join(lines, "")
		mg, merrs := syntax.ParsePartial(src)
		gg, gerrs := partialCanonical(src)
		if len(merrs) > 0 && len(gerrs) == 0 {
			t.Errorf("%s: the canonical partial parse reports no error", file)
		}
		m, g := len(declJSON(t, mg)), len(declJSON(t, gg))
		if g < m {
			t.Errorf("%s: the canonical partial parse keeps %d declarations, the hand-written parser %d", file, g, m)
		}
		manualKept += m
		canonicalKept += g
	}
	t.Logf("%d grammars: declarations kept: manual %d, canonical %d", len(files), manualKept, canonicalKept)
}

// TestPartialKeepsDeclarationsWithInvalidEscapes requires a declaration with an unknown or invalid
// escape to be kept, with the error of the escape reported, as the hand-written parser does.
func TestPartialKeepsDeclarationsWithInvalidEscapes(t *testing.T) {
	for _, src := range []string{
		"rule a = \"\\x\"\nrule b = \"y\"\n",
		"rule a = [\\q]\nrule b = \"y\"\n",
		"rule a = \"\\u{110000}\"\nrule b = \"y\"\n",
		"rule a = x => \"\\z\"\nrule b = \"y\"\n",
	} {
		mg, merrs := syntax.ParsePartial(src)
		gg, gerrs := partialCanonical(src)
		if len(merrs) == 0 || len(gerrs) == 0 || merrs[0].Pos != gerrs[0].Pos || merrs[0].Msg != gerrs[0].Msg {
			t.Errorf("%q: errors: manual %v, canonical %v", src, merrs, gerrs)
		}
		if m, g := len(declJSON(t, mg)), len(declJSON(t, gg)); g != m || g != 2 {
			t.Errorf("%q: declarations kept: manual %d, canonical %d, want 2", src, m, g)
		}
	}
}
