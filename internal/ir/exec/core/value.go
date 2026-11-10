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
	element := m.AddFunc("element", []Param{act, fn, P("v", datum)}, []Type{elem, failT}, nil, func(b *Builder) {
		kv := b.Let("k", kind(L("v")))
		b.If(Eq(kv, k(ops.KindNil)), func(b *Builder) { b.Return(Null(elem), ok) })
		b.If(isScalar(kv), func(b *Builder) { b.Return(C("val.box", L("c"), L("v")), ok) })
		o := b.LetN([]string{"n", "ok"}, C("node.of", L("v")))
		b.If(o[1], func(b *Builder) { b.Return(C("node.elem", o[0]), ok) })
		b.Return(Null(elem), failure(ops.ErrElement, L("v"), C("val.name", L("fn")), Int(0), Int(0)))
	})
	element.Doc = `element returns v as an element of a list or a tuple that the built-in function or literal fn makes: nil,
a node, a list, a tuple or a struct as it is, and an int, a string or a bool as val.box makes it. Any other value, such
as a function, fails.`

	items := m.AddFunc("items", []Param{fn, P("v", datum)}, []Type{THandle(ops.Seq), failT}, nil, func(b *Builder) {
		it := b.LetN([]string{"items", "ok"}, C("seq.items", L("v")))
		b.If(it[1], func(b *Builder) { b.Return(it[0], ok) })
		b.Return(it[0], failure(ops.ErrArgList, L("v"), C("val.name", L("fn")), Int(0), Int(0)))
	})
	items.Doc = `items returns the elements of v, an argument of the built-in function fn that takes a list: none for nil,
and those of a list or a tuple. Any other value fails.`

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

	logic.Doc = `logic checks the operand x of the logical operator that op indexes in ops.LogicalOps, and returns it. An
operand that is not a bool fails. The caller short-circuits: it evaluates the right operand of && only if the left is
true and of || only if it is false, and checks each operand it evaluates.`
}
