package runtime

import (
	"errors"
	"testing"

	"zeroparse.dev/zp/runtime/parse"
)

func TestDiagnosticRenameAccountsForLargeExpectationSets(t *testing.T) {
	ids := make([]ExpID, 512)
	for i := range ids {
		ids[i] = ExpID(i + 1)
	}
	p := &Parser{Control: newBudgetControl(nil, parse.Limits{Compute: 64})}
	var got error
	func() {
		defer func() {
			if v := recover(); v != nil {
				f, ok := v.(fatal)
				if !ok {
					panic(v)
				}
				got = f.err
			}
		}()
		RenamedExpected(p, ids, 999)
	}()
	if !errors.Is(got, parse.ErrLimit) {
		t.Fatalf("rename compute limit = %v, want ErrLimit", got)
	}
	if p.Control.budgetUsed() != 64 {
		t.Fatalf("rename steps = %d, want 64", p.Control.budgetUsed())
	}
}
