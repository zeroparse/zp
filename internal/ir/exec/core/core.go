// Package core holds the compositions of the parser VM (design 065): the algorithms that the
// hand-written runtimes repeat, written once in Exec IR over the operations of
// internal/ir/exec/ops, and their specializations for each runtime.
//
// The compositions are the call protocol of a rule (deferred memoization, memo lookup with its
// reuse conditions, the dependency memo of left-recursion contexts, store and restore on failure),
// left-recursion growth, the observation of a call and the bookkeeping of a call in a Document
// parse. The Go backend prints
// them into the generated Go runtime (GoRuntime) and the TypeScript backend into the TypeScript
// runtime (TSMethods).
package core

import (
	. "zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// docMode selects the Document bookkeeping of a memoized call.
type docMode int

const (
	// noDoc keeps no Document bookkeeping.
	noDoc docMode = iota
	// nodeDoc keeps it when the parse reads a Document (doc.active), with the record in the entry.
	nodeDoc
	// typedDoc keeps it always, in a call that only a Document parse makes, with the record beside
	// the entry, which a call forgets when it replaces the entry.
	typedDoc
	// engineDoc keeps it always, with the record in the entry, as the engine does in every parse.
	// A call's examined input starts empty and ends where its matches read to; a call restores the
	// enclosing call's examined input when it ends; and a reused entry's value is moved after the
	// entry's expectations are merged: the engine's values are nodes, which it can always move. It
	// also keeps the missing-token mode, which evaluates again the calls that can insert a missing
	// token and records the tokens a reused result inserted.
	engineDoc
)

// streamPruneWindow is the number of positions that a stream commits between prunings of the memo.
const streamPruneWindow = 1024

// Build returns the module of the operations and the compositions, verified.
func Build() *Module {
	m := ops.Pilot()
	ops.Transitional(m)
	rule, value := THandle(ops.Rule), THandle(ops.Value)
	result := []Type{value, TBool}
	r := L("r")
	silent := Bin(">", R(ops.Silent), Int(0))
	id := C("rule.id", r)
	locals := func(b *Builder) {
		b.Var("v", value)
		b.Var("ok", TBool)
	}
	pair := []Place{{Name: "v"}, {Name: "ok"}}
	ret := func(b *Builder) { b.Return(L("v"), L("ok")) }
	// charge charges one unit of the work budget in a form for parses with budget controls.
	charge := func(b *Builder) {
		b.If(F(ops.Budget), func(b *Builder) { b.Do("budget.charge", U64(1)) })
	}

	// tracked leaves the call protocol for the target's tracked path.
	tracked := func(b *Builder, track *Call) {
		b.If(And(F(ops.Document), C("call.tracked")), func(b *Builder) {
			b.SetN(pair, track)
			ret(b)
		})
	}
	// inDocument adds the Document bookkeeping body as doc tells.
	inDocument := func(b *Builder, doc docMode, body func(b *Builder)) {
		switch doc {
		case nodeDoc:
			b.If(C("doc.active"), body)
		case typedDoc, engineDoc:
			body(b)
		}
	}
	// exit returns the value and success of a call that reuse found: the result of the call, or with
	// hit set, the result of the begin of a split call (callBegin).
	exit := func(b *Builder, v, ok Expr) { b.Return(v, ok) }
	hit := func(b *Builder, v, ok Expr) { b.Return(v, ok, BoolC(true)) }
	// replay returns through exit the result s that the context of the call keeps (a seed if seed is
	// set), applied as the call that it records would have.
	replay := func(b *Builder, s Expr, seed bool, exit func(b *Builder, v, ok Expr)) {
		b.Do("ctx.replay", s, BoolC(seed))
		b.If(Not(C("ctx.ok", s)), func(b *Builder) { exit(b, Null(value), BoolC(false)) })
		exit(b, C("ctx.value", s), BoolC(true))
	}
	// reuse returns through exit a result that the call at start may reuse: the seed of the rule if it
	// grows at the position; the entry stored for the call, if the leaders growing at the position
	// allow it and, when it was computed in a silent context, the call is in one too; or the result
	// kept in the context of the call. Otherwise it starts recording the dependencies of the call and
	// isolates its expectations.
	//
	// With Document bookkeeping, a call in a Document parse reuses an entry that an edit moved once
	// its value is moved too (doc.move), and extends the input it examined to the entry's. With the
	// engine's bookkeeping, it also starts the examined input of the call (doc.enter), whose ends
	// finish restores from oldHigh and oldLow.
	reuse := func(b *Builder, level Expr, doc docMode, exit func(b *Builder, v, ok Expr)) {
		b.Let("env", C("env.key", r))
		cs := b.LetN([]string{"c", "seed"}, C("ctx.enter", r, level))
		c := cs[0]
		b.If(C("ctx.found", cs[1]), func(b *Builder) { replay(b, cs[1], true, exit) })
		b.LetN([]string{"e", "found"}, C("memo.lookup", id, L("start"), level, L("env")))
		e := L("e")
		reusable := And(And(L("found"), C("ctx.allows", c, e)), Bin("||", Not(C("memo.silent", e)), silent))
		// After an edit, an entry is reused once its value is moved to its position.
		switch doc {
		case nodeDoc:
			reusable = And(reusable, Bin("||", Not(C("doc.active")), C("doc.move", e)))
		case typedDoc:
			reusable = And(reusable, C("doc.move", e))
		case engineDoc:
			reusable = And(reusable, Not(And(F(ops.Missing), C("missing.recompute", r))))
		}
		b.If(reusable, func(b *Builder) {
			b.If(F(ops.Observe), func(b *Builder) { b.Do("obs.memoHit") })
			inDocument(b, doc, func(b *Builder) {
				b.Do("doc.touch", C("memo.examined", e))
				b.If(C("memo.before", e), func(b *Builder) { b.Do("doc.touchFrom", Add(L("start"), Int(-1))) })
				b.Do("doc.reused")
			})
			b.Do("mergeKept", C("memo.far", e), C("memo.expected", e))
			if doc == engineDoc {
				// The engine moves every value, so the result of doc.move is always true.
				b.Do("doc.move", e)
			}
			b.If(Not(C("memo.ok", e)), func(b *Builder) { exit(b, Null(value), BoolC(false)) })
			b.SetReg(ops.Pos, C("memo.end", e))
			if doc == engineDoc {
				b.IfElse(And(F(ops.Missing), C("missing.active")), func(b *Builder) { b.Do("missing.replay", e) },
					func(b *Builder) { b.Do("rec.add", C("memo.errs", e)) })
			} else {
				b.Do("rec.add", C("memo.errs", e))
			}
			exit(b, C("memo.value", e), BoolC(true))
		})
		b.If(C("ctx.active", c), func(b *Builder) {
			held := b.Let("held", C("ctx.lookup", c, r, level, L("env")))
			b.If(C("ctx.found", held), func(b *Builder) { replay(b, held, false, exit) })
		})
		b.Let("rec", C("rec.mark"))
		b.Do("ctx.push", c)
		if doc == engineDoc {
			b.LetN([]string{"oldHigh", "oldLow"}, C("doc.enter", L("start")))
		}
		// The memoized result includes the expectations recorded in this call.
		b.Let("m", C("exp.isolate", L("start")))
	}
	// finish records the result of the evaluated call where its dependencies send it: in the memo,
	// with the present set of its context, or in the table of its context. It passes the
	// dependencies to the enclosing evaluation, keeps the expectations of the call with its result
	// and returns the result.
	//
	// With doc, a call in a Document parse also records with its entry the input it examined, and
	// pins the entry of a rule whose results depend on positions. With typed Document bookkeeping, the record of the entry that the call replaces is
	// forgotten, so that it does not keep the entry alive.
	finish := func(b *Builder, level Expr, doc docMode) {
		c := L("c")
		b.LetN([]string{"far", "inner"}, C("exp.unisolate", L("m")))
		b.Let("kept", C("keep", L("inner")))
		if doc != engineDoc {
			inDocument(b, doc, func(b *Builder) { b.Do("doc.touch", Add(R(ops.Pos), Int(1))) })
		}
		b.Let("store", C("ctx.pop", c))
		b.IfElse(Eq(L("store"), C("ctx.storeMemo")), func(b *Builder) {
			if doc == typedDoc {
				b.If(L("found"), func(b *Builder) { b.Do("memo.forgetDoc", L("e")) })
			}
			entry := b.Let("entry", C("memo.store", id, L("start"), level, L("env"), L("ok"), R(ops.Pos), silent,
				L("v"), C("rec.since", L("rec"))))
			b.Do("memo.setPresent", entry, c)
			b.Do("memo.setExpected", entry, L("far"), L("kept"))
			inDocument(b, doc, func(b *Builder) {
				b.Do("memo.examine", entry, Bin("<", C("doc.low"), L("start")), C("doc.high"))
				b.If(C("rule.positional", r), func(b *Builder) { b.Do("memo.pin", entry) })
			})
		}, func(b *Builder) {
			b.If(Eq(L("store"), C("ctx.storeCtx")), func(b *Builder) {
				b.Do("ctx.keep", c, r, L("start"), level, L("env"), L("ok"), R(ops.Pos), silent, L("v"),
					C("rec.since", L("rec")), L("far"), L("kept"))
			})
		})
		b.Do("ctx.done", c)
		b.Do("mergeExpected", L("far"), L("inner"))
		if doc == engineDoc {
			b.Do("doc.leave", L("oldHigh"), L("oldLow"))
		}
		b.If(Not(L("ok")), func(b *Builder) { b.SetReg(ops.Pos, L("start")) })
		ret(b)
	}

	// unmemoized is a call without memo bookkeeping, whose expectations need no isolation, from start.
	// With missing, a failure also drops the missing tokens that the call inserted, from the mark that
	// the local mm holds.
	unmemoized := func(b *Builder, missing bool) {
		b.If(C("rule.plain", r), func(b *Builder) {
			b.SetN(pair, C("rule.invokePlain", r, L("min")))
			ret(b)
		})
		b.Let("rec", C("rec.mark"))
		if missing {
			b.If(F(ops.Missing), func(b *Builder) { b.Set("mm", C("missing.mark")) })
		}
		b.SetN(pair, C("rule.invoke", r, L("min")))
		b.If(Not(L("ok")), func(b *Builder) {
			b.SetReg(ops.Pos, L("start"))
			b.Do("rec.truncate", L("rec"))
			if missing {
				b.If(F(ops.Missing), func(b *Builder) { b.Do("missing.reset", L("mm")) })
			}
		})
		ret(b)
	}

	// evaluate evaluates the call after reuse found nothing to reuse: it grows a leader's seed, and
	// otherwise evaluates the body with run, dropping the errors recovered by a failure.
	evaluate := func(b *Builder, level Expr, run func(b *Builder)) {
		b.IfElse(C("rule.leader", r), func(b *Builder) {
			b.SetN(pair, C("grow", r, level))
		}, func(b *Builder) {
			run(b)
			b.If(Not(L("ok")), func(b *Builder) { b.Do("rec.truncate", L("rec")) })
		})
	}
	invoke := func(b *Builder) { b.SetN(pair, C("rule.invoke", r, L("min"))) }

	call := m.AddFunc("call", []Param{P("r", rule), P("min", TInt)}, result,
		[]string{ops.Document, ops.Observe, ops.Typed, ops.Budget}, func(b *Builder) {
			locals(b)
			tracked(b, C("call.track", r, L("min")))
			charge(b)
			b.Let("start", R(ops.Pos))
			b.If(Bin("||", And(Not(C("rule.memo", r)), Not(C("rule.leader", r))), C("call.first", r)), func(b *Builder) { unmemoized(b, false) })
			reuse(b, L("min"), noDoc, exit)
			evaluate(b, L("min"), invoke)
			finish(b, L("min"), noDoc)
		})
	call.Doc = `call calls rule r at binding level min: unmemoized while its memoization is deferred, and
otherwise through the memo and the dependency memo of left-recursion contexts, growing the seed of
a left-recursion leader.`

	callMemo := m.AddFunc("callMemo", []Param{P("r", rule), P("inv", THandle(ops.Body))}, result,
		[]string{ops.Document, ops.Observe, ops.Typed}, func(b *Builder) {
			locals(b)
			tracked(b, C("call.trackBody", r, L("inv")))
			b.Let("start", R(ops.Pos))
			reuse(b, Int(0), noDoc, exit)
			evaluate(b, Int(0), func(b *Builder) { b.SetN(pair, C("rule.run", L("inv"))) })
			finish(b, Int(0), noDoc)
		})
	callMemo.Doc = `callMemo is the memoized part of a rule call, for the rule r at binding level 0 invoked by inv:
the rule is called at the position again, or its memoization is no longer deferred.`

	grow := m.AddFunc("grow", []Param{P("r", rule), P("min", TInt)}, result,
		[]string{ops.Typed}, func(b *Builder) {
			g := b.Let("g", C("grow.begin", r, L("min")))
			b.Loop("l", func(b *Builder) {
				b.If(Not(C("grow.next", g)), func(b *Builder) { b.Break("l") })
				b.LetN([]string{"v", "ok"}, C("rule.invoke", r, L("min")))
				b.Do("grow.step", g, L("v"), L("ok"))
			})
			best := b.Let("best", C("grow.end", g))
			b.Return(C("ctx.value", best), C("ctx.ok", best))
		})
	grow.Doc = `grow grows the seed of a left-recursive leader from a failure: it evaluates the body again while
the match gets longer, and returns the last longer result.`

	observed := m.AddFunc("observed", []Param{P("r", rule), P("min", TInt)}, result,
		[]string{ops.Document, ops.Observe}, func(b *Builder) {
			b.Do("obs.enter", r, R(ops.Pos), L("min"))
			b.Let("h", C("doc.examinedBegin"))
			b.LetN([]string{"v", "ok"}, C("call.body", r, L("min")))
			b.LetN([]string{"x", "known"}, C("doc.examinedEnd", L("h")))
			b.Do("obs.exit", L("ok"), R(ops.Pos), L("x"), L("known"))
			b.Return(L("v"), L("ok"))
		})
	observed.Doc = `observed is a rule call in an observed parse: it reports the call to the observation, with how
far it examined the input in a Document parse, around the call without observation.`
	callUnmemoized := m.AddFunc("unmemoized", []Param{P("r", rule), P("min", TInt)}, result, nil, func(b *Builder) {
		b.Var("v", value)
		b.Var("ok", TBool)
		b.Let("start", R(ops.Pos))
		unmemoized(b, false)
	})
	callUnmemoized.Doc = `unmemoized calls rule r at binding level min without memo bookkeeping, as call does while the
rule's memoization is deferred: its expectations need no isolation, and a failure restores the
position and the recovered errors.`

	memoized := m.AddFunc("memoized", []Param{P("r", rule), P("min", TInt)}, result,
		[]string{ops.Document, ops.Observe}, func(b *Builder) {
			locals(b)
			b.Let("start", R(ops.Pos))
			reuse(b, L("min"), nodeDoc, exit)
			evaluate(b, L("min"), invoke)
			finish(b, L("min"), nodeDoc)
		})
	memoized.Doc = `memoized calls rule r at binding level min through the memo, as call does once the rule's
memoization is no longer deferred, with the observation of a reuse and, in a Document parse, the
bookkeeping of how far each call examined the input.`

	document := m.AddFunc("document", []Param{P("r", rule), P("min", TInt)}, result,
		[]string{ops.Document}, func(b *Builder) {
			b.LetN([]string{"oldHigh", "oldLow"}, C("doc.enter", R(ops.Pos)))
			b.Protect(func(b *Builder) {
				b.If(Not(C("rule.docMemo", r)), func(b *Builder) {
					b.LetN([]string{"v", "ok"}, C("unmemoized", r, L("min")))
					b.Return(L("v"), L("ok"))
				})
				b.LetN([]string{"v", "ok"}, C("memoized", r, L("min")))
				b.Return(L("v"), L("ok"))
			}, func(b *Builder) { b.Do("doc.leave", L("oldHigh"), L("oldLow")) })
		})
	document.Doc = `document is a rule call in a Document parse, which memoizes the rules that the Document reuses
and records the input each call examined. The enclosing call's examined input is restored however
the call ends, an interruption included, so that a parse after it starts from the state it would
start from otherwise.`
	// run evaluates the body of a typed rule call: through inv, the invocation that a generated call
	// passes, when it passes one, and otherwise by invoking the rule.
	run := func(b *Builder) {
		b.IfElse(Bin("!=", L("inv"), Null(THandle(ops.Body))), func(b *Builder) {
			b.Do("doc.evaluated")
			b.SetN(pair, C("rule.run", L("inv")))
		}, invoke)
	}
	typedDocument := m.AddFunc("typedDocument", []Param{P("r", rule), P("min", TInt), P("inv", THandle(ops.Body))}, result,
		[]string{ops.Document, ops.Observe, ops.Typed, ops.Budget}, func(b *Builder) {
			locals(b)
			charge(b)
			b.Let("start", R(ops.Pos))
			b.LetN([]string{"oldHigh", "oldLow"}, C("doc.enter", L("start")))
			b.Protect(func(b *Builder) {
				b.If(Not(C("rule.docMemo", r)), func(b *Builder) {
					b.Let("rec", C("rec.mark"))
					run(b)
					b.If(Not(L("ok")), func(b *Builder) {
						b.SetReg(ops.Pos, L("start"))
						b.Do("rec.truncate", L("rec"))
					})
					ret(b)
				})
				reuse(b, L("min"), typedDoc, exit)
				evaluate(b, L("min"), run)
				finish(b, L("min"), typedDoc)
			}, func(b *Builder) { b.Do("doc.leave", L("oldHigh"), L("oldLow")) })
		})
	typedDocument.Doc = `typedDocument is a typed rule call in a Document parse, for the rule r at binding level min,
invoked by inv when a generated call passes it: unmemoized for a rule that the Document does not
memoize, and otherwise through the memo, with the Document record of each entry kept beside it. As
in document, the enclosing call's frontier is restored however the call ends.`

	// The split call protocol of the engine: callBegin and callEnd hold the state of a memoized call
	// between them in a record that the caller keeps (call.state), so that a target that does not
	// recurse runs the rule body, or the growth of a leader, between them. engineCall is the
	// recursive call over them.
	engine := []string{ops.Document, ops.Observe, ops.Missing, ops.Budget}
	st := L("st")
	callBegin := m.AddFunc("callBegin", []Param{P("r", rule), P("min", TInt), P("st", THandle(ops.CallState))},
		[]Type{value, TBool, TBool}, engine, func(b *Builder) {
			b.Let("start", R(ops.Pos))
			reuse(b, L("min"), engineDoc, hit)
			b.Do("call.save", st, L("start"), L("min"), L("env"), L("c"), L("rec"), L("m"))
			b.Do("call.saveExamined", st, L("oldHigh"), L("oldLow"))
			b.If(F(ops.Missing), func(b *Builder) { b.Do("call.saveMissing", st, C("missing.mark")) })
			b.Return(Null(value), BoolC(false), BoolC(false))
		})
	callBegin.Doc = `callBegin begins a memoized call of rule r at binding level min, with the engine's
bookkeeping of the input each call examines: it returns hit set with a result that the call reuses,
and otherwise saves in st what the end of the call needs.`
	callEnd := m.AddFunc("callEnd", []Param{P("r", rule), P("st", THandle(ops.CallState)), P("v", value), P("ok", TBool)},
		result, engine, func(b *Builder) {
			b.Let("start", C("call.start", st))
			b.Let("min", C("call.min", st))
			b.Let("env", C("call.key", st))
			b.Let("c", C("call.ctx", st))
			b.Let("rec", C("call.rec", st))
			b.Let("m", C("call.mark", st))
			b.Let("oldHigh", C("call.high", st))
			b.Let("oldLow", C("call.low", st))
			// The growth of a leader settles its recovered errors itself (grow.end).
			b.If(And(Not(C("rule.leader", r)), Not(L("ok"))), func(b *Builder) {
				b.Do("rec.truncate", L("rec"))
				b.If(F(ops.Missing), func(b *Builder) { b.Do("missing.reset", C("call.missing", st)) })
			})
			finish(b, L("min"), engineDoc)
		})
	callEnd.Doc = `callEnd ends the memoized call of rule r whose state its begin saved in st with the value v
and success ok of its body, or of the growth of a leader: it records the result where its
dependencies send it, and restores the state from before the call.`
	engineCall := m.AddFunc("engineCall", []Param{P("r", rule), P("min", TInt)}, result, engine, func(b *Builder) {
		locals(b)
		b.Var("hit", TBool)
		b.Var("mm", TInt)
		charge(b)
		tracked(b, C("call.track", r, L("min")))
		b.Let("start", R(ops.Pos))
		b.If(Bin("||", And(Not(C("rule.memo", r)), Not(C("rule.leader", r))), C("call.first", r)), func(b *Builder) {
			unmemoized(b, true)
		})
		b.Let("st", C("call.state"))
		b.SetN([]Place{{Name: "v"}, {Name: "ok"}, {Name: "hit"}}, C("callBegin", r, L("min"), st))
		b.If(L("hit"), ret)
		b.IfElse(C("rule.leader", r), func(b *Builder) { b.SetN(pair, C("grow", r, L("min"))) }, invoke)
		b.SetN(pair, C("callEnd", r, st, L("v"), L("ok")))
		ret(b)
	})
	engineCall.Doc = `engineCall calls rule r at binding level min in the engine: unmemoized while its
memoization is deferred, and otherwise between the begin and the end of the split call protocol,
growing the seed of a left-recursion leader.`

	// ret2 returns the result of the call c of a function.
	ret2 := func(b *Builder, c *Call) {
		b.LetN([]string{"v", "ok"}, c)
		b.Return(L("v"), L("ok"))
	}
	body := m.AddFunc("body", []Param{P("r", rule), P("min", TInt)}, result, []string{ops.Document, ops.Budget}, func(b *Builder) {
		charge(b)
		b.If(And(F(ops.Document), C("doc.active")), func(b *Builder) { ret2(b, C("document", r, L("min"))) })
		b.If(Bin("||", And(Not(C("rule.memo", r)), Not(C("rule.leader", r))), C("call.first", r)), func(b *Builder) {
			ret2(b, C("unmemoized", r, L("min")))
		})
		ret2(b, C("memoized", r, L("min")))
	})
	body.Doc = `body is a rule call on the tracked path without observation: the Document call in a Document
parse, and otherwise the unmemoized call while the rule's memoization is deferred and the memoized
call after.`
	trackedCall := m.AddFunc("tracked", []Param{P("r", rule), P("min", TInt)}, result, []string{ops.Document, ops.Observe}, func(b *Builder) {
		b.If(And(F(ops.Observe), C("obs.active")), func(b *Builder) { ret2(b, C("observed", r, L("min"))) })
		ret2(b, C("body", r, L("min")))
	})
	trackedCall.Doc = `tracked is a rule call in an observed or Document parse: the observed call in an observed parse,
and otherwise the call without observation.`

	// The stream compositions: delivery and commitment.
	streamCommit := m.AddFunc("streamCommit", []Param{P("pos", TInt)}, nil, []string{ops.Stream}, func(b *Builder) {
		pos := L("pos")
		// The preceding unit is kept for detecting line starts.
		b.Do("stream.discard", Bin("-", pos, Int(1)))
		b.Do("stream.release")
		// Pruning scans the memo, so it runs once the committed prefix has grown by a window.
		b.If(Bin(">=", Bin("-", pos, C("memo.floor")), Int(streamPruneWindow)), func(b *Builder) {
			b.Do("stream.dropRecords", pos)
			b.Do("memo.prune", pos)
		})
		b.If(C("stream.tail"), func(b *Builder) { b.Do("stream.settleErrors") })
	})
	streamCommit.Doc = `streamCommit makes it final that the parse never backtracks before pos: the input before pos - 1
is discarded (the preceding unit stays for detecting line starts), the memory the committed elements
held is released, and the memo and the records before pos are pruned once at least a window of
input was committed since the last pruning.`
	streamEmit := m.AddFunc("streamEmit", []Param{P("v", value), P("end", TInt)}, nil, []string{ops.Stream}, func(b *Builder) {
		b.Do("stream.check")
		b.Do("stream.report")
		b.Do("stream.deliver", L("v"))
		b.Do("stream.check")
		b.Do("streamCommit", L("end"))
	})
	streamEmit.Doc = `streamEmit passes a committed element v of the start rule's #stream repetition to the caller and
commits the input before end. The recovered errors committed so far are reported first, so that an
error of the diagnostic callback ends the parse before the element is delivered. Elements are final
once emitted: a caller error ends the parse unchanged.`
	streamsIn := m.AddFunc("streamsIn", []Param{P("id", TInt), P("rec", TBool)}, []Type{TBool}, []string{ops.Stream}, func(b *Builder) {
		b.Return(And(And(C("stream.emits"), Eq(C("stream.start"), L("id"))),
			Bin("||", Not(L("rec")), Eq(C("depth.current"), Int(1)))))
	})
	streamsIn.Doc = `streamsIn reports whether the #stream repetition of the rule id streams: the parse delivers
elements, the rule is its start rule, and the rule runs at the top (a recursive rule counts its entries,
so its outermost entry is the first; any other rule is entered once). It is not a stable API.`

	// The expectation compositions: recording one expectation.
	expID := THandle(ops.ExpID)
	expParams := []Param{P("pos", TInt), P("id", expID)}
	pos, eid := L("pos"), L("id")
	// record records id at pos, which is not before the farthest position: a position past it starts
	// the record anew. A parse that does not record expectations keeps only the placeholder, once
	// per record. Otherwise a scan of the record leaves out an id that is in it, charging each entry
	// it compares in a parse with budget controls.
	record := func(b *Builder) {
		b.If(Bin(">", pos, C("exp.farthest")), func(b *Builder) { b.Do("exp.reset", pos) })
		b.If(Not(C("exp.recording")), func(b *Builder) {
			b.If(C("exp.fresh"), func(b *Builder) { b.Do("exp.placeholder") })
			b.Return()
		})
		b.If(C("exp.maybe", eid), func(b *Builder) {
			b.Var("i", TInt)
			b.For("scan", Assign("i", Int(0)), Lt(L("i"), C("exp.count")), Assign("i", Add(L("i"), Int(1))), func(b *Builder) {
				charge(b)
				b.If(Eq(C("exp.at", L("i")), eid), func(b *Builder) { b.Return() })
			})
		})
		b.Do("exp.add", eid)
	}
	expect := m.AddFunc("expect", expParams, nil, []string{ops.Document, ops.Budget}, func(b *Builder) {
		b.If(F(ops.Document), func(b *Builder) { b.Do("doc.touch", Add(pos, Int(1))) })
		b.If(Bin("||", silent, Lt(pos, C("exp.farthest"))), func(b *Builder) { b.Return() })
		record(b)
	})
	expect.Doc = `expect records that id was expected at pos: an expectation before the farthest failure or in a
lookahead is dropped, one past it starts the record anew, and one at it joins the entries. A form specialized
for Document parses also extends the examined frontier of the call past pos; a form without it leaves
that out. It is for parser code and not a stable API.`
	expectSlow := m.AddFunc("expectSlow", expParams, nil, []string{ops.Budget}, func(b *Builder) { record(b) })
	expectSlow.Doc = `expectSlow records the expectation of expectOrdinary at or beyond the farthest position. It is
separate so that the guard of expectOrdinary inlines into the generated code.`
	expectSlow.NoInline = true
	expectOrdinary := m.AddFunc("expectOrdinary", expParams, nil, []string{ops.Budget}, func(b *Builder) {
		b.If(Bin("||", silent, Lt(pos, C("exp.farthest"))), func(b *Builder) { b.Return() })
		b.Do("expectSlow", pos, eid)
	})
	expectOrdinary.Doc = `expectOrdinary is expect in an ordinary parse: one without limit controls and outside a Document.
The generated rule bodies have a branch for such parses that calls the Ordinary forms of the runtime
helpers, which leave out the budget charges and the Document's record of the examined input. It is for generated
parsers and not a stable API.`

	mergeKept := m.AddFunc("mergeKept", []Param{P("far", TInt), P("k", THandle(ops.Kept))}, nil,
		[]string{ops.Document, ops.Budget}, func(b *Builder) {
			b.Var("i", TInt)
			b.For("each", Assign("i", Int(0)), Lt(L("i"), C("exp.keptCount", L("k"))), Assign("i", Add(L("i"), Int(1))), func(b *Builder) {
				charge(b)
				b.Do("expect", L("far"), C("exp.keptAt", L("k"), L("i")))
			})
		})
	mergeKept.Doc = `mergeKept records the expectations that a memo entry kept at far, as expect does for each of them.`
	mergeExpected := m.AddFunc("mergeExpected", []Param{P("far", TInt), P("s", THandle(ops.Set))}, nil,
		[]string{ops.Document, ops.Budget}, func(b *Builder) {
			far, s := L("far"), L("s")
			b.Var("i", TInt)
			// Each expectation costs one unit, whether or not the record keeps it. A parse that does not
			// record expectations holds only the placeholder, which needs no call of expect.
			b.IfElse(C("exp.recording"), func(b *Builder) {
				b.For("each", Assign("i", Int(0)), Lt(L("i"), C("exp.setCount", s)), Assign("i", Add(L("i"), Int(1))), func(b *Builder) {
					charge(b)
					b.Do("expect", far, C("exp.setAt", s, L("i")))
				})
			}, func(b *Builder) {
				b.For("each", Assign("i", Int(0)), Lt(L("i"), C("exp.setCount", s)), Assign("i", Add(L("i"), Int(1))), func(b *Builder) {
					charge(b)
					b.If(F(ops.Document), func(b *Builder) { b.Do("doc.touch", Add(far, Int(1))) })
					b.If(Not(Bin("||", silent, Lt(far, C("exp.farthest")))), func(b *Builder) {
						b.If(Bin(">", far, C("exp.farthest")), func(b *Builder) { b.Do("exp.reset", far) })
						b.If(C("exp.fresh"), func(b *Builder) { b.Do("exp.placeholder") })
					})
				})
			})
		})
	mergeExpected.Doc = `mergeExpected records the expectations of s at far, as expect does for each of them, so that the
expectations of a call that a memo entry or a finished isolation kept become those of the enclosing
record. s may alias the stack above the current record: writes only happen below the read position. It
is for parser code and not a stable API.`

	// keep: a set that outlives the record.
	keep := m.AddFunc("keep", []Param{P("s", THandle(ops.Set))}, []Type{THandle(ops.Kept)}, []string{ops.Budget}, func(b *Builder) {
		s := L("s")
		b.Let("n", C("exp.setCount", s))
		b.If(Eq(L("n"), Int(0)), func(b *Builder) { b.Return(C("exp.empty")) })
		// The same set often repeats: the set kept last is returned again when it is equal, comparing
		// entries in order and charging each comparison in a parse with budget controls.
		b.Let("last", C("exp.last"))
		b.Block("copy", func(b *Builder) {
			b.If(Bin("!=", C("exp.keptCount", L("last")), L("n")), func(b *Builder) { b.Exit("copy") })
			b.Var("i", TInt)
			b.For("cmp", Assign("i", Int(0)), Lt(L("i"), L("n")), Assign("i", Add(L("i"), Int(1))), func(b *Builder) {
				charge(b)
				b.If(Bin("!=", C("exp.keptAt", L("last"), L("i")), C("exp.setAt", s, L("i"))), func(b *Builder) { b.Exit("copy") })
			})
			b.Return(L("last"))
		})
		b.Let("k", C("exp.copy", s))
		b.Return(L("k"))
	})
	keep.Doc = `keep returns the expectations of s as a set that outlives the record, for a memo entry, a
context or a recovery to keep: the empty set for none, the set kept last when it has the same
entries in the same order, and otherwise a copy, which becomes the set kept last. It is for parser
code and not a stable API.`

	addOperators(m)
	addDocument(m)

	if err := Verify(m); err != nil {
		panic(err)
	}
	return m
}
