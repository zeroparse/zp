package codegen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestGenerateStreamOptions(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGenerateStreamOptions)
}

func testGenerateStreamOptions(t *testing.T) {
	g, err := parseTest(`rule main = item* #stream $$
rule item = "a"
rule other = "b"`)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := gentest.GenerateCode(t, g, codegen.GenOptions{Package: "fixture", Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"stream:", "streamInput", "func ParseStream", "\"bufio\"", "WithDiagnostic", "func Iterate", "StreamIterator"} {
		if strings.Contains(string(plain), marker) {
			t.Errorf("ordinary output contains %s", marker)
		}
	}
	code, err := gentest.GenerateCode(t, g, codegen.GenOptions{Package: "fixture", Start: "main", Stream: true})
	if err != nil || !strings.Contains(string(code), "func ParseStream(") || strings.Contains(string(code), "stream:") {
		t.Fatalf("stream output: %v", err)
	}
	for _, tc := range []struct {
		opts codegen.GenOptions
		want string
	}{
		{codegen.GenOptions{Start: "other", Stream: true}, "needs a #stream repetition"},
	} {
		tc.opts.Package = "fixture"
		if _, err := gentest.GenerateCode(t, g, tc.opts); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: %v, want %q", tc.opts, err, tc.want)
		}
	}
	if _, err := gentest.GenerateTSBundle(g, codegen.GenOptions{Start: "main", Stream: true}); err == nil {
		t.Error("TypeScript accepted stream generation")
	}
	typed, err := gentest.GenerateCode(t, g, codegen.GenOptions{Package: "fixture", Start: "main", Stream: true, Types: true})
	if err != nil || !strings.Contains(string(typed), "func ParseStreamValue(") || !strings.Contains(string(typed), "type StreamValue = ") {
		t.Fatalf("typed stream output: %v", err)
	}
	if typed, err := gentest.GenerateCode(t, g, codegen.GenOptions{Package: "fixture", Start: "main", Types: true}); err != nil || strings.Contains(string(typed), "ParseStreamValue") {
		t.Fatalf("typed output without streams: %v", err)
	}
}

// TestGenerateStreamLeftRecursionRejected checks that generation reports the compile error of a
// #stream rule on a left-recursion cycle for every target, with or without stream entry points.
func TestGenerateStreamLeftRecursionRejected(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGenerateStreamLeftRecursionRejected)
}

func testGenerateStreamLeftRecursionRejected(t *testing.T) {
	const want = "#stream cannot be used in rule main, which is on a left-recursion cycle"
	g, err := parseTest("rule main = item* #stream\nrule item = \"a\" \"\\n\" / main")
	if err != nil {
		t.Fatal(err)
	}
	for name, generate := range map[string]func() error{
		"Go": func() error {
			_, err := gentest.GenerateCode(t, g, codegen.GenOptions{Start: "main", Package: "test"})
			return err
		},
		"Go stream": func() error {
			_, err := gentest.GenerateCode(t, g, codegen.GenOptions{Start: "main", Package: "test", Stream: true})
			return err
		},
		"TS": func() error { _, err := gentest.GenerateTSBundle(g, codegen.GenOptions{Start: "main"}); return err },
	} {
		if err := generate(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s generation: %v", name, err)
		}
	}
}
