package conform

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/exec"
	"zeroparse.dev/zp/internal/ir/code/exec/ref"
)

// The harness of what terms make (exec.Gathered and exec.Made): the operations that conformance cases use
// to make the context, the elements and the structs that the operations take, and the comparison of the
// elements gathered and the values made after each call that writes them.

// The conformance operations of what terms make. They are not operations of the parser VM.
const (
	// ActOp returns the context of the terms of a case (exec.Act); every call returns the same one.
	ActOp = "test.act"
	// ElemOp returns a new element (exec.Elem) with a conformance identifier.
	ElemOp = "test.elem"
	// ItemsOp returns the elements of a new list (exec.Seq), n new elements with the identifiers from
	// first on.
	ItemsOp = "test.items"
	// StructOp returns a new struct (exec.Obj) with a conformance identifier.
	StructOp = "test.struct"
)

func isTermStateTestOp(name string) bool {
	return name == ActOp || name == ElemOp || name == ItemsOp || name == StructOp
}

// Gathering is an Impl that provides the elements gathered for the lists and tuples that terms make
// (exec.Gathered). After each call of an operation that writes them, the harness compares them with the
// model's.
type Gathering interface {
	// Act returns the context of the terms of a case.
	Act() any
	// Elem returns a new element with the identifier id.
	Elem(id int64) any
	// Items returns a new list of new elements with the identifiers ids, as a handle that the harness can
	// compare by identity.
	Items(ids []int64) any
	// Gathered returns the identifiers of the elements gathered, from the bottom of the stack, 0 for a
	// null element.
	Gathered() []int64
	// ItemIDs returns the identifiers of the elements of items, a list that term.elemsAbove returned,
	// 0 for a null element.
	ItemIDs(items any) []int64
}

// Making is an Impl that provides the values that terms made (exec.Made). After each call of an operation
// that writes them, the harness compares them with the model's.
type Making interface {
	// Act returns the context of the terms of a case.
	Act() any
	// Struct returns a new struct with the identifier id.
	Struct(id int64) any
	// Made returns the identifiers of the structs made, in the order they were recorded.
	Made() []int64
}

// termStateHandle runs a conformance operation of what terms make on both sides.
func (p *pair) termStateHandle(name string, args []any) any {
	switch name {
	case ActOp:
		if g, ok := p.impl.(Gathering); ok {
			return p.wrap(ref.Act{}, g.Act())
		}
		return p.wrap(ref.Act{}, p.impl.(Making).Act())
	case ElemOp:
		id := args[0].(int64)
		return p.wrap(ref.Elem{ID: id}, p.impl.(Gathering).Elem(id))
	case ItemsOp:
		ids := make([]int64, args[1].(int64))
		for i := range ids {
			ids[i] = args[0].(int64) + int64(i)
		}
		return p.wrap(&ref.Items{IDs: ids}, p.impl.(Gathering).Items(ids))
	}
	id := args[0].(int64)
	return p.wrap(ref.Struct{ID: id}, p.impl.(Making).Struct(id))
}

// compareTermState compares the elements gathered and the values made after a call of o that writes them,
// and the elements that term.elemsAbove returned: want are the model's results and got the
// implementation's.
func (p *pair) compareTermState(o *code.Op, want, got []any) {
	if g, ok := p.impl.(Gathering); ok && o.Name == "term.elemsAbove" {
		if w, x := want[0].(*ref.Items).IDs, g.ItemIDs(got[0]); !slices.Equal(w, x) {
			panic(mismatch(fmt.Sprintf("elements taken off: model %v, implementation %v", w, x)))
		}
	}
	if g, ok := p.impl.(Gathering); ok && slices.Contains(o.Writes, exec.Gathered) {
		if w, x := p.model.Gathered(), g.Gathered(); !slices.Equal(w, x) {
			panic(mismatch(fmt.Sprintf("elements gathered: model %v, implementation %v", w, x)))
		}
	}
	if mk, ok := p.impl.(Making); ok && slices.Contains(o.Writes, exec.Made) {
		if w, x := p.model.Made(), mk.Made(); !slices.Equal(w, x) {
			panic(mismatch(fmt.Sprintf("values made: model %v, implementation %v", w, x)))
		}
	}
}

// TermStateModule returns the value declarations with the conformance operations of what terms make.
func TermStateModule() *code.Module {
	m := exec.Parser()
	m.AddOp(&code.Op{Name: ActOp, Component: exec.Host, Results: []code.Type{code.THandle(exec.Act)},
		Contract: "Returns the context of the terms of the case; every call returns the same one."})
	m.AddOp(&code.Op{Name: ElemOp, Component: exec.Host, Params: []code.Param{code.P("id", code.TInt)},
		Results: []code.Type{code.THandle(exec.Elem)}, Contract: "Returns a new element with a conformance identifier."})
	m.AddOp(&code.Op{Name: ItemsOp, Component: exec.Host, Params: []code.Param{code.P("first", code.TInt), code.P("n", code.TInt)},
		Results:  []code.Type{code.THandle(exec.Seq)},
		Contract: "Returns the elements of a new list: n new elements with the conformance identifiers from first on."})
	m.AddOp(&code.Op{Name: StructOp, Component: exec.Host, Params: []code.Param{code.P("id", code.TInt)},
		Results: []code.Type{code.THandle(exec.Obj)}, Contract: "Returns a new struct with a conformance identifier."})
	return m
}

// GatheredOps are the operations of the elements gathered, which the cases of the family cover.
var GatheredOps = []string{"act.push", "act.pushAll", "term.kids", "term.dropKids", "term.elemsAbove", "term.keep"}

// MadeOps are the operations of the values made, which the cases of the family cover.
var MadeOps = []string{"act.made", "term.created", "term.dropCreated"}

// termStateFeatures are the features of the functions of the cases: none of the operations depends on one.
var termStateFeatures = []string{"budget", "stream", "document", "missing"}

// GatheredCases returns the handwritten conformance cases of the elements gathered, in one module: the
// elements of nested lists pushed one by one, from a list and kept from a list, null elements, the elements
// of a list or a tuple taken off when it is made, and drops to marks, of nothing among them, as the predicate
// and the binding drop what a failed term gathered.
func GatheredCases() []Case {
	m := TermStateModule()
	var cs []Case
	add := func(name string, body func(b *code.Builder, c code.Expr)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, termStateFeatures, func(b *code.Builder) {
			body(b, b.Let("c", code.C(ActOp)))
		})})
	}
	i := code.Int
	kids := func(n int64) code.Expr { return code.Eq(code.C("term.kids"), i(n)) }
	push := func(b *code.Builder, c code.Expr, id int64) { b.Do("act.push", c, code.C(ElemOp, i(id))) }
	null := code.Null(code.THandle(exec.Elem))
	add("gathered_push", func(b *code.Builder, c code.Expr) {
		k0 := b.Let("k0", code.C("term.kids"))
		b.Assert(kids(0), "a parse starts with no elements gathered")
		push(b, c, 1)
		b.Do("act.push", c, null)
		b.Do("act.pushAll", c, code.C(ItemsOp, i(10), i(3)))
		b.Assert(kids(5), "each element counts, a null one and those of a list among them")
		k1 := b.Let("k1", code.C("term.kids"))
		push(b, c, 2)
		b.Do("act.pushAll", c, code.C(ItemsOp, i(20), i(2)))
		b.Do("term.dropKids", k1)
		b.Assert(kids(5), "a drop keeps the elements up to its mark")
		b.Do("term.dropKids", k0)
		b.Assert(kids(0), "and drops those after it")
	})
	add("gathered_nested", func(b *code.Builder, c code.Expr) {
		push(b, c, 1)
		outer := b.Let("outer", code.C("term.kids"))
		push(b, c, 2)
		inner := b.Let("inner", code.C("term.kids"))
		b.Do("act.pushAll", c, code.C(ItemsOp, i(30), i(0)))
		b.Assert(kids(2), "the elements of an empty list add none")
		push(b, c, 3)
		b.Do("act.push", c, null)
		b.Do("term.dropKids", inner)
		b.Assert(kids(2), "the inner list's elements are dropped")
		push(b, c, 4)
		b.Do("term.dropKids", inner)
		b.Do("term.dropKids", outer)
		b.Assert(kids(1), "and then the outer list's, leaving what came before")
	})
	add("gathered_drop_nothing", func(b *code.Builder, c code.Expr) {
		b.Do("act.pushAll", c, code.C(ItemsOp, i(40), i(4)))
		k := b.Let("k", code.C("term.kids"))
		b.Do("term.dropKids", k)
		b.Assert(kids(4), "a drop to the current mark drops nothing")
		b.Do("act.pushAll", c, code.C(ItemsOp, i(50), i(1)))
		b.Do("term.dropKids", k)
		b.Do("term.dropKids", i(0))
		b.Assert(kids(0), "a drop to the start drops everything")
	})
	add("gathered_take", func(b *code.Builder, c code.Expr) {
		push(b, c, 1)
		outer := b.Let("outer", code.C("term.kids"))
		push(b, c, 2)
		inner := b.Let("inner", code.C("term.kids"))
		push(b, c, 3)
		b.Do("act.push", c, null)
		b.Do("act.pushAll", c, code.C(ItemsOp, i(10), i(2)))
		b.Let("t1", code.C("term.elemsAbove", inner))
		b.Assert(kids(2), "making the inner list takes its elements off, a null one and those of a list among them")
		b.Let("t2", code.C("term.elemsAbove", inner))
		b.Assert(kids(2), "a list of no elements takes none")
		x := b.Let("x", code.C(ItemsOp, i(20), i(3)))
		b.Do("term.keep", x, i(2))
		b.Do("term.keep", x, i(0))
		b.Assert(kids(4), "each element kept counts")
		b.Let("t3", code.C("term.elemsAbove", outer))
		b.Assert(kids(1), "making the outer list takes off what the inner one left and the elements kept")
		push(b, c, 4)
		b.Let("t4", code.C("term.elemsAbove", i(0)))
		b.Assert(kids(0), "taking off from the start takes everything")
	})
	return cs
}

// MadeCases returns the handwritten conformance cases of the values made, in one module: structs recorded
// and forgotten back to marks, as the predicate and the binding forget what a term made and an action
// what it recorded.
func MadeCases() []Case {
	m := TermStateModule()
	var cs []Case
	add := func(name string, body func(b *code.Builder, c code.Expr)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, termStateFeatures, func(b *code.Builder) {
			body(b, b.Let("c", code.C(ActOp)))
		})})
	}
	i := code.Int
	created := func(n int64) code.Expr { return code.Eq(code.C("term.created"), i(n)) }
	made := func(b *code.Builder, c code.Expr, id int64) { b.Do("act.made", c, code.C(StructOp, i(id))) }
	add("made_record", func(b *code.Builder, c code.Expr) {
		m0 := b.Let("m0", code.C("term.created"))
		b.Assert(created(0), "a parse starts with no values made")
		made(b, c, 1)
		made(b, c, 2)
		m1 := b.Let("m1", code.C("term.created"))
		b.Assert(created(2), "each struct recorded counts")
		made(b, c, 3)
		b.Do("term.dropCreated", m1)
		b.Assert(created(2), "a drop keeps the values up to its mark")
		made(b, c, 4)
		b.Do("term.dropCreated", m0)
		b.Assert(created(0), "and forgets those after it")
	})
	add("made_nested", func(b *code.Builder, c code.Expr) {
		made(b, c, 1)
		outer := b.Let("outer", code.C("term.created"))
		made(b, c, 2)
		inner := b.Let("inner", code.C("term.created"))
		b.Do("term.dropCreated", inner)
		b.Assert(created(2), "a drop to the current mark forgets nothing")
		made(b, c, 3)
		made(b, c, 4)
		b.Do("term.dropCreated", inner)
		b.Assert(created(2), "an inner term's values are forgotten")
		b.Do("term.dropCreated", outer)
		b.Assert(created(1), "and then the outer one's, leaving what came before")
	})
	return cs
}

// randomTermState returns a randomized case of n calls of the operations of opNames, drawn with rng, that
// meets every precondition: elements and structs are new, and a drop goes to a mark that an earlier call
// returned and that is at most the current length.
func randomTermState(name string, rng *rand.Rand, opNames []string, n int) Case {
	m := TermStateModule()
	mirror := ref.New(ref.Config{})
	type markEntry struct {
		expr code.Expr
		n    int64
	}
	var kidsMarks, madeMarks []markEntry
	f := m.AddFunc(name, nil, nil, termStateFeatures, func(b *code.Builder) {
		count := 0
		fresh := func() string { count++; return fmt.Sprintf("x%d", count) }
		next := int64(1)
		id := func(k int64) int64 { v := next; next += k; return v }
		c := b.Let("c", code.C(ActOp))
		call := func(opName string, args []any, exprs ...code.Expr) []any {
			o := m.Op(opName)
			if !mirror.Allows(o, args) {
				panic("conform: a randomized case breaks the precondition of " + opName)
			}
			res := mirror.Call(o, args)
			if len(o.Results) == 0 {
				b.Do(opName, exprs...)
				return res
			}
			x := b.Let(fresh(), code.C(opName, exprs...))
			switch opName {
			case "term.kids":
				kidsMarks = append(kidsMarks, markEntry{x, res[0].(int64)})
			case "term.created":
				madeMarks = append(madeMarks, markEntry{x, res[0].(int64)})
			}
			return res
		}
		// drop drops to a mark that is at most the current length, the latest one most often.
		drop := func(op string, marks []markEntry, length int) {
			var ok []markEntry
			for _, mk := range marks {
				if mk.n <= int64(length) {
					ok = append(ok, mk)
				}
			}
			if len(ok) == 0 {
				return
			}
			mk := ok[len(ok)-1]
			if rng.IntN(3) == 0 {
				mk = ok[rng.IntN(len(ok))]
			}
			call(op, []any{mk.n}, mk.expr)
		}
		for k := 0; count < n && k < 40*n; k++ {
			switch op := opNames[rng.IntN(len(opNames))]; op {
			case "act.push":
				if rng.IntN(5) == 0 {
					call(op, []any{ref.Act{}, nil}, c, code.Null(code.THandle(exec.Elem)))
				} else {
					e := id(1)
					call(op, []any{ref.Act{}, ref.Elem{ID: e}}, c, code.C(ElemOp, code.Int(e)))
				}
			case "act.pushAll":
				k := rng.Int64N(4)
				first := id(k)
				ids := make([]int64, k)
				for j := range ids {
					ids[j] = first + int64(j)
				}
				call(op, []any{ref.Act{}, &ref.Items{IDs: ids}}, c, code.C(ItemsOp, code.Int(first), code.Int(k)))
			case "act.made":
				s := id(1)
				call(op, []any{ref.Act{}, ref.Struct{ID: s}}, c, code.C(StructOp, code.Int(s)))
			case "term.kids", "term.created":
				call(op, nil)
			case "term.dropKids":
				drop(op, kidsMarks, len(mirror.Gathered()))
			case "term.elemsAbove":
				drop(op, kidsMarks, len(mirror.Gathered()))
			case "term.keep":
				k := 1 + rng.Int64N(3)
				first := id(k)
				ids := make([]int64, k)
				for j := range ids {
					ids[j] = first + int64(j)
				}
				x := b.Let(fresh(), code.C(ItemsOp, code.Int(first), code.Int(k)))
				at := rng.Int64N(k)
				call(op, []any{&ref.Items{IDs: ids}, at}, x, code.Int(at))
			case "term.dropCreated":
				drop(op, madeMarks, len(mirror.Made()))
			}
		}
	})
	return Case{Module: m, Func: f}
}

// RandomGathered returns a randomized case of n calls of the operations of the elements gathered, drawn with
// rng, that meets every precondition.
func RandomGathered(name string, rng *rand.Rand, n int) Case {
	return randomTermState(name, rng, GatheredOps, n)
}

// RandomMade returns a randomized case of n calls of the operations of the values made, drawn with rng, that
// meets every precondition.
func RandomMade(name string, rng *rand.Rand, n int) Case {
	return randomTermState(name, rng, MadeOps, n)
}

// TermStateBroken is a wrong version of one operation of what terms make, the negative control of the
// cases: a model or a target whose operation does what Edit does fails a case.
type TermStateBroken struct {
	Name, Op string
	// Edit returns the results of the broken operation for args, given a function that calls the
	// correct operation named op of the model or the target.
	Edit func(call func(op string, args ...any) []any, args []any) []any
}

// GatheredBrokenOps returns the broken versions of the operations of the elements gathered that the cases
// must tell from the correct ones.
func GatheredBrokenOps() []TermStateBroken {
	bs := termStateBroken("act.push", "act.pushAll", "term.kids", "term.dropKids", true)
	add := func(name, op string, edit func(call func(string, ...any) []any, a []any) []any) {
		bs = append(bs, TermStateBroken{Name: name, Op: op, Edit: edit})
	}
	const take, keep = "term.elemsAbove", "term.keep"
	add(take+" leaves the elements", take, func(call func(string, ...any) []any, a []any) []any {
		n := call("term.kids")[0].(int64) - a[0].(int64)
		res := call(take, a[0])
		for j := range n {
			call(keep, res[0], j)
		}
		return res
	})
	add(take+" takes one fewer", take, func(call func(string, ...any) []any, a []any) []any {
		return call(take, min(a[0].(int64)+1, call("term.kids")[0].(int64)))
	})
	add(keep+" is ignored", keep, func(call func(string, ...any) []any, a []any) []any { return nil })
	add(keep+" keeps the first element", keep, func(call func(string, ...any) []any, a []any) []any {
		return call(keep, a[0], int64(0))
	})
	return bs
}

// MadeBrokenOps returns the broken versions of the operations of the values made that the cases must tell
// from the correct ones.
func MadeBrokenOps() []TermStateBroken {
	return termStateBroken("act.made", "", "term.created", "term.dropCreated", false)
}

// termStateBroken returns the broken versions of the operations of a family: push records an entry,
// pushAll (if any) several, mark returns the length and drop cuts back to a mark. A family with null
// elements also has a push that records the null element in place of its argument.
func termStateBroken(push, pushAll, mark, drop string, nulls bool) []TermStateBroken {
	var bs []TermStateBroken
	add := func(name, op string, edit func(call func(string, ...any) []any, a []any) []any) {
		bs = append(bs, TermStateBroken{Name: name, Op: op, Edit: edit})
	}
	ignore := func(op string) {
		add(op+" is ignored", op, func(call func(string, ...any) []any, a []any) []any { return nil })
	}
	ignore(push)
	ignore(drop)
	if nulls {
		add(push+" pushes the null element", push, func(call func(string, ...any) []any, a []any) []any {
			return call(push, a[0], nil)
		})
	}
	if pushAll != "" {
		ignore(pushAll)
		add(pushAll+" pushes one element fewer", pushAll, func(call func(string, ...any) []any, a []any) []any {
			call(pushAll, a...)
			if n := call(mark)[0].(int64); n > 0 {
				call(drop, n-1)
			}
			return nil
		})
	}
	add(mark+" is one more", mark, func(call func(string, ...any) []any, a []any) []any {
		return []any{call(mark)[0].(int64) + 1}
	})
	add(drop+" keeps one more", drop, func(call func(string, ...any) []any, a []any) []any {
		return call(drop, min(a[0].(int64)+1, call(mark)[0].(int64)))
	})
	return bs
}
