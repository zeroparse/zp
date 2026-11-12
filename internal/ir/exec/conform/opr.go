package conform

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// Operators is an Impl that provides the operations of the Operators Rule descriptors, matches and
// records. Handle returns a new implementation handle for the conformance operation name of the
// frames, backtracking points, values and records that the cases make (OprFrameOp, OprMarkOp,
// OprNodeOp and the operations of OprRecordOps), and Rule (Environment) the Operators Rule of the
// index in ref.Config.Oprs.
type Operators interface {
	Handle(name string) any
}

// The operations that conformance cases use to make a frame, a backtracking point, a value and a record
// of a split form of the Operators Rule loop. They are not operations of the parser VM.
const (
	OprFrameOp = "test.frame"
	OprMarkOp  = "test.mark"
	OprNodeOp  = "test.node"
)

// oprRecordOpPrefix starts the name of the operation that makes a record of the split form with the
// prefix that follows it (OprRecordOp).
const oprRecordOpPrefix = "test.oprRecord."

// OprRecordOp returns the operation that makes a record of the split form with the operations that
// start with prefix (oprLongest, oprNud or oprExpr).
func OprRecordOp(prefix string) string { return oprRecordOpPrefix + prefix }

// isOprTestOp reports whether name is an operation that makes a handle of the Operators Rule family.
func isOprTestOp(name string) bool {
	return name == OprFrameOp || name == OprMarkOp || name == OprNodeOp || strings.HasPrefix(name, oprRecordOpPrefix)
}

// oprRecordHandle is the handle type of the record with the prefix.
var oprRecordHandle = map[string]string{"oprLongest": ops.LongestState, "oprNud": ops.NudState, "oprExpr": ops.ExprState}

// OprDescriptorOps are the operations of the descriptors of an Operators Rule and the matches of
// operators.
var OprDescriptorOps = []string{
	"opr.led", "opr.prefix", "opr.count", "opr.at", "opr.operands", "opr.operand", "opr.line", "opr.level",
	"opr.infix", "opr.postfix", "opr.right", "opr.nonAssoc", "opr.scope", "opr.hasSkip",
	"opr.attempt", "opr.found", "opr.op", "opr.end", "opr.cut", "opr.env", "opr.errs",
	"opr.setCutFailed", "opr.cutFailed", "opr.setNudBound", "opr.nudBound", "opr.setAfterEmpty", "opr.afterEmpty",
}

// OprRecordOps are the operations of the records of the split forms of the loop: for each field, the
// getter and the setter.
var OprRecordOps = func() []string {
	var names []string
	for _, rec := range []string{"oprLongest", "oprNud", "oprExpr"} {
		for _, f := range ref.OprRecords[rec] {
			names = append(names, rec+"."+f, rec+".set"+strings.ToUpper(f[:1])+f[1:])
		}
	}
	return names
}()

// OprOps are all the operations of the Operators Rule family that cases for the reference model cover.
var OprOps = append(slices.Clone(OprDescriptorOps), OprRecordOps...)

// OprSupportOps are the operations that cases of the Operators Rule family call to make the errors and
// the environments they pass: the implementation provides them besides OprOps.
var OprSupportOps = []string{"rec.push", "rec.since", "env.bind", "env.scope"}

// OprSuiteOps returns OprOps and OprSupportOps, the operations a suite of the family provides.
func OprSuiteOps() []string { return append(slices.Clone(OprOps), OprSupportOps...) }

// OprModule returns the declarations of Module with those of the transitional operations, which
// hold the Operators Rule family, and the operations that make the handles of its cases.
func OprModule() *exec.Module {
	m := Module()
	ops.Transitional(m)
	test := func(name string, handle string) {
		m.AddOp(&exec.Op{Name: name, Component: ops.Rules, Results: []exec.Type{exec.THandle(handle)},
			Contract: "Returns a new handle for a conformance case."})
	}
	test(OprFrameOp, ops.Frame)
	test(OprMarkOp, ops.Backtrack)
	test(OprNodeOp, ops.Value)
	for rec, h := range oprRecordHandle {
		test(OprRecordOp(rec), h)
	}
	return m
}

// OprConfig returns the Operators Rules of the cases, with variables for the environment: a rule of
// each kind of operator that skips trivia, a rule without prefix operators, and one with no operator.
func OprConfig() ref.Config {
	op := func(kind, assoc string, level, scope int64) ref.OprOp {
		return ref.OprOp{Kind: kind, Assoc: assoc, Level: level, Scope: scope}
	}
	return ref.Config{
		Vars: []int64{1, 2, 3},
		Oprs: []ref.OprRule{
			{Prefix: []ref.OprOp{op("prefix", "left", 9, 1), op("prefix", "left", 8, 0)},
				Led: []ref.OprOp{op("infix", "left", 1, 0), op("infix", "left", 2, 2), op("infix", "right", 3, 1),
					op("postfix", "left", 4, 0), op("infix", "none", 5, 3)},
				Operands: []int64{0, 2}, Skip: true},
			{Led: []ref.OprOp{op("infix", "right", 1, 0), op("postfix", "none", 2, 1)}, Operands: []int64{1}},
			{},
		},
	}
}

// OprCases returns the handwritten conformance cases of the Operators Rule family, in one module.
func OprCases() []Case {
	m := OprModule()
	var cs []Case
	cfg := OprConfig()
	add := func(name string, body func(b *exec.Builder)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, []string{"budget", "stream", "document"}, body), Config: cfg})
	}
	i := exec.Int
	rule := func(n int64) exec.Expr { return exec.C(RuleOp, i(n)) }
	eq := exec.Eq
	nullAttempt := exec.Null(exec.THandle(ops.Attempt))

	add("opr_descriptors", func(b *exec.Builder) {
		r := b.Let("r", rule(0))
		led, prefix := b.Let("led", exec.C("opr.led", r)), b.Let("prefix", exec.C("opr.prefix", r))
		b.Assert(eq(exec.C("opr.count", led), i(5)), "the rule has five infix and postfix operators")
		b.Assert(eq(exec.C("opr.count", prefix), i(2)), "and two prefix operators")
		b.Let("again", exec.C("opr.led", r))
		for k, want := range []struct {
			level          int64
			infix, postfix bool
			right, none    bool
			scope          int64
		}{{1, true, false, false, false, 0}, {2, true, false, false, false, 2}, {3, true, false, true, false, 1},
			{4, false, true, false, false, 0}, {5, true, false, false, true, 3}} {
			o := b.Let(fmt.Sprint("o", k), exec.C("opr.at", led, i(int64(k))))
			b.Assert(eq(exec.C("opr.level", o), i(want.level)), "the binding power of an operator")
			b.Assert(eq(exec.C("opr.infix", o), exec.BoolC(want.infix)), "infix")
			b.Assert(eq(exec.C("opr.postfix", o), exec.BoolC(want.postfix)), "postfix")
			b.Assert(eq(exec.C("opr.right", o), exec.BoolC(want.right)), "right")
			b.Assert(eq(exec.C("opr.nonAssoc", o), exec.BoolC(want.none)), "non-associative")
			l := b.Let(fmt.Sprint("l", k), exec.C("opr.line", o))
			b.Assert(eq(exec.C("opr.scope", l), i(want.scope)), "the capture slots of an operator's line")
		}
		p0 := b.Let("p0", exec.C("opr.at", prefix, i(0)))
		b.Assert(eq(exec.C("opr.level", p0), i(9)), "a prefix operator's binding power")
		b.Assert(exec.Not(exec.C("opr.infix", p0)), "a prefix operator is not infix")
		b.Assert(eq(exec.C("opr.operands", r), i(2)), "the operand lines of the rule")
		ol := b.Let("ol", exec.C("opr.operand", r, i(1)))
		b.Assert(eq(exec.C("opr.scope", ol), i(2)), "the capture slots of an operand line")
		b.Assert(exec.C("opr.hasSkip", r), "the rule skips trivia")
	})
	add("opr_descriptors_other_rules", func(b *exec.Builder) {
		r1, r2 := b.Let("r1", rule(1)), b.Let("r2", rule(2))
		b.Assert(eq(exec.C("opr.count", exec.C("opr.led", r1)), i(2)), "the second rule's operators")
		b.Assert(eq(exec.C("opr.count", exec.C("opr.prefix", r1)), i(0)), "has no prefix operator")
		b.Assert(exec.Not(exec.C("opr.hasSkip", r1)), "and skips nothing")
		b.Assert(eq(exec.C("opr.operands", r1), i(1)), "with one operand line")
		b.Assert(eq(exec.C("opr.count", exec.C("opr.led", r2)), i(0)), "a rule with no operator")
		b.Assert(eq(exec.C("opr.operands", r2), i(0)), "has no operand line")
		o := b.Let("o", exec.C("opr.at", exec.C("opr.led", r1), i(1)))
		b.Assert(exec.And(exec.C("opr.postfix", o), exec.C("opr.nonAssoc", o)), "a postfix operator that does not associate")
	})
	add("opr_attempt", func(b *exec.Builder) {
		r := b.Let("r", rule(0))
		o := b.Let("o", exec.C("opr.at", exec.C("opr.led", r), i(2)))
		f := b.Let("f", exec.C(OprFrameOp))
		v := b.Let("v", exec.C(OprNodeOp))
		b.Do("rec.push", exec.C(ErrOp, i(4)))
		errs := b.Let("errs", exec.C("rec.since", i(0)))
		b.Do("env.bind", i(1), exec.C(DatumOp, i(7)))
		b.SetReg(ops.Pos, i(11))
		a := b.Let("a", exec.C("opr.attempt", o, f, v, i(5), exec.BoolC(true), errs))
		b.Do("env.bind", i(1), exec.C(DatumOp, i(8)))
		b.SetReg(ops.Pos, i(12))
		a2 := b.Let("a2", exec.C("opr.attempt", o, f, v, i(5), exec.BoolC(false), errs))
		b.Assert(exec.C("opr.found", a), "a match is found")
		b.Assert(exec.Not(exec.C("opr.found", nullAttempt)), "the null attempt is not")
		b.Assert(eq(exec.C("opr.end", a), i(11)), "a match ends at the position it was made at")
		b.Assert(eq(exec.C("opr.end", a2), i(12)), "and the next one at the next position")
		b.Assert(exec.C("opr.cut", a), "a cut is kept")
		b.Assert(exec.Not(exec.C("opr.cut", a2)), "and so is no cut")
		b.Let("o2", exec.C("opr.op", a))
		b.Let("e1", exec.C("opr.env", a))
		b.Let("e2", exec.C("opr.env", a2))
		b.Let("er", exec.C("opr.errs", a))
	})
	add("opr_flags", func(b *exec.Builder) {
		b.Do("opr.setCutFailed", exec.BoolC(true))
		b.Assert(exec.C("opr.cutFailed"), "a candidate failed after a cut")
		b.Do("opr.setCutFailed", exec.BoolC(false))
		b.Assert(exec.Not(exec.C("opr.cutFailed")), "and none did")
		b.Do("opr.setNudBound", i(7))
		b.Assert(eq(exec.C("opr.nudBound"), i(7)), "the binding level above which no operator follows")
		b.Do("opr.setNudBound", i(2))
		b.Assert(eq(exec.C("opr.nudBound"), i(2)), "as the last call set it")
		b.Assert(exec.Not(exec.C("opr.afterEmpty")), "no empty infix operator before any record of one")
		b.Do("opr.setAfterEmpty", exec.BoolC(true))
		b.Assert(exec.C("opr.afterEmpty"), "the right operand follows an empty infix operator")
		b.Do("opr.setAfterEmpty", exec.BoolC(false))
		b.Assert(exec.Not(exec.C("opr.afterEmpty")), "and then does not")
	})
	add("opr_records", func(b *exec.Builder) {
		r := b.Let("r", rule(0))
		led := b.Let("led", exec.C("opr.led", r))
		f1, f2 := b.Let("f1", exec.C(OprFrameOp)), b.Let("f2", exec.C(OprFrameOp))
		m0, m1 := b.Let("m0", exec.C(OprMarkOp)), b.Let("m1", exec.C(OprMarkOp))
		o := b.Let("o", exec.C("opr.at", led, i(0)))
		b.Do("rec.push", exec.C(ErrOp, i(1)))
		errs := b.Let("errs", exec.C("rec.since", i(0)))
		b.SetReg(ops.Pos, i(3))
		a := b.Let("a", exec.C("opr.attempt", o, f1, exec.C(OprNodeOp), i(1), exec.BoolC(false), errs))
		longest := b.Let("longest", exec.C(OprRecordOp("oprLongest")))
		b.Do("oprLongest.setL", longest, led)
		b.Do("oprLongest.setI", longest, i(4))
		b.Do("oprLongest.setM0", longest, m0)
		b.Do("oprLongest.setBest", longest, a)
		b.Do("oprLongest.setCutFailed", longest, exec.BoolC(true))
		b.Do("oprLongest.setPrevFrame", longest, f1)
		b.Do("oprLongest.setPrevCut", longest, exec.BoolC(true))
		b.Do("oprLongest.setF", longest, f2)
		b.Let("l", exec.C("oprLongest.l", longest))
		b.Assert(eq(exec.C("oprLongest.i", longest), i(4)), "the index of the longest match")
		b.Let("lm0", exec.C("oprLongest.m0", longest))
		b.Let("lbest", exec.C("oprLongest.best", longest))
		b.Assert(exec.C("oprLongest.cutFailed", longest), "a failure after a cut")
		b.Let("lpf", exec.C("oprLongest.prevFrame", longest))
		b.Assert(exec.C("oprLongest.prevCut", longest), "the cut flag before")
		b.Let("lf", exec.C("oprLongest.f", longest))
		b.Do("oprLongest.setBest", longest, nullAttempt)
		b.Assert(exec.Not(exec.C("opr.found", exec.C("oprLongest.best", longest))), "the null attempt is a record's best")
		nud := b.Let("nud", exec.C(OprRecordOp("oprNud")))
		b.Do("oprNud.setM0", nud, m1)
		b.Do("oprNud.setA", nud, a)
		b.Do("oprNud.setI", nud, i(2))
		b.Do("oprNud.setPrevFrame", nud, f2)
		b.Do("oprNud.setPrevCut", nud, exec.BoolC(false))
		b.Do("oprNud.setF", nud, f1)
		b.Let("nm0", exec.C("oprNud.m0", nud))
		b.Assert(exec.C("opr.found", exec.C("oprNud.a", nud)), "the match of the start")
		b.Assert(eq(exec.C("oprNud.i", nud), i(2)), "its index")
		b.Let("npf", exec.C("oprNud.prevFrame", nud))
		b.Assert(exec.Not(exec.C("oprNud.prevCut", nud)), "no cut before")
		b.Let("nf", exec.C("oprNud.f", nud))
		expr := b.Let("expr", exec.C(OprRecordOp("oprExpr")))
		b.Do("oprExpr.setMin", expr, i(3))
		b.Do("oprExpr.setLhs", expr, exec.C(OprNodeOp))
		b.Do("oprExpr.setLastNone", expr, i(5))
		b.Do("oprExpr.setBound", expr, i(6))
		b.Do("oprExpr.setGuard", expr, i(8))
		b.Do("oprExpr.setM0", expr, m0)
		b.Do("oprExpr.setA", expr, a)
		b.Assert(eq(exec.C("oprExpr.min", expr), i(3)), "the least binding level")
		b.Let("elhs", exec.C("oprExpr.lhs", expr))
		b.Assert(eq(exec.C("oprExpr.lastNone", expr), i(5)), "the level of the last operator that does not associate")
		b.Assert(eq(exec.C("oprExpr.bound", expr), i(6)), "the bound")
		b.Assert(eq(exec.C("oprExpr.guard", expr), i(8)), "the guard")
		b.Let("em0", exec.C("oprExpr.m0", expr))
		b.Let("ea", exec.C("oprExpr.a", expr))
		// Two records of a kind do not share their fields.
		expr2 := b.Let("expr2", exec.C(OprRecordOp("oprExpr")))
		b.Do("oprExpr.setMin", expr2, i(9))
		b.Assert(eq(exec.C("oprExpr.min", expr), i(3)), "another record leaves the fields of a record as they are")
	})
	return cs
}

// randomOprPool holds the handles a randomized case has made, by handle type, with the model's
// mirror of each so that the case can tell what a call does to them.
type oprEntry struct {
	expr exec.Expr
	h    any
}

// RandomOpr returns a randomized case of n calls of the operations of the Operators Rule family from
// opNames, drawn with rng, that meets every precondition. Handles come from the descriptors of the
// rules of cfg and from the operations that make them, and the positions from a small domain.
func RandomOpr(name string, rng *rand.Rand, opNames []string, n int, cfg ref.Config) Case {
	m := OprModule()
	mirror := ref.New(cfg)
	regs := map[string]any{"pos": int64(0)}
	mirror.UseRegisters(regs)
	pool := map[string][]oprEntry{}
	f := m.AddFunc(name, nil, nil, []string{"budget", "stream", "document"}, func(b *exec.Builder) {
		count := 0
		fresh := func() string { count++; return fmt.Sprintf("x%d", count) }
		keep := func(t exec.Type, expr exec.Expr, h any) {
			pool[t.Handle] = append(pool[t.Handle], oprEntry{expr, h})
		}
		call := func(opName string, args []any, exprs []exec.Expr) {
			o := m.Op(opName)
			if isOprTestOp(opName) {
				h := mirror.OprHandle(opName)
				keep(o.Results[0], b.Let(fresh(), exec.C(opName)), h)
				return
			}
			res := mirror.Call(o, args)
			switch len(o.Results) {
			case 0:
				b.Do(opName, exprs...)
			case 1:
				v := b.Let(fresh(), exec.C(opName, exprs...))
				if o.Results[0].Kind == exec.KHandle {
					keep(o.Results[0], v, res[0])
				}
			}
		}
		// The handles that the cases cannot get from the descriptors.
		for range 3 {
			call(OprFrameOp, nil, nil)
			call(OprMarkOp, nil, nil)
			call(OprNodeOp, nil, nil)
		}
		if slices.ContainsFunc(opNames, func(n string) bool { return slices.Contains(OprRecordOps, n) }) {
			for rec := range oprRecordHandle {
				call(OprRecordOp(rec), nil, nil)
				call(OprRecordOp(rec), nil, nil)
			}
		}
		for k := range 2 {
			e := rng.Int64N(20)
			call("rec.push", []any{&ref.Err{ID: e}}, []exec.Expr{exec.C(ErrOp, exec.Int(e))})
			mk := int64(rng.IntN(k + 1))
			call("rec.since", []any{mk}, []exec.Expr{exec.Int(mk)})
		}
		for range 2 {
			call("env.scope", nil, nil)
			slot := rng.Int64N(3)
			mirror.Call(m.Op("env.bind"), []any{slot, ref.Datum{ID: 5}})
			b.Do("env.bind", exec.Int(slot), exec.C(DatumOp, exec.Int(5)))
		}
		draw := func(p exec.Param, o *exec.Op) (any, exec.Expr, bool) {
			switch p.Type.Kind {
			case exec.KInt:
				v := rng.Int64N(8)
				return v, exec.Int(v), true
			case exec.KBool:
				v := rng.IntN(2) == 0
				return v, exec.BoolC(v), true
			}
			switch p.Type.Handle {
			case ops.Rule:
				i := rng.Int64N(int64(len(cfg.Oprs)))
				return ref.RuleRef{Index: i}, exec.C(RuleOp, exec.Int(i)), true
			case ops.Attempt:
				if rng.IntN(4) == 0 {
					return nil, exec.Null(p.Type), true
				}
			case ops.Errs:
				if o.Name == "opr.attempt" && len(pool[ops.Errs]) == 0 {
					return nil, nil, false
				}
			}
			es := pool[p.Type.Handle]
			if len(es) == 0 {
				return nil, nil, false
			}
			e := es[rng.IntN(len(es))]
			return e.h, e.expr, true
		}
		for i := 0; count < n && i < 80*n; i++ {
			o := m.Op(opNames[rng.IntN(len(opNames))])
			args := make([]any, len(o.Params))
			exprs := make([]exec.Expr, len(o.Params))
			ok := true
			for k, p := range o.Params {
				if args[k], exprs[k], ok = draw(p, o); !ok {
					break
				}
			}
			if !ok || !mirror.Allows(o, args) {
				continue
			}
			if o.Name == "opr.attempt" {
				pos := rng.Int64N(12)
				regs["pos"] = pos
				b.SetReg(ops.Pos, exec.Int(pos))
			}
			call(o.Name, args, exprs)
		}
	})
	return Case{Module: m, Func: f, Config: cfg}
}
