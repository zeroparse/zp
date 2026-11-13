package state

import "zeroparse.dev/zp/core"

// The engine's implementations of the operations of the stream compositions printed in
// stream_exec.go.

// streamReport delivers the recovered errors committed since the last report (stream.report); an
// error of the diagnostic callback ends the parse with it, before the element is delivered.
func (p *Parser) streamReport() {
	if err := p.ReportRecovered(); err != nil {
		panic(Fatal{Err: err})
	}
}

// streamRelease releases what the committed elements held (stream.release): the trail, the capture
// frames that only finished calls referred to, and the chunks that would chain the delivered
// elements to the parser.
func (p *Parser) streamRelease() {
	p.Trail = p.Trail[:0]
	if StreamFrameScratch {
		p.ResetStreamFrames()
	}
	p.splitChunks()
}

// streamDropRecords discards what the parse keeps before pos besides the memo (stream.dropRecords).
// Completed calls may have shortened the trail and the stack of the backend before the commit, so
// their inactive capacity is cleared here, once a window has passed: a large element's references
// do not survive indefinitely in a small stream.
func (p *Parser) streamDropRecords(pos int) {
	if StreamUndoCleanup {
		ClearSpend(p, p.Trail[:cap(p.Trail)])
	}
	if StreamStackCleanup && p.Backend != nil {
		p.Backend.ClearInactive()
	}
	p.PruneSkipRecords(pos)
}

// streamSettleErrors forgets the recovered errors before the boundary (stream.settleErrors). The
// element boundary is irreversible, and its recovered errors have already been delivered and
// collected. Without tracing, callbacks own defensive copies and memo entries own separate slices,
// so a small cleared backing array can serve the next element. A trace callback may retain
// Recovered(), and large scratch is released. The active start-rule call saved an empty recovery
// prefix; speculative element marks are no longer active after emission.
func (p *Parser) streamSettleErrors() {
	if p.Tr == nil && cap(p.Recovered) <= streamRecoveryScratchMax {
		ClearSpend(p, p.Recovered[:cap(p.Recovered)])
		p.Recovered = p.Recovered[:0]
	} else {
		p.Recovered = nil
	}
	p.Diagnosed, p.diagnosedSkip, p.diagnosedList = 0, 0, nil
	if BudgetControlsEnabled && p.Control != nil {
		// With a diagnostic limit, the errors are now held by the tail, which counts those it
		// keeps; without one, Recovered keeps them and errs keeps counting them.
		p.Control.errs = 0
	}
}

// ReportRecovered runs only at an irreversible stream boundary. Earlier
// recoveries in a speculative branch may have been removed by backtracking.
func (p *Parser) ReportRecovered() error {
	if p.Diagnostic == nil && p.DiagnosticTail.Limit == 0 {
		return nil
	}
	if p.Diagnosed > len(p.Recovered) {
		p.Diagnosed = len(p.Recovered)
	}
	var errs []*core.SyntaxError
	for p.Diagnosed < len(p.Recovered) {
		if e := p.Recovered[p.Diagnosed]; e.list == nil {
			// An error, delivered without listing it.
			if e.err != nil {
				if err := p.reportRecovered(e.err); err != nil {
					// The error ends the parse.
					return err
				}
			}
			p.Diagnosed++
			continue
		}
		errs = p.Recovered[p.Diagnosed].AppendErrors(errs[:0])
		for _, original := range errs[p.partialDiagnosed():] {
			if err := p.reportRecovered(original); err != nil {
				return err
			}
		}
		p.Diagnosed, p.diagnosedSkip, p.diagnosedList = p.Diagnosed+1, 0, nil
	}
	return nil
}

// reportRecovered delivers the recovered error original to the diagnostic callback, as a copy that
// the callback owns, and to the diagnostic tail.
func (p *Parser) reportRecovered(original *core.SyntaxError) error {
	if BudgetControlsEnabled {
		p.Control.Check()
	}
	if p.Diagnostic != nil {
		e := *original
		e.Expected = append([]string(nil), e.Expected...)
		e.Messages = append([]string(nil), e.Messages...)
		p.EmitCall = true
		err := p.Diagnostic(&e)
		p.EmitCall = false
		if err != nil {
			return err
		}
	}
	if p.DiagnosticTail.Limit > 0 {
		p.DiagnosticTail.Add(original)
	}
	if BudgetControlsEnabled {
		p.Control.Check()
	}
	return nil
}
