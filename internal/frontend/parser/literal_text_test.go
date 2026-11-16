package parser

import (
	"strconv"
	"testing"
	"unicode/utf8"

	"zeroparse.dev/zp/grammar"
)

// TestLiteralTextRoundTrip checks that the text of a literal as an expected item, written into a
// grammar, is the same literal: the text is copied back unchanged. The Go quoting of a control
// character is not (negative control).
func TestLiteralTextRoundTrip(t *testing.T) {
	t.Parallel()
	values := []string{"", "a", `"`, `\`, "\"\\", "\x00", "\a\b\f\v\n\r\t", "\x1f\x7f", "\u0080 ​ ",
		"é", "😀", "�", "\U0001f4a9", "\U000e0001", "Ab", "select", "a b", "\U0010ffff", "﷐"}
	for _, v := range values {
		for _, fold := range []bool{false, true} {
			text := grammar.LiteralText(v, fold)
			g, err := Parse("rule main = " + text + "\n")
			if err != nil {
				t.Errorf("%q: %s: %v", v, text, err)
				continue
			}
			var lit *grammar.Literal
			for _, d := range g.Rules() {
				lit, _ = d.Expr.(*grammar.Literal)
			}
			if lit == nil || lit.Value != v || lit.IgnoreCase != fold {
				t.Errorf("%q: %s parses back as %+v", v, text, lit)
			}
		}
	}
	for _, v := range []string{"\x00", "\U000e0001"} {
		if utf8.ValidString(v) {
			if _, err := Parse("rule main = " + strconv.Quote(v) + "\n"); err == nil {
				t.Errorf("negative control: the Go quoting %s is accepted as a grammar literal", strconv.Quote(v))
			}
		}
	}
}
