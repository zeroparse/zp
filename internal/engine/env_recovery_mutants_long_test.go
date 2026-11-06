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

// TestRecoveryMutantsFail is the negative control of the conformance of the engine's recovered
// errors: each mutant breaks one part of rec.since or stream.settleErrors, the operations with an
// implementation of their own; the others are printed forms that the conformance machine repeats.
func TestRecoveryMutantsFail(t *testing.T) {
	const conformance = "^TestEngineExecConformance$"
	const charges = "^TestSettlingChargesTheSlotsItClears$"
	core := []string{"./core"}
	t.Run("copy", func(t *testing.T) {
		runPrintedMutants(t, "core/budget_memory.go", []printedMutant{
			{"a copy shares the list",
				[]printedEdit{{"out := p.appendErrors(nil, src)\n", "out := src\n"}}, conformance, core},
			{"a copy leaves out the last error",
				[]printedEdit{{"out := p.appendErrors(nil, src)\n", "out := p.appendErrors(nil, src[:len(src)-1])\n"}}, conformance, core},
		})
	})
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
				[]printedEdit{{"\tp.Diagnosed = 0\n\tif BudgetControlsEnabled && p.Control != nil {\n\t\t// With a diagnostic",
					"\tif BudgetControlsEnabled && p.Control != nil {\n\t\t// With a diagnostic"}},
				"^TestStreamCommitForgetsTheDiagnosedCount$", []string{"."}},
			{"settling keeps the bytes of the errors",
				[]printedEdit{{"\t\tp.Control.errs = 0\n", "\t\t_ = 0\n"}}, "^TestMemoryLimitStream$", []string{"."}},
		})
	})
}
