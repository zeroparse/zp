package flow

import (
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
)

// TestDialectVerifies checks that the declarations of the Flow dialect verify at the Flow stage, that
// every operation is of the dialect, and that each has a contract.
func TestDialectVerifies(t *testing.T) {
	m := NewModule("flow")
	if err := code.Verify(m); err != nil {
		t.Fatal(err)
	}
	for _, o := range m.Ops {
		if o.Dialect != Dialect || o.Contract == "" || o.Kind != code.OpModeled {
			t.Errorf("%s: dialect %q, kind %s, contract %q", o.Name, o.Dialect, o.Kind, o.Contract)
		}
	}
}
