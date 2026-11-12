package conform

import (
	"fmt"
	"math/rand/v2"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
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
	"doc.examinedBegin", "doc.examinedEnd", "doc.enter", "doc.leave", "doc.active", "doc.touch",
	"doc.touchFrom", "doc.low", "doc.high", "doc.reused", "doc.evaluated",
}

// DocCoreOps are the doc operations of a target that does not observe calls: all but
// doc.examinedBegin and doc.examinedEnd.
var DocCoreOps = DocOps[2:]

// DocModule returns the declarations of Module with those of the transitional operations, which
// hold the doc operations, and DocStatOp.
func DocModule() *exec.Module {
	m := Module()
	ops.Transitional(m)
	m.AddOp(&exec.Op{Name: DocStatOp, Component: ops.Call, Params: []exec.Param{exec.P("i", exec.TInt)},
		Results:  []exec.Type{exec.TInt},
		Contract: "Returns the statistic i of the Document: the memo entries reused for 0, the rule bodies evaluated for 1."})
	return m
}

// docInt is an argument or result of a doc operation: a position of the input.
func docRegs() []string { return []string{"budget", "stream", "document"} }

// DocCases returns the handwritten conformance cases of the doc operations, in one module. A case
// does not tell where doc.enter starts the examined range of a call, the empty range or one unit,
// which targets differ in (ref.Config.DocEmptyEnter); it asserts what holds for both.
func DocCases() []Case {
	m := DocModule()
	var cs []Case
	add := func(name string, cfg ref.Config, body func(b *exec.Builder)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, docRegs(), body), Config: cfg})
	}
	doc := ref.Config{Document: true}
	low := func() exec.Expr { return exec.C("doc.low") }
	high := func() exec.Expr { return exec.C("doc.high") }
	i := exec.Int
	// starts asserts the range of a call that began at pos, in either target.
	starts := func(b *exec.Builder, pos int64, msg string) {
		b.Assert(exec.And(exec.Eq(low(), i(pos)), exec.And(exec.Bin(">=", high(), i(pos)), exec.Bin("<=", high(), i(pos+1)))), msg)
	}

	add("doc_active", ref.Config{}, func(b *exec.Builder) {
		b.Assert(exec.Not(exec.C("doc.active")), "a parse from scratch reads no Document")
		b.Do("doc.touch", i(9))
		b.Do("doc.touchFrom", i(0))
		b.Assert(exec.Not(exec.C("doc.active")), "touching the input does not make a Document")
	})
	add("doc_active_document", doc, func(b *exec.Builder) {
		b.Assert(exec.C("doc.active"), "a Document parse reads a Document")
	})

	add("doc_enter_leave", doc, func(b *exec.Builder) {
		r := b.LetN([]string{"h0", "l0"}, exec.C("doc.enter", i(5)))
		b.Assert(exec.And(exec.Eq(r[0], i(0)), exec.Eq(r[1], i(0))), "the first call encloses the empty range at 0")
		starts(b, 5, "a call starts its examined input at its position")
		b.Do("doc.touch", i(8))
		b.Assert(exec.And(exec.Eq(low(), i(5)), exec.Eq(high(), i(8))), "reading extends the frontier")
		b.Do("doc.touch", i(7))
		b.Assert(exec.Eq(high(), i(8)), "a frontier does not move back")
		b.Do("doc.touchFrom", i(3))
		b.Assert(exec.And(exec.Eq(low(), i(3)), exec.Eq(high(), i(8))), "examining before the start extends the start back")
		b.Do("doc.touchFrom", i(4))
		b.Assert(exec.Eq(low(), i(3)), "a start does not move forward")
		n := b.LetN([]string{"h1", "l1"}, exec.C("doc.enter", i(6)))
		b.Assert(exec.And(exec.Eq(n[0], i(8)), exec.Eq(n[1], i(3))), "a nested call encloses the range of its parent")
		starts(b, 6, "and starts its own at its position")
		b.Do("doc.touch", i(7))
		b.Do("doc.leave", n[0], n[1])
		b.Assert(exec.And(exec.Eq(low(), i(3)), exec.Eq(high(), i(8))), "the parent's range holds its own and the nested call's")
		m := b.LetN([]string{"h2", "l2"}, exec.C("doc.enter", i(1)))
		b.Do("doc.touch", i(12))
		b.Do("doc.leave", m[0], m[1])
		b.Assert(exec.And(exec.Eq(low(), i(1)), exec.Eq(high(), i(12))), "a nested call that reads farther widens the range at both ends")
		b.Do("doc.leave", r[0], r[1])
		b.Assert(exec.And(exec.Eq(low(), i(0)), exec.Eq(high(), i(12))), "the range of the outermost call merges into the enclosing empty range")
	})

	add("doc_leave_keeps_enclosing", doc, func(b *exec.Builder) {
		b.Do("doc.touch", i(20))
		b.Do("doc.touchFrom", i(0))
		r := b.LetN([]string{"h", "l"}, exec.C("doc.enter", i(5)))
		b.Assert(exec.And(exec.Eq(r[0], i(20)), exec.Eq(r[1], i(0))), "a call encloses the range before it")
		b.Do("doc.leave", r[0], r[1])
		b.Assert(exec.And(exec.Eq(low(), i(0)), exec.Eq(high(), i(20))), "a call that stays inside the enclosing range leaves it as it is")
	})

	add("doc_statistics", doc, func(b *exec.Builder) {
		stat := func(n int64) exec.Expr { return exec.C(DocStatOp, i(n)) }
		b.Assert(exec.And(exec.Eq(stat(0), i(0)), exec.Eq(stat(1), i(0))), "a Document starts with no reuse and no evaluation")
		b.Do("doc.reused")
		b.Do("doc.reused")
		b.Do("doc.evaluated")
		b.Assert(exec.And(exec.Eq(stat(0), i(2)), exec.Eq(stat(1), i(1))), "each count is its own")
	})

	add("doc_examined", doc, func(b *exec.Builder) {
		b.Do("doc.touch", i(3))
		b.SetReg(ops.Pos, i(4))
		h := b.Let("h", exec.C("doc.examinedBegin"))
		b.Assert(exec.Eq(h, i(3)), "an observed call encloses the frontier before it")
		b.Assert(exec.Eq(high(), i(4)), "its own frontier starts at the position")
		b.Do("doc.touch", i(9))
		r := b.LetN([]string{"x", "known"}, exec.C("doc.examinedEnd", h))
		b.Assert(exec.And(r[1], exec.Eq(r[0], i(9))), "it returns its frontier, known in a Document parse")
		b.Assert(exec.Eq(high(), i(9)), "and merges it into the enclosing one")
		b.SetReg(ops.Pos, i(1))
		g := b.Let("g", exec.C("doc.examinedBegin"))
		b.Assert(exec.Eq(g, i(9)), "the next observed call encloses what the last one left")
		r2 := b.LetN([]string{"y", "known2"}, exec.C("doc.examinedEnd", g))
		b.Assert(exec.And(r2[1], exec.Eq(r2[0], i(1))), "a call that read nothing examined up to its position")
		b.Assert(exec.Eq(high(), i(9)), "and leaves the enclosing frontier as it was")
	})

	add("doc_examined_no_document", ref.Config{}, func(b *exec.Builder) {
		b.SetReg(ops.Pos, i(4))
		h := b.Let("h", exec.C("doc.examinedBegin"))
		r := b.LetN([]string{"x", "known"}, exec.C("doc.examinedEnd", h))
		b.Assert(exec.Not(r[1]), "outside a Document parse the frontier is unknown")
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
	f := m.AddFunc(name, nil, nil, docRegs(), func(b *exec.Builder) {
		calls := 0
		for i := 0; calls < n && i < 50*n; i++ {
			o := m.Op(opNames[rng.IntN(len(opNames))])
			var args []any
			var exprs []exec.Expr
			pos := func() { v := rng.Int64N(12); args = append(args, v); exprs = append(exprs, exec.Int(v)) }
			switch o.Name {
			case "doc.enter", "doc.touch", "doc.touchFrom":
				pos()
			case "doc.leave":
				if len(enters) == 0 {
					continue
				}
				x := enters[len(enters)-1]
				args = []any{x.high, x.low}
				exprs = []exec.Expr{exec.L(x.name + "h"), exec.L(x.name + "l")}
			case "doc.examinedEnd":
				if len(begins) == 0 {
					continue
				}
				x := begins[len(begins)-1]
				args, exprs = []any{x.high}, []exec.Expr{exec.L(x.name)}
			case "doc.examinedBegin":
				// The position register is the position of the call.
				v := rng.Int64N(12)
				regs["pos"] = v
				b.SetReg(ops.Pos, exec.Int(v))
			}
			if !mirror.Allows(o, args) {
				continue
			}
			res := mirror.Call(o, args)
			calls++
			v := fmt.Sprintf("x%d", calls)
			switch o.Name {
			case "doc.enter":
				b.LetN([]string{v + "h", v + "l"}, exec.C(o.Name, exprs...))
				enters = append(enters, pair2{name: v, high: res[0].(int64), low: res[1].(int64)})
			case "doc.leave":
				enters = enters[:len(enters)-1]
				b.Do(o.Name, exprs...)
			case "doc.examinedBegin":
				b.Let(v, exec.C(o.Name))
				begins = append(begins, pair2{name: v, high: res[0].(int64)})
			case "doc.examinedEnd":
				begins = begins[:len(begins)-1]
				b.LetN([]string{v, v + "k"}, exec.C(o.Name, exprs...))
			default:
				if len(o.Results) > 0 {
					b.Let(v, exec.C(o.Name, exprs...))
				} else {
					b.Do(o.Name, exprs...)
				}
			}
			if o.Name == "doc.reused" || o.Name == "doc.evaluated" {
				b.Let(v+"s", exec.C(DocStatOp, exec.Int(rng.Int64N(2))))
			}
		}
	})
	return Case{Module: m, Func: f, Config: cfg}
}
