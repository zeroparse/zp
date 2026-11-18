//go:build !zp_budget_controls_reference

package state

import (
	"context"
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/pkg/runtime"
	"zeroparse.dev/zp/pkg/runtime/input"
)

func TestFragmentCursorFallbackInterruption(t *testing.T) {
	source := strings.Repeat("😀\xff", 1000) + "tail"
	in := NewTextInput(source, input.CodePoints, false)
	// Endpoint conversion itself must cooperate even after matching succeeded.
	in.Control = NewBudgetControl(nil, runtime.Limits{Compute: 8})
	if err := catchFatal(t, func() { in.PrefixByteOffset(source, 2000) }); !errors.Is(err, runtime.ErrLimit) {
		t.Fatalf("fallback compute limit: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	in.Control = NewBudgetControl(ctx, runtime.Limits{})
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
