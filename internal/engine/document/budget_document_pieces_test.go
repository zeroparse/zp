//go:build !zp_budget_controls_reference

package document_test

import (
	"errors"
	"strings"
	"testing"

	. "zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/runtime/parse"
)

func TestBudgetDocumentPieceLineTableRetry(t *testing.T) {
	prog := mustCompile(t, "var line int\nrule main = {line == 2000} .* $$")
	budgetDocumentVariants(t, func(t *testing.T, o ParseOptions) {
		d, err := prog.NewDocumentWith("main", strings.Repeat("x\n", 4096), o)
		if err != nil {
			t.Fatal(err)
		}
		o.Limits.Compute = 100
		if _, err = d.ParseWith(o); !errors.Is(err, parse.ErrLimit) {
			t.Fatalf("line table interruption: %v", err)
		}
		if d.Input().Lines != nil {
			t.Fatal("interrupted scan retained a partial line table")
		}
		budgetDocumentAssertRetry(t, d)
	})
}

func TestBudgetDocumentPieceLiteralRetry(t *testing.T) {
	text := strings.Repeat("a", 8192)
	prog := mustCompile(t, "rule main = "+`"`+text+`" $$`)
	budgetDocumentVariants(t, func(t *testing.T, o ParseOptions) {
		d, err := prog.NewDocumentWith("main", text, o)
		if err != nil {
			t.Fatal(err)
		}
		o.Limits.Compute = 100
		if _, err = d.ParseWith(o); !errors.Is(err, parse.ErrLimit) {
			t.Fatalf("piece literal bypassed the limit: %v", err)
		}
		budgetDocumentAssertRetry(t, d)
	})
}
