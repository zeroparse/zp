package syntax

import (
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
)

func TestSourceErrorsMoveToTheNextToken(t *testing.T) {
	for _, c := range []struct {
		name, src string
		off       int
		want      grammar.Pos
	}{
		{"at a token", "rule a", 5, grammar.Pos{Line: 1, Col: 6}},
		{"space", "rule  a", 4, grammar.Pos{Line: 1, Col: 7}},
		{"newline", "type T\r\nrule", 6, grammar.Pos{Line: 2, Col: 1}},
		{"line comment", "rule // x\n  a", 5, grammar.Pos{Line: 2, Col: 3}},
		{"nested block comment", "rule /* a /* b */ c */ x", 4, grammar.Pos{Line: 1, Col: 24}},
		{"inside a block comment", "rule /* a\n b */ x", 9, grammar.Pos{Line: 2, Col: 7}},
		{"inside a nested block comment", "rule /* a /* b */\n c */ x", 16, grammar.Pos{Line: 2, Col: 7}},
		{"end of input", "rule \n", 4, grammar.Pos{Line: 2, Col: 1}},
		{"line comment marker in a block comment", "rule a b /* x\n // y */ c\n", 14, grammar.Pos{Line: 2, Col: 10}},
		{"block comment marker in a line comment", "rule a // /* x\n b\n", 7, grammar.Pos{Line: 2, Col: 2}},
		{"inside an unterminated block comment", "rule /* a\n // b", 12, grammar.Pos{Line: 1, Col: 6}},
		{"unterminated block comment", "rule /* a", 4, grammar.Pos{Line: 1, Col: 6}},
		{"inside a name", "rule abc", 6, grammar.Pos{Line: 1, Col: 6}},
		{"inside a punctuation mark", "rule a := b", 8, grammar.Pos{Line: 1, Col: 8}},
		{"inside a string", "rule a = \"xyz\"", 12, grammar.Pos{Line: 1, Col: 13}},
	} {
		got := SourceErrors(c.src, []SourceError{{Offset: c.off, Msg: "m"}})
		if len(got) == 0 || got[len(got)-1].Pos != c.want {
			t.Errorf("%s: %q at %d: got %v, want %v", c.name, c.src, c.off, got, c.want)
		}
	}
}

func TestSourceErrorsStartWithTheLexicalErrors(t *testing.T) {
	src := "rule a = x\nrule b = \"y"
	got := SourceErrors(src, []SourceError{{Offset: 9, Msg: "syntax error"}})
	if len(got) != 2 || got[0].Pos != (grammar.Pos{Line: 2, Col: 10}) || got[1].Pos != (grammar.Pos{Line: 1, Col: 10}) {
		t.Errorf("got %v", got)
	}
}

func TestSourceErrorsAreCappedAsTheParserCapsThem(t *testing.T) {
	// More invalid characters than the parser reports: the grammar's failure comes
	// after them, so the list ends with "too many errors" and holds no more.
	src := strings.Repeat("\\ ", maxErrors+5) + "rule"
	got := SourceErrors(src, []SourceError{{Offset: 0, Msg: "syntax error"}})
	if len(got) != maxErrors+1 || got[maxErrors].Msg != "too many errors" {
		t.Fatalf("got %d errors, last %v", len(got), got[len(got)-1])
	}
	_, want := Parse(src)
	if el, ok := want.(ErrorList); !ok || len(el) != len(got) {
		t.Errorf("the parser reports %v", want)
	}
}

func TestWithoutInvalidCharacters(t *testing.T) {
	for _, c := range []struct {
		src, want string
		changed   bool
	}{
		{"rule a = b", "rule a = b", false},
		{"type T =\\{}", "type T = {}", true},
		{"type T = '{ ", "type T =  { ", true},
		{"rule a = \\\\ b\nrule c\\\\d", "rule a =    b\nrule c  d", true},
		{"rule a = \"\\\" // \\", "rule a = \"\\\" // \\", false},
	} {
		got, changed := WithoutInvalidCharacters(c.src)
		if got != c.want || changed != c.changed {
			t.Errorf("%q: got %q, %v; want %q, %v", c.src, got, changed, c.want, c.changed)
		}
		if len([]rune(got)) != len([]rune(c.src)) {
			t.Errorf("%q: the offsets moved", c.src)
		}
	}
}

func TestSourceErrorsAreInTheOrderOfTheSource(t *testing.T) {
	src := "rule a = x\nrule b = y\nrule c = z\n"
	got := SourceErrors(src, []SourceError{{Offset: 22, Msg: "c"}, {Offset: 0, Msg: "a"}, {Offset: 11, Msg: "b"}, {Offset: 11, Msg: "b2"}})
	var msgs []string
	for _, e := range got {
		msgs = append(msgs, e.Msg)
	}
	if strings.Join(msgs, " ") != "a b c" {
		t.Errorf("got %v", got)
	}
}

func TestSourceErrorsKeepTheFirstOfTheCap(t *testing.T) {
	// The grammar lists the failure that ends the parse first: the cap must keep the earliest errors.
	src := strings.Repeat("rule a = x\n", maxErrors+20)
	var errs []SourceError
	for i := maxErrors + 19; i >= 0; i-- {
		errs = append(errs, SourceError{Offset: i * 11, Msg: "e"})
	}
	got := SourceErrors(src, errs)
	if len(got) != maxErrors+1 || got[maxErrors].Msg != "too many errors" {
		t.Fatalf("got %d errors", len(got))
	}
	for i, e := range got[:maxErrors] {
		if e.Pos.Line != i+1 {
			t.Fatalf("error %d is at line %d, want %d", i, e.Pos.Line, i+1)
		}
	}
}

// A source with lexical errors only, and no failure of the grammar, reports exactly the lexical errors.
func TestSourceErrorsWithoutGrammarFailureAreTheLexicalErrors(t *testing.T) {
	src := "rule a = \"x\" \x01\n"
	lexical := lexSource(src).errs
	if len(lexical) == 0 {
		t.Fatalf("lexSource(%q) reports no error", src)
	}
	got := SourceErrors(src, nil)
	if !reflect.DeepEqual([]*Error(got), lexical) {
		t.Errorf("SourceErrors(%q, nil) = %v, want the lexical errors %v", src, got, lexical)
	}
}
