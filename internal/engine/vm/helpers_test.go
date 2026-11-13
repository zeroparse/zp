package vm

import (
	"context"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/noderep"
	"zeroparse.dev/zp/internal/compile"
	ecore "zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/value"
)

// catchFatal runs f and returns the error of the Fatal panic it raises, or nil.
func catchFatal(t *testing.T, f func()) error {
	t.Helper()
	var got error
	func() {
		defer func() {
			if v := recover(); v != nil {
				fatal, ok := v.(ecore.Fatal)
				if !ok {
					t.Fatalf("panic = %T(%v), want fatal", v, v)
				}
				got = fatal.Err
			}
		}()
		f()
	}()
	return got
}

func catchStreamPanic(f func()) (val any) { defer func() { val = recover() }(); f(); return }

func stateBudgetParser(lim core.Limits) *ecore.Parser {
	prog := &ecore.SharedProgram{}
	ecore.InitSharedProgram(prog, &compile.Program{})
	return &ecore.Parser{Prog: prog, Input: ecore.Input{Control: ecore.NewBudgetControl(nil, lim)}}
}

func undoFixture(p *ecore.Parser) (*ecore.Frame, []*core.Node) {
	old0 := value.Ptr(value.Data{Text: "old-zero", Fields: noderep.Fields{{Name: "keep", Value: "zero"}}})
	first0 := value.Ptr(value.Data{Text: "first-zero"})
	latest0 := value.Ptr(value.Data{Text: "latest-zero"})
	old1 := value.Ptr(value.Data{Text: "old-one"})
	last1 := value.Ptr(value.Data{Text: "last-one"})
	f := &ecore.Frame{Vals: []*core.Node{latest0, old1}}
	p.Trail = []ecore.Undo{{F: f, Slot: 0, Old: old0}, {F: f, Slot: 0, Old: first0}, {F: f, Slot: 1, Old: old1}, {F: f, Slot: 1, Old: last1}}
	return f, []*core.Node{old0, first0, old1, last1, latest0}
}

func diagnosticParser(lim core.Limits) *ecore.Parser {
	return &ecore.Parser{Input: ecore.Input{Control: ecore.NewBudgetControl(nil, lim)}}
}

type pollingContext struct {
	context.Context
	done   chan struct{}
	checks int
	at     int
}

func newPollingContext(at int) *pollingContext {
	return &pollingContext{Context: context.Background(), done: make(chan struct{}), at: at}
}

func (c *pollingContext) Done() <-chan struct{} { return c.done }
func (c *pollingContext) Err() error {
	c.checks++
	if c.at > 0 && c.checks >= c.at {
		return context.Canceled
	}
	return nil
}
