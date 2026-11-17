package exec

import "zeroparse.dev/zp/internal/ir/code"

// Suspended is the state component of the calls whose split protocol is suspended while their rule
// body runs.
const Suspended = "suspended"

// CallState is the name of the handle type of the state of a suspended call.
const CallState = "CallState"

// splitCall adds to m the operations of the split call protocol, which holds the state of a call
// between its begin and its end while a target that does not recurse runs the rule body, and the
// transitional operations of the missing-token mode, which only the engine has: the mode records the
// tokens it inserts with the recovered errors (Rec).
func splitCall(m *code.Module, add func(*code.Op), rule, values, ctx, mark, entry code.Type) {
	m.AddComponent(Suspended, `The calls whose rule body runs between the begin and the end of the split call
protocol, each with the state that its begin saved for its end in a record that its caller holds.`)
	st := m.AddHandle(CallState, Suspended, "The record of a suspended call, which its caller holds.")
	stArg := code.P("st", st)
	m.AddOp(&code.Op{Name: "call.state", Component: Suspended, Results: []code.Type{st}, Writes: []string{Suspended},
		Contract: `Returns a record for the state of a call that the caller holds until the call ends.`})
	m.AddOp(&code.Op{Name: "call.save", Component: Suspended,
		Params: []code.Param{stArg, code.P("start", code.TInt), code.P("min", code.TInt), code.P("key", values), code.P("c", ctx),
			code.P("rec", code.TInt), code.P("m", mark)},
		Writes: []string{Suspended},
		Contract: `Saves in st the state that the end of the call needs: its position, its binding level, the
values of its key, its context state, the mark of its recovered errors and the record that its
isolation set aside.`})
	m.AddOp(&code.Op{Name: "call.saveExamined", Component: Suspended,
		Params: []code.Param{stArg, code.P("examined", code.TInt), code.P("low", code.TInt)}, Writes: []string{Suspended},
		Features: []string{Document},
		Contract: `Saves in st the examined input of the enclosing call, which doc.enter returned.`})
	m.AddOp(&code.Op{Name: "call.saveMissing", Component: Suspended,
		Params: []code.Param{stArg, code.P("mark", code.TInt)}, Writes: []string{Suspended}, Features: []string{Missing},
		Contract: `Saves in st the mark of the missing-token record, which missing.mark returned.`})
	for _, a := range []struct {
		name     string
		t        code.Type
		features []string
		doc      string
	}{
		{"call.start", code.TInt, nil, "the position of the call"},
		{"call.min", code.TInt, nil, "the binding level of the call"},
		{"call.key", values, nil, "the values of the key of the call"},
		{"call.ctx", ctx, nil, "the context state of the call"},
		{"call.rec", code.TInt, nil, "the mark of the recovered errors of the call"},
		{"call.mark", mark, nil, "the record that the isolation of the call set aside"},
		{"call.examined", code.TInt, []string{Document}, "the end of the examined input of the enclosing call"},
		{"call.low", code.TInt, []string{Document}, "the start of the examined input of the enclosing call"},
		{"call.missing", code.TInt, []string{Missing}, "the mark of the missing-token record"},
	} {
		m.AddOp(&code.Op{Name: a.name, Component: Suspended, Params: []code.Param{stArg}, Results: []code.Type{a.t},
			Reads: []string{Suspended}, Features: a.features,
			Contract: "Returns " + a.doc + " that st holds.",
			Requires: "st holds a call that call.save saved."})
	}

	miss := []string{Missing}
	m.AddOp(&code.Op{Name: "missing.active", Component: Rec, Results: []code.Type{code.TBool},
		Reads: []string{Rec}, Features: miss,
		Contract: `Reports whether the parse runs in the missing-token mode.`})
	m.AddOp(&code.Op{Name: "missing.recompute", Component: Rec, Params: []code.Param{code.P("r", rule)},
		Results: []code.Type{code.TBool}, Reads: []string{Rec, Rules}, Features: miss,
		Contract: `Reports whether a result of the rule is evaluated again instead of being reused from the memo:
the parse runs in the missing-token mode and the rule or a rule it calls can insert a missing token.`})
	m.AddOp(&code.Op{Name: "missing.mark", Component: Rec, Results: []code.Type{code.TInt},
		Reads: []string{Rec}, Features: miss,
		Contract: `Returns a mark of the missing-token record, for missing.reset.`})
	m.AddOp(&code.Op{Name: "missing.reset", Component: Rec, Params: []code.Param{code.P("mark", code.TInt)},
		Writes: []string{Rec}, Features: miss, Cost: code.Cost{Terms: []code.CostTerm{{Coef: 1, Quantity: QTokens}}},
		Contract: `Removes the tokens inserted after mark, in the missing-token mode; does nothing outside it.`,
		Requires: `mark is a result of missing.mark at or before every later mark that is still in use.`})
	m.AddOp(&code.Op{Name: "missing.replay", Component: Rec, Params: []code.Param{code.P("e", entry)},
		Writes: []string{Rec}, Features: miss, NoReturn: true,
		Cost: code.Cost{Terms: []code.CostTerm{{Coef: 1, Quantity: QTokens}, {Coef: 1, Quantity: QErrs}}},
		Contract: `Records again the tokens that the call of entry e inserted and the errors it recovered, as
rec.add does for the errors outside the missing-token mode.`,
		Requires: `The parse runs in the missing-token mode.`})
	m.AddOp(&code.Op{Name: "missing.replayResult", Component: Rec, Params: []code.Param{code.P("s", code.THandle(Res))},
		Writes: []string{Rec}, Features: miss, NoReturn: true,
		Cost: code.Cost{Terms: []code.CostTerm{{Coef: 1, Quantity: QTokens}, {Coef: 1, Quantity: QErrs}}},
		Contract: `Records again the tokens that the call of the result s that a context kept inserted and the
errors it recovered, as missing.replay does for a memo entry.`,
		Requires: `The parse runs in the missing-token mode.`})
}
