package conform

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The cases of the dependency memo of left recursion (the ctx operations). They install growths and
// begin and end lookaheads with conformance operations (ref.CtxGrowOp and the others) instead of
// running the growth loop, so that the ctx operations are checked apart from how growth steps compute
// seed identities. A Value handle comes from OprNodeOp, so an implementation of the family is also an
// Operators that makes that handle.

// CtxOps are the operations of the dependency memo that cases for the reference model cover.
var CtxOps = []string{
	"ctx.enter", "ctx.found", "ctx.ok", "ctx.value", "ctx.replay", "ctx.allows", "ctx.active",
	"ctx.lookup", "ctx.push", "ctx.pop", "ctx.storeMemo", "ctx.storeCtx", "memo.setPresent",
	"ctx.keep", "ctx.done",
}

// CtxTestOps are the conformance operations that install growths and lookaheads.
var CtxTestOps = []string{ref.CtxGrowOp, ref.CtxReseedOp, ref.CtxUngrowOp, ref.CtxLookBeginOp, ref.CtxLookEndOp}

// CtxSupportOps are the operations that cases of the family call to make the entries, errors and
// expectations they pass: the implementation provides them besides CtxOps and CtxTestOps.
var CtxSupportOps = []string{"memo.store", "rec.mark", "rec.push", "rec.since", "exp.isolate", "exp.add",
	"exp.unisolate", "exp.copy", "exp.empty", "exp.count", "rec.truncate"}

// CtxSuiteOps returns the operations that a suite of the family provides.
func CtxSuiteOps() []string {
	return slices.Concat(CtxOps, CtxTestOps, CtxSupportOps)
}

// CtxModule returns the declarations of ExpModule with OprNodeOp and the conformance operations that
// install growths and lookaheads.
func CtxModule() *exec.Module {
	m := ExpModule()
	rule := exec.THandle(ops.Rule)
	m.AddOp(&exec.Op{Name: OprNodeOp, Component: ops.Rules, Results: []exec.Type{exec.THandle(ops.Value)},
		Contract: "Returns a new handle for a conformance case."})
	i, b := exec.TInt, exec.TBool
	m.AddOp(&exec.Op{Name: ref.CtxGrowOp, Component: ops.Growth, Writes: []string{ops.Growth},
		Params: []exec.Param{exec.P("r", rule), exec.P("min", i), exec.P("ok", b), exec.P("end", i), exec.P("id", i)},
		Contract: `Begins the growth of the leader r at the binding level min at the position with the seed
that ended at end, matched if ok, with the identity id.`,
		Requires: `r is a leader that does not grow at the position; a growth of another cycle at the position
is not the innermost.`})
	m.AddOp(&exec.Op{Name: ref.CtxReseedOp, Component: ops.Growth, Writes: []string{ops.Growth},
		Params:   []exec.Param{exec.P("ok", b), exec.P("end", i), exec.P("id", i)},
		Contract: `Gives the innermost growth the seed that ended at end, matched if ok, with the identity id.`,
		Requires: `A growth is in progress.`})
	m.AddOp(&exec.Op{Name: ref.CtxUngrowOp, Component: ops.Growth, Writes: []string{ops.Growth},
		Contract: `Ends the innermost growth, and the table of its context if it is the first growth there.`,
		Requires: `A growth is in progress, and every frame that began inside it ended.`})
	m.AddOp(&exec.Op{Name: ref.CtxLookBeginOp, Component: ops.Growth, Writes: []string{ops.Growth},
		Params:   []exec.Param{exec.P("site", i)},
		Contract: `Begins a lookahead of the site at the position.`, Requires: `site >= 0.`})
	m.AddOp(&exec.Op{Name: ref.CtxLookEndOp, Component: ops.Growth, Writes: []string{ops.Growth},
		Params: []exec.Param{exec.P("ok", b)},
		Contract: `Ends the innermost lookahead, which matched if ok. The case discards the errors recovered
inside it before, as a lookahead does.`,
		Requires: `Every frame that began inside the lookahead ended.`})
	return m
}

// CtxConfig returns the rules of the cases: a cycle of two leaders and a rule that is not one, a rule
// of no cycle, a second cycle, and a cycle of more leaders than a word of bits of present sets holds.
func CtxConfig() ref.Config {
	rules := []ref.CtxRule{{Cycle: 1, Leader: true}, {Cycle: 1, Leader: true}, {Cycle: 1}, {}, {Cycle: 2, Leader: true}, {Cycle: 2}}
	for range 33 {
		rules = append(rules, ref.CtxRule{Cycle: 3, Leader: true})
	}
	rules = append(rules, ref.CtxRule{Cycle: 3})
	return ref.Config{Rules: len(rules), CtxRules: rules}
}

// The rules of CtxConfig.
const (
	ctxLeaderA   = 0
	ctxLeaderB   = 1
	ctxMember    = 2
	ctxPlain     = 3
	ctxOtherLead = 4
	ctxOther     = 5
	// ctxNarrowLead is the first leader of the third cycle, which has a bit in a word of bits of present
	// sets, ctxWideLead a leader of that cycle without one, and ctxWide a rule of that cycle.
	ctxNarrowLead = 6
	// ctxEdgeLead is the leader of index 31 of the third cycle, the first that a word of 31 bits
	// leaves out.
	ctxEdgeLead = 37
	ctxWideLead = 38
	ctxWide     = 39
)

// CtxCases returns the handwritten conformance cases of the dependency memo, in one module.
func CtxCases() []Case {
	m := CtxModule()
	var cs []Case
	cfg := CtxConfig()
	add := func(name string, body func(b *exec.Builder)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, []string{"budget", "stream", "document"}, body), Config: cfg})
	}
	i, t, f := exec.Int, exec.BoolC(true), exec.BoolC(false)
	rule := func(n int64) exec.Expr { return exec.C(RuleOp, i(n)) }
	eq := exec.Eq
	grow := func(b *exec.Builder, r, min int64, ok bool, end, id int64) {
		b.Do(ref.CtxGrowOp, rule(r), i(min), exec.BoolC(ok), i(end), i(id))
	}
	enter := func(b *exec.Builder, name string, r int64) (exec.Expr, exec.Expr) {
		cs := b.LetN([]string{name, name + "s"}, exec.C("ctx.enter", rule(r), i(0)))
		return cs[0], cs[1]
	}
	noValue, noErrs := exec.Null(exec.THandle(ops.Value)), exec.Null(exec.THandle(ops.Errs))
	keep := func(b *exec.Builder, c exec.Expr, r, key int64, ok bool, end int64, value, errs, k exec.Expr) {
		b.Do("ctx.keep", c, rule(r), i(0), i(0), exec.C(ValuesOp, i(key)), exec.BoolC(ok), i(end), f, value, errs, i(end), k)
	}
	// readSeed evaluates a call of the rule that reads the seed of the leader r: its result depends on it.
	readSeed := func(b *exec.Builder, name string, r int64) {
		_, s := enter(b, name, r)
		b.Assert(exec.C("ctx.found", s), "a leader that grows at the position reads its seed")
	}

	add("ctx_outside", func(b *exec.Builder) {
		c, s := enter(b, "c", ctxPlain)
		b.Assert(exec.Not(exec.C("ctx.found", s)), "a rule of no cycle reads no seed")
		b.Assert(exec.Not(exec.C("ctx.active", c)), "and is in no context")
		b.Do("ctx.push", c)
		b.Assert(eq(exec.C("ctx.pop", c), exec.C("ctx.storeMemo")), "its result goes to the memo")
		b.Do("ctx.done", c)
		grow(b, ctxLeaderA, 0, true, 2, 7)
		c2, _ := enter(b, "c2", ctxOther)
		b.Assert(exec.Not(exec.C("ctx.active", c2)), "the growth of another cycle is no context of the call")
		b.SetReg(ops.Pos, i(1))
		c3, _ := enter(b, "c3", ctxMember)
		b.Assert(exec.Not(exec.C("ctx.active", c3)), "nor is a growth at another position")
	})
	add("ctx_seed", func(b *exec.Builder) {
		grow(b, ctxLeaderA, 0, true, 3, 7)
		_, s := enter(b, "c", ctxLeaderA)
		b.Assert(exec.C("ctx.found", s), "the leader that grows reads its seed")
		b.Assert(exec.C("ctx.ok", s), "which matched")
		b.Let("v", exec.C("ctx.value", s))
		b.Do("ctx.replay", s, t)
		b.Assert(eq(exec.R(ops.Pos), i(3)), "replaying the seed moves to its end")
		b.SetReg(ops.Pos, i(0))
		cb, sb := enter(b, "cb", ctxLeaderB)
		b.Assert(exec.Not(exec.C("ctx.found", sb)), "another leader of the cycle does not grow")
		b.Assert(exec.C("ctx.active", cb), "but is in the context")
		_, s1 := enter(b, "c1", ctxLeaderA)
		b.Assert(exec.Not(exec.C("ctx.found", exec.C("ctx.lookup", cb, rule(ctxLeaderB), i(0), exec.C(ValuesOp, i(0))))),
			"nothing is kept yet")
		b.Do("ctx.replay", s1, t)
		b.Do(ref.CtxReseedOp, f, i(0), i(8))
		b.SetReg(ops.Pos, i(0))
		_, s2 := enter(b, "c2", ctxLeaderA)
		b.Assert(exec.Not(exec.C("ctx.ok", s2)), "the new seed failed")
		b.Do("ctx.replay", s2, t)
		b.Assert(eq(exec.R(ops.Pos), i(0)), "and replaying it stays")
		b.Do(ref.CtxUngrowOp)
		_, s3 := enter(b, "c3", ctxLeaderA)
		b.Assert(exec.Not(exec.C("ctx.found", s3)), "after the growth there is no seed")
	})
	add("ctx_keep_lookup", func(b *exec.Builder) {
		grow(b, ctxLeaderA, 0, true, 3, 7)
		c, _ := enter(b, "c", ctxMember)
		b.Assert(exec.C("ctx.active", c), "a rule of the cycle is in the context")
		b.Assert(exec.Not(exec.C("ctx.found", exec.C("ctx.lookup", c, rule(ctxMember), i(0), exec.C(ValuesOp, i(1))))), "nothing kept")
		b.Do("ctx.push", c)
		readSeed(b, "in", ctxLeaderA)
		b.Assert(eq(exec.C("ctx.pop", c), exec.C("ctx.storeCtx")), "a result that read a seed goes to the context")
		v := b.Let("v", exec.C(OprNodeOp))
		b.Do("rec.push", exec.C(ErrOp, i(3)))
		errs := b.Let("errs", exec.C("rec.since", i(0)))
		keep(b, c, ctxMember, 1, true, 4, v, errs, exec.C("exp.empty"))
		b.Do("ctx.done", c)
		c2, _ := enter(b, "c2", ctxMember)
		h := b.Let("h", exec.C("ctx.lookup", c2, rule(ctxMember), i(0), exec.C(ValuesOp, i(1))))
		b.Assert(exec.C("ctx.found", h), "the kept result holds while the seed is the same")
		b.Assert(exec.C("ctx.ok", h), "it matched")
		b.Let("hv", exec.C("ctx.value", h))
		b.Do("ctx.replay", h, f)
		b.Assert(eq(exec.R(ops.Pos), i(4)), "replaying moves to its end")
		b.SetReg(ops.Pos, i(0))
		c3, _ := enter(b, "c3", ctxMember)
		b.Assert(exec.Not(exec.C("ctx.found", exec.C("ctx.lookup", c3, rule(ctxMember), i(0), exec.C(ValuesOp, i(2))))),
			"another key has no result")
		b.Do(ref.CtxReseedOp, t, i(5), i(9))
		c4, _ := enter(b, "c4", ctxMember)
		b.Assert(exec.Not(exec.C("ctx.found", exec.C("ctx.lookup", c4, rule(ctxMember), i(0), exec.C(ValuesOp, i(1))))),
			"nor a seed with another identity")
		b.Do(ref.CtxReseedOp, t, i(3), i(7))
		c5, _ := enter(b, "c5", ctxMember)
		b.Assert(exec.C("ctx.found", exec.C("ctx.lookup", c5, rule(ctxMember), i(0), exec.C(ValuesOp, i(1)))),
			"the identity it read holds again")
	})
	add("ctx_present", func(b *exec.Builder) {
		grow(b, ctxLeaderA, 0, true, 3, 7)
		c, _ := enter(b, "c", ctxMember)
		b.Do("ctx.push", c)
		b.Assert(eq(exec.C("ctx.pop", c), exec.C("ctx.storeMemo")), "a result that read no seed goes to the memo")
		e := b.Let("e", exec.C("memo.store", i(ctxMember), i(0), i(0), exec.C(ValuesOp, i(0)), t, i(1), f, noValue, noErrs, f))
		b.Do("memo.setPresent", e, c)
		b.Do("ctx.done", c)
		b.Assert(exec.C("ctx.allows", c, e), "the context where it was stored reuses it")
		grow(b, ctxLeaderB, 0, false, 0, 1)
		c2, _ := enter(b, "c2", ctxMember)
		b.Assert(exec.Not(exec.C("ctx.allows", c2, e)), "not where another leader grows")
		b.Do(ref.CtxUngrowOp)
		c3, _ := enter(b, "c3", ctxPlain)
		b.Assert(exec.C("ctx.allows", c3, e), "a call in no context reuses it")
		c4, _ := enter(b, "c4", ctxMember)
		b.Do("ctx.push", c4)
		b.Assert(eq(exec.C("ctx.pop", c4), exec.C("ctx.storeMemo")), "a result in both leaders' context")
		e2 := b.Let("e2", exec.C("memo.store", i(ctxMember), i(0), i(0), exec.C(ValuesOp, i(1)), t, i(1), f, noValue, noErrs, f))
		b.Do("memo.setPresent", e2, c4)
		b.Do("ctx.done", c4)
		b.Assert(exec.C("ctx.allows", c4, e2), "is reused there")
	})
	add("ctx_lookahead", func(b *exec.Builder) {
		grow(b, ctxLeaderA, 0, true, 3, 7)
		c, _ := enter(b, "c", ctxMember)
		b.Do("ctx.push", c)
		b.Do(ref.CtxLookBeginOp, i(5))
		b.SetReg(ops.Silent, i(1))
		readSeed(b, "in", ctxLeaderA)
		b.Do("rec.push", exec.C(ErrOp, i(2)))
		b.Do("rec.truncate", i(0))
		b.SetReg(ops.Silent, i(0))
		b.Do(ref.CtxLookEndOp, t)
		b.Assert(eq(exec.C("ctx.pop", c), exec.C("ctx.storeCtx")), "a result that observed a lookahead goes to the context")
		keep(b, c, ctxMember, 0, false, 0, noValue, noErrs, exec.C("exp.empty"))
		b.Do("ctx.done", c)
		c2, _ := enter(b, "c2", ctxMember)
		b.Assert(exec.C("ctx.found", exec.C("ctx.lookup", c2, rule(ctxMember), i(0), exec.C(ValuesOp, i(0)))),
			"the outcome of the lookahead holds in the context")
		b.Do(ref.CtxReseedOp, t, i(4), i(8))
		c3, _ := enter(b, "c3", ctxMember)
		b.Assert(exec.Not(exec.C("ctx.found", exec.C("ctx.lookup", c3, rule(ctxMember), i(0), exec.C(ValuesOp, i(0))))),
			"a new seed has no memoized outcome")
		// The lookahead again, with the new seed: its outcome is the same.
		b.Do("ctx.push", c3)
		b.Do(ref.CtxLookBeginOp, i(5))
		readSeed(b, "in2", ctxLeaderA)
		b.Do(ref.CtxLookEndOp, t)
		b.Assert(eq(exec.C("ctx.pop", c3), exec.C("ctx.storeCtx")), "observed again")
		keep(b, c3, ctxMember, 0, false, 0, noValue, noErrs, exec.C("exp.empty"))
		b.Do("ctx.done", c3)
		c4, _ := enter(b, "c4", ctxMember)
		b.Assert(exec.C("ctx.found", exec.C("ctx.lookup", c4, rule(ctxMember), i(0), exec.C(ValuesOp, i(0)))),
			"the result holds under the same outcome")
	})
	add("ctx_expectations", func(b *exec.Builder) {
		mk := b.Let("mk", exec.C("exp.isolate", i(0)))
		b.Do("exp.add", exec.C(ExpOp, i(3)))
		b.Do("exp.add", exec.C(ExpOp, i(4)))
		fs := b.LetN([]string{"far", "set"}, exec.C("exp.unisolate", mk))
		k := b.Let("k", exec.C("exp.copy", fs[1]))
		b.Do("exp.add", exec.C(ExpOp, i(4)))
		grow(b, ctxLeaderA, 0, true, 3, 7)
		c, _ := enter(b, "c", ctxMember)
		b.Do("ctx.push", c)
		readSeed(b, "in", ctxLeaderA)
		b.Assert(eq(exec.C("ctx.pop", c), exec.C("ctx.storeCtx")), "kept")
		b.Do("ctx.keep", c, rule(ctxMember), i(0), i(0), exec.C(ValuesOp, i(0)), t, i(2), f, noValue, noErrs, i(0), k)
		b.Do("ctx.done", c)
		c2, _ := enter(b, "c2", ctxMember)
		h := b.Let("h", exec.C("ctx.lookup", c2, rule(ctxMember), i(0), exec.C(ValuesOp, i(0))))
		b.Do("ctx.replay", h, f)
		b.Assert(eq(exec.C("exp.count"), i(2)), "replaying a result adds its expectations")
	})
	add("ctx_wide_edge", func(b *exec.Builder) {
		grow(b, ctxEdgeLead, 0, true, 1, 7)
		c, _ := enter(b, "c", ctxWide)
		b.Do("ctx.push", c)
		b.Assert(eq(exec.C("ctx.pop", c), exec.C("ctx.storeCtx")), "the leader of index 31 has no bit: a word holds 31 leaders")
	})
	add("ctx_wide_present", func(b *exec.Builder) {
		grow(b, ctxWideLead, 0, true, 1, 7)
		c, _ := enter(b, "c", ctxWide)
		b.Assert(exec.C("ctx.active", c), "the call is in the context")
		e := b.Let("e", exec.C("memo.store", i(ctxWide), i(0), i(0), exec.C(ValuesOp, i(0)), t, i(1), f, noValue, noErrs, f))
		b.Assert(exec.Not(exec.C("ctx.allows", c, e)), "a memo entry answers no call in a context with a leader that has no bit")
		b.Do("ctx.push", c)
		b.Assert(eq(exec.C("ctx.pop", c), exec.C("ctx.storeCtx")), "a result that read no seed goes to the context too")
		keep(b, c, ctxWide, 0, true, 1, noValue, noErrs, exec.C("exp.empty"))
		b.Do("ctx.done", c)
		c2, _ := enter(b, "c2", ctxWide)
		b.Assert(exec.C("ctx.found", exec.C("ctx.lookup", c2, rule(ctxWide), i(0), exec.C(ValuesOp, i(0)))),
			"the context finds the result under the same present set")
		grow(b, ctxNarrowLead, 0, true, 1, 8)
		c3, _ := enter(b, "c3", ctxWide)
		b.Assert(exec.Not(exec.C("ctx.found", exec.C("ctx.lookup", c3, rule(ctxWide), i(0), exec.C(ValuesOp, i(0))))),
			"another set of leaders has another present set")
		b.Do(ref.CtxUngrowOp)
		c4, _ := enter(b, "c4", ctxWide)
		b.Assert(exec.C("ctx.found", exec.C("ctx.lookup", c4, rule(ctxWide), i(0), exec.C(ValuesOp, i(0)))),
			"the first set names the same present set again")
		b.Do(ref.CtxUngrowOp)
		grow(b, ctxWideLead, 0, true, 1, 7)
		c5, _ := enter(b, "c5", ctxWide)
		b.Assert(exec.Not(exec.C("ctx.found", exec.C("ctx.lookup", c5, rule(ctxWide), i(0), exec.C(ValuesOp, i(0))))),
			"a new growth starts with an empty table")
	})
	return cs
}

// ctxEntry is a handle that a randomized case made, with the model's mirror of it.
type ctxEntry struct {
	expr exec.Expr
	h    any
}

// RandomCtx returns a randomized case of the dependency memo with about n calls, drawn with rng, that
// meets every precondition: growths, calls and lookaheads nest as the call protocol nests them, and
// a call stores its result where ctx.pop sends it. The positions, keys and identities come from small
// domains, so that results are found again.
func RandomCtx(name string, rng *rand.Rand, n int) Case {
	m := CtxModule()
	cfg := CtxConfig()
	mirror := ref.New(cfg)
	regs := map[string]any{ops.Pos: int64(0), ops.Silent: int64(0)}
	mirror.UseRegisters(regs)
	f := m.AddFunc(name, nil, nil, []string{"budget", "stream", "document"}, func(b *exec.Builder) {
		count := 0
		fresh := func() string { count++; return fmt.Sprintf("x%d", count) }
		var values, errs, kept, entries []ctxEntry
		// call runs the operation on the mirror and emits it; it returns the mirror's results and the
		// expressions of the case's, or false if the call breaks the precondition.
		call := func(opName string, args []any, exprs ...exec.Expr) ([]any, []exec.Expr, bool) {
			o := m.Op(opName)
			if !mirror.Allows(o, args) {
				return nil, nil, false
			}
			res := mirror.Call(o, args)
			switch len(o.Results) {
			case 0:
				b.Do(opName, exprs...)
				return nil, nil, true
			case 1:
				return res, []exec.Expr{b.Let(fresh(), exec.C(opName, exprs...))}, true
			}
			names := make([]string, len(o.Results))
			for k := range names {
				names[k] = fresh()
			}
			return res, b.LetN(names, exec.C(opName, exprs...)), true
		}
		setReg := func(reg string, v int64) {
			regs[reg] = v
			b.SetReg(reg, exec.Int(v))
		}
		pick := func(es []ctxEntry) ctxEntry { return es[rng.IntN(len(es))] }
		ruleArg := func(r int64) (any, exec.Expr) { return ref.RuleRef{Index: r}, exec.C(RuleOp, exec.Int(r)) }

		// The handles the calls pass.
		for range 2 {
			values = append(values, ctxEntry{b.Let(fresh(), exec.C(OprNodeOp)), mirror.OprHandle(OprNodeOp)})
		}
		values = append(values, ctxEntry{exec.Null(exec.THandle(ops.Value)), nil})
		errs = append(errs, ctxEntry{exec.Null(exec.THandle(ops.Errs)), nil})
		for k := range 2 {
			e := rng.Int64N(20)
			call("rec.push", []any{&ref.Err{ID: e}}, exec.C(ErrOp, exec.Int(e)))
			res, ex, _ := call("rec.since", []any{int64(k)}, exec.Int(int64(k)))
			errs = append(errs, ctxEntry{ex[0], res[0]})
		}
		res, ex, _ := call("exp.empty", nil)
		kept = append(kept, ctxEntry{ex[0], res[0]})
		mk, mkx, _ := call("exp.isolate", []any{int64(0)}, exec.Int(0))
		for _, id := range []int64{3, 4} {
			call("exp.add", []any{ref.ExpID{ID: id}}, exec.C(ExpOp, exec.Int(id)))
		}
		set, setx, _ := call("exp.unisolate", []any{mk[0]}, mkx[0])
		res, ex, _ = call("exp.copy", []any{set[1]}, setx[1])
		kept = append(kept, ctxEntry{ex[0], res[0]})

		// The rules that calls draw from, by cycle: the cycles of two leaders and of many.
		cycles := [][]int64{{ctxLeaderA, ctxLeaderB, ctxMember, ctxPlain}, {ctxOtherLead, ctxOther}, {ctxWideLead, ctxNarrowLead, ctxWide}}
		cycle := cycles[0]
		if rng.IntN(4) == 0 {
			cycle = cycles[1+rng.IntN(2)]
		}
		leaders := func() []int64 {
			var ls []int64
			for _, r := range cycle {
				if r != ctxMember && r != ctxPlain && r != ctxOther && r != ctxWide {
					ls = append(ls, r)
				}
			}
			return ls
		}()
		ids := func() int64 { return 1 + rng.Int64N(3) }
		pos := func() int64 {
			if rng.IntN(5) == 0 {
				return 1
			}
			return 0
		}

		var items func(depth int, inGrowth bool)
		var callBlock func(depth int, r, key int64)
		// callBlock is a memoized call of r with the key as the call protocol makes it.
		callBlock = func(depth int, r, key int64) {
			start := regs[ops.Pos].(int64)
			ra, rx := ruleArg(r)
			cs, cx, _ := call("ctx.enter", []any{ra, int64(0)}, rx, exec.Int(0))
			c, cExpr := cs[0], cx[0]
			if res, _, _ := call("ctx.found", []any{cs[1]}, cx[1]); res[0].(bool) {
				call("ctx.ok", []any{cs[1]}, cx[1])
				call("ctx.value", []any{cs[1]}, cx[1])
				call("ctx.replay", []any{cs[1], true}, cx[1], exec.BoolC(true))
				setReg(ops.Pos, start)
				return
			}
			// An entry that another store replaced is no longer a handle.
			entries = slices.DeleteFunc(entries, func(e ctxEntry) bool { return mirror.Dead(e.h) })
			if len(entries) > 0 && rng.IntN(2) == 0 {
				e := pick(entries)
				call("ctx.allows", []any{c, e.h}, cExpr, e.expr)
			}
			if res, _, _ := call("ctx.active", []any{c}, cExpr); res[0].(bool) {
				h, hx, _ := call("ctx.lookup", []any{c, ra, int64(0), key}, cExpr, rx, exec.Int(0), exec.C(ValuesOp, exec.Int(key)))
				if res, _, _ := call("ctx.found", []any{h[0]}, hx[0]); res[0].(bool) {
					call("ctx.ok", []any{h[0]}, hx[0])
					call("ctx.value", []any{h[0]}, hx[0])
					if regs[ops.Silent].(int64) == 0 {
						call("ctx.replay", []any{h[0], false}, hx[0], exec.BoolC(false))
						call("exp.count", nil)
					}
					setReg(ops.Pos, start)
					return
				}
			}
			call("ctx.push", []any{c}, cExpr)
			if rng.IntN(2) == 0 {
				// A read of a seed, so that the result goes to the context.
				callBlock(depth+1, leaders[rng.IntN(len(leaders))], 0)
			}
			items(depth+1, false)
			setReg(ops.Pos, start)
			res, _, _ := call("ctx.pop", []any{c}, cExpr)
			call("ctx.storeMemo", nil)
			call("ctx.storeCtx", nil)
			ok, end := rng.IntN(3) > 0, start+rng.Int64N(4)
			switch store := res[0].(int64); {
			case store == ref.CtxStoreCtx && !cfg.CtxRules[r].Leader:
				v, er, k := pick(values), pick(errs), pick(kept)
				silent := regs[ops.Silent].(int64) > 0
				call("ctx.keep", []any{c, ra, start, int64(0), key, ok, end, silent, v.h, er.h, end, k.h},
					cExpr, rx, exec.Int(start), exec.Int(0), exec.C(ValuesOp, exec.Int(key)), exec.BoolC(ok), exec.Int(end),
					exec.BoolC(silent), v.expr, er.expr, exec.Int(end), k.expr)
			case store == ref.CtxStoreMemo && rng.IntN(2) == 0:
				res, ex, _ := call("memo.store", []any{r, start, int64(0), key, ok, end, false, nil, nil, false},
					exec.Int(r), exec.Int(start), exec.Int(0), exec.C(ValuesOp, exec.Int(key)), exec.BoolC(ok), exec.Int(end),
					exec.BoolC(false), exec.Null(exec.THandle(ops.Value)), exec.Null(exec.THandle(ops.Errs)), exec.BoolC(false))
				call("memo.setPresent", []any{res[0], c}, ex[0], cExpr)
				entries = append(entries, ctxEntry{ex[0], res[0]})
			}
			call("ctx.done", []any{c}, cExpr)
		}
		items = func(depth int, inGrowth bool) {
			for k := rng.IntN(4); k >= 0 && count < n; k-- {
				switch x := rng.IntN(10); {
				case x < 2 && depth < 4:
					// A growth at a position, with its seeds.
					setReg(ops.Pos, pos())
					r := leaders[rng.IntN(len(leaders))]
					ra, rx := ruleArg(r)
					ok, end, id := rng.IntN(2) == 0, regs[ops.Pos].(int64)+rng.Int64N(3), ids()
					if _, _, allowed := call(ref.CtxGrowOp, []any{ra, int64(0), ok, end, id}, rx, exec.Int(0), exec.BoolC(ok), exec.Int(end), exec.Int(id)); !allowed {
						continue
					}
					items(depth+1, true)
					call(ref.CtxUngrowOp, nil)
				case x < 3 && inGrowth && rng.IntN(2) == 0:
					ok, end, id := rng.IntN(2) == 0, regs[ops.Pos].(int64)+rng.Int64N(3), ids()
					call(ref.CtxReseedOp, []any{ok, end, id}, exec.BoolC(ok), exec.Int(end), exec.Int(id))
				case x < 5 && depth < 5:
					site := rng.Int64N(3)
					mark, markx, _ := call("rec.mark", nil)
					call(ref.CtxLookBeginOp, []any{site}, exec.Int(site))
					silent := regs[ops.Silent].(int64)
					setReg(ops.Silent, silent+1)
					items(depth+1, false)
					if rng.IntN(3) == 0 {
						e := rng.Int64N(20)
						call("rec.push", []any{&ref.Err{ID: e}}, exec.C(ErrOp, exec.Int(e)))
					}
					call("rec.truncate", []any{mark[0]}, markx[0])
					setReg(ops.Silent, silent)
					ok := rng.IntN(2) == 0
					call(ref.CtxLookEndOp, []any{ok}, exec.BoolC(ok))
				case depth < 6:
					r, key := cycle[rng.IntN(len(cycle))], rng.Int64N(2)
					callBlock(depth, r, key)
					if rng.IntN(2) == 0 {
						// The same call again, which may reuse the result of the first.
						callBlock(depth, r, key)
					}
				}
			}
		}
		for count < n {
			items(0, false)
		}
	})
	return Case{Module: m, Func: f, Config: cfg}
}
