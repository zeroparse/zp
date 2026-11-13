package core

import (
	"context"
	"errors"
	"testing"

	"zeroparse.dev/zp/core"
)

// reserve runs c.Charge(n) and returns the error of the interruption it raises, or nil.
func reserve(c *BudgetControl, n uint64) (err error) {
	defer func() {
		if x := recover(); x != nil {
			f, ok := x.(Fatal)
			if !ok {
				panic(x)
			}
			err = f.Err
		}
	}()
	c.Charge(n)
	return nil
}

// TestReservationRules pins the rules "Reservations are atomic" and "Observing the context" of
// "Reservations and Interruptions" in spec/runtime/parsing.md on the reference reservation: a
// reservation that interrupts leaves the counters as they were; a cancelled context is observed only
// at a reservation that reaches the polling interval or would exceed the limit; and there its error
// wins over the exhausted limit.
func TestReservationRules(t *testing.T) {
	if BudgetCheckInterval != 256 {
		t.Fatalf("polling interval %d, the specification states 256", BudgetCheckInterval)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := NewBudgetControl(ctx, core.Limits{Compute: 1000})
	if err := reserve(c, 200); err != nil || c.Used != 200 || c.SinceCheck != 200 {
		t.Fatalf("reservation below the interval: %v, used %d, since %d", err, c.Used, c.SinceCheck)
	}
	cancel()
	// Neither polls nor exceeds: the cancellation is not observed yet.
	if err := reserve(c, 55); err != nil || c.Used != 255 {
		t.Fatalf("reservation before the poll: %v, used %d", err, c.Used)
	}
	// Reaches the interval: the context is observed, and the counters stay.
	if err := reserve(c, 1); !errors.Is(err, context.Canceled) || c.Used != 255 || c.SinceCheck != 255 {
		t.Fatalf("polling reservation: %v, used %d, since %d", err, c.Used, c.SinceCheck)
	}
	// Exceeds as well as polls: the context error wins over the limit.
	if err := reserve(c, 5000); !errors.Is(err, context.Canceled) || c.Used != 255 {
		t.Fatalf("exceeding reservation with a cancelled context: %v, used %d", err, c.Used)
	}
	// Without a context, exceeding interrupts with a limit error and leaves the counters.
	d := NewBudgetControl(nil, core.Limits{Compute: 10})
	if err := reserve(d, 4); err != nil {
		t.Fatal(err)
	}
	var le *core.LimitError
	if err := reserve(d, 7); !errors.As(err, &le) || le.Used != 4 || le.Requested != 7 || d.Used != 4 || d.SinceCheck != 4 {
		t.Fatalf("exceeding reservation: %v, used %d, since %d", err, d.Used, d.SinceCheck)
	}
	// An observation that finds no error starts the interval again.
	e := NewBudgetControl(context.Background(), core.Limits{Compute: 1 << 20})
	if err := reserve(e, 300); err != nil || e.Used != 300 || e.SinceCheck != 0 {
		t.Fatalf("polling reservation without an error: %v, used %d, since %d", err, e.Used, e.SinceCheck)
	}
}
