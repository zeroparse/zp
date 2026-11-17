//go:build zp_long

package engine_test

import "testing"

// TestMemoStorePolicyMutantsFail is the negative control of TestMemoStorePolicy: each mutant makes
// the engine defer the result of a later evaluation as well as of the first, in a copy of the
// source of its memo, and the test must fail when the engine builds with the copy.
func TestMemoStorePolicyMutantsFail(t *testing.T) {
	const test = "^TestMemoStorePolicy$"
	here := []string{"."}
	const firstCall = "\treturn p.memoFirstCall(p.Pos, r.Seen)\n"
	const dense = "\t\tfirst = t.MarkSeen(pos, r)\n"
	const sparse = "\t\tfirst = markSeenPage(page, pos)\n"
	runPrintedMutants(t, "state/runtime.go", []printedMutant{
		{"a call inside a lookahead defers its own first call",
			[]printedEdit{{firstCall, "\tpos := p.Pos\n\tif p.Silent > 0 {\n\t\tpos += 1 << 12\n\t}\n\treturn p.memoFirstCall(pos, r.Seen)\n"}},
			test, here},
	})
	runPrintedMutants(t, "state/memo.go", []printedMutant{
		{"a second call at a position defers as well (dense table)",
			[]printedEdit{{dense, "\t\tfirst = t.MarkSeen(pos, r) || t.MarkSeen(pos+1<<12, r)\n"}},
			test, here},
		{"a second call at a position defers as well (sparse table)",
			[]printedEdit{{sparse, "\t\tfirst = markSeenPage(page, pos) || t.MarkSparseSeen(pos+1<<12, r)\n"}},
			test, here},
	})
	// A Document parse stores every call: a Document that defers a first call evaluates more.
	runPrintedMutants(t, "document/document.go", []printedMutant{
		{"a Document defers the first call of a rule at a position",
			[]printedEdit{{"Memo: d.memo, MemoAll: true,", "Memo: d.memo, DeferMemo: true, MemoAll: true,"}},
			"^TestMemoStorePolicyDocument$", here},
	})
}
