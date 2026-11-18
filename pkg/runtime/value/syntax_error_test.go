package value

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSyntaxErrorMessage(t *testing.T) {
	e := &SyntaxError{Line: 2, Col: 5}
	if got := e.Error(); got != "2:5: syntax error" {
		t.Errorf("no expected entries: %q", got)
	}
	e.Expected = []string{"end of input", `"a"`, "[0-9]"}
	if got := e.Message(); got != `syntax error: expected end of input, "a", [0-9]` {
		t.Errorf("expected entries: %q", got)
	}
	var many []string
	for i := range expectedInMessage + 3 {
		many = append(many, fmt.Sprintf(`"%d"`, i))
	}
	e.Expected = many
	if got := e.Message(); !strings.HasSuffix(got, `"9", ... and 3 more`) || strings.Contains(got, `"10"`) {
		t.Errorf("cut list: %q", got)
	}
	if shown, omitted := e.ExpectedInMessage(); len(shown) != expectedInMessage || omitted != 3 || shown[9] != `"9"` {
		t.Errorf("ExpectedInMessage = %q, %d", shown, omitted)
	}
	e.Messages = []string{"bad", "worse"}
	if got := e.Message(); got != "bad; worse" {
		t.Errorf("messages: %q", got)
	}
}

func TestSyntaxErrorLists(t *testing.T) {
	a := &SyntaxError{Line: 1, Col: 1}
	b := &SyntaxError{Line: 3, Col: 2, Messages: []string{"m"}}
	l := SyntaxErrors{a, b}
	if got := l.Error(); got != "1:1: syntax error\n3:2: m" {
		t.Errorf("SyntaxErrors.Error() = %q", got)
	}
	tr := &TruncatedSyntaxErrors{Errors: l, Total: 5, Dropped: 3}
	if got := tr.Error(); got != "recovered syntax errors truncated (total 5, dropped 3):\n"+l.Error() {
		t.Errorf("TruncatedSyntaxErrors.Error() = %q", got)
	}
	var got SyntaxErrors
	if !errors.As(fmt.Errorf("w: %w", tr), &got) || len(got) != 2 {
		t.Errorf("errors.As(SyntaxErrors) = %v", got)
	}
}
