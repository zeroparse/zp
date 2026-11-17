package conform

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/exec"
	"zeroparse.dev/zp/internal/ir/code/exec/ref"
)

// noValue and noErrs are the value and the recovered errors that cases store in entries: the
// harness compares handles only by identity, and the model passes them through.
var noValue, noErrs = code.Null(code.THandle(exec.Value)), code.Null(code.THandle(exec.Errs))

// noDocOnly is the docOnly argument of memo.lookup and memo.store for a rule that a parse from scratch
// memoizes too. The cases leave the rules that only a Document memoizes to the tests of the budget.
var noDocOnly = code.BoolC(false)

// Cases returns the handwritten conformance cases of the operations of a memoized rule call, in one module. Each case
// states the behavior it checks; the model checks it too, so a case that is wrong about the
// contract fails on the model before it judges an implementation.
func Cases() []Case {
	m := Module()
	var cs []Case
	add := func(name string, cfg ref.Config, body func(b *code.Builder)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, []string{"budget", "stream", "document"}, body), Config: cfg})
	}
	vals := func(id int64) code.Expr { return code.C(ValuesOp, code.Int(id)) }
	key := func(rule, pos, level, id int64) []code.Expr {
		return []code.Expr{code.Int(rule), code.Int(pos), code.Int(level), vals(id)}
	}
	store := func(b *code.Builder, name string, k []code.Expr, ok bool, end int64) code.Expr {
		return b.Let(name, code.C("memo.store", append(slices.Clone(k), code.BoolC(ok), code.Int(end), code.BoolC(!ok), noValue, noErrs, noDocOnly)...))
	}
	lookup := func(b *code.Builder, e, found string, k []code.Expr) (code.Expr, code.Expr) {
		r := b.LetN([]string{e, found}, code.C("memo.lookup", append(slices.Clone(k), noDocOnly)...))
		return r[0], r[1]
	}

	add("memo_store_lookup", ref.Config{}, func(b *code.Builder) {
		k := key(1, 5, 0, 7)
		_, found := lookup(b, "none", "f0", k)
		b.Assert(code.Not(found), "an empty memo has no entry")
		e := store(b, "e", k, true, 9)
		got, found := lookup(b, "got", "f1", k)
		b.Assert(code.And(found, code.Eq(got, e)), "lookup returns the stored entry")
		b.Assert(code.And(code.C("memo.ok", got), code.Eq(code.C("memo.end", got), code.Int(9))), "the entry keeps its outcome and end")
		b.Assert(code.Not(code.C("memo.silent", got)), "the entry keeps whether it is silent")
		for i, other := range [][]code.Expr{key(1, 5, 0, 8), key(2, 5, 0, 7), key(1, 5, 1, 7), key(1, 6, 0, 7)} {
			_, f := lookup(b, fmt.Sprint("o", i), fmt.Sprint("of", i), other)
			b.Assert(code.Not(f), "keys differing in values, rule, level or position are distinct")
		}
		e2 := store(b, "e2", k, false, 5)
		got2, _ := lookup(b, "got2", "f2", k)
		b.Assert(code.And(code.Eq(got2, e2), code.Not(code.C("memo.ok", got2))), "a store replaces the entry under the same key")
		b.Assert(code.C("memo.silent", got2), "with its silent attribute")
	})

	add("memo_many_positions", ref.Config{}, func(b *code.Builder) {
		b.Let("pos", code.Int(0))
		b.Loop("fill", func(b *code.Builder) {
			b.If(code.Bin(">=", code.L("pos"), code.Int(300)), func(b *code.Builder) { b.Break("fill") })
			for rule := int64(0); rule < 3; rule++ {
				b.Do("memo.store", code.Int(rule), code.L("pos"), code.Int(0), vals(0), code.BoolC(true), code.Add(code.L("pos"), code.Int(rule)),
					code.BoolC(false), noValue, noErrs, noDocOnly)
			}
			b.Set("pos", code.Add(code.L("pos"), code.Int(7)))
		})
		b.Set("pos", code.Int(0))
		b.Loop("check", func(b *code.Builder) {
			b.If(code.Bin(">=", code.L("pos"), code.Int(300)), func(b *code.Builder) { b.Break("check") })
			r := b.LetN([]string{"e", "f"}, code.C("memo.lookup", code.Int(2), code.L("pos"), code.Int(0), vals(0), noDocOnly))
			b.Assert(code.And(r[1], code.Eq(code.C("memo.end", r[0]), code.Add(code.L("pos"), code.Int(2)))), "every stored position keeps its entry")
			b.Set("pos", code.Add(code.L("pos"), code.Int(7)))
		})
	})

	add("memo_prune", ref.Config{}, func(b *code.Builder) {
		b.Assert(code.Eq(code.C("memo.floor"), code.Int(0)), "the floor starts at 0")
		for _, pos := range []int64{0, 3, 4, 5, 1100} {
			store(b, fmt.Sprint("p", pos), key(1, pos, 0, 7), true, pos+2)
		}
		b.Do("memo.prune", code.Int(4))
		b.Assert(code.Eq(code.C("memo.floor"), code.Int(4)), "a prune raises the floor")
		for _, pos := range []int64{0, 3} {
			_, f := lookup(b, fmt.Sprint("gone", pos), fmt.Sprint("gf", pos), key(1, pos, 0, 7))
			b.Assert(code.Not(f), "entries before the floor are gone")
		}
		for _, pos := range []int64{4, 5, 1100} {
			e, f := lookup(b, fmt.Sprint("kept", pos), fmt.Sprint("kf", pos), key(1, pos, 0, 7))
			b.Assert(code.And(f, code.Eq(code.C("memo.end", e), code.Int(pos+2))), "entries at or after the floor are unchanged")
		}
		store(b, "below", key(1, 2, 0, 7), true, 9)
		_, f := lookup(b, "belowGot", "bf", key(1, 2, 0, 7))
		b.Assert(code.Not(f), "a store before the floor keeps nothing")
		store(b, "after", key(2, 1200, 0, 7), true, 1200)
		e, f := lookup(b, "afterGot", "af", key(2, 1200, 0, 7))
		b.Assert(code.And(f, code.Eq(e, code.L("after"))), "stores after the floor are kept")
		b.Do("memo.prune", code.Int(1100))
		b.Do("memo.prune", code.Int(1100))
		e, f = lookup(b, "again", "agf", key(1, 1100, 0, 7))
		b.Assert(code.And(f, code.Eq(code.C("memo.end", e), code.Int(1102))), "a prune at the floor keeps the entries at it")
		b.Do("memo.prune", code.Int(5000))
		_, f = lookup(b, "past", "pf", key(2, 1200, 0, 7))
		b.Assert(code.Not(f), "a prune past every entry empties the memo")
		store(b, "fresh", key(0, 5000, 0, 7), true, 5001)
		_, f = lookup(b, "freshGot", "ff", key(0, 5000, 0, 7))
		b.Assert(f, "the memo stores at its floor again")
	})

	add("memo_splice", ref.Config{}, func(b *code.Builder) {
		// Entries of rule 1 at 0, 2, 3, 4, 5 and 6, which examined the input up to 3, 4, 5, 7, 8 and 9,
		// the one at 6 from the unit before it.
		for _, x := range [][3]int64{{0, 3, 0}, {2, 4, 0}, {3, 5, 0}, {4, 7, 0}, {5, 8, 0}, {6, 9, 1}} {
			e := store(b, fmt.Sprint("s", x[0]), key(1, x[0], 0, 7), true, x[0]+1)
			b.Do("memo.examine", e, code.BoolC(x[2] == 1), code.Int(x[1]))
		}
		b.Assert(code.Eq(code.C("memo.examined", code.L("s4")), code.Int(7)), "an entry keeps its examined end")
		b.Assert(code.And(code.C("memo.before", code.L("s6")), code.Not(code.C("memo.before", code.L("s5")))),
			"an entry keeps whether its examined input starts before it")
		// Of rule 4, entries with recovered errors, at 0 before the edit and at 6 after it.
		for _, x := range [][2]int64{{0, 3}, {6, 9}} {
			e := b.Let(fmt.Sprint("er", x[0]), code.C("memo.store", append(key(4, x[0], 0, 7), code.BoolC(true), code.Int(x[0]+1),
				code.BoolC(false), noValue, code.C(ErrsOp), noDocOnly)...))
			b.Do("memo.examine", e, code.BoolC(false), code.Int(x[1]))
		}
		unexamined := store(b, "u", key(2, 1, 0, 7), true, 2)
		b.Assert(code.Eq(code.C("memo.examined", unexamined), code.Int(0)), "an entry without a record has examined end 0")
		b.Assert(code.Not(code.C("memo.before", unexamined)), "an entry without a record does not start before it")
		// Of rule 2, an entry at 5 that examined the unit before it; of rule 3, a pinned entry at 6.
		e := store(b, "b5", key(2, 5, 0, 7), true, 6)
		b.Do("memo.examine", e, code.BoolC(true), code.Int(8))
		e = store(b, "p6", key(3, 6, 0, 7), true, 7)
		b.Do("memo.examine", e, code.BoolC(false), code.Int(9))
		b.Do("memo.pin", e)

		// Replace [4, 5) with three units.
		b.Do("memo.splice", code.Int(4), code.Int(5), code.Int(2))
		for _, x := range [][2]int64{{0, 1}, {2, 3}} {
			e, f := lookup(b, fmt.Sprint("k", x[0]), fmt.Sprint("kf", x[0]), key(1, x[0], 0, 7))
			b.Assert(code.And(f, code.Eq(code.C("memo.end", e), code.Int(x[1]))), "an entry that examined only input before the edit stays")
		}
		b.Assert(code.Eq(code.C("memo.examined", code.L("k2")), code.Int(4)), "a kept entry keeps its examined end")
		for _, pos := range []int64{3, 4, 5, 6} {
			_, f := lookup(b, fmt.Sprint("d", pos), fmt.Sprint("df", pos), key(1, pos, 0, 7))
			b.Assert(code.Not(f), "an entry that examined the edited input is removed, and one after it moves")
		}
		m7, f := lookup(b, "m7", "m7f", key(1, 7, 0, 7))
		b.Assert(code.And(f, code.And(code.Eq(code.C("memo.end", m7), code.Int(8)), code.Eq(code.C("memo.examined", m7), code.Int(10)))),
			"an entry that examined only input after the edit moves with it")
		b.Assert(code.Not(code.C("memo.before", m7)), "a moved entry keeps where its examined input starts")
		m8, f := lookup(b, "m8", "m8f", key(1, 8, 0, 7))
		b.Assert(code.And(f, code.C("memo.before", m8)), "an entry whose examined input starts after the edit moves")
		for _, pos := range []int64{5, 7} {
			_, f := lookup(b, fmt.Sprint("q", pos), fmt.Sprint("qf", pos), key(2, pos, 0, 7))
			b.Assert(code.Not(f), "an entry that examined the unit before it, at the edit's end, is removed")
		}
		for _, pos := range []int64{6, 8} {
			_, f := lookup(b, fmt.Sprint("r", pos), fmt.Sprint("rf", pos), key(3, pos, 0, 7))
			b.Assert(code.Not(f), "a pinned entry is removed")
		}
		e0, f := lookup(b, "x0", "x0f", key(4, 0, 0, 7))
		b.Assert(code.And(f, code.Eq(code.C("memo.end", e0), code.Int(1))), "an entry with recovered errors that examined only input before the edit stays")
		for _, pos := range []int64{6, 8} {
			_, f := lookup(b, fmt.Sprint("x", pos), fmt.Sprint("xf", pos), key(4, pos, 0, 7))
			b.Assert(code.Not(f), "an entry with recovered errors that examined only input after the edit is removed")
		}
		_, f = lookup(b, "ug", "uf", key(2, 1, 0, 7))
		b.Assert(code.Not(f), "an entry without an examined record is removed")

		// Delete [0, 2).
		b.Do("memo.splice", code.Int(0), code.Int(2), code.Int(-2))
		_, f = lookup(b, "z", "zf", key(1, 2, 0, 7))
		b.Assert(code.Not(f), "an entry does not stay at its position when it moves")
		e, f = lookup(b, "z0", "z0f", key(1, 0, 0, 7))
		b.Assert(code.And(f, code.Eq(code.C("memo.end", e), code.Int(1))), "an entry at the end of a deletion moves to its start")
		e, f = lookup(b, "z6", "z6f", key(1, 6, 0, 7))
		b.Assert(code.And(f, code.Eq(code.C("memo.examined", e), code.Int(9))), "a moved entry moves again")

		// Replace the whole input [0, 20) with one unit.
		b.Do("memo.splice", code.Int(0), code.Int(20), code.Int(-19))
		for _, pos := range []int64{0, 5, 6} {
			_, f := lookup(b, fmt.Sprint("w", pos), fmt.Sprint("wf", pos), key(1, pos, 0, 7))
			b.Assert(code.Not(f), "an edit of all the input removes every entry")
		}
		e = store(b, "again", key(1, 0, 0, 7), true, 1)
		got, f := lookup(b, "againGot", "af", key(1, 0, 0, 7))
		b.Assert(code.And(f, code.Eq(got, e)), "the memo stores after an edit")
	})

	add("seen_first_and_repeat", ref.Config{Rules: 3}, func(b *code.Builder) {
		b.Assert(code.C("seen.first", code.Int(0), code.Int(10)), "the first call at a position is deferred")
		b.Assert(code.C("seen.first", code.Int(1), code.Int(10)), "rules are recorded separately")
		b.Assert(code.C("seen.first", code.Int(0), code.Int(11)), "positions are recorded separately")
		b.Assert(code.Not(code.C("seen.first", code.Int(0), code.Int(10))), "a repeated call is memoized")
	})

	add("seen_becomes_eager", ref.Config{Rules: 2}, func(b *code.Builder) {
		// 40 first calls, then repeats until more than calls/16 repeated: the rule turns eager.
		b.Let("pos", code.Int(0))
		b.Loop("firsts", func(b *code.Builder) {
			b.If(code.Bin(">=", code.L("pos"), code.Int(40)), func(b *code.Builder) { b.Break("firsts") })
			b.Assert(code.C("seen.first", code.Int(1), code.L("pos")), "first calls are deferred")
			b.Set("pos", code.Add(code.L("pos"), code.Int(1)))
		})
		b.Assert(code.Not(code.C("seen.first", code.Int(1), code.Int(0))), "repeat 1")
		b.Assert(code.Not(code.C("seen.first", code.Int(1), code.Int(1))), "repeat 2")
		b.Assert(code.Not(code.C("seen.first", code.Int(1), code.Int(2))), "repeat 3: 3 > 43/16, eager")
		b.Assert(code.Not(code.C("seen.first", code.Int(1), code.Int(100))), "an eager rule memoizes a first call")
		b.Assert(code.C("seen.first", code.Int(0), code.Int(100)), "another rule is not eager")
	})

	add("seen_regions", ref.Config{Rules: 130}, func(b *code.Builder) {
		for _, pos := range []int64{0, 63, 64, 1023, 1024, 1025, 5000, 65535} {
			for _, rule := range []int64{0, 64, 129} {
				b.Assert(code.C("seen.first", code.Int(rule), code.Int(pos)), "first call in a region")
			}
		}
		b.Assert(code.Not(code.C("seen.first", code.Int(129), code.Int(1023))), "returning to an earlier region keeps its record")
	})

	add("budget_limit", ref.Config{Limit: 10}, func(b *code.Builder) {
		for i := 0; i < 3; i++ {
			b.Do("budget.charge", code.U64(3))
		}
		b.Assert(code.Eq(code.C("budget.used"), code.U64(9)), "charges add up")
		b.Do("budget.charge", code.U64(1))
		b.Assert(code.Eq(code.C("budget.used"), code.U64(10)), "the limit itself is allowed")
		b.Do("budget.charge", code.U64(1)) // ends the parse
		b.Assert(code.BoolC(false), "unreachable: the charge beyond the limit interrupts")
	})

	add("budget_unlimited", ref.Config{}, func(b *code.Builder) {
		b.Do("budget.charge", code.U64(1<<40))
		b.Do("budget.charge", code.U64(0))
		b.Assert(code.Eq(code.C("budget.used"), code.U64(1<<40)), "no limit: every charge is added")
	})

	addEnvironmentCases(add)
	addRecoveryCases(add)

	add("depth", ref.Config{MaxDepth: 2}, func(b *code.Builder) {
		b.Do("depth.enter")
		b.Do("depth.enter")
		b.Do("depth.exit")
		b.Assert(code.Eq(code.C("depth.current"), code.Int(1)), "enter and exit balance")
		b.Do("depth.enter")
		b.Do("depth.enter") // ends the parse
		b.Assert(code.BoolC(false), "unreachable: the depth beyond the limit interrupts")
	})
	return cs
}

// growthSteps records the errors of two steps of a left-recursion growth as grow.step and grow.end
// state them: each step takes the errors it recovered after the growth's mark as one list and drops
// them when it returns to the start, and the second step replays the seed of the first, the list of
// two errors, before it recovers one more. The end records the last seed's list again. Without the
// truncation of a step (truncate false) the second step holds the seed's list twice, which the case
// asserts against.
func growthSteps(b *code.Builder, push func(*code.Builder, int), mark func(int64) code.Expr, truncate bool) {
	g := b.Let("g", code.C("rec.mark"))
	push(b, 2)
	seed := b.Let("seed", code.C("rec.since", g))
	if truncate {
		b.Do("rec.truncate", g)
	}
	b.Do("rec.add", seed)
	push(b, 1)
	b.Assert(mark(2), "the second step holds the seed's list once and its own error")
	best := b.Let("best", code.C("rec.since", g))
	b.Do("rec.truncate", g)
	b.Do("rec.add", best)
	b.Assert(mark(1), "the end records the last seed's list as one entry")
}

// addRecoveryCases adds the cases of the recovered-error list. The harness compares the list after each
// write, so that the cases need only reach the states.
func addRecoveryCases(add func(string, ref.Config, func(*code.Builder))) {
	next := int64(0)
	push := func(b *code.Builder, n int) {
		for range n {
			next++
			b.Do("rec.push", code.C(ErrOp, code.Int(next)))
		}
	}
	mark := func(n int64) code.Expr { return code.Eq(code.C("rec.mark"), code.Int(n)) }
	add("truncate drops what followed the mark", ref.Config{}, func(b *code.Builder) {
		push(b, 2)
		m := b.Let("m", code.C("rec.mark"))
		push(b, 3)
		b.Do("rec.truncate", m)
		b.Assert(mark(2), "truncate keeps the errors before the mark")
		b.Do("rec.truncate", code.Int(2))
		b.Assert(mark(2), "truncate at the end changes nothing")
		b.Do("rec.truncate", code.Int(0))
		b.Assert(mark(0), "truncate at the start empties the list")
	})
	add("a list survives writes of the list", ref.Config{}, func(b *code.Builder) {
		push(b, 3)
		l := b.Let("l", code.C("rec.since", code.Int(1)))
		b.Assert(mark(2), "since puts the list in place of the entries after the mark")
		b.Do("rec.truncate", code.Int(1))
		push(b, 4)
		b.Do("rec.add", l)
		b.Assert(mark(6), "add records the list as one entry")
		b.Do("rec.add", l)
		b.Assert(mark(7), "a list can be recorded twice")
	})
	add("a list of nothing is empty", ref.Config{}, func(b *code.Builder) {
		push(b, 2)
		l := b.Let("l", code.C("rec.since", code.Int(2)))
		b.Assert(code.C("rec.none", l), "nothing followed the mark")
		b.Assert(mark(2), "since of nothing changes nothing")
		b.Do("rec.add", l)
		b.Assert(mark(2), "adding nothing changes nothing")
		b.Assert(code.Not(code.C("rec.none", code.C("rec.since", code.Int(1)))), "an error followed the mark")
	})
	add("since over one list returns it", ref.Config{}, func(b *code.Builder) {
		push(b, 3)
		l := b.Let("l", code.C("rec.since", code.Int(0)))
		b.Assert(code.Eq(code.C("rec.since", code.Int(0)), l), "since over one list returns that list")
		b.Assert(mark(1), "and leaves the entries as they are")
		b.Do("rec.add", l)
		b.Assert(code.Not(code.Eq(code.C("rec.since", code.Int(0)), l)), "since over two entries makes a list")
		b.Assert(mark(1), "and puts it in their place")
	})
	add("since over one error returns it as the list", ref.Config{}, func(b *code.Builder) {
		push(b, 2)
		l := b.Let("l", code.C("rec.since", code.Int(1)))
		b.Assert(code.Not(code.C("rec.none", l)), "the error followed the mark")
		b.Assert(code.Eq(code.C("rec.since", code.Int(1)), l), "the list of one error is the entry")
		b.Do("rec.add", l)
		b.Assert(mark(3), "add records the list of one error as one entry")
	})
	add("growth steps replace the errors of the step before", ref.Config{}, func(b *code.Builder) {
		growthSteps(b, push, mark, true)
	})
	// A list of an error and a list of two errors has the largest unrolled size of three errors, 5,
	// which the harness checks for the lists that rec.since returns.
	add("a list of an error and a list of two", ref.Config{}, func(b *code.Builder) {
		m0 := b.Let("m0", code.C("rec.mark"))
		push(b, 1)
		m1 := b.Let("m1", code.C("rec.mark"))
		push(b, 2)
		b.Let("inner", code.C("rec.since", m1))
		b.Assert(mark(2), "the list of two is one entry after the error")
		l := b.Let("l", code.C("rec.since", m0))
		b.Assert(mark(1), "the list of three is the one entry")
		b.Do("rec.add", l)
		b.Assert(mark(2), "add records the list of three as one entry")
	})
	// Calls nested d deep, each storing what it recovered and replayed: the innermost recovers an
	// error, and every other only stores the entry after its mark, which is the list of the call it made.
	// The harness checks after each write that listing the errors visits fewer than twice as many lists.
	for _, d := range []int{1, 2, 8} {
		add(fmt.Sprintf("calls %d deep share one list", d), ref.Config{}, func(b *code.Builder) {
			var marks []code.Expr
			for i := range d {
				marks = append(marks, b.Let(fmt.Sprintf("m%d", i), code.C("rec.mark")))
			}
			push(b, 1)
			var l code.Expr
			for i := d - 1; i >= 0; i-- {
				l = b.Let(fmt.Sprintf("l%d", i), code.C("rec.since", marks[i]))
			}
			b.Do("rec.truncate", code.Int(0))
			for range 3 {
				b.Do("rec.add", l)
			}
			b.Assert(mark(3), "each replay is one entry")
		})
	}
	add("replays of replays share their lists", ref.Config{}, func(b *code.Builder) {
		push(b, 2)
		l := b.Let("l", code.C("rec.since", code.Int(0)))
		b.Do("rec.add", l)
		push(b, 1)
		l2 := b.Let("l2", code.C("rec.since", code.Int(0)))
		b.Do("rec.truncate", code.Int(0))
		b.Do("rec.add", l2)
		b.Do("rec.add", l)
		b.Do("rec.add", l2)
		b.Assert(mark(3), "each replay is one entry")
		b.Let("l3", code.C("rec.since", code.Int(0)))
		b.Assert(mark(1), "since puts one list in place of the three")
	})
	// Scratch of at most 16 slots stays for the next element; a larger list is released.
	for _, n := range []int{0, 1, 16, 17, 40} {
		add(fmt.Sprintf("settling %d errors", n), ref.Config{}, func(b *code.Builder) {
			push(b, n)
			b.Do("stream.settleErrors")
			b.Assert(mark(0), "settling forgets the errors")
			push(b, 2)
			b.Do("stream.settleErrors")
			b.Assert(mark(0), "settling forgets the errors of the next element")
		})
	}
	// A list pays for each entry it is made of, so one of many entries reaches the limit inside since.
	add("a list of many entries reaches the limit", ref.Config{Limit: 5}, func(b *code.Builder) {
		push(b, 40)
		b.Let("l", code.C("rec.since", code.Int(0)))
	})
}

// addEnvironmentCases adds the cases of the variable environment, over one vector of a single node
// and vectors with one and two levels above the leaves.
func addEnvironmentCases(add func(string, ref.Config, func(*code.Builder))) {
	datum := func(id int64) code.Expr { return code.C(DatumOp, code.Int(id)) }
	rule := func(i int64) code.Expr { return code.C(RuleOp, code.Int(i)) }
	same := func(a, b code.Expr) code.Expr { return code.Eq(a, b) }
	vars := func(n int) []int64 {
		v := make([]int64, n)
		for i := range v {
			v[i] = int64(i)
		}
		return v
	}
	for _, n := range []int{1, 4, 5, 16, 17, 256, 257} {
		add(fmt.Sprintf("env_scopes_%d", n), ref.Config{Vars: vars(n)}, func(b *code.Builder) {
			last := int64(n - 1)
			at := func(s int64, id int64, msg string) {
				b.Assert(same(code.C("env.lookup", code.Int(s)), datum(id)), msg)
			}
			at(0, 0, "a slot starts with its initial value")
			at(last, last, "the last slot starts with its initial value")
			outer := b.Let("outer", code.C("env.scope"))
			b.Do("env.bind", code.Int(last), datum(500))
			at(last, 500, "a binding changes the slot")
			if n > 1 {
				at(0, 0, "a binding leaves the other slots")
			}
			inner := b.Let("inner", code.C("env.scope"))
			b.Assert(code.Bin("!=", outer, inner), "a binding makes a new environment")
			b.Do("env.bind", code.Int(last), datum(501))
			if n > 1 {
				b.Do("env.bind", code.Int(0), datum(502))
			}
			at(last, 501, "a later binding shadows an earlier one")
			b.Do("env.restore", inner)
			at(last, 500, "a restore brings the inner bindings back")
			if n > 1 {
				at(0, 0, "and drops the later ones")
			}
			b.Do("env.restore", outer)
			at(last, last, "a restore of the first environment drops every binding")
			b.Assert(same(code.C("env.scope"), outer), "and it is that environment")
		})
	}
	add("env_equal_binding", ref.Config{Vars: []int64{7, 1003, 2003, 1000}}, func(b *code.Builder) {
		s := b.Let("s", code.C("env.scope"))
		b.Do("env.bind", code.Int(0), datum(7))
		b.Assert(same(code.C("env.scope"), s), "binding an equal integer keeps the environment")
		b.Do("env.bind", code.Int(1), datum(1003))
		b.Assert(same(code.C("env.scope"), s), "binding an equal string keeps the environment")
		b.Do("env.bind", code.Int(3), datum(1000))
		b.Assert(same(code.C("env.scope"), s), "binding an equal empty string keeps the environment")
		b.Do("env.bind", code.Int(1), datum(2003))
		b.Assert(code.Bin("!=", code.C("env.scope"), s), "a string of the same length that differs is a new value")
		b.Do("env.restore", s)
		b.Do("env.bind", code.Int(1), datum(1004))
		b.Assert(code.Bin("!=", code.C("env.scope"), s), "a string of another length is a new value")
		b.Do("env.restore", s)
		b.Do("env.bind", code.Int(0), datum(1000))
		b.Assert(code.Bin("!=", code.C("env.scope"), s), "a string is not equal to an integer")
	})
	add("env_long_strings", ref.Config{Vars: []int64{1000 + 900, 1000 + 901}}, func(b *code.Builder) {
		b.Do("env.bind", code.Int(0), datum(1000+900))
		b.Do("env.bind", code.Int(1), datum(1000+900))
		b.Do("env.bind", code.Int(1), datum(1000+900))
	})
	keyed := ref.Config{Vars: []int64{1, 2, 1003}, RuleVars: [][]int64{{}, {0}, {2, 0, 1}}}
	add("env_keys", keyed, func(b *code.Builder) {
		memo := func(r int64, key code.Expr) []code.Expr {
			return []code.Expr{code.Int(r), code.Int(5), code.Int(0), key}
		}
		k1 := b.Let("k1", code.C("env.key", rule(2)))
		b.Do("memo.store", append(memo(2, k1), code.BoolC(true), code.Int(9), code.BoolC(false), noValue, noErrs, noDocOnly)...)
		r := b.LetN([]string{"e1", "f1"}, code.C("memo.lookup", append(memo(2, code.C("env.key", rule(2))), noDocOnly)...))
		b.Assert(r[1], "keys of equal values are equal")
		b.Do("env.bind", code.Int(0), datum(40))
		r = b.LetN([]string{"e2", "f2"}, code.C("memo.lookup", append(memo(2, code.C("env.key", rule(2))), noDocOnly)...))
		b.Assert(code.Not(r[1]), "a key of another value differs")
		b.Do("env.bind", code.Int(0), datum(1))
		r = b.LetN([]string{"e3", "f3"}, code.C("memo.lookup", append(memo(2, code.C("env.key", rule(2))), noDocOnly)...))
		b.Assert(r[1], "rebinding the value makes the key equal again, in another environment")
		b.Do("env.bind", code.Int(2), datum(2003))
		r = b.LetN([]string{"e4", "f4"}, code.C("memo.lookup", append(memo(2, code.C("env.key", rule(2))), noDocOnly)...))
		b.Assert(code.Not(r[1]), "a string of the same length that differs makes another key")
		b.Do("env.bind", code.Int(2), datum(1003))
		// The variables the rule does not read leave its key as it is.
		k2 := b.Let("k2", code.C("env.key", rule(1)))
		b.Do("memo.store", append(memo(1, k2), code.BoolC(true), code.Int(9), code.BoolC(false), noValue, noErrs, noDocOnly)...)
		b.Do("env.bind", code.Int(2), datum(1004))
		r = b.LetN([]string{"e5", "f5"}, code.C("memo.lookup", append(memo(1, code.C("env.key", rule(1))), noDocOnly)...))
		b.Assert(r[1], "a binding of another variable keeps the key of a rule")
		k0 := b.Let("k0", code.C("env.key", rule(0)))
		b.Do("memo.store", append(memo(0, k0), code.BoolC(true), code.Int(9), code.BoolC(false), noValue, noErrs, noDocOnly)...)
		b.Do("env.bind", code.Int(1), datum(41))
		r = b.LetN([]string{"e6", "f6"}, code.C("memo.lookup", append(memo(0, code.C("env.key", rule(0))), noDocOnly)...))
		b.Assert(r[1], "a rule that reads no variable has one key")
	})
	wide := ref.Config{Vars: vars(300), RuleVars: [][]int64{{0, 17, 299}, {}}}
	add("env_keys_wide", wide, func(b *code.Builder) {
		b.Do("env.key", rule(0))
		b.Do("env.bind", code.Int(17), datum(900))
		b.Do("env.key", rule(0))
		b.Do("env.key", rule(1))
	})
}

// RandomVars returns the variables of the randomized case with the seed: vectors of a single node,
// with one level and with two levels above the leaves.
func RandomVars(seed int) []int64 {
	v := make([]int64, []int{3, 20, 300}[seed%3])
	for i := range v {
		v[i] = []int64{1, 1002, 1003}[i%3]
	}
	return v
}

// RandomRuleVars are the variables that the rules of a randomized case read.
var RandomRuleVars = [][]int64{{}, {0}, {2, 0, 1}}

// Random returns a randomized case of n operation calls from ops, drawn with rng, that meets every
// precondition. Arguments come from small domains, so that keys, positions and rules collide often,
// and from a few far positions, so that position-indexed structures grow and span regions.
func Random(name string, rng *rand.Rand, opNames []string, n int, cfg ref.Config) Case {
	m := Module()
	mirror := ref.New(cfg)
	type entry struct {
		name string
		ref  *ref.Entry
	}
	var entries []entry
	// keys are the keys that stores used, and those an edit moved them to: when the case edits the
	// memo, lookups draw from them, so that an entry that an edit moved is found again. A case that
	// does not edit draws exactly what it drew before.
	var keys [][4]int64
	edits := slices.Contains(opNames, "memo.splice")
	// scopes are the environments the case took, for env.restore.
	var scopes []string
	var scopeRefs []*ref.Scope
	// lists are the copies of recovered errors the case took, for rec.add and rec.none, and nextErr the
	// identifier of the last error it recorded.
	var lists []string
	var listRefs []*ref.ErrList
	var nextErr int64
	f := m.AddFunc(name, nil, nil, []string{"budget", "stream", "document"}, func(b *code.Builder) {
		pos := func() int64 {
			if rng.IntN(8) == 0 {
				return []int64{1023, 1024, 4096, 70000}[rng.IntN(4)]
			}
			return rng.Int64N(12)
		}
		calls := 0
		for i := 0; calls < n && i < 50*n; i++ {
			o := m.Op(opNames[rng.IntN(len(opNames))])
			var args []any
			var exprs []code.Expr
			intArg := func(v int64) { args = append(args, v); exprs = append(exprs, code.Int(v)) }
			keyArgs := func(withVals bool) {
				intArg(rng.Int64N(3))
				intArg(pos())
				intArg(rng.Int64N(2))
				if withVals {
					id := rng.Int64N(3)
					args = append(args, id)
					exprs = append(exprs, code.C(ValuesOp, code.Int(id)))
				}
			}
			switch o.Name {
			case "memo.lookup":
				keyArgs(true)
				if edits && len(keys) > 0 && rng.IntN(2) == 0 {
					k := keys[rng.IntN(len(keys))]
					args = []any{k[0], k[1], k[2], k[3]}
					exprs = []code.Expr{code.Int(k[0]), code.Int(k[1]), code.Int(k[2]), code.C(ValuesOp, code.Int(k[3]))}
				}
				args, exprs = append(args, false), append(exprs, noDocOnly)
			case "memo.store":
				keyArgs(true)
				ok := rng.IntN(2) == 0
				args, exprs = append(args, ok), append(exprs, code.BoolC(ok))
				intArg(args[1].(int64) + rng.Int64N(4))
				silent := rng.IntN(2) == 0
				args, exprs = append(args, silent, nil), append(exprs, code.BoolC(silent), noValue)
				// An entry with recovered errors is not moved by an edit (memo.splice). One in five has them,
				// chosen without drawing from rng, so that the cases of the other operations stay as they were.
				if (int64(len(entries))+args[1].(int64))%5 == 0 {
					args, exprs = append(args, RecoveredErrors), append(exprs, code.C(ErrsOp))
				} else {
					args, exprs = append(args, nil), append(exprs, noErrs)
				}
				args, exprs = append(args, false), append(exprs, noDocOnly)
			case "memo.ok", "memo.end", "memo.silent":
				if len(entries) == 0 {
					continue
				}
				e := entries[rng.IntN(len(entries))]
				args, exprs = append(args, e.ref), append(exprs, code.L(e.name))
			case "seen.first":
				intArg(rng.Int64N(int64(max(cfg.Rules, 1))))
				intArg(pos())
			case "memo.prune":
				intArg(pos())
			case "memo.splice":
				// Replace [start, end) with up to 3 units.
				start := pos()
				end := start + rng.Int64N(4)
				intArg(start)
				intArg(end)
				intArg(start - end + rng.Int64N(4))
			case "memo.examine", "memo.examined", "memo.before", "memo.pin":
				if len(entries) == 0 {
					continue
				}
				e := entries[rng.IntN(len(entries))]
				args, exprs = append(args, e.ref), append(exprs, code.L(e.name))
				if o.Name == "memo.examine" {
					before := rng.IntN(3) == 0
					args, exprs = append(args, before), append(exprs, code.BoolC(before))
					// An entry that examined no input records the empty range at its position.
					intArg(e.ref.Pos() + rng.Int64N(7))
				}
			case "budget.charge":
				v := rng.Uint64N(8)
				args, exprs = append(args, v), append(exprs, code.U64(v))
			case "env.lookup", "env.bind":
				if len(cfg.Vars) == 0 {
					continue
				}
				intArg(rng.Int64N(int64(len(cfg.Vars))))
				if o.Name == "env.bind" {
					// Few values, so that many bindings are equal to the value they replace.
					id := []int64{1, 2, 3, 1002, 1003, 2003}[rng.IntN(6)]
					args, exprs = append(args, ref.Datum{ID: id}), append(exprs, code.C(DatumOp, code.Int(id)))
				}
			case "env.restore":
				if len(scopes) == 0 {
					continue
				}
				i := rng.IntN(len(scopes))
				args, exprs = append(args, scopeRefs[i]), append(exprs, code.L(scopes[i]))
			case "rec.push":
				nextErr++
				args, exprs = append(args, &ref.Err{ID: nextErr}), append(exprs, code.C(ErrOp, code.Int(nextErr)))
			case "rec.truncate", "rec.since":
				intArg(rng.Int64N(mirror.Entries() + 1))
			case "rec.add", "rec.none":
				if len(lists) == 0 {
					continue
				}
				i := rng.IntN(len(lists))
				args, exprs = append(args, listRefs[i]), append(exprs, code.L(lists[i]))
			case "env.key":
				if len(cfg.RuleVars) == 0 {
					continue
				}
				i := rng.Int64N(int64(len(cfg.RuleVars)))
				args, exprs = append(args, ref.RuleRef{Index: i}), append(exprs, code.C(RuleOp, code.Int(i)))
			}
			if !mirror.Allows(o, args) {
				continue
			}
			res, stop := mirrorCall(mirror, o, args)
			calls++
			switch o.Name {
			case "memo.store":
				keys = append(keys, [4]int64{args[0].(int64), args[1].(int64), args[2].(int64), args[3].(int64)})
			case "memo.splice":
				end, delta := args[1].(int64), args[2].(int64)
				for _, k := range slices.Clone(keys) {
					if to := k[1] + delta; k[1] >= end-1 && to >= 0 {
						keys = append(keys, [4]int64{k[0], to, k[2], k[3]})
					}
				}
				keys = keys[max(0, len(keys)-48):]
			}
			switch {
			case o.Name == "env.scope":
				name := fmt.Sprintf("s%d", calls)
				b.Let(name, code.C(o.Name))
				scopes, scopeRefs = append(scopes, name), append(scopeRefs, res[0].(*ref.Scope))
			case o.Name == "rec.since":
				name := fmt.Sprintf("l%d", calls)
				b.Let(name, code.C(o.Name, exprs...))
				lists, listRefs = append(lists, name), append(listRefs, res[0].(*ref.ErrList))
			case o.Name == "env.lookup" || o.Name == "env.key":
				b.Let(fmt.Sprintf("v%d", calls), code.C(o.Name, exprs...))
			case len(o.Results) > 0 && o.Results[0].Kind == code.KHandle:
				name := fmt.Sprintf("e%d", calls)
				if len(o.Results) == 2 {
					b.LetN([]string{name, name + "f"}, code.C(o.Name, exprs...))
					if !res[1].(bool) {
						continue // an entry that was not found must not be used
					}
				} else {
					b.Let(name, code.C(o.Name, exprs...))
				}
				entries = append(entries, entry{name, res[0].(*ref.Entry)})
			default:
				b.Do(o.Name, exprs...)
			}
			if stop {
				return
			}
		}
	})
	return Case{Module: m, Func: f, Config: cfg}
}

// mirrorCall runs a call on the generator's model and reports whether it ended the parse.
func mirrorCall(m *ref.Model, o *code.Op, args []any) (res []any, stop bool) {
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(code.Interrupt); !ok {
				panic(x)
			}
			stop = true
		}
	}()
	return m.Call(o, args), false
}
