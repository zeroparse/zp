package ops

import "zeroparse.dev/zp/internal/ir/exec"

// streamWindow adds to m the window of a stream parse and its host operations: the input buffer,
// the memory of delivered elements, the delivery callbacks and cancellation at stream boundaries.
// Stream commitment composes them with memo.prune; what they do lies outside the parser VM's state,
// so each is checked against the charges of the target that implements it, not against a model.
func streamWindow(m *exec.Module, value exec.Type) {
	m.AddComponent(Win, `The window of a stream parse: the input read and kept, the callback that receives the
elements of the #stream repetition, the recovered errors delivered so far, and the memory that the
delivered elements held.`)
	stream := []string{Stream}
	host := func(o *exec.Op) {
		o.Host = true
		m.AddOp(o)
	}
	host(&exec.Op{Name: "stream.emits", Component: Win, Results: []exec.Type{exec.TBool}, Reads: []string{Win},
		Features: stream,
		Contract: `Reports whether the parse delivers the elements of a #stream repetition to a callback.`})
	host(&exec.Op{Name: "stream.start", Component: Win, Results: []exec.Type{exec.TInt}, Reads: []string{Win},
		Features: stream, Contract: `Returns the id of the start rule of the stream parse.`,
		Requires: `The parse delivers elements (stream.emits).`})
	host(&exec.Op{Name: "stream.check", Component: Bud, Reads: []string{Bud}, Writes: []string{Bud}, Features: stream,
		NoReturn: true,
		Contract: `Observes cancellation and the limits of the parse at a stream boundary, and ends the parse with
the interruption that applies.`})
	host(&exec.Op{Name: "stream.report", Component: Win, Reads: []string{Win}, Writes: []string{Win}, Features: stream,
		NoReturn: true,
		Contract: `Delivers the recovered errors committed since the last report to the diagnostic callback and the
retained tail. An error of the callback ends the parse with it, before the element is delivered.`})
	host(&exec.Op{Name: "stream.deliver", Component: Win, Params: []exec.Param{exec.P("v", value)},
		Reads: []string{Win}, Writes: []string{Win}, Features: stream, NoReturn: true,
		Contract: `Passes the element v, nil for a value-free repetition, to the callback; v is final from then on.
An error of the callback ends the parse unchanged.`,
		Requires: `The parse delivers elements.`})
	host(&exec.Op{Name: "stream.discard", Component: Win, Params: []exec.Param{exec.P("keep", exec.TInt)},
		Reads: []string{Win}, Writes: []string{Win}, Features: stream, NoReturn: true, Cost: exec.Cost{Terms: []exec.CostTerm{{Coef: 1, Quantity: QStreamScan}}},
		Contract: `Discards the input before keep, when at least half of the buffer can go; later reads before keep
are not supported. The line and column of the new base are kept.`})
	host(&exec.Op{Name: "stream.release", Component: Win, Reads: []string{Win}, Writes: []string{Win}, Features: stream,
		Contract: `Releases what the elements committed so far held: the trail, the capture frames and the node
chunks that no active call refers to, so that a delivered element is not kept alive through the parser.`,
		Requires: `Every call of the delivered elements has returned.`})
	host(&exec.Op{Name: "stream.dropRecords", Component: Win, Params: []exec.Param{exec.P("pos", exec.TInt)},
		Reads: []string{Win}, Writes: []string{Win}, Features: stream,
		Contract: `Discards what the parse keeps about the positions before pos besides the memo, which no later
call reads: the records of skips and of the profile, and what the trail's inactive capacity refers to.`})
	host(&exec.Op{Name: "stream.tail", Component: Win, Results: []exec.Type{exec.TBool}, Reads: []string{Win},
		Features: stream,
		Contract: `Reports whether the parse retains the tail of its recovered errors (a diagnostic limit is set).`})
}
