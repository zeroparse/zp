//go:build zp_long

package engine_test

import "testing"

// TestEnvironmentMutantsFail is the negative control of the conformance of the engine's variable
// environment: each mutant breaks one part of env.bind, env.lookup or env.key.
func TestEnvironmentMutantsFail(t *testing.T) {
	const conformance = "^TestEngineExecConformance$"
	core := []string{"./core"}
	t.Run("bind", func(t *testing.T) {
		const equal = "\tif p.rawValueEqual(p.Env.Get(slot, p.envLevels), val) {\n\t\treturn\n\t}\n"
		const rebind = "\tfor l := 0; l <= p.envLevels; l++ {\n\t\tp.Spend()\n\t}\n\t// Saved"
		runPrintedMutants(t, "core/budget_memo.go", []printedMutant{
			{"an equal binding makes a new environment", []printedEdit{{equal, ""}}, conformance, core},
			{"a binding does not charge the lookup of the old value",
				[]printedEdit{{"func (p *Parser) bindControlled(slot int, val any) {\n\tp.Spend()\n", "func (p *Parser) bindControlled(slot int, val any) {\n"}},
				conformance, core},
			{"a rebind does not charge the nodes it copies", []printedEdit{{rebind, "\t// Saved"}}, conformance, core},
			{"a rebind charges one node too many",
				[]printedEdit{{rebind, "\tfor l := 0; l <= p.envLevels+1; l++ {\n\t\tp.Spend()\n\t}\n\t// Saved"}}, conformance, core},
			{"a rebind binds nothing", []printedEdit{{"\tp.Env = p.Env.Set(slot, p.envLevels, val)\n\tp.ChargeValueBytes", "\tp.ChargeValueBytes"}},
				conformance, core},
		})
	})
	t.Run("uncontrolled bind", func(t *testing.T) {
		runPrintedMutants(t, "core/runtime.go", []printedMutant{
			{"an equal binding makes a new environment",
				[]printedEdit{{"\tif p.Env.Get(slot, p.envLevels) == val {\n\t\treturn\n\t}\n", ""}}, conformance, core},
			{"a key keeps the initial values after a binding",
				[]printedEdit{{"if p.Env == p.initEnv && r.InitVals != nil &&", "if r.InitVals != nil &&"}}, conformance, core},
			{"a key leaves out its first variable",
				[]printedEdit{{"return p.EnvValues(r.VarSlots)", "return p.EnvValues(r.VarSlots[min(1, len(r.VarSlots)):])"}}, conformance, core},
		})
	})
}

// TestRecoveryMutantsFail is the negative control of the tests of the engine's recovered errors: each
// mutant breaks one part of stream.settleErrors, of the shared lists (rec.since and rec.add), of a
// stream's reports of them, of the growths that take them, or of their replay with missing tokens,
// and the tests named for it must fail.
func TestRecoveryMutantsFail(t *testing.T) {
	const conformance = "^TestEngineExecConformance$"
	const charges = "^TestSettlingChargesTheSlotsItClears$"
	core := []string{"./core"}
	t.Run("settle", func(t *testing.T) {
		const keep = "if p.Tr == nil && cap(p.Recovered) <= streamRecoveryScratchMax {"
		runPrintedMutants(t, "core/stream_ops.go", []printedMutant{
			{"settling keeps the errors",
				[]printedEdit{{"\t\tp.Recovered = p.Recovered[:0]\n\t} else {", "\t} else {"}}, conformance, core},
			{"settling clears a list beyond the scratch", []printedEdit{{keep, "if p.Tr == nil {"}}, conformance, core},
			{"settling releases a list of one slot too many",
				[]printedEdit{{keep, "if p.Tr == nil && cap(p.Recovered) < streamRecoveryScratchMax {"}}, charges, core},
			{"settling clears without charging",
				[]printedEdit{{"ClearSpend(p, p.Recovered[:cap(p.Recovered)])", "clear(p.Recovered[:cap(p.Recovered)])"}}, charges, core},
			{"settling leaves the errors counted as delivered",
				[]printedEdit{{"\tp.Diagnosed, p.diagnosedSkip, p.diagnosedList = 0, 0, nil\n\tif BudgetControlsEnabled && p.Control != nil {\n\t\t// With a diagnostic",
					"\tif BudgetControlsEnabled && p.Control != nil {\n\t\t// With a diagnostic"}},
				"^TestStreamCommitForgetsTheDiagnosedCount$", []string{"."}},
			{"settling keeps the bytes of the errors",
				[]printedEdit{{"\t\tp.Control.errs = 0\n", "\t\t_ = 0\n"}}, "^TestMemoryLimitStream$", []string{"."}},
		})
	})
	t.Run("lists", func(t *testing.T) {
		runPrintedMutants(t, "core/recovered.go", []printedMutant{
			{"since leaves the entries in place",
				[]printedEdit{{"\tp.Recovered = append(p.Recovered[:m], RecEntry{list: l})\n", ""}}, conformance, core},
			{"since copies the errors of the lists",
				[]printedEdit{
					{"entries: AppendSpend(p, make([]RecEntry, 0, len(after)), after)}", "entries: flatEntries(p, after)}"},
					{"// countRecErrors returns", "func flatEntries(p *Parser, after []RecEntry) []RecEntry {\n\tvar out []RecEntry\n\tfor _, e := range appendRecErrors(nil, after) {\n\t\tp.SpendValue(1)\n\t\tout = append(out, RecEntry{err: e})\n\t}\n\treturn out\n}\n\n// countRecErrors returns"},
				}, conformance, core},
			// The control of the bound on unrolling: wrapping the one entry after a mark changes no
			// error and no charge, and only the bound fails.
			{"since wraps the one entry after its mark",
				[]printedEdit{{"\tif len(p.Recovered)-m == 1 {\n\t\tp.SpendValue(1)\n\t\treturn p.Recovered[m]\n\t}\n", ""}}, conformance, core},
			{"a list forgets the errors that a stream delivered",
				[]printedEdit{{"p.Diagnosed, p.diagnosedSkip, p.diagnosedList = m, skip, l", "p.Diagnosed = m\n\t\t_ = skip"}},
				"^TestReportRecoveredOfLists$", core},
		})
		runPrintedMutants(t, "core/stream_ops.go", []printedMutant{
			{"a report delivers no errors of a list",
				[]printedEdit{{"errs = p.Recovered[p.Diagnosed].AppendErrors(errs[:0])",
					"errs = errs[:0]\n\t\tif e := p.Recovered[p.Diagnosed]; e.err != nil {\n\t\t\terrs = append(errs, e.err)\n\t\t}"}},
				"^TestStreamDiagnosticsOfSharedLists$", []string{"."}},
		})
		runPrintedMutants(t, "core/runtime.go", []printedMutant{
			{"a growth step keeps the errors of the step before",
				[]printedEdit{{"\t\tp.Recovered = p.Recovered[:g.Rec]\n", ""}}, "^TestMissingTokensLeftRecursion$", []string{"."}},
			{"a growth that grew keeps the errors of the step before",
				[]printedEdit{{"\tp.Pos = g.start\n\tp.Recovered = p.Recovered[:g.Rec]\n\treturn true", "\tp.Pos = g.start\n\treturn true"}},
				"^TestLeftRecursionDependencyMemoUnexamined$", []string{"."}},
		})
		runPrintedMutants(t, "core/missing_token.go", []printedMutant{
			{"a replay records the errors of the insertions it repeats",
				[]printedEdit{{"\tif duplicate == nil {\n\t\tp.recAdd(errs)", "\tif true {\n\t\tp.recAdd(errs)"}},
				"^TestMissingTokensLedgerSeedBudget$", []string{"."}},
		})
	})
}
