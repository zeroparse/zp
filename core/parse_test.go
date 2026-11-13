package core

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestLimitError(t *testing.T) {
	for _, tc := range []struct {
		err  *LimitError
		want string
	}{
		{&LimitError{Kind: LimitCompute, Limit: 10, Used: 8, Requested: 3}, "parse compute limit exceeded: limit 10, used 8, requested 3"},
		{&LimitError{Kind: LimitInput, Limit: 4, Requested: 9}, "parse input limit exceeded: limit 4, length 9"},
		{&LimitError{Kind: LimitDepth, Limit: 7, Used: 7, Requested: 1}, "nesting too deep: more than 7 nested recursive rule calls"},
		{&LimitError{Kind: LimitMemory, Limit: 100, Used: 120}, "parse memory limit exceeded: limit 100, used 120"},
		{nil, "parse limit exceeded"},
	} {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
	}
	wrapped := fmt.Errorf("parse: %w", &LimitError{Kind: LimitDepth, Limit: 1})
	if !errors.Is(wrapped, ErrLimit) {
		t.Error("errors.Is(LimitError, ErrLimit) = false")
	}
	if errors.Is(errors.New("other"), ErrLimit) {
		t.Error("errors.Is(other, ErrLimit) = true")
	}
	var nilErr *LimitError
	if nilErr.Is(ErrLimit) {
		t.Error("nil LimitError matches ErrLimit")
	}
}

func TestPositionUnitString(t *testing.T) {
	if CodePoints.String() != "codepoints" || Bytes.String() != "bytes" {
		t.Errorf("String() = %q, %q", CodePoints, Bytes)
	}
	if CodePoints != 0 {
		t.Error("CodePoints is not the zero value")
	}
}

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

func TestSnapshotCaptureError(t *testing.T) {
	inner := errors.New("inner")
	e := &SnapshotCaptureError{Path: "root.x", ParseError: inner}
	if !errors.Is(e, inner) {
		t.Error("Unwrap does not return the parse error")
	}
	if got := e.Error(); got != "cannot capture snapshot field root.x with type <nil>" {
		t.Errorf("Error() = %q", got)
	}
}
