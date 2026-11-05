//go:build zp_long

package engine_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCallProtocolMutantsFail is the negative control of the tests of the engine's call protocol,
// which is printed from Exec IR (core/call_exec.go): each mutant breaks one part of the protocol
// in a copy of the printed file, and the tests named for it must fail when they build with the
// copy in place of the file.
func TestCallProtocolMutantsFail(t *testing.T) {
	const reuse = "if found && (!c.ctx || p.ctxAllows(&c, e)) && (!e.silent || p.Silent > 0) && "
	const examine = "entry.Examined, entry.From, entry.Positional = p.Hw, p.lw, false"
	const route = "\tif store == storeMemo {\n"
	const copyErrs = "Errs: p.copyErrors(p.Recovered[rec:])}"
	runPrintedMutants(t, "core/call_exec.go", []printedMutant{
		{"a failed call keeps its recovered errors",
			[]printedEdit{{"if !r.Leader && !ok {\n\t\tp.Recovered = p.Recovered[:rec]\n", "if !r.Leader && !ok {\n"}},
			"^TestDocumentFailedEntriesHoldNoErrors$", []string{"./document"}},
		{"an entry examines no input beyond its position",
			[]printedEdit{{examine, "entry.Examined, entry.From, entry.Positional = start, p.lw, false"}},
			"^TestDocumentMatchesFreshParse$", []string{"."}},
		{"an entry examines nothing before its position",
			[]printedEdit{{examine, "entry.Examined, entry.From, entry.Positional = p.Hw, start, false"}},
			"^TestDocumentEntriesExamineFromTheirPosition$", []string{"./document"}},
		{"a call reuses a result evaluated in a lookahead",
			[]printedEdit{{reuse, "if found && (!c.ctx || p.ctxAllows(&c, e)) && true && "}},
			"^TestMemoLookaheadStateEvaluations$", []string{"."}},
		{"a call in a context ignores the growths present",
			[]printedEdit{{reuse, "if found && true && (!e.silent || p.Silent > 0) && "}},
			"^TestLeftRecursionMemberMemoStale$", []string{"."}},
		{"a call leaves the examined end of its caller",
			[]printedEdit{{"p.Hw = max(oldHigh, p.Hw)", "p.Hw = oldHigh"}},
			"^TestDocumentMatchesFreshParse$", []string{"."}},
		{"a result that goes nowhere is stored in the memo",
			[]printedEdit{{route, "\tif store != storeCtx {\n"}},
			"^TestBudgetUnrecordedContextCalls$", []string{"."}},
		{"a result that read a growing seed is stored in the memo",
			[]printedEdit{{route, "\tif store != storeNowhere {\n"}},
			"^TestLeftRecursionMemberMemoStale$", []string{"."}},
		{"a call copies its recovered errors before it keeps its expectations",
			[]printedEdit{
				{"\tfar, inner := p.Unisolate(m)\n\tkept := p.Keep(inner)\n\tstore := storeMemo\n",
					"\terrs := p.copyErrors(p.Recovered[rec:])\n\tfar, inner := p.Unisolate(m)\n\tkept := p.Keep(inner)\n\tstore := storeMemo\n"},
				{"Gen: p.Gen, Vgen: p.Gen, " + copyErrs, "Gen: p.Gen, Vgen: p.Gen, Errs: errs}"},
				{"Positional: r.Positional, " + copyErrs, "Positional: r.Positional, Errs: errs}"},
			},
			"^TestBudgetCallEndChargesErrorsAfterExpectations$", []string{"."}},
	})
}

// TestExpectationRecordMutantsFail is the negative control of the tests of the engine's expectation
// record, which is printed from Exec IR into the same file as the call protocol. Its operations are
// transitional and have no conformance cases, so these mutants are what binds the printed code. Two
// edits are left out because they change nothing: without expectations a record holds at most the
// placeholder and its filter stays empty, so a merge past the farthest position that keeps the
// record or the filter leaves the state that resetting them gives.
func TestExpectationRecordMutantsFail(t *testing.T) {
	const (
		dropped   = "if p.Silent > 0 || pos < p.Farthest {\n\t\treturn"
		reset     = "\n\t\tp.Exp = p.Exp[:p.ExpBase]\n\t\tp.ExpBits = 0\n"
		off       = "\tif !p.Expectations {\n\t\tif len(p.Exp) == p.ExpBase {\n\t\t\tp.Exp = append(p.Exp, ExpFar)\n\t\t}\n\t\treturn\n\t}\n"
		mergeOff  = "if !(p.Silent > 0 || far < p.Farthest) {"
		mergeFill = "\t\t\t\tif len(p.Exp) == p.ExpBase {\n\t\t\t\t\tp.Exp = append(p.Exp, ExpFar)\n\t\t\t\t}\n"
		last      = "\tp.LastKept = p.Arena[off:len(p.Arena):len(p.Arena)]\n"

		scan   = "^TestBudgetExpectationCollisionScanBudgetsAndShortCircuits$"
		filter = "^TestBudgetExpectationFilterFollowsTheRecord$"
		merge  = "^TestBudgetMergeExpectedForwardAliasAndBudgets$"
		noExp  = "^TestBudgetMergeExpectedWithoutExpectations$"
		unrec  = "^TestBudgetUnrecordedContextCalls$"
		kept   = "^TestBudgetExpectationKeepSnapshotsAndAtomicCopy$"
		chunks = "^TestBudgetExpectationKeepFillsChunks$"
	)
	here := []string{"."}
	runPrintedMutants(t, "core/call_exec.go", []printedMutant{
		{"an expectation in a lookahead is recorded",
			[]printedEdit{{dropped, "if pos < p.Farthest {\n\t\treturn"}}, scan, here},
		{"an expectation before the farthest failure is recorded",
			[]printedEdit{{dropped, "if p.Silent > 0 {\n\t\treturn"}}, scan, here},
		{"an expectation past the farthest failure leaves the position",
			[]printedEdit{{"\t\tp.Farthest = pos\n\t\tp.Exp = p.Exp[:p.ExpBase]\n", "\t\tp.Exp = p.Exp[:p.ExpBase]\n"}}, merge, here},
		{"an expectation past the farthest failure keeps the entries",
			[]printedEdit{{reset, "\n\t\tp.ExpBits = 0\n"}}, merge, here},
		{"an expectation past the farthest failure keeps the filter",
			[]printedEdit{{reset, "\n\t\tp.Exp = p.Exp[:p.ExpBase]\n"}}, filter, here},
		{"a record without expectations has no placeholder",
			[]printedEdit{{off, "\tif !p.Expectations {\n\t\treturn\n\t}\n"}}, unrec, here},
		{"a record without expectations grows a placeholder for each",
			[]printedEdit{{"\tif !p.Expectations {\n\t\tif len(p.Exp) == p.ExpBase {\n", "\tif !p.Expectations {\n\t\tif true {\n"}}, unrec, here},
		{"a record without expectations records them",
			[]printedEdit{{off, ""}}, unrec, here},
		{"a repeated expectation joins the record",
			[]printedEdit{{"\t\t\tif p.Exp[p.ExpBase+i] == id {\n\t\t\t\treturn\n\t\t\t}\n", "\t\t\t_ = i\n"}}, scan, here},
		{"the scan reads the enclosing record",
			[]printedEdit{{"for i = 0; i < (len(p.Exp) - p.ExpBase); i++ {\n\t\t\tp.Spend()\n\t\t\tif p.Exp[p.ExpBase+i] == id {",
				"for i = 0; i < len(p.Exp); i++ {\n\t\t\tp.Spend()\n\t\t\tif p.Exp[i] == id {"}}, filter, here},
		{"the scan charges nothing",
			[]printedEdit{{"i++ {\n\t\t\tp.Spend()\n\t\t\tif p.Exp[p.ExpBase+i]", "i++ {\n\t\t\tif p.Exp[p.ExpBase+i]"}}, scan, here},
		{"every expectation scans the record",
			[]printedEdit{{"\tif p.ExpBits&expBit(id) != 0 {\n", "\tif true {\n"}}, merge, here},
		{"an added expectation stays out of the filter",
			[]printedEdit{{"\tp.ExpBits |= expBit(id)\n", ""}}, filter, here},
		{"a merge charges nothing for the expectations it records",
			[]printedEdit{{"\t\t\tp.Spend()\n\t\t\tp.Expect(far, s[i])\n", "\t\t\tp.Expect(far, s[i])\n"}}, merge, here},
		{"a merge without expectations charges nothing",
			[]printedEdit{{"\t\t\tp.Spend()\n\t\t\t" + mergeOff, "\t\t\t" + mergeOff}}, noExp, here},
		{"a merge without expectations ignores a lookahead",
			[]printedEdit{{mergeOff, "if !(far < p.Farthest) {"}}, noExp, here},
		{"a merge without expectations ignores the farthest failure",
			[]printedEdit{{mergeOff, "if !(p.Silent > 0) {"}}, noExp, here},
		{"a merge without expectations leaves the position",
			[]printedEdit{{"\t\t\t\t\tp.Farthest = far\n", ""}}, noExp, here},
		{"a merge without expectations has no placeholder",
			[]printedEdit{{mergeFill, ""}}, noExp, here},
		{"a merge without expectations grows a placeholder for each",
			[]printedEdit{{mergeFill, "\t\t\t\tp.Exp = append(p.Exp, ExpFar)\n"}}, noExp, here},
		{"an empty set is kept like the last",
			[]printedEdit{{"\tif n == 0 {\n\t\treturn nil\n\t}\n\tlast := p.LastKept\n", "\tlast := p.LastKept\n"}}, kept, here},
		{"a prefix of the set kept last reuses it",
			[]printedEdit{{"\tif len(last) != n {\n", "\tif len(last) < n {\n"}}, chunks, here},
		{"a set of the same length reuses the last",
			[]printedEdit{{"\t\tif last[i] != s[i] {\n\t\t\tgoto copy\n\t\t}\n", "\t\t_ = last[i]\n"}}, kept, here},
		{"comparing with the set kept last charges nothing",
			[]printedEdit{{"i < n; i++ {\n\t\tp.Spend()\n\t\tif last[i]", "i < n; i++ {\n\t\tif last[i]"}}, kept, here},
		{"an equal set is copied",
			[]printedEdit{{"\treturn last\ncopy:\n", "\tgoto copy\ncopy:\n"}}, kept, here},
		{"a chunk charges no memory",
			[]printedEdit{{"\t\tp.MemUsed += uint64(cap(p.Arena)) * expIDBytes\n", ""}}, chunks, here},
		{"a chunk is smaller than a large set",
			[]printedEdit{{"max(1024, len(s))", "1024"}}, chunks, here},
		{"a set overflows the room left in the chunk",
			[]printedEdit{{"if cap(p.Arena)-len(p.Arena) < len(s) {", "if cap(p.Arena) < len(s) {"}}, chunks, here},
		{"a copy does not become the set kept last",
			[]printedEdit{{last + "\tk := p.LastKept\n\treturn k\n", "\tk := p.Arena[off:len(p.Arena):len(p.Arena)]\n\treturn k\n"}}, kept, here},
		{"a kept set can grow into the next",
			[]printedEdit{{last, "\tp.LastKept = p.Arena[off:len(p.Arena)]\n"}}, kept, here},
	})
}

// TestOperatorsLoopMutantsFail is the negative control of the tests of the Operators Rule loop, whose
// recursive and split forms are printed into core/operators_exec.go. Each row breaks one function of
// the file; most fail the direct tests of both forms, and the rest fail the test named for them. Four
// edits are left out because they change nothing. A prefix or a postfix match that consumes nothing
// is no candidate, so a postfix application always moves past where the iteration started, and the
// recursive and the split test for no progress after one cannot fire. LongestStep saves the frame
// and the cut flag of its caller after restoring them from the record, so not saving them again
// leaves the same values.
func TestOperatorsLoopMutantsFail(t *testing.T) {
	const path = "core/operators_exec.go"
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	groups := []struct {
		tests, pkg string
		mutants    [][]funcEdit
	}{
		{"^TestOperators", "./core", [][]funcEdit{
			{{"OperatorsRuleParse", "\tp.Depth--\n", ""}},
			{{"operatorsRuleExpr", "if !ok {\n\t\treturn nil, false\n\t}\n\tled :=", "if false && !ok {\n\t\treturn nil, false\n\t}\n\tled :="}},
			{{"operatorsRuleExpr", "if len(led) == 0 {", "if false {"}},
			{{"operatorsRuleExpr", "level <= min ||", "level < min ||"}},
			{{"operatorsRuleExpr", "(op.Assoc == grammar.AssocNone) && level == lastNone", "false && level == lastNone"}},
			{{"operatorsRuleExpr", "\t\tp.Pos = a.End\n", ""}},
			{{"operatorsRuleExpr", "\t\tp.Env = a.Env\n", ""}},
			{{"operatorsRuleExpr", "if p.Missing != nil || len(a.Errs) != 0 {", "if len(a.Errs) != 0 {"}},
			{{"operatorsRuleExpr", "lhs, nil)\n\t\t\tlastNone = -1\n", "lhs, nil)\n"}},
			{{"operatorsRuleExpr", "\t\t\tcontinue\n\t\t}\n\t\trmin", "\t\t}\n\t\trmin"}},
			{{"operatorsRuleExpr", "if op.Assoc == grammar.AssocRight {", "if true {"}},
			{{"operatorsRuleExpr", "\t\t\tp.Reset(m0)\n\t\t\tbreak\n\t\t}\n\t\tlhs = p", "\t\t\tbreak\n\t\t}\n\t\tlhs = p"}},
			{{"operatorsRuleExpr", "lhs, rhs)\n\t\tlastNone = -1\n", "lhs, rhs)\n"}},
			{{"operatorsRuleExpr", "\t\t\tlastNone = level\n", ""}},
			{{"operatorsRuleExpr", "\t\tif p.Pos == start {\n\t\t\tbreak\n\t\t}\n\t}\n\treturn lhs", "\t}\n\treturn lhs"}},
			{{"ExprStart", "if !ok {", "if false && !ok {"}},
			{{"ExprStart", "if len(r.OperatorsRule.Led) == 0 {", "if false {"}},
			{{"ExprLed", "level <= st.Min ||", "level < st.Min ||"}},
			{{"ExprLed", "(op.Assoc == grammar.AssocNone) && level == st.LastNone", "false"}},
			{{"ExprLed", "\tp.Pos = st.A.End\n", ""}},
			{{"ExprLed", "\tp.Env = st.A.Env\n", ""}},
			{{"ExprLed", "if p.Missing != nil || len(st.A.Errs) != 0 {", "if len(st.A.Errs) != 0 {"}},
			{{"ExprLed", "st.Lhs, nil)\n\t\tst.LastNone = -1\n", "st.Lhs, nil)\n"}},
			{{"ExprLed", "if op.Assoc == grammar.AssocRight {", "if true {"}},
			{{"ExprRhs", "\t\tp.Reset(st.M0)\n\t\treturn true, true\n", "\t\treturn true, true\n"}},
			{{"ExprRhs", "\t\tst.LastNone = op.Level\n", ""}},
			{{"ExprRhs", "\tif p.Pos == st.M0.Pos {\n\t\treturn true, true\n\t}\n", ""}},
			{{"operatorsRuleNud", "\tp.skipTrivia(r)\n", ""}},
			{{"operatorsRuleNud", "\t\tp.Pos = a.End\n", ""}},
			{{"operatorsRuleNud", "\t\tp.Env = a.Env\n", ""}},
			{{"operatorsRuleNud", "if p.Missing != nil || len(a.Errs) != 0 {", "if len(a.Errs) != 0 {"}},
			{{"operatorsRuleNud", "\t\tp.Cut = false\n", ""}},
			{{"operatorsRuleNud", "\t\tp.Cut = prevCut\n", "\t\t_ = prevCut\n"}},
			{{"operatorsRuleNud", "\t\tp.Reset(m0)\n\t\tif cut {", "\t\tif cut {"}},
			{{"operatorsRuleNud", "\t\tif cut {", "\t\tif cut && false {"}},
			{{"NudPrefix", "\t\tp.Pos = st.A.End\n", ""}},
			{{"NudPrefix", "\t\tp.Env = st.A.Env\n", ""}},
			{{"NudPrefix", "if p.Missing != nil || len(st.A.Errs) != 0 {", "if len(st.A.Errs) != 0 {"}},
			{{"NudNext", "\tp.Cut = false\n", ""}},
			{{"NudNext", "\tst.PrevCut = prevCut\n", "\t_ = prevCut\n"}},
			{{"NudOperand", "\tp.Cut = st.PrevCut\n", ""}},
			{{"NudOperand", "\tp.Reset(st.M0)\n\tif cut {", "\tif cut {"}},
			{{"NudOperand", "\tif cut {", "\tif cut && false {"}},
			{{"Longest", "\t\tp.Cut = false\n", ""}},
			{{"Longest", "\t\tp.Cut = prevCut\n", "\t\t_ = prevCut\n"}},
			{{"Longest", "\t\tp.Frame = prevFrame\n", "\t\t_ = prevFrame\n"}},
			{{"Longest", "\t\tp.Frame = f\n", ""}},
			{{"Longest", "(op.Kind != grammar.Infix)", "true"}},
			{{"Longest", "matched := ok && !(p.Pos == start && (op.Kind != grammar.Infix))", "matched := ok"}},
			{{"Longest", "if !matched && cut {", "if cut {"}},
			{{"Longest", "if !matched && cut {", "if false && cut {"}},
			{{"Longest", "Errs: p.copyErrors(p.Recovered[m0.Recovered:])", "Errs: nil"}},
			{{"Longest", "missing: p.MissingEvents(m0.missing)", "missing: nil"}},
			{{"Longest", "Env: p.Env,", "Env: m0.Env,"}},
			{{"Longest", "Cut: cut,", "Cut: false,"}},
			{{"Longest", "\t\t\tp.Reset(m0)\n\t\t} else {", "\t\t} else {"}},
			{{"Longest", "\tp.cutFailed = cutFailed\n", "\t_ = cutFailed\n"}},
			{{"LongestNext", "\t\tp.cutFailed = st.CutFailed\n", ""}},
			{{"LongestNext", "\tp.Cut = false\n", ""}},
			{{"LongestNext", "\tst.PrevCut = prevCut\n", "\t_ = prevCut\n"}},
			{{"LongestNext", "\tst.PrevFrame = prevFrame\n", "\t_ = prevFrame\n"}},
			{{"LongestNext", "\tp.Frame = f\n", ""}},
			{{"LongestStep", "\tp.Cut = st.PrevCut\n", ""}},
			{{"LongestStep", "\tp.Frame = st.PrevFrame\n", ""}},
			{{"LongestStep", "(op.Kind != grammar.Infix)", "true"}},
			{{"LongestStep", "matched := ok && !(p.Pos == st.M0.Pos && (op.Kind != grammar.Infix))", "matched := ok"}},
			{{"LongestStep", "if !matched && cut {", "if cut {"}},
			{{"LongestStep", "if !matched && cut {", "if false && cut {"}},
			{{"LongestStep", "Errs: p.copyErrors(p.Recovered[st.M0.Recovered:])", "Errs: nil"}},
			{{"LongestStep", "missing: p.MissingEvents(st.M0.missing)", "missing: nil"}},
			{{"LongestStep", "Env: p.Env,", "Env: st.M0.Env,"}},
			{{"LongestStep", "Cut: cut,", "Cut: false,"}},
			{{"LongestStep", "\t\tp.Reset(st.M0)\n\t} else {", "\t} else {"}},
			{{"skipTrivia", "\tp.Silent = p.Silent + 1\n", ""}, {"skipTrivia", "\tp.Silent = p.Silent - 1\n", ""}},
			{{"skipTrivia", "\t\tp.Reset(m)\n", ""}},
			{{"skipTrivia", "\t\tif p.Missing != nil {\n\t\t\tp.ResetMissing(m.missing)\n\t\t}\n", ""}},
			{{"skipTrivia", "\t\tp.Recovered = p.Recovered[:m.Recovered]\n", ""}},
			{{"TriviaBegin", "\tp.Silent = p.Silent + 1\n", ""}, {"TriviaEnd", "\tp.Silent = p.Silent - 1\n", ""}},
			{{"TriviaEnd", "\t\tp.Reset(m)\n", ""}},
			{{"TriviaEnd", "\t\tif p.Missing != nil {\n\t\t\tp.ResetMissing(m.missing)\n\t\t}\n", ""}},
			{{"TriviaEnd", "\t\tp.Recovered = p.Recovered[:m.Recovered]\n", ""}},
			{{"Reset", "\tp.Pos = m.Pos\n", ""}},
			{{"Reset", "\tp.Env = m.Env\n", ""}},
			{{"Reset", "\tif p.Missing != nil {\n\t\tp.ResetMissing(m.missing)\n\t}\n", ""}},
			{{"Reset", "\tp.Recovered = p.Recovered[:m.Recovered]\n", ""}},
			{{"ExprRhs", "st.Lhs, rhs)\n\tst.LastNone = -1\n", "st.Lhs, rhs)\n"}},
			{{"LongestStep", "\t\tp.cutFailed = st.CutFailed\n", ""}},
			{{"LongestStep", "\tp.Cut = false\n", ""}},
			{{"LongestStep", "\tp.Frame = f\n", ""}},
			{{"LongestStep", "\tst.I = st.I + 1\n", "\tst.I = st.I + 2\n"}},
		}},
		{"^TestBackendsMatchOnCorpus$", ".", [][]funcEdit{
			{{"Longest", "p.Pos > best.End", "p.Pos >= best.End"}},
			{{"LongestStep", "p.Pos > st.Best.End", "p.Pos >= st.Best.End"}},
		}},
		{"^TestBudgetCoarseChargesPerIteration$", ".", [][]funcEdit{
			{{"operatorsRuleNud", "\t\tp.Spend()\n", ""}},
			{{"NudNext", "\tp.Spend()\n", ""}},
			{{"LongestBegin", "\tp.SpendN(uint64(len(l)))\n", ""}},
			// One unit for the whole match instead of one per line.
			{{"Longest", "\tp.SpendN(uint64(len(l)))\n", "\tif len(l) > 0 {\n\t\tp.Spend()\n\t}\n"}},
			{{"LongestBegin", "\tp.SpendN(uint64(len(l)))\n", "\tif len(l) > 0 {\n\t\tp.Spend()\n\t}\n"}},
		}},
		{"^TestBudgetDiagnosticRecoveredCopies$", "./vm", [][]funcEdit{
			{{"Longest", "\tp.SpendN(uint64(len(l)))\n", ""}},
		}},
		{"^TestBudgetResetRestoresRepeatedCaptureWritesInReverse$", ".", [][]funcEdit{
			{{"Reset", "\tp.Trail = p.Trail[:m.Trail]\n", ""}},
		}},
		{"^TestBudgetStateOperatorsRuleFrameSnapshotAndRestoration$", "./vm", [][]funcEdit{
			{{"Longest", "\t\t\tp.Saved = AppendSpend(p, p.Saved[:0], f.Vals)\n\t\t\tp.Reset(m0)\n\t\t\tCopySpend(p, f.Vals, p.Saved)\n", "\t\t\tp.Reset(m0)\n"}},
			{{"Longest", "\t\t\tp.Reset(m0)\n\t\t\tCopySpend", "\t\t\tCopySpend"}},
			{{"LongestStep", "\t\tp.Saved = AppendSpend(p, p.Saved[:0], st.F.Vals)\n\t\tp.Reset(st.M0)\n\t\tCopySpend(p, st.F.Vals, p.Saved)\n", "\t\tp.Reset(st.M0)\n"}},
			{{"LongestStep", "\t\tp.Reset(st.M0)\n\t\tCopySpend", "\t\tCopySpend"}},
			{{"Reset", "\tp.UndoCaptures(m.Trail)\n", ""}},
		}},
		{"^TestCompiledRecognitionRoundTrip$", ".", [][]funcEdit{
			{{"ExprLed", "if p.Missing != nil || len(st.A.Errs) != 0 {", "if p.Missing != nil {"}},
			{{"operatorsRuleNud", "if p.Missing != nil || len(a.Errs) != 0 {", "if p.Missing != nil {"}},
			{{"operatorsRuleNud", "p.OperatorsRuleParse(r, a.Op.Level)", "p.OperatorsRuleParse(r, a.Op.Level-1)"}},
			{{"NudRhs", "\tif st.A.Cut {\n\t\treturn nil, false, true\n\t}\n", ""}},
			{{"NudRhs", "\tp.Reset(st.M0)\n", ""}},
			{{"LongestStep", "((st.Best.Op == nil) || p.Pos > st.Best.End)", "true"}},
		}},
		{"^TestDefaultValueOperatorsRule$", ".", [][]funcEdit{
			{{"operatorsRuleNud", "LineResult(r, o, f, v, start, nil)", "LineResult(r, o, f, v, p.Pos+0*start, nil)"}},
		}},
		{"^TestDocumentAbortedParse$", "./document", [][]funcEdit{
			{{"OperatorsRuleParse", "\tp.Enter()\n", ""}, {"OperatorsRuleParse", "\tp.Depth--\n", ""}},
		}},
		{"^TestDocumentEditKeepsMemo$", "./document", [][]funcEdit{
			{{"operatorsRuleNud", "\t\tp.Frame = f\n", ""}},
		}},
		{"^TestDocumentMatchesFreshParse$", ".", [][]funcEdit{
			{{"operatorsRuleNud", "\t\tp.Frame = prevFrame\n", "\t\t_ = prevFrame\n"}},
		}},
		{"^TestDocumentMovesReusedTrees$", ".", [][]funcEdit{
			{{"NudNext", "\tp.Frame = f\n", ""}},
			{{"Longest", "Start: start,", "Start: p.Pos,"}},
			{{"LongestStep", "Start: st.M0.Pos,", "Start: p.Pos,"}},
		}},
		{"^TestIgnoreCaseOperatorsRulePragma$", ".", [][]funcEdit{
			{{"operatorsRuleExpr", "\t\tp.skipTrivia(r)\n", ""}},
		}},
		{"^TestOperatorsRuleActions$", ".", [][]funcEdit{
			{{"operatorsRuleExpr", "failed := p.cutFailed\n\t\t\tp.Reset(m0)\n", "failed := p.cutFailed\n"}},
			{{"ExprLed", "failed := p.cutFailed\n\t\tp.Reset(st.M0)\n", "failed := p.cutFailed\n"}},
			{{"NudNext", "\tst.PrevFrame = prevFrame\n", "\t_ = prevFrame\n"}},
		}},
		{"^TestOperatorsRuleCST$", ".", [][]funcEdit{
			{{"operatorsRuleExpr", "\t\t\trmin--\n", ""}},
		}},
		{"^TestOperatorsRuleCut$", ".", [][]funcEdit{
			{{"operatorsRuleExpr", "\t\t\tif a.Cut {\n\t\t\t\treturn nil, false\n\t\t\t}\n", ""}},
			{{"ExprRhs", "\t\tif st.A.Cut {\n\t\t\treturn true, false\n\t\t}\n", ""}},
		}},
		{"^TestOperatorsRuleJuxtaposition$", ".", [][]funcEdit{
			{{"ExprLed", "\t\tp.Reset(st.M0)\n\t\treturn true, true, -1\n\t}\n\tp.Pos = st.A.End", "\t\treturn true, true, -1\n\t}\n\tp.Pos = st.A.End"}},
		}},
		{"^TestOperatorsRulePositionsWithActions$", ".", [][]funcEdit{
			{{"operatorsRuleExpr", "\t\t\tp.Reset(m0)\n\t\t\tbreak\n\t\t}\n\t\tp.Pos = a.End", "\t\t\tbreak\n\t\t}\n\t\tp.Pos = a.End"}},
		}},
		{"^TestOperatorsRulePrefixPartCuts$", ".", [][]funcEdit{
			{{"NudPrefix", "} else if p.cutFailed {", "} else if false {"}},
			{{"Longest", "((best.Op == nil) || p.Pos > best.End)", "true"}},
		}},
		{"^TestOperatorsRuleStreamDepthLimit$", ".", [][]funcEdit{
			{{"ExprLed", "\t\trmin--\n", ""}},
		}},
		{"^TestRuleResultChecks$", ".", [][]funcEdit{
			{{"NudOperand", "LineResult(r, o, st.F, v, st.M0.Pos, nil)", "LineResult(r, o, st.F, v, p.Pos, nil)"}},
		}},
		{"^TestSampleCorpus$", ".", [][]funcEdit{
			{{"operatorsRuleNud", "\t\tif a.Cut {\n\t\t\treturn nil, false\n\t\t}\n", ""}},
			{{"operatorsRuleNud", "\t\tp.Reset(m0)\n\t} else if", "\t} else if"}},
			{{"operatorsRuleNud", "} else if p.cutFailed {", "} else if false {"}},
			{{"LongestStep", "((st.Best.Op == nil) || p.Pos > st.Best.End)", "(st.Best.Op == nil)"}},
		}},
		{"^TestTerminalOperatorsRuleNeedsAction$", ".", [][]funcEdit{
			{{"NudOperand", "\tp.Frame = st.PrevFrame\n", ""}},
		}},
		{"^TestTracesAgreeAcrossBackends$", ".", [][]funcEdit{
			{{"operatorsRuleExpr", "if failed {", "if failed && false {"}},
			{{"operatorsRuleExpr", "if p.Missing != nil || len(a.Errs) != 0 {", "if p.Missing != nil {"}},
			{{"ExprLed", "if failed {", "if failed && false {"}},
			{{"NudPrefix", "if p.Missing != nil || len(st.A.Errs) != 0 {", "if p.Missing != nil {"}},
			{{"Longest", "((best.Op == nil) || p.Pos > best.End)", "(best.Op == nil)"}},
		}},
	}
	var mutants []printedMutant
	for _, g := range groups {
		for _, edits := range g.mutants {
			mutants = append(mutants, printedMutant{mutantName(edits),
				[]printedEdit{{string(src), applyFuncEdits(t, string(src), edits)}}, g.tests, []string{g.pkg}})
		}
	}
	runPrintedMutants(t, path, mutants)
}

// TestStreamCommitMutantsFail is the negative control of the tests of the engine's stream commit,
// which is printed from Exec IR (core/stream_exec.go), and of its implementations of the operations
// (core/stream_ops.go). The operations of the window are host operations without reference models,
// so these mutants are what binds them. Each must fail the test named for it; the two backends'
// calls of the commit are held to their position too.
func TestStreamCommitMutantsFail(t *testing.T) {
	const (
		check   = "\tif BudgetControlsEnabled {\n\t\tp.Control.Check()\n\t}\n"
		report  = "\tif p.Diagnostic != nil || p.DiagnosticTail.Limit > 0 {\n\t\tp.streamReport()\n\t}\n"
		deliver = "\tif v != nil {\n\t\tvalue.D(v).Fresh = false\n\t}\n\tp.EmitCall = true\n\terr := p.Emit(v)\n\tp.EmitCall = false\n\tif err != nil {\n\t\tpanic(Fatal{Err: err})\n\t}\n"
		window  = "pos-p.Memo.Base >= 1024"
		prune   = "\tif " + window + " {\n\t\tp.streamDropRecords(pos)\n\t\tp.PruneStreamMemo(pos)\n\t}\n"
		settle  = "\tif p.DiagnosticTail.Limit > 0 {\n\t\tp.streamSettleErrors()\n\t}\n"
		depth   = "(!rec || p.Depth == 1)"
		reuse   = "if p.Tr == nil && cap(p.Recovered) <= streamRecoveryScratchMax {"

		boundary = "^TestBudgetStreamEmitBoundary$"
		after    = "^TestBudgetStreamCommitAfterDelivery$"
		order    = "^TestStreamDiagnosticCallbackReportsAllElementRecoveriesBeforeEmit$"
		storage  = "^TestStreamStorageCommitReferences$"
		stack    = "^TestBudgetStreamDiscardAndStackClear$"
		slide    = "^TestStreamCommitWindow$"
		records  = "^TestStreamCommitDropsRecordsBeforeItsPosition$"
		lefts    = "^TestStreamOutsideLeftRecursion$"
		start    = "^TestStreamStartRuleNeedsNoCount$"
		scratch  = "^TestStreamDiagnosticTailCommitScratchOwnership$"
		bounds   = "^TestStreamCommitBoundsTheErrorScratch$"
	)
	here, vm, inCore := []string{"."}, []string{"./vm"}, []string{"./core"}
	runPrintedMutants(t, "core/stream_exec.go", []printedMutant{
		{"cancellation is not observed before the delivery",
			[]printedEdit{{check + report, report}}, boundary, here},
		{"the recovered errors are not reported",
			[]printedEdit{{report, ""}}, order, here},
		{"a callback alone gets no report",
			[]printedEdit{{"if p.Diagnostic != nil || p.DiagnosticTail.Limit > 0 {", "if p.DiagnosticTail.Limit > 0 {"}}, order, here},
		{"a tail alone gets no report",
			[]printedEdit{{"if p.Diagnostic != nil || p.DiagnosticTail.Limit > 0 {", "if p.Diagnostic != nil {"}}, "^TestStreamCommitFeedsATailWithoutCallback$", here},
		{"the recovered errors are reported after their element",
			[]printedEdit{{report + deliver, deliver + report}}, order, here},
		{"the element is not delivered",
			[]printedEdit{{"\terr := p.Emit(v)\n", "\tvar err error\n"}}, boundary, here},
		{"a delivered element stays fresh",
			[]printedEdit{{"\tif v != nil {\n\t\tvalue.D(v).Fresh = false\n\t}\n", ""}}, "^TestStreamCommitFreezesDeliveredElements$", here},
		{"the callback is not marked as the caller's",
			[]printedEdit{{"\tp.EmitCall = true\n", ""}}, "^TestStreamCallerPanicIdentity$", here},
		{"the mark of the callback stays",
			[]printedEdit{{"\tp.EmitCall = false\n", ""}}, boundary, here},
		{"an error of the callback is ignored",
			[]printedEdit{{"\tif err != nil {\n\t\tpanic(Fatal{Err: err})\n\t}\n", "\t_ = err\n"}}, "^TestParseStreamEmitError$", here},
		{"cancellation is not observed after the delivery",
			[]printedEdit{{deliver + check, deliver}}, boundary, here},
		{"a delivered element is not committed",
			[]printedEdit{{"\tp.Commit(end)\n", ""}}, "^TestParseStreamDiscardsInput$", here},
		{"the commit passes the end of the element",
			[]printedEdit{{"\tp.Commit(end)\n", "\tp.Commit(end + 1)\n"}}, after, here},
		{"the commit stops before the end of the element",
			[]printedEdit{{"\tp.Commit(end)\n", "\tp.Commit(end - 1)\n"}}, after, here},
		{"the input before the element is kept",
			[]printedEdit{{"\tp.Discard(pos - 1)\n", ""}}, "^TestParseStreamDiscardsInput$", here},
		{"the unit before the element is discarded",
			[]printedEdit{{"p.Discard(pos - 1)", "p.Discard(pos)"}}, stack, vm},
		{"two units before the element are kept",
			[]printedEdit{{"p.Discard(pos - 1)", "p.Discard(pos - 2)"}}, stack, vm},
		{"what the elements held is not released",
			[]printedEdit{{"\tp.streamRelease()\n", ""}}, storage, vm},
		{"the release runs before the discard",
			[]printedEdit{{"\tp.Discard(pos - 1)\n\tp.streamRelease()\n", "\tp.streamRelease()\n\tp.Discard(pos - 1)\n"}},
			"^TestStreamCommitDiscardsBeforeItReleases$", here},
		{"a whole window does not prune", []printedEdit{{window, "pos-p.Memo.Base > 1024"}}, slide, inCore},
		{"less than a window prunes", []printedEdit{{window, "pos-p.Memo.Base >= 1023"}}, slide, inCore},
		{"the window ignores the floor", []printedEdit{{window, "pos >= 1024"}}, slide, inCore},
		{"the records stay", []printedEdit{{"\t\tp.streamDropRecords(pos)\n", ""}}, records, inCore},
		{"the record at the commit goes", []printedEdit{{"p.streamDropRecords(pos)", "p.streamDropRecords(pos + 1)"}}, records, inCore},
		{"the record before the commit stays", []printedEdit{{"p.streamDropRecords(pos)", "p.streamDropRecords(pos - 1)"}}, records, inCore},
		{"the memo is not pruned", []printedEdit{{"\t\tp.PruneStreamMemo(pos)\n", ""}}, slide, inCore},
		{"the memo loses the commit position", []printedEdit{{"p.PruneStreamMemo(pos)", "p.PruneStreamMemo(pos + 1)"}}, slide, inCore},
		{"the memo keeps the position before the commit", []printedEdit{{"p.PruneStreamMemo(pos)", "p.PruneStreamMemo(pos - 1)"}}, slide, inCore},
		{"the commit settles errors without a tail",
			[]printedEdit{{"if p.DiagnosticTail.Limit > 0 {", "if true {"}}, "^TestStreamElementRecovery$", here},
		{"the commit never settles errors",
			[]printedEdit{{"if p.DiagnosticTail.Limit > 0 {", "if false {"}}, scratch, here},
		{"the errors settle before the pruning",
			[]printedEdit{{prune + settle, settle + prune}}, "^TestStreamCommitPrunesBeforeItSettles$", here},
		{"a parse without a callback streams",
			[]printedEdit{{"(p.Emit != nil) && p.StreamID == id", "p.StreamID == id"}}, "^TestRunSitesMatchParsesWithoutRecords$", here},
		{"every rule streams", []printedEdit{{"(p.Emit != nil) && p.StreamID == id", "(p.Emit != nil)"}}, start, here},
		{"a recursive rule streams at any depth", []printedEdit{{depth, "true"}}, lefts, here},
		{"a recursive rule never streams", []printedEdit{{depth, "!rec"}}, start, here},
		{"a rule that is entered once counts its depth", []printedEdit{{depth, "p.Depth == 1"}}, after, here},
		{"a recursive rule streams outside its first entry", []printedEdit{{"p.Depth == 1)", "p.Depth == 0)"}}, start, here},
	})
	runPrintedMutants(t, "core/stream_ops.go", []printedMutant{
		{"an error of the diagnostic callback is ignored",
			[]printedEdit{{"if err := p.ReportRecovered(); err != nil {\n\t\tpanic(Fatal{Err: err})\n\t}", "_ = p.ReportRecovered()"}},
			"^TestStreamDiagnosticCallbackWithoutElementReceiverStopsBeforeLaterRead$", here},
		{"the trail stays", []printedEdit{{"\tp.Trail = p.Trail[:0]\n", ""}}, storage, vm},
		{"the frames stay", []printedEdit{{"\tif StreamFrameScratch {\n\t\tp.ResetStreamFrames()\n\t}\n", ""}}, after, here},
		{"the chunks stay", []printedEdit{{"\tp.splitChunks()\n", ""}}, "^TestParseStreamMemoryIsBounded$", here},
		{"the inactive trail stays",
			[]printedEdit{{"\tif StreamUndoCleanup {\n\t\tClearSpend(p, p.Trail[:cap(p.Trail)])\n\t}\n", ""}}, storage, vm},
		{"the inactive stack stays",
			[]printedEdit{{"\tif StreamStackCleanup && p.Backend != nil {\n\t\tp.Backend.ClearInactive()\n\t}\n", ""}}, storage, vm},
		{"the skip records stay", []printedEdit{{"\tp.PruneSkipRecords(pos)\n", ""}}, records, inCore},
		{"a traced parse reuses the scratch", []printedEdit{{reuse, "if cap(p.Recovered) <= streamRecoveryScratchMax {"}}, scratch, here},
		{"a large scratch is kept", []printedEdit{{reuse, "if p.Tr == nil {"}}, bounds, here},
		{"the scratch is never reused", []printedEdit{{reuse, "if false {"}}, scratch, here},
		{"the settled errors stay in the scratch", []printedEdit{{"\t\tClearSpend(p, p.Recovered[:cap(p.Recovered)])\n", ""}}, scratch, here},
		{"the settled errors stay recorded", []printedEdit{{"\t\tp.Recovered = p.Recovered[:0]\n", ""}}, scratch, here},
		{"the settled errors of a large scratch stay", []printedEdit{{"\t\tp.Recovered = nil\n", ""}}, scratch, here},
		{"the count of reported errors stays", []printedEdit{{"\tp.Diagnosed = 0\n", ""}}, "^TestStreamCommitForgetsTheDiagnosedCount$", here},
		{"the settled errors stay counted", []printedEdit{{"\t\tp.Control.errs = 0\n", ""}}, "^TestMemoryLimitStream$", here},
	})
	runPrintedMutants(t, "closure/builder.go", []printedMutant{
		{"the closure backend commits where the element started",
			[]printedEdit{{"p.EmitStream(v, p.Pos)", "p.EmitStream(v, m0.Pos)"}}, after, here},
	})
	runPrintedMutants(t, "vm/vm.go", []printedMutant{
		{"the VM commits where the element started",
			[]printedEdit{{"p.EmitStream(asNodeValue(mc.pop()), p.Pos)", "p.EmitStream(asNodeValue(mc.pop()), e.save.pos)"}}, after, here},
	})
}

// A funcEdit replaces the only copy of old in the function fn of the printed file.
type funcEdit struct{ fn, old, new string }

// applyFuncEdits returns the text with the edits made.
func applyFuncEdits(t *testing.T, text string, edits []funcEdit) string {
	t.Helper()
	for _, e := range edits {
		head := "func (p *Parser) " + e.fn + "("
		start := strings.Index(text, head)
		if start < 0 {
			t.Fatalf("no function %s", e.fn)
		}
		length := strings.Index(text[start:], "\n}\n") + 1
		body := text[start : start+length]
		if n := strings.Count(body, e.old); n != 1 {
			t.Fatalf("%s has %d copies of %q, not one", e.fn, n, e.old)
		}
		text = text[:start] + strings.Replace(body, e.old, e.new, 1) + text[start+length:]
	}
	return text
}

// mutantName names a mutant after its first edit.
func mutantName(edits []funcEdit) string {
	e := edits[0]
	return fmt.Sprintf("%s %q to %q", e.fn, strings.TrimSpace(e.old), strings.TrimSpace(e.new))
}

// A printedMutant is one broken variant of the printed file.
type printedMutant struct {
	name  string
	edits []printedEdit
	// tests is the -run pattern, pkgs the packages, of the tests that detect the mutant.
	tests string
	pkgs  []string
}

// A printedEdit replaces the only copy of old in the printed file.
type printedEdit struct{ old, new string }

// runPrintedMutants builds the tests of each mutant with the mutated copy of the printed file path
// in place of the file, and requires that they fail.
func runPrintedMutants(t *testing.T, path string, mutants []printedMutant) {
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for i, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			text := string(src)
			for _, e := range m.edits {
				if n := strings.Count(text, e.old); n != 1 {
					t.Fatalf("%s has %d copies of %q, not one", path, n, e.old)
				}
				text = strings.Replace(text, e.old, e.new, 1)
			}
			mutant := filepath.Join(dir, fmt.Sprintf("mutant_%d.go", i))
			if err := os.WriteFile(mutant, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			overlay, err := json.Marshal(map[string]map[string]string{"Replace": {file: mutant}})
			if err != nil {
				t.Fatal(err)
			}
			ov := mutant + ".json"
			if err := os.WriteFile(ov, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"test", "-count=1", "-overlay", ov, "-run", m.tests}, m.pkgs...)
			cmd := exec.Command("go", args...)
			// The tests run in this module, which resolves without -mod=mod (TestMain).
			cmd.Env = append(os.Environ(), "GOFLAGS=-trimpath")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the tests passed with the mutant:\n%s", out)
			}
			if !strings.Contains(string(out), "--- FAIL: "+strings.Trim(m.tests, "^$")) {
				t.Fatalf("the tests failed for another reason than a failing test:\n%s", out)
			}
		})
	}
}
