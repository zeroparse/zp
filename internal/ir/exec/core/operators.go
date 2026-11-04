package core

import (
	. "zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// addOperators adds the compositions of the Operators Rule loop: the longest match of an operator,
// the trivia between operands and operators, and the loop of binding powers with its prefix
// operators and operands. The node and the TypeScript runtimes print the forms without typed
// values; the typed runtime prints operatorsParseTyped, operatorsExprTyped and operatorsNudTyped,
// whose values travel with where they start (opr.buildTyped).
func addOperators(m *Module) {
	rule, value := THandle(ops.Rule), THandle(ops.Value)
	attempt, list := THandle(ops.Attempt), THandle(ops.Operators)
	r := L("r")
	charge := func(b *Builder) {
		b.If(F(ops.Budget), func(b *Builder) { b.Do("budget.charge", U64(1)) })
	}
	fail := func(b *Builder, typed bool) {
		if typed {
			b.Return(Null(value), Int(0), BoolC(false))
			return
		}
		b.Return(Null(value), BoolC(false))
	}
	// framed runs the line l in a frame of its own, with the cut flag of its own scope, and binds
	// its value, success, frame and cut flag to the locals v, ok, f and cut.
	framed := func(b *Builder, l Expr, ok string) {
		b.Let("prevFrame", C("frame.current"))
		b.Let("prevCut", R(ops.Cut))
		b.Let("f", C("frame.open", C("opr.scope", l)))
		b.Do("frame.use", L("f"))
		b.SetReg(ops.Cut, BoolC(false))
		b.LetN([]string{"v", ok}, C("opr.match", l))
		b.Let("cut", R(ops.Cut))
		b.Do("frame.use", L("prevFrame"))
		b.SetReg(ops.Cut, L("prevCut"))
	}
	// apply moves the parse to the end of the match a, with its environment and its recovered
	// errors.
	apply := func(b *Builder, a Expr) {
		b.SetReg(ops.Pos, C("opr.end", a))
		b.Do("env.restore", C("opr.env", a))
		b.If(Not(C("rec.none", C("opr.errs", a))), func(b *Builder) { b.Do("rec.add", C("opr.errs", a)) })
	}

	reset := m.AddFunc("reset", []Param{P("m", THandle(ops.Backtrack))}, nil, nil, func(b *Builder) {
		mk := L("m")
		b.SetReg(ops.Pos, C("frame.markPos", mk))
		b.Do("env.restore", C("frame.markEnv", mk))
		b.Do("rec.truncate", C("frame.markRec", mk))
		b.Do("frame.undo", C("frame.markTrail", mk))
	})
	reset.Doc = `reset returns to the backtracking point m: the position, the variable environment, the recovered
errors and the captures (undoing the trail) become what they were when m was taken.`

	skip := m.AddFunc("skipTrivia", []Param{P("r", rule)}, nil, nil, func(b *Builder) {
		b.If(Not(C("opr.hasSkip", r)), func(b *Builder) { b.Return() })
		b.Let("m", C("frame.mark"))
		b.SetReg(ops.Silent, Add(R(ops.Silent), Int(1)))
		b.Let("ok", C("opr.skip", r))
		b.SetReg(ops.Silent, Bin("-", R(ops.Silent), Int(1)))
		b.IfElse(Not(L("ok")), func(b *Builder) { b.Do("reset", L("m")) }, func(b *Builder) {
			b.Do("rec.truncate", C("frame.markRec", L("m")))
		})
	})
	skip.Doc = `skipTrivia skips the trivia of the Operators Rule r, if it has any, in a lookahead: a failed skip
leaves the parse as it was, and one that matches discards the errors recovered in it, as a
lookahead does.`

	longest := m.AddFunc("longest", []Param{P("l", list)}, []Type{attempt}, []string{ops.Budget}, func(b *Builder) {
		b.Let("m0", C("frame.mark"))
		start := b.Let("start", C("frame.markPos", L("m0")))
		b.Var("best", attempt)
		b.Var("cutFailed", TBool)
		b.Var("i", TInt)
		b.For("each", Assign("i", Int(0)), Lt(L("i"), C("opr.count", L("l"))), Assign("i", Add(L("i"), Int(1))), func(b *Builder) {
			charge(b)
			op := b.Let("op", C("opr.at", L("l"), L("i")))
			framed(b, C("opr.line", op), "ok")
			// An empty prefix or postfix match could be applied forever.
			b.Let("matched", And(L("ok"), Not(And(Eq(R(ops.Pos), start), Not(C("opr.infix", op))))))
			b.If(And(L("matched"), Bin("||", Not(C("opr.found", L("best"))), Bin(">", R(ops.Pos), C("opr.end", L("best"))))), func(b *Builder) {
				b.Set("best", C("opr.attempt", op, L("f"), L("v"), start, L("cut"), C("rec.since", C("frame.markRec", L("m0")))))
			})
			b.If(And(Not(L("matched")), L("cut")), func(b *Builder) { b.Set("cutFailed", BoolC(true)) })
			b.If(Eq(C("frame.slots", L("f")), Int(0)), func(b *Builder) {
				b.Do("reset", L("m0"))
				b.Continue("each")
			})
			// The reset undoes the captures of this candidate; they are restored. Nothing parses
			// between saving and restoring them.
			b.Do("frame.save", L("f"))
			b.Do("reset", L("m0"))
			b.Do("frame.restore", L("f"))
		})
		b.Do("opr.setCutFailed", L("cutFailed"))
		b.Return(L("best"))
	})
	longest.Doc = `longest tries every operator of l from the position and returns the longest match, the first
declared among equals, or the null attempt; it records whether a candidate failed after a cut in the
parser's cutFailed field (p.cutFailed in Go, this.cutFailed in TypeScript). The parse is left at the
position, with the captures of each candidate in its frame.`

	for _, typed := range []bool{false, true} {
		suffix, feats := "", []string{ops.Budget}
		results := []Type{value, TBool}
		if typed {
			suffix, feats = "Typed", []string{ops.Budget, ops.Typed}
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
			if typed {
				b.LetN([]string{"lhs", "lstart", "ok"}, C(nudName, r))
			} else {
				b.LetN([]string{"lhs", "ok"}, C(nudName, r))
			}
			b.If(Not(L("ok")), func(b *Builder) { b.Return(Null(value), BoolC(false)) })
			b.Let("lastNone", Int(-1))
			b.Let("led", C("opr.led", r))
			b.Loop("loop", func(b *Builder) {
				b.Let("m0", C("frame.mark"))
				start := b.Let("start", C("frame.markPos", L("m0")))
				b.Do("skipTrivia", r)
				a := b.Let("a", C("longest", L("led")))
				b.If(Not(C("opr.found", a)), func(b *Builder) {
					b.Let("failed", C("opr.cutFailed"))
					b.Do("reset", L("m0"))
					b.If(L("failed"), func(b *Builder) { b.Return(Null(value), BoolC(false)) })
					b.Break("loop")
				})
				op := b.Let("op", C("opr.op", a))
				level := b.Let("level", C("opr.level", op))
				b.If(Bin("||", Bin("<=", level, L("min")), And(C("opr.nonAssoc", op), Eq(level, L("lastNone")))), func(b *Builder) {
					b.Do("reset", L("m0"))
					b.Break("loop")
				})
				apply(b, a)
				b.If(C("opr.postfix", op), func(b *Builder) {
					build(b, L("lhs"), Null(value))
					b.Set("lastNone", Int(-1))
					b.If(Eq(R(ops.Pos), start), func(b *Builder) { b.Break("loop") })
					b.Continue("loop")
				})
				b.Let("rmin", level)
				b.If(C("opr.right", op), func(b *Builder) { b.Set("rmin", Bin("-", L("rmin"), Int(1))) })
				b.LetN([]string{"rhs", "rok"}, C(parseName, r, L("rmin")))
				b.If(Not(L("rok")), func(b *Builder) {
					b.If(C("opr.cut", a), func(b *Builder) { b.Return(Null(value), BoolC(false)) })
					b.Do("reset", L("m0"))
					b.Break("loop")
				})
				build(b, L("lhs"), L("rhs"))
				b.Set("lastNone", Int(-1))
				b.If(C("opr.nonAssoc", op), func(b *Builder) { b.Set("lastNone", level) })
				// An application that consumed no input ends the loop, as an empty iteration ends a
				// repetition: otherwise an operator that can match empty after an operand that can
				// loops forever.
				b.If(Eq(R(ops.Pos), start), func(b *Builder) { b.Break("loop") })
			})
			b.Return(L("lhs"), BoolC(true))
		})
		expr.Doc = exprName + ` parses an operand, or a prefix application, and then applies the operators that
follow it while they bind tighter than min: the longest operator match wins, a non-associative
operator does not apply again at its own level, a right-associative one parses its right operand at
one level less, and a failed right operand ends the loop before the operator unless the operator cut.`

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
				b.If(L("rok"), func(b *Builder) {
					if typed {
						b.LetN([]string{"x", "xstart"}, C("opr.buildTyped", r, a, Null(value), Int(0), L("rhs")))
						b.Return(L("x"), L("xstart"), BoolC(true))
						return
					}
					b.Return(C("opr.build", r, a, Null(value), L("rhs")), BoolC(true))
				})
				b.If(C("opr.cut", a), func(b *Builder) { fail(b, typed) })
				b.Do("reset", L("m0"))
			}, func(b *Builder) {
				b.If(C("opr.cutFailed"), func(b *Builder) { fail(b, typed) })
			})
			b.Var("i", TInt)
			b.For("each", Assign("i", Int(0)), Lt(L("i"), C("opr.operands", r)), Assign("i", Add(L("i"), Int(1))), func(b *Builder) {
				charge(b)
				o := b.Let("o", C("opr.operand", r, L("i")))
				framed(b, o, "ok")
				b.If(L("ok"), func(b *Builder) {
					if typed {
						b.Return(C("opr.lineValue", r, o, L("f"), L("v"), start), start, BoolC(true))
						return
					}
					b.Return(C("opr.lineValue", r, o, L("f"), L("v"), start), BoolC(true))
				})
				b.Do("reset", L("m0"))
				b.If(L("cut"), func(b *Builder) { fail(b, typed) })
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
}
