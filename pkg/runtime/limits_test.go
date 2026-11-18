package runtime

import (
	"errors"
	"fmt"
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
