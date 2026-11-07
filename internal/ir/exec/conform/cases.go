package conform

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// noValue and noErrs are the value and the recovered errors that cases store in entries: the
// harness compares handles only by identity, and the model passes them through.
var noValue, noErrs = exec.Null(exec.THandle(ops.Value)), exec.Null(exec.THandle(ops.Errs))

// Cases returns the handwritten conformance cases of the pilot operations, in one module. Each case
// states the behavior it checks; the model checks it too, so a case that is wrong about the
// contract fails on the model before it judges an implementation.
func Cases() []Case {
	m := Module()
	var cs []Case
	add := func(name string, cfg ref.Config, body func(b *exec.Builder)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, []string{"budget", "stream", "document"}, body), Config: cfg})
	}
	vals := func(id int64) exec.Expr { return exec.C(ValuesOp, exec.Int(id)) }
	key := func(rule, pos, level, id int64) []exec.Expr {
		return []exec.Expr{exec.Int(rule), exec.Int(pos), exec.Int(level), vals(id)}
	}
	store := func(b *exec.Builder, name string, k []exec.Expr, ok bool, end int64) exec.Expr {
		return b.Let(name, exec.C("memo.store", append(slices.Clone(k), exec.BoolC(ok), exec.Int(end), exec.BoolC(!ok), noValue, noErrs)...))
	}
	lookup := func(b *exec.Builder, e, found string, k []exec.Expr) (exec.Expr, exec.Expr) {
		r := b.LetN([]string{e, found}, exec.C("memo.lookup", k...))
		return r[0], r[1]
	}

	add("memo_store_lookup", ref.Config{}, func(b *exec.Builder) {
		k := key(1, 5, 0, 7)
		_, found := lookup(b, "none", "f0", k)
		b.Assert(exec.Not(found), "an empty memo has no entry")
		e := store(b, "e", k, true, 9)
		got, found := lookup(b, "got", "f1", k)
		b.Assert(exec.And(found, exec.Eq(got, e)), "lookup returns the stored entry")
		b.Assert(exec.And(exec.C("memo.ok", got), exec.Eq(exec.C("memo.end", got), exec.Int(9))), "the entry keeps its outcome and end")
		b.Assert(exec.Not(exec.C("memo.silent", got)), "the entry keeps whether it is silent")
		for i, other := range [][]exec.Expr{key(1, 5, 0, 8), key(2, 5, 0, 7), key(1, 5, 1, 7), key(1, 6, 0, 7)} {
			_, f := lookup(b, fmt.Sprint("o", i), fmt.Sprint("of", i), other)
			b.Assert(exec.Not(f), "keys differing in values, rule, level or position are distinct")
		}
		e2 := store(b, "e2", k, false, 5)
		got2, _ := lookup(b, "got2", "f2", k)
		b.Assert(exec.And(exec.Eq(got2, e2), exec.Not(exec.C("memo.ok", got2))), "a store replaces the entry under the same key")
		b.Assert(exec.C("memo.silent", got2), "with its silent attribute")
	})

	add("memo_many_positions", ref.Config{}, func(b *exec.Builder) {
		b.Let("pos", exec.Int(0))
		b.Loop("fill", func(b *exec.Builder) {
			b.If(exec.Bin(">=", exec.L("pos"), exec.Int(300)), func(b *exec.Builder) { b.Break("fill") })
			for rule := int64(0); rule < 3; rule++ {
				b.Do("memo.store", exec.Int(rule), exec.L("pos"), exec.Int(0), vals(0), exec.BoolC(true), exec.Add(exec.L("pos"), exec.Int(rule)),
					exec.BoolC(false), noValue, noErrs)
			}
			b.Set("pos", exec.Add(exec.L("pos"), exec.Int(7)))
		})
		b.Set("pos", exec.Int(0))
		b.Loop("check", func(b *exec.Builder) {
			b.If(exec.Bin(">=", exec.L("pos"), exec.Int(300)), func(b *exec.Builder) { b.Break("check") })
			r := b.LetN([]string{"e", "f"}, exec.C("memo.lookup", exec.Int(2), exec.L("pos"), exec.Int(0), vals(0)))
			b.Assert(exec.And(r[1], exec.Eq(exec.C("memo.end", r[0]), exec.Add(exec.L("pos"), exec.Int(2)))), "every stored position keeps its entry")
			b.Set("pos", exec.Add(exec.L("pos"), exec.Int(7)))
		})
	})

	add("memo_prune", ref.Config{}, func(b *exec.Builder) {
		b.Assert(exec.Eq(exec.C("memo.floor"), exec.Int(0)), "the floor starts at 0")
		for _, pos := range []int64{0, 3, 4, 5, 1100} {
			store(b, fmt.Sprint("p", pos), key(1, pos, 0, 7), true, pos+2)
		}
		b.Do("memo.prune", exec.Int(4))
		b.Assert(exec.Eq(exec.C("memo.floor"), exec.Int(4)), "a prune raises the floor")
		for _, pos := range []int64{0, 3} {
			_, f := lookup(b, fmt.Sprint("gone", pos), fmt.Sprint("gf", pos), key(1, pos, 0, 7))
			b.Assert(exec.Not(f), "entries before the floor are gone")
		}
		for _, pos := range []int64{4, 5, 1100} {
			e, f := lookup(b, fmt.Sprint("kept", pos), fmt.Sprint("kf", pos), key(1, pos, 0, 7))
			b.Assert(exec.And(f, exec.Eq(exec.C("memo.end", e), exec.Int(pos+2))), "entries at or after the floor are unchanged")
		}
		store(b, "below", key(1, 2, 0, 7), true, 9)
		_, f := lookup(b, "belowGot", "bf", key(1, 2, 0, 7))
		b.Assert(exec.Not(f), "a store before the floor keeps nothing")
		store(b, "after", key(2, 1200, 0, 7), true, 1200)
		e, f := lookup(b, "afterGot", "af", key(2, 1200, 0, 7))
		b.Assert(exec.And(f, exec.Eq(e, exec.L("after"))), "stores after the floor are kept")
		b.Do("memo.prune", exec.Int(1100))
		b.Do("memo.prune", exec.Int(1100))
		e, f = lookup(b, "again", "agf", key(1, 1100, 0, 7))
		b.Assert(exec.And(f, exec.Eq(exec.C("memo.end", e), exec.Int(1102))), "a prune at the floor keeps the entries at it")
		b.Do("memo.prune", exec.Int(5000))
		_, f = lookup(b, "past", "pf", key(2, 1200, 0, 7))
		b.Assert(exec.Not(f), "a prune past every entry empties the memo")
		store(b, "fresh", key(0, 5000, 0, 7), true, 5001)
		_, f = lookup(b, "freshGot", "ff", key(0, 5000, 0, 7))
		b.Assert(f, "the memo stores at its floor again")
	})

	add("memo_splice", ref.Config{}, func(b *exec.Builder) {
		// Entries of rule 1 at 0, 2, 3, 4, 5 and 6, which examined the input up to 3, 4, 5, 7, 8 and 9,
		// the one at 6 from the unit before it.
		for _, x := range [][3]int64{{0, 3, 0}, {2, 4, 0}, {3, 5, 0}, {4, 7, 0}, {5, 8, 0}, {6, 9, 1}} {
			e := store(b, fmt.Sprint("s", x[0]), key(1, x[0], 0, 7), true, x[0]+1)
			b.Do("memo.examine", e, exec.BoolC(x[2] == 1), exec.Int(x[1]))
		}
		b.Assert(exec.Eq(exec.C("memo.examined", exec.L("s4")), exec.Int(7)), "an entry keeps its examined end")
		b.Assert(exec.And(exec.C("memo.before", exec.L("s6")), exec.Not(exec.C("memo.before", exec.L("s5")))),
			"an entry keeps whether its examined input starts before it")
		// Of rule 4, entries with recovered errors, at 0 before the edit and at 6 after it.
		for _, x := range [][2]int64{{0, 3}, {6, 9}} {
			e := b.Let(fmt.Sprint("er", x[0]), exec.C("memo.store", append(key(4, x[0], 0, 7), exec.BoolC(true), exec.Int(x[0]+1),
				exec.BoolC(false), noValue, exec.C(ErrsOp))...))
			b.Do("memo.examine", e, exec.BoolC(false), exec.Int(x[1]))
		}
		unexamined := store(b, "u", key(2, 1, 0, 7), true, 2)
		b.Assert(exec.Eq(exec.C("memo.examined", unexamined), exec.Int(0)), "an entry without a record has examined end 0")
		b.Assert(exec.Not(exec.C("memo.before", unexamined)), "an entry without a record does not start before it")
		// Of rule 2, an entry at 5 that examined the unit before it; of rule 3, a pinned entry at 6.
		e := store(b, "b5", key(2, 5, 0, 7), true, 6)
		b.Do("memo.examine", e, exec.BoolC(true), exec.Int(8))
		e = store(b, "p6", key(3, 6, 0, 7), true, 7)
		b.Do("memo.examine", e, exec.BoolC(false), exec.Int(9))
		b.Do("memo.pin", e)

		// Replace [4, 5) with three units.
		b.Do("memo.splice", exec.Int(4), exec.Int(5), exec.Int(2))
		for _, x := range [][2]int64{{0, 1}, {2, 3}} {
			e, f := lookup(b, fmt.Sprint("k", x[0]), fmt.Sprint("kf", x[0]), key(1, x[0], 0, 7))
			b.Assert(exec.And(f, exec.Eq(exec.C("memo.end", e), exec.Int(x[1]))), "an entry that examined only input before the edit stays")
		}
		b.Assert(exec.Eq(exec.C("memo.examined", exec.L("k2")), exec.Int(4)), "a kept entry keeps its examined end")
		for _, pos := range []int64{3, 4, 5, 6} {
			_, f := lookup(b, fmt.Sprint("d", pos), fmt.Sprint("df", pos), key(1, pos, 0, 7))
			b.Assert(exec.Not(f), "an entry that examined the edited input is removed, and one after it moves")
		}
		m7, f := lookup(b, "m7", "m7f", key(1, 7, 0, 7))
		b.Assert(exec.And(f, exec.And(exec.Eq(exec.C("memo.end", m7), exec.Int(8)), exec.Eq(exec.C("memo.examined", m7), exec.Int(10)))),
			"an entry that examined only input after the edit moves with it")
		b.Assert(exec.Not(exec.C("memo.before", m7)), "a moved entry keeps where its examined input starts")
		m8, f := lookup(b, "m8", "m8f", key(1, 8, 0, 7))
		b.Assert(exec.And(f, exec.C("memo.before", m8)), "an entry whose examined input starts after the edit moves")
		for _, pos := range []int64{5, 7} {
			_, f := lookup(b, fmt.Sprint("q", pos), fmt.Sprint("qf", pos), key(2, pos, 0, 7))
			b.Assert(exec.Not(f), "an entry that examined the unit before it, at the edit's end, is removed")
		}
		for _, pos := range []int64{6, 8} {
			_, f := lookup(b, fmt.Sprint("r", pos), fmt.Sprint("rf", pos), key(3, pos, 0, 7))
			b.Assert(exec.Not(f), "a pinned entry is removed")
		}
		e0, f := lookup(b, "x0", "x0f", key(4, 0, 0, 7))
		b.Assert(exec.And(f, exec.Eq(exec.C("memo.end", e0), exec.Int(1))), "an entry with recovered errors that examined only input before the edit stays")
		for _, pos := range []int64{6, 8} {
			_, f := lookup(b, fmt.Sprint("x", pos), fmt.Sprint("xf", pos), key(4, pos, 0, 7))
			b.Assert(exec.Not(f), "an entry with recovered errors that examined only input after the edit is removed")
		}
		_, f = lookup(b, "ug", "uf", key(2, 1, 0, 7))
		b.Assert(exec.Not(f), "an entry without an examined record is removed")

		// Delete [0, 2).
		b.Do("memo.splice", exec.Int(0), exec.Int(2), exec.Int(-2))
		_, f = lookup(b, "z", "zf", key(1, 2, 0, 7))
		b.Assert(exec.Not(f), "an entry does not stay at its position when it moves")
		e, f = lookup(b, "z0", "z0f", key(1, 0, 0, 7))
		b.Assert(exec.And(f, exec.Eq(exec.C("memo.end", e), exec.Int(1))), "an entry at the end of a deletion moves to its start")
		e, f = lookup(b, "z6", "z6f", key(1, 6, 0, 7))
		b.Assert(exec.And(f, exec.Eq(exec.C("memo.examined", e), exec.Int(9))), "a moved entry moves again")

		// Replace the whole input [0, 20) with one unit.
		b.Do("memo.splice", exec.Int(0), exec.Int(20), exec.Int(-19))
		for _, pos := range []int64{0, 5, 6} {
			_, f := lookup(b, fmt.Sprint("w", pos), fmt.Sprint("wf", pos), key(1, pos, 0, 7))
			b.Assert(exec.Not(f), "an edit of all the input removes every entry")
		}
		e = store(b, "again", key(1, 0, 0, 7), true, 1)
		got, f := lookup(b, "againGot", "af", key(1, 0, 0, 7))
		b.Assert(exec.And(f, exec.Eq(got, e)), "the memo stores after an edit")
	})

	add("seen_first_and_repeat", ref.Config{Rules: 3}, func(b *exec.Builder) {
		b.Assert(exec.C("seen.first", exec.Int(0), exec.Int(10)), "the first call at a position is deferred")
		b.Assert(exec.C("seen.first", exec.Int(1), exec.Int(10)), "rules are recorded separately")
		b.Assert(exec.C("seen.first", exec.Int(0), exec.Int(11)), "positions are recorded separately")
		b.Assert(exec.Not(exec.C("seen.first", exec.Int(0), exec.Int(10))), "a repeated call is memoized")
	})

	add("seen_becomes_eager", ref.Config{Rules: 2}, func(b *exec.Builder) {
		// 40 first calls, then repeats until more than calls/16 repeated: the rule turns eager.
		b.Let("pos", exec.Int(0))
		b.Loop("firsts", func(b *exec.Builder) {
			b.If(exec.Bin(">=", exec.L("pos"), exec.Int(40)), func(b *exec.Builder) { b.Break("firsts") })
			b.Assert(exec.C("seen.first", exec.Int(1), exec.L("pos")), "first calls are deferred")
			b.Set("pos", exec.Add(exec.L("pos"), exec.Int(1)))
		})
		b.Assert(exec.Not(exec.C("seen.first", exec.Int(1), exec.Int(0))), "repeat 1")
		b.Assert(exec.Not(exec.C("seen.first", exec.Int(1), exec.Int(1))), "repeat 2")
		b.Assert(exec.Not(exec.C("seen.first", exec.Int(1), exec.Int(2))), "repeat 3: 3 > 43/16, eager")
		b.Assert(exec.Not(exec.C("seen.first", exec.Int(1), exec.Int(100))), "an eager rule memoizes a first call")
		b.Assert(exec.C("seen.first", exec.Int(0), exec.Int(100)), "another rule is not eager")
	})

	add("seen_regions", ref.Config{Rules: 130}, func(b *exec.Builder) {
		for _, pos := range []int64{0, 63, 64, 1023, 1024, 1025, 5000, 65535} {
			for _, rule := range []int64{0, 64, 129} {
				b.Assert(exec.C("seen.first", exec.Int(rule), exec.Int(pos)), "first call in a region")
			}
		}
		b.Assert(exec.Not(exec.C("seen.first", exec.Int(129), exec.Int(1023))), "returning to an earlier region keeps its record")
	})

	add("budget_limit", ref.Config{Limit: 10}, func(b *exec.Builder) {
		for i := 0; i < 3; i++ {
			b.Do("budget.charge", exec.U64(3))
		}
		b.Assert(exec.Eq(exec.C("budget.used"), exec.U64(9)), "charges add up")
		b.Do("budget.charge", exec.U64(1))
		b.Assert(exec.Eq(exec.C("budget.used"), exec.U64(10)), "the limit itself is allowed")
		b.Do("budget.charge", exec.U64(1)) // ends the parse
		b.Assert(exec.BoolC(false), "unreachable: the charge beyond the limit interrupts")
	})

	add("budget_unlimited", ref.Config{}, func(b *exec.Builder) {
		b.Do("budget.charge", exec.U64(1<<40))
		b.Do("budget.charge", exec.U64(0))
		b.Assert(exec.Eq(exec.C("budget.used"), exec.U64(1<<40)), "no limit: every charge is added")
	})

	addEnvironmentCases(add)
	addRecoveryCases(add)

	add("depth", ref.Config{MaxDepth: 2}, func(b *exec.Builder) {
		b.Do("depth.enter")
		b.Do("depth.enter")
		b.Do("depth.exit")
		b.Assert(exec.Eq(exec.C("depth.current"), exec.Int(1)), "enter and exit balance")
		b.Do("depth.enter")
		b.Do("depth.enter") // ends the parse
		b.Assert(exec.BoolC(false), "unreachable: the depth beyond the limit interrupts")
	})
	return cs
}

// addRecoveryCases adds the cases of the recovered-error list. The harness compares the list after each
// write, so that the cases need only reach the states.
func addRecoveryCases(add func(string, ref.Config, func(*exec.Builder))) {
	next := int64(0)
	push := func(b *exec.Builder, n int) {
		for range n {
			next++
			b.Do("rec.push", exec.C(ErrOp, exec.Int(next)))
		}
	}
	mark := func(n int64) exec.Expr { return exec.Eq(exec.C("rec.mark"), exec.Int(n)) }
	add("truncate drops what followed the mark", ref.Config{}, func(b *exec.Builder) {
		push(b, 2)
		m := b.Let("m", exec.C("rec.mark"))
		push(b, 3)
		b.Do("rec.truncate", m)
		b.Assert(mark(2), "truncate keeps the errors before the mark")
		b.Do("rec.truncate", exec.Int(2))
		b.Assert(mark(2), "truncate at the end changes nothing")
		b.Do("rec.truncate", exec.Int(0))
		b.Assert(mark(0), "truncate at the start empties the list")
	})
	add("a list survives writes of the list", ref.Config{}, func(b *exec.Builder) {
		push(b, 3)
		l := b.Let("l", exec.C("rec.since", exec.Int(1)))
		b.Assert(mark(2), "since puts the list in place of the entries after the mark")
		b.Do("rec.truncate", exec.Int(1))
		push(b, 4)
		b.Do("rec.add", l)
		b.Assert(mark(6), "add records the list as one entry")
		b.Do("rec.add", l)
		b.Assert(mark(7), "a list can be recorded twice")
	})
	add("a list of nothing is empty", ref.Config{}, func(b *exec.Builder) {
		push(b, 2)
		l := b.Let("l", exec.C("rec.since", exec.Int(2)))
		b.Assert(exec.C("rec.none", l), "nothing followed the mark")
		b.Assert(mark(2), "since of nothing changes nothing")
		b.Do("rec.add", l)
		b.Assert(mark(2), "adding nothing changes nothing")
		b.Assert(exec.Not(exec.C("rec.none", exec.C("rec.since", exec.Int(1)))), "an error followed the mark")
	})
	add("since over one list returns it", ref.Config{}, func(b *exec.Builder) {
		push(b, 3)
		l := b.Let("l", exec.C("rec.since", exec.Int(0)))
		b.Assert(exec.Eq(exec.C("rec.since", exec.Int(0)), l), "since over one list returns that list")
		b.Assert(mark(1), "and leaves the entries as they are")
		b.Do("rec.add", l)
		b.Assert(exec.Not(exec.Eq(exec.C("rec.since", exec.Int(0)), l)), "since over two entries makes a list")
		b.Assert(mark(1), "and puts it in their place")
	})
	add("since over one error returns it as the list", ref.Config{}, func(b *exec.Builder) {
		push(b, 2)
		l := b.Let("l", exec.C("rec.since", exec.Int(1)))
		b.Assert(exec.Not(exec.C("rec.none", l)), "the error followed the mark")
		b.Assert(exec.Eq(exec.C("rec.since", exec.Int(1)), l), "the list of one error is the entry")
		b.Do("rec.add", l)
		b.Assert(mark(3), "add records the list of one error as one entry")
	})
	// A list of an error and a list of two errors has the largest unrolled size of three errors, 5,
	// which the harness checks for the lists that rec.since returns.
	add("a list of an error and a list of two", ref.Config{}, func(b *exec.Builder) {
		m0 := b.Let("m0", exec.C("rec.mark"))
		push(b, 1)
		m1 := b.Let("m1", exec.C("rec.mark"))
		push(b, 2)
		b.Let("inner", exec.C("rec.since", m1))
		b.Assert(mark(2), "the list of two is one entry after the error")
		l := b.Let("l", exec.C("rec.since", m0))
		b.Assert(mark(1), "the list of three is the one entry")
		b.Do("rec.add", l)
		b.Assert(mark(2), "add records the list of three as one entry")
	})
	// Calls nested d deep, each storing what it recovered and replayed: the innermost recovers an
	// error, and every other only stores the entry after its mark, which is the list of the call it made.
	// The harness checks after each write that listing the errors visits fewer than twice as many lists.
	for _, d := range []int{1, 2, 8} {
		add(fmt.Sprintf("calls %d deep share one list", d), ref.Config{}, func(b *exec.Builder) {
			var marks []exec.Expr
			for i := range d {
				marks = append(marks, b.Let(fmt.Sprintf("m%d", i), exec.C("rec.mark")))
			}
			push(b, 1)
			var l exec.Expr
			for i := d - 1; i >= 0; i-- {
				l = b.Let(fmt.Sprintf("l%d", i), exec.C("rec.since", marks[i]))
			}
			b.Do("rec.truncate", exec.Int(0))
			for range 3 {
				b.Do("rec.add", l)
			}
			b.Assert(mark(3), "each replay is one entry")
		})
	}
	add("replays of replays share their lists", ref.Config{}, func(b *exec.Builder) {
		push(b, 2)
		l := b.Let("l", exec.C("rec.since", exec.Int(0)))
		b.Do("rec.add", l)
		push(b, 1)
		l2 := b.Let("l2", exec.C("rec.since", exec.Int(0)))
		b.Do("rec.truncate", exec.Int(0))
		b.Do("rec.add", l2)
		b.Do("rec.add", l)
		b.Do("rec.add", l2)
		b.Assert(mark(3), "each replay is one entry")
		b.Let("l3", exec.C("rec.since", exec.Int(0)))
		b.Assert(mark(1), "since puts one list in place of the three")
	})
	// Scratch of at most 16 slots stays for the next element; a larger list is released.
	for _, n := range []int{0, 1, 16, 17, 40} {
		add(fmt.Sprintf("settling %d errors", n), ref.Config{}, func(b *exec.Builder) {
			push(b, n)
			b.Do("stream.settleErrors")
			b.Assert(mark(0), "settling forgets the errors")
			push(b, 2)
			b.Do("stream.settleErrors")
			b.Assert(mark(0), "settling forgets the errors of the next element")
		})
	}
	// A list pays for each entry it is made of, so one of many entries reaches the limit inside since.
	add("a list of many entries reaches the limit", ref.Config{Limit: 5}, func(b *exec.Builder) {
		push(b, 40)
		b.Let("l", exec.C("rec.since", exec.Int(0)))
	})
}

// addEnvironmentCases adds the cases of the variable environment, over one vector of a single node
// and vectors with one and two levels above the leaves.
func addEnvironmentCases(add func(string, ref.Config, func(*exec.Builder))) {
	datum := func(id int64) exec.Expr { return exec.C(DatumOp, exec.Int(id)) }
	rule := func(i int64) exec.Expr { return exec.C(RuleOp, exec.Int(i)) }
	same := func(a, b exec.Expr) exec.Expr { return exec.Eq(a, b) }
	vars := func(n int) []int64 {
		v := make([]int64, n)
		for i := range v {
			v[i] = int64(i)
		}
		return v
	}
	for _, n := range []int{1, 4, 5, 16, 17, 256, 257} {
		add(fmt.Sprintf("env_scopes_%d", n), ref.Config{Vars: vars(n)}, func(b *exec.Builder) {
			last := int64(n - 1)
			at := func(s int64, id int64, msg string) {
				b.Assert(same(exec.C("env.lookup", exec.Int(s)), datum(id)), msg)
			}
			at(0, 0, "a slot starts with its initial value")
			at(last, last, "the last slot starts with its initial value")
			outer := b.Let("outer", exec.C("env.scope"))
			b.Do("env.bind", exec.Int(last), datum(500))
			at(last, 500, "a binding changes the slot")
			if n > 1 {
				at(0, 0, "a binding leaves the other slots")
			}
			inner := b.Let("inner", exec.C("env.scope"))
			b.Assert(exec.Bin("!=", outer, inner), "a binding makes a new environment")
			b.Do("env.bind", exec.Int(last), datum(501))
			if n > 1 {
				b.Do("env.bind", exec.Int(0), datum(502))
			}
			at(last, 501, "a later binding shadows an earlier one")
			b.Do("env.restore", inner)
			at(last, 500, "a restore brings the inner bindings back")
			if n > 1 {
				at(0, 0, "and drops the later ones")
			}
			b.Do("env.restore", outer)
			at(last, last, "a restore of the first environment drops every binding")
			b.Assert(same(exec.C("env.scope"), outer), "and it is that environment")
		})
	}
	add("env_equal_binding", ref.Config{Vars: []int64{7, 1003, 2003, 1000}}, func(b *exec.Builder) {
		s := b.Let("s", exec.C("env.scope"))
		b.Do("env.bind", exec.Int(0), datum(7))
		b.Assert(same(exec.C("env.scope"), s), "binding an equal integer keeps the environment")
		b.Do("env.bind", exec.Int(1), datum(1003))
		b.Assert(same(exec.C("env.scope"), s), "binding an equal string keeps the environment")
		b.Do("env.bind", exec.Int(3), datum(1000))
		b.Assert(same(exec.C("env.scope"), s), "binding an equal empty string keeps the environment")
		b.Do("env.bind", exec.Int(1), datum(2003))
		b.Assert(exec.Bin("!=", exec.C("env.scope"), s), "a string of the same length that differs is a new value")
		b.Do("env.restore", s)
		b.Do("env.bind", exec.Int(1), datum(1004))
		b.Assert(exec.Bin("!=", exec.C("env.scope"), s), "a string of another length is a new value")
		b.Do("env.restore", s)
		b.Do("env.bind", exec.Int(0), datum(1000))
		b.Assert(exec.Bin("!=", exec.C("env.scope"), s), "a string is not equal to an integer")
	})
	add("env_long_strings", ref.Config{Vars: []int64{1000 + 900, 1000 + 901}}, func(b *exec.Builder) {
		b.Do("env.bind", exec.Int(0), datum(1000+900))
		b.Do("env.bind", exec.Int(1), datum(1000+900))
		b.Do("env.bind", exec.Int(1), datum(1000+900))
	})
	keyed := ref.Config{Vars: []int64{1, 2, 1003}, RuleVars: [][]int64{{}, {0}, {2, 0, 1}}}
	add("env_keys", keyed, func(b *exec.Builder) {
		memo := func(r int64, key exec.Expr) []exec.Expr {
			return []exec.Expr{exec.Int(r), exec.Int(5), exec.Int(0), key}
		}
		k1 := b.Let("k1", exec.C("env.key", rule(2)))
		b.Do("memo.store", append(memo(2, k1), exec.BoolC(true), exec.Int(9), exec.BoolC(false), noValue, noErrs)...)
		r := b.LetN([]string{"e1", "f1"}, exec.C("memo.lookup", memo(2, exec.C("env.key", rule(2)))...))
		b.Assert(r[1], "keys of equal values are equal")
		b.Do("env.bind", exec.Int(0), datum(40))
		r = b.LetN([]string{"e2", "f2"}, exec.C("memo.lookup", memo(2, exec.C("env.key", rule(2)))...))
		b.Assert(exec.Not(r[1]), "a key of another value differs")
		b.Do("env.bind", exec.Int(0), datum(1))
		r = b.LetN([]string{"e3", "f3"}, exec.C("memo.lookup", memo(2, exec.C("env.key", rule(2)))...))
		b.Assert(r[1], "rebinding the value makes the key equal again, in another environment")
		b.Do("env.bind", exec.Int(2), datum(2003))
		r = b.LetN([]string{"e4", "f4"}, exec.C("memo.lookup", memo(2, exec.C("env.key", rule(2)))...))
		b.Assert(exec.Not(r[1]), "a string of the same length that differs makes another key")
		b.Do("env.bind", exec.Int(2), datum(1003))
		// The variables the rule does not read leave its key as it is.
		k2 := b.Let("k2", exec.C("env.key", rule(1)))
		b.Do("memo.store", append(memo(1, k2), exec.BoolC(true), exec.Int(9), exec.BoolC(false), noValue, noErrs)...)
		b.Do("env.bind", exec.Int(2), datum(1004))
		r = b.LetN([]string{"e5", "f5"}, exec.C("memo.lookup", memo(1, exec.C("env.key", rule(1)))...))
		b.Assert(r[1], "a binding of another variable keeps the key of a rule")
		k0 := b.Let("k0", exec.C("env.key", rule(0)))
		b.Do("memo.store", append(memo(0, k0), exec.BoolC(true), exec.Int(9), exec.BoolC(false), noValue, noErrs)...)
		b.Do("env.bind", exec.Int(1), datum(41))
		r = b.LetN([]string{"e6", "f6"}, exec.C("memo.lookup", memo(0, exec.C("env.key", rule(0)))...))
		b.Assert(r[1], "a rule that reads no variable has one key")
	})
	wide := ref.Config{Vars: vars(300), RuleVars: [][]int64{{0, 17, 299}, {}}}
	add("env_keys_wide", wide, func(b *exec.Builder) {
		b.Do("env.key", rule(0))
		b.Do("env.bind", exec.Int(17), datum(900))
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
	f := m.AddFunc(name, nil, nil, []string{"budget", "stream", "document"}, func(b *exec.Builder) {
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
			var exprs []exec.Expr
			intArg := func(v int64) { args = append(args, v); exprs = append(exprs, exec.Int(v)) }
			keyArgs := func(withVals bool) {
				intArg(rng.Int64N(3))
				intArg(pos())
				intArg(rng.Int64N(2))
				if withVals {
					id := rng.Int64N(3)
					args = append(args, id)
					exprs = append(exprs, exec.C(ValuesOp, exec.Int(id)))
				}
			}
			switch o.Name {
			case "memo.lookup":
				keyArgs(true)
				if edits && len(keys) > 0 && rng.IntN(2) == 0 {
					k := keys[rng.IntN(len(keys))]
					args = []any{k[0], k[1], k[2], k[3]}
					exprs = []exec.Expr{exec.Int(k[0]), exec.Int(k[1]), exec.Int(k[2]), exec.C(ValuesOp, exec.Int(k[3]))}
				}
			case "memo.store":
				keyArgs(true)
				ok := rng.IntN(2) == 0
				args, exprs = append(args, ok), append(exprs, exec.BoolC(ok))
				intArg(args[1].(int64) + rng.Int64N(4))
				silent := rng.IntN(2) == 0
				args, exprs = append(args, silent, nil), append(exprs, exec.BoolC(silent), noValue)
				// An entry with recovered errors is not moved by an edit (memo.splice). One in five has them,
				// chosen without drawing from rng, so that the cases of the other operations stay as they were.
				if (int64(len(entries))+args[1].(int64))%5 == 0 {
					args, exprs = append(args, RecoveredErrors), append(exprs, exec.C(ErrsOp))
				} else {
					args, exprs = append(args, nil), append(exprs, noErrs)
				}
			case "memo.ok", "memo.end", "memo.silent":
				if len(entries) == 0 {
					continue
				}
				e := entries[rng.IntN(len(entries))]
				args, exprs = append(args, e.ref), append(exprs, exec.L(e.name))
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
				args, exprs = append(args, e.ref), append(exprs, exec.L(e.name))
				if o.Name == "memo.examine" {
					before := rng.IntN(3) == 0
					args, exprs = append(args, before), append(exprs, exec.BoolC(before))
					// An entry that examined no input records the empty range at its position.
					intArg(e.ref.Pos() + rng.Int64N(7))
				}
			case "budget.charge":
				v := rng.Uint64N(8)
				args, exprs = append(args, v), append(exprs, exec.U64(v))
			case "env.lookup", "env.bind":
				if len(cfg.Vars) == 0 {
					continue
				}
				intArg(rng.Int64N(int64(len(cfg.Vars))))
				if o.Name == "env.bind" {
					// Few values, so that many bindings are equal to the value they replace.
					id := []int64{1, 2, 3, 1002, 1003, 2003}[rng.IntN(6)]
					args, exprs = append(args, ref.Datum{ID: id}), append(exprs, exec.C(DatumOp, exec.Int(id)))
				}
			case "env.restore":
				if len(scopes) == 0 {
					continue
				}
				i := rng.IntN(len(scopes))
				args, exprs = append(args, scopeRefs[i]), append(exprs, exec.L(scopes[i]))
			case "rec.push":
				nextErr++
				args, exprs = append(args, &ref.Err{ID: nextErr}), append(exprs, exec.C(ErrOp, exec.Int(nextErr)))
			case "rec.truncate", "rec.since":
				intArg(rng.Int64N(mirror.Entries() + 1))
			case "rec.add", "rec.none":
				if len(lists) == 0 {
					continue
				}
				i := rng.IntN(len(lists))
				args, exprs = append(args, listRefs[i]), append(exprs, exec.L(lists[i]))
			case "env.key":
				if len(cfg.RuleVars) == 0 {
					continue
				}
				i := rng.Int64N(int64(len(cfg.RuleVars)))
				args, exprs = append(args, ref.RuleRef{Index: i}), append(exprs, exec.C(RuleOp, exec.Int(i)))
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
				b.Let(name, exec.C(o.Name))
				scopes, scopeRefs = append(scopes, name), append(scopeRefs, res[0].(*ref.Scope))
			case o.Name == "rec.since":
				name := fmt.Sprintf("l%d", calls)
				b.Let(name, exec.C(o.Name, exprs...))
				lists, listRefs = append(lists, name), append(listRefs, res[0].(*ref.ErrList))
			case o.Name == "env.lookup" || o.Name == "env.key":
				b.Let(fmt.Sprintf("v%d", calls), exec.C(o.Name, exprs...))
			case len(o.Results) > 0 && o.Results[0].Kind == exec.KHandle:
				name := fmt.Sprintf("e%d", calls)
				if len(o.Results) == 2 {
					b.LetN([]string{name, name + "f"}, exec.C(o.Name, exprs...))
					if !res[1].(bool) {
						continue // an entry that was not found must not be used
					}
				} else {
					b.Let(name, exec.C(o.Name, exprs...))
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
func mirrorCall(m *ref.Model, o *exec.Op, args []any) (res []any, stop bool) {
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(exec.Interrupt); !ok {
				panic(x)
			}
			stop = true
		}
	}()
	return m.Call(o, args), false
}
