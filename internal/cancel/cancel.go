// Package cancel is the cancellation control of the static work on a grammar: compilation,
// analyses and classification. The work polls its control at every step; every PollStride steps
// the poll checks the context, and a cancelled context unwinds the work with a Cancellation panic,
// which the entry point that started the work recovers into the context's error. The unwind avoids
// threading errors through recursive construction. A nil control never cancels.
package cancel

import (
	"context"

	"zeroparse.dev/zp/grammar"
)

// Cancellation is the panic value that unwinds cancelled work with its error. Only the entry
// points that start the work catch it; unrelated panics keep their behavior. A control is local
// to one piece of work.
type Cancellation struct{ Err error }

// PollStride is the number of steps between context checks. A step is a visit of one
// expression, type or rule element; the heaviest, type-value inference, takes a few
// microseconds, so the stride bounds the work done after a cancellation to well under a
// millisecond (see design 042, "Compiler poll stride").
const PollStride = 16

// Control is the cancellation control of one piece of work: Steps counts the steps taken, and Ctx
// is checked every PollStride steps. A nil control never cancels.
type Control struct {
	Ctx   context.Context
	Steps uint32
}

// Check unwinds with a Cancellation if the context has ended.
func (c *Control) Check() {
	if c != nil {
		if err := c.Ctx.Err(); err != nil {
			panic(Cancellation{err})
		}
	}
}

// Poll counts one step and checks the context every PollStride steps.
func (c *Control) Poll() {
	if c == nil {
		return
	}
	c.Steps++
	if c.Steps%PollStride == 0 {
		c.Check()
	}
}

// Rules returns the rule definitions of g, unwinding if the context ends while they are listed.
func (c *Control) Rules(g *grammar.Grammar) []*grammar.RuleDef {
	if c == nil {
		return g.Rules()
	}
	result, err := g.RulesContext(c.Ctx)
	if err != nil {
		panic(Cancellation{err})
	}
	return result
}

// Types returns the type declarations of g, unwinding if the context ends while they are listed.
func (c *Control) Types(g *grammar.Grammar) []grammar.TypeDecl {
	if c == nil {
		return g.Types()
	}
	result, err := g.TypesContext(c.Ctx)
	if err != nil {
		panic(Cancellation{err})
	}
	return result
}
