package execcore

import (
	. "zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// addOperators adds the compositions of the Operators Rule loop: the longest match of an operator,
// the trivia between operands and operators, and the loop of binding powers with its prefix
// operators and operands. The node and the TypeScript runtimes print the forms without typed
// values; the typed runtime prints operatorsParseTyped, operatorsExprTyped and operatorsNudTyped,
// whose values travel with where they start (opr.buildTyped). The engine prints the forms without
// typed values and the split forms, which stop where the iterative VM runs a line, a skip or an
// expression, and keep their state in a record the VM holds; both come from the same builders.
// unbounded is the binding level above which no operator follows an operand: a level above every
// level that a rule declares, so that it bounds nothing.
const unbounded = 1 << 30

func addOperators(m *Module) {
	rule, value := THandle(ops.Rule), THandle(ops.Value)
	attempt, list, line := THandle(ops.Attempt), THandle(ops.Operators), THandle(ops.Line)
	r := L("r")
	st := L("st")
	charge := func(b *Builder) {
		b.If(F(ops.Budget), func(b *Builder) { b.Do("budget.charge", U64(1)) })
	}
	// chargeLines charges the lines of the longest match over l at once: the match tries every one of
	// them, and charges are not rolled back, so a fixed limit admits the same parses as one charge per
	// line, at a fraction of the calls. A default budget that grows with demand, as a stream's does,
	// can run out up to the lines of the match less one unit earlier.
	chargeLines := func(b *Builder, l Expr) {
		b.If(F(ops.Budget), func(b *Builder) { b.Do("budget.charge", Conv(TU64, C("opr.count", l))) })
	}
	fail := func(b *Builder, typed bool) {
		if typed {
			b.Return(Null(value), Int(0), BoolC(false))
			return
		}
		b.Return(Null(value), BoolC(false))
	}
	// enterLine gives the line l a frame of its own and the cut flag of its own scope, and binds the
	// frame to the local f and what it replaces to prevFrame and prevCut.
	enterLine := func(b *Builder, l Expr) {
		b.Let("prevFrame", C("frame.current"))
		b.Let("prevCut", R(ops.Cut))
		b.Let("f", C("frame.open", C("opr.scope", l)))
		b.Do("frame.use", L("f"))
		b.SetReg(ops.Cut, BoolC(false))
	}
	// leaveLine binds the cut flag of the line that ran to the local cut and restores what enterLine
	// replaced.
	leaveLine := func(b *Builder, prevFrame, prevCut Expr) {
		b.Let("cut", R(ops.Cut))
		b.Do("frame.use", prevFrame)
		b.SetReg(ops.Cut, prevCut)
	}
	// framed runs the line l in a frame of its own, with the cut flag of its own scope, and binds
	// its value, success, frame and cut flag to the locals v, ok, f and cut.
	framed := func(b *Builder, l Expr, ok string) {
		enterLine(b, l)
		b.LetN([]string{"v", ok}, C("opr.match", l))
		leaveLine(b, L("prevFrame"), L("prevCut"))
	}
	// saveLine keeps what enterLine bound in the record st of a split form.
	saveLine := func(b *Builder, rec string) {
		b.Do(rec+".setPrevFrame", st, L("prevFrame"))
		b.Do(rec+".setPrevCut", st, L("prevCut"))
		b.Do(rec+".setF", st, L("f"))
	}
	// apply moves the parse to the end of the match a, with its environment and its recovered
	// errors.
	apply := func(b *Builder, a Expr) {
		b.SetReg(ops.Pos, C("opr.end", a))
		b.Do("env.restore", C("opr.env", a))
		b.IfElse(F(ops.Missing), func(b *Builder) { b.Do("opr.replay", a) }, func(b *Builder) {
			b.If(Not(C("rec.none", C("opr.errs", a))), func(b *Builder) { b.Do("rec.add", C("opr.errs", a)) })
		})
	}
	// drop discards the errors recovered, and the tokens inserted, since the backtracking point m.
	drop := func(b *Builder, m Expr) {
		b.If(F(ops.Missing), func(b *Builder) { b.Do("missing.reset", C("frame.markMissing", m)) })
		b.Do("rec.truncate", C("frame.markRec", m))
	}

	reset := m.AddFunc("reset", []Param{P("m", THandle(ops.Backtrack))}, nil, []string{ops.Missing}, func(b *Builder) {
		mk := L("m")
		b.SetReg(ops.Pos, C("frame.markPos", mk))
		b.Do("env.restore", C("frame.markEnv", mk))
		drop(b, mk)
		b.Do("frame.undo", C("frame.markTrail", mk))
	})
	reset.Doc = `reset returns to the backtracking point m: the position, the variable environment, the recovered
errors and the captures (undoing the trail) become what they were when m was taken.`

	// skipEnter and skipLeave go around the trivia expression.
	skipEnter := func(b *Builder) {
		b.Let("m", C("frame.mark"))
		b.SetReg(ops.Silent, Add(R(ops.Silent), Int(1)))
	}
	skipLeave := func(b *Builder) {
		b.SetReg(ops.Silent, Bin("-", R(ops.Silent), Int(1)))
		b.IfElse(Not(L("ok")), func(b *Builder) { b.Do("reset", L("m")) }, func(b *Builder) { drop(b, L("m")) })
	}
	skip := m.AddFunc("skipTrivia", []Param{P("r", rule)}, nil, []string{ops.Missing}, func(b *Builder) {
		b.If(Not(C("opr.hasSkip", r)), func(b *Builder) { b.Return() })
		skipEnter(b)
		b.Let("ok", C("opr.skip", r))
		skipLeave(b)
	})
	skip.Doc = `skipTrivia skips the trivia of the Operators Rule r, if it has any, in a lookahead: a failed skip
leaves the parse as it was, and one that matches discards the errors recovered in it, as a
lookahead does.`

	// candidate settles the operator op of a longest match after its line ran, with the value and
	// success in the locals v and ok and the cut flag in cut.
	longestFeats := []string{ops.Budget, ops.Missing}
	candidate := func(b *Builder, op, m0, start, f, best Expr, setBest func(*Builder, Expr), setFailed func(*Builder)) {
		// An empty prefix or postfix match could be applied forever.
		b.Let("matched", And(L("ok"), Not(And(Eq(R(ops.Pos), start), Not(C("opr.infix", op))))))
		b.If(And(L("matched"), Bin("||", Not(C("opr.found", best)), Bin(">", R(ops.Pos), C("opr.end", best)))), func(b *Builder) {
			errs := C("rec.since", C("frame.markRec", m0))
			b.IfElse(F(ops.Missing), func(b *Builder) {
				setBest(b, C("opr.attemptMissing", op, f, L("v"), start, L("cut"), errs, C("frame.markMissing", m0)))
			}, func(b *Builder) { setBest(b, C("opr.attempt", op, f, L("v"), start, L("cut"), errs)) })
		})
		b.If(And(Not(L("matched")), L("cut")), setFailed)
		b.IfElse(Eq(C("frame.slots", f), Int(0)), func(b *Builder) { b.Do("reset", m0) }, func(b *Builder) {
			// The reset undoes the captures of this candidate; they are restored. Nothing parses
			// between saving and restoring them.
			b.Do("frame.save", f)
			b.Do("reset", m0)
			b.Do("frame.restore", f)
		})
	}

	longest := m.AddFunc("longest", []Param{P("l", list)}, []Type{attempt}, longestFeats, func(b *Builder) {
		b.Let("m0", C("frame.mark"))
		start := b.Let("start", C("frame.markPos", L("m0")))
		b.Var("best", attempt)
		b.Var("cutFailed", TBool)
		b.Var("i", TInt)
		chargeLines(b, L("l"))
		b.For("each", Assign("i", Int(0)), Lt(L("i"), C("opr.count", L("l"))), Assign("i", Add(L("i"), Int(1))), func(b *Builder) {
			op := b.Let("op", C("opr.at", L("l"), L("i")))
			framed(b, C("opr.line", op), "ok")
			candidate(b, op, L("m0"), start, L("f"), L("best"), func(b *Builder, x Expr) { b.Set("best", x) },
				func(b *Builder) { b.Set("cutFailed", BoolC(true)) })
		})
		b.Do("opr.setCutFailed", L("cutFailed"))
		b.Return(L("best"))
	})
	longest.Doc = `longest tries every operator of l from the position and returns the longest match, the first
declared among equals, or the null attempt; it records whether a candidate failed after a cut in the
parser's cutFailed field (p.cutFailed in Go, this.cutFailed in TypeScript). The parse is left at the
position, with the captures of each candidate in its frame.`

	// The steps of the Operators Rule loop that the recursive and the split forms share. An exit ends
	// a step: fail fails the expression, brk ends the loop with the left operand, and next goes on.
	type exits struct{ fail, brk, next func(*Builder) }
	// loopState names where a step of the loop reads and writes the state of the expression: bound is
	// the tightest level that may follow the start or the last operator applied, and guard is where
	// the expression began when it is the right operand of an infix operator applied without
	// consuming input, and -1 otherwise.
	type loopState struct {
		a, m0, start, min, lastNone, bound, guard Expr
		setLastNone, setBound              func(*Builder, Expr)
	}
	// ledStep settles the match a of the loop: it ends the loop, or applies the operator, and ends the
	// step for a postfix operator or else binds the level of the right operand to the local rmin.
	ledStep := func(b *Builder, s loopState, x exits, build func(b *Builder, lhs, rhs Expr), lhs Expr) {
		a := s.a
		b.If(Not(C("opr.found", a)), func(b *Builder) {
			b.Let("failed", C("opr.cutFailed"))
			b.Do("reset", s.m0)
			b.If(L("failed"), x.fail)
			x.brk(b)
		})
		op := b.Let("op", C("opr.op", a))
		level := b.Let("level", C("opr.level", op))
		b.If(Bin("||", Bin("||", Bin("<=", level, s.min), Bin(">", level, s.bound)), And(C("opr.nonAssoc", op), Eq(level, s.lastNone))), func(b *Builder) {
			b.Do("reset", s.m0)
			x.brk(b)
		})
		// The right operand of an infix operator applied without consuming input does not apply an
		// empty operator again before it consumes input: it would begin the same expression at the
		// same position without end.
		b.If(And(Eq(s.start, s.guard), Eq(C("opr.end", a), s.start)), func(b *Builder) {
			b.Do("reset", s.m0)
			x.brk(b)
		})
		apply(b, a)
		b.If(C("opr.postfix", op), func(b *Builder) {
			build(b, lhs, Null(value))
			s.setLastNone(b, Int(-1))
			s.setBound(b, Int(unbounded))
			b.If(Eq(R(ops.Pos), s.start), x.brk)
			x.next(b)
		})
		b.Let("rmin", level)
		b.If(C("opr.right", op), func(b *Builder) { b.Set("rmin", Bin("-", L("rmin"), Int(1))) })
		b.Do("opr.setAfterEmpty", Eq(R(ops.Pos), s.start))
	}
	// guardAt binds where the expression that begins here is guarded (see loopState) to the place
	// set, and clears the record that opr.setAfterEmpty made for it.
	guardAt := func(b *Builder, init, set func(b *Builder, x Expr)) {
		init(b, Int(-1))
		b.If(C("opr.afterEmpty"), func(b *Builder) { set(b, R(ops.Pos)) })
		b.Do("opr.setAfterEmpty", BoolC(false))
	}
	// rhsStep ends the step of an infix operator with its right operand in the locals rhs and rok.
	rhsStep := func(b *Builder, s loopState, op, level Expr, x exits, build func(b *Builder, lhs, rhs Expr), lhs Expr) {
		b.If(Not(L("rok")), func(b *Builder) {
			b.If(C("opr.cut", s.a), x.fail)
			b.Do("reset", s.m0)
			x.brk(b)
		})
		build(b, lhs, L("rhs"))
		s.setLastNone(b, Int(-1))
		b.If(C("opr.nonAssoc", op), func(b *Builder) { s.setLastNone(b, level) })
		// The right operand takes every operator that binds tighter than the operator's own level, so
		// when one that does follows, the right operand refused it (a non-associative operator
		// there does not chain, and an operator whose right operand fails is not applied): it cannot
		// apply to the operator and its operands as a whole either. bound is the tightest level that
		// can still follow.
		b.Let("rbound", level)
		b.If(C("opr.right", op), func(b *Builder) { b.Set("rbound", Bin("-", L("rbound"), Int(1))) })
		s.setBound(b, L("rbound"))
		// An application that consumed no input ends the loop, as an empty iteration ends a
		// repetition: otherwise an operator that can match empty after an operand that can
		// loops forever.
		b.If(Eq(R(ops.Pos), s.start), x.brk)
	}
	// prefixRhs ends the application of the prefix operator of a with its operand in the locals rhs
	// and rok: done returns the application, failed fails the start, and otherwise the parse is back
	// at m0.
	prefixRhs := func(b *Builder, a, m0 Expr, done, failed func(*Builder)) {
		b.If(L("rok"), done)
		b.If(C("opr.cut", a), failed)
		b.Do("reset", m0)
	}
	// operandEnd ends the operand line that ran: matched returns its value, failed fails the start.
	operandEnd := func(b *Builder, m0 Expr, matched, failed func(*Builder)) {
		b.If(L("ok"), matched)
		b.Do("reset", m0)
		b.If(L("cut"), failed)
	}

	for _, typed := range []bool{false, true} {
		suffix, feats := "", []string{ops.Budget, ops.Missing}
		results := []Type{value, TBool}
		if typed {
			suffix, feats = "Typed", []string{ops.Budget, ops.Missing, ops.Typed}
		}
		parseName, exprName, nudName := "operatorsParse"+suffix, "operatorsExpr"+suffix, "operatorsNud"+suffix
		// build binds lhs, and lstart for typed values, to the application of the operator of a.
		build := func(b *Builder, lhs, rhs Expr) {
			if typed {
				b.SetN([]Place{{Name: "lhs"}, {Name: "lstart"}}, C("opr.buildTyped", r, L("a"), lhs, L("lstart"), rhs))
				return
			}
			b.Set("lhs", C("opr.build", r, L("a"), lhs, rhs))
		}

		parse := m.AddFunc(parseName, []Param{P("r", rule), P("min", TInt)}, results, feats, func(b *Builder) {
			b.Do("depth.enter")
			b.LetN([]string{"v", "ok"}, C(exprName, r, L("min")))
			b.Do("depth.exit")
			b.Return(L("v"), L("ok"))
		})
		parse.Doc = parseName + ` parses an expression of the Operators Rule r with operators that bind tighter than
min. Each level of its recursion (the operand of a prefix operator, the right operand of an infix
operator) counts against the nesting limit, like the entry of a recursive rule.`

		expr := m.AddFunc(exprName, []Param{P("r", rule), P("min", TInt)}, results, feats, func(b *Builder) {
			guardAt(b, func(b *Builder, x Expr) { b.Let("guard", x) }, func(b *Builder, x Expr) { b.Set("guard", x) })
			if typed {
				b.LetN([]string{"lhs", "lstart", "ok"}, C(nudName, r))
			} else {
				b.LetN([]string{"lhs", "ok"}, C(nudName, r))
			}
			b.If(Not(L("ok")), func(b *Builder) { b.Return(Null(value), BoolC(false)) })
			b.Let("led", C("opr.led", r))
			// Without an infix or postfix operator nothing can follow the start: the loop would only
			// evaluate the skip again at every level of a chain of prefix operators.
			b.If(Eq(C("opr.count", L("led")), Int(0)), func(b *Builder) { b.Return(L("lhs"), BoolC(true)) })
			b.Let("lastNone", Int(-1))
			b.Let("bound", C("opr.nudBound"))
			b.Loop("loop", func(b *Builder) {
				b.Let("m0", C("frame.mark"))
				start := b.Let("start", C("frame.markPos", L("m0")))
				b.Do("skipTrivia", r)
				a := b.Let("a", C("longest", L("led")))
				s := loopState{a: a, m0: L("m0"), start: start, min: L("min"), lastNone: L("lastNone"), bound: L("bound"), guard: L("guard"),
					setLastNone: func(b *Builder, x Expr) { b.Set("lastNone", x) },
					setBound:    func(b *Builder, x Expr) { b.Set("bound", x) }}
				x := exits{fail: func(b *Builder) { b.Return(Null(value), BoolC(false)) },
					brk: func(b *Builder) { b.Break("loop") }, next: func(b *Builder) { b.Continue("loop") }}
				ledStep(b, s, x, build, L("lhs"))
				b.LetN([]string{"rhs", "rok"}, C(parseName, r, L("rmin")))
				rhsStep(b, s, L("op"), L("level"), x, build, L("lhs"))
			})
			b.Return(L("lhs"), BoolC(true))
		})
		expr.Doc = exprName + ` parses an operand, or a prefix application, and then applies the operators that
follow it while they bind tighter than min: the longest operator match wins, a non-associative
operator does not apply again at its own level, a right-associative one parses its right operand at
one level less, and a failed right operand ends the loop before the operator unless the operator cut.
An operator that binds tighter than the last one applied, or than the prefix operator of the start,
ends the loop: its right operand refused it, so it cannot apply to the operator as a whole.`

		nudResults := results
		if typed {
			nudResults = []Type{value, TInt, TBool}
		}
		nud := m.AddFunc(nudName, []Param{P("r", rule)}, nudResults, feats, func(b *Builder) {
			b.Do("skipTrivia", r)
			b.Let("m0", C("frame.mark"))
			start := b.Let("start", C("frame.markPos", L("m0")))
			a := b.Let("a", C("longest", C("opr.prefix", r)))
			b.IfElse(C("opr.found", a), func(b *Builder) {
				apply(b, a)
				b.LetN([]string{"rhs", "rok"}, C(parseName, r, C("opr.level", C("opr.op", a))))
				prefixRhs(b, a, L("m0"), func(b *Builder) {
					b.Do("opr.setNudBound", C("opr.level", C("opr.op", a)))
					if typed {
						b.LetN([]string{"x", "xstart"}, C("opr.buildTyped", r, a, Null(value), Int(0), L("rhs")))
						b.Return(L("x"), L("xstart"), BoolC(true))
						return
					}
					b.Return(C("opr.build", r, a, Null(value), L("rhs")), BoolC(true))
				}, func(b *Builder) { fail(b, typed) })
			}, func(b *Builder) {
				b.If(C("opr.cutFailed"), func(b *Builder) { fail(b, typed) })
			})
			b.Var("i", TInt)
			b.For("each", Assign("i", Int(0)), Lt(L("i"), C("opr.operands", r)), Assign("i", Add(L("i"), Int(1))), func(b *Builder) {
				charge(b)
				o := b.Let("o", C("opr.operand", r, L("i")))
				framed(b, o, "ok")
				operandEnd(b, L("m0"), func(b *Builder) {
					b.Do("opr.setNudBound", Int(unbounded))
					if typed {
						b.Return(C("opr.lineValue", r, o, L("f"), L("v"), start), start, BoolC(true))
						return
					}
					b.Return(C("opr.lineValue", r, o, L("f"), L("v"), start), BoolC(true))
				}, func(b *Builder) { fail(b, typed) })
			})
			fail(b, typed)
		})
		nud.Doc = nudName + ` parses the start of an expression of the Operators Rule r: the application of the
longest prefix operator to its operand, or else the first operand line that matches. A failure after
a cut fails the expression.`
		if typed {
			nud.Doc += ` Besides its value it returns where the value starts: a value that is not a node has no
span of its own.`
		}
	}

	// The split forms of the engine's iterative VM, which does not recurse: each function is a step
	// between two points where the VM runs a line, a skip or an expression, built from the steps of
	// the recursive forms, with what the next step needs in a record that the caller holds.
	split := func(name string, params []Param, results []Type, doc string, body func(b *Builder)) {
		m.AddFunc(name, params, results, longestFeats, body).Doc = doc
	}
	longestSt, nudSt, exprSt := P("st", THandle(ops.LongestState)), P("st", THandle(ops.NudState)), P("st", THandle(ops.ExprState))
	lg := func(field string) Expr { return C("oprLongest."+field, st) }
	nd := func(field string) Expr { return C("oprNud."+field, st) }
	ex := func(field string) Expr { return C("oprExpr."+field, st) }

	split("triviaBegin", nil, []Type{THandle(ops.Backtrack)}, `triviaBegin begins the trivia of an Operators Rule: the
caller runs the trivia expression and passes the mark to triviaEnd.`, func(b *Builder) {
		skipEnter(b)
		b.Return(L("m"))
	})
	split("triviaEnd", []Param{P("m", THandle(ops.Backtrack)), P("ok", TBool)}, nil, `triviaEnd ends the trivia that triviaBegin
began, whose expression reported ok, as skipTrivia does.`, skipLeave)

	split("longestBegin", []Param{longestSt, P("l", list)}, nil, `longestBegin begins the longest match of the
operators l at the position, in the empty record st.`, func(b *Builder) {
		b.Do("oprLongest.setL", st, L("l"))
		b.Do("oprLongest.setM0", st, C("frame.mark"))
		chargeLines(b, L("l"))
	})
	// nextLine ends a step of the match: it returns the line of the next operator, or false after the
	// last one.
	nextLine := func(b *Builder) {
		b.If(Eq(lg("i"), C("opr.count", lg("l"))), func(b *Builder) {
			b.Do("opr.setCutFailed", lg("cutFailed"))
			b.Return(Null(line), BoolC(false))
		})
		op := b.Let("nextOp", C("opr.at", lg("l"), lg("i")))
		enterLine(b, C("opr.line", op))
		saveLine(b, "oprLongest")
		b.Return(C("opr.line", op), BoolC(true))
	}
	split("longestNext", []Param{longestSt}, []Type{line, TBool}, `longestNext returns the first line of the match st,
which the caller runs and passes to longestStep, and false if it has none.`, nextLine)
	split("longestStep", []Param{longestSt, P("v", value), P("ok", TBool)}, []Type{line, TBool}, `longestStep ends the line
of the match st that the caller ran, with its value v and success ok, as longest does for each
operator, and returns the next line. It returns false after the last one, when the record holds the
match and the parser whether a candidate failed after a cut. It is one step, not two, because the VM
pays for a call at every line of every match.`, func(b *Builder) {
		op := b.Let("op", C("opr.at", lg("l"), lg("i")))
		leaveLine(b, lg("prevFrame"), lg("prevCut"))
		candidate(b, op, lg("m0"), C("frame.markPos", lg("m0")), lg("f"), lg("best"),
			func(b *Builder, x Expr) { b.Do("oprLongest.setBest", st, x) },
			func(b *Builder) { b.Do("oprLongest.setCutFailed", st, BoolC(true)) })
		b.Do("oprLongest.setI", st, Add(lg("i"), Int(1)))
		nextLine(b)
	})

	// The start of an expression: the caller skips the trivia, calls nudBegin, runs the longest match
	// of the prefix operators, puts it in the record and calls nudPrefix.
	split("nudBegin", []Param{nudSt}, nil, `nudBegin begins the start of an expression in the empty record st, after
its trivia.`, func(b *Builder) { b.Do("oprNud.setM0", st, C("frame.mark")) })
	split("nudPrefix", []Param{P("r", rule), nudSt}, []Type{TInt, TBool}, `nudPrefix settles the longest match of the prefix
operators in st: it applies the operator and returns the binding level of its operand, or 0 for
no operator, with false when the start fails.`, func(b *Builder) {
		a := nd("a")
		b.IfElse(C("opr.found", a), func(b *Builder) {
			apply(b, a)
			b.Return(C("opr.level", C("opr.op", a)), BoolC(true))
		}, func(b *Builder) {
			b.If(C("opr.cutFailed"), func(b *Builder) { b.Return(Int(0), BoolC(false)) })
		})
		b.Return(Int(0), BoolC(true))
	})
	split("nudRhs", []Param{P("r", rule), nudSt, P("rhs", value), P("rok", TBool)}, []Type{value, TBool, TBool}, `nudRhs ends the
application of a prefix operator to its operand rhs, whose parse reported rok: with done set it
returns the result of the start, and otherwise the parse goes on with the operand lines.`, func(b *Builder) {
		a := nd("a")
		prefixRhs(b, a, nd("m0"), func(b *Builder) {
			b.Do("opr.setNudBound", C("opr.level", C("opr.op", a)))
			b.Return(C("opr.build", r, a, Null(value), L("rhs")), BoolC(true), BoolC(true))
		}, func(b *Builder) { b.Return(Null(value), BoolC(false), BoolC(true)) })
		b.Return(Null(value), BoolC(false), BoolC(false))
	})
	split("nudNext", []Param{P("r", rule), nudSt}, []Type{line, TBool}, `nudNext returns the next operand line of the start st,
which the caller runs and passes to nudOperand, and false after the last one, when the start fails.`, func(b *Builder) {
		b.If(Eq(nd("i"), C("opr.operands", r)), func(b *Builder) { b.Return(Null(line), BoolC(false)) })
		charge(b)
		o := b.Let("o", C("opr.operand", r, nd("i")))
		enterLine(b, o)
		saveLine(b, "oprNud")
		b.Return(o, BoolC(true))
	})
	split("nudOperand", []Param{P("r", rule), nudSt, P("v", value), P("ok", TBool)}, []Type{value, TBool, TBool}, `nudOperand ends the
operand line that nudNext returned, with its value v and success ok: with done set it returns the
result of the start, and otherwise the next line is tried.`, func(b *Builder) {
		o := b.Let("o", C("opr.operand", r, nd("i")))
		leaveLine(b, nd("prevFrame"), nd("prevCut"))
		operandEnd(b, nd("m0"), func(b *Builder) {
			b.Do("opr.setNudBound", Int(unbounded))
			b.Return(C("opr.lineValue", r, o, nd("f"), L("v"), C("frame.markPos", nd("m0"))), BoolC(true), BoolC(true))
		}, func(b *Builder) { b.Return(Null(value), BoolC(false), BoolC(true)) })
		b.Do("oprNud.setI", st, Add(nd("i"), Int(1)))
		b.Return(Null(value), BoolC(false), BoolC(false))
	})

	// The loop of an expression. Its steps return done with the success of the expression, whose value
	// is the left operand in the record.
	exprState := func() loopState {
		return loopState{a: ex("a"), m0: ex("m0"), start: C("frame.markPos", ex("m0")), min: ex("min"), lastNone: ex("lastNone"), bound: ex("bound"), guard: ex("guard"),
			setLastNone: func(b *Builder, x Expr) { b.Do("oprExpr.setLastNone", st, x) },
			setBound:    func(b *Builder, x Expr) { b.Do("oprExpr.setBound", st, x) }}
	}
	exprBuild := func(b *Builder, lhs, rhs Expr) { b.Do("oprExpr.setLhs", st, C("opr.build", r, ex("a"), lhs, rhs)) }
	split("exprBegin", []Param{exprSt}, nil, `exprBegin begins the expression st, before its start.`, func(b *Builder) {
		set := func(b *Builder, x Expr) { b.Do("oprExpr.setGuard", st, x) }
		guardAt(b, set, set)
	})
	split("exprStart", []Param{P("r", rule), exprSt, P("lhs", value), P("ok", TBool)}, []Type{TBool, TBool}, `exprStart takes the start
lhs of an expression, whose parse reported ok, into the record st: done is set when nothing can
follow it.`, func(b *Builder) {
		b.If(Not(L("ok")), func(b *Builder) { b.Return(BoolC(true), BoolC(false)) })
		b.Do("oprExpr.setLhs", st, L("lhs"))
		// Without an infix or postfix operator nothing can follow the start: the loop would only
		// evaluate the skip again at every level of a chain of prefix operators.
		b.If(Eq(C("opr.count", C("opr.led", r)), Int(0)), func(b *Builder) { b.Return(BoolC(true), BoolC(true)) })
		b.Do("oprExpr.setLastNone", st, Int(-1))
		b.Do("oprExpr.setBound", st, C("opr.nudBound"))
		b.Return(BoolC(false), BoolC(true))
	})
	split("exprLoop", []Param{exprSt}, nil, `exprLoop begins an iteration of the loop of the expression st; the caller
then skips the trivia and runs the longest match of the operators into the record.`, func(b *Builder) {
		b.Do("oprExpr.setM0", st, C("frame.mark"))
	})
	split("exprLed", []Param{P("r", rule), exprSt}, []Type{TBool, TBool, TInt}, `exprLed settles the longest match in the record
st: with done set it ends the expression, with the level of a right operand to parse it returns
that level, and with a negative level the loop goes on.`, func(b *Builder) {
		ret := func(done, ok bool) func(*Builder) {
			return func(b *Builder) { b.Return(BoolC(done), BoolC(ok), Int(-1)) }
		}
		ledStep(b, exprState(), exits{fail: ret(true, false), brk: ret(true, true), next: ret(false, true)}, exprBuild, ex("lhs"))
		b.Return(BoolC(false), BoolC(true), L("rmin"))
	})
	split("exprRhs", []Param{P("r", rule), exprSt, P("rhs", value), P("rok", TBool)}, []Type{TBool, TBool}, `exprRhs ends the
application of an infix operator to its right operand rhs, whose parse reported rok: with done
set it ends the expression, and otherwise the loop goes on.`, func(b *Builder) {
		ret := func(done, ok bool) func(*Builder) {
			return func(b *Builder) { b.Return(BoolC(done), BoolC(ok)) }
		}
		op := b.Let("op", C("opr.op", ex("a")))
		rhsStep(b, exprState(), op, C("opr.level", op), exits{fail: ret(true, false), brk: ret(true, true)}, exprBuild, ex("lhs"))
		b.Return(BoolC(false), BoolC(true))
	})
}
