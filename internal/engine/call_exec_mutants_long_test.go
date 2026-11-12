//go:build zp_long

package engine_test

import (
	"encoding/json"
	"fmt"
	"maps"
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
	const copyErrs = "Errs: p.recSince(rec)}"
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
		{"a call takes its recovered errors before it keeps its expectations",
			[]printedEdit{
				{"\tfar, inner := p.Unisolate(m)\n\tkept := p.Keep(inner)\n\tstore := storeMemo\n",
					"\terrs := p.recSince(rec)\n\tfar, inner := p.Unisolate(m)\n\tkept := p.Keep(inner)\n\tstore := storeMemo\n"},
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
			{{"operatorsRuleExpr", "if p.Missing != nil || !a.Errs.None() {", "if !a.Errs.None() {"}},
			{{"operatorsRuleExpr", "lhs, nil)\n\t\t\tlastNone = -1\n", "lhs, nil)\n"}},
			{{"operatorsRuleExpr", "\t\t\tcontinue\n\t\t}\n\t\trmin", "\t\t}\n\t\trmin"}},
			{{"operatorsRuleExpr", "rmin := level\n\t\tif op.Assoc == grammar.AssocRight {", "rmin := level\n\t\tif true {"}},
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
			{{"ExprLed", "if p.Missing != nil || !st.A.Errs.None() {", "if !st.A.Errs.None() {"}},
			{{"ExprLed", "st.Lhs, nil)\n\t\tst.LastNone = -1\n", "st.Lhs, nil)\n"}},
			{{"ExprLed", "rmin := level\n\tif op.Assoc == grammar.AssocRight {", "rmin := level\n\tif true {"}},
			{{"ExprRhs", "\t\tp.Reset(st.M0)\n\t\treturn true, true\n", "\t\treturn true, true\n"}},
			{{"ExprRhs", "\t\tst.LastNone = op.Level\n", ""}},
			// The bound of the loop: the tightest level that may follow the start or the last operator.
			{{"operatorsRuleExpr", "level > bound ||", "level > bound && false ||"}},
			{{"operatorsRuleExpr", "bound := p.nudBound", "bound := 1073741824"}},
			{{"operatorsRuleExpr", "\t\t\tbound = 1073741824\n", ""}},
			{{"operatorsRuleExpr", "\t\tbound = rbound\n", ""}},
			{{"operatorsRuleExpr", "rbound := level\n\t\tif op.Assoc == grammar.AssocRight {", "rbound := level\n\t\tif false {"}},
			{{"ExprLed", "level <= st.Min || level > st.Bound ||", "level <= st.Min ||"}},
			{{"ExprStart", "\tst.Bound = p.nudBound\n", "\tst.Bound = 1073741824\n"}},
			{{"ExprLed", "\t\tst.Bound = 1073741824\n", ""}},
			{{"ExprRhs", "\tst.Bound = rbound\n", ""}},
			{{"ExprRhs", "rbound := op.Level\n\tif op.Assoc == grammar.AssocRight {", "rbound := op.Level\n\tif false {"}},
			{{"operatorsRuleNud", "\t\t\tp.nudBound = a.Op.Level\n", "\t\t\tp.nudBound = 1073741824\n"}},
			{{"operatorsRuleNud", "\t\t\tp.nudBound = 1073741824\n", "\t\t\tp.nudBound = 0\n"}},
			{{"NudRhs", "\t\tp.nudBound = st.A.Op.Level\n", "\t\tp.nudBound = 1073741824\n"}},
			{{"NudOperand", "\t\tp.nudBound = 1073741824\n", "\t\tp.nudBound = 0\n"}},
			{{"operatorsRuleNud", "\tp.skipTrivia(r)\n", ""}},
			{{"operatorsRuleNud", "\t\tp.Pos = a.End\n", ""}},
			{{"operatorsRuleNud", "\t\tp.Env = a.Env\n", ""}},
			{{"operatorsRuleNud", "if p.Missing != nil || !a.Errs.None() {", "if !a.Errs.None() {"}},
			{{"operatorsRuleNud", "\t\tp.Cut = false\n", ""}},
			{{"operatorsRuleNud", "\t\tp.Cut = prevCut\n", "\t\t_ = prevCut\n"}},
			{{"operatorsRuleNud", "\t\tp.Reset(m0)\n\t\tif cut {", "\t\tif cut {"}},
			{{"operatorsRuleNud", "\t\tif cut {", "\t\tif cut && false {"}},
			{{"NudPrefix", "\t\tp.Pos = st.A.End\n", ""}},
			{{"NudPrefix", "\t\tp.Env = st.A.Env\n", ""}},
			{{"NudPrefix", "if p.Missing != nil || !st.A.Errs.None() {", "if !st.A.Errs.None() {"}},
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
			{{"Longest", "Errs: p.recSince(m0.Recovered)", "Errs: RecEntry{}"}},
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
			{{"LongestStep", "Errs: p.recSince(st.M0.Recovered)", "Errs: RecEntry{}"}},
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
		// The right operand of an infix operator applied without consuming input.
		{"^TestOperatorsEmptyInfixTerminates$", ".", [][]funcEdit{
			{{"operatorsRuleExpr", "if start == guard && a.End == start {", "if start == guard && false {"}},
			{{"operatorsRuleExpr", "p.afterEmpty = p.Pos == start", "p.afterEmpty = false"}},
			{{"operatorsRuleExpr", "\tp.afterEmpty = false\n", ""}},
			{{"ExprLed", "if st.M0.Pos == st.Guard && st.A.End == st.M0.Pos {", "if false {"}},
			{{"ExprLed", "p.afterEmpty = p.Pos == st.M0.Pos", "p.afterEmpty = false"}},
			{{"ExprBegin", "\tp.afterEmpty = false\n", ""}},
			// An application that consumed no input ends the loop, which these tests reach.
			{{"ExprRhs", "\tif p.Pos == st.M0.Pos {\n\t\treturn true, true\n\t}\n", ""}},
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
		{"^TestBudgetDiagnosticRecoveredLists$", "./vm", [][]funcEdit{
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
			{{"ExprLed", "if p.Missing != nil || !st.A.Errs.None() {", "if p.Missing != nil {"}},
			{{"operatorsRuleNud", "if p.Missing != nil || !a.Errs.None() {", "if p.Missing != nil {"}},
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
			{{"ExprLed", "level == st.LastNone {\n\t\tp.Reset(st.M0)\n\t\treturn true, true, -1\n", "level == st.LastNone {\n\t\treturn true, true, -1\n"}},
		}},
		{"^TestOperatorsRulePositionsWithActions$", ".", [][]funcEdit{
			{{"operatorsRuleExpr", "level == lastNone {\n\t\t\tp.Reset(m0)\n\t\t\tbreak\n", "level == lastNone {\n\t\t\tbreak\n"}},
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
			{{"operatorsRuleExpr", "if p.Missing != nil || !a.Errs.None() {", "if p.Missing != nil {"}},
			{{"ExprLed", "if failed {", "if failed && false {"}},
			{{"NudPrefix", "if p.Missing != nil || !st.A.Errs.None() {", "if p.Missing != nil {"}},
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
//
// Four edits change nothing that a parse can show, and are left out:
//   - Moving the first checkpoint of EmitStream after the report, or dropping the checkpoint before
//     each diagnostic in ReportRecovered: nothing changes state between the two, and the checkpoint
//     after each diagnostic covers the next one, so each stands in for the other at the same state,
//     except for a cancellation that arrives between them, which no order can fix. Together they
//     are not equivalent, which the mutant that takes both edits shows.
//   - Dropping the clamp of Diagnosed in ReportRecovered: it exceeds the recovered errors only after
//     a rollback past a report. The elements' own marks are taken after the last report, and the
//     compiler rejects #recover around a #stream repetition, so what can roll back further is a
//     failure of the start rule, after which nothing is reported. A panic in its place passes the
//     tests of the engine.
//   - Splitting the chunks before the frames are reset in streamRelease: the split charges nothing
//     and reads only the capacity of the scratch, which the reset keeps, so the charges, the memory
//     counts and what stays referenced end the same, and an interruption between them leaves state
//     that nothing reads (the proof's Lemma 5).
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
		charges  = "^TestStreamCommitChargesItsDeclaredWork$"
		ends     = "^TestStreamDiagnosticCallbackEndsTheEmission$"
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
		{"the scratch of the errors is cleared free of charge",
			[]printedEdit{{"\t\tClearSpend(p, p.Recovered[:cap(p.Recovered)])\n", "\t\tclear(p.Recovered[:cap(p.Recovered)])\n"}}, charges, here},
		{"a panic of the diagnostic callback is the parser's",
			[]printedEdit{{"\t\tp.EmitCall = true\n", ""}}, "^TestStreamDiagnosticCallbackPanicIsTheCallers$", here},
		{"the mark of the diagnostic callback stays",
			[]printedEdit{{"\t\tp.EmitCall = false\n", ""}}, "^TestStreamDiagnosticCallbackMarkEndsWithIt$", here},
		{"cancellation in the last diagnostic is not observed",
			[]printedEdit{{"\t\tp.DiagnosticTail.Add(original)\n\t}\n\tif BudgetControlsEnabled {\n\t\tp.Control.Check()\n\t}\n", "\t\tp.DiagnosticTail.Add(original)\n\t}\n"}}, ends, here},
		{"the frames are scanned for free",
			[]printedEdit{{"\tif StreamFrameScratch {\n\t\tp.ResetStreamFrames()\n\t}\n", "\tif StreamFrameScratch {\n\t\tp.FrameSlab = p.FrameSlab[:0]\n\t}\n"}}, charges, here},
		{"the settled errors stay in the scratch", []printedEdit{{"\t\tClearSpend(p, p.Recovered[:cap(p.Recovered)])\n", ""}}, scratch, here},
		{"the settled errors stay recorded", []printedEdit{{"\t\tp.Recovered = p.Recovered[:0]\n", ""}}, scratch, here},
		{"the settled errors of a large scratch stay", []printedEdit{{"\t\tp.Recovered = nil\n", ""}}, scratch, here},
		{"the count of reported errors stays", []printedEdit{{"\tp.Diagnosed, p.diagnosedSkip, p.diagnosedList = 0, 0, nil\n", ""}}, "^TestStreamCommitForgetsTheDiagnosedCount$", here},
		{"the settled errors stay counted", []printedEdit{{"\t\tp.Control.errs = 0\n", ""}}, "^TestMemoryLimitStream$", here},
	})
	// The checkpoints before the first diagnostic stand in for each other, so only both go together.
	reportSrc, err := os.ReadFile("core/stream_ops.go")
	if err != nil {
		t.Fatal(err)
	}
	const before = "func (p *Parser) reportRecovered(original *SyntaxError) error {\n\tif BudgetControlsEnabled {\n\t\tp.Control.Check()\n\t}\n"
	if strings.Count(string(reportSrc), before) != 1 {
		t.Fatalf("core/stream_ops.go has no single checkpoint before a diagnostic")
	}
	reportMutant := filepath.Join(t.TempDir(), "stream_ops.go")
	unchecked := strings.Replace(string(reportSrc), before, "func (p *Parser) reportRecovered(original *SyntaxError) error {\n", 1)
	if err := os.WriteFile(reportMutant, []byte(unchecked), 0o644); err != nil {
		t.Fatal(err)
	}
	reportFile, err := filepath.Abs("core/stream_ops.go")
	if err != nil {
		t.Fatal(err)
	}
	runPrintedMutantsOver(t, "core/stream_exec.go", map[string]string{reportFile: reportMutant}, []printedMutant{
		{"a cancelled context still gets its diagnostics",
			[]printedEdit{{check + report, report + check}}, "^TestStreamCancelledContextDeliversNoDiagnostic$", here},
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

// TestDocumentReuseMutantsFail is the negative control of Document reuse: the two printed forms of
// AdvanceEntry, the engine's memo.splice and doc.move in core/document_ops.go, the charges of the runs,
// and the calls of the printed lookup. Each row breaks one step and fails the test it names. Three
// edits are left out because they change nothing a parse can reach.
//   - ShiftOf testing an edit's start for its end: the node of a reused result lies in the input the
//     result examined, and no edit since overlaps that input, so a node is before the start or at or
//     after the end, and the two tests agree.
//   - A lookup that applies the log to an entry that is current: the loop has no edit to apply, and
//     the entry is rewritten with its own values. Only a seed would differ, and the table holds none,
//     since the one store of a call takes its entry after GrowEnd cleared Growing.
//   - A lookup that reads all of p.Edits: a Document sets p.Gen to the length of the log it passes,
//     and no edit reaches the log during a parse.
func TestDocumentReuseMutantsFail(t *testing.T) {
	src, err := os.ReadFile("core/document_exec.go")
	if err != nil {
		t.Fatal(err)
	}
	const budgeted, plain = "func (p *Parser) AdvanceEntry(", "\nfunc AdvanceEntry("
	// form replaces the whole of one printed form, since both have the same lines.
	form := func(head, old, new string) []printedEdit {
		text := string(src)
		start := strings.Index(text, head)
		body := text[start : start+strings.Index(text[start:], "\n}\n")]
		return []printedEdit{{body, strings.Replace(body, old, new, 1)}}
	}
	runPrintedMutants(t, "core/document_exec.go", []printedMutant{
		{"budget: seed kept", form(budgeted, "\tif e.Growing {\n\t\treturn false\n\t}\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: seed always valid", form(budgeted, "\tif e.Growing {\n\t\treturn false\n\t}\n", "\tif e.Growing {\n\t\treturn true\n\t}\n"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: from wrong", form(budgeted, "from := e.From", "from := e.Pos"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: examined wrong", form(budgeted, "examined := e.Examined", "examined := e.End"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: shifted starts false", form(budgeted, "shifted := e.shifted", "shifted := false"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: start at zero", form(budgeted, "for i = int(e.Vgen); i < len(log); i++ {", "for i = 0; i < len(log); i++ {"), "^TestEngineExecConformance$", []string{"./core"}},
		{"budget: one edit short", form(budgeted, "i < len(log); i++ {", "i < len(log)-1; i++ {"), "^TestBudgetDocumentReuseCancellationAndFreshRetry$", []string{"./document"}},
		{"budget: before lt", form(budgeted, "if examined <= log[i].Start {", "if examined < log[i].Start {"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: before never", form(budgeted, "if examined <= log[i].Start {", "if false {"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: before off by one", form(budgeted, "if examined <= log[i].Start {", "if examined <= log[i].Start+1 {"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: after gt", form(budgeted, "from >= log[i].End &&", "from > log[i].End &&"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: after lower", form(budgeted, "from >= log[i].End &&", "from >= log[i].Start &&"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: after always", form(budgeted, "from >= log[i].End &&", "true &&"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: pin ignored", form(budgeted, " && !e.Positional", " "), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: errs ignored", form(budgeted, " && e.Errs.None()", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: invalid keeps going", form(budgeted, "\t\t\treturn false\n\t\t}\n\t\tdelta", "\t\t\tcontinue\n\t\t}\n\t\tdelta"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: from not moved", form(budgeted, "\t\tfrom = from + delta\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: examined not moved", form(budgeted, "\t\texamined = examined + delta\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: shift not summed", form(budgeted, "\t\tshift = shift + delta\n", "\t\tshift = delta\n"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: shifted always", form(budgeted, "shifted = shifted || delta != 0", "shifted = true"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: shifted overwritten", form(budgeted, "shifted = shifted || delta != 0", "shifted = delta != 0"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: shifted never", form(budgeted, "\t\tshifted = shifted || delta != 0\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: pos not moved", form(budgeted, "\te.Pos += shift\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: end not moved", form(budgeted, "\te.End += shift\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: range not stored", form(budgeted, "\te.From, e.Examined = from, examined\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: far not moved", form(budgeted, "\te.Far += shift\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: value shift lost", form(budgeted, "\te.shift += int32(shift)\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: value shift replaced", form(budgeted, "e.shift += int32(shift)", "e.shift = int32(shift)"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: shifted not stored", form(budgeted, "\te.shifted = shifted\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: vgen not stored", form(budgeted, "\te.Vgen = uint32(len(log))\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: partial publish", form(budgeted, "\t\tif !(from >= log[i].End", "\t\te.Vgen = uint32(i)\n\t\tif !(from >= log[i].End"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: seed kept", form(plain, "\tif e.Growing {\n\t\treturn false\n\t}\n", ""), "^TestEngineExecConformance$", []string{"./core"}},
		{"plain: seed always valid", form(plain, "\tif e.Growing {\n\t\treturn false\n\t}\n", "\tif e.Growing {\n\t\treturn true\n\t}\n"), "^TestEngineExecConformance$", []string{"./core"}},
		{"plain: from wrong", form(plain, "from := e.From", "from := e.Pos"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: examined wrong", form(plain, "examined := e.Examined", "examined := e.End"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: shifted starts false", form(plain, "shifted := e.shifted", "shifted := false"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: start at zero", form(plain, "for i = int(e.Vgen); i < len(log); i++ {", "for i = 0; i < len(log); i++ {"), "^TestEngineExecConformance$", []string{"./core"}},
		{"plain: one edit short", form(plain, "i < len(log); i++ {", "i < len(log)-1; i++ {"), "^TestBudgetDocumentReuseCancellationAndFreshRetry$", []string{"./document"}},
		{"plain: before lt", form(plain, "if examined <= log[i].Start {", "if examined < log[i].Start {"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: before never", form(plain, "if examined <= log[i].Start {", "if false {"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: before off by one", form(plain, "if examined <= log[i].Start {", "if examined <= log[i].Start+1 {"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: after gt", form(plain, "from >= log[i].End &&", "from > log[i].End &&"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: after lower", form(plain, "from >= log[i].End &&", "from >= log[i].Start &&"), "^TestEngineExecConformance$", []string{"./core"}},
		{"plain: after always", form(plain, "from >= log[i].End &&", "true &&"), "^TestEngineExecConformance$", []string{"./core"}},
		{"plain: pin ignored", form(plain, " && !e.Positional", " "), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: errs ignored", form(plain, " && e.Errs.None()", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: invalid keeps going", form(plain, "\t\t\treturn false\n\t\t}\n\t\tdelta", "\t\t\tcontinue\n\t\t}\n\t\tdelta"), "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"plain: from not moved", form(plain, "\t\tfrom = from + delta\n", ""), "^TestEngineExecConformance$", []string{"./core"}},
		{"plain: examined not moved", form(plain, "\t\texamined = examined + delta\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: shift not summed", form(plain, "\t\tshift = shift + delta\n", "\t\tshift = delta\n"), "^TestEngineExecConformance$", []string{"./core"}},
		{"plain: shifted always", form(plain, "shifted = shifted || delta != 0", "shifted = true"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: shifted overwritten", form(plain, "shifted = shifted || delta != 0", "shifted = delta != 0"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: shifted never", form(plain, "\t\tshifted = shifted || delta != 0\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: pos not moved", form(plain, "\te.Pos += shift\n", ""), "^TestEngineExecConformance$", []string{"./core"}},
		{"plain: end not moved", form(plain, "\te.End += shift\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: range not stored", form(plain, "\te.From, e.Examined = from, examined\n", ""), "^TestEngineExecConformance$", []string{"./core"}},
		{"plain: far not moved", form(plain, "\te.Far += shift\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: value shift lost", form(plain, "\te.shift += int32(shift)\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: value shift replaced", form(plain, "e.shift += int32(shift)", "e.shift = int32(shift)"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: shifted not stored", form(plain, "\te.shifted = shifted\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: vgen not stored", form(plain, "\te.Vgen = uint32(len(log))\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"plain: partial publish", form(plain, "\t\tif !(from >= log[i].End", "\t\te.Vgen = uint32(i)\n\t\tif !(from >= log[i].End"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: no charge", form(budgeted, "\t\tp.Spend()\n", ""), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: double charge", form(budgeted, "\t\tp.Spend()\n", "\t\tp.Spend()\n\t\tp.Spend()\n"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
		{"budget: charge after decision", form(budgeted, "\t\tp.Spend()\n\t\tif examined <= log[i].Start {\n\t\t\tcontinue\n\t\t}\n", "\t\tif examined <= log[i].Start {\n\t\t\tcontinue\n\t\t}\n\t\tp.Spend()\n"), "^TestAdvanceEntryMatchesItsContract$", []string{"./core"}},
	})
	runPrintedMutants(t, "core/document_ops.go", []printedMutant{
		{"edit not logged", []printedEdit{{"edits = append(edits, ed)\n\tt.Splice", "t.Splice"}}, "^TestBudgetDocumentReuseCancellationAndFreshRetry$", []string{"./document"}},
		{"edit splice wrong end", []printedEdit{{"t.Splice(ed.Start, ed.End, ed.Delta,", "t.Splice(ed.Start, ed.Start, ed.Delta,"}}, "^TestEngineExecConformance$", []string{"./core"}},
		{"edit decides none", []printedEdit{{"return AdvanceEntry(e, edits) })", "return false })"}}, "^TestEngineExecConformance$", []string{"./core"}},
		{"edit keeps all", []printedEdit{{"return AdvanceEntry(e, edits) })", "return true })"}}, "^TestEngineExecConformance$", []string{"./core"}},
		{"move base lost", []printedEdit{{"p.moveBase, p.baseShift = e.Gen, int(e.shift)\n", "p.moveBase, p.baseShift = 0, int(e.shift)\n"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"move shift lost", []printedEdit{{"p.moveBase, p.baseShift = e.Gen, int(e.shift)\n", "p.moveBase, p.baseShift = e.Gen, 0\n"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"entry gen kept", []printedEdit{{"e.Gen, e.shift, e.shifted = p.Gen, 0, false", "e.shift, e.shifted = 0, false"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"entry shift kept", []printedEdit{{"e.Gen, e.shift, e.shifted = p.Gen, 0, false", "e.Gen, e.shifted = p.Gen, false"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"entry shifted kept", []printedEdit{{"e.Gen, e.shift, e.shifted = p.Gen, 0, false", "e.Gen, e.shift = p.Gen, 0"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"nil moved not made", []printedEdit{{"\tif n == nil {\n\t\treturn\n\t}\n\tif p.Moved == nil {\n\t\tp.Moved = map[*value.Node]*value.Node{}\n\t}\n\tif value.D(n).Start == value.D(n).End {\n\t\te.Node", "\tif n == nil {\n\t\treturn\n\t}\n\tif value.D(n).Start == value.D(n).End {\n\t\te.Node"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"empty root not copied", []printedEdit{{"\t\te.Node = p.MoveEmpty(n, p.baseShift)\n\t} else if", "\t\tp.MoveEmpty(n, p.baseShift)\n\t} else if"}}, "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"root moved twice", []printedEdit{{"} else if value.D(n).Gen < p.Gen {\n\t\tp.MoveTree(n, p.ShiftOf(n))\n\t}\n\tp.ClearMovementSpend()\n}\n\n// ShiftOf", "} else {\n\t\tp.MoveTree(n, p.ShiftOf(n))\n\t}\n\tp.ClearMovementSpend()\n}\n\n// ShiftOf"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"no clear after entry", []printedEdit{{"\t\tp.MoveTree(n, p.ShiftOf(n))\n\t}\n\tp.ClearMovementSpend()\n}\n\n// ShiftOf", "\t\tp.MoveTree(n, p.ShiftOf(n))\n\t}\n}\n\n// ShiftOf"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"shiftof base lt", []printedEdit{{"if value.D(n).Gen <= p.moveBase {", "if value.D(n).Gen < p.moveBase {"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"shiftof never base", []printedEdit{{"if value.D(n).Gen <= p.moveBase {", "if false {"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"shiftof always base", []printedEdit{{"if value.D(n).Gen <= p.moveBase {", "if true {"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"shiftof no cache", []printedEdit{{"\tif d, ok := p.Shifts[value.D(n).Gen]; ok {\n\t\treturn d\n\t}\n", ""}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"shiftof no charge", []printedEdit{{"\t\tp.Spend()\n\t\tif s >= ed.End {", "\t\tif s >= ed.End {"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"shiftof gt", []printedEdit{{"if s >= ed.End {", "if s > ed.End {"}}, "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"shiftof no store", []printedEdit{{"\tp.Shifts[value.D(n).Gen] = s - int(value.D(n).Start)\n", ""}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"tree no charge", []printedEdit{{"func (p *Parser) MoveTree(n *value.Node, shift int) {\n\tp.Spend()\n", "func (p *Parser) MoveTree(n *value.Node, shift int) {\n"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"tree start", []printedEdit{{"\tvalue.D(n).Start += int32(shift)\n\tvalue.D(n).End += int32(shift)\n\tvalue.D(n).Gen = p.Gen", "\tvalue.D(n).End += int32(shift)\n\tvalue.D(n).Gen = p.Gen"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"tree end", []printedEdit{{"\tvalue.D(n).End += int32(shift)\n\tvalue.D(n).Gen = p.Gen", "\tvalue.D(n).Gen = p.Gen"}}, "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"tree gen", []printedEdit{{"\tvalue.D(n).Gen = p.Gen\n", ""}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"tree child no charge", []printedEdit{{"range value.D(n).Children {\n\t\tp.Spend()\n\t\tif c != nil", "range value.D(n).Children {\n\t\tif c != nil"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"tree child ge", []printedEdit{{"if c != nil && value.D(c).Gen < p.Gen {\n\t\t\tvalue.D(n).Children[i]", "if c != nil {\n\t\t\tvalue.D(n).Children[i]"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"tree child not stored", []printedEdit{{"value.D(n).Children[i] = p.moveChild(c, gen, shift)", "_ = i\n\t\t\tp.moveChild(c, gen, shift)"}}, "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"tree field no charge", []printedEdit{{"range value.D(n).Fields {\n\t\tp.Spend()\n\t\tif c, ok", "range value.D(n).Fields {\n\t\tif c, ok"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"tree field ge", []printedEdit{{"ok && c != nil && value.D(c).Gen < p.Gen {", "ok && c != nil {"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"tree field not stored", []printedEdit{{"value.D(n).Fields[i].Value = p.moveChild(c, gen, shift)", "_ = i\n\t\t\tp.moveChild(c, gen, shift)"}}, "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"child no generation", []printedEdit{{"\t\tif value.D(c).Gen != gen {\n\t\t\tshift = p.ShiftOf(c)\n\t\t}\n", ""}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"child always generation", []printedEdit{{"if value.D(c).Gen != gen {", "if true {"}}, "^TestBudgetDocumentMovementChargesEdges$", []string{"."}},
		{"child empty unmoved", []printedEdit{{"\tif shift == 0 {\n\t\treturn c\n\t}\n\treturn p.MoveEmpty(c, shift)", "\treturn c"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"child empty zero copies", []printedEdit{{"\tif shift == 0 {\n\t\treturn c\n\t}\n", ""}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"empty no charge", []printedEdit{{"func (p *Parser) MoveEmpty(n *value.Node, shift int) *value.Node {\n\tp.Spend()\n", "func (p *Parser) MoveEmpty(n *value.Node, shift int) *value.Node {\n"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"empty no sharing", []printedEdit{{"\tif c, ok := p.Moved[n]; ok {\n\t\treturn c\n\t}\n", ""}}, "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"empty start", []printedEdit{{"\tvalue.D(c).Start += int32(shift)\n\tvalue.D(c).End += int32(shift)\n\tp.Moved", "\tvalue.D(c).End += int32(shift)\n\tp.Moved"}}, "^TestRunOperationsChargeTheirDeclaredWork$", []string{"./core"}},
		{"empty end", []printedEdit{{"\tvalue.D(c).End += int32(shift)\n\tp.Moved", "\tp.Moved"}}, "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"empty not recorded", []printedEdit{{"\tp.Moved[n] = c\n", ""}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"empty children alias", []printedEdit{{"\t\tvalue.D(c).Children = p.Nodes(len(value.D(n).Children))\n", "\t\tvalue.D(c).Children = value.D(n).Children\n"}}, "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"empty child no charge", []printedEdit{{"range value.D(n).Children {\n\t\t\tp.Spend()\n\t\t\tvalue.D(c).Children[i]", "range value.D(n).Children {\n\t\t\tvalue.D(c).Children[i]"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"empty field no charge", []printedEdit{{"range value.D(n).Fields {\n\t\t\tp.Spend()\n\t\t\tif vn", "range value.D(n).Fields {\n\t\t\tif vn"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"empty field not moved", []printedEdit{{"\t\t\t\tf.Value = p.moveDescendant(vn, value.D(n).Gen, shift)\n", "\t\t\t\t_ = vn\n"}}, "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"descendant nil", []printedEdit{{"\tcase c == nil:\n\t\treturn nil\n", ""}}, "^TestDocumentLeftRecursionMemberReparse$", []string{"./document"}},
		{"descendant empty kept", []printedEdit{{"\t\treturn p.MoveEmpty(c, shift)\n\tcase value.D(c).Gen < p.Gen:", "\t\treturn c\n\tcase value.D(c).Gen < p.Gen:"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"descendant unmoved", []printedEdit{{"\t\treturn p.moveChild(c, gen, shift)\n\t}\n\treturn c", "\t\treturn c\n\t}\n\treturn c"}}, "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"descendant always moved", []printedEdit{{"case value.D(c).Gen < p.Gen:\n\t\treturn p.moveChild(c, gen, shift)", "case true:\n\t\treturn p.moveChild(c, gen, shift)"}}, "^TestBudgetDocumentShiftReplayAndSharedEmptyGraph$", []string{"."}},
		{"value nil", []printedEdit{{"\tif v == nil {\n\t\treturn nil\n\t}\n\tif p.Moved", "\tif p.Moved"}}, "^TestDocumentResumeEdges$", []string{"."}},
		{"value base", []printedEdit{{"p.moveBase, p.baseShift = base, shift", "p.moveBase, p.baseShift = 0, shift"}}, "^TestRunOperationsChargeTheirDeclaredWork$", []string{"./core"}},
		{"value shift", []printedEdit{{"p.moveBase, p.baseShift = base, shift", "p.moveBase, p.baseShift = base, 0"}}, "^TestDocumentMovesReusedTrees$", []string{"."}},
		{"value empty kept", []printedEdit{{"\t\tv = p.MoveEmpty(v, shift)\n", "\t\tp.MoveEmpty(v, shift)\n"}}, "^TestRunOperationsChargeTheirDeclaredWork$", []string{"./core"}},
		{"value gen", []printedEdit{{"} else if value.D(v).Gen < p.Gen {", "} else {"}}, "^TestRunOperationsChargeTheirDeclaredWork$", []string{"./core"}},
		{"value no clear", []printedEdit{{"\t\tp.MoveTree(v, p.ShiftOf(v))\n\t}\n\tp.ClearMovementSpend()\n\treturn v", "\t\tp.MoveTree(v, p.ShiftOf(v))\n\t}\n\treturn v"}}, "^TestRunOperationsChargeTheirDeclaredWork$", []string{"./core"}},
	})
	runPrintedMutants(t, "core/budget_document_checkpoints.go", []printedMutant{
		{"clear moved not charged", []printedEdit{{"\t\t\tp.Spend()\n\t\t\tdelete(p.Moved, key)", "\t\t\tdelete(p.Moved, key)"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"clear shifts not charged", []printedEdit{{"\t\tp.Spend()\n\t\tdelete(p.Shifts, key)", "\t\tdelete(p.Shifts, key)"}}, "^TestDocMoveChargesAtItsBound$", []string{"./core"}},
		{"search no charge", []printedEdit{{"\t\tp.Spend()\n\t\tmid := int", "\t\tmid := int"}}, "^TestRunOperationsChargeTheirDeclaredWork$", []string{"./core"}},
		{"plain clear shifts gone", []printedEdit{{"\t\tclear(p.Shifts)\n\t\treturn", "\t\treturn"}}, "^TestClearMovementSpend$", []string{"./core"}},
		{"plain clear moved gone", []printedEdit{{"\t\t} else {\n\t\t\tclear(p.Moved)\n\t\t}\n", "\t\t}\n"}}, "^TestClearMovementSpend$", []string{"./core"}},
		{"plain moved threshold low", []printedEdit{{"p.Control == nil {\n\t\tif len(p.Moved) > 1024 {", "p.Control == nil {\n\t\tif len(p.Moved) > 0 {"}}, "^TestClearMovementSpend$", []string{"./core"}},
		{"plain moved threshold high", []printedEdit{{"p.Control == nil {\n\t\tif len(p.Moved) > 1024 {", "p.Control == nil {\n\t\tif len(p.Moved) > 1025 {"}}, "^TestClearMovementSpend$", []string{"./core"}},
		{"budget moved threshold low", []printedEdit{{"\tif len(p.Moved) > 1024 {\n\t\tp.Moved = nil\n\t} else {\n\t\tfor key", "\tif len(p.Moved) > 8 {\n\t\tp.Moved = nil\n\t} else {\n\t\tfor key"}}, "^TestClearMovementSpend$", []string{"./core"}},
		{"budget moved threshold high", []printedEdit{{"\tif len(p.Moved) > 1024 {\n\t\tp.Moved = nil\n\t} else {\n\t\tfor key", "\tif len(p.Moved) > 1025 {\n\t\tp.Moved = nil\n\t} else {\n\t\tfor key"}}, "^TestClearMovementSpend$", []string{"./core"}},
		{"plain move keeps shifts", []printedEdit{{"\t\tclear(p.Shifts)\n\t\treturn", "\t\treturn"}}, "^TestDocumentMoveWithoutControl$", []string{"./core"}},
		{"plain move keeps a large map", []printedEdit{{"p.Control == nil {\n\t\tif len(p.Moved) > 1024 {", "p.Control == nil {\n\t\tif len(p.Moved) > 1 << 20 {"}}, "^TestDocumentMoveWithoutControl$", []string{"./core"}},
		{"scan no charge", []printedEdit{{"\t\tp.Spend()\n\t\tif el.From < end {", "\t\tif el.From < end {"}}, "^TestRunOperationsChargeTheirDeclaredWork$", []string{"./core"}},
	})
	// The model of a lazy memo and the declared cost of memo.lookup and memo.store, which TestMemoChargesMatchTheModel
	// holds to what the engine charges.
	runPrintedMutants(t, "../ir/exec/ref/ref.go", []printedMutant{
		{"ref pending inflated", []printedEdit{{"return m.edits - e.applied\n", "return m.edits - e.applied + 3\n"}}, "^TestMemoChargesMatchTheModel$", []string{"./core"}},
		{"ref held pending inflated", []printedEdit{{"return m.edits - applied\n", "return m.edits - applied + 3\n"}}, "^TestMemoChargesMatchTheModel$", []string{"./core"}},
		{"ref hold start inclusive", []printedEdit{{"case k.pos < start:", "case k.pos <= start:"}}, "^TestMemoChargesMatchTheModel$", []string{"./core"}},
		{"ref hold end inclusive", []printedEdit{{"case k.pos > end:", "case k.pos >= end:"}}, "^TestMemoChargesMatchTheModel$", []string{"./core"}},
		{"ref held survive store", []printedEdit{{"\tdelete(m.held, e.key)\n", ""}}, "^TestMemoChargesMatchTheModel$", []string{"./core"}},
		{"ref lookup keeps applied", []printedEdit{{"\t\tif e != nil {\n\t\t\te.applied = m.edits\n\t\t}\n", ""}}, "^TestMemoChargesMatchTheModel$", []string{"./core"}},
	})
	runPrintedMutants(t, "core/resume.go", []printedMutant{
		{"prefix no charge", []printedEdit{{"func (p *Parser) NextPrefix(r *RunState) *RunElem {\n\tp.Spend()\n", "func (p *Parser) NextPrefix(r *RunState) *RunElem {\n"}}, "^TestRunOperationsChargeTheirDeclaredWork$", []string{"./core"}},
		{"tail no charge", []printedEdit{{"func (p *Parser) NextTail(r *RunState) *RunElem {\n\tp.Spend()\n", "func (p *Parser) NextTail(r *RunState) *RunElem {\n"}}, "^TestRunOperationsChargeTheirDeclaredWork$", []string{"./core"}},
		{"mid no growth charge", []printedEdit{{"\t\t\tp.runListGrowth(len(r.Mid))\n", ""}}, "^TestRunOperationsChargeTheirDeclaredWork$", []string{"./core"}},
		{"elem tail merge skipped", []printedEdit{{"\t\tp.MergeExpected(el.far, el.exp)\n", ""}}, "^TestRunOperationsChargeTheirDeclaredWork$", []string{"./core"}},
		{"end merge skipped", []printedEdit{{"\tp.MergeExpected(far, inner)\n", ""}}, "^TestDocumentBatchFreshParity$", []string{"."}},
	})
	runPrintedMutants(t, "core/call_exec.go", []printedMutant{
		{"lookup no advance", []printedEdit{{"if found && e.Vgen != p.Gen && !p.AdvanceEntry(e, p.Edits[:p.Gen]) {", "if false {"}}, "^TestBudgetDocumentReuseCancellationAndFreshRetry$", []string{"./document"}},
		{"reuse always moves", []printedEdit{{"\t\tif e.shifted {\n", "\t\tif true {\n"}}, "^TestBudgetDiagnosticRecoveredLists$", []string{"./vm"}},
		{"reuse never moves", []printedEdit{{"\t\tif e.shifted {\n", "\t\tif false {\n"}}, "^TestBudgetDocumentStaleMemoCallReplayIsBounded$", []string{"."}},
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
	runPrintedMutantsOver(t, path, nil, mutants)
}

// runPrintedMutantsOver is runPrintedMutants with the replacements of over, by absolute path, in
// every overlay, for a mutant that takes edits of two files.
func runPrintedMutantsOver(t *testing.T, path string, over map[string]string, mutants []printedMutant) {
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
			replace := map[string]string{file: mutant}
			maps.Copy(replace, over)
			overlay, err := json.Marshal(map[string]map[string]string{"Replace": replace})
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
