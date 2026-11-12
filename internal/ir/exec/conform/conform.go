// Package conform is the conformance harness of the parser VM (design 065). It runs conformance
// cases, Exec IR functions that call operations, in lock-step on a reference model
// (internal/ir/exec/ref) and on an implementation under test, and fails on the first call where
// they differ: in results, in which handle a result is, in whether the call ended the parse, or in
// the units the implementation charged beyond the operation's cost bound. Cases are handwritten
// (Cases) or randomized sequences that respect the preconditions (Random).
package conform

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// Impl is an implementation under test. Values returns the implementation's Values handle for the
// conformance identifier id; two identifiers are equal values when they are equal.
type Impl interface {
	exec.Machine
	Values(id int64) any
}

// Environment is an Impl that provides the variable environment.
type Environment interface {
	// Datum returns the implementation's value for ref.Value(id).
	Datum(id int64) any
	// Rule returns the implementation's handle of the rule with the index in ref.Config.RuleVars.
	Rule(index int64) any
}

// Recovery is an Impl that provides the recovered-error list. After each call of an operation that
// writes it, the harness compares the errors with the model's, and checks that listing them visits
// fewer than twice as many lists as there are errors, and that listing the k errors of a list that
// rec.since returns visits at most 2k - 1 (linear-cost.md, Lemma 14).
type Recovery interface {
	// Err returns a new syntax error of the implementation with the identifier id.
	Err(id int64) any
	// Recovered returns the identifiers of the recovered errors.
	Recovered() []int64
	// Unrolled returns the number of lists that listing the recovered errors visits, every shared one
	// counted where it occurs; an implementation that keeps the errors in one array visits one for
	// each error.
	Unrolled() int
	// List returns the number of lists that listing the errors of the list l visits, counted as
	// Unrolled counts them, and the number of its errors.
	List(l any) (visits, errs int)
}

// Metered is an Impl that reports the units of budget it has charged so far, so that the harness
// checks each call against the operation's cost bound.
type Metered interface {
	Used() uint64
}

// Suite is an implementation to check.
type Suite struct {
	// Name names the implementation in failures.
	Name string
	// New returns an implementation in the state at the start of a parse with the configuration.
	New func(ref.Config) Impl
	// Ops are the operations the implementation provides. Cases that call others are skipped.
	Ops []string
	// LazyEdits sets ref.Config.LazyEdits for the model of every case: the implementation is a memo
	// that applies edits to an entry when it finds it.
	LazyEdits bool
	// DocEmptyEnter sets ref.Config.DocEmptyEnter: the implementation's doc.enter starts the examined
	// range of a call as the empty range at its position.
	DocEmptyEnter bool
	// DocCallMemo sets ref.Config.DocCallMemo: the implementation's Document parse calls rules through
	// the call protocol of a parse from scratch.
	DocCallMemo bool
	// CallUnitBefore sets ref.Config.CallUnitBefore: the implementation's call protocol charges the unit
	// of a call before rule.invoke and rule.invokePlain run.
	CallUnitBefore bool
	// Exact are the operations whose charges must equal their cost bound, not stay within it: in a
	// metered implementation, the model's quantities state the work to the unit.
	Exact []string
}

// ValuesOp is the operation that conformance cases use to make a Values handle from an identifier.
// It is not an operation of the parser VM.
const ValuesOp = "test.values"

// ErrsOp is the operation that conformance cases use to make the recovered errors of an entry that
// has some. It is not an operation of the parser VM. An implementation sees them as RecoveredErrors
// in the arguments of memo.store, in place of nil for an entry with none.
const ErrsOp = "test.errs"

// recoveredErrors is the type of RecoveredErrors.
type recoveredErrors struct{ _ byte }

// RecoveredErrors is the value of the recovered errors that ErrsOp makes. All entries that have errors
// share it: the harness compares handles only by identity, and the errors themselves are not examined.
var RecoveredErrors = &recoveredErrors{}

// DatumOp is the operation that conformance cases use to make a value from an identifier
// (ref.Value). It is not an operation of the parser VM.
const DatumOp = "test.datum"

// RuleOp is the operation that conformance cases use to make the handle of a rule of
// ref.Config.RuleVars. It is not an operation of the parser VM.
const RuleOp = "test.rule"

// ErrOp is the operation that conformance cases use to make a syntax error from an identifier. It is
// not an operation of the parser VM.
const ErrOp = "test.err"

// Module returns the pilot declarations with ValuesOp, ready for cases.
func Module() *exec.Module {
	m := ops.Pilot()
	m.AddOp(&exec.Op{Name: ValuesOp, Component: ops.Env, Params: []exec.Param{exec.P("id", exec.TInt)},
		Results:  []exec.Type{exec.THandle(ops.Values)},
		Contract: "Returns the Values handle of a conformance identifier; equal identifiers are equal values."})
	m.AddOp(&exec.Op{Name: ErrsOp, Component: ops.Rec, Results: []exec.Type{exec.THandle(ops.Errs)},
		Contract: "Returns the recovered errors of an entry that recovered some, as RecoveredErrors."})
	m.AddOp(&exec.Op{Name: ErrOp, Component: ops.Rec, Params: []exec.Param{exec.P("id", exec.TInt)},
		Results:  []exec.Type{exec.THandle(ops.Err)},
		Contract: "Returns a new syntax error with a conformance identifier."})
	m.AddOp(&exec.Op{Name: DatumOp, Component: ops.Host, Params: []exec.Param{exec.P("id", exec.TInt)},
		Results:  []exec.Type{exec.THandle(ops.Datum)},
		Contract: "Returns the value of a conformance identifier."})
	m.AddOp(&exec.Op{Name: RuleOp, Component: ops.Rules, Params: []exec.Param{exec.P("index", exec.TInt)},
		Results:  []exec.Type{exec.THandle(ops.Rule)},
		Contract: "Returns the handle of a rule of the case's configuration."})
	return m
}

// Case is one conformance case: a function of a module made by Module, run with Config.
type Case struct {
	Module *exec.Module
	Func   *exec.Func
	Config ref.Config
}

// Failure is a difference between the model and the implementation.
type Failure struct {
	Impl, Case string
	// Step is the index of the failing operation call, Trace the calls up to and including it.
	Step  int
	Trace []string
	Msg   string
	// Listing is the case as Exec IR.
	Listing string
}

func (f *Failure) Error() string {
	return fmt.Sprintf("%s: case %s, call %d: %s\ncalls:\n  %s\ncase:\n%s", f.Impl, f.Case, f.Step, f.Msg,
		strings.Join(f.Trace, "\n  "), f.Listing)
}

// Run runs case c on suite s. It returns nil when the case passes or calls an operation the suite
// does not provide (skipped reports which), and a *Failure otherwise.
func Run(s Suite, c Case) (skipped bool, err error) {
	if !provides(s, c.Module, c.Func, map[string]bool{}) {
		return true, nil
	}
	cfg := c.Config
	cfg.LazyEdits = s.LazyEdits
	cfg.DocEmptyEnter = s.DocEmptyEnter
	cfg.DocCallMemo = s.DocCallMemo
	cfg.CallUnitBefore = s.CallUnitBefore
	p := &pair{suite: s, model: ref.New(cfg), impl: s.New(cfg),
		refOrd: ordinals{at: map[any]int{}}, implOrd: ordinals{at: map[any]int{}}, handles: map[any]*handle{}}
	fail := func(msg string) error {
		return &Failure{Impl: s.Name, Case: c.Func.Name, Step: len(p.trace), Trace: p.trace, Msg: msg,
			Listing: exec.FuncString(c.Func)}
	}
	defer func() {
		if x := recover(); x != nil {
			m, ok := x.(mismatch)
			if !ok {
				panic(x)
			}
			err = fail(string(m))
		}
	}()
	_, err = exec.Run(c.Module, c.Func, p)
	var ae *exec.AssertionError
	if errors.As(err, &ae) {
		return false, fail("assertion of the case failed on the model and the implementation: " + ae.Msg)
	}
	// An interruption agreed on by both sides ends the case.
	return false, nil
}

// provides reports whether every operation that f calls, directly or through functions, is in
// s.Ops.
func provides(s Suite, m *exec.Module, f *exec.Func, visited map[string]bool) bool {
	if visited[f.Name] {
		return true
	}
	visited[f.Name] = true
	ok := true
	walkCalls(f.Body, func(c *exec.Call) {
		switch {
		case c.Callee == ValuesOp, c.Callee == ErrsOp, c.Callee == ErrOp, c.Callee == DatumOp, c.Callee == RuleOp,
			c.Callee == ExpOp, c.Callee == PlaceholderOp, c.Callee == DocStatOp, c.Callee == BodyOp:
		case m.Op(c.Callee) != nil:
			ok = ok && slices.Contains(s.Ops, c.Callee)
		case m.Func(c.Callee) != nil:
			ok = ok && provides(s, m, m.Func(c.Callee), visited)
		}
	})
	return ok
}

func walkCalls(b *exec.Block, visit func(*exec.Call)) {
	exec.Walk(b, nil, func(x exec.Expr) {
		if c, ok := x.(*exec.Call); ok {
			visit(c)
		}
	})
}

// mismatch is the panic value of a difference found during a call.
type mismatch string

// handle is a handle as the interpreter sees it: the model's and the implementation's.
type handle struct{ ref, impl any }

func (h *handle) String() string { return fmt.Sprintf("h%p", h) }

// pair is the machine that runs each call on the model and the implementation.
type pair struct {
	suite Suite
	model *ref.Model
	impl  Impl
	trace []string
	// refOrd and implOrd number the handles of each side in the order they first appear; a result
	// handle must have the same number on both sides. A handle that dies (Model.Dead) loses its
	// number, so that an implementation may reuse it.
	refOrd, implOrd ordinals
	handles         map[any]*handle // by the model's handle
	// regs are the registers of the run, and refRegs and implRegs each side's (UseRegisters).
	regs, refRegs, implRegs map[string]any
}

// ordinals numbers handles in the order they first appear.
type ordinals struct {
	at   map[any]int
	next int
}

// outcome is the result of one side's call.
type outcome struct {
	res         []any
	interrupted string
}

func call(m exec.Machine, o *exec.Op, args []any) (out outcome) {
	defer func() {
		if x := recover(); x != nil {
			i, ok := x.(exec.Interrupt)
			if !ok {
				panic(x)
			}
			out.interrupted = i.Reason
		}
	}()
	return outcome{res: m.Call(o, args)}
}

// implCall is call on the implementation, where a panic other than an interruption is a failure of
// the implementation, not of the harness.
func implCall(m exec.Machine, o *exec.Op, args []any) outcome {
	defer func() {
		if x := recover(); x != nil {
			panic(mismatch(fmt.Sprintf("the implementation panicked: %v", x)))
		}
	}()
	return call(m, o, args)
}

// UseRegisters keeps the registers of the run, and hands the model and an implementation that reads
// them registers of their own, which Call sets from the run's before each call and compares after it.
func (p *pair) UseRegisters(regs map[string]any) {
	p.regs, p.refRegs, p.implRegs = regs, map[string]any{}, map[string]any{}
	p.model.UseRegisters(p.refRegs)
	if rm, ok := p.impl.(exec.RegisterMachine); ok {
		rm.UseRegisters(p.implRegs)
	}
}

// syncRegisters sets the registers of each side from the run's.
func (p *pair) syncRegisters() {
	for r, v := range p.regs {
		p.refRegs[r], p.implRegs[r] = v, v
	}
}

// writeRegisters compares the registers that the call of o wrote on each side, except those the model
// leaves unspecified, and gives the run the model's.
func (p *pair) writeRegisters(o *exec.Op, res []any) {
	_, reads := p.impl.(exec.RegisterMachine)
	unspecified := p.model.Unspecified(o, res)
	for _, r := range o.Writes {
		w, ok := p.refRegs[r]
		if !ok {
			continue
		}
		if g := p.implRegs[r]; reads && g != w && !slices.Contains(unspecified, r) {
			panic(mismatch(fmt.Sprintf("register %s: model %v, implementation %v", r, w, g)))
		}
		p.regs[r] = w
	}
}

func (p *pair) Call(o *exec.Op, args []any) []any {
	refArgs, implArgs := make([]any, len(args)), make([]any, len(args))
	shown := make([]string, len(args))
	for i, a := range args {
		if h, ok := a.(*handle); ok {
			refArgs[i], implArgs[i] = h.ref, h.impl
			shown[i] = fmt.Sprintf("#%d", p.refOrd.at[h.ref])
		} else {
			refArgs[i], implArgs[i] = a, a
			shown[i] = fmt.Sprint(a)
		}
	}
	line := fmt.Sprintf("%s(%s)", o.Name, strings.Join(shown, ", "))
	p.trace = append(p.trace, line)
	if o.Name == ValuesOp {
		id := args[0].(int64)
		return []any{p.wrap(p.model.Values(id), p.impl.Values(id))}
	}
	if o.Name == ErrsOp {
		return []any{p.wrap(RecoveredErrors, RecoveredErrors)}
	}
	if o.Name == ErrOp {
		id := args[0].(int64)
		return []any{p.wrap(&ref.Err{ID: id}, p.impl.(Recovery).Err(id))}
	}
	if o.Name == DatumOp {
		id := args[0].(int64)
		return []any{p.wrap(ref.Datum{ID: id}, p.impl.(Environment).Datum(id))}
	}
	if o.Name == RuleOp {
		i := args[0].(int64)
		return []any{p.wrap(ref.RuleRef{Index: i}, p.impl.(Environment).Rule(i))}
	}
	if o.Name == ExpOp {
		id := args[0].(int64)
		return []any{p.wrap(ref.ExpID{ID: id}, p.impl.(Expectations).Exp(id))}
	}
	if o.Name == PlaceholderOp {
		return []any{p.wrap(ref.Placeholder, p.impl.(Expectations).Placeholder())}
	}
	if o.Name == BodyOp {
		i := args[0].(int64)
		return []any{p.wrap(ref.BodyRef{Index: i}, p.impl.(Bodies).Body(i))}
	}
	if o.Name == DocStatOp {
		i := args[0].(int64)
		want, got := p.model.DocStat(i), p.impl.(Documents).DocStat(i)
		if want != got {
			panic(mismatch(fmt.Sprintf("statistic %d of the Document: model %d, implementation %d", i, want, got)))
		}
		return []any{got}
	}
	if !p.model.Allows(o, refArgs) {
		panic(mismatch("the case breaks the precondition of " + o.Name))
	}
	bound := int64(o.Cost.Const)
	for _, t := range o.Cost.Terms {
		bound += int64(t.Coef) * p.model.Quantity(t.Quantity, o, refArgs)
	}
	meter, metered := p.impl.(Metered)
	var before uint64
	if metered {
		before = meter.Used()
	}
	p.syncRegisters()
	want := call(p.model, o, refArgs)
	got := implCall(p.impl, o, implArgs)
	charges := metered && o.Name != "budget.charge"
	if charges && want.interrupted == "" && got.interrupted == "limit" {
		// The implementation's own charges may exhaust the budget inside the operation, provided
		// that charges within the bound could.
		if !p.model.Exceeds(uint64(bound)) {
			panic(mismatch(fmt.Sprintf("interrupted by the limit, but charges within the bound %s = %d would not exceed it", o.Cost, bound)))
		}
		p.trace[len(p.trace)-1] += " -> interrupted inside: limit"
		panic(exec.Interrupt{Reason: "limit"})
	}
	if want.interrupted != got.interrupted {
		panic(mismatch(fmt.Sprintf("interrupted: model %q, implementation %q", want.interrupted, got.interrupted)))
	}
	if metered && want.interrupted == "" {
		used := meter.Used() - before
		if used > uint64(bound) {
			panic(mismatch(fmt.Sprintf("charged %d units, more than the bound %s = %d", used, o.Cost, bound)))
		}
		if charges && used != uint64(bound) && slices.Contains(p.suite.Exact, o.Name) {
			panic(mismatch(fmt.Sprintf("charged %d units, not the exact cost %s = %d", used, o.Cost, bound)))
		}
		if charges && p.model.Absorb(used) {
			panic(mismatch(fmt.Sprintf("charged %d units past the limit without interrupting", used)))
		}
	}
	if want.interrupted != "" {
		p.trace[len(p.trace)-1] += " -> interrupted: " + want.interrupted
		panic(exec.Interrupt{Reason: want.interrupted})
	}
	if r, ok := p.impl.(Recovery); ok && o.Name == "rec.since" {
		if n, k := r.List(got.res[0]); k > 0 && n > 2*k-1 {
			panic(mismatch(fmt.Sprintf("listing the %d errors of the list visits %d lists, more than %d", k, n, 2*k-1)))
		}
	}
	if r, ok := p.impl.(Recovery); ok && slices.Contains(o.Writes, ops.Rec) {
		if w, g := p.model.Recovered(), r.Recovered(); !slices.Equal(w, g) {
			panic(mismatch(fmt.Sprintf("recovered errors: model %v, implementation %v", w, g)))
		}
		if n, k := r.Unrolled(), len(p.model.Recovered()); n >= 2*k && n > 0 {
			panic(mismatch(fmt.Sprintf("listing %d recovered errors visits %d lists, not fewer than %d", k, n, 2*k)))
		}
	}
	p.writeRegisters(o, want.res)
	if ob, ok := p.impl.(Observer); ok {
		for _, c := range o.Writes {
			if w := p.model.Observe(c); w != nil {
				if g := ob.Observe(c); !slices.Equal(w, g) {
					panic(mismatch(fmt.Sprintf("%s: model %q, implementation %q", c, w, g)))
				}
			}
		}
	}
	res := make([]any, len(want.res))
	shown = shown[:0]
	for i, w := range want.res {
		g := got.res[i]
		if o.Results[i].Kind == exec.KHandle {
			// A handle that is not valid (the entry of a failed lookup) is not compared.
			if found := i+1 < len(want.res) && o.Results[i+1] == exec.TBool && !want.res[i+1].(bool); found {
				res[i] = &handle{}
				shown = append(shown, "_")
				continue
			}
			rn, in := p.refOrd.of(w), p.implOrd.of(g)
			if rn != in {
				panic(mismatch(fmt.Sprintf("result %d is handle #%d on the model and #%d on the implementation", i, rn, in)))
			}
			res[i] = p.wrap(w, g)
			shown = append(shown, fmt.Sprintf("#%d", rn))
			continue
		}
		if w != g && !(mayResults[o.Name] && g == true) {
			panic(mismatch(fmt.Sprintf("result %d: model %v, implementation %v", i, w, g)))
		}
		// A filter's answer that the implementation may give in place of the model's goes on.
		res[i] = g
		shown = append(shown, fmt.Sprint(g))
	}
	if len(shown) > 0 {
		p.trace[len(p.trace)-1] += " -> " + strings.Join(shown, ", ")
	}
	if slices.Contains(o.Writes, ops.Memo) {
		p.forgetDead()
	}
	return res
}

// forgetDead drops the numbers of the handles that died.
func (p *pair) forgetDead() {
	for r, h := range p.handles {
		if p.model.Dead(r) {
			delete(p.refOrd.at, r)
			delete(p.implOrd.at, h.impl)
			delete(p.handles, r)
		}
	}
}

func (o *ordinals) of(h any) int {
	n, ok := o.at[h]
	if !ok {
		o.next++
		n = o.next
		o.at[h] = n
	}
	return n
}

func (p *pair) wrap(r, i any) *handle {
	p.refOrd.of(r)
	p.implOrd.of(i)
	h := p.handles[r]
	if h == nil {
		h = &handle{ref: r, impl: i}
		p.handles[r] = h
	}
	return h
}
