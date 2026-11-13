package apitest_test

import (
	"context"
	"errors"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/parse"
)

func TestDocumentSnapshotPublicContract(t *testing.T) {
	p, err := compileTestSource(`rule main = "a"+ $$`, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range []parse.PositionUnit{parse.CodePoints, parse.Bytes} {
		d, err := p.NewDocument("aa", zp.WithUnit(unit))
		if err != nil {
			t.Fatal(err)
		}
		s, err := d.Snapshot()
		if err != nil || s == nil || !s.Root().Valid() || s.Text() != "aa" || s.Unit() != unit || s.Version() != d.Version() {
			t.Fatal("snapshot", err)
		}
		legacy := s.Materialize()
		if legacy == nil {
			t.Fatal("materialize")
		}
		value.SetNodeStart(legacy, 999)
		if s.Root().Start() == 999 {
			t.Fatal("snapshot aliases materialized result")
		}
		if err := d.Edit(0, 1, "b"); err != nil {
			t.Fatal(err)
		}
		failed, err := d.Snapshot()
		var syntax *parse.SyntaxError
		if failed == nil || !errors.As(err, &syntax) || failed.Text() != "ba" || failed.Version() == s.Version() || failed.Root().Valid() || s.Text() != "aa" {
			t.Fatal("failed version", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		interrupted, err := d.Snapshot(zp.WithContext(ctx))
		if interrupted != nil || err == nil {
			t.Fatal("cancelled snapshot", err)
		}
		_, parseErr := d.Parse(zp.WithContext(ctx))
		if parseErr == nil || parseErr.Error() != err.Error() {
			t.Fatalf("snapshot and Parse disagree on budget-control availability: %v / %v", err, parseErr)
		}
	}
}

// TestDocumentSnapshotFieldHandles checks that node values read from snapshot fields are
// zp.SnapshotNode handles, through Field and Fields alike, and that scalars stay scalars.
func TestDocumentSnapshotFieldHandles(t *testing.T) {
	p, err := compileTestSource(`type Pair = { K node[string]; N int }
rule main = k:"a" => Pair{K: $k, N: 1}`, "main")
	if err != nil {
		t.Fatal(err)
	}
	d, err := p.NewDocument("a")
	if err != nil {
		t.Fatal(err)
	}
	s, err := d.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	root := s.Root()
	k, ok := root.Field("K")
	if h, isNode := k.(zp.SnapshotNode); !ok || !isNode || h.Text() != "a" {
		t.Fatalf("Field(K) = %#v, %v", k, ok)
	}
	if n, ok := root.Field("N"); !ok || n != 1 {
		t.Fatalf("Field(N) = %#v, %v", n, ok)
	}
	fields := root.Fields()
	if len(fields) != 2 {
		t.Fatalf("Fields() = %#v", fields)
	}
	for _, f := range fields {
		v, _ := root.Field(f.Name)
		if f.Value != v {
			t.Errorf("Fields() %s = %#v, Field %#v", f.Name, f.Value, v)
		}
	}
}
