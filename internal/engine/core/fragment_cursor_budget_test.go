//go:build !zp_budget_controls_reference

package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/runtime/parse"
)

func TestFragmentCursorFallbackInterruption(t *testing.T) {
	source := strings.Repeat("😀\xff", 1000) + "tail"
	in := NewTextInput(source, parse.CodePoints, false)
	// Endpoint conversion itself must cooperate even after matching succeeded.
	in.Control = NewBudgetControl(nil, parse.Limits{Compute: 8})
	if err := catchFatal(t, func() { in.PrefixByteOffset(source, 2000) }); !errors.Is(err, parse.ErrLimit) {
		t.Fatalf("fallback compute limit: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	in.Control = NewBudgetControl(ctx, parse.Limits{})
	cancel()
	if err := catchFatal(t, func() { in.PrefixByteOffset(source, 2000) }); !errors.Is(err, context.Canceled) {
		t.Fatalf("fallback cancellation: %v", err)
	}
	if in.Src != source || in.Offs != nil {
		t.Fatal("interruption changed source or retained an index")
	}
	in.Control = nil
	if got := in.PrefixByteOffset(source, 2000); got != 5000 {
		t.Fatalf("retry endpoint=%d, want 5000", got)
	}
}
