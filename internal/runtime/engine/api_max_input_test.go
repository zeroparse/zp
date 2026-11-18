//go:build !zp_budget_controls_reference

package engine_test

import (
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/runtime/engine"
	"zeroparse.dev/zp/internal/testing/enginetest"
	"zeroparse.dev/zp/pkg/runtime"
	"zeroparse.dev/zp/pkg/runtime/input"
)

func wantInputLimit(t *testing.T, what string, err error, limit, length uint64) {
	t.Helper()
	var le *runtime.LimitError
	if !errors.As(err, &le) || !errors.Is(err, runtime.ErrLimit) {
		t.Fatalf("%s: error %v, want a LimitError", what, err)
	}
	if le.Kind != runtime.LimitInput || le.Limit != limit || le.Requested != length {
		t.Errorf("%s: %+v, want kind LimitInput, limit %d, length %d", what, *le, limit, length)
	}
}

// Limits.Input bounds the input of every public entry point in the position unit: zero and
// Unlimited set no bound, and an input over it fails with LimitInput and its length.
func TestMaxInputPublic(t *testing.T) {
	p, err := enginetest.CompileSource(`@start
rule main = [a-zé]* $$`, "main")
	if err != nil {
		t.Fatal(err)
	}
	stream, err := enginetest.CompileSource(`@start
rule main = line* #stream $$
rule line = [a-z]+ "\n"`, "main")
	if err != nil {
		t.Fatal(err)
	}
	text := "ééé" // 3 code points, 6 bytes
	for _, c := range []struct {
		unit input.PositionUnit
		n    uint64
	}{{input.CodePoints, 3}, {input.Bytes, 6}} {
		with := func(max uint64) []engine.ParseOption {
			return []engine.ParseOption{engine.WithUnit(c.unit), engine.WithLimits(runtime.Limits{Input: max})}
		}
		for _, max := range []uint64{0, runtime.Unlimited, c.n} {
			if _, err := p.Parse(text, with(max)...); err != nil {
				t.Errorf("unit %d, MaxInput %d: %v", c.unit, max, err)
			}
		}
		_, err := p.Parse(text, with(c.n-1)...)
		wantInputLimit(t, "Parse", err, c.n-1, c.n)
		_, err = p.NewDocument(text, with(c.n-1)...)
		wantInputLimit(t, "NewDocument", err, c.n-1, c.n)
		d, err := p.NewDocument("é", with(c.n)...)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Edit(0, 0, "éé"); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Parse(); err != nil {
			t.Fatalf("Document at the limit: %v", err)
		}
		if err := d.Edit(0, 0, "a"); err != nil {
			t.Fatal(err)
		}
		_, err = d.Parse()
		wantInputLimit(t, "Document.Parse after an edit", err, c.n, c.n+1)

		lines := "ab\ncd\n"
		sn := uint64(len(lines))
		if err := stream.ParseStream(strings.NewReader(lines), nil, with(sn)...); err != nil {
			t.Errorf("stream at the limit: %v", err)
		}
		err = stream.ParseStream(strings.NewReader(lines), nil, with(sn-1)...)
		wantInputLimit(t, "ParseStream", err, sn-1, sn)
		it, err := stream.Iterate(strings.NewReader(lines), with(sn-1)...)
		if err != nil {
			t.Fatal(err)
		}
		for {
			_, ok, err := it.Next()
			if err != nil {
				wantInputLimit(t, "Iterate", err, sn-1, sn)
				break
			}
			if !ok {
				t.Fatal("the iterator ended without the limit error")
			}
		}
	}
}
