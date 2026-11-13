package execcore

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/pass"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The tests of this file run the compositions on a model of the operations: the reference model of
// the pilot operations (ref.Model) and a model of the transitional ones written from their
// contracts, with rules whose bodies the tests script. They check the contracts of the compositions
// (what a call leaves in the parser state, what it charges, which entries it reuses) without a
// generated parser, and where a generated parser cannot tell (it restores the position after a failed call itself).

// trule is a rule of the model: its flags and its scripted body, which runs at the position of the
// machine and returns the value and success of the body.
type trule struct {
	id                   int64
	memo, leader, defer_ bool
	body                 func(m *machine) (any, bool)
	runs                 int // the evaluations of the body
}

// term is a term of the model: what it evaluates to and what it leaves on the machine.
type term struct {
	value         any
	fails         bool
	kids, created int
}

// fnValue is the function of the model's terms.
const fnValue = "function"

// record is an expectation record of the model: the farthest position and the expectations there.
type record struct {
	far  int64
	exps []string
}

// machine models the operations of the compositions.
type machine struct {
	*ref.Model
	regs map[string]any
	rec  []string
	// cur is the expectation record; saved holds the records that isolations set aside.
	cur   record
	saved []record
	// lastKept is the set that exp.copy made last.
	lastKept []string
	// exps holds the expectations of each entry (memo.setExpected).
	exps                   map[*ref.Entry]record
	spends, steps, memoHit int
	// kids and created count what terms left (term.eval), binds the bindings they made (env.bind).
	kids, created, binds int
	// ctx is what the context operations answer (ops.Growth): the context state of every call.
	ctx ctxModel
	// events lists what the observation was told (obs.enter, obs.exit); high is the examined frontier
	// of the active call (doc.examinedBegin, doc.examinedEnd).
	events []string
	high   int64
	low    int64
	// docMemo tells whether a Document parse memoizes the rules (rule.docMemo).
	docMemo bool
	// docActive tells whether the parse reads a Document (doc.active); examined holds what each
	// entry records (memo.examine); reused counts the reused entries (doc.reused).
	docActive, obsActive bool
	examined             map[*ref.Entry]int64
	reused               int
	// last is the entry stored last (memo.store).
	last *ref.Entry
	// evaluated counts the bodies evaluated through an invocation (doc.evaluated); forgot lists the
	// frontiers of the records forgotten (memo.forgetDoc).
	evaluated int
	forgot    []int64
}

// ctxModel scripts the context of the calls: the seed that ctx.enter returns, whether the call is
// in a context, the result kept there that ctx.lookup returns, whether the memo entries are allowed
// and where ctx.pop sends the result (1 for the memo, 2 for the context, 0 for nowhere). It records
// the context operations the calls made.
type ctxModel struct {
	seed, held *result
	active     bool
	deny       bool
	store      int64
	ops        []string
	kept       []*result
	present    map[*ref.Entry]bool
}

// result is a result that the context of a call keeps, or the last result of a growth.
type result struct {
	v    any
	ok   bool
	end  int64
	errs []string
	far  int64
	exps []string
}

// growth is the state of a growth in the model: a growth in no context.
type growth struct {
	start, rec int64
	best       *result
	done       bool
}

// callState is the record of a suspended call in the model (call.state).
type callState struct {
	start, min, key, rec, high, low, missing any
	ctx, mark                                any
	saved                                    bool
}

// stateOf returns the record st of a suspended call, which call.save must have filled.
func stateOf(x any) *callState {
	st := x.(*callState)
	if !st.saved {
		panic(exec.Interrupt{Reason: "the state of a call is read before call.save"})
	}
	return st
}

func resultOf(x any) *result {
	r, _ := x.(*result)
	return r
}

func newMachine() *machine {
	return &machine{Model: ref.New(ref.Config{Rules: 4, MaxDepth: 100}), exps: map[*ref.Entry]record{},
		ctx: ctxModel{store: 1, present: map[*ref.Entry]bool{}}, examined: map[*ref.Entry]int64{}}
}

func (m *machine) UseRegisters(regs map[string]any) { m.regs = regs }

func (m *machine) pos() int64         { return m.regs[ops.Pos].(int64) }
func (m *machine) setPos(p int64)     { m.regs[ops.Pos] = p }
func (m *machine) silent() bool       { return m.regs[ops.Silent].(int64) > 0 }
func (m *machine) recover(e string)   { m.rec = append(m.rec, e) }
func (m *machine) expect(e string)    { m.merge(m.pos(), []string{e}) }
func (m *machine) advance(n int64)    { m.setPos(m.pos() + n) }
func (m *machine) record() []string   { return slices.Clone(m.rec) }
func (m *machine) farthest() int64    { return m.cur.far }
func (m *machine) expected() []string { return m.cur.exps }

// merge adds the expectations at far to the record, as the runtime's Expect does: outside lookaheads,
// at the farthest position or past it.
func (m *machine) merge(far int64, exps []string) {
	for _, e := range exps {
		if m.silent() || far < m.cur.far {
			continue
		}
		if far > m.cur.far {
			m.cur = record{far: far}
		}
		if !slices.Contains(m.cur.exps, e) {
			m.cur.exps = append(m.cur.exps, e)
		}
	}
}

func entryOf(x any) *ref.Entry {
	e, _ := x.(*ref.Entry)
	if e == nil {
		panic(exec.Interrupt{Reason: "the null entry is used"})
	}
	return e
}

func (m *machine) Call(o *exec.Op, args []any) []any {
	switch o.Name {
	case "env.bind":
		m.binds++
		return nil
	case "term.eval":
		tm := args[0].(*term)
		m.kids += tm.kids
		m.created += tm.created
		return []any{tm.value, !tm.fails}
	case "term.created":
		return []any{int64(m.created)}
	case "term.kids":
		return []any{int64(m.kids)}
	case "term.dropCreated":
		mark := int(args[0].(int64))
		m.spends += m.created - mark
		m.created = mark
		return nil
	case "term.dropKids":
		mark := int(args[0].(int64))
		m.spends += m.kids - mark
		m.kids = mark
		return nil
	case "term.isFalse":
		return []any{args[0] == false}
	case "term.isFunction":
		return []any{args[0] == fnValue}
	case "call.tracked":
		return []any{false}
	case "budget.charge":
		m.spends += int(args[0].(uint64))
		return nil
	case "call.first":
		r := args[0].(*trule)
		if !r.defer_ {
			return []any{false}
		}
		return m.Model.Call(&exec.Op{Name: "seen.first"}, []any{r.id, m.pos()})
	case "rule.id":
		return []any{args[0].(*trule).id}
	case "rule.memo":
		return []any{args[0].(*trule).memo}
	case "rule.leader":
		return []any{args[0].(*trule).leader}
	case "rule.plain", "rule.docPlain", "rule.docOnlyMemo":
		return []any{false}
	case "budget.freeBegin":
		return []any{uint64(0)}
	case "budget.freeEnd":
		return nil
	case "rule.invoke":
		r := args[0].(*trule)
		r.runs++
		v, ok := r.body(m)
		return []any{v, ok}
	case "env.key":
		return []any{int64(0)}
	case "rec.mark":
		return []any{int64(len(m.rec))}
	case "rec.truncate":
		m.rec = m.rec[:args[0].(int64)]
		return nil
	case "rec.since":
		return []any{slices.Clone(m.rec[args[0].(int64):])}
	case "rec.add":
		errs, _ := args[0].([]string)
		m.rec = append(m.rec, errs...)
		return nil
	case "rec.none":
		errs, _ := args[0].([]string)
		return []any{len(errs) == 0}
	case "exp.isolate":
		m.saved = append(m.saved, m.cur)
		m.cur = record{far: args[0].(int64)}
		return []any{len(m.saved)}
	case "exp.unisolate":
		if args[0].(int) != len(m.saved) {
			panic("exp.unisolate of a record that is not the last set aside")
		}
		ended := m.cur
		m.cur, m.saved = m.saved[len(m.saved)-1], m.saved[:len(m.saved)-1]
		return []any{ended.far, ended.exps}
	case "exp.setCount", "exp.keptCount":
		s, _ := args[0].([]string)
		return []any{int64(len(s))}
	case "exp.setAt", "exp.keptAt":
		return []any{args[0].([]string)[args[1].(int64)]}
	case "exp.empty":
		return []any{[]string(nil)}
	case "exp.last":
		return []any{m.lastKept}
	case "exp.copy":
		m.lastKept = slices.Clone(args[0].([]string))
		return []any{m.lastKept}
	// The operations of the record that expect and the merges run, in a parse that records.
	case "exp.farthest":
		return []any{m.cur.far}
	case "exp.recording", "exp.maybe":
		return []any{true}
	case "exp.reset":
		m.cur = record{far: args[0].(int64)}
		return nil
	case "exp.count":
		return []any{int64(len(m.cur.exps))}
	case "exp.at":
		return []any{m.cur.exps[args[0].(int64)]}
	case "exp.add":
		m.cur.exps = append(m.cur.exps, args[0].(string))
		return nil
	case "memo.far":
		return []any{m.exps[entryOf(args[0])].far}
	case "memo.expected":
		return []any{m.exps[entryOf(args[0])].exps}
	case "memo.setExpected":
		s, _ := args[2].([]string)
		m.exps[entryOf(args[0])] = record{far: args[1].(int64), exps: s}
		return nil
	case "obs.memoHit":
		m.memoHit++
		return nil
	case "obs.enter":
		m.events = append(m.events, fmt.Sprintf("enter %d at %d level %d", args[0].(*trule).id, args[1], args[2]))
		return nil
	case "obs.exit":
		m.events = append(m.events, fmt.Sprintf("exit %v at %d examined %d %v", args[0], args[1], args[2], args[3]))
		return nil
	case "doc.examinedBegin":
		h := m.high
		m.high = m.pos()
		return []any{h}
	case "doc.examinedEnd":
		x := m.high
		m.high = max(args[0].(int64), x)
		return []any{x, true}
	case "doc.enter":
		h, l := m.high, m.low
		m.high, m.low = args[0].(int64)+1, args[0].(int64)
		return []any{h, l}
	case "doc.leave":
		m.high, m.low = max(args[0].(int64), m.high), min(args[1].(int64), m.low)
		return nil
	case "doc.touchFrom":
		m.low = min(m.low, args[0].(int64))
		return nil
	case "doc.low":
		return []any{m.low}
	case "doc.move":
		return []any{true}
	case "rule.positional":
		return []any{false}
	case "memo.before":
		return []any{false}
	case "memo.pin":
		return nil
	case "rule.docMemo":
		return []any{m.docMemo}
	case "obs.active":
		return []any{m.obsActive}
	case "doc.active":
		return []any{m.docActive}
	case "doc.touch":
		m.high = max(m.high, args[0].(int64))
		return nil
	case "doc.high":
		return []any{m.high}
	case "memo.examined":
		return []any{m.examined[entryOf(args[0])]}
	case "doc.reused":
		m.reused++
		return nil
	case "doc.evaluated":
		m.evaluated++
		return nil
	case "memo.forgetDoc":
		e := entryOf(args[0])
		m.forgot = append(m.forgot, m.examined[e])
		delete(m.examined, e)
		return nil
	case "rule.run":
		return args[0].(*trule).Call(m)
	case "memo.examine":
		m.examined[entryOf(args[0])] = args[2].(int64)
		return nil
	case "call.body":
		r := args[0].(*trule)
		r.runs++
		v, ok := r.body(m)
		return []any{v, ok}
	case "ctx.enter":
		m.ctx.ops = append(m.ctx.ops, "enter")
		return []any{&m.ctx, m.ctx.seed}
	case "ctx.found":
		return []any{resultOf(args[0]) != nil}
	case "ctx.ok":
		return []any{resultOf(args[0]).ok}
	case "ctx.value":
		return []any{resultOf(args[0]).v}
	case "ctx.replay":
		r := resultOf(args[0])
		m.memoHit++
		if !args[1].(bool) {
			m.merge(r.far, r.exps)
		}
		if r.ok {
			m.setPos(r.end)
			m.rec = append(m.rec, r.errs...)
		}
		return nil
	case "ctx.allows":
		return []any{!args[0].(*ctxModel).deny}
	case "ctx.active":
		return []any{args[0].(*ctxModel).active}
	case "ctx.lookup":
		m.ctx.ops = append(m.ctx.ops, "lookup")
		return []any{m.ctx.held}
	case "ctx.push", "ctx.done":
		m.ctx.ops = append(m.ctx.ops, strings.TrimPrefix(o.Name, "ctx."))
		return nil
	case "ctx.pop":
		m.ctx.ops = append(m.ctx.ops, "pop")
		return []any{m.ctx.store}
	case "ctx.storeMemo":
		return []any{int64(1)}
	case "ctx.storeCtx":
		return []any{int64(2)}
	case "memo.setPresent":
		m.ctx.present[entryOf(args[0])] = true
		return nil
	case "ctx.keep":
		errs, _ := args[9].([]string)
		k, _ := args[11].([]string)
		m.ctx.kept = append(m.ctx.kept, &result{v: args[8], ok: args[5].(bool), end: args[6].(int64), errs: errs, far: args[10].(int64), exps: k})
		return nil
	// The record of a suspended call, and the missing-token mode, which no call of the model is in.
	case "call.state":
		return []any{&callState{}}
	case "call.save":
		st := args[0].(*callState)
		st.start, st.min, st.key, st.ctx, st.rec, st.mark, st.saved = args[1], args[2], args[3], args[4], args[5], args[6], true
		return nil
	case "call.saveExamined":
		st := args[0].(*callState)
		st.high, st.low = args[1], args[2]
		return nil
	case "call.saveMissing":
		args[0].(*callState).missing = args[1]
		return nil
	case "call.start":
		return []any{stateOf(args[0]).start}
	case "call.min":
		return []any{stateOf(args[0]).min}
	case "call.key":
		return []any{stateOf(args[0]).key}
	case "call.ctx":
		return []any{stateOf(args[0]).ctx}
	case "call.rec":
		return []any{stateOf(args[0]).rec}
	case "call.mark":
		return []any{stateOf(args[0]).mark}
	case "call.high":
		return []any{stateOf(args[0]).high}
	case "call.low":
		return []any{stateOf(args[0]).low}
	case "call.missing":
		return []any{stateOf(args[0]).missing}
	case "missing.active", "missing.recompute":
		return []any{false}
	case "missing.mark":
		return []any{int64(len(m.rec))}
	case "missing.reset":
		return nil
	case "grow.begin":
		m.steps++
		return []any{&growth{start: m.pos(), rec: int64(len(m.rec)), best: &result{end: m.pos()}}}
	case "grow.next":
		return []any{!args[0].(*growth).done}
	case "grow.step":
		g := args[0].(*growth)
		if !args[2].(bool) || g.best.ok && m.pos() <= g.best.end {
			g.done = true
			return nil
		}
		m.steps++
		g.best = &result{v: args[1], ok: true, end: m.pos(), errs: slices.Clone(m.rec[g.rec:])}
		m.setPos(g.start)
		m.rec = m.rec[:g.rec]
		return nil
	case "grow.end":
		g := args[0].(*growth)
		m.setPos(g.best.end)
		m.rec = append(m.rec[:g.rec], g.best.errs...)
		return []any{g.best}
	}
	res := m.Model.Call(o, args)
	if o.Name == "memo.store" {
		m.last = res[0].(*ref.Entry)
	}
	return res
}

// composition returns the composition fn of the module m specialized for on.
func composition(t *testing.T, m *exec.Module, fn string, on ...string) *exec.Func {
	t.Helper()
	f, err := pass.Specialize(m, m.Func(fn), fn, on)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// callForms are the specializations of the rule calls that the tests of the call protocol run: the
// call of the generated runtimes and the engine's call over its split protocol.
var callForms = []struct {
	name, fn string
	on       []string
}{
	{"call", "call", []string{ops.Budget}},
	{"engine", "engineCall", []string{ops.Document, ops.Missing, ops.Budget}},
}

// forEachCall runs test for each of callForms with its specialization.
func forEachCall(t *testing.T, test func(t *testing.T, mod *exec.Module, f *exec.Func)) {
	for _, c := range callForms {
		t.Run(c.name, func(t *testing.T) {
			mod := Build()
			test(t, mod, composition(t, mod, c.fn, c.on...))
		})
	}
}

// callAt calls the rule r at the position pos with the composition call specialized for on, and
// returns its value and success.
func callAt(t *testing.T, mod *exec.Module, m *machine, f *exec.Func, r *trule, pos int64) (any, bool) {
	t.Helper()
	// Run starts with zero registers; the machine keeps the position and depth between calls.
	silent := int64(0)
	if m.regs != nil {
		silent = m.regs[ops.Silent].(int64)
	}
	res, err := exec.Run(mod, f, &start{machine: m, pos: pos, silent: silent}, r, int64(0))
	if err != nil {
		t.Fatal(err)
	}
	return res[0], res[1].(bool)
}

// start sets the registers of a run before the function runs.
type start struct {
	*machine
	pos, silent int64
}

func (s *start) UseRegisters(regs map[string]any) {
	regs[ops.Pos], regs[ops.Silent] = s.pos, s.silent
	s.machine.UseRegisters(regs)
}

// TestCallFailureRestoresState checks that a failed call, unmemoized and through the memo, leaves the
// position where it started and drops the errors its body recovered.
func TestCallFailureRestoresState(t *testing.T) {
	forEachCall(t, func(t *testing.T, mod *exec.Module, f *exec.Func) {
		m := newMachine()
		r := &trule{id: 1, memo: true, defer_: true, body: func(m *machine) (any, bool) {
			m.recover("skipped")
			m.advance(2)
			return nil, false
		}}
		for i, path := range []string{"unmemoized", "memoized", "reused"} {
			if _, ok := callAt(t, mod, m, f, r, 3); ok {
				t.Fatalf("%s call succeeded", path)
			}
			if m.pos() != 3 || len(m.rec) != 0 {
				t.Errorf("%s call: position %d, recovered %v; want 3 and none", path, m.pos(), m.rec)
			}
			if want := min(i+1, 2); r.runs != want {
				t.Errorf("%s call: %d evaluations of the body, want %d", path, r.runs, want)
			}
		}
		if m.spends != 3 {
			t.Errorf("%d charges for 3 calls", m.spends)
		}
	})
}

// TestUnmemoized checks that the unmemoized call of the tracked path evaluates the body each time,
// keeps what a successful body recovered, and restores the position and the recovered errors after
// a failed one.
func TestUnmemoized(t *testing.T) {
	mod := Build()
	f := composition(t, mod, "unmemoized")
	for _, ok := range []bool{true, false} {
		m := newMachine()
		m.rec = []string{"before"}
		r := &trule{id: 1, memo: true, body: func(m *machine) (any, bool) {
			m.recover("skipped")
			m.advance(2)
			return "v", ok
		}}
		for i := range 2 {
			v, got := callAt(t, mod, m, f, r, 3)
			want, wantRec := int64(5), []string{"before", "skipped"}
			if !ok {
				want, wantRec = 3, []string{"before"}
			}
			if got != ok || (ok && v != "v") || m.pos() != want || !slices.Equal(m.rec, wantRec) {
				t.Errorf("%v call %d: %v %v at %d, recovered %v; want %v at %d and %v", ok, i, v, got, m.pos(), m.rec, ok, want, wantRec)
			}
			m.rec = []string{"before"}
		}
		if r.runs != 2 || m.spends != 0 {
			t.Errorf("%v: %d evaluations and %d charges, want 2 and none", ok, r.runs, m.spends)
		}
	}
}

// TestCallSuccessKeepsErrors checks that a successful memoized call keeps the errors its body
// recovered and the expectations it recorded, and that the call that reuses its entry records them
// again and moves to its end.
func TestCallSuccessKeepsErrors(t *testing.T) {
	forEachCall(t, func(t *testing.T, mod *exec.Module, f *exec.Func) {
		m := newMachine()
		r := &trule{id: 1, memo: true, defer_: true, body: func(m *machine) (any, bool) {
			m.recover("skipped")
			m.advance(2)
			m.expect("x")
			return "v", true
		}}
		for i := range 3 {
			m.rec, m.cur = nil, record{}
			v, ok := callAt(t, mod, m, f, r, 3)
			if !ok || v != "v" || m.pos() != 5 || !slices.Equal(m.rec, []string{"skipped"}) {
				t.Errorf("call %d: %v %v at %d, recovered %v; want v true at 5 and one error", i, v, ok, m.pos(), m.rec)
			}
			if m.farthest() != 5 || !slices.Equal(m.expected(), []string{"x"}) {
				t.Errorf("call %d: the record holds %d %v, want 5 [x]", i, m.farthest(), m.expected())
			}
		}
		if r.runs != 2 {
			t.Errorf("%d evaluations of the body, want 2", r.runs)
		}
	})
}

// TestCallExpectationsFromStart checks that a memoized call records its expectations in a record
// that starts at its position: its entry keeps that position as the farthest one when the body
// recorded nothing past it, and the expectations it recorded otherwise.
func TestCallExpectationsFromStart(t *testing.T) {
	forEachCall(t, func(t *testing.T, mod *exec.Module, f *exec.Func) {
		for _, c := range []struct {
			name string
			exp  bool
		}{{"nothing recorded", false}, {"an expectation", true}} {
			t.Run(c.name, func(t *testing.T) {
				m := newMachine()
				r := &trule{id: 1, memo: true, body: func(m *machine) (any, bool) {
					if c.exp {
						m.advance(1)
						m.expect("x")
					}
					return nil, false
				}}
				callAt(t, mod, m, f, r, 4)
				res := m.Model.Call(&exec.Op{Name: "memo.lookup"}, []any{int64(1), int64(4), int64(0), int64(0)})
				e := res[0].(*ref.Entry)
				want := record{far: 4}
				if c.exp {
					want = record{far: 5, exps: []string{"x"}}
				}
				if got := m.exps[e]; got.far != want.far || !slices.Equal(got.exps, want.exps) {
					t.Errorf("the entry keeps %+v, want %+v", got, want)
				}
				// An empty set of expectations leaves the enclosing record as it is.
				outer := record{}
				if c.exp {
					outer = want
				}
				if m.farthest() != outer.far || !slices.Equal(m.expected(), outer.exps) {
					t.Errorf("the record holds %d %v, want %+v", m.farthest(), m.expected(), outer)
				}
			})
		}
	})
}

// TestCallReuseContexts checks which entries a call reuses: one computed in a lookahead only in
// one, and one computed outside a lookahead in both kinds of context, with or without recovered
// errors.
func TestCallReuseContexts(t *testing.T) {
	forEachCall(t, func(t *testing.T, mod *exec.Module, f *exec.Func) {
		for _, c := range []struct {
			name                string
			errs                bool
			storeSilent, silent int64
			reused              bool
		}{
			{"plain entry in a lookahead", false, 0, 1, true},
			{"silent entry outside", false, 1, 0, false},
			{"silent entry in a lookahead", false, 1, 1, true},
			{"entry with errors in a lookahead", true, 0, 1, true},
			{"silent entry with errors in a lookahead", true, 1, 1, true},
			{"silent entry with errors outside", true, 1, 0, false},
			{"entry with errors outside", true, 0, 0, true},
		} {
			t.Run(c.name, func(t *testing.T) {
				m := newMachine()
				r := &trule{id: 1, memo: true, body: func(m *machine) (any, bool) {
					if c.errs {
						m.recover("skipped")
					}
					m.advance(1)
					return "v", true
				}}
				m.regs = map[string]any{ops.Silent: c.storeSilent}
				callAt(t, mod, m, f, r, 0)
				m.regs[ops.Silent] = c.silent
				callAt(t, mod, m, f, r, 0)
				if reused := r.runs == 1; reused != c.reused {
					t.Errorf("reused %v, want %v", reused, c.reused)
				}
			})
		}
	})
}

// TestCallLookaheadStates checks that calls that alternate between a lookahead and outside one
// evaluate the rule once if the first is outside a lookahead and twice if it is inside, whether or
// not its result has recovered errors: the entry computed outside answers the calls in both
// states, and one computed inside only the calls inside one.
func TestCallLookaheadStates(t *testing.T) {
	forEachCall(t, func(t *testing.T, mod *exec.Module, f *exec.Func) {
		for _, c := range []struct {
			states string
			errs   bool
			runs   int
		}{
			{"OIOIOI", true, 1},
			{"IOIOIO", true, 2},
			{"OIOIOI", false, 1},
			{"IOIOIO", false, 2},
			{"IIII", true, 1},
			{"OOOO", true, 1},
		} {
			t.Run(fmt.Sprint(c.states, c.errs), func(t *testing.T) {
				m := newMachine()
				r := &trule{id: 1, memo: true, body: func(m *machine) (any, bool) {
					if c.errs {
						m.recover("skipped")
					}
					m.advance(1)
					return "v", true
				}}
				m.regs = map[string]any{}
				for _, s := range c.states {
					m.regs[ops.Silent] = map[rune]int64{'O': 0, 'I': 1}[s]
					if v, ok := callAt(t, mod, m, f, r, 0); !ok || v != "v" {
						t.Fatalf("call %c: %v %v", s, v, ok)
					}
				}
				if r.runs != c.runs {
					t.Errorf("%d evaluations, want %d", r.runs, c.runs)
				}
			})
		}
	})
}

// TestObserved checks that an observed call reports its beginning and its end to the observation,
// with its outcome, its end and how far its call examined the input, which joins the frontier of the
// enclosing call.
func TestObserved(t *testing.T) {
	mod := Build()
	f := composition(t, mod, "observed", ops.Document, ops.Observe)
	for _, ok := range []bool{true, false} {
		t.Run(fmt.Sprint(ok), func(t *testing.T) {
			m := newMachine()
			m.high = 12
			var at int64 = -1
			r := &trule{id: 2, body: func(m *machine) (any, bool) {
				at = m.high
				m.events = append(m.events, "body")
				m.advance(2)
				m.high = 9
				return "v", ok
			}}
			v, got := callAt(t, mod, m, f, r, 3)
			if got != ok || (ok && v != "v") || r.runs != 1 {
				t.Errorf("%v %v after %d evaluations, want the body's result after one", v, got, r.runs)
			}
			want := []string{"enter 2 at 3 level 0", "body", fmt.Sprintf("exit %v at 5 examined 9 true", ok)}
			if !slices.Equal(m.events, want) {
				t.Errorf("observed %q, want %q", m.events, want)
			}
			if at != 3 || m.high != 12 {
				t.Errorf("the body began with the frontier %d and the call left %d, want 3 and 12", at, m.high)
			}
		})
	}
}

// TestDocument checks that a Document call starts the frontier of the call past its position and
// leaves the enclosing call the farther frontier, when the call returns and when an operation in it
// interrupts the parse.
func TestDocument(t *testing.T) {
	mod := Build()
	f := composition(t, mod, "document", ops.Document, ops.Budget)
	checkDocument(t, mod, f)
	t.Run("typed", func(t *testing.T) {
		checkDocument(t, mod, composition(t, mod, "typedDocument", ops.Document, ops.Observe, ops.Typed, ops.Budget), nil)
	})
}

// TestDocumentCleanupMutant checks that TestDocument's checks fail for a document call whose
// cleanup runs before each return of its body but not when an operation interrupts the parse.
func TestDocumentCleanupMutant(t *testing.T) {
	mod := Build()
	f := composition(t, mod, "document", ops.Document, ops.Budget)
	stmts := f.Body.Stmts
	p := stmts[len(stmts)-1].(*exec.Protect)
	var unprotect func(b *exec.Block)
	unprotect = func(b *exec.Block) {
		var out []exec.Stmt
		for _, st := range b.Stmts {
			switch st := st.(type) {
			case *exec.Return:
				out = append(out, p.Finally.Stmts...)
			case *exec.If:
				unprotect(st.Then)
			}
			out = append(out, st)
		}
		b.Stmts = out
	}
	unprotect(p.Body)
	f.Body.Stmts = append(stmts[:len(stmts)-1:len(stmts)-1], p.Body.Stmts...)
	ft := &fakeT{}
	checkDocument(ft, mod, f)
	if !ft.failed {
		t.Error("the checks pass for a document call without its cleanup on an interruption")
	}
}

// fakeT records the failures of checks run on a mutant.
type fakeT struct {
	testing.TB
	failed bool
}

func (f *fakeT) Helper()                           {}
func (f *fakeT) Errorf(format string, args ...any) { f.failed = true }
func (f *fakeT) Fatal(args ...any)                 { f.failed = true; panic(f) }

func checkDocument(t testing.TB, mod *exec.Module, f *exec.Func, extra ...any) {
	t.Helper()
	for _, c := range []struct {
		name      string
		memo, ok  bool
		interrupt bool
		high      int64
		callHigh  int64
		wantHigh  int64
	}{
		{name: "memoized", memo: true, ok: true, high: 2, callHigh: 9, wantHigh: 9},
		{name: "unmemoized failure", ok: false, high: 12, callHigh: 9, wantHigh: 12},
		{name: "interrupted", memo: true, interrupt: true, high: 2, callHigh: 9, wantHigh: 9},
		{name: "interrupted unmemoized", interrupt: true, high: 12, callHigh: 9, wantHigh: 12},
	} {
		func() {
			defer func() {
				if x := recover(); x != nil && x != t {
					panic(x)
				}
			}()
			m := newMachine()
			m.high, m.docMemo, m.docActive = c.high, c.memo, true
			var atHigh int64 = -1
			r := &trule{id: 2, memo: true, body: func(m *machine) (any, bool) {
				atHigh = m.high
				m.advance(2)
				m.high = c.callHigh
				if c.interrupt {
					panic(exec.Interrupt{Reason: "limit"})
				}
				return "v", c.ok
			}}
			res, err := exec.Run(mod, f, &start{machine: m, pos: 3}, append([]any{r, int64(0)}, extra...)...)
			if c.interrupt {
				if _, ok := err.(exec.Interrupt); !ok {
					t.Errorf("%s: %v, %v, want an interruption", c.name, res, err)
				}
			} else if err != nil || res[1] != c.ok {
				t.Errorf("%s: %v, %v, want success %v", c.name, res, err, c.ok)
			}
			if atHigh != 4 {
				t.Errorf("%s: the call began with the frontier %d, want 4", c.name, atHigh)
			}
			if m.high != c.wantHigh {
				t.Errorf("%s: the call left the frontier %d, want %d", c.name, m.high, c.wantHigh)
			}
			if r.runs != 1 {
				t.Errorf("%s: evaluates the body %d times, want once", c.name, r.runs)
			}
		}()
	}
}

// TestTrackedPath checks the routing of a call on the tracked path: an observed parse reports the
// call to the observation, a Document parse takes the Document call, and otherwise the call is
// unmemoized while the rule's memoization is deferred and memoized after; each call is charged once.
func TestTrackedPath(t *testing.T) {
	mod := Build()
	f := composition(t, mod, "tracked", ops.Document, ops.Observe)
	for _, c := range []struct {
		observed, document bool
	}{{false, false}, {true, false}, {false, true}, {true, true}} {
		m := newMachine()
		m.obsActive, m.docActive, m.docMemo = c.observed, c.document, true
		r := &trule{id: 1, memo: true, defer_: true, body: func(m *machine) (any, bool) {
			m.advance(1)
			return "v", true
		}}
		var stored []bool
		for range 3 {
			m.last = nil
			if v, ok := callAt(t, mod, m, f, r, 2); !ok || v != "v" || m.pos() != 3 {
				t.Fatalf("%+v: %v %v at %d", c, v, ok, m.pos())
			}
			stored = append(stored, m.last != nil)
		}
		wantEvents := 0
		if c.observed {
			wantEvents = 6 // the enter and exit of each call
		}
		// The observed call evaluates the body itself (call.body); otherwise a deferred rule is
		// stored on its second call and reused on its third, and a Document memoizes on the first.
		wantStored, wantRuns, wantSpends := []bool{false, true, false}, 2, 3
		switch {
		case c.observed:
			wantStored, wantRuns, wantSpends = []bool{false, false, false}, 3, 0
		case c.document:
			wantStored, wantRuns = []bool{true, false, false}, 1
		}
		if len(m.events) != wantEvents || !slices.Equal(stored, wantStored) || r.runs != wantRuns || m.spends != wantSpends {
			t.Errorf("%+v: %d events, stored %v, %d evaluations and %d charges; want %d, %v, %d and %d",
				c, len(m.events), stored, r.runs, m.spends, wantEvents, wantStored, wantRuns, wantSpends)
		}
	}
}

// Call evaluates the rule's body, as an invocation that a generated call passes (rule.run).
func (r *trule) Call(m *machine) []any {
	r.runs++
	v, ok := r.body(m)
	return []any{v, ok}
}

// TestTypedDocument checks what the typed Document call adds: a body evaluated through the
// invocation that a generated call passes is counted, the record of an entry is kept beside it and
// read on a reuse, and a call that replaces an entry forgets the entry's record.
func TestTypedDocument(t *testing.T) {
	mod := Build()
	f := composition(t, mod, "typedDocument", ops.Document, ops.Observe, ops.Typed, ops.Budget)
	run := func(m *machine, r *trule, inv any, pos, silent int64) (any, bool) {
		t.Helper()
		res, err := exec.Run(mod, f, &start{machine: m, pos: pos, silent: silent}, r, int64(0), inv)
		if err != nil {
			t.Fatal(err)
		}
		return res[0], res[1].(bool)
	}
	// A rule that the Document does not memoize, invoked by the generated call or not.
	for _, viaInv := range []bool{true, false} {
		m := newMachine()
		r := &trule{id: 1, body: func(m *machine) (any, bool) {
			m.recover("x")
			m.advance(2)
			return nil, false
		}}
		var inv any
		if viaInv {
			inv = r
		}
		if _, ok := run(m, r, inv, 3, 0); ok || m.pos() != 3 || len(m.rec) != 0 || r.runs != 1 {
			t.Errorf("inv=%v: failure %v at %d, recovered %v after %d evaluations; want a failure at 3", viaInv, ok, m.pos(), m.rec, r.runs)
		}
		if want := map[bool]int{true: 1, false: 0}[viaInv]; m.evaluated != want {
			t.Errorf("inv=%v: %d counted evaluations, want %d", viaInv, m.evaluated, want)
		}
	}
	// A memoized rule: computed in a lookahead, its entry is not reused outside one, so the call
	// outside replaces it and forgets its record; a later call reuses the new entry and its record.
	m := newMachine()
	m.docMemo = true
	r := &trule{id: 1, memo: true, body: func(m *machine) (any, bool) {
		m.advance(2)
		m.high = max(m.high, 4+m.pos())
		return "v", true
	}}
	run(m, r, r, 3, 1)
	first := m.lastEntry(t)
	if m.examined[first] != 9 || m.evaluated != 1 {
		t.Fatalf("the entry records %d after %d counted evaluations, want 9 and 1", m.examined[first], m.evaluated)
	}
	m.high = 0
	run(m, r, nil, 3, 0)
	second := m.lastEntry(t)
	if second == first || !slices.Equal(m.forgot, []int64{9}) || m.examined[second] != 9 || r.runs != 2 {
		t.Errorf("replacing the entry forgot %v and records %d after %d evaluations; want [9], 9 and 2", m.forgot, m.examined[second], r.runs)
	}
	m.high = 0
	if v, ok := run(m, r, r, 3, 0); !ok || v != "v" || m.pos() != 5 || r.runs != 2 || m.reused != 1 || m.high != 9 {
		t.Errorf("the reuse gives %v %v at %d after %d evaluations, %d reuses and the frontier %d; want v true at 5, 2, 1 and 9",
			v, ok, m.pos(), r.runs, m.reused, m.high)
	}
	// A leader's grown entry is replaced in the same way.
	m = newMachine()
	m.docMemo = true
	n := 0
	leader := &trule{id: 2, memo: true, leader: true, body: func(m *machine) (any, bool) {
		if n%3 == 2 {
			n++
			return nil, false
		}
		m.advance(int64(n%3 + 1))
		n++
		return "v", true
	}}
	run(m, leader, nil, 3, 1)
	grown := m.lastEntry(t)
	run(m, leader, nil, 3, 0)
	if m.lastEntry(t) == grown || len(m.forgot) != 1 {
		t.Errorf("replacing the grown entry forgot %v, want its record", m.forgot)
	}
}

// TestMemoizedDocument checks the Document bookkeeping of a memoized call: the call that stores an
// entry records with it the frontier of the call, and the call that reuses it extends its own
// frontier to the entry's and counts the reuse. Outside a Document parse the call keeps no such
// record.
func TestMemoizedDocument(t *testing.T) {
	mod := Build()
	f := composition(t, mod, "memoized", ops.Document, ops.Observe, ops.Budget)
	for _, active := range []bool{true, false} {
		m := newMachine()
		m.docActive = active
		r := &trule{id: 1, memo: true, body: func(m *machine) (any, bool) {
			m.advance(2)
			m.high = max(m.high, 4) // the body examined the input up to 4, before the end of the call
			return "v", true
		}}
		m.high = 2
		if v, ok := callAt(t, mod, m, f, r, 3); !ok || v != "v" {
			t.Fatalf("active=%v: %v %v", active, v, ok)
		}
		e := m.lastEntry(t)
		if active && m.examined[e] != 6 {
			t.Errorf("the entry records the frontier %d, want 6 (past its end)", m.examined[e])
		}
		if !active && len(m.examined) != 0 {
			t.Errorf("outside a Document the entry records %v", m.examined)
		}
		// A later call at the position reuses the entry from a call that examined less.
		m.high = 3
		if v, ok := callAt(t, mod, m, f, r, 3); !ok || v != "v" || m.pos() != 5 || r.runs != 1 || m.memoHit != 1 {
			t.Fatalf("active=%v: reuse gives %v %v at %d after %d evaluations and %d hits", active, v, ok, m.pos(), r.runs, m.memoHit)
		}
		wantHigh, wantReused := int64(6), 1
		if !active {
			wantHigh, wantReused = 3, 0
		}
		if m.high != wantHigh || m.reused != wantReused {
			t.Errorf("active=%v: the reuse leaves the frontier %d and %d reuses, want %d and %d",
				active, m.high, m.reused, wantHigh, wantReused)
		}
	}
}

// lastEntry returns the entry that the last call stored or grew.
func (m *machine) lastEntry(t *testing.T) *ref.Entry {
	t.Helper()
	if m.last == nil {
		t.Fatal("no entry stored")
	}
	return m.last
}

// TestGrowth checks that a leader's call evaluates its body while the match gets longer, charging
// one growth step per evaluation, and ends at the longest match with its errors only.
func TestGrowth(t *testing.T) {
	forEachCall(t, func(t *testing.T, mod *exec.Module, f *exec.Func) {
		m := newMachine()
		lens := []int64{1, 3, 3}
		n := 0
		r := &trule{id: 2, memo: true, leader: true, body: func(m *machine) (any, bool) {
			if n >= len(lens) {
				return nil, false
			}
			m.recover(fmt.Sprint("e", n))
			m.advance(lens[n])
			n++
			return fmt.Sprint("v", n), true
		}}
		v, ok := callAt(t, mod, m, f, r, 2)
		if !ok || v != "v2" || m.pos() != 5 {
			t.Errorf("%v %v at %d, want v2 true at 5", v, ok, m.pos())
		}
		if !slices.Equal(m.rec, []string{"e1"}) {
			t.Errorf("recovered %v, want the errors of the longest match", m.rec)
		}
		if r.runs != 3 || m.steps != 3 {
			t.Errorf("%d evaluations and %d growth steps, want 3 and 3", r.runs, m.steps)
		}
		// The call stores the result of the growth in the memo, with its present set, and reuses it.
		e := m.lastEntry(t)
		if !m.ctx.present[e] {
			t.Error("the entry of the growth has no present set")
		}
		if v, ok := callAt(t, mod, m, f, r, 2); !ok || v != "v2" || m.pos() != 5 || r.runs != 3 {
			t.Errorf("the reuse gives %v %v at %d after %d evaluations, want v2 true at 5 after 3", v, ok, m.pos(), r.runs)
		}
	})
}

// TestCallContext checks how a memoized call uses the context of its call: it returns the seed of
// its rule growing at the position without evaluating or recording anything, records its result
// where the dependencies of its evaluation send it (the memo, the table of the context or nowhere),
// reuses a memo entry only if the context allows it, and reuses the result kept in the context.
func TestCallContext(t *testing.T) {
	forEachCall(t, func(t *testing.T, mod *exec.Module, f *exec.Func) {
		body := func(m *machine) (any, bool) {
			m.recover("skipped")
			m.advance(2)
			m.expect("x")
			return "v", true
		}
		t.Run("seed", func(t *testing.T) {
			m := newMachine()
			m.ctx.seed = &result{v: "s", ok: true, end: 7, far: 9, exps: []string{"y"}}
			r := &trule{id: 1, memo: true, leader: true, body: body}
			if v, ok := callAt(t, mod, m, f, r, 3); !ok || v != "s" || m.pos() != 7 || r.runs != 0 {
				t.Errorf("%v %v at %d after %d evaluations, want the seed at 7 without evaluation", v, ok, m.pos(), r.runs)
			}
			if m.farthest() != 0 || !slices.Equal(m.ctx.ops, []string{"enter"}) || m.last != nil {
				t.Errorf("the read of the seed recorded %d %v, made %v and stored %v", m.farthest(), m.expected(), m.ctx.ops, m.last)
			}
		})
		for _, c := range []struct {
			name   string
			store  int64
			stored bool
			kept   int
		}{{"memo", 1, true, 0}, {"context", 2, false, 1}, {"nowhere", 0, false, 0}} {
			t.Run(c.name, func(t *testing.T) {
				m := newMachine()
				m.ctx.active, m.ctx.store = true, c.store
				r := &trule{id: 1, memo: true, body: body}
				if v, ok := callAt(t, mod, m, f, r, 3); !ok || v != "v" || m.pos() != 5 {
					t.Fatalf("%v %v at %d", v, ok, m.pos())
				}
				if want := []string{"enter", "lookup", "push", "pop", "done"}; !slices.Equal(m.ctx.ops, want) {
					t.Errorf("context operations %v, want %v", m.ctx.ops, want)
				}
				if (m.last != nil) != c.stored || len(m.ctx.kept) != c.kept {
					t.Errorf("stored %v and kept %d results, want %v and %d", m.last != nil, len(m.ctx.kept), c.stored, c.kept)
				}
				if c.kept == 1 {
					k := m.ctx.kept[0]
					if k.v != "v" || !k.ok || k.end != 5 || !slices.Equal(k.errs, []string{"skipped"}) || k.far != 5 || !slices.Equal(k.exps, []string{"x"}) {
						t.Errorf("kept %+v", *k)
					}
				}
			})
		}
		t.Run("denied entry", func(t *testing.T) {
			m := newMachine()
			r := &trule{id: 1, memo: true, body: body}
			callAt(t, mod, m, f, r, 3)
			m.ctx.deny, m.rec, m.cur = true, nil, record{}
			callAt(t, mod, m, f, r, 3)
			if r.runs != 2 {
				t.Errorf("%d evaluations, want the entry not reused", r.runs)
			}
		})
		t.Run("kept result", func(t *testing.T) {
			m := newMachine()
			m.ctx.active = true
			m.ctx.held = &result{v: "k", ok: true, end: 6, errs: []string{"e"}, far: 6, exps: []string{"z"}}
			r := &trule{id: 1, memo: true, body: body}
			if v, ok := callAt(t, mod, m, f, r, 3); !ok || v != "k" || m.pos() != 6 || r.runs != 0 {
				t.Errorf("%v %v at %d after %d evaluations, want the kept result", v, ok, m.pos(), r.runs)
			}
			if !slices.Equal(m.rec, []string{"e"}) || m.farthest() != 6 || !slices.Equal(m.expected(), []string{"z"}) || m.memoHit != 1 {
				t.Errorf("the reuse recorded %v, %d %v and %d hits", m.rec, m.farthest(), m.expected(), m.memoHit)
			}
		})
	})
}

// TestEngineCallExamined checks the engine's bookkeeping of the examined input: a memoized call
// records with its entry the input its body examined, the enclosing call's examined input becomes
// the union of its own and the call's, and a reuse extends it to the entry's, in every parse.
func TestEngineCallExamined(t *testing.T) {
	mod := Build()
	f := composition(t, mod, "engineCall", ops.Document, ops.Missing, ops.Budget)
	m := newMachine()
	r := &trule{id: 1, memo: true, body: func(m *machine) (any, bool) {
		m.advance(2)
		m.high = max(m.high, 6)
		return "v", true
	}}
	m.high, m.low = 1, 0
	if v, ok := callAt(t, mod, m, f, r, 3); !ok || v != "v" {
		t.Fatalf("%v %v", v, ok)
	}
	e := m.lastEntry(t)
	if m.examined[e] != 6 || m.high != 6 || m.low != 0 {
		t.Errorf("entry examined %d, enclosing call [%d, %d); want 6 and [0, 6)", m.examined[e], m.low, m.high)
	}
	m.high = 1
	if v, ok := callAt(t, mod, m, f, r, 3); !ok || v != "v" || r.runs != 1 {
		t.Fatalf("the reuse gives %v %v after %d evaluations", v, ok, r.runs)
	}
	if m.high != 6 || m.reused != 1 {
		t.Errorf("the reuse leaves the frontier at %d with %d reuses, want 6 and 1", m.high, m.reused)
	}
}

// TestTerms runs predicate and bind over the outcomes of a term: what each answers, what it binds,
// what it leaves and what it charges for dropping the rest.
func TestTerms(t *testing.T) {
	cases := []struct {
		name             string
		t                term
		predicate, bind  bool
		binds            bool
		kidsLeft, charge int
	}{
		{name: "true", t: term{value: true}, predicate: true, bind: true, binds: true},
		{name: "false", t: term{value: false}, bind: true, binds: true},
		{name: "nil holds", t: term{value: nil}, predicate: true, bind: true, binds: true},
		{name: "a number holds", t: term{value: int64(0)}, predicate: true, bind: true, binds: true},
		{name: "a function", t: term{value: fnValue}, predicate: true},
		{name: "created nodes go", t: term{value: true, created: 3}, predicate: true, bind: true, binds: true, charge: 3},
		// A successful term consumed its gathered values into a list; none stays above the mark.
		{name: "kept kids stay", t: term{value: true, kids: 2}, predicate: true, bind: true, binds: true, kidsLeft: 2},
		{name: "failure", t: term{fails: true}},
		{name: "failure drops", t: term{value: true, fails: true, kids: 2, created: 3}, charge: 5},
		{name: "failure of a function", t: term{value: fnValue, fails: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mod := Build()
			for _, fn := range []string{"predicate", "bind"} {
				m := newMachine()
				m.kids, m.created = 1, 1 // what is below the marks stays
				tm := c.t
				var res []any
				var err error
				f := composition(t, mod, fn)
				want := c.predicate
				if fn == "bind" {
					res, err = exec.Run(mod, f, &start{machine: m}, int64(0), &tm)
					want = c.bind
				} else {
					res, err = exec.Run(mod, f, &start{machine: m}, &tm)
				}
				if err != nil {
					t.Fatal(err)
				}
				if res[0] != want {
					t.Errorf("%s = %v, want %v", fn, res[0], want)
				}
				if m.kids != 1+c.kidsLeft || m.created != 1 {
					t.Errorf("%s leaves %d values and %d nodes, want %d and 1", fn, m.kids, m.created, 1+c.kidsLeft)
				}
				if m.spends != c.charge {
					t.Errorf("%s charges %d, want %d", fn, m.spends, c.charge)
				}
				if bound := m.binds == 1; bound != (fn == "bind" && c.binds) {
					t.Errorf("%s made %d bindings", fn, m.binds)
				}
			}
		})
	}
}
