package core

import (
	. "zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// addValue adds the compositions of the operators of terms: the policy of which operands an operator
// accepts, over the primitives of the value component (ops.AddValue), which act on one kind of value
// each. A composition returns its value and, in place of an exception, the failure that val.fail
// made (nil if there is none), which the caller reports as the failure of the term. Every target
// prints them, so the operators mean the same on all.
//
// The logical operators && and || are the composition logic and the sequencing of their caller: the
// right operand is evaluated only when the left one does not decide the result, which needs an operand
// that is not evaluated yet, and Exec IR has no such value. The caller checks each operand it
// evaluates with logic.
func addValue(m *Module) {
	ops.AddValue(m)
	datum, failT := THandle(ops.Datum), THandle(ops.Fail)
	val := []Type{datum, failT}
	l, r, x, op := L("l"), L("r"), L("x"), L("op")
	kind := func(e Expr) Expr { return C("val.kind", e) }
	k := func(n int) Expr { return Int(int64(n)) }
	isKind := func(e Expr, n int) Expr { return Eq(kind(e), k(n)) }
	failure := func(code int, a, b, o, n Expr) Expr { return C("val.fail", k(code), a, b, o, n) }
	none := Null(datum)
	// ok is the failure of a composition that does not fail.
	ok := Null(failT)
	// fail returns the failure code with the values a and b and the operator o; n is a count.
	fail := func(b *Builder, code int, a, bb, o, n Expr) { b.Return(none, failure(code, a, bb, o, n)) }
	opIs := func(c int) Expr { return Eq(op, k(c)) }

	equal := m.AddFunc("equal", []Param{P("l", datum), P("r", datum)}, []Type{TBool, TBool}, nil, func(b *Builder) {
		b.Let("a", C("val.unbox", l))
		b.Let("b", C("val.unbox", r))
		ka, kb := b.Let("ka", kind(L("a"))), b.Let("kb", kind(L("b")))
		// Any value equals nil only if it is nil.
		b.If(Bin("||", Eq(ka, k(ops.KindNil)), Eq(kb, k(ops.KindNil))), func(b *Builder) { b.Return(Eq(ka, kb), BoolC(true)) })
		// Two values of one scalar kind compare; every other pair, two structured values among them, has no equality.
		b.If(Bin("!=", ka, kb), func(b *Builder) { b.Return(BoolC(false), BoolC(false)) })
		b.If(Eq(ka, k(ops.KindInt)), func(b *Builder) { b.Return(Eq(C("int.cmp", L("a"), L("b")), Int(0)), BoolC(true)) })
		b.If(Eq(ka, k(ops.KindString)), func(b *Builder) { b.Return(C("str.eq", L("a"), L("b")), BoolC(true)) })
		b.If(Eq(ka, k(ops.KindBool)), func(b *Builder) {
			b.Return(Eq(C("val.toBool", L("a")), C("val.toBool", L("b"))), BoolC(true))
		})
		b.Return(BoolC(false), BoolC(false))
	})
	equal.Doc = `equal applies == to l and r: it compares two ints, two strings or two bools, and any value with nil,
which equals only nil, a box counting as the value it holds. It returns false for ok when the pair has no
equality, which is every other pair.`

	index := m.AddFunc("index", []Param{P("l", datum), P("r", datum)}, val, nil, func(b *Builder) {
		it := b.LetN([]string{"items", "ok"}, C("seq.items", l))
		b.If(Not(it[1]), func(b *Builder) { fail(b, ops.ErrNotList, l, none, k(ops.OpIndex), Int(0)) })
		b.If(Bin("!=", kind(r), k(ops.KindInt)), func(b *Builder) { fail(b, ops.ErrIndexType, r, none, Int(0), Int(0)) })
		b.Let("n", C("seq.len", it[0]))
		at := b.LetN([]string{"i", "inside"}, C("int.index", r, L("n")))
		b.If(Not(at[1]), func(b *Builder) { fail(b, ops.ErrIndexRange, r, none, Int(0), L("n")) })
		b.Return(C("seq.at", it[0], at[0]), ok)
	})
	index.Doc = `index returns element r of l, a list or a tuple (nil has no elements), with a box opened. It fails if l is
another value, if r is not an int, and if r is not an index of l.`

	binary := m.AddFunc("binary", []Param{P("op", TInt), P("l", datum), P("r", datum)}, val, nil, func(b *Builder) {
		b.If(opIs(ops.OpIndex), func(b *Builder) {
			b.LetN([]string{"v", "e"}, C("index", l, r))
			b.Return(L("v"), L("e"))
		})
		b.If(Bin("||", opIs(ops.OpEq), opIs(ops.OpNe)), func(b *Builder) {
			eq := b.LetN([]string{"eq", "ok"}, C("equal", l, r))
			b.If(Not(eq[1]), func(b *Builder) {
				// A comparison of a value that is not a scalar says why it fails.
				b.Let("code", k(ops.ErrOperands))
				b.If(Bin("||", isKind(C("val.unbox", l), ops.KindOther), isKind(C("val.unbox", r), ops.KindOther)), func(b *Builder) {
					b.Set("code", k(ops.ErrCompare))
				})
				b.Return(none, C("val.fail", L("code"), l, r, op, Int(0)))
			})
			b.Return(C("val.fromBool", Eq(eq[0], opIs(ops.OpEq))), ok)
		})
		// ordered returns the result of the comparison op of two values whose order is c.
		ordered := func(b *Builder, c Expr) {
			for _, o := range []struct {
				code int
				cmp  string
			}{{ops.OpLt, "<"}, {ops.OpLe, "<="}, {ops.OpGt, ">"}} {
				b.If(opIs(o.code), func(b *Builder) { b.Return(C("val.fromBool", Bin(o.cmp, c, Int(0))), ok) })
			}
			b.Return(C("val.fromBool", Bin(">=", c, Int(0))), ok)
		}
		kl, kr := b.Let("kl", kind(l)), b.Let("kr", kind(r))
		b.If(And(Eq(kl, k(ops.KindInt)), Eq(kr, k(ops.KindInt))), func(b *Builder) {
			for _, o := range []struct {
				code int
				fn   string
			}{{ops.OpAdd, "int.add"}, {ops.OpSub, "int.sub"}, {ops.OpMul, "int.mul"}} {
				b.If(opIs(o.code), func(b *Builder) { b.Return(C(o.fn, l, r), ok) })
			}
			b.If(Bin("||", opIs(ops.OpQuo), opIs(ops.OpRem)), func(b *Builder) {
				b.If(C("int.zero", r), func(b *Builder) { fail(b, ops.ErrDivZero, none, none, Int(0), Int(0)) })
				b.If(opIs(ops.OpQuo), func(b *Builder) { b.Return(C("int.quo", l, r), ok) })
				b.Return(C("int.rem", l, r), ok)
			})
			ordered(b, C("int.cmp", l, r))
		})
		b.If(And(Eq(kl, k(ops.KindString)), Eq(kr, k(ops.KindString))), func(b *Builder) {
			b.If(opIs(ops.OpAdd), func(b *Builder) { b.Return(C("str.add", l, r), ok) })
			b.If(Bin(">=", op, k(ops.OpLt)), func(b *Builder) { ordered(b, C("str.cmp", l, r)) })
		})
		fail(b, ops.ErrOperands, l, r, op, Int(0))
	})
	binary.Doc = `binary applies the binary operator that op indexes in ops.BinaryOps to l and r. Ints combine with + - *
/ % and compare with < <= > >=, with 64-bit wraparound; / and % truncate toward zero, and a division by zero fails.
Strings concatenate with + and compare with < <= > >= by code point. == and != apply equal and fail where it has
no equality. [] is index. Any other operands fail.`

	unary := m.AddFunc("unary", []Param{P("op", TInt), P("x", datum)}, val, nil, func(b *Builder) {
		b.If(And(opIs(ops.OpNeg), isKind(x, ops.KindInt)), func(b *Builder) { b.Return(C("int.neg", x), ok) })
		b.If(And(opIs(ops.OpNot), isKind(x, ops.KindBool)), func(b *Builder) {
			b.Return(C("val.fromBool", Not(C("val.toBool", x))), ok)
		})
		fail(b, ops.ErrUnary, x, none, op, Int(0))
	})
	unary.Doc = `unary applies the unary operator that op indexes in ops.UnaryOps to x: - negates an int, with
wraparound, and ! negates a bool. Any other operand fails.`

	logic := m.AddFunc("logic", []Param{P("op", TInt), P("x", datum)}, []Type{TBool, failT}, nil, func(b *Builder) {
		b.If(Bin("!=", kind(x), k(ops.KindBool)), func(b *Builder) {
			b.Return(BoolC(false), failure(ops.ErrLogical, x, none, op, Int(0)))
		})
		b.Return(C("val.toBool", x), ok)
	})
	member := m.AddFunc("member", []Param{P("c", THandle(ops.Act)), P("kind", TInt), P("x", datum), P("name", THandle(ops.Name)),
		P("field", THandle(ops.Name))}, val, nil, func(b *Builder) {
		kindIs := func(c int) Expr { return Eq(L("kind"), k(c)) }
		o := b.LetN([]string{"n", "ok"}, C("node.of", x))
		b.If(Not(o[1]), func(b *Builder) { fail(b, ops.ErrMemberOf, x, C("val.name", L("name")), Int(0), Int(0)) })
		n := o[0]
		// Start, End and Value are members of a node; of any other value they are fields.
		b.If(And(Bin(">=", L("kind"), k(ops.MemberStart)), Bin("<=", L("kind"), k(ops.MemberValue))), func(b *Builder) {
			b.If(C("node.isNode", n), func(b *Builder) {
				b.If(kindIs(ops.MemberStart), func(b *Builder) { b.Return(C("node.start", n), ok) })
				b.If(kindIs(ops.MemberEnd), func(b *Builder) { b.Return(C("node.end", n), ok) })
				b.Return(C("node.value", n), ok)
			})
		})
		st := b.Let("st", C("node.isStruct", n))
		b.If(kindIs(ops.MemberCapture), func(b *Builder) {
			// A struct has no captures; an unmatched capture is nil.
			b.If(st, func(b *Builder) { fail(b, ops.ErrNoCapture, x, C("val.name", L("field")), Int(0), Int(0)) })
			b.LetN([]string{"v", "held"}, C("node.field", n, L("field")))
			b.Return(C("val.unbox", L("v")), ok)
		})
		// A field that is set is read first: a struct holds only the fields of its type.
		f := b.LetN([]string{"f", "set"}, C("node.field", n, L("field")))
		b.If(f[1], func(b *Builder) { b.Return(f[0], ok) })
		b.If(Not(st), func(b *Builder) { b.Return(none, ok) })
		// A field the constructor left unset reads as the zero value of its declared type.
		z := b.LetN([]string{"z", "declared"}, C("node.zero", L("c"), n, L("field")))
		b.If(z[1], func(b *Builder) { b.Return(z[0], ok) })
		fail(b, ops.ErrNoField, x, C("val.name", L("field")), Int(0), Int(0))
	})
	member.Doc = `member reads the member name of x, whose kind is kind (ops.MemberKinds) and which reads the field or
capture field. Start, End and Value of a node read its range and value; a capture of a node reads it with a box
opened, nil if it did not match; a field reads what the value holds, and of a struct the zero value of the declared
type of a field that is not set. A value that has no members, a capture of a struct and a field that a struct does
not declare fail.`

	fn, act := P("fn", THandle(ops.Name)), P("c", THandle(ops.Act))
	isScalar := func(e Expr) Expr {
		return Bin("||", Bin("||", Eq(e, k(ops.KindInt)), Eq(e, k(ops.KindString))), Eq(e, k(ops.KindBool)))
	}
	elem := THandle(ops.Elem)
	push := m.AddFunc("push", []Param{act, fn, P("v", datum)}, []Type{failT}, nil, func(b *Builder) {
		kv := b.Let("k", kind(L("v")))
		b.If(Eq(kv, k(ops.KindNil)), func(b *Builder) {
			b.Do("act.push", L("c"), Null(elem))
			b.Return(ok)
		})
		b.If(isScalar(kv), func(b *Builder) {
			b.Do("act.push", L("c"), C("val.box", L("c"), L("v")))
			b.Return(ok)
		})
		o := b.LetN([]string{"n", "ok"}, C("node.of", L("v")))
		b.If(o[1], func(b *Builder) {
			b.Do("act.push", L("c"), C("node.elem", o[0]))
			b.Return(ok)
		})
		b.Return(failure(ops.ErrElement, L("v"), C("val.name", L("fn")), Int(0), Int(0)))
	})
	push.Doc = `push pushes v as the next element of the list or the tuple that the built-in function or literal fn makes in c:
nil, a node, a list, a tuple or a struct as it is, and an int, a string or a bool as val.box makes it. Any other
value, such as a function, fails and pushes nothing.`

	pushItems := m.AddFunc("pushItems", []Param{act, fn, P("v", datum)}, []Type{failT}, nil, func(b *Builder) {
		it := b.LetN([]string{"items", "ok"}, C("seq.items", L("v")))
		b.If(it[1], func(b *Builder) {
			b.Do("act.pushAll", L("c"), it[0])
			b.Return(ok)
		})
		b.Return(failure(ops.ErrArgList, L("v"), C("val.name", L("fn")), Int(0), Int(0)))
	})
	pushItems.Doc = `pushItems pushes the elements of v, an argument of the built-in function fn that concatenates lists, as the
next elements of the list that fn makes in c: none for nil, and those of a list or a tuple as they are. Any other value
fails and pushes nothing.`

	items := m.AddFunc("items", []Param{fn, P("v", datum)}, []Type{THandle(ops.Seq), failT}, nil, func(b *Builder) {
		it := b.LetN([]string{"items", "ok"}, C("seq.items", L("v")))
		b.If(it[1], func(b *Builder) { b.Return(it[0], ok) })
		b.Return(it[0], failure(ops.ErrArgList, L("v"), C("val.name", L("fn")), Int(0), Int(0)))
	})
	items.Doc = `items returns the elements of v, an argument of the built-in function fn that takes a list: none for nil,
and those of a list or a tuple. Any other value fails.`

	seqT := THandle(ops.Seq)
	itemAt := m.AddFunc("itemAt", []Param{P("items", seqT), P("i", TInt), P("right", TBool)}, []Type{datum}, nil, func(b *Builder) {
		b.If(L("right"), func(b *Builder) {
			b.Return(C("seq.at", L("items"), Bin("-", Bin("-", C("seq.len", L("items")), Int(1)), L("i"))))
		})
		b.Return(C("seq.at", L("items"), L("i")))
	})
	itemAt.Doc = `itemAt returns the element that a built-in function iterating over items (map, filter, foldl, foldr) passes to
its function at step i, with a box opened: element i, or element i from the end if right (foldr). It charges
nothing; the loop charges the step (seq.step) before it, so that itemAt stays small enough for a target to inline.`

	filterKeep := m.AddFunc("filterKeep", []Param{P("v", datum)}, []Type{TBool, failT}, nil, func(b *Builder) {
		b.If(Bin("!=", kind(L("v")), k(ops.KindBool)), func(b *Builder) {
			b.Return(BoolC(false), failure(ops.ErrFilter, L("v"), none, Int(0), Int(0)))
		})
		b.Return(C("val.toBool", L("v")), ok)
	})
	filterKeep.Doc = `filterKeep returns whether filter keeps an element for which its function returned v. A v that is not a bool fails.`

	length := m.AddFunc("length", []Param{P("x", datum)}, val, nil, func(b *Builder) {
		kx := b.Let("k", kind(x))
		b.If(Eq(kx, k(ops.KindString)), func(b *Builder) { b.Return(C("str.len", x), ok) })
		b.If(Eq(kx, k(ops.KindNil)), func(b *Builder) { b.Return(C("val.fromInt", Int(0)), ok) })
		o := b.LetN([]string{"n", "ok"}, C("node.of", x))
		b.If(o[1], func(b *Builder) {
			// The text of a string node is the input of its range, whose length the range holds in the
			// position unit.
			b.If(C("node.hasText", o[0]), func(b *Builder) { b.Return(C("node.width", o[0]), ok) })
			b.Return(C("node.count", o[0]), ok)
		})
		fail(b, ops.ErrLenArg, x, none, Int(0), Int(0))
	})
	length.Doc = `length is the built-in function len: the length of a string in the position unit, of the range of a string
node, and the number of elements of a list, a tuple or another node; nil has length 0. Any other value fails.`

	text := m.AddFunc("text", []Param{P("x", datum)}, []Type{THandle(ops.Str), failT}, nil, func(b *Builder) {
		kx := b.Let("k", kind(x))
		b.If(Eq(kx, k(ops.KindNil)), func(b *Builder) { b.Return(C("str.empty"), ok) })
		b.If(Eq(kx, k(ops.KindString)), func(b *Builder) { b.Return(C("str.of", x), ok) })
		o := b.LetN([]string{"n", "ok"}, C("node.of", x))
		b.If(o[1], func(b *Builder) {
			// A string node or an error holds its text; the text of any other node is the input of its
			// range.
			h := b.LetN([]string{"t", "held"}, C("node.heldText", o[0]))
			b.If(h[1], func(b *Builder) { b.Return(h[0], ok) })
			b.Return(C("node.rangeText", o[0]), ok)
		})
		b.Return(C("str.empty"), failure(ops.ErrTextArg, x, none, Int(0), Int(0)))
	})
	text.Doc = `text is the built-in function text: a string itself, the text that a string node or an error holds,
the input of the range of any other node, and the empty string for nil. Any other value fails.`

	convert := m.AddFunc("convert", []Param{P("t", THandle(ops.NodeType)), P("x", datum)}, val, nil, func(b *Builder) {
		o := b.LetN([]string{"n", "ok"}, C("node.of", x))
		b.If(o[1], func(b *Builder) {
			b.If(C("node.hasBase", o[0]), func(b *Builder) { b.Return(C("node.convert", o[0], L("t")), ok) })
		})
		// A box holds an int, a string or a bool, which has no base, and the failure names the type of
		// the value it holds.
		fail(b, ops.ErrConvert, x, C("val.name", C("node.typeName", L("t"))), Int(0), Int(0))
	})
	convert.Doc = `convert is the conversion T{x} to the named node type t: a string, tuple or list node, of a named node
type or not, becomes a node of t. Any other value fails.`

	cover := m.AddFunc("cover", []Param{P("n", THandle(ops.Obj)), P("v", datum), P("first", TBool)}, []Type{TBool}, nil, func(b *Builder) {
		o := b.LetN([]string{"k", "ok"}, C("node.of", L("v")))
		// A field value that is not a node, nil among them, adds nothing to the range.
		b.If(Not(o[1]), func(b *Builder) { b.Return(L("first")) })
		c := b.LetN([]string{"cs", "ce"}, C("node.span", o[0]))
		w := b.LetN([]string{"ns", "ne"}, C("node.span", L("n")))
		b.Let("lo", w[0])
		b.Let("hi", w[1])
		b.If(Bin("||", L("first"), Bin("<", c[0], w[0])), func(b *Builder) { b.Set("lo", c[0]) })
		b.If(Bin("||", L("first"), Bin(">", c[1], w[1])), func(b *Builder) { b.Set("hi", c[1]) })
		b.Do("node.setSpan", L("n"), L("lo"), L("hi"))
		b.Return(BoolC(false))
	})
	cover.Doc = `cover widens the range of the struct n, which a term is making, to cover the range of the field value v if
v is a node, a list, a tuple or a struct, and returns whether n still has no node among its field values: first is
whether it had none before, and the first node gives n its range.`

	made := m.AddFunc("made", []Param{P("c", THandle(ops.Act)), P("n", THandle(ops.Obj)), P("first", TBool)}, nil, nil, func(b *Builder) {
		// A struct with no field value that is a node takes the range of the rule.
		b.If(L("first"), func(b *Builder) {
			r := b.LetN([]string{"rs", "re"}, C("act.span", L("c")))
			b.Do("node.setSpan", L("n"), r[0], r[1])
		})
		b.Do("act.made", L("c"), L("n"))
	})
	made.Doc = `made finishes the struct n that c made of field values that cover has gone over: n takes the range of the
rule if no field value is a node, and c records n.`

	hull := m.AddFunc("hull", []Param{P("c", THandle(ops.Act)), P("tuple", TBool), P("kids", THandle(ops.Seq))}, []Type{THandle(ops.Obj)}, nil, func(b *Builder) {
		b.Let("n", C("val.seq", L("c"), L("tuple"), L("kids")))
		// The range starts empty at the start of the rule, and the first element that adds a range gives
		// it that range.
		r := b.LetN([]string{"rs", "re"}, C("act.span", L("c")))
		b.Let("lo", r[0])
		b.Let("hi", r[0])
		b.Let("first", BoolC(true))
		b.Let("count", C("seq.len", L("kids")))
		b.Var("j", TInt)
		b.For("kids", Assign("j", Int(0)), Lt(L("j"), L("count")), Assign("j", Add(L("j"), Int(1))), func(b *Builder) {
			b.Do("seq.step")
			e := b.LetN([]string{"es", "ee", "has"}, C("seq.span", L("c"), L("kids"), L("j")))
			b.If(e[2], func(b *Builder) {
				b.If(Bin("||", L("first"), Bin("<", e[0], L("lo"))), func(b *Builder) { b.Set("lo", e[0]) })
				b.If(Bin("||", L("first"), Bin(">", e[1], L("hi"))), func(b *Builder) { b.Set("hi", e[1]) })
				b.Set("first", BoolC(false))
			})
		})
		b.Do("node.setSpan", L("n"), L("lo"), L("hi"))
		b.Return(L("n"))
	})
	hull.Doc = `hull makes the tuple value, if tuple, or the list value of the elements kids in c, which it takes over, and
gives it its range: the one that covers the ranges that the elements add (seq.span), and the empty range at the start
of the rule if none adds one. It charges a step for each element. The value model does not show the range of a list or a tuple, but a
struct that holds one covers it.`

	// The cast: the check of a value against a type, over the accessors of the type (ftype.*) and the
	// primitives of the values it checks.
	ftype := THandle(ops.FType)
	t, v, n := L("t"), L("v"), L("n")
	shape := L("shape")
	shapeIs := func(c int) Expr { return Eq(shape, k(c)) }
	// forAlts returns whether pred holds of an alternative of the join t, and false if it holds of none.
	forAlts := func(b *Builder, pred func(alt Expr) Expr) {
		b.Let("count", C("ftype.count", t))
		b.Var("i", TInt)
		b.For("alts", Assign("i", Int(0)), Lt(L("i"), L("count")), Assign("i", Add(L("i"), Int(1))), func(b *Builder) {
			b.If(pred(C("ftype.alt", t, L("i"))), func(b *Builder) { b.Return(BoolC(true)) })
		})
		b.Return(BoolC(false))
	}
	elems := m.AddFunc("elemsConform", []Param{P("t", ftype), P("items", seqT)}, []Type{TBool}, nil, func(b *Builder) {
		b.If(Eq(C("ftype.kind", t), k(ops.FtTop)), func(b *Builder) { b.Return(BoolC(true)) })
		b.Let("count", C("seq.len", L("items")))
		b.Var("j", TInt)
		b.For("elems", Assign("j", Int(0)), Lt(L("j"), L("count")), Assign("j", Add(L("j"), Int(1))), func(b *Builder) {
			b.Do("seq.step")
			b.If(Not(C("conforms", t, C("seq.at", L("items"), L("j")))), func(b *Builder) { b.Return(BoolC(false)) })
		})
		b.Return(BoolC(true))
	})
	elems.Doc = `elemsConform returns whether every element of items, the elements of a list, is of the element type t
of the list, charging a step for each element it examines; the type _ takes every element, and charges nothing.`
	tuple := m.AddFunc("tupleConform", []Param{P("t", ftype), P("items", seqT)}, []Type{TBool}, nil, func(b *Builder) {
		b.Let("count", C("seq.len", L("items")))
		b.If(Bin("!=", L("count"), C("ftype.count", t)), func(b *Builder) { b.Return(BoolC(false)) })
		b.Var("j", TInt)
		b.For("elems", Assign("j", Int(0)), Lt(L("j"), L("count")), Assign("j", Add(L("j"), Int(1))), func(b *Builder) {
			b.Do("seq.step")
			b.If(Not(C("conforms", C("ftype.alt", t, L("j")), C("seq.at", L("items"), L("j")))), func(b *Builder) { b.Return(BoolC(false)) })
		})
		b.Return(BoolC(true))
	})
	tuple.Doc = `tupleConform returns whether the elements items of a tuple are of the element types of the tuple type t:
as many, and each of its own type, charging a step for each element it examines.`
	valueConforms := m.AddFunc("valueConforms", []Param{P("t", ftype), P("n", THandle(ops.Obj)), P("shape", TInt)}, []Type{TBool}, nil, func(b *Builder) {
		kt := b.Let("kt", C("ftype.kind", t))
		isKind := func(c int) Expr { return Eq(kt, k(c)) }
		b.Comment("A node of a type whose value is a string, a tuple or a list.")
		b.If(isKind(ops.FtTop), func(b *Builder) {
			b.Return(Bin("||", Bin("||", shapeIs(ops.ShapeString), shapeIs(ops.ShapeTuple)), shapeIs(ops.ShapeList)))
		})
		b.If(isKind(ops.FtString), func(b *Builder) { b.Return(shapeIs(ops.ShapeString)) })
		b.If(isKind(ops.FtList), func(b *Builder) {
			b.If(Not(shapeIs(ops.ShapeList)), func(b *Builder) { b.Return(BoolC(false)) })
			b.Return(C("elemsConform", C("ftype.elem", t), C("node.children", n)))
		})
		b.If(isKind(ops.FtTuple), func(b *Builder) {
			b.If(Not(shapeIs(ops.ShapeTuple)), func(b *Builder) { b.Return(BoolC(false)) })
			b.Return(C("tupleConform", t, C("node.children", n)))
		})
		// A node always has a value.
		b.If(isKind(ops.FtOptional), func(b *Builder) { b.Return(C("valueConforms", C("ftype.elem", t), n, shape)) })
		b.If(isKind(ops.FtJoin), func(b *Builder) {
			forAlts(b, func(alt Expr) Expr { return C("valueConforms", alt, n, shape) })
		})
		// No node has a value of another type yet.
		b.Return(BoolC(false))
	})
	valueConforms.Doc = `valueConforms returns whether the value of the node n, whose shape is shape, is of the type t: a string
node has a string, and a tuple node or a list node its elements as value.`
	conforms := m.AddFunc("conforms", []Param{P("t", ftype), P("v", datum)}, []Type{TBool}, nil, func(b *Builder) {
		kt := b.Let("kt", C("ftype.kind", t))
		isKind := func(c int) Expr { return Eq(kt, k(c)) }
		// A box stands for the value it holds, and a nil node for nil.
		b.Let("x", C("val.unbox", v))
		kx := b.Let("kx", kind(L("x")))
		b.Comment("The type _ takes every value.")
		b.If(isKind(ops.FtTop), func(b *Builder) { b.Return(BoolC(true)) })
		b.Comment("The scalar types and nil take the value of their kind.")
		for _, s := range []struct{ ft, kind int }{{ops.FtInt, ops.KindInt}, {ops.FtString, ops.KindString}, {ops.FtBool, ops.KindBool}, {ops.FtNil, ops.KindNil}} {
			b.If(isKind(s.ft), func(b *Builder) { b.Return(Eq(kx, k(s.kind))) })
		}
		b.Comment("An optional type takes nil and the values of its type; a join the values of an alternative.")
		b.If(isKind(ops.FtOptional), func(b *Builder) {
			b.Return(Bin("||", Eq(kx, k(ops.KindNil)), C("conforms", C("ftype.elem", t), L("x"))))
		})
		b.If(isKind(ops.FtJoin), func(b *Builder) {
			forAlts(b, func(alt Expr) Expr { return C("conforms", alt, L("x")) })
		})
		b.Comment("Every other type takes a node, a list value, a tuple value or a struct.")
		o := b.LetN([]string{"n", "ok"}, C("node.of", L("x")))
		b.If(Not(o[1]), func(b *Builder) { b.Return(BoolC(false)) })
		b.Let("shape", C("node.shape", n))
		b.Comment("node[_] takes a node whose value is a string, a tuple or a list; node[string] one whose value is a string.")
		b.If(isKind(ops.FtNode), func(b *Builder) {
			b.Return(Bin("||", Bin("||", shapeIs(ops.ShapeString), shapeIs(ops.ShapeTuple)), shapeIs(ops.ShapeList)))
		})
		b.If(isKind(ops.FtStringNode), func(b *Builder) { b.Return(shapeIs(ops.ShapeString)) })
		b.Comment("error takes an error value, and missing the error value of a missing token.")
		b.If(isKind(ops.FtError), func(b *Builder) { b.Return(shapeIs(ops.ShapeError)) })
		b.If(isKind(ops.FtMissing), func(b *Builder) { b.Return(And(shapeIs(ops.ShapeError), C("node.isMissing", n))) })
		b.Comment("A struct type or a named node type takes the values that have its name.")
		b.If(isKind(ops.FtNamed), func(b *Builder) { b.Return(C("node.isType", n, t)) })
		b.Comment("A list type takes a list value, and a tuple type a tuple value, whose elements are of its types.")
		b.If(isKind(ops.FtList), func(b *Builder) {
			b.If(Not(shapeIs(ops.ShapeListValue)), func(b *Builder) { b.Return(BoolC(false)) })
			b.Return(C("elemsConform", C("ftype.elem", t), C("node.children", n)))
		})
		b.If(isKind(ops.FtTuple), func(b *Builder) {
			b.If(Not(shapeIs(ops.ShapeTupleValue)), func(b *Builder) { b.Return(BoolC(false)) })
			b.Return(C("tupleConform", t, C("node.children", n)))
		})
		b.Comment("node[T] takes a node whose value is of type T.")
		b.If(isKind(ops.FtNodeOf), func(b *Builder) { b.Return(C("valueConforms", C("ftype.elem", t), n, shape)) })
		b.Return(BoolC(false))
	})
	conforms.Doc = `conforms returns whether v is a value of the type t of a cast in the value model, charging a step for each
element of a list or a tuple it examines. A box counts as the value it holds. node[T] is a node whose value is of type T
(node[_] any node with a value), and error an error value; a list or tuple type takes a list value or a tuple value,
not the node of a repetition or a sequence.`

	logic.Doc = `logic checks the operand x of the logical operator that op indexes in ops.LogicalOps, and returns it. An
operand that is not a bool fails. The caller short-circuits: it evaluates the right operand of && only if the left is
true and of || only if it is false, and checks each operand it evaluates.`
}
