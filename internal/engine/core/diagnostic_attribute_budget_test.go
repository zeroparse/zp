//go:build !zp_budget_controls_reference

package core

import (
	"context"
	"errors"
	"testing"

	"zeroparse.dev/zp/runtime/parse"
)

func TestDiagnosticRenameAccountsForLargeExpectationSets(t *testing.T) {
	ids := make([]ExpID, 512)
	for i := range ids {
		ids[i] = ExpID(i + 1)
	}
	p := &Parser{Input: Input{Control: NewBudgetControl(nil, parse.Limits{Compute: 64})}}
	if err := catchFatal(t, func() { RenamedExpected(p, ids, 999) }); !errors.Is(err, parse.ErrLimit) {
		t.Fatalf("rename compute limit = %v, want ErrLimit", err)
	}
	if got := p.Control.Used; got != 64 {
		t.Fatalf("rename steps = %d, want 64", got)
	}

	c := newPollingContext(1)
	p.Control = NewBudgetControl(c, parse.Limits{})
	if err := catchFatal(t, func() { RenamedExpected(p, ids, 999) }); !errors.Is(err, context.Canceled) {
		t.Fatalf("rename cancellation = %v, want context.Canceled", err)
	}
}
