package conform

import (
	"fmt"
	"math/rand/v2"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/exec"
	"zeroparse.dev/zp/internal/ir/code/exec/ref"
)

// Documents is an Impl that provides the doc operations, which an implementation that has a
// Document provides together with the statistics of it.
type Documents interface {
	// DocStat returns the statistics of the Document: the memo entries reused for 0, the rule bodies
	// evaluated for 1.
	DocStat(i int64) int64
}

// DocStatOp is the operation that conformance cases use to read a statistic of the Document
// (Documents.DocStat). It is not an operation of the parser VM.
const DocStatOp = "test.docStat"

// DocOps are the doc operations of the examined input of a Document parse.
var DocOps = []string{
	"doc.examinedBegin", "doc.examinedEnd", "doc.tracking", "doc.enter", "doc.leave", "doc.active", "doc.touch",
	"doc.touchFrom", "doc.low", "doc.examined", "doc.reused", "doc.evaluated",
}

// DocCoreOps are the doc operations of a target that does not observe calls and tracks the examined
// input of every parse: all but doc.examinedBegin, doc.examinedEnd and doc.tracking.
var DocCoreOps = DocOps[3:]

// DocModule returns the declarations of Module with DocStatOp.
func DocModule() *code.Module {
	m := Module()
	m.AddOp(&code.Op{Name: DocStatOp, Component: exec.Call, Params: []code.Param{code.P("i", code.TInt)},
		Results:  []code.Type{code.TInt},
		Contract: "Returns the statistic i of the Document: the memo entries reused for 0, the rule bodies evaluated for 1."})
	return m
}

// docInt is an argument or result of a doc operation: a position of the input.
func docRegs() []string { return []string{"budget", "stream", "document"} }

// DocCases returns the handwritten conformance cases of the doc operations, in one module.
func DocCases() []Case {
	m := DocModule()
	var cs []Case
	add := func(name string, cfg ref.Config, body func(b *code.Builder)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, docRegs(), body), Config: cfg})
	}
	doc := ref.Config{Document: true}
	observed := ref.Config{Observed: true}
	low := func() code.Expr { return code.C("doc.low") }
	high := func() code.Expr { return code.C("doc.examined") }
	i := code.Int
	// starts asserts the range of a call that began at pos: the empty range there, since a call has
	// examined nothing when it begins.
	starts := func(b *code.Builder, pos int64, msg string) {
		b.Assert(code.And(code.Eq(low(), i(pos)), code.Eq(high(), i(pos))), msg)
	}

	add("doc_active", ref.Config{}, func(b *code.Builder) {
		b.Assert(code.Not(code.C("doc.active")), "a parse from scratch reads no Document")
		b.Assert(code.Not(code.C("doc.tracking")), "nor tracks the examined input unless it is observed")
		b.Do("doc.touch", i(9))
		b.Do("doc.touchFrom", i(0))
		b.Assert(code.Not(code.C("doc.active")), "touching the input does not make a Document")
	})
	add("doc_active_document", doc, func(b *code.Builder) {
		b.Assert(code.C("doc.active"), "a Document parse reads a Document")
		b.Assert(code.C("doc.tracking"), "and tracks the examined input")
	})
	add("doc_tracking_observed", observed, func(b *code.Builder) {
		b.Assert(code.Not(code.C("doc.active")), "an observed parse from scratch reads no Document")
		b.Assert(code.C("doc.tracking"), "but tracks the examined input")
		b.Do("doc.touch", i(6))
		b.Assert(code.Eq(high(), i(6)), "and reading extends its examined end")
	})

	add("doc_enter_leave", doc, func(b *code.Builder) {
		r := b.LetN([]string{"h0", "l0"}, code.C("doc.enter", i(5)))
		b.Assert(code.And(code.Eq(r[0], i(0)), code.Eq(r[1], i(0))), "the first call encloses the empty range at 0")
		starts(b, 5, "a call starts its examined input at its position")
		b.Do("doc.touch", i(8))
		b.Assert(code.And(code.Eq(low(), i(5)), code.Eq(high(), i(8))), "reading extends the examined end")
		b.Do("doc.touch", i(7))
		b.Assert(code.Eq(high(), i(8)), "an examined end does not move back")
		b.Do("doc.touchFrom", i(3))
		b.Assert(code.And(code.Eq(low(), i(3)), code.Eq(high(), i(8))), "examining before the start extends the start back")
		b.Do("doc.touchFrom", i(4))
		b.Assert(code.Eq(low(), i(3)), "a start does not move forward")
		n := b.LetN([]string{"h1", "l1"}, code.C("doc.enter", i(6)))
		b.Assert(code.And(code.Eq(n[0], i(8)), code.Eq(n[1], i(3))), "a nested call encloses the range of its parent")
		starts(b, 6, "and starts its own at its position")
		b.Do("doc.touch", i(7))
		b.Do("doc.leave", n[0], n[1])
		b.Assert(code.And(code.Eq(low(), i(3)), code.Eq(high(), i(8))), "the parent's range holds its own and the nested call's")
		m := b.LetN([]string{"h2", "l2"}, code.C("doc.enter", i(1)))
		b.Do("doc.touch", i(12))
		b.Do("doc.leave", m[0], m[1])
		b.Assert(code.And(code.Eq(low(), i(1)), code.Eq(high(), i(12))), "a nested call that reads farther widens the range at both ends")
		b.Do("doc.leave", r[0], r[1])
		b.Assert(code.And(code.Eq(low(), i(0)), code.Eq(high(), i(12))), "the range of the outermost call merges into the enclosing empty range")
	})

	add("doc_leave_keeps_enclosing", doc, func(b *code.Builder) {
		b.Do("doc.touch", i(20))
		b.Do("doc.touchFrom", i(0))
		r := b.LetN([]string{"h", "l"}, code.C("doc.enter", i(5)))
		b.Assert(code.And(code.Eq(r[0], i(20)), code.Eq(r[1], i(0))), "a call encloses the range before it")
		b.Do("doc.leave", r[0], r[1])
		b.Assert(code.And(code.Eq(low(), i(0)), code.Eq(high(), i(20))), "a call that stays inside the enclosing range leaves it as it is")
	})

	add("doc_statistics", doc, func(b *code.Builder) {
		stat := func(n int64) code.Expr { return code.C(DocStatOp, i(n)) }
		b.Assert(code.And(code.Eq(stat(0), i(0)), code.Eq(stat(1), i(0))), "a Document starts with no reuse and no evaluation")
		b.Do("doc.reused")
		b.Do("doc.reused")
		b.Do("doc.evaluated")
		b.Assert(code.And(code.Eq(stat(0), i(2)), code.Eq(stat(1), i(1))), "each count is its own")
	})

	add("doc_examined", doc, func(b *code.Builder) {
		b.Do("doc.touch", i(3))
		b.SetReg(exec.Pos, i(4))
		h := b.Let("h", code.C("doc.examinedBegin"))
		b.Assert(code.Eq(h, i(3)), "an observed call encloses the examined end before it")
		b.Assert(code.Eq(high(), i(4)), "its own examined end starts at the position")
		b.Do("doc.touch", i(9))
		x := b.Let("x", code.C("doc.examinedEnd", h))
		b.Assert(code.Eq(x, i(9)), "it returns its examined end")
		b.Assert(code.Eq(high(), i(9)), "and merges it into the enclosing one")
		b.SetReg(exec.Pos, i(1))
		g := b.Let("g", code.C("doc.examinedBegin"))
		b.Assert(code.Eq(g, i(9)), "the next observed call encloses what the last one left")
		y := b.Let("y", code.C("doc.examinedEnd", g))
		b.Assert(code.Eq(y, i(1)), "a call that read nothing examined up to its position")
		b.Assert(code.Eq(high(), i(9)), "and leaves the enclosing examined end as it was")
	})

	add("doc_examined_observed", observed, func(b *code.Builder) {
		b.SetReg(exec.Pos, i(4))
		h := b.Let("h", code.C("doc.examinedBegin"))
		b.Do("doc.touch", i(7))
		x := b.Let("x", code.C("doc.examinedEnd", h))
		b.Assert(code.Eq(x, i(7)), "an observed parse from scratch measures the examined end as a Document parse does")
	})
	return cs
}

// RandomDoc returns a randomized case of n calls of the doc operations from opNames, drawn with rng,
// that meets every precondition. Positions come from a small domain, so that ranges overlap often.
func RandomDoc(name string, rng *rand.Rand, opNames []string, n int, cfg ref.Config) Case {
	m := DocModule()
	mirror := ref.New(cfg)
	regs := map[string]any{"pos": int64(0)}
	mirror.UseRegisters(regs)
	type pair2 struct {
		name      string
		high, low int64
		hasLow    bool
	}
	var enters, begins []pair2
	f := m.AddFunc(name, nil, nil, docRegs(), func(b *code.Builder) {
		calls := 0
		for i := 0; calls < n && i < 50*n; i++ {
			o := m.Op(opNames[rng.IntN(len(opNames))])
			var args []any
			var exprs []code.Expr
			pos := func() { v := rng.Int64N(12); args = append(args, v); exprs = append(exprs, code.Int(v)) }
			switch o.Name {
			case "doc.enter", "doc.touch", "doc.touchFrom":
				pos()
			case "doc.leave":
				if len(enters) == 0 {
					continue
				}
				x := enters[len(enters)-1]
				args = []any{x.high, x.low}
				exprs = []code.Expr{code.L(x.name + "h"), code.L(x.name + "l")}
			case "doc.examinedEnd":
				if len(begins) == 0 {
					continue
				}
				x := begins[len(begins)-1]
				args, exprs = []any{x.high}, []code.Expr{code.L(x.name)}
			case "doc.examinedBegin":
				// The position register is the position of the call.
				v := rng.Int64N(12)
				regs["pos"] = v
				b.SetReg(exec.Pos, code.Int(v))
			}
			if !mirror.Allows(o, args) {
				continue
			}
			res := mirror.Call(o, args)
			calls++
			v := fmt.Sprintf("x%d", calls)
			switch o.Name {
			case "doc.enter":
				b.LetN([]string{v + "h", v + "l"}, code.C(o.Name, exprs...))
				enters = append(enters, pair2{name: v, high: res[0].(int64), low: res[1].(int64)})
			case "doc.leave":
				enters = enters[:len(enters)-1]
				b.Do(o.Name, exprs...)
			case "doc.examinedBegin":
				b.Let(v, code.C(o.Name))
				begins = append(begins, pair2{name: v, high: res[0].(int64)})
			case "doc.examinedEnd":
				begins = begins[:len(begins)-1]
				b.Let(v, code.C(o.Name, exprs...))
			default:
				if len(o.Results) > 0 {
					b.Let(v, code.C(o.Name, exprs...))
				} else {
					b.Do(o.Name, exprs...)
				}
			}
			if o.Name == "doc.reused" || o.Name == "doc.evaluated" {
				b.Let(v+"s", code.C(DocStatOp, code.Int(rng.Int64N(2))))
			}
		}
	})
	return Case{Module: m, Func: f, Config: cfg}
}
