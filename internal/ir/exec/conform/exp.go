package conform

import (
	"fmt"
	"math/rand/v2"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// Expectations is an Impl that provides the expectation record (component expected).
type Expectations interface {
	// Exp returns the implementation's expectation for the conformance identifier id, at least 0.
	Exp(id int64) any
	// Placeholder returns the implementation's placeholder entry (exp.placeholder).
	Placeholder() any
}

// ExpOp is the operation that conformance cases use to make an expectation from an identifier. It is
// not an operation of the parser VM.
const ExpOp = "test.exp"

// PlaceholderOp is the operation that conformance cases use to name the placeholder entry, which
// exp.placeholder adds and exp.at may return. It is not an operation of the parser VM.
const PlaceholderOp = "test.placeholder"

// ExpOps are the operations of the expectation record, with those of the memo that keep the
// expectations of an entry and the memo operations their cases need.
var ExpOps = []string{
	"exp.isolate", "exp.unisolate", "exp.farthest", "exp.recording", "exp.reset", "exp.fresh",
	"exp.placeholder", "exp.maybe", "exp.count", "exp.at", "exp.add", "exp.setCount", "exp.setAt",
	"exp.keptCount", "exp.keptAt", "exp.empty", "exp.last", "exp.copy",
	"memo.far", "memo.expected", "memo.setExpected",
	"memo.lookup", "memo.store", "memo.examine", "memo.splice",
}

// RecordOps are the operations of the expectation record alone, which randomized cases draw from.
var RecordOps = ExpOps[:18]

// ExpModule returns the declarations of Module with those of the transitional operations, which
// hold the expectation record, and ExpOp and PlaceholderOp.
func ExpModule() *exec.Module {
	m := Module()
	ops.Transitional(m)
	m.AddOp(&exec.Op{Name: ExpOp, Component: ops.Exp, Params: []exec.Param{exec.P("id", exec.TInt)},
		Results:  []exec.Type{exec.THandle(ops.ExpID)},
		Contract: "Returns the expectation of a conformance identifier; equal identifiers are equal expectations."})
	m.AddOp(&exec.Op{Name: PlaceholderOp, Component: ops.Exp, Results: []exec.Type{exec.THandle(ops.ExpID)},
		Contract: "Returns the placeholder entry of a record."})
	return m
}

// mayResults are the operations whose result is a filter: an implementation may answer true where the
// model answers false (exp.maybe: true means that the expectation may be in the record).
var mayResults = map[string]bool{"exp.maybe": true}

// ExpCases returns the handwritten conformance cases of the expectation record, in one module. As
// with Cases, the model checks each case first.
func ExpCases() []Case {
	m := ExpModule()
	var cs []Case
	add := func(name string, cfg ref.Config, body func(b *exec.Builder)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, []string{"budget", "stream", "document"}, body), Config: cfg})
	}
	id := func(n int64) exec.Expr { return exec.C(ExpOp, exec.Int(n)) }
	at := func(i int64) exec.Expr { return exec.C("exp.at", exec.Int(i)) }
	count := func(n int64) exec.Expr { return exec.Eq(exec.C("exp.count"), exec.Int(n)) }
	far := func(n int64) exec.Expr { return exec.Eq(exec.C("exp.farthest"), exec.Int(n)) }
	setAt := func(s string, i int64) exec.Expr { return exec.C("exp.setAt", exec.L(s), exec.Int(i)) }
	keptAt := func(k string, i int64) exec.Expr { return exec.C("exp.keptAt", exec.L(k), exec.Int(i)) }
	setCount := func(s string, n int64) exec.Expr { return exec.Eq(exec.C("exp.setCount", exec.L(s)), exec.Int(n)) }
	keptCount := func(k string, n int64) exec.Expr { return exec.Eq(exec.C("exp.keptCount", exec.L(k)), exec.Int(n)) }

	add("exp_record", ref.Config{}, func(b *exec.Builder) {
		b.Assert(exec.And(far(0), exec.C("exp.fresh")), "a parse starts with an empty record at 0")
		b.Assert(exec.C("exp.recording"), "a parse records expectations unless configured otherwise")
		b.Do("exp.reset", exec.Int(3))
		b.Do("exp.add", id(2))
		b.Do("exp.add", id(1))
		b.Do("exp.add", id(4))
		b.Assert(exec.And(far(3), count(3)), "the record keeps its farthest position and every entry")
		b.Assert(exec.Not(exec.C("exp.fresh")), "a record with entries is not fresh")
		b.Assert(exec.And(exec.Eq(at(0), id(2)), exec.And(exec.Eq(at(1), id(1)), exec.Eq(at(2), id(4)))),
			"entries keep the order they were added in")
		b.Assert(exec.And(exec.C("exp.maybe", id(1)), exec.C("exp.maybe", id(4))), "an entry may be in the record")
		b.Do("exp.add", id(1))
		b.Assert(exec.And(count(4), exec.Eq(at(3), id(1))), "exp.add appends without looking for the entry")
		b.Do("exp.reset", exec.Int(2))
		b.Assert(exec.And(far(2), exec.C("exp.fresh")), "exp.reset sets the position and drops the entries, also backwards")
		b.Do("exp.maybe", id(2))
		b.Do("exp.add", id(3))
		b.Assert(exec.And(count(1), exec.Eq(at(0), id(3))), "a reset record grows from its start")
	})

	add("exp_placeholder", ref.Config{NoExpectations: true}, func(b *exec.Builder) {
		b.Assert(exec.Not(exec.C("exp.recording")), "the parse records no expectations")
		b.Do("exp.reset", exec.Int(5))
		b.Do("exp.placeholder")
		b.Assert(exec.And(count(1), exec.Eq(at(0), exec.C(PlaceholderOp))), "the placeholder is an entry of the record")
		b.Assert(exec.Not(exec.C("exp.fresh")), "a record with the placeholder is not fresh")
		b.Let("m", exec.C("exp.isolate", exec.Int(5)))
		b.Do("exp.placeholder")
		r := b.LetN([]string{"f", "s"}, exec.C("exp.unisolate", exec.L("m")))
		b.Assert(exec.And(exec.Eq(r[0], exec.Int(5)), setCount("s", 1)), "an isolated record returns its placeholder")
		b.Assert(exec.Eq(setAt("s", 0), exec.C(PlaceholderOp)), "as its entry")
		b.Let("k", exec.C("exp.copy", exec.L("s")))
		b.Assert(exec.And(keptCount("k", 1), exec.Eq(keptAt("k", 0), exec.C(PlaceholderOp))), "a kept set keeps the placeholder")
	})

	add("exp_isolate_nested", ref.Config{}, func(b *exec.Builder) {
		b.Do("exp.reset", exec.Int(3))
		b.Do("exp.add", id(1))
		b.Let("m1", exec.C("exp.isolate", exec.Int(7)))
		b.Assert(exec.And(far(7), exec.C("exp.fresh")), "an isolation starts an empty record at its position")
		b.Do("exp.add", id(2))
		b.Let("m2", exec.C("exp.isolate", exec.Int(4)))
		b.Assert(exec.And(far(4), exec.C("exp.fresh")), "a nested isolation starts its own record, also before the farthest one")
		b.Do("exp.add", id(3))
		b.Do("exp.add", id(1))
		r2 := b.LetN([]string{"f2", "s2"}, exec.C("exp.unisolate", exec.L("m2")))
		b.Assert(exec.And(exec.Eq(r2[0], exec.Int(4)), setCount("s2", 2)), "unisolate returns the farthest position and the entries of the ended record")
		b.Assert(exec.And(exec.Eq(setAt("s2", 0), id(3)), exec.Eq(setAt("s2", 1), id(1))), "in order")
		b.Assert(exec.And(far(7), exec.And(count(1), exec.Eq(at(0), id(2)))), "and restores the record set aside")
		b.Let("k", exec.C("exp.copy", exec.L("s2")))
		r1 := b.LetN([]string{"f1", "s1"}, exec.C("exp.unisolate", exec.L("m1")))
		b.Assert(exec.And(exec.Eq(r1[0], exec.Int(7)), exec.And(setCount("s1", 1), exec.Eq(setAt("s1", 0), id(2)))), "the outer isolation ends with its own entries")
		b.Assert(exec.And(far(3), exec.And(count(1), exec.Eq(at(0), id(1)))), "the outermost record is as it was")
		b.Assert(exec.And(keptCount("k", 2), exec.And(exec.Eq(keptAt("k", 0), id(3)), exec.Eq(keptAt("k", 1), id(1)))), "a kept set outlives its record")
	})

	add("exp_isolate_empty", ref.Config{}, func(b *exec.Builder) {
		b.Do("exp.add", id(1))
		b.Let("m", exec.C("exp.isolate", exec.Int(9)))
		b.Do("exp.reset", exec.Int(11))
		r := b.LetN([]string{"f", "s"}, exec.C("exp.unisolate", exec.L("m")))
		b.Assert(exec.And(exec.Eq(r[0], exec.Int(11)), setCount("s", 0)), "an isolated record without entries returns an empty set at its farthest position")
		b.Assert(exec.And(far(0), exec.And(count(1), exec.Eq(at(0), id(1)))), "a reset in an isolation leaves the record set aside")
	})

	add("exp_merge_aliasing", ref.Config{}, func(b *exec.Builder) {
		// The merge of an isolated record into the restored one, entry by entry, as mergeExpected does:
		// each write follows the read of the entry it may overwrite.
		b.Do("exp.add", id(1))
		b.Let("m", exec.C("exp.isolate", exec.Int(0)))
		for _, n := range []int64{2, 3, 4} {
			b.Do("exp.add", id(n))
		}
		b.LetN([]string{"f", "s"}, exec.C("exp.unisolate", exec.L("m")))
		for i := range int64(3) {
			b.Do("exp.add", setAt("s", i))
		}
		b.Assert(count(4), "the merged record holds every entry")
		for i, n := range []int64{1, 2, 3, 4} {
			b.Assert(exec.Eq(at(int64(i)), id(n)), fmt.Sprintf("entry %d is merged in order", i))
		}
		// A merge into a record that a farther position starts anew.
		b.Let("m2", exec.C("exp.isolate", exec.Int(6)))
		b.Do("exp.add", id(5))
		b.Do("exp.add", id(6))
		b.LetN([]string{"f2", "s2"}, exec.C("exp.unisolate", exec.L("m2")))
		b.Do("exp.reset", exec.Int(6))
		b.Do("exp.add", setAt("s2", 0))
		b.Do("exp.add", setAt("s2", 1))
		b.Assert(exec.And(far(6), exec.And(count(2), exec.And(exec.Eq(at(0), id(5)), exec.Eq(at(1), id(6))))), "a merge after a reset")
	})

	add("exp_keep", ref.Config{}, func(b *exec.Builder) {
		b.Let("e", exec.C("exp.empty"))
		b.Assert(keptCount("e", 0), "the empty set has no expectations")
		b.Assert(exec.Eq(exec.C("exp.last"), exec.L("e")), "the set kept last is the empty set before the first copy")
		b.Let("m", exec.C("exp.isolate", exec.Int(2)))
		b.Do("exp.add", id(1))
		b.Do("exp.add", id(2))
		b.LetN([]string{"f", "s"}, exec.C("exp.unisolate", exec.L("m")))
		b.Let("k", exec.C("exp.copy", exec.L("s")))
		b.Assert(exec.And(keptCount("k", 2), exec.And(exec.Eq(keptAt("k", 0), id(1)), exec.Eq(keptAt("k", 1), id(2)))), "a copy keeps the entries in order")
		b.Assert(exec.Eq(exec.C("exp.last"), exec.L("k")), "a copy becomes the set kept last")
		b.Let("k2", exec.C("exp.copy", exec.L("s")))
		b.Assert(exec.And(exec.Not(exec.Eq(exec.L("k2"), exec.L("k"))), exec.Eq(exec.C("exp.last"), exec.L("k2"))), "every copy is a new set")
		b.Do("exp.add", id(3))
		b.Do("exp.reset", exec.Int(8))
		b.Do("exp.add", id(4))
		b.Assert(exec.And(keptCount("k", 2), exec.And(exec.Eq(keptAt("k", 0), id(1)), exec.Eq(keptAt("k", 1), id(2)))), "a kept set does not change with the record")
		b.Assert(exec.Eq(exec.C("exp.empty"), exec.L("e")), "the empty set is one set")
		b.Let("m3", exec.C("exp.isolate", exec.Int(8)))
		b.Do("exp.add", id(5))
		b.LetN([]string{"f3", "s3"}, exec.C("exp.unisolate", exec.L("m3")))
		b.Let("k3", exec.C("exp.copy", exec.L("s3")))
		b.Assert(exec.And(keptCount("k3", 1), exec.Eq(keptAt("k3", 0), id(5))), "a copy of other entries")
		b.Assert(exec.And(exec.Eq(keptAt("k", 0), id(1)), exec.Eq(keptAt("k", 1), id(2))), "leaves the sets kept before it")
	})

	add("exp_memo_entry", ref.Config{}, func(b *exec.Builder) {
		k := []exec.Expr{exec.Int(0), exec.Int(5), exec.Int(0), exec.C(ValuesOp, exec.Int(0))}
		b.Let("m", exec.C("exp.isolate", exec.Int(5)))
		b.Do("exp.add", id(1))
		b.LetN([]string{"f", "s"}, exec.C("exp.unisolate", exec.L("m")))
		b.Let("kept", exec.C("exp.copy", exec.L("s")))
		e := b.Let("e", exec.C("memo.store", append(append([]exec.Expr{}, k...), exec.BoolC(true), exec.Int(7), exec.BoolC(false), noValue, noErrs, noDocOnly)...))
		b.Do("memo.setExpected", e, exec.Int(9), exec.L("kept"))
		b.Assert(exec.And(exec.Eq(exec.C("memo.far", e), exec.Int(9)), exec.Eq(exec.C("memo.expected", e), exec.L("kept"))), "an entry keeps its expectations")
		b.Do("memo.setExpected", e, exec.Int(8), exec.C("exp.empty"))
		b.Assert(exec.And(exec.Eq(exec.C("memo.far", e), exec.Int(8)), exec.Eq(exec.C("memo.expected", e), exec.C("exp.empty"))), "a later record replaces them")
		b.Do("memo.setExpected", e, exec.Int(9), exec.L("kept"))
		// An edit before the entry moves it, and the far position of its expectations with it.
		b.Do("memo.examine", e, exec.BoolC(false), exec.Int(10))
		b.Do("memo.splice", exec.Int(1), exec.Int(2), exec.Int(3))
		r := b.LetN([]string{"moved", "found"}, exec.C("memo.lookup", exec.Int(0), exec.Int(8), exec.Int(0), exec.C(ValuesOp, exec.Int(0)), noDocOnly))
		b.Assert(r[1], "the edit moves the entry")
		b.Assert(exec.And(exec.Eq(exec.C("memo.far", r[0]), exec.Int(12)), exec.Eq(exec.C("memo.expected", r[0]), exec.L("kept"))), "with the far position of its expectations")
	})
	return cs
}

// RandomExp returns a randomized case of n calls of the operations of the expectation record from
// opNames, drawn with rng, that meets every precondition. Identifiers and positions come from small
// domains, so that entries repeat and records start anew often.
func RandomExp(name string, rng *rand.Rand, opNames []string, n int, cfg ref.Config) Case {
	m := ExpModule()
	mirror := ref.New(cfg)
	type named[T any] struct {
		name string
		ref  T
	}
	var marks []named[*ref.ExpMark]
	var sets []named[*ref.ExpSet]
	var kept []named[*ref.ExpKept]
	// ids are expectations that the case read from the record or a set, besides those of ExpOp.
	var ids []named[ref.ExpID]
	f := m.AddFunc(name, nil, nil, []string{"budget", "stream", "document"}, func(b *exec.Builder) {
		calls := 0
		for i := 0; calls < n && i < 50*n; i++ {
			o := m.Op(opNames[rng.IntN(len(opNames))])
			var args []any
			var exprs []exec.Expr
			intArg := func(v int64) { args = append(args, v); exprs = append(exprs, exec.Int(v)) }
			idArg := func() {
				if len(ids) > 0 && rng.IntN(4) == 0 {
					x := ids[rng.IntN(len(ids))]
					args, exprs = append(args, x.ref), append(exprs, exec.L(x.name))
					return
				}
				v := rng.Int64N(6)
				args, exprs = append(args, ref.ExpID{ID: v}), append(exprs, exec.C(ExpOp, exec.Int(v)))
			}
			switch o.Name {
			case "exp.isolate", "exp.reset":
				intArg(rng.Int64N(6))
			case "exp.unisolate":
				if len(marks) == 0 {
					continue
				}
				// Mostly the innermost isolation, which the precondition requires.
				x := marks[len(marks)-1]
				if rng.IntN(8) == 0 {
					x = marks[rng.IntN(len(marks))]
				}
				args, exprs = append(args, x.ref), append(exprs, exec.L(x.name))
			case "exp.maybe", "exp.add":
				idArg()
			case "exp.at":
				intArg(rng.Int64N(4))
			case "exp.setCount", "exp.setAt", "exp.copy":
				if len(sets) == 0 {
					continue
				}
				x := sets[rng.IntN(len(sets))]
				args, exprs = append(args, x.ref), append(exprs, exec.L(x.name))
				if o.Name == "exp.setAt" {
					intArg(rng.Int64N(max(mirror.Call(m.Op("exp.setCount"), args)[0].(int64), 1)))
				}
			case "exp.keptCount", "exp.keptAt":
				if len(kept) == 0 {
					continue
				}
				x := kept[rng.IntN(len(kept))]
				args, exprs = append(args, x.ref), append(exprs, exec.L(x.name))
				if o.Name == "exp.keptAt" {
					intArg(rng.Int64N(max(mirror.Call(m.Op("exp.keptCount"), args)[0].(int64), 1)))
				}
			case "exp.placeholder":
				// The placeholder belongs to a parse without expectations, once per record; the contract
				// allows it anywhere, and cases draw it in both kinds of parse.
			}
			if !mirror.Allows(o, args) {
				continue
			}
			res, stop := mirrorCall(mirror, o, args)
			calls++
			v := fmt.Sprintf("x%d", calls)
			switch o.Name {
			case "exp.isolate":
				b.Let(v, exec.C(o.Name, exprs...))
				marks = append(marks, named[*ref.ExpMark]{v, res[0].(*ref.ExpMark)})
			case "exp.unisolate":
				b.LetN([]string{v + "f", v}, exec.C(o.Name, exprs...))
				marks = marks[:len(marks)-1]
				x := res[1].(*ref.ExpSet)
				sets = append(sets, named[*ref.ExpSet]{v, x})
				// Half of the sets are kept at once, as a memo entry keeps them, and half are merged into
				// the restored record, entry by entry, as mergeExpected merges them; the others stay for
				// later calls.
				if len(x.IDs()) > 0 && rng.IntN(2) == 0 {
					k := mirror.Call(m.Op("exp.copy"), []any{x})[0].(*ref.ExpKept)
					b.Let(v+"k", exec.C("exp.copy", exec.L(v)))
					kept = append(kept, named[*ref.ExpKept]{v + "k", k})
				}
				if rng.IntN(2) == 0 {
					setAt, add := m.Op("exp.setAt"), m.Op("exp.add")
					for j := range mirror.Call(m.Op("exp.setCount"), []any{x})[0].(int64) {
						id := mirror.Call(setAt, []any{x, j})[0]
						mirror.Call(add, []any{id})
						b.Do("exp.add", exec.C("exp.setAt", exec.L(v), exec.Int(j)))
					}
				}
			case "exp.copy", "exp.empty", "exp.last":
				b.Let(v, exec.C(o.Name, exprs...))
				kept = append(kept, named[*ref.ExpKept]{v, res[0].(*ref.ExpKept)})
			case "exp.at", "exp.setAt", "exp.keptAt":
				b.Let(v, exec.C(o.Name, exprs...))
				ids = append(ids, named[ref.ExpID]{v, res[0].(ref.ExpID)})
			default:
				if len(o.Results) > 0 {
					b.Let(v, exec.C(o.Name, exprs...))
				} else {
					b.Do(o.Name, exprs...)
				}
			}
			if stop {
				return
			}
		}
	})
	return Case{Module: m, Func: f, Config: cfg}
}
