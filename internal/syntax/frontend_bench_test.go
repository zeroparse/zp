package syntax

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/syntax/genparser"
)

// frontendSample is a grammar source of the frontend benchmark. A malformed sample has syntax errors,
// which both frontends must report.
type frontendSample struct {
	name, src string
	malformed bool
}

// frontendSamples returns the sources of BenchmarkFrontend: a one-line grammar, a comment-heavy
// one, the grammar of the language, the grammars of testdata/grammars (those of the ready-made
// parsers) and a malformed copy of the Go grammar, in which every tenth rule line ends with a stray
// parenthesis, so that the parsers recover.
func frontendSamples(tb testing.TB) []frontendSample {
	tb.Helper()
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			tb.Fatal(err)
		}
		return string(b)
	}
	var comments strings.Builder
	for i := 0; i < 64; i++ {
		fmt.Fprintf(&comments, "// detached %d\n\n// documentation %d\nrule r%d = \"x\" // trailing %d\n/* block %d */\n\n", i, i, i, i, i)
	}
	samples := []frontendSample{
		{name: "small", src: `rule main = [a-z]+ #atomic`},
		{name: "comments", src: comments.String()},
		{name: "zp", src: read("../../zp.zp")},
	}
	paths, err := filepath.Glob("../../testdata/grammars/*/*.zp")
	if err != nil || len(paths) == 0 {
		tb.Fatalf("no grammars in testdata/grammars: %v", err)
	}
	for _, p := range paths {
		samples = append(samples, frontendSample{name: strings.TrimSuffix(filepath.Base(p), ".zp"), src: read(p)})
	}
	lines := strings.SplitAfter(read("../../testdata/grammars/golang/golang.zp"), "\n")
	for i, rules := 0, 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "rule ") {
			if rules%10 == 0 {
				lines[i] = strings.TrimSuffix(lines[i], "\n") + " )\n"
			}
			rules++
		}
	}
	return append(samples, frontendSample{name: "malformed", src: strings.Join(lines, ""), malformed: true})
}

// frontends are the grammar-source frontends that BenchmarkFrontend compares. manual is the
// hand-written lexer and parser and manual-aa the same function under another name, whose spread
// against manual is the noise of a run. generated is ParseAST of the generated parser followed by
// FromSourceTree, which builds the same grammar as manual.
var frontends = []struct {
	name  string
	parse func(src string) (any, error)
}{
	{"manual", func(src string) (any, error) { return Parse(src) }},
	{"manual-aa", func(src string) (any, error) { return Parse(src) }},
	{"generated", parseGenerated},
	{"generated-exp", func(src string) (any, error) { return parseGeneratedWith(src, genparser.WithExpectations()) }},
	{"manual-partial", parsePartialManual},
	{"generated-partial", func(src string) (any, error) { return parsePartialGenerated(src) }},
	{"generated-partial-exp", func(src string) (any, error) { return parsePartialGenerated(src, genparser.WithExpectations()) }},
}

// parsePartialManual is ParsePartial, with its errors as the error.
func parsePartialManual(src string) (any, error) {
	g, errs := ParsePartial(src)
	if len(errs) > 0 {
		return g, errs
	}
	return g, nil
}

// parsePartialGenerated parses src as ParsePartial does with the generated parser: the syntax
// errors of the parse, then the declarations that are complete and the errors of the checks.
func parsePartialGenerated(src string, opts ...genparser.ParseOption) (any, error) {
	text, _ := WithoutInvalidCharacters(src)
	f, err := genparser.ParseAST(text, opts...)
	var errs ErrorList
	if err != nil {
		var list genparser.SyntaxErrors
		var one *genparser.SyntaxError
		var out []SourceError
		switch {
		case errors.As(err, &list):
			for _, e := range list {
				out = append(out, SourceError{Offset: e.Pos, Msg: e.Message()})
			}
		case errors.As(err, &one):
			out = append(out, SourceError{Offset: one.Pos, Msg: one.Message()})
		default:
			return nil, err
		}
		errs = SourceErrors(src, out)
	}
	g, checks := FromPartialSourceTree(src, f)
	errs = append(errs, checks...)
	if len(errs) > 0 {
		return g, errs
	}
	return g, nil
}

// parseGenerated parses src by the generated parser and its adapter.
func parseGenerated(src string) (any, error) { return parseGeneratedWith(src) }

func parseGeneratedWith(src string, opts ...genparser.ParseOption) (any, error) {
	text, invalid := WithoutInvalidCharacters(src)
	f, err := genparser.ParseAST(text, opts...)
	if err != nil {
		return nil, err
	}
	if invalid {
		return nil, SourceErrors(src, nil)
	}
	return FromSourceTree(src, f)
}

var frontendSink any

// BenchmarkFrontend measures parsing a grammar source by each frontend: time, bytes and allocations
// per source. Compilation is not part of it. bench/sourceparser.sh runs it in interleaved rounds.
func BenchmarkFrontend(b *testing.B) {
	for _, s := range frontendSamples(b) {
		b.Run("src="+s.name, func(b *testing.B) {
			for _, f := range frontends {
				b.Run("fe="+f.name, func(b *testing.B) {
					b.SetBytes(int64(len(s.src)))
					b.ReportAllocs()
					for b.Loop() {
						v, err := f.parse(s.src)
						if (err != nil) != s.malformed {
							b.Fatalf("error %v, want one: %v", err, s.malformed)
						}
						frontendSink = v
					}
				})
			}
		})
	}
}

// TestFrontendSamples requires that both frontends accept every valid sample and reject the
// malformed one, so that the benchmark measures complete parses.
func TestFrontendSamples(t *testing.T) {
	for _, s := range frontendSamples(t) {
		for _, f := range frontends {
			if _, err := f.parse(s.src); (err != nil) != s.malformed {
				t.Errorf("%s by %s: error %v", s.name, f.name, err)
			}
		}
	}
}
