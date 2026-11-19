package vm

import (
	"testing"

	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/bytecode"
)

// TestFlowFixedExpectations checks that the expectation IDs of Flow bodies count the fixed
// expectations of the parser's table as the module format says: a module file stores the IDs, and
// the VM passes them to the parser as they are.
func TestFlowFixedExpectations(t *testing.T) {
	if bytecode.FixedExps != int(program.NumFixedDescs) {
		t.Fatalf("bytecode.FixedExps = %d; the parser has %d fixed expectations", bytecode.FixedExps, program.NumFixedDescs)
	}
}
