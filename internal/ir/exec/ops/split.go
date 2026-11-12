package ops

import "zeroparse.dev/zp/internal/ir/exec"

// Suspended is the state component of the calls whose split protocol is suspended while their rule
// body runs.
const Suspended = "suspended"

// CallState is the name of the handle type of the state of a suspended call.
const CallState = "CallState"

// splitCall adds to m the operations of the split call protocol, which holds the state of a call
// between its begin and its end while a target that does not recurse runs the rule body, and the
// transitional operations of the missing-token mode, which only the engine has: the mode records the
// tokens it inserts with the recovered errors (Rec).
func splitCall(m *exec.Module, add func(*exec.Op), rule, values, ctx, mark, entry exec.Type) {
	m.AddComponent(Suspended, `The calls whose rule body runs between the begin and the end of the split call
protocol, each with the state that its begin saved for its end in a record that its caller holds.`)
	st := m.AddHandle(CallState, Suspended, "The record of a suspended call, which its caller holds.")
	stArg := exec.P("st", st)
	m.AddOp(&exec.Op{Name: "call.state", Component: Suspended, Results: []exec.Type{st}, Writes: []string{Suspended},
		Contract: `Returns a record for the state of a call that the caller holds until the call ends.`})
	m.AddOp(&exec.Op{Name: "call.save", Component: Suspended,
		Params: []exec.Param{stArg, exec.P("start", exec.TInt), exec.P("min", exec.TInt), exec.P("key", values), exec.P("c", ctx),
			exec.P("rec", exec.TInt), exec.P("m", mark)},
		Writes: []string{Suspended},
		Contract: `Saves in st the state that the end of the call needs: its position, its binding level, the
values of its key, its context state, the mark of its recovered errors and the record that its
isolation set aside.`})
	m.AddOp(&exec.Op{Name: "call.saveExamined", Component: Suspended,
		Params: []exec.Param{stArg, exec.P("high", exec.TInt), exec.P("low", exec.TInt)}, Writes: []string{Suspended},
		Features: []string{Document},
		Contract: `Saves in st the examined input of the enclosing call, which doc.enter returned.`})
	m.AddOp(&exec.Op{Name: "call.saveMissing", Component: Suspended,
		Params: []exec.Param{stArg, exec.P("mark", exec.TInt)}, Writes: []string{Suspended}, Features: []string{Missing},
		Contract: `Saves in st the mark of the missing-token record, which missing.mark returned.`})
	for _, a := range []struct {
		name     string
		t        exec.Type
		features []string
		doc      string
	}{
		{"call.start", exec.TInt, nil, "the position of the call"},
		{"call.min", exec.TInt, nil, "the binding level of the call"},
		{"call.key", values, nil, "the values of the key of the call"},
		{"call.ctx", ctx, nil, "the context state of the call"},
		{"call.rec", exec.TInt, nil, "the mark of the recovered errors of the call"},
		{"call.mark", mark, nil, "the record that the isolation of the call set aside"},
		{"call.high", exec.TInt, []string{Document}, "the end of the examined input of the enclosing call"},
		{"call.low", exec.TInt, []string{Document}, "the start of the examined input of the enclosing call"},
		{"call.missing", exec.TInt, []string{Missing}, "the mark of the missing-token record"},
	} {
		m.AddOp(&exec.Op{Name: a.name, Component: Suspended, Params: []exec.Param{stArg}, Results: []exec.Type{a.t},
			Reads: []string{Suspended}, Features: a.features,
			Contract: "Returns " + a.doc + " that st holds.",
			Requires: "st holds a call that call.save saved."})
	}

	miss := []string{Missing}
	m.AddOp(&exec.Op{Name: "missing.active", Component: Rec, Results: []exec.Type{exec.TBool},
		Reads: []string{Rec}, Features: miss,
		Contract: `Reports whether the parse runs in the missing-token mode.`})
	m.AddOp(&exec.Op{Name: "missing.recompute", Component: Rec, Params: []exec.Param{exec.P("r", rule)},
		Results: []exec.Type{exec.TBool}, Reads: []string{Rec, Rules}, Features: miss,
		Contract: `Reports whether a result of the rule is evaluated again instead of being reused from the memo:
the parse runs in the missing-token mode and the rule or a rule it calls can insert a missing token.`})
	m.AddOp(&exec.Op{Name: "missing.mark", Component: Rec, Results: []exec.Type{exec.TInt},
		Reads: []string{Rec}, Features: miss,
		Contract: `Returns a mark of the missing-token record, for missing.reset.`})
	m.AddOp(&exec.Op{Name: "missing.reset", Component: Rec, Params: []exec.Param{exec.P("mark", exec.TInt)},
		Writes: []string{Rec}, Features: miss, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QTokens}}},
		Contract: `Removes the tokens inserted after mark, in the missing-token mode; does nothing outside it.`,
		Requires: `mark is a result of missing.mark at or before every later mark that is still in use.`})
	m.AddOp(&exec.Op{Name: "missing.replay", Component: Rec, Params: []exec.Param{exec.P("e", entry)},
		Writes: []string{Rec}, Features: miss, NoReturn: true,
		Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QTokens}, {Coef: 1, Quantity: QErrs}}},
		Contract: `Records again the tokens that the call of entry e inserted and the errors it recovered, as
rec.add does for the errors outside the missing-token mode.`,
		Requires: `The parse runs in the missing-token mode.`})
}
