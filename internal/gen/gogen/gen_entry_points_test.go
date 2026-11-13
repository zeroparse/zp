package gogen_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// singleCharLiteral matches a literal of one character, which the negative control changes.
var singleCharLiteral = regexp.MustCompile(`"[^"\\]"`)

// entryCombination is a selection of the entry points of a generated Go parser.
type entryCombination struct {
	name string
	opts gen.GenOptions
	// The entry points the selection generates.
	parse, typed, recognize, document bool
}

var entryCombinations = []entryCombination{
	{name: "tree", parse: true},
	{name: "tree_recognize", opts: gen.GenOptions{Recognize: true}, parse: true, recognize: true},
	{name: "recognize_only", opts: gen.GenOptions{RecognizeOnly: true}, recognize: true},
	{name: "types", opts: gen.GenOptions{Types: true}, parse: true, typed: true},
	{name: "types_recognize", opts: gen.GenOptions{Types: true, Recognize: true}, parse: true, typed: true, recognize: true},
	{name: "typed_only", opts: gen.GenOptions{Types: true, TypedOnly: true}, typed: true},
	{name: "typed_only_recognize", opts: gen.GenOptions{Types: true, TypedOnly: true, Recognize: true}, typed: true, recognize: true},
	{name: "types_recognize_document", opts: gen.GenOptions{Types: true, Recognize: true, Document: true}, parse: true, typed: true, recognize: true, document: true},
}

// entryTranscript is the test program of a combination: it prints, for each input, the outcome of
// every entry point the combination has, one line per entry point, so that the lines of one entry
// point can be compared across combinations.
func entryTranscript(c entryCombination) string {
	var b strings.Builder
	b.WriteString(`package PKG

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func errText(err error) string {
	if err == nil {
		return "ok"
	}
	e := err.Error()
	var se *SyntaxError
	if errors.As(err, &se) {
		e += fmt.Sprintf(" @%d:%d:%d", se.Line, se.Col, se.Pos)
	}
	return e
}

var _ = json.Marshal

func TestTranscript(t *testing.T) {
	var b strings.Builder
	for _, in := range INPUTS {
`)
	if c.parse {
		b.WriteString("\t\t{\n\t\t\t_, err := Parse(in)\n\t\t\tfmt.Fprintf(&b, \"parse %q: %s\\n\", in, errText(err))\n\t\t}\n")
		b.WriteString("\t\t{\n\t\t\t_, err := ParseRule(\"main\", in)\n\t\t\tfmt.Fprintf(&b, \"parse-rule %q: %s\\n\", in, errText(err))\n\t\t}\n")
	}
	if c.typed {
		b.WriteString("\t\t{\n\t\t\tv, err := ParseValue(in)\n\t\t\tj, _ := json.Marshal(v)\n\t\t\tfmt.Fprintf(&b, \"ast %q: %s | %s\\n\", in, j, errText(err))\n\t\t}\n")
	}
	if c.recognize {
		b.WriteString("\t\t{\n\t\t\terr := Recognize(in)\n\t\t\tfmt.Fprintf(&b, \"recognize %q: %s\\n\", in, errText(err))\n\t\t}\n")
	}
	if c.document {
		b.WriteString("\t\t{\n\t\t\td, err := NewDocument(in)\n\t\t\tfmt.Fprintf(&b, \"document %q: %s %v\\n\", in, errText(err), d != nil)\n\t\t}\n")
	}
	b.WriteString(`	}
	if err := os.WriteFile(filepath.Join(os.Getenv("TRANSCRIPT_DIR"), "PKG.txt"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
`)
	return b.String()
}

// entryLines returns the lines of a transcript that belong to the entry point.
func entryLines(transcript, entry string) string {
	var b strings.Builder
	for _, l := range strings.Split(transcript, "\n") {
		if strings.HasPrefix(l, entry+" ") {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

// testEntryCombinations generates each combination for each grammar, builds it, and checks that
// the entry points it has give what the same entry points of the full parser give, and that it has
// no other entry point.
func testEntryCombinations(t *testing.T, cases []gentest.Case) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gentest.GoMod("ordtest")), 0o644); err != nil {
				t.Fatal(err)
			}
			var pkgs []string
			for _, c := range entryCombinations {
				gentest.OrdinaryOnlyPackage(t, dir, c.name, tc.Src, c.opts, entryTranscript(c), tc.Inputs)
				pkgs = append(pkgs, c.name)
				src, err := os.ReadFile(filepath.Join(dir, c.name, "parser.go"))
				if err != nil {
					t.Fatal(err)
				}
				for _, e := range []struct {
					fn  string
					has bool
				}{{"func Parse(", c.parse}, {"func ParseRule(", c.parse}, {"func ParseValue(", c.typed}, {"func Recognize(", c.recognize}, {"func NewDocument(", c.document}} {
					if got := strings.Contains(string(src), e.fn); got != e.has {
						t.Errorf("%s: %q generated = %v, want %v", c.name, e.fn, got, e.has)
					}
				}
			}
			// A mutated grammar is a negative control: the comparison must be able to fail.
			gentest.OrdinaryOnlyPackage(t, dir, "mutated", singleCharLiteral.ReplaceAllString(tc.Src, `"?"`), gen.GenOptions{Types: true, Recognize: true}, entryTranscript(entryCombinations[4]), tc.Inputs)
			out := gentest.RunTranscripts(t, dir, append(pkgs, "mutated")...)
			ref := out["types_recognize_document"]
			for _, c := range entryCombinations {
				for _, e := range []struct {
					name string
					has  bool
				}{{"parse", c.parse}, {"parse-rule", c.parse}, {"ast", c.typed}, {"recognize", c.recognize}, {"document", c.document}} {
					if !e.has {
						continue
					}
					want, got := entryLines(ref, e.name), entryLines(out[c.name], e.name)
					if got != want {
						t.Errorf("%s: %s differs from the full parser:\n%s", c.name, e.name, gentest.FirstTranscriptDifference(want, got))
					}
				}
			}
			if out["mutated"] == out["types_recognize"] {
				t.Error("the transcripts of a mutated grammar are the same: the comparison cannot fail")
			}
			if !strings.Contains(entryLines(ref, "recognize"), "syntax error") || !strings.Contains(entryLines(ref, "recognize"), ": ok") {
				t.Errorf("the corpus has no accepted and rejected input:\n%s", entryLines(ref, "recognize"))
			}
		})
	}
}

func TestGeneratedEntryPointCombinations(t *testing.T) {
	t.Parallel()
	testEntryCombinations(t, []gentest.Case{
		{Name: "converted", Src: gentest.OrdinaryOnlyConverted, Inputs: gentest.OrdinaryOnlyConvertedInputs},
		{Name: "hidden", Src: ordinaryOnlyHidden, Inputs: ordinaryOnlyHiddenInputs},
	})
}

// The combinations that no generated parser can have fail generation with an error that names why.
func TestGeneratedEntryPointRejections(t *testing.T) {
	t.Parallel()
	g, err := parseTest(gentest.OrdinaryOnlyConverted)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		opts gen.GenOptions
		want string
	}{
		{gen.GenOptions{Types: true, TypedOnly: true, Document: true}, "cannot have a Document or a stream"},
		{gen.GenOptions{Types: true, TypedOnly: true, Stream: true}, "cannot have a Document or a stream"},
		{gen.GenOptions{TypedOnly: true}, "needs typed output"},
		{gen.GenOptions{RecognizeOnly: true, Types: true}, "recognize-only output generates only Recognize"},
		{gen.GenOptions{RecognizeOnly: true, TypedOnly: true, Types: true}, "recognize-only output generates only Recognize"},
		{gen.GenOptions{RecognizeOnly: true, Document: true}, "recognize-only output generates only Recognize"},
		{gen.GenOptions{RecognizeOnly: true, Stream: true}, "recognize-only output generates only Recognize"},
	} {
		tc.opts.Package, tc.opts.Module, tc.opts.Start, tc.opts.Runtime = "p", "example.com/p", "main", gen.RuntimeImport
		_, err := gentest.Generate(t, g, tc.opts)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: error %v, want one containing %q", tc.opts, err, tc.want)
		}
	}
}
