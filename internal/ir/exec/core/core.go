// Package core holds the compositions of the parser VM (design 065): the algorithms that the
// hand-written runtimes repeat, written once in Exec IR over the operations of
// internal/ir/exec/ops.
//
// The compositions are the call protocol of a rule (deferred memoization, memo lookup with its
// reuse conditions, the dependency memo of left-recursion contexts, store and restore on failure),
// left-recursion growth, the observation of a call and the bookkeeping of a call in a Document
// parse. The compositions know no target: internal/codegen/go/lib prints them into the generated Go
// runtime and the engine, and the TypeScript generator (internal/codegen/ts) into the TypeScript
// runtime, each choosing the specializations it needs.
package core

import (
	. "zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// docMode selects the Document bookkeeping of a memoized call.
type docMode int

const (
	// nodeDoc keeps it when the parse reads a Document (doc.active), with the record in the entry, and
	// the examined end when the parse tracks it (doc.tracking).
	nodeDoc docMode = iota
	// typedDoc keeps it always, in a call that only a Document parse makes, with the record beside
	// the entry, which a call forgets when it replaces the entry.
	typedDoc
	// typedTrack keeps only the examined end, in the typed runtime's call outside a Document parse,
	// when the parse tracks it (doc.tracking): an observed parse reports it. The node runtime's call
	// leaves such a parse to its tracked path (memoized), so only the typed form keeps it.
	typedTrack
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
	m := ops.Parser()
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

	// leave returns the result of the call c from the call protocol.
	leave := func(c *Call) func(b *Builder) {
		return func(b *Builder) {
			b.SetN(pair, c)
			ret(b)
		}
	}
	// tracked leaves the call protocol of a rule call that is also the entry of Document parses for
	// the tracked path: the typed call of a Document parse for typedDocument, which takes the invocation
	// inv; the node call of a Document or an observed parse for tracked, which observes the call and
	// keeps the Document's bookkeeping (a node call at binding level 0 passes no invocation).
	tracked := func(b *Builder, level, inv Expr) {
		b.IfElse(F(ops.Typed), func(b *Builder) {
			b.If(And(F(ops.Document), C("doc.active")), leave(C("typedDocument", r, level, inv)))
		}, func(b *Builder) {
			b.If(F(ops.Document), func(b *Builder) {
				b.If(Bin("||", C("obs.active"), C("doc.active")), leave(C("tracked", r, level)))
			})
		})
	}
	// observedHere reports whether a call that leaves its parse to tracked can run in an observed parse:
	// the node call with Document bookkeeping hands every observed parse to the tracked path.
	observedHere := And(F(ops.Observe), Bin("||", F(ops.Typed), Not(F(ops.Document))))
	// engineTracked leaves the engine's call protocol for its traced path.
	engineTracked := func(b *Builder) {
		b.If(And(F(ops.Document), C("call.tracked")), leave(C("call.track", r, L("min"))))
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
	// tracking adds body, the record of an examined end, where the parse tracks it as doc tells: under
	// doc.tracking in the node runtime, which tracks in Document and observed parses, and in the typed
	// runtime's call outside a Document parse; always where the Document bookkeeping is kept always.
	tracking := func(b *Builder, doc docMode, body func(b *Builder)) {
		switch doc {
		case nodeDoc:
			b.If(C("doc.tracking"), body)
		case typedTrack:
			b.If(F(ops.Typed), func(b *Builder) { b.If(C("doc.tracking"), body) })
		case typedDoc, engineDoc:
			body(b)
		}
	}
	// docOpen and docClose bracket the memo protocol of a rule that only a Document parse memoizes, in the
	// node and typed runtimes, as work that only a Document does: a parse under the default limit
	// leaves what the bracketed code charges out of the count against it. A parse from scratch does not
	// memoize such a rule, so its protocol has no counterpart that the Document parse could be held to;
	// the protocol of every other rule counts, except the work of moving an entry and its value after
	// an edit, which opens its own region. The body of the rule is outside the brackets. A node
	// runtime's call is also the call of an observed parse that reads no Document, which has no such
	// work. The local fm holds the mark of the open bracket.
	// A call that opens no region leaves fm at the mark that FreeBegin returns for none, and pays no call.
	noFree := U64(^uint64(0))
	docOpen := func(b *Builder, doc docMode) {
		b.If(F(ops.Budget), func(b *Builder) {
			b.Set("fm", noFree)
			var on Expr = C("rule.docOnlyMemo", r)
			if doc == nodeDoc {
				on = And(C("doc.active"), on)
			}
			b.If(on, func(b *Builder) { b.Set("fm", C("budget.freeBegin", BoolC(true))) })
		})
	}
	docClose := func(b *Builder) {
		b.If(F(ops.Budget), func(b *Builder) {
			b.If(Not(Eq(L("fm"), noFree)), func(b *Builder) { b.Do("budget.freeEnd", L("fm")) })
		})
	}
	// exit returns the value and success of a call that reuse found: the result of the call, or with
	// hit set, the result of the begin of a split call (callBegin).
	exit := func(b *Builder, v, ok Expr) { b.Return(v, ok) }
	hit := func(b *Builder, v, ok Expr) { b.Return(v, ok, BoolC(true)) }
	// replay returns through exit the result s that the context of the call at start keeps (a seed if
	// seed is set), applied as the call that it records would have, as reuse applies a memo entry:
	// observed reports the reuse, the Document bookkeeping extends the examined input of the call to the
	// result's, and the expectations of a result that is not a seed are merged. A result does not move
	// after an edit: it lives within one parse.
	replay := func(b *Builder, s Expr, seed bool, doc docMode, observed Expr, exit func(b *Builder, v, ok Expr)) {
		b.If(observed, func(b *Builder) { b.Do("obs.memoHit") })
		tracking(b, doc, func(b *Builder) {
			b.Do("doc.touch", C("ctx.examined", s))
			inDocument(b, doc, func(b *Builder) {
				b.If(C("ctx.before", s, L("start")), func(b *Builder) { b.Do("doc.touchFrom", Add(L("start"), Int(-1))) })
				b.Do("doc.reused")
			})
		})
		if !seed {
			b.Do("mergeKept", C("ctx.far", s), C("ctx.expected", s))
		}
		b.If(Not(C("ctx.ok", s)), func(b *Builder) { exit(b, Null(value), BoolC(false)) })
		b.SetReg(ops.Pos, C("ctx.end", s))
		if doc == engineDoc {
			b.IfElse(And(F(ops.Missing), C("missing.active")), func(b *Builder) { b.Do("missing.replayResult", s) },
				func(b *Builder) { b.Do("rec.add", C("ctx.errs", s)) })
		} else {
			b.Do("rec.add", C("ctx.errs", s))
		}
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
	// finish restores from oldExamined and oldLow.
	reuse := func(b *Builder, level Expr, doc docMode, exit func(b *Builder, v, ok Expr)) {
		b.Let("env", C("env.key", r))
		// The node call that leaves observed parses to the tracked path reports no reuse itself.
		var observed Expr = F(ops.Observe)
		if doc == typedTrack {
			observed = observedHere
		}
		cs := b.LetN([]string{"c", "seed"}, C("ctx.enter", r, level))
		c := cs[0]
		b.If(C("ctx.found", cs[1]), func(b *Builder) { replay(b, cs[1], true, doc, observed, exit) })
		b.LetN([]string{"e", "found"}, C("memo.lookup", id, L("start"), level, L("env"), C("rule.docOnlyMemo", r)))
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
			b.If(observed, func(b *Builder) { b.Do("obs.memoHit") })
			// A Document parse tracks the examined end, so its bookkeeping nests in the tracking: a parse
			// that tracks nothing tests once, as before a parse could track without a Document.
			tracking(b, doc, func(b *Builder) {
				b.Do("doc.touch", C("memo.examined", e))
				inDocument(b, doc, func(b *Builder) {
					b.If(C("memo.before", e), func(b *Builder) { b.Do("doc.touchFrom", Add(L("start"), Int(-1))) })
					b.Do("doc.reused")
				})
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
			b.If(C("ctx.found", held), func(b *Builder) { replay(b, held, false, doc, observed, exit) })
		})
		b.Let("rec", C("rec.mark"))
		b.Do("ctx.push", c)
		if doc == engineDoc {
			b.LetN([]string{"oldExamined", "oldLow"}, C("doc.enter", L("start")))
		}
		// The memoized result includes the expectations recorded in this call.
		b.Let("m", C("exp.isolate", L("start")))
	}
	// recordEntry records with the entry that the call stored the expectations of the call and, with
	// doc, the input it examined, and pins the entry of a rule whose results depend on positions.
	recordEntry := func(b *Builder, entry Expr, doc docMode) {
		b.Do("memo.setExpected", entry, L("far"), L("kept"))
		// The start of the examined input serves only a Document (doc.low), the end also the report of a
		// reuse in an observed parse.
		examine := func(before Expr) func(b *Builder) {
			return func(b *Builder) { b.Do("memo.examine", entry, before, C("doc.examined")) }
		}
		pin := func(b *Builder) { b.If(C("rule.positional", r), func(b *Builder) { b.Do("memo.pin", entry) }) }
		tracking(b, doc, func(b *Builder) {
			switch doc {
			case nodeDoc:
				b.IfElse(C("doc.active"), func(b *Builder) {
					examine(Bin("<", C("doc.low"), L("start")))(b)
					pin(b)
				}, examine(BoolC(false)))
			case typedTrack:
				examine(BoolC(false))(b)
			default:
				examine(Bin("<", C("doc.low"), L("start")))(b)
				pin(b)
			}
		})
	}
	// finish records the result of the evaluated call where its dependencies send it: in the memo,
	// joined with the entry stored there (memo.store, memo.setPresent), or in the table of its context. It passes the
	// dependencies to the enclosing evaluation, keeps the expectations of the call with its result
	// and returns the result.
	//
	// With doc, a call in a Document parse also records with its entry the input it examined, and
	// pins the entry of a rule whose results depend on positions.
	finish := func(b *Builder, level Expr, doc docMode) {
		c := L("c")
		if doc == nodeDoc || doc == typedDoc {
			docOpen(b, doc)
		}
		b.LetN([]string{"far", "inner"}, C("exp.unisolate", L("m")))
		b.Let("kept", C("keep", L("inner")))
		b.Let("store", C("ctx.pop", c))
		b.IfElse(Eq(L("store"), C("ctx.storeMemo")), func(b *Builder) {
			// The store keeps an entry computed outside a lookahead that a result computed inside one
			// would narrow; the present set of the context joins that of the entry kept either way.
			entry := b.Let("entry", C("memo.store", id, L("start"), level, L("env"), L("ok"), R(ops.Pos), silent,
				L("v"), C("rec.since", L("rec")), C("rule.docOnlyMemo", r)))
			b.Do("memo.setPresent", entry, c)
			b.If(Bin("||", Not(silent), C("memo.silent", entry)), func(b *Builder) { recordEntry(b, entry, doc) })
		}, func(b *Builder) {
			b.If(Eq(L("store"), C("ctx.storeCtx")), func(b *Builder) {
				b.Do("ctx.keep", c, r, L("start"), level, L("env"), L("ok"), R(ops.Pos), silent, L("v"),
					C("rec.since", L("rec")), L("far"), L("kept"))
			})
		})
		b.Do("ctx.done", c)
		b.Do("mergeExpected", L("far"), L("inner"))
		if doc == engineDoc {
			b.Do("doc.leave", L("oldExamined"), L("oldLow"))
		}
		b.If(Not(L("ok")), func(b *Builder) { b.SetReg(ops.Pos, L("start")) })
		if doc == nodeDoc || doc == typedDoc {
			docClose(b)
		}
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

	firstCall := m.AddFunc("firstCall", []Param{P("r", rule)}, []Type{TBool}, nil, func(b *Builder) {
		b.Let("s", C("rule.seen", r))
		b.If(Not(And(C("seen.defers"), Bin(">=", L("s"), Int(0)))), func(b *Builder) { b.Return(BoolC(false)) })
		b.Return(C("seen.first", L("s"), R(ops.Pos)))
	})
	firstCall.Doc = `firstCall records a call of the memoized rule r at the position and reports whether its
memoization is deferred, in which case the call is not memoized. A result is reused only when the rule
is called again at the same position, which in practice is rare, so memoizing every first call would
cost a memo entry per call for nothing. Deferring memoization to the second call evaluates the body of
a rule at most twice per position, binding level and combination of variable values. That bounds how
often a rule body runs, not the time of a parse: loops inside a body, left-recursion growth and
variable environments do work that the memo does not share. A parse that memoizes on the first call, a
rule without a seen index and an eager rule are memoized at once.`

	call := m.AddFunc("call", []Param{P("r", rule), P("min", TInt)}, result,
		[]string{ops.Document, ops.Observe, ops.Typed, ops.Budget}, func(b *Builder) {
			locals(b)
			tracked(b, L("min"), Null(THandle(ops.Body)))
			charge(b)
			b.Let("start", R(ops.Pos))
			b.If(Bin("||", And(Not(C("rule.memo", r)), Not(C("rule.leader", r))), C("firstCall", r)), func(b *Builder) { unmemoized(b, false) })
			reuse(b, L("min"), typedTrack, exit)
			evaluate(b, L("min"), invoke)
			finish(b, L("min"), typedTrack)
		})
	call.Doc = `call calls rule r at binding level min: unmemoized while its memoization is deferred, and
otherwise through the memo and the dependency memo of left-recursion contexts, growing the seed of
a left-recursion leader. A call that is also the entry of Document
parses first hands a Document parse to its tracked path (typedDocument for typed values), and for
nodes an observed parse too (tracked).`

	callMemo := m.AddFunc("callMemo", []Param{P("r", rule), P("inv", THandle(ops.Body))}, result,
		[]string{ops.Document, ops.Observe, ops.Typed}, func(b *Builder) {
			locals(b)
			tracked(b, Int(0), L("inv"))
			b.Let("start", R(ops.Pos))
			reuse(b, Int(0), typedTrack, exit)
			evaluate(b, Int(0), func(b *Builder) { b.SetN(pair, C("rule.run", L("inv"))) })
			finish(b, Int(0), typedTrack)
		})
	callMemo.Doc = `callMemo is the memoized part of a rule call, for the rule r at binding level 0 invoked by inv:
the rule is called at the position again, or its memoization is no longer deferred.`

	// steps takes the steps of the growth g outside a context: it evaluates the body until a step does
	// not match more than the seed, and adopts each step that does with the errors it recovered after
	// the mark rec, dropping them from the record before the next step, and calling reset after.
	steps := func(b *Builder, g Expr, reset func(b *Builder)) {
		b.Let("rec", C("rec.mark"))
		b.Loop("m", func(b *Builder) {
			b.LetN([]string{"v", "ok"}, C("rule.invoke", r, L("min")))
			b.If(Not(C("grow.longer", g, L("ok"))), func(b *Builder) { b.Break("m") })
			b.Do("grow.adopt", g, L("v"), C("rec.since", L("rec")))
			b.Do("rec.truncate", L("rec"))
			reset(b)
		})
	}
	grow := m.AddFunc("grow", []Param{P("r", rule), P("min", TInt)}, result,
		[]string{ops.Typed, ops.Missing}, func(b *Builder) {
			g := b.Let("g", C("grow.begin", r, L("min")))
			b.IfElse(C("grow.ctx", g), func(b *Builder) {
				b.Loop("l", func(b *Builder) {
					b.If(Not(C("grow.next", g)), func(b *Builder) { b.Break("l") })
					b.LetN([]string{"v", "ok"}, C("rule.invoke", r, L("min")))
					b.Do("grow.step", g, L("v"), L("ok"))
				})
			}, func(b *Builder) {
				// In the missing-token mode, a step also drops the tokens it inserted.
				b.IfElse(F(ops.Missing), func(b *Builder) {
					mm := b.Let("mm", C("missing.mark"))
					steps(b, g, func(b *Builder) { b.Do("missing.reset", mm) })
				}, func(b *Builder) { steps(b, g, func(*Builder) {}) })
			})
			best := b.Let("best", C("grow.end", g))
			b.Return(C("ctx.value", best), C("ctx.ok", best))
		})
	grow.Doc = `grow grows the seed of a left-recursive leader from a failure: it evaluates the body again while
the match gets longer, and returns the last longer result. A growth in a context takes its steps
through the context, which may give them without evaluating the body. Outside a context, the form
specialized for missing also drops the tokens that each adopted step inserted.`

	// observe reports the call that body makes to the observation, with how far it examined the input.
	observe := func(body func(b *Builder)) func(b *Builder) {
		return func(b *Builder) {
			locals(b)
			b.Do("obs.enter", r, R(ops.Pos), L("min"))
			b.Let("h", C("doc.examinedBegin"))
			body(b)
			b.Let("x", C("doc.examinedEnd", L("h")))
			b.Do("obs.exit", L("ok"), R(ops.Pos), L("x"))
			ret(b)
		}
	}
	observed := m.AddFunc("observed", []Param{P("r", rule), P("min", TInt)}, result,
		[]string{ops.Document, ops.Observe, ops.Typed}, observe(func(b *Builder) {
			b.IfElse(F(ops.Typed), func(b *Builder) { b.SetN(pair, C("call", r, L("min"))) },
				func(b *Builder) { b.SetN(pair, C("body", r, L("min"))) })
		}))
	observed.Doc = `observed is a rule call in an observed parse: it reports the call to the observation, with how
far it examined the input, around the call without observation: the typed call, which observes nothing
itself, or the node call of the tracked path without observation.`
	observedPlain := m.AddFunc("observedPlain", []Param{P("r", rule), P("min", TInt)}, result,
		[]string{ops.Document, ops.Observe}, observe(func(b *Builder) { b.SetN(pair, C("rule.invokePlain", r, L("min"))) }))
	observedPlain.Doc = `observedPlain is the plain invocation of rule r at binding level min in an observed parse: it
reports the invocation to the observation as observed reports a call.`
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
		[]string{ops.Document, ops.Observe, ops.Budget}, func(b *Builder) {
			locals(b)
			b.Var("fm", TU64)
			b.Let("start", R(ops.Pos))
			docOpen(b, nodeDoc)
			reuse(b, L("min"), nodeDoc, func(b *Builder, v, ok Expr) { docClose(b); exit(b, v, ok) })
			docClose(b)
			evaluate(b, L("min"), invoke)
			finish(b, L("min"), nodeDoc)
		})
	memoized.Doc = `memoized calls rule r at binding level min through the memo, as call does once the rule's
memoization is no longer deferred, with the observation of a reuse and, in a Document parse, the
bookkeeping of how far each call examined the input.`

	document := m.AddFunc("document", []Param{P("r", rule), P("min", TInt)}, result,
		[]string{ops.Document, ops.Budget}, func(b *Builder) {
			b.LetN([]string{"oldExamined", "oldLow"}, C("doc.enter", R(ops.Pos)))
			b.Protect(func(b *Builder) {
				b.If(Not(C("rule.docMemo", r)), func(b *Builder) {
					b.LetN([]string{"v", "ok"}, C("unmemoized", r, L("min")))
					b.Return(L("v"), L("ok"))
				})
				b.LetN([]string{"v", "ok"}, C("memoized", r, L("min")))
				b.Return(L("v"), L("ok"))
			}, func(b *Builder) { b.Do("doc.leave", L("oldExamined"), L("oldLow")) })
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
			b.Var("fm", TU64)
			charge(b)
			b.Let("start", R(ops.Pos))
			b.LetN([]string{"oldExamined", "oldLow"}, C("doc.enter", L("start")))
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
				docOpen(b, typedDoc)
				reuse(b, L("min"), typedDoc, func(b *Builder, v, ok Expr) { docClose(b); exit(b, v, ok) })
				docClose(b)
				evaluate(b, L("min"), run)
				finish(b, L("min"), typedDoc)
			}, func(b *Builder) { b.Do("doc.leave", L("oldExamined"), L("oldLow")) })
		})
	typedDocument.Doc = `typedDocument is a typed rule call in a Document parse, for the rule r at binding level min,
invoked by inv when a generated call passes it: unmemoized for a rule that the Document does not
memoize, and otherwise through the memo, with the Document record of each entry kept beside it. As
in document, the enclosing call's examined input is restored however the call ends.`

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
			b.Do("call.saveExamined", st, L("oldExamined"), L("oldLow"))
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
			b.Let("oldExamined", C("call.examined", st))
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
	// docWork runs body as work that only a Document parse does: a parse under the default limit
	// leaves what body charges out of the count against it, if on holds and the parse reads a Document
	// (a test of one field, so that a parse from scratch pays almost nothing for the region). Other
	// forms run body as it is. The region ends before the rule body runs.
	// ret2 returns the result of the call c of a function.
	ret2 := func(b *Builder, c *Call) {
		b.LetN([]string{"v", "ok"}, c)
		b.Return(L("v"), L("ok"))
	}
	docWork := func(b *Builder, on Expr, body func(b *Builder)) {
		b.Set("fm", C("budget.freeBegin", on))
		body(b)
		b.Do("budget.freeEnd", L("fm"))
	}
	// engineBody builds the call of the engine. With doc, it is the call of a Document parse, where
	// what a parse from scratch does not do is Document work; without it, the call of any other
	// parse, which pays nothing for that work and hands a Document parse to the first.
	engineBody := func(doc bool) func(b *Builder) {
		return func(b *Builder) {
			locals(b)
			b.Var("hit", TBool)
			b.Var("mm", TInt)
			b.Var("direct", TBool)
			work := func(b *Builder, on Expr, body func(b *Builder)) { body(b) }
			if doc {
				b.Var("fm", TU64)
				work = docWork
			} else {
				b.If(C("doc.active"), func(b *Builder) {
					b.SetN(pair, C("engineCallDoc", r, L("min")))
					ret(b)
				})
			}
			// A parse from scratch calls the plain body of a plain rule directly, so the unit of the call
			// protocol is Document work there; for any other rule it counts.
			work(b, C("rule.docPlain", r), charge)
			engineTracked(b)
			b.Let("start", R(ops.Pos))
			// The memo protocol (the deferral, the begin and the end of the call) of a rule that only a
			// Document parse memoizes is Document work: a parse from scratch does not memoize the rule, so it has
			// no counterpart that the Document parse could be held to. The protocol of every other rule counts,
			// the copy and the replay of recovered errors included, except the work of moving an entry and its
			// value after an edit, which opens its own region.
			work(b, C("rule.docOnlyMemo", r), func(b *Builder) {
				b.Set("direct", Bin("||", And(Not(C("rule.memo", r)), Not(C("rule.leader", r))), C("firstCall", r)))
			})
			b.If(L("direct"), func(b *Builder) {
				unmemoized(b, true)
			})
			b.Let("st", C("call.state"))
			work(b, C("rule.docOnlyMemo", r), func(b *Builder) {
				b.SetN([]Place{{Name: "v"}, {Name: "ok"}, {Name: "hit"}}, C("callBegin", r, L("min"), st))
			})
			b.If(L("hit"), ret)
			b.IfElse(C("rule.leader", r), func(b *Builder) { b.SetN(pair, C("grow", r, L("min"))) }, invoke)
			work(b, C("rule.docOnlyMemo", r), func(b *Builder) {
				b.SetN(pair, C("callEnd", r, st, L("v"), L("ok")))
			})
			ret(b)
		}
	}
	engineCallDoc := m.AddFunc("engineCallDoc", []Param{P("r", rule), P("min", TInt)}, result, engine, engineBody(true))
	engineCall := m.AddFunc("engineCall", []Param{P("r", rule), P("min", TInt)}, result, engine, engineBody(false))
	engineCall.Doc = `engineCall calls rule r at binding level min in the engine: unmemoized while its
memoization is deferred, and otherwise between the begin and the end of the split call protocol,
growing the seed of a left-recursion leader. A parse that reads a Document makes the call of
engineCallDoc instead, which is decided by one test of a field that the parse sets once.`
	engineCallDoc.Doc = `engineCallDoc is engineCall for a parse that reads a Document. Under the default limit, what a
parse from scratch does not do is Document work and leaves the count against the limit: the unit of
the call protocol for a plain rule, and the memo protocol (its deferral, the begin and the end of
the call) of a rule that a parse from scratch does not memoize.`

	body := m.AddFunc("body", []Param{P("r", rule), P("min", TInt)}, result, []string{ops.Document, ops.Budget}, func(b *Builder) {
		charge(b)
		b.If(And(F(ops.Document), C("doc.active")), func(b *Builder) { ret2(b, C("document", r, L("min"))) })
		b.If(Bin("||", And(Not(C("rule.memo", r)), Not(C("rule.leader", r))), C("firstCall", r)), func(b *Builder) {
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
	expect := m.AddFunc("expect", expParams, nil, []string{ops.Budget}, func(b *Builder) {
		b.If(Bin("||", silent, Lt(pos, C("exp.farthest"))), func(b *Builder) { b.Return() })
		record(b)
	})
	expect.Doc = `expect records that id was expected at pos: an expectation before the farthest failure or in a
lookahead is dropped, one past it starts the record anew, and one at it joins the entries. It records no
examined input: the read that failed at pos recorded what it examined. It is for parser code and not a
stable API.`
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
		[]string{ops.Budget}, func(b *Builder) {
			b.Var("i", TInt)
			b.For("each", Assign("i", Int(0)), Lt(L("i"), C("exp.keptCount", L("k"))), Assign("i", Add(L("i"), Int(1))), func(b *Builder) {
				charge(b)
				b.Do("expect", L("far"), C("exp.keptAt", L("k"), L("i")))
			})
		})
	mergeKept.Doc = `mergeKept records the expectations that a memo entry kept at far, as expect does for each of them.`
	mergeExpected := m.AddFunc("mergeExpected", []Param{P("far", TInt), P("s", THandle(ops.Set))}, nil,
		[]string{ops.Budget}, func(b *Builder) {
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
	addValue(m)
	addTerms(m)

	if err := Verify(m); err != nil {
		panic(err)
	}
	return m
}

// addTerms adds the compositions of a predicate and a binding: the application of a term and what
// surrounds it. A term runs no rule body, so neither needs a split form. The term is the code of a
// lowered term (a Term handle), whose value is never a function: the lowering accepts a lambda only as
// the function argument of a built-in that it turns into a loop.
func addTerms(m *Module) {
	t := P("t", THandle(ops.Term))
	// evaluate applies t and leaves what it made behind: the gathered values of a term that failed,
	// and the nodes it created in either outcome. An interruption unwinds past this cleanup, because
	// it ends the parse and the next parse starts without them. A term that failed returns false; evaluate
	// returns the value of one that did not. The context is made after the marks, so that its base of created
	// nodes is the mark.
	evaluate := func(b *Builder) Expr {
		b.Let("created", C("term.created"))
		b.Let("kids", C("term.kids"))
		ctx := b.Let("ctx", C("term.ctx"))
		r := b.LetN([]string{"v", "f"}, Ap(L("t"), ctx))
		b.If(Bin("!=", r[1], Null(THandle(ops.Fail))), func(b *Builder) {
			b.Do("term.dropKids", L("kids"))
			b.Do("term.dropCreated", L("created"))
			b.Return(BoolC(false))
		})
		b.Do("term.dropCreated", L("created"))
		return r[0]
	}
	predicate := m.AddFunc("predicate", []Param{t}, []Type{TBool}, nil, func(b *Builder) {
		v := evaluate(b)
		// Only the boolean false fails: a box of false is a value of another kind.
		b.Return(Bin("||", Bin("!=", C("val.kind", v), Int(ops.KindBool)), C("val.toBool", v)))
	})
	predicate.Doc = `predicate applies the term of a predicate to the context that term.ctx makes. It fails on the boolean
false and on a term that fails, and holds on every other value. The value of a term is never a function.`
	bind := m.AddFunc("bind", []Param{P("slot", TInt), t}, []Type{TBool}, nil, func(b *Builder) {
		v := evaluate(b)
		b.Do("env.bind", L("slot"), v)
		b.Return(BoolC(true))
	})
	bind.Doc = `bind binds the value of the term of a binding to the variable in slot. A term that fails fails the
binding and binds nothing. The compiler has checked that the value fits the variable. The value of a term is never a
function.`
}
