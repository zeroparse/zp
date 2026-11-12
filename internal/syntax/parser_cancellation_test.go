package syntax

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParsePartialContextCancelsWithinLongStatement(t *testing.T) {
	for _, tc := range []struct {
		name, source string
	}{
		{"ASCII", "rule main = " + strings.Repeat(`"x" `, 2048)},
		{"Unicode", "rule main = " + strings.Repeat(`"😀" `, 2048)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Count the polls of a whole parse first: the check before parsing, the polls of
			// the parser and the check after it. The source is one statement, so every poll of
			// the parser is within it; it must poll at least twice.
			baseline := &cancelOnCheck{cancelAt: int(^uint(0) >> 1)}
			if _, _, err := ParsePartialContext(baseline, tc.source); err != nil {
				t.Fatal(err)
			}
			if baseline.checks < 4 {
				t.Fatalf("the parse polled %d times, want two polls within the statement", baseline.checks)
			}
			// Cancel at the first poll of the parser. The parse must stop there, before the
			// polls that follow it within the statement: after the poll that sees the
			// cancellation, only the check of the caller runs.
			ctx := &cancelOnCheck{cancelAt: 2}
			g, diagnostics, err := ParsePartialContext(ctx, tc.source)
			if g != nil || diagnostics != nil || !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled parse = %v, %v, %v after %d/%d polls", g, diagnostics, err, ctx.checks, baseline.checks)
			}
			if ctx.checks != ctx.cancelAt+1 {
				t.Fatalf("the cancelled parse polled %d times, want %d: it went on after the cancellation", ctx.checks, ctx.cancelAt+1)
			}
			wantG, wantDiagnostics := ParsePartial(tc.source)
			gotG, gotDiagnostics, err := ParsePartialContext(context.Background(), tc.source)
			if err != nil || !reflect.DeepEqual(gotG, wantG) || !reflect.DeepEqual(gotDiagnostics, wantDiagnostics) {
				t.Fatalf("parse parity: err=%v, grammar equal=%v, diagnostics equal=%v", err, reflect.DeepEqual(gotG, wantG), reflect.DeepEqual(gotDiagnostics, wantDiagnostics))
			}
		})
	}
}

func TestParsePartialContextChecksAfterShortParse(t *testing.T) {
	const source = `rule main = "x"`
	baseline := &cancelOnCheck{cancelAt: int(^uint(0) >> 1)}
	if _, _, err := ParsePartialContext(baseline, source); err != nil {
		t.Fatal(err)
	}
	// Cancellation that arrives at the last check, after the parse completed, must discard the
	// result.
	ctx := &cancelOnCheck{cancelAt: baseline.checks}
	g, diagnostics, err := ParsePartialContext(ctx, source)
	if g != nil || diagnostics != nil || !errors.Is(err, context.Canceled) || ctx.checks != ctx.cancelAt {
		t.Fatalf("short cancelled parse = %v, %v, %v after %d/%d polls", g, diagnostics, err, ctx.checks, ctx.cancelAt)
	}
}

func TestParsePartialContextLongSameLineAttributesParity(t *testing.T) {
	source := `rule main = ("x" ` + strings.Repeat(`@a `, 1024) + "\n@a)"
	wantG, wantDiagnostics := ParsePartial(source)
	gotG, gotDiagnostics, err := ParsePartialContext(context.Background(), source)
	if err != nil || !reflect.DeepEqual(gotG, wantG) || !reflect.DeepEqual(gotDiagnostics, wantDiagnostics) {
		t.Fatalf("attribute break parity: err=%v, grammar equal=%v, diagnostics equal=%v", err, reflect.DeepEqual(gotG, wantG), reflect.DeepEqual(gotDiagnostics, wantDiagnostics))
	}
}
