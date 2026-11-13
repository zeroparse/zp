//go:build zp_budget_controls_reference

package engine

import (
	"context"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/gentest"
)

func TestBudgetStreamReferenceAdmission(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, gentest.Records)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, o := range []ParseOptions{{Context: ctx}, {Limits: core.Limits{Compute: 1}}, {Limits: core.Limits{Memory: 1}}} {
		r := &streamCallbackReader{text: "#records\na=1\n"}
		callbacks := 0
		r.onRead = func() { callbacks++ }
		o.Trace = func(TraceEvent) { callbacks++ }
		err := prog.ParseStreamWith("main", r, func(*core.Node) error { callbacks++; return nil }, o)
		if err == nil || !strings.Contains(err.Error(), "reference build") || callbacks != 0 || r.offset != 0 || r.closed {
			t.Fatalf("error=%v callbacks=%d offset=%d", err, callbacks, r.offset)
		}
	}
	for _, o := range []ParseOptions{{}, {Context: context.Background()}} {
		if err := prog.ParseStreamWith("main", strings.NewReader("#records\na=1\n"), nil, o); err != nil {
			t.Fatal(err)
		}
	}
}

// The reference build has no controls, so it refuses a finite Memory, and accepts Unlimited.
func TestMemoryLimitReferenceAdmission(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, gentest.Records)
	const text = "#records\na=1\n"
	o := ParseOptions{Limits: core.Limits{Memory: 1 << 30}}
	if n, err := prog.ParseWith("main", text, o); n != nil || err == nil || !strings.Contains(err.Error(), "reference build") {
		t.Fatalf("batch: node=%v error=%v", n, err)
	}
	if d, err := prog.NewDocumentWith("main", text, o); d != nil || err == nil || !strings.Contains(err.Error(), "reference build") {
		t.Fatalf("document: error=%v", err)
	}
	if _, err := prog.ParseWith("main", text, ParseOptions{Limits: core.Limits{Memory: core.Unlimited}}); err != nil {
		t.Fatal(err)
	}
}
