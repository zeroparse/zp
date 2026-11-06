package core

import (
	. "zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// addDocument adds the composition of the edits that the engine applies to a memo entry when a lookup
// finds it, which is its implementation of memo.splice. The contract of memo.splice decides which entries stay,
// move and go; a target that applies an edit to every entry at once needs no composition for that, since the
// decision is one condition of its loop over the table.
func addDocument(m *Module) {
	e, log := L("e"), L("log")
	entry, edits := THandle(ops.Entry), THandle(ops.EditLog)
	advance := m.AddFunc("memoAdvance", []Param{P("e", entry), P("log", edits)}, []Type{TBool}, []string{ops.Document, ops.Budget}, func(b *Builder) {
		// A guard for memo.seed: a seed lives within one parse, when no edit is pending, so a lookup never
		// applies edits to one.
		b.If(C("memo.seed", e), func(b *Builder) { b.Return(BoolC(false)) })
		// An entry that lags behind more edits than the limit is not applied them: the call evaluates
		// again what it would reuse, which costs less than the edits do.
		b.If(Bin(">", Bin("-", C("edit.count", log), C("memo.applied", e)), Int(ops.PendingEditLimit)), func(b *Builder) { b.Return(BoolC(false)) })
		// The edits of a Document are work that a parse from scratch does not do.
		b.Var("fm", TU64)
		b.If(And(F(ops.Document), F(ops.Budget)), func(b *Builder) { b.Set("fm", C("budget.freeBegin", BoolC(true))) })
		b.Let("from", C("memo.from", e))
		b.Let("examined", C("memo.examined", e))
		b.Let("shift", Int(0))
		b.Let("shifted", C("memo.shifted", e))
		b.Var("i", TInt)
		from, examined, shift, shifted, i := L("from"), L("examined"), L("shift"), L("shifted"), L("i")
		b.For("edits", Assign("i", C("memo.applied", e)), Lt(i, C("edit.count", log)), Assign("i", Add(i, Int(1))), func(b *Builder) {
			b.If(F(ops.Budget), func(b *Builder) { b.Do("budget.charge", U64(1)) })
			b.If(Bin("<=", examined, C("edit.start", log, i)), func(b *Builder) { b.Continue("edits") })
			// The positions of recovered errors and of a pinned result do not move with the input.
			b.If(Not(And(And(Bin(">=", from, C("edit.end", log, i)), Not(C("memo.pinned", e))), C("rec.none", C("memo.errs", e)))),
				func(b *Builder) {
					b.If(And(F(ops.Document), F(ops.Budget)), func(b *Builder) { b.Do("budget.freeEnd", L("fm")) })
					b.Return(BoolC(false))
				})
			b.Let("delta", C("edit.delta", log, i))
			b.Set("from", Add(from, L("delta")))
			b.Set("examined", Add(examined, L("delta")))
			b.Set("shift", Add(shift, L("delta")))
			// An edit that keeps the length moves nothing.
			b.Set("shifted", Bin("||", shifted, Bin("!=", L("delta"), Int(0))))
		})
		b.Do("memo.advance", e, from, examined, shift, shifted, C("edit.count", log))
		b.If(And(F(ops.Document), F(ops.Budget)), func(b *Builder) { b.Do("budget.freeEnd", L("fm")) })
		b.Return(BoolC(true))
	})
	advance.Doc = `memoAdvance applies to the memo entry e the edits of log it does not account for yet, and reports
whether it is still valid. For each edit, in order, an entry that examined only input before it stays, one
that examined only input after it moves by the edit's change in length, unless it is pinned or holds
recovered errors, and any other is invalid. An entry that lags behind more than PendingEditLimit edits is invalid.
A form specialized for budget charges one unit for each edit it applies, as work of the Document that a parse
from scratch does not do. The entry changes only if it is still valid, and after every charge: one that is not valid
stays in its slot until a store replaces it, and a lookup before that applies the same edits again. The
first test, of memo.seed, is a guard that no production path reaches: the seed of a growth lives within one parse,
when no edit is pending, and a lookup applies edits only to an entry of an earlier parse.`
}
